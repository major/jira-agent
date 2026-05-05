package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// issueCommentCommand returns the "comment" subcommand group for issues.
func issueCommentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Comment operations (list, add, edit, delete)",
		Example: `jira-agent issue comment list PROJ-123
jira-agent issue comment add PROJ-123 --body "This is a comment"`,
	}
	cmd.AddCommand(
		commentListCommand(apiClient, w, format),
		commentGetCommand(apiClient, w, format),
		commentListByIDsCommand(apiClient, w, format),
		commentAddCommand(apiClient, w, format, allowWrites),
		commentEditCommand(apiClient, w, format, allowWrites),
		commentDeleteCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// commentGetCommand gets a single comment by ID.
// GET /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <issue-key> <comment-id>",
		Short:   "Get a single comment",
		Example: `jira-agent issue comment get PROJ-123 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "comment ID")
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
	cmd.Flags().String("expand", "", "Comma-separated expansions (renderedBody, properties)")
	return cmd
}

// commentListByIDsCommand gets comments across issues by comment ID.
// POST /rest/api/3/comment/list
func commentListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-by-ids",
		Short:   "Get comments by IDs",
		Example: `jira-agent issue comment list-by-ids --ids 10001,10002`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			idsFlag, err := requireFlag(cmd, "ids")
			if err != nil {
				return err
			}
			ids, err := parseInt64List(idsFlag)
			if err != nil {
				return err
			}
			body := map[string]any{"ids": ids}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams("/comment/list", params)

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		},
	}
	cmd.Flags().String("ids", "", "Comma-separated comment IDs (required)")
	cmd.Flags().String("expand", "", "Comma-separated expansions (renderedBody, properties)")
	return cmd
}

// commentListCommand lists comments on an issue with pagination.
// GET /rest/api/3/issue/{issueIdOrKey}/comment
func commentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <issue-key>",
		Short: "List comments on an issue",
		Example: `jira-agent issue comment list PROJ-123
jira-agent issue comment list PROJ-123 --order-by -created`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"order-by": "orderBy",
				"expand":   "expand",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/comment", params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	cmd.Flags().String("order-by", "", "Sort field (prefix with - for descending, e.g. -created)")
	cmd.Flags().String("expand", "", "Comma-separated expansions (renderedBody, properties)")
	return cmd
}

// commentAddCommand adds a comment to an issue.
// POST /rest/api/3/issue/{issueIdOrKey}/comment
func commentAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-key>",
		Short: "Add a comment to an issue",
		Example: `jira-agent issue comment add PROJ-123 --body "This is a comment"
jira-agent issue comment add PROJ-123 --body "Internal note" --visibility-type role --visibility-value Developers`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			body := map[string]any{
				"body": toADF(mustGetString(cmd, "body")),
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
	cmd.Flags().StringP("body", "b", "", "Comment body (plain text or ADF JSON)")
	_ = cmd.MarkFlagRequired("body")
	cmd.Flags().String("visibility-type", "", "Visibility restriction type: group or role")
	cmd.Flags().String("visibility-value", "", "Visibility restriction value (group/role name)")
	cmd.Flags().String("expand", "", "Comma-separated expansions (renderedBody)")
	return cmd
}

// commentEditCommand updates an existing comment.
// PUT /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <issue-key> <comment-id>",
		Short: "Edit an existing comment",
		Example: `jira-agent issue comment edit PROJ-123 10001 --body "Updated comment"
jira-agent issue comment edit PROJ-123 10001 --body "Updated" --notify=false`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "comment ID")
			if err != nil {
				return err
			}
			key, commentID := args[0], args[1]

			body := map[string]any{
				"body": toADF(mustGetString(cmd, "body")),
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
			if !mustGetBool(cmd, "notify") {
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
	cmd.Flags().StringP("body", "b", "", "New comment body (plain text or ADF JSON)")
	_ = cmd.MarkFlagRequired("body")
	cmd.Flags().String("visibility-type", "", "Visibility restriction type: group or role")
	cmd.Flags().String("visibility-value", "", "Visibility restriction value (group/role name)")
	cmd.Flags().Bool("notify", true, "Send notification to watchers")
	cmd.Flags().Bool("override-editable-flag", false, "Override comment editable flag")
	cmd.Flags().String("expand", "", "Comma-separated expansions (renderedBody)")
	return cmd
}

// commentDeleteCommand deletes a comment from an issue.
// DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}
func commentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <issue-key> <comment-id>",
		Short:   "Delete a comment",
		Example: `jira-agent issue comment delete PROJ-123 10001`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "comment ID")
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
	return cmd
}
