-- This file contains the configuration overrides for specific Neovim plugins.

return {
  -- Change configuration for trouble.nvim
  {
    -- Plugin: trouble.nvim
    -- URL: https://github.com/folke/trouble.nvim
    -- Description: A pretty list for showing diagnostics, references, telescope results, quickfix and location lists.
    "folke/trouble.nvim",
    -- NOTE: trouble.nvim removed its `use_diagnostic_signs` option; signs are now
    -- derived from `vim.diagnostic.config().signs` automatically. The previous
    -- `opts = { use_diagnostic_signs = true }` was a dead key (stored but never
    -- read), so it has been removed.
    opts = {},
  },

  -- Add symbols-outline.nvim plugin
  {
    -- Plugin: symbols-outline.nvim
    -- URL: https://github.com/simrat39/symbols-outline.nvim
    -- Description: A tree like view for symbols in Neovim using the Language Server Protocol.
    "simrat39/symbols-outline.nvim",
    cmd = "SymbolsOutline", -- Command to open the symbols outline
    keys = { { "<leader>cs", "<cmd>SymbolsOutline<cr>", desc = "Symbols Outline" } }, -- Keybinding to open the symbols outline
    config = true, -- Use default configuration
  },

  -- Remove inlay hints from default configuration
  {
    -- Plugin: nvim-lspconfig
    -- URL: https://github.com/neovim/nvim-lspconfig
    -- Description: Quickstart configurations for the Neovim LSP client.
    "neovim/nvim-lspconfig",
    event = "VeryLazy", -- Load this plugin on the 'VeryLazy' event
    opts = {
      inlay_hints = { enabled = false }, -- Disable inlay hints
      servers = {
        angularls = {
          -- Configuration for Angular Language Server
          root_dir = function(fname)
            return require("lspconfig.util").root_pattern("angular.json", "project.json")(fname)
          end,
        },
        nil_ls = {
          -- Configuration for nil (Nix Language Server), already installed via nix
          cmd = { "nil" },
          autostart = true,
          mason = false, -- Explicitly disable mason management for nil_ls
          settings = {
            ["nil"] = {
              formatting = { command = { "nixpkgs-fmt" } },
            },
          },
        },
      },
    },
  },
}
