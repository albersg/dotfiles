package sshcfg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diagnoseTestRunner is the test double for Runner used by the diagnosis
// tests. It records every invocation and answers every `ssh -G` from a canned
// result. It starts no process, so no test opens a real connection and none of
// them touches the developer's own configuration.
type diagnoseTestRunner struct {
	calls []Command
	sshG  Result
}

func (r *diagnoseTestRunner) Run(_ context.Context, cmd Command) Result {
	r.calls = append(r.calls, cmd)
	return r.sshG
}

// diagnoseTestEnv is the fixed environment every case starts from: a temporary
// home, a system config path inside it and the three tool paths stubbed.
type diagnoseTestEnv struct {
	home string
	m    *Manager
	run  *diagnoseTestRunner
}

func diagnoseTestSetup(t *testing.T) *diagnoseTestEnv {
	t.Helper()
	home := t.TempDir()
	run := &diagnoseTestRunner{}
	m := &Manager{
		Paths: PathsFrom(home, func(string) string { return "" }),
		Run:   run,
		Caps:  Caps{SSH: "/usr/bin/ssh", Keygen: "/usr/bin/ssh-keygen", Add: "/usr/bin/ssh-add"},
	}
	m.Paths.SystemConfig = filepath.Join(home, "system_ssh_config")
	return &diagnoseTestEnv{home: home, m: m, run: run}
}

func diagnoseTestMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func diagnoseTestWrite(t *testing.T, path string, mode os.FileMode, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	// WriteFile applies the umask, so the mode is set again explicitly to make
	// the case mean what it says.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func diagnoseTestCodes(got []Finding) []string {
	codes := make([]string, 0, len(got))
	for _, f := range got {
		codes = append(codes, f.Code)
	}
	return codes
}

func diagnoseTestLookup(got []Finding, code string) *Finding {
	for i := range got {
		if got[i].Code == code {
			return &got[i]
		}
	}
	return nil
}

func diagnoseTestFindingsWithCode(got []Finding, code string) []Finding {
	var out []Finding
	for _, f := range got {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// diagnoseTestAssertCodes asserts the whole report in order, so a test fails
// when a finding appears, disappears or moves.
func diagnoseTestAssertCodes(t *testing.T, got []Finding, want ...string) {
	t.Helper()
	if lens, lenw := len(got), len(want); lens != lenw {
		t.Fatalf("report has %d findings %v, want %d %v", lens, diagnoseTestCodes(got), lenw, want)
	}
	for i := range want {
		if got[i].Code != want[i] {
			t.Fatalf("finding %d is %q, want %q (report %v)", i, got[i].Code, want[i], diagnoseTestCodes(got))
		}
	}
}

// diagnoseTestAssertOrdered asserts the stable order the diagnosis promises:
// problems before warnings before information, and by Code inside one severity.
func diagnoseTestAssertOrdered(t *testing.T, got []Finding) {
	t.Helper()
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if cur.Severity > prev.Severity {
			t.Fatalf("finding %q (%s) sorts before %q (%s): severity is not descending", prev.Code, prev.Severity, cur.Code, cur.Severity)
		}
		if cur.Severity == prev.Severity && cur.Code < prev.Code {
			t.Fatalf("finding %q sorts after %q inside severity %s: Code is not ascending", cur.Code, prev.Code, cur.Severity)
		}
	}
}

// TestDiagnose covers every finding the diagnosis produces. Each case builds a
// temporary home, calls Diagnose once and asserts the entire report in order.
func TestDiagnose(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, e *diagnoseTestEnv)
		check func(t *testing.T, e *diagnoseTestEnv, got []Finding)
	}{
		{
			name:  "missing ssh directory",
			setup: func(*testing.T, *diagnoseTestEnv) {},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "no-ssh-directory")
				if f == nil {
					t.Fatal("no-ssh-directory not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if f.File != "" {
					t.Errorf("File = %q, want empty", f.File)
				}
				if f.Fix != "" {
					t.Errorf("Fix = %q, want empty", f.Fix)
				}
				if !strings.Contains(f.Detail, "no") || !strings.Contains(f.Detail, "0700") {
					t.Errorf("Detail %q does not explain the directory and its permissions", f.Detail)
				}
			},
		},
		{
			name: "0700 directory is clean",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "ssh-dir-permissions"); f != nil {
					t.Errorf("0700 directory produced %+v", f)
				}
				if f := diagnoseTestLookup(got, "ssh-dir-readable-by-others"); f != nil {
					t.Errorf("0700 directory produced %+v", f)
				}
			},
		},
		{
			name: "0777 directory is a problem",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o777)
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "ssh-dir-permissions")
				if f == nil {
					t.Fatal("ssh-dir-permissions not found")
				}
				if f.Severity != SeverityProblem {
					t.Errorf("severity = %s, want problem", f.Severity)
				}
				if f.File != e.m.Paths.SSHDir {
					t.Errorf("File = %q, want %q", f.File, e.m.Paths.SSHDir)
				}
				if !strings.Contains(f.Detail, "0777") {
					t.Errorf("Detail %q does not name the mode", f.Detail)
				}
				if f.Fix != "chmod 700 ~/.ssh" {
					t.Errorf("Fix = %q, want the chmod line", f.Fix)
				}
			},
		},
		{
			name: "0755 directory is readable by others",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o755)
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "ssh-dir-readable-by-others")
				if f == nil {
					t.Fatal("ssh-dir-readable-by-others not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if !strings.Contains(f.Detail, "0755") {
					t.Errorf("Detail %q does not name the mode", f.Detail)
				}
				if f.Fix != "chmod 700 ~/.ssh" {
					t.Errorf("Fix = %q, want the chmod line", f.Fix)
				}
				if f := diagnoseTestLookup(got, "ssh-dir-permissions"); f != nil {
					t.Errorf("read-only exposure produced the write problem: %+v", f)
				}
			},
		},
		{
			name: "0644 private key is too open",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "id_rsa"), 0o644, "not a real key\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				key := filepath.Join(e.m.Paths.SSHDir, "id_rsa")
				f := diagnoseTestLookup(got, "private-key-permissions")
				if f == nil {
					t.Fatal("private-key-permissions not found")
				}
				if f.Severity != SeverityProblem {
					t.Errorf("severity = %s, want problem", f.Severity)
				}
				if f.File != key {
					t.Errorf("File = %q, want %q", f.File, key)
				}
				if !strings.Contains(f.Detail, "0644") {
					t.Errorf("Detail %q does not name the mode", f.Detail)
				}
				if f.Fix != "chmod 600 "+key {
					t.Errorf("Fix = %q, want chmod 600 %s", f.Fix, key)
				}
			},
		},
		{
			name: "0600 private key is fine",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "id_ed25519"), 0o600, "not a real key\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "private-key-permissions"); f != nil {
					t.Errorf("0600 key produced %+v", f)
				}
			},
		},
		{
			name: "public half alone is not a private key",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "only.pub"), 0o644, "ssh-ed25519 AAAA test\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "private-key-permissions"); f != nil {
					t.Errorf("public half alone produced %+v", f)
				}
			},
		},
		{
			name: "unconventional name with a public sibling is a private key",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "deploy"), 0o644, "not a real key\n")
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "deploy.pub"), 0o644, "ssh-ed25519 AAAA test\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				key := filepath.Join(e.m.Paths.SSHDir, "deploy")
				f := diagnoseTestLookup(got, "private-key-permissions")
				if f == nil {
					t.Fatal("private-key-permissions not found for a key with a .pub sibling")
				}
				if f.File != key {
					t.Errorf("File = %q, want %q", f.File, key)
				}
			},
		},
		{
			name: "world-writable config",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o666, "Host a\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "config-permissions")
				if f == nil {
					t.Fatal("config-permissions not found")
				}
				if f.Severity != SeverityProblem {
					t.Errorf("severity = %s, want problem", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig {
					t.Errorf("File = %q, want %q", f.File, e.m.Paths.UserConfig)
				}
				if !strings.Contains(f.Detail, "0666") {
					t.Errorf("Detail %q does not name the mode", f.Detail)
				}
				if f.Fix != "chmod 600 "+e.m.Paths.UserConfig {
					t.Errorf("Fix = %q, want the chmod line", f.Fix)
				}
			},
		},
		{
			name: "config with a syntax error",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "this is not a directive\n")
				e.run.sshG = Result{Code: 1, Stderr: e.m.Paths.UserConfig + ": line 1: Bad configuration option: this\nsecond line\n"}
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "config-syntax")
				if f == nil {
					t.Fatal("config-syntax not found")
				}
				if f.Severity != SeverityProblem {
					t.Errorf("severity = %s, want problem", f.Severity)
				}
				wantDetail := e.m.Paths.UserConfig + ": line 1: Bad configuration option: this"
				if f.Detail != wantDetail {
					t.Errorf("Detail = %q, want the client's first stderr line %q", f.Detail, wantDetail)
				}
				if f.File != e.m.Paths.UserConfig {
					t.Errorf("File = %q, want %q", f.File, e.m.Paths.UserConfig)
				}
				if len(e.run.calls) != 1 {
					t.Fatalf("runner called %d times, want 1", len(e.run.calls))
				}
				wantArgs := []string{"-G", "-F", e.m.Paths.UserConfig, "localhost"}
				if gotArgs := e.run.calls[0].Args; !equalStrings(gotArgs, wantArgs) {
					t.Errorf("ssh args = %v, want %v", gotArgs, wantArgs)
				}
			},
		},
		{
			name: "config is not checked without the client",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				e.m.Caps.SSH = ""
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host a\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "config-syntax-unchecked")
				if f == nil {
					t.Fatal("config-syntax-unchecked not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if len(e.run.calls) != 0 {
					t.Errorf("runner was called %d times without a client", len(e.run.calls))
				}
				if f := diagnoseTestLookup(got, "config-syntax"); f != nil {
					t.Errorf("syntax was checked without a client: %+v", f)
				}
			},
		},
		{
			name: "duplicate alias names both lines",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host prod\n    HostName one.example\n\nHost prod\n    HostName two.example\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "alias-duplicate")
				if f == nil {
					t.Fatal("alias-duplicate not found")
				}
				if f.Severity != SeverityWarning {
					t.Errorf("severity = %s, want warning", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig || f.Line != 4 {
					t.Errorf("File:Line = %s:%d, want %s:4", f.File, f.Line, e.m.Paths.UserConfig)
				}
				first := "1"
				if !strings.Contains(f.Detail, first) || !strings.Contains(f.Detail, "4") {
					t.Errorf("Detail %q does not name both positions", f.Detail)
				}
			},
		},
		{
			name: "duplicate alias across an Include",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host prod\n    HostName one.example\nInclude conf.d/extra\n")
				diagnoseTestWrite(t, filepath.Join(e.m.Paths.UserConfigDir, "conf.d", "extra"), 0o600, "Host prod\n    HostName two.example\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "alias-duplicate")
				if f == nil {
					t.Fatal("alias-duplicate not found in an included file")
				}
				included := filepath.Join(e.m.Paths.UserConfigDir, "conf.d", "extra")
				if f.File != included || f.Line != 1 {
					t.Errorf("File:Line = %s:%d, want %s:1", f.File, f.Line, included)
				}
			},
		},
		{
			name: "missing identity file",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host web\n    HostName web.example\n    IdentityFile ~/.ssh/id_missing\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "identity-missing")
				if f == nil {
					t.Fatal("identity-missing not found")
				}
				if f.Severity != SeverityWarning {
					t.Errorf("severity = %s, want warning", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig || f.Line != 3 {
					t.Errorf("File:Line = %s:%d, want %s:3", f.File, f.Line, e.m.Paths.UserConfig)
				}
				if !strings.Contains(f.Detail, "~/.ssh/id_missing") {
					t.Errorf("Detail %q does not print the path as written", f.Detail)
				}
				if !strings.Contains(f.Detail, "default") {
					t.Errorf("Detail %q does not mention the fallback", f.Detail)
				}
			},
		},
		{
			name: "IdentityFile in a wildcard stanza is skipped",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host *.example\n    IdentityFile ~/.ssh/id_missing\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "identity-missing"); f != nil {
					t.Errorf("wildcard stanza produced %+v", f)
				}
			},
		},
		{
			name: "StrictHostKeyChecking no is a warning",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host legacy\n    StrictHostKeyChecking no\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "host-key-check-disabled")
				if f == nil {
					t.Fatal("host-key-check-disabled not found")
				}
				if f.Severity != SeverityWarning {
					t.Errorf("severity = %s, want warning", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig || f.Line != 2 {
					t.Errorf("File:Line = %s:%d, want %s:2", f.File, f.Line, e.m.Paths.UserConfig)
				}
				if !strings.Contains(f.Detail, "StrictHostKeyChecking no") {
					t.Errorf("Detail %q does not quote the directive", f.Detail)
				}
				if f.Fix != "" {
					t.Errorf("Fix = %q, want empty: disabling verification is the user's decision", f.Fix)
				}
			},
		},
		{
			name: "StrictHostKeyChecking accept-new is a warning",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host legacy\n    StrictHostKeyChecking accept-new\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "host-key-check-disabled"); f == nil {
					t.Fatal("accept-new was not reported")
				}
			},
		},
		{
			name: "UserKnownHostsFile /dev/null is a warning",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host legacy\n    UserKnownHostsFile /dev/null\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "host-key-check-disabled")
				if f == nil {
					t.Fatal("UserKnownHostsFile /dev/null was not reported")
				}
				if !strings.Contains(f.Detail, "UserKnownHostsFile /dev/null") {
					t.Errorf("Detail %q does not quote the directive", f.Detail)
				}
			},
		},
		{
			name: "missing known_hosts",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host a\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "known-hosts-missing")
				if f == nil {
					t.Fatal("known-hosts-missing not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if f.File != e.m.Paths.KnownHosts {
					t.Errorf("File = %q, want %q", f.File, e.m.Paths.KnownHosts)
				}
			},
		},
		{
			name: "known_hosts present produces no finding",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.KnownHosts, 0o600, "")
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host a\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "known-hosts-missing"); f != nil {
					t.Errorf("known_hosts exists but produced %+v", f)
				}
			},
		},
		{
			name: "WSL true reports the split",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				e.m.Paths.WSL = true
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "wsl-separate-config")
				if f == nil {
					t.Fatal("wsl-separate-config not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if f.File != "" {
					t.Errorf("File = %q, want empty: it is a statement, not a file", f.File)
				}
				if !strings.Contains(f.Detail, "/mnt/c") || !strings.Contains(f.Detail, "WSL") {
					t.Errorf("Detail %q does not explain the split", f.Detail)
				}
			},
		},
		{
			name: "WSL false reports nothing",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if f := diagnoseTestLookup(got, "wsl-separate-config"); f != nil {
					t.Errorf("non-WSL host produced %+v", f)
				}
			},
		},
		{
			name: "empty Include",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Include ~/.ssh/conf.d/*\nHost a\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "include-empty")
				if f == nil {
					t.Fatal("include-empty not found")
				}
				if f.Severity != SeverityWarning {
					t.Errorf("severity = %s, want warning", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig || f.Line != 1 {
					t.Errorf("File:Line = %s:%d, want %s:1", f.File, f.Line, e.m.Paths.UserConfig)
				}
				if !strings.Contains(f.Detail, "~/.ssh/conf.d/*") {
					t.Errorf("Detail %q does not name the pattern", f.Detail)
				}
			},
		},
		{
			name: "Match exec is not evaluated here",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Match exec \"test -f /tmp/x\"\n    HostName x\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "match-not-evaluated")
				if f == nil {
					t.Fatal("match-not-evaluated not found")
				}
				if f.Severity != SeverityInfo {
					t.Errorf("severity = %s, want info", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig || f.Line != 1 {
					t.Errorf("File:Line = %s:%d, want %s:1", f.File, f.Line, e.m.Paths.UserConfig)
				}
			},
		},
		{
			name: "config with no read bit is unreadable",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host a\n")
				if err := os.Chmod(e.m.Paths.UserConfig, 0o000); err != nil {
					t.Fatalf("chmod: %v", err)
				}
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				f := diagnoseTestLookup(got, "config-unreadable")
				if f == nil {
					t.Fatal("config-unreadable not found")
				}
				if f.Severity != SeverityProblem {
					t.Errorf("severity = %s, want problem", f.Severity)
				}
				if f.File != e.m.Paths.UserConfig {
					t.Errorf("File = %q, want %q", f.File, e.m.Paths.UserConfig)
				}
				if len(e.run.calls) != 0 {
					t.Errorf("syntax check ran on an unreadable config: %d calls", len(e.run.calls))
				}
			},
		},
		{
			name: "clean environment produces an empty report",
			setup: func(t *testing.T, e *diagnoseTestEnv) {
				diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o700)
				diagnoseTestWrite(t, e.m.Paths.KnownHosts, 0o600, "")
				diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600, "Host a\n    HostName a.example\n")
			},
			check: func(t *testing.T, e *diagnoseTestEnv, got []Finding) {
				t.Helper()
				if len(got) != 0 {
					t.Errorf("clean environment produced %v", diagnoseTestCodes(got))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := diagnoseTestSetup(t)
			tc.setup(t, e)
			got := e.m.Diagnose(context.Background())

			diagnoseTestAssertOrdered(t, got)
			for _, f := range got {
				if f.Code == "" {
					t.Errorf("finding %+v has no Code", f)
				}
				if strings.Contains(f.Fix, "\n") {
					t.Errorf("Fix %q is not one line", f.Fix)
				}
			}
			tc.check(t, e, got)
		})
	}
}

// TestDiagnoseReportOrder pins the whole order with three severities at once.
func TestDiagnoseReportOrder(t *testing.T) {
	e := diagnoseTestSetup(t)
	diagnoseTestMkdir(t, e.m.Paths.SSHDir, 0o777)
	diagnoseTestWrite(t, filepath.Join(e.m.Paths.SSHDir, "id_rsa"), 0o644, "not a real key\n")
	diagnoseTestWrite(t, e.m.Paths.UserConfig, 0o600,
		"Host prod\n    HostName one.example\n\nHost prod\n    HostName two.example\n")
	e.m.Paths.WSL = true

	got := e.m.Diagnose(context.Background())
	want := []string{
		"private-key-permissions",
		"ssh-dir-permissions",
		"alias-duplicate",
		"known-hosts-missing",
		"wsl-separate-config",
	}
	diagnoseTestAssertCodes(t, got, want...)
	diagnoseTestAssertOrdered(t, got)
}

// TestSeverityString pins the words the interface prints.
func TestSeverityString(t *testing.T) {
	if got := SeverityInfo.String(); got != "info" {
		t.Errorf("SeverityInfo = %q, want info", got)
	}
	if got := SeverityWarning.String(); got != "warning" {
		t.Errorf("SeverityWarning = %q, want warning", got)
	}
	if got := SeverityProblem.String(); got != "problem" {
		t.Errorf("SeverityProblem = %q, want problem", got)
	}
}
