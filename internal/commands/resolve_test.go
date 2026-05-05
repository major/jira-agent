package commands

import (
	"errors"
	"strings"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
)

func TestResolverMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		usageHint string
	}{
		{
			name:      "with usage hint",
			usageHint: "jira-agent issue assign <key> --assignee <id>",
		},
		{
			name:      "empty usage hint",
			usageHint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meta := resolverMetadata(tt.usageHint)

			if meta.UsageHint != tt.usageHint {
				t.Errorf("UsageHint: got %q, want %q", meta.UsageHint, tt.usageHint)
			}
			if meta.Timestamp == "" {
				t.Error("Timestamp: expected non-empty, got empty")
			}
		})
	}
}

func TestRequireQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		entityName string
		want       string
		wantErr    bool
	}{
		{
			name:       "valid query",
			args:       []string{"John Doe"},
			entityName: "user",
			want:       "John Doe",
			wantErr:    false,
		},
		{
			name:       "empty args",
			args:       []string{},
			entityName: "project",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "empty string arg",
			args:       []string{""},
			entityName: "status",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "query with spaces",
			args:       []string{"My Project Name"},
			entityName: "project",
			want:       "My Project Name",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := requireQuery(tt.args, tt.entityName)

			if (err != nil) != tt.wantErr {
				t.Errorf("error: got %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				// Verify it's a ValidationError
				var validationErr *apperr.ValidationError
				if !errors.As(err, &validationErr) {
					t.Errorf("error type: got %T, want ValidationError", err)
				}
				// Verify error message contains entity name
				if !strings.Contains(err.Error(), tt.entityName) {
					t.Errorf("error message: got %q, want to contain %q", err.Error(), tt.entityName)
				}
			}

			if got != tt.want {
				t.Errorf("query: got %q, want %q", got, tt.want)
			}
		})
	}
}


