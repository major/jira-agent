package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueWorklogCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "worklog",
		Usage: "Worklog operations (list, add, edit, delete)",
		UsageText: `jira-agent issue worklog list PROJ-123
jira-agent issue worklog add PROJ-123 --time-spent 2h`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			worklogListCommand(apiClient, w, format),
			worklogGetCommand(apiClient, w, format),
			worklogUpdatedCommand(apiClient, w, format),
			worklogDeletedCommand(apiClient, w, format),
			worklogListByIDsCommand(apiClient, w, format),
			worklogAddCommand(apiClient, w, format, allowWrites),
			worklogEditCommand(apiClient, w, format, allowWrites),
			worklogDeleteCommand(apiClient, w, format, allowWrites),
		},
	}
}

func worklogGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a single worklog",
		UsageText: `jira-agent issue worklog get PROJ-123 12345`,
		ArgsUsage: "<issue-key> <worklog-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (properties)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "worklog ID")
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
}

func worklogUpdatedCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "updated",
		Usage:     "List updated worklogs",
		UsageText: `jira-agent issue worklog updated --since 1700000000000`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "since", Usage: "Unix timestamp in milliseconds"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (properties)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"since": "since", "expand": "expand"})
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/worklog/updated", params, result)
			})
		},
	}
}

func worklogDeletedCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "deleted",
		Usage:     "List deleted worklogs",
		UsageText: `jira-agent issue worklog deleted --since 1700000000000`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "since", Usage: "Unix timestamp in milliseconds"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"since": "since"})
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/worklog/deleted", params, result)
			})
		},
	}
}

func worklogListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list-by-ids",
		Usage:     "Get worklogs by IDs",
		UsageText: `jira-agent issue worklog list-by-ids --ids 12345,67890`,
		Metadata:  requiredFlagMetadata("ids"),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "ids", Usage: "Comma-separated worklog IDs (required)"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (properties)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
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
}

func worklogListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List worklogs on an issue",
		UsageText: `jira-agent issue worklog list PROJ-123`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "start-at", Usage: "Pagination offset", Value: 0},
			&cli.IntFlag{Name: "max-results", Usage: "Page size", Value: 20},
			&cli.StringFlag{Name: "started-after", Usage: "Only worklogs started after this Unix timestamp in milliseconds"},
			&cli.StringFlag{Name: "started-before", Usage: "Only worklogs started before this Unix timestamp in milliseconds"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (properties)", Value: ""},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"started-after":  "startedAfter",
				"started-before": "startedBefore",
				"expand":         "expand",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/worklog", params, result)
			})
		},
	}
}

func worklogAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a worklog to an issue",
		UsageText: `jira-agent issue worklog add PROJ-123 --time-spent 2h
jira-agent issue worklog add PROJ-123 --time-spent 30m --started "2025-01-15T09:00:00.000+0000"`,
		ArgsUsage: "<issue-key>",
		Flags:     worklogMutationFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
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
}

func worklogEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "Edit an existing worklog",
		UsageText: `jira-agent issue worklog edit PROJ-123 12345 --time-spent 3h`,
		ArgsUsage: "<issue-key> <worklog-id>",
		Flags:     worklogMutationFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "worklog ID")
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
}

func worklogDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a worklog",
		UsageText: `jira-agent issue worklog delete PROJ-123 12345`,
		ArgsUsage: "<issue-key> <worklog-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "notify", Usage: "Send notification to watchers", Value: true},
			&cli.StringFlag{Name: "adjust-estimate", Usage: "Estimate adjustment: auto, leave, manual, new"},
			&cli.StringFlag{Name: "new-estimate", Usage: "New remaining estimate when adjust-estimate is new"},
			&cli.StringFlag{Name: "increase-by", Usage: "Increase remaining estimate when adjust-estimate is manual"},
			&cli.BoolFlag{Name: "override-editable-flag", Usage: "Override worklog editable flag"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "worklog ID")
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
}

func worklogMutationFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "started", Usage: "Worklog start timestamp, e.g. 2026-04-27T10:00:00.000-0500"},
		&cli.StringFlag{Name: "time-spent", Usage: "Time spent, such as 1h 30m"},
		&cli.IntFlag{Name: "time-spent-seconds", Usage: "Time spent in seconds"},
		&cli.StringFlag{Name: "comment", Usage: "Worklog comment (plain text or ADF JSON)"},
		&cli.StringFlag{Name: "visibility-type", Usage: "Visibility restriction type: group or role"},
		&cli.StringFlag{Name: "visibility-value", Usage: "Visibility restriction value (group/role name)"},
		&cli.StringFlag{Name: "properties-json", Usage: "JSON array of worklog properties"},
		&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions (properties)"},
		&cli.BoolFlag{Name: "notify", Usage: "Send notification to watchers", Value: true},
		&cli.StringFlag{Name: "adjust-estimate", Usage: "Estimate adjustment: auto, leave, manual, new"},
		&cli.StringFlag{Name: "new-estimate", Usage: "New remaining estimate when adjust-estimate is new"},
		&cli.StringFlag{Name: "reduce-by", Usage: "Amount to reduce remaining estimate when adjust-estimate is manual"},
		&cli.BoolFlag{Name: "override-editable-flag", Usage: "Override worklog editable flag"},
	}
}

func buildWorklogBody(cmd *cli.Command, requireCoreFields bool) (map[string]any, error) {
	body := map[string]any{}
	if started := cmd.String("started"); started != "" {
		body["started"] = started
	}
	if timeSpent := cmd.String("time-spent"); timeSpent != "" {
		body["timeSpent"] = timeSpent
	}
	if seconds := cmd.Int("time-spent-seconds"); seconds > 0 {
		body["timeSpentSeconds"] = seconds
	}
	if comment := cmd.String("comment"); comment != "" {
		body["comment"] = toADF(comment)
	}
	vt, vv, err := requireVisibilityFlags(cmd)
	if err != nil {
		return nil, err
	}
	if vt != "" {
		body["visibility"] = map[string]any{"type": vt, "value": vv}
	}
	if propertiesJSON := cmd.String("properties-json"); propertiesJSON != "" {
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

func worklogMutationParams(cmd *cli.Command, includeIncreaseBy bool) map[string]string {
	params := map[string]string{}
	if !cmd.Bool("notify") {
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
