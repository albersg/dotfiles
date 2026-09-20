package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

func terminalListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestSupportedTerminalsIsThePlatformRule pins the only platform-dependent
// value on the terminal axis. The CLI validator, the --terminal flag
// description and stepInstallTerminal all read this list, so a change here
// changes what the tool offers, refuses and names in a refusal.
func TestSupportedTerminalsIsThePlatformRule(t *testing.T) {
	mac := SupportedTerminals("darwin")
	linux := SupportedTerminals("linux")

	if !terminalListContains(mac, "kitty") {
		t.Errorf("SupportedTerminals(darwin) = %v, want it to include kitty", mac)
	}
	if terminalListContains(linux, "kitty") {
		t.Errorf("SupportedTerminals(linux) = %v, want it to exclude kitty", linux)
	}

	// Every other value is platform-independent, and "none" is a choice rather
	// than a terminal, so it has to survive on both platforms.
	for _, goos := range []string{"darwin", "linux"} {
		for _, terminal := range []string{"alacritty", "wezterm", "ghostty", "none"} {
			if !terminalListContains(SupportedTerminals(goos), terminal) {
				t.Errorf("SupportedTerminals(%s) = %v, want it to include %s", goos, SupportedTerminals(goos), terminal)
			}
		}
	}
}

// TestValidateTerminalRefusesAndNamesWhatItSupports covers the CLI boundary:
// the input is refused before anything is planned, and the message names every
// value the platform does support.
func TestValidateTerminalRefusesAndNamesWhatItSupports(t *testing.T) {
	cases := []struct {
		name     string
		terminal string
		goos     string
		wantErr  bool
	}{
		{"kitty is honoured on macOS", "kitty", "darwin", false},
		{"kitty is refused off macOS", "kitty", "linux", true},
		{"unknown terminal is refused", "hyper", "linux", true},
		{"unknown terminal is refused on macOS too", "hyper", "darwin", true},
		{"none is accepted everywhere", "none", "linux", false},
		{"alacritty is accepted everywhere", "alacritty", "linux", false},
		{"wezterm is accepted everywhere", "wezterm", "linux", false},
		{"ghostty is accepted everywhere", "ghostty", "linux", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTerminal(tc.terminal, tc.goos)

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("ValidateTerminal(%q, %q) = %v, want nil", tc.terminal, tc.goos, err)
				}
				// The acceptance set has to be exactly what the same rule
				// returns, or the refusal below would be describing a different
				// tool than the one that accepts values.
				for _, supported := range SupportedTerminals(tc.goos) {
					if err := ValidateTerminal(supported, tc.goos); err != nil {
						t.Errorf("ValidateTerminal(%q, %q) = %v, but SupportedTerminals lists it", supported, tc.goos, err)
					}
				}
				return
			}

			if err == nil {
				t.Fatalf("ValidateTerminal(%q, %q) = nil, want a refusal", tc.terminal, tc.goos)
			}
			message := err.Error()
			if !strings.Contains(message, tc.terminal) {
				t.Errorf("refusal %q does not name the refused value %q", message, tc.terminal)
			}

			_, validList, found := strings.Cut(message, "(valid: ")
			if !found {
				t.Fatalf("refusal %q does not name the supported values", message)
			}
			validList = strings.TrimSuffix(validList, ")")
			for _, supported := range SupportedTerminals(tc.goos) {
				if !strings.Contains(validList, supported) {
					t.Errorf("refusal %q does not name %q as supported on %s", message, supported, tc.goos)
				}
			}
			if strings.Contains(validList, tc.terminal) {
				t.Errorf("refusal %q offers the refused value %q as valid", message, tc.terminal)
			}
		})
	}
}

// TestKittyActionNeverReportsPresentWithoutVerifyingIt is the invariant the old
// branch broke: "Kitty already installed" was reachable on a platform where the
// binary had never been looked for. The presence report is now only reachable
// from a verified presence, and a platform this installer has no route for is
// refused whatever the presence check found.
func TestKittyActionNeverReportsPresentWithoutVerifyingIt(t *testing.T) {
	cases := []struct {
		name    string
		onMac   bool
		present bool
		want    kittyAction
	}{
		{"macOS, binary absent", true, false, kittyInstall},
		{"macOS, binary present", true, true, kittyPresent},
		{"Linux, binary absent", false, false, kittyRefuse},
		{"Linux, binary present", false, true, kittyRefuse},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kittyActionFor(tc.onMac, tc.present)
			if got != tc.want {
				t.Fatalf("kittyActionFor(onMac=%v, present=%v) = %v, want %v", tc.onMac, tc.present, got, tc.want)
			}
			if got == kittyPresent && !tc.present {
				t.Error("kitty would be reported present although the presence check found it absent")
			}
		})
	}
}

// TestStepInstallTerminalRefusesKittyOnEveryNonMacPlatform exercises the step
// itself, not only the decision function, and asserts the refusal leaves the
// machine untouched: the step is the last line of defence when a value reaches
// it without passing the CLI validator.
func TestStepInstallTerminalRefusesKittyOnEveryNonMacPlatform(t *testing.T) {
	platforms := []struct {
		name   string
		osType system.OSType
	}{
		{"Linux", system.OSLinux},
		{"Debian/Ubuntu", system.OSDebian},
		{"Fedora", system.OSFedora},
		{"Arch", system.OSArch},
		{"WSL", system.OSWSL},
		{"Termux", system.OSTermux},
		{"unknown", system.OSUnknown},
	}

	for _, platform := range platforms {
		t.Run(platform.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			m := &Model{
				SystemInfo: &system.SystemInfo{OS: platform.osType, OSName: platform.name},
				Choices:    UserChoices{Terminal: "kitty", OS: "linux"},
			}

			err := stepInstallTerminal(m)
			if err == nil {
				t.Fatalf("stepInstallTerminal(kitty) on %s = nil, want a refusal", platform.name)
			}

			message := err.Error()
			if !strings.Contains(message, "Kitty is only installable on macOS") {
				t.Errorf("refusal on %s = %q, want it to say kitty is macOS-only", platform.name, message)
			}
			for _, supported := range SupportedTerminals("linux") {
				if !strings.Contains(message, supported) {
					t.Errorf("refusal on %s = %q, want it to name %q as supported", platform.name, message, supported)
				}
			}

			// Refused means nothing was written, not even the configuration
			// directory the step used to create after claiming an installation.
			configDir := filepath.Join(home, ".config", "kitty")
			if _, statErr := os.Stat(configDir); !os.IsNotExist(statErr) {
				t.Errorf("stepInstallTerminal created %s on %s although it refused the terminal", configDir, platform.name)
			}
		})
	}
}
