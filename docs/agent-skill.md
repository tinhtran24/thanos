Add Agent Skills System to Thanos.

Inspiration:
- addyosmani/agent-skills: https://github.com/addyosmani/agent-skills
- Use skills as executable workflows, not documentation.
- Do not copy repo code directly. Implement a compatible Thanos-native skill system.

Goal:
Thanos must support project-level skills that guide Planner, Coder, Reviewer, and Tester agents through repeatable engineering workflows.

New architecture:

internal/
  skills/
    registry.go
    loader.go
    matcher.go
    validator.go
    executor.go

.thanos/
  skills/
    using-agent-skills/
      SKILL.md
    feature-spec/
      SKILL.md
    implementation-plan/
      SKILL.md
    code-review/
      SKILL.md
    testing/
      SKILL.md
    debugging/
      SKILL.md
    refactoring/
      SKILL.md
    frontend-flow-component/
      SKILL.md
    tailwind-ui/
      SKILL.md
    tauri-desktop/
      SKILL.md

Skill format:

---
name: feature-spec
description: Create clear feature specs with acceptance criteria before implementation.
applies_to:
  - planning
  - feature
agents:
  - planner
required_evidence:
  - acceptance_criteria
  - risk_notes
  - files_to_touch
---

# Skill: Feature Spec

## When to use
Use before coding any feature.

## Workflow
1. Read ticket.
2. Identify user goal.
3. Define acceptance criteria.
4. Identify edge cases.
5. Identify files likely to change.
6. Ask user for approval.

## Anti-rationalization
| Excuse | Rebuttal |
|---|---|
| This is simple | Simple tasks still need acceptance criteria |
| I can code now | Coding before approval creates rework |

## Exit criteria
- Acceptance criteria exist.
- Risks are listed.
- User approved the spec.

Thanos behavior:
1. When a task enters Planning, load only relevant skills.
2. Do not inject all skills into context.
3. Use using-agent-skills as router.
4. Match skills by task type, agent role, files, and workflow stage.
5. Show active skills in the Task Detail right sidebar.
6. Require each skill to define exit criteria.
7. Do not allow task transition unless required evidence exists.
8. Store skill outputs into project memory.
9. Allow custom project skills in .thanos/skills.
10. Allow global skills but project skills override global ones.

Skill lifecycle:
- discovered
- matched
- activated
- running
- evidence_pending
- completed
- failed

Database models:

Skill
  id
  project_id
  name
  path
  description
  applies_to[]
  agents[]
  version
  source: project | global | builtin

SkillRun
  id
  task_id
  skill_id
  agent_session_id
  status
  evidence_json
  started_at
  completed_at

Evidence
  id
  task_id
  skill_run_id
  type
  content
  verified_by
  created_at

UI requirements:
- Task Detail shows Active Skills panel.
- Each active skill shows:
  - name
  - agent
  - status
  - required evidence
  - exit criteria
- Plan approval button disabled until required skill evidence exists.
- Review approval button disabled until testing/review evidence exists.
- User can open SKILL.md inside UI.
- User can enable/disable skills per project.

Default Thanos skills:
1. using-agent-skills
2. feature-spec
3. implementation-plan
4. safe-code-change
5. frontend-flow-component
6. tailwind-ui
7. code-review
8. testing
9. debugging
10. memory-update
11. git-worktree-safety
12. release-checklist

Important rules:
- Skills are workflow contracts.
- Skills are not long documentation dumps.
- Load only the smallest relevant skill set.
- Every skill must end with evidence.
- Every evidence item must be visible to user.
- Agents cannot claim “done” without evidence.
- Scope discipline is mandatory.
- Do not let skills override user approval gates.
- Treat downloaded third-party skills as untrusted until reviewed.

Security:
- Never auto-run shell scripts from skills.
- Never load remote skills without user approval.
- Mark external skills as untrusted.
- Show diff when installing or updating a skill.
- Skills cannot access secrets unless explicitly allowed.
