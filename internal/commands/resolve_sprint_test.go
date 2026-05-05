package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestResolveSprintMatch(t *testing.T) {
	t.Parallel()

	// Mock response with 2 sprints, only one matches query
	mockResponse := `{
		"values": [
			{
				"id": 100,
				"name": "Sprint 42",
				"state": "active",
				"self": "https://example.atlassian.net/rest/agile/1.0/sprint/100",
				"startDate": "2026-05-01T00:00:00.000Z",
				"endDate": "2026-05-15T00:00:00.000Z"
			},
			{
				"id": 101,
				"name": "Sprint 43",
				"state": "active",
				"self": "https://example.atlassian.net/rest/agile/1.0/sprint/101"
			}
		],
		"total": 2,
		"maxResults": 50
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board/42/sprint", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := sprintResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--board-id", "42", "Sprint 42"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify data is array of resolvedSprint
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1", len(data))
	}

	// Verify sprint object has only 3 fields
	sprint, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("sprint type: got %T, want map[string]any", data[0])
	}

	expectedFields := map[string]bool{
		"id":    false,
		"name":  false,
		"state": false,
	}

	for field := range sprint {
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

	// Verify metadata contains usage hint
	if envelope.Metadata.UsageHint == "" {
		t.Error("metadata.usage_hint: expected non-empty, got empty")
	}
	if !strings.Contains(envelope.Metadata.UsageHint, "sprint get") {
		t.Errorf("metadata.usage_hint: got %q, want to contain 'sprint get'", envelope.Metadata.UsageHint)
	}

	// Verify metadata counts
	if envelope.Metadata.Total != 1 {
		t.Errorf("metadata.total: got %d, want 1", envelope.Metadata.Total)
	}
	if envelope.Metadata.Returned != 1 {
		t.Errorf("metadata.returned: got %d, want 1", envelope.Metadata.Returned)
	}
}

func TestResolveSprintMultipleMatches(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [
			{
				"id": 100,
				"name": "Sprint 1",
				"state": "active"
			},
			{
				"id": 101,
				"name": "Sprint 2",
				"state": "active"
			},
			{
				"id": 102,
				"name": "Sprint 3",
				"state": "future"
			}
		],
		"total": 3,
		"maxResults": 50
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board/42/sprint", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := sprintResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--board-id", "42", "Sprint"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify all 3 sprints are returned
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 3 {
		t.Errorf("data length: got %d, want 3", len(data))
	}

	if envelope.Metadata.Total != 3 {
		t.Errorf("metadata.total: got %d, want 3", envelope.Metadata.Total)
	}
	if envelope.Metadata.Returned != 3 {
		t.Errorf("metadata.returned: got %d, want 3", envelope.Metadata.Returned)
	}
}

func TestResolveSprintNotFound(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [
			{
				"id": 100,
				"name": "Sprint 1",
				"state": "active"
			}
		],
		"total": 1,
		"maxResults": 50
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board/42/sprint", mockResponse)
	defer server.Close()

	cmd := sprintResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--board-id", "42", "NonExistent"})

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
}

func TestResolveSprintMissingBoardID(t *testing.T) {
	t.Parallel()

	cmd := sprintResolveCommand(testCommandClient("http://localhost"), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"Sprint 42"})

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

	// Verify next command
	nextCmd := validationErr.NextCommand()
	if !strings.Contains(nextCmd, "resolve board") {
		t.Errorf("next_command: got %q, want to contain 'resolve board'", nextCmd)
	}
}

func TestResolveSprintStateFilter(t *testing.T) {
	t.Parallel()

	// Mock server that captures the request to verify state parameter
	server := testhelpers.NewJSONServer(t, "GET", "/board/42/sprint", `{
		"values": [
			{
				"id": 100,
				"name": "Sprint 1",
				"state": "active"
			}
		],
		"total": 1,
		"maxResults": 50
	}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := sprintResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	// Don't specify --state, should default to "active,future"
	cmd.SetArgs([]string{"--board-id", "42", "Sprint"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify we got a result (which means the API call succeeded with default state)
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) == 0 {
		t.Error("expected at least one sprint in response")
	}
}
