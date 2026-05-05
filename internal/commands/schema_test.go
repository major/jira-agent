package commands

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
	"github.com/spf13/cobra"
)

type schemaTestEnvelope struct {
	Data schemaPayload `json:"data"`
}

func TestSchemaCommandOutputsMachineReadableCommandTree(t *testing.T) {
	t.Parallel()

	format := output.FormatJSON
	root := &cobra.Command{Use: "jira-agent", Version: "test-version"}
	issue := &cobra.Command{Use: "issue", Short: "Issue operations", Example: "jira-agent issue"}
	search := &cobra.Command{Use: "search", Short: "Search issues with JQL", Example: `jira-agent issue search --jql "project = PROJ"`}
	search.Flags().String("jql", "", "JQL query string")
	search.Flags().String("fields-preset", "", "Named fields preset")
	if err := search.MarkFlagRequired("jql"); err != nil {
		t.Fatalf("MarkFlagRequired() error = %v", err)
	}
	markMutuallyExclusive(search, "jql", "fields-preset")
	hidden := &cobra.Command{Use: "hidden", Short: "Hidden command", Hidden: true}
	help := &cobra.Command{Use: "help", Short: "Help command"}

	issue.AddCommand(search, hidden)
	root.AddCommand(issue, help)
	SetCommandCategory(issue, commandCategoryRead)
	SetCommandCategory(search, commandCategoryRead)
	setCommandAnnotation(issue, commandAnnotationRequiresAuth, commandRequiresAuthTrue)
	setCommandAnnotation(search, commandAnnotationRequiresAuth, commandRequiresAuthTrue)

	var buf strings.Builder
	cmd := SchemaCommand(root, &buf, &format)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("SchemaCommand RunE error = %v", err)
	}

	got := decodeSchemaTestEnvelope(t, buf.String()).Data
	if got.Version != "test-version" {
		t.Errorf("version = %q, want test-version", got.Version)
	}
	if findSchemaCommand(got.Commands, "hidden") != nil {
		t.Error("hidden command appeared in schema")
	}
	if findSchemaCommand(got.Commands, "help") != nil {
		t.Error("help command appeared in schema")
	}

	entry := findSchemaCommand(got.Commands, "issue search")
	if entry == nil {
		t.Fatalf("issue search missing from schema commands: %#v", got.Commands)
	}
	if entry.Description != "Search issues with JQL" {
		t.Errorf("description = %q, want Search issues with JQL", entry.Description)
	}
	if entry.Category != commandCategoryRead {
		t.Errorf("category = %q, want %q", entry.Category, commandCategoryRead)
	}
	if entry.WriteProtected {
		t.Error("write_protected = true, want false")
	}
	if !entry.RequiresAuth {
		t.Error("requires_auth = false, want true")
	}
	if !slices.Equal(entry.Examples, []string{`jira-agent issue search --jql "project = PROJ"`}) {
		t.Errorf("examples = %#v, want single search example", entry.Examples)
	}

	flag := findSchemaFlag(entry.Flags, "--jql")
	if flag == nil {
		t.Fatalf("--jql flag missing from schema flags: %#v", entry.Flags)
	}
	if flag.Type != "string" || flag.Default != "" || !flag.Required || flag.Description != "JQL query string" {
		t.Errorf("--jql flag = %#v, want string required JQL flag", *flag)
	}
	if len(entry.FlagGroups) != 1 {
		t.Fatalf("flag_groups length = %d, want 1", len(entry.FlagGroups))
	}
	if entry.FlagGroups[0].Type != flagGroupTypeMutuallyExclusive {
		t.Errorf("flag group type = %q, want %q", entry.FlagGroups[0].Type, flagGroupTypeMutuallyExclusive)
	}
	if !slices.Equal(entry.FlagGroups[0].Flags, []string{"--jql", "--fields-preset"}) {
		t.Errorf("flag group flags = %#v, want prefixed flag names", entry.FlagGroups[0].Flags)
	}
}

func TestSchemaCommandIncludesLiveIssueCommandMetadata(t *testing.T) {
	t.Parallel()

	allowWrites := false
	dryRun := false
	format := output.FormatJSON
	apiClient := &client.Ref{}
	var commandOutput strings.Builder
	root := &cobra.Command{Use: "jira-agent", Version: "dev"}
	root.AddCommand(IssueCommand(apiClient, &commandOutput, &format, &allowWrites, &dryRun))
	MarkWriteProtectedCommands(root)

	var schemaOutput strings.Builder
	cmd := SchemaCommand(root, &schemaOutput, &format)
	root.AddCommand(cmd)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("SchemaCommand RunE error = %v", err)
	}

	got := decodeSchemaTestEnvelope(t, schemaOutput.String()).Data
	search := findSchemaCommand(got.Commands, "issue search")
	if search == nil {
		t.Fatal("issue search missing from live schema")
	}
	if findSchemaFlag(search.Flags, "--jql") == nil {
		t.Errorf("issue search flags = %#v, want --jql", search.Flags)
	}
	if search.Category != commandCategoryRead || search.WriteProtected || !search.RequiresAuth {
		t.Errorf("issue search metadata = category %q write %t auth %t", search.Category, search.WriteProtected, search.RequiresAuth)
	}

	transition := findSchemaCommand(got.Commands, "issue transition")
	if transition == nil {
		t.Fatal("issue transition missing from live schema")
	}
	if transition.Category != commandCategoryWrite || !transition.WriteProtected || !transition.RequiresAuth {
		t.Errorf("issue transition metadata = category %q write %t auth %t", transition.Category, transition.WriteProtected, transition.RequiresAuth)
	}
	if len(transition.FlagGroups) == 0 {
		t.Errorf("issue transition flag_groups = %#v, want at least one group", transition.FlagGroups)
	}
	if findSchemaCommand(got.Commands, "issue bulk-create") != nil {
		t.Error("hidden legacy issue bulk-create appeared in schema")
	}
	if schema := findSchemaCommand(got.Commands, "schema"); schema == nil || schema.RequiresAuth || schema.Category != commandCategoryDiscovery {
		t.Errorf("schema command metadata = %#v, want discovery no-auth command", schema)
	}
}

func decodeSchemaTestEnvelope(t *testing.T, raw string) schemaTestEnvelope {
	t.Helper()

	var envelope schemaTestEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, output %s", err, raw)
	}
	return envelope
}

func findSchemaCommand(commands []schemaCommand, path string) *schemaCommand {
	for i := range commands {
		if commands[i].Path == path {
			return &commands[i]
		}
	}
	return nil
}

func findSchemaFlag(flags []schemaFlag, name string) *schemaFlag {
	for i := range flags {
		if flags[i].Name == name {
			return &flags[i]
		}
	}
	return nil
}

func TestSchemaResolveCommands(t *testing.T) {
	t.Parallel()

	format := output.FormatJSON
	apiClient := &client.Ref{}
	var commandOutput strings.Builder
	root := &cobra.Command{Use: "jira-agent", Version: "dev"}
	root.AddCommand(ResolveCommand(apiClient, &commandOutput, &format))
	MarkWriteProtectedCommands(root)

	var schemaOutput strings.Builder
	cmd := SchemaCommand(root, &schemaOutput, &format)
	root.AddCommand(cmd)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("SchemaCommand RunE error = %v", err)
	}

	got := decodeSchemaTestEnvelope(t, schemaOutput.String()).Data

	// Verify all 5 resolve subcommands appear with discovery category
	resolveSubcommands := []string{"resolve board", "resolve field", "resolve sprint", "resolve transition", "resolve user"}
	for _, path := range resolveSubcommands {
		cmd := findSchemaCommand(got.Commands, path)
		if cmd == nil {
			t.Errorf("%q missing from schema commands", path)
			continue
		}
		if cmd.Category != commandCategoryDiscovery {
			t.Errorf("%q category = %q, want %q", path, cmd.Category, commandCategoryDiscovery)
		}
		if cmd.WriteProtected {
			t.Errorf("%q write_protected = true, want false", path)
		}
		if !cmd.RequiresAuth {
			t.Errorf("%q requires_auth = false, want true", path)
		}
	}
}
