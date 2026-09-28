package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Pinned OfficeCLI release used by the installer.
//
// The upstream project publishes a shell installer at d.officecli.ai, but that
// script is unpinned: it always fetches whatever is current, so it cannot be
// verified before it runs. The installer instead downloads one versioned
// release asset per platform and checks it against the SHA-256 recorded here
// before the bytes are made executable, so a changed upstream or a tampered
// transfer is refused rather than installed.
//
// The update process for these pins is documented in docs/ai-configuration.md
// ("Updating the pinned OfficeCLI release").
const (
	officeCLIReleaseRepo    = "iOfficeAI/OfficeCLI"
	officeCLIReleaseVersion = "v1.0.152"
	officeCLIReleaseBaseURL = "https://github.com/" + officeCLIReleaseRepo +
		"/releases/download/" + officeCLIReleaseVersion + "/"

	officeCLILinuxX64Name    = "officecli-linux-x64"
	officeCLILinuxX64SHA     = "e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9"
	officeCLILinuxARM64Name  = "officecli-linux-arm64"
	officeCLILinuxARM64SHA   = "bc06deaa0ad931f5208717a40b94018dc44cdff0d8eefa842c4f4daf89fb35a8"
	officeCLIAlpineX64Name   = "officecli-linux-alpine-x64"
	officeCLIAlpineX64SHA    = "390e246303bf43b4739e3195e9b171a660223b4c11218b65894d5fadf9a52755"
	officeCLIAlpineARM64Name = "officecli-linux-alpine-arm64"
	officeCLIAlpineARM64SHA  = "65c65e05100bac1376e23f6ca97086a046afdfcaf50f2bc215a8c139b3bc22ba"
	officeCLIMacX64Name      = "officecli-mac-x64"
	officeCLIMacX64SHA       = "5071abef56c1d4a4d60e28ed12bc66183d8dc6a9783529c3f1a9cf6bdfe6c2dd"
	officeCLIMacARM64Name    = "officecli-mac-arm64"
	officeCLIMacARM64SHA     = "e2ed6eba5cd46d6800139f2835097828b8ccd7c8c9b679463b50e45ba2f1dbf5"
)

// officeCLIStepID is the step that installs the pinned OfficeCLI binary. It
// needs no sudo, so it runs through the shared executor rather than the
// interactive script builders.
const officeCLIStepID = "officecli"

// Alpine publishes musl-linked Linux assets, so the installer has to pick them
// on an Alpine host instead of the glibc build. The marker file is the same one
// apk and other tools use.
const (
	defaultAlpineReleasePath = "/etc/alpine-release"
	envAlpineReleasePath     = "DOTFILES_ALPINE_RELEASE"
)

// officeCLIAsset is one pinned release artifact: the file name to append to the
// release base URL and the SHA-256 its bytes must hash to.
type officeCLIAsset struct {
	name   string
	sha256 string
}

// officeCLIAssetForPlatform maps a target platform to its pinned release asset.
//
// goos and goarch are the values runtime.GOOS and runtime.GOARCH would return,
// and alpine reports whether a Linux host is Alpine/musl. The second return
// value is false for a platform the release does not build for, which the
// caller turns into a clear failure instead of guessing an asset.
func officeCLIAssetForPlatform(goos, goarch string, alpine bool) (officeCLIAsset, bool) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			if alpine {
				return officeCLIAsset{officeCLIAlpineX64Name, officeCLIAlpineX64SHA}, true
			}
			return officeCLIAsset{officeCLILinuxX64Name, officeCLILinuxX64SHA}, true
		case "arm64":
			if alpine {
				return officeCLIAsset{officeCLIAlpineARM64Name, officeCLIAlpineARM64SHA}, true
			}
			return officeCLIAsset{officeCLILinuxARM64Name, officeCLILinuxARM64SHA}, true
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return officeCLIAsset{officeCLIMacX64Name, officeCLIMacX64SHA}, true
		case "arm64":
			return officeCLIAsset{officeCLIMacARM64Name, officeCLIMacARM64SHA}, true
		}
	}
	return officeCLIAsset{}, false
}

// officeCLIAssetURL builds the download URL for one pinned asset.
func officeCLIAssetURL(assetName string) string {
	return officeCLIReleaseBaseURL + assetName
}

// isAlpineLinux reports whether the host is an Alpine/musl Linux, which decides
// between the glibc and the Alpine release assets. envAlpineReleasePath lets a
// test point the probe at a fixture instead of the real marker file.
func isAlpineLinux() bool {
	path := os.Getenv(envAlpineReleasePath)
	if path == "" {
		path = defaultAlpineReleasePath
	}
	return system.PathExists(path)
}

// officeCLITargetPlatform reports the platform the running installer should
// install for. It is a variable so tests can exercise an unsupported host or the
// Alpine asset without cross-compiling the test binary.
var officeCLITargetPlatform = func() (goos, goarch string, alpine bool) {
	return runtime.GOOS, runtime.GOARCH, isAlpineLinux()
}

// officeCLIAssetLookup resolves a platform to its pinned asset. It is a variable
// so tests can substitute a locally generated fixture whose SHA-256 they control
// while still exercising the real download, verify, chmod and rename sequence.
var officeCLIAssetLookup = officeCLIAssetForPlatform

// stepInstallOfficeCLI installs the pinned OfficeCLI release binary into
// ~/.local/bin/officecli.
//
// It is idempotent and never discards a user's binary: an existing destination
// is reported and left untouched. A missing one is staged beside the
// destination, checksum-verified, made executable and then renamed into place,
// so a failed download or a mismatch leaves neither a target nor a staging
// directory behind. Termux is skipped with a clear log because the release
// publishes no Android asset, and any other unsupported OS/architecture fails
// closed instead of installing an unverified or incompatible binary.
func stepInstallOfficeCLI(m *Model) error {
	stepID := officeCLIStepID

	homeDir := os.Getenv("HOME")
	if homeDir == "" && m.SystemInfo != nil {
		homeDir = m.SystemInfo.HomeDir
	}
	if homeDir == "" {
		return wrapStepError(stepID, "Install OfficeCLI",
			"Cannot determine the home directory to install OfficeCLI into",
			errors.New("HOME is not set"))
	}

	binDir := filepath.Join(homeDir, ".local", "bin")
	dest := filepath.Join(binDir, "officecli")

	// Preserve whatever is already there. A newer or locally built binary
	// survives an installer run untouched.
	if system.PathExists(dest) {
		SendLog(stepID, fmt.Sprintf("OfficeCLI already installed at %s; leaving it untouched", dest))
		return nil
	}

	// The release carries no Android/Termux asset, so this host is skipped
	// rather than failed: OfficeCLI is optional and the rest of the run stands.
	if m.SystemInfo != nil && m.SystemInfo.IsTermux {
		SendLog(stepID, fmt.Sprintf(
			"Skipping OfficeCLI: the %s release has no Android/Termux asset. Install it with `pkg install officecli` if a Termux build is available.",
			officeCLIReleaseVersion))
		return nil
	}

	goos, goarch, alpine := officeCLITargetPlatform()
	asset, ok := officeCLIAssetLookup(goos, goarch, alpine)
	if !ok {
		return wrapStepError(stepID, "Install OfficeCLI",
			fmt.Sprintf("OfficeCLI %s has no release asset for %s/%s", officeCLIReleaseVersion, goos, goarch),
			errors.New("unsupported platform"))
	}

	if err := system.EnsureDir(binDir); err != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			"Failed to create ~/.local/bin", err)
	}

	// Staging lives in the destination directory so the final rename stays on
	// one filesystem and is atomic. It is removed on every path out.
	stagingDir, err := os.MkdirTemp(binDir, ".officecli-staging-")
	if err != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			"Failed to create a staging directory", err)
	}
	defer func() {
		if err := os.RemoveAll(stagingDir); err != nil {
			SendLog(stepID, fmt.Sprintf("Warning: could not remove the staging directory %s: %v", stagingDir, err))
		}
	}()

	staged := filepath.Join(stagingDir, "officecli")
	url := officeCLIAssetURL(asset.name)

	SendLog(stepID, fmt.Sprintf("Downloading OfficeCLI %s for %s/%s...", officeCLIReleaseVersion, goos, goarch))
	download := system.RunWithLogs(fmt.Sprintf("curl -fsSL %q -o %q", url, staged), nil, func(line string) {
		SendLog(stepID, line)
	})
	if download.Error != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			fmt.Sprintf("Failed to download %s", url), download.Error)
	}

	if err := verifyFileSHA256(staged, asset.sha256); err != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			fmt.Sprintf("OfficeCLI checksum verification failed for %s", url), err)
	}

	// A destination that appeared while the download ran is preserved rather
	// than overwritten.
	if system.PathExists(dest) {
		SendLog(stepID, fmt.Sprintf("OfficeCLI appeared at %s while downloading; leaving it untouched", dest))
		return nil
	}

	if err := os.Chmod(staged, 0o755); err != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			"Failed to mark the downloaded binary executable", err)
	}
	if err := os.Rename(staged, dest); err != nil {
		return wrapStepError(stepID, "Install OfficeCLI",
			"Failed to install the verified binary", err)
	}

	SendLog(stepID, fmt.Sprintf("✓ OfficeCLI %s installed at %s", officeCLIReleaseVersion, dest))
	return nil
}
