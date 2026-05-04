package commands

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	apperr "github.com/major/jira-agent/internal/errors"
)

func TestBuildWorklogBody(t *testing.T) {
	t.Parallel()

	t.Run("builds full body", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			var err error
			got, err = buildWorklogBody(cmd, true)
			return err
		})
		addTestWorklogMutationFlags(cmd)
		args := []string{
			"--started", "2026-04-27T10:00:00.000-0500",
			"--time-spent", "1h 30m",
			"--time-spent-seconds", "5400",
			"--comment", "Implemented the CLI change",
			"--visibility-type", "group",
			"--visibility-value", "jira-users",
			"--properties-json", `[{"key":"source","value":"agent"}]`,
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("command failed: %v", err)
		}

		if got["started"] != "2026-04-27T10:00:00.000-0500" {
			t.Errorf("started = %v, want timestamp", got["started"])
		}
		if got["timeSpent"] != "1h 30m" {
			t.Errorf("timeSpent = %v, want 1h 30m", got["timeSpent"])
		}
		if got["timeSpentSeconds"] != 5400 {
			t.Errorf("timeSpentSeconds = %v, want 5400", got["timeSpentSeconds"])
		}
		assertADFText(t, got["comment"], "Implemented the CLI change")
		wantVisibility := map[string]any{"type": "group", "value": "jira-users"}
		if !reflect.DeepEqual(got["visibility"], wantVisibility) {
			t.Errorf("visibility = %v, want %v", got["visibility"], wantVisibility)
		}
		wantProperties := []any{map[string]any{"key": "source", "value": "agent"}}
		if !reflect.DeepEqual(got["properties"], wantProperties) {
			t.Errorf("properties = %v, want %v", got["properties"], wantProperties)
		}
	})

	t.Run("requires core fields for create", func(t *testing.T) {
		t.Parallel()

		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			_, err := buildWorklogBody(cmd, true)
			return err
		})
		addTestWorklogMutationFlags(cmd)
		cmd.SetArgs([]string{"--time-spent", "1h"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("buildWorklogBody() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
		if !strings.Contains(err.Error(), "--started is required") {
			t.Errorf("error = %q, want --started is required", err.Error())
		}
	})

	t.Run("requires at least one edit field", func(t *testing.T) {
		t.Parallel()

		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			_, err := buildWorklogBody(cmd, false)
			return err
		})
		addTestWorklogMutationFlags(cmd)
		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("buildWorklogBody() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})
}

func TestBuildFilterShareBody(t *testing.T) {
	t.Parallel()

	t.Run("merges raw shorthand and explicit flags", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			var err error
			got, err = buildFilterShareBody(cmd)
			return err
		})
		addTestFilterShareBodyFlags(cmd)
		args := []string{
			"--body-json", `{"type":"global","rights":1}`,
			"--with", "project:10000",
			"--type", "projectRole",
			"--project-role-id", "20000",
			"--rights", "3",
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("command failed: %v", err)
		}

		want := map[string]any{
			"type":          "projectRole",
			"projectId":     "10000",
			"projectRoleId": "20000",
			"rights":        3,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("body = %v, want %v", got, want)
		}
	})

	t.Run("requires share type", func(t *testing.T) {
		t.Parallel()

		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			_, err := buildFilterShareBody(cmd)
			return err
		})
		addTestFilterShareBodyFlags(cmd)
		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("buildFilterShareBody() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})

	t.Run("rejects invalid raw JSON", func(t *testing.T) {
		t.Parallel()

		cmd := newBodyBuilderTestCommand(func(cmd *cobra.Command) error {
			_, err := buildFilterShareBody(cmd)
			return err
		})
		addTestFilterShareBodyFlags(cmd)
		cmd.SetArgs([]string{"--body-json", "not-json"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("buildFilterShareBody() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})
}

func newBodyBuilderTestCommand(run func(*cobra.Command) error) *cobra.Command {
	return &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd)
		},
	}
}

func addTestWorklogMutationFlags(cmd *cobra.Command) {
	cmd.Flags().String("started", "", "")
	cmd.Flags().String("time-spent", "", "")
	cmd.Flags().Int("time-spent-seconds", 0, "")
	cmd.Flags().String("comment", "", "")
	cmd.Flags().String("visibility-type", "", "")
	cmd.Flags().String("visibility-value", "", "")
	cmd.Flags().String("properties-json", "", "")
}

func addTestFilterShareBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("body-json", "", "")
	cmd.Flags().String("with", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().String("account-id", "", "")
	cmd.Flags().String("group-id", "", "")
	cmd.Flags().String("groupname", "", "")
	cmd.Flags().String("project-id", "", "")
	cmd.Flags().String("project-role-id", "", "")
	cmd.Flags().Int("rights", 0, "")
}
