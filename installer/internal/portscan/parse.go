package portscan

import (
	"strconv"
	"strings"
)

// The parsers every reader shares. Nothing here touches a filesystem, a
// command or a platform path: each function turns text a reader already has
// into the typed facts the screen shows.
//
// They live in an untagged file because both readers use them. The /proc reader
// and the macOS lsof/netstat reader each run on their own platform, but the
// parsers are the same Go everywhere, and keeping them behind a GOOS suffix is
// what made the macOS reader fail to build on Linux and the /proc reader fail
// to build on macOS: a _linux.go or _darwin.go file is invisible to every other
// platform's build.

// socketState names the state a socket really has: a TCP socket that reached
// this point was listening, and every UDP socket is unconnected by nature.
func socketState(proto Protocol) State {
	if proto.Stream() {
		return StateListen
	}
	return StateUnconn
}

// splitEndpoint splits "127.0.0.1:8080", "[::1]:8080" or "::1:8080" into a
// numeric host and a port. Brackets are the unambiguous form and are used when
// present; a bare IPv6 endpoint is split at its last colon.
func splitEndpoint(endpoint string) (string, int, bool) {
	if endpoint == "" {
		return "", 0, false
	}

	host, portText := "", ""
	bracketed := false
	if strings.HasPrefix(endpoint, "[") {
		closing := strings.Index(endpoint, "]")
		if closing < 0 {
			return "", 0, false
		}
		bracketed = true
		host = endpoint[1:closing]
		rest := endpoint[closing+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", 0, false
		}
		portText = rest[1:]
	} else {
		colon := strings.LastIndex(endpoint, ":")
		if colon < 0 {
			return "", 0, false
		}
		host, portText = endpoint[:colon], endpoint[colon+1:]
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	host = strings.Trim(host, "[]")
	if host == "*" {
		// A bracketed wildcard is an IPv6 socket: lsof prints [*] when the
		// binding is on ::, and 0.0.0.0 would name the wrong family.
		if bracketed {
			host = "::"
		} else {
			host = "0.0.0.0"
		}
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port, true
}

// normalizeHostForFamily makes an anonymous wildcard agree with its family: a
// tool that prints "*" for both IPv4 and IPv6 gets "::" for an IPv6 socket and
// "0.0.0.0" for an IPv4 one, so the two never look like the same binding.
func normalizeHostForFamily(host string, v6 bool) string {
	switch {
	case v6 && host == "0.0.0.0":
		return "::"
	case !v6 && host == "::":
		return "0.0.0.0"
	default:
		return host
	}
}

// firstNonEmptyLine returns the first line of a tool's stderr that carries
// text, trimmed. It is what a failure reports instead of a blank message, and
// it is shared by the readers and by the real probe's command error.
func firstNonEmptyLine(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
