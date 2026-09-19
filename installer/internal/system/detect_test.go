package system

import (
	"os"
	"runtime"
	"testing"
)

func TestOSTypes(t *testing.T) {
	t.Run("OSFedora should be defined", func(t *testing.T) {
		// Verify OSFedora is a valid OSType
		var osType OSType = OSFedora
		if osType == OSUnknown {
			t.Error("OSFedora should not equal OSUnknown")
		}
	})

	t.Run("all OS types should be distinct", func(t *testing.T) {
		osTypes := []OSType{OSMac, OSLinux, OSArch, OSDebian, OSFedora, OSWSL, OSTermux, OSUnknown}
		seen := make(map[OSType]bool)
		for _, ot := range osTypes {
			if seen[ot] {
				t.Errorf("Duplicate OS type value found: %d", ot)
			}
			seen[ot] = true
		}
	})
}

func TestDetect(t *testing.T) {
	info := Detect()

	t.Run("should detect OS", func(t *testing.T) {
		if info.OS == OSUnknown && runtime.GOOS != "windows" {
			t.Error("OS should not be unknown on unix systems")
		}
	})

	t.Run("should have home directory", func(t *testing.T) {
		if info.HomeDir == "" {
			t.Error("HomeDir should not be empty")
		}
	})

	t.Run("should detect current shell", func(t *testing.T) {
		// Only test if SHELL env is set
		if os.Getenv("SHELL") != "" && info.UserShell == "unknown" {
			t.Error("UserShell should be detected when SHELL env is set")
		}
	})

	t.Run("OSName should match OS type", func(t *testing.T) {
		switch runtime.GOOS {
		case "darwin":
			if info.OSName != "macOS" {
				t.Errorf("Expected OSName to be 'macOS', got '%s'", info.OSName)
			}
		case "linux":
			validNames := []string{"Linux", "Arch Linux", "Debian/Ubuntu", "Fedora/RHEL", "WSL", "Termux"}
			found := false
			for _, name := range validNames {
				if info.OSName == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected OSName for Linux: '%s'", info.OSName)
			}
		}
	})

	t.Run("WSLVersion should be set when not on WSL", func(t *testing.T) {
		if !info.IsWSL {
			if info.WSLVersion != 0 {
				t.Errorf("WSLVersion should be 0 on non-WSL, got %d", info.WSLVersion)
			}
		}
	})
}

// TestClassifyLinuxKeepsDistributionUnderWSL pins the fix that stopped WSL from
// being modelled as an OS: the distribution underneath stays visible through OS
// while IsWSL keeps driving the WSL-specific steps, and an unrecognised WSL
// distribution still degrades to OSWSL rather than claiming to be a distro.
func TestClassifyLinuxKeepsDistributionUnderWSL(t *testing.T) {
	const wslKernel = "Linux version 5.15.90.1-microsoft-standard-WSL2 (gcc version 11.2.0) #1 SMP"

	t.Run("microsoft kernel reports WSL and the Debian-like distribution", func(t *testing.T) {
		t.Setenv("WSL_DISTRO_NAME", "")

		info := &SystemInfo{}
		classifyLinux(info, wslKernel, 2, distroSignals{debian: true})

		if !info.IsWSL {
			t.Fatal("IsWSL must stay true on a WSL kernel")
		}
		if info.OS != OSDebian {
			t.Errorf("OS = %v, want OSDebian", info.OS)
		}
		if info.OSName != "Debian/Ubuntu" {
			t.Errorf("OSName = %q, want %q", info.OSName, "Debian/Ubuntu")
		}
		if info.WSLVersion != 2 {
			t.Errorf("WSLVersion = %d, want 2", info.WSLVersion)
		}
	})

	t.Run("unrecognised WSL distribution falls back to OSWSL", func(t *testing.T) {
		t.Setenv("WSL_DISTRO_NAME", "")

		info := &SystemInfo{}
		classifyLinux(info, wslKernel, 2, distroSignals{})

		if !info.IsWSL {
			t.Fatal("IsWSL must stay true on a WSL kernel")
		}
		if info.OS != OSWSL {
			t.Errorf("OS = %v, want OSWSL for an unknown WSL distribution", info.OS)
		}
		if info.OSName != "WSL" {
			t.Errorf("OSName = %q, want %q", info.OSName, "WSL")
		}
	})

	t.Run("non-WSL kernel is unchanged for every platform", func(t *testing.T) {
		t.Setenv("WSL_DISTRO_NAME", "")

		const plainKernel = "Linux version 6.8.0-45-generic (buildd@lcy02-amd64-013) #45-Ubuntu SMP"

		cases := []struct {
			name     string
			signals  distroSignals
			wantOS   OSType
			wantName string
		}{
			{"arch", distroSignals{arch: true}, OSArch, "Arch Linux"},
			{"fedora", distroSignals{fedora: true}, OSFedora, "Fedora/RHEL"},
			{"debian", distroSignals{debian: true}, OSDebian, "Debian/Ubuntu"},
			{"plain linux", distroSignals{}, OSLinux, "Linux"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				info := &SystemInfo{}
				// A WSL version is passed to prove it is ignored off WSL.
				classifyLinux(info, plainKernel, 2, tc.signals)

				if info.IsWSL {
					t.Error("IsWSL must be false on a non-WSL kernel")
				}
				if info.WSLVersion != 0 {
					t.Errorf("WSLVersion = %d, want 0 on non-WSL", info.WSLVersion)
				}
				if info.OS != tc.wantOS {
					t.Errorf("OS = %v, want %v", info.OS, tc.wantOS)
				}
				if info.OSName != tc.wantName {
					t.Errorf("OSName = %q, want %q", info.OSName, tc.wantName)
				}
			})
		}
	})
}

func TestCommandExists(t *testing.T) {
	t.Run("should find common commands", func(t *testing.T) {
		// These should exist on any unix system
		commonCmds := []string{"ls", "echo", "cat"}
		for _, cmd := range commonCmds {
			if !CommandExists(cmd) {
				t.Errorf("Command '%s' should exist", cmd)
			}
		}
	})

	t.Run("should not find non-existent commands", func(t *testing.T) {
		if CommandExists("this-command-definitely-does-not-exist-xyz123") {
			t.Error("Should not find non-existent command")
		}
	})
}

func TestGetBrewPrefix(t *testing.T) {
	prefix := GetBrewPrefix()

	t.Run("should return valid prefix based on OS and arch", func(t *testing.T) {
		switch runtime.GOOS {
		case "darwin":
			if runtime.GOARCH == "arm64" {
				if prefix != "/opt/homebrew" {
					t.Errorf("Expected '/opt/homebrew' on macOS ARM64, got '%s'", prefix)
				}
			} else {
				if prefix != "/usr/local" {
					t.Errorf("Expected '/usr/local' on macOS Intel, got '%s'", prefix)
				}
			}
		case "linux":
			if prefix != "/home/linuxbrew/.linuxbrew" {
				t.Errorf("Expected '/home/linuxbrew/.linuxbrew' on Linux, got '%s'", prefix)
			}
		}
	})
}

func TestCheckWSL(t *testing.T) {
	// This test just ensures the function doesn't panic
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("checkWSL panicked: %v", r)
			}
		}()
		_ = checkWSL()
	})
}

func TestDetectWSLVersion(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("detectWSLVersion panicked: %v", r)
			}
		}()
		version := detectWSLVersion()
		// Should return 0, 1, or 2
		if version < 0 || version > 2 {
			t.Errorf("Unexpected WSL version: %d (expected 0-2)", version)
		}
	})

	t.Run("should return valid value", func(t *testing.T) {
		version := detectWSLVersion()
		t.Logf("Detected WSL version: %d", version)
	})
}

func TestIsArchLinux(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isArchLinux panicked: %v", r)
			}
		}()
		_ = isArchLinux()
	})
}

func TestIsDebian(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isDebian panicked: %v", r)
			}
		}()
		_ = isDebian()
	})
}

func TestIsFedora(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isFedora panicked: %v", r)
			}
		}()
		_ = isFedora()
	})

	t.Run("should return bool", func(t *testing.T) {
		result := isFedora()
		// Just verify it returns a boolean without panicking
		if result {
			t.Log("Running on Fedora/RHEL system")
		} else {
			t.Log("Not running on Fedora/RHEL system")
		}
	})
}

func TestIsTermux(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isTermux panicked: %v", r)
			}
		}()
		_ = isTermux()
	})

	t.Run("should detect TERMUX_VERSION env", func(t *testing.T) {
		// Save original value
		original := os.Getenv("TERMUX_VERSION")
		defer os.Setenv("TERMUX_VERSION", original)

		// Set Termux env
		os.Setenv("TERMUX_VERSION", "0.118.0")
		if !isTermux() {
			t.Error("Should detect Termux when TERMUX_VERSION is set")
		}

		// Unset
		os.Unsetenv("TERMUX_VERSION")
		// Note: might still be true if PREFIX contains termux
	})

	t.Run("should detect PREFIX with termux path", func(t *testing.T) {
		// Save original values
		originalVersion := os.Getenv("TERMUX_VERSION")
		originalPrefix := os.Getenv("PREFIX")
		defer func() {
			os.Setenv("TERMUX_VERSION", originalVersion)
			os.Setenv("PREFIX", originalPrefix)
		}()

		// Clear TERMUX_VERSION, set PREFIX
		os.Unsetenv("TERMUX_VERSION")
		os.Setenv("PREFIX", "/data/data/com.termux/files/usr")

		if !isTermux() {
			t.Error("Should detect Termux when PREFIX contains 'com.termux'")
		}
	})
}

func TestCheckPkg(t *testing.T) {
	t.Run("should not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("checkPkg panicked: %v", r)
			}
		}()
		_ = checkPkg()
	})
}

func TestDetectTermuxFields(t *testing.T) {
	t.Run("SystemInfo should have Termux fields", func(t *testing.T) {
		info := Detect()
		// Just verify the fields exist and are initialized
		_ = info.IsTermux
		_ = info.HasPkg
		_ = info.Prefix
	})

	t.Run("Non-Termux system should have IsTermux=false", func(t *testing.T) {
		// Skip this test if we're actually running in Termux
		// (the directory /data/data/com.termux will exist regardless of env vars)
		if _, err := os.Stat("/data/data/com.termux"); err == nil {
			t.Skip("Skipping test: running in actual Termux environment")
		}

		// Save original values
		originalVersion := os.Getenv("TERMUX_VERSION")
		originalPrefix := os.Getenv("PREFIX")
		defer func() {
			if originalVersion != "" {
				os.Setenv("TERMUX_VERSION", originalVersion)
			}
			if originalPrefix != "" {
				os.Setenv("PREFIX", originalPrefix)
			}
		}()

		// Clear Termux env vars
		os.Unsetenv("TERMUX_VERSION")
		os.Setenv("PREFIX", "/usr/local") // Non-termux prefix

		// On non-Termux systems, IsTermux should be false
		info := Detect()
		if info.IsTermux {
			t.Error("IsTermux should be false on non-Termux systems")
		}
	})
}

// Helper to check if string contains termux
func containsTermux(s string) bool {
	return len(s) > 0 && (s == "/data/data/com.termux/files/usr" ||
		(len(s) > 10 && s[:10] == "/data/data"))
}
