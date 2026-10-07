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

// TestTerminalQueryDoesNotHangWhenTheTerminalNeverAnswers is the guard the whole
// utility exists under. A probe that waits for a reply it never receives leaves
// the interface dead, which is worse than not probing at all, so the read is
// bounded and a terminal that says nothing produces an empty answer within the
// timeout rather than a frozen frame.
//
// The reader is the strictest case: it supports SetReadDeadline (as the real
// controlling terminal does), so the deadline path is what is measured.
func TestTerminalQueryDoesNotHangWhenTheTerminalNeverAnswers(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	// The write end is held open and nothing is written: a terminal that is
	// silent rather than closed, which is exactly the case that would hang.
	defer writeEnd.Close()

	var out bytes.Buffer
	done := make(chan string, 1)
	started := time.Now()
	go func() {
		done <- queryTerminal(&out, readEnd, terminalCapabilityQuery, 60*time.Millisecond)
	}()

	select {
	case got := <-done:
		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Fatalf("the query took %s to give up, so a silent terminal would stall the interface", elapsed)
		}
		if got != "" {
			t.Errorf("a silent terminal produced an answer %q, want the honest empty reply", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queryTerminal never returned against a terminal that never answers: the interface would hang")
	}
}

// TestTerminalQueryReturnsWhatTheTerminalSends is the other half: a bounded read
// must still capture a real reply, so the bound is not achieved by ignoring the
// terminal.
func TestTerminalQueryReturnsWhatTheTerminalSends(t *testing.T) {
	const reply = "\x1b[?2026;1$y\x1bP1+r524742=1\x1b\\"
	var out bytes.Buffer
	got := queryTerminal(&out, strings.NewReader(reply), terminalCapabilityQuery, time.Second)
	if got != reply {
		t.Errorf("the reply = %q, want %q", got, reply)
	}
	if out.String() != terminalCapabilityQuery {
		t.Errorf("the query written = %q, want %q", out.String(), terminalCapabilityQuery)
	}
}

// TestTerminalQueryFallsBackToATimerWhenDeadlinesAreUnsupported covers the
// stream that cannot take a read deadline. The bound has to hold there too, and
// the abandoned read has to be the only leak: the guard closes the reader so the
// goroutine it left behind ends rather than outliving the test.
func TestTerminalQueryFallsBackToATimerWhenDeadlinesAreUnsupported(t *testing.T) {
	blocking := newBlockingTerminal()
	defer blocking.close()

	var out bytes.Buffer
	started := time.Now()
	got := queryTerminal(&out, blocking, terminalCapabilityQuery, 60*time.Millisecond)
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("the fallback took %s, want the bounded timeout", elapsed)
	}
	if got != "" {
		t.Errorf("a silent terminal produced %q, want the empty reply", got)
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
