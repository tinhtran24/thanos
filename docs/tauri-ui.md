# Thanos Desktop Workbench

`apps/thanos-desktop` is the local desktop shell for the new Thanos workbench.
It is no longer designed as a thin UI around `thanos task ...` commands.

## Phase 1 Scope

The current desktop app provides the product shell:

- left navigation for Projects, Board, Workbench, Memory, Agents, and Settings
- top project/worktree/agent status bar
- Kanban board
- selected task workbench
- plan, file, diff, terminal, browser preview, and git changes panels
- right task inspector with plan, messages, and related memory
- bottom Chat, Terminal, Timeline, and Logs tabs

The screen uses local mock data until Phase 2+ services are wired.

## Architecture Boundary

The desktop UI will call local Thanos backend services directly through Tauri
commands and realtime event subscriptions. Native agent CLIs such as Codex,
Claude Code, Gemini CLI, and OpenCode are launched in managed terminals or
ACP/MCP adapters, not hidden behind the legacy CLI workflow.

Hard rules:

- no agent modifies the main branch directly
- every task gets an isolated branch/worktree before execution
- plan approval is required before coder start
- final approval requires review and tests
- the user can inspect changes before merge
- agent conversations are captured
- important decisions are stored in memory

## Upcoming Tauri Commands

```text
open_project(path)
list_projects()
list_tasks(project_id)
create_task(input)
approve_plan(task_id)
request_changes(task_id, notes)
start_agent_session(task_id, profile_name)
stop_agent_session(session_id)
run_tests(task_id)
list_reviews(task_id)
approve_merge(task_id)
subscribe_events()
```

Phase 1 only establishes the UI and backend domain foundation. Worktree
creation, PTY sessions, and review diff plumbing come next.

Phase 2 adds native backend commands:

```text
prepare_task_worktree(request)
start_agent_session(request)
stop_agent_session()
resume_agent_session(workspace, session_id)
```

`start_agent_session` launches the configured native CLI inside the task
worktree, streams `agent-session-output` events, emits `agent-session-exit`,
and records conversation output under `.thanos/logs/sessions/`.

Phase 3 adds planner/coder workflow commands:

```text
save_execution_plan(request)
read_execution_plan(request)
approve_execution_plan(request)
```

Plans are saved to `.thanos/plans/{task-id}.json` and
`.thanos/plans/{task-id}.md`. Approval emits `plan.approved`. Coder sessions are
rejected unless the persisted plan has `approval_status: approved`.

Phase 4 review commands:

```text
collect_git_diff(request)
run_task_tests(request)
save_review(request)
approve_review(request)
```

Review approval requires a passing test summary and never auto-merges.

Phase 5 memory commands:

```text
write_memory_node(request)
search_memory(request)
```

Memory uses `.thanos/memory/workbench.sqlite` with FTS5 search.

## Frontend Flow Pattern

Route files only mount screen-level flows. The current entrypoint mounts
`WorkbenchRoute`, which composes:

- `BoardFlow` for board filtering, column state, drag/drop, task creation, and
  task selection.
- `TaskWorkbenchFlow` for the selected task, tabs, approval actions, tests, git
  changes, files, terminal, and chat.
- `AgentSessionFlow` for native CLI session lifecycle and output streaming.

Domain state is typed in `src/domain/models.ts`. Task lifecycle changes must go
through `src/state/taskMachine.ts`. Presentational components under
`src/shared/ui` receive typed props and do not call APIs or mutate domain state.
