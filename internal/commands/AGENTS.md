# internal/commands agent guide

Generated: Mon May 04 2026. Applies to the Jira command tree.

## Command architecture

This package defines the LLM-facing Jira command surface. Treat command metadata, examples, required flags, output shape, pagination, and write-protection as public contracts because agents discover and execute commands through `jira-agent --help` and `jira-agent <command> --help`.

Keep this file and the root `AGENTS.md` current anytime command code changes. Also update `skills/jira-agent` files in the same change whenever command code changes affect command paths, aliases, flags, args, examples, write behavior, pagination, output, errors, or recommended LLM workflows.

Common constructor shapes:

```go
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites, dryRun *bool) *cobra.Command
```

Composite commands that support `--dry-run` use the third form. Inside the handler, check `IsDryRun(dryRun)` first; call `requireWriteAccess(allowWrites)` only when not in dry-run mode. Do not use `writeGuard` for composites because dry-run must bypass the write check.

Do not allocate standalone clients inside commands. Capture the shared `*client.Ref`, output writer, output format pointer, and write-enable pointer provided by root wiring.

## Command struct rules

Every `&cobra.Command{}` needs:

- `Use`
- `Short`
- `Example`
- `setDefaultSubcommand(cmd, "name")` on parents when the default is obvious
- positional argument placeholders in `Use` on leaves with positional args
- declarative Cobra annotations for behavior that is otherwise hidden in handlers. `setDefaultSubcommand` records `jira-agent/default-subcommand`; use shared annotation helpers rather than ad hoc keys.

### Annotation categories

Commands are annotated with a category via `SetCommandCategory(cmd, category)`. Known categories: `read`, `write`, `bulk`, `discovery` (schema), `workflow` (composites), `admin`. The `jira-agent/requires-auth` annotation controls whether `PersistentPreRunE` loads auth; set `commandRequiresAuthFalse` on commands like `schema` that work without credentials.

### Flag groups

Use `markMutuallyExclusive(cmd, flagA, flagB)` to enforce at-most-one at the Cobra parse layer and record the constraint in the `jira-agent/flag-groups` annotation for schema introspection. `FlagGroups(cmd)` reads back the annotation. For pairwise exclusion of one flag against many (e.g., `--payload-json` vs individual field flags), call `markMutuallyExclusive` once per pair.

Naming and text:

- Names are lowercase kebab-case.
- `Usage` is short imperative text, 3 to 8 words, no period.
- `Example` uses full `jira-agent <path>` invocations, newline-separated when there are multiple examples.
- Positional args in `Use` use `<name>` for required args, `[name]` for optional args, and `[name...]` for repeatable args.
- Field order: `Use`, `Aliases`, `Short`, `Example`, `RunE`.
- Add flags after command construction with `cmd.Flags()`, then add children with `cmd.AddCommand(...)`.

## Write protection

- All mutations must be wrapped with `writeGuard(allowWrites, action)` or explicitly call `requireWriteAccess(allowWrites)` before side effects.
- Mutating commands must carry `jira-agent/write-protected=true`; root wiring records this through the authoritative annotator `MarkWriteProtectedCommands`, which uses the explicit `WriteProtectedCommandPaths` configuration list.
- Do not add hidden write paths in read-looking commands.
- Blocked writes must remain validation failures with exit 5 and remediation for config/env write enablement.
- Mutating commands are identifiable by both their write-protection annotation and the `writeGuard` wrapper or explicit `requireWriteAccess` in their handlers.
- `TestWriteProtectedCommandsAnnotated` is the CI contract check that keeps `jira-agent/write-protected` annotations aligned with `MarkWriteProtectedCommands`; update the path list and tests together when adding write commands.

## Validation helpers

Use the shared helpers so errors remain consistent:

- `requireArg(args, label)` for one positional arg.
- `requireArgs(args, labels...)` for multiple positional args.
- `requireFlag(cmd, flagName)` for required string flags.
- `requireFlagWithDetails(cmd, flagName, details)` when remediation needs extra context.
- `requireVisibilityFlags(cmd)` when `--visibility-type` and `--visibility-value` must be set together.

Return typed validation errors rather than generic `fmt.Errorf` for user-correctable input failures.

## Output helpers

- Use `writeAPIResult(w, format, call)` for single-result API calls.
- Use `writePaginatedAPIResult(w, format, call)` for list/search calls with pagination metadata. This function nests pagination fields under `metadata.pagination` with `type` ("offset" or "cursor").
- Default JSON removes noisy Jira API metadata such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. `issue get` and `issue search` convert ADF descriptions to plain text by default via `--description-output-format text|markdown|adf`, while `--raw` preserves Jira's original ADF. `issue search` additionally reshapes Jira's response into flattened `.data.issues[]` rows, while `--raw` uses the unmodified paginated API response. Also extracts `issueChangeLogs` array. Pagination fields are nested under `metadata.pagination`.
- Do not write JSON, CSV, TSV, or errors directly from commands.
- Preserve pagination metadata extraction for arrays named `issues`, `values`, `comments`, `worklogs`, `records`, and `issueChangeLogs`.

## Pagination and query params

- Standard offset pagination flags are `--max-results` default 50 and `--start-at` default 0. These values appear in `metadata.pagination.max_results` and `metadata.pagination.start_at`.
- `buildPaginationParams` builds API params from pagination plus optional filters. Output metadata is nested under `metadata.pagination`.
- Some Jira endpoints use cursor tokens such as `nextPageToken`; `issue search` and `changelog bulk-fetch` use cursor pagination. Cursor responses expose `metadata.pagination.next_token` and `metadata.pagination.has_more` (derived from `isLast`). Use `--next-page-token` flag to pass the token to the next call.
- `appendQueryParams` sorts query keys for deterministic URLs and tests.
- Use `escapePathSegment` for path segments, not raw string interpolation.

## Issue field mutation rules

- Issue create/edit can set fields through individual flags, repeated `--field key=value`, and `--fields-json`.
- Merge order is individual flags, then repeated `--field`, then `--fields-json`. Last writer wins.
- `--field` parses JSON values when valid, otherwise keeps raw strings.
- Prefer `--fields-json` in LLM examples for complex payloads and custom fields.
- Use `issue meta` examples before create/edit examples when required fields are project/type-dependent.

## Command complexity hotspots

- `issue.go`: largest surface, nested issue interactions, field merging, bulk operations, comments, worklogs, links, attachments, watchers, votes, transitions, ranking, notify, metadata.
- `composite.go`: composite workflow commands (`start-work`, `close`, `create-and-assign`, `create-and-link`, `move-to-sprint`). Each uses a params struct, a prepare function (read-only API calls), a dry-run function (diff computation), and an execute function (mutations with partial failure). Keep each function under the gocognit limit of 30.
- `shortcuts.go`: `issue mine` and `issue recent` shortcut commands. Read-only; no `allowWrites`/`dryRun` params.
- `user_group_filter.go`: user/group/filter searches and pagination.
- `permission_dashboard.go`, `board.go`, `sprint.go`, `version.go`, `link.go`, and `property.go`: moderate complexity with write paths and Jira-specific API shapes.

## Resolve command tree

`resolve` is a read-only discovery command group. Category: `discovery`. No write protection needed.

### Subcommands and signatures

| Subcommand | Positional arg | Required flags | Optional flags | API endpoint |
| --- | --- | --- | --- | --- |
| `resolve user <query>` | email or display name | none | none | `GET /rest/api/3/users/search` |
| `resolve board <query>` | board name | none | none | `GET /rest/agile/1.0/board` |
| `resolve sprint <query>` | sprint name | `--board-id <id>` | `--state <states>` | `GET /rest/agile/1.0/board/{id}/sprint` |
| `resolve field <query>` | field name | none | none | `GET /rest/api/3/field/search` |
| `resolve transition <query>` | transition name or target status | `--issue <key>` | none | `GET /rest/api/3/issue/{key}/transitions` |

### Output shapes

All resolvers return `output.WriteSuccess` with a typed array in `data` and `metadata.usage_hint`.

- `resolve user`: `[]{account_id, display_name, email_address, active}`
- `resolve board`: `[]{id, name, type}`
- `resolve sprint`: `[]{id, name, state}`
- `resolve field`: `[]{id, name, custom}`
- `resolve transition`: `[]{id, name}`

### Metadata

`resolverMetadata` sets `total`, `returned`, and `usage_hint` on every resolve response. `usage_hint` is a ready-to-run follow-up command string:

| Resolver | `usage_hint` |
| --- | --- |
| user | `jira-agent issue assign <issue-key> --assignee <account_id>` |
| board | `jira-agent resolve sprint --board-id <id> <sprint-name>` |
| sprint | `jira-agent sprint get <id>` |
| field | `jira-agent issue get <issue-key> --fields <id>` |
| transition | `jira-agent issue transition <key> --transition-id <id>` |

### Context flags

- `resolve sprint` requires `--board-id <id>`. Missing board ID returns a `ValidationError` with `next_command: "jira-agent resolve board <board-name>"`.
- `resolve sprint --state` accepts comma-separated values: `future`, `active`, `closed`. Default is `active,future`. Sprint matching is client-side case-insensitive substring, capped at 10 results.
- `resolve transition` requires `--issue <key>`. Missing issue key returns a `ValidationError` with `next_command: "jira-agent issue get <issue-key>"`. Transition matching checks both the transition name and the target status name (`to.name`), case-insensitive substring. Not-found errors include `available_actions` listing all transition names for the issue.

### LLM nudge mechanisms

1. Schema category: all resolve subcommands inherit `category: "discovery"` from the parent `ResolveCommand`.
2. `usage_hint` in `metadata`: each resolver embeds the follow-up command so agents know what to do with the resolved ID without guessing.
3. Validation error `next_command`: `parseBoardID` in `board.go` and `parseSprintID` in `sprint.go` include `next_command` pointing to the appropriate resolve subcommand when an ID is invalid or missing.

### Shared helpers in resolve.go

- `resolverMetadata(total, returned int, usageHint string) output.Metadata`: builds metadata with `total`, `returned`, and `usage_hint`.
- `requireQuery(args []string, entityName string) (string, error)`: validates that a non-empty positional query arg is present; returns `ValidationError` otherwise.

## Gotchas agents depend on

- Issue descriptions default to auto mode on writes: plain text auto-converts to ADF and structured ADF JSON passes through. Use `--description-format wiki` when the input uses Jira wiki markup such as `h4.` headings or `*` bullet lists. On reads, `issue get` and `issue search` default to `--description-output-format text`; use `markdown`, `adf`, or `--raw` when callers need richer formatting or Jira's unmodified payload.
- Assignments use account IDs, not email addresses.
- Transitions ultimately require transition IDs, even when helpers resolve case-insensitive names.
- Bulk create is limited by Jira API constraints; keep examples small.
- Parent commands use `setDefaultSubcommand(cmd, "list")` when the default is obvious.
- Hidden legacy aliases may exist for compatibility, but examples should use canonical paths.
