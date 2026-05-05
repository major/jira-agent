package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// resolvedTransition represents a transition resolved from Jira with minimal fields.
type resolvedTransition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// filterTransitions filters transitions by query (case-insensitive substring match).
// Returns matching transitions and all available transition names.
func filterTransitions(transitionsAny []any, query string) (matches []resolvedTransition, availableNames []string) {
	for _, t := range transitionsAny {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}

		// Extract transition name
		name, _ := tm["name"].(string)
		if name != "" {
			availableNames = append(availableNames, name)
		}

		// Extract transition ID
		id, _ := tm["id"].(string)

		// Match against transition name (case-insensitive substring)
		if name != "" && strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			matches = append(matches, resolvedTransition{
				ID:   id,
				Name: name,
			})
			continue
		}

		// Match against target status name in "to" object (case-insensitive substring)
		if to, ok := tm["to"].(map[string]any); ok {
			if toName, ok := to["name"].(string); ok && strings.Contains(strings.ToLower(toName), strings.ToLower(query)) {
				matches = append(matches, resolvedTransition{
					ID:   id,
					Name: name,
				})
			}
		}
	}

	return matches, availableNames
}

// transitionResolveCommand returns the "resolve transition" subcommand for resolving
// transition queries (by name or target status) to transition IDs.
func transitionResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transition <query>",
		Short: "Resolve transition by name or target status",
		Example: `jira-agent resolve transition --issue PROJ-123 "Done"
jira-agent resolve transition --issue PROJ-123 "In Progress"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate query argument
			query, err := requireQuery(args, "transition")
			if err != nil {
				return err
			}

			// Get issue key from flag
			issueKey, err := cmd.Flags().GetString("issue")
			if err != nil {
				return apperr.NewValidationError(
					"failed to read --issue flag",
					err,
				)
			}

			if issueKey == "" {
				return apperr.NewValidationError(
					"--issue is required",
					nil,
					apperr.WithNextCommand("jira-agent issue get <issue-key>"),
				)
			}

			// Call Jira API to get transitions for the issue
			var transitionsResp map[string]any
			if err := apiClient.Get(ctx, fmt.Sprintf("/issue/%s/transitions", issueKey), nil, &transitionsResp); err != nil {
				// Wrap 404 errors with issue-specific message
				var notFoundErr *apperr.NotFoundError
				if errors.As(err, &notFoundErr) {
					return apperr.NewNotFoundError(
						fmt.Sprintf("issue %s not found", issueKey),
						err,
					)
				}
				return err
			}

			// Extract transitions array from response
			transitionsAny, ok := transitionsResp["transitions"].([]any)
			if !ok {
				return apperr.NewAPIError(
					"unexpected transitions response format: missing or invalid \"transitions\" array",
					0,
					"",
					nil,
				)
			}

			// Filter transitions by query
			matches, availableNames := filterTransitions(transitionsAny, query)

			// If no matches found, return error with available actions
			if len(matches) == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no transitions matching %q for issue %s", query, issueKey),
					nil,
					apperr.WithAvailableActions(availableNames),
				)
			}

			// Build metadata with usage hint
			meta := resolverMetadata(
				fmt.Sprintf("jira-agent issue transition %s --transition-id <id>", issueKey),
			)

			return output.WriteSuccess(w, matches, meta, *format)
		},
	}

	cmd.Flags().String("issue", "", "Issue key to get transitions for (required)")

	return cmd
}
