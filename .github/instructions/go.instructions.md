---
applyTo: "internal/**/*.go"
---

# Go review instructions

- Use `fmt.Errorf("context: %w", err)` when wrapping errors.
- Use `errors.As()` or `errors.AsType` for typed error matching, not raw type assertions.
- Avoid `init()` functions. Setup belongs in `main()` and the root pre-run hook.
- Command constructors should accept `(apiClient *client.Ref, w io.Writer, format *output.Format)`.
- All JSON output must go through `output.WriteSuccess`, `output.WriteResult`, `output.WriteError`, or `output.WritePartial`.
- Do not suggest style-only changes that gofmt or golangci-lint already enforces.
