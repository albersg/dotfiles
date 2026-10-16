package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui"
)

// TestTheOperatingSystemFlagIsDeclaredAndRefusesWhatItCannotOffer holds both
// ends of --os: that registerFlags really declares it (the help guard then
// demands it be documented), and that the value the flag carries is refused
// before the run starts rather than silently replaced by the detected platform,
// which is what the non-interactive path used to do.
func TestTheOperatingSystemFlagIsDeclaredAndRefusesWhatItCannotOffer(t *testing.T) {
	fs := flag.NewFlagSet("os-flag-check", flag.ContinueOnError)
	registerFlags(fs, &cliFlags{})
	if fs.Lookup("os") == nil {
		t.Fatal("registerFlags declares no --os, so this guard proves nothing")
	}

	platform, err := platformFromFlags(&cliFlags{os: ""})
	if err != nil {
		t.Errorf("no platform given is the machine's answer and must be accepted: %v", err)
	}
	if platform != "" {
		t.Errorf("no platform given produced %q, want nothing so the path can detect the machine", platform)
	}

	for _, chosen := range tui.SupportedOperatingSystems() {
		got, err := platformFromFlags(&cliFlags{os: chosen})
		if err != nil {
			t.Errorf("--os=%s is a value the screen offers and the flag refuses it: %v", chosen, err)
			continue
		}
		if got != chosen {
			t.Errorf("--os=%s reached the run as %q", chosen, got)
		}
	}

	for _, refused := range []string{"wsl", "windows", "Mac"} {
		if _, err := platformFromFlags(&cliFlags{os: refused}); err == nil {
			t.Errorf("--os=%s is not a value the screen offers and the flag accepted it", refused)
			continue
		} else {
			for _, value := range tui.SupportedOperatingSystems() {
				if !strings.Contains(err.Error(), value) {
					t.Errorf("the refusal of %q does not name %q, so it does not teach what is accepted: %v", refused, value, err)
				}
			}
		}
	}
}

// TestTheHelpNamesEveryPlatformTheFlagAccepts is the help's half of the same
// contract: the accepted values are the only thing that tells a user what to
// write, so a list that drifts from the flag's vocabulary is worse than no list.
// The guard reads both from the flag's own vocabulary rather than from a copy.
func TestTheHelpNamesEveryPlatformTheFlagAccepts(t *testing.T) {
	values := tui.SupportedOperatingSystems()
	if len(values) == 0 {
		t.Fatal("--os offers nothing, so this guard proves nothing")
	}
	for _, value := range values {
		if !strings.Contains(helpText, value) {
			t.Errorf("the help does not name the platform %q, so the accepted values and the flag have drifted apart", value)
		}
	}
}
