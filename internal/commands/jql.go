package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// JQLCommand returns the "jql" parent command for JQL autocomplete and
// validation helpers.
func JQLCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jql",
		Short: "JQL autocomplete and validation helpers",
		Example: `jira-agent jql fields
jira-agent jql suggest --field-name status
jira-agent jql validate --query "project = PROJ AND status = Open"`,
	}
	cmd.AddCommand(
		jqlFieldsCommand(apiClient, w, format),
		jqlSuggestCommand(apiClient, w, format),
		jqlValidateCommand(apiClient, w, format),
	)
	return cmd
}

// GET /rest/api/3/jql/autocompletedata
func jqlFieldsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fields",
		Short:   "List JQL field reference data (field names, operators, functions, reserved words)",
		Example: `jira-agent jql fields`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/jql/autocompletedata", nil, result)
			})
		},
	}
	return cmd
}

// GET /rest/api/3/jql/autocompletedata/suggestions
func jqlSuggestCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Get JQL autocomplete suggestions for a field value",
		Example: `jira-agent jql suggest --field-name status
jira-agent jql suggest --field-name priority --field-value Hi`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fieldName, _ := cmd.Flags().GetString("field-name")
			params := map[string]string{
				"fieldName": fieldName,
			}
			addOptionalParams(cmd, params, map[string]string{
				"field-value":     "fieldValue",
				"predicate-name":  "predicateName",
				"predicate-value": "predicateValue",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/jql/autocompletedata/suggestions", params, result)
			})
		},
	}
	cmd.Flags().String("field-name", "", "field to get suggestions for (e.g. reporter, assignee, project)")
	_ = cmd.MarkFlagRequired("field-name")
	cmd.Flags().String("field-value", "", "partial value to filter suggestions")
	cmd.Flags().String("predicate-name", "", "CHANGED operator predicate (by, from, to)")
	cmd.Flags().String("predicate-value", "", "partial predicate value to filter suggestions")
	return cmd
}

// POST /rest/api/3/jql/parse
func jqlValidateCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Parse and validate one or more JQL queries",
		Example: `jira-agent jql validate --query "project = PROJ AND status = Open"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			queries, _ := cmd.Flags().GetStringSlice("query")
			validation, _ := cmd.Flags().GetString("validation")
			body := map[string]any{
				"queries": queries,
			}
			path := appendQueryParams("/jql/parse", map[string]string{
				"validation": validation,
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		},
	}
	cmd.Flags().StringSliceP("query", "q", nil, "JQL query to validate (repeatable)")
	_ = cmd.MarkFlagRequired("query")
	cmd.Flags().String("validation", "strict", "validation mode: strict, warn, or none")
	return cmd
}
