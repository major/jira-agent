package commands

import (
	"context"
	"io"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// BacklogCommand returns the top-level "backlog" command with Agile backlog operations.
func BacklogCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "backlog",
		Usage: "Agile backlog operations (list, move)",
		UsageText: `jira-agent backlog list --board-id 42
jira-agent backlog move --issues PROJ-1,PROJ-2`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			backlogListCommand(apiClient, w, format),
			backlogMoveCommand(apiClient, w, format, allowWrites),
		},
	}
}

// backlogListCommand lists issues in the backlog for a board.
// GET /rest/agile/1.0/board/{boardId}/backlog
func backlogListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List backlog issues for a board",
		UsageText: `jira-agent backlog list --board-id 42`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:     "board-id",
				Usage:    "Board ID (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "jql",
				Usage: "JQL filter to apply",
			},
			&cli.StringFlag{
				Name:  "fields",
				Usage: "Comma-separated list of fields to return",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			boardID, err := parseBoardID(cmd.String("board-id"))
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			path := "/board/" + strconv.FormatInt(boardID, 10) + "/backlog"

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			})
		},
	}
}

// backlogMoveCommand moves issues to the backlog.
// POST /rest/agile/1.0/backlog/issue
func backlogMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "move",
		Usage: "Move issues to the backlog",
		UsageText: `jira-agent backlog move --issues PROJ-1,PROJ-2`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "issues",
				Usage:    "Comma-separated issue keys to move to backlog (required)",
				Required: true,
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			issues := splitTrimmed(cmd.String("issues"))
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
}
