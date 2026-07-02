# Thanos Rebuild Migration Plan

## Direction

Thanos is being rebuilt as a local-first AI development workbench. The desktop
and web surfaces are the primary execution UX. The CLI becomes a thin launcher
and compatibility surface, not the owner of the product workflow.

Inspired product patterns:

- Kandev: Kanban-driven multi-agent work, worktree isolation, integrated review
  workspace, raw CLI passthrough.
- WRAI.TH: persistent memory, task graph, inter-agent coordination, strict state
  transitions, MCP-style tools.

No implementation code is copied from either project.

## Current Repository Analysis

The current repo is a Go terminal-first workflow CLI with a Tauri shell:

- `cmd/thanos` starts the CLI.
- `internal/cli` owns command parsing and many workflow routes.
- `internal/orchestrator`, `internal/state`, `internal/model`, and
  `internal/workspace` implement the legacy feature workflow and `.thanos`
  artifact layout.
- `internal/taskworkflow` contains a smaller task lifecycle:
  `backlog -> plan -> execute -> verify -> done`.
- `apps/thanos-desktop` was a Tauri UI that invoked the CLI and read `.thanos`
  artifacts.
- `.thanos/memory` and feature graph files exist, but memory is not yet a
  SQLite/FTS workbench subsystem.

The main gap is product ownership: the old CLI owns workflow execution UX, while
the rebuild needs a desktop/web workbench backed by local services, event
streams, SQLite state, git worktrees, and native agent terminals.

## Target Structure

```text
apps/
  thanos-desktop/       Tauri shell, React/TS workbench UI
  thanos-web/           optional browser UI
  thanos-cli/           thin launcher

internal/
  orchestrator/         state machine, approval gates, event dispatch
  board/                kanban state and task tree
  agents/               profiles and provider adapters
  executor/             process/docker/ssh runtime profiles
  workspace/            repo, branch, and worktree management
  memory/               SQLite + FTS project memory
  review/               diff, tests, approve/reject flow
  terminal/             PTY sessions and conversation capture
  mcp/                  task/memory tools exposed to agents
  events/               realtime event hub and WebSocket gateway
  workbench/            Phase 1 domain models and schema
```

## Phases

### Phase 1: Core Rewrite Foundation

- Add workbench domain models for project, feature, task, execution plan, agent
  session, review, and memory.
- Add SQLite schema, including memory FTS and compact event storage.
- Add Kanban grouping and approval-gated task state validation.
- Add realtime event hub with WebSocket broadcast endpoint.
- Replace the desktop CLI-control screen with the workbench layout: board, task
  workbench, right inspector, bottom chat/terminal/timeline/logs panel.

Out of scope:

- Creating real git worktrees.
- Running native agent CLIs.
- Capturing PTY sessions.
- Applying or merging diffs.
- MCP/ACP tools.

### Phase 2: Worktree + Terminal

- Implement worktree manager and branch guard.
- Implement PTY session manager.
- Start, stop, capture, and resume native agent CLI sessions.
- Store conversation logs per `AgentSession`.

Status:

- Added Go worktree manager command construction and protected branch guard.
- Added Tauri commands for task worktree preparation.
- Added Tauri PTY-backed agent session start/stop/resume.
- Agent session output is streamed through Tauri events and appended to
  `.thanos/logs/sessions/{session-id}.log`.
- Agent session metadata is persisted to
  `.thanos/logs/sessions/{session-id}.json`.
- Frontend `AgentSessionFlow` now prepares worktrees and starts/stops native
  sessions through the backend adapter.

### Phase 3: Planner/Coder Workflow

- Persist generated `ExecutionPlan`.
- Require plan approval before coder launch.
- Start coder only in task worktree.
- Emit workflow events for every stage.

Status:

- Added Tauri commands to save, read, and approve execution plans.
- Execution plans are persisted as JSON and Markdown under `.thanos/plans/`.
- Plan save and approval emit workflow events to Tauri and
  `.thanos/events/workflow.jsonl`.
- Planner sessions can run in the project root.
- Coder sessions are blocked until the persisted plan is approved.
- Coder sessions still require an existing isolated task worktree.
- Frontend approval flow saves the current plan, approves it, updates local
  state, and only then allows coder start.

### Phase 4: Review System

- Parse git diff by repo and file.
- Collect test command results.
- Gate merge on review approval and passing tests.
- Keep no-auto-merge as a hard rule.

Status:

- Added Tauri command to collect git diff summary, changed files, and patch
  from a task worktree.
- Added Tauri command to run a configured test command in the task worktree.
- Test summaries are persisted under `.thanos/tests/`.
- Reviews are saved under `.thanos/reviews/`.
- Review approval is blocked until the latest test summary has passed.
- Approval emits workflow events and still does not auto-merge.

### Phase 5: Memory + Plan Graph

- Store decisions, touched files, conventions, tasks, and feature graph nodes in
  SQLite.
- Add FTS search and related-memory retrieval.
- Update memory after approved work only.

Status:

- Added local SQLite memory DB at `.thanos/memory/workbench.sqlite`.
- Added `memory_nodes` table and FTS5 index.
- Added memory write and search Tauri commands.
- Approved reviews write task memory with changed file links.

### Phase 6: MCP/ACP Bridge

- Expose task and memory tools to agents.
- Allow agents to create subtasks, message sibling tasks, inspect related work,
  attach branches, and request user review.

## Phase 1 Acceptance

- `internal/workbench` contains domain models and a SQLite schema.
- `internal/orchestrator` validates the new workbench task state transitions.
- `internal/events` can broadcast compact JSON events over WebSocket.
- Desktop UI shows the required workbench regions without relying on CLI command
  execution.
- Tests cover board grouping, event hub publishing, and approval gates.
