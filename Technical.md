# Thanos Technical Reference

Thanos is a multi-role AI development framework written in Go. It splits the
software engineering loop into isolated specialist phases:

```
Design -> Design Review -> Code -> Review -> Test -> Deep Review -> Accept
                              ^       |        |          |
                              +-------+--------+----------+
                                      Amend
```

Each phase is driven by a dedicated AI agent. The Go CLI owns deterministic
concerns—state transitions, dependency checks, round limits, required artifacts,
event logs, and the human completion gate—while AI runners receive role-specific
prompts through stdin.

Like 4X strategy games (eXplore, eXpand, eXploit, eXterminate), distinct roles
with distinct strengths converge to conquer complexity.

## Features

- Full prompt suite derived from the upstream 4x role templates
- Designer, Design Reviewer, Coder, Reviewer, Tester, Deep Reviewer, and Acceptor pipeline
- Mini-Coder, Re-Verifier, Synthesizer, and Gate prompts for specialized workflows
- File-based `.thanos/` protocol for auditability and crash recovery
- Local codebase graph with symbols, calls, imports, tests, hubs, and conventions
- Deterministic state machine and amendment round budget
- Configurable runner commands; Codex is the default
- Feature dependencies and short feature IDs such as `F001`
- Required artifact validation between phases
- Append-only JSONL event history
- Human `pending-review` gate before completion
- No AI SDK dependency or vendor lock-in

## Install

Requires Go 1.20 or newer.

```bash
go install github.com/tinhtran/thanos/cmd/thanos@latest
```

For local development:

```bash
make check
./bin/thanos help
```

## Quick start

```bash
cd your-project

thanos init --runner codex --runner-command codex

# Search the open skill directory:
thanos skill find golang

# Install from GitHub and enable it for all roles:
thanos skill add abc/skill

# Install a specific skill and limit prompt injection to selected roles:
thanos skill add vercel-labs/agent-skills \
  --skill web-design-guidelines \
  --roles designer,coder,reviewer

# Configure Claude Code plugins:
thanos plugin marketplace add claude anthropics/claude-code
thanos plugin install claude commit-commands@claude-code-plugins --scope project

# Add another runner; configured skills are linked into its native directory:
thanos runner add claude --command claude

thanos new "OAuth2 authentication" \
  --description "Add Google OAuth2 login and protected sessions." \
  --acceptance "Login succeeds;Invalid state is rejected;Tests pass"

thanos run F001
thanos status

# Review the code and reports, then:
thanos done F001
```

`thanos init` is network-free and does not install a hardcoded skill repository.
Skills and plugins are explicit project-level operations.

When source files already exist, initialization writes
`.thanos/codebase/graph.json` and `.thanos/codebase/summary.md`.

`thanos run` resumes from the phase stored in `.thanos/F001-.../state.json`.

### Project framework metadata

Initialization persists at most one canonical framework as
`project.framework` in `.thanos/settings.json`. A trimmed non-empty
`--framework VALUE` overrides automatic detection. Automatic selection uses the
final project language after `--language` is applied.

Canonical values are `wordpress`, `laravel`, `nextjs`, `nestjs`, `angular`,
`nuxt`, `gin`, `echo`, `django`, `flask`, `fastapi`, `actix-web`, `axum`, and
`rocket`.

Only root evidence is inspected:

- PHP reads `composer.json` and checks `artisan` plus `bootstrap/app.php`, and
  the `wp-admin`, `wp-includes`, and `wp-content` directories.
- TypeScript reads `package.json`.
- Go reads `go.mod`.
- Python reads `pyproject.toml` and root `requirements*.txt` files.
- Rust reads `Cargo.toml`.

Zero matches produce no framework. Multiple matches are ambiguous and also
produce no framework. An empty framework is omitted from settings. Framework
detection is local, read-only, and network-free. It does not run a package
manager or execute a project command.

## Runner contract

A runner is any executable that:

1. Reads a complete role prompt from stdin.
2. Operates in the project working directory.
3. Creates the files explicitly required by that prompt.
4. Exits non-zero on failure.

Configure runners in `.thanos/settings.json`:

```json
{
  "default_runner": "codex",
  "runners": {
    "codex": {
      "command": "codex",
      "args": ["exec", "--full-auto", "-"]
    },
    "claude": {
      "command": "claude",
      "args": ["--print", "--dangerously-skip-permissions"]
    }
  }
}
```

Runner arguments vary by installed CLI version. `thanos doctor` checks that the
configured executables exist.

## File protocol

```text
.thanos/
  settings.json
  features/
    F001-oauth2-authentication.yaml
  F001-oauth2-authentication/
    state.json
    events.jsonl
    task-brief.md
    acceptance-criteria.md
    test-strategy.yaml
    design-review-report.md
    final-report.md
    retro-learnings.json
    rounds/
      round-1/
        coder-report.md
        review-report.md
        test-report.md
        deep-review-report.md
```

Feature YAML files are the human-editable backlog. Runtime state and reports are
kept separately so interrupted runs can resume without shared model context.

## Commands

| Command | Purpose |
|---|---|
| `thanos init` | Initialize `.thanos/` without network access |
| `thanos new` | Create a feature |
| `thanos run` | Run or resume the agent loop |
| `thanos status` | List feature and phase status |
| `thanos prompt` | Render one role prompt without running it |
| `thanos transition` | Perform a validated manual transition |
| `thanos done` | Apply human approval |
| `thanos doctor` | Validate configured runner executables |
| `thanos scan` | Build or refresh the local codebase graph |
| `thanos skill find` | Search the open agent-skill directory through `npx skills find` |
| `thanos skill add` | Install a Git/local skill source and register discovered skills |
| `thanos plugin marketplace add` | Add a runner-specific plugin marketplace |
| `thanos plugin install` | Install and record a runner plugin |
| `thanos runner add` | Add a runner and synchronize configured skills |

## Skills

Thanos delegates skill discovery and installation to the open
[`vercel-labs/skills`](https://github.com/vercel-labs/skills) CLI:

```bash
thanos skill find security
thanos skill add owner/repo
thanos skill add owner/repo --skill security-review --roles reviewer,deep-reviewer
```

The equivalent manual command is:

```bash
npx skills add owner/repo --agent universal --yes --copy
```

Thanos runs this command in project scope, discovers the resulting `SKILL.md`
files, and persists them in `.thanos/settings.json`:

```json
{
  "skills": [
    {
      "name": "security-review",
      "path": ".agents/skills/security-review/SKILL.md",
      "source": "owner/repo",
      "roles": ["coder", "tester"]
    }
  ]
}
```

An empty `roles` list applies the skill to every prompt.

## Plugins

Runner plugins use each runner's native package manager. Claude Code is
supported initially:

```bash
thanos plugin marketplace add claude anthropics/claude-code
thanos plugin install claude commit-commands@claude-code-plugins --scope project
```

These map to:

```bash
claude plugin marketplace add anthropics/claude-code
claude plugin install commit-commands@claude-code-plugins --scope project
```

Successful marketplace and plugin operations are recorded in
`.thanos/settings.json`. Plugins can execute code with user privileges; only
install trusted sources.

## Runner skill synchronization

Thanos keeps installed project skills in a canonical location and synchronizes
them to runner-native directories with relative symlinks. This follows the
single-source approach described in
[One Repo, Zero Copy-Paste](https://dev.to/opensite/how-to-sync-ai-coding-agent-skills-across-every-platform-one-repo-zero-copy-paste-ba0).

```bash
thanos runner add claude --command claude
thanos runner add codex --command codex
thanos runner add custom-agent \
  --command custom-agent \
  --agent custom-agent \
  --skills-dir .custom-agent/skills
```

Known mappings include Claude Code (`.claude/skills`) and Codex, Cursor, and
Gemini (`.agents/skills`). Adding a runner links every skill already recorded in
settings. Adding a new skill also synchronizes it to all configured runners.
Existing non-symlink skill directories are never overwritten.

## Codebase graph

```bash
thanos scan
```

The scanner ignores generated and dependency directories including `.git`,
`.thanos`, `.sense`, `node_modules`, `vendor`, `dist`, `build`, `.build`,
`.nestjs`, and `.medusa`. It extracts Go symbols and call relationships with the
Go parser, plus lightweight symbol detection for TypeScript, JavaScript, and
Python.

Outputs:

- `.thanos/codebase/graph.json` — nodes, relationships, language counts, and
  detected conventions.
- `.thanos/codebase/summary.md` — key symbols, hubs, repository statistics, and
  conventions for agent cold start.

Every role prompt points to this summary. The graph is generated automatically
during initialization of an existing project and refreshed after successful
feature acceptance.

## Safety model

Thanos does not treat an LLM prompt as a security boundary. The CLI enforces the
phase graph, dependencies, round limits, report presence, and completion gate.
Runner processes still execute with the operating-system permissions of the user,
so use them only in repositories where autonomous edits are acceptable.
