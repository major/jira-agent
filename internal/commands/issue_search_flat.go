package commands

import "github.com/major/jira-agent/internal/output"

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
	response, ok := result.(map[string]any)
	if !ok {
		return result
	}

	flat := map[string]any{}
	if issues, ok := response["issues"].([]any); ok {
		flat["issues"] = flattenIssues(issues)
	}

	for _, key := range []string{"nextPageToken", "isLast"} {
		if value, ok := response[key]; ok {
			flat[key] = value
		}
	}

	return flat
}

func flattenIssues(issues []any) []map[string]any {
	flat := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		issueMap, ok := issue.(map[string]any)
		if !ok {
			continue
		}
		flat = append(flat, flattenIssue(issueMap))
	}
	return flat
}

func flattenIssue(issue map[string]any) map[string]any {
	row := map[string]any{}
	if key, ok := issue["key"]; ok {
		row["key"] = key
	}

	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		return row
	}
	for name, value := range fields {
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
