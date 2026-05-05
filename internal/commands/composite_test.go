package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestCloseIssue(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 1: got %s %s, want GET /issue/PROJ-123", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"In Progress"},"resolution":null}}`)
			case 2:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 2: got %s %s, want GET /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`)
			case 3:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 3: got %s %s, want POST /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "31" {
					t.Errorf("transition id = %v, want 31", transition["id"])
				}
				fields := body["fields"].(map[string]any)
				resolution := fields["resolution"].(map[string]any)
				if resolution["name"] != "Done" {
					t.Errorf("resolution = %v, want Done", resolution["name"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCloseCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-123" {
			t.Errorf("key = %v, want PROJ-123", data["key"])
		}
		if data["transitioned"] != true {
			t.Errorf("transitioned = %v, want true", data["transitioned"])
		}
		if data["to"] != "Done" {
			t.Errorf("to = %v, want Done", data["to"])
		}
		if data["resolution"] != "Done" {
			t.Errorf("resolution = %v, want Done", data["resolution"])
		}
		if requests != 3 {
			t.Errorf("requests = %d, want 3", requests)
		}
	})

	t.Run("custom resolution", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/issue/PROJ-123":
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"resolution":null}}`)
			case "/issue/PROJ-123/transitions":
				if r.Method == http.MethodGet {
					testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`)
				} else {
					body := testhelpers.DecodeJSONBody(t, r)
					fields := body["fields"].(map[string]any)
					resolution := fields["resolution"].(map[string]any)
					if resolution["name"] != "Won't Do" {
						t.Errorf("resolution = %v, want Won't Do", resolution["name"])
					}
					w.WriteHeader(http.StatusNoContent)
				}
			default:
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCloseCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--resolution", "Won't Do", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["resolution"] != "Won't Do" {
			t.Errorf("resolution = %v, want Won't Do", data["resolution"])
		}
	})

	t.Run("with comment", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"In Progress"},"resolution":null}}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`)
			case 3:
				w.WriteHeader(http.StatusNoContent)
			case 4:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/comment" {
					t.Errorf("request 4: got %s %s, want POST /issue/PROJ-123/comment", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				if body["body"] == nil {
					t.Error("comment body = nil, want ADF content")
				}
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001"}`)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCloseCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--comment", "Closing this out", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["commented"] != true {
			t.Errorf("commented = %v, want true", data["commented"])
		}
		if requests != 4 {
			t.Errorf("requests = %d, want 4", requests)
		}
	})

	t.Run("transition not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/issue/PROJ-123":
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"resolution":null}}`)
			case "/issue/PROJ-123/transitions":
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"11","name":"In Progress","to":{"name":"In Progress"}},{"id":"12","name":"Won't Do","to":{"name":"Won't Do"}}]}`)
			default:
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCloseCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"PROJ-123"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want not found error")
		}

		var nfErr *apperr.NotFoundError
		if !errors.As(err, &nfErr) {
			t.Fatalf("error type = %T, want *apperr.NotFoundError", err)
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "Done") {
			t.Errorf("error = %q, want to mention target status Done", errStr)
		}
		actions := nfErr.AvailableActions()
		if len(actions) != 2 {
			t.Fatalf("available_actions length = %d, want 2", len(actions))
		}
		if actions[0] != "In Progress" || actions[1] != "Won't Do" {
			t.Errorf("available_actions = %v, want [In Progress, Won't Do]", actions)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"In Progress"},"resolution":null}}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`)
			default:
				t.Errorf("unexpected request %d: %s %s (dry-run should only GET)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		dryRunEnabled := true
		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCloseCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), &dryRunEnabled),
			"PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["command"] != "issue close" {
			t.Errorf("command = %v, want issue close", data["command"])
		}
		if data["issue_key"] != "PROJ-123" {
			t.Errorf("issue_key = %v, want PROJ-123", data["issue_key"])
		}

		diffRaw, ok := data["diff"].([]any)
		if !ok {
			t.Fatalf("diff type = %T, want []any", data["diff"])
		}
		if len(diffRaw) < 2 {
			t.Errorf("diff length = %d, want at least 2 (status + resolution)", len(diffRaw))
		}

		if requests != 2 {
			t.Errorf("requests = %d, want 2 (only GETs)", requests)
		}
	})

	t.Run("write blocked", func(t *testing.T) {
		t.Parallel()

		writesDisabled := false
		dryRunDisabled := false
		var buf bytes.Buffer
		cmd := issueCloseCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), &writesDisabled, &dryRunDisabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"PROJ-123"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})
}

func TestStartWork(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.Method != http.MethodGet || r.URL.Path != "/myself" {
					t.Errorf("request 1: got %s %s, want GET /myself", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case 2:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 2: got %s %s, want GET /issue/PROJ-123", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 3:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 3: got %s %s, want GET /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","name":"Start Progress","to":{"name":"In Progress"}}]}`)
			case 4:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 4: got %s %s, want POST /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "21" {
					t.Errorf("transition id = %v, want 21", transition["id"])
				}
				w.WriteHeader(http.StatusNoContent)
			case 5:
				if r.Method != http.MethodPut || r.URL.Path != "/issue/PROJ-123/assignee" {
					t.Errorf("request 5: got %s %s, want PUT /issue/PROJ-123/assignee", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				if body["accountId"] != "abc123" {
					t.Errorf("accountId = %v, want abc123", body["accountId"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-123" {
			t.Errorf("key = %v, want PROJ-123", data["key"])
		}
		if data["transitioned"] != true {
			t.Errorf("transitioned = %v, want true", data["transitioned"])
		}
		if data["assigned"] != true {
			t.Errorf("assigned = %v, want true", data["assigned"])
		}
		if data["to"] != "In Progress" {
			t.Errorf("to = %v, want In Progress", data["to"])
		}
		if requests != 5 {
			t.Errorf("requests = %d, want 5", requests)
		}
	})

	t.Run("with comment", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 3:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			case 4:
				w.WriteHeader(http.StatusNoContent)
			case 5:
				w.WriteHeader(http.StatusNoContent)
			case 6:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/comment" {
					t.Errorf("request 6: got %s %s, want POST /issue/PROJ-123/comment", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				if body["body"] == nil {
					t.Error("comment body = nil, want ADF content")
				}
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001"}`)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--comment", "Starting work", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["commented"] != true {
			t.Errorf("commented = %v, want true", data["commented"])
		}
		if requests != 6 {
			t.Errorf("requests = %d, want 6", requests)
		}
	})

	t.Run("custom assignee", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 1: path = %s, want /issue/PROJ-123", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			case 3:
				w.WriteHeader(http.StatusNoContent)
			case 4:
				body := testhelpers.DecodeJSONBody(t, r)
				if body["accountId"] != "def456" {
					t.Errorf("accountId = %v, want def456", body["accountId"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--assignee", "def456", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["assignee"] != "def456" {
			t.Errorf("assignee = %v, want def456", data["assignee"])
		}
		if requests != 4 {
			t.Errorf("requests = %d, want 4 (no /myself call)", requests)
		}
	})

	t.Run("partial failure assign", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 3:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			case 4:
				w.WriteHeader(http.StatusNoContent)
			case 5:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"errorMessages":["Permission denied"]}`))
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["transitioned"] != true {
			t.Errorf("transitioned = %v, want true", data["transitioned"])
		}
		if data["assigned"] != false {
			t.Errorf("assigned = %v, want false", data["assigned"])
		}
		if len(envelope.Errors) == 0 {
			t.Error("errors = empty, want at least one error")
		}
		if data["next_command"] == nil {
			t.Error("next_command = nil, want remediation command")
		}
		if requests != 5 {
			t.Errorf("requests = %d, want 5", requests)
		}
	})

	t.Run("skip assign", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 1: path = %s, want /issue/PROJ-123", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 2:
				if r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 2: path = %s, want /issue/PROJ-123/transitions", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			case 3:
				if r.Method != http.MethodPost {
					t.Errorf("request 3: method = %s, want POST", r.Method)
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s (no /myself or PUT expected)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--skip-assign", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["transitioned"] != true {
			t.Errorf("transitioned = %v, want true", data["transitioned"])
		}
		if _, hasAssigned := data["assigned"]; hasAssigned {
			t.Errorf("assigned field present, want absent when --skip-assign")
		}
		if requests != 3 {
			t.Errorf("requests = %d, want 3", requests)
		}
	})

	t.Run("skip transition", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.URL.Path != "/myself" {
					t.Errorf("request 1: path = %s, want /myself", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case 2:
				if r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 2: path = %s, want /issue/PROJ-123", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 3:
				if r.Method != http.MethodPut || r.URL.Path != "/issue/PROJ-123/assignee" {
					t.Errorf("request 3: got %s %s, want PUT /issue/PROJ-123/assignee", r.Method, r.URL.Path)
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s (no transitions expected)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--skip-transition", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if _, hasTransitioned := data["transitioned"]; hasTransitioned {
			t.Errorf("transitioned field present, want absent when --skip-transition")
		}
		if data["assigned"] != true {
			t.Errorf("assigned = %v, want true", data["assigned"])
		}
		if requests != 3 {
			t.Errorf("requests = %d, want 3", requests)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 3:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			default:
				t.Errorf("unexpected request %d: %s %s (dry-run should only GET)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		dryRunEnabled := true
		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), &dryRunEnabled),
			"PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["command"] != "issue start-work" {
			t.Errorf("command = %v, want issue start-work", data["command"])
		}
		if data["issue_key"] != "PROJ-123" {
			t.Errorf("issue_key = %v, want PROJ-123", data["issue_key"])
		}

		// Verify diff contains expected changes
		diffRaw, ok := data["diff"].([]any)
		if !ok {
			t.Fatalf("diff type = %T, want []any", data["diff"])
		}
		if len(diffRaw) < 2 {
			t.Errorf("diff length = %d, want at least 2 (status + assignee)", len(diffRaw))
		}

		if requests != 3 {
			t.Errorf("requests = %d, want 3 (only GETs)", requests)
		}
	})

	t.Run("dry run skips write check", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/myself":
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123"}`)
			case "/issue/PROJ-123":
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case "/issue/PROJ-123/transitions":
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","to":{"name":"In Progress"}}]}`)
			default:
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		// Writes disabled but dry-run enabled: should succeed
		writesDisabled := false
		dryRunEnabled := true
		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), &writesDisabled, &dryRunEnabled),
			"PROJ-123",
		)

		if !strings.Contains(buf.String(), `"command"`) {
			t.Errorf("output missing dry-run command field: %s", buf.String())
		}
	})

	t.Run("write blocked", func(t *testing.T) {
		t.Parallel()

		writesDisabled := false
		dryRunDisabled := false
		var buf bytes.Buffer
		cmd := issueStartWorkCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), &writesDisabled, &dryRunDisabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"PROJ-123"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("missing issue key", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueStartWorkCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("custom status", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"},"assignee":null}}`)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","to":{"name":"In Review"}}]}`)
			case 3:
				body := testhelpers.DecodeJSONBody(t, r)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "31" {
					t.Errorf("transition id = %v, want 31", transition["id"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueStartWorkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--status", "In Review", "--skip-assign", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["to"] != "In Review" {
			t.Errorf("to = %v, want In Review", data["to"])
		}
	})
}

func TestCreateAndLink(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		var createBody, linkBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				createBody = testhelpers.DecodeJSONBody(t, r)
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456","self":"https://example.atlassian.net/rest/api/3/issue/10001"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issueLink":
				linkBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusCreated)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		// Verify create payload.
		fields, ok := createBody["fields"].(map[string]any)
		if !ok {
			t.Fatal("create body missing fields")
		}
		if fields["summary"] != "New feature" {
			t.Errorf("summary = %v, want New feature", fields["summary"])
		}
		project, ok := fields["project"].(map[string]any)
		if !ok {
			t.Fatal("create body missing project")
		}
		if project["key"] != "PROJ" {
			t.Errorf("project key = %v, want PROJ", project["key"])
		}

		// Verify link payload.
		linkType, ok := linkBody["type"].(map[string]any)
		if !ok {
			t.Fatal("link body missing type")
		}
		if linkType["name"] != "Blocks" {
			t.Errorf("link type = %v, want Blocks", linkType["name"])
		}
		outward, ok := linkBody["outwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing outwardIssue")
		}
		if outward["key"] != "PROJ-456" {
			t.Errorf("outward key = %v, want PROJ-456", outward["key"])
		}
		inward, ok := linkBody["inwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing inwardIssue")
		}
		if inward["key"] != "PROJ-100" {
			t.Errorf("inward key = %v, want PROJ-100", inward["key"])
		}

		// Verify output envelope.
		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-456" {
			t.Errorf("key = %v, want PROJ-456", data["key"])
		}
		if data["linked"] != true {
			t.Errorf("linked = %v, want true", data["linked"])
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456","self":"https://example.atlassian.net/rest/api/3/issue/10001"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issueLink":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errorMessages":["Link type not found"]}`))
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-456" {
			t.Errorf("key = %v, want PROJ-456", data["key"])
		}
		if data["linked"] != false {
			t.Errorf("linked = %v, want false", data["linked"])
		}
		if data["next_command"] == nil {
			t.Error("next_command = nil, want remediation command")
		}
		if len(envelope.Errors) == 0 {
			t.Error("errors = empty, want at least one error")
		}
	})

	t.Run("link direction inward", func(t *testing.T) {
		t.Parallel()

		var linkBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456","self":"https://example.atlassian.net/rest/api/3/issue/10001"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issueLink":
				linkBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusCreated)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
			"--link-direction", "inward",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		// Inward: new issue is inwardIssue, target is outwardIssue.
		inward, ok := linkBody["inwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing inwardIssue")
		}
		if inward["key"] != "PROJ-456" {
			t.Errorf("inward key = %v, want PROJ-456 (new issue)", inward["key"])
		}
		outward, ok := linkBody["outwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing outwardIssue")
		}
		if outward["key"] != "PROJ-100" {
			t.Errorf("outward key = %v, want PROJ-100 (target)", outward["key"])
		}
	})

	t.Run("link direction outward", func(t *testing.T) {
		t.Parallel()

		var linkBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456","self":"https://example.atlassian.net/rest/api/3/issue/10001"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issueLink":
				linkBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusCreated)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
			"--link-direction", "outward",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		// Outward (default): new issue is outwardIssue, target is inwardIssue.
		outward, ok := linkBody["outwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing outwardIssue")
		}
		if outward["key"] != "PROJ-456" {
			t.Errorf("outward key = %v, want PROJ-456 (new issue)", outward["key"])
		}
		inward, ok := linkBody["inwardIssue"].(map[string]any)
		if !ok {
			t.Fatal("link body missing inwardIssue")
		}
		if inward["key"] != "PROJ-100" {
			t.Errorf("inward key = %v, want PROJ-100 (target)", inward["key"])
		}
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			t.Errorf("unexpected request %d: %s %s (dry-run should not make API calls)", requests, r.Method, r.URL.Path)
		}))
		defer server.Close()

		dryRunEnabled := true
		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), &dryRunEnabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["command"] != "issue create-and-link" {
			t.Errorf("command = %v, want issue create-and-link", data["command"])
		}
		if data["issue_key"] != "(new)" {
			t.Errorf("issue_key = %v, want (new)", data["issue_key"])
		}

		diffRaw, ok := data["diff"].([]any)
		if !ok {
			t.Fatalf("diff type = %T, want []any", data["diff"])
		}
		if len(diffRaw) < 4 {
			t.Errorf("diff length = %d, want at least 4 (summary, type, link_type, link_target)", len(diffRaw))
		}

		if requests != 0 {
			t.Errorf("requests = %d, want 0 (dry-run should not call API)", requests)
		}
	})

	t.Run("write blocked", func(t *testing.T) {
		t.Parallel()

		writesDisabled := false
		dryRunDisabled := false
		var buf bytes.Buffer
		cmd := issueCreateAndLinkCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), &writesDisabled, &dryRunDisabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{
			"--project", "PROJ", "--type", "Story", "--summary", "New feature",
			"--link-type", "Blocks", "--link-target", "PROJ-100",
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})
}

func TestCreateAndAssign(t *testing.T) {
	t.Parallel()

	t.Run("happy path self assign", func(t *testing.T) {
		t.Parallel()

		var createBody, assignBody map[string]any
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/myself":
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				createBody = testhelpers.DecodeJSONBody(t, r)
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456"}`)
			case r.Method == http.MethodPut && r.URL.Path == "/issue/PROJ-456/assignee":
				assignBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Story", "--summary", "New feature"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		fields, ok := createBody["fields"].(map[string]any)
		if !ok {
			t.Fatal("create body missing fields")
		}
		if fields["summary"] != "New feature" {
			t.Errorf("summary = %v, want New feature", fields["summary"])
		}
		if assignBody["accountId"] != "abc123" {
			t.Errorf("accountId = %v, want abc123", assignBody["accountId"])
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-456" {
			t.Errorf("key = %v, want PROJ-456", data["key"])
		}
		if data["assigned"] != true {
			t.Errorf("assigned = %v, want true", data["assigned"])
		}
		if data["assignee"] != "abc123" {
			t.Errorf("assignee = %v, want abc123", data["assignee"])
		}
		if requests != 3 {
			t.Errorf("requests = %d, want 3", requests)
		}
	})

	t.Run("explicit assignee", func(t *testing.T) {
		t.Parallel()

		var createBody map[string]any
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				createBody = testhelpers.DecodeJSONBody(t, r)
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456"}`)
			case r.Method == http.MethodPut && r.URL.Path == "/issue/PROJ-456/assignee":
				body := testhelpers.DecodeJSONBody(t, r)
				if body["accountId"] != "def456" {
					t.Errorf("accountId = %v, want def456", body["accountId"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Bug", "--summary", "Fix", "--assignee", "def456"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}
		fields, ok := createBody["fields"].(map[string]any)
		if !ok {
			t.Fatal("create body missing fields")
		}
		if _, hasAssignee := fields["assignee"]; hasAssignee {
			t.Errorf("create assignee field present, want assignment only via PUT")
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["assignee"] != "def456" {
			t.Errorf("assignee = %v, want def456", data["assignee"])
		}
		if requests != 2 {
			t.Errorf("requests = %d, want 2 (no /myself call)", requests)
		}
	})

	t.Run("skip assign", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456"}`)
			default:
				t.Errorf("unexpected request %d: %s %s (no assignment expected)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Task", "--summary", "Chore", "--skip-assign"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["assigned"] != false {
			t.Errorf("assigned = %v, want false", data["assigned"])
		}
		if _, hasAssignee := data["assignee"]; hasAssignee {
			t.Errorf("assignee field present, want absent when --skip-assign")
		}
		if requests != 1 {
			t.Errorf("requests = %d, want 1", requests)
		}
	})

	t.Run("payload json strips assignee", func(t *testing.T) {
		t.Parallel()

		var createBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/myself":
				testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Test User"}`)
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				createBody = testhelpers.DecodeJSONBody(t, r)
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456"}`)
			case r.Method == http.MethodPut && r.URL.Path == "/issue/PROJ-456/assignee":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		// Payload includes fields.assignee, which should be stripped by
		// buildCreatePayload so assignment is managed by the command's own step.
		cmd.SetArgs([]string{
			"--payload-json", `{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Embedded","assignee":{"accountId":"embedded-id"}}}`,
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		fields, ok := createBody["fields"].(map[string]any)
		if !ok {
			t.Fatal("create body missing fields")
		}
		if _, hasAssignee := fields["assignee"]; hasAssignee {
			t.Errorf("create body contains assignee, want stripped from payload-json")
		}
	})

	t.Run("dry run zero mutations", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			t.Errorf("unexpected request %d: %s %s (dry-run should not make API calls)", requests, r.Method, r.URL.Path)
		}))
		defer server.Close()

		dryRunEnabled := true
		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), &dryRunEnabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Story", "--summary", "New"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["command"] != "issue create-and-assign" {
			t.Errorf("command = %v, want issue create-and-assign", data["command"])
		}
		if data["issue_key"] != "(new)" {
			t.Errorf("issue_key = %v, want (new)", data["issue_key"])
		}
		after, ok := data["after"].(map[string]any)
		if !ok {
			t.Fatalf("after type = %T, want map[string]any", data["after"])
		}
		if after["assignee"] != "(resolved from /myself)" {
			t.Errorf("assignee = %v, want resolved placeholder", after["assignee"])
		}
		if requests != 0 {
			t.Errorf("requests = %d, want 0", requests)
		}
	})

	t.Run("write blocked", func(t *testing.T) {
		t.Parallel()

		writesDisabled := false
		dryRunDisabled := false
		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), &writesDisabled, &dryRunDisabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Story", "--summary", "New feature"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("partial failure assign", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/issue":
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001","key":"PROJ-456"}`)
			case r.Method == http.MethodPut && r.URL.Path == "/issue/PROJ-456/assignee":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"errorMessages":["Permission denied"]}`))
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := issueCreateAndAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun())
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--project", "PROJ", "--type", "Story", "--summary", "New feature", "--assignee", "def456"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("cmd.Execute() error = %v", err)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-456" {
			t.Errorf("key = %v, want PROJ-456", data["key"])
		}
		if data["assigned"] != false {
			t.Errorf("assigned = %v, want false", data["assigned"])
		}
		wantNext := "jira-agent issue assign PROJ-456 def456"
		if data["next_command"] != wantNext {
			t.Errorf("next_command = %v, want %s", data["next_command"], wantNext)
		}
		if len(envelope.Errors) == 0 {
			t.Error("errors = empty, want at least one error")
		}
	})
}

func TestMoveToSprint(t *testing.T) {
	t.Parallel()

	t.Run("move only", func(t *testing.T) {
		t.Parallel()

		var requests int
		var agileBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/sprint/42/issue":
				agileBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--sprint-id", "42", "PROJ-123",
		)

		// Verify Agile API payload.
		issues, ok := agileBody["issues"].([]any)
		if !ok {
			t.Fatalf("agile body issues type = %T, want []any", agileBody["issues"])
		}
		if len(issues) != 1 || issues[0] != "PROJ-123" {
			t.Errorf("issues = %v, want [PROJ-123]", issues)
		}

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["key"] != "PROJ-123" {
			t.Errorf("key = %v, want PROJ-123", data["key"])
		}
		if data["moved_to_sprint"] != float64(42) {
			t.Errorf("moved_to_sprint = %v, want 42", data["moved_to_sprint"])
		}
		if requests != 1 {
			t.Errorf("requests = %d, want 1 (POST sprint only)", requests)
		}
	})

	t.Run("move with transition", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.Method != http.MethodPost || r.URL.Path != "/sprint/42/issue" {
					t.Errorf("request 1: got %s %s, want POST /sprint/42/issue", r.Method, r.URL.Path)
				}
				w.WriteHeader(http.StatusNoContent)
			case 2:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 2: got %s %s, want GET /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","name":"In Progress","to":{"name":"In Progress"}}]}`)
			case 3:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/transitions" {
					t.Errorf("request 3: got %s %s, want POST /issue/PROJ-123/transitions", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "21" {
					t.Errorf("transition id = %v, want 21", transition["id"])
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--sprint-id", "42", "--status", "In Progress", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["transitioned"] != true {
			t.Errorf("transitioned = %v, want true", data["transitioned"])
		}
		if data["to"] != "In Progress" {
			t.Errorf("to = %v, want In Progress", data["to"])
		}
		if requests != 3 {
			t.Errorf("requests = %d, want 3", requests)
		}
	})

	t.Run("move succeeds transition fails partial", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				w.WriteHeader(http.StatusNoContent)
			case 2:
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"21","name":"In Progress","to":{"name":"In Progress"}}]}`)
			case 3:
				// Transition fails.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errorMessages":["Transition not allowed"]}`))
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--sprint-id", "42", "--status", "In Progress", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["moved_to_sprint"] != float64(42) {
			t.Errorf("moved_to_sprint = %v, want 42", data["moved_to_sprint"])
		}
		if data["transitioned"] != false {
			t.Errorf("transitioned = %v, want false", data["transitioned"])
		}
		if len(envelope.Errors) == 0 {
			t.Error("errors = empty, want at least one error for failed transition")
		}
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/PROJ-123" {
					t.Errorf("request 1: got %s %s, want GET /issue/PROJ-123", r.Method, r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"status":{"name":"Open"}}}`)
			default:
				t.Errorf("unexpected request %d: %s %s (dry-run should only GET)", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		dryRunEnabled := true
		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), &dryRunEnabled),
			"--sprint-id", "42", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["command"] != "issue move-to-sprint" {
			t.Errorf("command = %v, want issue move-to-sprint", data["command"])
		}
		if data["issue_key"] != "PROJ-123" {
			t.Errorf("issue_key = %v, want PROJ-123", data["issue_key"])
		}
		diffRaw, ok := data["diff"].([]any)
		if !ok {
			t.Fatalf("diff type = %T, want []any", data["diff"])
		}
		if len(diffRaw) < 1 {
			t.Errorf("diff length = %d, want at least 1 (sprint)", len(diffRaw))
		}
		if requests != 1 {
			t.Errorf("requests = %d, want 1 (only GET issue)", requests)
		}
	})

	t.Run("write blocked", func(t *testing.T) {
		t.Parallel()

		writesDisabled := false
		dryRunDisabled := false
		var buf bytes.Buffer
		cmd := issueMoveToSprintCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), &writesDisabled, &dryRunDisabled)
		prepareCommandForTest(cmd)
		cmd.SetArgs([]string{"--sprint-id", "42", "PROJ-123"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() = nil, want validation error")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("error type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("move with comment", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				w.WriteHeader(http.StatusNoContent)
			case 2:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/PROJ-123/comment" {
					t.Errorf("request 2: got %s %s, want POST /issue/PROJ-123/comment", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				if body["body"] == nil {
					t.Error("comment body = nil, want ADF content")
				}
				testhelpers.WriteJSONResponse(t, w, `{"id":"10001"}`)
			default:
				t.Errorf("unexpected request %d: %s %s", requests, r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--sprint-id", "42", "--comment", "Moved to sprint 42", "PROJ-123",
		)

		var envelope output.Envelope
		if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type = %T, want map[string]any", envelope.Data)
		}
		if data["commented"] != true {
			t.Errorf("commented = %v, want true", data["commented"])
		}
		if requests != 2 {
			t.Errorf("requests = %d, want 2", requests)
		}
	})

	t.Run("move with rank before", func(t *testing.T) {
		t.Parallel()

		var agileBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/sprint/42/issue":
				agileBody = testhelpers.DecodeJSONBody(t, r)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMoveToSprintCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites(), testDryRun()),
			"--sprint-id", "42", "--rank-before", "PROJ-100", "PROJ-123",
		)

		if agileBody["rankBeforeIssue"] != "PROJ-100" {
			t.Errorf("rankBeforeIssue = %v, want PROJ-100", agileBody["rankBeforeIssue"])
		}
	})
}
