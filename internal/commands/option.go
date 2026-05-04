package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// fieldOptionCommand returns the "option" subcommand group for custom field
// context options.
func fieldOptionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "option",
		Short: "Custom field option operations (list, create, update, delete, reorder)",
		Example: `jira-agent field option list customfield_10001 10200
jira-agent field option create customfield_10001 10200 --options-json '[{"value":"Option A"}]'`,
	}
	cmd.AddCommand(
		optionListCommand(apiClient, w, format),
		optionCreateCommand(apiClient, w, format, allowWrites),
		optionUpdateCommand(apiClient, w, format, allowWrites),
		optionDeleteCommand(apiClient, w, format, allowWrites),
		optionReorderCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// optionListCommand lists options for a custom field context.
// GET /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <field-id> <context-id>",
		Short:   "List options for a custom field context",
		Example: `jira-agent field option list customfield_10001 10200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			params := buildPaginationParams(cmd, nil)

			path := fmt.Sprintf("/field/%s/context/%s/option", fieldID, contextID)
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

// optionCreateCommand creates options for a custom field context.
// POST /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <field-id> <context-id>",
		Short:   "Create options for a custom field context",
		Example: `jira-agent field option create customfield_10001 10200 --options-json '[{"value":"Option A"},{"value":"Option B"}]'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			valuesStr, _ := cmd.Flags().GetString("values")
			values := splitTrimmed(valuesStr)
			options := make([]map[string]any, 0, len(values))
			for _, v := range values {
				options = append(options, map[string]any{"value": v})
			}

			body := map[string]any{"options": options}
			path := fmt.Sprintf("/field/%s/context/%s/option", fieldID, contextID)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
	cmd.Flags().String("values", "", "Comma-separated option values to create")
	_ = cmd.MarkFlagRequired("values")
	return cmd
}

// optionUpdateCommand updates options for a custom field context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <field-id> <context-id>",
		Short:   "Update options for a custom field context",
		Example: `jira-agent field option update customfield_10001 10200 --options-json '[{"id":"10300","value":"Updated option"}]'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			optionID, _ := cmd.Flags().GetString("option-id")
			value, _ := cmd.Flags().GetString("value")
			option := map[string]any{
				"id":    optionID,
				"value": value,
			}
			if cmd.Flags().Changed("disabled") {
				disabled, _ := cmd.Flags().GetBool("disabled")
				option["disabled"] = disabled
			}

			body := map[string]any{
				"options": []map[string]any{option},
			}

			path := fmt.Sprintf("/field/%s/context/%s/option", fieldID, contextID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
	cmd.Flags().String("option-id", "", "ID of the option to update")
	_ = cmd.MarkFlagRequired("option-id")
	cmd.Flags().String("value", "", "New option value")
	_ = cmd.MarkFlagRequired("value")
	cmd.Flags().Bool("disabled", false, "Disable the option")
	return cmd
}

// optionDeleteCommand deletes a custom field option.
// DELETE /rest/api/3/field/{fieldId}/context/{contextId}/option/{optionId}
func optionDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <field-id> <context-id> <option-id>",
		Short:   "Delete a custom field option",
		Example: `jira-agent field option delete customfield_10001 10200 10300`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID", "option ID")
			if err != nil {
				return err
			}
			fieldID, contextID, optionID := posArgs[0], posArgs[1], posArgs[2]

			path := fmt.Sprintf("/field/%s/context/%s/option/%s", fieldID, contextID, optionID)
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"fieldId":   fieldID,
				"contextId": contextID,
				"optionId":  optionID,
				"deleted":   true,
			}, *format)
		}),
	}
	return cmd
}

// optionReorderCommand reorders custom field options within a context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}/option/move
func optionReorderCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reorder <field-id> <context-id>",
		Short: "Reorder custom field options",
		Example: `jira-agent field option reorder customfield_10001 10200 --option-ids 10300,10301,10302
jira-agent field option reorder customfield_10001 10200 --option-ids 10301 --after 10300`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := posArgs[0], posArgs[1]

			optionIDs, _ := cmd.Flags().GetString("option-ids")
			position, _ := cmd.Flags().GetString("position")
			body := map[string]any{
				"customFieldOptionIds": splitTrimmed(optionIDs),
				"position":             position,
			}

			if position == "Before" || position == "After" {
				anchor, _ := cmd.Flags().GetString("anchor")
				if anchor == "" {
					return apperr.NewValidationError(
						"--anchor is required for Before/After positions",
						nil,
					)
				}
				body["after"] = anchor
			}

			path := fmt.Sprintf("/field/%s/context/%s/option/move", fieldID, contextID)
			if err := apiClient.Put(ctx, path, body, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"fieldId":   fieldID,
				"contextId": contextID,
				"reordered": true,
			}, *format)
		}),
	}
	cmd.Flags().String("option-ids", "", "Comma-separated option IDs to move (in desired order)")
	_ = cmd.MarkFlagRequired("option-ids")
	cmd.Flags().String("position", "", "Position: First, Last, Before, After")
	_ = cmd.MarkFlagRequired("position")
	cmd.Flags().String("anchor", "", "Anchor option ID (required for Before/After positions)")
	return cmd
}
