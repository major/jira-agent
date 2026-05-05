package commands

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/major/jira-agent/internal/output"
)

func TestDryRunResultJSONFields(t *testing.T) {
	t.Parallel()

	result := DryRunResult{
		Command:  "issue start-work",
		IssueKey: "PROJ-123",
		Before:   map[string]any{"status": "Open"},
		After:    map[string]any{"status": "In Progress"},
		Diff:     []FieldChange{{Field: "status", OldValue: "Open", NewValue: "In Progress"}},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}

	for _, field := range []string{`"command"`, `"issue_key"`, `"before"`, `"after"`, `"diff"`, `"old_value"`, `"new_value"`} {
		if !strings.Contains(string(payload), field) {
			t.Errorf("payload = %s, want field %s", payload, field)
		}
	}
}

func TestComputeFieldDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before map[string]any
		after  map[string]any
		want   []FieldChange
	}{
		{
			name: "only changed fields sorted by name",
			before: map[string]any{
				"assignee": nil,
				"priority": "High",
				"status":   "Open",
			},
			after: map[string]any{
				"assignee": "abc123",
				"priority": "High",
				"status":   "In Progress",
			},
			want: []FieldChange{
				{Field: "assignee", OldValue: nil, NewValue: "abc123"},
				{Field: "status", OldValue: "Open", NewValue: "In Progress"},
			},
		},
		{
			name: "includes added and removed fields",
			before: map[string]any{
				"labels": []any{"bug"},
				"parent": "PROJ-1",
			},
			after: map[string]any{
				"labels":       []any{"bug"},
				"story_points": float64(3),
			},
			want: []FieldChange{
				{Field: "parent", OldValue: "PROJ-1", NewValue: nil},
				{Field: "story_points", OldValue: nil, NewValue: float64(3)},
			},
		},
		{
			name: "nil maps produce no diff",
			want: []FieldChange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ComputeFieldDiff(tt.before, tt.after)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeFieldDiff() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestIsDryRun(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer is false", func(t *testing.T) {
		t.Parallel()

		if IsDryRun(nil) {
			t.Error("IsDryRun(nil) = true, want false")
		}
	})

	t.Run("returns pointed value", func(t *testing.T) {
		t.Parallel()

		enabled := true
		if !IsDryRun(&enabled) {
			t.Error("IsDryRun(&true) = false, want true")
		}

		enabled = false
		if IsDryRun(&enabled) {
			t.Error("IsDryRun(&false) = true, want false")
		}
	})
}

func TestWriteDryRunResult(t *testing.T) {
	t.Parallel()

	result := DryRunResult{
		Command:  "issue start-work",
		IssueKey: "PROJ-123",
		Before: map[string]any{
			"status": "Open",
			"url":    "https://example.test?a=1&b=2",
		},
		After: map[string]any{
			"status": "In Progress",
			"url":    "https://example.test?a=1&b=2",
		},
		Diff: []FieldChange{
			{Field: "status", OldValue: "Open", NewValue: "In Progress"},
		},
	}

	var buf strings.Builder
	if err := WriteDryRunResult(&buf, result, output.FormatJSON); err != nil {
		t.Fatalf("WriteDryRunResult() error = %v, want nil", err)
	}

	if strings.Contains(buf.String(), `\u0026`) {
		t.Errorf("output = %q, want HTML escaping disabled", buf.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal([]byte(buf.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", envelope.Data)
	}
	if data["command"] != "issue start-work" {
		t.Errorf("command = %v, want issue start-work", data["command"])
	}
	if data["issue_key"] != "PROJ-123" {
		t.Errorf("issue_key = %v, want PROJ-123", data["issue_key"])
	}
	if envelope.Metadata.Timestamp == "" {
		t.Error("metadata timestamp is empty")
	}
}
