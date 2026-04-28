package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// UserCommand returns the top-level "user" command for Jira user lookups.
func UserCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Work with Jira users",
		UsageText: `jira-agent user search --query "john"
jira-agent user get --account-id 5b10a2844c20165700ede21g
jira-agent user groups --account-id 5b10a2844c20165700ede21g`,
		DefaultCommand: "search",
		Commands: []*cli.Command{
			userGetCommand(apiClient, w, format),
			userSearchCommand(apiClient, w, format),
			userGroupsCommand(apiClient, w, format),
		},
	}
}

// GroupCommand returns the top-level "group" command for Jira groups.
func GroupCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "group",
		Usage: "Work with Jira groups",
		UsageText: `jira-agent group list
jira-agent group get --groupname "jira-users"
jira-agent group members --groupname "jira-users"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			groupListCommand(apiClient, w, format),
			groupGetCommand(apiClient, w, format),
			groupCreateCommand(apiClient, w, format, allowWrites),
			groupDeleteCommand(apiClient, w, format, allowWrites),
			groupMembersCommand(apiClient, w, format),
			groupAddMemberCommand(apiClient, w, format, allowWrites),
			groupRemoveMemberCommand(apiClient, w, format, allowWrites),
			groupMemberPickerCommand(apiClient, w, format),
		},
	}
}

// FilterCommand returns the top-level "filter" command for Jira filters.
func FilterCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "filter",
		Usage: "Work with Jira filters",
		UsageText: `jira-agent filter list
jira-agent filter get 10001
jira-agent filter favorites`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
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
		},
	}
}

func userGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a user by account ID",
		UsageText: `jira-agent user get --account-id 5b10a2844c20165700ede21g`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "account-id", Usage: "User account ID", Required: true},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{"accountId": cmd.String("account-id")}
			addOptionalParams(cmd, params, map[string]string{"expand": "expand"})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/user", params, result)
			})
		},
	}
}

func userSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search users",
		UsageText: `jira-agent user search --query "john"
jira-agent user search --query "john.doe@example.com"`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{Name: "query", Usage: "User search query"},
			&cli.StringFlag{Name: "account-id", Usage: "Exact account ID"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, map[string]string{
				"query":      "query",
				"account-id": "accountId",
				"expand":     "expand",
			})

			var users []any
			if err := apiClient.Get(ctx, "/users/search", params, &users); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = len(users)
			return output.WriteSuccess(w, users, meta, *format)
		},
	}
}

func userGroupsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "groups",
		Usage:     "List groups for a user",
		UsageText: `jira-agent user groups --account-id 5b10a2844c20165700ede21g`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "account-id", Usage: "User account ID", Required: true},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var groups []any
			if err := apiClient.Get(ctx, "/user/groups", map[string]string{"accountId": cmd.String("account-id")}, &groups); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = len(groups)
			meta.Total = len(groups)
			return output.WriteSuccess(w, groups, meta, *format)
		},
	}
}

func groupListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "Find groups",
		UsageText: `jira-agent group list`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "query", Usage: "Group name query"},
			&cli.IntFlag{Name: "max-results", Usage: "Page size", Value: 50},
			&cli.BoolFlag{Name: "case-insensitive", Usage: "Search case-insensitively"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildMaxResultsParams(cmd, map[string]string{"query": "query"})
			addBoolParam(cmd, params, "case-insensitive", "caseInsensitive")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/groups/picker", params, result)
			})
		},
	}
}

func groupGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get group details",
		UsageText: `jira-agent group get --groupname "jira-users"
jira-agent group get --group-id abc-123`,
		Flags: append(groupIdentityFlags(), &cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
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
}

func groupCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a group",
		UsageText: `jira-agent group create "new-team"`,
		ArgsUsage: "<group-name>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			name, err := requireArg(cmd, "group name")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/group", map[string]any{"name": name}, result)
			})
		}),
	}
}

func groupDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a group",
		UsageText: `jira-agent group delete --groupname "old-team"`,
		Flags: append(groupIdentityFlags(),
			&cli.StringFlag{Name: "swap-group", Usage: "Group name receiving transferred restrictions"},
			&cli.StringFlag{Name: "swap-group-id", Usage: "Group ID receiving transferred restrictions"},
		),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
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
}

func groupMembersCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "members",
		Usage:     "List members of a group",
		UsageText: `jira-agent group members --groupname "jira-users"`,
		Flags:     appendPaginationFlags(append(groupIdentityFlags(), &cli.BoolFlag{Name: "include-inactive", Usage: "Include inactive users"})),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			maps.Copy(params, buildPaginationParams(cmd, nil))
			addBoolParam(cmd, params, "include-inactive", "includeInactiveUsers")

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/group/member", params, result)
			})
		},
	}
}

func groupAddMemberCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "add-member",
		Usage:     "Add a user to a group",
		UsageText: `jira-agent group add-member --groupname "dev-team" --account-id 5b10a2844c20165700ede21g`,
		Flags:     append(groupIdentityFlags(), &cli.StringFlag{Name: "account-id", Usage: "User account ID", Required: true}),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, appendQueryParams("/group/user", params), map[string]any{"accountId": cmd.String("account-id")}, result)
			})
		}),
	}
}

func groupRemoveMemberCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "remove-member",
		Usage:     "Remove a user from a group",
		UsageText: `jira-agent group remove-member --groupname "dev-team" --account-id 5b10a2844c20165700ede21g`,
		Flags:     append(groupIdentityFlags(), &cli.StringFlag{Name: "account-id", Usage: "User account ID", Required: true}),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			params, err := groupIdentityParams(cmd)
			if err != nil {
				return err
			}
			params["accountId"] = cmd.String("account-id")
			if err := apiClient.Delete(ctx, appendQueryParams("/group/user", params), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"accountId": cmd.String("account-id"), "removed": true}, *format)
		}),
	}
}

func groupMemberPickerCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "member-picker",
		Usage:     "Search users and groups for pickers",
		UsageText: `jira-agent group member-picker --query "john"`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "query", Usage: "Picker query"},
			&cli.StringFlag{Name: "issue-key", Usage: "Issue key for visibility context"},
			&cli.StringFlag{Name: "project-id", Usage: "Project ID for visibility context"},
			&cli.BoolFlag{Name: "show-avatar", Usage: "Include avatar URLs"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"query": "query", "issue-key": "issueKey", "project-id": "projectId"})
			addBoolParam(cmd, params, "show-avatar", "showAvatar")

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/groupuserpicker", params, result)
			})
		},
	}
}

func filterListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"search"},
		Usage:   "Search filters",
		UsageText: `jira-agent filter list
jira-agent filter list --expand owner,jql`,
		Flags: appendPaginationFlags([]cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Filter name query"},
			&cli.StringFlag{Name: "account-id", Usage: "Owner account ID"},
			&cli.StringFlag{Name: "groupname", Usage: "Shared group name"},
			&cli.StringFlag{Name: "group-id", Usage: "Shared group ID"},
			&cli.StringFlag{Name: "project-id", Usage: "Shared project ID"},
			&cli.StringFlag{Name: "order-by", Usage: "Sort field"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
			&cli.BoolFlag{Name: "override-share-permissions", Usage: "Override share permission filtering"},
			&cli.BoolFlag{Name: "substring", Usage: "Use substring matching for --name"},
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
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

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/filter/search", params, result)
			})
		},
	}
}

func filterGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a filter",
		UsageText: `jira-agent filter get 10001`,
		ArgsUsage: "<filter-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
			&cli.BoolFlag{Name: "override-share-permissions", Usage: "Override share permission filtering"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
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
}

func filterCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a filter",
		UsageText: `jira-agent filter create --name "My open bugs" --jql "project = PROJ AND type = Bug AND status = Open"`,
		Flags: append(filterBodyFlags(),
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
			&cli.BoolFlag{Name: "override-share-permissions", Usage: "Override share permission checks"},
		),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
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
}

func filterUpdateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a filter",
		UsageText: `jira-agent filter update 10001 --name "Updated filter" --jql "project = PROJ AND status != Done"`,
		ArgsUsage: "<filter-id>",
		Flags: append(filterBodyFlags(),
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated expansions"},
			&cli.BoolFlag{Name: "override-share-permissions", Usage: "Override share permission checks"},
		),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
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
}

func filterDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a filter",
		UsageText: `jira-agent filter delete 10001`,
		ArgsUsage: "<filter-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
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

func filterFavoritesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "favorites",
		Usage:     "List favorite filters",
		UsageText: `jira-agent filter favorites`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var filters []any
			if err := apiClient.Get(ctx, "/filter/favou"+"rite", nil, &filters); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = len(filters)
			meta.Total = len(filters)
			return output.WriteSuccess(w, filters, meta, *format)
		},
	}
}

func filterPermissionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "permissions",
		Usage:     "List filter share permissions",
		UsageText: `jira-agent filter permissions 10001`,
		ArgsUsage: "<filter-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
			if err != nil {
				return err
			}
			filterIDPath := escapePathSegment(filterID)

			var permissions []any
			if err := apiClient.Get(ctx, "/filter/"+filterIDPath+"/permission", nil, &permissions); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Returned = len(permissions)
			meta.Total = len(permissions)
			return output.WriteSuccess(w, permissions, meta, *format)
		},
	}
}

func filterShareCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "share",
		Usage: "Add a filter share permission",
		UsageText: `jira-agent filter share 10001 --with user:5b10a2844c20165700ede21g
jira-agent filter share 10001 --with group:team-group-id
jira-agent filter share 10001 --type project --project-id 10000`,
		ArgsUsage: "<filter-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "with", Usage: "Permission shorthand: global, authenticated, user:<account-id>, group:<group-id>, groupname:<name>, project:<project-id>, project-role:<project-id>:<role-id>"},
			&cli.StringFlag{Name: "type", Usage: "Share permission type"},
			&cli.StringFlag{Name: "account-id", Usage: "User account ID"},
			&cli.StringFlag{Name: "group-id", Usage: "Group ID"},
			&cli.StringFlag{Name: "groupname", Usage: "Group name"},
			&cli.StringFlag{Name: "project-id", Usage: "Project ID"},
			&cli.StringFlag{Name: "project-role-id", Usage: "Project role ID"},
			&cli.IntFlag{Name: "rights", Usage: "Share rights value"},
			&cli.StringFlag{Name: "body-json", Usage: "Raw share permission JSON body"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
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
			meta := output.NewMetadata()
			meta.Returned = len(permissions)
			meta.Total = len(permissions)
			return output.WriteSuccess(w, permissions, meta, *format)
		}),
	}
}

func filterUnshareCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "unshare",
		Usage:     "Remove a filter share permission",
		UsageText: `jira-agent filter unshare 10001 --permission-id 10002`,
		ArgsUsage: "<filter-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "permission-id", Usage: "Share permission ID", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			filterID, err := requireArg(cmd, "filter ID")
			if err != nil {
				return err
			}
			permissionID := cmd.String("permission-id")
			filterIDPath := escapePathSegment(filterID)
			permissionIDPath := escapePathSegment(permissionID)
			if err := apiClient.Delete(ctx, "/filter/"+filterIDPath+"/permission/"+permissionIDPath, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"filterId": filterID, "permissionId": permissionID, "deleted": true}, *format)
		}),
	}
}

func filterDefaultShareScopeCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "default-share-scope",
		Usage: "Work with the default filter share scope",
		UsageText: `jira-agent filter default-share-scope get
jira-agent filter default-share-scope set --scope PRIVATE`,
		DefaultCommand: "get",
		Commands: []*cli.Command{
			filterDefaultShareScopeGetCommand(apiClient, w, format),
			filterDefaultShareScopeSetCommand(apiClient, w, format, allowWrites),
		},
	}
}

func filterDefaultShareScopeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get the default filter share scope",
		UsageText: `jira-agent filter default-share-scope get`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/filter/defaultShareScope", nil, result)
			})
		},
	}
}

func filterDefaultShareScopeSetCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Set the default filter share scope",
		UsageText: `jira-agent filter default-share-scope set --scope PRIVATE`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "scope", Usage: "Default scope: GLOBAL, AUTHENTICATED, or PRIVATE", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			scope := cmd.String("scope")
			if err := validateFilterShareScope(scope); err != nil {
				return err
			}
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, "/filter/defaultShareScope", map[string]any{"scope": scope}, result)
			})
		}),
	}
}

func groupIdentityFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "groupname", Usage: "Group name"},
		&cli.StringFlag{Name: "group-id", Usage: "Group ID"},
	}
}

func groupIdentityParams(cmd *cli.Command) (map[string]string, error) {
	groupName := cmd.String("groupname")
	groupID := cmd.String("group-id")
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

func filterBodyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "name", Usage: "Filter name"},
		&cli.StringFlag{Name: "description", Usage: "Filter description"},
		&cli.StringFlag{Name: "jql", Usage: "Filter JQL"},
		&cli.StringFlag{Name: "body-json", Usage: "Raw filter JSON body"},
	}
}

func buildFilterBody(cmd *cli.Command, requireNameAndJQL bool) (map[string]any, error) {
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
	if v := cmd.String("jql"); v != "" {
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

func buildFilterShareBody(cmd *cli.Command) (map[string]any, error) {
	body := map[string]any{}
	if raw := cmd.String("body-json"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return nil, apperr.NewValidationError(fmt.Sprintf("invalid --body-json: %v", err), err)
		}
	}
	if shorthand := cmd.String("with"); shorthand != "" {
		parsed, err := parseFilterShareShorthand(shorthand)
		if err != nil {
			return nil, err
		}
		maps.Copy(body, parsed)
	}
	if v := cmd.String("type"); v != "" {
		body["type"] = v
	}
	if v := cmd.String("account-id"); v != "" {
		body["accountId"] = v
	}
	if v := cmd.String("group-id"); v != "" {
		body["groupId"] = v
	}
	if v := cmd.String("groupname"); v != "" {
		body["groupname"] = v
	}
	if v := cmd.String("project-id"); v != "" {
		body["projectId"] = v
	}
	if v := cmd.String("project-role-id"); v != "" {
		body["projectRoleId"] = v
	}
	if rights := cmd.Int("rights"); rights != 0 {
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
