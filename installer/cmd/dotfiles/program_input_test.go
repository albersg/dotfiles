package main

import (
	"os"
	"testing"

	"github.com/charmbracelet/x/term"
)

// TestAPipedStdinIsLeftToBubbletea is the regression guard for the rule that
// matters most here: a program given an input option reads that option, and
// bubbletea only opens the controlling terminal itself when it is not given one.
// So an input that is not the terminal must be left alone -- otherwise a piped
// run reaches a program that is reading a pipe no key can arrive on.
func TestAPipedStdinIsLeftToBubbletea(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer read.Close()
	defer write.Close()

	if term.IsTerminal(read.Fd()) {
		t.Fatal("a pipe reports as a terminal here, so this guard cannot make its point")
	}
	if input := programInput(read); input != nil {
		t.Errorf("programInput(pipe) = %T, want nil: bubbletea must keep its own rule for an input that is not the terminal", input)
	}
}

// TestACharacterDeviceThatIsNotATerminalIsLeftToBubbletea holds the case the
// repository's own isTerminal would have got wrong: /dev/null is a character
// device and is not a terminal, and a run started with it as its input must keep
// the terminal bubbletea would have opened.
func TestACharacterDeviceThatIsNotATerminalIsLeftToBubbletea(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		t.Skipf("no %s on this host: %v", os.DevNull, err)
	}
	defer devNull.Close()

	if term.IsTerminal(devNull.Fd()) {
		t.Fatalf("%s reports as a terminal here, so this guard cannot make its point", os.DevNull)
	}
	if input := programInput(devNull); input != nil {
		t.Errorf("programInput(%s) = %T, want nil: it is a character device, and it is not the terminal", os.DevNull, input)
	}
}

// TestTheTerminalIsReadThroughTheOneReader is the other half: when stdin IS the
// terminal, the interface is handed the reader the capability probe takes its
// answer from.
//
// The guard is observable only where the tests have a terminal on stdin, which
// `go test` does not give them: run the compiled test under `script(1)` --
// `go test -c -o /tmp/cmd.test ./cmd/dotfiles` and then
// `script -qec "/tmp/cmd.test -test.run TestTheTerminalIsReadThroughTheOneReader -test.v"`.
// When it skips it prints what stdin is, so a skip that happens for the wrong
// reason is visible instead of silent -- which is exactly how the flaw this guard
// now covers was found.
func TestTheTerminalIsReadThroughTheOneReader(t *testing.T) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		info, err := os.Stdin.Stat()
		if err != nil {
			t.Skipf("stdin cannot be inspected here: %v", err)
		}
		t.Skipf("the tests are run with stdin away from a terminal (%v), so the positive half cannot be observed here; run the compiled test under script(1) to observe it", info.Sys())
	}

	if input := programInput(os.Stdin); input == nil {
		t.Error("programInput(os.Stdin) = nil with a terminal on stdin, want the reader the probe diverts its answer from")
	}
}
