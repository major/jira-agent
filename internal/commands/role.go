package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// RoleCommand returns the top-level "role" command for global project role definitions.
func RoleCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Project role definition operations",
		Example: `jira-agent role list
jira-agent role get 10000`,
	}
	cmd.AddCommand(
		roleListCommand(apiClient, w, format),
		roleGetCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// roleListCommand lists global project role definitions.
// GET /rest/api/3/role
func roleListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List project role definitions",
		Example: `jira-agent role list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeArrayAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/role", nil, result)
			})
		},
	}
	return cmd
}

// roleGetCommand fetches one global project role definition by ID.
// GET /rest/api/3/role/{id}
func roleGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get project role definition",
		Example: `jira-agent role get 10000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			roleID, err := requireArg(args, "role ID")
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
	return cmd
}
