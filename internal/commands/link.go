package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueLinkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "link",
		Usage: "Issue link operations (list, add, delete, types)",
		UsageText: `jira-agent issue link list PROJ-123
jira-agent issue link add --type Blocks --inward PROJ-1 --outward PROJ-2
jira-agent issue link types`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			issueLinkListCommand(apiClient, w, format),
			issueLinkAddCommand(apiClient, w, format, allowWrites),
			issueLinkDeleteCommand(apiClient, w, format, allowWrites),
			issueLinkTypesCommand(apiClient, w, format),
		},
	}
}

func issueRemoteLinkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:    "remote-link",
		Aliases: []string{"remotelink"},
		Usage:   "Remote issue link operations (list, add, edit, delete)",
		UsageText: `jira-agent issue remote-link list PROJ-123
jira-agent issue remote-link add PROJ-123 --url "https://example.com" --title "Docs"`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			remoteLinkListCommand(apiClient, w, format),
			remoteLinkAddCommand(apiClient, w, format, allowWrites),
			remoteLinkEditCommand(apiClient, w, format, allowWrites),
			remoteLinkDeleteCommand(apiClient, w, format, allowWrites),
		},
	}
}

// GET /rest/api/3/issue/{issueIdOrKey}?fields=issuelinks
func issueLinkListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List issue links on an issue",
		UsageText: `jira-agent issue link list PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			var result map[string]any
			if err := apiClient.Get(ctx, "/issue/"+key, map[string]string{"fields": "issuelinks"}, &result); err != nil {
				return err
			}

			links, err := extractIssueLinks(result)
			if err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Total = len(links)
			meta.Returned = len(links)
			return output.WriteSuccess(w, links, meta, *format)
		},
	}
}

// POST /rest/api/3/issueLink
func issueLinkAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a link between two issues",
		UsageText: `jira-agent issue link add --type Blocks --inward PROJ-1 --outward PROJ-2
jira-agent issue link add --type-id 10001 --inward PROJ-1 --outward PROJ-2`,
		Metadata: writeCommandMetadata(),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: "Issue link type name, such as Blocks"},
			&cli.StringFlag{Name: "type-id", Usage: "Issue link type ID"},
			&cli.StringFlag{Name: "inward", Usage: "Inward issue key or ID", Required: true},
			&cli.StringFlag{Name: "outward", Usage: "Outward issue key or ID", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			body, err := buildIssueLinkBody(cmd)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issueLink", body, result)
			})
		}),
	}
}

// DELETE /rest/api/3/issueLink/{linkId}
func issueLinkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an issue link",
		UsageText: `jira-agent issue link delete 12345`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<link-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			linkID, err := requireArg(cmd, "link ID")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/issueLink/"+linkID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"linkId": linkID, "deleted": true}, *format)
		}),
	}
}

// GET /rest/api/3/issueLinkType
func issueLinkTypesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "types",
		Usage:     "List issue link types",
		UsageText: `jira-agent issue link types`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var result map[string]any
			if err := apiClient.Get(ctx, "/issueLinkType", nil, &result); err != nil {
				return err
			}

			items, _ := result["issueLinkTypes"].([]any)
			meta := output.NewMetadata()
			meta.Total = len(items)
			meta.Returned = len(items)
			return output.WriteSuccess(w, result, meta, *format)
		},
	}
}

// GET /rest/api/3/issue/{issueIdOrKey}/remotelink
func remoteLinkListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List remote links on an issue",
		UsageText: `jira-agent issue remote-link list PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			var result []any
			if err := apiClient.Get(ctx, "/issue/"+key+"/remotelink", nil, &result); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Total = len(result)
			meta.Returned = len(result)
			return output.WriteSuccess(w, result, meta, *format)
		},
	}
}

// POST /rest/api/3/issue/{issueIdOrKey}/remotelink
func remoteLinkAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a remote link to an issue",
		UsageText: `jira-agent issue remote-link add PROJ-123 --url "https://example.com" --title "Related doc"`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key>",
		Flags:     remoteLinkFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			body := buildRemoteLinkBody(cmd)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/"+key+"/remotelink", body, result)
			})
		}),
	}
}

// PUT /rest/api/3/issue/{issueIdOrKey}/remotelink/{linkId}
func remoteLinkEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "Edit a remote link on an issue",
		UsageText: `jira-agent issue remote-link edit PROJ-123 10001 --url "https://example.com/updated" --title "Updated doc"`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key> <remote-link-id>",
		Flags:     remoteLinkFlags(),
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "remote link ID")
			if err != nil {
				return err
			}
			key, linkID := args[0], args[1]

			body := buildRemoteLinkBody(cmd)
			path := fmt.Sprintf("/issue/%s/remotelink/%s", key, linkID)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Put(ctx, path, body, result)
			})
		}),
	}
}

// DELETE /rest/api/3/issue/{issueIdOrKey}/remotelink/{linkId}
func remoteLinkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a remote link from an issue",
		UsageText: `jira-agent issue remote-link delete PROJ-123 10001`,
		Metadata:  writeCommandMetadata(),
		ArgsUsage: "<issue-key> <remote-link-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			args, err := requireArgs(cmd, "issue key", "remote link ID")
			if err != nil {
				return err
			}
			key, linkID := args[0], args[1]

			if err := apiClient.Delete(ctx, fmt.Sprintf("/issue/%s/remotelink/%s", key, linkID), nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "remoteLinkId": linkID, "deleted": true}, *format)
		}),
	}
}

func buildIssueLinkBody(cmd *cli.Command) (map[string]any, error) {
	typeName := cmd.String("type")
	typeID := cmd.String("type-id")
	if typeName == "" && typeID == "" {
		return nil, apperr.NewValidationError("--type or --type-id is required", nil)
	}
	if typeName != "" && typeID != "" {
		return nil, apperr.NewValidationError("--type and --type-id cannot both be set", nil)
	}

	linkType := map[string]any{}
	if typeName != "" {
		linkType["name"] = typeName
	} else {
		linkType["id"] = typeID
	}

	return map[string]any{
		"type":         linkType,
		"inwardIssue":  map[string]any{"key": cmd.String("inward")},
		"outwardIssue": map[string]any{"key": cmd.String("outward")},
	}, nil
}

func remoteLinkFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "url", Usage: "Remote object URL", Required: true},
		&cli.StringFlag{Name: "title", Usage: "Remote object title", Required: true},
		&cli.StringFlag{Name: "global-id", Usage: "Stable global ID for the remote link"},
		&cli.StringFlag{Name: "relationship", Usage: "Relationship description, such as mentioned in"},
	}
}

func buildRemoteLinkBody(cmd *cli.Command) map[string]any {
	body := map[string]any{
		"object": map[string]any{
			"url":   cmd.String("url"),
			"title": cmd.String("title"),
		},
	}
	if globalID := cmd.String("global-id"); globalID != "" {
		body["globalId"] = globalID
	}
	if relationship := cmd.String("relationship"); relationship != "" {
		body["relationship"] = relationship
	}
	return body
}

func extractIssueLinks(result map[string]any) ([]any, error) {
	return extractFieldArray(result, "issuelinks")
}
