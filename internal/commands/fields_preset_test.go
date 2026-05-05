package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

func TestFieldsPresetExpansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		preset     string
		wantFields []string
	}{
		{
			name:       "minimal preset",
			preset:     "minimal",
			wantFields: []string{"summary", "status"},
		},
		{
			name:       "triage preset",
			preset:     "triage",
			wantFields: []string{"summary", "status", "priority", "assignee", "labels"},
		},
		{
			name:       "detail preset",
			preset:     "detail",
			wantFields: []string{"summary", "status", "priority", "assignee", "labels", "description", "created", "updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := expandFieldsPreset(tt.preset)
			if err != nil {
				t.Fatalf("expandFieldsPreset(%q) error = %v", tt.preset, err)
			}

			// The expanded value should contain all expected fields.
			expanded := issueSearchFields(got)
			for _, wantField := range tt.wantFields {
				if !slices.Contains(expanded, wantField) {
					t.Errorf("expanded fields %v missing expected field %q", expanded, wantField)
				}
			}
		})
	}
}

func TestFieldsPresetExpansion_InvalidPreset(t *testing.T) {
	t.Parallel()

	_, err := expandFieldsPreset("nonexistent")
	if err == nil {
		t.Fatal("expandFieldsPreset(nonexistent) = nil, want error")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestFieldsPresetMutuallyExclusive(t *testing.T) {
	t.Parallel()

	// Set up a test server that returns a valid search response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	// Setting both --fields-preset and --fields should fail.
	cmd.SetArgs([]string{
		"--jql", "project = TEST",
		"--fields-preset", "minimal",
		"--fields", "key,summary",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --fields-preset and --fields are set, got nil")
	}
}

func TestFieldsPresetSearch_UsesPreset(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
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
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"--jql", "project = TEST",
		"--fields-preset", "minimal",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify the API request used the preset's fields.
	fields, ok := capturedBody["fields"].([]any)
	if !ok {
		t.Fatalf("expected fields in request body, got %v", capturedBody)
	}

	// minimal preset = key,summary,status; key is stripped by issueSearchFields.
	wantFields := map[string]bool{"summary": false, "status": false}
	for _, f := range fields {
		fieldStr, ok := f.(string)
		if !ok {
			continue
		}
		if _, tracked := wantFields[fieldStr]; tracked {
			wantFields[fieldStr] = true
		}
	}
	for field, found := range wantFields {
		if !found {
			t.Errorf("expected field %q in API request, got fields = %v", field, fields)
		}
	}
}

func TestFieldsPresetGet_UsesPreset(t *testing.T) {
	t.Parallel()

	var capturedParams string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedParams = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key":    "TEST-1",
			"fields": map[string]any{"summary": "Test"},
		})
	}))
	defer srv.Close()

	apiClient := &client.Ref{
		Client: client.NewClient("dGVzdDp0ZXN0", client.WithBaseURL(srv.URL)),
	}
	format := output.FormatJSON
	var buf bytes.Buffer

	cmd := issueGetCommand(apiClient, &buf, &format)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"TEST-1",
		"--fields-preset", "minimal",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// minimal preset = key,summary,status.
	if capturedParams == "" {
		t.Fatal("expected fields query parameter, got empty")
	}
	for _, want := range []string{"key", "summary", "status"} {
		if !containsField(capturedParams, want) {
			t.Errorf("expected field %q in params %q", want, capturedParams)
		}
	}
}

// containsField checks if a comma-separated field string contains a specific field.
func containsField(fieldList, field string) bool {
	return slices.Contains(splitTrimmed(fieldList), field)
}
