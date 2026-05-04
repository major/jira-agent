package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cliContractSnapshotPath = "cli_contract_snapshot.json"

// commandContract captures stable Cobra command metadata that agents rely on.
type commandContract struct {
	Path            string         `json:"path"`
	Use             string         `json:"use"`
	Aliases         []string       `json:"aliases,omitempty"`
	Short           string         `json:"short"`
	Long            string         `json:"long,omitempty"`
	Example         string         `json:"example,omitempty"`
	Hidden          bool           `json:"hidden,omitempty"`
	Deprecated      string         `json:"deprecated,omitempty"`
	LocalFlags      []flagContract `json:"local_flags,omitempty"`
	PersistentFlags []flagContract `json:"persistent_flags,omitempty"`
	Subcommands     []string       `json:"subcommands,omitempty"`
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

// collectCommandContracts walks the command tree and returns sorted contracts.
func collectCommandContracts(root *cobra.Command) []commandContract {
	var contracts []commandContract
	collectCommandContract(root, &contracts)
	slices.SortFunc(contracts, func(a, b commandContract) int {
		return compareStrings(a.Path, b.Path)
	})
	return contracts
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
			Default:     flag.DefValue,
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
func compareStrings(a string, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
