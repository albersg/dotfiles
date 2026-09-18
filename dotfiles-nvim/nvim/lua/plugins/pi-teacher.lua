-- pi-teacher: floating "Neovim teacher" chat powered by the `pi` CLI.
-- Opened with <leader>a (toggle) or the :Teacher command.
-- pi runs in clean/read-only mode: no extensions/packages, no skills,
-- no prompt templates, no themes, no session persistence; only
-- read/grep/find/ls tools.
-- On every open the live keymap section is rebuilt (lua/teacher/dump.lua)
-- and cached at ~/.cache/nvim/teacher-live-keymaps.md so an already-running
-- chat can re-read the freshest dump with its `read` tool.
--
-- UX determinism rules:
--   * Focusing the float ALWAYS lands you in terminal mode, ready to
--     type: <leader>a shows it with a synchronous startinsert; ANY
--     WinEnter into the float (mouse click from another window, <C-w>w
--     cycling, reopen) re-enters terminal mode; a click inside the float
--     runs the builtin click and goes straight to terminal mode; and
--     i/a/o in normal mode use the core terminal-buffer default (they
--     enter Terminal mode).
--   * Deliberate normal-mode scrolling inside the float (j/k, <C-u>,
--     <ScrollWheel>) is never overridden: those paths never fire WinEnter
--     or <LeftMouse>, so normal mode is a stable place to read/scroll.
--     Escape back to typing is the obvious one-key move: i.
--   * <C-q> (terminal AND normal mode) hides the float and restores
--     focus, and is processed before any dump/cache work.
--
-- NOTE: the float is implemented with plain nvim floating windows +
-- termopen, NOT Snacks.terminal. Verified on nvim 0.12.4: quitting nvim
-- with a Snacks TERMINAL float visible hangs `:qa!` indefinitely (even a
-- plain `sleep 300` job), while a plain float terminal exits cleanly.
-- Keeping the job alive in a hidden buffer behaves like any other terminal.

-- Model choice (verified by actually running the candidate on pi 0.85.1).
-- The teacher overrides the user's slow/expensive default (openai-codex/
-- gpt-5.6-luna at thinking high, ~3.2-7.1s one-shot) with a flash-class
-- model. The user explicitly chose this one over the previous latency-based
-- pick (glm-5.3-flash): deepseek-v4.1-flash is fast enough with thinking OFF,
-- and the "off" level removes the reasoning pass that made it slow before.
--
-- Measured one-shot with the assembled teacher prompt/flags (this session,
-- 2026-09-13, wall time in seconds):
--   opencode-go/deepseek-v4.1-flash, thinking off -> chosen
--     (3.43/4.35/4.90, median 4.35).
--   opencode-go/glm-5.3-flash, thinking low (previous choice, re-measured)
--     (3.34/5.06/6.11, median 5.06).
--   => within run-to-run noise, deepseek-v4.1-flash + thinking off is NOT
--      slower than glm; the old 7.79/8.29 figure was measured with the
--      default reasoning level before this change.
-- Previously rejected (auth reports "ready" but inference fails at launch):
--   openai-codex/gpt-5.4-mini        -> 403: not supported with a ChatGPT (Codex) account.
--   openai-codex/gpt-5.3-codex-spark -> same 403 (Codex with a ChatGPT account).
--   opencode-go/deepseek-v4-flash    -> 403 RegionError (China-only hosting).
-- --provider is passed explicitly because bare --model ids can be ambiguous
-- across providers (gpt-5.6-luna exists in both openai-codex and opencode-go).
-- To change the model later, edit TEACHER_PROVIDER / TEACHER_MODEL /
-- TEACHER_THINKING below; run `pi --list-models` to see the catalog.
local TEACHER_PROVIDER = "opencode-go"
local TEACHER_MODEL = "deepseek-v4.1-flash"
-- "off" is accepted by pi (`--help`: off, minimal, low, medium, high, xhigh,
-- max). The user wants NO reasoning for the teacher; with thinking off this
-- model matches the previous glm pick (see the timings above).
local TEACHER_THINKING = "off"

local dump = require("teacher.dump")
local usage = require("teacher.usage")
local plugins = require("teacher.plugins")

-- PI_TEACHER_BIN exists only so headless tests can plug in a stub binary;
-- real sessions always resolve the npm install below.
local pi_bin = vim.env.PI_TEACHER_BIN or vim.fn.expand("~/.npm-global/bin/pi")
local teacher_dir = vim.fn.expand("~/.config/nvim/teacher")

-- ---------------------------------------------------------------------------
-- Float geometry: explicit and tweakable. Recomputed from the CURRENT
-- terminal size on every open; the hard grid bounds (border included)
-- always win over the minimums, so the float can never exceed the screen.
-- ---------------------------------------------------------------------------
local TEACHER_WIDTH_RATIO = 0.9 -- share of the terminal columns
local TEACHER_HEIGHT_RATIO = 0.92 -- share of the usable editor rows
local TEACHER_MIN_WIDTH = 56 -- floor for tiny terminals
local TEACHER_MIN_HEIGHT = 10

-- Title must stay <= ~46 chars: at 80 columns the float is ~72 wide and
-- nvim truncates over-long centered titles. It teaches the ONE move the
-- UX relies on: i to (re)enter typing. Normal mode inside the float is
-- standard nvim (<C-\><C-n>, j/k to scroll, i to type again).
local TEACHER_TITLE = " pi-teacher · i escribe · <C-q> atrás "

--- Read a text file, returns "" on failure.
---@param path string
---@return string
local function read_file(path)
  local f = io.open(path, "r")
  if not f then
    return ""
  end
  local content = f:read("*a")
  f:close()
  return content
end

-- Teacher state: buffer (terminal), floating window. The buffer persists
-- across toggles so the chat history survives hiding.
local state = { buf = nil, win = nil }

-- Forward declarations: setup_teacher_buffer closes over open_teacher.
local open_teacher
local setup_teacher_buffer

--- Geometry for the current editor grid, clamped to it (border included).
---@return integer width, integer height, integer row, integer col
local function compute_geometry()
  local cols = vim.o.columns
  -- Usable rows: full lines minus the cmdline band and the statusline row
  -- (floats may still overlap the tabline at the top; nvim centers inside
  -- the usable band, so the tabline stays visible).
  local rows = vim.o.lines - vim.o.cmdheight - (vim.o.laststatus > 0 and 1 or 0)
  local width = math.floor(cols * TEACHER_WIDTH_RATIO)
  local height = math.floor(rows * TEACHER_HEIGHT_RATIO)
  -- Minimums first, hard grid bounds last: the grid always wins, so the
  -- float never breaks on a tiny terminal.
  width = math.min(math.max(width, TEACHER_MIN_WIDTH), math.max(cols - 2, 1))
  height = math.min(math.max(height, TEACHER_MIN_HEIGHT), math.max(rows - 2, 1))
  local row = math.max(0, math.floor((rows - height) / 2))
  local col = math.max(0, math.floor((cols - width) / 2))
  return width, height, row, col
end

--- True while the teacher's pi process is still running in a valid buffer.
local function teacher_alive()
  if not (state.buf and vim.api.nvim_buf_is_loaded(state.buf) and vim.bo[state.buf].buftype == "terminal") then
    return false
  end
  -- nvim 0.12+: b:terminal was removed; the job id lives in b:terminal_job_id.
  local job = vim.b[state.buf].terminal_job_id
  if not job or job <= 0 then
    return false
  end
  -- jobwait(timeout 0) returns -1 for a job still running.
  local ok, running = pcall(vim.fn.jobwait, { job }, 0)
  return ok and running and running[1] == -1
end

--- First non-floating window in the current tabpage (focus fallback).
local function first_normal_win()
  for _, win in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
    if vim.api.nvim_win_is_valid(win) and vim.api.nvim_win_get_config(win).relative == "" then
      return win
    end
  end
  return nil
end

--- Hide the float and restore focus deterministically. Order matters:
--- exit terminal/insert mode WHILE the float still has focus, close it,
--- then jump back to the window <leader>a was pressed in. If that window
--- was closed meanwhile, land on the first real (non-floating) window.
local function close_float()
  if not (state.win and vim.api.nvim_win_is_valid(state.win)) then
    return
  end
  vim.cmd.stopinsert()
  local prev = vim.w[state.win].teacher_prev_win
  vim.api.nvim_win_close(state.win, true)
  state.win = nil
  if prev and vim.api.nvim_win_is_valid(prev) and prev ~= vim.api.nvim_get_current_win() then
    pcall(vim.fn.win_gotoid, prev)
  end
  -- Focus fallback: focus must not be left on another float (e.g. a
  -- LazyVim notifier); land on a real file window instead.
  local cur = vim.api.nvim_get_current_win()
  if vim.api.nvim_win_is_valid(cur) and vim.api.nvim_win_get_config(cur).relative ~= "" then
    local target = first_normal_win()
    if target and target ~= cur then
      pcall(vim.fn.win_gotoid, target)
    end
  end
end

--- Open (or reshow) the floating window for the teacher buffer.
--- Geometry is recomputed from the current terminal size on every call.
local function show_float(prev_win)
  local width, height, row, col = compute_geometry()
  local cfg = vim.api.nvim_open_win(state.buf, true, {
    relative = "editor",
    width = width,
    height = height,
    row = row,
    col = col,
    border = "rounded",
    title = TEACHER_TITLE,
    title_pos = "center",
    zindex = 60,
  })
  state.win = cfg
  vim.wo[cfg].cursorline = false
  vim.wo[cfg].wrap = true
  vim.w[cfg].teacher_prev_win = prev_win
end

--- Buffer-local focus/mode rules, registered once per buffer creation.
setup_teacher_buffer = function(buf)
  -- One-key escape hatch from ANYWHERE in the float (terminal mode
  -- included): <C-q> hides the float and returns to the file.
  -- Buffer-local, so no other terminal ever gets it.
  vim.keymap.set("t", "<C-q>", open_teacher, { buffer = buf, desc = "Teacher: volver al archivo" })
  -- Same escape hatch after <C-\><C-n> (normal mode inside the float).
  vim.keymap.set("n", "<C-q>", open_teacher, { buffer = buf, desc = "Teacher: volver al archivo" })

  -- Click inside the float (normal mode): run the builtin click first —
  -- the 'n' flag keeps mappings (ours) out of the way, so the cursor
  -- lands where the user clicked — then go straight to terminal mode so
  -- typing reaches pi. Clicks from ANOTHER window don't hit this
  -- buffer-local mapping; the WinEnter autocmd below covers those.
  vim.keymap.set("n", "<LeftMouse>", function()
    vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes("<LeftMouse>", true, false, true), "nx", false)
    vim.cmd("startinsert")
  end, { buffer = buf, desc = "Teacher: clic = enfocar y escribir" })

  -- ANY WinEnter into the float (<leader>a, mouse click from another
  -- window, <C-w>w cycling, reopen) re-enters terminal mode. Normal-mode
  -- scrolling inside the float never fires WinEnter, so it is never
  -- overridden. The buftype check skips the synchronous open_win WinEnter
  -- in the fresh-spawn path: the buffer is not a terminal until termopen
  -- runs a few lines below, and the explicit startinsert after termopen
  -- covers that entry deterministically.
  vim.api.nvim_create_autocmd("WinEnter", {
    buffer = buf,
    desc = "pi-teacher: entering the float (re)enters terminal mode",
    callback = function()
      if vim.bo[buf].buftype == "terminal" then
        vim.cmd("startinsert")
      end
    end,
  })
end

--- Launch the pi teacher terminal (toggleable).
open_teacher = function()
  -- Toggle: float visible -> hide it and restore focus to the file.
  -- (Do this BEFORE any cache work: <C-q> must be instant, no dump build.)
  if state.win and vim.api.nvim_win_is_valid(state.win) then
    close_float()
    return
  end

  -- Refresh the live keymap cache on every SHOW (pure Lua, no subprocess):
  -- even a persistent chat can read the freshest dump via the `read` tool.
  -- write_cache() already returns the built section; reuse it so the dump
  -- is only ever built ONCE per open (measured: ~4-11ms per build).
  -- The usage section (cumulative command/search history) is likewise
  -- rebuilt + cached on every show (~0.2ms: a small table read + sort).
  -- The plugins section (live enabled+installed plugin inventory) is built
  -- from lazy.nvim runtime state on every show (measured <1ms: one pass over
  -- the spec map) so a re-disabled plugin disappears automatically.
  local live_section = dump.write_cache()
  local usage_section = usage.write_cache()
  local plugins_section = plugins.write_cache()
  local from_win = vim.api.nvim_get_current_win()

  if not teacher_alive() then
    -- Dead chat from a previous run: wipe its buffer so the next spawn
    -- starts a FRESH chat (teacher_alive already returned false here).
    if state.buf and vim.api.nvim_buf_is_valid(state.buf) then
      pcall(vim.api.nvim_buf_delete, state.buf, { force = true })
    end
    state.buf = nil

    if vim.fn.executable(pi_bin) ~= 1 then
      vim.notify(("pi-teacher: binario pi no encontrado (%s)"):format(pi_bin), vim.log.levels.ERROR)
      return
    end

    -- (Re)start: fresh pi process with the freshly assembled prompt.
    -- Section order (matches system-prompt.md): instructions -> keymaps
    -- knowledge base -> active plugins inventory -> usage history patterns
    -- -> live keymap dump LAST (final override: it wins over everything
    -- above, so it must stay the last section).
    local prompt = read_file(teacher_dir .. "/system-prompt.md")
      .. "\n\n---\n\n"
      .. read_file(teacher_dir .. "/keymaps.md")
      .. "\n\n---\n\n"
      .. plugins_section
      .. "\n\n---\n\n"
      .. usage_section
      .. "\n\n---\n\n"
      .. live_section

    if not prompt:find("%S") then
      vim.notify(
        "pi-teacher: prompt vacío — faltan teacher/system-prompt.md y/o teacher/keymaps.md",
        vim.log.levels.ERROR
      )
      return
    end

    -- Flags audited against pi 0.85.1 (docs/usage.md + dist/core/resource-loader.js):
    --   --no-extensions        disables extension discovery AND packages from
    --                          ~/.pi/agent/settings.json (packages contribute
    --                          extension paths; there is no --no-packages flag).
    --   --no-skills            disables skill discovery (package skills included).
    --   --no-prompt-templates  disables prompt template discovery.
    --   --no-themes            disables theme discovery (user theme not loaded).
    --   --no-session           ephemeral run: nothing persisted to ~/.pi/sessions.
    --   --no-context-files     no AGENTS.md / CLAUDE.md from cwd, ~/.pi, project.
    --   --tools read,grep,find,ls  read-only tool allowlist.
    --   --append-system-prompt assembled teacher prompt (also defeats the
    --                          discovered ~/.pi/agent/APPEND_SYSTEM.md, which is
    --                          only used when no explicit append prompt is given).
    local cmd = {
      pi_bin,
      "--no-extensions",
      "--no-skills",
      "--no-prompt-templates",
      "--no-themes",
      "--no-session",
      "--no-context-files",
      "--tools",
      "read,grep,find,ls",
      -- Teacher model override: fast "flash"-class model, see locals above.
      -- Both flags are explicit so the teacher never inherits (or ambiguously
      -- resolves) the user's default settings (gpt-5.6-luna), which are left
      -- untouched for normal pi use.
      "--provider",
      TEACHER_PROVIDER,
      "--model",
      TEACHER_MODEL,
      "--thinking",
      TEACHER_THINKING,
      "--append-system-prompt",
      prompt,
    }

    state.buf = vim.api.nvim_create_buf(false, true)
    setup_teacher_buffer(state.buf)
    show_float(from_win)
    -- scrollback: the 250-line termopen default can clip long teacher answers.
    local ok, err = pcall(vim.fn.termopen, cmd, {
      env = { TEACHER_FILE = vim.fn.expand("%:p") },
      scrollback = 5000,
    })
    if not ok then
      vim.notify("pi-teacher: no pude lanzar pi: " .. tostring(err), vim.log.levels.ERROR)
      close_float()
      return
    end
    -- Synchronous terminal mode: open_win(enter=true) already focused the
    -- float, so typing reaches pi with zero delay — no deferred
    -- startinsert racing the user's first keystrokes.
    vim.cmd("startinsert")
    return
  end

  -- Chat alive, buffer exists: just show the float again (history intact).
  show_float(from_win)
  -- Synchronous terminal mode (see the fresh-spawn branch above): the
  -- float already has focus, so this lands in Terminal mode, and the
  -- buffer-local WinEnter autocmd re-asserts it on every later entry.
  vim.cmd("startinsert")
end

return {
  "folke/snacks.nvim", -- piggyback on the spec lazy.nvim already manages
  keys = {
    {
      "<leader>a",
      open_teacher,
      mode = { "n", "v" },
      desc = "Teacher (ask pi)",
    },
  },
  init = function()
    vim.api.nvim_create_user_command("Teacher", open_teacher, { desc = "Toggle the pi teacher float" })
    -- Snapshot the command/search history baseline and register the
    -- VimLeavePre merge (separate hook from the job-stop one below).
    usage.setup()
    -- Kill the chat job at exit so nvim exits instantly with :qa / :qa!.
    vim.api.nvim_create_autocmd("VimLeavePre", {
      desc = "pi-teacher: stop chat job so nvim can exit",
      callback = function()
        if teacher_alive() then
          local job = vim.b[state.buf].terminal_job_id
          if job and job > 0 then
            pcall(vim.fn.jobstop, job)
          end
        end
      end,
    })
  end,
}
