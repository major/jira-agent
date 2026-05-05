package commands

import (
	"fmt"
	"sort"
	"strings"
)

// buildJQLFromFlags constructs a JQL query string from semantic flag values.
// Conditions are joined with AND in alphabetical order for determinism, and
// ORDER BY updated DESC is appended as the default sort. Returns an empty
// string when no flags are set.
func buildJQLFromFlags(assignee, status, issueType, priority, sprint, updatedSince string, labels []string) string {
	var conditions []string

	if assignee == "me" {
		conditions = append(conditions, "assignee = currentUser()")
	} else if assignee != "" {
		conditions = append(conditions, fmt.Sprintf("assignee = %q", assignee))
	}

	if issueType != "" {
		conditions = append(conditions, fmt.Sprintf("issuetype = %q", issueType))
	}

	for _, label := range labels {
		conditions = append(conditions, fmt.Sprintf("labels = %q", label))
	}

	if priority != "" {
		conditions = append(conditions, fmt.Sprintf("priority = %q", priority))
	}

	if sprint == "current" {
		conditions = append(conditions, "sprint in openSprints()")
	} else if sprint != "" {
		conditions = append(conditions, fmt.Sprintf("sprint = %q", sprint))
	}

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = %q", status))
	}

	if updatedSince != "" {
		conditions = append(conditions, fmt.Sprintf("updated >= %q", "-"+updatedSince))
	}

	sort.Strings(conditions)

	if len(conditions) == 0 {
		return ""
	}

	return strings.Join(conditions, " AND ") + " ORDER BY updated DESC"
}
