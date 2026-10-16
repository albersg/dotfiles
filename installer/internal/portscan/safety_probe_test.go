package portscan

import (
	"context"
	"strings"
	"testing"
)

// The regression tests the independent verification of the safety path asked
// for. They pin the two failures it found -- a credential that survives the
// redaction and reaches the screen, and a port reported free on the strength of
// a reading that never ran -- and the three limitations it raised, so each one
// stays fixed rather than agreed with once.

// TestRedactedArgumentsDoNotLeakCredentials is the first failure. Every input
// here is a string a real process can carry, and each one is a credential the
// screen would otherwise show and the diagnostic would otherwise copy.
func TestRedactedArgumentsDoNotLeakCredentials(t *testing.T) {
	const secret = "s3cret"

	cases := []struct {
		name string
		argv []string
	}{
		{"a URL's userinfo", []string{"psql", "postgres://alice:" + secret + "@localhost/db"}},
		{"a curl login", []string{"curl", "-u", "alice:" + secret, "https://api.example.com"}},
		{"a long-form login", []string{"app", "--user=alice:" + secret}},
		{"an SMB login", []string{"smbclient", "-U", "alice%" + secret, "//host/share"}},
		{"a JSON body on the command line", []string{"app", "--json", `{"password":"` + secret + `"}`}},
		{"a joined short password", []string{"mysql", "-u", "root", "-p" + secret, "db"}},
		{"a secret flag's separate value", []string{"app", "--token", secret}},
		{"a secret assignment", []string{"app", "--api-key=" + secret}},
		{"a short secret assignment", []string{"app", "--pw=" + secret}},
		{"a bearer header", []string{"curl", "-H", "Authorization: Bearer " + secret}},
		{"a two-word joined name", []string{"app", "--access-key=" + secret}},
		{"a database password", []string{"mysqladmin", "--password=" + secret, "status"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(RedactArgs(tc.argv), " ")
			if strings.Contains(got, secret) {
				t.Errorf("RedactArgs(%q) = %q, and the credential survived", tc.argv, got)
			}
		})
	}
}

// TestRedactedArgumentsKeepWhatIsNotASecret is the other half of the same rule:
// the redaction stops at the values that are credentials, because a diagnostic
// that mangles everything explains nothing. A value is masked by its own shape
// or by its flag's name, never by a guess.
func TestRedactedArgumentsKeepWhatIsNotASecret(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"a port", []string{"app", "--port=3000"}},
		{"a host and a port", []string{"app", "--listen", "127.0.0.1:3000"}},
		{"a bare account name", []string{"curl", "-u", "alice", "https://api.example.com"}},
		{"a path", []string{"node", "server.js"}},
		{"a clock time", []string{"app", "--since", "12:30"}},
		{"a user and a host", []string{"app", "--connect", "alice@db.example.com"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactArgs(tc.argv); strings.Join(got, " ") != strings.Join(tc.argv, " ") {
				t.Errorf("RedactArgs(%q) = %q, want it unchanged", tc.argv, got)
			}
		})
	}
}

// TestStopDoesNotClaimFreeWhenThePostReadingSeesNothing is the second failure.
// The process really did exit, but the readers stopped answering at the moment
// of the verification: the sentence may report the exit and must not describe
// the ports as free.
func TestStopDoesNotClaimFreeWhenThePostReadingSeesNothing(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	listener, snap := stopFixture(t, p)
	p.onSignal = func(p *fakeProbe, pid int, sig Signal) {
		p.alive[999] = false
		for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
			p.fileErr[name] = permissionDenied(name)
		}
	}

	result := Stop(context.Background(), p, StopTarget{
		Process: *listener.Owner,
		Sockets: snap.ListenersFor(999),
	}, stopOptions(p, SignalTerminate))

	if result.Outcome != OutcomeStopped {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStopped, result.Message)
	}
	if strings.Contains(result.Message, "are free.") {
		t.Errorf("message = %q, and it calls the ports free on the strength of a reading that saw nothing", result.Message)
	}
	if !strings.Contains(result.Message, "could not be checked") {
		t.Errorf("message = %q, want the part it could not check named", result.Message)
	}
	if result.Verified {
		t.Errorf("Verified = true, but the ports could not be read again")
	}
}

// TestBlindToNamesTheTransportsTheReadingCouldNotAnswerFor keeps the sentence
// honest one transport at a time: a reading that answered for TCP and failed on
// UDP is blind to UDP alone, and a reader that names no transport read the whole
// table.
func TestBlindToNamesTheTransportsTheReadingCouldNotAnswerFor(t *testing.T) {
	shown := []Listener{
		{Protocol: ProtocolTCP, Port: 3000},
		{Protocol: ProtocolUDP, Port: 5353},
		{Protocol: ProtocolTCP, Port: 3001},
	}

	cases := []struct {
		name    string
		fresh   Snapshot
		want    []string
		wantLen int
	}{
		{
			name: "the UDP reader failed",
			fresh: Snapshot{Sources: []Source{
				{Name: "/proc/net/tcp", Rows: 2},
				{Name: "/proc/net/udp", Err: "not readable here"},
			}},
			want: []string{"udp"},
		},
		{
			name:    "a reader that names no transport answered",
			fresh:   Snapshot{Sources: []Source{{Name: "ss", Rows: 3}}},
			wantLen: 0,
		},
		{
			name:    "every reader failed",
			fresh:   Snapshot{Sources: []Source{{Name: "lsof TCP", Err: "lsof reported: no such file"}, {Name: "lsof UDP", Err: "timed out"}}},
			wantLen: 2,
		},
		{
			name: "the macOS readers name their own transport",
			fresh: Snapshot{Sources: []Source{
				{Name: "lsof TCP", Rows: 1},
				{Name: "lsof UDP", Err: "timed out"},
			}},
			want: []string{"udp"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := blindTo(tc.fresh, shown)
			if len(got) != tc.wantLen && tc.want == nil {
				t.Fatalf("blindTo = %v, want %d transports", got, tc.wantLen)
			}
			if tc.want != nil && (len(got) != len(tc.want) || got[0] != tc.want[0]) {
				t.Errorf("blindTo = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDiscoverFallbackNeverEmitsAnEmptyLimit pins the third finding: the ss
// fallback answered with a row and an empty stderr, and the empty sentence
// reached the snapshot's limitations, where a screen can only show a blank line.
func TestDiscoverFallbackNeverEmitsAnEmptyLimit(t *testing.T) {
	p := newFakeProbe("linux")
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		p.fileErr[name] = permissionDenied(name)
	}
	p.tools["ss"] = "/usr/bin/ss"
	p.commands[commandKey("/usr/bin/ss", []string{"-H", "-tulpn"})] = CommandResult{
		Stdout: []byte("tcp   LISTEN 0      4096   127.0.0.1:3000    0.0.0.0:*    users:((\"node\",pid=999,fd=3))\n"),
	}

	listeners, _, limits := discoverLinuxFallbacks(context.Background(), p)
	if len(listeners) == 0 {
		t.Fatalf("the fixture produced no listeners, so the path under test is not the one that ran")
	}
	for i, limit := range limits {
		if strings.TrimSpace(limit) == "" {
			t.Errorf("limits = %q, and limits[%d] is empty: a screen cannot show a limitation nobody wrote", limits, i)
		}
	}
}

// TestStopRefusalFailsClosedWhenTheScopeIsUnknown pins the advisory: a target
// built by hand, with no scope at all, must not be signalled on the guess that
// it is local. Discover fills every owner's scope, so this is the guard for
// everything that does not come from a reading.
func TestStopRefusalFailsClosedWhenTheScopeIsUnknown(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	_, snap := stopFixture(t, p)

	target := StopTarget{Process: Process{PID: 999, Name: "node"}, Sockets: snap.ListenersFor(999)}

	reason := StopRefusal(p, target)
	if reason == "" {
		t.Fatalf("a target whose scope could not be established was allowed")
	}
	if !strings.Contains(reason, "scope") {
		t.Errorf("reason = %q, want the scope named as what is missing", reason)
	}

	result := Stop(context.Background(), p, target, stopOptions(p, SignalTerminate))
	if result.Outcome != OutcomeRefused {
		t.Errorf("outcome = %q, want %q: %s", result.Outcome, OutcomeRefused, result.Message)
	}
	if len(p.signals) != 0 {
		t.Errorf("a process whose scope is unknown was signalled: %+v", p.signals)
	}
}

// TestRedactedArgumentsMaskAPasswordFlag covers the second round of the
// verification: the word pass is a credential name, and sshpass takes its
// password as the value apart from -p.
func TestRedactedArgumentsMaskAPasswordFlag(t *testing.T) {
	const secret = "s3cret"

	cases := []struct {
		name string
		argv []string
	}{
		{"openssl's password flag", []string{"openssl", "-pass", "pass:" + secret}},
		{"a long pass flag with its value joined", []string{"app", "--pass=" + secret}},
		{"a long pass flag with its value apart", []string{"app", "--pass", secret}},
		{"an environment assignment", []string{"app", "DB_PASS=" + secret}},
		{"sshpass's password apart from -p", []string{"sshpass", "-p", secret, "ssh", "alice@host"}},
		{"a password that begins with a dash", []string{"app", "--password", "-" + secret}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(RedactArgs(tc.argv), " "); strings.Contains(got, secret) {
				t.Errorf("RedactArgs(%q) = %q, and the credential survived", tc.argv, got)
			}
		})
	}
}

// TestRedactedArgumentsMaskAnEscapedQuoteValue pins the third leak: a JSON body
// escapes its inner quotes, and a scan that stops at the escaped one leaves the
// rest of the secret in the clear.
func TestRedactedArgumentsMaskAnEscapedQuoteValue(t *testing.T) {
	const secret = "s3cret"
	argv := []string{"app", `{"password":"a\"` + secret + `"}`}

	if got := strings.Join(RedactArgs(argv), " "); strings.Contains(got, secret) {
		t.Errorf("RedactArgs(%q) = %q, and the credential survived", argv, got)
	}
}

// TestRedactedArgumentsKeepAPortOrAFlagAfterTheShortLetter is the no-over-mask
// guard for the same rules: -p is a port where the tools take a port, and
// -print and -path are flag names that merely begin with the secret letter.
func TestRedactedArgumentsKeepAPortOrAFlagAfterTheShortLetter(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"ssh's port", []string{"ssh", "-p", "2222", "host"}},
		{"a published port", []string{"docker", "run", "-p8080:80", "img"}},
		{"a flag whose name begins with p", []string{"find", ".", "-print"}},
		{"a path flag", []string{"find", ".", "-path", "*.go"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactArgs(tc.argv); strings.Join(got, " ") != strings.Join(tc.argv, " ") {
				t.Errorf("RedactArgs(%q) = %q, want it unchanged", tc.argv, got)
			}
		})
	}
}

// TestBlindToTreatsEachSocketFamilyAsItsOwnReader pins the advisory the second
// verification raised: the /proc/net files are one table each, so a reading that
// lost tcp6 cannot answer for an IPv6 row on the strength of its IPv4 sibling.
func TestBlindToTreatsEachSocketFamilyAsItsOwnReader(t *testing.T) {
	shown := []Listener{{Protocol: ProtocolTCP6, Port: 3000}}

	lostIPv6 := Snapshot{Sources: []Source{
		{Name: "/proc/net/tcp6", Err: "not readable here"},
		{Name: "/proc/net/tcp", Rows: 0},
	}}
	if got := blindTo(lostIPv6, shown); len(got) == 0 {
		t.Errorf("blindTo = %v, want tcp6 named because its own reader failed", got)
	}

	answeredIPv6 := Snapshot{Sources: []Source{
		{Name: "/proc/net/tcp", Err: "not readable here"},
		{Name: "/proc/net/tcp6", Rows: 1},
	}}
	if got := blindTo(answeredIPv6, shown); len(got) != 0 {
		t.Errorf("blindTo = %v, want nothing: the tcp6 reader answered", got)
	}

	wholeTransport := Snapshot{Sources: []Source{{Name: "lsof TCP", Rows: 2}}}
	if got := blindTo(wholeTransport, shown); len(got) != 0 {
		t.Errorf("blindTo = %v, want nothing: lsof answers for both families", got)
	}
}

// TestDiscoverReportsADeniedFileDescriptorLinkAsAPermissionLimit pins the last
// advisory: a link read the kernel refuses is the same limit as a directory it
// refuses, and the row says so instead of blaming an exit.
func TestDiscoverReportsADeniedFileDescriptorLinkAsAPermissionLimit(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.linkErr["/proc/999/fd/3"] = permissionDenied("/proc/999/fd/3")

	snap := discoverFixture(p)
	if len(snap.Listeners) != 1 {
		t.Fatalf("the fixture produced %d listeners, want 1", len(snap.Listeners))
	}
	if owner := snap.Listeners[0].Owner; owner != nil {
		t.Fatalf("the owner was resolved through a denied link: %+v", owner)
	}
	if err := snap.Listeners[0].OwnerErr; !strings.Contains(err, "privilege") {
		t.Errorf("OwnerErr = %q, want the permission limit named", err)
	}
}
