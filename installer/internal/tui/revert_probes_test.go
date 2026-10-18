package tui

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A link inside a written key can reach a configuration key that the record names as
// one the run did NOT write. Reporting it is a false alarm of my own making: the
// record's own word is that the run left that key alone, so what the link reaches is
// not a place this run wrote, and the machine is fine.
func TestALinkIntoAKeyTheRecordSaysTheRunDidNotWriteIsNotAFalseAlarm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim, fish := system.ConfigPaths()["nvim"], system.ConfigPaths()["fish"]
	if nvim == "" || fish == "" {
		t.Skip("this machine has no nvim or fish configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	mustWrite(t, os.MkdirAll(fish, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(fish, "config.fish"), []byte("untouched"), 0o644))
	mustWrite(t, os.Symlink(fish, filepath.Join(nvim, "fishlink")))
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("after"), 0o644))

	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("before"), 0o644))
	mustWrite(t, os.Symlink(fish, filepath.Join(backup, "fishlink")))

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries: []installEntry{
			{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup},
			{Key: "fish", Path: fish, Action: installActionFound},
		},
		Changes: []fileChange{{
			Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
			Before: fileDigest(filepath.Join(backup, "init.lua")),
			After:  fileDigest(filepath.Join(nvim, "init.lua")),
			Backup: filepath.Join(backup, "init.lua"),
		}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("false alarm: %s is a link into %s, a key this record says the run did not write; got %v (%+v)",
			filepath.Join(nvim, "fishlink"), fish, err, report.Files)
	}
	if data, _ := os.ReadFile(filepath.Join(nvim, "init.lua")); string(data) != "before" {
		t.Fatalf("the change the record does name was not reverted: init.lua holds %q", data)
	}
}

// A change naming the entry's own directory is not evidence about what is inside that
// directory. Accepting it as coverage stops the walk before it starts, and the revert
// then reports the machine back with a link inside the tree still exactly where the
// run left it.
func TestAChangeNamingTheEntryDirectoryCannotHideWhatIsInsideIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	outside := filepath.Join(home, "outside")
	mustWrite(t, os.WriteFile(outside, []byte("outside"), 0o644))
	mustWrite(t, os.Symlink(outside, filepath.Join(nvim, "lua")))

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionCreated}},
		Changes:   []fileChange{{Key: "nvim", Path: nvim, Action: fileActionCreated}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err == nil && report.Complete() {
		if _, statErr := os.Lstat(filepath.Join(nvim, "lua")); statErr == nil {
			t.Fatalf("false success: the change names the entry directory %s, and the link inside it that nothing records survived a complete revert", nvim)
		}
	}
}

// The record is built by the run's own writer, and the key this run writes through is
// a link to a directory: the file the run changed lives on the other side of it. If the
// writer cannot see through that link, it records a key it found and no change at all,
// and the revert has nothing to act on -- exit 0, on an ordinary layout, with every
// file the run wrote still holding what it left.
func TestARunThatWroteThroughASymlinkedKeyIsNotReportedBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	real := filepath.Join(home, "repo", "nvim")
	mustWrite(t, os.MkdirAll(real, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(real, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.MkdirAll(filepath.Dir(nvim), 0o755))
	mustWrite(t, os.Symlink(real, nvim))

	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	mustWrite(t, os.WriteFile(filepath.Join(real, "init.lua"), []byte("two"), 0o644))
	m.recordRun(runStatusComplete, nil)

	record := readLastInstall()
	named := false
	if record != nil {
		for _, change := range record.Changes {
			if change.Path == filepath.Join(nvim, "init.lua") || change.Path == filepath.Join(real, "init.lua") {
				named = true
			}
		}
	}
	if !named {
		t.Errorf("the record says the run changed nothing inside a key it wrote through a link: %+v", record)
	}

	report, err := RevertLastRun(RevertOptions{})
	got, _ := os.ReadFile(filepath.Join(real, "init.lua"))
	if err == nil && report.Complete() && string(got) == "two" {
		t.Fatalf("false success: the revert reports the machine back while the file the run wrote through %s still holds %q", nvim, got)
	}
}

// A path the record says the run created, and that is not there any more, is a path the
// machine is already back on. Reporting it as one that cannot be put back is an alarm
// about work that is already done.
func TestACreatedPathAlreadyGoneIsNotUnrevertible(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if zsh == "" {
		t.Skip("this machine has no zsh configuration path")
	}
	_ = os.Remove(zsh)

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"zsh: " + zsh},
		Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionCreated}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("the path the run created is gone, so the machine is already back; got %v (%+v)", err, report.Files)
	}
}

// "Inside a path the record wrote" is not on its own evidence that the record lists
// what is there: a record can name a tree and omit a file inside it. The run's own copy
// of that tree is the before-state, so it is what settles the question -- a file that
// matches its copy was not changed by the run, and one that does not, with no change
// naming it, is a file the record cannot account for.
func TestAReachInsideAWrittenKeyIsVouchedForByItsFiles(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		onDisk   string // what the file inside the reached tree holds now
		complete bool
	}{
		{name: "unchanged, matching its copy", onDisk: "one", complete: true},
		{name: "changed, named by no change", onDisk: "two", complete: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

			nvim := system.ConfigPaths()["nvim"]
			if nvim == "" {
				t.Skip("this machine has no nvim configuration path")
			}
			mustWrite(t, os.MkdirAll(filepath.Join(nvim, "real"), 0o755))
			mustWrite(t, os.WriteFile(filepath.Join(nvim, "real", "foo.lua"), []byte(testCase.onDisk), 0o644))
			mustWrite(t, os.Symlink(filepath.Join(nvim, "real"), filepath.Join(nvim, "link")))
			mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644))

			backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
			mustWrite(t, os.MkdirAll(filepath.Join(backup, "real"), 0o755))
			mustWrite(t, os.WriteFile(filepath.Join(backup, "real", "foo.lua"), []byte("one"), 0o644))
			mustWrite(t, os.Symlink(filepath.Join(nvim, "real"), filepath.Join(backup, "link")))
			mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644))

			mustWrite(t, writeLastInstall(lastInstall{
				Timestamp: time.Now(),
				Files:     []string{"nvim: " + nvim},
				Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
				Changes: []fileChange{{
					Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
					Before: fileDigest(filepath.Join(backup, "init.lua")),
					After:  fileDigest(filepath.Join(nvim, "init.lua")),
					Backup: filepath.Join(backup, "init.lua"),
				}},
			}))

			report, err := RevertLastRun(RevertOptions{})
			if got := err == nil; got != testCase.complete {
				t.Fatalf("answered nothing = %v, want %v (err %v, report %+v)", got, testCase.complete, err, report.Files)
			}
		})
	}
}

// A named pipe inside a key the run wrote is a kind the per-file record cannot carry, the
// copier cannot copy and this installer never makes: it writes files and links, and nothing
// in the installer calls mkfifo, mkfifo-like syscalls or a listener on a path. So the pipe
// is not the run's doing, and the revert, whose job is to put the run's changes back, must
// keep quiet about it. The eleventh pass reported this shape as an advisory; the round that
// fixed the links with no copy fixed its shape, not its kinds.
func TestANamedPipeInsideAWrittenKeyIsNotTheRunsDoing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	agent := filepath.Join(nvim, "agent.pipe")
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	if err := syscall.Mkfifo(agent, 0o600); err != nil {
		t.Skipf("this system cannot make a named pipe: %v", err)
	}
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644))

	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644))

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes: []fileChange{{
			Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
			Before: fileDigest(filepath.Join(backup, "init.lua")),
			After:  fileDigest(filepath.Join(nvim, "init.lua")),
			Backup: filepath.Join(backup, "init.lua"),
		}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("a named pipe the run never made failed the revert: %v (%+v)", err, report.Files)
	}
	if _, statErr := os.Lstat(agent); statErr != nil {
		t.Fatalf("the revert removed the pipe: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(nvim, "init.lua")); statErr != nil {
		t.Fatalf("the file the run modified is gone: %v", statErr)
	}
}

func mustWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
