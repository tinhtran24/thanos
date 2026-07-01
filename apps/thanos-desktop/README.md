# Thanos Desktop UI

This is a standalone Tauri shell for the Thanos CLI workflow.

The desktop app does not own workflow state. It runs `thanos` commands in a selected local workspace and displays stdout/stderr. `.thanos/` remains the local project database and the CLI remains the source of truth for task transitions, review gates, worktrees, tests, and completion rules.

## Development

```sh
npm install
npm run tauri dev
```

Set the workspace field to an initialized Thanos project and the binary field to either `thanos` or an absolute path to a local Thanos build.
