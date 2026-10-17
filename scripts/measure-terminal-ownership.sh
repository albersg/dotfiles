#!/usr/bin/env bash
# measure-terminal-ownership.sh -- who reads the terminal when the capability
# probe queries it. The diagnostic of odd/tasks/terminal-probe-ownership.md (T2).
#
# usage: scripts/measure-terminal-ownership.sh alone|shared|diverted [runs] [raw|canonical]
#
#   alone      the probe on a terminal nobody else is reading (the baseline)
#   shared     the probe with the interface reading os.Stdin, as cmd/dotfiles used
#              to start it: the arrangement the defect was measured in
#   diverted   the probe with the interface reading through TerminalInput, which
#              is what cmd/dotfiles does now: one reader, the answer diverted
#   raw        the terminal in the raw mode the interface sets in production (default)
#   canonical  the terminal as it is before the interface starts, which is what a
#              probe moved ahead of the interface would find
#
# Each run gives the child a controlling terminal from script(1), holds the
# terminal in the raw mode the interface sets in production, and puts a stub
# terminal (scripts/measure-terminal-ownership-stub.py) on the far side that
# answers the probe's query the way a real terminal does. The probe reaches the
# terminal through /dev/tty, exactly as it does in production, so the only
# difference between the two cases is which readers are attached.
#
# It asserts nothing: it prints what each reader received, and the reading is
# recorded in the front's log.
set -uo pipefail

case_name="${1:-}"
runs="${2:-3}"
term_mode="${3:-raw}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/.." && pwd)"
stub="$here/measure-terminal-ownership-stub.py"

case "$case_name" in
  alone | shared | diverted) ;;
  *)
    echo "usage: $0 alone|shared|diverted [runs] [raw|canonical]" >&2
    exit 2
    ;;
esac

case "$term_mode" in
  raw) stty_args='raw -echo' ;;
  canonical) stty_args='sane' ;;
  *)
    echo "usage: $0 alone|shared|diverted [runs] [raw|canonical]" >&2
    exit 2
    ;;
esac

for tool in script python3 go; do
  command -v "$tool" >/dev/null || {
    echo "$0: $tool is not installed" >&2
    exit 2
  }
done

cd "$root/installer" || exit 2

# The test binary is built once and then run under script(1) directly, rather
# than through `go test`: `go test` hands the binary a stdin that is not the
# terminal it was started from, and this measurement is about which reader the
# terminal's answer reaches. Running the binary itself is also what puts the
# pty on stdin, which is the arrangement production has.
work="$(mktemp -d)"
testbin="$work/tui.test"
trap 'rm -rf "$work"' EXIT
if ! go test -c -o "$testbin" ./internal/tui; then
  echo "$0: could not build the test binary" >&2
  exit 2
fi

for run in $(seq 1 "$runs"); do
  echo "--- $case_name run $run/$runs ($term_mode terminal) ---"
  # The stub answers on demand, so the measurement does not depend on how long
  # the binary takes to start.
  coproc STUB { python3 -u "$stub"; }
  timeout 180 script -qec \
    "stty $stty_args; DOTFILES_PROBE_MEASURE=$case_name $testbin -test.run TestMeasureWhoReadsTheTerminal -test.v" \
    /dev/null <&"${STUB[0]}" >&"${STUB[1]}"
  status=$?
  echo "--- $case_name run $run/$runs exited $status ---"
  wait "$STUB_PID" 2>/dev/null
done
