# PLAN.md

## Objective

Refactor Thanos into a simpler, token-efficient workflow engine while preserving existing functionality.

The CLI remains the single source of truth.

Desktop applications (Tauri) are presentation layers only and must never implement workflow logic.

---

# Architecture Principles

## Source of Truth

* Workflow state is owned by the CLI.
* Persistent state is stored under `.thanos/`.
* Every command is deterministic.
* Every stage produces reusable artifacts.
* Never regenerate information that already exists.

---

# Workflow

Replace the current task workflow:

```
Backlog
→ Analysis
→ Plan
→ Dev
→ Review
→ Test
→ Done
```

with:

```
Backlog
→ Plan
→ Execute
→ Verify
→ Done
```

---

## Stage 1 — Backlog

Input

* ticket
* issue
* user request

Output

```
.thanos/tasks/{task-id}.json
```

Contains only metadata.

No code analysis.

No implementation.

---

## Stage 2 — Plan

Goal

Understand the task once.

Produce reusable implementation artifacts.

Generate

```
.thanos/plans/{task-id}.md
```

The plan should include:

* Requirement summary
* Acceptance criteria
* Affected modules
* Risks
* Task checklist
* Test strategy

Rules

* Read the ticket once.
* Never repeat analysis later.
* Future stages consume the saved plan.

---

## Stage 3 — Execute

Goal

Implement the plan.

Execute

* coding
* refactoring
* command execution
* migrations

Never re-analyze the ticket.

Read only

```
.thanos/plans/{task-id}.md
```

Generate

```
.thanos/logs/{task-id}.md
```

Include

* changed files
* completed checklist
* execution summary

---

## Stage 4 — Verify

Goal

Validate implementation.

Includes

* self review
* unit tests
* lint
* type checking

Generate

```
.thanos/reviews/{task-id}.md
```

```
.thanos/tests/{task-id}.md
```

Done is blocked unless

```
review_approved == true
tests_passed == true
```

Review is always a human approval gate.

Never auto merge.

---

## Stage 5 — Done

Requirements

* Review approved
* Tests passed

Update

```
.thanos/tasks/{task-id}.json
```

Status

```
done
```

---

# Artifact Ownership

Task

```
.thanos/tasks/
```

Planning

```
.thanos/plans/
```

Execution

```
.thanos/logs/
```

Reviews

```
.thanos/reviews/
```

Tests

```
.thanos/tests/
```

Worktrees

```
.thanos/worktrees/
```

---

# Token Optimization Rules

These rules are mandatory.

## Rule 1

Plan only once.

Never regenerate a plan.

---

## Rule 2

Each stage reads only the minimum required artifact.

Plan

reads

```
ticket
```

Execute

reads

```
plan
```

Verify

reads

```
plan
execution summary
```

Done

reads

```
status flags
```

---

## Rule 3

Never send the full ticket after planning.

---

## Rule 4

Never send the entire repository.

Load only affected modules.

---

## Rule 5

Large files must not be loaded unless directly modified.

Examples

Skip

```
internal/tui/**
internal/prompts/**
internal/codegraph/**
internal/featuregraph/**
```

---

## Rule 6

Avoid duplicate context.

Reference existing artifacts instead.

---

# CLI Responsibilities

The CLI owns

* workflow transitions
* persistence
* artifact generation
* validation
* execution

Desktop UI must only

* render status
* display logs
* show Kanban board
* invoke CLI commands

No business logic.

---

# Definition of Done

A task is complete when

* implementation finished
* review approved
* tests passed
* artifacts generated
* workflow status updated

No additional analysis should be required after completion.

---

# Future Refactoring

Split responsibilities into small packages.

```
internal/cli/
    root.go
    task.go
    feature.go
    render.go
    help.go

internal/taskworkflow/

internal/workspace/

internal/events/

internal/runner/
```

Each package should have one responsibility only.

---

# Success Criteria

The workflow should:

* minimize AI token usage
* avoid repeated analysis
* avoid workflow loops
* reuse artifacts instead of regenerating context
* keep CLI as the only workflow engine
* support both terminal and Tauri desktop without duplicating logic
