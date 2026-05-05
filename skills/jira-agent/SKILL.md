---
name: jira-agent
description: "Jira Cloud CLI for AI agents. Structured JSON/CSV/TSV output, semantic exit codes. Covers: issue CRUD, search, bulk ops (create/fetch/delete/edit/move/transition), transitions, assignments, comments, worklogs, watchers, votes, attachments, issue links, remote links, changelog, ranking, notifications; agile boards, sprints, epics, backlogs; projects, components, versions; fields (contexts/options), users, groups, filters, permissions, dashboards, workflows, statuses, priorities, resolutions, issue types, labels, JQL helpers, audit records, tasks, time tracking configuration, and server info. Triggers: 'jira', 'jira issue', 'jira search', 'jql', 'jira create', 'jira bulk', 'jira transition', 'jira assign', 'jira comment', 'jira worklog', 'jira sprint', 'jira epic', 'jira board', 'jira backlog', 'jira component', 'jira version', 'jira project', 'jira field', 'jira user', 'jira group', 'jira filter', 'jira permission', 'jira dashboard', 'jira workflow', 'jira status', 'jira-agent'."
metadata:
  author: "Major Hayden"
  version: "2.0.0"
---

# jira-agent CLI

Go CLI for Jira Cloud REST API v3. All output is structured, errors always JSON, exit codes are semantic.

## Feedback

If you hit bugs, confusing usability, missing guidance, or token-inefficient workflows while using `jira-agent`, encourage the user to open a GitHub issue at `github.com/major/jira-agent`. Offer to open the issue for them with GitHub's `gh` CLI if it is installed and the user wants you to file it.

## Companion Files

This is the entry point. Command details are split by theme:

| File | Scope |
|------|-------|
| [issues.md](issues.md) | Issue CRUD, search, bulk ops, meta, count |
| [issue-workflows.md](issue-workflows.md) | Transition, assign, comment, worklog, watcher, vote, attachment, link, remote-link, changelog, rank, notify |
| [agile.md](agile.md) | Board, sprint, epic, backlog |
| [project-management.md](project-management.md) | Project, project property, component, version |
| [admin-reference.md](admin-reference.md) | Field, user, group, filter, permission, dashboard, workflow, status, priority, resolution, issuetype, label, JQL, audit records, tasks, time tracking, server-info |

## Auth

Env vars (override config file):

```bash
export JIRA_INSTANCE="your-domain.atlassian.net"
export JIRA_EMAIL="you@example.com"
export JIRA_API_KEY="your-api-token"
export JIRA_ALLOW_WRITES=true  # optional: enable write operations
```

Config file fallback: `~/.config/jira-agent/config.json` (XDG_CONFIG_HOME aware). Verify with `jira-agent whoami`.

### Write Protection

Writes (create, edit, delete, transition, assign) are disabled by default. Enable:

- Config: `"i-too-like-to-live-dangerously": true`
- Env: `JIRA_ALLOW_WRITES=true`

Blocked writes return exit 5 with remediation. Read-only commands always work. `issue transition --list` is read-only.

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | | Default Jira project key (also `JIRA_PROJECT` env) |
| `--output` | `-o` | `json` | `json`, `csv`, or `tsv` |
| `--compact` | | off | Strip nulls/empties from JSON, flatten single-key objects, JSON Lines for arrays |
| `--dry-run` | | off | Preview field changes without mutating (composite commands only) |
| `--verbose` | `-v` | off | Verbose logging to stderr |
| `--config` | | | Config file path override |

## Schema Discovery

`jira-agent schema` outputs the full CLI structure as JSON without requiring auth. Use it to discover all commands, flags, flag groups, categories, and constraints.

```bash
jira-agent schema
jira-agent schema --output json
```

The output includes `commands[]` with `path`, `description`, `category`, `write_protected`, `requires_auth`, `flags[]`, `flag_groups[]`, and `examples[]` for each command.

## Command Discovery

Use `--help` to explore commands and flags before guessing:

```bash
jira-agent --help                  # top-level command list
jira-agent issue --help            # issue subcommands
jira-agent issue create --help     # flags for a specific command
```

For reads/searches, request only needed fields: `--fields key,summary,status`. See **JSON shape** for default compaction and raw-output guidance. Use CSV/TSV for simple tables, JSON for updates or nested data.

## Output

### Success envelope (JSON)

```json
{
  "data": { ... },
  "errors": [],
  "metadata": { "timestamp": "...", "total": 42, "returned": 10, "start_at": 0, "max_results": 50, "has_more": true, "next_command": "jira-agent issue search --jql '...' --max-results 10 --start-at 10" }
}
```

Access results via `.data`. Check `metadata.has_more` for pagination. When `has_more` is true, `metadata.next_command` contains the ready-to-run command for the next page.

### Error response (always JSON, regardless of --output)

```json
{
  "error": { "code": "NOT_FOUND", "message": "transition 'Done' not found", "details": "...", "next_command": "jira-agent issue transition PROJ-123", "available_actions": ["To Do", "In Progress", "Done"] }
}
```

`next_command` and `available_actions` appear in errors when applicable. Use them to recover without guessing.

| Code | Exit | Meaning |
|------|------|---------|
| `AUTH_FAILED` | 3 | Missing or invalid credentials |
| `NOT_FOUND` | 2 | Resource does not exist |
| `API_ERROR` | 4 | Jira API error |
| `VALIDATION_ERROR` | 5 | Invalid input or blocked write |
| `UNKNOWN` | 1 | Unexpected error |

### CSV/TSV

Flat rows with header, no envelope. Nested values become inline JSON in cells.

## Composite Workflow Commands

Composite commands collapse multiple Jira API calls into one CLI invocation. All require `JIRA_ALLOW_WRITES=true`. Use `--dry-run` to preview field changes without mutating.

```bash
jira-agent issue start-work PROJ-123 --dry-run
```

Dry-run output includes `diff[]` with `field`, `old_value`, and `new_value` for each planned change.

### issue start-work

Transitions to In Progress, assigns to current user, and optionally adds a comment.

```bash
jira-agent issue start-work PROJ-123
jira-agent issue start-work PROJ-123 --status "In Review" --comment "Starting review"
jira-agent issue start-work PROJ-123 --assignee abc123 --skip-transition
jira-agent issue start-work PROJ-123 --skip-assign --comment "Picked up"
```

### issue close

Transitions to Done, sets resolution, and optionally adds a comment.

```bash
jira-agent issue close PROJ-123
jira-agent issue close PROJ-123 --resolution "Won't Do"
jira-agent issue close PROJ-123 --status "Closed" --resolution "Duplicate" --comment "Duplicate of PROJ-100"
```

### issue create-and-link

Creates an issue and links it to an existing issue in one call.

```bash
jira-agent issue create-and-link --project PROJ --type Story --summary "New feature" --link-type Blocks --link-target PROJ-100
jira-agent issue create-and-link --project PROJ --type Bug --summary "Fix" --link-type "is blocked by" --link-target PROJ-200
jira-agent issue create-and-link --payload-json '{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task"}}' --link-type Blocks --link-target PROJ-100
```

### issue move-to-sprint

Moves an issue to a sprint using the Agile API, with optional transition and comment.

```bash
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --status "In Progress"
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --comment "Moved to sprint" --rank-before PROJ-100
```

## Smart Shortcuts

### issue mine

Returns issues assigned to the current user, ordered by last updated.

```bash
jira-agent issue mine
jira-agent issue mine --status "In Progress"
jira-agent issue mine --fields key,summary,status,priority --max-results 20
jira-agent issue mine --fields-preset triage
```

### issue recent

Returns issues recently touched by the current user (assigned, reported, or watching). Default window is 7 days.

```bash
jira-agent issue recent
jira-agent issue recent --since 14d
jira-agent issue recent --fields key,summary,status,assignee --max-results 20
jira-agent issue recent --fields-preset minimal
```

### sprint current

Returns the active sprint for a board.

```bash
jira-agent sprint current --board-id 42
```

Returns a single sprint object when one active sprint exists, or an array when multiple are active. Returns a not-found error with remediation when no active sprint exists.

## Semantic JQL Flags

`issue search` accepts convenience flags that build JQL automatically. These are mutually exclusive with `--jql`.

```bash
jira-agent issue search --assignee me
jira-agent issue search --status "In Progress" --priority High
jira-agent issue search --sprint current
jira-agent issue search --type Bug --label backend
jira-agent issue search --updated-since 7d
jira-agent issue search --assignee me --status "In Progress" --sprint current
```

Flags: `--assignee` (use `"me"` for `currentUser()`), `--status`, `--type`, `--priority`, `--label` (repeatable), `--sprint` (use `"current"` for `openSprints()`), `--updated-since` (e.g. `"7d"`). Conditions join with AND in alphabetical order.

## Token-Efficient Output

`--compact` strips null values, empty strings, and empty arrays from JSON output. Single-key nested objects flatten to dot notation. Arrays output as JSON Lines (one envelope per line).

```bash
jira-agent issue get PROJ-123 --compact
jira-agent issue search --jql "project = PROJ" --compact
```

`--fields-preset` selects a predefined field set. Mutually exclusive with `--fields`.

```bash
jira-agent issue get PROJ-123 --fields-preset minimal
jira-agent issue search --jql "project = PROJ" --fields-preset triage
```

Presets: `minimal` (key, summary, status), `triage` (+ priority, assignee, labels), `detail` (+ description, created, updated).

## Pagination

Paginated responses include `has_more` (bool, always present) and `next_command` (string, present when `has_more` is true) in `metadata`.

```bash
jira-agent issue search --jql "project = PROJ" --max-results 10
# metadata.has_more: true
# metadata.next_command: "jira-agent issue search --jql 'project = PROJ' --max-results 10 --start-at 10"
```

## Gotchas

- **Description**: `--description` defaults to auto mode on writes: plain text auto-converts to ADF and valid ADF JSON passes through. Use `--description-format wiki` for Jira wiki markup like `h4.` headings and `*` bullet lists. `issue get` and `issue search` default description output to plain text; use `--description-output-format markdown` for headings/lists, `adf` for compact ADF, or `--raw` for Jira's unmodified response.
- **Custom fields**: `--field key=value` parses value as JSON if valid, else raw string. Quote carefully in shell.
- **Project flag**: Command-level `--project` overrides root `-p`.
- **Type resolution**: Issue type matching is case-insensitive.
- **Transition resolution**: `issue transition --to` matches status/transition name case-insensitively. Use `--transition-id` when you already know Jira's numeric transition ID.
- **Pagination**: `issue search` uses `--next-page-token` (cursor). Most other list commands use `--start-at` (offset).
- **JSON shape**: Default JSON removes common Jira metadata noise, including `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. `issue search` additionally flattens common objects to useful scalar values, for example `status` to its name and `assignee` to display name. `issue get --raw` and `issue search --raw` restore Jira's nested response and bypass description conversion; use `--help` to check flag availability on any command.
- **Assignment**: `issue assign` accepts account ID, not email. `--unassign` clears, `--default` uses project default.
- **Visibility**: Both `--visibility-type` and `--visibility-value` must be set together.
- **Write protection**: All writes blocked unless `JIRA_ALLOW_WRITES=true` or config set. Exit 5 with remediation.
