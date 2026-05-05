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
	flat, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("flattenIssueSearchResult() type = %T, want map[string]any", got)
	}
	for _, key := range []string{"nextPageToken", "isLast"} {
		if _, ok := flat[key]; ok {
			t.Errorf("flattenIssueSearchResult() included %q, want omitted", key)
		}
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

func TestIssueSearchCommand_CursorPaginationMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/search/jql" {
			t.Errorf("path = %s, want /search/jql", r.URL.Path)
		}
		testhelpers.WriteJSONResponse(t, w, issueSearchResponseJSON())
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--max-results", "50",
		"--fields", "key,summary",
	)

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Type != "cursor" {
		t.Errorf("pagination type = %q, want cursor", pagination.Type)
	}
	if !pagination.HasMore {
		t.Error("pagination has_more = false, want true")
	}
	if pagination.NextToken != "next-page" {
		t.Errorf("pagination next_token = %q, want next-page", pagination.NextToken)
	}
	wantNextCommand := "search --fields key,summary --jql \"project = RSPEED\" --max-results 50 --next-page-token next-page"
	if pagination.NextCommand != wantNextCommand {
		t.Errorf("pagination next_command = %q, want %q", pagination.NextCommand, wantNextCommand)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	for _, key := range []string{"nextPageToken", "isLast"} {
		if _, ok := data[key]; ok {
			t.Errorf("data included %q, want omitted", key)
		}
	}
}

func TestIssueSearchCommand_CursorPaginationMetadataLastPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{
			"maxResults": 50,
			"nextPageToken": "",
			"isLast": true,
			"issues": [{"key":"RSPEED-2911","fields":{"summary":"Done"}}]
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = RSPEED",
		"--fields", "key,summary",
	)

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Type != "cursor" {
		t.Errorf("pagination type = %q, want cursor", pagination.Type)
	}
	if pagination.HasMore {
		t.Error("pagination has_more = true, want false")
	}
	if pagination.NextToken != "" {
		t.Errorf("pagination next_token = %q, want empty", pagination.NextToken)
	}
	if pagination.NextCommand != "" {
		t.Errorf("pagination next_command = %q, want empty", pagination.NextCommand)
	}
}

func TestIssueMineCommand_OffsetPaginationMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := testhelpers.DecodeJSONBody(t, r)
		if body["startAt"] != float64(0) {
			t.Errorf("startAt = %v, want 0", body["startAt"])
		}
		testhelpers.WriteJSONResponse(t, w, `{
			"startAt": 0,
			"maxResults": 50,
			"total": 42,
			"issues": [{"key":"RSPEED-2911","fields":{"summary":"Mine"}}]
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Type != "offset" {
		t.Errorf("pagination type = %q, want offset", pagination.Type)
	}
	if pagination.Total != 42 {
		t.Errorf("pagination total = %d, want 42", pagination.Total)
	}
	if pagination.StartAt != 0 {
		t.Errorf("pagination start_at = %d, want 0", pagination.StartAt)
	}
	if pagination.MaxResults != 50 {
		t.Errorf("pagination max_results = %d, want 50", pagination.MaxResults)
	}
	if !pagination.HasMore {
		t.Error("pagination has_more = false, want true")
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

	env := decodeEnvelope(t, outputBytes)
	pagination := requirePaginationMetadata(t, &env)
	if pagination.Total != 22 {
		t.Errorf("metadata total = %d, want 22", pagination.Total)
	}
	if pagination.Returned != 1 {
		t.Errorf("metadata returned = %d, want 1", pagination.Returned)
	}
	if pagination.MaxResults != 50 {
		t.Errorf("metadata max results = %d, want 50", pagination.MaxResults)
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
	for _, key := range []string{"nextPageToken", "isLast"} {
		if _, ok := data[key]; ok {
			t.Errorf("data included %q, want omitted", key)
		}
	}
	return issue
}

func decodeEnvelope(t *testing.T, outputBytes []byte) output.Envelope {
	t.Helper()

	var env output.Envelope
	if err := json.Unmarshal(outputBytes, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

func requirePaginationMetadata(t *testing.T, env *output.Envelope) *output.PaginationMeta {
	t.Helper()

	if env.Metadata.Pagination == nil {
		t.Fatal("metadata pagination = nil, want populated")
	}
	return env.Metadata.Pagination
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

func TestIssueSearchRaw_CursorPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{
			"issues": [
				{"key":"PROJ-1","summary":"First issue"},
				{"key":"PROJ-2","summary":"Second issue"}
			],
			"nextPageToken": "cursor123",
			"isLast": false
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--jql", "project = PROJ",
		"--raw",
	)

	env := decodeEnvelope(t, buf.Bytes())
	pagination := requirePaginationMetadata(t, &env)

	// Verify cursor metadata is present
	if pagination.Type != "cursor" {
		t.Errorf("pagination type = %q, want cursor", pagination.Type)
	}
	if !pagination.HasMore {
		t.Error("pagination has_more = false, want true")
	}
	if pagination.NextToken != "cursor123" {
		t.Errorf("pagination next_token = %q, want cursor123", pagination.NextToken)
	}
	if pagination.NextCommand == "" {
		t.Error("pagination next_command is empty, want non-empty")
	}
	if !strings.Contains(pagination.NextCommand, "--next-page-token cursor123") {
		t.Errorf("pagination next_command = %q, want to contain --next-page-token cursor123", pagination.NextCommand)
	}

	// Verify raw data still contains cursor fields (raw = unmodified)
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if _, ok := data["nextPageToken"]; !ok {
		t.Error("raw data missing nextPageToken field (raw mode should preserve it)")
	}
	if _, ok := data["isLast"]; !ok {
		t.Error("raw data missing isLast field (raw mode should preserve it)")
	}
}
