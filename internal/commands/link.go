package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueLinkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Issue link operations (list, add, delete, types)",
		Example: `jira-agent issue link list PROJ-123
jira-agent issue link add --type Blocks --inward PROJ-1 --outward PROJ-2
jira-agent issue link types`,
	}
	cmd.AddCommand(
		issueLinkListCommand(apiClient, w, format),
		issueLinkGetCommand(apiClient, w, format),
		issueLinkAddCommand(apiClient, w, format, allowWrites),
		issueLinkDeleteCommand(apiClient, w, format, allowWrites),
		issueLinkTypesCommand(apiClient, w, format),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func issueRemoteLinkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remote-link",
		Aliases: []string{"remotelink"},
		Short:   "Remote issue link operations (list, add, edit, delete)",
		Example: `jira-agent issue remote-link list PROJ-123
jira-agent issue remote-link add PROJ-123 --url "https://example.com" --title "Docs"`,
	}
	cmd.AddCommand(
		remoteLinkListCommand(apiClient, w, format),
		remoteLinkGetCommand(apiClient, w, format),
		remoteLinkAddCommand(apiClient, w, format, allowWrites),
		remoteLinkEditCommand(apiClient, w, format, allowWrites),
		remoteLinkDeleteCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

// GET /rest/api/3/issue/{issueIdOrKey}?fields=issuelinks
func issueLinkListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List issue links on an issue",
		Example: `jira-agent issue link list PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
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
	return cmd
}

// GET /rest/api/3/issueLink/{linkId}
func issueLinkGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <link-id>",
		Short:   "Get an issue link",
		Example: `jira-agent issue link get 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			linkID, err := requireArg(args, "link ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issueLink/"+linkID, nil, result)
			})
		},
	}
	return cmd
}

// POST /rest/api/3/issueLink
func issueLinkAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a link between two issues",
		Example: `jira-agent issue link add --type Blocks --inward PROJ-1 --outward PROJ-2
jira-agent issue link add --type-id 10001 --inward PROJ-1 --outward PROJ-2`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body, err := buildIssueLinkBody(cmd)
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issueLink", body, result)
			})
		}),
	}
	cmd.Flags().String("type", "", "Issue link type name, such as Blocks")
	cmd.Flags().String("type-id", "", "Issue link type ID")
	cmd.Flags().String("inward", "", "Inward issue key or ID")
	_ = cmd.MarkFlagRequired("inward")
	cmd.Flags().String("outward", "", "Outward issue key or ID")
	_ = cmd.MarkFlagRequired("outward")
	cmd.Flags().String("comment", "", "Comment to add with the issue link")
	markMutuallyExclusive(cmd, "type", "type-id")
	return cmd
}

// DELETE /rest/api/3/issueLink/{linkId}
func issueLinkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <link-id>",
		Short:   "Delete an issue link",
		Example: `jira-agent issue link delete 12345`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			linkID, err := requireArg(args, "link ID")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/issueLink/"+linkID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"linkId": linkID, "deleted": true}, *format)
		}),
	}
	return cmd
}

// GET /rest/api/3/issueLinkType
func issueLinkTypesCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "types",
		Short:   "List issue link types",
		Example: `jira-agent issue link types`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
	return cmd
}

// GET /rest/api/3/issue/{issueIdOrKey}/remotelink
func remoteLinkListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List remote links on an issue",
		Example: `jira-agent issue remote-link list PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			params := map[string]string{}
			addOptionalParams(cmd, params, map[string]string{"global-id": "globalId"})
			path := appendQueryParams("/issue/"+key+"/remotelink", params)

			var result []any
			if err := apiClient.Get(ctx, path, nil, &result); err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Total = len(result)
			meta.Returned = len(result)
			return output.WriteSuccess(w, result, meta, *format)
		},
	}
	cmd.Flags().String("global-id", "", "Filter by stable global ID")
	return cmd
}

// GET /rest/api/3/issue/{issueIdOrKey}/remotelink/{linkId}
func remoteLinkGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <issue-key> <remote-link-id>",
		Short:   "Get a remote issue link",
		Example: `jira-agent issue remote-link get PROJ-123 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "remote link ID")
			if err != nil {
				return err
			}
			key, linkID := args[0], args[1]

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, fmt.Sprintf("/issue/%s/remotelink/%s", key, linkID), nil, result)
			})
		},
	}
	return cmd
}

// POST /rest/api/3/issue/{issueIdOrKey}/remotelink
func remoteLinkAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add <issue-key>",
		Short:   "Add a remote link to an issue",
		Example: `jira-agent issue remote-link add PROJ-123 --url "https://example.com" --title "Related doc"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			body := buildRemoteLinkBody(cmd)
			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Post(ctx, "/issue/"+key+"/remotelink", body, result)
			})
		}),
	}
	appendRemoteLinkFlags(cmd)
	return cmd
}

// PUT /rest/api/3/issue/{issueIdOrKey}/remotelink/{linkId}
func remoteLinkEditCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit <issue-key> <remote-link-id>",
		Short:   "Edit a remote link on an issue",
		Example: `jira-agent issue remote-link edit PROJ-123 10001 --url "https://example.com/updated" --title "Updated doc"`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			args, err := requireArgs(args, "issue key", "remote link ID")
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
	appendRemoteLinkFlags(cmd)
	return cmd
}

// DELETE /rest/api/3/issue/{issueIdOrKey}/remotelink/{linkId}
func remoteLinkDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <issue-key> [remote-link-id]",
		Short: "Delete a remote link from an issue",
		Example: `jira-agent issue remote-link delete PROJ-123 10001
jira-agent issue remote-link delete PROJ-123 --global-id system=https://example.com&id=123`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}
			linkID := ""
			path := "/issue/" + key + "/remotelink"
			if globalID := mustGetString(cmd, "global-id"); globalID != "" {
				path = appendQueryParams(path, map[string]string{"globalId": globalID})
			} else {
				args := args
				if len(args) < 2 {
					return apperr.NewValidationError("remote link ID or --global-id is required", nil)
				}
				linkID = args[1]
				path = fmt.Sprintf("/issue/%s/remotelink/%s", key, linkID)
			}

			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "remoteLinkId": linkID, "deleted": true}, *format)
		}),
	}
	cmd.Flags().String("global-id", "", "Delete by stable global ID instead of remote link ID")
	return cmd
}

func buildIssueLinkBody(cmd *cobra.Command) (map[string]any, error) {
	typeName := mustGetString(cmd, "type")
	typeID := mustGetString(cmd, "type-id")
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

	body := map[string]any{
		"type":         linkType,
		"inwardIssue":  map[string]any{"key": mustGetString(cmd, "inward")},
		"outwardIssue": map[string]any{"key": mustGetString(cmd, "outward")},
	}
	if comment := mustGetString(cmd, "comment"); comment != "" {
		body["comment"] = map[string]any{"body": toADF(comment)}
	}

	return body, nil
}

func appendRemoteLinkFlags(cmd *cobra.Command) {
	cmd.Flags().String("url", "", "Remote object URL")
	_ = cmd.MarkFlagRequired("url")
	cmd.Flags().String("title", "", "Remote object title")
	_ = cmd.MarkFlagRequired("title")
	cmd.Flags().String("global-id", "", "Stable global ID for the remote link")
	cmd.Flags().String("relationship", "", "Relationship description, such as mentioned in")
}

func buildRemoteLinkBody(cmd *cobra.Command) map[string]any {
	body := map[string]any{
		"object": map[string]any{
			"url":   mustGetString(cmd, "url"),
			"title": mustGetString(cmd, "title"),
		},
	}
	if globalID := mustGetString(cmd, "global-id"); globalID != "" {
		body["globalId"] = globalID
	}
	if relationship := mustGetString(cmd, "relationship"); relationship != "" {
		body["relationship"] = relationship
	}
	return body
}

func extractIssueLinks(result map[string]any) ([]any, error) {
	return extractFieldArray(result, "issuelinks")
}
