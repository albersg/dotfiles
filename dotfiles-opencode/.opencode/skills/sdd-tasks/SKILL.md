# SDD Tasks Skill

## Objective

Break design into executable implementation tasks with clear sequencing.

## Workflow

1. Convert requirements/design into task groups.
2. Order tasks by dependency and risk.
3. Add validation step for each group.
4. Mark parallelizable vs sequential tasks.

## Output Format

- `status`
- `executive_summary`
- `tasks`: numbered markdown checklist
  - Setup
  - Implementation
  - Tests
  - Validation
  - Documentation
- `next_recommended`
- `risks`

## Guardrails

- Each task must be atomic and verifiable.
- Avoid broad tasks like "refactor codebase".
