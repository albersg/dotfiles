package tui

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// The two configurations this guard compares. They are the files themselves, not
// a copy of what they are believed to contain.
const (
	zshShippedConfig  = "../../../dotfiles-zsh/.zshrc"
	fishShippedConfig = "../../../dotfiles-fish/fish/config.fish"
)

// configAlias is one `alias <name>=<command>` line, in the order it was read.
type configAlias struct {
	name    string
	command string
}

// readConfigAliases returns every alias a shell configuration declares.
//
// Both shells spell an alias the same way — `alias name='command'` — so one
// reader serves both, and the quote handling below is what keeps a command that
// contains the other kind of quote (fish's `fzfbat`, whose preview is
// double-quoted inside a single-quoted command) from ending the alias early.
func readConfigAliases(t *testing.T, path string) []configAlias {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var aliases []configAlias
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "alias ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "alias "))
		eq := strings.Index(rest, "=")
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(rest[:eq])
		if name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		value := strings.TrimSpace(rest[eq+1:])
		if len(value) < 2 {
			continue
		}
		quote := value[0]
		if quote != '\'' && quote != '"' {
			continue
		}
		end := strings.LastIndexByte(value, quote)
		if end <= 0 {
			continue
		}
		aliases = append(aliases, configAlias{name: name, command: value[1:end]})
	}
	return aliases
}

// ezaAliases keeps the aliases whose command is `eza`, keyed by alias name, with
// the command's own words sorted so two spellings of the same flags compare
// equal and a different flag set does not.
//
// Only the flags are compared. The guard is about the policy the two shells have
// to agree on — which tool lists files, with which flags — and not about the text
// of two configurations written in two different languages.
func ezaAliases(aliases []configAlias) map[string]string {
	out := map[string]string{}
	for _, alias := range aliases {
		words := strings.Fields(alias.command)
		if len(words) == 0 || words[0] != "eza" {
			continue
		}
		flags := append([]string(nil), words[1:]...)
		sort.Strings(flags)
		out[alias.name] = strings.Join(flags, " ")
	}
	return out
}

// TestFishListingAliasesMatchTheOnesZshDefines is the guard for the listing half
// of the fish/zsh asymmetry.
//
// `dotfiles-zsh/.zshrc` aliases `ls` to `eza --icons --group-directories-first`
// and adds `ll`, `la`, `lt` and `tree`; `dotfiles-fish/fish/config.fish` aliased
// `ls` to `gls --color=auto` and added nothing. `eza` is on the fish `PATH` on
// every host this installer supports — it is named by the Brewfile, by the
// profiles and by the Arch and Fedora columns of `shellPlatformPackages`, and
// Debian takes it from the Homebrew this installer puts in place — so fish had
// the tool and never called it, and the listing printed no icons. That is
// indistinguishable from the font being wrong, which is how it was reported.
//
// A flag changed in one shell and not the other is the same defect returning one
// flag at a time, which is why the comparison is the whole flag set rather than
// only the presence of the alias.
func TestFishListingAliasesMatchTheOnesZshDefines(t *testing.T) {
	zsh := ezaAliases(readConfigAliases(t, zshShippedConfig))
	fish := ezaAliases(readConfigAliases(t, fishShippedConfig))

	// A guard that read no alias at all would pass while checking nothing.
	if len(zsh) == 0 {
		t.Fatalf("read no `eza` alias from %s: this guard is not looking at the file it names", zshShippedConfig)
	}
	if len(fish) == 0 {
		t.Fatalf("read no `eza` alias from %s: fish lists files without eza, so its `ls` prints no icons while zsh's does", fishShippedConfig)
	}

	for name, want := range zsh {
		got, ok := fish[name]
		if !ok {
			t.Errorf("zsh aliases `%s` to `eza %s`, and fish has no such alias, so fish's listing is not the listing zsh's is", name, want)
			continue
		}
		if got != want {
			t.Errorf("alias `%s` disagrees between the shells: zsh runs `eza %s`, fish runs `eza %s`", name, want, got)
		}
	}
	for name := range fish {
		if _, ok := zsh[name]; !ok {
			t.Errorf("fish aliases `%s` to eza and zsh does not: the two shells are meant to list files the same way", name)
		}
	}
}

// TestFishGuardsItsListingAliasesOnEza is the other half of D1's guard, and it is
// the half about a host that cannot resolve the tool.
//
// The repository's rule for an alias whose tool may be missing is to ask whether
// it resolves: zsh's `trip` and `btop` aliases are declared only inside
// `command -v` tests for exactly this reason. fish's listing aliases have to
// follow it, because the previous `alias ls='gls --color=auto'` did not — and
// `gls` is a Homebrew-only name, so on a Debian host with no Homebrew the alias
// failed on every call. This guard rejects the unguarded form.
func TestFishGuardsItsListingAliasesOnEza(t *testing.T) {
	body, err := os.ReadFile(fishShippedConfig)
	if err != nil {
		t.Fatalf("reading %s: %v", fishShippedConfig, err)
	}
	config := string(body)

	guard := strings.Index(config, "command -q eza")
	if guard < 0 {
		t.Fatalf("%s declares eza aliases without asking whether eza resolves, so a host that has not got it gets an alias that fails on every call", fishShippedConfig)
	}

	for name := range ezaAliases(readConfigAliases(t, fishShippedConfig)) {
		at := strings.Index(config, "alias "+name+"=")
		if at < 0 {
			t.Errorf("alias `%s` was read but not found again in %s", name, fishShippedConfig)
			continue
		}
		if at < guard {
			t.Errorf("alias `%s` is declared above the `command -q eza` guard, so it is declared on a host where eza does not resolve", name)
		}
	}
}

// TestFishKeepsAColourLsWithoutEza rejects the other way of getting the guard
// wrong: guarding the listing by deleting the `ls` fish already had.
//
// A host where the installer could not put eza in place must still get a coloured
// listing, which is exactly what `ls --color=auto` on macOS and `gls --color=auto`
// on Linux were for. Both are required rather than one of them: keeping only the
// macOS branch is a Linux host losing its colour silently, which is the shape of
// defect this front exists to remove. The `gls` fallback additionally has to ask
// whether `gls` resolves, because `gls` is a Homebrew-only name and a Debian host
// without Homebrew has only the system `ls`.
func TestFishKeepsAColourLsWithoutEza(t *testing.T) {
	byCommand := map[string]bool{}
	for _, alias := range readConfigAliases(t, fishShippedConfig) {
		if alias.name == "ls" && !strings.HasPrefix(alias.command, "eza ") {
			byCommand[alias.command] = true
		}
	}

	for _, command := range []string{"ls --color=auto", "gls --color=auto"} {
		if !byCommand[command] {
			t.Errorf("%s no longer aliases `ls` to `%s`: a host where eza does not resolve loses the colour the previous configuration gave it", fishShippedConfig, command)
		}
	}

	if byCommand["gls --color=auto"] {
		body, err := os.ReadFile(fishShippedConfig)
		if err != nil {
			t.Fatalf("reading %s: %v", fishShippedConfig, err)
		}
		if !strings.Contains(string(body), "command -q gls") {
			t.Errorf("%s aliases `ls` to `gls --color=auto` without asking whether gls resolves, so a host without Homebrew's coreutils gets an alias that fails on every call", fishShippedConfig)
		}
	}
}
