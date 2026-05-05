package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// fieldContextCommand returns the "context" subcommand group for custom fields.
func fieldContextCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Custom field context operations (list, create, update, delete)",
		Example: `jira-agent field context list customfield_10001
jira-agent field context create customfield_10001 --name "Global context"`,
	}
	cmd.AddCommand(
		contextListCommand(apiClient, w, format),
		contextCreateCommand(apiClient, w, format, allowWrites),
		contextUpdateCommand(apiClient, w, format, allowWrites),
		contextDeleteCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// contextListCommand lists contexts for a custom field.
// GET /rest/api/3/field/{fieldId}/context
func contextListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <field-id>",
		Short:   "List contexts for a custom field",
		Example: `jira-agent field context list customfield_10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fieldID, err := requireArg(args, "field ID")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			path := fmt.Sprintf("/field/%s/context", fieldID)
			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

// contextCreateCommand creates a new context for a custom field.
// POST /rest/api/3/field/{fieldId}/context
func contextCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <field-id>",
		Short:   "Create a context for a custom field",
		Example: `jira-agent field context create customfield_10001 --name "Global context"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fieldID, err := requireArg(args, "field ID")
			if err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("name")
			body := map[string]any{
				"name": name,
			}
			if d, _ := cmd.Flags().GetString("description"); d != "" {
				body["description"] = d
			}
			if p, _ := cmd.Flags().GetString("projects"); p != "" {
				body["projectIds"] = splitTrimmed(p)
			}
			if it, _ := cmd.Flags().GetString("issue-types"); it != "" {
				body["issueTypeIds"] = splitTrimmed(it)
			}

			path := fmt.Sprintf("/field/%s/context", fieldID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Context name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("description", "", "Context description")
	cmd.Flags().String("projects", "", "Comma-separated project IDs to associate")
	cmd.Flags().String("issue-types", "", "Comma-separated issue type IDs to associate")
	return cmd
}

// contextUpdateCommand updates an existing custom field context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}
func contextUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <field-id> <context-id>",
		Short:   "Update a custom field context",
		Example: `jira-agent field context update customfield_10001 10200 --name "Updated context"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			body := map[string]any{}
			if n, _ := cmd.Flags().GetString("name"); n != "" {
				body["name"] = n
			}
			if d, _ := cmd.Flags().GetString("description"); d != "" {
				body["description"] = d
			}

			if len(body) == 0 {
				return apperr.NewValidationError(
					"at least one of --name or --description is required",
					nil,
				)
			}

			path := fmt.Sprintf("/field/%s/context/%s", fieldID, contextID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "New context name")
	cmd.Flags().String("description", "", "New context description")
	return cmd
}

// contextDeleteCommand deletes a custom field context.
// DELETE /rest/api/3/field/{fieldId}/context/{contextId}
func contextDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <field-id> <context-id>",
		Short:   "Delete a custom field context",
		Example: `jira-agent field context delete customfield_10001 10200`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			path := fmt.Sprintf("/field/%s/context/%s", fieldID, contextID)
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"fieldId":   fieldID,
				"contextId": contextID,
				"deleted":   true,
			}, *format)
		}),
	}
	return cmd
}
