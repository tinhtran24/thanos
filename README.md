# Thanos — Multi-Agent AI Development Framework for Go, Codex, and Claude Code

[Tiếng Việt](README.vi.md) · [Technical Reference](Technical.md)

Thanos is an open-source, multi-agent AI development framework written in Go.
It orchestrates specialized AI coding agents across a deterministic software
engineering workflow:

```text
Design → Design Review → Code → Review → Test → Deep Review → Accept
                            ↑        │       │          │
                            └────────┴───────┴──────────┘
                                      Amend
```

Use Thanos when one AI agent is not enough. Instead of asking the same model to
design, implement, review, and approve its own work, Thanos assigns each phase
to an isolated role with explicit inputs, required outputs, and automated
quality gates.

Thanos works with Codex, Claude Code, Cursor, Gemini CLI, and custom command-line
AI runners. It also installs Agent Skills from GitHub, synchronizes skills
between runners, and manages Claude Code plugin marketplaces.

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

- **Multi-agent software development:** dedicated Designer, Coder, Reviewer,
  Tester, Deep Reviewer, and Acceptor roles.
- **Deterministic quality gates:** legal phase transitions and required reports
  are enforced by Go code instead of prompts.
- **AI runner agnostic:** use Codex, Claude Code, Cursor, Gemini CLI, or a custom
  executable.
- **Crash-resistant workflow:** every feature stores state, events, prompts, and
  reports under `.thanos/`.
- **Local codebase graph:** indexes files, symbols, calls, imports, tests, hub
  symbols, and repository conventions for every AI role.
- **Adversarial code review:** normal review and deep review catch different
  classes of defects.
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
| Designer | Convert requirements into implementation-ready scope | `task-brief.md`, acceptance criteria, test strategy |
| Design Reviewer | Find architecture gaps before coding begins | `design-review-report.md` |
| Coder | Implement only the approved task brief | Source changes and `coder-report.md` |
| Reviewer | Check correctness, scope, security, and project rules | `review-report.md` |
| Tester | Verify every acceptance criterion with evidence | `test-report.md` |
| Deep Reviewer | Run adversarial, cross-file, and architectural review | `deep-review-report.md` |
| Acceptor | Summarize readiness and unresolved issues | `final-report.md` |

Specialized prompts are also included for Mini-Coder fixes, re-verification,
parallel review synthesis, and evolution value gating.

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
    ├── task-brief.md
    ├── acceptance-criteria.md
    ├── test-strategy.yaml
    ├── design-review-report.md
    ├── final-report.md
    ├── retro-learnings.json
    └── rounds/
        └── round-1/
            ├── coder-report.md
            ├── review-report.md
            ├── test-report.md
            └── deep-review-report.md
```

The filesystem is the source of truth. Agents do not need shared hidden context,
and a failed process can restart from the latest validated phase.

## CLI Commands

| Command | Description |
|---|---|
| `thanos init` | Initialize a network-free Thanos workspace |
| `thanos new` | Create a feature specification |
| `thanos run` | Run or resume the multi-agent workflow |
| `thanos status` | Display feature phase and round status |
| `thanos prompt` | Render a role prompt without executing a runner |
| `thanos transition` | Apply a validated manual phase transition |
| `thanos done` | Approve a pending feature |
| `thanos doctor` | Check configured runner executables |
| `thanos scan` | Build or refresh the local codebase graph |
| `thanos skill find` | Search available Agent Skills |
| `thanos skill add` | Install and register skills from Git or local sources |
| `thanos runner add` | Register a runner and synchronize existing skills |
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
