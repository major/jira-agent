package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func TestTypedErrors_ExitCodesAndCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantExit int
		wantCode string
	}{
		{name: "jira", err: NewJiraError("general", nil), wantExit: 1, wantCode: "UNKNOWN"},
		{name: "auth", err: NewAuthError("auth", nil), wantExit: 3, wantCode: "AUTH_FAILED"},
		{name: "not found", err: NewNotFoundError("missing", nil), wantExit: 2, wantCode: "NOT_FOUND"},
		{name: "api", err: NewAPIError("api", 500, "body", nil), wantExit: 4, wantCode: "API_ERROR"},
		{name: "validation", err: NewValidationError("bad", nil), wantExit: 5, wantCode: "VALIDATION_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ExitCodeFor(tt.err); got != tt.wantExit {
				t.Errorf("ExitCodeFor() = %d, want %d", got, tt.wantExit)
			}
			if got := ErrorCode(tt.err); got != tt.wantCode {
				t.Errorf("ErrorCode() = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestJiraError_DetailsAndUnwrap(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("root cause")
	err := NewAuthError("auth failed", cause, WithDetails("check credentials"))

	if got := err.Error(); got != "auth failed" {
		t.Errorf("Error() = %q, want %q", got, "auth failed")
	}
	if got := err.Details(); got != "check credentials" {
		t.Errorf("Details() = %q, want %q", got, "check credentials")
	}
	if !stderrors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false, want true")
	}
}

func TestExitCodeFor_WrappedAndNil(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("wrap: %w", NewValidationError("bad input", nil))
	if got := ExitCodeFor(wrapped); got != 5 {
		t.Errorf("ExitCodeFor(wrapped) = %d, want %d", got, 5)
	}
	if got := ExitCodeFor(nil); got != 0 {
		t.Errorf("ExitCodeFor(nil) = %d, want %d", got, 0)
	}
	if got := ExitCodeFor(stderrors.New("plain")); got != 1 {
		t.Errorf("ExitCodeFor(plain) = %d, want %d", got, 1)
	}
}

func TestAPIError_Fields(t *testing.T) {
	t.Parallel()

	err := NewAPIError("api failed", 503, `{"error":"down"}`, nil, WithDetails("retry later"))
	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, 503)
	}
	if err.Body != `{"error":"down"}` {
		t.Errorf("Body = %q, want %q", err.Body, `{"error":"down"}`)
	}
	if got := err.Details(); got != "retry later" {
		t.Errorf("Details() = %q, want %q", got, "retry later")
	}
}

func TestErrorRemediation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		err                  error
		wantNextCommand      string
		wantAvailableActions []string
	}{
		{
			name:                 "write-blocked validation error provides write-enable command",
			err:                  NewValidationError("write access is disabled", nil, WithWriteBlocked()),
			wantNextCommand:      "export JIRA_ALLOW_WRITES=true",
			wantAvailableActions: nil,
		},
		{
			name:                 "validation error without write block returns empty next command",
			err:                  NewValidationError("issue key is required", nil),
			wantNextCommand:      "",
			wantAvailableActions: nil,
		},
		{
			name:                 "not found error provides search command when resource key set",
			err:                  NewNotFoundError("issue PROJ-123 not found", nil, WithResourceKey("PROJ-123")),
			wantNextCommand:      `jira-agent issue search --jql "key = PROJ-123"`,
			wantAvailableActions: nil,
		},
		{
			name:                 "not found error returns empty next command when no resource key",
			err:                  NewNotFoundError("resource not found", nil),
			wantNextCommand:      "",
			wantAvailableActions: nil,
		},
		{
			name:                 "not found error uses custom next command",
			err:                  NewNotFoundError("no active sprint found", nil, WithNextCommand("jira-agent sprint list --board-id 42 --state active")),
			wantNextCommand:      "jira-agent sprint list --board-id 42 --state active",
			wantAvailableActions: nil,
		},
		{
			name:                 "api error provides available actions",
			err:                  NewAPIError("transition failed", 400, "", nil, WithAvailableActions([]string{"In Progress", "Won't Do"})),
			wantNextCommand:      "",
			wantAvailableActions: []string{"In Progress", "Won't Do"},
		},
		{
			name:                 "api error returns nil actions when not set",
			err:                  NewAPIError("api failed", 500, "", nil),
			wantNextCommand:      "",
			wantAvailableActions: nil,
		},
		{
			name:                 "base jira error returns empty remediation",
			err:                  NewJiraError("general error", nil),
			wantNextCommand:      "",
			wantAvailableActions: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// All JiraError types expose NextCommand and AvailableActions.
			// Use errors.As to support wrapped errors.
			type remediator interface {
				NextCommand() string
				AvailableActions() []string
			}

			var r remediator
			if !stderrors.As(tt.err, &r) {
				t.Fatalf("error does not implement remediator interface")
			}

			if got := r.NextCommand(); got != tt.wantNextCommand {
				t.Errorf("NextCommand() = %q, want %q", got, tt.wantNextCommand)
			}

			gotActions := r.AvailableActions()
			if len(gotActions) != len(tt.wantAvailableActions) {
				t.Fatalf("AvailableActions() len = %d, want %d", len(gotActions), len(tt.wantAvailableActions))
			}
			for i, want := range tt.wantAvailableActions {
				if gotActions[i] != want {
					t.Errorf("AvailableActions()[%d] = %q, want %q", i, gotActions[i], want)
				}
			}
		})
	}
}
