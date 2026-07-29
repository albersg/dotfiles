# OpenCode Skills Guide

This folder contains baseline SDD skills for OpenCode.

## Purpose

Use these skills to run Spec-Driven Development in clear phases, with traceability and predictable outputs.

## Included Skills

- `sdd-init`: initialize SDD context and detect current state.
- `sdd-explore`: analyze problem, constraints, and options.
- `sdd-propose`: create proposal with scope, non-goals, success criteria.
- `sdd-spec`: define testable requirements and acceptance criteria.
- `sdd-design`: define technical design and rollout/rollback.
- `sdd-tasks`: break work into atomic, verifiable tasks.
- `sdd-apply`: implement approved tasks in small batches.
- `sdd-verify`: validate behavior against acceptance criteria.
- `sdd-archive`: close change with final record and follow-ups.

## Recommended Flow

1. `/sdd-init`
2. `/sdd-explore <topic>`
3. `/sdd-new <change-name>` or proposal phase
4. `spec` + `design`
5. `tasks`
6. `apply` (in batches)
7. `verify`
8. `archive`

For fast planning, use `/sdd-ff <change-name>` to create planning artifacts in sequence.

## When to Use SDD

Use SDD for:

- multi-file features
- refactors affecting architecture/contracts
- risky changes requiring explicit validation
- work that benefits from phased approvals

Avoid SDD for:

- small one-file fixes
- quick typo/docs changes
- one-command maintenance tasks

## Skill Input Template (recommended)

When invoking a phase, provide:

- project path
- change name
- artifact mode (`engram`, `openspec`, `none`)
- links/paths to previous artifacts
- explicit task objective

Example prompt pattern:

```text
You are an SDD sub-agent.
Read ~/.opencode/skills/sdd-<phase>/SKILL.md first.

Context:
- Project: <path>
- Change: <change-name>
- Artifact mode: <engram|openspec|none>
- Previous artifacts: <list>

Task:
<what to do now>

Return:
- status
- executive_summary
- artifacts
- next_recommended
- risks
```

## Phase Output Contract

All phases should return, at minimum:

- `status`: `success` or `blocked`
- `executive_summary`: concise summary of outcome
- `next_recommended`: single next command/action
- `risks`: known risks/blockers

Additional sections are phase-specific and defined in each `SKILL.md`.

## Artifact Modes

- `engram`: persistent memory artifacts (recommended if available).
- `openspec`: file-based artifacts inside project.
- `none`: inline-only output, no artifact persistence.

## Apply and Verify Strategy

- Implement in small batches (for example, 1-3 tasks at a time).
- After each batch, run targeted checks first, then broader checks.
- Do not mark phase complete without command evidence.

## Safety and Quality Rules

- Keep scope tight; avoid speculative refactors.
- Do not edit unrelated files.
- Preserve repository architecture and conventions.
- Prefer reversible changes and explicit rollback notes.
- Treat security and secrets as first-class concerns.

## Customization

To tailor these skills:

1. Edit each `SKILL.md` with your domain constraints.
2. Add project-specific validation commands.
3. Add framework skills (React, TypeScript, Python, etc.) as needed.
4. Keep a stable output format so orchestrators remain predictable.

## Install / Sync

Run from repository root:

```bash
./opencode/install.sh
```

This syncs `opencode/skills/` to `~/.opencode/skills/`.
