package netdiag

import (
	"net"
	"strings"
	"testing"
)

// TestParseTargetAcceptsWhatTheScreenPromises pins the inputs the checker says it
// diagnoses, with the normalized form each one becomes. The normalized form is
// the part that matters: it is what the rest of the run dials, so a target that
// parses into the wrong host is worse than one that is refused.
func TestParseTargetAcceptsWhatTheScreenPromises(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		host    string
		port    int
		portSet bool
		kind    TargetKind
		scheme  string
		unicode string
	}{
		{name: "host name", input: "example.com", host: "example.com", port: 443, kind: KindHost},
		{name: "host with port", input: "example.com:8443", host: "example.com", port: 8443, portSet: true, kind: KindHost},
		{name: "upper case", input: "Example.COM", host: "example.com", port: 443, kind: KindHost},
		{name: "trailing dot", input: "example.com.", host: "example.com.", port: 443, kind: KindHost},
		{name: "underscore label", input: "_ldap._tcp.corp.local", host: "_ldap._tcp.corp.local", port: 443, kind: KindHost},
		{name: "ipv4", input: "192.0.2.10", host: "192.0.2.10", port: 443, kind: KindHost},
		{name: "ipv4 with port", input: "192.0.2.10:8080", host: "192.0.2.10", port: 8080, portSet: true, kind: KindHost},
		{name: "ipv6 with port", input: "[2001:db8::1]:443", host: "2001:db8::1", port: 443, portSet: true, kind: KindHost},
		{name: "bare ipv6", input: "2001:db8::1", host: "2001:db8::1", port: 443, kind: KindHost},
		{name: "link local with zone", input: "[fe80::1%eth0]:443", host: "fe80::1%eth0", port: 443, portSet: true, kind: KindHost},
		{name: "internationalized", input: "bücher.de", host: "xn--bcher-kva.de", unicode: "bücher.de", port: 443, kind: KindHost},
		{name: "decomposed internationalized", input: "bu\u0308cher.de", host: "xn--bcher-kva.de", unicode: "bu\u0308cher.de", port: 443, kind: KindHost},
		{name: "https url", input: "https://example.com/a/b", host: "example.com", port: 443, kind: KindURL, scheme: "https"},
		{name: "http url with port", input: "http://example.com:8080/x", host: "example.com", port: 8080, portSet: true, kind: KindURL, scheme: "http"},
		{name: "url default http port", input: "http://example.com/x", host: "example.com", port: 80, kind: KindURL, scheme: "http"},
		{name: "url with ipv6 host", input: "https://[2001:db8::1]/x", host: "2001:db8::1", port: 443, kind: KindURL, scheme: "https"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseTarget(c.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q) refused a target the screen promises to diagnose: %v", c.input, err)
			}
			if got.Host != c.host {
				t.Errorf("host = %q, want %q", got.Host, c.host)
			}
			if got.Port != c.port {
				t.Errorf("port = %d, want %d", got.Port, c.port)
			}
			if got.PortSet != c.portSet {
				t.Errorf("portSet = %v, want %v (an assumed port must be distinguishable from a typed one)", got.PortSet, c.portSet)
			}
			if got.Kind != c.kind {
				t.Errorf("kind = %v, want %v", got.Kind, c.kind)
			}
			if got.Scheme != c.scheme {
				t.Errorf("scheme = %q, want %q", got.Scheme, c.scheme)
			}
			if got.Unicode != c.unicode {
				t.Errorf("unicode = %q, want %q", got.Unicode, c.unicode)
			}
			if got.Raw != c.input {
				t.Errorf("raw = %q, want the input %q kept for the screen to echo", got.Raw, c.input)
			}
		})
	}
}

// TestParseTargetRefusesWithAReason pins the other half: every refused input
// carries a sentence that names what was wrong, because a generic failure leaves
// the user with nothing to fix.
func TestParseTargetRefusesWithAReason(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "   ", want: "no target was entered"},
		{name: "space inside", input: "exa mple.com", want: "contains no spaces"},
		{name: "control character", input: "example.com\x07", want: "control character"},
		{name: "unsupported scheme", input: "ftp://example.com", want: "only http and https"},
		{name: "url without host", input: "https:///path", want: "names no host"},
		{name: "port is not a number", input: "example.com:https", want: "not a number between 1 and 65535"},
		{name: "port out of range", input: "example.com:70000", want: "not a number between 1 and 65535"},
		{name: "dangling colon", input: "example.com:", want: "names no port"},
		{name: "unclosed bracket", input: "[2001:db8::1", want: "no ]"},
		{name: "bracket with trailing text", input: "[2001:db8::1]x", want: "after its closing bracket"},
		{name: "colons that are not an address", input: "example.com:80:90", want: "neither a host with one port nor an IPv6 address"},
		{name: "empty label", input: "a..b.com", want: "empty label"},
		{name: "leading hyphen", input: "-example.com", want: "starts or ends with a hyphen"},
		{name: "label too long", input: strings.Repeat("a", 64) + ".com", want: "longer than 63 bytes"},
		{name: "right to left label", input: "مثال.إختبار", want: "right-to-left"},
		{name: "joiner", input: "a\u200Db.com", want: "zero-width joiner"},
		{name: "slash in a bare host", input: "example.com/path", want: "not allowed in a host name"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseTarget(c.input)
			if err == nil {
				t.Fatalf("ParseTarget(%q) accepted an input the screen must refuse", c.input)
			}
			var inputErr *InputError
			if !asInputError(err, &inputErr) {
				t.Fatalf("ParseTarget(%q) returned %T, want an *InputError the screen can show as a reason", c.input, err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the reason for %q is %q, want it to contain %q", c.input, err.Error(), c.want)
			}
		})
	}
}

// asInputError is errors.As with the pointer shape the callers use.
func asInputError(err error, target **InputError) bool {
	inputErr, ok := err.(*InputError)
	if ok {
		*target = inputErr
	}
	return ok
}

// TestParseTargetDropsCredentialsAndShowsNoQuery pins two promises the screen
// makes: a URL's credentials are never sent and never shown, and the query string
// -- where a token lives -- is never part of the target's description, although
// the request itself keeps it.
func TestParseTargetDropsCredentialsAndShowsNoQuery(t *testing.T) {
	target, err := ParseTarget("https://alice:s3cr3t@example.com/a?token=abc123#frag")
	if err != nil {
		t.Fatalf("ParseTarget refused a URL with credentials: %v", err)
	}

	if !target.HadCredentials {
		t.Error("the target does not record that the URL carried credentials, so the screen cannot say they were dropped")
	}
	if target.URL == nil {
		t.Fatal("the URL target kept no URL")
	}
	if target.URL.User != nil {
		t.Error("the sanitized URL still carries credentials, which every request would send")
	}
	if got := target.URL.RawQuery; got != "token=abc123" {
		t.Errorf("the request's query is %q, want the query the user asked for", got)
	}
	if target.URL.Fragment != "" {
		t.Error("the fragment is still part of the target; it is never sent to a server")
	}

	described := target.Describe()
	for _, secret := range []string{"alice", "s3cr3t", "token", "abc123", "frag"} {
		if strings.Contains(described, secret) {
			t.Errorf("Describe() = %q, which shows %q: a report may not carry it", described, secret)
		}
	}
	if !strings.Contains(described, "https://example.com/a") {
		t.Errorf("Describe() = %q, want the scheme, host and path", described)
	}
}

// TestTargetEndpointSaysWhetherTLSApplies pins the rule the TLS check depends
// on: https means a handshake, a bare host means one only on a port whose whole
// traffic is TLS, and nothing else is guessed.
func TestTargetEndpointSaysWhetherTLSApplies(t *testing.T) {
	cases := []struct {
		input string
		tls   bool
	}{
		{input: "https://example.com/", tls: true},
		{input: "http://example.com/", tls: false},
		{input: "example.com", tls: true},
		{input: "example.com:8443", tls: true},
		{input: "example.com:8080", tls: false},
		{input: "example.com:22", tls: false},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			target, err := ParseTarget(c.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q): %v", c.input, err)
			}
			if got := target.Endpoint().TLS(); got != c.tls {
				t.Errorf("Endpoint().TLS() = %v, want %v for %q", got, c.tls, c.input)
			}
		})
	}
}

// TestEndpointAddressBracketsIPv6 pins the one place a malformed address would
// reach a socket: an IPv6 host must come back bracketed, exactly as the dialer
// requires, and a link-local address must keep its zone.
func TestEndpointAddressBracketsIPv6(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "example.com", want: "example.com:443"},
		{input: "example.com:80", want: "example.com:80"},
		{input: "2001:db8::1", want: "[2001:db8::1]:443"},
		{input: "[fe80::1%eth0]:8443", want: "[fe80::1%eth0]:8443"},
		{input: "192.0.2.10:22", want: "192.0.2.10:22"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			target, err := ParseTarget(c.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q): %v", c.input, err)
			}
			if got := target.Endpoint().Address(); got != c.want {
				t.Errorf("Address() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestDestinationEndpointNamesNoHostItWasNotGiven pins the rule that keeps this
// package from inventing a destination: a zero Destination yields no endpoint, an
// empty URL leaves the host alone, and a URL fills in the scheme's port.
func TestDestinationEndpointNamesNoHostItWasNotGiven(t *testing.T) {
	if ep, ok := (Session{}).Endpoint(); ok {
		t.Errorf("a session with no target and no destination produced %v, want no endpoint", ep)
	}

	d := Destination{Host: "api.github.com", Source: "the host this installer already contacts"}
	ep := d.Endpoint()
	if ep.Host != "api.github.com" || ep.Port != DefaultPort {
		t.Errorf("endpoint = %s:%d, want api.github.com:%d", ep.Host, ep.Port, DefaultPort)
	}
	if ep.URL != nil {
		t.Errorf("a destination with no URL produced the URL %v", ep.URL)
	}

	d = Destination{URL: "https://api.github.com/", Source: "the update check's own endpoint"}
	ep = d.Endpoint()
	if ep.Host != "api.github.com" || ep.Port != 443 || ep.URL == nil {
		t.Fatalf("endpoint = %+v, want the URL's host, port and URL", ep)
	}
	if !ep.TLS() {
		t.Error("an https destination does not say TLS applies")
	}
}

// TestSessionEndpointPrefersTheUsersTarget pins the precedence: when the user
// typed a target, the run diagnoses that, not the predefined destination.
func TestSessionEndpointPrefersTheUsersTarget(t *testing.T) {
	target, err := ParseTarget("example.com:8443")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	s := Session{Options: Options{
		Target:      target,
		Destination: Destination{Host: "api.github.com"},
	}}
	ep, ok := s.Endpoint()
	if !ok {
		t.Fatal("a session with a target produced no endpoint")
	}
	if ep.Host != "example.com" || ep.Port != 8443 {
		t.Errorf("endpoint = %s:%d, want the target example.com:8443", ep.Host, ep.Port)
	}
	if !s.UserTarget() {
		t.Error("a session with a target does not report a user target")
	}
	if ep.Source != "you entered it" {
		t.Errorf("source = %q, want the report to say the user entered it", ep.Source)
	}
}

// TestParseTargetKeepsALiteralIPRecognizable pins that a literal address is
// recognized as one: the DNS check must not ask a resolver for a name that is
// already an address.
func TestParseTargetKeepsALiteralIPRecognizable(t *testing.T) {
	target, err := ParseTarget("2001:db8::1")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	if target.IP == nil || !target.IP.Equal(net.ParseIP("2001:db8::1")) {
		t.Errorf("IP = %v, want the parsed address", target.IP)
	}

	named, err := ParseTarget("example.com")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	if named.IP != nil {
		t.Errorf("IP = %v for a host name, want nil", named.IP)
	}
}
