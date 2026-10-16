package sshcfg

import (
	"context"
	"path/filepath"
	"testing"
)

// TestAManagerWithoutAnInjectedRunnerReportsInsteadOfPanicking covers what a
// Manager promises its callers: the runner is the seam the tests use to stay off
// the machine, but a Manager built without one is not a broken Manager -- it
// falls back to the real tool and reports what that tool said. The path here is
// one that does not exist, so the fallback fails without running anything, and a
// missing guard shows up as a panic rather than as a report.
func TestAManagerWithoutAnInjectedRunnerReportsInsteadOfPanicking(t *testing.T) {
	home := t.TempDir()
	paths := PathsFrom(home, func(string) string { return "" })
	// An advertised socket is what makes the code ask the agent at all: without one
	// it reports that no agent is advertised and never reaches a command.
	paths.AgentSocket = filepath.Join(home, "agent.sock")
	m := &Manager{Paths: paths}
	m.Caps = Caps{Add: filepath.Join(home, "bin", "ssh-add")}

	state := m.AgentState(context.Background())
	if state.Reachable {
		t.Fatalf("an agent that could not be asked is not reachable: %+v", state)
	}
	if state.Reason == "" {
		t.Fatalf("a missing ssh-add was not reported: %+v", state)
	}
}

// TestAnEntryWriteWithoutAnInjectedRunnerStillValidates covers the other place a
// write runs a command: the client is asked to parse the new file before that
// file replaces the old one. A Manager built without a runner falls back here
// too, and the outcome has to be a reported validation rather than a panic -- the
// entry is still written, with the reason it could not be validated, because a
// client that could not be run is not a reason to refuse a change the user asked
// for.
func TestAnEntryWriteWithoutAnInjectedRunnerStillValidates(t *testing.T) {
	home := t.TempDir()
	m := &Manager{Paths: PathsFrom(home, func(string) string { return "" })}
	m.Caps = Caps{SSH: filepath.Join(home, "bin", "ssh")}

	plan, err := m.PlanCreate(context.Background(), HostSpec{Alias: "work", HostName: "w.example"})
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	res, err := m.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Validated {
		t.Fatal("a client that could not be run did not validate anything")
	}
	if res.Validation == "" {
		t.Fatal("the write did not report why the new file was not parsed")
	}
}
