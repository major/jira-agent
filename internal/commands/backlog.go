package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// BacklogCommand returns the top-level "backlog" command with Agile backlog operations.
func BacklogCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlog",
		Short: "Agile backlog operations (list, move)",
		Example: `jira-agent backlog list --board-id 42
jira-agent backlog move --issues PROJ-1,PROJ-2`,
	}
	cmd.AddCommand(
		backlogListCommand(apiClient, w, format),
		backlogMoveCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// backlogListCommand lists issues in the backlog for a board.
// GET /rest/agile/1.0/board/{boardId}/backlog
func backlogListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List backlog issues for a board",
		Example: `jira-agent backlog list --board-id 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardIDStr, _ := cmd.Flags().GetString("board-id")
			boardID, err := parseBoardID(boardIDStr)
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			path := "/board/" + strconv.FormatInt(boardID, 10) + "/backlog"

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			})
		},
	}
	cmd.Flags().String("board-id", "", "Board ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	cmd.Flags().String("jql", "", "JQL filter to apply")
	cmd.Flags().String("fields", "", "Comma-separated list of fields to return")
	appendPaginationFlags(cmd)
	return cmd
}

// backlogMoveCommand moves issues to the backlog.
// POST /rest/agile/1.0/backlog/issue
func backlogMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "move",
		Short:   "Move issues to the backlog",
		Example: `jira-agent backlog move --issues PROJ-1,PROJ-2`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issuesStr, _ := cmd.Flags().GetString("issues")
			issues := splitTrimmed(issuesStr)
			if len(issues) == 0 {
				return apperr.NewValidationError("at least one issue key is required", nil)
			}

			body := map[string]any{
				"issues": issues,
			}

			// Move to backlog returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, "/backlog/issue", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"moved": issues,
			}, *format)
		}),
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys to move to backlog (required)")
	_ = cmd.MarkFlagRequired("issues")
	return cmd
}
