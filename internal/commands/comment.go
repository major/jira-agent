package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// issueCommentCommand returns the "comment" subcommand group for issues.
func issueCommentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "comment",
		Usage: "Comment operations (list, add, edit, delete)",
		UsageText: `jira-agent issue comment list PROJ-123
jira-agent issue comment add PROJ-123 --body "This is a comment"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			commentListCommand(apiClient, w, format),
			commentGetCommand(apiClient, w, format),
			commentListByIDsCommand(apiClient, w, format),
			commentAddCommand(apiClient, w, format, allowWrites),
			commentEditCommand(apiClient, w, format, allowWrites),
			commentDeleteCommand(apiClient, w, format, allowWrites),
		},
	}
}

// commentGetCommand gets a single comment by ID.
// GET /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a single comment",
		UsageText: `jira-agent issue comment get PROJ-123 10001`,
		ArgsUsage: "<issue-key> <comment-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (renderedBody, properties)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "comment ID")
			if err != nil {
				return err
			}
			key, commentID := args[0], args[1]

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams(fmt.Sprintf("/issue/%s/comment/%s", key, commentID), params)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, nil, result)
			})
		},
	}
}

// commentListByIDsCommand gets comments across issues by comment ID.
// POST /rest/api/3/comment/list
func commentListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list-by-ids",
		Usage:     "Get comments by IDs",
		UsageText: `jira-agent issue comment list-by-ids --ids 10001,10002`,
		Metadata:  requiredFlagMetadata("ids"),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "ids", Usage: "Comma-separated comment IDs (required)"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (renderedBody, properties)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			idsFlag, err := requireFlag(cmd, "ids")
			if err != nil {
				return err
			}
			body := map[string]any{"ids": splitTrimmed(idsFlag)}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams("/comment/list", params)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		},
	}
}

// commentListCommand lists comments on an issue with pagination.
// GET /rest/api/3/issue/{issueIdOrKey}/comment
func commentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List comments on an issue",
		UsageText: `jira-agent issue comment list PROJ-123
jira-agent issue comment list PROJ-123 --order-by -created`,
		ArgsUsage: "<issue-key>",
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Sort order: created, -created, +created",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (renderedBody)",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"order-by": "orderBy",
				"expand":   "expand",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/comment", params, result)
			})
		},
	}
}

// commentAddCommand adds a comment to an issue.
// POST /rest/api/3/issue/{issueIdOrKey}/comment
func commentAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a comment to an issue",
		UsageText: `jira-agent issue comment add PROJ-123 --body "This is a comment"
jira-agent issue comment add PROJ-123 --body "Internal note" --visibility-type role --visibility-value Developers`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "body",
				Aliases:  []string{"b"},
				Usage:    "Comment body (plain text or ADF JSON)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "visibility-type",
				Usage: "Visibility restriction type: group or role",
			},
			&cli.StringFlag{
				Name:  "visibility-value",
				Usage: "Visibility restriction value (group/role name)",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (renderedBody)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			body := map[string]any{
				"body": toADF(cmd.String("body")),
			}

			vt, vv, err := requireVisibilityFlags(cmd)
			if err != nil {
				return err
			}
			if vt != "" {
				body["visibility"] = map[string]any{
					"type":  vt,
					"value": vv,
				}
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams("/issue/"+key+"/comment", params)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
}

// commentEditCommand updates an existing comment.
// PUT /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "edit",
		Usage: "Edit an existing comment",
		UsageText: `jira-agent issue comment edit PROJ-123 10001 --body "Updated comment"
jira-agent issue comment edit PROJ-123 10001 --body "Updated" --notify=false`,
		ArgsUsage: "<issue-key> <comment-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "body",
				Aliases:  []string{"b"},
				Usage:    "New comment body (plain text or ADF JSON)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "visibility-type",
				Usage: "Visibility restriction type: group or role",
			},
			&cli.StringFlag{
				Name:  "visibility-value",
				Usage: "Visibility restriction value (group/role name)",
			},
			&cli.BoolFlag{
				Name:  "notify",
				Usage: "Send notification to watchers",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "override-editable-flag",
				Usage: "Override comment editable flag",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (renderedBody)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "comment ID")
			if err != nil {
				return err
			}
			key, commentID := args[0], args[1]

			body := map[string]any{
				"body": toADF(cmd.String("body")),
			}

			vt, vv, err := requireVisibilityFlags(cmd)
			if err != nil {
				return err
			}
			if vt != "" {
				body["visibility"] = map[string]any{
					"type":  vt,
					"value": vv,
				}
			}

			params := map[string]string{}
			if !cmd.Bool("notify") {
				params["notifyUsers"] = "false"
			}
			addBoolParam(cmd, params, "override-editable-flag", "overrideEditableFlag")
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams(fmt.Sprintf("/issue/%s/comment/%s", key, commentID), params)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
}

// commentDeleteCommand deletes a comment from an issue.
// DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a comment",
		UsageText: `jira-agent issue comment delete PROJ-123 10001`,
		ArgsUsage: "<issue-key> <comment-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "comment ID")
			if err != nil {
				return err
			}
			key, commentID := args[0], args[1]

			path := fmt.Sprintf("/issue/%s/comment/%s", key, commentID)
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":       key,
				"commentId": commentID,
				"deleted":   true,
			}, *format)
		}),
	}
}
