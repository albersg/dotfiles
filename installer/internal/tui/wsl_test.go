package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// newWSLLayout builds a throwaway repository checkout plus the two destinations
// the WSL step writes to, so the step can be exercised without touching the
// real Windows profile or /etc/wsl.conf.
func newWSLLayout(t *testing.T) (repoDir, winHome, wslConf string) {
	t.Helper()

	repoDir = t.TempDir()
	for asset, content := range map[string]string{
		repoAssetWSLConfig: "[wsl2]\nmemory=6GB\n",
		repoAssetWSLConf:   "[boot]\nsystemd=true\n",
	} {
		path := filepath.Join(repoDir, asset)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	winHome = filepath.Join(t.TempDir(), "winhome")
	if err := os.MkdirAll(winHome, 0o755); err != nil {
		t.Fatal(err)
	}
	wslConf = filepath.Join(t.TempDir(), "wsl.conf")

	t.Setenv(envWSLWindowsHome, winHome)
	t.Setenv(envWSLConfPath, wslConf)
	return repoDir, winHome, wslConf
}

func wslModel(repoDir string, isWSL bool) Model {
	m := NewModel()
	m.RepoDir = repoDir
	m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: isWSL, OSName: "WSL"}
	return m
}

func TestStepInstallWSLConfigSkipsNonWSLHosts(t *testing.T) {
	repoDir, winHome, _ := newWSLLayout(t)
	m := wslModel(repoDir, false)

	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("non-WSL host must be a no-op, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(winHome, ".wslconfig")); !os.IsNotExist(err) {
		t.Error("non-WSL host must not receive .wslconfig")
	}
}

func TestStepInstallWSLConfigInstallsBothArtifacts(t *testing.T) {
	repoDir, winHome, wslConf := newWSLLayout(t)
	m := wslModel(repoDir, true)

	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("step failed: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{".wslconfig", filepath.Join(winHome, ".wslconfig"), "[wsl2]\nmemory=6GB\n"},
		{"wsl.conf", wslConf, "[boot]\nsystemd=true\n"},
	} {
		got, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("%s was not installed at %s: %v", tc.name, tc.path, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s content = %q, want %q", tc.name, got, tc.want)
		}
		info, err := os.Stat(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("%s mode = %o, want 644", tc.name, info.Mode().Perm())
		}
	}
}

func TestStepInstallWSLConfigBacksUpExistingFiles(t *testing.T) {
	repoDir, winHome, wslConf := newWSLLayout(t)
	m := wslModel(repoDir, true)

	existing := filepath.Join(winHome, ".wslconfig")
	if err := os.WriteFile(existing, []byte("[wsl2]\nmemory=32GB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wslConf, []byte("[boot]\nsystemd=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("step failed: %v", err)
	}

	backups, err := filepath.Glob(existing + ".bak-dotfiles-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected one backup of the previous .wslconfig, got %v", backups)
	}
	saved, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != "[wsl2]\nmemory=32GB\n" {
		t.Errorf("backup content = %q, want the previous configuration", saved)
	}

	confBackups, err := filepath.Glob(wslConf + ".bak-dotfiles-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(confBackups) != 1 {
		t.Fatalf("expected one backup of the previous wsl.conf, got %v", confBackups)
	}
}

func TestStepInstallWSLConfigFailsWhenArtifactIsMissing(t *testing.T) {
	repoDir, _, _ := newWSLLayout(t)
	if err := os.Remove(filepath.Join(repoDir, repoAssetWSLConfig)); err != nil {
		t.Fatal(err)
	}
	m := wslModel(repoDir, true)

	err := stepInstallWSLConfig(&m)
	if err == nil {
		t.Fatal("expected an error when the repository artifact is missing")
	}
	if !strings.Contains(err.Error(), ".wslconfig") {
		t.Errorf("error should name the missing artifact, got: %v", err)
	}
}

func TestStepInstallWSLConfigFailsWithoutACheckout(t *testing.T) {
	m := wslModel("", true)
	if err := stepInstallWSLConfig(&m); err == nil {
		t.Fatal("expected an error when the repository was never cloned")
	}
}

func TestWindowsUserProfileHonoursOverride(t *testing.T) {
	winHome := t.TempDir()
	t.Setenv(envWSLWindowsHome, winHome)

	got, err := windowsUserProfile()
	if err != nil {
		t.Fatalf("override should be accepted: %v", err)
	}
	if got != winHome {
		t.Errorf("profile = %q, want %q", got, winHome)
	}

	t.Setenv(envWSLWindowsHome, filepath.Join(t.TempDir(), "does-not-exist"))
	if _, err := windowsUserProfile(); err == nil {
		t.Error("a non-existent override must be rejected")
	}
}

func TestWSLStepIsScheduledOnlyOnWSL(t *testing.T) {
	for _, tc := range []struct {
		name  string
		isWSL bool
		want  bool
	}{
		{"wsl host", true, true},
		{"plain linux host", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel()
			m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: tc.isWSL}
			m.Choices = UserChoices{OS: "linux", Shell: "zsh", Terminal: "none", WindowMgr: "none"}

			steps := buildStepsForChoices(&m)
			found := false
			for _, step := range steps {
				if step.ID == "wslconfig" {
					found = true
				}
			}
			if found != tc.want {
				t.Errorf("wslconfig step scheduled = %v, want %v", found, tc.want)
			}
		})
	}
}

func TestStepInstallWSLConfigSkipsWindowsSideWhenProfileUnavailable(t *testing.T) {
	repoDir, _, wslConf := newWSLLayout(t)
	// Without the override and without cmd.exe interop there is no Windows
	// profile to write to. The in-distribution half of the step must still run.
	t.Setenv(envWSLWindowsHome, "")

	m := wslModel(repoDir, true)
	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("a missing Windows profile must not fail the step: %v", err)
	}
	if _, err := os.Stat(wslConf); err != nil {
		t.Errorf("wsl.conf was not installed: %v", err)
	}
}

func TestApplyArtifactWritesWhenTheBackupFails(t *testing.T) {
	withPackageCommandMocks(t, errors.New("sudo refused"))

	dir := t.TempDir()
	dst := filepath.Join(dir, ".wslconfig")
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the backup destination unwritable so copyArtifact fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}

	src := filepath.Join(t.TempDir(), ".wslconfig")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applyArtifact(src, dst, wslStepID); err != nil {
		t.Fatalf("a failed backup must not abort the write: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("destination = %q, want %q", got, "new")
	}
}
