package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/testhelpers"
)

func TestPropertyCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cmd        func(*client.Ref, *bytes.Buffer) *cli.Command
		args       []string
		method     string
		path       string
		wantBody   map[string]any
		response   string
		wantOutput string
	}{
		{
			name: "issue list uses REST properties endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyListCommand(issuePropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"PROJ-1"},
			method:     http.MethodGet,
			path:       "/rest/api/3/issue/PROJ-1/properties",
			response:   `{"keys":[{"key":"com.example.flag"}]}`,
			wantOutput: `"com.example.flag"`,
		},
		{
			name: "issue get uses REST property endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyGetCommand(issuePropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"PROJ-1", "com.example.flag"},
			method:     http.MethodGet,
			path:       "/rest/api/3/issue/PROJ-1/properties/com.example.flag",
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "issue set sends raw JSON value",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertySetCommand(issuePropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:   []string{"--value-json", `{"enabled":true}`, "PROJ-1", "com.example.flag"},
			method: http.MethodPut,
			path:   "/rest/api/3/issue/PROJ-1/properties/com.example.flag",
			wantBody: map[string]any{
				"enabled": true,
			},
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "issue delete emits confirmation",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyDeleteCommand(issuePropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:       []string{"PROJ-1", "com.example.flag"},
			method:     http.MethodDelete,
			path:       "/rest/api/3/issue/PROJ-1/properties/com.example.flag",
			response:   `{}`,
			wantOutput: `"deleted":true`,
		},
		{
			name: "project list uses REST properties endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyListCommand(projectPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"PROJ"},
			method:     http.MethodGet,
			path:       "/rest/api/3/project/PROJ/properties",
			response:   `{"keys":[{"key":"com.example.flag"}]}`,
			wantOutput: `"com.example.flag"`,
		},
		{
			name: "project get uses REST property endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyGetCommand(projectPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"PROJ", "com.example.flag"},
			method:     http.MethodGet,
			path:       "/rest/api/3/project/PROJ/properties/com.example.flag",
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "project set sends raw JSON value",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertySetCommand(projectPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:   []string{"--value-json", `{"enabled":true}`, "PROJ", "com.example.flag"},
			method: http.MethodPut,
			path:   "/rest/api/3/project/PROJ/properties/com.example.flag",
			wantBody: map[string]any{
				"enabled": true,
			},
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "project delete emits confirmation",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyDeleteCommand(projectPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:       []string{"PROJ", "com.example.flag"},
			method:     http.MethodDelete,
			path:       "/rest/api/3/project/PROJ/properties/com.example.flag",
			response:   `{}`,
			wantOutput: `"deleted":true`,
		},
		{
			name: "sprint list uses Agile properties endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyListCommand(sprintPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"100"},
			method:     http.MethodGet,
			path:       "/rest/agile/1.0/sprint/100/properties",
			response:   `{"keys":[{"key":"com.example.flag"}]}`,
			wantOutput: `"com.example.flag"`,
		},
		{
			name: "sprint get uses Agile property endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyGetCommand(sprintPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"100", "com.example.flag"},
			method:     http.MethodGet,
			path:       "/rest/agile/1.0/sprint/100/properties/com.example.flag",
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "sprint set sends raw JSON value",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertySetCommand(sprintPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:   []string{"--value-json", `{"enabled":true}`, "100", "com.example.flag"},
			method: http.MethodPut,
			path:   "/rest/agile/1.0/sprint/100/properties/com.example.flag",
			wantBody: map[string]any{
				"enabled": true,
			},
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "sprint delete emits confirmation",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyDeleteCommand(sprintPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:       []string{"100", "com.example.flag"},
			method:     http.MethodDelete,
			path:       "/rest/agile/1.0/sprint/100/properties/com.example.flag",
			response:   `{}`,
			wantOutput: `"deleted":true`,
		},
		{
			name: "board list uses Agile properties endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyListCommand(boardPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"42"},
			method:     http.MethodGet,
			path:       "/rest/agile/1.0/board/42/properties",
			response:   `{"keys":[{"key":"com.example.flag"}]}`,
			wantOutput: `"com.example.flag"`,
		},
		{
			name: "board get uses Agile property endpoint",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyGetCommand(boardPropertyTarget(apiClient), buf, testCommandFormat())
			},
			args:       []string{"42", "com.example.flag"},
			method:     http.MethodGet,
			path:       "/rest/agile/1.0/board/42/properties/com.example.flag",
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "board set sends raw JSON value",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertySetCommand(boardPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:   []string{"--value-json", `{"enabled":true}`, "42", "com.example.flag"},
			method: http.MethodPut,
			path:   "/rest/agile/1.0/board/42/properties/com.example.flag",
			wantBody: map[string]any{
				"enabled": true,
			},
			response:   `{"key":"com.example.flag","value":{"enabled":true}}`,
			wantOutput: `"enabled":true`,
		},
		{
			name: "board delete emits confirmation",
			cmd: func(apiClient *client.Ref, buf *bytes.Buffer) *cli.Command {
				return propertyDeleteCommand(boardPropertyTarget(apiClient), buf, testCommandFormat(), testAllowWrites())
			},
			args:       []string{"42", "com.example.flag"},
			method:     http.MethodDelete,
			path:       "/rest/agile/1.0/board/42/properties/com.example.flag",
			response:   `{}`,
			wantOutput: `"deleted":true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("method = %q, want %q", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("path = %q, want %q", r.URL.Path, tt.path)
				}
				if tt.wantBody != nil {
					got := testhelpers.DecodeJSONBody(t, r)
					if !equalJSONMaps(got, tt.wantBody) {
						t.Errorf("request body = %#v, want %#v", got, tt.wantBody)
					}
				}
				testhelpers.WriteJSONResponse(t, w, tt.response)
			}))
			t.Cleanup(server.Close)

			basePrefix := "/rest/agile/1.0"
			if strings.HasPrefix(tt.path, "/rest/api/3") {
				basePrefix = "/rest/api/3"
			}
			runCommandAction(t, tt.cmd(testCommandClient(server.URL+basePrefix), &buf), tt.args...)
			if got := buf.String(); !bytes.Contains([]byte(got), []byte(tt.wantOutput)) {
				t.Errorf("output = %s, want substring %s", got, tt.wantOutput)
			}
		})
	}
}

func TestPropertyCommandsUsePopulatedClientRefAtExecution(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/properties" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/rest/api/3/issue/PROJ-1/properties")
		}
		testhelpers.WriteJSONResponse(t, w, `{"keys":[{"key":"com.example.flag"}]}`)
	}))
	t.Cleanup(server.Close)

	apiClient := &client.Ref{}
	cmd := propertyListCommand(issuePropertyTarget(apiClient), &buf, testCommandFormat())
	apiClient.Client = client.NewClient(
		"Basic token",
		client.WithBaseURL(server.URL+"/rest/api/3"),
		client.WithAgileBaseURL(server.URL+"/rest/agile/1.0"),
	)

	runCommandAction(t, cmd, "PROJ-1")
	if got := buf.String(); !strings.Contains(got, `"com.example.flag"`) {
		t.Errorf("output = %s, want property key", got)
	}
}

func TestParsePropertyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		valueJSON string
		wantErr   bool
	}{
		{name: "object", valueJSON: `{"enabled":true}`},
		{name: "string", valueJSON: `"enabled"`},
		{name: "number", valueJSON: `1`},
		{name: "empty", wantErr: true},
		{name: "invalid", valueJSON: `{`, wantErr: true},
		{name: "null", valueJSON: `null`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parsePropertyValue(tt.valueJSON)
			if tt.wantErr && err == nil {
				t.Fatalf("parsePropertyValue() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("parsePropertyValue() error = %v, want nil", err)
			}
		})
	}
}

func equalJSONMaps(got, want map[string]any) bool {
	gotJSON, gotErr := json.Marshal(got)
	wantJSON, wantErr := json.Marshal(want)
	return gotErr == nil && wantErr == nil && bytes.Equal(gotJSON, wantJSON)
}
