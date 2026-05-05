package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// ResolveCommand returns the top-level "resolve" command for resolving
// human-friendly names to Jira internal IDs.
func ResolveCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve human-friendly names to Jira internal IDs",
		Example: `jira-agent resolve user "John Doe"
jira-agent resolve project "My Project"
jira-agent resolve status "In Progress"`,
	}
	SetCommandCategory(cmd, commandCategoryDiscovery)
	return cmd
}

// resolverMetadata builds metadata for resolve commands with total, returned,
// and usage hint fields.
func resolverMetadata(total, returned int, usageHint string) output.Metadata {
	meta := output.NewMetadata()
	meta.Total = total
	meta.Returned = returned
	meta.UsageHint = usageHint
	return meta
}

// requireQuery validates that a query argument is present and non-empty.
// Returns a ValidationError if the query is missing or empty.
func requireQuery(args []string, entityName string) (string, error) {
	if len(args) == 0 {
		return "", apperr.NewValidationError(
			fmt.Sprintf("query is required for resolve %s", entityName),
			nil,
		)
	}
	if args[0] == "" {
		return "", apperr.NewValidationError(
			fmt.Sprintf("query is required for resolve %s", entityName),
			nil,
		)
	}
	return args[0], nil
}
