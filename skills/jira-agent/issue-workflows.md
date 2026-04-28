# Issue Workflows

Transition, assign, comment, worklog, watcher, vote, attachment, link, remote-link, changelog, rank, notify.

## issue transition

```bash
jira-agent issue transition KEY-123 --to "In Progress"
jira-agent issue transition KEY-123 --to Done --comment "Completed"
jira-agent issue transition KEY-123 --to Done --field customfield_10001=value
jira-agent issue transition KEY-123 --transition-id 31
jira-agent issue transition KEY-123 --transition-id 31 \
  --payload-json '{"properties":[{"key":"source","value":"cli"}]}'
jira-agent issue transition KEY-123 --list
jira-agent issue transition KEY-123 --list --expand transitions.fields
```

| Flag | Notes |
|------|-------|
| `--to` | Target status name (case-insensitive). Required unless `--transition-id` or `--list` |
| `--transition-id` | Direct transition ID when already known |
| `--comment` | Transition comment |
| `--field` | Repeatable `key=value` for transition fields |
| `--payload-json` | Full transition payload merged after flag-derived body |
| `--list` | List available transitions (read-only, bypasses write guard) |
| `--expand` | Transition list expansion, e.g., `transitions.fields` |
| `--list-transition-id` | Filter transition list by ID |
| `--include-unavailable-transitions` | Include transitions blocked by conditions |
| `--skip-remote-only-condition` | Skip remote-only transition condition checks |
| `--sort-by-ops-bar-and-status` | Sort transition list like Jira operations bar |

Matches against both transition names and target status names when `--to` is used.

## issue assign

```bash
jira-agent issue assign KEY-123 5b10ac8d82e05b22cc7d4ef5
jira-agent issue assign KEY-123 --unassign
jira-agent issue assign KEY-123 --default
```

Second positional arg is `accountId` (not email). `--unassign` clears, `--default` uses project default.

## Comments

### issue comment list

```bash
jira-agent issue comment list KEY-123
jira-agent issue comment list KEY-123 --order-by -created --max-results 10
```

| Flag | Default | Notes |
|------|---------|-------|
| `--order-by` | | `created`, `-created`, `+created` |
| `--expand` | | `renderedBody` |
| `--max-results` | 50 | Page size |
| `--start-at` | 0 | Offset |

### issue comment get / list-by-ids

```bash
jira-agent issue comment get KEY-123 10005
jira-agent issue comment get KEY-123 10005 --expand renderedBody
jira-agent issue comment list-by-ids --ids 10005,10006 --expand renderedBody
```

Use `get` for one known comment ID. Use `list-by-ids` to fetch comments across issues by ID; `--ids` is comma-separated and required.

### issue comment add

```bash
jira-agent issue comment add KEY-123 --body "This is a comment"
jira-agent issue comment add KEY-123 --body '{"type":"doc",...}' \
  --visibility-type role --visibility-value Developers
```

| Flag | Notes |
|------|-------|
| `--body` / `-b` | Required. Plain text or ADF JSON |
| `--visibility-type` | `group` or `role` |
| `--visibility-value` | Name (required if type set) |
| `--expand` | `renderedBody` |

### issue comment edit

```bash
jira-agent issue comment edit KEY-123 10005 --body "Updated comment"
jira-agent issue comment edit KEY-123 10005 --body "Updated" --override-editable-flag
```

Same flags as add, plus `--notify` (default true) and `--override-editable-flag`. Args: `<issue-key> <comment-id>`.

### issue comment delete

```bash
jira-agent issue comment delete KEY-123 10005
```

## Worklogs

### issue worklog list

```bash
jira-agent issue worklog list KEY-123
jira-agent issue worklog list KEY-123 --started-after 1700000000000 --max-results 20
```

| Flag | Default | Notes |
|------|---------|-------|
| `--start-at` | 0 | Offset |
| `--max-results` | 20 | Page size |
| `--started-after` | | Unix ms timestamp |
| `--started-before` | | Unix ms timestamp |
| `--expand` | | `properties` |

### issue worklog get / updated / deleted / list-by-ids

```bash
jira-agent issue worklog get KEY-123 10005 --expand properties
jira-agent issue worklog updated --since 1700000000000 --expand properties
jira-agent issue worklog deleted --since 1700000000000
jira-agent issue worklog list-by-ids --ids 10005,10006 --expand properties
```

Use `updated` and `deleted` for Jira worklog sync cursors. `--since` is a Unix millisecond timestamp. Use `list-by-ids` when you already have worklog IDs from sync results.

### issue worklog add

```bash
jira-agent issue worklog add KEY-123 \
  --started 2026-04-27T10:00:00.000-0500 --time-spent 1h --comment "Investigated"
jira-agent issue worklog add KEY-123 \
  --started 2026-04-27T10:00:00.000-0500 --time-spent-seconds 1800 \
  --adjust-estimate leave --notify=false
```

| Flag | Notes |
|------|-------|
| `--started` | Required timestamp, e.g., `2026-04-27T10:00:00.000-0500` |
| `--time-spent` | Required unless `--time-spent-seconds`, e.g., `1h 30m` |
| `--time-spent-seconds` | Required unless `--time-spent` |
| `--comment` | Plain text or ADF JSON |
| `--visibility-type` | `group` or `role` |
| `--visibility-value` | Name (required if type set) |
| `--properties-json` | JSON array of worklog properties |
| `--notify` | Default true |
| `--adjust-estimate` | `auto`, `leave`, `manual`, or `new` |
| `--new-estimate` | For estimate adjustment |
| `--reduce-by` | For manual estimate reduction |
| `--expand` | e.g., `properties` |
| `--override-editable-flag` | Override Jira's editable check |

### issue worklog edit

```bash
jira-agent issue worklog edit KEY-123 10005 --time-spent 2h
jira-agent issue worklog edit KEY-123 10005 --comment "Updated" --override-editable-flag
```

Same flags as add but all optional. At least one field required. Args: `<issue-key> <worklog-id>`.

### issue worklog delete

```bash
jira-agent issue worklog delete KEY-123 10005
jira-agent issue worklog delete KEY-123 10005 --adjust-estimate manual --increase-by 1h
```

| Flag | Notes |
|------|-------|
| `--notify` | Default true |
| `--adjust-estimate` | `auto`, `leave`, `manual`, or `new` |
| `--new-estimate` | New remaining estimate |
| `--increase-by` | For manual adjustment |
| `--override-editable-flag` | Override editable check |

## Watchers

### issue watcher list / add / remove

```bash
jira-agent issue watcher list KEY-123
jira-agent issue watcher add KEY-123 --account-id 5b10ac8d82e05b22cc7d4ef5
jira-agent issue watcher remove KEY-123 --account-id 5b10ac8d82e05b22cc7d4ef5
```

`--account-id` is required for add/remove.

## Votes

### issue vote get / add / remove

```bash
jira-agent issue vote get KEY-123
jira-agent issue vote add KEY-123
jira-agent issue vote remove KEY-123
```

No flags. Operates on the authenticated user's vote.

## Attachments

### issue attachment list / add / get / delete

```bash
jira-agent issue attachment list KEY-123
jira-agent issue attachment add KEY-123 --file /path/to/document.pdf
jira-agent issue attachment add KEY-123 --file doc.pdf --file image.png
jira-agent issue attachment get 10500
jira-agent issue attachment delete 10500
```

`list` and `add` take issue key. `--file` is repeatable. `get` and `delete` take attachment ID.

## Issue Links

### issue link list / add / delete / types

```bash
jira-agent issue link list KEY-123
jira-agent issue link get 10500
jira-agent issue link add --inward KEY-123 --outward KEY-456 --type "Blocks"
jira-agent issue link add --inward KEY-123 --outward KEY-456 --type-id 10000
jira-agent issue link add --inward KEY-123 --outward KEY-456 --type "Blocks" \
  --comment "Related root cause"
jira-agent issue link delete 10500
jira-agent issue link types
```

| Flag | Notes |
|------|-------|
| `--inward` | Required for add. Inward issue key |
| `--outward` | Required for add. Outward issue key |
| `--type` | Link type name (e.g., "Blocks", "Cloners") |
| `--type-id` | Link type ID (alternative to `--type`) |
| `--comment` | Optional link comment, plain text or ADF JSON |

`get` and `delete` take link ID. `types` lists all available link types.

## Remote Links

### issue remote-link list / add / edit / delete

```bash
jira-agent issue remote-link list KEY-123
jira-agent issue remote-link list KEY-123 --global-id "ci-build-42"
jira-agent issue remote-link get KEY-123 10500
jira-agent issue remote-link add KEY-123 --title "Design Doc" --url "https://..."
jira-agent issue remote-link add KEY-123 --title "CI Build" --url "https://..." \
  --global-id "ci-build-42" --relationship "is built by"
jira-agent issue remote-link edit KEY-123 10500 --title "Updated" --url "https://new-url"
jira-agent issue remote-link delete KEY-123 10500
jira-agent issue remote-link delete KEY-123 --global-id "ci-build-42"
```

| Flag | Notes |
|------|-------|
| `--title` | Required for add/edit |
| `--url` | Required for add/edit |
| `--global-id` | Optional unique ID for add/edit, filter for list, or delete selector |
| `--relationship` | Optional relationship description |

## Changelog

```bash
jira-agent issue changelog KEY-123
jira-agent issue changelog KEY-123 --max-results 10 --start-at 0
jira-agent issue changelog list-by-ids KEY-123 --ids 10001,10002
jira-agent issue changelog bulk-fetch --issues KEY-123,KEY-456 \
  --field-ids status,assignee --max-results 100 --next-page-token TOKEN
```

Returns field change history. Direct issue changelog listing uses offset pagination. Use `list-by-ids` when Jira returned specific changelog IDs. Use `bulk-fetch` for cross-issue changelog sync, optionally filtered to up to 10 field IDs and paginated with `--next-page-token`.

## Ranking

```bash
jira-agent issue rank --issues KEY-1,KEY-2 --before KEY-5
jira-agent issue rank --issues KEY-3 --after KEY-10
```

| Flag | Notes |
|------|-------|
| `--issues` | Required, comma-separated keys to rank |
| `--before` | Rank before this issue (mutually exclusive with `--after`) |
| `--after` | Rank after this issue |

## Notifications

```bash
jira-agent issue notify KEY-123 --subject "Urgent update" --text-body "Please review"
jira-agent issue notify KEY-123 --subject "Status change" --html-body "<b>Done</b>" \
  --to-assignee --to-watchers --to-users 5b10ac8d82e05b22cc7d4ef5 --to-users 5b10ac8d82e05b22cc7d4ef6
```

| Flag | Notes |
|------|-------|
| `--subject` | Required |
| `--text-body` | Plain text body |
| `--html-body` | HTML body |
| `--to-assignee` | Notify assignee |
| `--to-reporter` | Notify reporter |
| `--to-users` | Repeatable account IDs (not emails) |
| `--to-voters` | Notify voters |
| `--to-watchers` | Notify watchers |
