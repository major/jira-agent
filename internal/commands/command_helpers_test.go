package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func runCobraTestCommand(cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRequireArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		labels  []string
		want    []string
		wantErr string
	}{
		{
			name:   "all positional args present",
			args:   []string{"ISSUE-1", "10000"},
			labels: []string{"issue key", "comment ID"},
			want:   []string{"ISSUE-1", "10000"},
		},
		{
			name:    "reports missing arg label",
			args:    []string{"ISSUE-1"},
			labels:  []string{"issue key", "comment ID"},
			wantErr: "comment ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			var gotErr error
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					got, gotErr = requireArgs(args, tt.labels...)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if tt.wantErr != "" {
				if gotErr == nil || !strings.Contains(gotErr.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", gotErr, tt.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("requireArgs failed: %v", gotErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("requireArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		want        string
		wantErr     string
		wantDetails string
	}{
		{
			name: "returns string flag value",
			args: []string{"--jql", "project = TEST"},
			want: "project = TEST",
		},
		{
			name:    "reports missing flag",
			wantErr: "--jql is required",
		},
		{
			name:        "adds details when provided",
			wantErr:     "--project is required",
			wantDetails: "use --project flag or set JIRA_PROJECT env var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			var gotErr error
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					if tt.wantDetails != "" {
						got, gotErr = requireFlagWithDetails(cmd, "project", tt.wantDetails)
						return nil
					}
					got, gotErr = requireFlag(cmd, "jql")
					return nil
				},
			}
			cmd.Flags().String("jql", "", "")
			cmd.Flags().String("project", "", "")

			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if tt.wantErr != "" {
				if gotErr == nil || !strings.Contains(gotErr.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", gotErr, tt.wantErr)
				}
				var validationErr *apperr.ValidationError
				if !errors.As(gotErr, &validationErr) {
					t.Fatalf("error type = %T, want *ValidationError", gotErr)
				}
				if validationErr.Details() != tt.wantDetails {
					t.Errorf("Details() = %q, want %q", validationErr.Details(), tt.wantDetails)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("requireFlag() error = %v, want nil", gotErr)
			}
			if got != tt.want {
				t.Errorf("requireFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteAPIResult(t *testing.T) {
	t.Parallel()

	t.Run("writes successful result", func(t *testing.T) {
		t.Parallel()

		var buf strings.Builder
		format := output.FormatJSON
		err := writeAPIResult(&buf, format, func(result any) error {
			ptr, ok := result.(*any)
			if !ok {
				t.Fatalf("result type = %T, want *any", result)
			}
			*ptr = map[string]any{"key": "TEST-1"}
			return nil
		})
		if err != nil {
			t.Fatalf("writeAPIResult() error = %v, want nil", err)
		}
		if !strings.Contains(buf.String(), `"key":"TEST-1"`) {
			t.Errorf("output = %q, want issue key", buf.String())
		}
	})

	t.Run("returns request error", func(t *testing.T) {
		t.Parallel()

		wantErr := fmt.Errorf("request failed")
		var buf strings.Builder
		format := output.FormatJSON
		err := writeAPIResult(&buf, format, func(_ any) error {
			return wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("writeAPIResult() error = %v, want %v", err, wantErr)
		}
		if buf.String() != "" {
			t.Errorf("output = %q, want empty", buf.String())
		}
	})
}

func TestWritePaginatedAPIResult(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	format := output.FormatJSON
	cmd := &cobra.Command{Use: "list"}
	err := writePaginatedAPIResult(cmd, &buf, format, func(result any) error {
		ptr, ok := result.(*any)
		if !ok {
			t.Fatalf("result type = %T, want *any", result)
		}
		*ptr = map[string]any{
			"total":      float64(2),
			"startAt":    float64(0),
			"maxResults": float64(50),
			"values": []any{
				map[string]any{"id": "1"},
				map[string]any{"id": "2"},
			},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("writePaginatedAPIResult() error = %v, want nil", err)
	}
	for _, want := range []string{`"total":2`, `"returned":2`, `"max_results":50`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("output = %q, want %s", buf.String(), want)
		}
	}
}

func TestPaginationHasMore(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"total":      float64(3),
		"startAt":    float64(0),
		"maxResults": float64(2),
		"values": []any{
			map[string]any{"id": "1"},
			map[string]any{"id": "2"},
		},
	}

	meta := extractPaginationMeta(testPaginationCommand(t, nil), result)

	if !meta.HasMore {
		t.Error("HasMore = false, want true")
	}
}

func TestPaginationLastPage(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"total":      float64(4),
		"startAt":    float64(2),
		"maxResults": float64(2),
		"values": []any{
			map[string]any{"id": "3"},
			map[string]any{"id": "4"},
		},
	}

	meta := extractPaginationMeta(testPaginationCommand(t, nil), result)

	if meta.HasMore {
		t.Error("HasMore = true, want false")
	}
	if meta.NextCommand != "" {
		t.Errorf("NextCommand = %q, want empty", meta.NextCommand)
	}

	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal Metadata error = %v, want nil", err)
	}
	if !strings.Contains(string(encoded), `"has_more":false`) {
		t.Errorf("metadata JSON = %s, want explicit has_more false", encoded)
	}
}

func TestPaginationNextCommand(t *testing.T) {
	t.Parallel()

	cmd := testPaginationCommand(t, []string{
		"--jql", "project = TEST",
		"--fields", "key,summary",
		"--max-results", "25",
		"--start-at", "25",
	})
	result := map[string]any{
		"total":      float64(100),
		"startAt":    float64(25),
		"maxResults": float64(25),
		"issues":     []any{map[string]any{"key": "TEST-1"}},
	}

	meta := extractPaginationMeta(cmd, result)

	want := `jira-agent issue search --fields key,summary --jql "project = TEST" --max-results 25 --start-at 26`
	if meta.NextCommand != want {
		t.Errorf("NextCommand = %q, want %q", meta.NextCommand, want)
	}
}

func TestPaginationNextCommandWithPositionalArgs(t *testing.T) {
	t.Parallel()

	// Simulate "jira-agent issue comment list PROJ-123 --max-results 5"
	root := &cobra.Command{Use: "jira-agent"}
	issue := &cobra.Command{Use: "issue"}
	comment := &cobra.Command{Use: "comment"}
	list := &cobra.Command{
		Use: "list <issue-key>",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	appendPaginationFlags(list)
	comment.AddCommand(list)
	issue.AddCommand(comment)
	root.AddCommand(issue)

	root.SetArgs([]string{"issue", "comment", "list", "PROJ-123", "--max-results", "5"})
	if err := root.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	result := map[string]any{
		"total":      float64(10),
		"startAt":    float64(0),
		"maxResults": float64(5),
		"comments":   []any{map[string]any{"id": "1"}},
	}

	meta := extractPaginationMeta(list, result)

	if !meta.HasMore {
		t.Error("HasMore = false, want true")
	}

	want := "jira-agent issue comment list PROJ-123 --max-results 5 --start-at 1"
	if meta.NextCommand != want {
		t.Errorf("NextCommand = %q, want %q", meta.NextCommand, want)
	}
}

func testPaginationCommand(t *testing.T, args []string) *cobra.Command {
	t.Helper()

	root := &cobra.Command{Use: "jira-agent"}
	issue := &cobra.Command{Use: "issue"}
	search := &cobra.Command{
		Use: "search",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	search.Flags().String("jql", "", "")
	search.Flags().String("fields", "", "")
	appendPaginationFlags(search)
	issue.AddCommand(search)
	root.AddCommand(issue)

	if args != nil {
		root.SetArgs(append([]string{"issue", "search"}, args...))
		if err := root.Execute(); err != nil {
			t.Fatalf("command failed: %v", err)
		}
	}

	return search
}

func TestBuildPaginationParams(t *testing.T) {
	t.Parallel()

	var got map[string]string
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			got = buildPaginationParams(cmd, map[string]string{
				"query":    "query",
				"order-by": "orderBy",
			})
			return nil
		},
	}
	cmd.Flags().Int("max-results", 25, "")
	cmd.Flags().Int("start-at", 10, "")
	cmd.Flags().String("query", "", "")
	cmd.Flags().String("order-by", "", "")

	err := runCobraTestCommand(cmd, []string{"--query", "agent", "--order-by", "name"})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	want := map[string]string{
		"maxResults": "25",
		"startAt":    "10",
		"query":      "agent",
		"orderBy":    "name",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildPaginationParams() = %v, want %v", got, want)
	}
}

func TestBuildMaxResultsParams(t *testing.T) {
	t.Parallel()

	var got map[string]string
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			got = buildMaxResultsParams(cmd, map[string]string{"query": "query"})
			return nil
		},
	}
	cmd.Flags().Int("max-results", 25, "")
	cmd.Flags().String("query", "", "")

	err := runCobraTestCommand(cmd, []string{"--query", "agent"})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	want := map[string]string{
		"maxResults": "25",
		"query":      "agent",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildMaxResultsParams() = %v, want %v", got, want)
	}
}

func TestAddOptionalParams(t *testing.T) {
	t.Parallel()

	var got map[string]string
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			got = map[string]string{"existing": "value"}
			addOptionalParams(cmd, got, map[string]string{
				"query":  "queryString",
				"expand": "expand",
			})
			return nil
		},
	}
	cmd.Flags().String("query", "", "")
	cmd.Flags().String("expand", "", "")

	if err := runCobraTestCommand(cmd, []string{"--query", "agent"}); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	want := map[string]string{
		"existing":    "value",
		"queryString": "agent",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("params = %v, want %v", got, want)
	}
}

func TestAddBoolParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "adds true bool flag",
			args: []string{"--case-insensitive"},
			want: map[string]string{"caseInsensitive": "true"},
		},
		{
			name: "omits false bool flag",
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := map[string]string{}
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					addBoolParam(cmd, got, "case-insensitive", "caseInsensitive")
					return nil
				},
			}
			cmd.Flags().Bool("case-insensitive", false, "")

			if err := runCobraTestCommand(cmd, tt.args); err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("params = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitTrimmedDropsEmptyValues(t *testing.T) {
	t.Parallel()

	got := splitTrimmed(" alpha, , beta ,gamma ")
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitTrimmed() = %v, want %v", got, want)
	}
}

func TestNormalizeCSV(t *testing.T) {
	t.Parallel()

	got := normalizeCSV(" BROWSE_PROJECTS, ,ADMINISTER_PROJECTS, ")
	want := "BROWSE_PROJECTS,ADMINISTER_PROJECTS"
	if got != want {
		t.Errorf("normalizeCSV() = %q, want %q", got, want)
	}
}

func TestRequireVisibilityFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{
			name: "both unset",
		},
		{
			name: "both set",
			args: []string{"--visibility-type", "group", "--visibility-value", "team"},
			want: "team",
		},
		{
			name:    "missing second flag",
			args:    []string{"--visibility-type", "group"},
			wantErr: "--visibility-value is required when --visibility-type is set",
		},
		{
			name:    "missing first flag",
			args:    []string{"--visibility-value", "team"},
			wantErr: "--visibility-type is required when --visibility-value is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			var gotErr error
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					_, value, err := requireVisibilityFlags(cmd)
					got = value
					gotErr = err
					return nil
				},
			}
			cmd.Flags().String("visibility-type", "", "")
			cmd.Flags().String("visibility-value", "", "")

			if err := runCobraTestCommand(cmd, tt.args); err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if tt.wantErr != "" {
				if gotErr == nil || !strings.Contains(gotErr.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", gotErr, tt.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("requireVisibilityFlags failed: %v", gotErr)
			}
			if got != tt.want {
				t.Errorf("paired flag value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequireWriteAccess(t *testing.T) {
	t.Parallel()

	t.Run("blocks when nil", func(t *testing.T) {
		t.Parallel()

		err := requireWriteAccess(nil)
		if err == nil {
			t.Fatal("requireWriteAccess(nil) = nil, want error")
		}
		var ve *apperr.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("error type = %T, want *ValidationError", err)
		}
		if !strings.Contains(err.Error(), "write operations are not enabled") {
			t.Errorf("error = %q, want 'write operations are not enabled'", err.Error())
		}
	})

	t.Run("blocks when false", func(t *testing.T) {
		t.Parallel()

		allow := false
		err := requireWriteAccess(&allow)
		if err == nil {
			t.Fatal("requireWriteAccess(&false) = nil, want error")
		}
		var ve *apperr.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("error type = %T, want *ValidationError", err)
		}
		details := ve.Details()
		if !strings.Contains(details, "i-too-like-to-live-dangerously") {
			t.Errorf("details = %q, want config key hint", details)
		}
		if !strings.Contains(details, "JIRA_ALLOW_WRITES") {
			t.Errorf("details = %q, want env var hint", details)
		}
	})

	t.Run("allows when true", func(t *testing.T) {
		t.Parallel()

		allow := true
		if err := requireWriteAccess(&allow); err != nil {
			t.Fatalf("requireWriteAccess(&true) = %v, want nil", err)
		}
	})
}

func TestWriteGuard(t *testing.T) {
	t.Parallel()

	t.Run("blocks action when writes disabled", func(t *testing.T) {
		t.Parallel()

		called := false
		allow := false
		wrapped := writeGuard(&allow, func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		})
		cmd := &cobra.Command{
			Use:  "test",
			RunE: wrapped,
		}
		err := runCobraTestCommand(cmd, nil)
		if err == nil {
			t.Fatal("writeGuard(false) = nil, want error")
		}
		if called {
			t.Error("inner action was called, want it blocked")
		}
	})

	t.Run("runs action when writes enabled", func(t *testing.T) {
		t.Parallel()

		called := false
		allow := true
		wrapped := writeGuard(&allow, func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		})
		cmd := &cobra.Command{
			Use:  "test",
			RunE: wrapped,
		}
		if err := runCobraTestCommand(cmd, nil); err != nil {
			t.Fatalf("writeGuard(true) = %v, want nil", err)
		}
		if !called {
			t.Error("inner action was not called, want it called")
		}
	})
}

func TestCommandAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("keeps llm routing annotation keys stable", func(t *testing.T) {
		t.Parallel()

		if commandAnnotationCategory != "jira-agent/category" {
			t.Errorf("commandAnnotationCategory = %q, want jira-agent/category", commandAnnotationCategory)
		}
		if commandAnnotationRequiresAuth != "jira-agent/requires-auth" {
			t.Errorf("commandAnnotationRequiresAuth = %q, want jira-agent/requires-auth", commandAnnotationRequiresAuth)
		}
		if commandRequiresAuthTrue != "true" || commandRequiresAuthFalse != "false" {
			t.Errorf("requires-auth values = %q/%q, want true/false", commandRequiresAuthTrue, commandRequiresAuthFalse)
		}
	})

	t.Run("records default subcommand", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "parent"}
		setDefaultSubcommand(cmd, "list")

		got := cmd.Annotations[commandAnnotationDefaultSubcommand]
		if got != "list" {
			t.Errorf("default subcommand annotation = %q, want list", got)
		}
		if cmd.RunE == nil {
			t.Error("RunE was not set, want default subcommand handler")
		}
	})

	t.Run("records write protection", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "write"}
		MarkWriteProtected(cmd)

		got := cmd.Annotations[commandAnnotationWriteProtected]
		if got != "true" {
			t.Errorf("write protection annotation = %q, want true", got)
		}
	})

	t.Run("records write protection from command tree", func(t *testing.T) {
		t.Parallel()

		root := &cobra.Command{Use: "jira-agent"}
		issue := &cobra.Command{Use: "issue"}
		create := &cobra.Command{Use: "create"}
		get := &cobra.Command{Use: "get"}
		issue.AddCommand(create, get)
		root.AddCommand(issue)

		MarkWriteProtectedCommands(root)

		got := create.Annotations[commandAnnotationWriteProtected]
		if got != "true" {
			t.Errorf("write protection annotation = %q, want true", got)
		}
		if _, ok := get.Annotations[commandAnnotationWriteProtected]; ok {
			t.Error("read command was marked write-protected")
		}
	})
}

func TestSetCommandCategory(t *testing.T) {
	t.Parallel()

	t.Run("records valid category", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "search"}
		SetCommandCategory(cmd, "read")

		got := cmd.Annotations[commandAnnotationCategory]
		if got != "read" {
			t.Errorf("category annotation = %q, want read", got)
		}
	})

	t.Run("panics for invalid category", func(t *testing.T) {
		t.Parallel()

		defer func() {
			panicValue := recover()
			if panicValue == nil {
				t.Fatal("SetCommandCategory accepted invalid category, want panic")
			}
			if !strings.Contains(fmt.Sprint(panicValue), "invalid command category") {
				t.Fatalf("panic = %v, want invalid command category", panicValue)
			}
		}()

		SetCommandCategory(&cobra.Command{Use: "bad"}, "unsafe")
	})
}

func TestCommandCategories(t *testing.T) {
	t.Parallel()

	got := CommandCategories()
	for _, category := range []string{"read", "write", "bulk", "discovery", "workflow", "admin"} {
		paths, ok := got[category]
		if !ok {
			t.Fatalf("CommandCategories()[%q] missing", category)
		}
		if !sort.StringsAreSorted(paths) {
			t.Errorf("CommandCategories()[%q] = %v, want sorted paths", category, paths)
		}
	}

	wantSamples := map[string][]string{
		"read":      {"issue get", "issue search"},
		"write":     {"issue create", "issue transition"},
		"bulk":      {"issue bulk create", "issue bulk transition"},
		"discovery": {"field list", "project list", "resolve", "resolve board"},
		"workflow":  {"workflow list", "status list"},
		"admin":     {"audit list", "server-info"},
	}
	for category, paths := range wantSamples {
		for _, path := range paths {
			if !slices.Contains(got[category], path) {
				t.Errorf("CommandCategories()[%q] = %v, want %q", category, got[category], path)
			}
		}
	}
}

func TestAllCommandsHaveCategory(t *testing.T) {
	t.Parallel()

	allowWrites := false
	dryRun := false
	format := output.FormatJSON
	apiClient := &client.Ref{}
	var w strings.Builder
	root := &cobra.Command{Use: "jira-agent"}
	root.AddCommand(
		AuditCommand(apiClient, &w, &format),
		IssueCommand(apiClient, &w, &format, &allowWrites, &dryRun),
		FieldCommand(apiClient, &w, &format, &allowWrites),
		ProjectCommand(apiClient, &w, &format, &allowWrites),
		RoleCommand(apiClient, &w, &format),
		BoardCommand(apiClient, &w, &format, &allowWrites),
		UserCommand(apiClient, &w, &format),
		GroupCommand(apiClient, &w, &format, &allowWrites),
		FilterCommand(apiClient, &w, &format, &allowWrites),
		PermissionCommand(apiClient, &w, &format),
		DashboardCommand(apiClient, &w, &format, &allowWrites),
		WorkflowCommand(apiClient, &w, &format),
		StatusCommand(apiClient, &w, &format),
		PriorityCommand(apiClient, &w, &format),
		ResolutionCommand(apiClient, &w, &format),
		IssueTypeCommand(apiClient, &w, &format),
		LabelCommand(apiClient, &w, &format),
		ComponentCommand(apiClient, &w, &format, &allowWrites),
		VersionCommand(apiClient, &w, &format, &allowWrites),
		SprintCommand(apiClient, &w, &format, &allowWrites),
		EpicCommand(apiClient, &w, &format, &allowWrites),
		BacklogCommand(apiClient, &w, &format, &allowWrites),
		TaskCommand(apiClient, &w, &format, &allowWrites),
		TimeTrackingCommand(apiClient, &w, &format, &allowWrites),
		ServerInfoCommand(apiClient, &w, &format),
		JQLCommand(apiClient, &w, &format),
	)

	MarkWriteProtectedCommands(root)

	for _, cmd := range allCommands(root) {
		if cmd == root {
			continue
		}
		if got := cmd.Annotations[commandAnnotationCategory]; got == "" {
			t.Errorf("%q category annotation missing", cmd.CommandPath())
		}
	}
}

func TestAppendQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		params map[string]string
		want   string
	}{
		{
			name: "sorts and encodes populated query params",
			path: "/issue/KEY/comment/1",
			params: map[string]string{
				"expand":      "renderedBody",
				"notifyUsers": "false",
			},
			want: "/issue/KEY/comment/1?expand=renderedBody&notifyUsers=false",
		},
		{
			name:   "leaves path unchanged with no params",
			path:   "/issue/KEY/comment/1",
			params: map[string]string{},
			want:   "/issue/KEY/comment/1",
		},
		{
			name: "omits empty params",
			path: "/issue/KEY/comment/1",
			params: map[string]string{
				"expand": "",
			},
			want: "/issue/KEY/comment/1",
		},
		{
			name: "escapes query param values",
			path: "/issue/KEY/comment/1",
			params: map[string]string{
				"jql": "project = TEST",
			},
			want: "/issue/KEY/comment/1?jql=project+%3D+TEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := appendQueryParams(tt.path, tt.params)
			if got != tt.want {
				t.Errorf("appendQueryParams() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapePathSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "leaves simple issue key unchanged",
			value: "PROJ-123",
			want:  "PROJ-123",
		},
		{
			name:  "escapes slash so it cannot create path segments",
			value: "parent/child",
			want:  "parent%2Fchild",
		},
		{
			name:  "escapes spaces and query punctuation",
			value: "team name?expand=all",
			want:  "team%20name%3Fexpand=all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := escapePathSegment(tt.value)
			if got != tt.want {
				t.Errorf("escapePathSegment(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestParsePositiveIntID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		label   string
		want    int64
		wantErr string
	}{
		{
			name:  "parses positive integer",
			value: "42",
			label: "board ID",
			want:  42,
		},
		{
			name:    "rejects zero",
			value:   "0",
			label:   "board ID",
			wantErr: "board ID must be a positive integer",
		},
		{
			name:    "rejects negative integer",
			value:   "-1",
			label:   "filter ID",
			wantErr: "filter ID must be a positive integer",
		},
		{
			name:    "rejects non integer",
			value:   "abc",
			label:   "sprint ID",
			wantErr: "sprint ID must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePositiveIntID(tt.value, tt.label)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parsePositiveIntID() error = %v, want it to contain %q", err, tt.wantErr)
				}
				var validationErr *apperr.ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("error type = %T, want *ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePositiveIntID() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("parsePositiveIntID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseBoardID(t *testing.T) {
	t.Parallel()

	got, err := parseBoardID("10000")
	if err != nil {
		t.Fatalf("parseBoardID() error = %v, want nil", err)
	}
	if got != 10000 {
		t.Errorf("parseBoardID() = %d, want %d", got, int64(10000))
	}
}

func TestParseBoardIDNudge(t *testing.T) {
	t.Parallel()

	_, err := parseBoardID("my-board")
	if err == nil {
		t.Fatalf("parseBoardID() error = nil, want ValidationError")
	}

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}

	wantNextCommand := "jira-agent resolve board <name>"
	if validationErr.NextCommand() != wantNextCommand {
		t.Errorf("NextCommand() = %q, want %q", validationErr.NextCommand(), wantNextCommand)
	}
}

func TestParseSprintIDNudge(t *testing.T) {
	t.Parallel()

	_, err := parseSprintID("my-sprint")
	if err == nil {
		t.Fatalf("parseSprintID() error = nil, want ValidationError")
	}

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}

	wantNextCommand := "jira-agent resolve sprint --board-id <board-id> <name>"
	if validationErr.NextCommand() != wantNextCommand {
		t.Errorf("NextCommand() = %q, want %q", validationErr.NextCommand(), wantNextCommand)
	}
}

func TestParseFilterShareShorthand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    map[string]any
		wantErr string
	}{
		{
			name:  "global share",
			value: "global",
			want:  map[string]any{"type": "global"},
		},
		{
			name:  "authenticated share",
			value: "authenticated",
			want:  map[string]any{"type": "authenticated"},
		},
		{
			name:  "user share",
			value: "user:abc123",
			want:  map[string]any{"type": "user", "accountId": "abc123"},
		},
		{
			name:  "group ID share",
			value: "group:group-id",
			want:  map[string]any{"type": "group", "groupId": "group-id"},
		},
		{
			name:  "group name share",
			value: "groupname:jira-users",
			want:  map[string]any{"type": "group", "groupname": "jira-users"},
		},
		{
			name:  "project share",
			value: "project:10001",
			want:  map[string]any{"type": "project", "projectId": "10001"},
		},
		{
			name:  "project role share",
			value: "project-role:10001:10002",
			want: map[string]any{
				"type":          "projectRole",
				"projectId":     "10001",
				"projectRoleId": "10002",
			},
		},
		{
			name:    "empty user share",
			value:   "user:",
			wantErr: "user share shorthand requires an account ID",
		},
		{
			name:    "malformed project role share",
			value:   "project-role:10001",
			wantErr: "project-role share shorthand requires project and role IDs",
		},
		{
			name:    "unsupported share",
			value:   "team:blue",
			wantErr: "unsupported --with value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFilterShareShorthand(tt.value)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseFilterShareShorthand() error = %v, want it to contain %q", err, tt.wantErr)
				}
				var validationErr *apperr.ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("error type = %T, want *ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFilterShareShorthand() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseFilterShareShorthand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseWorklogProperties(t *testing.T) {
	t.Parallel()

	t.Run("parses JSON array", func(t *testing.T) {
		t.Parallel()

		got, err := parseWorklogProperties(`[{"key":"tempo","value":"billable"}]`)
		if err != nil {
			t.Fatalf("parseWorklogProperties() error = %v, want nil", err)
		}
		want := []any{map[string]any{"key": "tempo", "value": "billable"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseWorklogProperties() = %v, want %v", got, want)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		_, err := parseWorklogProperties(`{"key":"tempo"}`)
		if err == nil {
			t.Fatal("parseWorklogProperties() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "invalid --properties-json") {
			t.Errorf("parseWorklogProperties() error = %q, want invalid properties-json", err.Error())
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("error type = %T, want *ValidationError", err)
		}
	})
}

func TestExtractFieldArray(t *testing.T) {
	t.Parallel()

	t.Run("returns named field array", func(t *testing.T) {
		t.Parallel()

		result := map[string]any{
			"fields": map[string]any{
				"attachment": []any{map[string]any{"id": "10000"}},
			},
		}
		got, err := extractFieldArray(result, "attachment")
		if err != nil {
			t.Fatalf("extractFieldArray() error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Errorf("len(extractFieldArray()) = %d, want %d", len(got), 1)
		}
	})

	t.Run("requires fields object", func(t *testing.T) {
		t.Parallel()

		_, err := extractFieldArray(map[string]any{}, "attachment")
		if err == nil {
			t.Fatal("extractFieldArray() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "missing 'fields' object") {
			t.Errorf("extractFieldArray() error = %q, want missing fields object", err.Error())
		}
	})

	t.Run("requires named array", func(t *testing.T) {
		t.Parallel()

		result := map[string]any{"fields": map[string]any{}}
		_, err := extractFieldArray(result, "issuelinks")
		if err == nil {
			t.Fatal("extractFieldArray() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "missing 'issuelinks' array") {
			t.Errorf("extractFieldArray() error = %q, want missing issuelinks array", err.Error())
		}
	})
}

func TestTransitionFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	apiClient := &client.Ref{}
	var buf strings.Builder
	format := output.FormatJSON
	allowWrites := true

	t.Run("rejects both --to and --transition-id", func(t *testing.T) {
		t.Parallel()

		cmd := issueTransitionCommand(apiClient, &buf, &format, &allowWrites)
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"PROJ-123", "--to", "Done", "--transition-id", "42"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when both --to and --transition-id are set, got nil")
		}
		if !strings.Contains(err.Error(), "if any flags in the group") {
			t.Errorf("error = %q, want Cobra mutual exclusion error", err.Error())
		}
	})

	t.Run("rejects --payload-json with --summary on create", func(t *testing.T) {
		t.Parallel()

		cmd := issueCreateCommand(apiClient, &buf, &format, &allowWrites)
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"--payload-json", `{"fields":{}}`, "--summary", "test"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when both --payload-json and --summary are set, got nil")
		}
		if !strings.Contains(err.Error(), "if any flags in the group") {
			t.Errorf("error = %q, want Cobra mutual exclusion error", err.Error())
		}
	})

	t.Run("rejects --payload-json with --type on create", func(t *testing.T) {
		t.Parallel()

		cmd := issueCreateCommand(apiClient, &buf, &format, &allowWrites)
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"--payload-json", `{"fields":{}}`, "--type", "Bug"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when both --payload-json and --type are set, got nil")
		}
		if !strings.Contains(err.Error(), "if any flags in the group") {
			t.Errorf("error = %q, want Cobra mutual exclusion error", err.Error())
		}
	})

	t.Run("rejects --type and --type-id on link add", func(t *testing.T) {
		t.Parallel()

		cmd := issueLinkAddCommand(apiClient, &buf, &format, &allowWrites)
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{
			"--type", "Blocks", "--type-id", "10001",
			"--inward", "PROJ-1", "--outward", "PROJ-2",
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when both --type and --type-id are set, got nil")
		}
		if !strings.Contains(err.Error(), "if any flags in the group") {
			t.Errorf("error = %q, want Cobra mutual exclusion error", err.Error())
		}
	})
}

func TestAssignValidation(t *testing.T) {
	t.Parallel()

	apiClient := &client.Ref{}
	var buf strings.Builder
	format := output.FormatJSON
	allowWrites := true

	cmd := issueAssignCommand(apiClient, &buf, &format, &allowWrites)

	// Assign uses custom positional arg + switch validation, not Cobra flag groups.
	groups := FlagGroups(cmd)
	if len(groups) != 0 {
		t.Errorf("FlagGroups() = %v, want empty (assign uses custom validation)", groups)
	}
}

func TestValidFlagCombinations(t *testing.T) {
	t.Parallel()

	// Verify valid flag combinations pass Cobra's flag group validation.
	// Each subtest builds a minimal command with the same flags and groups
	// as the real command, but with a no-op RunE to avoid API calls.
	tests := []struct {
		name  string
		setup func() *cobra.Command
		args  []string
	}{
		{
			name: "transition with only --to",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "transition", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("to", "", "")
				cmd.Flags().String("transition-id", "", "")
				markMutuallyExclusive(cmd, "to", "transition-id")
				return cmd
			},
			args: []string{"--to", "Done"},
		},
		{
			name: "transition with only --transition-id",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "transition", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("to", "", "")
				cmd.Flags().String("transition-id", "", "")
				markMutuallyExclusive(cmd, "to", "transition-id")
				return cmd
			},
			args: []string{"--transition-id", "42"},
		},
		{
			name: "transition with neither exclusive flag",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "transition", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("to", "", "")
				cmd.Flags().String("transition-id", "", "")
				cmd.Flags().Bool("list", false, "")
				markMutuallyExclusive(cmd, "to", "transition-id")
				return cmd
			},
			args: []string{"--list"},
		},
		{
			name: "link add with only --type",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "add", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("type", "", "")
				cmd.Flags().String("type-id", "", "")
				markMutuallyExclusive(cmd, "type", "type-id")
				return cmd
			},
			args: []string{"--type", "Blocks"},
		},
		{
			name: "link add with only --type-id",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "add", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("type", "", "")
				cmd.Flags().String("type-id", "", "")
				markMutuallyExclusive(cmd, "type", "type-id")
				return cmd
			},
			args: []string{"--type-id", "10001"},
		},
		{
			name: "create with individual flags",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "create", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("type", "", "")
				cmd.Flags().String("summary", "", "")
				cmd.Flags().String("payload-json", "", "")
				markMutuallyExclusive(cmd, "payload-json", "type")
				markMutuallyExclusive(cmd, "payload-json", "summary")
				return cmd
			},
			args: []string{"--type", "Story", "--summary", "Test"},
		},
		{
			name: "create with only --payload-json",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "create", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
				cmd.Flags().String("type", "", "")
				cmd.Flags().String("summary", "", "")
				cmd.Flags().String("payload-json", "", "")
				markMutuallyExclusive(cmd, "payload-json", "type")
				markMutuallyExclusive(cmd, "payload-json", "summary")
				return cmd
			},
			args: []string{"--payload-json", `{"fields":{"summary":"test"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.setup()
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Errorf("valid flag combination rejected: %v", err)
			}
		})
	}
}

func TestFlagGroupsHelper(t *testing.T) {
	t.Parallel()

	t.Run("returns groups from annotated command", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("flag-a", "", "")
		cmd.Flags().String("flag-b", "", "")
		cmd.Flags().String("flag-c", "", "")

		markMutuallyExclusive(cmd, "flag-a", "flag-b")
		markMutuallyExclusive(cmd, "flag-a", "flag-c")

		groups := FlagGroups(cmd)
		if len(groups) != 2 {
			t.Fatalf("FlagGroups() returned %d groups, want 2", len(groups))
		}
		if groups[0].Type != flagGroupTypeMutuallyExclusive {
			t.Errorf("groups[0].Type = %q, want %q", groups[0].Type, flagGroupTypeMutuallyExclusive)
		}
		wantFlags := []string{"flag-a", "flag-b"}
		if !reflect.DeepEqual(groups[0].Flags, wantFlags) {
			t.Errorf("groups[0].Flags = %v, want %v", groups[0].Flags, wantFlags)
		}
	})

	t.Run("returns nil for command without groups", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("flag-a", "", "")

		groups := FlagGroups(cmd)
		if groups != nil {
			t.Errorf("FlagGroups() = %v, want nil", groups)
		}
	})

	t.Run("returns groups from real transition command", func(t *testing.T) {
		t.Parallel()

		apiClient := &client.Ref{}
		var buf strings.Builder
		format := output.FormatJSON
		allowWrites := true

		cmd := issueTransitionCommand(apiClient, &buf, &format, &allowWrites)
		groups := FlagGroups(cmd)

		if len(groups) == 0 {
			t.Fatal("FlagGroups() returned no groups for transition command")
		}
		found := false
		for _, g := range groups {
			if g.Type == flagGroupTypeMutuallyExclusive &&
				slices.Contains(g.Flags, "to") &&
				slices.Contains(g.Flags, "transition-id") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("FlagGroups() = %v, want group containing [to, transition-id]", groups)
		}
	})

	t.Run("returns groups from real create command", func(t *testing.T) {
		t.Parallel()

		apiClient := &client.Ref{}
		var buf strings.Builder
		format := output.FormatJSON
		allowWrites := true

		cmd := issueCreateCommand(apiClient, &buf, &format, &allowWrites)
		groups := FlagGroups(cmd)

		// Should have mutually_exclusive groups for payload-json vs each
		// individual field flag.
		excludedFlags := []string{
			"summary", "type", "description", "assignee",
			"priority", "labels", "components", "parent",
		}
		for _, flag := range excludedFlags {
			found := false
			for _, g := range groups {
				if g.Type == flagGroupTypeMutuallyExclusive &&
					slices.Contains(g.Flags, "payload-json") &&
					slices.Contains(g.Flags, flag) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("FlagGroups() missing mutually_exclusive group for [payload-json, %s]", flag)
			}
		}
	})

	t.Run("serializes to stable JSON", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("flag-x", "", "")
		cmd.Flags().String("flag-y", "", "")

		markMutuallyExclusive(cmd, "flag-x", "flag-y")

		groups := FlagGroups(cmd)
		data, err := json.Marshal(groups)
		if err != nil {
			t.Fatalf("json.Marshal(FlagGroups) error = %v", err)
		}
		want := `[{"type":"mutually_exclusive","flags":["flag-x","flag-y"]}]`
		if string(data) != want {
			t.Errorf("JSON = %s, want %s", data, want)
		}
	})
}
