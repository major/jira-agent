package commands

import (
	"context"
	"io"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// RoleCommand returns the top-level "role" command for global project role definitions.
func RoleCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "role",
		Usage: "Project role definition operations",
		UsageText: `jira-agent role list
jira-agent role get 10000`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			roleListCommand(apiClient, w, format),
			roleGetCommand(apiClient, w, format),
		},
	}
}

// roleListCommand lists global project role definitions.
// GET /rest/api/3/role
func roleListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List project role definitions",
		UsageText: `jira-agent role list`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/role", nil, result)
			})
		},
	}
}

// roleGetCommand fetches one global project role definition by ID.
// GET /rest/api/3/role/{id}
func roleGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get project role definition",
		UsageText: `jira-agent role get 10000`,
		ArgsUsage: "<role-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			roleID, err := requireArg(cmd, "role ID")
			if err != nil {
				return err
			}
			parsedRoleID, err := parsePositiveIntID(roleID, "role ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/role/"+strconv.FormatInt(parsedRoleID, 10), nil, result)
			})
		},
	}
}
