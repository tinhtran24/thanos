# Thanos — Multi-Agent AI Development Framework for Go, Codex, and Claude Code

[Tiếng Việt](README.vi.md) · [Technical Reference](Technical.md)

Thanos is an open-source, multi-agent AI development framework written in Go.
It orchestrates specialized AI coding agents across a deterministic software
engineering workflow:

```text
Plan ─▶ split the feature into ordered execution chunks (EC-1, EC-2, …)

then run EACH chunk to completion before the next one starts:

  EC-n: Code → EC Test
          ↑       │
          └───────┘
           Amend

after the last chunk: Overview → Human Review → Done
```

A planning step first breaks a feature into ordered **execution chunks (ECs)**.
Thanos drives EC-1 through coding and acceptance tests, then EC-2, and so on.
Failed EC tests return directly to coding with evidence for the next round.

Thanos assigns planning, implementation, testing, and final overview to explicit
roles with required outputs and deterministic transitions.

Thanos works with Codex, Claude Code, Cursor, Gemini CLI, and custom command-line
AI runners. It also installs Agent Skills from GitHub, synchronizes skills
between runners, and manages Claude Code plugin marketplaces.

The default project experience is a full-screen terminal UI built with Bubble
Tea and Lip Gloss. It keeps Thanos's deterministic phase graph, role isolation,
evidence gates, and human approval while presenting each feature as a resumable
work session.

## Why Thanos?

Single-agent AI coding is fast, but it creates predictable risks:

- The implementation inherits mistakes from the original design.
- The same agent reviews its own assumptions.
- Tests may validate the implementation instead of the requirement.
- Interrupted sessions lose context and progress.
- Skills become duplicated across Codex, Claude Code, Cursor, and Gemini.

Thanos moves critical controls into a deterministic Go CLI. The language model
handles reasoning and code generation; Thanos handles phase transitions,
dependencies, amendment limits, artifact validation, resumable state, and human
approval.

## Key Features

- **Focused agent workflow:** dedicated Planner, Coder, Tester, and Overview
  roles without repeated design/review rounds.
- **Deterministic quality gates:** legal phase transitions and required reports
  are enforced by Go code instead of prompts.
- **AI runner agnostic:** use Codex, Claude Code, Cursor, Gemini CLI, or a custom
  executable.
- **Session terminal UI:** browse, create, run, resume, and approve project work
  without leaving the terminal.
- **Mid-session runner switching:** change the selected LLM runner while
  preserving the specification, artifacts, event history, phase, and round.
- **LSP capability registry:** detect or register project language servers for
  runner context and health checks.
- **MCP capability registry:** configure `stdio`, `http`, and `sse` servers at
  project scope and expose them to compatible runners.
- **Persistent feature memory:** map business rules, architectural decisions,
  dependencies, related features, and affected code paths across sessions.
- **Impact-aware bugfixes:** attach a bugfix to its parent feature so every role
  receives the complete known cross-layer impact before changing code.
- **Portable Go binary:** release targets cover macOS, Linux, Windows,
  Android terminals, FreeBSD, OpenBSD, and NetBSD.
- **Crash-resistant workflow:** every feature stores state, events, prompts, and
  reports under `.thanos/`.
- **Local codebase graph:** indexes files, symbols, calls, imports, tests, hub
  symbols, and repository conventions for every AI role.
- **EC acceptance testing:** every execution chunk must pass its acceptance
  cases before the next chunk starts.
- **Human approval:** completed AI work stops at `pending-review` until a human
  runs `thanos done`.
- **GitHub Agent Skills:** search and install skills with the open
  `npx skills` ecosystem.
- **Cross-runner skill sync:** one canonical skill directory is linked into each
  runner's native skill location.
- **Claude Code plugins:** add marketplaces and install project plugins through
  Claude's native CLI.
- **No AI SDK lock-in:** runners communicate through prompts, files, and process
  exit codes.

## Installation

Homebrew (macOS / Linux)

```
brew install tinhtran24/tap/thanos
```

Thanos requires Go 1.20 or newer.

```bash
go install github.com/tinhtran/thanos/cmd/thanos@latest
```

Build from source:

```bash
git clone https://github.com/tinhtran/thanos.git
cd thanos
make check
./bin/thanos help
```

## Quick Start

Initialize Thanos inside an existing software project:

```bash
cd your-project
thanos init --runner codex --runner-command codex
```

Open the session UI:

```bash
thanos
# or: thanos ui
```

Key bindings: `↑/↓` select, `enter` run or resume, `m` switch runner, `n`
create a session, `d` approve, `r` refresh, and `q` quit.

For an existing project, initialization automatically writes:

```text
.thanos/codebase/graph.json
.thanos/codebase/summary.md
```

Create a feature:

```bash
thanos new "OAuth2 authentication" \
  --description "Add Google OAuth2 login and protected sessions." \
  --acceptance "Login succeeds;Invalid state is rejected;Tests pass"
```

Run the full AI development loop:

```bash
thanos run F001
thanos status
```

After reviewing the generated code and reports:

```bash
thanos done F001
```

Interrupted runs resume from `.thanos/<feature-id>/state.json`.

### Framework detection during init

`thanos init` stores a single canonical value in `project.framework` inside
`.thanos/settings.json`. Use `--framework VALUE` to supply an explicit value;
surrounding whitespace is trimmed. The supported auto-detected values are
`wordpress`, `laravel`, `nextjs`, `nestjs`, `angular`, `nuxt`, `gin`, `echo`,
`django`, `flask`, `fastapi`, `actix-web`, `axum`, and `rocket`.

Detection uses only root evidence for the final selected language, after any
`--language` override:

- PHP: `composer.json`, or the `artisan` and `bootstrap/app.php` Laravel
  markers, or the `wp-admin`, `wp-includes`, and `wp-content` WordPress
  directories.
- TypeScript: `package.json`.
- Go: `go.mod`.
- Python: `pyproject.toml` and root `requirements*.txt` files.
- Rust: `Cargo.toml`.

If evidence identifies multiple supported frameworks, detection is ambiguous
and writes no framework. An empty framework is omitted from settings. Detection
is local, read-only, and network-free; it runs no package manager and executes
no project command.

## Persistent feature memory and bugfix mapping

Create features with durable rules and known scope:

```bash
thanos new "Password policy" \
  --rules "Passwords require at least 12 characters;Registration and reset share the same policy" \
  --scope "internal/auth/password.go;web/auth/password.ts;docs/security.md"
```

Map a bugfix to that feature:

```bash
thanos bugfix F001 "Password reset accepts short passwords" \
  --description "Reset validation does not enforce the shared password policy." \
  --acceptance "Registration, reset, and account settings enforce the same minimum"
```

Before any role runs, Thanos resolves:

- The parent feature and connected dependencies or related features.
- Stored business rules and acceptance invariants.
- Architectural decisions learned during feature acceptance.
- Explicitly declared paths and files recorded by prior coder reports.
- Neighboring callers and callees inferred from the local code graph.
- Tests, frontend files, contracts, and documentation associated with those
  paths.

Inspect memory directly:

```bash
thanos memory
thanos memory F002
```

Accepted features produce `.thanos/<feature-id>/feature-memory.json`. Thanos
merges it into `.thanos/memory/feature-graph.json` and injects the resolved
impact map into Planner, Coder, Tester, and Overview prompts.

## LSP and MCP capabilities

Thanos records a matching language server during `init` when the executable is
already installed. Servers can also be registered explicitly:

```bash
thanos lsp add go --command gopls
thanos lsp add typescript --command typescript-language-server --args "--stdio"
```

Register MCP servers with `stdio`, `http`, or `sse` transport:

```bash
thanos mcp add filesystem --type stdio --command node --args "/path/to/server.js"
thanos mcp add github --type http --url https://api.githubcopilot.com/mcp/
thanos mcp add events --type sse --url https://example.com/mcp/sse
```

`thanos doctor` validates runner, LSP, and MCP configuration. Registered
capabilities are included in role prompts; the selected runner must expose the
matching native LSP or MCP tools to execute them.

## Local Codebase Graph for AI Agents

A codebase is structure, not only text. Thanos records source files, programming
languages, symbols, function calls, imports, test relationships, hub symbols,
and detected repository conventions.

Every role is instructed to read `.thanos/codebase/summary.md` before exploring
source files. The complete machine-readable graph is stored in
`.thanos/codebase/graph.json`.

Refresh it manually after large external changes:

```bash
thanos scan
```

The graph is also refreshed automatically after successful feature acceptance.
Everything remains local; no SaaS account, API key, or source upload is needed.

## Multi-Agent Development Roles

| Role | Responsibility | Main output |
|---|---|---|
| Planner | Analyze the ticket and split it into implementation-ready EC tasks | `execution-plan.yaml` |
| Coder | Implement the current EC; amend it when tests reject the result | Source changes and `coder-report.md` |
| Tester | Run each EC's acceptance cases with evidence | `test-report.md` |
| Overview | Summarize readiness, evidence, and unresolved issues | `final-report.md` |

Specialized prompts are also included for Mini-Coder fixes, re-verification,
parallel review synthesis, and evolution value gating.

Each chunk's artifacts live under `.thanos/<feature-id>/ec-<n>/` when a feature
has more than one chunk (single-chunk features keep the flat layout). If a role
hits a genuinely ambiguous decision it writes a `clarify.json` question and the
run pauses for a human answer — in the TUI a popup lets you choose, or run
`thanos clarify FEATURE_ID "<answer>"`. A project `.thanos/coding-style.md`, when
present, is injected into planner and coder prompts so generated code matches
your conventions.

## Working session UI

The default experience (`thanos` or `thanos ui`) is a chat-first terminal app:

- **Left** — the role-by-role agent conversation for the selected feature, with a
  persistent multiline workflow panel (`planning → coding → EC tests → overview
  → human review → done`). It shows completed, active, pending, rejected, and
  blocked states above the scrollable chat output.
- **Right sidebar** — the THANOS logo, a clickable **Feature → EC tree** (each
  chunk shows its status), the active model runner, and configured MCP servers.
- **Command box** — type `/` for the full command palette (run, continue, new,
  bugfix, runner, transition, prompt, status, scan, doctor, memory, skill,
  plugin, lsp, mcp, find, copy, clear, help). Attach files by pasting a path or
  referencing `@path`; they are passed to the agent as run context. Paste
  single-line or multiline text directly into the composer; line breaks are
  preserved, and Enter submits or advances the active guided form.

Tree keys: `↑↓` move, `→/←` descend into / out of a feature's ECs, `enter` run,
`x` remove an EC, `c` answer a clarification, `n` new feature, `tab` switch panes.

## Agent Skills from GitHub

Search the Agent Skills ecosystem:

```bash
thanos skill find golang
thanos skill find security
```

Install skills from a GitHub repository:

```bash
thanos skill add abc/skill
```

Install one skill and enable it only for selected Thanos roles:

```bash
thanos skill add vercel-labs/agent-skills \
  --skill web-design-guidelines \
  --roles designer,coder,reviewer
```

Thanos delegates installation to the open Skills CLI:

```bash
npx skills add owner/repo --agent universal --yes --copy
```

Discovered `SKILL.md` files are recorded in `.thanos/settings.json` and injected
into matching role prompts.

## Synchronize Skills Across Codex, Claude Code, Cursor, and Gemini

Add another AI coding runner:

```bash
thanos runner add claude --command claude
thanos runner add codex --command codex
```

Thanos links configured skills from the canonical project skill directory into
runner-native directories:

| Runner | Skill directory |
|---|---|
| Claude Code | `.claude/skills/` |
| Codex | `.agents/skills/` |
| Cursor | `.agents/skills/` |
| Gemini CLI | `.agents/skills/` |

Configure a custom runner:

```bash
thanos runner add custom-agent \
  --command custom-agent \
  --agent custom-agent \
  --skills-dir .custom-agent/skills
```

Relative symlinks provide one source of truth. Thanos never overwrites an
existing non-symlink skill directory.

## Claude Code Plugin Management

Add a Claude Code plugin marketplace:

```bash
thanos plugin marketplace add claude anthropics/claude-code
```

Install a plugin for the current project:

```bash
thanos plugin install claude \
  commit-commands@claude-code-plugins \
  --scope project
```

These commands invoke Claude Code's native plugin CLI and record successful
operations in `.thanos/settings.json`.

## File-Based AI Agent Protocol

```text
.thanos/
├── settings.json
├── features/
│   └── F001-oauth2-authentication.yaml
└── F001-oauth2-authentication/
    ├── state.json
    ├── events.jsonl
    ├── execution-plan.yaml
    ├── final-report.md
    ├── retro-learnings.json
    ├── feature-memory.json
    └── rounds/
        └── round-1/
            ├── coder-report.md
            └── test-report.md
```

The filesystem is the source of truth. Agents do not need shared hidden context,
and a failed process can restart from the latest validated phase.

## CLI Commands

| Command | Description |
|---|---|
| `thanos` / `thanos ui` | Open the project session TUI |
| `thanos init` | Initialize a network-free Thanos workspace |
| `thanos new` | Create a feature specification |
| `thanos bugfix` | Create a bugfix mapped to an existing feature |
| `thanos run` | Run or resume the multi-agent workflow |
| `thanos continue` | Resume a stalled feature from its last failed round |
| `thanos status` | Display feature phase and round status |
| `thanos plan ls\|add\|rm` | List, add, or remove a feature's execution chunks (ECs) |
| `thanos clarify` | Answer a paused clarification and resume the run |
| `thanos ask "<prompt>"` | Send a one-off prompt to the runner (headless, no pipeline) |
| `thanos prompt` | Render a role prompt without executing a runner |
| `thanos transition` | Apply a validated manual phase transition |
| `thanos done` | Approve a pending feature |
| `thanos doctor` | Check configured runner executables |
| `thanos scan` | Build or refresh the local codebase graph |
| `thanos skill find` | Search available Agent Skills |
| `thanos skill add` | Install and register skills from Git or local sources |
| `thanos runner add` | Register a runner and synchronize existing skills |
| `thanos lsp add` | Register a project language server |
| `thanos mcp add` | Register a `stdio`, `http`, or `sse` MCP server |
| `thanos memory` | Inspect the persistent project or feature impact graph |
| `thanos plugin marketplace add` | Add a runner plugin marketplace |
| `thanos plugin install` | Install and record a runner plugin |

For configuration details, runner contracts, settings examples, and safety
behavior, read the [Technical Reference](Technical.md).

## Use Cases

Thanos is designed for:

- Security-sensitive features such as authentication and authorization.
- Payment, billing, migration, and data integrity work.
- AI-assisted pull requests that require independent review.
- Teams that need auditable AI-generated code and test evidence.
- Long-running features that must survive interrupted AI sessions.
- Developers using multiple AI coding agents with shared skills.

For one-line fixes or exploratory prototypes, a direct single-agent workflow may
be faster.

## Safety

AI runners and plugins execute with your operating-system permissions. Thanos
enforces workflow rules, but it is not an operating-system sandbox. Review
third-party skills and plugins before installation, use isolated branches or
worktrees, and inspect generated changes before running `thanos done`.

## Development

```bash
make build
make test
make lint
make check
```

## Inspiration and Standards

- Agent Skills integration through [vercel-labs/skills](https://github.com/vercel-labs/skills)
- Claude Code plugins through the [official plugin system](https://code.claude.com/docs/en/discover-plugins)

## License

Thanos is available under the [MIT License](LICENSE).
