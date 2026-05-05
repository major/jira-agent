package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// VersionCommand returns the top-level "version" command with Jira project
// version operations (list, get, create, update, delete, merge, move,
// issue-counts, unresolved-count).
func VersionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Project version operations (list, get, create, update, delete, merge, move, issue-counts, unresolved-count)",
		Example: `jira-agent version list --project PROJ
jira-agent version get 10001
jira-agent version create --project PROJ --name "v1.0"`,
	}
	cmd.AddCommand(
		versionListCommand(apiClient, w, format),
		versionGetCommand(apiClient, w, format),
		versionCreateCommand(apiClient, w, format, allowWrites),
		versionUpdateCommand(apiClient, w, format, allowWrites),
		versionDeleteCommand(apiClient, w, format, allowWrites),
		versionMergeCommand(apiClient, w, format, allowWrites),
		versionMoveCommand(apiClient, w, format, allowWrites),
		versionIssueCountsCommand(apiClient, w, format),
		versionUnresolvedCountCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// versionListCommand lists project versions with pagination.
// GET /rest/api/3/project/{projectIdOrKey}/version
func versionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project versions",
		Example: `jira-agent version list --project PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			project, err := requireFlag(cmd, "project")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"order-by": "orderBy",
				"status":   "status",
				"expand":   "expand",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+project+"/version", params, result)
			})
		},
	}
	cmd.Flags().String("query", "", "Filter versions by name or description (case-insensitive)")
	cmd.Flags().String("order-by", "", "Order results by field (name, releaseDate, sequence, startDate, description)")
	cmd.Flags().String("status", "", "Filter by status: released, unreleased, archived (comma-separated)")
	cmd.Flags().String("expand", "", "Expand additional info: issuesstatus, operations, driver, approvers (comma-separated)")
	appendPaginationFlags(cmd)
	return cmd
}

// versionGetCommand fetches a single project version by ID.
// GET /rest/api/3/version/{id}
func versionGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <version-id>",
		Short:   "Get version details by ID",
		Example: `jira-agent version get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
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
	cmd.Flags().String("expand", "", "Expand additional info: issuesstatus, operations, driver, approvers (comma-separated)")
	return cmd
}

// versionCreateCommand creates a new project version.
// POST /rest/api/3/version
func versionCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project version",
		Example: `jira-agent version create --project PROJ --name "v1.0"
jira-agent version create --project PROJ --name "v2.0" --start-date 2025-01-01 --release-date 2025-06-01`,
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
			if v, _ := cmd.Flags().GetString("release-date"); v != "" {
				body["releaseDate"] = v
			}
			if v, _ := cmd.Flags().GetString("start-date"); v != "" {
				body["startDate"] = v
			}
			if cmd.Flags().Changed("released") {
				val, _ := cmd.Flags().GetBool("released")
				body["released"] = val
			}
			if cmd.Flags().Changed("archived") {
				val, _ := cmd.Flags().GetBool("archived")
				body["archived"] = val
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/version", body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Version name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("description", "", "Version description")
	cmd.Flags().String("release-date", "", "Release date (YYYY-MM-DD)")
	cmd.Flags().String("start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().Bool("released", false, "Mark version as released")
	cmd.Flags().Bool("archived", false, "Mark version as archived")
	return cmd
}

// versionUpdateCommand updates an existing project version.
// PUT /rest/api/3/version/{id}
func versionUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <version-id>",
		Short: "Update a project version",
		Example: `jira-agent version update 10001 --name "v1.1"
jira-agent version update 10001 --released`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
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
			if v, _ := cmd.Flags().GetString("release-date"); v != "" {
				body["releaseDate"] = v
			}
			if v, _ := cmd.Flags().GetString("start-date"); v != "" {
				body["startDate"] = v
			}
			if cmd.Flags().Changed("released") {
				val, _ := cmd.Flags().GetBool("released")
				body["released"] = val
			}
			if cmd.Flags().Changed("archived") {
				val, _ := cmd.Flags().GetBool("archived")
				body["archived"] = val
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/version/"+id, body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "New version name")
	cmd.Flags().String("description", "", "New version description")
	cmd.Flags().String("release-date", "", "Release date (YYYY-MM-DD)")
	cmd.Flags().String("start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().Bool("released", false, "Mark version as released")
	cmd.Flags().Bool("archived", false, "Mark version as archived")
	return cmd
}

// versionDeleteCommand deletes a project version.
// DELETE /rest/api/3/version/{id}
func versionDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <version-id>",
		Short:   "Delete a project version",
		Example: `jira-agent version delete 10001`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
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
	cmd.Flags().String("move-fix-issues-to", "", "Version ID to reassign fixVersion issues to")
	cmd.Flags().String("move-affected-issues-to", "", "Version ID to reassign affectedVersion issues to")
	return cmd
}

// versionMergeCommand merges one version into another, deleting the source.
// PUT /rest/api/3/version/{id}/mergeto/{moveIssuesTo}
func versionMergeCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "merge <source-version-id> <target-version-id>",
		Short:   "Merge a version into another (deletes the source version)",
		Example: `jira-agent version merge 10001 10002`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "source version ID", "target version ID")
			if err != nil {
				return err
			}

			if err := apiClient.Put(ctx, "/version/"+posArgs[0]+"/mergeto/"+posArgs[1], nil, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"sourceId": posArgs[0],
				"targetId": posArgs[1],
				"merged":   true,
			}, *format)
		}),
	}
	return cmd
}

// versionMoveCommand reorders a version within the project.
// POST /rest/api/3/version/{id}/move
func versionMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <version-id>",
		Short: "Move a version's position within the project",
		Example: `jira-agent version move 10001 --after 10002
jira-agent version move 10001 --position Earlier`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
			if err != nil {
				return err
			}

			body := map[string]any{}
			if v, _ := cmd.Flags().GetString("after"); v != "" {
				body["after"] = v
			}
			if v, _ := cmd.Flags().GetString("position"); v != "" {
				body["position"] = v
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/version/"+id+"/move", body, result)
			})
		}),
	}
	cmd.Flags().String("after", "", "URL of the version to place after")
	cmd.Flags().String("position", "", "Position keyword: Earlier, Later, First, Last")
	return cmd
}

// versionIssueCountsCommand returns related issue counts for a version.
// GET /rest/api/3/version/{id}/relatedIssueCounts
func versionIssueCountsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue-counts <version-id>",
		Short:   "Get related issue counts for a version",
		Example: `jira-agent version issue-counts 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/version/"+id+"/relatedIssueCounts", nil, result)
			})
		},
	}
	return cmd
}

// versionUnresolvedCountCommand returns unresolved issue counts for a version.
// GET /rest/api/3/version/{id}/unresolvedIssueCount
func versionUnresolvedCountCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unresolved-count <version-id>",
		Short:   "Get unresolved issue count for a version",
		Example: `jira-agent version unresolved-count 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := requireArg(args, "version ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/version/"+id+"/unresolvedIssueCount", nil, result)
			})
		},
	}
	return cmd
}
