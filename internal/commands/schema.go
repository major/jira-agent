package commands

import (
	"io"
	"sort"
	"strings"

	"github.com/major/jira-agent/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type schemaPayload struct {
	Version  string          `json:"version"`
	Commands []schemaCommand `json:"commands"`
}

type schemaCommand struct {
	Path           string          `json:"path"`
	Description    string          `json:"description"`
	Category       string          `json:"category"`
	WriteProtected bool            `json:"write_protected"`
	RequiresAuth   bool            `json:"requires_auth"`
	Flags          []schemaFlag    `json:"flags"`
	FlagGroups     []flagGroupInfo `json:"flag_groups"`
	Examples       []string        `json:"examples"`
}

type schemaFlag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// SchemaCommand returns a no-auth command that emits the live Cobra command
// tree as machine-readable JSON for LLM command discovery.
func SchemaCommand(rootCmd *cobra.Command, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "Output machine-readable CLI schema",
		Example: "jira-agent schema",
		RunE: func(_ *cobra.Command, _ []string) error {
			meta := output.NewMetadata()
			return output.WriteSuccess(w, buildSchemaPayload(rootCmd), meta, *format)
		},
	}
	SetCommandCategory(cmd, commandCategoryDiscovery)
	setCommandAnnotation(cmd, commandAnnotationRequiresAuth, commandRequiresAuthFalse)
	return cmd
}

func buildSchemaPayload(rootCmd *cobra.Command) schemaPayload {
	version := rootCmd.Version
	if version == "" {
		version = "dev"
	}

	commands := make([]schemaCommand, 0)
	rootPath := rootCmd.CommandPath()
	for _, cmd := range allCommands(rootCmd) {
		if cmd == rootCmd || cmd.Hidden || cmd.Name() == "help" {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), rootPath))
		commands = append(commands, schemaCommand{
			Path:           path,
			Description:    cmd.Short,
			Category:       cmd.Annotations[commandAnnotationCategory],
			WriteProtected: cmd.Annotations[commandAnnotationWriteProtected] == "true",
			RequiresAuth:   cmd.Annotations[commandAnnotationRequiresAuth] != commandRequiresAuthFalse,
			Flags:          schemaFlags(cmd),
			FlagGroups:     schemaFlagGroups(cmd),
			Examples:       schemaExamples(cmd),
		})
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Path < commands[j].Path
	})

	return schemaPayload{Version: version, Commands: commands}
}

func schemaFlags(cmd *cobra.Command) []schemaFlag {
	flags := make([]schemaFlag, 0)
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, schemaFlag{
			Name:        "--" + flag.Name,
			Shorthand:   flag.Shorthand,
			Type:        flag.Value.Type(),
			Default:     flag.DefValue,
			Required:    schemaFlagRequired(flag),
			Description: flag.Usage,
		})
	})
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})
	return flags
}

func schemaFlagRequired(flag *pflag.Flag) bool {
	values := flag.Annotations[cobra.BashCompOneRequiredFlag]
	return len(values) > 0 && values[0] == "true"
}

func schemaFlagGroups(cmd *cobra.Command) []flagGroupInfo {
	groups := FlagGroups(cmd)
	if len(groups) == 0 {
		return nil
	}
	formatted := make([]flagGroupInfo, 0, len(groups))
	for _, group := range groups {
		flags := make([]string, 0, len(group.Flags))
		for _, flag := range group.Flags {
			flags = append(flags, "--"+strings.TrimPrefix(flag, "--"))
		}
		formatted = append(formatted, flagGroupInfo{Type: group.Type, Flags: flags})
	}
	return formatted
}

func schemaExamples(cmd *cobra.Command) []string {
	lines := strings.Split(cmd.Example, "\n")
	examples := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			examples = append(examples, trimmed)
		}
	}
	return examples
}
