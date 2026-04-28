package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

const timeTrackingPath = "/configuration/timetracking"

// TimeTrackingCommand returns the top-level "time-tracking" command for Jira
// time tracking provider and settings configuration.
func TimeTrackingCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "time-tracking",
		Usage: "Work with Jira time tracking configuration",
		UsageText: `jira-agent time-tracking get
jira-agent time-tracking providers
jira-agent time-tracking select --key JIRA
jira-agent time-tracking options get
jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
		DefaultCommand: "get",
		Commands: []*cli.Command{
			timeTrackingGetCommand(apiClient, w, format),
			timeTrackingProvidersCommand(apiClient, w, format),
			timeTrackingSelectCommand(apiClient, w, format, allowWrites),
			timeTrackingOptionsCommand(apiClient, w, format, allowWrites),
		},
	}
}

// GET /rest/api/3/configuration/timetracking
func timeTrackingGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get selected time tracking provider",
		UsageText: `jira-agent time-tracking get`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath, nil, result)
			})
		},
	}
}

// GET /rest/api/3/configuration/timetracking/list
func timeTrackingProvidersCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "providers",
		Usage:     "List time tracking providers",
		UsageText: `jira-agent time-tracking providers`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath+"/list", nil, result)
			})
		},
	}
}

// PUT /rest/api/3/configuration/timetracking
func timeTrackingSelectCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "select",
		Usage:     "Select time tracking provider",
		UsageText: `jira-agent time-tracking select --key JIRA`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "key",
				Usage:    "Provider key from time-tracking providers",
				Required: true,
			},
		},
		Metadata: commandMetadata(writeCommandMetadata(), requiredFlagMetadata("key")),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireFlag(cmd, "key")
			if err != nil {
				return err
			}

			body := map[string]string{"key": key}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, timeTrackingPath, body, result)
			})
		}),
	}
}

func timeTrackingOptionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "options",
		Usage: "Work with time tracking settings",
		UsageText: `jira-agent time-tracking options get
jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
		DefaultCommand: "get",
		Commands: []*cli.Command{
			timeTrackingOptionsGetCommand(apiClient, w, format),
			timeTrackingOptionsSetCommand(apiClient, w, format, allowWrites),
		},
	}
}

// GET /rest/api/3/configuration/timetracking/options
func timeTrackingOptionsGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get time tracking settings",
		UsageText: `jira-agent time-tracking options get`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath+"/options", nil, result)
			})
		},
	}
}

// PUT /rest/api/3/configuration/timetracking/options
func timeTrackingOptionsSetCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Set time tracking settings",
		UsageText: `jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
		Flags: []cli.Flag{
			&cli.Float64Flag{
				Name:     "working-hours-per-day",
				Usage:    "Working hours in one day",
				Required: true,
			},
			&cli.Float64Flag{
				Name:     "working-days-per-week",
				Usage:    "Working days in one week",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "time-format",
				Usage:    "Time display format: pretty, days, or hours",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "default-unit",
				Usage:    "Default time unit: minute, hour, day, or week",
				Required: true,
			},
		},
		Metadata: commandMetadata(
			writeCommandMetadata(),
			requiredFlagMetadata("working-hours-per-day", "working-days-per-week", "time-format", "default-unit"),
		),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body := map[string]any{
				"workingHoursPerDay": cmd.Float64("working-hours-per-day"),
				"workingDaysPerWeek": cmd.Float64("working-days-per-week"),
				"timeFormat":         cmd.String("time-format"),
				"defaultUnit":        cmd.String("default-unit"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, timeTrackingPath+"/options", body, result)
			})
		}),
	}
}
