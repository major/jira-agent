package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// resolvedUser represents a user resolved from Jira with minimal fields.
type resolvedUser struct {
	AccountID    string `json:"account_id"`
	DisplayName  string `json:"display_name"`
	EmailAddress string `json:"email_address"`
	Active       bool   `json:"active"`
}

// userResolveCommand returns the "resolve user" subcommand for resolving
// user queries (email or display name) to account IDs.
func userResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <query>",
		Short: "Resolve user by email or display name",
		Example: `jira-agent resolve user "john@example.com"
jira-agent resolve user "John Doe"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate query argument
			query, err := requireQuery(args, "user")
			if err != nil {
				return err
			}

			// Call Jira API to search for users
			params := map[string]string{
				"query":      query,
				"maxResults": "10",
			}

			// Jira API returns array of user objects with camelCase fields
			var jiraUsers []map[string]any
			if err := apiClient.Get(ctx, "/users/search", params, &jiraUsers); err != nil {
				return err
			}

			// Check if no users found
			if len(jiraUsers) == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no users found matching %q", query),
					nil,
					apperr.WithNextCommand(fmt.Sprintf("jira-agent user search --query %q", query)),
				)
			}

			// Map Jira response to resolvedUser, stripping extra fields
			users := make([]resolvedUser, len(jiraUsers))
			for i, jiraUser := range jiraUsers {
				users[i] = resolvedUser{
					AccountID:    getStringField(jiraUser, "accountId"),
					DisplayName:  getStringField(jiraUser, "displayName"),
					EmailAddress: getStringField(jiraUser, "emailAddress"),
					Active:       getBoolField(jiraUser, "active"),
				}
			}

			// Build metadata with usage hint
			meta := resolverMetadata(
				"jira-agent issue assign <issue-key> --assignee <account_id>",
			)

			return output.WriteSuccess(w, users, meta, *format)
		},
	}

	return cmd
}

// getStringField safely extracts a string field from a map.
func getStringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getBoolField safely extracts a bool field from a map.
func getBoolField(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
