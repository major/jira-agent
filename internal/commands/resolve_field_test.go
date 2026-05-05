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

func TestResolveFieldMatch(t *testing.T) {
	t.Parallel()

	// Mock response with extra fields that should be stripped
	mockResponse := `{
		"values": [
			{
				"id": "summary",
				"name": "Summary",
				"custom": false,
				"schema": {
					"type": "string"
				},
				"clauseNames": ["summary"]
			},
			{
				"id": "customfield_10001",
				"name": "Story Points",
				"custom": true,
				"schema": {
					"type": "number"
				},
				"clauseNames": ["cf[10001]"]
			}
		],
		"total": 2
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/field/search", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := fieldResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"story"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify data is array of resolvedField
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 2 {
		t.Errorf("data length: got %d, want 2", len(data))
	}

	// Verify first field (system field)
	field1, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("field1 type: got %T, want map[string]any", data[0])
	}

	if id, ok := field1["id"].(string); !ok || id != "summary" {
		t.Errorf("field1.id: got %v, want 'summary'", field1["id"])
	}
	if name, ok := field1["name"].(string); !ok || name != "Summary" {
		t.Errorf("field1.name: got %v, want 'Summary'", field1["name"])
	}
	if custom, ok := field1["custom"].(bool); !ok || custom != false {
		t.Errorf("field1.custom: got %v, want false", field1["custom"])
	}

	// Verify second field (custom field)
	field2, ok := data[1].(map[string]any)
	if !ok {
		t.Fatalf("field2 type: got %T, want map[string]any", data[1])
	}

	if id, ok := field2["id"].(string); !ok || id != "customfield_10001" {
		t.Errorf("field2.id: got %v, want 'customfield_10001'", field2["id"])
	}
	if name, ok := field2["name"].(string); !ok || name != "Story Points" {
		t.Errorf("field2.name: got %v, want 'Story Points'", field2["name"])
	}
	if custom, ok := field2["custom"].(bool); !ok || custom != true {
		t.Errorf("field2.custom: got %v, want true", field2["custom"])
	}

	// Verify no extra fields leaked (schema, clauseNames should not be present)
	expectedFields := map[string]bool{
		"id":     false,
		"name":   false,
		"custom": false,
	}

	for field := range field1 {
		if _, expected := expectedFields[field]; !expected {
			t.Errorf("unexpected field in field1 output: %q", field)
		}
	}

	for field := range field2 {
		if _, expected := expectedFields[field]; !expected {
			t.Errorf("unexpected field in field2 output: %q", field)
		}
	}

	// Verify metadata contains usage hint
	if envelope.Metadata.UsageHint == "" {
		t.Error("metadata.usage_hint: expected non-empty, got empty")
	}
	if !strings.Contains(envelope.Metadata.UsageHint, "issue get") {
		t.Errorf("metadata.usage_hint: got %q, want to contain 'issue get'", envelope.Metadata.UsageHint)
	}
	if !strings.Contains(envelope.Metadata.UsageHint, "--fields") {
		t.Errorf("metadata.usage_hint: got %q, want to contain '--fields'", envelope.Metadata.UsageHint)
	}

	// Verify metadata counts
}

func TestResolveFieldSingleMatch(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [
			{
				"id": "description",
				"name": "Description",
				"custom": false
			}
		],
		"total": 1
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/field/search", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := fieldResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"description"})

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

}

func TestResolveFieldNotFound(t *testing.T) {
	t.Parallel()

	mockResponse := `{
		"values": [],
		"total": 0
	}`

	server := testhelpers.NewJSONServer(t, "GET", "/field/search", mockResponse)
	defer server.Close()

	cmd := fieldResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

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
	hasFieldList := false
	for _, action := range notFoundErr.AvailableActions() {
		if strings.Contains(action, "field list") {
			hasFieldList = true
			break
		}
	}
	if !hasFieldList {
		t.Errorf("available_actions: got %v, want to contain 'field list'", notFoundErr.AvailableActions())
	}
}

func TestResolveFieldEmptyQuery(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, "GET", "/field/search", `{"values": [], "total": 0}`)
	defer server.Close()

	cmd := fieldResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

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
