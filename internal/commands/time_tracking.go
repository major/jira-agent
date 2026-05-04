package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

const timeTrackingPath = "/configuration/timetracking"

// TimeTrackingCommand returns the top-level "time-tracking" command for Jira
// time tracking provider and settings configuration.
func TimeTrackingCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time-tracking",
		Short: "Work with Jira time tracking configuration",
		Example: `jira-agent time-tracking get
jira-agent time-tracking providers
jira-agent time-tracking select --key JIRA
jira-agent time-tracking options get
jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
	}
	cmd.AddCommand(
		timeTrackingGetCommand(apiClient, w, format),
		timeTrackingProvidersCommand(apiClient, w, format),
		timeTrackingSelectCommand(apiClient, w, format, allowWrites),
		timeTrackingOptionsCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "get")
	return cmd
}

// GET /rest/api/3/configuration/timetracking
func timeTrackingGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Short:   "Get selected time tracking provider",
		Example: `jira-agent time-tracking get`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath, nil, result)
			})
		},
	}
}

// GET /rest/api/3/configuration/timetracking/list
func timeTrackingProvidersCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "providers",
		Short:   "List time tracking providers",
		Example: `jira-agent time-tracking providers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath+"/list", nil, result)
			})
		},
	}
}

// PUT /rest/api/3/configuration/timetracking
func timeTrackingSelectCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "select",
		Short:   "Select time tracking provider",
		Example: `jira-agent time-tracking select --key JIRA`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			key, err := requireFlag(cmd, "key")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			body := map[string]string{"key": key}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, timeTrackingPath, body, result)
			})
		}),
	}
	cmd.Flags().String("key", "", "Provider key from time-tracking providers")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func timeTrackingOptionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Work with time tracking settings",
		Example: `jira-agent time-tracking options get
jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
	}
	cmd.AddCommand(
		timeTrackingOptionsGetCommand(apiClient, w, format),
		timeTrackingOptionsSetCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "get")
	return cmd
}

// GET /rest/api/3/configuration/timetracking/options
func timeTrackingOptionsGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Short:   "Get time tracking settings",
		Example: `jira-agent time-tracking options get`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, timeTrackingPath+"/options", nil, result)
			})
		},
	}
}

// PUT /rest/api/3/configuration/timetracking/options
func timeTrackingOptionsSetCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Set time tracking settings",
		Example: `jira-agent time-tracking options set --working-hours-per-day 8 --working-days-per-week 5 --time-format pretty --default-unit minute`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			hoursPerDay, _ := cmd.Flags().GetFloat64("working-hours-per-day")
			daysPerWeek, _ := cmd.Flags().GetFloat64("working-days-per-week")
			timeFormat, _ := cmd.Flags().GetString("time-format")
			defaultUnit, _ := cmd.Flags().GetString("default-unit")

			body := map[string]any{
				"workingHoursPerDay": hoursPerDay,
				"workingDaysPerWeek": daysPerWeek,
				"timeFormat":         timeFormat,
				"defaultUnit":        defaultUnit,
			}

			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, timeTrackingPath+"/options", body, result)
			})
		}),
	}
	cmd.Flags().Float64("working-hours-per-day", 0, "Working hours in one day")
	_ = cmd.MarkFlagRequired("working-hours-per-day")
	cmd.Flags().Float64("working-days-per-week", 0, "Working days in one week")
	_ = cmd.MarkFlagRequired("working-days-per-week")
	cmd.Flags().String("time-format", "", "Time display format: pretty, days, or hours")
	_ = cmd.MarkFlagRequired("time-format")
	cmd.Flags().String("default-unit", "", "Default time unit: minute, hour, day, or week")
	_ = cmd.MarkFlagRequired("default-unit")
	return cmd
}
