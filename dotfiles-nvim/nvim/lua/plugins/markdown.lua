return {
  "MeanderingProgrammer/render-markdown.nvim",
  dependencies = { "nvim-treesitter/nvim-treesitter", "nvim-mini/mini.nvim" }, -- if you use the mini.nvim suite
  ---@module 'render-markdown'
  ---@type render.md.UserConfig
  opts = {
    heading = {
      enabled = true,
      sign = true,
      style = "full",
      icons = { "① ", "② ", "③ ", "④ ", "⑤ ", "⑥ " },
      left_pad = 1,
    },
    bullet = {
      enabled = true,
      icons = { "●", "○", "◆", "◇" },
      right_pad = 1,
      highlight = "render-markdownBullet",
    },
    checkbox = {
      enabled = true,
      unchecked = {
        icon = "󰄱     ",
        highlight = "RenderMarkdownUnchecked",
      },
      checked = {
        icon = "󰱒     ",
        highlight = "RenderMarkdownChecked",
      },
      custom = {
        todo = { raw = "[-]", rendered = "󰥔     ", highlight = "RenderMarkdownTodo" },
      },
    },
  },
  {
    "mfussenegger/nvim-lint",
    optional = true,
    opts = function(_, opts)
      local linter = require("lint").linters["markdownlint-cli2"]
      if linter and not linter._dotfiles_hide_md013 then
        local parser = linter.parser
        linter.parser = function(...)
          local diagnostics = parser(...)
          return vim.tbl_filter(function(diagnostic)
            local code = diagnostic.code or ""
            local message = diagnostic.message or ""
            return not code:match("^MD013") and not message:match("MD013/line%-length")
          end, diagnostics)
        end
        linter._dotfiles_hide_md013 = true
      end

      opts.linters_by_ft = opts.linters_by_ft or {}
      opts.linters_by_ft.markdown = { "markdownlint-cli2" }
    end,
  },
}
