package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// WorkflowCommand returns the top-level "workflow" command for Jira workflow
// configuration lookups. These endpoints are read-only, but Jira usually
// requires administrator permissions to call them.
func WorkflowCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Work with Jira workflows",
		Example: `jira-agent workflow list
jira-agent workflow get wf-10001
jira-agent workflow statuses`,
	}
	cmd.AddCommand(
		workflowListCommand(apiClient, w, format),
		workflowGetCommand(apiClient, w, format),
		workflowStatusesCommand(apiClient, w, format),
		workflowSchemeCommand(apiClient, w, format),
		workflowTransitionRulesCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func workflowListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Search workflows",
		Example: `jira-agent workflow list
jira-agent workflow list --expand transitions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	cmd.Flags().String("query", "", "Case-insensitive workflow name filter")
	cmd.Flags().String("order-by", "", "Sort order: name, created, updated, +name, or -name")
	cmd.Flags().String("scope", "", "Workflow scope: GLOBAL or PROJECT")
	cmd.Flags().Bool("is-active", false, "Only return active workflows")
	cmd.Flags().String("expand", "", "Comma-separated expansions, such as values.transitions")
	appendPaginationFlags(cmd)
	return cmd
}

func workflowGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <workflow-id>",
		Short:   "Get a workflow by workflow ID or name",
		Example: `jira-agent workflow get wf-10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			workflowID := ""
			if len(args) > 0 {
				workflowID = args[0]
			}
			workflowName, _ := cmd.Flags().GetString("name")
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
	cmd.Flags().String("name", "", "Workflow name to read instead of a workflow ID")
	return cmd
}

func workflowStatusesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return flatListCommand("statuses", "List Jira statuses used by workflows", "jira-agent workflow statuses", "/status", apiClient, w, format)
}

func workflowSchemeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheme",
		Short: "Work with workflow schemes",
		Example: `jira-agent workflow scheme list
jira-agent workflow scheme get 10001`,
	}
	cmd.AddCommand(
		workflowSchemeListCommand(apiClient, w, format),
		workflowSchemeGetCommand(apiClient, w, format),
		workflowSchemeProjectCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func workflowSchemeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List workflow schemes",
		Example: `jira-agent workflow scheme list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflowscheme", params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

func workflowSchemeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <scheme-id>",
		Short:   "Get a workflow scheme",
		Example: `jira-agent workflow scheme get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			schemeID, err := requireNumericArg(args, "scheme ID")
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
	cmd.Flags().Bool("return-draft", false, "Return the draft scheme when one exists")
	return cmd
}

func workflowSchemeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project <project-id>",
		Short:   "Get workflow scheme associations for a numeric project ID",
		Example: `jira-agent workflow scheme project 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectID, err := requireNumericArg(args, "project ID")
			if err != nil {
				return err
			}

			params := map[string]string{"projectId": projectID}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/workflowscheme/project", params, result)
			})
		},
	}
	return cmd
}

func workflowTransitionRulesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transition-rules",
		Short:   "List available workflow transition rule capabilities",
		Example: `jira-agent workflow transition-rules --workflow-ids wf-10001,wf-10002`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	cmd.Flags().String("workflow-id", "", "Workflow UUID context")
	cmd.Flags().String("project-id", "", "Numeric project ID context")
	cmd.Flags().String("issue-type-id", "", "Issue type ID context")
	return cmd
}
