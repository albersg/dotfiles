package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// --- the markers bound exactly what the set provides -------------------------

// TestTheZshPluginMarkersBoundTheTextTheInstallerTreatsAsTheSet is the guard the
// zsh patch cannot do without, the same one the Tmux set carries. A run that leaves
// the shell's set out rewrites the installed .zshrc between the markers, so the
// markers decide what the user loses: a block that is too wide takes a line the set
// does not own -- a PATH entry, an alias, an eval of another tool -- and a loader
// left outside the block survives the strip and asks the shell for a plugin that is
// not installed.
func TestTheZshPluginMarkersBoundTheTextTheInstallerTreatsAsTheSet(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "dotfiles-zsh", ".zshrc"))
	if err != nil {
		t.Fatalf("the zsh configuration this installer ships could not be read: %v", err)
	}
	content := string(data)

	for _, marker := range []string{zshPluginBlockStart, zshPluginBlockEnd} {
		if got := strings.Count(content, marker); got != 1 {
			t.Errorf("the zsh configuration carries %q %d times, want exactly once: the installer strips what is between the markers, and a duplicate or a missing one would leave the machine with text nobody owns", marker, got)
		}
	}

	start := strings.Index(content, zshPluginBlockStart)
	end := strings.Index(content, zshPluginBlockEnd)
	if start < 0 || end < start {
		t.Fatalf("the markers are not in the order the installer strips them: start %d, end %d", start, end)
	}
	block := content[start+len(zshPluginBlockStart) : end]
	outside := content[:start] + content[end+len(zshPluginBlockEnd):]

	// The declarations and the loaders the set owns are inside the block.
	for _, needed := range []string{
		"plugins=(",
		`source "$ZSH/oh-my-zsh.sh"`,
		"source_first_existing",
		"zsh-autosuggestions",
		"zsh-syntax-highlighting",
		"powerlevel10k",
	} {
		if !strings.Contains(block, needed) {
			t.Errorf("the marked block does not carry %q, so the markers bound the wrong text", needed)
		}
	}

	// Nothing the set owns survives outside it: a loader left behind would name a
	// plugin a run that left the set out never installed.
	for _, leaked := range []string{
		"plugins=(",
		`source "$ZSH/oh-my-zsh.sh"`,
		"source_first_existing",
		"$PREFIX/share/zsh-autosuggestions",
		"$PREFIX/share/zsh-syntax-highlighting",
		"$PREFIX/share/powerlevel10k",
		"$BREW_SHARE/zsh-autosuggestions",
		"$BREW_SHARE/zsh-syntax-highlighting",
		"$BREW_SHARE/powerlevel10k",
	} {
		if strings.Contains(outside, leaked) {
			t.Errorf("the marked block leaves %q outside the markers, so a run that left the set out would leave the loader behind", leaked)
		}
	}

	// A block wide enough to swallow a setting that is not the set's is the other
	// failure mode: turning the set off would silently remove it.
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"export ", "alias ", "eval ", "zstyle ", "autoload ", "zmodload ", "bindkey "} {
			if strings.HasPrefix(trimmed, prefix) {
				t.Errorf("the marked block swallows a setting that is not the plugin set: %q", trimmed)
			}
		}
	}
}

// --- with the set off the configuration names nothing of it ------------------

// TestTurningOffTheShellsSetLeavesNoPluginInTheConfiguration pins both halves of the
// switch where the user feels it: with the set on the configuration is the one this
// repository ships, and with the set off it sources no Oh My Zsh, loads no plugin of
// the set, carries the line saying why instead of the block, and still parses under
// zsh.
func TestTurningOffTheShellsSetLeavesNoPluginInTheConfiguration(t *testing.T) {
	on := runZshShellStepWithSet(t, nil)
	off := runZshShellStepWithSet(t, []string{pluginSetIDShell})

	if !strings.Contains(on.config, `source "$ZSH/oh-my-zsh.sh"`) || !strings.Contains(on.config, "plugins=(") {
		t.Errorf("with the set on the installed configuration does not source Oh My Zsh and declare its plugins:\n%s", on.config)
	}

	for _, gone := range []string{
		`source "$ZSH/oh-my-zsh.sh"`,
		"plugins=(",
		"source_first_existing",
	} {
		if strings.Contains(off.config, gone) {
			t.Errorf("with the set off the installed configuration still carries %q, so it asks for a plugin the run did not install:\n%s", gone, off.config)
		}
	}
	if !strings.Contains(off.config, zshPluginSetLeftOut) {
		t.Errorf("with the set off the installed configuration does not say why the plugins are gone:\n%s", off.config)
	}
	if !strings.Contains(off.log, "plugin set is turned off") {
		t.Errorf("a set the user turned off was not reported: %q", off.log)
	}

	// Every plugin loader left outside the markers carries its own presence test, so
	// the configuration that remains still asks for nothing that is not there. The
	// guard holds this for the file the run actually writes, not only for the asset.
	assertZshPluginLoadersAreGuarded(t, zshConfigOutsidePlugins(off.config))

	// zsh itself reads the file the run wrote, rather than the guard reading it as
	// text: a configuration that does not parse would break every new shell.
	runZshSyntaxCheck(t, on.configPath)
	runZshSyntaxCheck(t, off.configPath)
}

// --- the strip refuses rather than guesses -----------------------------------

// TestTheZshStripRefusesWhenAMarkerIsMissingOrUnterminated is the contract the
// stripper shares with the Tmux one: a file whose markers are gone or unbalanced is
// left byte-for-byte alone and named in an error, because stripping between a marker
// that is not there would either remove nothing or remove the rest of the file.
func TestTheZshStripRefusesWhenAMarkerIsMissingOrUnterminated(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), ".zshrc")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	refuse := func(t *testing.T, path, want string) {
		t.Helper()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		err = leaveZshPluginsOut(path)
		if err == nil {
			t.Fatalf("a configuration without a usable marker pair was rewritten instead of refused")
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal says %q, want it to name %q", err, want)
		}
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("a refused file was modified:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	}

	t.Run("no markers at all", func(t *testing.T) {
		refuse(t, write(t, "export EDITOR=nvim\n"), zshPluginBlockStart)
	})

	t.Run("start without end", func(t *testing.T) {
		body := "export EDITOR=nvim\n" + zshPluginBlockStart + "\nplugins=(\n)\n"
		refuse(t, write(t, body), zshPluginBlockEnd)
	})

	t.Run("a marker pair is stripped", func(t *testing.T) {
		path := write(t, "export EDITOR=nvim\n"+zshPluginBlockStart+"\nplugins=(\n  command-not-found\n)\n"+zshPluginBlockEnd+"\nexport LANG=C\n")
		if err := leaveZshPluginsOut(path); err != nil {
			t.Fatalf("a configuration carrying both markers must be stripped: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got := string(data)
		if strings.Contains(got, "command-not-found") || strings.Contains(got, zshPluginBlockStart) || strings.Contains(got, zshPluginBlockEnd) {
			t.Errorf("the stripped configuration still carries the block:\n%s", got)
		}
		if !strings.Contains(got, zshPluginSetLeftOut) {
			t.Errorf("the stripped configuration does not say why the plugins are gone:\n%s", got)
		}
		if !strings.Contains(got, "export EDITOR=nvim") || !strings.Contains(got, "export LANG=C") {
			t.Errorf("the strip removed a line it does not own:\n%s", got)
		}
	})
}

// --- the step installs nothing of the set when it is off ---------------------

// TestTheZshStepInstallsNothingOfTheSetWhenItIsOff pins the executor half of the
// switch: with the set off the step runs no Oh My Zsh installer, requests no package
// that exists only to provide a plugin of the set, and leaves no Oh My Zsh checkout
// behind; with the set on both happen.
func TestTheZshStepInstallsNothingOfTheSetWhenItIsOff(t *testing.T) {
	ranPackage := func(calls []packageCommandCall, name string) bool {
		for _, call := range calls {
			if call.runner == "oh-my-zsh" {
				continue
			}
			if strings.Contains(" "+call.command+" ", " "+name+" ") || strings.Contains(call.command, name) {
				return true
			}
		}
		return false
	}
	ranOhMyZsh := func(calls []packageCommandCall) bool {
		for _, call := range calls {
			if call.runner == "oh-my-zsh" {
				return true
			}
		}
		return false
	}

	on := runZshShellStepWithSet(t, nil)
	if !ranOhMyZsh(on.calls) {
		t.Error("with the set on the step ran no Oh My Zsh installer")
	}
	for _, name := range []string{"zsh-autosuggestions", "zsh-syntax-highlighting", "powerlevel10k"} {
		if !ranPackage(on.calls, name) {
			t.Errorf("with the set on the step requested no %s", name)
		}
	}

	off := runZshShellStepWithSet(t, []string{pluginSetIDShell})
	if ranOhMyZsh(off.calls) {
		t.Errorf("with the set off the step still ran the Oh My Zsh installer: %v", off.calls)
	}
	for _, name := range zshPluginPackageNames {
		if ranPackage(off.calls, name) {
			t.Errorf("with the set off the step still requested %s, a package that exists only to provide a plugin of the set: %v", name, off.calls)
		}
	}
	if _, err := os.Stat(filepath.Join(off.home, ".oh-my-zsh")); err == nil {
		t.Error("with the set off the step left an Oh My Zsh checkout behind")
	}
}

// --- every plugin named outside the markers is guarded -----------------------

// zshPluginOutsideLoaders is the loading lines that stay outside the markers, each
// with the presence test the configuration puts in front of it. carapace, fzf-tab and
// zsh-completions are plugins of the set whose loaders the block does not own -- they
// sit among completion settings that are not the set's -- so their packages are not
// requested when the set is off (zshPluginPackageNames) and the loaders have to turn
// themselves off; the Powerlevel10k prompt is the same, for the config file the run
// writes rather than a package.
var zshPluginOutsideLoaders = []struct{ plugin, loader, guard string }{
	{
		plugin: "powerlevel10k",
		loader: `source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"`,
		guard:  `if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then`,
	},
	{
		plugin: "powerlevel10k",
		loader: "source ~/.p10k.zsh",
		guard:  "[[ ! -f ~/.p10k.zsh ]] ||",
	},
	{
		plugin: "carapace",
		loader: "source <(carapace _carapace)",
		guard:  "if command -v carapace >/dev/null 2>&1; then",
	},
	{
		plugin: "fzf-tab",
		loader: `source "${BREW_SHARE:h}/opt/fzf-tab/share/fzf-tab/fzf-tab.zsh"`,
		guard:  `if [[ -n "$BREW_SHARE" && -f "${BREW_SHARE:h}/opt/fzf-tab/share/fzf-tab/fzf-tab.zsh" ]]; then`,
	},
	{
		plugin: "zsh-completions",
		loader: `fpath=("$(dirname "$BREW_BIN")/share/zsh-completions" $fpath)`,
		guard:  `if [[ -n "$BREW_BIN" && -d "$(dirname "$BREW_BIN")/share/zsh-completions" ]]; then`,
	},
}

// zshPluginOutsideTokens are the names that make a line a plugin loader. A line
// outside the markers that loads one of them has to carry its own presence test:
// without it, turning the set off would leave the shell asking for a plugin that is
// not installed.
var zshPluginOutsideTokens = []string{
	"p10k",
	"powerlevel10k",
	"carapace",
	"fzf-tab",
	"zsh-completions",
	"zsh-autosuggestions",
	"zsh-syntax-highlighting",
	"zsh-autocomplete",
}

// TestEveryPluginTheZshConfigurationNamesOutsideTheMarkersCarriesAPresenceTest is
// the boundary the markers do not cover, held for the asset this repository ships
// and for nothing else: every loader outside the block is one this guard knows and
// tests before it runs, and a new loader that names a plugin without a test fails
// here instead of reaching a machine that left the set out.
func TestEveryPluginTheZshConfigurationNamesOutsideTheMarkersCarriesAPresenceTest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "dotfiles-zsh", ".zshrc"))
	if err != nil {
		t.Fatalf("the zsh configuration this installer ships could not be read: %v", err)
	}
	assertZshPluginLoadersAreGuarded(t, zshConfigOutsidePlugins(string(data)))
}

// assertZshPluginLoadersAreGuarded holds the rule for a stretch of configuration:
// each known loader is present with its presence test in front of it, and no other
// line in the stretch loads a plugin by name.
func assertZshPluginLoadersAreGuarded(t *testing.T, outside string) {
	t.Helper()

	for _, want := range zshPluginOutsideLoaders {
		guardAt := strings.Index(outside, want.guard)
		if guardAt < 0 {
			t.Errorf("the configuration no longer tests for %s before loading it, so turning the set off would ask the shell for a plugin that is not there: %q", want.plugin, want.guard)
			continue
		}
		loadAt := strings.Index(outside, want.loader)
		if loadAt < 0 {
			t.Errorf("the configuration no longer loads %s; if it was removed, this guard and its entry are stale rather than this being a defect", want.plugin)
			continue
		}
		if loadAt < guardAt {
			t.Errorf("the configuration loads %s before it tests for it: loader at %d, guard at %d", want.plugin, loadAt, guardAt)
		}
	}

	for _, line := range strings.Split(outside, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "source ") {
			continue
		}
		names := ""
		for _, token := range zshPluginOutsideTokens {
			if strings.Contains(trimmed, token) {
				names = token
				break
			}
		}
		if names == "" {
			continue
		}
		known := false
		for _, want := range zshPluginOutsideLoaders {
			if strings.Contains(trimmed, want.loader) {
				known = true
				break
			}
		}
		if !known {
			t.Errorf("a line outside the marked block loads the plugin %s without a presence test this guard knows: %q", names, trimmed)
		}
	}
}

// zshConfigOutsidePlugins is the stretch of a configuration the markers do not
// cover. A file whose markers are already stripped has no block and is returned
// whole, so the same rule can be applied to the file a run with the set off writes.
func zshConfigOutsidePlugins(content string) string {
	start := strings.Index(content, zshPluginBlockStart)
	end := strings.Index(content, zshPluginBlockEnd)
	if start >= 0 && end > start {
		return content[:start] + content[end+len(zshPluginBlockEnd):]
	}
	return content
}

// runZshSyntaxCheck hands the file the run wrote to zsh itself. Where zsh is not
// installed there is nothing to check it with, and the test says so rather than
// passing on a claim it did not measure.
func runZshSyntaxCheck(t *testing.T, path string) {
	t.Helper()

	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not available to check the configuration the run wrote")
	}
	var stderr bytes.Buffer
	cmd := exec.Command(zsh, "-n", path)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the configuration this run wrote does not parse under zsh: %v\n%s", err, stderr.String())
	}
}

// zshShellRun is one shell step's observable result: the file it wrote, the
// commands it ran and what it reported.
type zshShellRun struct {
	home       string
	configPath string
	config     string
	calls      []packageCommandCall
	log        string
}

// runZshShellStepWithSet runs the zsh branch of the shell step for these choices and
// returns what it did. The package managers, the Oh My Zsh installer and fnm are
// stubbed, so the step touches no network and the machine's own tools stay out of the
// measurement; the configuration it writes is the real one.
func runZshShellStepWithSet(t *testing.T, disabled []string) zshShellRun {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	readLog := verboseTerminalStep(t)
	calls := withZshMocks(t, home)
	withFnmMocks(t, func() string { return "" })

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none", DisabledPluginSets: disabled}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("the shell step must succeed with the plugin set %v: %v", disabled, err)
	}

	configPath := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("the shell step wrote no .zshrc: %v", err)
	}
	return zshShellRun{home: home, configPath: configPath, config: string(data), calls: *calls, log: readLog()}
}
