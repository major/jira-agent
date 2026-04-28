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

func TestBuildWorklogBody(t *testing.T) {
	t.Parallel()

	t.Run("builds full body", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		cmd := &cli.Command{
			Name:  "test",
			Flags: worklogMutationFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				var err error
				got, err = buildWorklogBody(cmd, true)
				return err
			},
		}
		args := []string{
			"test",
			"--started", "2026-04-27T10:00:00.000-0500",
			"--time-spent", "1h 30m",
			"--time-spent-seconds", "5400",
			"--comment", "Implemented the CLI change",
			"--visibility-type", "group",
			"--visibility-value", "jira-users",
			"--properties-json", `[{"key":"source","value":"agent"}]`,
		}
		if err := cmd.Run(context.Background(), args); err != nil {
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

		cmd := &cli.Command{
			Name:  "test",
			Flags: worklogMutationFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				_, err := buildWorklogBody(cmd, true)
				return err
			},
		}
		cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}
		err := cmd.Run(context.Background(), []string{"test", "--time-spent", "1h"})
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

		cmd := &cli.Command{
			Name:  "test",
			Flags: worklogMutationFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				_, err := buildWorklogBody(cmd, false)
				return err
			},
		}
		cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}
		err := cmd.Run(context.Background(), []string{"test"})
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
		cmd := &cli.Command{
			Name:  "test",
			Flags: filterShareBodyFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				var err error
				got, err = buildFilterShareBody(cmd)
				return err
			},
		}
		args := []string{
			"test",
			"--body-json", `{"type":"global","rights":1}`,
			"--with", "project:10000",
			"--type", "projectRole",
			"--project-role-id", "20000",
			"--rights", "3",
		}
		if err := cmd.Run(context.Background(), args); err != nil {
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

		cmd := &cli.Command{
			Name:  "test",
			Flags: filterShareBodyFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				_, err := buildFilterShareBody(cmd)
				return err
			},
		}
		cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}
		err := cmd.Run(context.Background(), []string{"test"})
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

		cmd := &cli.Command{
			Name:  "test",
			Flags: filterShareBodyFlags(),
			Action: func(_ context.Context, cmd *cli.Command) error {
				_, err := buildFilterShareBody(cmd)
				return err
			},
		}
		cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}
		err := cmd.Run(context.Background(), []string{"test", "--body-json", "not-json"})
		if err == nil {
			t.Fatal("buildFilterShareBody() error = nil, want error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})
}

func filterShareBodyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "body-json"},
		&cli.StringFlag{Name: "with"},
		&cli.StringFlag{Name: "type"},
		&cli.StringFlag{Name: "account-id"},
		&cli.StringFlag{Name: "group-id"},
		&cli.StringFlag{Name: "groupname"},
		&cli.StringFlag{Name: "project-id"},
		&cli.StringFlag{Name: "project-role-id"},
		&cli.IntFlag{Name: "rights"},
	}
}
