# Implementation Note

## What Was Done

- Replaced the active round-based phase path with planning, development, review, testing, memory, and done.
- Added deterministic review approval and test artifact gates.
- Reopened development and stopped the run when review or testing fails.
- Removed active round fields, retry budgets, round directories, status output, and continue-round commands.
- Added stable per-EC evidence files and legacy report promotion.
- Expanded planner EC metadata and deterministic plan validation.
- Added work-item labels such as `Feature 001-EC1 Implement ABC`.
- Updated prompts, TUI workflow display, chat metadata, docs, and feature-graph learning.
- Recreated `.thanos/prompts/` before every role prompt write.
- Replaced the legacy `pending-review` completion check with implementation,
  approved-review, and passing-test evidence validation.
- Reconciled this ticket to `Feature 001-EC1 Apply project workflow contract`
  in the reviewing phase.

## Files Changed

- internal/model/model.go - workflow data model.
- internal/state/machine.go - deterministic phase transitions.
- internal/orchestrator/orchestrator.go - review, testing, and completion gates.
- internal/prompts - role contracts and stable evidence paths.
- internal/workspace/workspace.go - legacy artifact migration and completion evidence validation.
- internal/cli - remove round and continue surfaces.
- internal/tui - display the workflow and EC work-item names.
- internal/featuregraph/graph.go - learn affected paths from implementation notes.
- README.md - document the new protocol.
- README.vi.md - document the new protocol in Vietnamese.
- Technical.md - update the technical reference.

## Verification

- `go test ./...`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- Focused state, prompt, orchestrator, CLI, TUI, workspace, and feature-graph tests: PASS
- Prompt directory recreation regression: PASS
- Ready-for-review completion error regression: PASS

## Known Limitations

- Legacy phase constants remain readable for old sessions.
- The global Homebrew `thanos` binary is still version 1.0.4; use `./bin/thanos`
  for this working tree until the new build is installed.
- Independent review must be performed by a different reviewer before this ticket is marked done.

## Revision

- Working tree revision
- 2026-06-25T02:12:20Z
