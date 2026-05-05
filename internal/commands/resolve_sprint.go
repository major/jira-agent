package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/jira"
	"github.com/major/jira-agent/internal/output"
)

// resolvedSprint represents a sprint resolved from Jira with minimal fields.
type resolvedSprint struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// sprintResolveCommand returns the "resolve sprint" subcommand for resolving
// sprint queries (name) to sprint IDs on a specific board.
func sprintResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprint <query>",
		Short: "Resolve sprint by name on a board",
		Example: `jira-agent resolve sprint --board-id 42 "Sprint 5"
jira-agent resolve sprint --board-id 42 --state active "Sprint"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate query argument
			query, err := requireQuery(args, "sprint")
			if err != nil {
				return err
			}

			// Get and validate board ID
			boardIDStr, _ := cmd.Flags().GetString("board-id")
			if boardIDStr == "" {
				return apperr.NewValidationError(
					"--board-id is required",
					nil,
					apperr.WithNextCommand("jira-agent resolve board <board-name>"),
				)
			}

			boardID, err := parseBoardID(boardIDStr)
			if err != nil {
				return err
			}

			// Get state filter
			state, _ := cmd.Flags().GetString("state")

			// Call Jira Agile API to search for sprints
			params := map[string]string{
				"state":      state,
				"maxResults": "50",
			}

			// Jira Agile API returns object with values array
			var jiraResponse struct {
				Values []map[string]any `json:"values"`
				Total  int              `json:"total"`
			}
			if err := apiClient.AgileGet(ctx, fmt.Sprintf("/board/%d/sprint", boardID), params, &jiraResponse); err != nil {
				return err
			}

			// Client-side filtering: case-insensitive name matching.
			// Count all matches but only keep the first 10 results.
			const maxResults = 10
			var matches []resolvedSprint
			totalMatches := 0
			for _, jiraSprint := range jiraResponse.Values {
				ext := jira.NewExtract(jiraSprint)
				sprintName := ext.String("name")
				if strings.Contains(strings.ToLower(sprintName), strings.ToLower(query)) {
					totalMatches++
					if len(matches) < maxResults {
						matches = append(matches, resolvedSprint{
							ID:    ext.Int64("id"),
							Name:  sprintName,
							State: ext.String("state"),
						})
					}
				}
			}

			// Check if no sprints found
			if totalMatches == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no sprints found matching %q on board %d", query, boardID),
					nil,
				)
			}

			// Build metadata with usage hint
			meta := resolverMetadata(
				"jira-agent sprint get <id>",
			)

			return output.WriteSuccess(w, matches, meta, *format)
		},
	}

	cmd.Flags().String("board-id", "", "Board ID (required)")
	cmd.Flags().String("state", "active,future", "Filter by state: future, active, closed (comma-separated)")

	return cmd
}
