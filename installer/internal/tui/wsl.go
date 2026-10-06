package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

const (
	// wslStepID is the installation step that applies the WSL artifacts.
	wslStepID = "wslconfig"

	// envWSLWindowsHome overrides the Windows profile lookup. It exists for
	// tests and for setups where the Windows drive is mounted somewhere other
	// than /mnt/c.
	envWSLWindowsHome = "DOTFILES_WSL_WINDOWS_HOME"

	// envWSLConfPath overrides the in-distribution wsl.conf destination. It
	// exists for tests and for distributions that keep the file elsewhere.
	envWSLConfPath = "DOTFILES_WSL_CONF_PATH"

	// envWSLUsersDir overrides the Windows users mount the interop-free profile
	// lookup scans. It exists for tests and for a Windows drive mounted somewhere
	// other than /mnt/c.
	envWSLUsersDir = "DOTFILES_WSL_USERS_DIR"

	// defaultWindowsUsersMount is where WSL mounts the Windows user profiles by
	// default.
	defaultWindowsUsersMount = "/mnt/c/Users"

	// defaultWSLConfPath is where WSL reads the per-distribution settings.
	defaultWSLConfPath = "/etc/wsl.conf"

	// defaultBinfmtDir is where the kernel exposes its binary format handlers.
	defaultBinfmtDir = "/proc/sys/fs/binfmt_misc"

	// envBinfmtDir overrides the binfmt directory, so the interop check can be
	// exercised against a fixture. It exists for tests only.
	envBinfmtDir = "DOTFILES_BINFMT_DIR"

	// win32yankVersion is the release the clipboard bridge is pinned to.
	win32yankVersion = "v0.1.1"

	// win32yankArchive is the only asset that release offers; there is no ARM64
	// build, so an ARM64 Windows host runs this one through emulation.
	win32yankArchive = "https://github.com/equalsraf/win32yank/releases/download/" +
		win32yankVersion + "/win32yank-x64.zip"

	// win32yankSHA256 pins the archive. The installer downloads it and then
	// executes what it contains, so the checksum is checked before anything is
	// unpacked.
	win32yankSHA256 = "247c9a05b94387a884b49d3db13f806b1677dfc38020f955f719be6902260cd6"
)

// stepInstallWSLConfig installs the WSL artifacts shipped in dotfiles-wsl into
// the two places WSL actually reads: the Windows user profile for .wslconfig,
// and /etc/wsl.conf inside the running distribution.
//
// The .wslconfig template is rendered for the Windows host this installer runs
// on, so the machine-derived keys match the real capacities instead of the
// values of whichever machine the file was authored on. The step is a no-op
// outside WSL, so it is safe to call unconditionally.
func stepInstallWSLConfig(m *Model) error {
	if !m.SystemInfo.IsWSL {
		SendLog(wslStepID, "Not running under WSL; nothing to configure")
		return nil
	}

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(wslStepID, "Configure WSL",
			"Failed to locate the cloned repository", err)
	}

	// The machine-derived keys come from the Windows host. The content is built
	// once, by the same builder the utilities section calls, and written by the
	// same writer, so the two routes cannot disagree about the file they install.
	profileDir, profileErr := windowsUserProfile()
	if profileErr != nil {
		SendLog(wslStepID, fmt.Sprintf(
			"Skipping .wslconfig: %v. Set %s to override the lookup.", profileErr, envWSLWindowsHome))
	} else {
		destination := filepath.Join(profileDir, ".wslconfig")
		if err := installRepoWSLConfig(repoDir, destination, wslStepID); err != nil {
			return wrapStepError(wslStepID, "Configure WSL",
				"Failed to install .wslconfig into the Windows user profile", err)
		}
		SendLog(wslStepID, fmt.Sprintf("✓ .wslconfig installed at %s", destination))
	}

	// wsl.conf is read from inside the distribution and needs root to replace.
	confDst := os.Getenv(envWSLConfPath)
	if confDst == "" {
		confDst = defaultWSLConfPath
	}
	if err := applyArtifact(filepath.Join(repoDir, repoAssetWSLConf), confDst, wslStepID); err != nil {
		return wrapStepError(wslStepID, "Configure WSL",
			"Failed to install /etc/wsl.conf", err)
	}
	SendLog(wslStepID, fmt.Sprintf("✓ wsl.conf installed at %s", confDst))

	// Deliberately not fatal, for the same reason the xclip and wl-clipboard
	// providers are not: the editor works without a clipboard bridge, and a
	// download that fails should not fail an otherwise completed WSL setup.
	if !peInteropHealthy() {
		SendLog(wslStepID, "Skipping win32yank: this distribution cannot execute Windows binaries stored on the Linux filesystem, so the bridge would install into a path that cannot run. Neovim will keep using wl-clipboard. See the binfmt note in this step's source.")
	} else if err := installWin32Yank(wslStepID); err != nil {
		SendLog(wslStepID, "Could not install win32yank, so Neovim will not reach the Windows clipboard: "+err.Error())
	} else {
		SendLog(wslStepID, "✓ Windows clipboard bridge ready")
	}

	// Both files are only read when the WSL VM starts.
	SendLog(wslStepID, "Run `wsl --shutdown` on Windows and reopen the terminal to apply the changes")
	return nil
}

// renderedRepoWSLConfig reads the shipped template, detects the Windows host,
// renders the machine-derived keys for it, and merges the result over the
// .wslconfig this machine already has.
//
// The merge happens here as well as in the step because the interactive route
// copies these bytes straight over the destination (getWSLConfigScript's
// install_artifact, interactive.go): the interactive install, the
// non-interactive step and the utilities section all preserve the user's own
// keys, or none of them does. The destination is resolved the same way the
// interactive script resolves it and the same way the utilities section resolves
// it, so all three name one file.
//
// The host and the plan come back with the bytes so the caller can report where
// the values came from.
func renderedRepoWSLConfig(repoDir string) ([]byte, system.HostResources, WSLResources, error) {
	// A destination that cannot be resolved leaves nothing to merge with; the
	// interactive script skips the .wslconfig write in that case anyway, and the
	// step reports the lookup failure itself.
	destination := ""
	if path, err := wslConfigDestination(); err == nil {
		destination = path
	}
	return wslConfigContentForHost(repoDir, destination)
}

// installRepoWSLConfig builds the .wslconfig for this host and installs it at
// dst. It is the installation route into the shared content builder and the
// shared writer, and the utilities section's write goes through the same two
// functions: one render, one merge, one writer, two entries.
func installRepoWSLConfig(repoDir, dst, stepID string) error {
	content, host, plan, err := wslConfigContentForHost(repoDir, dst)
	if err != nil {
		return err
	}
	logWSLResourcePlan(host, plan)

	if _, err := writeWSLConfig(content, dst, stepID); err != nil {
		return err
	}
	return nil
}

// logWSLResourcePlan reports the machine-derived part of the rendered
// .wslconfig: the host capacities it was computed from, the three planned
// values, and whether they came from host detection or from the
// omit-everything fallback, so a user can audit what the installer chose for
// this machine.
func logWSLResourcePlan(host system.HostResources, plan WSLResources) {
	if host.MemoryBytes == 0 && host.LogicalCPUs == 0 {
		SendLog(wslStepID, "Windows host capacities could not be read; omitting memory, processors and swap so WSL applies its own proportional defaults")
	} else {
		SendLog(wslStepID, fmt.Sprintf("Detected Windows host capacities: %d MiB RAM, %d logical CPUs",
			host.MemoryBytes/bytesPerMiB, host.LogicalCPUs))
	}

	SendLog(wslStepID, fmt.Sprintf("Derived .wslconfig values: memory=%s, processors=%s, swap=%s",
		wslResourceLabel(plan.MemoryMB, "MB"),
		wslResourceLabel(plan.Processors, ""),
		wslResourceLabel(plan.SwapMB, "MB")))
}

// wslResourceLabel renders one planned value, naming an omitted key as such
// instead of printing a zero that would read as a real limit.
func wslResourceLabel(value int, unit string) string {
	if value <= 0 {
		return "omitted"
	}
	return fmt.Sprintf("%d%s", value, unit)
}

// peInteropHealthy reports whether WSL can execute a Windows binary that lives on
// the Linux filesystem, which is what a win32yank.exe in ~/.local/bin has to be.
//
// WSL dispatches PE files to /init through a binfmt_misc entry named WSLInterop.
// Installing another binfmt manager removes that entry, which several packages
// do, and WSL then re-registers it as WSLInterop-late with fewer flags. The late
// form still runs Windows binaries kept under /mnt/c, so it looks healthy, but a
// PE file placed on the Linux filesystem fails with EINVAL. That is the state of
// the machine this was written for: cmd.exe under /mnt/c runs, the same binary
// copied to /tmp fails with "Invalid argument", and win32yank.exe fails the same
// way.
//
// Repairing it means re-registering the canonical entry as root, which is a
// kernel-level change this installer has no verified way to make, so the bridge
// is skipped and reported instead of installed and silently dead.
func peInteropHealthy() bool {
	dir := os.Getenv(envBinfmtDir)
	if dir == "" {
		dir = defaultBinfmtDir
	}
	data, err := os.ReadFile(filepath.Join(dir, "WSLInterop"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "enabled")
}

// installWin32Yank installs the bridge from the distribution's clipboard to the
// Windows one.
//
// Neovim detects WSL and, when win32yank.exe is on PATH, uses it to reach the
// Windows clipboard. That is more dependable than WSLg's Wayland clipboard, which
// exists only while WSLg is running and disappears along with its runtime
// directory: the same missing-paths failure that broke fnm on every new pane
// earlier. It is also what makes copying work when there is no Wayland display at
// all, which is the normal case over SSH or with WSLg disabled.
func installWin32Yank(stepID string) error {
	if system.CommandExists("win32yank.exe") {
		SendLog(stepID, "win32yank already installed")
		return nil
	}

	binDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := system.EnsureDir(binDir); err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "win32yank")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	archive := filepath.Join(workDir, "win32yank.zip")
	SendLog(stepID, "Downloading win32yank for the Windows clipboard...")
	download := system.RunWithLogs(fmt.Sprintf("curl -fsSL %q -o %q", win32yankArchive, archive), nil, func(line string) {
		SendLog(stepID, line)
	})
	if download.Error != nil {
		return download.Error
	}

	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	actual := sha256.Sum256(data)
	if hex.EncodeToString(actual[:]) != win32yankSHA256 {
		return fmt.Errorf("win32yank checksum mismatch for %s", win32yankArchive)
	}

	// The archive also carries a licence and a readme. Only the executable is
	// wanted, and it is moved rather than copied so the temporary directory can be
	// discarded whole.
	extract := system.RunWithLogs(fmt.Sprintf("unzip -o -q %q -d %q", archive, workDir), nil, func(line string) {
		SendLog(stepID, line)
	})
	if extract.Error != nil {
		return extract.Error
	}

	dest := filepath.Join(binDir, "win32yank.exe")
	if err := os.Rename(filepath.Join(workDir, "win32yank.exe"), dest); err != nil {
		return err
	}
	// A Windows executable launched through WSL interop still needs the
	// executable bit on the Linux side.
	return os.Chmod(dest, 0755)
}

// windowsUserProfile resolves the Windows %USERPROFILE% directory as a path
// inside the Linux mount, so the installer can write .wslconfig where WSL reads
// it from.
func windowsUserProfile() (string, error) {
	if override := os.Getenv(envWSLWindowsHome); override != "" {
		if !system.DirExists(override) {
			return "", fmt.Errorf("%s points to %s, which is not a directory", envWSLWindowsHome, override)
		}
		return override, nil
	}

	// Preferred route, when interop works: ask cmd.exe for %USERPROFILE% and
	// translate the Windows path with wslpath. This is the only route that
	// resolves a Windows drive mounted somewhere unusual, so it is tried first.
	if userProfile := windowsEnvVar("USERPROFILE"); userProfile != "" {
		if translated := wslPathUnix(userProfile); translated != "" && system.DirExists(translated) {
			return translated, nil
		}
	}

	// Fallback that never invokes interop. It exists for the host whose
	// WSLInterop binfmt entry is missing: cmd.exe cannot run there, so the route
	// above fails for the same reason the fallback is needed, and the .wslconfig
	// half of the step would be skipped on exactly those machines.
	return windowsProfileFromMount()
}

// windowsProfileFromMount resolves the Windows user profile by reading the
// mounted users directory. It never runs cmd.exe or wslpath, so it works on a
// distribution whose WSLInterop binfmt entry is missing.
//
// A single candidate directory is used directly. With several, the one named
// after the Linux user wins, because WSL creates the Linux account from the
// Windows one by default and the names normally match. If none matches, the
// lookup is ambiguous and is refused with the candidates named, so an arbitrary
// profile is never written to; DOTFILES_WSL_WINDOWS_HOME chooses one explicitly.
func windowsProfileFromMount() (string, error) {
	usersDir := os.Getenv(envWSLUsersDir)
	if usersDir == "" {
		usersDir = defaultWindowsUsersMount
	}

	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w; set %s to override the lookup", usersDir, err, envWSLWindowsHome)
	}

	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() || windowsSystemUserDir(entry.Name()) {
			continue
		}
		candidates = append(candidates, filepath.Join(usersDir, entry.Name()))
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no Windows user profile found under %s; set %s to override the lookup", usersDir, envWSLWindowsHome)
	case 1:
		return candidates[0], nil
	}

	for _, candidate := range candidates {
		if windowsUserMatches(candidate, os.Getenv("USER")) || windowsUserMatches(candidate, os.Getenv("USERNAME")) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("several Windows user profiles found under %s (%s); set %s to choose one",
		usersDir, strings.Join(baseNames(candidates), ", "), envWSLWindowsHome)
}

// windowsSystemUserDir reports whether a directory under the Windows users
// mount belongs to Windows rather than to a person. Writing .wslconfig into one
// of these would either fail or configure the wrong account.
func windowsSystemUserDir(name string) bool {
	switch strings.ToLower(name) {
	case "public", "default", "default user", "all users", "defaultapppool":
		return true
	}
	return false
}

// windowsUserMatches reports whether the profile directory is named after the
// current Linux (or Windows) account.
func windowsUserMatches(candidate, name string) bool {
	return name != "" && strings.EqualFold(filepath.Base(candidate), name)
}

// baseNames returns the final path element of each path, for a message that
// names the candidates without repeating the whole mount path in front of each.
func baseNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}

// windowsEnvVar reads a Windows environment variable through cmd.exe and strips
// the trailing carriage return and any cmd.exe startup noise.
func windowsEnvVar(name string) string {
	cmd := exec.Command("cmd.exe", "/c", "echo", "%"+name+"%")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		value := strings.TrimSpace(lines[i])
		// An unresolved variable comes back verbatim as %NAME%.
		if value == "" || strings.HasPrefix(value, "%") {
			continue
		}
		return value
	}
	return ""
}

// wslPathUnix translates a Windows path into its Linux mount equivalent.
func wslPathUnix(windowsPath string) string {
	out, err := exec.Command("wslpath", "-u", windowsPath).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// applyArtifact copies src over dst, backing up an existing destination first.
// It reads the file and delegates to applyArtifactContent, so a pre-rendered
// artifact (the .wslconfig the step renders at install time) takes exactly the
// same path as a file copied straight from the checkout.
func applyArtifact(src, dst, stepID string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}

	return applyArtifactContent(data, dst, stepID)
}

// applyArtifactContent installs data over dst, backing up an existing
// destination first. It writes directly when the current user owns the
// destination and escalates to sudo only when the plain write is rejected, so
// an unprivileged temporary destination (used in tests) never triggers a
// password prompt.
func applyArtifactContent(data []byte, dst, stepID string) error {
	if _, err := os.Stat(dst); err == nil {
		backup := fmt.Sprintf("%s.bak-dotfiles-%s", dst, time.Now().Format("20060102-150405"))
		if err := copyArtifact(dst, backup, stepID); err != nil {
			// A destination that is readable but not yet writable (a root-owned
			// /etc/wsl.conf, for instance) is precisely what the escalating write
			// below is for, so a failed backup warns instead of aborting.
			SendLog(stepID, fmt.Sprintf("Warning: could not back up %s: %v", dst, err))
		} else {
			SendLog(stepID, fmt.Sprintf("Previous %s backed up to %s", filepath.Base(dst), backup))
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
		if err := os.WriteFile(dst, data, 0o644); err == nil {
			return nil
		}
	}

	return writeWithSudo(data, dst, stepID)
}

// copyArtifact copies dst to backup, escalating to sudo when the direct copy is
// not permitted. The escalation covers both an unreadable source and a
// destination directory this user cannot write to.
func copyArtifact(dst, backup, stepID string) (err error) {
	data, readErr := os.ReadFile(dst)
	switch {
	case readErr == nil:
		if writeErr := os.WriteFile(backup, data, 0o644); writeErr == nil {
			return nil
		}
	case !os.IsPermission(readErr):
		return readErr
	}

	result := runSudoWithLogs(fmt.Sprintf("cp -a %q %q", dst, backup), nil, func(line string) {
		SendLog(stepID, line)
	})
	return result.Error
}

// writeWithSudo installs the artifact through a temporary file using sudo.
func writeWithSudo(data []byte, dst, stepID string) error {
	tmp, err := os.CreateTemp("", "dotfiles-wsl-artifact-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	result := runSudoWithLogs(fmt.Sprintf("install -m 0644 %q %q", tmp.Name(), dst), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return result.Error
	}

	// Verify the file actually landed with the expected content.
	written, err := os.ReadFile(dst)
	if err != nil {
		return err
	}
	if string(written) != string(data) {
		return fmt.Errorf("%s was not written correctly", dst)
	}
	return nil
}

// ============================================================================
// THE UTILITIES SECTION: THE SAME FILE, REACHED FROM THE MENU
// ============================================================================
//
// The installation step above applies .wslconfig as part of a normal install, so
// a freshly installed machine already comes out right. This section is the other
// way in: it shows the managed values the file holds today beside the values
// recommended from the real Windows host, lets the user change memory,
// processors and swap, and writes them back through the same content builder and
// the same writer the step uses. Everything below is reading or reporting; the
// calculation (PlanWSLResources) and the write (writeWSLConfig) are shared.

// wslResourceState is everything the utility draws and writes: where the file
// is, what the host can give, what was recommended from it, what the file holds
// now, and the draft the user is editing.
//
// It is read in a command rather than in View, because detecting the host runs
// powershell.exe and a screen must never block on interop.
type wslResourceState struct {
	// Resolved reports whether the read finished. Until it has, the section says
	// it is still reading rather than claiming there is nothing to offer.
	Resolved bool
	// Available reports whether the utility is offered at all.
	Available bool
	// Reason is why it is not offered, in the section's own words.
	Reason string
	// Path is the user's .wslconfig, the file WSL reads.
	Path string
	// RepoDir is the checkout holding the shipped template the write renders.
	RepoDir string
	// Host is what Windows reported, and Plan is the recommendation derived from
	// it by PlanWSLResources -- the same call the installation step makes.
	Host system.HostResources
	Plan WSLResources
	// Current is what the file sets today; a zero field is a key the file omits.
	Current WSLResources
	// HasFile reports whether there is a .wslconfig to read at all.
	HasFile bool
	// Draft is what the rows edit and the write applies.
	Draft WSLResources
}

// wslConfigDestination resolves the .wslconfig the utility edits: the same
// Windows user profile the installation step installs into, through the same
// lookup and the same override.
func wslConfigDestination() (string, error) {
	profileDir, err := windowsUserProfile()
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, ".wslconfig"), nil
}

// resolveWSLTemplateDir finds the checkout that holds the shipped WSL template,
// using the same search order the theme definitions use: $DOTFILES_DIR, the
// clone, the working directory and its parents, ~/dotfiles and ~/.dotfiles. The
// utility is reached from the menu, so it cannot assume the clone step ran.
func resolveWSLTemplateDir(repoDir string) (string, error) {
	for _, dir := range themeDefinitionDirs(repoDir) {
		if _, err := os.Stat(filepath.Join(dir, repoAssetWSLConfig)); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no repository holding %s was found in $%s, the clone, the working directory or its parents, ~/dotfiles or ~/.dotfiles",
		repoAssetWSLConfig, dotfilesDirEnv)
}

// loadWSLResourceState reads everything the utility needs. The unavailable
// answers are all resolved here, into a reason the section states in its own
// body: a utility that is missing without saying why is the failure this shape
// exists to prevent.
func loadWSLResourceState(repoDir string, isWSL bool) wslResourceState {
	st := wslResourceState{Resolved: true}

	if !isWSL {
		st.Reason = ".wslconfig is a Windows file read only by WSL"
		return st
	}

	path, err := wslConfigDestination()
	if err != nil {
		st.Reason = err.Error()
		return st
	}
	st.Path = path

	resolvedRepo, err := resolveWSLTemplateDir(repoDir)
	if err != nil {
		st.Reason = err.Error()
		return st
	}
	st.RepoDir = resolvedRepo

	st.Host = system.DetectHostResources()
	st.Plan = PlanWSLResources(st.Host)

	if existing, readErr := os.ReadFile(path); readErr == nil {
		st.HasFile = true
		st.Current = ParseWSLConfigValues(existing)
	}

	st.Draft = wslDraftFrom(st.Current, st.Plan)
	st.Available = true
	return st
}

// writeWSLResourceDraft writes the draft through the shared content builder and
// the shared writer, and says what it did. It recomputes nothing: the plan on
// screen came from PlanWSLResources and the file is built by wslConfigContent,
// which is what makes the two routes one implementation. wrote reports whether
// the file was actually written, which a dry run never is.
func writeWSLResourceDraft(st wslResourceState, stepID string) (notice string, wrote bool, err error) {
	content, err := wslConfigContent(st.RepoDir, st.Draft, st.Path)
	if err != nil {
		return "", false, err
	}

	wrote, err = writeWSLConfig(content, st.Path, stepID)
	if err != nil {
		return "", false, err
	}
	if !wrote {
		return fmt.Sprintf("DRY RUN: %s was not changed.", st.Path), false, nil
	}
	return fmt.Sprintf("%s written. Run `wsl --shutdown` on Windows and reopen the terminal to apply it.", st.Path), true, nil
}
