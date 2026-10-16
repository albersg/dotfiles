package netdiag

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// THE DOUBLES
// ============================================================================

// checksTestProbe is a probe whose every effect is a double: a simulated
// resolver, a simulated socket, a simulated handshake. Nothing here can reach a
// network, and a test that forgets to replace one gets a failure rather than this
// developer's own connection.
func checksTestProbe(t *testing.T, mutate func(*Probe)) Probe {
	t.Helper()

	p := envTestProbe(t, nil)
	p.Now = advancingClock(time.Millisecond)
	p.NewResolver = func(server string) *net.Resolver {
		if server == "" {
			return net.DefaultResolver
		}
		return &net.Resolver{}
	}
	p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, errors.New("connect: connection refused")
	}
	p.HTTPClient = func(b Bounds) *http.Client { return defaultHTTPClient(b) }
	p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
		return tls.ConnectionState{}, errors.New("this test did not replace the handshake")
	}
	p.ExecOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("this test did not replace the program")
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

// advancingClock is a clock that moves by one step per reading, so a check that
// measures a duration measures the step instead of a real elapsed time.
func advancingClock(step time.Duration) func() time.Time {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		now = now.Add(step)
		return now
	}
}

// checksTestSession builds a session for a destination the user typed. The target
// goes through the real validator, so a test cannot set up a target the product
// would have refused.
func checksTestSession(t *testing.T, raw string, mutate func(*Options)) Session {
	t.Helper()

	options := Options{Bounds: DefaultBounds()}
	if raw != "" {
		target, err := ParseTarget(raw)
		if err != nil {
			t.Fatalf("the test target %q is one the validator refuses: %v", raw, err)
		}
		options.Target = target
	}
	if mutate != nil {
		mutate(&options)
	}
	return Session{Options: options, Ctx: context.Background()}
}

// checksTestOverviewSession is a run with no target at all: the overview, which
// reaches for nothing unless the caller names a destination.
func checksTestOverviewSession(destination Destination) Session {
	return Session{Options: Options{Bounds: DefaultBounds(), Destination: destination}, Ctx: context.Background()}
}

// stubConn is a connection that answers with whatever the test says it is. It
// stands in for a socket so a TCP check can be tested without one.
type stubConn struct{ local, remote string }

func (c stubConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c stubConn) Write(b []byte) (int, error)      { return len(b), nil }
func (c stubConn) Close() error                     { return nil }
func (c stubConn) LocalAddr() net.Addr              { return stubAddr(c.local) }
func (c stubConn) RemoteAddr() net.Addr             { return stubAddr(c.remote) }
func (c stubConn) SetDeadline(time.Time) error      { return nil }
func (c stubConn) SetReadDeadline(time.Time) error  { return nil }
func (c stubConn) SetWriteDeadline(time.Time) error { return nil }

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

// testCertificate builds a leaf certificate signed by a test authority, so the
// report's own lines about who issued what are exercised the way a real chain
// exercises them: an issuer that is not the subject, and a name that is not
// printed twice in the same sentence.
type certificateOptions struct {
	subject    string
	issuer     string
	dnsNames   []string
	notBefore  time.Time
	notAfter   time.Time
	selfIssued bool
}

func testCertificate(t *testing.T, o certificateOptions) *x509.Certificate {
	t.Helper()

	if o.subject == "" {
		o.subject = "example.com"
	}
	if o.issuer == "" {
		o.issuer = "Test Authority"
	}
	if o.notBefore.IsZero() {
		o.notBefore = time.Now().Add(-24 * time.Hour)
	}
	if o.notAfter.IsZero() {
		o.notAfter = time.Now().Add(90 * 24 * time.Hour)
	}

	authorityKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating the test authority's key failed: %v", err)
	}
	authority := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: o.issuer},
		NotBefore:             o.notBefore,
		NotAfter:              o.notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	authorityRaw, err := x509.CreateCertificate(rand.Reader, authority, authority,
		&authorityKey.PublicKey, authorityKey)
	if err != nil {
		t.Fatalf("creating the test authority failed: %v", err)
	}
	authorityCert, err := x509.ParseCertificate(authorityRaw)
	if err != nil {
		t.Fatalf("parsing the test authority failed: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating the test key failed: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(20261016),
		Subject:      pkix.Name{CommonName: o.subject},
		NotBefore:    o.notBefore,
		NotAfter:     o.notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     o.dnsNames,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, authorityCert, &leafKey.PublicKey, authorityKey)
	if err != nil {
		t.Fatalf("creating the test certificate failed: %v", err)
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatalf("parsing the test certificate failed: %v", err)
	}
	if o.selfIssued {
		cert.Issuer = cert.Subject
	}
	return cert
}

// handedState is the connection state a stubbed handshake returns.
func handedState(cert *x509.Certificate) tls.ConnectionState {
	return tls.ConnectionState{
		Version:            tls.VersionTLS13,
		CipherSuite:        tls.TLS_AES_128_GCM_SHA256,
		NegotiatedProtocol: "h2",
		PeerCertificates:   []*x509.Certificate{cert},
	}
}

// ============================================================================
// DNS
// ============================================================================

func TestCheckDNSSaysWhatAnsweredAndHowLongItTook(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return []string{"140.82.121.6", "140.82.121.7"}, nil
		}
	})

	check, facts := checkDNS(p, checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: %v", check.Status, check.Evidence)
	}
	if check.ID != "dns.resolve" || check.Stage != StageResolver {
		t.Errorf("the check is %s in stage %s, want dns.resolve in stage dns", check.ID, check.Stage)
	}
	if len(facts.Addresses) != 2 || facts.Addresses[0] != "140.82.121.6" {
		t.Errorf("the facts hold %v, want the addresses the resolver answered", facts.Addresses)
	}
	if !evidenceHas(check, "the system resolver answered 140.82.121.6, 140.82.121.7") {
		t.Errorf("the evidence does not state the answer: %v", check.Evidence)
	}
	if !evidenceHas(check, "2 addresses were returned") {
		t.Errorf("the evidence does not state that several addresses were returned: %v", check.Evidence)
	}
}

func TestCheckDNSSaysTheNameDoesNotExist(t *testing.T) {
	p := checksTestProbe(t, nil)

	check, facts := checkDNS(p, checksTestSession(t, "no-such-host.example", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed for a name the resolver says does not exist", check.Status)
	}
	if !strings.Contains(check.Cause, "does not exist") {
		t.Errorf("the cause is %q, want it to say the name does not exist", check.Cause)
	}
	if len(facts.Addresses) != 0 {
		t.Errorf("the facts hold %v, want nothing: the name resolved to nothing", facts.Addresses)
	}
}

func TestCheckDNSSaysTheResolverStayedSilent(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return nil, &net.DNSError{Err: "i/o timeout", Name: host, IsTimeout: true}
		}
	})

	check, _ := checkDNS(p, checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed when the resolver never answered", check.Status)
	}
	if !strings.Contains(check.Cause, "bound") {
		t.Errorf("the cause is %q, want it to name the bound that was reached", check.Cause)
	}
}

func TestCheckDNSReportsAnEmptyAnswerAsUndetermined(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return nil, nil
		}
	})

	check, _ := checkDNS(p, checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown: an answer without an address is not a successful resolution",
			check.Status)
	}
	if check.Next == "" {
		t.Error("the check names no next step for an answer it could not state")
	}
}

func TestCheckDNSDoesNotResolveALiteralAddress(t *testing.T) {
	lookups := 0
	p := checksTestProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			lookups++
			return nil, errors.New("nothing should be resolved here")
		}
	})

	check, facts := checkDNS(p, checksTestSession(t, "140.82.121.6", nil))

	if lookups != 0 {
		t.Errorf("a literal address was resolved %d times, want none", lookups)
	}
	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok", check.Status)
	}
	if !evidenceHas(check, "already an address") {
		t.Errorf("the evidence does not say why nothing was resolved: %v", check.Evidence)
	}
	if len(facts.Addresses) != 1 || facts.Addresses[0] != "140.82.121.6" {
		t.Errorf("the facts hold %v, want the literal address", facts.Addresses)
	}
}

func TestCheckDNSNamesTheServerThatAnsweredDirectly(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.ReadFile = func(path string) ([]byte, error) {
			if path == "/etc/resolv.conf" {
				return []byte("nameserver 10.0.0.1\nnameserver 10.0.0.2\n"), nil
			}
			return nil, fmt.Errorf("open %s: %w", path, errors.New("not found"))
		}
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			if r == net.DefaultResolver {
				return []string{"140.82.121.6"}, nil
			}
			return []string{"140.82.121.6"}, nil
		}
	})

	check, facts := checkDNS(p, checksTestSession(t, "api.github.com", nil))

	if facts.Server != "10.0.0.1" {
		t.Errorf("the facts name the server as %q, want the first configured one", facts.Server)
	}
	if !evidenceHas(check, "10.0.0.1, the first configured server, answered 140.82.121.6") {
		t.Errorf("the evidence does not name the server that answered: %v", check.Evidence)
	}
}

func TestCheckDNSSaysAServerDidNotAnswerWhileTheSystemOneDid(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.ReadFile = func(path string) ([]byte, error) {
			return []byte("nameserver 10.0.0.1\n"), nil
		}
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			if r == net.DefaultResolver {
				return []string{"140.82.121.6"}, nil
			}
			return nil, &net.DNSError{Err: "i/o timeout", Name: host, IsTimeout: true}
		}
	})

	check, _ := checkDNS(p, checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: the system resolver answered, and a difference is a fact rather "+
			"than a contradiction: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "10.0.0.1, the first configured server, did not answer") {
		t.Errorf("the evidence does not state that the configured server stayed silent: %v", check.Evidence)
	}
}

func TestCheckDNSSkipsWithoutADestination(t *testing.T) {
	check, facts := checkDNS(checksTestProbe(t, nil), checksTestOverviewSession(Destination{}))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when there is nothing to resolve", check.Status)
	}
	if len(facts.Addresses) != 0 {
		t.Errorf("the facts hold %v, want nothing", facts.Addresses)
	}
}

// ============================================================================
// TCP
// ============================================================================

func TestCheckTCPReportsTheHandshakeTimeAsTheLatency(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return stubConn{local: "192.168.1.42:51234", remote: "140.82.121.6:443"}, nil
		}
	})

	check := checkTCP(p, checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: %v", check.Status, check.Evidence)
	}
	if check.Duration <= 0 {
		t.Error("the check reports no duration, so the handshake time is not what the report states")
	}
	if !evidenceHas(check, "the TCP handshake with 140.82.121.6:443 was established") {
		t.Errorf("the evidence does not state the handshake: %v", check.Evidence)
	}
	if !evidenceHas(check, "no ping was sent") {
		t.Errorf("the evidence does not say the latency is the handshake's rather than a ping's: %v",
			check.Evidence)
	}
}

func TestCheckTCPTellsARefusalFromASilence(t *testing.T) {
	refusedProbe := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("dial tcp 140.82.121.6:443: connect: connection refused")
		}
	})
	refused := checkTCP(refusedProbe, checksTestSession(t, "api.github.com", nil))
	if refused.Status != StatusFail {
		t.Fatalf("a refusal is %s, want failed", refused.Status)
	}
	if !strings.Contains(refused.Cause, "refused the connection") ||
		!strings.Contains(refused.Cause, "host itself is reachable") {
		t.Errorf("the cause is %q, want it to separate a refusal from a silence", refused.Cause)
	}

	silentProbe := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
	})
	silent := checkTCP(silentProbe, checksTestSession(t, "api.github.com", nil))
	if silent.Status != StatusFail {
		t.Fatalf("a silence is %s, want failed", silent.Status)
	}
	if !strings.Contains(silent.Cause, "firewall") {
		t.Errorf("the cause is %q, want it to name the shape of a filtered connection", silent.Cause)
	}
	if silent.Cause == refused.Cause {
		t.Error("a refusal and a silence reach the same conclusion, so the reader cannot tell them apart")
	}
}

func TestCheckTCPNamesAnUnreachableNetworkAsARoutingProblem(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("dial tcp 10.9.9.9:443: connect: no route to host")
		}
	})

	check := checkTCP(p, checksTestSession(t, "10.9.9.9", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed", check.Status)
	}
	if !strings.Contains(check.Cause, "never left this machine") {
		t.Errorf("the cause is %q, want it to say the packets never left", check.Cause)
	}
	if !strings.Contains(check.Next, "default route") {
		t.Errorf("the next step is %q, want it to point at the route the local stage describes", check.Next)
	}
}

func TestCheckTCPStopsAtTheAttemptBound(t *testing.T) {
	dials := 0
	p := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			dials++
			return nil, errors.New("connect: connection refused")
		}
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return []string{"140.82.121.6", "140.82.121.7", "140.82.121.8", "140.82.121.9", "140.82.121.10"}, nil
		}
	})

	session := checksTestSession(t, "api.github.com", nil)
	_, facts := checkDNS(p, session)
	session.Facts = facts
	check := checkTCP(p, session)

	if dials != DefaultBounds().Attempts {
		t.Errorf("the check dialled %d addresses, want the attempt bound of %d", dials, DefaultBounds().Attempts)
	}
	if !evidenceHas(check, "stopped after the attempt bound") {
		t.Errorf("the evidence does not say the attempts were cut short: %v", check.Evidence)
	}
}

func TestCheckTCPReportsWhichAddressAnswered(t *testing.T) {
	attempts := 0
	p := checksTestProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			attempts++
			if strings.Contains(address, "140.82.121.6") {
				return nil, errors.New("connect: connection refused")
			}
			return stubConn{remote: address}, nil
		}
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return []string{"140.82.121.6", "140.82.121.7"}, nil
		}
	})

	session := checksTestSession(t, "api.github.com", nil)
	_, facts := checkDNS(p, session)
	session.Facts = facts
	check := checkTCP(p, session)

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when one of the addresses answered: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "was established") {
		t.Errorf("the evidence does not state the handshake: %v", check.Evidence)
	}
	if !evidenceHas(check, "the earlier 1 addresses did not answer") {
		t.Errorf("the evidence does not say which address answered and which did not: %v", check.Evidence)
	}
}

func TestCheckTCPSkipsWithoutADestination(t *testing.T) {
	check := checkTCP(checksTestProbe(t, nil), checksTestOverviewSession(Destination{}))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when there is nothing to connect to", check.Status)
	}
}

// ============================================================================
// HTTP
// ============================================================================

func TestCheckHTTPReportsTheStatusAndNeverTheBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"secret":"token-from-the-service"}`)
	}))
	defer server.Close()

	check := checkHTTP(checksTestProbe(t, nil), checksTestSession(t, server.URL+"/status", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "the service answered 200 OK") {
		t.Errorf("the evidence does not state the status: %v", check.Evidence)
	}
	if !evidenceHas(check, "application/json") {
		t.Errorf("the evidence does not state the content type: %v", check.Evidence)
	}
	if !evidenceHas(check, "a response body is never part of this report") {
		t.Errorf("the evidence does not say the body was discarded: %v", check.Evidence)
	}
	for _, line := range check.Evidence {
		if strings.Contains(line, "token-from-the-service") {
			t.Fatalf("the report kept a response body: %q", line)
		}
	}
}

func TestCheckHTTPBoundsWhatItReads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(make([]byte, 200<<10))
	}))
	defer server.Close()

	check := checkHTTP(checksTestProbe(t, nil), checksTestSession(t, server.URL+"/page", nil))

	if !evidenceHas(check, fmt.Sprintf("%d bytes of the body were read and discarded",
		DefaultBounds().BodyBytes)) {
		t.Errorf("the report does not say it read exactly the bound's worth of a 200 KiB body: %v", check.Evidence)
	}
}

func TestCheckHTTPReportsAnApplicationErrorAsAnAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	check := checkHTTP(checksTestProbe(t, nil), checksTestSession(t, server.URL+"/missing", nil))

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted: the service answered, so the network is not what failed",
			check.Status)
	}
	if !strings.Contains(check.Cause, "the application's own decision") {
		t.Errorf("the cause is %q, want it to say the service made the decision", check.Cause)
	}
}

// TestCheckHTTPSaysWhenTheRequestEndedSomewhereElse pins the fact the interception
// check builds on: a request that did not end where it was sent says so, and the
// answer finally received is the one that is reported.
func TestCheckHTTPSaysWhenTheRequestEndedSomewhereElse(t *testing.T) {
	landing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer landing.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, landing.URL+"/login", http.StatusFound)
	}))
	defer server.Close()

	check := checkHTTP(checksTestProbe(t, nil), checksTestSession(t, server.URL+"/start", nil))

	if !evidenceHas(check, "the request ended at") {
		t.Errorf("the evidence does not say where the request ended: %v", check.Evidence)
	}
	if !evidenceHas(check, "the service answered 200 OK") {
		t.Errorf("the evidence does not state the answer that was finally received: %v", check.Evidence)
	}
}

// TestLandsOnAnotherHost pins the comparison the report makes its own line from:
// the names are compared rather than the ports, and the comparison is not fooled
// by the case a host name may be written in.
func TestLandsOnAnotherHost(t *testing.T) {
	cases := []struct {
		target string
		landed string
		want   bool
	}{
		{"https://api.github.com/x", "https://api.github.com/y", false},
		{"https://api.github.com/x", "https://api.github.com:8443/y", false},
		{"https://api.github.com/x", "https://portal.example/login", true},
		{"https://api.github.com/x", "https://API.GitHub.com/y", false},
	}
	for _, c := range cases {
		target, err := url.Parse(c.target)
		if err != nil {
			t.Fatalf("parsing %q failed: %v", c.target, err)
		}
		landed, err := url.Parse(c.landed)
		if err != nil {
			t.Fatalf("parsing %q failed: %v", c.landed, err)
		}
		if got := landsOnAnotherHost(target, landed); got != c.want {
			t.Errorf("landsOnAnotherHost(%q, %q) = %v, want %v", c.target, c.landed, got, c.want)
		}
	}
}

func TestCheckHTTPStopsAtTheRedirectBound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	defer server.Close()

	session := checksTestSession(t, server.URL+"/loop", func(o *Options) { o.Bounds.Redirects = 2 })
	check := checkHTTP(checksTestProbe(t, nil), session)

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted when the chain was cut", check.Status)
	}
	if !evidenceHas(check, "the chain was cut at the bound of 2") {
		t.Errorf("the evidence does not say the redirects were cut: %v", check.Evidence)
	}
}

func TestCheckHTTPReportsARequestThatNeverAnswered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	session := checksTestSession(t, server.URL+"/slow", func(o *Options) { o.Bounds.HTTP = 50 * time.Millisecond })
	check := checkHTTP(checksTestProbe(t, nil), session)

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed when nothing answered inside the bound", check.Status)
	}
	if !strings.Contains(check.Cause, "bound") {
		t.Errorf("the cause is %q, want it to name the bound", check.Cause)
	}
}

func TestCheckHTTPNeverPrintsAQueryString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	check := checkHTTP(checksTestProbe(t, nil), checksTestSession(t, server.URL+"/auth?token=s3cr3t-token", nil))

	for _, line := range check.Evidence {
		if strings.Contains(line, "s3cr3t-token") {
			t.Fatalf("the report printed a query string: %q", line)
		}
	}
	if !evidenceHas(check, "the request carried a query string, which is not shown here") {
		t.Errorf("the evidence does not say a query was dropped: %v", check.Evidence)
	}
}

func TestCheckHTTPSkipsADestinationItCannotRequest(t *testing.T) {
	// The bound is tight on purpose: the point of the skip is that no request is
	// made at all, so a regression that made one anyway has to fail in
	// milliseconds rather than reach a real host.
	session := checksTestSession(t, "example.com:22", func(o *Options) { o.Bounds.HTTP = 50 * time.Millisecond })
	check := checkHTTP(checksTestProbe(t, nil), session)

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped for a destination that is not http or https: a bare port is a "+
			"port, and asking it an HTTP question would be a guess", check.Status)
	}
	if !strings.Contains(check.Cause, "enter a URL") {
		t.Errorf("the reason is %q, want it to say what would diagnose HTTP on that port", check.Cause)
	}
}

// TestSchemeForPort pins the ports this utility is willing to assume a scheme for.
// The list is deliberately short: every port in it is one whose whole traffic is
// the web protocol, so turning a bare host:port into a request is a fact about
// the port rather than a guess about somebody's service.
func TestSchemeForPort(t *testing.T) {
	cases := []struct {
		port   int
		scheme string
		ok     bool
	}{
		{80, "http", true},
		{8080, "http", true},
		{443, "https", true},
		{8443, "https", true},
		{9443, "https", true},
		{22, "", false},
		{25, "", false},
		{3306, "", false},
	}
	for _, c := range cases {
		scheme, ok := schemeForPort(c.port)
		if scheme != c.scheme || ok != c.ok {
			t.Errorf("schemeForPort(%d) = (%q, %v), want (%q, %v)", c.port, scheme, ok, c.scheme, c.ok)
		}
	}
}

// ============================================================================
// TLS
// ============================================================================

func TestCheckTLSReportsWhoAnswered(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "api.github.com", issuer: "Test Authority",
		dnsNames: []string{"api.github.com", "www.api.github.com"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), nil
		}
	})

	check := checkTLS(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: %v", check.Status, check.Evidence)
	}
	if check.Stage != StageCertificate {
		t.Errorf("the check is in stage %s, want certificate", check.Stage)
	}
	for _, want := range []string{
		"completed in",
		"TLS 1.3",
		"the certificate is for api.github.com, www.api.github.com",
		"it was issued by CN=Test Authority",
		"it covers 2 names",
		"verification was not skipped",
	} {
		if !evidenceHas(check, want) {
			t.Errorf("the evidence does not state %q: %v", want, check.Evidence)
		}
	}
}

func TestCheckTLSRefusesACertificateForAnotherName(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "wrong.example", dnsNames: []string{"wrong.example"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), x509.HostnameError{Certificate: cert, Host: serverName}
		}
	})

	check := checkTLS(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed for a certificate that does not cover the host", check.Status)
	}
	if !strings.Contains(check.Cause, "does not cover") {
		t.Errorf("the cause is %q, want it to say the name does not match", check.Cause)
	}
	if !evidenceHas(check, "it does not cover api.github.com") {
		t.Errorf("the evidence does not state which name was asked for: %v", check.Evidence)
	}
}

func TestCheckTLSRefusesAnAuthorityThisMachineDoesNotTrust(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			if err != nil {
				return tls.ConnectionState{}, err
			}
			defer conn.Close()
			tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName})
			handshakeErr := tlsConn.HandshakeContext(ctx)
			return tlsConn.ConnectionState(), handshakeErr
		}
	})

	host, _, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("reading the test server's address failed: %v", err)
	}
	session := checksTestSession(t, "https://"+host+":443/", nil)
	check := checkTLS(p, session)

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed: a certificate from an authority this machine does not trust is "+
			"the finding, never something to accept: %v", check.Status, check.Evidence)
	}
	if !strings.Contains(check.Cause, "not signed by an authority this machine trusts") {
		t.Errorf("the cause is %q, want it to name the untrusted authority", check.Cause)
	}
	if strings.Contains(check.Cause+strings.Join(check.Evidence, " "), "InsecureSkipVerify") {
		t.Error("the report mentions skipping verification, which this utility never offers")
	}
}

func TestCheckTLSRefusesAnExpiredCertificate(t *testing.T) {
	cert := testCertificate(t, certificateOptions{
		subject:   "api.github.com",
		dnsNames:  []string{"api.github.com"},
		notBefore: time.Now().Add(-48 * time.Hour),
		notAfter:  time.Now().Add(-24 * time.Hour),
	})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), x509.CertificateInvalidError{Cert: cert, Reason: x509.Expired}
		}
	})

	check := checkTLS(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed", check.Status)
	}
	if !strings.Contains(check.Cause, "validity window") {
		t.Errorf("the cause is %q, want it to say the certificate is outside its window", check.Cause)
	}
	if !strings.Contains(check.Next, "clock") {
		t.Errorf("the next step is %q, want it to mention this machine's clock as well", check.Next)
	}
}

func TestCheckTLSCallsOutAnObsoleteProtocol(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "api.github.com", dnsNames: []string{"api.github.com"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			state := handedState(cert)
			state.Version = tls.VersionTLS10
			return state, nil
		}
	})

	check := checkTLS(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted for a protocol every platform calls obsolete", check.Status)
	}
	if !strings.Contains(check.Cause, "obsolete") {
		t.Errorf("the cause is %q, want it to name the obsolete protocol", check.Cause)
	}
}

func TestCheckTLSSkipsADestinationThatIsNotTLS(t *testing.T) {
	check := checkTLS(checksTestProbe(t, nil), checksTestSession(t, "example.com:22", nil))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when there is no certificate to validate", check.Status)
	}
	if !strings.Contains(check.Cause, "not https") {
		t.Errorf("the reason is %q, want it to say the destination is not https", check.Cause)
	}
}

// ============================================================================
// The traceroute
// ============================================================================

func TestCheckTraceIsOffUnlessItWasAskedFor(t *testing.T) {
	check := checkTrace(checksTestProbe(t, nil), checksTestSession(t, "api.github.com", nil))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped: the traceroute is opt-in", check.Status)
	}
	if check.Manual == "" {
		t.Error("the skip does not say how to get the path")
	}
}

func TestCheckTraceIsNotRunForAnOverview(t *testing.T) {
	p := checksTestProbe(t, nil)
	session := checksTestSession(t, "", func(o *Options) {
		o.Trace = true
		o.Destination = Destination{Host: "api.github.com", Port: 443}
	})

	check := checkTrace(p, session)

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped: a traceroute needs a destination the user entered", check.Status)
	}
}

func TestCheckTraceSkipsWhenTheToolIsNotInstalled(t *testing.T) {
	p := checksTestProbe(t, nil)
	check := checkTrace(p, checksTestSession(t, "api.github.com", func(o *Options) { o.Trace = true }))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when neither tool exists", check.Status)
	}
	if !strings.Contains(check.Cause, "installs nothing") {
		t.Errorf("the reason is %q, want it to say nothing is installed to get the path", check.Cause)
	}
}

func TestCheckTraceSaysWhatNeedsWhat(t *testing.T) {
	read := false
	p := checksTestProbe(t, func(p *Probe) {
		p.LookPath = func(name string) (string, error) {
			if name == "tracepath" {
				return "/usr/bin/tracepath", nil
			}
			return "", errors.New("not installed")
		}
		p.ExecOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			read = true
			if name != "tracepath" {
				t.Errorf("the check ran %s, want the tool it found first", name)
			}
			if len(args) != 2 || args[0] != "-n" || args[1] != "api.github.com" {
				t.Errorf("the program was run with %v, want numeric output and the validated host", args)
			}
			return []byte(" 1  192.168.1.1  1.234ms\n 2  *\n 3  140.82.121.6  9.876ms\n"), nil
		}
	})

	check := checkTrace(p, checksTestSession(t, "api.github.com", func(o *Options) { o.Trace = true }))

	if !read {
		t.Fatal("the tool was never run")
	}
	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when the path arrived: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "2 hops answered and 1 stayed silent") {
		t.Errorf("the evidence does not count the silence: %v", check.Evidence)
	}
	if !evidenceHas(check, "it is not by itself a fault") {
		t.Errorf("the evidence does not say a silent hop is filtering rather than a fault: %v", check.Evidence)
	}
}

func TestCheckTraceSaysWhenTheBoundCutIt(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.LookPath = func(name string) (string, error) { return "/usr/bin/traceroute", nil }
		p.ExecOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			<-ctx.Done()
			return []byte(" 1  192.168.1.1  1.234ms\n"), ctx.Err()
		}
	})

	session := checksTestSession(t, "api.github.com", func(o *Options) {
		o.Trace = true
		o.Bounds.Trace = 30 * time.Millisecond
	})
	check := checkTrace(p, session)

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted when the bound cut the trace: %v", check.Status, check.Evidence)
	}
	if !strings.Contains(check.Cause, "cut when the") {
		t.Errorf("the cause is %q, want it to say the trace was cut rather than that it ended", check.Cause)
	}
}

func TestCheckTraceIsUndeterminedWhenItCannotRun(t *testing.T) {
	p := checksTestProbe(t, func(p *Probe) {
		p.LookPath = func(name string) (string, error) { return "/usr/bin/traceroute", nil }
		p.ExecOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("operation not permitted")
		}
	})

	check := checkTrace(p, checksTestSession(t, "api.github.com", func(o *Options) { o.Trace = true }))

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown when the tool could not run at all", check.Status)
	}
	if check.Manual == "" {
		t.Error("the check does not offer the command to run by hand")
	}
}

// ============================================================================
// The interception cross-check
// ============================================================================

func TestCheckInterceptionConfirmsAMatchingCertificate(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "api.github.com", dnsNames: []string{"api.github.com"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), nil
		}
	})

	check := checkInterception(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "which covers api.github.com") {
		t.Errorf("the evidence does not state what settles the question: %v", check.Evidence)
	}
}

func TestCheckInterceptionFailsWhenSomethingElseAnswered(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "filter.corp", dnsNames: []string{"filter.corp"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), x509.UnknownAuthorityError{Cert: cert}
		}
	})

	check := checkInterception(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed when the certificate that answered is not for the destination",
			check.Status)
	}
	if !strings.Contains(check.Cause, "something in the path answered instead") {
		t.Errorf("the cause is %q, want it to say something answered in the destination's place", check.Cause)
	}
	if !strings.Contains(check.Next, "disabling verification") {
		t.Errorf("the next step is %q, want it to refuse the shortcut of skipping verification", check.Next)
	}
}

func TestCheckInterceptionNotesAProxyWithAMatchingCertificate(t *testing.T) {
	cert := testCertificate(t, certificateOptions{subject: "api.github.com", dnsNames: []string{"api.github.com"}})
	p := checksTestProbe(t, func(p *Probe) {
		p.Environ = func() []string { return []string{"https_proxy=http://proxy.corp:3128"} }
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), nil
		}
	})

	check := checkInterception(p, checksTestSession(t, "https://api.github.com/", nil))

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted: a proxy is a fact, and the certificate still matched", check.Status)
	}
	if !strings.Contains(check.Cause, "still matches") {
		t.Errorf("the cause is %q, want it to say nothing is pretending to be the destination", check.Cause)
	}
}

func TestCheckInterceptionIsUndeterminedWhenItHasNoEvidence(t *testing.T) {
	check := checkInterception(checksTestProbe(t, nil), checksTestSession(t, "example.com:80", nil))

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown: without a certificate there is nothing to compare, and this "+
			"check concludes nothing from an inference", check.Status)
	}
	if !strings.Contains(check.Next, "https") {
		t.Errorf("the next step is %q, want it to point at the evidence that would settle it", check.Next)
	}
}

func TestCheckInterceptionSkipsWithoutADestination(t *testing.T) {
	check := checkInterception(checksTestProbe(t, nil), checksTestOverviewSession(Destination{}))

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when there is nothing to compare against", check.Status)
	}
}
