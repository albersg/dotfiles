package netdiag

import (
	"strings"
	"testing"
)

// TestHostToASCIIMatchesThePunycodeVectors pins the encoder against the values
// the definition's own examples produce. If this test fails, every
// internationalized domain this utility diagnoses is the wrong domain.
func TestHostToASCIIMatchesThePunycodeVectors(t *testing.T) {
	cases := []struct {
		host  string
		want  string
		typed string
	}{
		{host: "example.com", want: "example.com"},
		// Case alone is not a conversion worth reporting: DNS is case-insensitive,
		// and naming the domain twice would only add noise to the report.
		{host: "Example.COM", want: "example.com"},
		{host: "bücher", want: "xn--bcher-kva", typed: "bücher"},
		{host: "bücher.de", want: "xn--bcher-kva.de", typed: "bücher.de"},
		{host: "münchen.de", want: "xn--mnchen-3ya.de", typed: "münchen.de"},
		{host: "日本語.jp", want: "xn--wgv71a119e.jp", typed: "日本語.jp"},
		{host: "пример.рф", want: "xn--e1afmkfd.xn--p1ai", typed: "пример.рф"},
		{host: "café.fr", want: "xn--caf-dma.fr", typed: "café.fr"},
		{host: "español.es", want: "xn--espaol-zwa.es", typed: "español.es"},
		{host: "straße.de", want: "xn--strae-oqa.de", typed: "straße.de"},
		{host: "ñandú.cl", want: "xn--and-6ma2c.cl", typed: "ñandú.cl"},
		// An already-converted name is kept as it stands, so a target pasted from
		// a browser works the same way.
		{host: "xn--bcher-kva.de", want: "xn--bcher-kva.de"},
		// A trailing dot is a fully qualified name, not an empty label.
		{host: "bücher.de.", want: "xn--bcher-kva.de.", typed: "bücher.de."},
	}

	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			ascii, typed, err := hostToASCII(c.host)
			if err != nil {
				t.Fatalf("hostToASCII(%q) failed: %v", c.host, err)
			}
			if ascii != c.want {
				t.Errorf("hostToASCII(%q) = %q, want %q", c.host, ascii, c.want)
			}
			if typed != c.typed {
				t.Errorf("hostToASCII(%q) kept %q as the name as typed, want %q: the report has to say which domain was actually resolved", c.host, typed, c.typed)
			}
		})
	}
}

// TestPunycodeRoundTripIsTheDecodersOwnAnswer pins the self-check: whatever the
// encoder produces must decode back to the label it came from. The encoder's
// caller runs the same comparison for every internationalized label, and this
// test is what proves the decoder it compares against is not merely agreeing with
// a shared mistake.
func TestPunycodeRoundTripIsTheDecodersOwnAnswer(t *testing.T) {
	labels := []string{
		"bcher-kva", "mnchen-3ya", "wgv71a119e", "e1afmkfd", "caf-dma",
		"espaol-zwa", "strae-oqa", "and-6ma2c", "maana-pta", "bcher",
	}
	for _, label := range labels {
		decoded, err := punyDecode(label)
		if err != nil {
			t.Fatalf("punyDecode(%q): %v", label, err)
		}
		encoded, err := punyEncode(decoded)
		if err != nil {
			t.Fatalf("punyEncode(%q): %v", decoded, err)
		}
		if encoded != label {
			t.Errorf("round trip of %q went through %q and came back %q", label, decoded, encoded)
		}
	}

	if _, err := punyDecode("bcher-kv$"); err == nil {
		t.Error("punyDecode accepted a label with a character that is not a punycode digit")
	}
	if _, err := punyDecode(strings.Repeat("9", 64)); err == nil {
		t.Error("punyDecode accepted a label that decodes past the last code point instead of refusing it")
	}
}

// TestHostToASCIIRefusesWhatItCannotStandBehind pins the conservative half: a
// right-to-left or joiner label is refused with a reason instead of converted,
// because a guess there is a spoofing hazard.
func TestHostToASCIIRefusesWhatItCannotStandBehind(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{host: "مثال.إختبار", want: "right-to-left"},
		{host: "a\u200Db.com", want: "zero-width joiner"},
		{host: "español!.es", want: "not allowed in a host name"},
		{host: "exa_mple.com", want: ""},
		{host: "bücher-.de", want: "ends with a hyphen"},
		{host: "", want: ""},
	}

	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			_, _, err := hostToASCII(c.host)
			if c.want == "" && c.host == "exa_mple.com" {
				// An underscore is allowed: service labels such as _ldap._tcp are
				// real host names on a corporate network, and refusing them would
				// be refusing the networks this utility exists for.
				if err != nil {
					t.Fatalf("hostToASCII(%q) refused an underscore label: %v", c.host, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("hostToASCII(%q) was accepted; the mapping must refuse what it cannot convert", c.host)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("the reason for %q is %q, want it to contain %q", c.host, err.Error(), c.want)
			}
		})
	}
}

// TestValidateHostRefusesShapesAResolverCannotTake pins the validation of the
// ASCII result, which is the last place a malformed name can be caught before a
// resolver sees it.
func TestValidateHostRefusesShapesAResolverCannotTake(t *testing.T) {
	tooLongLabel := strings.Repeat("a", 64) + ".com"
	tooLongTotal := strings.Repeat("abcdefghij.", 26) + "com"

	cases := []struct {
		host string
		ok   bool
	}{
		{host: "example.com", ok: true},
		{host: "example.com.", ok: true},
		{host: "xn--bcher-kva.de", ok: true},
		{host: "_ldap._tcp.corp.local", ok: true},
		{host: "a", ok: true},
		{host: "", ok: false},
		{host: ".", ok: false},
		{host: "a..b", ok: false},
		{host: "-a.com", ok: false},
		{host: "a-.com", ok: false},
		{host: "UPPER.com", ok: false},
		{host: tooLongLabel, ok: false},
		{host: tooLongTotal, ok: false},
	}

	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			err := validateHost(c.host)
			if c.ok && err != nil {
				t.Errorf("validateHost(%q) refused a valid name: %v", c.host, err)
			}
			if !c.ok && err == nil {
				t.Errorf("validateHost(%q) accepted a name a resolver cannot take", c.host)
			}
		})
	}
}
