package netdiag

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// DefaultPort is the port a bare host is probed on when the user names no port.
// It is 443 because the destination people diagnose by hand is normally a web
// service, and the screen says which port it assumed rather than leaving the
// reader to guess.
const DefaultPort = 443

// TargetKind says which of the two shapes a target has.
type TargetKind int

const (
	// KindHost: a host name or a literal address, with an optional port.
	KindHost TargetKind = iota
	// KindURL: a complete http or https URL.
	KindURL
)

// Target is a user-entered destination, validated and normalized into what the
// checks can use. A target that does not parse never reaches a check: it becomes
// an InputError the screen shows as the reason the run did not start, which is
// cheaper and clearer than a dial that fails for a reason nobody can act on.
//
// Host is the ASCII form the resolver and the socket take: an internationalized
// name is converted to its punycode form, and Unicode keeps the name as it was
// typed so the report can show which domain was actually resolved.
type Target struct {
	// Raw is the input exactly as the user typed it, for the screen to echo.
	Raw string
	// Kind is whether the target was given as a URL or as a bare host.
	Kind TargetKind
	// Scheme is "http" or "https" for a URL, and empty for a bare host.
	Scheme string
	// Host is the ASCII host the checks use: no brackets, no trailing dot
	// removed, no case.
	Host string
	// Unicode is the host as it was typed, when the conversion to ASCII changed
	// more than the case. It is empty otherwise.
	Unicode string
	// Port is the effective port: the one in the input, or the scheme's, or
	// DefaultPort.
	Port int
	// PortSet says whether the port came from the input.
	PortSet bool
	// URL is the sanitized URL for a URL target: credentials, fragment and every
	// raw escape that could re-introduce one are gone. It is nil for a bare host.
	URL *url.URL
	// HadCredentials records that the input carried user:password@, so the screen
	// can say the credentials were dropped instead of appearing to have used
	// them.
	HadCredentials bool
	// IP is the literal address when the host is one, and nil when it is a name.
	IP net.IP
}

// IsZero reports whether no target was entered, which is how the overview asks
// for this machine's own connectivity rather than a destination's.
func (t Target) IsZero() bool { return t.Raw == "" }

// Describe is the target in the report's own words: the safe URL for a URL, and
// host:port for a host. A query string is never part of it.
func (t Target) Describe() string {
	if t.Kind == KindURL && t.URL != nil {
		return SafeURL(t.URL)
	}
	return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

// Endpoint turns the target into what the checks talk to.
func (t Target) Endpoint() Endpoint {
	return Endpoint{
		Host:    t.Host,
		Port:    t.Port,
		URL:     t.URL,
		Unicode: t.Unicode,
		Source:  "you entered it",
	}
}

// InputError is a target the utility refuses to diagnose. Reason is the sentence
// the screen shows: it always names what was wrong with the input, never a
// generic failure, because the user can fix an input.
type InputError struct{ Reason string }

func (e *InputError) Error() string { return e.Reason }

// inputErrorf builds one.
func inputErrorf(format string, args ...any) error {
	return &InputError{Reason: fmt.Sprintf(format, args...)}
}

// ParseTarget validates and normalizes one user-entered destination. It accepts
//
//	example.com            a host name, probed on DefaultPort
//	example.com:8443       a host name with a port
//	192.0.2.10             a literal IPv4 address
//	[2001:db8::1]:443      a literal IPv6 address with a port
//	2001:db8::1            a bare IPv6 address
//	bücher.de             an internationalized name, converted to xn--bcher-kva.de
//	https://example.com/x  a complete URL
//
// and refuses everything else with a sentence that says why. It performs no I/O:
// a refused target costs nothing but the message.
func ParseTarget(raw string) (Target, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return Target{}, inputErrorf("no target was entered: type a host name, an address, a URL or host:port")
	}
	if strings.ContainsAny(input, " \t\r\n") {
		return Target{}, inputErrorf("a target contains no spaces, and %q has one: use host:port or a URL", input)
	}
	for _, r := range input {
		if r < 0x20 || r == 0x7f {
			return Target{}, inputErrorf("the target contains a control character, which cannot be diagnosed")
		}
	}

	if strings.Contains(input, "://") {
		return parseURLTarget(input)
	}
	return parseHostTarget(input)
}

// parseHostTarget handles everything that is not a URL: a host name, an
// internationalized name, a literal address, and any of those with a port.
func parseHostTarget(input string) (Target, error) {
	host, portText, portSet := input, "", false

	if strings.HasPrefix(input, "[") {
		end := strings.IndexByte(input, ']')
		if end < 0 {
			return Target{}, inputErrorf("the address %q starts with [ and has no ]: write it as [2001:db8::1]:443", input)
		}
		host = input[1:end]
		rest := input[end+1:]
		switch {
		case rest == "":
		case strings.HasPrefix(rest, ":"):
			portText, portSet = rest[1:], true
		default:
			return Target{}, inputErrorf("the address %q has something after its closing bracket: write the port as [2001:db8::1]:443", input)
		}
	} else {
		switch colons := strings.Count(input, ":"); {
		case colons == 0:
			host = input
		case colons == 1:
			host, portText, _ = strings.Cut(input, ":")
			portSet = true
		default:
			// More than one colon and no brackets: an IPv6 address, or a typo
			// that would otherwise be dialled as nonsense.
			if parseLiteralIP(input) == nil {
				return Target{}, inputErrorf("%q is neither a host with one port nor an IPv6 address: an IPv6 address with a port is written [2001:db8::1]:443", input)
			}
			host = input
		}
	}

	port, err := parsePort(portText, portSet)
	if err != nil {
		return Target{}, err
	}
	if portSet && portText == "" {
		return Target{}, inputErrorf("the target %q ends with : and names no port", input)
	}

	ascii, unicode, ip, err := parseTargetHost(host)
	if err != nil {
		return Target{}, err
	}

	return Target{
		Raw:     input,
		Kind:    KindHost,
		Host:    ascii,
		Unicode: unicode,
		Port:    port,
		PortSet: portSet,
		IP:      ip,
	}, nil
}

// parseURLTarget handles a complete URL.
func parseURLTarget(input string) (Target, error) {
	u, err := url.Parse(input)
	if err != nil {
		return Target{}, inputErrorf("that URL could not be read: %v", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return Target{}, inputErrorf("only http and https URLs are diagnosed, and this one is %q", u.Scheme)
	}
	if u.Host == "" {
		return Target{}, inputErrorf("the URL %q names no host", input)
	}

	// The credentials are dropped here, once, for the whole run: nothing dialled
	// from a target ever carries them, and the report says they were dropped
	// rather than showing an "authenticated" result nobody can reproduce.
	hadCredentials := u.User != nil
	sanitized := *u
	sanitized.Scheme = scheme
	sanitized.User = nil
	sanitized.Fragment = ""
	sanitized.RawFragment = ""

	port, set := 0, false
	if text := u.Port(); text != "" {
		parsed, err := parsePort(text, true)
		if err != nil {
			return Target{}, err
		}
		port, set = parsed, true
	} else {
		port = defaultPortFor(scheme)
	}

	ascii, unicode, ip, err := parseTargetHost(u.Hostname())
	if err != nil {
		return Target{}, err
	}

	return Target{
		Raw:            input,
		Kind:           KindURL,
		Scheme:         scheme,
		Host:           ascii,
		Unicode:        unicode,
		Port:           port,
		PortSet:        set,
		URL:            &sanitized,
		HadCredentials: hadCredentials,
		IP:             ip,
	}, nil
}

// parsePort validates a port. An empty port with portSet false means "assume
// DefaultPort"; an empty port with portSet true is the caller's error to report,
// because only the caller knows whether the input ended in a colon.
func parsePort(text string, portSet bool) (int, error) {
	if text == "" {
		if portSet {
			return 0, nil
		}
		return DefaultPort, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > 65535 {
		return 0, inputErrorf("the port %q is not a number between 1 and 65535", text)
	}
	return n, nil
}

// defaultPortFor is the port a scheme uses when none is named.
func defaultPortFor(scheme string) int {
	if strings.EqualFold(scheme, "http") {
		return 80
	}
	return DefaultPort
}

// parseTargetHost normalizes one host: a literal address is kept as the platform
// spells it, and a name is converted to ASCII.
func parseTargetHost(host string) (ascii string, unicode string, ip net.IP, err error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", "", nil, inputErrorf("the target names no host")
	}
	if literal := parseLiteralIP(host); literal != nil {
		return normalizeLiteralIP(host, literal), "", literal, nil
	}
	ascii, unicode, err = hostToASCII(strings.ToLower(host))
	if err != nil {
		return "", "", nil, inputErrorf("%s is not a host this utility can diagnose: %v", host, err)
	}
	return ascii, unicode, nil, nil
}

// parseLiteralIP recognizes an IPv4 or IPv6 address, with a zone when one is
// present ("fe80::1%eth0"), and returns nil for anything else. The zone is not
// part of the returned address -- net.IP cannot carry one -- so the caller keeps
// the host string for dialling and uses this only to answer "is it an address?".
func parseLiteralIP(host string) net.IP {
	address, _ := splitZone(host)
	return net.ParseIP(address)
}

// splitZone separates the zone of a link-local address from its address.
func splitZone(host string) (address, zone string) {
	if i := strings.LastIndexByte(host, '%'); i > 0 {
		return host[:i], host[i+1:]
	}
	return host, ""
}

// normalizeLiteralIP is the address as the checks spell it: the platform's own
// form of the address, with the zone put back when the user named one, because a
// link-local address without its zone cannot be dialled.
func normalizeLiteralIP(host string, ip net.IP) string {
	_, zone := splitZone(host)
	if zone == "" {
		return ip.String()
	}
	return ip.String() + "%" + zone
}

// Endpoint is what a check talks to: the host, the port, and the URL when the
// destination was given as one. A user target and a predefined destination both
// resolve into this shape, so the checks are written once.
type Endpoint struct {
	// Host is the ASCII host or literal address, with a zone when it has one.
	Host string
	// Port is the port the transport checks dial.
	Port int
	// URL is the sanitized URL when the destination is a URL, and nil when it is
	// a bare host.
	URL *url.URL
	// Unicode is the host as the user typed it, when it was converted.
	Unicode string
	// Source says where the destination came from, in the report's words.
	Source string
}

// Address is host:port in the form the socket takes, with brackets around an
// IPv6 address.
func (e Endpoint) Address() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

// Describe is the endpoint as the report names it: the safe URL for a URL, and
// host:port otherwise.
func (e Endpoint) Describe() string {
	if e.URL != nil {
		return SafeURL(e.URL)
	}
	return e.Address()
}

// TLS reports whether a TLS handshake belongs to this endpoint: the URL's scheme
// says so when there is one, and otherwise the port does. The three ports named
// are the ones whose whole traffic is TLS on the wire, so a handshake there is
// the protocol rather than a guess.
func (e Endpoint) TLS() bool {
	if e.URL != nil {
		return e.URL.Scheme == "https"
	}
	switch e.Port {
	case 443, 8443, 9443:
		return true
	}
	return false
}

// ServerName is the name TLS verifies the certificate against. It is the ASCII
// host without its zone: a certificate names a host, never a link-local zone.
func (e Endpoint) ServerName() string {
	host, _ := splitZone(e.Host)
	return host
}

// Destination is a predefined place the overview reaches for, named by the
// caller rather than by this package, so nothing here contacts a host the screen
// did not declare. Source is why that destination: "the host this installer
// already contacts for updates".
type Destination struct {
	// Host is the host name or literal address to resolve and connect to. It may
	// be empty when URL is set.
	Host string
	// Port is the port for the transport checks. Zero takes the URL's scheme
	// default, or DefaultPort.
	Port int
	// URL is the absolute URL for the HTTP check, and may be empty when the
	// caller only wants a name resolved and a port dialled.
	URL string
	// Unicode is the host as it was written, when it is not ASCII.
	Unicode string
	// Source is why this destination is the one used, in the screen's words.
	Source string
}

// IsZero reports whether no predefined destination was named, in which case the
// checks that need one report themselves as skipped rather than inventing a host.
func (d Destination) IsZero() bool { return d.Host == "" && d.URL == "" }

// Endpoint turns the destination into what the checks talk to.
func (d Destination) Endpoint() Endpoint {
	ep := Endpoint{Host: d.Host, Port: d.Port, Unicode: d.Unicode, Source: d.Source}
	if d.URL != "" {
		if u, err := url.Parse(d.URL); err == nil && u.Host != "" {
			ep.URL = u
			ep.Host = u.Hostname()
			if ep.Port == 0 {
				ep.Port = defaultPortFor(u.Scheme)
			}
		}
	}
	if ep.Port == 0 {
		ep.Port = DefaultPort
	}
	return ep
}
