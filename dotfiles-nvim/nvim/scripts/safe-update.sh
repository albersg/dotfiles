#!/usr/bin/env bash
# =============================================================================
# safe-update.sh — explicit, reversible LazyVim plugin update
# =============================================================================
#
# WHY THIS EXISTS
#   `:Lazy sync` updates every plugin to its latest commit. When something in
#   that set breaks Neovim, the failure is silent until the next interactive
#   start, and there is no single obvious rollback point. This script makes the
#   update an explicit, reviewable transaction with a guaranteed rollback.
#
# WHAT IT DOES (in order)
#   1. Snapshots the config: commits any uncommitted change under the nvim
#      config repo with a timestamped message, so the whole tree has a hash.
#   2. Backs up lazy-lock.json to the cache dir (the canonical plugin rollback).
#   3. Runs `nvim --headless "+Lazy! sync" +qa`, capturing full output to a log.
#   4. Health-checks the result: a headless start must exit 0 AND write zero
#      bytes to stderr. A `:checkhealth` ERROR count is recorded and compared to
#      a saved baseline (informational unless --strict-health).
#   5. On failure, restores the backed-up lockfile + `:Lazy restore`, prints the
#      exact rollback commands and the config snapshot hash, and exits non-zero.
#   6. On success, prints the lockfile delta, the log path and the snapshot hash.
#
# USAGE
#   scripts/safe-update.sh                 # real update (snapshot + sync + gate)
#   scripts/safe-update.sh --dry-run       # report only: Lazy check + health, no writes
#   scripts/safe-update.sh --strict-health  # also fail if checkhealth ERRORs grow
#
#   From inside Neovim:
#     :NzUpdatePlugins          " run the real update in a floating terminal
#     :NzUpdatePlugins check    " dry-run (no changes)
#
# ROLLBACK
#   Plugins : cp ~/.cache/nvim/lazy-lock.pre-update.json ~/.config/nvim/lazy-lock.json
#             nvim --headless "+Lazy! restore" +qa
#   Config  : git -C ~/.config/nvim reset --hard <snapshot hash printed at the end>
#
# NOTES
#   * No sudo, no destructive commands (no `rm -rf`, no force-push, no reset by
#     default). The only writes are inside the nvim config repo and the cache dir.
#   * Nothing here runs automatically. There is no timer or background checker.
#   * Requires: nvim, git, bash. Uses only POSIX-ish coreutils beyond that.
# =============================================================================
set -euo pipefail

NVIM_CONFIG="${NVIM_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/nvim}"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/nvim"
LOG="$CACHE_DIR/lazy-update.log"
LOCK_BACKUP="$CACHE_DIR/lazy-lock.pre-update.json"
HEALTH_STDERR="$CACHE_DIR/lazy-update-health.stderr"
HEALTH_REPORT="$CACHE_DIR/lazy-update-health.txt"
HEALTH_BASELINE="$CACHE_DIR/lazy-update-health.baseline"

DRY_RUN=0
STRICT_HEALTH=0

usage() {
  sed -n '2,50p' "$0" | sed 's/^# \{0,1\}//'
}

for arg in "$@"; do
  case "$arg" in
    -n | --dry-run | --check-only) DRY_RUN=1 ;;
    --strict-health) STRICT_HEALTH=1 ;;
    -h | --help) usage; exit 0 ;;
    *)
      printf 'safe-update: unknown option: %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

log() { printf '[safe-update] %s\n' "$*"; }
die() {
  printf '[safe-update] ERROR: %s\n' "$*" >&2
  exit 1
}

command -v nvim >/dev/null 2>&1 || die "nvim not found in PATH"
command -v git >/dev/null 2>&1 || die "git not found in PATH"
[ -d "$NVIM_CONFIG/.git" ] || die "$NVIM_CONFIG is not a git repo; cannot snapshot"
[ -f "$NVIM_CONFIG/lazy-lock.json" ] || die "lazy-lock.json not found in $NVIM_CONFIG"
mkdir -p "$CACHE_DIR"
cd "$NVIM_CONFIG"

# --- 1. Snapshot -------------------------------------------------------------
# The snapshot is the config rollback point. In a real run we commit the current
# tree so the hash captures today's WIP as well as the pre-update state.
SNAPSHOT="$(git rev-parse HEAD)"
if [ "$DRY_RUN" -eq 0 ]; then
  if [ -n "$(git status --porcelain)" ]; then
    log "uncommitted changes found — committing pre-update snapshot"
    git add -A
    git commit -q -m "chore: pre-plugin-update snapshot $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    SNAPSHOT="$(git rev-parse HEAD)"
  fi
  log "config snapshot (rollback point): $SNAPSHOT"
elif [ -n "$(git status --porcelain)" ]; then
  log "[dry-run] tree is dirty; a real run would commit a snapshot first"
fi

# --- 2. Back up the lockfile -------------------------------------------------
# lazy-lock.json is the canonical plugin rollback: `:Lazy restore` replays it.
if [ "$DRY_RUN" -eq 0 ]; then
  cp -f lazy-lock.json "$LOCK_BACKUP"
  log "lockfile backed up -> $LOCK_BACKUP"
fi

# --- 3. Update ---------------------------------------------------------------
if [ "$DRY_RUN" -eq 1 ]; then
  log "[dry-run] running 'Lazy! check' (read-only)"
  LAZY_CMD="+Lazy! check"
else
  log "running headless 'Lazy! sync' (install + update + clean)"
  LAZY_CMD="+Lazy! sync"
fi

if nvim --headless "$LAZY_CMD" +qa >"$LOG" 2>&1; then
  log "lazy command finished (exit 0), output: $LOG"
else
  rc=$?
  log "lazy command FAILED (exit $rc) — log: $LOG"
  if [ "$DRY_RUN" -eq 0 ]; then
    log "restoring plugins from the pre-update lockfile"
    cp -f "$LOCK_BACKUP" lazy-lock.json
    nvim --headless "+Lazy! restore" +qa >>"$LOG" 2>&1 || true
  fi
  printf '[safe-update] Roll back config: git -C %s reset --hard %s\n' "$NVIM_CONFIG" "$SNAPSHOT" >&2
  exit 1
fi

# --- 4. Health check ---------------------------------------------------------
# Warm caches first so a first-run message cannot be mistaken for a regression.
nvim --headless +qa >/dev/null 2>&1 || true
: >"$HEALTH_STDERR"
if nvim --headless +qa >/dev/null 2>"$HEALTH_STDERR"; then
  health_rc=0
else
  health_rc=$?
fi
stderr_bytes="$(wc -c <"$HEALTH_STDERR" | tr -d ' ')"
log "health: headless exit=$health_rc stderr_bytes=$stderr_bytes"

# Informational checkhealth baseline (only enforced with --strict-health).
nvim --headless "+checkhealth" "+silent! w! $HEALTH_REPORT" +qa >/dev/null 2>&1 || true
errors="$(grep -c 'ERROR' "$HEALTH_REPORT" 2>/dev/null || true)"
errors="${errors:-0}"
if [ -f "$HEALTH_BASELINE" ]; then
  base="$(cat "$HEALTH_BASELINE" 2>/dev/null || printf '%s' "$errors")"
  log "checkhealth ERROR lines: $errors (baseline $base)"
else
  base="$errors"
  printf '%s\n' "$errors" >"$HEALTH_BASELINE"
  log "checkhealth ERROR lines: $errors (baseline saved)"
fi

health_ok=1
if [ "$health_rc" -ne 0 ] || [ "$stderr_bytes" -ne 0 ]; then
  health_ok=0
fi
if [ "$STRICT_HEALTH" -eq 1 ] && [ "$errors" -gt "$base" ]; then
  log "checkhealth ERRORs grew beyond baseline (--strict-health)"
  health_ok=0
fi

# --- 5. Rollback on failure --------------------------------------------------
if [ "$health_ok" -eq 0 ]; then
  if [ "$DRY_RUN" -eq 0 ]; then
    log "HEALTH CHECK FAILED — restoring plugins from the pre-update lockfile"
    cp -f "$LOCK_BACKUP" lazy-lock.json
    nvim --headless "+Lazy! restore" +qa >>"$LOG" 2>&1 || true
    log "plugins restored from $LOCK_BACKUP"
  fi
  {
    printf '[safe-update] Rollback instructions:\n'
    printf '  plugins: cp %s %s/lazy-lock.json && nvim --headless "+Lazy! restore" +qa\n' \
      "$LOCK_BACKUP" "$NVIM_CONFIG"
    printf '  config : git -C %s reset --hard %s\n' "$NVIM_CONFIG" "$SNAPSHOT"
  } >&2
  log "update FAILED (log: $LOG)"
  exit 1
fi

# --- 6. Summary --------------------------------------------------------------
log "update OK (log: $LOG)"
if [ "$DRY_RUN" -eq 0 ] && [ -f "$LOCK_BACKUP" ]; then
  changed="$(diff "$LOCK_BACKUP" lazy-lock.json 2>/dev/null | grep -c '^[<>]' || true)"
  changed="${changed:-0}"
  log "lockfile lines changed: $changed (< pre-update, > post-update)"
  diff "$LOCK_BACKUP" lazy-lock.json 2>/dev/null | grep '^[<>]' || true
fi
log "config snapshot (rollback point): $SNAPSHOT"
exit 0
