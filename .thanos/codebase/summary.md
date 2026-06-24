# Thanos Codebase Graph

Generated: 2026-06-24T02:56:23Z

- Files: 48
- Symbols: 499
- Relationships: 1760

## Languages

- go: 48 files

## Key Symbols

- `main` (function) — cmd/thanos/main.go:15
- `appendUniqueMarketplace` (function) — internal/cli/cli.go:1046
- `appendUniquePlugin` (function) — internal/cli/cli.go:1055
- `runExternalCommand` (function) — internal/cli/cli.go:1064
- `printExecLog` (function) — internal/cli/cli.go:1087
- `parseSkillRoles` (function) — internal/cli/cli.go:1091
- `phaseForRole` (function) — internal/cli/cli.go:1123
- `splitList` (function) — internal/cli/cli.go:1144
- `splitCSV` (function) — internal/cli/cli.go:1154
- `detectedLSPs` (function) — internal/cli/cli.go:1164
- `defaultRunnerArgs` (function) — internal/cli/cli.go:1182
- `defaultRunnerAgent` (function) — internal/cli/cli.go:1193
- `defaultSkillsDir` (function) — internal/cli/cli.go:1208
- `intersperseFlags` (function) — internal/cli/cli.go:1219
- `printHelp` (function) — internal/cli/cli.go:1244
- `modelGraph` (struct) — internal/cli/cli.go:180
- `runScan` (function) — internal/cli/cli.go:185
- `runNew` (function) — internal/cli/cli.go:228
- `runBugfix` (function) — internal/cli/cli.go:308
- `runStatus` (function) — internal/cli/cli.go:318

## Hub Symbols

- `DotDir` — 43 incoming relationships (internal/workspace/workspace.go:28)
- `ReadConfig` — 24 incoming relationships (internal/workspace/workspace.go:46)
- `Open` — 20 incoming relationships (internal/workspace/workspace.go:24)
- `SaveFeature` — 16 incoming relationships (internal/workspace/workspace.go:62)
- `printExecLog` — 16 incoming relationships (internal/cli/cli.go:1087)
- `assertFramework` — 15 incoming relationships (internal/project/project_test.go:381)
- `rerender` — 14 incoming relationships (internal/tui/chat/chat.go:275)
- `Parse` — 14 incoming relationships (internal/tui/input/input.go:146)
- `frameworkRoot` — 13 incoming relationships (internal/project/project_test.go:367)
- `RuntimeDir` — 13 incoming relationships (internal/workspace/workspace.go:116)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 13 test files use language-native test naming.
- **Internal package boundary:** 47 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
