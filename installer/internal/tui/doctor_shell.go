package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE SHELLS AND THEIR CONFIGURATION
// ============================================================================
//
// The shells are where the installer's own work lands, and they are the part of
// the machine most often edited by hand. These checks answer three different
// questions and keep them apart: which shell this process is really running in,
// which shell a terminal starts, and which shells have a configuration here at
// all. A shell the user never chose is not a fault, so its absence is reported
// as not applicable rather than as a warning.
//
// Two rules hold the whole file:
//
//   - **A checker is used only when the shell has one.** zsh and bash read a
//     file without running it (-n), fish does the same (--no-execute) and
//     nushell has its own ide check. Where a shell has no such mode the answer
//     is unknown with the manual command, never a verdict.
//   - **No script is interpreted.** The references and the conflicts come from
//     literal lines -- a quoted path after `source`, an `alias name=value` whose
//     value is a literal -- and a line that would need a shell to understand is
//     skipped rather than guessed at.
//
// The paths come from system.ConfigPathsFor, the same map the backup step, the
// overwrite count and the last-install record read, so this file cannot grow a
// second list of what the installer writes.

// doctorShellFile is one file a shell reads, named by the installer's own config
// key so the path is derived rather than typed.
type doctorShellFile struct {
	// Key is the key system.ConfigPathsFor knows: "fish", "zsh", "nushell", ...
	Key string
	// Sub is the path under that key. Empty means the key's own path.
	Sub string
	// Why says what the file is for, in the section's voice.
	Why string
	// Needed marks the file the shell cannot do without when the installer has
	// configured this shell: its absence is a warning rather than a statement.
	Needed bool
	// Check marks the files the syntax check reads: the shell's own startup
	// files and the directory the installer writes into.
	Check bool
	// Dir marks an entry that is a directory rather than a file.
	Dir bool
}

// doctorShellProfile is one shell the installer knows how to configure.
type doctorShellProfile struct {
	// Name is the binary name, which is also what `$SHELL` ends with.
	Name string
	// Files are the files this installer writes for the shell, in the order the
	// shell reads them.
	Files []doctorShellFile
	// SyntaxArgs names the shell's own non-executing syntax check. The file is
	// appended.
	SyntaxArgs []string
	// SyntaxNote is why there is no check, when SyntaxArgs is empty.
	SyntaxNote string
}

// doctorShellProfiles is the catalogue: one profile per shell this installer
// configures. The config keys it names are the keys of system.ConfigPathsFor,
// so a path this installer writes but never checks here is visibly missing from
// this table rather than silently unchecked.
func doctorShellProfiles() []doctorShellProfile {
	return []doctorShellProfile{
		{
			Name: "zsh",
			Files: []doctorShellFile{
				{Key: "zshenv", Why: "read by every zsh invocation, including the non-interactive ones", Needed: true, Check: true},
				{Key: "zsh", Why: "the interactive configuration this installer writes", Needed: true, Check: true},
				{Key: "zsh_p10k", Why: "the Powerlevel10k prompt the zsh configuration selects", Check: true},
				{Key: "oh-my-zsh", Sub: "oh-my-zsh.sh", Why: "the plugin framework the zsh configuration sources", Needed: true},
			},
			SyntaxArgs: []string{"-n"},
		},
		{
			Name: "fish",
			Files: []doctorShellFile{
				{Key: "fish", Sub: "config.fish", Why: "the interactive configuration this installer writes", Needed: true, Check: true},
				{Key: "fish", Sub: "conf.d", Why: "the drop-in directory the installer writes its own files into", Dir: true},
				{Key: "fish", Sub: "functions", Why: "the functions this repository vendors", Dir: true},
			},
			SyntaxArgs: []string{"--no-execute"},
		},
		{
			Name: "nu",
			Files: []doctorShellFile{
				{Key: "nushell", Sub: "config.nu", Why: "the interactive configuration this installer writes", Needed: true, Check: true},
				{Key: "nushell", Sub: "env.nu", Why: "the environment the configuration is built on", Check: true},
				{Key: "nushell_macos", Why: "where nushell reads its configuration on macOS", Check: true},
			},
			SyntaxArgs: []string{"--ide-check", "5"},
		},
		{
			Name: "bash",
			Files: []doctorShellFile{
				{Key: "bashrc", Why: "the file the Homebrew and default-shell steps append to", Check: true},
			},
			SyntaxArgs: []string{"-n"},
		},
	}
}

// doctorShellProfileFor returns the profile of a shell by its binary name, and
// whether the installer knows the shell at all.
func doctorShellProfileFor(name string) (doctorShellProfile, bool) {
	for _, profile := range doctorShellProfiles() {
		if profile.Name == name {
			return profile, true
		}
	}
	return doctorShellProfile{}, false
}

// doctorShellPath is the resolved path for one profile file, or the empty string
// when there is no home to resolve it against.
func doctorShellPath(scan *doctorScan, file doctorShellFile) string {
	paths := system.ConfigPathsFor(scan.home())
	base, ok := paths[file.Key]
	if !ok || base == "" {
		return ""
	}
	if file.Sub == "" {
		return base
	}
	return filepath.Join(base, file.Sub)
}

// doctorShellChecks is the shells category's catalogue.
func doctorShellChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "shell.identity",
			Category: doctorCategoryShell,
			Title:    "Which shell runs now, and which shell a terminal starts",
			Run:      doctorCheckShellIdentity,
		},
		{
			ID:       "shell.files",
			Category: doctorCategoryShell,
			Title:    "The installed shells have the files their configuration expects",
			Required: true,
			Run:      doctorCheckShellFiles,
		},
		{
			ID:       "shell.syntax",
			Category: doctorCategoryShell,
			Title:    "The installer's shell files pass the shell's own syntax check",
			Run:      doctorCheckShellSyntax,
		},
		{
			ID:       "shell.references",
			Category: doctorCategoryShell,
			Title:    "The files the configuration sources are there",
			Run:      doctorCheckShellReferences,
		},
		{
			ID:       "shell.conflicts",
			Category: doctorCategoryShell,
			Title:    "No alias or variable is defined twice with different values",
			Run:      doctorCheckShellConflicts,
		},
	}
}

// doctorCheckShellIdentity names the three shells that are easy to confuse: the
// shell this process was started from, the login shell the system would start,
// and the shells that have a configuration on this machine.
//
// The finding is informational: running zsh inside fish, or having a login shell
// that is not the one the user works in, is a normal configuration and not a
// fault. What the check establishes is that the doctor knows which is which
// before it says anything about a shell's files.
func doctorCheckShellIdentity(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding

	current := doctorCurrentShell(scan)
	login := scan.getenv("SHELL")

	switch {
	case current != "" && login != "":
		findings = append(findings, doctorFinding{
			Subject:  "The shell running this process, and the login shell",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("this process runs under %s; $SHELL names %s", current, login),
			Impact:   "Both are normal: the shell a terminal opens and the shell a script runs under are allowed to differ.",
			Advice:   "Nothing to do.",
		})
	case login != "":
		findings = append(findings, doctorFinding{
			Subject:  "The login shell",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("$SHELL names %s, and the shell this process runs under could not be read", login),
			Impact:   "Which shell is actually running this process is not known; the login shell is.",
			Advice:   "Nothing to do: the rest of the shell checks work from the login shell and from the files on disk.",
		})
	case current != "":
		findings = append(findings, doctorFinding{
			Subject:  "The shell running this process",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("this process runs under %s; $SHELL is not set", current),
			Impact:   "The system's login shell is unnamed, so a terminal and the installer may not start the same shell.",
			Advice:   "Set SHELL in the environment that starts the installer, or with `chsh -s $(command -v " + current + ")`.",
		})
	default:
		findings = append(findings, doctorFinding{
			Subject:  "Which shell is running",
			Status:   doctorUnknown,
			Evidence: "$SHELL is not set and the parent process name could not be read",
			Impact:   "Neither the login shell nor the running shell is known.",
			Advice:   "Set SHELL, or start the installer from a named shell.",
		})
	}

	// Which shells have a configuration here. This is the "other shells" half of
	// the brief: a machine can have three shells configured and the user only
	// ever starts one.
	var configured []string
	var installed []string
	for _, profile := range doctorShellProfiles() {
		lookup := scan.lookup(profile.Name)
		if lookup.found() {
			installed = append(installed, profile.Name)
		}
		if doctorShellHasConfiguration(scan, profile) {
			configured = append(configured, profile.Name)
		}
	}
	if len(installed) > 0 || len(configured) > 0 {
		findings = append(findings, doctorFinding{
			Subject: "The shells on this machine",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("installed: %s; configured by this installer: %s",
				doctorPathList(installed, 6), doctorPathList(configured, 6)),
			Impact: "A shell that is installed and not configured is a shell the installer left alone, which is not a fault.",
			Advice: "Nothing to do.",
		})
	}
	return findings
}

// doctorCurrentShell answers which shell this process runs under, without a
// command where the platform can be read directly: on Linux the parent process
// name is one file, and everywhere else ps is asked once.
//
// The answer is the process that started the installer, which is what a user
// means by "the shell I am in". It is deliberately the parent and not this
// process: the installer is its own binary, and asking what it is would answer
// "dotfiles".
func doctorCurrentShell(scan *doctorScan) string {
	ppid := doctorParentPID()
	if ppid < 0 {
		return ""
	}
	if name := doctorParentProcessName(scan, ppid); name != "" {
		return name
	}
	return ""
}

// doctorParentPID is this process's parent. It is a variable so a test can name
// a parent without depending on the process tree the test runner happens to
// have.
var doctorParentPID = os.Getppid

// doctorParentProcessName reads the parent's name from /proc when the platform
// has one, and falls back to ps. A machine with neither reports nothing, which
// the caller turns into unknown rather than into a guess.
func doctorParentProcessName(scan *doctorScan, ppid int) string {
	if data, err := scan.readFile(filepath.Join(scan.procRoot(), fmt.Sprintf("%d", ppid), "comm")); err == nil {
		if name := strings.TrimSpace(data); name != "" {
			return name
		}
	}
	lookup := scan.lookup("ps")
	if !lookup.found() {
		return ""
	}
	result := scan.probe(doctorProbeTimeout, lookup.Path, "-o", "comm=", "-p", fmt.Sprintf("%d", ppid))
	if result.Err != "" {
		return ""
	}
	return doctorFirstLine(result.Stdout)
}

// doctorShellHasConfiguration reports whether this installer's configuration for
// a shell is on the machine: the shell's main file or its directory exists.
func doctorShellHasConfiguration(scan *doctorScan, profile doctorShellProfile) bool {
	for _, file := range profile.Files {
		path := doctorShellPath(scan, file)
		if path == "" {
			continue
		}
		if _, err := scan.stat(path); err == nil {
			return true
		}
	}
	return false
}

// doctorCheckShellFiles walks each profile's files and reports what it found.
//
// The states are deliberately different per shell. A shell whose configuration
// the installer wrote, with a file missing, is a warning. A shell that is
// installed and has nothing of this installer's in it is not a fault -- the user
// did not choose it -- and is reported as not applicable with the reason. A
// shell that is not installed at all gets no rows of its own.
func doctorCheckShellFiles(scan *doctorScan) []doctorFinding {
	home := scan.home()
	if home == "" {
		return []doctorFinding{{
			Subject:  "The shells' configuration files",
			Status:   doctorUnknown,
			Evidence: "neither $HOME nor the platform's user lookup answered",
			Impact:   "The files the installer writes for the shells cannot be looked for.",
			Advice:   "Set HOME and run the doctor again.",
		}}
	}

	var findings []doctorFinding
	anyInstalled := false
	for _, profile := range doctorShellProfiles() {
		if !scan.lookup(profile.Name).found() {
			// The shell is not installed: its configuration is not a subject. The
			// tools check already named the shell itself.
			continue
		}
		anyInstalled = true
		if !doctorShellHasConfiguration(scan, profile) {
			findings = append(findings, doctorFinding{
				Subject: profile.Name + " configuration",
				Status:  doctorNotApplicable,
				Evidence: fmt.Sprintf("%s is installed and none of the files this installer writes for it is present (%s)",
					profile.Name, doctorShellFileList(scan, profile, 3)),
				Impact: "This installer never configured this shell here, so there is nothing of its own to check in it.",
				Advice: "Nothing to do; run the installer's shell step if you want this shell configured.",
			})
			continue
		}
		findings = append(findings, doctorShellFileFindings(scan, profile)...)
	}

	if !anyInstalled {
		return []doctorFinding{{
			Subject:  "The shells' configuration files",
			Status:   doctorNotApplicable,
			Evidence: "none of zsh, fish, nushell or bash resolves on PATH",
			Impact:   "There is no shell configuration to check.",
			Advice:   "Install a shell; the installer's shell step provisions zsh, fish and nushell.",
		}}
	}
	return findings
}

// doctorShellFileFindings reports one profile's files.
func doctorShellFileFindings(scan *doctorScan, profile doctorShellProfile) []doctorFinding {
	var findings []doctorFinding
	var clear []string
	for _, file := range profile.Files {
		path := doctorShellPath(scan, file)
		if path == "" {
			continue
		}
		info, err := scan.stat(path)
		switch {
		case err != nil:
			status := doctorNotApplicable
			if file.Needed {
				status = doctorWarning
			}
			findings = append(findings, doctorFinding{
				Subject:  profile.Name + ": " + doctorRelHome(scan, path),
				Status:   status,
				Evidence: "the path is not there",
				Impact:   doctorShellFileImpact(file, false),
				Advice:   "Let the installer's shell step write it again, or restore it from a backup.",
			})
		case file.Dir && !info.IsDir():
			findings = append(findings, doctorFinding{
				Subject:  profile.Name + ": " + doctorRelHome(scan, path),
				Status:   doctorWarning,
				Evidence: "the path is a file, and this is where the shell reads a directory of files",
				Impact:   doctorShellFileImpact(file, true),
				Advice:   "Move the file aside and let the installer's shell step create the directory.",
			})
		default:
			clear = append(clear, doctorRelHome(scan, path))
		}
	}
	if len(clear) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  profile.Name + " configuration files",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("present: %s", doctorPathList(clear, 4)),
			Impact:   "The configuration this installer wrote for this shell is where the shell reads it.",
			Advice:   "Nothing to do.",
		})
	}
	return findings
}

// doctorShellFileImpact says what a missing or wrongly-typed file costs.
func doctorShellFileImpact(file doctorShellFile, wrongType bool) string {
	if wrongType {
		return "The shell looks for a directory here and finds a file: " + file.Why + "."
	}
	if file.Needed {
		return "The shell reads this file on every start, so what this installer configured for it does not load: " + file.Why + "."
	}
	return "The shell reads this file when it is there: " + file.Why + "."
}

// doctorShellFileList names up to most of a profile's files, for an evidence
// line.
func doctorShellFileList(scan *doctorScan, profile doctorShellProfile, most int) string {
	var paths []string
	for _, file := range profile.Files {
		if path := doctorShellPath(scan, file); path != "" {
			paths = append(paths, doctorRelHome(scan, path))
		}
	}
	return doctorPathList(paths, most)
}

// doctorCheckShellSyntax runs each shell's own syntax check over the files that
// shell reads, in that shell's own words.
//
// The check is not run for a shell whose binary is absent: without the shell
// there is no checker, and the answer is unknown with the manual command rather
// than a claim about syntax nobody verified.
func doctorCheckShellSyntax(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding
	for _, profile := range doctorShellProfiles() {
		lookup := scan.lookup(profile.Name)
		if !lookup.found() {
			continue
		}
		// Only the shells this installer configured are checked: running a syntax
		// check over files the installer never wrote is a different job, and the
		// section's own scope is the configuration it manages.
		if !doctorShellHasConfiguration(scan, profile) {
			continue
		}

		files := doctorShellFilesToCheck(scan, profile)
		if len(files) == 0 {
			findings = append(findings, doctorFinding{
				Subject:  profile.Name + " syntax",
				Status:   doctorNotApplicable,
				Evidence: fmt.Sprintf("%s is installed, and none of the files it would read is present", profile.Name),
				Impact:   "There is no file to check.",
				Advice:   "Nothing to do.",
			})
			continue
		}
		if len(profile.SyntaxArgs) == 0 {
			findings = append(findings, doctorFinding{
				Subject:  profile.Name + " syntax",
				Status:   doctorUnknown,
				Evidence: fmt.Sprintf("%s has no mode that reads a file without running it here: %s", profile.Name, profile.SyntaxNote),
				Impact:   "The doctor will not run a configuration file to find out whether it parses: running it is the thing a check must not do.",
				Advice:   "Check it by hand with the shell's own tooling.",
			})
			continue
		}

		// The two ways a check can fail are not the same thing: a shell that ran and
		// refused the file, and a shell that could not be asked at all. The first is
		// a broken file, the second is a limit of the doctor's own method, and the
		// report says which one it is.
		var refused, unasked, checked []string
		for _, path := range files {
			args := append(append([]string(nil), profile.SyntaxArgs...), path)
			result := scan.probe(doctorProbeTimeout, lookup.Path, args...)
			detail := doctorFirstLine(result.Stdout)
			if detail == "" {
				detail = doctorFirstLine(result.Stderr)
			}
			if detail == "" {
				detail = result.Err
			}
			switch {
			case result.Err == "":
				checked = append(checked, doctorRelHome(scan, path))
			case result.TimedOut || result.Code <= 0:
				// No exit status: the checker itself never ran or never finished.
				unasked = append(unasked, fmt.Sprintf("%s: %s", doctorRelHome(scan, path), detail))
			default:
				refused = append(refused, fmt.Sprintf("%s: %s", doctorRelHome(scan, path), detail))
			}
		}

		if len(refused) > 0 || len(unasked) > 0 {
			status := doctorError
			var parts []string
			if len(refused) > 0 {
				parts = append(parts, "refused "+doctorPathList(refused, 3))
				if len(refused) == len(files) {
					// Every file was refused, which is the signature of a checker that
					// does not understand these files rather than of files that are
					// broken (a fish older than --no-execute, a nushell that does not
					// know --ide-check). The doctor says so instead of blaming the
					// files.
					status = doctorUnknown
				}
			}
			if len(unasked) > 0 {
				parts = append(parts, "could not be asked about "+doctorPathList(unasked, 3))
				if len(refused) == 0 {
					status = doctorUnknown
				}
			}
			impact := "A file that does not parse is a file the shell stops reading: everything after the fault does not load, and the failure appears at every shell start."
			advice := fmt.Sprintf("Check it by hand: `%s %s <file>`", doctorRelHome(scan, lookup.Path), strings.Join(profile.SyntaxArgs, " "))
			if len(refused) == 0 {
				impact = "Whether these files parse is unknown: the doctor asks the shell rather than reading the file itself, and the shell did not answer."
				advice = fmt.Sprintf("Run the same check by hand: `%s %s <file>`, and look at the checker itself with `%s --version`.",
					doctorRelHome(scan, lookup.Path), strings.Join(profile.SyntaxArgs, " "), doctorRelHome(scan, lookup.Path))
			}
			findings = append(findings, doctorFinding{
				Subject:  profile.Name + " configuration syntax",
				Status:   status,
				Evidence: fmt.Sprintf("%s %s %s", profile.Name, strings.Join(profile.SyntaxArgs, " "), strings.Join(parts, "; ")),
				Impact:   impact,
				Advice:   advice,
			})
			continue
		}
		findings = append(findings, doctorFinding{
			Subject: profile.Name + " configuration syntax",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("%s file(s) parsed by `%s %s`: %s", doctorCounted(len(checked), "file", "files"),
				filepath.Base(lookup.Path), strings.Join(profile.SyntaxArgs, " "), doctorPathList(checked, 4)),
			Impact: "The shell reads these files without stopping at a syntax error.",
			Advice: "Nothing to do.",
		})
	}

	if len(findings) == 0 {
		return []doctorFinding{{
			Subject:  "The shells' configuration syntax",
			Status:   doctorNotApplicable,
			Evidence: "no shell this installer configures is installed on this machine",
			Impact:   "There is nothing to parse.",
			Advice:   "Nothing to do.",
		}}
	}
	return findings
}

// doctorShellFilesToCheck lists the files one shell's syntax check reads: the
// files the profile names, plus the directory a shell reads many files from.
// The list is capped so a directory with hundreds of files does not turn one
// check into hundreds of processes.
func doctorShellFilesToCheck(scan *doctorScan, profile doctorShellProfile) []string {
	const mostFiles = 40
	var files []string
	for _, file := range profile.Files {
		path := doctorShellPath(scan, file)
		if path == "" {
			continue
		}
		info, err := scan.stat(path)
		if err != nil {
			continue
		}
		if file.Dir {
			if !info.IsDir() {
				continue
			}
			entries, err := scan.readDir(path)
			if err != nil {
				// A directory that cannot be listed is named by the files check's
				// own reading of the same path; here it is simply skipped.
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() || !doctorShellSyntaxExtension(profile.Name, entry.Name()) {
					continue
				}
				files = append(files, filepath.Join(path, entry.Name()))
				if len(files) >= mostFiles {
					return files
				}
			}
			continue
		}
		if !file.Check || info.IsDir() {
			continue
		}
		files = append(files, path)
		if len(files) >= mostFiles {
			return files
		}
	}
	return files
}

// doctorShellSyntaxExtension reports whether a file inside a directory a shell
// reads is one that shell would parse.
func doctorShellSyntaxExtension(shell, name string) bool {
	switch shell {
	case "fish":
		return strings.HasSuffix(name, ".fish")
	case "nu":
		return strings.HasSuffix(name, ".nu")
	case "zsh":
		return strings.HasSuffix(name, ".zsh") || !strings.Contains(name, ".")
	default:
		return true
	}
}

// doctorSourcePattern matches the two ways a shell's file includes another file
// with a literal path: `source <path>` and `. <path>`. The path may be quoted.
var doctorSourcePattern = regexp.MustCompile(`(?m)^\s*(?:source|\.)\s+("[^"]+"|'[^']+'|[^\s;#]+)`)

// doctorAliasPattern matches a literal alias definition in zsh, bash and fish.
// The value must be a plain word or a quoted string; anything with a variable, a
// command substitution or an unquoted space is skipped rather than guessed at.
var doctorAliasPattern = regexp.MustCompile(`(?m)^\s*alias\s+([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*("[^"$` + "`" + `]*"|'[^'$` + "`" + `]*'|[^\s;#"'` + "`" + `]+)`)

// doctorExportPattern matches a literal variable assignment in the POSIX shells
// and in fish, whose syntax differs but whose meaning for this check does not:
// the same name set to two different literal values in two files.
var doctorExportPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?set\s+-[gUx]+\s+([A-Za-z_][A-Za-z0-9_]*)\s+("[^"$` + "`" + `]*"|'[^'$` + "`" + `]*'|[^\s;#"'` + "`" + `]+)|^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*("[^"$` + "`" + `]*"|'[^'$` + "`" + `]*'|[^\s;#"'` + "`" + `]+)`)

// doctorShellReadableFiles lists the files of the configured shells that can be
// read, capped, so the reference and conflict checks read the same set.
func doctorShellReadableFiles(scan *doctorScan) []string {
	const mostFiles = 60
	var files []string
	seen := map[string]bool{}
	for _, profile := range doctorShellProfiles() {
		// A shell that is not installed has no configuration of this installer's to
		// read; one that is installed but has no files contributes nothing either.
		for _, file := range profile.Files {
			path := doctorShellPath(scan, file)
			if path == "" || seen[path] {
				continue
			}
			info, err := scan.stat(path)
			if err != nil {
				continue
			}
			if info.IsDir() {
				entries, err := scan.readDir(path)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if entry.IsDir() || !doctorShellSyntaxExtension(profile.Name, entry.Name()) {
						continue
					}
					full := filepath.Join(path, entry.Name())
					if seen[full] {
						continue
					}
					seen[full] = true
					files = append(files, full)
					if len(files) >= mostFiles {
						return files
					}
				}
				continue
			}
			if !file.Check {
				continue
			}
			seen[path] = true
			files = append(files, path)
			if len(files) >= mostFiles {
				return files
			}
		}
	}
	return files
}

// doctorShellReference is one literal include found in a configuration file.
type doctorShellReference struct {
	file   string
	target string
	// resolvable is the absolute path the literal names, or empty when the
	// literal needs a shell to understand it (a variable other than the ones
	// named below, a command substitution, a glob).
	resolvable string
}

// doctorCheckShellReferences resolves the literal includes of the configured
// shells' files. `source ~/.foo` and `. $HOME/.bar` are read literally, and a
// line that needs a shell to expand it -- `source $ZSH/oh-my-zsh.sh`, a glob, a
// command substitution -- is skipped, because guessing at what it expands to
// would be interpreting the script.
func doctorCheckShellReferences(scan *doctorScan) []doctorFinding {
	files := doctorShellReadableFiles(scan)
	if len(files) == 0 {
		return []doctorFinding{{
			Subject:  "The files the shell configuration sources",
			Status:   doctorNotApplicable,
			Evidence: "no configuration file of a shell this installer configured could be read",
			Impact:   "There is nothing to resolve.",
			Advice:   "Nothing to do.",
		}}
	}

	var broken []string
	var resolved []string
	var skipped int
	for _, file := range files {
		content, err := scan.readFile(file)
		if err != nil {
			skipped++
			continue
		}
		for _, match := range doctorSourcePattern.FindAllStringSubmatch(content, -1) {
			raw := strings.Trim(match[1], `"'`)
			target, ok := doctorResolveShellLiteral(scan, file, raw)
			if !ok {
				skipped++
				continue
			}
			if _, err := scan.stat(target); err != nil {
				broken = append(broken, fmt.Sprintf("%s sources %s, which is not there", doctorRelHome(scan, file), doctorRelHome(scan, target)))
				continue
			}
			resolved = append(resolved, doctorRelHome(scan, target))
		}
	}

	if len(broken) > 0 {
		return []doctorFinding{{
			Subject:  "Broken includes in the shell configuration",
			Status:   doctorWarning,
			Evidence: doctorPathList(broken, 3),
			Impact:   "The shell prints an error at every start, and the configuration the missing file carried does not load.",
			Advice:   "Fix the path in the file that sources it, or restore the file from a backup; an include this installer wrote can be written again by its shell step.",
		}}
	}
	if len(resolved) == 0 {
		return []doctorFinding{{
			Subject: "The files the shell configuration sources",
			Status:  doctorNotApplicable,
			Evidence: fmt.Sprintf("%s read, none with a literal include the doctor can resolve (%d include lines need the shell to expand them)",
				doctorCounted(len(files), "configuration file", "configuration files"), skipped),
			Impact: "Nothing about includes is established; the doctor does not expand a variable it cannot resolve.",
			Advice: "Nothing to do.",
		}}
	}
	evidence := fmt.Sprintf("%s resolve: %s", doctorCounted(len(resolved), "literal include", "literal includes"), doctorPathList(resolved, 5))
	if skipped > 0 {
		evidence += fmt.Sprintf("; %s need the shell to expand and were not resolved", doctorCounted(skipped, "include line", "include lines"))
	}
	return []doctorFinding{{
		Subject:  "The files the shell configuration sources",
		Status:   doctorOK,
		Evidence: evidence,
		Impact:   "Every include the doctor can read literally is present.",
		Advice:   "Nothing to do.",
	}}
}

// doctorResolveShellLiteral turns the literal after source into an absolute path
// when it can be read literally: a quoted or unquoted path, with the leading ~,
// $HOME, $ZDOTDIR or $XDG_CONFIG_HOME expanded. Anything else is not resolved.
func doctorResolveShellLiteral(scan *doctorScan, file, raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	home := scan.home()
	switch {
	case raw == "~" || strings.HasPrefix(raw, "~/"):
		if home == "" {
			return "", false
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(raw, "~"), "/")), true
	case strings.HasPrefix(raw, "$HOME/"):
		if home == "" {
			return "", false
		}
		return filepath.Join(home, strings.TrimPrefix(raw, "$HOME/")), true
	case strings.HasPrefix(raw, "${HOME}/"):
		if home == "" {
			return "", false
		}
		return filepath.Join(home, strings.TrimPrefix(raw, "${HOME}/")), true
	case strings.HasPrefix(raw, "$ZDOTDIR/"):
		base := scan.getenv("ZDOTDIR")
		if base == "" {
			base = home
		}
		if base == "" {
			return "", false
		}
		return filepath.Join(base, strings.TrimPrefix(raw, "$ZDOTDIR/")), true
	case strings.HasPrefix(raw, "$XDG_CONFIG_HOME/"):
		base := scan.configHome()
		if base == "" {
			return "", false
		}
		return filepath.Join(base, strings.TrimPrefix(raw, "$XDG_CONFIG_HOME/")), true
	case strings.Contains(raw, "$"), strings.Contains(raw, "`"), strings.ContainsAny(raw, "*?["):
		return "", false
	case strings.HasPrefix(raw, "/"):
		return filepath.Clean(raw), true
	case strings.HasPrefix(raw, "./"), strings.HasPrefix(raw, "../"):
		// Relative includes are relative to the file that holds them, which is the
		// only honest way to resolve them.
		return filepath.Clean(filepath.Join(filepath.Dir(file), raw)), true
	default:
		// A bare name is a command on PATH, not a file: `source nvm.sh` needs the
		// shell to search. The doctor does not chase it.
		return "", false
	}
}

// doctorShellDefinition is one literal definition of an alias or a variable.
type doctorShellDefinition struct {
	file  string
	line  int
	value string
}

// doctorCheckShellConflicts reports a name that two configuration files define
// with two different literal values. Only literals are compared, and only names
// that appear in more than one file: the same alias defined twice in the same
// file is the file's own business, and a value that needs a shell to expand is
// not compared at all.
func doctorCheckShellConflicts(scan *doctorScan) []doctorFinding {
	files := doctorShellReadableFiles(scan)
	if len(files) == 0 {
		return []doctorFinding{{
			Subject:  "Aliases and variables defined twice",
			Status:   doctorNotApplicable,
			Evidence: "no configuration file of a shell this installer configured could be read",
			Impact:   "There is nothing to compare.",
			Advice:   "Nothing to do.",
		}}
	}

	aliases := map[string][]doctorShellDefinition{}
	variables := map[string][]doctorShellDefinition{}
	var read []string
	for _, file := range files {
		content, err := scan.readFile(file)
		if err != nil {
			continue
		}
		read = append(read, file)
		for _, match := range doctorAliasPattern.FindAllStringSubmatchIndex(content, -1) {
			name := content[match[2]:match[3]]
			value := strings.Trim(content[match[4]:match[5]], `"'`)
			aliases[name] = append(aliases[name], doctorShellDefinition{
				file: file, line: doctorLineOf(content, match[0]), value: value,
			})
		}
		for _, match := range doctorExportPattern.FindAllStringSubmatchIndex(content, -1) {
			name, value := "", ""
			if match[2] >= 0 {
				name = content[match[2]:match[3]]
				value = strings.Trim(content[match[4]:match[5]], `"'`)
			} else if match[6] >= 0 {
				name = content[match[6]:match[7]]
				value = strings.Trim(content[match[8]:match[9]], `"'`)
			}
			if name == "" {
				continue
			}
			variables[name] = append(variables[name], doctorShellDefinition{
				file: file, line: doctorLineOf(content, match[0]), value: value,
			})
		}
	}

	if len(read) == 0 {
		return []doctorFinding{{
			Subject:  "Aliases and variables defined twice",
			Status:   doctorUnknown,
			Evidence: "the configuration files could not be read",
			Impact:   "Nothing about conflicting definitions is known.",
			Advice:   "Check the files' permissions, or run the doctor as the user that owns them.",
		}}
	}

	var findings []doctorFinding
	for _, name := range doctorSortedKeys(aliases) {
		if finding, ok := doctorConflictFinding(scan, "alias", name, aliases[name]); ok {
			findings = append(findings, finding)
		}
	}
	for _, name := range doctorSortedKeys(variables) {
		if finding, ok := doctorConflictFinding(scan, "variable", name, variables[name]); ok {
			findings = append(findings, finding)
		}
	}

	if len(findings) == 0 {
		return []doctorFinding{{
			Subject: "Aliases and variables defined twice",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("%s read, and no name is defined twice with two different literal values",
				doctorCounted(len(read), "configuration file", "configuration files")),
			Impact: "Which definition wins is decided by the load order the configuration already has.",
			Advice: "Nothing to do.",
		}}
	}
	return findings
}

// doctorConflictFinding turns one name's definitions into a finding, or reports
// no finding when the definitions agree or come from one file.
func doctorConflictFinding(scan *doctorScan, kind, name string, definitions []doctorShellDefinition) (doctorFinding, bool) {
	if len(definitions) < 2 {
		return doctorFinding{}, false
	}
	values := map[string]bool{}
	files := map[string]bool{}
	for _, definition := range definitions {
		values[definition.value] = true
		files[definition.file] = true
	}
	if len(values) < 2 || len(files) < 2 {
		return doctorFinding{}, false
	}

	var where []string
	for _, definition := range definitions {
		where = append(where, fmt.Sprintf("%s:%d sets %q", doctorRelHome(scan, definition.file), definition.line, definition.value))
	}
	sort.Strings(where)
	return doctorFinding{
		Subject:  fmt.Sprintf("The %s %s is defined %d times with different values", kind, name, len(values)),
		Status:   doctorWarning,
		Evidence: doctorPathList(where, 4),
		Impact:   "Which value wins depends on the order the shell reads the files in, so the same shell can behave differently in a terminal and in a script.",
		Advice:   "Keep one definition, or make the later file's value the one you want deliberately.",
	}, true
}

// doctorLineOf is the 1-based line a byte offset falls on, used to name where a
// definition was found.
func doctorLineOf(content string, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	return strings.Count(content[:offset], "\n") + 1
}
