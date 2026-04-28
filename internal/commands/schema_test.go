package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// testSchemaApp builds a small CLI app for schema testing with nested
// commands and various flag types.
func testSchemaApp() *cli.Command {
	return &cli.Command{
		Name:  "test-app",
		Usage: "A test application",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Output format",
				Value: "json",
			},
		},
		Commands: []*cli.Command{
			{
				Name:           "account",
				Aliases:        []string{"acct"},
				Usage:          "Account operations",
				DefaultCommand: "list",
				Commands: []*cli.Command{
					{
						Name:      "list",
						Usage:     "List all accounts",
						ArgsUsage: "<account-id> [field]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "format",
								Aliases: []string{"f"},
								Usage:   "Output format",
								Value:   "table",
							},
							&cli.BoolFlag{
								Name:     "all",
								Usage:    "Show all accounts",
								Required: true,
							},
						},
					},
				},
			},
			{
				Name:  "quote",
				Usage: "Get stock quotes",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "symbol",
						Usage:    "Stock symbol",
						Required: true,
					},
					&cli.IntFlag{
						Name:  "count",
						Usage: "Number of quotes",
						Value: 10,
					},
					&cli.Float64Flag{
						Name:  "threshold",
						Usage: "Price threshold",
						Value: 0.5,
					},
					&cli.StringMapFlag{
						Name:  "field",
						Usage: "Custom field (key=value)",
					},
					&cli.StringSliceFlag{
						Name:     "query",
						Aliases:  []string{"q"},
						Usage:    "Query terms",
						Required: true,
					},
				},
			},
		},
	}
}

func testFullSchemaApp() *cli.Command {
	apiClient := &client.Ref{}
	format := output.FormatJSON
	var buf bytes.Buffer
	return &cli.Command{
		Name: "jira-agent",
		Commands: []*cli.Command{
			BacklogCommand(apiClient, &buf, &format, testAllowWrites()),
			BoardCommand(apiClient, &buf, &format, testAllowWrites()),
			ComponentCommand(apiClient, &buf, &format, testAllowWrites()),
			DashboardCommand(apiClient, &buf, &format, testAllowWrites()),
			EpicCommand(apiClient, &buf, &format, testAllowWrites()),
			FieldCommand(apiClient, &buf, &format, testAllowWrites()),
			FilterCommand(apiClient, &buf, &format, testAllowWrites()),
			GroupCommand(apiClient, &buf, &format, testAllowWrites()),
			IssueCommand(apiClient, &buf, &format, testAllowWrites()),
			PermissionCommand(apiClient, &buf, &format),
			SprintCommand(apiClient, &buf, &format, testAllowWrites()),
			VersionCommand(apiClient, &buf, &format, testAllowWrites()),
		},
	}
}

// runSchema runs the schema command against the given app and returns the
// parsed SchemaOutput.
func runSchema(t *testing.T, app *cli.Command, args ...string) SchemaOutput {
	t.Helper()
	buf := runSchemaRaw(t, app, args...)

	var schema SchemaOutput
	if err := json.Unmarshal(buf, &schema); err != nil {
		t.Fatalf("failed to parse schema JSON: %v", err)
	}
	return schema
}

// runSchemaRaw runs the schema command and returns raw JSON bytes.
func runSchemaRaw(t *testing.T, app *cli.Command, args ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	cmdArgs := append([]string{"schema"}, args...)
	if err := schemaCmd.Run(context.Background(), cmdArgs); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	return buf.Bytes()
}

func TestSchemaCommand_FullOutput(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp())

	// All commands present (parent and leaf nodes).
	if got := len(schema.Commands); got != 3 {
		t.Fatalf("expected 3 commands, got %d", got)
	}
	for _, name := range []string{"account", "account list", "quote"} {
		if _, ok := schema.Commands[name]; !ok {
			t.Errorf("missing command %q", name)
		}
	}

	// Verify descriptions.
	tests := map[string]string{
		"account":      "Account operations",
		"account list": "List all accounts",
		"quote":        "Get stock quotes",
	}
	for cmd, want := range tests {
		if got := schema.Commands[cmd].Description; got != want {
			t.Errorf("command %q description = %q, want %q", cmd, got, want)
		}
	}
	if got := schema.Commands["account"].Canonical; got != "account" {
		t.Errorf("account canonical = %q, want %q", got, "account")
	}
	if got := schema.Commands["account"].DefaultCommand; got != "list" {
		t.Errorf("account default command = %q, want %q", got, "list")
	}
	if got := schema.Commands["account"].Aliases; len(got) != 1 || got[0] != "acct" {
		t.Errorf("account aliases = %v, want [acct]", got)
	}

	// Verify global flags.
	if got := len(schema.GlobalFlags); got != 2 {
		t.Fatalf("expected 2 global flags, got %d", got)
	}
	if got := schema.GlobalFlags["--verbose"].Type; got != "bool" {
		t.Errorf("--verbose type = %q, want %q", got, "bool")
	}
	if got := schema.GlobalFlags["--output"].Type; got != "string" {
		t.Errorf("--output type = %q, want %q", got, "string")
	}
	if got := schema.GlobalFlags["--output"].Default; got != "json" {
		t.Errorf("--output default = %v, want %q", got, "json")
	}
	if got := schema.GlobalFlags["--verbose"].Aliases; len(got) != 1 || got[0] != "-v" {
		t.Errorf("--verbose aliases = %v, want [-v]", got)
	}
}

func TestSchemaCommand_FlagTypes(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp())

	// String flag (required, empty default).
	symbolFlag := schema.Commands["quote"].Flags["--symbol"]
	if symbolFlag.Type != "string" {
		t.Errorf("--symbol type = %q, want %q", symbolFlag.Type, "string")
	}
	if !symbolFlag.Required {
		t.Error("--symbol should be required")
	}
	if symbolFlag.Default != "" {
		t.Errorf("--symbol default = %v, want empty", symbolFlag.Default)
	}
	if symbolFlag.Description != "Stock symbol" {
		t.Errorf("--symbol description = %q, want %q", symbolFlag.Description, "Stock symbol")
	}

	// Int flag (optional, non-zero default). JSON numbers decode as float64.
	countFlag := schema.Commands["quote"].Flags["--count"]
	if countFlag.Type != "int" {
		t.Errorf("--count type = %q, want %q", countFlag.Type, "int")
	}
	if countFlag.Required {
		t.Error("--count should not be required")
	}
	if countFlag.Default != float64(10) {
		t.Errorf("--count default = %v, want %v", countFlag.Default, float64(10))
	}

	// Float flag (optional, fractional default).
	thresholdFlag := schema.Commands["quote"].Flags["--threshold"]
	if thresholdFlag.Type != "float" {
		t.Errorf("--threshold type = %q, want %q", thresholdFlag.Type, "float")
	}
	if thresholdFlag.Default != 0.5 {
		t.Errorf("--threshold default = %v, want %v", thresholdFlag.Default, 0.5)
	}

	// Bool flag (required, false default).
	allFlag := schema.Commands["account list"].Flags["--all"]
	if allFlag.Type != "bool" {
		t.Errorf("--all type = %q, want %q", allFlag.Type, "bool")
	}
	if !allFlag.Required {
		t.Error("--all should be required")
	}
	if allFlag.Default != false {
		t.Errorf("--all default = %v, want false", allFlag.Default)
	}

	// StringMap flag.
	fieldFlag := schema.Commands["quote"].Flags["--field"]
	if fieldFlag.Type != "string-map" {
		t.Errorf("--field type = %q, want %q", fieldFlag.Type, "string-map")
	}
	if fieldFlag.Default != nil {
		t.Errorf("--field default = %v, want nil", fieldFlag.Default)
	}

	queryFlag := schema.Commands["quote"].Flags["--query"]
	if queryFlag.Type != "string-slice" {
		t.Errorf("--query type = %q, want %q", queryFlag.Type, "string-slice")
	}
	if !queryFlag.Required {
		t.Error("--query should be required")
	}
	if !queryFlag.MultiValue {
		t.Error("--query should be marked multi-value")
	}
	if got := queryFlag.Aliases; len(got) != 1 || got[0] != "-q" {
		t.Errorf("--query aliases = %v, want [-q]", got)
	}
}

func TestSchemaCommand_CommandPaginationFlagDescriptions(t *testing.T) {
	t.Parallel()

	format := testCommandFormat()
	var buf bytes.Buffer
	app := &cli.Command{
		Name: "test-app",
		Commands: []*cli.Command{
			ProjectCommand(&client.Ref{}, &buf, format),
			FieldCommand(&client.Ref{}, &buf, format, testAllowWrites()),
		},
	}
	schema := runSchema(t, app)

	tests := []struct {
		name           string
		command        string
		maxResultsDesc string
		startAtDesc    string
	}{
		{
			name:           "project list preserves custom descriptions",
			command:        "project list",
			maxResultsDesc: "Page size (max 100)",
			startAtDesc:    "Pagination offset",
		},
		{
			name:           "field search preserves custom descriptions",
			command:        "field search",
			maxResultsDesc: "Maximum number of results to return",
			startAtDesc:    "Index of the first result to return",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			flags := schema.Commands[tt.command].Flags
			if got := flags["--max-results"].Description; got != tt.maxResultsDesc {
				t.Errorf("--max-results description = %q, want %q", got, tt.maxResultsDesc)
			}
			if got := flags["--start-at"].Description; got != tt.startAtDesc {
				t.Errorf("--start-at description = %q, want %q", got, tt.startAtDesc)
			}
		})
	}
}

func TestSchemaCommand_FilterByCommand(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--command", "account list")

	// Only the filtered command appears.
	if got := len(schema.Commands); got != 1 {
		t.Fatalf("expected 1 command, got %d", got)
	}
	cmd, ok := schema.Commands["account list"]
	if !ok {
		t.Fatal("missing command \"account list\"")
	}
	if cmd.Description != "List all accounts" {
		t.Errorf("description = %q, want %q", cmd.Description, "List all accounts")
	}
	if got := len(cmd.Flags); got != 2 {
		t.Errorf("expected 2 flags, got %d", got)
	}

	// Global flags still present.
	if got := len(schema.GlobalFlags); got != 2 {
		t.Fatalf("expected 2 global flags, got %d", got)
	}
}

func TestSchemaCommand_FilterByParentCommandIncludesDescendants(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--command", "account")

	if got := len(schema.Commands); got != 2 {
		t.Fatalf("expected parent and child commands, got %d", got)
	}
	for _, name := range []string{"account", "account list"} {
		if _, ok := schema.Commands[name]; !ok {
			t.Errorf("missing command %q", name)
		}
	}
	if _, ok := schema.Commands["quote"]; ok {
		t.Error("unexpected sibling command in filtered schema")
	}
}

func TestSchemaCommand_FilterByCommandAlias(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--command", "acct list")

	if got := len(schema.Commands); got != 1 {
		t.Fatalf("expected 1 command, got %d", got)
	}
	if _, ok := schema.Commands["account list"]; !ok {
		t.Fatal("missing canonical command \"account list\"")
	}
}

func TestSchemaCommand_FilterByCategoryAlias(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--category", "acct")

	for _, name := range []string{"account", "account list"} {
		if _, ok := schema.Commands[name]; !ok {
			t.Errorf("missing command %q", name)
		}
	}
	if _, ok := schema.Commands["quote"]; ok {
		t.Error("unexpected quote command in account category")
	}
}

func TestSchemaCommand_DepthLimitsDescendants(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name: "order",
				Commands: []*cli.Command{
					{
						Name: "place",
						Commands: []*cli.Command{
							{Name: "equity"},
						},
					},
				},
			},
		},
	}
	schema := runSchema(t, app, "--command", "order", "--depth", "1")

	for _, path := range []string{"order", "order place"} {
		if _, ok := schema.Commands[path]; !ok {
			t.Errorf("missing command %q", path)
		}
	}
	if _, ok := schema.Commands["order place equity"]; ok {
		t.Error("unexpected grandchild command with depth 1")
	}
}

func TestTopLevelCommandPaths(t *testing.T) {
	t.Parallel()

	commands := map[string]CommandSchema{
		"issue get":       {},
		"board":           {},
		"issue":           {},
		"board configure": {},
		"version":         {},
	}

	got := topLevelCommandPaths(commands)
	want := []string{"board", "issue", "version"}
	if len(got) != len(want) {
		t.Fatalf("len(topLevelCommandPaths()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("topLevelCommandPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSchemaCommand_ParentCommandListsChildren(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--command", "account")

	children := schema.Commands["account"].Children
	if len(children) != 1 || children[0] != "account list" {
		t.Errorf("account children = %v, want [account list]", children)
	}
}

func TestSchemaCommand_ArgsUsage(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--command", "account list")

	usage, ok := schema.Commands["account list"].Args["usage"]
	if !ok {
		t.Fatal("missing args usage")
	}
	if usage != "<account-id> [field]" {
		t.Errorf("args usage = %v, want %q", usage, "<account-id> [field]")
	}
}

func TestSchemaCommand_FilterByCategory(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--category", "account")

	if got := len(schema.Commands); got != 2 {
		t.Fatalf("expected 2 account commands, got %d", got)
	}
	for _, name := range []string{"account", "account list"} {
		if _, ok := schema.Commands[name]; !ok {
			t.Errorf("missing command %q", name)
		}
	}
	if _, ok := schema.Commands["quote"]; ok {
		t.Error("unexpected quote command in account category")
	}
}

func TestSchemaCommand_RequiredOnly(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testSchemaApp(), "--required-only")

	quoteFlags := schema.Commands["quote"].Flags
	if got := len(quoteFlags); got != 2 {
		t.Fatalf("expected 2 required quote flags, got %d", got)
	}
	if _, ok := quoteFlags["--symbol"]; !ok {
		t.Error("missing required --symbol flag")
	}
	if _, ok := quoteFlags["--query"]; !ok {
		t.Error("missing required --query flag")
	}

	accountFlags := schema.Commands["account list"].Flags
	if got := len(accountFlags); got != 1 {
		t.Fatalf("expected 1 required account list flag, got %d", got)
	}
	if _, ok := accountFlags["--all"]; !ok {
		t.Error("missing required --all flag")
	}
}

func TestSchemaCommand_CompactOutput(t *testing.T) {
	t.Parallel()
	raw := runSchemaRaw(t, testSchemaApp(), "--compact")

	if bytes.Contains(raw, []byte("\n  ")) {
		t.Errorf("compact schema should not be indented: %q", raw)
	}
	for _, unexpected := range []string{"global_flags", "examples", "\"flags\""} {
		if bytes.Contains(raw, []byte(unexpected)) {
			t.Errorf("compact schema contains %q: %s", unexpected, raw)
		}
	}

	var schema CompactSchemaOutput
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("failed to parse compact schema JSON: %v", err)
	}
	account := schema.Commands["account"]
	if account.Canonical != "account" {
		t.Errorf("compact account canonical = %q, want %q", account.Canonical, "account")
	}
	if account.Description != "Account operations" {
		t.Errorf("compact account description = %q, want %q", account.Description, "Account operations")
	}
	if len(account.Children) != 1 || account.Children[0] != "account list" {
		t.Errorf("compact account children = %v, want [account list]", account.Children)
	}
	if account.ChildCount != 1 {
		t.Errorf("compact account child count = %d, want 1", account.ChildCount)
	}
	if got := account.Aliases; len(got) != 1 || got[0] != "acct" {
		t.Errorf("compact account aliases = %v, want [acct]", got)
	}
	if _, ok := schema.Commands["quote"]; !ok {
		t.Error("missing compact quote command")
	}
	quote := schema.Commands["quote"]
	if got := quote.RequiredFlags; len(got) != 2 || got[0] != "--query" || got[1] != "--symbol" {
		t.Errorf("compact quote required flags = %v, want [--query --symbol]", got)
	}
}

func TestSchemaCommand_CompactOutputIncludesNestedDescendants(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name: "order",
				Commands: []*cli.Command{
					{
						Name: "place",
						Commands: []*cli.Command{
							{Name: "equity"},
						},
					},
				},
			},
		},
	}

	raw := runSchemaRaw(t, app, "--compact")

	var schema CompactSchemaOutput
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("failed to parse compact schema JSON: %v", err)
	}
	want := []string{"order place"}
	got := schema.Commands["order"].Children
	if len(got) != len(want) {
		t.Fatalf("compact order children = %v, want %v", got, want)
	}
	for i, wantItem := range want {
		if got[i] != wantItem {
			t.Errorf("compact order children[%d] = %q, want %q", i, got[i], wantItem)
		}
	}
}

func TestSchemaCommand_CompactOutputRespectsDepth(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name: "order",
				Commands: []*cli.Command{
					{
						Name: "place",
						Commands: []*cli.Command{
							{Name: "equity"},
						},
					},
				},
			},
		},
	}
	raw := runSchemaRaw(t, app, "--command", "order", "--depth", "1", "--compact")

	var schema CompactSchemaOutput
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("failed to parse compact schema JSON: %v", err)
	}
	if _, ok := schema.Commands["order place equity"]; ok {
		t.Error("unexpected grandchild command with compact depth 1")
	}
	if got := schema.Commands["order"].ChildCount; got != 1 {
		t.Errorf("compact order child count = %d, want 1", got)
	}
}

func TestSchemaCommand_WriteMetadata(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name:     "create",
				Usage:    "Create a thing",
				Metadata: writeCommandMetadata(),
			},
			{
				Name:  "list",
				Usage: "List things",
			},
		},
	}
	schema := runSchema(t, app)

	if !schema.Commands["create"].Mutating {
		t.Error("create should be marked mutating")
	}
	if !schema.Commands["create"].RequiresWriteAccess {
		t.Error("create should require write access")
	}
	if schema.Commands["list"].Mutating {
		t.Error("list should not be marked mutating")
	}
}

func TestSchemaCommand_IssueBulkUsesCanonicalSubtree(t *testing.T) {
	t.Parallel()
	format := testCommandFormat()
	var buf bytes.Buffer
	app := &cli.Command{
		Name: "jira-agent",
		Commands: []*cli.Command{
			IssueCommand(&client.Ref{}, &buf, format, testAllowWrites()),
		},
	}
	schema := runSchema(t, app, "--command", "issue bulk", "--required-only")

	for _, path := range []string{
		"issue bulk",
		"issue bulk create",
		"issue bulk fetch",
		"issue bulk delete",
		"issue bulk edit-fields",
		"issue bulk edit",
		"issue bulk move",
		"issue bulk transitions",
		"issue bulk transition",
	} {
		if _, ok := schema.Commands[path]; !ok {
			t.Errorf("missing command %q", path)
		}
	}
	if _, ok := schema.Commands["issue bulk-create"]; ok {
		t.Error("legacy issue bulk-create should not appear in canonical schema subtree")
	}
	create := schema.Commands["issue bulk create"]
	if !create.Mutating || !create.RequiresWriteAccess {
		t.Error("issue bulk create should be marked write-gated")
	}
	if _, ok := create.Flags["--issues-json"]; !ok {
		t.Error("issue bulk create should expose required --issues-json")
	}
}

func TestSchemaCommand_LegacyIssueBulkCommandResolvesCanonical(t *testing.T) {
	t.Parallel()
	format := testCommandFormat()
	var buf bytes.Buffer
	app := &cli.Command{
		Name: "jira-agent",
		Commands: []*cli.Command{
			IssueCommand(&client.Ref{}, &buf, format, testAllowWrites()),
		},
	}
	schema := runSchema(t, app, "--command", "issue bulk-create")

	if _, ok := schema.Commands["issue bulk create"]; !ok {
		t.Error("legacy issue bulk-create should resolve to issue bulk create")
	}
	if _, ok := schema.Commands["issue bulk-create"]; ok {
		t.Error("legacy issue bulk-create should not be emitted")
	}
}

func TestSchemaCommand_IssueRemoteLinkAliasResolvesCanonical(t *testing.T) {
	t.Parallel()
	format := testCommandFormat()
	var buf bytes.Buffer
	app := &cli.Command{
		Name: "jira-agent",
		Commands: []*cli.Command{
			IssueCommand(&client.Ref{}, &buf, format, testAllowWrites()),
		},
	}
	schema := runSchema(t, app, "--command", "issue remotelink")

	remoteLink, ok := schema.Commands["issue remote-link"]
	if !ok {
		t.Fatal("missing canonical issue remote-link command")
	}
	if got := remoteLink.Aliases; len(got) != 1 || got[0] != "remotelink" {
		t.Errorf("remote-link aliases = %v, want [remotelink]", remoteLink.Aliases)
	}
	if _, ok := schema.Commands["issue remotelink"]; ok {
		t.Error("legacy issue remotelink should not be emitted as a canonical path")
	}
	if !schema.Commands["issue remote-link add"].RequiresWriteAccess {
		t.Error("issue remote-link add should be marked write-gated")
	}
}

func TestSchemaCommand_IssueLinkWriteMetadata(t *testing.T) {
	t.Parallel()
	format := testCommandFormat()
	var buf bytes.Buffer
	app := &cli.Command{
		Name: "jira-agent",
		Commands: []*cli.Command{
			IssueCommand(&client.Ref{}, &buf, format, testAllowWrites()),
		},
	}
	schema := runSchema(t, app, "--command", "issue link")

	for _, path := range []string{"issue link add", "issue link delete"} {
		cmd := schema.Commands[path]
		if !cmd.Mutating || !cmd.RequiresWriteAccess {
			t.Errorf("%s should be marked write-gated", path)
		}
	}
	if schema.Commands["issue link list"].Mutating {
		t.Error("issue link list should not be marked mutating")
	}
}

func TestSchemaCommand_KnownWriteCommandsExposeWriteMetadata(t *testing.T) {
	t.Parallel()
	schema := runSchema(t, testFullSchemaApp())

	for path := range writeCommandPaths() {
		cmd, ok := schema.Commands[path]
		if !ok {
			t.Errorf("write command path %q missing from schema", path)
			continue
		}
		if !cmd.Mutating || !cmd.RequiresWriteAccess {
			t.Errorf("%s write metadata = mutating:%v requires_write_access:%v, want both true", path, cmd.Mutating, cmd.RequiresWriteAccess)
		}
	}
}

func TestSchemaCommand_SkipsHelpCommands(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "help", Usage: "show help"},
			&cli.BoolFlag{Name: "verbose", Usage: "show verbose output"},
		},
		Commands: []*cli.Command{
			{Name: "help", Usage: "Shows a list of commands or help for one command"},
			{
				Name: "issue",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "help", Usage: "show help"},
					&cli.BoolFlag{Name: "verbose", Usage: "show verbose output"},
				},
				Commands: []*cli.Command{
					{Name: "help", Usage: "Shows a list of commands or help for one command"},
					{Name: "get", Usage: "Get issue"},
				},
			},
		},
	}
	schema := runSchema(t, app)

	for _, unexpected := range []string{"help", "issue help"} {
		if _, ok := schema.Commands[unexpected]; ok {
			t.Errorf("unexpected help command %q", unexpected)
		}
	}
	if _, ok := schema.Commands["issue get"]; !ok {
		t.Error("missing non-help command")
	}
	if _, ok := schema.GlobalFlags["--help"]; ok {
		t.Error("unexpected global --help flag")
	}
	if _, ok := schema.Commands["issue"].Flags["--help"]; ok {
		t.Error("unexpected command --help flag")
	}
	if _, ok := schema.GlobalFlags["--verbose"]; !ok {
		t.Error("missing non-help global flag")
	}
	if _, ok := schema.Commands["issue"].Flags["--verbose"]; !ok {
		t.Error("missing non-help command flag")
	}
}

func TestSchemaCommand_FilterNotFound(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(testSchemaApp(), &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema", "--command", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent command filter")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "nonexistent") || !strings.Contains(errMsg, "not found") {
		t.Errorf("error = %q, want it to contain %q and %q", errMsg, "nonexistent", "not found")
	}
}

func TestSchemaCommand_EmptyApp(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name:  "empty",
		Usage: "An empty application",
	}
	schema := runSchema(t, app)

	if len(schema.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(schema.Commands))
	}
	if len(schema.GlobalFlags) != 0 {
		t.Errorf("expected 0 global flags, got %d", len(schema.GlobalFlags))
	}
}

func TestSchemaCommand_NestedCommandPath(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name:  "order",
				Usage: "Order operations",
				Commands: []*cli.Command{
					{
						Name:  "place",
						Usage: "Place an order",
						Commands: []*cli.Command{
							{
								Name:  "equity",
								Usage: "Place an equity order",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:     "symbol",
										Usage:    "Stock symbol",
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	schema := runSchema(t, app)

	// All levels appear with space-separated paths.
	for _, path := range []string{"order", "order place", "order place equity"} {
		if _, ok := schema.Commands[path]; !ok {
			t.Errorf("missing command %q", path)
		}
	}

	// Deepest command has the flag.
	equityCmd := schema.Commands["order place equity"]
	if equityCmd.Description != "Place an equity order" {
		t.Errorf("description = %q, want %q", equityCmd.Description, "Place an equity order")
	}
	symbolFlag, ok := equityCmd.Flags["--symbol"]
	if !ok {
		t.Fatal("missing --symbol flag on equity command")
	}
	if !symbolFlag.Required {
		t.Error("--symbol should be required")
	}
}

func TestClassifyFlag_UnknownType_FallsBackToString(t *testing.T) {
	t.Parallel()
	f := &cli.UintFlag{Name: "retries", Usage: "retry count"}
	name, schema := classifyFlag(f)
	if name != "retries" {
		t.Errorf("name = %q, want %q", name, "retries")
	}
	if schema.Type != "string" {
		t.Errorf("type = %q, want %q (unknown flag types fall back to string)", schema.Type, "string")
	}
}

func TestSchemaCommand_RawJSONOutput(t *testing.T) {
	t.Parallel()
	// Verify schema outputs raw JSON, not wrapped in the standard envelope.
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(testSchemaApp(), &buf)

	if err := schemaCmd.Run(context.Background(), []string{"schema"}); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	// Parse raw JSON and verify top-level keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse raw JSON: %v", err)
	}

	if _, ok := raw["commands"]; !ok {
		t.Error("missing top-level key \"commands\"")
	}
	if _, ok := raw["global_flags"]; !ok {
		t.Error("missing top-level key \"global_flags\"")
	}
	// Must NOT have envelope keys.
	for _, key := range []string{"data", "metadata", "error"} {
		if _, ok := raw[key]; ok {
			t.Errorf("unexpected envelope key %q in raw schema output", key)
		}
	}
}

func TestParseExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		usageText string
		want      []string
	}{
		{
			name:      "empty",
			usageText: "",
			want:      []string{},
		},
		{
			name:      "whitespace only",
			usageText: " \n\t ",
			want:      []string{},
		},
		{
			name:      "trims and drops blanks",
			usageText: "  jira-agent schema\n\n\tjira-agent schema --command issue\n",
			want: []string{
				"jira-agent schema",
				"jira-agent schema --command issue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseExamples(tt.usageText)
			if len(got) != len(tt.want) {
				t.Fatalf("parseExamples() length = %d, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("parseExamples()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
