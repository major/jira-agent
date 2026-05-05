# LLM UX Improvement Areas

This project should not aim for exact 1:1 CLI-to-API parity. The higher-value goal is friendly porcelain that helps LLMs complete Jira workflows in fewer calls, fewer tokens, and with less confusion.

## 1. Add resolver porcelain for Jira IDs

LLMs still need extra calls to resolve account IDs, board IDs, sprint IDs, field IDs, context IDs, and transition IDs. Add friendly resolution flows such as `user resolve`, `board resolve`, `field resolve`, or allow commands to accept names and emails with a `--resolve` flag.

This would cut common two- and three-call workflows down to one call.

## 2. Make schema and metadata discovery automatic

Issue creation and edits often require `issue meta` first, especially for required custom fields. Add `issue create --auto-discover`, better create/edit validation hints, or error responses that suggest the exact discovery command.

When Jira rejects a mutation, the CLI should help the LLM recover by explaining the next command to run.

## 3. Standardize pagination metadata

Offset pagination is exposed in `.metadata`, but `issue search` cursor pagination uses `.data.nextPageToken` and `.data.isLast`. Move all pagination state into a consistent metadata shape, such as `metadata.pagination.type`, `metadata.pagination.next_token`, and `metadata.pagination.has_more`.

LLMs should not need command-specific pagination logic.

## 4. Add workflow-level porcelain commands

Common goals currently require chained atomic calls, for example creating an issue, assigning it, adding a comment, transitioning it, and watching it. Add composite commands such as `issue create-and-assign`, `issue transition-and-comment`, `bulk transition --wait`, or `sprint summarize`.

This is where the CLI can beat API parity by packaging Jira workflows into LLM-friendly actions.

## 5. Improve embedded skill docs with recipes and recovery playbooks

The skill docs cover commands well, but they need more end-to-end recipes: bulk transition with validation, issue creation with custom fields, partial bulk failure handling, large search pagination, and recovery from auth, write, and API errors.

This would improve LLM success immediately without requiring code changes and would give agents safer defaults.

## Suggested sequence

Start with resolver porcelain and pagination metadata because they reduce confusion across many commands. Then add workflow commands around the most common Jira tasks.
