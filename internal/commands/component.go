package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// ComponentCommand returns the top-level "component" command with Jira project
// component operations (list, get, create, update, delete, issue-counts).
func ComponentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "component",
		Usage: "Project component operations (list, get, create, update, delete, issue-counts)",
		UsageText: `jira-agent component list --project PROJ
jira-agent component get 10100
jira-agent component create --project PROJ --name "Backend"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			componentListCommand(apiClient, w, format),
			componentGetCommand(apiClient, w, format),
			componentCreateCommand(apiClient, w, format, allowWrites),
			componentUpdateCommand(apiClient, w, format, allowWrites),
			componentDeleteCommand(apiClient, w, format, allowWrites),
			componentIssueCountsCommand(apiClient, w, format),
		},
	}
}

// componentListCommand lists project components with pagination.
// GET /rest/api/3/component
func componentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List project components",
		UsageText: `jira-agent component list --project PROJ`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:     "project",
				Usage:    "Project key or ID",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "query",
				Usage: "Filter components by name (case-insensitive substring match)",
			},
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Order results by field (e.g. name, description)",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"order-by": "orderBy",
			})
			params["projectIdsOrKeys"] = cmd.String("project")

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component", params, result)
			})
		},
	}
}

// componentGetCommand fetches a single project component by ID.
// GET /rest/api/3/component/{id}
func componentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get component details by ID",
		UsageText: `jira-agent component get 10100`,
		ArgsUsage: "<component-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "component ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component/"+id, nil, result)
			})
		},
	}
}

// componentCreateCommand creates a new project component.
// POST /rest/api/3/component
func componentCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a project component",
		UsageText: `jira-agent component create --project PROJ --name "Backend"
jira-agent component create --project PROJ --name "Frontend" --lead-account-id 5b10a284`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Component name", Required: true},
			&cli.StringFlag{Name: "project", Usage: "Project key", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Component description"},
			&cli.StringFlag{Name: "lead-account-id", Usage: "Account ID of the component lead"},
			&cli.StringFlag{Name: "assignee-type", Usage: "Default assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, UNASSIGNED"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body := map[string]any{
				"name":    cmd.String("name"),
				"project": cmd.String("project"),
			}
			if v := cmd.String("description"); v != "" {
				body["description"] = v
			}
			if v := cmd.String("lead-account-id"); v != "" {
				body["leadAccountId"] = v
			}
			if v := cmd.String("assignee-type"); v != "" {
				body["assigneeType"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/component", body, result)
			})
		}),
	}
}

// componentUpdateCommand updates an existing project component.
// PUT /rest/api/3/component/{id}
func componentUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a project component",
		UsageText: `jira-agent component update 10100 --name "Backend Services"`,
		ArgsUsage: "<component-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "New component name"},
			&cli.StringFlag{Name: "description", Usage: "New component description"},
			&cli.StringFlag{Name: "lead-account-id", Usage: "Account ID of the component lead"},
			&cli.StringFlag{Name: "assignee-type", Usage: "Default assignee type: PROJECT_DEFAULT, COMPONENT_LEAD, PROJECT_LEAD, UNASSIGNED"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "component ID")
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v := cmd.String("name"); v != "" {
				body["name"] = v
			}
			if v := cmd.String("description"); v != "" {
				body["description"] = v
			}
			if v := cmd.String("lead-account-id"); v != "" {
				body["leadAccountId"] = v
			}
			if v := cmd.String("assignee-type"); v != "" {
				body["assigneeType"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/component/"+id, body, result)
			})
		}),
	}
}

// componentDeleteCommand deletes a project component.
// DELETE /rest/api/3/component/{id}
func componentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a project component",
		UsageText: `jira-agent component delete 10100`,
		ArgsUsage: "<component-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "move-issues-to", Usage: "Component ID to reassign issues to before deletion"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "component ID")
			if err != nil {
				return err
			}

			path := "/component/" + id
			if moveTo := cmd.String("move-issues-to"); moveTo != "" {
				path = appendQueryParams(path, map[string]string{"moveIssuesTo": moveTo})
			}

			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"id": id, "deleted": true}, *format)
		}),
	}
}

// componentIssueCountsCommand returns issue counts for a component.
// GET /rest/api/3/component/{id}/relatedIssueCounts
func componentIssueCountsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "issue-counts",
		Usage:     "Get related issue counts for a component",
		UsageText: `jira-agent component issue-counts 10100`,
		ArgsUsage: "<component-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "component ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/component/"+id+"/relatedIssueCounts", nil, result)
			})
		},
	}
}
