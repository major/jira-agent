# jira-agent agent guide

Generated: Mon May 04 2026. Source state: branch `feat/llm-ux-features`, commit `5a0eeab`.

## Purpose and audience

`jira-agent` is a command line tool meant to be used by large tool-calling LLMs first: Claude Opus, GPT, and similar agents. Human users may enjoy it too, but the primary contract is helping an LLM get the right Jira answer on the first try. Favor structured output, schema discovery, bounded responses, deterministic behavior, and remediation-rich errors over human-oriented prose.

Keep this file and any nested `AGENTS.md` files current anytime code changes. Every code change must include a quick check that the affected guidance still matches the implementation. Keep `skills/jira-agent` files updated in the same change whenever code changes affect commands, flags, args, output contracts, auth/write behavior, errors, pagination, examples, or recommended LLM workflows. Keep guidance compact and source-backed.

## Project map

| Path | Role |
| --- | --- |
| `cmd/jira-agent/main.go` | Real executable entrypoint, root CLI wiring, global flags, `PersistentPreRunE` auth hook, command registration. |
| `internal/commands` | Domain command tree. See `internal/commands/AGENTS.md` before changing commands. |
| `internal/commands/composite.go` | Composite workflow commands: `issue start-work`, `issue close`, `issue create-and-link`, `issue move-to-sprint`. |
| `internal/commands/shortcuts.go` | Smart shortcut commands: `issue mine`, `issue recent`. |
| `internal/commands/dryrun.go` | Dry-run infrastructure: `DryRunResult`, `ComputeFieldDiff`, `WriteDryRunResult`, `IsDryRun`. |
| `internal/commands/schema.go` | Schema command: emits live Cobra command tree as machine-readable JSON, no auth required. |
| `internal/commands/jql_helpers.go` | `buildJQLFromFlags`: pure function that builds JQL from semantic flag values. |
| `internal/client` | Jira REST API v3 and Agile API HTTP client, Basic auth headers, content-type checks, typed error mapping. |
| `internal/output` | All user-visible envelopes and CSV/TSV serialization. Output is an LLM-facing contract. |
| `internal/errors` | Typed errors and exit codes. Preserve wrapping and typed inspection. |
| `internal/auth` | Config loading, env overrides, Basic auth, write-enable config. |
| `internal/testhelpers` | Shared test HTTP helpers. |
| `skills/jira-agent` | Embedded LLM skill docs. See `skills/jira-agent/AGENTS.md` before changing skill files. |

Module: `github.com/major/jira-agent`. Go version: `1.26`. CLI framework: `github.com/spf13/cobra`.

## OpenAPI references

- Jira Cloud Platform REST API (`/rest/api/3`): `https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json`.
- Jira Software Cloud REST API (`/rest/agile/1.0`): `https://dac-static.atlassian.com/cloud/jira/software/swagger.v3.json`.

## LLM-first CLI contract

- Prefer canonical paths in docs and examples, not legacy aliases. Examples: `issue bulk <action>`, `issue remote-link`.
- Output must be organized so an LLM can parse it without scraping help text or human prose.
- Bounded output matters. Prefer commands and examples that request only needed fields, for example `--fields key,summary,status`.
- Default JSON removes noisy Jira API metadata such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects. `issue get` and `issue search` convert ADF descriptions to text by default unless `--description-output-format markdown|adf` or `--raw` is used. `issue search` is additionally compact by default: `.data.issues[]` contains flattened issue rows and common Jira wrapper objects collapse to useful scalar values. Use `--raw` on commands that expose it only when an agent needs Jira's unmodified nested response.
- Use CSV/TSV only for simple flat tables. Use JSON for nested Jira data, updates, errors, and partial success.
- `--pretty` is for humans only. It must never appear in `skills/jira-agent` files or LLM-facing examples.
- `jira-agent schema` emits the full CLI command tree as JSON without requiring auth. Use it for command discovery before falling back to `--help` scraping.

## Root CLI invariants

- `buildAppWithDeps` pre-allocates `apiClient := &client.Ref{}` and fills it in the root `PersistentPreRunE` hook. Do not replace this with per-command clients; command closures share that reference intentionally.
- `outputFormat` and `allowWrites` are shared pointers set during root initialization and consumed by commands.
- `help`, `--version`, and empty root invocation must not load auth. Jira project-version commands require auth.
- Root flags include `--project/-p` from `JIRA_PROJECT`, `--output/-o` as `json|csv|tsv`, `--pretty`, `--verbose/-v`, `--config`, `--compact` (strips nulls/empties from JSON output, flattens single-key nested objects, JSON Lines for arrays), and `--dry-run` (composite commands only: preview field diff without mutating).
- cobra uses `SilenceErrors = true` and `SilenceUsage = true` on the root command; error output goes through `output.WriteError` in `main`.
- Verbose logging uses JSON slog to stderr. Data output stays on the configured writer.

## Auth, config, and writes

- Basic auth is `base64(email:api_key)`.
- Config precedence is environment over file: `JIRA_INSTANCE`, `JIRA_EMAIL`, `JIRA_API_KEY`, `JIRA_ALLOW_WRITES`, then `~/.config/jira-agent/config.json` or `XDG_CONFIG_HOME`.
- Config field `"i-too-like-to-live-dangerously": true` or env `JIRA_ALLOW_WRITES=true` enables writes.
- Writes are disabled by default. Mutating commands must use `writeGuard` or `requireWriteAccess`.
- Blocked writes return `ValidationError`, exit 5, with remediation: set `"i-too-like-to-live-dangerously": true` in config or `JIRA_ALLOW_WRITES=true`.

## Output and error contracts

- Successful JSON goes through `output.WriteSuccess` or `output.WriteResult`; partial success goes through `output.WritePartial`.
- Errors always go through `output.WriteError` and are JSON regardless of requested output format.
- The JSON success envelope has `data`, optional `errors`, and `metadata`. Do not document or add a stale `status` field unless the code changes.
- `metadata` carries `timestamp`, `total`, `returned`, `start_at`, and `max_results` when available. Paginated responses also include `has_more` (bool, always present) and `next_command` (string, present when `has_more` is true) so agents can page without constructing the next command manually.
- Error detail objects include `next_command` (string) and `available_actions` ([]string) when applicable, so agents can recover without guessing.
- JSON encoders must call `SetEscapeHTML(false)` so Jira URLs and text stay LLM-readable.
- CSV/TSV output is flat rows with deterministic sorted headers; nested values serialize as inline JSON strings.
- Exit codes: not found 2, auth 3, API 4, validation 5, unknown 1.

## Client and Jira API rules

- HTTP defaults: 30 second timeout, 10 MB response cap, default user-agent `jira-agent/dev` unless overridden.
- Set `Content-Type` only when a request body exists.
- Validate response `Content-Type` before JSON decode.
- Map 401/403 to auth errors, 404 to not found, and 400+ to API errors with useful response details.
- Use Jira search POST `/rest/api/3/search/jql`; GET can hit URL limits.
- Jira transitions need transition IDs. If a workflow helper accepts status names, resolve them explicitly and keep matching case-insensitive.
- Jira Cloud REST v3 requires ADF for issue descriptions. `--description` defaults to auto mode for plain text or ADF JSON; `--description-format wiki` converts common Jira wiki headings and bullet lists before sending. Read commands default to text descriptions with `--description-output-format text|markdown|adf` on `issue get` and `issue search`.

## Error handling style

- Use typed errors from `internal/errors`.
- Wrap with `%w` so callers can inspect causes.
- Inspect typed errors with `errors.As` or Go 1.26 `errors.AsType`, not raw type assertions.
- Do not swallow API response details; LLMs need the body/status context to recover.

## Testing workflow

- TDD is mandatory for behavior changes.
- Prefer Makefile targets: `make lint`, `make test`, `make build`, `make vuln`, `make release-check`, `make release-snapshot`.
- Focused tests: `go test -run TestName ./path/to/package`.
- Full CI-style tests: `go test -v -race -shuffle=on -coverprofile=coverage.out ./...`.
- Use standard Go assertions only. Do not add testify or mocking frameworks.
- Setup failures use `t.Fatalf`; assertions use `t.Errorf` with got/want wording.
- Put `t.Parallel()` first in tests and subtests unless shared state makes parallelism unsafe.
- Use table-driven `t.Run`; helpers are unexported and call `t.Helper()`.
- Prefer `httptest`, in-memory buffers, `appDeps`, and `client.WithHTTPClient`. No networked Jira integration tests without an explicit prerequisite or build tag.

## Lint and release

- `.golangci.yml` enables `bodyclose`, `errorlint`, `gocognit`, `gocritic`, `gosec`, `misspell`, `modernize`, `nolintlint`, `revive`, `unconvert`, and `unparam`.
- Every `nolint` needs a linter name and explanation.
- CI checks `go mod tidy && git diff --exit-code -- go.mod go.sum`, full race/shuffle tests, `go build ./cmd/jira-agent/`, govulncheck, and `goreleaser check`.
- Releases use `make release VERSION=vX.Y.Z` from clean `main`, run checks, and create a signed tag from conventional commit history. `make release-snapshot` skips signing.

## Release process

- Release from a clean, up-to-date `main` branch. Confirm with `git status --short --branch`, `git fetch --tags origin`, and `git tag --list --sort=version:refname` before choosing the next SemVer tag.
- Use `make release-snapshot` before the real release when changing GoReleaser config or release packaging. It runs `goreleaser check` and `goreleaser release --snapshot --clean --skip=sign` without creating a tag or publishing artifacts.
- Create the signed release tag with `make release VERSION=vX.Y.Z`. The target requires `VERSION`, verifies `main`, verifies a clean tree, runs `make test`, `make lint`, `make vuln`, and signs an annotated tag whose message is built from conventional commits since the previous tag.
- Push only the release tag after the Makefile succeeds: `git push origin vX.Y.Z`. The tag push triggers `.github/workflows/release.yml`, which runs GoReleaser with GitHub release permissions and Sigstore keyless checksum signing.
- Do not run GoReleaser locally for publishing. Local release work should stop at the signed tag; GitHub Actions owns published archives, checksums, signatures, and the GitHub Release.
