# Issues

Issue CRUD, search, bulk operations, metadata, and count.

## issue get

```bash
jira-agent issue get KEY-123
jira-agent issue get KEY-123 --fields summary,status,assignee --expand changelog
jira-agent issue get KEY-123 --properties request --fields-by-keys --update-history
```

Useful flags: `--properties`, `--fields-by-keys`, `--update-history`, and `--fail-fast=false` for partial field-resolution failures.

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
jira-agent issue search --jql "..." --next-page-token TOKEN
jira-agent issue search --jql "project = PROJ" --properties request --fields-by-keys --fail-fast=false
```

| Flag | Default | Notes |
|------|---------|-------|
| `--jql` | (required) | JQL query string |
| `--fields` | summary,status,assignee,priority | Comma-separated |
| `--max-results` | 50 | Page size |
| `--next-page-token` | | Cursor from previous `.metadata` |
| `--expand` | | e.g., `changelog,renderedFields` |
| `--order-by` | | Sort field |
| `--order` | | `asc` or `desc` |
| `--properties` | | Comma-separated issue properties |
| `--fields-by-keys` | false | Treat field identifiers as field keys |
| `--fail-fast` | true | Set false to tolerate unresolved fields |
| `--reconcile-issues` | | Comma-separated issue IDs for Jira reconciliation |

## issue create

```bash
jira-agent issue create --project PROJ --type Task --summary "Fix the bug"
jira-agent issue create --project PROJ --type Story --summary "New feature" \
  --description "Details" --priority High --labels bug,urgent \
  --assignee 5b10ac8d82e05b22cc7d4ef5 --field customfield_10016=5
```

| Flag | Notes |
|------|-------|
| `--project` | Required (also reads `JIRA_PROJECT` env) |
| `--type` | Required, case-insensitive |
| `--summary` | Required |
| `--description` | Plain text auto-converts to ADF; valid ADF JSON passes through |
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

Same optional field flags as create, except `--project` and `--type`, plus `--notify` (default true). At least one field change or `--payload-json` update is required. Use `--payload-json` for top-level edit payloads like `update`, `properties`, `historyMetadata`, or `transition`.

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

Returns issue count without fetching full results. `--jql` is required.

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
# Extract next-page-token from .metadata, pass to subsequent calls
jira-agent issue search --jql "project = PROJ" --max-results 50 \
  --next-page-token "TOKEN_FROM_PREVIOUS_RESPONSE"
```

### Bulk sprint status check

```bash
jira-agent issue search \
  --jql "project = PROJ AND sprint in openSprints()" \
  --fields key,summary,status,assignee --output csv
```
