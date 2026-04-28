package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// VersionCommand returns the top-level "version" command with Jira project
// version operations (list, get, create, update, delete, merge, move,
// issue-counts, unresolved-count).
func VersionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Project version operations (list, get, create, update, delete, merge, move, issue-counts, unresolved-count)",
		UsageText: `jira-agent version list --project PROJ
jira-agent version get 10001
jira-agent version create --project PROJ --name "v1.0"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			versionListCommand(apiClient, w, format),
			versionGetCommand(apiClient, w, format),
			versionCreateCommand(apiClient, w, format, allowWrites),
			versionUpdateCommand(apiClient, w, format, allowWrites),
			versionDeleteCommand(apiClient, w, format, allowWrites),
			versionMergeCommand(apiClient, w, format, allowWrites),
			versionMoveCommand(apiClient, w, format, allowWrites),
			versionIssueCountsCommand(apiClient, w, format),
			versionUnresolvedCountCommand(apiClient, w, format),
		},
	}
}

// versionListCommand lists project versions with pagination.
// GET /rest/api/3/project/{projectIdOrKey}/version
func versionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List project versions",
		UsageText: `jira-agent version list --project PROJ`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{
				Name:     "project",
				Usage:    "Project key or ID",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "query",
				Usage: "Filter versions by name or description (case-insensitive)",
			},
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Order results by field (name, releaseDate, sequence, startDate, description)",
			},
			&cli.StringFlag{
				Name:  "status",
				Usage: "Filter by status: released, unreleased, archived (comma-separated)",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Expand additional info: issuesstatus, operations, driver, approvers (comma-separated)",
			},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			project := cmd.String("project")
			params := buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"order-by": "orderBy",
				"status":   "status",
				"expand":   "expand",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+project+"/version", params, result)
			})
		},
	}
}

// versionGetCommand fetches a single project version by ID.
// GET /rest/api/3/version/{id}
func versionGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get version details by ID",
		UsageText: `jira-agent version get 10001`,
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Expand additional info: issuesstatus, operations, driver, approvers (comma-separated)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/version/"+id, params, result)
			})
		},
	}
}

// versionCreateCommand creates a new project version.
// POST /rest/api/3/version
func versionCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a project version",
		UsageText: `jira-agent version create --project PROJ --name "v1.0"
jira-agent version create --project PROJ --name "v2.0" --start-date 2025-01-01 --release-date 2025-06-01`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Version name", Required: true},
			&cli.StringFlag{Name: "project", Usage: "Project key", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Version description"},
			&cli.StringFlag{Name: "release-date", Usage: "Release date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "start-date", Usage: "Start date (YYYY-MM-DD)"},
			&cli.BoolFlag{Name: "released", Usage: "Mark version as released"},
			&cli.BoolFlag{Name: "archived", Usage: "Mark version as archived"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body := map[string]any{
				"name":    cmd.String("name"),
				"project": cmd.String("project"),
			}
			if v := cmd.String("description"); v != "" {
				body["description"] = v
			}
			if v := cmd.String("release-date"); v != "" {
				body["releaseDate"] = v
			}
			if v := cmd.String("start-date"); v != "" {
				body["startDate"] = v
			}
			if cmd.IsSet("released") {
				body["released"] = cmd.Bool("released")
			}
			if cmd.IsSet("archived") {
				body["archived"] = cmd.Bool("archived")
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/version", body, result)
			})
		}),
	}
}

// versionUpdateCommand updates an existing project version.
// PUT /rest/api/3/version/{id}
func versionUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update a project version",
		UsageText: `jira-agent version update 10001 --name "v1.1"
jira-agent version update 10001 --released`,
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "New version name"},
			&cli.StringFlag{Name: "description", Usage: "New version description"},
			&cli.StringFlag{Name: "release-date", Usage: "Release date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "start-date", Usage: "Start date (YYYY-MM-DD)"},
			&cli.BoolFlag{Name: "released", Usage: "Mark version as released"},
			&cli.BoolFlag{Name: "archived", Usage: "Mark version as archived"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
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
			if v := cmd.String("release-date"); v != "" {
				body["releaseDate"] = v
			}
			if v := cmd.String("start-date"); v != "" {
				body["startDate"] = v
			}
			if cmd.IsSet("released") {
				body["released"] = cmd.Bool("released")
			}
			if cmd.IsSet("archived") {
				body["archived"] = cmd.Bool("archived")
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/version/"+id, body, result)
			})
		}),
	}
}

// versionDeleteCommand deletes a project version.
// DELETE /rest/api/3/version/{id}
func versionDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a project version",
		UsageText: `jira-agent version delete 10001`,
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "move-fix-issues-to", Usage: "Version ID to reassign fixVersion issues to"},
			&cli.StringFlag{Name: "move-affected-issues-to", Usage: "Version ID to reassign affectedVersion issues to"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
			if err != nil {
				return err
			}

			path := "/version/" + id
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"move-fix-issues-to":      "moveFixIssuesTo",
				"move-affected-issues-to": "moveAffectedIssuesTo",
			})
			if len(params) > 0 {
				path = appendQueryParams(path, params)
			}

			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"id": id, "deleted": true}, *format)
		}),
	}
}

// versionMergeCommand merges one version into another, deleting the source.
// PUT /rest/api/3/version/{id}/mergeto/{moveIssuesTo}
func versionMergeCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "merge",
		Usage:     "Merge a version into another (deletes the source version)",
		UsageText: `jira-agent version merge 10001 10002`,
		ArgsUsage: "<source-version-id> <target-version-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "source version ID", "target version ID")
			if err != nil {
				return err
			}

			if err := apiClient.Put(ctx, "/version/"+args[0]+"/mergeto/"+args[1], nil, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"sourceId": args[0],
				"targetId": args[1],
				"merged":   true,
			}, *format)
		}),
	}
}

// versionMoveCommand reorders a version within the project.
// POST /rest/api/3/version/{id}/move
func versionMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "move",
		Usage: "Move a version's position within the project",
		UsageText: `jira-agent version move 10001 --after 10002
jira-agent version move 10001 --position Earlier`,
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "after", Usage: "URL of the version to place after"},
			&cli.StringFlag{Name: "position", Usage: "Position keyword: Earlier, Later, First, Last"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v := cmd.String("after"); v != "" {
				body["after"] = v
			}
			if v := cmd.String("position"); v != "" {
				body["position"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/version/"+id+"/move", body, result)
			})
		}),
	}
}

// versionIssueCountsCommand returns related issue counts for a version.
// GET /rest/api/3/version/{id}/relatedIssueCounts
func versionIssueCountsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "issue-counts",
		Usage:     "Get related issue counts for a version",
		UsageText: `jira-agent version issue-counts 10001`,
		ArgsUsage: "<version-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/version/"+id+"/relatedIssueCounts", nil, result)
			})
		},
	}
}

// versionUnresolvedCountCommand returns unresolved issue counts for a version.
// GET /rest/api/3/version/{id}/unresolvedIssueCount
func versionUnresolvedCountCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "unresolved-count",
		Usage:     "Get unresolved issue count for a version",
		UsageText: `jira-agent version unresolved-count 10001`,
		ArgsUsage: "<version-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id, err := requireArg(cmd, "version ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/version/"+id+"/unresolvedIssueCount", nil, result)
			})
		},
	}
}
