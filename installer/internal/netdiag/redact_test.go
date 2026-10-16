package netdiag

import (
	"net/url"
	"strings"
	"testing"
)

// TestRedactCredentialsMasksEveryURLPassword is the unconditional half of
// redaction: a line a check built carelessly must still not carry a proxy's
// password out of the process.
func TestRedactCredentialsMasksEveryURLPassword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "proxy with user and password",
			in:   "the proxy is http://alice:s3cr3t@proxy.corp:3128 for this run",
			want: "the proxy is http://***:***@proxy.corp:3128 for this run",
		},
		{
			name: "proxy with user only",
			in:   "HTTPS_PROXY=http://alice@proxy.corp:3128",
			want: "HTTPS_PROXY=http://***@proxy.corp:3128",
		},
		{
			name: "socks proxy with percent-escaped password",
			in:   "all_proxy=socks5h://u:p%40ss@proxy:1080",
			want: "all_proxy=socks5h://***:***@proxy:1080",
		},
		{
			name: "no credentials",
			in:   "https://example.com/a?token=1",
			want: "https://example.com/a?token=1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactCredentials(c.in)
			if got != c.want {
				t.Errorf("RedactCredentials(%q) = %q, want %q", c.in, got, c.want)
			}
			for _, secret := range []string{"s3cr3t", "p%40ss", "alice"} {
				if strings.Contains(c.in, secret) && strings.Contains(got, secret) {
					t.Errorf("the credential %q survived redaction: %q", secret, got)
				}
			}
		})
	}
}

// TestRedactProxyKeepsTheEndpointAndDropsTheSecret pins what a report may say
// about a proxy: enough to recognise the configuration, never enough to leak it.
func TestRedactProxyKeepsTheEndpointAndDropsTheSecret(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "with credentials", in: "http://alice:s3cr3t@proxy.corp:3128", want: "http://***:***@proxy.corp:3128"},
		{name: "without credentials", in: "http://proxy.corp:3128", want: "http://proxy.corp:3128"},
		{name: "no scheme", in: "proxy.corp:3128", want: "http://proxy.corp:3128"},
		{name: "socks", in: "socks5://alice:pw@127.0.0.1:1080", want: "socks5://***:***@127.0.0.1:1080"},
		{name: "empty", in: "  ", want: ""},
		{name: "garbage", in: "not a url at all", want: "not a url at all"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RedactProxy(c.in); got != c.want {
				t.Errorf("RedactProxy(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSafeURLShowsTheEndpointAndNeverTheQuery pins the URL a report may print:
// the scheme, the host, the port and the path, with the query string -- where a
// token lives -- dropped rather than truncated.
func TestSafeURLShowsTheEndpointAndNeverTheQuery(t *testing.T) {
	u, err := url.Parse("https://alice:pw@api.example.com:8443/v2/items?token=abcdef#frag")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	got := SafeURL(u)
	if got != "https://api.example.com:8443/v2/items" {
		t.Errorf("SafeURL = %q, want the endpoint and the path only", got)
	}
	for _, secret := range []string{"alice", "pw", "token", "abcdef", "frag"} {
		if strings.Contains(got, secret) {
			t.Errorf("SafeURL leaked %q: %q", secret, got)
		}
	}
	if !URLHasQuery(u) {
		t.Error("URLHasQuery said the URL has no query, so a check could not tell that one was dropped")
	}
	if URLHasQuery(&url.URL{Scheme: "https", Host: "example.com"}) {
		t.Error("URLHasQuery said a URL with no query has one")
	}
}

// TestAnonymizeReplacesAddressesNamesAndPaths pins the anonymization the user
// chooses before sending a report to somebody else, and the trap it avoids: a
// timestamp must not be mistaken for an address.
func TestAnonymizeReplacesAddressesNamesAndPaths(t *testing.T) {
	r := newRedactor(TextOptions{Anonymize: true, HostName: "alice-laptop"})

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "public ipv4", in: "api.github.com resolved to 140.82.121.4", want: "api.github.com resolved to <ipv4>"},
		{name: "private ipv4 with prefix", in: "enp3s0 is up: 192.168.1.42/24", want: "enp3s0 is up: <ipv4>/24"},
		{name: "ipv6", in: "the answer was fe80::1%eth0", want: "the answer was <ipv6>%eth0"},
		{name: "bracketed ipv6 with port", in: "dialling [2001:db8::1]:443 failed", want: "dialling [<ipv6>]:443 failed"},
		{name: "ipv4 with port", in: "dialled 192.0.2.10:22", want: "dialled <ipv4>:22"},
		{name: "hostname", in: "the resolver is alice-laptop", want: "the resolver is <host>"},
		{name: "timestamp is not an address", in: "started: 2026-10-16 12:04:05 UTC", want: "started: 2026-10-16 12:04:05 UTC"},
		{name: "gateway in a sentence", in: "the default route goes through 192.168.1.1", want: "the default route goes through <ipv4>"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.line(c.in); got != c.want {
				t.Errorf("line(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestAnonymizeReplacesTheHomeDirectoryAndTheUserName pins the two names that
// make a report personal, and the boundary rule that keeps a name from eating the
// words around it: the user "alber" must not rewrite "albersg/dotfiles".
func TestAnonymizeReplacesTheHomeDirectoryAndTheUserName(t *testing.T) {
	t.Setenv("HOME", "/home/alber")
	t.Setenv("USER", "alber")
	t.Setenv("USERNAME", "")

	r := newRedactor(TextOptions{Anonymize: true, HostName: "alber-pc"})

	got := r.line("the config lives at /home/alber/.config/dotfiles for alber on alber-pc")
	want := "the config lives at ~/.config/dotfiles for <user> on <host>"
	if got != want {
		t.Errorf("line() = %q, want %q", got, want)
	}

	// The same name inside a longer word is not a name: replacing it would make
	// the report unreadable and hide which repository was mentioned.
	got = r.line("cloned from github.com/albersg/dotfiles")
	if strings.Contains(got, "<user>") {
		t.Errorf("line() = %q: the user name replaced part of a longer word", got)
	}
}

// TestRedactionIsOffByDefaultButCredentialsNeverAre pins the two levels: a plain
// report keeps the addresses and the names it measured, and even then no
// password survives.
func TestRedactionIsOffByDefaultButCredentialsNeverAre(t *testing.T) {
	r := newRedactor(TextOptions{})

	got := r.line("enp3s0 is up: 192.168.1.42/24, proxy http://alice:s3cr3t@proxy.corp:3128")
	want := "enp3s0 is up: 192.168.1.42/24, proxy http://***:***@proxy.corp:3128"
	if got != want {
		t.Errorf("line() = %q, want %q", got, want)
	}
}
