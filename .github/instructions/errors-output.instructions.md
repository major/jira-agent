---
applyTo: "internal/{errors,output}/**/*.go"
---

# Errors and output review instructions

- All project errors should embed the `JiraError` base type and use constructor functions.
- Exit codes are auth 3, not found 2, API 4, validation 5, and unknown 1.
- Error options should preserve remediation hints through `WithDetails` where useful.
- Error responses are always JSON regardless of the selected output format.
- CSV and TSV output should stay flat and deterministic.
- Schema command output is raw JSON for direct LLM consumption.
- JSON encoders must call `SetEscapeHTML(false)`.
