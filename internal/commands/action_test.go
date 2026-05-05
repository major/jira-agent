package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
	"github.com/major/jira-agent/internal/testhelpers"
)

func testCommandFormat() *output.Format {
	f := output.FormatJSON
	return &f
}

func testCommandClient(baseURL string) *client.Ref {
	return &client.Ref{Client: client.NewClient("Basic token", client.WithBaseURL(baseURL), client.WithAgileBaseURL(baseURL))}
}

func testAllowWrites() *bool {
	v := true
	return &v
}

func testDryRun() *bool {
	v := false
	return &v
}

func runCommandAction(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetOut(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %q failed: %v", cmd.Name(), err)
	}
	return buf.String()
}

func prepareCommandForTest(cmd *cobra.Command) {
	cmd.SetContext(context.Background())
	if cmd.RunE != nil && len(cmd.Commands()) > 0 {
		cmd.Args = cobra.ArbitraryArgs
	}
	if cmd.Flags().Lookup("project") == nil {
		cmd.Flags().String("project", "", "")
	}
	if cmd.Flags().Lookup("start-at") == nil {
		cmd.Flags().Int("start-at", 0, "")
	}
	if cmd.Flags().Lookup("max-results") == nil {
		cmd.Flags().Int("max-results", 50, "")
	}
	if cmd.Flags().Lookup("order-by") == nil {
		cmd.Flags().String("order-by", "", "")
	}
	if cmd.Flags().Lookup("expand") == nil {
		cmd.Flags().String("expand", "", "")
	}
	for _, sub := range cmd.Commands() {
		prepareCommandForTest(sub)
	}
}

func TestCommandGroups_RegisterSubcommands(t *testing.T) {
	t.Parallel()

	apiClient := &client.Ref{}
	format := testCommandFormat()
	var buf bytes.Buffer

	tests := []struct {
		name string
		cmd  *cobra.Command
		want int
	}{
		{name: "audit", cmd: AuditCommand(apiClient, &buf, format), want: 1},
		{name: "board", cmd: BoardCommand(apiClient, &buf, format, testAllowWrites()), want: 11},
		{name: "field", cmd: FieldCommand(apiClient, &buf, format, testAllowWrites()), want: 4},
		{name: "filter", cmd: FilterCommand(apiClient, &buf, format, testAllowWrites()), want: 10},
		{name: "group", cmd: GroupCommand(apiClient, &buf, format, testAllowWrites()), want: 8},
		{name: "dashboard", cmd: DashboardCommand(apiClient, &buf, format, testAllowWrites()), want: 7},
		{name: "issuetype", cmd: IssueTypeCommand(apiClient, &buf, format), want: 3},
		{name: "label", cmd: LabelCommand(apiClient, &buf, format), want: 1},
		{name: "permission", cmd: PermissionCommand(apiClient, &buf, format), want: 3},
		{name: "priority", cmd: PriorityCommand(apiClient, &buf, format), want: 1},
		{name: "project", cmd: ProjectCommand(apiClient, &buf, format, testAllowWrites()), want: 5},
		{name: "role", cmd: RoleCommand(apiClient, &buf, format), want: 2},
		{name: "resolution", cmd: ResolutionCommand(apiClient, &buf, format), want: 1},
		{name: "status", cmd: StatusCommand(apiClient, &buf, format), want: 3},
		{name: "user", cmd: UserCommand(apiClient, &buf, format), want: 3},
		{name: "workflow", cmd: WorkflowCommand(apiClient, &buf, format), want: 5},
		{name: "component", cmd: ComponentCommand(apiClient, &buf, format, testAllowWrites()), want: 6},
		{name: "version", cmd: VersionCommand(apiClient, &buf, format, testAllowWrites()), want: 9},
		{name: "sprint", cmd: SprintCommand(apiClient, &buf, format, testAllowWrites()), want: 9},
		{name: "epic", cmd: EpicCommand(apiClient, &buf, format, testAllowWrites()), want: 6},
		{name: "backlog", cmd: BacklogCommand(apiClient, &buf, format, testAllowWrites()), want: 2},
		{name: "task", cmd: TaskCommand(apiClient, &buf, format, testAllowWrites()), want: 2},
		{name: "time-tracking", cmd: TimeTrackingCommand(apiClient, &buf, format, testAllowWrites()), want: 4},
		{name: "issue", cmd: IssueCommand(apiClient, &buf, format, testAllowWrites(), testDryRun()), want: 31},
		{name: "jql", cmd: JQLCommand(apiClient, &buf, format), want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := len(tt.cmd.Commands()); got != tt.want {
				t.Errorf("commands length = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDashboardAndFilterSharingCommandMetadata(t *testing.T) {
	t.Parallel()

	apiClient := &client.Ref{}
	format := testCommandFormat()
	var buf bytes.Buffer

	tests := []struct {
		name               string
		cmd                *cobra.Command
		wantExample        bool
		wantDefaultCommand string
	}{
		{name: "dashboard", cmd: DashboardCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true, wantDefaultCommand: "list"},
		{name: "dashboard create", cmd: dashboardCreateCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "dashboard update", cmd: dashboardUpdateCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "dashboard delete", cmd: dashboardDeleteCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "dashboard copy", cmd: dashboardCopyCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "filter", cmd: FilterCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true, wantDefaultCommand: "list"},
		{name: "filter permissions", cmd: filterPermissionsCommand(apiClient, &buf, format), wantExample: true},
		{name: "filter share", cmd: filterShareCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "filter unshare", cmd: filterUnshareCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
		{name: "filter default-share-scope", cmd: filterDefaultShareScopeCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true, wantDefaultCommand: "get"},
		{name: "filter default-share-scope get", cmd: filterDefaultShareScopeGetCommand(apiClient, &buf, format), wantExample: true},
		{name: "filter default-share-scope set", cmd: filterDefaultShareScopeSetCommand(apiClient, &buf, format, testAllowWrites()), wantExample: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantExample {
				if tt.cmd.Example == "" {
					t.Error("Example should not be empty")
				}
			}
			if tt.wantDefaultCommand != "" && tt.cmd.RunE == nil {
				t.Errorf("default command %q is not wired", tt.wantDefaultCommand)
			}
		})
	}
}

func testDenyWrites() *bool {
	v := false
	return &v
}

func TestWriteGuard_BlocksWriteCommands(t *testing.T) {
	t.Parallel()

	apiClient := &client.Ref{}
	format := testCommandFormat()

	tests := []struct {
		name string
		cmd  func(*bytes.Buffer) *cobra.Command
		args []string
	}{
		{
			name: "issue_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--project", "P", "--type", "Bug", "--summary", "s"},
		},
		{
			name: "issue_bulk_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueBulkCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues-json", `[{}]`},
		},
		{
			name: "issue_edit",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueEditCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--field", "summary=x", "TEST-1"},
		},
		{
			name: "issue_assign",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueAssignCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1", "user@example.com"},
		},
		{
			name: "comment_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return commentAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--body", "text", "TEST-1"},
		},
		{
			name: "comment_edit",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return commentEditCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--body", "text", "TEST-1", "10000"},
		},
		{
			name: "comment_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return commentDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1", "10000"},
		},
		{
			name: "worklog_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return worklogAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--time-spent", "1h", "TEST-1"},
		},
		{
			name: "worklog_edit",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return worklogEditCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--time-spent", "2h", "TEST-1", "10000"},
		},
		{
			name: "worklog_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return worklogDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1", "10000"},
		},
		{
			name: "issue_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1"},
		},
		{
			name: "issue_rank",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueRankCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "TEST-1,TEST-2", "--before", "TEST-3"},
		},
		{
			name: "issue_notify",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueNotifyCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--subject", "Test", "TEST-1"},
		},
		{
			name: "issue_link_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueLinkAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--type", "Blocks", "--inward", "TEST-1", "--outward", "TEST-2"},
		},
		{
			name: "issue_link_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueLinkDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "issue_remotelink_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return remoteLinkAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--url", "https://example.com", "--title", "Example", "TEST-1"},
		},
		{
			name: "issue_remotelink_edit",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return remoteLinkEditCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--url", "https://example.com", "--title", "Example", "TEST-1", "10000"},
		},
		{
			name: "issue_remotelink_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return remoteLinkDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1", "10000"},
		},
		{
			name: "issue_watcher_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return watcherAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--account-id", "abc123", "TEST-1"},
		},
		{
			name: "issue_watcher_remove",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return watcherRemoveCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--account-id", "abc123", "TEST-1"},
		},
		{
			name: "issue_vote_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return voteAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1"},
		},
		{
			name: "issue_vote_remove",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return voteRemoveCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"TEST-1"},
		},
		{
			name: "issue_attachment_add",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return attachmentAddCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--file", "does-not-matter.txt", "TEST-1"},
		},
		{
			name: "issue_attachment_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return attachmentDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "group_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return groupCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"jira-users"},
		},
		{
			name: "group_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return groupDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--group-id", "gid-1"},
		},
		{
			name: "group_add_member",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return groupAddMemberCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--group-id", "gid-1", "--account-id", "abc123"},
		},
		{
			name: "group_remove_member",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return groupRemoveMemberCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--group-id", "gid-1", "--account-id", "abc123"},
		},
		{
			name: "filter_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Mine", "--jql", "assignee = currentUser()"},
		},
		{
			name: "filter_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Mine", "10000"},
		},
		{
			name: "filter_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "filter_share",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterShareCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--with", "user:abc123", "10000"},
		},
		{
			name: "filter_unshare",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterUnshareCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--permission-id", "10001", "10000"},
		},
		{
			name: "filter_default_share_scope_set",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return filterDefaultShareScopeSetCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--scope", "PRIVATE"},
		},
		{
			name: "dashboard_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return dashboardCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Team"},
		},
		{
			name: "dashboard_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return dashboardUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Team", "10000"},
		},
		{
			name: "dashboard_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return dashboardDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "dashboard_copy",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return dashboardCopyCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Team copy", "10000"},
		},
		{
			name: "board_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return boardCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Team board", "--type", "scrum", "--filter", "10000"},
		},
		{
			name: "board_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return boardDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"84"},
		},
		{
			name: "task_cancel",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return taskCancelCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10641"},
		},
		{
			name: "time_tracking_select",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return timeTrackingSelectCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--key", "JIRA"},
		},
		{
			name: "time_tracking_options_set",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return timeTrackingOptionsSetCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{
				"--working-hours-per-day", "8",
				"--working-days-per-week", "5",
				"--time-format", "pretty",
				"--default-unit", "minute",
			},
		},
		{
			name: "project_roles_add_actor",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return projectRoleAddActorCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--user", "abc123", "PROJ", "10000"},
		},
		{
			name: "project_roles_remove_actor",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return projectRoleRemoveActorCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--user", "abc123", "PROJ", "10000"},
		},
		{
			name: "context_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return contextCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "ctx", "customfield_10000"},
		},
		{
			name: "context_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return contextUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "ctx2", "customfield_10000", "10001"},
		},
		{
			name: "context_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return contextDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"customfield_10000", "10001"},
		},
		{
			name: "option_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return optionCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--values", "opt1", "customfield_10000", "10001"},
		},
		{
			name: "option_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return optionUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--option-id", "10002", "--value", "opt2", "customfield_10000", "10001"},
		},
		{
			name: "option_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return optionDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"customfield_10000", "10001", "10002"},
		},
		{
			name: "option_reorder",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return optionReorderCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--option-ids", "1", "--position", "First", "customfield_10000", "10001"},
		},
		{
			name: "component_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return componentCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Backend", "--project", "PROJ"},
		},
		{
			name: "component_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return componentUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Backend v2", "10000"},
		},
		{
			name: "component_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return componentDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "version_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return versionCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "v1.0", "--project", "PROJ"},
		},
		{
			name: "version_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return versionUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "v1.1", "10000"},
		},
		{
			name: "version_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return versionDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000"},
		},
		{
			name: "version_merge",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return versionMergeCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"10000", "10001"},
		},
		{
			name: "version_move",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return versionMoveCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--position", "First", "10000"},
		},
		{
			name: "sprint_create",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return sprintCreateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Sprint 1", "--board-id", "1"},
		},
		{
			name: "sprint_update",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return sprintUpdateCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--name", "Sprint 1", "100"},
		},
		{
			name: "sprint_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return sprintDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"100"},
		},
		{
			name: "sprint_move_issues",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return sprintMoveIssuesCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "PROJ-1", "100"},
		},
		{
			name: "sprint_swap",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return sprintSwapCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"100", "101"},
		},
		{
			name: "issue_property_set",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertySetCommand(issuePropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"--value-json", `{"enabled":true}`, "PROJ-1", "com.example.flag"},
		},
		{
			name: "issue_property_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertyDeleteCommand(issuePropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"PROJ-1", "com.example.flag"},
		},
		{
			name: "sprint_property_set",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertySetCommand(sprintPropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"--value-json", `{"enabled":true}`, "100", "com.example.flag"},
		},
		{
			name: "sprint_property_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertyDeleteCommand(sprintPropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"100", "com.example.flag"},
		},
		{
			name: "board_property_set",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertySetCommand(boardPropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"--value-json", `{"enabled":true}`, "42", "com.example.flag"},
		},
		{
			name: "board_property_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return propertyDeleteCommand(boardPropertyTarget(apiClient), buf, format, testDenyWrites())
			},
			args: []string{"42", "com.example.flag"},
		},
		{
			name: "epic_move_issues",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return epicMoveIssuesCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "PROJ-1", "EPIC-1"},
		},
		{
			name: "epic_remove_issues",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return epicRemoveIssuesCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "PROJ-1"},
		},
		{
			name: "epic_rank",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return epicRankCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--rank-before", "EPIC-2", "EPIC-1"},
		},
		{
			name: "backlog_move",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return backlogMoveCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "PROJ-1"},
		},
		{
			name: "issue_bulk_delete",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueBulkDeleteCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--issues", "PROJ-1"},
		},
		{
			name: "issue_bulk_edit",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueBulkEditCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--payload-json", `{"selectedIssueIdsOrKeys":["PROJ-1"],"selectedActions":["summary"],"editedFieldsInput":{}}`},
		},
		{
			name: "issue_bulk_move",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueBulkMoveCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--payload-json", `{"targetToSourcesMapping":{}}`},
		},
		{
			name: "issue_bulk_transition",
			cmd: func(buf *bytes.Buffer) *cobra.Command {
				return issueBulkTransitionCommand(apiClient, buf, format, testDenyWrites())
			},
			args: []string{"--transitions-json", `[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := tt.cmd(&buf)

			prepareCommandForTest(cmd)
			cmd.SetContext(context.Background())
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected write-guard error, got nil")
			}

			var valErr *apperr.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("error type = %T, want *apperr.ValidationError", err)
			}
			if got := valErr.Error(); got != "write operations are not enabled" {
				t.Errorf("error message = %q, want %q", got, "write operations are not enabled")
			}
			wantDetails := `set "i-too-like-to-live-dangerously": true in config file or JIRA_ALLOW_WRITES=true env var to enable write operations`
			if got := valErr.Details(); got != wantDetails {
				t.Errorf("details = %q, want %q", got, wantDetails)
			}
		})
	}
}

func TestWriteGuard_NilPointerBlocksWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := issueCreateCommand(&client.Ref{}, &buf, testCommandFormat(), nil)

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--project", "P", "--type", "Bug", "--summary", "s"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected write-guard error for nil allowWrites, got nil")
	}

	var valErr *apperr.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error type = %T, want *apperr.ValidationError", err)
	}
}

func TestSchemaRequiredFlagsDoNotBypassRuntimeValidation(t *testing.T) {
	t.Parallel()

	format := testCommandFormat()

	t.Run("read command keeps typed validation error", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueSearchCommand(&client.Ref{}, &buf, format)

		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
		if got := valErr.Error(); got != "--jql is required" {
			t.Errorf("error message = %q, want %q", got, "--jql is required")
		}
	})

	t.Run("write guard runs before missing payload validation", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueBulkCreateCommand(&client.Ref{}, &buf, format, testDenyWrites())

		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected write-guard error, got nil")
		}

		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
		if got := valErr.Error(); got != "write operations are not enabled" {
			t.Errorf("error message = %q, want %q", got, "write operations are not enabled")
		}
	})
}

func TestWriteGuard_TransitionListBypassesGuard(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"1","name":"Done"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := issueTransitionCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testDenyWrites())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--list", "TEST-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("transition --list should bypass write guard: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"transitions"`)) {
		t.Errorf("output = %q, want transitions list", buf.String())
	}
}

func TestWriteGuard_TransitionWriteBlocked(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"1","name":"Done"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := issueTransitionCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testDenyWrites())

	prepareCommandForTest(cmd)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--to", "Done", "TEST-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected write-guard error for transition without --list, got nil")
	}

	var valErr *apperr.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error type = %T, want *apperr.ValidationError", err)
	}
}

func TestBoardCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board")
			}
			if got := r.URL.Query().Get("type"); got != "scrum" {
				t.Errorf("type = %q, want %q", got, "scrum")
			}
			if got := r.URL.Query().Get("name"); got != "Platform" {
				t.Errorf("name = %q, want %q", got, "Platform")
			}
			if got := r.URL.Query().Get("projectKeyOrId"); got != "PROJ" {
				t.Errorf("projectKeyOrId = %q, want %q", got, "PROJ")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"isLast":true,"maxResults":25,"startAt":0,"total":1,"values":[{"id":84,"name":"Platform","type":"scrum"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			boardListCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
			"--type", "scrum", "--name", "Platform", "--project", "PROJ", "--max-results", "25",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want total metadata", buf.String())
		}
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/rest/agile/1.0/board" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board")
			}

			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["name"]; got != "Team board" {
				t.Errorf("name = %v, want %q", got, "Team board")
			}
			if got := body["type"]; got != "scrum" {
				t.Errorf("type = %v, want %q", got, "scrum")
			}
			if got := body["filterId"]; got != float64(10000) {
				t.Errorf("filterId = %v, want %v", got, float64(10000))
			}

			location, ok := body["location"].(map[string]any)
			if !ok {
				t.Fatalf("location = %T, want object", body["location"])
			}
			if got := location["type"]; got != "project" {
				t.Errorf("location.type = %v, want %q", got, "project")
			}
			if got := location["projectKeyOrId"]; got != "PROJ" {
				t.Errorf("location.projectKeyOrId = %v, want %q", got, "PROJ")
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if _, err := w.Write([]byte(`{"id":84,"name":"Team board","type":"scrum"}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			boardCreateCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
			"--name", "Team board", "--type", "scrum", "--filter", "10000", "--location-project", "PROJ",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"id":84`)) {
			t.Errorf("output = %q, want created board payload", buf.String())
		}
	})

	t.Run("filter", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/filter/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/filter/10000")
			}
			if got := r.URL.Query().Get("maxResults"); got != "10" {
				t.Errorf("maxResults = %q, want %q", got, "10")
			}
			testhelpers.WriteJSONResponse(t, w, `{"maxResults":10,"startAt":0,"total":1,"values":[{"id":84,"name":"Team board"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardFilterCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "10000", "--max-results", "10")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardDeleteCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()), "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":84,"name":"Platform","type":"scrum"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardGetCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Platform"`)) {
			t.Errorf("output = %q, want board payload", buf.String())
		}
	})

	t.Run("get rejects non numeric ID", func(t *testing.T) {
		t.Parallel()

		_, err := parseBoardID("84/../../sprint")
		if err == nil {
			t.Fatal("error = nil, want validation error")
		}
		if !errors.As(err, new(*apperr.ValidationError)) {
			t.Fatalf("error type = %T, want *errors.ValidationError", err)
		}
	})

	t.Run("config", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84/configuration" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/configuration")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":84,"name":"Platform","columnConfig":{"columns":[{"name":"To Do"}]}}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardConfigCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"columnConfig"`)) {
			t.Errorf("output = %q, want config payload", buf.String())
		}
	})

	t.Run("epics", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84/epic" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/epic")
			}
			if got := r.URL.Query().Get("done"); got != "true" {
				t.Errorf("done = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":1,"values":[{"id":1,"name":"Epic 1"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardEpicsCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "--done", "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})

	t.Run("issues", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84/issue" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/issue")
			}
			if got := r.URL.Query().Get("jql"); got != "status=Done" {
				t.Errorf("jql = %q, want %q", got, "status=Done")
			}
			if got := r.URL.Query().Get("fields"); got != "summary,status" {
				t.Errorf("fields = %q, want %q", got, "summary,status")
			}
			testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":2,"issues":[{"key":"TEST-1"},{"key":"TEST-2"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
			"--jql", "status=Done", "--fields", "summary,status", "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":2`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})

	t.Run("projects", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84/project" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/project")
			}
			testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":1,"values":[{"key":"PROJ","name":"Project"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardProjectsCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/rest/agile/1.0/board/84/version" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/version")
			}
			testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":1,"values":[{"id":100,"name":"v1.0"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, boardVersionsCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "84")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})
}

func TestFieldListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/field" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/field")
		}
		testhelpers.WriteJSONResponse(t, w, `[{"id":"summary"},{"id":"customfield_10000"}]`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, fieldListCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
	if !bytes.Contains(buf.Bytes(), []byte(`"returned":2`)) {
		t.Errorf("output = %q, want returned metadata", buf.String())
	}
}

func TestFieldSearchCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/field/search" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/field/search")
		}
		if got := r.URL.Query().Get("query"); got != "Story Points" {
			t.Errorf("query = %q, want %q", got, "Story Points")
		}
		if got := r.URL.Query().Get("type"); got != "custom" {
			t.Errorf("type = %q, want %q", got, "custom")
		}
		if got := r.URL.Query().Get("maxResults"); got != "2" {
			t.Errorf("maxResults = %q, want %q", got, "2")
		}
		testhelpers.WriteJSONResponse(t, w, `{"total":1,"startAt":0,"maxResults":2,"values":[{"id":"customfield_10000"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		fieldSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--query", "Story Points", "--type", "custom", "--max-results", "2",
	)
}

func TestReferenceListCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		cmd      func(*client.Ref, io.Writer, *output.Format) *cobra.Command
		response string
	}{
		{
			name:     "status",
			path:     "/status",
			cmd:      statusListCommand,
			response: `[{"id":"1","name":"To Do"},{"id":"2","name":"Done"}]`,
		},
		{
			name:     "priority",
			path:     "/priority",
			cmd:      priorityListCommand,
			response: `[{"id":"1","name":"Highest"},{"id":"2","name":"Low"}]`,
		},
		{
			name:     "resolution",
			path:     "/resolution",
			cmd:      resolutionListCommand,
			response: `[{"id":"1","name":"Done"},{"id":"2","name":"Duplicate"}]`,
		},
		{
			name:     "issuetype",
			path:     "/issuetype",
			cmd:      issueTypeListCommand,
			response: `[{"id":"10001","name":"Story"},{"id":"10002","name":"Bug"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := testhelpers.NewJSONServer(t, http.MethodGet, tt.path, tt.response)

			var buf bytes.Buffer
			runCommandAction(t, tt.cmd(testCommandClient(server.URL), &buf, testCommandFormat()))
			if !bytes.Contains(buf.Bytes(), []byte(`"returned":2`)) {
				t.Errorf("output = %q, want returned metadata", buf.String())
			}
		})
	}
}

func TestUserCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/user" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/user")
			}
			if got := r.URL.Query().Get("accountId"); got != "abc123" {
				t.Errorf("accountId = %q, want %q", got, "abc123")
			}
			if got := r.URL.Query().Get("expand"); got != "groups" {
				t.Errorf("expand = %q, want %q", got, "groups")
			}
			testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Ada"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, userGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--account-id", "abc123", "--expand", "groups")
		if !bytes.Contains(buf.Bytes(), []byte(`"displayName":"Ada"`)) {
			t.Errorf("output = %q, want user payload", buf.String())
		}
	})

	t.Run("search", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/users/search" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/users/search")
			}
			if got := r.URL.Query().Get("query"); got != "ada" {
				t.Errorf("query = %q, want %q", got, "ada")
			}
			if got := r.URL.Query().Get("startAt"); got != "5" {
				t.Errorf("startAt = %q, want %q", got, "5")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"accountId":"abc123"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, userSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--query", "ada", "--start-at", "5")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("groups", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/user/groups" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/user/groups")
			}
			if got := r.URL.Query().Get("accountId"); got != "abc123" {
				t.Errorf("accountId = %q, want %q", got, "abc123")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"groupId":"gid-1","name":"jira-users"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, userGroupsCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--account-id", "abc123")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})
}

func TestGroupCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/groups/picker" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/groups/picker")
			}
			if got := r.URL.Query().Get("query"); got != "jira" {
				t.Errorf("query = %q, want %q", got, "jira")
			}
			if got := r.URL.Query().Get("caseInsensitive"); got != "true" {
				t.Errorf("caseInsensitive = %q, want %q", got, "true")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"groups":[{"groupId":"gid-1","name":"jira-users"}],"total":1}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			groupListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--query",
			"jira",
			"--case-insensitive",
			"--max-results",
			"25",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want group picker payload", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/group" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/group")
			}
			if got := r.URL.Query().Get("groupId"); got != "gid-1" {
				t.Errorf("groupId = %q, want %q", got, "gid-1")
			}
			testhelpers.WriteJSONResponse(t, w, `{"groupId":"gid-1","name":"jira-users"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, groupGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--group-id", "gid-1")
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["name"]; got != "jira-users" {
				t.Errorf("name = %v, want %q", got, "jira-users")
			}
			testhelpers.WriteJSONResponse(t, w, `{"name":"jira-users"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, groupCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "jira-users")
	})

	t.Run("members", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/group/member" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/group/member")
			}
			if got := r.URL.Query().Get("includeInactiveUsers"); got != "true" {
				t.Errorf("includeInactiveUsers = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":50,"total":1,"values":[{"accountId":"abc123"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, groupMembersCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--groupname", "jira-users", "--include-inactive")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("add member", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/group/user" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/group/user")
			}
			if got := r.URL.Query().Get("groupId"); got != "gid-1" {
				t.Errorf("groupId = %q, want %q", got, "gid-1")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["accountId"]; got != "abc123" {
				t.Errorf("accountId = %v, want %q", got, "abc123")
			}
			testhelpers.WriteJSONResponse(t, w, `{"groupId":"gid-1"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, groupAddMemberCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--group-id", "gid-1", "--account-id", "abc123")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/group" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/group")
			}
			if got := r.URL.Query().Get("swapGroupId"); got != "swap-1" {
				t.Errorf("swapGroupId = %q, want %q", got, "swap-1")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, groupDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--group-id", "gid-1", "--swap-group-id", "swap-1")
	})
}

func TestFilterCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/filter/search" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/search")
			}
			if got := r.URL.Query().Get("filterName"); got != "mine" {
				t.Errorf("filterName = %q, want %q", got, "mine")
			}
			if got := r.URL.Query().Get("isSubstringMatch"); got != "true" {
				t.Errorf("isSubstringMatch = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":50,"total":1,"values":[{"id":"10000","name":"Mine"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--name", "mine", "--substring")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/filter/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/10000")
			}
			if got := r.URL.Query().Get("expand"); got != "jql,owner" {
				t.Errorf("expand = %q, want %q", got, "jql,owner")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"Mine"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--expand", "jql,owner", "10000")
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["jql"]; got != "assignee = currentUser()" {
				t.Errorf("jql = %v, want current user JQL", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"Mine"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--name", "Mine", "--jql", "assignee = currentUser()")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/filter/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/10000")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["description"]; got != "Updated" {
				t.Errorf("description = %v, want %q", got, "Updated")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","description":"Updated"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--description", "Updated", "10000")
	})

	t.Run("permissions", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/filter/10000/permission",
			`[{"id":10001,"type":"user"}]`,
		)

		var buf bytes.Buffer
		runCommandAction(t, filterPermissionsCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("share", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/filter/10000/permission" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/10000/permission")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["type"]; got != "user" {
				t.Errorf("type = %v, want user", got)
			}
			if got := body["accountId"]; got != "abc123" {
				t.Errorf("accountId = %v, want abc123", got)
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":10001,"type":"user"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterShareCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--with", "user:abc123", "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("share escapes filter ID path segment", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI != "/filter/10000%2Fcustom%3Fexpand=all/permission" {
				t.Errorf("request URI = %q, want escaped filter ID", r.RequestURI)
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":10001,"type":"user"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterShareCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--with", "user:abc123", "10000/custom?expand=all")
	})

	t.Run("unshare", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/filter/10000/permission/10001" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/10000/permission/10001")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterUnshareCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--permission-id", "10001", "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})

	t.Run("default share scope get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/filter/defaultShareScope",
			`{"scope":"PRIVATE"}`,
		)

		var buf bytes.Buffer
		runCommandAction(t, filterDefaultShareScopeGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"scope":"PRIVATE"`)) {
			t.Errorf("output = %q, want scope payload", buf.String())
		}
	})

	t.Run("default share scope set", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/filter/defaultShareScope" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/filter/defaultShareScope")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["scope"]; got != "PRIVATE" {
				t.Errorf("scope = %v, want PRIVATE", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"scope":"PRIVATE"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterDefaultShareScopeSetCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--scope", "PRIVATE")
	})

	t.Run("favorites", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			favoritePath := "/filter/favou" + "rite"
			if r.URL.Path != favoritePath {
				t.Errorf("path = %q, want %q", r.URL.Path, favoritePath)
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"10000","name":"Mine"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, filterFavoritesCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})
}

func TestDashboardCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/dashboard" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard")
			}
			if got := r.URL.Query().Get("filter"); got != "my" {
				t.Errorf("filter = %q, want %q", got, "my")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":25,"total":1,"dashboards":[{"id":"10000","name":"System Dashboard"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--filter", "my", "--max-results", "25")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want total metadata", buf.String())
		}
	})

	t.Run("list_search", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/dashboard/search" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard/search")
			}
			if got := r.URL.Query().Get("searchTerm"); got != "ops" {
				t.Errorf("searchTerm = %q, want %q", got, "ops")
			}
			if got := r.URL.Query().Get("maxResults"); got != "10" {
				t.Errorf("maxResults = %q, want %q", got, "10")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":10,"total":2,"values":[{"id":"100","name":"Ops Board"},{"id":"101","name":"Ops Alerts"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--search", "ops", "--max-results", "10")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":2`)) {
			t.Errorf("output = %q, want total metadata", buf.String())
		}
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":2`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("list_search_and_filter_conflict", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := dashboardListCommand(testCommandClient("http://example.invalid"), &buf, testCommandFormat())
		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs([]string{"--search", "ops", "--filter", "my"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() error = nil, want validation error")
		}

		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("errors.As(ValidationError) = false for %T", err)
		}
		if validationErr.Message != "--search and --filter cannot be used together; choose one" {
			t.Errorf("ValidationError.Message = %q, want conflict message", validationErr.Message)
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/dashboard/10000",
			`{"id":"10000","name":"System Dashboard"}`,
		)

		var buf bytes.Buffer
		runCommandAction(t, dashboardGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"System Dashboard"`)) {
			t.Errorf("output = %q, want dashboard payload", buf.String())
		}
	})

	t.Run("gadgets", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/dashboard/10000/gadget" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard/10000/gadget")
			}
			if got := r.URL.Query().Get("moduleKey"); got != "activity-stream" {
				t.Errorf("moduleKey = %q, want %q", got, "activity-stream")
			}
			testhelpers.WriteJSONResponse(t, w, `{"gadgets":[{"id":10001,"title":"Activity"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardGadgetsCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--module-key", "activity-stream", "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/dashboard" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard")
			}
			if got := r.URL.Query().Get("extendAdminPermissions"); got != "true" {
				t.Errorf("extendAdminPermissions = %q, want true", got)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["name"]; got != "Team" {
				t.Errorf("name = %v, want Team", got)
			}
			if _, ok := body["sharePermissions"].([]any); !ok {
				t.Errorf("sharePermissions = %#v, want array", body["sharePermissions"])
			}
			if _, ok := body["editPermissions"].([]any); !ok {
				t.Errorf("editPermissions = %#v, want array", body["editPermissions"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"Team"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--name", "Team", "--extend-admin-permissions")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/dashboard/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard/10000")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["description"]; got != "Updated" {
				t.Errorf("description = %v, want Updated", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","description":"Updated"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--name", "Team", "--description", "Updated", "10000")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/dashboard/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})

	t.Run("copy", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/dashboard/10000/copy" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/dashboard/10000/copy")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["name"]; got != "Team copy" {
				t.Errorf("name = %v, want Team copy", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10001","name":"Team copy"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardCopyCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--name", "Team copy", "10000")
	})

	t.Run("copy escapes dashboard ID path segment", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI != "/dashboard/10000%2Fcustom%3Fexpand=all/copy" {
				t.Errorf("request URI = %q, want escaped dashboard ID", r.RequestURI)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10001","name":"Team copy"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, dashboardCopyCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--name", "Team copy", "10000/custom?expand=all")
	})
}

func TestPermissionCommands(t *testing.T) {
	t.Parallel()

	t.Run("check", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/mypermissions" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/mypermissions")
			}
			if got := r.URL.Query().Get("permissions"); got != "BROWSE_PROJECTS,EDIT_ISSUES" {
				t.Errorf("permissions = %q, want permission list", got)
			}
			if got := r.URL.Query().Get("projectKey"); got != "TEST" {
				t.Errorf("projectKey = %q, want %q", got, "TEST")
			}
			testhelpers.WriteJSONResponse(t, w, `{"permissions":{"BROWSE_PROJECTS":{"havePermission":true}}}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, permissionCheckCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--permissions", "BROWSE_PROJECTS,EDIT_ISSUES", "--project-key", "TEST")
		if !bytes.Contains(buf.Bytes(), []byte(`"havePermission":true`)) {
			t.Errorf("output = %q, want permission payload", buf.String())
		}
	})

	t.Run("schemes list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/permissionscheme" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/permissionscheme")
			}
			if got := r.URL.Query().Get("expand"); got != "permissions" {
				t.Errorf("expand = %q, want %q", got, "permissions")
			}
			testhelpers.WriteJSONResponse(t, w, `{"permissionSchemes":[{"id":10000,"name":"Default"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, permissionSchemeListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--expand", "permissions")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("schemes project", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/project/TEST/permissionscheme",
			`{"id":10000,"name":"Default"}`,
		)

		var buf bytes.Buffer
		runCommandAction(t, permissionSchemeProjectCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST")
	})
}

func TestWorkflowCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflows/search" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflows/search")
			}
			if got := r.URL.Query().Get("queryString"); got != "Default" {
				t.Errorf("queryString = %q, want %q", got, "Default")
			}
			if got := r.URL.Query().Get("isActive"); got != "true" {
				t.Errorf("isActive = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":50,"total":1,"values":[{"id":"wf-1","name":"Default"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--query", "Default", "--is-active")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want total metadata", buf.String())
		}
	})

	t.Run("get by id", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflows" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflows")
			}
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			workflowIDs, ok := body["workflowIds"].([]any)
			if !ok || len(workflowIDs) != 1 || workflowIDs[0] != "wf-1" {
				t.Errorf("workflowIds = %#v, want [wf-1]", body["workflowIds"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"workflows":[{"id":"wf-1","name":"Default"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "wf-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Default"`)) {
			t.Errorf("output = %q, want workflow payload", buf.String())
		}
	})

	t.Run("statuses", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/status" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/status")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"1","name":"Open"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowStatusesCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("scheme list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflowscheme" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflowscheme")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"startAt":0,"maxResults":25,"total":1,"values":[{"id":10000,"name":"Default"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowSchemeListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--max-results", "25")
		if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
			t.Errorf("output = %q, want total metadata", buf.String())
		}
	})

	t.Run("scheme get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflowscheme/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflowscheme/10000")
			}
			if got := r.URL.Query().Get("returnDraftIfExists"); got != "true" {
				t.Errorf("returnDraftIfExists = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000,"name":"Default"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowSchemeGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--return-draft", "10000")
	})

	t.Run("scheme project", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflowscheme/project" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflowscheme/project")
			}
			if got := r.URL.Query().Get("projectId"); got != "10001" {
				t.Errorf("projectId = %q, want %q", got, "10001")
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[{"projectIds":["10001"],"workflowScheme":{"id":10000}}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowSchemeProjectCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10001")
	})

	t.Run("transition rules", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/workflows/capabilities" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/workflows/capabilities")
			}
			if got := r.URL.Query().Get("workflowId"); got != "wf-1" {
				t.Errorf("workflowId = %q, want %q", got, "wf-1")
			}
			if got := r.URL.Query().Get("projectId"); got != "10001" {
				t.Errorf("projectId = %q, want %q", got, "10001")
			}
			testhelpers.WriteJSONResponse(t, w, `{"connectRules":[],"forgeRules":[]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, workflowTransitionRulesCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--workflow-id", "wf-1", "--project-id", "10001")
	})
}

func TestStatusCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/status/10000",
			`{"id":"10000","name":"In Progress"}`,
		)

		var buf bytes.Buffer
		runCommandAction(t, statusGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"In Progress"`)) {
			t.Errorf("output = %q, want status payload", buf.String())
		}
	})

	t.Run("categories", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/statuses/categories",
			`[{"id":2,"key":"new"},{"id":4,"key":"indeterminate"}]`,
		)

		var buf bytes.Buffer
		runCommandAction(t, statusCategoriesCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":2`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})
}

func TestIssueTypeCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(
			t,
			http.MethodGet,
			"/issuetype/10001",
			`{"id":"10001","name":"Story"}`,
		)

		var buf bytes.Buffer
		runCommandAction(t, issueTypeGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10001")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Story"`)) {
			t.Errorf("output = %q, want issue type payload", buf.String())
		}
	})

	t.Run("project", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issuetype/project" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issuetype/project")
			}
			if got := r.URL.Query().Get("projectId"); got != "10000" {
				t.Errorf("projectId = %q, want %q", got, "10000")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"10001","name":"Story"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueTypeProjectCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})
}

func TestServerInfoCommand(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, http.MethodGet, "/serverInfo",
		`{"baseUrl":"https://jira.example.com","version":"9.0.0","serverTitle":"My Jira"}`)

	var buf bytes.Buffer
	runCommandAction(t, ServerInfoCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
	if !bytes.Contains(buf.Bytes(), []byte(`"serverTitle":"My Jira"`)) {
		t.Errorf("output = %q, want serverTitle", buf.String())
	}
}

func TestTaskCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodGet, "/task/10641",
			`{"id":"10641","status":"COMPLETE","progress":100}`)

		var buf bytes.Buffer
		runCommandAction(t, taskGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10641")
		if !bytes.Contains(buf.Bytes(), []byte(`"status":"COMPLETE"`)) {
			t.Errorf("output = %q, want task status", buf.String())
		}
	})

	t.Run("cancel", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodPost, "/task/10641/cancel",
			`{"id":"10641","status":"CANCELED"}`)

		var buf bytes.Buffer
		runCommandAction(t, taskCancelCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "10641")
		if !bytes.Contains(buf.Bytes(), []byte(`"status":"CANCELED"`)) {
			t.Errorf("output = %q, want canceled task status", buf.String())
		}
	})
}

func TestAuditCommands(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/auditing/record" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/auditing/record")
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Errorf("limit = %q, want %q", got, "25")
		}
		if got := r.URL.Query().Get("offset"); got != "50" {
			t.Errorf("offset = %q, want %q", got, "50")
		}
		if got := r.URL.Query().Get("filter"); got != "user created" {
			t.Errorf("filter = %q, want %q", got, "user created")
		}
		if got := r.URL.Query().Get("from"); got != "2024-01-01T00:00:00.000+0000" {
			t.Errorf("from = %q, want %q", got, "2024-01-01T00:00:00.000+0000")
		}
		if got := r.URL.Query().Get("to"); got != "2024-12-31T23:59:59.000+0000" {
			t.Errorf("to = %q, want %q", got, "2024-12-31T23:59:59.000+0000")
		}
		testhelpers.WriteJSONResponse(t, w, `{
			"offset":50,
			"limit":25,
			"total":2,
			"records":[{
				"id":10000,
				"summary":"User created",
				"authorAccountId":"5b10ac8d82e05b22cc7d4ef5"
			}]
		}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		auditListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--limit", "25",
		"--offset", "50",
		"--filter", "user created",
		"--from", "2024-01-01T00:00:00.000+0000",
		"--to", "2024-12-31T23:59:59.000+0000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"summary":"User created"`)) {
		t.Errorf("output = %q, want audit record summary", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
		t.Errorf("output = %q, want returned metadata", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"start_at":50`)) {
		t.Errorf("output = %q, want offset metadata", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"max_results":25`)) {
		t.Errorf("output = %q, want limit metadata", buf.String())
	}
}

func TestTimeTrackingCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodGet, "/configuration/timetracking",
			`{"key":"JIRA","name":"JIRA provided time tracking"}`)

		var buf bytes.Buffer
		runCommandAction(t, timeTrackingGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"JIRA"`)) {
			t.Errorf("output = %q, want selected provider", buf.String())
		}
	})

	t.Run("providers", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodGet, "/configuration/timetracking/list",
			`[{"key":"JIRA","name":"JIRA provided time tracking"}]`)

		var buf bytes.Buffer
		runCommandAction(t, timeTrackingProvidersCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"JIRA provided time tracking"`)) {
			t.Errorf("output = %q, want provider name", buf.String())
		}
	})

	t.Run("select", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/configuration/timetracking" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/configuration/timetracking")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["key"]; got != "JIRA" {
				t.Errorf("key = %v, want %q", got, "JIRA")
			}
			testhelpers.WriteJSONResponse(t, w, `{"key":"JIRA"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			timeTrackingSelectCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--key", "JIRA",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"JIRA"`)) {
			t.Errorf("output = %q, want selected provider key", buf.String())
		}
	})

	t.Run("options get", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodGet, "/configuration/timetracking/options",
			`{"workingHoursPerDay":8,"workingDaysPerWeek":5,"timeFormat":"pretty","defaultUnit":"minute"}`)

		var buf bytes.Buffer
		runCommandAction(t, timeTrackingOptionsGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"timeFormat":"pretty"`)) {
			t.Errorf("output = %q, want time tracking options", buf.String())
		}
	})

	t.Run("options set", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/configuration/timetracking/options" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/configuration/timetracking/options")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["workingHoursPerDay"]; got != float64(7) {
				t.Errorf("workingHoursPerDay = %v, want %v", got, float64(7))
			}
			if got := body["workingDaysPerWeek"]; got != float64(5) {
				t.Errorf("workingDaysPerWeek = %v, want %v", got, float64(5))
			}
			if got := body["timeFormat"]; got != "hours" {
				t.Errorf("timeFormat = %v, want %q", got, "hours")
			}
			if got := body["defaultUnit"]; got != "hour" {
				t.Errorf("defaultUnit = %v, want %q", got, "hour")
			}
			testhelpers.WriteJSONResponse(t, w, `{"workingHoursPerDay":7,"workingDaysPerWeek":5,"timeFormat":"hours","defaultUnit":"hour"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			timeTrackingOptionsSetCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--working-hours-per-day", "7",
			"--working-days-per-week", "5",
			"--time-format", "hours",
			"--default-unit", "hour",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"defaultUnit":"hour"`)) {
			t.Errorf("output = %q, want updated options", buf.String())
		}
	})
}

func TestLabelListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/label" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/label")
		}
		if got := r.URL.Query().Get("maxResults"); got != "2" {
			t.Errorf("maxResults = %q, want %q", got, "2")
		}
		if got := r.URL.Query().Get("startAt"); got != "4" {
			t.Errorf("startAt = %q, want %q", got, "4")
		}
		testhelpers.WriteJSONResponse(t, w, `{"total":3,"startAt":4,"maxResults":2,"values":["backend","frontend"]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, labelListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--max-results", "2", "--start-at", "4")
	if !bytes.Contains(buf.Bytes(), []byte(`"total":3`)) {
		t.Errorf("output = %q, want total metadata", buf.String())
	}
}

func TestProjectCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/project/search" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/search")
			}
			if got := r.URL.Query().Get("typeKey"); got != "software" {
				t.Errorf("typeKey = %q, want %q", got, "software")
			}
			if got := r.URL.Query().Get("orderBy"); got != "key" {
				t.Errorf("orderBy = %q, want %q", got, "key")
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"values":[{"key":"TEST"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			projectListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--query", "TEST", "--type-key", "software", "--order-by", "key", "--expand", "lead",
		)
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/project/TEST" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/TEST")
			}
			if got := r.URL.Query().Get("expand"); got != "lead" {
				t.Errorf("expand = %q, want %q", got, "lead")
			}
			testhelpers.WriteJSONResponse(t, w, `{"key":"TEST"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, projectGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST", "--expand", "lead")
	})

	t.Run("roles", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/project/TEST/role" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/TEST/role")
			}
			testhelpers.WriteJSONResponse(t, w, `{"Developers":"https://example.atlassian.net/rest/api/3/project/TEST/role/10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, projectRolesCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "list", "TEST")
		if !bytes.Contains(buf.Bytes(), []byte(`"Developers"`)) {
			t.Errorf("output = %q, want role map", buf.String())
		}
	})

	t.Run("role", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/project/TEST/role/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/TEST/role/10000")
			}
			if got := r.URL.Query().Get("excludeInactiveUsers"); got != "true" {
				t.Errorf("excludeInactiveUsers = %q, want %q", got, "true")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000,"name":"Developers","actors":[{"displayName":"Jane"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, projectRoleCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST", "10000", "--exclude-inactive-users")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Developers"`)) {
			t.Errorf("output = %q, want role payload", buf.String())
		}
	})

	t.Run("add role actor", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/project/TEST/role/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/TEST/role/10000")
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading request body: %v", err)
			}
			for _, want := range [][]byte{
				[]byte(`"user":["abc123"]`),
				[]byte(`"group":["jira-users"]`),
				[]byte(`"groupId":["gid-1"]`),
			} {
				if !bytes.Contains(body, want) {
					t.Errorf("body = %s, want %s", body, want)
				}
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000,"name":"Developers","actors":[{"accountId":"abc123"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			projectRoleAddActorCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"TEST", "10000", "--user", "abc123", "--group", "jira-users", "--group-id", "gid-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"actors"`)) {
			t.Errorf("output = %q, want updated project role", buf.String())
		}
	})

	t.Run("remove role actor", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/project/TEST/role/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/project/TEST/role/10000")
			}
			if got := r.URL.Query().Get("groupId"); got != "gid-1" {
				t.Errorf("groupId = %q, want %q", got, "gid-1")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			projectRoleRemoveActorCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"TEST", "10000", "--group-id", "gid-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"removed":true`)) {
			t.Errorf("output = %q, want removal confirmation", buf.String())
		}
	})

	t.Run("categories", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/projectCategory" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/projectCategory")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"10000","name":"Internal"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, projectCategoriesCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("category", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/projectCategory/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/projectCategory/10000")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"Internal"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, projectCategoryCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Internal"`)) {
			t.Errorf("output = %q, want category payload", buf.String())
		}
	})
}

func TestRoleCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/role" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/role")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":10000,"name":"Developers"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, roleListCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/role/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/role/10000")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000,"name":"Developers"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, roleGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"name":"Developers"`)) {
			t.Errorf("output = %q, want role payload", buf.String())
		}
	})
}

func TestCommentCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/comment" {
				t.Errorf("path = %q, want comment list path", r.URL.Path)
			}
			if got := r.URL.Query().Get("startAt"); got != "5" {
				t.Errorf("startAt = %q, want %q", got, "5")
			}
			if got := r.URL.Query().Get("orderBy"); got != "-created" {
				t.Errorf("orderBy = %q, want %q", got, "-created")
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"comments":[{"id":"10000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			commentListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--start-at", "5", "--max-results", "10", "--order-by", "-created", "--expand", "renderedBody", "TEST-1",
		)
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/comment/10000" {
				t.Errorf("path = %q, want comment get path", r.URL.Path)
			}
			if got := r.URL.Query().Get("expand"); got != "renderedBody" {
				t.Errorf("expand = %q, want renderedBody", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			commentGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--expand", "renderedBody", "TEST-1", "10000",
		)
	})

	t.Run("list by ids", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/comment/list" {
				t.Errorf("path = %q, want /comment/list", r.URL.Path)
			}
			if got := r.URL.Query().Get("expand"); got != "renderedBody" {
				t.Errorf("expand = %q, want renderedBody", got)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			ids := body["ids"].([]any)
			if len(ids) != 2 || ids[0] != float64(10000) || ids[1] != float64(10001) {
				t.Errorf("ids = %v, want [10000 10001]", ids)
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10000"}],"total":1}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			commentListByIDsCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--ids", "10000,10001", "--expand", "renderedBody",
		)
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.String() != "/issue/TEST-1/comment?expand=renderedBody" {
				t.Errorf("url = %q, want %q", r.URL.String(), "/issue/TEST-1/comment?expand=renderedBody")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			visibility := body["visibility"].(map[string]any)
			if visibility["type"] != "role" {
				t.Errorf("visibility[type] = %v, want %v", visibility["type"], "role")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			commentAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--body", "hello", "--visibility-type", "role", "--visibility-value", "Developers", "--expand", "renderedBody", "TEST-1",
		)
	})

	t.Run("edit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1/comment/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/comment/10000")
			}
			if got := r.URL.Query().Get("notifyUsers"); got != "false" {
				t.Errorf("notifyUsers = %q, want %q", got, "false")
			}
			if got := r.URL.Query().Get("expand"); got != "renderedBody" {
				t.Errorf("expand = %q, want %q", got, "renderedBody")
			}
			if got := r.URL.Query().Get("overrideEditableFlag"); got != "true" {
				t.Errorf("overrideEditableFlag = %q, want true", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			commentEditCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--body", "updated", "--notify=false", "--override-editable-flag", "--expand", "renderedBody", "TEST-1", "10000",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/comment/10000" {
				t.Errorf("path = %q, want delete path", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, commentDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1", "10000")
	})
}

func TestIssueLinkCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			if got := r.URL.Query().Get("fields"); got != "issuelinks" {
				t.Errorf("fields = %q, want %q", got, "issuelinks")
			}
			testhelpers.WriteJSONResponse(t, w, `{"fields":{"issuelinks":[{"id":"10000","type":{"name":"Blocks"}}]}}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueLinkListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issueLink/10000" {
				t.Errorf("path = %q, want /issueLink/10000", r.URL.Path)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueLinkGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
	})

	t.Run("types", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issueLinkType" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issueLinkType")
			}
			testhelpers.WriteJSONResponse(t, w, `{"issueLinkTypes":[{"id":"10000","name":"Blocks"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueLinkTypesCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issueLink" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issueLink")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			linkType, ok := body["type"].(map[string]any)
			if !ok {
				t.Fatalf("type = %T, want object", body["type"])
			}
			if got := linkType["name"]; got != "Blocks" {
				t.Errorf("type.name = %v, want %q", got, "Blocks")
			}
			inward, ok := body["inwardIssue"].(map[string]any)
			if !ok {
				t.Fatalf("inwardIssue = %T, want object", body["inwardIssue"])
			}
			if got := inward["key"]; got != "TEST-1" {
				t.Errorf("inwardIssue.key = %v, want %q", got, "TEST-1")
			}
			outward, ok := body["outwardIssue"].(map[string]any)
			if !ok {
				t.Fatalf("outwardIssue = %T, want object", body["outwardIssue"])
			}
			if got := outward["key"]; got != "TEST-2" {
				t.Errorf("outwardIssue.key = %v, want %q", got, "TEST-2")
			}
			comment := body["comment"].(map[string]any)
			assertADFText(t, comment["body"], "linked for duplicate tracking")
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueLinkAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--type", "Blocks", "--inward", "TEST-1", "--outward", "TEST-2", "--comment", "linked for duplicate tracking",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issueLink/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issueLink/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueLinkDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})
}

func TestRemoteLinkCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/remotelink")
			}
			if got := r.URL.Query().Get("globalId"); got != "system=https://example.com&id=1" {
				t.Errorf("globalId = %q, want global ID", got)
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":10000,"object":{"title":"Example"}}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			remoteLinkListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--global-id", "system=https://example.com&id=1", "TEST-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink/10000" {
				t.Errorf("path = %q, want remote link get path", r.URL.Path)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, remoteLinkGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST-1", "10000")
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/remotelink")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["globalId"]; got != "system=https://example.com&id=1" {
				t.Errorf("globalId = %v, want %q", got, "system=https://example.com&id=1")
			}
			object, ok := body["object"].(map[string]any)
			if !ok {
				t.Fatalf("object = %T, want object", body["object"])
			}
			if got := object["url"]; got != "https://example.com" {
				t.Errorf("object.url = %v, want %q", got, "https://example.com")
			}
			if got := object["title"]; got != "Example" {
				t.Errorf("object.title = %v, want %q", got, "Example")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			remoteLinkAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--url", "https://example.com", "--title", "Example", "--global-id", "system=https://example.com&id=1", "TEST-1",
		)
	})

	t.Run("edit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/remotelink/10000")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got := body["relationship"]; got != "mentioned in" {
				t.Errorf("relationship = %v, want %q", got, "mentioned in")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":10000}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			remoteLinkEditCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--url", "https://example.com", "--title", "Example", "--relationship", "mentioned in", "TEST-1", "10000",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/remotelink/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, remoteLinkDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1", "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})

	t.Run("delete by global id", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/remotelink" {
				t.Errorf("path = %q, want remote link collection", r.URL.Path)
			}
			if got := r.URL.Query().Get("globalId"); got != "system=https://example.com&id=1" {
				t.Errorf("globalId = %q, want global ID", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			remoteLinkDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--global-id", "system=https://example.com&id=1", "TEST-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})
}

func TestWatcherCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/watchers" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/watchers")
			}
			testhelpers.WriteJSONResponse(t, w, `{"watchCount":1,"watchers":[{"accountId":"abc123"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, watcherListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST-1")
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/watchers" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/watchers")
			}
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if got := string(bodyBytes); got != `"abc123"` {
				t.Errorf("body = %q, want %q", got, `"abc123"`)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, watcherAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--account-id", "abc123", "TEST-1")
	})

	t.Run("remove", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/watchers" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/watchers")
			}
			if got := r.URL.Query().Get("accountId"); got != "abc123" {
				t.Errorf("accountId = %q, want %q", got, "abc123")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, watcherRemoveCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--account-id", "abc123", "TEST-1")
	})
}

func TestVoteCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/votes" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/votes")
			}
			testhelpers.WriteJSONResponse(t, w, `{"votes":1,"hasVoted":false}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, voteGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST-1")
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/votes" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/votes")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, voteAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1")
	})

	t.Run("remove", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/votes" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/votes")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, voteRemoveCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1")
	})
}

func TestAttachmentCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			if got := r.URL.Query().Get("fields"); got != "attachment" {
				t.Errorf("fields = %q, want %q", got, "attachment")
			}
			testhelpers.WriteJSONResponse(t, w, `{"fields":{"attachment":[{"id":"10000","filename":"a.txt"}]}}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, attachmentListCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want returned metadata", buf.String())
		}
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "a.txt")
		if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
			t.Fatalf("write temp file: %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/attachments" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/attachments")
			}
			if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
				t.Errorf("X-Atlassian-Token = %q, want %q", got, "no-check")
			}
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Fatalf("parse multipart form: %v", err)
			}
			files := r.MultipartForm.File["file"]
			if len(files) != 1 {
				t.Fatalf("file count = %d, want 1", len(files))
			}
			if got := files[0].Filename; got != "a.txt" {
				t.Errorf("filename = %q, want %q", got, "a.txt")
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"10000","filename":"a.txt"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, attachmentAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--file", path, "TEST-1")
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/attachment/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/attachment/10000")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000","filename":"a.txt"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, attachmentGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "10000")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/attachment/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/attachment/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, attachmentDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "10000")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want delete confirmation", buf.String())
		}
	})
}

func TestContextCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/field/customfield_10000/context" {
				t.Errorf("path = %q, want context list path", r.URL.Path)
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"values":[{"id":"20000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			contextListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--start-at", "3", "--max-results", "25", "customfield_10000",
		)
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/field/customfield_10000/context" {
				t.Errorf("path = %q, want context path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["name"] != "Bug context" {
				t.Errorf("name = %v, want %v", body["name"], "Bug context")
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"20000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			contextCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--name", "Bug context", "--description", "desc", "--projects", "10000,10001", "--issue-types", "10002", "customfield_10000",
		)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/field/customfield_10000/context/20000" {
				t.Errorf("path = %q, want update path", r.URL.Path)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"20000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			contextUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--name", "Renamed", "customfield_10000", "20000",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/field/customfield_10000/context/20000" {
				t.Errorf("path = %q, want delete path", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, contextDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "customfield_10000", "20000")
	})
}

func TestOptionCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/field/customfield_10000/context/20000/option" {
				t.Errorf("path = %q, want option list path", r.URL.Path)
			}
			if got := r.URL.Query().Get("startAt"); got != "2" {
				t.Errorf("startAt = %q, want %q", got, "2")
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"values":[{"id":"30000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			optionListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--start-at", "2", "--max-results", "20", "customfield_10000", "20000",
		)
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/field/customfield_10000/context/20000/option" {
				t.Errorf("path = %q, want option path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			options := body["options"].([]any)
			if len(options) != 2 {
				t.Errorf("options length = %d, want %d", len(options), 2)
			}
			testhelpers.WriteJSONResponse(t, w, `{"options":[{"id":"30000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			optionCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--values", "One, Two", "customfield_10000", "20000",
		)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			options := body["options"].([]any)
			option := options[0].(map[string]any)
			if option["disabled"] != true {
				t.Errorf("disabled = %v, want %v", option["disabled"], true)
			}
			testhelpers.WriteJSONResponse(t, w, `{"options":[{"id":"30000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			optionUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--option-id", "30000", "--value", "Done", "--disabled", "customfield_10000", "20000",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/field/customfield_10000/context/20000/option/30000" {
				t.Errorf("path = %q, want option delete path", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			optionDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"customfield_10000", "20000", "30000",
		)
	})

	t.Run("reorder", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/field/customfield_10000/context/20000/option/move" {
				t.Errorf("path = %q, want move path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["position"] != "After" {
				t.Errorf("position = %v, want %v", body["position"], "After")
			}
			if body["after"] != "29999" {
				t.Errorf("after = %v, want %v", body["after"], "29999")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			optionReorderCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--option-ids", "30000,30001", "--position", "After", "--anchor", "29999", "customfield_10000", "20000",
		)
	})
}

func TestIssueGetAndSearchCommands(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			if got := r.URL.Query().Get("fields"); got != "summary,status" {
				t.Errorf("fields = %q, want %q", got, "summary,status")
			}
			if got := r.URL.Query().Get("expand"); got != "changelog" {
				t.Errorf("expand = %q, want %q", got, "changelog")
			}
			if got := r.URL.Query().Get("properties"); got != "request" {
				t.Errorf("properties = %q, want %q", got, "request")
			}
			if got := r.URL.Query().Get("fieldsByKeys"); got != "true" {
				t.Errorf("fieldsByKeys = %q, want %q", got, "true")
			}
			if got := r.URL.Query().Get("updateHistory"); got != "true" {
				t.Errorf("updateHistory = %q, want %q", got, "true")
			}
			if got := r.URL.Query().Get("failFast"); got != "false" {
				t.Errorf("failFast = %q, want %q", got, "false")
			}
			testhelpers.WriteJSONResponse(t, w, `{"key":"TEST-1"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--fields", "summary,status", "--expand", "changelog", "--properties", "request",
			"--fields-by-keys", "--update-history", "--fail-fast=false", "TEST-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"TEST-1"`)) {
			t.Errorf("output = %q, want issue key", buf.String())
		}
	})

	t.Run("picker", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/picker" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/picker")
			}
			if got := r.URL.Query().Get("query"); got != "login" {
				t.Errorf("query = %q, want %q", got, "login")
			}
			if got := r.URL.Query().Get("currentJQL"); got != "project = TEST" {
				t.Errorf("currentJQL = %q, want %q", got, "project = TEST")
			}
			if got := r.URL.Query().Get("showSubTasks"); got != "false" {
				t.Errorf("showSubTasks = %q, want %q", got, "false")
			}
			testhelpers.WriteJSONResponse(t, w, `{"sections":[{"issues":[{"key":"TEST-1"}]}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issuePickerCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--query", "login", "--current-jql", "project = TEST", "--show-subtasks=false",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"TEST-1"`)) {
			t.Errorf("output = %q, want issue key", buf.String())
		}
	})

	t.Run("search", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/search/jql" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/search/jql")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["jql"] != "project = TEST ORDER BY created DESC" {
				t.Errorf("jql = %v, want ORDER BY clause", body["jql"])
			}
			if body["nextPageToken"] != "token-1" {
				t.Errorf("nextPageToken = %v, want %v", body["nextPageToken"], "token-1")
			}
			fields := body["fields"].([]any)
			if len(fields) != 2 || fields[0] != "summary" || fields[1] != "status" {
				t.Errorf("fields = %v, want [summary status]", fields)
			}
			properties := body["properties"].([]any)
			if len(properties) != 1 || properties[0] != "request" {
				t.Errorf("properties = %v, want [request]", properties)
			}
			if body["fieldsByKeys"] != true {
				t.Errorf("fieldsByKeys = %v, want true", body["fieldsByKeys"])
			}
			if body["failFast"] != false {
				t.Errorf("failFast = %v, want false", body["failFast"])
			}
			reconcileIssues := body["reconcileIssues"].([]any)
			if len(reconcileIssues) != 1 || reconcileIssues[0] != "10001" {
				t.Errorf("reconcileIssues = %v, want [10001]", reconcileIssues)
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"startAt":0,"maxResults":25,"issues":[{"key":"TEST-1"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueSearchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--jql", "project = TEST", "--fields", "summary,status", "--max-results", "25",
			"--next-page-token", "token-1", "--order-by", "created", "--order", "desc",
			"--properties", "request", "--fields-by-keys", "--fail-fast=false", "--reconcile-issues", "10001",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})
}

func TestIssueMutationCommands(t *testing.T) {
	t.Parallel()

	t.Run("bulk create wraps issue updates array", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/bulk" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/bulk")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			updates := body["issueUpdates"].([]any)
			if len(updates) != 1 {
				t.Fatalf("issueUpdates length = %d, want %d", len(updates), 1)
			}
			issue := updates[0].(map[string]any)
			fields := issue["fields"].(map[string]any)
			if fields["summary"] != "Bulk issue" {
				t.Errorf("summary = %v, want %v", fields["summary"], "Bulk issue")
			}
			testhelpers.WriteJSONResponse(t, w, `{"issues":[{"key":"TEST-10"}],"errors":[]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueBulkCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--issues-json", `[{"fields":{"project":{"key":"TEST"},"issuetype":{"name":"Task"},"summary":"Bulk issue"}}]`,
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"TEST-10"`)) {
			t.Errorf("output = %q, want created issue key", buf.String())
		}
	})

	t.Run("bulk fetch posts issue key list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/bulkfetch" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/bulkfetch")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			issues := body["issueIdsOrKeys"].([]any)
			if len(issues) != 2 || issues[0] != "TEST-1" || issues[1] != "10002" {
				t.Errorf("issueIdsOrKeys = %v, want [TEST-1 10002]", issues)
			}
			fields := body["fields"].([]any)
			if len(fields) != 2 || fields[0] != "summary" || fields[1] != "status" {
				t.Errorf("fields = %v, want [summary status]", fields)
			}
			expand := body["expand"].([]any)
			if len(expand) != 1 || expand[0] != "changelog" {
				t.Errorf("expand = %v, want [changelog]", expand)
			}
			properties := body["properties"].([]any)
			if len(properties) != 1 || properties[0] != "custom" {
				t.Errorf("properties = %v, want [custom]", properties)
			}
			if body["fieldsByKeys"] != true {
				t.Errorf("fieldsByKeys = %v, want true", body["fieldsByKeys"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"issues":[{"key":"TEST-1"}],"issueErrors":[]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueBulkFetchCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--issues", "TEST-1,10002", "--fields", "summary,status", "--expand", "changelog",
			"--properties", "custom", "--fields-by-keys",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"key":"TEST-1"`)) {
			t.Errorf("output = %q, want fetched issue key", buf.String())
		}
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			fields := body["fields"].(map[string]any)
			project := fields["project"].(map[string]any)
			if project["key"] != "TEST" {
				t.Errorf("project[key] = %v, want %v", project["key"], "TEST")
			}
			if fields["summary"] != "New issue" {
				t.Errorf("summary = %v, want %v", fields["summary"], "New issue")
			}
			labels := fields["labels"].([]any)
			if len(labels) != 2 {
				t.Errorf("labels length = %d, want %d", len(labels), 2)
			}
			testhelpers.WriteJSONResponse(t, w, `{"key":"TEST-2"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--project", "TEST", "--type", "Task", "--summary", "New issue", "--labels", "bug,cli",
			"--field", "customfield_10000=5", "--fields-json", `{"priority":{"name":"High"}}`,
		)
	})

	t.Run("create payload-json", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			fields := body["fields"].(map[string]any)
			project := fields["project"].(map[string]any)
			if project["key"] != "TEST" {
				t.Errorf("project[key] = %v, want %v", project["key"], "TEST")
			}
			if fields["summary"] != "Full payload" {
				t.Errorf("summary = %v, want %v", fields["summary"], "Full payload")
			}
			properties := body["properties"].([]any)
			property := properties[0].(map[string]any)
			if property["key"] != "request" {
				t.Errorf("property key = %v, want request", property["key"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"key":"TEST-3"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--project", "TEST",
			"--payload-json", `{"fields":{"summary":"Full payload","issuetype":{"name":"Task"}},"properties":[{"key":"request","value":"cli"}]}`,
		)
	})

	t.Run("edit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.String() != "/issue/TEST-1?notifyUsers=false" {
				t.Errorf("url = %q, want edit URL", r.URL.String())
			}
			body := testhelpers.DecodeJSONBody(t, r)
			fields := body["fields"].(map[string]any)
			if fields["summary"] != "Updated" {
				t.Errorf("summary = %v, want %v", fields["summary"], "Updated")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueEditCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--summary", "Updated", "--notify=false", "TEST-1",
		)
	})

	t.Run("edit payload json", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want edit path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			update := body["update"].(map[string]any)
			labels := update["labels"].([]any)
			labelAdd := labels[0].(map[string]any)
			if labelAdd["add"] != "triaged" {
				t.Errorf("label add = %v, want triaged", labelAdd["add"])
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueEditCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--payload-json", `{"update":{"labels":[{"add":"triaged"}]}}`, "TEST-1",
		)
	})

	t.Run("assign account", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1/assignee" {
				t.Errorf("path = %q, want assign path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["accountId"] != "abc123" {
				t.Errorf("accountId = %v, want %v", body["accountId"], "abc123")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueAssignCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1", "abc123")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})

	t.Run("delete with subtasks", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1")
			}
			if got := r.URL.Query().Get("deleteSubtasks"); got != "true" {
				t.Errorf("deleteSubtasks = %q, want %q", got, "true")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, issueDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()), "--delete-subtasks", "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})
}

func TestIssueTransitionAndMetaCommands(t *testing.T) {
	t.Parallel()

	t.Run("transition", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.Method != http.MethodGet || r.URL.Path != "/issue/TEST-1/transitions" {
					t.Errorf("first request = %s %s, want GET transitions", r.Method, r.URL.Path)
				}
				if got := r.URL.Query().Get("expand"); got != "transitions.fields" {
					t.Errorf("expand = %q, want %q", got, "transitions.fields")
				}
				if got := r.URL.Query().Get("includeUnavailableTransitions"); got != "true" {
					t.Errorf("includeUnavailableTransitions = %q, want true", got)
				}
				testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31","name":"Start Progress","to":{"name":"In Progress"}}]}`)
			case 2:
				if r.Method != http.MethodPost || r.URL.Path != "/issue/TEST-1/transitions" {
					t.Errorf("second request = %s %s, want POST transitions", r.Method, r.URL.Path)
				}
				body := testhelpers.DecodeJSONBody(t, r)
				transition := body["transition"].(map[string]any)
				if transition["id"] != "31" {
					t.Errorf("transition[id] = %v, want %v", transition["id"], "31")
				}
				if body["update"] == nil {
					t.Errorf("update = nil, want transition comment")
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("request count = %d, want at most 2", requests)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueTransitionCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--to", "in progress", "--field", "resolution=null", "--comment", "moving",
			"--expand", "transitions.fields", "--include-unavailable-transitions", "TEST-1",
		)
	})

	t.Run("transition direct id payload", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/transitions" {
				t.Errorf("path = %q, want transition path", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			transition := body["transition"].(map[string]any)
			if transition["id"] != "41" {
				t.Errorf("transition[id] = %v, want 41", transition["id"])
			}
			properties := body["properties"].([]any)
			property := properties[0].(map[string]any)
			if property["key"] != "request" {
				t.Errorf("property key = %v, want request", property["key"])
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueTransitionCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--transition-id", "41", "--payload-json", `{"properties":[{"key":"request","value":"cli"}]}`, "TEST-1",
		)
	})

	t.Run("transition list filters", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/transitions" {
				t.Errorf("path = %q, want transitions path", r.URL.Path)
			}
			if got := r.URL.Query().Get("transitionId"); got != "31" {
				t.Errorf("transitionId = %q, want 31", got)
			}
			if got := r.URL.Query().Get("skipRemoteOnlyCondition"); got != "true" {
				t.Errorf("skipRemoteOnlyCondition = %q, want true", got)
			}
			if got := r.URL.Query().Get("sortByOpsBarAndStatus"); got != "true" {
				t.Errorf("sortByOpsBarAndStatus = %q, want true", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"transitions":[{"id":"31"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueTransitionCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--list", "--list-transition-id", "31", "--skip-remote-only-condition", "--sort-by-ops-bar-and-status", "TEST-1",
		)
	})

	t.Run("meta create type", func(t *testing.T) {
		t.Parallel()

		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			switch requests {
			case 1:
				if r.URL.Path != "/issue/createmeta/TEST/issuetypes" {
					t.Errorf("path = %q, want issue types path", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10001","name":"Task"}]}`)
			case 2:
				if r.URL.Path != "/issue/createmeta/TEST/issuetypes/10001" {
					t.Errorf("path = %q, want type metadata path", r.URL.Path)
				}
				testhelpers.WriteJSONResponse(t, w, `{"fields":{"summary":{"required":true}}}`)
			default:
				t.Errorf("request count = %d, want at most 2", requests)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMetaCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--project", "TEST", "--type", "task",
		)
	})

	t.Run("meta edit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/issue/TEST-1/editmeta" {
				t.Errorf("path = %q, want editmeta path", r.URL.Path)
			}
			testhelpers.WriteJSONResponse(t, w, `{"fields":{}}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueMetaCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--operation", "edit", "--issue", "TEST-1",
		)
	})
}

func TestIssueChangelogCommand(t *testing.T) {
	t.Parallel()

	t.Run("list changelogs", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != "/issue/TEST-1/changelog" {
				t.Errorf("path = %q, want /issue/TEST-1/changelog", r.URL.Path)
			}
			if got := r.URL.Query().Get("maxResults"); got != "5" {
				t.Errorf("maxResults = %q, want %q", got, "5")
			}
			testhelpers.WriteJSONResponse(t, w, `{
				"values": [{"id":"10001","created":"2024-01-01T00:00:00.000+0000","items":[{"field":"status","fromString":"Open","toString":"In Progress"}]}],
				"startAt": 0,
				"maxResults": 5,
				"total": 1
			}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--max-results", "5", "TEST-1",
		)

		if !bytes.Contains(buf.Bytes(), []byte("In Progress")) {
			t.Errorf("output missing changelog item, got %s", buf.String())
		}
	})

	t.Run("pagination params", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("startAt"); got != "10" {
				t.Errorf("startAt = %q, want %q", got, "10")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[],"startAt":10,"maxResults":25,"total":10}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--start-at", "10", "--max-results", "25", "TEST-1",
		)
	})

	t.Run("list by ids", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/issue/TEST-1/changelog/list" {
				t.Errorf("path = %q, want /issue/TEST-1/changelog/list", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			ids, ok := body["changelogIds"].([]any)
			if !ok || len(ids) != 2 || ids[0] != float64(10001) || ids[1] != float64(10002) {
				t.Errorf("changelogIds = %v, want [10001 10002]", body["changelogIds"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10001"}],"startAt":0,"maxResults":2,"total":1}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"list-by-ids", "--ids", "10001,10002", "TEST-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"10001"`)) {
			t.Errorf("output = %q, want changelog ID", buf.String())
		}
	})

	t.Run("bulk fetch", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/changelog/bulkfetch" {
				t.Errorf("path = %q, want /changelog/bulkfetch", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			issues, ok := body["issueIdsOrKeys"].([]any)
			if !ok || len(issues) != 2 || issues[0] != "TEST-1" || issues[1] != "TEST-2" {
				t.Errorf("issueIdsOrKeys = %v, want [TEST-1 TEST-2]", body["issueIdsOrKeys"])
			}
			fields, ok := body["fieldIds"].([]any)
			if !ok || len(fields) != 2 || fields[0] != "status" || fields[1] != "assignee" {
				t.Errorf("fieldIds = %v, want [status assignee]", body["fieldIds"])
			}
			if body["maxResults"] != float64(100) {
				t.Errorf("maxResults = %v, want 100", body["maxResults"])
			}
			if body["nextPageToken"] != "token-1" {
				t.Errorf("nextPageToken = %v, want token-1", body["nextPageToken"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"issueChangeLogs":[{"issueId":"10001","changeHistories":[{"id":"20001"}]}],"nextPageToken":"token-2"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueChangelogCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"bulk-fetch", "--issues", "TEST-1,TEST-2", "--field-ids", "status,assignee", "--max-results", "100", "--next-page-token", "token-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"issueId":"10001"`)) {
			t.Errorf("output = %q, want bulk changelog result", buf.String())
		}
	})
}

func TestIssueRankCommand(t *testing.T) {
	t.Parallel()

	t.Run("rank before", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %s, want PUT", r.Method)
			}
			if r.URL.Path != "/rest/agile/1.0/issue/rank" {
				t.Errorf("path = %q, want /rest/agile/1.0/issue/rank", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			issues, ok := body["issues"].([]any)
			if !ok || len(issues) != 2 {
				t.Fatalf("issues = %v, want 2 items", body["issues"])
			}
			if got, want := issues[0], "TEST-1"; got != want {
				t.Errorf("issues[0] = %v, want %s", got, want)
			}
			if got, want := body["rankBeforeIssue"], "TEST-3"; got != want {
				t.Errorf("rankBeforeIssue = %v, want %s", got, want)
			}
			if _, found := body["rankAfterIssue"]; found {
				t.Errorf("rankAfterIssue should not be present")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueRankCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
			"--issues", "TEST-1,TEST-2", "--before", "TEST-3",
		)

		if !bytes.Contains(buf.Bytes(), []byte("TEST-1")) {
			t.Errorf("output missing ranked issue, got %s", buf.String())
		}
	})

	t.Run("rank after", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := testhelpers.DecodeJSONBody(t, r)
			if got, want := body["rankAfterIssue"], "TEST-5"; got != want {
				t.Errorf("rankAfterIssue = %v, want %s", got, want)
			}
			if _, found := body["rankBeforeIssue"]; found {
				t.Errorf("rankBeforeIssue should not be present")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueRankCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
			"--issues", "TEST-4", "--after", "TEST-5",
		)
	})

	t.Run("requires before or after", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueRankCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), testAllowWrites())

		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs([]string{"--issues", "TEST-1"})
		err := cmd.Execute()

		var validErr *apperr.ValidationError
		if !errors.As(err, &validErr) {
			t.Errorf("err type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("rejects both before and after", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueRankCommand(testCommandClient("http://unused"), &buf, testCommandFormat(), testAllowWrites())

		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs([]string{"--issues", "TEST-1", "--before", "TEST-2", "--after", "TEST-3"})
		err := cmd.Execute()

		var validErr *apperr.ValidationError
		if !errors.As(err, &validErr) {
			t.Errorf("err type = %T, want *apperr.ValidationError", err)
		}
	})
}

func TestIssueCountCommand(t *testing.T) {
	t.Parallel()

	t.Run("counts issues", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/search/jql" {
				t.Errorf("path = %q, want /search/jql", r.URL.Path)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got, want := body["jql"], "project = FOO"; got != want {
				t.Errorf("jql = %v, want %s", got, want)
			}
			if got, want := body["maxResults"], float64(0); got != want {
				t.Errorf("maxResults = %v, want %v", got, want)
			}
			testhelpers.WriteJSONResponse(t, w, `{"issues":[],"maxResults":0,"startAt":0,"total":42}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			issueCountCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--jql", "project = FOO",
		)

		var got struct {
			Data struct {
				Total int `json:"total"`
			} `json:"data"`
			Metadata struct {
				Total int `json:"total"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v, output %s", err, buf.String())
		}
		if got.Data.Total != 42 {
			t.Errorf("data.total = %d, want 42", got.Data.Total)
		}
		if got.Metadata.Total != 42 {
			t.Errorf("metadata.total = %d, want 42", got.Metadata.Total)
		}
	})

	t.Run("requires jql", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueCountCommand(testCommandClient("http://unused"), &buf, testCommandFormat())

		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("cmd.Execute() error = nil, want validation error")
		}

		var validErr *apperr.ValidationError
		if !errors.As(err, &validErr) {
			t.Errorf("err type = %T, want *apperr.ValidationError", err)
		}
		if !strings.Contains(err.Error(), "--jql is required") {
			t.Errorf("err = %q, want --jql is required", err.Error())
		}
	})
}

func TestIssueWorklogCommands(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/worklog" {
				t.Errorf("path = %q, want worklog list path", r.URL.Path)
			}
			if got := r.URL.Query().Get("startAt"); got != "5" {
				t.Errorf("startAt = %q, want %q", got, "5")
			}
			if got := r.URL.Query().Get("maxResults"); got != "25" {
				t.Errorf("maxResults = %q, want %q", got, "25")
			}
			if got := r.URL.Query().Get("startedAfter"); got != "1700000000000" {
				t.Errorf("startedAfter = %q, want %q", got, "1700000000000")
			}
			testhelpers.WriteJSONResponse(t, w, `{"total":1,"startAt":5,"maxResults":25,"worklogs":[{"id":"10000"}]}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--start-at", "5", "--max-results", "25", "--started-after", "1700000000000", "TEST-1",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"returned":1`)) {
			t.Errorf("output = %q, want pagination metadata", buf.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/issue/TEST-1/worklog/10000" {
				t.Errorf("path = %q, want worklog get path", r.URL.Path)
			}
			if got := r.URL.Query().Get("expand"); got != "properties" {
				t.Errorf("expand = %q, want properties", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--expand", "properties", "TEST-1", "10000",
		)
	})

	t.Run("updated", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/worklog/updated" {
				t.Errorf("path = %q, want /worklog/updated", r.URL.Path)
			}
			if got := r.URL.Query().Get("since"); got != "1700000000000" {
				t.Errorf("since = %q, want timestamp", got)
			}
			if got := r.URL.Query().Get("expand"); got != "properties" {
				t.Errorf("expand = %q, want properties", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[{"worklogId":"10000"}],"since":1700000000000}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogUpdatedCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--since", "1700000000000", "--expand", "properties",
		)
	})

	t.Run("deleted", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/worklog/deleted" {
				t.Errorf("path = %q, want /worklog/deleted", r.URL.Path)
			}
			if got := r.URL.Query().Get("since"); got != "1700000000000" {
				t.Errorf("since = %q, want timestamp", got)
			}
			testhelpers.WriteJSONResponse(t, w, `{"values":[{"worklogId":"10000"}],"since":1700000000000}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(t, worklogDeletedCommand(testCommandClient(server.URL), &buf, testCommandFormat()), "--since", "1700000000000")
	})

	t.Run("list by ids", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/worklog/list" {
				t.Errorf("path = %q, want /worklog/list", r.URL.Path)
			}
			if got := r.URL.Query().Get("expand"); got != "properties" {
				t.Errorf("expand = %q, want properties", got)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			ids := body["ids"].([]any)
			if len(ids) != 2 || ids[0] != float64(10000) || ids[1] != float64(10001) {
				t.Errorf("ids = %v, want [10000 10001]", ids)
			}
			testhelpers.WriteJSONResponse(t, w, `[{"id":"10000"}]`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogListByIDsCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--ids", "10000,10001", "--expand", "properties",
		)
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/issue/TEST-1/worklog" {
				t.Errorf("path = %q, want worklog add path", r.URL.Path)
			}
			if got := r.URL.Query().Get("adjustEstimate"); got != "new" {
				t.Errorf("adjustEstimate = %q, want %q", got, "new")
			}
			if got := r.URL.Query().Get("newEstimate"); got != "2d" {
				t.Errorf("newEstimate = %q, want %q", got, "2d")
			}
			if got := r.URL.Query().Get("notifyUsers"); got != "false" {
				t.Errorf("notifyUsers = %q, want %q", got, "false")
			}
			if got := r.URL.Query().Get("expand"); got != "properties" {
				t.Errorf("expand = %q, want properties", got)
			}
			if got := r.URL.Query().Get("reduceBy"); got != "30m" {
				t.Errorf("reduceBy = %q, want 30m", got)
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["started"] != "2026-04-27T10:00:00.000-0500" {
				t.Errorf("started = %v, want timestamp", body["started"])
			}
			if body["timeSpent"] != "1h" {
				t.Errorf("timeSpent = %v, want %v", body["timeSpent"], "1h")
			}
			if body["comment"] == nil {
				t.Errorf("comment = nil, want ADF comment")
			}
			visibility := body["visibility"].(map[string]any)
			if visibility["value"] != "Developers" {
				t.Errorf("visibility[value] = %v, want Developers", visibility["value"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogAddCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--started", "2026-04-27T10:00:00.000-0500", "--time-spent", "1h", "--comment", "worked",
			"--visibility-type", "role", "--visibility-value", "Developers", "--notify=false",
			"--adjust-estimate", "new", "--new-estimate", "2d", "--reduce-by", "30m", "--expand", "properties", "TEST-1",
		)
	})

	t.Run("edit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
			}
			if r.URL.Path != "/issue/TEST-1/worklog/10000" {
				t.Errorf("path = %q, want worklog edit path", r.URL.Path)
			}
			if got := r.URL.Query().Get("overrideEditableFlag"); got != "true" {
				t.Errorf("overrideEditableFlag = %q, want %q", got, "true")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if body["timeSpentSeconds"] != float64(1800) {
				t.Errorf("timeSpentSeconds = %v, want %v", body["timeSpentSeconds"], 1800)
			}
			testhelpers.WriteJSONResponse(t, w, `{"id":"10000"}`)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogEditCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--time-spent-seconds", "1800", "--override-editable-flag", "TEST-1", "10000",
		)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/issue/TEST-1/worklog/10000" {
				t.Errorf("path = %q, want worklog delete path", r.URL.Path)
			}
			if got := r.URL.Query().Get("adjustEstimate"); got != "manual" {
				t.Errorf("adjustEstimate = %q, want %q", got, "manual")
			}
			if got := r.URL.Query().Get("increaseBy"); got != "1h" {
				t.Errorf("increaseBy = %q, want %q", got, "1h")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			worklogDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--adjust-estimate", "manual", "--increase-by", "1h", "TEST-1", "10000",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})
}

func TestComponentListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/component" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/component")
		}
		if got := r.URL.Query().Get("projectIdsOrKeys"); got != "PROJ" {
			t.Errorf("projectIdsOrKeys = %q, want %q", got, "PROJ")
		}
		testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10000","name":"Backend"}],"total":1,"startAt":0,"maxResults":50}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		componentListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--project", "PROJ",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Backend"`)) {
		t.Errorf("output = %q, want component name", buf.String())
	}
}

func TestComponentGetCommand(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, http.MethodGet, "/component/10000",
		`{"id":"10000","name":"Backend","project":"PROJ"}`)

	var buf bytes.Buffer
	runCommandAction(
		t,
		componentGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Backend"`)) {
		t.Errorf("output = %q, want component name", buf.String())
	}
}

func TestComponentCreateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/component" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/component")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "Backend" {
			t.Errorf("name = %v, want %q", got, "Backend")
		}
		if got := body["project"]; got != "PROJ" {
			t.Errorf("project = %v, want %q", got, "PROJ")
		}
		if got := body["description"]; got != "Backend services" {
			t.Errorf("description = %v, want %q", got, "Backend services")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"id":"10001","name":"Backend","project":"PROJ"}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		componentCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "Backend", "--project", "PROJ", "--description", "Backend services",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"10001"`)) {
		t.Errorf("output = %q, want component id", buf.String())
	}
}

func TestComponentUpdateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/component/10000" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/component/10000")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "Backend v2" {
			t.Errorf("name = %v, want %q", got, "Backend v2")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"Backend v2"}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		componentUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "Backend v2", "10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Backend v2"`)) {
		t.Errorf("output = %q, want updated name", buf.String())
	}
}

func TestComponentDeleteCommand(t *testing.T) {
	t.Parallel()

	t.Run("basic_delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/component/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/component/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			componentDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"10000",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})

	t.Run("delete_with_move_issues", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if got := r.URL.Query().Get("moveIssuesTo"); got != "10001" {
				t.Errorf("moveIssuesTo = %q, want %q", got, "10001")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			componentDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--move-issues-to", "10001", "10000",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})
}

func TestComponentIssueCountsCommand(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, http.MethodGet, "/component/10000/relatedIssueCounts",
		`{"issueCount":42,"self":"https://jira.example.com/rest/api/3/component/10000"}`)

	var buf bytes.Buffer
	runCommandAction(
		t,
		componentIssueCountsCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`42`)) {
		t.Errorf("output = %q, want issue count", buf.String())
	}
}

func TestVersionListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/project/PROJ/version" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/project/PROJ/version")
		}
		if got := r.URL.Query().Get("status"); got != "released" {
			t.Errorf("status = %q, want %q", got, "released")
		}
		testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10000","name":"v1.0","released":true}],"total":1,"startAt":0,"maxResults":50}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionListCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--project", "PROJ", "--status", "released",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"v1.0"`)) {
		t.Errorf("output = %q, want version name", buf.String())
	}
}

func TestVersionGetCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/version/10000" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/version/10000")
		}
		if got := r.URL.Query().Get("expand"); got != "issuesstatus" {
			t.Errorf("expand = %q, want %q", got, "issuesstatus")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"v1.0","released":true}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionGetCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"--expand", "issuesstatus", "10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"v1.0"`)) {
		t.Errorf("output = %q, want version name", buf.String())
	}
}

func TestVersionCreateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/version" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/version")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "v2.0" {
			t.Errorf("name = %v, want %q", got, "v2.0")
		}
		if got := body["project"]; got != "PROJ" {
			t.Errorf("project = %v, want %q", got, "PROJ")
		}
		if got := body["releaseDate"]; got != "2025-06-01" {
			t.Errorf("releaseDate = %v, want %q", got, "2025-06-01")
		}
		if got, ok := body["released"]; !ok || got != true {
			t.Errorf("released = %v, want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"id":"10001","name":"v2.0","project":"PROJ"}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionCreateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "v2.0", "--project", "PROJ", "--release-date", "2025-06-01", "--released",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"10001"`)) {
		t.Errorf("output = %q, want version id", buf.String())
	}
}

func TestVersionUpdateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/version/10000" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/version/10000")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "v1.1" {
			t.Errorf("name = %v, want %q", got, "v1.1")
		}
		if got, ok := body["archived"]; !ok || got != true {
			t.Errorf("archived = %v, want true", got)
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"v1.1","archived":true}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionUpdateCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "v1.1", "--archived", "10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"v1.1"`)) {
		t.Errorf("output = %q, want updated name", buf.String())
	}
}

func TestVersionDeleteCommand(t *testing.T) {
	t.Parallel()

	t.Run("basic_delete", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if r.URL.Path != "/version/10000" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/version/10000")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			versionDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"10000",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})

	t.Run("delete_with_move_params", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
			}
			if got := r.URL.Query().Get("moveFixIssuesTo"); got != "10001" {
				t.Errorf("moveFixIssuesTo = %q, want %q", got, "10001")
			}
			if got := r.URL.Query().Get("moveAffectedIssuesTo"); got != "10002" {
				t.Errorf("moveAffectedIssuesTo = %q, want %q", got, "10002")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		var buf bytes.Buffer
		runCommandAction(
			t,
			versionDeleteCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--move-fix-issues-to", "10001", "--move-affected-issues-to", "10002", "10000",
		)
		if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
			t.Errorf("output = %q, want deleted result", buf.String())
		}
	})
}

func TestVersionMergeCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/version/10000/mergeto/10001" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/version/10000/mergeto/10001")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionMergeCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"10000", "10001",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"merged":true`)) {
		t.Errorf("output = %q, want merged result", buf.String())
	}
}

func TestVersionMoveCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/version/10000/move" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/version/10000/move")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["position"]; got != "First" {
			t.Errorf("position = %v, want %q", got, "First")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":"10000","name":"v1.0"}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionMoveCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
		"--position", "First", "10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"v1.0"`)) {
		t.Errorf("output = %q, want version name", buf.String())
	}
}

func TestVersionIssueCountsCommand(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, http.MethodGet, "/version/10000/relatedIssueCounts",
		`{"issuesFixedCount":23,"issuesAffectedCount":101}`)

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionIssueCountsCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`23`)) {
		t.Errorf("output = %q, want issue count", buf.String())
	}
}

func TestVersionUnresolvedCountCommand(t *testing.T) {
	t.Parallel()

	server := testhelpers.NewJSONServer(t, http.MethodGet, "/version/10000/unresolvedIssueCount",
		`{"issuesCount":30,"issuesUnresolvedCount":23}`)

	var buf bytes.Buffer
	runCommandAction(
		t,
		versionUnresolvedCountCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
		"10000",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`23`)) {
		t.Errorf("output = %q, want unresolved count", buf.String())
	}
}

func TestSprintListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/board/42/sprint" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/42/sprint")
		}
		if got := r.URL.Query().Get("state"); got != "active" {
			t.Errorf("state = %q, want %q", got, "active")
		}
		testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":100,"name":"Sprint 1","state":"active"}],"total":1,"startAt":0,"maxResults":50}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintListCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
		"--board-id", "42", "--state", "active",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Sprint 1"`)) {
		t.Errorf("output = %q, want sprint name", buf.String())
	}
}

func TestSprintGetCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":100,"name":"Sprint 1","state":"active","goal":"Ship v1"}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintGetCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
		"100",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Ship v1"`)) {
		t.Errorf("output = %q, want sprint goal", buf.String())
	}
}

func TestSprintCreateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "Sprint 2" {
			t.Errorf("name = %v, want %q", got, "Sprint 2")
		}
		if got, ok := body["originBoardId"].(float64); !ok || int(got) != 42 {
			t.Errorf("originBoardId = %v, want 42", body["originBoardId"])
		}
		if got := body["goal"]; got != "Ship v2" {
			t.Errorf("goal = %v, want %q", got, "Ship v2")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":101,"name":"Sprint 2","state":"future","goal":"Ship v2"}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintCreateCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "Sprint 2", "--board-id", "42", "--goal", "Ship v2",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Sprint 2"`)) {
		t.Errorf("output = %q, want sprint name", buf.String())
	}
}

func TestSprintUpdateCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got := body["name"]; got != "Sprint 1 (updated)" {
			t.Errorf("name = %v, want %q", got, "Sprint 1 (updated)")
		}
		if got := body["state"]; got != "closed" {
			t.Errorf("state = %v, want %q", got, "closed")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":100,"name":"Sprint 1 (updated)","state":"closed"}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintUpdateCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--name", "Sprint 1 (updated)", "--state", "closed", "100",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"Sprint 1 (updated)"`)) {
		t.Errorf("output = %q, want updated name", buf.String())
	}
}

func TestSprintDeleteCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintDeleteCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"100",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"deleted":true`)) {
		t.Errorf("output = %q, want deleted result", buf.String())
	}
}

func TestSprintIssuesCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100/issue")
		}
		if got := r.URL.Query().Get("jql"); got != "status=Done" {
			t.Errorf("jql = %q, want %q", got, "status=Done")
		}
		testhelpers.WriteJSONResponse(t, w, `{"issues":[{"key":"PROJ-1","fields":{"summary":"Test"}}],"total":1,"startAt":0,"maxResults":50}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
		"--jql", "status=Done", "100",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"PROJ-1"`)) {
		t.Errorf("output = %q, want issue key", buf.String())
	}
}

func TestSprintMoveIssuesCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100/issue")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		issues, ok := body["issues"].([]any)
		if !ok || len(issues) != 2 {
			t.Fatalf("issues = %v, want 2 items", body["issues"])
		}
		if got := issues[0]; got != "PROJ-1" {
			t.Errorf("issues[0] = %v, want %q", got, "PROJ-1")
		}
		if got := body["rankBeforeIssue"]; got != "PROJ-5" {
			t.Errorf("rankBeforeIssue = %v, want %q", got, "PROJ-5")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintMoveIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--issues", "PROJ-1,PROJ-2", "--rank-before", "PROJ-5", "100",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"moved"`)) {
		t.Errorf("output = %q, want moved result", buf.String())
	}
}

func TestSprintSwapCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/sprint/100/swap" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/sprint/100/swap")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got, ok := body["sprintToSwapWith"].(float64); !ok || int(got) != 101 {
			t.Errorf("sprintToSwapWith = %v, want 101", body["sprintToSwapWith"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(
		t,
		sprintSwapCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"100", "101",
	)
	if !bytes.Contains(buf.Bytes(), []byte(`"swapped":true`)) {
		t.Errorf("output = %q, want swapped result", buf.String())
	}
}

func TestParseSprintID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "valid", input: "42", want: 42},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "non_numeric", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSprintID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSprintID(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseSprintID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEpicGetCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/EPIC-1" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/EPIC-1")
		}
		testhelpers.WriteJSONResponse(t, w, `{"id":1,"key":"EPIC-1","name":"My Epic","done":false}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicGetCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()), "EPIC-1")
	if !bytes.Contains(buf.Bytes(), []byte(`"key":"EPIC-1"`)) {
		t.Errorf("output = %q, want epic payload", buf.String())
	}
}

func TestEpicIssuesCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/EPIC-1/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/EPIC-1/issue")
		}
		if got := r.URL.Query().Get("jql"); got != "status=Done" {
			t.Errorf("jql = %q, want %q", got, "status=Done")
		}
		if got := r.URL.Query().Get("fields"); got != "summary" {
			t.Errorf("fields = %q, want %q", got, "summary")
		}
		testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":1,"issues":[{"key":"TEST-1"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
		"--jql", "status=Done", "--fields", "summary", "EPIC-1")
	if !bytes.Contains(buf.Bytes(), []byte(`"total":1`)) {
		t.Errorf("output = %q, want pagination metadata", buf.String())
	}
}

func TestEpicMoveIssuesCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/EPIC-1/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/EPIC-1/issue")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		issues, ok := body["issues"].([]any)
		if !ok || len(issues) != 2 {
			t.Errorf("issues = %v, want 2 items", body["issues"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicMoveIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--issues", "TEST-1,TEST-2", "EPIC-1")
	if !bytes.Contains(buf.Bytes(), []byte(`"epic":"EPIC-1"`)) {
		t.Errorf("output = %q, want move confirmation", buf.String())
	}
}

func TestEpicOrphansCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/none/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/none/issue")
		}
		testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":3,"issues":[{"key":"TEST-5"},{"key":"TEST-6"},{"key":"TEST-7"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicOrphansCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()))
	if !bytes.Contains(buf.Bytes(), []byte(`"total":3`)) {
		t.Errorf("output = %q, want pagination metadata", buf.String())
	}
}

func TestEpicRemoveIssuesCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/none/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/none/issue")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		issues, ok := body["issues"].([]any)
		if !ok || len(issues) != 1 {
			t.Errorf("issues = %v, want 1 item", body["issues"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicRemoveIssuesCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--issues", "TEST-1")
	if !bytes.Contains(buf.Bytes(), []byte(`"removed"`)) {
		t.Errorf("output = %q, want removal confirmation", buf.String())
	}
}

func TestEpicRankCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPut)
		}
		if r.URL.Path != "/rest/agile/1.0/epic/EPIC-1/rank" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/epic/EPIC-1/rank")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		if got, ok := body["rankBeforeEpic"].(string); !ok || got != "EPIC-2" {
			t.Errorf("rankBeforeEpic = %v, want %q", body["rankBeforeEpic"], "EPIC-2")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, epicRankCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--rank-before", "EPIC-2", "EPIC-1")
	if !bytes.Contains(buf.Bytes(), []byte(`"ranked":true`)) {
		t.Errorf("output = %q, want rank confirmation", buf.String())
	}
}

func TestBacklogListCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/agile/1.0/board/84/backlog" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/board/84/backlog")
		}
		if got := r.URL.Query().Get("jql"); got != "type=Bug" {
			t.Errorf("jql = %q, want %q", got, "type=Bug")
		}
		testhelpers.WriteJSONResponse(t, w, `{"maxResults":50,"startAt":0,"total":2,"issues":[{"key":"TEST-1"},{"key":"TEST-2"}]}`)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, backlogListCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat()),
		"--board-id", "84", "--jql", "type=Bug")
	if !bytes.Contains(buf.Bytes(), []byte(`"total":2`)) {
		t.Errorf("output = %q, want pagination metadata", buf.String())
	}
}

func TestBacklogMoveCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/rest/agile/1.0/backlog/issue" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/agile/1.0/backlog/issue")
		}
		body := testhelpers.DecodeJSONBody(t, r)
		issues, ok := body["issues"].([]any)
		if !ok || len(issues) != 2 {
			t.Errorf("issues = %v, want 2 items", body["issues"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	runCommandAction(t, backlogMoveCommand(testCommandClient(server.URL+"/rest/agile/1.0"), &buf, testCommandFormat(), testAllowWrites()),
		"--issues", "TEST-1,TEST-2")
	if !bytes.Contains(buf.Bytes(), []byte(`"moved"`)) {
		t.Errorf("output = %q, want move confirmation", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Issue notify command
// ---------------------------------------------------------------------------

func TestIssueNotifyCommand(t *testing.T) {
	t.Parallel()

	t.Run("sends notification with recipients", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/issue/TEST-1/notify" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/issue/TEST-1/notify")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if got, _ := body["subject"].(string); got != "Build failed" {
				t.Errorf("subject = %q, want %q", got, "Build failed")
			}
			if got, _ := body["textBody"].(string); got != "Check CI" {
				t.Errorf("textBody = %q, want %q", got, "Check CI")
			}
			to, _ := body["to"].(map[string]any)
			if got, _ := to["watchers"].(bool); !got {
				t.Errorf("to.watchers = %v, want true", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueNotifyCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--subject", "Build failed", "--text-body", "Check CI", "--to-watchers", "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"notified"`)) {
			t.Errorf("output = %q, want notification confirmation", buf.String())
		}
	})

	t.Run("sends notification with user accounts", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := testhelpers.DecodeJSONBody(t, r)
			to, _ := body["to"].(map[string]any)
			users, ok := to["users"].([]any)
			if !ok || len(users) != 1 {
				t.Fatalf("to.users = %v, want 1 user", to["users"])
			}
			u, _ := users[0].(map[string]any)
			if got, _ := u["accountId"].(string); got != "abc123" {
				t.Errorf("accountId = %q, want %q", got, "abc123")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueNotifyCommand(testCommandClient(server.URL), &buf, testCommandFormat(), testAllowWrites()),
			"--subject", "Update", "--to-users", "abc123", "TEST-1")
		if !bytes.Contains(buf.Bytes(), []byte(`"notified"`)) {
			t.Errorf("output = %q, want notification confirmation", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// JQL commands
// ---------------------------------------------------------------------------

func TestJQLCommands(t *testing.T) {
	t.Parallel()

	t.Run("fields returns reference data", func(t *testing.T) {
		t.Parallel()

		server := testhelpers.NewJSONServer(t, http.MethodGet, "/jql/autocompletedata",
			`{"visibleFieldNames":[],"visibleFunctionNames":[],"jqlReservedWords":["and","or"]}`)

		var buf bytes.Buffer
		runCommandAction(t, jqlFieldsCommand(testCommandClient(server.URL), &buf, testCommandFormat()))
		if !bytes.Contains(buf.Bytes(), []byte(`"jqlReservedWords"`)) {
			t.Errorf("output = %q, want jqlReservedWords", buf.String())
		}
	})

	t.Run("suggest passes query params", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/jql/autocompletedata/suggestions" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/jql/autocompletedata/suggestions")
			}
			if got := r.URL.Query().Get("fieldName"); got != "reporter" {
				t.Errorf("fieldName = %q, want %q", got, "reporter")
			}
			if got := r.URL.Query().Get("fieldValue"); got != "john" {
				t.Errorf("fieldValue = %q, want %q", got, "john")
			}
			testhelpers.WriteJSONResponse(t, w, `{"results":[{"value":"john.doe","displayName":"John Doe"}]}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, jqlSuggestCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--field-name", "reporter", "--field-value", "john")
		if !bytes.Contains(buf.Bytes(), []byte(`"john.doe"`)) {
			t.Errorf("output = %q, want john.doe suggestion", buf.String())
		}
	})

	t.Run("validate posts queries with validation mode", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/jql/parse" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/jql/parse")
			}
			if got := r.URL.Query().Get("validation"); got != "warn" {
				t.Errorf("validation = %q, want %q", got, "warn")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			queries, ok := body["queries"].([]any)
			if !ok || len(queries) != 2 {
				t.Errorf("queries = %v, want 2 items", body["queries"])
			}
			testhelpers.WriteJSONResponse(t, w, `{"queries":[{"query":"project = TEST"},{"query":"status = Open"}]}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, jqlValidateCommand(testCommandClient(server.URL), &buf, testCommandFormat()),
			"--query", "project = TEST", "--query", "status = Open", "--validation", "warn")
		if !bytes.Contains(buf.Bytes(), []byte(`"queries"`)) {
			t.Errorf("output = %q, want queries in output", buf.String())
		}
	})
}

func TestIssueBulkOperations(t *testing.T) {
	t.Parallel()

	format := testCommandFormat()

	t.Run("bulk delete sends issue keys", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/bulk/issues/delete" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/delete")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			issues, ok := body["selectedIssueIdsOrKeys"].([]any)
			if !ok || len(issues) != 2 {
				t.Errorf("selectedIssueIdsOrKeys = %v, want 2 items", body["selectedIssueIdsOrKeys"])
			}
			if send, ok := body["sendBulkNotification"].(bool); !ok || send {
				t.Errorf("sendBulkNotification = %v, want false", body["sendBulkNotification"])
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			testhelpers.WriteJSONResponse(t, w, `{"taskId":"10641"}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueBulkDeleteCommand(testCommandClient(server.URL), &buf, format, testAllowWrites()),
			"--issues", "PROJ-1,PROJ-2", "--send-notification=false")
		if !bytes.Contains(buf.Bytes(), []byte(`"taskId"`)) {
			t.Errorf("output = %q, want taskId in output", buf.String())
		}
	})

	t.Run("bulk edit fields lists editable fields", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/bulk/issues/fields" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/fields")
			}
			if got := r.URL.Query().Get("issueIdsOrKeys"); got != "PROJ-1,PROJ-2" {
				t.Errorf("issueIdsOrKeys = %q, want %q", got, "PROJ-1,PROJ-2")
			}
			if got := r.URL.Query().Get("searchText"); got != "summary" {
				t.Errorf("searchText = %q, want %q", got, "summary")
			}
			testhelpers.WriteJSONResponse(t, w, `{"fields":[{"id":"summary","name":"Summary"}]}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueBulkEditFieldsCommand(testCommandClient(server.URL), &buf, format),
			"--issues", "PROJ-1,PROJ-2", "--search-text", "summary")
		if !bytes.Contains(buf.Bytes(), []byte(`"summary"`)) {
			t.Errorf("output = %q, want summary field in output", buf.String())
		}
	})

	t.Run("bulk edit submits payload", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/bulk/issues/fields" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/fields")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if _, ok := body["selectedIssueIdsOrKeys"]; !ok {
				t.Error("missing selectedIssueIdsOrKeys in body")
			}
			if _, ok := body["selectedActions"]; !ok {
				t.Error("missing selectedActions in body")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			testhelpers.WriteJSONResponse(t, w, `{"taskId":"10642"}`)
		}))
		t.Cleanup(server.Close)

		payload := `{"selectedIssueIdsOrKeys":["PROJ-1"],"selectedActions":["summary"],"editedFieldsInput":{"summary":"Updated"}}`
		var buf bytes.Buffer
		runCommandAction(t, issueBulkEditCommand(testCommandClient(server.URL), &buf, format, testAllowWrites()),
			"--payload-json", payload)
		if !bytes.Contains(buf.Bytes(), []byte(`"10642"`)) {
			t.Errorf("output = %q, want taskId 10642 in output", buf.String())
		}
	})

	t.Run("bulk move submits payload", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/bulk/issues/move" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/move")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			if _, ok := body["targetToSourcesMapping"]; !ok {
				t.Error("missing targetToSourcesMapping in body")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			testhelpers.WriteJSONResponse(t, w, `{"taskId":"10643"}`)
		}))
		t.Cleanup(server.Close)

		payload := `{"targetToSourcesMapping":{"PROJ,10001":{"issueIdsOrKeys":["ISSUE-1"]}},"sendBulkNotification":false}`
		var buf bytes.Buffer
		runCommandAction(t, issueBulkMoveCommand(testCommandClient(server.URL), &buf, format, testAllowWrites()),
			"--payload-json", payload)
		if !bytes.Contains(buf.Bytes(), []byte(`"10643"`)) {
			t.Errorf("output = %q, want taskId 10643 in output", buf.String())
		}
	})

	t.Run("bulk transitions lists available transitions", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/bulk/issues/transition" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/transition")
			}
			if got := r.URL.Query().Get("issueIdsOrKeys"); got != "PROJ-1,PROJ-2" {
				t.Errorf("issueIdsOrKeys = %q, want %q", got, "PROJ-1,PROJ-2")
			}
			testhelpers.WriteJSONResponse(t, w, `{"availableTransitions":[{"issues":["PROJ-1"],"transitions":[{"transitionId":11,"transitionName":"Done"}]}]}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueBulkTransitionsCommand(testCommandClient(server.URL), &buf, format),
			"--issues", "PROJ-1,PROJ-2")
		if !bytes.Contains(buf.Bytes(), []byte(`"availableTransitions"`)) {
			t.Errorf("output = %q, want availableTransitions in output", buf.String())
		}
	})

	t.Run("bulk transition submits transition inputs", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/bulk/issues/transition" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/bulk/issues/transition")
			}
			body := testhelpers.DecodeJSONBody(t, r)
			inputs, ok := body["bulkTransitionInputs"].([]any)
			if !ok || len(inputs) != 1 {
				t.Errorf("bulkTransitionInputs = %v, want 1 item", body["bulkTransitionInputs"])
			}
			if send, ok := body["sendBulkNotification"].(bool); !ok || send {
				t.Errorf("sendBulkNotification = %v, want false", body["sendBulkNotification"])
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			testhelpers.WriteJSONResponse(t, w, `{"taskId":"10644"}`)
		}))
		t.Cleanup(server.Close)

		transitionsJSON := `[{"selectedIssueIdsOrKeys":["PROJ-1"],"transitionId":"11"}]`
		var buf bytes.Buffer
		runCommandAction(t, issueBulkTransitionCommand(testCommandClient(server.URL), &buf, format, testAllowWrites()),
			"--transitions-json", transitionsJSON, "--send-notification=false")
		if !bytes.Contains(buf.Bytes(), []byte(`"10644"`)) {
			t.Errorf("output = %q, want taskId 10644 in output", buf.String())
		}
	})

	t.Run("bulk status gets progress", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/bulk/queue/10641" {
				t.Errorf("path = %q, want /bulk/queue/10641", r.URL.Path)
			}
			testhelpers.WriteJSONResponse(t, w, `{"taskId":"10641","status":"COMPLETE","progressPercent":100}`)
		}))
		t.Cleanup(server.Close)

		var buf bytes.Buffer
		runCommandAction(t, issueBulkStatusCommand(testCommandClient(server.URL), &buf, format), "10641")
		if !bytes.Contains(buf.Bytes(), []byte(`"COMPLETE"`)) {
			t.Errorf("output = %q, want COMPLETE status", buf.String())
		}
	})

	t.Run("bulk edit rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueBulkEditCommand(testCommandClient("http://unused"), &buf, format, testAllowWrites())

		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs([]string{"--payload-json", "not-json"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
	})

	t.Run("bulk transition rejects empty array", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		cmd := issueBulkTransitionCommand(testCommandClient("http://unused"), &buf, format, testAllowWrites())

		prepareCommandForTest(cmd)
		cmd.SetContext(context.Background())
		cmd.SetArgs([]string{"--transitions-json", "[]"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for empty transitions, got nil")
		}
		var valErr *apperr.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("error type = %T, want *apperr.ValidationError", err)
		}
	})
}
