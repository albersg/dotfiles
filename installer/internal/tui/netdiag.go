package tui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/netdiag"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UTILITIES SECTION: A NETWORK DIAGNOSIS
// ============================================================================
//
// This is the utility that answers "why can I not reach it", and it answers it
// by stages: the local network, DNS, the transport, the certificate and the
// application. A name that does not resolve, a port that refuses, a socket that
// is filtered into silence and a certificate this machine does not trust are
// four different failures with four different owners, and a screen that says
// "no internet" for all of them is worse than no screen at all.
//
// It is a diagnosis, not a scanner: every check reads, nothing is configured and
// nothing is written, and the run is bounded per stage and cancellable from this
// screen. The overview reaches for the host this installer already contacts for
// its own update check -- named on the screen, never hidden -- and everything
// else is a destination the user typed. A target the field refuses is refused by
// the validator with the reason, before anything is dialled.
//
// The rows are the report: one row per check, carrying its status, and the body
// under the cursor says what that step measured and what to do about it. That is
// what keeps the number of checks from being bounded by the height of the frame:
// the list windows around the cursor the way every other list on a utilities
// screen does.
//
// The screen keeps its own rows and its own rendering in this file rather than
// in view.go, because the utilities section's shared files are being edited by
// other fronts at the same time; the only entries there are one row constant's
// worth of wiring.

const (
	// utilitiesNetworkRow is the utilities section's row for the diagnosis, and
	// the row the section's handler matches. It is always offered: the report is
	// read-only, and there is always a machine whose connectivity is worth
	// describing.
	utilitiesNetworkRow = "Network Diagnostics"

	// networkRunRow starts a run and starts another one afterwards. Its label is
	// deliberately the same before and after the first report: the cursor holds
	// the row it was on while check rows arrive under it, and a label that changed
	// with the state would leave the cursor on a row that no longer exists. It is a
	// command, so it carries the letter that runs it; Enter runs nothing here.
	networkRunRow = networkRunMark + "Run the connectivity checks"

	// networkTargetRow opens the field a destination is typed into. It opens, so
	// Enter is its key and the arrow in front of it is what says that.
	networkTargetRow = networkOpenMark + "Check a host, URL or port…"

	// networkTraceLabel is the one check that starts an external program. It is off
	// by default, it is only ever offered for a destination the user entered, and the
	// row's own body says what the system's tool needs and why a hop that answers
	// nothing is not a fault. Its row runs on its own letter and says the value it
	// holds, because nothing on this screen acts on Enter.
	networkTraceLabel = "Include a traceroute (needs the system's own tool)"

	// networkPreviewRow opens the report as text. It opens a detail of this screen,
	// so Enter keeps it. The write only ever happens from the state the preview
	// shows, which is the preview the user has to see before anything is saved.
	networkPreviewRow = networkOpenMark + "Review the report as text"

	// networkSaveRow writes the reviewed report. It is offered only in the
	// preview, so the file is never written from a state the user has not read. It
	// is a command and carries its own letter.
	networkSaveRow = networkSaveMark + "Save the report to the file shown"

	// networkAnonymizeLabel is the anonymization the preview and the write use:
	// every address, the user name, the home directory and the machine name are
	// replaced, so the file can be sent to somebody else. Like the traceroute its row
	// runs on its own letter and says the value it holds.
	networkAnonymizeLabel = "Anonymize the report before saving (local names and addresses)"

	// networkBackRow is the way back. Escape is its key, and the arrow pointing away
	// from the screen is what says so.
	networkBackRow = networkBackMark + "Back"

	// networkReportPrefix names the file inside the installer's own state
	// directory. It carries no secrets by construction, which is why it may live
	// there at all; its mode is the user's alone because it still describes this
	// machine's network.
	networkReportPrefix = "network-report-"

	// networkInputMax bounds the destination field. A host, a URL with a path and a
	// query is far shorter than this, and a field with no bound is a field a paste
	// can fill until the model is carrying megabytes.
	networkInputMax = 512

	// networkEventBuffer is how many check results may be waiting for the update
	// loop. It only needs to cover the gap between two reads of the channel.
	networkEventBuffer = 8

	// networkReportMode is the permission the saved report is created with. The
	// report names addresses and interfaces of this machine -- redacted of
	// credentials, not of everything -- so it belongs to the user alone.
	networkReportMode = 0o600
)

// The keys this screen's rows run on, and the marks that advertise them. Every row
// that acts where it stands carries its letter in brackets, and the two rows that
// hold a value say that value at the end of the row. The rows that open something
// carry the arrow instead, because Enter is their key, and the way back carries the
// arrow pointing the other way, because Escape is its key (odd/tasks/key-grammar.md).
const (
	networkRunKey       = "r"
	networkSaveKey      = "s"
	networkTraceKey     = "t"
	networkAnonymizeKey = "a"

	networkRunMark       = "[" + networkRunKey + "] "
	networkSaveMark      = "[" + networkSaveKey + "] "
	networkTraceMark     = "[" + networkTraceKey + "] "
	networkAnonymizeMark = "[" + networkAnonymizeKey + "] "

	networkOpenMark = "→ "
	networkBackMark = "← "
)

// networkRowMarks is the table a row's claim is read from: the mark a row carries
// in front of its label, and the key that keeps the promise it makes. The handler
// and the grammar's own test both read it, so a mark cannot claim a key the screen
// does not honour. A row with no mark is a check row or a divider, and no key
// changes the screen through it.
var networkRowMarks = map[string]string{
	networkRunMark:       networkRunKey,
	networkSaveMark:      networkSaveKey,
	networkTraceMark:     networkTraceKey,
	networkAnonymizeMark: networkAnonymizeKey,
	networkOpenMark:      "enter",
	networkBackMark:      "esc",
}

// networkRowKey is the key a row declares on itself, or "" for a row that only
// reports what it measured.
func networkRowKey(label string) string {
	for mark, key := range networkRowMarks {
		if strings.HasPrefix(label, mark) {
			return key
		}
	}
	return ""
}

// networkValueRow draws a row that holds a value: the letter that turns it over in
// front, and the value it holds now after the label. The value is written as a word
// rather than in a switch's brackets, because this screen has no switches: Space
// belongs to the leader key here, so a row that showed a state in a checkbox would
// be advertising a press that never arrives.
func networkValueRow(mark, label string, on bool) string {
	state := "off"
	if on {
		state = "on"
	}
	return mark + label + " — " + state
}

// networkTraceRow and networkAnonymizeRow are the screen's two value rows as they
// read now: each says the value its own letter would turn over.
func (m Model) networkTraceRow() string {
	return networkValueRow(networkTraceMark, networkTraceLabel, m.Network.Trace)
}

func (m Model) networkAnonymizeRow() string {
	return networkValueRow(networkAnonymizeMark, networkAnonymizeLabel, m.Network.Anonymize)
}

// networkIsActionRow reports whether a row acts rather than reporting a check: the
// rows above the checks, the export rows below them, the way back, and the divider
// that separates them. It is what tells the cursor's index into the options apart
// from its index into the checks.
func (m Model) networkIsActionRow(row string) bool {
	switch row {
	case networkRunRow, networkTargetRow, networkPreviewRow, networkSaveRow, networkBackRow,
		m.networkTraceRow(), m.networkAnonymizeRow():
		return true
	}
	return strings.HasPrefix(row, menuSeparatorPrefix)
}

// networkState is everything the diagnosis screen draws: the destination field,
// the run in flight, the checks as they arrive, and the export.
//
// The split between Checks and Report is what makes the screen progressive: the
// checks are appended as they finish, so a slow stage does not hide a fast one,
// and the finished report replaces them whole -- including the checks a stop
// left unrun -- when the run ends.
type networkState struct {
	// Input is what has been typed into the destination field, and InputActive is
	// whether the field is open. InputErr is the validator's reason the last
	// attempt did not run; it is cleared when the field is opened again.
	Input       string
	InputActive bool
	InputErr    string

	// Running is true from the press until the run reports, and Run is the run the
	// events are read from. Both are needed: Running keeps a second press from
	// starting a second run, and Run is what the pending read is re-armed on.
	Running bool
	Run     *networkRun
	// Checks is what has arrived so far, and Report the finished run. Exactly one
	// of them is the screen's data at a time.
	Checks []netdiag.Check
	Report *netdiag.Report

	// ReportText is the report exactly as the preview shows it and the write saves
	// it. It is built when the preview opens and when the anonymization is toggled --
	// both are key presses -- because building it anonymizes by looking this machine
	// up, and the render path must not read a file or a hostname to draw a frame.
	ReportText string

	// Trace asks for the opt-in traceroute, Preview shows the report as text, and
	// Anonymize is the toggle the preview and the write use.
	Trace     bool
	Preview   bool
	Anonymize bool

	// ReportPath is the file the export would write, resolved when the report
	// lands so the row and the write cannot disagree about it. It is empty when the
	// state directory could not be determined, in which case no save row is
	// offered and the screen says why.
	ReportPath string
	// Notice is the screen's result line: what is running, what was refused, what
	// was saved.
	Notice string

	// Probe is the seam the run's effects go through. It is nil in a real run --
	// the screen then builds the default probe, which reads this machine -- and a
	// test sets it so no check can reach a network or a file.
	Probe *netdiag.Probe
}

// networkRun is one diagnosis in flight: the context that stops it, and the
// channel its checks arrive on. The channel is buffered so a check that finishes
// while the update loop is drawing does not block the run.
type networkRun struct {
	ctx    context.Context
	cancel context.CancelFunc
	events chan tea.Msg
}

// networkCheckMsg is one check finishing while the run continues.
type networkCheckMsg struct{ check netdiag.Check }

// networkDoneMsg is the finished report. It replaces the streamed checks whole,
// so the checks a stop left unrun are named as skipped rather than absent.
type networkDoneMsg struct {
	report  netdiag.Report
	stopped bool
}

// networkClosedMsg is the event channel closing without a report: the run was
// abandoned. It is what stops the screen believing a run is still in flight.
type networkClosedMsg struct{}

// networkProbe is the probe the run uses: this machine's own, unless a test put
// one in the model. The WSL flag comes from the host detection the installer
// already made, because the guest's resolver and interfaces are not the host's
// and the report has to say which side it is describing.
func (m Model) networkProbe() netdiag.Probe {
	probe := netdiag.DefaultProbe()
	if m.Network.Probe != nil {
		probe = *m.Network.Probe
	}
	if m.SystemInfo != nil {
		probe.WSL = m.SystemInfo.IsWSL
	}
	return probe
}

// networkDestination is the predefined destination an overview reaches for. It is
// derived from the release endpoint the installer already reads for its own
// update check -- the same host, the API's root rather than a release path -- so
// the screen can name it and this utility contacts no third party the installer
// was not already talking to.
func networkDestination() netdiag.Destination {
	u, err := url.Parse(dotfilesReleaseAPIURL)
	if err != nil || u.Host == "" {
		return netdiag.Destination{}
	}
	root := &url.URL{Scheme: u.Scheme, Host: u.Host}
	return netdiag.Destination{
		URL:    root.String(),
		Source: "the host this installer already contacts for its own update check",
	}
}

// networkOptions builds what the run is asked for from the screen's own state.
// It returns the validator's error rather than a target it could not parse: a
// destination that cannot be diagnosed is refused before anything is dialled.
func (m Model) networkOptions() (netdiag.Options, error) {
	options := netdiag.Options{
		Destination: networkDestination(),
		Bounds:      netdiag.DefaultBounds(),
	}
	if raw := strings.TrimSpace(m.Network.Input); raw != "" {
		target, err := netdiag.ParseTarget(raw)
		if err != nil {
			return netdiag.Options{}, err
		}
		options.Target = target
		// The traceroute is only ever run for a destination the user asked about:
		// an overview has no single host whose path would mean anything, and
		// starting an external program on the installer's own initiative is exactly
		// what this utility does not do.
		options.Trace = m.Network.Trace
	}
	return options, nil
}

// startNetworkRun opens a run and returns the command that starts it. The model
// keeps the run, so the checks that arrive while the user is on another screen
// still land on this one: leaving a screen is not cancelling the work it asked
// for.
func (m Model) startNetworkRun() (tea.Model, tea.Cmd) {
	if m.Network.Running {
		return m, nil
	}
	options, err := m.networkOptions()
	if err != nil {
		// The refusal is the field's own reason, so the field is what the user
		// needs to see: it is reopened with the message under it.
		m.Network.InputErr = err.Error()
		m.Network.InputActive = true
		m.Network.Notice = "The destination was refused; nothing was dialled."
		return m, nil
	}

	run := &networkRun{events: make(chan tea.Msg, networkEventBuffer)}
	run.ctx, run.cancel = context.WithCancel(context.Background())

	m.Network.Run = run
	m.Network.Running = true
	m.Network.Checks = nil
	m.Network.Report = nil
	m.Network.Preview = false
	m.Network.ReportPath = ""
	m.Network.InputErr = ""
	m.Network.InputActive = false
	m.Network.Notice = "Running the checks; Esc stops the run."
	return m, m.networkRunCmd(options)
}

// networkRunCmd starts the diagnosis off the update loop and waits for its first
// event. The goroutine is the only thing that dials; every event it produces is
// read by the pending wait, which the update loop re-arms for each one.
func (m Model) networkRunCmd(options netdiag.Options) tea.Cmd {
	run, probe := m.Network.Run, m.networkProbe()
	return func() tea.Msg {
		go runNetworkDiagnosis(run, probe, options)
		return waitNetworkEvent(run)()
	}
}

// runNetworkDiagnosis runs one diagnosis and feeds its checks and its report to
// the model. The report is always delivered, a cancelled one included, so a stop
// shows the checks that finished instead of an empty screen; the context is what
// makes a slow stage give up rather than hold the run open.
func runNetworkDiagnosis(run *networkRun, probe netdiag.Probe, options netdiag.Options) {
	defer close(run.events)

	report := netdiag.Run(run.ctx, probe, options, func(check netdiag.Check) {
		select {
		case run.events <- networkCheckMsg{check: check}:
		case <-run.ctx.Done():
			// The screen stopped reading, which only happens when the program is
			// leaving: nothing is waiting for this check.
		}
	})

	select {
	case run.events <- networkDoneMsg{report: report, stopped: report.Stopped}:
	case <-run.ctx.Done():
	}
}

// waitNetworkEvent waits for one event of a run. It is re-armed for each one, and
// the close of the channel is what tells the screen the run is over when the
// report itself was dropped.
func waitNetworkEvent(run *networkRun) tea.Cmd {
	return func() tea.Msg {
		if run == nil {
			return networkClosedMsg{}
		}
		msg, ok := <-run.events
		if !ok {
			return networkClosedMsg{}
		}
		return msg
	}
}

// cancelNetworkRun stops the run in flight. It is what Esc does while the screen
// says a run is going: the report still arrives, with the checks that finished,
// so stopping early shows what was found rather than throwing it away.
func (m *Model) cancelNetworkRun() {
	if m.Network.Run != nil {
		m.Network.Run.cancel()
	}
}

// networkChecks is the checks the screen draws: the finished report's when there
// is one, and the streamed ones while the run is still going.
func (m Model) networkChecks() []netdiag.Check {
	if m.Network.Report != nil {
		return m.Network.Report.Checks
	}
	return m.Network.Checks
}

// networkRows is what the screen offers, derived from its state rather than
// typed: the rows that act, the check rows that are the report, the export rows
// once there is something to export, and the way back.
//
// The check rows are inert by design and the cursor on them is what selects the
// detail in the body. The rows are also what keeps a long report on a short
// frame: the list windows around the cursor, so the number of checks is not
// bounded by the height of the terminal.
func (m Model) networkRows() []string {
	rows := []string{networkRunRow}
	if !m.Network.InputActive {
		rows = append(rows, networkTargetRow)
	}

	checks := m.networkChecks()
	if len(checks) > 0 {
		rows = append(rows, m.menuSeparator())
		for _, check := range checks {
			rows = append(rows, networkCheckRow(check))
		}
	}

	var export []string
	export = append(export, m.networkTraceRow())
	if m.Network.Report != nil {
		export = append(export, networkPreviewRow)
		if m.Network.Preview {
			// The write is offered only from the preview: the state that shows the file
			// line for line is the consent the write runs under, so a user cannot save a
			// report they have not read (S10). The anonymization toggle appears here too,
			// because it changes what the file that is about to be written holds.
			if m.Network.ReportPath != "" {
				export = append(export, networkSaveRow)
			}
			export = append(export, m.networkAnonymizeRow())
		}
	}
	rows = append(rows, m.menuSeparator())
	rows = append(rows, export...)

	return append(rows, m.menuSeparator(), networkBackRow)
}

// networkCheckRow is one check as a row: its status word, the stage it answers
// for and what it asked. The words carry the state, not a colour and not a
// glyph, so the row still reads on a terminal with no colour at all.
func networkCheckRow(check netdiag.Check) string {
	return fmt.Sprintf("[%s] %s — %s", check.Status, networkStageLabel(check.Stage), check.Title)
}

// networkStageLabel is the stage's short name for a row. The report's own
// sentences use the long form ("the transport did not answer"); a row has to stay
// short enough to sit beside its own status.
func networkStageLabel(stage netdiag.Stage) string {
	switch stage {
	case netdiag.StageLocal:
		return "local"
	case netdiag.StageResolver:
		return "DNS"
	case netdiag.StageTransport:
		return "transport"
	case netdiag.StageApplication:
		return "application"
	case netdiag.StageCertificate:
		return "certificate"
	case netdiag.StageProxy:
		return "proxy"
	}
	return string(stage)
}

// networkCheckRowStart is the index of the first check row in the current option
// list: the rows before it are the actions and a separator, and it is what turns
// the cursor into a check. It returns an index past the end when there is no
// check row, which the caller's bounds check turns into "no check selected".
func (m Model) networkCheckRowStart() int {
	options := m.GetCurrentOptions()
	start := 0
	for start < len(options) && m.networkIsActionRow(options[start]) {
		start++
	}
	return start
}

// selectedNetworkCheck is the check the cursor is on, when it is on one.
func (m Model) selectedNetworkCheck() (netdiag.Check, bool) {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return netdiag.Check{}, false
	}
	checks := m.networkChecks()
	index := m.Cursor - m.networkCheckRowStart()
	if index < 0 || index >= len(checks) {
		return netdiag.Check{}, false
	}
	return checks[index], true
}

// ============================================================================
// Drawing
// ============================================================================

// The diagnosis screen's own hints. Each names the key its row carries: the four
// commands on their letters, Enter on the rows that open something, and Escape on
// the way back -- or on stopping the run, which is what Escape does while one is in
// flight. Space is in none of them: this screen has no switches.
var (
	networkRunHint       = installerHint{"[" + networkRunKey + "]", "run"}
	networkSaveHint      = installerHint{"[" + networkSaveKey + "]", "save"}
	networkTraceHint     = installerHint{"[" + networkTraceKey + "]", "traceroute"}
	networkAnonymizeHint = installerHint{"[" + networkAnonymizeKey + "]", "anonymize"}
	// networkStopRunHint is Escape while a run is going: the key stops the run
	// rather than leaving the screen, so the footer names that meaning and not the
	// other one.
	networkStopRunHint = installerHint{"[Esc]", "stop the run"}
)

// renderNetworkDiagnostics draws the screen: the field while a destination is
// being typed, the report as text while the preview is open, the detail of the
// check under the cursor, or the answer the run gave.
func (m Model) renderNetworkDiagnostics() string {
	var hints []installerHint
	switch {
	case m.Network.InputActive:
		// While the field is open its keys are the field's: Enter diagnoses what
		// is typed and Esc closes it, which is what the legend says.
		hints = []installerHint{hintValidate, hintCancel}
	case m.Network.Running:
		// A run in flight: Escape stops it rather than leaving the screen, and the
		// footer says so instead of offering the way back it is not doing.
		hints = []installerHint{hintUp, hintDown, networkStopRunHint}
	default:
		// Enter opens the two rows that lead somewhere, so it is in the legend with
		// the shared spelling of its one meaning; the letters are this screen's own.
		hints = []installerHint{hintUp, hintDown, hintOpen, networkRunHint, networkTraceHint}
		if m.Network.Report != nil && m.Network.Preview {
			// The write's row and the anonymization exist only in the preview, so their
			// letters are named only there: a footer entry with no row behind it would
			// offer a press that does nothing.
			if m.Network.ReportPath != "" {
				hints = append(hints, networkSaveHint)
			}
			hints = append(hints, networkAnonymizeHint)
		}
		hints = append(hints, hintBack)
	}
	return m.renderThemeScreen(Model.networkDescription, m.networkNoticeLine(), hints)
}

// networkNoticeLine is the screen's result line: what is running right now, or
// the last outcome. The progress is drawn from the checks that have arrived
// rather than from a counter, so the number on screen is a number of finished
// steps and not an estimate.
func (m Model) networkNoticeLine() string {
	if m.Network.Running {
		return fmt.Sprintf("Running the checks… %d finished so far. Esc stops the run and keeps what finished.",
			len(m.Network.Checks))
	}
	return m.Network.Notice
}

// networkDescription is the body under the rows, and it follows the cursor: the
// field while it is open, the report text in the preview, the detail of the
// check the cursor is on, and otherwise the answer.
//
// Every state of it is the prose of networkAreas, which is what the card beside the
// list draws: one row per fact, so the floor and the wide frame cannot explain one
// check two ways.
func (m Model) networkDescription() []string {
	return factProse(factAreaFacts(m.networkAreas()))
}

// networkAreas is the diagnosis's detail as the areas of whichever state the screen
// is in: the destination being typed, the run's own answer, the step under the
// cursor, or the report exactly as it would be saved.
//
// Its bodies were paragraphs that mixed the fact with its justification in one value
// ("The latency reported here is the TCP handshake's, never a ping's: a network that
// drops ICMP is diagnosed exactly like one that answers it."), which is a row and a
// sentence. The row carries the fact and the sentence is dropped where it only
// restated it; what cannot become a row is the saved report itself, which is a file's
// own lines and keeps them (below).
func (m Model) networkAreas() []factArea {
	switch {
	case m.Network.InputActive:
		return m.networkInputAreas()
	case m.Network.Preview:
		return m.networkPreviewAreas()
	case len(m.networkChecks()) == 0:
		return m.networkIntroAreas()
	}
	if check, ok := m.selectedNetworkCheck(); ok {
		return networkCheckAreas(check)
	}
	return m.networkOverviewAreas()
}

// networkInputDescription is the field: what to type, what is typed, the reason
// the last attempt was refused when there is one, and what Enter does.
func (m Model) networkInputDescription() []string {
	return factProse(factAreaFacts(m.networkInputAreas()))
}

// networkInputAreas is the field as rows: what it accepts, the value in it now, and
// the two keys that act on it.
func (m Model) networkInputAreas() []factArea {
	target := "▏"
	if m.Network.Input != "" {
		target = m.Network.Input + "▏"
	}
	rows := []iconFact{
		{Icon: wslFactIconAction, Label: "Target", Value: target},
		{Icon: wslFactIconEvidence, Label: "Accepts",
			Value: "a host, an IPv4 or IPv6 address, host:port, an internationalized name, or an http/https URL"},
		{Icon: wslFactIconWhy, Label: "Validated", Value: "here first: a destination that cannot be diagnosed is refused"},
	}
	if m.Network.InputErr != "" {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Refused", Value: factClause(m.Network.InputErr)})
	}
	return append(rowsAreas("The destination", rows),
		factArea{Name: "The keys", Facts: []iconFact{
			{Icon: wslFactIconBenefit, Label: "Enter", Value: "diagnoses it"},
			{Icon: wslFactIconReadOnly, Label: "Escape", Value: "closes the field"},
			{Icon: wslFactIconLimit, Label: "No port", Value: "checked on 443, and the report says the port was assumed"},
			{Icon: wslFactIconReadOnly, Label: "Credentials", Value: "in a URL are never sent and never shown"},
		}},
	)
}

// rowsAreas wraps one block of rows in the single area its name gives, which is how a
// state with one section is built without a second shape.
func rowsAreas(name string, rows []iconFact) []factArea {
	return []factArea{{Name: name, Facts: rows}}
}

// networkPreviewDescription is the report as it would be saved, line for line,
// with the file it would be written to and the anonymization in force. Nothing
// is written from this state: the save row does that, and it is offered only
// after this preview has been shown.
func (m Model) networkPreviewDescription() []string {
	return factProse(factAreaFacts(m.networkPreviewAreas()))
}

// networkPreviewAreas is the preview as rows: what redaction is always applied, the
// anonymization in force, where it would be saved -- and then the report itself.
//
// The report is the one value here that cannot become rows: it is a file's own lines,
// exactly as it would be written, and wrapping them into labelled facts would join
// lines that mean different things (a check's evidence, its cause and its next step
// all live in one check's block). It is one labelled block, drawn line by line by
// factLines' block path.
func (m Model) networkPreviewAreas() []factArea {
	rows := []iconFact{
		{Icon: wslFactIconReadOnly, Label: "Redacted",
			Value: "credentials never appear, and a request's query string is not part of it"},
	}
	switch {
	case m.Network.Anonymize:
		rows = append(rows, iconFact{Icon: wslFactIconReadOnly, Label: "Anonymized",
			Value: "addresses, user name, home directory and machine name replaced; the host diagnosed is kept"})
	case m.Network.ReportPath != "":
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Anonymized",
			Value: "off: the report names this machine's addresses and interfaces"})
	}
	if m.Network.ReportPath != "" {
		rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "Saved to", Value: m.Network.ReportPath})
	} else {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Saved to",
			Value: "nowhere: there is no state directory here, so it is shown and not written"})
	}

	areas := rowsAreas("The file", rows)
	text := strings.TrimRight(m.Network.ReportText, "\n")
	if strings.TrimSpace(text) == "" {
		return append(areas, factArea{Name: "The report", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Report", Value: "there is no report to show yet"},
		}})
	}
	return append(areas, factArea{Name: "The report", Facts: []iconFact{
		{Icon: wslFactIconEvidence, Label: "Report", Value: text},
	}})
}

// networkIntroDescription is what the screen says before it has run anything: the
// stages it separates, what it reaches for, and the two promises -- it changes
// nothing, and every check is bounded.
func (m Model) networkIntroDescription() []string {
	return factProse(factAreaFacts(m.networkIntroAreas()))
}

// networkIntroAreas is the screen before a run as rows: the stages it separates, what
// the checks reach for, and the promises they carry -- one row per promise, with the
// bounds read from the engine's own defaults rather than restated here.
func (m Model) networkIntroAreas() []factArea {
	bounds := netdiag.DefaultBounds()

	stages := []iconFact{
		{Icon: wslFactIconResolver, Label: "Stages",
			Value: "local network, resolver, transport, certificate, application"},
		{Icon: wslFactIconWhy, Label: "Verdict",
			Value: "names the stage that stopped it, or says there is not enough evidence to name one"},
	}
	if destination := networkDestination(); !destination.IsZero() {
		stages = append(stages,
			iconFact{Icon: wslFactIconAction, Label: "Reaches for", Value: destination.Endpoint().Describe()},
			iconFact{Icon: wslFactIconEvidence, Label: "Source", Value: destination.Source},
			iconFact{Icon: wslFactIconReadOnly, Label: "Nothing else", Value: "no other address leaves this machine"},
		)
	}

	return []factArea{
		{Name: "What it separates", Facts: stages},
		{Name: "What it promises", Facts: []iconFact{
			{Icon: wslFactIconReadOnly, Label: "Writes",
				Value: "no resolver, route, firewall, proxy or certificate setting; no verification skipped; no tool installed"},
			{Icon: wslFactIconLimit, Label: "Bounded",
				Value: fmt.Sprintf("%s to resolve, %s to connect, %s for a request, %s for TLS",
					netdiag.Duration(bounds.DNS), netdiag.Duration(bounds.TCP),
					netdiag.Duration(bounds.HTTP), netdiag.Duration(bounds.TLS))},
			{Icon: wslFactIconBenefit, Label: "Cancellable", Value: "yes, and the checks run one at a time"},
			{Icon: wslFactIconShell, Label: "Latency", Value: "the TCP handshake's, never a ping's"},
		}},
	}
}

// networkOverviewDescription is the answer: the verdict, the counts, and -- when
// a stage failed -- what the failing check saw and what to try next. The rows
// above carry the details of every step, so this body stays the summary a reader
// can act on without scrolling.
func (m Model) networkOverviewDescription() []string {
	return factProse(factAreaFacts(m.networkOverviewAreas()))
}

// networkOverviewAreas is the answer as rows: the verdict and the counts, the
// destination, and where the run stopped with what to try next.
func (m Model) networkOverviewAreas() []factArea {
	report := m.Network.Report
	if report == nil {
		// A run in flight has no report yet, and the screen says so instead of
		// showing an empty answer.
		return []factArea{{Name: "Running", Facts: []iconFact{
			{Icon: wslFactIconBenefit, Label: "Running", Value: "each check is bounded, so the run ends by itself"},
			{Icon: wslFactIconFolder, Label: "Rows below", Value: "the steps that have finished"},
		}}}
	}

	summary := report.Summary()
	verdict := []iconFact{
		{Icon: wslFactIconBenefit, Label: "Verdict", Value: factClause(networkSentence(report.Verdict()))},
		{Icon: wslFactIconEvidence, Label: "Checks",
			Value: fmt.Sprintf("%d checks: %d ok, %d noted, %d failed, %d undetermined, %d skipped",
				summary.Total(), summary.OK, summary.Noted, summary.Failed, summary.Unknown, summary.Skipped)},
	}
	if report.Elapsed > 0 {
		verdict = append(verdict, iconFact{Icon: wslFactIconShell, Label: "Took", Value: netdiag.Duration(report.Elapsed)})
	}
	if report.Endpoint != "" {
		source := report.Source
		if source == "" {
			source = "you entered it"
		}
		verdict = append(verdict,
			iconFact{Icon: wslFactIconAction, Label: "Destination", Value: report.Endpoint},
			iconFact{Icon: wslFactIconEvidence, Label: "From", Value: source},
		)
	}

	areas := []factArea{{Name: "The answer", Facts: verdict}}
	if fault, ok := report.FaultCheck(); ok {
		stopped := []iconFact{
			{Icon: wslFactIconLimit, Label: "Stopped at", Value: fault.Stage.Title() + " — " + fault.Title},
		}
		if fault.Cause != "" {
			stopped = append(stopped, iconFact{Icon: wslFactIconWhy, Label: "Cause", Value: factClause(networkSentence(fault.Cause))})
		}
		if fault.Next != "" {
			stopped = append(stopped, iconFact{Icon: wslFactIconAction, Label: "Next check", Value: factClause(networkSentence(fault.Next))})
		}
		if fault.Manual != "" {
			stopped = append(stopped, iconFact{Icon: wslFactIconShell, Label: "By hand", Value: fault.Manual})
		}
		areas = append(areas, factArea{Name: "Where it stopped", Facts: stopped})
	}

	return append(areas, factArea{Name: "Reading a step", Facts: []iconFact{
		{Icon: wslFactIconFolder, Label: "Cursor", Value: "move it onto a step to read everything that step measured"},
	}})
}

// networkCheckParagraphs is one step in full: what was asked, what answered, how
// long it took, and -- when it did not answer cleanly -- the plausible cause, the
// next check and the command to run by hand. It is the same content the saved
// report carries for that step.
func networkCheckParagraphs(check netdiag.Check) []string {
	return factProse(factAreaFacts(networkCheckAreas(check)))
}

// networkCheckAreas is one step as rows: the stage and the question, the status, how
// long it took, each line of evidence on its own row, and what to try next. The lines
// the probes recorded are one row each rather than one paragraph, because a step's
// evidence is a list of readings and not a sentence.
func networkCheckAreas(check netdiag.Check) []factArea {
	rows := []iconFact{
		{Icon: wslFactIconFinding, Label: "Step", Value: check.Stage.Title() + " — " + check.Title},
		{Icon: wslFactIconBenefit, Label: "Status", Value: check.Status.String()},
	}
	if check.Duration > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconShell, Label: "Took", Value: netdiag.Duration(check.Duration)})
	}
	if len(check.Evidence) == 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Evidence", Value: "none was recorded for this step"})
	}
	for _, line := range check.Evidence {
		rows = append(rows, iconFact{Icon: wslFactIconEvidence, Label: "Evidence", Value: line})
	}

	areas := rowsAreas("The step", rows)
	var next []iconFact
	if check.Cause != "" {
		next = append(next, iconFact{Icon: wslFactIconWhy, Label: "Cause", Value: factClause(networkSentence(check.Cause))})
	}
	if check.Next != "" {
		next = append(next, iconFact{Icon: wslFactIconAction, Label: "Next check", Value: factClause(networkSentence(check.Next))})
	}
	if check.Manual != "" {
		next = append(next, iconFact{Icon: wslFactIconShell, Label: "By hand", Value: check.Manual})
	}
	if len(next) > 0 {
		areas = append(areas, factArea{Name: "What to try", Facts: next})
	}
	return areas
}

// networkElapsedClause is how long the run took, or nothing when it took no
// measurable time. It is a clause rather than a sentence so the counts line reads
// as one.
func networkElapsedClause(elapsed time.Duration) string {
	if elapsed <= 0 {
		return ""
	}
	return " in " + netdiag.Duration(elapsed)
}

// networkSentence ends a line with a full stop unless it already carries one, so
// a step's own wording ("skipped: nothing to dial") and a cause the probe wrote
// both read as sentences without a doubled period.
func networkSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasSuffix(text, ".") || strings.HasSuffix(text, "…") || strings.HasSuffix(text, "!") {
		return text
	}
	return text + "."
}

// ============================================================================
// Keys
// ============================================================================

// handleNetworkKeys drives the diagnosis screen. It changes the screen, the
// cursor, the two values and the state of the run, and nothing else: every check
// runs behind a command, because a keypress must not wait on a network.
//
// Every key is the key its row carries: Enter opens (the destination field and the
// report text, which are the two rows that lead anywhere), `r` runs the checks, `s`
// writes the reviewed report, `t` and `a` turn the traceroute and the anonymization
// over, and Escape goes back or stops the run. A command is never Enter and Enter is
// never a command (odd/tasks/key-grammar.md). Space runs nothing here: the screen has
// no switches, and the leader key is what it opens on this screen.
func (m Model) handleNetworkKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter":
		// Enter opens a row's own thing and nothing else: the field a destination is
		// typed into, and the report as text. The arrow in front of those two rows is
		// the mark that says so, and every other row ignores the key.
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case networkTargetRow:
			m.Network.InputActive = true
			m.Network.InputErr = ""
			m.Network.Notice = ""
		case networkPreviewRow:
			m.Network.Preview = true
			m.Network.ReportText = m.networkReportText()
			m.Network.Notice = ""
		}
	case networkRunKey:
		// The letter runs the row, so it is honoured only while the row is on screen.
		if !slices.Contains(options, networkRunRow) {
			return m, nil
		}
		return m.startNetworkRun()
	case networkSaveKey:
		// The write's row exists only once the report has been read and there is a
		// file to write, so the letter writes under the same two conditions.
		if !slices.Contains(options, networkSaveRow) {
			return m, nil
		}
		m.saveNetworkReport()
	case networkTraceKey:
		if !slices.Contains(options, m.networkTraceRow()) {
			return m, nil
		}
		m.Network.Trace = !m.Network.Trace
		m.Network.Notice = networkTraceNotice(m.Network.Trace)
	case networkAnonymizeKey:
		// The anonymization belongs to the preview: the row exists only there, so the
		// letter turns it over only there.
		if !slices.Contains(options, m.networkAnonymizeRow()) {
			return m, nil
		}
		m.Network.Anonymize = !m.Network.Anonymize
		// The text is rebuilt rather than drawn from a flag: what the preview shows
		// and what the write saves stay one string.
		m.Network.ReportText = m.networkReportText()
	}

	return m, nil
}

// updateNetworkInput owns every key while the destination field is open. It is
// taken before the global meanings of the keys -- space opens the leader key,
// backspace means "back", Esc leaves the screen -- because a field that loses a
// character to a navigation key is not a field.
func (m Model) updateNetworkInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			// A space and a control character are not accepted: a destination
			// contains neither, and swallowing the key silently would look like the
			// terminal lost it.
			if r == ' ' || r < 0x20 {
				continue
			}
			if len([]rune(m.Network.Input)) >= networkInputMax {
				break
			}
			m.Network.Input += string(r)
		}
		m.Network.InputErr = ""
		return m, nil

	case tea.KeySpace:
		return m, nil

	case tea.KeyBackspace:
		if runes := []rune(m.Network.Input); len(runes) > 0 {
			m.Network.Input = string(runes[:len(runes)-1])
		}
		m.Network.InputErr = ""
		return m, nil

	case tea.KeyEnter:
		return m.startNetworkRun()

	case tea.KeyEsc:
		// Closing the field keeps what was typed, so a destination can be corrected
		// rather than retyped.
		m.Network.InputActive = false
		m.Network.InputErr = ""
		return m, nil
	}

	return m, nil
}

// leaveNetworkScreen steps back to the section that opened the screen, with the
// two states that own Escape handled first: a run in flight is stopped, and an
// open preview is closed. Both keep the report.
func (m Model) leaveNetworkScreen() (tea.Model, tea.Cmd) {
	switch {
	case m.Network.Running:
		m.cancelNetworkRun()
		m.Network.Notice = "Stopping the run; the checks that finished are kept."
		return m, nil
	case m.Network.Preview:
		m.Network.Preview = false
		return m, nil
	}
	m.Network.Notice = ""
	m.Screen = ScreenUtilities
	m.Cursor = 0
	return m, nil
}

// networkTraceNotice is what the screen says when the traceroute is turned on or
// off. The text names the tool the machine must already have and the one thing a
// traceroute is often read wrongly as saying.
func networkTraceNotice(on bool) string {
	if !on {
		return "The traceroute is off; the checks stay inside this utility."
	}
	return "The traceroute will use the system's own tracepath or traceroute when it has one, and only for a " +
		"destination you typed. A hop that answers nothing is filtered, not a fault."
}

// ============================================================================
// The export
// ============================================================================

// networkReportPath is the file a report would be written to: the installer's own
// state directory, one file per run, named for the run's start. An empty state
// directory yields no path, and the screen then says the report can be read but
// not written instead of failing at the press.
func networkReportPath(now time.Time) string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, networkReportPrefix+now.UTC().Format("20060102T150405Z")+".txt")
}

// networkReportText is the report as text, with the anonymization the screen
// currently shows. It is the builder behind ReportText, and it is called from the
// two key presses that change the text -- opening the preview and toggling the
// anonymization -- so the render path only ever reads the string it produced.
func (m Model) networkReportText() string {
	if m.Network.Report == nil {
		return ""
	}
	return m.Network.Report.Text(netdiag.TextOptions{Anonymize: m.Network.Anonymize})
}

// saveNetworkReport writes the reviewed report. It is only reachable from the
// preview state, so the preview is the consent the write runs under, and it
// creates its own directory under the installer's state home rather than writing
// anywhere the user keeps something.
func (m *Model) saveNetworkReport() {
	if m.Network.Report == nil {
		m.Network.Notice = "There is no report to save yet."
		return
	}
	if !m.Network.Preview {
		// The rule holds whether or not a caller offers the row: the write happens
		// only from the state that shows the text, so the file cannot differ from what
		// was read.
		m.Network.Notice = "Review the report before saving it."
		return
	}
	if m.Network.ReportPath == "" {
		m.Network.Notice = "There is no state directory here to save the report in."
		return
	}
	// The text the preview showed, never a freshly built one: the file cannot differ
	// from what was read, and nothing is built while the frame is being drawn.
	text := m.Network.ReportText
	if err := writeNetworkReport(m.Network.ReportPath, text); err != nil {
		m.Network.Notice = "The report could not be saved: " + err.Error()
		return
	}
	m.Network.Notice = fmt.Sprintf("Saved %d bytes to %s.", len(text), m.Network.ReportPath)
}

// writeNetworkReport writes the text and nothing else: no rotation, no history,
// no other file touched, and the file is created for the user alone.
func writeNetworkReport(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), stateDirMode); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), networkReportMode)
}

// networkPanelValue is what the Utilities panel says the diagnosis row holds. It
// reads the model and dials nothing: a panel is drawn on every keypress.
func networkPanelValue(state networkState) string {
	switch {
	case state.Running:
		return fmt.Sprintf("running (%d so far)", len(state.Checks))
	case state.Report != nil:
		if stage, ok := state.Report.StageFault(); ok {
			return stage.Title() + " did not answer"
		}
		return "every stage answered"
	default:
		return "not run yet"
	}
}
