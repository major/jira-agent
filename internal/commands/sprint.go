package commands

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

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
		sprintCurrentCommand(apiClient, w, format),
		sprintGetCommand(apiClient, w, format),
		sprintSummarizeCommand(apiClient, w, format),
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

func parseSprintID(sprintID string) (int64, error) {
	return parsePositiveIntID(sprintID, "sprint ID", apperr.WithNextCommand("jira-agent resolve sprint --board-id <board-id> <name>"))
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

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
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

// sprintCurrentCommand fetches the active sprint(s) for a given board.
// GET /rest/agile/1.0/board/{boardId}/sprint?state=active
func sprintCurrentCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "current",
		Short:   "Get the active sprint for a board",
		Example: `jira-agent sprint current --board-id 42`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			boardIDStr, _ := cmd.Flags().GetString("board-id")
			boardID, err := parseBoardID(boardIDStr)
			if err != nil {
				return err
			}

			params := map[string]string{"state": "active"}
			path := "/board/" + strconv.FormatInt(boardID, 10) + "/sprint"

			var result map[string]any
			if err := apiClient.AgileGet(ctx, path, params, &result); err != nil {
				return err
			}

			values, _ := result["values"].([]any)
			if len(values) == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no active sprint found for board %d", boardID),
					nil,
					apperr.WithNextCommand(fmt.Sprintf("jira-agent sprint list --board-id %d --state active", boardID)),
				)
			}

			// Return a single object when exactly one active sprint exists,
			// an array when multiple active sprints are running concurrently.
			if len(values) == 1 {
				return output.WriteResult(w, values[0], *format)
			}
			return output.WriteResult(w, values, *format)
		},
	}
	cmd.Flags().String("board-id", "", "Board ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
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

// sprintSummarizeCommand aggregates issue counts and story points for a sprint.
// It combines Agile sprint metadata with a paginated POST /rest/api/3/search/jql
// query so large sprint contents are summarized without exposing issue details.
func sprintSummarizeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize <sprint-id>",
		Short: "Summarize sprint status counts and story points",
		Example: `jira-agent sprint summarize 42
jira-agent sprint summarize 42 --story-points-field customfield_10016`,
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

			var sprintData map[string]any
			if err := apiClient.AgileGet(ctx, "/sprint/"+strconv.FormatInt(sprintID, 10), nil, &sprintData); err != nil {
				return err
			}

			spFieldID, _ := cmd.Flags().GetString("story-points-field")
			if spFieldID == "" {
				discovered, err := discoverStoryPointsField(ctx, apiClient)
				if err != nil {
					return err
				}
				spFieldID = discovered
			}

			statusCounts, spByStatus, totalIssues, totalSP, hasStoryPoints, err := summarizeSprintIssues(ctx, apiClient, sprintID, spFieldID)
			if err != nil {
				return err
			}

			var spResult any
			if spFieldID != "" && hasStoryPoints {
				spResult = map[string]any{
					"total":     totalSP,
					"by_status": spByStatus,
					"field":     spFieldID,
				}
			}

			result := map[string]any{
				"sprint": map[string]any{
					"id":            sprintID,
					"name":          sprintData["name"],
					"state":         sprintData["state"],
					"start_date":    sprintData["startDate"],
					"end_date":      sprintData["endDate"],
					"complete_date": sprintData["completeDate"],
				},
				"issues": map[string]any{
					"total":     totalIssues,
					"by_status": statusCounts,
				},
				"story_points": spResult,
			}

			return output.WriteResult(w, result, *format, CompactOptsFromCmd(cmd)...)
		},
	}
	cmd.Flags().String("story-points-field", "", "Custom field ID for story points (e.g. customfield_10016)")
	SetCommandCategory(cmd, commandCategoryRead)
	return cmd
}

func discoverStoryPointsField(ctx context.Context, apiClient *client.Ref) (string, error) {
	var result any
	if err := apiClient.Get(ctx, "/field/search", map[string]string{"query": "story points", "type": "custom"}, &result); err != nil {
		return "", err
	}

	fields := resultList(result, "values")
	if len(fields) == 0 {
		fields = resultList(result, "")
	}

	// Filter by exact name match (case-insensitive) for known story points
	// field names. The /field/search query returns loosely matching fields,
	// so blindly taking the first result can select an unrelated custom field.
	for _, f := range fields {
		field, ok := f.(map[string]any)
		if !ok {
			continue
		}
		name, _ := field["name"].(string)
		lower := strings.ToLower(name)
		if lower == "story points" || lower == "story point estimate" {
			id, _ := field["id"].(string)
			if id != "" {
				return id, nil
			}
		}
	}
	return "", nil
}

func summarizeSprintIssues(ctx context.Context, apiClient *client.Ref, sprintID int64, spFieldID string) (map[string]int, map[string]float64, int, float64, bool, error) {
	statusCounts := map[string]int{}
	spByStatus := map[string]float64{}
	totalIssues := 0
	totalSP := 0.0
	hasStoryPoints := false
	startAt := 0

	fields := []string{"status"}
	if spFieldID != "" {
		fields = append(fields, spFieldID)
	}

	for {
		body := map[string]any{
			"jql":        fmt.Sprintf("sprint = %d", sprintID),
			"fields":     fields,
			"maxResults": 100,
			"startAt":    startAt,
		}
		var result map[string]any
		if err := apiClient.Post(ctx, "/search/jql", body, &result); err != nil {
			return nil, nil, 0, 0, false, err
		}

		if total, ok := numberFromAny(result["total"]); ok {
			totalIssues = int(total)
		}

		issues := resultList(result, "issues")
		for _, issue := range issues {
			statusName, spValue, hasSP := sprintIssueSummaryValues(issue, spFieldID)
			if statusName == "" {
				continue
			}
			statusCounts[statusName]++
			if hasSP {
				hasStoryPoints = true
				totalSP += spValue
				spByStatus[statusName] += spValue
			}
		}

		startAt += len(issues)
		if len(issues) == 0 || startAt >= totalIssues {
			break
		}
	}

	return statusCounts, spByStatus, totalIssues, totalSP, hasStoryPoints, nil
}

func sprintIssueSummaryValues(issue any, spFieldID string) (string, float64, bool) {
	issueMap, ok := issue.(map[string]any)
	if !ok {
		return "", 0, false
	}
	fields, ok := issueMap["fields"].(map[string]any)
	if !ok {
		return "", 0, false
	}
	status, _ := fields["status"].(map[string]any)
	statusName, _ := status["name"].(string)
	if spFieldID == "" {
		return statusName, 0, false
	}
	spValue, hasSP := numberFromAny(fields[spFieldID])
	return statusName, spValue, hasSP
}

func resultList(result any, key string) []any {
	if key == "" {
		items, _ := result.([]any)
		return items
	}
	resultMap, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	items, _ := resultMap[key].([]any)
	return items
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
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

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
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
