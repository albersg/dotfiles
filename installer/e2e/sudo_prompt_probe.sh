#!/bin/sh
# Container-side probe for the claim PR #304 makes: a whole install asks for the
# administrator password once, and only once.
#
# The Go tests prove the generated scripts carry `sudo -n` and no keypress
# prompt. They cannot prove what a real sudo does, because sudo caches the
# credential: a run whose scripts all used plain `sudo` would also ask only once
# while the cache stayed warm, and the cached credential is the reason the old
# installer and the new one look alike from the outside. This probe clears the
# cache (`sudo -k`) before every step script, so each script starts as if its
# step were the first, and counts how many of them ask.
#
# Counting is done by the askpass program sudo runs in place of a terminal
# prompt (see /home/tester/bin/askpass): one append to the log is one time a
# person would have been asked for the password. A wrapper at
# /home/tester/bin/sudo makes plain `sudo` read its password through that
# program, because this container has no terminal for sudo to prompt on. `sudo
# -n` never reaches it: with a cleared cache it fails immediately and asks
# nothing, which is exactly the property under test.
#
# Usage: called by sudo-prompt-test.sh with the rendered scripts mounted at
# STEP_SCRIPTS_DIR. Reads manifest.txt in the mount for the step order.

set -u

SCRIPTS_DIR="${STEP_SCRIPTS_DIR:-/opt/step-scripts}"
MANIFEST="$SCRIPTS_DIR/manifest.txt"
LOG="/home/tester/sudo-prompts.log"

# The counter and the -A wrapper live in /home/tester/bin, ahead of /usr/bin, so
# plain `sudo` in a rendered script reads its password through the counter.
export SUDO_ASKPASS="/home/tester/bin/askpass"
export PATH="/home/tester/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_test() { printf "${YELLOW}[TEST]${NC} %s\n" "$1"; }
log_pass() { printf "${GREEN}[PASS]${NC} %s\n" "$1"; }
log_fail() { printf "${RED}[FAIL]${NC} %s\n" "$1"; }
log_section() {
    printf "\n${CYAN}════════════════════════════════════════${NC}\n"
    printf "${CYAN}  %s${NC}\n" "$1"
    printf "${CYAN}════════════════════════════════════════${NC}\n\n"
}

# prompt_count is the number of times sudo has asked for the password so far.
prompt_count() {
    if [ -f "$LOG" ]; then
        wc -l < "$LOG" | tr -d ' '
    else
        echo 0
    fi
}

if [ ! -f "$MANIFEST" ]; then
    log_fail "no manifest at $MANIFEST; the scripts were not rendered"
    exit 2
fi

: > "$LOG"

log_section "One password, one prompt"

# The primer is the single credential request the fixed run makes before the
# first root step. It is run first, with a cleared cache, so its prompt is
# counted on its own. Every step script afterwards clears the cache again: that
# is what makes the measurement sensitive to a script that prompts instead of to
# a cache that happens to be warm.
log_test "primer: sudo -v asks for the credential once"
before=$(prompt_count)
sudo -k
sudo -v >/dev/null 2>&1
primer_prompts=$(( $(prompt_count) - before ))
if [ "$primer_prompts" -eq 1 ]; then
    log_pass "the primer asked once"
else
    log_fail "the primer asked $primer_prompts times, want 1"
fi

# A prompt is only half the claim: the credential it wrote has to be usable by
# the non-interactive `sudo -n` the step scripts run, with no terminal. This
# proves the cache the primer warms is the cache the steps read.
before=$(prompt_count)
sudo -n true >/dev/null 2>&1
primed_ok=$?
primed_prompts=$(( $(prompt_count) - before ))
if [ "$primed_ok" -eq 0 ] && [ "$primed_prompts" -eq 0 ]; then
    log_pass "sudo -n used the cached credential without asking"
else
    log_fail "sudo -n did not use the cached credential (exit $primed_ok, $primed_prompts prompt(s))"
fi

steps=0
prompting_steps=0
step_prompts=0

while IFS= read -r step; do
    [ -z "$step" ] && continue
    script="$SCRIPTS_DIR/$step.sh"
    if [ ! -f "$script" ]; then
        log_fail "step $step has no rendered script at $script"
        exit 2
    fi

    steps=$((steps + 1))
    before=$(prompt_count)
    # Clear the cached credential so this script starts cold, exactly as the
    # first root-needing step of a run does.
    sudo -k
    # stdin from /dev/null: the run has no terminal, and the old scripts'
    # enter-to-continue reads must not block the count waiting for a keypress.
    /bin/sh "$script" </dev/null >/dev/null 2>&1 || true
    asked=$(( $(prompt_count) - before ))
    step_prompts=$((step_prompts + asked))
    if [ "$asked" -eq 0 ]; then
        log_pass "step $step did not ask"
    else
        log_fail "step $step asked $asked time(s)"
        prompting_steps=$((prompting_steps + 1))
    fi
done < "$MANIFEST"

total=$((primer_prompts + step_prompts))
printf "\n"
printf "SUDO_PRIMED_CACHE=%s\n" "$primed_ok"
printf "SUDO_PROMPTS_PRIMER=%s\n" "$primer_prompts"
printf "SUDO_PROMPTS_STEPS=%s\n" "$step_prompts"
printf "SUDO_PROMPTING_STEPS=%s/%s\n" "$prompting_steps" "$steps"
printf "SUDO_PROMPTS_TOTAL=%s\n" "$total"

# The numbers are always printed. They are asserted only when the caller asked
# for an expectation, which lets the same probe both prove the fixed tree and
# measure a tree that is expected to fail.
status=0
if [ "$primed_ok" -ne 0 ] || [ "$primed_prompts" -ne 0 ]; then
    status=1
fi
if [ -n "${EXPECT_TOTAL:-}" ] && [ "$total" -ne "$EXPECT_TOTAL" ]; then
    log_fail "whole run asked $total time(s), want $EXPECT_TOTAL"
    status=1
fi
if [ -n "${EXPECT_STEPS:-}" ] && [ "$step_prompts" -ne "$EXPECT_STEPS" ]; then
    log_fail "step scripts asked $step_prompts time(s), want $EXPECT_STEPS"
    status=1
fi
if [ "$status" -eq 0 ] && [ -n "${EXPECT_TOTAL:-}" ]; then
    log_pass "the whole run asked $total time(s), as expected"
fi

exit "$status"
