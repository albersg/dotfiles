package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The capability matrix: which shell, which start, which instrument
// ---------------------------------------------------------------------------
//
// The utility used to start one command -- `$SHELL -i -c exit` -- and call it
// "the shell's startup". It is one start of several, and each shell spells the
// others differently: zsh skips its own files with `-d -f`, bash with
// `--noprofile --norc`, fish and nushell with a flag of their own, and `sh` has
// no flag at all and has to be pointed at an empty $ENV. The guards below pin
// the matrix, and pin the two refusals that keep a "clean" measurement from
// quietly becoming a measurement of the user's own files: a shell whose family
// is unknown, and a clean start with no directory to be clean in.

// shellAuditStartEnvValue is one variable of a start's environment, or the empty
// string when the start does not set it.
func shellAuditStartEnvValue(start shellAuditStart, key string) string {
	for _, entry := range start.Env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			return value
		}
	}
	return ""
}

// TestShellAuditKindOfNamesTheFamiliesItKnows pins the mapping from the binary
// to the family, including the shapes a real $SHELL has: a full path, an
// upper-case name and a Windows `.exe`.
func TestShellAuditKindOfNamesTheFamiliesItKnows(t *testing.T) {
	tests := []struct {
		name string
		want shellAuditKind
	}{
		{"zsh", shellAuditKindZsh},
		{"/usr/bin/zsh", shellAuditKindZsh},
		{"ZSH", shellAuditKindZsh},
		{"/home/user/.local/bin/zsh", shellAuditKindZsh},
		{"bash", shellAuditKindBash},
		{"/bin/bash", shellAuditKindBash},
		{"bash.exe", shellAuditKindBash},
		{"fish", shellAuditKindFish},
		{"/usr/local/bin/fish", shellAuditKindFish},
		{"nu", shellAuditKindNushell},
		{"nushell", shellAuditKindNushell},
		{"/usr/bin/nu", shellAuditKindNushell},
		{"sh", shellAuditKindPosix},
		{"/bin/sh", shellAuditKindPosix},
		{"dash", shellAuditKindPosix},
		{"ash", shellAuditKindPosix},
		{"ksh", shellAuditKindPosix},
		{"mksh", shellAuditKindPosix},
		{"yash", shellAuditKindPosix},
		// A shell this utility has never seen is not guessed at: it is measured
		// with the one start every shell that answers `-c` shares, and refused
		// every other start with a reason.
		{"elvish", shellAuditKindUnknown},
		{"xonsh", shellAuditKindUnknown},
		{"tcsh", shellAuditKindUnknown},
		{"powershell.exe", shellAuditKindUnknown},
		{"", shellAuditKindUnknown},
	}

	for _, tt := range tests {
		if got := shellAuditKindOf(tt.name); got != tt.want {
			t.Errorf("shellAuditKindOf(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestShellAuditModeNamesTheStartItDescribes pins the two names every mode has:
// the phrase the screen draws and the stable key the history is grouped by. A
// screen that called a login shell "interactive" would be describing a start the
// user never gets.
func TestShellAuditModeNamesTheStartItDescribes(t *testing.T) {
	tests := []struct {
		mode  shellAuditMode
		label string
		key   string
	}{
		{shellAuditModeInteractive, "interactive", "interactive"},
		{shellAuditModeLogin, "login, interactive", "login"},
		{shellAuditModeNonInteractive, "non-interactive", "non-interactive"},
		{shellAuditModeClean, "interactive, no configuration", "interactive-clean"},
		{shellAuditMode{Interactive: true, Login: true, Clean: true}, "login, interactive, no configuration", "login-clean"},
		{shellAuditMode{Clean: true}, "non-interactive, no configuration", "non-interactive-clean"},
	}

	for _, tt := range tests {
		if got := tt.mode.label(); got != tt.label {
			t.Errorf("%+v.label() = %q, want %q", tt.mode, got, tt.label)
		}
		if got := tt.mode.key(); got != tt.key {
			t.Errorf("%+v.key() = %q, want %q: the history is grouped by this key", tt.mode, got, tt.key)
		}
	}
}

// TestShellAuditStartForUsesEachShellsOwnMechanism is the heart of the matrix:
// every family is started the way that family documents, and the clean start
// reads none of the user's files.
func TestShellAuditStartForUsesEachShellsOwnMechanism(t *testing.T) {
	const cleanDir = "/var/tmp/dotfiles-shell-audit-clean"

	tests := []struct {
		name      string
		kind      shellAuditKind
		mode      shellAuditMode
		wantArgs  []string
		wantEnv   map[string]string
		wantClean bool
	}{
		{
			name:     "zsh interactive",
			kind:     shellAuditKindZsh,
			mode:     shellAuditModeInteractive,
			wantArgs: []string{"-i", "-c", "exit"},
		},
		{
			name:     "zsh login",
			kind:     shellAuditKindZsh,
			mode:     shellAuditModeLogin,
			wantArgs: []string{"-l", "-i", "-c", "exit"},
		},
		{
			name:     "zsh non-interactive",
			kind:     shellAuditKindZsh,
			mode:     shellAuditModeNonInteractive,
			wantArgs: []string{"-c", "exit"},
		},
		{
			name:      "zsh clean",
			kind:      shellAuditKindZsh,
			mode:      shellAuditModeClean,
			wantArgs:  []string{"-d", "-f", "-i", "-c", "exit"},
			wantEnv:   map[string]string{"ZDOTDIR": cleanDir},
			wantClean: true,
		},
		{
			name:     "bash interactive",
			kind:     shellAuditKindBash,
			mode:     shellAuditModeInteractive,
			wantArgs: []string{"-i", "-c", "exit"},
		},
		{
			name:     "bash login",
			kind:     shellAuditKindBash,
			mode:     shellAuditModeLogin,
			wantArgs: []string{"-l", "-i", "-c", "exit"},
		},
		{
			name:      "bash clean",
			kind:      shellAuditKindBash,
			mode:      shellAuditModeClean,
			wantArgs:  []string{"--noprofile", "--norc", "-i", "-c", "exit"},
			wantEnv:   map[string]string{"BASH_ENV": "/dev/null", "ENV": "/dev/null"},
			wantClean: true,
		},
		{
			name:     "fish interactive",
			kind:     shellAuditKindFish,
			mode:     shellAuditModeInteractive,
			wantArgs: []string{"-i", "-c", "exit"},
		},
		{
			name:     "fish login",
			kind:     shellAuditKindFish,
			mode:     shellAuditModeLogin,
			wantArgs: []string{"-l", "-i", "-c", "exit"},
		},
		{
			name:      "fish clean",
			kind:      shellAuditKindFish,
			mode:      shellAuditModeClean,
			wantArgs:  []string{"--no-config", "-i", "-c", "exit"},
			wantEnv:   map[string]string{"XDG_CONFIG_HOME": cleanDir},
			wantClean: true,
		},
		{
			name:     "nushell interactive",
			kind:     shellAuditKindNushell,
			mode:     shellAuditModeInteractive,
			wantArgs: []string{"-i", "-c", "exit"},
		},
		{
			name:     "nushell login",
			kind:     shellAuditKindNushell,
			mode:     shellAuditModeLogin,
			wantArgs: []string{"-l", "-i", "-c", "exit"},
		},
		{
			name:      "nushell clean",
			kind:      shellAuditKindNushell,
			mode:      shellAuditModeClean,
			wantArgs:  []string{"--no-config-file", "-i", "-c", "exit"},
			wantEnv:   map[string]string{"XDG_CONFIG_HOME": cleanDir},
			wantClean: true,
		},
		{
			name:     "posix interactive",
			kind:     shellAuditKindPosix,
			mode:     shellAuditModeInteractive,
			wantArgs: []string{"-i", "-c", "exit"},
		},
		{
			name:     "posix login",
			kind:     shellAuditKindPosix,
			mode:     shellAuditModeLogin,
			wantArgs: []string{"-l", "-i", "-c", "exit"},
		},
		{
			name:      "posix clean",
			kind:      shellAuditKindPosix,
			mode:      shellAuditModeClean,
			wantArgs:  []string{"-i", "-c", "exit"},
			wantEnv:   map[string]string{"ENV": "/dev/null"},
			wantClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, reason := shellAuditStartFor(tt.kind, tt.mode, cleanDir)
			if reason != "" {
				t.Fatalf("%s/%s is not measurable: %s", tt.kind, tt.mode.label(), reason)
			}
			if got, want := strings.Join(start.Args, " "), strings.Join(tt.wantArgs, " "); got != want {
				t.Errorf("args = %q, want %q", got, want)
			}
			if start.Clean != tt.wantClean {
				t.Errorf("the start reports clean = %v, want %v", start.Clean, tt.wantClean)
			}
			for key, want := range tt.wantEnv {
				if got := shellAuditStartEnvValue(start, key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			// A start that is not clean must not set a variable that redirects
			// configuration: measuring the user's shell with XDG_CONFIG_HOME
			// moved would describe a shell nobody has.
			if !tt.wantClean && (shellAuditStartEnvValue(start, "ZDOTDIR") != "" ||
				shellAuditStartEnvValue(start, "XDG_CONFIG_HOME") != "" ||
				shellAuditStartEnvValue(start, "ENV") != "") {
				t.Errorf("the configured start moves the shell's configuration: %v", start.Env)
			}
		})
	}
}

// TestShellAuditStartForRefusesWhatItCannotReproduce pins the refusals. Each one
// is a sentence the screen can print, and the empty-directory refusal is the one
// with teeth: `ZDOTDIR=` or `XDG_CONFIG_HOME=` is not "no configuration", it is
// "look in the home directory", which would make a clean measurement a
// measurement of the user's own files.
func TestShellAuditStartForRefusesWhatItCannotReproduce(t *testing.T) {
	tests := []struct {
		name     string
		kind     shellAuditKind
		mode     shellAuditMode
		cleanDir string
		wantIn   string
	}{
		{
			name:   "an unknown shell's login start",
			kind:   shellAuditKindUnknown,
			mode:   shellAuditModeLogin,
			wantIn: "unknown",
		},
		{
			name:   "an unknown shell's non-interactive start",
			kind:   shellAuditKindUnknown,
			mode:   shellAuditModeNonInteractive,
			wantIn: "unknown",
		},
		{
			// The directory is supplied here on purpose: the refusal under test is
			// the unknown shell's, and the empty-directory refusal is a different
			// one with its own case below.
			name:     "an unknown shell's clean start",
			kind:     shellAuditKindUnknown,
			mode:     shellAuditModeClean,
			cleanDir: "/var/tmp/dotfiles-shell-audit-clean",
			wantIn:   "unknown",
		},
		{
			name:     "an interactive start that would move the user's configuration",
			kind:     shellAuditKindZsh,
			mode:     shellAuditMode{Interactive: true, Clean: true},
			cleanDir: "",
			wantIn:   "directory",
		},
		{
			name:   "zsh clean with no directory to be clean in",
			kind:   shellAuditKindZsh,
			mode:   shellAuditModeClean,
			wantIn: "directory",
		},
		{
			name:   "fish clean with no directory to be clean in",
			kind:   shellAuditKindFish,
			mode:   shellAuditModeClean,
			wantIn: "directory",
		},
		{
			name:   "nushell clean with no directory to be clean in",
			kind:   shellAuditKindNushell,
			mode:   shellAuditModeClean,
			wantIn: "directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, reason := shellAuditStartFor(tt.kind, tt.mode, tt.cleanDir)
			if reason == "" {
				t.Fatalf("%s/%s was accepted where it cannot be reproduced", tt.kind, tt.mode.label())
			}
			if !strings.Contains(reason, tt.wantIn) {
				t.Errorf("the refusal = %q, want it to name %q", reason, tt.wantIn)
			}
		})
	}

	// The other side of the same rule: the one start an unknown shell shares is
	// still offered, so a shell nobody has seen is not refused a total.
	if _, reason := shellAuditStartFor(shellAuditKindUnknown, shellAuditModeInteractive, ""); reason != "" {
		t.Errorf("an unknown shell's interactive start was refused: %s", reason)
	}
}

// TestShellAuditProfilerFollowsTheShell pins which shell gets which instrument,
// and pins that the shell which gets none is told why in a sentence that names
// what is missing. A version below the one an instrument needs is a reason, not
// a failed run.
func TestShellAuditProfilerFollowsTheShell(t *testing.T) {
	tests := []struct {
		name        string
		kind        shellAuditKind
		version     string
		want        shellAuditProfiler
		reasonIn    string
		wantNoProbe bool
	}{
		{name: "zsh with zprof", kind: shellAuditKindZsh, version: "zsh 5.9.2 (x86_64-pc-linux-gnu)", want: shellAuditProfilerZprof},
		{name: "bash 5 with its own trace", kind: shellAuditKindBash, version: "GNU bash, version 5.3.20(1)-release", want: shellAuditProfilerXtrace},
		{name: "bash 4 without nanosecond timestamps", kind: shellAuditKindBash, version: "GNU bash, version 4.4.20(1)-release", reasonIn: "5.0"},
		{name: "a bash that would not name its version", kind: shellAuditKindBash, version: "", want: shellAuditProfilerXtrace},
		{name: "fish 4 with its own profile", kind: shellAuditKindFish, version: "fish, version 4.9.3", want: shellAuditProfilerFishStartup},
		{name: "fish 3 with its own profile", kind: shellAuditKindFish, version: "fish, version 3.6.0", want: shellAuditProfilerFishStartup},
		{name: "fish 2 without a profile", kind: shellAuditKindFish, version: "fish, version 2.7.1", reasonIn: "3"},
		{name: "nushell has none", kind: shellAuditKindNushell, version: "0.116.1", reasonIn: "nushell"},
		{name: "a posix shell has none", kind: shellAuditKindPosix, version: "", reasonIn: "dash"},
		{name: "an unknown shell has none", kind: shellAuditKindUnknown, version: "", reasonIn: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := shellAuditProfilerFor(tt.kind, shellAuditParseVersion(tt.version))
			if got != tt.want {
				t.Fatalf("the instrument = %q, want %q (reason %q)", got, tt.want, reason)
			}
			if tt.reasonIn == "" {
				if reason != "" {
					t.Errorf("an available instrument came with the reason %q", reason)
				}
				return
			}
			if reason == "" {
				t.Fatalf("the shell with no instrument was given no reason")
			}
			if !strings.Contains(reason, tt.reasonIn) {
				t.Errorf("the reason = %q, want it to name %q", reason, tt.reasonIn)
			}
		})
	}
}

// TestShellAuditModesOffersTheFourStarts pins the list the screen cycles: the
// interactive start first, because it is the one the user waits through when
// opening a terminal, and the clean start last, because it is a comparison
// rather than a start anybody gets.
func TestShellAuditModesOffersTheFourStarts(t *testing.T) {
	modes := shellAuditModes()
	if len(modes) != 4 {
		t.Fatalf("the utility offers %d starts, want four: %v", len(modes), modes)
	}
	want := []shellAuditMode{shellAuditModeInteractive, shellAuditModeLogin, shellAuditModeNonInteractive, shellAuditModeClean}
	for i, mode := range want {
		if modes[i] != mode {
			t.Errorf("start %d = %+v, want %+v", i+1, modes[i], mode)
		}
	}
}

// TestShellAuditParseVersionReadsWhatTheShellPrinted pins the parser against the
// real first lines of the four shells' own version commands, taken on this
// machine. The raw line is kept short enough to draw, and a shell that printed
// no version is unknown rather than zero.
func TestShellAuditParseVersionReadsWhatTheShellPrinted(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantNumber string
		wantKnown  bool
		wantMajor  int
		wantMinor  int
	}{
		{name: "zsh", raw: "zsh 5.9.2 (x86_64-pc-linux-gnu)\n", wantNumber: "5.9.2", wantKnown: true, wantMajor: 5, wantMinor: 9},
		{name: "bash", raw: "GNU bash, version 5.3.20(1)-release (x86_64-pc-linux-gnu)\n", wantNumber: "5.3.20", wantKnown: true, wantMajor: 5, wantMinor: 3},
		{name: "fish", raw: "fish, version 4.9.3\n", wantNumber: "4.9.3", wantKnown: true, wantMajor: 4, wantMinor: 9},
		{name: "nushell", raw: "0.116.1\n", wantNumber: "0.116.1", wantKnown: true, wantMajor: 0, wantMinor: 116},
		{name: "a shell whose version command failed", raw: "dash: 0: Illegal option --\n", wantKnown: false},
		{name: "nothing at all", raw: "", wantKnown: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := shellAuditParseVersion(tt.raw)
			if version.Known != tt.wantKnown {
				t.Fatalf("known = %v, want %v (%+v)", version.Known, tt.wantKnown, version)
			}
			if !tt.wantKnown {
				if version.Number != "" || version.Major != 0 || version.Minor != 0 {
					t.Errorf("an unknown version still carries numbers: %+v", version)
				}
				return
			}
			if version.Number != tt.wantNumber {
				t.Errorf("number = %q, want %q", version.Number, tt.wantNumber)
			}
			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor {
				t.Errorf("version = %d.%d, want %d.%d", version.Major, version.Minor, tt.wantMajor, tt.wantMinor)
			}
			if strings.Contains(version.Raw, "\n") {
				t.Errorf("the raw line still holds a newline: %q", version.Raw)
			}
			if len(version.Raw) > shellAuditVersionRawLimit {
				t.Errorf("the raw line is %d bytes, want at most the %d a frame can draw", len(version.Raw), shellAuditVersionRawLimit)
			}
		})
	}
}

// TestShellAuditVersionOfTheStartNamesTheStartItRuns puts the two halves
// together: the command line the screen prints is built from the same arguments
// the run gets, so the line a reader copies cannot drift from the line that ran.
func TestShellAuditVersionOfTheStartNamesTheStartItRuns(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		kind     shellAuditKind
		mode     shellAuditMode
		wantLine string
	}{
		{name: "zsh interactive", shell: "zsh", kind: shellAuditKindZsh, mode: shellAuditModeInteractive, wantLine: "zsh -i -c exit"},
		{name: "zsh login", shell: "zsh", kind: shellAuditKindZsh, mode: shellAuditModeLogin, wantLine: "zsh -l -i -c exit"},
		{name: "zsh clean", shell: "zsh", kind: shellAuditKindZsh, mode: shellAuditModeClean, wantLine: "ZDOTDIR=/tmp/dotfiles-shell-audit zsh -d -f -i -c exit"},
		{name: "fish clean", shell: "fish", kind: shellAuditKindFish, mode: shellAuditModeClean, wantLine: "XDG_CONFIG_HOME=/tmp/dotfiles-shell-audit fish --no-config -i -c exit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, reason := shellAuditStartFor(tt.kind, tt.mode, "/tmp/dotfiles-shell-audit")
			if reason != "" {
				t.Fatalf("the start is not measurable: %s", reason)
			}
			if got := shellAuditCommandLine(tt.shell, start); got != tt.wantLine {
				t.Errorf("the line on screen = %q, want %q", got, tt.wantLine)
			}
		})
	}
}
