# Task Brief — Feature Auto detect framework when init project

## Goal

Extend `thanos init` to deterministically detect the primary framework for PHP,
TypeScript, Go, Python, and Rust projects, then persist one canonical framework
name as `project.framework` in `.thanos/settings.json`.

Detection must be local, read-only, language-scoped, and evidence-based. It must
not execute project commands or use the network. Missing or malformed optional
evidence yields no match, while genuine filesystem failures stop initialization.
If valid evidence identifies more than one supported framework for the selected
language, the result is empty.

## Tasks

1. Update `internal/model/model.go`:
   - Add `Framework string` to `model.Project` with JSON tag
     `json:"framework,omitempty"`.
   - Preserve backward compatibility: an empty framework is omitted from JSON.

2. Add framework detection to `internal/project/project.go` with the exported
   integration point:
   - `DetectFramework(root, language string) (string, error)`.
   - Normalize `language` with `strings.ToLower(strings.TrimSpace(language))`
     only when selecting a detector.
   - Return only these canonical values:
     - PHP: `wordpress`, `laravel`
     - TypeScript: `nextjs`, `nestjs`, `angular`, `nuxt`
     - Go: `gin`, `echo`
     - Python: `django`, `flask`, `fastapi`
     - Rust: `actix-web`, `axum`, `rocket`
   - Collect matches in a set. Return the sole match, or `""` when there are
     zero or multiple matches. Never resolve ambiguity by iteration order or
     framework precedence.

3. Implement the following exact PHP evidence contract:
   - Parse only root `composer.json`.
   - Inspect exact package keys in both top-level `require` and `require-dev`.
     Match `laravel/framework` case-insensitively after trimming the key.
     Composer aliases or values do not create a match unless the dependency key
     itself is `laravel/framework`.
   - Also match Laravel when root `artisan` and root `bootstrap/app.php` both
     resolve through `os.Stat` to regular files.
   - Match WordPress only when root `wp-admin`, `wp-includes`, and `wp-content`
     all resolve through `os.Stat` to directories.
   - A partial marker set is not evidence. Marker type mismatches are not
     evidence.

4. Implement the following exact TypeScript evidence contract:
   - Parse only root `package.json`.
   - Inspect exact keys in top-level `dependencies` and `devDependencies`.
   - Map `next` to `nextjs`, `@nestjs/core` to `nestjs`, `@angular/core` to
     `angular`, and `nuxt` to `nuxt`.
   - Package keys are matched exactly and case-sensitively, consistent with npm
     package naming. Values, scripts, comments, package-lock files, nested
     manifests, and similarly named packages are not evidence.

5. Implement the following exact Go evidence contract:
   - Parse only root `go.mod` with a module-aware parser such as
     `golang.org/x/mod/modfile`; adding that maintained dependency to `go.mod`
     and `go.sum` is in scope.
   - Inspect `require` directives, including grouped and single-line forms.
   - Match exact module path `github.com/gin-gonic/gin`.
   - Match `github.com/labstack/echo` and versioned major paths matching
     `github.com/labstack/echo/vN`, where `N` is a decimal major version.
   - `replace`, `exclude`, comments, source imports, `go.sum`, and path prefixes
     such as `github.com/gin-gonic/gin-contrib` are not evidence by themselves.

6. Implement the following exact Python evidence contract:
   - Parse only root `pyproject.toml` and regular root files whose names start
     with `requirements` and end with `.txt`, discovered with `os.ReadDir`.
   - A TOML parser may be added to `go.mod`/`go.sum`; do not implement TOML with
     substring matching.
   - Supported `pyproject.toml` declaration locations are:
     - PEP 621 arrays: `project.dependencies` and every array under
       `project.optional-dependencies`.
     - Poetry tables: `tool.poetry.dependencies`,
       `tool.poetry.dev-dependencies`, and every
       `tool.poetry.group.<group>.dependencies` table.
     - PEP 735 arrays under top-level `dependency-groups`; include direct string
       requirements and ignore non-string include/table entries.
   - For Poetry tables, the dependency key is the distribution name. Ignore the
     `python` key. A renamed source alias is not inferred from a value.
   - For arrays and requirements files, parse the leading PEP 508 distribution
     name before extras, version specifiers, URL markers, or environment
     markers. Ignore blank lines, full-line comments, trailing comments,
     options beginning with `-`, include directives, editable paths, direct
     local paths, and VCS URLs that do not expose a valid leading distribution
     name.
   - Normalize distribution names per PEP 503: lowercase and collapse runs of
     `.`, `_`, or `-` to `-`. Match `django`, `flask`, or `fastapi`.

7. Implement the following exact Rust evidence contract:
   - Parse only root `Cargo.toml` with a TOML parser.
   - Inspect root `dependencies`, `dev-dependencies`, and
     `build-dependencies`; `workspace.dependencies`; and dependency tables under
     every `target.<target>`.
   - For a string or table dependency, use its table key as the package name.
     For a renamed table dependency containing a string `package`, use that
     `package` value instead. A table containing only `workspace = true` uses
     its key.
   - Normalize candidate crate names to lowercase with `_` converted to `-`.
     Match `actix-web`, `axum`, or `rocket`.
   - Features, values, comments, `Cargo.lock`, source files, and nested
     `Cargo.toml` files are not evidence.

8. Define one filesystem and parse-error contract for all detectors:
   - Missing optional manifests or marker paths are ignored.
   - Invalid JSON, TOML, or `go.mod` syntax is ignored for that evidence source;
     detection continues with other valid sources for the selected language.
   - A malformed source can neither add a framework nor suppress a valid match
     from another source.
   - Any non-`os.ErrNotExist` error while reading a discovered manifest or
     requirements file, reading the root directory, or statting a marker is
     returned with path context and causes `thanos init` to fail.
   - Symlinks are followed by `os.Stat`; the resolved target must have the
     required regular-file or directory type.

9. Integrate detection in `internal/cli/cli.go` `runInit`:
   - Add `--framework` to the `init` flag set and `printHelp`.
   - Keep the existing project and command detection flow unchanged.
   - Apply the existing `--language` override first, then call
     `project.DetectFramework(ws.Root, detected.Language)`, and assign the
     result to `detected.Framework`.
   - Trim surrounding whitespace from `--framework`. A non-empty trimmed value
     overrides auto-detection and is persisted exactly as trimmed; a
     whitespace-only value is treated as no override, so auto-detection remains
     active.
   - Wrap detector failures as `detect framework: ...` and do not call
     `Workspace.Init` after a detector error.

10. Extend `internal/project/project_test.go` with table-driven temporary-root
    fixtures:
    - Cover every canonical framework and every supported declaration location.
    - Cover Composer `require-dev`, npm `devDependencies`, grouped Go requires,
      versioned Echo modules, all supported Python TOML sections and
      `requirements*.txt`, renamed Cargo dependencies, target dependencies, and
      workspace dependencies.
    - Cover unsupported/mixed-case/whitespace language selectors, absent
      evidence, malformed files, partial and wrong-type markers, comments,
      lock/source/nested-file false positives, similarly named dependencies,
      and same-source plus cross-source ambiguity.
    - Prove malformed evidence can coexist with valid evidence.
    - Prove genuine read, root-directory, and marker stat errors are returned;
      introduce a minimal package-local filesystem seam only if portable
      temporary-file fixtures cannot produce a required error.

11. Extend `internal/cli/cli_test.go`:
    - Prove auto-detection is persisted in `.thanos/settings.json`.
    - Prove `--language` is applied before framework selection.
    - Prove a trimmed non-empty `--framework` overrides detected evidence.
    - Prove whitespace-only `--framework` retains auto-detection.
    - Prove an empty result omits the JSON field.
    - Prove a detector error prevents `.thanos/settings.json` creation.
    - Stub `runExternal` to fail and prove framework-aware `runInit` never
      executes a subprocess.
    - Assert generated help contains `--framework`.

12. Update `README.md` and `Technical.md`:
    - Document `project.framework`, `--framework`, all canonical values, the
      supported root evidence sources, final-language selection, ambiguity, and
      empty/omitted behavior.
    - State that detection is local, read-only, network-free, and does not run
      package managers or project commands.
    - Add `TestFrameworkDocumentation` in `internal/cli/cli_test.go`. The test
      must read both root documents independently and require every document to
      contain assertions for:
      - `project.framework` and `--framework`;
      - every canonical framework value;
      - the root evidence manifests/markers for PHP, TypeScript, Go, Python,
        and Rust;
      - final-language selection after `--language`;
      - ambiguity producing no framework;
      - empty framework omission from settings;
      - local, read-only, network-free detection with no package-manager or
        project-command execution.
      Use a table of required literal tokens or stable phrases per document so
      one document cannot satisfy requirements for the other.

## Scope

- Root-level, selected-language framework detection during `thanos init`.
- `model.Project`, `internal/project`, `runInit`, CLI help, focused tests, and
  initialization/project-detection documentation.
- Maintained parsing dependencies required for exact `go.mod` and TOML
  semantics, with corresponding `go.mod` and `go.sum` updates.
- A single canonical framework string; ambiguity produces no value.

## Out of Scope

- Framework detection during `thanos scan` or migration of existing settings.
- Nested package/workspace framework aggregation or recursive dependency scans.
- Lock-file, vendor, generated-directory, source-code, runtime, or network
  inspection.
- Multiple framework values, confidence scores, evidence details, plugin
  registries, or arbitrary framework extensibility.
- Executing package managers, framework CLIs, build commands, or test commands
  during detection.
- Changes to build/test/lint command inference, codebase graph schemas,
  orchestrator behavior, feature artifacts, phase transitions, or state-machine
  rules.

## Risks

- TOML and module formats have aliases and workspace inheritance. Restricting
  accepted declaration locations prevents false positives but intentionally
  leaves unsupported forms undetected.
- Multiple valid frameworks may be deliberate. Returning empty is safer than
  silently inventing precedence.
- Loose text matching would classify comments, similarly named packages, and
  lock files. Structured parsers and exact normalized names are required.
- Running detection before the final language override would persist
  inconsistent metadata; CLI integration order needs a regression test.
- Filesystem permission tests can be platform-sensitive. Prefer deterministic
  error-producing fixtures or a narrow read/stat seam rather than broad
  filesystem abstraction.
- New parser dependencies increase the module surface and must be pinned,
  reviewed, and covered by the complete repository test suite.
- Documentation can drift while broad text searches continue to pass. The
  documentation contract test must check each required concept in each
  document independently.
