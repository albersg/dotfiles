//go:build linux

package tui

// This file is the diagnostic of odd/tasks/terminal-probe-ownership.md, not a
// guard. It answers a question no guard can: who reads the terminal when the
// interface and the capability probe both want it? It reports what each reader
// received instead of asserting what it should.
//
// Two arrangements, one per invocation (DOTFILES_PROBE_MEASURE):
//
//	diverted  the interface reading through TerminalInput, which is what
//	          cmd/dotfiles starts: one reader, and the answer leaves it
//	notowner  no reader of ours, so the interface reads the terminal for itself
//	          because stdin is not it -- the arrangement T5 is about, where the
//	          probe must write no query at all rather than race that reader
//
// Two readings this file once made are retired with the code that could produce
// their arrangement. `shared` -- the interface on the terminal's input with the
// probe opening it for itself -- is the defect's own arrangement, and no query is
// written now without the reader. `alone` -- the probe asking with nobody else
// reading -- cannot exist either: with no reader loop running, the query is written
// and answered and nobody reads the answer, which is what its last round showed
// (`case A: probe alone received 0 bytes` against the stub's own `query seen ...
// answering with 41 bytes`).
//
// It is gated on DOTFILES_PROBE_MEASURE, so an ordinary `go test` runs nothing,
// and it needs a controlling terminal and a terminal on the far side that answers
// the query: scripts/measure-terminal-ownership.sh supplies both.

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMeasureWhoReadsTheTerminal runs one measurement per invocation.
func TestMeasureWhoReadsTheTerminal(t *testing.T) {
	switch mode := os.Getenv("DOTFILES_PROBE_MEASURE"); mode {
	case "diverted":
		measureProbeDiverted(t)
	case "notowner":
		measureProbeNotOwner(t)
	default:
		t.Skip("set DOTFILES_PROBE_MEASURE=diverted or notowner, or run scripts/measure-terminal-ownership.sh")
	}
}

// probeReading is one run's reading: what came back, what the report made of it,
// and whether a query was written at all. `asked` is the field T5 turns on: in the
// arrangement where the interface reads the terminal itself, the honest reading is
// "nothing was asked", not "nothing arrived".
type probeReading struct {
	reply   string
	report  terminalCapabilities
	asked   bool
	elapsed time.Duration
}

// measureProbeDiverted is the arrangement the installer has now: the interface
// reads the terminal through TerminalInput, the one reader the probe takes its
// answer from.
func measureProbeDiverted(t *testing.T) {
	TerminalInput(os.Stdin)
	reader := activeTerminalInput()

	measureWithProgram(t, []tea.ProgramOption{tea.WithInput(reader)}, func() probeReading { return probe(reader) })
}

// measureProbeNotOwner is the arrangement the installer has when its input is not
// the terminal: no reader of ours is installed and bubbletea reads the controlling
// terminal for itself. The probe must write no query here -- a query would put a
// second reader on that terminal and race bubbletea for the answer, which is what
// the front measured before the fix. The runner redirects stdin away from the pty
// for this case, and the stub on the far side is the witness that nothing was
// written: it logs every query it sees.
func measureProbeNotOwner(t *testing.T) {
	terminalInputMu.Lock()
	terminalInput = nil
	terminalInputMu.Unlock()

	measureWithProgram(t, nil, func() probeReading {
		msg := loadTerminalCapabilityCmd()()
		capabilityMsg, ok := msg.(terminalCapabilityMsg)
		if !ok {
			t.Fatalf("the probe produced %T, want a terminalCapabilityMsg", msg)
		}
		return probeReading{report: capabilityMsg.caps, asked: false}
	})
}

// probe asks the terminal the way production does -- through the reader that reads
// the interface -- and returns what came back with what the report made of it.
func probe(reader *replyDivertingReader) probeReading {
	start := time.Now()
	reply := askTerminalThroughTheReader(reader)
	elapsed := time.Since(start)
	return probeReading{
		reply:   reply,
		report:  detectTerminalCapabilities(os.Getenv, reply),
		asked:   true,
		elapsed: elapsed,
	}
}

// measureWithProgram starts bubbletea, runs the probe while it is reading, and
// reports what each side received. The arrangements that have a program running
// share it, so their readings compare line by line.
func measureWithProgram(t *testing.T, options []tea.ProgramOption, measure func() probeReading) {
	t.Helper()
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

	reading := measure()

	// Whatever was going to arrive has arrived: the probe's own bound is 250ms.
	time.Sleep(250 * time.Millisecond)
	during := recorded.count() - before

	program.Quit()
	<-done

	// Which stream the program's stdin actually is decides what this reading means:
	// bubbletea reads stdin only when it is the terminal, and opens the controlling
	// terminal itself when it is not. Naming it here is what keeps a reading of
	// "nothing arrived" from being read as a defect in the reader.
	stdinTarget, readlinkErr := os.Readlink("/proc/self/fd/0")
	if readlinkErr != nil {
		stdinTarget = readlinkErr.Error()
	}
	t.Logf("case B: the program's stdin is %s", stdinTarget)
	t.Logf("case B: the program's own error: %v", runErr)

	if reading.asked {
		t.Logf("case B: the probe wrote a query and received %d bytes after %s: %q",
			len(reading.reply), reading.elapsed.Round(time.Millisecond), reading.reply)
		t.Logf("case B: interpretation: %s", describeReport(reading.report))
	} else {
		t.Logf("case E: the probe wrote no query; the report says: colour=%s clipboard=%q sync=%q",
			reading.report.Color.Source, reading.report.Clipboard.Reason, reading.report.Sync.Reason)
	}

	t.Logf("case C: the interface received %d messages while the probe ran", during)
	for _, seen := range recorded.messages() {
		t.Logf("case C: the interface received %s", seen)
	}
}

// describeReport names each answer with its state and the source it came from, so
// the measurement also says whether what arrived is what the report can use.
func describeReport(caps terminalCapabilities) string {
	state := func(a terminalAnswer) string {
		return fmt.Sprintf("%s (%s)", a.State, a.Source)
	}
	return fmt.Sprintf("colour=%s clipboard=%s sync=%s nerd-font=%s",
		state(caps.Color), state(caps.Clipboard), state(caps.Sync), state(caps.NerdFont))
}

// recordingModel keeps every message bubbletea delivers, which is where a terminal
// reply consumed as input would land. It also counts them, because a reply the
// parser swallows silently still counts as consumed.
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

// describeMessage names a message the way the measurement needs to read it: a key
// message keeps its runes, so a reply that arrived as input is visible as the
// escape bytes it was.
func describeMessage(msg tea.Msg) string {
	if key, ok := msg.(tea.KeyMsg); ok {
		return fmt.Sprintf("KeyMsg type=%v runes=%q alt=%v paste=%v",
			key.Type, key.Runes, key.Alt, key.Paste)
	}
	return fmt.Sprintf("%T", msg)
}
