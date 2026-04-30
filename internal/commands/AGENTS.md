# internal/commands agent guide

Generated: Tue Apr 28 2026. Applies to the Jira command tree and schema introspection code.

## Command architecture

This package defines the LLM-facing Jira command surface. Treat command metadata, examples, required flags, output shape, pagination, and write-protection as public contracts because agents discover and execute commands through `jira-agent schema`.

Keep this file and the root `AGENTS.md` current anytime command code changes. Also update `skills/jira-agent` files in the same change whenever command code changes affect command paths, aliases, flags, args, examples, schema metadata, write behavior, pagination, output, errors, or recommended LLM workflows.

Common constructor shape:

```go
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cli.Command
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cli.Command
```

Do not allocate standalone clients inside commands. Capture the shared `*client.Ref`, output writer, output format pointer, and write-enable pointer provided by root wiring.

## Command struct rules

Every `&cli.Command{}` needs:

- `Name`
- `Usage`
- `UsageText`
- `DefaultCommand` on parents when the default is obvious
- `ArgsUsage` on leaves with positional args

Naming and text:

- Names are lowercase kebab-case.
- `Usage` is short imperative text, 3 to 8 words, no period.
- `UsageText` uses full `jira-agent <path>` invocations, newline-separated when there are multiple examples.
- `ArgsUsage` uses `<name>` for required args, `[name]` for optional args, and `[name...]` for repeatable args.
- Field order: `Name`, `Aliases`, `Usage`, `UsageText`, `DefaultCommand`, `ArgsUsage`, `Flags`, `Commands`, `Action`.

## Schema metadata

- The `schema` command is source of truth for LLM discovery.
- Use `writeCommandMetadata()` for mutating commands.
- Use `requiredFlagMetadata("flag")` for required flags.
- Use `commandMetadata(...)` to merge schema metadata.
- Metadata keys include `schema_mutating`, `schema_requires_write_access`, and `schema_required_flags`.
- `SchemaCommand` emits raw JSON and must keep `SetEscapeHTML(false)`.
- `--compact` should stay token-efficient: command paths, usage, aliases, required flags, mutation bits, and enough child metadata to decide the next schema call.

## Write protection

- All mutations must be wrapped with `writeGuard(allowWrites, action)` or explicitly call `requireWriteAccess(allowWrites)` before side effects.
- Do not add hidden write paths in read-looking commands.
- Blocked writes must remain validation failures with exit 5 and remediation for config/env write enablement.
- Mark mutating commands in schema metadata so LLMs can avoid accidental writes.

## Validation helpers

Use the shared helpers so errors remain consistent:

- `requireArg(cmd, label)` for one positional arg.
- `requireArgs(cmd, labels...)` for multiple positional args.
- `requireFlag(cmd, flagName)` for required string flags.
- `requireFlagWithDetails(cmd, flagName, details)` when remediation needs extra context.
- `requireVisibilityFlags(cmd)` when `--visibility-type` and `--visibility-value` must be set together.

Return typed validation errors rather than generic `fmt.Errorf` for user-correctable input failures.

## Output helpers

- Use `writeAPIResult(w, format, call)` for single-result API calls.
- Use `writePaginatedAPIResult(w, format, call)` for list/search calls with pagination metadata.
- Default JSON removes noisy Jira API metadata such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. `issue get` and `issue search` convert ADF descriptions to plain text by default via `--description-output-format text|markdown|adf`, while `--raw` preserves Jira's original ADF. `issue search` additionally reshapes Jira's response into flattened `.data.issues[]` rows, while `--raw` uses the unmodified paginated API response.
- Do not write JSON, CSV, TSV, or errors directly from commands unless the command is `schema`.
- Preserve pagination metadata extraction for arrays named `issues`, `values`, `comments`, and `worklogs`.

## Pagination and query params

- Standard offset pagination flags are `--max-results` default 50 and `--start-at` default 0.
- `buildPaginationParams` builds API params from pagination plus optional filters.
- Some Jira endpoints use cursor tokens such as `nextPageToken`; document and test those exceptions.
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
- `schema.go`: recursive command-tree walking, filtering, metadata extraction, compact output.
- `user_group_filter.go`: user/group/filter searches and pagination.
- `permission_dashboard.go`, `board.go`, `sprint.go`, `version.go`, `link.go`, and `property.go`: moderate complexity with write paths and Jira-specific API shapes.

## Gotchas agents depend on

- Issue descriptions default to auto mode on writes: plain text auto-converts to ADF and structured ADF JSON passes through. Use `--description-format wiki` when the input uses Jira wiki markup such as `h4.` headings or `*` bullet lists. On reads, `issue get` and `issue search` default to `--description-output-format text`; use `markdown`, `adf`, or `--raw` when callers need richer formatting or Jira's unmodified payload.
- Assignments use account IDs, not email addresses.
- Transitions ultimately require transition IDs, even when helpers resolve case-insensitive names.
- Bulk create is limited by Jira API constraints; keep examples small and schema-backed.
- Parent commands often have `DefaultCommand: "list"`; keep defaults obvious and documented by schema.
- Hidden legacy aliases may exist for compatibility, but examples should use canonical paths.
