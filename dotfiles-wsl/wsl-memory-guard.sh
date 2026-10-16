#!/usr/bin/env bash
# Cap init.scope so that a runaway workload cannot take the whole distribution
# down with it -- and, what matters just as much, cannot freeze it instead.
#
# Evidence, 2026-10-16. Three events in a VM limited to memory=7680MB:
#
#   11:39:48  oom-kill: ... global_oom, task_memcg=/init.scope, task=pi
#             (969 MB anon-rss). The distribution died ~90 s later.
#   11:53:54  the same, 1283 MB anon-rss. The distribution died ~90 s later.
#   12:26:30  a full `go test ./internal/tui/` at 12 vCPUs. The distribution
#             became completely unresponsive for about six minutes and had to
#             be killed from Windows. No OOM line survived, because none was
#             written: nothing was killed.
#
# Those are two different failures, and a guard that fixes only one of them
# makes the other worse. That is why this script sets three things.
#
# 1. MemoryMax = total - 1536 MiB. Past it the cgroup OOM killer picks one task,
#    which is the failure mode that recovers by itself. This cap is only
#    reachable if nothing throttles below it.
#
# 2. No MemoryHigh, and MemorySwapMax bounded. Past memory.high the kernel puts
#    the tasks of the cgroup under heavy reclaim pressure and sleeps them, and
#    normal allocation cannot get past the throttle to memory.max, so the OOM
#    killer is never reached. For a service that is the point: slow the noisy
#    neighbour down. For init.scope, which in WSL holds every process in the
#    distribution including PID 1, systemd and journald, "slow down" means
#    "stop" -- the 1 GiB between the old high and max, plus 1.5 GiB of swap, was
#    2.5 GiB of unresponsiveness with nothing killed to recover from it. The
#    12:26 event is exactly that. MemorySwapMax bounds the same window from the
#    other side: memory.max does not count swap, so an unlimited swap.max would
#    let a task keep running past memory.max by pushing its anonymous pages out.
#
# 3. OOMPolicy=continue. systemd ships init.scope with OOMPolicy=stop
#    (DefaultOOMPolicy=stop), so when the kernel killed any task in the scope,
#    systemd stopped the whole unit: SIGKILL to every process in the
#    distribution. That is what took the distribution down at 11:39 and 11:53 --
#    the killed process was only the trigger. Nothing had noticed this half, and
#    without it a memory cap only makes the teardown happen sooner.
#
# Why init.scope: in WSL every terminal process lands there (there is no logind
# session around a wsl.exe relay), so it is the only cgroup that can hold the
# limit. PID 1 lives in it as well; the kernel never picks global init as an OOM
# victim, and it is never the largest task in the group.
#
# Why through systemd and not by writing the cgroup attribute: a direct write is
# reverted by the next `systemctl daemon-reload`, which happens on any package
# install. systemd then re-applies the value it knows for the unit, and it knows
# "unlimited". Measured on systemd 257: `set-property --runtime` is accepted on
# init.scope, it stores per-property drop-ins under /run/systemd/transient/, and
# the value survives a reload because systemd re-applies its own number. /run
# does not survive a reboot, so nothing goes stale when the VM changes size:
# this script recomputes at every boot.
#
# The reserve is absolute rather than proportional: what the distribution needs
# in order to stop cleanly is a fixed amount of memory, and the failure being
# prevented is exactly a stop that cannot finish.
#
# Every failure path either says why it cannot act, or acts and reports that the
# result is not durable. The guard never claims a protection it did not verify:
# after asking systemd it reads every attribute back and compares it.
set -euo pipefail

# The paths are overridable so every branch below can be exercised against a
# fabricated cgroup, an ordinary file and a stub. The unit that runs this in
# production sets none of them, so all three defaults are what a boot uses.
meminfo=${WSL_GUARD_MEMINFO:-/proc/meminfo}
cgroup=${WSL_GUARD_CGROUP:-/sys/fs/cgroup/init.scope}
systemctl=${WSL_GUARD_SYSTEMCTL:-systemctl}

# The headroom the distribution keeps for itself, and the swap it may use before
# the cap bites. Both are absolute for the same reason: a stop that cannot
# finish needs a fixed amount of memory, not a share of a machine.
reserve_max_mib=1536
swap_max_mib=512
min_total_mib=4096
bytes_per_mib=1048576

# want_policy is the OOMPolicy the scope needs so that one killed task does not
# become every killed task.
want_policy=continue

# It cannot be asked for the way the cap is. Measured on systemd 257:
# `set-property --runtime init.scope OOMPolicy=continue` is refused, because
# OOMPolicy is a unit property rather than a cgroup attribute and set-property
# accepts only the resource-control ones. A drop-in scoped to init.scope is tried
# instead, and which of the two worked is never assumed: the effective policy is
# read back after each attempt, and a refusal is reported rather than papered
# over.
dropin=${WSL_GUARD_OOMPOLICY_DROPIN:-/etc/systemd/system/init.scope.d/10-oompolicy.conf}
dropin_body='# Written by the WSL memory guard (dotfiles). Do not edit: the guard
# rewrites this file whenever the effective policy is not OOMPolicy=continue.
#
# Why the policy lives here and not in the guard service: init.scope is a
# transient scope systemd creates itself, and set-property accepts only
# resource-control properties for it. This policy is what keeps one OOM kill
# inside the scope from making systemd stop the scope -- and in WSL every process
# in the distribution lives in that scope.
[Scope]
OOMPolicy=continue
'

if [ ! -e "$cgroup/memory.max" ]; then
  echo "wsl-memory-guard: $cgroup has no memory controller; nothing to cap" >&2
  exit 0
fi

total_kib=$(awk '/^MemTotal:/ {print $2}' "$meminfo")
case "$total_kib" in
  '' | *[!0-9]*)
    echo "wsl-memory-guard: could not read MemTotal from $meminfo; leaving it uncapped" >&2
    exit 0
    ;;
esac

total_mib=$((total_kib / 1024))

if [ "$total_mib" -lt "$min_total_mib" ]; then
  echo "wsl-memory-guard: ${total_mib}MiB total is too small to divide; leaving it uncapped" >&2
  exit 0
fi

max_mib=$((total_mib - reserve_max_mib))
want_bytes=$((max_mib * bytes_per_mib))
want_swap_bytes=$((swap_max_mib * bytes_per_mib))

# Only if the kernel supports it. At 1 the killer would take every task in the
# group, which is the failure this guard exists to avoid. systemd agrees with the
# value (its own default for the unit is 0), so a reload cannot undo it.
if [ -e "$cgroup/memory.oom.group" ]; then
  echo 0 > "$cgroup/memory.oom.group"
fi

status=0

# The cap, and explicitly no throttle above it. MemoryHigh is set to its
# unlimited value rather than left alone: the drop-in an earlier version of this
# guard wrote stays for the life of the boot, and leaving it in place would keep
# the throttle that froze the distribution.
if "$systemctl" set-property --runtime init.scope \
  "MemoryHigh=infinity" "MemoryMax=${max_mib}M" "MemorySwapMax=${swap_max_mib}M"; then
  got_bytes=$(cat "$cgroup/memory.max" 2>/dev/null || echo "")
  got_swap_bytes=$(cat "$cgroup/memory.swap.max" 2>/dev/null || echo "unset")
  got_high=$(cat "$cgroup/memory.high" 2>/dev/null || echo "unset")

  case "$got_bytes" in
    "$want_bytes")
      case "$got_high" in
        max) high_ok=1 ;;
        *) high_ok=0 ;;
      esac
      case "$got_swap_bytes" in
        "$want_swap_bytes" | unset) swap_ok=1 ;;
        *) swap_ok=0 ;;
      esac
      if [ "$high_ok" = 1 ] && [ "$swap_ok" = 1 ]; then
        echo "wsl-memory-guard: ${total_mib}MiB VM capped at ${max_mib}M with no throttle band (swap max ${swap_max_mib}M), owned by systemd"
      else
        echo "wsl-memory-guard: memory.max is right but the throttle band is not: memory.high reads '${got_high}' (want max) and memory.swap.max reads '${got_swap_bytes}' (want ${want_swap_bytes}); a spike can still hang the distribution instead of being killed" >&2
        status=1
      fi
      ;;
    *)
      echo "wsl-memory-guard: set-property reported success but $cgroup/memory.max reads '${got_bytes}', expected ${want_bytes}" >&2
      status=1
      ;;
  esac
else
  # systemd refused the property. A direct write still protects this boot, but the
  # next daemon-reload clears it, and the exit status says so.
  #
  # The attributes take bytes (or the literal "max"), never the "8409M" form the
  # drop-in uses: writing the suffixed form straight into the cgroup is rejected
  # with EINVAL and leaves the scope uncapped, which is the one outcome this
  # branch exists to avoid.
  if [ -e "$cgroup/memory.max" ]; then
    echo max > "$cgroup/memory.high"
    echo "$want_bytes" > "$cgroup/memory.max"
    if [ -e "$cgroup/memory.swap.max" ]; then
      echo "$want_swap_bytes" > "$cgroup/memory.swap.max"
    fi
  fi
  echo "wsl-memory-guard: set-property was refused; the cap is set but NOT durable (a daemon-reload clears it)" >&2
  status=1
fi

# The half that turns "one process died" into "the distribution survived".
policy_now() { "$systemctl" show init.scope -p OOMPolicy --value 2>/dev/null || echo ""; }

got_policy=$(policy_now)
if [ "$got_policy" != "$want_policy" ]; then
  # No footprint at all: ask systemd to own the property for this boot. It does
  # not on systemd 257, which is why the drop-in below exists, but trying costs
  # nothing and it is the right answer wherever systemd does accept it.
  "$systemctl" set-property --runtime init.scope "OOMPolicy=${want_policy}" 2>/dev/null || true
  got_policy=$(policy_now)
fi

if [ "$got_policy" != "$want_policy" ]; then
  # A drop-in scoped to init.scope: the narrowest thing that can carry a unit
  # property. Written only when its content differs, so a boot that is already
  # right does not touch the file, and rewritten whenever something else put a
  # different value there.
  #
  # The manager-wide alternative -- DefaultOOMPolicy=continue in
  # /etc/systemd/system.conf -- is deliberately NOT written here. It would change
  # the OOM semantics of every unit on the machine, and a service whose main
  # process is OOM-killed would no longer be marked failed, so its Restart= would
  # not fire. That trade belongs to the machine's owner, so this script reports it
  # and fails instead of taking it.
  if [ "$(cat "$dropin" 2>/dev/null || true)" != "$dropin_body" ]; then
    if mkdir -p "$(dirname "$dropin")" 2>/dev/null && printf '%s' "$dropin_body" > "$dropin" 2>/dev/null; then
      "$systemctl" daemon-reload
    else
      echo "wsl-memory-guard: could not write $dropin; the OOMPolicy half is not in place" >&2
    fi
  fi
  got_policy=$(policy_now)
fi

if [ "$got_policy" = "$want_policy" ]; then
  echo "wsl-memory-guard: init.scope OOMPolicy is ${want_policy}, so an OOM kill in the scope no longer stops it"
else
  echo "wsl-memory-guard: init.scope OOMPolicy reads '${got_policy}', so one OOM kill still stops the scope and takes every process in the distribution with it. Neither set-property nor a drop-in scoped to init.scope changed it, so the only remaining route is the manager default, which reaches every unit: set DefaultOOMPolicy=${want_policy} in /etc/systemd/system.conf, run systemctl daemon-reload, restart this unit, and give any service whose Restart= matters an explicit OOMPolicy=stop." >&2
  status=1
fi

exit "$status"
