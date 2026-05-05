package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

func TestBuildJQLFromFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		assignee     string
		status       string
		issueType    string
		priority     string
		sprint       string
		updatedSince string
		labels       []string
		want         string
	}{
		{
			name: "no flags returns empty",
			want: "",
		},
		{
			name:     "assignee me uses currentUser()",
			assignee: "me",
			want:     "assignee = currentUser() ORDER BY updated DESC",
		},
		{
			name:     "assignee name is quoted",
			assignee: "john",
			want:     `assignee = "john" ORDER BY updated DESC`,
		},
		{
			name:   "status is quoted",
			status: "In Progress",
			want:   `status = "In Progress" ORDER BY updated DESC`,
		},
		{
			name:      "type maps to issuetype",
			issueType: "Bug",
			want:      `issuetype = "Bug" ORDER BY updated DESC`,
		},
		{
			name:     "priority is quoted",
			priority: "High",
			want:     `priority = "High" ORDER BY updated DESC`,
		},
		{
			name:   "sprint current uses openSprints()",
			sprint: "current",
			want:   "sprint in openSprints() ORDER BY updated DESC",
		},
		{
			name:   "sprint name is quoted",
			sprint: "Sprint 42",
			want:   `sprint = "Sprint 42" ORDER BY updated DESC`,
		},
		{
			name:         "updatedSince adds relative date",
			updatedSince: "7d",
			want:         `updated >= "-7d" ORDER BY updated DESC`,
		},
		{
			name:   "single label",
			labels: []string{"backend"},
			want:   `labels = "backend" ORDER BY updated DESC`,
		},
		{
			name:   "multiple labels become AND conditions",
			labels: []string{"backend", "urgent"},
			want:   `labels = "backend" AND labels = "urgent" ORDER BY updated DESC`,
		},
		{
			name:     "multiple flags in alphabetical order",
			assignee: "me",
			status:   "In Progress",
			priority: "High",
			want:     `assignee = currentUser() AND priority = "High" AND status = "In Progress" ORDER BY updated DESC`,
		},
		{
			name:         "all flags combined",
			assignee:     "me",
			status:       "Done",
			issueType:    "Story",
			priority:     "Medium",
			sprint:       "current",
			updatedSince: "14d",
			labels:       []string{"frontend"},
			want:         `assignee = currentUser() AND issuetype = "Story" AND labels = "frontend" AND priority = "Medium" AND sprint in openSprints() AND status = "Done" AND updated >= "-14d" ORDER BY updated DESC`,
		},
		{
			name:   "empty labels slice returns empty",
			labels: []string{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildJQLFromFlags(tt.assignee, tt.status, tt.issueType, tt.priority, tt.sprint, tt.updatedSince, tt.labels)
			if got != tt.want {
				t.Errorf("buildJQLFromFlags() =\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

func TestSemanticJQLFlags_IntegrationWithSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantJQL string
	}{
		{
			name:    "assignee me builds currentUser JQL",
			args:    []string{"--assignee", "me"},
			wantJQL: "assignee = currentUser() ORDER BY updated DESC",
		},
		{
			name:    "status and priority combined",
			args:    []string{"--status", "In Progress", "--priority", "High"},
			wantJQL: `priority = "High" AND status = "In Progress" ORDER BY updated DESC`,
		},
		{
			name:    "sprint current builds openSprints JQL",
			args:    []string{"--sprint", "current"},
			wantJQL: "sprint in openSprints() ORDER BY updated DESC",
		},
		{
			name:    "type flag maps to issuetype",
			args:    []string{"--type", "Bug"},
			wantJQL: `issuetype = "Bug" ORDER BY updated DESC`,
		},
		{
			name:    "label flag",
			args:    []string{"--label", "backend"},
			wantJQL: `labels = "backend" ORDER BY updated DESC`,
		},
		{
			name:    "multiple labels via repeated flag",
			args:    []string{"--label", "backend", "--label", "urgent"},
			wantJQL: `labels = "backend" AND labels = "urgent" ORDER BY updated DESC`,
		},
		{
			name:    "updated-since flag",
			args:    []string{"--updated-since", "7d"},
			wantJQL: `updated >= "-7d" ORDER BY updated DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedJQL string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				capturedJQL, _ = body["jql"].(string)

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"issues": []any{},
					"total":  float64(0),
				})
			}))
			defer srv.Close()

			apiClient := &client.Ref{
				Client: client.NewClient("dGVzdDp0ZXN0", client.WithBaseURL(srv.URL)),
			}
			format := output.FormatJSON
			var buf bytes.Buffer
			cmd := issueSearchCommand(apiClient, &buf, &format)
			cmd.SetContext(context.Background())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if capturedJQL != tt.wantJQL {
				t.Errorf("JQL sent to API:\n  got:  %q\n  want: %q", capturedJQL, tt.wantJQL)
			}
		})
	}
}

func TestSemanticJQLFlags_MutuallyExclusiveWithJQL(t *testing.T) {
	t.Parallel()

	// Each semantic flag should be mutually exclusive with --jql.
	semanticFlags := []struct {
		name string
		args []string
	}{
		{name: "assignee", args: []string{"--jql", "project = TEST", "--assignee", "me"}},
		{name: "status", args: []string{"--jql", "project = TEST", "--status", "Open"}},
		{name: "type", args: []string{"--jql", "project = TEST", "--type", "Bug"}},
		{name: "priority", args: []string{"--jql", "project = TEST", "--priority", "High"}},
		{name: "label", args: []string{"--jql", "project = TEST", "--label", "backend"}},
		{name: "sprint", args: []string{"--jql", "project = TEST", "--sprint", "current"}},
		{name: "updated-since", args: []string{"--jql", "project = TEST", "--updated-since", "7d"}},
	}

	for _, tt := range semanticFlags {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			format := output.FormatJSON
			var buf bytes.Buffer
			// No server needed; Cobra should reject before RunE.
			apiClient := &client.Ref{}
			cmd := issueSearchCommand(apiClient, &buf, &format)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error when --%s and --jql are both set, got nil", tt.name)
			}
		})
	}
}
