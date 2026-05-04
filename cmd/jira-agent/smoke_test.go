package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	authExitCode       = 3
	validationExitCode = 5
)

func TestCLISmoke_VersionFlag(t *testing.T) {
	t.Parallel()

	result := runBuiltCLI(t, "--version")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; output = %q", result.exitCode, result.output)
	}
	if !strings.Contains(result.output, "jira-agent version") {
		t.Errorf("expected output to contain %q, got %q", "jira-agent version", result.output)
	}
}

func TestCLISmoke_UnknownCommandSuggestion(t *testing.T) {
	t.Parallel()

	// Cobra's legacyArgs validator catches unknown subcommands before RunE,
	// producing a plain error (exit code 1) with "Did you mean this?" format
	// instead of a typed ValidationError (exit code 5).
	result := runBuiltCLI(t, "whomai")
	if result.exitCode == 0 {
		t.Fatalf("exit code = 0, want non-zero; output = %q", result.output)
	}
	if !strings.Contains(result.output, "whomai") {
		t.Errorf("expected output to contain %q, got %q", "whomai", result.output)
	}
}

func TestCLISmoke_UnauthenticatedCommandFails(t *testing.T) {
	t.Parallel()

	result := runBuiltCLI(t, "whoami")
	if result.exitCode != authExitCode {
		t.Fatalf("exit code = %d, want %d; output = %q", result.exitCode, authExitCode, result.output)
	}
	if !strings.Contains(result.output, `"code":"AUTH_FAILED"`) {
		t.Errorf("expected output to contain %q, got %q", `"code":"AUTH_FAILED"`, result.output)
	}
}

func TestCLISmoke_VersionCommandRequiresAuth(t *testing.T) {
	t.Parallel()

	result := runBuiltCLI(t, "version", "list", "--project", "PROJ")
	if result.exitCode != authExitCode {
		t.Fatalf("exit code = %d, want %d; output = %q", result.exitCode, authExitCode, result.output)
	}
	if !strings.Contains(result.output, `"code":"AUTH_FAILED"`) {
		t.Errorf("expected output to contain %q, got %q", `"code":"AUTH_FAILED"`, result.output)
	}
}

type cliResult struct {
	output   string
	exitCode int
}

// runBuiltCLI compiles the CLI binary and runs it with the given args in an
// isolated environment (no Jira env vars, temp HOME/XDG_CONFIG_HOME).
func runBuiltCLI(t *testing.T, args ...string) cliResult {
	t.Helper()

	binaryPath := buildCLIBinary(t)
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = isolatedCLIEnv(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return cliResult{output: string(out)}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return cliResult{output: string(out), exitCode: exitErr.ExitCode()}
	}
	t.Fatalf("command failed without an exit status: %v; output = %q", err, out)
	return cliResult{}
}

// buildCLIBinary compiles the jira-agent binary into a temp directory.
func buildCLIBinary(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "jira-agent")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v; output = %q", err, out)
	}
	return binaryPath
}

// isolatedCLIEnv returns an environment with Jira/HOME/XDG vars replaced by
// temp directories, preventing real credentials from leaking into tests.
func isolatedCLIEnv(t *testing.T) []string {
	t.Helper()

	home := filepath.Join(t.TempDir(), "home")
	xdgConfigHome := filepath.Join(t.TempDir(), "xdg")
	return append(filterJiraEnv(os.Environ()),
		"HOME="+home,
		"XDG_CONFIG_HOME="+xdgConfigHome,
	)
}

// filterJiraEnv removes JIRA_, HOME, and XDG_CONFIG_HOME entries from env so
// tests run against a clean slate.
func filterJiraEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "JIRA_") || strings.HasPrefix(entry, "XDG_CONFIG_HOME=") || strings.HasPrefix(entry, "HOME=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
