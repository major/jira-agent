# Issues

Issue CRUD, search, bulk operations, metadata, and count.

## issue get

```bash
jira-agent issue get KEY-123
jira-agent issue get KEY-123 --fields summary,status,assignee --expand changelog
jira-agent issue get KEY-123 --fields description --description-output-format markdown
jira-agent issue get KEY-123 --properties request --fields-by-keys --update-history
jira-agent issue get KEY-123 --raw
```

Default JSON removes common Jira metadata noise such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. Description fields default to plain text to avoid token-heavy ADF JSON; use `--description-output-format markdown` for headings/lists or `--description-output-format adf` to keep ADF in compact output. Useful flags: `--properties`, `--fields-by-keys`, `--update-history`, `--fail-fast=false` for partial field-resolution failures, and `--raw` when you need Jira's unmodified nested response.

## issue picker

```bash
jira-agent issue picker --query KEY-123
jira-agent issue picker --query login --current-jql "project = PROJ" --show-subtasks=false
```

Find issues using Jira's picker endpoint. Useful when the user provides partial keys or summary text instead of exact issue keys.

## issue search

```bash
jira-agent issue search --jql "project = PROJ AND status = 'In Progress'"
jira-agent issue search --jql "assignee = currentUser()" --fields key,summary,status --max-results 20
jira-agent issue search --jql "project = PROJ" --fields key,description --description-output-format markdown
jira-agent issue search --jql "..." --next-page-token TOKEN
jira-agent issue search --jql "project = PROJ" --properties request --fields-by-keys --fail-fast=false
jira-agent issue search --jql "project = PROJ" --raw
```

Default JSON output is flattened for LLM efficiency: `.data.issues[]` contains compact rows such as `key`, `summary`, `status`, `assignee`, and `priority`, with common Jira wrapper fields like `self`, `avatarUrls`, `iconUrl`, and `statusCategory` removed. Use `--raw` only for debugging, scripting against Jira's original response shape, or field discovery that needs the full nested API payload.

| Flag | Default | Notes |
|------|---------|-------|
| `--jql` | (required unless using semantic flags) | JQL query string |
| `--fields` | key,summary,status,assignee,priority | Comma-separated; `key` is returned from the issue object and not sent as a Jira field |
| `--fields-preset` | | Named preset: `minimal`, `triage`, or `detail`; mutually exclusive with `--fields` |
| `--description-output-format` | text | `text`, `markdown`, or `adf`; ignored by `--raw` |
| `--max-results` | 50 | Page size |
| `--next-page-token` | | Cursor from previous `metadata.pagination.next_token` |
| `--expand` | | e.g., `changelog,renderedFields` |
| `--order-by` | | Sort field |
| `--order` | | `asc` or `desc` |
| `--properties` | | Comma-separated issue properties |
| `--fields-by-keys` | false | Treat field identifiers as field keys |
| `--fail-fast` | true | Set false to tolerate unresolved fields |
| `--reconcile-issues` | | Comma-separated issue IDs for Jira reconciliation |
| `--raw` | false | Return the unmodified Jira search response instead of compact issue rows |

### Semantic JQL flags

These flags build JQL automatically and are mutually exclusive with `--jql`.

```bash
jira-agent issue search --assignee me
jira-agent issue search --status "In Progress" --priority High
jira-agent issue search --sprint current
jira-agent issue search --type Bug --label backend --label urgent
jira-agent issue search --updated-since 7d
jira-agent issue search --assignee me --status "In Progress" --sprint current
```

| Flag | Notes |
|------|-------|
| `--assignee` | Use `"me"` for `currentUser()`, or any account ID/name |
| `--status` | Status name (e.g. `"In Progress"`) |
| `--type` | Issue type name (e.g. `Bug`, `Story`) |
| `--priority` | Priority name (e.g. `High`, `Medium`) |
| `--label` | Repeatable; each value becomes a separate `labels = "..."` condition |
| `--sprint` | Use `"current"` for `openSprints()`, or a sprint name |
| `--updated-since` | Duration string (e.g. `7d`, `30d`); maps to `updated >= "-Nd"` |

Conditions join with AND in alphabetical order by JQL field name.

## issue create

```bash
jira-agent issue create --project PROJ --type Task --summary "Fix the bug"
jira-agent issue create --project PROJ --type Story --summary "New feature" \
  --description "Details" --priority High --labels bug,urgent \
  --assignee 5b10ac8d82e05b22cc7d4ef5 --field customfield_10016=5
jira-agent issue create --project PROJ --type Task --summary "Formatted" \
  --description $'h4. Goal\n\nSome text.\n\n* First bullet\n* Second bullet' \
  --description-format wiki
```

| Flag | Notes |
|------|-------|
| `--project` | Required (also reads `JIRA_PROJECT` env) |
| `--type` | Required, case-insensitive |
| `--summary` | Required |
| `--description` | Auto mode accepts plain text or valid ADF JSON; use `--description-format wiki` for Jira wiki markup |
| `--description-format` | `auto` (default), `plain`, `adf`, or `wiki` |
| `--assignee` | Account ID only (not email) |
| `--priority` | Name: High, Medium, Low |
| `--labels` | Comma-separated |
| `--components` | Comma-separated |
| `--parent` | Parent key (subtasks) |
| `--field` | Repeatable `key=value` (JSON-parsed if valid) |
| `--fields-json` | JSON object merged into fields, overrides individual flags |
| `--payload-json` | Full issue create payload merged after field flags. Use for top-level `properties`, `update`, `historyMetadata`, or `transition` |

## issue edit

```bash
jira-agent issue edit KEY-123 --summary "Updated title"
jira-agent issue edit KEY-123 --field customfield_10001='{"complex":"value"}'
jira-agent issue edit KEY-123 --fields-json '{"summary":"New","priority":{"name":"High"}}'
jira-agent issue edit KEY-123 --payload-json '{"update":{"labels":[{"add":"triaged"}]}}'
```

Same optional field flags as create, except `--project` and `--type`, plus `--notify` (default true). At least one field change or `--payload-json` update is required. Use `--description-format wiki` when editing descriptions from Jira wiki markup, and use `--payload-json` for top-level edit payloads like `update`, `properties`, `historyMetadata`, or `transition`.

## issue delete

```bash
jira-agent issue delete KEY-123
jira-agent issue delete KEY-123 --delete-subtasks
```

## issue meta

```bash
jira-agent issue meta --project PROJ
jira-agent issue meta --project PROJ --type Bug
jira-agent issue meta --operation edit --issue KEY-123
```

Discover required/available fields before creating or editing.

| Flag | Notes |
|------|-------|
| `--project` | Project key |
| `--type` | Filter to issue type |
| `--operation` | `create` (default) or `edit` |
| `--issue` | Required for `--operation edit` |

## issue count

```bash
jira-agent issue count --jql "project = PROJ AND status = 'To Do'"
```

Returns `data.total` without fetching issue rows. The command sends the JQL to Jira search with `maxResults: 0`, then copies Jira's `total` into both `data.total` and response metadata. `--jql` is required.

## Bulk Operations

Bulk mutating ops require write access. Read-only helpers like `bulk fetch`, `bulk transitions`, and `bulk status` do not. Issue limits noted per command.

### issue bulk status

```bash
jira-agent issue bulk status 10641
```

Use the `taskId` returned by async bulk delete, edit, move, or transition operations to poll `/bulk/queue/{taskId}` progress. Jira keeps bulk progress records for a limited time.

### issue bulk create

```bash
jira-agent issue bulk create --issues-json '[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"First"}}]'
jira-agent issue bulk create --issues-json '{"issueUpdates":[{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Bug"},"summary":"Second"}}]}'
```

Up to 50 issues. Accepts raw array or `{"issueUpdates": [...]}` wrapper. Use `issue meta` first for field schemas.

### issue bulk fetch

```bash
jira-agent issue bulk fetch --issues PROJ-1,PROJ-2 --fields key,summary,status
jira-agent issue bulk fetch --issues PROJ-1,10002 --expand changelog --fields-by-keys
```

Up to 100 issues by key or ID. Compare returned `issues` and `issueErrors` arrays for completeness.

| Flag | Notes |
|------|-------|
| `--issues` | Required, comma-separated keys or IDs |
| `--fields` | Comma-separated field names or IDs |
| `--expand` | e.g., `changelog` |
| `--properties` | Comma-separated, max 5 |
| `--fields-by-keys` | Treat `--fields` as field keys |

### issue bulk delete

```bash
jira-agent issue bulk delete --issues PROJ-1,PROJ-2,PROJ-3
jira-agent issue bulk delete --issues PROJ-1,PROJ-2 --send-notification=false
```

Up to 1000 issues. `--send-notification` defaults true.

### issue bulk edit

```bash
jira-agent issue bulk edit --payload-json '{
  "selectedIssueIdsOrKeys": ["PROJ-1","PROJ-2"],
  "selectedActions": ["priority"],
  "editedFieldsInput": {"priority": {"name": "High"}}
}'
```

Up to 1000 issues, 200 fields. Use `issue bulk edit-fields` to discover editable fields first.

### issue bulk edit-fields

```bash
jira-agent issue bulk edit-fields --issues PROJ-1,PROJ-2
jira-agent issue bulk edit-fields --issues PROJ-1 --search-text "priority"
```

Lists fields available for bulk editing. Cursor pagination via `--starting-after`/`--ending-before`.

### issue bulk move

```bash
jira-agent issue bulk move --payload-json '{
  "targetToSourcesMapping": {"DEST": {"issueIdsOrKeys": ["PROJ-1"], "targetIssueType": "Task"}},
  "sendBulkNotification": false
}'
```

Up to 1000 issues. Payload maps target projects to source issues.

### issue bulk transition

```bash
jira-agent issue bulk transition --transitions-json '[
  {"selectedIssueIdsOrKeys":["PROJ-1","PROJ-2"],"transitionId":"31"}
]' --send-notification=false
```

Up to 1000 issues. Requires transition IDs (not names). Use `issue bulk transitions` to discover them.

### issue bulk transitions

```bash
jira-agent issue bulk transitions --issues PROJ-1,PROJ-2
```

Lists available transitions for bulk transition. Cursor pagination via `--starting-after`/`--ending-before`.

## Workflows

### Search, inspect, update

```bash
jira-agent issue search --jql "project = PROJ AND status = 'To Do' AND assignee = currentUser()"
jira-agent issue get PROJ-42 --expand changelog
jira-agent issue edit PROJ-42 --priority Critical
jira-agent issue transition PROJ-42 --to "In Progress"
```

### Create with custom fields

```bash
jira-agent issue meta --project PROJ --type Story
jira-agent field search -q "story points"
jira-agent issue create --project PROJ --type Story \
  --summary "Implement caching" --priority High \
  --field customfield_10016=5 --labels backend,performance
```

### Paginate search results

```bash
RESULT=$(jira-agent issue search --jql "project = PROJ" --max-results 50)
# Extract next_token from metadata.pagination.next_token, pass to subsequent calls via --next-page-token
jira-agent issue search --jql "project = PROJ" --max-results 50 \
  --next-page-token "TOKEN_FROM_PREVIOUS_RESPONSE"
```

### Bulk sprint status check

```bash
jira-agent issue search \
  --jql "project = PROJ AND sprint in openSprints()" \
  --fields key,summary,status,assignee --output csv
```

## Smart Shortcuts

> **Tip**: Run `jira-agent schema` to discover all commands, flags, and constraints as machine-readable JSON (no auth required).

### issue mine

Returns issues assigned to the current user, ordered by last updated. No `--jql` needed.

```bash
jira-agent issue mine
jira-agent issue mine --status "In Progress"
jira-agent issue mine --fields key,summary,status,priority --max-results 20
jira-agent issue mine --fields-preset triage
```

| Flag | Default | Notes |
|------|---------|-------|
| `--status` | | Filter by status name |
| `--fields` | key,summary,status,assignee,priority | Comma-separated |
| `--fields-preset` | | `minimal`, `triage`, or `detail`; mutually exclusive with `--fields` |
| `--max-results` | 50 | Page size |
| `--start-at` | 0 | Pagination offset |

### issue recent

Returns issues recently touched by the current user (assigned, reported, or watching). Default window is 7 days.

```bash
jira-agent issue recent
jira-agent issue recent --since 14d
jira-agent issue recent --since 30d --max-results 20
jira-agent issue recent --fields-preset minimal
```

| Flag | Default | Notes |
|------|---------|-------|
| `--since` | `7d` | Time window (e.g. `1d`, `7d`, `30d`) |
| `--fields` | key,summary,status,assignee,priority | Comma-separated |
| `--fields-preset` | | `minimal`, `triage`, or `detail`; mutually exclusive with `--fields` |
| `--max-results` | 10 | Page size |
| `--start-at` | 0 | Pagination offset |

## Composite Workflow Commands

Composite commands collapse multiple Jira API calls into one invocation. All require `JIRA_ALLOW_WRITES=true`. Use `--dry-run` to preview without mutating.

### issue start-work

Transitions to In Progress, assigns to current user, and optionally adds a comment. Transition failure is fatal; assign and comment failures are partial (exit 0 with partial data and `next_command` remediation).

```bash
jira-agent issue start-work PROJ-123
jira-agent issue start-work PROJ-123 --status "In Review" --comment "Starting review"
jira-agent issue start-work PROJ-123 --assignee abc123 --skip-transition
jira-agent issue start-work PROJ-123 --skip-assign --comment "Picked up"
jira-agent issue start-work PROJ-123 --dry-run
```

| Flag | Default | Notes |
|------|---------|-------|
| `--status` | `In Progress` | Target transition status |
| `--assignee` | | Account ID (default: current user) |
| `--comment` | | Comment to add after transition |
| `--skip-assign` | false | Skip assignment step |
| `--skip-transition` | false | Skip transition step |
| `--dry-run` | false | Preview diff without mutating |

### issue close

Transitions to Done, sets resolution, and optionally adds a comment. Transition failure is fatal; comment failure is partial.

```bash
jira-agent issue close PROJ-123
jira-agent issue close PROJ-123 --resolution "Won't Do"
jira-agent issue close PROJ-123 --status "Closed" --resolution "Duplicate" --comment "Duplicate of PROJ-100"
jira-agent issue close PROJ-123 --dry-run
```

| Flag | Default | Notes |
|------|---------|-------|
| `--status` | `Done` | Target transition status |
| `--resolution` | `Done` | Resolution value (e.g. `Done`, `Won't Do`, `Duplicate`) |
| `--comment` | | Comment to add after transition |
| `--dry-run` | false | Preview diff without mutating |

### issue create-and-link

Creates an issue and links it to an existing issue in one call. Create failure is fatal; link failure is partial with a `next_command` remediation.

```bash
jira-agent issue create-and-link --project PROJ --type Story --summary "New feature" --link-type Blocks --link-target PROJ-100
jira-agent issue create-and-link --project PROJ --type Bug --summary "Fix" --link-type "is blocked by" --link-target PROJ-200
jira-agent issue create-and-link --payload-json '{"fields":{"project":{"key":"PROJ"},"issuetype":{"name":"Task"},"summary":"Task"}}' --link-type Blocks --link-target PROJ-100
jira-agent issue create-and-link --project PROJ --type Story --summary "New" --link-type Blocks --link-target PROJ-100 --dry-run
```

| Flag | Notes |
|------|-------|
| `--link-type` | Required. Link type name (e.g. `Blocks`, `"is blocked by"`) |
| `--link-target` | Required. Issue key to link to |
| `--link-direction` | `outward` (default) or `inward` |
| `--payload-json` | Full create payload; mutually exclusive with individual field flags |
| `--project`, `--type`, `--summary` | Required when not using `--payload-json` |
| `--dry-run` | Preview without mutating |

### issue transition-jql

Searches issues with POST `/rest/api/3/search/jql`, resolves bulk transition IDs by target status, and submits a fire-and-forget bulk transition task. Rejects JQL matching more than 1000 issues.

```bash
jira-agent issue transition-jql --jql 'project = PROJ AND status = "In Progress"' --status Done
jira-agent issue transition-jql --jql 'assignee = currentUser() AND status = Open' --status "In Progress" --send-notification=false
jira-agent issue transition-jql --jql 'project = PROJ AND status = Open' --status Done --dry-run
```

| Flag | Default | Notes |
|------|---------|-------|
| `--jql` | | Required. JQL selecting up to 1000 issues |
| `--status` | | Required. Target status name, matched case-insensitively against transition `to.name` |
| `--send-notification` | true | Send Jira bulk notification email |
| `--dry-run` | false | Search and resolve transitions, then output matched, skipped, and target counts without submitting the bulk task |

### issue move-to-sprint

Moves an issue to a sprint using the Agile API, with optional transition and comment. Sprint move failure is fatal; transition and comment failures are partial.

```bash
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --status "In Progress"
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --comment "Moved to sprint" --rank-before PROJ-100
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --dry-run
```

| Flag | Notes |
|------|-------|
| `--sprint-id` | Required. Sprint ID (integer) |
| `--status` | Transition issue to this status after moving |
| `--comment` | Add a comment after the operation |
| `--rank-before` | Rank issue before this issue key in the sprint |
| `--rank-after` | Rank issue after this issue key in the sprint |
| `--dry-run` | Preview without mutating |
