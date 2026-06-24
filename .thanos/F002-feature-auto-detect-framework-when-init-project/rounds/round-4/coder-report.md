# Coder Report — Round 4
## What Was Done
Fixed the round-3 Rust evidence defect. Cargo dependency declarations now contribute framework evidence only when their TOML value is a string or table, as required by the feature contract. Boolean, numeric, array, and other unsupported value types are ignored. Existing table behavior remains unchanged: a string `package` field selects a renamed crate, otherwise the dependency key is used.

Added regression coverage proving that boolean, numeric, and array declarations using supported framework names do not produce matches.

This round resumed an interrupted session: the implementation, tests, and a partial report were already present in the working tree. They were inspected against the round-3 review before verification. Unrelated existing UI, state-machine, and orchestrator changes were not modified.

## Files Changed
- internal/project/project.go — restrict Rust dependency evidence to string and table declarations
- internal/project/project_test.go — add negative tests for boolean, numeric, and array Cargo dependency values
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-4/coder-report.md — record round-4 changes and verification evidence

## Verification
- `go build ./...`: PASS
- `go vet ./...`: PASS
- `go test ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run TestDetectFrameworkRust -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run Framework -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/cli -run 'Init.*Framework|InitHelpIncludesFramework|^TestFrameworkDocumentation$' -v`: PASS
- `test -z "$(gofmt -l internal/model/model.go internal/project/project.go internal/project/project_test.go internal/cli/cli.go internal/cli/cli_test.go)"`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go mod tidy` with before/after hashes for `go.mod` and `go.sum`: PASS, no changes
- `git diff --check`: PASS
- `! rg -n '"net(/http)?"|"os/exec"' internal/project`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go run ./cmd/thanos help | rg -- '--framework'`: PASS
- `git diff --exit-code -- internal/state internal/orchestrator`: FAIL due to pre-existing unrelated state-machine and orchestrator diffs present before round-4 work; round 4 did not modify those files
- `git commit -m "fix: reject unsupported Cargo dependency values"`: BLOCKED because the managed environment denied creation of `.git/index.lock`
