# SDD Verify Skill

## Objective

Validate that implementation satisfies specs and quality gates.

## Workflow

1. Run required validation commands from project docs.
2. Execute targeted regression tests for changed areas.
3. Compare delivered behavior with acceptance criteria.
4. Produce pass/fail report with evidence.

## Output Format

- `status`
- `executive_summary`
- `verification_matrix`: criteria -> evidence -> result
- `commands_executed`
- `failures` (if any)
- `next_recommended`
- `risks`

## Guardrails

- Do not claim success without command output evidence.
- If checks fail, report root cause and remediation plan.
