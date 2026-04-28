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

// SprintCommand returns the top-level "sprint" command with Agile sprint operations.
func SprintCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "sprint",
		Usage: "Agile sprint operations (list, get, create, update, delete, issues, move-issues, swap, property)",
		UsageText: `jira-agent sprint list --board-id 42
jira-agent sprint get 100
jira-agent sprint issues 100
jira-agent sprint swap 100 101`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			sprintListCommand(apiClient, w, format),
			sprintGetCommand(apiClient, w, format),
			sprintCreateCommand(apiClient, w, format, allowWrites),
			sprintUpdateCommand(apiClient, w, format, allowWrites),
			sprintDeleteCommand(apiClient, w, format, allowWrites),
			sprintIssuesCommand(apiClient, w, format),
			sprintMoveIssuesCommand(apiClient, w, format, allowWrites),
			sprintSwapCommand(apiClient, w, format, allowWrites),
			sprintPropertyCommand(apiClient, w, format, allowWrites),
		},
	}
}

func parseSprintID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NewValidationError("sprint ID must be a positive integer", err)
	}
	return id, nil
}

// sprintListCommand lists sprints for a given board.
// GET /rest/agile/1.0/board/{boardId}/sprint
func sprintListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List sprints for a board",
		UsageText: `jira-agent sprint list --board-id 42
jira-agent sprint list --board-id 42 --state active`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:     "board-id",
				Usage:    "Board ID (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "state",
				Usage: "Filter by state: future, active, closed (comma-separated)",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			boardID, err := parseBoardID(cmd.String("board-id"))
			if err != nil {
				return err
			}
			params := buildPaginationParams(cmd, map[string]string{
				"state": "state",
			})
			path := "/board/" + strconv.FormatInt(boardID, 10) + "/sprint"

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			})
		},
	}
}

// sprintGetCommand fetches a single sprint by ID.
// GET /rest/agile/1.0/sprint/{sprintId}
func sprintGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get sprint details by ID",
		UsageText: `jira-agent sprint get 100`,
		ArgsUsage: "<sprint-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			arg, err := requireArg(cmd, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/sprint/"+strconv.FormatInt(sprintID, 10), nil, result)
			})
		},
	}
}

// sprintCreateCommand creates a new sprint.
// POST /rest/agile/1.0/sprint
func sprintCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a new sprint",
		UsageText: `jira-agent sprint create --board-id 42 --name "Sprint 5" --start-date 2025-01-01 --end-date 2025-01-14`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Usage:    "Sprint name (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "board-id",
				Usage:    "Board ID to create the sprint on (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "goal",
				Usage: "Sprint goal",
			},
			&cli.StringFlag{
				Name:  "start-date",
				Usage: "Start date (ISO 8601, e.g. 2025-01-15T09:00:00.000Z)",
			},
			&cli.StringFlag{
				Name:  "end-date",
				Usage: "End date (ISO 8601, e.g. 2025-01-29T17:00:00.000Z)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			boardID, err := parseBoardID(cmd.String("board-id"))
			if err != nil {
				return err
			}
			body := map[string]any{
				"name":          cmd.String("name"),
				"originBoardId": boardID,
			}
			if v := cmd.String("goal"); v != "" {
				body["goal"] = v
			}
			if v := cmd.String("start-date"); v != "" {
				body["startDate"] = v
			}
			if v := cmd.String("end-date"); v != "" {
				body["endDate"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgilePost(ctx, "/sprint", body, result)
			})
		}),
	}
}

// sprintUpdateCommand updates an existing sprint.
// PUT /rest/agile/1.0/sprint/{sprintId}
func sprintUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update a sprint",
		UsageText: `jira-agent sprint update 100 --name "Sprint 5 (revised)"
jira-agent sprint update 100 --state closed`,
		ArgsUsage: "<sprint-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Sprint name"},
			&cli.StringFlag{Name: "goal", Usage: "Sprint goal"},
			&cli.StringFlag{Name: "state", Usage: "Sprint state: future, active, closed"},
			&cli.StringFlag{Name: "start-date", Usage: "Start date (ISO 8601)"},
			&cli.StringFlag{Name: "end-date", Usage: "End date (ISO 8601)"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			arg, err := requireArg(cmd, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v := cmd.String("name"); v != "" {
				body["name"] = v
			}
			if v := cmd.String("goal"); v != "" {
				body["goal"] = v
			}
			if v := cmd.String("state"); v != "" {
				body["state"] = v
			}
			if v := cmd.String("start-date"); v != "" {
				body["startDate"] = v
			}
			if v := cmd.String("end-date"); v != "" {
				body["endDate"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgilePut(ctx, "/sprint/"+strconv.FormatInt(sprintID, 10), body, result)
			})
		}),
	}
}

// sprintDeleteCommand deletes a sprint.
// DELETE /rest/agile/1.0/sprint/{sprintId}
func sprintDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a sprint",
		UsageText: `jira-agent sprint delete 100`,
		ArgsUsage: "<sprint-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			arg, err := requireArg(cmd, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}

			if err := apiClient.AgileDelete(ctx, "/sprint/"+strconv.FormatInt(sprintID, 10), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"id":      sprintID,
				"deleted": true,
			}, *format)
		}),
	}
}

// sprintIssuesCommand lists issues in a sprint.
// GET /rest/agile/1.0/sprint/{sprintId}/issue
func sprintIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "issues",
		Usage: "List issues in a sprint",
		UsageText: `jira-agent sprint issues 100
jira-agent sprint issues 100 --jql "status = Done"`,
		ArgsUsage: "<sprint-id>",
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:  "jql",
				Usage: "JQL filter to apply within the sprint",
			},
			&cli.StringFlag{
				Name:  "fields",
				Usage: "Comma-separated list of fields to return",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated list of expansions",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			arg, err := requireArg(cmd, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}
			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
				"expand": "expand",
			})
			path := "/sprint/" + strconv.FormatInt(sprintID, 10) + "/issue"

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, path, params, result)
			})
		},
	}
}

// sprintMoveIssuesCommand moves issues to a sprint.
// POST /rest/agile/1.0/sprint/{sprintId}/issue
func sprintMoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "move-issues",
		Usage:     "Move issues to a sprint",
		UsageText: `jira-agent sprint move-issues 100 --issues PROJ-1,PROJ-2`,
		ArgsUsage: "<sprint-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "issues",
				Usage:    "Comma-separated issue keys to move (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "rank-before",
				Usage: "Issue key or ID to rank moved issues before",
			},
			&cli.StringFlag{
				Name:  "rank-after",
				Usage: "Issue key or ID to rank moved issues after",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			arg, err := requireArg(cmd, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
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
			if v := cmd.String("rank-before"); v != "" {
				body["rankBeforeIssue"] = v
			}
			if v := cmd.String("rank-after"); v != "" {
				body["rankAfterIssue"] = v
			}

			path := "/sprint/" + strconv.FormatInt(sprintID, 10) + "/issue"
			// Move issues returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, path, body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"sprintId": sprintID,
				"moved":    issues,
			}, *format)
		}),
	}
}

// sprintSwapCommand swaps the backlog order of two sprints.
// POST /rest/agile/1.0/sprint/{sprintId}/swap
func sprintSwapCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "swap",
		Usage:     "Swap two sprint positions",
		UsageText: `jira-agent sprint swap 100 101`,
		ArgsUsage: "<sprint-id> <sprint-to-swap-with>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "sprint ID", "sprint to swap with")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(args[0])
			if err != nil {
				return err
			}
			sprintToSwapWith, err := parseSprintID(args[1])
			if err != nil {
				return err
			}

			body := map[string]any{
				"sprintToSwapWith": sprintToSwapWith,
			}
			path := "/sprint/" + strconv.FormatInt(sprintID, 10) + "/swap"
			if err := apiClient.AgilePost(ctx, path, body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"sprintId":         sprintID,
				"sprintToSwapWith": sprintToSwapWith,
				"swapped":          true,
			}, *format)
		}),
	}
}
