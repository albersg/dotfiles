package system

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type OSType int

const (
	OSMac OSType = iota
	OSLinux
	OSArch
	OSDebian    // Debian-based (Debian, Ubuntu, etc.)
	OSFedora    // Fedora/RHEL-based (Fedora, CentOS, RHEL, etc.)
	OSTermux    // Termux on Android
	OSWSL       // WSL (Windows Subsystem for Linux)
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
		info.OS = OSLinux
		info.OSName = "Linux"
		info.IsWSL = checkWSL()
		if info.IsWSL {
			info.OS = OSWSL
			info.OSName = "WSL"
			info.WSLVersion = detectWSLVersion()
			break // Skip distro checks — WSL is the primary OS type
		}

		if isArchLinux() {
			info.OS = OSArch
			info.OSName = "Arch Linux"
		} else if isFedora() {
			info.OS = OSFedora
			info.OSName = "Fedora/RHEL"
		} else if isDebian() {
			info.OS = OSDebian
			info.OSName = "Debian/Ubuntu"
		}
	}

	info.HasBrew = checkBrew()
	info.UserShell = detectCurrentShell()

	return info
}

func checkWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	if strings.Contains(content, "microsoft") || strings.Contains(content, "wsl") {
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
	_, err := exec.LookPath("brew")
	return err == nil
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
