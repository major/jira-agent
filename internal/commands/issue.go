// Package commands implements the CLI command tree for jira-agent.
//
// Each file corresponds to a top-level command group: issue.go for issue
// operations, field.go for field management. Command constructors accept a
// shared client.Ref, writer, and output format pointer configured by the
// root command's Before hook.
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

// IssueCommand returns the "issue" parent command with all subcommands.
func IssueCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "issue",
		Usage: "Issue operations (get, search, create, edit, delete, transition, assign, comment, changelog, rank, count, notify, meta, bulk, property)",
		UsageText: `jira-agent issue get PROJ-123
jira-agent issue search --jql "project = PROJ AND status = Open"
jira-agent issue create --project PROJ --type Story --summary "New feature"
jira-agent issue transition PROJ-123 --to "In Progress"`,
		Commands: []*cli.Command{
			issueGetCommand(apiClient, w, format),
			issuePickerCommand(apiClient, w, format),
			issueSearchCommand(apiClient, w, format),
			issueCreateCommand(apiClient, w, format, allowWrites),
			issueBulkCommand(apiClient, w, format, allowWrites),
			legacyIssueBulkCommand(issueBulkCreateCommand(apiClient, w, format, allowWrites)),
			legacyIssueBulkCommand(issueBulkFetchCommand(apiClient, w, format)),
			legacyIssueBulkCommand(issueBulkDeleteCommand(apiClient, w, format, allowWrites)),
			legacyIssueBulkCommand(issueBulkEditFieldsCommand(apiClient, w, format)),
			legacyIssueBulkCommand(issueBulkEditCommand(apiClient, w, format, allowWrites)),
			legacyIssueBulkCommand(issueBulkMoveCommand(apiClient, w, format, allowWrites)),
			legacyIssueBulkCommand(issueBulkTransitionsCommand(apiClient, w, format)),
			legacyIssueBulkCommand(issueBulkTransitionCommand(apiClient, w, format, allowWrites)),
			legacyIssueBulkCommand(issueBulkStatusCommand(apiClient, w, format)),
			issueEditCommand(apiClient, w, format, allowWrites),
			issueTransitionCommand(apiClient, w, format, allowWrites),
			issueAssignCommand(apiClient, w, format, allowWrites),
			issueCommentCommand(apiClient, w, format, allowWrites),
			issueWatcherCommand(apiClient, w, format, allowWrites),
			issueVoteCommand(apiClient, w, format, allowWrites),
			issueAttachmentCommand(apiClient, w, format, allowWrites),
			issueLinkCommand(apiClient, w, format, allowWrites),
			issueRemoteLinkCommand(apiClient, w, format, allowWrites),
			issueWorklogCommand(apiClient, w, format, allowWrites),
			issueDeleteCommand(apiClient, w, format, allowWrites),
			issueChangelogCommand(apiClient, w, format),
			issueRankCommand(apiClient, w, format, allowWrites),
			issueCountCommand(apiClient, w, format),
			issueNotifyCommand(apiClient, w, format, allowWrites),
			issueMetaCommand(apiClient, w, format),
			issuePropertyCommand(apiClient, w, format, allowWrites),
		},
	}
}

func issueBulkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "bulk",
		Usage: "Bulk issue operations",
		UsageText: `jira-agent issue bulk fetch --issues PROJ-1,PROJ-2 --fields key,summary,status
jira-agent issue bulk create --issues-json '[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task 1"}}]'
jira-agent issue bulk transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]'`,
		Commands: []*cli.Command{
			canonicalIssueBulkCommand(issueBulkCreateCommand(apiClient, w, format, allowWrites), "create"),
			canonicalIssueBulkCommand(issueBulkFetchCommand(apiClient, w, format), "fetch"),
			canonicalIssueBulkCommand(issueBulkDeleteCommand(apiClient, w, format, allowWrites), "delete"),
			canonicalIssueBulkCommand(issueBulkEditFieldsCommand(apiClient, w, format), "edit-fields"),
			canonicalIssueBulkCommand(issueBulkEditCommand(apiClient, w, format, allowWrites), "edit"),
			canonicalIssueBulkCommand(issueBulkMoveCommand(apiClient, w, format, allowWrites), "move"),
			canonicalIssueBulkCommand(issueBulkTransitionsCommand(apiClient, w, format), "transitions"),
			canonicalIssueBulkCommand(issueBulkTransitionCommand(apiClient, w, format, allowWrites), "transition"),
			canonicalIssueBulkCommand(issueBulkStatusCommand(apiClient, w, format), "status"),
		},
	}
}

func canonicalIssueBulkCommand(cmd *cli.Command, name string) *cli.Command {
	legacyInvocation := "jira-agent issue " + cmd.Name
	cmd.Name = name
	cmd.UsageText = strings.ReplaceAll(cmd.UsageText, legacyInvocation, "jira-agent issue bulk "+name)
	return cmd
}

func legacyIssueBulkCommand(cmd *cli.Command) *cli.Command {
	cmd.Hidden = true
	return cmd
}

func issueBulkCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "bulk-create",
		Usage:     "Create multiple issues from Jira issueUpdates JSON",
		UsageText: `jira-agent issue bulk-create --issues-json '[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task 1"}}]'`,
		Metadata:  commandMetadata(writeCommandMetadata(), requiredFlagMetadata("issues-json")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "issues-json",
				Usage: "JSON array of issueUpdates or object with issueUpdates (required, max 50 issues)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body, err := parseBulkCreateBody(cmd.String("issues-json"))
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/bulk", body, result)
			})
		}),
	}
}

func issueBulkFetchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "bulk-fetch",
		Usage: "Fetch multiple issues by key or ID",
		UsageText: `jira-agent issue bulk-fetch --issues PROJ-1,PROJ-2,PROJ-3
jira-agent issue bulk-fetch --issues PROJ-1,PROJ-2 --fields key,summary,status`,
		Metadata: requiredFlagMetadata("issues"),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "issues",
				Usage: "Comma-separated issue keys or IDs (required, max 100 issues)",
			},
			&cli.StringFlag{
				Name:  "fields",
				Usage: "Comma-separated field list",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (names, schema, changelog, operations)",
			},
			&cli.StringFlag{
				Name:  "properties",
				Usage: "Comma-separated issue properties to include (max 5)",
			},
			&cli.BoolFlag{
				Name:  "fields-by-keys",
				Usage: "Treat field identifiers as field keys instead of field IDs",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			body, err := buildBulkFetchBody(cmd)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/bulkfetch", body, result)
			})
		},
	}
}

func issueGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get issue by key or ID",
		UsageText: `jira-agent issue get PROJ-123
jira-agent issue get PROJ-123 --fields key,summary,status,assignee
jira-agent issue get PROJ-123 --expand changelog`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fields",
				Usage: "Comma-separated field list (default: all navigable)",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (names, schema, changelog, operations)",
			},
			&cli.StringFlag{
				Name:  "properties",
				Usage: "Comma-separated issue properties to include",
			},
			&cli.BoolFlag{
				Name:  "fields-by-keys",
				Usage: "Treat field identifiers as field keys instead of field IDs",
			},
			&cli.BoolFlag{
				Name:  "update-history",
				Usage: "Add the issue to the user's recently viewed history",
			},
			&cli.BoolFlag{
				Name:  "fail-fast",
				Usage: "Fail immediately if a requested field cannot be resolved",
				Value: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"fields":     "fields",
				"expand":     "expand",
				"properties": "properties",
			})
			addBoolParam(cmd, params, "fields-by-keys", "fieldsByKeys")
			addBoolParam(cmd, params, "update-history", "updateHistory")
			if cmd.IsSet("fail-fast") {
				params["failFast"] = fmt.Sprintf("%t", cmd.Bool("fail-fast"))
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key, params, result)
			})
		},
	}
}

func issuePickerCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "picker",
		Usage: "Find issues for picker UI",
		UsageText: `jira-agent issue picker --query PROJ-123
jira-agent issue picker --query login --current-jql "project = PROJ" --show-subtasks=false`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "query", Usage: "Issue key, summary text, or search text"},
			&cli.StringFlag{Name: "current-jql", Usage: "JQL context for current search results"},
			&cli.StringFlag{Name: "current-issue-key", Usage: "Current issue key for context"},
			&cli.StringFlag{Name: "current-project-id", Usage: "Current project ID for context"},
			&cli.BoolFlag{Name: "show-subtasks", Usage: "Include subtasks in picker results", Value: true},
			&cli.BoolFlag{Name: "show-subtask-parent", Usage: "Show parent summaries for subtasks", Value: true},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"query":              "query",
				"current-jql":        "currentJQL",
				"current-issue-key":  "currentIssueKey",
				"current-project-id": "currentProjectId",
			})
			if cmd.IsSet("show-subtasks") {
				params["showSubTasks"] = fmt.Sprintf("%t", cmd.Bool("show-subtasks"))
			}
			if cmd.IsSet("show-subtask-parent") {
				params["showSubTaskParent"] = fmt.Sprintf("%t", cmd.Bool("show-subtask-parent"))
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/picker", params, result)
			})
		},
	}
}

func issueSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search issues via JQL",
		UsageText: `jira-agent issue search --jql "project = PROJ AND status = Open"
jira-agent issue search --jql "assignee = currentUser()" --fields key,summary,status
jira-agent issue search --jql "project = PROJ" --max-results 10 --order-by created --order desc`,
		Metadata: requiredFlagMetadata("jql"),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "jql",
				Usage: "JQL query string (required)",
			},
			&cli.StringFlag{
				Name:  "fields",
				Usage: "Comma-separated field list",
				Value: "summary,status,assignee,priority",
			},
			&cli.IntFlag{
				Name:  "max-results",
				Usage: "Page size",
				Value: 50,
			},
			&cli.StringFlag{
				Name:  "next-page-token",
				Usage: "Token for fetching next page of results",
			},
			&cli.StringFlag{
				Name:  "expand",
				Usage: "Comma-separated expansions (names, schema, changelog, operations)",
			},
			&cli.StringFlag{
				Name:  "properties",
				Usage: "Comma-separated issue properties to include",
			},
			&cli.BoolFlag{
				Name:  "fields-by-keys",
				Usage: "Treat field identifiers as field keys instead of field IDs",
			},
			&cli.BoolFlag{
				Name:  "fail-fast",
				Usage: "Fail immediately if a requested field cannot be resolved",
				Value: true,
			},
			&cli.StringFlag{
				Name:  "reconcile-issues",
				Usage: "Comma-separated issue IDs to reconcile for approximate-count drift",
			},
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Sort field (appended to JQL as ORDER BY clause)",
			},
			&cli.StringFlag{
				Name:  "order",
				Usage: "Sort direction: asc or desc (used with --order-by)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			jql, err := requireFlag(cmd, "jql")
			if err != nil {
				return err
			}

			// Append ORDER BY clause from flags if provided.
			if orderBy := cmd.String("order-by"); orderBy != "" {
				direction := strings.ToUpper(cmd.String("order"))
				if direction == "" {
					direction = "ASC"
				}
				jql += " ORDER BY " + orderBy + " " + direction
			}

			body := map[string]any{
				"jql":        jql,
				"maxResults": cmd.Int("max-results"),
			}

			if t := cmd.String("next-page-token"); t != "" {
				body["nextPageToken"] = t
			}
			if f := cmd.String("fields"); f != "" {
				body["fields"] = splitTrimmed(f)
			}
			if e := cmd.String("expand"); e != "" {
				body["expand"] = e
			}
			if properties := splitTrimmed(cmd.String("properties")); len(properties) > 0 {
				body["properties"] = properties
			}
			if cmd.Bool("fields-by-keys") {
				body["fieldsByKeys"] = true
			}
			if cmd.IsSet("fail-fast") {
				body["failFast"] = cmd.Bool("fail-fast")
			}
			if reconcile := splitTrimmed(cmd.String("reconcile-issues")); len(reconcile) > 0 {
				body["reconcileIssues"] = reconcile
			}

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/search/jql", body, result)
			})
		},
	}
}

func issueCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new issue",
		UsageText: `jira-agent issue create --project PROJ --type Story --summary "New feature"
jira-agent issue create --project PROJ --type Bug --summary "Fix login" --priority High --labels bug,urgent
jira-agent issue create --project PROJ --type Task --summary "Subtask" --parent PROJ-100`,
		Metadata: commandMetadata(writeCommandMetadata(), requiredFlagMetadata("type", "summary")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "project",
				Usage:   "Project key",
				Sources: cli.EnvVars("JIRA_PROJECT"),
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Issue type name (e.g. Story, Bug, Task)",
			},
			&cli.StringFlag{
				Name:  "summary",
				Usage: "Issue summary",
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "Description (plain text or ADF JSON)",
			},
			&cli.StringFlag{
				Name:  "assignee",
				Usage: "Assignee account ID",
			},
			&cli.StringFlag{
				Name:  "priority",
				Usage: "Priority name (e.g. High, Medium, Low)",
			},
			&cli.StringFlag{
				Name:  "labels",
				Usage: "Comma-separated labels",
			},
			&cli.StringFlag{
				Name:  "components",
				Usage: "Comma-separated component names",
			},
			&cli.StringFlag{
				Name:  "parent",
				Usage: "Parent issue key (for subtasks/child issues)",
			},
			&cli.StringMapFlag{
				Name:  "field",
				Usage: "Custom field value (key=value, repeatable)",
			},
			&cli.StringFlag{
				Name:  "fields-json",
				Usage: "JSON object of fields (alternative to individual flags)",
			},
			&cli.StringFlag{
				Name:  "payload-json",
				Usage: "Full JSON issue create payload, merged after field flags",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			project, err := requireProject(cmd)
			if err != nil {
				return err
			}

			issueType, err := requireFlag(cmd, "type")
			if err != nil {
				return err
			}

			summary, err := requireFlag(cmd, "summary")
			if err != nil {
				return err
			}

			fields := map[string]any{
				"project":   map[string]any{"key": project},
				"issuetype": map[string]any{"name": issueType},
				"summary":   summary,
			}

			applyCommonFields(fields, cmd)

			if err := applyMerges(fields, cmd); err != nil {
				return err
			}

			body := map[string]any{"fields": fields}
			if err := mergePayloadJSON(body, cmd.String("payload-json")); err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue", body, result)
			})
		}),
	}
}

func issueEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "edit",
		Usage: "Update issue fields",
		UsageText: `jira-agent issue edit PROJ-123 --summary "Updated summary"
jira-agent issue edit PROJ-123 --priority High --labels bug,critical
jira-agent issue edit PROJ-123 --fields-json '{"customfield_10001":"value"}'`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "summary", Usage: "Issue summary"},
			&cli.StringFlag{Name: "description", Usage: "Description (plain text or ADF JSON)"},
			&cli.StringFlag{Name: "assignee", Usage: "Assignee account ID"},
			&cli.StringFlag{Name: "priority", Usage: "Priority name"},
			&cli.StringFlag{Name: "labels", Usage: "Comma-separated labels"},
			&cli.StringFlag{Name: "components", Usage: "Comma-separated component names"},
			&cli.StringFlag{Name: "parent", Usage: "Parent issue key"},
			&cli.StringMapFlag{Name: "field", Usage: "Custom field value (key=value, repeatable)"},
			&cli.StringFlag{Name: "fields-json", Usage: "JSON object of fields"},
			&cli.StringFlag{Name: "payload-json", Usage: "Full JSON issue edit payload, merged after field flags"},
			&cli.BoolFlag{Name: "notify", Value: true, Usage: "Send notification to watchers"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			fields := map[string]any{}

			if v := cmd.String("summary"); v != "" {
				fields["summary"] = v
			}

			applyCommonFields(fields, cmd)

			if err := applyMerges(fields, cmd); err != nil {
				return err
			}

			body := map[string]any{"fields": fields}
			if err := mergePayloadJSON(body, cmd.String("payload-json")); err != nil {
				return err
			}
			if len(fields) == 0 && cmd.String("payload-json") == "" {
				return apperr.NewValidationError("at least one field or --payload-json update is required", nil)
			}

			path := "/issue/" + key
			if !cmd.Bool("notify") {
				path += "?notifyUsers=false"
			}
			if err := apiClient.Put(ctx, path, body, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":     key,
				"updated": true,
			}, *format)
		}),
	}
}

func issueTransitionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "transition",
		Usage: "Transition issue to a new status",
		UsageText: `jira-agent issue transition PROJ-123 --list
jira-agent issue transition PROJ-123 --to "In Progress"
jira-agent issue transition PROJ-123 --to Done --comment "Completed"`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to", Usage: "Target status or transition name"},
			&cli.StringFlag{Name: "transition-id", Usage: "Transition ID to apply directly"},
			&cli.StringMapFlag{Name: "field", Usage: "Transition screen field (key=value, repeatable)"},
			&cli.StringFlag{Name: "comment", Usage: "Comment to add during transition"},
			&cli.StringFlag{Name: "payload-json", Usage: "Full JSON transition payload, merged after flags"},
			&cli.StringFlag{Name: "expand", Usage: "Comma-separated transition expansions (transitions.fields)"},
			&cli.StringFlag{Name: "list-transition-id", Usage: "Only return this transition ID when listing transitions"},
			&cli.BoolFlag{Name: "include-unavailable-transitions", Usage: "Include transitions unavailable to the current user"},
			&cli.BoolFlag{Name: "skip-remote-only-condition", Usage: "Skip remote-only workflow conditions"},
			&cli.BoolFlag{Name: "sort-by-ops-bar-and-status", Usage: "Sort listed transitions by ops bar sequence and status"},
			&cli.BoolFlag{Name: "list", Usage: "List available transitions instead of performing one"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"expand":             "expand",
				"list-transition-id": "transitionId",
			})
			addBoolParam(cmd, params, "include-unavailable-transitions", "includeUnavailableTransitions")
			addBoolParam(cmd, params, "skip-remote-only-condition", "skipRemoteOnlyCondition")
			addBoolParam(cmd, params, "sort-by-ops-bar-and-status", "sortByOpsBarAndStatus")

			var transitions any
			if cmd.Bool("list") || cmd.String("transition-id") == "" {
				if err := apiClient.Get(ctx, "/issue/"+key+"/transitions", params, &transitions); err != nil {
					return err
				}
			}

			if cmd.Bool("list") {
				return output.WriteResult(w, transitions, *format)
			}

			if err := requireWriteAccess(allowWrites); err != nil {
				return err
			}

			targetStatus := cmd.String("to")
			transitionID := cmd.String("transition-id")
			if transitionID == "" {
				if targetStatus == "" {
					return apperr.NewValidationError(
						"--to or --transition-id is required (or use --list to see available transitions)",
						nil,
					)
				}
				var findErr error
				transitionID, findErr = findTransitionID(transitions, targetStatus)
				if findErr != nil {
					return findErr
				}
			}

			reqBody := map[string]any{
				"transition": map[string]any{"id": transitionID},
			}

			if fieldMap := cmd.StringMap("field"); len(fieldMap) > 0 {
				transFields := map[string]any{}
				applyFieldOverrides(transFields, fieldMap)
				reqBody["fields"] = transFields
			}

			if comment := cmd.String("comment"); comment != "" {
				reqBody["update"] = map[string]any{
					"comment": []any{
						map[string]any{
							"add": map[string]any{
								"body": toADF(comment),
							},
						},
					},
				}
			}
			if err := mergePayloadJSON(reqBody, cmd.String("payload-json")); err != nil {
				return err
			}

			if err := apiClient.Post(ctx, "/issue/"+key+"/transitions", reqBody, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":          key,
				"transitioned": true,
				"to":           targetStatus,
			}, *format)
		},
	}
}

// DELETE /rest/api/3/issue/{issueIdOrKey}
func issueDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete an issue",
		UsageText: `jira-agent issue delete PROJ-123
jira-agent issue delete PROJ-123 --delete-subtasks`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "delete-subtasks", Usage: "Delete subtasks of the issue"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			path := "/issue/" + key
			if cmd.Bool("delete-subtasks") {
				path = appendQueryParams(path, map[string]string{"deleteSubtasks": "true"})
			}

			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":     key,
				"deleted": true,
			}, *format)
		}),
	}
}

func issueChangelogCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "changelog",
		Usage: "List changelog entries for an issue",
		UsageText: `jira-agent issue changelog PROJ-123
jira-agent issue changelog PROJ-123 --max-results 10
jira-agent issue changelog list-by-ids PROJ-123 --ids 10001,10002
jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2`,
		ArgsUsage: "<issue-key>",
		Flags:     appendPaginationFlags(nil),
		Commands: []*cli.Command{
			issueChangelogListByIDsCommand(apiClient, w, format),
			issueChangelogBulkFetchCommand(apiClient, w, format),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/changelog", params, result)
			})
		},
	}
}

func issueChangelogListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list-by-ids",
		Usage:     "Get issue changelogs by IDs",
		UsageText: `jira-agent issue changelog list-by-ids PROJ-123 --ids 10001,10002`,
		Metadata:  requiredFlagMetadata("ids"),
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "ids", Usage: "Comma-separated changelog IDs (required)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}
			idsFlag, err := requireFlag(cmd, "ids")
			if err != nil {
				return err
			}

			ids, err := parseInt64List(idsFlag)
			if err != nil {
				return err
			}
			body := map[string]any{"changelogIds": ids}
			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/"+key+"/changelog/list", body, result)
			})
		},
	}
}

func issueChangelogBulkFetchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "bulk-fetch",
		Usage: "Bulk fetch changelogs for issues",
		UsageText: `jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2
jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2 --field-ids status,assignee --max-results 100`,
		Metadata: requiredFlagMetadata("issues"),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "issues", Usage: "Comma-separated issue keys or IDs (required, max 1000)"},
			&cli.StringFlag{Name: "field-ids", Usage: "Comma-separated field IDs to filter changelogs (max 10)"},
			&cli.IntFlag{Name: "max-results", Usage: "Maximum changelog items to return", Value: 1000},
			&cli.StringFlag{Name: "next-page-token", Usage: "Cursor token from previous response"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			issuesFlag, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}
			issues := splitTrimmed(issuesFlag)
			if len(issues) > 1000 {
				return apperr.NewValidationError("--issues supports at most 1000 issues", nil)
			}

			body := map[string]any{"issueIdsOrKeys": issues}
			if fields := splitTrimmed(cmd.String("field-ids")); len(fields) > 0 {
				if len(fields) > 10 {
					return apperr.NewValidationError("--field-ids supports at most 10 fields", nil)
				}
				body["fieldIds"] = fields
			}
			if cmd.IsSet("max-results") {
				body["maxResults"] = cmd.Int("max-results")
			}
			if token := cmd.String("next-page-token"); token != "" {
				body["nextPageToken"] = token
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/changelog/bulkfetch", body, result)
			})
		},
	}
}

func issueRankCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "rank",
		Usage: "Rank issues before or after a target issue",
		UsageText: `jira-agent issue rank --issues PROJ-1,PROJ-2 --before PROJ-10
jira-agent issue rank --issues PROJ-5 --after PROJ-3`,
		Metadata: writeCommandMetadata(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "issues",
				Usage:    "Comma-separated issue keys to rank",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "before",
				Usage: "Issue key to rank before (mutually exclusive with --after)",
			},
			&cli.StringFlag{
				Name:  "after",
				Usage: "Issue key to rank after (mutually exclusive with --before)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			before := cmd.String("before")
			after := cmd.String("after")

			if before == "" && after == "" {
				return apperr.NewValidationError("either --before or --after is required", nil)
			}

			if before != "" && after != "" {
				return apperr.NewValidationError("--before and --after are mutually exclusive", nil)
			}

			issues := splitTrimmed(cmd.String("issues"))
			body := map[string]any{
				"issues": issues,
			}

			if before != "" {
				body["rankBeforeIssue"] = before
			}

			if after != "" {
				body["rankAfterIssue"] = after
			}

			// Rank issues returns 204 No Content on success.
			if err := apiClient.AgilePut(ctx, "/issue/rank", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"ranked": issues,
			}, *format)
		}),
	}
}

func issueCountCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "count",
		Usage: "Get approximate count of issues matching a JQL query",
		UsageText: `jira-agent issue count --jql "project = PROJ AND status = Open"
jira-agent issue count --jql "assignee = currentUser() AND resolution = Unresolved"`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "jql",
				Usage:    "JQL query to count results for",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			body := map[string]string{
				"jql": cmd.String("jql"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/search/approximate-count", body, result)
			})
		},
	}
}

func issueMetaCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "meta",
		Usage: "Get create or edit metadata for issues",
		UsageText: `jira-agent issue meta --project PROJ
jira-agent issue meta --project PROJ --type Story
jira-agent issue meta --operation edit --issue PROJ-123`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "project",
				Usage:   "Project key",
				Sources: cli.EnvVars("JIRA_PROJECT"),
			},
			&cli.StringFlag{Name: "type", Usage: "Issue type name (filters to a single type)"},
			&cli.StringFlag{Name: "operation", Value: "create", Usage: "Operation: create or edit"},
			&cli.StringFlag{Name: "issue", Usage: "Issue key (required for edit metadata)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			operation := cmd.String("operation")

			switch operation {
			case "edit":
				issueKey, err := requireFlag(cmd, "issue")
				if err != nil {
					return apperr.NewValidationError("--issue is required for edit metadata", nil)
				}

				return writeAPIResult(w, *format, func(result any) error {
					return apiClient.Get(ctx, "/issue/"+issueKey+"/editmeta", nil, result)
				})

			case "create":
				project := resolveProject(cmd)
				if project == "" {
					return apperr.NewValidationError(
						"--project is required for create metadata",
						nil,
						apperr.WithDetails("use --project flag or set JIRA_PROJECT env var"),
					)
				}

				typeName := cmd.String("type")
				if typeName == "" {
					// List available issue types for the project.
					path := "/issue/createmeta/" + project + "/issuetypes"
					return writePaginatedAPIResult(w, *format, func(result any) error {
						return apiClient.Get(ctx, path, nil, result)
					})
				}

				// Resolve issue type name to ID, then fetch its field metadata.
				var typeList any
				if err := apiClient.Get(ctx, "/issue/createmeta/"+project+"/issuetypes", nil, &typeList); err != nil {
					return err
				}
				typeID, err := findIssueTypeID(typeList, typeName)
				if err != nil {
					return err
				}

				path := "/issue/createmeta/" + project + "/issuetypes/" + typeID
				return writePaginatedAPIResult(w, *format, func(result any) error {
					return apiClient.Get(ctx, path, nil, result)
				})

			default:
				return apperr.NewValidationError(
					fmt.Sprintf("unknown operation %q (valid: create, edit)", operation),
					nil,
				)
			}
		},
	}
}

// applyCommonFields sets optional issue fields that are shared between create
// and edit: description, assignee, priority, labels, components, and parent.
// Each field is only set when the corresponding flag has a non-empty value.
func applyCommonFields(fields map[string]any, cmd *cli.Command) {
	if v := cmd.String("description"); v != "" {
		fields["description"] = toADF(v)
	}
	if v := cmd.String("assignee"); v != "" {
		fields["assignee"] = map[string]any{"accountId": v}
	}
	if v := cmd.String("priority"); v != "" {
		fields["priority"] = map[string]any{"name": v}
	}
	if v := cmd.String("labels"); v != "" {
		fields["labels"] = splitTrimmed(v)
	}
	if v := cmd.String("components"); v != "" {
		names := splitTrimmed(v)
		comps := make([]map[string]any, len(names))
		for i, n := range names {
			comps[i] = map[string]any{"name": n}
		}
		fields["components"] = comps
	}
	if v := cmd.String("parent"); v != "" {
		fields["parent"] = map[string]any{"key": v}
	}
}

// applyMerges applies --field overrides and --fields-json merges to the fields
// map. This consolidates the two-step merge pattern used by create and edit.
func applyMerges(fields map[string]any, cmd *cli.Command) error {
	applyFieldOverrides(fields, cmd.StringMap("field"))
	return applyFieldsJSON(fields, cmd.String("fields-json"))
}

// resolveProject returns the project key from the command's own --project flag,
// falling back to the root command's --project/-p flag (which also reads
// JIRA_PROJECT via its Sources).
func resolveProject(cmd *cli.Command) string {
	if p := cmd.String("project"); p != "" {
		return p
	}
	return cmd.Root().String("project")
}

func requireProject(cmd *cli.Command) (string, error) {
	project := resolveProject(cmd)
	if project != "" {
		return project, nil
	}
	return "", apperr.NewValidationError(
		"--project is required",
		nil,
		apperr.WithDetails("use --project flag or set JIRA_PROJECT env var"),
	)
}

// toADF wraps plain text in an Atlassian Document Format paragraph.
// If the input is already valid ADF JSON (has a "type" key), it is returned as-is.
func toADF(text string) any {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if _, hasType := parsed["type"]; hasType {
			return parsed
		}
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}

// applyFieldOverrides merges --field key=value pairs into the fields map.
// Values that parse as valid JSON are stored as structured values; otherwise
// the raw string is used.
func applyFieldOverrides(fields map[string]any, overrides map[string]string) {
	for k, v := range overrides {
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			fields[k] = parsed
		} else {
			fields[k] = v
		}
	}
}

// applyFieldsJSON merges a --fields-json string into the fields map.
func applyFieldsJSON(fields map[string]any, jsonStr string) error {
	if jsonStr == "" {
		return nil
	}
	var jsonFields map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &jsonFields); err != nil {
		return apperr.NewValidationError(
			fmt.Sprintf("invalid --fields-json: %v", err),
			err,
		)
	}
	maps.Copy(fields, jsonFields)
	return nil
}

func parseBulkCreateBody(jsonStr string) (map[string]any, error) {
	if jsonStr == "" {
		return nil, apperr.NewValidationError("--issues-json is required", nil)
	}

	var parsed any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("invalid --issues-json: %v", err),
			err,
		)
	}

	switch v := parsed.(type) {
	case []any:
		if len(v) == 0 {
			return nil, apperr.NewValidationError("--issues-json must include at least one issue", nil)
		}
		if len(v) > 50 {
			return nil, apperr.NewValidationError("--issues-json supports at most 50 issues", nil)
		}
		return map[string]any{"issueUpdates": v}, nil
	case map[string]any:
		updates, ok := v["issueUpdates"].([]any)
		if !ok {
			return nil, apperr.NewValidationError("--issues-json object must include issueUpdates array", nil)
		}
		if len(updates) == 0 {
			return nil, apperr.NewValidationError("--issues-json must include at least one issue", nil)
		}
		if len(updates) > 50 {
			return nil, apperr.NewValidationError("--issues-json supports at most 50 issues", nil)
		}
		return v, nil
	default:
		return nil, apperr.NewValidationError("--issues-json must be a JSON array or object", nil)
	}
}

func buildBulkFetchBody(cmd *cli.Command) (map[string]any, error) {
	issueFlag, err := requireFlag(cmd, "issues")
	if err != nil {
		return nil, err
	}
	issues := splitTrimmed(issueFlag)
	if len(issues) == 0 {
		return nil, apperr.NewValidationError("--issues is required", nil)
	}
	if len(issues) > 100 {
		return nil, apperr.NewValidationError("--issues supports at most 100 issues", nil)
	}

	body := map[string]any{
		"issueIdsOrKeys": issues,
	}
	if fields := splitTrimmed(cmd.String("fields")); len(fields) > 0 {
		body["fields"] = fields
	}
	if expand := splitTrimmed(cmd.String("expand")); len(expand) > 0 {
		body["expand"] = expand
	}
	if properties := splitTrimmed(cmd.String("properties")); len(properties) > 0 {
		if len(properties) > 5 {
			return nil, apperr.NewValidationError("--properties supports at most 5 properties", nil)
		}
		body["properties"] = properties
	}
	if cmd.Bool("fields-by-keys") {
		body["fieldsByKeys"] = true
	}

	return body, nil
}

func issueAssignCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "assign",
		Usage: "Assign issue to a user",
		UsageText: `jira-agent issue assign PROJ-123 5b10a2844c20165700ede21g
jira-agent issue assign PROJ-123 --unassign
jira-agent issue assign PROJ-123 --default`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key> [account-id]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "unassign", Usage: "Remove current assignee"},
			&cli.BoolFlag{Name: "default", Usage: "Use project default assignee"},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			var body map[string]any
			switch {
			case cmd.Bool("unassign"):
				body = map[string]any{"accountId": nil}
			case cmd.Bool("default"):
				body = map[string]any{"accountId": "-1"}
			default:
				args, err := requireArgs(cmd, "issue key", "account ID")
				if err != nil {
					return apperr.NewValidationError(
						"account ID is required (or use --unassign / --default)",
						err,
					)
				}
				body = map[string]any{"accountId": args[1]}
			}

			if err := apiClient.Put(ctx, "/issue/"+key+"/assignee", body, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":      key,
				"assigned": true,
			}, *format)
		}),
	}
}

// POST /rest/api/3/issue/{issueIdOrKey}/notify
func issueNotifyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "notify",
		Usage: "Send an email notification for an issue",
		UsageText: `jira-agent issue notify PROJ-123 --subject "Action needed" --text-body "Please review"
jira-agent issue notify PROJ-123 --subject "Update" --to-watchers --to-assignee`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "subject",
				Usage:    "notification subject line",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "text-body",
				Usage: "plain text body",
			},
			&cli.StringFlag{
				Name:  "html-body",
				Usage: "HTML body",
			},
			&cli.BoolFlag{
				Name:  "to-assignee",
				Usage: "send to assignee",
			},
			&cli.BoolFlag{
				Name:  "to-reporter",
				Usage: "send to reporter",
			},
			&cli.BoolFlag{
				Name:  "to-voters",
				Usage: "send to voters",
			},
			&cli.BoolFlag{
				Name:  "to-watchers",
				Usage: "send to watchers",
			},
			&cli.StringSliceFlag{
				Name:  "to-users",
				Usage: "account IDs of additional recipients (repeatable)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			issueKey, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			to := map[string]any{}
			if cmd.IsSet("to-assignee") {
				to["assignee"] = cmd.Bool("to-assignee")
			}
			if cmd.IsSet("to-reporter") {
				to["reporter"] = cmd.Bool("to-reporter")
			}
			if cmd.IsSet("to-voters") {
				to["voters"] = cmd.Bool("to-voters")
			}
			if cmd.IsSet("to-watchers") {
				to["watchers"] = cmd.Bool("to-watchers")
			}
			if users := cmd.StringSlice("to-users"); len(users) > 0 {
				userObjs := make([]map[string]string, len(users))
				for i, u := range users {
					userObjs[i] = map[string]string{"accountId": u}
				}
				to["users"] = userObjs
			}

			body := map[string]any{
				"subject": cmd.String("subject"),
			}
			if v := cmd.String("text-body"); v != "" {
				body["textBody"] = v
			}
			if v := cmd.String("html-body"); v != "" {
				body["htmlBody"] = v
			}
			if len(to) > 0 {
				body["to"] = to
			}

			path := "/issue/" + issueKey + "/notify"
			if err := apiClient.Post(ctx, path, body, nil); err != nil {
				return err
			}

			return output.WriteResult(w, map[string]any{
				"key":      issueKey,
				"notified": true,
			}, *format)
		}),
	}
}

// POST /rest/api/3/bulk/issues/delete
func issueBulkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "bulk-delete",
		Usage: "Bulk delete issues (up to 1000)",
		UsageText: `jira-agent issue bulk-delete --issues PROJ-1,PROJ-2,PROJ-3
jira-agent issue bulk-delete --issues PROJ-1,PROJ-2 --send-notification=false`,
		Metadata: commandMetadata(writeCommandMetadata(), requiredFlagMetadata("issues")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "issues",
				Usage: "Comma-separated issue keys or IDs (required)",
			},
			&cli.BoolFlag{
				Name:  "send-notification",
				Usage: "Send bulk notification email",
				Value: true,
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			issueStr, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}
			issues := splitTrimmed(issueStr)

			body := map[string]any{
				"selectedIssueIdsOrKeys": issues,
				"sendBulkNotification":   cmd.Bool("send-notification"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/delete", body, result)
			})
		}),
	}
}

// GET /rest/api/3/bulk/issues/fields
func issueBulkEditFieldsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "bulk-edit-fields",
		Usage: "List editable fields for bulk edit",
		UsageText: `jira-agent issue bulk-edit-fields --issues PROJ-1,PROJ-2
jira-agent issue bulk-edit-fields --issues PROJ-1,PROJ-2 --search-text priority`,
		Metadata: requiredFlagMetadata("issues"),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "issues",
				Usage: "Comma-separated issue keys or IDs (required)",
			},
			&cli.StringFlag{
				Name:  "search-text",
				Usage: "Filter fields by name",
			},
			&cli.StringFlag{
				Name:  "ending-before",
				Usage: "End cursor for pagination",
			},
			&cli.StringFlag{
				Name:  "starting-after",
				Usage: "Start cursor for pagination",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			issueStr, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}

			params := map[string]string{"issueIdsOrKeys": issueStr}
			addOptionalParams(cmd, params, map[string]string{
				"search-text":    "searchText",
				"ending-before":  "endingBefore",
				"starting-after": "startingAfter",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/bulk/issues/fields", params, result)
			})
		},
	}
}

// POST /rest/api/3/bulk/issues/fields
func issueBulkEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "bulk-edit",
		Usage:     "Bulk edit issues (up to 1000 issues, 200 fields)",
		UsageText: `jira-agent issue bulk-edit --payload-json '{"selectedIssueIdsOrKeys":["PROJ-1","PROJ-2"],"selectedActions":["priority"],"editedFieldsInput":{"priority":{"name":"High"}}}'`,
		Metadata:  commandMetadata(writeCommandMetadata(), requiredFlagMetadata("payload-json")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "payload-json",
				Usage: "Full JSON payload with selectedIssueIdsOrKeys, selectedActions, editedFieldsInput, and optional sendBulkNotification (required)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body, err := parseJSONPayload(cmd.String("payload-json"), "payload-json")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/fields", body, result)
			})
		}),
	}
}

// POST /rest/api/3/bulk/issues/move
func issueBulkMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "bulk-move",
		Usage:     "Bulk move issues to a target project and issue type (up to 1000)",
		UsageText: `jira-agent issue bulk-move --payload-json '{"targetToSourcesMapping":{"DEST":{"issueIdsOrKeys":["PROJ-1"],"targetIssueType":"Task"}}}'`,
		Metadata:  commandMetadata(writeCommandMetadata(), requiredFlagMetadata("payload-json")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "payload-json",
				Usage: "Full JSON payload with targetToSourcesMapping and optional sendBulkNotification (required)",
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body, err := parseJSONPayload(cmd.String("payload-json"), "payload-json")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/move", body, result)
			})
		}),
	}
}

// GET /rest/api/3/bulk/issues/transition
func issueBulkTransitionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "bulk-transitions",
		Usage:     "List available transitions for bulk transition",
		UsageText: `jira-agent issue bulk-transitions --issues PROJ-1,PROJ-2,PROJ-3`,
		Metadata:  requiredFlagMetadata("issues"),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "issues",
				Usage: "Comma-separated issue keys or IDs (required)",
			},
			&cli.StringFlag{
				Name:  "ending-before",
				Usage: "End cursor for pagination",
			},
			&cli.StringFlag{
				Name:  "starting-after",
				Usage: "Start cursor for pagination",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			issueStr, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}

			params := map[string]string{"issueIdsOrKeys": issueStr}
			addOptionalParams(cmd, params, map[string]string{
				"ending-before":  "endingBefore",
				"starting-after": "startingAfter",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/bulk/issues/transition", params, result)
			})
		},
	}
}

// POST /rest/api/3/bulk/issues/transition
func issueBulkTransitionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "bulk-transition",
		Usage: "Bulk transition issue statuses (up to 1000)",
		UsageText: `jira-agent issue bulk-transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]'
jira-agent issue bulk-transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]' --send-notification=false`,
		Metadata: commandMetadata(writeCommandMetadata(), requiredFlagMetadata("transitions-json")),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "transitions-json",
				Usage: `JSON array of transition inputs, e.g. [{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}] (required)`,
			},
			&cli.BoolFlag{
				Name:  "send-notification",
				Usage: "Send bulk notification email",
				Value: true,
			},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			jsonStr := cmd.String("transitions-json")
			if jsonStr == "" {
				return apperr.NewValidationError("--transitions-json is required", nil)
			}

			var inputs []any
			if err := json.Unmarshal([]byte(jsonStr), &inputs); err != nil {
				return apperr.NewValidationError(
					fmt.Sprintf("invalid --transitions-json: %v", err),
					err,
				)
			}
			if len(inputs) == 0 {
				return apperr.NewValidationError("--transitions-json must include at least one transition input", nil)
			}

			body := map[string]any{
				"bulkTransitionInputs": inputs,
				"sendBulkNotification": cmd.Bool("send-notification"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/transition", body, result)
			})
		}),
	}
}

// GET /rest/api/3/bulk/queue/{taskId}
func issueBulkStatusCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "bulk-status",
		Usage:     "Get bulk issue operation progress",
		UsageText: `jira-agent issue bulk-status 10641`,
		ArgsUsage: "<task-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			taskID, err := requireArg(cmd, "task ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/bulk/queue/"+taskID, nil, result)
			})
		},
	}
}

// parseJSONPayload validates and parses a raw JSON string flag into a map.
func parseJSONPayload(jsonStr, flagName string) (map[string]any, error) {
	if jsonStr == "" {
		return nil, apperr.NewValidationError("--"+flagName+" is required", nil)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &body); err != nil {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("invalid --%s: %v", flagName, err),
			err,
		)
	}

	return body, nil
}

func mergePayloadJSON(body map[string]any, jsonStr string) error {
	if jsonStr == "" {
		return nil
	}
	payload, err := parseJSONPayload(jsonStr, "payload-json")
	if err != nil {
		return err
	}
	maps.Copy(body, payload)
	return nil
}

// findTransitionID searches the transitions response for a transition matching
// the target name (case-insensitive). Checks both the transition name and the
// target status name in the "to" object.
func findTransitionID(transitions any, targetStatus string) (string, error) {
	m, ok := transitions.(map[string]any)
	if !ok {
		return "", apperr.NewValidationError("unexpected transitions response format", nil)
	}
	transSlice, ok := m["transitions"].([]any)
	if !ok {
		return "", apperr.NewValidationError("no transitions found in response", nil)
	}

	var available []string

	for _, t := range transSlice {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}

		name, _ := tm["name"].(string)
		if name != "" {
			available = append(available, name)
		}

		// Match against the transition name itself.
		if strings.EqualFold(name, targetStatus) {
			if id, ok := tm["id"].(string); ok {
				return id, nil
			}
		}

		// Match against the target status name ("to" object).
		if to, ok := tm["to"].(map[string]any); ok {
			if toName, ok := to["name"].(string); ok && strings.EqualFold(toName, targetStatus) {
				if id, ok := tm["id"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", apperr.NewNotFoundError(
		fmt.Sprintf("no transition found for status %q", targetStatus),
		nil,
		apperr.WithDetails(fmt.Sprintf("available transitions: %s", strings.Join(available, ", "))),
	)
}

// findIssueTypeID searches a paginated issue type list for a type matching
// the given name (case-insensitive) and returns its ID.
func findIssueTypeID(typeList any, typeName string) (string, error) {
	m, ok := typeList.(map[string]any)
	if !ok {
		return "", apperr.NewValidationError("unexpected issue type response format", nil)
	}
	values, ok := m["values"].([]any)
	if !ok {
		return "", apperr.NewValidationError("no issue types found in response", nil)
	}

	var available []string
	for _, v := range values {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name, _ := vm["name"].(string)
		if name != "" {
			available = append(available, name)
		}
		if strings.EqualFold(name, typeName) {
			if id, ok := vm["id"].(string); ok {
				return id, nil
			}
		}
	}

	return "", apperr.NewNotFoundError(
		fmt.Sprintf("issue type %q not found in project", typeName),
		nil,
		apperr.WithDetails(fmt.Sprintf("available types: %s", strings.Join(available, ", "))),
	)
}
