# Thanos Token-Efficient Task Workflow

Thanos supports a terminal-first task workflow for local projects. The CLI owns workflow state and all durable data lives under `.thanos/`.

Desktop applications are presentation layers only. They call Thanos CLI commands and must not duplicate workflow logic.

## Workflow

Tasks move through:

```text
backlog -> plan -> execute -> verify -> done
```

The workflow is optimized to avoid repeated AI context:

- `plan` reads the ticket once and writes a reusable plan.
- `execute` reads only the saved plan.
- `verify` reads only the saved plan and execution summary.
- `done` reads only status flags.

`verify` is the human approval gate. Thanos never auto-merges.

## Files

- `.thanos/tasks/{task-id}.json` stores task metadata and status flags.
- `.thanos/plans/{task-id}.md` stores the reusable plan.
- `.thanos/logs/{task-id}.md` stores execution output and changed files.
- `.thanos/reviews/{task-id}.md` stores review evidence.
- `.thanos/tests/{task-id}.md` stores test command output and verdict.
- `.thanos/worktrees/{task-id}` stores the isolated execution worktree.
- `.thanos/agents.yaml` stores agent profiles.
- `.thanos/plan-graph/features/{feature-name}.md` optionally stores reusable planning memory.

## Commands

Create and inspect work:

```sh
thanos task create "Add login audit log" --description "Record successful and failed login attempts" --priority high
thanos board
thanos task list --json
thanos task show T001-add-login-audit-log --json
```

Split a larger task into reviewable subtasks:

```sh
thanos task split T001-add-login-audit-log
```

Plan once:

```sh
thanos task plan T001-add-login-audit-log
```

Planning writes `.thanos/plans/{task-id}.md` with a requirement summary, acceptance criteria, affected modules, risks, task checklist, and test strategy. If the plan already exists, Thanos reuses it instead of regenerating analysis.

Execute from the saved plan:

```sh
thanos task execute T001-add-login-audit-log
```

Thanos creates `.thanos/worktrees/{task-id}` on branch `thanos/{task-id}-{slug}`, sends only the saved plan to the configured agent profile, writes an execution summary, and stops at `verify`.

Verify and approve:

```sh
thanos task verify T001-add-login-audit-log
thanos task verify T001-add-login-audit-log approve
thanos task verify T001-add-login-audit-log request-changes
thanos task verify T001-add-login-audit-log rerun-agent
thanos task verify T001-add-login-audit-log reopen-plan
```

`verify` writes review and test artifacts. `done` is blocked until both `review_approved` and `tests_passed` are true:

```sh
thanos task done T001-add-login-audit-log
```

## Agents

Agent profiles are configured in `.thanos/agents.yaml`:

```yaml
agents:
  - name: codex
    command: codex
    args: ["exec", "--full-auto", "-"]
    env: {}
    role: implementation
    allowed_steps: [plan, execute]
  - name: custom
    command: ./scripts/local-agent
    args: []
    env:
      MODE: local
    role: implementation
    allowed_steps: [plan, execute, verify]
```

The profile format is vendor-neutral: `name`, `command`, `args`, `env`, `role`, and `allowed_steps`. A profile with an empty command is valid for local-only planning; Thanos writes prompts and artifacts without launching an external agent.

## Planning Memory

Planning may load matching files from `.thanos/plan-graph/features/` before writing a plan. Later stages do not reload the ticket or memory; they consume the saved plan and generated artifacts.
