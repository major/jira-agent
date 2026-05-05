package commands

import (
	"context"
	"fmt"
	"io"
	"maps"

	"github.com/spf13/cobra"

	"github.com/major/jira-agent/internal/client"
	apperr "github.com/major/jira-agent/internal/errors"
	"github.com/major/jira-agent/internal/output"
)

// startWorkParams holds the parsed flags for the start-work composite command.
type startWorkParams struct {
	key            string
	targetStatus   string
	assigneeID     string
	comment        string
	skipAssign     bool
	skipTransition bool
}

// startWorkState holds the read-only state gathered during the prepare phase.
type startWorkState struct {
	before       map[string]any
	transitionID string
}

// issueStartWorkCommand returns a composite command that transitions an issue,
// assigns it, and optionally adds a comment in a single invocation.
func issueStartWorkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites, dryRun *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-work <issue-key>",
		Short: "Transition, assign, and optionally comment on an issue in one step",
		Example: `jira-agent issue start-work PROJ-123
jira-agent issue start-work PROJ-123 --status "In Review" --comment "Starting review"
jira-agent issue start-work PROJ-123 --skip-assign --comment "Picked up"
jira-agent issue start-work PROJ-123 --assignee abc123 --skip-transition`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			isDry := IsDryRun(dryRun)
			if !isDry {
				if err := requireWriteAccess(allowWrites); err != nil {
					return err
				}
			}

			p := startWorkParams{
				key:            key,
				targetStatus:   mustGetString(cmd, "status"),
				assigneeID:     mustGetString(cmd, "assignee"),
				comment:        mustGetString(cmd, "comment"),
				skipAssign:     mustGetBool(cmd, "skip-assign"),
				skipTransition: mustGetBool(cmd, "skip-transition"),
			}

			ctx := cmd.Context()
			state, err := startWorkPrepare(ctx, apiClient, &p)
			if err != nil {
				return err
			}

			opts := CompactOptsFromCmd(cmd)
			if isDry {
				return startWorkDryRun(w, *format, p, state, opts...)
			}
			return startWorkExecute(ctx, apiClient, w, *format, p, state, opts...)
		},
	}
	cmd.Flags().String("status", "In Progress", "Target status for transition")
	cmd.Flags().String("assignee", "", "Account ID to assign (default: self)")
	cmd.Flags().String("comment", "", "Comment to add after transition")
	cmd.Flags().Bool("skip-assign", false, "Skip assignment step")
	cmd.Flags().Bool("skip-transition", false, "Skip transition step")
	return cmd
}

// startWorkPrepare resolves the assignee, fetches issue state, and finds the
// transition ID. It mutates p.assigneeID when resolving self.
func startWorkPrepare(ctx context.Context, apiClient *client.Ref, p *startWorkParams) (startWorkState, error) {
	var state startWorkState

	// Resolve self account ID when assignment is requested without explicit ID.
	if !p.skipAssign && p.assigneeID == "" {
		id, err := resolveAccountID(ctx, apiClient)
		if err != nil {
			return state, err
		}
		p.assigneeID = id
	}

	// Get current issue state for context and dry-run diff.
	var issueData any
	if err := apiClient.Get(ctx, "/issue/"+p.key, map[string]string{"fields": "status,assignee"}, &issueData); err != nil {
		return state, err
	}
	state.before = extractIssueState(issueData)

	// Find transition ID when transition is requested.
	if !p.skipTransition {
		var transitions any
		if err := apiClient.Get(ctx, "/issue/"+p.key+"/transitions", nil, &transitions); err != nil {
			return state, err
		}
		var err error
		state.transitionID, err = findTransitionID(transitions, p.targetStatus)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

// startWorkDryRun computes the expected diff and writes a dry-run result.
func startWorkDryRun(w io.Writer, format output.Format, p startWorkParams, state startWorkState, opts ...output.WriteOption) error {
	after := maps.Clone(state.before)
	if !p.skipTransition {
		after["status"] = p.targetStatus
	}
	if !p.skipAssign {
		after["assignee"] = p.assigneeID
	}
	if p.comment != "" {
		after["comment"] = p.comment
	}
	return WriteDryRunResult(w, DryRunResult{
		Command:  "issue start-work",
		IssueKey: p.key,
		Before:   state.before,
		After:    after,
		Diff:     ComputeFieldDiff(state.before, after),
	}, format, opts...)
}

// startWorkExecute performs the transition, assignment, and comment mutations.
// Transition failure is fatal; assignment and comment failures are partial.
func startWorkExecute(ctx context.Context, apiClient *client.Ref, w io.Writer, format output.Format, p startWorkParams, state startWorkState, opts ...output.WriteOption) error {
	result := map[string]any{"key": p.key}
	var errMsgs []string

	if !p.skipTransition {
		body := map[string]any{"transition": map[string]any{"id": state.transitionID}}
		if err := apiClient.Post(ctx, "/issue/"+p.key+"/transitions", body, nil); err != nil {
			return err
		}
		result["transitioned"] = true
		result["to"] = p.targetStatus
	}

	if !p.skipAssign {
		body := map[string]any{"accountId": p.assigneeID}
		if err := apiClient.Put(ctx, "/issue/"+p.key+"/assignee", body, nil); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("assign: %v", err))
			result["assigned"] = false
			result["next_command"] = fmt.Sprintf("jira-agent issue assign %s %s", p.key, p.assigneeID)
		} else {
			result["assigned"] = true
			result["assignee"] = p.assigneeID
		}
	}

	if p.comment != "" {
		body := map[string]any{"body": toADF(p.comment)}
		if err := apiClient.Post(ctx, "/issue/"+p.key+"/comment", body, nil); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("comment: %v", err))
			result["commented"] = false
		} else {
			result["commented"] = true
		}
	}

	if len(errMsgs) > 0 {
		return output.WritePartial(w, result, errMsgs, output.NewMetadata(), format, opts...)
	}
	return output.WriteResult(w, result, format, opts...)
}

// resolveAccountID calls GET /myself and returns the current user's account ID.
func resolveAccountID(ctx context.Context, apiClient *client.Ref) (string, error) {
	var myself map[string]any
	if err := apiClient.Get(ctx, "/myself", nil, &myself); err != nil {
		return "", err
	}
	id, ok := myself["accountId"].(string)
	if !ok {
		return "", apperr.NewAPIError("could not resolve account ID from /myself response", 0, "", nil)
	}
	return id, nil
}

// closeParams holds the parsed flags for the close composite command.
type closeParams struct {
	key          string
	targetStatus string
	resolution   string
	comment      string
}

// closeState holds the read-only state gathered during the close prepare phase.
type closeState struct {
	before       map[string]any
	transitionID string
}

// issueCloseCommand returns a composite command that transitions an issue to a
// closed status with a resolution and optionally adds a comment.
func issueCloseCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites, dryRun *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <issue-key>",
		Short: "Transition an issue to a closed status with resolution",
		Example: `jira-agent issue close PROJ-123
jira-agent issue close PROJ-123 --resolution "Won't Do"
jira-agent issue close PROJ-123 --status "Closed" --resolution "Duplicate" --comment "Duplicate of PROJ-100"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			isDry := IsDryRun(dryRun)
			if !isDry {
				if err := requireWriteAccess(allowWrites); err != nil {
					return err
				}
			}

			p := closeParams{
				key:          key,
				targetStatus: mustGetString(cmd, "status"),
				resolution:   mustGetString(cmd, "resolution"),
				comment:      mustGetString(cmd, "comment"),
			}

			ctx := cmd.Context()
			state, err := closePrepare(ctx, apiClient, &p)
			if err != nil {
				return err
			}

			opts := CompactOptsFromCmd(cmd)
			if isDry {
				return closeDryRun(w, *format, p, state, opts...)
			}
			return closeExecute(ctx, apiClient, w, *format, p, state, opts...)
		},
	}
	cmd.Flags().String("status", "Done", "Target status for transition")
	cmd.Flags().String("resolution", "Done", "Resolution value (e.g. Done, Won't Do, Duplicate)")
	cmd.Flags().String("comment", "", "Comment to add after transition")
	return cmd
}

// closePrepare fetches issue state and finds the transition ID for the close
// command.
func closePrepare(ctx context.Context, apiClient *client.Ref, p *closeParams) (closeState, error) {
	var state closeState

	// Get current issue state for context and dry-run diff.
	var issueData any
	if err := apiClient.Get(ctx, "/issue/"+p.key, map[string]string{"fields": "status,resolution"}, &issueData); err != nil {
		return state, err
	}
	state.before = extractCloseState(issueData)

	// Fetch transitions and find the target transition ID.
	var transitions any
	if err := apiClient.Get(ctx, "/issue/"+p.key+"/transitions", nil, &transitions); err != nil {
		return state, err
	}

	// Collect available transition names for error reporting.
	var availableNames []string
	if tm, ok := transitions.(map[string]any); ok {
		if list, ok := tm["transitions"].([]any); ok {
			for _, t := range list {
				if entry, ok := t.(map[string]any); ok {
					if name, ok := entry["name"].(string); ok {
						availableNames = append(availableNames, name)
					}
				}
			}
		}
	}

	var err error
	state.transitionID, err = findTransitionID(transitions, p.targetStatus)
	if err != nil {
		return state, apperr.NewNotFoundError(
			fmt.Sprintf("transition %q not found", p.targetStatus),
			err,
			apperr.WithAvailableActions(availableNames),
		)
	}

	return state, nil
}

// closeDryRun computes the expected diff and writes a dry-run result for the
// close command.
func closeDryRun(w io.Writer, format output.Format, p closeParams, state closeState, opts ...output.WriteOption) error {
	after := maps.Clone(state.before)
	after["status"] = p.targetStatus
	after["resolution"] = p.resolution
	if p.comment != "" {
		after["comment"] = p.comment
	}
	return WriteDryRunResult(w, DryRunResult{
		Command:  "issue close",
		IssueKey: p.key,
		Before:   state.before,
		After:    after,
		Diff:     ComputeFieldDiff(state.before, after),
	}, format, opts...)
}

// closeExecute performs the transition with resolution and optional comment.
// Transition failure is fatal; comment failure is partial.
func closeExecute(ctx context.Context, apiClient *client.Ref, w io.Writer, format output.Format, p closeParams, state closeState, opts ...output.WriteOption) error {
	result := map[string]any{"key": p.key}

	body := map[string]any{
		"transition": map[string]any{"id": state.transitionID},
		"fields":     map[string]any{"resolution": map[string]any{"name": p.resolution}},
	}
	if err := apiClient.Post(ctx, "/issue/"+p.key+"/transitions", body, nil); err != nil {
		return err
	}
	result["transitioned"] = true
	result["to"] = p.targetStatus
	result["resolution"] = p.resolution

	var errMsgs []string
	if p.comment != "" {
		commentBody := map[string]any{"body": toADF(p.comment)}
		if err := apiClient.Post(ctx, "/issue/"+p.key+"/comment", commentBody, nil); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("comment: %v", err))
			result["commented"] = false
		} else {
			result["commented"] = true
		}
	}

	if len(errMsgs) > 0 {
		return output.WritePartial(w, result, errMsgs, output.NewMetadata(), format, opts...)
	}
	return output.WriteResult(w, result, format, opts...)
}

// extractCloseState extracts status name and resolution name from a Jira issue
// response for use in the close command's dry-run diff computation.
func extractCloseState(issueData any) map[string]any {
	state := map[string]any{
		"status":     nil,
		"resolution": nil,
	}
	m, ok := issueData.(map[string]any)
	if !ok {
		return state
	}
	fields, ok := m["fields"].(map[string]any)
	if !ok {
		return state
	}
	if status, ok := fields["status"].(map[string]any); ok {
		if name, ok := status["name"].(string); ok {
			state["status"] = name
		}
	}
	if resolution, ok := fields["resolution"].(map[string]any); ok {
		if name, ok := resolution["name"].(string); ok {
			state["resolution"] = name
		}
	}
	return state
}

// extractIssueState extracts status name and assignee account ID from a Jira
// issue response for use in dry-run diff computation.
func extractIssueState(issueData any) map[string]any {
	state := map[string]any{
		"status":   nil,
		"assignee": nil,
	}
	m, ok := issueData.(map[string]any)
	if !ok {
		return state
	}
	fields, ok := m["fields"].(map[string]any)
	if !ok {
		return state
	}
	if status, ok := fields["status"].(map[string]any); ok {
		if name, ok := status["name"].(string); ok {
			state["status"] = name
		}
	}
	if assignee, ok := fields["assignee"].(map[string]any); ok {
		if id, ok := assignee["accountId"].(string); ok {
			state["assignee"] = id
		}
	}
	return state
}
