package auth

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_BasicAuthHeader(t *testing.T) {
	t.Parallel()

	cfg := &Config{Email: "user@example.com", APIKey: "secret"}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:secret"))

	if got := cfg.BasicAuthHeader(); got != want {
		t.Errorf("BasicAuthHeader() = %q, want %q", got, want)
	}
}

func TestConfig_BaseURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		instance  string
		wantBase  string
		wantAgile string
	}{
		{
			name:      "domain only",
			instance:  "example.atlassian.net",
			wantBase:  "https://example.atlassian.net/rest/api/3",
			wantAgile: "https://example.atlassian.net/rest/agile/1.0",
		},
		{
			name:      "https with trailing slash",
			instance:  "https://example.atlassian.net/",
			wantBase:  "https://example.atlassian.net/rest/api/3",
			wantAgile: "https://example.atlassian.net/rest/agile/1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Instance: tt.instance}
			if got := cfg.BaseURL(); got != tt.wantBase {
				t.Errorf("BaseURL() = %q, want %q", got, tt.wantBase)
			}
			if got := cfg.AgileBaseURL(); got != tt.wantAgile {
				t.Errorf("AgileBaseURL() = %q, want %q", got, tt.wantAgile)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		wantMissing []string
	}{
		{
			name: "valid config",
			cfg:  Config{Instance: "example.atlassian.net", Email: "user@example.com", APIKey: "secret"},
		},
		{
			name:        "all required fields missing",
			cfg:         Config{},
			wantErr:     true,
			wantMissing: []string{"instance", "email", "api_key"},
		},
		{
			name:        "api key missing",
			cfg:         Config{Instance: "example.atlassian.net", Email: "user@example.com"},
			wantErr:     true,
			wantMissing: []string{"api_key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Validate() error = nil, want error")
				}
				if !errors.Is(err, ErrMissingCredentials) {
					t.Errorf("Validate() error = %v, want ErrMissingCredentials", err)
				}
				for _, field := range tt.wantMissing {
					if !strings.Contains(err.Error(), field) {
						t.Errorf("Validate() error = %q, want field %q", err.Error(), field)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestLoadConfig_FileAndEnvironmentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"instance":"file.atlassian.net","email":"file@example.com","api_key":"file-key","default_project":"FILE"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("JIRA_INSTANCE", "env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_API_KEY", "env-key")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.Instance != "env.atlassian.net" {
		t.Errorf("Instance = %q, want %q", cfg.Instance, "env.atlassian.net")
	}
	if cfg.Email != "env@example.com" {
		t.Errorf("Email = %q, want %q", cfg.Email, "env@example.com")
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
	if cfg.DefaultProject != "FILE" {
		t.Errorf("DefaultProject = %q, want %q", cfg.DefaultProject, "FILE")
	}
}

func TestLoadConfig_PartialEnvironmentPrecedence(t *testing.T) {
	fileConfig := `{"instance":"file.atlassian.net","email":"file@example.com","api_key":"file-key","default_project":"FILE"}`

	tests := []struct {
		name         string
		envInstance  string
		envEmail     string
		envAPIKey    string
		wantInstance string
		wantEmail    string
		wantAPIKey   string
	}{
		{
			name:         "instance from environment",
			envInstance:  "env.atlassian.net",
			wantInstance: "env.atlassian.net",
			wantEmail:    "file@example.com",
			wantAPIKey:   "file-key",
		},
		{
			name:         "email from environment",
			envEmail:     "env@example.com",
			wantInstance: "file.atlassian.net",
			wantEmail:    "env@example.com",
			wantAPIKey:   "file-key",
		},
		{
			name:         "api key from environment",
			envAPIKey:    "env-key",
			wantInstance: "file.atlassian.net",
			wantEmail:    "file@example.com",
			wantAPIKey:   "env-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(fileConfig), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			t.Setenv("JIRA_INSTANCE", tt.envInstance)
			t.Setenv("JIRA_EMAIL", tt.envEmail)
			t.Setenv("JIRA_API_KEY", tt.envAPIKey)

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}
			if cfg.Instance != tt.wantInstance {
				t.Errorf("Instance = %q, want %q", cfg.Instance, tt.wantInstance)
			}
			if cfg.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", cfg.Email, tt.wantEmail)
			}
			if cfg.APIKey != tt.wantAPIKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.wantAPIKey)
			}
			if cfg.DefaultProject != "FILE" {
				t.Errorf("DefaultProject = %q, want %q", cfg.DefaultProject, "FILE")
			}
		})
	}
}

func TestLoadConfig_EnvironmentOnly(t *testing.T) {
	t.Setenv("JIRA_INSTANCE", "env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_API_KEY", "env-key")

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.Instance != "env.atlassian.net" {
		t.Errorf("Instance = %q, want %q", cfg.Instance, "env.atlassian.net")
	}
	if cfg.Email != "env@example.com" {
		t.Errorf("Email = %q, want %q", cfg.Email, "env@example.com")
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
}

func TestLoadConfig_AllowWritesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"instance":"test.atlassian.net","email":"a@b.com","api_key":"k","i-too-like-to-live-dangerously":true}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if !cfg.AllowWrites {
		t.Error("AllowWrites = false, want true")
	}
}

func TestLoadConfig_AllowWritesDefaultsFalse(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"instance":"test.atlassian.net","email":"a@b.com","api_key":"k"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.AllowWrites {
		t.Error("AllowWrites = true, want false (default)")
	}
}

func TestLoadConfig_AllowWritesEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"instance":"test.atlassian.net","email":"a@b.com","api_key":"k"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("JIRA_ALLOW_WRITES", "true")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if !cfg.AllowWrites {
		t.Error("AllowWrites = false, want true (from env)")
	}
}

func TestLoadConfig_AllowWritesEnvInvalidValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"instance":"test.atlassian.net","email":"a@b.com","api_key":"k"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("JIRA_ALLOW_WRITES", "notabool")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg.AllowWrites {
		t.Error("AllowWrites = true, want false (invalid env value should be treated as false)")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "parse config file") {
		t.Errorf("LoadConfig() error = %q, want parse config file", err.Error())
	}
}

func TestDefaultConfigPath_UsesXDGConfigHome(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	want := filepath.Join(configHome, "jira-agent", "config.json")
	if got := DefaultConfigPath(); got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_UserHomeDirFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	original := userHomeDir
	userHomeDir = func() (string, error) {
		return "", errors.New("home unavailable")
	}
	t.Cleanup(func() {
		userHomeDir = original
	})

	want := filepath.Join(".config", "jira-agent", "config.json")
	if got := DefaultConfigPath(); got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}
