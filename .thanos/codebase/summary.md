# Thanos Codebase Graph

Generated: 2026-07-01T01:31:17Z

- Files: 407
- Symbols: 2440
- Relationships: 28370

## Languages

- go: 385 files
- python: 16 files
- rust: 3 files
- typescript: 2 files
- javascript: 1 files

## Key Symbols

- `stepCopy` (symbol) — apps/thanos-desktop/src/main.ts:1002
- `ownerInitial` (symbol) — apps/thanos-desktop/src/main.ts:1012
- `formatUpdated` (symbol) — apps/thanos-desktop/src/main.ts:1016
- `pushActivity` (symbol) — apps/thanos-desktop/src/main.ts:1023
- `upsertTask` (symbol) — apps/thanos-desktop/src/main.ts:1033
- `restoreSettings` (symbol) — apps/thanos-desktop/src/main.ts:1041
- `saveSettings` (symbol) — apps/thanos-desktop/src/main.ts:1046
- `fallbackAgents` (symbol) — apps/thanos-desktop/src/main.ts:1051
- `installedAgentSummary` (symbol) — apps/thanos-desktop/src/main.ts:1061
- `basename` (symbol) — apps/thanos-desktop/src/main.ts:1066
- `escapeHtml` (symbol) — apps/thanos-desktop/src/main.ts:1071
- `renderAppShell` (symbol) — apps/thanos-desktop/src/main.ts:139
- `renderSidebar` (symbol) — apps/thanos-desktop/src/main.ts:160
- `renderTopbar` (symbol) — apps/thanos-desktop/src/main.ts:244
- `renderChatBar` (symbol) — apps/thanos-desktop/src/main.ts:269
- `renderCommandPalette` (symbol) — apps/thanos-desktop/src/main.ts:291
- `renderAll` (symbol) — apps/thanos-desktop/src/main.ts:305
- `renderSkills` (symbol) — apps/thanos-desktop/src/main.ts:323
- `renderAgentProfiles` (symbol) — apps/thanos-desktop/src/main.ts:343
- `renderAgentSelect` (symbol) — apps/thanos-desktop/src/main.ts:363

## Hub Symbols

- `icon` — 1716 incoming relationships (apps/thanos-desktop/src/main.ts:550)
- `pushActivity` — 1300 incoming relationships (apps/thanos-desktop/src/main.ts:1023)
- `renderAll` — 1196 incoming relationships (apps/thanos-desktop/src/main.ts:305)
- `String` — 1162 incoming relationships (cli-sample/internal/agents/types/event.go:70)
- `byId` — 1040 incoming relationships (apps/thanos-desktop/src/main.ts:976)
- `escapeHtml` — 988 incoming relationships (apps/thanos-desktop/src/main.ts:1071)
- `trim` — 902 incoming relationships (cli-sample/internal/session/log/record.go:120)
- `normalizeStatus` — 572 incoming relationships (apps/thanos-desktop/src/main.ts:988)
- `filter` — 373 incoming relationships (cli-sample/internal/session/summary/prompt.go:30)
- `renderArtifactPanel` — 312 incoming relationships (apps/thanos-desktop/src/main.ts:511)

## Detected Conventions

- **Go formatting and tests:** Go source is present; use gofmt and keep tests in *_test.go files beside the package.
- **Test organization:** 26 test files use language-native test naming.
- **Internal package boundary:** 51 files live under internal/; keep non-public implementation there.

Full machine-readable graph: `.thanos/codebase/graph.json`.
