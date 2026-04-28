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
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
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
	return writeEnvelope(w, data, nil, metadata, format)
}

func writeEnvelope(w io.Writer, data any, errs []string, metadata Metadata, format Format) error {
	switch format {
	case FormatCSV:
		return writeDelimited(w, data, ',')
	case FormatTSV:
		return writeDelimited(w, data, '\t')
	default:
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

	errorEnvelope := ErrorEnvelope{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	return encodeJSON(w, errorEnvelope, false)
}

// WritePartial writes a response with both data and errors (partial success).
// For CSV/TSV formats, only the data rows are emitted; error details require
// the JSON format.
func WritePartial(w io.Writer, data any, errs []string, metadata Metadata, format Format) error {
	return writeEnvelope(w, data, errs, metadata, format)
}
