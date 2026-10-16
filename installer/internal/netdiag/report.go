package netdiag

import (
	"fmt"
	"strings"
	"time"
)

// Status is what one check found. The five states exist because a diagnosis has
// more than two answers: a stage can be healthy, it can be a note rather than a
// fault (a proxy is configured, a VPN interface is up), it can have failed, it
// can be undetermined here, or it can have been skipped because what it needs
// was not available -- and "undetermined" and "skipped" must never be printed as
// a failure, which is the whole reason they are separate from StatusFail.
type Status int

const (
	// StatusOK: the check ran and the stage answered.
	StatusOK Status = iota
	// StatusNote: the check ran, something is present or unusual, and it is not
	// by itself a fault: a proxy is configured, a VPN interface is up, a direct
	// query to a configured resolver was refused while the stub resolver answered.
	StatusNote
	// StatusFail: the check ran and the stage did not answer.
	StatusFail
	// StatusUnknown: the check could not determine the answer on this platform
	// or with the capabilities available, and the reason is named.
	StatusUnknown
	// StatusSkipped: the check was not applicable, or what it needed was missing;
	// the reason is named.
	StatusSkipped
)

// String is the report's own word for a status, and it is the word the screen
// prints: "ok", "noted", "failed", "unknown", "skipped". "noted" is deliberate --
// a configured proxy is not a warning, and a report that colours it like one
// teaches the reader to fear their own configuration.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusNote:
		return "noted"
	case StatusFail:
		return "failed"
	case StatusUnknown:
		return "unknown"
	case StatusSkipped:
		return "skipped"
	}
	return "unknown"
}

// Fault reports whether the status is a failure: the one state that means the
// stage did not answer.
func (s Status) Fault() bool { return s == StatusFail }

// Stage is the layer of the stack a check answers for. Naming the layer is what
// turns a list of observations into a diagnosis: the reader can see that DNS
// answered and the transport did not.
type Stage string

const (
	// StageLocal: the interfaces, the addresses, the default route.
	StageLocal Stage = "local"
	// StageResolver: the resolver configuration and name resolution.
	StageResolver Stage = "dns"
	// StageTransport: the TCP connection and the handshake time.
	StageTransport Stage = "transport"
	// StageApplication: the HTTP conversation, including redirects and the
	// application's own status code.
	StageApplication Stage = "application"
	// StageCertificate: the TLS handshake and the certificate's verification.
	StageCertificate Stage = "certificate"
	// StageProxy: the observable proxy and VPN parameters, which can explain a
	// difference without being a fault.
	StageProxy Stage = "proxy"
)

// Title is the stage's name in a sentence: "the local network", "DNS".
func (s Stage) Title() string {
	switch s {
	case StageLocal:
		return "the local network"
	case StageResolver:
		return "DNS"
	case StageTransport:
		return "the transport"
	case StageApplication:
		return "the application"
	case StageCertificate:
		return "the certificate"
	case StageProxy:
		return "the proxy"
	}
	return string(s)
}

// stageOrder is the order the report prints the stages in: the order a request
// travels through them, so the first failure in the list is the earliest stage
// that stopped it.
var stageOrder = []Stage{StageLocal, StageResolver, StageProxy, StageTransport, StageCertificate, StageApplication}

// Check is one step of a diagnosis: what was asked, what answered, how long it
// took and what it means.
//
// Evidence carries the measured values, one short line each, already redacted by
// the check that measured them. Cause, Next and Manual are what the reader does
// with a check that did not answer cleanly: the plausible reasons, the next
// check to run, and a command to run by hand. A check that answered cleanly
// carries none of the three, so their presence on screen is itself a signal.
type Check struct {
	// ID is the stable identifier of the check ("dns.resolve", "tcp.connect").
	// Numbers in a report come from the order the run emitted, never from this.
	ID string
	// Stage is the layer this check answers for.
	Stage Stage
	// Title is the short human title of the check, in the screen's own words.
	Title string
	// Status is what the check found.
	Status Status
	// Evidence is what was measured, one short line each.
	Evidence []string
	// Cause is the most plausible explanation for a check that did not answer.
	Cause string
	// Next is the next check to run when this one failed.
	Next string
	// Manual is a command the user can run by hand, or a manual step.
	Manual string
	// Duration is how long the check took. It is zero for a check that ran
	// nothing (a skipped one) and never invented for one that timed out.
	Duration time.Duration
}

// newCheck starts a check. It is a constructor rather than a literal so the
// stage and the identifier cannot be transposed in a check that builds its
// result in several steps.
func newCheck(id string, stage Stage, title string) Check {
	return Check{ID: id, Stage: stage, Title: title, Status: StatusOK}
}

// addEvidence appends the non-empty lines of a check's evidence.
func (c *Check) addEvidence(lines ...string) {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			c.Evidence = append(c.Evidence, line)
		}
	}
}

// conclude sets the status and the three lines a check that did not answer
// cleanly carries. Empty arguments are allowed and stay empty: a check that
// knows why it failed but not what to run next says so by naming one and not the
// other, rather than by inventing either.
func (c *Check) conclude(status Status, cause, next, manual string) {
	c.Status = status
	c.Cause = cause
	c.Next = next
	c.Manual = manual
}

// Facts is what one check learned that a later check can use, so a diagnosis
// does not ask the same question twice and then have to explain two answers.
type Facts struct {
	// Addresses is what the name resolved to, in the resolver's own order.
	Addresses []string
	// Server is the resolver that answered, when it could be known.
	Server string
	// DNSDuration is how long the resolution took.
	DNSDuration time.Duration
}

// Summary is the count of each status in a report.
type Summary struct {
	OK      int
	Noted   int
	Failed  int
	Unknown int
	Skipped int
}

// Total is how many checks the report holds.
func (s Summary) Total() int {
	return s.OK + s.Noted + s.Failed + s.Unknown + s.Skipped
}

// Report is one diagnostic run: the target it was asked about, the checks in
// the order they ran, and how it all ended.
type Report struct {
	// Target is the target exactly as the user typed it, or empty for the
	// overview of this machine's own connectivity.
	Target string
	// Endpoint describes what was diagnosed, already redacted: never a query
	// string, never credentials.
	Endpoint string
	// Source says where the destination came from ("you entered it", "the host
	// this installer already contacts for updates").
	Source string
	// Started is when the run began and Elapsed how long it took.
	Started time.Time
	Elapsed time.Duration
	// Checks is every check the run ran or skipped, in emission order.
	Checks []Check
	// Stopped is true when the run was cancelled before it finished, so a report
	// that stops halfway says so instead of reading like a complete one.
	Stopped bool
}

// Summary counts the report's checks by status.
func (r Report) Summary() Summary {
	var s Summary
	for _, c := range r.Checks {
		switch c.Status {
		case StatusOK:
			s.OK++
		case StatusNote:
			s.Noted++
		case StatusFail:
			s.Failed++
		case StatusUnknown:
			s.Unknown++
		case StatusSkipped:
			s.Skipped++
		}
	}
	return s
}

// Check returns the check with an identifier, for a caller that needs one step.
func (r Report) Check(id string) (Check, bool) {
	for _, c := range r.Checks {
		if c.ID == id {
			return c, true
		}
	}
	return Check{}, false
}

// StageFault names the earliest stage that failed, in the order a request
// travels through the stages. It is the answer to "where does it break", and it
// deliberately ignores unknown, skipped and noted checks: only a failure that
// was observed locates a fault.
func (r Report) StageFault() (Stage, bool) {
	for _, stage := range stageOrder {
		for _, c := range r.Checks {
			if c.Stage == stage && c.Status.Fault() {
				return stage, true
			}
		}
	}
	return "", false
}

// FaultCheck returns the first failing check in stage order: the concrete step
// the verdict names.
func (r Report) FaultCheck() (Check, bool) {
	for _, stage := range stageOrder {
		for _, c := range r.Checks {
			if c.Stage == stage && c.Status.Fault() {
				return c, true
			}
		}
	}
	return Check{}, false
}

// Verdict is the report's one-sentence answer, in the report's own voice: the
// stage that failed and the check that observed it, that every check answered,
// or that the run could not establish an answer.
func (r Report) Verdict() string {
	if len(r.Checks) == 0 {
		if r.Stopped {
			return "the run was stopped before any check finished"
		}
		return "no check was run"
	}

	if fault, ok := r.FaultCheck(); ok {
		line := fmt.Sprintf("%s did not answer: %s", fault.Stage.Title(), fault.Title)
		if r.Stopped {
			line += " (the run was stopped, so later stages were not checked)"
		}
		return line
	}

	s := r.Summary()
	switch {
	case s.Unknown == s.Total():
		return fmt.Sprintf("no stage could be established on this platform (%d checks undetermined)", s.Unknown)
	case s.Unknown > 0 && s.OK == 0 && s.Noted == 0:
		return fmt.Sprintf("no stage answered, and %d checks could not be determined here", s.Unknown)
	case s.Unknown > 0:
		return fmt.Sprintf("every check that answered was healthy, and %d could not be determined here", s.Unknown)
	case r.Stopped:
		return "every check that ran before the run was stopped was healthy"
	default:
		return "every stage answered: this machine reached the destination end to end"
	}
}

// TextOptions control the text report.
type TextOptions struct {
	// Anonymize replaces every address, the home directory, the user name and
	// the machine name, so the report can be sent to somebody else.
	Anonymize bool
	// HostName overrides the machine name that Anonymize replaces. It is empty in
	// normal use, where the name comes from the operating system.
	HostName string
}

// Text is the report as a plain text document, for the screen's review state and
// for the copy the user saves for support. It is redacted by construction -- the
// checks never put a password, a cookie, an authorization header or a query
// string into it -- and Anonymize removes the addresses and the names as well.
func (r Report) Text(o TextOptions) string {
	redactor := newRedactor(o)

	var b strings.Builder
	b.WriteString("dotfiles network diagnostics\n")
	if r.Target != "" {
		source := r.Source
		if source == "" {
			source = "you entered it"
		}
		fmt.Fprintf(&b, "target: %s (%s)\n", redactor.line(r.Endpoint), source)
	} else {
		fmt.Fprintf(&b, "target: this machine (an overview of its own connectivity)\n")
	}
	fmt.Fprintf(&b, "started: %s\n", r.Started.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&b, "elapsed: %s\n", Duration(r.Elapsed))

	s := r.Summary()
	fmt.Fprintf(&b, "checks: %d run (%d ok, %d noted, %d failed, %d unknown, %d skipped)\n",
		s.Total(), s.OK, s.Noted, s.Failed, s.Unknown, s.Skipped)
	fmt.Fprintf(&b, "verdict: %s\n", r.Verdict())

	for i, c := range r.Checks {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d. [%s] %s — %s", i+1, c.Status, c.Stage.Title(), c.Title)
		if c.Duration > 0 {
			fmt.Fprintf(&b, " (%s)", Duration(c.Duration))
		}
		b.WriteString("\n")
		for _, line := range c.Evidence {
			fmt.Fprintf(&b, "   %s\n", redactor.line(line))
		}
		if c.Cause != "" {
			fmt.Fprintf(&b, "   cause: %s\n", redactor.line(c.Cause))
		}
		if c.Next != "" {
			fmt.Fprintf(&b, "   next: %s\n", redactor.line(c.Next))
		}
		if c.Manual != "" {
			fmt.Fprintf(&b, "   manual: %s\n", redactor.line(c.Manual))
		}
	}

	b.WriteString("\nNothing was changed: no resolver, route, firewall, proxy or certificate setting was written,\n")
	b.WriteString("and no verification was skipped.\n")
	return b.String()
}

// Duration renders a duration the way the reports and the screen print it: a
// measurement, never a rounded zero that would read as "instantaneous" when the
// check in fact timed out. A zero duration prints as "0 ms" and is only ever
// used for a check that ran nothing.
func Duration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0 ms"
	case d < time.Millisecond:
		return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d < 10*time.Second:
		return fmt.Sprintf("%.2f s", d.Seconds())
	default:
		return fmt.Sprintf("%.1f s", d.Seconds())
	}
}
