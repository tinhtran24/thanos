You are the Coder for feature "Feature Auto detect framework when init project" (F002-feature-auto-detect-framework-when-init-project), round 6.

== MANDATORY OUTPUT ==
Write .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-6/coder-report.md.

== Inputs ==
- .thanos/F002-feature-auto-detect-framework-when-init-project/task-brief.md
- .thanos/F002-feature-auto-detect-framework-when-init-project/acceptance-criteria.md
- .thanos/F002-feature-auto-detect-framework-when-init-project/test-strategy.yaml
- Previous round review and test reports under rounds/round-5


== Verify Commands ==
- Build: go build ./...
- Lint: go vet ./...
- Test: go test ./...


== Project Rules ==
- Keep role outputs isolated in .thanos.
- Do not bypass the deterministic phase state machine.
- Every implementation change requires objective test evidence.


== Commit Policy ==
Commit after each meaningful unit of work. Use messages that state what changed and why.
Before implementing, inspect git log, git diff, and any partial coder report so an interrupted
session resumes instead of repeating completed work.

== coder-report.md format ==
# Coder Report — Round 6
## What Was Done
## Files Changed
- path — description
## Verification
- command: result

== If blocked by a genuinely ambiguous decision ==
Do NOT guess. Write `.thanos/F002-feature-auto-detect-framework-when-init-project/clarify.json` as
{"question":"...","options":["option A","option B"]} and stop; the run pauses for a
human answer, then read `.thanos/F002-feature-auto-detect-framework-when-init-project/clarify-answer.md` on resume.

== Constraints ==
- Stay within the task brief and declared feature scope.
- Check every path in Persistent Feature Memory before concluding the fix is local.
- Run verification after changes.
- Do not modify acceptance criteria or other roles' reports.

== Persistent Feature Memory ==
Use this impact map before planning or changing code. Treat inherited business rules as invariants unless the task explicitly changes them.
Target: F002-feature-auto-detect-framework-when-init-project — Feature Auto detect framework when init project (feature)
Business rules and acceptance invariants:
- PHP framework Wordpress, Laravel, Go framework Gin Echo, ...
Known and inferred impact paths:
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-1/coder-report.md [documentation; coder-report]
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-2/coder-report.md [documentation; coder-report]
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-3/coder-report.md [documentation; coder-report]
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-4/coder-report.md [documentation; coder-report]
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-5/coder-report.md [documentation; coder-report]
- README.md [documentation; coder-report]
- Technical.md [documentation; coder-report]
- cmd/thanos/main.go [code; code-graph-neighbor]
- go.mod [code; coder-report]
- go.sum [code; coder-report]
- internal/cli/cli.go [code; coder-report]
- internal/cli/cli_test.go [test; coder-report]
- internal/codegraph/graph.go [code; code-graph-neighbor]
- internal/featuregraph/graph.go [code; code-graph-neighbor]
- internal/model/model.go [code; coder-report]
- internal/project/project.go [code; coder-report]
- internal/project/project_test.go [test; coder-report]
- internal/prompts/prompts.go [code; code-graph-neighbor]
- internal/state/machine.go [code; code-graph-neighbor]
- internal/tui/attachments/attachments.go [code; code-graph-neighbor]
- internal/tui/attachments/store.go [code; code-graph-neighbor]
- internal/tui/chat/chat.go [code; code-graph-neighbor]
- internal/tui/chat/stream.go [code; code-graph-neighbor]
- internal/tui/dialog/picker.go [code; code-graph-neighbor]
- internal/tui/input/input.go [code; code-graph-neighbor]
- internal/tui/list/sessions.go [code; code-graph-neighbor]
- internal/tui/styles/roles.go [code; code-graph-neighbor]
- internal/tui/tui.go [code; coder-report]
- internal/tui/tui_test.go [test; coder-report]
- internal/tui/util/text.go [code; code-graph-neighbor]
- internal/tui/view.go [code; code-graph-neighbor]
- internal/tui/workbench_test.go [test; code-graph-neighbor]
- internal/ui/exec_log.go [frontend; code-graph-neighbor]
- internal/ui/logger.go [frontend; code-graph-neighbor]
- internal/ui/status.go [frontend; code-graph-neighbor]
- internal/workspace/workspace.go [code; code-graph-neighbor]

== Codebase Graph ==
Read `.thanos/codebase/summary.md` before exploring source files. It contains the local codebase map, hub symbols, relationships, and detected conventions. Use `.thanos/codebase/graph.json` for machine-readable edges.
