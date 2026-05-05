package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestResolveTransitionMatchByName(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"transitions": [
			{
				"id": "21",
				"name": "Start Progress",
				"to": {
					"id": "3",
					"name": "In Progress",
					"statusCategory": {"key": "indigo"}
				}
			},
			{
				"id": "31",
				"name": "Resolve",
				"to": {
					"id": "5",
					"name": "Done",
					"statusCategory": {"key": "green"}
				}
			}
		]
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := transitionResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--issue", "PROJ-123", "Start Progress"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify data is array of resolvedTransition
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1", len(data))
	}

	// Verify transition object has only 2 fields
	transition, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("transition type: got %T, want map[string]any", data[0])
	}

	expectedFields := map[string]bool{
		"id":   false,
		"name": false,
	}

	for field := range transition {
		if _, expected := expectedFields[field]; !expected {
			t.Errorf("unexpected field in output: %q", field)
		}
		expectedFields[field] = true
	}

	for field, found := range expectedFields {
		if !found {
			t.Errorf("missing expected field: %q", field)
		}
	}

	// Verify metadata
	if envelope.Metadata.UsageHint == "" {
		t.Error("metadata.usage_hint: expected non-empty, got empty")
	}
}

func TestResolveTransitionMatchByToName(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"transitions": [
			{
				"id": "21",
				"name": "Start Progress",
				"to": {
					"id": "3",
					"name": "In Progress",
					"statusCategory": {"key": "indigo"}
				}
			},
			{
				"id": "31",
				"name": "Resolve",
				"to": {
					"id": "5",
					"name": "Done",
					"statusCategory": {"key": "green"}
				}
			}
		]
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := transitionResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--issue", "PROJ-123", "Done"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1", len(data))
	}

	transition, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("transition type: got %T, want map[string]any", data[0])
	}

	if id, ok := transition["id"].(string); !ok || id != "31" {
		t.Errorf("transition id: got %v, want \"31\"", transition["id"])
	}

	if name, ok := transition["name"].(string); !ok || name != "Resolve" {
		t.Errorf("transition name: got %v, want \"Resolve\"", transition["name"])
	}
}

func TestResolveTransitionMultipleMatches(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"transitions": [
			{
				"id": "11",
				"name": "In Progress",
				"to": {
					"name": "In Progress"
				}
			},
			{
				"id": "21",
				"name": "In Review",
				"to": {
					"name": "In Review"
				}
			},
			{
				"id": "31",
				"name": "Resolve",
				"to": {
					"name": "Done"
				}
			}
		]
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := transitionResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--issue", "PROJ-123", "In"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 2 {
		t.Errorf("data length: got %d, want 2", len(data))
	}

	for i, item := range data {
		transition, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("transition[%d] type: got %T, want map[string]any", i, item)
		}
		if id, ok := transition["id"].(string); !ok || id == "" {
			t.Errorf("transition[%d] id: got %v, want non-empty string", i, transition["id"])
		}
		if name, ok := transition["name"].(string); !ok || name == "" {
			t.Errorf("transition[%d] name: got %v, want non-empty string", i, transition["name"])
		}
	}

}

func TestResolveTransitionNotFound(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"transitions": [
			{
				"id": "21",
				"name": "Start Progress",
				"to": {
					"id": "3",
					"name": "In Progress"
				}
			},
			{
				"id": "31",
				"name": "Resolve",
				"to": {
					"id": "5",
					"name": "Done"
				}
			}
		]
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", mockResponse)
	defer server.Close()

	cmd := transitionResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--issue", "PROJ-123", "Blocked"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify it's a NotFoundError
	var notFoundErr *apperr.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("error type: got %T, want NotFoundError", err)
	}

	// Verify error code
	code := apperr.ErrorCode(err)
	if code != "NOT_FOUND" {
		t.Errorf("error code: got %q, want NOT_FOUND", code)
	}

	// Verify exit code
	exitCode := apperr.ExitCodeFor(err)
	if exitCode != 2 {
		t.Errorf("exit code: got %d, want 2", exitCode)
	}

	// Verify available actions include transition names
	if len(notFoundErr.AvailableActions()) != 2 {
		t.Errorf("available_actions length: got %d, want 2", len(notFoundErr.AvailableActions()))
	}

	actions := notFoundErr.AvailableActions()
	if len(actions) > 0 && actions[0] != "Start Progress" {
		t.Errorf("first available action: got %q, want \"Start Progress\"", actions[0])
	}
	if len(actions) > 1 && actions[1] != "Resolve" {
		t.Errorf("second available action: got %q, want \"Resolve\"", actions[1])
	}
}

func TestResolveTransitionMissingIssue(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", `{"transitions":[]}`)
	defer server.Close()

	cmd := transitionResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"Done"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify it's a ValidationError
	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("error type: got %T, want ValidationError", err)
	}

	// Verify error code
	code := apperr.ErrorCode(err)
	if code != "VALIDATION_ERROR" {
		t.Errorf("error code: got %q, want VALIDATION_ERROR", code)
	}

	// Verify exit code
	exitCode := apperr.ExitCodeFor(err)
	if exitCode != 5 {
		t.Errorf("exit code: got %d, want 5", exitCode)
	}

	// Verify next command contains "issue get"
	nextCmd := validationErr.NextCommand()
	if nextCmd == "" {
		t.Error("next_command: expected non-empty, got empty")
	}
	if !strings.Contains(nextCmd, "issue get") {
		t.Errorf("next_command: got %q, want to contain 'issue get'", nextCmd)
	}
}

func TestResolveTransitionCaseInsensitive(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"transitions": [
			{
				"id": "21",
				"name": "Start Progress",
				"to": {
					"id": "3",
					"name": "In Progress"
				}
			},
			{
				"id": "31",
				"name": "Resolve",
				"to": {
					"id": "5",
					"name": "Done"
				}
			}
		]
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/issue/PROJ-123/transitions", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := transitionResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--issue", "PROJ-123", "done"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1 (case-insensitive match should work)", len(data))
	}
}

func TestResolveTransitionIssueNotFound(t *testing.T) {
	t.Parallel()

	// Create a custom server that returns 404 for the transitions endpoint
	server := testhelpers.NewServer(t, "GET", "/issue/PROJ-999/transitions", func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"errorMessages":["Issue does not exist"],"errors":{}}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer server.Close()

	cmd := transitionResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--issue", "PROJ-999", "Done"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify it's a NotFoundError
	var notFoundErr *apperr.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("error type: got %T, want NotFoundError", err)
	}

	// Verify error message contains the issue key
	if !strings.Contains(err.Error(), "PROJ-999") {
		t.Errorf("error message: got %q, want to contain 'PROJ-999'", err.Error())
	}

	// Verify error code
	code := apperr.ErrorCode(err)
	if code != "NOT_FOUND" {
		t.Errorf("error code: got %q, want NOT_FOUND", code)
	}

	// Verify exit code
	exitCode := apperr.ExitCodeFor(err)
	if exitCode != 2 {
		t.Errorf("exit code: got %d, want 2", exitCode)
	}
}
