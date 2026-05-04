package commands

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

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
func writePaginatedAPIResult(w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	meta := extractPaginationMeta(result)
	return output.WriteSuccess(w, result, meta, format)
}

// writeRawPaginatedAPIResult preserves Jira's original JSON shape for commands
// with an explicit raw-output flag while still extracting envelope metadata.
func writeRawPaginatedAPIResult(w io.Writer, format output.Format, call apiResultFunc) error {
	var result any
	if err := call(&result); err != nil {
		return err
	}
	meta := extractPaginationMeta(result)
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
func extractPaginationMeta(result any) output.Metadata {
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
	return meta
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
