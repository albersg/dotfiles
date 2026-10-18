package main

import (
	"io"
	"os"

	"github.com/albersg/dotfiles/installer/internal/tui"
	"github.com/charmbracelet/x/term"
)

// The program's input.
//
// The terminal gets one reader: the interface reads the terminal through it, and
// the capability probe takes its answer from that same stream, so a reply to a
// query cannot be eaten by the interface's reader and cannot arrive at the
// interface as keystrokes (odd/tasks/terminal-probe-ownership.md).
//
// Handing that reader over is a decision, not a default, and the decision is
// bubbletea's to make. With no input option bubbletea uses stdin when it is a
// terminal and opens the controlling terminal when it is not (`tty_unix.go:17`
// takes stdin when `term.IsTerminal(f.Fd())` holds; `tea.go:604-617` opens
// `/dev/tty` otherwise). Passing any input option takes that choice away -- a
// program given a pipe reads the pipe, and no key ever reaches it -- so the
// reader is handed over exactly when bubbletea would have chosen stdin itself.
//
// The predicate below is therefore bubbletea's own, on the same descriptor: the
// wrapper reports stdin's descriptor, so the stream the interface reads is the
// stream bubbletea would have read, and the only thing that changes is that the
// probe's answer leaves it.
//
// Two tempting tests are wrong here, and both were tried before this one, which
// is why they are written down. `isTerminal` (sync_output.go) is `ModeCharDevice`,
// and `/dev/null` satisfies it: a run started with it as its input would have
// been handed a stream that never carries a key. And comparing the devices with
// `os.SameFile` against an opened `/dev/tty` matches **nothing, ever**: that
// descriptor reports the devtmpfs node (dev 5, inode 10) and not the terminal it
// stands for, so the check answered false even with the terminal on stdin and
// the reader was never handed over -- the change was inert until the guard's own
// evidence (both devices, printed in its skip message) said so.
func programInput(stdin *os.File) io.Reader {
	if stdin == nil || !term.IsTerminal(stdin.Fd()) {
		return nil
	}
	return tui.TerminalInput(stdin)
}
