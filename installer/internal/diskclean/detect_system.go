package diskclean

import (
	"context"
	"path/filepath"
	"strings"
)

// The system package managers. Their caches hold downloaded packages that can
// be fetched again, and every one of them ships the command that clears the
// cache -- which is the only mechanism used here, never a removal from the
// package manager's own directory.
//
// The commands that need administrator rights are marked as such. The screen
// attempts them with `sudo -n` and, when that is refused, prints the exact
// command instead; no password prompt is raised from the disk cleaner, and no
// package manager store is ever deleted from the filesystem.

// packageManagerMechanism is how a system package manager's cache is cleaned.
// The command is the manager's own, and it is the only mechanism used: a store
// the package manager owns is never removed from the filesystem, so a manager
// that is not installed leaves the item measured and refused rather than
// cleanable.
func packageManagerMechanism(program string, requiresRoot bool, args ...string) Mechanism {
	if program == "" {
		return Mechanism{
			Kind: MechanismReportOnly,
			Reason: "the package manager is not on PATH, and its store is never removed from the " +
				"filesystem; run its own clean command from a terminal that has it",
		}
	}
	return Mechanism{Kind: MechanismTool, Program: program, Args: args, RequiresRoot: requiresRoot}
}

// detectApt finds the downloaded .deb archives apt keeps. On Termux the cache
// lives under $PREFIX and the command needs no administrator rights.
func detectApt(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "apt-get", "apt")
	var cachePath string
	requiresRoot := true
	if opts.Facts.Termux && opts.Facts.Prefix != "" {
		cachePath = filepath.Join(opts.Facts.Prefix, "var", "cache", "apt", "archives")
		requiresRoot = false
	} else {
		cachePath = filepath.Join(systemRoot(opts.Facts), "var", "cache", "apt", "archives")
	}
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the apt package cache directory exists"
	switch {
	case len(paths) > 0 && program != "":
		reason = program + " is on PATH and its package cache exists at " + cachePath
	case len(paths) > 0:
		reason = "the apt package cache exists at " + cachePath + ", though apt is not on PATH"
	default:
		reason = program + " is on PATH, and its package cache is empty or has not been created yet"
	}

	return []Item{{
		ID:        "apt-archives",
		Tool:      "apt",
		Title:     "The apt package cache",
		Detail:    "the .deb packages apt downloaded. `apt-get clean` removes them from apt's own cache directory; a package is downloaded again if it is installed or upgraded later.",
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: packageManagerMechanism(program, requiresRoot, "clean"),
	}}
}

// detectDnf finds dnf's downloaded packages and metadata cache.
func detectDnf(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "dnf")
	cachePath := filepath.Join(systemRoot(opts.Facts), "var", "cache", "dnf")
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the dnf cache directory exists"
	if len(paths) > 0 && program != "" {
		reason = "dnf is on PATH and its cache exists at " + cachePath
	} else if len(paths) > 0 {
		reason = "the dnf cache exists at " + cachePath + ", though dnf is not on PATH"
	} else {
		reason = "dnf is on PATH, and its cache is empty or has not been created yet"
	}

	return []Item{{
		ID:        "dnf-cache",
		Tool:      "dnf",
		Title:     "The dnf package cache",
		Detail:    "the packages and repository metadata dnf downloaded. `dnf clean all` removes both; the metadata is fetched again on the next command that needs it.",
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: packageManagerMechanism(program, true, "clean", "all"),
	}}
}

// detectPacman finds pacman's package cache.
func detectPacman(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "pacman")
	cachePath := filepath.Join(systemRoot(opts.Facts), "var", "cache", "pacman", "pkg")
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the pacman package cache directory exists"
	if len(paths) > 0 && program != "" {
		reason = "pacman is on PATH and its package cache exists at " + cachePath
	} else if len(paths) > 0 {
		reason = "the pacman package cache exists at " + cachePath + ", though pacman is not on PATH"
	} else {
		reason = "pacman is on PATH, and its package cache is empty or has not been created yet"
	}

	return []Item{{
		ID:        "pacman-packages",
		Tool:      "pacman",
		Title:     "The pacman package cache",
		Detail:    "the package files pacman downloaded. `pacman -Sc` removes the versions that are not installed; the version an installed package is running stays, so a downgrade to it keeps working. `-Scc` would remove everything, and this utility does not run it.",
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: packageManagerMechanism(program, true, "-Sc", "--noconfirm"),
	}}
}

// detectBrew finds Homebrew's download cache, which `brew --cache` names.
func detectBrew(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "brew")
	if program == "" {
		return nil
	}
	cachePath := ""
	if output, err := runTool(ctx, probe, opts, program, "--cache"); err == nil {
		cachePath = strings.TrimSpace(string(output))
	}
	paths := existingPaths(probe, cachePath)

	return []Item{{
		ID:        "brew-cache",
		Tool:      "brew",
		Title:     "The Homebrew cache",
		Detail:    "the bottles and source archives Homebrew downloaded. `brew cleanup` removes the cache and the old versions of installed formulae; the current version of each stays, and a formula that is installed again downloads again.",
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    "brew is on PATH and `brew --cache` named " + cachePath,
		Paths:     paths,
		Mechanism: Mechanism{Kind: MechanismTool, Program: program, Args: []string{"cleanup"}},
	}}
}
