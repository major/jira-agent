package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestSprintCurrentCommand(t *testing.T) {
	t.Parallel()

	t.Run("active sprint found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/42/sprint" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/42/sprint")
			}
			if got := r.URL.Query().Get("state"); got != "active" {
				t.Errorf("state param = %q, want %q", got, "active")
			}
			testhelpers.WriteJSONResponse(t, w, `{
				"values": [{"id":100,"name":"Sprint 1","state":"active","goal":"Ship v1"}],
				"total": 1,
				"startAt": 0,
				"maxResults": 50,
				"isLast": true
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			sprintCurrentCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
			"--board-id", "42",
		)

		// Single active sprint should be returned as an object (not array).
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}
		if got, ok := envelope.Data["name"].(string); !ok || got != "Sprint 1" {
			t.Errorf("data.name = %v, want %q", envelope.Data["name"], "Sprint 1")
		}
	})

	t.Run("multiple active sprints", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testhelpers.WriteJSONResponse(t, w, `{
				"values": [
					{"id":100,"name":"Sprint 1","state":"active"},
					{"id":101,"name":"Sprint 2","state":"active"}
				],
				"total": 2,
				"startAt": 0,
				"maxResults": 50,
				"isLast": true
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			sprintCurrentCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
			"--board-id", "42",
		)

		// Multiple active sprints should be returned as an array.
		var envelope struct {
			Data []any `json:"data"`
		}
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}
		if len(envelope.Data) != 2 {
			t.Errorf("data length = %d, want 2", len(envelope.Data))
		}
	})

	t.Run("no active sprint returns NotFoundError", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testhelpers.WriteJSONResponse(t, w, `{
				"values": [],
				"total": 0,
				"startAt": 0,
				"maxResults": 50,
				"isLast": true
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := sprintCurrentCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat())
		prepareCommandForTest(cmd)
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--board-id", "42"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for empty values, got nil")
		}

		var notFound *apperr.NotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("error type = %T, want *apperr.NotFoundError", err)
		}
		wantNextCommand := "jira-agent sprint list --board-id 42 --state active"
		if got := notFound.NextCommand(); got != wantNextCommand {
			t.Errorf("NextCommand() = %q, want %q", got, wantNextCommand)
		}
	})

	t.Run("invalid board ID returns ValidationError", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := sprintCurrentCommand(testCommandClient("http://unused"), &buf, testCommandFormat())
		prepareCommandForTest(cmd)
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--board-id", "abc"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for invalid board ID, got nil")
		}

		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
	})
}

func TestSprintSummarize(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/agile/1.0/sprint/42":
				testhelpers.WriteJSONResponse(t, w, `{
					"id": 42,
					"name": "Sprint 42",
					"state": "active",
					"startDate": "2026-05-01T00:00:00.000Z",
					"endDate": "2026-05-15T00:00:00.000Z"
				}`)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field/search":
				if got := r.URL.Query().Get("query"); got != "story points" {
					t.Errorf("query = %q, want %q", got, "story points")
				}
				if got := r.URL.Query().Get("type"); got != "custom" {
					t.Errorf("type = %q, want %q", got, "custom")
				}
				testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"customfield_10016","name":"Story Points"}]}`)
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
				assertSprintSummarySearchBody(t, r, "sprint = 42", []string{"status", "customfield_10016"})
				testhelpers.WriteJSONResponse(t, w, `{
					"total": 3,
					"startAt": 0,
					"maxResults": 100,
					"issues": [
						{"fields":{"status":{"name":"To Do"},"customfield_10016":3}},
						{"fields":{"status":{"name":"In Progress"},"customfield_10016":5}},
						{"fields":{"status":{"name":"To Do"},"customfield_10016":2}}
					]
				}`)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, sprintSummarizeCommand(testSprintSummaryClient(server.URL), &buf, testCommandFormat()), "42")

		envelope := decodeSprintSummaryEnvelope(t, buf.Bytes())
		assertNumber(t, envelope.Data["sprint"].(map[string]any)["id"], 42)
		if got := envelope.Data["sprint"].(map[string]any)["state"]; got != "active" {
			t.Errorf("sprint.state = %v, want %q", got, "active")
		}
		assertNumber(t, envelope.Data["issues"].(map[string]any)["total"], 3)
		assertNumber(t, envelope.Data["issues"].(map[string]any)["by_status"].(map[string]any)["To Do"], 2)
		assertNumber(t, envelope.Data["issues"].(map[string]any)["by_status"].(map[string]any)["In Progress"], 1)
		storyPoints := envelope.Data["story_points"].(map[string]any)
		assertNumber(t, storyPoints["total"], 10)
		assertNumber(t, storyPoints["by_status"].(map[string]any)["To Do"], 5)
		assertNumber(t, storyPoints["by_status"].(map[string]any)["In Progress"], 5)
		if got := storyPoints["field"]; got != "customfield_10016" {
			t.Errorf("story_points.field = %v, want %q", got, "customfield_10016")
		}
	})

	t.Run("zero_issues", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/agile/1.0/sprint/42":
				testhelpers.WriteJSONResponse(t, w, `{"id":42,"name":"Sprint 42","state":"active"}`)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field/search":
				testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"customfield_10016","name":"Story Points"}]}`)
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
				testhelpers.WriteJSONResponse(t, w, `{"total":0,"startAt":0,"maxResults":100,"issues":[]}`)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, sprintSummarizeCommand(testSprintSummaryClient(server.URL), &buf, testCommandFormat()), "42")

		envelope := decodeSprintSummaryEnvelope(t, buf.Bytes())
		assertNumber(t, envelope.Data["issues"].(map[string]any)["total"], 0)
		if got := len(envelope.Data["issues"].(map[string]any)["by_status"].(map[string]any)); got != 0 {
			t.Errorf("by_status length = %d, want 0", got)
		}
		if envelope.Data["story_points"] != nil {
			t.Errorf("story_points = %v, want nil", envelope.Data["story_points"])
		}
	})

	t.Run("field_discovery_filters_by_name", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/agile/1.0/sprint/42":
				testhelpers.WriteJSONResponse(t, w, `{"id":42,"name":"Sprint 42","state":"active"}`)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field/search":
				// Return an unrelated field first, then the real Story Points field.
				testhelpers.WriteJSONResponse(t, w, `{"values":[
					{"id":"customfield_99999","name":"Story Points Estimate (Custom)"},
					{"id":"customfield_10016","name":"Story Points"}
				]}`)
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
				assertSprintSummarySearchBody(t, r, "sprint = 42", []string{"status", "customfield_10016"})
				testhelpers.WriteJSONResponse(t, w, `{
					"total": 1,
					"startAt": 0,
					"maxResults": 100,
					"issues": [{"fields":{"status":{"name":"In Progress"},"customfield_10016":3}}]
				}`)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, sprintSummarizeCommand(testSprintSummaryClient(server.URL), &buf, testCommandFormat()), "42")

		envelope := decodeSprintSummaryEnvelope(t, buf.Bytes())
		sp, ok := envelope.Data["story_points"].(map[string]any)
		if !ok {
			t.Fatal("story_points missing or wrong type")
		}
		if sp["field"] != "customfield_10016" {
			t.Errorf("story_points.field = %v, want customfield_10016", sp["field"])
		}
	})

	t.Run("no_story_points_field", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/agile/1.0/sprint/42":
				testhelpers.WriteJSONResponse(t, w, `{"id":42,"name":"Sprint 42","state":"active"}`)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field/search":
				testhelpers.WriteJSONResponse(t, w, `{"values":[]}`)
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
				assertSprintSummarySearchBody(t, r, "sprint = 42", []string{"status"})
				testhelpers.WriteJSONResponse(t, w, `{
					"total": 1,
					"startAt": 0,
					"maxResults": 100,
					"issues": [{"fields":{"status":{"name":"Done"}}}]
				}`)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, sprintSummarizeCommand(testSprintSummaryClient(server.URL), &buf, testCommandFormat()), "42")

		envelope := decodeSprintSummaryEnvelope(t, buf.Bytes())
		if envelope.Data["story_points"] != nil {
			t.Errorf("story_points = %v, want nil", envelope.Data["story_points"])
		}
	})

	t.Run("explicit_sp_field", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/agile/1.0/sprint/42":
				testhelpers.WriteJSONResponse(t, w, `{"id":42,"name":"Sprint 42","state":"active"}`)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field/search":
				t.Fatal("field search endpoint should not be called with explicit story points field")
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
				assertSprintSummarySearchBody(t, r, "sprint = 42", []string{"status", "customfield_10016"})
				testhelpers.WriteJSONResponse(t, w, `{
					"total": 1,
					"startAt": 0,
					"maxResults": 100,
					"issues": [{"fields":{"status":{"name":"Done"},"customfield_10016":8}}]
				}`)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			sprintSummarizeCommand(testSprintSummaryClient(server.URL), &buf, testCommandFormat()),
			"42", "--story-points-field", "customfield_10016",
		)

		envelope := decodeSprintSummaryEnvelope(t, buf.Bytes())
		storyPoints := envelope.Data["story_points"].(map[string]any)
		if got := storyPoints["field"]; got != "customfield_10016" {
			t.Errorf("story_points.field = %v, want %q", got, "customfield_10016")
		}
	})

	t.Run("invalid_sprint_id", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := sprintSummarizeCommand(testCommandClient("http://unused"), &buf, testCommandFormat())
		prepareCommandForTest(cmd)
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"abc"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for invalid sprint ID, got nil")
		}

		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
	})
}

func assertSprintSummarySearchBody(t *testing.T, r *http.Request, wantJQL string, wantFields []string) {
	t.Helper()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var body struct {
		JQL        string   `json:"jql"`
		Fields     []string `json:"fields"`
		MaxResults int      `json:"maxResults"`
		StartAt    int      `json:"startAt"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if body.JQL != wantJQL {
		t.Errorf("jql = %q, want %q", body.JQL, wantJQL)
	}
	if body.MaxResults != 100 {
		t.Errorf("maxResults = %d, want 100", body.MaxResults)
	}
	if body.StartAt != 0 {
		t.Errorf("startAt = %d, want 0", body.StartAt)
	}
	if len(body.Fields) != len(wantFields) {
		t.Fatalf("fields = %v, want %v", body.Fields, wantFields)
	}
	for i, want := range wantFields {
		if body.Fields[i] != want {
			t.Errorf("fields[%d] = %q, want %q", i, body.Fields[i], want)
		}
	}
}

func testSprintSummaryClient(serverURL string) *client.Ref {
	return &client.Ref{Client: client.NewClient(
		"Basic token",
		client.WithBaseURL(serverURL+"/rest/api/3"),
		client.WithAgileBaseURL(serverURL+"/rest/agile/1.0"),
	)}
}

func decodeSprintSummaryEnvelope(t *testing.T, outputBytes []byte) struct {
	Data map[string]any `json:"data"`
} {
	t.Helper()

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(outputBytes, &envelope); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	return envelope
}

func assertNumber(t *testing.T, got any, want float64) {
	t.Helper()

	gotNumber, ok := got.(float64)
	if !ok {
		t.Fatalf("value = %v (%T), want number", got, got)
	}
	if gotNumber != want {
		t.Errorf("number = %v, want %v", gotNumber, want)
	}
}
