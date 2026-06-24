# Review Report — Round 1
## Summary
FAIL
## Checklist
| Item | Status | Notes |
|---|---|---|
| Codebase graph read before source exploration | INFO | Required `.thanos/codebase/summary.md` and `.thanos/codebase/graph.json` are absent, so no graph could be read. |
| Task brief, acceptance criteria, and coder report | PASS | Reviewed the complete round inputs. |
| Git diff and enclosing code | PASS | Reviewed all nine changed implementation/documentation files and the full enclosing detector, init, model, and test code. |
| Framework detector behavior | FAIL | Python requirements parsing accepts invalid trailing text as framework evidence. |
| CLI integration and persistence | PASS | Detection runs after the language override and before `Workspace.Init`; trimmed non-empty framework overrides detection; empty values use `omitempty`. |
| Documentation contract | PASS | Both root documents independently contain the required framework values, evidence categories, selection, ambiguity, omission, and execution guarantees. |
| Project rule isolation and state-machine integrity | PASS | Implementation changes are outside `.thanos`; only this reviewer output is added there, and no `internal/state` or `internal/orchestrator` diff exists. |
| Objective test evidence | FAIL | Focused tests, documentation test, formatting checks, module consistency, help check, and `go test ./...` pass; an added read-only overlay regression for `django garbage` fails. |
## Issues
### [MEDIUM] Python evidence contract — internal/project/project.go:267
`addPythonRequirement` extracts a leading distribution-shaped token and accepts any suffix beginning with whitespace. As a result, the invalid requirements line `django garbage` is classified as `django`, even though the task requires parsing a leading PEP 508 requirement rather than substring-like prefix matching. Evidence: a temporary overlay test calling `DetectFramework` with a root `requirements.txt` containing only `django garbage` returned `"django"` and failed with `framework = "django", want empty for invalid requirement`. This can persist a false framework from arbitrary prose or malformed requirement lines and can also create false ambiguity that suppresses a valid framework from another Python source. Replace the first-character suffix check with actual PEP 508 requirement validation (or a strict parser covering extras, version specifiers including parenthesized forms, direct references, markers, and comments), and add positive and negative regression cases such as `Django(>=5)` and `django garbage`.
## Verdict
FAIL
