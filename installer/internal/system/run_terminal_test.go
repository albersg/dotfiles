package system

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRunTerminalGivesTheCommandTheTerminalAndCapturesNothing pins what the
// runner is for and what it must not do.
//
// A command that has to ask the person something -- sudo reading a password --
// needs the terminal, so this runner attaches the process's own streams and
// deliberately captures nothing: the output belongs to the terminal it was
// written on. The capturing runners beside it hand the child pipes instead, which
// is exactly why a prompt run through them would be written into a buffer nobody
// reads.
func TestRunTerminalGivesTheCommandTheTerminalAndCapturesNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the runner spells its command through the platform's shell")
	}

	result := RunTerminal("printf attached-to-the-terminal", nil)
	if result.Error != nil {
		t.Fatalf("RunTerminal failed: %v", result.Error)
	}
	if result.Output != "" || result.Stderr != "" {
		t.Fatalf("RunTerminal captured output (%q / %q); the command's output belongs to the terminal", result.Output, result.Stderr)
	}
	if result.Command != "printf attached-to-the-terminal" {
		t.Fatalf("result.Command = %q, want the command it was given", result.Command)
	}
}

// TestRunTerminalReportsTheCommandsFailure pins the other half: a command that
// refuses is reported with its exit code instead of being treated as success, so
// a caller can tell a cached credential from a refused one.
func TestRunTerminalReportsTheCommandsFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the runner spells its command through the platform's shell")
	}

	result := RunTerminal("exit 3", nil)
	if result.Error == nil {
		t.Fatal("a command that exited non-zero was reported as success")
	}
	if result.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", result.ExitCode)
	}
}

// TestRunTerminalKeepsTheCallersDirectory pins that the options the other runners
// honour are not silently dropped here: the work directory reaches the child.
func TestRunTerminalKeepsTheCallersDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the runner spells its command through the platform's shell")
	}

	dir := t.TempDir()
	result := RunTerminal("pwd > "+filepath.Join(dir, "where")+" ; exit 0", &ExecOptions{WorkDir: dir})
	if result.Error != nil {
		t.Fatalf("RunTerminal failed: %v", result.Error)
	}

	where, err := os.ReadFile(filepath.Join(dir, "where"))
	if err != nil {
		t.Fatalf("the command did not run in the caller's directory: %v", err)
	}
	got, gotErr := filepath.EvalSymlinks(strings.TrimSpace(string(where)))
	want, wantErr := filepath.EvalSymlinks(dir)
	if gotErr != nil || wantErr != nil {
		t.Fatalf("resolving the directories: %v / %v", gotErr, wantErr)
	}
	if got != want {
		t.Fatalf("the command ran in %q, want %q", got, want)
	}
}
