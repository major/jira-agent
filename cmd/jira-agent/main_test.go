package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/major/jira-agent/internal/auth"
	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestBuildApp_RootCommands(t *testing.T) {
	t.Parallel()

	app := buildApp(&bytes.Buffer{})

	var names []string
	for _, cmd := range app.Commands() {
		names = append(names, cmd.Name())
	}

	want := []string{"whoami", "issue", "field", "project", "role", "board"}
	for _, name := range want {
		if !slices.Contains(names, name) {
			t.Errorf("command %q should be registered", name)
		}
	}
}

func TestBuildAppWithDeps_HelpIncludesRepositorySupport(t *testing.T) {
	t.Parallel()

	deps := appDeps{
		newClient: client.NewClient,
		loadConfig: func(string) (*auth.Config, error) {
			return nil, errors.New("auth should not load for help")
		},
	}
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"--help"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"https://github.com/major/jira-agent",
		"File new bugs and RFEs there.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q", want)
		}
	}
}

func TestBuildAppWithDeps_SkipsAuthForVersionFlag(t *testing.T) {
	t.Parallel()

	deps := appDeps{
		newClient: client.NewClient,
		loadConfig: func(string) (*auth.Config, error) {
			return nil, errors.New("auth should not load for version flag")
		},
	}
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"--version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "jira-agent version") {
		t.Errorf("expected output to contain %q, got %q", "jira-agent version", buf.String())
	}
}

func TestBuildAppWithDeps_VersionCommandRequiresAuth(t *testing.T) {
	t.Parallel()

	deps := appDeps{
		newClient: client.NewClient,
		loadConfig: func(string) (*auth.Config, error) {
			return nil, errors.New("missing config")
		},
	}
	app := buildAppWithDeps(&bytes.Buffer{}, deps)

	app.SetArgs([]string{"version", "list", "--project", "PROJ"})
	err := app.Execute()
	if err == nil {
		t.Fatal("version list should fail without auth")
	}

	var authErr *apperr.AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("error should be AuthError, got: %v", err)
	}
}

func TestBuildAppWithDeps_VersionCommandLoadsConfigAndClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project/PROJ/version" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/project/PROJ/version")
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
			t.Errorf("Authorization = %q, want Basic prefix", got)
		}
		testhelpers.WriteJSONResponse(t, w, `{"values":[{"id":"10000","name":"v1.0"}],"total":1,"startAt":0,"maxResults":50}`)
	}))
	defer server.Close()

	deps := appDeps{
		newClient: func(authHeader string, _ ...client.Option) *client.Client {
			return client.NewClient(authHeader, client.WithBaseURL(server.URL))
		},
		loadConfig: func(string) (*auth.Config, error) {
			return &auth.Config{
				Instance: "example.atlassian.net",
				Email:    "user@example.com",
				APIKey:   "token",
			}, nil
		},
	}

	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"version", "list", "--project", "PROJ"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"name":"v1.0"`) {
		t.Errorf("expected output to contain %q, got %q", `"name":"v1.0"`, buf.String())
	}
}

func TestBuildAppWithDeps_WhoamiLoadsConfigAndClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/myself" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/myself")
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
			t.Errorf("Authorization = %q, want Basic prefix", got)
		}
		testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123"}`)
	}))
	defer server.Close()

	var loadedPath string
	deps := appDeps{
		newClient: func(authHeader string, _ ...client.Option) *client.Client {
			return client.NewClient(authHeader, client.WithBaseURL(server.URL))
		},
		loadConfig: func(path string) (*auth.Config, error) {
			loadedPath = path
			return &auth.Config{
				Instance: "example.atlassian.net",
				Email:    "user@example.com",
				APIKey:   "token",
			}, nil
		},
	}

	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"--config", "/tmp/jira-agent.json", "whoami"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loadedPath != "/tmp/jira-agent.json" {
		t.Errorf("loadedPath = %q, want %q", loadedPath, "/tmp/jira-agent.json")
	}
	if !strings.Contains(buf.String(), `"accountId":"abc123"`) {
		t.Errorf("expected output to contain %q, got %q", `"accountId":"abc123"`, buf.String())
	}
}

func TestBuildAppWithDeps_PrettyPrintsJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123"}`)
	}))
	defer server.Close()

	deps := appDeps{
		newClient: func(authHeader string, _ ...client.Option) *client.Client {
			return client.NewClient(authHeader, client.WithBaseURL(server.URL))
		},
		loadConfig: func(string) (*auth.Config, error) {
			return &auth.Config{
				Instance: "example.atlassian.net",
				Email:    "user@example.com",
				APIKey:   "token",
			}, nil
		},
	}

	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"--pretty", "whoami"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\n  ") {
		t.Errorf("expected pretty-printed output with indentation")
	}
	if !strings.Contains(got, `"accountId": "abc123"`) {
		t.Errorf("expected output to contain %q, got %q", `"accountId": "abc123"`, got)
	}
}

func TestBuildAppWithDeps_PrettyDoesNotAffectCSV(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		testhelpers.WriteJSONResponse(t, w, `{"accountId":"abc123","displayName":"Alice"}`)
	}))
	defer server.Close()

	deps := appDeps{
		newClient: func(authHeader string, _ ...client.Option) *client.Client {
			return client.NewClient(authHeader, client.WithBaseURL(server.URL))
		},
		loadConfig: func(string) (*auth.Config, error) {
			return &auth.Config{
				Instance: "example.atlassian.net",
				Email:    "user@example.com",
				APIKey:   "token",
			}, nil
		},
	}

	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	app.SetArgs([]string{"--pretty", "--output", "csv", "whoami"})
	if err := app.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "{") {
		t.Errorf("expected CSV output to not contain %q, got %q", "{", got)
	}
	if strings.Contains(got, "\n  ") {
		t.Errorf("expected CSV output to not contain indentation, got %q", got)
	}
	if !strings.Contains(got, "accountId,displayName") {
		t.Errorf("expected output to contain %q, got %q", "accountId,displayName", got)
	}
}

func TestBuildAppWithDeps_Errors(t *testing.T) {
	t.Parallel()

	t.Run("invalid output format", func(t *testing.T) {
		t.Parallel()

		app := buildAppWithDeps(&bytes.Buffer{}, appDeps{
			newClient:  client.NewClient,
			loadConfig: auth.LoadConfig,
		})

		app.SetArgs([]string{"--output", "xml", "whoami"})
		err := app.Execute()
		if err == nil {
			t.Fatal("invalid output format should return error")
		}

		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("error should be ValidationError, got: %v", err)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Parallel()

		app := buildAppWithDeps(&bytes.Buffer{}, appDeps{
			newClient:  client.NewClient,
			loadConfig: auth.LoadConfig,
		})

		app.SetArgs([]string{"whomai"})
		err := app.Execute()
		if err == nil {
			t.Fatal("unknown command should return error")
		}
		if !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("expected error to contain %q, got %q", "unknown command", err.Error())
		}
		if !strings.Contains(err.Error(), "whomai") {
			t.Errorf("expected error to contain %q, got %q", "whomai", err.Error())
		}
	})
}
