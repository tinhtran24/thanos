# AGENTS.md

## Role

You are the implementation agent for Thanos.

Thanos is being rebuilt as a local-first AI development workbench. The product
is inspired by AI Kanban, native agent terminals, task graphs, memory, review
gates, and MCP/ACP-style coordination. Do not copy implementation code from
external repositories.

## Product Principles

1. The human stays in control.
2. Important steps require review or approval.
3. Agents work in parallel only through isolated git worktrees.
4. Planner, Coder, Reviewer, and Tester are separate roles.
5. The user can chat at any time.
6. The user can inspect plan, files, terminal, browser preview, git diff, and
   task state in one workspace.
7. Native agent CLIs are first-class. Do not hide Codex, Claude Code, Gemini
   CLI, OpenCode, or custom commands behind a heavy CLI workflow.

## New Architecture

```text
apps/
  thanos-desktop/   Tauri desktop shell and workbench UI
  thanos-web/       optional browser UI
  thanos-cli/       thin launcher only

internal/
  orchestrator/     workflow engine, transitions, gates, event dispatch
  board/            kanban columns and task tree
  agents/           provider profiles and adapters
  executor/         process/docker/ssh runtime profiles
  workspace/        repo, branch, worktree management
  memory/           SQLite + FTS project memory
  review/           diff parser, tests, approve/reject flow
  terminal/         PTY sessions and conversation capture
  mcp/              tools exposed to agents
  events/           compact JSON events and WebSocket streams
  workbench/        Phase 1 domain models and SQLite schema
```

## Workflow Rules

Task statuses:

```text
backlog -> planning -> waiting_approval -> ready -> running -> in_review -> done
```

Allowed side paths:

```text
blocked
failed
request changes -> ready or planning
```

Hard gates:

- `ready` from `waiting_approval` requires plan approval.
- `running` requires an isolated worktree and branch.
- `done` requires `review_approved = true` and `tests_passed = true`.
- Review is a hard pause.
- No auto-merge.
- Never let an agent modify main directly.
- Always capture agent conversation.
- Always show changed files and diff before merge.
- Always store important decisions into memory.

## Phase Discipline

Implement only the requested phase.

Current phase:

- Phase 1: domain model, SQLite schema, board/workbench UI shell, validated
  transitions, realtime events.

Do not implement Phase 2+ behavior unless explicitly requested:

- worktree creation
- PTY terminal manager
- real agent start/stop
- diff parser
- merge flow
- MCP/ACP bridge

## Engineering Rules

- Keep modules small.
- No hardcoded providers.
- All agent commands are configurable.
- All state transitions are validated in Go.
- Add tests for orchestrator state machine changes.
- Add mock executors before testing agent flows.
- Use event-driven architecture.
- Prefer simple implementation first.
- E2E: Playwright.

## Data

Use SQLite for local workbench state. Use Git as the source of truth for code
changes. Use compact JSON for events and machine state. Use Markdown only for
human-readable plans, logs, reviews, and test summaries.


### Frontend architecture update:

Use Flow Component Design Pattern.

Core idea:
- Screens only compose flows.
- Flows own business interaction.
- Components stay dumb/presentational.
- State machine controls task lifecycle.
- API hooks are separated from UI.

Structure:

src/
  app/
    routes/
    providers/
    shell/

  flows/
    board-flow/
      BoardFlow.tsx
      BoardToolbar.tsx
      BoardColumnFlow.tsx
      TaskCardFlow.tsx
      useBoardFlow.ts
      board-flow.machine.ts

    task-workbench-flow/
      TaskWorkbenchFlow.tsx
      TaskHeaderFlow.tsx
      PlanReviewFlow.tsx
      AgentAssignmentFlow.tsx
      FilesFlow.tsx
      TerminalFlow.tsx
      ChatFlow.tsx
      GitChangesFlow.tsx
      useTaskWorkbenchFlow.ts
      task-workbench.machine.ts

    agent-session-flow/
      AgentSessionFlow.tsx
      AgentTerminalFlow.tsx
      AgentMessageFlow.tsx
      useAgentSessionFlow.ts

  features/
    tasks/
      api/
      model/
      hooks/
      components/

    agents/
      api/
      model/
      hooks/
      components/

    memory/
      api/
      model/
      hooks/
      components/

    workspace/
      api/
      model/
      hooks/
      components/

  shared/
    ui/
      Button.tsx
      Card.tsx
      Tabs.tsx
      Badge.tsx
      Dialog.tsx
      ScrollArea.tsx
      SplitPane.tsx
      CommandInput.tsx

    layout/
      AppShell.tsx
      LeftSidebar.tsx
      RightSidebar.tsx
      BottomPanel.tsx

    lib/
      cn.ts
      event-bus.ts
      websocket.ts
      shortcuts.ts
