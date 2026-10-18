#!/usr/bin/env python3
"""A stub terminal for scripts/measure-terminal-ownership.sh.

The installer's probe asks three questions -- DECRQM for mode 2026 and two
XTGETTCAP asks, for RGB and for Ms -- and reads the answers off the same
terminal it is reading keys from. This script stands in for the terminal on the
far side: it reads the program's output (which the caller pipes in from the pty
master), answers the query as a terminal that supports all three would, and
writes the answer to stdout, which the caller feeds back into the pty.

It reports on stderr, never on stdout, so its own reports cannot be mistaken for
a terminal reply.
"""

import os
import select
import sys
import time

# The exact payload the installer sends, and a real terminal's answer to it:
# DECRQM says mode 2026 is recognized and currently reset, and each XTGETTCAP
# reply is a success ("1+r<key>=<value>", the value hex "31" = "1").
QUERY = b"\x1b[?2026$p" + b"\x1bP+q524742\x1b\\" + b"\x1bP+q4D73\x1b\\"
REPLY = b"\x1b[?2026;2$y" + b"\x1bP1+r524742=31\x1b\\" + b"\x1bP1+r4D73=31\x1b\\"

# Lines of the child's own report worth showing the human reading the measurement.
#
# The match is on the file the test logs from, not on a list of case letters. A
# whitelist of letters silently dropped the `case E:` line this front added for T5:
# the line was printed, the filter did not forward it, and a line that was never
# forwarded reads exactly like a line that was never printed. Anything the
# diagnostic says, it now says through this prefix.
REPORTABLE = (b"probe_ownership_measure_test.go:", b"PASS", b"FAIL", b"skip")


def log(message):
    sys.stderr.write("stub: %s\n" % message)
    sys.stderr.flush()


def report(data):
    """Forward the child's own report lines to the human, not its frames."""
    for line in data.replace(b"\r\n", b"\n").split(b"\n"):
        if any(marker in line for marker in REPORTABLE):
            sys.stderr.write("stub| %s\n" % line.decode("utf-8", "replace"))
    sys.stderr.flush()


def main():
    started = time.time()
    seen = b""
    queries = 0
    while True:
        # Before the first query the program may still be building; after one it
        # has answered, so only a short grace period is needed to keep forwarding
        # the program's own report before the pty closes.
        idle = 15 if queries == 0 else 2
        ready, _, _ = select.select([sys.stdin], [], [], idle)
        if not ready:
            log("the program produced nothing for %ds; stopping" % idle)
            return
        data = os.read(sys.stdin.fileno(), 4096)
        if not data:
            log("the pty closed after %d quer%s; stopping"
                % (queries, "y" if queries == 1 else "ies"))
            return
        report(data)
        seen = (seen + data)[-8192:]
        if QUERY in seen:
            queries += 1
            log("query seen at %.3fs; answering with %d bytes" % (time.time() - started, len(REPLY)))
            os.write(sys.stdout.fileno(), REPLY)
            sys.stdout.flush()
            seen = b""


main()
