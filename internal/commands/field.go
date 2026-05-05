package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// FieldCommand returns the top-level "field" command with list and search
// subcommands.
func FieldCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "field",
		Aliases: []string{"f"},
		Short:   "Work with Jira fields",
		Example: `jira-agent field list
jira-agent field search --type custom --query "story points"`,
	}
	cmd.AddCommand(
		fieldListCommand(apiClient, w, format),
		fieldSearchCommand(apiClient, w, format),
		fieldContextCommand(apiClient, w, format, allowWrites),
		fieldOptionCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// fieldListCommand returns all system and custom fields.
// GET /rest/api/3/field
func fieldListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all fields (system and custom)",
		Example: `jira-agent field list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var fields []any
			if err := apiClient.Get(ctx, "/field", nil, &fields); err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Total = len(fields)
			meta.Returned = len(fields)

			return output.WriteSuccess(w, fields, meta, *format)
		},
	}
	return cmd
}

// fieldSearchCommand searches fields with pagination.
// GET /rest/api/3/field/search
func fieldSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search fields with filtering and pagination",
		Example: `jira-agent field search --type custom
jira-agent field search --query "story points"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			fieldType, _ := cmd.Flags().GetString("type")
			if fieldType != "" && fieldType != "custom" && fieldType != "system" {
				return fmt.Errorf("invalid field type %q: must be custom or system", fieldType)
			}

			params := buildPaginationParams(cmd, map[string]string{
				"query": "query",
				"type":  "type",
			})

			var result map[string]any
			if err := apiClient.Get(ctx, "/field/search", params, &result); err != nil {
				return err
			}

			// The search endpoint returns paginated results with a "values" key.
			values, ok := result["values"]
			if !ok {
				return apperr.NewJiraError(
					"unexpected response: missing 'values' key",
					nil,
				)
			}

			meta := extractPaginationMeta(cmd, result)
			return output.WriteSuccess(w, values, meta, *format)
		},
	}
	cmd.Flags().String("query", "", "Filter fields by name (substring match)")
	cmd.Flags().String("type", "", "Filter by field type: custom, system")
	appendPaginationFlagsWithUsage(cmd, paginationFlagUsage{
		maxResults: "Maximum number of results to return",
		startAt:    "Index of the first result to return",
	})
	return cmd
}
