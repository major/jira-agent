package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// ProjectCommand returns the top-level "project" command with project operations.
func ProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project operations (list, get, roles, property, categories)",
		Example: `jira-agent project list
jira-agent project get PROJ
jira-agent project roles PROJ
jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project property get PROJ com.example.flag
jira-agent project categories`,
	}
	cmd.AddCommand(
		projectListCommand(apiClient, w, format),
		projectGetCommand(apiClient, w, format),
		projectRolesCommand(apiClient, w, format, allowWrites),
		projectPropertyCommand(apiClient, w, format, allowWrites),
		projectCategoriesCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// projectListCommand searches projects with pagination.
// GET /rest/api/3/project/search
func projectListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects (paginated, filterable)",
		Example: `jira-agent project list
jira-agent project list --expand lead,description`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"type-key": "typeKey",
				"order-by": "orderBy",
				"expand":   "expand",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/search", params, result)
			})
		},
	}
	cmd.Flags().StringP("query", "q", "", "Filter by project name or key (case-insensitive substring match)")
	cmd.Flags().String("type-key", "", "Filter by project type: business, service_desk, software")
	cmd.Flags().String("order-by", "", "Sort field (category, key, name, owner); prefix with - for descending")
	cmd.Flags().String("expand", "", "Comma-separated expansions (description, lead, issueTypes, url, insight)")
	appendPaginationFlagsWithUsage(cmd, paginationFlagUsage{
		maxResults: "Page size (max 100)",
		startAt:    "Pagination offset",
	})
	return cmd
}

// projectRolesCommand returns project-scoped role operations.
func projectRolesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Work with project roles",
		Example: `jira-agent project roles PROJ
jira-agent project roles get PROJ 10000 --exclude-inactive-users
jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles remove-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2`,
	}
	cmd.AddCommand(
		projectRolesListCommand(apiClient, w, format),
		projectRoleCommand(apiClient, w, format),
		projectRoleAddActorCommand(apiClient, w, format, allowWrites),
		projectRoleRemoveActorCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// POST /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleAddActorCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-actor <project-key> <role-id>",
		Short: "Add actors to a project role",
		Example: `jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles add-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2
jira-agent project roles add-actor PROJ 10000 --group "jira-developers"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(posArgs[1], "role ID")
			if err != nil {
				return err
			}

			body, err := buildRoleActorBody(cmd)
			if err != nil {
				return err
			}
			path := "/project/" + escapePathSegment(posArgs[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
	cmd.Flags().StringSlice("user", nil, "User account ID to add (repeatable)")
	cmd.Flags().StringSlice("group", nil, "Group name to add (repeatable)")
	cmd.Flags().StringSlice("group-id", nil, "Group ID to add (repeatable)")
	return cmd
}

// DELETE /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleRemoveActorCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-actor <project-key> <role-id>",
		Short: "Remove an actor from a project role",
		Example: `jira-agent project roles remove-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles remove-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2
jira-agent project roles remove-actor PROJ 10000 --group "jira-developers"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(posArgs[1], "role ID")
			if err != nil {
				return err
			}

			params, err := buildRoleActorRemoveParams(cmd)
			if err != nil {
				return err
			}
			path := "/project/" + escapePathSegment(posArgs[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			if err := apiClient.Delete(ctx, appendQueryParams(path, params), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"removed": true}, *format)
		}),
	}
	cmd.Flags().String("user", "", "User account ID to remove")
	cmd.Flags().String("group", "", "Group name to remove")
	cmd.Flags().String("group-id", "", "Group ID to remove")
	return cmd
}

func buildRoleActorBody(cmd *cobra.Command) (map[string][]string, error) {
	body := map[string][]string{}
	if users, _ := cmd.Flags().GetStringSlice("user"); len(users) > 0 {
		body["user"] = users
	}
	if groups, _ := cmd.Flags().GetStringSlice("group"); len(groups) > 0 {
		body["group"] = groups
	}
	if groupIDs, _ := cmd.Flags().GetStringSlice("group-id"); len(groupIDs) > 0 {
		body["groupId"] = groupIDs
	}
	if len(body) == 0 {
		return nil, apperr.NewValidationError("at least one of --user, --group, or --group-id is required", nil)
	}
	return body, nil
}

func buildRoleActorRemoveParams(cmd *cobra.Command) (map[string]string, error) {
	params := map[string]string{}
	if user, _ := cmd.Flags().GetString("user"); user != "" {
		params["user"] = user
	}
	if group, _ := cmd.Flags().GetString("group"); group != "" {
		params["group"] = group
	}
	if groupID, _ := cmd.Flags().GetString("group-id"); groupID != "" {
		params["groupId"] = groupID
	}
	if len(params) != 1 {
		return nil, apperr.NewValidationError("exactly one of --user, --group, or --group-id is required", nil)
	}
	return params, nil
}

// projectRolesListCommand lists role URLs for a project.
// GET /rest/api/3/project/{projectIdOrKey}/role
func projectRolesListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <project-key>",
		Short:   "List project role URLs",
		Example: `jira-agent project roles list PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectKey, err := requireArg(args, "project key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+escapePathSegment(projectKey)+"/role", nil, result)
			})
		},
	}
	return cmd
}

// projectRoleCommand fetches a role assigned within a project.
// GET /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-key> <role-id>",
		Short: "Get project role details",
		Example: `jira-agent project roles get PROJ 10000
jira-agent project roles get PROJ 10000 --exclude-inactive-users`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			posArgs, err := requireArgs(args, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(posArgs[1], "role ID")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addBoolParam(cmd, params, "exclude-inactive-users", "excludeInactiveUsers")
			path := "/project/" + escapePathSegment(posArgs[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
	cmd.Flags().Bool("exclude-inactive-users", false, "Exclude inactive users from role actors")
	return cmd
}

// projectCategoriesCommand returns project category operations.
func projectCategoriesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Project category operations",
		Example: `jira-agent project categories
jira-agent project categories get 10000`,
	}
	cmd.AddCommand(
		projectCategoriesListCommand(apiClient, w, format),
		projectCategoryCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// projectCategoriesListCommand lists all project categories.
// GET /rest/api/3/projectCategory
func projectCategoriesListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project categories",
		Example: `jira-agent project categories list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/projectCategory", nil, result)
			})
		},
	}
	return cmd
}

// projectCategoryCommand fetches one project category by ID.
// GET /rest/api/3/projectCategory/{id}
func projectCategoryCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <category-id>",
		Short:   "Get project category details",
		Example: `jira-agent project categories get 10000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			categoryID, err := requireArg(args, "category ID")
			if err != nil {
				return err
			}
			parsedCategoryID, err := parsePositiveIntID(categoryID, "category ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/projectCategory/"+strconv.FormatInt(parsedCategoryID, 10), nil, result)
			})
		},
	}
	return cmd
}

func writeArrayAPIResult(w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}

	meta := output.NewMetadata()
	if items, ok := result.([]any); ok {
		meta.Returned = len(items)
	}
	return output.WriteSuccess(w, result, meta, format)
}

// projectGetCommand fetches a single project by key or ID.
// GET /rest/api/3/project/{projectIdOrKey}
func projectGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-key>",
		Short: "Get project details by key or ID",
		Example: `jira-agent project get PROJ
jira-agent project get PROJ --expand lead,description`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "project key")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+key, params, result)
			})
		},
	}
	cmd.Flags().String("expand", "", "Comma-separated expansions (description, issueTypes, lead, projectKeys)")
	return cmd
}
