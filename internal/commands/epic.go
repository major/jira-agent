package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// EpicCommand returns the top-level "epic" command with Agile epic operations.
func EpicCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "epic",
		Usage: "Agile epic operations (get, issues, move-issues, orphans, remove-issues, rank)",
		UsageText: `jira-agent epic get PROJ-50
jira-agent epic issues PROJ-50
jira-agent epic move-issues PROJ-50 --issues PROJ-1,PROJ-2`,
		DefaultCommand: "get",
		Commands: []*cli.Command{
			epicGetCommand(apiClient, w, format),
			epicIssuesCommand(apiClient, w, format),
			epicMoveIssuesCommand(apiClient, w, format, allowWrites),
			epicOrphansCommand(apiClient, w, format),
			epicRemoveIssuesCommand(apiClient, w, format, allowWrites),
			epicRankCommand(apiClient, w, format, allowWrites),
		},
	}
}

// epicGetCommand fetches a single epic by ID or key.
// GET /rest/agile/1.0/epic/{epicIdOrKey}
func epicGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get epic details by ID or key",
		UsageText: `jira-agent epic get PROJ-50
jira-agent epic get 12345`,
		ArgsUsage: "<epic-id-or-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			epicID, err := requireArg(cmd, "epic ID or key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/"+epicID, nil, result)
			})
		},
	}
}

// epicIssuesCommand lists issues belonging to an epic.
// GET /rest/agile/1.0/epic/{epicIdOrKey}/issue
func epicIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "issues",
		Usage:     "List issues in an epic",
		UsageText: `jira-agent epic issues PROJ-50
jira-agent epic issues PROJ-50 --jql "status = Open"`,
		ArgsUsage: "<epic-id-or-key>",
		Flags: appendPaginationFlags([]cli.Flag{
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
			epicID, err := requireArg(cmd, "epic ID or key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/"+epicID+"/issue", params, result)
			})
		},
	}
}

// epicMoveIssuesCommand moves issues to an epic.
// POST /rest/agile/1.0/epic/{epicIdOrKey}/issue
func epicMoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "move-issues",
		Usage:     "Move issues to an epic",
		UsageText: `jira-agent epic move-issues PROJ-50 --issues PROJ-1,PROJ-2`,
		ArgsUsage: "<epic-id-or-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "issues",
				Usage:    "Comma-separated issue keys to move (required)",
				Required: true,
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			epicID, err := requireArg(cmd, "epic ID or key")
			if err != nil {
				return err
			}
			issues := splitTrimmed(cmd.String("issues"))
			if len(issues) == 0 {
				return apperr.NewValidationError("at least one issue key is required", nil)
			}

			body := map[string]any{
				"issues": issues,
			}

			// Move issues returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, "/epic/"+epicID+"/issue", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"epic":  epicID,
				"moved": issues,
			}, *format)
		}),
	}
}

// epicOrphansCommand lists issues without an epic.
// GET /rest/agile/1.0/epic/none/issue
func epicOrphansCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "orphans",
		Usage: "List issues without an epic",
		UsageText: `jira-agent epic orphans`,
		Flags: appendPaginationFlags([]cli.Flag{
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
			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/none/issue", params, result)
			})
		},
	}
}

// epicRemoveIssuesCommand removes issues from their epic by moving them to "none".
// POST /rest/agile/1.0/epic/none/issue
func epicRemoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "remove-issues",
		Usage: "Remove issues from their epic",
		UsageText: `jira-agent epic remove-issues --issues PROJ-1,PROJ-2`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "issues",
				Usage:    "Comma-separated issue keys to remove from their epic (required)",
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

			// Remove issues returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, "/epic/none/issue", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"removed": issues,
			}, *format)
		}),
	}
}

// epicRankCommand changes the rank of an epic.
// PUT /rest/agile/1.0/epic/{epicIdOrKey}/rank
func epicRankCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "rank",
		Usage:     "Rank an epic before or after another epic",
		UsageText: `jira-agent epic rank PROJ-50 --before PROJ-49
jira-agent epic rank PROJ-50 --after PROJ-51`,
		ArgsUsage: "<epic-id-or-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "rank-before",
				Usage: "Epic ID or key to rank before",
			},
			&cli.StringFlag{
				Name:  "rank-after",
				Usage: "Epic ID or key to rank after",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			epicID, err := requireArg(cmd, "epic ID or key")
			if err != nil {
				return err
			}

			body := map[string]any{}
			before := cmd.String("rank-before")
			after := cmd.String("rank-after")
			if before == "" && after == "" {
				return apperr.NewValidationError("either --rank-before or --rank-after is required", nil)
			}
			if before != "" {
				body["rankBeforeEpic"] = before
			}
			if after != "" {
				body["rankAfterEpic"] = after
			}

			// Rank returns 204 No Content on success.
			if err := apiClient.AgilePut(ctx, "/epic/"+epicID+"/rank", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"epic":   epicID,
				"ranked": true,
			}, *format)
		}),
	}
}
