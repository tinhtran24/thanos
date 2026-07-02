# Thanos

Thanos is being rebuilt as a local-first AI development workbench for planning,
running, reviewing, and approving AI coding tasks across multiple agent CLIs.

The human stays in control. Plans, code execution, review, tests, and merge
approval are explicit gates. Agents can work in parallel only through isolated
git worktrees.

## Product Shape

- Desktop-first workbench with an optional web UI.
- Kanban board for features, tasks, subtasks, priority, agent assignment, and
  approval state.
- Task workbench with plan, current step, files, diff, terminal, browser
  preview, git changes, chat, timeline, and logs.
- Native agent CLI passthrough for Codex, Claude Code, Gemini CLI, OpenCode, and
  custom commands.
- Local SQLite state with FTS memory.
- Git remains the source of truth for code changes.
- WebSocket events power realtime UI updates.

## Workflow

```text
Backlog
  -> Planning
  -> Waiting Approval
  -> Ready
  -> Running
  -> In Review
  -> Done
```

Rules:

- Planner writes an execution plan.
- User approves the plan before coder starts.
- Coder runs only in an isolated task worktree and branch.
- Reviewer inspects diff and test results.
- User approves merge or requests changes.
- Done requires approved review and passing tests.
- No auto-merge.

## Phase 1 Status

Implemented:

- Phase 1 workbench data model in `internal/workbench`.
- SQLite schema with project, repo, feature, task, plan, agent session, review,
  memory, FTS, and event tables.
- Kanban task grouping.
- Approval-gated workbench task transition validation in `internal/orchestrator`.
- Realtime event hub and WebSocket broadcast endpoint in `internal/events`.
- Desktop workbench shell in `apps/thanos-desktop`.
- Phase 2 worktree guard and PTY-backed agent session commands.
- Phase 3 persisted execution plans, plan approval gate, and coder launch guard.
- Phase 4 git diff, test runner, review approval gate, and no-auto-merge review plumbing.
- Phase 5 SQLite memory nodes with FTS search and approved-review memory updates.

Not implemented yet:

- MCP/ACP bridge.

See [docs/rebuild-plan.md](docs/rebuild-plan.md) for the migration plan.

## Development

Run Go tests:

```sh
go test ./...
```

Run the desktop UI:

```sh
cd apps/thanos-desktop
npm install
npm run dev
```

Build the desktop frontend:

```sh
cd apps/thanos-desktop
npm run build
```

## Legacy CLI Compatibility

The existing `cmd/thanos` CLI remains during the rebuild. Its `init` command
still writes `project.framework` into `.thanos/settings.json` and supports
`--framework` plus `--language` overrides.

Framework detection is read-only and network-free. It does not run a package manager
or project command. If root evidence is ambiguous, the framework is
omitted. Supported detected framework values are final canonical strings:
`wordpress`, `laravel`, `nextjs`, `nestjs`, `angular`, `nuxt`, `gin`, `echo`,
`django`, `flask`, `fastapi`, `actix-web`, `axum`, and `rocket`.

Detection evidence includes `composer.json`, `artisan`, `bootstrap/app.php`,
`wp-admin`, `wp-includes`, `wp-content`, `package.json`, `go.mod`,
`pyproject.toml`, `requirements*.txt`, and `Cargo.toml`.
