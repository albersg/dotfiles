package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/portscan"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UTILITIES SECTION: WHAT HOLDS A PORT, AND WHETHER IT MAY BE FREED
// ============================================================================
//
// This is the utility that reads the kernel. "What is using port 3000?" is a
// question the machine answers with a bound socket and the process behind it,
// and the installer's job is to show that answer honestly: a port whose owner
// could not be read says so instead of reading as free, TCP LISTEN and bound
// UDP are different states, and a host/guest boundary is named rather than
// crossed.
//
// The reading itself lives in internal/portscan, behind an injectable Probe.
// Nothing here opens /proc, runs a command or sends a signal while drawing: the
// reading and the stop are commands off the update loop, and their results land
// on the model. That is what keeps the interface responsive on a host where a
// reader hangs, and it is what lets the tests exercise the destructive path
// against a double instead of a real process.

const (
	// portRefreshInterval is the period of the optional automatic refresh. Five
	// seconds is short enough that a port freed by hand stops looking busy, and
	// long enough that /proc is not read on every frame. The refresh preserves
	// the cursor and pauses on the stop screen and while a filter is typed.
	portRefreshInterval = 5 * time.Second

	// portStopGrace and portStopPoll are the bounds one stop runs under: the
	// process is given the grace period to exit after SIGTERM, and liveness is
	// asked on the poll interval inside it. They are stated here so the screen
	// can name them and the command cannot disagree with the sentence.
	portStopGrace = 5 * time.Second
	portStopPoll  = 150 * time.Millisecond

	// portDetailRowCap is the most rows the detail block may spend. The block is
	// the second thing the frame gives up, after the list has taken the rows it
	// needs: the list is the data the user is moving through and the detail is an
	// expansion of one of its rows.
	portDetailRowCap = 10

	// portManagerBodyFixed is the part of the manager's body that is not a list row
	// and not the detail or notice: the title, the scope line, the state line and
	// the blank above the list. It is the floor the list's budget is computed from,
	// so a change to the chrome moves the number here rather than silently eating a
	// list row at the 80x24 floor.
	portManagerBodyFixed = 4

	// portNoticeRowCap and portScopeRowCap bound the two prose blocks that are not
	// the list, so a long notice or a WSL scope sentence cannot push the list off
	// a short frame. portStateRowCap is the same floor for the filter and sort line,
	// which a narrow terminal wraps in two.
	portNoticeRowCap = 2
	portScopeRowCap  = 2
	portStateRowCap  = 2
)

// portManagerState is everything the manager draws. The reading half is filled
// only when the reading returns; the view half is the user's controls and lives
// across readings, because a filter, a sort and a cursor position are the
// user's, not the machine's.
type portManagerState struct {
	// Snapshot is the last reading, with its own time and scope. A render reads
	// it; a render never takes one.
	Snapshot portscan.Snapshot
	// Probe is the machine surface every command reads through. It is a field and
	// not a call to portscan.DefaultProbe, so a test can hand in a double and the
	// reading and the protection check cannot read two different machines.
	Probe portscan.Probe
	// Loading reports a reading in flight, Loaded that one has ever returned, and
	// LoadErr the reason one could not. A platform with no reader produces a
	// snapshot with the reason on it, so LoadErr stays for the failure a command
	// could not turn into a snapshot.
	Loading, Loaded bool
	LoadErr         string
	// Cursor is the row under the marker in the filtered list, and Scroll is
	// reserved for a screen that stores its own window top. The window here follows
	// the cursor, so Scroll stays zero and is kept for the struct's shape.
	Cursor, Scroll int
	// Filter is the text the filter field holds, Filtering says the field is being
	// typed, and Transport, Owner and Sort are the three cycles. The values are
	// the three-word vocabularies the screen prints, never free text.
	Filter    string
	Filtering bool
	Transport string
	Owner     string
	Sort      string
	// Detail says the selected row is expanded, and Auto says the periodic refresh
	// is on. Both are the user's choices and survive a reading.
	Detail bool
	Auto   bool
	// Notice is the line beside the list: a protection refusal, a copy's outcome,
	// or what changed between two readings.
	Notice string
	// SelectedKey is the identity of the row the cursor was on when a reading
	// started. It is what lets a refresh keep the cursor on the same socket even
	// when a reader reports the rows in a different order, and the counts are what
	// the notice reports: how many rows are new and how many are gone since the
	// reading before.
	SelectedKey string
	NewSince    int
	GoneSince   int
}

// portStopState is the stop confirmation. Target is the one process the user
// chose with every port the reading attributes to it, Refusal is why the screen
// offers no signal at all, Running says an attempt is in flight, and Result is
// the whole account of the last attempt.
type portStopState struct {
	Target  portscan.StopTarget
	Refusal string
	Running bool
	Result  *portscan.StopResult
	Forced  bool
}

// The messages the two commands send back. Each carries the value the package
// produced, so the handler stores a fact rather than re-deriving one.
type (
	// portsLoadedMsg carries one reading. Discover never returns an error: a
	// platform that cannot be read is a snapshot whose Limits say so.
	portsLoadedMsg struct{ snap portscan.Snapshot }
	// portsStopDoneMsg carries the account of one stop attempt.
	portsStopDoneMsg struct{ result portscan.StopResult }
	// portsTickMsg is one period of the optional automatic refresh.
	portsTickMsg time.Time
)

// portScanCmd reads the machine off the update loop. It reads through the model's
// probe, never a default fetched at call time, so the double a test injects is
// the surface the command reads.
func (m Model) portScanCmd() tea.Cmd {
	probe := m.Ports.Probe
	return func() tea.Msg {
		return portsLoadedMsg{snap: portscan.Discover(context.Background(), probe)}
	}
}

// portStopCmd sends one signal to the one process of the target, after the
// package's own revalidation, and returns the account of the attempt. The delay
// bounds are passed explicitly so the sentence on screen names the same bounds.
func (m Model) portStopCmd(sig portscan.Signal) tea.Cmd {
	probe := m.Ports.Probe
	target := m.PortStop.Target
	return func() tea.Msg {
		result := portscan.Stop(context.Background(), probe, target, portscan.StopOptions{
			Signal: sig,
			Grace:  portStopGrace,
			Poll:   portStopPoll,
		})
		return portsStopDoneMsg{result: result}
	}
}

// portsTickCmd arms one period of the automatic refresh. It is armed by the `p`
// key and re-armed by its own message while the refresh is still on.
func portsTickCmd() tea.Cmd {
	return tea.Tick(portRefreshInterval, func(t time.Time) tea.Msg {
		return portsTickMsg(t)
	})
}

// portVisibleListeners is the reading narrowed by the filter field and the two
// transport and owner cycles, then sorted. It reads the model only: evaluating
// it during a render costs a sort and touches no machine.
func (m Model) portVisibleListeners() []portscan.Listener {
	filter := strings.ToLower(strings.TrimSpace(m.Ports.Filter))
	var out []portscan.Listener
	for _, l := range m.Ports.Snapshot.Listeners {
		if !m.portTransportMatches(l) {
			continue
		}
		if !m.portOwnerMatches(l) {
			continue
		}
		if filter != "" && !portListenerMatches(l, filter) {
			continue
		}
		out = append(out, l)
	}
	m.portSortListeners(out)
	return out
}

// portListenerMatches reports whether a row contains the filter text. The text is
// matched as a case-insensitive substring of every column a reader recognises:
// the port, the protocol, the process name, the user and the address. Matching
// the fields and not the rendered row means a filter for "node" finds a row whose
// process is named node even when the owner column is laid out differently.
func portListenerMatches(l portscan.Listener, filter string) bool {
	columns := []string{
		strconv.Itoa(l.Port),
		string(l.Protocol),
		l.Address,
	}
	if l.Owner != nil {
		columns = append(columns, l.Owner.Name, l.Owner.User)
	}
	for _, column := range columns {
		if column != "" && strings.Contains(strings.ToLower(column), filter) {
			return true
		}
	}
	return false
}

// portTransportMatches applies the transport cycle. A TCP socket is anything
// whose protocol reads as TCP, which keeps the IPv6 variants beside the IPv4
// ones instead of hiding them behind a family filter.
func (m Model) portTransportMatches(l portscan.Listener) bool {
	switch m.Ports.Transport {
	case "tcp":
		return l.Protocol.Transport() == "TCP"
	case "udp":
		return l.Protocol.Transport() == "UDP"
	}
	return true
}

// portOwnerMatches applies the owner cycle. The three answers are "all", "me"
// and "others", and "me" is decided by account name, never by PID: see
// portMeAccount.
func (m Model) portOwnerMatches(l portscan.Listener) bool {
	switch m.Ports.Owner {
	case "me":
		return m.portOwnerIsMe(l)
	case "others":
		return !m.portOwnerIsMe(l)
	}
	return true
}

// portOwnerIsMe reports whether a row belongs to the account this run is running
// as. The Probe exposes LookupUser(uid) but no current uid, so the reading is the
// only evidence: a row whose process id is the installer's own carries the
// account this run belongs to, and that name is the one compared. The PID is used
// only to find the name, never to classify a row: a row is "mine" because its
// account matches, and a stranger's recycled PID is not made mine by sharing a
// number.
//
// When no row names the installer's own process there is no account to compare.
// "me" then keeps the local-scope rows whose account the reader could not name at
// all -- a missing name is not demonstrably another account -- and every named
// row counts as another account's.
func (m Model) portOwnerIsMe(l portscan.Listener) bool {
	if me := m.portMeAccount(); me != "" {
		return l.Owner != nil && l.Owner.User == me
	}
	return l.Owner != nil && l.Owner.User == "" && l.Owner.Scope == portscan.ScopeSelf
}

// portMeAccount resolves the account this run belongs to from the reading alone.
// It is the account on the row owned by the installer's own process, or the empty
// string when the reading names no such row.
func (m Model) portMeAccount() string {
	if m.Ports.Probe == nil {
		return ""
	}
	self := m.Ports.Probe.SelfPID()
	for _, l := range m.Ports.Snapshot.Listeners {
		if l.Owner != nil && l.Owner.PID == self && l.Owner.User != "" {
			return l.Owner.User
		}
	}
	return ""
}

// portSortListeners sorts a copy of the reading in place by the sort cycle. The
// order is derived from the rows alone, so two readings of an unchanged machine
// list their rows identically whatever order the reader reported.
func (m Model) portSortListeners(list []portscan.Listener) {
	switch m.Ports.Sort {
	case "process":
		sort.SliceStable(list, func(i, j int) bool {
			left, right := portProcessSortKey(list[i]), portProcessSortKey(list[j])
			if left != right {
				return left < right
			}
			return list[i].Port < list[j].Port
		})
	case "family":
		sort.SliceStable(list, func(i, j int) bool {
			left, right := list[i].Protocol.Family(), list[j].Protocol.Family()
			if left != right {
				return left < right
			}
			return list[i].Port < list[j].Port
		})
	default: // "port": the port, then the protocol so IPv4 and IPv6 order is stable.
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Port != list[j].Port {
				return list[i].Port < list[j].Port
			}
			return list[i].Protocol < list[j].Protocol
		})
	}
}

// portProcessSortKey is the process name a row sorts by, lowercased so the order
// is not decided by case. A row whose owner is unknown sorts first, which is
// where an empty column belongs.
func portProcessSortKey(l portscan.Listener) string {
	if l.Owner != nil && l.Owner.Name != "" {
		return strings.ToLower(l.Owner.Name)
	}
	return ""
}

// portSelectedListener is the row under the cursor, or false when the filtered
// list is empty.
func (m Model) portSelectedListener() (portscan.Listener, bool) {
	visible := m.portVisibleListeners()
	if m.Ports.Cursor < 0 || m.Ports.Cursor >= len(visible) {
		return portscan.Listener{}, false
	}
	return visible[m.Ports.Cursor], true
}

// portSelectedKey is the identity of the row under the cursor, or the empty
// string when the filtered list is empty. It is captured before a reading starts
// and compared after it lands, which is how the cursor survives a refresh.
func (m Model) portSelectedKey() string {
	l, ok := m.portSelectedListener()
	if !ok {
		return ""
	}
	return l.Key()
}

// portManagerRows is the filtered list as the frame draws it: one line per row,
// the port first. The list a render draws and the list the cursor walks are the
// same function, so the two cannot disagree about what is on screen.
func (m Model) portManagerRows() []string {
	visible := m.portVisibleListeners()
	rows := make([]string, 0, len(visible))
	for _, l := range visible {
		rows = append(rows, m.portManagerRow(l))
	}
	return rows
}

// portManagerRow renders one socket as one line. The port is first because it is
// the question the utility exists to answer; the protocol and state follow so a
// TCP listener and a bound UDP socket never read the same; the address names the
// interface the bind covers; and the owner is named with its PID and account, or
// says plainly that it could not be read.
//
// The conflict marks are words rather than symbols: "loopback only" and "all
// interfaces" are the S4 difference between a service reachable from the network
// and one reachable only from this machine, and "shared" marks a port bound by
// more than one row.
func (m Model) portManagerRow(l portscan.Listener) string {
	parts := []string{
		":" + strconv.Itoa(l.Port),
		string(l.Protocol),
		string(l.State),
		l.Address,
		portOwnerLabel(l),
	}
	switch {
	case l.Loopback():
		parts = append(parts, "loopback only")
	case l.Wildcard():
		parts = append(parts, "all interfaces")
	}
	if m.portShared(l) {
		parts = append(parts, "shared")
	}
	return strings.Join(parts, "  ")
}

// portOwnerLabel is the owner column in the screen's own words: the name, the
// PID and the account when the reader could resolve them, and the honest
// "owner not readable" when it could not. A row whose owner is unknown never
// reads as a free port.
func portOwnerLabel(l portscan.Listener) string {
	if l.Owner == nil {
		if l.OwnerErr != "" {
			return "owner not readable"
		}
		return "owner not readable"
	}
	name := l.Owner.Name
	if name == "" {
		name = "pid " + strconv.Itoa(l.Owner.PID)
	} else if l.Owner.PID > 0 {
		name = fmt.Sprintf("%s (pid %d", name, l.Owner.PID)
		if l.Owner.User != "" {
			name += ", " + l.Owner.User
		}
		name += ")"
	}
	if l.Owner.User == "" && !strings.Contains(name, "pid ") {
		name += " (owner account not readable)"
	}
	return name
}

// portShared reports whether a port is bound by more than one row of the reading,
// which is what SO_REUSEPORT and a second bind look like from here. The count is
// over the same port and transport: two different transports on one number are
// two services, not a conflict.
func (m Model) portShared(l portscan.Listener) bool {
	count := 0
	for _, other := range m.Ports.Snapshot.Listeners {
		if other.Port == l.Port && other.Protocol == l.Protocol {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}

// portStopTarget builds the confirmation target from the row under the cursor.
// Sockets is every port the reading attributes to the process, not only the
// selected one: the confirmation has to list everything a stop releases, and a
// stop that only knows about the socket it was opened from would understate what
// it is about to do.
func (m Model) portStopTarget() (portscan.StopTarget, bool) {
	l, ok := m.portSelectedListener()
	if !ok || l.Owner == nil {
		return portscan.StopTarget{}, false
	}
	proc := *l.Owner
	return portscan.StopTarget{
		Process: proc,
		Sockets: m.Ports.Snapshot.ListenersFor(proc.PID),
	}, true
}

// portStopForcedOffered reports whether the forced signal is on offer. It is a
// second, separate action and only after a soft attempt left the process running:
// never the tail of the soft one, and never before one was tried.
func (m Model) portStopForcedOffered() bool {
	r := m.PortStop.Result
	return r != nil && r.Outcome == portscan.OutcomeStillRunning && r.Signal == portscan.SignalTerminate
}

// portStopRows is the stop screen's action rows. The screen draws them under the
// account of what the stop would do, and each one carries the letter that runs
// it: a command says how it is pressed on its own row. Exactly one of the two is
// offered at a time -- the forced one only after a soft attempt left the process
// running -- and the way back is the same row every screen in the section draws.
func (m Model) portStopRows() []string {
	if m.PortStop.Refusal != "" {
		return []string{portBackRow}
	}
	rows := []string{}
	if m.portStopForcedOffered() {
		rows = append(rows, portHintForce.keys+" Force the stop (SIGKILL)")
	} else if r := m.PortStop.Result; r == nil || r.Outcome == portscan.OutcomeStillRunning {
		rows = append(rows, portHintTerminate.keys+" Stop it gently (SIGTERM)")
	}
	return append(rows, portBackRow)
}

// portBackRow is the way back, spelled the one way the section spells it.
const portBackRow = "← Back"

// The stop confirmation's two actions and the letters they run on. The gentle
// stop keeps the t of SIGTERM, and the forced one is the f of "force": j and k
// belong to the cursor, so no stop may run on either of them. They are declared
// here rather than beside the other hints because they are this screen's own two
// answers, and the grammar guard reads them as its own hints.
var (
	portHintTerminate = installerHint{"[t]", "stop gently"}
	portHintForce     = installerHint{"[f]", "force stop"}
)

// portDiagnosticText is the reproducible account of the selected row, built from
// redacted data only. The arguments the reading carries are already redacted by
// the package, so a token on the command line cannot reach the clipboard through
// this text.
func (m Model) portDiagnosticText() string {
	l, ok := m.portSelectedListener()
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "port %d/%s %s %s\n", l.Port, l.Protocol, l.State, l.Address)
	fmt.Fprintf(&b, "scope %s\n", m.portScope())
	fmt.Fprintf(&b, "source %s\n", l.Source)
	if l.Owner == nil {
		fmt.Fprintf(&b, "owner not readable: %s\n", l.OwnerErr)
		return b.String()
	}
	owner := *l.Owner
	fmt.Fprintf(&b, "owner %s pid %d user %s\n", owner.Name, owner.PID, owner.User)
	fmt.Fprintf(&b, "exe %s\n", owner.Exe)
	fmt.Fprintf(&b, "args %s\n", portscan.RedactArgs(owner.Args))
	fmt.Fprintf(&b, "started %s\n", owner.StartedText())
	return b.String()
}

// copyPortDiagnosticCmd writes the diagnostic to the terminal's clipboard over
// OSC 52, without a clipboard helper. The terminal is the only thing that can
// take it; when it is not known to support OSC 52 the screen keeps the text on
// screen instead of claiming a copy nobody received.
func copyPortDiagnosticCmd(text string) tea.Cmd {
	return tea.Printf("%s", "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(text))+"\x07")
}

// portScope is the scope name the screen prints, falling back to the local one
// when a reading has not landed yet.
func (m Model) portScope() string {
	if m.Ports.Snapshot.Scope != "" {
		return m.Ports.Snapshot.Scope
	}
	return portscan.ScopeSelf
}

// portScopeSentence is the one line at the top of the manager that says which
// kernel the rows belong to. In WSL it says plainly that the Windows host's own
// ports and processes are not visible from here and cannot be stopped from here,
// so no row can be mistaken for a host process.
func (m Model) portScopeSentence() string {
	if m.portScope() == portscan.ScopeWSLGuest {
		return "Scope: WSL guest. This is the distro's own kernel: the Windows host's ports and processes are not " +
			"visible from here and cannot be stopped from here."
	}
	return "Scope: this machine. Every row is a socket this kernel holds."
}

// portStateLine is the filter, sort and refresh state in one line, so the user
// can see why the list is what it is without opening anything. It also names the
// reading's time and whether one is in flight.
func (m Model) portStateLine() string {
	read := "not read yet"
	switch {
	case m.Ports.Loading:
		read = "reading now"
	case !m.Ports.Snapshot.At.IsZero():
		read = "read at " + m.Ports.Snapshot.At.Format("15:04:05")
	}
	filter := "none"
	if strings.TrimSpace(m.Ports.Filter) != "" {
		filter = "\"" + m.Ports.Filter + "\""
	}
	auto := "off"
	if m.Ports.Auto {
		auto = "every " + portStopDuration(portRefreshInterval)
	}
	return fmt.Sprintf("Filter %s  Transport %s  Owner %s  Sort %s  Auto %s  %s",
		filter, m.Ports.Transport, m.Ports.Owner, m.Ports.Sort, auto, read)
}

// portStopDuration renders a duration the way the sentences read: whole seconds.
func portStopDuration(d time.Duration) string {
	return fmt.Sprintf("%d s", int(d.Seconds()))
}

// portDetailLines is the expanded block for the row under the cursor: the whole
// socket, the owner's identity, the redacted arguments and start time, and the
// reasons the reading could not resolve something. It is written in the screen's
// own words and carries no secret.
func (m Model) portDetailLines() []string {
	l, ok := m.portSelectedListener()
	if !ok {
		return nil
	}

	lines := []string{
		fmt.Sprintf("Selected: :%d %s %s %s", l.Port, l.Protocol, l.State, l.Address),
		fmt.Sprintf("Family: %s  Port: %d  State: %s  Source: %s", l.Protocol.Family(), l.Port, l.State, l.Source),
	}

	if l.Owner == nil {
		reason := l.OwnerErr
		if reason == "" {
			reason = "no process this run can see holds this socket."
		}
		lines = append(lines, "Owner: not readable. "+reason)
		lines = append(lines, "A port whose owner is unknown is not a free port.")
	} else {
		owner := *l.Owner
		lines = append(lines, fmt.Sprintf("Owner: %s (pid %d, user %s).", portOwnerName(owner), owner.PID, portUserLabel(owner)))
		if owner.Exe != "" {
			lines = append(lines, "Executable: "+owner.Exe)
		}
		if len(owner.Args) > 0 {
			lines = append(lines, "Arguments (redacted): "+strings.Join(owner.Args, " "))
		}
		if started := owner.StartedText(); started != "" {
			lines = append(lines, "Started: "+started)
		}
		if !owner.IdentityKnown() {
			lines = append(lines, "Identity cannot be revalidated on this platform; a stop checks liveness only.")
		}
	}

	// The reading's own declared limits are part of the answer, so they are shown
	// with the row they qualify rather than left for the user to infer from a gap.
	for _, limit := range m.Ports.Snapshot.Limits {
		lines = append(lines, "Limitation: "+limit)
	}
	return lines
}

// portOwnerName is a process's name or its bare PID when it has none.
func portOwnerName(proc portscan.Process) string {
	if proc.Name != "" {
		return proc.Name
	}
	return "pid " + strconv.Itoa(proc.PID)
}

// portUserLabel is the account or the honest gap.
func portUserLabel(proc portscan.Process) string {
	if proc.User != "" {
		return proc.User
	}
	return "not readable"
}

// portManagerHints is the manager's footer. The keys in it are exactly the keys
// the handler honours: the cursor walks the reading on ↑/k and ↓/j, Enter opens
// the selected row's detail, and every command the screen has runs on its own
// letter. The rows are the machine's data and not switches, so Space is not
// offered here.
func (m Model) portManagerHints() []installerHint {
	if m.Ports.Filtering {
		return []installerHint{hintFilterTyping, hintBack}
	}
	return []installerHint{hintUp, hintDown, hintOpen, hintFilter, hintCycle, hintRefresh, hintAuto, hintStop, hintCopy, hintBack}
}

// portStopHints is the stop screen's footer. A refusal offers only the way back;
// a soft stop in progress offers nothing; a completed soft stop that left the
// process running offers the forced action as its own row and its own letter.
func (m Model) portStopHints() []installerHint {
	if m.PortStop.Refusal != "" {
		return []installerHint{hintBack}
	}
	if m.PortStop.Running {
		return []installerHint{hintBack}
	}
	switch {
	case m.portStopForcedOffered():
		return []installerHint{portHintForce, hintBack}
	case m.PortStop.Result != nil && m.PortStop.Result.Outcome != portscan.OutcomeStillRunning:
		return []installerHint{hintBack}
	default:
		return []installerHint{portHintTerminate, hintBack}
	}
}

// renderPortManager draws the manager: the scope line, the filter and sort state,
// the windowed list of rows, the detail block for the selected row when it is
// open, and the notice. It reads the model and nothing else: no clock, no
// environment and no file.
func (m Model) renderPortManager() string {
	width := contentWidth(m)
	hints := m.portManagerHints()
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	title := BrandStyle.Render(m.GetScreenTitle())

	scope := capPortLines(wrapText(m.portScopeSentence(), width, 0), portScopeRowCap)
	state := capPortLines(wrapText(m.portStateLine(), width, 0), portStateRowCap)

	var notice []string
	if strings.TrimSpace(m.Ports.Notice) != "" {
		notice = capPortLines(wrapText(m.Ports.Notice, width, 0), portNoticeRowCap)
	}

	var detail []string
	if m.Ports.Detail {
		// The detail is prose, so it is wrapped to the same width as the rest of the
		// body before it is capped. A single long line would otherwise set the whole
		// frame's block width and pad every other row past the terminal edge.
		var wrapped []string
		for _, line := range m.portDetailLines() {
			wrapped = append(wrapped, wrapText(line, width, 0)...)
		}
		detail = capPortLines(wrapped, portDetailRowCap)
	}

	rows := m.portManagerRows()
	empty := len(rows) == 0
	if empty {
		// The list has one row of its own so the frame still says what the empty
		// list means instead of leaving a void under the header.
		rows = []string{MutedStyle.Render(portEmptyListLine(m.Ports.Loaded, m.Ports.Loading))}
	}

	// The fixed rows are the title, the scope, the state line and the blank above
	// the list. The detail and the notice are given up before the list, and the
	// scope before them, until at least one list row fits.
	fixed := portManagerBodyFixed
	if len(scope) > 1 {
		fixed += len(scope) - 1
	}
	if len(state) > 1 {
		fixed += len(state) - 1
	}
	for bodyRows-fixed-len(detail)-len(notice) < 1 {
		switch {
		case len(detail) > 0:
			detail = detail[:len(detail)-1]
		case len(notice) > 0:
			notice = notice[:len(notice)-1]
		case len(scope) > 1:
			scope = scope[:len(scope)-1]
		default:
		}
		if len(detail) == 0 && len(notice) == 0 && len(scope) <= 1 {
			break
		}
	}

	listBudget := bodyRows - fixed - len(detail) - len(notice)
	if listBudget < 1 {
		listBudget = 1
	}
	start, end := listWindow(m.Ports.Cursor, listBudget, len(rows))
	if empty {
		start, end = 0, 1
	}
	listLines := m.menuRows(rows, m.Ports.Cursor)[start:end]

	body := []string{title}
	for _, line := range scope {
		body = append(body, MutedStyle.Render(line))
	}
	for _, line := range state {
		body = append(body, MutedStyle.Render(line))
	}
	body = append(body, "")
	body = append(body, listLines...)
	for _, line := range detail {
		body = append(body, MutedStyle.Render(line))
	}
	for _, line := range notice {
		body = append(body, MutedStyle.Render(line))
	}

	vital := ""
	if !empty && end-start < len(rows) {
		vital = scrollVital("Showing", start+1, end, len(rows))
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// portEmptyListLine is what the empty list says. It distinguishes "not read yet"
// from "read and found nothing" from "filtered away", because the three mean
// different things to a person.
func portEmptyListLine(loaded, loading bool) string {
	switch {
	case loading:
		return "Reading the bound sockets on this machine…"
	case !loaded:
		return "The bound sockets have not been read yet."
	default:
		return "No bound socket matches the current filter."
	}
}

// renderPortStop draws the confirmation. Nothing is signalled while it draws: the
// paragraphs name the identity, the owner, every port the process holds and the
// consequences, the refusal when there is one, and the outcome once an attempt
// has landed -- and the rows under them name the one action on offer and the
// letter that runs it.
func (m Model) renderPortStop() string {
	width := contentWidth(m)
	hints := m.portStopHints()
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	title := BrandStyle.Render(m.GetScreenTitle())
	rows := m.portStopRows()
	// The title and the rows keep their place; the prose is what gives way on a
	// short frame, because the action is what the screen exists to offer.
	lines := descriptionLines(m.portStopParagraphs(), width, bodyRows-1-len(rows)-1)

	body := append([]string{title}, mutedLines(lines)...)
	body = append(body, "")
	body = append(body, rows...)
	return m.frame(m.headerName(), "", body, hints)
}

// portStopParagraphs is the whole confirmation in prose. Exactly one of the
// actions is offered at a time: the forced one only after a soft attempt left the
// process running, and nothing at all while a refusal stands.
func (m Model) portStopParagraphs() []string {
	target := m.PortStop.Target
	proc := target.Process

	paragraphs := []string{}
	if m.PortStop.Refusal != "" {
		paragraphs = append(paragraphs, "This stop is refused before anything is sent: "+m.PortStop.Refusal)
		paragraphs = append(paragraphs, "No signal is offered here. The port is still held by whoever the reading found.")
		return append(paragraphs, portStopPortsSentence(target))
	}

	paragraphs = append(paragraphs, fmt.Sprintf("Process: %s, pid %d, owned by %s.", portOwnerName(proc), proc.PID, portUserLabel(proc)))
	if proc.Exe != "" {
		paragraphs = append(paragraphs, "Executable: "+proc.Exe)
	}
	if len(proc.Args) > 0 {
		paragraphs = append(paragraphs, "Arguments (redacted): "+strings.Join(proc.Args, " "))
	}
	if started := proc.StartedText(); started != "" {
		paragraphs = append(paragraphs, "Started: "+started)
	}
	if !proc.IdentityKnown() {
		paragraphs = append(paragraphs, "This platform gives no identity token, so a stop revalidates liveness only; it "+
			"cannot tell this exact incarnation from a recycled PID.")
	}
	paragraphs = append(paragraphs, portStopPortsSentence(target))

	if m.PortStop.Running {
		paragraphs = append(paragraphs, "A signal is being sent and the machine is being re-read…")
		return paragraphs
	}

	if m.portStopForcedOffered() {
		paragraphs = append(paragraphs, "The soft stop was sent and the process is still running. The forced signal asks the "+
			"kernel to end it immediately: it cannot be caught, the process cannot clean up, and unsaved work is lost. It is "+
			"a separate action, and it is the only one offered here now.")
	} else if m.PortStop.Result == nil {
		paragraphs = append(paragraphs, fmt.Sprintf("The soft stop sends SIGTERM, which asks the process to exit cleanly, and "+
			"gives it %s to do so. This run then checks whether it exited and whether its ports are still bound; if it is "+
			"still running, a forced stop becomes a separate action.", portStopDuration(portStopGrace)))
	}

	if result := m.PortStop.Result; result != nil {
		paragraphs = append(paragraphs, "Outcome: "+result.Message)
		if result.Verified {
			paragraphs = append(paragraphs, "The machine was re-read after the attempt, so this outcome is what was observed, "+
				"not what was expected.")
		}
		if len(result.PortsStillHeld) > 0 && result.Outcome != portscan.OutcomeStopped {
			paragraphs = append(paragraphs, portStopHeldSentence(result.PortsStillHeld))
		}
	}

	return paragraphs
}

// portStopPortsSentence names every port the process holds, which is what the
// stop releases. The list comes from the reading that built the target, so the
// confirmation cannot promise less than the stop would do.
func portStopPortsSentence(target portscan.StopTarget) string {
	if len(target.Sockets) == 0 {
		return "The reading attributes no bound socket to this process any more; a stop would have nothing to release."
	}
	parts := make([]string, 0, len(target.Sockets))
	for _, l := range target.Sockets {
		parts = append(parts, fmt.Sprintf(":%d %s %s %s", l.Port, l.Protocol, l.State, l.Address))
	}
	label := "port"
	if len(parts) != 1 {
		label = "ports"
	}
	return fmt.Sprintf("Every %s this process holds, and which a stop releases: %s.", label, strings.Join(parts, ", "))
}

// portStopHeldSentence names the ports a fresh reading found still bound.
func portStopHeldSentence(held []portscan.Listener) string {
	parts := make([]string, 0, len(held))
	for _, l := range held {
		parts = append(parts, fmt.Sprintf(":%d %s", l.Port, l.Protocol))
	}
	return "A fresh reading still finds " + strings.Join(parts, ", ") + " bound, so the port is not free yet."
}

// mutedLines wraps lines in the muted style, so the two renderers spell a prose
// block once.
func mutedLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = MutedStyle.Render(line)
	}
	return out
}

// capPortLines cuts a wrapped block to at most rows lines, replacing the tail
// with a marker so the cut is declared rather than silent. The frame guards use
// the same rule the description budget uses.
func capPortLines(lines []string, rows int) []string {
	if rows < 1 {
		return nil
	}
	if len(lines) <= rows {
		return lines
	}
	hidden := len(lines) - rows + 1
	out := append([]string(nil), lines[:rows-1]...)
	return append(out, fmt.Sprintf("… and %d more", hidden))
}

// portManagerTitle and portStopTitle are the two titles, kept in one place so a
// page's name and the screen's name cannot drift. They are the same strings
// GetScreenTitle returns.
const (
	portManagerTitle = "Ports in use"
	portStopTitle    = "Stop a process"
)
