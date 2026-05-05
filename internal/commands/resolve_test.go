package commands

import (
	"errors"
	"testing"

	apperr "github.com/major/jira-agent/internal/errors"
)

func TestResolverMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		total     int
		returned  int
		usageHint string
	}{
		{
			name:      "all fields populated",
			total:     42,
			returned:  10,
			usageHint: "use --max-results to limit",
		},
		{
			name:      "zero values",
			total:     0,
			returned:  0,
			usageHint: "",
		},
		{
			name:      "partial results",
			total:     100,
			returned:  25,
			usageHint: "more results available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := resolverMetadata(tt.total, tt.returned, tt.usageHint)

			if meta.Total != tt.total {
				t.Errorf("Total: got %d, want %d", meta.Total, tt.total)
			}
			if meta.Returned != tt.returned {
				t.Errorf("Returned: got %d, want %d", meta.Returned, tt.returned)
			}
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
				if !contains(err.Error(), tt.entityName) {
					t.Errorf("error message: got %q, want to contain %q", err.Error(), tt.entityName)
				}
			}

			if got != tt.want {
				t.Errorf("query: got %q, want %q", got, tt.want)
			}
		})
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
