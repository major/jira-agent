package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// EpicCommand returns the top-level "epic" command with Agile epic operations.
func EpicCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Agile epic operations (get, issues, move-issues, orphans, remove-issues, rank)",
		Example: `jira-agent epic get PROJ-50
jira-agent epic issues PROJ-50
jira-agent epic move-issues PROJ-50 --issues PROJ-1,PROJ-2`,
	}
	cmd.AddCommand(
		epicGetCommand(apiClient, w, format),
		epicIssuesCommand(apiClient, w, format),
		epicMoveIssuesCommand(apiClient, w, format, allowWrites),
		epicOrphansCommand(apiClient, w, format),
		epicRemoveIssuesCommand(apiClient, w, format, allowWrites),
		epicRankCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "get")
	return cmd
}

// epicGetCommand fetches a single epic by ID or key.
// GET /rest/agile/1.0/epic/{epicIdOrKey}
func epicGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get epic details by ID or key",
		Example: `jira-agent epic get PROJ-50
jira-agent epic get 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			epicID, err := requireArg(args, "epic ID or key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/"+epicID, nil, result)
			})
		},
	}
}

// epicIssuesCommand lists issues belonging to an epic.
// GET /rest/agile/1.0/epic/{epicIdOrKey}/issue
func epicIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "List issues in an epic",
		Example: `jira-agent epic issues PROJ-50
jira-agent epic issues PROJ-50 --jql "status = Open"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			epicID, err := requireArg(args, "epic ID or key")
			if err != nil {
				return err
			}

			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/"+epicID+"/issue", params, result)
			})
		},
	}
	cmd.Flags().String("jql", "", "JQL filter to apply")
	cmd.Flags().String("fields", "", "Comma-separated list of fields to return")
	appendPaginationFlags(cmd)
	return cmd
}

// epicMoveIssuesCommand moves issues to an epic.
// POST /rest/agile/1.0/epic/{epicIdOrKey}/issue
func epicMoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "move-issues",
		Short:   "Move issues to an epic",
		Example: `jira-agent epic move-issues PROJ-50 --issues PROJ-1,PROJ-2`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			epicID, err := requireArg(args, "epic ID or key")
			if err != nil {
				return err
			}
			issuesStr, _ := cmd.Flags().GetString("issues")
			issues := splitTrimmed(issuesStr)
			if len(issues) == 0 {
				return apperr.NewValidationError("at least one issue key is required", nil)
			}

			body := map[string]any{
				"issues": issues,
			}

			// Move issues returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, "/epic/"+epicID+"/issue", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"epic":  epicID,
				"moved": issues,
			}, *format)
		}),
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys to move (required)")
	_ = cmd.MarkFlagRequired("issues")
	return cmd
}

// epicOrphansCommand lists issues without an epic.
// GET /rest/agile/1.0/epic/none/issue
func epicOrphansCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orphans",
		Short:   "List issues without an epic",
		Example: `jira-agent epic orphans`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, map[string]string{
				"jql":    "jql",
				"fields": "fields",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.AgileGet(ctx, "/epic/none/issue", params, result)
			})
		},
	}
	cmd.Flags().String("jql", "", "JQL filter to apply")
	cmd.Flags().String("fields", "", "Comma-separated list of fields to return")
	appendPaginationFlags(cmd)
	return cmd
}

// epicRemoveIssuesCommand removes issues from their epic by moving them to "none".
// POST /rest/agile/1.0/epic/none/issue
func epicRemoveIssuesCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-issues",
		Short:   "Remove issues from their epic",
		Example: `jira-agent epic remove-issues --issues PROJ-1,PROJ-2`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issuesStr, _ := cmd.Flags().GetString("issues")
			issues := splitTrimmed(issuesStr)
			if len(issues) == 0 {
				return apperr.NewValidationError("at least one issue key is required", nil)
			}

			body := map[string]any{
				"issues": issues,
			}

			// Remove issues returns 204 No Content on success.
			if err := apiClient.AgilePost(ctx, "/epic/none/issue", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"removed": issues,
			}, *format)
		}),
	}
	cmd.Flags().String("issues", "", "Comma-separated issue keys to remove from their epic (required)")
	_ = cmd.MarkFlagRequired("issues")
	return cmd
}

// epicRankCommand changes the rank of an epic.
// PUT /rest/agile/1.0/epic/{epicIdOrKey}/rank
func epicRankCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank an epic before or after another epic",
		Example: `jira-agent epic rank PROJ-50 --before PROJ-49
jira-agent epic rank PROJ-50 --after PROJ-51`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			epicID, err := requireArg(args, "epic ID or key")
			if err != nil {
				return err
			}

			body := map[string]any{}
			before, _ := cmd.Flags().GetString("rank-before")
			after, _ := cmd.Flags().GetString("rank-after")
			if before == "" && after == "" {
				return apperr.NewValidationError("either --rank-before or --rank-after is required", nil)
			}
			if before != "" {
				body["rankBeforeEpic"] = before
			}
			if after != "" {
				body["rankAfterEpic"] = after
			}

			// Rank returns 204 No Content on success.
			if err := apiClient.AgilePut(ctx, "/epic/"+epicID+"/rank", body, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{
				"epic":   epicID,
				"ranked": true,
			}, *format)
		}),
	}
	cmd.Flags().String("rank-before", "", "Epic ID or key to rank before")
	cmd.Flags().String("rank-after", "", "Epic ID or key to rank after")
	return cmd
}
