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

// fieldContextCommand returns the "context" subcommand group for custom fields.
func fieldContextCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "context",
		Usage: "Custom field context operations (list, create, update, delete)",
		UsageText: `jira-agent field context list customfield_10001
jira-agent field context create customfield_10001 --name "Global context"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			contextListCommand(apiClient, w, format),
			contextCreateCommand(apiClient, w, format, allowWrites),
			contextUpdateCommand(apiClient, w, format, allowWrites),
			contextDeleteCommand(apiClient, w, format, allowWrites),
		},
	}
}

// contextListCommand lists contexts for a custom field.
// GET /rest/api/3/field/{fieldId}/context
func contextListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List contexts for a custom field",
		UsageText: `jira-agent field context list customfield_10001`,
		ArgsUsage: "<field-id>",
		Flags:     appendPaginationFlags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fieldID, err := requireArg(cmd, "field ID")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			path := fmt.Sprintf("/field/%s/context", fieldID)
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
}

// contextCreateCommand creates a new context for a custom field.
// POST /rest/api/3/field/{fieldId}/context
func contextCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a context for a custom field",
		UsageText: `jira-agent field context create customfield_10001 --name "Global context"`,
		ArgsUsage: "<field-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Usage:    "Context name",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "Context description",
			},
			&cli.StringFlag{
				Name:  "projects",
				Usage: "Comma-separated project IDs to associate",
			},
			&cli.StringFlag{
				Name:  "issue-types",
				Usage: "Comma-separated issue type IDs to associate",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			fieldID, err := requireArg(cmd, "field ID")
			if err != nil {
				return err
			}

			body := map[string]any{
				"name": cmd.String("name"),
			}
			if d := cmd.String("description"); d != "" {
				body["description"] = d
			}
			if p := cmd.String("projects"); p != "" {
				body["projectIds"] = splitTrimmed(p)
			}
			if it := cmd.String("issue-types"); it != "" {
				body["issueTypeIds"] = splitTrimmed(it)
			}

			path := fmt.Sprintf("/field/%s/context", fieldID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
}

// contextUpdateCommand updates an existing custom field context.
// PUT /rest/api/3/field/{fieldId}/context/{contextId}
func contextUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a custom field context",
		UsageText: `jira-agent field context update customfield_10001 10200 --name "Updated context"`,
		ArgsUsage: "<field-id> <context-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "name",
				Usage: "New context name",
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "New context description",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

			body := map[string]any{}
			if n := cmd.String("name"); n != "" {
				body["name"] = n
			}
			if d := cmd.String("description"); d != "" {
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
}

// contextDeleteCommand deletes a custom field context.
// DELETE /rest/api/3/field/{fieldId}/context/{contextId}
func contextDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a custom field context",
		UsageText: `jira-agent field context delete customfield_10001 10200`,
		ArgsUsage: "<field-id> <context-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "field ID", "context ID")
			if err != nil {
				return err
			}
			fieldID, contextID := args[0], args[1]

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
}
