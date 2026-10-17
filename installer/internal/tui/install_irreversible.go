package tui

// irreversibleEffects names what an install changes that no command on this
// machine can undo. It exists so the boundary is said where the user looks
// instead of being omitted: the scope decision put package installations, the
// Apple developer tools, the default-shell change and the two unpinned
// `curl | bash` installers outside the reversal this branch delivers, and a
// reader who is told that has a correct picture of what a revert means.
//
// The same list is what the failure screen and the review screen show, so the
// answer a person reads after a failure is the answer the command line gives.
func irreversibleEffects() []string {
	return []string{
		"packages installed by the platform package manager (apt, brew, pacman, dnf, pkg) stay installed: a revert removes no package",
		"the Xcode command line tools stay installed",
		"the default shell recorded for your account stays changed (/etc/shells and the login shell)",
		"anything installed by a script fetched from the internet (Claude Code, OpenCode) stays installed",
		"files this program wrote outside the configuration paths it backs up (fonts, caches, ~/.local/bin, ~/.pi/agent/skills) are not restored by a revert",
	}
}

// wayBackBeforeTheRun is what the review screen says, before anything has been
// written: where the copy of what the run replaces will be, the one command that
// puts it back, and what no command undoes. It is the answer a failure screen
// gives in the other tense, so a person reading either one learns the same thing
// about their machine.
func wayBackBeforeTheRun(backupEnabled bool) []string {
	lines := []string{}
	if backupEnabled {
		lines = append(lines,
			"Before it changes anything, the run copies every configuration it replaces into ~/.dotfiles-backup-<timestamp>.",
			"Put it back with dotfiles --revert, or Restore from Backup in the main menu.")
	} else {
		lines = append(lines,
			"Backups are off for this run, so nothing will copy the configuration it replaces.",
			"Afterwards, dotfiles --revert can remove what the run created but cannot put back what it replaced.")
	}
	return append(lines, notUndoneLine())
}

// wayBackAfterAFailure is what the failure screen says: where the copies of what
// the run replaced are, the one command that puts them back, and what no command
// undoes. A failure is where a person most needs to know what the run already
// changed and how to put it back -- and the screen used to offer retry or quit and
// never mention that a backup existed.
func wayBackAfterAFailure(backupDir string) []string {
	lines := []string{}
	if backupDir != "" {
		lines = append(lines, "What this run replaced is in "+backupDir)
		lines = append(lines, "Put it back with dotfiles --revert, or Restore from Backup in the main menu.")
	} else {
		lines = append(lines,
			"This run took no backup, so what it replaced cannot be put back.",
			"dotfiles --revert removes what it created and names what it cannot put back.")
	}
	return append(lines, notUndoneLine())
}

// notUndoneLine is irreversibleEffects as one row, for the screens that have a
// frame to fill: the categories are named, and the command that prints them one by
// one is named on the row above. The list itself stays the source: this row is a
// summary of it and never a second list to keep true.
func notUndoneLine() string {
	return "Not undone by a revert: packages, the Xcode tools, the login shell, downloaded scripts, and files " +
		"outside the paths it backs up."
}
