package tui

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The terminal capability probe: what this terminal can do, and what the answer
// means for the interface.
//
// The probe is deliberately the only place in the installer that talks to the
// terminal. Everything else draws; this reads. Three rules keep it honest:
//
//   - Nothing blocks. The queries are written and the replies are read under a
//     deadline, so a terminal that never answers produces an honest empty reply
//     instead of a frozen interface. A probe that waits forever is worse than no
//     probe at all.
//   - Nothing is invented. An answer the probe could not find is unknown, with
//     the reason and a way to check by hand -- never a "no" the installer made
//     up about a terminal it did not understand.
//   - Every answer names its source: the exact environment variable, the
//     terminal's own reply to a query, or "not determined". A reader can audit
//     each row.
//
// Nothing here runs at startup. The probe is a command the utilities section
// arms only when the capability screen is opened, so a run that never visits
// that screen writes not a single query byte.

const (
	// envColorTerm is the variable terminals export to describe their colour
	// depth ("truecolor", "24bit"). It is the strongest environment signal.
	envColorTerm = "COLORTERM"

	// envTermProgram is the variable a terminal exports to name itself
	// ("kitty", "WezTerm", "iTerm.app"). A named terminal is a known terminal.
	envTermProgram = "TERM_PROGRAM"

	// envSync names the switch that turns the installer's own synchronized-output
	// writer off. It is read here only to explain an unknown mode answer: the
	// terminal's capability and the installer's use of it are different facts.
	envSync = "DOTFILES_SYNC"

	// envSyncOff is the value of envSync that turns the writer off.
	envSyncOff = "0"

	// controllingTerminal is the terminal the probe reads and writes: the
	// controlling terminal, opened separately so the queries do not depend on
	// whether the run's stdin was redirected. It does not exist in a container or
	// a service without a terminal, and the probe then answers unknown with that
	// reason.
	controllingTerminal = "/dev/tty"

	// terminalQueryTimeout bounds the whole exchange. It is short on purpose: a
	// human opened a screen and is waiting, so a terminal that has not answered in
	// a quarter of a second is treated as one that will not.
	terminalQueryTimeout = 250 * time.Millisecond
)

// The three capability states. unknown is the zero value on purpose: an answer
// the probe did not build is unknown, never a silent "no".
type capabilityState int

const (
	capabilityUnknown capabilityState = iota
	capabilitySupported
	capabilityUnsupported
)

// String names the state the way the screen and the guards read it.
func (s capabilityState) String() string {
	switch s {
	case capabilitySupported:
		return "supported"
	case capabilityUnsupported:
		return "not supported"
	default:
		return "unknown"
	}
}

// The colour depths the screen names, lowest first. They are the levels the
// utility must report: truecolor, 256, 16, or none.
const (
	colorValueTrueColor = "truecolor (24-bit)"
	colorValue256       = "256 colours"
	colorValue16        = "16 colours"
	colorValueNone      = "no colour"
)

// terminalAnswer is one capability's answer: its state, the wording the screen
// shows, where it came from, why it is unknown (when it is) and how a reader can
// check by hand. Source, Reason and Manual are what make the report auditable
// instead of a set of guesses.
type terminalAnswer struct {
	State  capabilityState
	Value  string
	Source string
	Reason string
	Manual string
}

// stateWord is the plain three-state wording, used by the guards and by the
// capabilities that are a plain yes/no.
func (a terminalAnswer) stateWord() string { return a.State.String() }

// terminalCapabilities is the whole report, resolved once when the screen is
// opened and stored on the model so the render never probes.
type terminalCapabilities struct {
	Resolved  bool
	Color     terminalAnswer
	Clipboard terminalAnswer
	Sync      terminalAnswer
	NerdFont  terminalAnswer
}

// terminalCapabilityMsg carries a finished report back to the update loop.
type terminalCapabilityMsg struct{ caps terminalCapabilities }

// terminalCapabilityRow is one row of the report: the capability, the answer and
// what the answer implies for the themes and the interface.
type terminalCapabilityRow struct {
	Name        string
	Answer      terminalAnswer
	Implication string
}

// rows is the report in the order the screen shows it. It is built here, beside
// the answers, so the screen has no second list to keep in step.
func (c terminalCapabilities) rows() []terminalCapabilityRow {
	return []terminalCapabilityRow{
		{"Colour depth", c.Color, colourImplication(c.Color)},
		{"Clipboard (OSC 52)", c.Clipboard, clipboardImplication(c.Clipboard)},
		{"Synchronized output (mode 2026)", c.Sync, syncImplication(c.Sync)},
		{"Nerd Font glyphs", c.NerdFont, nerdFontImplication(c.NerdFont)},
	}
}

// panelValue is what the main menu's Utilities panel says the terminal row
// holds. Before the screen has been opened there is no reading yet, and the
// panel says so rather than inventing one; once it has run, the colour depth is
// the one-word answer the row is named for.
func (c terminalCapabilities) panelValue() string {
	if !c.Resolved {
		return "not inspected yet"
	}
	return c.Color.Value
}

// ============================================================================
// The query half
// ============================================================================

// The queries the probe sends, in one payload so there is one write and one
// bounded read rather than a deadline per question:
//
//   - DECRQM for private mode 2026 is the terminal's own answer to whether it
//     implements synchronized output (DECSET 2026). The reply is
//     "\x1b[?2026;<state>$y": 0 means the mode is not recognized.
//   - XTGETTCAP asks terminfo directly. "RGB" is the boolean that says the
//     terminal will draw 24-bit colour, and "Ms" is the capability behind
//     OSC 52, the clipboard escape that makes copying work over SSH. Both are
//     sent as hex names, the form XTGETTCAP requires.
const (
	decrqmSyncQuery = "\x1b[?2026$p"

	xtgettcapRGBQuery       = "\x1bP+q524742\x1b\\" // "RGB"
	xtgettcapClipboardQuery = "\x1bP+q4D73\x1b\\"   // "Ms"

	xtgettcapRGBKey       = "524742"
	xtgettcapClipboardKey = "4D73"

	terminalCapabilityQuery = decrqmSyncQuery + xtgettcapRGBQuery + xtgettcapClipboardQuery
)

// trueColorTerminalPrograms are the terminals whose names alone settle the
// colour depth. The list is short and deliberate: it holds the terminals this
// repository ships configuration for, never a guess about one it does not know.
var trueColorTerminalPrograms = map[string]bool{
	"alacritty": true,
	"wezterm":   true,
	"kitty":     true,
	"ghostty":   true,
}

// queryTerminal writes the capability payload to out and reads the terminal's
// reply from in for at most timeout. It is the only place the installer touches
// the terminal, and the only place a silent terminal could stall the interface,
// so the read is bounded: whatever arrived by the deadline is the answer, and
// what did not arrive is treated as no answer at all.
func queryTerminal(out io.Writer, in io.Reader, payload string, timeout time.Duration) string {
	if _, err := io.WriteString(out, payload); err != nil {
		return ""
	}
	return readTerminalReply(in, timeout)
}

// readTerminalReply reads for at most timeout. A stream that supports a read
// deadline -- the controlling terminal does -- is read under that deadline; a
// stream that does not is read on a goroutine the deadline abandons. Either way
// the call returns by the deadline.
func readTerminalReply(in io.Reader, timeout time.Duration) string {
	if d, ok := in.(interface{ SetReadDeadline(time.Time) error }); ok {
		if err := d.SetReadDeadline(time.Now().Add(timeout)); err == nil {
			defer func() { _ = d.SetReadDeadline(time.Time{}) }()
			return drainTerminal(in)
		}
	}
	return readTerminalReplyWithTimer(in, timeout)
}

// drainTerminal collects everything the stream has until it errors or the
// deadline fires. A read deadline surfaces as an error, which is what ends the
// loop, so this cannot outlive the deadline set by readTerminalReply.
func drainTerminal(in io.Reader) string {
	var b strings.Builder
	buf := make([]byte, 256)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}
		if err != nil {
			return b.String()
		}
	}
}

// readTerminalReplyWithTimer is the fallback for a stream that cannot take a
// read deadline. The read is abandoned on the timeout rather than cancelled (a
// Reader cannot be cancelled), so the abandoned goroutine is the price of a
// bound the caller cannot otherwise get; the probe never waits on it.
func readTerminalReplyWithTimer(in io.Reader, timeout time.Duration) string {
	done := make(chan string, 1)
	go func() { done <- drainTerminal(in) }()
	select {
	case reply := <-done:
		return reply
	case <-time.After(timeout):
		return ""
	}
}

// terminalReply opens the controlling terminal and runs the capability query.
// A host with no controlling terminal -- a container, a service, a run whose
// stdin is a pipe and whose /dev/tty does not exist -- gets an empty reply, and
// the screen then reports unknown with that reason instead of failing.
func terminalReply() string {
	f, err := os.OpenFile(controllingTerminal, os.O_RDWR, 0)
	if err != nil {
		return ""
	}
	defer f.Close()
	return queryTerminal(f, f, terminalCapabilityQuery, terminalQueryTimeout)
}

// ============================================================================
// The interpretation half
// ============================================================================

// loadTerminalCapabilityCmd runs the whole probe off the update loop: the
// terminal open, the bounded query and the interpretation. It is armed when the
// user opens the capability screen.
func loadTerminalCapabilityCmd() tea.Cmd {
	return func() tea.Msg {
		return terminalCapabilityMsg{caps: detectTerminalCapabilities(os.Getenv, terminalReply())}
	}
}

// terminalCapabilityCmdIfNeeded arms the probe the first time the capability
// screen is opened, the way the theme definitions and the WSL state are read. It
// returns nil once the report is resolved, so revisiting the screen does not
// query the terminal again.
func (m *Model) terminalCapabilityCmdIfNeeded() tea.Cmd {
	if m.TerminalCapabilities.Resolved {
		return nil
	}
	return loadTerminalCapabilityCmd()
}

// detectTerminalCapabilities turns the environment and the terminal's reply into
// the report. It performs no I/O, so the three-state rules are testable without
// a terminal.
func detectTerminalCapabilities(getenv func(string) string, reply string) terminalCapabilities {
	return terminalCapabilities{
		Resolved:  true,
		Color:     probeColour(getenv, reply),
		Clipboard: probeClipboard(reply),
		Sync:      probeSync(getenv, reply),
		NerdFont:  probeNerdFont(),
	}
}

// probeColour answers the depth the terminal will paint. The terminal's own RGB
// reply wins because it is the terminal answering for itself; then COLORTERM,
// which is what modern terminals export for exactly this; then the terminal's
// name; then TERM, which carries the depth in its suffix. Nothing knows is
// unknown.
func probeColour(getenv func(string) string, reply string) terminalAnswer {
	if state, ok := parseXTGetTCAP(reply, xtgettcapRGBKey); ok && state == capabilitySupported {
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValueTrueColor,
			Source: "the terminal's reply to the RGB capability query",
		}
	}

	colorTerm := strings.ToLower(strings.TrimSpace(getenv(envColorTerm)))
	term := strings.TrimSpace(getenv(envTerm))
	program := strings.ToLower(strings.TrimSpace(getenv(envTermProgram)))

	switch {
	case colorTerm == "24bit" || colorTerm == "truecolor":
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValueTrueColor,
			Source: envColorTerm + "=" + getenv(envColorTerm),
		}
	case trueColorTerminalPrograms[program]:
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValueTrueColor,
			Source: envTermProgram + "=" + getenv(envTermProgram),
		}
	case strings.Contains(strings.ToLower(term), "256color"):
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValue256,
			Source: envTerm + "=" + term,
		}
	case term == "xterm" || term == "linux":
		// The two names that predate the "color" suffix still promise the
		// sixteen ANSI colours; termenv reads them the same way.
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValue16,
			Source: envTerm + "=" + term,
		}
	case strings.Contains(strings.ToLower(term), "color") || strings.Contains(strings.ToLower(term), "ansi"):
		return terminalAnswer{
			State:  capabilitySupported,
			Value:  colorValue16,
			Source: envTerm + "=" + term,
		}
	case term == "dumb":
		return terminalAnswer{
			State:  capabilityUnsupported,
			Value:  colorValueNone,
			Source: envTerm + "=dumb",
		}
	default:
		return terminalAnswer{
			State:  capabilityUnknown,
			Value:  "unknown",
			Source: "not determined",
			Reason: fmt.Sprintf("the terminal did not answer the colour query and neither %s nor %s describes a depth",
				envColorTerm, envTerm),
			Manual: fmt.Sprintf("run `echo \"$%s $%s\"` and compare it with the terminal's documentation", envTerm, envColorTerm),
		}
	}
}

// probeClipboard answers OSC 52 support from the terminal's terminfo reply. The
// environment does not describe it -- a terminal can support OSC 52 without any
// variable saying so -- so silence is unknown, not a no.
func probeClipboard(reply string) terminalAnswer {
	if state, ok := parseXTGetTCAP(reply, xtgettcapClipboardKey); ok {
		return terminalAnswer{
			State:  state,
			Value:  state.String(),
			Source: "the terminal's reply to the Ms capability query",
		}
	}
	return terminalAnswer{
		State:  capabilityUnknown,
		Value:  "unknown",
		Source: "not determined",
		Reason: "the terminal did not answer the Ms capability query and no environment variable describes OSC 52",
		Manual: "copy text here and paste it into another window; over SSH that is the check that matters",
	}
}

// probeSync answers synchronized-output support from the terminal's DECRQM reply
// for mode 2026: the mode itself saying whether it is recognized. DOTFILES_SYNC
// is read only to explain the report -- it is the installer's own switch, not
// the terminal's capability -- and silence is unknown.
func probeSync(getenv func(string) string, reply string) terminalAnswer {
	if state, ok := parseSyncSupport(reply); ok {
		return terminalAnswer{
			State:  state,
			Value:  state.String(),
			Source: "the terminal's reply to the mode 2026 query",
		}
	}

	source := "not determined"
	reason := "the terminal did not answer the mode 2026 query"
	if getenv(envSync) == envSyncOff {
		source = envSync + "=" + envSyncOff
		reason += fmt.Sprintf("; %s=%s also turns the installer's own use of the mode off", envSync, envSyncOff)
	}
	return terminalAnswer{
		State:  capabilityUnknown,
		Value:  "unknown",
		Source: source,
		Reason: reason,
		Manual: "watch a large redraw: if the frame tears as it paints, the terminal does not implement DECSET 2026",
	}
}

// probeNerdFont is the one capability with no query. Whether a glyph renders as
// an icon depends on the font the terminal was given, which the terminal does
// not report, so the answer is unknown with a check the reader can make with
// their own eyes rather than a guess from a variable that does not describe it.
func probeNerdFont() terminalAnswer {
	return terminalAnswer{
		State:  capabilityUnknown,
		Value:  "unknown",
		Source: "not determined",
		Reason: "no terminal query reports whether a Nerd Font is installed; the terminal draws the glyphs it is given",
		Manual: "look at the marks on this screen: a box or a question mark where an icon belongs means the font is missing",
	}
}

// parseSyncSupport reads the DECRQM reply for mode 2026. The bool says whether
// the reply answered at all: a positive mode state is supported and the "not
// recognized" state (0) is unsupported, while a reply about another mode leaves
// the answer unknown rather than being read as a no.
func parseSyncSupport(reply string) (capabilityState, bool) {
	marker := "?2026;"
	idx := strings.Index(reply, marker)
	if idx < 0 {
		return capabilityUnknown, false
	}
	rest := reply[idx+len(marker):]
	end := strings.IndexByte(rest, '$')
	if end < 0 {
		return capabilityUnknown, false
	}
	state, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil {
		return capabilityUnknown, false
	}
	if state == 0 {
		return capabilityUnsupported, true
	}
	return capabilitySupported, true
}

// parseXTGetTCAP reads an XTGETTCAP reply for one capability key. A success is
// "1+r<KEY>[=<value>]" and a refusal is "0+r<KEY>"; the bool says whether the
// terminal answered about this key at all, so an unrelated success is not read
// as an answer.
func parseXTGetTCAP(reply, key string) (capabilityState, bool) {
	if strings.Contains(reply, "1+r"+key) {
		return capabilitySupported, true
	}
	if strings.Contains(reply, "0+r"+key) {
		return capabilityUnsupported, true
	}
	return capabilityUnknown, false
}

// ============================================================================
// What each answer means
// ============================================================================

// colourImplication states the consequence for the themes, and it is stated for
// every state: a depth the probe did not determine still warns that the themes
// will look approximate if it is not truecolor.
func colourImplication(a terminalAnswer) string {
	switch {
	case a.State == capabilityUnknown:
		return "if this is not truecolor the themes will look approximate"
	case a.Value == colorValueTrueColor:
		return "the themes are painted with their exact colours"
	default:
		return "the themes will look approximate (the palette is reduced to this depth)"
	}
}

// clipboardImplication states the consequence for copying, and it matters most
// over SSH, which is where OSC 52 is the only route to the local clipboard.
func clipboardImplication(a terminalAnswer) string {
	switch a.State {
	case capabilitySupported:
		return "copying reaches the clipboard, including over SSH"
	case capabilityUnsupported:
		return "copying will not reach the clipboard over SSH"
	default:
		return "copying over SSH may not reach the clipboard"
	}
}

// syncImplication states the consequence for the redraw: without the mode the
// frame is painted as it arrives, which is the tearing the repository already
// fixed once.
func syncImplication(a terminalAnswer) string {
	switch a.State {
	case capabilitySupported:
		return "the frame is painted at once, so a redraw should not tear"
	case capabilityUnsupported:
		return "a large redraw may flicker or tear"
	default:
		return "a large redraw may flicker or tear if the terminal does not implement the mode"
	}
}

// nerdFontImplication states what a missing font looks like on screen.
func nerdFontImplication(terminalAnswer) string {
	return "icons may draw as boxes or question marks if the Nerd Font is missing"
}
