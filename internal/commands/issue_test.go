package commands

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	apperr "github.com/major/jira-agent/internal/errors"
)

func TestToADF(t *testing.T) {
	t.Parallel()

	t.Run("plain text", func(t *testing.T) {
		t.Parallel()

		got := toADF("hello")
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("toADF() type = %T, want map[string]any", got)
		}
		if m["type"] != "doc" {
			t.Errorf("type = %v, want %v", m["type"], "doc")
		}
		content := m["content"].([]any)
		paragraph := content[0].(map[string]any)
		text := paragraph["content"].([]any)[0].(map[string]any)
		if text["text"] != "hello" {
			t.Errorf("text = %v, want %v", text["text"], "hello")
		}
	})

	t.Run("existing adf json", func(t *testing.T) {
		t.Parallel()

		got := toADF(`{"type":"doc","version":1,"content":[]}`)
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("toADF() type = %T, want map[string]any", got)
		}
		if m["type"] != "doc" {
			t.Errorf("type = %v, want %v", m["type"], "doc")
		}
		if _, ok := m["content"].([]any); !ok {
			t.Errorf("content type = %T, want []any", m["content"])
		}
	})

	t.Run("json without type becomes text", func(t *testing.T) {
		t.Parallel()

		got := toADF(`{"content":"not adf"}`)
		m := got.(map[string]any)
		content := m["content"].([]any)
		paragraph := content[0].(map[string]any)
		text := paragraph["content"].([]any)[0].(map[string]any)
		if text["text"] != `{"content":"not adf"}` {
			t.Errorf("text = %v, want %v", text["text"], `{"content":"not adf"}`)
		}
	})
}

func TestSplitTrimmed(t *testing.T) {
	t.Parallel()

	got := splitTrimmed(" alpha, ,beta,gamma ")
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitTrimmed() = %v, want %v", got, want)
	}
}

func TestParseInt64List(t *testing.T) {
	t.Parallel()

	t.Run("valid IDs", func(t *testing.T) {
		t.Parallel()

		got, err := parseInt64List(" 10000,10001 ")
		if err != nil {
			t.Fatalf("parseInt64List() error = %v, want nil", err)
		}
		want := []int64{10000, 10001}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseInt64List() = %v, want %v", got, want)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		t.Parallel()

		_, err := parseInt64List("10000,abc")
		if err == nil {
			t.Fatal("parseInt64List() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})
}

func TestApplyFieldOverrides(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"existing": "keep"}
	applyFieldOverrides(fields, map[string]string{
		"raw":    "hello",
		"number": "42",
		"object": `{"id":"abc"}`,
	})

	if fields["existing"] != "keep" {
		t.Errorf("existing = %v, want %v", fields["existing"], "keep")
	}
	if fields["raw"] != "hello" {
		t.Errorf("raw = %v, want %v", fields["raw"], "hello")
	}
	if fields["number"] != float64(42) {
		t.Errorf("number = %v, want %v", fields["number"], float64(42))
	}
	object, ok := fields["object"].(map[string]any)
	if !ok {
		t.Fatalf("object type = %T, want map[string]any", fields["object"])
	}
	if object["id"] != "abc" {
		t.Errorf("object[id] = %v, want %v", object["id"], "abc")
	}
}

func TestApplyFieldsJSON(t *testing.T) {
	t.Parallel()

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{"summary": "old"}
		if err := applyFieldsJSON(fields, ""); err != nil {
			t.Fatalf("applyFieldsJSON() error = %v, want nil", err)
		}
		if fields["summary"] != "old" {
			t.Errorf("summary = %v, want %v", fields["summary"], "old")
		}
	})

	t.Run("merges object", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{"summary": "old"}
		if err := applyFieldsJSON(fields, `{"summary":"new","priority":{"name":"High"}}`); err != nil {
			t.Fatalf("applyFieldsJSON() error = %v, want nil", err)
		}
		if fields["summary"] != "new" {
			t.Errorf("summary = %v, want %v", fields["summary"], "new")
		}
		priority := fields["priority"].(map[string]any)
		if priority["name"] != "High" {
			t.Errorf("priority[name] = %v, want %v", priority["name"], "High")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		err := applyFieldsJSON(map[string]any{}, `{`)
		if err == nil {
			t.Fatal("applyFieldsJSON() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})
}

func TestApplyCommonFields(t *testing.T) {
	t.Parallel()

	var got map[string]any
	cmd := &cli.Command{
		Name: "test",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "description"},
			&cli.StringFlag{Name: "assignee"},
			&cli.StringFlag{Name: "priority"},
			&cli.StringFlag{Name: "labels"},
			&cli.StringFlag{Name: "components"},
			&cli.StringFlag{Name: "parent"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			got = map[string]any{"summary": "Keep existing summary"}
			applyCommonFields(got, cmd)
			return nil
		},
	}

	args := []string{
		"test",
		"--description", "Plain description",
		"--assignee", "account-123",
		"--priority", "High",
		"--labels", " bug, needs-triage ",
		"--components", " API , CLI ",
		"--parent", "PROJ-1",
	}
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got["summary"] != "Keep existing summary" {
		t.Errorf("summary = %v, want existing value preserved", got["summary"])
	}
	assertADFText(t, got["description"], "Plain description")
	if !reflect.DeepEqual(got["assignee"], map[string]any{"accountId": "account-123"}) {
		t.Errorf("assignee = %v, want account ID map", got["assignee"])
	}
	if !reflect.DeepEqual(got["priority"], map[string]any{"name": "High"}) {
		t.Errorf("priority = %v, want priority name map", got["priority"])
	}
	if !reflect.DeepEqual(got["labels"], []string{"bug", "needs-triage"}) {
		t.Errorf("labels = %v, want trimmed labels", got["labels"])
	}
	wantComponents := []map[string]any{{"name": "API"}, {"name": "CLI"}}
	if !reflect.DeepEqual(got["components"], wantComponents) {
		t.Errorf("components = %v, want %v", got["components"], wantComponents)
	}
	if !reflect.DeepEqual(got["parent"], map[string]any{"key": "PROJ-1"}) {
		t.Errorf("parent = %v, want parent key map", got["parent"])
	}
}

func TestParseBulkCreateBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jsonInput string
		wantCount int
		wantErr   string
	}{
		{
			name:      "wraps issue array",
			jsonInput: `[{"fields":{"summary":"one"}}]`,
			wantCount: 1,
		},
		{
			name:      "keeps issueUpdates object",
			jsonInput: `{"issueUpdates":[{"fields":{"summary":"one"}}],"properties":{"source":"test"}}`,
			wantCount: 1,
		},
		{
			name:    "requires input",
			wantErr: "--issues-json is required",
		},
		{
			name:      "rejects invalid JSON",
			jsonInput: `{`,
			wantErr:   "invalid --issues-json",
		},
		{
			name:      "rejects empty array",
			jsonInput: `[]`,
			wantErr:   "--issues-json must include at least one issue",
		},
		{
			name:      "rejects object without issueUpdates",
			jsonInput: `{"issues":[]}`,
			wantErr:   "--issues-json object must include issueUpdates array",
		},
		{
			name:      "rejects scalar JSON",
			jsonInput: `true`,
			wantErr:   "--issues-json must be a JSON array or object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseBulkCreateBody(tt.jsonInput)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseBulkCreateBody() error = %v, want it to contain %q", err, tt.wantErr)
				}
				var validationErr *apperr.ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("error type = %T, want *ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBulkCreateBody() error = %v, want nil", err)
			}
			updates, ok := got["issueUpdates"].([]any)
			if !ok {
				t.Fatalf("issueUpdates type = %T, want []any", got["issueUpdates"])
			}
			if len(updates) != tt.wantCount {
				t.Errorf("len(issueUpdates) = %d, want %d", len(updates), tt.wantCount)
			}
		})
	}
}

func TestParseBulkCreateBody_LimitsIssueCount(t *testing.T) {
	t.Parallel()

	issueJSON := `{"fields":{"summary":"one"}}`
	tooManyIssues := "[" + strings.Repeat(issueJSON+",", 50) + issueJSON + "]"
	_, err := parseBulkCreateBody(tooManyIssues)
	if err == nil {
		t.Fatal("parseBulkCreateBody() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--issues-json supports at most 50 issues") {
		t.Errorf("parseBulkCreateBody() error = %q, want issue count limit", err.Error())
	}
}

func TestExtractPaginationMeta(t *testing.T) {
	t.Parallel()

	result := map[string]any{
		"total":      float64(10),
		"startAt":    float64(2),
		"maxResults": float64(5),
		"issues":     []any{map[string]any{"key": "TEST-1"}, map[string]any{"key": "TEST-2"}},
	}
	meta := extractPaginationMeta(result)
	if meta.Total != 10 {
		t.Errorf("Total = %d, want %d", meta.Total, 10)
	}
	if meta.StartAt != 2 {
		t.Errorf("StartAt = %d, want %d", meta.StartAt, 2)
	}
	if meta.MaxResults != 5 {
		t.Errorf("MaxResults = %d, want %d", meta.MaxResults, 5)
	}
	if meta.Returned != 2 {
		t.Errorf("Returned = %d, want %d", meta.Returned, 2)
	}
}

func assertADFText(t *testing.T, value any, want string) {
	t.Helper()

	doc, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("ADF type = %T, want map[string]any", value)
	}
	content, ok := doc["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("ADF content = %v, want non-empty []any", doc["content"])
	}
	paragraph, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("ADF paragraph type = %T, want map[string]any", content[0])
	}
	paragraphContent, ok := paragraph["content"].([]any)
	if !ok || len(paragraphContent) == 0 {
		t.Fatalf("ADF paragraph content = %v, want non-empty []any", paragraph["content"])
	}
	textNode, ok := paragraphContent[0].(map[string]any)
	if !ok {
		t.Fatalf("ADF text node type = %T, want map[string]any", paragraphContent[0])
	}
	if textNode["text"] != want {
		t.Errorf("ADF text = %v, want %q", textNode["text"], want)
	}
}

func TestFindTransitionID(t *testing.T) {
	t.Parallel()

	transitions := map[string]any{
		"transitions": []any{
			map[string]any{"id": "11", "name": "Start Progress", "to": map[string]any{"name": "In Progress"}},
			map[string]any{"id": "21", "name": "Resolve", "to": map[string]any{"name": "Done"}},
		},
	}

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "matches transition name", target: "resolve", want: "21"},
		{name: "matches status name", target: "in progress", want: "11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := findTransitionID(transitions, tt.target)
			if err != nil {
				t.Fatalf("findTransitionID() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("findTransitionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindTransitionID_Errors(t *testing.T) {
	t.Parallel()

	_, err := findTransitionID(map[string]any{"transitions": []any{}}, "Done")
	if err == nil {
		t.Fatal("findTransitionID() error = nil, want error")
	}
	var notFoundErr *apperr.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("errors.As(NotFoundError) = false, want true")
	}
	if !strings.Contains(notFoundErr.Details(), "available transitions") {
		t.Errorf("Details() = %q, want available transitions", notFoundErr.Details())
	}

	_, err = findTransitionID("bad", "Done")
	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("errors.As(ValidationError) = false, want true")
	}
}

func TestFindIssueTypeID(t *testing.T) {
	t.Parallel()

	typeList := map[string]any{
		"values": []any{
			map[string]any{"id": "10001", "name": "Story"},
			map[string]any{"id": "10002", "name": "Bug"},
		},
	}

	got, err := findIssueTypeID(typeList, "bug")
	if err != nil {
		t.Fatalf("findIssueTypeID() error = %v, want nil", err)
	}
	if got != "10002" {
		t.Errorf("findIssueTypeID() = %q, want %q", got, "10002")
	}

	_, err = findIssueTypeID(typeList, "Task")
	if err == nil {
		t.Fatal("findIssueTypeID() error = nil, want error")
	}
	var notFoundErr *apperr.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("errors.As(NotFoundError) = false, want true")
	}
}
