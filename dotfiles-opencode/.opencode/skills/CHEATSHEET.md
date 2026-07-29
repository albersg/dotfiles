# OpenCode SDD Cheatsheet

Quick reference for daily use.

## Core Commands

- `/sdd-init` -> initialize SDD context
- `/sdd-explore <topic>` -> explore idea and constraints
- `/sdd-new <change-name>` -> start proposal
- `/sdd-continue [change-name]` -> create next artifact
- `/sdd-ff [change-name]` -> proposal + spec + design + tasks
- `/sdd-apply [change-name]` -> implement approved tasks
- `/sdd-verify [change-name]` -> validate against criteria
- `/sdd-archive [change-name]` -> close and record final state

## Recommended Path

1. Init: `/sdd-init`
2. Explore: `/sdd-explore`
3. Plan: `/sdd-new` or `/sdd-ff`
4. Build: `/sdd-apply`
5. Validate: `/sdd-verify`
6. Close: `/sdd-archive`

## When to Use SDD

Use SDD for:

- multi-file features
- architecture-impacting refactors
- API/schema contract changes
- risky changes requiring strict verification

Skip SDD for:

- one-file small fixes
- typo/docs-only edits
- trivial maintenance commands

## Artifact Mode Decision

- `engram` -> best if available (persistent memory artifacts)
- `openspec` -> use when you want artifacts as project files
- `none` -> inline-only output, fastest setup

## Per-Phase Output (minimum)

- `status`: success | blocked
- `executive_summary`
- `next_recommended`
- `risks`

## Apply Phase Rules

- implement in small batches (1-3 tasks)
- run targeted checks after each batch
- then run broader validation
- no unrelated file edits

## Verify Phase Rules

- map acceptance criteria -> evidence -> result
- include executed commands
- do not mark success without evidence

## Prompt Template for Sub-Agents

```text
You are an SDD sub-agent.
Read ~/.opencode/skills/sdd-<phase>/SKILL.md first.

Context:
- Project: <path>
- Change: <change-name>
- Artifact mode: <engram|openspec|none>
- Previous artifacts: <list>

Task:
<goal for this phase>

Return:
- status
- executive_summary
- artifacts
- next_recommended
- risks
```

## Install / Sync

```bash
./opencode/install.sh
```

This syncs local config and `opencode/skills/*` to `~/.opencode/skills/`.
