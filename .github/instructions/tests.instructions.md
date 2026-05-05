---
applyTo: "**/*_test.go"
---

# Test review instructions

- Use standard Go assertions only. Do not add assertion libraries.
- Setup failures use `t.Fatalf`; assertions use `t.Errorf` with got/want wording.
- Put `t.Parallel()` first in tests and subtests unless shared state makes it unsafe.
- Prefer table-driven subtests with `t.Run()`.
- Test helpers should be unexported and call `t.Helper()`.
- Use inline test data and avoid a `testdata/` directory.
- Prefer `httptest`, in-memory buffers, `appDeps`, and dependency injection over mocks.
