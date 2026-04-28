package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	want := []string{"whoami", "issue", "field", "project", "role", "board", "schema"}
	for _, name := range want {
		if app.Command(name) == nil {
			t.Errorf("command %q = nil, want registered", name)
		}
	}
}

func TestBuildAppWithDeps_SkipsAuthForSchema(t *testing.T) {
	t.Parallel()

	deps := appDeps{
		newClient: client.NewClient,
		loadConfig: func(string) (*auth.Config, error) {
			return nil, errors.New("auth should not load for schema")
		},
	}
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, deps)

	if err := app.Run(context.Background(), []string{"jira-agent", "schema"}); err != nil {
		t.Fatalf("schema Run() error = %v, want nil", err)
	}
	if !strings.Contains(buf.String(), `"whoami"`) {
		t.Errorf("schema output = %q, want whoami command", buf.String())
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

	if err := app.Run(context.Background(), []string{"jira-agent", "--version"}); err != nil {
		t.Fatalf("version Run() error = %v, want nil", err)
	}
	if !strings.Contains(buf.String(), "jira-agent version") {
		t.Errorf("version output = %q, want app version", buf.String())
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

	err := app.Run(context.Background(), []string{"jira-agent", "version", "list", "--project", "PROJ"})
	if err == nil {
		t.Fatal("version list Run() error = nil, want auth error")
	}
	var authErr *apperr.AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("errors.As(AuthError) = false, want true")
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
	if err := app.Run(context.Background(), []string{"jira-agent", "version", "list", "--project", "PROJ"}); err != nil {
		t.Fatalf("version list Run() error = %v, want nil", err)
	}
	if !strings.Contains(buf.String(), `"name":"v1.0"`) {
		t.Errorf("version list output = %q, want version name", buf.String())
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
	if err := app.Run(context.Background(), []string{"jira-agent", "--config", "/tmp/jira-agent.json", "whoami"}); err != nil {
		t.Fatalf("whoami Run() error = %v, want nil", err)
	}
	if loadedPath != "/tmp/jira-agent.json" {
		t.Errorf("loadedPath = %q, want %q", loadedPath, "/tmp/jira-agent.json")
	}
	if !strings.Contains(buf.String(), `"accountId":"abc123"`) {
		t.Errorf("whoami output = %q, want account ID", buf.String())
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
	if err := app.Run(context.Background(), []string{"jira-agent", "--pretty", "whoami"}); err != nil {
		t.Fatalf("whoami Run() error = %v, want nil", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\n  ") {
		t.Errorf("whoami output = %q, want pretty-printed JSON", got)
	}
	if !strings.Contains(got, `"accountId": "abc123"`) {
		t.Errorf("whoami output = %q, want pretty-printed account ID", got)
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
	if err := app.Run(context.Background(), []string{"jira-agent", "--pretty", "--output", "csv", "whoami"}); err != nil {
		t.Fatalf("whoami Run() error = %v, want nil", err)
	}

	got := buf.String()
	if strings.Contains(got, "{") || strings.Contains(got, "\n  ") {
		t.Errorf("whoami output = %q, want CSV output unaffected by --pretty", got)
	}
	if !strings.Contains(got, "accountId,displayName") {
		t.Errorf("whoami output = %q, want CSV header", got)
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
		err := app.Run(context.Background(), []string{"jira-agent", "--output", "xml", "schema"})
		if err == nil {
			t.Fatal("Run() error = nil, want validation error")
		}
		var validationErr *apperr.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("errors.As(ValidationError) = false, want true")
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		t.Parallel()

		app := buildAppWithDeps(&bytes.Buffer{}, appDeps{
			newClient:  client.NewClient,
			loadConfig: auth.LoadConfig,
		})
		err := app.Run(context.Background(), []string{"jira-agent", "whomai"})
		if err == nil {
			t.Fatal("Run() error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), `Did you mean "whoami"`) {
			t.Errorf("error = %q, want whoami suggestion", err.Error())
		}
	})
}
