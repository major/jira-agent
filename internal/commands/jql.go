package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	"github.com/major/jira-agent/internal/output"
)

// JQLCommand returns the "jql" parent command for JQL autocomplete and
// validation helpers.
func JQLCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "jql",
		Usage: "JQL autocomplete and validation helpers",
		UsageText: `jira-agent jql fields
jira-agent jql suggest --field-name status
jira-agent jql validate --query "project = PROJ AND status = Open"`,
		Commands: []*cli.Command{
			jqlFieldsCommand(apiClient, w, format),
			jqlSuggestCommand(apiClient, w, format),
			jqlValidateCommand(apiClient, w, format),
		},
	}
}

// GET /rest/api/3/jql/autocompletedata
func jqlFieldsCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "fields",
		Usage: "List JQL field reference data (field names, operators, functions, reserved words)",
		UsageText: `jira-agent jql fields`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/jql/autocompletedata", nil, result)
			})
		},
	}
}

// GET /rest/api/3/jql/autocompletedata/suggestions
func jqlSuggestCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "suggest",
		Usage: "Get JQL autocomplete suggestions for a field value",
		UsageText: `jira-agent jql suggest --field-name status
jira-agent jql suggest --field-name priority --field-value Hi`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "field-name",
				Usage:    "field to get suggestions for (e.g. reporter, assignee, project)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "field-value",
				Usage: "partial value to filter suggestions",
			},
			&cli.StringFlag{
				Name:  "predicate-name",
				Usage: "CHANGED operator predicate (by, from, to)",
			},
			&cli.StringFlag{
				Name:  "predicate-value",
				Usage: "partial predicate value to filter suggestions",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			params := map[string]string{
				"fieldName": cmd.String("field-name"),
			}
			addOptionalParams(cmd, params, map[string]string{
				"field-value":     "fieldValue",
				"predicate-name":  "predicateName",
				"predicate-value": "predicateValue",
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/jql/autocompletedata/suggestions", params, result)
			})
		},
	}
}

// POST /rest/api/3/jql/parse
func jqlValidateCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Parse and validate one or more JQL queries",
		UsageText: `jira-agent jql validate --query "project = PROJ AND status = Open"`,
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "JQL query to validate (repeatable)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "validation",
				Usage: "validation mode: strict, warn, or none",
				Value: "strict",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			body := map[string]any{
				"queries": cmd.StringSlice("query"),
			}
			path := appendQueryParams("/jql/parse", map[string]string{
				"validation": cmd.String("validation"),
			})

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, path, body, result)
			})
		},
	}
}
