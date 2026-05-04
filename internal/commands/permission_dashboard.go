package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// DashboardCommand returns the top-level "dashboard" command for Jira dashboards.
func DashboardCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Work with Jira dashboards",
		Example: `jira-agent dashboard list
jira-agent dashboard get 10001
jira-agent dashboard gadgets 10001`,
	}
	cmd.AddCommand(
		dashboardListCommand(apiClient, w, format),
		dashboardGetCommand(apiClient, w, format),
		dashboardGadgetsCommand(apiClient, w, format),
		dashboardCreateCommand(apiClient, w, format, allowWrites),
		dashboardUpdateCommand(apiClient, w, format, allowWrites),
		dashboardDeleteCommand(apiClient, w, format, allowWrites),
		dashboardCopyCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// PermissionCommand returns the top-level "permission" command for Jira permissions.
func PermissionCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Work with Jira permissions",
		Example: `jira-agent permission check --permissions BROWSE_PROJECTS,CREATE_ISSUES
jira-agent permission list
jira-agent permission schemes list`,
	}
	cmd.AddCommand(
		permissionCheckCommand(apiClient, w, format),
		permissionListCommand(apiClient, w, format),
		permissionSchemeCommand(apiClient, w, format),
	)
	return cmd
}

func dashboardListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		Example: `jira-agent dashboard list
jira-agent dashboard list --filter my
jira-agent dashboard list --search "ops"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			search, _ := cmd.Flags().GetString("search")
			filter, _ := cmd.Flags().GetString("filter")

			if search != "" && filter != "" {
				return apperr.NewValidationError("--search and --filter cannot be used together; choose one", nil)
			}

			if search != "" {
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
	cmd.Flags().String("filter", "", "Dashboard filter: my or favorite")
	cmd.Flags().String("search", "", "Search dashboards by name substring")
	appendPaginationFlags(cmd)
	return cmd
}

func dashboardGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <dashboard-id>",
		Short:   "Get a dashboard",
		Example: `jira-agent dashboard get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dashboardID, err := requireArg(args, "dashboard ID")
			if err != nil {
				return err
			}
			dashboardIDPath := escapePathSegment(dashboardID)

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/dashboard/"+dashboardIDPath, nil, result)
			})
		},
	}
	return cmd
}

func dashboardGadgetsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gadgets <dashboard-id>",
		Short:   "List dashboard gadgets",
		Example: `jira-agent dashboard gadgets 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dashboardID, err := requireArg(args, "dashboard ID")
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
	cmd.Flags().String("module-key", "", "Filter by gadget module key")
	cmd.Flags().String("uri", "", "Filter by gadget URI")
	cmd.Flags().String("gadget-id", "", "Filter by gadget ID")
	return cmd
}

func dashboardCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a dashboard",
		Example: `jira-agent dashboard create --name "Team dashboard"
jira-agent dashboard create --name "Team dashboard" --share-permissions-json '[{"type":"authenticated"}]'
jira-agent dashboard create --body-json '{"name":"Team dashboard","sharePermissions":[],"editPermissions":[]}'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	addDashboardBodyFlags(cmd)
	return cmd
}

func dashboardUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <dashboard-id>",
		Short: "Update a dashboard",
		Example: `jira-agent dashboard update 10001 --name "Team dashboard"
jira-agent dashboard update 10001 --name "Team dashboard" --description "Team metrics"
jira-agent dashboard update 10001 --body-json '{"name":"Team dashboard","sharePermissions":[],"editPermissions":[]}'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dashboardID, err := requireArg(args, "dashboard ID")
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
	addDashboardBodyFlags(cmd)
	return cmd
}

func dashboardDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <dashboard-id>",
		Short:   "Delete a dashboard",
		Example: `jira-agent dashboard delete 10001`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dashboardID, err := requireArg(args, "dashboard ID")
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
	return cmd
}

func dashboardCopyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy <dashboard-id>",
		Short: "Copy a dashboard",
		Example: `jira-agent dashboard copy 10001 --name "Team dashboard copy"
jira-agent dashboard copy 10001 --name "Team dashboard copy" --extend-admin-permissions`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dashboardID, err := requireArg(args, "dashboard ID")
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
	addDashboardBodyFlags(cmd)
	return cmd
}

func addDashboardBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "Dashboard name")
	cmd.Flags().String("description", "", "Dashboard description")
	cmd.Flags().String("share-permissions-json", "[]", "Dashboard sharePermissions JSON array")
	cmd.Flags().String("edit-permissions-json", "[]", "Dashboard editPermissions JSON array")
	cmd.Flags().String("body-json", "", "Raw dashboard JSON body")
	cmd.Flags().Bool("extend-admin-permissions", false, "Extend admin permissions for the operation")
}

func buildDashboardBody(cmd *cobra.Command) (map[string]any, error) {
	body := map[string]any{}
	raw, _ := cmd.Flags().GetString("body-json")
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return nil, apperr.NewValidationError(fmt.Sprintf("invalid --body-json: %v", err), err)
		}
	}
	name, _ := cmd.Flags().GetString("name")
	if name != "" {
		body["name"] = name
	}
	desc, _ := cmd.Flags().GetString("description")
	if desc != "" {
		body["description"] = desc
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

func parseJSONArrayFlag(cmd *cobra.Command, flagName string) ([]any, error) {
	var value []any
	raw, _ := cmd.Flags().GetString(flagName)
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, apperr.NewValidationError(fmt.Sprintf("invalid --%s: %v", flagName, err), err)
	}
	return value, nil
}

func permissionCheckCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check permissions for the current user",
		Example: `jira-agent permission check --permissions BROWSE_PROJECTS,CREATE_ISSUES
jira-agent permission check --permissions ADMINISTER_PROJECTS --project-key PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			permissions, _ := cmd.Flags().GetString("permissions")
			params := map[string]string{"permissions": normalizeCSV(permissions)}
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
	cmd.Flags().String("permissions", "", "Comma-separated permission keys")
	cmd.Flags().String("project-key", "", "Project key context")
	cmd.Flags().String("project-id", "", "Project ID context")
	cmd.Flags().String("issue-key", "", "Issue key context")
	cmd.Flags().String("issue-id", "", "Issue ID context")
	cmd.Flags().String("comment-id", "", "Comment ID context")
	_ = cmd.MarkFlagRequired("permissions")
	return cmd
}

func permissionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Jira permissions",
		Example: `jira-agent permission list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/permissions", nil, result)
			})
		},
	}
	return cmd
}

func permissionSchemeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schemes",
		Short: "Work with Jira permission schemes",
		Example: `jira-agent permission schemes list
jira-agent permission schemes get 10001`,
	}
	cmd.AddCommand(
		permissionSchemeListCommand(apiClient, w, format),
		permissionSchemeGetCommand(apiClient, w, format),
		permissionSchemeProjectCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func permissionSchemeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List permission schemes",
		Example: `jira-agent permission schemes list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	return cmd
}

func permissionSchemeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a permission scheme",
		Example: `jira-agent permission schemes get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			schemeID, err := requireNumericArg(args, "scheme ID")
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
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	return cmd
}

func permissionSchemeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Get the permission scheme assigned to a project",
		Example: `jira-agent permission schemes project PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectKeyOrID, err := requireArg(args, "project key or ID")
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
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	return cmd
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

func requireNumericArg(args []string, label string) (string, error) {
	value, err := requireArg(args, label)
	if err != nil {
		return "", err
	}
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return "", apperr.NewValidationError(label+" must be numeric", err)
	}
	return value, nil
}
