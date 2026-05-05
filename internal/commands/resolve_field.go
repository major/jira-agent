package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// resolvedField represents a field resolved from Jira with minimal fields.
type resolvedField struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

// fieldResolveCommand returns the "resolve field" subcommand for resolving
// field queries (name) to field IDs.
func fieldResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "field <query>",
		Short:   "Resolve field by name",
		Example: `jira-agent resolve field "summary"
jira-agent resolve field "story points"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate query argument
			query, err := requireQuery(args, "field")
			if err != nil {
				return err
			}

			// Call Jira API to search for fields
			params := map[string]string{
				"query":      query,
				"maxResults": "10",
			}

			// Jira API returns object with "values" key containing array of field objects
			var jiraResponse map[string]any
			if err := apiClient.Get(ctx, "/field/search", params, &jiraResponse); err != nil {
				return err
			}

			// Extract values array from response
			values, ok := jiraResponse["values"]
			if !ok {
				return apperr.NewJiraError(
					"unexpected response: missing 'values' key",
					nil,
				)
			}

			// Convert to slice of maps
			jiraFields, ok := values.([]any)
			if !ok {
				return apperr.NewJiraError(
					"unexpected response: 'values' is not an array",
					nil,
				)
			}

			// Check if no fields found
			if len(jiraFields) == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no fields found matching %q", query),
					nil,
					apperr.WithAvailableActions([]string{"jira-agent field list", "jira-agent field search --query <name>"}),
				)
			}

			// Map Jira response to resolvedField, stripping extra fields
			fields := make([]resolvedField, len(jiraFields))
			for i, jiraField := range jiraFields {
				fieldMap, ok := jiraField.(map[string]any)
				if !ok {
					continue
				}
				fields[i] = resolvedField{
					ID:     GetStringField(fieldMap, "id"),
					Name:   GetStringField(fieldMap, "name"),
					Custom: GetBoolField(fieldMap, "custom"),
				}
			}

			// Build metadata with usage hint
			meta := resolverMetadata(
				len(fields),
				len(fields),
				"jira-agent issue get <issue-key> --fields <id>",
			)

			return output.WriteSuccess(w, fields, meta, *format)
		},
	}

	return cmd
}
