# SDD Init Skill

## Objective

Initialize Spec-Driven Development context for a repository.

## Workflow

1. Confirm project root and current branch.
2. Detect existing planning artifacts (proposal/spec/design/tasks).
3. Determine artifact store mode (`engram`, `openspec`, or `none`).
4. Output initial state and recommended next command.

## Output Format

- `status`: success | blocked
- `executive_summary`: one paragraph
- `artifacts`: list of discovered artifacts
- `state`: current phase completion map
- `next_recommended`: one command
- `risks`: list

## Guardrails

- Do not write implementation code in this phase.
- Do not create speculative artifacts.
- If artifact store is missing, continue with `none` and note limitation.
