# Thanos Codebase Graph

Generated: 2026-06-23T08:56:24Z

- Files: 24
- Symbols: 239
- Relationships: 874

## Languages

- go: 24 files

## Key Symbols

- `main` (function) — cmd/thanos/main.go:15
- `modelGraph` (struct) — internal/cli/cli.go:159
- `runScan` (function) — internal/cli/cli.go:164
- `runNew` (function) — internal/cli/cli.go:191
- `runStatus` (function) — internal/cli/cli.go:230
- `runFeature` (function) — internal/cli/cli.go:257
- `runPrompt` (function) — internal/cli/cli.go:277
- `runTransition` (function) — internal/cli/cli.go:304
- `runDone` (function) — internal/cli/cli.go:336
- `Execute` (function) — internal/cli/cli.go:33
- `runDoctor` (function) — internal/cli/cli.go:371
- `runSkill` (function) — internal/cli/cli.go:404
- `runSkillAdd` (function) — internal/cli/cli.go:420
- `runRunner` (function) — internal/cli/cli.go:507
- `syncSkillsToRunners` (function) — internal/cli/cli.go:580
- `syncSkillsToRunner` (function) — internal/cli/cli.go:589
- `discoverSkillFiles` (function) — internal/cli/cli.go:643
- `runPlugin` (function) — internal/cli/cli.go:673
- `appendUniqueMarketplace` (function) — internal/cli/cli.go:729
- `appendUniquePlugin` (function) — internal/cli/cli.go:738

## Hub Symbols

- `Render` — 30 incoming relationships (internal/prompts/prompts.go:37)
- `DotDir` — 24 incoming relationships (internal/workspace/workspace.go:28)
- `ReadConfig` — 20 incoming relationships (internal/workspace/workspace.go:46)
- `assertFramework` — 15 incoming relationships (internal/project/project_test.go:378)
- `Open` — 15 incoming relationships (internal/workspace/workspace.go:24)
- `writeFile` — 13 incoming relationships (internal/project/project_test.go:389)
- `frameworkRoot` — 13 incoming relationships (internal/project/project_test.go:364)
- `transition` — 12 incoming relationships (internal/orchestrator/orchestrator.go:206)
- `printExecLog` — 12 incoming relationships (internal/cli/cli.go:770)
- `runInit` — 11 incoming relationships (internal/cli/cli.go:79)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 8 test files use language-native test naming.
- **Internal package boundary:** 23 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
