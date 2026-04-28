# jira-agent

[![CI](https://github.com/major/jira-agent/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/major/jira-agent/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/major/jira-agent.svg)](https://pkg.go.dev/github.com/major/jira-agent)
[![Go Report Card](https://goreportcard.com/badge/github.com/major/jira-agent)](https://goreportcard.com/report/github.com/major/jira-agent)
[![Go Version](https://img.shields.io/github/go-mod/go-version/major/jira-agent)](https://go.dev/)
[![License](https://img.shields.io/github/license/major/jira-agent)](./LICENSE)

`jira-agent` is an LLM-first command line tool for Jira Cloud.

This is an unofficial project. It is not produced, endorsed, supported, or affiliated with Atlassian or the official Jira product teams.

It wraps the Jira REST API v3 and Jira Software Agile API with structured output, schema discovery, deterministic exit codes, and write protection by default. Human users can run it directly, but the main contract is helping tool-calling agents discover the right Jira command without scraping help text.

## Install

Download a release archive from [GitHub Releases](https://github.com/major/jira-agent/releases), or install from source:

```bash
go install github.com/major/jira-agent/cmd/jira-agent@latest
```

Verify the binary:

```bash
jira-agent --version
jira-agent schema --compact
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

Start with schema discovery instead of guessing flags:

```bash
jira-agent schema --compact
jira-agent schema --category issue --required-only --depth 1
jira-agent schema --command "issue create" --required-only
```

Common read-only commands:

```bash
jira-agent issue get PROJ-123 --fields key,summary,status,assignee
jira-agent issue search --jql "assignee = currentUser()" --fields key,summary,status
jira-agent project list --output csv
```

Mutation commands require write access:

```bash
jira-agent issue create --project PROJ --type Story --summary "New feature"
```

JSON is the default output format. CSV and TSV are available for flat tables:

```bash
jira-agent issue search --jql "project = PROJ" --fields key,summary,status --output tsv
```

## Command areas

| Area | Commands | Detailed docs |
| --- | --- | --- |
| Issues | `issue get`, `issue search`, `issue create`, `issue edit`, `issue bulk` | [`skills/jira-agent/issues.md`](./skills/jira-agent/issues.md) |
| Issue workflows | transitions, assignment, comments, worklogs, watchers, votes, links, attachments | [`skills/jira-agent/issue-workflows.md`](./skills/jira-agent/issue-workflows.md) |
| Agile | boards, sprints, epics, backlog | [`skills/jira-agent/agile.md`](./skills/jira-agent/agile.md) |
| Projects | projects, components, versions | [`skills/jira-agent/project-management.md`](./skills/jira-agent/project-management.md) |
| Admin/reference | fields, users, groups, filters, permissions, dashboards, workflows, statuses, JQL, server info | [`skills/jira-agent/admin-reference.md`](./skills/jira-agent/admin-reference.md) |

The skill entry point, [`skills/jira-agent/SKILL.md`](./skills/jira-agent/SKILL.md), documents auth, global flags, schema discovery, output contracts, exit codes, and Jira-specific gotchas for agents.

## Output contract

Successful JSON responses use a stable envelope:

```json
{
  "data": {},
  "metadata": {
    "timestamp": "2026-04-28T00:00:00Z",
    "total": 42,
    "returned": 10
  }
}
```

Errors are always JSON, regardless of `--output`:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "write operations are not enabled",
    "details": "set \"i-too-like-to-live-dangerously\": true in config file or JIRA_ALLOW_WRITES=true env var to enable write operations"
  }
}
```

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

The project uses Go 1.26, `urfave/cli/v3`, `golangci-lint`, `govulncheck`, and GoReleaser. See [`AGENTS.md`](./AGENTS.md) for repository architecture, command conventions, testing expectations, and release workflow details.

## License

MIT, see [`LICENSE`](./LICENSE).
