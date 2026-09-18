-- Live keymap dump for the pi "Neovim teacher" chat.
-- Builds a markdown table of ALL mappings effective in this session
-- (user, LazyVim defaults, and every loaded plugin) in pure Lua: no
-- headless nvim spawn, no `map`/`dump` shellout.
--
-- Data source: vim.fn.maplist() — returns global AND buffer-local
-- mappings, one entry per (lhs, mode-group) with lhs ALREADY rendered in
-- readable notation ("<S-Tab>", "<Space>yy"). Do NOT pass that lhs
-- through keytrans(); it escapes "<" into "<lt>" and corrupts it.
-- vim.api.nvim_get_keymap() was rejected: it is global-only and

-- duplicates v-defined maps in both the 'v' and 'x' lists.
--
-- Tested shape of a maplist() entry (nvim 0.12.4):
--   rhs may be nil when a Lua `callback` is used instead of a
--   right-hand-side string; desc, mode, buffer are regular fields.

local M = {}

-- The generated section is also written here so an already-running chat
-- can read the freshest dump with its `read` tool (the inline prompt
-- snapshot only refreshes when the pi process restarts).
M.cache_path = vim.fn.expand("~/.cache/nvim/teacher-live-keymaps.md")

-- Stable sort order for the mode column.
local MODE_RANK = { n = 1, i = 2, v = 3, x = 4, o = 5, t = 6 }

local function sanitize(s)
  if not s then
    return ""
  end
  -- control chars -> spaces, then escape cells/rows that would
  -- break the markdown table.
  s = s:gsub("%c", " ")
  s = s:gsub("%|", "\\|")
  return vim.trim(s)
end

local function truncate(s, max)
  max = max or 60
  if #s <= max then
    return s
  end
  return s:sub(1, max - 1) .. "…"
end

--- Render lhs: maplist already yields readable notation; relabel
--- <Space> tokens as <leader> (mapleader is " ") to match keymaps.md.
local function lhs_display(lhs)
  return (lhs:gsub("<Space>", "<leader>"))
end

--- Preferred description: explicit desc, else a short rhs summary,
--- else a marker for Lua-callback mappings.
local function describe(m)
  if m.desc and m.desc ~= "" then
    return sanitize(m.desc)
  end
  if type(m.rhs) == "string" and m.rhs ~= "" then
    return "acción: `" .. truncate(sanitize(m.rhs)) .. "`"
  end
  if m.callback then
    return "acción: (función Lua)"
  end
  return ""
end

--- Noise filter: skip empty/internal no-op entries, keep everything
--- else (including <Nop> mappings WITH a desc: leader group labels and
--- deliberately disabled keys are informative).
local function is_noise(m)
  local has_callback = m.callback ~= nil
  local has_desc = m.desc ~= nil and m.desc ~= ""
  if has_callback or has_desc then
    return false
  end
  if type(m.rhs) == "string" and m.rhs ~= "" and m.rhs:upper() ~= "<NOP>" then
    return false
  end
  return true
end

--- Build the live keymap markdown table (header + rows only, no
--- surrounding section wrapper). Dedupes by lhs+mode+desc+buffer.
---@return string
function M.table()
  local seen, rows = {}, {}
  for _, m in ipairs(vim.fn.maplist()) do
    if not is_noise(m) then
      local key = m.lhs .. "\1" .. m.mode .. "\1" .. sanitize(m.desc) .. "\1" .. tostring(m.buffer)
      if not seen[key] then
        seen[key] = true
        local mode = (m.mode or "?"):gsub("%c", "")
        rows[#rows + 1] = {
          lhs = sanitize(lhs_display(m.lhs or "")),
          mode = mode ~= "" and mode or "?",
          desc = describe(m),
          rank = MODE_RANK[mode:sub(1, 1)] or 9,
        }
      end
    end
  end
  table.sort(rows, function(a, b)
    if a.lhs ~= b.lhs then
      return a.lhs < b.lhs
    end
    if a.rank ~= b.rank then
      return a.rank < b.rank
    end
    return a.mode < b.mode
  end)

  local lines = {
    "| Tecla | Modo | Descripción / acción |",
    "|---|---|---|",
  }
  for _, r in ipairs(rows) do
    lines[#lines + 1] = ("| `%s` | `%s` | %s |"):format(r.lhs, r.mode, r.desc)
  end
  return table.concat(lines, "\n")
end

--- Full live section with its header and the override note (Spanish,
--- neutral, tú form — this text feeds the chat prompt).
---@return string
function M.section()
  return table.concat({
    "## Mapeos activos en esta sesión (generado en vivo, fuente de verdad actual)",
    "",
    "Nota: esta sección se genera en vivo al abrir el chat. Si algo aquí contradice las tablas anteriores, esta sección prevalece.",
    "",
    M.table(),
  }, "\n")
end

--- Build the live section and write it to the stable cache file so an
--- already-running chat can re-read it with the `read` tool.
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

return M
