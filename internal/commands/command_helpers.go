package commands

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

const (
	commandAnnotationDefaultSubcommand = "jira-agent/default-subcommand"
	commandAnnotationWriteProtected    = "jira-agent/write-protected"
	commandAnnotationCategory          = "jira-agent/category"
	commandAnnotationRequiresAuth      = "jira-agent/requires-auth"

	commandCategoryRead      = "read"
	commandCategoryWrite     = "write"
	commandCategoryBulk      = "bulk"
	commandCategoryDiscovery = "discovery"
	commandCategoryWorkflow  = "workflow"
	commandCategoryAdmin     = "admin"

	commandRequiresAuthTrue  = "true"
	commandRequiresAuthFalse = "false"
)

var validCommandCategories = map[string]struct{}{
	commandCategoryRead:      {},
	commandCategoryWrite:     {},
	commandCategoryBulk:      {},
	commandCategoryDiscovery: {},
	commandCategoryWorkflow:  {},
	commandCategoryAdmin:     {},
}

func setCommandAnnotation(cmd *cobra.Command, key, value string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[key] = value
}

// SetCommandCategory annotates a command with an LLM routing category. It
// panics on invalid categories so command-tree construction fails loudly during
// tests instead of emitting ambiguous schema metadata later.
func SetCommandCategory(cmd *cobra.Command, category string) {
	if _, ok := validCommandCategories[category]; !ok {
		panic(fmt.Sprintf("invalid command category %q", category))
	}
	setCommandAnnotation(cmd, commandAnnotationCategory, category)
}

// MarkWriteProtected annotates a command as write-protected without changing
// its runtime guard. This keeps mutating command metadata discoverable by
// contract tests and future schema generators.
func MarkWriteProtected(cmd *cobra.Command) {
	setCommandAnnotation(cmd, commandAnnotationWriteProtected, "true")
}

// MarkWriteProtectedCommands annotates every known mutating command in a built
// command tree. Runtime write protection still lives in writeGuard and
// requireWriteAccess; this pass exposes that behavior for contract snapshots and
// future schema generators without changing command execution.
func MarkWriteProtectedCommands(root *cobra.Command) {
	rootPath := root.CommandPath()
	for _, cmd := range allCommands(root) {
		path := strings.TrimPrefix(cmd.CommandPath(), rootPath+" ")
		if cmd == root {
			setCommandAnnotation(cmd, commandAnnotationRequiresAuth, commandRequiresAuthFalse)
			continue
		}
		setCommandAnnotation(cmd, commandAnnotationRequiresAuth, commandRequiresAuthTrue)
		SetCommandCategory(cmd, commandCategoryForPath(path))
		if _, ok := writeProtectedCommandPaths[path]; ok {
			MarkWriteProtected(cmd)
		}
	}
}

// CommandCategories returns command paths grouped by LLM routing category in a
// deterministic order for contract tests and future schema generation.
func CommandCategories() map[string][]string {
	categories := map[string][]string{
		commandCategoryRead:      {},
		commandCategoryWrite:     {},
		commandCategoryBulk:      {},
		commandCategoryDiscovery: {},
		commandCategoryWorkflow:  {},
		commandCategoryAdmin:     {},
	}
	for _, path := range commandKnownPaths {
		category := commandCategoryForPath(path)
		categories[category] = append(categories[category], path)
	}
	for category := range categories {
		sort.Strings(categories[category])
	}
	return categories
}

func commandCategoryForPath(path string) string {
	if category, ok := commandCategoryPathOverrides[path]; ok {
		return category
	}
	if strings.HasPrefix(path, "issue bulk") {
		return commandCategoryBulk
	}
	if _, ok := writeProtectedCommandPaths[path]; ok {
		return commandCategoryWrite
	}
	if strings.HasPrefix(path, "workflow") || strings.HasPrefix(path, "status") {
		return commandCategoryWorkflow
	}
	if strings.HasPrefix(path, "audit") || strings.HasPrefix(path, "permission") || strings.HasPrefix(path, "time-tracking") || path == "server-info" {
		return commandCategoryAdmin
	}
	if strings.HasPrefix(path, "field") || strings.HasPrefix(path, "project") || strings.HasPrefix(path, "board") || strings.HasPrefix(path, "role") || strings.HasPrefix(path, "priority") || strings.HasPrefix(path, "resolution") || strings.HasPrefix(path, "issuetype") || strings.HasPrefix(path, "label") || strings.HasPrefix(path, "jql") {
		return commandCategoryDiscovery
	}
	return commandCategoryRead
}

// WriteProtectedCommandPaths returns the configured write-protected command
// paths in deterministic order for contract tests and schema generation checks.
func WriteProtectedCommandPaths() []string {
	paths := make([]string, 0, len(writeProtectedCommandPaths))
	for path := range writeProtectedCommandPaths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func allCommands(root *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{root}
	for _, child := range root.Commands() {
		commands = append(commands, allCommands(child)...)
	}
	return commands
}

var commandCategoryPathOverrides = map[string]string{
	"audit list":           commandCategoryAdmin,
	"audit records":        commandCategoryAdmin,
	"field option reorder": commandCategoryWrite,
	"issue bulk":           commandCategoryBulk,
	"issue transition":     commandCategoryWrite,
	"project list":         commandCategoryDiscovery,
	"server-info":          commandCategoryAdmin,
}

var commandKnownPaths = []string{
	"audit", "audit list", "backlog", "backlog list", "backlog move", "board", "board config", "board create", "board delete", "board epics", "board filter", "board get", "board issues", "board list", "board projects", "board property", "board property delete", "board property get", "board property list", "board property set", "board versions", "component", "component create", "component delete", "component get", "component issue-counts", "component list", "component update", "dashboard", "dashboard copy", "dashboard create", "dashboard delete", "dashboard gadgets", "dashboard get", "dashboard list", "dashboard update", "epic", "epic get", "epic issues", "epic move-issues", "epic orphans", "epic rank", "epic remove-issues", "field", "field context", "field context create", "field context delete", "field context list", "field context update", "field list", "field option", "field option create", "field option delete", "field option get", "field option list", "field option reorder", "field option update", "field search", "filter", "filter create", "filter default-share-scope", "filter default-share-scope get", "filter default-share-scope set", "filter delete", "filter favorites", "filter get", "filter list", "filter permissions", "filter share", "filter unshare", "filter update", "group", "group add-member", "group create", "group delete", "group get", "group list", "group member-picker", "group members", "group remove-member", "issue", "issue assign", "issue attachment", "issue attachment add", "issue attachment delete", "issue attachment get", "issue attachment list", "issue bulk", "issue bulk create", "issue bulk delete", "issue bulk edit", "issue bulk edit-fields", "issue bulk fetch", "issue bulk move", "issue bulk status", "issue bulk transition", "issue bulk transitions", "issue bulk-create", "issue bulk-delete", "issue bulk-edit", "issue bulk-edit-fields", "issue bulk-fetch", "issue bulk-move", "issue bulk-status", "issue bulk-transition", "issue bulk-transitions", "issue changelog", "issue changelog bulk-fetch", "issue changelog list-by-ids", "issue comment", "issue comment add", "issue comment delete", "issue comment edit", "issue comment get", "issue comment list", "issue comment list-by-ids", "issue count", "issue create", "issue delete", "issue edit", "issue get", "issue link", "issue link add", "issue link delete", "issue link get", "issue link list", "issue link types", "issue meta", "issue notify", "issue picker", "issue property", "issue property delete", "issue property get", "issue property list", "issue property set", "issue rank", "issue remote-link", "issue remote-link add", "issue remote-link delete", "issue remote-link edit", "issue remote-link get", "issue remote-link list", "issue search", "issue transition", "issue vote", "issue vote add", "issue vote get", "issue vote remove", "issue watcher", "issue watcher add", "issue watcher list", "issue watcher remove", "issue worklog", "issue worklog add", "issue worklog delete", "issue worklog deleted", "issue worklog edit", "issue worklog get", "issue worklog list", "issue worklog list-by-ids", "issue worklog updated", "issuetype", "issuetype get", "issuetype list", "issuetype project", "jql", "jql fields", "jql suggest", "jql validate", "label", "label list", "permission", "permission check", "permission list", "permission schemes", "permission schemes get", "permission schemes list", "permission schemes project", "priority", "priority list", "project", "project categories", "project categories get", "project categories list", "project get", "project list", "project property", "project property delete", "project property get", "project property list", "project property set", "project roles", "project roles add-actor", "project roles get", "project roles list", "project roles remove-actor", "resolution", "resolution list", "role", "role get", "role list", "server-info", "sprint", "sprint create", "sprint delete", "sprint get", "sprint issues", "sprint list", "sprint move-issues", "sprint property", "sprint property delete", "sprint property get", "sprint property list", "sprint property set", "sprint swap", "sprint update", "status", "status categories", "status get", "status list", "task", "task cancel", "task get", "time-tracking", "time-tracking get", "time-tracking options", "time-tracking options get", "time-tracking options set", "time-tracking providers", "time-tracking select", "user", "user get", "user groups", "user search", "version", "version create", "version delete", "version get", "version issue-counts", "version list", "version merge", "version move", "version unresolved-count", "version update", "workflow", "workflow get", "workflow list", "workflow scheme", "workflow scheme get", "workflow scheme list", "workflow scheme project", "workflow statuses", "workflow transition-rules",
}

var writeProtectedCommandPaths = map[string]struct{}{
	"backlog move":                   {},
	"board create":                   {},
	"board delete":                   {},
	"board property delete":          {},
	"board property set":             {},
	"component create":               {},
	"component delete":               {},
	"component update":               {},
	"dashboard copy":                 {},
	"dashboard create":               {},
	"dashboard delete":               {},
	"dashboard update":               {},
	"epic move-issues":               {},
	"epic rank":                      {},
	"epic remove-issues":             {},
	"field context create":           {},
	"field context delete":           {},
	"field context update":           {},
	"field option create":            {},
	"field option delete":            {},
	"field option reorder":           {},
	"field option update":            {},
	"filter create":                  {},
	"filter default-share-scope set": {},
	"filter delete":                  {},
	"filter share":                   {},
	"filter unshare":                 {},
	"filter update":                  {},
	"group add-member":               {},
	"group create":                   {},
	"group delete":                   {},
	"group remove-member":            {},
	"issue assign":                   {},
	"issue attachment add":           {},
	"issue attachment delete":        {},
	"issue bulk create":              {},
	"issue bulk delete":              {},
	"issue bulk edit":                {},
	"issue bulk move":                {},
	"issue bulk transition":          {},
	"issue bulk-create":              {},
	"issue bulk-delete":              {},
	"issue bulk-edit":                {},
	"issue bulk-move":                {},
	"issue bulk-transition":          {},
	"issue comment add":              {},
	"issue comment delete":           {},
	"issue comment edit":             {},
	"issue create":                   {},
	"issue delete":                   {},
	"issue edit":                     {},
	"issue link add":                 {},
	"issue link delete":              {},
	"issue notify":                   {},
	"issue property delete":          {},
	"issue property set":             {},
	"issue rank":                     {},
	"issue remote-link add":          {},
	"issue remote-link delete":       {},
	"issue remote-link edit":         {},
	"issue transition":               {},
	"issue vote add":                 {},
	"issue vote remove":              {},
	"issue watcher add":              {},
	"issue watcher remove":           {},
	"issue worklog add":              {},
	"issue worklog delete":           {},
	"issue worklog edit":             {},
	"project roles add-actor":        {},
	"project roles remove-actor":     {},
	"sprint create":                  {},
	"sprint delete":                  {},
	"sprint move-issues":             {},
	"sprint property delete":         {},
	"sprint property set":            {},
	"sprint swap":                    {},
	"sprint update":                  {},
	"task cancel":                    {},
	"time-tracking options set":      {},
	"time-tracking select":           {},
	"version create":                 {},
	"version delete":                 {},
	"version merge":                  {},
	"version move":                   {},
	"version update":                 {},
}

// requireWriteAccess returns a ValidationError when write operations are not
// enabled. The error message tells the caller how to enable writes.
func requireWriteAccess(allowWrites *bool) error {
	if allowWrites == nil || !*allowWrites {
		return apperr.NewValidationError(
			"write operations are not enabled",
			nil,
			apperr.WithDetails(
				`set "i-too-like-to-live-dangerously": true in config file or JIRA_ALLOW_WRITES=true env var to enable write operations`,
			),
		)
	}
	return nil
}

// writeGuard wraps a cobra action so it returns a write-protection error unless
// writes are explicitly enabled in the configuration.
func writeGuard(allowWrites *bool, action func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := requireWriteAccess(allowWrites); err != nil {
			return err
		}
		return action(cmd, args)
	}
}

// requireArgs extracts positional arguments in order and returns a
// ValidationError naming the first missing label.
func requireArgs(args []string, labels ...string) ([]string, error) {
	values := make([]string, 0, len(labels))
	for i, label := range labels {
		var value string
		if i < len(args) {
			value = args[i]
		}
		if value == "" {
			return nil, apperr.NewValidationError(label+" is required", nil)
		}
		values = append(values, value)
	}
	return values, nil
}

// requireArg extracts the first positional argument and returns a
// ValidationError if it is missing.
func requireArg(args []string, label string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return "", apperr.NewValidationError(label+" is required", nil)
	}
	return args[0], nil
}

// requireFlag returns a non-empty string flag value or a ValidationError using
// the repository's existing "--flag is required" message convention.
func requireFlag(cmd *cobra.Command, flagName string) (string, error) {
	return requireFlagWithDetails(cmd, flagName, "")
}

// requireFlagWithDetails returns a non-empty string flag value or a
// ValidationError with remediation details. It keeps command validation terse
// without changing the user-facing error contract.
func requireFlagWithDetails(cmd *cobra.Command, flagName, details string) (string, error) {
	value, _ := cmd.Flags().GetString(flagName)
	if value != "" {
		return value, nil
	}

	message := "--" + flagName + " is required"
	if details == "" {
		return "", apperr.NewValidationError(message, nil)
	}
	return "", apperr.NewValidationError(message, nil, apperr.WithDetails(details))
}

type apiResultFunc func(result any) error

// writeAPIResult runs an API call that writes into result, then emits the
// standard success envelope for commands that return a single result object.
func writeAPIResult(w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	return output.WriteResult(w, result, format)
}

// writeRawAPIResult preserves Jira's original JSON shape for single-result
// commands that expose an explicit raw-output flag.
func writeRawAPIResult(w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	return output.WriteRawSuccess(w, result, output.NewMetadata(), format)
}

// writePaginatedAPIResult runs an API call that writes into result, extracts
// Jira pagination metadata, then emits the standard success envelope.
func writePaginatedAPIResult(cmd *cobra.Command, w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	meta := extractPaginationMeta(cmd, result)
	return output.WriteSuccess(w, result, meta, format)
}

// writeRawPaginatedAPIResult preserves Jira's original JSON shape for commands
// with an explicit raw-output flag while still extracting envelope metadata.
func writeRawPaginatedAPIResult(cmd *cobra.Command, w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	meta := extractPaginationMeta(cmd, result)
	return output.WriteRawSuccess(w, result, meta, format)
}

// splitTrimmed splits s by comma and trims whitespace from each element,
// discarding empty strings.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseInt64List(s string) ([]int64, error) {
	parts := splitTrimmed(s)
	if len(parts) == 0 {
		return nil, apperr.NewValidationError("--ids must include at least one ID", nil)
	}

	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("invalid --ids ID %q", part),
				err,
			)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

type paginationFlagUsage struct {
	maxResults string
	startAt    string
}

func appendPaginationFlags(cmd *cobra.Command) {
	appendPaginationFlagsWithUsage(cmd, paginationFlagUsage{
		maxResults: "Page size",
		startAt:    "Pagination offset",
	})
}

func appendPaginationFlagsWithUsage(cmd *cobra.Command, usage paginationFlagUsage) {
	cmd.Flags().Int("max-results", 50, usage.maxResults)
	cmd.Flags().Int("start-at", 0, usage.startAt)
}

func addBoolParam(cmd *cobra.Command, params map[string]string, flagName, paramName string) {
	val, _ := cmd.Flags().GetBool(flagName)
	if val {
		params[paramName] = "true"
	}
}

// buildMaxResultsParams returns Jira query parameters for picker endpoints that
// support a page size but not an offset.
func buildMaxResultsParams(cmd *cobra.Command, optionalFlags map[string]string) map[string]string {
	maxResults, _ := cmd.Flags().GetInt("max-results")
	params := map[string]string{
		"maxResults": strconv.Itoa(maxResults),
	}
	addOptionalParams(cmd, params, optionalFlags)
	return params
}

// buildPaginationParams returns standard Jira pagination parameters and any
// optional string flags mapped from CLI flag name to REST query parameter name.
func buildPaginationParams(cmd *cobra.Command, optionalFlags map[string]string) map[string]string {
	maxResults, _ := cmd.Flags().GetInt("max-results")
	startAt, _ := cmd.Flags().GetInt("start-at")
	params := map[string]string{
		"maxResults": strconv.Itoa(maxResults),
		"startAt":    strconv.Itoa(startAt),
	}
	addOptionalParams(cmd, params, optionalFlags)
	return params
}

// addOptionalParams copies non-empty string flags into REST query parameters.
func addOptionalParams(cmd *cobra.Command, params, optionalFlags map[string]string) {
	for flagName, paramName := range optionalFlags {
		value, _ := cmd.Flags().GetString(flagName)
		if value != "" {
			params[paramName] = value
		}
	}
}

// extractPaginationMeta pulls total, startAt, maxResults, and item count
// from a standard Jira paginated response.
func extractPaginationMeta(cmd *cobra.Command, result any) output.Metadata {
	meta := output.NewMetadata()
	m, ok := result.(map[string]any)
	if !ok {
		return meta
	}
	if v, ok := m["total"].(float64); ok {
		meta.Total = int(v)
	}
	if v, ok := m["startAt"].(float64); ok {
		meta.StartAt = int(v)
	}
	if v, ok := m["offset"].(float64); ok {
		meta.StartAt = int(v)
	}
	if v, ok := m["maxResults"].(float64); ok {
		meta.MaxResults = int(v)
	}
	if v, ok := m["limit"].(float64); ok {
		meta.MaxResults = int(v)
	}
	for _, key := range []string{"issues", "values", "comments", "worklogs", "records"} {
		if items, ok := m[key].([]any); ok {
			meta.Returned = len(items)
			break
		}
	}
	meta.HasMore = meta.Total > meta.StartAt+meta.Returned
	if meta.HasMore {
		meta.NextCommand = buildNextPageCommand(cmd, meta.StartAt+meta.Returned)
	}
	return meta
}

func buildNextPageCommand(cmd *cobra.Command, nextStartAt int) string {
	if cmd == nil {
		return ""
	}

	parts := []string{cmd.CommandPath()}
	for _, arg := range cmd.Flags().Args() {
		parts = append(parts, shellQuoteFlagValue(arg))
	}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "start-at" {
			return
		}
		parts = append(parts, "--"+flag.Name)
		if flag.Value.Type() != "bool" || flag.Value.String() != "true" {
			parts = append(parts, shellQuoteFlagValue(flag.Value.String()))
		}
	})
	parts = append(parts, "--start-at", strconv.Itoa(nextStartAt))
	return strings.Join(parts, " ")
}

func shellQuoteFlagValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t\n\"'\\") {
		return strconv.Quote(value)
	}
	return value
}

func extractFieldArray(result map[string]any, fieldName string) ([]any, error) {
	fields, ok := result["fields"].(map[string]any)
	if !ok {
		return nil, apperr.NewJiraError("unexpected response: missing 'fields' object", nil)
	}
	items, ok := fields[fieldName].([]any)
	if !ok {
		return nil, apperr.NewJiraError(
			fmt.Sprintf("unexpected response: missing '%s' array", fieldName),
			nil,
		)
	}
	return items, nil
}

// requireVisibilityFlags returns the visibility type and value when both are
// set. If only one visibility flag is set, it returns a validation error.
func requireVisibilityFlags(cmd *cobra.Command) (visibilityType, visibilityValue string, err error) {
	visibilityType, _ = cmd.Flags().GetString("visibility-type")
	visibilityValue, _ = cmd.Flags().GetString("visibility-value")
	switch {
	case visibilityType == "" && visibilityValue == "":
		return "", "", nil
	case visibilityType == "":
		return "", "", apperr.NewValidationError(
			"--visibility-type is required when --visibility-value is set",
			nil,
		)
	case visibilityValue == "":
		return "", "", apperr.NewValidationError(
			"--visibility-value is required when --visibility-type is set",
			nil,
		)
	default:
		return visibilityType, visibilityValue, nil
	}
}

// appendQueryParams appends query parameters in sorted key order so generated
// paths are deterministic in tests and logs.
func appendQueryParams(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := url.Values{}
	for _, key := range keys {
		if params[key] != "" {
			values.Set(key, params[key])
		}
	}
	if len(values) == 0 {
		return path
	}

	return path + "?" + values.Encode()
}

// escapePathSegment escapes a user-provided value before it is interpolated
// into a Jira REST path. Query parameters are handled separately by
// appendQueryParams.
func escapePathSegment(value string) string {
	return url.PathEscape(value)
}

// setDefaultSubcommand configures parent to run childName's RunE when invoked
// without a subcommand. This replicates the previous DefaultCommand behavior
// for the common case where no positional args are passed to the parent.
func setDefaultSubcommand(parent *cobra.Command, childName string) {
	setCommandAnnotation(parent, commandAnnotationDefaultSubcommand, childName)
	parent.RunE = func(cmd *cobra.Command, args []string) error {
		for _, sub := range parent.Commands() {
			if sub.Name() == childName {
				return sub.RunE(sub, args)
			}
		}
		return apperr.NewValidationError(
			fmt.Sprintf("default subcommand %q not found in %q", childName, parent.Name()),
			nil,
		)
	}
}
