package tui

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// updateThemeArtifacts regenerates the shipped blocks from themes/*.toml. It is
// the only writer of those blocks besides the installer itself; running the
// guard without it is what fails on drift.
var updateThemeArtifacts = flag.Bool("update-theme-artifacts", false,
	"regenerate the shipped theme blocks from themes/*.toml")

// repoRoot resolves the repository checkout from the package directory, so the
// tests exercise the very files the installer ships.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !system.DirExists(filepath.Join(root, "installer")) {
		t.Fatalf("repository root not found at %s", root)
	}
	return root
}

// TestRepoAssetsExist guards the whole class of defect where a path moves in the
// repository and the installer keeps copying from the old location. Herdr and
// the Tmux plugin seed both broke exactly this way during the rebrand.
func TestRepoAssetsExist(t *testing.T) {
	root := repoRoot(t)

	optional := map[string]bool{}
	for _, asset := range optionalRepoAssets {
		optional[asset] = true
	}

	for _, asset := range repoAssets {
		if _, err := os.Stat(filepath.Join(root, asset)); err != nil {
			if optional[asset] {
				continue
			}
			t.Errorf("installer copies %s, which does not exist in the repository: %v", asset, err)
		}
	}

	if len(repoAssets) == 0 {
		t.Fatal("no repository assets declared")
	}
}

// installerHomeJoin is one filepath.Join rooted at the user's home, with the path
// it resolves to relative to that home. path is empty when the join is built from
// something other than string literals, which the guard reports rather than
// skipping: an unresolvable destination is exactly where an unprotected write can
// hide.
type installerHomeJoin struct {
	file string
	line int
	path string
	expr string
}

// installerHomeJoinRE matches the joins that land in the user's home. The home
// itself is spelled three ways in this package: the homeDir local the step
// functions take, the home local the resolution helpers take, and a direct
// os.Getenv("HOME").
var installerHomeJoinRE = regexp.MustCompile(`filepath\.Join\((?:(?:homeDir|home)|os\.Getenv\("HOME"\))\s*,\s*([^)]*)\)`)

// installerOwnedHomePaths are the home paths the installer creates, reads or runs
// that are not user configuration, so the backup step could not restore anything
// the user wrote there. Every other home path the sources join must be named by
// ConfigPaths(), which is the map the backup step, the overwrite count and the
// last-install record all read. The reason is for the reader of a failure.
var installerOwnedHomePaths = map[string]string{
	".config":                               "the XDG config root the shell step creates; each namespace inside it is classified by its own entry",
	".cache/starship":                       "starship's own cache directory",
	".cache/carapace":                       "carapace's own cache directory",
	".local/share":                          "the XDG data root; every path under it here is tool data, not configuration",
	".local/share/atuin":                    "atuin's own data directory, created empty by the shell step",
	".local/share/fonts":                    "the user's font directory: font files, and the fonts are installed from an archive rather than copied by the installer",
	".local/share/fnm":                      "fnm's own installation directory",
	".local/share/fnm/aliases":              "fnm's own alias store, written by fnm itself",
	".local/bin":                            "where the installer puts the command-line tools it manages (OfficeCLI, agent skills)",
	".local/state/dotfiles":                 "the installer's own state directory, which is where the last-install record lives",
	".local/share/dotfiles":                 "the installer's own data root: the WSL template and theme definitions it installs for later runs",
	".cargo/bin/cargo":                      "the cargo binary the shell step looks for, never written",
	".termux":                               "Termux's own application directory (its font and properties are Termux's)",
	".tmux":                                 "the parent of the plugin checkout, created empty",
	".tmux/plugins":                         "the plugin checkout TPM owns; the multiplexer's configuration is ~/.tmux.conf",
	".tmux/plugins/tpm":                     "TPM's own clone",
	".tmux/plugins/tpm/bin/install_plugins": "TPM's own installer, which the multiplexer step runs",
	".zshrc.d":                              "the drop-in directory the .zshrc preservation writes into; the existing ~/.zshrc is what the map backs up",
	".config/obsidian":                      "a directory created for the user's notes, with no file written into it",
	".pi/agent/skills":                      "the agent-skills destination: the step reports an existing skill and leaves it untouched",
	"dotfiles":                              "a clone location the installer looks for",
	".dotfiles":                             "a clone location the installer looks for",
}

// TestEveryHomePathTheInstallerWritesIsClassified is the guard for the class of
// defect in issue #238. The backup step copies only what ConfigPaths() names, and
// the overwrite count and the last-install record are built from the same map, so
// a file the installer writes into $HOME that the map omits is replaced with no
// copy and with nothing anywhere saying so. ~/.zshenv was exactly that, and the
// comment eight lines above the write shows the pair was meant to be protected.
//
// It reads the installer's own sources rather than a hand-written list, so a new
// write into $HOME fails here until its destination is either named in the map or
// classified below as something the backup step could not restore anyway.
func TestEveryHomePathTheInstallerWritesIsClassified(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPaths := system.ConfigPaths()
	protected := make([]string, 0, len(configPaths))
	byKey := make(map[string]string, len(configPaths))
	for key, path := range configPaths {
		rel, err := filepath.Rel(home, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("ConfigPaths() names %s outside $HOME: %s", key, path)
		}
		rel = filepath.ToSlash(rel)
		protected = append(protected, rel)
		byKey[key] = rel
	}

	joins := scanInstallerHomeJoins(t)
	if len(joins) == 0 {
		t.Fatal("the scan found no $HOME path in the installer's sources; this guard is not reading the files it claims to")
	}

	for _, problem := range homePathViolations(joins, protected, byKey) {
		t.Error(problem)
	}
	for _, problem := range staleOwnedHomePaths(joins) {
		t.Error(problem)
	}
}

// staleOwnedHomePaths reports every installer-owned path no join in the sources
// targets. The table is a classification, so an entry that stopped being written
// has to stop being excused: a path left in it would silently allow a future
// write there to go unclassified, and the reason it carries would stop being true
// without anyone noticing.
func staleOwnedHomePaths(joins []installerHomeJoin) []string {
	var problems []string

	for path, reason := range installerOwnedHomePaths {
		joined := false
		for _, join := range joins {
			if join.path == path {
				joined = true
				break
			}
		}
		if !joined {
			problems = append(problems, fmt.Sprintf("installerOwnedHomePaths classifies ~/%s (%s), but no join in the installer targets it; the entry is stale", path, reason))
		}
	}

	return problems
}

// homePathViolations returns the problems the guard reports for a set of joins,
// as the messages a reader would see. It is separated from the scan so the test
// below can feed it the exact join a defect adds without editing the sources.
func homePathViolations(joins []installerHomeJoin, protected []string, byKey map[string]string) []string {
	var problems []string

	for _, join := range joins {
		if join.path == "" {
			problems = append(problems, fmt.Sprintf("%s:%d joins $HOME with %s, which this guard cannot resolve: name the destination with string literals so it can be classified", join.file, join.line, join.expr))
			continue
		}
		if underAnyHomePath(join.path, protected) {
			continue
		}
		if _, ok := installerOwnedHomePaths[join.path]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s:%d writes ~/%s, which ConfigPaths() does not name: the backup step would not copy it and the last-install record would not mention it", join.file, join.line, join.path))
	}

	// The other direction: a key the map names that no write targets is stale, and
	// a stale key is worse than useless here, because it is the path nobody writes
	// that gets backed up while the file that is replaced goes unprotected. That is
	// how `wezterm` pointed at ~/.wezterm.lua while the terminal step wrote
	// ~/.config/wezterm/wezterm.lua.
	for key, rel := range byKey {
		written := false
		for _, join := range joins {
			if join.path != "" && underAnyHomePath(join.path, []string{rel}) {
				written = true
				break
			}
		}
		if !written {
			problems = append(problems, fmt.Sprintf("ConfigPaths() names %s at ~/%s, but no write in the installer targets it; the key is stale", key, rel))
		}
	}

	return problems
}

// TestTheHomePathGuardFailsOnAnUnprotectedWrite pins the guard's teeth. The guard
// above only proves that today's tree is clean; this proves that the tree with the
// reported defect fails, that the failure names the file and the path, and that
// the classification is exact where it has to be: ~/.zshrc.d is not covered by a
// ~/.zshrc entry, and an allowlisted directory does not cover anything under it,
// because the allowlist is what says a path is not user configuration.
func TestTheHomePathGuardFailsOnAnUnprotectedWrite(t *testing.T) {
	protected := []string{".zshrc", ".config/fish"}
	byKey := map[string]string{"zsh": ".zshrc", "fish": ".config/fish", "zsh_p10k": ".p10k.zsh"}

	for _, tc := range []struct {
		name     string
		join     installerHomeJoin
		want     string
		wantNone bool
	}{
		{
			name: "the user's own .zshenv is reported",
			join: installerHomeJoin{file: "installer.go", line: 6133, path: ".zshenv"},
			want: "installer.go:6133 writes ~/.zshenv, which ConfigPaths() does not name",
		},
		{
			name:     "a path inside a protected directory is not reported",
			join:     installerHomeJoin{file: "installer.go", line: 6031, path: ".config/fish/dotfiles.d"},
			wantNone: true,
		},
		{
			name: "a sibling that shares the protected file's name is reported",
			join: installerHomeJoin{file: "installer.go", line: 6140, path: ".zshrc.bak"},
			want: "writes ~/.zshrc.bak",
		},
		{
			name:     "an allowlisted path is not reported",
			join:     installerHomeJoin{file: "installer.go", line: 5152, path: ".local/share/fonts"},
			wantNone: true,
		},
		{
			name: "a path under an allowlisted directory is still reported",
			join: installerHomeJoin{file: "installer.go", line: 9999, path: ".local/share/fonts/new-config.toml"},
			want: "writes ~/.local/share/fonts/new-config.toml",
		},
		{
			name: "a destination the guard cannot resolve is reported",
			join: installerHomeJoin{file: "installer.go", line: 4388, expr: "rcFile"},
			want: "installer.go:4388 joins $HOME with rcFile, which this guard cannot resolve",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := homePathViolations([]installerHomeJoin{tc.join}, protected, nil)
			if tc.wantNone {
				if len(problems) != 0 {
					t.Errorf("the guard reported %v for a classified path", problems)
				}
				return
			}
			if len(problems) != 1 {
				t.Fatalf("the guard reported %v, want one problem", problems)
			}
			if !strings.Contains(problems[0], tc.want) {
				t.Errorf("the guard said %q, want it to name %q", problems[0], tc.want)
			}
		})
	}

	t.Run("a config key nothing writes is reported as stale", func(t *testing.T) {
		problems := homePathViolations([]installerHomeJoin{{file: "installer.go", line: 1702, path: ".zshrc"}}, protected, byKey)
		stale := false
		for _, problem := range problems {
			if strings.Contains(problem, "names zsh_p10k at ~/.p10k.zsh") {
				stale = true
			}
		}
		if !stale {
			t.Errorf("a ConfigPaths() key no write targets was not reported: %v", problems)
		}
	})
}

// underAnyHomePath reports whether path is one of roots or a descendant of it.
// The separator is part of the test on purpose: ~/.zshenv must not count as a
// descendant of ~/.zshrc, and ~/.config/fish must not count as one of ~/.config.
func underAnyHomePath(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// scanInstallerHomeJoins reads every non-test source file in this package and
// returns the filepath.Join sites rooted at $HOME, resolved to a slash-separated
// path relative to it. A join whose arguments are not all string literals or
// string constants comes back with an empty path and the arguments it used.
func scanInstallerHomeJoins(t *testing.T) []installerHomeJoin {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		sources[name] = string(data)
	}

	// String constants are how a path component is spelled without repeating the
	// literal; stateAppDir is the one the sources use.
	constDecl := regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"((?:[^"\\]|\\.)*)"\s*$`)
	consts := map[string]string{}
	for _, src := range sources {
		for _, m := range constDecl.FindAllStringSubmatch(src, -1) {
			if value, err := strconv.Unquote(`"` + m[2] + `"`); err == nil {
				consts[m[1]] = value
			}
		}
	}

	var joins []installerHomeJoin
	files := make([]string, 0, len(sources))
	for name := range sources {
		files = append(files, name)
	}
	sort.Strings(files)
	for _, name := range files {
		for i, line := range strings.Split(sources[name], "\n") {
			for _, m := range installerHomeJoinRE.FindAllStringSubmatch(line, -1) {
				join := installerHomeJoin{file: name, line: i + 1, expr: strings.TrimSpace(m[1])}
				join.path = resolveHomeJoin(m[1], consts)
				joins = append(joins, join)
			}
		}
	}

	return joins
}

// resolveHomeJoin turns the arguments of a $HOME-rooted filepath.Join into a
// slash-separated path relative to that home, or an empty string when any
// argument is neither a string literal nor a known string constant.
func resolveHomeJoin(args string, consts map[string]string) string {
	var parts []string
	for _, part := range strings.Split(args, ",") {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			value, err := strconv.Unquote(part)
			if err != nil {
				return ""
			}
			parts = append(parts, value)
			continue
		}
		value, ok := consts[part]
		if !ok {
			return ""
		}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

// TestTheShellStepBacksUpEveryHomeFileItReplaces pins the user's symptom from
// issue #238 against the steps themselves: ~/.zshenv is read by every zsh
// invocation, so a user's own file there is the one destructive report that
// matters most, and the backup step only copies what the detection names. The
// detection is derived from ConfigPaths(), so a key missing from that map means
// the file is replaced with no copy at all.
func TestTheShellStepBacksUpEveryHomeFileItReplaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		rel  string
	}{
		{name: ".zshenv", key: "zshenv", rel: ".zshenv"},
		{name: ".gitconfig-personal", key: "gitconfig-personal", rel: ".gitconfig-personal"},
		{name: ".gitconfig", key: "gitconfig", rel: ".gitconfig"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			withZshMocks(t, home)

			// The user's own file, with content the repository does not ship.
			userFile := filepath.Join(home, tc.rel)
			userContent := "# mine, written by me\nexport PATH=\"$HOME/.cargo/bin:$PATH\"\n"
			if err := os.WriteFile(userFile, []byte(userContent), 0o644); err != nil {
				t.Fatal(err)
			}

			m := NewModel()
			m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
			m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
			m.RepoDir = repoRoot(t)

			// The detection drives the backup step, so it has to name the file.
			m.ExistingConfigs = system.DetectExistingConfigs()
			if !slices.Contains(m.ExistingConfigs, tc.key+": "+userFile) {
				t.Errorf("the detection does not name the user's ~/%s, so the backup step cannot copy it: %v", tc.rel, m.ExistingConfigs)
			}

			if err := stepBackupConfigs(&m); err != nil {
				t.Fatalf("the backup step failed: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(m.BackupDir, tc.key))
			if err != nil {
				t.Fatalf("the user's ~/%s was replaced with no copy in the backup: %v", tc.rel, err)
			}
			if string(got) != userContent {
				t.Errorf("the backup holds %q, want the user's own bytes %q", got, userContent)
			}

			// The shell step is what replaces it, and it still installs the
			// repository's own file over the user's.
			if err := stepInstallShell(&m); err != nil {
				t.Fatalf("the shell step failed: %v", err)
			}
			installed, err := os.ReadFile(userFile)
			if err != nil {
				t.Fatalf("~/%s was not installed: %v", tc.rel, err)
			}
			if string(installed) == userContent {
				t.Errorf("the shell step left the user's ~/%s in place, so this test no longer covers a replacement", tc.rel)
			}
		})
	}
}

// TestEveryProgramTheRepositoryShipsIsInstalledExecutable is the guard for the
// class of defect in issue #241: a repository that ships programs relies on the
// mode it gives them, and the installer replaced that decision with a constant.
//
// The requirement is derived from the repository's own bytes rather than from a
// list: a file whose first two bytes are "#!" is meant to be run, so it must be
// executable in the repository and executable once the installer has copied it
// into the user's home. An asset added later that needs the bit is covered
// without anyone remembering to add it here, which is the same reason the theme
// artifacts are generated from their definitions instead of hand-written.
func TestEveryProgramTheRepositoryShipsIsInstalledExecutable(t *testing.T) {
	root := repoRoot(t)
	dest := t.TempDir()

	programs := 0
	for _, asset := range repoAssets {
		src := filepath.Join(root, asset)
		info, err := os.Stat(src)
		if err != nil {
			t.Fatalf("the installer copies %s, which does not exist: %v", asset, err)
		}

		var relative []string
		if info.IsDir() {
			if err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				relative = append(relative, rel)
				return nil
			}); err != nil {
				t.Fatalf("walking %s: %v", asset, err)
			}
		} else {
			relative = []string{asset}
		}

		for _, rel := range relative {
			path := filepath.Join(root, rel)
			shebang := make([]byte, 2)
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("reading %s: %v", rel, err)
			}
			n, _ := file.Read(shebang)
			_ = file.Close()
			// A file shorter than two bytes cannot be a program, and an empty asset is
			// nothing this guard is about.
			if n < len(shebang) || string(shebang) != "#!" {
				continue
			}
			programs++

			// First the repository itself: a program that is not executable there is
			// already broken for anyone who runs it from the checkout, and the installer
			// cannot invent the bit the repository did not set.
			if info, err := os.Stat(path); err != nil {
				t.Fatalf("stat %s: %v", rel, err)
			} else if info.Mode().Perm()&0o111 == 0 {
				t.Errorf("%s starts with #! but is %v in the repository: it is a program the checkout cannot run", rel, info.Mode().Perm())
			}

			// Then the install: copying it the way the installer does must keep the bit,
			// because these files are invoked as programs, not read as configuration.
			installed := filepath.Join(dest, rel)
			if err := system.CopyFile(path, installed); err != nil {
				t.Fatalf("installing %s: %v", rel, err)
			}
			if info, err := os.Stat(installed); err != nil {
				t.Fatalf("stat %s: %v", installed, err)
			} else if info.Mode().Perm()&0o111 == 0 {
				t.Errorf("%s is installed as %v: the program the repository ships cannot be run from the user's home", rel, info.Mode().Perm())
			}
		}
	}

	if programs == 0 {
		t.Fatal("no file among the installer's assets starts with #!, so this guard proves nothing")
	}
}

// TestInstalledAssetsKeepTheRepositorysMode is the assertion issue #241 found
// missing: nothing in the installer's tests looked at the mode of an installed
// asset, so a copy that dropped the executable bit could not fail here. Both
// files are programs the repository ships executable and both are run as
// programs: bash-env-json by bash-env.nu through the nushell environment, and
// safe-update.sh through installConfigDir, which is CopyDirPruned.
func TestInstalledAssetsKeepTheRepositorysMode(t *testing.T) {
	root := repoRoot(t)

	t.Run("the shell step installs the bash helper executable", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withPackageCommandMocks(t, nil)

		m := NewModel()
		m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: true}
		m.Choices = UserChoices{OS: "linux", Shell: "nushell", WindowMgr: "none"}
		m.RepoDir = root

		if err := stepInstallShell(&m); err != nil {
			t.Fatalf("the nushell step failed: %v", err)
		}

		configDir := filepath.Join(home, ".config")
		assertInstalledModeMatchesSource(t, filepath.Join(configDir, "bash-env-json"), filepath.Join(root, repoAssetBashEnvJSON))
		assertInstalledModeMatchesSource(t, filepath.Join(configDir, "bash-env.nu"), filepath.Join(root, repoAssetBashEnvNu))
	})

	t.Run("the nvim step installs the update script executable", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withPackageCommandMocks(t, nil)

		m := NewModel()
		// Termux keeps the optional Claude and OpenCode installers, which shell out to
		// the network, out of this test.
		m.SystemInfo = &system.SystemInfo{IsTermux: true}
		m.Choices = UserChoices{OS: "termux", InstallNvim: true}
		m.RepoDir = root

		if err := stepInstallNvim(&m); err != nil {
			t.Fatalf("the nvim step failed: %v", err)
		}

		assertInstalledModeMatchesSource(t,
			filepath.Join(home, ".config", "nvim", "scripts", "safe-update.sh"),
			filepath.Join(root, repoAssetNvim, "scripts", "safe-update.sh"))
	})
}

// assertInstalledModeMatchesSource compares an installed file's mode with the
// mode the repository ships it with, so a copy that happens to be executable for
// the wrong reason -- a 0755 constant, say -- still fails.
func assertInstalledModeMatchesSource(t *testing.T, installed, source string) {
	t.Helper()

	want, err := os.Stat(source)
	if err != nil {
		t.Fatalf("the repository copy %s is missing: %v", source, err)
	}
	got, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("%s was not installed: %v", installed, err)
	}
	if got.Mode().Perm() != want.Mode().Perm() {
		t.Errorf("%s is installed as %v, want the repository's %v", installed, got.Mode().Perm(), want.Mode().Perm())
	}
}

func TestStepInstallWMHerdrCopiesConfigFromRepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	m := NewModel()
	// OS is macOS so the Herdr binary step takes the (mocked) Homebrew route
	// instead of downloading a release.
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("herdr step failed: %v", err)
	}

	installed := filepath.Join(home, ".config", "herdr", "config.toml")
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("herdr configuration was not installed: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetHerdrConfig))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("installed herdr configuration does not match the repository copy")
	}
}

func TestStepInstallWMTmuxToleratesMissingPluginSeed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "tmux"}
	m.RepoDir = repoRoot(t)

	// The repository ships no plugin seed: tmux.conf declares them through TPM.
	if system.DirExists(filepath.Join(m.RepoDir, repoAssetTmuxPlugins)) {
		t.Skip("a plugin seed is present again; this guard no longer applies")
	}

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("tmux step must not fail without a plugin seed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".tmux.conf")); err != nil {
		t.Errorf("tmux.conf was not installed: %v", err)
	}
}

func TestStepInstallShellZshInstallsZshenvAndZshrc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withZshMocks(t, home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("zsh step failed: %v", err)
	}

	for _, tc := range []struct {
		asset string
		dst   string
	}{
		{repoAssetZshEnv, ".zshenv"},
		{repoAssetZshrc, ".zshrc"},
		{repoAssetP10k, ".p10k.zsh"},
	} {
		got, err := os.ReadFile(filepath.Join(home, tc.dst))
		if err != nil {
			t.Fatalf("%s was not installed: %v", tc.dst, err)
		}
		want, err := os.ReadFile(filepath.Join(m.RepoDir, tc.asset))
		if err != nil {
			t.Fatal(err)
		}
		// .zshrc is patched for the chosen window manager, so only check that the
		// content is derived from the repository copy.
		if !strings.HasPrefix(string(got), strings.SplitN(string(want), "\n", 2)[0]) {
			t.Errorf("%s does not derive from %s", tc.dst, tc.asset)
		}
	}

	// Oh My Zsh is delegated to its own installer, never vendored: a fresh HOME
	// has nothing there, so the official installer must have been invoked.
	if !callsContain(calls, "oh-my-zsh") {
		t.Error("the official Oh My Zsh installer was not invoked on a fresh HOME")
	}

	// Pin the command shape. Termux does not run commands through a shell, it
	// splits and execs them, so two things break there: a leading `VAR=value`
	// assignment becomes the program name, and `sh -c "$(curl ...)"` reaches sh
	// unexpanded and tries to execute the downloaded script as a command.
	assertOhMyZshCommandsAreShellIndependent(t, calls)
}

// assertOhMyZshCommandsAreShellIndependent checks that every command the step
// runs starts with a real program and never relies on an outer shell.
func assertOhMyZshCommandsAreShellIndependent(t *testing.T, calls *[]packageCommandCall) {
	t.Helper()

	var commands []string
	for _, call := range *calls {
		if call.runner == "oh-my-zsh" {
			commands = append(commands, call.command)
		}
	}
	if len(commands) != 2 {
		t.Fatalf("expected a download and a run command, got %v", commands)
	}

	for _, command := range commands {
		firstToken := strings.Fields(command)[0]
		if strings.Contains(firstToken, "=") {
			t.Errorf("command starts with an assignment, which Termux would exec: %q", command)
		}
		if strings.Contains(command, "$(") {
			t.Errorf("command needs an outer shell to expand a substitution: %q", command)
		}
	}

	if !strings.Contains(commands[0], "curl ") || !strings.Contains(commands[0], "-o ") {
		t.Errorf("the first command must download the installer: %q", commands[0])
	}
	for _, want := range []string{"env ", "ZSH=", "RUNZSH=no", "CHSH=no", "KEEP_ZSHRC=yes", "sh "} {
		if !strings.Contains(commands[1], want) {
			t.Errorf("the run command is missing %q: %q", want, commands[1])
		}
	}
}

// withZshMocks extends the package mocks so the Oh My Zsh installer behaves like
// the real one: it records its calls and creates the installation directory that
// the step verifies afterwards.
func withZshMocks(t *testing.T, home string) *[]packageCommandCall {
	t.Helper()

	calls := withPackageCommandMocks(t, nil)

	runOhMyZshInstaller = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		*calls = append(*calls, packageCommandCall{runner: "oh-my-zsh", command: command})
		// The download writes to a temporary file; only the second command
		// installs anything.
		if !strings.Contains(command, " -o ") {
			omzDir := filepath.Join(home, ".oh-my-zsh")
			if err := os.MkdirAll(omzDir, 0o755); err != nil {
				t.Fatalf("mock installer could not create the directory: %v", err)
			}
			// The step verifies the entry point .zshrc sources, not the directory.
			if err := os.WriteFile(filepath.Join(omzDir, ohMyZshEntrypoint), []byte("# mock\n"), 0o644); err != nil {
				t.Fatalf("mock installer could not create the entry point: %v", err)
			}
		}
		return &system.ExecResult{Command: command}
	}

	return calls
}

func callsContain(calls *[]packageCommandCall, runner string) bool {
	for _, call := range *calls {
		if call.runner == runner {
			return true
		}
	}
	return false
}

// TestShouldInstallOhMyZsh covers the guard: only a missing installation is
// installed. A real directory and a symlinked one both count as present, which
// is what stops the installer from overwriting a clone that manages itself.
func TestShouldInstallOhMyZsh(t *testing.T) {
	t.Run("missing directory wants an install", func(t *testing.T) {
		if !shouldInstallOhMyZsh(filepath.Join(t.TempDir(), ".oh-my-zsh")) {
			t.Error("a missing ~/.oh-my-zsh must be installed")
		}
	})

	t.Run("a complete installation is left alone", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".oh-my-zsh")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ohMyZshEntrypoint), []byte("# omz\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if shouldInstallOhMyZsh(dir) {
			t.Error("an existing ~/.oh-my-zsh must not be reinstalled")
		}
	})

	t.Run("a directory without the entry point is an interrupted install", func(t *testing.T) {
		// The installer creates the directory early, so a failed clone leaves one
		// behind. Treating it as installed made the failure unrecoverable.
		dir := filepath.Join(t.TempDir(), ".oh-my-zsh")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if !shouldInstallOhMyZsh(dir) {
			t.Error("a partial ~/.oh-my-zsh must be reinstalled")
		}
	})

	t.Run("symlinked directory counts as installed", func(t *testing.T) {
		home := t.TempDir()
		real := filepath.Join(home, "ohmyzsh-real")
		if err := os.MkdirAll(real, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(real, ohMyZshEntrypoint), []byte("# omz\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(home, ".oh-my-zsh")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("symlinks are not available here: %v", err)
		}
		if shouldInstallOhMyZsh(link) {
			t.Error("a symlinked ~/.oh-my-zsh must not be reinstalled")
		}
	})

	t.Run("broken symlink is not an install", func(t *testing.T) {
		home := t.TempDir()
		link := filepath.Join(home, ".oh-my-zsh")
		if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
			t.Skipf("symlinks are not available here: %v", err)
		}
		if !shouldInstallOhMyZsh(link) {
			t.Error("a broken symlink is not a working installation")
		}
	})
}

// TestStepInstallShellNeverTouchesAnExistingOhMyZsh pins the reason the vendored
// tree was removed: writing into a real clone dirties its tracked files and
// breaks `omz update`.
func TestStepInstallShellNeverTouchesAnExistingOhMyZsh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withZshMocks(t, home)

	omz := filepath.Join(home, ".oh-my-zsh")
	if err := os.MkdirAll(filepath.Join(omz, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(omz, "oh-my-zsh.sh")
	if err := os.WriteFile(marker, []byte("# user clone\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("zsh step failed: %v", err)
	}

	if callsContain(calls, "oh-my-zsh") {
		t.Error("an existing ~/.oh-my-zsh must not be reinstalled")
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the existing installation was removed: %v", err)
	}
	if string(got) != "# user clone\n" {
		t.Errorf("the existing installation was overwritten: %q", got)
	}
}

// TestPatchZshForWMHandlesTheShippedZshrc keeps the window-manager patching
// verified against the real configuration instead of a mock, so the markers it
// depends on cannot silently disappear.
func TestPatchZshForWMHandlesTheShippedZshrc(t *testing.T) {
	source := filepath.Join(repoRoot(t), repoAssetZshrc)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	for _, wm := range []string{"tmux", "zellij", "herdr", "none"} {
		t.Run(wm, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), ".zshrc")
			if err := os.WriteFile(target, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := system.PatchZshForWM(target, wm, true); err != nil {
				t.Fatalf("patch failed: %v", err)
			}
			patched, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}

			wantWMVar := wm != "none"
			if got := strings.Contains(string(patched), `WM_VAR="`); got != wantWMVar {
				t.Errorf("WM_VAR present = %v, want %v", got, wantWMVar)
			}
			if got := strings.Contains(string(patched), `WM_CMD=(`); got != wantWMVar {
				t.Errorf("WM_CMD present = %v, want %v", got, wantWMVar)
			}

			expects := map[string]string{
				"tmux":   `WM_CMD=(tmux new-session -A -s main)`,
				"zellij": `WM_CMD=(zellij attach -c main)`,
				"herdr":  `WM_CMD=(herdr)`,
			}
			if expect, ok := expects[wm]; ok && !strings.Contains(string(patched), expect) {
				t.Errorf("patched .zshrc is missing %s", expect)
			}
		})
	}
}

func TestDryRunSkipsEveryStep(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "herdr", InstallNvim: true}

	for _, stepID := range []string{"backup", "deps", "clone", "homebrew", "shell", "wm", "nvim", "wslconfig", "cleanup", "setshell"} {
		if err := executeStep(stepID, &m); err != nil {
			t.Errorf("dry run must not run step %q: %v", stepID, err)
		}
	}

	if m.RepoDir != "" || m.WorkDir != "" {
		t.Errorf("dry run must not clone anything: WorkDir=%q RepoDir=%q", m.WorkDir, m.RepoDir)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dry run wrote into HOME: %v", entries)
	}
}

// TestStepInstallShellInstallsGitConfig covers the two files the installer copies
// from the repository root. Both are copied verbatim, so equality is asserted
// rather than the prefix check the .zshrc case needs: a truncated or partial copy
// would still satisfy a prefix comparison.
func TestStepInstallShellInstallsGitConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withZshMocks(t, home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("zsh step failed: %v", err)
	}

	for _, tc := range []struct {
		asset string
		dst   string
	}{
		{repoAssetGitconfig, ".gitconfig"},
		{repoAssetGitconfigPersonal, ".gitconfig-personal"},
	} {
		got, err := os.ReadFile(filepath.Join(home, tc.dst))
		if err != nil {
			t.Fatalf("%s was not installed: %v", tc.dst, err)
		}
		want, err := os.ReadFile(filepath.Join(m.RepoDir, tc.asset))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s does not match %s in the repository", tc.dst, tc.asset)
		}
	}
}

// TestInstallConfigDirReportsRemovedPaths pins the reporting requirement: a
// pruned file has to be visible in the install log instead of disappearing
// silently.
func TestInstallConfigDirReportsRemovedPaths(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "init.lua"), []byte("init"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dst, "lua", "plugins", "veil.lua")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("dead"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installConfigDir("nvim", src, dst); err != nil {
		t.Fatalf("installConfigDir failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), stale) {
		t.Errorf("the removed path was not reported in the install log: %q", out)
	}
}

// TestStepInstallNvimPrunesLeftoversAndKeepsUserState covers issue #13 through
// the step: a file the repository no longer ships is removed from
// ~/.config/nvim, while lazy-lock.json, which lazy.nvim owns, survives.
//
// The fixture ships its own lazy-lock.json on purpose, so the test proves the
// user's content wins rather than the file merely being copied from the source.
func TestStepInstallNvimPrunesLeftoversAndKeepsUserState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	repo := t.TempDir()
	nvimSrc := filepath.Join(repo, repoAssetNvim)
	if err := os.MkdirAll(filepath.Join(nvimSrc, "lua", "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(nvimSrc, "init.lua"):                     "init",
		filepath.Join(nvimSrc, "lua", "plugins", "editor.lua"): "editor",
		filepath.Join(nvimSrc, "lazy-lock.json"):               `{"repo":"pinned"}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	nvimDir := filepath.Join(home, ".config", "nvim")
	staleFile := filepath.Join(nvimDir, "lua", "plugins", "veil.lua")
	staleDirFile := filepath.Join(nvimDir, "lua", "config", "legacy", "utils.lua")
	lock := filepath.Join(nvimDir, "lazy-lock.json")
	for path, content := range map[string]string{
		staleFile:    "-- removed upstream\n",
		staleDirFile: "-- removed upstream\n",
		lock:         `{"user":true}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := NewModel()
	// Termux keeps the optional Claude and OpenCode installers, which shell out to
	// the network, out of this test; the directory copy is platform-independent.
	m.SystemInfo = &system.SystemInfo{IsTermux: true}
	m.Choices = UserChoices{OS: "termux", InstallNvim: true}
	m.RepoDir = repo

	if err := stepInstallNvim(&m); err != nil {
		t.Fatalf("nvim step failed: %v", err)
	}

	for _, path := range []string{staleFile, staleDirFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("a path the repository no longer ships must be removed: %s: %v", path, err)
		}
	}
	got, err := os.ReadFile(lock)
	if err != nil {
		t.Fatalf("lazy-lock.json must survive the install: %v", err)
	}
	if string(got) != `{"user":true}` {
		t.Errorf("lazy-lock.json content = %q, want the user's state", got)
	}
	if _, err := os.Stat(filepath.Join(nvimDir, "init.lua")); err != nil {
		t.Errorf("the repository Neovim config was not installed: %v", err)
	}
}

// TestShippedZshrcUsesAbsoluteWSLgWaylandSocket pins the WSLg display fix that
// the tracked template lost. WAYLAND_DISPLAY must be absolute because WSLg's
// socket lives outside XDG_RUNTIME_DIR, so a relative "wayland-0" resolves to a
// non-existent path and every Wayland client (wl-copy, wl-paste, browsers)
// fails to connect. The installer copies this asset verbatim, so asserting it
// here is the only thing that stops the regression from shipping again.
func TestShippedZshrcUsesAbsoluteWSLgWaylandSocket(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetZshrc))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)

	const waylandSocket = "/mnt/wslg/runtime-dir/wayland-0"

	// The probe and the assignment have to agree on the same absolute socket.
	if want := "[[ -S " + waylandSocket + " ]]"; !strings.Contains(script, want) {
		t.Errorf(".zshrc no longer probes the WSLg socket before exporting it: %s", want)
	}
	if want := `export WAYLAND_DISPLAY="` + waylandSocket + `"`; !strings.Contains(script, want) {
		t.Errorf(".zshrc is missing the absolute WSLg assignment: %s", want)
	}

	// DISPLAY is guarded by WSLg's own X11 socket, not by the interop socket
	// that can be missing while WSLg works.
	if want := "[[ -d /mnt/wslg/.X11-unix ]]"; !strings.Contains(script, want) {
		t.Errorf(".zshrc no longer guards DISPLAY with WSLg's X11 socket: %s", want)
	}

	// Collect every assignment so a relative one or a reintroduced duplicate
	// fails here instead of silently overwriting the correct value at runtime.
	type assignment struct {
		line  int
		value string
		text  string
	}
	var found []assignment
	for i, line := range strings.Split(script, "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		idx := strings.Index(text, "WAYLAND_DISPLAY=")
		if idx < 0 {
			continue
		}
		value := strings.Trim(strings.TrimPrefix(text[idx:], "WAYLAND_DISPLAY="), `"'`)
		found = append(found, assignment{line: i + 1, value: value, text: text})
	}

	if len(found) != 1 {
		t.Fatalf("expected exactly one WAYLAND_DISPLAY assignment, found %d: %v", len(found), found)
	}
	if got := found[0]; !strings.HasPrefix(got.value, "/") {
		t.Errorf("line %d assigns a relative WAYLAND_DISPLAY (%q): %s", got.line, got.value, got.text)
	}
	if got := found[0].value; got != waylandSocket {
		t.Errorf("WAYLAND_DISPLAY = %q, want the absolute WSLg socket %q", got, waylandSocket)
	}
}

// ---------------------------------------------------------------------------
// The utilities section: the system theme switch
// ---------------------------------------------------------------------------

// withThemeCommandMock replaces the one seam the theme utility runs its commands
// through, so a test can drive every branch without a desktop, a PATH shim or a
// real gsettings. The results are handed back in call order; a call past the end
// of the list succeeds with no output. It returns the commands the utility ran,
// in order, which is what proves what was and was not executed.
func withThemeCommandMock(t *testing.T, results ...*system.ExecResult) *[]string {
	t.Helper()

	original := runThemeCommand
	calls := []string{}
	i := 0
	runThemeCommand = func(command string, opts *system.ExecOptions) *system.ExecResult {
		calls = append(calls, command)
		var result *system.ExecResult
		if i < len(results) {
			result = results[i]
		}
		i++
		if result == nil {
			result = &system.ExecResult{}
		}
		result.Command = command
		return result
	}
	t.Cleanup(func() { runThemeCommand = original })
	return &calls
}

// TestDetectThemeSwitchIsNarrowAndHonest pins the detection rule: a desktop is
// offered only when both the session says which desktop it is and the tool that
// switches it is actually on PATH. The table includes the near misses on purpose
// - a GNOME session without gsettings, a Plasma session without the reader that
// makes the change reversible - because offering a switch that cannot be undone
// is the failure this rule exists to prevent.
func TestDetectThemeSwitchIsNarrowAndHonest(t *testing.T) {
	hasAll := func(names ...string) func(string) bool {
		present := map[string]bool{}
		for _, name := range names {
			present[name] = true
		}
		return func(name string) bool { return present[name] }
	}

	tests := []struct {
		name          string
		goos          string
		desktop       string
		session       string
		tools         []string
		wantID        string
		wantDetection bool
	}{
		{"a GNOME session with gsettings", "linux", "GNOME", "", []string{"gsettings"}, "gnome", true},
		{"a GNOME session without gsettings", "linux", "GNOME", "", nil, "", false},
		{"a Plasma 6 session", "linux", "KDE", "plasma6", []string{"plasma-apply-colorscheme", "kreadconfig6"}, "kde", true},
		{"a Plasma session without the reader", "linux", "KDE", "plasma6", []string{"plasma-apply-colorscheme"}, "", false},
		{"macOS", "darwin", "", "", []string{"defaults"}, "macos", true},
		{"macOS without defaults", "darwin", "", "", nil, "", false},
		{"a bare server with the tools installed", "linux", "", "", []string{"gsettings", "plasma-apply-colorscheme", "kreadconfig6"}, "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			target, ok := detectThemeSwitch(tt.goos, tt.desktop, tt.session, hasAll(tt.tools...))
			if ok != tt.wantDetection {
				t.Fatalf("detectThemeSwitch = %v, want %v", ok, tt.wantDetection)
			}
			if target.ID != tt.wantID {
				t.Errorf("detected target = %q, want %q", target.ID, tt.wantID)
			}
			if !ok {
				return
			}
			for field, value := range map[string]string{
				"Read": target.Read, "Dark": target.Dark, "Light": target.Light,
			} {
				if strings.TrimSpace(value) == "" {
					t.Errorf("the %s target's %s command is empty, so the utility cannot run", target.ID, field)
				}
			}
			if target.Restore("Breeze Dark") == "" {
				t.Errorf("the %s target cannot build a restore command", target.ID)
			}
		})
	}
}

// TestThemeSwitchIsNotOfferedOnTermuxOrAnUnknownHost pins the two facts
// detection cannot see from the session alone: Termux has no desktop, and a host
// whose platform was never detected has nothing to describe.
//
// It is host-independent on purpose, and the reason is the one the macOS runner
// exposed: both refusals happen in the wrapper, before the rule ever asks the
// PATH for a tool. A desktop-shaped environment is set anyway so the test says
// what it means -- even a GNOME session is refused on Termux -- but no assertion
// here reads the runner's PATH, so `defaults` being present on macOS cannot
// change the answer. The other half, that the same GNOME session with gsettings
// present does detect the gnome target, is the "a GNOME session with gsettings"
// row of TestDetectThemeSwitchIsNarrowAndHonest, so this test is not vacuous.
func TestThemeSwitchIsNotOfferedOnTermuxOrAnUnknownHost(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")

	if _, ok := currentThemeSwitch(&system.SystemInfo{IsTermux: true}); ok {
		t.Error("a theme switch was offered on Termux, which has no desktop theme to switch")
	}
	if _, ok := currentThemeSwitch(nil); ok {
		t.Error("a theme switch was offered with no detected host")
	}
}

// TestThemeStatePathLivesUnderTheStateDirectory pins the record's location: the
// installer owns it, so it lives with the other state file rather than in a
// configuration directory the user is invited to edit.
func TestThemeStatePathLivesUnderTheStateDirectory(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	want := filepath.Join(stateHome, "dotfiles", "theme.json")
	if got := themeStatePath(); got != want {
		t.Errorf("themeStatePath() = %q, want %q", got, want)
	}
}

// TestThemeSwitchRecordsTheSettingItReplaces is the reversibility contract: the
// switch reads the setting that is there, applies the new one, and stores what
// it replaced in its own record so the change can be undone. The commands are
// asserted in order, so a version that applied before reading cannot pass.
func TestThemeSwitchRecordsTheSettingItReplaces(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t,
		&system.ExecResult{Output: "'default'\n"},
		&system.ExecResult{},
	)

	rec, notice, err := applyTheme(target, true)
	if err != nil {
		t.Fatalf("applyTheme: %v", err)
	}
	if notice == "" {
		t.Error("a successful switch said nothing about what it changed")
	}
	if want := []string{target.Read, target.Dark}; !reflect.DeepEqual(*calls, want) {
		t.Errorf("commands = %v, want %v", *calls, want)
	}
	if rec == nil || rec.Target != "gnome" || rec.Value != "default" || rec.WasDark || !rec.ToDark {
		t.Fatalf("record = %+v, want the gnome target with the replaced value \"default\" and ToDark true", rec)
	}
	stored := readThemeRecord()
	if stored == nil || stored.Value != "default" || stored.Target != "gnome" {
		t.Errorf("stored record = %+v, want the deprecated value \"default\" for gnome", stored)
	}
}

// TestThemeUndoPutsTheRecordedSettingBack is the other half of reversibility:
// the undo re-applies exactly the value the record holds, and it records the
// value it is itself replacing, so the utility can always be run again.
func TestThemeUndoPutsTheRecordedSettingBack(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t,
		&system.ExecResult{Output: "'prefer-dark'\n"},
		&system.ExecResult{},
	)
	rec := themeRecord{Target: "gnome", Value: "default", WasDark: false, ToDark: true}

	next, _, err := undoTheme(target, rec)
	if err != nil {
		t.Fatalf("undoTheme: %v", err)
	}
	wantRestore := "gsettings set org.gnome.desktop.interface color-scheme 'default'"
	if got := (*calls)[len(*calls)-1]; got != wantRestore {
		t.Errorf("undo ran %q, want %q", got, wantRestore)
	}
	if next == nil || next.Value != "prefer-dark" || !next.WasDark {
		t.Errorf("record after undo = %+v, want the value the undo replaced (\"prefer-dark\", WasDark true)", next)
	}
}

// TestThemeSwitchRefusesAValueItCannotPutBack covers the case that makes the
// utility safe: a setting the tool reports in a form the restore command cannot
// safely carry back is left exactly as it was, and the reason is reported rather
// than swallowed.
func TestThemeSwitchRefusesAValueItCannotPutBack(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t, &system.ExecResult{Output: "'$(touch /tmp/pwned)'\n"})

	if _, _, err := applyTheme(target, true); err == nil {
		t.Error("a value the restore command cannot carry was accepted")
	}
	if len(*calls) != 1 {
		t.Errorf("commands = %v, want only the read: nothing may be written when the value cannot be put back", *calls)
	}
	if readThemeRecord() != nil {
		t.Error("a refused switch wrote a record")
	}
}

// TestThemeSwitchSurfacesAReadFailureAsAnError keeps the honest failure: when the
// tool cannot be read, the utility says so instead of changing a setting it could
// not undo.
func TestThemeSwitchSurfacesAReadFailureAsAnError(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t, &system.ExecResult{Error: &system.ExecError{Command: target.Read, ExitCode: 1}})

	if _, _, err := applyTheme(target, true); err == nil {
		t.Error("an unreadable setting was switched anyway")
	}
	if len(*calls) != 1 {
		t.Errorf("commands = %v, want only the read", *calls)
	}
}

// TestThemeUndoRefusesARecordFromAnotherDesktop pins that a record is only
// usable against the desktop that wrote it: a machine that changed desktops must
// not have an old value written into the new desktop through a mismatched tool.
func TestThemeUndoRefusesARecordFromAnotherDesktop(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t)

	if _, _, err := undoTheme(target, themeRecord{Target: "macos", Value: "Dark"}); err == nil {
		t.Error("a record written by another desktop was applied")
	}
	if len(*calls) != 0 {
		t.Errorf("commands = %v, want none", *calls)
	}
}

// TestDryRunSkipsTheThemeUtility is the regression guard for the defect --dry-run
// exists to prevent: the interactive path ran for real under a flag that
// documents a no-op run. The switch and the undo are gated exactly as
// executeStep is, so under the flag neither runs a command nor writes a record.
func TestDryRunSkipsTheThemeUtility(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t)

	if _, _, err := applyTheme(target, true); err != nil {
		t.Fatalf("dry run switch returned an error: %v", err)
	}
	if _, _, err := undoTheme(target, themeRecord{Target: "gnome", Value: "default"}); err != nil {
		t.Fatalf("dry run undo returned an error: %v", err)
	}

	if len(*calls) != 0 {
		t.Errorf("a dry run ran %d theme command(s): %v", len(*calls), *calls)
	}
	if readThemeRecord() != nil {
		t.Error("a dry run wrote a theme record")
	}
}

// TestReadThemeRecordTreatsAPartialFileAsNoRecord pins the same rule the last
// install record follows: a file that cannot be read, cannot be parsed or names
// no desktop is not a record, because a restore must never be built from half a
// file.
func TestReadThemeRecordTreatsAPartialFileAsNoRecord(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	if rec := readThemeRecord(); rec != nil {
		t.Errorf("a missing record read as %+v", rec)
	}

	path := themeStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := readThemeRecord(); rec != nil {
		t.Errorf("a corrupt record read as %+v", rec)
	}

	if err := os.WriteFile(path, []byte(`{"value":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := readThemeRecord(); rec != nil {
		t.Errorf("a record naming no desktop read as %+v", rec)
	}
}

// --- The unified dotfiles theme ---------------------------------------------
//
// The guards below cover the rule the unified theme exists for: the list is
// derived from themes/*.toml and never typed, a theme the repository cannot fill
// is reported rather than offered, and a shipped block that stops matching its
// definition fails here instead of being found by putting two terminals side by
// side.

// themeIDsOnDisk reads themes/*.toml directly, so the derivation guard compares
// the loader against the files rather than against itself.
func themeIDsOnDisk(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join(repoRoot(t), "themes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".toml") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".toml"))
	}
	sort.Strings(ids)
	return ids
}

// TestThemeListIsDerivedFromDefinitions covers the rule that adding a theme is
// adding a definition file: the menu's list is read from themes/*.toml, and
// this fails when the two lists disagree.
func TestThemeListIsDerivedFromDefinitions(t *testing.T) {
	onDisk := themeIDsOnDisk(t)
	if len(onDisk) < 2 {
		t.Fatalf("themes/ holds %d definitions, so this guard proves nothing", len(onDisk))
	}

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	got := make([]string, len(defs))
	for i, def := range defs {
		got[i] = def.ID
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, onDisk) {
		t.Errorf("the theme list is %v but themes/ holds %v: the list is typed somewhere instead of derived", got, onDisk)
	}
	t.Logf("themes derived from themes/*.toml: %v", got)
}

// TestOnlyCompleteThemesAreOffered covers the honest-degradation rule: a theme
// missing canonical roles is partial, is reported with its reason, and is never
// offered as a switch.
func TestOnlyCompleteThemesAreOffered(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	var complete, partial []string
	for _, def := range defs {
		if def.Complete() {
			complete = append(complete, def.ID)
			continue
		}
		if !def.Partial {
			t.Errorf("theme %q misses %v but is not marked partial", def.ID, def.missingRoles())
		}
		if def.PartialReason == "" {
			t.Errorf("theme %q is partial without saying why", def.ID)
		}
		partial = append(partial, def.ID)
	}
	if len(complete) == 0 {
		t.Fatal("no theme defines every canonical role, so the switch would offer nothing")
	}

	if offered, want := offeredThemeIDs(defs), complete; !reflect.DeepEqual(offered, want) {
		t.Errorf("the menu offers %v, the complete themes are %v: a partial theme must never be offered", offered, want)
	}
	t.Logf("complete themes (offered): %v; partial themes (reported, not offered): %v", complete, partial)
}

// TestOnlyThemesThatCanBePreviewedAreOffered covers the completion rule the
// task made explicit: offering a theme means it can be applied *and* shown, so
// a theme must carry the two [syntax] members the preview reads. A definition
// with all twenty-two terminal roles but no syntax cannot be previewed and is
// therefore not complete and not offered. The guard has teeth: leaving syntax
// out of Complete() offers the fixture below, which the preview refuses.
func TestOnlyThemesThatCanBePreviewedAreOffered(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	offered := offeredThemeIDs(defs)
	for _, id := range offered {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		for _, role := range themeSyntaxRequired {
			if def.Syntax[role] == "" {
				t.Errorf("theme %q is offered but defines no [syntax] %s, so it cannot be previewed", id, role)
			}
		}
		if _, err := themePreviewColors(def); err != nil {
			t.Errorf("theme %q is offered but cannot be previewed: %v", id, err)
		}
	}
	if len(offered) == 0 {
		t.Fatal("no theme is offered, so this guard proves nothing")
	}

	// Every canonical role, and nothing in [syntax]: applicable but not showable.
	full := themeDefinition{ID: "no-syntax", Palette: map[string]string{}, Prompt: map[string]string{}, Syntax: map[string]string{}}
	for _, role := range themePaletteRoles {
		full.Palette[role] = "#112233"
	}
	if full.Complete() {
		t.Error("a definition with every terminal role and no [syntax] reported itself complete")
	}
	if ids := offeredThemeIDs([]themeDefinition{full}); len(ids) != 0 {
		t.Errorf("the menu offers %v, which the preview would refuse", ids)
	}

	// The two members the preview reads are what closes the gap.
	full.Syntax = map[string]string{"keyword_dark": "#cba6f7", "string_dark": "#a6e3a1"}
	if !full.Complete() {
		t.Error("a definition with the two [syntax] members the preview reads is not complete")
	}
	if ids := offeredThemeIDs([]themeDefinition{full}); len(ids) != 1 {
		t.Errorf("the menu offers %v, want the now-showable theme", ids)
	}
}

// TestNoInventedThemeRoleSlipsIn covers the other half of the rule: a colour has
// to belong to a role this program knows about, and a definition has to say
// where its values come from. A palette role outside the canonical set and a
// syntax role outside the four the code display reads are both refused at load
// time rather than painted; a definition that names no provenance is refused by
// the guard below, so nothing lands without a cited source.
func TestNoInventedThemeRoleSlipsIn(t *testing.T) {
	inventedPalette := "[theme]\nid = \"invented\"\npartial = true\npartial_reason = \"fixture\"\n\n[palette]\nbase = \"#000000\"\nmauve = \"#000000\"\n"
	if _, err := parseThemeDefinition([]byte(inventedPalette)); err == nil {
		t.Error("a palette role outside the canonical twenty-two was accepted")
	}

	inventedSyntax := "[theme]\nid = \"invented\"\npartial = true\npartial_reason = \"fixture\"\n\n[syntax]\nkeyword_dark = \"#000000\"\nkeyword_extra = \"#000000\"\n"
	if _, err := parseThemeDefinition([]byte(inventedSyntax)); err == nil {
		t.Error("a [syntax] role outside the four the preview reads was accepted")
	}

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	for _, def := range defs {
		if strings.TrimSpace(def.Provenance) == "" {
			t.Errorf("theme %q names no provenance, so its colours have no cited source", def.ID)
		}
	}
}

// TestEveryOfferedThemePaintsEveryTool is the requirement as a guard: every
// theme the menu offers paints **every** tool the switch names, so no theme
// leaves anything on the old palette. The list it reads is the coverage data, so
// a theme that loses a role table, an artifact or a name is named here with the
// tool it stopped painting rather than failing as "something is wrong".
//
// Its teeth are the definitions: dropping a [bat] table (or an [nvim] name, or a
// theme's place in the artifact table) makes this fail with the theme and the
// tool, which is the case the requirement is about. Nothing is pinned by hand,
// so an exclusion cannot come back unnoticed either.
func TestEveryOfferedThemePaintsEveryTool(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	offered := offeredThemeIDs(defs)
	if len(offered) == 0 {
		t.Fatal("no theme is offered, so this guard proves nothing")
	}
	if len(themeTools) == 0 {
		t.Fatal("the switch names no tool, so this guard proves nothing")
	}

	names := make([]string, len(themeTools))
	for i, tool := range themeTools {
		names[i] = tool.Name
	}
	for _, id := range offered {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		covered, uncovered := themeCoverage(def)
		if len(uncovered) > 0 {
			t.Errorf("theme %q leaves out %s: every offered theme must paint every tool (%s)",
				id, strings.Join(uncovered, ", "), strings.Join(names, ", "))
		}
		if len(covered) != len(themeTools) {
			t.Errorf("theme %q covers %d of the %d tools the switch names: %v",
				id, len(covered), len(themeTools), covered)
		}
		t.Logf("%s: covers %d of %d tools; leaves out %v", id, len(covered), len(themeTools), uncovered)
	}

	// The row the menu draws reads the same coverage, so the guard cannot pass
	// while a row still names a tool it leaves out. A theme that covered
	// everything but still printed an exclusion would be a reader being told
	// something untrue.
	for _, id := range offered {
		def, ok := themeByID(defs, id)
		if !ok {
			continue
		}
		if row := dotfilesThemeRow(def); strings.Contains(row, "(not ") {
			t.Errorf("the row for %q still names an exclusion though it leaves out no tool: %q", id, row)
		}
	}

	// A partial theme is still reported, never offered, and never counted here.
	for _, def := range defs {
		if def.Complete() {
			continue
		}
		t.Logf("partial theme %q is reported and not offered: %v", def.ID, def.missingRequiredRoles())
	}
}

// kanagawaNushellValues are the eighteen colours the shipped
// dotfiles-nushell/config.nu held that themes/dotfiles.toml does not define, in
// the order issue #240 lists them. They are the palette the shell kept while
// every other tool changed, and the guard below refuses any of them in a
// generated nushell block.
var kanagawaNushellValues = []string{
	"1d1f21", "1f1f28", "54546d", "6a9589", "7e9cd8", "85b5ba",
	"92a2d5", "957fb8", "98bb6c", "aca1cf", "c4c9c6", "c9c7cd",
	"d27e99", "dca561", "e29eca", "e46876", "e6c384", "ea83a5",
}

// nushellLSColourRE pulls the SGR triple out of an LS_COLORS entry. nushell
// paints `ls` output from decimal RGB triples, not from #rrggbb, so the shared
// "invent no colour" guard cannot read them and they are decoded here.
var nushellLSColourRE = regexp.MustCompile(`38;2;(\d+);(\d+);(\d+)`)

// nushellRGB renders a #rrggbb colour as the `r;g;b` triple LS_COLORS carries.
func nushellRGB(t *testing.T, hex string) string {
	t.Helper()

	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		t.Fatalf("the definition holds %q, which is not a #rrggbb colour", hex)
	}
	parts := make([]string, 3)
	for i := 0; i < 3; i++ {
		value, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			t.Fatalf("parse %q: %v", hex, err)
		}
		parts[i] = strconv.FormatUint(value, 10)
	}
	return strings.Join(parts, ";")
}

// TestNushellIsPaintedByTheThemeSwitch is the user's case from issue #240:
// choosing any theme but Kanagawa repainted every tool while `nu` kept the
// Kanagawa palette, and no row could say so because nushell was not one of the
// tools the model paints. The shell is a tool now, so the same derivation the
// other tools get has to paint it from the theme's own palette - the theme's
// record and its LS_COLORS table - and none of the eighteen Kanagawa values the
// shipped config.nu held may survive a switch.
//
// Its teeth are the definitions: a theme that cannot derive the roles the shell
// reads is named here with the shell, exactly as the coverage guard names it.
func TestNushellIsPaintedByTheThemeSwitch(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	inModel := false
	for _, tool := range themeTools {
		if tool.ID == "nushell" {
			inModel = true
		}
	}
	if !inModel {
		t.Fatal("nushell is not one of the tools the switch names, so no switch repaints it and no row can say it was left behind")
	}

	art := artifactByName(t, "nushell")
	if path := themeInstalledPath(art, "/home/fixture"); !strings.HasSuffix(path, "nushell/config.nu") {
		t.Errorf("the switch looks for the shell's colours at %q, which is not nushell's own config.nu", path)
	}

	// The shipped file is what a fresh install copies, so no theme's own
	// hand-written value may survive there either. Issue #240 counted eighteen
	// Kanagawa values in it, and two of them sat in the `explore` block as well as
	// in the region the generator rewrites; both are gone now, one because the
	// region is generated and the other because the block takes the terminal's own
	// ANSI slots (which every theme paints) instead of a colour of its own.
	shipped, err := os.ReadFile(filepath.Join(repoRoot(t), art.Path))
	if err != nil {
		t.Fatalf("read %s: %v", art.Path, err)
	}
	for _, value := range kanagawaNushellValues {
		if regexp.MustCompile(`(?i)\b` + value + `\b`).MatchString(string(shipped)) {
			t.Errorf("%s still carries the Kanagawa value #%s: every colour the shell reads has to be a palette role or an ANSI slot the terminal paints", art.Path, value)
		}
	}

	checked := 0
	for _, id := range offeredThemeIDs(defs) {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		if _, uncovered := themeCoverage(def); slices.Contains(uncovered, "nushell") {
			t.Errorf("theme %q leaves nushell out: %v", id, uncovered)
			continue
		}

		block, err := themeArtifactBlock(art, def)
		if err != nil {
			t.Errorf("theme %q covers nushell but cannot render its block: %v", id, err)
			continue
		}
		// The shell must take the theme's own palette rather than the one the shipped
		// file used to hold. A value the definition itself holds is the theme's own
		// colour - Kanagawa's theme is Kanagawa - so only the values it does not hold
		// are refused here; the LS_COLORS triples are checked below.
		holds := map[string]bool{}
		for _, hex := range def.Palette {
			holds[strings.ToLower(strings.TrimPrefix(hex, "#"))] = true
		}
		for _, value := range kanagawaNushellValues {
			if holds[value] {
				continue
			}
			if regexp.MustCompile(`(?i)\b` + value + `\b`).MatchString(block) {
				t.Errorf("the nushell block for %q still emits the Kanagawa value #%s: the shell has to take the theme's own palette", id, value)
			}
		}

		// Every colour the block emits has to be one the definition holds. The theme
		// record writes #rrggbb, which the shared guard reads; the LS_COLORS table
		// writes SGR triples, which it cannot, so they are decoded and checked here.
		owned := map[string]bool{}
		for _, hex := range def.Palette {
			owned["38;2;"+nushellRGB(t, hex)] = true
		}
		triples := nushellLSColourRE.FindAllString(block, -1)
		if len(triples) == 0 {
			t.Errorf("the nushell block for %q paints no LS_COLORS entry, so the table was left on the old palette", id)
		}
		for _, triple := range triples {
			if !owned[triple] {
				t.Errorf("the nushell block for %q emits the LS_COLORS colour %s, which themes/%s.toml does not hold", id, triple, id)
			}
			checked++
		}

		carries := false
		lower := strings.ToLower(block)
		for _, hex := range def.Palette {
			if strings.Contains(lower, strings.ToLower(hex)) {
				carries = true
				break
			}
		}
		if !carries {
			t.Errorf("the nushell block for %q carries none of the theme's own colours", id)
		}
	}
	if checked == 0 {
		t.Fatal("no LS_COLORS colour was checked, so this guard proves nothing")
	}
	t.Logf("nushell takes the palette of every offered theme; checked %d LS_COLORS colours", checked)
}

// TestNushellRegionAdoptionLeavesTheRestOfTheUserFileIntact covers the two
// promises the region adoption makes for the shell's own config: only the bytes
// between the two anchors the generator knows are rewritten, and Undo puts the
// file back byte-for-byte. The fixture has the shape the user's config.nu has -
// the colour region, the user's own PATH line beside it, and the rest of the
// file - because the region adoption is what has to leave that line alone.
func TestNushellRegionAdoptionLeavesTheRestOfTheUserFileIntact(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	before := []byte(`# the user's own nushell config
$env.PATH = ($env.PATH | prepend "/home/someone/.local/bin")

$env.LS_COLORS = (
    "di=38;2;146;162;213:" +
    "fi=38;2;201;199;205:" +
    "*=38;2;201;199;205"
)

let dark_theme = {
    separator: "#54546D"
    search_result: { fg: "#1F1F28", bg: "#E46876" }
    shape_variable: "#DCA561"
}

# the rest of the user's file
$env.EDITOR = "nvim"
`)

	art := artifactByName(t, "nushell")
	dst := themeInstalledPath(art, home)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, before, 0o644); err != nil {
		t.Fatal(err)
	}

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to the user's own config.nu: %v", err)
	}
	if rec == nil {
		t.Fatal("the adopted region was not recorded, so the change is not reversible")
	}
	if !strings.Contains(notice, "Adopted") {
		t.Errorf("the notice does not say the shell's region was adopted: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	upper := strings.ToUpper(string(got))
	for _, want := range []string{target.Palette["blue"], target.Palette["base"]} {
		if want != "" && !strings.Contains(upper, strings.ToUpper(want)) {
			t.Errorf("the rewritten region does not carry the %s colour %s", target.Name, want)
		}
	}
	for _, foreign := range []string{
		"# the user's own nushell config",
		`$env.PATH = ($env.PATH | prepend "/home/someone/.local/bin")`,
		"# the rest of the user's file",
		`$env.EDITOR = "nvim"`,
	} {
		if !strings.Contains(string(got), foreign) {
			t.Errorf("the switch rewrote the user's own line %q", foreign)
		}
	}
	for _, kanagawa := range []string{"#54546D", "#1F1F28", "#E46876", "#DCA561"} {
		if strings.Contains(upper, kanagawa) {
			t.Errorf("the adopted region still carries the Kanagawa value %s", kanagawa)
		}
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the adopted region: %v", err)
	}
	restored, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, before) {
		t.Errorf("undo did not restore the user's config.nu byte-for-byte\n got: %s\nwant: %s", restored, before)
	}
}

// TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme is the applicability
// half of the completion rule: a theme the menu offers must actually render a
// block for every tool it claims to cover, and every tool the switch names must
// have a generator to render it with. A definition marked complete but missing a
// role a generator reads would otherwise be offered and then fail on apply. It
// exercises every offered theme, including the transcribed ones, so "the
// generators exist" is proved for every palette rather than assumed, and it walks
// the switch's own tool list rather than a table of artifacts so a tool cannot be
// named by the menu with nothing behind it.
func TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	checked := 0
	offered := offeredThemeIDs(defs)
	for _, id := range offered {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		covered, _ := themeCoverage(def)
		for _, tool := range themeTools {
			if !slices.Contains(covered, tool.Name) {
				continue
			}
			generated := false
			for _, art := range themeActiveArtifacts {
				if art.Tool != tool.ID {
					continue
				}
				block, err := themeArtifactBlock(art, def)
				if err != nil {
					t.Errorf("theme %q covers %s but cannot render its block: %v", id, tool.Name, err)
					continue
				}
				if strings.TrimSpace(block) == "" {
					t.Errorf("theme %q covers %s but rendered an empty block", id, tool.Name)
					continue
				}
				generated = true
				checked++
			}
			if !generated {
				t.Errorf("theme %q covers %s but the switch generates no block for it", id, tool.Name)
			}
		}
		if len(covered) == 0 {
			t.Errorf("the offered theme %q covers no tool", id)
		}
	}
	if checked == 0 {
		t.Fatal("no tool block was generated, so this guard proves nothing")
	}
	t.Logf("generated %d covered-tool blocks across %d offered themes", checked, len(offered))
}

// TestThePromptDerivationCoversEveryRequiredRole pins the map the prompt's
// derived roles come from: every role Starship's table needs has to name a
// canonical palette role, and that role has to be one the definitions hold. A
// role missing from the map would silently drop out of a generated table, and a
// role naming a palette role no theme has would be the invented value the rule
// forbids.
func TestThePromptDerivationCoversEveryRequiredRole(t *testing.T) {
	paletteRoles := map[string]bool{}
	for _, role := range themePaletteRoles {
		paletteRoles[role] = true
	}

	covered := 0
	for _, role := range themePromptRequired {
		paletteRole, ok := themePromptDerivation[role]
		if !ok {
			t.Errorf("prompt role %q has no palette role in themePromptDerivation, so a theme with no [prompt] table cannot be painted", role)
			continue
		}
		if !paletteRoles[paletteRole] {
			t.Errorf("prompt role %q derives from %q, which is not one of the canonical palette roles", role, paletteRole)
			continue
		}
		covered++
	}
	if covered != len(themePromptRequired) {
		t.Errorf("the derivation covers %d of the %d prompt roles Starship needs", covered, len(themePromptRequired))
	}

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	for _, id := range offeredThemeIDs(defs) {
		def, _ := themeByID(defs, id)
		roles, err := themePromptRolesFor(def)
		if err != nil {
			t.Errorf("theme %q cannot supply the prompt roles: %v", id, err)
			continue
		}
		for _, role := range themePromptRequired {
			if roles[role] == "" {
				t.Errorf("theme %q has no value for the prompt role %q, so its Starship table would be a hole", id, role)
			}
		}
		// The derived values are logged so their provenance can be audited without
		// opening the definition: each one is a palette value this theme already
		// holds, chosen by the mapping above rather than by eye.
		if len(def.Prompt) == 0 {
			t.Logf("%s derives its prompt roles from its palette: mauve=%s pink=%s teal=%s peach=%s subtext0=%s overlay0=%s rosewater=%s",
				id, roles["mauve"], roles["pink"], roles["teal"], roles["peach"], roles["subtext0"], roles["overlay0"], roles["rosewater"])
		}
	}
}

// TestThePromptDerivationAgreesWithThePreview pins the one thing two readings of
// the same prompt roles could disagree about. The installer's own preview already
// reads three of them with a palette fallback (subtext0 -> bright_black, mauve ->
// bright_blue, peach -> yellow); the Starship table has to read them the same
// way, or the prompt and the interface would call two different colours "muted".
func TestThePromptDerivationAgreesWithThePreview(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	checked := 0
	for _, def := range defs {
		if len(def.Prompt) > 0 {
			// The definition declares the roles itself, so there is nothing derived
			// for the preview to disagree with.
			continue
		}
		roles, err := themePromptRolesFor(def)
		if err != nil {
			t.Errorf("theme %q cannot derive its prompt roles: %v", def.ID, err)
			continue
		}
		colors, err := themePreviewColors(def)
		if err != nil {
			t.Errorf("theme %q cannot be previewed: %v", def.ID, err)
			continue
		}

		pairs := []struct{ Prompt, Palette string }{
			{"subtext0", "bright_black"},
			{"mauve", "bright_blue"},
			{"peach", "yellow"},
		}
		for _, pair := range pairs {
			if got, want := roles[pair.Prompt], def.Palette[pair.Palette]; got != want {
				t.Errorf("theme %q derives the prompt role %q as %s, want the palette role %s = %s",
					def.ID, pair.Prompt, got, pair.Palette, want)
			}
		}
		readings := []struct {
			What    string
			Preview lipgloss.AdaptiveColor
			Role    string
		}{
			{"text_muted", colors.TextMuted, "bright_black"},
			{"secondary", colors.Secondary, "bright_blue"},
			{"warning", colors.Warning, "yellow"},
		}
		for _, reading := range readings {
			if reading.Preview != themePreviewColor(def.Palette[reading.Role]) {
				t.Errorf("theme %q: the preview's %s does not read %s, which the Starship derivation uses",
					def.ID, reading.What, reading.Role)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no theme derives its prompt roles, so this guard proves nothing")
	}
	t.Logf("the prompt derivation agrees with the preview for %d theme(s)", checked)
}

// TestTheNeovimColorschemeNamesResolve is the Neovim half of the completion
// rule: an offered theme's [nvim] name has to resolve to something the machine
// will have - a colorscheme the repository's Neovim plugin install provides, or a
// generated file this repository ships under dotfiles-nvim/nvim/colors/ whose own
// name is the name the definition selects. A name that resolves to nothing would
// be a row that says it paints Neovim while the switch leaves it on the old
// colorscheme.
func TestTheNeovimColorschemeNamesResolve(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	generated, plugin := 0, 0
	for _, id := range offeredThemeIDs(defs) {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		if def.Nvim == "" {
			t.Errorf("theme %q is offered but names no Neovim colorscheme", id)
			continue
		}
		if slices.Contains(themeNvimPluginColorschemes, def.Nvim) {
			plugin++
			continue
		}

		path := themeNvimGeneratedFile(id)
		if path == "" {
			t.Errorf("theme %q names the Neovim colorscheme %q, which neither the repository's plugin install nor a generated file provides",
				id, def.Nvim)
			continue
		}
		// The file has to be one the installer copies, or it never reaches the
		// machine the switch runs on.
		if !strings.HasPrefix(path, repoAssetNvim+"/") {
			t.Errorf("the generated colorscheme %s is not under %s, which is the directory the Neovim step installs", path, repoAssetNvim)
		}
		if want := def.Nvim + ".lua"; filepath.Base(path) != want {
			t.Errorf("theme %q selects the colorscheme %q but ships %s: the file name is what :colorscheme resolves",
				id, def.Nvim, path)
		}
		data, err := os.ReadFile(filepath.Join(repoRoot(t), path))
		if err != nil {
			t.Errorf("read the generated colorscheme %s: %v", path, err)
			continue
		}
		if !strings.Contains(string(data), `vim.g.colors_name = "`+def.Nvim+`"`) {
			t.Errorf("the generated colorscheme %s does not name itself %q, so the file and the selection disagree", path, def.Nvim)
		}
		generated++
	}
	if generated == 0 {
		t.Fatal("no offered theme ships a generated colorscheme, so this guard proves nothing")
	}
	if plugin == 0 {
		t.Fatal("no offered theme is painted by a plugin colorscheme, so the other half of this guard proves nothing")
	}
	t.Logf("%d offered theme(s) select a generated colorscheme, %d select a plugin's", generated, plugin)
}

// TestTheNvimColorschemeLineNamesOnlyAColorschemeTheMachineHas reproduces the
// user's case: a generated theme was applied, the switch wrote the line naming
// its colorscheme, and the generated file had never reached
// ~/.config/nvim/colors, so Neovim raised E185 on every start. The line may only
// be written after the name is made reachable on this machine, and a plugin's
// colorscheme may only be named when the plugin is installed. Removing either
// guarantee makes this test fail again.
func TestTheNvimColorschemeLineNamesOnlyAColorschemeTheMachineHas(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	// nocturne is the theme the user applied: a generated colorscheme, not a
	// plugin's, so it resolves only if the generated file is on the machine.
	generated, ok := themeByID(defs, "nocturne")
	if !ok {
		t.Fatal("the nocturne definition is missing")
	}
	if themeNvimGeneratedFile(generated.ID) == "" {
		t.Fatalf("nocturne no longer uses a generated colorscheme, so this test no longer reproduces the user's case")
	}

	// The user's machine exactly: Neovim is installed (its colorscheme.lua is
	// there) but ~/.config/nvim/colors/ was never written.
	home := t.TempDir()
	installThemeFiles(t, home, "nvim")
	colorsFile := filepath.Join(home, ".config", "nvim", "colors", generated.Nvim+".lua")
	if _, err := os.Stat(colorsFile); !os.IsNotExist(err) {
		t.Fatalf("fixture: %s already exists, so this does not reproduce the user's case", colorsFile)
	}

	if _, _, err := applyDotfilesTheme(home, repoRoot(t), generated); err != nil {
		t.Fatalf("apply the %s theme: %v", generated.Name, err)
	}

	installed, err := os.ReadFile(themeInstalledPath(artifactByName(t, "nvim"), home))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), `colorscheme = "`+generated.Nvim+`"`) {
		t.Fatalf("the switch did not write the %s line, so the theme never reaches Neovim:\n%s", generated.Nvim, installed)
	}
	data, err := os.ReadFile(colorsFile)
	if err != nil {
		t.Fatalf("the line names %q but Neovim cannot find it: %v (this is the E185 the user saw)", generated.Nvim, err)
	}
	if !strings.Contains(string(data), `vim.g.colors_name = "`+generated.Nvim+`"`) {
		t.Errorf("%s does not register itself as %q, so :colorscheme %s would still fail", colorsFile, generated.Nvim, generated.Nvim)
	}

	// The other half of the rule: a plugin theme whose plugin this machine does
	// not have is not named, so Neovim is left on the colorscheme it had rather
	// than started broken.
	plugin, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}
	if themeNvimGeneratedFile(plugin.ID) != "" {
		t.Fatalf("catppuccin-mocha now uses a generated colorscheme, so the plugin half proves nothing")
	}
	pluginHome := t.TempDir()
	installThemeFiles(t, pluginHome, "nvim", "alacritty")
	_, notice, err := applyDotfilesTheme(pluginHome, repoRoot(t), plugin)
	if err != nil {
		t.Fatalf("apply the %s theme: %v", plugin.Name, err)
	}
	pluginInstalled, err := os.ReadFile(themeInstalledPath(artifactByName(t, "nvim"), pluginHome))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pluginInstalled), `colorscheme = "`+plugin.Nvim+`"`) {
		t.Errorf("the switch named the %s colorscheme although the plugin is not installed, so Neovim would start broken", plugin.Nvim)
	}
	if !strings.Contains(notice, "Neovim") || !strings.Contains(notice, plugin.Nvim) {
		t.Errorf("the switch did not say why Neovim was left out: %q", notice)
	}
}

// TestTheCommittedNvimColorschemeResolvesWithoutAPlugin reproduces the default
// half of the warning the user saw: lualine printed "Theme `kanagawa` not
// found" on a machine without the kanagawa plugin, because the repository's
// committed Neovim artifact named a colorscheme only that plugin registers. A
// fresh install copies this file verbatim, so the block it holds is the default
// every new machine starts on. A default that needs a plugin is a default that
// breaks, so the committed line must name a colorscheme this repository ships
// itself: the generated file under dotfiles-nvim/nvim/colors/ travels in the same
// config copy. Putting kanagawa back into the committed block makes this fail.
func TestTheCommittedNvimColorschemeResolvesWithoutAPlugin(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	art := artifactByName(t, "nvim")
	data, err := os.ReadFile(filepath.Join(repoRoot(t), art.Path))
	if err != nil {
		t.Fatal(err)
	}

	// The default a fresh install gets: stepInstallNvim copies this file verbatim.
	id, ok := themeBlockID(string(data), art.Block)
	if !ok {
		t.Fatalf("%s carries no generated block, so this guard proves nothing about the default", art.Path)
	}
	def, ok := themeByID(defs, id)
	if !ok {
		t.Fatalf("%s holds %q, which themes/ does not define", art.Path, id)
	}

	if slices.Contains(themeNvimPluginColorschemes, def.Nvim) {
		t.Errorf("the committed %s names the %s colorscheme, which a plugin registers: on a machine without that plugin Neovim raises E185 and lualine warns \"Theme %s not found\". The default has to be a colorscheme this repository ships.",
			art.Path, def.Nvim, def.Nvim)
	}
	generated := themeNvimGeneratedFile(def.ID)
	if generated == "" {
		t.Fatalf("the default theme %q names %q, which no shipped file provides, so nothing proves it resolves on a fresh machine", def.ID, def.Nvim)
	}
	if _, err := os.Stat(filepath.Join(repoRoot(t), generated)); err != nil {
		t.Errorf("the default colorscheme %q is installed from %s, which this checkout does not have: %v", def.Nvim, generated, err)
	}
	t.Logf("the committed nvim colorscheme is %q, shipped by %s", def.Nvim, generated)
}

// TestTheNvimStatuslineLeavesTheThemeToLazyVim reproduces the other half of the
// warning. LazyVim's own lualine spec sets `options.theme = "auto"`, which
// derives the statusline colours from the active colorscheme: lualine reads
// vim.g.colors_name, loads the named theme when one exists and generates one from
// the highlight groups otherwise. Naming a theme in this configuration overrides
// that and makes the statusline depend on the named theme file being present -
// lualine asserts `Theme <name> not found` and falls back to auto when it is not,
// which is exactly what the user saw with kanagawa, whose lualine theme ships
// inside the kanagawa plugin. Removing the line restores the derivation, which
// cannot drift from the colorscheme; writing it again makes this fail.
func TestTheNvimStatuslineLeavesTheThemeToLazyVim(t *testing.T) {
	path := filepath.Join(repoRoot(t), "dotfiles-nvim/nvim/lua/plugins/ui.lua")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for i, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "opts.options.theme") {
			continue
		}
		t.Errorf("dotfiles-nvim/nvim/lua/plugins/ui.lua:%d sets lualine's theme by hand: %s\nLazyVim already sets options.theme = \"auto\", which follows the applied colorscheme. A fixed name is only found when the plugin that provides that lualine theme is installed; otherwise lualine warns \"Theme <name> not found\" and falls back anyway. Leave the line out so the statusline cannot drift from the colorscheme.",
			i+1, strings.TrimSpace(line))
	}
}

// TestTheNvimAdoptionAnchorMatchesTheLineNotTheTheme pins the other half of
// moving the nvim default off a plugin's colorscheme: the anchor the switch uses
// to adopt a file written before the generated block existed. The line it
// rewrites holds whatever theme was applied, so an anchor naming one theme (the
// old default, kanagawa) adopts that one file and refuses every other. The anchor
// is the assignment, and this guard adopts a line naming each of them.
func TestTheNvimAdoptionAnchorMatchesTheLineNotTheTheme(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	art := artifactByName(t, "nvim")
	if !strings.Contains(art.AdoptStart, "colorscheme") {
		t.Fatalf("the nvim anchor no longer names the colorscheme line: %q", art.AdoptStart)
	}

	for _, name := range []string{"kanagawa", "nocturne", "dotfiles"} {
		home := t.TempDir()
		dst := themeInstalledPath(art, home)
		// A file from before the generated block: the plugin spec, the line the
		// switch rewrites, and nothing else.
		writeThemeFileAt(t, dst, "    {\n      \"LazyVim/LazyVim\",\n      opts = {\n        colorscheme = \""+name+"\",\n      },\n    },\n")

		target, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)
		if _, _, err := applyDotfilesTheme(home, repoRoot(t), target); err != nil {
			t.Errorf("adopting a legacy nvim file naming %q: %v", name, err)
			continue
		}
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "dotfiles-managed-config: nvim") {
			t.Errorf("the legacy nvim file naming %q was not adopted:\n%s", name, got)
		}
		if !strings.Contains(string(got), `colorscheme = "`+defaultThemeID+`"`) {
			t.Errorf("the adopted nvim file naming %q does not carry the %s colorscheme:\n%s", name, defaultThemeID, got)
		}
	}
}

// TestTheGeneratedColorschemeDeclaresTheBackgroundItsBaseImplies pins the one
// derived non-colour in a generated colorscheme: Neovim's own `background` is
// read from the theme's base role, so Catppuccin Latte - the one light theme in
// the library - declares a light background and the others declare a dark one. A
// constant would paint Latte's groups as if the terminal behind them were dark,
// and the guard refuses a constant by requiring both answers.
func TestTheGeneratedColorschemeDeclaresTheBackgroundItsBaseImplies(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	light, dark, checked := 0, 0, 0
	for _, id := range offeredThemeIDs(defs) {
		def, _ := themeByID(defs, id)
		if themeNvimGeneratedFile(id) == "" {
			continue
		}
		rendered, err := renderNvimTheme(def)
		if err != nil {
			t.Errorf("render the %s colorscheme: %v", id, err)
			continue
		}
		want := "dark"
		if themeBaseIsLight(def) {
			want, light = "light", light+1
		} else {
			dark++
		}
		if !strings.Contains(rendered, `vim.o.background = "`+want+`"`) {
			t.Errorf("the %s colorscheme does not declare background = %q for its base %s", id, want, def.Palette["base"])
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no colorscheme was checked, so this guard proves nothing")
	}
	if light == 0 || dark == 0 {
		t.Errorf("the background rule produced %d light and %d dark colorscheme(s): a constant would pass this guard", light, dark)
	}
	t.Logf("%d generated colorscheme(s) checked: %d light, %d dark", checked, light, dark)
}

// TestTheBatThemesTheSwitchOffersAreGeneratedAtInstallTime is the bat half of the
// completion rule. bat selects a theme by a name its themes directory has to
// hold, and only two .tmTheme files are under version control, so a theme that
// names one of the others would leave bat on "Catppuccin Mocha" unless the
// installer writes it. This drives the same helper the shell step calls and
// checks the file, its name, and the name inside it.
func TestTheBatThemesTheSwitchOffersAreGeneratedAtInstallTime(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	dir := t.TempDir()

	generated, err := generateBatThemesFromDefinitions(repoRoot(t), dir)
	if err != nil {
		t.Fatalf("generate the bat themes: %v", err)
	}

	named, shipped := 0, 0
	for _, id := range offeredThemeIDs(defs) {
		def, _ := themeByID(defs, id)
		if def.Bat == "" || def.BatFile == "" {
			t.Errorf("the offered theme %q names no bat theme, so bat would stay on the old one", id)
			continue
		}
		named++
		got, err := os.ReadFile(filepath.Join(dir, def.BatFile))
		if err != nil {
			t.Errorf("theme %q names the bat theme %q but the installer generated no %s: %v", id, def.Bat, def.BatFile, err)
			continue
		}
		want, err := renderBatTheme(def)
		if err != nil {
			t.Errorf("render the %s bat theme: %v", id, err)
			continue
		}
		if string(got) != want {
			t.Errorf("the generated %s is not what themes/%s.toml produces", def.BatFile, id)
		}
		if !strings.Contains(string(got), "<string>"+def.Bat+"</string>") {
			t.Errorf("the generated %s does not carry the name %q the switch exports as BAT_THEME", def.BatFile, def.Bat)
		}
		if _, err := os.Stat(filepath.Join(repoRoot(t), "dotfiles-bat", "themes", def.BatFile)); err == nil {
			shipped++
		}
	}
	if named == 0 {
		t.Fatal("no theme names a bat theme, so this guard proves nothing")
	}
	if generated != named {
		t.Errorf("the installer generated %d bat theme(s) for the %d theme(s) that name one", generated, named)
	}
	if shipped == named {
		t.Fatal("every bat theme is also under version control, so this guard proves nothing about the generated ones")
	}
	t.Logf("%d bat theme(s) generated from the definitions; %d of them are also under version control", generated, shipped)
}

// TestGeneratedBatThemesAreValidXML pins that every .tmTheme the generator emits
// is well-formed XML, not merely a file bat tolerates. bat's own parser accepts a
// comment whose body contains "--", and XML forbids it, so a text search for the
// sequence is not enough - it would have to know which lines are comments. The
// guard hands each rendered file to encoding/xml, whose strict comment rule
// (`invalid sequence "--" not allowed in comments`) refuses it. Every theme that
// names a bat theme is checked, so the four .tmTheme files the installer
// generates at run time rather than shipping are covered too.
func TestGeneratedBatThemesAreValidXML(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	checked := 0
	for _, def := range defs {
		if def.Bat == "" || def.BatFile == "" {
			continue
		}
		rendered, err := renderBatTheme(def)
		if err != nil {
			t.Errorf("render the %s bat theme: %v", def.ID, err)
			continue
		}
		// A plist's root is a dict; decoding into an empty struct still consumes the
		// whole document, so a syntax error anywhere - including "--" inside the
		// header comment - is reported rather than skipped.
		var document struct {
			XMLName xml.Name
		}
		if err := xml.Unmarshal([]byte(rendered), &document); err != nil {
			t.Errorf("the generated %s is not valid XML: %v", def.BatFile, err)
			continue
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no bat theme was checked, so this guard proves nothing")
	}
	t.Logf("%d generated bat theme(s) parse as XML", checked)
}

// TestTheBatSelectionNamesTheFileBatRegisters pins the name the switch exports as
// BAT_THEME. bat registers a custom .tmTheme under its **file** name, not under
// the `<key>name</key>` the file carries: measured with bat 0.26.1, a fresh cache
// built from `dotfiles-bat/themes/` lists `catppuccin-mocha` beside its own
// bundled "Catppuccin Mocha", `BAT_THEME=catppuccin-mocha` paints the repository's
// blue keyword and `BAT_THEME="Catppuccin Mocha"` paints the bundled theme's
// mauve. The selection is therefore the file's stem, and this guard refuses a
// definition whose `[bat] name` would be exported in its place - the defect that
// made Catppuccin Mocha paint bat's own bundled theme rather than the file this
// repository ships - as well as two themes whose files share a name, which bat
// would collapse into one entry.
func TestTheBatSelectionNamesTheFileBatRegisters(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	selected := map[string]string{}
	differing := []string{}
	for _, id := range offeredThemeIDs(defs) {
		def, _ := themeByID(defs, id)
		name, err := batSelectionName(def)
		if err != nil {
			t.Errorf("theme %q cannot be selected in bat: %v", id, err)
			continue
		}
		if want := strings.TrimSuffix(filepath.Base(def.BatFile), ".tmTheme"); name != want {
			t.Errorf("theme %q exports BAT_THEME=%q, but bat registers %s as %q", id, name, def.BatFile, want)
		}
		if other, dup := selected[name]; dup {
			t.Errorf("themes %q and %q both export BAT_THEME=%q: bat would keep one of the two files", other, id, name)
		}
		selected[name] = id
		if def.Bat != name {
			differing = append(differing, id+": [bat] name "+def.Bat+" vs file "+def.BatFile)
		}

		block, err := renderBatSelection(def)
		if err != nil {
			t.Errorf("render the bat selection for %q: %v", id, err)
			continue
		}
		if !strings.Contains(block, "export BAT_THEME=\""+name+"\"") {
			t.Errorf("the bat selection block for %q does not export BAT_THEME=%q", id, name)
		}
		if def.BatFile != "" && !strings.Contains(block, "bat/themes/"+def.BatFile) {
			t.Errorf("the bat selection block for %q does not check for %s", id, def.BatFile)
		}
	}
	if len(selected) == 0 {
		t.Fatal("no offered theme names a bat theme, so this guard proves nothing")
	}
	t.Logf("%d theme(s) select their bat theme by the file's name; %d of them carry a [bat] name that differs from it: %v",
		len(selected), len(differing), differing)
}

// TestShippedThemeBlocksMatchTheirDefinition is the drift guard: every value a
// definition claims must still be present in the shipped block it came from. A
// hand edit that changes a digit in one of the six copies fails here.
func TestShippedThemeBlocksMatchTheirDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	checked := 0
	for id, blocks := range themeSourceBlocks {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Errorf("themeSourceBlocks names %q, which no definition defines", id)
			continue
		}
		for _, block := range blocks {
			data, err := os.ReadFile(filepath.Join(repoRoot(t), block.Path))
			if err != nil {
				t.Errorf("read %s: %v", block.Path, err)
				continue
			}
			for _, role := range block.Roles {
				value := def.Palette[role]
				if value == "" {
					t.Errorf("theme %q has no value for %q, so %s cannot be checked", id, role, block.Path)
					continue
				}
				if !themeValueInFile(value, string(data)) {
					t.Errorf("%s (%s) no longer carries %s = %s from themes/%s.toml; regenerate the block from the definition instead of editing it by hand",
						block.Path, block.Tool, role, value, id)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no shipped block was checked, so this guard proves nothing")
	}
	t.Logf("checked %d shipped role values against %d theme definitions", checked, len(defs))
}

// TestGeneratedThemeArtifactsMatchTheirDefinition is the generation guard: each
// shipped file's block must be byte-for-byte what the generator produces from
// themes/dotfiles.toml. Run with -update-theme-artifacts to regenerate; running
// without it is what fails when somebody edits a generated block by hand.
func TestGeneratedThemeArtifactsMatchTheirDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	if _, ok := themeByID(defs, defaultThemeID); !ok {
		t.Fatalf("the committed files hold %q, which themes/ does not define", defaultThemeID)
	}

	checked := 0
	for _, art := range themeActiveArtifacts {
		// A file whose committed value is another theme is checked against the theme
		// it actually holds.
		themeID := art.DefaultTheme
		if themeID == "" {
			themeID = defaultThemeID
		}
		def, ok := themeByID(defs, themeID)
		if !ok {
			t.Errorf("%s holds %q, which themes/ does not define", art.Path, themeID)
			continue
		}
		path := filepath.Join(repoRoot(t), art.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", art.Path, err)
			continue
		}
		block, err := themeArtifactBlock(art, def)
		if err != nil {
			t.Errorf("render the %s block: %v", art.Tool, err)
			continue
		}
		updated, adopted := replaceThemeBlock(string(data), block, art.Block)
		if !adopted {
			updated, adopted = adoptThemeBlock(string(data), art, block)
			if !adopted {
				t.Errorf("%s carries neither generated block markers nor a DOTFILES THEME block to adopt", art.Path)
				continue
			}
			if !*updateThemeArtifacts {
				t.Errorf("%s has not been adopted yet: run the guard with -update-theme-artifacts once to wrap its block in markers", art.Path)
				continue
			}
		}
		checked++

		if *updateThemeArtifacts {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				t.Errorf("write %s: %v", art.Path, err)
			}
			continue
		}
		if updated != string(data) {
			t.Errorf("%s no longer matches themes/%s.toml: regenerate it with "+
				"go test ./internal/tui -run TestGeneratedThemeArtifactsMatchTheirDefinition -update-theme-artifacts",
				art.Path, themeID)
		}
	}
	if checked == 0 {
		t.Fatal("no theme artifact was checked, so this guard proves nothing")
	}
	if *updateThemeArtifacts {
		t.Logf("regenerated %d theme artifacts", checked)
	} else {
		t.Logf("%d shipped theme artifacts match their definitions byte-for-byte", checked)
	}
}

// TestThemeGeneratorUsesTheDefinition covers the other half of the generation
// guard: the block a tool gets is the definition's colours and not the default
// theme's. Without it, a generator that ignored its argument would pass the
// byte-for-byte guard above.
func TestThemeGeneratorUsesTheDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	var rendered int
	for _, art := range themeActiveArtifacts {
		dotfiles, ok := themeByID(defs, "dotfiles")
		if !ok {
			t.Fatal("the dotfiles definition is missing")
		}
		catppuccin, ok := themeByID(defs, "catppuccin-mocha")
		if !ok {
			t.Fatal("the catppuccin-mocha definition is missing")
		}

		one, err := themeArtifactBlock(art, dotfiles)
		if err != nil {
			// An artifact a theme cannot paint (Neovim under dotfiles, which names
			// no colorscheme) is reported by the coverage guard, not here.
			continue
		}
		two, err := themeArtifactBlock(art, catppuccin)
		if err != nil {
			continue
		}
		if one == two {
			t.Errorf("the %s block is the same for dotfiles and catppuccin-mocha, so the generator ignores the definition", art.Tool)
		}
		// The block must carry at least one of the theme's own colours, unless it is
		// the palette-selection line, which names the palette and holds no colour.
		if themeHexRE.MatchString(two) {
			carries := false
			for _, value := range catppuccin.Palette {
				if strings.Contains(strings.ToLower(two), strings.ToLower(value)) ||
					strings.Contains(strings.ToLower(two), strings.ToLower(strings.TrimPrefix(value, "#"))) {
					carries = true
					break
				}
			}
			if !carries {
				t.Errorf("the %s block for catppuccin-mocha carries none of its colours", art.Tool)
			}
		}
		rendered++
	}
	if rendered == 0 {
		t.Fatal("no theme artifact was rendered, so this guard proves nothing")
	}
}

// TestThemeGeneratorRefusesAMissingRole covers the honest-degradation edge: a
// definition that misses a role cannot be rendered, rather than emitting an
// empty colour into a tool's config.
func TestThemeGeneratorRefusesAMissingRole(t *testing.T) {
	incomplete := themeDefinition{ID: "half", Palette: map[string]string{"base": "#000000"}, Prompt: map[string]string{}}
	for _, art := range themeActiveArtifacts {
		if art.Block == "palette" {
			// The palette-selection line names the palette; it needs no colour.
			continue
		}
		if _, err := themeArtifactBlock(art, incomplete); err == nil {
			t.Errorf("the %s renderer produced a block from a definition with one role", art.Tool)
		}
	}
	if _, err := renderStarshipPaletteTable(incomplete); err == nil {
		t.Error("the Starship palette table rendered from a definition with one role")
	}
}

// themeHexTokenRE matches a whole colour token a generated block or file may
// carry: a #rrggbb value or a bare six-digit hex, which is how the fish config
// writes one. The word boundaries matter: a six-letter word spelt with hex digits
// - "bedded", inside bat's `embedded` scope name - is not a colour, and reading
// it as one would fail a generated file for a colour it never emitted. It is only
// used to scan generated output for invented colours.
var themeHexTokenRE = regexp.MustCompile(`(?i)\b[0-9a-f]{6}\b`)

// TestGeneratedThemeBlocksInventNoColour covers the provenance rule at the
// renderer: every colour a generated block emits must be one the definition
// already holds, either as a canonical palette role or as one of its fish roles.
// A renderer that filled a role by eye fails here, which is the guard the tmux
// and fish blocks could otherwise pass while carrying a colour of their own.
func TestGeneratedThemeBlocksInventNoColour(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	owned := func(def themeDefinition, token string) bool {
		value := strings.ToLower(strings.TrimPrefix(token, "#"))
		for _, hex := range def.Palette {
			if strings.TrimPrefix(strings.ToLower(hex), "#") == value {
				return true
			}
		}
		for _, hex := range def.Fish {
			if strings.TrimPrefix(strings.ToLower(hex), "#") == value {
				return true
			}
		}
		for _, hex := range def.Prompt {
			if strings.TrimPrefix(strings.ToLower(hex), "#") == value {
				return true
			}
		}
		return false
	}

	tools := map[string]bool{}
	checked := 0
	for _, id := range offeredThemeIDs(defs) {
		def, _ := themeByID(defs, id)
		for _, art := range themeActiveArtifacts {
			block, err := themeArtifactBlock(art, def)
			if err != nil {
				continue
			}
			tools[art.Tool] = true
			for _, line := range strings.Split(block, "\n") {
				// A comment may cite a historical colour (the p10k block records the
				// Kanagawa values it used to hold); only emitted lines are checked.
				if art.Comment != "" && strings.HasPrefix(strings.TrimSpace(line), art.Comment) {
					continue
				}
				for _, token := range themeHexTokenRE.FindAllString(line, -1) {
					if !owned(def, token) {
						t.Errorf("the %s block for %q emits %s, which themes/%s.toml does not hold: a generated block may not invent a colour",
							art.Tool, id, token, id)
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no generated colour was checked, so this guard proves nothing")
	}
	for _, tool := range []string{"fish", "tmux"} {
		if !tools[tool] {
			t.Errorf("no %s block was rendered for any offered theme, so the guard does not cover it", tool)
		}
	}

	// The per-theme files (fish's theme files, bat's .tmTheme files and the
	// generated Neovim colorschemes) are whole files, so their comment lines are
	// skipped by the shape a comment has in them rather than by a tool's marker:
	// the rule is the same, an emitted colour has to be one the definition holds.
	perTheme := 0
	for _, file := range themeThemeFiles {
		def, ok := themeByID(defs, file.Theme)
		if !ok {
			t.Errorf("themeThemeFiles names %q, which no definition defines", file.Theme)
			continue
		}
		rendered, err := file.Render(def)
		if err != nil {
			t.Errorf("render the %s file for %q: %v", file.Tool, file.Theme, err)
			continue
		}
		tools[file.Tool] = true
		for _, line := range strings.Split(rendered, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "<!--") {
				continue
			}
			for _, token := range themeHexTokenRE.FindAllString(line, -1) {
				if !owned(def, token) {
					t.Errorf("the generated %s file %s for %q emits %s, which themes/%s.toml does not hold: a generated file may not invent a colour",
						file.Tool, file.Path, file.Theme, token, file.Theme)
				}
				checked++
				perTheme++
			}
		}
	}
	if perTheme == 0 {
		t.Fatal("no per-theme file emitted a colour, so this guard proves nothing about them")
	}
	for _, tool := range []string{"bat", "nvim"} {
		if !tools[tool] {
			t.Errorf("no per-theme %s file was checked, so the guard does not cover it", tool)
		}
	}
	t.Logf("checked %d generated colour tokens across %d tool(s), %d of them in per-theme files", checked, len(tools), perTheme)
}

// TestTheFishDerivationMatchesTheDotfilesTable pins the equivalence the
// derivation claims: the [fish] table themes/dotfiles.toml records is the
// mechanical mapping, so deriving it (as a theme with no [fish] table, such as
// catppuccin-mocha, must) produces the same values. A change to either side that
// breaks the equivalence fails here instead of silently giving two themes two
// different fish palettes.
func TestTheFishDerivationMatchesTheDotfilesTable(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, "dotfiles")
	if !ok {
		t.Fatal("the dotfiles definition is missing")
	}
	if len(def.Fish) == 0 {
		t.Fatal("the dotfiles definition carries no [fish] table, so this guard proves nothing")
	}

	checked := 0
	for role, paletteRole := range themeFishDerivation {
		want := strings.TrimPrefix(def.Palette[paletteRole], "#")
		if want == "" {
			t.Errorf("the derivation names palette role %q, which themes/dotfiles.toml does not define", paletteRole)
			continue
		}
		if got := def.Fish[role]; got != want {
			t.Errorf("fish role %q is %q in themes/dotfiles.toml but %q derived from palette role %q", role, got, want, paletteRole)
		}
		checked++
	}
	if checked != len(themeFishRoles) {
		t.Errorf("the derivation covers %d fish role(s), want %d", checked, len(themeFishRoles))
	}
	t.Logf("the fish derivation matches themes/dotfiles.toml for all %d fish roles", checked)
}

// TestTmuxThemeBlockLoadsAfterPlugins pins the ordering decision behind the tmux
// switch. tmux runs `run-shell` synchronously: measurement shows the server does
// not finish reading tmux.conf until the command returns, so TPM has already
// sourced the kanagawa plugin's own styles by the time the generated block is
// read. The block must therefore sit after that `run` line; before it, the
// plugin would win and the applied palette would not be seen.
func TestTmuxThemeBlockLoadsAfterPlugins(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "dotfiles-tmux/tmux.conf"))
	if err != nil {
		t.Fatal(err)
	}

	runLine, blockLine := -1, -1
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "tpm/tpm") {
			runLine = i
		}
		if strings.Contains(line, themeBeginTag("")) {
			blockLine = i
		}
	}
	if runLine < 0 {
		t.Fatal("tmux.conf no longer runs TPM, so this guard proves nothing about plugin ordering")
	}
	if blockLine < 0 {
		t.Fatal("tmux.conf carries no generated theme block to order")
	}
	if blockLine < runLine {
		t.Errorf("the tmux theme block is at line %d, before the plugin run at line %d: the plugin's async styles would overwrite it",
			blockLine+1, runLine+1)
	}
	t.Logf("tmux theme block at line %d, plugin run at line %d", blockLine+1, runLine+1)
}

// tempThemeRepo copies themes/*.toml into a temporary root, so a switch test
// runs against the real definitions without reading the working tree.
func tempThemeRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "themes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), "themes", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "themes", entry.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// themeRootWith writes one minimal definition under root/themes and returns the
// root. A resolution test tells which candidate won by the id it reads back;
// the definition is partial on purpose, because resolution is about finding a
// checkout, not about offering a theme.
func themeRootWith(t *testing.T, id string) string {
	t.Helper()

	return writeThemeAt(t, t.TempDir(), id)
}

// themeIDsIn loads root's definitions and returns their ids, so a resolution
// test proves the winning candidate's files were actually read.
func themeIDsIn(t *testing.T, root string) []string {
	t.Helper()

	defs, err := loadThemeDefinitions(root)
	if err != nil {
		t.Fatalf("load the definitions from %s: %v", root, err)
	}
	ids := make([]string, 0, len(defs))
	for _, def := range defs {
		ids = append(ids, def.ID)
	}
	return ids
}

// TestThemeResolutionTriesDotfilesDirFirst pins candidate 1: $DOTFILES_DIR is
// named by the user, so it is answered before the clone or the working tree.
func TestThemeResolutionTriesDotfilesDirFirst(t *testing.T) {
	envRoot := themeRootWith(t, "from-env")
	cloneRoot := themeRootWith(t, "from-clone")
	cwdRoot := themeRootWith(t, "from-cwd")
	t.Setenv("DOTFILES_DIR", envRoot)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(cwdRoot)

	got, err := resolveThemeDefinitionsDir(cloneRoot)
	if err != nil {
		t.Fatalf("resolve with three candidates present: %v", err)
	}
	if got != envRoot {
		t.Fatalf("resolved %q, want $DOTFILES_DIR first (%q)", got, envRoot)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-env" {
		t.Errorf("read definitions %v, want the $DOTFILES_DIR theme", ids)
	}
}

// TestThemeResolutionUsesTheCloneBeforeTheWorkdir pins candidate 2: when no
// directory is named, the clone this run made wins over the working tree.
func TestThemeResolutionUsesTheCloneBeforeTheWorkdir(t *testing.T) {
	cloneRoot := themeRootWith(t, "from-clone")
	cwdRoot := themeRootWith(t, "from-cwd")
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(cwdRoot)

	got, err := resolveThemeDefinitionsDir(cloneRoot)
	if err != nil {
		t.Fatalf("resolve with a clone and a working tree: %v", err)
	}
	if got != cloneRoot {
		t.Fatalf("resolved %q, want the clone first (%q)", got, cloneRoot)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-clone" {
		t.Errorf("read definitions %v, want the clone's theme", ids)
	}
}

// TestThemeResolutionUsesTheWorkdirAndItsParents pins candidate 3: with no
// clone, the working directory and its ancestors are walked from the nearest
// theme-bearing directory upward, because launching the installer from inside
// the checkout is the normal case.
func TestThemeResolutionUsesTheWorkdirAndItsParents(t *testing.T) {
	root := themeRootWith(t, "from-workdir")
	deep := filepath.Join(root, "installer", "internal", "tui")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(deep)

	got, err := resolveThemeDefinitionsDir("")
	if err != nil {
		t.Fatalf("resolve from inside a checkout: %v", err)
	}
	if got != root {
		t.Fatalf("resolved %q, want the workdir's ancestor (%q)", got, root)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-workdir" {
		t.Errorf("read definitions %v, want the workdir's theme", ids)
	}
}

// TestThemeResolutionFallsBackToHomeDotfiles pins candidate 4: with no clone
// and no checkout above the working directory, ~/dotfiles is tried.
func TestThemeResolutionFallsBackToHomeDotfiles(t *testing.T) {
	home := t.TempDir()
	homeRoot := writeThemeAt(t, filepath.Join(home, "dotfiles"), "from-home-dotfiles")
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	got, err := resolveThemeDefinitionsDir("")
	if err != nil {
		t.Fatalf("resolve from home: %v", err)
	}
	if got != homeRoot {
		t.Fatalf("resolved %q, want ~/dotfiles (%q)", got, homeRoot)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-home-dotfiles" {
		t.Errorf("read definitions %v, want ~/dotfiles theme", ids)
	}
}

// TestThemeResolutionFallsBackToHiddenHomeDotfiles pins candidate 5: ~/.dotfiles
// is tried after ~/dotfiles when the visible one is absent.
func TestThemeResolutionFallsBackToHiddenHomeDotfiles(t *testing.T) {
	home := t.TempDir()
	homeRoot := writeThemeAt(t, filepath.Join(home, ".dotfiles"), "from-hidden-dotfiles")
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	got, err := resolveThemeDefinitionsDir("")
	if err != nil {
		t.Fatalf("resolve from hidden home: %v", err)
	}
	if got != homeRoot {
		t.Fatalf("resolved %q, want ~/.dotfiles (%q)", got, homeRoot)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-hidden-dotfiles" {
		t.Errorf("read definitions %v, want ~/.dotfiles theme", ids)
	}
}

// TestThemeResolutionFallsBackToTheInstalledDefinitions is the user's case: the
// installer was run once, so it left a copy of the definitions in the per-user
// data directory, and the program is now launched from a working directory that
// is not a checkout, with no clone and nothing under ~/dotfiles. The copy is the
// last candidate, and it is what makes the theme switch offered from anywhere.
func TestThemeResolutionFallsBackToTheInstalledDefinitions(t *testing.T) {
	dataHome := t.TempDir()
	installedRoot := writeThemeAt(t, filepath.Join(dataHome, stateAppDir), "from-installed-copy")
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	got, err := resolveThemeDefinitionsDir("")
	if err != nil {
		t.Fatalf("resolve from a foreign working directory with only an installed copy: %v", err)
	}
	if got != installedRoot {
		t.Fatalf("resolved %q, want the installed copy (%q)", got, installedRoot)
	}
	if ids := themeIDsIn(t, got); len(ids) != 1 || ids[0] != "from-installed-copy" {
		t.Errorf("read definitions %v, want the installed copy's theme", ids)
	}
}

// TestThemeResolutionReportsWhenNothingIsFound pins the honest state: no
// candidate holds themes/*.toml, so resolution fails and names what it looked
// for -- including the per-user directory the installer copies the definitions
// into -- rather than returning an empty directory.
func TestThemeResolutionReportsWhenNothingIsFound(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	got, err := resolveThemeDefinitionsDir("")
	if err == nil {
		t.Fatalf("resolved %q with no candidate present, want a failure", got)
	}
	if !strings.Contains(err.Error(), "theme definitions") {
		t.Errorf("the failure does not name what it looked for: %v", err)
	}
	// The search has to name where it looked: the installed copy is the candidate
	// that makes the utility work from anywhere, so an unmentioned hole in it is
	// the failure this assertion exists to prevent.
	if want := filepath.Join(dataHome, stateAppDir); !strings.Contains(err.Error(), want) {
		t.Errorf("the failure does not name the installed-copy directory %q: %v", want, err)
	}
}

// TestInstallThemeDefinitionsCopiesOnlyMissingOrDifferent pins the copy rules the
// installed definitions follow: a shipped definition is written when it is
// missing or differs, an identical one is left as it is, and a file the
// repository does not ship is never touched.
func TestInstallThemeDefinitionsCopiesOnlyMissingOrDifferent(t *testing.T) {
	repo := themeRootWith(t, "shipped")
	// A second definition, so the missing-file rule is covered beside the
	// differing-file one.
	writeThemeAt(t, repo, "second")
	shipped, err := os.ReadFile(filepath.Join(repo, themesDirName, "shipped.toml"))
	if err != nil {
		t.Fatal(err)
	}

	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	dest := filepath.Join(dataHome, stateAppDir, themesDirName)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(dest, "mine.toml")
	if err := os.WriteFile(userFile, []byte("[theme]\nid = \"mine\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dest, "shipped.toml")
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	copied, current, gotDest, err := installThemeDefinitions(repo)
	if err != nil {
		t.Fatalf("install the definitions: %v", err)
	}
	if copied != 2 || current != 0 {
		t.Errorf("the first copy wrote %d and kept %d, want 2 written and 0 current", copied, current)
	}
	if gotDest != dest {
		t.Errorf("wrote to %q, want %q", gotDest, dest)
	}
	if got, _ := os.ReadFile(stale); !bytes.Equal(got, shipped) {
		t.Errorf("the differing shipped definition was not refreshed: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "second.toml")); err != nil || len(got) == 0 {
		t.Errorf("the missing shipped definition was not written: %q, %v", got, err)
	}
	if got, _ := os.ReadFile(userFile); string(got) != "[theme]\nid = \"mine\"\n" {
		t.Errorf("a file the repository does not ship was touched: %q", got)
	}

	// A second run with nothing new writes nothing and says what is current.
	copied, current, _, err = installThemeDefinitions(repo)
	if err != nil {
		t.Fatalf("install the definitions a second time: %v", err)
	}
	if copied != 0 || current != 2 {
		t.Errorf("the second copy wrote %d and kept %d, want 0 written and 2 current", copied, current)
	}
}

// writeThemeAt writes one minimal definition under root/themes and returns root,
// for the home fallbacks where the root's own name is part of the assertion.
func writeThemeAt(t *testing.T, root, id string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, themesDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[theme]\nid = \"" + id + "\"\npartial = true\npartial_reason = \"fixture\"\n\n[palette]\nbase = \"#000000\"\n"
	if err := os.WriteFile(filepath.Join(root, themesDirName, id+".toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// artifactByName finds an active artifact by tool, failing the test when it is
// gone so a renamed tool cannot silently empty a switch test.
func artifactByName(t *testing.T, tool string) themeArtifact {
	t.Helper()
	for _, art := range themeActiveArtifacts {
		if art.Tool == tool {
			return art
		}
	}
	t.Fatalf("no theme artifact for %q", tool)
	return themeArtifact{}
}

// installThemeFiles copies two generated files into a temporary home and returns
// their contents before the switch, keyed by path.
func installThemeFiles(t *testing.T, home string, tools ...string) map[string][]byte {
	t.Helper()

	before := map[string][]byte{}
	for _, tool := range tools {
		art := artifactByName(t, tool)
		data, err := os.ReadFile(filepath.Join(repoRoot(t), art.Path))
		if err != nil {
			t.Fatal(err)
		}
		dst := themeInstalledPath(art, home)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
		before[dst] = data
	}
	return before
}

// TestDotfilesThemeSwitchIsReversible covers the core contract: the switch
// writes the theme into every owned file, records what was there, and puts the
// exact bytes back.
func TestDotfilesThemeSwitchIsReversible(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	before := installThemeFiles(t, home, "alacritty", "kitty", "fish", "tmux")

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme: %v", err)
	}
	if rec == nil {
		t.Fatal("the switch returned no record, so the change is not reversible")
	}
	if strings.Contains(notice, "DRY RUN") {
		t.Fatalf("a real run reported a dry run: %q", notice)
	}
	if len(rec.Files) != len(before) {
		t.Errorf("the record holds %d file(s), the switch wrote %d", len(rec.Files), len(before))
	}

	// Every file the switch rewrote must carry at least one of the new theme's own
	// colours. Checking for the base colour alone would fail on the fish block,
	// which derives its roles from other palette entries and holds no base value.
	values := map[string]bool{}
	add := func(hex string) {
		if hex == "" || hex == "none" {
			return
		}
		values[strings.ToLower(strings.TrimPrefix(hex, "#"))] = true
	}
	for _, hex := range target.Palette {
		add(hex)
	}
	for _, hex := range target.Fish {
		add(hex)
	}
	for _, hex := range target.Prompt {
		add(hex)
	}
	carriesTheme := func(content []byte) bool {
		lower := strings.ToLower(string(content))
		for value := range values {
			if strings.Contains(lower, value) {
				return true
			}
		}
		return false
	}

	for path, was := range before {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(got, was) {
			t.Errorf("%s was not changed by the switch", path)
		}
		if !carriesTheme(got) {
			t.Errorf("%s carries none of the catppuccin-mocha colours", path)
		}
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the theme: %v", err)
	}
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s was not restored byte-for-byte", path)
		}
	}
	if rec := readDotfilesThemeRecord(); rec != nil {
		t.Errorf("the record was not cleared after the undo: %+v", rec)
	}
}

// TestDotfilesThemeSwitchSkipsOnDryRun covers the gate PR #134 established: a
// dry run changes nothing, on disk or in the record.
func TestDotfilesThemeSwitchSkipsOnDryRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "1")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	before := installThemeFiles(t, home, "alacritty", "kitty", "fish", "tmux")

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("a dry run returned an error: %v", err)
	}
	if rec != nil {
		t.Errorf("a dry run recorded a change: %+v", rec)
	}
	if !strings.Contains(notice, "DRY RUN") {
		t.Errorf("a dry run's notice does not say so: %q", notice)
	}
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("a dry run wrote %s", path)
		}
	}
	if rec := readDotfilesThemeRecord(); rec != nil {
		t.Errorf("a dry run wrote a record: %+v", rec)
	}
}

// TestDotfilesThemeSwitchRefusesAnUnownedFile covers the preserve-user-configs
// rule: a file with no ownership marker is left exactly as the user wrote it.
func TestDotfilesThemeSwitchRefusesAnUnownedFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	art := artifactByName(t, "alacritty")
	dst := themeInstalledPath(art, home)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	user := []byte("# my own alacritty config\nbackground = \"#000000\"\n")
	if err := os.WriteFile(dst, user, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := applyDotfilesTheme(home, repoRoot(t), target); err == nil {
		t.Fatal("the switch rewrote a file that carries no ownership marker")
	} else if !strings.Contains(err.Error(), "not owned") {
		t.Errorf("the refusal does not name the ownership rule: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, user) {
		t.Error("the unowned file was changed")
	}
}

// installThemeFileWithoutOwnershipMarker writes the file the repository ships
// for tool into home with the ownership-marker line removed, which is the state
// an install from before the marker leaves behind. It returns the bytes written.
func installThemeFileWithoutOwnershipMarker(t *testing.T, home, tool string) []byte {
	t.Helper()

	art := artifactByName(t, tool)
	shipped, err := os.ReadFile(filepath.Join(repoRoot(t), art.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shipped), themeOwnershipMarker) {
		t.Fatalf("the repository's %s file carries no ownership marker, so this fixture proves nothing", tool)
	}

	var kept []string
	for _, line := range strings.Split(string(shipped), "\n") {
		if strings.Contains(line, themeOwnershipMarker) {
			continue
		}
		kept = append(kept, line)
	}
	content := []byte(strings.Join(kept, "\n"))
	if strings.Contains(string(content), themeOwnershipMarker) {
		t.Fatalf("stripping the marker from the repository's %s file left one behind", tool)
	}

	dst := themeInstalledPath(art, home)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return content
}

// TestThemeAdoptionAcceptsAFileThatMatchesTheRepository covers case (a): an
// installed file with no ownership marker whose content is what the repository
// ships (the repository version without the marker) is adopted, and the theme can
// then be applied to it. An install from before the marker leaves exactly this
// file, and refusing it is what left the user unable to change the theme.
func TestThemeAdoptionAcceptsAFileThatMatchesTheRepository(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	installThemeFileWithoutOwnershipMarker(t, home, "herdr")
	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to an unmarked file whose content matches the repository: %v", err)
	}
	if rec == nil {
		t.Fatal("the adopted file was not recorded, so the change is not reversible")
	}
	if !strings.Contains(notice, "Adopted") {
		t.Errorf("the notice does not say a file was adopted: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeOwnershipMarker) {
		t.Error("the adopted file did not get the ownership marker")
	}
	if !strings.Contains(string(got), target.Palette["selection"]) {
		t.Errorf("the adopted file does not carry the %s selection colour %s", target.Name, target.Palette["selection"])
	}
}

// TestThemeAdoptionUndoRemovesTheMarker covers case (c): undo of an adoption
// leaves the file byte-for-byte as it was before the adoption - that is, with no
// ownership marker. That byte-for-byte return is what makes adoption honest
// rather than a matter of trusting the installer.
func TestThemeAdoptionUndoRemovesTheMarker(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	before := installThemeFileWithoutOwnershipMarker(t, home, "herdr")
	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)

	rec, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to an adoptable file: %v", err)
	}
	if rec == nil {
		t.Fatal("the adoption returned no record, so it cannot be undone")
	}
	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the adopted file: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, before) {
		t.Errorf("undo did not restore the file byte-for-byte\n got: %q\nwant: %q", got, before)
	}
	if strings.Contains(string(got), themeOwnershipMarker) {
		t.Error("undo left the ownership marker behind on a file that had none before")
	}
}

// TestThemeAdoptionUndoRestoresAFileWithTwoBlocks covers the file two artifacts
// share: Starship has two generated blocks (its palette line and its palette
// table) in one file, and .zshrc carries the zsh block and the bat block. The
// first artifact records the file's original bytes; the second must not overwrite
// that record with the state the first one left, or undo cannot reach the file's
// original bytes and the adoption is not reversible.
func TestThemeAdoptionUndoRestoresAFileWithTwoBlocks(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	before := installThemeFileWithoutOwnershipMarker(t, home, "starship")
	art := artifactByName(t, "starship")
	dst := themeInstalledPath(art, home)

	rec, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to an adoptable file with two blocks: %v", err)
	}
	if rec == nil {
		t.Fatal("the adoption returned no record, so it cannot be undone")
	}
	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the adopted file: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, before) {
		t.Errorf("undo did not restore the two-block file byte-for-byte, so the second block overwrote the recorded original")
	}
}

// TestThemeAdoptionDoesNotTouchAnOwnedFile covers case (d): a file that already
// carries the ownership marker is applied to as before, and adoption does not run
// a second time or write a duplicate marker line.
func TestThemeAdoptionDoesNotTouchAnOwnedFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	installThemeFiles(t, home, "herdr")
	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)

	_, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to an already-owned file: %v", err)
	}
	if strings.Contains(notice, "Adopted") {
		t.Errorf("an already-owned file was reported as adopted: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), themeOwnershipMarker); n != 1 {
		t.Errorf("the ownership marker appears %d times after the apply, want exactly 1", n)
	}
}

// installThemeFileOwnedWithoutBlock writes a managed file that carries the
// ownership marker but no generated block and no region anchors, which is the one
// shape neither replaceThemeBlock nor adoptThemeBlock can rewrite: a hand-removed
// generated block, or an install window that wrote the marker without a block. It
// returns the bytes written and the path, so a test can prove the file is left
// untouched.
func installThemeFileOwnedWithoutBlock(t *testing.T, home, tool string) ([]byte, string) {
	t.Helper()

	art := artifactByName(t, tool)
	content := []byte(themeOwnershipMarkerLine(art) + "\n# " + art.Tool + " config from an install that predates the generated block\n")
	dst := themeInstalledPath(art, home)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return content, dst
}

// TestThemeSwitchReportsAnOwnedFileWithNoGeneratedBlock covers the residual gap
// recorded in odd/tasks/unified-themes.md: a file that carries the ownership
// marker but neither a generated block nor the region anchors is ours, yet neither
// the block replacement nor region adoption can touch it, and it used to be
// skipped in silence. The switch must apply the rest of the files and name the one
// it could not update, with the remedy, instead of reporting a change it did not
// make.
func TestThemeSwitchReportsAnOwnedFileWithNoGeneratedBlock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	staleBefore, stalePath := installThemeFileOwnedWithoutBlock(t, home, "alacritty")
	goodBefore := installThemeFiles(t, home, "herdr")
	goodPath := themeInstalledPath(artifactByName(t, "herdr"), home)

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme with one unrefreshable file installed: %v", err)
	}
	if rec == nil {
		t.Fatal("the switch returned no record for the files it did change")
	}
	if len(rec.Files) != 1 {
		t.Errorf("the record holds %d file(s), want only the one that changed", len(rec.Files))
	}

	// The file that cannot be updated is left exactly as it was.
	stale, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stale, staleBefore) {
		t.Error("the file with no generated block was changed")
	}
	// The rest of the files are applied.
	good, err := os.ReadFile(goodPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(good, goodBefore[goodPath]) {
		t.Error("the file that could be switched was not switched")
	}

	// The switch says which file it could not update, and what refreshes it.
	if !strings.Contains(notice, stalePath) {
		t.Errorf("the notice does not name the file it could not update: %q", notice)
	}
	if !strings.Contains(notice, "generated block") || !strings.Contains(notice, "ownership marker") {
		t.Errorf("the notice does not name the condition: %q", notice)
	}
	if !strings.Contains(notice, "Reinstall") {
		t.Errorf("the notice does not say a reinstall refreshes the file: %q", notice)
	}
}

// TestThemeSwitchNamesEveryFileItCannotUpdate gives the report one line per
// affected file: with two unrefreshable managed files the notice names both and
// counts them, so the single-file case is not the only one covered.
func TestThemeSwitchNamesEveryFileItCannotUpdate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	_, firstPath := installThemeFileOwnedWithoutBlock(t, home, "alacritty")
	_, secondPath := installThemeFileOwnedWithoutBlock(t, home, "kitty")
	installThemeFiles(t, home, "herdr")

	_, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme with two unrefreshable files installed: %v", err)
	}
	for _, path := range []string{firstPath, secondPath} {
		if !strings.Contains(notice, path) {
			t.Errorf("the notice does not name %s: %q", path, notice)
		}
	}
	if !strings.Contains(notice, "2 managed file") {
		t.Errorf("the notice does not count both unrefreshable files: %q", notice)
	}
}

// TestThemeSwitchReportsWhenNothingCanBeUpdated covers the all-or-nothing edge:
// when every installed managed file is unrefreshable, nothing is written and the
// failure names the files and the remedy instead of the generic "no installed
// theme block was found", which would leave the user with no idea which file or
// what to do.
func TestThemeSwitchReportsWhenNothingCanBeUpdated(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	target, _ := themeByID(defs, "catppuccin-mocha")

	staleBefore, stalePath := installThemeFileOwnedWithoutBlock(t, home, "alacritty")

	rec, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err == nil {
		t.Fatal("the switch reported success with nothing it could update")
	}
	if rec != nil {
		t.Errorf("the switch returned a record with nothing to record: %+v", rec)
	}
	if !strings.Contains(err.Error(), stalePath) {
		t.Errorf("the failure does not name the file it could not update: %v", err)
	}
	if !strings.Contains(err.Error(), "Reinstall") {
		t.Errorf("the failure does not say a reinstall refreshes the file: %v", err)
	}
	stale, readErr := os.ReadFile(stalePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(stale, staleBefore) {
		t.Error("the unrefreshable file was changed")
	}
	if rec := readDotfilesThemeRecord(); rec != nil {
		t.Errorf("a failed switch wrote a record: %+v", rec)
	}
}

// TestThemeSwitchAppliesAMarkedFileWithAnchors is the boundary the merge created:
// after region adoption landed, a managed file that carries the ownership marker
// but lost its generated block no longer divides into "rewrite" and "silent
// skip". If it still carries the region anchors, the switch adopts that region,
// so it must NOT be reported as unrefreshable; if it does not, it is named (the
// sibling TestThemeSwitchReportsAnOwnedFileWithNoGeneratedBlock). This keeps the
// anchor-bearing branch alive and pins that the two cases stay apart.
func TestThemeSwitchAppliesAMarkedFileWithAnchors(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)
	def, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)

	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)
	// A marked file whose generated block was replaced by the old hand-written
	// region: the marker is still there, the block is not, the anchors are, and the
	// marker sits exactly where the block carried it, above the region.
	original := userForeignPrefix + themeOwnershipMarkerLine(art) + "\n" +
		legacyThemeFile(t, art, def) + userForeignSuffix
	writeThemeFileAt(t, dst, original)

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to a marked file whose region still carries the anchors: %v", err)
	}
	if rec == nil {
		t.Fatal("the region adoption returned no record, so the change is not reversible")
	}
	if strings.Contains(notice, "cannot be updated") || strings.Contains(notice, "no generated block") {
		t.Errorf("the marked anchor-bearing file was reported as unrefreshable: %q", notice)
	}
	if !strings.Contains(notice, "applied to 1 file") {
		t.Errorf("the marked anchor-bearing file was not applied: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the marked file was not adopted into the generated form:\n%s", got)
	}
	if !strings.Contains(string(got), userForeignPrefix) {
		t.Error("the user's bytes before the region were lost")
	}
	if !strings.HasSuffix(string(got), userForeignSuffix) {
		t.Error("the user's bytes after the region were lost")
	}
	if n := strings.Count(string(got), themeOwnershipMarker); n != 1 {
		t.Errorf("the ownership marker appears %d times after apply, want exactly 1:\n%s", n, got)
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the region adoption: %v", err)
	}
	back, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, []byte(original)) {
		t.Error("undo did not restore the marked file byte-for-byte")
	}
}

// TestThemeInstalledFileIsOursProvesByContent covers the rule the adoption rests
// on: the proof is the bytes, not the path. A file identical to what the
// repository ships without the marker is ours (proof 1), a file carrying a
// generated block marker is ours (proof 2), and anything else is not ours even
// when it sits at exactly the path a managed file lives at.
func TestThemeInstalledFileIsOursProvesByContent(t *testing.T) {
	root := t.TempDir()
	shipped := "# dotfiles-managed-config: x\nbody = 1\n"
	if err := os.WriteFile(filepath.Join(root, "x.conf"), []byte(shipped), 0o644); err != nil {
		t.Fatal(err)
	}
	art := themeArtifact{Tool: "x", Path: "x.conf", Comment: "#"}

	// Proof 1: the repository version without the marker. The shipped fixture
	// carries no generated block, so this proof has to stand on its own.
	if !themeInstalledFileIsOurs(root, art, "body = 1\n") {
		t.Error("a file identical to the shipped file without the marker was not recognised as ours")
	}
	// Proof 2: a generated block marker only this repository's generator writes.
	blockTag := themeBeginTag("")
	if !themeInstalledFileIsOurs(root, art, "# "+blockTag+" x (generated) >>>\nbody = 1\n") {
		t.Error("a file carrying the generated block marker was not recognised as ours")
	}
	// Neither proof: a file the user wrote proves nothing, marker path or not.
	if themeInstalledFileIsOurs(root, art, "# my own file\nbody = 2\n") {
		t.Error("a file that proves nothing was recognised as ours")
	}
	// No repository to compare against leaves only proof 2, so an unmarked file
	// with no block marker stays refused rather than being adopted on faith.
	if themeInstalledFileIsOurs("", art, "body = 1\n") {
		t.Error("an unmarked file was adopted with no shipped file to compare against")
	}
}

// TestThemeAdoptionCoversEveryShippedArtifact guards the class: for every file
// the switch can rewrite, the file the repository ships must still be adoptable
// once only its ownership-marker line is removed. That is the shape an install
// from before the marker leaves behind, so if a future renderer ships an artifact
// whose block is not recognisable, an old installation silently goes back to
// being unable to change the theme.
func TestThemeAdoptionCoversEveryShippedArtifact(t *testing.T) {
	root := repoRoot(t)
	checked := 0
	for _, art := range themeActiveArtifacts {
		shipped, err := os.ReadFile(filepath.Join(root, art.Path))
		if err != nil {
			t.Errorf("read %s: %v", art.Path, err)
			continue
		}
		if !strings.Contains(string(shipped), themeOwnershipMarker) {
			t.Errorf("%s ships no ownership marker, so this guard proves nothing for it", art.Path)
			continue
		}
		if !themeInstalledFileIsOurs(root, art, stripThemeOwnershipMarkers(string(shipped))) {
			t.Errorf("%s is not adoptable after only its marker is removed, so an install from before the marker could not change the theme", art.Path)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no shipped artifact was checked, so this guard proves nothing")
	}
	t.Logf("%d shipped artifact(s) are adoptable after only the marker is removed", checked)
}

// TestGeneratedPerThemeFilesMatchTheirDefinition covers the per-theme whole-file
// artifacts (the fish theme files and the bat .tmTheme files): each must be
// byte-for-byte what its definition produces. Run with -update-theme-artifacts
// to regenerate.
func TestGeneratedPerThemeFilesMatchTheirDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	checked := 0
	for _, file := range themeThemeFiles {
		def, ok := themeByID(defs, file.Theme)
		if !ok {
			t.Errorf("themeThemeFiles names %q, which no definition defines", file.Theme)
			continue
		}
		want, err := file.Render(def)
		if err != nil {
			t.Errorf("render the %s per-theme file: %v", file.Theme, err)
			continue
		}
		path := filepath.Join(repoRoot(t), file.Path)
		got, readErr := os.ReadFile(path)

		if *updateThemeArtifacts {
			// A per-theme file can be the first thing in its directory (the Neovim
			// colorschemes live in dotfiles-nvim/nvim/colors/), so the update path
			// creates it: a guard that cannot regenerate a file whose directory is not
			// in the checkout yet would need a hand-made empty directory to work.
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Errorf("create %s: %v", filepath.Dir(file.Path), err)
				continue
			}
			if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
				t.Errorf("write %s: %v", file.Path, err)
			}
			checked++
			continue
		}
		if readErr != nil {
			t.Errorf("read %s: %v", file.Path, readErr)
			continue
		}
		if string(got) != want {
			t.Errorf("%s no longer matches themes/%s.toml: regenerate it with "+
				"go test ./internal/tui -run TestGeneratedPerThemeFilesMatchTheirDefinition -update-theme-artifacts",
				file.Path, file.Theme)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no per-theme file was checked, so this guard proves nothing")
	}
	if *updateThemeArtifacts {
		t.Logf("regenerated %d per-theme files", checked)
	} else {
		t.Logf("%d per-theme files match their definitions byte-for-byte", checked)
	}
}

// TestTheRemovedStarshipPaletteIsRecreatable covers the condition on deleting
// the unused [palettes.catppuccin_mocha] table: the generator must be able to
// rebuild it from the definition, so nothing was lost. It pins all 26 values the
// deleted table held.
func TestTheRemovedStarshipPaletteIsRecreatable(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	catppuccin, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	table, err := renderStarshipPaletteTable(catppuccin)
	if err != nil {
		t.Fatalf("rebuild the deleted Starship table: %v", err)
	}
	if !strings.Contains(table, "[palettes.catppuccin-mocha]") {
		t.Errorf("the rebuilt table is not named as the deleted one was:\n%s", table)
	}

	// Every role the template can hold, and the values the deleted file held.
	for _, role := range themePromptRoles {
		if catppuccin.Prompt[role] == "" {
			t.Errorf("the definition no longer holds prompt role %q, so the deleted table cannot be rebuilt", role)
			continue
		}
		if !strings.Contains(table, role+" = ") {
			t.Errorf("the rebuilt table has no %q key", role)
		}
	}
	deleted := []string{
		"#f5e0dc", "#f2cdcd", "#f5c2e7", "#cba6f7", "#f38ba8", "#eba0ac", "#fab387",
		"#f9e2af", "#a6e3a1", "#94e2d5", "#89dceb", "#74c7ec", "#89b4fa", "#b4befe",
		"#cdd6f4", "#bac2de", "#a6adc8", "#9399b2", "#7f849c", "#6c7086", "#585b70",
		"#45475a", "#313244", "#1e1e2e", "#181825", "#11111b",
	}
	for _, value := range deleted {
		if !strings.Contains(table, value) {
			t.Errorf("the rebuilt table no longer holds %s, which the deleted one did", value)
		}
	}
	t.Logf("the deleted [palettes.catppuccin_mocha] table rebuilds from themes/catppuccin-mocha.toml with all %d values", len(deleted))
}

// legacyThemeFile is the shape an installer from before the ownership marker
// left on disk: the generated body with its marker and its two tags removed, and
// one colour drifted from what the repository ships. The drift is what defeats
// the content-based adoption (a byte-identical body would be adopted, not
// refused), so this is the file the refresh exists for.
func legacyThemeFile(t *testing.T, art themeArtifact, def themeDefinition) string {
	t.Helper()

	block, err := themeArtifactBlock(art, def)
	if err != nil {
		t.Fatalf("render the %s block: %v", art.Tool, err)
	}
	var kept []string
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, themeOwnershipMarker) {
			continue
		}
		if strings.Contains(line, themeBeginTag(art.Block)) {
			continue
		}
		if strings.Contains(line, themeEndTag(art.Block)) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Replace(strings.Join(kept, "\n"), def.Palette["base"], "#010203", 1)
}

// writeThemeFileAt writes a fixture file, making the parent directories.
func writeThemeFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestThemeRefreshBringsAnOldManagedFileUpToDate is the user's case: the file
// was installed by a checkout that predates the ownership marker and the
// generated block, so the theme switch refuses it and the feature is unusable.
// The refresh must adopt the old file and bring it up to date.
func TestThemeRefreshBringsAnOldManagedFileUpToDate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}

	art := artifactByName(t, "alacritty")
	dst := themeInstalledPath(art, home)
	old := legacyThemeFile(t, art, def)
	writeThemeFileAt(t, dst, old)

	// The file predates the marker and the generated block, which is the state an
	// install from an older checkout leaves behind. The switch can now adopt its
	// region in place (the third ownership proof); the refresh is still the path
	// that preserves a file that may be the user's - into the sourced drop-in
	// directory here - before it replaces it, and records the previous bytes so
	// Undo puts them back. This test keeps that path covered.

	candidates := findThemeRefreshCandidates(home, defs)
	if len(candidates) != 1 {
		t.Fatalf("detection returned %d candidate(s), want the one outdated file: %+v", len(candidates), candidates)
	}
	if c := candidates[0]; c.Path != dst {
		t.Errorf("detection named %q, want %q", c.Path, dst)
	} else if c.Problem != "" {
		t.Fatalf("the outdated file was reported unfixable: %s", c.Problem)
	}

	rec, paragraphs, _, err := refreshThemeFiles(home, defs, candidates)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rec == nil || len(rec.Files) != 1 {
		t.Fatalf("refresh recorded %+v, want the one file it replaced", rec)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeOwnershipMarker) {
		t.Errorf("the refreshed file carries no ownership marker:\n%s", got)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the refreshed file carries no generated block:\n%s", got)
	}
	plain := ansiEscape.ReplaceAllString(strings.Join(paragraphs, "\n"), "")
	if !strings.Contains(plain, dst) {
		t.Errorf("the refresh result does not name %s:\n%s", dst, plain)
	}
}

// TestThemeRefreshPreservesUserContentAndSaysWhere covers the rule that is not
// negotiated: a file that may hold the user's own content is copied to the
// installer's usual place first, and the result says where it went.
func TestThemeRefreshPreservesUserContentAndSaysWhere(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	art := artifactByName(t, "zsh")
	dst := themeInstalledPath(art, home)
	user := "# ─── Palette ─────────────────────────────────────────────────────────────────\n" +
		"typeset -g PALETTE_BASE=\"#06080f\"\n" +
		"Gd=${PALETTE_YELLOW_SGR}\"\n" +
		"# my own zsh aliases\nalias ll='ls -la'\n"
	writeThemeFileAt(t, dst, user)

	candidates := findThemeRefreshCandidates(home, defs)
	if len(candidates) != 1 || candidates[0].Path != dst {
		t.Fatalf("detection returned %+v, want the one outdated %s", candidates, dst)
	}

	_, paragraphs, _, err := refreshThemeFiles(home, defs, candidates)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	dropIn := filepath.Join(home, ".zshrc.d")
	entries, err := os.ReadDir(dropIn)
	if err != nil {
		t.Fatalf("the preserved file is not in %s: %v", dropIn, err)
	}
	if len(entries) != 1 {
		t.Fatalf("%s holds %d entries, want the one preserved file", dropIn, len(entries))
	}
	preserved := filepath.Join(dropIn, entries[0].Name())
	back, err := os.ReadFile(preserved)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != user {
		t.Errorf("the preserved copy is not byte-for-byte the user's file:\n--- got ---\n%s\n--- want ---\n%s", back, user)
	}
	if got, _ := os.ReadFile(dst); !strings.Contains(string(got), themeOwnershipMarker) {
		t.Error("the user's .zshrc was not refreshed after it was preserved")
	}
	plain := ansiEscape.ReplaceAllString(strings.Join(paragraphs, "\n"), "")
	if !strings.Contains(plain, preserved) {
		t.Errorf("the result does not say where the file was preserved (%s):\n%s", preserved, plain)
	}
}

// TestThemeRefreshIsReversible covers the record: the previous bytes are stored
// the same way the switch stores them, so Undo puts the file back byte-for-byte.
func TestThemeRefreshIsReversible(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, _ := themeByID(defs, defaultThemeID)
	art := artifactByName(t, "alacritty")
	dst := themeInstalledPath(art, home)
	old := legacyThemeFile(t, art, def)
	writeThemeFileAt(t, dst, old)

	candidates := findThemeRefreshCandidates(home, defs)
	rec, _, _, err := refreshThemeFiles(home, defs, candidates)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rec == nil {
		t.Fatal("the refresh returned no record, so it is not reversible")
	}
	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the refresh: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != old {
		t.Errorf("undo did not restore the file byte-for-byte")
	}
	if readDotfilesThemeRecord() != nil {
		t.Error("the record was not cleared after the undo")
	}
}

// TestThemeRefreshSkipsOnDryRun covers the write gate: a dry run changes
// nothing, on disk or in the record.
func TestThemeRefreshSkipsOnDryRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "1")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, _ := themeByID(defs, defaultThemeID)
	art := artifactByName(t, "alacritty")
	dst := themeInstalledPath(art, home)
	old := legacyThemeFile(t, art, def)
	writeThemeFileAt(t, dst, old)

	candidates := findThemeRefreshCandidates(home, defs)
	rec, _, notice, err := refreshThemeFiles(home, defs, candidates)
	if err != nil {
		t.Fatalf("a dry run returned an error: %v", err)
	}
	if rec != nil {
		t.Errorf("a dry run recorded a change: %+v", rec)
	}
	if !strings.Contains(notice, "DRY RUN") {
		t.Errorf("a dry run's notice does not say so: %q", notice)
	}
	if got, _ := os.ReadFile(dst); string(got) != old {
		t.Error("a dry run wrote the file")
	}
	if entries, _ := os.ReadDir(filepath.Join(home, ".zshrc.d")); len(entries) != 0 {
		t.Errorf("a dry run created %d preserved file(s)", len(entries))
	}
	if readDotfilesThemeRecord() != nil {
		t.Error("a dry run wrote a record")
	}
}

// TestThemeRefreshSkipsAnUnrefreshableFileAndContinues covers honest
// degradation: a file that cannot be refreshed is named and skipped, and the
// files around it are still refreshed. It never aborts the whole refresh.
func TestThemeRefreshSkipsAnUnrefreshableFileAndContinues(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, _ := themeByID(defs, defaultThemeID)

	goodArt := artifactByName(t, "alacritty")
	goodDst := themeInstalledPath(goodArt, home)
	goodOld := legacyThemeFile(t, goodArt, def)
	writeThemeFileAt(t, goodDst, goodOld)

	// One unrefreshable file because its content is not a recognizable block.
	badArt := artifactByName(t, "herdr")
	badDst := themeInstalledPath(badArt, home)
	badContent := "# unrelated configuration\nsome_setting = true\n"
	writeThemeFileAt(t, badDst, badContent)

	// One unrefreshable because it is not a regular file at all.
	dirArt := artifactByName(t, "ghostty")
	dirDst := themeInstalledPath(dirArt, home)
	if err := os.MkdirAll(dirDst, 0o755); err != nil {
		t.Fatal(err)
	}

	candidates := findThemeRefreshCandidates(home, defs)
	if len(candidates) != 3 {
		t.Fatalf("detection returned %d candidate(s), want the refreshable file and the two unfixable ones: %+v", len(candidates), candidates)
	}

	rec, paragraphs, notice, err := refreshThemeFiles(home, defs, candidates)
	if err != nil {
		t.Fatalf("a refresh with an unfixable file returned an error: %v", err)
	}
	if rec == nil || len(rec.Files) != 1 {
		t.Fatalf("the refreshable file was not refreshed alongside the failures: %+v", rec)
	}
	if got, _ := os.ReadFile(goodDst); !strings.Contains(string(got), themeOwnershipMarker) {
		t.Error("the refreshable file was not brought up to date")
	}
	if got, _ := os.ReadFile(badDst); string(got) != badContent {
		t.Error("the unrecognizable file was changed")
	}
	if info, err := os.Stat(dirDst); err != nil || !info.IsDir() {
		t.Error("the directory that is not a regular file was changed")
	}
	plain := ansiEscape.ReplaceAllString(strings.Join(paragraphs, "\n"), "")
	for _, path := range []string{badDst, dirDst} {
		if !strings.Contains(plain, path) {
			t.Errorf("the result does not name the unfixable file %s:\n%s", path, plain)
		}
	}
	if !strings.Contains(notice, "could not") {
		t.Errorf("the notice does not say a file could not be refreshed: %q", notice)
	}
}
