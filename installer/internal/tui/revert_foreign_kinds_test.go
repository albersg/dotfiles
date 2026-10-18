package tui

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The record is written by hand in these probes on purpose: configFingerprints reads every
// file under a key to fingerprint it, and reading a named pipe blocks. That is a finding of
// this round about state.go, not about the rule under test here, and a probe must measure
// its own subject.

// A named pipe inside a key the run wrote, with no copy of the key at all: the shape the
// ninth pass fixed for links. The installer never makes one, so its presence is not the
// run's doing, and a revert that put every change back must not fail over it.
func TestANamedPipeInsideAWrittenKeyWithNoCopyIsNotAFalseAlarm(t *testing.T) {
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
	created := filepath.Join(nvim, "new.lua")
	if err := os.WriteFile(created, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionCreated}},
		Changes: []fileChange{{
			Key: "nvim", Path: created, Action: fileActionCreated,
			After: fileDigest(created),
		}},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("a named pipe the run never made failed the revert: %v (%+v)", err, report.Files)
	}
	if _, statErr := os.Lstat(pipe); statErr != nil {
		t.Fatalf("the revert removed the pipe: %v", statErr)
	}
	if _, statErr := os.Lstat(created); !os.IsNotExist(statErr) {
		t.Fatalf("the file the run created is still here: %v", statErr)
	}
}

// The same kind reached behind a link is walked by the content check, and reading a named
// pipe blocks until a writer appears. The revert of a run that touched nothing there has to
// finish; the bound lives inside the test so a regression fails it instead of hanging it.
func TestANamedPipeBehindALinkIsNotReadAndDoesNotBlockTheRevert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	paths := system.ConfigPaths()
	nvim, zsh := paths["nvim"], paths["zsh"]
	if nvim == "" || zsh == "" {
		t.Skip("this machine has no nvim or zsh configuration path")
	}
	for _, dir := range []string{nvim, zsh} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The link is named before the pipe is, so the walk reaches it first and the content
	// check is the route that meets the pipe.
	if err := os.Symlink(zsh, filepath.Join(nvim, "aaa")); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(zsh, "zzz.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zsh, "aaa.zsh"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	nvimBackup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	zshBackup := filepath.Join(home, ".dotfiles-backup-1", "zsh")
	for _, dir := range []string{nvimBackup, zshBackup} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zshBackup, "aaa.zsh"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim, "zsh: " + zsh},
		Entries: []installEntry{
			{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: nvimBackup},
			{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: zshBackup},
		},
		Changes: []fileChange{
			{Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(nvimBackup, "init.lua")),
				After:  fileDigest(filepath.Join(nvim, "init.lua")),
				Backup: filepath.Join(nvimBackup, "init.lua")},
			{Key: "zsh", Path: filepath.Join(zsh, "aaa.zsh"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(zshBackup, "aaa.zsh")),
				After:  fileDigest(filepath.Join(zsh, "aaa.zsh")),
				Backup: filepath.Join(zshBackup, "aaa.zsh")},
		},
	}); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := RevertLastRun(RevertOptions{})
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("the revert did not finish within 5s: it is blocked reading a named pipe it cannot have made")
	}
}

// A link to a named pipe puts the pipe behind a name the read follows: the content check
// must not follow it either, for the same reason and with the same bound.
func TestALinkToANamedPipeIsNotReadAndDoesNotBlockTheRevert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	paths := system.ConfigPaths()
	nvim, zsh := paths["nvim"], paths["zsh"]
	if nvim == "" || zsh == "" {
		t.Skip("this machine has no nvim or zsh configuration path")
	}
	for _, dir := range []string{nvim, zsh} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(zsh, filepath.Join(nvim, "aaa")); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(home, "outside.pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	if err := os.Symlink(fifo, filepath.Join(zsh, "zzz-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zsh, "aaa.zsh"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	nvimBackup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	zshBackup := filepath.Join(home, ".dotfiles-backup-1", "zsh")
	for _, dir := range []string{nvimBackup, zshBackup} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zshBackup, "aaa.zsh"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim, "zsh: " + zsh},
		Entries: []installEntry{
			{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: nvimBackup},
			{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: zshBackup},
		},
		Changes: []fileChange{
			{Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(nvimBackup, "init.lua")),
				After:  fileDigest(filepath.Join(nvim, "init.lua")),
				Backup: filepath.Join(nvimBackup, "init.lua")},
			{Key: "zsh", Path: filepath.Join(zsh, "aaa.zsh"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(zshBackup, "aaa.zsh")),
				After:  fileDigest(filepath.Join(zsh, "aaa.zsh")),
				Backup: filepath.Join(zshBackup, "aaa.zsh")},
		},
	}); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := RevertLastRun(RevertOptions{})
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("the revert did not finish within 5s: it is blocked following a link into a named pipe")
	}
}
