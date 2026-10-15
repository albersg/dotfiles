package tui

import (
	"os"
	"testing"
)

// TestTheStepScriptIsReadableOnlyByItsOwner pins the mode of the script a
// root-needing step is written to.
//
// Every interactive step is written to a temporary file and run by handing that
// path to a shell, and the file is created under os.TempDir() -- on Linux the
// shared, sticky, world-listable /tmp. The file lives for the whole of its step,
// which for the toolset step is minutes, and its contents name the exact package
// commands the run is about to execute.
//
// Two directions, because either one alone is satisfied by a wrong answer:
//
//   - No bit outside the owner's. 0755 passes a test that only asks whether the
//     owner can read it, and 0600 passes a test that only asks whether the group
//     and others are excluded.
//   - The owner's read bit set. The script is not executed by its own path: it is
//     handed to a shell as an argument (`<shell> <path>`), so the shell reads the
//     file. A mode that took the read bit away would stop every root-needing step
//     from running at all.
//
// The execute bit is deliberately not asserted in either direction. Nothing in
// the run consults it, so requiring it would pin a choice rather than the
// property that matters; forbidding it would be equally arbitrary.
func TestTheStepScriptIsReadableOnlyByItsOwner(t *testing.T) {
	path, err := writeStepScript("#!/bin/sh\necho dotfiles step script\n")
	if err != nil {
		t.Fatalf("writeStepScript: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	mode := info.Mode().Perm()

	if outside := mode & 0o077; outside != 0 {
		t.Errorf("the step script is written %04o, so any account on the host can read it: every account can list %s, and the file names the package commands the run is about to execute",
			mode, os.TempDir())
	}
	if mode&0o400 == 0 {
		t.Errorf("the step script is written %04o, so the account running it cannot read its own script: the shell is handed the path and reads the file, so the step would not run",
			mode)
	}
}
