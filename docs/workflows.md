# Thanos Review-First Task Workflow

Thanos supports a terminal-first task workflow for local projects. All task data and generated artifacts live under `.thanos/` so the project source tree remains the source of truth and task state stays portable.

## Workflow

Tasks move through:

```text
Backlog -> Analysis -> Plan -> Dev -> Review -> Test -> Done
```

`Review` is the required pause point after development. Thanos never auto-merges. `Done` is only allowed after review approval and a passing test result.

## Files

- `.thanos/tasks/{task-id}.yaml` stores task metadata.
- `.thanos/plans/{task-id}.md` stores the plan that Dev must read before editing.
- `.thanos/logs/{task-id}.log` stores analysis and agent logs.
- `.thanos/reviews/{task-id}-diff.md` stores changed files, diff summary, and risks.
- `.thanos/tests/{task-id}.md` stores test command output and verdict.
- `.thanos/worktrees/{task-id}` stores the isolated Dev worktree.
- `.thanos/agents.yaml` stores agent profiles.
- `.thanos/plan-graph/features/{feature-name}.md` stores persistent feature memory.

## Commands

Create and inspect work:

```sh
thanos task create "Add login audit log" --description "Record successful and failed login attempts" --priority high
thanos board
```

Split a larger task into reviewable subtasks:

```sh
thanos task split T001-add-login-audit-log
```

Plan before coding:

```sh
thanos task plan T001-add-login-audit-log
```

Planning writes `.thanos/plans/{task-id}.md` with a requirement summary, impacted files, implementation steps, risks, test strategy, rollback plan, and an optional Mermaid diagram. If related feature memory exists under `.thanos/plan-graph/features/`, Thanos includes it in the plan context.

Run Dev in an isolated worktree:

```sh
thanos task run T001-add-login-audit-log
```

Thanos creates `.thanos/worktrees/{task-id}` on branch `thanos/{task-id}-{slug}`, sends the approved plan to the configured agent profile, writes a diff summary, and stops at `Review`.

Review gate actions:

```sh
thanos task review T001-add-login-audit-log
thanos task review T001-add-login-audit-log approve
thanos task review T001-add-login-audit-log request-changes
thanos task review T001-add-login-audit-log rerun-agent
thanos task review T001-add-login-audit-log reopen-plan
```

Run tests and complete:

```sh
thanos task test T001-add-login-audit-log
thanos task done T001-add-login-audit-log
```

`task test` uses `project.test` from `.thanos/settings.json`. If no test command is configured, it falls back to `go test ./...` for local smoke coverage.

## Agents

Agent profiles are configured in `.thanos/agents.yaml`:

```yaml
agents:
  - name: codex
    command: codex
    args: ["exec", "--full-auto", "-"]
    env: {}
    role: implementation
    allowed_steps: [Analysis, Plan, Dev]
  - name: custom
    command: ./scripts/local-agent
    args: []
    env:
      MODE: local
    role: implementation
    allowed_steps: [Analysis, Plan, Dev, Test]
```

The profile format is vendor-neutral: `name`, `command`, `args`, `env`, `role`, and `allowed_steps`. A profile with an empty command is valid for local-only planning; Thanos writes prompts and artifacts without launching an external agent.

## Feature Memory

After `task done`, Thanos updates `.thanos/plan-graph/features/{feature-name}.md` with changed behavior, important files, decisions, test notes, and future risks. Later planning runs load matching feature memory before generating the next plan.
