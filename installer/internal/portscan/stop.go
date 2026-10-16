package portscan

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The stop path. It is written as a single ordered sequence -- refusal, liveness,
// identity, ports, signal, verification -- because each step narrows what the
// next one is allowed to believe, and a step that is skipped is a stop that
// could signal the wrong process or claim an outcome it did not check.

// StopOptions are the bounds one stop runs under.
type StopOptions struct {
	// Signal is the signal to send. The zero value is the soft one.
	Signal Signal
	// Grace is how long the process is given to exit after the signal; the
	// default is DefaultStopGrace.
	Grace time.Duration
	// Poll is how often liveness is asked during the grace period; the default
	// is DefaultStopPoll. A zero poll asks once and reports the first answer,
	// which is what a test with a fake clock wants.
	Poll time.Duration
}

const (
	// DefaultStopGrace is how long a process is given to exit after SIGTERM
	// before the outcome is reported as "still running".
	DefaultStopGrace = 5 * time.Second
	// DefaultStopPoll is how often liveness is re-asked inside that window.
	DefaultStopPoll = 150 * time.Millisecond
)

// Outcome names what a stop attempt came to. The wording is the screen's own:
// each value is a different sentence, and none of them claims more than the
// checks established.
type Outcome string

const (
	// OutcomeStopped means the process no longer exists after the signal.
	OutcomeStopped Outcome = "stopped"
	// OutcomeStillRunning means it was still alive when the grace period ended.
	OutcomeStillRunning Outcome = "still running"
	// OutcomeAlreadyGone means it had exited before the signal was sent.
	OutcomeAlreadyGone Outcome = "already gone"
	// OutcomePermissionDenied means the kernel refused the signal.
	OutcomePermissionDenied Outcome = "permission denied"
	// OutcomeRefused means the stop was refused by this package's own rules --
	// protection, scope, a changed identity, or a port the process no longer
	// holds -- and nothing was sent.
	OutcomeRefused Outcome = "refused"
	// OutcomeFailed means the attempt could not be completed or confirmed.
	OutcomeFailed Outcome = "failed"
)

// StopResult is the whole account of one attempt: what was sent, what the
// process did afterwards, whether the answer could be confirmed, and which of
// the shown ports are still bound by anyone.
type StopResult struct {
	// Outcome is what happened, and Signal is what was sent.
	Outcome Outcome
	Signal  Signal
	// Message is the sentence the screen shows.
	Message string
	// Alive is the process's liveness after the attempt, as far as it could be
	// established. A refusal that sent nothing still reports the liveness and
	// the ports, so the screen never states a fact it did not check.
	Alive bool
	// PortsStillHeld are the shown ports that the fresh reading still finds
	// bound, by this process or by any other.
	PortsStillHeld []Listener
	// Verified reports whether a signal was sent and the follow-up check
	// completed. It is false for every refusal, because nothing was sent, and
	// false when liveness could not be asked.
	Verified bool
}

// Stop sends one signal to the one process in the target, after revalidating
// that it is still the same process holding the same ports, and then checks
// whether it exited and whether the ports are still bound. It never kills by
// name, never iterates a port's processes, and never escalates its privileges:
// every one of those would be a different action than the one the user
// confirmed.
func Stop(ctx context.Context, p Probe, target StopTarget, opts StopOptions) StopResult {
	if p == nil {
		p = DefaultProbe()
	}
	opts = normalizeStopOptions(opts)

	proc := target.Process
	result := StopResult{Signal: opts.Signal}

	if refusal := StopRefusal(p, target); refusal != "" {
		result.Outcome = OutcomeRefused
		result.Message = refusal
		result = observeAfter(ctx, p, target, result)
		return result
	}

	alive, err := p.Alive(proc.PID)
	if err != nil {
		result.Outcome = OutcomeFailed
		result.Message = "whether pid " + strconv.Itoa(proc.PID) + " still exists could not be asked (" + err.Error() +
			"), so nothing was sent. Refresh and try again."
		return result
	}
	if !alive {
		result.Outcome = OutcomeAlreadyGone
		result.Message = "pid " + strconv.Itoa(proc.PID) + " had already exited before anything was sent, so nothing was stopped."
		result = observeAfter(ctx, p, target, result)
		return result
	}

	current, known := currentProcessIdentity(ctx, p, proc)
	if !known {
		result.Outcome = OutcomeRefused
		result.Message = "pid " + strconv.Itoa(proc.PID) + "'s identity could not be read again, so this run cannot tell " +
			"whether it is still the process the screen showed and nothing was sent. Refresh and try again."
		return result
	}
	if !proc.SameIdentity(current) {
		result.Outcome = OutcomeRefused
		result.Message = "pid " + strconv.Itoa(proc.PID) + " is no longer the process the screen showed (it now reports " +
			describeProcess(current) + "); nothing was sent. Refresh and choose the process you meant."
		result = observeAfter(ctx, p, target, result)
		return result
	}

	fresh := Discover(ctx, p)
	if matched := stillHeld(fresh, proc, target.Sockets); len(matched) == 0 {
		result.Outcome = OutcomeRefused
		result.PortsStillHeld = portsStillHeld(fresh, target.Sockets)
		result.Message = "pid " + strconv.Itoa(proc.PID) + " no longer holds the ports this screen showed, so nothing was sent: " +
			"the facts on screen were stale. Refresh to see who holds them now."
		return result
	}

	if err := p.Signal(proc.PID, opts.Signal); err != nil {
		switch {
		case errors.Is(err, ErrNoSuchProcess):
			result.Outcome = OutcomeAlreadyGone
			result.Message = "pid " + strconv.Itoa(proc.PID) + " exited between the check and the signal, so nothing was stopped."
		case errors.Is(err, ErrPermissionDenied):
			result.Outcome = OutcomePermissionDenied
			result.Message = "the kernel refused " + opts.Signal.String() + " to pid " + strconv.Itoa(proc.PID) +
				": the process belongs to another account. This run does not elevate its privileges to change that, so " +
				"nothing was stopped. Stop it from an account that owns it."
		default:
			result.Outcome = OutcomeFailed
			result.Message = opts.Signal.String() + " to pid " + strconv.Itoa(proc.PID) + " failed: " + err.Error() + "."
		}
		result = observeAfter(ctx, p, target, result)
		return result
	}

	deadline := p.Now().Add(opts.Grace)
	aliveAfter, waitErr := waitForExit(ctx, p, proc.PID, deadline, opts.Poll)
	result.Alive = aliveAfter

	after := Discover(ctx, p)
	result.PortsStillHeld = portsStillHeld(after, target.Sockets)
	blind := blindTo(after, target.Sockets)

	switch {
	case waitErr != nil:
		result.Outcome = OutcomeFailed
		result.Message = opts.Signal.String() + " was sent to pid " + strconv.Itoa(proc.PID) + ", but whether it exited could " +
			"not be confirmed (" + waitErr.Error() + "). " + portsSentence(result.PortsStillHeld, blind)
	case !aliveAfter:
		// The check completed only as far as it could: a port is reported free
		// when a reader that covers it answered, and the sentence says which
		// transports it could not ask about.
		result.Verified = len(blind) == 0
		result.Outcome = OutcomeStopped
		result.Message = "pid " + strconv.Itoa(proc.PID) + " (" + processLabel(proc) + ") exited. " +
			portsSentence(result.PortsStillHeld, blind)
	case !opts.Signal.Forced():
		result.Verified = true
		result.Outcome = OutcomeStillRunning
		result.Message = "pid " + strconv.Itoa(proc.PID) + " (" + processLabel(proc) + ") was still running after " +
			formatDuration(opts.Grace) + " of SIGTERM. A forced stop is a separate action."
	default:
		result.Verified = true
		result.Outcome = OutcomeStillRunning
		result.Message = "pid " + strconv.Itoa(proc.PID) + " (" + processLabel(proc) + ") was still running after SIGKILL; " +
			"it may be stuck in the kernel."
	}
	return result
}

// Forced reports whether the signal cannot be caught. It is what makes the
// forced stop a second, differently-worded action rather than the tail of the
// soft one.
func (s Signal) Forced() bool { return s == SignalKill }

// observeAfter reports the liveness and the ports after an attempt that sent
// nothing or whose signal failed. It is what keeps a refusal to stop from
// becoming a claim that nothing changed.
func observeAfter(ctx context.Context, p Probe, target StopTarget, result StopResult) StopResult {
	if alive, err := p.Alive(target.Process.PID); err == nil {
		result.Alive = alive
	}
	after := Discover(ctx, p)
	result.PortsStillHeld = portsStillHeld(after, target.Sockets)
	return result
}

// stillHeld returns the listeners in a fresh reading that are still the ports
// the screen showed AND are held by the process the user chose. Both halves
// matter: a port that moved to another process is not this process's port any
// more, and stopping on the strength of the old row would be a stop by port.
func stillHeld(fresh Snapshot, proc Process, shown []Listener) []Listener {
	if len(shown) == 0 {
		return fresh.ListenersFor(proc.PID)
	}

	wanted := make(map[string]bool, len(shown))
	for _, l := range shown {
		wanted[l.Key()] = true
	}

	var matched []Listener
	for _, l := range fresh.ListenersFor(proc.PID) {
		if wanted[l.Key()] {
			matched = append(matched, l)
		}
	}
	return matched
}

// portsStillHeld returns the ports the fresh reading still finds bound,
// whoever holds them now. It compares the endpoint rather than the socket
// identity: the question after an attempt is whether the port is occupied, and
// a port that moved to another process is still occupied.
func portsStillHeld(fresh Snapshot, shown []Listener) []Listener {
	if len(shown) == 0 {
		return nil
	}
	wanted := make(map[string]bool, len(shown))
	for _, l := range shown {
		wanted[portKey(l)] = true
	}

	var held []Listener
	for _, l := range fresh.Listeners {
		if wanted[portKey(l)] {
			held = append(held, l)
		}
	}
	return held
}

// portKey is the endpoint a port is occupied at: the protocol, the address and
// the port. It is what makes "is this port still busy?" a question about the
// port rather than about the socket that used to hold it.
func portKey(l Listener) string {
	return string(l.Protocol) + "|" + l.Address + "|" + strconv.Itoa(l.Port)
}

// stillHeldSentence describes the ports a fresh reading still found bound, so
// a stopped process never reads as "the port is free" when it is not.
func stillHeldSentence(held []Listener) string {
	if len(held) == 0 {
		return "The ports it held are free."
	}

	parts := make([]string, 0, len(held))
	for _, l := range held {
		owner := "another process this run cannot name"
		if l.Owner != nil {
			owner = "pid " + strconv.Itoa(l.Owner.PID)
			if l.Owner.Name != "" {
				owner += " (" + l.Owner.Name + ")"
			}
		}
		parts = append(parts, l.Protocol.Transport()+"/"+strconv.Itoa(l.Port)+" is still bound by "+owner)
	}
	return "The fresh reading still finds " + strings.Join(parts, ", ") + "."
}

// portsSentence is the post-attempt sentence about the ports: what the fresh
// reading found still bound and what it could not answer for. The two are
// different facts and both are stated, because a port is only free when a
// reader that covers it said so -- a reading whose readers failed is not
// evidence of a free port, and calling one free on its strength is the false
// positive this utility must not produce.
func portsSentence(held []Listener, blind []string) string {
	switch {
	case len(blind) == 0:
		return stillHeldSentence(held)
	case len(held) == 0:
		return "Whether the ports it held are free could not be checked: the fresh reading did not answer for its " +
			strings.Join(blind, " and ") + " sockets."
	default:
		return stillHeldSentence(held) + " Whether the rest are free could not be checked: the fresh reading did not " +
			"answer for its " + strings.Join(blind, " and ") + " sockets."
	}
}

// blindTo names the transports a fresh reading could not answer for, out of
// those the screen showed. It is what keeps the verification from describing a
// reading that never happened as a clean bill of health.
func blindTo(fresh Snapshot, shown []Listener) []string {
	var blind []string
	seen := make(map[string]bool, 2)
	for _, l := range shown {
		transport := l.Protocol.Transport()
		if seen[transport] {
			continue
		}
		seen[transport] = true
		if !answeredFor(fresh, transport) {
			blind = append(blind, transport)
		}
	}
	return blind
}

// answeredFor reports whether at least one reader that completed without an
// error can speak for the ports of one transport. A reader names itself after
// what it read -- /proc/net/tcp, /proc/net/udp, lsof TCP, lsof UDP, ss, netstat
// -- so a reader whose own name names the other transport cannot stand in for
// this one, while a reader that names no transport read the whole table.
func answeredFor(fresh Snapshot, transport string) bool {
	other := "udp"
	if transport == "UDP" {
		other = "tcp"
	}
	for _, src := range fresh.Sources {
		if src.Err != "" {
			continue
		}
		if strings.Contains(strings.ToLower(src.Name), other) {
			continue
		}
		return true
	}
	return false
}

// waitForExit reports whether the process is still running, asking the kernel
// until it stops being so, the caller's grace period runs out, or the context is
// cancelled. A poll of zero asks once and returns the first answer.
func waitForExit(ctx context.Context, p Probe, pid int, deadline time.Time, poll time.Duration) (bool, error) {
	for {
		alive, err := processRunning(ctx, p, pid)
		if err != nil {
			return true, err
		}
		if !alive {
			return false, nil
		}
		if poll <= 0 || !p.Now().Before(deadline) {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-time.After(poll):
		}
	}
}

// processRunning reports whether a PID is a process that could still hold a
// port. It is deliberately not the same question as the probe's Alive: a
// process that exited but has not been reaped by its parent is a zombie, and to
// the kernel kill(pid, 0) answers yes for it. A zombie holds no sockets and can
// never run again, so a stop that reported "still running" for one would be the
// false positive this utility cannot afford.
func processRunning(ctx context.Context, p Probe, pid int) (bool, error) {
	alive, err := p.Alive(pid)
	if err != nil {
		return false, err
	}
	if !alive {
		return false, nil
	}

	switch p.GOOS() {
	case "linux":
		if state, ok := linuxProcessState(p, pid); ok && state == "Z" {
			return false, nil
		}
	case "darwin":
		if state, ok := darwinProcessState(ctx, p, pid); ok && strings.HasPrefix(state, "Z") {
			return false, nil
		}
	}
	return true, nil
}

// linuxProcessState reads the state letter out of /proc/<pid>/stat, which is
// field 3 and therefore the first field after the command's closing parenthesis.
func linuxProcessState(p Probe, pid int) (string, bool) {
	data, err := p.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", false
	}
	text := string(data)
	closing := strings.LastIndex(text, ")")
	if closing < 0 {
		return "", false
	}
	fields := strings.Fields(text[closing+1:])
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}

// darwinProcessState asks ps for the process's state letter. An empty answer is
// a process ps could not find, which the caller reads as not running.
func darwinProcessState(ctx context.Context, p Probe, pid int) (string, bool) {
	ps, err := p.LookPath("ps")
	if err != nil {
		return "", false
	}
	callCtx, cancel := context.WithTimeout(ctx, darwinReaderTimeout)
	defer cancel()
	result, err := p.Run(callCtx, ps, []string{"-o", "state=", "-p", strconv.Itoa(pid)}, []string{"LC_ALL=C"})
	if err != nil {
		return "", false
	}
	state := strings.TrimSpace(string(result.Stdout))
	return state, state != ""
}

// currentProcessIdentity reads the process again, by the same platform path the
// snapshot used, so the comparison is between two readings of one platform and
// not between a reading and an assumption.
func currentProcessIdentity(ctx context.Context, p Probe, proc Process) (Process, bool) {
	switch p.GOOS() {
	case "linux":
		current := readProcProcess(p, proc.PID, proc.Scope, readBootFacts(p))
		return current, current.token != ""
	case "darwin":
		ps, err := p.LookPath("ps")
		if err != nil {
			return Process{PID: proc.PID, Scope: proc.Scope}, false
		}
		details, err := darwinPSIdentity(ctx, p, ps, []int{proc.PID})
		if err != nil {
			return Process{PID: proc.PID, Scope: proc.Scope}, false
		}
		detail, ok := details[proc.PID]
		if !ok || detail.started == "" {
			return Process{PID: proc.PID, Scope: proc.Scope}, false
		}
		return Process{
			PID:          proc.PID,
			Name:         filepath.Base(detail.comm),
			User:         detail.user,
			Exe:          detail.comm,
			StartedLabel: detail.started,
			Scope:        proc.Scope,
			token:        detail.started,
		}, true
	default:
		return Process{PID: proc.PID, Scope: proc.Scope}, false
	}
}

// normalizeStopOptions fills the defaults and clamps a negative bound.
func normalizeStopOptions(opts StopOptions) StopOptions {
	if opts.Grace <= 0 {
		opts.Grace = DefaultStopGrace
	}
	if opts.Poll <= 0 {
		opts.Poll = DefaultStopPoll
	}
	return opts
}

// describeProcess is the short identity a refusal quotes so the user can see
// which process the PID now names.
func describeProcess(proc Process) string {
	label := processLabel(proc)
	if started := proc.StartedText(); started != "" {
		return label + ", started " + started
	}
	return label
}

// processLabel is a process's name when it has one and its bare PID otherwise.
func processLabel(proc Process) string {
	if proc.Name != "" {
		return proc.Name
	}
	return "pid " + strconv.Itoa(proc.PID)
}

// formatDuration renders a bound the way a sentence reads: whole seconds below
// a minute, and a plain millisecond count below that.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return strconv.FormatInt(d.Milliseconds(), 10) + " ms"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d.Seconds()), 10) + " s"
	}
	return d.Round(100 * time.Millisecond).String()
}
