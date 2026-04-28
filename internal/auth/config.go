// Package auth handles configuration loading and Basic auth header construction
// for Jira Cloud REST API authentication.
//
// Configuration is resolved with the following precedence (highest first):
//  1. Environment variables (JIRA_INSTANCE, JIRA_EMAIL, JIRA_API_KEY)
//  2. Config file (~/.config/jira-agent/config.json, respects XDG_CONFIG_HOME)
//
// The API key and email are combined into a Basic auth header value:
//
//	Authorization: Basic base64(email:api_key)
package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrMissingCredentials is returned when required auth credentials are absent.
var ErrMissingCredentials = errors.New("missing required credentials")

var userHomeDir = os.UserHomeDir

// Config holds the Jira connection settings resolved from file and environment.
type Config struct {
	Instance       string `json:"instance"`
	Email          string `json:"email"`
	APIKey         string `json:"api_key"`
	DefaultProject string `json:"default_project,omitempty"`
	AllowWrites    bool   `json:"i-too-like-to-live-dangerously,omitempty"`
}

// BasicAuthHeader returns the base64-encoded Basic auth header value
// in the form "Basic base64(email:api_key)".
func (c *Config) BasicAuthHeader() string {
	credentials := c.Email + ":" + c.APIKey
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	return "Basic " + encoded
}

// BaseURL returns the Jira REST API v3 base URL derived from the instance.
// The instance field should be the Atlassian domain (e.g., "mycompany.atlassian.net").
func (c *Config) BaseURL() string {
	instance := strings.TrimRight(c.Instance, "/")
	if !strings.HasPrefix(instance, "https://") {
		instance = "https://" + instance
	}
	return instance + "/rest/api/3"
}

// AgileBaseURL returns the Jira Agile API base URL derived from the instance.
func (c *Config) AgileBaseURL() string {
	instance := strings.TrimRight(c.Instance, "/")
	if !strings.HasPrefix(instance, "https://") {
		instance = "https://" + instance
	}
	return instance + "/rest/agile/1.0"
}

// Validate checks that all required fields are present.
func (c *Config) Validate() error {
	missing := []string{}
	if c.Instance == "" {
		missing = append(missing, "instance")
	}
	if c.Email == "" {
		missing = append(missing, "email")
	}
	if c.APIKey == "" {
		missing = append(missing, "api_key")
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingCredentials, strings.Join(missing, ", "))
	}
	return nil
}

// LoadConfig reads the config file at path and applies environment variable
// overrides. Environment variables take precedence over file values.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	// Try to load from file (not an error if it doesn't exist).
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", path, err)
		}
	}

	// Environment variables override file values.
	if v := os.Getenv("JIRA_INSTANCE"); v != "" {
		cfg.Instance = v
	}
	if v := os.Getenv("JIRA_EMAIL"); v != "" {
		cfg.Email = v
	}
	if v := os.Getenv("JIRA_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("JIRA_ALLOW_WRITES"); v != "" {
		cfg.AllowWrites, _ = strconv.ParseBool(v)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// DefaultConfigPath returns the default config file location, respecting
// XDG_CONFIG_HOME. Falls back to ~/.config/jira-agent/config.json.
func DefaultConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := userHomeDir()
		if err != nil {
			return filepath.Join(".config", "jira-agent", "config.json")
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configHome, "jira-agent", "config.json")
}
