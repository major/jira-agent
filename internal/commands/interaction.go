package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueWatcherCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "watcher",
		Usage: "Watcher operations (list, add, remove)",
		UsageText: `jira-agent issue watcher list PROJ-123
jira-agent issue watcher add PROJ-123 --account-id 5b10a284
jira-agent issue watcher remove PROJ-123 --account-id 5b10a284`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			watcherListCommand(apiClient, w, format),
			watcherAddCommand(apiClient, w, format, allowWrites),
			watcherRemoveCommand(apiClient, w, format, allowWrites),
		},
	}
}

func issueVoteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "vote",
		Usage: "Vote operations (get, add, remove)",
		UsageText: `jira-agent issue vote get PROJ-123
jira-agent issue vote add PROJ-123
jira-agent issue vote remove PROJ-123`,
		Commands: []*cli.Command{
			voteGetCommand(apiClient, w, format),
			voteAddCommand(apiClient, w, format, allowWrites),
			voteRemoveCommand(apiClient, w, format, allowWrites),
		},
	}
}

func issueAttachmentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:  "attachment",
		Usage: "Attachment operations (list, add, get, delete)",
		UsageText: `jira-agent issue attachment list PROJ-123
jira-agent issue attachment add PROJ-123 --file report.pdf
jira-agent issue attachment get 10001`,
		DefaultCommand: "list",
		Commands: []*cli.Command{
			attachmentListCommand(apiClient, w, format),
			attachmentAddCommand(apiClient, w, format, allowWrites),
			attachmentGetCommand(apiClient, w, format),
			attachmentDeleteCommand(apiClient, w, format, allowWrites),
		},
	}
}

func watcherListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List watchers on an issue",
		UsageText: `jira-agent issue watcher list PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/watchers", nil, result)
			})
		},
	}
}

func watcherAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a watcher to an issue",
		UsageText: `jira-agent issue watcher add PROJ-123 --account-id 5b10a2844c20165700ede21g`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "account-id", Usage: "Watcher account ID", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			accountID := cmd.String("account-id")
			if err := apiClient.Post(ctx, "/issue/"+key+"/watchers", accountID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "accountId": accountID, "added": true}, *format)
		}),
	}
}

func watcherRemoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove a watcher from an issue",
		UsageText: `jira-agent issue watcher remove PROJ-123 --account-id 5b10a2844c20165700ede21g`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "account-id", Usage: "Watcher account ID", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			accountID := cmd.String("account-id")
			path := appendQueryParams("/issue/"+key+"/watchers", map[string]string{"accountId": accountID})
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "accountId": accountID, "removed": true}, *format)
		}),
	}
}

func voteGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get votes on an issue",
		UsageText: `jira-agent issue vote get PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/votes", nil, result)
			})
		},
	}
}

func voteAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add your vote to an issue",
		UsageText: `jira-agent issue vote add PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			if err := apiClient.Post(ctx, "/issue/"+key+"/votes", nil, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "voted": true}, *format)
		}),
	}
}

func voteRemoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove your vote from an issue",
		UsageText: `jira-agent issue vote remove PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/issue/"+key+"/votes", nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "voted": false}, *format)
		}),
	}
}

func attachmentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List attachments on an issue",
		UsageText: `jira-agent issue attachment list PROJ-123`,
		ArgsUsage: "<issue-key>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			var result map[string]any
			if err := apiClient.Get(ctx, "/issue/"+key, map[string]string{"fields": "attachment"}, &result); err != nil {
				return err
			}
			attachments, err := extractAttachments(result)
			if err != nil {
				return err
			}
			meta := output.NewMetadata()
			meta.Total = len(attachments)
			meta.Returned = len(attachments)
			return output.WriteSuccess(w, attachments, meta, *format)
		},
	}
}

func attachmentAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add an attachment to an issue",
		UsageText: `jira-agent issue attachment add PROJ-123 --file report.pdf
jira-agent issue attachment add PROJ-123 --file doc.pdf --file image.png`,
		ArgsUsage: "<issue-key>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "file", Usage: "File path to attach (repeatable)", Required: true},
		},
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			key, err := requireArg(cmd, "issue key")
			if err != nil {
				return err
			}

			files, closeFiles, err := openAttachmentFiles(cmd.StringSlice("file"))
			if err != nil {
				return err
			}
			defer closeFiles()

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.PostMultipart(ctx, "/issue/"+key+"/attachments", files, result)
			})
		}),
	}
}

func attachmentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get attachment metadata",
		UsageText: `jira-agent issue attachment get 10001`,
		ArgsUsage: "<attachment-id>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			attachmentID, err := requireArg(cmd, "attachment ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/attachment/"+attachmentID, nil, result)
			})
		},
	}
}

func attachmentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an attachment",
		UsageText: `jira-agent issue attachment delete 10001`,
		ArgsUsage: "<attachment-id>",
		Action: writeGuard(allowWrites, func(ctx context.Context, cmd *cli.Command) error {
			attachmentID, err := requireArg(cmd, "attachment ID")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/attachment/"+attachmentID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"attachmentId": attachmentID, "deleted": true}, *format)
		}),
	}
}

func openAttachmentFiles(paths []string) ([]client.MultipartFile, func(), error) {
	if len(paths) == 0 {
		return nil, nil, apperr.NewValidationError("--file is required", nil)
	}

	opened := make([]*os.File, 0, len(paths))
	closeFiles := func() {
		for _, file := range opened {
			_ = file.Close()
		}
	}

	files := make([]client.MultipartFile, 0, len(paths))
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			closeFiles()
			return nil, nil, apperr.NewValidationError("open attachment file: "+path, err)
		}
		opened = append(opened, file)
		files = append(files, client.MultipartFile{
			FieldName: "file",
			FileName:  filepath.Base(path),
			Reader:    file,
		})
	}

	return files, closeFiles, nil
}

func extractAttachments(result map[string]any) ([]any, error) {
	return extractFieldArray(result, "attachment")
}
