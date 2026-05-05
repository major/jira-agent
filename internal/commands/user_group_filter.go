package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// UserCommand returns the top-level "user" command for Jira user lookups.
func UserCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Work with Jira users",
		Example: `jira-agent user search --query "john"
jira-agent user get --account-id 5b10a2844c20165700ede21g
jira-agent user groups --account-id 5b10a2844c20165700ede21g`,
	}
	cmd.AddCommand(
		userGetCommand(apiClient, w, format),
		userSearchCommand(apiClient, w, format),
		userGroupsCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "search")
	return cmd
}

// GroupCommand returns the top-level "group" command for Jira groups.
func GroupCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Work with Jira groups",
		Example: `jira-agent group list
jira-agent group get --groupname "jira-users"
jira-agent group members --groupname "jira-users"`,
	}
	cmd.AddCommand(
		groupListCommand(apiClient, w, format),
		groupGetCommand(apiClient, w, format),
		groupCreateCommand(apiClient, w, format, allowWrites),
		groupDeleteCommand(apiClient, w, format, allowWrites),
		groupMembersCommand(apiClient, w, format),
		groupAddMemberCommand(apiClient, w, format, allowWrites),
		groupRemoveMemberCommand(apiClient, w, format, allowWrites),
		groupMemberPickerCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// FilterCommand returns the top-level "filter" command for Jira filters.
func FilterCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Work with Jira filters",
		Example: `jira-agent filter list
jira-agent filter get 10001
jira-agent filter favorites`,
	}
	cmd.AddCommand(
		filterListCommand(apiClient, w, format),
		filterGetCommand(apiClient, w, format),
		filterCreateCommand(apiClient, w, format, allowWrites),
		filterUpdateCommand(apiClient, w, format, allowWrites),
		filterDeleteCommand(apiClient, w, format, allowWrites),
		filterFavoritesCommand(apiClient, w, format),
		filterPermissionsCommand(apiClient, w, format),
		filterShareCommand(apiClient, w, format, allowWrites),
		filterUnshareCommand(apiClient, w, format, allowWrites),
		filterDefaultShareScopeCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func userGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a user by account ID",
		Example: `jira-agent user get --account-id 5b10a2844c20165700ede21g`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			accountID, _ := cmd.Flags().GetString("account-id")
			params := map[string]string{"accountId": accountID}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/user", params, result)
			})
		},
	}
	cmd.Flags().String("account-id", "", "User account ID")
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func userSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search users",
		Example: `jira-agent user search --query "john"
jira-agent user search --query "john.doe@example.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, map[string]string{
				"query":      "query",
				"account-id": "accountId",
				"expand":     "expand",
			})

			var users []any
			if err := apiClient.Get(ctx, "/users/search", params, &users); err != nil {
				return err
			}
			return output.WriteSuccess(w, users, output.NewMetadata(), *format)
		},
	}
	cmd.Flags().String("query", "", "User search query")
	cmd.Flags().String("account-id", "", "Exact account ID")
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	appendPaginationFlags(cmd)
	return cmd
}

func userGroupsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "groups",
		Short:   "List groups for a user",
		Example: `jira-agent user groups --account-id 5b10a2844c20165700ede21g`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			accountID, _ := cmd.Flags().GetString("account-id")
			var groups []any
			if err := apiClient.Get(ctx, "/user/groups", map[string]string{"accountId": accountID}, &groups); err != nil {
				return err
			}
			return output.WriteSuccess(w, groups, output.NewMetadata(), *format)
		},
	}
	cmd.Flags().String("account-id", "", "User account ID")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func groupListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Find groups",
		Example: `jira-agent group list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildMaxResultsParams(cmd, map[string]string{"query": "query"})
			addBoolParam(cmd, params, "case-insensitive", "caseInsensitive")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/groups/picker", params, result)
			})
		},
	}
	cmd.Flags().String("query", "", "Group name query")
	cmd.Flags().Int("max-results", 50, "Page size")
	cmd.Flags().Bool("case-insensitive", false, "Search case-insensitively")
	return cmd
}

func groupGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get group details",
		Example: `jira-agent group get --groupname "jira-users"
jira-agent group get --group-id abc-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/group", params, result)
			})
		},
	}
	addGroupIdentityFlags(cmd)
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	return cmd
}

func groupCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "create <group-name>",
		Short:   "Create a group",
		Example: `jira-agent group create "new-team"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := requireArg(args, "group name")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/group", map[string]any{"name": name}, result)
			})
		}),
	}
}

func groupDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a group",
		Example: `jira-agent group delete --groupname "old-team"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			addOptionalParams(cmd, params, map[string]string{"swap-group": "swapGroup", "swap-group-id": "swapGroupId"})

			if err := apiClient.Delete(ctx, appendQueryParams("/group", params), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"deleted": true}, *format)
		}),
	}
	addGroupIdentityFlags(cmd)
	cmd.Flags().String("swap-group", "", "Group name receiving transferred restrictions")
	cmd.Flags().String("swap-group-id", "", "Group ID receiving transferred restrictions")
	return cmd
}

func groupMembersCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "members",
		Short:   "List members of a group",
		Example: `jira-agent group members --groupname "jira-users"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			maps.Copy(params, buildPaginationParams(cmd, nil))
			addBoolParam(cmd, params, "include-inactive", "includeInactiveUsers")

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/group/member", params, result)
			})
		},
	}
	addGroupIdentityFlags(cmd)
	cmd.Flags().Bool("include-inactive", false, "Include inactive users")
	appendPaginationFlags(cmd)
	return cmd
}

func groupAddMemberCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add-member",
		Short:   "Add a user to a group",
		Example: `jira-agent group add-member --groupname "dev-team" --account-id 5b10a2844c20165700ede21g`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			accountID, _ := cmd.Flags().GetString("account-id")
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, appendQueryParams("/group/user", params), map[string]any{"accountId": accountID}, result)
			})
		}),
	}
	addGroupIdentityFlags(cmd)
	cmd.Flags().String("account-id", "", "User account ID")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func groupRemoveMemberCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-member",
		Short:   "Remove a user from a group",
		Example: `jira-agent group remove-member --groupname "dev-team" --account-id 5b10a2844c20165700ede21g`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			accountID, _ := cmd.Flags().GetString("account-id")
			params["accountId"] = accountID
			if err := apiClient.Delete(ctx, appendQueryParams("/group/user", params), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"accountId": accountID, "removed": true}, *format)
		}),
	}
	addGroupIdentityFlags(cmd)
	cmd.Flags().String("account-id", "", "User account ID")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func groupMemberPickerCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "member-picker",
		Short:   "Search users and groups for pickers",
		Example: `jira-agent group member-picker --query "john"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"query": "query", "issue-key": "issueKey", "project-id": "projectId"})
			addBoolParam(cmd, params, "show-avatar", "showAvatar")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/groupuserpicker", params, result)
			})
		},
	}
	cmd.Flags().String("query", "", "Picker query")
	cmd.Flags().String("issue-key", "", "Issue key for visibility context")
	cmd.Flags().String("project-id", "", "Project ID for visibility context")
	cmd.Flags().Bool("show-avatar", false, "Include avatar URLs")
	return cmd
}

func filterListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"search"},
		Short:   "Search filters",
		Example: `jira-agent filter list
jira-agent filter list --expand owner,jql`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, map[string]string{
				"name":       "filterName",
				"account-id": "accountId",
				"groupname":  "groupname",
				"group-id":   "groupId",
				"project-id": "projectId",
				"order-by":   "orderBy",
				"expand":     "expand",
			})
			addBoolParam(cmd, params, "override-share-permissions", "overrideSharePermissions")
			addBoolParam(cmd, params, "substring", "isSubstringMatch")

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/filter/search", params, result)
			})
		},
	}
	cmd.Flags().String("name", "", "Filter name query")
	cmd.Flags().String("account-id", "", "Owner account ID")
	cmd.Flags().String("groupname", "", "Shared group name")
	cmd.Flags().String("group-id", "", "Shared group ID")
	cmd.Flags().String("project-id", "", "Shared project ID")
	cmd.Flags().String("order-by", "", "Sort field")
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	cmd.Flags().Bool("override-share-permissions", false, "Override share permission filtering")
	cmd.Flags().Bool("substring", false, "Use substring matching for --name")
	appendPaginationFlags(cmd)
	return cmd
}

func filterGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <filter-id>",
		Short:   "Get a filter",
		Example: `jira-agent filter get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			addBoolParam(cmd, params, "override-share-permissions", "overrideSharePermissions")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/filter/"+filterIDPath, params, result)
			})
		},
	}
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	cmd.Flags().Bool("override-share-permissions", false, "Override share permission filtering")
	return cmd
}

func filterCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a filter",
		Example: `jira-agent filter create --name "My open bugs" --jql "project = PROJ AND type = Bug AND status = Open"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := buildFilterBody(cmd, true)
			if err != nil {
				return err
			}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			addBoolParam(cmd, params, "override-share-permissions", "overrideSharePermissions")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, appendQueryParams("/filter", params), body, result)
			})
		}),
	}
	addFilterBodyFlags(cmd)
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	cmd.Flags().Bool("override-share-permissions", false, "Override share permission checks")
	return cmd
}

func filterUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <filter-id>",
		Short:   "Update a filter",
		Example: `jira-agent filter update 10001 --name "Updated filter" --jql "project = PROJ AND status != Done"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)
			body, err := buildFilterBody(cmd, false)
			if err != nil {
				return err
			}
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})
			addBoolParam(cmd, params, "override-share-permissions", "overrideSharePermissions")

			path := appendQueryParams("/filter/"+filterIDPath, params)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
	addFilterBodyFlags(cmd)
	cmd.Flags().String("expand", "", "Comma-separated expansions")
	cmd.Flags().Bool("override-share-permissions", false, "Override share permission checks")
	return cmd
}

func filterDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <filter-id>",
		Short:   "Delete a filter",
		Example: `jira-agent filter delete 10001`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)
			if err := apiClient.Delete(ctx, "/filter/"+filterIDPath, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"filterId": filterID, "deleted": true}, *format)
		}),
	}
}

func filterFavoritesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "favorites",
		Short:   "List favorite filters",
		Example: `jira-agent filter favorites`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var filters []any
			if err := apiClient.Get(ctx, "/filter/favou"+"rite", nil, &filters); err != nil {
				return err
			}
			return output.WriteSuccess(w, filters, output.NewMetadata(), *format)
		},
	}
}

func filterPermissionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "permissions <filter-id>",
		Short:   "List filter share permissions",
		Example: `jira-agent filter permissions 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)

			var permissions []any
			if err := apiClient.Get(ctx, "/filter/"+filterIDPath+"/permission", nil, &permissions); err != nil {
				return err
			}
			return output.WriteSuccess(w, permissions, output.NewMetadata(), *format)
		},
	}
}

func filterShareCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share <filter-id>",
		Short: "Add a filter share permission",
		Example: `jira-agent filter share 10001 --with user:5b10a2844c20165700ede21g
jira-agent filter share 10001 --with group:team-group-id
jira-agent filter share 10001 --type project --project-id 10000`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)
			body, err := buildFilterShareBody(cmd)
			if err != nil {
				return err
			}

			var permissions []any
			if err := apiClient.Post(ctx, "/filter/"+filterIDPath+"/permission", body, &permissions); err != nil {
				return err
			}
			return output.WriteSuccess(w, permissions, output.NewMetadata(), *format)
		}),
	}
	addFilterShareFlags(cmd)
	return cmd
}

func filterUnshareCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unshare <filter-id>",
		Short:   "Remove a filter share permission",
		Example: `jira-agent filter unshare 10001 --permission-id 10002`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			permissionID, _ := cmd.Flags().GetString("permission-id")
			filterIDPath := escapePathSegment(filterID)
			permissionIDPath := escapePathSegment(permissionID)
			if err := apiClient.Delete(ctx, "/filter/"+filterIDPath+"/permission/"+permissionIDPath, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"filterId": filterID, "permissionId": permissionID, "deleted": true}, *format)
		}),
	}
	cmd.Flags().String("permission-id", "", "Share permission ID")
	_ = cmd.MarkFlagRequired("permission-id")
	return cmd
}

func filterDefaultShareScopeCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default-share-scope",
		Short: "Work with the default filter share scope",
		Example: `jira-agent filter default-share-scope get
jira-agent filter default-share-scope set --scope PRIVATE`,
	}
	cmd.AddCommand(
		filterDefaultShareScopeGetCommand(apiClient, w, format),
		filterDefaultShareScopeSetCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "get")
	return cmd
}

func filterDefaultShareScopeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Short:   "Get the default filter share scope",
		Example: `jira-agent filter default-share-scope get`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/filter/defaultShareScope", nil, result)
			})
		},
	}
}

func filterDefaultShareScopeSetCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Set the default filter share scope",
		Example: `jira-agent filter default-share-scope set --scope PRIVATE`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			scope, _ := cmd.Flags().GetString("scope")
			if err := validateFilterShareScope(scope); err != nil {
				return err
			}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/filter/defaultShareScope", map[string]any{"scope": scope}, result)
			})
		}),
	}
	cmd.Flags().String("scope", "", "Default scope: GLOBAL, AUTHENTICATED, or PRIVATE")
	_ = cmd.MarkFlagRequired("scope")
	return cmd
}

func addGroupIdentityFlags(cmd *cobra.Command) {
	cmd.Flags().String("groupname", "", "Group name")
	cmd.Flags().String("group-id", "", "Group ID")
}

func groupIdentityParams(cmd *cobra.Command) (map[string]string, error) {
	groupName, _ := cmd.Flags().GetString("groupname")
	groupID, _ := cmd.Flags().GetString("group-id")
	switch {
	case groupName == "" && groupID == "":
		return nil, apperr.NewValidationError("--groupname or --group-id is required", nil)
	case groupName != "" && groupID != "":
		return nil, apperr.NewValidationError("--groupname and --group-id cannot be used together", nil)
	case groupName != "":
		return map[string]string{"groupname": groupName}, nil
	default:
		return map[string]string{"groupId": groupID}, nil
	}
}

func addFilterBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "Filter name")
	cmd.Flags().String("description", "", "Filter description")
	cmd.Flags().String("jql", "", "Filter JQL")
	cmd.Flags().String("body-json", "", "Raw filter JSON body")
}

func buildFilterBody(cmd *cobra.Command, requireNameAndJQL bool) (map[string]any, error) {
	body := map[string]any{}
	if raw, _ := cmd.Flags().GetString("body-json"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return nil, apperr.NewValidationError(fmt.Sprintf("invalid --body-json: %v", err), err)
		}
	}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		body["name"] = v
	}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		body["description"] = v
	}
	if v, _ := cmd.Flags().GetString("jql"); v != "" {
		body["jql"] = v
	}
	if requireNameAndJQL {
		if body["name"] == nil {
			return nil, apperr.NewValidationError("--name is required", nil)
		}
		if body["jql"] == nil {
			return nil, apperr.NewValidationError("--jql is required", nil)
		}
	} else if len(body) == 0 {
		return nil, apperr.NewValidationError("at least one filter field is required", nil)
	}
	return body, nil
}

func addFilterShareFlags(cmd *cobra.Command) {
	cmd.Flags().String("with", "", "Permission shorthand: global, authenticated, user:<account-id>, group:<group-id>, groupname:<name>, project:<project-id>, project-role:<project-id>:<role-id>")
	cmd.Flags().String("type", "", "Share permission type")
	cmd.Flags().String("account-id", "", "User account ID")
	cmd.Flags().String("group-id", "", "Group ID")
	cmd.Flags().String("groupname", "", "Group name")
	cmd.Flags().String("project-id", "", "Project ID")
	cmd.Flags().String("project-role-id", "", "Project role ID")
	cmd.Flags().Int("rights", 0, "Share rights value")
	cmd.Flags().String("body-json", "", "Raw share permission JSON body")
}

func buildFilterShareBody(cmd *cobra.Command) (map[string]any, error) {
	body := map[string]any{}
	if raw, _ := cmd.Flags().GetString("body-json"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return nil, apperr.NewValidationError(fmt.Sprintf("invalid --body-json: %v", err), err)
		}
	}
	if shorthand, _ := cmd.Flags().GetString("with"); shorthand != "" {
		parsed, err := parseFilterShareShorthand(shorthand)
		if err != nil {
			return nil, err
		}
		maps.Copy(body, parsed)
	}
	if v, _ := cmd.Flags().GetString("type"); v != "" {
		body["type"] = v
	}
	if v, _ := cmd.Flags().GetString("account-id"); v != "" {
		body["accountId"] = v
	}
	if v, _ := cmd.Flags().GetString("group-id"); v != "" {
		body["groupId"] = v
	}
	if v, _ := cmd.Flags().GetString("groupname"); v != "" {
		body["groupname"] = v
	}
	if v, _ := cmd.Flags().GetString("project-id"); v != "" {
		body["projectId"] = v
	}
	if v, _ := cmd.Flags().GetString("project-role-id"); v != "" {
		body["projectRoleId"] = v
	}
	if rights, _ := cmd.Flags().GetInt("rights"); rights != 0 {
		body["rights"] = rights
	}
	if body["type"] == nil {
		return nil, apperr.NewValidationError("--with or --type is required", nil)
	}
	return body, nil
}

func parseFilterShareShorthand(value string) (map[string]any, error) {
	switch {
	case value == "global":
		return map[string]any{"type": "global"}, nil
	case value == "authenticated":
		return map[string]any{"type": "authenticated"}, nil
	case strings.HasPrefix(value, "user:"):
		accountID := strings.TrimPrefix(value, "user:")
		if accountID == "" {
			return nil, apperr.NewValidationError("user share shorthand requires an account ID", nil)
		}
		return map[string]any{"type": "user", "accountId": accountID}, nil
	case strings.HasPrefix(value, "group:"):
		groupID := strings.TrimPrefix(value, "group:")
		if groupID == "" {
			return nil, apperr.NewValidationError("group share shorthand requires a group ID", nil)
		}
		return map[string]any{"type": "group", "groupId": groupID}, nil
	case strings.HasPrefix(value, "groupname:"):
		groupName := strings.TrimPrefix(value, "groupname:")
		if groupName == "" {
			return nil, apperr.NewValidationError("groupname share shorthand requires a group name", nil)
		}
		return map[string]any{"type": "group", "groupname": groupName}, nil
	case strings.HasPrefix(value, "project:"):
		projectID := strings.TrimPrefix(value, "project:")
		if projectID == "" {
			return nil, apperr.NewValidationError("project share shorthand requires a project ID", nil)
		}
		return map[string]any{"type": "project", "projectId": projectID}, nil
	case strings.HasPrefix(value, "project-role:"):
		parts := strings.Split(strings.TrimPrefix(value, "project-role:"), ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, apperr.NewValidationError("project-role share shorthand requires project and role IDs", nil)
		}
		return map[string]any{"type": "projectRole", "projectId": parts[0], "projectRoleId": parts[1]}, nil
	default:
		return nil, apperr.NewValidationError("unsupported --with value", nil)
	}
}

func validateFilterShareScope(scope string) error {
	switch scope {
	case "GLOBAL", "AUTHENTICATED", "PRIVATE":
		return nil
	default:
		return apperr.NewValidationError("--scope must be GLOBAL, AUTHENTICATED, or PRIVATE", nil)
	}
}
