package commands

import (
	"context"
	"io"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// AuditCommand returns the top-level "audit" command for Jira audit records.
func AuditCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "audit",
		Usage: "Work with Jira audit records",
		UsageText: `jira-agent audit list
jira-agent audit list --from 2024-01-01T00:00:00.000+0000 --to 2024-12-31T23:59:59.000+0000
jira-agent audit list --filter "user created" --limit 25`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			auditListCommand(apiClient, w, format),
		},
	}
}

// GET /rest/api/3/auditing/record
func auditListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List Jira audit records",
		UsageText: `jira-agent audit list
jira-agent audit list --from 2024-01-01T00:00:00.000+0000 --to 2024-12-31T23:59:59.000+0000
jira-agent audit list --filter "user created" --limit 25`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "filter", Usage: "Text filter across audit record fields"},
			&cli.StringFlag{Name: "from", Usage: "Start date or datetime for created records"},
			&cli.StringFlag{Name: "to", Usage: "End date or datetime for created records"},
			&cli.IntFlag{Name: "limit", Usage: "Page size", Value: 1000},
			&cli.IntFlag{Name: "offset", Usage: "Pagination offset", Value: 0},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{
				"limit":  strconv.Itoa(cmd.Int("limit")),
				"offset": strconv.Itoa(cmd.Int("offset")),
			}
			addOptionalParams(cmd, params, map[string]string{
				"filter": "filter",
				"from":   "from",
				"to":     "to",
			})

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/auditing/record", params, result)
			})
		},
	}
}
