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

// createAndLinkParams holds the parsed flags for the create-and-link composite command.
type createAndLinkParams struct {
	project       string
	issueType     string
	summary       string
	payloadJSON   string
	linkType      string
	linkTarget    string
	linkDirection string
}

// issueCreateAndLinkCommand returns a composite command that creates an issue
// and links it to an existing issue in a single invocation.
func issueCreateAndLinkCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites, dryRun *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-and-link",
		Short: "Create an issue and link it to an existing issue in one step",
		Example: `jira-agent issue create-and-link --project PROJ --type Story --summary "New feature" --link-type Blocks --link-target PROJ-100
jira-agent issue create-and-link --project PROJ --type Bug --summary "Fix" --link-type "is blocked by" --link-target PROJ-200 --link-direction inward
jira-agent issue create-and-link --payload-json '{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task"}}' --link-type Blocks --link-target PROJ-100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			isDry := IsDryRun(dryRun)
			if !isDry {
				if err := requireWriteAccess(allowWrites); err != nil {
					return err
				}
			}

			p := createAndLinkParams{
				project:       resolveProject(cmd),
				issueType:     mustGetString(cmd, "type"),
				summary:       mustGetString(cmd, "summary"),
				payloadJSON:   mustGetString(cmd, "payload-json"),
				linkType:      mustGetString(cmd, "link-type"),
				linkTarget:    mustGetString(cmd, "link-target"),
				linkDirection: mustGetString(cmd, "link-direction"),
			}

		// Validate required fields when not using full payload mode.
		if p.payloadJSON == "" {
			if p.summary == "" {
				return apperr.NewValidationError("--summary is required when not using --payload-json", nil)
			}
			if p.issueType == "" {
				return apperr.NewValidationError("--type is required when not using --payload-json", nil)
			}
		}

		opts := CompactOptsFromCmd(cmd)
		if isDry {
			return createAndLinkDryRun(w, *format, &p, opts...)
		}
			return createAndLinkExecute(cmd, cmd.Context(), apiClient, w, *format, &p, opts...)
		},
	}
	cmd.Flags().String("type", "", "Issue type name (e.g. Story, Bug, Task)")
	cmd.Flags().String("summary", "", "Issue summary")
	cmd.Flags().String("description", "", "Description text, ADF JSON, or wiki markup with --description-format")
	cmd.Flags().String("description-format", "auto", "Description input format: auto, plain, adf, or wiki")
	cmd.Flags().String("assignee", "", "Assignee account ID")
	cmd.Flags().String("priority", "", "Priority name (e.g. High, Medium, Low)")
	cmd.Flags().String("labels", "", "Comma-separated labels")
	cmd.Flags().String("components", "", "Comma-separated component names")
	cmd.Flags().String("parent", "", "Parent issue key (for subtasks/child issues)")
	cmd.Flags().StringToString("field", map[string]string{}, "Custom field value (key=value, repeatable)")
	cmd.Flags().String("fields-json", "", "JSON object of fields (alternative to individual flags)")
	cmd.Flags().String("payload-json", "", "Full JSON issue create payload (mutually exclusive with individual field flags)")
	cmd.Flags().String("link-type", "", "Link type name (e.g. Blocks, \"is blocked by\")")
	_ = cmd.MarkFlagRequired("link-type")
	cmd.Flags().String("link-target", "", "Issue key to link to (e.g. PROJ-100)")
	_ = cmd.MarkFlagRequired("link-target")
	cmd.Flags().String("link-direction", "outward", "Link direction: outward (default) or inward")
	for _, flag := range []string{"summary", "type", "description", "assignee", "priority", "labels", "components", "parent", "field", "fields-json"} {
		markMutuallyExclusive(cmd, "payload-json", flag)
	}
	return cmd
}

// createAndLinkDryRun outputs what would be created and linked without making
// any API calls.
func createAndLinkDryRun(w io.Writer, format output.Format, p *createAndLinkParams, opts ...output.WriteOption) error {
	before := map[string]any{}
	after := map[string]any{
		"link_type":   p.linkType,
		"link_target": p.linkTarget,
	}
	if p.payloadJSON != "" {
		after["payload_json"] = "(provided)"
	} else {
		after["summary"] = p.summary
		after["type"] = p.issueType
	}
	if p.project != "" {
		after["project"] = p.project
	}
	return WriteDryRunResult(w, DryRunResult{
		Command:  "issue create-and-link",
		IssueKey: "(new)",
		Before:   before,
		After:    after,
		Diff:     ComputeFieldDiff(before, after),
	}, format, opts...)
}

// createAndLinkExecute creates an issue and then links it to the target. If the
// create succeeds but the link fails, a partial result is returned with a
// remediation command.
func createAndLinkExecute(cmd *cobra.Command, ctx context.Context, apiClient *client.Ref, w io.Writer, format output.Format, p *createAndLinkParams, opts ...output.WriteOption) error {
	// Build create payload.
	body, err := buildCreatePayload(cmd, p)
	if err != nil {
		return err
	}

	// POST /rest/api/3/issue
	var createResult map[string]any
	if err := apiClient.Post(ctx, "/issue", body, &createResult); err != nil {
		return err
	}

	newKey, _ := createResult["key"].(string)
	if newKey == "" {
		return apperr.NewAPIError("create response missing issue key", 0, "", nil)
	}

	// Build and POST /rest/api/3/issueLink.
	linkBody := buildLinkPayload(p.linkType, newKey, p.linkTarget, p.linkDirection)
	if err := apiClient.Post(ctx, "/issueLink", linkBody, nil); err != nil {
		// Partial failure: issue created, link failed.
		result := map[string]any{
			"key":    newKey,
			"linked": false,
			"next_command": fmt.Sprintf(
				"jira-agent issue link add --type %q --inward %s --outward %s",
				p.linkType, linkInwardKey(newKey, p.linkTarget, p.linkDirection), linkOutwardKey(newKey, p.linkTarget, p.linkDirection),
			),
		}
		return output.WritePartial(w, result, []string{"link: " + err.Error()}, output.NewMetadata(), format, opts...)
	}

	result := map[string]any{
		"key":         newKey,
		"linked":      true,
		"link_type":   p.linkType,
		"link_target": p.linkTarget,
	}
	return output.WriteResult(w, result, format, opts...)
}

// buildCreatePayload constructs the issue create body from command flags or
// --payload-json, reusing the same helpers as issueCreateCommand.
func buildCreatePayload(cmd *cobra.Command, p *createAndLinkParams) (map[string]any, error) {
	if p.payloadJSON != "" {
		body := map[string]any{}
		if err := mergePayloadJSON(body, p.payloadJSON); err != nil {
			return nil, err
		}
		// Inject project when the payload omits it.
		if p.project != "" {
			fields, ok := body["fields"].(map[string]any)
			if !ok {
				fields = map[string]any{}
				body["fields"] = fields
			}
			if _, hasProject := fields["project"]; !hasProject {
				fields["project"] = map[string]any{"key": p.project}
			}
		}
		return body, nil
	}

	// Individual flags mode requires project, type, and summary.
	if p.project == "" {
		return nil, apperr.NewValidationError(
			"--project is required",
			nil,
			apperr.WithDetails("use --project flag or set JIRA_PROJECT env var"),
		)
	}
	if p.issueType == "" {
		return nil, apperr.NewValidationError("--type is required", nil)
	}
	if p.summary == "" {
		return nil, apperr.NewValidationError("--summary is required", nil)
	}

	fields := map[string]any{
		"project":   map[string]any{"key": p.project},
		"issuetype": map[string]any{"name": p.issueType},
		"summary":   p.summary,
	}

	if err := applyCommonFields(fields, cmd); err != nil {
		return nil, err
	}
	if err := applyMerges(fields, cmd); err != nil {
		return nil, err
	}

	return map[string]any{"fields": fields}, nil
}

// buildLinkPayload constructs the issue link body for POST /issueLink.
func buildLinkPayload(linkType, newKey, targetKey, direction string) map[string]any {
	return map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": linkInwardKey(newKey, targetKey, direction)},
		"outwardIssue": map[string]any{"key": linkOutwardKey(newKey, targetKey, direction)},
	}
}

// linkInwardKey returns the key that should appear as inwardIssue based on
// direction. For "inward", the new issue is inward; for "outward" (default),
// the target is inward.
func linkInwardKey(newKey, targetKey, direction string) string {
	if direction == "inward" {
		return newKey
	}
	return targetKey
}

// linkOutwardKey returns the key that should appear as outwardIssue based on
// direction. For "outward" (default), the new issue is outward; for "inward",
// the target is outward.
func linkOutwardKey(newKey, targetKey, direction string) string {
	if direction == "inward" {
		return targetKey
	}
	return newKey
}

// moveToSprintParams holds the parsed flags for the move-to-sprint composite command.
type moveToSprintParams struct {
	key          string
	sprintID     int64
	targetStatus string
	comment      string
	rankBefore   string
	rankAfter    string
}

// issueMoveToSprintCommand returns a composite command that moves an issue to a
// sprint using the Agile API, with optional status transition and comment.
func issueMoveToSprintCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites, dryRun *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move-to-sprint <issue-key>",
		Short: "Move an issue to a sprint with optional transition and comment",
		Example: `jira-agent issue move-to-sprint PROJ-123 --sprint-id 42
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --status "In Progress"
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --comment "Moved to sprint" --rank-before PROJ-100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := requireArg(args, "issue key")
			if err != nil {
				return err
			}

			sprintIDStr, err := requireFlag(cmd, "sprint-id")
			if err != nil {
				return err
			}
			sprintID, err := parseSprintID(sprintIDStr)
			if err != nil {
				return err
			}

			isDry := IsDryRun(dryRun)
			if !isDry {
				if err := requireWriteAccess(allowWrites); err != nil {
					return err
				}
			}

			p := moveToSprintParams{
				key:          key,
				sprintID:     sprintID,
				targetStatus: mustGetString(cmd, "status"),
				comment:      mustGetString(cmd, "comment"),
				rankBefore:   mustGetString(cmd, "rank-before"),
				rankAfter:    mustGetString(cmd, "rank-after"),
			}

			opts := CompactOptsFromCmd(cmd)
			ctx := cmd.Context()
			if isDry {
				before, err := moveToSprintPrepare(ctx, apiClient, p.key)
				if err != nil {
					return err
				}
				// Validate transition exists when --status is set so
				// dry-run does not preview a status the live path would
				// reject after the sprint move has already happened.
				if p.targetStatus != "" {
					var transitions any
					if err := apiClient.Get(ctx, "/issue/"+p.key+"/transitions", nil, &transitions); err != nil {
						return err
					}
					if _, err := findTransitionID(transitions, p.targetStatus); err != nil {
						return err
					}
				}
				return moveToSprintDryRun(w, *format, &p, before, opts...)
			}
			return moveToSprintExecute(ctx, apiClient, w, *format, &p, opts...)
		},
	}
	cmd.Flags().String("sprint-id", "", "Sprint ID to move the issue to (required)")
	_ = cmd.MarkFlagRequired("sprint-id")
	cmd.Flags().String("status", "", "Transition issue to this status after moving")
	cmd.Flags().String("comment", "", "Add a comment after the operation")
	cmd.Flags().String("rank-before", "", "Rank issue before this issue key in the sprint")
	cmd.Flags().String("rank-after", "", "Rank issue after this issue key in the sprint")
	return cmd
}

// moveToSprintPrepare fetches the current issue state for dry-run diff.
func moveToSprintPrepare(ctx context.Context, apiClient *client.Ref, key string) (map[string]any, error) {
	var issueData any
	if err := apiClient.Get(ctx, "/issue/"+key, map[string]string{"fields": "status"}, &issueData); err != nil {
		return nil, err
	}
	return extractMoveToSprintState(issueData), nil
}

// moveToSprintDryRun computes the expected diff and writes a dry-run result.
func moveToSprintDryRun(w io.Writer, format output.Format, p *moveToSprintParams, before map[string]any, opts ...output.WriteOption) error {
	after := maps.Clone(before)
	after["sprint"] = p.sprintID
	if p.targetStatus != "" {
		after["status"] = p.targetStatus
	}
	if p.comment != "" {
		after["comment"] = p.comment
	}
	return WriteDryRunResult(w, DryRunResult{
		Command:  "issue move-to-sprint",
		IssueKey: p.key,
		Before:   before,
		After:    after,
		Diff:     ComputeFieldDiff(before, after),
	}, format, opts...)
}

// moveToSprintExecute performs the sprint move, optional transition, and
// optional comment. Sprint move failure is fatal; transition and comment
// failures are partial.
func moveToSprintExecute(ctx context.Context, apiClient *client.Ref, w io.Writer, format output.Format, p *moveToSprintParams, opts ...output.WriteOption) error {
	result := map[string]any{"key": p.key}

	// Sprint move is the primary operation; failure is fatal.
	body := map[string]any{"issues": []string{p.key}}
	if p.rankBefore != "" {
		body["rankBeforeIssue"] = p.rankBefore
	}
	if p.rankAfter != "" {
		body["rankAfterIssue"] = p.rankAfter
	}
	sprintPath := fmt.Sprintf("/sprint/%d/issue", p.sprintID)
	if err := apiClient.AgilePost(ctx, sprintPath, body, nil); err != nil {
		return err
	}
	result["moved_to_sprint"] = p.sprintID

	var errMsgs []string

	// Optional transition after sprint move.
	if p.targetStatus != "" {
		if err := moveToSprintTransition(ctx, apiClient, p.key, p.targetStatus); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("transition: %v", err))
			result["transitioned"] = false
			result["next_command"] = fmt.Sprintf("jira-agent issue transition %s --to %q", p.key, p.targetStatus)
		} else {
			result["transitioned"] = true
			result["to"] = p.targetStatus
		}
	}

	// Optional comment after move (and transition if any).
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

// moveToSprintTransition fetches available transitions and performs the status
// transition for the move-to-sprint command.
func moveToSprintTransition(ctx context.Context, apiClient *client.Ref, key, targetStatus string) error {
	var transitions any
	if err := apiClient.Get(ctx, "/issue/"+key+"/transitions", nil, &transitions); err != nil {
		return err
	}
	transitionID, err := findTransitionID(transitions, targetStatus)
	if err != nil {
		return err
	}
	transBody := map[string]any{"transition": map[string]any{"id": transitionID}}
	return apiClient.Post(ctx, "/issue/"+key+"/transitions", transBody, nil)
}

// extractMoveToSprintState extracts status name from a Jira issue response
// for use in the move-to-sprint command's dry-run diff computation.
func extractMoveToSprintState(issueData any) map[string]any {
	state := map[string]any{
		"status": nil,
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
