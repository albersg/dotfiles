# SDD Spec Skill

## Objective

Convert proposal into precise behavioral specifications.

## Workflow

1. Derive functional requirements from proposal.
2. Define input/output contracts and error cases.
3. Add acceptance criteria per requirement.
4. Identify test scenarios that prove correctness.

## Output Format

- `status`
- `executive_summary`
- `specs`: markdown with sections
  - Functional Requirements
  - Non-Functional Requirements
  - API/Contract Changes
  - Edge Cases
  - Acceptance Criteria
  - Test Matrix
- `next_recommended`
- `risks`

## Guardrails

- Requirements must be testable.
- Distinguish MUST vs NICE-TO-HAVE.
