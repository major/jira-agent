package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
	"github.com/urfave/cli/v3"
)

// FieldCommand returns the top-level "field" command with list and search
// subcommands.
func FieldCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:    "field",
		Usage: "Work with Jira fields",
		UsageText: `jira-agent field list
jira-agent field search --type custom --query "story points"`,
		DefaultCommand: "list",
		Aliases:        []string{"f"},
		Commands: []*cli.Command{
			fieldListCommand(apiClient, w, format),
			fieldSearchCommand(apiClient, w, format),
			fieldContextCommand(apiClient, w, format, allowWrites),
			fieldOptionCommand(apiClient, w, format, allowWrites),
		},
	}
}

// fieldListCommand returns all system and custom fields.
// GET /rest/api/3/field
func fieldListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List all fields (system and custom)",
		UsageText: `jira-agent field list`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var fields []any
			if err := apiClient.Get(ctx, "/field", nil, &fields); err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Total = len(fields)
			meta.Returned = len(fields)

			return output.WriteSuccess(w, fields, meta, *format)
		},
	}
}

// fieldSearchCommand searches fields with pagination.
// GET /rest/api/3/field/search
func fieldSearchCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search fields with filtering and pagination",
		UsageText: `jira-agent field search --type custom
jira-agent field search --query "story points"`,
		Flags: appendPaginationFlagsWithUsage([]cli.Flag{
			&cli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Filter fields by name (substring match)",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Filter by field type: custom, system",
				Validator: func(s string) error {
					if s != "" && s != "custom" && s != "system" {
						return fmt.Errorf("invalid field type %q: must be custom or system", s)
					}
					return nil
				},
			},
		}, paginationFlagUsage{
			maxResults: "Maximum number of results to return",
			startAt:    "Index of the first result to return",
		}),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := buildPaginationParams(cmd, map[string]string{
				"query": "query",
				"type":  "type",
			})

			var result map[string]any
			if err := apiClient.Get(ctx, "/field/search", params, &result); err != nil {
				return err
			}

			// The search endpoint returns paginated results with a "values" key.
			values, ok := result["values"]
			if !ok {
				return apperr.NewJiraError(
					"unexpected response: missing 'values' key",
					nil,
				)
			}

			meta := extractPaginationMeta(result)
			return output.WriteSuccess(w, values, meta, *format)
		},
	}
}
