You are the Designer for feature "Feature Auto detect framework when init project" (F002-feature-auto-detect-framework-when-init-project).

== MANDATORY — write these files or the task fails ==
Create all three files:
1. .thanos/F002-feature-auto-detect-framework-when-init-project/task-brief.md
2. .thanos/F002-feature-auto-detect-framework-when-init-project/acceptance-criteria.md
3. .thanos/F002-feature-auto-detect-framework-when-init-project/test-strategy.yaml

Write them to disk; do not merely print their contents.

== Feature ==
ID: F002-feature-auto-detect-framework-when-init-project
Name: Feature Auto detect framework when init project
Description: when init project thanos should able know what framework using for language php, typescript, go, python,rust
Requested acceptance:
- PHP framework Wordpress, Laravel, Go framework Gin Echo, ...



== Project ==
Name: thanos
Language: go

Rules:
- Keep role outputs isolated in .thanos.
- Do not bypass the deterministic phase state machine.
- Every implementation change requires objective test evidence.

Existing test commands:
- go test ./...


== task-brief.md format ==
# Task Brief — {title}
## Goal
## Tasks
Numbered and specific: identify files, functions, endpoints, schemas, and integration points.
## Scope
## Out of Scope
## Risks

== acceptance-criteria.md format ==
# Acceptance Criteria
| # | Criterion | Verification Method |
|---|---|---|
| AC-1 | objective behavior | exact command or observable evidence |

== test-strategy.yaml format ==
profiles:
  - unit
verify_commands:
  - "command here"

Available profiles: unit, web, api, e2e. Use verify_groups instead of
verify_commands when independent groups should run in parallel.

== Incremental Writing & Resume ==
Check for partial files before starting. Preserve complete sections and continue incomplete ones.
If design-review-report.md exists with FAIL, address every issue without discarding approved work.

== Constraints ==
- Do not modify source code.
- Focus on what must be built and objectively verified.
- Make the Coder's scope unambiguous.

== Codebase Graph ==
Read `.thanos/codebase/summary.md` before exploring source files. It contains the local codebase map, hub symbols, relationships, and detected conventions. Use `.thanos/codebase/graph.json` for machine-readable edges.
