package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// BoardCommand returns the top-level "board" command with Jira Software board operations.
func BoardCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Jira Software board operations (list, create, get, filter, delete, properties)",
		Example: `jira-agent board list
jira-agent board create --name "Team board" --type scrum --filter 10000
jira-agent board get 42
jira-agent board filter 10000
jira-agent board issues 42 --jql "status = Open"
jira-agent board property list 42`,
	}
	cmd.AddCommand(
		boardListCommand(apiClient, w, format),
		boardCreateCommand(apiClient, w, format, allowWrites),
		boardGetCommand(apiClient, w, format),
		boardFilterCommand(apiClient, w, format),
		boardDeleteCommand(apiClient, w, format, allowWrites),
		boardConfigCommand(apiClient, w, format),
		boardEpicsCommand(apiClient, w, format),
		boardIssuesCommand(apiClient, w, format),
		boardProjectsCommand(apiClient, w, format),
		boardVersionsCommand(apiClient, w, format),
		boardPropertyCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// boardCreateCommand creates a Jira Software board from an existing saved filter.
// POST /rest/agile/1.0/board
func boardCreateCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Jira Software board",
		Example: `jira-agent board create --name "Team board" --type scrum --filter 10000
jira-agent board create --name "Team board" --type kanban --filter 10000 --location-project PROJ`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, err := requireFlag(cmd, "name")
			if err != nil {
				return err
			}
			boardType, err := requireFlag(cmd, "type")
			if err != nil {
				return err
			}
			filterID, _ := cmd.Flags().GetInt64("filter")
			if filterID <= 0 {
				return apperr.NewValidationError("--filter must be a positive integer", nil)
			}

			body := map[string]any{
				"name":     name,
				"type":     boardType,
				"filterId": filterID,
			}
			if project, _ := cmd.Flags().GetString("location-project"); project != "" {
				body["location"] = map[string]any{
					"type":           "project",
					"projectKeyOrId": project,
				}
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgilePost(ctx, "/board", body, result)
			})
		}),
	}
	cmd.Flags().String("name", "", "Board name")
	cmd.Flags().String("type", "", "Board type: scrum or kanban")
	cmd.Flags().Int64("filter", 0, "Saved filter ID backing the board")
	cmd.Flags().String("location-project", "", "Project key or ID for the board location")
	return cmd
}

// boardListCommand lists Jira Software boards with pagination and common filters.
// GET /rest/agile/1.0/board
func boardListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Jira Software boards",
		Example: `jira-agent board list
jira-agent board list --type scrum
jira-agent board list --project PROJ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, map[string]string{
				"type":    "type",
				"name":    "name",
				"project": "projectKeyOrId",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board", params, result)
			})
		},
	}
	cmd.Flags().String("type", "", "Filter by board type: scrum or kanban")
	cmd.Flags().String("name", "", "Filter by board name")
	cmd.Flags().String("project", "", "Filter by project key or ID referenced by the board filter")
	appendPaginationFlags(cmd)
	return cmd
}

// boardGetCommand fetches a single Jira Software board by ID.
// GET /rest/agile/1.0/board/{boardId}
func boardGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Short:   "Get Jira Software board details by ID",
		Example: `jira-agent board get 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			parsedBoardID, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(parsedBoardID, 10), nil, result)
			})
		},
	}
}

// boardFilterCommand lists boards that use a saved filter.
// GET /rest/agile/1.0/board/filter/{filterId}
func boardFilterCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "List boards using a filter",
		Example: `jira-agent board filter 10000
jira-agent board filter 10000 --max-results 25`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			filterID, err := requireArg(args, "filter ID")
			if err != nil {
				return err
			}
			parsedFilterID, err := parsePositiveIntID(filterID, "filter ID")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/filter/"+strconv.FormatInt(parsedFilterID, 10), params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

// boardDeleteCommand deletes a Jira Software board by ID.
// DELETE /rest/agile/1.0/board/{boardId}
func boardDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "delete",
		Short:   "Delete a Jira Software board",
		Example: `jira-agent board delete 42`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			parsedBoardID, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			id := strconv.FormatInt(parsedBoardID, 10)
			if err := apiClient.AgileDelete(ctx, "/board/"+id, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"id": parsedBoardID, "deleted": true}, *format)
		}),
	}
}

func parseBoardID(boardID string) (int64, error) {
	return parsePositiveIntID(boardID, "board ID")
}

func parsePositiveIntID(value, label string) (int64, error) {
	parsedID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsedID <= 0 {
		return 0, apperr.NewValidationError(label+" must be a positive integer", err)
	}

	return parsedID, nil
}

// boardConfigCommand fetches the configuration for a Jira Software board.
// GET /rest/agile/1.0/board/{boardId}/configuration
func boardConfigCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "Get board configuration (columns, estimation, ranking)",
		Example: `jira-agent board config 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			id, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(id, 10)+"/configuration", nil, result)
			})
		},
	}
}

// boardEpicsCommand lists epics associated with a board.
// GET /rest/agile/1.0/board/{boardId}/epic
func boardEpicsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "epics",
		Short:   "List epics on a board",
		Example: `jira-agent board epics 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			id, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)
			addBoolParam(cmd, params, "done", "done")

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(id, 10)+"/epic", params, result)
			})
		},
	}
	cmd.Flags().Bool("done", false, "Include done epics")
	appendPaginationFlags(cmd)
	return cmd
}

// boardIssuesCommand lists issues on a board with optional JQL filtering.
// GET /rest/agile/1.0/board/{boardId}/issue
func boardIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List issues on a board",
		Example: `jira-agent board issues 42
jira-agent board issues 42 --jql "status = Open"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			id, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
				"expand": "expand",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(id, 10)+"/issue", params, result)
			})
		},
	}
	cmd.Flags().String("jql", "", "JQL filter to apply")
	cmd.Flags().String("fields", "", "Comma-separated list of fields to return")
	cmd.Flags().String("expand", "", "Comma-separated list of expansions")
	appendPaginationFlags(cmd)
	return cmd
}

// boardProjectsCommand lists projects associated with a board.
// GET /rest/agile/1.0/board/{boardId}/project
func boardProjectsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Short:   "List projects associated with a board",
		Example: `jira-agent board projects 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			id, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(id, 10)+"/project", params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

// boardVersionsCommand lists versions associated with a board.
// GET /rest/agile/1.0/board/{boardId}/version
func boardVersionsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "versions",
		Short:   "List versions associated with a board",
		Example: `jira-agent board versions 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			boardID, err := requireArg(args, "board ID")
			if err != nil {
				return err
			}
			id, err := parseBoardID(boardID)
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/board/"+strconv.FormatInt(id, 10)+"/version", params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}
