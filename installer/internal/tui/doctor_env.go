package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE ENVIRONMENT AND THE PLATFORM
// ============================================================================
//
// The faults here are the ones that break a run before any tool is reached: a
// home directory that is not there, an XDG variable the tools will ignore, a
// temporary directory nothing can be written to, a disk with no room, a locale
// that mangles the text, a terminal whose terminfo entry is missing, and the
// installer's own environment variables set to values that mean nothing.
//
// The last check is the platform one, and it is the reason this file exists as
// its own category: WSL, Termux and macOS each have faults the others cannot
// have, and a check that reported them on the wrong platform would be the
// confusion between the Windows host and the distribution that the brief names.
// Every one of them therefore answers not-applicable, with the reason, on any
// other platform.

// doctorEnvironmentChecks is the environment category's catalogue.
func doctorEnvironmentChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "env.home",
			Category: doctorCategoryEnvironment,
			Title:    "The home directory is there and writable",
			Required: true,
			Run:      doctorCheckHome,
		},
		{
			ID:       "env.xdg",
			Category: doctorCategoryEnvironment,
			Title:    "The XDG base directories name directories this user can use",
			Run:      doctorCheckXDGDirs,
		},
		{
			ID:       "env.temp",
			Category: doctorCategoryEnvironment,
			Title:    "A temporary directory can be written to",
			Required: true,
			Run:      doctorCheckTempDir,
		},
		{
			ID:       "env.capacity",
			Category: doctorCategoryEnvironment,
			Title:    "There is room for the files a run writes, and enough descriptors",
			Run:      doctorCheckCapacity,
		},
		{
			ID:       "env.locale",
			Category: doctorCategoryEnvironment,
			Title:    "The locale is one the interface and git can print",
			Run:      doctorCheckLocale,
		},
		{
			ID:       "env.term",
			Category: doctorCategoryEnvironment,
			Title:    "The terminal this runs in is one terminfo knows",
			Run:      doctorCheckTerminfo,
		},
		{
			ID:       "env.vars",
			Category: doctorCategoryEnvironment,
			Title:    "The installer's own variables hold values that mean something",
			Run:      doctorCheckInstallerVariables,
		},
		{
			ID:       "env.platform",
			Category: doctorCategoryEnvironment,
			Title:    "The platform-specific faults this environment can have",
			Run:      doctorCheckPlatformSpecifics,
		},
	}
}

// doctorCheckHome answers the one directory everything else is relative to.
func doctorCheckHome(scan *doctorScan) []doctorFinding {
	home := scan.home()
	if home == "" {
		return []doctorFinding{{
			Subject:  "The home directory",
			Status:   doctorError,
			Evidence: "neither $HOME nor the platform's user lookup names a home directory",
			Impact:   "Every path this installer writes is under the home directory, so no step can write anything.",
			Advice:   "Set HOME in the environment that starts the installer, or start it as the user that owns the account.",
		}}
	}

	info, err := scan.stat(home)
	switch {
	case err != nil:
		return []doctorFinding{{
			Subject:  "The home directory",
			Status:   doctorError,
			Evidence: fmt.Sprintf("HOME names %s, which cannot be read: %v", home, err),
			Impact:   "Every path this installer writes is under it, so every step fails at the first write.",
			Advice:   "Create the directory and give it to this user, or correct HOME.",
		}}
	case !info.IsDir():
		return []doctorFinding{{
			Subject:  "The home directory",
			Status:   doctorError,
			Evidence: fmt.Sprintf("HOME names %s, which is a file (mode %s)", home, doctorPermString(info.Mode())),
			Impact:   "Nothing can be written under a path that is not a directory.",
			Advice:   "Correct HOME to the directory this account owns.",
		}}
	case !doctorWritable(scan, home):
		return []doctorFinding{{
			Subject:  "The home directory",
			Status:   doctorError,
			Evidence: fmt.Sprintf("%s has mode %s: its owner cannot write to it", home, doctorPermString(info.Mode())),
			Impact:   "No configuration can be installed, no backup can be taken and no state can be recorded.",
			Advice:   "Give the owner the write bit: `chmod u+w " + home + "`.",
		}}
	}
	return []doctorFinding{{
		Subject:  "The home directory",
		Status:   doctorOK,
		Evidence: fmt.Sprintf("%s is a directory with mode %s", home, doctorPermString(info.Mode())),
		Impact:   "The steps that write configuration have a place to write it.",
		Advice:   "Nothing to do.",
	}}
}

// doctorCheckXDGDirs reads the four XDG base-directory variables. The rule is
// the specification's own: a value that is not an absolute path is ignored by
// the tools that follow it, so the tools write somewhere else than the user
// thinks -- which is a fault worth naming even though nothing is broken.
func doctorCheckXDGDirs(scan *doctorScan) []doctorFinding {
	variables := []struct {
		name string
		what string
	}{
		{"XDG_CONFIG_HOME", "configuration"},
		{"XDG_DATA_HOME", "data this installer copies for the utilities"},
		{"XDG_STATE_HOME", "the record of the last installation and this report"},
		{"XDG_CACHE_HOME", "caches"},
	}

	var findings []doctorFinding
	var clear []string
	for _, variable := range variables {
		// The documented fallback is resolved once, by the scan, and rendered the
		// way the user recognises it: the advice below still spells the absolute
		// path, because ~ is exactly what the rule being reported rejects.
		documented := doctorRelHome(scan, scan.xdgFallback(variable.name))
		value := scan.getenv(variable.name)
		if value == "" {
			clear = append(clear, fmt.Sprintf("%s is unset, so %s is used", variable.name, documented))
			continue
		}
		if !filepath.IsAbs(value) {
			findings = append(findings, doctorFinding{
				Subject:  variable.name,
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("%s=%s is not an absolute path", variable.name, value),
				Impact:   fmt.Sprintf("Every tool that follows the XDG specification ignores a relative value, so %s is written under the home directory instead of where this says.", variable.what),
				Advice:   fmt.Sprintf("Make it absolute (`%s=%s`), or unset it to use the documented fallback.", variable.name, scan.xdgFallback(variable.name)),
			})
			continue
		}
		info, err := scan.stat(value)
		switch {
		case err != nil:
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", variable.name, doctorRelHome(scan, value)),
				Status:   doctorWarning,
				Evidence: "the directory does not exist, so the specification says " + documented + " is not used either",
				Impact:   fmt.Sprintf("Tools that honour the variable fail to write %s until the directory exists.", variable.what),
				Advice:   "Create the directory with `mkdir -p " + value + "`, or let the doctor do it.",
				Fix:      &doctorFix{Kind: doctorFixMakeDir, Path: value},
			})
		case !info.IsDir():
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", variable.name, doctorRelHome(scan, value)),
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("the path is a file (mode %s)", doctorPermString(info.Mode())),
				Impact:   fmt.Sprintf("Tools that honour the variable cannot write %s.", variable.what),
				Advice:   "Point the variable at a directory, or move the file aside.",
			})
		case !doctorWritable(scan, value):
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", variable.name, doctorRelHome(scan, value)),
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("the directory has mode %s: its owner cannot write to it", doctorPermString(info.Mode())),
				Impact:   fmt.Sprintf("Tools that honour the variable cannot write %s there.", variable.what),
				Advice:   "Give the owner the write bit: `chmod u+w " + value + "`.",
			})
		default:
			clear = append(clear, fmt.Sprintf("%s=%s", variable.name, doctorRelHome(scan, value)))
		}
	}

	if len(findings) == 0 {
		return []doctorFinding{{
			Subject:  "The XDG base directories",
			Status:   doctorOK,
			Evidence: doctorCounted(len(clear), "variable", "variables") + " usable: " + doctorPathList(clear, 4),
			Impact:   "The tools that follow the specification and this installer's own record agree about where files go.",
			Advice:   "Nothing to do.",
		}}
	}
	return findings
}

// doctorCheckTempDir answers where a temporary file would go, and whether it can
// be written. Every check that runs a command and every step that unpacks an
// archive needs one.
func doctorCheckTempDir(scan *doctorScan) []doctorFinding {
	type candidate struct {
		name  string
		value string
	}
	var candidates []candidate
	for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
		if value := scan.getenv(name); value != "" {
			candidates = append(candidates, candidate{name: name, value: value})
		}
	}
	if len(candidates) == 0 {
		candidates = append(candidates, candidate{name: "(default)", value: "/tmp"})
	}

	var findings []doctorFinding
	for _, option := range candidates {
		info, err := scan.stat(option.value)
		switch {
		case err != nil:
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", option.name, option.value),
				Status:   doctorError,
				Evidence: "the directory does not exist: " + err.Error(),
				Impact:   "Anything that writes a temporary file fails: unpacking an archive, a step's own scratch space, and any tool that uses the directory this variable names.",
				Advice:   "Create it with `mkdir -p " + option.value + "`, unset the variable to use the default, or let the doctor do it.",
				Fix:      &doctorFix{Kind: doctorFixMakeDir, Path: option.value},
			})
			continue
		case !info.IsDir():
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", option.name, option.value),
				Status:   doctorError,
				Evidence: fmt.Sprintf("the path is a file (mode %s)", doctorPermString(info.Mode())),
				Impact:   "Nothing can be written to a temporary path that is not a directory.",
				Advice:   "Point the variable at a directory, or unset it to use the default.",
			})
			continue
		case !doctorWritable(scan, option.value):
			findings = append(findings, doctorFinding{
				Subject:  fmt.Sprintf("%s=%s", option.name, option.value),
				Status:   doctorError,
				Evidence: fmt.Sprintf("the directory has mode %s: its owner cannot write to it", doctorPermString(info.Mode())),
				Impact:   "Anything that writes a temporary file fails, which is most of the steps.",
				Advice:   "Give the owner the write bit, or unset the variable so the default is used.",
			})
			continue
		}
		findings = append(findings, doctorFinding{
			Subject:  fmt.Sprintf("The temporary directory (%s)", option.name),
			Status:   doctorOK,
			Evidence: fmt.Sprintf("%s is a directory with mode %s", option.value, doctorPermString(info.Mode())),
			Impact:   "Temporary files can be written.",
			Advice:   "Nothing to do.",
		})
	}
	return findings
}

// doctorFilesystemSpace is how much room one path's filesystem has.
type doctorFilesystemSpace struct {
	// AvailableKB is the space a non-root user may use.
	AvailableKB int64
	// Source is how the number was obtained, so the report can be reproduced.
	Source string
	// Err is why there is no number.
	Err string
}

// doctorSpaceFor answers how much room is left on the filesystem a path is on.
// It asks df, which every platform this installer supports ships, and reports
// unknown where it does not: there is no portable way to ask the kernel from
// this process on every platform, and inventing a number would be worse than
// saying nothing.
func doctorSpaceFor(scan *doctorScan, path string) doctorFilesystemSpace {
	lookup := scan.lookup("df")
	if !lookup.found() {
		return doctorFilesystemSpace{Err: "df does not resolve on PATH, so the free space cannot be read here"}
	}
	result := scan.probe(doctorProbeTimeout, lookup.Path, "-Pk", path)
	if result.TimedOut {
		return doctorFilesystemSpace{Err: fmt.Sprintf("`df -Pk %s` did not finish inside %s", path, doctorDuration(doctorProbeTimeout))}
	}
	if result.Err != "" {
		return doctorFilesystemSpace{Err: fmt.Sprintf("`df -Pk %s` failed: %s", path, result.Err)}
	}

	// POSIX output is one header line and one line per filesystem:
	// "Filesystem 1024-blocks Used Available Capacity Mounted-on". The last line
	// is the one for the path that was asked about, and the fourth field is what
	// is available.
	lines := doctorNonEmptyLines(result.Stdout)
	if len(lines) < 2 {
		return doctorFilesystemSpace{Err: fmt.Sprintf("`df -Pk %s` printed no data line", path)}
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return doctorFilesystemSpace{Err: fmt.Sprintf("`df -Pk %s` printed a line the doctor does not understand: %q", path, lines[len(lines)-1])}
	}
	available, ok := doctorParseInt(fields[3])
	if !ok {
		return doctorFilesystemSpace{Err: fmt.Sprintf("`df -Pk %s` printed %q where a number belongs", path, fields[3])}
	}
	return doctorFilesystemSpace{AvailableKB: int64(available), Source: fmt.Sprintf("df -Pk %s → %s KB available", path, fields[3])}
}

// Capacity thresholds. They are floors for a run that downloads a toolset and
// unpacks a font archive, not for a whole workstation; the wording of the
// findings says so rather than claiming a machine is full.
const (
	doctorCapacityWarnKB  = 1024 * 1024 // 1 GiB
	doctorCapacityErrorKB = 256 * 1024  // 256 MiB
	doctorDescriptorsWarn = 256
)

// doctorCheckCapacity reports the room a run has and the descriptors it may
// open. Both answers are stated with their source, and a platform where neither
// can be read says unknown rather than guessing.
func doctorCheckCapacity(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding
	home := scan.home()
	if home != "" {
		space := doctorSpaceFor(scan, home)
		switch {
		case space.Err != "":
			findings = append(findings, doctorFinding{
				Subject:  "Room under the home directory",
				Status:   doctorUnknown,
				Evidence: space.Err,
				Impact:   "Whether a run has room for the tools and fonts it downloads is unknown.",
				Advice:   "Check by hand with `df -h " + home + "`.",
			})
		case space.AvailableKB < doctorCapacityErrorKB:
			findings = append(findings, doctorFinding{
				Subject:  "Room under the home directory",
				Status:   doctorError,
				Evidence: fmt.Sprintf("%s (%d MB available)", space.Source, space.AvailableKB/1024),
				Impact:   "A run downloads a toolset and unpacks archives; a few hundred megabytes is not enough for it to finish.",
				Advice:   "Free space under " + home + " before installing, or install only the parts you need.",
			})
		case space.AvailableKB < doctorCapacityWarnKB:
			findings = append(findings, doctorFinding{
				Subject:  "Room under the home directory",
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("%s (%d MB available)", space.Source, space.AvailableKB/1024),
				Impact:   "A full run downloads a toolset, fonts and plugins; a gigabyte is enough for one, but not for a large toolset with room to spare.",
				Advice:   "Free some space, or install a smaller selection.",
			})
		default:
			findings = append(findings, doctorFinding{
				Subject:  "Room under the home directory",
				Status:   doctorOK,
				Evidence: fmt.Sprintf("%s (%d MB available)", space.Source, space.AvailableKB/1024),
				Impact:   "There is room for a full run.",
				Advice:   "Nothing to do.",
			})
		}
	}

	// The descriptor limit is the other capacity that stops a run: a toolset of
	// plugins that each open a file hits it before the disk does.
	if strings.HasPrefix(scan.goos(), "windows") {
		findings = append(findings, doctorFinding{
			Subject:  "Open files per process",
			Status:   doctorNotApplicable,
			Evidence: "this platform does not have the POSIX limit the doctor reads",
			Impact:   "There is nothing to read.",
			Advice:   "Nothing to do.",
		})
		return findings
	}
	shell := scan.lookup("sh")
	if !shell.found() {
		findings = append(findings, doctorFinding{
			Subject:  "Open files per process",
			Status:   doctorUnknown,
			Evidence: "sh does not resolve on PATH, so `ulimit -n` cannot be asked",
			Impact:   "Whether a plugin-heavy tool would hit the limit is unknown.",
			Advice:   "Check by hand in a shell with `ulimit -n`.",
		})
		return findings
	}
	result := scan.probe(doctorProbeTimeout, shell.Path, "-c", "ulimit -n")
	if result.Err != "" {
		findings = append(findings, doctorFinding{
			Subject:  "Open files per process",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("`ulimit -n` failed: %s", result.Err),
			Impact:   "Whether a plugin-heavy tool would hit the limit is unknown.",
			Advice:   "Check by hand in a shell with `ulimit -n`.",
		})
		return findings
	}
	limit, ok := doctorParseInt(doctorFirstLine(result.Stdout))
	if !ok {
		findings = append(findings, doctorFinding{
			Subject:  "Open files per process",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("`ulimit -n` printed %q, which is not a number", doctorFirstLine(result.Stdout)),
			Impact:   "Whether a plugin-heavy tool would hit the limit is unknown.",
			Advice:   "Check by hand in a shell with `ulimit -n`.",
		})
		return findings
	}
	status := doctorOK
	impact := "A shell and its plugins can open their files."
	advice := "Nothing to do."
	if limit < doctorDescriptorsWarn {
		status = doctorWarning
		impact = "A tool with many plugins (Neovim with LazyVim, a shell with several) can hit this limit and fail to open a file, which appears as an unrelated error."
		advice = "Raise it for the shell you use, for example `ulimit -n 4096` in the shell's own startup file, or through the system's limits configuration."
	}
	findings = append(findings, doctorFinding{
		Subject:  "Open files per process",
		Status:   status,
		Evidence: fmt.Sprintf("`ulimit -n` in sh reports %d", limit),
		Impact:   impact,
		Advice:   advice,
	})
	return findings
}

// doctorCheckLocale answers whether the machine's text will print. A locale that
// is not UTF-8 mangles the box glyphs the interface draws and the accents in a
// commit message; a locale that is not installed at all makes programs fall back
// to the C locale, which prints question marks.
func doctorCheckLocale(scan *doctorScan) []doctorFinding {
	lang := scan.getenv("LANG")
	lcAll := scan.getenv("LC_ALL")
	lcCtype := scan.getenv("LC_CTYPE")

	var findings []doctorFinding
	utf8 := strings.Contains(strings.ToUpper(lang+lcAll+lcCtype), "UTF")

	switch {
	case lang == "" && lcAll == "" && lcCtype == "":
		findings = append(findings, doctorFinding{
			Subject:  "The locale",
			Status:   doctorWarning,
			Evidence: "LANG, LC_ALL and LC_CTYPE are all unset",
			Impact:   "Programs use the C locale: the interface's own glyphs, accented characters and any non-ASCII output can be printed as question marks or boxes.",
			Advice:   "Set a UTF-8 locale where the shell starts, for example `export LANG=en_US.UTF-8` (the exact name is in `locale -a`).",
		})
	case !utf8:
		findings = append(findings, doctorFinding{
			Subject:  "The locale",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s and %s name a locale with no UTF-8 in it", doctorEnvEntry(scan, "LANG"), doctorEnvEntry(scan, "LC_ALL")),
			Impact:   "Text outside ASCII is printed as question marks, which corrupts paths with accents and commit messages.",
			Advice:   "Use a UTF-8 locale, for example `export LANG=en_US.UTF-8`, or the `.UTF-8` variant of the locale already set.",
		})
	case lcAll != "" && lcAll != lang:
		findings = append(findings, doctorFinding{
			Subject:  "The locale",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s overrides %s", doctorEnvEntry(scan, "LC_ALL"), doctorEnvEntry(scan, "LANG")),
			Impact:   "LC_ALL wins over LANG and every category-specific variable, so a tool that asks for a different collation or character class cannot get it.",
			Advice:   "Unset LC_ALL unless it was deliberate: `unset LC_ALL` in the shell's startup file.",
		})
	case lcCtype != "" && lcCtype != lang && !strings.Contains(strings.ToUpper(lcCtype), "UTF"):
		findings = append(findings, doctorFinding{
			Subject:  "The locale",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s overrides %s for character handling", doctorEnvEntry(scan, "LC_CTYPE"), doctorEnvEntry(scan, "LANG")),
			Impact:   "LC_CTYPE decides how characters are classified, and a locale with no UTF-8 in it classifies every non-ASCII byte on its own: accented paths and commit messages are printed and sorted wrongly.",
			Advice:   "Unset LC_CTYPE to let LANG decide, or set it to a UTF-8 locale.",
		})
	default:
		findings = append(findings, doctorFinding{
			Subject:  "The locale",
			Status:   doctorOK,
			Evidence: doctorEnvEntry(scan, "LANG") + ", " + doctorEnvEntry(scan, "LC_ALL"),
			Impact:   "Programs print UTF-8 and agree about the locale.",
			Advice:   "Nothing to do.",
		})
	}
	return findings
}

// doctorCheckTerminfo asks whether this terminal is one terminfo knows, using
// infocmp when it is installed. The terminal's own name is passed as an argument
// and never interpolated into a shell command.
func doctorCheckTerminfo(scan *doctorScan) []doctorFinding {
	term := scan.getenv("TERM")
	if term == "" {
		return []doctorFinding{{
			Subject:  "The terminal's terminfo entry",
			Status:   doctorWarning,
			Evidence: "TERM is unset",
			Impact:   "Programs cannot know what this terminal can do: the interface falls back to plain output, and the terminal capability report in this same section has nothing to describe.",
			Advice:   "Set TERM in the environment that starts the installer, or start it from a terminal, which sets it.",
		}}
	}
	if term == "dumb" {
		return []doctorFinding{{
			Subject:  "The terminal's terminfo entry",
			Status:   doctorWarning,
			Evidence: "TERM=dumb",
			Impact:   "The terminal claims no capabilities at all, so colours, the alternate screen and the interface's cursor handling are disabled.",
			Advice:   "Run the installer from a real terminal, or set TERM to the terminal you are actually using.",
		}}
	}

	lookup := scan.lookup("infocmp")
	if !lookup.found() {
		return []doctorFinding{{
			Subject:  "The terminal's terminfo entry",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("TERM=%s and infocmp does not resolve on PATH, so the entry cannot be looked up", term),
			Impact:   "Whether this terminal has a description on the machine is unknown.",
			Advice:   fmt.Sprintf("Check by hand with `infocmp %s`, or install ncurses (which ships infocmp).", term),
		}}
	}
	result := scan.probe(doctorProbeTimeout, lookup.Path, term)
	if result.Err == "" {
		return []doctorFinding{{
			Subject:  "The terminal's terminfo entry",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("TERM=%s and `infocmp %s` found its description", term, term),
			Impact:   "Programs can look up what this terminal can do.",
			Advice:   "Nothing to do.",
		}}
	}
	return []doctorFinding{{
		Subject:  "The terminal's terminfo entry",
		Status:   doctorWarning,
		Evidence: fmt.Sprintf("TERM=%s and `infocmp %s` failed: %s", term, term, doctorFirstLine(result.Stderr)),
		Impact:   "Programs cannot find a description for this terminal, so keys like the arrows and the function keys may not work and the interface can draw wrongly.",
		Advice:   fmt.Sprintf("Set TERM to a description the machine has (usually `xterm-256color`), or install the terminfo entry for %s.", term),
	}}
}

// doctorCheckInstallerVariables reads the variables this installer's own steps
// and utilities read, and reports the values that mean nothing: a directory that
// is not a directory, a switch whose value is neither on nor off.
//
// Nothing here is a secret, and the values are printed: a path is what makes a
// finding actionable. A variable whose name says otherwise is redacted by
// doctorEnvEntry.
func doctorCheckInstallerVariables(scan *doctorScan) []doctorFinding {
	type switchVariable struct {
		name    string
		effect  string
		onTrue  string
		onFalse string
	}
	switches := []switchVariable{
		{"DOTFILES_ANIM", "the animations in the interface", "the animation draws", "the interface draws without animation"},
		{"DOTFILES_MOUSE", "mouse tracking", "the pointer is tracked and the terminal's own selection is given up", "the pointer is left to the terminal"},
		{"DOTFILES_SPRITE", "the volumetric creature", "the shaded sprite is drawn", "the creature is drawn as glyphs"},
		{"DOTFILES_DRY_RUN", "whether a run changes the machine", "every step prints what it would do and writes nothing", "steps change the machine"},
		{"DOTFILES_SKIP_TOOLSET", "the toolset step", "the toolset step is left out", "the toolset step runs"},
	}

	var findings []doctorFinding
	var clear []string
	for _, variable := range switches {
		value := scan.getenv(variable.name)
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "0", "false", "1", "true", "yes", "no", "on", "off":
			on := value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
			state := variable.onFalse
			if on {
				state = variable.onTrue
			}
			clear = append(clear, fmt.Sprintf("%s=%s (%s)", variable.name, value, state))
		default:
			findings = append(findings, doctorFinding{
				Subject:  variable.name,
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("%s=%s, which is neither on nor off", variable.name, value),
				Impact:   "The switch that decides " + variable.effect + " reads this as off, which is the opposite of what most people expect from a value nobody documented.",
				Advice:   fmt.Sprintf("Use 1 or 0, or unset it: `unset %s`.", variable.name),
			})
		}
	}

	// The directories the installer's own tools read.
	directories := []struct {
		name string
		what string
		why  string
	}{
		{"DOTFILES_DIR", "the checkout the utilities read the themes and the WSL template from", "The theme picker and the WSL resources fall back to the copies an install left under the data directory, or say they cannot find them."},
		{"ZDOTDIR", "where zsh reads its startup files from", "zsh reads .zshrc, .zshenv and .p10k.zsh from that directory, so the files this installer wrote are not read."},
	}
	for _, directory := range directories {
		value := scan.getenv(directory.name)
		if value == "" {
			continue
		}
		info, err := scan.stat(value)
		switch {
		case err != nil:
			findings = append(findings, doctorFinding{
				Subject:  directory.name,
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("%s=%s, which is not there", directory.name, value),
				Impact:   directory.why,
				Advice:   fmt.Sprintf("Correct %s to an existing directory, or unset it (`unset %s`).", directory.name, directory.name),
			})
		case !info.IsDir():
			findings = append(findings, doctorFinding{
				Subject:  directory.name,
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("%s=%s, which is a file", directory.name, value),
				Impact:   directory.why,
				Advice:   fmt.Sprintf("Point %s at a directory, or unset it.", directory.name),
			})
		default:
			clear = append(clear, fmt.Sprintf("%s=%s is a directory", directory.name, value))
		}
	}

	if len(findings) == 0 {
		evidence := "no installer variable is set to a value that means nothing"
		if len(clear) > 0 {
			evidence = doctorPathList(clear, 3)
		}
		return []doctorFinding{{
			Subject:  "The installer's own variables",
			Status:   doctorOK,
			Evidence: evidence,
			Impact:   "Every switch and directory the installer reads holds a value it can act on.",
			Advice:   "Nothing to do.",
		}}
	}
	return findings
}

// doctorCheckPlatformSpecifics is the check that exists only where the platform
// allows it. It answers not-applicable with the reason everywhere else, which is
// what keeps a Linux machine from being told to fix a WSL setting.
func doctorCheckPlatformSpecifics(scan *doctorScan) []doctorFinding {
	info := scan.systemInfo()
	if info == nil {
		return []doctorFinding{{
			Subject:  "The platform's own checks",
			Status:   doctorUnknown,
			Evidence: "the platform was not detected",
			Impact:   "Which platform-specific faults apply here is unknown.",
			Advice:   "This is a defect in the doctor: the installer detects the platform before it builds this screen.",
		}}
	}

	switch {
	case info.IsWSL:
		return doctorCheckWSLSpecifics(scan, info.WSLVersion)
	case info.IsTermux:
		return doctorCheckTermuxSpecifics(scan)
	case info.OS == system.OSMac || scan.isMac():
		return doctorCheckMacSpecifics(scan)
	default:
		return []doctorFinding{{
			Subject:  "The platform's own checks",
			Status:   doctorNotApplicable,
			Evidence: fmt.Sprintf("%s has no platform-specific fault the doctor knows about", scan.platformLabel()),
			Impact:   "The checks that exist for WSL, Termux and macOS do not apply here; nothing about them is claimed.",
			Advice:   "Nothing to do.",
		}}
	}
}

// doctorCheckWSLSpecifics looks at the three things that decide whether a WSL
// distribution behaves: interop with the Windows host, whether the Windows
// drives are mounted, and whether the Windows PATH has leaked into PATH.
func doctorCheckWSLSpecifics(scan *doctorScan, version int) []doctorFinding {
	var findings []doctorFinding

	// The Windows mount. Its absence means the drives are not shared, which is a
	// deliberate configuration as often as it is a fault, so it is stated with
	// its consequence.
	if _, err := scan.stat("/mnt/c"); err != nil {
		findings = append(findings, doctorFinding{
			Subject:  "The Windows drives",
			Status:   doctorWarning,
			Evidence: "/mnt/c is not mounted: " + err.Error(),
			Impact:   "Nothing on the Windows side is reachable: a Windows binary cannot be run, and a font the installer places through interop cannot be copied.",
			Advice:   "Mount the drives with `sudo mount -t drvfs C: /mnt/c`, or leave it: everything that does not need Windows works without them.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "The Windows drives",
			Status:   doctorOK,
			Evidence: "/mnt/c is mounted",
			Impact:   "The interop path the WSL-specific steps use is available.",
			Advice:   "Nothing to do.",
		})
	}

	// Interop. The installer asks powershell.exe for the Windows host's memory
	// and processors, so an interop that is off is a degradation rather than a
	// failure, and the WSL resources screen says which values it could not read.
	powershell := scan.lookup("powershell.exe")
	if powershell.found() {
		findings = append(findings, doctorFinding{
			Subject:  "Interop with the Windows host",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("powershell.exe resolves to %s", powershell.Path),
			Impact:   "The host's memory and processor count can be read, which is what the WSL resource recommendation is built from.",
			Advice:   "Nothing to do.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "Interop with the Windows host",
			Status:   doctorWarning,
			Evidence: "powershell.exe does not resolve on PATH",
			Impact:   "The Windows host's real memory and processor count cannot be read, so the WSL resource recommendation falls back to the distribution's own numbers.",
			Advice:   "Enable interop in /etc/wsl.conf (`[interop]` section) and restart the distribution, or set DOTFILES_WSL_HOST_MEMORY_MB and DOTFILES_WSL_HOST_CPUS by hand.",
		})
	}

	// The Windows PATH inside a POSIX PATH. This is the classic WSL fault: a
	// Windows directory in PATH makes `which python` answer a Windows program, and
	// every Unix tool that resolves a command finds an .exe.
	var windowsEntries []string
	for _, entry := range scan.pathEntries() {
		if strings.HasPrefix(entry, "/mnt/c/") || strings.Contains(entry, "Windows/System32") ||
			strings.Contains(entry, "Windows\\System32") {
			windowsEntries = append(windowsEntries, entry)
		}
	}
	if len(windowsEntries) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  "Windows directories in the Linux PATH",
			Status:   doctorWarning,
			Evidence: doctorCounted(len(windowsEntries), "entry", "entries") + ": " + doctorPathList(windowsEntries, 3),
			Impact:   "A Unix command can resolve to a Windows program of the same name, which then reads Windows paths and reports Windows errors; scripted steps fail in ways that look unrelated.",
			Advice:   "Append the Windows interop path last in PATH, or turn it off in /etc/wsl.conf (`appendWindowsPath = false`), and open a new shell.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "Windows directories in the Linux PATH",
			Status:   doctorOK,
			Evidence: "no PATH entry resolves into the Windows drives",
			Impact:   "A Unix command cannot resolve to a Windows program with the same name.",
			Advice:   "Nothing to do.",
		})
	}

	findings = append(findings, doctorFinding{
		Subject:  "The WSL version",
		Status:   doctorOK,
		Evidence: fmt.Sprintf("this distribution reports WSL%d", version),
		Impact:   "The checks that assume WSL2's virtual disk and interop apply.",
		Advice:   "Nothing to do.",
	})
	return findings
}

// doctorCheckTermuxSpecifics reads the two things Termux answers differently
// from the Linux distributions: its own prefix, and whether shared storage was
// granted.
func doctorCheckTermuxSpecifics(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding

	prefix := scan.getenv("PREFIX")
	if prefix == "" {
		findings = append(findings, doctorFinding{
			Subject:  "The Termux prefix",
			Status:   doctorWarning,
			Evidence: "$PREFIX is not set, and this is Termux",
			Impact:   "The package manager and every binary live under the prefix, so a process without it cannot resolve Termux's own programs.",
			Advice:   "Start the installer from a Termux shell, which sets PREFIX itself.",
		})
	} else if info, err := scan.stat(prefix); err != nil || !info.IsDir() {
		findings = append(findings, doctorFinding{
			Subject:  "The Termux prefix",
			Status:   doctorError,
			Evidence: fmt.Sprintf("$PREFIX names %s, which is not a directory (%v)", prefix, err),
			Impact:   "Termux's package manager and its binaries cannot be resolved or installed.",
			Advice:   "Reinstall Termux, or correct PREFIX to the directory the app ships.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "The Termux prefix",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("$PREFIX names %s", prefix),
			Impact:   "Termux's own programs and packages resolve.",
			Advice:   "Nothing to do.",
		})
	}

	storage := filepath.Join(scan.home(), "storage")
	if info, err := scan.stat(storage); err != nil || !info.IsDir() {
		findings = append(findings, doctorFinding{
			Subject:  "Shared storage",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s is not a directory (%v)", doctorRelHome(scan, storage), err),
			Impact:   "No file outside Termux's private directory can be read or written, so a font or an archive copied from elsewhere is unreachable.",
			Advice:   "Grant it once with `termux-setup-storage`, and accept the permission the system asks for.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "Shared storage",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("%s is a directory", doctorRelHome(scan, storage)),
			Impact:   "Files outside Termux's private directory are reachable.",
			Advice:   "Nothing to do.",
		})
	}
	return findings
}

// doctorCheckMacSpecifics answers the one macOS fault that stops a run before it
// starts: the developer tools git is part of, and the xcode-select path the
// dependency step uses.
func doctorCheckMacSpecifics(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding

	lookup := scan.lookup("xcode-select")
	switch {
	case !lookup.found():
		findings = append(findings, doctorFinding{
			Subject:  "The developer tools",
			Status:   doctorWarning,
			Evidence: "xcode-select does not resolve on PATH",
			Impact:   "The dependency step installs the developer tools through it, and the system's own git comes with them.",
			Advice:   "Install the command line developer tools: `xcode-select --install`.",
		})
	default:
		result := scan.probe(doctorProbeTimeout, lookup.Path, "-p")
		if result.Err != "" {
			findings = append(findings, doctorFinding{
				Subject:  "The developer tools",
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("`xcode-select -p` failed: %s", firstNonEmpty(doctorFirstLine(result.Stderr), result.Err)),
				Impact:   "The developer tools are not installed or not selected, so the compiler and the system's git are missing.",
				Advice:   "Install them with `xcode-select --install`.",
			})
		} else {
			findings = append(findings, doctorFinding{
				Subject:  "The developer tools",
				Status:   doctorOK,
				Evidence: fmt.Sprintf("xcode-select points at %s", strings.TrimSpace(result.Stdout)),
				Impact:   "The compiler and the tools the steps need are installed.",
				Advice:   "Nothing to do.",
			})
		}
	}

	// Homebrew's prefix on Apple silicon is /opt/homebrew, which is not in the
	// default PATH for every shell: a brew that is installed and not on PATH is
	// the fault the managed tools check would report as missing.
	if _, err := scan.stat("/opt/homebrew/bin"); err == nil {
		if !doctorContains(scan.pathEntries(), "/opt/homebrew/bin") {
			findings = append(findings, doctorFinding{
				Subject:  "Homebrew's path on Apple silicon",
				Status:   doctorWarning,
				Evidence: "/opt/homebrew/bin exists and is not in PATH",
				Impact:   "Every tool installed through Homebrew on this Mac is invisible: the installer would report them as missing and install them again.",
				Advice:   "Add it where PATH is built for the login shell: `eval \"$(/opt/homebrew/bin/brew shellenv)\"`.",
			})
		} else {
			findings = append(findings, doctorFinding{
				Subject:  "Homebrew's path on Apple silicon",
				Status:   doctorOK,
				Evidence: "/opt/homebrew/bin is in PATH",
				Impact:   "The tools Homebrew installs resolve.",
				Advice:   "Nothing to do.",
			})
		}
	}
	return findings
}

// firstNonEmpty is a small helper for evidence lines that prefer one source and
// settle for another.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
