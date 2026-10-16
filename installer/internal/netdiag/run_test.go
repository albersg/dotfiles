package netdiag

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// stubRoundTripper answers a request with what the test asks for, so the HTTP
// check can be exercised without a server: no port is opened and nothing is
// reached.
type stubRoundTripper struct {
	status int
	header http.Header
	body   string
}

func (rt stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	header := rt.header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Request:    &http.Request{URL: req.URL},
	}, nil
}

// diagnosisProbe is a machine where everything works: one interface with an
// address, a default route, a configured resolver, an answer, a socket, a
// certificate that covers the destination and a service that answers 200. It is
// the baseline a test bends to study one stage at a time.
func diagnosisProbe(t *testing.T, mutate func(*Probe)) Probe {
	t.Helper()

	return checksTestProbe(t, func(p *Probe) {
		p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
			[]net.Interface{{Name: "eth0", Flags: interfaceUp}},
			map[string][]string{"eth0": {"192.168.1.42/24"}})
		p.ReadFile = func(path string) ([]byte, error) {
			switch path {
			case "/proc/net/route":
				return envTestRouteFile("eth0\t00000000\t0102A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0")(path)
			case "/etc/resolv.conf":
				return []byte("nameserver 10.0.0.1\n"), nil
			}
			return nil, fmt.Errorf("open %s: no such file", path)
		}
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			return []string{"140.82.121.6"}, nil
		}
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return stubConn{local: "192.168.1.42:51234", remote: address}, nil
		}
		cert := testCertificate(t, certificateOptions{
			subject: "api.github.com", dnsNames: []string{"api.github.com"}})
		p.TLSHandshake = func(ctx context.Context, address, serverName string) (tls.ConnectionState, error) {
			return handedState(cert), nil
		}
		p.HTTPClient = func(b Bounds) *http.Client {
			return &http.Client{Transport: stubRoundTripper{
				status: http.StatusOK,
				header: http.Header{"Content-Type": {"text/plain"}},
				body:   "ok",
			}}
		}
		if mutate != nil {
			mutate(p)
		}
	})
}

// overviewOptions is the run the screen asks for when nothing was typed: one
// declared destination.
func overviewOptions() Options {
	return Options{
		Destination: Destination{
			Host:   "api.github.com",
			Port:   443,
			Source: "the host this installer already contacts for its own update check",
		},
		Bounds: DefaultBounds(),
	}
}

// checkIDs is the report's checks by identifier, in the order they are in it.
func checkIDs(report Report) []string {
	ids := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		ids = append(ids, check.ID)
	}
	return ids
}

func TestRunFollowsTheStageOrderAndStreamsEveryCheck(t *testing.T) {
	streamed := []Check{}
	report := Run(context.Background(), diagnosisProbe(t, nil), overviewOptions(), func(check Check) {
		streamed = append(streamed, check)
	})

	want := []string{
		"env.interfaces", "env.route", "env.resolver", "dns.resolve", "env.proxy",
		"tcp.connect", "trace.route", "tls.handshake", "http.get", "http.interception",
	}
	got := checkIDs(report)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the report holds %v, want the stages in the order a connection happens: %v", got, want)
	}
	if len(streamed) != len(report.Checks) {
		t.Fatalf("%d checks were streamed and the report holds %d: the screen would show a run that "+
			"never finished", len(streamed), len(report.Checks))
	}
	for i, check := range streamed {
		if check.ID != report.Checks[i].ID {
			t.Errorf("the streamed check %d is %s and the report holds %s: the two are not the same run",
				i, check.ID, report.Checks[i].ID)
		}
	}
	if !strings.Contains(report.Verdict(), "every stage answered") {
		t.Errorf("the verdict is %q, want it to say the destination was reached end to end",
			report.Verdict())
	}
	if report.Stopped {
		t.Error("the report says the run was stopped when nothing stopped it")
	}
	if report.Elapsed <= 0 {
		t.Error("the report states no duration for a run that ran")
	}
	if report.Endpoint != "api.github.com:443" {
		t.Errorf("the report names the destination as %q, want what was dialled", report.Endpoint)
	}
	if report.Source == "" {
		t.Error("the report does not say where the destination came from")
	}
}

func TestRunSkipsTheStagesTheTransportNeverReached(t *testing.T) {
	p := diagnosisProbe(t, func(p *Probe) {
		p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, fmt.Errorf("dial tcp %s: connect: connection refused", address)
		}
	})

	report := Run(context.Background(), p, overviewOptions(), nil)

	byID := map[string]Check{}
	for _, check := range report.Checks {
		byID[check.ID] = check
	}
	transport := byID["tcp.connect"]
	if transport.Status != StatusFail {
		t.Fatalf("the transport is %s, want failed: the destination refused the connection", transport.Status)
	}
	for _, id := range []string{"tls.handshake", "http.get", "http.interception"} {
		check := byID[id]
		if check.Status != StatusSkipped {
			t.Errorf("%s is %s, want skipped: five failures with one cause read as five problems", id, check.Status)
			continue
		}
		if !strings.Contains(check.Cause, "the transport did not connect") {
			t.Errorf("%s says it was skipped because %q, want it to name the stage that stopped the run",
				id, check.Cause)
		}
		if check.ID == "" || check.Title == "" {
			t.Errorf("%s is in the report without a name, so the reader cannot tell what did not run", id)
		}
	}

	fault, ok := report.FaultCheck()
	if !ok || fault.ID != "tcp.connect" {
		t.Errorf("the fault is %v, want the transport alone", fault.ID)
	}
	if !strings.Contains(report.Verdict(), "the transport did not answer") {
		t.Errorf("the verdict is %q, want it to name the transport", report.Verdict())
	}
	if summary := report.Summary(); summary.Failed != 1 {
		t.Errorf("the report counts %d failures, want exactly one: the skipped stages did not fail",
			summary.Failed)
	}
}

func TestRunReportsAStoppedRunWithoutLosingWhatFinished(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := diagnosisProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			// The user pressed the key that stops the run while the resolution was
			// in flight.
			cancel()
			return nil, context.Canceled
		}
	})

	report := Run(ctx, p, overviewOptions(), nil)

	if !report.Stopped {
		t.Fatal("the report does not say the run was stopped")
	}
	byID := map[string]Check{}
	for _, check := range report.Checks {
		byID[check.ID] = check
	}
	if byID["env.interfaces"].Status != StatusOK {
		t.Errorf("a check that finished before the stop is %s, want the answer it found",
			byID["env.interfaces"].Status)
	}
	if byID["dns.resolve"].Status != StatusFail {
		t.Errorf("the check that was interrupted is %s, want it to report what happened to it",
			byID["dns.resolve"].Status)
	}
	for _, id := range []string{"tcp.connect", "tls.handshake", "http.get"} {
		check := byID[id]
		if check.Status != StatusSkipped {
			t.Errorf("%s is %s, want skipped: a stopped run says what it did not reach", id, check.Status)
			continue
		}
		if !strings.Contains(check.Cause, "stopped before this check ran") {
			t.Errorf("%s says %q, want it to name the stop as the reason", id, check.Cause)
		}
	}
	if len(report.Checks) != 10 {
		t.Errorf("the stopped report holds %d checks, want all ten: a reader has to see what did not run",
			len(report.Checks))
	}
}

func TestRunBoundsTheWholeRun(t *testing.T) {
	p := diagnosisProbe(t, func(p *Probe) {
		p.LookupHost = func(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
	})
	options := overviewOptions()
	options.Bounds.Total = 40 * time.Millisecond

	started := time.Now()
	report := Run(context.Background(), p, options, nil)
	elapsed := time.Since(started)

	if elapsed > 5*time.Second {
		t.Fatalf("the run took %s with a total bound of 40 ms: the bound is what keeps the screen honest",
			elapsed)
	}
	if !report.Stopped {
		t.Error("the report does not say the run ended before its last check")
	}
	last := report.Checks[len(report.Checks)-1]
	if last.Status != StatusSkipped || !strings.Contains(last.Cause, "bound of 40 ms") {
		t.Errorf("the last check is %s saying %q, want it to name the run's own bound", last.Status, last.Cause)
	}
}

func TestRunNamesTheDestinationTheUserTyped(t *testing.T) {
	target, err := ParseTarget("api.github.com:8443")
	if err != nil {
		t.Fatalf("the test target is one the validator refuses: %v", err)
	}
	options := Options{Target: target, Bounds: DefaultBounds()}

	report := Run(context.Background(), diagnosisProbe(t, nil), options, nil)

	if report.Target != "api.github.com:8443" {
		t.Errorf("the report names the target as %q, want what the user typed", report.Target)
	}
	if report.Endpoint != "api.github.com:8443" {
		t.Errorf("the report names the endpoint as %q, want what was dialled", report.Endpoint)
	}
}

func TestRunNeverPutsAQueryStringOrACredentialInTheReport(t *testing.T) {
	target, err := ParseTarget("https://alice:s3cr3t@api.github.com/status?token=s3cr3t-token")
	if err != nil {
		t.Fatalf("the test target is one the validator refuses: %v", err)
	}
	report := Run(context.Background(), diagnosisProbe(t, nil),
		Options{Target: target, Bounds: DefaultBounds()}, nil)

	surface := []string{report.Target, report.Endpoint, report.Source, report.Verdict(),
		report.Text(TextOptions{})}
	for _, check := range report.Checks {
		surface = append(surface, check.Evidence...)
		surface = append(surface, check.Cause, check.Next, check.Manual, check.Title)
	}
	for _, line := range surface {
		// The word "token" is not itself a secret and the report explains why a query
		// is dropped; what must never appear is the value that was in the query and
		// the credentials the URL carried.
		for _, secret := range []string{"s3cr3t", "s3cr3t-token", "alice", "@api.github.com/status?"} {
			if strings.Contains(line, secret) {
				t.Fatalf("the report carries %q: %q", secret, line)
			}
		}
	}
}

func TestRunAnonymizesTheReportWhenAsked(t *testing.T) {
	report := Run(context.Background(), diagnosisProbe(t, nil), overviewOptions(), nil)

	plain := report.Text(TextOptions{})
	if !strings.Contains(plain, "140.82.121.6") {
		t.Fatalf("the report does not state the address it reached, so there is nothing to anonymize:\n%s", plain)
	}
	anonymized := report.Text(TextOptions{Anonymize: true})
	if strings.Contains(anonymized, "140.82.121.6") || strings.Contains(anonymized, "192.168.1.42") {
		t.Errorf("the anonymized report still carries an address:\n%s", anonymized)
	}
	if !strings.Contains(anonymized, "<ipv4>") {
		t.Errorf("the anonymized report does not show what replaced the addresses:\n%s", anonymized)
	}
}

func TestNotRunChecksAreNamedLikeTheChecksThemselves(t *testing.T) {
	// The same destination reached two ways: one run whose transport failed, where
	// the certificate check is skipped, and one whose transport worked, where it
	// runs. The two reports have to name that step with the same words, or a reader
	// comparing them sees two different checks.
	//
	// A destination that is not TLS is tried as well as one that is: the certificate
	// check concludes differently for each, and the difference must not be what gives
	// the step a second name.
	for _, raw := range []string{"api.github.com:443", "example.com:80"} {
		t.Run(raw, func(t *testing.T) {
			target, err := ParseTarget(raw)
			if err != nil {
				t.Fatalf("the test target is one the validator refuses: %v", err)
			}
			options := Options{Target: target, Bounds: DefaultBounds()}

			down := Run(context.Background(), diagnosisProbe(t, func(p *Probe) {
				p.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
					return nil, fmt.Errorf("dial tcp %s: connect: connection refused", address)
				}
			}), options, nil)

			up := Run(context.Background(), diagnosisProbe(t, nil), options, nil)

			name := func(report Report, id string) string {
				for _, check := range report.Checks {
					if check.ID == id {
						return check.Title
					}
				}
				return ""
			}
			for _, id := range []string{"tls.handshake", "http.get"} {
				if skipped, ran := name(down, id), name(up, id); skipped != ran {
					t.Errorf("the %s check is named %q when it was skipped and %q when it ran, want one name",
						id, skipped, ran)
				}
			}
		})
	}
}
