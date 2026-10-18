package tui

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ============================================================================
// THE TOOLS THIS INSTALLER MANAGES, AND WHAT A RUN NEEDS
// ============================================================================
//
// Two questions live here, and they are different questions. The first is which
// of the tools this installer provisions are on the machine and answer their own
// version command: a tool that was never installed is a component somebody chose
// to leave out (a warning, with the unit that would have installed it), while a
// tool that resolves but does not answer is broken (an error when a run needs
// it). The second is what this installer itself needs to finish a run: a POSIX
// shell, git, curl, the platform's package manager and the way it becomes root.
//
// The catalogue below is checked against the repository, not trusted: the guard
// doctor_tools_test.go requires every name to appear in the sources this
// repository declares -- the Brewfile, the package list per distribution, the
// units' own tables -- so a tool nobody installs cannot be invented here.

// doctorPlatformGate decides on which hosts a managed tool exists at all. A tool
// that is not for this platform is left out of the run rather than reported as
// missing, because "you do not have df on Windows" is not a finding.
type doctorPlatformGate int

const (
	doctorGateAny doctorPlatformGate = iota
	doctorGateMac
	doctorGateTermux
	doctorGateUnix // every platform this installer runs on except Termux
)

// applies reports whether the tool exists on this host at all.
func (g doctorPlatformGate) applies(scan *doctorScan) bool {
	switch g {
	case doctorGateMac:
		return scan.isMac()
	case doctorGateTermux:
		return scan.isTermux()
	case doctorGateUnix:
		return !scan.isTermux()
	default:
		return true
	}
}

// doctorTool is one tool this installer really provisions.
type doctorTool struct {
	// Name is the binary a terminal would run, and the name the report uses.
	Name string
	// Unit is the install unit that provisions it, by the ID installUnits
	// declares. The guard fails a name that no unit knows.
	Unit string
	// VersionArgs is the command that prints the tool's version, in the tool's own
	// convention.
	VersionArgs []string
	// Required marks the tools an installation cannot finish without. A tool that
	// is required and missing is an error; every other missing tool is a warning
	// that names the unit it belongs to.
	Required bool
	// RequiredFor names the step whose own need makes it required, for the tools
	// that are only needed by a step the user can switch off.
	RequiredFor string
	// Missing is what it means on this machine when the tool is not installed.
	Missing string
	// Advice is what a person can do about it.
	Advice string
	// Gate is the hosts where the tool exists at all.
	Gate doctorPlatformGate
}

// doctorManagedTools is the catalogue: one entry per tool this installer
// provisions through an install unit, plus the tools the installer uses to do it.
//
// It is deliberately not the whole Brewfile. A tool that nothing in the
// installer's own flow references -- a personal CLI somebody added to the
// repository's list -- is not this installer's to diagnose, and reporting it
// would be the "owned by somebody else" mistake the dotfiles check refuses.
func doctorManagedTools() []doctorTool {
	installHint := func(pkg string) string {
		if pkg == "" {
			return "Install it with the package manager of this platform, or let the installer's own step do it."
		}
		return "Install it with the package manager of this platform, or let the installer's step do it: " + pkg + "."
	}
	return []doctorTool{
		{
			Name: "git", Unit: "clone", VersionArgs: []string{"--version"}, Required: true,
			Missing: "The run cannot check the repository out, so every step that reads it has nothing to work from.",
			Advice:  installHint("git"),
		},
		{
			Name: "curl", Unit: "deps", VersionArgs: []string{"--version"}, Required: true,
			Missing: "Every download the run makes -- Homebrew's installer, the fonts, the agent skills, this installer's own release -- goes through curl.",
			Advice:  installHint("curl"),
		},
		{
			Name: "unzip", Unit: "font", VersionArgs: []string{"-v"}, RequiredFor: "the font step",
			Missing: "The font step extracts its archive with unzip, so the Nerd Font is not installed although the step ran.",
			Advice:  installHint("unzip"),
		},
		{
			Name: "file", Unit: "deps", VersionArgs: []string{"--version"},
			Missing: "The dependency step installs it for the shells' own file-type tests; a shell function that asks what a file is answers differently without it.",
			Advice:  installHint("file"),
		},
		{
			Name: "fc-cache", Unit: "font", VersionArgs: []string{"-v"}, RequiredFor: "the font step",
			Missing: "The font the run installs is never indexed, so terminals that ask fontconfig for it do not see it.",
			Advice:  installHint("fontconfig"),
		},
		{
			Name: "zsh", Unit: "shell", VersionArgs: []string{"--version"},
			Missing: "The zsh configuration this installer writes is not read by anything.",
			Advice:  installHint("zsh"),
		},
		{
			Name: "fish", Unit: "shell", VersionArgs: []string{"--version"},
			Missing: "The fish configuration this installer writes is not read by anything.",
			Advice:  installHint("fish"),
		},
		{
			Name: "nu", Unit: "shell", VersionArgs: []string{"--version"},
			Missing: "The nushell configuration this installer writes is not read by anything.",
			Advice:  installHint("nushell"),
		},
		{
			Name: "tmux", Unit: "wm", VersionArgs: []string{"-V"},
			Missing: "The tmux configuration and its plugins have no multiplexer to run in.",
			Advice:  installHint("tmux"),
		},
		{
			Name: "zellij", Unit: "wm", VersionArgs: []string{"--version"},
			Missing: "The zellij configuration has no multiplexer to run in.",
			Advice:  installHint("zellij"),
		},
		{
			Name: "herdr", Unit: "wm", VersionArgs: []string{"--version"},
			Missing: "The herdr configuration and its plugins have no tool to run in.",
			Advice:  "Install herdr with the installer's own Herdr step, or from its releases.",
		},
		{
			Name: "nvim", Unit: "nvim", VersionArgs: []string{"--version"},
			Missing: "The Neovim configuration and its LazyVim plugin set are not loaded by anything.",
			Advice:  installHint("neovim"),
		},
		{
			Name: "starship", Unit: "shell", VersionArgs: []string{"--version"}, RequiredFor: "the shell step",
			Missing: "The prompt every shell this installer configures asks for is not there, so the prompt falls back to the shell's own.",
			Advice:  installHint("starship"),
		},
		{
			Name: "bat", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The shell aliases and the Neovim configuration that pipe through bat fall back to cat.",
			Advice:  installHint("bat"),
		},
		{
			Name: "eza", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The directory listing aliases fall back to ls.",
			Advice:  installHint("eza"),
		},
		{
			Name: "fd", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The find aliases and the pickers that use fd fall back to find.",
			Advice:  installHint("fd"),
		},
		{
			Name: "rg", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The search aliases and the Neovim keymaps that use ripgrep fall back to grep, or stop working.",
			Advice:  installHint("ripgrep"),
		},
		{
			Name: "fzf", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The fuzzy pickers in the shells' configuration have nothing to open.",
			Advice:  installHint("fzf"),
		},
		{
			Name: "jq", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The scripts that read JSON fall back to nothing.",
			Advice:  installHint("jq"),
		},
		{
			Name: "delta", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "git's diff pager is not there, so git falls back to its own pager.",
			Advice:  installHint("git-delta"),
		},
		{
			Name: "lazygit", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The git alias that opens lazygit has nothing to open.",
			Advice:  installHint("lazygit"),
		},
		{
			Name: "tree-sitter", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The parser builds the Neovim configuration triggers cannot run.",
			Advice:  installHint("tree-sitter"),
		},
		{
			Name: "gh", Unit: "toolset", VersionArgs: []string{"--version"},
			Missing: "The GitHub aliases and the PR workflow the agent skills describe have no command.",
			Advice:  installHint("gh"),
		},
		{
			Name: "gitleaks", Unit: "toolset", VersionArgs: []string{"version"},
			Missing: "The secret scan this repository configures for its hooks cannot run.",
			Advice:  installHint("gitleaks"),
		},
		{
			Name: "officecli", Unit: "officecli", VersionArgs: []string{"--version"},
			Missing: "The OfficeCLI step installed nothing that answers, so the agent skills that call it cannot.",
			Advice:  "Re-run the installer's OfficeCLI step, which installs it from its own releases.",
		},
		{
			Name: "brew", Unit: "homebrew", VersionArgs: []string{"--version"}, Required: true,
			Missing: "Homebrew is how this installer provisions the toolset on macOS: without it the toolset and the dependency step install nothing.",
			Advice:  "Let the installer's Homebrew step install it, or run the command it prints.",
			Gate:    doctorGateMac,
		},
		{
			Name: "brew", Unit: "homebrew", VersionArgs: []string{"--version"},
			Missing: "Homebrew is how this installer would provision the toolset here; without it the run falls back to the distribution's packages.",
			Advice:  "Install Homebrew, or let the run use the distribution's packages.",
			Gate:    doctorGateUnix,
		},
	}
}

// doctorToolsForPlatform is the catalogue cut down to the tools that exist on
// this host, in catalogue order. The name is not repeated: a platform's own
// entry for the same tool replaces the other one.
func doctorToolsForPlatform(scan *doctorScan) []doctorTool {
	var tools []doctorTool
	seen := map[string]bool{}
	for _, tool := range doctorManagedTools() {
		if !tool.Gate.applies(scan) {
			continue
		}
		if seen[tool.Name] {
			continue
		}
		seen[tool.Name] = true
		tools = append(tools, tool)
	}
	return tools
}

// doctorToolChecks is the tools category's catalogue.
func doctorToolChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "tools.managed",
			Category: doctorCategoryTools,
			Title:    "The tools this installer provisions are installed and answer",
			Required: true,
			Run:      doctorCheckToolsManaged,
		},
		{
			ID:       "tools.run",
			Category: doctorCategoryTools,
			Title:    "This installer's own dependencies are in place",
			Required: true,
			Run:      doctorCheckInstallerDependencies,
		},
	}
}

// doctorToolInstallState is what one managed tool's check found.
type doctorToolInstallState struct {
	// Installed is the path the tool resolved to, empty when nothing did.
	Installed string
	// Version is the answer of the tool's own version command.
	Version doctorVersion
	// Broken reports a tool that is there and did not answer.
	Broken bool
}

// doctorCheckToolsManaged answers, per tool, one of four things: it is installed
// and answers; it is installed and does not answer (broken); it is not installed
// and a run needs it (an error); it is not installed and it is a component
// somebody may have left out (a warning naming the unit).
//
// The login shell is the one tool whose requiredness is decided by this machine
// rather than by the catalogue: a machine whose $SHELL does not resolve has a
// broken environment, and that is an error even though the catalogue lists every
// shell as optional.
func doctorCheckToolsManaged(scan *doctorScan) []doctorFinding {
	tools := doctorToolsForPlatform(scan)
	loginShell := ""
	if shell := scan.getenv("SHELL"); shell != "" {
		loginShell = filepath.Base(shell)
	}

	var findings []doctorFinding
	installed, broken, missing := 0, 0, 0
	var versions []string
	for _, tool := range tools {
		lookup := scan.lookup(tool.Name)
		if !lookup.found() {
			missing++
			if reason := doctorMissingToolFinding(scan, tool, loginShell); reason != nil {
				findings = append(findings, *reason)
			}
			continue
		}

		version := scan.toolVersion(lookup.Path, tool.VersionArgs...)
		if version.Line == "" {
			broken++
			findings = append(findings, doctorBrokenToolFinding(scan, tool, lookup, version, loginShell))
			continue
		}
		installed++
		versions = append(versions, tool.Name+" "+doctorVersionNumber(version.Line))
	}

	if installed > 0 {
		evidence := fmt.Sprintf("%s installed, each answering its own version command: %s",
			doctorCounted(installed, "tool", "tools"), doctorPathList(versions, 8))
		impact := "The configuration this installer writes for these tools has something to configure."
		if missing > 0 || broken > 0 {
			// The clean half is still worth stating, and it belongs in the same
			// report as the rest: the counts above already named the rest.
			impact = fmt.Sprintf("These %d are in place; the %s above are not.",
				installed, doctorCounted(missing+broken, "finding", "findings"))
		}
		findings = append(findings, doctorFinding{
			Subject:  "Managed tools that are in place",
			Status:   doctorOK,
			Evidence: evidence,
			Impact:   impact,
			Advice:   "Nothing to do.",
		})
	}

	if len(findings) == 0 {
		return []doctorFinding{{
			Subject:  "Managed tools",
			Status:   doctorNotApplicable,
			Evidence: "the platform's catalogue holds no tool to look for",
			Impact:   "Nothing about the managed tools is known.",
			Advice:   "This is a defect in the doctor: every supported platform has a catalogue.",
		}}
	}
	return findings
}

// doctorMissingToolFinding is the finding for a tool that is not installed, or
// nil when its absence is not worth a row: a tool that this platform does not
// use, and that a run does not need, is not a finding.
func doctorMissingToolFinding(scan *doctorScan, tool doctorTool, loginShell string) *doctorFinding {
	required := tool.Required || (loginShell != "" && loginShell == tool.Name) || tool.RequiredFor != ""
	// A shell that is not the login shell and that no run needs is a shell the
	// user simply does not use: reporting it would be the falsely categorical
	// diagnosis the doctor refuses. They are named in one summary instead.
	if !required && tool.Unit == "shell" {
		return nil
	}

	status := doctorWarning
	requiredBy := tool.RequiredFor
	if tool.Required || (loginShell != "" && loginShell == tool.Name) {
		status = doctorError
		if tool.Required {
			requiredBy = "the installer's own run"
		} else {
			requiredBy = "the login shell this machine starts"
		}
	}
	advice := tool.Advice
	if status == doctorWarning {
		advice = "Install it with the installer's " + tool.Unit + " step if you want it, or leave it: nothing the run does depends on it."
	}
	impact := tool.Missing
	if requiredBy != "" {
		impact = fmt.Sprintf("Required by %s. %s", requiredBy, tool.Missing)
	}

	return &doctorFinding{
		Subject:  tool.Name + " is not installed",
		Status:   status,
		Evidence: fmt.Sprintf("no executable named %s is on PATH (%s)", tool.Name, doctorPathList(scan.pathEntries(), 3)),
		Impact:   impact,
		Advice:   advice,
	}
}

// doctorBrokenToolFinding is the finding for a tool that resolves and does not
// answer: the binary exists, runs, and fails or prints nothing. It is the
// "present but broken" case the brief asks to tell apart from "not installed".
func doctorBrokenToolFinding(scan *doctorScan, tool doctorTool, lookup doctorLookup, version doctorVersion, loginShell string) doctorFinding {
	required := tool.Required || (loginShell != "" && loginShell == tool.Name)
	status := doctorWarning
	if required {
		status = doctorError
	}
	evidence := fmt.Sprintf("%s is installed at %s, and `%s %s` reported nothing: %s",
		tool.Name, doctorRelHome(scan, lookup.Path), doctorRelHome(scan, lookup.Path),
		strings.Join(tool.VersionArgs, " "), doctorVersionProblem(version))
	impact := "The tool is present but does not work, which reads as installed to everything that only checks whether the file exists."
	if !required {
		impact = "Every configuration that calls this tool will fail or fall back, although the file is there."
	}
	return doctorFinding{
		Subject:  tool.Name + " is installed but does not answer",
		Status:   status,
		Evidence: evidence,
		Impact:   impact,
		Advice: fmt.Sprintf("Run it by hand to see the error: `%s %s`. Reinstall it with the installer's %s step.",
			doctorRelHome(scan, lookup.Path), strings.Join(tool.VersionArgs, " "), tool.Unit),
	}
}

// doctorVersionNumber pulls the version out of a tool's own banner so the
// report's summary reads like a version list rather than like a command log:
// "git version 2.44.0" beats a full line, and the full line is still what the
// per-tool evidence carries.
func doctorVersionNumber(line string) string {
	fields := strings.Fields(line)
	for _, field := range fields {
		if len(field) > 0 && field[0] >= '0' && field[0] <= '9' {
			return strings.TrimRight(field, ",;")
		}
	}
	if len(line) > 40 {
		return line[:40] + "…"
	}
	return line
}

// doctorCheckInstallerDependencies looks at what a run of this installer needs
// that is not one of the tools it provisions: a shell to run its scripts with,
// the platform's package manager, and the way it becomes root.
//
// The findings here are the ones that make the difference between "the toolset
// is not installed" and "the step that installs the toolset cannot run at all".
func doctorCheckInstallerDependencies(scan *doctorScan) []doctorFinding {
	var findings []doctorFinding

	shell := scan.lookup("bash")
	if !shell.found() {
		sh := scan.lookup("sh")
		if sh.found() {
			findings = append(findings, doctorFinding{
				Subject:  "The shell the run's scripts execute in",
				Status:   doctorWarning,
				Evidence: fmt.Sprintf("bash is not on PATH; sh resolves to %s", doctorRelHome(scan, sh.Path)),
				Impact:   "The generated scripts run under sh, which is enough for the steps themselves; a script that uses a bash-ism would fail.",
				Advice:   "Install bash where it is available (macOS and most distributions have it).",
			})
		} else {
			findings = append(findings, doctorFinding{
				Subject:  "The shell the run's scripts execute in",
				Status:   doctorError,
				Evidence: "neither bash nor sh resolves on PATH",
				Impact:   "No step can run: every installation step executes a shell script.",
				Advice:   "Install a POSIX shell (bash or sh); the machine cannot run scripts without one.",
			})
		}
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "The shell the run's scripts execute in",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("bash resolves to %s", doctorRelHome(scan, shell.Path)),
			Impact:   "Every step's script can run.",
			Advice:   "Nothing to do.",
		})
	}

	findings = append(findings, doctorPackageManagerFindings(scan)...)
	return findings
}

// doctorPackagesNeededForRoot names the ways this platform becomes root, in the
// order the installer itself tries them.
type doctorRootPath struct {
	// Name is the binary or condition the run needs.
	Name string
	// Why is what the run does with it.
	Why string
	// Optional marks the paths that are alternatives rather than requirements.
	Optional bool
}

// doctorPackageManagerFindings answers which package manager this distribution
// has, and whether the run can become root to use it. sudo is checked as a
// command, never as a password prompt: the installer has its own screen for the
// administrator password (admin_password.go), so a missing sudo is a finding and
// a sudo that would ask a question is not this check's business.
func doctorPackageManagerFindings(scan *doctorScan) []doctorFinding {
	if scan.isTermux() {
		if lookup := scan.lookup("pkg"); lookup.found() {
			return []doctorFinding{{
				Subject:  "The package manager",
				Status:   doctorOK,
				Evidence: fmt.Sprintf("pkg resolves to %s", doctorRelHome(scan, lookup.Path)),
				Impact:   "The dependency and toolset steps install through Termux's own package manager.",
				Advice:   "Nothing to do.",
			}}
		}
		return []doctorFinding{{
			Subject:  "The package manager",
			Status:   doctorError,
			Evidence: "pkg does not resolve on PATH, and this is Termux",
			Impact:   "The dependency step cannot install anything, and its packages are what the rest of the run is built on.",
			Advice:   "Termux always ships pkg; if it does not resolve, PATH is the first thing to repair.",
		}}
	}

	type manager struct {
		name string
		note string
	}
	managers := []manager{
		{"apt-get", "Debian and Ubuntu"},
		{"dnf", "Fedora and RHEL"},
		{"pacman", "Arch"},
		{"zypper", "openSUSE"},
		{"apk", "Alpine"},
		{"brew", "Homebrew"},
	}
	if scan.isMac() {
		managers = []manager{{"brew", "Homebrew"}}
	}

	var found []manager
	var paths []string
	for _, candidate := range managers {
		lookup := scan.lookup(candidate.name)
		if lookup.found() {
			found = append(found, candidate)
			paths = append(paths, candidate.name+" ("+candidate.note+") at "+doctorRelHome(scan, lookup.Path))
		}
	}

	switch {
	case len(found) == 0:
		return []doctorFinding{{
			Subject: "The package manager",
			Status:  doctorError,
			Evidence: fmt.Sprintf("none of %s resolves on PATH",
				doctorPathList([]string{"apt-get", "dnf", "pacman", "zypper", "apk", "brew"}, 6)),
			Impact: "The dependency and toolset steps have no way to install packages, so a run cannot provision the tools it needs.",
			Advice: "Install the package manager of this platform, or run the installer's steps that need no packages.",
		}}
	case scan.isMac():
		// brew is the only manager on macOS and it is already reported missing by
		// the managed tools check when it is absent, so the fact here is only that
		// the run has a manager to use.
		return []doctorFinding{{
			Subject:  "The package manager",
			Status:   doctorOK,
			Evidence: doctorPathList(paths, 3),
			Impact:   "The dependency and toolset steps install through it.",
			Advice:   "Nothing to do.",
		}}
	}

	findings := []doctorFinding{{
		Subject:  "The package manager",
		Status:   doctorOK,
		Evidence: doctorCounted(len(found), "package manager resolves", "package managers resolve") + ": " + doctorPathList(paths, 3),
		Impact:   "The dependency and toolset steps have a manager to install through.",
		Advice:   "Nothing to do.",
	}}

	// Becoming root. The installer boots a password prompt for sudo when it needs
	// one, so sudo has to exist and has to be allowed to prompt; whether it is
	// passwordless is the user's business, not a finding.
	if lookup := scan.lookup("sudo"); lookup.found() {
		findings = append(findings, doctorFinding{
			Subject:  "Becoming the administrator for packages",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("sudo resolves to %s", doctorRelHome(scan, lookup.Path)),
			Impact:   "The steps that install system packages can ask for the password on the installer's own screen.",
			Advice:   "Nothing to do.",
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "Becoming the administrator for packages",
			Status:   doctorWarning,
			Evidence: "sudo does not resolve on PATH",
			Impact:   "A step that installs system packages cannot elevate; the run either skips those steps or fails at the first one.",
			Advice:   "Install sudo (most distributions have it), or run the installer as a user with the right to install packages.",
		})
	}
	return findings
}
