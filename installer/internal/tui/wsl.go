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

// stepInstallWSLConfig copies the WSL artifacts shipped in dotfiles-wsl into the
// two places WSL actually reads: the Windows user profile for .wslconfig, and
// /etc/wsl.conf inside the running distribution.
//
// The step is a no-op outside WSL, so it is safe to call unconditionally.
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

	// .wslconfig is a Windows file: it belongs in the Windows user profile. The
	// lookup can legitimately fail (interop disabled, unusual mount layout) and
	// that must not stop the in-distribution half of this step.
	wslconfigSrc := filepath.Join(repoDir, repoAssetWSLConfig)
	profileDir, profileErr := windowsUserProfile()
	if profileErr != nil {
		SendLog(wslStepID, fmt.Sprintf(
			"Skipping .wslconfig: %v. Set %s to override the lookup.", profileErr, envWSLWindowsHome))
	} else {
		destination := filepath.Join(profileDir, ".wslconfig")
		if err := applyArtifact(wslconfigSrc, destination, wslStepID); err != nil {
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

	// Preferred route: ask cmd.exe and translate the Windows path with wslpath.
	if userProfile := windowsEnvVar("USERPROFILE"); userProfile != "" {
		if translated := wslPathUnix(userProfile); translated != "" && system.DirExists(translated) {
			return translated, nil
		}
	}

	// Fallback: the conventional mount point plus the Windows user name.
	if userName := windowsEnvVar("USERNAME"); userName != "" {
		candidate := filepath.Join("/mnt/c/Users", userName)
		if system.DirExists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not resolve the Windows user profile")
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
// It writes directly when the current user owns the destination and escalates
// to sudo only when the plain write is rejected, so an unprivileged temporary
// destination (used in tests) never triggers a password prompt.
func applyArtifact(src, dst, stepID string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}

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
