package tui

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

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
