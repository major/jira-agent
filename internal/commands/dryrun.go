package commands

import (
	"io"
	"reflect"
	"sort"

	"github.com/major/jira-agent/internal/output"
)

// DryRunResult describes a planned mutating command without performing it.
type DryRunResult struct {
	Command  string         `json:"command"`
	IssueKey string         `json:"issue_key,omitempty"`
	Before   map[string]any `json:"before"`
	After    map[string]any `json:"after"`
	Diff     []FieldChange  `json:"diff"`
}

// FieldChange describes a single field-level difference in a dry-run result.
type FieldChange struct {
	Field    string `json:"field"`
	OldValue any    `json:"old_value"`
	NewValue any    `json:"new_value"`
}

// ComputeFieldDiff returns deterministic field changes between two snapshots.
func ComputeFieldDiff(before, after map[string]any) []FieldChange {
	fieldSet := make(map[string]struct{}, len(before)+len(after))
	for field := range before {
		fieldSet[field] = struct{}{}
	}
	for field := range after {
		fieldSet[field] = struct{}{}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	changes := make([]FieldChange, 0, len(fields))
	for _, field := range fields {
		oldValue := before[field]
		newValue := after[field]
		if reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		changes = append(changes, FieldChange{
			Field:    field,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}

	return changes
}

// WriteDryRunResult writes a dry-run result through the standard output envelope.
func WriteDryRunResult(w io.Writer, result DryRunResult, format output.Format, opts ...output.WriteOption) error {
	return output.WriteSuccess(w, result, output.NewMetadata(), format, opts...)
}

// IsDryRun safely reports whether dry-run mode is enabled.
func IsDryRun(dryRun *bool) bool {
	return dryRun != nil && *dryRun
}
