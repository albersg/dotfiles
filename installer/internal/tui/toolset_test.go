package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// TestFilterBrewfile pins the pure filtering rule the toolset step applies
// before it hands the Brewfile to `brew bundle`. Only the vscode and winget
// directives are dropped; every other line has to survive byte for byte, in
// order, including comments, blank lines and the inline OS.linux? guard.
func TestFilterBrewfile(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		dropped map[string]int
	}{
		{
			name: "mixed directives",
			input: strings.Join([]string{
				"# Machine toolset.",
				"",
				`brew "btop"`,
				`tap "homebrew/cask-fonts"`,
				`cask "font-iosevka-term-nerd-font"`,
				`mas "Xcode", id: 497799835`,
				`go "example.com/tools/v2/cmd/tool"`,
				`npm "@scope/package"`,
				`uv "harlequin[mysql]"`,
				`vscode "ms-python.python"`,
				`vscode "github.copilot"`,
				`winget "Git.Git"`,
				`brew "spotify_player" if OS.linux?`,
				"# trailing comment",
				"",
			}, "\n"),
			want: strings.Join([]string{
				"# Machine toolset.",
				"",
				`brew "btop"`,
				`tap "homebrew/cask-fonts"`,
				`cask "font-iosevka-term-nerd-font"`,
				`mas "Xcode", id: 497799835`,
				`go "example.com/tools/v2/cmd/tool"`,
				`npm "@scope/package"`,
				`uv "harlequin[mysql]"`,
				`brew "spotify_player" if OS.linux?`,
				"# trailing comment",
				"",
			}, "\n"),
			dropped: map[string]int{"vscode": 2, "winget": 1},
		},
		{
			name:    "empty input",
			input:   "",
			want:    "",
			dropped: map[string]int{},
		},
		{
			name: "no droppable directive",
			input: strings.Join([]string{
				"# Nothing to drop here.",
				"",
				`brew "btop"`,
				`cask "font-iosevka"`,
				`brew "spotify_player" if OS.linux?`,
				"",
			}, "\n"),
			want: strings.Join([]string{
				"# Nothing to drop here.",
				"",
				`brew "btop"`,
				`cask "font-iosevka"`,
				`brew "spotify_player" if OS.linux?`,
				"",
			}, "\n"),
			dropped: map[string]int{},
		},
		{
			name: "similar prefixes are kept",
			input: strings.Join([]string{
				`vscodex "not-a-directive"`,
				`winget-cli "also-not-a-directive"`,
				`vscode "really-dropped"`,
				`# vscode "commented-out"`,
				"",
			}, "\n"),
			want: strings.Join([]string{
				`vscodex "not-a-directive"`,
				`winget-cli "also-not-a-directive"`,
				`# vscode "commented-out"`,
				"",
			}, "\n"),
			dropped: map[string]int{"vscode": 1},
		},
		{
			name: "no trailing newline is preserved",
			input: `brew "btop"` + "\n" +
				`winget "Git.Git"`,
			want:    `brew "btop"` + "\n",
			dropped: map[string]int{"winget": 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, dropped := filterBrewfile(tc.input)
			if got != tc.want {
				t.Errorf("filterBrewfile output = %q, want %q", got, tc.want)
			}
			if dropped == nil {
				t.Fatal("filterBrewfile must return a non-nil map so a caller can count every directive")
			}
			if !reflect.DeepEqual(dropped, tc.dropped) {
				t.Errorf("filterBrewfile dropped = %#v, want %#v", dropped, tc.dropped)
			}
		})
	}
}

// TestFilteredRepositoryBrewfile runs the filter over the real inventory the
// installer ships, so a future edit to the Brewfile that reintroduces a dropped
// directive, or removes one of the entries the toolset must provision, is
// noticed here. The dropped counts are deliberately not pinned: the point is
// that the report stays honest, not that the inventory never grows.
func TestFilteredRepositoryBrewfile(t *testing.T) {
	path := filepath.Join(repoRoot(t), repoAssetBrewfile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", repoAssetBrewfile, err)
	}

	filtered, dropped := filterBrewfile(string(content))

	for line := range strings.SplitSeq(filtered, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "vscode") || strings.HasPrefix(trimmed, "winget") {
			t.Errorf("the filtered toolset still carries %q", trimmed)
		}
	}
	for _, want := range []string{
		`brew "btop"`,
		`brew "sops"`,
		`brew "age"`,
		`brew "k9s"`,
		`go "`,
	} {
		if !strings.Contains(filtered, want) {
			t.Errorf("the filtered toolset no longer contains %q", want)
		}
	}
	if dropped["vscode"] <= 0 {
		t.Errorf("the report does not count the dropped vscode lines: %#v", dropped)
	}
	if dropped["winget"] <= 0 {
		t.Errorf("the report does not count the dropped winget lines: %#v", dropped)
	}
}

// stubToolsetBrew replaces the brew runner the toolset step reaches and counts
// how many times it is called. It follows the stubbing pattern the dependency
// and package tests use, restoring the original runner on cleanup.
func stubToolsetBrew(t *testing.T) *int {
	t.Helper()

	original := runBrewWithLogs
	calls := 0
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls++
		return &system.ExecResult{Command: args}
	}
	t.Cleanup(func() { runBrewWithLogs = original })

	return &calls
}

// withoutBrew pins system.BrewInstalled to false without touching the host: PATH
// is reduced to an empty directory and HOMEBREW_PREFIX points at a path that
// does not exist. Both checks the detection performs then miss.
func withoutBrew(t *testing.T) {
	t.Helper()

	t.Setenv("HOMEBREW_PREFIX", filepath.Join(t.TempDir(), "no-homebrew-here"))
	t.Setenv("PATH", t.TempDir())
}

// TestStepInstallToolsetHonoursSkipEnv proves the documented escape hatch runs
// nothing.
func TestStepInstallToolsetHonoursSkipEnv(t *testing.T) {
	calls := stubToolsetBrew(t)
	t.Setenv(envSkipToolset, "1")

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}}
	if err := stepInstallToolset(m); err != nil {
		t.Fatalf("a requested skip must not fail: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("a requested skip must not run brew, got %d calls", *calls)
	}
}

// TestStepInstallToolsetSkipsTermux proves a Termux host never reaches brew,
// which Termux does not carry.
func TestStepInstallToolsetSkipsTermux(t *testing.T) {
	calls := stubToolsetBrew(t)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSTermux, IsTermux: true}}
	if err := stepInstallToolset(m); err != nil {
		t.Fatalf("Termux must not fail: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("Termux must not run brew, got %d calls", *calls)
	}
}

// TestStepInstallToolsetSkipsWithoutHomebrew proves a host where the Homebrew
// step left no usable brew skips the toolset instead of reporting a failure.
func TestStepInstallToolsetSkipsWithoutHomebrew(t *testing.T) {
	calls := stubToolsetBrew(t)
	withoutBrew(t)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}}
	if err := stepInstallToolset(m); err != nil {
		t.Fatalf("a host without Homebrew must not fail: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("a host without Homebrew must not run brew, got %d calls", *calls)
	}
}

// TestStepInstallToolsetRunsBrewBundle is the positive control: with a usable
// brew and a checkout that carries a Brewfile, the step runs brew bundle once
// against a filtered temporary file, disables the upgrade brew bundle would
// otherwise perform, and removes the temporary file on the way out.
func TestStepInstallToolsetRunsBrewBundle(t *testing.T) {
	original := runBrewWithLogs
	calls := 0
	var gotArgs string
	var gotEnv []string
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls++
		gotArgs = args
		if opts != nil {
			gotEnv = opts.Env
		}
		return &system.ExecResult{Command: args}
	}
	t.Cleanup(func() { runBrewWithLogs = original })

	// A stub brew on PATH makes system.BrewInstalled report true.
	binDir := t.TempDir()
	writeStubCommand(t, binDir, "brew", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir)
	t.Setenv("HOMEBREW_PREFIX", binDir)

	repoDir := t.TempDir()
	brewfile := "brew \"btop\"\nvscode \"ms-python.python\"\n"
	if err := os.WriteFile(filepath.Join(repoDir, repoAssetBrewfile), []byte(brewfile), 0o644); err != nil {
		t.Fatalf("cannot write the fixture Brewfile: %v", err)
	}

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}, RepoDir: repoDir}
	if err := stepInstallToolset(m); err != nil {
		t.Fatalf("a successful toolset run must not fail: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the step must run brew bundle exactly once, got %d", calls)
	}
	if !strings.HasPrefix(gotArgs, "bundle install --file=") {
		t.Errorf("brew was not asked to bundle install: %q", gotArgs)
	}
	if !reflect.DeepEqual(gotEnv, []string{"HOMEBREW_BUNDLE_NO_UPGRADE=1"}) {
		t.Errorf("brew bundle must run with HOMEBREW_BUNDLE_NO_UPGRADE=1, got %v", gotEnv)
	}

	tmpPath := strings.Trim(strings.TrimPrefix(gotArgs, "bundle install --file="), `"`)
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("the temporary Brewfile %s survived the step", tmpPath)
	}
}
