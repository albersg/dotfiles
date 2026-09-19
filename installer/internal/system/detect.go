package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type OSType int

const (
	OSMac OSType = iota
	OSLinux
	OSArch
	OSDebian // Debian-based (Debian, Ubuntu, etc.)
	OSFedora // Fedora/RHEL-based (Fedora, CentOS, RHEL, etc.)
	OSTermux // Termux on Android
	OSWSL    // WSL (Windows Subsystem for Linux)
	OSUnknown
)

type SystemInfo struct {
	OS         OSType
	OSName     string
	IsWSL      bool
	WSLVersion int // 0=none, 1=WSL1, 2=WSL2
	IsARM      bool
	IsTermux   bool
	HomeDir    string
	HasBrew    bool
	HasPkg     bool // Termux package manager
	HasXcode   bool
	UserShell  string
	Prefix     string // Termux $PREFIX or empty for other systems
}

func Detect() *SystemInfo {
	info := &SystemInfo{
		OS:      OSUnknown,
		OSName:  "Unknown",
		HomeDir: os.Getenv("HOME"),
		IsARM:   runtime.GOARCH == "arm64" || runtime.GOARCH == "arm",
		Prefix:  os.Getenv("PREFIX"),
	}

	// Check for Termux FIRST (it runs on Linux but is special)
	if isTermux() {
		info.OS = OSTermux
		info.OSName = "Termux"
		info.IsTermux = true
		info.HasPkg = checkPkg()
		info.HasBrew = false // Termux doesn't use Homebrew
		info.UserShell = detectCurrentShell()
		return info
	}

	switch runtime.GOOS {
	case "darwin":
		info.OS = OSMac
		info.OSName = "macOS"
		info.HasXcode = checkXcode()
	case "linux":
		classifyLinux(info, readKernelVersion(), detectWSLVersion(), detectDistroSignals())
	}

	info.HasBrew = checkBrew()
	info.UserShell = detectCurrentShell()

	return info
}

// classifyLinux fills in the OS fields for a Linux host.
//
// WSL is a hosting environment, not a distribution: underneath it there is
// always a real distribution. Modelling it as an OS made every dispatch that
// reads OS blind on WSL, so detection identifies the distribution on WSL too,
// keeps IsWSL set, and uses OSWSL only when no distribution is recognised. An
// unknown WSL environment therefore degrades exactly as it did before instead of
// newly claiming to be a distribution.
//
// kernelVersion, wslVersion and signals are parameters rather than direct reads
// of /proc and /etc so the WSL/distribution interaction can be exercised in a
// test with a fixed kernel string. Detect passes the real values.
func classifyLinux(info *SystemInfo, kernelVersion string, wslVersion int, signals distroSignals) {
	info.IsWSL = isWSLKernel(kernelVersion) || os.Getenv("WSL_DISTRO_NAME") != ""
	if info.IsWSL {
		info.WSLVersion = wslVersion
	}

	if distroOS, distroName := signals.osType(); distroOS != OSUnknown {
		info.OS = distroOS
		info.OSName = distroName
	} else if info.IsWSL {
		info.OS = OSWSL
		info.OSName = "WSL"
	} else {
		info.OS = OSLinux
		info.OSName = "Linux"
	}
}

// readKernelVersion returns the contents of /proc/version, or an empty string
// when it cannot be read (non-Linux hosts, restricted /proc).
func readKernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return ""
	}
	return string(data)
}

// isWSLKernel reports whether a /proc/version string identifies a WSL kernel.
func isWSLKernel(kernelVersion string) bool {
	content := strings.ToLower(kernelVersion)
	return strings.Contains(content, "microsoft") || strings.Contains(content, "wsl")
}

// distroSignals records which distribution families are present on the host. It
// is a value rather than three direct file checks so detection can be tested
// without depending on the host's /etc/*-release files.
type distroSignals struct {
	arch   bool
	fedora bool
	debian bool
}

func detectDistroSignals() distroSignals {
	return distroSignals{
		arch:   isArchLinux(),
		fedora: isFedora(),
		debian: isDebian(),
	}
}

// osType maps the signals to the OSType and display name, in the same order the
// original detection used.
func (s distroSignals) osType() (OSType, string) {
	switch {
	case s.arch:
		return OSArch, "Arch Linux"
	case s.fedora:
		return OSFedora, "Fedora/RHEL"
	case s.debian:
		return OSDebian, "Debian/Ubuntu"
	default:
		return OSUnknown, ""
	}
}

func checkWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	if isWSLKernel(string(data)) {
		return true
	}
	// Secondary check: WSL_DISTRO_NAME env var is set by WSL init
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	return false
}

// detectWSLVersion detects WSL 1 or 2 from /proc/sys/kernel/osrelease
// WSL2 uses a kernel version >= 5.x (actually 5.10+)
func detectWSLVersion() int {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		// Can't detect, assume WSL2 (current default)
		return 2
	}
	content := strings.TrimSpace(string(data))
	// WSL2 reports kernel like "5.10.16.3-microsoft-standard-WSL2"
	if strings.Contains(strings.ToLower(content), "wsl2") {
		return 2
	}
	// WSL1: older kernel, typically 4.x without WSL2 marker
	if strings.Contains(strings.ToLower(content), "microsoft") {
		return 1
	}
	// Fallback: assume WSL2 if we got here (WSL1 is rare now)
	return 2
}

func isArchLinux() bool {
	_, err := os.Stat("/etc/arch-release")
	return err == nil
}

func isDebian() bool {
	_, err := os.Stat("/etc/debian_version")
	return err == nil
}

func isFedora() bool {
	// Check for Fedora specifically
	if _, err := os.Stat("/etc/fedora-release"); err == nil {
		return true
	}
	// Check for RHEL/CentOS (also use dnf)
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		return true
	}
	return false
}

// isTermux detects if we're running in Termux on Android
func isTermux() bool {
	// Check TERMUX_VERSION environment variable
	if os.Getenv("TERMUX_VERSION") != "" {
		return true
	}
	// Check PREFIX contains termux path
	prefix := os.Getenv("PREFIX")
	if strings.Contains(prefix, "com.termux") {
		return true
	}
	// Check for Termux-specific paths
	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		return true
	}
	return false
}

// checkPkg checks if Termux pkg command is available
func checkPkg() bool {
	_, err := exec.LookPath("pkg")
	return err == nil
}

func checkBrew() bool {
	return BrewInstalled()
}

// BrewInstalled reports whether Homebrew is available.
//
// It checks PATH first and then the well-known installation prefixes. The
// prefix check matters right after the installer installs Homebrew itself: the
// install script's `brew shellenv` runs in a child process, so the parent PATH
// is never refreshed and LookPath alone would keep reporting false for the rest
// of the run.
func BrewInstalled() bool {
	if _, err := exec.LookPath("brew"); err == nil {
		return true
	}

	brewPrefix := os.Getenv("HOMEBREW_PREFIX")
	if brewPrefix == "" {
		brewPrefix = GetBrewPrefix()
	}
	info, err := os.Stat(filepath.Join(brewPrefix, "bin", "brew"))
	return err == nil && !info.IsDir()
}

func checkXcode() bool {
	cmd := exec.Command("xcode-select", "-p")
	return cmd.Run() == nil
}

func detectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}

// CommandExists checks if a command is available in PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// GetBrewPrefix returns the homebrew prefix path
func GetBrewPrefix() string {
	if runtime.GOOS == "darwin" {
		// Apple Silicon (arm64) uses /opt/homebrew
		// Intel (amd64) uses /usr/local
		if runtime.GOARCH == "arm64" {
			return "/opt/homebrew"
		}
		return "/usr/local"
	}
	return "/home/linuxbrew/.linuxbrew"
}
