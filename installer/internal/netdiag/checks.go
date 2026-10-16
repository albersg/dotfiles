package netdiag

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ============================================================================
// THE PROTOCOL CHECKS
// ============================================================================
//
// These are the checks that actually reach out: a resolution, a connection, a
// request, a handshake, optionally a trace, and the cross-check that asks whether
// what answered was really the destination. Each one runs exactly one kind of
// probe and reaches its own conclusion, because the whole point of the report is
// to tell "the name resolved and the port is closed" apart from "the name did not
// resolve": a check that swallowed the next stage's failure would put the fault
// in the wrong place, which is the one error a diagnosis cannot afford.
//
// Three rules hold them together. Nothing dials without a destination, and a
// check with none reports itself as skipped rather than failing. Every effect is
// bounded twice, by its own stage and by the run's context. And latency is
// always the TCP handshake's: nothing here sends or interprets an ICMP echo, so a
// network that drops pings is diagnosed exactly like one that answers them (S2).

// checkDNS resolves the destination and reports what answered, how long it took
// and -- when it can be known -- which server it was.
//
// It asks the system resolver first, because that is what every other program on
// this machine uses, and then one configured server directly, because the system
// resolver does not say which server answered. Reporting both is what separates
// "nothing resolved" from "this machine's resolver is the thing that is broken":
// a slow or wrong answer from one server while the other is right is a
// configuration problem, not a broken network (S4).
func checkDNS(p Probe, s Session) (Check, Facts) {
	check := newCheck("dns.resolve", StageResolver, "Resolve the destination")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to resolve", "", "")
		return check, Facts{}
	}
	check.Title = titleForDNS(ep, true)

	if ip := net.ParseIP(ep.ServerName()); ip != nil {
		// A literal address needs no resolution, and saying so is not the same as
		// saying a resolution worked.
		check.addEvidence("the destination is already an address, so nothing had to be resolved")
		facts := Facts{Addresses: []string{ep.ServerName()}}
		check.conclude(StatusOK, "", "", "")
		return check, facts
	}
	if p.NewResolver == nil || p.LookupHost == nil {
		check.conclude(StatusSkipped, "this platform cannot resolve a name through this utility", "", "")
		return check, Facts{}
	}

	facts := Facts{}
	bounds := s.Bounds()
	ctx, cancel := s.WithTimeout(bounds.dns())
	defer cancel()

	resolver := p.NewResolver("")
	started := p.Now()
	addresses, err := p.LookupHost(ctx, resolver, ep.ServerName())
	elapsed := p.Now().Sub(started)

	answered := err == nil && len(addresses) > 0
	switch {
	case answered:
		facts.Addresses = addresses
		facts.DNSDuration = elapsed
		check.addEvidence(fmt.Sprintf("the system resolver answered %s in %s",
			strings.Join(addresses, ", "), Duration(elapsed)))
		if len(addresses) > 1 {
			check.addEvidence(fmt.Sprintf("%d addresses were returned; the connection check tries them in "+
				"order up to its own attempt bound", len(addresses)))
		}
	case err == nil:
		// A resolver that answers without an address is not a resolver that failed:
		// it is one whose answer this report cannot state, and saying so is the only
		// honest line (S4, S12).
		check.addEvidence(fmt.Sprintf("the system resolver answered in %s without an address", Duration(elapsed)))
		check.conclude(StatusUnknown,
			"the resolver answered without an address",
			"run the resolution by hand and compare it with this report",
			dnsManualCommand(p.GOOS))
	default:
		check.addEvidence(fmt.Sprintf("the system resolver did not answer inside the %s bound",
			Duration(bounds.dns())))
		status, cause, next, manual := classifyResolution(p, err, ep.ServerName(), bounds.dns())
		check.conclude(status, cause, next, manual)
	}

	// The direct query is what names a server. It is asked only when the
	// configuration names one, and its failure is reported as the fact it is: a
	// server that does not answer while the system resolver did is a difference
	// worth seeing, not a contradiction.
	if server := firstConfiguredServer(p); server != "" {
		directCtx, directCancel := s.WithTimeout(bounds.dnsServer())
		defer directCancel()
		directStarted := p.Now()
		directAddresses, directErr := p.LookupHost(directCtx, p.NewResolver(server), ep.ServerName())
		directElapsed := p.Now().Sub(directStarted)
		switch {
		case directErr == nil && len(directAddresses) > 0:
			facts.Server = server
			check.addEvidence(fmt.Sprintf("%s, the first configured server, answered %s in %s",
				server, strings.Join(directAddresses, ", "), Duration(directElapsed)))
		default:
			check.addEvidence(fmt.Sprintf("%s, the first configured server, did not answer: %s",
				server, RedactCredentials(resolutionReason(directErr, bounds.dnsServer()))))
		}
	}

	if answered {
		check.conclude(StatusOK, "", "", "")
	}
	return check, facts
}

// classifyResolution turns a resolution failure into the report's own words. The
// three kinds are kept apart because they send the reader to three different
// places: a name that does not exist is a typo or a wrong search domain, a
// resolver that timed out is the network or the resolver itself, and a temporary
// failure is the server asking to be tried again.
func classifyResolution(p Probe, err error, host string, bound time.Duration) (Status, string, string, string) {
	var dnsErr *net.DNSError
	switch {
	case err != nil && errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return StatusFail,
			fmt.Sprintf("%q does not exist according to the resolver", host),
			"check the name is spelled the way the service publishes it, and check the search domain this " +
				"machine appends to a short name",
			dnsManualCommand(p.GOOS)
	case (err != nil && errors.As(err, &dnsErr) && dnsErr.IsTimeout) || errors.Is(err, context.DeadlineExceeded):
		return StatusFail,
			fmt.Sprintf("nothing answered inside the %s bound", Duration(bound)),
			"check the resolver itself is reachable: it may be a server that is down, or a network that drops " +
				"traffic to it",
			dnsManualCommand(p.GOOS)
	case err != nil && errors.As(err, &dnsErr) && dnsErr.IsTemporary:
		return StatusFail,
			"the resolver reported a temporary failure, which usually means it is overloaded or being restarted",
			"try the resolution again in a moment",
			dnsManualCommand(p.GOOS)
	default:
		return StatusFail,
			"the resolution failed: " + RedactCredentials(resolutionReason(err, bound)),
			"compare the answer with another resolver, and check whether the search domain this machine " +
				"appends changed the name that was asked for",
			dnsManualCommand(p.GOOS)
	}
}

// resolutionReason is one line for a resolution error, and the bound's own
// wording when the error carries nothing: a report line that says only
// "context deadline exceeded" leaves the reader to guess which limit it hit.
func resolutionReason(err error, bound time.Duration) string {
	if err == nil {
		return fmt.Sprintf("it answered without an address inside the %s bound", Duration(bound))
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return fmt.Sprintf("it did not answer inside the %s bound", Duration(bound))
	}
	return err.Error()
}

// isTimeout reports whether an error is the network's own timeout.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// firstConfiguredServer is the first nameserver the resolver configuration names,
// or an empty string when there is none to ask.
func firstConfiguredServer(p Probe) string {
	if p.ReadFile == nil {
		return ""
	}
	data, err := p.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	if servers := parseResolvConf(string(data)).servers; len(servers) > 0 {
		return servers[0]
	}
	return ""
}

func dnsManualCommand(goos string) string {
	switch goos {
	case "darwin":
		return "dscacheutil -q host -a name <name>"
	case "linux":
		return "getent hosts <name>"
	}
	return "resolve the name with the platform's own tool"
}

// checkTCP connects to the destination and reports how long the handshake took.
//
// That number is the latency this utility reports, and it is deliberately not a
// ping: ICMP is filtered on many networks while TCP is not, so a ping is the one
// measurement whose absence proves nothing. A refusal, a silence and an
// unreachable network are three different conclusions, and each one names its own
// next step, because "the port is closed" and "a firewall is dropping the
// packets" are the two answers a reader most needs told apart (S2, S5).
func checkTCP(p Probe, s Session) Check {
	check := newCheck("tcp.connect", StageTransport, "Connect to the destination")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to connect to", "", "")
		return check
	}
	check.Title = titleForTCP(ep, true)

	if p.DialContext == nil {
		check.conclude(StatusSkipped, "this platform cannot open a connection through this utility", "", "")
		return check
	}

	bounds := s.Bounds()
	candidates := connectionCandidates(s, ep)
	attempts := 0
	var (
		firstError  error
		refused     []string
		unreachable []string
		timedOut    []string
	)

	for _, candidate := range candidates {
		if attempts >= bounds.attempts() {
			check.addEvidence(fmt.Sprintf("stopped after the attempt bound of %d addresses", bounds.attempts()))
			break
		}
		attempts++

		ctx, cancel := s.WithTimeout(bounds.tcp())
		started := p.Now()
		conn, err := p.DialContext(ctx, "tcp", candidate)
		elapsed := p.Now().Sub(started)
		if err == nil {
			local, remote := "", ""
			if conn.LocalAddr() != nil {
				local = conn.LocalAddr().String()
			}
			if conn.RemoteAddr() != nil {
				remote = conn.RemoteAddr().String()
			}
			_ = conn.Close()
			cancel()

			check.Duration = elapsed
			check.addEvidence(fmt.Sprintf("the TCP handshake with %s was established in %s", remote, Duration(elapsed)))
			if local != "" {
				check.addEvidence("the connection left this machine from " + local)
			}
			check.addEvidence("this handshake is the latency this report states; no ping was sent, so a " +
				"network that drops ICMP is measured exactly like one that answers it")
			if len(candidates) > 1 && attempts > 1 {
				check.addEvidence(fmt.Sprintf("the earlier %d addresses did not answer; this one is the "+
					"one that worked", attempts-1))
			}
			if elapsed > time.Second {
				check.addEvidence("a handshake over a second is slow: it points at the path or at the far " +
					"side rather than at the name resolution")
			}
			check.conclude(StatusOK, "", "", "")
			return check
		}
		cancel()

		if firstError == nil {
			firstError = err
		}
		switch {
		case isRefused(err):
			refused = append(refused, candidate)
			check.addEvidence(fmt.Sprintf("%s answered and refused the connection", candidate))
		case isTimeout(err):
			timedOut = append(timedOut, candidate)
			check.addEvidence(fmt.Sprintf("%s did not answer inside %s", candidate, Duration(bounds.tcp())))
		case isUnreachable(err):
			unreachable = append(unreachable, candidate)
			check.addEvidence(fmt.Sprintf("%s is not reachable: %s",
				candidate, RedactCredentials(err.Error())))
		default:
			check.addEvidence(fmt.Sprintf("%s failed: %s", candidate, RedactCredentials(err.Error())))
		}
	}

	switch {
	case len(refused) == len(candidates):
		return concludeRefused(check, ep, refused, p.GOOS)
	case len(timedOut) == len(candidates):
		check.conclude(StatusFail,
			fmt.Sprintf("every attempt was silent: nothing answered inside %s. A firewall that drops the "+
				"packets looks exactly like this, and so does a server that is switched off",
				Duration(bounds.tcp())),
			"check whether the port is reachable from a different network, and whether the service is meant "+
				"to be listening on this address",
			tcpManualCommand(p.GOOS, ep))
	case len(unreachable) == len(candidates):
		check.conclude(StatusFail,
			"there is no route to any of the destination's addresses: the packets never left this machine",
			"check the local network and the default route, which the checks above describe",
			routeManualCommand(p.GOOS))
	default:
		check.conclude(StatusFail,
			"the connection failed: "+RedactCredentials(errorSummary(firstError)),
			"compare the addresses below: one of them may be reachable while another is not",
			tcpManualCommand(p.GOOS, ep))
	}
	return check
}

// connectionCandidates is what the connection tries, in order: what the
// resolution learned when it learned anything, and the destination itself
// otherwise, which lets the dialer's own resolver have a turn when the name was
// never resolved.
func connectionCandidates(s Session, ep Endpoint) []string {
	if len(s.Facts.Addresses) > 0 {
		candidates := make([]string, 0, len(s.Facts.Addresses))
		for _, address := range s.Facts.Addresses {
			candidates = append(candidates, net.JoinHostPort(address, fmt.Sprint(ep.Port)))
		}
		return candidates
	}
	return []string{ep.Address()}
}

// concludeRefused is the port that answered and closed the door. It is separated
// because it is the most-misread failure there is: the name resolved, the host is
// up and the network is fine, and only the service is missing.
func concludeRefused(check Check, ep Endpoint, refused []string, goos string) Check {
	families := addressFamilies(refused)
	detail := fmt.Sprintf("the address answered and refused the connection, so nothing is listening on port %d",
		ep.Port)
	if len(families) == 1 {
		detail += fmt.Sprintf(" over %s", families[0])
	}
	check.conclude(StatusFail,
		detail+" -- the host itself is reachable, and the service is not there",
		"check the port the service publishes, and whether it is bound to this address rather than only to "+
			"another one",
		tcpManualCommand(goos, ep))
	return check
}

// addressFamilies names the address families a set of addresses belongs to, so a
// report can say that one family works while the other does not -- which is what
// an IPv6-disabled network looks like from here (S11).
func addressFamilies(addresses []string) []string {
	v4, v6 := false, false
	for _, address := range addresses {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
			if ip.To4() != nil {
				v4 = true
			} else {
				v6 = true
			}
		}
	}
	var families []string
	if v4 {
		families = append(families, "IPv4")
	}
	if v6 {
		families = append(families, "IPv6")
	}
	return families
}

// errorSummary is one error as a line, without the address prefix a net.OpError
// repeats: the evidence above already names the address it belongs to.
func errorSummary(err error) string {
	if err == nil {
		return "the attempt failed without a reason this utility can name"
	}
	text := err.Error()
	if idx := strings.LastIndex(text, ": "); idx >= 0 && strings.Contains(text[:idx], ":") {
		// "dial tcp 1.2.3.4:443: connect: connection refused" -> the tail is the
		// part a reader needs; the head is repeated in the evidence.
		if tail := text[idx+2:]; tail != "" {
			return tail
		}
	}
	return text
}

// isRefused, isUnreachable and isTimeout classify a connection error. The
// classification is by text because the platform's own error types differ between
// Linux, macOS and Windows while the wording the kernel gives is what a reader
// would see by hand; an error that matches nothing is reported verbatim rather
// than being forced into a category it may not belong to.
func isRefused(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "refused")
}

func isUnreachable(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no route to host") || strings.Contains(text, "network is unreachable") ||
		strings.Contains(text, "host is unreachable")
}

func tcpManualCommand(goos string, ep Endpoint) string {
	target := fmt.Sprintf("<host> %d", ep.Port)
	if ep.Port == DefaultPort {
		target = "<host>"
	}
	switch goos {
	case "darwin":
		return "nc -vz " + target
	case "linux":
		return "nc -vz " + target
	}
	return "connect to " + target + " with the platform's own tool"
}

// checkHTTP makes one bounded request and reports what the service answered.
//
// The body is never read into the report: a length, a content type and a status
// are what a diagnosis needs, and a response body is where a token, a session or
// somebody's private data would end up in a file the user is about to send to
// support (S5, S10). The query string is dropped from every URL the report prints
// for the same reason, and redaction happens again when the text is built.
func checkHTTP(p Probe, s Session) Check {
	check := newCheck("http.get", StageApplication, "HTTP request")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to request", "", "")
		return check
	}

	target, ok := endpointURL(ep)
	if !ok {
		check.conclude(StatusSkipped,
			"the destination is not an http or https address: a bare port is a port, and asking it an HTTP "+
				"question would be a guess -- enter a URL to diagnose HTTP on it", "", "")
		return check
	}
	check.Title = titleForHTTP(ep, true)

	if p.HTTPClient == nil {
		check.conclude(StatusSkipped, "this platform cannot make an HTTP request through this utility", "", "")
		return check
	}

	bounds := s.Bounds()
	ctx, cancel := s.WithTimeout(bounds.http())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		check.conclude(StatusUnknown, "the request could not be built: "+RedactCredentials(err.Error()), "", "")
		return check
	}
	// The client is this machine's own: the proxy configuration it already has,
	// the redirect limit and the timeouts from the bounds, and verification that
	// cannot be asked to stand down.
	req.Header.Set("User-Agent", "dotfiles-netdiag/1")
	req.Header.Set("Accept", "*/*")

	client := p.HTTPClient(bounds)
	if client == nil {
		check.conclude(StatusSkipped,
			"this platform has no HTTP client to build a request with", "", "")
		return check
	}
	started := p.Now()
	resp, err := client.Do(req)
	elapsed := p.Now().Sub(started)

	if err != nil {
		status, cause, next := classifyRequestError(err, bounds, target)
		check.Duration = elapsed
		check.conclude(status, cause, next, "")
		return check
	}
	defer resp.Body.Close()

	// At most the bound's worth of body: enough to recognise a portal page and far
	// too little to pull a payload.
	read, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, bounds.bodyBytes()))
	check.Duration = elapsed
	check.addEvidence(fmt.Sprintf("the service answered %s in %s", resp.Status, Duration(elapsed)))
	check.addEvidence(fmt.Sprintf("%d bytes of the body were read and discarded; a response body is never "+
		"part of this report", read))
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		check.addEvidence("the response declares itself as " + RedactCredentials(contentType))
	}
	if URLHasQuery(target) {
		check.addEvidence("the request carried a query string, which is not shown here: a query is where a " +
			"token lives")
	}

	// Where the request ended, and whether anything along the way took it
	// somewhere else. This is a fact; whether it means something is answering in
	// the destination's place is the interception check's conclusion, not this
	// one's.
	if resp.Request != nil && resp.Request.URL != nil {
		landed := resp.Request.URL
		if SafeURL(landed) != SafeURL(target) {
			check.addEvidence("the request ended at " + SafeURL(landed))
			if landsOnAnotherHost(target, landed) {
				check.addEvidence("that is a different host from the one that was asked for")
			}
		}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.Header.Get("Location") != "" {
		check.addEvidence(fmt.Sprintf("it was still redirecting, and the chain was cut at the bound of %d",
			bounds.redirects()))
	}
	if readErr != nil {
		check.addEvidence("reading the body stopped early: " + RedactCredentials(readErr.Error()))
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		check.conclude(StatusOK, "", "", "")
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		check.conclude(StatusNote,
			fmt.Sprintf("the service keeps redirecting (%s), so the answer depends on where the chain ends",
				resp.Status),
			"follow the chain by hand and diagnose the address it ends at",
			"")
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		check.conclude(StatusNote,
			fmt.Sprintf("the network reached the service and the service answered %s: this is the "+
				"application's own decision, not a connection fault", resp.Status),
			"check the path and the credentials the service expects; the connection itself worked",
			"")
	default:
		check.conclude(StatusNote,
			fmt.Sprintf("the network reached the service and the service answered %s: the connection "+
				"worked and the service is the one failing", resp.Status),
			"check the service's own health; nothing in this report's earlier stages is at fault",
			"")
	}
	return check
}

// endpointURL is the URL the request is made to: the one the user gave when they
// gave one, and otherwise the endpoint's own address with the scheme its port
// implies.
//
// A bare host:port is only turned into a request when the port is one whose whole
// traffic is HTTP or HTTPS. Any other port is a port the user is diagnosing, and
// asking it an HTTP question would be a guess -- one that sends a request to
// somebody's SSH or SMTP service. Saying that instead is the honest answer (S5,
// S17).
func endpointURL(ep Endpoint) (*url.URL, bool) {
	if ep.URL != nil {
		switch ep.URL.Scheme {
		case "http", "https":
			return ep.URL, true
		}
		return nil, false
	}
	scheme, ok := schemeForPort(ep.Port)
	if !ok {
		return nil, false
	}
	return &url.URL{Scheme: scheme, Host: ep.Address(), Path: "/"}, true
}

// schemeForPort is the scheme a port implies, and whether it implies one at all.
// The ports named are the ones whose traffic is the web protocol on the wire, so
// the assumption is a fact about the port rather than a guess about the service.
func schemeForPort(port int) (string, bool) {
	switch port {
	case 80, 8080:
		return "http", true
	case 443, 8443, 9443:
		return "https", true
	}
	return "", false
}

// landsOnAnotherHost answers whether a request ended somewhere other than where it
// was sent. The names are compared rather than the ports: a redirect to another
// port of the same host is the service moving the request along, while another
// name is the shape a portal or a proxy answering in the destination's place
// takes.
func landsOnAnotherHost(target, landed *url.URL) bool {
	return target != nil && landed != nil && !strings.EqualFold(landed.Hostname(), target.Hostname())
}

// classifyRequestError separates the failures a request can have, because they
// belong to different stages: a name that does not resolve never left the
// resolver, a refused connection never left this machine's network, and a
// timeout is the silence in between (S5).
func classifyRequestError(err error, bounds Bounds, target *url.URL) (Status, string, string) {
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return StatusFail, fmt.Sprintf("%q does not exist according to the resolver",
			target.Hostname()), "check the name is spelled the way the service publishes it"
	case (errors.As(err, &dnsErr) && dnsErr.IsTimeout) || errors.Is(err, context.DeadlineExceeded):
		return StatusFail, fmt.Sprintf("nothing answered inside the %s bound", Duration(bounds.http())),
			"check whether the service is meant to answer from this network, and whether a proxy or a " +
				"firewall is in the way"
	case isRefused(err):
		port := target.Port()
		if port == "" {
			port = target.Scheme
		}
		return StatusFail, fmt.Sprintf("the address refused the connection on port %s", port),
			"check the port the service publishes"
	case isUnreachable(err):
		return StatusFail, "there is no route to the address: the packets never left this machine",
			"check the local network and the default route, which the checks above describe"
	default:
		return StatusFail, "the request failed: " + RedactCredentials(err.Error()),
			"compare this with the connection check above: if the connection worked, the failure is in the " +
				"HTTP layer or in what sits between this machine and the service"
	}
}

// checkTLS performs a verified handshake and reports who answered and what they
// presented.
//
// Verification is never skipped and there is no way to ask for it to be: the
// certificate is what tells a diagnosis that the host it reached is the host it
// asked for, and a utility that could be talked out of checking it would report a
// working connection to whoever is in the middle (S6, S17).
func checkTLS(p Probe, s Session) Check {
	check := newCheck("tls.handshake", StageCertificate, "TLS handshake")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to hand a TLS session to", "", "")
		return check
	}
	// The name is set here rather than next to the handshake, because a destination
	// that is not https leaves through the branch below without one: the run's own
	// plan names this step with the same words, and a reader comparing a report that
	// skipped the step with one that ran it must not see two different checks.
	check.Title = titleForTLS(ep, true)
	if !ep.TLS() {
		check.conclude(StatusSkipped,
			"the destination is not https, so there is no certificate to validate", "", "")
		return check
	}
	if p.TLSHandshake == nil {
		check.conclude(StatusSkipped, "this platform cannot open a TLS session through this utility", "", "")
		return check
	}

	bounds := s.Bounds()
	ctx, cancel := s.WithTimeout(bounds.tls())
	defer cancel()

	started := p.Now()
	state, err := p.TLSHandshake(ctx, ep.Address(), ep.ServerName())
	elapsed := p.Now().Sub(started)
	check.Duration = elapsed

	if err != nil {
		// The state is reported even when the handshake failed: a certificate that
		// was presented and refused is the evidence, and without it the reader
		// cannot tell a wrong certificate from a service that does not speak TLS.
		if len(state.PeerCertificates) > 0 {
			check.addEvidence("a certificate was presented before verification failed:")
			reportCertificate(&check, state.PeerCertificates[0], ep.ServerName())
		} else {
			check.addEvidence("no certificate was presented: the far side did not finish a TLS handshake")
		}
		check.addEvidence("verification was on: this utility never offers to skip it")
		status, cause, next, manual := classifyTLSError(err, ep, bounds)
		check.conclude(status, cause, next, manual)
		return check
	}

	leaf := state.PeerCertificates[0]
	check.addEvidence(fmt.Sprintf("the handshake with %s completed in %s using %s and %s",
		ep.Address(), Duration(elapsed), tls.VersionName(state.Version), tls.CipherSuiteName(state.CipherSuite)))
	if state.NegotiatedProtocol != "" {
		check.addEvidence("the application protocol is " + state.NegotiatedProtocol)
	}
	reportCertificate(&check, leaf, ep.ServerName())
	check.addEvidence(fmt.Sprintf("the chain holds %d %s and was verified against this machine's own trust "+
		"store", len(state.PeerCertificates), plural(len(state.PeerCertificates), "certificate", "certificates")))
	check.addEvidence("verification was not skipped: a certificate this machine does not trust would have " +
		"failed here rather than being accepted")

	switch {
	case state.Version < tls.VersionTLS12:
		check.conclude(StatusNote,
			fmt.Sprintf("the far side negotiated %s, which every platform now considers obsolete",
				tls.VersionName(state.Version)),
			"ask the operator of the service to enable TLS 1.2 or newer; nothing on this machine can "+
				"negotiate better with a peer that does not offer it",
			"")
	case daysLeft(leaf) < 21:
		check.conclude(StatusNote,
			fmt.Sprintf("the certificate expires in %d days", daysLeft(leaf)),
			"tell the operator of the service before it expires: a change here would break every client, "+
				"not just this machine",
			"")
	default:
		check.conclude(StatusOK, "", "", "")
	}
	return check
}

// reportCertificate states what was presented: the names it is for, who issued
// it, how long it is valid and how many names it covers. The serial number and
// the fingerprints are deliberately left out -- they identify a certificate
// without helping a reader decide anything.
func reportCertificate(check *Check, cert *x509.Certificate, askedFor string) {
	names := certificateNames(cert)
	check.addEvidence("the certificate is for " + strings.Join(names, ", "))
	check.addEvidence(fmt.Sprintf("it was issued by %s and is valid from %s to %s (%d days left)",
		cert.Issuer.String(), cert.NotBefore.UTC().Format(time.RFC3339),
		cert.NotAfter.UTC().Format(time.RFC3339), daysLeft(cert)))
	switch {
	case len(cert.DNSNames) > 1:
		check.addEvidence(fmt.Sprintf("it covers %d names, so the host that was asked for is one of several",
			len(cert.DNSNames)))
	case cert.Subject.String() == cert.Issuer.String():
		check.addEvidence("it is self-issued: the certificate names itself as its own issuer, which is what a " +
			"private authority or a deliberately intercepted connection looks like")
	}
	if askedFor != "" && !certificateMatches(cert, askedFor) {
		check.addEvidence(fmt.Sprintf("it does not cover %s", askedFor))
	}
}

// certificateNames is what the certificate claims to be for: the common name and
// every name it covers, each said once. A certificate whose common name repeats
// one of its own names is common, and printing it twice reads as two names.
func certificateNames(cert *x509.Certificate) []string {
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	add(cert.Subject.CommonName)
	for _, name := range cert.DNSNames {
		add(name)
	}
	if len(names) == 0 {
		names = append(names, "(no name)")
	}
	return names
}

// certificateMatches answers whether the certificate is valid for a host, using
// the same rule the verification itself uses. It is called on a certificate that
// already failed verification, to say which name it carries instead.
func certificateMatches(cert *x509.Certificate, host string) bool {
	return cert.VerifyHostname(host) == nil
}

func daysLeft(cert *x509.Certificate) int {
	return int(time.Until(cert.NotAfter).Hours() / 24)
}

// classifyTLSError says which kind of refusal this was. A name that does not
// match, an authority this machine does not trust and a certificate outside its
// validity window are three different findings, and a server that answers
// something which is not TLS at all is a fourth (S5).
func classifyTLSError(err error, ep Endpoint, bounds Bounds) (Status, string, string, string) {
	var (
		hostnameErr  x509.HostnameError
		authorityErr x509.UnknownAuthorityError
		invalidErr   x509.CertificateInvalidError
		recordErr    tls.RecordHeaderError
	)
	switch {
	case errors.As(err, &hostnameErr):
		return StatusFail,
			fmt.Sprintf("the certificate does not cover %s: it is for %s, so something other than the "+
				"destination answered", ep.ServerName(), hostnameErr.Certificate.Subject.String()),
			"check whether a proxy or a filter is in the path, and whether the service published the " +
				"certificate it means to use",
			""
	case errors.As(err, &authorityErr):
		return StatusFail,
			"the certificate is not signed by an authority this machine trusts: this is what an " +
				"intercepted connection or a private authority looks like from here",
			"if the service legitimately uses a private authority, its certificate must be installed on this " +
				"machine; do not skip verification to get past this",
			""
	case errors.As(err, &invalidErr):
		switch invalidErr.Reason {
		case x509.Expired:
			return StatusFail,
				fmt.Sprintf("the certificate is outside its validity window: %s",
					RedactCredentials(err.Error())),
				"check this machine's clock as well: a wrong clock makes every valid certificate look expired",
				""
		default:
			return StatusFail,
				"the certificate was rejected: " + RedactCredentials(err.Error()),
				"read the certificate by hand and compare what it says with what the service publishes", ""
		}
	case errors.As(err, &recordErr):
		return StatusFail,
			"the far side answered something that is not a TLS handshake: something else is listening on " +
				"that port",
			"check whether the port is the one the service's TLS endpoint uses", ""
	case isTimeout(err) || errors.Is(err, context.DeadlineExceeded):
		return StatusFail,
			fmt.Sprintf("the handshake did not finish inside the %s bound", Duration(bounds.tls())),
			"the connection check above says whether the port was reachable at all; a handshake that hangs " +
				"on a reachable port is usually a filtered connection",
			""
	default:
		return StatusFail,
			"the handshake failed: " + RedactCredentials(err.Error()),
			"compare this with the connection check above, which says whether the port was reachable", ""
	}
}

// checkTrace is the one check that starts a program, and it is off by default.
//
// It runs the system's own tracepath or traceroute, only when the machine already
// has one, only for a destination the user entered, and never with a shell: the
// arguments are passed as they are, so nothing typed can become a command (S8,
// S16, S17).
func checkTrace(p Probe, s Session) Check {
	check := newCheck("trace.route", StageTransport, "The path to the destination")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to trace a path to", "", "")
		return check
	}
	if !s.Options.Trace {
		check.conclude(StatusSkipped, "the traceroute was not asked for; the switch is on the diagnosis screen",
			"turn on the traceroute row and run the checks again", "tracepath <host>")
		return check
	}
	if !s.UserTarget() {
		check.conclude(StatusSkipped,
			"a traceroute is only run for a destination you entered: an overview has no single host whose "+
				"path would mean anything", "", "")
		return check
	}
	if p.LookPath == nil || p.ExecOutput == nil {
		check.conclude(StatusSkipped, "this platform cannot start a program through this utility", "", "")
		return check
	}

	name, args := "", []string(nil)
	for _, candidate := range []struct {
		name string
		args []string
	}{
		{"tracepath", []string{"-n", ep.ServerName()}},
		{"traceroute", []string{"-n", "-w", "1", "-q", "1", "-m", "15", ep.ServerName()}},
	} {
		if _, err := p.LookPath(candidate.name); err == nil {
			name, args = candidate.name, candidate.args
			break
		}
	}
	if name == "" {
		check.conclude(StatusSkipped,
			"neither tracepath nor traceroute is installed here, and this utility installs nothing",
			"install the tool yourself if you want the path, or read the stages above, which need nothing",
			"")
		return check
	}

	bounds := s.Bounds()
	ctx, cancel := s.WithTimeout(bounds.trace())
	defer cancel()

	started := p.Now()
	output, err := p.ExecOutput(ctx, name, args...)
	elapsed := p.Now().Sub(started)
	check.Duration = elapsed
	check.addEvidence(fmt.Sprintf("%s ran for %s", name, Duration(elapsed)))

	if err != nil && len(output) == 0 {
		check.conclude(StatusUnknown,
			fmt.Sprintf("%s could not run: %s", name, RedactCredentials(err.Error())),
			"run it by hand to see whether it needs privileges this process does not have",
			strings.Join(append([]string{name}, args...), " "))
		return check
	}

	hops, answered, silent, reached := parseTrace(string(output))
	for _, hop := range hops {
		check.addEvidence(hop)
	}
	truncated := errors.Is(ctx.Err(), context.DeadlineExceeded)

	check.addEvidence(fmt.Sprintf("%d hops answered and %d stayed silent", answered, silent))
	check.addEvidence("a hop that answers nothing is usually a router that filters its own replies: it is " +
		"not by itself a fault, and traffic can pass straight through it")

	switch {
	case truncated:
		check.conclude(StatusNote,
			fmt.Sprintf("the trace was cut when the %s bound was reached, so the path below is the part "+
				"that was seen", Duration(bounds.trace())),
			"the silence near the end may be the bound rather than the network",
			strings.Join(append([]string{name}, args...), " "))
	case reached:
		check.conclude(StatusOK, "", "", "")
	case silent == len(hops) && len(hops) > 0:
		check.conclude(StatusNote,
			"nothing along the path answered: the routers are filtering their replies while the "+
				"connection above may still work",
			"read the connection and TLS checks: this one says nothing when the path is silent", "")
	default:
		check.conclude(StatusOK, "", "", "")
	}
	return check
}

// parseTrace turns the tool's own output into one line per hop, and counts the
// hops that answered and the ones that stayed silent. Both tools print a numbered
// line per hop with the probes' times, and a `*` for a probe that got no reply.
// The last line's silence is what says whether the trace arrived: a hop in the
// middle of a path can filter its replies and the traffic still passes through.
func parseTrace(output string) (hopLines []string, answered, silent int, reached bool) {
	lastSilent := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(RedactCredentials(raw))
		if line == "" || !startsWithDigit(line) {
			continue
		}
		if strings.Contains(line, "*") {
			silent++
			lastSilent = true
		} else {
			answered++
			lastSilent = false
		}
		if len(hopLines) < maxTraceHops {
			hopLines = append(hopLines, line)
		}
	}
	return hopLines, answered, silent, answered > 0 && !lastSilent
}

// maxTraceHops bounds what a trace contributes to the report: the first hops and
// the silence near the middle are what identify where a path stops, and a reader
// does not need thirty lines of a path that arrived.
const maxTraceHops = 12

func startsWithDigit(line string) bool {
	return line != "" && line[0] >= '0' && line[0] <= '9'
}

// plural picks the word a count needs, so a report line does not say "1
// certificates".
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// checkInterception asks whether what answered was really the destination.
//
// It is the cross-check the other checks cannot make on their own: the request
// check sees where a request ended, the certificate check sees who signed what,
// and the proxy check sees what the environment declares, but only a check that
// looks at all three can say "the certificate matches, so nothing in the path is
// answering in the destination's place" -- or that something is (S5, S6).
//
// It concludes nothing from an inference alone: when the certificate check could
// not run and the environment declares nothing, the answer is undetermined, which
// is a legitimate answer and the only honest one.
func checkInterception(p Probe, s Session) Check {
	check := newCheck("http.interception", StageApplication, "Is something answering in the destination's place")

	ep, ok := s.Endpoint()
	if !ok {
		check.conclude(StatusSkipped, "there is no destination to compare against", "", "")
		return check
	}

	proxyDeclared := declaredProxyNames(p)
	if len(proxyDeclared) > 0 {
		check.addEvidence("the environment declares " + strings.Join(proxyDeclared, ", ") +
			", so the traffic does not go straight to the destination")
	}
	if len(s.Facts.Addresses) > 0 {
		check.addEvidence("the destination resolved to " + strings.Join(s.Facts.Addresses, ", "))
	}

	if !ep.TLS() {
		switch {
		case len(proxyDeclared) > 0:
			check.conclude(StatusNote,
				"the run goes through something declared in the environment, and the destination is not "+
					"https, so nothing here can prove whether the answer came from it",
				"check the destination over https as well if that is available: a verified certificate is "+
					"what settles this question",
				"")
		default:
			check.conclude(StatusUnknown,
				"nothing in the environment declares a proxy, and the destination is not https, so there is "+
					"no certificate to compare against",
				"diagnose the same host over https: a verified certificate is the evidence this check needs",
				"")
		}
		return check
	}
	if p.TLSHandshake == nil {
		check.conclude(StatusSkipped, "this platform cannot open a TLS session through this utility", "", "")
		return check
	}

	ctx, cancel := s.WithTimeout(s.Bounds().tls())
	defer cancel()
	state, err := p.TLSHandshake(ctx, ep.Address(), ep.ServerName())
	if err != nil {
		if len(state.PeerCertificates) > 0 {
			check.addEvidence("the certificate that answered was not accepted, and it is for " +
				strings.Join(certificateNames(state.PeerCertificates[0]), ", "))
			check.conclude(StatusFail,
				fmt.Sprintf("the certificate presented for %s is not one this machine trusts for it: "+
					"something in the path answered instead of the destination", ep.ServerName()),
				"do not continue past this by disabling verification: find out what is in the path, whether "+
					"it is a proxy this machine's own configuration declares, or something that was not asked for",
				"")
			return check
		}
		check.conclude(StatusUnknown,
			"no certificate could be read: "+RedactCredentials(err.Error()),
			"read the certificate check above: it says whether a certificate was presented at all", "")
		return check
	}

	leaf := state.PeerCertificates[0]
	names := certificateNames(leaf)
	if !certificateMatches(leaf, ep.ServerName()) {
		check.conclude(StatusFail,
			fmt.Sprintf("the certificate is for %s, not for %s", strings.Join(names, ", "), ep.ServerName()),
			"check whether a proxy or a filter is answering for the destination", "")
		return check
	}

	check.addEvidence("the certificate is for " + strings.Join(names, ", ") + ", which covers " +
		ep.ServerName())
	if len(proxyDeclared) > 0 {
		check.conclude(StatusNote,
			"the run goes through something declared in the environment, and the certificate still matches "+
				"the destination, so nothing in the path is answering in its place",
			"read the stages above against the proxy that is configured: a proxy can change what a service "+
				"answers without pretending to be it",
			"")
		return check
	}
	check.conclude(StatusOK, "", "", "")
	return check
}

// declaredProxyNames is the proxy variables the environment sets, without their
// values: the interception check asks whether a proxy could be in the path, not
// what it is called.
func declaredProxyNames(p Probe) []string {
	if p.Environ == nil {
		return nil
	}
	var names []string
	for _, entry := range p.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		switch strings.ToLower(name) {
		case "http_proxy", "https_proxy", "all_proxy":
			names = append(names, strings.ToLower(name))
		}
	}
	return names
}
