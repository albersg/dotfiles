// Package portscan reads the sockets a machine has bound and the processes
// behind them, and stops one of those processes only after revalidating that it
// is still the process the user was shown.
//
// It exists because "what is using port 3000?" is a question the machine can
// answer and a search engine cannot: the answer is a specific bound socket, the
// process that owns it, and whether this run may signal that process at all.
// Four rules shape everything here:
//
//   - A reading is what the platform reports, never a guess. TCP listeners and
//     bound UDP sockets are different facts with different states, and a socket
//     whose owner could not be read says so instead of appearing free.
//   - Every capability is probed, not assumed. /proc, lsof, ss and netstat are
//     each tried where they exist, and a host that has none of them produces a
//     snapshot that names the gap rather than an empty list that reads as "no
//     ports are in use".
//   - Nothing is signalled by name, by port or in bulk. A stop names one PID,
//     revalidates its identity and its ports first, and verifies the outcome
//     afterwards.
//   - The package is a library. It never runs as a render: every reading goes
//     through the Probe, which the TUI hands in from a command off the update
//     loop and the tests replace with a double.
//
// The Probe boundary is also the safety boundary: the tests can exercise a
// process that does not exist, a permission the kernel would refuse and a PID
// that has been recycled without ever signalling a real process.
package portscan

import (
	"context"
	"errors"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Protocol is one socket family as the platform reports it. TCP and UDP are
// kept apart on purpose: a TCP socket in StateListen is waiting for
// connections, while a UDP socket is a bound receiver with no listening state
// at all, and folding the two into one "open port" row would misdescribe both.
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolTCP6 Protocol = "tcp6"
	ProtocolUDP  Protocol = "udp"
	ProtocolUDP6 Protocol = "udp6"
)

// Transport names the transport protocol the way a reader recognises it.
func (p Protocol) Transport() string {
	if strings.HasPrefix(string(p), "tcp") {
		return "TCP"
	}
	return "UDP"
}

// Family names the address family the protocol belongs to.
func (p Protocol) Family() string {
	if strings.HasSuffix(string(p), "6") {
		return "IPv6"
	}
	return "IPv4"
}

// Stream reports whether the transport is connection-oriented. It decides
// whether a listening state is a fact about the socket or an invention.
func (p Protocol) Stream() bool { return p.Transport() == "TCP" }

// State is the socket's state in the vocabulary the explanation needs, not the
// kernel's numeric code. StateListen is a TCP socket waiting for connections;
// StateUnconn is a bound socket with no connection, which is what every UDP
// receiver is.
type State string

const (
	StateListen State = "LISTEN"
	StateUnconn State = "UNCONN"
)

// Signal is one of the two signals this package may send. The zero value is the
// soft one: a caller that decides nothing still stops softly. The forced signal
// is never sent by a loop or a fallback, only by an explicit second request.
type Signal int

const (
	// SignalTerminate asks the process to exit. It is the default.
	SignalTerminate Signal = iota
	// SignalKill cannot be caught by the process. It is the separate, explicit
	// second action after a terminate that left the process running.
	SignalKill
)

// String names the signal the way the screen and the outcome read it.
func (s Signal) String() string {
	if s == SignalKill {
		return "SIGKILL"
	}
	return "SIGTERM"
}

// Process is the identity of one process, as far as the platform would tell it.
// Every field can be empty on its own: a host that hides another user's
// command line still reports the PID, and a process whose start time is not
// readable is still a process. IdentityKnown reports whether the platform gave
// enough for a later revalidation to recognise the same process again.
type Process struct {
	// PID is the process id the platform reported.
	PID int
	// Name is the process's short name: /proc/<pid>/comm on Linux, the command
	// name on macOS.
	Name string
	// User is the owning account's name, empty when it could not be resolved.
	User string
	// Exe is the executable the process resolved to, empty when unreadable.
	Exe string
	// Args are the process's arguments, already redacted by this package. They
	// are never the raw command line: a server started with a token in its
	// arguments must not put that token on a screen or in a diagnostic.
	Args []string
	// StartedAt is the process's start time when the platform gave one that
	// parses exactly, and the zero time otherwise.
	StartedAt time.Time
	// StartedLabel is the platform's own wording for the start time, kept
	// verbatim for a platform whose format this package does not parse. It is
	// what makes the identity auditable even when StartedAt is zero.
	StartedLabel string
	// Scope names the side of a host/guest boundary the reading came from,
	// because a PID only means something inside one kernel.
	Scope string
	// token is the platform's identity token: Linux uses the boot id and the
	// start ticks, macOS the raw start-time string. Two readings of the same
	// process carry the same token; a recycled PID carries a different one.
	token string
}

// IdentityKnown reports whether this reading carries a token a later reading
// can be compared against. A false answer is not a failure: it is the reason
// the confirmation screen says identity cannot be revalidated and offers only
// the liveness check.
func (p Process) IdentityKnown() bool { return p.token != "" }

// SameIdentity reports whether two readings describe the same process
// incarnation. When either reading carries no token the two PIDs are compared,
// which is the most the platform allows and exactly what the caller was told.
func (p Process) SameIdentity(other Process) bool {
	if p.PID != other.PID {
		return false
	}
	if p.token != "" && other.token != "" {
		return p.token == other.token
	}
	return true
}

// StartedText is the process's start time in the most precise wording the
// platform gave: the parsed time when it parsed, the platform's own string when
// it did not, and an empty string when the platform reported neither.
func (p Process) StartedText() string {
	if !p.StartedAt.IsZero() {
		return p.StartedAt.Format("2006-01-02 15:04:05")
	}
	return p.StartedLabel
}

// Listener is one socket the machine has bound. Address is numeric and carries
// no brackets: "127.0.0.1", "0.0.0.0", "::1", "::".
type Listener struct {
	// Protocol and State describe the socket itself.
	Protocol Protocol
	State    State
	// Address is the local address the socket is bound to, numeric and without
	// brackets, and Port the local port. Address "+" is never invented: a
	// platform that could not name the address reports the socket without it.
	Address string
	Port    int
	// Inode identifies the socket inside the kernel and is what ownership is
	// resolved through. Empty when the reader could not report one.
	Inode string
	// Owner is the process holding the socket, or nil when it could not be
	// resolved here. A nil owner never means the port is free: OwnerErr says
	// what stood in the way.
	Owner *Process
	// OwnerErr is why the owner is unknown, in the screen's own words.
	OwnerErr string
	// Source names the reader that reported the socket, so a row can be traced
	// back to the command or file it came from.
	Source string
}

// Endpoint renders the local address the way a socket prints: bracketed for
// IPv6, and plain for IPv4.
func (l Listener) Endpoint() string {
	return net.JoinHostPort(l.Address, strconv.Itoa(l.Port))
}

// Wildcard reports whether the socket is bound to every interface, which is
// the difference between a port reachable from the network and one reachable
// only from this machine.
func (l Listener) Wildcard() bool { return l.Address == "0.0.0.0" || l.Address == "::" }

// Loopback reports whether the socket is bound to a loopback address only.
func (l Listener) Loopback() bool {
	ip := net.ParseIP(l.Address)
	return ip != nil && ip.IsLoopback()
}

// Key is the identity of one bound socket across two readings: the protocol,
// the address, the port and the socket's inode. It is what the screen compares
// to keep the cursor on the same row across a refresh, and what the stop path
// compares to know whether the port the user saw is still the port it stops.
func (l Listener) Key() string {
	return string(l.Protocol) + "|" + l.Address + "|" + strconv.Itoa(l.Port) + "|" + l.Inode
}

// Source is one reader's contribution to a snapshot: how many rows it reported
// and, when it failed, why. A snapshot with several sources can be read row by
// row, and a source that failed is named rather than silently dropped.
type Source struct {
	Name string
	Rows int
	Err  string
}

// Snapshot is one reading of the machine: the sockets that were visible, the
// readers that answered, and the limits that stood in the way.
type Snapshot struct {
	// At is the moment the reading was taken and Scope the side of a host/guest
	// boundary it belongs to.
	At    time.Time
	Scope string
	// Listeners are the bound sockets, sorted by port. Owner is nil where the
	// owner could not be resolved.
	Listeners []Listener
	// Sources are the readers that were tried, in the order they were tried. A
	// reader that reported nothing because the host has no /proc is a fact of
	// its own.
	Sources []Source
	// Limits are the declared limitations of this reading -- a permission, a
	// platform, a missing tool -- each one a sentence the screen can show
	// beside the rows it qualifies.
	Limits []string
}

// ListenersFor returns the listeners a process holds in this snapshot, which is
// what makes "the ports affected by stopping this process" a fact rather than a
// promise. The result is a copy: the caller never reorders the snapshot.
func (s Snapshot) ListenersFor(pid int) []Listener {
	var held []Listener
	for _, l := range s.Listeners {
		if l.Owner != nil && l.Owner.PID == pid {
			held = append(held, l)
		}
	}
	return held
}

// CommandResult is what one executed argv produced. Stdout is the answer and
// Stderr the tool's own words about what it could not do, which is how a reader
// reports a partial answer instead of a silent one.
type CommandResult struct {
	Stdout []byte
	Stderr []byte
}

// The two failures a signal can produce, named so the stop path can report them
// as outcomes rather than as platform error strings. A probe translates the
// kernel's own codes into these; a test double returns them directly.
var (
	// ErrNoSuchProcess means the PID did not exist when the kernel was asked.
	ErrNoSuchProcess = errors.New("portscan: no such process")
	// ErrPermissionDenied means the kernel refused; privileges are never
	// escalated to turn this into a success.
	ErrPermissionDenied = errors.New("portscan: permission denied")
)

// Probe is the machine surface this package reads. The TUI hands in the real
// one; the tests hand in a double, so the destructive path is exercised without
// a signal ever reaching a process the test does not own.
//
// Every method is a leaf: it reads, runs one command, or sends one signal, and
// it decides nothing. The decisions -- which reader to trust, what counts as
// the same process, whether a process may be stopped -- live in this package,
// where they can be tested.
type Probe interface {
	// Now is the clock, so a snapshot's time is injectable.
	Now() time.Time
	// GOOS names the platform the way Go names it, and IsWSL reports whether
	// this run is a guest inside Windows, which decides the scope every row
	// carries.
	GOOS() string
	IsWSL() bool
	// SelfPID is the installer's own PID, which is never a stop target, and
	// Parent reports a process's parent, which is how the ancestor chain is
	// protected.
	SelfPID() int
	Parent(pid int) (int, bool)
	// The filesystem leaves the readers use: /proc and, on macOS, nothing.
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Readlink(name string) (string, error)
	// LookPath finds a tool without running it, and Run executes one argv with
	// no shell in between. A command that fails carries its stderr in the error
	// so a reader can report what the tool said.
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, args []string, env []string) (CommandResult, error)
	// Alive reports whether a PID exists. An unreadable process that is
	// certainly not ours is still alive; an error means the platform could not
	// answer.
	Alive(pid int) (bool, error)
	// Signal sends one signal to one PID.
	Signal(pid int, sig Signal) error
	// LookupUser resolves a numeric uid to an account name.
	LookupUser(uid int) (string, error)
}

// DefaultProbe returns the real machine surface for this platform.
func DefaultProbe() Probe { return defaultProbe{} }

// ScopeSelf is the scope name for sockets this machine's own kernel holds.
const ScopeSelf = "this machine"

// ScopeWSLGuest is the scope name for sockets inside a WSL distro. It is a
// name of its own because a PID inside the guest and a PID on the Windows host
// are different namespaces: nothing read here is a host process, and nothing
// here can signal one.
const ScopeWSLGuest = "WSL guest"

// Discover reads the machine and returns one snapshot. It never returns an
// error: a platform that cannot be read at all is a snapshot whose Limits say
// so, because an empty list and a refused read must not look the same.
func Discover(ctx context.Context, p Probe) Snapshot {
	if p == nil {
		p = DefaultProbe()
	}

	var snap Snapshot
	switch p.GOOS() {
	case "linux":
		snap = discoverLinux(ctx, p)
	case "darwin":
		snap = discoverDarwin(ctx, p)
	default:
		snap = Snapshot{Limits: []string{
			"this platform (" + p.GOOS() + ") has no socket reader in this installer yet, so no port can be listed here. " +
				"Nothing was guessed in its place.",
		}}
	}

	snap.At = p.Now()
	snap.Scope = localScope(p)
	// A reader that reported only a pid -- ss, netstat, lsof without ps -- leaves
	// the scope and the source empty. Both are filled here, once, from the scope
	// the snapshot itself was taken in, so no row can be read as belonging to a
	// different kernel than the reading.
	for i := range snap.Listeners {
		if snap.Listeners[i].Owner != nil && snap.Listeners[i].Owner.Scope == "" {
			snap.Listeners[i].Owner.Scope = snap.Scope
		}
		if snap.Listeners[i].Source == "" {
			snap.Listeners[i].Source = "the platform reader"
		}
	}
	sortListeners(snap.Listeners)
	return snap
}

// localScope names the kernel this run reads from.
func localScope(p Probe) string {
	if p.IsWSL() {
		return ScopeWSLGuest
	}
	return ScopeSelf
}

// sortListeners puts a snapshot in the order the screen lists it: by port, then
// by transport, address and inode. The order is derived from the rows alone, so
// two readings of an unchanged machine list their rows identically.
func sortListeners(listeners []Listener) {
	sort.SliceStable(listeners, func(i, j int) bool {
		a, b := listeners[i], listeners[j]
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.Address != b.Address {
			return a.Address < b.Address
		}
		return a.Inode < b.Inode
	})
}

// joinLimits appends one limitation sentence when it is not empty, so callers
// can name a gap in one line without repeating the empty check.
func joinLimits(limits []string, line string) []string {
	if strings.TrimSpace(line) == "" {
		return limits
	}
	return append(limits, line)
}
