package netdiag

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Redaction is the difference between a report that can be sent to somebody else
// and one that leaks a password. It happens where the text is built, not where a
// check decides to be careful: every line of every report passes through a
// redactor, so a check that forgets is masked rather than trusted.
//
// Two things are always removed, whether or not the user asked for
// anonymization:
//
//   - the credentials of a URL, and specifically of a proxy URL, which is where a
//     password is most likely to appear: "http://alice:s3cr3t@proxy:3128" is
//     printed as "http://***:***@proxy:3128";
//   - nothing else. The query string of a request is dropped when the URL is
//     rendered (SafeURL), so a token in a query is never part of a report at all.
//
// Anonymization is the extra step the user chooses: every address becomes
// <ipv4> or <ipv6>, the home directory becomes ~, the user name becomes <user>
// and the machine name becomes <host>. It is done on the rendered line, token by
// token, so a timestamp like 12:04:05 is not mistaken for an IPv6 address the
// way a blind pattern would.

// credentialsInURL matches the credentials of a URL, so that even a line a check
// built carelessly cannot carry a proxy password out of the process.
var credentialsInURL = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^/@\s:]*:[^/@\s]*@`)

// userInURL matches a URL with a user name and no password.
var userInURL = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^/@\s:]*@`)

// RedactCredentials masks the user name and the password of every URL in a line
// of text. It is the unconditional half of redaction.
func RedactCredentials(text string) string {
	text = credentialsInURL.ReplaceAllString(text, "${1}***:***@")
	return userInURL.ReplaceAllString(text, "${1}***@")
}

// RedactProxy renders a proxy URL as the report may show it: the scheme, the
// host and the port, with the credentials replaced by ***. A value that does not
// parse is passed through RedactCredentials instead, which leaves no password
// behind either.
func RedactProxy(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		// A proxy named without its scheme is still a proxy. A value with a space
		// or a slash is not one, and is passed through the credential redaction
		// alone rather than being dressed up as a URL.
		if strings.ContainsAny(raw, " \t/") {
			return RedactCredentials(raw)
		}
		return RedactCredentials("http://" + raw)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return RedactCredentials(raw)
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	if u.User != nil {
		return fmt.Sprintf("%s://***:***@%s", scheme, u.Host)
	}
	return fmt.Sprintf("%s://%s", scheme, u.Host)
}

// SafeURL is what a report may show of a URL: the scheme, the host, the port and
// the path. The credentials are not there by construction, and the query string
// and the fragment are dropped, because a query is where a token lives and a
// diagnosis never needs one.
func SafeURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// URLHasQuery reports whether a URL carried a query string, so a check can say
// that one was not shown instead of appearing to have received a different URL
// from the one the user typed.
func URLHasQuery(u *url.URL) bool {
	return u != nil && (u.RawQuery != "" || u.Fragment != "")
}

// redactor applies the redaction rules to every line of a report.
type redactor struct {
	enabled  bool
	literals []literalReplacement
}

// literalReplacement is one literal name replaced when anonymization is on. The
// replacements run longest first, so /home/alice/.ssh is replaced whole before
// /home/alice is, and the result never ends in a half-replaced path.
type literalReplacement struct {
	old, new string
}

// newRedactor builds the redactor a report runs its lines through. With
// anonymization off it still masks the credentials of a URL; with it on, it also
// replaces the addresses and the three names this machine is reachable under.
func newRedactor(o TextOptions) redactor {
	if !o.Anonymize {
		return redactor{}
	}

	r := redactor{enabled: true}

	// The home directory first: it is the longest literal, and it contains the
	// user name, so replacing it whole is what keeps a path readable as ~/...
	// instead of becoming <user>/.ssh.
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home != "" && home != "/" {
		r.literals = append(r.literals, literalReplacement{old: home, new: "~"})
	}

	for _, name := range []string{os.Getenv("USER"), os.Getenv("USERNAME"), os.Getenv("LOGNAME")} {
		if name != "" {
			r.literals = append(r.literals, literalReplacement{old: name, new: "<user>"})
			break
		}
	}

	host := o.HostName
	if host == "" {
		host, _ = os.Hostname()
	}
	if host != "" && !strings.EqualFold(host, "localhost") {
		r.literals = append(r.literals, literalReplacement{old: host, new: "<host>"})
	}

	// Longest first, so a name that contains another one is replaced before the
	// shorter name can rewrite part of it: the machine "alber-pc" becomes <host>
	// rather than <user>-pc.
	sort.SliceStable(r.literals, func(i, j int) bool {
		return len(r.literals[i].old) > len(r.literals[j].old)
	})

	return r
}

// line redacts one line of a report: always the credentials of a URL, then the
// names and the addresses when anonymization is on.
func (r redactor) line(text string) string {
	text = RedactCredentials(text)
	if !r.enabled {
		return text
	}
	for _, l := range r.literals {
		text = replaceWholeWord(text, l.old, l.new)
	}
	return anonymizeAddresses(text)
}

// replaceWholeWord replaces a literal when it stands as a whole word, so the
// user "alber" does not turn "albersg/dotfiles" into "<user>sg/dotfiles". A home
// directory is a path prefix rather than a word, so a slash after it counts as a
// boundary too.
func replaceWholeWord(text, old, replacement string) string {
	if old == "" {
		return text
	}
	var b strings.Builder
	for {
		i := strings.Index(text, old)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		beforeOK := i == 0 || !isWordRune(rune(text[i-1]))
		end := i + len(old)
		afterOK := end >= len(text) || !isWordRune(rune(text[end]))
		b.WriteString(text[:i])
		if beforeOK && afterOK {
			b.WriteString(replacement)
		} else {
			b.WriteString(old)
		}
		text = text[end:]
	}
}

// isWordRune reports whether a rune can be part of the same word as a name: a
// letter, a digit or the underscore. A dot, a slash, a colon and a space are all
// boundaries.
func isWordRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 0x7F
}

// anonymizeAddresses replaces every address in a line, token by token. Splitting
// the line is what makes it safe to run over a report that also contains times:
// "12:04:05" is offered to net.ParseIP as one token and rejected, while
// "fe80::1/64" and "[::1]:443" are recognised.
func anonymizeAddresses(line string) string {
	if !strings.ContainsAny(line, "0123456789:") {
		return line
	}
	var b strings.Builder
	start := 0
	for i := 0; i <= len(line); i++ {
		if i == len(line) || line[i] == ' ' || line[i] == '\t' {
			b.WriteString(anonymizeField(line[start:i]))
			if i < len(line) {
				b.WriteByte(line[i])
			}
			start = i + 1
		}
	}
	return b.String()
}

// anonymizeField replaces the address inside one whitespace-delimited field,
// keeping the punctuation and the port or prefix that surround it. A field with
// no address is returned unchanged.
func anonymizeField(field string) string {
	lead := 0
	// An opening bracket is not trimmed here: it is how a bracketed address is
	// recognised in the branch below, and a bracket removed before that check
	// would make "[2001:db8::1]:443" read as a bare address with a colon.
	for lead < len(field) && strings.IndexByte("(<\"'", field[lead]) >= 0 {
		lead++
	}
	tail := len(field)
	for tail > lead && strings.IndexByte(")]>\",;.", field[tail-1]) >= 0 {
		tail--
	}
	core := field[lead:tail]
	if core == "" {
		return field
	}

	// "[::1]:443" and "[::1]" keep their brackets and their port.
	if strings.HasPrefix(core, "[") {
		if i := strings.IndexByte(core, ']'); i > 0 {
			if replacement, ok := anonymizeAddress(core[1:i]); ok {
				return field[:lead] + "[" + replacement + "]" + core[i+1:] + field[tail:]
			}
		}
		return field
	}

	// "fe80::1%eth0" keeps its zone and "192.168.1.42/24" its prefix length.
	address, suffix := core, ""
	if i := strings.IndexAny(core, "/%"); i > 0 {
		address, suffix = core[:i], core[i:]
	}
	if replacement, ok := anonymizeAddress(address); ok {
		return field[:lead] + replacement + suffix + field[tail:]
	}

	// "192.0.2.10:443": an address with a port, where the address itself was not
	// bracketed. The plain form is tried first, so an IPv6 address whose last
	// group happens to be digits is recognised as the address it is before the
	// colon is read as a port separator.
	if i := strings.LastIndexByte(core, ':'); i > 0 && allDigits(core[i+1:]) {
		if replacement, ok := anonymizeAddress(core[:i]); ok {
			return field[:lead] + replacement + core[i:] + field[tail:]
		}
	}
	return field
}

// allDigits reports whether a string is one or more ASCII digits, and empty is
// not: a trailing colon with nothing after it is not a port.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// anonymizeAddress recognizes one address and returns the token that replaces
// it. An address this machine could reach over a private link and one it could
// not are both replaced: a report for support should not carry either.
func anonymizeAddress(address string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(address))
	if ip == nil {
		return "", false
	}
	if ip.To4() != nil {
		return "<ipv4>", true
	}
	return "<ipv6>", true
}
