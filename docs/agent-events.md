# Agent Events

This document defines the desktop UI event boundary for Thanos.

## Current V1 Behavior

The desktop app uses command-response execution:

```text
Tauri UI -> run_thanos(...) -> Thanos CLI -> stdout/stderr -> Tauri UI
```

The CLI owns workflow state and writes all durable artifacts under `.thanos/`.

Important artifacts:

- `.thanos/tasks/{task-id}.yaml`
- `.thanos/plans/{task-id}.md`
- `.thanos/logs/{task-id}.log`
- `.thanos/reviews/{task-id}-diff.md`
- `.thanos/tests/{task-id}.md`
- `.thanos/worktrees/{task-id}`
- `.thanos/plan-graph/features/{feature-name}.md`

The desktop UI displays command output but does not interpret agent state as authority.

## Event Principles

Any future event stream should follow these rules:

- Events are emitted by the CLI or a CLI-owned helper process.
- Events are append-only observations, not workflow authority.
- The UI may render events, but transitions still happen through CLI commands.
- Events must reference task IDs and artifact paths under `.thanos/`.
- Events must not contain merge instructions or bypass review gates.

## Proposed Event Shape

When the CLI exposes machine-readable events, use JSON Lines:

```json
{"type":"task.status","task_id":"T001-example","status":"Plan","message":"Plan generated"}
{"type":"agent.command.started","task_id":"T001-example","command":"codex exec --full-auto -","workdir":".thanos/worktrees/T001-example"}
{"type":"agent.command.finished","task_id":"T001-example","exit_code":0,"duration_ms":1250}
{"type":"review.gate","task_id":"T001-example","actions":["approve","request-changes","rerun-agent","reopen-plan"]}
{"type":"test.result","task_id":"T001-example","passed":true,"path":".thanos/tests/T001-example.md"}
```

## Suggested Event Types

- `task.created`
- `task.status`
- `task.plan.updated`
- `agent.command.started`
- `agent.output`
- `agent.command.finished`
- `review.gate`
- `review.approved`
- `review.changes_requested`
- `test.result`
- `task.done`
- `task.reopened`

## Desktop Handling

The desktop UI may use events to:

- update the visible active step;
- show running command blocks;
- show completed checks;
- show failed checks;
- refresh the review screen.

The desktop UI must still call the CLI for:

- creating tasks;
- splitting tasks;
- planning;
- running Dev;
- review actions;
- tests;
- Done;
- reopen.
