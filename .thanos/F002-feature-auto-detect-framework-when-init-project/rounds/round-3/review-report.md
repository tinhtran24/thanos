# Review Report — Round 3
## Summary
FAIL
## Checklist
| Item | Status | Notes |
|---|---|---|
| Required inputs and codebase graph | INFO | `task-brief.md`, `acceptance-criteria.md`, and `coder-report.md` reviewed. Required `.thanos/codebase/summary.md` and `.thanos/codebase/graph.json` are absent, so direct repository inspection was used. |
| Changed hunks and enclosing code | PASS | Reviewed all nine changed implementation, test, module, and documentation files, plus the enclosing detector, initialization, workspace persistence, model, and prior-round review context. |
| Framework detection correctness | FAIL | Round-2 Python marker regressions are fixed, but Rust detection accepts unsupported TOML value types as dependency evidence instead of limiting declarations to strings and tables. |
| CLI integration, persistence, and help | PASS | Final language selection precedes detection; trimmed non-empty overrides are persisted; whitespace-only overrides retain detection; detector errors precede `Workspace.Init`; empty values are omitted; help includes `--framework`. |
| Documentation contract | PASS | `README.md` and `Technical.md` independently document the metadata/flag, all canonical values, root evidence categories, final-language selection, ambiguity, omission, and no-network/no-command guarantees; the documentation contract test passes. |
| Project rules and state-machine isolation | PASS | Reviewer output is isolated to this round report. Feature state is `reviewing`, reviewer, round 3 with ordered `amending` → `reviewing` events; no `internal/state` or `internal/orchestrator` diff exists. |
| Objective verification evidence | FAIL | Focused project/CLI tests, full `go test ./...`, build, vet, formatting, diff checks, help, architecture guards, and coder-reported module consistency pass. No test covers unsupported Rust dependency value types, allowing the contract violation above to pass the suite. |
## Issues
### [MEDIUM] Rust evidence contract — internal/project/project.go:603
`addCargoDependencies` ignores the type of each dependency declaration unless it happens to be a table with a string `package` field. It then matches the table key for every other TOML value, so declarations such as `axum = true`, `rocket = 1`, or `actix_web = ["4"]` are classified as supported frameworks. The task contract permits only string or table dependency declarations; other TOML value types are malformed/unsupported evidence and must not add a match. This can persist a false framework or create false ambiguity that suppresses a valid Rust framework. Add a type switch that processes only `string` and `map[string]any` declarations, retaining the current renamed-`package` handling for tables, and add negative tests for boolean, numeric, and array dependency values.
## Verdict
FAIL
