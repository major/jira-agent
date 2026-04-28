// Package output provides response formatting for jira-agent CLI output.
//
// Supported formats: JSON (default), CSV, and TSV. JSON wraps data in an
// envelope with metadata; CSV/TSV emit flat delimited rows suitable for
// spreadsheet import or piping to other tools.
package output

import (
	"fmt"
	"strings"
)

// Format represents a supported output format.
type Format string

const (
	// FormatJSON is the default structured envelope format.
	FormatJSON Format = "json"

	// FormatJSONPretty writes indented JSON for humans.
	FormatJSONPretty Format = "json-pretty"

	// FormatCSV writes comma-separated values with a header row.
	FormatCSV Format = "csv"

	// FormatTSV writes tab-separated values with a header row.
	FormatTSV Format = "tsv"
)

// ParseFormat converts a user-supplied string into a validated Format.
// An empty string defaults to FormatJSON.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatJSON, "":
		return FormatJSON, nil
	case FormatCSV:
		return FormatCSV, nil
	case FormatTSV:
		return FormatTSV, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (valid: json, csv, tsv)", s)
	}
}
