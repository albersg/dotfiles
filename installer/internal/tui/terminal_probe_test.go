package tui

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The terminal capability probe
// ---------------------------------------------------------------------------
//
// The probe is the one part of the utility that talks to the terminal, so it is
// the one part that can hang the interface. These guards pin the three things
// that make it honest: the read is bounded, an answer the probe did not
// determine is reported as unknown (never as an invented "no"), and every
// answer names where it came from.

// emptyTerminalEnv is the environment a probe sees when nothing describes the
// terminal. Real runs export at least TERM, but the empty map is what proves the
// unknown branch is reachable rather than theoretical.
func emptyTerminalEnv(string) string { return "" }

// envFrom builds a getenv function from a map, so a guard can pin exactly the
// variables the probe is allowed to read.
func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// blockingTerminal is a reader that never answers, the terminal the guard uses
// to prove the probe does not hang. It returns an error only once the test
// closes it, so the abandoned read cannot outlive the guard either.
type blockingTerminal struct {
	done chan struct{}
	once sync.Once
}

func newBlockingTerminal() *blockingTerminal {
	return &blockingTerminal{done: make(chan struct{})}
}

func (b *blockingTerminal) Read([]byte) (int, error) {
	<-b.done
	return 0, errTerminalClosed
}

func (b *blockingTerminal) close() { b.once.Do(func() { close(b.done) }) }

// errTerminalClosed is the error a blockingTerminal returns once the guard is
// done with it.
var errTerminalClosed = &terminalClosedError{}

type terminalClosedError struct{}

func (*terminalClosedError) Error() string { return "terminal closed" }

// TestTheProbeDoesNotHangWhenTheTerminalNeverAnswers is the guard the whole
// utility exists under. A probe that waits for a reply it never receives leaves
// the interface dead, which is worse than not probing at all, so the probe returns
// an honest empty answer within its timeout instead of a frozen frame.
//
// It runs through the reader, which is the only way the probe asks now, and the
// source is the strictest case: a stream that never yields while the write
// succeeds, so nothing but the probe's own deadline can end the wait.
func TestTheProbeDoesNotHangWhenTheTerminalNeverAnswers(t *testing.T) {
	blocking := newBlockingTerminal()
	defer blocking.close()
	reader := newReplyDivertingReader(blocking)

	done := make(chan string, 1)
	started := time.Now()
	go func() { done <- reader.ask(func() error { return nil }, 60*time.Millisecond) }()

	select {
	case got := <-done:
		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Fatalf("the probe took %s to give up, so a silent terminal would stall the interface", elapsed)
		}
		if got != "" {
			t.Errorf("a silent terminal produced an answer %q, want the honest empty reply", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never returned against a terminal that never answers: the interface would hang")
	}
}

// TestUnknownIsItsOwnStateNotAnInventedNo is the honesty guard, and it is the
// one a two-state probe cannot pass. With nothing to read -- no reply and no
// variable describing the terminal -- every capability is unknown, and each
// unknown answer says why and how a reader can check by hand. A "no" the probe
// invented is a lie about the terminal, so the unknown state cannot be folded
// into "unsupported".
func TestUnknownIsItsOwnStateNotAnInventedNo(t *testing.T) {
	caps := detectTerminalCapabilities(emptyTerminalEnv, "")

	if !caps.Resolved {
		t.Fatal("the probe did not mark the report resolved")
	}
	for _, row := range caps.rows() {
		if row.Answer.State != capabilityUnknown {
			t.Errorf("%s = %v with nothing to read, want unknown: the probe invented an answer",
				row.Name, row.Answer.State)
		}
		if row.Answer.Value != "unknown" {
			t.Errorf("%s shows %q, want \"unknown\"", row.Name, row.Answer.Value)
		}
		if row.Answer.Source == "" {
			t.Errorf("%s does not name where its answer came from", row.Name)
		}
		if row.Answer.Reason == "" {
			t.Errorf("%s is unknown without a reason, so the reader cannot tell why", row.Name)
		}
		if row.Answer.Manual == "" {
			t.Errorf("%s is unknown without a manual check, so the reader cannot settle it", row.Name)
		}
	}
}

// TestTheStatesAreThreeAndDistinct is the teeth behind the unknown state: the
// zero value has to be unknown, and unknown has to be different from both
// supported and unsupported. Removing the unknown case -- for instance by making
// the zero value mean "unsupported" -- breaks this test by name.
func TestTheStatesAreThreeAndDistinct(t *testing.T) {
	if capabilityUnknown == capabilitySupported || capabilityUnknown == capabilityUnsupported ||
		capabilitySupported == capabilityUnsupported {
		t.Fatal("the capability states are not three distinct values")
	}
	if got := (terminalAnswer{}).stateWord(); got != "unknown" {
		t.Errorf("the zero answer reads %q, want \"unknown\"", got)
	}
	if got := (terminalAnswer{State: capabilitySupported}).stateWord(); got != "supported" {
		t.Errorf("the supported answer reads %q", got)
	}
	if got := (terminalAnswer{State: capabilityUnsupported}).stateWord(); got != "not supported" {
		t.Errorf("the unsupported answer reads %q", got)
	}
}

// TestEveryAnswerNamesItsSource pins the auditability rule: an environment
// variable, the terminal's own reply, or nothing (unknown). A supported or
// unsupported answer must carry the exact variable it read or the reply it
// parsed, so a reader can re-derive it.
func TestEveryAnswerNamesItsSource(t *testing.T) {
	caps := detectTerminalCapabilities(envFrom(map[string]string{
		envColorTerm: "truecolor",
		envTerm:      "xterm-256color",
	}), "")

	if caps.Color.State != capabilitySupported || caps.Color.Value != colorValueTrueColor {
		t.Fatalf("COLORTERM=truecolor gave %v/%q, want supported truecolor", caps.Color.State, caps.Color.Value)
	}
	if !strings.Contains(caps.Color.Source, envColorTerm) {
		t.Errorf("the colour answer names %q as its source, want the variable %s", caps.Color.Source, envColorTerm)
	}

	replyCaps := detectTerminalCapabilities(emptyTerminalEnv, "\x1b[?2026;1$y")
	if replyCaps.Sync.State != capabilitySupported {
		t.Fatalf("the mode 2026 reply gave sync %v, want supported", replyCaps.Sync.State)
	}
	if !strings.Contains(strings.ToLower(replyCaps.Sync.Source), "reply") {
		t.Errorf("the sync answer names %q as its source, want the terminal's reply", replyCaps.Sync.Source)
	}
}

// TestTheColourDepthIsNamedNotJustYesOrNo pins the four depths the utility must
// report. The terminal reply, COLORTERM and TERM are three different sources for
// the same fact, and each level is named as the level it is.
func TestTheColourDepthIsNamedNotJustYesOrNo(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		reply string
		state capabilityState
		value string
	}{
		{"the terminal's own RGB reply", nil, "\x1bP1+r524742=1\x1b\\", capabilitySupported, colorValueTrueColor},
		{"COLORTERM=24bit", map[string]string{envColorTerm: "24bit"}, "", capabilitySupported, colorValueTrueColor},
		{"a known truecolor terminal", map[string]string{envTermProgram: "kitty"}, "", capabilitySupported, colorValueTrueColor},
		{"TERM=xterm-256color", map[string]string{envTerm: "xterm-256color"}, "", capabilitySupported, colorValue256},
		{"TERM=xterm", map[string]string{envTerm: "xterm"}, "", capabilitySupported, colorValue16},
		{"TERM=dumb", map[string]string{envTerm: "dumb"}, "", capabilityUnsupported, colorValueNone},
		{"nothing describes it", nil, "", capabilityUnknown, "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			caps := detectTerminalCapabilities(envFrom(tt.env), tt.reply)
			if caps.Color.State != tt.state || caps.Color.Value != tt.value {
				t.Errorf("colour = %v/%q, want %v/%q",
					caps.Color.State, caps.Color.Value, tt.state, tt.value)
			}
		})
	}
}

// TestTheSyncReplyIsReadAsTheModesOwnAnswer pins the DECRQM reply: a positive
// mode state means the terminal implements DECSET 2026, the "not recognized"
// state means it does not, and the absence of a reply is unknown rather than a
// no. The mode is what this repository already relies on to stop the frame
// tearing, so the screen has to report it honestly.
func TestTheSyncReplyIsReadAsTheModesOwnAnswer(t *testing.T) {
	tests := []struct {
		reply string
		want  capabilityState
	}{
		{"\x1b[?2026;1$y", capabilitySupported},   // set
		{"\x1b[?2026;2$y", capabilitySupported},   // reset
		{"\x1b[?2026;0$y", capabilityUnsupported}, // not recognized
		{"", capabilityUnknown},                   // no reply
		{"\x1b[?1;2c", capabilityUnknown},         // a reply to something else
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.reply, func(t *testing.T) {
			caps := detectTerminalCapabilities(emptyTerminalEnv, tt.reply)
			if caps.Sync.State != tt.want {
				t.Errorf("sync from %q = %v, want %v", tt.reply, caps.Sync.State, tt.want)
			}
		})
	}
}

// TestTheClipboardReplyIsReadFromTheTerminfoQuery pins the OSC 52 half: the
// terminal advertises the "Ms" capability (or refuses it) in its XTGETTCAP
// reply, and silence is unknown. OSC 52 is what makes copying work over SSH, so
// a quiet terminal must not be reported as a yes.
func TestTheClipboardReplyIsReadFromTheTerminfoQuery(t *testing.T) {
	tests := []struct {
		reply string
		want  capabilityState
	}{
		{"\x1bP1+r4D73\x1b\\", capabilitySupported},
		{"\x1bP0+r4D73\x1b\\", capabilityUnsupported},
		{"", capabilityUnknown},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.reply, func(t *testing.T) {
			caps := detectTerminalCapabilities(emptyTerminalEnv, tt.reply)
			if caps.Clipboard.State != tt.want {
				t.Errorf("clipboard from %q = %v, want %v", tt.reply, caps.Clipboard.State, tt.want)
			}
		})
	}
}

// TestTheNerdFontAnswerIsAlwaysUnknown pins the one capability with no query:
// whether a glyph renders as an icon is not something the terminal can be asked,
// so the probe says unknown with a manual check rather than guessing from a
// variable that does not describe it.
func TestTheNerdFontAnswerIsAlwaysUnknown(t *testing.T) {
	for _, env := range []map[string]string{
		nil,
		{envTerm: "xterm-256color", envTermProgram: "kitty", envColorTerm: "truecolor"},
	} {
		caps := detectTerminalCapabilities(envFrom(env), "\x1b[?2026;1$y\x1bP1+r524742=1\x1b\\")
		if caps.NerdFont.State != capabilityUnknown {
			t.Errorf("nerd font = %v, want unknown: there is no terminal query for it", caps.NerdFont.State)
		}
		if caps.NerdFont.Value != "unknown" || caps.NerdFont.Manual == "" {
			t.Errorf("nerd font = %q (manual %q), want unknown with a manual check", caps.NerdFont.Value, caps.NerdFont.Manual)
		}
	}
}

// TestTheQueryCarriesTheChecksTheProbeParses pins the payload so the two halves
// cannot drift: every reply the probe parses has to be asked for in the query.
func TestTheQueryCarriesTheChecksTheProbeParses(t *testing.T) {
	for _, want := range []string{
		"\x1b[?2026$p",        // DECRQM for the synchronized-output mode
		"\x1bP+q524742\x1b\\", // XTGETTCAP for the RGB terminfo capability
		"\x1bP+q4D73\x1b\\",   // XTGETTCAP for the Ms (clipboard) capability
	} {
		if !strings.Contains(terminalCapabilityQuery, want) {
			t.Errorf("the query does not ask %q, so the reply it parses can never arrive", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Who the probe is allowed to ask
// ---------------------------------------------------------------------------

// withoutTerminalInput puts the package back into the state of a program started
// without the one reader -- the run whose input is not the terminal, so bubbletea
// opens the controlling terminal by itself. It is the arrangement in which the
// probe must not ask anything (odd/tasks/terminal-probe-ownership.md, T5).
func withoutTerminalInput(t *testing.T) {
	t.Helper()
	terminalInputMu.Lock()
	previous := terminalInput
	terminalInput = nil
	terminalInputMu.Unlock()
	t.Cleanup(func() {
		terminalInputMu.Lock()
		terminalInput = previous
		terminalInputMu.Unlock()
	})
}

// capabilityReportFrom runs the probe the way the capability screen arms it and
// returns the report it produced.
func capabilityReportFrom(t *testing.T) terminalCapabilities {
	t.Helper()
	msg := loadTerminalCapabilityCmd()()
	capabilityMsg, ok := msg.(terminalCapabilityMsg)
	if !ok {
		t.Fatalf("the probe produced %T, want a terminalCapabilityMsg", msg)
	}
	return capabilityMsg.caps
}

// TestTheProbeDoesNotAskWhenTheInterfaceIsNotTheReader is T5's guard. With the
// interface reading the terminal for itself there is no reader to take the answer
// out of, and a probe that opened the terminal anyway races it: measured, one run
// in twenty lost the answer, and in that run the terminal's answer arrived as
// seven input messages. So the probe asks nothing, and the rows that only a query
// can answer say why -- never that the terminal stayed silent, which it was.
func TestTheProbeDoesNotAskWhenTheInterfaceIsNotTheReader(t *testing.T) {
	withoutTerminalInput(t)

	caps := capabilityReportFrom(t)

	const wantReason = "this run does not read the terminal from its own input, so the probe did not ask it anything (asking would put two readers on one terminal)"
	if !caps.Resolved {
		t.Error("the report is unresolved with nothing to wait for: no query was written, so there is nothing to answer it")
	}
	for _, row := range []struct {
		name   string
		answer terminalAnswer
	}{
		{"clipboard", caps.Clipboard},
		{"sync", caps.Sync},
	} {
		if row.answer.State != capabilityUnknown {
			t.Errorf("%s reports %s, want unknown: nothing was asked", row.name, row.answer.State)
		}
		if row.answer.Reason != wantReason {
			t.Errorf("%s says %q\nwant %q", row.name, row.answer.Reason, wantReason)
		}
		if row.answer.Source != "not determined" {
			t.Errorf("%s names %q as its source, want %q: there is no answer to name", row.name, row.answer.Source, "not determined")
		}
	}
}

// TestAnUnaskedProbeStillReportsTheColourFromTheEnvironment guards what the
// decision not to ask must NOT cost: reading the environment is not a query, so a
// depth COLORTERM describes is reported exactly as it was before.
func TestAnUnaskedProbeStillReportsTheColourFromTheEnvironment(t *testing.T) {
	withoutTerminalInput(t)
	t.Setenv(envColorTerm, "truecolor")

	caps := capabilityReportFrom(t)

	if caps.Color.State != capabilitySupported || caps.Color.Value != colorValueTrueColor {
		t.Errorf("colour reports %s/%s, want truecolor: reading the environment is not a query, so it survives not asking",
			caps.Color.State, caps.Color.Value)
	}
	if caps.Color.Source != envColorTerm+"=truecolor" {
		t.Errorf("colour names %q as its source, want %q", caps.Color.Source, envColorTerm+"=truecolor")
	}
}

// TestTheProbeNeverReadsTheTerminalForItself is the structural half of the same
// rule, and it is a source guard rather than a behavioural one for a reason a
// terminal-less host makes plain: with no terminal here, behaviour cannot tell
// "the probe did not ask" from "the probe asked and nothing answered". What it
// pins is the code that could race -- a private read of the controlling terminal
// -- so the one place the probe opens it stays write-only, and the only open in
// the file is that one. The first version of this guard looked for the read modes
// by name, which let `os.Open`, `os.NewFile` or a read through a variable pass
// while the document claimed the guard "fails if anything can read from a
// terminal the probe opened": it now names the shape of the whole file rather
// than one forbidden constant.
func TestTheProbeNeverReadsTheTerminalForItself(t *testing.T) {
	source, err := os.ReadFile("terminal_probe.go")
	if err != nil {
		t.Fatalf("reading the probe's source: %v", err)
	}
	for _, readMode := range []string{"os.O_RDWR", "os.O_RDONLY"} {
		if bytes.Contains(source, []byte(readMode)) {
			t.Errorf("terminal_probe.go opens the controlling terminal with %s: a second reader on the terminal the interface reads is the race this front measured", readMode)
		}
	}
	if !bytes.Contains(source, []byte("os.O_WRONLY")) {
		t.Error("terminal_probe.go no longer opens the controlling terminal write-only: the query still has to be written to it")
	}
	// One open, and it is the write-only one above: another way in, or a read
	// taken from this one, is the race coming back through a shape the two
	// constants above would not name.
	if got := bytes.Count(source, []byte("os.OpenFile(")); got != 1 {
		t.Errorf("terminal_probe.go has %d os.OpenFile calls, want exactly the one write-only open of the controlling terminal", got)
	}
	for _, forbidden := range []string{"os.Open(", "os.NewFile(", "bufio.NewReader", "io.ReadAll", ".Read("} {
		if bytes.Contains(source, []byte(forbidden)) {
			t.Errorf("terminal_probe.go contains %q: the probe writes to the terminal and never reads it, and the answer arrives through the reader the interface reads", forbidden)
		}
	}
}

// TestTheDocumentsThisFrontPublishesCarryNoConflictMarkers pins the class of
// defect a hand resolution can leave behind. The rebase that put this branch on
// top of main resolved one conflict in the guide by hand and left the separator
// line in the published file: the check the resolution ran looked for the
// opening, the closing and the base markers and never for the separator, so a
// line no reader expects shipped in the guide, in the commit and in the pull
// request, and nothing in the suite could see it. The documents this front
// publishes are read by people and by nothing else, so the guard is here.
func TestTheDocumentsThisFrontPublishesCarryNoConflictMarkers(t *testing.T) {
	for _, path := range []string{"../../../docs/tui-installer.md", "../../../CHANGELOG.md"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for i, line := range strings.Split(string(source), "\n") {
			if line == "=======" || strings.HasPrefix(line, "<<<<<<<") ||
				strings.HasPrefix(line, ">>>>>>>") || strings.HasPrefix(line, "|||||||") {
				t.Errorf("%s:%d carries a merge-conflict marker %q", path, i+1, line)
			}
		}
	}
}

// TestAProbeWithItsOwnReaderStillAsks is the other side of the decision: when the
// interface does read the terminal through the one reader, the probe asks, and a
// terminal that stays silent is reported as one that did not answer -- not as one
// that was never asked.
func TestAProbeWithItsOwnReaderStillAsks(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer read.Close()
	defer write.Close()
	TerminalInput(read)
	t.Cleanup(func() {
		terminalInputMu.Lock()
		terminalInput = nil
		terminalInputMu.Unlock()
	})

	caps := capabilityReportFrom(t)

	if strings.Contains(caps.Sync.Reason, "did not ask") {
		t.Errorf("sync says %q with the reader registered, want the silent-terminal reason: the probe did ask", caps.Sync.Reason)
	}
	if !strings.Contains(caps.Sync.Reason, "did not answer") {
		t.Errorf("sync says %q, want the reason that the terminal did not answer", caps.Sync.Reason)
	}
}
