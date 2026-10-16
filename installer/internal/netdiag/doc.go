// Package netdiag answers one question: when this machine cannot reach a
// service, which stage of the stack stopped it.
//
// It is a diagnostic, not a scanner. Every check reads: nothing here writes a
// resolver, a route, a firewall rule, a proxy setting or a certificate, no
// verification is ever skipped for convenience, and no tool is installed. The
// only two checks that leave the machine are the ones the user's own target
// asks for, plus the destination the caller names and the screen declares --
// this package hard-codes no host of its own.
//
// The stages are separate on purpose. A name that does not resolve, a port that
// refuses a connection, a socket that is filtered into silence, a TLS
// certificate this machine does not trust and an application answering 500 are
// five different failures with five different owners, and a report that says
// "no internet" for all of them is worse than no report. The latency printed
// here is the TCP handshake time: nothing depends on ICMP, so a network that
// drops pings is diagnosed exactly like one that answers them.
//
// Everything a check does is bounded: a timeout per stage, a limit on the
// addresses tried, a limit on redirects and on the bytes read from a response,
// and limited concurrency. Every effect sits behind Probe, so the whole package
// runs against simulated resolvers, sockets and HTTP servers in a test, with no
// network and no user configuration involved.
package netdiag
