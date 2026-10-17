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

// TestTheFlowFixtureAnswersTheDarwinReaderToo is the local half of the macOS
// fix: readDarwinProcess is untagged, so the very fixture the stop tests build
// (linuxHost) can be driven through the darwin reader here on Linux, and the
// facts must be the ones the /proc reader yields from the same fixture in
// TestDiscoverResolvesOwnerIdentityOnLinux. Without this, the ps bridge could
// only be validated by a macOS runner, and a typo in it would look like a
// platform failure instead of a fixture bug.
//
// The expected literals are the /proc reader's own answers, so the two dialects
// are pinned to one result rather than each being self-consistent: node,
// alber, /usr/bin/node, ps's start wording as the token, and the masked token
// argument. The start time is asserted as the instant btime plus the start
// ticks name, which is the same 1700000123 the /proc test asserts.
func TestTheFlowFixtureAnswersTheDarwinReaderToo(t *testing.T) {
	p := linuxHost(999, 3000, "777")

	proc := readDarwinProcess(context.Background(), p, 999, ScopeSelf)

	if !proc.IdentityKnown() {
		t.Fatal("the darwin reader got no identity token from the shared fixture")
	}
	if proc.Name != "node" {
		t.Errorf("name = %q, want node", proc.Name)
	}
	if proc.User != "alber" {
		t.Errorf("user = %q, want alber", proc.User)
	}
	if proc.Exe != "/usr/bin/node" {
		t.Errorf("exe = %q, want /usr/bin/node", proc.Exe)
	}
	if proc.StartedLabel == "" || proc.token != proc.StartedLabel {
		t.Errorf("token = %q with label %q; want ps's own start wording as the token", proc.token, proc.StartedLabel)
	}
	if got, want := proc.StartedAt.Unix(), int64(1700000123); got != want {
		t.Errorf("started = %d, want %d (boot time plus the start ticks)", got, want)
	}
	if got, want := strings.Join(proc.Args, " "), "node server.js --token ****"; got != want {
		t.Errorf("args = %q, want %q: a token must not survive into the reading", got, want)
	}

	// The snapshot's owner is read by the platform's compiled fact reader: on
	// Linux this comparison is genuinely between the /proc reader and the ps
	// bridge, and on macOS it re-affirms that the snapshot and the stop path
	// read one process the same way. Two facts are platform-shaped by D6: the
	// identity token's wording (Linux boot id plus start ticks, macOS ps's
	// lstart), and the start time's precision (10 ms ticks against lstart's whole
	// second). Every other fact must be identical.
	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	owner := listener.Owner
	if owner == nil {
		t.Fatalf("the snapshot resolved no owner to compare with: %q", listener.OwnerErr)
	}
	if owner.Name != proc.Name || owner.User != proc.User || owner.Exe != proc.Exe {
		t.Errorf("snapshot owner = %s/%s/%s, darwin reader = %s/%s/%s; want the same facts",
			owner.Name, owner.User, owner.Exe, proc.Name, proc.User, proc.Exe)
	}
	// StartedAt is compared to the second, not to the instant: the /proc reader
	// has 10 ms start ticks while ps's lstart has one-second granularity (D6),
	// so the same process is only pinnable to the second on both. The sub-second
	// remainder is a precision Linux has and macOS does not.
	if owner.StartedAt.Unix() != proc.StartedAt.Unix() {
		t.Errorf("snapshot started = %v, darwin started = %v; want the same second", owner.StartedAt, proc.StartedAt)
	}
	if strings.Join(owner.Args, " ") != strings.Join(proc.Args, " ") {
		t.Errorf("snapshot args = %q, darwin args = %q; want the same redacted argv", owner.Args, proc.Args)
	}
}

// TestTheFlowFixtureMovesTheDarwinIdentityWithTheStartTicks is the proof that
// the two readings a stop compares are both served by the darwin dialect: the
// recycled PID and the vanished start file are the two ways a fixture changes
// identity between two readings, and they must change it for the darwin reader
// exactly as they do for /proc. It is the local stand-in for the macOS runner's
// TestStopRefusesARecycledPID and TestStopRefusesWhenTheIdentityCannotBeReadAgain.
func TestTheFlowFixtureMovesTheDarwinIdentityWithTheStartTicks(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	shown := readDarwinProcess(context.Background(), p, 999, ScopeSelf)
	if !shown.IdentityKnown() {
		t.Fatal("the shared fixture gave the darwin reader no identity to start from")
	}

	// A live PID whose start ticks changed is a different incarnation, and the
	// start ticks are the only fact that changed.
	p.files["/proc/999/stat"] = []byte(procStatLine(999, 1, "python3", 99999))
	recycled := readDarwinProcess(context.Background(), p, 999, ScopeSelf)
	if !recycled.IdentityKnown() {
		t.Fatal("the recycled process produced no darwin identity token")
	}
	if shown.SameIdentity(recycled) {
		t.Errorf("the darwin reader compared %q and %q as one process", shown.token, recycled.token)
	}

	// A start file the fixture no longer has is an identity the darwin reader
	// must report as unreadable, not invent one for.
	delete(p.files, "/proc/999/stat")
	unreadable := readDarwinProcess(context.Background(), p, 999, ScopeSelf)
	if unreadable.IdentityKnown() {
		t.Errorf("the darwin reader invented the token %q for a process with no start file", unreadable.token)
	}
}
