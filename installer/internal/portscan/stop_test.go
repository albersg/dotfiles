package portscan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The stop tests drive the whole ordered path through the double: refusal,
// liveness, identity, ports, signal, verification. None of them signals a real
// process; the only test that does spawns the process it stops, in
// integration_test.go.

// stopFixture discovers the one listener the Linux fixture holds and returns it
// with the snapshot, so every test starts from what the screen would have shown.
func stopFixture(t *testing.T, p *fakeProbe) (Listener, Snapshot) {
	t.Helper()
	snap := discoverFixture(p)
	return soleListener(t, snap), snap
}

// stopOptions is what a test stops with: the clock steps a second on every
// reading, so the grace deadline is reached without a real wait while the first
// liveness answer still decides the outcome.
func stopOptions(p *fakeProbe, sig Signal) StopOptions {
	p.clockStep = time.Second
	return StopOptions{Signal: sig, Grace: time.Millisecond, Poll: time.Millisecond}
}

func TestStopRefusesAProtectedProcess(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	_, snap := stopFixture(t, p)

	result := Stop(context.Background(), p, StopTarget{
		Process: Process{PID: 1, Name: "systemd", Scope: ScopeSelf},
		Sockets: snap.Listeners,
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeRefused {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeRefused)
	}
	if len(p.signals) != 0 {
		t.Fatalf("a protected process was signalled: %+v", p.signals)
	}
	if !strings.Contains(result.Message, "init process") {
		t.Errorf("message = %q, want the reason named", result.Message)
	}
}

func TestStopRefusesAnAncestorOfTheInstaller(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.parents[p.selfPID] = 900
	p.parents[900] = 1

	result := Stop(context.Background(), p, StopTarget{
		Process: Process{PID: 900, Name: "zsh", Scope: ScopeSelf},
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeRefused || len(p.signals) != 0 {
		t.Fatalf("outcome = %q with %d signals, want a refusal and none", result.Outcome, len(p.signals))
	}
	if !strings.Contains(result.Message, "ancestor") {
		t.Errorf("message = %q, want the ancestor reason", result.Message)
	}
}

func TestStopReportsAProcessThatAlreadyExited(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.alive[999] = false

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeAlreadyGone {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeAlreadyGone)
	}
	if len(p.signals) != 0 {
		t.Fatalf("a dead PID was signalled: %+v", p.signals)
	}
	if result.Verified {
		t.Error("Verified is true although nothing was sent")
	}
}

func TestStopRefusesARecycledPID(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	// The PID is alive again, but it is a different incarnation: the start ticks
	// are the platform's own proof of that.
	p.files["/proc/999/stat"] = []byte(procStatLine(999, 1, "python3", 99999))

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeRefused {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeRefused)
	}
	if len(p.signals) != 0 {
		t.Fatalf("a recycled PID was signalled: %+v", p.signals)
	}
	if !strings.Contains(result.Message, "no longer the process") {
		t.Errorf("message = %q, want the recycled identity named", result.Message)
	}
}

func TestStopRefusesWhenTheIdentityCannotBeReadAgain(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	delete(p.files, "/proc/999/stat")

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeRefused || len(p.signals) != 0 {
		t.Fatalf("outcome = %q with %d signals, want a refusal and none", result.Outcome, len(p.signals))
	}
	if !strings.Contains(result.Message, "identity could not be read again") {
		t.Errorf("message = %q, want the unreadable identity named", result.Message)
	}
}

func TestStopRefusesWhenThePortMovedToAnotherProcess(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	// The port is freed before the signal: the row the user confirmed is stale.
	p.files["/proc/net/tcp"] = []byte(procNetHeader)

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeRefused || len(p.signals) != 0 {
		t.Fatalf("outcome = %q with %d signals, want a refusal and none", result.Outcome, len(p.signals))
	}
	if !strings.Contains(result.Message, "no longer holds the ports") {
		t.Errorf("message = %q, want the stale facts named", result.Message)
	}
}

func TestStopStopsAndVerifiesThePortIsFree(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {
		if pid != 999 {
			t.Errorf("the signal went to pid %d, want 999", pid)
		}
		p.alive[999] = false
		p.files["/proc/net/tcp"] = []byte(procNetHeader)
	}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeStopped {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStopped, result.Message)
	}
	if !result.Verified {
		t.Error("Verified is false after a completed check")
	}
	if result.Alive {
		t.Error("Alive is true after the process exited")
	}
	if len(result.PortsStillHeld) != 0 {
		t.Errorf("PortsStillHeld = %+v, want none", result.PortsStillHeld)
	}
	if !strings.Contains(result.Message, "exited") || !strings.Contains(result.Message, "free") {
		t.Errorf("message = %q, want the exit and the free ports stated", result.Message)
	}
	if len(p.signals) != 1 || p.signals[0].pid != 999 || p.signals[0].sig != SignalTerminate {
		t.Fatalf("signals = %+v, want exactly one SIGTERM to 999", p.signals)
	}
}

func TestStopReportsAPortStillHeldByAnotherProcess(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)

	// The process exits, but something else takes the port before the fresh
	// reading: the outcome must not claim the port is free.
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {
		p.alive[999] = false
		p.links["/proc/999/fd/3"] = "socket:[888]"
		p.addDir("/proc", fakeEntry{name: "999", isDir: true}, fakeEntry{name: "555", isDir: true})
		p.addDir("/proc/555/fd", fakeEntry{name: "5"})
		p.addLink("/proc/555/fd/5", "socket:[777]")
	}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeStopped {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStopped, result.Message)
	}
	if len(result.PortsStillHeld) != 1 {
		t.Fatalf("PortsStillHeld = %+v, want the port that is still bound", result.PortsStillHeld)
	}
	if owner := result.PortsStillHeld[0].Owner; owner == nil || owner.PID != 555 {
		t.Errorf("port owner = %+v, want the new holder named", owner)
	}
	if !strings.Contains(result.Message, "still bound by pid 555") {
		t.Errorf("message = %q, want the new holder named", result.Message)
	}
}

func TestStopReportsAProcessStillRunningAfterTerminate(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	// The signal is accepted and the process ignores it.
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeStillRunning {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStillRunning, result.Message)
	}
	if !result.Alive {
		t.Error("Alive is false although the process ignored the signal")
	}
	if !strings.Contains(result.Message, "separate action") {
		t.Errorf("message = %q, want the forced action left as a separate decision", result.Message)
	}
	if len(p.signals) != 1 || p.signals[0].sig != SignalTerminate {
		t.Fatalf("signals = %+v, want one SIGTERM and no escalation", p.signals)
	}
}

func TestStopReportsAProcessStillRunningAfterTheForcedSignal(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalKill))

	if result.Outcome != OutcomeStillRunning || !result.Alive {
		t.Fatalf("outcome = %q, alive = %v; want still running", result.Outcome, result.Alive)
	}
	if len(p.signals) != 1 || p.signals[0].sig != SignalKill {
		t.Fatalf("signals = %+v, want one SIGKILL", p.signals)
	}
	if !strings.Contains(result.Message, "SIGKILL") {
		t.Errorf("message = %q, want the forced signal named", result.Message)
	}
}

func TestStopReportsPermissionDeniedWithoutEscalating(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.signalErr = fmt.Errorf("%w: operation not permitted", ErrPermissionDenied)

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomePermissionDenied {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomePermissionDenied)
	}
	if !strings.Contains(result.Message, "does not elevate") {
		t.Errorf("message = %q, want the no-escalation rule stated", result.Message)
	}
	if len(p.signals) != 1 {
		t.Errorf("signals = %+v, want the one refused attempt recorded", p.signals)
	}
}

func TestStopReportsAProcessThatExitedBeforeTheSignal(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.signalErr = fmt.Errorf("%w: no such process", ErrNoSuchProcess)

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeAlreadyGone {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeAlreadyGone)
	}
}

func TestStopReportsAFailedSignal(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.signalErr = errors.New("some other kernel refusal")

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeFailed)
	}
	if !strings.Contains(result.Message, "some other kernel refusal") {
		t.Errorf("message = %q, want the failure quoted", result.Message)
	}
}

func TestStopSignalsOneProcessOfASharedPort(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	// A second process holds the same port, which SO_REUSEPORT allows.
	p.alive[555] = true
	p.addDir("/proc/555/fd", fakeEntry{name: "5"})
	p.addLink("/proc/555/fd/5", "socket:[888]")
	p.addDir("/proc", fakeEntry{name: "999", isDir: true}, fakeEntry{name: "555", isDir: true})
	p.files["/proc/net/tcp"] = []byte(procNetHeader +
		procNetRow("0100007F", "0BB8", "0A", 1000, "777") +
		procNetRow("0100007F", "0BB8", "0A", 1000, "888"))

	snap := discoverFixture(p)
	if len(snap.Listeners) != 2 {
		t.Fatalf("the fixture produced %d rows, want 2", len(snap.Listeners))
	}
	target := snap.ListenersFor(999)
	if len(target) != 1 {
		t.Fatalf("pid 999 holds %d rows, want 1", len(target))
	}

	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {
		if pid == 999 {
			p.alive[999] = false
			p.files["/proc/net/tcp"] = []byte(procNetHeader + procNetRow("0100007F", "0BB8", "0A", 1000, "888"))
		}
	}

	result := Stop(context.Background(), p, StopTarget{
		Process: *target[0].Owner,
		Sockets: target,
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeStopped {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStopped, result.Message)
	}
	if len(p.signals) != 1 || p.signals[0].pid != 999 {
		t.Fatalf("signals = %+v, want exactly one, to the chosen pid", p.signals)
	}
	if len(result.PortsStillHeld) != 1 || result.PortsStillHeld[0].Owner == nil || result.PortsStillHeld[0].Owner.PID != 555 {
		t.Errorf("PortsStillHeld = %+v, want the other holder reported and untouched", result.PortsStillHeld)
	}
}

func TestStopUsesTheGraceDeadlineWithoutHanging(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.clockStep = 10 * time.Millisecond
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, StopOptions{Signal: SignalTerminate, Grace: 5 * time.Millisecond, Poll: time.Millisecond})

	if result.Outcome != OutcomeStillRunning {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeStillRunning)
	}
	if !strings.Contains(result.Message, "5 ms") {
		t.Errorf("message = %q, want the grace bound named", result.Message)
	}
}

func TestStopHonoursACancelledContext(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.clockStep = time.Millisecond
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := Stop(ctx, p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, StopOptions{Signal: SignalTerminate, Grace: time.Minute, Poll: time.Millisecond})

	if result.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeFailed, result.Message)
	}
	if !strings.Contains(result.Message, "context canceled") {
		t.Errorf("message = %q, want the cancellation named", result.Message)
	}
}

func TestStopOptionsDefaultsAreTheSoftOnes(t *testing.T) {
	opts := normalizeStopOptions(StopOptions{})
	if opts.Signal != SignalTerminate {
		t.Errorf("signal = %v, want the soft signal", opts.Signal)
	}
	if opts.Grace != DefaultStopGrace || opts.Poll != DefaultStopPoll {
		t.Errorf("bounds = %v/%v, want %v/%v", opts.Grace, opts.Poll, DefaultStopGrace, DefaultStopPoll)
	}
	if !SignalKill.Forced() || SignalTerminate.Forced() {
		t.Error("Forced reports the wrong signal as uncatchable")
	}
	if SignalKill.String() != "SIGKILL" || SignalTerminate.String() != "SIGTERM" {
		t.Errorf("signal names = %q, %q", SignalTerminate, SignalKill)
	}
}

func TestSameIdentityComparesTheTokensWhenBothAreKnown(t *testing.T) {
	shown := Process{PID: 42, token: "boot-1:100"}
	other := Process{PID: 42, token: "boot-1:200"}
	if shown.SameIdentity(other) {
		t.Error("two readings of different processes compared equal")
	}
	if !shown.SameIdentity(Process{PID: 42, token: "boot-1:100"}) {
		t.Error("two readings of one process compared different")
	}
	// Without a token the PID is all the platform gave, and that is what is
	// compared -- the confirmation screen says so before the user commits.
	if !shown.SameIdentity(Process{PID: 42}) {
		t.Error("a tokenless reading of the same pid compared different")
	}
	if shown.SameIdentity(Process{PID: 43, token: "boot-1:100"}) {
		t.Error("a different pid compared equal")
	}
}

func TestProcessStartedText(t *testing.T) {
	withTime := Process{StartedAt: time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)}
	if got := withTime.StartedText(); got != "2026-03-04 05:06:07" {
		t.Errorf("StartedText = %q, want the parsed time", got)
	}
	labelOnly := Process{StartedAt: time.Time{}, StartedLabel: "Thu Oct 16 11:21:00 2025"}
	if got := labelOnly.StartedText(); got != "Thu Oct 16 11:21:00 2025" {
		t.Errorf("StartedText = %q, want the platform's own wording", got)
	}
	if got := (Process{}).StartedText(); got != "" {
		t.Errorf("StartedText = %q, want an empty string", got)
	}
}

func TestListenerHelpers(t *testing.T) {
	v6 := Listener{Protocol: ProtocolTCP6, Address: "::", Port: 8080, Inode: "1"}
	if got := v6.Endpoint(); got != "[::]:8080" {
		t.Errorf("Endpoint = %q, want [::]:8080", got)
	}
	if !v6.Wildcard() || v6.Loopback() {
		t.Errorf("wildcard = %v, loopback = %v; want true, false", v6.Wildcard(), v6.Loopback())
	}
	v4 := Listener{Protocol: ProtocolTCP, Address: "127.0.0.1", Port: 3000, Inode: "2"}
	if got := v4.Endpoint(); got != "127.0.0.1:3000" {
		t.Errorf("Endpoint = %q, want 127.0.0.1:3000", got)
	}
	if !v4.Loopback() || v4.Wildcard() {
		t.Errorf("loopback = %v, wildcard = %v; want true, false", v4.Loopback(), v4.Wildcard())
	}
	if v4.Key() == v6.Key() {
		t.Error("two different sockets share an identity key")
	}
}

func TestSnapshotListenersForCountsEveryPortOfOneProcess(t *testing.T) {
	snap := Snapshot{Listeners: []Listener{
		{Protocol: ProtocolTCP, Address: "127.0.0.1", Port: 3000, Owner: &Process{PID: 7}},
		{Protocol: ProtocolTCP, Address: "127.0.0.1", Port: 3001, Owner: &Process{PID: 7}},
		{Protocol: ProtocolTCP, Address: "127.0.0.1", Port: 3002, Owner: &Process{PID: 8}},
		{Protocol: ProtocolUDP, Address: "0.0.0.0", Port: 3002},
	}}

	held := snap.ListenersFor(7)
	if len(held) != 2 {
		t.Fatalf("ListenersFor(7) = %+v, want the two ports pid 7 holds", held)
	}
	if len(snap.ListenersFor(9)) != 0 {
		t.Error("ListenersFor invented a row for a pid the snapshot does not mention")
	}
}
