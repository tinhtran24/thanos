# Thanos Codebase Graph

Generated: 2026-06-24T04:21:45Z

- Files: 49
- Symbols: 566
- Relationships: 2018

## Languages

- go: 49 files

## Key Symbols

- `main` (function) — cmd/thanos/main.go:15
- `runMCP` (function) — internal/cli/cli.go:1013
- `runInit` (function) — internal/cli/cli.go:102
- `syncSkillsToRunners` (function) — internal/cli/cli.go:1065
- `syncSkillsToRunner` (function) — internal/cli/cli.go:1074
- `discoverSkillFiles` (function) — internal/cli/cli.go:1128
- `runPlugin` (function) — internal/cli/cli.go:1158
- `appendUniqueMarketplace` (function) — internal/cli/cli.go:1214
- `appendUniquePlugin` (function) — internal/cli/cli.go:1223
- `runExternalCommand` (function) — internal/cli/cli.go:1232
- `printExecLog` (function) — internal/cli/cli.go:1255
- `parseSkillRoles` (function) — internal/cli/cli.go:1259
- `phaseForRole` (function) — internal/cli/cli.go:1291
- `splitList` (function) — internal/cli/cli.go:1312
- `splitCSV` (function) — internal/cli/cli.go:1322
- `detectedLSPs` (function) — internal/cli/cli.go:1332
- `defaultRunnerArgs` (function) — internal/cli/cli.go:1350
- `defaultRunnerAgent` (function) — internal/cli/cli.go:1361
- `defaultSkillsDir` (function) — internal/cli/cli.go:1376
- `intersperseFlags` (function) — internal/cli/cli.go:1387

## Hub Symbols

- `DotDir` — 47 incoming relationships (internal/workspace/workspace.go:28)
- `ReadConfig` — 25 incoming relationships (internal/workspace/workspace.go:46)
- `Open` — 23 incoming relationships (internal/workspace/workspace.go:24)
- `contains` — 22 incoming relationships (internal/orchestrator/orchestrator_test.go:44)
- `RuntimeDir` — 21 incoming relationships (internal/workspace/workspace.go:116)
- `printExecLog` — 19 incoming relationships (internal/cli/cli.go:1255)
- `SaveFeature` — 18 incoming relationships (internal/workspace/workspace.go:62)
- `Truncate` — 16 incoming relationships (internal/tui/util/text.go:11)
- `assertFramework` — 15 incoming relationships (internal/project/project_test.go:381)
- `Parse` — 15 incoming relationships (internal/tui/input/input.go:168)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 13 test files use language-native test naming.
- **Internal package boundary:** 48 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
