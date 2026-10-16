package sshcfg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// SHARED TEST HELPERS
// ============================================================================
//
// The tests in this package never touch the machine they run on. Every path is
// under t.TempDir(), every command goes to a runner that records it instead of
// running it, and no test opens a real key, a real agent or a real
// configuration file. These helpers are what makes that possible for more than
// one test file.

// equalStrings reports whether two argument lists are the same, element by
// element. Comparing argument lists as strings would hide the case a test cares
// about most: an argument that arrived as part of a longer string.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// testRunner is a Runner that answers from a script instead of running
// anything. A call with no matching handler is a failure: a test that did not
// expect a command should not have one run behind its back.
type testRunner struct {
	// calls is every command the code under test ran, in order.
	calls []Command
	// handler answers one command. want decides which commands it answers.
	handler func(cmd Command) (Result, bool)
	// fallback answers a command no handler claimed, and the test decides
	// whether that is acceptable.
	fallback func(cmd Command) Result
}

func (r *testRunner) Run(_ context.Context, cmd Command) Result {
	r.calls = append(r.calls, cmd)
	if r.handler != nil {
		if res, ok := r.handler(cmd); ok {
			return res
		}
	}
	if r.fallback != nil {
		return r.fallback(cmd)
	}
	return Result{Code: 127, Err: os.ErrNotExist, Stderr: "no such command in this test"}
}

// testManager is a Manager whose paths point into a temporary directory and
// whose runner is a script. It is what every test in this package works with.
type testManager struct {
	manager *Manager
	runner  *testRunner
	home    string
}

// testManagerSetup builds a manager over a fresh empty home. The capabilities
// are set to a plausible OpenSSH install unless the test overrides them: a test
// about permissions should not silently also be a test about a missing client.
func testManagerSetup(t *testing.T) *testManager {
	t.Helper()
	home := t.TempDir()
	paths := PathsFrom(home, func(key string) string { return "" })
	runner := &testRunner{}
	m := &Manager{Paths: paths, Run: runner}
	m.Caps = Caps{
		SSH:        "/usr/bin/ssh",
		Keygen:     "/usr/bin/ssh-keygen",
		Add:        "/usr/bin/ssh-add",
		SSHVersion: "OpenSSH_10.0p2 Debian-7",
		// The types a recent ssh-keygen advertises, so a test about an unsupported
		// type is refused before anything runs rather than by the script's silence.
		KeygenSupports: []string{"ssh-ed25519", "ecdsa-sha2-nistp256", "sk-ssh-ed25519@openssh.com", "ssh-rsa"},
	}
	return &testManager{manager: m, runner: runner, home: home}
}

// mkdir creates a directory inside the test's home.
func (e *testManager) mkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// write creates a file inside the test's home.
func (e *testManager) write(t *testing.T, path string, mode os.FileMode, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// read reads a file the test wrote.
func (e *testManager) read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// okResult is a successful command result.
func okResult(stdout string) Result { return Result{Stdout: stdout} }

// failResult is a failed command result with a code and a message.
func failResult(code int, stderr string) Result {
	return Result{Code: code, Stderr: stderr}
}

// containsAll reports whether every wanted fragment appears in text.
func containsAll(text string, wanted ...string) bool {
	for _, want := range wanted {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}
