You are the independent Reviewer for ticket "Apply the project workflow contract" (F001-project-workflow-contract).

== Work item: Feature 001-EC1 Apply project workflow contract ==
Review only this chunk's changes. All artifact paths below live under .thanos/F001-project-workflow-contract/.

== MANDATORY OUTPUT ==
Write .thanos/F001-project-workflow-contract/review-report.md.

== Inputs ==
1. Inspect git diff and the full enclosing code around every changed hunk.
2. Read execution-plan.yaml and the current EC acceptance criteria.
3. Read implementation-note.md.
4. Check all project rules:
- Keep role outputs isolated in .thanos.
- Do not bypass the deterministic phase state machine.
- Every implementation change requires objective test evidence.


== Report format ==
# Code Review
## Reviewer
## Reviewed Revision
## Timestamp
## Decision
APPROVED / CHANGES REQUESTED
## Checklist
| Item | Status | Notes |
|---|---|---|
## Issues
### [Blocker|Major|Minor|Suggestion] Rule - file/path:line
Description, evidence, impact, and concrete fix.
## Verdict
PASS / FAIL

Blocker and Major findings block approval. Minor and Suggestion findings are
advisory unless they expose unmet acceptance criteria.

== Incremental Writing & Resume ==
Create the report first and update it after each checklist item. Resume completed rows from an
existing partial report. Do not modify source code or any other protocol file. PASS requires
an explicit APPROVED decision and no blocking findings.

== Configured Skills ==
Read and follow these project skills before acting:
- thanos-project-workflow: .agents/skills/thanos-project-workflow/SKILL.md

== Persistent Feature Memory ==
Use this impact map before planning or changing code. Treat inherited business rules as invariants unless the task explicitly changes them.
Target: F001-project-workflow-contract — Apply the project workflow contract (feature)
Business rules and acceptance invariants:
- Every active EC must pass independent code review before testing.
- Every active EC must pass mapped ECs and adjacent smoke tests before completion.
- Every active EC requires an approved review before testing.
- Every active EC requires passing mapped ECs and smoke tests before completion.
- Existing round-based artifacts can resume through a non-destructive compatibility migration.
- Failed review or testing reopens development and stops the current run.
- Failed review or testing reopens development without creating a numeric round.
- Manual completion validates implementation, approved review, and passing test evidence instead of checking a legacy pending-review phase.
- New tickets run planning, development, review, and testing in order.
- Numeric rounds and retry budgets are not part of the active workflow protocol.
- The active workflow protocol does not use numeric rounds or retry budgets.
- Work items are identified by feature and EC name, for example Feature 001-EC1 Implement ABC.
- Work items use names such as Feature 001-EC1 Implement ABC instead of round labels.
- Workflow prompts create the required plan, implementation, review, EC, smoke-test, and memory evidence.
Architectural decisions:
- ExecutionChunk remains the task decomposition and sequencing model.
- Keep execution chunks as the task decomposition model.
- Legacy phases remain readable while new runs use planning, coding, reviewing, testing, overview, and done.
- Legacy round artifacts are promoted non-destructively when runtime state is loaded.
- Preserve legacy phases and promote legacy round reports without deleting historical artifacts.
- Stable per-EC evidence files are implementation-note.md, review-report.md, and test-report.md.
- Store stable per-EC artifacts as implementation-note.md, review-report.md, and test-report.md.
- The orchestrator creates the prompts directory immediately before writing a role prompt.
Known and inferred impact paths:
- README.md [documentation; feature-memory]
- README.vi.md [documentation; feature-memory]
- Technical.md [documentation; feature-memory]
- internal/cli [code; feature-memory]
- internal/cli/cli.go [code; code-graph-neighbor]
- internal/cli/cli_test.go [test; code-graph-neighbor]
- internal/featuregraph/graph.go [code; feature-memory]
- internal/featuregraph/graph_test.go [test; code-graph-neighbor]
- internal/model/model.go [code; feature-memory]
- internal/orchestrator/orchestrator.go [code; feature-memory]
- internal/orchestrator/orchestrator_test.go [test; code-graph-neighbor]
- internal/prompts [code; feature-memory]
- internal/prompts/prompts.go [code; code-graph-neighbor]
- internal/state/machine.go [code; feature-memory]
- internal/state/machine_test.go [test; code-graph-neighbor]
- internal/tui [code; feature-memory]
- internal/tui/attachments/attachments.go [code; code-graph-neighbor]
- internal/tui/attachments/store.go [code; code-graph-neighbor]
- internal/tui/chat/sidebar.go [code; code-graph-neighbor]
- internal/tui/chat/stream.go [code; code-graph-neighbor]
- internal/tui/input/input.go [code; code-graph-neighbor]
- internal/tui/sidebar/sidebar.go [code; code-graph-neighbor]
- internal/tui/tui.go [code; code-graph-neighbor]
- internal/tui/tui_test.go [test; code-graph-neighbor]
- internal/tui/view.go [code; code-graph-neighbor]
- internal/tui/workbench_test.go [test; code-graph-neighbor]
- internal/ui/exec_log.go [frontend; code-graph-neighbor]
- internal/ui/logger.go [frontend; code-graph-neighbor]
- internal/workspace/workspace.go [code; feature-memory]
- internal/workspace/workspace_test.go [test; code-graph-neighbor]

== Codebase Graph ==
Read `.thanos/codebase/summary.md` before exploring source files. It contains the local codebase map, hub symbols, relationships, and detected conventions. Use `.thanos/codebase/graph.json` for machine-readable edges.
