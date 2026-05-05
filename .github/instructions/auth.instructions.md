---
applyTo: "internal/auth/**/*.go"
---

# Auth review instructions

- Basic auth is `base64(email:api_key)` in the Authorization header.
- Config precedence is environment variables, then config file, then defaults.
- `JIRA_INSTANCE`, `JIRA_EMAIL`, `JIRA_API_KEY`, and `JIRA_ALLOW_WRITES` override config file values.
- Verify credentials are never logged or exposed in structured errors.
- Config loading should degrade gracefully on missing files or permission errors.
