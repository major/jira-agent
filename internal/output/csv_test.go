package output

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
)

type csvTaggedStruct struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteDelimited_MapRows(t *testing.T) {
	t.Parallel()

	data := []map[string]any{
		{
			"name":   "alpha,beta",
			"count":  float64(42),
			"active": true,
			"nested": map[string]any{"key": "value"},
			"empty":  nil,
		},
	}

	var buf bytes.Buffer
	if err := writeDelimited(&buf, data, ','); err != nil {
		t.Fatalf("writeDelimited() error = %v, want nil", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records length = %d, want %d", len(records), 2)
	}
	wantHeader := []string{"active", "count", "empty", "name", "nested"}
	for i, want := range wantHeader {
		if records[0][i] != want {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], want)
		}
	}
	wantRow := []string{"true", "42", "", "alpha,beta", `{"key":"value"}`}
	for i, want := range wantRow {
		if records[1][i] != want {
			t.Errorf("row[%d] = %q, want %q", i, records[1][i], want)
		}
	}
}

func TestNormalizeToMaps_Structs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data any
		want int
	}{
		{name: "single struct", data: csvTaggedStruct{Name: "one", Count: 1}, want: 1},
		{name: "struct pointer", data: &csvTaggedStruct{Name: "one", Count: 1}, want: 1},
		{name: "struct slice", data: []csvTaggedStruct{{Name: "one", Count: 1}, {Name: "two", Count: 2}}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows, err := normalizeToMaps(tt.data)
			if err != nil {
				t.Fatalf("normalizeToMaps() error = %v, want nil", err)
			}
			if len(rows) != tt.want {
				t.Errorf("rows length = %d, want %d", len(rows), tt.want)
			}
			if got := rows[0]["name"]; got != "one" {
				t.Errorf("rows[0][name] = %v, want %v", got, "one")
			}
		})
	}
}

func TestNormalizeToMaps_Errors(t *testing.T) {
	t.Parallel()

	_, err := normalizeToMaps("plain string")
	if err == nil {
		t.Fatal("normalizeToMaps() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "requires a map, slice, or struct") {
		t.Errorf("normalizeToMaps() error = %q, want requires a map, slice, or struct", err.Error())
	}
}

// failWriter is an io.Writer that always returns an error. Small writes
// succeed inside bufio's buffer, so the error surfaces only when the
// csv.Writer is flushed.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func TestWriteDelimited_FlushError(t *testing.T) {
	t.Parallel()

	data := []map[string]any{
		{"key": "VAL-1", "summary": "test issue"},
	}

	err := writeDelimited(failWriter{}, data, ',')
	if err == nil {
		t.Fatal("writeDelimited() error = nil, want flush error propagated")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("writeDelimited() error = %q, want containing %q", err.Error(), "disk full")
	}
}

func TestFormatValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "string", in: "hello", want: "hello"},
		{name: "true", in: true, want: "true"},
		{name: "false", in: false, want: "false"},
		{name: "whole float", in: float64(42), want: "42"},
		{name: "fractional float", in: 42.5, want: "42.5"},
		{name: "slice", in: []string{"a", "b"}, want: `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := formatValue(tt.in); got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
