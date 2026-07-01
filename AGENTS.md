# AGENT.md

## Role

You are the implementation agent for Thanos.

Thanos is a Go terminal-first AI workflow CLI.

Your job is to read the project structure, follow the task plan, make scoped changes, and preserve workflow rules.

---

## Core Rule

The CLI is the source of truth.

All durable workflow data must live under `.thanos/`.

Do not move workflow ownership into Tauri, UI code, or external services.

---

## Current Workflows

Thanos has two workflow layers.

Do not confuse them.

---

## 1. Legacy Feature Workflow

Used by existing feature commands.

Core model:

* `model.Feature`
* `model.State`
* `model.ExecutionPlan`

Core files:

```text
internal/model/model.go
internal/state/machine.go
internal/orchestrator/orchestrator.go
internal/workspace/workspace.go
```

Flow:

```text
init -> planning -> coding -> reviewing -> testing -> overview/done
```

Commands:

```text
thanos new
thanos run
thanos done
thanos status
thanos plan
thanos transition
```

Artifacts:

```text
.thanos/{feature-id}/
```

Do not modify this workflow unless the task explicitly asks for it.

---

## 2. New Task Workflow

Used by review-first task commands.

Core model:

* `model.Task`
* `model.AgentProfile`

Core files:

```text
internal/taskworkflow/workflow.go
internal/cli/cli.go
docs/workflows.md
```

Current flow:

```text
Backlog -> Analysis -> Plan -> Dev -> Review -> Test -> Done
```

Target simplified flow:

```text
Backlog -> Plan -> Execute -> Verify -> Done
```

Artifacts:

```text
.thanos/tasks/
.thanos/plans/
.thanos/logs/
.thanos/reviews/
.thanos/tests/
.thanos/worktrees/
```

Rules:

* `Plan` includes analysis, affected files, subtasks, risks, and test strategy.
* `Execute` includes coding, refactoring, command execution, and implementation logs.
* `Verify` includes review, unit tests, lint, typecheck, and approval gate.
* `Done` requires:

  * `review_approved = true`
  * `tests_passed = true`
* Review is a hard pause.
* No auto-merge.
* Dev/Execute must read `.thanos/plans/{task-id}.md`.
* Do not regenerate analysis after the plan exists.

---

## Target Project Structure

Use this structure as the preferred direction.

```text
thanos/
├── cmd/
│   └── thanos/
│       └── main.go
│
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── task.go
│   │   ├── feature.go
│   │   ├── render.go
│   │   ├── help.go
│   │   └── errors.go
│   │
│   ├── model/
│   │   ├── task.go
│   │   ├── feature.go
│   │   └── artifact.go
│   │
│   ├── taskworkflow/
│   │   ├── workflow.go
│   │   ├── transition.go
│   │   └── guard.go
│   │
│   ├── featureworkflow/
│   │   ├── machine.go
│   │   └── orchestrator.go
│   │
│   ├── workspace/
│   │   ├── workspace.go
│   │   ├── repository.go
│   │   └── lock.go
│   │
│   ├── runner/
│   │   ├── command.go
│   │   ├── git.go
│   │   ├── test.go
│   │   └── worktree.go
│   │
│   ├── agent/
│   │   ├── agent.go
│   │   ├── planner.go
│   │   ├── executor.go
│   │   └── reviewer.go
│   │
│   ├── events/
│   │   ├── event.go
│   │   ├── emitter.go
│   │   └── subscriber.go
│   │
│   └── config/
│       ├── config.go
│       └── defaults.go
│
├── apps/
│   └── thanos-desktop/
│
├── docs/
│   ├── workflows.md
│   ├── cli.md
│   └── desktop.md
│
└── .thanos/
    ├── tasks/
    ├── plans/
    ├── logs/
    ├── reviews/
    ├── tests/
    └── worktrees/
```

---

## Package Rules

### `cmd/`

Only starts the app.

Do not put business logic here.

---

### `internal/cli/`

Owns command parsing and command routing.

Allowed:

* parse args
* call workflow services
* print human output
* print JSON output
* return errors

Not allowed:

* duplicate workflow rules
* directly own persistence rules
* contain large prompt logic
* mix feature and task workflow logic in the same function

---

### `internal/taskworkflow/`

Owns task lifecycle rules.

This package answers:

```text
Is this task transition valid?
```

It should own:

* status graph
* transition validation
* guards
* Done requirements

---

### `internal/workspace/`

Owns `.thanos/` paths and persistence.

This package answers:

```text
Where is this artifact stored?
```

All `.thanos/` paths should come from this package.

Avoid hardcoded `.thanos/...` paths outside workspace code.

---

### `internal/runner/`

Owns command execution.

This includes:

* shell commands
* git commands
* worktree commands
* tests
* lint
* typecheck

---

### `internal/agent/`

Owns AI-related execution.

This includes:

* planner
* executor
* reviewer
* prompt construction

Agent code must consume saved artifacts instead of rebuilding context.

---

### `internal/events/`

Owns structured workflow events.

Events are used by:

* CLI
* TUI
* Tauri
* scripts

Use compact JSON events.

Example:

```json
{
  "task_id": "TASK-123",
  "event": "stage.completed",
  "stage": "plan",
  "status": "planned",
  "artifact": ".thanos/plans/TASK-123.md"
}
```

---

### `apps/thanos-desktop/`

Tauri desktop UI only.

It must call CLI commands.

It must not directly import or duplicate workflow logic.

Allowed:

```text
thanos task list --json
thanos task show {id} --json
thanos task plan {id}
thanos task execute {id}
thanos task verify {id}
thanos task done {id}
```

Not allowed:

```text
desktop -> taskworkflow directly
desktop -> workspace directly
desktop -> custom workflow state machine
```

---

## Dependency Direction

Follow this direction:

```text
cmd
 ↓
cli
 ↓
taskworkflow / featureworkflow
 ↓
workspace / runner / agent / events
 ↓
model / config
```

Forbidden imports:

```text
taskworkflow -> cli
workspace -> cli
runner -> cli
agent -> cli
events -> cli
desktop -> internal/taskworkflow
desktop -> internal/workspace
```

---

## Token Optimization Rules

Minimize context and avoid repeated analysis.

### Rule 1

Plan once.

Do not regenerate a task plan if `.thanos/plans/{task-id}.md` already exists.

---

### Rule 2

Each stage reads only the minimum required artifacts.

```text
Plan:
- reads ticket only

Execute:
- reads saved plan only

Verify:
- reads saved plan
- reads execution summary only

Done:
- reads status flags only
```

---

### Rule 3

Never resend the full ticket after planning.

---

### Rule 4

Never load the whole repository.

Load only affected files.

---

### Rule 5

Avoid loading large files unless directly editing them.

Avoid by default:

```text
internal/tui/**
internal/prompts/**
internal/codegraph/**
internal/featuregraph/**
apps/thanos-desktop/**
```

---

### Rule 6

Prefer compact machine-readable files for state.

Use Markdown only for human-readable artifacts:

```text
plans
logs
reviews
tests
```

Use JSON for machine state:

```text
tasks
status
events
```

---

## CLI Output Rules

Every task command should support:

```text
--json
```

Human output is for terminal users.

JSON output is for:

* Tauri
* scripts
* tests
* integrations

Do not break existing human output unless the task explicitly asks for it.

---

## Refactor Priorities

Follow this order unless the task says otherwise.

### 1. Split CLI

Refactor `internal/cli/cli.go` into:

```text
internal/cli/root.go
internal/cli/task.go
internal/cli/feature.go
internal/cli/render.go
internal/cli/agent.go
internal/cli/help.go
internal/cli/errors.go
```

Goal:

* reduce token cost
* isolate task workflow
* isolate legacy feature workflow
* make future edits smaller

---

### 2. Split Models

Refactor `internal/model/model.go` into:

```text
internal/model/task.go
internal/model/feature.go
internal/model/artifact.go
```

Do not change JSON compatibility unless required.

---

### 3. Simplify Task Workflow

Move from:

```text
Backlog -> Analysis -> Plan -> Dev -> Review -> Test -> Done
```

to:

```text
Backlog -> Plan -> Execute -> Verify -> Done
```

Keep review and tests, but merge them into `Verify`.

---

### 4. Centralize Workspace Paths

Move all `.thanos/` path construction into `internal/workspace`.

---

### 5. Add JSON Output

Ensure task commands can output stable JSON.

---

### 6. Add Events

Emit compact workflow events for task transitions.

---

## Implementation Rules

Before editing:

1. Read the task.
2. Identify which workflow it affects.
3. Load only relevant files.
4. Avoid touching unrelated packages.
5. Preserve existing behavior unless explicitly changing it.

During editing:

1. Keep commits/patches small.
2. Avoid broad rewrites.
3. Prefer extracting functions over changing logic.
4. Preserve command compatibility.
5. Preserve artifact compatibility where possible.

After editing:

1. Run `gofmt`.
2. Run tests if available.
3. Run relevant CLI command manually if possible.
4. Update docs only when behavior changes.

---

## Task Plan Format

When creating a plan, use this format:

```markdown
# Task Plan: {title}

## Summary

One short paragraph.

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Affected Files

- `path/to/file.go` — reason

## Implementation Steps

- [ ] Step 1
- [ ] Step 2

## Test Strategy

- command
- expected result

## Risks

- risk
- mitigation
```

---

## Execute Summary Format

When execution finishes, write:

````markdown
# Execution Summary: {task-id}

## Completed

- item

## Changed Files

- `path/to/file.go`

## Commands Run

```text
go test ./...
````

## Notes

Short notes only.

````

---

## Review Format

When verifying, write:

```markdown
# Review: {task-id}

## Result

approved | changes_requested

## Findings

- finding

## Risk Check

- risk

## Approval

review_approved: true | false
````

---

## Test Summary Format

When tests run, write:

````markdown
# Test Summary: {task-id}

## Result

passed | failed

## Commands

```text
go test ./...
````

## Failures

* failure

## Status

tests_passed: true | false

````

---

## Done Guard

A task can move to Done only when:

```text
review_approved == true
tests_passed == true
````

Never bypass this guard.

---

## Final Rule

When unsure, prefer the smaller change.

Do not create new architecture unless it supports:

* fewer tokens
* fewer loops
* clearer workflow state
* better separation between CLI, workflow, workspace, runner, and UI
