package jira_test

import (
	"testing"

	"github.com/major/jira-agent/internal/jira"
)

func TestExtract_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{name: "present", m: map[string]any{"k": "val"}, key: "k", want: "val"},
		{name: "missing key", m: map[string]any{"other": "val"}, key: "k", want: ""},
		{name: "wrong type", m: map[string]any{"k": 42}, key: "k", want: ""},
		{name: "nil map", m: nil, key: "k", want: ""},
		{name: "empty string", m: map[string]any{"k": ""}, key: "k", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).String(tt.key)
			if got != tt.want {
				t.Errorf("String(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtract_Bool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want bool
	}{
		{name: "true", m: map[string]any{"k": true}, key: "k", want: true},
		{name: "false", m: map[string]any{"k": false}, key: "k", want: false},
		{name: "missing key", m: map[string]any{}, key: "k", want: false},
		{name: "wrong type", m: map[string]any{"k": "true"}, key: "k", want: false},
		{name: "nil map", m: nil, key: "k", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).Bool(tt.key)
			if got != tt.want {
				t.Errorf("Bool(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtract_Int64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want int64
	}{
		{name: "float64", m: map[string]any{"k": float64(42)}, key: "k", want: 42},
		{name: "int64", m: map[string]any{"k": int64(99)}, key: "k", want: 99},
		{name: "int", m: map[string]any{"k": int(7)}, key: "k", want: 7},
		{name: "missing key", m: map[string]any{}, key: "k", want: 0},
		{name: "wrong type", m: map[string]any{"k": "42"}, key: "k", want: 0},
		{name: "nil map", m: nil, key: "k", want: 0},
		{name: "zero", m: map[string]any{"k": float64(0)}, key: "k", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).Int64(tt.key)
			if got != tt.want {
				t.Errorf("Int64(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtract_Float64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want float64
	}{
		{name: "present", m: map[string]any{"k": 3.14}, key: "k", want: 3.14},
		{name: "missing key", m: map[string]any{}, key: "k", want: 0},
		{name: "wrong type", m: map[string]any{"k": "3.14"}, key: "k", want: 0},
		{name: "nil map", m: nil, key: "k", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).Float64(tt.key)
			if got != tt.want {
				t.Errorf("Float64(%q) = %f, want %f", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtract_Map(t *testing.T) {
	t.Parallel()
	nested := map[string]any{"inner": "value"}
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantNil bool
	}{
		{name: "present", m: map[string]any{"k": nested}, key: "k", wantNil: false},
		{name: "missing key", m: map[string]any{}, key: "k", wantNil: true},
		{name: "wrong type", m: map[string]any{"k": "not a map"}, key: "k", wantNil: true},
		{name: "nil map", m: nil, key: "k", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).Map(tt.key)
			if tt.wantNil && got != nil {
				t.Errorf("Map(%q) = %v, want nil", tt.key, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Map(%q) = nil, want non-nil", tt.key)
			}
		})
	}
}

func TestExtract_Slice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantNil bool
	}{
		{name: "present", m: map[string]any{"k": []any{1, 2}}, key: "k", wantNil: false},
		{name: "missing key", m: map[string]any{}, key: "k", wantNil: true},
		{name: "wrong type", m: map[string]any{"k": "not a slice"}, key: "k", wantNil: true},
		{name: "nil map", m: nil, key: "k", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := jira.NewExtract(tt.m).Slice(tt.key)
			if tt.wantNil && got != nil {
				t.Errorf("Slice(%q) = %v, want nil", tt.key, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Slice(%q) = nil, want non-nil", tt.key)
			}
		})
	}
}

func TestExtract_Nested(t *testing.T) {
	t.Parallel()

	t.Run("chained access", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{
			"fields": map[string]any{
				"status": map[string]any{
					"name": "In Progress",
				},
			},
		}
		got := jira.NewExtract(m).Nested("fields").Nested("status").String("name")
		if got != "In Progress" {
			t.Errorf("chained Nested().String() = %q, want %q", got, "In Progress")
		}
	})

	t.Run("missing intermediate key", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{"fields": map[string]any{}}
		got := jira.NewExtract(m).Nested("fields").Nested("status").String("name")
		if got != "" {
			t.Errorf("missing intermediate key = %q, want empty", got)
		}
	})

	t.Run("nil map nested", func(t *testing.T) {
		t.Parallel()
		got := jira.NewExtract(nil).Nested("any").String("key")
		if got != "" {
			t.Errorf("nil map nested = %q, want empty", got)
		}
	})
}
