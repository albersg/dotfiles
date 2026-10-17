package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A record says two things about a run: which paths it wrote (an entry per key, and
// the Files list) and, file by file, what changed inside them. Only the second half
// carries a before-state, and the index behind it steps over anything that is not a
// regular file -- so a directory can be named as written while the thing that
// changed inside it is a symlink the record never lists.
//
// This is the ordinary case, written by the run's own writer and not by hand: a
// configuration directory whose plugin directory is a link, with a file inside it
// edited. A revert that acts on the file changes and stays silent about the rest
// reports a machine as back while that link still points where the run left it.
func TestARevertDoesNotCallItselfCompleteWithALinkTheRecordDoesNotName(t *testing.T) {
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
	initLua := filepath.Join(nvim, "init.lua")
	plugin := filepath.Join(nvim, "plugin")
	if err := os.WriteFile(initLua, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("targets/a", plugin); err != nil {
		t.Fatal(err)
	}

	// The baseline is taken here, as a run takes it before its first write.
	// The run takes its usual copy of the key before writing anything.
	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	copyOfNvim := filepath.Join(backupDir, "nvim")
	if err := os.MkdirAll(copyOfNvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyOfNvim, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("targets/a", filepath.Join(copyOfNvim, "plugin")); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	// The run edits a file the index sees and retargets a link it does not.
	if err := os.WriteFile(initLua, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(plugin); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("targets/b", plugin); err != nil {
		t.Fatal(err)
	}
	m.recordRun(runStatusComplete, nil)

	record := readLastInstall()
	if record == nil || len(record.Changes) == 0 {
		t.Fatalf("this test needs a record with per-file changes, got %+v", record)
	}
	wroteKey := false
	for i := range record.Entries {
		if record.Entries[i].Key == "nvim" && record.Entries[i].Action == installActionReplaced {
			wroteKey = true
			record.Entries[i].Backup = copyOfNvim
		}
	}
	if !wroteKey {
		t.Fatalf("the record does not say the run wrote the nvim path; entries: %+v", record.Entries)
	}
	nameCopies(t, record, nvim, copyOfNvim)
	if err := writeLastInstall(*record); err != nil {
		t.Fatal(err)
	}

	report, err := RevertLastRun(RevertOptions{})
	if err == nil && report.Complete() {
		target, _ := os.Readlink(plugin)
		t.Fatalf("the revert reported the machine back while the link the record does not name still points at %q", target)
	}
	if err == nil {
		t.Fatalf("the revert answered nothing while %s is a path it cannot put back", plugin)
	}
	if !strings.Contains(err.Error(), plugin) {
		t.Errorf("the answer does not name the path it cannot account for: %v", err)
	}
	if !strings.Contains(err.Error(), backupDir) {
		t.Errorf("the answer does not name the backup that can put the whole entry back: %v", err)
	}
}

// The other side of the same rule: a link the run left alone is evidence, not a
// reason for alarm. The run's backup holds the before-state, so when the link on
// the machine matches the copy, nothing is missing and the revert is complete --
// reporting it would turn every configuration directory that holds an untouched
// link into a false alarm.
func TestALinkTheRunLeftAloneDoesNotStopTheRevertFromBeingComplete(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		copied    string // what the run's backup holds for the link
		onMachine string // where the link points now
		complete  bool
	}{
		{name: "left alone", copied: "targets/a", onMachine: "targets/a", complete: true},
		{name: "retargeted", copied: "targets/a", onMachine: "targets/b", complete: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
			initLua := filepath.Join(nvim, "init.lua")
			plugin := filepath.Join(nvim, "plugin")
			if err := os.WriteFile(initLua, []byte("one"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("targets/a", plugin); err != nil {
				t.Fatal(err)
			}

			backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
			copyOfNvim := filepath.Join(backupDir, "nvim")
			if err := os.MkdirAll(copyOfNvim, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(copyOfNvim, "init.lua"), []byte("one"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(testCase.copied, filepath.Join(copyOfNvim, "plugin")); err != nil {
				t.Fatal(err)
			}

			m := NewModel()
			m.ExistingConfigs = []string{"nvim: " + nvim}
			m.ConfigFingerprints = configFingerprints()
			m.ConfigFileIndex = configFileIndex()

			if err := os.WriteFile(initLua, []byte("two"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(plugin); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(testCase.onMachine, plugin); err != nil {
				t.Fatal(err)
			}
			m.recordRun(runStatusComplete, nil)

			// The run's backup holds the copies, as a run that took one leaves them.
			record := readLastInstall()
			if record == nil {
				t.Fatal("the run wrote no record")
			}
			for i := range record.Entries {
				record.Entries[i].Backup = copyOfNvim
			}
			nameCopies(t, record, nvim, copyOfNvim)
			if err := writeLastInstall(*record); err != nil {
				t.Fatal(err)
			}

			report, err := RevertLastRun(RevertOptions{})
			if got := err == nil; got != testCase.complete {
				t.Errorf("the revert answering nothing = %v, want %v (err %v, report %+v)", got, testCase.complete, err, report.Files)
			}
			if got := report.Complete(); got != (err == nil) {
				t.Errorf("the report says complete = %v while the answer was %v: the two have to agree", got, err)
			}
		})
	}
}

// nameCopies points every change under a key at the copy the run's backup holds of
// that file, the way resolveChanges does: the copy is what the revert reads to put
// the file back, and a change without one cannot be acted on for that reason alone.
func nameCopies(t *testing.T, record *lastInstall, keyPath, copyOfKey string) {
	t.Helper()
	for i := range record.Changes {
		rel, err := filepath.Rel(keyPath, record.Changes[i].Path)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		record.Changes[i].Backup = filepath.Join(copyOfKey, rel)
	}
}

// A link inside a written key whose target still matches the run's copy is only half
// the question: what it reaches can be somewhere the record never walked. A link into
// a checkout, a shared directory or a cache is a place this run could have written
// with nothing on the other side of the record -- reporting the machine back there
// claims knowledge the record does not have, and saying nothing is the same false
// success in a quieter voice.
func TestALinkIntoATreeTheRecordDoesNotCoverIsReportedNotAssumed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	reach := filepath.Join(home, "checkout", "lua")
	if err := os.MkdirAll(reach, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reach, "plugin.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	lua := filepath.Join(nvim, "lua")
	if err := os.Symlink(reach, lua); err != nil {
		t.Fatal(err)
	}
	initLua := filepath.Join(nvim, "init.lua")
	if err := os.WriteFile(initLua, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	copyOfNvim := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000", "nvim")
	if err := os.MkdirAll(copyOfNvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyOfNvim, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(reach, filepath.Join(copyOfNvim, "lua")); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	if err := os.WriteFile(initLua, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.recordRun(runStatusComplete, nil)

	record := readLastInstall()
	if record == nil {
		t.Fatal("the run wrote no record")
	}
	for i := range record.Entries {
		if record.Entries[i].Key == "nvim" {
			record.Entries[i].Backup = copyOfNvim
		}
	}
	nameCopies(t, record, nvim, copyOfNvim)
	if err := writeLastInstall(*record); err != nil {
		t.Fatal(err)
	}

	// The link is exactly where the run found it; what it reaches is not.
	_, err := RevertLastRun(RevertOptions{})
	if err == nil {
		t.Fatalf("%s reaches %s, a tree the record covers nowhere, and the revert answered nothing", lua, reach)
	}
	if !strings.Contains(err.Error(), lua) || !strings.Contains(err.Error(), reach) {
		t.Errorf("the answer names neither the link nor where it reaches: %v", err)
	}
}
