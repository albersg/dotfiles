package main

import (
	"io"
	"os"
)

// Synchronized output: the frame the terminal never shows half-painted.
//
// bubbletea's renderer builds each frame in a buffer and hands it to the
// program's writer in one Write call (standard_renderer.go's flush ends with
// `r.out.Write(buf.Bytes())`), but the terminal knows nothing about that: it
// paints the bytes as they arrive, so a frame written a line at a time is
// visible mid-update. That is the tearing the animation shows. The fix is the
// terminal's own frame buffer, the synchronized-output mode (DECSET 2026): the
// terminal holds everything between the begin and end sequences and shows it in
// one repaint.
//
// bubbletea v1.3.10 does not emit these sequences anywhere, and it does not need
// a custom renderer to be made to: `tea.WithOutput` is a documented extension
// point, so the whole change is a writer wrapped around the program's output.
// A terminal that does not implement the mode ignores the two sequences, which
// is why this needs no feature detection and why the worst case is exactly the
// behaviour of a build without it.
//
// The two rules below are the discipline of the writer:
//
//   - It is installed only when stdout is a terminal. A pipe or a file has
//     nobody painting frames, and the sequences in it would be noise, so a
//     redirected run keeps its bytes exactly as they are.
//   - DOTFILES_SYNC=0 turns it off, in the same spirit as the animation and
//     pointer gates: a multiplexer that mishandles the mode has to be escapable
//     without a rebuild.
const (
	// syncBegin opens a synchronized update: the terminal stops painting.
	syncBegin = "\x1b[?2026h"

	// syncEnd closes it: the terminal repaints once, with the whole frame.
	syncEnd = "\x1b[?2026l"

	// envSync names the environment switch that turns synchronized output off.
	envSync = "DOTFILES_SYNC"
)

// syncWriter brackets every write in the terminal's synchronized-output mode.
//
// It embeds the file it writes to rather than merely holding an io.Writer,
// because bubbletea decides whether the program's output is a terminal by
// type-asserting the writer to term.File, and it uses the result for more than
// the renderer: tty_unix.go and tty_windows.go set `p.ttyOutput` from that
// assertion, the window size is read from `p.ttyOutput.Fd()` (tty.go's
// checkResize), and on Windows the same branch turns on
// ENABLE_VIRTUAL_TERMINAL_PROCESSING. A plain wrapper holding an io.Writer would
// fail the assertion, so the program would never receive a WindowSizeMsg, and on
// Windows it would not even enable the escape processing this writer relies on.
// Keeping the file in the struct keeps Fd, Read and Close reaching the terminal,
// so nothing above the renderer can tell the wrapper from the stream.
type syncWriter struct {
	*os.File
}

// The shape bubbletea type-asserts its output to (x/term's File: a read/write
// closer with a file descriptor). Asserting it here means an edit that drops one
// of the promoted methods fails to build, instead of silently costing the program
// its window size and, on Windows, its escape processing.
var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = (*syncWriter)(nil)

// Write sends the begin sequence, the payload and the end sequence as one
// bracketed frame. It reports the payload's length, not the frame's, so it
// honours the io.Writer contract for the caller that matters.
func (w *syncWriter) Write(p []byte) (int, error) {
	// A zero-length write is not a frame: it must not open and close the mode
	// around nothing, which would toggle the terminal for no payload at all.
	if len(p) == 0 {
		return 0, nil
	}

	if _, err := io.WriteString(w.File, syncBegin); err != nil {
		return 0, err
	}

	n, err := w.File.Write(p)

	// Close the frame even when the payload failed, so a broken write cannot
	// leave the terminal buffering every later byte.
	if _, endErr := io.WriteString(w.File, syncEnd); endErr != nil && err == nil {
		err = endErr
	}

	return n, err
}

// syncGate answers whether this run may bracket its frames. stdout must be a
// terminal, because there is no one to show a frame to otherwise, and
// DOTFILES_SYNC=0 refuses even then. A non-terminal cannot be forced on: the
// sequences would be written into a file or a pipe, which is what the gate
// exists to prevent.
func syncGate(stdout *os.File) bool {
	if os.Getenv(envSync) == "0" {
		return false
	}
	return isTerminal(stdout)
}

// isTerminal reports whether f is a terminal-like character device, the same
// test the tui package's own gates use (anim.go's isCharDevice). A pipe or a
// regular file is not, which is how a redirected or piped run is excluded
// without anyone setting a variable.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// outputWriter is the writer the program should draw through. When the gate
// allows it, the stream is wrapped so every frame is bracketed; otherwise the
// stream itself is handed back, so a gated run and a piped run write exactly
// the bytes they wrote before this writer existed.
func outputWriter(stdout *os.File) io.Writer {
	if !syncGate(stdout) {
		return stdout
	}
	return &syncWriter{File: stdout}
}
