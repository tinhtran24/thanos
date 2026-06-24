# Design Review Report — Round 0
## Summary
PASS

The design is ready for implementation. It converts the broad feature request into deterministic, selected-language, root-only framework detection with canonical outputs, explicit evidence rules, ambiguity handling, consistent parse/filesystem error behavior, CLI override semantics, persistence behavior, documentation requirements, and objective verification.

All acceptance criteria have executable verification methods, and `test-strategy.yaml` covers the focused detector tests, init integration, documentation, architecture guards, formatting/module consistency, and full repository regression required by AC-1 through AC-13.

The mandated `.thanos/codebase/summary.md` and `.thanos/codebase/graph.json` are absent, so graph-first review was not possible. Relevant integration points and conventions were instead verified directly in `internal/model`, `internal/project`, `internal/cli`, `internal/workspace`, existing tests, `go.mod`, and the project rules in `.thanos/settings.json`.

## Architecture Risks

- `DetectFramework` must remain a separate integration step after the final `--language` override. Folding it into the existing `project.Detect` flow would select evidence using the pre-override language.
- Framework detection must not reuse existing recursive package-manifest discovery because the feature contract permits root evidence only.
- Invalid syntax must discard the entire malformed evidence source, while non-not-found filesystem failures must be returned with path context. Shared helpers should preserve this distinction consistently.
- The Go module parser and one maintained TOML parser are justified. Their versions must remain pinned and `go mod tidy` must produce no uncommitted module-file changes.
- `runInit` scans source before framework detection. A detector failure can therefore leave generated codebase graph artifacts, but the design correctly requires only that `Workspace.Init` is not called and `.thanos/settings.json` is not created.
- A package-local filesystem seam is acceptable only where portable fixtures cannot reliably trigger read, directory-read, or stat failures.

## Overengineering

- The design appropriately excludes recursive scans, lock/source inspection, runtime commands, network access, confidence scores, multiple framework values, plugin registries, and arbitrary detector extensibility.
- A small language dispatch with language-specific structured parsers and a match set is sufficient. A generic detector framework, precedence engine, broad virtual filesystem, or ecosystem-specific parser stack would be unnecessary.
- The requested parser dependencies are proportionate to exact `go.mod`, Cargo, Poetry, PEP 621, and PEP 735 semantics.

## Missing Requirements

No blocking feature requirement is missing or ambiguous.

The previous verification gaps are resolved:

- AC-11 now requires `TestFrameworkDocumentation` to validate every required concept independently in both `README.md` and `Technical.md`.
- AC-13 and `test-strategy.yaml` now use a failing `gofmt -l` assertion rather than a non-failing formatting diff.

The absent codebase summary and graph should be regenerated for later graph-based reviews, but their absence does not create an implementation ambiguity in this design.

## Verdict
PASS
