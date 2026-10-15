package tui

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The last piece of the fish/zsh parity mandate: the two hooks that run when the
// directory changes -- a project's virtual environment and its tasks -- and the three
// commands that keep secrets encrypted. They are guarded the way the four commands were,
// by comparing what the two shells *do* rather than how they spell it. A hook name, the
// candidates an environment may live under, the boundary of the search, the files a
// project's tasks are read from and the sentence a mistake prints are policy;
// `add-zsh-hook chpwd` against `--on-variable PWD`, and `export` against `set -gx`, are
// spelling, and a guard over the spelling would forbid the translation it exists to check.

// secretsGateName is the one function every secrets command starts with, in both shells.
const secretsGateName = "_dotfiles_secrets_ready"

var (
	zshDirectoryHook   = regexp.MustCompile(`^add-zsh-hook\s+chpwd\s+(\S+)$`)
	fishDirectoryHook  = regexp.MustCompile(`^function\s+(\S+)\s+--on-variable\s+PWD$`)
	venvCandidateLine  = regexp.MustCompile(`\bfor candidate in (.+)$`)
	venvActivationGlob = regexp.MustCompile(`\bbin/(activate[.a-z]*)`)
	taskSourceName     = regexp.MustCompile(`\b(justfile|Justfile|Makefile|makefile|package\.json|Taskfile\.ya?ml)\b`)
	zshSopsFileSetting = regexp.MustCompile(`^typeset\s+-g\s+DOTFILES_SOPS_FILE=(\S+)$`)
	fishSopsFileEntity = regexp.MustCompile(`^set\s+-g\s+DOTFILES_SOPS_FILE\s+(\S+)$`)
	gitInvocation      = regexp.MustCompile(`\bgit\s+([a-z][a-z-]*)((?:\s+--?[^\s;&|]+)*)`)
)

func TestFishDeclaresTheSameHooksAndTheSameSecretsCommands(t *testing.T) {
	zshHooks := directoryChangeHooks(t, zshShippedConfig, "zsh")
	fishHooks := directoryChangeHooks(t, fishShippedConfig, "fish")
	if len(zshHooks) < 2 {
		t.Fatalf("read %d directory-change hooks from %s (%v): this guard is not looking at the file it names", len(zshHooks), zshShippedConfig, zshHooks)
	}
	if strings.Join(zshHooks, ", ") != strings.Join(fishHooks, ", ") {
		t.Errorf("zsh runs %v when the directory changes and fish runs %v, so entering a project does something different depending on which shell is running", zshHooks, fishHooks)
	}

	names := append(append([]string{}, zshHooks...), secretsGateName, "env-edit", "env-load", "env-export")
	for _, name := range names {
		if readShellFunction(t, fishShippedConfig, "fish", name) == nil {
			t.Errorf("%s declares no %s, so fish is missing a hook or a command zsh has: %v", fishShippedConfig, name, names)
		}
	}
}

func TestTheVirtualEnvironmentSearchIsBoundedTheSameWay(t *testing.T) {
	zsh := readShellFunction(t, zshShippedConfig, "zsh", "_dotfiles_activate_venv")
	fish := readShellFunction(t, fishShippedConfig, "fish", "_dotfiles_activate_venv")
	if zsh == nil || fish == nil {
		t.Fatalf("the hook is missing on one side: zsh %d lines, fish %d lines", len(zsh), len(fish))
	}

	zshCandidates, fishCandidates := venvCandidates(zsh), venvCandidates(fish)
	if len(zshCandidates) == 0 {
		t.Fatalf("read no candidate list from %s: this guard is not looking at the hook it names", zshShippedConfig)
	}
	if strings.Join(zshCandidates, " ") != strings.Join(fishCandidates, " ") {
		t.Errorf("zsh looks for %v and fish looks for %v, so the same directory is a Python project under one shell and an ordinary one under the other", zshCandidates, fishCandidates)
	}

	for _, want := range []string{".git", "_DOTFILES_VENV", "VIRTUAL_ENV", "HOME", "venv: "} {
		zshHas, fishHas := bodyMentions(zsh, want), bodyMentions(fish, want)
		if !zshHas || !fishHas {
			t.Errorf("the hook must name %q in both shells (zsh: %t, fish: %t): .git and HOME are what bound the search, _DOTFILES_VENV compared against VIRTUAL_ENV is what keeps an environment the user activated by hand from being deactivated, and `venv: ` is the line that says one was activated", want, zshHas, fishHas)
		}
	}

	// The one deliberate difference, held rather than assumed equal: zsh sources the
	// environment's POSIX `bin/activate`, and fish cannot read that file at all. fish
	// requires and sources `bin/activate.fish`, which the same `python3 -m venv` writes
	// beside it, so an environment without one is left alone rather than half-activated.
	if got := venvActivationScript(fish); got != "activate.fish" {
		t.Errorf("fish activates through %q; it has to be activate.fish, because the POSIX activate beside it cannot be sourced from fish", got)
	}
	if got := venvActivationScript(zsh); got != "activate" {
		t.Errorf("zsh activates through %q, and zsh has always sourced the POSIX activate", got)
	}
}

func TestBothShellsAnnounceTheSameTaskSources(t *testing.T) {
	zsh := readShellFunction(t, zshShippedConfig, "zsh", "_dotfiles_show_tasks")
	fish := readShellFunction(t, fishShippedConfig, "fish", "_dotfiles_show_tasks")
	if zsh == nil || fish == nil {
		t.Fatalf("the hook is missing on one side: zsh %d lines, fish %d lines", len(zsh), len(fish))
	}

	zshSources, fishSources := taskSources(hookCode(zsh)), taskSources(hookCode(fish))
	if len(zshSources) == 0 {
		t.Fatalf("read no task source from %s: this guard is not looking at the hook it names", zshShippedConfig)
	}
	if strings.Join(zshSources, ", ") != strings.Join(fishSources, ", ") {
		t.Errorf("zsh reads a project's tasks from %v and fish from %v, so the same project announces its tasks under one shell and stays silent under the other", zshSources, fishSources)
	}

	for _, want := range []string{"DOTFILES_NO_TASKS", "just --list", "task --list", "jq", "npm run", "awk", "head -20"} {
		zshHas, fishHas := bodyMentions(zsh, want), bodyMentions(fish, want)
		if !zshHas || !fishHas {
			t.Errorf("both hooks must name %q (zsh: %t, fish: %t): it is part of announcing the tasks a project declares without running any of them", want, zshHas, fishHas)
		}
	}
}

func TestBothShellsMirrorTheSameSecretsDefaults(t *testing.T) {
	shells := []struct{ name, path, pattern string }{}
	zshLines := hooksShippedLines(t, zshShippedConfig)
	fishLines := hooksShippedLines(t, fishShippedConfig)

	zshSopsFile := sopsFileSetting(zshLines, zshSopsFileSetting)
	if zshSopsFile != "secrets.env" {
		t.Fatalf("read DOTFILES_SOPS_FILE as %q from %s: this guard is not looking at the file it names", zshSopsFile, zshShippedConfig)
	}
	if fishSopsFile := sopsFileSetting(fishLines, fishSopsFileEntity); fishSopsFile != zshSopsFile {
		t.Errorf("zsh encrypts %q by default and fish %q, so the same project needs a different command depending on the shell", zshSopsFile, fishSopsFile)
	}

	for _, want := range []string{"SOPS_AGE_KEY_FILE", ".config/sops/age/keys.txt", "age-keygen"} {
		zshHas, fishHas := bodyMentions(zshLines, want), bodyMentions(fishLines, want)
		if !zshHas || !fishHas {
			t.Errorf("both configurations must name %q (zsh: %t, fish: %t): where the age key lives, and the command that creates it, are what a user is told when it is missing", want, zshHas, fishHas)
		}
	}

	shells = append(shells,
		struct{ name, path, pattern string }{"zsh", zshShippedConfig, "zsh"},
		struct{ name, path, pattern string }{"fish", fishShippedConfig, "fish"},
	)
	for _, shell := range shells {
		for _, name := range []string{"env-edit", "env-load", "env-export"} {
			body := readShellFunction(t, shell.path, shell.pattern, name)
			if len(body) == 0 {
				t.Errorf("%s declares no %s, so this guard has nothing to read", shell.name, name)
				continue
			}
			if got := firstStatement(body); !strings.HasPrefix(got, secretsGateName) {
				t.Errorf("%s's %s starts with %q; every secrets command starts with %s, so a missing tool or a missing key is reported once, with the command that fixes it", shell.name, name, got, secretsGateName)
			}
		}
	}
}

func TestTheSecretsCommandsAskGitTheSameQuestions(t *testing.T) {
	for _, name := range []string{"env-load", "env-export"} {
		zsh := readShellFunction(t, zshShippedConfig, "zsh", name)
		fish := readShellFunction(t, fishShippedConfig, "fish", name)
		if zsh == nil || fish == nil {
			t.Fatalf("one side declares no %s: zsh %d lines, fish %d lines", name, len(zsh), len(fish))
		}
		if name == "env-load" {
			zshGit, fishGit := gitInvocations(zsh), gitInvocations(fish)
			if len(zshGit) < 2 {
				t.Fatalf("read %d git invocations from %s's %s: the guard expects the two that ask whether this is a work tree and whether the destination is ignored", len(zshGit), zshShippedConfig, name)
			}
			if strings.Join(zshGit, " AND ") != strings.Join(fishGit, " AND ") {
				t.Errorf("env-load asks git %v under zsh and %v under fish, so a plaintext file is refused under one shell and written under the other", zshGit, fishGit)
			}
			if !bodyMentions(fish, "rm -f") {
				t.Errorf("fish's env-load does not remove the destination when decryption fails, so a failed decrypt leaves an empty file that looks like a secrets file")
			}
			if !bodyMentions(fish, "secrets: wrote ") {
				t.Errorf("fish's env-load does not report `secrets: wrote `, so the user cannot tell a successful decrypt from a silent one")
			}
		}
		zshSops, fishSops := sopsInvocations(zsh), sopsInvocations(fish)
		if strings.Join(zshSops, " AND ") != strings.Join(fishSops, " AND ") {
			t.Errorf("%s runs sops as %v under zsh and %v under fish, so the two shells decrypt differently", name, zshSops, fishSops)
		}
	}
}

func TestTheSecretsMessagesAreTheSameInBothShells(t *testing.T) {
	for _, name := range []string{"env-edit", "env-load", "env-export"} {
		zsh := errorMessages(readShellFunction(t, zshShippedConfig, "zsh", name), "zsh")
		fish := errorMessages(readShellFunction(t, fishShippedConfig, "fish", name), "fish")
		if len(zsh) == 0 {
			t.Fatalf("read no message from %s's %s: this guard is not looking at the command it names", zshShippedConfig, name)
		}
		if strings.Join(zsh, " ") != strings.Join(fish, " ") {
			t.Errorf("%s tells the user %v under zsh and %v under fish, so the same mistake reads differently depending on which shell is running", name, zsh, fish)
		}
	}
}

func TestTheHooksAndTheSecretsAreDeclaredOutsideTheGeneratedBlock(t *testing.T) {
	lines := hooksShippedLines(t, fishShippedConfig)
	sentinel, loader := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(line, "# <<< dotfiles-theme") {
			sentinel = i
		}
		if strings.HasPrefix(line, "# Files in this directory belong to the user") {
			loader = i
		}
	}
	if sentinel < 0 || loader < 0 {
		t.Fatalf("found the theme block's closing sentinel at %d and the drop-in loader at %d in %s: this guard is not looking at the file it names, or the file no longer has the shape it guards", sentinel, loader, fishShippedConfig)
	}
	if sentinel > loader {
		t.Fatalf("the theme block closes at %d, after the drop-in loader at %d: the blocks have been reordered", sentinel, loader)
	}

	names := append(directoryChangeHooks(t, fishShippedConfig, "fish"), secretsGateName, "env-edit", "env-load", "env-export")
	for _, name := range names {
		at := -1
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "function "+name) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Errorf("%s declares no %s", fishShippedConfig, name)
			continue
		}
		if at < sentinel || at > loader {
			t.Errorf("%s declares %s on line %d, outside the region between the closing sentinel (line %d) and the drop-in loader (line %d): a hook inside the generated block is overwritten by the theme switch, and one after the loader is overridden by the user's own drop-ins without either saying so", fishShippedConfig, name, at+1, sentinel+1, loader+1)
		}
	}
}

// directoryChangeHooks returns the names each shell runs when the working directory
// changes. zsh registers them with add-zsh-hook, fish declares the handler and the
// variable it watches in one line.
func directoryChangeHooks(t *testing.T, path, shell string) []string {
	t.Helper()
	pattern := zshDirectoryHook
	if shell == "fish" {
		pattern = fishDirectoryHook
	}
	names := []string{}
	for _, line := range hooksShippedLines(t, path) {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			names = append(names, match[1])
		}
	}
	return names
}

func hooksShippedLines(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.Split(string(body), "\n")
}

// hookCode drops whole-line comments, so a filename named in prose cannot pass for one
// the hook actually looks at.
func hookCode(body []string) []string {
	out := []string{}
	for _, line := range body {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func bodyMentions(body []string, want string) bool {
	return strings.Contains(strings.Join(body, "\n"), want)
}

// venvCandidates reads the list of directory names the hook will look for. zsh ends its
// for-loop line with `; do` and fish does not, so the list stops at the first keyword.
func venvCandidates(body []string) []string {
	for _, line := range hookCode(body) {
		match := venvCandidateLine.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		out := []string{}
		for _, field := range strings.Fields(match[1]) {
			// zsh writes the list as `.venv venv .env; do` and fish as `.venv venv .env`:
			// the semicolon arrives glued to the last candidate, not as a word of its own.
			field = strings.TrimSuffix(field, ";")
			if field == "" || field == "do" {
				break
			}
			out = append(out, field)
		}
		return out
	}
	return nil
}

func venvActivationScript(body []string) string {
	if match := venvActivationGlob.FindStringSubmatch(strings.Join(hookCode(body), "\n")); match != nil {
		return match[1]
	}
	return ""
}

func taskSources(body []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, line := range body {
		for _, match := range taskSourceName.FindAllString(line, -1) {
			if !seen[match] {
				seen[match] = true
				out = append(out, match)
			}
		}
	}
	sort.Strings(out)
	return out
}

// gitInvocations reads the questions a command asks git: the subcommand and its flags,
// without paths or redirections, which is the part that decides whether plaintext is
// refused.
func gitInvocations(body []string) []string {
	out := []string{}
	for _, line := range hookCode(body) {
		// Quoted strings are removed first: `secrets: git does not ignore …` is a message, and
		// read as code it looks like a third question asked of git. This class of mistake --
		// a reader that takes in more than the thing it is reading about -- was made twice in
		// the previous stage's guards and twice in this one, and stripping the strings is
		// where it stops.
		for _, match := range gitInvocation.FindAllStringSubmatch(unquoted(line), -1) {
			out = append(out, strings.Join(strings.Fields("git "+match[1]+match[2]), " "))
		}
	}
	sort.Strings(out)
	return out
}

// unquoted removes the quoted strings from a line, so a message that names a command is
// not read as that command being run.
func unquoted(line string) string {
	for _, pattern := range []string{`'[^']*'`, `"[^"]*"`} {
		line = regexp.MustCompile(pattern).ReplaceAllString(line, "")
	}
	return line
}

// sopsInvocations reads the sops command a shell runs -- the command and the words that
// belong to it, stopping at the first path, redirection or shell keyword, so that zsh's
// `if ! sops -d …` and fish's `if not sops -d …` compare as the same invocation instead of
// as each shell's own way of writing `if`.
func sopsInvocations(body []string) []string {
	out := []string{}
	for _, line := range hookCode(body) {
		fields := strings.Fields(line)
		for i, field := range fields {
			if strings.Trim(field, `"'`) != "sops" {
				continue
			}
			invocation := []string{"sops"}
			for _, next := range fields[i+1:] {
				if !commandWord(next) {
					break
				}
				invocation = append(invocation, strings.Trim(next, `"'`))
			}
			out = append(out, strings.Join(invocation, " "))
		}
	}
	sort.Strings(out)
	return out
}

// commandWord answers whether a word belongs to a command rather than being an argument
// to it: a path, a redirection or a shell keyword ends the invocation, which is what keeps
// `--output-type dotenv` inside the comparison while the file it reads is left out of it.
func commandWord(word string) bool {
	if word == "" || strings.Contains(word, "$") ||
		strings.HasPrefix(word, ">") || strings.HasPrefix(word, "<") ||
		strings.HasPrefix(word, "|") || strings.HasPrefix(word, "&") {
		return false
	}
	switch word {
	case "if", "then", "else", "elif", "fi", "do", "done", "and", "or", "not", "!", "end", ";;":
		return false
	}
	return true
}

func sopsFileSetting(lines []string, pattern *regexp.Regexp) string {
	for _, line := range lines {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			return strings.Trim(match[1], `"'`)
		}
	}
	return ""
}

func firstStatement(body []string) string {
	for _, line := range hookCode(body) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
