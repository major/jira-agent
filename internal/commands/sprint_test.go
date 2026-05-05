package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
		if notFound.NextCommand() == "" {
			t.Error("expected non-empty next_command with remediation hint")
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
