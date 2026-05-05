---
applyTo: "internal/client/**/*.go"
---

# Client review instructions

- `client.Ref` embeds `*Client`; it is pre-allocated and shared by commands.
- The root before hook populates `ref.Client` after auth completes.
- Preserve functional options such as `WithBaseURL`, `WithHTTPClient`, `WithLogger`, and `WithUserAgent`.
- Keep the 30 second timeout and 10 MB response cap unless the reason is documented.
- Set `Content-Type` only when a request has a body.
- Validate response `Content-Type` before JSON decode because Jira proxies may return HTML.
- Map HTTP failures to typed errors with useful response details.
