package updates

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// probeSeqRunner answers the n-th command it is asked with the n-th output, in
// order, so a guard can hold the two reads one adapter makes -- the simulation
// and the manager's hold list -- to different answers without depending on the
// argv those reads happen to use.
type probeSeqRunner struct {
	outs  []Output
	calls []Command
}

func (r *probeSeqRunner) Run(_ context.Context, cmd Command, _ func(string)) Output {
	index := len(r.calls)
	r.calls = append(r.calls, cmd)
	if index >= len(r.outs) {
		return Output{ExitCode: 127, Err: errors.New("the guard recorded no answer for " + QuoteArgv(cmd.Argv))}
	}
	return r.outs[index]
}

// TestAnUnreadableHoldListIsSaidOutLoud pins the honesty of a hold this package
// could not read. apt's and pkg's holds decide whether an entry is offered as
// movable, so a hold list that could not be read is a caveat about the list --
// and a silent one would offer an entry the manager is holding as if nothing
// were keeping it in place. pacman's ignore list already says so; this holds apt
// and pkg to the same rule.
func TestAnUnreadableHoldListIsSaidOutLoud(t *testing.T) {
	runner := &probeSeqRunner{outs: []Output{
		ok("Inst tar [1.35-1] (1.35-2 Debian:stable [amd64])\n"),
		failed("E: apt-mark: command failed"),
	}}
	c := &Center{
		Adapters: []Adapter{AptAdapter{}},
		Runner:   runner,
		Env:      testEnv("apt-get", "apt-mark"),
	}

	result := c.Read(context.Background(), ManagerApt)
	if result.Status != StatusUpdates {
		t.Fatalf("the read answered %q, want an update: %s", result.Status, result.Reason)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("the read ran %d commands, want the simulation and the hold list: %v", len(runner.calls), runner.calls)
	}
	for _, note := range result.Notes {
		if strings.Contains(strings.ToLower(note), "hold") {
			return
		}
	}
	t.Errorf("apt's hold list could not be read and the answer says nothing about it, so an entry apt is holding is offered as movable: notes=%q",
		result.Notes)
}

// TestAnUnreadableHoldListDoesNotClaimThereAreNoHolds is the other half: the note
// must not become a claim in the other direction. A machine without apt-mark has
// no holds this package can read at all, and saying "apt is holding nothing" would
// be inventing an answer.
func TestAnUnreadableHoldListDoesNotClaimThereAreNoHolds(t *testing.T) {
	runner := &probeSeqRunner{outs: []Output{
		ok("Inst tar [1.35-1] (1.35-2 Debian:stable [amd64])\n"),
	}}
	c := &Center{
		Adapters: []Adapter{AptAdapter{}},
		Runner:   runner,
		Env:      testEnv("apt-get"),
	}

	result := c.Read(context.Background(), ManagerApt)
	for _, note := range result.Notes {
		if strings.Contains(strings.ToLower(note), "no holds") || strings.Contains(note, "nothing is held") {
			t.Errorf("a hold list that could not be read is reported as a hold list with nothing in it: %q", note)
		}
	}
}
