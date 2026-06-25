# Thanos Codebase Graph

Generated: 2026-06-25T08:12:44Z

- Files: 400
- Symbols: 2328
- Relationships: 13936

## Languages

- go: 383 files
- python: 16 files
- javascript: 1 files

## Key Symbols

- `buildAgentRegistry` (function) — cli-sample/cmd/app/buildAgentRegistry.go:23
- `dispatcherSelector` (function) — cli-sample/cmd/app/buildAgentRegistry.go:67
- `summarySelector` (function) — cli-sample/cmd/app/buildAgentRegistry.go:76
- `refreshHost` (function) — cli-sample/cmd/app/buildAgentRegistry.go:85
- `reloadTelegram` (function) — cli-sample/cmd/app/cmdDeamon.go:100
- `reloadKuradb` (function) — cli-sample/cmd/app/cmdDeamon.go:134
- `disableKuradb` (function) — cli-sample/cmd/app/cmdDeamon.go:167
- `cmdDaemon` (function) — cli-sample/cmd/app/cmdDeamon.go:182
- `watchConfig` (function) — cli-sample/cmd/app/cmdDeamon.go:322
- `runSkill` (function) — cli-sample/cmd/app/cmdDeamon.go:385
- `reloadDiscord` (function) — cli-sample/cmd/app/cmdDeamon.go:66
- `cmdMCPServer` (function) — cli-sample/cmd/app/cmdMCPServer.go:16
- `daemonSlogHandler` (struct) — cli-sample/cmd/app/daemonSlog.go:18
- `Enabled` (function) — cli-sample/cmd/app/daemonSlog.go:22
- `Handle` (function) — cli-sample/cmd/app/daemonSlog.go:26
- `WithAttrs` (function) — cli-sample/cmd/app/daemonSlog.go:44
- `WithGroup` (function) — cli-sample/cmd/app/daemonSlog.go:48
- `installDaemonSlog` (function) — cli-sample/cmd/app/daemonSlog.go:52
- `setSummaryCron` (function) — cli-sample/cmd/app/main.go:104
- `initMCP` (function) — cli-sample/cmd/app/main.go:133

## Hub Symbols

- `String` — 574 incoming relationships (cli-sample/internal/agents/types/event.go:70)
- `parse_optional_float` — 121 incoming relationships (cli-sample/doc/demo/fetch_crypto_price/script.py:171)
- `Path` — 113 incoming relationships (internal/featuregraph/graph.go:20)
- `as_string` — 99 incoming relationships (cli-sample/doc/demo/fetch_crypto_price/script.py:153)
- `match` — 93 incoming relationships (cli-sample/internal/session/summary/validate.go:12)
- `extract_js_ts_info` — 87 incoming relationships (cli-sample/extensions/skills/readme-generate/scripts/analyze_project.py:474)
- `TypeInfo` — 87 incoming relationships (cli-sample/extensions/skills/readme-generate/scripts/analyze_project.py:28)
- `find_balanced` — 80 incoming relationships (cli-sample/extensions/skills/tool-reviewer/scripts/scan_tools.py:312)
- `Regist` — 70 incoming relationships (cli-sample/internal/tools/register/register.go:43)
- `create_skill` — 70 incoming relationships (cli-sample/extensions/skills/skill-creator/scripts/test_package_skill.py:43)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 25 test files use language-native test naming.
- **Internal package boundary:** 49 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
