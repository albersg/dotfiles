-- Robust override for LazyVim's <leader>fm mini.files mapping.
--
-- Upstream (lazyvim.plugins.extras.editor.mini-files) defines:
--   require("mini.files").open(vim.api.nvim_buf_get_name(0), true)
-- When the current buffer is an oil.nvim buffer its name is an `oil://...`
-- URI, not a filesystem path, so MiniFiles.open() rejects it with:
--   (mini.files) `path` is not a valid path ("oil:/...")
--
-- Instead of stripping the `oil://` scheme after the fact (which would hide the
-- design issue of feeding a buffer name straight into a filesystem API), we
-- resolve a real filesystem anchor for the current buffer:
--   * oil buffer        -> the directory Oil is currently showing
--   * real file/dir     -> that path (mini.files focuses on a file)
--                         or its parent when the file does not exist yet
--   * terminal / unnamed / other URI buffers -> current working directory
local function mini_files_anchor()
  local name = vim.api.nvim_buf_get_name(0)

  -- 1) Oil buffers: ask Oil for the underlying directory.
  if vim.bo.filetype == "oil" then
    local ok, oil = pcall(require, "oil")
    if ok then
      local ok_dir, dir = pcall(oil.get_current_dir, 0)
      if ok_dir and dir and dir ~= "" then
        return dir
      end
    end
  end

  -- 2) Regular file or directory buffers (buftype "" and not a URI scheme).
  local is_uri = name:match("^%a[%w+.-]*://") ~= nil
  if name ~= "" and vim.bo.buftype == "" and not is_uri then
    if vim.uv.fs_stat(name) then
      return name
    end
    local dir = vim.fs.dirname(name)
    if dir and dir ~= "" and vim.uv.fs_stat(dir) then
      return dir
    end
  end

  -- 3) Terminal, unnamed, help, quickfix and other URI buffers: use cwd.
  return vim.uv.cwd() or vim.fn.getcwd()
end

return {
  {
    "nvim-mini/mini.files",
    -- Only <leader>fm is overridden; LazyVim's opts and its <leader>fM
    -- (which already uses the cwd) are preserved by lazy.nvim's spec merge.
    keys = {
      {
        "<leader>fm",
        function()
          require("mini.files").open(mini_files_anchor(), true)
        end,
        desc = "Open mini.files (Directory of Current File)",
      },
    },
  },
}
