-- WP-D: CodeGraph integration.
--
-- Read-only wrapper around the installed `codegraph` CLI (v1.3.0+) that turns
-- its JSON output into a quickfix list + snacks picker, so you can answer
-- "what does changing this influence?" without leaving Neovim.
--
-- Design notes:
--   * No new plugins: results go to the quickfix list and are shown with
--     Snacks.picker.qflist() (falls back to :copen when snacks is absent).
--   * The CLI runs asynchronously with vim.system() and never blocks the UI.
--   * Everything degrades gracefully: missing CLI, missing .codegraph index,
--     command failure and unparseable JSON each produce a friendly notify or a
--     raw-output scratch buffer instead of an error.
--   * Keymaps live on the free <leader>i prefix (verified free at runtime).
--
-- Verified CLI shapes (codegraph 1.3.0):
--   query     -> [ { node: {...}, score } ]
--   callers   -> { symbol, callers:   [ { name, kind, filePath, startLine } ] }
--   callees   -> { symbol, callees:   [ { name, kind, filePath, startLine } ] }
--   impact    -> { symbol, affected:  [ { name, kind, filePath, startLine } ] }
--   affected  -> { changedFiles, affectedTests, totalDependentsTraversed }
--   status    -> takes a positional path (no -p flag).

local M = {}

local function notify(msg, level)
  vim.notify("[codegraph] " .. msg, level or vim.log.levels.INFO)
end

local function cli_available()
  return vim.fn.executable("codegraph") == 1
end

---Nearest ancestor directory containing a `.codegraph` index, or nil.
---@return string|nil
local function project_root()
  local name = vim.api.nvim_buf_get_name(0)
  local start = name ~= "" and vim.fs.dirname(name) or vim.fn.getcwd()
  return vim.fs.root(start, { ".codegraph" })
end

local function decode(text)
  local ok, data = pcall(vim.json.decode, text)
  if not ok or type(data) ~= "table" then
    return nil
  end
  return data
end

---Open a scratch buffer showing raw text.
local function scratch(text, title)
  local lines = vim.split(text or "", "\n")
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].filetype = "json"
  vim.api.nvim_buf_set_name(buf, title)
  vim.cmd("botright split")
  vim.api.nvim_win_set_buf(0, buf)
end

---Show parseable entries in the quickfix list + snacks picker.
local function open_items(items, title)
  if #items == 0 then
    notify("No results for " .. title, vim.log.levels.INFO)
    return
  end
  vim.fn.setqflist({}, " ", {
    title = "codegraph: " .. title,
    items = items,
  })
  if package.loaded["snacks"] and Snacks and Snacks.picker then
    if pcall(Snacks.picker.qflist, { title = "codegraph: " .. title }) then
      return
    end
  end
  vim.cmd("copen")
end

---Turn CLI node entries into quickfix items.
local function node_items(entries, root, prefix)
  local items = {}
  for _, e in ipairs(entries or {}) do
    local file = e.filePath or e.file or e.path
    if file then
      if file:sub(1, 1) ~= "/" then
        file = vim.fs.joinpath(root, file)
      end
      local label = e.name or file
      if e.kind then
        label = label .. " [" .. e.kind .. "]"
      end
      items[#items + 1] = {
        filename = file,
        lnum = e.startLine or e.line or 1,
        text = (prefix and (prefix .. ": ") or "") .. label,
      }
    end
  end
  return items
end

---Run a codegraph command asynchronously.
---@param args string[]  Full argument list (without the leading binary).
---@param root string    Project root used as cwd.
---@param on_ok fun(out: string, root: string)
local function run(args, root, on_ok)
  if not cli_available() then
    notify("CLI not found in PATH. Install it with: npm i -g @colbymchenry/codegraph", vim.log.levels.WARN)
    return
  end

  local cmd = { "codegraph" }
  vim.list_extend(cmd, args)
  vim.system(cmd, { cwd = root, text = true }, function(obj)
    vim.schedule(function()
      if obj.code ~= 0 then
        local err = (obj.stderr and obj.stderr ~= "") and obj.stderr or (obj.stdout or "")
        scratch(err, "codegraph-error://" .. table.concat(args, "_"))
        notify("Command failed: codegraph " .. table.concat(args, " "), vim.log.levels.ERROR)
        return
      end
      on_ok(obj.stdout or "", root)
    end)
  end)
end

---Resolve the project root or notify and return nil.
local function require_root()
  if not cli_available() then
    notify("CLI not found in PATH. Install it with: npm i -g @colbymchenry/codegraph", vim.log.levels.WARN)
    return nil
  end
  local root = project_root()
  if not root then
    notify("No .codegraph index for this file. Run 'codegraph init' in the project root.", vim.log.levels.WARN)
    return nil
  end
  return root
end

local function symbol_under_cursor()
  local word = vim.fn.expand("<cword>")
  if word == "" then
    notify("No symbol under the cursor", vim.log.levels.WARN)
    return nil
  end
  return word
end

local function symbol_command(args, label, key, root)
  local sym = symbol_under_cursor()
  if not sym then
    return
  end
  local argv = { args, sym, "--json", "-p", root }
  run(argv, root, function(out, r)
    local data = decode(out)
    if not data then
      scratch(out, "codegraph-raw://" .. label)
      notify("Could not parse JSON; showing raw output", vim.log.levels.WARN)
      return
    end
    open_items(node_items(data[key], r, sym), label .. ": " .. sym)
  end)
end

-- Public actions -----------------------------------------------------------

function M.impact()
  local root = require_root()
  if root then
    symbol_command("impact", "impact", "affected", root)
  end
end

function M.callers()
  local root = require_root()
  if root then
    symbol_command("callers", "callers", "callers", root)
  end
end

function M.callees()
  local root = require_root()
  if root then
    symbol_command("callees", "callees", "callees", root)
  end
end

function M.affected()
  local root = require_root()
  if not root then
    return
  end
  local name = vim.api.nvim_buf_get_name(0)
  if name == "" then
    notify("Current buffer has no file on disk", vim.log.levels.WARN)
    return
  end
  local rel = vim.fs.relpath(root, name) or name
  run({ "affected", rel, "--json" }, root, function(out, r)
    local data = decode(out)
    if not data then
      scratch(out, "codegraph-raw://affected")
      notify("Could not parse JSON; showing raw output", vim.log.levels.WARN)
      return
    end
    local items = {}
    for _, entry in ipairs(data.affectedTests or {}) do
      local file = type(entry) == "string" and entry or (entry.filePath or entry.path or entry.file)
      if file then
        items[#items + 1] = {
          filename = vim.fs.joinpath(r, file),
          lnum = (type(entry) == "table" and (entry.startLine or entry.line)) or 1,
          text = "affected test",
        }
      end
    end
    open_items(items, "affected: " .. rel)
  end)
end

function M.query()
  local root = require_root()
  if not root then
    return
  end
  local query = vim.fn.input("codegraph query: ")
  if query == "" then
    return
  end
  run({ "query", query, "--json", "-l", "50", "-p", root }, root, function(out, r)
    local data = decode(out)
    if not data then
      scratch(out, "codegraph-raw://query")
      notify("Could not parse JSON; showing raw output", vim.log.levels.WARN)
      return
    end
    local nodes = {}
    for _, entry in ipairs(data) do
      if entry.node then
        nodes[#nodes + 1] = entry.node
      end
    end
    open_items(node_items(nodes, r, query), "query: " .. query)
  end)
end

function M.status()
  local root = require_root()
  if not root then
    return
  end
  run({ "status", root, "--json" }, root, function(out)
    scratch(out, "codegraph-status://" .. vim.fn.fnamemodify(root, ":t"))
  end)
end

---Register the <leader>i* keymaps. Called from config/keymaps.lua.
function M.setup()
  local map = function(lhs, fn, desc)
    vim.keymap.set("n", lhs, fn, { desc = desc, silent = true })
  end
  map("<leader>ia", M.affected, "Affected tests (current file)")
  map("<leader>ic", M.callers, "Callers (codegraph)")
  map("<leader>iC", M.callees, "Callees (codegraph)")
  map("<leader>ii", M.impact, "Impact (codegraph)")
  map("<leader>iq", M.query, "Query symbols (codegraph)")
  map("<leader>is", M.status, "Index status (codegraph)")
end

return M
