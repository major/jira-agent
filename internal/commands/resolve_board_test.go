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

func TestResolveBoardSingleMatch(t *testing.T) {
	t.Parallel()

	// Mock response with extra fields that should be stripped
	mockResponse := `{
		"values": [
			{
				"id": 42,
				"name": "My Scrum Board",
				"type": "scrum",
				"self": "https://example.atlassian.net/rest/agile/1.0/board/42",
				"location": {
					"projectKey": "PROJ"
				}
			}
		],
		"total": 1,
		"maxResults": 10
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := boardResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"My Scrum Board"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify data is array of resolvedBoard
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1", len(data))
	}

	// Verify board object has only 3 fields
	board, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("board type: got %T, want map[string]any", data[0])
	}

	expectedFields := map[string]bool{
		"id":   false,
		"name": false,
		"type": false,
	}

	for field := range board {
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
	if !strings.Contains(envelope.Metadata.UsageHint, "resolve sprint") {
		t.Errorf("metadata.usage_hint: got %q, want to contain 'resolve sprint'", envelope.Metadata.UsageHint)
	}
	if !strings.Contains(envelope.Metadata.UsageHint, "--board-id") {
		t.Errorf("metadata.usage_hint: got %q, want to contain '--board-id'", envelope.Metadata.UsageHint)
	}

	// Verify metadata counts
	if envelope.Metadata.Total != 1 {
		t.Errorf("metadata.total: got %d, want 1", envelope.Metadata.Total)
	}
	if envelope.Metadata.Returned != 1 {
		t.Errorf("metadata.returned: got %d, want 1", envelope.Metadata.Returned)
	}
}

func TestResolveBoardMultipleMatches(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [
			{
				"id": 42,
				"name": "My Scrum Board",
				"type": "scrum"
			},
			{
				"id": 43,
				"name": "My Kanban Board",
				"type": "kanban"
			},
			{
				"id": 44,
				"name": "My Simple Board",
				"type": "simple"
			}
		],
		"total": 3,
		"maxResults": 10
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := boardResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"My"})

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

func TestResolveBoardNotFound(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [],
		"total": 0,
		"maxResults": 10
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/board", mockResponse)
	defer server.Close()

	cmd := boardResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"nonexistent"})

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

	// Verify available actions
	if len(notFoundErr.AvailableActions()) == 0 {
		t.Error("available_actions: expected non-empty, got empty")
	}
	if !strings.Contains(notFoundErr.AvailableActions()[0], "board list") {
		t.Errorf("available_actions: got %q, want to contain 'board list'", notFoundErr.AvailableActions()[0])
	}
}

func TestResolveBoardEmptyQuery(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, "GET", "/board", `{"values": [], "total": 0}`)
	defer server.Close()

	cmd := boardResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{})

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
}
