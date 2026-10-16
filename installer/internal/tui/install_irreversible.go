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
