# Workflow Contract

Use this reference when a ticket changes workflow persistence, entities, state
transitions, UI, or the password-validation acceptance scenario.

## Contents

1. Workflow states
2. Quality-gate data
3. Entity model
4. Relationships
5. UI contract
6. Thanos architecture mapping
7. Acceptance scenario

## Workflow States

Desired parent-ticket lifecycle:

```text
Backlog
-> QA Analysis
-> Planning
-> Ready for Development
-> In Progress
-> Ready for QA
-> Done
```

Desired subtask lifecycle:

```text
Ready for Development
-> In Development
-> Ready for Review
-> Changes Requested
-> Ready for Testing
-> Testing
-> Done
```

Failed testing:

```text
Testing
-> Failed Testing
-> In Development
```

Treat these as product requirements, not permission to bypass the current
Thanos state machine. Inspect `internal/model`, `internal/state`,
`internal/orchestrator`, persistence, CLI, TUI, and tests before adding or
mapping transitions. Preserve resumability for existing feature artifacts.

## Quality-Gate Data

### QA analysis

- scope
- affected modules
- dependencies
- risks and edge cases
- unclear requirements
- ordered subtasks

### Subtask

- title
- objective
- acceptance criteria
- dependencies
- priority
- complexity
- owner role
- affected modules or files
- risks and edge cases

### Implementation evidence

- changed files
- summary
- test commands and results
- build, lint, and typecheck results
- known limitations
- revision and timestamp

### Review record

- reviewer identity
- decision
- severity-classified comments
- reviewed revision
- timestamp

### EC/test case

- EC ID
- title
- preconditions
- steps
- expected result
- actual result
- status
- linked task or acceptance criterion
- evidence

### Smoke test

- affected feature flow
- adjacent critical flows
- environment
- command or steps
- result
- evidence
- timestamp

## Entity Model

Add or extend these entities only when required by the ticket and after
inspecting the existing persistence model:

```text
Project
Ticket
Task
TaskDependency
WorkflowTransition
QAAnalysis
AcceptanceCriterion
ImplementationNote
CodeReview
TestCase
TestExecution
SmokeTestRun
Evidence
BugLink
FeatureGraphLink
```

For every new persisted entity define:

- stable identifier
- timestamps
- serialization format
- validation rules
- migration/backward-compatibility behavior
- ownership and deletion semantics
- tests for round-trip persistence

## Relationships

```text
Ticket -> many Tasks
Task -> many child Tasks
Task -> many dependencies
Task -> many acceptance criteria
Task -> many implementation notes
Task -> many code reviews
Task -> many test cases
TestCase -> many test executions
TestExecution -> many evidence items
Task -> feature graph node
Bug -> source test execution
```

Prevent cycles in task dependencies and parent-child relationships. A blocked
task must identify the incomplete dependency causing the block.

## UI Contract

Ticket detail layout:

```text
Header
Workflow Timeline
Plan / Subtasks
Task Detail Panel
Activity Timeline
```

Subtask tree requirements:

- nested subtasks
- collapse and expand
- dependency indicators
- role badges
- state badges
- completed/total progress
- blocked state for incomplete dependencies

Role-aware actions:

```text
QA Analyst:
  Start Analysis
  Generate Subtasks
  Submit Plan

Developer:
  Start Development
  Add Implementation Note
  Attach Test Result
  Submit for Review

Reviewer:
  Approve Review
  Request Changes

Tester:
  Create EC
  Run Smoke Test
  Pass Testing
  Fail Testing
  Create Bug
```

Enforce permissions and transitions in domain logic, not only by hiding buttons.
Render loading, empty, error, blocked, and completed states. Preserve keyboard
and mouse interaction conventions in the existing TUI.

## Thanos Architecture Mapping

Current Thanos concepts include:

- `model.Feature` as the ticket/feature record
- `model.ExecutionPlan` and `ExecutionChunk` as ordered implementation slices
- `model.State` and `internal/state` as deterministic workflow state
- role reports and test artifacts under `.thanos/<feature-id>/`
- events as the audit trail
- `.thanos/memory/feature-graph.json` and Markdown summary as long-term memory

Prefer extending these concepts over creating a parallel workflow system.
Preserve legacy transitions and old artifact readability when expanding states.

Feature-graph memory should surface:

- related feature plan
- previous decisions
- related tickets and bugs
- affected modules
- prior ECs
- known risks

## Acceptance Scenario

Ticket: `Add password length validation`

Required QA plan:

1. Identify current password validation.
2. Update backend validation.
3. Update frontend validation.
4. Update API error message.
5. Add unit tests.
6. Create smoke-test ECs.

Required proof:

- one subtask is implemented with unit tests
- independent review decision is recorded
- ECs map to acceptance criteria
- smoke tests cover the changed and adjacent authentication flows
- failed EC handling creates/links a bug and reopens development
- parent completion waits for all active subtasks
- feature graph records the final password-length rule and affected paths
