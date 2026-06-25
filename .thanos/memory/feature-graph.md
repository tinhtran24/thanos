# Thanos Feature Memory

Generated: 2026-06-25T02:26:35Z

- Features: 1
- Relationships: 0

## F001-project-workflow-contract — Apply the project workflow contract

- Type: feature
- Status: ready-for-review
- Rules:
  - Every active EC must pass independent code review before testing.
  - Every active EC must pass mapped ECs and adjacent smoke tests before completion.
  - Every active EC requires an approved review before testing.
  - Every active EC requires passing mapped ECs and smoke tests before completion.
  - Failed review or testing reopens development and stops the current run.
  - Manual completion validates implementation, approved review, and passing test evidence instead of checking a legacy pending-review phase.
  - Numeric rounds and retry budgets are not part of the active workflow protocol.
  - The active workflow protocol does not use numeric rounds or retry budgets.
  - Work items are identified by feature and EC name, for example Feature 001-EC1 Implement ABC.
- Architectural decisions:
  - ExecutionChunk remains the task decomposition and sequencing model.
  - Keep execution chunks as the task decomposition model.
  - Legacy phases remain readable while new runs use planning, coding, reviewing, testing, overview, and done.
  - Legacy round artifacts are promoted non-destructively when runtime state is loaded.
  - Preserve legacy phases and promote legacy round reports without deleting historical artifacts.
  - Stable per-EC evidence files are implementation-note.md, review-report.md, and test-report.md.
  - Store stable per-EC artifacts as implementation-note.md, review-report.md, and test-report.md.
  - The orchestrator creates the prompts directory immediately before writing a role prompt.
- Affected paths:
  - `internal/model/model.go` (code)
  - `internal/state/machine.go` (code)
  - `internal/orchestrator/orchestrator.go` (code)
  - `internal/prompts` (code)
  - `internal/workspace/workspace.go` (code)
  - `internal/cli` (code)
  - `internal/tui` (code)
  - `internal/featuregraph/graph.go` (code)
  - `README.md` (documentation)
  - `README.vi.md` (documentation)
  - `Technical.md` (documentation)

