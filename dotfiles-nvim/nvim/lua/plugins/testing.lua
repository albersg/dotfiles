-- Running tests from the editor.
--
-- LazyVim's test.core extra, imported in lua/config/lazy.lua, brings neotest in
-- and connects it to the debugging setup already configured through dap.core. It
-- ships no adapters, and an adapter is what knows how to find and run a test in a
-- given language, so without this file neotest starts and discovers nothing in
-- any language.
--
-- The adapters are declared as dependencies so lazy.nvim installs them, and
-- listed in opts so neotest loads them. Both are needed: the dependency alone
-- installs a plugin nobody uses.
return {
  {
    "nvim-neotest/neotest",
    dependencies = {
      "nvim-neotest/neotest-go",
      "nvim-neotest/neotest-python",
    },
    opts = {
      adapters = {
        ["neotest-go"] = {},
        ["neotest-python"] = {
          -- The defaults are python3 and pytest, which is what is installed here,
          -- so they are stated rather than changed: `python3 -m pytest` works on
          -- this machine and the adapter invokes exactly that.
          runner = "pytest",
          dap = { justMyCode = false },
        },
      },
    },
  },
}
