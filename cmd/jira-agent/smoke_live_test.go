//go:build smoke_live

package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// smokeEnv reads a required environment variable, skipping the test if unset.
func smokeEnv(t *testing.T, key string) string {
	t.Helper()

	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s not set", key)
	}

	return v
}

// runLiveCLI runs the pre-built CLI binary with the user's real credentials.
// Unlike runBuiltCLI, this does NOT strip JIRA_ env vars or isolate HOME.
func runLiveCLI(t *testing.T, binary string, args ...string) cliResult {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err == nil {
		return cliResult{output: string(out)}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return cliResult{output: string(out), exitCode: exitErr.ExitCode()}
	}

	t.Fatalf("command failed: %v; output = %q", err, out)

	return cliResult{}
}

// assertLiveCommand runs a command against live Jira and asserts exit 0 plus
// a substring match on the output.
func assertLiveCommand(t *testing.T, binary string, args []string, wantSubstr string) {
	t.Helper()

	result := runLiveCLI(t, binary, args...)
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; args=%v output=%q", result.exitCode, args, result.output)
	}
	if !strings.Contains(result.output, wantSubstr) {
		t.Errorf("expected output to contain %q; args=%v output=%q", wantSubstr, args, result.output)
	}
}

// TestSmokeLive_GlobalReadOnly tests read-only commands that need no
// project or issue context.
func TestSmokeLive_GlobalReadOnly(t *testing.T) {
	binary := buildCLIBinary(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"whoami", []string{"whoami"}, `"data"`},
		{"server_info", []string{"server-info"}, `"data"`},
		{"status_list", []string{"status", "list"}, `"data"`},
		{"priority_list", []string{"priority", "list"}, `"data"`},
		{"resolution_list", []string{"resolution", "list"}, `"data"`},
		{"issuetype_list", []string{"issuetype", "list"}, `"data"`},
		{"field_list", []string{"field", "list"}, `"data"`},
		{"label_list", []string{"label", "list"}, `"data"`},
		{"dashboard_list", []string{"dashboard", "list"}, `"data"`},
		{"filter_list", []string{"filter", "list"}, `"data"`},
		{"board_list", []string{"board", "list"}, `"data"`},
		{"permission_list", []string{"permission", "list"}, `"data"`},
		{"workflow_list", []string{"workflow", "list"}, `"data"`},
		{"workflow_statuses", []string{"workflow", "statuses"}, `"data"`},
		{"jql_fields", []string{"jql", "fields"}, `"data"`},
		// time-tracking get/providers require admin (403).
		// role list requires admin (403).
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLiveCommand(t, binary, tt.args, tt.want)
		})
	}
}

// TestSmokeLive_ProjectScoped tests read-only commands that need a project key.
// Set SMOKE_PROJECT to the target project (e.g. RSPEED).
func TestSmokeLive_ProjectScoped(t *testing.T) {
	project := smokeEnv(t, "SMOKE_PROJECT")
	binary := buildCLIBinary(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"project_get", []string{"project", "get", project}, project},
		{"project_list", []string{"project", "list"}, `"data"`},
		{"component_list", []string{"component", "list", "--project", project}, `"data"`},
		{"version_list", []string{"version", "list", "--project", project}, `"data"`},
		// component issue-counts requires a component ID arg, not just --project.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLiveCommand(t, binary, tt.args, tt.want)
		})
	}
}

// TestSmokeLive_IssueScoped tests read-only commands that need an issue key.
// Set SMOKE_ISSUE to a known issue (e.g. RSPEED-2229).
func TestSmokeLive_IssueScoped(t *testing.T) {
	issue := smokeEnv(t, "SMOKE_ISSUE")
	binary := buildCLIBinary(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"issue_get", []string{"issue", "get", issue}, issue},
		{"issue_get_fields", []string{"issue", "get", issue, "--fields", "key,summary,status"}, issue},
		{"issue_changelog", []string{"issue", "changelog", issue}, `"data"`},
		{"issue_comment_list", []string{"issue", "comment", "list", issue}, `"data"`},
		{"issue_link_list", []string{"issue", "link", "list", issue}, `"data"`},
		{"issue_worklog_list", []string{"issue", "worklog", "list", issue}, `"data"`},
		{"issue_watcher_list", []string{"issue", "watcher", "list", issue}, `"data"`},
		{"issue_vote_get", []string{"issue", "vote", "get", issue}, `"data"`},
		{"issue_property_list", []string{"issue", "property", "list", issue}, `"data"`},
		{"issue_remote_link_list", []string{"issue", "remote-link", "list", issue}, `"data"`},
		{"issue_attachment_list", []string{"issue", "attachment", "list", issue}, `"data"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLiveCommand(t, binary, tt.args, tt.want)
		})
	}
}

// TestSmokeLive_JQL tests JQL-based read-only commands.
// Requires both SMOKE_PROJECT and SMOKE_ISSUE.
func TestSmokeLive_JQL(t *testing.T) {
	issue := smokeEnv(t, "SMOKE_ISSUE")
	project := smokeEnv(t, "SMOKE_PROJECT")
	binary := buildCLIBinary(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"issue_search", []string{"issue", "search", "--jql", "key = " + issue, "--fields", "key,summary"}, issue},
		// issue count has a pre-existing bug: sends maxResults=0 which Jira Cloud rejects.
		{"jql_validate", []string{"jql", "validate", "--query", "project = " + project}, `"data"`},
		{"jql_suggest", []string{"jql", "suggest", "--field-name", "status"}, `"data"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLiveCommand(t, binary, tt.args, tt.want)
		})
	}
}

// TestSmokeLive_OutputFormats verifies CSV and TSV output work on a known command.
func TestSmokeLive_OutputFormats(t *testing.T) {
	binary := buildCLIBinary(t)

	t.Run("csv", func(t *testing.T) {
		t.Parallel()
		result := runLiveCLI(t, binary, "--output", "csv", "status", "list")
		if result.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0; output=%q", result.exitCode, result.output)
		}
		if !strings.Contains(result.output, "name") {
			t.Errorf("expected output to contain %q, got %q", "name", result.output)
		}
	})

	t.Run("tsv", func(t *testing.T) {
		t.Parallel()
		result := runLiveCLI(t, binary, "--output", "tsv", "priority", "list")
		if result.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0; output=%q", result.exitCode, result.output)
		}
		if !strings.Contains(result.output, "name") {
			t.Errorf("expected output to contain %q, got %q", "name", result.output)
		}
	})

	t.Run("json_pretty", func(t *testing.T) {
		t.Parallel()
		result := runLiveCLI(t, binary, "--pretty", "whoami")
		if result.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0; output=%q", result.exitCode, result.output)
		}
		// Pretty JSON has indentation.
		if !strings.Contains(result.output, "  ") {
			t.Errorf("expected pretty-printed output with indentation, got %q", result.output)
		}
	})
}
