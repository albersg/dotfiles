package portscan

import (
	"strings"
	"testing"
)

// The third verification round ran the redaction rules against the argument
// forms real tools take rather than against the ones the rules were written
// for, and six of them survived: a curl proxy login, redis-cli's own password
// letter, mysql's joined password word and its all-digit password, a personal
// access token carried as a URL userinfo, and a password that begins with a
// dash. These are the regression tests for that round, written from the tool
// that takes each form.

// TestRedactedArgumentsDoNotLeakAToolCredential is the leak half: every one of
// these argv shapes hands a real program its password, and none of them may
// reach the screen or the diagnostic in the clear.
func TestRedactedArgumentsDoNotLeakAToolCredential(t *testing.T) {
	cases := []struct {
		name   string
		argv   []string
		secret string
	}{
		{"curl's proxy login", []string{"curl", "--proxy-user", "alice:s3cret", "https://api"}, "s3cret"},
		{"redis-cli's password letter", []string{"redis-cli", "-a", "s3cret"}, "s3cret"},
		{"mysql's joined password word", []string{"mysql", "-psecret", "db"}, "secret"},
		{"mysql's joined digit password", []string{"mysql", "-p123456", "db"}, "123456"},
		{"mysql's digit password apart", []string{"mysql", "-p", "123456", "db"}, "123456"},
		{"a token as a URL userinfo", []string{"git", "clone", "https://s3cret@host/repo.git"}, "s3cret"},
		{"a password that begins with a dash", []string{"sshpass", "-p", "-s3cret", "host"}, "s3cret"},
		{"a proxy login without a scheme", []string{"curl", "--proxy", "alice:s3cret@proxy:8080", "https://api"}, "s3cret"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(RedactArgs(tc.argv), " "); strings.Contains(got, tc.secret) {
				t.Errorf("RedactArgs(%q) = %q, and the credential %q survived", tc.argv, got, tc.secret)
			}
		})
	}
}

// TestRedactedArgumentsKeepTheWordAfterAJoinedPassword pins the second half of
// the same defect: reading -psecret as a flag name masked the database name
// after it and showed the password, which is both a leak and a lie about the
// command that was run.
func TestRedactedArgumentsKeepTheWordAfterAJoinedPassword(t *testing.T) {
	got := RedactArgs([]string{"mysql", "-psecret", "db"})
	if strings.Join(got, " ") != "mysql -p"+redacted+" db" {
		t.Errorf("RedactArgs = %q, want the database name kept and the password masked", got)
	}
}

// TestRedactedArgumentsKeepTheNamesThatCarryNoSecret is the over-mask half.
// A name that ends in a short secret word, names the absence of one, or names
// the place one lives is not a credential, and masking its value costs the
// diagnostic the fidelity it is read for.
func TestRedactedArgumentsKeepTheNamesThatCarryNoSecret(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"a bypass flag", []string{"app", "--bypass=true"}},
		{"a compass flag", []string{"app", "--compass", "north"}},
		{"an overpass flag", []string{"app", "--overpass", "x"}},
		{"a pass-through flag", []string{"app", "--pass-through", "x"}},
		{"a token-file path", []string{"app", "--token-file", "/etc/token"}},
		{"a path after a boolean flag that shares the letter", []string{"ls", "-a", "/tmp"}},
		{"a mysql flag-like joined word", []string{"mysql", "-pfoo", "db"}},
		{"a flag that negates its credential", []string{"app", "--no-auth", "--port", "3000"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactArgs(tc.argv); strings.Join(got, " ") != strings.Join(tc.argv, " ") {
				t.Errorf("RedactArgs(%q) = %q, want it unchanged", tc.argv, got)
			}
		})
	}
}

// TestRedactedArgumentsMaskASessionIDCookie pins a credential name with no test
// of its own: the session id a browser and a container registry hand over is a
// bearer credential, and it travels as a name=value pair inside a header.
func TestRedactedArgumentsMaskASessionIDCookie(t *testing.T) {
	argv := []string{"curl", "-H", "Cookie: sessionid=s3cret"}
	if got := strings.Join(RedactArgs(argv), " "); strings.Contains(got, "s3cret") {
		t.Errorf("RedactArgs(%q) = %q, and the session id survived", argv, got)
	}
}

// TestRedactedArgumentsKeepABareAccountInAUserAssignment states the boundary
// the rules stop at: a login flag carries a user:password pair, and the account
// half of it names the service rather than the secret, so --user=alice stays
// readable while --user=alice:s3cret does not.
func TestRedactedArgumentsKeepABareAccountInAUserAssignment(t *testing.T) {
	argv := []string{"app", "--user=alice"}
	if got := RedactArgs(argv); strings.Join(got, " ") != strings.Join(argv, " ") {
		t.Errorf("RedactArgs(%q) = %q, want the bare account kept", argv, got)
	}

	pair := []string{"app", "--user=alice:s3cret"}
	if got := strings.Join(RedactArgs(pair), " "); strings.Contains(got, "s3cret") {
		t.Errorf("RedactArgs(%q) = %q, and the password survived", pair, got)
	}
}

// TestStopRefusalNamesAnUnestablishedScope pins the wording of the branch that
// carries the fail-closed decision. The scope that is not local is refused by
// the neighbouring branch, so an empty scope must be refused as one that could
// not be established rather than as one that is somewhere else.
func TestStopRefusalNamesAnUnestablishedScope(t *testing.T) {
	p := linuxHost(999, 3000, "777")

	reason := StopRefusal(p, StopTarget{Process: Process{PID: 999, Name: "node"}})
	if !strings.Contains(reason, "could not be established") {
		t.Errorf("reason = %q, want the unestablished scope named as what is missing", reason)
	}
}
