package commands

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

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

// writeGuard wraps a cli action so it returns a write-protection error unless
// writes are explicitly enabled in the configuration.
func writeGuard(allowWrites *bool, action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if err := requireWriteAccess(allowWrites); err != nil {
			return err
		}
		return action(ctx, cmd)
	}
}

// requireArgs extracts positional arguments in order and returns a
// ValidationError naming the first missing label.
func requireArgs(cmd *cli.Command, labels ...string) ([]string, error) {
	values := make([]string, 0, len(labels))
	for i, label := range labels {
		value := cmd.Args().Get(i)
		if value == "" {
			return nil, apperr.NewValidationError(label+" is required", nil)
		}
		values = append(values, value)
	}
	return values, nil
}

// requireArg extracts the first positional argument and returns a
// ValidationError if it is missing.
func requireArg(cmd *cli.Command, label string) (string, error) {
	v := cmd.Args().First()
	if v == "" {
		return "", apperr.NewValidationError(label+" is required", nil)
	}
	return v, nil
}

// requireFlag returns a non-empty string flag value or a ValidationError using
// the repository's existing "--flag is required" message convention.
func requireFlag(cmd *cli.Command, flagName string) (string, error) {
	return requireFlagWithDetails(cmd, flagName, "")
}

// requireFlagWithDetails returns a non-empty string flag value or a
// ValidationError with remediation details. It keeps command validation terse
// without changing the user-facing error contract.
func requireFlagWithDetails(cmd *cli.Command, flagName, details string) (string, error) {
	value := cmd.String(flagName)
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

func appendPaginationFlags(flags []cli.Flag) []cli.Flag {
	return appendPaginationFlagsWithUsage(flags, paginationFlagUsage{
		maxResults: "Page size",
		startAt:    "Pagination offset",
	})
}

func appendPaginationFlagsWithUsage(flags []cli.Flag, usage paginationFlagUsage) []cli.Flag {
	return append(flags,
		&cli.IntFlag{Name: "max-results", Usage: usage.maxResults, Value: 50},
		&cli.IntFlag{Name: "start-at", Usage: usage.startAt, Value: 0},
	)
}

func addBoolParam(cmd *cli.Command, params map[string]string, flagName, paramName string) {
	if cmd.Bool(flagName) {
		params[paramName] = "true"
	}
}

// buildMaxResultsParams returns Jira query parameters for picker endpoints that
// support a page size but not an offset.
func buildMaxResultsParams(cmd *cli.Command, optionalFlags map[string]string) map[string]string {
	params := map[string]string{
		"maxResults": strconv.Itoa(cmd.Int("max-results")),
	}
	addOptionalParams(cmd, params, optionalFlags)
	return params
}

// buildPaginationParams returns standard Jira pagination parameters and any
// optional string flags mapped from CLI flag name to REST query parameter name.
func buildPaginationParams(cmd *cli.Command, optionalFlags map[string]string) map[string]string {
	params := map[string]string{
		"maxResults": strconv.Itoa(cmd.Int("max-results")),
		"startAt":    strconv.Itoa(cmd.Int("start-at")),
	}
	addOptionalParams(cmd, params, optionalFlags)
	return params
}

// addOptionalParams copies non-empty string flags into REST query parameters.
func addOptionalParams(cmd *cli.Command, params, optionalFlags map[string]string) {
	for flagName, paramName := range optionalFlags {
		if value := cmd.String(flagName); value != "" {
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
func requireVisibilityFlags(cmd *cli.Command) (visibilityType, visibilityValue string, err error) {
	visibilityType = cmd.String("visibility-type")
	visibilityValue = cmd.String("visibility-value")
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
