package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/jira"
	"github.com/major/jira-agent/internal/output"
)

// resolvedBoard represents a board resolved from Jira with minimal fields.
type resolvedBoard struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// boardResolveCommand returns the "resolve board" subcommand for resolving
// board queries (name) to board IDs.
func boardResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "board <query>",
		Short: "Resolve board by name",
		Example: `jira-agent resolve board "My Scrum Board"
jira-agent resolve board "Kanban"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate query argument
			query, err := requireQuery(args, "board")
			if err != nil {
				return err
			}

			// Call Jira Agile API to search for boards
			params := map[string]string{
				"name":       query,
				"maxResults": "10",
			}

			// Jira Agile API returns object with values array
			var jiraResponse struct {
				Values []map[string]any `json:"values"`
				Total  int              `json:"total"`
			}
			if err := apiClient.AgileGet(ctx, "/board", params, &jiraResponse); err != nil {
				return err
			}

			// Check if no boards found
			if len(jiraResponse.Values) == 0 {
				return apperr.NewNotFoundError(
					fmt.Sprintf("no boards found matching %q", query),
					nil,
					apperr.WithAvailableActions([]string{"jira-agent board list"}),
				)
			}

			// Map Jira response to resolvedBoard, stripping extra fields
			boards := make([]resolvedBoard, len(jiraResponse.Values))
			for i, jiraBoard := range jiraResponse.Values {
				ext := jira.NewExtract(jiraBoard)
				boards[i] = resolvedBoard{
					ID:   ext.Int64("id"),
					Name: ext.String("name"),
					Type: ext.String("type"),
				}
			}

			// Build metadata with usage hint
			meta := resolverMetadata(
				"jira-agent resolve sprint --board-id <id> <sprint-name>",
			)

			return output.WriteSuccess(w, boards, meta, *format)
		},
	}

	return cmd
}
