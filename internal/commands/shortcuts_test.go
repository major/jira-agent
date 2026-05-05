package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestIssueMine(t *testing.T) {
	t.Parallel()

	t.Run("default JQL", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/search/jql" {
				t.Errorf("got %s %s, want POST /search/jql", r.Method, r.URL.Path)
			}
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		wantJQL := "assignee = currentUser() ORDER BY updated DESC"
		if gotBody["jql"] != wantJQL {
			t.Errorf("jql = %v, want %v", gotBody["jql"], wantJQL)
		}
	})

	t.Run("with status filter", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--status", "In Progress",
		)

		wantJQL := "assignee = currentUser() AND status = 'In Progress' ORDER BY updated DESC"
		if gotBody["jql"] != wantJQL {
			t.Errorf("jql = %v, want %v", gotBody["jql"], wantJQL)
		}
	})

	t.Run("default pagination is 50", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		if got, ok := gotBody["maxResults"].(float64); !ok || int(got) != 50 {
			t.Errorf("maxResults = %v, want 50", gotBody["maxResults"])
		}
	})

	t.Run("fields-preset expands to fields", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--fields-preset", "minimal",
		)

		// "minimal" preset: key,summary,status -> issueSearchFields strips "key" -> [summary, status]
		fields, ok := gotBody["fields"].([]any)
		if !ok {
			t.Fatalf("fields type = %T, want []any", gotBody["fields"])
		}
		if len(fields) != 2 {
			t.Errorf("fields length = %d, want 2", len(fields))
		}
	})

	t.Run("flattens JSON search results", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testhelpers.WriteJSONResponse(t, w, `{
				"issues": [{"key": "PROJ-1", "fields": {"summary": "Test issue", "status": {"name": "Open"}}}],
				"total": 1,
				"maxResults": 50,
				"startAt": 0
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueMineCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		issues, ok := data["issues"].([]any)
		if !ok {
			t.Fatalf("issues type = %T, want []any", data["issues"])
		}
		if len(issues) != 1 {
			t.Errorf("issues length = %d, want 1", len(issues))
		}
		issue, ok := issues[0].(map[string]any)
		if !ok {
			t.Fatalf("issues[0] type = %T, want map[string]any", issues[0])
		}
		if issue["key"] != "PROJ-1" {
			t.Errorf("key = %v, want PROJ-1", issue["key"])
		}
		// Flattened: status should be scalar "Open", not nested object
		if issue["status"] != "Open" {
			t.Errorf("status = %v, want Open (flattened)", issue["status"])
		}
	})
}

func TestIssueRecent(t *testing.T) {
	t.Parallel()

	t.Run("default JQL with 7d window", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/search/jql" {
				t.Errorf("got %s %s, want POST /search/jql", r.Method, r.URL.Path)
			}
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueRecentCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		wantJQL := `(assignee = currentUser() OR reporter = currentUser() OR watcher = currentUser()) AND updated >= "-7d" ORDER BY updated DESC`
		if gotBody["jql"] != wantJQL {
			t.Errorf("jql = %v, want %v", gotBody["jql"], wantJQL)
		}
	})

	t.Run("default max-results is 10", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueRecentCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		if got, ok := gotBody["maxResults"].(float64); !ok || int(got) != 10 {
			t.Errorf("maxResults = %v, want 10", gotBody["maxResults"])
		}
	})

	t.Run("custom since overrides window", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueRecentCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--since", "1d",
		)

		wantJQL := `(assignee = currentUser() OR reporter = currentUser() OR watcher = currentUser()) AND updated >= "-1d" ORDER BY updated DESC`
		if gotBody["jql"] != wantJQL {
			t.Errorf("jql = %v, want %v", gotBody["jql"], wantJQL)
		}
	})

	t.Run("fields-preset expands to fields", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody = testhelpers.DecodeJSONBody(t, r)
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"total":0}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueRecentCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--fields-preset", "triage",
		)

		// "triage" preset: key,summary,status,priority,assignee,labels -> strips "key" -> 5 fields
		fields, ok := gotBody["fields"].([]any)
		if !ok {
			t.Fatalf("fields type = %T, want []any", gotBody["fields"])
		}
		if len(fields) != 5 {
			t.Errorf("fields length = %d, want 5", len(fields))
		}
	})

	t.Run("flattens JSON search results", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testhelpers.WriteJSONResponse(t, w, `{
				"issues": [{"key": "PROJ-5", "fields": {"summary": "Recent task", "status": {"name": "Done"}}}],
				"total": 1,
				"maxResults": 10,
				"startAt": 0
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueRecentCommand(testCommandClient(server.URL), &buf, testCommandFormat()))

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		issues, ok := data["issues"].([]any)
		if !ok {
			t.Fatalf("issues type = %T, want []any", data["issues"])
		}
		if len(issues) != 1 {
			t.Errorf("issues length = %d, want 1", len(issues))
		}
		issue, ok := issues[0].(map[string]any)
		if !ok {
			t.Fatalf("issues[0] type = %T, want map[string]any", issues[0])
		}
		if issue["key"] != "PROJ-5" {
			t.Errorf("key = %v, want PROJ-5", issue["key"])
		}
		if issue["status"] != "Done" {
			t.Errorf("status = %v, want Done (flattened)", issue["status"])
		}
	})
}
