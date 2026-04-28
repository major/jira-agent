package commands

import (
	"context"
	"io"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// ProjectCommand returns the top-level "project" command with project operations.
func ProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "project",
		Usage: "Project operations (list, get, roles, categories)",
		UsageText: `jira-agent project list
jira-agent project get PROJ
jira-agent project roles PROJ
jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project categories`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			projectListCommand(apiClient, w, format),
			projectGetCommand(apiClient, w, format),
			projectRolesCommand(apiClient, w, format, allowWrites),
			projectCategoriesCommand(apiClient, w, format),
		},
	}
}

// projectListCommand searches projects with pagination.
// GET /rest/api/3/project/search
func projectListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List projects (paginated, filterable)",
		UsageText: `jira-agent project list
jira-agent project list --expand lead,description`,
		Flags: appendPaginationFlagsWithUsage([]cli.Flag{
			&cli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Filter by project name or key (case-insensitive substring match)",
			},
			&cli.StringFlag{
				Name:  "type-key",
				Usage: "Filter by project type: business, service_desk, software",
			},
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Sort field (category, key, name, owner); prefix with - for descending",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (description, lead, issueTypes, url, insight)",
			},
		}, paginationFlagUsage{
			maxResults: "Page size (max 100)",
			startAt:    "Pagination offset",
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
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
}

// projectRolesCommand returns project-scoped role operations.
func projectRolesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "roles",
		Usage: "Work with project roles",
		UsageText: `jira-agent project roles PROJ
jira-agent project roles get PROJ 10000 --exclude-inactive-users
jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles remove-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2`,
		DefaultCommand: "list",
		ArgsUsage:      "<project-key>",
		Commands: []*cli.Command{
			projectRolesListCommand(apiClient, w, format),
			projectRoleCommand(apiClient, w, format),
			projectRoleAddActorCommand(apiClient, w, format, allowWrites),
			projectRoleRemoveActorCommand(apiClient, w, format, allowWrites),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectKey, err := requireArg(cmd, "project key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+escapePathSegment(projectKey)+"/role", nil, result)
			})
		},
	}
}

// POST /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleAddActorCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "add-actor",
		Usage: "Add actors to a project role",
		UsageText: `jira-agent project roles add-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles add-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2
jira-agent project roles add-actor PROJ 10000 --group "jira-developers"`,
		ArgsUsage: "<project-key> <role-id>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "user", Usage: "User account ID to add (repeatable)"},
			&cli.StringSliceFlag{Name: "group", Usage: "Group name to add (repeatable)"},
			&cli.StringSliceFlag{Name: "group-id", Usage: "Group ID to add (repeatable)"},
		},
		Metadata: writeCommandMetadata(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(args[1], "role ID")
			if err != nil {
				return err
			}

			body, err := buildRoleActorBody(cmd)
			if err != nil {
				return err
			}
			path := "/project/" + escapePathSegment(args[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
}

// DELETE /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleRemoveActorCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "remove-actor",
		Usage: "Remove an actor from a project role",
		UsageText: `jira-agent project roles remove-actor PROJ 10000 --user 5b10ac8d82e05b22cc7d4ef5
jira-agent project roles remove-actor PROJ 10000 --group-id 952d12c3-5b5b-4d04-bb32-44d383afc4b2
jira-agent project roles remove-actor PROJ 10000 --group "jira-developers"`,
		ArgsUsage: "<project-key> <role-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "user", Usage: "User account ID to remove"},
			&cli.StringFlag{Name: "group", Usage: "Group name to remove"},
			&cli.StringFlag{Name: "group-id", Usage: "Group ID to remove"},
		},
		Metadata: writeCommandMetadata(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(args[1], "role ID")
			if err != nil {
				return err
			}

			params, err := buildRoleActorRemoveParams(cmd)
			if err != nil {
				return err
			}
			path := "/project/" + escapePathSegment(args[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			if err := apiClient.Delete(ctx, appendQueryParams(path, params), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"removed": true}, *format)
		}),
	}
}

func buildRoleActorBody(cmd *cli.Command) (map[string][]string, error) {
	body := map[string][]string{}
	if users := cmd.StringSlice("user"); len(users) > 0 {
		body["user"] = users
	}
	if groups := cmd.StringSlice("group"); len(groups) > 0 {
		body["group"] = groups
	}
	if groupIDs := cmd.StringSlice("group-id"); len(groupIDs) > 0 {
		body["groupId"] = groupIDs
	}
	if len(body) == 0 {
		return nil, apperr.NewValidationError("at least one of --user, --group, or --group-id is required", nil)
	}
	return body, nil
}

func buildRoleActorRemoveParams(cmd *cli.Command) (map[string]string, error) {
	params := map[string]string{}
	if user := cmd.String("user"); user != "" {
		params["user"] = user
	}
	if group := cmd.String("group"); group != "" {
		params["group"] = group
	}
	if groupID := cmd.String("group-id"); groupID != "" {
		params["groupId"] = groupID
	}
	if len(params) != 1 {
		return nil, apperr.NewValidationError("exactly one of --user, --group, or --group-id is required", nil)
	}
	return params, nil
}

// projectRolesListCommand lists role URLs for a project.
// GET /rest/api/3/project/{projectIdOrKey}/role
func projectRolesListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List project role URLs",
		UsageText: `jira-agent project roles list PROJ`,
		ArgsUsage: "<project-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectKey, err := requireArg(cmd, "project key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/project/"+escapePathSegment(projectKey)+"/role", nil, result)
			})
		},
	}
}

// projectRoleCommand fetches a role assigned within a project.
// GET /rest/api/3/project/{projectIdOrKey}/role/{id}
func projectRoleCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get project role details",
		UsageText: `jira-agent project roles get PROJ 10000
jira-agent project roles get PROJ 10000 --exclude-inactive-users`,
		ArgsUsage: "<project-key> <role-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "exclude-inactive-users",
				Usage: "Exclude inactive users from role actors",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "project key", "role ID")
			if err != nil {
				return err
			}
			roleID, err := parsePositiveIntID(args[1], "role ID")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addBoolParam(cmd, params, "exclude-inactive-users", "excludeInactiveUsers")
			path := "/project/" + escapePathSegment(args[0]) + "/role/" + strconv.FormatInt(roleID, 10)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
}

// projectCategoriesCommand returns project category operations.
func projectCategoriesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "categories",
		Usage: "Project category operations",
		UsageText: `jira-agent project categories
jira-agent project categories get 10000`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			projectCategoriesListCommand(apiClient, w, format),
			projectCategoryCommand(apiClient, w, format),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/projectCategory", nil, result)
			})
		},
	}
}

// projectCategoriesListCommand lists all project categories.
// GET /rest/api/3/projectCategory
func projectCategoriesListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List project categories",
		UsageText: `jira-agent project categories list`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/projectCategory", nil, result)
			})
		},
	}
}

// projectCategoryCommand fetches one project category by ID.
// GET /rest/api/3/projectCategory/{id}
func projectCategoryCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get project category details",
		UsageText: `jira-agent project categories get 10000`,
		ArgsUsage: "<category-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			categoryID, err := requireArg(cmd, "category ID")
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
func projectGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get project details by key or ID",
		UsageText: `jira-agent project get PROJ
jira-agent project get PROJ --expand lead,description`,
		ArgsUsage: "<project-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (description, issueTypes, lead, projectKeys)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "project key")
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
}
