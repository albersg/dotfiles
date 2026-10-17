package portscan

import (
	"strings"
	"testing"
)

func TestRedactArgs(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "a separate flag and value",
			input: []string{"node", "server.js", "--token", "SECRET"},
			want:  []string{"node", "server.js", "--token", "****"},
		},
		{
			name:  "an assignment",
			input: []string{"node", "--token=SECRET"},
			want:  []string{"node", "--token=****"},
		},
		{
			name:  "an environment style name",
			input: []string{"env", "POSTGRES_PASSWORD=hunter2", "PORT=3000"},
			want:  []string{"env", "POSTGRES_PASSWORD=****", "PORT=3000"},
		},
		{
			name:  "a two-word account name",
			input: []string{"server", "--api-key", "abc", "--access-key=def"},
			want:  []string{"server", "--api-key", "****", "--access-key=****"},
		},
		{
			name:  "a header value",
			input: []string{"curl", "-H", "Authorization: Bearer abc.def", "-H", "Accept: text/plain"},
			want:  []string{"curl", "-H", "****", "-H", "Accept: text/plain"},
		},
		{
			name:  "a bare bearer token",
			input: []string{"Bearer", "abc.def"},
			want:  []string{"Bearer", "abc.def"},
		},
		{
			name:  "a credential inside a URL",
			input: []string{"psql", "postgres://alber:s3cret@localhost/app"},
			want:  []string{"psql", "postgres://alber:****@localhost/app"},
		},
		{
			name:  "a name that only looks secret",
			input: []string{"node", "--passwordless", "yes"},
			want:  []string{"node", "--passwordless", "yes"},
		},
		{
			name:  "nothing to redact",
			input: []string{"node", "server.js", "--port", "3000"},
			want:  []string{"node", "server.js", "--port", "3000"},
		},
		{
			name:  "an empty argv",
			input: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("RedactArgs(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("RedactArgs(%v) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestRedactArgsNeverMutatesItsInput(t *testing.T) {
	input := []string{"node", "--token", "SECRET"}
	RedactArgs(input)
	if input[2] != "SECRET" {
		t.Errorf("the input was modified: %v", input)
	}
}

func TestLooksLikeSecretName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"password", true},
		{"--password", true},
		{"POSTGRES_PASSWORD", true},
		{"--db.password", true},
		{"api-key", true},
		{"X-Api-Key", true},
		{"mytoken", true},
		{"sessionsecret", true},
		{"authorization", true},
		{"--authenticator", false},
		{"--passwordless", false},
		{"port", false},
		{"--hostname", false},
	}

	for _, tt := range tests {
		if got := looksLikeSecretName(tt.name); got != tt.want {
			t.Errorf("looksLikeSecretName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestProtectedReason(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	p.parents[p.selfPID] = 900
	p.parents[900] = 1

	tests := []struct {
		name     string
		proc     Process
		contains string
	}{
		{name: "pid 1", proc: Process{PID: 1, Name: "systemd"}, contains: "init process"},
		{name: "the installer itself", proc: Process{PID: p.selfPID, Name: "dotfiles"}, contains: "installer's own process"},
		{name: "the parent shell", proc: Process{PID: 900, Name: "zsh"}, contains: "ancestor"},
		{name: "the parent shell's parent", proc: Process{PID: 1, Name: "init"}, contains: "init process"},
		{name: "a protected daemon by name", proc: Process{PID: 700, Name: "systemd-resolved"}, contains: "refuses to stop"},
		{name: "a protected daemon with a suffix", proc: Process{PID: 701, Name: "kernel_task"}, contains: "refuses to stop"},
		{name: "an ordinary process", proc: Process{PID: 999, Name: "node"}, contains: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProtectedReason(p, tt.proc)
			if tt.contains == "" {
				if got != "" {
					t.Fatalf("ProtectedReason(%+v) = %q, want no refusal", tt.proc, got)
				}
				return
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("ProtectedReason(%+v) = %q, want it to mention %q", tt.proc, got, tt.contains)
			}
		})
	}
}

func TestStopRefusalNamesAScopeItCannotReach(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	reason := StopRefusal(p, StopTarget{Process: Process{PID: 999, Name: "node", Scope: ScopeWSLGuest}})
	if !strings.Contains(reason, ScopeWSLGuest) || !strings.Contains(reason, ScopeSelf) {
		t.Errorf("StopRefusal = %q, want both scopes named so the user sees why it is refused", reason)
	}
	if !strings.Contains(reason, "Nothing was stopped") {
		t.Errorf("StopRefusal = %q, want the outcome stated", reason)
	}
}

func TestStopRefusalNeedsAProcessID(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	if reason := StopRefusal(p, StopTarget{Process: Process{PID: 0}}); !strings.Contains(reason, "nothing to stop") {
		t.Errorf("StopRefusal = %q, want the missing pid named", reason)
	}
}

func TestStopRefusalAllowsAnOrdinaryProcess(t *testing.T) {
	p := linuxHost(999, 3000, "777")
	if reason := StopRefusal(p, StopTarget{Process: Process{PID: 999, Name: "node", Scope: ScopeSelf}}); reason != "" {
		t.Errorf("StopRefusal = %q, want no refusal for an ordinary process", reason)
	}
}
