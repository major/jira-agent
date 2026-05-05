package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	apperr "github.com/major/jira-agent/internal/errors"
)

// fieldsPresets maps preset names to their comma-separated field lists.
// "key" is always included because issueSearchFields strips it before the
// API call (Jira always returns key), but it is useful in the preset
// definition for documentation clarity.
var fieldsPresets = map[string]string{
	"minimal": "key,summary,status",
	"triage":  "key,summary,status,priority,assignee,labels",
	"detail":  "key,summary,status,priority,assignee,labels,description,created,updated",
}

// applyFieldsPreset expands --fields-preset to --fields when set. It is a
// no-op when --fields-preset is not provided.
func applyFieldsPreset(cmd *cobra.Command) error {
	preset := mustGetString(cmd, "fields-preset")
	if preset == "" {
		return nil
	}
	expanded, err := expandFieldsPreset(preset)
	if err != nil {
		return err
	}
	return cmd.Flags().Set("fields", expanded)
}

// expandFieldsPreset returns the field list for a named preset or an error
// if the preset name is not recognized.
func expandFieldsPreset(name string) (string, error) {
	fields, ok := fieldsPresets[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		valid := make([]string, 0, len(fieldsPresets))
		for k := range fieldsPresets {
			valid = append(valid, k)
		}
		sort.Strings(valid)
		return "", apperr.NewValidationError(
			fmt.Sprintf("unknown --fields-preset %q (valid: %s)", name, strings.Join(valid, ", ")),
			nil,
		)
	}
	return fields, nil
}
