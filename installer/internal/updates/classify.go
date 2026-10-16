package updates

import (
	"strings"
)

// This file is the one place a manager's refusal is turned into a status. Five
// of the states the update center shows are decided here rather than by the
// manager, because the managers do not agree on how to say them: apt prints
// "Could not get lock", pacman "unable to lock database", dnf "another app is
// currently holding the yum lock", and a reader who is shown "failed" for all
// three learns nothing about which of them to wait for, fix or run as root.
//
// Nothing here invents a severity. A status is a classification of what the
// manager said, and Reason always carries the manager's own words, so a claim
// that cannot be checked against the output is not made.

// failureRule is one classification: the status its markers decide, the sentence
// the reason starts with, and the markers themselves, lowercased.
type failureRule struct {
	status  Status
	lead    string
	markers []string
}

// failureRules are tried in order. The order is the point: a permission refusal
// often also mentions a lock ("the lock file could not be opened: Permission
// denied"), and reporting that as a lock would send the user to wait for a
// process that is not there, while reporting it as a permission problem sends
// them to the one thing that fixes it.
var failureRules = []failureRule{
	{
		status: StatusPermission,
		lead:   "the manager needs administrator permission",
		markers: []string{
			"a password is required",
			"a terminal is required",
			"no tty present",
			"askpass",
			"must be run as root",
			"must be run with root",
			"are you root",
			"cannot perform this operation unless you are root",
			"run as root",
			"root privileges",
			"superuser privileges",
			"requires superuser",
			"permission denied",
			"access denied",
			"insufficient privileges",
			"operation not permitted",
			"not in the sudoers",
			"incorrect password",
			"authentication is required",
			"not authorized",
			"polkit",
		},
	},
	{
		status: StatusLocked,
		lead:   "another process holds the manager's own lock",
		markers: []string{
			"could not get lock",
			"could not open lock file",
			"unable to acquire the dpkg frontend lock",
			"is another process using it",
			"unable to lock database",
			"database is locked",
			"failed to init transaction",
			"another app is currently holding the yum lock",
			"waiting for cache lock",
			"waiting for file lock",
			"lock is held",
			"another transaction is in progress",
			"frontend lock",
			"is currently holding the lock",
		},
	},
	{
		status: StatusOffline,
		lead:   "the manager could not reach its source",
		markers: []string{
			"could not resolve host",
			"temporary failure in name resolution",
			"name or service not known",
			"network is unreachable",
			"no route to host",
			"connection timed out",
			"connection refused",
			"operation timed out",
			"failed to fetch",
			"failed to download",
			"unable to download",
			"could not download",
			"could not connect",
			"cannot connect",
			"cannot query the store",
			"tls handshake timeout",
			"ssl certificate problem",
			"certificate verify failed",
			"check your internet connection",
			"502 bad gateway",
			"503 service unavailable",
			"504 gateway",
			"repository is unreachable",
		},
	},
	{
		status: StatusConflict,
		lead:   "the manager refused on dependency grounds",
		markers: []string{
			"conflicting dependencies",
			"conflicts with",
			"unresolvable dependencies",
			"unable to satisfy dependency",
			"cannot satisfy dependencies",
			"held broken packages",
			"broken dependencies",
			"has unresolvable dependencies",
			"breaks the following",
			"would break",
			"dependency resolution failed",
		},
	},
}

// Classify turns one command's output into the state it ended in, with the
// manager's own words as the reason.
//
// A command that exited zero is StatusCurrent: it succeeded, and whether anything
// actually moved is a question only a fresh read of the manager can answer, which
// is what Apply does next. A cancellation is reported as such before any marker is
// consulted, because an interrupted command's output frequently also carries a
// half-written error that is not the reason it stopped.
func Classify(output Output) (Status, string) {
	switch {
	case output.Cancelled:
		return StatusCancelled, "the run was stopped at your request; the manager may have completed part of the command"
	case output.TimedOut:
		return StatusFailed, "the command did not finish within its timeout; the manager was asked to stop"
	case output.ExitCode == 0 && output.Err == nil:
		return StatusCurrent, ""
	}

	text := strings.ToLower(output.Text())
	for _, rule := range failureRules {
		for _, marker := range rule.markers {
			if !strings.Contains(text, marker) {
				continue
			}
			return rule.status, rule.lead + ": " + evidence(output, marker)
		}
	}

	if reason := evidence(output, ""); reason != "" {
		return StatusFailed, "the manager refused: " + reason
	}
	if output.Err != nil {
		return StatusFailed, "the command could not be run: " + output.Err.Error()
	}
	return StatusFailed, "the command exited with status " + itoa(output.ExitCode)
}

// evidence returns the line of output that carries marker, or the first
// meaningful line when marker is empty. The line is redacted and bounded, because
// it is drawn on a screen and kept in a log: a private registry's URL can carry a
// token, and a manager's progress line can be a webpage long.
func evidence(output Output, marker string) string {
	for _, line := range strings.Split(output.Text(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if marker != "" && !strings.Contains(strings.ToLower(trimmed), marker) {
			continue
		}
		return boundEvidence(RedactLine(trimmed))
	}
	if marker == "" {
		return ""
	}
	// The marker was found in the combined text but not on a line of its own --
	// possible when a manager wraps a message across lines. The whole of the
	// text's first line is still better than a reason without evidence.
	for _, line := range strings.Split(output.Text(), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return boundEvidence(RedactLine(trimmed))
		}
	}
	return ""
}

// boundEvidence keeps one evidence line to a length a screen row can hold.
const evidenceLimit = 200

func boundEvidence(line string) string {
	if len(line) > evidenceLimit {
		return line[:evidenceLimit] + "…"
	}
	return line
}

// itoa is strconv.Itoa without the import, so this file reads as one unit.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
