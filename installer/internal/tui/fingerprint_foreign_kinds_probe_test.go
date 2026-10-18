package tui

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

func runBounded(t *testing.T, what string, run func()) {
	t.Helper()
	done := make(chan struct{}, 1)
	go func() { run(); done <- struct{}{} }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatalf("%s blocked", what)
	}
}

func probeHome(t *testing.T) (string, map[string]string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	return home, system.ConfigPaths()
}

// C1: a link to a regular file is still read and digested.
func TestProbeSymlinkToRegularFileIsDigested(t *testing.T) {
	home, paths := probeHome(t)
	real := filepath.Join(home, "real.lua")
	if err := os.WriteFile(real, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	nvim := paths["nvim"]
	if err := os.MkdirAll(filepath.Dir(nvim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, nvim); err != nil {
		t.Fatal(err)
	}
	idx := configFileIndex()
	if idx[nvim] == "" || idx[nvim] != fileDigest(real) {
		t.Fatalf("index lost a link to a regular file: %v", idx)
	}
	if fingerprintPath(nvim) == "" {
		t.Fatal("fingerprint empty for a link to a regular file")
	}
}

// C1: a regular file and a directory keep their digests.
func TestProbeRegularFileAndDirectoryKeysStay(t *testing.T) {
	_, paths := probeHome(t)
	if err := os.WriteFile(paths["zsh"], []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths["nvim"], 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(paths["nvim"], "init.lua")
	if err := os.WriteFile(inner, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := configFileIndex()
	if idx[paths["zsh"]] == "" || idx[inner] == "" {
		t.Fatalf("index lost a regular file: %v", idx)
	}
	if fingerprintPath(paths["zsh"]) == "" || fingerprintPath(paths["nvim"]) == "" {
		t.Fatal("fingerprint empty for a regular file or directory key")
	}
}

// C2: a pipe behind two links, seen by all three entry points.
func TestProbePipeBehindTwoLinks(t *testing.T) {
	home, paths := probeHome(t)
	if err := os.MkdirAll(filepath.Dir(paths["nvim"]), 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(home, "p")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skip(err)
	}
	l2 := filepath.Join(home, "l2")
	if err := os.Symlink(pipe, l2); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(l2, paths["nvim"]); err != nil {
		t.Fatal(err)
	}
	runBounded(t, "fingerprintPath two links", func() { fingerprintPath(paths["nvim"]) })
	runBounded(t, "configFileIndex two links", func() { configFileIndex() })
	runBounded(t, "configFingerprints two links", func() { configFingerprints() })
}

// C2: a pipe inside a tree reached through a key that is a link to a directory.
func TestProbePipeInsideLinkedKeyDir(t *testing.T) {
	home, paths := probeHome(t)
	real := filepath.Join(home, "realnvim")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(real, "agent.pipe"), 0o600); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(real, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths["nvim"]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, paths["nvim"]); err != nil {
		t.Fatal(err)
	}
	runBounded(t, "fingerprintPath linked dir key", func() { fingerprintPath(paths["nvim"]) })
	runBounded(t, "configFileIndex linked dir key", func() { configFileIndex() })
}

// C2: a socket key returns; it never blocked, but it pins the kind rule.
func TestProbeSocketKeyReturns(t *testing.T) {
	_, paths := probeHome(t)
	if err := os.MkdirAll(filepath.Dir(paths["nvim"]), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", paths["nvim"])
	if err != nil {
		t.Skipf("no unix socket: %v", err)
	}
	defer ln.Close()
	runBounded(t, "fingerprintPath socket", func() { fingerprintPath(paths["nvim"]) })
	runBounded(t, "configFileIndex socket", func() { configFileIndex() })
}

// C2: a link to a device is not read.
func TestProbeDeviceSymlinkReturns(t *testing.T) {
	_, paths := probeHome(t)
	if err := os.MkdirAll(filepath.Dir(paths["nvim"]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/urandom", paths["nvim"]); err != nil {
		t.Skip(err)
	}
	runBounded(t, "fingerprintPath device", func() { fingerprintPath(paths["nvim"]) })
	runBounded(t, "configFileIndex device", func() { configFileIndex() })
}

// C4: the change loop must not read a pipe a record names (non-empty Before).
func TestProbeRevertRecordNamesPipeReachesNoRead(t *testing.T) {
	home, paths := probeHome(t)
	nvim := paths["nvim"]
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(nvim, "agent.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skip(err)
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
		Changes:   []fileChange{{Key: "nvim", Path: pipe, Action: fileActionModified, Before: "deadbeef", Backup: filepath.Join(backup, "agent.pipe")}},
	}); err != nil {
		t.Fatal(err)
	}
	runBounded(t, "revert record names a pipe with Before", func() { _, _ = RevertLastRun(RevertOptions{DryRun: true}) })
}

// C4: the record's own copy must not be read through fileDigest(copied).
func TestProbeRevertOwnCopyPipeReachesNoRead(t *testing.T) {
	home, paths := probeHome(t)
	nvim := paths["nvim"]
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "real.lua"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "other.lua"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.lua", filepath.Join(nvim, "link")); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.lua", filepath.Join(backup, "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(backup, "real.lua"), 0o600); err != nil {
		t.Skip(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes:   []fileChange{{Key: "nvim", Path: filepath.Join(nvim, "other.lua"), Action: fileActionModified, Before: "bb", After: "cc", Backup: filepath.Join(backup, "other.lua")}},
	}); err != nil {
		t.Fatal(err)
	}
	runBounded(t, "revert own copy is a pipe", func() { _, _ = RevertLastRun(RevertOptions{DryRun: true}) })
}
