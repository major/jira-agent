package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// StatusCommand returns the top-level "status" command for Jira status and
// status category lookups.
func StatusCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Work with Jira statuses",
		UsageText: `jira-agent status list
jira-agent status get "In Progress"
jira-agent status categories`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			statusListCommand(apiClient, w, format),
			statusGetCommand(apiClient, w, format),
			statusCategoriesCommand(apiClient, w, format),
		},
	}
}

// PriorityCommand returns the top-level "priority" command for Jira priorities.
func PriorityCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "priority",
		Usage: "Work with Jira priorities",
		UsageText: `jira-agent priority list`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			priorityListCommand(apiClient, w, format),
		},
	}
}

// ResolutionCommand returns the top-level "resolution" command for Jira
// resolutions.
func ResolutionCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "resolution",
		Usage: "Work with Jira resolutions",
		UsageText: `jira-agent resolution list`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			resolutionListCommand(apiClient, w, format),
		},
	}
}

// IssueTypeCommand returns the top-level "issuetype" command for Jira issue
// types.
func IssueTypeCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:    "issuetype",
		Usage:   "Work with Jira issue types",
		UsageText: `jira-agent issuetype list
jira-agent issuetype get 10001
jira-agent issuetype project 10001`,
		DefaultCommand: "list",
		Aliases: []string{"issue-type"},
		Commands: []*cli.Command{
			issueTypeListCommand(apiClient, w, format),
			issueTypeGetCommand(apiClient, w, format),
			issueTypeProjectCommand(apiClient, w, format),
		},
	}
}

// LabelCommand returns the top-level "label" command for Jira labels.
func LabelCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "label",
		Usage: "Work with Jira labels",
		UsageText: `jira-agent label list`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			labelListCommand(apiClient, w, format),
		},
	}
}

// GET /rest/api/3/status
func statusListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return flatListCommand("list", "List all statuses", "/status", apiClient, w, format)
}

// GET /rest/api/3/status/{idOrName}
func statusGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return singleGetCommand("get", "Get status details by ID or name", "<status-id-or-name>", "status ID or name", "/status/", apiClient, w, format)
}

// GET /rest/api/3/statuses/categories
func statusCategoriesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	cmd := flatListCommand("categories", "List status categories", "/statuses/categories", apiClient, w, format)
	cmd.UsageText = `jira-agent status categories`
	return cmd
}

// GET /rest/api/3/priority
func priorityListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return flatListCommand("list", "List all priorities", "/priority", apiClient, w, format)
}

// GET /rest/api/3/resolution
func resolutionListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return flatListCommand("list", "List all resolutions", "/resolution", apiClient, w, format)
}

// GET /rest/api/3/issuetype
func issueTypeListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return flatListCommand("list", "List all issue types", "/issuetype", apiClient, w, format)
}

// GET /rest/api/3/issuetype/{id}
func issueTypeGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return singleGetCommand("get", "Get issue type details by ID", "<issue-type-id>", "issue type ID", "/issuetype/", apiClient, w, format)
}

// GET /rest/api/3/issuetype/project?projectId={projectId}
func issueTypeProjectCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "project",
		Usage:     "List issue types for a project ID",
		UsageText: `jira-agent issuetype project 10001`,
		ArgsUsage: "<project-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			projectID, err := requireArg(cmd, "project ID")
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
}

// GET /rest/api/3/label
func labelListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List labels",
		Flags: appendPaginationFlags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, nil)

			return writePaginatedAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/label", params, result)
			})
		},
	}
}

// ServerInfoCommand returns the "server-info" command for Jira instance metadata.
// GET /rest/api/3/serverInfo
func ServerInfoCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "server-info",
		Usage: "Show Jira server information",
		UsageText: `jira-agent server-info`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/serverInfo", nil, result)
			})
		},
	}
}

func flatListCommand(
	name string,
	usage string,
	path string,
	apiClient *client.Ref,
	w io.Writer,
	format *output.Format,
) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
		Action: func(ctx context.Context, cmd *cli.Command) error {
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
) *cli.Command {
	return &cli.Command{
		Name:      name,
		Usage:     usage,
		ArgsUsage: argsUsage,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			value, err := requireArg(cmd, argLabel)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, pathPrefix+value, nil, result)
			})
		},
	}
}
