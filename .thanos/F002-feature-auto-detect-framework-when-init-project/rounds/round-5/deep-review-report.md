# Deep Review Report — Round 5
## Summary
FAIL

The framework detector and init integration pass their focused and repository-wide checks. However, AC-10 is still unresolved: its working-tree guard reports success only because forbidden state-machine and orchestrator changes were committed as part of the feature history. Comparing the current feature state with the pre-feature baseline shows both directories changed, and the direct state-mutation helper introduced by the feature still exists.

## Acceptance Criteria Trace
| # | Criterion | Implementation | Verified |
|---|---|---|---|
| AC-1 | Optional persisted `project.framework` with omitted empty value | `internal/model/model.go:39-50`; `internal/cli/cli.go:129-156` | Yes |
| AC-2 | PHP Laravel and WordPress root evidence | `internal/project/project.go:90-135` | Yes |
| AC-3 | TypeScript exact root dependency evidence | `internal/project/project.go:137-165` | Yes |
| AC-4 | Go exact `require` evidence for Gin and Echo | `internal/project/project.go:167-187` | Yes |
| AC-5 | Python supported TOML and requirements evidence | `internal/project/project.go:189-567` | Yes |
| AC-6 | Rust supported dependency-table evidence | `internal/project/project.go:569-619` | Yes |
| AC-7 | Normalized language selection and ambiguity handling | `internal/project/project.go:59-88` | Yes |
| AC-8 | Missing/malformed evidence and filesystem error contract | `internal/project/project.go:93-134`, `140-149`, `170-177`, `192-230`, `572-579`, `621-644` | Yes |
| AC-9 | Final language, framework override, and detector failure ordering | `internal/cli/cli.go:119-156` | Yes |
| AC-10 | Root-only/read-only/no-command behavior; no state-machine or orchestrator changes | Detector behavior: `internal/project/project.go:59-644`; forbidden changes: `internal/state/machine.go:61-75`, `internal/orchestrator/orchestrator.go:31-45` | **No** |
| AC-11 | Help and independent documentation contract | `internal/cli/cli.go:101`; `README.md:112-137`; `Technical.md:119-145` | Yes |
| AC-12 | Focused detector and CLI tests | `internal/project/project_test.go:103-365`; `internal/cli/cli_test.go:87-217` | Yes |
| AC-13 | Full suite, formatting, vet, and module consistency | Repository verification completed in round 5 | Yes |

## Issues
### [WARNING] Angle 11: Acceptance trace — AC-10 is checked against the wrong baseline and forbidden feature changes remain — internal/state/machine.go:61 (confidence: 100)
**What**:
Round 5 removed the active call to `ResumeFailedRound`, but did not remove the exported helper or the feature’s committed state-machine/orchestrator changes. The reported `git diff --exit-code -- internal/state internal/orchestrator` only compares the working tree with `HEAD`; it cannot detect changes already committed by this feature.

**Why it matters**:
AC-10 and the task scope explicitly require that state-machine and orchestrator files are unchanged. The surviving helper directly assigns phase, role, round, active state, and reason instead of using `Transition`, weakening the inherited invariant that phase changes go through the deterministic state machine. The feature also changes orchestrator startup and acceptance behavior, so framework detection is not isolated from workflow execution.

**Evidence**:
- `internal/state/machine.go:61-75` still defines `ResumeFailedRound` and directly mutates workflow state.
- `internal/orchestrator/orchestrator.go:31-45` adds feature-memory rebuild/update work to every orchestrator run.
- `internal/orchestrator/orchestrator.go:136-158` changes required acceptance artifacts and persistence behavior.
- `internal/orchestrator/orchestrator.go:198-220` changes role-execution output and timing behavior.
- `.thanos/F002-feature-auto-detect-framework-when-init-project/acceptance-criteria.md:14` requires no state-machine or orchestrator changes.
- `.thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-5/test-report.md:16` cites only the working-tree guard as proof.
- Against the pre-feature baseline `f86420b`, `git diff --exit-code f86420b -- internal/state internal/orchestrator` exits 1.

**Suggested fix**:
Split or rebase F002 so its patch contains only the approved framework-detection files. Remove `ResumeFailedRound` and all state/orchestrator changes from this feature. Verify AC-10 against the actual feature base, for example `git diff --exit-code <feature-base>...HEAD -- internal/state internal/orchestrator`, not only against the working tree.

## Statistics
- Acceptance criteria: 13 total; 12 verified; 1 failed.
- Blocking issues: 0 CRITICAL, 1 WARNING.
- Independent checks: `go test ./...`, `go vet ./...`, targeted gofmt, `git diff --check`, forbidden detector imports, and help output passed.
- Feature-memory impact paths traced: detector/model/CLI/tests/docs verified; state-machine and orchestrator paths are affected rather than proven unchanged.

## Verdict
FAIL
