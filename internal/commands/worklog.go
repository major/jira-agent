package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueWorklogCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worklog",
		Short: "Worklog operations (list, add, edit, delete)",
		Example: `jira-agent issue worklog list PROJ-123
jira-agent issue worklog add PROJ-123 --time-spent 2h`,
	}
	cmd.AddCommand(
		worklogListCommand(apiClient, w, format),
		worklogGetCommand(apiClient, w, format),
		worklogUpdatedCommand(apiClient, w, format),
		worklogDeletedCommand(apiClient, w, format),
		worklogListByIDsCommand(apiClient, w, format),
		worklogAddCommand(apiClient, w, format, allowWrites),
		worklogEditCommand(apiClient, w, format, allowWrites),
		worklogDeleteCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func worklogGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <issue-key> <worklog-id>",
		Short:   "Get a single worklog",
		Example: `jira-agent issue worklog get PROJ-123 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "worklog ID")
			if err != nil {
				return err
			}
			key, worklogID := args[0], args[1]
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			path := appendQueryParams(fmt.Sprintf("/issue/%s/worklog/%s", key, worklogID), params)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, nil, result)
			})
		},
	}
	cmd.Flags().String("expand", "", "Comma-separated expansions (properties)")
	return cmd
}

func worklogUpdatedCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "updated",
		Short:   "List updated worklogs",
		Example: `jira-agent issue worklog updated --since 1700000000000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"since": "since", "expand": "expand"})
			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/worklog/updated", params, result)
			})
		},
	}
	cmd.Flags().String("since", "", "Unix timestamp in milliseconds")
	cmd.Flags().String("expand", "", "Comma-separated expansions (properties)")
	return cmd
}

func worklogDeletedCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deleted",
		Short:   "List deleted worklogs",
		Example: `jira-agent issue worklog deleted --since 1700000000000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"since": "since"})
			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/worklog/deleted", params, result)
			})
		},
	}
	cmd.Flags().String("since", "", "Unix timestamp in milliseconds")
	return cmd
}

func worklogListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-by-ids",
		Short:   "Get worklogs by IDs",
		Example: `jira-agent issue worklog list-by-ids --ids 12345,67890`,
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
			path := appendQueryParams("/worklog/list", params)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		},
	}
	cmd.Flags().String("ids", "", "Comma-separated worklog IDs (required)")
	cmd.Flags().String("expand", "", "Comma-separated expansions (properties)")
	return cmd
}

func worklogListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List worklogs on an issue",
		Example: `jira-agent issue worklog list PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"started-after":  "startedAfter",
				"started-before": "startedBefore",
				"expand":         "expand",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/worklog", params, result)
			})
		},
	}
	cmd.Flags().Int("start-at", 0, "Pagination offset")
	cmd.Flags().Int("max-results", 20, "Page size")
	cmd.Flags().String("started-after", "", "Only worklogs started after this Unix timestamp in milliseconds")
	cmd.Flags().String("started-before", "", "Only worklogs started before this Unix timestamp in milliseconds")
	cmd.Flags().String("expand", "", "Comma-separated expansions (properties)")
	return cmd
}

func worklogAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-key>",
		Short: "Add a worklog to an issue",
		Example: `jira-agent issue worklog add PROJ-123 --time-spent 2h
jira-agent issue worklog add PROJ-123 --time-spent 30m --started "2025-01-15T09:00:00.000+0000"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			body, err := buildWorklogBody(cmd, true)
			if err != nil {
				return err
			}
			path := appendQueryParams("/issue/"+key+"/worklog", worklogMutationParams(cmd, false))

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
	appendWorklogMutationFlags(cmd)
	return cmd
}

func worklogEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit <issue-key> <worklog-id>",
		Short:   "Edit an existing worklog",
		Example: `jira-agent issue worklog edit PROJ-123 12345 --time-spent 3h`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "worklog ID")
			if err != nil {
				return err
			}
			key, worklogID := args[0], args[1]

			body, err := buildWorklogBody(cmd, false)
			if err != nil {
				return err
			}
			path := appendQueryParams(
				fmt.Sprintf("/issue/%s/worklog/%s", key, worklogID),
				worklogMutationParams(cmd, false),
			)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
	appendWorklogMutationFlags(cmd)
	return cmd
}

func worklogDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <issue-key> <worklog-id>",
		Short:   "Delete a worklog",
		Example: `jira-agent issue worklog delete PROJ-123 12345`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "worklog ID")
			if err != nil {
				return err
			}
			key, worklogID := args[0], args[1]

			path := appendQueryParams(
				fmt.Sprintf("/issue/%s/worklog/%s", key, worklogID),
				worklogMutationParams(cmd, true),
			)
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":       key,
				"worklogId": worklogID,
				"deleted":   true,
			}, *format)
		}),
	}
	cmd.Flags().Bool("notify", true, "Send notification to watchers")
	cmd.Flags().String("adjust-estimate", "", "Estimate adjustment: auto, leave, manual, new")
	cmd.Flags().String("new-estimate", "", "New remaining estimate when adjust-estimate is new")
	cmd.Flags().String("increase-by", "", "Increase remaining estimate when adjust-estimate is manual")
	cmd.Flags().Bool("override-editable-flag", false, "Override worklog editable flag")
	return cmd
}

func appendWorklogMutationFlags(cmd *cobra.Command) {
	cmd.Flags().String("started", "", "Worklog start timestamp, e.g. 2026-04-27T10:00:00.000-0500")
	cmd.Flags().String("time-spent", "", "Time spent, such as 1h 30m")
	cmd.Flags().Int("time-spent-seconds", 0, "Time spent in seconds")
	cmd.Flags().String("comment", "", "Worklog comment (plain text or ADF JSON)")
	cmd.Flags().String("visibility-type", "", "Visibility restriction type: group or role")
	cmd.Flags().String("visibility-value", "", "Visibility restriction value (group/role name)")
	cmd.Flags().String("properties-json", "", "JSON array of worklog properties")
	cmd.Flags().String("expand", "", "Comma-separated expansions (properties)")
	cmd.Flags().Bool("notify", true, "Send notification to watchers")
	cmd.Flags().String("adjust-estimate", "", "Estimate adjustment: auto, leave, manual, new")
	cmd.Flags().String("new-estimate", "", "New remaining estimate when adjust-estimate is new")
	cmd.Flags().String("reduce-by", "", "Amount to reduce remaining estimate when adjust-estimate is manual")
	cmd.Flags().Bool("override-editable-flag", false, "Override worklog editable flag")
}

func buildWorklogBody(cmd *cobra.Command, requireCoreFields bool) (map[string]any, error) {
	body := map[string]any{}
	if started := mustGetString(cmd, "started"); started != "" {
		body["started"] = started
	}
	if timeSpent := mustGetString(cmd, "time-spent"); timeSpent != "" {
		body["timeSpent"] = timeSpent
	}
	if seconds := mustGetInt(cmd, "time-spent-seconds"); seconds > 0 {
		body["timeSpentSeconds"] = seconds
	}
	if comment := mustGetString(cmd, "comment"); comment != "" {
		body["comment"] = toADF(comment)
	}
	vt, vv, err := requireVisibilityFlags(cmd)
	if err != nil {
		return nil, err
	}
	if vt != "" {
		body["visibility"] = map[string]any{"type": vt, "value": vv}
	}
	if propertiesJSON := mustGetString(cmd, "properties-json"); propertiesJSON != "" {
		properties, err := parseWorklogProperties(propertiesJSON)
		if err != nil {
			return nil, err
		}
		body["properties"] = properties
	}

	if requireCoreFields {
		if body["started"] == nil {
			return nil, apperr.NewValidationError("--started is required", nil)
		}
		if body["timeSpent"] == nil && body["timeSpentSeconds"] == nil {
			return nil, apperr.NewValidationError("--time-spent or --time-spent-seconds is required", nil)
		}
	}
	if len(body) == 0 {
		return nil, apperr.NewValidationError("at least one worklog field is required", nil)
	}

	return body, nil
}

func parseWorklogProperties(jsonStr string) ([]any, error) {
	var properties []any
	if err := json.Unmarshal([]byte(jsonStr), &properties); err != nil {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("invalid --properties-json: %v", err),
			err,
		)
	}
	return properties, nil
}

func worklogMutationParams(cmd *cobra.Command, includeIncreaseBy bool) map[string]string {
	params := map[string]string{}
	if !mustGetBool(cmd, "notify") {
		params["notifyUsers"] = "false"
	}
	addOptionalParams(cmd, params, map[string]string{
		"adjust-estimate": "adjustEstimate",
		"new-estimate":    "newEstimate",
		"reduce-by":       "reduceBy",
		"expand":          "expand",
	})
	if includeIncreaseBy {
		addOptionalParams(cmd, params, map[string]string{"increase-by": "increaseBy"})
	}
	addBoolParam(cmd, params, "override-editable-flag", "overrideEditableFlag")
	return params
}
