package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// TaskCommand returns the top-level "task" command for Jira async task status.
func TaskCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Work with Jira async tasks",
		Example: `jira-agent task get 10641
jira-agent task cancel 10641`,
	}
	cmd.AddCommand(
		taskGetCommand(apiClient, w, format),
		taskCancelCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "get")
	return cmd
}

// GET /rest/api/3/task/{taskId}
func taskGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get <task-id>",
		Short:   "Get async task status",
		Example: `jira-agent task get 10641`,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := requireArg(args, "task ID")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			path := "/task/" + escapePathSegment(taskID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, nil, result)
			})
		},
	}
}

// POST /rest/api/3/task/{taskId}/cancel
func taskCancelCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "cancel <task-id>",
		Short:   "Cancel an async task",
		Example: `jira-agent task cancel 10641`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			taskID, err := requireArg(args, "task ID")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			path := "/task/" + escapePathSegment(taskID) + "/cancel"
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, nil, result)
			})
		}),
	}
}
