---
applyTo: "internal/commands/**/*.go"
---

# Command review instructions

- Command constructors should accept `(apiClient *client.Ref, w io.Writer, format *output.Format)`.
- JQL search must use POST `/rest/api/3/search/jql`.
- Transition helpers need transition IDs, not status names.
- `toADF` should convert plain text to Atlassian Document Format and pass valid ADF JSON through.
- `applyFieldOverrides` should parse values as JSON when valid and otherwise use the raw string.
- Pagination metadata should include total, returned, start, max results, `has_more`, and `next_command` when applicable.
- Mutating commands must enforce write access through `writeGuard` or `requireWriteAccess`.
