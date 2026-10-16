package netdiag

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Internationalized host names.
//
// A host name typed on a keyboard is not the host name the resolver takes: the
// resolver needs ASCII, and the ASCII form of an internationalized label is its
// punycode form. The mapping here is the conservative subset of the standard
// (UTS-46, lookup profile) that this utility can stand behind:
//
//   - the label is normalized to NFC and lowercased;
//   - a label that is entirely ASCII is kept as it is, minus its case;
//   - a label with non-ASCII characters is punycode-encoded with the xn-- prefix,
//     and every encoded label is decoded again and compared with what was
//     encoded, so an encoder defect cannot silently produce a different domain;
//   - a label with a right-to-left character, a bidirectional control or a
//     joiner is refused with a reason rather than converted, because a wrong
//     mapping there is a spoofing hazard and this utility has no business
//     guessing;
//   - the result is validated as a host name: labels of 1 to 63 bytes, a total of
//     at most 253 bytes, letters, digits, hyphens and underscores only, and no
//     label that starts or ends with a hyphen.
//
// The punycode algorithm is RFC 3492. It is implemented here rather than pulled
// in as a dependency because this module already carries golang.org/x/text (for
// the NFC normalization) and adding golang.org/x/net/idna would rewrite the
// module graph this repository shares with the other agents working in it.

// Punycode parameters, RFC 3492 section 5.
const (
	punyBase        = 36
	punyTMin        = 1
	punyTMax        = 26
	punySkew        = 38
	punyDamp        = 700
	punyInitialBias = 72
	punyInitialN    = 128
	punyDelimiter   = '-'
	punyACE         = "xn--"
	// maxCodePoint is the highest Unicode code point, which a decoded punycode
	// label may not exceed.
	maxCodePoint = 0x10FFFF
)

// errNotConvertible is the reason every refusal in this file carries into the
// screen: a host name that cannot be converted is refused, never resolved as
// something else.
var errNotConvertible = errors.New("not convertible to an ASCII host name")

// hostToASCII returns the ASCII form of a host name and, when the conversion
// changed more than the case, the name as it was typed. The second value is what
// the report shows beside the ASCII form, so the reader can see which domain was
// actually resolved.
func hostToASCII(host string) (ascii string, typed string, err error) {
	if host == "" {
		return "", "", errNotConvertible
	}

	labels := strings.Split(host, ".")
	converted := make([]string, 0, len(labels))
	for _, label := range labels {
		if label == "" {
			// The trailing dot of a fully qualified name is kept by the join
			// below; an empty label anywhere else is refused by validateHost.
			converted = append(converted, "")
			continue
		}
		if isASCII(label) {
			converted = append(converted, strings.ToLower(label))
			continue
		}

		mapped := strings.ToLower(norm.NFC.String(label))
		if err := checkConvertibleLabel(mapped); err != nil {
			return "", "", err
		}
		encoded, err := punyEncode(mapped)
		if err != nil {
			return "", "", fmt.Errorf("%s: %w", label, errNotConvertible)
		}
		encoded = punyACE + encoded
		if len(encoded) > 63 {
			return "", "", fmt.Errorf("the label %q needs more than 63 bytes in its ASCII form", label)
		}

		// The round trip is the check that the encoder agrees with the decoder on
		// this label: a defect here would resolve a domain nobody asked for.
		decoded, err := punyDecode(strings.TrimPrefix(encoded, punyACE))
		if err != nil || decoded != mapped {
			return "", "", fmt.Errorf("the label %q could not be converted to its ASCII form reliably", label)
		}
		converted = append(converted, encoded)
	}

	ascii = strings.Join(converted, ".")
	if err := validateHost(ascii); err != nil {
		return "", "", err
	}
	if strings.ToLower(host) != ascii {
		typed = host
	}
	return ascii, typed, nil
}

// checkConvertibleLabel refuses a label this mapping will not guess at: a
// bidirectional control, a joiner, or a character that is not a letter, a digit,
// a mark or a hyphen. The refusal names what was found, so the reason on screen
// is actionable.
func checkConvertibleLabel(label string) error {
	for _, r := range label {
		switch {
		case r == 0x200C || r == 0x200D:
			return fmt.Errorf("the label %q contains a zero-width joiner, which this utility does not convert: use the ASCII (xn--) form", label)
		case unicode.Is(unicode.Bidi_Control, r) || (r >= 0x2066 && r <= 0x2069) || (r >= 0x0590 && r <= 0x08FF) || (r >= 0xFB1D && r <= 0xFDFF) || (r >= 0xFE70 && r <= 0xFEFF):
			return fmt.Errorf("the label %q is written in a right-to-left script, which this utility does not convert: use the ASCII (xn--) form", label)
		case r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			continue
		default:
			return fmt.Errorf("the label %q contains %q, which is not allowed in a host name", label, r)
		}
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return fmt.Errorf("the label %q starts or ends with a hyphen", label)
	}
	return nil
}

// validateHost checks the ASCII form as the resolver will receive it.
func validateHost(host string) error {
	if len(host) > 253 {
		return fmt.Errorf("a host name may be at most 253 bytes, and %q needs %d", host, len(host))
	}
	trimmed := strings.TrimSuffix(host, ".")
	if trimmed == "" {
		return errors.New("a host name cannot be empty")
	}
	for _, label := range strings.Split(trimmed, ".") {
		if label == "" {
			return fmt.Errorf("the host name %q has an empty label: only a single trailing dot is allowed", host)
		}
		if len(label) > 63 {
			return fmt.Errorf("the label %q is longer than 63 bytes", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("the label %q starts or ends with a hyphen", label)
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			switch {
			case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
				continue
			default:
				return fmt.Errorf("the label %q contains %q, which is not allowed in a host name", label, string(c))
			}
		}
	}
	return nil
}

// isASCII reports whether a string is entirely ASCII.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// punyAdapt is RFC 3492 section 6.1.
func punyAdapt(delta, numPoints int64, firstTime bool) int64 {
	if firstTime {
		delta /= punyDamp
	} else {
		delta /= 2
	}
	delta += delta / numPoints
	k := int64(0)
	for delta > ((punyBase-punyTMin)*punyTMax)/2 {
		delta /= punyBase - punyTMin
		k += punyBase
	}
	return k + (punyBase-punyTMin+1)*delta/(delta+punySkew)
}

// punyEncode is RFC 3492 section 6.3. It returns the encoded label without the
// ACE prefix.
func punyEncode(label string) (string, error) {
	runes := []rune(label)
	var out strings.Builder

	basic := 0
	for _, r := range runes {
		if r >= punyInitialN {
			continue
		}
		out.WriteRune(r)
		basic++
	}

	h := int64(basic)
	if basic > 0 {
		out.WriteByte(punyDelimiter)
	}

	n, delta, bias := int64(punyInitialN), int64(0), int64(punyInitialBias)
	total := int64(len(runes))
	for h < total {
		// The smallest code point at or above n, which is where the next delta
		// starts from.
		m := int64(maxCodePoint + 1)
		for _, r := range runes {
			if int64(r) >= n && int64(r) < m {
				m = int64(r)
			}
		}
		if m > maxCodePoint {
			return "", errors.New("the label contains an unencodable code point")
		}
		delta += (m - n) * (h + 1)
		if delta > int64(maxCodePoint)*int64(len(runes)+1) {
			return "", errors.New("the label is too long to encode")
		}
		n = m

		for _, r := range runes {
			c := int64(r)
			if c < n {
				delta++
				if delta > int64(maxCodePoint)*int64(len(runes)+1) {
					return "", errors.New("the label is too long to encode")
				}
			}
			if c != n {
				continue
			}
			q := delta
			for k := int64(punyBase); ; k += punyBase {
				t := k - bias
				switch {
				case t < punyTMin:
					t = punyTMin
				case t > punyTMax:
					t = punyTMax
				}
				if q < t {
					break
				}
				out.WriteByte(punyDigit(t + (q-t)%(punyBase-t)))
				q = (q - t) / (punyBase - t)
			}
			out.WriteByte(punyDigit(q))
			// firstTime is true only on the pass that follows the basic code
			// points: after one non-basic code point has been written, h is no
			// longer the count it started from (RFC 3492 section 6.3).
			bias = punyAdapt(delta, h+1, h == int64(basic))
			delta = 0
			h++
		}
		delta++
		n++
	}
	return out.String(), nil
}

// punyDecode is RFC 3492 section 6.2. Only the round-trip check uses it, which
// is exactly why it is here: an encoder nobody decodes is an encoder nobody has
// tested against the definition.
func punyDecode(input string) (string, error) {
	var output []rune

	pos := strings.LastIndexByte(input, punyDelimiter)
	if pos > 0 {
		for i := 0; i < pos; i++ {
			if input[i] >= 0x80 {
				return "", errors.New("the encoded label has a non-ASCII basic code point")
			}
			output = append(output, rune(input[i]))
		}
		input = input[pos+1:]
	}

	n, i, bias := int64(punyInitialN), int64(0), int64(punyInitialBias)
	for pos := 0; pos < len(input); {
		oldi := i
		w := int64(1)
		for k := int64(punyBase); ; k += punyBase {
			if pos >= len(input) {
				return "", errors.New("the encoded label ends inside a digit sequence")
			}
			digit, err := punyDigitValue(input[pos])
			if err != nil {
				return "", err
			}
			pos++
			i += digit * w
			if i > int64(maxCodePoint)*int64(len(input)+1) {
				return "", errors.New("the encoded label overflows")
			}
			t := k - bias
			switch {
			case t < punyTMin:
				t = punyTMin
			case t > punyTMax:
				t = punyTMax
			}
			if digit < t {
				break
			}
			w *= punyBase - t
			if w > int64(maxCodePoint)*int64(len(input)+1) {
				return "", errors.New("the encoded label overflows")
			}
		}
		bias = punyAdapt(i-oldi, int64(len(output))+1, oldi == 0)
		n += i / (int64(len(output)) + 1)
		i %= int64(len(output)) + 1
		if n > maxCodePoint {
			return "", errors.New("the encoded label decodes past the last code point")
		}
		output = append(output, 0)
		copy(output[i+1:], output[i:])
		output[i] = rune(n)
		i++
	}
	return string(output), nil
}

// punyDigitValue is one digit's value, case-insensitively.
func punyDigitValue(b byte) (int64, error) {
	switch {
	case b >= 'a' && b <= 'z':
		return int64(b - 'a'), nil
	case b >= 'A' && b <= 'Z':
		return int64(b - 'A'), nil
	case b >= '0' && b <= '9':
		return int64(b-'0') + 26, nil
	}
	return 0, fmt.Errorf("the encoded label contains %q, which is not a punycode digit", string(b))
}

// punyDigit is the inverse of punyDigitValue.
func punyDigit(d int64) byte {
	if d < 26 {
		return byte('a' + d)
	}
	return byte('0' + d - 26)
}
