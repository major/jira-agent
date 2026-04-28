package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// WorkflowCommand returns the top-level "workflow" command for Jira workflow
// configuration lookups. These endpoints are read-only, but Jira usually
// requires administrator permissions to call them.
func WorkflowCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "workflow",
		Usage: "Work with Jira workflows",
		UsageText: `jira-agent workflow list
jira-agent workflow get wf-10001
jira-agent workflow statuses`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			workflowListCommand(apiClient, w, format),
			workflowGetCommand(apiClient, w, format),
			workflowStatusesCommand(apiClient, w, format),
			workflowSchemeCommand(apiClient, w, format),
			workflowTransitionRulesCommand(apiClient, w, format),
		},
	}
}

func workflowListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "Search workflows",
		UsageText: `jira-agent workflow list
jira-agent workflow list --expand transitions`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{Name: "query", Usage: "Case-insensitive workflow name filter"},
			&cli.StringFlag{Name: "order-by", Usage: "Sort order: name, created, updated, +name, or -name"},
			&cli.StringFlag{Name: "scope", Usage: "Workflow scope: GLOBAL or PROJECT"},
			&cli.BoolFlag{Name: "is-active", Usage: "Only return active workflows"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions, such as values.transitions"},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, map[string]string{
				"query":    "queryString",
				"order-by": "orderBy",
				"scope":    "scope",
				"expand":   "expand",
			})
			addBoolParam(cmd, params, "is-active", "isActive")

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflows/search", params, result)
			})
		},
	}
}

func workflowGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a workflow by workflow ID or name",
		UsageText: `jira-agent workflow get wf-10001`,
		ArgsUsage: "<workflow-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Workflow name to read instead of a workflow ID"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			workflowID := cmd.Args().First()
			workflowName := cmd.String("name")
			switch {
			case workflowID == "" && workflowName == "":
				return apperr.NewValidationError("workflow ID or --name is required", nil)
			case workflowID != "" && workflowName != "":
				return apperr.NewValidationError("workflow ID and --name cannot be used together", nil)
			}

			body := map[string]any{}
			if workflowID != "" {
				body["workflowIds"] = []string{workflowID}
			} else {
				body["workflowNames"] = []string{workflowName}
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/workflows", body, result)
			})
		},
	}
}

func workflowStatusesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	cmd := flatListCommand("statuses", "List Jira statuses used by workflows", "/status", apiClient, w, format)
	cmd.UsageText = `jira-agent workflow statuses`
	return cmd
}

func workflowSchemeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "scheme",
		Usage: "Work with workflow schemes",
		UsageText: `jira-agent workflow scheme list
jira-agent workflow scheme get 10001`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			workflowSchemeListCommand(apiClient, w, format),
			workflowSchemeGetCommand(apiClient, w, format),
			workflowSchemeProjectCommand(apiClient, w, format),
		},
	}
}

func workflowSchemeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List workflow schemes",
		UsageText: `jira-agent workflow scheme list`,
		Flags:     appendPaginationFlags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflowscheme", params, result)
			})
		},
	}
}

func workflowSchemeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a workflow scheme",
		UsageText: `jira-agent workflow scheme get 10001`,
		ArgsUsage: "<scheme-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "return-draft", Usage: "Return the draft scheme when one exists"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			schemeID, err := requireNumericArg(cmd, "scheme ID")
			if err != nil {
				return err
			}
			params := map[string]string{}
			addBoolParam(cmd, params, "return-draft", "returnDraftIfExists")

			path := appendQueryParams("/workflowscheme/"+schemeID, params)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, nil, result)
			})
		},
	}
}

func workflowSchemeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "project",
		Usage:     "Get workflow scheme associations for a numeric project ID",
		UsageText: `jira-agent workflow scheme project 10001`,
		ArgsUsage: "<project-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectID, err := requireNumericArg(cmd, "project ID")
			if err != nil {
				return err
			}

			params := map[string]string{"projectId": projectID}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflowscheme/project", params, result)
			})
		},
	}
}

func workflowTransitionRulesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "transition-rules",
		Usage:     "List available workflow transition rule capabilities",
		UsageText: `jira-agent workflow transition-rules --workflow-ids wf-10001,wf-10002`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "workflow-id", Usage: "Workflow UUID context"},
			&cli.StringFlag{Name: "project-id", Usage: "Numeric project ID context"},
			&cli.StringFlag{Name: "issue-type-id", Usage: "Issue type ID context"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"workflow-id":   "workflowId",
				"project-id":    "projectId",
				"issue-type-id": "issueTypeId",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflows/capabilities", params, result)
			})
		},
	}
}
