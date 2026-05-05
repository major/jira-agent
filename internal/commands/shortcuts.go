package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// issueMineCommand creates a shortcut that lists issues assigned to the current
// user. It builds a fixed JQL query with an optional --status filter and
// delegates to the same POST /search/jql endpoint used by issue search.
func issueMineCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mine",
		Short: "List issues assigned to you",
		Example: `jira-agent issue mine
jira-agent issue mine --status "In Progress"
jira-agent issue mine --fields key,summary,status,priority
jira-agent issue mine --fields-preset triage`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := applyFieldsPreset(cmd); err != nil {
				return err
			}

			jql := buildMineJQL(mustGetString(cmd, "status"))
			body := buildShortcutSearchBody(cmd, jql)

			return runShortcutSearch(cmd, apiClient, w, *format, body)
		},
	}
	cmd.Flags().String("status", "", "Filter by status (e.g. \"In Progress\")")
	cmd.Flags().String("fields", "key,summary,status,assignee,priority", "Comma-separated field list")
	cmd.Flags().String("fields-preset", "", "Named field preset: minimal, triage, or detail")
	appendPaginationFlags(cmd)
	markMutuallyExclusive(cmd, "fields-preset", "fields")
	SetCommandCategory(cmd, commandCategoryRead)
	return cmd
}

// issueRecentCommand creates a shortcut that lists recently updated issues
// where the current user is the assignee, reporter, or watcher. The --since
// flag controls the time window (default 7d).
func issueRecentCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "List recently updated issues you are involved in",
		Example: `jira-agent issue recent
jira-agent issue recent --since 1d
jira-agent issue recent --since 30d --max-results 20
jira-agent issue recent --fields-preset minimal`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := applyFieldsPreset(cmd); err != nil {
				return err
			}

			jql := buildRecentJQL(mustGetString(cmd, "since"))
			body := buildShortcutSearchBody(cmd, jql)

			return runShortcutSearch(cmd, apiClient, w, *format, body)
		},
	}
	cmd.Flags().String("since", "7d", "Time window (e.g. 1d, 7d, 30d)")
	cmd.Flags().String("fields", "key,summary,status,assignee,priority", "Comma-separated field list")
	cmd.Flags().String("fields-preset", "", "Named field preset: minimal, triage, or detail")
	cmd.Flags().Int("max-results", 10, "Page size")
	cmd.Flags().Int("start-at", 0, "Pagination offset")
	markMutuallyExclusive(cmd, "fields-preset", "fields")
	SetCommandCategory(cmd, commandCategoryRead)
	return cmd
}

// buildMineJQL constructs the JQL for issue mine. When status is non-empty,
// it adds a status filter before the ORDER BY clause.
func buildMineJQL(status string) string {
	if status != "" {
		return fmt.Sprintf("assignee = currentUser() AND status = '%s' ORDER BY updated DESC", status)
	}
	return "assignee = currentUser() ORDER BY updated DESC"
}

// buildRecentJQL constructs the JQL for issue recent using the given time
// window duration (e.g. "7d", "1d", "30d").
func buildRecentJQL(since string) string {
	return fmt.Sprintf(
		`(assignee = currentUser() OR reporter = currentUser() OR watcher = currentUser()) AND updated >= "-%s" ORDER BY updated DESC`,
		since,
	)
}

// buildShortcutSearchBody constructs the POST body for a shortcut search
// command from the JQL string and standard flags (--fields, --max-results).
func buildShortcutSearchBody(cmd *cobra.Command, jql string) map[string]any {
	body := map[string]any{
		"jql":        jql,
		"maxResults": mustGetInt(cmd, "max-results"),
	}
	if f := issueSearchFields(mustGetString(cmd, "fields")); len(f) > 0 {
		body["fields"] = f
	}
	return body
}

// runShortcutSearch executes a JQL search and writes the result using the same
// flattened output format as issue search for JSON, or the raw result for
// CSV/TSV.
func runShortcutSearch(cmd *cobra.Command, apiClient *client.Ref, w io.Writer, format output.Format, body map[string]any) error {
	ctx := cmd.Context()
	var result any
	if err := apiClient.Post(ctx, "/search/jql", body, &result); err != nil {
		return err
	}
	meta := extractPaginationMeta(cmd, result)
	if !isJSONOutputFormat(format) {
		return output.WriteSuccess(w, result, meta, format, CompactOptsFromCmd(cmd)...)
	}
	return output.WriteSuccess(w, flattenIssueSearchResult(result), meta, format, CompactOptsFromCmd(cmd)...)
}
