#!/usr/bin/env bash
# Install the WSL memory guard and prove it works.
#
# Run it once, from inside the distribution, from the checkout that holds it. It
# needs sudo.
#
# It never runs `wsl --shutdown`: that is what applies .wslconfig, and it would
# kill the session running this script. Its last lines tell you to run it from
# Windows.
#
# The installer applies the same artifact and the same verification as the
# Configure WSL step of `dotfiles`; this script exists for the case where you want
# the guard without running the installer, which is also how the 2026-10-16
# incident was recovered.
#
# A guard that reports a protection it could not put in place exits non-zero, and
# that exit must not stop this script before it has printed why. Measured on the
# first version: `systemctl restart` failed, `set -e` ended the run right there,
# and the only thing the user saw was "Job for wsl-memory-guard.service failed
# because the control process exited with error code" -- a sentence that says
# nothing about which protection is missing or what to do about it. So the
# restart's status is captured, the guard's own report is printed either way, and
# every check below still runs.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
payload_src="$here/wsl-memory-guard.sh"
payload_dst=/usr/local/lib/dotfiles-wsl/wsl-memory-guard.sh
unit_src="$here/wsl-memory-guard.service"
unit_dst=/etc/systemd/system/wsl-memory-guard.service

if [ ! -f "$payload_src" ] || [ ! -f "$unit_src" ]; then
  echo "!! this script must sit beside wsl-memory-guard.sh and wsl-memory-guard.service;" >&2
  echo "   run it from the checkout's dotfiles-wsl/ directory." >&2
  exit 1
fi

echo "==> installing $payload_dst"
sudo install -D -m 0755 "$payload_src" "$payload_dst"
echo "==> installing $unit_dst"
sudo install -D -m 0644 "$unit_src" "$unit_dst"

echo "==> reloading systemd, then enabling and restarting the guard"
sudo systemctl daemon-reload
sudo systemctl enable wsl-memory-guard.service

restart_rc=0
sudo systemctl restart wsl-memory-guard.service || restart_rc=$?

failed=0

live() {
  printf 'systemd: high=%s max=%s swap=%s oom_policy=%s | cgroup: high=%s max=%s swap.max=%s oom.group=%s\n' \
    "$(systemctl show init.scope -p MemoryHigh --value)" \
    "$(systemctl show init.scope -p MemoryMax --value)" \
    "$(systemctl show init.scope -p MemorySwapMax --value)" \
    "$(systemctl show init.scope -p OOMPolicy --value)" \
    "$(cat /sys/fs/cgroup/init.scope/memory.high)" \
    "$(cat /sys/fs/cgroup/init.scope/memory.max)" \
    "$(cat /sys/fs/cgroup/init.scope/memory.swap.max 2>/dev/null || echo unsupported)" \
    "$(cat /sys/fs/cgroup/init.scope/memory.oom.group 2>/dev/null || echo unsupported)"
}

echo "    $(live)"

echo "==> what the guard reported"
sudo journalctl -u wsl-memory-guard.service -n 8 --no-pager -o cat 2>&1 | sed 's/^/    /' || true

if [ "$restart_rc" -ne 0 ]; then
  echo
  echo "!!  the guard reported at least one protection it could not put in place;"
  echo "    the line above beginning 'wsl-memory-guard:' names it and the remedy."
  failed=1
fi

echo "==> checking the cap and the throttle survive a systemd reload"
sudo systemctl daemon-reload
if [ "$(cat /sys/fs/cgroup/init.scope/memory.max)" = "max" ]; then
  echo "!!  a daemon-reload cleared the cap; report this output"
  failed=1
fi
if [ "$(cat /sys/fs/cgroup/init.scope/memory.high)" != "max" ]; then
  echo "!!  a throttle is set again; a spike would hang the distribution instead of being killed"
  failed=1
fi
if [ "$failed" -eq 0 ]; then
  echo "    survived: $(live)"
fi

# The policy is the half that turns one killed process into a surviving
# distribution. On 2026-10-16 a single OOM kill in init.scope took every process
# with it, twice, so it is checked here rather than trusted to the guard alone.
if [ "$(systemctl show init.scope -p OOMPolicy --value)" != "continue" ]; then
  echo "!!  init.scope OOMPolicy is not 'continue', so one OOM kill still stops the whole"
  echo "    scope and takes the distribution with it. The guard tries set-property and a"
  echo "    drop-in at /etc/systemd/system/init.scope.d/10-oompolicy.conf; if neither took,"
  echo "    run: systemctl status wsl-memory-guard.service --no-pager"
  failed=1
fi

echo
if [ "$failed" -ne 0 ]; then
  echo "NOT PROTECTED: a protection the guard claims is not in place. Nothing below is a"
  echo "suggestion -- read the '!!' lines above, which name each missing half and its remedy."
  exit 1
fi

echo "OK: init.scope is capped without a throttle band, OOMPolicy=continue, and a reload keeps both."
echo
echo "Remaining step, run it from Windows. It closes every WSL session, including"
echo "the one reading this, and it is what applies memory=10240MB:"
echo
echo "    wsl --shutdown"
echo
echo "After reopening, confirm both halves:"
echo
echo "    free -h                                     # expect ~9.6Gi total"
echo "    cat /sys/fs/cgroup/init.scope/memory.high   # expect max, never a number"
echo "    cat /sys/fs/cgroup/init.scope/memory.max    # expect ~8380M, scaled to the new VM"
echo "    systemctl show init.scope -p OOMPolicy --value   # expect continue"
echo "    systemctl status wsl-memory-guard --no-pager"
