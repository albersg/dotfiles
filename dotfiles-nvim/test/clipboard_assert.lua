-- Headless assertion helper for the Neovim OSC 52 SSH clipboard fix.
--
-- This file is not part of the Neovim configuration. It exists so that
-- scripts/check-nvim-clipboard.sh can load lua/config/options.lua in a real
-- Neovim process and assert what it does to the clipboard, without booting
-- LazyVim or touching the network.
--
-- Inputs (environment):
--   OPTIONS_PATH  absolute path to lua/config/options.lua (required)
--   EXPECT        "osc52"  -> assert the OSC 52 clipboard is installed
--                 "native" -> assert the OSC 52 clipboard is NOT installed
--
-- Run via:
--   nvim --headless -u NONE -c "luafile <this file>" -c "qa!"
-- Exits non-zero on the first failed assertion.

local options_path = vim.env.OPTIONS_PATH
local expect = vim.env.EXPECT

local function fail(msg)
  io.stderr:write("FAIL: " .. msg .. "\n")
  vim.cmd("cquit 1")
end

local function assert_eq(actual, expected, label)
  if actual ~= expected then
    fail(string.format("%s: expected %s, got %s", label, vim.inspect(expected), vim.inspect(actual)))
  end
  print(string.format("ok: %s == %s", label, vim.inspect(actual)))
end

if not options_path or options_path == "" then
  fail("OPTIONS_PATH is not set")
end

local loaded, load_err = pcall(dofile, options_path)
if not loaded then
  fail("options.lua failed to load: " .. tostring(load_err))
end

print(string.format("info: vim.fn.has('wsl') == %d", vim.fn.has("wsl")))

assert_eq(vim.opt.clipboard:get()[1], "unnamedplus", "clipboard option")

local name = vim.g.clipboard and vim.g.clipboard.name or nil

if expect == "osc52" then
  assert_eq(name, "OSC 52", "g:clipboard.name")
  assert_eq(type(vim.g.clipboard.copy), "table", "g:clipboard.copy type")
  assert_eq(type(vim.g.clipboard.paste), "table", "g:clipboard.paste type")
  assert_eq(type(vim.g.clipboard.copy["+"]), "function", "g:clipboard.copy['+'] type")
  assert_eq(type(vim.g.clipboard.copy["*"]), "function", "g:clipboard.copy['*'] type")
  assert_eq(type(vim.g.clipboard.paste["+"]), "function", "g:clipboard.paste['+'] type")
  assert_eq(type(vim.g.clipboard.paste["*"]), "function", "g:clipboard.paste['*'] type")
elseif expect == "native" then
  if name == "OSC 52" then
    fail("g:clipboard.name must not be OSC 52 without SSH_CONNECTION")
  end
  print(string.format("ok: g:clipboard.name is not OSC 52 (got %s)", vim.inspect(name)))
else
  fail("EXPECT must be 'osc52' or 'native', got " .. vim.inspect(expect))
end

print("PASS")
