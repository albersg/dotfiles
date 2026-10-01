package main

import (
	"flag"
	"strings"
	"testing"
)

// TestHelpDocumentsEveryFlag guards the class of defect where a flag is added to
// the binary and never reaches the help. It happened with --no-mouse and
// --no-sprite: both shipped, both were read by the model, and neither was
// mentioned in `dotfiles --help`, which is the only place a user on an odd
// terminal would go looking for them.
//
// The list comes from registerFlags rather than from a written-out copy, so a
// flag cannot be added without the help being asked for it.
func TestHelpDocumentsEveryFlag(t *testing.T) {
	fs := flag.NewFlagSet("help-check", flag.ContinueOnError)
	registerFlags(fs, &cliFlags{})

	var names []string
	fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	if len(names) == 0 {
		t.Fatal("no flags were registered, so this guard proves nothing")
	}

	var missing []string
	for _, name := range names {
		if !helpDocumentsFlag(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("dotfiles --help does not mention %d of %d flags: %v\n"+
			"The help is the only place a user can find a switch, so a flag that is not in it is a flag "+
			"nobody has: add it to helpText.", len(missing), len(names), missing)
	}
}

// helpDocumentsFlag reports whether the help shows the flag. A long flag is
// looked up as --name; a one-letter shorthand is looked up as the "-x," the help
// writes for it, so that "-v" is not satisfied by the word "verbose".
func helpDocumentsFlag(name string) bool {
	if len(name) > 1 {
		return strings.Contains(helpText, "--"+name)
	}
	return strings.Contains(helpText, "-"+name+",")
}

// TestHelpDocumentsTheDisplayGates fails when the help stops explaining a switch
// a user needs when the drawing is wrong.
//
// This is a written list on purpose, and not every DOTFILES_* variable the source
// reads: most of them are internal wiring - test mode, dry run, the WSL host, the
// Alpine release pin - and demanding a place in the help for each would fill it
// with switches nobody sets by hand. The line is "a person needs this to fix what
// they are looking at", and every entry here is on that side of it. A new gate
// arrives with a flag, and TestHelpDocumentsEveryFlag covers that.
func TestHelpDocumentsTheDisplayGates(t *testing.T) {
	gates := []string{
		"--no-anim", "DOTFILES_ANIM",
		"--no-mouse", "DOTFILES_MOUSE",
		"--no-sprite", "DOTFILES_SPRITE",
		"DOTFILES_SYNC",
		"DOTFILES_VERBOSE",
	}

	for _, gate := range gates {
		if !strings.Contains(helpText, gate) {
			t.Errorf("dotfiles --help no longer mentions %s, which is one of the switches a user needs "+
				"when the terminal does not draw the installer well.", gate)
		}
	}
}
