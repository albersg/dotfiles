package sshcfg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// THE SSH AGENT
// ============================================================================
//
// The assertions here are mostly about what a person is told. An agent that is
// running and empty is not a broken agent, an agent this tool cannot identify is
// not assumed to behave like the one the reader has in mind, and a passphrase
// never appears in an argument list or an environment value -- the last one is
// the assertion that matters most, so it is checked on every add.

// agentTestEnv is a manager whose agent socket is set, so the kinds can be
// exercised without a real agent.
func agentTestEnv(t *testing.T, socket string) *testManager {
	t.Helper()
	e := testManagerSetup(t)
	e.manager.Paths.AgentSocket = socket
	return e
}

func TestAgentKindIsNamedFromTheSocket(t *testing.T) {
	cases := []struct {
		name   string
		socket string
		want   string
		note   string
	}{
		{
			name:   "gpg-agent",
			socket: "/run/user/1000/gnupg/S.gpg-agent.ssh",
			want:   agentKindGPG,
			note:   "ignores ssh-add's lifetime",
		},
		{
			name:   "1Password",
			socket: "/home/alice/.1password/agent.sock",
			want:   agentKindOnepass,
			note:   "already there",
		},
		{
			name:   "KeePassXC",
			socket: "/run/user/1000/keepassxc/agent.sock",
			want:   agentKindKeePass,
			note:   "can be refused",
		},
		{
			name:   "the OpenSSH agent",
			socket: "/tmp/ssh-XXXXXXabc/agent.1234",
			want:   agentKindOpenSSH,
		},
		{
			name:   "an agent this tool cannot name",
			socket: "/run/user/1000/some-other-agent.sock",
			want:   "",
			note:   "not one this tool can describe",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			kind, notes := agentKind(tt.socket)
			if tt.want == "" {
				if kind == "" || !strings.HasPrefix(kind, "unknown agent") {
					t.Fatalf("kind = %q, want an unknown agent named by its socket", kind)
				}
			} else if kind != tt.want {
				t.Fatalf("kind = %q, want %q", kind, tt.want)
			}
			if tt.note != "" && !containsAll(strings.Join(notes, " "), tt.note) {
				t.Errorf("notes = %v, want one mentioning %q", notes, tt.note)
			}
			if tt.want == agentKindOpenSSH && len(notes) != 0 {
				t.Errorf("the OpenSSH agent got notes it does not need: %v", notes)
			}
		})
	}
}

func TestAgentStateReportsWhatTheAgentHolds(t *testing.T) {
	const listing = "256 SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa alice@laptop (ED25519)\n" +
		"4096 SHA256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb a key with spaces (RSA)\n"

	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	e.runner.handler = func(cmd Command) (Result, bool) {
		if cmd.Name == e.manager.Caps.Add && equalStrings(cmd.Args, []string{"-l"}) {
			return okResult(listing), true
		}
		return Result{}, false
	}

	state := e.manager.AgentState(context.Background())
	if !state.Advertised || !state.Reachable {
		t.Fatalf("state = %+v, want an advertised and reachable agent", state)
	}
	if state.Kind != agentKindOpenSSH || !state.LifetimeSupported {
		t.Errorf("kind = %q lifetime = %v, want the OpenSSH agent with lifetime support", state.Kind, state.LifetimeSupported)
	}
	if len(state.Identities) != 2 {
		t.Fatalf("identities = %+v, want two", state.Identities)
	}
	first, second := state.Identities[0], state.Identities[1]
	if first.Bits != 256 || first.Type != "ED25519" || first.Comment != "alice@laptop" {
		t.Errorf("first identity = %+v", first)
	}
	// A comment with spaces is the case a naive field split gets wrong.
	if second.Comment != "a key with spaces" || second.Type != "RSA" || second.Bits != 4096 {
		t.Errorf("second identity = %+v, want the comment to keep its spaces", second)
	}
	if !state.CanAdd || !state.CanRemove || !state.CanRemoveAll {
		t.Errorf("state = %+v, want every agent action offered", state)
	}
}

func TestAgentStateTreatsAnEmptyAgentAsWorking(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	// Exit 1 is what ssh-add answers for an agent that is running and holds
	// nothing. Reporting it as a failure is how a screen ends up telling a user
	// their agent is broken when it is merely empty.
	e.runner.handler = func(cmd Command) (Result, bool) { return failResult(1, ""), true }

	state := e.manager.AgentState(context.Background())
	if !state.Reachable {
		t.Errorf("state = %+v, want the empty agent to be reachable", state)
	}
	if len(state.Identities) != 0 {
		t.Errorf("identities = %+v, want none", state.Identities)
	}
	if state.Reason != "" {
		t.Errorf("Reason = %q, want no complaint about a working agent", state.Reason)
	}
	if !state.CanAdd {
		t.Error("an empty agent refuses to accept a key")
	}
}

func TestAgentStateExplainsAnAgentThatDoesNotAnswer(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return failResult(2, "Could not open a connection to your authentication agent.\n"), true
	}

	state := e.manager.AgentState(context.Background())
	if state.Reachable {
		t.Error("a refused connection was reported as a reachable agent")
	}
	if !strings.Contains(state.Reason, "Could not open a connection") {
		t.Errorf("Reason = %q, want the agent's own words", state.Reason)
	}
	if state.CanAdd || state.CanRemove {
		t.Errorf("state = %+v offers actions on an agent that does not answer", state)
	}
}

func TestAgentStateWithoutAnAgentOrTheTool(t *testing.T) {
	t.Run("no socket is advertised", func(t *testing.T) {
		e := testManagerSetup(t)
		state := e.manager.AgentState(context.Background())
		if state.Advertised {
			t.Error("an agent was reported without SSH_AUTH_SOCK")
		}
		if !strings.Contains(state.Reason, "SSH_AUTH_SOCK") {
			t.Errorf("Reason = %q, want it to name the variable that was missing", state.Reason)
		}
		if len(e.runner.calls) != 0 {
			t.Errorf("commands were run without an agent: %v", e.runner.calls)
		}
	})

	t.Run("ssh-add is not installed", func(t *testing.T) {
		e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
		e.manager.Caps.Add = ""
		state := e.manager.AgentState(context.Background())
		if state.Reachable {
			t.Error("the agent was reported as questioned without ssh-add")
		}
		if !strings.Contains(state.Reason, "ssh-add") {
			t.Errorf("Reason = %q, want it to name the missing tool", state.Reason)
		}
		if len(e.runner.calls) != 0 {
			t.Errorf("commands were run without ssh-add: %v", e.runner.calls)
		}
	})
}

func TestAddToAgentNeverPutsThePassphraseInAnArgument(t *testing.T) {
	const passphrase = "correct horse battery staple"
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	key := filepath.Join(e.home, ".ssh", "id_work")
	e.write(t, key, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return okResult("Identity added: " + key + " (alice@laptop)\n"), true
	}

	action := e.manager.AddToAgent(context.Background(), key, passphrase, 0)
	if !action.OK {
		t.Fatalf("AddToAgent failed: %+v", action)
	}
	if len(e.runner.calls) != 1 {
		t.Fatalf("ran %d commands, want 1", len(e.runner.calls))
	}
	call := e.runner.calls[0]
	if !equalStrings(call.Args, []string{key}) {
		t.Errorf("args = %v, want the key path alone", call.Args)
	}
	for _, arg := range call.Args {
		if strings.Contains(arg, passphrase) {
			t.Fatal("the passphrase was passed as an argument, where every process on the host can read it")
		}
	}
	for name, value := range call.Env {
		if strings.Contains(value, passphrase) {
			t.Fatalf("the passphrase was passed in the environment (%s), where every process on the host can read it", name)
		}
	}
	if call.Stdin != passphrase+"\n" {
		t.Errorf("Stdin = %q, want the passphrase on the agent's standard input", call.Stdin)
	}
	if !call.Detach {
		t.Error("the child was not detached, so it may read the terminal instead of the pipe")
	}
	if call.Env["DISPLAY"] != "" || call.Env["SSH_ASKPASS"] != askPassDisabled {
		t.Errorf("Env = %v, want the graphical prompter disabled", call.Env)
	}
	if strings.Contains(action.Explanation, passphrase) || strings.Contains(action.Args[len(action.Args)-1], passphrase) {
		t.Error("the passphrase appears in what the action reports")
	}
}

func TestAddToAgentWithoutAPassphraseDoesNotPipeOne(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	key := filepath.Join(e.home, ".ssh", "id_work")
	e.write(t, key, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return okResult("Identity added: " + key + "\n"), true
	}

	if action := e.manager.AddToAgent(context.Background(), key, "", 0); !action.OK {
		t.Fatalf("AddToAgent failed: %+v", action)
	}
	call := e.runner.calls[0]
	if call.Stdin != "" {
		t.Errorf("Stdin = %q, want nothing piped for a key with no passphrase", call.Stdin)
	}
	if call.Detach {
		t.Error("a child that needs no input was detached from the terminal anyway")
	}
}

func TestAddToAgentAsksTheAgentForALifetimeOnlyWhenOneWasChosen(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	key := filepath.Join(e.home, ".ssh", "id_work")
	e.write(t, key, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	e.runner.handler = func(cmd Command) (Result, bool) { return okResult("Identity added\n"), true }

	if action := e.manager.AddToAgent(context.Background(), key, "", 4*time.Hour); !action.OK {
		t.Fatalf("AddToAgent failed: %+v", action)
	}
	if !equalStrings(e.runner.calls[0].Args, []string{"-t", "14400", key}) {
		t.Errorf("args = %v, want a lifetime in seconds before the key", e.runner.calls[0].Args)
	}
}

func TestAddToAgentRefusesAKeyItCannotSee(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	action := e.manager.AddToAgent(context.Background(), filepath.Join(e.home, "gone"), "", 0)
	if action.OK {
		t.Error("AddToAgent reported success for a key that does not exist")
	}
	if len(e.runner.calls) != 0 {
		t.Errorf("a command ran for a key that does not exist: %v", e.runner.calls)
	}
	if action.Explanation == "" {
		t.Error("the refusal explains nothing")
	}
}

func TestAddToAgentReportsWhatTheAgentSaid(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	key := filepath.Join(e.home, ".ssh", "id_work")
	e.write(t, key, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return failResult(1, "Bad passphrase, try again for "+key+"\n"), true
	}

	action := e.manager.AddToAgent(context.Background(), key, "wrong", 0)
	if action.OK {
		t.Fatal("AddToAgent reported success for a refused passphrase")
	}
	if !strings.Contains(action.Output, "Bad passphrase") {
		t.Errorf("Output = %q, want the agent's own message", action.Output)
	}
	if !strings.Contains(action.Explanation, "not accepted") {
		t.Errorf("Explanation = %q, want it to say the passphrase was refused", action.Explanation)
	}
	if strings.Contains(action.Explanation, "wrong") {
		t.Error("the refused passphrase was quoted back in the report")
	}
}

func TestRemoveFromAgentWantsThePublicFileNotAFingerprint(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	pub := filepath.Join(e.home, ".ssh", "id_work.pub")
	e.write(t, pub, 0o644, "ssh-ed25519 AAAA alice@laptop\n")
	e.runner.handler = func(cmd Command) (Result, bool) { return okResult("Identity removed: " + pub + "\n"), true }

	t.Run("a fingerprint is refused without running anything", func(t *testing.T) {
		action := e.manager.RemoveFromAgent(context.Background(), "SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if action.OK {
			t.Error("a fingerprint was accepted as an identity to remove")
		}
		if len(e.runner.calls) != 0 {
			t.Fatalf("a command ran for a fingerprint: %v", e.runner.calls)
		}
		if !strings.Contains(action.Explanation, "public key file") {
			t.Errorf("Explanation = %q, want it to say what ssh-add needs", action.Explanation)
		}
	})

	t.Run("the public file is what runs", func(t *testing.T) {
		action := e.manager.RemoveFromAgent(context.Background(), pub)
		if !action.OK {
			t.Fatalf("RemoveFromAgent failed: %+v", action)
		}
		if !equalStrings(e.runner.calls[len(e.runner.calls)-1].Args, []string{"-d", pub}) {
			t.Errorf("args = %v, want -d with the public key file", e.runner.calls[len(e.runner.calls)-1].Args)
		}
	})
}

func TestRemoveAllFromAgentSaysTheKeysThemselvesAreUntouched(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	e.runner.handler = func(cmd Command) (Result, bool) { return okResult("All identities removed.\n"), true }

	action := e.manager.RemoveAllFromAgent(context.Background())
	if !action.OK {
		t.Fatalf("RemoveAllFromAgent failed: %+v", action)
	}
	if !equalStrings(e.runner.calls[0].Args, []string{"-D"}) {
		t.Errorf("args = %v, want -D alone", e.runner.calls[0].Args)
	}
	if !strings.Contains(action.Explanation, "untouched on disk") {
		t.Errorf("Explanation = %q, want it to say the keys were not deleted", action.Explanation)
	}
}

func TestRemoveFromAgentExplainsAnAgentThatDoesNotHoldTheKey(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	pub := filepath.Join(e.home, ".ssh", "id_work.pub")
	e.write(t, pub, 0o644, "ssh-ed25519 AAAA alice@laptop\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return failResult(1, "Could not remove identity "+pub+": no such identity\n"), true
	}

	action := e.manager.RemoveFromAgent(context.Background(), pub)
	if action.OK {
		t.Fatal("a refused removal was reported as a removal")
	}
	if !strings.Contains(action.Explanation, "does not hold an identity") {
		t.Errorf("Explanation = %q, want the one interpretation this tool is sure of", action.Explanation)
	}
}

func TestLifetimeIsOnlyPromisedForTheOpenSSHAgent(t *testing.T) {
	kinds := []struct {
		socket string
		want   bool
	}{
		{"/tmp/ssh-XXXXXXabc/agent.1234", true},
		{"/run/user/1000/gnupg/S.gpg-agent.ssh", false},
		{"/home/alice/.1password/agent.sock", false},
		{"/run/user/1000/keepassxc/agent.sock", false},
	}
	for _, tt := range kinds {
		t.Run(filepath.Base(tt.socket), func(t *testing.T) {
			e := agentTestEnv(t, tt.socket)
			e.runner.handler = func(cmd Command) (Result, bool) { return okResult(""), true }
			state := e.manager.AgentState(context.Background())
			if state.LifetimeSupported != tt.want {
				t.Errorf("LifetimeSupported = %v for %s, want %v", state.LifetimeSupported, state.Kind, tt.want)
			}
			if !tt.want && len(state.Notes) == 0 {
				t.Errorf("%s is offered without saying what it does differently", state.Kind)
			}
		})
	}
}

func TestAgentNotesAreTheOnesForThisPlatform(t *testing.T) {
	// The macOS keychain is only named on macOS: on any other platform that socket
	// path belongs to some other program, and naming the keychain there would be a
	// guess presented as a fact.
	kind, _ := agentKind("/private/tmp/com.apple.launchd.abc/keychain.sock")
	if runtime.GOOS == "darwin" {
		if kind != agentKindKeychain {
			t.Errorf("kind = %q, want the login keychain's agent on darwin", kind)
		}
		return
	}
	if kind == agentKindKeychain {
		t.Error("the macOS keychain was named on a platform that has none")
	}
}

func TestParseAgentListKeepsWhatTheToolPrinted(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		want  AgentIdentity
		count int
	}{
		{
			name:  "a certificate",
			line:  "256 SHA256:abc alice@host (ED25519-CERT)",
			want:  AgentIdentity{Bits: 256, Fingerprint: "SHA256:abc", Comment: "alice@host", Type: "ED25519-CERT"},
			count: 1,
		},
		{
			name:  "a line with no type",
			line:  "2048 SHA256:def",
			want:  AgentIdentity{Bits: 2048, Fingerprint: "SHA256:def"},
			count: 1,
		},
		{
			name:  "blank lines are not identities",
			line:  "\n",
			count: 0,
		},
		{
			name:  "a Windows line ending",
			line:  "256 SHA256:abc alice@host (ED25519)\r",
			want:  AgentIdentity{Bits: 256, Fingerprint: "SHA256:abc", Comment: "alice@host", Type: "ED25519"},
			count: 1,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAgentList(tt.line)
			if len(got) != tt.count {
				t.Fatalf("parsed %d identities, want %d: %+v", len(got), tt.count, got)
			}
			if tt.count == 1 && got[0] != tt.want {
				t.Errorf("parsed %+v, want %+v", got[0], tt.want)
			}
		})
	}
}

func TestRemoveFromAgentRefusesAPublicFileItCannotSee(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	action := e.manager.RemoveFromAgent(context.Background(), filepath.Join(e.home, ".ssh", "gone.pub"))
	if action.OK || len(e.runner.calls) != 0 {
		t.Errorf("action = %+v calls = %v, want a refusal with nothing run", action, e.runner.calls)
	}
	if !strings.Contains(action.Explanation, "could not be read") {
		t.Errorf("Explanation = %q, want it to name the file it could not read", action.Explanation)
	}
}

func TestAddToAgentWithoutTheToolExplainsItself(t *testing.T) {
	e := agentTestEnv(t, "/tmp/ssh-XXXXXXabc/agent.1234")
	e.manager.Caps.Add = ""
	key := filepath.Join(e.home, ".ssh", "id_work")
	e.write(t, key, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")

	action := e.manager.AddToAgent(context.Background(), key, "", 0)
	if action.OK {
		t.Error("AddToAgent reported success without ssh-add")
	}
	if !strings.Contains(action.Explanation, "ssh-add") {
		t.Errorf("Explanation = %q, want it to name the missing tool", action.Explanation)
	}
	if _, err := os.Stat(key); err != nil {
		t.Errorf("the key file was touched: %v", err)
	}
}
