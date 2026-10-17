//go:build linux

package tui

// This file is the diagnostic of odd/tasks/terminal-probe-ownership.md (T2), not a
// guard. It answers a question no guard can: when the interface is reading stdin
// and the capability probe opens /dev/tty on the same terminal, who gets the
// terminal's reply? It reports what each reader received instead of asserting
// what it should.
//
// It is gated on DOTFILES_PROBE_MEASURE, so an ordinary `go test` runs nothing,
// and it needs a controlling terminal and a terminal on the far side that
// answers the query: scripts/measure-terminal-ownership.sh supplies both.

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMeasureWhoReadsTheTerminal runs one measurement per invocation. Which one
// is DOTFILES_PROBE_MEASURE: "alone" for the probe on a terminal nobody else is
// reading, "shared" for the probe with the interface reading the same stdin the
// installer gives it in production.
func TestMeasureWhoReadsTheTerminal(t *testing.T) {
	switch mode := os.Getenv("DOTFILES_PROBE_MEASURE"); mode {
	case "alone":
		measureProbeAlone(t)
	case "shared":
		measureProbeShared(t)
	case "diverted":
		measureProbeDiverted(t)
	default:
		t.Skip("set DOTFILES_PROBE_MEASURE=alone, shared or diverted, or run scripts/measure-terminal-ownership.sh")
	}
}

// measureProbeAlone is the baseline: the terminal answers the query, and the
// probe is the only reader. If this does not report bytes, the harness is wrong
// or the terminal does not answer, and the shared case says nothing.
func measureProbeAlone(t *testing.T) {
	start := time.Now()
	reply := terminalReply()
	t.Logf("case A: probe alone received %d bytes after %s: %q",
		len(reply), time.Since(start).Round(time.Millisecond), reply)
	t.Logf("case A: interpretation: %s", describeProbe(reply))
}

// measureProbeShared is the arrangement the installer had: a bubbletea program
// reading os.Stdin with no input option, and the probe running while that reader
// is attached. It is kept because it is the measurement of the defect itself.
func measureProbeShared(t *testing.T) {
	measureSharedWith(t)
}

// measureProbeDiverted is the arrangement the installer has now: the interface
// reads the terminal through TerminalInput, the one reader the probe takes its
// answer from. The same reading, so the two arrangements compare line by line.
func measureProbeDiverted(t *testing.T) {
	measureSharedWith(t, tea.WithInput(TerminalInput(os.Stdin)))
}

func measureSharedWith(t *testing.T, options ...tea.ProgramOption) {
	recorded := &recordingModel{}
	program := tea.NewProgram(recorded, options...)
	// The program's own error is reported rather than discarded: a program that
	// could not start reads nothing, and the reading below would then look like an
	// answer nobody could read rather than like a program that never ran.
	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		_, runErr = program.Run()
	}()

	// The interface's input reader attaches during Run; the probe in production
	// runs long after that, when a human opens the capability screen.
	time.Sleep(300 * time.Millisecond)
	before := recorded.count()

	start := time.Now()
	reply := terminalReply()
	elapsed := time.Since(start)

	// Whatever was going to arrive has arrived: the probe's own bound is 250ms.
	time.Sleep(250 * time.Millisecond)
	during := recorded.count() - before

	program.Quit()
	<-done

	// Which stream the program's stdin actually is decides what this reading
	// means: bubbletea reads stdin only when it is the terminal, and opens the
	// controlling terminal itself when it is not. Naming it here is what keeps a
	// reading of "nothing arrived" from being read as a defect in the reader.
	stdinTarget, readlinkErr := os.Readlink("/proc/self/fd/0")
	if readlinkErr != nil {
		stdinTarget = readlinkErr.Error()
	}
	t.Logf("case B: the program's stdin is %s", stdinTarget)

	t.Logf("case B: the program's own error: %v", runErr)
	t.Logf("case B: probe with the interface reading stdin received %d bytes after %s: %q",
		len(reply), elapsed.Round(time.Millisecond), reply)
	t.Logf("case B: interpretation: %s", describeProbe(reply))
	t.Logf("case C: the interface received %d messages while the query was outstanding", during)
	for _, seen := range recorded.messages() {
		t.Logf("case C: the interface received %s", seen)
	}
}

// describeProbe runs the interpretation half over what the probe read, so the
// measurement also says whether the bytes it received are the bytes it can use.
func describeProbe(reply string) string {
	caps := detectTerminalCapabilities(os.Getenv, reply)
	state := func(a terminalAnswer) string {
		return fmt.Sprintf("%s (%s)", a.State, a.Source)
	}
	return fmt.Sprintf("colour=%s clipboard=%s sync=%s nerd-font=%s",
		state(caps.Color), state(caps.Clipboard), state(caps.Sync), state(caps.NerdFont))
}

// recordingModel keeps every message bubbletea delivers, which is where a
// terminal reply consumed as input would land. It also counts them, because a
// reply the parser swallows silently still counts as consumed.
type recordingModel struct {
	mu   sync.Mutex
	seen []string
}

func (m *recordingModel) Init() tea.Cmd { return nil }

func (m *recordingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen = append(m.seen, describeMessage(msg))
	return m, nil
}

func (m *recordingModel) View() string { return "" }

func (m *recordingModel) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.seen)
}

func (m *recordingModel) messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.seen))
	copy(out, m.seen)
	return out
}

// describeMessage names a message the way the measurement needs to read it: a
// key message keeps its runes, so a reply that arrived as input is visible as
// the escape bytes it was.
func describeMessage(msg tea.Msg) string {
	if key, ok := msg.(tea.KeyMsg); ok {
		return fmt.Sprintf("KeyMsg type=%v runes=%q alt=%v paste=%v",
			key.Type, key.Runes, key.Alt, key.Paste)
	}
	return fmt.Sprintf("%T", msg)
}
