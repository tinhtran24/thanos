# Thanos Codebase Graph

Generated: 2026-06-24T08:24:58Z

- Files: 50
- Symbols: 610
- Relationships: 2192

## Languages

- go: 50 files

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
- `Value` — 26 incoming relationships (internal/tui/input/input.go:129)
- `ReadConfig` — 25 incoming relationships (internal/workspace/workspace.go:46)
- `Open` — 25 incoming relationships (internal/workspace/workspace.go:24)
- `SetValue` — 22 incoming relationships (internal/tui/input/input.go:138)
- `RuntimeDir` — 22 incoming relationships (internal/workspace/workspace.go:116)
- `Height` — 19 incoming relationships (internal/tui/input/input.go:126)
- `SaveFeature` — 19 incoming relationships (internal/workspace/workspace.go:62)
- `printExecLog` — 19 incoming relationships (internal/cli/cli.go:1255)
- `newTestModel` — 19 incoming relationships (internal/tui/workbench_test.go:19)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 13 test files use language-native test naming.
- **Internal package boundary:** 49 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
