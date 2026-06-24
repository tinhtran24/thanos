# Coder Report — Round 2
## What Was Done
Fixed the round-1 review defect in Python framework detection. Requirements parsing now validates the supported PEP 508 suffix forms instead of accepting any text after a leading distribution name. It supports extras, version clauses including parenthesized clauses, direct references, environment markers, and trailing comments while rejecting malformed trailing prose, invalid extras, and incomplete version specifiers.

Added focused regression coverage for `Django(>=5)`, version lists with markers, direct references with markers, `django garbage`, malformed extras, and incomplete versions.

The required `.thanos/codebase/summary.md` and `.thanos/codebase/graph.json` inputs were absent. Commit creation was attempted after the fix, but the environment denied creation of `.git/index.lock` because `.git` is read-only. The round-1 implementation and round-2 fix therefore remain uncommitted.

## Files Changed
- internal/project/project.go — validate Python requirement suffix syntax before recording framework evidence
- internal/project/project_test.go — add positive and negative PEP 508 regression cases
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-2/coder-report.md — record round-2 work and verification evidence

## Verification
- `go build ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `go vet ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `go test ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go build ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go vet ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run Framework -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/cli -run 'Init.*Framework|InitHelpIncludesFramework|^TestFrameworkDocumentation$' -v`: PASS
- `test -z "$(gofmt -l internal/model/model.go internal/project/project.go internal/project/project_test.go internal/cli/cli.go internal/cli/cli_test.go)"`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go mod tidy` with before/after checksums: PASS, no changes
- `git diff --check`: PASS
- `! rg -n '"net(/http)?"|"os/exec"' internal/project`: PASS
- `git diff --exit-code -- internal/state internal/orchestrator`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go run ./cmd/thanos help | rg -- '--framework'`: PASS
- `git commit -m "fix: validate Python framework requirements syntax"`: BLOCKED by read-only `.git/index.lock`
