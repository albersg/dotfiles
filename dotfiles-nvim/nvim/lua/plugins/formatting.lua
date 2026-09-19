-- Code formatting.
--
-- LazyVim ships conform.nvim through the `formatting.prettier` extra and already
-- maps 21 filetypes for the web stack. The languages actually written on this
-- machine had no formatter at all: Go, Shell, Python and Lua were absent from
-- `formatters_by_ft`, so format-on-save silently did nothing there.
--
-- The formatter binaries are installed by Mason rather than assumed. Without the
-- `ensure_installed` list below, the filetypes would be mapped but every
-- formatter would be missing, which is the state this file was written to fix.
return {
  {
    "mason-org/mason.nvim",
    opts = {
      ensure_installed = {
        -- Only the formatters nothing else in this configuration requests.
        -- LazyVim's own extras already pull stylua, shfmt and prettier, and
        -- repeating them here would list each one twice, because the opts merge
        -- concatenates lists rather than deduplicating them.
        --
        -- Go. gofumpt is a stricter gofmt; goimports also fixes the import block.
        "gofumpt",
        "goimports",
        -- Python. ruff replaces both black and isort, and handles the imports.
        "ruff",
        -- Web, for the mappings the prettier extra already declares but does not
        -- install a binary for.
        "biome",
      },
    },
  },
  {
    "stevearc/conform.nvim",
    opts = {
      formatters_by_ft = {
        go = { "goimports", "gofumpt" },
        lua = { "stylua" },
        sh = { "shfmt" },
        bash = { "shfmt" },
        zsh = { "shfmt" },
        python = { "ruff_organize_imports", "ruff_format" },
        -- These come from the LazyVim extra, repeated here only to document what
        -- is expected to be present.
        javascript = { "biome", "prettier" },
        typescript = { "biome", "prettier" },
      },
      -- Stop the formatter with a short timeout rather than blocking the editor
      -- when a formatter hangs, and fall back to the LSP when no formatter is
      -- configured for the buffer.
      format_on_save = {
        timeout_ms = 1000,
        lsp_format = "fallback",
      },
    },
  },
}
