package portscan

import (
	"strconv"
	"strings"
)

// The refusal rules and the redaction rules. They live together because they
// are the same kind of decision -- what this package refuses to show and what it
// refuses to do -- and both are tested directly.

// redacted is what replaces a value that must not be read. It is four plain
// characters rather than a glyph, because the point is to be unmistakable on a
// terminal with no font at all.
const redacted = "****"

// secretNames are the account words that make a flag's value a secret. The
// comparison is per segment of the flag's own name, so POSTGRES_PASSWORD and
// --db-password are caught by the word password, while --passwordless is not
// caught at all.
var secretNames = map[string]bool{
	"password":      true,
	"passwd":        true,
	"passphrase":    true,
	"pwd":           true,
	"token":         true,
	"secret":        true,
	"apikey":        true,
	"accesskey":     true,
	"secretkey":     true,
	"privatekey":    true,
	"credential":    true,
	"credentials":   true,
	"auth":          true,
	"authorization": true,
}

// RedactArgs returns a copy of a process's arguments with every recognisable
// secret replaced. The input is never modified, and a flag whose value is not
// recognisable as a secret is passed through unchanged: guessing at values
// would mangle the arguments a diagnostic is supposed to reproduce exactly.
func RedactArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	out := make([]string, len(args))
	maskNext := false
	for i, arg := range args {
		if maskNext {
			out[i] = redacted
			maskNext = false
			continue
		}

		switch {
		case isSecretAssignment(arg):
			key, _, _ := strings.Cut(arg, "=")
			out[i] = key + "=" + redacted
		case isSecretFlag(arg):
			out[i] = arg
			maskNext = i+1 < len(args)
		case isRedactableValue(arg):
			out[i] = redacted
		default:
			out[i] = redactURLUserinfo(arg)
		}
	}
	return out
}

// isSecretAssignment reports whether an argument is a name=value pair whose
// name is a secret, which covers --password=x, -p=x and PASSWORD=x.
func isSecretAssignment(arg string) bool {
	name, _, found := strings.Cut(arg, "=")
	if !found || name == "" {
		return false
	}
	return looksLikeSecretName(name)
}

// isSecretFlag reports whether an argument is a flag whose separate value is
// the secret, so the mask lands on the argument that follows rather than on the
// flag's own name.
func isSecretFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	return looksLikeSecretName(arg)
}

// isRedactableValue reports whether a whole argument is a credential that names
// itself: a bearer token, a Basic header, or an HTTP header whose name is a
// secret.
func isRedactableValue(arg string) bool {
	lower := strings.ToLower(arg)
	if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") {
		return true
	}
	if name, _, found := strings.Cut(arg, ":"); found && looksLikeSecretName(name) {
		return true
	}
	return false
}

// looksLikeSecretName reports whether a flag or key name is one of the account
// words. The name is split on its separators first, so env(1) style
// POSTGRES_PASSWORD and --db-password both resolve to the segment password,
// and a whole name with no separator is checked for a secret suffix so that
// --mytoken and --sessionsecret are caught without matching --authenticator.
func looksLikeSecretName(name string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(strings.TrimLeft(name, "-")))
	if cleaned == "" {
		return false
	}

	segments := strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	if len(segments) > 1 {
		for i, segment := range segments {
			if secretNames[segment] {
				return true
			}
			// A two-word name is one account word when the words are joined:
			// api-key, access-key and secret-key are apikey, accesskey and
			// secretkey spelled apart.
			if i+1 < len(segments) && secretNames[segment+segments[i+1]] {
				return true
			}
		}
		return false
	}

	if secretNames[cleaned] {
		return true
	}
	for word := range secretNames {
		if strings.HasSuffix(cleaned, word) {
			return true
		}
	}
	return false
}

// redactURLUserinfo masks the password half of user:password@host inside a URL
// argument, which is how a database or registry URL carries a credential where
// no flag names it.
func redactURLUserinfo(arg string) string {
	schemeAt := strings.Index(arg, "://")
	if schemeAt < 0 {
		return arg
	}
	authority := arg[schemeAt+3:]
	at := strings.Index(authority, "@")
	if at < 0 {
		return arg
	}
	userinfo := authority[:at]
	colon := strings.Index(userinfo, ":")
	if colon < 0 {
		return arg
	}
	return arg[:schemeAt+3] + userinfo[:colon] + ":" + redacted + authority[at:]
}

// protectedProcessNames are the processes this utility refuses to stop whatever
// the user asks: the OS's own init and session processes, whose loss is the
// machine or the session. The list is deliberately short and names only
// processes nobody stops on purpose; everything else is decided by the user,
// and a refusal that was not a real danger would train them to look for a way
// around it.
var protectedProcessNames = map[string]string{
	"systemd":          "the system's init process",
	"init":             "the system's init process",
	"launchd":          "the system's init process",
	"kernel_task":      "the kernel's own task",
	"windowserver":     "the process that draws every window on this Mac",
	"loginwindow":      "the process that owns the login session",
	"systemd-journald": "the system's log daemon",
	"systemd-udevd":    "the system's device manager",
	"systemd-resolved": "the system's name resolver",
	"systemd-networkd": "the system's network manager",
	"dbus-daemon":      "the message bus every desktop program uses",
	"smss":             "a Windows session process",
	"csrss":            "a Windows session process",
	"wininit":          "a Windows session process",
	"winlogon":         "a Windows session process",
	"services":         "the Windows service manager",
}

// ProtectedReason reports why a process must not be stopped from here, in the
// screen's own words, or the empty string when it may be. The ancestor walk is
// bounded so a corrupted parent chain cannot loop, and it is the reason a
// process holding a terminal stays safe: stopping it would end the session the
// installer is running in.
func ProtectedReason(p Probe, proc Process) string {
	if p == nil {
		p = DefaultProbe()
	}

	if proc.PID <= 1 {
		return "pid " + strconv.Itoa(proc.PID) + " is the system's own init process; stopping it stops the machine."
	}

	self := p.SelfPID()
	if proc.PID == self {
		return "this is the installer's own process (pid " + strconv.Itoa(self) + "); stopping it would end this run."
	}

	const maxHops = 64
	pid, ok := p.Parent(self)
	for hops := 0; ok && pid > 1 && hops < maxHops; hops++ {
		if proc.PID == pid {
			return "pid " + strconv.Itoa(proc.PID) + " is an ancestor of the installer (the shell or terminal " +
				"that started it); stopping it would end this session."
		}
		pid, ok = p.Parent(pid)
	}

	if reason, protected := protectedProcessNames[normalizeProcessName(proc.Name)]; protected {
		return "this process (" + proc.Name + ") is " + reason + ", which this utility refuses to stop."
	}
	return ""
}

// normalizeProcessName lowercases a process name and drops the executable
// suffix, so Kernel_Task, kernel_task and kernel_task.exe are one entry.
func normalizeProcessName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".exe")
}

// StopTarget is one process the user has chosen, together with the sockets the
// screen showed as affected. The sockets are part of the target on purpose: a
// stop that cannot confirm the process still holds them is refused.
type StopTarget struct {
	Process Process
	Sockets []Listener
}

// StopRefusal reports why a stop must not be attempted at all, or the empty
// string when it may. It is the check the confirmation screen shows before the
// user commits, and Stop runs it again so the screen and the signal can never
// disagree.
func StopRefusal(p Probe, target StopTarget) string {
	if p == nil {
		p = DefaultProbe()
	}

	proc := target.Process
	if proc.Scope != "" {
		local := localScope(p)
		if proc.Scope != local {
			return "this process belongs to " + proc.Scope + ", and this run (" + local + ") cannot signal a process " +
				"outside its own scope. Nothing was stopped."
		}
	}
	if proc.PID <= 0 {
		return "no process id was reported for this socket, so there is nothing to stop here."
	}
	return ProtectedReason(p, proc)
}
