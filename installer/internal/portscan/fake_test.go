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

// linuxHost builds a fake Linux host with a process that holds one TCP socket,
// which is the fixture almost every discovery and stop test starts from.
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

	base := "/proc/" + strconv.Itoa(pid)
	p.addFile(base+"/comm", "node\n")
	p.addRawFile(base+"/cmdline", []byte("node\x00server.js\x00--token\x00SECRET\x00"))
	p.addFile(base+"/status", "Name:\tnode\nUid:\t1000\t1000\t1000\t1000\n")
	p.addLink(base+"/exe", "/usr/bin/node")
	p.addFile(base+"/stat", procStatLine(pid, 1, "node", 12345))
	p.addDir(base+"/fd", fakeEntry{name: "3"})
	p.addLink(base+"/fd/3", "socket:["+inode+"]")

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
