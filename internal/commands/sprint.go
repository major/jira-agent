package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// SprintCommand returns the top-level "sprint" command with Agile sprint operations.
func SprintCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprint",
		Short: "Agile sprint operations (list, get, create, update, delete, issues, move-issues, swap, property)",
		Example: `jira-agent sprint list --board-id 42
jira-agent sprint get 100
jira-agent sprint issues 100
jira-agent sprint swap 100 101`,
	}
	cmd.AddCommand(
		sprintListCommand(apiClient, w, format),
		sprintGetCommand(apiClient, w, format),
		sprintCreateCommand(apiClient, w, format, allowWrites),
		sprintUpdateCommand(apiClient, w, format, allowWrites),
		sprintDeleteCommand(apiClient, w, format, allowWrites),
		sprintIssuesCommand(apiClient, w, format),
		sprintMoveIssuesCommand(apiClient, w, format, allowWrites),
		sprintSwapCommand(apiClient, w, format, allowWrites),
		sprintPropertyCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
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
func sprintListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sprints for a board",
		Example: `jira-agent sprint list --board-id 42
jira-agent sprint list --board-id 42 --state active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardIDStr, _ := cmd.Flags().GetString("board-id")
			boardID, err := parseBoardID(boardIDStr)
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
	cmd.Flags().String("board-id", "", "Board ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	cmd.Flags().String("state", "", "Filter by state: future, active, closed (comma-separated)")
	appendPaginationFlags(cmd)
	return cmd
}

// sprintGetCommand fetches a single sprint by ID.
// GET /rest/agile/1.0/sprint/{sprintId}
func sprintGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Short:   "Get sprint details by ID",
		Example: `jira-agent sprint get 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg, err := requireArg(args, "sprint ID")
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
func sprintCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new sprint",
		Example: `jira-agent sprint create --board-id 42 --name "Sprint 5" --start-date 2025-01-01 --end-date 2025-01-14`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardIDStr, _ := cmd.Flags().GetString("board-id")
			boardID, err := parseBoardID(boardIDStr)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			body := map[string]any{
				"name":          name,
				"originBoardId": boardID,
			}
			if v, _ := cmd.Flags().GetString("goal"); v != "" {
				body["goal"] = v
			}
			if v, _ := cmd.Flags().GetString("start-date"); v != "" {
				body["startDate"] = v
			}
			if v, _ := cmd.Flags().GetString("end-date"); v != "" {
				body["endDate"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgilePost(ctx, "/sprint", body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Sprint name (required)")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("board-id", "", "Board ID to create the sprint on (required)")
	_ = cmd.MarkFlagRequired("board-id")
	cmd.Flags().String("goal", "", "Sprint goal")
	cmd.Flags().String("start-date", "", "Start date (ISO 8601, e.g. 2025-01-15T09:00:00.000Z)")
	cmd.Flags().String("end-date", "", "End date (ISO 8601, e.g. 2025-01-29T17:00:00.000Z)")
	return cmd
}

// sprintUpdateCommand updates an existing sprint.
// PUT /rest/agile/1.0/sprint/{sprintId}
func sprintUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a sprint",
		Example: `jira-agent sprint update 100 --name "Sprint 5 (revised)"
jira-agent sprint update 100 --state closed`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg, err := requireArg(args, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				body["name"] = v
			}
			if v, _ := cmd.Flags().GetString("goal"); v != "" {
				body["goal"] = v
			}
			if v, _ := cmd.Flags().GetString("state"); v != "" {
				body["state"] = v
			}
			if v, _ := cmd.Flags().GetString("start-date"); v != "" {
				body["startDate"] = v
			}
			if v, _ := cmd.Flags().GetString("end-date"); v != "" {
				body["endDate"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgilePut(ctx, "/sprint/"+strconv.FormatInt(sprintID, 10), body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Sprint name")
	cmd.Flags().String("goal", "", "Sprint goal")
	cmd.Flags().String("state", "", "Sprint state: future, active, closed")
	cmd.Flags().String("start-date", "", "Start date (ISO 8601)")
	cmd.Flags().String("end-date", "", "End date (ISO 8601)")
	return cmd
}

// sprintDeleteCommand deletes a sprint.
// DELETE /rest/agile/1.0/sprint/{sprintId}
func sprintDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "delete",
		Short:   "Delete a sprint",
		Example: `jira-agent sprint delete 100`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg, err := requireArg(args, "sprint ID")
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
func sprintIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List issues in a sprint",
		Example: `jira-agent sprint issues 100
jira-agent sprint issues 100 --jql "status = Done"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg, err := requireArg(args, "sprint ID")
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
	cmd.Flags().String("jql", "", "JQL filter to apply within the sprint")
	cmd.Flags().String("fields", "", "Comma-separated list of fields to return")
	cmd.Flags().String("expand", "", "Comma-separated list of expansions")
	appendPaginationFlags(cmd)
	return cmd
}

// sprintMoveIssuesCommand moves issues to a sprint.
// POST /rest/agile/1.0/sprint/{sprintId}/issue
func sprintMoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "move-issues",
		Short:   "Move issues to a sprint",
		Example: `jira-agent sprint move-issues 100 --issues PROJ-1,PROJ-2`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg, err := requireArg(args, "sprint ID")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(arg)
			if err != nil {
				return err
			}
			issuesStr, _ := cmd.Flags().GetString("issues")
			issues := splitTrimmed(issuesStr)
			if len(issues) == 0 {
				return apperr.NewValidationError("at least one issue key is required", nil)
			}

			body := map[string]any{
				"issues": issues,
			}
			if v, _ := cmd.Flags().GetString("rank-before"); v != "" {
				body["rankBeforeIssue"] = v
			}
			if v, _ := cmd.Flags().GetString("rank-after"); v != "" {
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
	cmd.Flags().String("issues", "", "Comma-separated issue keys to move (required)")
	_ = cmd.MarkFlagRequired("issues")
	cmd.Flags().String("rank-before", "", "Issue key or ID to rank moved issues before")
	cmd.Flags().String("rank-after", "", "Issue key or ID to rank moved issues after")
	return cmd
}

// sprintSwapCommand swaps the backlog order of two sprints.
// POST /rest/agile/1.0/sprint/{sprintId}/swap
func sprintSwapCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "swap",
		Short:   "Swap two sprint positions",
		Example: `jira-agent sprint swap 100 101`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "sprint ID", "sprint to swap with")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(posArgs[0])
			if err != nil {
				return err
			}
			sprintToSwapWith, err := parseSprintID(posArgs[1])
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
