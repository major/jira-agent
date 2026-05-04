# internal/commands agent guide

Generated: Mon May 04 2026. Applies to the Jira command tree.

## Command architecture

This package defines the LLM-facing Jira command surface. Treat command metadata, examples, required flags, output shape, pagination, and write-protection as public contracts because agents discover and execute commands through `jira-agent --help` and `jira-agent <command> --help`.

Keep this file and the root `AGENTS.md` current anytime command code changes. Also update `skills/jira-agent` files in the same change whenever command code changes affect command paths, aliases, flags, args, examples, write behavior, pagination, output, errors, or recommended LLM workflows.

Common constructor shape:

```go
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format) *cobra.Command
func XCommand(apiClient *client.Ref, w io.Writer, format *output.Format, allowWrites *bool) *cobra.Command
```

Do not allocate standalone clients inside commands. Capture the shared `*client.Ref`, output writer, output format pointer, and write-enable pointer provided by root wiring.

## Command struct rules

Every `&cobra.Command{}` needs:

- `Use`
- `Short`
- `Example`
- `setDefaultSubcommand(cmd, "name")` on parents when the default is obvious
- positional argument placeholders in `Use` on leaves with positional args

Naming and text:

- Names are lowercase kebab-case.
- `Usage` is short imperative text, 3 to 8 words, no period.
- `Example` uses full `jira-agent <path>` invocations, newline-separated when there are multiple examples.
- Positional args in `Use` use `<name>` for required args, `[name]` for optional args, and `[name...]` for repeatable args.
- Field order: `Use`, `Aliases`, `Short`, `Example`, `RunE`.
- Add flags after command construction with `cmd.Flags()`, then add children with `cmd.AddCommand(...)`.

## Write protection

- All mutations must be wrapped with `writeGuard(allowWrites, action)` or explicitly call `requireWriteAccess(allowWrites)` before side effects.
- Do not add hidden write paths in read-looking commands.
- Blocked writes must remain validation failures with exit 5 and remediation for config/env write enablement.
- Mutating commands are identifiable by the `writeGuard` wrapper in their `RunE`.

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
- Use `writePaginatedAPIResult(w, format, call)` for list/search calls with pagination metadata.
- Default JSON removes noisy Jira API metadata such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. `issue get` and `issue search` convert ADF descriptions to plain text by default via `--description-output-format text|markdown|adf`, while `--raw` preserves Jira's original ADF. `issue search` additionally reshapes Jira's response into flattened `.data.issues[]` rows, while `--raw` uses the unmodified paginated API response.
- Do not write JSON, CSV, TSV, or errors directly from commands.
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
- `user_group_filter.go`: user/group/filter searches and pagination.
- `permission_dashboard.go`, `board.go`, `sprint.go`, `version.go`, `link.go`, and `property.go`: moderate complexity with write paths and Jira-specific API shapes.

## Gotchas agents depend on

- Issue descriptions default to auto mode on writes: plain text auto-converts to ADF and structured ADF JSON passes through. Use `--description-format wiki` when the input uses Jira wiki markup such as `h4.` headings or `*` bullet lists. On reads, `issue get` and `issue search` default to `--description-output-format text`; use `markdown`, `adf`, or `--raw` when callers need richer formatting or Jira's unmodified payload.
- Assignments use account IDs, not email addresses.
- Transitions ultimately require transition IDs, even when helpers resolve case-insensitive names.
- Bulk create is limited by Jira API constraints; keep examples small.
- Parent commands use `setDefaultSubcommand(cmd, "list")` when the default is obvious.
- Hidden legacy aliases may exist for compatibility, but examples should use canonical paths.
