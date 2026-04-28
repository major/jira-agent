package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// TaskCommand returns the top-level "task" command for Jira async task status.
func TaskCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "task",
		Usage: "Work with Jira async tasks",
		UsageText: `jira-agent task get 10641
jira-agent task cancel 10641`,
		DefaultCommand: "get",
		Commands: []*cli.Command{
			taskGetCommand(apiClient, w, format),
			taskCancelCommand(apiClient, w, format, allowWrites),
		},
	}
}

// GET /rest/api/3/task/{taskId}
func taskGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get async task status",
		UsageText: `jira-agent task get 10641`,
		ArgsUsage: "<task-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			taskID, err := requireArg(cmd, "task ID")
			if err != nil {
				return err
			}

			path := "/task/" + escapePathSegment(taskID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, nil, result)
			})
		},
	}
}

// POST /rest/api/3/task/{taskId}/cancel
func taskCancelCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "cancel",
		Usage:     "Cancel an async task",
		UsageText: `jira-agent task cancel 10641`,
		ArgsUsage: "<task-id>",
		Metadata:  writeCommandMetadata(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			taskID, err := requireArg(cmd, "task ID")
			if err != nil {
				return err
			}

			path := "/task/" + escapePathSegment(taskID) + "/cancel"
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, nil, result)
			})
		}),
	}
}
