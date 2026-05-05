package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// AuditCommand returns the top-level "audit" command for Jira audit records.
func AuditCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Work with Jira audit records",
		Example: `jira-agent audit list
jira-agent audit list --from 2024-01-01T00:00:00.000+0000 --to 2024-12-31T23:59:59.000+0000
jira-agent audit list --filter "user created" --limit 25`,
	}
	cmd.AddCommand(
		auditListCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// GET /rest/api/3/auditing/record
func auditListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Jira audit records",
		Example: `jira-agent audit list
jira-agent audit list --from 2024-01-01T00:00:00.000+0000 --to 2024-12-31T23:59:59.000+0000
jira-agent audit list --filter "user created" --limit 25`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			params := map[string]string{
				"limit":  strconv.Itoa(limit),
				"offset": strconv.Itoa(offset),
			}
			addOptionalParams(cmd, params, map[string]string{
				"filter": "filter",
				"from":   "from",
				"to":     "to",
			})

			return writePaginatedAPIResult(cmd, w, *format, func(result any) error {
				return apiClient.Get(ctx, "/auditing/record", params, result)
			})
		},
	}
	cmd.Flags().String("filter", "", "Text filter across audit record fields")
	cmd.Flags().String("from", "", "Start date or datetime for created records")
	cmd.Flags().String("to", "", "End date or datetime for created records")
	cmd.Flags().Int("limit", 1000, "Page size")
	cmd.Flags().Int("offset", 0, "Pagination offset")
	return cmd
}
