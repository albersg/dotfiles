package portscan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// fakeProbe is the machine surface the tests read through. It is an in-memory
// filesystem, a scripted command table and a signal recorder, so a test can
// exercise the whole stop path -- including the destructive half -- without a
// signal ever reaching a process the test does not own.
type fakeProbe struct {
	goos    string
	wsl     bool
	selfPID int
	now     time.Time
	// clockStep is how far the fake clock moves on every reading. A test that
	// exercises the grace period sets it so the deadline is reached without a
	// real sleep; every other test leaves it at zero and the clock stands still.
	clockStep time.Duration

	parents map[int]int
	users   map[int]string

	files   map[string][]byte
	fileErr map[string]error
	dirs    map[string][]os.DirEntry
	dirErr  map[string]error
	links   map[string]string
	linkErr map[string]error

	tools     map[string]string
	commands  map[string]CommandResult
	commandFn map[string]func() (CommandResult, error)
	// derived answers a tool whose output the fake can render from the state it
	// already holds, when no exact argv was scripted. The ps bridge is the one
	// user: a flow fixture states one process's facts in /proc, and the macOS
	// reader asks ps about the same process.
	derived map[string]func(p *fakeProbe, args []string) (CommandResult, error)

	alive    map[int]bool
	aliveErr map[int]error

	signals   []signalCall
	signalErr error
	onSignal  func(p *fakeProbe, pid int, sig Signal)
}

type signalCall struct {
	pid int
	sig Signal
}

func newFakeProbe(goos string) *fakeProbe {
	return &fakeProbe{
		goos:      goos,
		selfPID:   4242,
		now:       time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		parents:   map[int]int{},
		users:     map[int]string{},
		files:     map[string][]byte{},
		fileErr:   map[string]error{},
		dirs:      map[string][]os.DirEntry{},
		dirErr:    map[string]error{},
		links:     map[string]string{},
		linkErr:   map[string]error{},
		tools:     map[string]string{},
		commands:  map[string]CommandResult{},
		commandFn: map[string]func() (CommandResult, error){},
		derived:   map[string]func(p *fakeProbe, args []string) (CommandResult, error){},
		alive:     map[int]bool{},
		aliveErr:  map[int]error{},
	}
}

func (p *fakeProbe) Now() time.Time {
	p.now = p.now.Add(p.clockStep)
	return p.now
}

func (p *fakeProbe) GOOS() string { return p.goos }

func (p *fakeProbe) IsWSL() bool { return p.wsl }

func (p *fakeProbe) SelfPID() int { return p.selfPID }

func (p *fakeProbe) Parent(pid int) (int, bool) {
	parent, ok := p.parents[pid]
	return parent, ok
}

func (p *fakeProbe) ReadFile(name string) ([]byte, error) {
	if err, ok := p.fileErr[name]; ok {
		return nil, err
	}
	if data, ok := p.files[name]; ok {
		return data, nil
	}
	return nil, notExist(name)
}

func (p *fakeProbe) ReadDir(name string) ([]os.DirEntry, error) {
	if err, ok := p.dirErr[name]; ok {
		return nil, err
	}
	if entries, ok := p.dirs[name]; ok {
		return entries, nil
	}
	return nil, notExist(name)
}

func (p *fakeProbe) Readlink(name string) (string, error) {
	if err, ok := p.linkErr[name]; ok {
		return "", err
	}
	if target, ok := p.links[name]; ok {
		return target, nil
	}
	return "", notExist(name)
}

func (p *fakeProbe) LookPath(name string) (string, error) {
	if tool, ok := p.tools[name]; ok {
		return tool, nil
	}
	return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
}

func (p *fakeProbe) Run(ctx context.Context, name string, args []string, env []string) (CommandResult, error) {
	key := commandKey(name, args)
	if fn, ok := p.commandFn[key]; ok {
		return fn()
	}
	if result, ok := p.commands[key]; ok {
		return result, nil
	}
	if fn, ok := p.derived[path.Base(name)]; ok {
		return fn(p, args)
	}
	return CommandResult{}, fmt.Errorf("%s: no scripted answer for %q", name, strings.Join(args, " "))
}

func (p *fakeProbe) Alive(pid int) (bool, error) {
	if err, ok := p.aliveErr[pid]; ok {
		return false, err
	}
	return p.alive[pid], nil
}

func (p *fakeProbe) Signal(pid int, sig Signal) error {
	p.signals = append(p.signals, signalCall{pid: pid, sig: sig})
	if p.signalErr != nil {
		return p.signalErr
	}
	if p.onSignal != nil {
		p.onSignal(p, pid, sig)
	}
	return nil
}

func (p *fakeProbe) LookupUser(uid int) (string, error) {
	if name, ok := p.users[uid]; ok {
		return name, nil
	}
	return "", fmt.Errorf("user: unknown userid %d", uid)
}

// The builders below keep a test's setup to the facts it is about.

func (p *fakeProbe) addFile(name, content string) *fakeProbe {
	p.files[name] = []byte(content)
	return p
}

func (p *fakeProbe) addRawFile(name string, content []byte) *fakeProbe {
	p.files[name] = content
	return p
}

func (p *fakeProbe) addDir(name string, entries ...os.DirEntry) *fakeProbe {
	p.dirs[name] = entries
	return p
}

func (p *fakeProbe) addLink(name, target string) *fakeProbe {
	p.links[name] = target
	return p
}

func (p *fakeProbe) addTool(name string) *fakeProbe {
	p.tools[name] = path.Join("/usr/bin", name)
	return p
}

func (p *fakeProbe) addCommand(name string, args []string, result CommandResult) *fakeProbe {
	p.commands[commandKey(name, args)] = result
	return p
}

func (p *fakeProbe) addCommandErr(name string, args []string, result CommandResult, err error) *fakeProbe {
	p.commandFn[commandKey(name, args)] = func() (CommandResult, error) { return result, err }
	return p
}

// answerPSFromProc makes the fake answer the two ps runs the macOS identity
// reader issues, rendered from the same /proc files the Linux reader reads. A
// flow fixture therefore states each process fact once: whichever reader the
// platform compiled is the one that asks, and this fake answers both from one
// state. Registering the tool is part of the bridge because readDarwinProcess
// calls LookPath("ps") before it runs anything, and a fake that answered ps
// without ps on PATH would not exercise that gate.
//
// The answers are derived rather than scripted so a test that changes a fact
// between two readings -- the start ticks of a recycled PID, the file that
// disappeared, the port that moved -- changes it in one place and both dialects
// follow.
func (p *fakeProbe) answerPSFromProc() *fakeProbe {
	p.addTool("ps")
	p.derived["ps"] = renderPSFromProc
	return p
}

// renderPSFromProc renders one ps argv from the fake's /proc state. The two
// shapes the darwin reader asks for are the identity run (lstart, user, comm)
// and the command run (command), and each is answered by reading the files the
// Linux reader would have read.
func renderPSFromProc(p *fakeProbe, args []string) (CommandResult, error) {
	spec := ""
	if len(args) >= 2 && args[0] == "-o" {
		spec = args[1]
	}
	switch {
	case strings.Contains(spec, "lstart="):
		return psIdentityFromProc(p, psTargetPIDs(args)), nil
	case strings.Contains(spec, "command="):
		return psCommandsFromProc(p, psTargetPIDs(args)), nil
	default:
		return CommandResult{}, fmt.Errorf("the fake has no ps renderer for %q", strings.Join(args, " "))
	}
}

// psTargetPIDs reads the repeated -p flags one ps run carries.
func psTargetPIDs(args []string) []int {
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

// psIdentityFromProc renders the lstart/user/comm line the macOS reader parses.
// A pid whose start time or account the fixture does not carry gets no line at
// all, which is the same answer ps gives for a process it cannot read: the
// reader then reports that the identity cannot be revalidated instead of being
// handed a row with empty fields.
func psIdentityFromProc(p *fakeProbe, pids []int) CommandResult {
	var out strings.Builder
	for _, pid := range pids {
		user := psAccountFromProc(p, pid)
		comm := psExecutableFromProc(p, pid)
		started := psStartedFromProc(p, pid)
		if user == "" || comm == "" || started == "" {
			continue
		}
		fmt.Fprintf(&out, " %d %s %s %s\n", pid, started, user, comm)
	}
	return CommandResult{Stdout: []byte(out.String())}
}

// psAccountFromProc resolves the account the way the Linux reader does: the
// real uid from /proc/<pid>/status, then the account name, then the number when
// the host has no passwd entry for it.
func psAccountFromProc(p *fakeProbe, pid int) string {
	data, err := p.ReadFile(procPath(pid, "status"))
	if err != nil {
		return ""
	}
	uid, ok := procStatusUID(data)
	if !ok {
		return ""
	}
	if name, err := p.LookupUser(uid); err == nil && name != "" {
		return name
	}
	return strconv.Itoa(uid)
}

// psExecutableFromProc is the path ps prints for comm: the /proc/<pid>/exe
// target when the fixture set one, and the command name otherwise, which is the
// same preference the Linux reader applies.
func psExecutableFromProc(p *fakeProbe, pid int) string {
	if exe, err := p.Readlink(procPath(pid, "exe")); err == nil {
		return strings.TrimSuffix(exe, " (deleted)")
	}
	if data, err := p.ReadFile(procPath(pid, "comm")); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// psStartedFromProc renders ps's own lstart wording from the boot time in
// /proc/stat and the start ticks in /proc/<pid>/stat, which are the two facts
// the Linux reader turns into the same instant. The fake prints the wording in
// UTC so a test that parses it back gets one answer whatever zone the host
// running the test is in.
func psStartedFromProc(p *fakeProbe, pid int) string {
	btime, ok := procBtime(p)
	if !ok {
		return ""
	}
	ticks, ok := procStartTicks(p, pid)
	if !ok {
		return ""
	}
	when := time.Unix(btime, 0).Add(time.Duration(ticks) * (time.Second / fakeUserHZ))
	return when.UTC().Format("Mon Jan _2 15:04:05 2006")
}

// psCommandsFromProc renders the pid/command line from /proc/<pid>/cmdline.
func psCommandsFromProc(p *fakeProbe, pids []int) CommandResult {
	var out strings.Builder
	for _, pid := range pids {
		data, err := p.ReadFile(procPath(pid, "cmdline"))
		if err != nil {
			continue
		}
		args := splitNULTokens(data)
		if len(args) == 0 {
			continue
		}
		fmt.Fprintf(&out, " %d %s\n", pid, strings.Join(args, " "))
	}
	return CommandResult{Stdout: []byte(out.String())}
}

// The small /proc parsers below are deliberately test-local: they let the fake
// read the same files the Linux reader reads instead of keeping a second copy
// of the facts, and the production parsers are not visible to this untagged
// file on every platform.

func procPath(pid int, rest string) string {
	return "/proc/" + strconv.Itoa(pid) + "/" + rest
}

// procBtime reads the boot time out of /proc/stat.
func procBtime(p *fakeProbe) (int64, bool) {
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

// procStartTicks reads field 22 out of /proc/<pid>/stat, counting from the last
// closing parenthesis the way the production parser does.
func procStartTicks(p *fakeProbe, pid int) (uint64, bool) {
	data, err := p.ReadFile(procPath(pid, "stat"))
	if err != nil {
		return 0, false
	}
	text := string(data)
	closing := strings.LastIndex(text, ")")
	if closing < 0 {
		return 0, false
	}
	fields := strings.Fields(text[closing+1:])
	if len(fields) < 20 {
		return 0, false
	}
	ticks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return ticks, true
}

// procStatusUID reads the real uid from /proc/<pid>/status.
func procStatusUID(data []byte) (int, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "Uid:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		uid, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, false
		}
		return uid, true
	}
	return 0, false
}

// splitNULTokens splits the NUL-separated argv /proc/<pid>/cmdline carries.
func splitNULTokens(data []byte) []string {
	var out []string
	for _, part := range strings.Split(string(data), "\x00") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// commandKey is the identity one scripted command is looked up by: the tool's
// name and its whole argv, so two invocations of the same tool are two answers.
func commandKey(name string, args []string) string {
	return path.Base(name) + " " + strings.Join(args, " ")
}

// fakeEntry is one directory entry for the fake filesystem.
type fakeEntry struct {
	name  string
	isDir bool
}

func (e fakeEntry) Name() string               { return e.name }
func (e fakeEntry) IsDir() bool                { return e.isDir }
func (e fakeEntry) Type() fs.FileMode          { return 0 }
func (e fakeEntry) Info() (fs.FileInfo, error) { return nil, fmt.Errorf("not implemented") }

// notExist builds the error the readers see for a file this fake has no entry
// for, so a missing /proc behaves like a missing /proc.
func notExist(name string) error {
	return &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func permissionDenied(name string) error {
	return &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
}

// fakeUserHZ mirrors USER_HZ, the unit /proc counts start times in, which the
// ps bridge needs to turn start ticks into ps's own start wording.
const fakeUserHZ = 100

// procStatLine renders one /proc/<pid>/stat line with the fixed field count the
// parser counts from: state, parent, the counters, then the start ticks at
// field 22.
func procStatLine(pid, ppid int, comm string, startTicks uint64) string {
	fields := []string{
		"S",                // 3 state
		strconv.Itoa(ppid), // 4 ppid
		"1", "1", "0", "1", // 5..8 pgrp, session, tty, tpgid
		"0", "0", "0", "0", "0", // 9..13 flags and the page counters
		"0", "0", "0", "0", // 14..17 user and system times
		"20", "0", "1", "0", // 18..21 priority, nice, threads, itrealvalue
		strconv.FormatUint(startTicks, 10), // 22 starttime
	}
	return fmt.Sprintf("%d (%s) %s\n", pid, comm, strings.Join(fields, " "))
}

// procNetHeader is the header line of every /proc/net table, which the parser
// skips.
const procNetHeader = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"

// procNetRow renders one /proc/net row. hexAddress is the kernel's own hex
// form, so a test can spell an IPv4 or an IPv6 address the way the kernel does.
func procNetRow(hexAddress, hexPort, state string, uid int, inode string) string {
	return fmt.Sprintf("   0: %s:%s 00000000:0000 %s 00000000:00000000 00:00000000 00000000 %6d        0 %s 1 0000000000000000 100 0 0 10 0\n",
		hexAddress, hexPort, state, uid, inode)
}

// procFacts is one process as a fixture states it. The facts are written to the
// /proc files the Linux reader reads; the ps bridge renders the macOS dialect
// from those same files, so a test states each fact once and the compiled
// platform reader picks the dialect it asks in.
type procFacts struct {
	pid        int
	ppid       int
	name       string
	exe        string
	args       []string
	uid        int
	startTicks uint64
}

// addProcProcess writes one process's facts the way /proc presents them. It is
// callable again: writing the same paths a second time replaces the process's
// identity in both dialects at once, which is what a recycled-PID or a
// disappeared-start-file test needs.
func (p *fakeProbe) addProcProcess(f procFacts) *fakeProbe {
	uid := strconv.Itoa(f.uid)
	p.addFile(procPath(f.pid, "comm"), f.name+"\n")
	p.addRawFile(procPath(f.pid, "cmdline"), []byte(joinNUL(f.args)))
	p.addFile(procPath(f.pid, "status"), "Name:\t"+f.name+"\nUid:\t"+uid+"\t"+uid+"\t"+uid+"\t"+uid+"\n")
	if f.exe != "" {
		p.addLink(procPath(f.pid, "exe"), f.exe)
	}
	p.addFile(procPath(f.pid, "stat"), procStatLine(f.pid, f.ppid, f.name, f.startTicks))
	return p
}

// joinNUL renders an argv the way the kernel writes /proc/<pid>/cmdline: the
// arguments joined by NUL bytes and terminated by one.
func joinNUL(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, "\x00") + "\x00"
}

// linuxHost builds a fake Linux host with a process that holds one TCP socket,
// which is the fixture almost every discovery and stop test starts from.
//
// The fixture answers in both dialects on purpose. Its GOOS is linux, so
// Discover and the stop path dispatch to the linux branches, but the compiled
// fact reader behind those branches is the platform's own: on Linux it reads
// these /proc files, and on macOS the same names drive ps, which this fixture
// answers through answerPSFromProc. That is what lets one stop test pin the
// platform-independent decision while the macOS runner exercises the darwin
// identity path.
func linuxHost(pid, port int, inode string) *fakeProbe {
	p := newFakeProbe("linux")
	p.alive[pid] = true
	p.users[1000] = "alber"
	p.parents[pid] = 1
	p.parents[p.selfPID] = 900
	p.parents[900] = 1

	p.addFile("/proc/stat", "cpu  1 2 3 4\nbtime 1700000000\n")
	p.addFile("/proc/sys/kernel/random/boot_id", "boot-1\n")
	p.addDir("/proc", fakeEntry{name: strconv.Itoa(pid), isDir: true})
	p.addProcProcess(procFacts{
		pid:        pid,
		ppid:       1,
		name:       "node",
		exe:        "/usr/bin/node",
		args:       []string{"node", "server.js", "--token", "SECRET"},
		uid:        1000,
		startTicks: 12345,
	})
	p.answerPSFromProc()

	base := procPath(pid, "")
	p.addDir(base+"fd", fakeEntry{name: "3"})
	p.addLink(base+"fd/3", "socket:["+inode+"]")

	p.addFile("/proc/net/tcp", procNetHeader+procNetRow("0100007F", fmt.Sprintf("%04X", port), "0A", 1000, inode))
	p.addFile("/proc/net/tcp6", procNetHeader)
	p.addFile("/proc/net/udp", procNetHeader)
	p.addFile("/proc/net/udp6", procNetHeader)
	return p
}

// soleListener is the one listener a fixture produced, with the caller's own
// failure message when the fixture is wrong.
func soleListener(t interface{ Fatalf(string, ...any) }, snap Snapshot) Listener {
	if len(snap.Listeners) != 1 {
		t.Fatalf("the snapshot has %d listeners, want 1: %+v", len(snap.Listeners), snap.Listeners)
	}
	return snap.Listeners[0]
}

// discoverFixture runs Discover against a fixture the way the TUI command does.
func discoverFixture(p Probe) Snapshot {
	return Discover(context.Background(), p)
}
