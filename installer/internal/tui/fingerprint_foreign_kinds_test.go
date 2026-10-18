package tui

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// bounded runs something that can block forever and fails the test instead of hanging it. The
// installer reads a path to learn what it holds; a named pipe has no answer and never gives one,
// so every read of a path whose kind the installer cannot have made is a hole of this shape.
func bounded(t *testing.T, what string, run func()) {
	t.Helper()
	done := make(chan struct{}, 1)
	go func() {
		run()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not finish within 5s: something is reading a path it cannot have made", what)
	}
}

// A key that holds a named pipe: the fingerprint walks the tree and reads every non-directory it
// meets, which is how a pipe under a configuration path hangs the installer before any run starts.
func TestFingerprintingAKeyThatHoldsANamedPipeDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(nvim, "agent.pipe"), 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	bounded(t, "fingerprinting a key that holds a pipe", func() { fingerprintPath(nvim) })
}

// A key that is itself a named pipe: the same read, reached without walking anything.
func TestFingerprintingAKeyThatIsANamedPipeDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(filepath.Dir(nvim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(nvim, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	bounded(t, "fingerprinting a key that is a pipe", func() { fingerprintPath(nvim) })
}

// A key that is a link to a named pipe: the fingerprint resolves the link and reads what it finds,
// and the file index digests a key that is not a directory. Both follow the link to the pipe.
func TestFingerprintingAKeyThatIsALinkToANamedPipeDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(filepath.Dir(nvim), 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(home, "outside.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.Symlink(pipe, nvim); err != nil {
		t.Fatal(err)
	}
	bounded(t, "fingerprinting a key that is a link to a pipe", func() { fingerprintPath(nvim) })
}

func TestIndexingAKeyThatIsALinkToANamedPipeDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(filepath.Dir(nvim), 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(home, "outside.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.Symlink(pipe, nvim); err != nil {
		t.Fatal(err)
	}
	bounded(t, "indexing a key that is a link to a pipe", func() { configFileIndex() })
}

// The two routes the independent pass over the revert round reported as advisories: the run's own
// copy is read to compare content, and the change loop reads the path the record names. Neither can
// come from a run, and both block. They are the same hole in the same helper.
func TestARevertDoesNotBlockOnAPipeThatStandsInForItsOwnCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(backup, "init.lua"), 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes: []fileChange{{
			Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
			After:  fileDigest(filepath.Join(nvim, "init.lua")),
			Backup: filepath.Join(backup, "init.lua"),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	var report RevertReport
	bounded(t, "reverting a file whose copy is a pipe", func() {
		report, _ = RevertLastRun(RevertOptions{})
	})
	if report.Complete() {
		t.Fatalf("a copy that cannot be read was reported as put back: %+v", report.Files)
	}
}

func TestARevertDoesNotBlockOnAPipeTheRecordNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(nvim, "agent.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes: []fileChange{{
			Key: "nvim", Path: pipe, Action: fileActionModified,
			Backup: filepath.Join(backup, "agent.pipe"),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	var report RevertReport
	bounded(t, "reverting a record that names a pipe", func() {
		report, _ = RevertLastRun(RevertOptions{})
	})
	if report.Complete() {
		t.Fatalf("a path the record names and nothing can put back was reported as complete: %+v", report.Files)
	}
}
