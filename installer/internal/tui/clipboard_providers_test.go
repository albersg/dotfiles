package tui

import (
	"reflect"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// TestClipboardProvidersForNeovim pins which platforms get a clipboard provider
// and which do not. Neovim ships with clipboard=unnamedplus in this
// configuration, so on a platform that gets no provider every yank stays inside
// Neovim and silently never reaches the desktop clipboard.
func TestClipboardProvidersForNeovim(t *testing.T) {
	cases := []struct {
		name   string
		info   *system.SystemInfo
		needed bool
		reason string
	}{
		{
			name:   "macOS",
			info:   &system.SystemInfo{OS: system.OSMac},
			needed: false,
			reason: "Neovim uses pbcopy and pbpaste, so no package is required",
		},
		{
			name:   "Termux",
			info:   &system.SystemInfo{OS: system.OSTermux, IsTermux: true},
			needed: false,
			reason: "Termux has neither package and uses the termux-api clipboard",
		},
		{
			name:   "Debian",
			info:   &system.SystemInfo{OS: system.OSDebian},
			needed: true,
			reason: "desktop Linux needs a provider",
		},
		{
			name:   "Arch",
			info:   &system.SystemInfo{OS: system.OSArch},
			needed: true,
			reason: "desktop Linux needs a provider",
		},
		{
			name:   "Fedora",
			info:   &system.SystemInfo{OS: system.OSFedora},
			needed: true,
			reason: "desktop Linux needs a provider",
		},
		{
			name:   "WSL",
			info:   &system.SystemInfo{OS: system.OSWSL, IsWSL: true, HasBrew: true},
			needed: true,
			reason: "WSLg offers Wayland and XWayland, and either may be the one that works",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packages, needed := clipboardProvidersFor(tc.info)

			if needed != tc.needed {
				t.Fatalf("clipboardProvidersFor(%s) needed = %v, want %v: %s", tc.name, needed, tc.needed, tc.reason)
			}
			if !tc.needed {
				if packages != "" {
					t.Errorf("clipboardProvidersFor(%s) returned %q, want no packages", tc.name, packages)
				}
				return
			}
			if packages != "xclip wl-clipboard" {
				t.Errorf("clipboardProvidersFor(%s) packages = %q, want %q", tc.name, packages, "xclip wl-clipboard")
			}
		})
	}
}

// TestClipboardPlatformPackagesAreFilled keeps every package manager wired to
// the providers. An empty field means that platform installs nothing and says
// nothing, which is the exact failure this whole change exists to remove.
func TestClipboardPlatformPackagesAreFilled(t *testing.T) {
	packages := clipboardPlatformPackages("xclip wl-clipboard")

	fields := map[string]string{
		"Brew":   packages.Brew,
		"Arch":   packages.Arch,
		"Fedora": packages.Fedora,
		"Debian": packages.Debian,
	}
	for name, value := range fields {
		if value != "xclip wl-clipboard" {
			t.Errorf("platformPackages.%s = %q, want %q", name, value, "xclip wl-clipboard")
		}
	}
}

// TestClipboardProvidersAreInstalledPerPlatform runs the real dispatch to check
// the command each package manager receives. The providers are plain package
// names, so getting the dispatch wrong here means the packages are never
// installed and the failure stays invisible until someone yanks a line.
func TestClipboardProvidersAreInstalledPerPlatform(t *testing.T) {
	packages, needed := clipboardProvidersFor(&system.SystemInfo{OS: system.OSDebian})
	if !needed {
		t.Fatal("expected Debian to need clipboard providers")
	}

	cases := []struct {
		name     string
		info     *system.SystemInfo
		expected []packageCommandCall
	}{
		{
			name: "Debian without Homebrew uses apt",
			info: &system.SystemInfo{OS: system.OSDebian, HasBrew: false},
			expected: []packageCommandCall{
				{runner: "sudo", command: "apt-get install -y xclip wl-clipboard"},
			},
		},
		{
			name: "Debian with Homebrew uses brew",
			info: &system.SystemInfo{OS: system.OSDebian, HasBrew: true},
			expected: []packageCommandCall{
				{runner: "brew", command: "install xclip wl-clipboard"},
			},
		},
		{
			name: "Arch uses pacman",
			info: &system.SystemInfo{OS: system.OSArch, HasBrew: false},
			expected: []packageCommandCall{
				{runner: "sudo", command: "pacman -S --needed --noconfirm xclip wl-clipboard"},
			},
		},
		{
			name: "Fedora uses dnf",
			info: &system.SystemInfo{OS: system.OSFedora, HasBrew: false},
			expected: []packageCommandCall{
				{runner: "sudo", command: "dnf install -y --skip-unavailable xclip wl-clipboard"},
			},
		},
		{
			// WSL with no recognised distribution stays OSWSL. With Homebrew
			// present the default branch installs through it.
			name: "WSL uses Homebrew",
			info: &system.SystemInfo{OS: system.OSWSL, IsWSL: true, HasBrew: true},
			expected: []packageCommandCall{
				{runner: "brew", command: "install xclip wl-clipboard"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := withPackageCommandMocks(t, nil)

			m := &Model{SystemInfo: tc.info}
			result := installPlatformPackages(m, "nvim", clipboardPlatformPackages(packages), nil)

			if result.Error != nil {
				t.Fatalf("expected the clipboard install to succeed, got error: %v", result.Error)
			}
			if !reflect.DeepEqual(*calls, tc.expected) {
				t.Fatalf("calls = %#v, want %#v", *calls, tc.expected)
			}
		})
	}
}
