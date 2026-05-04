package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	commandpkg "github.com/major/jira-agent/internal/commands"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cliContractSnapshotPath = "cli_contract_snapshot.json"

const writeProtectedAnnotation = "jira-agent/write-protected"

// commandContract captures stable Cobra command metadata that agents rely on.
type commandContract struct {
	Path            string            `json:"path"`
	Use             string            `json:"use"`
	Aliases         []string          `json:"aliases,omitempty"`
	Short           string            `json:"short"`
	Long            string            `json:"long,omitempty"`
	Example         string            `json:"example,omitempty"`
	Hidden          bool              `json:"hidden,omitempty"`
	Deprecated      string            `json:"deprecated,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	LocalFlags      []flagContract    `json:"local_flags,omitempty"`
	PersistentFlags []flagContract    `json:"persistent_flags,omitempty"`
	Subcommands     []string          `json:"subcommands,omitempty"`
}

// flagContract captures stable flag metadata without runtime parse state.
type flagContract struct {
	Name        string              `json:"name"`
	Shorthand   string              `json:"shorthand,omitempty"`
	Type        string              `json:"type"`
	Default     string              `json:"default,omitempty"`
	Usage       string              `json:"usage"`
	Hidden      bool                `json:"hidden,omitempty"`
	Deprecated  string              `json:"deprecated,omitempty"`
	Annotations map[string][]string `json:"annotations,omitempty"`
}

// TestCLIContractSnapshot verifies the command metadata contract stays stable.
func TestCLIContractSnapshot(t *testing.T) {
	t.Parallel()

	app := buildApp(&bytes.Buffer{})
	contract := collectCommandContracts(app)
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		t.Fatalf("marshal CLI contract: %v", err)
	}
	data = append(data, '\n')

	snapshotPath := filepath.Join("testdata", cliContractSnapshotPath)
	if os.Getenv("UPDATE_SNAPSHOT") == "1" {
		if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
			t.Fatalf("create snapshot directory: %v", err)
		}
		if err := os.WriteFile(snapshotPath, data, 0o644); err != nil {
			t.Fatalf("write CLI contract snapshot: %v", err)
		}
	}

	want, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read CLI contract snapshot: %v", err)
	}
	if !bytes.Equal(want, data) {
		t.Fatalf("CLI contract snapshot mismatch; run UPDATE_SNAPSHOT=1 go test ./cmd/jira-agent -run TestCLIContractSnapshot and review the diff")
	}
}

// TestSkillCommandExamplesMatchCLIContract catches stale command paths in the
// embedded LLM skill docs before agents learn commands that no longer exist.
func TestSkillCommandExamplesMatchCLIContract(t *testing.T) {
	t.Parallel()

	app := buildApp(&bytes.Buffer{})
	validPaths := collectCommandPathSet(app)
	skillPaths, err := filepath.Glob(filepath.Join("..", "..", "skills", "jira-agent", "*.md"))
	if err != nil {
		t.Fatalf("glob skill docs: %v", err)
	}
	if len(skillPaths) == 0 {
		t.Fatal("glob skill docs found no markdown files")
	}

	examples := 0
	for _, skillPath := range skillPaths {
		data, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("read skill doc %s: %v", skillPath, err)
		}
		for lineNumber, commandLine := range skillCommandLines(string(data)) {
			examples++
			if _, ok := longestCommandPath(commandLine, validPaths); !ok {
				t.Errorf("%s:%d: command example %q does not match any CLI command path", skillPath, lineNumber, commandLine)
			}
		}
	}
	if examples == 0 {
		t.Fatal("skill docs contain no jira-agent command examples")
	}
}

// TestWriteProtectedCommandsAnnotated keeps the declarative mutation metadata
// in sync with the configured write-protected command list.
func TestWriteProtectedCommandsAnnotated(t *testing.T) {
	t.Parallel()

	app := buildApp(&bytes.Buffer{})
	contracts := collectCommandContracts(app)
	annotatedPaths := map[string]struct{}{}
	for index := range contracts {
		annotations := contracts[index].Annotations
		if annotations[writeProtectedAnnotation] == "true" {
			path := strings.TrimPrefix(contracts[index].Path, "jira-agent ")
			annotatedPaths[path] = struct{}{}
		}
	}

	wantPaths := commandpkg.WriteProtectedCommandPaths()
	if len(annotatedPaths) != len(wantPaths) {
		t.Errorf("write-protected annotation count = %d, want %d", len(annotatedPaths), len(wantPaths))
	}
	for _, path := range wantPaths {
		if _, ok := annotatedPaths[path]; !ok {
			t.Errorf("command %q is missing %s=true", path, writeProtectedAnnotation)
		}
	}
	for path := range annotatedPaths {
		if !slices.Contains(wantPaths, path) {
			t.Errorf("command %q has unexpected %s=true", path, writeProtectedAnnotation)
		}
	}
}

// collectCommandContracts walks the command tree and returns sorted contracts.
func collectCommandContracts(root *cobra.Command) []commandContract {
	var contracts []commandContract
	collectCommandContract(root, &contracts)
	slices.SortFunc(contracts, func(a, b commandContract) int {
		return compareStrings(a.Path, b.Path)
	})
	return contracts
}

// collectCommandPathSet returns every command path exposed by the Cobra tree.
func collectCommandPathSet(root *cobra.Command) map[string]struct{} {
	paths := map[string]struct{}{}
	contracts := collectCommandContracts(root)
	for index := range contracts {
		paths[contracts[index].Path] = struct{}{}
	}
	return paths
}

// collectCommandContract appends metadata for cmd and all of its children.
func collectCommandContract(cmd *cobra.Command, contracts *[]commandContract) {
	contract := commandContract{
		Path:            cmd.CommandPath(),
		Use:             cmd.Use,
		Aliases:         sortedStrings(cmd.Aliases),
		Short:           cmd.Short,
		Long:            cmd.Long,
		Example:         cmd.Example,
		Hidden:          cmd.Hidden,
		Deprecated:      cmd.Deprecated,
		Annotations:     sortedCommandAnnotations(cmd.Annotations),
		LocalFlags:      collectFlagContracts(cmd.LocalFlags()),
		PersistentFlags: collectFlagContracts(cmd.PersistentFlags()),
		Subcommands:     sortedCommandNames(cmd.Commands()),
	}
	*contracts = append(*contracts, contract)

	children := cmd.Commands()
	slices.SortFunc(children, func(a, b *cobra.Command) int {
		return compareStrings(a.CommandPath(), b.CommandPath())
	})
	for _, child := range children {
		collectCommandContract(child, contracts)
	}
}

// skillCommandLines extracts jira-agent invocations from bash fences in skill docs.
func skillCommandLines(markdown string) map[int]string {
	commands := map[int]string{}
	inBashFence := false
	for index, line := range strings.Split(markdown, "\n") {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inBashFence = trimmed == "```bash"
		case inBashFence:
			commandLine, ok := normalizeSkillCommandLine(trimmed)
			if ok {
				commands[lineNumber] = commandLine
			}
		}
	}
	return commands
}

// normalizeSkillCommandLine returns a single-line jira-agent invocation.
func normalizeSkillCommandLine(line string) (string, bool) {
	line = strings.TrimSuffix(line, " \\")
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "$ ")
	if !strings.HasPrefix(line, "jira-agent") {
		return "", false
	}
	return line, true
}

// longestCommandPath returns the deepest CLI command path used by commandLine.
func longestCommandPath(commandLine string, validPaths map[string]struct{}) (string, bool) {
	fields := strings.Fields(commandLine)
	for end := len(fields); end > 0; end-- {
		candidate := strings.Join(fields[:end], " ")
		if _, ok := validPaths[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// collectFlagContracts returns stable metadata for all flags in a flag set.
func collectFlagContracts(flags *pflag.FlagSet) []flagContract {
	if flags == nil {
		return nil
	}

	var contracts []flagContract
	flags.VisitAll(func(flag *pflag.Flag) {
		contracts = append(contracts, flagContract{
			Name:        flag.Name,
			Shorthand:   flag.Shorthand,
			Type:        flag.Value.Type(),
			Default:     normalizeFlagDefault(flag),
			Usage:       flag.Usage,
			Hidden:      flag.Hidden,
			Deprecated:  flag.Deprecated,
			Annotations: sortedAnnotations(flag.Annotations),
		})
	})
	slices.SortFunc(contracts, func(a, b flagContract) int {
		return compareStrings(a.Name, b.Name)
	})
	return contracts
}

// normalizeFlagDefault removes machine-local values from the golden contract.
func normalizeFlagDefault(flag *pflag.Flag) string {
	if flag.Name == "config" {
		return "${HOME}/.config/jira-agent/config.json"
	}
	return flag.DefValue
}

// sortedCommandAnnotations returns a deterministic copy of Cobra annotations.
func sortedCommandAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}

	sorted := make(map[string]string, len(annotations))
	maps.Copy(sorted, annotations)
	return sorted
}

// sortedAnnotations returns a deterministic copy of pflag annotations.
func sortedAnnotations(annotations map[string][]string) map[string][]string {
	if len(annotations) == 0 {
		return nil
	}

	sorted := make(map[string][]string, len(annotations))
	for key, values := range annotations {
		sortedValues := append([]string(nil), values...)
		slices.Sort(sortedValues)
		sorted[key] = sortedValues
	}
	return sorted
}

// sortedCommandNames returns sorted command names for a stable subcommand list.
func sortedCommandNames(commands []*cobra.Command) []string {
	if len(commands) == 0 {
		return nil
	}

	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name())
	}
	slices.Sort(names)
	return names
}

// sortedStrings returns a deterministic copy of values.
func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	sorted := append([]string(nil), values...)
	slices.Sort(sorted)
	return sorted
}

// compareStrings compares a and b for use with slices.SortFunc.
func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
