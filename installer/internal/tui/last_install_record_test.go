package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The last-install record's guards. The record is the file the next run and the
// "Last install" panel read back as the state of the machine, so it has to name
// what the run did, not what the run found: issue #239 is a record built from
// the pre-run detection, which counts an untouched config exactly like one the
// run overwrote.
//
// The guard here drives real steps in a temporary $HOME and compares the record
// against the bytes on disk. It never reaches the network, never reads the real
// $HOME and never installs anything: the command runners the steps use are
// replaced with fakes.

// runFiles is the ground truth the guard compares the record against: the bytes
// of the configuration paths before the run. It answers "did the run write
// this?" from those bytes rather than from modification times, so it is
// independent of the classifier the installer uses and cannot pass merely by
// agreeing with it.
type runFiles struct {
	before map[string]string
}

func snapshotRunFiles(t *testing.T, paths ...string) runFiles {
	t.Helper()

	rf := runFiles{before: map[string]string{}}
	for _, path := range paths {
		rf.recordInto(rf.before, path)
	}
	return rf
}

// recordInto stores the bytes of a file, or of every regular file under a
// directory, so a directory is compared entry by entry. A missing path records
// nothing: its absence is what makes a later write a creation.
func (rf runFiles) recordInto(into map[string]string, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		if content, err := os.ReadFile(path); err == nil {
			into[path] = string(content)
		}
		return
	}
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if content, err := os.ReadFile(p); err == nil {
			into[p] = string(content)
		}
		return nil
	})
}

// changed reports whether anything at path was written after the snapshot: a
// file that did not exist then, or whose bytes differ now. A directory is
// changed when any file inside it is, which is how the installer writes a
// configuration directory, file by file.
func (rf runFiles) changed(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		content, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		was, ok := rf.before[path]
		return !ok || was != string(content)
	}

	changed := false
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if was, ok := rf.before[p]; !ok || was != string(content) {
			changed = true
			return fs.SkipAll
		}
		return nil
	})
	return changed
}

// lastInstallProblems reports every claim the record makes that the filesystem
// contradicts. changed answers whether the run wrote a path; the caller supplies
// it from ground truth taken before the run, so the guard has teeth: fed a record
// that claims a replacement that did not happen, it names the claim instead of
// agreeing with the record.
func lastInstallProblems(rec lastInstall, changed func(string) bool) []string {
	var problems []string

	for _, entry := range rec.Entries {
		switch entry.Action {
		case installActionReplaced:
			if !pathExists(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was replaced, but "+entry.Path+" does not exist")
			}
			if !changed(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was replaced, but nothing at "+entry.Path+" was written during the run")
			}
			if entry.Backup == "" {
				problems = append(problems, "the record says "+entry.Key+" was replaced, but names no backup copy")
			} else if !pathExists(entry.Backup) {
				problems = append(problems, "the record says "+entry.Key+" was replaced, but the backup it names is not there: "+entry.Backup)
			}
		case installActionBackedUp:
			if entry.Backup == "" || !pathExists(entry.Backup) {
				problems = append(problems, "the record says "+entry.Key+" was backed up, but "+entry.Backup+" is not there")
			}
			if changed(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was only backed up, but the run also wrote it")
			}
		case installActionCreated:
			if !pathExists(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was created, but "+entry.Path+" does not exist")
			}
			if entry.Backup != "" {
				problems = append(problems, "the record says "+entry.Key+" was created, but names a backup: "+entry.Backup)
			}
			if !changed(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was created, but nothing at "+entry.Path+" was written during the run")
			}
		case installActionFound:
			if !pathExists(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was found, but "+entry.Path+" does not exist")
			}
			if changed(entry.Path) {
				problems = append(problems, "the record says "+entry.Key+" was only found, but the run wrote it")
			}
		default:
			problems = append(problems, "the record gives "+entry.Key+" the unknown action "+string(entry.Action))
		}
	}

	return problems
}

func entryFor(rec lastInstall, key string) (installEntry, bool) {
	for _, entry := range rec.Entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return installEntry{}, false
}

// writeConfigFile writes a file a guard seeds, creating its directory.
func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// withNoNetworkFnm replaces the fnm runner the shell step reaches when fnm is on
// the machine, so a guard cannot start an fnm download. The step itself is not
// under test here; the shell step's file writes are.
func withNoNetworkFnm(t *testing.T) {
	t.Helper()

	original := runFnmWithLogs
	runFnmWithLogs = func(command string, _ *system.ExecOptions, _ system.LogCallback) *system.ExecResult {
		return &system.ExecResult{Command: command}
	}
	t.Cleanup(func() { runFnmWithLogs = original })
}

// TestTheLastInstallRecordNamesWhatTheRunDid drives the backup, shell and
// window-manager steps against a temporary $HOME and checks the record written
// at the end against the bytes on disk. A config the run replaced has to read as
// replaced, a config it created has to read as created, and a config it merely
// found must not read as touched at all.
func TestTheLastInstallRecordNamesWhatTheRunDid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "")
	withZshMocks(t, home)
	withNoNetworkFnm(t)

	replacedPath := filepath.Join(home, ".zshenv")
	untouchedPath := filepath.Join(home, ".config", "kitty", "kitty.conf")
	writeConfigFile(t, replacedPath, "# the user's own .zshenv\nexport USER_MARK=1\n")
	writeConfigFile(t, untouchedPath, "# the user's own kitty config\n")

	before := snapshotRunFiles(t, replacedPath, untouchedPath)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)
	// The detection drives the backup step and tells the record what existed
	// before the run.
	m.ExistingConfigs = system.DetectExistingConfigs()

	// Drive the steps through executeStep, which is the entry point the run
	// itself uses: it is what takes the before-run fingerprint the record is built
	// from, so a guard that called the step functions directly would not cover the
	// path the installer walks.
	for _, stepID := range []string{"backup", "shell", "wm"} {
		if err := executeStep(stepID, &m); err != nil {
			t.Fatalf("the %s step failed: %v", stepID, err)
		}
	}

	m.recordRun(runStatusComplete, nil)

	rec := readLastInstall()
	if rec == nil {
		t.Fatalf("the run wrote no record at %s", lastInstallPath())
	}

	// Every claim the record makes has to hold against the bytes on disk.
	for _, problem := range lastInstallProblems(*rec, before.changed) {
		t.Error(problem)
	}

	// The two seeded paths pin the defect from both sides: the replaced one
	// reads as replaced, the untouched one does not read as touched.
	for _, tc := range []struct {
		key    string
		path   string
		action installAction
	}{
		{"zshenv", replacedPath, installActionReplaced},
		{"kitty", filepath.Join(home, ".config", "kitty"), installActionBackedUp},
		{"herdr", filepath.Join(home, ".config", "herdr", "config.toml"), installActionCreated},
	} {
		entry, ok := entryFor(*rec, tc.key)
		if !ok {
			t.Errorf("the record says nothing about %s (%s)", tc.key, tc.path)
			continue
		}
		if entry.Action != tc.action {
			t.Errorf("%s: action = %q, want %q", tc.key, entry.Action, tc.action)
		}
	}

	// The legacy touched list is what the panel counts, so it must hold only the
	// paths the run wrote.
	if slices.Contains(rec.Files, "kitty: "+filepath.Join(home, ".config", "kitty")) {
		t.Errorf("the record counts the untouched kitty config as touched: %v", rec.Files)
	}
	if !slices.Contains(rec.Files, "zshenv: "+replacedPath) {
		t.Errorf("the record does not count the replaced .zshenv as touched: %v", rec.Files)
	}
}

// TestTheLastInstallGuardFailsWhenAReplacementDidNotHappen pins the guard's
// teeth. The defect in issue #239 was a record that named the paths the run
// found as if it had replaced them, so the guard is fed exactly that claim and
// must name it rather than accept it.
func TestTheLastInstallGuardFailsWhenAReplacementDidNotHappen(t *testing.T) {
	home := t.TempDir()
	kittyDir := filepath.Join(home, ".config", "kitty")
	untouched := filepath.Join(kittyDir, "kitty.conf")
	writeConfigFile(t, untouched, "# the user's own kitty config\n")

	before := snapshotRunFiles(t, kittyDir)

	// A record that claims a detected path was replaced even though nothing
	// changed it, which is the claim issue #239 reported; the named backup is
	// deliberately not there, so the guard has two contradictions to name.
	defect := lastInstall{
		Timestamp: time.Now(),
		Version:   "v0.6.4",
		Files:     []string{"kitty: " + kittyDir},
		Entries: []installEntry{{
			Key:    "kitty",
			Path:   kittyDir,
			Action: installActionReplaced,
			Backup: filepath.Join(home, ".dotfiles-backup-2026-10-10-143854", "kitty"),
		}},
	}

	problems := lastInstallProblems(defect, before.changed)
	if len(problems) == 0 {
		t.Fatal("the guard accepted a record claiming a replacement that did not happen")
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "kitty") {
		t.Errorf("the guard does not name the false claim:\n%s", joined)
	}
	if !strings.Contains(joined, "was written during the run") {
		t.Errorf("the guard does not say why the replacement did not happen:\n%s", joined)
	}
}

// TestAnOldLastInstallRecordStillLoads pins backward compatibility. A record
// written before the outcomes existed carries only the legacy paths, and the
// reader has to load it without crashing and report its paths as found: an old
// build copied the detection, so it never established a replacement and the
// reader must not invent one.
func TestAnOldLastInstallRecordStillLoads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	path := lastInstallPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"timestamp":"2026-10-10T14:38:54Z","version":"v0.6.3","files":["nvim: /home/u/.config/nvim","fish: /home/u/.config/fish"]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := readLastInstall()
	if rec == nil {
		t.Fatal("an old record did not load")
	}
	if len(rec.Entries) != 2 {
		t.Fatalf("the old record loaded %d entries, want 2: %+v", len(rec.Entries), rec.Entries)
	}
	for _, entry := range rec.Entries {
		if entry.Action != installActionFound {
			t.Errorf("old entry %q read as %q, want found: the old build cannot say it replaced it", entry.Key, entry.Action)
		}
	}
}
