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

func TestResolveUserSingleMatch(t *testing.T) {
	t.Parallel()

	// Mock response with extra fields that should be stripped
	mockResponse := `[
		{
			"accountId": "5b10ac8d82e05b22cc7d4ef5",
			"displayName": "John Doe",
			"emailAddress": "john@example.com",
			"active": true,
			"avatarUrls": {
				"48x48": "https://example.com/avatar.png"
			},
			"self": "https://example.atlassian.net/rest/api/3/user?accountId=5b10ac8d82e05b22cc7d4ef5",
			"timeZone": "America/New_York",
			"accountType": "atlassian"
		}
	]`

	server := testhelpers.NewJSONServer(t, "GET", "/users/search", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := userResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())
	
	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"john@example.com"})
	
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v (result: %q)", err, buf.String())
	}

	// Verify data is array of resolvedUser
	data, ok := envelope.Data.([]any)
	if !ok {
		t.Fatalf("data type: got %T, want []any", envelope.Data)
	}

	if len(data) != 1 {
		t.Errorf("data length: got %d, want 1", len(data))
	}

	// Verify user object has only 4 fields
	user, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("user type: got %T, want map[string]any", data[0])
	}

	expectedFields := map[string]bool{
		"account_id":    false,
		"display_name":  false,
		"email_address": false,
		"active":        false,
	}

	for field := range user {
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
	if !strings.Contains(envelope.Metadata.UsageHint, "issue assign") {
		t.Errorf("metadata.usage_hint: got %q, want to contain 'issue assign'", envelope.Metadata.UsageHint)
	}

	// Verify metadata counts
}

func TestResolveUserMultipleMatches(t *testing.T) {
	t.Parallel()

	mockResponse := `[
		{
			"accountId": "5b10ac8d82e05b22cc7d4ef5",
			"displayName": "John Doe",
			"emailAddress": "john@example.com",
			"active": true
		},
		{
			"accountId": "5b10ac8d82e05b22cc7d4ef6",
			"displayName": "John Smith",
			"emailAddress": "jsmith@example.com",
			"active": true
		},
		{
			"accountId": "5b10ac8d82e05b22cc7d4ef7",
			"displayName": "John Johnson",
			"emailAddress": "jjohnson@example.com",
			"active": false
		}
	]`

	server := testhelpers.NewJSONServer(t, "GET", "/users/search", mockResponse)
	defer server.Close()

	var buf bytes.Buffer
	cmd := userResolveCommand(testCommandClient(server.URL), &buf, testCommandFormat())
	
	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"John"})
	
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

}

func TestResolveUserNotFound(t *testing.T) {
	t.Parallel()

	mockResponse := `[]`

	server := testhelpers.NewJSONServer(t, "GET", "/users/search", mockResponse)
	defer server.Close()

	cmd := userResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

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
}

func TestResolveUserEmptyQuery(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, "GET", "/users/search", `[]`)
	defer server.Close()

	cmd := userResolveCommand(testCommandClient(server.URL), &bytes.Buffer{}, testCommandFormat())

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
