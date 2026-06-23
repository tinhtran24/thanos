# Coder Report — Round 1
## What Was Done
Implemented deterministic root-only framework detection for PHP, TypeScript, Go, Python, and Rust. Added canonical framework persistence, ambiguity handling, structured TOML and `go.mod` parsing, filesystem/parse error semantics, `thanos init` auto-detection and override behavior, CLI help, documentation, and focused regression tests.

Commit creation was attempted after the detector unit, but the environment denied creation of `.git/index.lock` because `.git` is read-only. All implementation changes therefore remain uncommitted.

## Files Changed
- go.mod — add maintained TOML and Go module parser dependencies
- go.sum — record parser dependency checksums
- internal/model/model.go — add optional `project.framework`
- internal/project/project.go — add language-scoped framework detectors and error handling
- internal/project/project_test.go — cover canonical frameworks, evidence locations, ambiguity, false positives, malformed evidence, and filesystem failures
- internal/cli/cli.go — integrate final-language detection, `--framework`, persistence, and help
- internal/cli/cli_test.go — cover init integration, overrides, omission, detector failure, no subprocess execution, help, and documentation
- README.md — document framework detection behavior and evidence
- Technical.md — document framework metadata and detection contract
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-1/coder-report.md — record implementation and verification evidence

## Verification
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go build ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go vet ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run Framework -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/cli -run 'Init.*Framework|InitHelpIncludesFramework' -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/cli -run '^TestFrameworkDocumentation$' -v`: PASS
- `test -z "$(gofmt -l internal/model/model.go internal/project/project.go internal/project/project_test.go internal/cli/cli.go internal/cli/cli_test.go)"`: PASS
- `GOPROXY=off GOCACHE=/tmp/thanos-go-cache go mod tidy` with before/after checksums: PASS, no changes
- `! rg -n '"net(/http)?"|"os/exec"' internal/project`: PASS
- `git diff --exit-code -- internal/state internal/orchestrator`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go run ./cmd/thanos help | rg -- '--framework'`: PASS
- `git diff --check`: PASS
- `git commit -m "feat: detect frameworks from root project evidence"`: BLOCKED by read-only `.git/index.lock`
