package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// DashboardCommand returns the top-level "dashboard" command for Jira dashboards.
func DashboardCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "dashboard",
		Usage: "Work with Jira dashboards",
		UsageText: `jira-agent dashboard list
jira-agent dashboard get 10001
jira-agent dashboard gadgets 10001`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			dashboardListCommand(apiClient, w, format),
			dashboardGetCommand(apiClient, w, format),
			dashboardGadgetsCommand(apiClient, w, format),
			dashboardCreateCommand(apiClient, w, format, allowWrites),
			dashboardUpdateCommand(apiClient, w, format, allowWrites),
			dashboardDeleteCommand(apiClient, w, format, allowWrites),
			dashboardCopyCommand(apiClient, w, format, allowWrites),
		},
	}
}

// PermissionCommand returns the top-level "permission" command for Jira permissions.
func PermissionCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "permission",
		Usage: "Work with Jira permissions",
		UsageText: `jira-agent permission check --permissions BROWSE_PROJECTS,CREATE_ISSUES
jira-agent permission list
jira-agent permission schemes list`,
		Commands: []*cli.Command{
			permissionCheckCommand(apiClient, w, format),
			permissionListCommand(apiClient, w, format),
			permissionSchemeCommand(apiClient, w, format),
		},
	}
}

func dashboardListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List dashboards",
		UsageText: `jira-agent dashboard list
jira-agent dashboard list --filter my
jira-agent dashboard list --search "ops"`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{Name: "filter", Usage: "Dashboard filter: my or favorite"},
			&cli.StringFlag{Name: "search", Usage: "Search dashboards by name substring"},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.String("search") != "" && cmd.String("filter") != "" {
				return apperr.NewValidationError("--search and --filter cannot be used together; choose one", nil)
			}

			if cmd.String("search") != "" {
				params := buildPaginationParams(cmd, map[string]string{"search": "searchTerm"})
				return writePaginatedAPIResult(w, *format, func(result any) error {
					return apiClient.Get(ctx, "/dashboard/search", params, result)
				})
			}

			params := buildPaginationParams(cmd, map[string]string{"filter": "filter"})
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/dashboard", params, result)
			})
		},
	}
}

func dashboardGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a dashboard",
		UsageText: `jira-agent dashboard get 10001`,
		ArgsUsage: "<dashboard-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dashboardID, err := requireArg(cmd, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/dashboard/"+dashboardIDPath, nil, result)
			})
		},
	}
}

func dashboardGadgetsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "gadgets",
		Usage:     "List dashboard gadgets",
		UsageText: `jira-agent dashboard gadgets 10001`,
		ArgsUsage: "<dashboard-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "module-key", Usage: "Filter by gadget module key"},
			&cli.StringFlag{Name: "uri", Usage: "Filter by gadget URI"},
			&cli.StringFlag{Name: "gadget-id", Usage: "Filter by gadget ID"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dashboardID, err := requireArg(cmd, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"module-key": "moduleKey", "uri": "uri", "gadget-id": "gadgetId"})

			var result any
			if err := apiClient.Get(ctx, "/dashboard/"+dashboardIDPath+"/gadget", params, &result); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = countNamedArray(result, "gadgets")
			meta.Total = meta.Returned
			return output.WriteSuccess(w, result, meta, *format)
		},
	}
}

func dashboardCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a dashboard",
		UsageText: `jira-agent dashboard create --name "Team dashboard"
jira-agent dashboard create --name "Team dashboard" --share-permissions-json '[{"type":"authenticated"}]'
jira-agent dashboard create --body-json '{"name":"Team dashboard","sharePermissions":[],"editPermissions":[]}'`,
		Flags: dashboardBodyFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body, err := buildDashboardBody(cmd)
			if err != nil {
				return err
			}
			params := map[string]string{}
			addBoolParam(cmd, params, "extend-admin-permissions", "extendAdminPermissions")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, appendQueryParams("/dashboard", params), body, result)
			})
		}),
	}
}

func dashboardUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update a dashboard",
		UsageText: `jira-agent dashboard update 10001 --name "Team dashboard"
jira-agent dashboard update 10001 --name "Team dashboard" --description "Team metrics"
jira-agent dashboard update 10001 --body-json '{"name":"Team dashboard","sharePermissions":[],"editPermissions":[]}'`,
		ArgsUsage: "<dashboard-id>",
		Flags:     dashboardBodyFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			dashboardID, err := requireArg(cmd, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)
			body, err := buildDashboardBody(cmd)
			if err != nil {
				return err
			}
			params := map[string]string{}
			addBoolParam(cmd, params, "extend-admin-permissions", "extendAdminPermissions")

			path := appendQueryParams("/dashboard/"+dashboardIDPath, params)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
}

func dashboardDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a dashboard",
		UsageText: `jira-agent dashboard delete 10001`,
		ArgsUsage: "<dashboard-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			dashboardID, err := requireArg(cmd, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)
			if err := apiClient.Delete(ctx, "/dashboard/"+dashboardIDPath, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"dashboardId": dashboardID, "deleted": true}, *format)
		}),
	}
}

func dashboardCopyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "copy",
		Usage: "Copy a dashboard",
		UsageText: `jira-agent dashboard copy 10001 --name "Team dashboard copy"
jira-agent dashboard copy 10001 --name "Team dashboard copy" --extend-admin-permissions`,
		ArgsUsage: "<dashboard-id>",
		Flags:     dashboardBodyFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			dashboardID, err := requireArg(cmd, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)
			body, err := buildDashboardBody(cmd)
			if err != nil {
				return err
			}
			params := map[string]string{}
			addBoolParam(cmd, params, "extend-admin-permissions", "extendAdminPermissions")

			path := appendQueryParams("/dashboard/"+dashboardIDPath+"/copy", params)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		}),
	}
}

func dashboardBodyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "name", Usage: "Dashboard name"},
		&cli.StringFlag{Name: "description", Usage: "Dashboard description"},
		&cli.StringFlag{Name: "share-permissions-json", Usage: "Dashboard sharePermissions JSON array", Value: "[]"},
		&cli.StringFlag{Name: "edit-permissions-json", Usage: "Dashboard editPermissions JSON array", Value: "[]"},
		&cli.StringFlag{Name: "body-json", Usage: "Raw dashboard JSON body"},
		&cli.BoolFlag{Name: "extend-admin-permissions", Usage: "Extend admin permissions for the operation"},
	}
}

func buildDashboardBody(cmd *cli.Command) (map[string]any, error) {
	body := map[string]any{}
	if raw := cmd.String("body-json"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return nil, apperr.NewValidationError(fmt.Sprintf("invalid --body-json: %v", err), err)
		}
	}
	if v := cmd.String("name"); v != "" {
		body["name"] = v
	}
	if v := cmd.String("description"); v != "" {
		body["description"] = v
	}
	if body["name"] == nil {
		return nil, apperr.NewValidationError("--name is required", nil)
	}

	sharePermissions, err := parseJSONArrayFlag(cmd, "share-permissions-json")
	if err != nil {
		return nil, err
	}
	editPermissions, err := parseJSONArrayFlag(cmd, "edit-permissions-json")
	if err != nil {
		return nil, err
	}
	if body["sharePermissions"] == nil {
		body["sharePermissions"] = sharePermissions
	}
	if body["editPermissions"] == nil {
		body["editPermissions"] = editPermissions
	}
	return body, nil
}

func parseJSONArrayFlag(cmd *cli.Command, flagName string) ([]any, error) {
	var value []any
	if err := json.Unmarshal([]byte(cmd.String(flagName)), &value); err != nil {
		return nil, apperr.NewValidationError(fmt.Sprintf("invalid --%s: %v", flagName, err), err)
	}
	return value, nil
}

func permissionCheckCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "Check permissions for the current user",
		UsageText: `jira-agent permission check --permissions BROWSE_PROJECTS,CREATE_ISSUES
jira-agent permission check --permissions ADMINISTER_PROJECTS --project-key PROJ`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "permissions", Usage: "Comma-separated permission keys", Required: true},
			&cli.StringFlag{Name: "project-key", Usage: "Project key context"},
			&cli.StringFlag{Name: "project-id", Usage: "Project ID context"},
			&cli.StringFlag{Name: "issue-key", Usage: "Issue key context"},
			&cli.StringFlag{Name: "issue-id", Usage: "Issue ID context"},
			&cli.StringFlag{Name: "comment-id", Usage: "Comment ID context"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{"permissions": normalizeCSV(cmd.String("permissions"))}
			addOptionalParams(cmd, params, map[string]string{
				"project-key": "projectKey",
				"project-id":  "projectId",
				"issue-key":   "issueKey",
				"issue-id":    "issueId",
				"comment-id":  "commentId",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/mypermissions", params, result)
			})
		},
	}
}

func permissionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List Jira permissions",
		UsageText: `jira-agent permission list`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/permissions", nil, result)
			})
		},
	}
}

func permissionSchemeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "schemes",
		Usage: "Work with Jira permission schemes",
		UsageText: `jira-agent permission schemes list
jira-agent permission schemes get 10001`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			permissionSchemeListCommand(apiClient, w, format),
			permissionSchemeGetCommand(apiClient, w, format),
			permissionSchemeProjectCommand(apiClient, w, format),
		},
	}
}

func permissionSchemeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List permission schemes",
		UsageText: `jira-agent permission schemes list`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			var result any
			if err := apiClient.Get(ctx, "/permissionscheme", params, &result); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = countNamedArray(result, "permissionSchemes")
			meta.Total = meta.Returned
			return output.WriteSuccess(w, result, meta, *format)
		},
	}
}

func permissionSchemeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a permission scheme",
		UsageText: `jira-agent permission schemes get 10001`,
		ArgsUsage: "<scheme-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			schemeID, err := requireNumericArg(cmd, "scheme ID")
			if err != nil {
				return err
			}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/permissionscheme/"+schemeID, params, result)
			})
		},
	}
}

func permissionSchemeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "project",
		Usage:     "Get the permission scheme assigned to a project",
		UsageText: `jira-agent permission schemes project PROJ`,
		ArgsUsage: "<project-key-or-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectKeyOrID, err := requireArg(cmd, "project key or ID")
			if err != nil {
				return err
			}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			path := "/project/" + projectKeyOrID + "/permissionscheme"
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, path, params, result)
			})
		},
	}
}

func countNamedArray(result any, key string) int {
	object, ok := result.(map[string]any)
	if !ok {
		return 0
	}
	values, ok := object[key].([]any)
	if !ok {
		return 0
	}
	return len(values)
}

func normalizeCSV(value string) string {
	return strings.Join(splitTrimmed(value), ",")
}

func requireNumericArg(cmd *cli.Command, label string) (string, error) {
	value, err := requireArg(cmd, label)
	if err != nil {
		return "", err
	}
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return "", apperr.NewValidationError(label+" must be numeric", err)
	}
	return value, nil
}
