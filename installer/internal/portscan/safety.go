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
	"pass":          true,
	"passphrase":    true,
	"pwd":           true,
	"pw":            true,
	"sessionid":     true,
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

// shortSecretNames are the words that are a secret where they stand alone or
// end a name, and nowhere else: pass, pw and pwd are also the tail of ordinary
// words (bypass, compass, overpass) and of names that describe a step rather
// than a value (--pass-through), so a rule that matched them anywhere would
// mask a value that is not a secret at all.
var shortSecretNames = map[string]bool{
	"pass": true,
	"pw":   true,
	"pwd":  true,
}

// negationSegments are the words that turn the segment after them into the
// absence of a thing rather than the thing: --no-auth takes no credential, so
// its value is not one, and the argument after it is not masked.
var negationSegments = map[string]bool{
	"no": true, "not": true, "non": true, "without": true, "disable": true, "disabled": true,
	"skip": true, "ignore": true, "false": true, "off": true,
}

// containerSegments are the words that name where a secret lives rather than
// the secret itself: --token-file takes a path, and a path is not a token.
var containerSegments = map[string]bool{
	"file": true, "path": true, "dir": true, "directory": true, "store": true, "source": true,
	"env": true, "var": true, "name": true, "id": true, "prefix": true, "suffix": true,
	"length": true, "count": true, "ttl": true, "type": true, "scheme": true, "uri": true, "url": true,
}

// RedactArgs returns a copy of a process's arguments with every recognisable
// secret replaced. The input is never modified. The rules are deliberately
// narrow in what they guess at and deliberately broad in where they look,
// because a secret that survives is shown on the screen and copied into the
// diagnostic:
//
//   - a flag whose own name is a secret (--password, --api-key,
//     POSTGRES_PASSWORD, --pass) masks its separate value and its =value;
//   - a short password flag (-p is mysql's, -a is redis-cli's) masks the value
//     apart from it unless that value can be the port the same letter names in
//     another tool, and a short flag with its value joined to it (-ps3cret)
//     masks the value unless it reads as a port or as a flag name that carries
//     no secret;
//   - a name:value or name=value pair inside a larger argument is masked
//     wherever it appears, which is what catches a JSON body on a command
//     line;
//   - a user:password pair is masked, after a credential flag (-u, --user,
//     --login, --proxy-user) and inside a URL or a bare userinfo;
//   - Bearer and Basic credentials are masked whole.
//
// What is left alone is what carries no credential: a bare account name, a
// host:port pair, a port mapping, a path, the absence of a thing (--no-auth),
// the place a secret lives (--token-file), a flag that merely begins with a
// secret letter. Over-masking costs a diagnostic the fidelity it is read for,
// so the rules stop at the values that are credentials by their own shape or by
// their flag's name.
func RedactArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	out := make([]string, len(args))
	maskNext := false
	for i, arg := range args {
		if maskNext {
			// A flag whose own name is a secret was given this argument as its
			// value, so it is masked whatever it looks like: a password that
			// begins with a dash is still a password.
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
		case isShortSecretFlag(arg):
			// -p is mysql's password and -a is redis-cli's, but the same letters
			// name ports and flags elsewhere (ssh -p 2222, grep -a -n), so the
			// value decides: what can be a port or a path is left alone.
			out[i] = arg
			maskNext = i+1 < len(args) && shortFlagValueIsSecret(args[i+1])
		case isCredentialFlag(arg):
			out[i] = arg
			maskNext = i+1 < len(args) && looksLikeCredentialPair(args[i+1])
		case isCredentialAssignment(arg):
			key, value, _ := strings.Cut(arg, "=")
			if looksLikeCredentialPair(value) {
				out[i] = key + "=" + redacted
			} else {
				out[i] = key + "=" + value
			}
		case isJoinedShortSecret(arg):
			out[i] = arg[:2] + redacted
		default:
			out[i] = redactValue(arg)
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
// flag's own name. A single-dash name that only ends with a secret word is not
// one of these: -psecret is a short password flag with its value joined to it,
// and reading it as a flag would mask the database name after it and show the
// password.
func isSecretFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	if !looksLikeSecretName(arg) {
		return false
	}
	name := strings.TrimLeft(arg, "-")
	if !strings.HasPrefix(arg, "--") && len(name) > 1 && !secretNames[name] &&
		strings.ContainsRune(shortSecretLetters, rune(name[0])) {
		return false
	}
	return true
}

// credentialFlags are the flags whose value is a user:password pair, which is
// how curl and the database clients take a login. The pair is masked and not
// the whole flag, and not a bare account name either: the account is half the
// credential, but on its own it is not the secret.
var credentialFlags = map[string]bool{
	"u":          true,
	"U":          true,
	"user":       true,
	"username":   true,
	"user-name":  true,
	"login":      true,
	"login-name": true,
	"proxy-user": true,
	"proxyuser":  true,
}

// isCredentialFlag reports whether a flag is one that takes a login, in either
// its short or its long spelling.
func isCredentialFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.Contains(arg, "=") {
		return false
	}
	if credentialFlags[arg] {
		return true
	}
	return credentialFlags[strings.TrimLeft(arg, "-")]
}

// isCredentialAssignment reports whether an argument is the =value form of a
// credential flag, which is --user=alice:s3cret.
func isCredentialAssignment(arg string) bool {
	name, _, found := strings.Cut(arg, "=")
	if !found {
		return false
	}
	return isCredentialFlag(name)
}

// shortSecretLetters are the single letters that take a credential in the tools
// this installer describes: -p is mysql's password, -a is redis-cli's. Only
// those are listed, because every other letter is a port, a boolean or a
// pattern far more often than it is a password.
const shortSecretLetters = "pa"

// isShortSecretFlag reports whether an argument is a short secret flag standing
// on its own (-p), whose separate value is the secret: mysql -p s3cret.
func isShortSecretFlag(arg string) bool {
	return len(arg) == 2 && arg[0] == '-' && strings.ContainsRune(shortSecretLetters, rune(arg[1]))
}

// shortFlagValueIsSecret reports whether the argument after a standalone short
// password flag is the secret rather than the port or the path the same letter
// names in another tool. What can be a port (2222, 8080:80, /dev/tty) is not a
// password; a value that is a flag of its own is not one either, unless it
// carries what a flag name would not (-s3cret is a password, -t is sshpass's
// own option).
func shortFlagValueIsSecret(value string) bool {
	if looksLikeAPort(value) {
		return false
	}
	if strings.HasPrefix(value, "-") {
		return carriesNonFlagCharacter(value)
	}
	return true
}

// isJoinedShortSecret reports whether a short flag carries its value joined to
// it and that value is a secret. Three things keep the rule from eating a flag's
// own name: a joined value that can be a port (-p8080:80, -p2222) is a port, a
// bare word that names no secret (-print, -path, -pfoo) is a flag name, and
// everything else is the password it looks like (-psecret, -p123456, -ps3cret).
func isJoinedShortSecret(arg string) bool {
	if len(arg) < 3 || arg[0] != '-' || arg[1] == '-' {
		return false
	}
	if !strings.ContainsRune(shortSecretLetters, rune(arg[1])) {
		return false
	}
	tail := arg[2:]
	if looksLikeAPort(tail) {
		return false
	}
	if isBareFlagWord(tail) && !looksLikeSecretName(tail) {
		return false
	}
	return true
}

// looksLikeAPort reports whether a value can be the port a short flag takes: a
// number in the port range, an address or a mapping, or a path. It is what keeps
// ssh -p 2222, docker -p 8080:80 and ls -a /tmp readable while a password --
// which is none of those -- is masked.
func looksLikeAPort(value string) bool {
	if value == "" {
		return false
	}
	if port, err := strconv.Atoi(value); err == nil {
		return port > 0 && port <= 65535
	}
	if isPath(value) {
		return true
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			continue
		}
		switch r {
		case '.', ':', ',', '/', '-', '[', ']', '*':
			continue
		}
		return false
	}
	return true
}

// isPath reports whether a value names a place in the file system rather than a
// credential: it begins with a slash, a dot-slash, a parent or a home. A path is
// what ls -a /tmp carries after a boolean flag that happens to share its letter
// with a password flag, and a secret is not spelled like one.
func isPath(value string) bool {
	return strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") || strings.HasPrefix(value, "~/")
}

// isBareFlagWord reports whether a value is a word a flag name could be: lower
// case letters only, with no digit and no punctuation.
func isBareFlagWord(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// carriesNonFlagCharacter reports whether a value carries a character a flag
// name would not: a digit, an upper-case letter or one of the punctuation
// characters a password uses.
func carriesNonFlagCharacter(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			return true
		}
		if strings.ContainsRune("!$%&*#@+?=~^", r) {
			return true
		}
	}
	return false
}

// looksLikeCredentialPair reports whether a value is the user:password form
// rather than a host:port, a path or a URL. A URL carries its credential after
// a scheme and is masked by redactURLUserinfo instead, and a host:port pair has
// no second part to keep secret. The separators are the two the login flags of
// the tools this utility runs under use: a colon, and the per cent that the SMB
// clients separate the account from the password with.
func looksLikeCredentialPair(value string) bool {
	if strings.Contains(value, "://") {
		return false
	}
	colon := strings.IndexAny(value, ":%")
	return colon > 0 && colon < len(value)-1
}

// redactValue masks the secrets inside one argument that is not itself a flag:
// a self-naming credential whole, and every embedded pair whose name is a
// secret.
func redactValue(arg string) string {
	if isRedactableValue(arg) {
		return redacted
	}
	return redactEmbeddedSecrets(redactBareUserinfo(redactURLUserinfo(arg)))
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

// redactEmbeddedSecrets masks the value half of every name:value or name=value
// pair inside one argument whose name is a secret. It is what catches a
// credential travelling inside a larger argument -- a JSON body on a command
// line ({"password":"hunter2"}), a header list, a query string -- where no flag
// names it and no whole-argument rule can see it.
func redactEmbeddedSecrets(arg string) string {
	var out strings.Builder
	i := 0
	for i < len(arg) {
		rel := strings.IndexAny(arg[i:], ":=")
		if rel < 0 {
			out.WriteString(arg[i:])
			break
		}
		at := i + rel
		out.WriteString(arg[i : at+1])
		if !looksLikeSecretName(nameBeforeSeparator(arg[:at])) {
			i = at + 1
			continue
		}

		valueStart := at + 1
		quote := byte(0)
		if valueStart < len(arg) && (arg[valueStart] == '"' || arg[valueStart] == '\'') {
			quote = arg[valueStart]
			out.WriteByte(quote)
			valueStart++
		}
		end := valueStart
		escaped := false
		for end < len(arg) {
			c := arg[end]
			if escaped {
				// The character after a backslash is part of the value, quote or
				// not: a JSON body escapes its quotes, and stopping at the escaped
				// one would leave the rest of the secret in the clear.
				escaped = false
				end++
				continue
			}
			if c == '\\' {
				escaped = true
				end++
				continue
			}
			if quote != 0 {
				if c == quote {
					break
				}
			} else if c == ' ' || c == ',' || c == '}' || c == ']' || c == ';' {
				break
			}
			end++
		}
		if end > valueStart {
			out.WriteString(redacted)
		}
		i = end
	}
	return out.String()
}

// nameBeforeSeparator is the key a separator belongs to: the run of characters
// since the last delimiter, with any quoting removed. A prefix that carries no
// key -- a URL scheme, a clock time of the form 12:30 -- yields something that
// is not a secret name, which is what keeps the scan from masking a value that
// names no secret.
func nameBeforeSeparator(prefix string) string {
	start := strings.LastIndexAny(prefix, " ,;{[(|")
	return strings.Trim(strings.TrimSpace(prefix[start+1:]), "\"'")
}

// looksLikeSecretName reports whether a flag or key name is one of the account
// words. The name is split on its separators first, so env(1) style
// POSTGRES_PASSWORD and --db-password both resolve to the segment password,
// and a whole name with no separator is checked for a secret suffix so that
// --mytoken and --sessionsecret are caught without matching --authenticator.
// Three rules keep a name that carries no secret readable: a word that is a
// secret only where it stands alone (pass, pw, pwd) counts only as the last
// segment, so DB_PASS is one and bypass and --pass-through are not; a segment
// after a negation is not a secret, because --no-auth takes none; and a secret
// word followed by the name of a place is not one either, because --token-file
// takes a path.
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
			if i > 0 && negationSegments[segments[i-1]] {
				continue
			}
			if !secretNames[segment] {
				// A two-word name is one account word when the words are joined:
				// api-key, access-key and secret-key are apikey, accesskey and
				// secretkey spelled apart.
				if i+1 < len(segments) && secretNames[segment+segments[i+1]] &&
					!containerSegments[segments[i+1]] {
					return true
				}
				continue
			}
			// The short words are a secret only at the end of a name: DB_PASS is
			// one, while --pass-through names a step rather than a credential.
			if shortSecretNames[segment] && i != len(segments)-1 {
				continue
			}
			// The place a secret lives is not the secret: --token-file and
			// --secret-path take a path, and a path is not a credential.
			if i+1 < len(segments) && containerSegments[segments[i+1]] {
				continue
			}
			return true
		}
		return false
	}

	if secretNames[cleaned] {
		return true
	}
	for word := range secretNames {
		// The short words are a secret on their own or at the end of a name and
		// nowhere else: bypass, compass and overpass end in pass and name no
		// credential.
		if shortSecretNames[word] {
			continue
		}
		if strings.HasSuffix(cleaned, word) {
			return true
		}
	}
	return false
}

// redactURLUserinfo masks the password half of user:password@host inside a URL
// argument, which is how a database or registry URL carries a credential where
// no flag names it. A userinfo with no colon is an account name where it is one
// (https://alice@host keeps its account) and the token itself where it carries
// what a name does not (https://ghp_Abc123@host), which is the shape a personal
// access token is handed to git in.
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
		if carriesNonFlagCharacter(userinfo) {
			return arg[:schemeAt+3] + redacted + authority[at:]
		}
		return arg
	}
	return arg[:schemeAt+3] + userinfo[:colon] + ":" + redacted + authority[at:]
}

// redactBareUserinfo masks the password half of a user:password@host that
// carries no scheme, which is how a proxy with a login is handed to curl on a
// command line. It stops at the first separator that is not inside the userinfo
// so that a URL that already carries a scheme is left to redactURLUserinfo.
func redactBareUserinfo(arg string) string {
	if strings.Contains(arg, "://") {
		return arg
	}
	at := strings.Index(arg, "@")
	if at <= 0 {
		return arg
	}
	userinfo := arg[:at]
	colon := strings.Index(userinfo, ":")
	if colon <= 0 || colon == len(userinfo)-1 {
		return arg
	}
	return userinfo[:colon] + ":" + redacted + arg[at:]
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
	if proc.PID <= 0 {
		return "no process id was reported for this socket, so there is nothing to stop here."
	}

	// An unknown scope fails closed. Discover fills every owner's scope from the
	// kernel the reading came from, so this is the hand-built target: a signal
	// sent across a host/guest boundary because nobody said which side the
	// process is on is exactly the mistake the scope rule exists to prevent.
	local := localScope(p)
	if proc.Scope == "" {
		return "the scope this process belongs to could not be established, so this run cannot tell whether a process of " +
			local + " or of the other side of a host/guest boundary is meant. Nothing was stopped; refresh so the reading " +
			"names the scope."
	}
	if proc.Scope != local {
		return "this process belongs to " + proc.Scope + ", and this run (" + local + ") cannot signal a process " +
			"outside its own scope. Nothing was stopped."
	}
	return ProtectedReason(p, proc)
}
