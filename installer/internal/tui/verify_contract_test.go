package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVerifyInstallContractIsPinned holds the exported contract the new screen
// was built on top of.
//
// `VerifyInstall` used to walk the record itself; it now calls `InspectInstall`
// and renders what that returns, so the command line's `--verify` and the
// Utilities section's install check are two presenters of one walk. That is the
// reason to make the walk single, and it is also what puts an exported
// function's output at risk: a screen added for one user must not change the
// lines a script reads, or the exit status `cmd/dotfiles` derives from them.
//
// This guard is the record shapes themselves -- no record, an unreadable one, and
// one carrying a present path, a missing path, a present backup and a missing
// backup -- asserted through `VerifyInstall`, the exported function, rather than
// through `InspectInstall`, which is the seam a future refactor is free to move.
// An independent verifier walked ten record shapes through both implementations
// and found them byte-identical; this guard keeps the four that carry every
// branch the report has, so the next extraction cannot drift the contract in
// silence.
func TestVerifyInstallContractIsPinned(t *testing.T) {
	base, home := t.TempDir(), t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	t.Setenv("HOME", home)

	dir := filepath.Join(base, stateAppDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, lastInstallFileName)
	here := func(name string) string {
		p := filepath.Join(home, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// No record: nothing was installed here, and that is not a failure.
	r := VerifyInstall()
	if !r.Consistent || r.Unreadable || len(r.Lines) != 2 || !strings.Contains(r.Lines[1], "No record of an install") {
		t.Fatalf("the no-record report drifted: %#v", r)
	}

	// A record that cannot be parsed: unreadable, and reported as such rather
	// than as a machine with nothing missing.
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = VerifyInstall()
	if r.Consistent || !r.Unreadable || len(r.Lines) != 2 || !strings.Contains(r.Lines[1], "Could not read the record") {
		t.Fatalf("the unreadable report drifted: %#v", r)
	}

	// One entry per branch the report has, so a shape that stops being rendered
	// fails here rather than in a script that parses the output.
	rec := lastInstall{
		Timestamp: time.Date(2026, 10, 15, 12, 24, 0, 0, time.UTC),
		Version:   "v0.9.0",
		Entries: []installEntry{
			{Key: "present", Path: here("present"), Action: installActionReplaced},
			{Key: "gone", Path: filepath.Join(home, "gone"), Action: installActionCreated},
			{Key: "backed", Path: here("backed"), Action: installActionReplaced, Backup: here("backed.bak")},
			{Key: "lost", Path: here("lost"), Action: installActionReplaced, Backup: filepath.Join(home, "lost.bak")},
		},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	r = VerifyInstall()
	if r.Consistent || r.Unreadable {
		t.Fatalf("the mixed record's flags drifted: %#v", r)
	}
	flat := strings.Join(r.Lines, "\n")
	for _, want := range []string{"here     present", "missing  gone", "here     backup of backed", "missing  backup of lost"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the report lost %q:\n%s", want, flat)
		}
	}
}
