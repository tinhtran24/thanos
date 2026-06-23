# Coder Report — Round 3
## What Was Done
Fixed the round-2 review defect in Python framework evidence parsing. Environment markers are now parsed with a deterministic PEP 508 marker grammar instead of being accepted whenever text follows `;`.

The shared marker validator covers bare requirements, normal and parenthesized version clauses, and direct references. It validates supported marker variables, comparison and membership operators, quoted values, `and`/`or` expressions, and nested parentheses. Malformed or incomplete markers no longer create framework matches. Direct-reference URLs containing semicolons remain valid when the semicolon is part of the URL rather than a whitespace-delimited environment marker.

Added regression tests for the three malformed examples from the round-2 review and for malformed markers in every supported suffix path. Added positive coverage for compound expressions, membership operators, nested parentheses, and direct-reference URL semicolons.

The required `.thanos/codebase/summary.md` and `.thanos/codebase/graph.json` inputs are absent. Commit creation was attempted after the implementation unit, but the environment denied creation of `.git/index.lock` because `.git` is read-only. The feature implementation and round-3 fix therefore remain uncommitted.

## Files Changed
- internal/project/project.go — parse and validate complete Python PEP 508 environment markers before accepting framework evidence
- internal/project/project_test.go — add positive marker grammar coverage and malformed-marker regressions across all requirement suffix forms
- .thanos/F002-feature-auto-detect-framework-when-init-project/rounds/round-3/coder-report.md — record round-3 changes and verification evidence

## Verification
- `go build ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `go vet ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `go test ./...`: BLOCKED by sandbox permissions on the default Go build cache
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go build ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go vet ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./...`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run TestDetectFrameworkPython -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/project -run Framework -v`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go test ./internal/cli -run 'Init.*Framework|InitHelpIncludesFramework|^TestFrameworkDocumentation$' -v`: PASS
- `test -z "$(gofmt -l internal/model/model.go internal/project/project.go internal/project/project_test.go internal/cli/cli.go internal/cli/cli_test.go)"`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go mod tidy` with before/after file comparison: PASS, no changes
- `git diff --check`: PASS
- `! rg -n '"net(/http)?"|"os/exec"' internal/project`: PASS
- `git diff --exit-code -- internal/state internal/orchestrator`: PASS
- `GOCACHE=/tmp/thanos-go-cache GOPROXY=off go run ./cmd/thanos help | rg -- '--framework'`: PASS
- `git commit -m "fix: reject malformed Python framework markers"`: BLOCKED by read-only `.git/index.lock`
