-- User-facing entry point for scripts/safe-update.sh.
--
-- The script performs the whole safe update transaction (snapshot, lockfile
-- backup, headless sync, health gate, automatic rollback). This module only
-- exposes it as an explicit user command; nothing is scheduled or automatic.
--
--   :NzUpdatePlugins          run the real update in a floating terminal
--   :NzUpdatePlugins check    dry-run: Lazy check + health, no writes
--
-- The terminal is Snacks'. If Snacks is missing, we fall back to a notify with
-- the exact command to run manually instead of failing silently.
local M = {}

local function script_path()
  return vim.fs.joinpath(vim.fn.stdpath("config"), "scripts", "safe-update.sh")
end

function M.setup()
  vim.api.nvim_create_user_command("NzUpdatePlugins", function(opts)
    local script = script_path()
    if vim.fn.filereadable(script) == 0 then
      vim.notify("[safe-update] script not found: " .. script, vim.log.levels.ERROR)
      return
    end

    local cmd = { script }
    if opts.args == "check" or opts.args == "dry-run" then
      cmd[#cmd + 1] = "--dry-run"
    end

    local ok, snacks = pcall(require, "snacks")
    if ok and snacks.terminal then
      snacks.terminal(cmd, {
        cwd = vim.fn.stdpath("config"),
        auto_close = false,
        win = { position = "float", border = "rounded", width = 0.9, height = 0.85 },
      })
    else
      vim.notify(
        "[safe-update] Snacks unavailable. Run manually:\n" .. table.concat(cmd, " "),
        vim.log.levels.WARN
      )
    end
  end, {
    desc = "Update plugins safely (scripts/safe-update.sh; 'check' for dry-run)",
    nargs = "?",
    complete = function()
      return { "check", "dry-run" }
    end,
  })
end

return M
