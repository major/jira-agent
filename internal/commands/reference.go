package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// StatusCommand returns the top-level "status" command for Jira status and
// status category lookups.
func StatusCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Work with Jira statuses",
		Example: `jira-agent status list
jira-agent status get "In Progress"
jira-agent status categories`,
	}
	cmd.AddCommand(
		statusListCommand(apiClient, w, format),
		statusGetCommand(apiClient, w, format),
		statusCategoriesCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// PriorityCommand returns the top-level "priority" command for Jira priorities.
func PriorityCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "priority",
		Short: "Work with Jira priorities",
		Example: `jira-agent priority list`,
	}
	cmd.AddCommand(
		priorityListCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// ResolutionCommand returns the top-level "resolution" command for Jira
// resolutions.
func ResolutionCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolution",
		Short: "Work with Jira resolutions",
		Example: `jira-agent resolution list`,
	}
	cmd.AddCommand(
		resolutionListCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// IssueTypeCommand returns the top-level "issuetype" command for Jira issue
// types.
func IssueTypeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issuetype",
		Short:   "Work with Jira issue types",
		Aliases: []string{"issue-type"},
		Example: `jira-agent issuetype list
jira-agent issuetype get 10001
jira-agent issuetype project 10001`,
	}
	cmd.AddCommand(
		issueTypeListCommand(apiClient, w, format),
		issueTypeGetCommand(apiClient, w, format),
		issueTypeProjectCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// LabelCommand returns the top-level "label" command for Jira labels.
func LabelCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Work with Jira labels",
		Example: `jira-agent label list`,
	}
	cmd.AddCommand(
		labelListCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// GET /rest/api/3/status
func statusListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return flatListCommand("list", "List all statuses", "/status", apiClient, w, format)
}

// GET /rest/api/3/status/{idOrName}
func statusGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return singleGetCommand("get", "Get status details by ID or name", "<status-id-or-name>", "status ID or name", "/status/", apiClient, w, format)
}

// GET /rest/api/3/statuses/categories
func statusCategoriesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := flatListCommand("categories", "List status categories", "/statuses/categories", apiClient, w, format)
	cmd.Example = `jira-agent status categories`
	return cmd
}

// GET /rest/api/3/priority
func priorityListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return flatListCommand("list", "List all priorities", "/priority", apiClient, w, format)
}

// GET /rest/api/3/resolution
func resolutionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return flatListCommand("list", "List all resolutions", "/resolution", apiClient, w, format)
}

// GET /rest/api/3/issuetype
func issueTypeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return flatListCommand("list", "List all issue types", "/issuetype", apiClient, w, format)
}

// GET /rest/api/3/issuetype/{id}
func issueTypeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	return singleGetCommand("get", "Get issue type details by ID", "<issue-type-id>", "issue type ID", "/issuetype/", apiClient, w, format)
}

// GET /rest/api/3/issuetype/project?projectId={projectId}
func issueTypeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project <project-id>",
		Short:   "List issue types for a project ID",
		Example: `jira-agent issuetype project 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectID, err := requireArg(args, "project ID")
			if err != nil {
				return err
			}

			var items []any
			if err := apiClient.Get(ctx, "/issuetype/project", map[string]string{"projectId": projectID}, &items); err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Total = len(items)
			meta.Returned = len(items)
			return output.WriteSuccess(w, items, meta, *format)
		},
	}
	return cmd
}

// GET /rest/api/3/label
func labelListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/label", params, result)
			})
		},
	}
	appendPaginationFlags(cmd)
	return cmd
}

// ServerInfoCommand returns the "server-info" command for Jira instance metadata.
// GET /rest/api/3/serverInfo
func ServerInfoCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server-info",
		Short:   "Show Jira server information",
		Example: `jira-agent server-info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/serverInfo", nil, result)
			})
		},
	}
	return cmd
}

func flatListCommand(
	name string,
	usage string,
	path string,
	apiClient *client.Ref,
	w io.Writer,
	format *output.Format,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var items []any
			if err := apiClient.Get(ctx, path, nil, &items); err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Total = len(items)
			meta.Returned = len(items)
			return output.WriteSuccess(w, items, meta, *format)
		},
	}
	return cmd
}

func singleGetCommand(
	name string,
	usage string,
	argsUsage string,
	argLabel string,
	pathPrefix string,
	apiClient *client.Ref,
	w io.Writer,
	format *output.Format,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " " + argsUsage,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			value, err := requireArg(args, argLabel)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, pathPrefix+value, nil, result)
			})
		},
	}
	return cmd
}
