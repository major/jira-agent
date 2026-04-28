package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"
)

// SchemaOutput is the top-level schema introspection structure.
type SchemaOutput struct {
	Commands    map[string]CommandSchema `json:"commands"`
	GlobalFlags map[string]FlagSchema    `json:"global_flags"`
}

// CompactSchemaOutput is a token-efficient command index for LLM discovery.
// It intentionally omits examples and full flag objects. Agents can request a
// full command schema later with --command once they choose a command path.
type CompactSchemaOutput struct {
	Commands map[string]CompactCommandSchema `json:"commands"`
}

// CompactCommandSchema is a reduced command summary for command discovery.
type CompactCommandSchema struct {
	Canonical           string   `json:"canonical"`
	Description         string   `json:"description,omitempty"`
	Aliases             []string `json:"aliases,omitempty"`
	DefaultCommand      string   `json:"default_command,omitempty"`
	Children            []string `json:"children,omitempty"`
	ChildCount          int      `json:"child_count"`
	RequiredFlags       []string `json:"required_flags,omitempty"`
	Mutating            bool     `json:"mutating,omitempty"`
	RequiresWriteAccess bool     `json:"requires_write_access,omitempty"`
}

// CommandSchema describes a single CLI command in the schema.
type CommandSchema struct {
	Canonical           string                `json:"canonical"`
	Description         string                `json:"description"`
	Aliases             []string              `json:"aliases,omitempty"`
	DefaultCommand      string                `json:"default_command,omitempty"`
	Flags               map[string]FlagSchema `json:"flags"`
	Args                map[string]any        `json:"args"`
	Children            []string              `json:"children,omitempty"`
	Examples            []string              `json:"examples"`
	Mutating            bool                  `json:"mutating,omitempty"`
	RequiresWriteAccess bool                  `json:"requires_write_access,omitempty"`
}

// FlagSchema describes a single CLI flag in the schema.
type FlagSchema struct {
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases,omitempty"`
	MultiValue  bool     `json:"multi_value,omitempty"`
}

const (
	commandMetadataMutating            = "schema_mutating"
	commandMetadataRequiresWriteAccess = "schema_requires_write_access"
	commandMetadataRequiredFlags       = "schema_required_flags"
)

func writeCommandMetadata() map[string]any {
	return map[string]any{
		commandMetadataMutating:            true,
		commandMetadataRequiresWriteAccess: true,
	}
}

func requiredFlagMetadata(names ...string) map[string]any {
	return map[string]any{
		commandMetadataRequiredFlags: names,
	}
}

func commandMetadata(values ...map[string]any) map[string]any {
	metadata := map[string]any{}
	for _, value := range values {
		maps.Copy(metadata, value)
	}
	return metadata
}

// SchemaCommand returns the CLI command for schema introspection.
// It walks the provided app's command tree and emits a JSON description
// of all commands, flags, and their types. Output is raw JSON, not wrapped
// in the standard success/error envelope.
func SchemaCommand(app *cli.Command, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "Display JSON schema of all available commands",
		UsageText: `jira-agent schema
jira-agent schema --command "issue create"
jira-agent schema --command issue --depth 1
jira-agent schema --compact
jira-agent schema --category issue --required-only`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "command",
				Usage: "Filter to a single command path (e.g., \"issue create\")",
			},
			&cli.StringFlag{
				Name:  "category",
				Usage: "Filter to a top-level command category (e.g., issue)",
			},
			&cli.BoolFlag{
				Name:  "required-only",
				Usage: "Only include required flags in full schema output",
			},
			&cli.BoolFlag{
				Name:  "compact",
				Usage: "Emit a compact command index for token-efficient LLM discovery",
			},
			&cli.IntFlag{
				Name:  "depth",
				Usage: "Limit descendants below selected command roots; -1 means unlimited",
				Value: -1,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			commands := make(map[string]CommandSchema)
			walkCommands(app, "", commands)

			globalFlags := extractFlags(app.Flags)
			selectedRoots := []string{}
			if category := cmd.String("category"); category != "" {
				category = resolveCategoryPath(app, category)
				commands = filterCommandCategory(commands, category)
				if len(commands) == 0 {
					return fmt.Errorf("category %q not found", category)
				}
				selectedRoots = []string{category}
			}

			// Filter to a single command when --command is set.
			filter := cmd.String("command")
			if filter != "" {
				filter = resolveCommandPath(app, filter)
				commands = filterCommandSubtree(commands, filter)
				if len(commands) == 0 {
					return fmt.Errorf("command %q not found", filter)
				}
				selectedRoots = []string{filter}
			}
			commands = filterCommandDepth(commands, selectedRoots, cmd.Int("depth"))
			attachCommandChildren(commands)
			if cmd.Bool("required-only") {
				commands = filterRequiredCommandFlags(commands)
				globalFlags = filterRequiredFlags(globalFlags)
			}

			enc := json.NewEncoder(w)
			enc.SetEscapeHTML(false)
			if cmd.Bool("compact") {
				return enc.Encode(CompactSchemaOutput{Commands: compactCommands(commands)})
			}

			schema := SchemaOutput{
				Commands:    commands,
				GlobalFlags: globalFlags,
			}

			enc.SetIndent("", "  ")
			return enc.Encode(schema)
		},
	}
}

// filterCommandCategory keeps commands under one top-level command category.
func filterCommandCategory(commands map[string]CommandSchema, category string) map[string]CommandSchema {
	filtered := make(map[string]CommandSchema)
	for path := range commands {
		if path == category || strings.HasPrefix(path, category+" ") {
			filtered[path] = commands[path]
		}
	}
	return filtered
}

// filterCommandSubtree keeps the requested command and all descendants. This
// makes parent commands useful for discovery without forcing agents to guess
// every nested leaf path up front.
func filterCommandSubtree(commands map[string]CommandSchema, commandPath string) map[string]CommandSchema {
	filtered := make(map[string]CommandSchema)
	for path := range commands {
		if path == commandPath || strings.HasPrefix(path, commandPath+" ") {
			filtered[path] = commands[path]
		}
	}
	return filtered
}

// filterRequiredCommandFlags returns a copy with optional command flags omitted.
func filterRequiredCommandFlags(commands map[string]CommandSchema) map[string]CommandSchema {
	filtered := make(map[string]CommandSchema, len(commands))
	for path := range commands {
		schema := commands[path]
		schema.Flags = filterRequiredFlags(schema.Flags)
		filtered[path] = schema
	}
	return filtered
}

// filterRequiredFlags keeps only flags marked required.
func filterRequiredFlags(flags map[string]FlagSchema) map[string]FlagSchema {
	filtered := make(map[string]FlagSchema)
	for name, schema := range flags {
		if schema.Required {
			filtered[name] = schema
		}
	}
	return filtered
}

// compactCommands groups command paths by top-level command. Children are all
// compactCommands returns a reduced command map for deterministic, low-token
// discovery while avoiding full flag and example payloads.
func compactCommands(commands map[string]CommandSchema) map[string]CompactCommandSchema {
	compact := make(map[string]CompactCommandSchema, len(commands))
	for path := range commands {
		schema := commands[path]
		compact[path] = CompactCommandSchema{
			Canonical:           schema.Canonical,
			Description:         schema.Description,
			Aliases:             schema.Aliases,
			DefaultCommand:      schema.DefaultCommand,
			Children:            schema.Children,
			ChildCount:          len(schema.Children),
			RequiredFlags:       requiredFlagNames(schema.Flags),
			Mutating:            schema.Mutating,
			RequiresWriteAccess: schema.RequiresWriteAccess,
		}
	}
	return compact
}

func requiredFlagNames(flags map[string]FlagSchema) []string {
	names := make([]string, 0)
	for name, flag := range flags {
		if flag.Required {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// walkCommands recursively traverses the command tree and populates the
// commands map with space-separated command paths as keys.
func walkCommands(cmd *cli.Command, prefix string, commands map[string]CommandSchema) {
	for _, sub := range cmd.Commands {
		if sub.Name == "help" || sub.Hidden {
			continue
		}

		path := sub.Name
		if prefix != "" {
			path = prefix + " " + sub.Name
		}

		flags := extractFlags(sub.Flags)
		applySchemaRequiredFlags(flags, commandMetadataStrings(sub, commandMetadataRequiredFlags))
		writes := commandMetadataBool(sub, commandMetadataMutating) || writeCommandPath(path)
		requiresWrites := commandMetadataBool(sub, commandMetadataRequiresWriteAccess) || writeCommandPath(path)

		commands[path] = CommandSchema{
			Canonical:           path,
			Description:         sub.Usage,
			Aliases:             sub.Aliases,
			DefaultCommand:      sub.DefaultCommand,
			Flags:               flags,
			Args:                extractArgs(sub.ArgsUsage),
			Examples:            parseExamples(sub.UsageText),
			Mutating:            writes,
			RequiresWriteAccess: requiresWrites,
		}

		walkCommands(sub, path, commands)
	}
}

func applySchemaRequiredFlags(flags map[string]FlagSchema, required []string) {
	for _, name := range required {
		flagName := "--" + name
		flag, ok := flags[flagName]
		if !ok {
			continue
		}
		flag.Required = true
		flags[flagName] = flag
	}
}

func commandMetadataBool(cmd *cli.Command, key string) bool {
	if cmd.Metadata == nil {
		return false
	}
	value, ok := cmd.Metadata[key]
	if !ok {
		return false
	}
	result, _ := value.(bool)
	return result
}

func commandMetadataStrings(cmd *cli.Command, key string) []string {
	if cmd.Metadata == nil {
		return []string{}
	}
	value, ok := cmd.Metadata[key]
	if !ok {
		return []string{}
	}
	result, ok := value.([]string)
	if !ok {
		return []string{}
	}
	return result
}

func writeCommandPath(path string) bool {
	_, ok := writeCommandPaths()[path]
	return ok
}

func writeCommandPaths() map[string]struct{} {
	paths := []string{
		"backlog move",
		"board create",
		"board delete",
		"board property delete",
		"board property set",
		"component create",
		"component delete",
		"component update",
		"dashboard copy",
		"dashboard create",
		"dashboard delete",
		"dashboard update",
		"epic move-issues",
		"epic rank",
		"epic remove-issues",
		"field context create",
		"field context delete",
		"field context update",
		"field option create",
		"field option delete",
		"field option reorder",
		"field option update",
		"filter create",
		"filter default-share-scope set",
		"filter delete",
		"filter share",
		"filter unshare",
		"filter update",
		"group add-member",
		"group create",
		"group delete",
		"group remove-member",
		"issue assign",
		"issue attachment add",
		"issue attachment delete",
		"issue bulk create",
		"issue bulk delete",
		"issue bulk edit",
		"issue bulk move",
		"issue bulk transition",
		"issue comment add",
		"issue comment delete",
		"issue comment edit",
		"issue create",
		"issue delete",
		"issue edit",
		"issue link add",
		"issue link delete",
		"issue notify",
		"issue property delete",
		"issue property set",
		"issue rank",
		"issue remote-link add",
		"issue remote-link delete",
		"issue remote-link edit",
		"issue transition",
		"issue vote add",
		"issue vote remove",
		"issue watcher add",
		"issue watcher remove",
		"issue worklog add",
		"issue worklog delete",
		"issue worklog edit",
		"project property delete",
		"project property set",
		"project roles add-actor",
		"project roles remove-actor",
		"sprint create",
		"sprint delete",
		"sprint move-issues",
		"sprint property delete",
		"sprint property set",
		"sprint swap",
		"sprint update",
		"task cancel",
		"time-tracking options set",
		"time-tracking select",
		"version create",
		"version delete",
		"version merge",
		"version move",
		"version update",
	}
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[path] = struct{}{}
	}
	return result
}

func resolveCategoryPath(app *cli.Command, category string) string {
	for _, sub := range app.Commands {
		if sub.HasName(category) {
			return sub.Name
		}
	}
	return category
}

func resolveCommandPath(app *cli.Command, commandPath string) string {
	if canonical, ok := legacyCanonicalCommandPaths()[commandPath]; ok {
		return canonical
	}

	parts := strings.Fields(commandPath)
	if len(parts) == 0 {
		return commandPath
	}

	current := app
	canonical := make([]string, 0, len(parts))
	for _, part := range parts {
		var next *cli.Command
		for _, sub := range current.Commands {
			if sub.HasName(part) {
				next = sub
				break
			}
		}
		if next == nil {
			return commandPath
		}
		canonical = append(canonical, next.Name)
		current = next
	}
	return strings.Join(canonical, " ")
}

func legacyCanonicalCommandPaths() map[string]string {
	return map[string]string{
		"issue bulk-create":      "issue bulk create",
		"issue bulk-fetch":       "issue bulk fetch",
		"issue bulk-delete":      "issue bulk delete",
		"issue bulk-edit-fields": "issue bulk edit-fields",
		"issue bulk-edit":        "issue bulk edit",
		"issue bulk-move":        "issue bulk move",
		"issue bulk-transitions": "issue bulk transitions",
		"issue bulk-transition":  "issue bulk transition",
	}
}

func filterCommandDepth(commands map[string]CommandSchema, roots []string, depth int) map[string]CommandSchema {
	if depth < 0 {
		return commands
	}
	if len(roots) == 0 {
		roots = topLevelCommandPaths(commands)
	}

	filtered := make(map[string]CommandSchema, len(commands))
	for path := range commands {
		for _, root := range roots {
			if path == root || strings.HasPrefix(path, root+" ") {
				relative := strings.TrimPrefix(path, root)
				relative = strings.TrimSpace(relative)
				if commandDepth(relative) <= depth {
					filtered[path] = commands[path]
				}
				break
			}
		}
	}
	return filtered
}

func topLevelCommandPaths(commands map[string]CommandSchema) []string {
	roots := make([]string, 0)
	for path := range commands {
		if !strings.Contains(path, " ") {
			roots = append(roots, path)
		}
	}
	sort.Strings(roots)
	return roots
}

func commandDepth(relativePath string) int {
	if relativePath == "" {
		return 0
	}
	return len(strings.Fields(relativePath))
}

// attachCommandChildren annotates each command with its immediate child command
// paths. Full paths avoid ambiguity for repeated child names such as "list".
func attachCommandChildren(commands map[string]CommandSchema) {
	childrenByParent := make(map[string][]string)
	for path := range commands {
		lastSpace := strings.LastIndex(path, " ")
		if lastSpace == -1 {
			continue
		}

		parent := path[:lastSpace]
		if _, ok := commands[parent]; ok {
			childrenByParent[parent] = append(childrenByParent[parent], path)
		}
	}

	for parent, children := range childrenByParent {
		sort.Strings(children)
		schema := commands[parent]
		schema.Children = children
		commands[parent] = schema
	}
}

// extractArgs preserves urfave/cli positional argument usage in the schema.
// It does not try to parse the free-form grammar because commands use varied
// conventions like "<key>", "[field]", and comma-separated descriptions.
func extractArgs(argsUsage string) map[string]any {
	args := map[string]any{}
	argsUsage = strings.TrimSpace(argsUsage)
	if argsUsage == "" {
		return args
	}

	args["usage"] = argsUsage
	return args
}

// extractFlags converts CLI flag definitions to schema flag descriptions.
func extractFlags(flags []cli.Flag) map[string]FlagSchema {
	result := make(map[string]FlagSchema)
	for _, f := range flags {
		name, schema := classifyFlag(f)
		if name != "" && name != "help" {
			result["--"+name] = schema
		}
	}
	return result
}

// classifyFlag determines the type and properties of a single CLI flag
// via type assertion on concrete urfave/cli v3 flag types.
func classifyFlag(f cli.Flag) (string, FlagSchema) {
	switch tf := f.(type) {
	case *cli.StringFlag:
		return tf.Name, FlagSchema{
			Type:        "string",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
		}
	case *cli.StringSliceFlag:
		return tf.Name, FlagSchema{
			Type:        "string-slice",
			Required:    tf.Required,
			Default:     []string{},
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
			MultiValue:  true,
		}
	case *cli.IntFlag:
		return tf.Name, FlagSchema{
			Type:        "int",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
		}
	case *cli.Float64Flag:
		return tf.Name, FlagSchema{
			Type:        "float",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
		}
	case *cli.BoolFlag:
		return tf.Name, FlagSchema{
			Type:        "bool",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
		}
	case *cli.StringMapFlag:
		return tf.Name, FlagSchema{
			Type:        "string-map",
			Required:    tf.Required,
			Default:     nil,
			Description: tf.Usage,
			Aliases:     flagAliases(tf.Aliases),
			MultiValue:  true,
		}
	default:
		// Fall back to string for unknown flag types.
		names := f.Names()
		if len(names) == 0 {
			return "", FlagSchema{}
		}
		return names[0], FlagSchema{
			Type: "string",
		}
	}
}

func flagAliases(aliases []string) []string {
	result := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		prefix := "--"
		if len(alias) == 1 {
			prefix = "-"
		}
		result = append(result, prefix+alias)
	}
	return result
}

// parseExamples splits a UsageText string into individual example lines,
// trimming whitespace and dropping blanks. Returns an empty slice (not nil)
// when there are no examples so JSON output stays consistent.
func parseExamples(usageText string) []string {
	if strings.TrimSpace(usageText) == "" {
		return []string{}
	}

	lines := strings.Split(usageText, "\n")
	examples := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			examples = append(examples, trimmed)
		}
	}

	if len(examples) == 0 {
		return []string{}
	}

	return examples
}
