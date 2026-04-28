// jira-agent is a CLI tool for LLMs and AI agents to interact with
// Jira Cloud via its REST API v3.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

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
	app := buildApp(os.Stdout)
	ctx := context.Background()

	if err := app.Run(ctx, os.Args); err != nil {
		_ = output.WriteError(os.Stdout, err)
		os.Exit(apperr.ExitCodeFor(err))
	}
}

// buildApp constructs the CLI root command with production defaults.
func buildApp(w io.Writer) *cli.Command {
	return buildAppWithDeps(w, defaultAppDeps())
}

// buildAppWithDeps constructs the CLI root command with the given dependencies.
func buildAppWithDeps(w io.Writer, deps appDeps) *cli.Command {
	configPath := auth.DefaultConfigPath()

	// Pre-allocate the client ref so command closures always share the live
	// client. The Before hook populates ref.Client after authentication.
	apiClient := &client.Ref{}

	// Output format pointer: set in Before, read by commands. Defaults to
	// JSON so commands work even if Before is skipped (help, schema).
	outputFormat := new(output.Format)
	*outputFormat = output.FormatJSON

	// Write-protection pointer: set in Before from config, checked by write
	// commands. Defaults to false so writes are blocked unless explicitly
	// enabled via config file or JIRA_ALLOW_WRITES env var.
	allowWrites := new(bool)

	app := &cli.Command{
		Name:  "jira-agent",
		Usage: "CLI tool for LLMs to interact with Jira Cloud REST API",
		UsageText: `jira-agent issue get PROJ-123
jira-agent issue search --jql "project = PROJ"
jira-agent schema --compact
jira-agent whoami`,
		Version:   version,
		Suggest:   true,
		Writer:    w,
		ErrWriter: os.Stderr,
		ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {
			// Let main() render the JSON error envelope and set the exit code.
		},
		OnUsageError: func(_ context.Context, cmd *cli.Command, err error, _ bool) error {
			// When an unknown command is used, urfave/cli treats the unknown
			// name as arguments and chokes on subcommand flags. Detect that
			// case and return a clear error with a suggestion.
			if name := cmd.Args().First(); name != "" && !strings.HasPrefix(name, "-") {
				if cmd.Command(name) == nil {
					return unknownCommandError(cmd, name)
				}
			}
			return err
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "project",
				Aliases: []string{"p"},
				Usage:   "Override default Jira project key",
				Sources: cli.EnvVars("JIRA_PROJECT"),
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output format (json, csv, tsv)",
				Value:   "json",
			},
			&cli.BoolFlag{
				Name:  "pretty",
				Usage: "Pretty-print JSON output",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable debug logging to stderr",
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path to config file",
				Value: configPath,
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			subcommand := cmd.Args().First()

			// Catch unknown commands before attempting auth.
			if subcommand != "" && cmd.Command(subcommand) == nil {
				return ctx, unknownCommandError(cmd, subcommand)
			}

			// Parse output format early so it fails fast on invalid values.
			f, err := output.ParseFormat(cmd.String("output"))
			if err != nil {
				return ctx, apperr.NewValidationError(err.Error(), err)
			}
			if cmd.Bool("pretty") && f == output.FormatJSON {
				f = output.FormatJSONPretty
			}
			*outputFormat = f

			// Commands that don't require authentication.
			if subcommand == "help" || subcommand == "schema" || subcommand == "" {
				return ctx, nil
			}

			if cmd.Bool("verbose") {
				logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
				slog.SetDefault(logger)
			}

			resolvedConfigPath := cmd.String("config")
			if resolvedConfigPath == "" {
				resolvedConfigPath = configPath
			}

			cfg, err := deps.loadConfig(resolvedConfigPath)
			if err != nil {
				return ctx, apperr.NewAuthError(
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

			if cmd.Bool("verbose") {
				clientOptions = append(clientOptions, client.WithLogger(slog.Default()))
			}

			apiClient.Client = deps.newClient(cfg.BasicAuthHeader(), clientOptions...)
			return ctx, nil
		},
		Commands: []*cli.Command{
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
			commands.ServerInfoCommand(apiClient, w, outputFormat),
			commands.JQLCommand(apiClient, w, outputFormat),
		},
	}

	// Schema command is added after construction because it needs a
	// reference to the app itself for command tree introspection.
	app.Commands = append(app.Commands, commands.SchemaCommand(app, w))

	return app
}

// whoamiCommand returns the "whoami" command that displays the authenticated user.
func whoamiCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "whoami",
		Usage:     "Display the authenticated Jira user",
		UsageText: `jira-agent whoami`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var result any
			if err := apiClient.Get(ctx, "/myself", nil, &result); err != nil {
				return err
			}

			return output.WriteResult(w, result, *format)
		},
	}
}

// unknownCommandError builds a ValidationError for an unrecognized command,
// including a fuzzy-match suggestion when a close match exists.
func unknownCommandError(cmd *cli.Command, name string) error {
	msg := fmt.Sprintf("unknown command %q for %q", name, cmd.Name)

	if suggestion := cli.SuggestCommand(cmd.Commands, name); suggestion != "" {
		msg += fmt.Sprintf(". Did you mean %q?", suggestion)
	}

	return apperr.NewValidationError(msg, nil)
}
