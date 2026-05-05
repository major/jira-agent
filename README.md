# jira-agent

[![CI](https://github.com/major/jira-agent/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/major/jira-agent/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/major/jira-agent.svg)](https://pkg.go.dev/github.com/major/jira-agent)
[![Go Report Card](https://goreportcard.com/badge/github.com/major/jira-agent)](https://goreportcard.com/report/github.com/major/jira-agent)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/major/jira-agent/badge)](https://securityscorecards.dev/viewer/?uri=github.com/major/jira-agent)
[![Go Version](https://img.shields.io/github/go-mod/go-version/major/jira-agent)](https://go.dev/)
[![License](https://img.shields.io/github/license/major/jira-agent)](./LICENSE)

`jira-agent` is an LLM-first command line tool for Jira Cloud.

This is an unofficial project. It is not produced, endorsed, supported, or affiliated with Atlassian or the official Jira product teams.

It wraps the Jira REST API v3 and Jira Software Agile API with structured output, command-line help, deterministic exit codes, and write protection by default. Human users can run it directly, but the main contract is helping tool-calling agents discover the right Jira command without scraping help text.

## Install

Download a release archive from [GitHub Releases](https://github.com/major/jira-agent/releases), or install from source:

```bash
go install github.com/major/jira-agent/cmd/jira-agent@latest
```

Verify the binary:

```bash
jira-agent --version
jira-agent --help
```

## Configure

Environment variables override the config file:

```bash
export JIRA_INSTANCE="your-domain.atlassian.net"
export JIRA_EMAIL="you@example.com"
export JIRA_API_KEY="your-api-token"
```

The config file fallback is `~/.config/jira-agent/config.json`, with `XDG_CONFIG_HOME` support:

```json
{
  "instance": "your-domain.atlassian.net",
  "email": "you@example.com",
  "api_key": "your-api-token"
}
```

Confirm authentication:

```bash
jira-agent whoami
```

Writes are disabled unless explicitly enabled:

```bash
export JIRA_ALLOW_WRITES=true
```

Or set this in the config file:

```json
{
  "i-too-like-to-live-dangerously": true
}
```

Blocked writes return a JSON validation error with exit code `5` and remediation guidance.

## Usage

Use `jira-agent schema` to discover all commands, flags, and constraints as machine-readable JSON without requiring auth:

```bash
jira-agent schema
```

Or use `--help` to explore interactively:

```bash
jira-agent --help
jira-agent issue --help
jira-agent issue create --help
```

Common read-only commands:

```bash
jira-agent issue get PROJ-123 --fields key,summary,status,assignee
jira-agent issue search --jql "assignee = currentUser()"
jira-agent issue mine
jira-agent sprint current --board-id 42
jira-agent project list --output csv
```

Semantic JQL flags let you skip writing raw JQL for common searches:

```bash
jira-agent issue search --assignee me --status "In Progress"
jira-agent issue search --sprint current --type Bug
```

Composite commands collapse multiple API calls into one invocation and require write access:

```bash
jira-agent issue start-work PROJ-123
jira-agent issue close PROJ-123 --resolution "Won't Do"
jira-agent issue move-to-sprint PROJ-123 --sprint-id 42 --status "In Progress"
```

Use `--dry-run` on composite commands to preview changes without mutating:

```bash
jira-agent issue start-work PROJ-123 --dry-run
```

Mutation commands require write access:

```bash
jira-agent issue create --project PROJ --type Story --summary "New feature"
```

JSON is the default output format. CSV and TSV are available for flat tables:

```bash
jira-agent issue search --jql "project = PROJ" --fields key,summary,status --output tsv
```

Use `--compact` to strip nulls and empty values from JSON output, flatten single-key nested objects, and get JSON Lines for arrays:

```bash
jira-agent issue get PROJ-123 --compact
jira-agent issue search --jql "project = PROJ" --compact
```

Default JSON removes noisy Jira metadata such as `self`, `expand`, `avatarUrls`, `iconUrl`, and nested `statusCategory` objects to keep LLM context small. `issue get` and `issue search` convert ADF descriptions to text by default; use `--description-output-format markdown` or `adf` when needed. `issue search` also returns compact rows by default. Add `--raw` on commands that expose it when you need Jira's full nested API response.

## Command areas

| Area | Commands | Detailed docs |
| --- | --- | --- |
| Issues | `issue get`, `issue search`, `issue count`, `issue create`, `issue edit`, `issue bulk`, `issue mine`, `issue recent` | [`skills/jira-agent/issues.md`](./skills/jira-agent/issues.md) |
| Issue composites | `issue start-work`, `issue close`, `issue create-and-link`, `issue move-to-sprint` | [`skills/jira-agent/issues.md`](./skills/jira-agent/issues.md) |
| Issue workflows | transitions, assignment, comments, worklogs, watchers, votes, links, attachments | [`skills/jira-agent/issue-workflows.md`](./skills/jira-agent/issue-workflows.md) |
| Agile | boards, sprints, epics, backlog, `sprint current` | [`skills/jira-agent/agile.md`](./skills/jira-agent/agile.md) |
| Projects | projects, components, versions | [`skills/jira-agent/project-management.md`](./skills/jira-agent/project-management.md) |
| Admin/reference | fields, users, groups, filters, permissions, dashboards, workflows, statuses, JQL, server info | [`skills/jira-agent/admin-reference.md`](./skills/jira-agent/admin-reference.md) |
| Discovery | `schema` (no auth required) | [`skills/jira-agent/SKILL.md`](./skills/jira-agent/SKILL.md) |

The skill entry point, [`skills/jira-agent/SKILL.md`](./skills/jira-agent/SKILL.md), documents auth, global flags, command discovery, output contracts, exit codes, and Jira-specific gotchas for agents.

## Output contract

Successful JSON responses use a stable envelope:

```json
{
  "data": {},
  "metadata": {
    "timestamp": "2026-04-28T00:00:00Z",
    "pagination": {
      "type": "offset",
      "has_more": true,
      "next_command": "jira-agent issue mine --max-results 10 --start-at 10",
      "returned": 10,
      "total": 42,
      "start_at": 0,
      "max_results": 50
    }
  }
}
```

`metadata.pagination.has_more` is always present in paginated responses. When true, `metadata.pagination.next_command` contains the ready-to-run command for the next page.

Errors are always JSON, regardless of `--output`:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "write operations are not enabled",
    "details": "set \"i-too-like-to-live-dangerously\": true in config file or JIRA_ALLOW_WRITES=true env var to enable write operations",
    "next_command": "export JIRA_ALLOW_WRITES=true",
    "available_actions": []
  }
}
```

`next_command` and `available_actions` appear in errors when applicable.

| Exit | Meaning |
| --- | --- |
| `0` | Success |
| `2` | Not found |
| `3` | Authentication or authorization failed |
| `4` | Jira API error |
| `5` | Validation error, including blocked writes |
| `1` | Unknown error |

## Development

```bash
make build
make test
make lint
make vuln
```

The project uses Go 1.26, `spf13/cobra`, `golangci-lint`, `govulncheck`, and GoReleaser. See [`AGENTS.md`](./AGENTS.md) for repository architecture, command conventions, testing expectations, and release workflow details.

## License

MIT, see [`LICENSE`](./LICENSE).
