package netdiag

import (
	"strings"
	"testing"
	"time"
)

// reportFixture is a small report with one failure in the transport and one
// undetermined check, which is the shape the verdict has to describe.
func reportFixture() Report {
	return Report{
		Target:   "example.com:443",
		Endpoint: "example.com:443",
		Source:   "you entered it",
		Started:  time.Date(2026, time.October, 16, 12, 4, 5, 0, time.UTC),
		Elapsed:  1420 * time.Millisecond,
		Checks: []Check{
			{ID: "env.interfaces", Stage: StageLocal, Title: "Interfaces and addresses", Status: StatusOK,
				Evidence: []string{"enp3s0 is up: 192.168.1.42/24"}, Duration: 120 * time.Microsecond},
			{ID: "env.route", Stage: StageLocal, Title: "Default route", Status: StatusOK,
				Evidence: []string{"the default route is 192.168.1.1 through enp3s0"}},
			{ID: "dns.resolve", Stage: StageResolver, Title: "Resolve example.com", Status: StatusOK,
				Evidence: []string{"the system resolver answered 93.184.216.34"}, Duration: 22 * time.Millisecond},
			{ID: "tcp.connect", Stage: StageTransport, Title: "Connect to example.com:443", Status: StatusFail,
				Evidence: []string{"the connection to 93.184.216.34:443 was refused"},
				Cause:    "nothing is listening on that port",
				Next:     "check the port the service is published on",
				Manual:   "nc -vz example.com 443",
				Duration: 41 * time.Millisecond},
			{ID: "tls.handshake", Stage: StageCertificate, Title: "TLS handshake", Status: StatusSkipped,
				Evidence: []string{"skipped: no connection was established"}},
			{ID: "http.get", Stage: StageApplication, Title: "HTTP request", Status: StatusUnknown,
				Evidence: []string{"the HTTP check needs a URL"},
				Cause:    "the destination was given as a bare host"},
		},
	}
}

// TestSummaryCountsEveryStatus pins the counts the screen and the report print:
// a status that is not counted is a check the reader cannot account for.
func TestSummaryCountsEveryStatus(t *testing.T) {
	got := reportFixture().Summary()
	want := Summary{OK: 3, Failed: 1, Unknown: 1, Skipped: 1}
	if got != want {
		t.Errorf("Summary() = %+v, want %+v", got, want)
	}
	if got.Total() != 6 {
		t.Errorf("Total() = %d, want the six checks in the report", got.Total())
	}
}

// TestStageFaultNamesTheEarliestStageThatFailed pins the answer to "where does
// it break": the stages are ordered the way a request travels through them, so a
// later failure is reported as the consequence of the earliest one.
func TestStageFaultNamesTheEarliestStageThatFailed(t *testing.T) {
	r := reportFixture()

	// A failure recorded out of order must still be attributed to the earlier
	// stage: the check list is in emission order, not in stage order.
	r.Checks = []Check{
		{ID: "http.get", Stage: StageApplication, Title: "HTTP request", Status: StatusFail},
		{ID: "dns.resolve", Stage: StageResolver, Title: "Resolve", Status: StatusFail},
	}
	stage, ok := r.StageFault()
	if !ok {
		t.Fatal("StageFault found no fault in a report with two failures")
	}
	if stage != StageResolver {
		t.Errorf("StageFault = %q, want the earliest stage (DNS) to be the fault", stage)
	}
	fault, ok := r.FaultCheck()
	if !ok || fault.ID != "dns.resolve" {
		t.Errorf("FaultCheck = %+v, want the DNS check that failed first", fault)
	}

	// A report with no failure has no fault, even when checks were skipped or
	// undetermined: only an observed failure locates one.
	healthy := Report{Checks: []Check{
		{ID: "a", Stage: StageLocal, Status: StatusOK},
		{ID: "b", Stage: StageResolver, Status: StatusSkipped},
		{ID: "c", Stage: StageTransport, Status: StatusUnknown},
		{ID: "d", Stage: StageProxy, Status: StatusNote},
	}}
	if stage, ok := healthy.StageFault(); ok {
		t.Errorf("StageFault = %q in a report with no failure, want none", stage)
	}
}

// TestVerdictSaysWhereItBreaksOrThatItCannot pins the one sentence the screen
// leads with, in all five shapes it can have.
func TestVerdictSaysWhereItBreaksOrThatItCannot(t *testing.T) {
	cases := []struct {
		name   string
		report Report
		want   string
	}{
		{
			name:   "a fault names the stage and the check",
			report: reportFixture(),
			want:   "the transport did not answer: Connect to example.com:443",
		},
		{
			name: "everything answered",
			report: Report{Checks: []Check{
				{ID: "a", Stage: StageLocal, Status: StatusOK},
				{ID: "b", Stage: StageResolver, Status: StatusOK},
			}},
			want: "every stage answered",
		},
		{
			name: "healthy with an undetermined check",
			report: Report{Checks: []Check{
				{ID: "a", Stage: StageLocal, Status: StatusOK},
				{ID: "b", Stage: StageResolver, Status: StatusUnknown},
			}},
			want: "1 could not be determined here",
		},
		{
			name: "nothing determined",
			report: Report{Checks: []Check{
				{ID: "a", Stage: StageLocal, Status: StatusUnknown},
				{ID: "b", Stage: StageResolver, Status: StatusUnknown},
			}},
			want: "no stage could be established on this platform",
		},
		{
			name:   "stopped before anything ran",
			report: Report{Stopped: true},
			want:   "stopped before any check finished",
		},
		{
			name:   "nothing asked for",
			report: Report{},
			want:   "no check was run",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.report.Verdict()
			if !strings.Contains(got, c.want) {
				t.Errorf("Verdict() = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

// TestVerdictSaysWhenAStoppedRunLeftLaterStagesUnchecked pins the honesty a
// cancelled run needs: a failure found before the stop is a failure, and the
// stages that never ran are named as unchecked rather than implied healthy.
func TestVerdictSaysWhenAStoppedRunLeftLaterStagesUnchecked(t *testing.T) {
	r := Report{Stopped: true, Checks: []Check{
		{ID: "dns.resolve", Stage: StageResolver, Title: "Resolve", Status: StatusFail},
	}}
	got := r.Verdict()
	if !strings.Contains(got, "DNS did not answer") {
		t.Errorf("Verdict() = %q, want the fault named", got)
	}
	if !strings.Contains(got, "later stages were not checked") {
		t.Errorf("Verdict() = %q, want it to say the run stopped before the later stages", got)
	}
}

// TestTextIsRedactedByConstruction pins the exported document: the header, the
// steps in order with their evidence, causes, next checks and manual commands,
// and the promise that nothing was changed.
func TestTextIsRedactedByConstruction(t *testing.T) {
	r := reportFixture()
	r.Checks[1].Evidence = []string{"proxy http://alice:s3cr3t@proxy.corp:3128 is configured"}

	text := r.Text(TextOptions{})

	for _, want := range []string{
		"dotfiles network diagnostics",
		"target: example.com:443 (you entered it)",
		"started: 2026-10-16 12:04:05 UTC",
		"elapsed: 1.42 s",
		"checks: 6 run (3 ok, 0 noted, 1 failed, 1 unknown, 1 skipped)",
		"verdict: the transport did not answer",
		"1. [ok] the local network — Interfaces and addresses (0.12 ms)",
		"enp3s0 is up: 192.168.1.42/24",
		"4. [failed] the transport — Connect to example.com:443 (41 ms)",
		"cause: nothing is listening on that port",
		"next: check the port the service is published on",
		"manual: nc -vz example.com 443",
		"Nothing was changed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the report text does not contain %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, "s3cr3t") || strings.Contains(text, "alice") {
		t.Errorf("the text carried a proxy password:\n%s", text)
	}
	if !strings.Contains(text, "http://***:***@proxy.corp:3128") {
		t.Errorf("the text does not show the redacted proxy:\n%s", text)
	}
}

// TestTextAnonymizesWhenTheUserChoosesIt pins the export the support case needs:
// no address, no user name, no home directory and no machine name.
func TestTextAnonymizesWhenTheUserChoosesIt(t *testing.T) {
	t.Setenv("HOME", "/home/alber")
	t.Setenv("USER", "alber")
	t.Setenv("USERNAME", "")

	r := reportFixture()
	r.Checks[0].Evidence = []string{"enp3s0 is up: 192.168.1.42/24 at /home/alber, machine alber-pc"}
	r.Checks[2].Evidence = []string{"the system resolver answered 93.184.216.34 and fe80::1"}

	text := r.Text(TextOptions{Anonymize: true, HostName: "alber-pc"})

	for _, secret := range []string{"192.168.1.42", "93.184.216.34", "fe80::1", "/home/alber", "alber-pc", "alber"} {
		if strings.Contains(text, secret) {
			t.Errorf("the anonymized text still contains %q:\n%s", secret, text)
		}
	}
	for _, want := range []string{"<ipv4>/24", "~", "<host>", "2026-10-16 12:04:05 UTC"} {
		if !strings.Contains(text, want) {
			t.Errorf("the anonymized text lost %q:\n%s", want, text)
		}
	}
}

// TestDurationNeverRoundsATimeoutAway pins the number format: a check that took
// a measurable time prints one, and the zero used for a check that ran nothing is
// printed as zero rather than as an empty cell.
func TestDurationNeverRoundsATimeoutAway(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{in: 0, want: "0 ms"},
		{in: -time.Second, want: "0 ms"},
		{in: 120 * time.Microsecond, want: "0.12 ms"},
		{in: 41 * time.Millisecond, want: "41 ms"},
		{in: 999 * time.Millisecond, want: "999 ms"},
		{in: 1420 * time.Millisecond, want: "1.42 s"},
		{in: 12 * time.Second, want: "12.0 s"},
	}

	for _, c := range cases {
		if got := Duration(c.in); got != c.want {
			t.Errorf("Duration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCheckHelpersKeepTheReasonFieldsTogether pins the two constructors the
// check functions build their results with, so a step cannot end up failed
// without the cause that explains it or healthy with a stale one.
func TestCheckHelpersKeepTheReasonFieldsTogether(t *testing.T) {
	c := newCheck("dns.resolve", StageResolver, "Resolve example.com")
	if c.Status != StatusOK || c.ID != "dns.resolve" || c.Stage != StageResolver {
		t.Fatalf("newCheck produced %+v", c)
	}
	c.addEvidence("first", "", "  ", "second")
	if len(c.Evidence) != 2 {
		t.Errorf("addEvidence kept %d lines, want the two non-empty ones: %v", len(c.Evidence), c.Evidence)
	}
	c.conclude(StatusFail, "the resolver did not answer", "try the configured server directly", "dig @1.1.1.1 example.com")
	if !c.Status.Fault() || c.Cause == "" || c.Next == "" || c.Manual == "" {
		t.Errorf("conclude left the check incomplete: %+v", c)
	}
}
