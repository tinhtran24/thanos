---
name: thanos-project-workflow
description: Run one Thanos project ticket through QA planning, implementation, independent code review, EC-based testing, smoke testing, completion, and `.thanos` feature-graph memory updates. Use when creating or updating a ticket workflow, decomposing work into subtasks or execution chunks, implementing a ticket, reviewing changes, defining or executing ECs/test cases, handling failed testing, recording evidence, or updating long-term Thanos project memory.
---

# Thanos Project Workflow

Move one ticket through:

```text
QA Analyst -> Developer -> Reviewer -> Tester -> Done
```

Treat the filesystem and `.thanos` artifacts as the source of truth. Preserve
the repository's current architecture and state-machine invariants.

## Start With Repository Discovery

Before changing code:

1. Read the ticket, acceptance criteria, linked artifacts, and screenshots.
2. Inspect the repository structure and project instructions.
3. Inspect the current ticket/feature model, state machine, storage, CLI/TUI,
   orchestration, tests, and `.thanos` feature graph.
4. Read related memory from `.thanos/memory/feature-graph.json`,
   `.thanos/memory/feature-graph.md`, and the ticket artifact directory.
5. Write an implementation plan ordered by dependency.

Do not invent files, entities, workflow transitions, or commands that the
repository does not support. If the ticket requires extending those systems,
make that extension an explicit subtask with migration and regression coverage.

Read [references/workflow-contract.md](references/workflow-contract.md) when
planning data-model, state, UI, evidence, or acceptance-scenario work.

## Map Workflow Roles to Thanos

Use the repository's native roles unless the ticket explicitly adds new ones:

| Workflow role | Thanos role |
| --- | --- |
| QA Analyst | `planner` |
| Developer | `coder` |
| Reviewer | `reviewer` |
| Tester | `tester` |

Keep role outputs isolated in their expected `.thanos` artifacts. A role must
not approve its own work or perform a later role's gate.

## 1. QA Analyst: Plan the Ticket

Analyze scope, affected modules, dependencies, risks, edge cases, and unclear
requirements. Create ordered subtasks or execution chunks.

Each item must record:

- title and objective
- acceptance criteria
- dependencies
- priority and complexity
- suggested owner role
- affected modules or files, when known
- risks and edge cases

Save the analysis and plan before development. Create at least one subtask even
for small changes. The QA Analyst must not write production code.

Gate: do not start development without a saved plan and acceptance criteria.

## 2. Developer: Implement One Ready Item

Read the parent ticket, QA plan, selected item, acceptance criteria, and related
feature-graph context. Inspect existing code before editing.

Implement using current project conventions. Add or update the smallest
relevant automated test, even for a one-file change. Run applicable unit tests,
lint, type checks, and build commands from project settings.

Record implementation evidence:

- changed files
- behavior summary
- commands run
- results
- known limitations or deferred work

Submit the item for independent review. Do not mark it done.

Gate: no review submission without implementation notes and objective test
evidence.

## 3. Reviewer: Review Independently

The reviewer must be different from the developer. Review the parent ticket,
item, implementation evidence, changed files, and tests.

Check:

- correctness and acceptance coverage
- regressions and edge cases
- security and performance
- maintainability and architecture fit
- missing or weak tests

Classify comments as `Blocker`, `Major`, `Minor`, or `Suggestion`. Record the
reviewer, decision, comments, reviewed revision, and timestamp.

- Request changes when any blocking issue remains.
- Approve only when the implementation can enter testing.

Gate: testing cannot start without an explicit approval decision.

## 4. Tester: Create ECs and Run Smoke Tests

Create ECs mapped to acceptance criteria before executing tests. Each EC must
include:

- EC ID and title
- preconditions
- steps
- expected and actual results
- status: `Passed`, `Failed`, `Blocked`, or `Not Run`
- linked task or acceptance criterion
- evidence

Run focused tests for the changed behavior and smoke tests for adjacent critical
flows.

If an EC fails:

1. Record evidence.
2. Create or link a bug.
3. Reopen development using a valid repository transition.
4. Do not mark the task or parent ticket done.

If every required EC and smoke test passes, mark the item done.

## 5. Complete the Parent and Update Memory

Complete the parent only when every active child item has passed review and
testing.

When behavior changes, update the feature artifact and `.thanos` memory with:

- feature area and related node
- affected modules and paths
- dependencies and related tickets
- business rules and decisions
- implementation and review history
- EC, test, and smoke-test history
- known risks and related bugs

Use existing feature-graph APIs or project commands where available. Rebuild or
sync the graph and verify both JSON and Markdown summaries remain valid.

## Required Quality Gates

Reject or stop the workflow when:

- the plan has no subtasks or acceptance criteria
- implementation notes or test evidence are missing
- the developer attempts to approve their own work
- review has no explicit decision
- testing has no mapped ECs
- testing begins before approval
- a failed EC has no bug link or reopen action
- a task is marked done before testing passes
- the parent is marked done while an active child remains incomplete
- changed behavior is not recorded in feature-graph memory

## Evidence Standard

Prefer reproducible evidence: exact commands, exit status, concise output,
revision identifiers, artifact paths, and timestamps. Never claim a check
passed if it was not run. Record blocked checks and their reason.

## Acceptance Scenario

For `Add password length validation`, the completed workflow must demonstrate:

1. QA creates subtasks for current validation discovery, backend and frontend
   rules, API error behavior, unit tests, and smoke ECs.
2. Development implements one ready item with unit tests and evidence.
3. An independent reviewer approves or requests changes.
4. Testing executes mapped ECs and adjacent smoke coverage.
5. A failed EC creates or links a bug and reopens development.
6. The parent completes only after all active items pass.
7. `.thanos` memory records the resulting password validation rule.
