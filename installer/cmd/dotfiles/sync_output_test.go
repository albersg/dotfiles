package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// TestSyncWriterFramesEveryWrite pins the whole framing contract: one begin and
// one end around each payload, in order, with the payload's length reported and
// nothing at all written for a zero-length write. The last point is what keeps a
// renderer that flushes nothing from toggling the terminal's mode around an empty
// frame.
func TestSyncWriterFramesEveryWrite(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "sync-writer")
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer file.Close()

	w := &syncWriter{File: file}

	first := []byte("menu row one")
	n, err := w.Write(first)
	if err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if n != len(first) {
		t.Errorf("first Write returned %d, want %d (the payload's length, not the frame's)", n, len(first))
	}

	second := []byte("menu row two")
	if _, err := w.Write(second); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	if n, err := w.Write(nil); err != nil {
		t.Errorf("zero-length Write: %v", err)
	} else if n != 0 {
		t.Errorf("zero-length Write returned %d, want 0", n)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	want := syncBegin + string(first) + syncEnd + syncBegin + string(second) + syncEnd
	if string(got) != want {
		t.Errorf("framed output =\n%q\nwant\n%q", got, want)
	}
}

// TestSyncWriterReportsAFailedFrame makes sure the two sequences do not turn an
// error into a silent success: a stream that refuses the begin sequence returns
// that error and reports no bytes written.
func TestSyncWriterReportsAFailedFrame(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "sync-writer-fail")
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	w := &syncWriter{File: file}
	n, err := w.Write([]byte("frame"))
	if err == nil {
		t.Error("a write to a closed stream did not report an error")
	}
	if n != 0 {
		t.Errorf("failed Write returned %d, want 0", n)
	}
}

// TestSyncWriterKeepsTheTerminalsFileDescriptor pins the reason the wrapper
// embeds the file instead of holding an io.Writer: bubbletea type-asserts the
// program's output to term.File and reads the window size from the resulting
// Fd(). If the wrapper hid the descriptor the program would stop receiving
// WindowSizeMsg (and, on Windows, stop enabling virtual terminal processing), so
// the descriptor and the stream's Read and Close have to keep reaching the
// terminal underneath.
func TestSyncWriterKeepsTheTerminalsFileDescriptor(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "sync-fd")
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer file.Close()

	w := &syncWriter{File: file}
	if got, want := w.Fd(), file.Fd(); got != want {
		t.Errorf("Fd() = %d, want the stream's %d", got, want)
	}

	if _, err := w.Write([]byte("frame")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	buf := make([]byte, 5)
	if n, err := w.Read(buf); err != nil || n != len(buf) {
		t.Errorf("Read through the wrapper = %d, %v; want %d, nil", n, err, len(buf))
	}

	if err := w.Close(); err != nil {
		t.Errorf("Close through the wrapper: %v", err)
	}
	if _, err := file.Write([]byte("after close")); err == nil {
		t.Error("Close through the wrapper did not reach the stream")
	}
}

// TestSyncGateTurnsOffOnTheEnvironmentAndTheStream pins the gate: a non-terminal
// stdout turns the mode off by itself (a redirected run would otherwise stream
// DECSET 2026 into a file), DOTFILES_SYNC=0 turns it off, and a character device
// with no override is the only combination that leaves it on. /dev/null stands in
// for a terminal because it is the one character device a test can rely on
// without opening the user's tty.
func TestSyncGateTurnsOffOnTheEnvironmentAndTheStream(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "sync-gate")
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer file.Close()

	if syncGate(file) {
		t.Error("a regular file was treated as a terminal: a redirected run would stream synchronized-output escapes into a file")
	}
	if syncGate(nil) {
		t.Error("a nil stream was treated as a terminal")
	}

	term := charDevice(t)

	t.Setenv(envSync, "")
	if !syncGate(term) {
		t.Error("a character device with no override did not bracket frames")
	}

	t.Setenv(envSync, "0")
	if syncGate(term) {
		t.Error("DOTFILES_SYNC=0 did not turn synchronized output off")
	}
}

// TestOutputWriterLeavesANonTerminalStreamUntouched is the case that protects a
// pipe: with no terminal behind the bytes, outputWriter hands the stream back
// itself, and what is written through it is byte-for-byte the payload. A run that
// piped the installer somewhere must never see the two sequences.
func TestOutputWriterLeavesANonTerminalStreamUntouched(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer r.Close()

	out := outputWriter(w)
	if out != io.Writer(w) {
		t.Fatalf("a pipe was wrapped: outputWriter returned %T, want the stream itself", out)
	}

	payload := []byte("a frame with no terminal behind it\n")
	if _, err := out.Write(payload); err != nil {
		t.Fatalf("write through the pipe writer: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close write end: %v", err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("piped output changed: got %q, want %q", got, payload)
	}
}

// TestOutputWriterWrapsATerminalAndTheGateUnwrapsIt pins the other side: a
// terminal gets the framing writer, and DOTFILES_SYNC=0 is enough to get the
// stream back, which is the escape hatch a multiplexer that mishandles the mode
// needs.
func TestOutputWriterWrapsATerminalAndTheGateUnwrapsIt(t *testing.T) {
	term := charDevice(t)

	t.Setenv(envSync, "")
	out := outputWriter(term)
	sw, ok := out.(*syncWriter)
	if !ok {
		t.Fatalf("a terminal was not wrapped: outputWriter returned %T", out)
	}
	if sw.File != term {
		t.Errorf("the wrapper writes to %p, want the terminal %p", sw.File, term)
	}

	t.Setenv(envSync, "0")
	if out := outputWriter(term); out != io.Writer(term) {
		t.Fatalf("DOTFILES_SYNC=0 still wrapped the stream: got %T", out)
	}
}

// charDevice returns a character device to stand in for a terminal, skipping the
// test on hosts where /dev/null is not one.
func charDevice(t *testing.T) *os.File {
	t.Helper()

	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("no %s to stand in for a terminal: %v", os.DevNull, err)
	}
	t.Cleanup(func() { f.Close() })

	if !isTerminal(f) {
		t.Skipf("%s is not a character device on this host, so it cannot stand in for a terminal", os.DevNull)
	}
	return f
}
