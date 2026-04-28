package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// fieldOptionCommand returns the "option" subcommand group for custom field
// context options.
func fieldOptionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "option",
		Usage: "Custom field option operations (list, create, update, delete, reorder)",
		UsageText: `jira-agent field option list customfield_10001 10200
jira-agent field option create customfield_10001 10200 --options-json '[{"value":"Option A"}]'`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			optionListCommand(apiClient, w, format),
			optionCreateCommand(apiClient, w, format, allowWrites),
			optionUpdateCommand(apiClient, w, format, allowWrites),
			optionDeleteCommand(apiClient, w, format, allowWrites),
			optionReorderCommand(apiClient, w, format, allowWrites),
		},
	}
}

// optionListCommand lists options for a custom field context.
// GET /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List options for a custom field context",
		UsageText: `jira-agent field option list customfield_10001 10200`,
		ArgsUsage: "<field-id> <context-id>",
		Flags:     appendPaginationFlags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

			params := buildPaginationParams(cmd, nil)

			path := fmt.Sprintf("/field/%s/context/%s/option", fieldID, contextID)
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
}

// optionCreateCommand creates options for a custom field context.
// POST /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create options for a custom field context",
		UsageText: `jira-agent field option create customfield_10001 10200 --options-json '[{"value":"Option A"},{"value":"Option B"}]'`,
		ArgsUsage: "<field-id> <context-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "values",
				Usage:    "Comma-separated option values to create",
				Required: true,
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

			values := splitTrimmed(cmd.String("values"))
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
}

// optionUpdateCommand updates options for a custom field context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}/option
func optionUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update options for a custom field context",
		UsageText: `jira-agent field option update customfield_10001 10200 --options-json '[{"id":"10300","value":"Updated option"}]'`,
		ArgsUsage: "<field-id> <context-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "option-id",
				Usage:    "ID of the option to update",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "value",
				Usage:    "New option value",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "disabled",
				Usage: "Disable the option",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

			option := map[string]any{
				"id":    cmd.String("option-id"),
				"value": cmd.String("value"),
			}
			if cmd.IsSet("disabled") {
				option["disabled"] = cmd.Bool("disabled")
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
}

// optionDeleteCommand deletes a custom field option.
// DELETE /rest/api/3/field/{fieldId}/context/{contextId}/option/{optionId}
func optionDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a custom field option",
		UsageText: `jira-agent field option delete customfield_10001 10200 10300`,
		ArgsUsage: "<field-id> <context-id> <option-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID", "option ID")
			if err != nil {
				return err
			}
			fieldID, contextID, optionID := args[0], args[1], args[2]

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
}

// optionReorderCommand reorders custom field options within a context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}/option/move
func optionReorderCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "reorder",
		Usage:     "Reorder custom field options",
		UsageText: `jira-agent field option reorder customfield_10001 10200 --option-ids 10300,10301,10302
jira-agent field option reorder customfield_10001 10200 --option-ids 10301 --after 10300`,
		ArgsUsage: "<field-id> <context-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "option-ids",
				Usage:    "Comma-separated option IDs to move (in desired order)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "position",
				Usage:    "Position: First, Last, Before, After",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "anchor",
				Usage: "Anchor option ID (required for Before/After positions)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

			position := cmd.String("position")
			body := map[string]any{
				"customFieldOptionIds": splitTrimmed(cmd.String("option-ids")),
				"position":             position,
			}

			if position == "Before" || position == "After" {
				anchor := cmd.String("anchor")
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
}
