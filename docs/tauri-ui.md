# Thanos Desktop UI

`apps/thanos-desktop` is a standalone Tauri app for operating the existing Thanos CLI workflow from a desktop window.

## Architecture Boundary

The desktop UI is a client of the CLI, not a second implementation of the workflow.

- The CLI remains the source of truth.
- `.thanos/` remains the local project database.
- The UI calls `thanos` commands in the selected workspace.
- The UI does not write task JSON, plan files, review files, test results, worktrees, or feature memory directly.
- The UI does not add a database for v1.
- The UI does not auto-merge code.
- Workflow rules, review gates, and Done gating stay in the Go CLI.

## Module Layout

```text
apps/thanos-desktop/
  index.html
  package.json
  vite.config.ts
  src/
    main.ts
    styles.css
  src-tauri/
    Cargo.toml
    build.rs
    tauri.conf.json
    src/
      lib.rs
      main.rs
```

Tauri expects `tauri.conf.json` under `src-tauri/`. The desktop module remains separate from the Go CLI module.

## Runtime Contract

The Tauri command surface is intentionally small:

```rust
run_thanos(workspace, binary, args) -> { command, stdout, stderr, code }
```

The command validates that `workspace/.thanos` exists, then executes the configured Thanos binary with the supplied CLI arguments in that workspace.

Examples:

```text
thanos board
thanos task list --json
thanos task show T001-example --json
thanos task create "Task title" --description "..."
thanos task plan T001-example
thanos task execute T001-example
thanos task verify T001-example
thanos task verify T001-example approve
thanos task done T001-example
```

If a workflow transition is invalid, the CLI returns the error and the UI displays stderr. The UI must not special-case around that failure.

## Development

From `apps/thanos-desktop`:

```sh
npm install
npm run tauri dev
```

Use the workspace field to point at an initialized Thanos project. Use the binary field to point at `thanos` on `PATH` or an absolute path to a local build.

## Workspace And Agents

The desktop app lets the user enter a local workspace path and connect to that workspace. The Tauri backend validates that `.thanos/` exists before running workflow commands.

Agent profile selection is based on CLIs already installed on the current computer. The backend checks `PATH` for known commands such as `codex`, `claude`, `gemini`, and `opencode`, then the UI can write the selected profile to `.thanos/agents.yaml`. New tasks created from the chat command use the selected profile:

```text
/new Refactor task workflow
```

## V1 Non-Goals

- No embedded database.
- No direct YAML mutation from the frontend.
- No duplicate task transition graph in TypeScript.
- No direct git merge, rebase, or push actions.
- No long-running agent process supervisor in the UI.

Future versions can add richer board parsing or live event streaming only after the CLI exposes stable machine-readable commands or events.
