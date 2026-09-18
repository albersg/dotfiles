-- Cumulative usage-history context for the pi "Neovim teacher" chat.
--
-- Goal: let the teacher know the user's REAL Ex-command / search usage so it
-- can suggest improvements (shortcut replacements, commands used manually
-- that already have keymaps, etc.).
--
-- Design (empirically verified on nvim 0.12.4):
--   * Counting from `histget(':', i)` at exit alone would be dishonest:
--     shada restores PREVIOUS sessions' history into it (verified: entries
--     from earlier sessions show up), and `histadd()` dedupes/moves-to-front
--     (the list never contains duplicates), so neither "per-occurrence" nor
--     "delta-vs-baseline" counting can see repeated usage across sessions
--     accurately.
--   * HONEST per-occurrence semantics: count on `CmdlineLeave`, which fires
--     only for cmdlines typed interactively in THIS session (never for
--     script/vim.cmd executions). Every distinct entry the user types,
--     whether new or repeated from a previous session, increments exactly
--     once per occurrence. Esc-aborted lines are also kept in the history by
--     vim (`:h c_Esc`) and are semi-intentional input; they are counted too.
--   * Store: cumulative counters at ~/.local/share/nvim/teacher-usage.json,
--     merged on VimLeavePre. A corrupt store is backed up as .bak and a
--     fresh one is started — never crash the editor.
--
-- Performance: merge work happens ONLY at exit; M.top() is a cheap sort of
-- the small top-level tables on every teacher open.

local M = {}

local store_path = vim.fn.expand("~/.local/share/nvim/teacher-usage.json")
-- Written on every teacher open (like the live keymap cache) so a
-- long-lived pi chat can read fresh usage data via its `read` tool.
M.cache_path = vim.fn.expand("~/.cache/nvim/teacher-usage.md")

-- Max stored entries per table: guards against unbounded JSON growth.
local MAX_ENTRIES = 500
-- Skip pasted junk / pathological lines.
local MAX_LEN = 80

-- Short, deliberate blocklist of noisy session-internal commands.
-- Rationale: ^q/^w/^x cover q, qa, q!, w, wq, wa, x, xa, xit, ... (quit and
-- write variants are never teachable usage); the rest are session plumbing.
local COMMAND_BLOCKLIST = {
  "^q", -- quit / qa / q! / ...
  "^w", -- write / wq / wa / wall / ...
  "^x", -- xit / xa / ...
  "^source", -- manual reloads during config hacking
  "^redir", -- output capture sessions
  "^noh", -- transient highlight clear
  "^set", -- option poking during experimentation
}

local function is_blocked(entry)
  for _, pat in ipairs(COMMAND_BLOCKLIST) do
    if entry:find(pat) == 1 then
      return true
    end
  end
  return false
end

--- True when the entry is worth counting.
---@param entry string
---@param is_search boolean
---@return boolean
local function is_worth_counting(entry, is_search)
  if entry == "" or #entry > MAX_LEN then
    return false
  end
  if not is_search and is_blocked(entry) then
    return false
  end
  return true
end

-- This session's occurrences, keyed per cmdline type.
local session_counts = { commands = {}, searches = {} }

--- Record one interactive cmdline occurrence. Public for testing.
---@param kind string ":" or "/"
---@param line string
function M.record(kind, line)
  if kind == ":" then
    line = line:gsub("^:", "", 1) -- strip a leading ':' just in case
  end
  line = vim.trim(line)
  if not is_worth_counting(line, kind == "/") then
    return
  end
  local t = kind == ":" and session_counts.commands or session_counts.searches
  t[line] = (t[line] or 0) + 1
end

--- Read the store from disk. Corrupt file -> .bak + fresh store.
---@return table
local function read_store()
  local f = io.open(store_path, "r")
  if not f then
    return { commands = {}, searches = {}, updated = 0, sessions = 0 }
  end
  local content = f:read("*a")
  f:close()
  local ok, decoded = pcall(vim.json.decode, content)
  if
    ok
    and type(decoded) == "table"
    and type(decoded.commands) == "table"
    and type(decoded.searches) == "table"
  then
    return decoded
  end
  -- Backup the corrupt file (overwrite any previous .bak) and start fresh.
  os.rename(store_path, store_path .. ".bak")
  return { commands = {}, searches = {}, updated = 0, sessions = 0 }
end

---@param store table
local function write_store(store)
  vim.fn.mkdir(vim.fn.fnamemodify(store_path, ":h"), "p")
  store.updated = os.time()
  local f = io.open(store_path, "w")
  if f then
    f:write(vim.json.encode(store))
    f:close()
  end
end

--- Merge this session's occurrences into the persistent store.
--- Exposed for testing; in production it runs once from VimLeavePre.
---@param opts table|nil { count_session = boolean, default true }
---@return table store the merged store
function M.merge_now(opts)
  opts = opts or {}
  local store = read_store()
  for _, pair in ipairs({
    { src = session_counts.commands, dst = store.commands },
    { src = session_counts.searches, dst = store.searches },
  }) do
    for term, n in pairs(pair.src) do
      pair.dst[term] = (pair.dst[term] or 0) + n
    end
  end

  -- Trim each table to the most-used MAX_ENTRIES entries.
  for field in pairs({ commands = true, searches = true }) do
    local t = store[field]
    local size, entries = 0, {}
    for k, v in pairs(t) do
      size = size + 1
      entries[#entries + 1] = { k, v }
    end
    if size > MAX_ENTRIES then
      table.sort(entries, function(a, b)
        if a[2] ~= b[2] then
          return a[2] > b[2]
        end
        return a[1] < b[1]
      end)
      local trimmed = {}
      for i = 1, MAX_ENTRIES do
        trimmed[entries[i][1]] = entries[i][2]
      end
      store[field] = trimmed
    end
  end

  if opts.count_session ~= false then
    store.sessions = (store.sessions or 0) + 1
  end
  write_store(store)
  return store
end

--- Drop this session's in-memory counts (test helper; production merges at
--- exit and process exit clears them naturally).
function M.reset_session()
  session_counts = { commands = {}, searches = {} }
end

--- Sort a counter table by count desc, then name asc; return the top n.
---@param t table
---@param n integer
---@return table list of { name, count }
local function top_entries(t, n)
  local list = {}
  for k, v in pairs(t) do
    list[#list + 1] = { name = k, count = v }
  end
  table.sort(list, function(a, b)
    if a.count ~= b.count then
      return a.count > b.count
    end
    return a.name < b.name
  end)
  local out = {}
  for i = 1, math.min(n, #list) do
    out[i] = list[i]
  end
  return out
end

local function rows_to_md(rows, label)
  if #rows == 0 then
    return ""
  end
  local lines = {
    ("| %s | Usos |"):format(label),
    "|---|---|",
  }
  for _, r in ipairs(rows) do
    lines[#lines + 1] = ("| `%s` | %d |"):format(r.name:gsub("%|", "\\|"), r.count)
  end
  return table.concat(lines, "\n")
end

--- Build the usage markdown section (neutral Spanish, tú form).
---@param n_commands integer|nil default 15
---@param n_searches integer|nil default 10
---@return string
function M.section(n_commands, n_searches)
  n_commands = n_commands or 15
  n_searches = n_searches or 10
  local store = read_store()
  local parts = {
    "## Tus patrones de uso (histórico acumulado)",
    "",
  }
  local cmds = top_entries(store.commands or {}, n_commands)
  local searches = top_entries(store.searches or {}, n_searches)
  if #cmds == 0 and #searches == 0 then
    parts[#parts + 1] = "Aún sin datos; se acumulará al cerrar sesiones."
  else
    local cmd_md = rows_to_md(cmds, "Comando Ex")
    if cmd_md ~= "" then
      parts[#parts + 1] = cmd_md
      parts[#parts + 1] = ""
    end
    local search_md = rows_to_md(searches, "Búsqueda")
    if search_md ~= "" then
      parts[#parts + 1] = search_md
      parts[#parts + 1] = ""
    end
    parts[#parts + 1] = ("Sesiones registradas: %d. Usa estos datos para sugerir mejoras: atajos que sustituyan comandos manuales frecuentes, comandos usados a mano que ya tienen un keymap en la config, patrones de búsqueda que se podrían automatizar, etc. El fichero `~/.cache/nvim/teacher-usage.md` siempre contiene esta sección actualizada; léelo con la herramienta `read` si lo necesitas."):format(
      store.sessions or 0
    )
  end
  return table.concat(parts, "\n")
end

--- Build the section and write it to the cache file (like the keymap dump).
---@return string section
function M.write_cache()
  local section = M.section()
  vim.fn.mkdir(vim.fn.fnamemodify(M.cache_path, ":h"), "p")
  local f = io.open(M.cache_path, "w")
  if f then
    f:write(section .. "\n")
    f:close()
  end
  return section
end

--- Setup: register the interactive-cmdline recorder and the exit merge.
--- Called once from pi-teacher.lua (lazy spec `init`, which always runs).
--- Kept separate from the teacher's own VimLeavePre job-stop hook.
function M.setup()
  vim.api.nvim_create_autocmd("CmdlineLeave", {
    desc = "teacher-usage: record interactive Ex/search cmdline occurrences",
    callback = function()
      pcall(M.record, vim.fn.getcmdtype(), vim.fn.getcmdline())
    end,
  })
  vim.api.nvim_create_autocmd("VimLeavePre", {
    desc = "teacher-usage: merge this session's command/search usage",
    callback = function()
      pcall(M.merge_now)
    end,
  })
end

return M
