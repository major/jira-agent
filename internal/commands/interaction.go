package commands

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

func issueWatcherCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watcher",
		Short: "Watcher operations (list, add, remove)",
		Example: `jira-agent issue watcher list PROJ-123
jira-agent issue watcher add PROJ-123 --account-id 5b10a284
jira-agent issue watcher remove PROJ-123 --account-id 5b10a284`,
	}
	cmd.AddCommand(
		watcherListCommand(apiClient, w, format),
		watcherAddCommand(apiClient, w, format, allowWrites),
		watcherRemoveCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func issueVoteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote",
		Short: "Vote operations (get, add, remove)",
		Example: `jira-agent issue vote get PROJ-123
jira-agent issue vote add PROJ-123
jira-agent issue vote remove PROJ-123`,
	}
	cmd.AddCommand(
		voteGetCommand(apiClient, w, format),
		voteAddCommand(apiClient, w, format, allowWrites),
		voteRemoveCommand(apiClient, w, format, allowWrites),
	)
	return cmd
}

func issueAttachmentCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Attachment operations (list, add, get, delete)",
		Example: `jira-agent issue attachment list PROJ-123
jira-agent issue attachment add PROJ-123 --file report.pdf
jira-agent issue attachment get 10001`,
	}
	cmd.AddCommand(
		attachmentListCommand(apiClient, w, format),
		attachmentAddCommand(apiClient, w, format, allowWrites),
		attachmentGetCommand(apiClient, w, format),
		attachmentDeleteCommand(apiClient, w, format, allowWrites),
	)
	setDefaultSubcommand(cmd, "list")
	return cmd
}

func watcherListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List watchers on an issue",
		Example: `jira-agent issue watcher list PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/watchers", nil, result)
			})
		},
	}
	return cmd
}

func watcherAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add <issue-key>",
		Short:   "Add a watcher to an issue",
		Example: `jira-agent issue watcher add PROJ-123 --account-id 5b10a2844c20165700ede21g`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			accountID := mustGetString(cmd, "account-id")
			if err := apiClient.Post(ctx, "/issue/"+key+"/watchers", accountID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "accountId": accountID, "added": true}, *format)
		}),
	}
	cmd.Flags().String("account-id", "", "Watcher account ID")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func watcherRemoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <issue-key>",
		Short:   "Remove a watcher from an issue",
		Example: `jira-agent issue watcher remove PROJ-123 --account-id 5b10a2844c20165700ede21g`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			accountID := mustGetString(cmd, "account-id")
			path := appendQueryParams("/issue/"+key+"/watchers", map[string]string{"accountId": accountID})
			if err := apiClient.Delete(ctx, path, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "accountId": accountID, "removed": true}, *format)
		}),
	}
	cmd.Flags().String("account-id", "", "Watcher account ID")
	_ = cmd.MarkFlagRequired("account-id")
	return cmd
}

func voteGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <issue-key>",
		Short:   "Get votes on an issue",
		Example: `jira-agent issue vote get PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/issue/"+key+"/votes", nil, result)
			})
		},
	}
	return cmd
}

func voteAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add <issue-key>",
		Short:   "Add your vote to an issue",
		Example: `jira-agent issue vote add PROJ-123`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			if err := apiClient.Post(ctx, "/issue/"+key+"/votes", nil, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "voted": true}, *format)
		}),
	}
	return cmd
}

func voteRemoveCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <issue-key>",
		Short:   "Remove your vote from an issue",
		Example: `jira-agent issue vote remove PROJ-123`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/issue/"+key+"/votes", nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"key": key, "voted": false}, *format)
		}),
	}
	return cmd
}

func attachmentListCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <issue-key>",
		Short:   "List attachments on an issue",
		Example: `jira-agent issue attachment list PROJ-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
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
	return cmd
}

func attachmentAddCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-key>",
		Short: "Add an attachment to an issue",
		Example: `jira-agent issue attachment add PROJ-123 --file report.pdf
jira-agent issue attachment add PROJ-123 --file doc.pdf --file image.png`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			files, closeFiles, err := openAttachmentFiles(mustGetStringSlice(cmd, "file"))
			if err != nil {
				return err
			}
			defer closeFiles()

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.PostMultipart(ctx, "/issue/"+key+"/attachments", files, result)
			})
		}),
	}
	cmd.Flags().StringSlice("file", []string{}, "File path to attach (repeatable)")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func attachmentGetCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <attachment-id>",
		Short:   "Get attachment metadata",
		Example: `jira-agent issue attachment get 10001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			attachmentID, err := requireArg(args, "attachment ID")
			if err != nil {
				return err
			}

			return writeAPIResult(w, *format, func(result any) error {
				return apiClient.Get(ctx, "/attachment/"+attachmentID, nil, result)
			})
		},
	}
	return cmd
}

func attachmentDeleteCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <attachment-id>",
		Short:   "Delete an attachment",
		Example: `jira-agent issue attachment delete 10001`,
		RunE: writeGuard(allowWrites, func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			attachmentID, err := requireArg(args, "attachment ID")
			if err != nil {
				return err
			}

			if err := apiClient.Delete(ctx, "/attachment/"+attachmentID, nil); err != nil {
				return err
			}
			return output.WriteResult(w, map[string]any{"attachmentId": attachmentID, "deleted": true}, *format)
		}),
	}
	return cmd
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
