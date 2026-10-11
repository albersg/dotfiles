#!/usr/bin/env bash
#
# check-nvim-clipboard.sh - guard for the Neovim OSC 52 SSH clipboard fix.
#
# The Neovim configuration moved `vim.opt.clipboard = "unnamedplus"` from
# lua/config/lazy.lua to lua/config/options.lua and installs an OSC 52 clipboard
# in options.lua when the session runs over SSH. With no Lua test harness in this
# repository, this script is the harness: it greps the plain facts it can read as
# text and it loads options.lua in a real headless Neovim to assert behaviour.
#
# Checks:
#   1. nvim is installed. If it is not, this guard FAILS (clear SKIP message);
#      it never passes without checking anything.
#   2. vim.opt.clipboard is set from exactly one place in the config and that
#      place is options.lua; lazy.lua no longer sets it.
#   3. options.lua guards the OSC 52 clipboard with vim.env.SSH_CONNECTION and
#      uses the vim.ui.clipboard.osc52 provider named "OSC 52".
#   3b. The WSL clipboard handling is still wired (informational: this tree has
#      no win32yank g:clipboard branch, so WSL relies on unnamedplus).
#   4. Loading options.lua with SSH_CONNECTION set installs the OSC 52
#      clipboard, and loading it without SSH_CONNECTION does not.
#
# Uses only `nvim --headless`; no display, TTY or plugins are required.
#
# Usage:
#   bash scripts/check-nvim-clipboard.sh
#
# Exit codes: 0 all checks passed, 1 a check failed or nvim is missing.

set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
NVIM_DIR="$REPO_ROOT/dotfiles-nvim/nvim"
OPTIONS="$NVIM_DIR/lua/config/options.lua"
LAZY="$NVIM_DIR/lua/config/lazy.lua"
CONFIG_LUA="$NVIM_DIR/lua"
ASSERT="$REPO_ROOT/dotfiles-nvim/test/clipboard_assert.lua"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

# 1. nvim must exist, otherwise nothing below can be verified. A guard that
#    cannot check must fail loudly, never report success.
if ! command -v nvim >/dev/null 2>&1; then
  printf 'SKIP: nvim is not installed, so the clipboard guard cannot run.\n' >&2
  printf 'SKIP: install Neovim to run scripts/check-nvim-clipboard.sh.\n' >&2
  printf 'SKIP: failing instead of passing without checking anything.\n' >&2
  exit 1
fi

printf 'nvim: %s\n' "$(nvim --version | head -n1)"

# 2. The clipboard option is set once, from options.lua only.
clipboard_hits=$(grep -rn 'vim\.opt\.clipboard' "$CONFIG_LUA" || true)
clipboard_count=$(printf '%s\n' "$clipboard_hits" | grep -c 'vim\.opt\.clipboard' || true)
if [ "$clipboard_count" -ne 1 ]; then
  printf '%s\n' "$clipboard_hits" >&2
  fail "vim.opt.clipboard must be set exactly once in lua/, found $clipboard_count"
fi
case "$clipboard_hits" in
  "$OPTIONS":*) ;;
  *) fail "vim.opt.clipboard must be set in options.lua, found it at $clipboard_hits" ;;
esac
grep -q 'vim\.opt\.clipboard *= *"unnamedplus"' "$OPTIONS" \
  || fail 'options.lua must set vim.opt.clipboard = "unnamedplus"'
if grep -q 'vim\.opt\.clipboard' "$LAZY"; then
  fail 'lazy.lua must no longer set vim.opt.clipboard (it moved to options.lua)'
fi
printf 'ok: vim.opt.clipboard set once, in %s\n' "$clipboard_hits"

# 3. The SSH branch must be guarded and use the OSC 52 provider.
grep -q 'vim\.env\.SSH_CONNECTION' "$OPTIONS" \
  || fail 'options.lua must guard the OSC 52 clipboard with vim.env.SSH_CONNECTION'
grep -q 'vim\.ui\.clipboard\.osc52' "$OPTIONS" \
  || fail 'options.lua must use the vim.ui.clipboard.osc52 provider'
grep -q '"OSC 52"' "$OPTIONS" \
  || fail 'options.lua must name the clipboard "OSC 52"'
printf 'ok: options.lua guards OSC 52 behind SSH_CONNECTION\n'

# 3b. The WSL clipboard handling must still be present. In this tree the WSL
#     behaviour is vim.opt.clipboard = "unnamedplus" (there is no win32yank
#     g:clipboard branch), so the named WSL clipboard is covered by check 2.
if grep -Eq 'has\(["'"'"']wsl["'"'"']\)|win32yank\.exe' "$LAZY" "$OPTIONS"; then
  printf 'info: WSL clipboard branch present\n'
else
  printf 'info: no WSL win32yank branch exists in this tree; WSL relies on unnamedplus\n'
fi

# 4. Behavioural checks in real headless Neovim.
run_case() {
  local expect="$1" label="$2"
  shift 2
  local output
  if ! output=$(env -i HOME="${HOME:-/tmp}" PATH="$PATH" TERM=dumb \
    OPTIONS_PATH="$OPTIONS" EXPECT="$expect" "$@" \
    nvim --headless -u NONE -c "luafile $ASSERT" -c "qa!" 2>&1); then
    printf '%s\n' "$output" >&2
    fail "$label"
  fi
  printf '%s\n' "$output"
  printf 'ok: %s\n' "$label"
}

run_case osc52 'OSC 52 clipboard installed when SSH_CONNECTION is set' SSH_CONNECTION='1 2 3 4'
run_case native 'OSC 52 clipboard absent when SSH_CONNECTION is unset'

printf 'PASS: nvim clipboard guard\n'
