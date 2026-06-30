# Best Practice Project Structure

```text
thanos/
├── cmd/
│   └── thanos/
│       └── main.go
│
├── internal/
│   ├── cli/
│   │   ├── root.go              # root command wiring
│   │   ├── task.go              # thanos task commands
│   │   ├── feature.go           # legacy feature commands
│   │   ├── render.go            # terminal output rendering
│   │   ├── help.go              # help text and examples
│   │   └── errors.go            # CLI error formatting
│   │
│   ├── model/
│   │   ├── task.go              # Task, TaskStatus, AgentProfile
│   │   ├── feature.go           # legacy Feature, State, ExecutionPlan
│   │   └── artifact.go          # shared artifact references
│   │
│   ├── taskworkflow/
│   │   ├── workflow.go          # Backlog -> Plan -> Execute -> Verify -> Done
│   │   ├── transition.go        # transition validation
│   │   └── guard.go             # review/test approval guards
│   │
│   ├── featureworkflow/
│   │   ├── machine.go           # legacy feature state machine
│   │   └── orchestrator.go      # legacy orchestrator wrapper
│   │
│   ├── workspace/
│   │   ├── workspace.go         # .thanos path resolver
│   │   ├── repository.go        # read/write task artifacts
│   │   └── lock.go              # prevent concurrent state writes
│   │
│   ├── runner/
│   │   ├── command.go           # shell command runner
│   │   ├── git.go               # git helpers
│   │   ├── test.go              # test/lint/typecheck runner
│   │   └── worktree.go          # git worktree helpers
│   │
│   ├── agent/
│   │   ├── agent.go             # agent interface
│   │   ├── planner.go           # plan generation
│   │   ├── executor.go          # execution agent
│   │   └── reviewer.go          # review agent
│   │
│   ├── events/
│   │   ├── event.go             # structured event model
│   │   ├── emitter.go           # JSON/event stream emitter
│   │   └── subscriber.go        # UI/Tauri consumers
│   │
│   └── config/
│       ├── config.go            # config loading
│       └── defaults.go
│
├── apps/
│   └── thanos-desktop/
│       ├── src/                 # Tauri frontend only
│       └── src-tauri/           # calls thanos CLI/sidecar only
│
├── docs/
│   ├── workflows.md
│   ├── cli.md
│   └── desktop.md
│
├── .thanos/
│   ├── tasks/
│   ├── plans/
│   ├── logs/
│   ├── reviews/
│   ├── tests/
│   └── worktrees/
│
├── go.mod
├── go.sum
└── README.md
```

## Structure Rules

* `cmd/` only starts the app.
* `internal/cli/` only parses commands and calls services.
* `internal/taskworkflow/` owns task state transitions.
* `internal/featureworkflow/` isolates legacy feature workflow.
* `internal/workspace/` owns all `.thanos/` paths and persistence.
* `internal/runner/` owns shell, git, test, lint, and worktree execution.
* `internal/agent/` owns AI planning, execution, and review prompts.
* `internal/events/` owns streaming updates for CLI/TUI/Tauri.
* `apps/thanos-desktop/` must not duplicate workflow logic.

## Dependency Direction

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

Forbidden:

```text
taskworkflow -> cli
workspace -> cli
runner -> cli
agent -> cli
desktop -> taskworkflow directly
desktop -> workspace directly
```

Tauri must only call:

```text
thanos task list --json
thanos task show {id} --json
thanos task plan {id}
thanos task execute {id}
thanos task verify {id}
thanos task done {id}
```

## Package Responsibility Rule

Each package should answer one question:

```text
cli            = what command did the user run?
taskworkflow   = is this transition valid?
workspace      = where is the data stored?
runner         = how do we run commands?
agent          = what should AI generate?
events         = what changed?
model          = what is the data shape?
```

## Token-Efficient File Loading

For most task workflow changes, load only:

```text
internal/model/task.go
internal/taskworkflow/workflow.go
internal/taskworkflow/transition.go
internal/cli/task.go
internal/workspace/workspace.go
docs/workflows.md
```

Avoid loading:

```text
internal/cli/root.go
internal/cli/feature.go
internal/tui/**
internal/prompts/**
internal/codegraph/**
internal/featuregraph/**
apps/thanos-desktop/**
```

unless the change directly touches those areas.

## CLI Output Rule

Every command should support:

```text
--json
```

Human output is for terminal users.

JSON output is for:

* Tauri
* scripts
* tests
* future integrations

## Event Format

All workflow updates should emit compact structured events:

```json
{
  "task_id": "TASK-123",
  "event": "stage.completed",
  "stage": "plan",
  "status": "planned",
  "artifact": ".thanos/plans/TASK-123.md"
}
```

## Recommended Refactor Order

1. Split `internal/cli/cli.go`.
2. Extract task models from `model.go`.
3. Simplify task workflow states.
4. Centralize `.thanos/` paths in `workspace`.
5. Add JSON output for task commands.
6. Add event emitter.
7. Let Tauri consume CLI JSON only.
