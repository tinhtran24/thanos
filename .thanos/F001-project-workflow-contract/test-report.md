# Test Report

## ECs

### EC-1: Active workflow follows QA, development, review, and testing

- Linked criterion: New tickets run planning, development, review, and testing in order.
- Preconditions: Go 1.25 toolchain and project dependencies available.
- Steps: Run the state and orchestrator tests, including review and test failure paths.
- Expected result: Reviewer runs before tester; failed gates reopen development and stop.
- Actual result: Focused tests passed.
- Status: Passed
- Evidence: `go test ./internal/state ./internal/orchestrator`

### EC-2: Round metadata is removed from the active protocol

- Linked criterion: Failed review or testing reopens development without creating a numeric round.
- Preconditions: Updated model, CLI, prompts, TUI, and artifact paths.
- Steps: Run all package tests and search active code/templates for round fields and paths.
- Expected result: No active round state, retry budget, round artifact path, or continue-round command.
- Actual result: Full tests passed; remaining legacy phase constants are compatibility-only.
- Status: Passed
- Evidence: `go test ./...`

### EC-3: EC work-item names replace round labels

- Linked criterion: Work items use names such as Feature 001-EC1 Implement ABC instead of round labels.
- Preconditions: Role event and chat rendering changes applied.
- Steps: Run orchestrator and chat tests for work-item naming.
- Expected result: Runner labels, events, prompts, and chat show the EC name.
- Actual result: Tests passed with `Feature 001-EC1 Implement ABC`.
- Status: Passed
- Evidence: `TestWorkItemNameUsesFeatureAndEC`, `TestRoleStartShowsWorkItemInsteadOfRound`

### EC-4: Legacy artifacts migrate without deletion

- Linked criterion: Existing round-based artifacts can resume through a non-destructive compatibility migration.
- Preconditions: A legacy `rounds/round-2/coder-report.md` exists.
- Steps: Load runtime state through `EnsureRuntime`.
- Expected result: Latest report is copied to `implementation-note.md`; legacy file remains.
- Actual result: Workspace migration test passed.
- Status: Passed
- Evidence: `TestEnsureRuntimeMigratesLatestLegacyRoundArtifacts`

### EC-5: Missing prompt directory is recreated

- Linked criterion: Workflow prompts create the required plan, implementation, review, EC, smoke-test, and memory evidence.
- Preconditions: `.thanos/prompts/` is removed after workspace initialization.
- Steps: Run the workflow from Planning.
- Expected result: The orchestrator recreates the directory and writes the planner prompt.
- Actual result: Regression test passed.
- Status: Passed
- Evidence: `TestRunUsesPlanDevelopmentReviewTestingFlow`

### EC-6: Manual completion uses workflow evidence gates

- Linked criterion: New tickets run planning, development, review, and testing in order.
- Preconditions: Ticket is in reviewing with implementation and test evidence but review is pending.
- Steps: Run `./bin/thanos done F001-project-workflow-contract`.
- Expected result: Completion is rejected with the EC name and missing approval gate.
- Actual result: Rejected with `Feature 001-EC1 Apply project workflow contract review is not approved`.
- Status: Passed
- Evidence: `TestValidateCompletionEvidenceExplainsPendingReview`, local CLI smoke command

## Smoke Tests

- Flow: Initialize, plan, develop, approve review, test, update memory, complete.
- Result: Passed in orchestrator integration tests.
- Evidence: `TestRunUsesPlanDevelopmentReviewTestingFlow`
- Flow: Failed review and failed testing.
- Result: Passed; both reopen development and require a later run.
- Evidence: `TestFailedReviewReopensDevelopmentAndStops`, `TestFailedTestingReopensDevelopmentAndStops`
- Flow: Multi-EC sequencing.
- Result: Passed; every EC creates implementation, review, and test evidence.
- Evidence: `TestRunCyclesEachChunkThroughReviewAndTesting`

## Bugs

No failed ECs.

## Verdict

PASS
