package portscan

import (
	"strings"
	"testing"
	"time"
)

func TestDiscoverResolvesOwnerIdentityOnLinux(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	snap := discoverFixture(p)

	if snap.Scope != ScopeSelf {
		t.Errorf("scope = %q, want %q", snap.Scope, ScopeSelf)
	}
	listener := soleListener(t, snap)
	if listener.Protocol != ProtocolTCP || listener.State != StateListen {
		t.Errorf("listener = %+v, want a TCP LISTEN row", listener)
	}
	if listener.Address != "127.0.0.1" || listener.Port != 3000 {
		t.Errorf("endpoint = %s, want 127.0.0.1:3000", listener.Endpoint())
	}
	if listener.Source != "/proc/net/tcp" {
		t.Errorf("source = %q, want /proc/net/tcp", listener.Source)
	}
	if listener.Owner == nil {
		t.Fatalf("owner was not resolved: %q", listener.OwnerErr)
	}
	owner := *listener.Owner
	if owner.PID != 999 || owner.Name != "node" || owner.User != "alber" {
		t.Errorf("owner = %+v, want pid 999, node, alber", owner)
	}
	if owner.Exe != "/usr/bin/node" {
		t.Errorf("exe = %q, want /usr/bin/node", owner.Exe)
	}
	if got, want := strings.Join(owner.Args, " "), "node server.js --token ****"; got != want {
		t.Errorf("args = %q, want %q: a token must not survive into the reading", got, want)
	}
	if owner.Scope != ScopeSelf {
		t.Errorf("owner scope = %q, want %q", owner.Scope, ScopeSelf)
	}
	if !owner.IdentityKnown() {
		t.Error("the owner carries no identity token, so a stop could not be revalidated")
	}
	if got, want := owner.StartedAt.Unix(), int64(1700000123); got != want {
		t.Errorf("started = %d, want %d (boot time plus the start ticks)", got, want)
	}
	if snap.At.IsZero() {
		t.Error("the snapshot carries no time")
	}
	if !listener.Loopback() || listener.Wildcard() {
		t.Errorf("loopback = %v, wildcard = %v; want true, false for 127.0.0.1", listener.Loopback(), listener.Wildcard())
	}
}

func TestDiscoverListsIPv6AndUDPWithoutMixingTheirStates(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.addFile("/proc/net/tcp6", procNetHeader+procNetRow("00000000000000000000000001000000", "1F90", "0A", 1000, "888"))
	p.addFile("/proc/net/udp", procNetHeader+procNetRow("00000000", "14E9", "07", 1000, "999"))

	snap := discoverFixture(p)
	if len(snap.Listeners) != 3 {
		t.Fatalf("got %d listeners, want 3: %+v", len(snap.Listeners), snap.Listeners)
	}

	// The snapshot is ordered by port, so 3000 (tcp), 5353 (udp) and 8080 (tcp6).
	byPort := make(map[int]Listener, len(snap.Listeners))
	for _, l := range snap.Listeners {
		byPort[l.Port] = l
	}

	ipv6, ok := byPort[8080]
	if !ok || ipv6.Protocol != ProtocolTCP6 || ipv6.Address != "::1" || ipv6.State != StateListen {
		t.Errorf("the IPv6 listener = %+v, want tcp6 ::1 LISTEN", ipv6)
	}
	udp, ok := byPort[5353]
	if !ok || udp.Protocol != ProtocolUDP || udp.State != StateUnconn {
		t.Errorf("the UDP socket = %+v, want udp UNCONN: a UDP socket is not a listener", udp)
	}
}

func TestDiscoverReportsAnUnresolvableOwnerWithoutInventingOne(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	// The socket's owner lives in another account: reading its file descriptors
	// is what the kernel refuses.
	p.dirErr["/proc/999/fd"] = permissionDenied("/proc/999/fd")

	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	if listener.Owner != nil {
		t.Fatalf("an owner was invented: %+v", listener.Owner)
	}
	if !strings.Contains(listener.OwnerErr, "privileges") {
		t.Errorf("owner reason = %q, want the privilege limitation named", listener.OwnerErr)
	}
	joined := strings.Join(snap.Limits, " ")
	if !strings.Contains(joined, "no readable owner") {
		t.Errorf("limits = %q, want the missing-owner limitation", snap.Limits)
	}
	if listener.Port != 3000 || listener.State != StateListen {
		t.Errorf("the socket itself was lost: %+v", listener)
	}
}

func TestDiscoverFallsBackToSSWhenProcNetIsUnreadable(t *testing.T) {
	p := newFakeProbe("linux")
	p.users[1000] = "alber"
	p.alive[1234] = true
	p.addFile("/proc/stat", "btime 1700000000\n")
	p.addFile("/proc/sys/kernel/random/boot_id", "boot-1\n")
	p.addProcProcess(procFacts{
		pid:        1234,
		ppid:       1,
		name:       "cupsd",
		args:       []string{"cupsd", "--password", "hunter2"},
		uid:        1000,
		startTicks: 42,
	})
	p.answerPSFromProc()
	p.addDir("/proc", fakeEntry{name: "1234", isDir: true})
	// /proc/net is not readable at all.
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		p.fileErr[name] = permissionDenied(name)
	}
	p.addTool("ss")
	p.addCommand("ss", []string{"-H", "-tulpn"}, CommandResult{
		Stdout: []byte("tcp   LISTEN 0      4096   127.0.0.1:631    0.0.0.0:*    users:((\"cupsd\",pid=1234,fd=7))\n"),
	})

	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	if listener.Source != "ss" {
		t.Errorf("source = %q, want ss", listener.Source)
	}
	if listener.Owner == nil || listener.Owner.PID != 1234 {
		t.Fatalf("owner = %+v, want pid 1234 from ss", listener.Owner)
	}
	if listener.Owner.Name != "cupsd" || listener.Owner.User != "alber" {
		t.Errorf("owner = %+v, want the name from ss and the account from /proc", listener.Owner)
	}
	if got := strings.Join(listener.Owner.Args, " "); !strings.Contains(got, "--password ****") {
		t.Errorf("args = %q, want the password masked", got)
	}
	if !listener.Owner.IdentityKnown() {
		t.Error("the enriched owner carries no identity token")
	}
	if len(snap.Sources) == 0 {
		t.Error("the snapshot names no source, so the reader that failed is invisible")
	}
}

func TestDiscoverFallsBackToNetstatWhenSSIsMissing(t *testing.T) {
	p := newFakeProbe("linux")
	p.addTool("netstat")
	p.addCommand("netstat", []string{"-tulpn"}, CommandResult{
		Stdout: []byte("tcp        0      0 127.0.0.1:631           0.0.0.0:*               LISTEN      1234/cupsd\n"),
	})
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		p.fileErr[name] = permissionDenied(name)
	}

	snap := discoverFixture(p)
	listener := soleListener(t, snap)
	if listener.Source != "netstat" || listener.Port != 631 {
		t.Errorf("listener = %+v, want the netstat row on 631", listener)
	}
	if listener.Owner == nil || listener.Owner.PID != 1234 || listener.Owner.Name != "cupsd" {
		t.Errorf("owner = %+v, want pid 1234 cupsd from netstat", listener.Owner)
	}
}

func TestDiscoverSaysSoWhenNoReaderIsAvailable(t *testing.T) {
	p := newFakeProbe("linux")
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		p.fileErr[name] = permissionDenied(name)
	}

	snap := discoverFixture(p)
	if len(snap.Listeners) != 0 {
		t.Fatalf("a reading with no reader produced %+v", snap.Listeners)
	}
	joined := strings.Join(snap.Limits, " ")
	if !strings.Contains(joined, "no socket could be listed") {
		t.Errorf("limits = %q, want the no-reader sentence", snap.Limits)
	}
	for _, source := range snap.Sources {
		if source.Name == "ss" && source.Err == "" {
			t.Errorf("source %q has no failure reason", source.Name)
		}
	}
}

func TestDiscoverNamesTheWSLCuestScope(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.wsl = true

	snap := discoverFixture(p)
	if snap.Scope != ScopeWSLGuest {
		t.Errorf("scope = %q, want %q", snap.Scope, ScopeWSLGuest)
	}
	if owner := soleListener(t, snap).Owner; owner == nil || owner.Scope != ScopeWSLGuest {
		t.Errorf("owner scope = %+v, want WSL guest: a PID only means something inside one kernel", owner)
	}
}

func TestDiscoverRefusesAPlatformItCannotRead(t *testing.T) {
	p := newFakeProbe("plan9")

	snap := discoverFixture(p)
	if len(snap.Listeners) != 0 {
		t.Fatalf("a platform with no reader produced %+v", snap.Listeners)
	}
	if snap.Scope != ScopeSelf {
		t.Errorf("scope = %q, want %q", snap.Scope, ScopeSelf)
	}
	joined := strings.Join(snap.Limits, " ")
	if !strings.Contains(joined, "plan9") || !strings.Contains(joined, "no socket reader") {
		t.Errorf("limits = %q, want the platform named with the reason", snap.Limits)
	}
}

func TestDiscoverSortsRowsSoTwoReadingsListIdentically(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.addFile("/proc/net/udp", procNetHeader+
		procNetRow("00000000", "0016", "07", 1000, "a")+ // 22
		procNetRow("0100007F", "0BB8", "07", 1000, "b")) // 3000

	first := discoverFixture(p)
	second := discoverFixture(p)

	ports := make([]int, 0, len(first.Listeners))
	for _, l := range first.Listeners {
		ports = append(ports, l.Port)
	}
	want := []int{22, 3000, 3000}
	if len(ports) != len(want) {
		t.Fatalf("ports = %v, want %v", ports, want)
	}
	for i := range want {
		if ports[i] != want[i] {
			t.Fatalf("ports = %v, want %v", ports, want)
		}
	}

	firstKeys := ""
	secondKeys := ""
	for _, l := range first.Listeners {
		firstKeys += l.Key() + "\n"
	}
	for _, l := range second.Listeners {
		secondKeys += l.Key() + "\n"
	}
	if firstKeys != secondKeys {
		t.Errorf("two readings of one machine ordered differently:\n%s\n%s", firstKeys, secondKeys)
	}
}

func TestDiscoverKeepsTheStatedTime(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.now = time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)

	snap := discoverFixture(p)
	if !snap.At.Equal(p.now) {
		t.Errorf("At = %v, want the probe's clock %v", snap.At, p.now)
	}
}

func TestDiscoverCarriesTheToolWarningAsALimit(t *testing.T) {
	p := newFakeProbe("linux")
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		p.fileErr[name] = permissionDenied(name)
	}
	p.addTool("ss")
	p.addCommand("ss", []string{"-H", "-tulpn"}, CommandResult{
		Stdout: []byte("tcp   LISTEN 0      4096   127.0.0.1:631    0.0.0.0:*\n"),
		Stderr: []byte("Warning: can't inspect every process\n"),
	})

	snap := discoverFixture(p)
	if len(snap.Listeners) != 1 {
		t.Fatalf("got %d listeners, want the row ss did report", len(snap.Listeners))
	}
	if joined := strings.Join(snap.Limits, " "); !strings.Contains(joined, "can't inspect every process") {
		t.Errorf("limits = %q, want the tool's own warning", snap.Limits)
	}
}
