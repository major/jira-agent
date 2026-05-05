package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCompactStripNulls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		wantKeys []string
		goneKeys []string
	}{
		{
			name: "removes null values",
			input: map[string]any{
				"key":         "PROJ-1",
				"summary":     "Test issue",
				"description": nil,
				"assignee":    nil,
			},
			wantKeys: []string{"key", "summary"},
			goneKeys: []string{"description", "assignee"},
		},
		{
			name: "removes empty string values",
			input: map[string]any{
				"key":     "PROJ-1",
				"summary": "Test issue",
				"labels":  "",
			},
			wantKeys: []string{"key", "summary"},
			goneKeys: []string{"labels"},
		},
		{
			name: "removes empty array values",
			input: map[string]any{
				"key":     "PROJ-1",
				"summary": "Test issue",
				"labels":  []any{},
			},
			wantKeys: []string{"key", "summary"},
			goneKeys: []string{"labels"},
		},
		{
			name: "removes empty map values",
			input: map[string]any{
				"key":    "PROJ-1",
				"fields": map[string]any{},
			},
			wantKeys: []string{"key"},
			goneKeys: []string{"fields"},
		},
		{
			name: "preserves non-empty values",
			input: map[string]any{
				"key":     "PROJ-1",
				"summary": "Test issue",
				"count":   float64(0),
				"active":  false,
				"labels":  []any{"bug"},
			},
			wantKeys: []string{"key", "summary", "count", "active", "labels"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := compactStripEmpty(tt.input)
			result, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("compactStripEmpty() type = %T, want map[string]any", got)
			}

			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("expected key %q to be present", key)
				}
			}
			for _, key := range tt.goneKeys {
				if _, ok := result[key]; ok {
					t.Errorf("expected key %q to be stripped, but it was present", key)
				}
			}
		})
	}
}

func TestCompactStripNullsNested(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"summary":     "Test",
			"description": nil,
			"assignee": map[string]any{
				"displayName": "Alice",
				"email":       nil,
			},
		},
	}

	stripped := compactStripEmpty(input)
	got, ok := stripped.(map[string]any)
	if !ok {
		t.Fatalf("compactStripEmpty() type = %T, want map[string]any", stripped)
	}
	fields, ok := got["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields type = %T, want map[string]any", got["fields"])
	}

	if _, ok := fields["description"]; ok {
		t.Errorf("nested null field 'description' should be stripped")
	}
	assignee, ok := fields["assignee"].(map[string]any)
	if !ok {
		t.Fatalf("assignee type = %T, want map[string]any", fields["assignee"])
	}
	if _, ok := assignee["email"]; ok {
		t.Errorf("deeply nested null field 'email' should be stripped")
	}
	if assignee["displayName"] != "Alice" {
		t.Errorf("displayName = %v, want Alice", assignee["displayName"])
	}
}

func TestCompactFlatten(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      map[string]any
		wantKeys   map[string]any
		wantAbsent []string
	}{
		{
			name: "flattens single-key nested objects to dot notation",
			input: map[string]any{
				"key": "PROJ-1",
				"status": map[string]any{
					"name": "In Progress",
				},
				"priority": map[string]any{
					"name": "High",
				},
			},
			wantKeys: map[string]any{
				"key":           "PROJ-1",
				"status.name":   "In Progress",
				"priority.name": "High",
			},
			wantAbsent: []string{"status", "priority"},
		},
		{
			name: "does not flatten multi-key nested objects",
			input: map[string]any{
				"key": "PROJ-1",
				"assignee": map[string]any{
					"displayName": "Alice",
					"accountId":   "abc123",
				},
			},
			wantKeys: map[string]any{
				"key": "PROJ-1",
			},
		},
		{
			name: "does not flatten deeply nested (only one level)",
			input: map[string]any{
				"outer": map[string]any{
					"inner": map[string]any{
						"deep": "value",
					},
				},
			},
			wantKeys: map[string]any{
				"outer.inner": map[string]any{"deep": "value"},
			},
			wantAbsent: []string{"outer"},
		},
		{
			name: "preserves non-map values",
			input: map[string]any{
				"key":    "PROJ-1",
				"count":  float64(42),
				"labels": []any{"bug"},
			},
			wantKeys: map[string]any{
				"key":    "PROJ-1",
				"count":  float64(42),
				"labels": nil, // just check key exists
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := compactFlatten(tt.input)

			for key, wantVal := range tt.wantKeys {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("expected key %q to be present in %v", key, got)
					continue
				}
				if wantVal != nil {
					// For map comparison, marshal to JSON.
					if _, isMap := wantVal.(map[string]any); isMap {
						wantJSON, _ := json.Marshal(wantVal)
						gotJSON, _ := json.Marshal(gotVal)
						if !bytes.Equal(wantJSON, gotJSON) {
							t.Errorf("key %q = %v, want %v", key, gotVal, wantVal)
						}
					} else if gotVal != wantVal {
						t.Errorf("key %q = %v, want %v", key, gotVal, wantVal)
					}
				}
			}
			for _, key := range tt.wantAbsent {
				if _, ok := got[key]; ok {
					t.Errorf("expected key %q to be absent after flattening", key)
				}
			}
		})
	}
}

func TestCompactOutput_JSONLines(t *testing.T) {
	t.Parallel()

	data := []any{
		map[string]any{"key": "PROJ-1", "summary": "First"},
		map[string]any{"key": "PROJ-2", "summary": "Second"},
		map[string]any{"key": "PROJ-3", "summary": "Third"},
	}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, &meta, FormatJSON, WithCompact(true)); err != nil {
		t.Fatalf("WriteSuccess(compact) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// JSON Lines: each array element on its own line, no outer array wrapper.
	if len(lines) != len(data) {
		t.Fatalf("got %d lines, want %d (one per array element)", len(lines), len(data))
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		// Each line should have data and metadata envelope.
		if _, ok := obj["data"]; !ok {
			t.Errorf("line %d: missing 'data' key", i)
		}
		if _, ok := obj["metadata"]; !ok {
			t.Errorf("line %d: missing 'metadata' key", i)
		}
	}
}

func TestCompactOutput_SingleObject(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"key":      "PROJ-1",
		"summary":  "Test issue",
		"assignee": nil,
	}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, &meta, FormatJSON, WithCompact(true)); err != nil {
		t.Fatalf("WriteSuccess(compact) error = %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	// Compact should strip nulls.
	d, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("envelope data type = %T, want map[string]any", env.Data)
	}
	if _, ok := d["assignee"]; ok {
		t.Errorf("expected null 'assignee' to be stripped in compact mode")
	}
	if d["key"] != "PROJ-1" {
		t.Errorf("key = %v, want PROJ-1", d["key"])
	}
}

func TestCompactOutput_NoEffectOnCSV(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "PROJ-1", "empty": nil}

	var compactBuf, normalBuf bytes.Buffer
	if err := WriteSuccess(&compactBuf, data, &Metadata{}, FormatCSV, WithCompact(true)); err != nil {
		t.Fatalf("WriteSuccess(compact CSV) error = %v", err)
	}
	if err := WriteSuccess(&normalBuf, data, &Metadata{}, FormatCSV); err != nil {
		t.Fatalf("WriteSuccess(normal CSV) error = %v", err)
	}

	if compactBuf.String() != normalBuf.String() {
		t.Errorf("compact should not affect CSV output\ngot:  %q\nwant: %q", compactBuf.String(), normalBuf.String())
	}
}

func TestCompactOutput_DefaultOff(t *testing.T) {
	t.Parallel()

	// Without compact option, nulls should be preserved in the envelope data.
	data := map[string]any{
		"key":         "PROJ-1",
		"description": nil,
	}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, &Metadata{}, FormatJSON); err != nil {
		t.Fatalf("WriteSuccess() error = %v", err)
	}

	// Default behavior: null values remain in the JSON output.
	if !strings.Contains(buf.String(), `"description":null`) {
		t.Errorf("default mode should preserve null values, got: %s", buf.String())
	}
}

func TestCompactOutput_FlattenInEnvelope(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"key": "PROJ-1",
		"status": map[string]any{
			"name": "Open",
		},
	}
	meta := Metadata{Timestamp: "2025-01-01T00:00:00Z"}

	var buf bytes.Buffer
	if err := WriteSuccess(&buf, data, &meta, FormatJSON, WithCompact(true)); err != nil {
		t.Fatalf("WriteSuccess(compact) error = %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	d, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("envelope data type = %T, want map[string]any", env.Data)
	}
	if d["status.name"] != "Open" {
		t.Errorf("expected flattened 'status.name' = Open, got %v", d["status.name"])
	}
	if _, ok := d["status"]; ok {
		t.Errorf("expected 'status' to be removed after flattening")
	}
}
