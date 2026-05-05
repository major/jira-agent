// Package output provides response formatting for jira-agent CLI output.
//
// All CLI output flows through this package to ensure a consistent structure
// for LLM consumers. JSON (default) wraps data in an envelope with metadata;
// CSV and TSV emit flat delimited rows suitable for spreadsheet import or
// piping to tools that prefer tabular data.
//
// Error responses always use JSON regardless of the chosen format.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	apperr "github.com/major/jira-agent/internal/errors"
)

type errorDetailsProvider interface {
	error
	Details() string
}

// remediationProvider is satisfied by errors that can suggest a follow-up
// command or list available actions for recovery.
type remediationProvider interface {
	error
	NextCommand() string
	AvailableActions() []string
}

// Metadata holds the standard metadata fields for response envelopes.
// Fields are tailored to Jira's paginated API responses.
type Metadata struct {
	Timestamp  string `json:"timestamp"`
	Total      int    `json:"total,omitempty"`
	Returned   int    `json:"returned,omitempty"`
	StartAt    int    `json:"start_at,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// NewMetadata returns metadata pre-populated with the current UTC timestamp.
func NewMetadata() Metadata {
	return Metadata{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Envelope is the standard JSON response wrapper for successful operations.
type Envelope struct {
	Data     any      `json:"data"`
	Errors   []string `json:"errors,omitempty"`
	Metadata Metadata `json:"metadata"`
}

// ErrorEnvelope is the standard JSON response wrapper for error responses.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code, message, and optional details.
type ErrorDetail struct {
	Code             string   `json:"code"`
	Message          string   `json:"message"`
	Details          string   `json:"details,omitempty"`
	NextCommand      string   `json:"next_command,omitempty"`
	AvailableActions []string `json:"available_actions,omitempty"`
}

var jiraJSONNoiseFields = map[string]struct{}{
	"avatarUrls": {},
	"expand":     {},
	"iconUrl":    {},
	"self":       {},
}

var jiraUserNoiseFields = map[string]struct{}{
	"accountType": {},
	"active":      {},
	"timeZone":    {},
}

// encodeJSON writes v as JSON to w with HTML escaping disabled.
func encodeJSON(w io.Writer, v any, pretty bool) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(v)
}

// WriteResult is a convenience wrapper for WriteSuccess with fresh metadata.
// Use WriteSuccess directly when custom metadata (e.g. pagination) is needed.
func WriteResult(w io.Writer, data any, format Format) error {
	return WriteSuccess(w, data, NewMetadata(), format)
}

// WriteSuccess writes a successful response with data and metadata to the writer.
// For CSV/TSV formats, only the data is emitted as delimited rows (no envelope).
func WriteSuccess(w io.Writer, data any, metadata Metadata, format Format) error {
	return writeEnvelope(w, data, nil, metadata, format, false)
}

// WriteRawSuccess writes a successful response without compacting Jira's JSON
// response shape. Use it only for commands that explicitly expose raw Jira API
// payloads as part of their public contract.
func WriteRawSuccess(w io.Writer, data any, metadata Metadata, format Format) error {
	return writeEnvelope(w, data, nil, metadata, format, true)
}

func writeEnvelope(w io.Writer, data any, errs []string, metadata Metadata, format Format, raw bool) error {
	switch format {
	case FormatCSV:
		return writeDelimited(w, data, ',')
	case FormatTSV:
		return writeDelimited(w, data, '\t')
	default:
		if !raw {
			data = compactJiraJSON(data)
		}
		envelope := Envelope{
			Data:     data,
			Errors:   errs,
			Metadata: metadata,
		}
		return encodeJSON(w, envelope, format == FormatJSONPretty)
	}
}

// WriteError writes an error response to the writer.
// It maps the error type to an appropriate error code string using apperr.ErrorCode().
func WriteError(w io.Writer, err error) error {
	code := apperr.ErrorCode(err)
	message := err.Error()

	// Extract details from the base error type.
	var details string
	if detailsErr, ok := errors.AsType[errorDetailsProvider](err); ok {
		details = detailsErr.Details()
	}

	// Include the upstream response body when available so API errors from
	// Jira are visible in the CLI output instead of just "status: NNN".
	if apiErr, ok := errors.AsType[*apperr.APIError](err); ok {
		if apiErr.Body != "" {
			details = fmt.Sprintf("status: %d, body: %s", apiErr.StatusCode, apiErr.Body)
		} else {
			details = fmt.Sprintf("status: %d", apiErr.StatusCode)
		}
	}

	// Extract remediation hints (next command, available actions) from errors
	// that implement the remediationProvider interface.
	var nextCommand string
	var availableActions []string
	if rp, ok := errors.AsType[remediationProvider](err); ok {
		nextCommand = rp.NextCommand()
		availableActions = rp.AvailableActions()
	}

	errorEnvelope := ErrorEnvelope{
		Error: ErrorDetail{
			Code:             code,
			Message:          message,
			Details:          details,
			NextCommand:      nextCommand,
			AvailableActions: availableActions,
		},
	}
	return encodeJSON(w, errorEnvelope, false)
}

// WritePartial writes a response with both data and errors (partial success).
// For CSV/TSV formats, only the data rows are emitted; error details require
// the JSON format.
func WritePartial(w io.Writer, data any, errs []string, metadata Metadata, format Format) error {
	return writeEnvelope(w, data, errs, metadata, format, false)
}

func compactJiraJSON(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return compactJiraJSONMap(v)
	case []any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, compactJiraJSON(item))
		}
		return items
	default:
		return value
	}
}

func compactJiraJSONMap(value map[string]any) map[string]any {
	compacted := make(map[string]any, len(value))
	userObject := isJiraUserObject(value)
	for key, nested := range value {
		if _, skip := jiraJSONNoiseFields[key]; skip {
			continue
		}
		if userObject {
			if _, skip := jiraUserNoiseFields[key]; skip {
				continue
			}
		}
		if key == "statusCategory" {
			if name, ok := jiraStatusCategoryName(nested); ok {
				compacted[key] = name
				continue
			}
		}
		compacted[key] = compactJiraJSON(nested)
	}
	return compacted
}

func isJiraUserObject(value map[string]any) bool {
	_, hasAccountID := value["accountId"]
	_, hasDisplayName := value["displayName"]
	_, hasEmailAddress := value["emailAddress"]
	_, hasAvatarURLs := value["avatarUrls"]
	return hasAccountID || hasDisplayName || hasEmailAddress || hasAvatarURLs
}

func jiraStatusCategoryName(value any) (string, bool) {
	category, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	name, ok := category["name"].(string)
	return name, ok
}
