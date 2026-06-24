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

- Bubble Tea terminal UI with project sessions, phase progress, capability
  status, and keyboard-driven task operations
- Per-session runner selection with filesystem context preserved across model
  changes
- Persistent feature graph containing business rules, architectural decisions,
  feature relationships, and affected paths
- Bugfix-to-feature mapping with inherited impact context
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

## Terminal UI architecture

The TUI ports the transferable parts of Charm's Crush architecture (now on
**Bubble Tea v2** / Lip Gloss v2, `charm.land/...`) without replacing Thanos's
workflow engine. It lives in `internal/tui/` as component packages:

| Package | Responsibility |
|---|---|
| `tui` | Root `tea.Model`: state, message routing, focus, mouse, layout (`view.go`) |
| `chat` | Role-attributed agent log (viewport), bubble selection/copy, phase-flow strip |
| `sidebar` | Logo + clickable Feature→EC tree + model/MCP info (right column) |
| `dialog` | Feature picker, help, and the clarification popup |
| `input` | Command box (slash-commands, completions, real terminal cursor) |
| `attachments` | Staged files/`@`-refs and the context-manifest writer |
| `styles` / `util` | Palette + role/phase styling; text, clipboard (OSC52), overlay |

Key points:

1. One top-level model owns terminal size, tree selection (feature + EC cursor),
   task execution, and rendering. `View()` returns a `tea.View` whose `AltScreen`
   and `MouseMode` fields are declarative (v2 dropped the program-option form).
2. Features are project-scoped sessions; their YAML and
   `.thanos/<feature-id>/state.json` (now including `ec_index`/`ec_total`) remain
   the source of truth. Each feature's `execution-plan.yaml` drives the tree.
3. Agent runs execute as a subprocess of the `thanos` binary itself
   (`os.Executable()`), streamed into the chat; the engine writes role events to
   `events.jsonl`, which the TUI tails to render each role as a chat bubble.
4. The right sidebar reports the active runner, code graph, LSPs, MCPs, and the
   per-EC status tree; clicking a feature/EC row selects it.
5. The command box exposes the full CLI surface as slash-commands and stages
   `@path`/pasted files into the run context manifest.

Switching runners updates the feature and runtime state only — it does not
rewrite prompts, reports, events, or artifacts, so another LLM can continue the
same session at its current validated phase.

## Execution chunks (ECs) & the per-EC engine

`internal/orchestrator/orchestrator.go` first runs the **planner** role, which
writes `execution-plan.yaml` (`model.ExecutionPlan` → `[]model.ExecutionChunk`).
The orchestrator then iterates the chunks: each EC runs the full
`design → design-review → code ↔ amend → review → test → deep-review` cycle to a
passing deep review before the next EC starts; after the last EC the feature-level
`accept` synthesizes results and moves to `pending-review`.

- `model.State` gains `ECIndex` (1-based; 0 during planning), `ECTotal`, `ECID`;
  `Round` is the **per-EC** amend counter, reset when a chunk's design begins.
- Artifacts are EC-scoped via `ec-<i>/…` when `ECTotal > 1`; a single implicit
  chunk keeps the flat feature-root layout (backward compatible). Helpers
  `ecDir`/`ecPrefix`/`ecJoin` build the paths; prompts receive the prefix as
  `Data.ECPrefix` and the chunk as `Data.ExecutionChunk`.
- State-machine additions (`internal/state/machine.go`): `Init → Plan`,
  `Plan → Design`, and `DeepReview → {Accept, Design (next EC), Amend}`.

### Clarification protocol

Any role may write `clarify.json` (`{"question": "...", "options": [...]}`) instead
of finishing. After the role runs, `Orchestrator.clarifyPending` detects an
unanswered/newer `clarify.json` and the run pauses cleanly (`Active=false`,
`Reason="needs clarification"`, phase preserved). `thanos clarify` (or the TUI
popup) writes `clarify-answer.md` and re-runs; the role reads the answer and
proceeds. Paths are EC-scoped like other chunk artifacts.

### Coding style & attachments

If `.thanos/coding-style.md` exists it is loaded into `prompts.Data.CodingStyle`
and injected into the designer/coder/reviewer templates. Before a run the TUI
writes staged attachments and `@`-file references to
`.thanos/<id>/context/attachments.md`; `prompts.Render` references it (EC-level
overrides feature-level) so the agent reads it as primary context.

## Feature memory graph

The symbol graph and feature graph have separate responsibilities:

- `.thanos/codebase/graph.json` describes files, symbols, calls, imports, tests,
  and repository conventions.
- `.thanos/memory/feature-graph.json` describes product features, bugfixes,
  business invariants, architectural decisions, dependencies, related
  features, and known affected paths.

A bugfix feature stores `type: bugfix` and a full parent feature ID. At run
time, Thanos rebuilds feature metadata from YAML, merges learned memory from
accepted work, resolves the connected feature context, and expands known paths
through code-graph relationships. The resulting impact map is appended to every
role prompt under `Persistent Feature Memory`.

The Acceptor must write:

```json
{
  "business_rules": ["durable behavior or invariant"],
  "architectural_decisions": ["decision and rationale"],
  "affected_paths": ["project/relative/path"]
}
```

Coder reports also contribute paths from their `Files Changed` section. This
means project memory improves after each completed workflow while remaining
auditable and editable on disk.

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
  coding-style.md            # optional; injected into design/code/review prompts
  features/
    F001-oauth2-authentication.yaml
  F001-oauth2-authentication/
    state.json               # includes ec_index / ec_total / ec_id
    events.jsonl             # role-start/end, transition, ec-start/end, clarify
    execution-plan.yaml      # the planner's ordered execution chunks (ECs)
    final-report.md          # feature-level accept artifacts (after the last EC)
    retro-learnings.json
    feature-memory.json
    ec-1/                    # per-chunk artifacts (only when >1 chunk)
      task-brief.md
      acceptance-criteria.md
      test-strategy.yaml
      design-review-report.md
      clarify.json           # written by a role when blocked; pauses the run
      clarify-answer.md      # written by `thanos clarify`; consumed on resume
      context/attachments.md # staged files / @-refs passed to the agent
      rounds/
        round-1/
          coder-report.md
          review-report.md
          test-report.md
          deep-review-report.md
    ec-2/
      ...
```

A single-chunk feature keeps the flat layout (artifacts directly under the
feature directory, no `ec-<n>/`), so existing features and runs are unaffected.
Feature YAML files are the human-editable backlog. Runtime state and reports are
kept separately so interrupted runs can resume without shared model context.

## Commands

| Command | Purpose |
|---|---|
| `thanos init` | Initialize `.thanos/` without network access |
| `thanos new` | Create a feature |
| `thanos run` | Run or resume the agent loop |
| `thanos continue` | Resume a stalled feature from its last failed round |
| `thanos status` | List feature and phase status |
| `thanos plan ls\|add\|rm FEATURE_ID` | List/add/remove a feature's execution chunks |
| `thanos clarify FEATURE_ID "<answer>"` | Answer a paused clarification and resume |
| `thanos ask "<prompt>" [--runner NAME]` | One-off headless prompt to the runner |
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
