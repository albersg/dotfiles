package tui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/portscan"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE PORT MANAGER'S TESTS AND DOUBLES
// ============================================================================
//
// The reading and the stop both go through portscan.Probe, so the tests replace
// the machine with an in-memory one: a small /proc the reader parses exactly as
// it parses the real one, a liveness table and a signal recorder. Nothing here
// signals a process the test does not own, and nothing reads the runner's own
// machine.

// portTestDirEntry is one in-memory directory entry. Every entry the fake lists
// under /proc is a process directory, so IsDir is true; the entries under a
// process's fd directory are not inspected for type.
type portTestDirEntry struct{ name string }

func (e portTestDirEntry) Name() string               { return e.name }
func (e portTestDirEntry) IsDir() bool                { return true }
func (e portTestDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e portTestDirEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// portTestProbe is the machine surface the manager tests read. It is the same
// shape as the package's own fake, trimmed to what the TUI needs: /proc/net and
// the process directories for discovery, liveness for the stop, and a recorder
// for the signal so a test can assert that one was (or was not) sent.
type portTestProbe struct {
	goos    string
	wsl     bool
	selfPID int
	now     time.Time

	files   map[string][]byte
	dirs    map[string][]os.DirEntry
	links   map[string]string
	users   map[int]string
	alive   map[int]bool
	parents map[int]int

	signalErr error
	signals   []portTestSignal
}

type portTestSignal struct {
	pid int
	sig portscan.Signal
}

func (p *portTestProbe) Now() time.Time { return p.now }
func (p *portTestProbe) GOOS() string   { return p.goos }
func (p *portTestProbe) IsWSL() bool    { return p.wsl }
func (p *portTestProbe) SelfPID() int   { return p.selfPID }

func (p *portTestProbe) Parent(pid int) (int, bool) {
	parent, ok := p.parents[pid]
	return parent, ok
}

func (p *portTestProbe) ReadFile(name string) ([]byte, error) {
	if data, ok := p.files[name]; ok {
		return data, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (p *portTestProbe) ReadDir(name string) ([]os.DirEntry, error) {
	if entries, ok := p.dirs[name]; ok {
		return entries, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (p *portTestProbe) Readlink(name string) (string, error) {
	if target, ok := p.links[name]; ok {
		return target, nil
	}
	return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
}

// LookPath answers for ps and refuses every other tool. The darwin reader asks
// for ps before it reads anything, because that is where a macOS build gets its
// process facts; every other name belongs to the Linux reader's fallback path,
// where a test that leaves the tool out means it to be missing.
func (p *portTestProbe) LookPath(name string) (string, error) {
	if name == portTestPS {
		return portTestPS, nil
	}
	return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
}

// Run answers the two ps shapes the darwin reader asks for and nothing else.
// The process readers are chosen when the package is compiled -- a darwin build
// reads ps where a Linux build reads /proc -- so a fixture that carries only
// /proc files leaves every process unnamed on a macOS runner, which is what
// made five manager tests fail there while passing on Linux. Rendering ps from
// that same /proc state keeps one fixture and one set of facts for both
// dialects, exactly as the portscan package's own fake does.
func (p *portTestProbe) Run(ctx context.Context, name string, args []string, env []string) (portscan.CommandResult, error) {
	if name == portTestPS {
		if result, ok := p.runPS(args); ok {
			return result, nil
		}
	}
	return portscan.CommandResult{}, fmt.Errorf("%s: no scripted answer for %q", name, strings.Join(args, " "))
}

func (p *portTestProbe) Alive(pid int) (bool, error) { return p.alive[pid], nil }

func (p *portTestProbe) Signal(pid int, sig portscan.Signal) error {
	p.signals = append(p.signals, portTestSignal{pid: pid, sig: sig})
	return p.signalErr
}

func (p *portTestProbe) LookupUser(uid int) (string, error) {
	if name, ok := p.users[uid]; ok {
		return name, nil
	}
	return "", fmt.Errorf("user: unknown userid %d", uid)
}

// portTestNetHeader is the header line of a /proc/net table, which the parser
// skips.
const portTestNetHeader = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"

// portTestNetRow renders one /proc/net row in the kernel's own layout: the local
// address and port in hex, the state, the owning uid and the socket inode.
func portTestNetRow(hexAddress string, port int, state string, uid int, inode string) string {
	return fmt.Sprintf("   0: %s:%04X 00000000:0000 %s 00000000:00000000 00:00000000 00000000 %6d        0 %s 1 0000000000000000 100 0 0 10 0\n",
		hexAddress, port, state, uid, inode)
}

// portTestStatLine renders one /proc/<pid>/stat line with the start ticks in
// field 22, which is where the identity token comes from.
func portTestStatLine(pid, ppid int, comm string, startTicks uint64) string {
	fields := []string{
		"S", strconv.Itoa(ppid),
		"1", "1", "0", "1",
		"0", "0", "0", "0", "0",
		"0", "0", "0", "0",
		"20", "0", "1", "0",
		strconv.FormatUint(startTicks, 10),
	}
	return fmt.Sprintf("%d (%s) %s\n", pid, comm, strings.Join(fields, " "))
}

// addProc gives the fake one process that holds one socket, plus the /proc files
// the reader asks for. fdName is the descriptor the socket appears at.
func (p *portTestProbe) addProc(pid int, name string, uid int, inode, fdName string) {
	p.alive[pid] = true
	base := "/proc/" + strconv.Itoa(pid)
	p.files[base+"/comm"] = []byte(name + "\n")
	p.files[base+"/cmdline"] = []byte(name + "\x00--serve\x00\x00")
	p.files[base+"/status"] = []byte("Name:\t" + name + "\nUid:\t" +
		strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\n")
	p.links[base+"/exe"] = "/usr/bin/" + name
	p.files[base+"/stat"] = []byte(portTestStatLine(pid, 1, name, 12345))
	p.dirs[base+"/fd"] = []os.DirEntry{portTestDirEntry{fdName}}
	p.links[base+"/fd/"+fdName] = "socket:[" + inode + "]"
}

// addNet writes one /proc/net table from rows.
func (p *portTestProbe) addNet(path string, rows ...string) {
	p.files[path] = []byte(portTestNetHeader + strings.Join(rows, ""))
}

// portTestPS is the tool a darwin build reads its process facts from, and
// portTestUserHZ is the rate /proc counts start times in.
const (
	portTestPS     = "/usr/bin/ps"
	portTestUserHZ = 100
)

// runPS renders one ps argv from the fixture's /proc state. The two shapes the
// darwin reader asks for are the identity run (lstart, user, comm) and the
// command run (command), and each is answered by reading the files the Linux
// reader would have read. Any other argv is left unscripted, so a test that
// wants a tool to be missing still gets the refusal it expects.
func (p *portTestProbe) runPS(args []string) (portscan.CommandResult, bool) {
	spec := ""
	if len(args) >= 2 && args[0] == "-o" {
		spec = args[1]
	}
	switch {
	case strings.Contains(spec, "lstart="):
		return p.psIdentity(portTestPSPids(args)), true
	case strings.Contains(spec, "command="):
		return p.psCommands(portTestPSPids(args)), true
	default:
		return portscan.CommandResult{}, false
	}
}

// portTestPSPids reads the repeated -p flags one ps run carries.
func portTestPSPids(args []string) []int {
	var pids []int
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "-p" {
			continue
		}
		if pid, err := strconv.Atoi(args[i+1]); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
		i++
	}
	return pids
}

// psIdentity renders the pid/lstart/user/comm lines the darwin reader parses by
// position: the pid, the five lstart tokens, the account and the executable. A
// pid whose start time or account the fixture does not carry gets no line at
// all, which is ps's own answer for a process it cannot read; the reader then
// reports that the identity cannot be read instead of being handed a row of
// empty fields.
func (p *portTestProbe) psIdentity(pids []int) portscan.CommandResult {
	var out strings.Builder
	for _, pid := range pids {
		account := p.psAccount(pid)
		executable := p.psExecutable(pid)
		started := p.psStarted(pid)
		if account == "" || executable == "" || started == "" {
			continue
		}
		fmt.Fprintf(&out, " %d %s %s %s\n", pid, started, account, executable)
	}
	return portscan.CommandResult{Stdout: []byte(out.String())}
}

// psCommands renders the pid/command lines from /proc/<pid>/cmdline, which is
// the same argv the Linux reader joins from that file.
func (p *portTestProbe) psCommands(pids []int) portscan.CommandResult {
	var out strings.Builder
	for _, pid := range pids {
		data, err := p.ReadFile(portTestProcPath(pid, "cmdline"))
		if err != nil {
			continue
		}
		var argv []string
		for _, field := range strings.Split(string(data), "\x00") {
			if field != "" {
				argv = append(argv, field)
			}
		}
		if len(argv) == 0 {
			continue
		}
		fmt.Fprintf(&out, " %d %s\n", pid, strings.Join(argv, " "))
	}
	return portscan.CommandResult{Stdout: []byte(out.String())}
}

// psAccount is the account ps prints: the real uid from /proc/<pid>/status
// resolved through the fixture's passwd table, or the number when the table has
// no entry, which is what the Linux reader does with the same fact.
func (p *portTestProbe) psAccount(pid int) string {
	data, err := p.ReadFile(portTestProcPath(pid, "status"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "Uid:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return ""
		}
		uid, err := strconv.Atoi(fields[0])
		if err != nil {
			return ""
		}
		if name, err := p.LookupUser(uid); err == nil && name != "" {
			return name
		}
		return strconv.Itoa(uid)
	}
	return ""
}

// psExecutable is the path ps prints for comm: the target of /proc/<pid>/exe
// when the fixture set one, which is where the reader takes the process name
// from, and the comm name otherwise.
func (p *portTestProbe) psExecutable(pid int) string {
	if target, err := p.Readlink(portTestProcPath(pid, "exe")); err == nil {
		return strings.TrimSuffix(target, " (deleted)")
	}
	data, err := p.ReadFile(portTestProcPath(pid, "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// psStarted renders ps's own lstart wording from the boot time in /proc/stat
// and the start ticks in /proc/<pid>/stat, the two facts the Linux reader turns
// into the same instant. The wording is UTC so a test that parses it back gets
// one answer whatever zone the machine running it is in.
func (p *portTestProbe) psStarted(pid int) string {
	btime, ok := p.psBtime()
	if !ok {
		return ""
	}
	ticks, ok := p.psStartTicks(pid)
	if !ok {
		return ""
	}
	when := time.Unix(btime, 0).Add(time.Duration(ticks) * (time.Second / portTestUserHZ))
	return when.UTC().Format("Mon Jan _2 15:04:05 2006")
}

// psBtime reads the boot time out of /proc/stat.
func (p *portTestProbe) psBtime() (int64, bool) {
	data, err := p.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "btime ")
		if !found {
			continue
		}
		seconds, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return 0, false
		}
		return seconds, true
	}
	return 0, false
}

// psStartTicks reads field 22 of /proc/<pid>/stat, which is where the fixture
// writes the start ticks and where the Linux reader reads them from.
func (p *portTestProbe) psStartTicks(pid int) (uint64, bool) {
	data, err := p.ReadFile(portTestProcPath(pid, "stat"))
	if err != nil {
		return 0, false
	}
	text := string(data)
	close := strings.LastIndex(text, ")")
	if close < 0 {
		return 0, false
	}
	fields := strings.Fields(text[close+1:])
	if len(fields) < 20 {
		return 0, false
	}
	ticks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return ticks, true
}

// portTestProcPath is the /proc path of one fact about one process.
func portTestProcPath(pid int, rest string) string {
	return "/proc/" + strconv.Itoa(pid) + "/" + rest
}

// newPortTestProbe builds the in-memory machine the manager tests read:
//
//   - pid 1 systemd on 0.0.0.0:22 (protected)
//   - pid 1000 node on 127.0.0.1:3000 and 0.0.0.0:8080 (two ports, one process)
//   - pid 2000 python3 on 127.0.0.1:9000
//   - pid 4242 the installer itself on 127.0.0.1:7420 (its account is alber)
//   - a bound UDP socket on 5353 whose owner no process links
func newPortTestProbe() *portTestProbe {
	p := &portTestProbe{
		goos:    "linux",
		selfPID: 4242,
		now:     time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		files:   map[string][]byte{},
		dirs:    map[string][]os.DirEntry{},
		links:   map[string]string{},
		users:   map[int]string{0: "root", 1000: "alber"},
		alive:   map[int]bool{},
		parents: map[int]int{},
	}

	p.files["/proc/stat"] = []byte("btime 1700000000\n")
	p.files["/proc/sys/kernel/random/boot_id"] = []byte("boot-1\n")
	p.dirs["/proc"] = []os.DirEntry{
		portTestDirEntry{"1"},
		portTestDirEntry{"1000"},
		portTestDirEntry{"2000"},
		portTestDirEntry{"4242"},
	}

	p.addProc(1, "systemd", 0, "55555", "3")
	p.addProc(1000, "node", 1000, "11111", "3")
	p.addProc(2000, "python3", 1000, "22222", "4")
	p.addProc(4242, "dotfiles", 1000, "44444", "5")

	// The second socket of pid 1000 is added by hand so one process can be shown
	// holding two ports without a second process directory.
	p.dirs["/proc/1000/fd"] = []os.DirEntry{portTestDirEntry{"3"}, portTestDirEntry{"6"}}
	p.links["/proc/1000/fd/6"] = "socket:[66666]"

	p.addNet("/proc/net/tcp",
		portTestNetRow("00000000", 22, "0A", 0, "55555"),
		portTestNetRow("0100007F", 3000, "0A", 1000, "11111"),
		portTestNetRow("0100007F", 7420, "0A", 1000, "44444"),
		portTestNetRow("00000000", 8080, "0A", 1000, "66666"),
		portTestNetRow("0100007F", 9000, "0A", 1000, "22222"),
	)
	p.addNet("/proc/net/tcp6")
	p.addNet("/proc/net/udp",
		portTestNetRow("0100007F", 5353, "07", 1000, "33333"),
	)
	p.addNet("/proc/net/udp6")
	return p
}

// TestTheTUIProbeAnswersTheDarwinPSRuns holds the fixture to the reader a darwin
// build uses. The process reader is chosen when the package is compiled rather
// than from the probe's own GOOS value, so the /proc files this file carries
// leave every process unnamed on a macOS runner -- which is what made five
// manager tests fail there while passing here, the first time this slice ran on
// macOS at all. The two shapes the reader asks for, and the fields it takes by
// position, are stated here, so a fixture that stops answering ps fails on any
// machine instead of only on the one nobody runs locally.
func TestTheTUIProbeAnswersTheDarwinPSRuns(t *testing.T) {
	probe := newPortTestProbe()
	ctx := context.Background()

	identity, err := probe.Run(ctx, portTestPS, []string{"-o", "pid=,lstart=,user=,comm=", "-p", "2000"}, []string{"LC_ALL=C"})
	if err != nil {
		t.Fatalf("the fixture does not answer the identity run: %v", err)
	}
	line := strings.TrimSpace(string(identity.Stdout))
	// The boot time is /proc/stat's btime and the start ticks are field 22 of
	// the process's stat line, which the fixture writes as 12345 -- a hundredth
	// of a second each, so 123 s after the boot time.
	const wantIdentity = "2000 Tue Nov 14 22:15:23 2023 alber /usr/bin/python3"
	if line != wantIdentity {
		t.Errorf("the identity line is %q, want %q", line, wantIdentity)
	}
	if fields := strings.Fields(line); len(fields) < 8 {
		t.Errorf("the identity line carries %d fields, want at least 8: the reader takes the pid, five start tokens, the account and the executable by position", len(fields))
	}

	// The same process asked for twice answers with the same wording, which is
	// what lets a stop revalidate the identity it was shown.
	again, err := probe.Run(ctx, portTestPS, []string{"-o", "pid=,lstart=,user=,comm=", "-p", "2000"}, []string{"LC_ALL=C"})
	if err != nil {
		t.Fatalf("the fixture stops answering the identity run: %v", err)
	}
	if string(again.Stdout) != string(identity.Stdout) {
		t.Errorf("two identity runs for one process differ: %q then %q", identity.Stdout, again.Stdout)
	}

	commands, err := probe.Run(ctx, portTestPS, []string{"-o", "pid=,command=", "-p", "2000"}, []string{"LC_ALL=C"})
	if err != nil {
		t.Fatalf("the fixture does not answer the command run: %v", err)
	}
	if got, want := strings.TrimSpace(string(commands.Stdout)), "2000 python3 --serve"; got != want {
		t.Errorf("the command line is %q, want %q", got, want)
	}

	// A process the fixture does not carry is absent from the run rather than
	// answered with empty fields, which is how the reader learns that this
	// process's identity cannot be read at all.
	missing, err := probe.Run(ctx, portTestPS, []string{"-o", "pid=,lstart=,user=,comm=", "-p", "9999"}, []string{"LC_ALL=C"})
	if err != nil {
		t.Fatalf("the fixture refuses an unknown pid instead of answering with no line: %v", err)
	}
	if strings.TrimSpace(string(missing.Stdout)) != "" {
		t.Errorf("an unknown pid answered %q, want no line", missing.Stdout)
	}
}

// portTestModel is a model parked on the manager with the fake probe and the
// reading already applied, so a test can start from a known list.
func portTestModel(t *testing.T, probe *portTestProbe) Model {
	t.Helper()
	m := NewModel()
	m.Width, m.Height = 80, 24
	m.Animating = false
	m.Ports.Probe = probe
	m.Screen = ScreenPortManager
	m.Ports.Loaded = true
	m.Ports.Snapshot = portscan.Discover(context.Background(), probe)
	return m
}

// portTestRunCmd runs a command the way the Bubbletea runtime would.
func portTestRunCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("the handler returned no command where a reading was expected")
	}
	return cmd()
}

// portTestCursorToPID points the manager's cursor at the row owned by pid,
// clearing the filter so the row is visible.
func portTestCursorToPID(t *testing.T, m *Model, pid int) {
	t.Helper()
	m.Ports.Filter = ""
	for i, l := range m.portVisibleListeners() {
		if l.Owner != nil && l.Owner.PID == pid {
			m.Ports.Cursor = i
			return
		}
	}
	t.Fatalf("no visible row is owned by pid %d", pid)
}

// TestPortManagerIsOfferedAndOpens reads the section's own options and follows
// the row the way a keypress does, so the row and the screen cannot disagree
// about how the manager is reached.
func TestPortManagerIsOfferedAndOpens(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 80, 24
	m.Screen = ScreenUtilities

	index := -1
	for i, option := range m.GetCurrentOptions() {
		if option == utilitiesPortRow {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("the utilities section does not offer %q: %v", utilitiesPortRow, m.GetCurrentOptions())
	}

	m.Cursor = index
	next, cmd := m.handleUtilitiesKeys("enter")
	got := next.(Model)
	if got.Screen != ScreenPortManager {
		t.Fatalf("pressing the row left the screen at %v, want the port manager", got.Screen)
	}
	if cmd == nil {
		t.Fatal("opening the manager returned no reading command")
	}
	if !got.Ports.Loading {
		t.Fatal("opening the manager did not mark the first reading as in flight")
	}
}

// TestPortManagerReadingStoresTheSnapshot runs the command the screen arms and
// folds its message in, which is the pair the runtime performs.
func TestPortManagerReadingStoresTheSnapshot(t *testing.T) {
	probe := newPortTestProbe()
	m := NewModel()
	m.Ports.Probe = probe
	m.Screen = ScreenPortManager
	m.Ports.Loading = true

	msg := portTestRunCmd(t, m.portScanCmd())
	loaded, ok := msg.(portsLoadedMsg)
	if !ok {
		t.Fatalf("the reading command produced %T, want portsLoadedMsg", msg)
	}
	if len(loaded.snap.Listeners) == 0 {
		t.Fatal("the reading found no listener on a host that has five")
	}

	next, _ := m.Update(loaded)
	got := next.(Model)
	if !got.Ports.Loaded || got.Ports.Loading {
		t.Fatalf("after the reading loaded=%v loading=%v, want loaded and not loading", got.Ports.Loaded, got.Ports.Loading)
	}
	if len(got.Ports.Snapshot.Listeners) != len(loaded.snap.Listeners) {
		t.Fatalf("the stored snapshot has %d listeners, want the reading's %d",
			len(got.Ports.Snapshot.Listeners), len(loaded.snap.Listeners))
	}
}

// TestPortManagerFilterNarrowsTheRows covers the three filter dimensions the
// brief names: a port, a process name and a protocol.
func TestPortManagerFilterNarrowsTheRows(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())

	if got := len(m.portVisibleListeners()); got != 6 {
		t.Fatalf("the unfiltered list has %d rows, want 6", got)
	}

	m.Ports.Filter = "3000"
	if got := len(m.portVisibleListeners()); got != 1 {
		t.Fatalf("filtering by port 3000 left %d rows, want 1", got)
	}

	m.Ports.Filter = "python"
	if got := len(m.portVisibleListeners()); got != 1 {
		t.Fatalf("filtering by process name python left %d rows, want 1", got)
	}

	m.Ports.Filter = ""
	m.Ports.Transport = "udp"
	visible := m.portVisibleListeners()
	if len(visible) != 1 || visible[0].Protocol.Transport() != "UDP" {
		t.Fatalf("the UDP transport filter left %+v, want the single bound UDP socket", visible)
	}
}

// TestPortManagerRefreshPreservesCursorBySocketIdentity pins the rule the cursor
// follows a refresh by: the socket's identity, never its index, so an owner pid
// that changed under an unchanged socket does not move the cursor to a different
// port.
func TestPortManagerRefreshPreservesCursorBySocketIdentity(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())
	m.Ports.Filter = "9000"
	if got := len(m.portVisibleListeners()); got != 1 {
		t.Fatalf("filtering to port 9000 left %d rows, want 1", got)
	}
	m.Ports.Cursor = 0
	before := m.portVisibleListeners()[m.Ports.Cursor]
	m.Ports.SelectedKey = m.portSelectedKey()

	// The same socket, now owned by a different process id.
	changed := m.Ports.Snapshot
	changed.Listeners = append([]portscan.Listener(nil), changed.Listeners...)
	for i := range changed.Listeners {
		if changed.Listeners[i].Key() == before.Key() {
			owner := *changed.Listeners[i].Owner
			owner.PID = 7777
			owner.Name = "renamed"
			changed.Listeners[i].Owner = &owner
		}
	}
	m.applyPortSnapshot(changed, true)

	got, ok := m.portSelectedListener()
	if !ok {
		t.Fatal("the cursor points at no row after the refresh")
	}
	if got.Key() != before.Key() {
		t.Fatalf("the cursor moved to %q after the refresh, want the same socket %q", got.Key(), before.Key())
	}
	if got.Owner == nil || got.Owner.PID != 7777 {
		t.Fatalf("the selected row carries owner %+v, want the refreshed pid 7777", got.Owner)
	}
}

// TestPortManagerRefreshNeverMovesTheCursor proves the refresh is not allowed to
// change the selection at all when the machine did not.
func TestPortManagerRefreshNeverMovesTheCursor(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())
	m.Ports.Filter = "8080"
	m.Ports.Cursor = 0
	m.Ports.SelectedKey = m.portSelectedKey()

	before := m.Ports.Cursor
	m.applyPortSnapshot(m.Ports.Snapshot, true)
	if m.Ports.Cursor != before {
		t.Fatalf("the cursor moved from %d to %d across a refresh of an unchanged reading", before, m.Ports.Cursor)
	}
}

// TestPortManagerProtectsPIDOne pins the pre-signal protection: the row is
// visible, the key is pressed, and the screen shows the refusal instead of
// opening the confirmation.
func TestPortManagerProtectsPIDOne(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())
	portTestCursorToPID(t, &m, 1)

	next, cmd := m.handlePortManagerKeys("x")
	got := next.(Model)
	if got.Screen == ScreenPortStop {
		t.Fatal("the confirmation opened for pid 1, which the package refuses to stop")
	}
	if cmd != nil {
		t.Fatal("pressing x on a protected process returned a command")
	}
	if !strings.Contains(got.Ports.Notice, "system") {
		t.Fatalf("the refusal %q does not name the system process it protects", got.Ports.Notice)
	}
}

// TestPortStopScreensListsIdentityOwnerAndEveryPort pins the confirmation's
// content: the process, its account and every socket the reading attributes to
// it, not only the row the key was pressed on.
func TestPortStopScreensListsIdentityOwnerAndEveryPort(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())
	m.Ports.Filter = "3000"
	portTestCursorToPID(t, &m, 1000)

	next, cmd := m.handlePortManagerKeys("x")
	got := next.(Model)
	if got.Screen != ScreenPortStop {
		t.Fatalf("pressing x on an allowed process left the screen at %v, want the confirmation", got.Screen)
	}
	if cmd != nil {
		t.Fatal("opening the confirmation sent a signal; the signal is a later, separate press")
	}
	if len(got.PortStop.Target.Sockets) != 2 {
		t.Fatalf("the target carries %d sockets, want every one of the process's two", len(got.PortStop.Target.Sockets))
	}

	view := got.View()
	for _, want := range []string{"node", "pid 1000", "alber", ":3000", ":8080"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the confirmation does not name %q:\n%s", want, view)
		}
	}
}

// TestPortStopOutcomeStoppedStaysOnTheScreen feeds the verified success and
// checks the outcome is drawn without navigating anywhere.
func TestPortStopOutcomeStoppedStaysOnTheScreen(t *testing.T) {
	m := portTestModel(t, newPortTestProbe())
	m.Ports.Filter = "3000"
	portTestCursorToPID(t, &m, 1000)
	next, _ := m.handlePortManagerKeys("x")
	m = next.(Model)

	result := portscan.StopResult{
		Outcome:  portscan.OutcomeStopped,
		Signal:   portscan.SignalTerminate,
		Message:  "pid 1000 (node) exited. The ports it held are free.",
		Alive:    false,
		Verified: true,
	}
	next, _ = m.Update(portsStopDoneMsg{result: result})
	got := next.(Model)
	if got.Screen != ScreenPortStop {
		t.Fatalf("the outcome moved the screen to %v, want to stay on the confirmation", got.Screen)
	}
	if got.PortStop.Result == nil || got.PortStop.Result.Outcome != portscan.OutcomeStopped {
		t.Fatalf("the outcome was not stored: %+v", got.PortStop.Result)
	}
	view := got.View()
	for _, want := range []string{"Outcome", "exited", "re-read"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the rendered outcome does not name %q:\n%s", want, view)
		}
	}
}

// TestPortStopPermissionDeniedNamesTheKernelAndNoEscalation runs the real stop
// path against a probe whose signal the kernel refuses, then checks the screen's
// words: the refusal is reported, and the run does not elevate its privileges to
// turn it into a success.
func TestPortStopPermissionDeniedNamesTheKernelAndNoEscalation(t *testing.T) {
	probe := newPortTestProbe()
	probe.signalErr = portscan.ErrPermissionDenied
	m := portTestModel(t, probe)
	m.Ports.Filter = "3000"
	portTestCursorToPID(t, &m, 1000)
	next, _ := m.handlePortManagerKeys("x")
	m = next.(Model)

	msg := portTestRunCmd(t, m.portStopCmd(portscan.SignalTerminate))
	done, ok := msg.(portsStopDoneMsg)
	if !ok {
		t.Fatalf("the stop command produced %T, want portsStopDoneMsg", msg)
	}
	if done.result.Outcome != portscan.OutcomePermissionDenied {
		t.Fatalf("the outcome is %q, want permission denied", done.result.Outcome)
	}

	next, _ = m.Update(done)
	got := next.(Model)
	view := got.View()
	for _, want := range []string{"kernel refused", "elevate"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the refused stop does not say %q:\n%s", want, view)
		}
	}
}

// TestPortManagerWSLScopeDeclaresTheHostOutsideReach pins the one thing a WSL
// reading must never imply: that the Windows host's own ports are on screen or
// controllable from here.
func TestPortManagerWSLScopeDeclaresTheHostOutsideReach(t *testing.T) {
	probe := newPortTestProbe()
	probe.wsl = true
	m := portTestModel(t, probe)
	if m.Ports.Snapshot.Scope != portscan.ScopeWSLGuest {
		t.Fatalf("the reading's scope is %q, want the WSL guest scope", m.Ports.Snapshot.Scope)
	}

	view := m.View()
	for _, want := range []string{"WSL guest", "Windows host", "cannot be stopped from here"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the WSL scope line does not say %q:\n%s", want, view)
		}
	}
}

// TestPortManagerPeriodicRefreshPausesOnTheStopScreenAndWhileFiltering pins the
// three states the automatic refresh must not read in, and that it never moves
// the cursor when it does.
func TestPortManagerPeriodicRefreshPausesOnTheStopScreenAndWhileFiltering(t *testing.T) {
	probe := newPortTestProbe()
	m := portTestModel(t, probe)
	m.Ports.Auto = true
	m.Ports.Cursor = 2
	m.Ports.SelectedKey = m.portSelectedKey()

	// On the confirmation screen: no reading is started.
	m.Screen = ScreenPortStop
	next, cmd := m.Update(portsTickMsg(probe.now))
	if cmd != nil {
		t.Fatal("the tick started a reading while the stop screen was in front")
	}
	_ = next

	// While the filter is being typed: the tick re-arms but starts nothing.
	m.Screen = ScreenPortManager
	m.Ports.Filtering = true
	next, cmd = m.Update(portsTickMsg(probe.now))
	got := next.(Model)
	if len(got.Ports.Snapshot.Listeners) == 0 {
		t.Fatal("the tick cleared the reading while the filter was being typed")
	}
	if cmd == nil {
		t.Fatal("the tick stopped re-arming itself while the refresh was still on")
	}

	// On the manager with a reading in flight: the tick does not start a second.
	m.Ports.Filtering = false
	m.Ports.Loading = true
	next, cmd = m.Update(portsTickMsg(probe.now))
	got = next.(Model)
	if !got.Ports.Loading {
		t.Fatal("the tick cleared the in-flight flag")
	}
	if cmd == nil {
		t.Fatal("the tick stopped re-arming itself while the refresh was still on")
	}
}

// TestPortManagerCopySendsOnlyRedactedText pins the one invariant the other
// tests leave open: what the copy action hands to the terminal. The diagnostic
// is the payload the command carries verbatim, so a password on the command line
// of the process behind the port cannot reach a clipboard through this screen.
func TestPortManagerCopySendsOnlyRedactedText(t *testing.T) {
	probe := newPortTestProbe()
	// A password in the joined short form mysql takes, on the command line of the
	// process behind port 9000, so the package's redaction is the only thing
	// keeping it off the screen and out of the diagnostic.
	probe.files["/proc/2000/cmdline"] = []byte("python3\x00--serve\x00-ps3cret\x00\x00")

	m := portTestModel(t, probe)
	portTestCursorToPID(t, &m, 2000)

	text := m.portDiagnosticText()
	if text == "" {
		t.Fatal("the copy action has no diagnostic for a row whose owner is known")
	}
	if strings.Contains(text, "s3cret") {
		t.Fatalf("the diagnostic carries the command line's credential:\n%s", text)
	}
	for _, want := range []string{"port 9000/tcp", "scope " + portscan.ScopeSelf, "owner python3 pid 2000", "args [python3 --serve -p" + "****]"} {
		if !strings.Contains(text, want) {
			t.Errorf("the diagnostic does not name %q:\n%s", want, text)
		}
	}

	// A terminal that has not confirmed OSC 52 gets no command at all, and the
	// screen says the text was left for the user to copy by hand instead.
	next, cmd := m.handlePortManagerKeys("y")
	got := next.(Model)
	if cmd != nil {
		t.Fatal("a copy was sent to a terminal that has not confirmed OSC 52")
	}
	if !strings.Contains(got.Ports.Notice, "left on screen") {
		t.Errorf("notice = %q, want it to say the diagnostic was left on screen", got.Ports.Notice)
	}

	// A terminal that has confirmed it gets exactly one command.
	got.TerminalCapabilities.Clipboard.State = capabilitySupported
	next, cmd = got.handlePortManagerKeys("y")
	got = next.(Model)
	if cmd == nil {
		t.Fatal("no copy command was produced for a terminal that takes OSC 52")
	}
	if !strings.Contains(got.Ports.Notice, "OSC 52") {
		t.Errorf("notice = %q, want it to name the copy it made", got.Ports.Notice)
	}
}
