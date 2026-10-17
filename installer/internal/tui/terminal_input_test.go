package tui

// The guards for the terminal's single reader. The interface reads the terminal
// through one reader, and while the capability probe has a query outstanding
// that reader takes the terminal's answer out of the stream -- because a reply
// that reaches the interface instead arrives as keystrokes, and a reply the
// interface eats is a reply the probe never sees
// (odd/tasks/terminal-probe-ownership.md).
//
// Two rules these guards hold, in this order of importance:
//
//   - Outside the probe's window the input is untouched, byte for byte. The
//     reader is on the path every keystroke takes, so anything it does when the
//     probe is not asking is a bug in the interface.
//   - Inside the window, only an unambiguous answer leaves the stream. A person
//     at a keyboard cannot produce "\x1b[?2026;" or "\x1bP1+r" with one
//     keystroke, which is what makes the filter safe.

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// stubAnswer is what scripts/measure-terminal-ownership-stub.py answers with: the
// shape a terminal that supports all three queries sends, a DECRQM reply for
// mode 2026 followed by two XTGETTCAP replies.
const stubAnswer = "\x1b[?2026;2$y\x1bP1+r524742=31\x1b\\\x1bP1+r4D73=31\x1b\\"

// terminalFile is the shape bubbletea requires before it will put the terminal
// into raw mode (tea's initInput: a ReadWriteCloser whose descriptor is a
// terminal). Asserted here rather than with the term package's own type, so the
// guard does not add a dependency.
type terminalFile interface {
	io.ReadWriteCloser
	Fd() uintptr
}

var _ terminalFile = (*replyDivertingReader)(nil)

func TestAnAnswerIsTakenOutOfTheInputStream(t *testing.T) {
	filter := &answerFilter{}

	forward, answer := filter.Sift([]byte(stubAnswer))

	if len(forward) != 0 {
		t.Errorf("the interface received %q, want nothing: the whole read was the terminal's answer", forward)
	}
	if string(answer) != stubAnswer {
		t.Errorf("the probe received %q, want the answer whole: %q", answer, stubAnswer)
	}
}

func TestTypingIsForwardedUntouched(t *testing.T) {
	filter := &answerFilter{}
	typed := []byte("q\x1b[A3a")

	forward, answer := filter.Sift(typed)

	if string(forward) != string(typed) {
		t.Errorf("the interface received %q, want the typing untouched: %q", forward, typed)
	}
	if len(answer) != 0 {
		t.Errorf("the probe received %q, want nothing: no query was outstanding", answer)
	}
}

func TestTypingBeforeAnAnswerStillReachesTheInterface(t *testing.T) {
	filter := &answerFilter{}

	forward, answer := filter.Sift([]byte("ab" + stubAnswer))

	if string(forward) != "ab" {
		t.Errorf("the interface received %q, want the two keystrokes before the answer: %q", forward, "ab")
	}
	if string(answer) != stubAnswer {
		t.Errorf("the probe received %q, want the answer: %q", answer, stubAnswer)
	}
}

func TestAnAnswerSplitAfterItsHeaderIsDivertedWhole(t *testing.T) {
	filter := &answerFilter{}
	// Split inside the first XTGETTCAP value, after its header: the first read
	// cannot know how much is left, and the second must continue the answer
	// rather than feed it to the interface.
	head, tail := stubAnswer[:20], stubAnswer[20:]

	forwardHead, answerHead := filter.Sift([]byte(head))
	forwardTail, answerTail := filter.Sift([]byte(tail))

	if string(forwardHead)+string(forwardTail) != "" {
		t.Errorf("the interface received %q%q, want nothing: the answer spans both reads",
			forwardHead, forwardTail)
	}
	if string(answerHead)+string(answerTail) != stubAnswer {
		t.Errorf("the probe received %q%q, want the answer whole: %q",
			answerHead, answerTail, stubAnswer)
	}
}

func TestATerminatorSplitAcrossReadsEndsTheAnswer(t *testing.T) {
	filter := &answerFilter{}
	// The string terminator itself split: the answer ends on the next read, and
	// what follows it belongs to the interface again.
	head, tail := stubAnswer[:len(stubAnswer)-1], stubAnswer[len(stubAnswer)-1:]+"q"

	forwardHead, answerHead := filter.Sift([]byte(head))
	forwardTail, answerTail := filter.Sift([]byte(tail))

	if string(forwardHead)+string(forwardTail) != "q" {
		t.Errorf("the interface received %q%q, want only the keystroke after the answer",
			forwardHead, forwardTail)
	}
	if string(answerHead)+string(answerTail) != stubAnswer {
		t.Errorf("the probe received %q%q, want the answer whole: %q",
			answerHead, answerTail, stubAnswer)
	}
}

// TestASplitInsideAnAnswerHeaderLetsTheFragmentThrough records this filter's
// limit on purpose. A read boundary that falls inside a header -- "\x1b[?2026;"
// or "\x1bP1+r" -- leaves the fragment looking like ordinary input, so it is
// forwarded, and the answer it belonged to is lost. The consequence is bounded
// and one-sided: the answers whose headers stayed whole still reach the probe,
// and the forwarded fragment is a handful of bytes that no keystroke produces.
// If a later change makes a cut header wait for the rest of itself, this guard
// is the one that should fail, and it should then be rewritten rather than
// deleted.
func TestASplitInsideAnAnswerHeaderLetsTheFragmentThrough(t *testing.T) {
	filter := &answerFilter{}
	head, tail := stubAnswer[:14], stubAnswer[14:]

	forwardHead, answerHead := filter.Sift([]byte(head))
	forwardTail, answerTail := filter.Sift([]byte(tail))

	forward := string(forwardHead) + string(forwardTail)
	if !strings.Contains(forward, "\x1bP1") {
		t.Errorf("the interface received %q, want the cut header fragment forwarded: it cannot be told from input", forward)
	}
	answer := string(answerHead) + string(answerTail)
	if !strings.Contains(answer, "?2026;") || !strings.Contains(answer, "1+r4D73") {
		t.Errorf("the probe received %q, want the answers whose headers stayed whole", answer)
	}
}

func TestTheInterfaceLosesNothingWhenTheProbeIsNotAsking(t *testing.T) {
	reader := newReplyDivertingReader(strings.NewReader(stubAnswer))
	buf := make([]byte, 128)

	n, err := reader.Read(buf)

	if err != nil && err != io.EOF {
		t.Fatalf("reading the terminal: %v", err)
	}
	if string(buf[:n]) != stubAnswer {
		t.Errorf("the interface received %q, want every byte the probe is not asking for: %q", buf[:n], stubAnswer)
	}
}

func TestTheWindowIsOpenBeforeTheQueryIsWritten(t *testing.T) {
	// The answer can arrive while the query is still being written, so the
	// window has to be open first. A reader that opened it afterwards would drop
	// the fastest terminals' answers.
	reader := newReplyDivertingReader(strings.NewReader(stubAnswer))

	answer := reader.ask(func() error {
		buf := make([]byte, 128)
		_, _ = reader.Read(buf)
		return nil
	}, 100*time.Millisecond)

	if !strings.Contains(answer, "?2026;") || !strings.Contains(answer, "1+r524742") {
		t.Errorf("the probe received %q, want the answer that arrived during the write", answer)
	}
}

// silentModel is a program that reads the terminal and says nothing: the wiring
// guard below needs one that is reading, not one that reacts.
type silentModel struct{ received int }

func (m *silentModel) Init() tea.Cmd { return nil }

func (m *silentModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	m.received++
	return m, nil
}

func (m *silentModel) View() string { return "" }

// TestTheInterfaceReadsThroughTheReaderAndTheProbeGetsTheAnswer is the wiring
// guard: the program is given this reader, the terminal's answer arrives on the
// input the program is reading, and the probe receives it while the interface
// receives nothing. It runs on a pipe rather than a terminal -- enough for the
// question, which is who reads the answer, and it needs no pty. It also fails
// loudly when the program cannot start at all, which is the failure mode that
// otherwise looks exactly like an answer nobody could read.
func TestTheInterfaceReadsThroughTheReaderAndTheProbeGetsTheAnswer(t *testing.T) {
	terminal, feed, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer terminal.Close()
	defer feed.Close()

	reader := newReplyDivertingReader(terminal)
	model := &silentModel{}
	program := tea.NewProgram(model, tea.WithInput(reader), tea.WithOutput(io.Discard))
	runErr := make(chan error, 1)
	go func() {
		_, err := program.Run()
		runErr <- err
	}()
	// Let the program attach its input reader before the answer arrives.
	time.Sleep(200 * time.Millisecond)

	answer := reader.ask(func() error {
		_, err := feed.WriteString(stubAnswer)
		return err
	}, 300*time.Millisecond)

	program.Quit()
	if err := <-runErr; err != nil {
		t.Fatalf("the program did not run, so nothing was reading the terminal: %v", err)
	}

	if !strings.Contains(answer, "?2026;") || !strings.Contains(answer, "1+r524742") {
		t.Errorf("the probe received %q, want the answer the terminal sent", answer)
	}
	if model.received != 0 {
		t.Errorf("the interface received %d messages, want none: the answer belongs to the probe", model.received)
	}
}

// errorSource is a terminal that fails, which is the one thing the interface
// must always receive: a reader that swallowed an error would leave the program
// waiting for input that is never coming.
type errorSource struct {
	data []byte
	err  error
}

func (s *errorSource) Read(p []byte) (int, error) {
	if s.data == nil {
		return 0, s.err
	}
	n := copy(p, s.data)
	s.data = nil
	return n, s.err
}

func TestAnErrorFromTheTerminalIsHandedOn(t *testing.T) {
	wanted := errors.New("the terminal is gone")
	reader := newReplyDivertingReader(&errorSource{err: wanted})

	_, err := reader.Read(make([]byte, 32))

	if !errors.Is(err, wanted) {
		t.Errorf("the interface received %v, want the terminal's own error %v", err, wanted)
	}
}

func TestAnErrorThatArrivesWithTheAnswerIsStillHandedOn(t *testing.T) {
	wanted := io.ErrUnexpectedEOF
	reader := newReplyDivertingReader(&errorSource{data: []byte(stubAnswer), err: wanted})

	var handedToInterface int
	var interfaceErr error
	answer := reader.ask(func() error {
		handedToInterface, interfaceErr = reader.Read(make([]byte, 128))
		return nil
	}, 50*time.Millisecond)

	if string(answer) != stubAnswer {
		t.Errorf("the probe received %q, want the whole answer: %q", answer, stubAnswer)
	}
	if handedToInterface != 0 {
		t.Errorf("the interface received %d bytes of the answer, want none", handedToInterface)
	}
	if !errors.Is(interfaceErr, wanted) {
		t.Errorf("the interface received %v, want the terminal's error %v", interfaceErr, wanted)
	}
}

func TestTheAnswerIsDivertedWholeWhenItArrivesInPieces(t *testing.T) {
	filter := &answerFilter{}
	// Four reads, none of them a whole answer. Every boundary falls inside an
	// XTGETTCAP answer's body and never inside a header, so what is under test is
	// the continuation. The boundaries are derived from the three answers rather
	// than written as offsets: an off-by-one here cuts the escape that introduces
	// the next header, and the reader then forwards it -- correctly, because it
	// never holds a byte back -- and the test measures the limit instead of the
	// continuation.
	mode := len("\x1b[?2026;2$y")
	rgb := len("\x1bP1+r524742=31\x1b\\")
	pieces := []string{
		stubAnswer[:mode],             // the mode answer, whole
		stubAnswer[mode : mode+6],     // the first XTGETTCAP header and one byte of its body
		stubAnswer[mode+6 : mode+rgb], // the rest of the first answer
		stubAnswer[mode+rgb:],         // the second answer, whole
	}

	var forward, answer []byte
	for _, piece := range pieces {
		kept, taken := filter.Sift([]byte(piece))
		forward = append(forward, kept...)
		answer = append(answer, taken...)
	}

	if len(forward) != 0 {
		t.Errorf("the interface received %q, want nothing: every piece belonged to the answer", forward)
	}
	if string(answer) != stubAnswer {
		t.Errorf("the probe received %q, want the answer whole: %q", answer, stubAnswer)
	}
}

func TestTwoAnswersInOneReadAreBothDiverted(t *testing.T) {
	filter := &answerFilter{}
	twice := stubAnswer + stubAnswer

	forward, answer := filter.Sift([]byte(twice))

	if len(forward) != 0 {
		t.Errorf("the interface received %q, want nothing: both reads were answers", forward)
	}
	if string(answer) != twice {
		t.Errorf("the probe received %q, want both answers: %q", answer, twice)
	}
}

func TestTypingOnBothSidesOfAnAnswerIsForwarded(t *testing.T) {
	filter := &answerFilter{}

	forward, answer := filter.Sift([]byte("ab" + stubAnswer + "cd"))

	if string(forward) != "abcd" {
		t.Errorf("the interface received %q, want every keystroke around the answer: %q", forward, "abcd")
	}
	if string(answer) != stubAnswer {
		t.Errorf("the probe received %q, want only the answer: %q", answer, stubAnswer)
	}
}

func TestAWindowLeavesNoStateBehind(t *testing.T) {
	reader := newReplyDivertingReader(strings.NewReader(stubAnswer))

	if answer := reader.ask(func() error { return nil }, 20*time.Millisecond); answer != "" {
		t.Fatalf("the probe received %q from a terminal that answered nothing, want nothing", answer)
	}

	buf := make([]byte, 128)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("reading the terminal: %v", err)
	}
	if string(buf[:n]) != stubAnswer {
		t.Errorf("the interface received %q, want every byte: a closed window must not filter anything", buf[:n])
	}
}

func TestTheProbeReturnsByItsDeadline(t *testing.T) {
	reader := newReplyDivertingReader(strings.NewReader(""))
	start := time.Now()

	answer := reader.ask(func() error { return nil }, 50*time.Millisecond)

	if answer != "" {
		t.Errorf("the probe received %q, want nothing: the terminal never answered", answer)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("the probe took %s, want to return by its deadline", elapsed)
	}
}
