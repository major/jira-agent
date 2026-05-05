package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/output"
)

var flatIssueNoiseFields = map[string]struct{}{
	"accountId":      {},
	"accountType":    {},
	"active":         {},
	"avatarUrls":     {},
	"expand":         {},
	"iconUrl":        {},
	"self":           {},
	"statusCategory": {},
	"timeZone":       {},
}

func isJSONOutputFormat(format output.Format) bool {
	return format == output.FormatJSON || format == output.FormatJSONPretty
}

// buildIssueSearchBody constructs the JQL search request body from command
// flags, including ORDER BY, fields, expand, and pagination options. Returns
// the resolved JQL string and the request body map.
func buildIssueSearchBody(cmd *cobra.Command) (jql string, body map[string]any, err error) {
	jql, err = requireFlag(cmd, "jql")
	if err != nil {
		return "", nil, err
	}

	if orderBy := mustGetString(cmd, "order-by"); orderBy != "" {
		direction := strings.ToUpper(mustGetString(cmd, "order"))
		if direction == "" {
			direction = "ASC"
		}
		jql += " ORDER BY " + orderBy + " " + direction
	}

	body = map[string]any{
		"jql":        jql,
		"maxResults": mustGetInt(cmd, "max-results"),
	}

	if t := mustGetString(cmd, "next-page-token"); t != "" {
		body["nextPageToken"] = t
	}
	if f := issueSearchFields(mustGetString(cmd, "fields")); cmd.Flags().Changed("fields") || len(f) > 0 {
		body["fields"] = f
	}
	if e := mustGetString(cmd, "expand"); e != "" {
		body["expand"] = e
	}
	if properties := splitTrimmed(mustGetString(cmd, "properties")); len(properties) > 0 {
		body["properties"] = properties
	}
	if mustGetBool(cmd, "fields-by-keys") {
		body["fieldsByKeys"] = true
	}
	if cmd.Flags().Changed("fail-fast") {
		body["failFast"] = mustGetBool(cmd, "fail-fast")
	}
	if reconcile := splitTrimmed(mustGetString(cmd, "reconcile-issues")); len(reconcile) > 0 {
		body["reconcileIssues"] = reconcile
	}

	return jql, body, nil
}

func issueSearchFields(fields string) []string {
	requested := splitTrimmed(fields)
	apiFields := make([]string, 0, len(requested))
	for _, field := range requested {
		if field == "key" {
			continue
		}
		apiFields = append(apiFields, field)
	}
	return apiFields
}

func flattenIssueSearchResult(result any) any {
	return flattenIssueSearchResultWithDescriptionFormat(result, descriptionOutputFormatText)
}

func flattenIssueSearchResultWithDescriptionFormat(result any, descriptionFormat string) any {
	response, ok := result.(map[string]any)
	if !ok {
		return result
	}

	flat := map[string]any{}
	if issues, ok := response["issues"].([]any); ok {
		flat["issues"] = flattenIssues(issues, descriptionFormat)
	}

	return flat
}

func flattenIssues(issues []any, descriptionFormat string) []map[string]any {
	flat := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		issueMap, ok := issue.(map[string]any)
		if !ok {
			continue
		}
		flat = append(flat, flattenIssue(issueMap, descriptionFormat))
	}
	return flat
}

func flattenIssue(issue map[string]any, descriptionFormat string) map[string]any {
	row := map[string]any{}
	if key, ok := issue["key"]; ok {
		row["key"] = key
	}

	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		return row
	}
	for name, value := range fields {
		if name == "description" {
			row[name] = convertDescriptionOutputValue(value, descriptionFormat)
			continue
		}
		row[name] = flattenIssueFieldValue(value)
	}
	return row
}

func flattenIssueFieldValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any:
		if scalar, ok := flattenDisplayValue(v); ok {
			return scalar
		}
		return flattenIssueFieldMap(v)
	case []any:
		values := make([]any, 0, len(v))
		for _, item := range v {
			values = append(values, flattenIssueFieldValue(item))
		}
		return values
	default:
		return v
	}
}

func flattenDisplayValue(value map[string]any) (any, bool) {
	for _, key := range []string{"displayName", "name", "value", "key"} {
		if scalar, ok := scalarIssueFieldValue(value[key]); ok {
			return scalar, true
		}
	}
	return nil, false
}

func scalarIssueFieldValue(value any) (any, bool) {
	switch value.(type) {
	case string, bool, float64:
		return value, true
	default:
		return nil, false
	}
}

func flattenIssueFieldMap(value map[string]any) map[string]any {
	flat := map[string]any{}
	for key, nested := range value {
		if _, skip := flatIssueNoiseFields[key]; skip {
			continue
		}
		flat[key] = flattenIssueFieldValue(nested)
	}
	return flat
}
