package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestIssueSearchFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields string
		want   []string
	}{
		{
			name:   "removes top-level key",
			fields: "key, summary, status, assignee",
			want:   []string{"summary", "status", "assignee"},
		},
		{
			name:   "key only leaves no Jira fields",
			fields: "key",
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := issueSearchFields(tt.fields)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issueSearchFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlattenIssueSearchResult(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"expand":        "schema,names",
		"nextPageToken": "next-page",
		"isLast":        false,
		"issues": []any{
			map[string]any{
				"id":     "10001",
				"key":    "PROJ-1",
				"self":   "https://example.atlassian.net/rest/api/3/issue/10001",
				"expand": "operations",
				"fields": map[string]any{
					"summary": "Akamai WAF blocks requests",
					"status": map[string]any{
						"self": "https://example.atlassian.net/rest/api/3/status/3",
						"name": "Review",
						"statusCategory": map[string]any{
							"name": "In Progress",
						},
					},
					"assignee": map[string]any{
						"accountId":   "abc123",
						"accountType": "atlassian",
						"active":      true,
						"avatarUrls": map[string]any{
							"48x48": "https://avatar.example/48",
						},
						"displayName": "Sam Doran",
						"self":        "https://example.atlassian.net/rest/api/3/user?accountId=abc123",
						"timeZone":    "America/Chicago",
					},
					"priority": map[string]any{
						"id":      "1",
						"iconUrl": "https://example.atlassian.net/images/icons/priorities/blocker.svg",
						"name":    "Blocker",
						"self":    "https://example.atlassian.net/rest/api/3/priority/1",
					},
					"components": []any{
						map[string]any{"id": "10", "name": "API", "self": "https://example/component/10"},
					},
					"customfield_10028": float64(8),
					"customfield_10020": []any{
						map[string]any{"id": float64(12), "name": "Sprint 12", "self": "https://example/sprint/12"},
					},
				},
			},
		},
	}

	got := flattenIssueSearchResult(result)
	want := map[string]any{
		"nextPageToken": "next-page",
		"isLast":        false,
		"issues": []map[string]any{
			{
				"key":               "PROJ-1",
				"summary":           "Akamai WAF blocks requests",
				"status":            "Review",
				"assignee":          "Sam Doran",
				"priority":          "Blocker",
				"components":        []any{"API"},
				"customfield_10028": float64(8),
				"customfield_10020": []any{"Sprint 12"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("flattenIssueSearchResult() = %#v, want %#v", got, want)
	}
}

func TestIssueSearchCommand_FlattensJSONByDefault(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/search/jql" {
			t.Errorf("path = %s, want /search/jql", r.URL.Path)
		}

		body := testhelpers.DecodeJSONBody(t, r)
		assertRequestedFields(t, body, []string{"summary", "status", "assignee", "priority"})

		testhelpers.WriteJSONResponse(t, w, issueSearchResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,summary,status,assignee,priority",
	)

	if strings.Contains(buf.String(), "avatarUrls") {
		t.Fatalf("flat output contains avatarUrls: %s", buf.String())
	}
	if strings.Contains(buf.String(), "statusCategory") {
		t.Fatalf("flat output contains statusCategory: %s", buf.String())
	}

	issue := decodeFirstSearchIssue(t, buf.Bytes())
	wantIssue := map[string]any{
		"key":               "RSPEED-2911",
		"summary":           "Akamai WAF blocks requests",
		"status":            "Review",
		"assignee":          "Sam Doran",
		"priority":          "Blocker",
		"customfield_10028": float64(8),
	}
	if !reflect.DeepEqual(issue, wantIssue) {
		t.Errorf("issue = %#v, want %#v", issue, wantIssue)
	}
}

func TestIssueSearchCommand_ConvertsDescriptionOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := testhelpers.DecodeJSONBody(t, r)
		assertRequestedFields(t, body, []string{"description"})
		testhelpers.WriteJSONResponse(t, w, issueSearchDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,description",
		"--description-output-format", "markdown",
	)

	issue := decodeFirstSearchIssueWithoutMetadataCheck(t, buf.Bytes())
	if issue["description"] != "## Goal\nFix the login bug\n- First bullet" {
		t.Errorf("description = %#v, want markdown text", issue["description"])
	}
	if strings.Contains(buf.String(), `"type":"doc"`) {
		t.Fatalf("description output contains ADF JSON: %s", buf.String())
	}
}

func TestIssueSearchCommand_RawJSONPreservesDescriptionADF(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueSearchDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,description",
		"--description-output-format", "markdown",
		"--raw",
	)

	if !strings.Contains(buf.String(), `"type":"doc"`) {
		t.Fatalf("raw description output missing ADF JSON: %s", buf.String())
	}
}

func TestIssueSearchCommand_RawJSONIgnoresDescriptionOutputFormat(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueSearchDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,description",
		"--description-output-format", "wiki",
		"--raw",
	)

	if !strings.Contains(buf.String(), `"type":"doc"`) {
		t.Fatalf("raw description output missing ADF JSON: %s", buf.String())
	}
}

func TestIssueSearchCommand_SendsEmptyFieldsForKeyOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := testhelpers.DecodeJSONBody(t, r)
		assertRequestedFields(t, body, []string{})
		testhelpers.WriteJSONResponse(t, w, `{"issues":[{"key":"RSPEED-2911","fields":{}}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key",
	)

	issue := decodeFirstSearchIssueWithoutMetadataCheck(t, buf.Bytes())
	wantIssue := map[string]any{"key": "RSPEED-2911"}
	if !reflect.DeepEqual(issue, wantIssue) {
		t.Errorf("issue = %#v, want %#v", issue, wantIssue)
	}
}

func TestIssueSearchCommand_RawJSONPreservesAPIResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := testhelpers.DecodeJSONBody(t, r)
		assertRequestedFields(t, body, []string{"summary", "status", "assignee", "priority"})
		testhelpers.WriteJSONResponse(t, w, issueSearchResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,summary,status,assignee,priority",
		"--raw",
	)

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	for _, rawField := range []string{"avatarUrls", "accountType", "timeZone", "iconUrl", "\"self\"", "\"expand\"", "statusCategory"} {
		if !strings.Contains(buf.String(), rawField) {
			t.Fatalf("raw issue search output missing field %q: %s", rawField, buf.String())
		}
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	issue, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("issue type = %T, want map[string]any", issues[0])
	}
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields type = %T, want map[string]any", issue["fields"])
	}
	assignee, ok := fields["assignee"].(map[string]any)
	if !ok {
		t.Fatalf("assignee type = %T, want map[string]any", fields["assignee"])
	}
	if assignee["accountId"] != "abc123" {
		t.Errorf("assignee accountId = %v, want abc123", assignee["accountId"])
	}
}

func TestIssueSearchCommand_CSVPreservesAPIResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := testhelpers.DecodeJSONBody(t, r)
		assertRequestedFields(t, body, []string{"summary", "status", "assignee", "priority"})
		testhelpers.WriteJSONResponse(t, w, issueSearchResponseJSON())
	}))
	defer server.Close()

	format := output.FormatCSV
	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, &format),
		"--jql", "project = RSPEED",
		"--fields", "key,summary,status,assignee,priority",
	)

	got := buf.String()
	if !strings.Contains(got, "avatarUrls") {
		t.Fatalf("CSV output did not preserve nested API response: %s", got)
	}
	if !strings.Contains(got, "statusCategory") {
		t.Fatalf("CSV output did not preserve statusCategory: %s", got)
	}
	if strings.Contains(got, "Sam Doran,Blocker,Review") {
		t.Fatalf("CSV output unexpectedly used flattened issue row shape: %s", got)
	}
}

func TestIssueGetCommand_CompactsJSONByDefault(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/issue/RSPEED-2911" {
			t.Errorf("path = %s, want /issue/RSPEED-2911", r.URL.Path)
		}
		testhelpers.WriteJSONResponse(t, w, issueGetResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "RSPEED-2911")

	for _, noisy := range []string{"avatarUrls", "accountType", "timeZone", "iconUrl", "\"self\"", "\"expand\"", "colorName"} {
		if strings.Contains(buf.String(), noisy) {
			t.Fatalf("compact issue get output contains noisy field %q: %s", noisy, buf.String())
		}
	}

	issue := decodeIssueData(t, buf.Bytes())
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields type = %T, want map[string]any", issue["fields"])
	}
	assignee, ok := fields["assignee"].(map[string]any)
	if !ok {
		t.Fatalf("assignee type = %T, want map[string]any", fields["assignee"])
	}
	if assignee["accountId"] != "abc123" {
		t.Errorf("assignee accountId = %v, want abc123", assignee["accountId"])
	}
	if assignee["displayName"] != "Sam Doran" {
		t.Errorf("assignee displayName = %v, want Sam Doran", assignee["displayName"])
	}
	status, ok := fields["status"].(map[string]any)
	if !ok {
		t.Fatalf("status type = %T, want map[string]any", fields["status"])
	}
	if status["statusCategory"] != "In Progress" {
		t.Errorf("statusCategory = %v, want In Progress", status["statusCategory"])
	}
}

func TestIssueGetCommand_ConvertsDescriptionOutputByDefault(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueGetDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "RSPEED-2911")

	issue := decodeIssueData(t, buf.Bytes())
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields type = %T, want map[string]any", issue["fields"])
	}
	if fields["description"] != "Goal\nFix the login bug\nFirst bullet" {
		t.Errorf("description = %#v, want plain text", fields["description"])
	}
}

func TestIssueGetCommand_ADFDescriptionOutputPreservesADF(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueGetDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"RSPEED-2911",
		"--description-output-format", "adf",
	)

	if !strings.Contains(buf.String(), `"type":"doc"`) {
		t.Fatalf("ADF description output missing ADF JSON: %s", buf.String())
	}
}

func TestIssueGetCommand_RawJSONIgnoresDescriptionOutputFormat(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueGetDescriptionResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"RSPEED-2911",
		"--description-output-format", "wiki",
		"--raw",
	)

	if !strings.Contains(buf.String(), `"type":"doc"`) {
		t.Fatalf("raw description output missing ADF JSON: %s", buf.String())
	}
}

func TestIssueGetCommand_RawCSVPreservesDescriptionADF(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueGetDescriptionResponseJSON())
	}))
	defer server.Close()

	format := output.FormatCSV
	var buf bytes.Buffer
	runCommandAction(
		t,
		issueGetCommand(testCommandClient(server.URL), &buf, &format),
		"RSPEED-2911",
		"--raw",
	)

	got := buf.String()
	if !strings.Contains(got, `""type"":""doc""`) {
		t.Fatalf("raw CSV description output missing ADF JSON: %s", got)
	}
	if strings.Contains(got, "Goal\nFix the login bug\nFirst bullet") {
		t.Fatalf("raw CSV description output was converted to text: %s", got)
	}
}

func TestIssueGetCommand_RawJSONPreservesAPIResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, issueGetResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "RSPEED-2911", "--raw")

	for _, rawField := range []string{"avatarUrls", "accountType", "timeZone", "iconUrl", "\"self\"", "\"expand\"", "colorName"} {
		if !strings.Contains(buf.String(), rawField) {
			t.Fatalf("raw issue get output missing field %q: %s", rawField, buf.String())
		}
	}
}

func assertRequestedFields(t *testing.T, body map[string]any, want []string) {
	t.Helper()

	fields, ok := body["fields"].([]any)
	if !ok {
		t.Fatalf("fields type = %T, want []any", body["fields"])
	}
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		gotField, ok := field.(string)
		if !ok {
			t.Fatalf("field type = %T, want string", field)
		}
		got = append(got, gotField)
	}
	if !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

func decodeFirstSearchIssue(t *testing.T, outputBytes []byte) map[string]any {
	t.Helper()

	var env output.Envelope
	if err := json.Unmarshal(outputBytes, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Metadata.Total != 22 {
		t.Errorf("metadata total = %d, want 22", env.Metadata.Total)
	}
	if env.Metadata.Returned != 1 {
		t.Errorf("metadata returned = %d, want 1", env.Metadata.Returned)
	}
	if env.Metadata.MaxResults != 50 {
		t.Errorf("metadata max results = %d, want 50", env.Metadata.MaxResults)
	}

	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	if len(issues) != 1 {
		t.Fatalf("issues length = %d, want 1", len(issues))
	}
	issue, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("issue type = %T, want map[string]any", issues[0])
	}
	if data["nextPageToken"] != "next-page" {
		t.Errorf("nextPageToken = %v, want next-page", data["nextPageToken"])
	}
	return issue
}

func decodeFirstSearchIssueWithoutMetadataCheck(t *testing.T, outputBytes []byte) map[string]any {
	t.Helper()

	var env output.Envelope
	if err := json.Unmarshal(outputBytes, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	issues, ok := data["issues"].([]any)
	if !ok {
		t.Fatalf("issues type = %T, want []any", data["issues"])
	}
	if len(issues) != 1 {
		t.Fatalf("issues length = %d, want 1", len(issues))
	}
	issue, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("issue type = %T, want map[string]any", issues[0])
	}
	return issue
}

func issueSearchResponseJSON() string {
	return `{
		"expand": "schema,names",
		"maxResults": 50,
		"nextPageToken": "next-page",
		"total": 22,
		"issues": [
			{
				"id": "10001",
				"key": "RSPEED-2911",
				"self": "https://example.atlassian.net/rest/api/3/issue/10001",
				"expand": "operations",
				"fields": {
					"summary": "Akamai WAF blocks requests",
					"status": {"self":"https://example/status/3","name":"Review","statusCategory":{"name":"In Progress"}},
					"assignee": {"accountId":"abc123","accountType":"atlassian","active":true,"avatarUrls":{"48x48":"https://avatar.example/48"},"displayName":"Sam Doran","self":"https://example/user","timeZone":"America/Chicago"},
					"priority": {"id":"1","self":"https://example/priority/1","iconUrl":"https://example/icon.svg","name":"Blocker"},
					"customfield_10028": 8
				}
			}
		]
	}`
}

func issueSearchDescriptionResponseJSON() string {
	return `{
		"maxResults": 50,
		"total": 1,
		"issues": [
			{
				"key": "RSPEED-2911",
				"fields": {
					"description": {
						"type":"doc",
						"version":1,
						"content":[
							{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Goal"}]},
							{"type":"paragraph","content":[{"type":"text","text":"Fix the login bug"}]},
							{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"First bullet"}]}]}]}
						]
					}
				}
			}
		]
	}`
}

func decodeIssueData(t *testing.T, outputBytes []byte) map[string]any {
	t.Helper()

	var env output.Envelope
	if err := json.Unmarshal(outputBytes, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	issue, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	return issue
}

func issueGetResponseJSON() string {
	return `{
		"expand": "renderedFields,names",
		"id": "10001",
		"key": "RSPEED-2911",
		"self": "https://example.atlassian.net/rest/api/3/issue/10001",
		"fields": {
			"summary": "Akamai WAF blocks requests",
			"status": {
				"self": "https://example/status/3",
				"iconUrl": "https://example/status.svg",
				"name": "Review",
				"statusCategory": {
					"colorName": "yellow",
					"id": 4,
					"key": "indeterminate",
					"name": "In Progress",
					"self": "https://example/statuscategory/4"
				}
			},
			"assignee": {
				"accountId": "abc123",
				"accountType": "atlassian",
				"active": true,
				"avatarUrls": {"48x48": "https://avatar.example/48"},
				"displayName": "Sam Doran",
				"emailAddress": "sdoran@example.com",
				"self": "https://example/user",
				"timeZone": "America/Chicago"
			},
			"priority": {"id":"1","self":"https://example/priority/1","iconUrl":"https://example/icon.svg","name":"Blocker"}
		}
	}`
}

func issueGetDescriptionResponseJSON() string {
	return `{
		"id": "10001",
		"key": "RSPEED-2911",
		"fields": {
			"description": {
				"type":"doc",
				"version":1,
				"content":[
					{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Goal"}]},
					{"type":"paragraph","content":[{"type":"text","text":"Fix the login bug"}]},
					{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"First bullet"}]}]}]}
				]
			}
		}
	}`
}
