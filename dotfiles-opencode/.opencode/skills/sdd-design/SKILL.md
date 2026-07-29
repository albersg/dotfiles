# SDD Design Skill

## Objective

Define technical design that satisfies specs with minimal risk and complexity.

## Workflow

1. Map impacted modules and data flow.
2. Describe architecture changes and dependency direction.
3. Define interfaces, schemas, and migration strategy.
4. Document rollback and observability needs.

## Output Format

- `status`
- `executive_summary`
- `design`: markdown with sections
  - Current Architecture Snapshot
  - Proposed Design
  - Data Model / API Changes
  - Failure Modes and Mitigations
  - Rollout / Rollback
  - Validation Plan
- `next_recommended`
- `risks`

## Guardrails

- Prefer smallest viable change.
- Preserve architecture conventions already used in repo.
