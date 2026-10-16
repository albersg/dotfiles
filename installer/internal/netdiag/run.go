package netdiag

import (
	"context"
	"fmt"
)

// ============================================================================
// THE RUNNER
// ============================================================================
//
// Run is the one function the screen calls. It runs the checks in the order the
// report reads in -- the local network, the resolver, the proxy, the transport,
// the certificate, the application -- and hands each one to the caller as it
// finishes, so a slow stage does not hide a fast one and the interface can say
// how many steps are done instead of how long they might take.
//
// Three decisions here are worth stating.
//
// The order is fixed rather than concurrent. The report's meaning depends on it:
// the connection check dials the addresses the resolution found, and a reader
// follows the stages the way a connection does. One check runs at a time, which
// is the strongest form of the concurrency bound this utility promises: a
// diagnosis must never be what saturates a link it is diagnosing.
//
// A stage whose predecessor never connected is reported as skipped, with that
// reason, instead of failing again in its own words. Five failures with one cause
// read as five problems, and the verdict has to name the one stage that actually
// stopped it.
//
// A cancelled run still produces a report. Every check that did not run is
// present and says why, so stopping early shows what was found rather than
// throwing it away.

// Run runs one diagnosis and returns its report. It calls onCheck for every
// check as the check finishes, including the ones that were skipped, so the
// caller can show progress without waiting for the end. onCheck may be nil.
//
// The run is bounded by the options' total bound and cancelled by ctx, so a
// caller that stops the run stops the dialling with it. Neither a nil ctx nor a
// zero Bounds can produce an unbounded run: the bounds substitute their own
// defaults field by field.
func Run(ctx context.Context, p Probe, o Options, onCheck func(Check)) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	bounds := o.Bounds
	runCtx, cancel := context.WithTimeout(ctx, bounds.total())
	defer cancel()

	report := Report{Started: p.Now()}
	session := &Session{Ctx: runCtx, Options: o}
	if !o.Target.IsZero() {
		report.Target = o.Target.Describe()
	}
	if ep, ok := session.Endpoint(); ok {
		report.Endpoint = ep.Describe()
		report.Source = ep.Source
	}

	emit := func(check Check) {
		report.Checks = append(report.Checks, check)
		if onCheck != nil {
			onCheck(check)
		}
	}

	for _, step := range runPlan(p, session, bounds) {
		switch {
		case runCtx.Err() != nil:
			// The run is over: everything left is named as what it is, a check that
			// did not run, rather than left out of the report.
			report.Stopped = true
			emit(notRun(step, runStoppedReason(ctx, runCtx, bounds)))
		case step.why != nil && step.why() != "":
			emit(notRun(step, step.why()))
		default:
			emit(step.run())
		}
	}

	report.Elapsed = p.Now().Sub(report.Started)
	return report
}

// runStep is one check the runner is going to run: what it is, and how to run it.
// A step with a why is skipped when the reason is not empty, which is how a stage
// says it has nothing to do because the stage before it never connected.
type runStep struct {
	id    string
	stage Stage
	title string
	why   func() string
	run   func() Check
}

// runState is what the runner learned from the stages before the one it is about
// to run. It exists so a downstream stage can say it was skipped because of the
// stage that failed, instead of running into the same wall and reporting it a
// second time in different words.
type runState struct {
	tcpRan bool
	tcp    Status
}

// transportDown reports whether the connection stage ran and reached nothing, in
// which case the stages that dial the same address cannot do better.
func (s runState) transportDown() bool { return s.tcpRan && s.tcp == StatusFail }

// runPlan is the run in the order it happens. It is built as one list so the
// order is a single readable decision rather than an emergent property of which
// function calls which.
func runPlan(p Probe, session *Session, bounds Bounds) []runStep {
	ep, hasEndpoint := session.Endpoint()
	state := &runState{}

	transportDownReason := func(what string) func() string {
		return func() string {
			if state.transportDown() {
				return "the transport did not connect, so there was " + what
			}
			return ""
		}
	}

	return []runStep{
		{
			id: "env.interfaces", stage: StageLocal, title: "Interfaces and addresses",
			run: func() Check { return checkInterfaces(p, *session) },
		},
		{
			id: "env.route", stage: StageLocal, title: "Default route",
			run: func() Check { return checkRoute(p, *session) },
		},
		{
			id: "env.resolver", stage: StageResolver, title: "Resolver configuration",
			run: func() Check { return checkResolver(p, *session) },
		},
		{
			id: "dns.resolve", stage: StageResolver, title: titleForDNS(ep, hasEndpoint),
			run: func() Check {
				check, facts := checkDNS(p, *session)
				// The facts are what the transport dials: without them it would
				// resolve the name a second time and could dial a different address
				// from the one the report just named.
				session.Facts = facts
				return check
			},
		},
		{
			id: "env.proxy", stage: StageProxy, title: "Proxy and VPN",
			run: func() Check { return checkProxy(p, *session) },
		},
		{
			id: "tcp.connect", stage: StageTransport, title: titleForTCP(ep, hasEndpoint),
			run: func() Check {
				check := checkTCP(p, *session)
				state.tcpRan, state.tcp = true, check.Status
				return check
			},
		},
		{
			// The trace answers a different question from the connection -- not
			// whether the destination is reachable but where the path goes -- so it
			// runs even when the connection failed. It gates itself: it is skipped
			// when it was not asked for, when the destination is the overview's, and
			// when the system has no tool.
			id: "trace.route", stage: StageTransport, title: "The path to the destination",
			run: func() Check { return checkTrace(p, *session) },
		},
		{
			id: "tls.handshake", stage: StageCertificate, title: titleForTLS(ep, hasEndpoint),
			why: transportDownReason("nothing to hand a TLS session to"),
			run: func() Check { return checkTLS(p, *session) },
		},
		{
			id: "http.get", stage: StageApplication, title: titleForHTTP(ep, hasEndpoint),
			why: transportDownReason("no service to ask"),
			run: func() Check { return checkHTTP(p, *session) },
		},
		{
			id: "http.interception", stage: StageApplication, title: "Is something answering in the destination's place",
			why: transportDownReason("nothing to compare against"),
			run: func() Check { return checkInterception(p, *session) },
		},
	}
}

// notRun is a check the runner did not run, named the way the check would have
// named itself, with the reason it did not run. It is a skip and never a failure:
// a stage this utility did not ask is not a stage that failed.
func notRun(step runStep, reason string) Check {
	check := newCheck(step.id, step.stage, step.title)
	check.conclude(StatusSkipped, reason, "", "")
	return check
}

// runStoppedReason says which of the two ways a run ended early this was: the
// user stopped it, or the run's own bound ran out. They read differently to
// somebody deciding whether to run it again, so they are not one message.
func runStoppedReason(callerCtx, runCtx context.Context, bounds Bounds) string {
	if callerCtx.Err() != nil {
		return "the run was stopped before this check ran"
	}
	return fmt.Sprintf("the run's own bound of %s was reached before this check ran", Duration(bounds.total()))
}

// ============================================================================
// THE TITLES A CHECK STATES FOR ITSELF
// ============================================================================
//
// A check that the runner did not run still appears in the report, and it has to
// be named exactly the way the check names itself: two spellings of the same
// step, one for the runs that happened and one for the runs that did not, would
// make the report read as if it held different checks.

func titleForDNS(ep Endpoint, ok bool) string {
	if !ok {
		return "Resolve the destination"
	}
	return "Resolve " + ep.ServerName()
}

func titleForTCP(ep Endpoint, ok bool) string {
	if !ok {
		return "Connect to the destination"
	}
	return "Connect to " + ep.Address()
}

func titleForTLS(ep Endpoint, ok bool) string {
	if !ok {
		return "TLS handshake"
	}
	return "TLS handshake with " + ep.Address()
}

func titleForHTTP(ep Endpoint, ok bool) string {
	if !ok {
		return "HTTP request"
	}
	if target, ok := endpointURL(ep); ok {
		return "Request " + SafeURL(target)
	}
	return "HTTP request"
}
