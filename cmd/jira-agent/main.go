// jira-agent is a CLI tool for LLMs and AI agents to interact with
// Jira Cloud via its REST API v3.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/auth"
	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/commands"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

// appDeps holds injectable dependencies for the Before hook.
// Tests override individual fields; production uses defaultAppDeps().
type appDeps struct {
	newClient  func(authHeader string, opts ...client.Option) *client.Client
	loadConfig func(path string) (*auth.Config, error)
}

func defaultAppDeps() appDeps {
	return appDeps{
		newClient:  client.NewClient,
		loadConfig: auth.LoadConfig,
	}
}

func main() {
	ctx := context.Background()
	rootCmd := buildApp(os.Stdout)
	rootCmd.SetContext(ctx)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		_ = output.WriteError(os.Stdout, err)
		os.Exit(apperr.ExitCodeFor(err))
	}
}

// buildApp constructs the CLI root command with production defaults.
func buildApp(w io.Writer) *cobra.Command {
	return buildAppWithDeps(w, defaultAppDeps())
}

// buildAppWithDeps constructs the CLI root command with the given dependencies.
func buildAppWithDeps(w io.Writer, deps appDeps) *cobra.Command {
	configPath := auth.DefaultConfigPath()

	// Pre-allocate the client ref so command closures always share the live
	// client. The PersistentPreRunE hook populates ref.Client after authentication.
	apiClient := &client.Ref{}

	// Output format pointer: set in PersistentPreRunE, read by commands. Defaults
	// to JSON so commands work even if PersistentPreRunE is skipped for help.
	outputFormat := new(output.Format)
	*outputFormat = output.FormatJSON

	// Write-protection pointer: set in PersistentPreRunE from config, checked by
	// write commands. Defaults to false so writes are blocked unless explicitly
	// enabled via config file or JIRA_ALLOW_WRITES env var.
	allowWrites := new(bool)

	rootCmd := &cobra.Command{
		Use:   "jira-agent",
		Args:  cobra.ArbitraryArgs,
		Short: "CLI tool for LLMs to interact with Jira Cloud REST API",
		Long:  "Project repository: https://github.com/major/jira-agent\nFile new bugs and RFEs there.",
		Example: `jira-agent issue get PROJ-123 --fields key,summary,status
jira-agent issue search --jql "assignee = currentUser()"
jira-agent project list --output csv`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return unknownCommandError(cmd, args[0])
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			bindEnvDefault(cmd, "project", "JIRA_PROJECT")

			// Parse output format early so it fails fast on invalid values.
			formatValue, err := cmd.Flags().GetString("output")
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
			}
			f, err := output.ParseFormat(formatValue)
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
			}
			pretty, err := cmd.Flags().GetBool("pretty")
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
			}
			if pretty && f == output.FormatJSON {
				f = output.FormatJSONPretty
			}
			*outputFormat = f

			// Commands that don't require authentication.
			if cmd.Name() == cmd.Root().Name() {
				return nil
			}

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
			}
			if verbose {
				logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
				slog.SetDefault(logger)
			}

			resolvedConfigPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
			}
			if resolvedConfigPath == "" {
				resolvedConfigPath = configPath
			}

			cfg, err := deps.loadConfig(resolvedConfigPath)
			if err != nil {
				return apperr.NewAuthError(
					"failed to load configuration",
					err,
					apperr.WithDetails("set JIRA_INSTANCE, JIRA_EMAIL, and JIRA_API_KEY env vars, or create a config file at "+configPath),
				)
			}

			*allowWrites = cfg.AllowWrites

			clientOptions := []client.Option{
				client.WithUserAgent("jira-agent/" + version),
				client.WithBaseURL(cfg.BaseURL()),
				client.WithAgileBaseURL(cfg.AgileBaseURL()),
			}

			if verbose {
				clientOptions = append(clientOptions, client.WithLogger(slog.Default()))
			}

			apiClient.Client = deps.newClient(cfg.BasicAuthHeader(), clientOptions...)
			return nil
		},
	}
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetOut(w)
	rootCmd.SetErr(os.Stderr)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	flags := rootCmd.PersistentFlags()
	flags.StringP("project", "p", "", "Override default Jira project key")
	flags.StringP("output", "o", "json", "Output format (json, csv, tsv)")
	flags.Bool("pretty", false, "Pretty-print JSON output")
	flags.BoolP("verbose", "v", false, "Enable debug logging to stderr")
	flags.String("config", configPath, "Path to config file")

	rootCmd.AddCommand(
		whoamiCommand(apiClient, w, outputFormat),
		commands.AuditCommand(apiClient, w, outputFormat),
		commands.IssueCommand(apiClient, w, outputFormat, allowWrites),
		commands.FieldCommand(apiClient, w, outputFormat, allowWrites),
		commands.ProjectCommand(apiClient, w, outputFormat, allowWrites),
		commands.RoleCommand(apiClient, w, outputFormat),
		commands.BoardCommand(apiClient, w, outputFormat, allowWrites),
		commands.UserCommand(apiClient, w, outputFormat),
		commands.GroupCommand(apiClient, w, outputFormat, allowWrites),
		commands.FilterCommand(apiClient, w, outputFormat, allowWrites),
		commands.PermissionCommand(apiClient, w, outputFormat),
		commands.DashboardCommand(apiClient, w, outputFormat, allowWrites),
		commands.WorkflowCommand(apiClient, w, outputFormat),
		commands.StatusCommand(apiClient, w, outputFormat),
		commands.PriorityCommand(apiClient, w, outputFormat),
		commands.ResolutionCommand(apiClient, w, outputFormat),
		commands.IssueTypeCommand(apiClient, w, outputFormat),
		commands.LabelCommand(apiClient, w, outputFormat),
		commands.ComponentCommand(apiClient, w, outputFormat, allowWrites),
		commands.VersionCommand(apiClient, w, outputFormat, allowWrites),
		commands.SprintCommand(apiClient, w, outputFormat, allowWrites),
		commands.EpicCommand(apiClient, w, outputFormat, allowWrites),
		commands.BacklogCommand(apiClient, w, outputFormat, allowWrites),
		commands.TaskCommand(apiClient, w, outputFormat, allowWrites),
		commands.TimeTrackingCommand(apiClient, w, outputFormat, allowWrites),
		commands.ServerInfoCommand(apiClient, w, outputFormat),
		commands.JQLCommand(apiClient, w, outputFormat),
	)

	return rootCmd
}

// whoamiCommand returns the "whoami" command that displays the authenticated user.
func whoamiCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Short:   "Display the authenticated Jira user",
		Example: `jira-agent whoami`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var result any
			if err := apiClient.Get(ctx, "/myself", nil, &result); err != nil {
				return err
			}

			return output.WriteResult(w, result, *format)
		},
	}
}

// bindEnvDefault sets a flag's value from an env var if the flag was not
// explicitly passed and the env var is non-empty.
func bindEnvDefault(cmd *cobra.Command, flagName, envVar string) {
	if val := os.Getenv(envVar); val != "" && !cmd.Flags().Changed(flagName) {
		_ = cmd.Flags().Set(flagName, val)
	}
}

// unknownCommandError builds a ValidationError for an unrecognized command,
// including a fuzzy-match suggestion when a close match exists.
func unknownCommandError(cmd *cobra.Command, name string) error {
	msg := fmt.Sprintf("unknown command %q for %q", name, cmd.CommandPath())

	if suggestions := cmd.Root().SuggestionsFor(name); len(suggestions) > 0 {
		msg += fmt.Sprintf(". Did you mean %q?", suggestions[0])
	}

	return apperr.NewValidationError(msg, nil)
}
