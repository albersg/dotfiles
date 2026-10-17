package portscan

import (
	"context"
	"strings"
	"testing"
)

func TestDiscoverDarwinReadsLsofAndEnrichesWithPS(t *testing.T) {
	p := newFakeProbe("darwin")
	p.alive[1234] = true
	p.users[501] = "alber"
	p.addTool("lsof")
	p.addTool("ps")
	p.addCommand("lsof", []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcuLn"}, CommandResult{
		Stdout: []byte("p1234\ncnode\nu501\nLalber\nn127.0.0.1:3000\n"),
	})
	p.addCommand("lsof", []string{"-nP", "-iUDP", "-F", "pcuLn"}, CommandResult{
		Stdout: []byte("p1234\ncnode\nu501\nLalber\nn*:5353\n"),
	})
	p.addCommand("ps", []string{"-o", "pid=,lstart=,user=,comm=", "-p", "1234"}, CommandResult{
		Stdout: []byte(" 1234 Thu Oct 16 11:21:00 2025 alber /usr/local/bin/node\n"),
	})
	p.addCommand("ps", []string{"-o", "pid=,command=", "-p", "1234"}, CommandResult{
		Stdout: []byte(" 1234 /usr/local/bin/node server.js --api-key abc123\n"),
	})

	snap := discoverFixture(p)
	if len(snap.Listeners) != 2 {
		t.Fatalf("got %d listeners, want 2: %+v", len(snap.Listeners), snap.Listeners)
	}
	byPort := make(map[int]Listener, len(snap.Listeners))
	for _, l := range snap.Listeners {
		byPort[l.Port] = l
	}

	tcp := byPort[3000]
	if tcp.Protocol != ProtocolTCP || tcp.Address != "127.0.0.1" || tcp.State != StateListen || tcp.Source != "lsof" {
		t.Errorf("tcp row = %+v, want tcp 127.0.0.1:3000 LISTEN from lsof", tcp)
	}
	udp := byPort[5353]
	if udp.Protocol != ProtocolUDP || udp.State != StateUnconn {
		t.Errorf("udp row = %+v, want a bound UDP receiver", udp)
	}

	if tcp.Owner == nil {
		t.Fatal("the owner was not resolved from lsof")
	}
	owner := *tcp.Owner
	if owner.PID != 1234 || owner.Name != "node" || owner.User != "alber" {
		t.Errorf("owner = %+v, want pid 1234, node, alber", owner)
	}
	if owner.Exe != "/usr/local/bin/node" {
		t.Errorf("exe = %q, want the executable ps reported", owner.Exe)
	}
	if got := strings.Join(owner.Args, " "); !strings.Contains(got, "--api-key ****") {
		t.Errorf("args = %q, want the api key masked", got)
	}
	if owner.StartedLabel != "Thu Oct 16 11:21:00 2025" {
		t.Errorf("started label = %q, want ps's own wording", owner.StartedLabel)
	}
	if !owner.IdentityKnown() {
		t.Error("the owner carries no identity token, so a stop could not be revalidated")
	}
	if owner.StartedAt.IsZero() {
		t.Error("the start time was not parsed from ps's format")
	}
}

func TestDiscoverDarwinFallsBackToNetstatWithoutLsof(t *testing.T) {
	p := newFakeProbe("darwin")
	p.addTool("netstat")
	p.addCommand("netstat", []string{"-anv", "-p", "tcp"}, CommandResult{
		Stdout: []byte("Proto Recv-Q Send-Q  Local Address          Foreign Address        (state)\n" +
			"tcp4       0      0  127.0.0.1.3000         *.*                    LISTEN\n"),
	})
	p.addCommand("netstat", []string{"-anv", "-p", "udp"}, CommandResult{Stdout: []byte("Proto\n")})

	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	if listener.Source != "netstat" || listener.Port != 3000 {
		t.Errorf("listener = %+v, want the netstat row on 3000", listener)
	}
	if listener.Owner != nil {
		t.Errorf("an owner was invented from a reader that does not name one: %+v", listener.Owner)
	}
	if joined := strings.Join(snap.Limits, " "); !strings.Contains(joined, "does not name the process") {
		t.Errorf("limits = %q, want the reason the owner column is missing", snap.Limits)
	}
}

func TestDiscoverDarwinSaysWhenTheIdentityCannotBeRead(t *testing.T) {
	p := newFakeProbe("darwin")
	p.addTool("lsof")
	// ps is deliberately absent, so the identity can be named but not re-read.
	p.addCommand("lsof", []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcuLn"}, CommandResult{
		Stdout: []byte("p1234\ncnode\nLalber\nn127.0.0.1:3000\n"),
	})
	p.addCommand("lsof", []string{"-nP", "-iUDP", "-F", "pcuLn"}, CommandResult{Stdout: []byte("")})

	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	if listener.Owner == nil || listener.Owner.PID != 1234 {
		t.Fatalf("owner = %+v, want the pid lsof reported", listener.Owner)
	}
	if listener.Owner.IdentityKnown() {
		t.Error("an identity token was invented without ps")
	}
	if joined := strings.Join(snap.Limits, " "); !strings.Contains(joined, "identity cannot be revalidated") {
		t.Errorf("limits = %q, want the revalidation limitation named", snap.Limits)
	}
}

// TestDarwinIdentityIsTheRawPSLstartString pins D6 on the platform that has no
// boot id and no /proc: the identity token is ps's own lstart wording, compared
// as a string and never re-derived. It runs on Linux because the reader is
// portable Go over the Probe and the fake probe reports darwin, which is the
// same seam the snapshot uses to exercise the macOS reader here at all. The
// second reading goes through currentProcessIdentity, so the comparison the
// stop path makes is what is pinned, not a helper beside it.
func TestDarwinIdentityIsTheRawPSLstartString(t *testing.T) {
	const started = "Thu Oct 16 11:21:00 2025"

	p := newFakeProbe("darwin")
	p.addTool("ps")
	p.addCommand("ps", []string{"-o", "pid=,lstart=,user=,comm=", "-p", "1234"}, CommandResult{
		Stdout: []byte(" 1234 " + started + " alber /usr/local/bin/node\n"),
	})
	p.addCommand("ps", []string{"-o", "pid=,command=", "-p", "1234"}, CommandResult{
		Stdout: []byte(" 1234 /usr/local/bin/node server.js --api-key abc123\n"),
	})

	proc := readDarwinProcess(context.Background(), p, 1234, ScopeSelf)
	if !proc.IdentityKnown() {
		t.Fatal("the darwin reader produced no identity token")
	}
	if proc.token != started {
		t.Errorf("identity token = %q, want ps's own lstart string %q", proc.token, started)
	}
	if proc.StartedLabel != started {
		t.Errorf("started label = %q, want %q", proc.StartedLabel, started)
	}
	if proc.StartedAt.IsZero() {
		t.Error("the start time was not parsed from ps's format")
	}
	if proc.Name != "node" {
		t.Errorf("name = %q, want the executable's own name node", proc.Name)
	}
	if got := strings.Join(proc.Args, " "); !strings.Contains(got, "--api-key ****") {
		t.Errorf("args = %q, want the api key masked", got)
	}

	// A second ps reading with the same lstart is the same process, which is
	// the only revalidation this platform can offer: no boot id exists to
	// compare, so the token is the platform's wording and nothing more.
	current, known := currentProcessIdentity(context.Background(), p, proc)
	if !known {
		t.Fatal("the second darwin reading carried no identity token")
	}
	if !proc.SameIdentity(current) {
		t.Errorf("the same lstart did not compare equal: %q vs %q", proc.token, current.token)
	}
}
