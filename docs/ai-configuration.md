# AI Configuration for Neovim

Every conventional AI plugin in this configuration is **disabled**. That is a
decision, not an oversight: `lua/config/lazy.lua` states "AI integrations are
intentionally disabled in this configuration" and `lua/plugins/disabled.lua`
enforces it plugin by plugin.

The one assistant that is active is the `pi` floating chat, wired in
`lua/plugins/pi-teacher.lua`.

## Table of Contents

- [Disabled AI Plugins](#disabled-ai-plugins)
- [The Active Assistant](#the-active-assistant)
- [Enabling a Disabled Plugin](#enabling-a-disabled-plugin)
- [Required CLI Tools](#required-cli-tools)
- [Pi Agent Skills](#pi-agent-skills)
- [OfficeCLI CLI Binary](#officecli-cli-binary)
- [Configurations That Are Kept Anyway](#configurations-that-are-kept-anyway)

---

## Disabled AI Plugins

| Plugin | Description | State |
|--------|-------------|-------|
| **Claude Code.nvim** | Claude Code integration (`coder/claudecode.nvim`) | Disabled |
| **OpenCode.nvim** | OpenCode integration (`NickvanDyke/opencode.nvim`) | Disabled |
| **Avante.nvim** | AI coding assistant (`yetone/avante.nvim`) | Disabled |
| **CopilotChat.nvim** | GitHub Copilot chat (`CopilotC-Nvim/CopilotChat.nvim`) | Disabled |
| **copilot.lua** | GitHub Copilot inline suggestions | Disabled |
| **CodeCompanion.nvim** | Multi-provider assistant (`olimorris/codecompanion.nvim`) | Disabled |
| **Gemini.nvim** | Google Gemini integration (`jonroosevelt/gemini-cli.nvim`) | Disabled |

None of these is enabled "by default", and no plugin spec in `lua/plugins/` sets
`enabled = true` at the plugin level. Any documentation or keymap table that
describes Copilot completions, an OpenCode keymap group, or a Claude Code default
describes a configuration that does not exist.

## The Active Assistant

| Keys | Description | Mode |
|------|-------------|------|
| `<leader>a` | Open the `pi` floating chat | n |

`pi-teacher.lua` also records the command lines you type interactively into
`~/.local/share/nvim/teacher-usage.json`, writes a summary to
`~/.cache/nvim/teacher-usage.md`, and passes it to a remote model through
`--append-system-prompt` on every launch (`lua/teacher/usage.lua:29,32,251`).
That is worth knowing before using it on a machine that handles anything
sensitive.

## Enabling a Disabled Plugin

Plugin states live in two places, and both matter:

```bash
nvim ~/.config/nvim/lua/plugins/disabled.lua
```

1. Remove the plugin's entry from `disabled.lua`, **or** set `enabled = true` in
   that plugin's own spec under `lua/plugins/`.
2. Restart Neovim.

> **Only enable one at a time, and expect a keymap collision.** Several of these
> plugins claim `<leader>a` and its children, while `pi-teacher` already binds
> `<leader>a` as a direct action rather than as a group. The first plugin you
> re-enable will fight it, so reconcile those keys in the same change.

## Required CLI Tools

The installer fetches two of the CLIs unconditionally, even though the plugins
that use them are disabled:

| Tool | Installation Command |
|------|---------------------|
| Claude Code CLI | `curl -fsSL https://claude.ai/install.sh \| bash` |
| OpenCode CLI | `curl -fsSL https://opencode.ai/install \| bash` |
| Gemini CLI | `npm install -g @google/gemini-cli` |

The CLI installs are non-fatal and skipped on Termux. If you never enable the
plugins, those two commands are effort spent on something that stays off.

> Some services require API keys. Check each plugin's documentation for details.

## Pi Agent Skills

The installer provisions three Pi skill packages under `~/.pi/agent/skills/`. Pi
is the only agent platform targeted: nothing is installed for Claude or OpenCode.

| Skill | Destination | Pinned source |
|-------|-------------|---------------|
| `security-audit` | `~/.pi/agent/skills/security-audit` | Cloudflare `security-audit-skill` commit `c1c8a8c1471069fb0e188eeaff69b8e8db6564a8` (codeload ZIP) |
| `archify` | `~/.pi/agent/skills/archify` | tt-a1i `archify` commit `9e35d2b0b39b155553ba9fcfe0b4f2a5198dd993` (codeload ZIP) |
| `officecli` | `~/.pi/agent/skills/officecli` | iOfficeAI `OfficeCLI` commit `ffa8a0afbe2e9686abd636368e3da38c50f22131` (`LICENSE` and `skills/officecli/SKILL.md` from `raw.githubusercontent.com` at that commit) |

The step needs network access: it downloads each artifact from
`codeload.github.com` or `raw.githubusercontent.com`. Every download is checked against a
hard-coded SHA-256 before anything is staged, and a mismatch or a failed
transfer fails the step without touching the destination. Nothing from the
upstream repositories is executed; only the archive and file bytes are verified,
extracted and copied. ZIP extraction rejects entries that would escape the
staging directory, and files keep the permission bits the archive recorded.

The step is idempotent and never discards user data. A destination directory
that already exists is reported and left untouched, so a locally modified or
newer skill survives an installer run; only missing destinations are installed.

### Updating the pinned skill sources

The pins live in `installer/internal/tui/agent_skills.go`. To move a skill to a
new upstream revision:

1. Pick the immutable commit and build the codeload archive URL (or the
   `raw.githubusercontent.com` URL at the pinned commit for the OfficeCLI files).
2. Download the artifact and compute its SHA-256:
   `curl -fsSL -o /tmp/skill.zip <url> && sha256sum /tmp/skill.zip`.
3. Update the URL and the matching checksum constant together, then re-run
   `cd installer && go test ./internal/tui`. `TestAgentSkillPackagesArePinned`
   asserts the exact URLs, checksums and layout, so it must be updated in the
   same change and then guards against future drift.
4. Confirm the archive layout still matches the constants: the Cloudflare
   package is `skills/security-audit/` with the archive root `LICENSE`, and the
   Archify package is the `archify/` subtree of its whole-repository archive.

## OfficeCLI CLI Binary

The installer also provisions the OfficeCLI executable itself, separately from
the `officecli` Pi skill. It comes from the versioned GitHub release below and is
verified against a hard-coded SHA-256 before it is made executable. The unpinned
`d.officecli.ai/install.sh` script is deliberately not used: it always fetches
whatever is current, so it cannot be verified before it runs.

Release base URL: `https://github.com/iOfficeAI/OfficeCLI/releases/download/v1.0.152/`

| Platform | Asset | SHA-256 |
|----------|-------|---------|
| Linux x86_64 | `officecli-linux-x64` | `e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9` |
| Linux arm64 | `officecli-linux-arm64` | `bc06deaa0ad931f5208717a40b94018dc44cdff0d8eefa842c4f4daf89fb35a8` |
| Linux Alpine x86_64 | `officecli-linux-alpine-x64` | `390e246303bf43b4739e3195e9b171a660223b4c11218b65894d5fadf9a52755` |
| Linux Alpine arm64 | `officecli-linux-alpine-arm64` | `65c65e05100bac1376e23f6ca97086a046afdfcaf50f2bc215a8c139b3bc22ba` |
| macOS x86_64 | `officecli-mac-x64` | `5071abef56c1d4a4d60e28ed12bc66183d8dc6a9783529c3f1a9cf6bdfe6c2dd` |
| macOS arm64 | `officecli-mac-arm64` | `e2ed6eba5cd46d6800139f2835097828b8ccd7c8c9b679463b50e45ba2f1dbf5` |

The Alpine asset is chosen when `/etc/alpine-release` exists, because Alpine
needs the musl-linked build. The step needs network access and downloads with
`curl`; the bytes are checked against the matching SHA-256 and only then made
executable and renamed into `~/.local/bin/officecli`, so a failed transfer or a
mismatch leaves neither a target nor a staging directory behind. The step is
idempotent and preserves user data: an existing `~/.local/bin/officecli` is
reported and left untouched. Termux is skipped with a clear log because the
release publishes no Android asset, and any other unsupported OS/architecture
fails closed rather than installing an unverified or incompatible binary.

### Updating the pinned OfficeCLI release

The pins live in `installer/internal/tui/officecli.go`. To move to a new release:

1. Pick the release tag and confirm the asset names on the release page. The
   current assets are the six names in the table above; a renamed or added asset
   needs a matching change to the platform mapping.
2. Download each asset and compute its SHA-256:
   `curl -fsSL -o /tmp/officecli <url> && sha256sum /tmp/officecli`.
3. Update `officeCLIReleaseVersion`/`officeCLIReleaseBaseURL` and the asset
   name/checksum constants together, then re-run
   `cd installer && go test ./internal/tui`. `TestOfficeCLIAssetsArePinned`
   asserts the exact names, URLs and checksums, so it must be updated in the
   same change and then guards against future drift.

## Configurations That Are Kept Anyway

The configuration files for the disabled plugins are still present and still
maintained: `avante.lua`, `code-companion.lua`, `copilot-chat.lua`, `copilot.lua`,
`gemini.lua`, `opencode.lua` and `claude-code.lua`. `lazy-lock.json` still locks
several of them, so they remain pinned and reproducible even though they never
load.

That has a cost worth naming: three of them carry a near-verbatim copy of the
same Spanish assistant persona, and `lua/plugins/ui.lua` still contains lualine
hooks that only fire for `codecompanion` filetypes, which is unreachable while the
plugin is disabled. Keeping them is a choice about being able to switch back
quickly, not a requirement of the active configuration.
