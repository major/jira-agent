# jira-agent review instructions

Review this repository as an LLM-first Go CLI for Jira Cloud. The main contract is structured output, schema discovery, bounded responses, deterministic exit codes, write protection by default, and recovery-rich errors.

Focus on correctness, security, data loss, broken command contracts, and repository conventions. Do not nitpick formatting or style that gofmt or golangci-lint already handles.

## Project invariants

- Write operations are disabled unless config field `i-too-like-to-live-dangerously` or `JIRA_ALLOW_WRITES=true` enables them.
- Successful JSON uses `data`, optional `errors`, and `metadata`. Do not add or document a stale `status` field.
- Errors always go through `output.WriteError` and remain JSON regardless of `--output`.
- JSON encoders must call `SetEscapeHTML(false)`.
- Use typed errors from `internal/errors`, wrap causes with `%w`, and inspect with `errors.As()` or `errors.AsType`.
- Keep `AGENTS.md`, `skills/jira-agent`, README, CodeRabbit, and Copilot review guidance aligned when commands, flags, output, auth, write behavior, or workflows change.

## Jira and security checks

- Flag any leaked Jira instance, email, API key, auth header, issue-private data, or credentials in logs, errors, fixtures, or docs.
- Jira search must use POST `/rest/api/3/search/jql` because GET can hit URL limits.
- Jira transitions need transition IDs. Status-name helpers must resolve IDs explicitly and match case-insensitively.
- ADF description conversion must preserve valid ADF JSON and convert plain text safely.
- Custom field overrides should parse valid JSON values and otherwise keep raw strings intentionally.
