// Package errors defines the error hierarchy and exit code mapping for jira-agent.
//
// Each error type carries a machine-readable code for JSON responses and a
// process exit code for CLI consumers. The hierarchy follows the exit codes
// defined in COMMAND_STRUCTURE.md:
//
//	0 = success
//	1 = general error
//	2 = not found
//	3 = authentication error
//	4 = API error
//	5 = validation error
package errors

import (
	"errors"
	"fmt"
)

// ErrorOption configures optional fields on JiraError during construction.
type ErrorOption func(*JiraError)

// WithDetails sets a human-readable hint or remediation message on the error.
func WithDetails(d string) ErrorOption {
	return func(e *JiraError) { e.details = d }
}

// WithNextCommand sets a concrete follow-up command for agent remediation.
func WithNextCommand(cmd string) ErrorOption {
	return func(e *JiraError) { e.nextCommand = cmd }
}

// WithWriteBlocked marks a validation error as a write-access failure so
// NextCommand can suggest the correct remediation.
func WithWriteBlocked() ErrorOption {
	return func(e *JiraError) { e.writeBlocked = true }
}

// WithResourceKey sets the resource identifier (e.g. issue key) on the error
// so remediation can suggest a follow-up command referencing it.
func WithResourceKey(key string) ErrorOption {
	return func(e *JiraError) { e.resourceKey = key }
}

// WithAvailableActions sets the list of available actions on the error
// so agents can discover valid alternatives after a failed operation.
func WithAvailableActions(actions []string) ErrorOption {
	return func(e *JiraError) { e.availableActions = actions }
}

// JiraError is the base error type for all jira-agent errors.
// It wraps an underlying error to preserve the error chain.
type JiraError struct {
	Message          string
	Cause            error
	details          string
	nextCommand      string
	resourceKey      string
	availableActions []string
	writeBlocked     bool
}

// Details returns the human-readable hint or remediation message, if any.
func (e *JiraError) Details() string {
	return e.details
}

// ResourceKey returns the resource identifier associated with this error, if any.
func (e *JiraError) ResourceKey() string {
	return e.resourceKey
}

// AvailableActions returns the list of valid actions an agent can take, if any.
func (e *JiraError) AvailableActions() []string {
	return e.availableActions
}

// NextCommand returns a suggested follow-up CLI command. The base implementation
// returns a configured command when present; subtypes override this with
// context-specific fallbacks.
func (e *JiraError) NextCommand() string {
	if e.nextCommand != "" {
		return e.nextCommand
	}
	return ""
}

// Error implements the error interface.
func (e *JiraError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error, enabling error chain traversal.
func (e *JiraError) Unwrap() error {
	return e.Cause
}

// ExitCode returns the process exit code for this error type.
func (e *JiraError) ExitCode() int {
	return 1
}

// newBase initializes a JiraError with options applied. Shared by all
// typed constructors to avoid repeating the option-application loop.
func newBase(message string, cause error, opts []ErrorOption) JiraError {
	e := JiraError{Message: message, Cause: cause}
	for _, o := range opts {
		o(&e)
	}
	return e
}

// NewJiraError creates a new JiraError wrapping the given cause.
func NewJiraError(message string, cause error, opts ...ErrorOption) *JiraError {
	e := newBase(message, cause, opts)
	return &e
}

// AuthError indicates that authentication failed or credentials are missing.
type AuthError struct {
	JiraError
}

// NewAuthError creates a new AuthError wrapping the given cause.
func NewAuthError(message string, cause error, opts ...ErrorOption) *AuthError {
	return &AuthError{JiraError: newBase(message, cause, opts)}
}

// ExitCode returns the process exit code for authentication errors.
func (e *AuthError) ExitCode() int {
	return 3
}

// NotFoundError indicates that the requested resource was not found.
type NotFoundError struct {
	JiraError
}

// NewNotFoundError creates a new NotFoundError wrapping the given cause.
func NewNotFoundError(message string, cause error, opts ...ErrorOption) *NotFoundError {
	return &NotFoundError{JiraError: newBase(message, cause, opts)}
}

// ExitCode returns the process exit code for not-found errors.
func (e *NotFoundError) ExitCode() int {
	return 2
}

// NextCommand returns a search command to help locate the missing resource.
// Returns an empty string when no resource key was provided.
func (e *NotFoundError) NextCommand() string {
	if e.nextCommand != "" {
		return e.nextCommand
	}
	if e.resourceKey == "" {
		return ""
	}
	return fmt.Sprintf("jira-agent issue search --jql \"key = %s\"", e.resourceKey)
}

// APIError indicates that the Jira API returned a non-2xx response.
type APIError struct {
	JiraError
	StatusCode int
	Body       string
}

// NewAPIError creates a new APIError wrapping the given cause.
func NewAPIError(message string, statusCode int, body string, cause error, opts ...ErrorOption) *APIError {
	return &APIError{
		JiraError:  newBase(message, cause, opts),
		StatusCode: statusCode,
		Body:       body,
	}
}

// ExitCode returns the process exit code for API errors.
func (e *APIError) ExitCode() int {
	return 4
}

// ValidationError indicates that input validation failed.
type ValidationError struct {
	JiraError
}

// NewValidationError creates a new ValidationError wrapping the given cause.
func NewValidationError(message string, cause error, opts ...ErrorOption) *ValidationError {
	return &ValidationError{JiraError: newBase(message, cause, opts)}
}

// ExitCode returns the process exit code for validation errors.
func (e *ValidationError) ExitCode() int {
	return 5
}

// NextCommand returns the command to enable write access when writes are
// blocked. Otherwise it falls back to any custom hint set via WithNextCommand.
func (e *ValidationError) NextCommand() string {
	if e.writeBlocked {
		return "export JIRA_ALLOW_WRITES=true"
	}
	return e.JiraError.NextCommand()
}

// exitCoder is an interface for types that can provide an exit code.
type exitCoder interface {
	error
	ExitCode() int
}

// Compile-time interface satisfaction: every error type must provide an exit
// code for CLI consumers. Checking exitCoder implicitly verifies error too.
var (
	_ exitCoder = (*JiraError)(nil)
	_ exitCoder = (*AuthError)(nil)
	_ exitCoder = (*NotFoundError)(nil)
	_ exitCoder = (*APIError)(nil)
	_ exitCoder = (*ValidationError)(nil)
)

// ExitCodeFor determines the appropriate exit code for the given error.
// Returns 0 if err is nil, otherwise extracts the exit code from the error
// chain or defaults to 1.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	if coder, ok := errors.AsType[exitCoder](err); ok {
		return coder.ExitCode()
	}

	return 1
}

// ErrorCode returns the machine-readable error classification for JSON responses.
func ErrorCode(err error) string {
	if _, ok := errors.AsType[*AuthError](err); ok {
		return "AUTH_FAILED"
	}

	if _, ok := errors.AsType[*NotFoundError](err); ok {
		return "NOT_FOUND"
	}

	if _, ok := errors.AsType[*APIError](err); ok {
		return "API_ERROR"
	}

	if _, ok := errors.AsType[*ValidationError](err); ok {
		return "VALIDATION_ERROR"
	}

	return "UNKNOWN"
}
