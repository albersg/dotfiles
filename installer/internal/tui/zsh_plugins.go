package tui

import (
	"fmt"
	"os"
	"strings"
)

// The markers the zsh configuration this repository ships carries around the text
// the installer treats as the shell's plugin set.
//
// They exist because the set is what the file declares in one place and loads in
// two others -- Oh My Zsh's own initialization and the third-party plugin loaders --
// and because settings that are not the set's sit around and between those lines
// (the Powerlevel10k prompt, the carapace and fzf-tab completion blocks, the PATH
// configuration). Without a marker the patch would have to guess from a pattern, and
// either guess is a defect: too wide removes a setting the user asked to keep, too
// narrow leaves a plugin loaded on a machine that does not have it.
//
// TestTheZshPluginMarkersBoundTheTextTheInstallerTreatsAsTheSet holds them to exactly
// that: exactly one of each, in the order the stripper reads them, every declaration
// and loader of the set inside, and no setting of another tool inside.
const (
	zshPluginBlockStart = "# >>> dotfiles-plugins >>>"
	zshPluginBlockEnd   = "# <<< dotfiles-plugins <<<"
)

// zshPluginSetLeftOut is what the stripped block is replaced with: why the
// declarations and loaders are not there, in the file the user reads.
const zshPluginSetLeftOut = "# The Zsh plugin set was left out of this run: Oh My Zsh and the plugin\n" +
	"# packages its configuration loads are not installed, and none of them is named here.\n" +
	"# Run the installer again with the set on to get them back."

// zshPluginPackageNames are the package names that exist only to provide a plugin of
// the shell's set, and therefore the names a run that left the set out must not ask a
// package manager for. They are the names the platform columns of
// shellPlatformPackages carry. The loaders of carapace, fzf-tab and zsh-completions
// sit outside the marked block because the configuration already guards them (they sit
// among completion settings that are not the set's), so they stay in the file and turn
// themselves off when the run did not install them.
var zshPluginPackageNames = []string{
	"zsh-autosuggestions",
	"zsh-syntax-highlighting",
	"zsh-autocomplete",
	"zsh-completions",
	"fzf-tab",
	"carapace",
	"powerlevel10k",
	"zsh-theme-powerlevel10k",
}

// withoutZshPluginPackages returns the shell's package lists with every package that
// exists only to provide a plugin of the set removed, and the unavailable-name lists
// filtered the same way so the log cannot name a package the run decided not to
// request. It is what keeps a run that left the set out from installing the plugins
// the configuration it writes no longer loads.
func withoutZshPluginPackages(p platformPackages, unavailable []string) (platformPackages, []string) {
	drop := func(field string) string {
		kept := make([]string, 0, len(strings.Fields(field)))
		for _, name := range strings.Fields(field) {
			if !zshPluginPackage(name) {
				kept = append(kept, name)
			}
		}
		return strings.Join(kept, " ")
	}
	dropList := func(names []string) []string {
		if len(names) == 0 {
			return names
		}
		kept := make([]string, 0, len(names))
		for _, name := range names {
			if !zshPluginPackage(name) {
				kept = append(kept, name)
			}
		}
		return kept
	}

	p.Termux = drop(p.Termux)
	p.Brew = drop(p.Brew)
	p.Arch = drop(p.Arch)
	p.Fedora = drop(p.Fedora)
	p.Debian = drop(p.Debian)
	p.archUnavailable = dropList(p.archUnavailable)
	p.fedoraUnavailable = dropList(p.fedoraUnavailable)
	return p, dropList(unavailable)
}

// zshPluginPackage reports whether a package name exists only to provide a plugin of
// the shell's set.
func zshPluginPackage(name string) bool {
	for _, plugin := range zshPluginPackageNames {
		if name == plugin {
			return true
		}
	}
	return false
}

// leaveZshPluginsOut rewrites an installed .zshrc so that it sources no Oh My Zsh,
// declares no plugin and loads none of the set's plugin packages, which is what a run
// that left the plugin set out owes the machine: a configuration that still sources
// $ZSH/oh-my-zsh.sh errors on every shell start on a machine where Oh My Zsh was
// never installed, and one that loads a plugin path that is not there at best does
// nothing.
//
// It refuses rather than guesses when a marker is missing or unterminated, because
// the alternative is a configuration that breaks silently and a step that reports
// success over it.
func leaveZshPluginsOut(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	kept := make([]string, 0, len(data))
	inBlock := false
	sawBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case inBlock:
			if trimmed == zshPluginBlockEnd {
				inBlock = false
			}
		case trimmed == zshPluginBlockStart:
			inBlock, sawBlock = true, true
			kept = append(kept, zshPluginSetLeftOut)
		default:
			kept = append(kept, line)
		}
	}
	if inBlock {
		return fmt.Errorf("the installed .zshrc opens a block with %q and never closes it with %q", zshPluginBlockStart, zshPluginBlockEnd)
	}
	if !sawBlock {
		return fmt.Errorf("the installed .zshrc does not carry the block this installer writes (%q), so this run cannot tell which lines are the plugin set", zshPluginBlockStart)
	}

	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}
