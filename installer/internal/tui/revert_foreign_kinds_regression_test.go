package tui

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The independent pass's own probe, kept because it pins the direction a fix like this one can
// break: a true alarm has to survive it. A nested link retargeted to a named pipe is still
// reported, and the report costs no read. Its semantics are the pass's, verbatim; the layout is
// gofmt's.
func TestANestedLinkRetargetedToAFifoIsReportedWithoutReadingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim, zsh := system.ConfigPaths()["nvim"], system.ConfigPaths()["zsh"]
	if nvim == "" || zsh == "" {
		t.Skip("no nvim or zsh path")
	}
	for _, d := range []string{nvim, zsh} {
		mustWrite(t, os.MkdirAll(d, 0o755))
	}
	mustWrite(t, os.Symlink(zsh, filepath.Join(nvim, "aaa")))
	pipe := filepath.Join(home, "zzz.pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	targetA := filepath.Join(home, "targetA")
	mustWrite(t, os.WriteFile(targetA, []byte("A"), 0o644))
	mustWrite(t, os.Symlink(pipe, filepath.Join(zsh, "lnk")))
	mustWrite(t, os.WriteFile(filepath.Join(zsh, "aaa.zsh"), []byte("two"), 0o644))
	nvB, zB := filepath.Join(home, ".dotfiles-backup-1", "nvim"), filepath.Join(home, ".dotfiles-backup-1", "zsh")
	for _, d := range []string{nvB, zB} {
		mustWrite(t, os.MkdirAll(d, 0o755))
	}
	mustWrite(t, os.WriteFile(filepath.Join(nvB, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.WriteFile(filepath.Join(zB, "aaa.zsh"), []byte("one"), 0o644))
	mustWrite(t, os.Symlink(targetA, filepath.Join(zB, "lnk")))
	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim, "zsh: " + zsh},
		Entries: []installEntry{
			{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: nvB},
			{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: zB},
		},
		Changes: []fileChange{
			{Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(nvB, "init.lua")),
				After:  fileDigest(filepath.Join(nvim, "init.lua")),
				Backup: filepath.Join(nvB, "init.lua")},
			{Key: "zsh", Path: filepath.Join(zsh, "aaa.zsh"), Action: fileActionModified,
				Before: fileDigest(filepath.Join(zB, "aaa.zsh")),
				After:  fileDigest(filepath.Join(zsh, "aaa.zsh")),
				Backup: filepath.Join(zB, "aaa.zsh")},
		},
	}))
	done := make(chan error, 1)
	go func() {
		_, err := RevertLastRun(RevertOptions{})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("a nested link retargeted to a fifo was not reported")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("revert blocked reading a fifo behind a nested link")
	}
}
