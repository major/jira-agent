package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// ComponentCommand returns the top-level "component" command with Jira project
// component operations (list, get, create, update, delete, issue-counts).
func ComponentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: "Project component operations (list, get, create, update, delete, issue-counts)",
		Example: `jira-agent component list --project PROJ
jira-agent component get 10100
jira-agent component create --project PROJ --name "Backend"`,
	}
	cmd.AddCommand(
		componentListCommand(apiClient, w, format),
		componentGetCommand(apiClient, w, format),
		componentCreateCommand(apiClient, w, format, allowWrites),
		componentUpdateCommand(apiClient, w, format, allowWrites),
		componentDeleteCommand(apiClient, w, format, allowWrites),
		componentIssueCountsCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// componentListCommand lists project components with pagination.
// GET /rest/api/3/component
func componentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project components",
		Example: `jira-agent component list --project PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			project, err := requireFlag(cmd, "project")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"order-by": "orderBy",
			})
			params["projectIdsOrKeys"] = project

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component", params, result)
			})
		},
	}
	cmd.Flags().String("query", "", "Filter components by name (case-insensitive substring match)")
	cmd.Flags().String("order-by", "", "Order results by field (e.g. name, description)")
	appendPaginationFlags(cmd)
	return cmd
}

// componentGetCommand fetches a single project component by ID.
// GET /rest/api/3/component/{id}
func componentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <component-id>",
		Short:   "Get component details by ID",
		Example: `jira-agent component get 10100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "component ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component/"+id, nil, result)
			})
		},
	}
	return cmd
}

// componentCreateCommand creates a new project component.
// POST /rest/api/3/component
func componentCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project component",
		Example: `jira-agent component create --project PROJ --name "Backend"
jira-agent component create --project PROJ --name "Frontend" --lead-account-id 5b10a284`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, _ := cmd.Flags().GetString("name")
			project, err := requireFlag(cmd, "project")
			if err != nil {
				return err
			}

			body := map[string]any{
				"name":    name,
				"project": project,
			}
			if v, _ := cmd.Flags().GetString("description"); v != "" {
				body["description"] = v
			}
			if v, _ := cmd.Flags().GetString("lead-account-id"); v != "" {
				body["leadAccountId"] = v
			}
			if v, _ := cmd.Flags().GetString("assignee-type"); v != "" {
				body["assigneeType"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/component", body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Component name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("description", "", "Component description")
	cmd.Flags().String("lead-account-id", "", "Account ID of the component lead")
	cmd.Flags().String("assignee-type", "", "Default assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, UNASSIGNED")
	return cmd
}

// componentUpdateCommand updates an existing project component.
// PUT /rest/api/3/component/{id}
func componentUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <component-id>",
		Short:   "Update a project component",
		Example: `jira-agent component update 10100 --name "Backend Services"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "component ID")
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				body["name"] = v
			}
			if v, _ := cmd.Flags().GetString("description"); v != "" {
				body["description"] = v
			}
			if v, _ := cmd.Flags().GetString("lead-account-id"); v != "" {
				body["leadAccountId"] = v
			}
			if v, _ := cmd.Flags().GetString("assignee-type"); v != "" {
				body["assigneeType"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/component/"+id, body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "New component name")
	cmd.Flags().String("description", "", "New component description")
	cmd.Flags().String("lead-account-id", "", "Account ID of the component lead")
	cmd.Flags().String("assignee-type", "", "Default assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, UNASSIGNED")
	return cmd
}

// componentDeleteCommand deletes a project component.
// DELETE /rest/api/3/component/{id}
func componentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <component-id>",
		Short:   "Delete a project component",
		Example: `jira-agent component delete 10100`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "component ID")
			if err != nil {
				return err
			}

			path := "/component/" + id
			if moveTo, _ := cmd.Flags().GetString("move-issues-to"); moveTo != "" {
				path = appendQueryParams(path, map[string]string{"moveIssuesTo": moveTo})
			}

			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"id": id, "deleted": true}, *format)
		}),
	}
	cmd.Flags().String("move-issues-to", "", "Component ID to reassign issues to before deletion")
	return cmd
}

// componentIssueCountsCommand returns issue counts for a component.
// GET /rest/api/3/component/{id}/relatedIssueCounts
func componentIssueCountsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue-counts <component-id>",
		Short:   "Get related issue counts for a component",
		Example: `jira-agent component issue-counts 10100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "component ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component/"+id+"/relatedIssueCounts", nil, result)
			})
		},
	}
	return cmd
}
