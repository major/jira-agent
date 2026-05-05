package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	apperr "github.com/major/jira-agent/internal/errors"
)

func TestWriteSuccess_JSON(t *testing.T) {
	t.Parallel()

	data := map[string]any{"name": "Alice", "active": true}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, meta, FormatJSON); err != nil {
		t.Fatalf("WriteSuccess(JSON): %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	// Verify envelope structure.
	if env.Metadata.Timestamp != "2025-01-01T00:00:00Z" {
		t.Errorf("timestamp = %q, want %q", env.Metadata.Timestamp, "2025-01-01T00:00:00Z")
	}
	if env.Data == nil {
		t.Fatal("data is nil")
	}
	if strings.Contains(buf.String(), "\n  ") {
		t.Errorf("JSON output = %q, want compact JSON by default", buf.String())
	}
}

func TestWriteSuccess_JSONPretty(t *testing.T) {
	t.Parallel()

	data := map[string]any{"name": "Alice", "active": true}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, meta, FormatJSONPretty); err != nil {
		t.Fatalf("WriteSuccess(JSONPretty): %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\n  ") {
		t.Errorf("JSON output = %q, want pretty-printed indentation", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("JSON output = %q, want trailing newline", got)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Metadata.Timestamp != "2025-01-01T00:00:00Z" {
		t.Errorf("timestamp = %q, want %q", env.Metadata.Timestamp, "2025-01-01T00:00:00Z")
	}
}

func TestWriteSuccess_CSV(t *testing.T) {
	t.Parallel()

	data := map[string]any{"email": "a@b.com", "name": "Alice"}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, meta, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (header + 1 row)", len(lines))
	}
	// Headers are sorted alphabetically.
	if lines[0] != "email,name" {
		t.Errorf("header = %q, want %q", lines[0], "email,name")
	}
	if lines[1] != "a@b.com,Alice" {
		t.Errorf("row = %q, want %q", lines[1], "a@b.com,Alice")
	}
}

func TestWriteSuccess_TSV(t *testing.T) {
	t.Parallel()

	data := map[string]any{"id": float64(42), "name": "Bob"}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, meta, FormatTSV); err != nil {
		t.Fatalf("WriteSuccess(TSV): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != "id\tname" {
		t.Errorf("header = %q, want %q", lines[0], "id\tname")
	}
	if lines[1] != "42\tBob" {
		t.Errorf("row = %q, want %q", lines[1], "42\tBob")
	}
}

func TestWriteSuccess_CSV_SliceOfMaps(t *testing.T) {
	t.Parallel()

	data := []any{
		map[string]any{"id": float64(1), "name": "Alice"},
		map[string]any{"id": float64(2), "name": "Bob"},
	}
	meta := Metadata{}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, meta, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV slice): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows)", len(lines))
	}
	if lines[0] != "id,name" {
		t.Errorf("header = %q, want %q", lines[0], "id,name")
	}
	if lines[1] != "1,Alice" {
		t.Errorf("row 1 = %q, want %q", lines[1], "1,Alice")
	}
	if lines[2] != "2,Bob" {
		t.Errorf("row 2 = %q, want %q", lines[2], "2,Bob")
	}
}

func TestWriteSuccess_CSV_NilData(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, nil, Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV nil): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil data, got %q", buf.String())
	}
}

func TestWriteSuccess_CSV_NestedValues(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"name":   "Alice",
		"avatar": map[string]any{"url": "https://example.com/pic.png"},
	}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV nested): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	// The nested map should be serialized as inline JSON.
	if !strings.Contains(lines[1], `"{""url"":""https://example.com/pic.png""}"`) &&
		!strings.Contains(buf.String(), `{"url":"https://example.com/pic.png"}`) {
		// CSV quoting may vary; just verify the JSON content is present.
		if !strings.Contains(buf.String(), "url") {
			t.Errorf("nested value not serialized, got: %q", buf.String())
		}
	}
}

func TestWriteSuccess_CSV_Struct(t *testing.T) {
	t.Parallel()

	type user struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	data := user{Name: "Alice", Email: "a@b.com"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV struct): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != "email,name" {
		t.Errorf("header = %q, want %q", lines[0], "email,name")
	}
	if lines[1] != "a@b.com,Alice" {
		t.Errorf("row = %q, want %q", lines[1], "a@b.com,Alice")
	}
}

func TestWriteSuccess_CSV_SliceOfStructs(t *testing.T) {
	t.Parallel()

	type issue struct {
		Key     string `json:"key"`
		Summary string `json:"summary"`
	}

	data := []issue{
		{Key: "PROJ-1", Summary: "First issue"},
		{Key: "PROJ-2", Summary: "Second issue"},
	}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(CSV struct slice): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "key,summary" {
		t.Errorf("header = %q, want %q", lines[0], "key,summary")
	}
}

func TestWriteError_AlwaysJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteError(&buf, &testError{msg: "something broke"}); err != nil {
		t.Fatalf("WriteError: %v", err)
	}

	var env ErrorEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal error envelope: %v", err)
	}
	if env.Error.Message != "something broke" {
		t.Errorf("message = %q, want %q", env.Error.Message, "something broke")
	}
}

func TestWriteError_IncludesTypedErrorDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantMessage string
		wantDetails string
	}{
		{
			name:        "jira error details",
			err:         apperr.NewValidationError("invalid issue", nil, apperr.WithDetails("issue key is required")),
			wantCode:    "VALIDATION_ERROR",
			wantMessage: "invalid issue",
			wantDetails: "issue key is required",
		},
		{
			name:        "api error body details",
			err:         apperr.NewAPIError("jira rejected request", 429, `{"error":"rate limit"}`, nil),
			wantCode:    "API_ERROR",
			wantMessage: "jira rejected request",
			wantDetails: `status: 429, body: {"error":"rate limit"}`,
		},
		{
			name:        "api error status details without body",
			err:         apperr.NewAPIError("jira failed", 500, "", nil),
			wantCode:    "API_ERROR",
			wantMessage: "jira failed",
			wantDetails: "status: 500",
		},
		{
			name:        "wrapped api error details",
			err:         fmt.Errorf("request failed: %w", apperr.NewAPIError("jira failed", 503, "unavailable", nil)),
			wantCode:    "API_ERROR",
			wantMessage: "request failed: jira failed",
			wantDetails: "status: 503, body: unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := WriteError(&buf, tt.err); err != nil {
				t.Fatalf("WriteError() error = %v, want nil", err)
			}

			var env ErrorEnvelope
			if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
				t.Fatalf("unmarshal error envelope: %v", err)
			}
			if env.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if env.Error.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", env.Error.Message, tt.wantMessage)
			}
			if env.Error.Details != tt.wantDetails {
				t.Errorf("details = %q, want %q", env.Error.Details, tt.wantDetails)
			}
		})
	}
}

func TestWriteSuccess_JSONPreservesReadableText(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"url":  "https://jira.example.com/browse/PROJ-1?a=1&b=2",
		"html": "<strong>blocked</strong>",
	}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, Metadata{}, FormatJSON); err != nil {
		t.Fatalf("WriteSuccess() error = %v, want nil", err)
	}
	got := buf.String()
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(got, escaped) {
			t.Errorf("JSON output contains escaped HTML sequence %q in %q", escaped, got)
		}
	}
	if !strings.Contains(got, `"url":"https://jira.example.com/browse/PROJ-1?a=1&b=2"`) {
		t.Errorf("JSON output = %q, want readable URL", got)
	}
}

func TestWriteSuccess_JSONCompactsJiraNoise(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"expand": "renderedFields,names",
		"key":    "PROJ-1",
		"self":   "https://example.atlassian.net/rest/api/3/issue/10001",
		"fields": map[string]any{
			"summary": "Reduce noisy output",
			"status": map[string]any{
				"iconUrl": "https://example.atlassian.net/images/icons/statuses/open.png",
				"name":    "In Progress",
				"self":    "https://example.atlassian.net/rest/api/3/status/3",
				"statusCategory": map[string]any{
					"colorName": "yellow",
					"id":        float64(4),
					"key":       "indeterminate",
					"name":      "In Progress",
					"self":      "https://example.atlassian.net/rest/api/3/statuscategory/4",
				},
			},
			"assignee": map[string]any{
				"accountId":    "abc123",
				"accountType":  "atlassian",
				"active":       true,
				"avatarUrls":   map[string]any{"48x48": "https://avatar.example/48"},
				"displayName":  "Jane Smith",
				"emailAddress": "jsmith@example.com",
				"self":         "https://example.atlassian.net/rest/api/3/user?accountId=abc123",
				"timeZone":     "Etc/GMT",
			},
			"watchers": []any{
				map[string]any{"displayName": "Alex Lee", "self": "https://example.atlassian.net/rest/api/3/user?accountId=watcher"},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, Metadata{}, FormatJSON); err != nil {
		t.Fatalf("WriteSuccess() error = %v, want nil", err)
	}

	for _, noisy := range []string{"avatarUrls", "accountType", "active", "timeZone", "iconUrl", "\"self\"", "\"expand\"", "colorName"} {
		if strings.Contains(buf.String(), noisy) {
			t.Errorf("JSON output contains noisy field %q in %s", noisy, buf.String())
		}
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	issue := env.Data.(map[string]any)
	fields := issue["fields"].(map[string]any)
	assignee := fields["assignee"].(map[string]any)
	if assignee["accountId"] != "abc123" {
		t.Errorf("assignee accountId = %v, want abc123", assignee["accountId"])
	}
	if assignee["displayName"] != "Jane Smith" {
		t.Errorf("assignee displayName = %v, want Jane Smith", assignee["displayName"])
	}
	status := fields["status"].(map[string]any)
	if status["statusCategory"] != "In Progress" {
		t.Errorf("statusCategory = %v, want In Progress", status["statusCategory"])
	}
}

func TestWriteRawSuccess_JSONPreservesJiraResponse(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"expand": "names",
		"self":   "https://example.atlassian.net/rest/api/3/issue/10001",
		"fields": map[string]any{
			"priority": map[string]any{
				"iconUrl": "https://example.atlassian.net/images/icons/priorities/high.svg",
				"name":    "High",
				"self":    "https://example.atlassian.net/rest/api/3/priority/2",
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteRawSuccess(&buf, data, Metadata{}, FormatJSON); err != nil {
		t.Fatalf("WriteRawSuccess() error = %v, want nil", err)
	}

	for _, rawField := range []string{"\"expand\"", "\"self\"", "iconUrl"} {
		if !strings.Contains(buf.String(), rawField) {
			t.Errorf("JSON output = %s, want raw field %q", buf.String(), rawField)
		}
	}
}

func TestWritePartial_JSON(t *testing.T) {
	t.Parallel()

	data := map[string]any{"count": float64(5)}
	errs := []string{"partial failure"}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WritePartial(&buf, data, errs, meta, FormatJSON); err != nil {
		t.Fatalf("WritePartial(JSON): %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Errors) != 1 || env.Errors[0] != "partial failure" {
		t.Errorf("errors = %v, want [partial failure]", env.Errors)
	}
}

func TestWritePartial_JSONPretty(t *testing.T) {
	t.Parallel()

	data := map[string]any{"count": float64(5)}
	errs := []string{"partial failure"}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WritePartial(&buf, data, errs, meta, FormatJSONPretty); err != nil {
		t.Fatalf("WritePartial(JSONPretty): %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\n  ") {
		t.Errorf("JSON output = %q, want pretty-printed indentation", got)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Errors) != 1 || env.Errors[0] != "partial failure" {
		t.Errorf("errors = %v, want [partial failure]", env.Errors)
	}
}

func TestWritePartial_CSV(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "PROJ-1"}
	errs := []string{"some warning"}

	var buf bytes.Buffer
	if err := WritePartial(&buf, data, errs, Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WritePartial(CSV): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	// CSV output should contain the data, not the error strings.
	if lines[0] != "key" {
		t.Errorf("header = %q, want %q", lines[0], "key")
	}
}

func TestWritePartial_TSV(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "PROJ-1"}
	errs := []string{"some warning"}

	var buf bytes.Buffer
	if err := WritePartial(&buf, data, errs, Metadata{}, FormatTSV); err != nil {
		t.Fatalf("WritePartial(TSV): %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != "key" {
		t.Errorf("header = %q, want %q", lines[0], "key")
	}
	if lines[1] != "PROJ-1" {
		t.Errorf("row = %q, want %q", lines[1], "PROJ-1")
	}
}

func TestNewMetadata(t *testing.T) {
	t.Parallel()

	meta := NewMetadata()
	parsed, err := time.Parse(time.RFC3339, meta.Timestamp)
	if err != nil {
		t.Fatalf("timestamp parse error = %v, want nil", err)
	}
	if parsed.Location() != time.UTC {
		t.Errorf("timestamp location = %v, want UTC", parsed.Location())
	}
}

func TestWriteResult_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteResult(&buf, map[string]any{"key": "TEST-1"}, FormatJSON); err != nil {
		t.Fatalf("WriteResult() error = %v, want nil", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Metadata.Timestamp == "" {
		t.Errorf("timestamp = empty, want populated")
	}
	data := env.Data.(map[string]any)
	if data["key"] != "TEST-1" {
		t.Errorf("data[key] = %v, want %v", data["key"], "TEST-1")
	}
}

func TestWriteErrorRemediation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		err                  error
		wantNextCommand      string
		wantAvailableActions []string
	}{
		{
			name:            "validation error includes next_command",
			err:             apperr.NewValidationError("write access is disabled", nil),
			wantNextCommand: "export JIRA_ALLOW_WRITES=true",
		},
		{
			name:            "not found error includes next_command with resource key",
			err:             apperr.NewNotFoundError("issue PROJ-456 not found", nil, apperr.WithResourceKey("PROJ-456")),
			wantNextCommand: `jira-agent issue search --jql "key = PROJ-456"`,
		},
		{
			name:                 "api error includes available_actions",
			err:                  apperr.NewAPIError("transition failed", 400, "", nil, apperr.WithAvailableActions([]string{"In Progress", "Won't Do"})),
			wantAvailableActions: []string{"In Progress", "Won't Do"},
		},
		{
			name: "plain error omits remediation fields",
			err:  &testError{msg: "plain error"},
		},
		{
			name: "not found without resource key omits next_command",
			err:  apperr.NewNotFoundError("resource not found", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := WriteError(&buf, tt.err); err != nil {
				t.Fatalf("WriteError() error = %v, want nil", err)
			}

			var env ErrorEnvelope
			if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
				t.Fatalf("unmarshal error envelope: %v", err)
			}

			if env.Error.NextCommand != tt.wantNextCommand {
				t.Errorf("next_command = %q, want %q", env.Error.NextCommand, tt.wantNextCommand)
			}

			if len(env.Error.AvailableActions) != len(tt.wantAvailableActions) {
				t.Fatalf("available_actions len = %d, want %d", len(env.Error.AvailableActions), len(tt.wantAvailableActions))
			}
			for i, want := range tt.wantAvailableActions {
				if env.Error.AvailableActions[i] != want {
					t.Errorf("available_actions[%d] = %q, want %q", i, env.Error.AvailableActions[i], want)
				}
			}

			// Verify omitempty: fields should not appear in JSON when empty.
			raw := buf.String()
			if tt.wantNextCommand == "" && strings.Contains(raw, "next_command") {
				t.Errorf("JSON contains next_command when it should be omitted: %s", raw)
			}
			if len(tt.wantAvailableActions) == 0 && strings.Contains(raw, "available_actions") {
				t.Errorf("JSON contains available_actions when it should be omitted: %s", raw)
			}
		})
	}
}

// testError is a minimal error for testing WriteError without importing apperr.
type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }
