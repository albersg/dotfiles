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
