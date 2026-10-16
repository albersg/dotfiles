package tui

import (
	"fmt"
	"os"
	"strings"
)

// The markers the Tmux configuration this repository ships carries around the text
// the installer treats as the plugin set.
//
// They exist because the set is spread across the file -- the declarations and the
// options that belong to them at the top, the line that runs TPM near the bottom --
// and because key bindings sit between them. Without a marker the patch would have
// to guess from a pattern, and either guess is a defect: too wide removes a keymap
// from the user's configuration, too narrow leaves a plugin declared on a machine
// that has none.
//
// TestThePluginMarkersBoundTheTextTheInstallerTreatsAsTheSet holds them to exactly
// that: every declaration is inside the first block, the line that runs TPM is
// inside the second, and no key binding is inside either.
const (
	tmuxPluginBlockStart = "# >>> dotfiles-plugins >>>"
	tmuxPluginBlockEnd   = "# <<< dotfiles-plugins <<<"
	tmuxTPMBlockStart    = "# >>> dotfiles-tpm >>>"
	tmuxTPMBlockEnd      = "# <<< dotfiles-tpm <<<"
)

// tmuxPluginSetLeftOut is what the stripped block is replaced with: why the
// declarations are not there, in the file the user reads.
const tmuxPluginSetLeftOut = "# The Tmux plugin set was left out of this run: no plugin is declared here and\n" +
	"# TPM is not installed. Run the installer again with the set on to get them back."

// leaveTmuxPluginsOut rewrites an installed tmux.conf so that it declares no plugin
// and runs no TPM, which is what a run that left the plugin set out owes the
// machine: a configuration that still runs a checkout nobody installed asks tmux to
// run a file that is not there, on every single start.
//
// It refuses rather than guesses when a marker is missing or unterminated, because
// the alternative is a configuration that breaks silently and a step that reports
// success over it.
func leaveTmuxPluginsOut(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	kept := make([]string, 0, len(data))
	closing := ""
	sawPlugins, sawTPM := false, false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case closing != "":
			if trimmed == closing {
				closing = ""
			}
		case trimmed == tmuxPluginBlockStart:
			closing, sawPlugins = tmuxPluginBlockEnd, true
			kept = append(kept, tmuxPluginSetLeftOut)
		case trimmed == tmuxTPMBlockStart:
			closing, sawTPM = tmuxTPMBlockEnd, true
		default:
			kept = append(kept, line)
		}
	}
	if closing != "" {
		return fmt.Errorf("the installed tmux.conf opens a block with %q and never closes it", closing)
	}
	if !sawPlugins || !sawTPM {
		return fmt.Errorf("the installed tmux.conf does not carry the blocks this installer writes (%q and %q), so this run cannot tell which lines are the plugin set",
			tmuxPluginBlockStart, tmuxTPMBlockStart)
	}

	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}
