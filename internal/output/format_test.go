package output

import (
	"testing"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    Format
		wantErr bool
	}{
		{name: "json lowercase", input: "json", want: FormatJSON},
		{name: "csv lowercase", input: "csv", want: FormatCSV},
		{name: "tsv lowercase", input: "tsv", want: FormatTSV},
		{name: "JSON uppercase", input: "JSON", want: FormatJSON},
		{name: "CSV uppercase", input: "CSV", want: FormatCSV},
		{name: "TSV uppercase", input: "TSV", want: FormatTSV},
		{name: "mixed case", input: "Csv", want: FormatCSV},
		{name: "empty defaults to json", input: "", want: FormatJSON},
		{name: "whitespace defaults to json", input: "  ", want: FormatJSON},
		{name: "leading/trailing space", input: " csv ", want: FormatCSV},
		{name: "invalid format", input: "xml", wantErr: true},
		{name: "text is invalid", input: "text", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFormat(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFormat(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
