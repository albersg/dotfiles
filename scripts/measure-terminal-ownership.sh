#!/usr/bin/env bash
# measure-terminal-ownership.sh -- who reads the terminal when the capability
# probe queries it. The diagnostic of odd/tasks/terminal-probe-ownership.md (T2).
#
# usage: scripts/measure-terminal-ownership.sh diverted|notowner [runs]
#
#   diverted   the interface reading through TerminalInput, which is what
#              cmd/dotfiles starts: one reader, and the probe's answer leaves it
#   notowner   no reader of ours and stdin away from the pty, so the interface
#              reads the controlling terminal for itself: the arrangement in which
#              the probe must write no query at all (T5)
#
# Three modes are retired with the code that could produce their arrangement, and
# the readings they gave are in this front's log rather than here. `shared` (the
# interface on the terminal's input with the probe opening it for itself) is the
# defect's own arrangement. `alone` (the probe asking with nobody else reading)
# cannot exist any more either, and its last round shows why: `case A: probe alone
# received 0 bytes` against the stub's own `query seen ... answering with 41
# bytes` -- the query was written and answered, and with no reader loop running
# nobody read the answer. `canonical` went with `alone`: the probe no longer reads
# a stream it opened, so whether the terminal is raw is the interface's business,
# and the interface makes it raw.
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
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/.." && pwd)"
stub="$here/measure-terminal-ownership-stub.py"
# Every mode has a program running, and the interface sets the terminal raw in
# production, so the terminal is held raw for all of them.
stty_args='raw -echo'

case "$case_name" in
  diverted | notowner) ;;
  *)
    echo "usage: $0 diverted|notowner [runs]" >&2
    exit 2
    ;;
esac

# The notowner case is the arrangement T5 is about: the installer's input is not
# the terminal, so bubbletea opens the controlling terminal for itself. Running the
# binary with its stdin away from the pty reproduces it, while the pty stays the
# controlling terminal.
stdin_redirect=""
if [ "$case_name" = "notowner" ]; then
  stdin_redirect="< /dev/null"
fi

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
  echo "--- $case_name run $run/$runs ---"
  # The stub answers on demand, so the measurement does not depend on how long
  # the binary takes to start.
  coproc STUB { python3 -u "$stub"; }
  timeout 180 script -qec \
    "stty $stty_args; DOTFILES_PROBE_MEASURE=$case_name $testbin -test.run TestMeasureWhoReadsTheTerminal -test.v $stdin_redirect" \
    /dev/null <&"${STUB[0]}" >&"${STUB[1]}"
  status=$?
  echo "--- $case_name run $run/$runs exited $status ---"
  wait "$STUB_PID" 2>/dev/null
done
