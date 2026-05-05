// Package commands implements the CLI command tree for jira-agent.
//
// Each file corresponds to a top-level command group: issue.go for issue
// operations, field.go for field management. Command constructors accept a
// shared client.Ref, writer, and output format pointer configured by the
// root command's Before hook.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// IssueCommand returns the "issue" parent command with all subcommands.
func IssueCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Issue operations (get, search, create, edit, delete, transition, assign, comment, changelog, rank, count, notify, meta, bulk, property)",
		Example: `jira-agent issue get PROJ-123
jira-agent issue search --jql "project = PROJ AND status = Open"
jira-agent issue create --project PROJ --type Story --summary "New feature"
jira-agent issue transition PROJ-123 --to "In Progress"`,
	}
	cmd.AddCommand(
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
	)
	return cmd
}

func issueBulkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk issue operations",
		Example: `jira-agent issue bulk fetch --issues PROJ-1,PROJ-2 --fields key,summary,status
jira-agent issue bulk create --issues-json '[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task 1"}}]'
jira-agent issue bulk transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]'`,
	}
	cmd.AddCommand(
		canonicalIssueBulkCommand(issueBulkCreateCommand(apiClient, w, format, allowWrites), "create"),
		canonicalIssueBulkCommand(issueBulkFetchCommand(apiClient, w, format), "fetch"),
		canonicalIssueBulkCommand(issueBulkDeleteCommand(apiClient, w, format, allowWrites), "delete"),
		canonicalIssueBulkCommand(issueBulkEditFieldsCommand(apiClient, w, format), "edit-fields"),
		canonicalIssueBulkCommand(issueBulkEditCommand(apiClient, w, format, allowWrites), "edit"),
		canonicalIssueBulkCommand(issueBulkMoveCommand(apiClient, w, format, allowWrites), "move"),
		canonicalIssueBulkCommand(issueBulkTransitionsCommand(apiClient, w, format), "transitions"),
		canonicalIssueBulkCommand(issueBulkTransitionCommand(apiClient, w, format, allowWrites), "transition"),
		canonicalIssueBulkCommand(issueBulkStatusCommand(apiClient, w, format), "status"),
	)
	return cmd
}

func canonicalIssueBulkCommand(cmd *cobra.Command, name string) *cobra.Command {
	legacyInvocation := "jira-agent issue " + cmd.Name()
	parts := strings.SplitN(cmd.Use, " ", 2)
	cmd.Use = name
	if len(parts) == 2 {
		cmd.Use += " " + parts[1]
	}
	cmd.Example = strings.ReplaceAll(cmd.Example, legacyInvocation, "jira-agent issue bulk "+name)
	return cmd
}

func legacyIssueBulkCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Hidden = true
	return cmd
}

func issueBulkCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bulk-create",
		Short:   "Create multiple issues from Jira issueUpdates JSON",
		Example: `jira-agent issue bulk-create --issues-json '[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task 1"}}]'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := parseBulkCreateBody(mustGetString(cmd, "issues-json"))
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/bulk", body, result)
			})
		}),
	}
	cmd.Flags().String("issues-json", "", "JSON array of issueUpdates or object with issueUpdates (required, max 50 issues)")
	return cmd
}

func issueBulkFetchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-fetch",
		Short: "Fetch multiple issues by key or ID",
		Example: `jira-agent issue bulk-fetch --issues PROJ-1,PROJ-2,PROJ-3
jira-agent issue bulk-fetch --issues PROJ-1,PROJ-2 --fields key,summary,status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := buildBulkFetchBody(cmd)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/bulkfetch", body, result)
			})
		},
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys or IDs (required, max 100 issues)")
	cmd.Flags().String("fields", "", "Comma-separated field list")
	cmd.Flags().String("expand", "", "Comma-separated expansions (names, schema, changelog, operations)")
	cmd.Flags().String("properties", "", "Comma-separated issue properties to include (max 5)")
	cmd.Flags().Bool("fields-by-keys", false, "Treat field identifiers as field keys instead of field IDs")
	return cmd
}

func issueGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <issue-key>",
		Short: "Get issue by key or ID",
		Example: `jira-agent issue get PROJ-123
jira-agent issue get PROJ-123 --fields key,summary,status,assignee
jira-agent issue get PROJ-123 --fields-preset minimal
jira-agent issue get PROJ-123 --fields description --description-output-format markdown
jira-agent issue get PROJ-123 --expand changelog
jira-agent issue get PROJ-123 --raw`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			if err := applyFieldsPreset(cmd); err != nil {
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
			if cmd.Flags().Changed("fail-fast") {
				params["failFast"] = fmt.Sprintf("%t", mustGetBool(cmd, "fail-fast"))
			}

			if mustGetBool(cmd, "raw") {
				return writeRawAPIResult(w, *format, func(result any) error {
					return apiClient.Get(ctx, "/issue/"+key, params, result)
				})
			}

			descriptionFormat, err := parseDescriptionOutputFormat(mustGetString(cmd, "description-output-format"))
			if err != nil {
				return err
			}

			var result any
			if err := apiClient.Get(ctx, "/issue/"+key, params, &result); err != nil {
				return err
			}
			return output.WriteResult(w, convertDescriptionOutputFields(result, descriptionFormat), *format, CompactOptsFromCmd(cmd)...)
		},
	}
	cmd.Flags().String("fields", "", "Comma-separated field list (default: all navigable)")
	cmd.Flags().String("fields-preset", "", "Named field preset: minimal, triage, or detail")
	cmd.Flags().String("expand", "", "Comma-separated expansions (names, schema, changelog, operations)")
	cmd.Flags().String("properties", "", "Comma-separated issue properties to include")
	cmd.Flags().String("description-output-format", descriptionOutputFormatText, "Description output format: text, markdown, or adf")
	cmd.Flags().Bool("fields-by-keys", false, "Treat field identifiers as field keys instead of field IDs")
	cmd.Flags().Bool("update-history", false, "Add the issue to the user's recently viewed history")
	cmd.Flags().Bool("fail-fast", true, "Fail immediately if a requested field cannot be resolved")
	cmd.Flags().Bool("raw", false, "Return the unmodified Jira API response for JSON output")
	markMutuallyExclusive(cmd, "fields-preset", "fields")
	return cmd
}

func issuePickerCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "picker",
		Short: "Find issues for picker UI",
		Example: `jira-agent issue picker --query PROJ-123
jira-agent issue picker --query login --current-jql "project = PROJ" --show-subtasks=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{
				"query":              "query",
				"current-jql":        "currentJQL",
				"current-issue-key":  "currentIssueKey",
				"current-project-id": "currentProjectId",
			})
			if cmd.Flags().Changed("show-subtasks") {
				params["showSubTasks"] = fmt.Sprintf("%t", mustGetBool(cmd, "show-subtasks"))
			}
			if cmd.Flags().Changed("show-subtask-parent") {
				params["showSubTaskParent"] = fmt.Sprintf("%t", mustGetBool(cmd, "show-subtask-parent"))
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/picker", params, result)
			})
		},
	}
	cmd.Flags().String("query", "", "Issue key, summary text, or search text")
	cmd.Flags().String("current-jql", "", "JQL context for current search results")
	cmd.Flags().String("current-issue-key", "", "Current issue key for context")
	cmd.Flags().String("current-project-id", "", "Current project ID for context")
	cmd.Flags().Bool("show-subtasks", true, "Include subtasks in picker results")
	cmd.Flags().Bool("show-subtask-parent", true, "Show parent summaries for subtasks")
	return cmd
}

func issueSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search issues via JQL",
		Example: `jira-agent issue search --jql "project = PROJ AND status = Open"
jira-agent issue search --jql "assignee = currentUser()" --fields key,summary,status
jira-agent issue search --jql "assignee = currentUser()" --fields-preset triage
jira-agent issue search --jql "project = PROJ" --fields key,description --description-output-format markdown
jira-agent issue search --jql "project = PROJ" --max-results 10 --order-by created --order desc
jira-agent issue search --jql "project = PROJ" --raw`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := applyFieldsPreset(cmd); err != nil {
				return err
			}

			jql, body, err := buildIssueSearchBody(cmd)
			if err != nil {
				return err
			}
			_ = jql

			if mustGetBool(cmd, "raw") {
				return writeRawPaginatedAPIResult(cmd, w, *format, func(result any) error {
					return apiClient.Post(ctx, "/search/jql", body, result)
				})
			}

			descriptionFormat, err := parseDescriptionOutputFormat(mustGetString(cmd, "description-output-format"))
			if err != nil {
				return err
			}

			var result any
			if err := apiClient.Post(ctx, "/search/jql", body, &result); err != nil {
				return err
			}
			meta := extractPaginationMeta(cmd, result)
			if !isJSONOutputFormat(*format) {
				return output.WriteSuccess(w, convertDescriptionOutputFields(result, descriptionFormat), meta, *format, CompactOptsFromCmd(cmd)...)
			}
			return output.WriteSuccess(w, flattenIssueSearchResultWithDescriptionFormat(result, descriptionFormat), meta, *format, CompactOptsFromCmd(cmd)...)
		},
	}
	cmd.Flags().String("jql", "", "JQL query string (required)")
	cmd.Flags().String("fields", "key,summary,status,assignee,priority", "Comma-separated field list")
	cmd.Flags().String("fields-preset", "", "Named field preset: minimal, triage, or detail")
	cmd.Flags().Int("max-results", 50, "Page size")
	cmd.Flags().String("next-page-token", "", "Token for fetching next page of results")
	cmd.Flags().String("expand", "", "Comma-separated expansions (names, schema, changelog, operations)")
	cmd.Flags().String("properties", "", "Comma-separated issue properties to include")
	cmd.Flags().String("description-output-format", descriptionOutputFormatText, "Description output format: text, markdown, or adf")
	cmd.Flags().Bool("fields-by-keys", false, "Treat field identifiers as field keys instead of field IDs")
	cmd.Flags().Bool("fail-fast", true, "Fail immediately if a requested field cannot be resolved")
	cmd.Flags().String("reconcile-issues", "", "Comma-separated issue IDs to reconcile for approximate-count drift")
	cmd.Flags().String("order-by", "", "Sort field (appended to JQL as ORDER BY clause)")
	cmd.Flags().String("order", "", "Sort direction: asc or desc (used with --order-by)")
	cmd.Flags().Bool("raw", false, "Return the unmodified Jira API response for JSON output")
	markMutuallyExclusive(cmd, "fields-preset", "fields")
	return cmd
}

func issueCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new issue",
		Example: `jira-agent issue create --project PROJ --type Story --summary "New feature"
jira-agent issue create --project PROJ --type Bug --summary "Fix login" --priority High --labels bug,urgent
jira-agent issue create --project PROJ --type Task --summary "Subtask" --parent PROJ-100`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Full payload mode: --payload-json provides the complete issue
			// body and is mutually exclusive with individual field flags.
			if payloadJSON := mustGetString(cmd, "payload-json"); payloadJSON != "" {
				body := map[string]any{}
				if err := mergePayloadJSON(body, payloadJSON); err != nil {
					return err
				}
				// Inject project from --project when the payload omits it.
				if project := resolveProject(cmd); project != "" {
					fields, ok := body["fields"].(map[string]any)
					if !ok {
						fields = map[string]any{}
						body["fields"] = fields
					}
					if _, hasProject := fields["project"]; !hasProject {
						fields["project"] = map[string]any{"key": project}
					}
				}
				return writeAPIResult(w, *format, func(result any) error {
					return apiClient.Post(ctx, "/issue", body, result)
				})
			}

			// Individual flags mode.
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

			if err := applyCommonFields(fields, cmd); err != nil {
				return err
			}

			if err := applyMerges(fields, cmd); err != nil {
				return err
			}

			body := map[string]any{"fields": fields}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue", body, result)
			})
		}),
	}
	cmd.Flags().String("type", "", "Issue type name (e.g. Story, Bug, Task)")
	cmd.Flags().String("summary", "", "Issue summary")
	cmd.Flags().String("description", "", "Description text, ADF JSON, or wiki markup with --description-format")
	cmd.Flags().String("description-format", "auto", "Description input format: auto, plain, adf, or wiki")
	cmd.Flags().String("assignee", "", "Assignee account ID")
	cmd.Flags().String("priority", "", "Priority name (e.g. High, Medium, Low)")
	cmd.Flags().String("labels", "", "Comma-separated labels")
	cmd.Flags().String("components", "", "Comma-separated component names")
	cmd.Flags().String("parent", "", "Parent issue key (for subtasks/child issues)")
	cmd.Flags().StringToString("field", map[string]string{}, "Custom field value (key=value, repeatable)")
	cmd.Flags().String("fields-json", "", "JSON object of fields (alternative to individual flags)")
	cmd.Flags().String("payload-json", "", "Full JSON issue create payload, merged after field flags")
	for _, flag := range []string{"summary", "type", "description", "assignee", "priority", "labels", "components", "parent", "field", "fields-json"} {
		markMutuallyExclusive(cmd, "payload-json", flag)
	}
	return cmd
}

func issueEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <issue-key>",
		Short: "Update issue fields",
		Example: `jira-agent issue edit PROJ-123 --summary "Updated summary"
jira-agent issue edit PROJ-123 --priority High --labels bug,critical
jira-agent issue edit PROJ-123 --fields-json '{"customfield_10001":"value"}'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			fields := map[string]any{}

			if v := mustGetString(cmd, "summary"); v != "" {
				fields["summary"] = v
			}

			if err := applyCommonFields(fields, cmd); err != nil {
				return err
			}

			if err := applyMerges(fields, cmd); err != nil {
				return err
			}

			body := map[string]any{"fields": fields}
			if err := mergePayloadJSON(body, mustGetString(cmd, "payload-json")); err != nil {
				return err
			}
			if len(fields) == 0 && mustGetString(cmd, "payload-json") == "" {
				return apperr.NewValidationError("at least one field or --payload-json update is required", nil)
			}

			path := "/issue/" + key
			if !mustGetBool(cmd, "notify") {
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
	cmd.Flags().String("summary", "", "Issue summary")
	cmd.Flags().String("description", "", "Description text, ADF JSON, or wiki markup with --description-format")
	cmd.Flags().String("description-format", "auto", "Description input format: auto, plain, adf, or wiki")
	cmd.Flags().String("assignee", "", "Assignee account ID")
	cmd.Flags().String("priority", "", "Priority name")
	cmd.Flags().String("labels", "", "Comma-separated labels")
	cmd.Flags().String("components", "", "Comma-separated component names")
	cmd.Flags().String("parent", "", "Parent issue key")
	cmd.Flags().StringToString("field", map[string]string{}, "Custom field value (key=value, repeatable)")
	cmd.Flags().String("fields-json", "", "JSON object of fields")
	cmd.Flags().String("payload-json", "", "Full JSON issue edit payload, merged after field flags")
	cmd.Flags().Bool("notify", true, "Send notification to watchers")
	return cmd
}

func issueTransitionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transition <issue-key>",
		Short: "Transition issue to a new status",
		Example: `jira-agent issue transition PROJ-123 --list
jira-agent issue transition PROJ-123 --to "In Progress"
jira-agent issue transition PROJ-123 --to Done --comment "Completed"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
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
			if mustGetBool(cmd, "list") || mustGetString(cmd, "transition-id") == "" {
				if err := apiClient.Get(ctx, "/issue/"+key+"/transitions", params, &transitions); err != nil {
					return err
				}
			}

			if mustGetBool(cmd, "list") {
				return output.WriteResult(w, transitions, *format)
			}

			if err := requireWriteAccess(allowWrites); err != nil {
				return err
			}

			targetStatus := mustGetString(cmd, "to")
			transitionID := mustGetString(cmd, "transition-id")
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

			if fieldMap := mustGetStringToString(cmd, "field"); len(fieldMap) > 0 {
				transFields := map[string]any{}
				applyFieldOverrides(transFields, fieldMap)
				reqBody["fields"] = transFields
			}

			if comment := mustGetString(cmd, "comment"); comment != "" {
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
			if err := mergePayloadJSON(reqBody, mustGetString(cmd, "payload-json")); err != nil {
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
	cmd.Flags().String("to", "", "Target status or transition name")
	cmd.Flags().String("transition-id", "", "Transition ID to apply directly")
	cmd.Flags().StringToString("field", map[string]string{}, "Transition screen field (key=value, repeatable)")
	cmd.Flags().String("comment", "", "Comment to add during transition")
	cmd.Flags().String("payload-json", "", "Full JSON transition payload, merged after flags")
	cmd.Flags().String("expand", "", "Comma-separated transition expansions (transitions.fields)")
	cmd.Flags().String("list-transition-id", "", "Only return this transition ID when listing transitions")
	cmd.Flags().Bool("include-unavailable-transitions", false, "Include transitions unavailable to the current user")
	cmd.Flags().Bool("skip-remote-only-condition", false, "Skip remote-only workflow conditions")
	cmd.Flags().Bool("sort-by-ops-bar-and-status", false, "Sort listed transitions by ops bar sequence and status")
	cmd.Flags().Bool("list", false, "List available transitions instead of performing one")
	markMutuallyExclusive(cmd, "to", "transition-id")
	return cmd
}

// DELETE /rest/api/3/issue/{issueIdOrKey}
func issueDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <issue-key>",
		Short: "Delete an issue",
		Example: `jira-agent issue delete PROJ-123
jira-agent issue delete PROJ-123 --delete-subtasks`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			path := "/issue/" + key
			if mustGetBool(cmd, "delete-subtasks") {
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
	cmd.Flags().Bool("delete-subtasks", false, "Delete subtasks of the issue")
	return cmd
}

func issueChangelogCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changelog <issue-key>",
		Short: "List changelog entries for an issue",
		Example: `jira-agent issue changelog PROJ-123
jira-agent issue changelog PROJ-123 --max-results 10
jira-agent issue changelog list-by-ids PROJ-123 --ids 10001,10002
jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/changelog", params, result)
			})
		},
	}
	cmd.AddCommand(
		issueChangelogListByIDsCommand(apiClient, w, format),
		issueChangelogBulkFetchCommand(apiClient, w, format),
	)
	appendPaginationFlags(cmd)
	return cmd
}

func issueChangelogListByIDsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-by-ids <issue-key>",
		Short:   "Get issue changelogs by IDs",
		Example: `jira-agent issue changelog list-by-ids PROJ-123 --ids 10001,10002`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
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
			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/"+key+"/changelog/list", body, result)
			})
		},
	}
	cmd.Flags().String("ids", "", "Comma-separated changelog IDs (required)")
	return cmd
}

func issueChangelogBulkFetchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-fetch",
		Short: "Bulk fetch changelogs for issues",
		Example: `jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2
jira-agent issue changelog bulk-fetch --issues PROJ-1,PROJ-2 --field-ids status,assignee --max-results 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issuesFlag, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}
			issues := splitTrimmed(issuesFlag)
			if len(issues) > 1000 {
				return apperr.NewValidationError("--issues supports at most 1000 issues", nil)
			}

			body := map[string]any{"issueIdsOrKeys": issues}
			if fields := splitTrimmed(mustGetString(cmd, "field-ids")); len(fields) > 0 {
				if len(fields) > 10 {
					return apperr.NewValidationError("--field-ids supports at most 10 fields", nil)
				}
				body["fieldIds"] = fields
			}
			if cmd.Flags().Changed("max-results") {
				body["maxResults"] = mustGetInt(cmd, "max-results")
			}
			if token := mustGetString(cmd, "next-page-token"); token != "" {
				body["nextPageToken"] = token
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/changelog/bulkfetch", body, result)
			})
		},
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys or IDs (required, max 1000)")
	cmd.Flags().String("field-ids", "", "Comma-separated field IDs to filter changelogs (max 10)")
	cmd.Flags().Int("max-results", 1000, "Maximum changelog items to return")
	cmd.Flags().String("next-page-token", "", "Cursor token from previous response")
	return cmd
}

func issueRankCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank issues before or after a target issue",
		Example: `jira-agent issue rank --issues PROJ-1,PROJ-2 --before PROJ-10
jira-agent issue rank --issues PROJ-5 --after PROJ-3`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			before := mustGetString(cmd, "before")
			after := mustGetString(cmd, "after")

			if before == "" && after == "" {
				return apperr.NewValidationError("either --before or --after is required", nil)
			}

			if before != "" && after != "" {
				return apperr.NewValidationError("--before and --after are mutually exclusive", nil)
			}

			issues := splitTrimmed(mustGetString(cmd, "issues"))
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
	cmd.Flags().String("issues", "", "Comma-separated issue keys to rank")
	_ = cmd.MarkFlagRequired("issues")
	cmd.Flags().String("before", "", "Issue key to rank before (mutually exclusive with --after)")
	cmd.Flags().String("after", "", "Issue key to rank after (mutually exclusive with --before)")
	return cmd
}

func issueCountCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "count",
		Short: "Count issues matching a JQL query",
		Example: `jira-agent issue count --jql "project = PROJ AND status = Open"
jira-agent issue count --jql "assignee = currentUser() AND resolution = Unresolved"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			jql, err := requireFlag(cmd, "jql")
			if err != nil {
				return err
			}

			body := map[string]any{
				"jql":        jql,
				"maxResults": 0,
			}

			var result any
			if err := apiClient.Post(ctx, "/search/jql", body, &result); err != nil {
				return err
			}
			meta := extractPaginationMeta(cmd, result)
			return output.WriteSuccess(w, map[string]any{"total": meta.Total}, meta, *format)
		},
	}
	cmd.Flags().String("jql", "", "JQL query to count results for (required)")
	return cmd
}

func issueMetaCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Get create or edit metadata for issues",
		Example: `jira-agent issue meta --project PROJ
jira-agent issue meta --project PROJ --type Story
jira-agent issue meta --operation edit --issue PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			operation := mustGetString(cmd, "operation")

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

				typeName := mustGetString(cmd, "type")
				if typeName == "" {
					// List available issue types for the project.
					path := "/issue/createmeta/" + project + "/issuetypes"
					return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
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
				return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
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
	cmd.Flags().String("type", "", "Issue type name (filters to a single type)")
	cmd.Flags().String("operation", "create", "Operation: create or edit")
	cmd.Flags().String("issue", "", "Issue key (required for edit metadata)")
	return cmd
}

func mustGetString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func mustGetBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func mustGetInt(cmd *cobra.Command, name string) int {
	value, _ := cmd.Flags().GetInt(name)
	return value
}

func mustGetStringSlice(cmd *cobra.Command, name string) []string {
	value, _ := cmd.Flags().GetStringSlice(name)
	return value
}

func mustGetStringToString(cmd *cobra.Command, name string) map[string]string {
	value, _ := cmd.Flags().GetStringToString(name)
	return value
}

// applyCommonFields sets optional issue fields that are shared between create
// and edit: description, assignee, priority, labels, components, and parent.
// Each field is only set when the corresponding flag has a non-empty value.
func applyCommonFields(fields map[string]any, cmd *cobra.Command) error {
	if v := mustGetString(cmd, "description"); v != "" {
		description, err := descriptionToADF(v, mustGetString(cmd, "description-format"))
		if err != nil {
			return err
		}
		fields["description"] = description
	}
	if v := mustGetString(cmd, "assignee"); v != "" {
		fields["assignee"] = map[string]any{"accountId": v}
	}
	if v := mustGetString(cmd, "priority"); v != "" {
		fields["priority"] = map[string]any{"name": v}
	}
	if v := mustGetString(cmd, "labels"); v != "" {
		fields["labels"] = splitTrimmed(v)
	}
	if v := mustGetString(cmd, "components"); v != "" {
		names := splitTrimmed(v)
		comps := make([]map[string]any, len(names))
		for i, n := range names {
			comps[i] = map[string]any{"name": n}
		}
		fields["components"] = comps
	}
	if v := mustGetString(cmd, "parent"); v != "" {
		fields["parent"] = map[string]any{"key": v}
	}
	return nil
}

// applyMerges applies --field overrides and --fields-json merges to the fields
// map. This consolidates the two-step merge pattern used by create and edit.
func applyMerges(fields map[string]any, cmd *cobra.Command) error {
	applyFieldOverrides(fields, mustGetStringToString(cmd, "field"))
	return applyFieldsJSON(fields, mustGetString(cmd, "fields-json"))
}

// resolveProject returns the project key from the command's own --project flag,
// falling back to the root command's --project/-p flag (which also reads
// JIRA_PROJECT via its Sources).
func resolveProject(cmd *cobra.Command) string {
	project, _ := cmd.Flags().GetString("project")
	return project
}

func requireProject(cmd *cobra.Command) (string, error) {
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
	return plainTextADF(text)
}

func descriptionToADF(text, format string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "auto":
		return toADF(text), nil
	case "plain":
		return plainTextADF(text), nil
	case "adf":
		var parsed map[string]any
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("invalid --description ADF JSON: %v", err),
				err,
			)
		}
		if _, hasType := parsed["type"]; !hasType {
			return nil, apperr.NewValidationError(
				"invalid --description ADF JSON: root object must include a type field",
				nil,
			)
		}
		return parsed, nil
	case "wiki":
		return wikiToADF(text), nil
	default:
		return nil, apperr.NewValidationError(
			fmt.Sprintf("invalid --description-format %q", format),
			nil,
			apperr.WithDetails("valid formats: auto, plain, adf, wiki"),
		)
	}
}

func plainTextADF(text string) map[string]any {
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

func wikiToADF(text string) map[string]any {
	lines := strings.Split(text, "\n")
	content := make([]any, 0, len(lines))
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		if level, heading, ok := parseWikiHeading(line); ok {
			content = append(content, headingADF(level, heading))
			i++
			continue
		}

		if item, ok := parseWikiBullet(line); ok {
			items := []any{listItemADF(item)}
			i++
			for i < len(lines) {
				itemLine := strings.TrimSpace(lines[i])
				nextItem, isItem := parseWikiBullet(itemLine)
				if !isItem {
					break
				}
				items = append(items, listItemADF(nextItem))
				i++
			}
			content = append(content, map[string]any{"type": "bulletList", "content": items})
			continue
		}

		paragraphLines := []string{line}
		i++
		for i < len(lines) {
			nextLine := strings.TrimSpace(lines[i])
			if nextLine == "" {
				break
			}
			if _, _, ok := parseWikiHeading(nextLine); ok {
				break
			}
			if _, ok := parseWikiBullet(nextLine); ok {
				break
			}
			paragraphLines = append(paragraphLines, nextLine)
			i++
		}
		content = append(content, paragraphADF(strings.Join(paragraphLines, "\n")))
	}

	if len(content) == 0 {
		content = append(content, paragraphADF(""))
	}

	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

func parseWikiHeading(line string) (level int, heading string, ok bool) {
	if len(line) < len("h1. ") || line[0] != 'h' || line[2] != '.' {
		return 0, "", false
	}
	level, err := strconv.Atoi(line[1:2])
	if err != nil || level < 1 || level > 6 {
		return 0, "", false
	}
	heading = strings.TrimSpace(line[3:])
	if heading == "" {
		return 0, "", false
	}
	return level, heading, true
}

func parseWikiBullet(line string) (string, bool) {
	if !strings.HasPrefix(line, "*") {
		return "", false
	}
	item := strings.TrimSpace(strings.TrimPrefix(line, "*"))
	if item == "" {
		return "", false
	}
	return item, true
}

func headingADF(level int, text string) map[string]any {
	return map[string]any{
		"type":  "heading",
		"attrs": map[string]any{"level": level},
		"content": []any{
			map[string]any{"type": "text", "text": text},
		},
	}
}

func listItemADF(text string) map[string]any {
	return map[string]any{
		"type": "listItem",
		"content": []any{
			paragraphADF(text),
		},
	}
}

func paragraphADF(text string) map[string]any {
	return map[string]any{
		"type": "paragraph",
		"content": []any{
			map[string]any{"type": "text", "text": text},
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

func buildBulkFetchBody(cmd *cobra.Command) (map[string]any, error) {
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
	if fields := splitTrimmed(mustGetString(cmd, "fields")); len(fields) > 0 {
		body["fields"] = fields
	}
	if expand := splitTrimmed(mustGetString(cmd, "expand")); len(expand) > 0 {
		body["expand"] = expand
	}
	if properties := splitTrimmed(mustGetString(cmd, "properties")); len(properties) > 0 {
		if len(properties) > 5 {
			return nil, apperr.NewValidationError("--properties supports at most 5 properties", nil)
		}
		body["properties"] = properties
	}
	if mustGetBool(cmd, "fields-by-keys") {
		body["fieldsByKeys"] = true
	}

	return body, nil
}

func issueAssignCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <issue-key> [account-id]",
		Short: "Assign issue to a user",
		Example: `jira-agent issue assign PROJ-123 5b10a2844c20165700ede21g
jira-agent issue assign PROJ-123 --unassign
jira-agent issue assign PROJ-123 --default`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			var body map[string]any
			switch {
			case mustGetBool(cmd, "unassign"):
				body = map[string]any{"accountId": nil}
			case mustGetBool(cmd, "default"):
				body = map[string]any{"accountId": "-1"}
			default:
				args, err := requireArgs(args, "issue key", "account ID")
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
	cmd.Flags().Bool("unassign", false, "Remove current assignee")
	cmd.Flags().Bool("default", false, "Use project default assignee")
	return cmd
}

// POST /rest/api/3/issue/{issueIdOrKey}/notify
func issueNotifyCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify <issue-key>",
		Short: "Send an email notification for an issue",
		Example: `jira-agent issue notify PROJ-123 --subject "Action needed" --text-body "Please review"
jira-agent issue notify PROJ-123 --subject "Update" --to-watchers --to-assignee`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueKey, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			to := map[string]any{}
			if cmd.Flags().Changed("to-assignee") {
				to["assignee"] = mustGetBool(cmd, "to-assignee")
			}
			if cmd.Flags().Changed("to-reporter") {
				to["reporter"] = mustGetBool(cmd, "to-reporter")
			}
			if cmd.Flags().Changed("to-voters") {
				to["voters"] = mustGetBool(cmd, "to-voters")
			}
			if cmd.Flags().Changed("to-watchers") {
				to["watchers"] = mustGetBool(cmd, "to-watchers")
			}
			if users := mustGetStringSlice(cmd, "to-users"); len(users) > 0 {
				userObjs := make([]map[string]string, len(users))
				for i, u := range users {
					userObjs[i] = map[string]string{"accountId": u}
				}
				to["users"] = userObjs
			}

			body := map[string]any{
				"subject": mustGetString(cmd, "subject"),
			}
			if v := mustGetString(cmd, "text-body"); v != "" {
				body["textBody"] = v
			}
			if v := mustGetString(cmd, "html-body"); v != "" {
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
	cmd.Flags().String("subject", "", "notification subject line")
	_ = cmd.MarkFlagRequired("subject")
	cmd.Flags().String("text-body", "", "plain text body")
	cmd.Flags().String("html-body", "", "HTML body")
	cmd.Flags().Bool("to-assignee", false, "send to assignee")
	cmd.Flags().Bool("to-reporter", false, "send to reporter")
	cmd.Flags().Bool("to-voters", false, "send to voters")
	cmd.Flags().Bool("to-watchers", false, "send to watchers")
	cmd.Flags().StringSlice("to-users", []string{}, "account IDs of additional recipients (repeatable)")
	return cmd
}

// POST /rest/api/3/bulk/issues/delete
func issueBulkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-delete",
		Short: "Bulk delete issues (up to 1000)",
		Example: `jira-agent issue bulk-delete --issues PROJ-1,PROJ-2,PROJ-3
jira-agent issue bulk-delete --issues PROJ-1,PROJ-2 --send-notification=false`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueStr, err := requireFlag(cmd, "issues")
			if err != nil {
				return err
			}
			issues := splitTrimmed(issueStr)

			body := map[string]any{
				"selectedIssueIdsOrKeys": issues,
				"sendBulkNotification":   mustGetBool(cmd, "send-notification"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/delete", body, result)
			})
		}),
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys or IDs (required)")
	cmd.Flags().Bool("send-notification", true, "Send bulk notification email")
	return cmd
}

// GET /rest/api/3/bulk/issues/fields
func issueBulkEditFieldsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-edit-fields",
		Short: "List editable fields for bulk edit",
		Example: `jira-agent issue bulk-edit-fields --issues PROJ-1,PROJ-2
jira-agent issue bulk-edit-fields --issues PROJ-1,PROJ-2 --search-text priority`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	cmd.Flags().String("issues", "", "Comma-separated issue keys or IDs (required)")
	cmd.Flags().String("search-text", "", "Filter fields by name")
	cmd.Flags().String("ending-before", "", "End cursor for pagination")
	cmd.Flags().String("starting-after", "", "Start cursor for pagination")
	return cmd
}

// POST /rest/api/3/bulk/issues/fields
func issueBulkEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bulk-edit",
		Short:   "Bulk edit issues (up to 1000 issues, 200 fields)",
		Example: `jira-agent issue bulk-edit --payload-json '{"selectedIssueIdsOrKeys":["PROJ-1","PROJ-2"],"selectedActions":["priority"],"editedFieldsInput":{"priority":{"name":"High"}}}'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := parseJSONPayload(mustGetString(cmd, "payload-json"), "payload-json")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/fields", body, result)
			})
		}),
	}
	cmd.Flags().String("payload-json", "", "Full JSON payload with selectedIssueIdsOrKeys, selectedActions, editedFieldsInput, and optional sendBulkNotification (required)")
	return cmd
}

// POST /rest/api/3/bulk/issues/move
func issueBulkMoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bulk-move",
		Short:   "Bulk move issues to a target project and issue type (up to 1000)",
		Example: `jira-agent issue bulk-move --payload-json '{"targetToSourcesMapping":{"DEST":{"issueIdsOrKeys":["PROJ-1"],"targetIssueType":"Task"}}}'`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := parseJSONPayload(mustGetString(cmd, "payload-json"), "payload-json")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/move", body, result)
			})
		}),
	}
	cmd.Flags().String("payload-json", "", "Full JSON payload with targetToSourcesMapping and optional sendBulkNotification (required)")
	return cmd
}

// GET /rest/api/3/bulk/issues/transition
func issueBulkTransitionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bulk-transitions",
		Short:   "List available transitions for bulk transition",
		Example: `jira-agent issue bulk-transitions --issues PROJ-1,PROJ-2,PROJ-3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	cmd.Flags().String("issues", "", "Comma-separated issue keys or IDs (required)")
	cmd.Flags().String("ending-before", "", "End cursor for pagination")
	cmd.Flags().String("starting-after", "", "Start cursor for pagination")
	return cmd
}

// POST /rest/api/3/bulk/issues/transition
func issueBulkTransitionCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-transition",
		Short: "Bulk transition issue statuses (up to 1000)",
		Example: `jira-agent issue bulk-transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]'
jira-agent issue bulk-transition --transitions-json '[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]' --send-notification=false`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			jsonStr := mustGetString(cmd, "transitions-json")
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
				"sendBulkNotification": mustGetBool(cmd, "send-notification"),
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/bulk/issues/transition", body, result)
			})
		}),
	}
	cmd.Flags().String("transitions-json", "", `JSON array of transition inputs, e.g. [{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}] (required)`)
	cmd.Flags().Bool("send-notification", true, "Send bulk notification email")
	return cmd
}

// GET /rest/api/3/bulk/queue/{taskId}
func issueBulkStatusCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bulk-status <task-id>",
		Short:   "Get bulk issue operation progress",
		Example: `jira-agent issue bulk-status 10641`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			taskID, err := requireArg(args, "task ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/bulk/queue/"+taskID, nil, result)
			})
		},
	}
	return cmd
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
