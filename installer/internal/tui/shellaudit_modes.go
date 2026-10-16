package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================================
// WHICH SHELL, WHICH START, WHICH INSTRUMENT
// ============================================================================
//
// "Measure the shell's startup" is four different measurements. A shell that is
// opened as a terminal is interactive and not a login shell; the shell a
// terminal emulator is started as is a login shell; `sh -c` is neither; and a
// start with none of the user's files is a fifth thing that describes nobody's
// terminal and exists to be compared against. A utility that runs one command
// and calls it "the startup" is measuring one of the four and reporting it as
// all of them.
//
// Each family spells its starts differently, and the difference is not
// cosmetic. zsh skips its own files with `-d -f`; bash with `--noprofile
// --norc`; fish with `--no-config`; nushell with `--no-config-file`; and a
// POSIX shell has no flag at all and has to be pointed at an empty $ENV. The
// matrix below is that knowledge and nothing else: it decides which starts are
// offered, what each one runs, and which instrument the shell itself has. It
// starts nothing.
//
// A shell the utility has never seen is not guessed at. It gets the one start
// every shell that answers `-c` shares, and a sentence for every other start.

// shellAuditKind is the family a shell belongs to. The family, not the binary:
// `/bin/sh` and `dash` are the same startup, and a user's zsh installed under
// `~/.local/bin` is the same startup as the one in `/usr/bin`.
type shellAuditKind string

const (
	shellAuditKindZsh     shellAuditKind = "zsh"
	shellAuditKindBash    shellAuditKind = "bash"
	shellAuditKindFish    shellAuditKind = "fish"
	shellAuditKindNushell shellAuditKind = "nushell"
	shellAuditKindPosix   shellAuditKind = "sh"
	shellAuditKindUnknown shellAuditKind = "an unknown shell"
)

// shellAuditKindOf names the family from the executable's name: a full path, a
// Windows `.exe` and an upper-case name are all the same binary to this
// function, so `$SHELL` can be used as it is.
func shellAuditKindOf(name string) shellAuditKind {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "zsh":
		return shellAuditKindZsh
	case "bash":
		return shellAuditKindBash
	case "fish":
		return shellAuditKindFish
	case "nu", "nushell":
		return shellAuditKindNushell
	case "sh", "dash", "ash", "ksh", "ksh93", "mksh", "pdksh", "yash", "busybox":
		// The POSIX family: one startup semantics, no profiler, and an $ENV
		// variable as the only way to point it at nothing.
		return shellAuditKindPosix
	default:
		return shellAuditKindUnknown
	}
}

// shellAuditMode is one way a start happens: whether the shell is interactive,
// whether it is a login shell, and whether it reads the user's own
// configuration. The three booleans are the whole of what a reader of the
// screen needs to know about the number beside them.
//
// The zero value is the non-interactive start, because that is the start with
// none of the three: it is the one `sh -c` makes, and a mode built by hand that
// forgot to choose cannot silently claim to be the interactive one.
type shellAuditMode struct {
	Interactive bool
	Login       bool
	Clean       bool
}

// The four starts the utility offers. Interactive first, because it is the one
// the user waits through when a terminal opens, and the clean start last,
// because it is a comparison rather than a start anybody gets.
var (
	shellAuditModeInteractive    = shellAuditMode{Interactive: true}
	shellAuditModeLogin          = shellAuditMode{Interactive: true, Login: true}
	shellAuditModeNonInteractive = shellAuditMode{}
	shellAuditModeClean          = shellAuditMode{Interactive: true, Clean: true}
)

// shellAuditModes is the list the screen cycles, in the order a reader wants
// them: what a terminal opens with, then the login shell, then the plain
// command run, then the comparison.
func shellAuditModes() []shellAuditMode {
	return []shellAuditMode{
		shellAuditModeInteractive,
		shellAuditModeLogin,
		shellAuditModeNonInteractive,
		shellAuditModeClean,
	}
}

// label is the phrase the screen draws for the mode, and the one the description
// uses to explain what the number describes.
func (m shellAuditMode) label() string {
	parts := make([]string, 0, 3)
	if m.Login {
		parts = append(parts, "login")
	}
	if m.Interactive {
		parts = append(parts, "interactive")
	} else {
		parts = append(parts, "non-interactive")
	}
	if m.Clean {
		parts = append(parts, "no configuration")
	}
	return strings.Join(parts, ", ")
}

// key is the stable name the history groups measurements by. It changes only
// when the start changes, so two measurements of the same start are comparable
// across versions of this utility.
func (m shellAuditMode) key() string {
	base := "non-interactive"
	switch {
	case m.Login:
		base = "login"
	case m.Interactive:
		base = "interactive"
	}
	if m.Clean {
		return base + "-clean"
	}
	return base
}

// shellAuditStart is how one shell family is started in one mode: the arguments,
// the environment it needs on top of the process's own, and whether it is the
// start that reads none of the user's files.
type shellAuditStart struct {
	// Args is the complete argument list, including the `-c exit` that makes the
	// start end by itself.
	Args []string
	// Env is added to the environment the utility already has. Variables named
	// here replace theirs, so a start that points a shell at an empty directory
	// cannot end up with two answers.
	Env []string
	// Clean reports that this start reads none of the user's own configuration.
	Clean bool
	// Note names what a clean start still includes, where the shell offers no way
	// to leave it out. It is empty when the start is clean in full.
	Note string
}

// shellAuditStartFor answers how to start one shell family in one mode, or why
// that start cannot be reproduced here.
//
// cleanDir is a directory of the utility's own, already empty, that the clean
// starts point the shell at. It is required for every clean start and the
// refusal is the one with teeth: an empty ZDOTDIR or XDG_CONFIG_HOME does not
// mean "no configuration" to a shell, it means the home directory, so a clean
// measurement with no directory would quietly be a measurement of the user's
// own files.
func shellAuditStartFor(kind shellAuditKind, mode shellAuditMode, cleanDir string) (shellAuditStart, string) {
	if mode.Clean && cleanDir == "" {
		return shellAuditStart{}, "a clean start needs a directory of the utility's own to be pointed at, and none " +
			"was prepared, so nothing is measurable: an empty ZDOTDIR or XDG_CONFIG_HOME is not \"no configuration\" " +
			"to a shell, it is the home directory, and the run would measure the user's own files."
	}

	if kind == shellAuditKindUnknown {
		if mode.Login || mode.Clean || !mode.Interactive {
			return shellAuditStart{}, "this is an unknown shell: the utility has not seen it and does not guess at its " +
				"startup flags, so only the interactive start (`-i -c exit`) is offered, which every shell that answers " +
				"`-c` accepts. The other starts are left out rather than run with a flag that may mean something else."
		}
		return shellAuditStart{Args: []string{"-i", "-c", "exit"}}, ""
	}

	start := shellAuditStart{Clean: mode.Clean}

	// The flags that keep a start away from the user's configuration, in the
	// order the shell's own documentation lists them.
	switch kind {
	case shellAuditKindZsh:
		if mode.Clean {
			// -d skips the global rc files and -f the user's own; ZDOTDIR below
			// points the search at the utility's empty directory as well, so a
			// shell that re-enables rc files during startup still finds nothing.
			start.Args = append(start.Args, "-d", "-f")
			start.Env = append(start.Env, "ZDOTDIR="+cleanDir)
		}
	case shellAuditKindBash:
		if mode.Clean {
			start.Args = append(start.Args, "--noprofile", "--norc")
			// A non-interactive bash reads $BASH_ENV and a POSIX one $ENV; both are
			// pointed at nothing so a clean run has no file to read whichever mode
			// it is in.
			start.Env = append(start.Env, "BASH_ENV=/dev/null", "ENV=/dev/null")
		}
	case shellAuditKindFish:
		if mode.Clean {
			start.Args = append(start.Args, "--no-config")
			start.Env = append(start.Env, "XDG_CONFIG_HOME="+cleanDir)
		}
	case shellAuditKindNushell:
		if mode.Clean {
			start.Args = append(start.Args, "--no-config-file")
			start.Env = append(start.Env, "XDG_CONFIG_HOME="+cleanDir)
		}
	case shellAuditKindPosix:
		if mode.Clean {
			start.Env = append(start.Env, "ENV=/dev/null")
			// A POSIX shell has no flag for the system profile: a login shell reads
			// /etc/profile because that is what a login shell is.
			if mode.Login {
				start.Note = "a POSIX login shell reads /etc/profile, and there is no flag that skips it, so the " +
					"system profile is still part of this measurement."
			}
		}
	}

	if mode.Login {
		start.Args = append(start.Args, "-l")
	}
	if mode.Interactive {
		start.Args = append(start.Args, "-i")
	}
	// The command is last and is the same one everywhere: the start ends by
	// itself, so a measurement can never wait on a prompt.
	start.Args = append(start.Args, "-c", "exit")

	return start, ""
}

// shellAuditCommandLine is the line the screen prints and the line that runs. It
// is built from the same arguments the run gets, so a reader who copies it
// reproduces the number instead of something near it.
func shellAuditCommandLine(shell string, start shellAuditStart) string {
	parts := make([]string, 0, len(start.Env)+len(start.Args)+1)
	parts = append(parts, start.Env...)
	parts = append(parts, shell)
	parts = append(parts, start.Args...)
	return strings.Join(parts, " ")
}

// shellAuditVersionRawLimit is how much of a shell's own version line is kept.
// The line is drawn in the test's context; a shell that printed a paragraph does
// not get to push the frame off the screen.
const shellAuditVersionRawLimit = 80

// shellAuditVersion is what a shell said about itself. A shell whose version
// could not be read is unknown rather than zero: `dash --version` exits with a
// usage error, and a zero would read as "version 0" in a comparison.
type shellAuditVersion struct {
	// Raw is the first line the shell printed, trimmed to what a frame can draw.
	Raw string
	// Number is the version token itself, Major and Minor its first two parts.
	Number string
	Major  int
	Minor  int
	// Known reports that a version token was found at all.
	Known bool
}

// shellAuditVersionPattern matches the version token in a shell's own version
// output: `zsh 5.9.2 (x86_64-pc-linux-gnu)`, `GNU bash, version 5.3.20(1)`,
// `fish, version 4.9.3` and nushell's bare `0.116.1` all hold one. At least one
// dot is required, because the alternative is a false version: `dash --version`
// exits with the usage error `dash: 0: Illegal option --`, and a pattern that
// took the bare 0 would report that shell as version 0.0.
var shellAuditVersionPattern = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)

// shellAuditParseVersion reads the version a shell printed. It keeps the first
// line only, because every shell here prints its version first and the rest is
// copyright text, and it looks for the first dotted number rather than parsing
// four formats.
func shellAuditParseVersion(raw string) shellAuditVersion {
	line := strings.TrimSpace(raw)
	if index := strings.IndexAny(line, "\r\n"); index >= 0 {
		line = strings.TrimSpace(line[:index])
	}

	version := shellAuditVersion{Raw: shellAuditTruncate(line, shellAuditVersionRawLimit)}

	number := shellAuditVersionPattern.FindString(line)
	if number == "" {
		return version
	}
	version.Number = number
	version.Known = true

	parts := strings.Split(number, ".")
	if major, err := strconv.Atoi(parts[0]); err == nil {
		version.Major = major
	}
	if len(parts) > 1 {
		if minor, err := strconv.Atoi(parts[1]); err == nil {
			version.Minor = minor
		}
	}
	return version
}

// shellAuditTruncate cuts a line to at most limit bytes without splitting a
// multi-byte character, and marks that it cut.
func shellAuditTruncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	runes := []rune(s)
	for len(string(runes)) > limit && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRight(string(runes), " ") + "…"
}

// shellAuditProfiler names the instrument a shell carries for its own startup.
// The name is the shell's own: `zsh/zprof` is a zsh module, `bash/xtrace` is
// bash's own trace, and `fish/--profile-startup` is fish's own flag.
type shellAuditProfiler string

const (
	shellAuditProfilerZprof       shellAuditProfiler = "zsh/zprof"
	shellAuditProfilerXtrace      shellAuditProfiler = "bash/xtrace"
	shellAuditProfilerFishStartup shellAuditProfiler = "fish/--profile-startup"
	// shellAuditProfilerNone means the shell carries no instrument the utility
	// can stand behind, and the returned reason says why.
	shellAuditProfilerNone shellAuditProfiler = ""
)

// shellAuditProfilerFor answers which instrument a shell has, or why it has
// none here. The reason is a sentence the screen prints in place of the
// attribution, so a shell that cannot be profiled is explained rather than left
// with a blank where a culprit would be.
//
// The version matters for two of the four. bash cannot timestamp a traced
// command before 5.0, and fish's `--profile` deliberately starts after the
// configuration has been read -- the flag that profiles the startup itself is
// `--profile-startup`, which arrived in fish 3.6. Using `--profile` on fish
// would be worse than using nothing: it would produce a table that answers a
// different question.
func shellAuditProfilerFor(kind shellAuditKind, version shellAuditVersion) (shellAuditProfiler, string) {
	switch kind {
	case shellAuditKindZsh:
		return shellAuditProfilerZprof, ""

	case shellAuditKindBash:
		if version.Known && version.Major < 5 {
			return shellAuditProfilerNone, fmt.Sprintf("bash's own trace can be told when each command ran only "+
				"through EPOCHREALTIME, which arrived in bash 5.0, and this bash reports %s: on this shell only the "+
				"total above is measurable, and no function is guessed at in its place.", version.Number)
		}
		// A bash whose version would not be read is still tried: the trace either
		// carries its timestamps or it does not, and the parser names nothing when
		// it does not.
		return shellAuditProfilerXtrace, ""

	case shellAuditKindFish:
		if !version.Known || version.Major > 3 || (version.Major == 3 && version.Minor >= 6) {
			return shellAuditProfilerFishStartup, ""
		}
		return shellAuditProfilerNone, fmt.Sprintf("fish's --profile begins after the configuration has been read, "+
			"and the flag that profiles the startup itself (--profile-startup) arrived in fish 3.6; this fish reports "+
			"%s, so a table from it would answer a different question and none is shown: only the total above is "+
			"measurable.", version.Number)

	case shellAuditKindNushell:
		return shellAuditProfilerNone, "nushell ships no profiler for its own startup, so nothing inside it can be " +
			"named: only the total above is measurable, and no plugin is guessed at in its place."

	case shellAuditKindPosix:
		return shellAuditProfilerNone, "the POSIX shells (sh, dash, ash, ksh) ship no profiler of their own, so " +
			"nothing inside the startup can be named: only the total above is measurable."

	default:
		return shellAuditProfilerNone, "this is an unknown shell, so the utility cannot say which profiler it has and " +
			"does not guess: only the total above is measurable, and nothing is named in its place."
	}
}
