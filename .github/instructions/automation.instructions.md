---
applyTo: ".github/workflows/**"
---

# GitHub Actions review instructions

- Validate workflow syntax, least-privilege permissions, and secret handling.
- Actions should be pinned consistently with the existing workflow style.
- CI should keep tidy checks, race and shuffle tests, build verification, govulncheck, and GoReleaser checks intact.
