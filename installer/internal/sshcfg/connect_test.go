package sshcfg

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// THE CONNECTION TEST
// ============================================================================
//
// This is the only part of the package that opens a network connection to
// somebody else's machine, so its tests are about the flags more than about the
// outcomes: the outcome table can be re-derived from the client's words, but a
// flag that stops being passed is a promise that quietly stopped holding. The
// first test therefore compares the whole argument list, and it is meant to fail
// loudly when one option is edited away.

// connectTestWantArgs is the argument list the test must run with, option by
// option. Each line carries the guarantee it buys.
func connectTestWantArgs() []string {
	return []string{
		"-o", "BatchMode=yes", // never raises an authentication prompt
		"-o", "ConnectTimeout=10", // the bound the caller asked for
		"-o", "StrictHostKeyChecking=yes", // never accepts a new host key
		"-o", "UpdateHostKeys=no", // never writes to known_hosts
		"-o", "NumberOfPasswordPrompts=0", // never asks for a password
		"-o", "PasswordAuthentication=no", // never tries a password
		"-o", "KbdInteractiveAuthentication=no", // never tries keyboard-interactive
		"-o", "ClearAllForwardings=yes", // never carries the user's forwards
		"-o", "ControlMaster=no", // never reuses somebody else's session
		"-o", "ControlPath=none", // never multiplexes at all
		"-n", "-T", "--", // no input, no terminal, end of options
		"web", "true", // the host, and a command that changes nothing
	}
}

func TestConnectionTestRunsWithEverySafetyOption(t *testing.T) {
	e := testManagerSetup(t)
	e.runner.handler = func(cmd Command) (Result, bool) { return okResult(""), true }

	result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web"})

	if len(e.runner.calls) != 1 {
		t.Fatalf("ran %d commands, want 1", len(e.runner.calls))
	}
	call := e.runner.calls[0]
	if call.Name != "/usr/bin/ssh" {
		t.Errorf("ran %q, want the ssh client", call.Name)
	}
	want := connectTestWantArgs()
	if !equalStrings(call.Args, want) {
		t.Errorf("ssh args =\n%v\nwant\n%v", call.Args, want)
	}
	if !equalStrings(result.Args, append([]string{"/usr/bin/ssh"}, want...)) {
		t.Errorf("the reported args =\n%v\nwant the program followed by the same list", result.Args)
	}
	// A test that needs its own session is a test that can leave a process
	// behind; this one must not.
	if call.Detach {
		t.Error("the test was detached into its own session")
	}
	if call.Stdin != "" {
		t.Errorf("Stdin = %q, want nothing piped to the client", call.Stdin)
	}
	if call.Env != nil {
		t.Errorf("Env = %v, want no environment override", call.Env)
	}
}

func TestConnectionTestOutcomeIsReadFromTheClientsWords(t *testing.T) {
	cases := []struct {
		name        string
		stderr      string
		code        int
		want        ConnectOutcome
		wantChanged bool
	}{
		{
			name:   "a connection that worked",
			want:   OutcomeOK,
			stderr: "",
		},
		{
			name:   "a host key that is not known",
			stderr: "Host key verification failed.\n",
			code:   255,
			want:   OutcomeHostKeyUnknown,
		},
		{
			name:        "a host key that changed",
			stderr:      "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\n",
			code:        255,
			want:        OutcomeHostKeyChanged,
			wantChanged: true,
		},
		{
			name:   "a name that does not resolve",
			stderr: "ssh: Could not resolve hostname nope.example.com: Name or service not known\n",
			code:   255,
			want:   OutcomeDNS,
		},
		{
			name:   "a port that refuses",
			stderr: "ssh: connect to host web port 22: Connection refused\n",
			code:   255,
			want:   OutcomeRefused,
		},
		{
			name:   "a network that is unreachable",
			stderr: "ssh: connect to host web port 22: No route to host\n",
			code:   255,
			want:   OutcomeUnreachable,
		},
		{
			name:   "a connection that timed out",
			stderr: "ssh: connect to host web port 22: Connection timed out\n",
			code:   255,
			want:   OutcomeUnreachable,
		},
		{
			name:   "credentials that were refused",
			stderr: "alice@web: Permission denied (publickey).\n",
			code:   255,
			want:   OutcomeAuth,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := testManagerSetup(t)
			e.runner.handler = func(cmd Command) (Result, bool) {
				return failResult(tt.code, tt.stderr), true
			}
			result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web"})
			if result.Outcome != tt.want {
				t.Errorf("Outcome = %q, want %q (explanation: %s)", result.Outcome, tt.want, result.Explanation)
			}
			if result.HostKeyChanged != tt.wantChanged {
				t.Errorf("HostKeyChanged = %v, want %v", result.HostKeyChanged, tt.wantChanged)
			}
			if result.Explanation == "" {
				t.Error("the outcome has no explanation for the user")
			}
			if result.Alias != "web" {
				t.Errorf("Alias = %q, want the host that was tested", result.Alias)
			}
		})
	}
}

func TestConnectionTestNeverClaimsAChangedHostKeyIsAnythingElse(t *testing.T) {
	e := testManagerSetup(t)
	const warning = "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n" +
		"@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\n" +
		"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n" +
		"IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\n" +
		"Offending key for IP in /home/alice/.ssh/known_hosts:3\n"
	e.runner.handler = func(cmd Command) (Result, bool) { return failResult(255, warning), true }

	result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web"})
	if !result.HostKeyChanged {
		t.Fatal("the changed host key was not reported as a change")
	}
	if !strings.Contains(result.Explanation, "man-in-the-middle") {
		t.Errorf("Explanation = %q, want it to say what this looks like", result.Explanation)
	}
	if !strings.Contains(result.Explanation, "did not accept, replace or remove any key") {
		t.Errorf("Explanation = %q, want it to say nothing was changed", result.Explanation)
	}
	// The client's own warning is kept as output the user can read, and it is what
	// makes the outcome verifiable rather than asserted.
	if !strings.Contains(result.Stderr, "REMOTE HOST IDENTIFICATION HAS CHANGED") {
		t.Error("the client's warning was dropped from the result")
	}
}

func TestConnectionTestReportsATimeoutAndACancellation(t *testing.T) {
	t.Run("a deadline that has passed", func(t *testing.T) {
		e := testManagerSetup(t)
		e.runner.handler = func(cmd Command) (Result, bool) {
			return Result{Code: -1, Err: context.DeadlineExceeded}, true
		}
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()

		result := e.manager.TestConnection(ctx, ConnectRequest{Alias: "web", Timeout: 5 * time.Second})
		if result.Outcome != OutcomeTimeout {
			t.Errorf("Outcome = %q, want %q", result.Outcome, OutcomeTimeout)
		}
		if !strings.Contains(result.Explanation, "5s") {
			t.Errorf("Explanation = %q, want it to name the bound that was exceeded", result.Explanation)
		}
	})

	t.Run("a test the user stopped", func(t *testing.T) {
		e := testManagerSetup(t)
		e.runner.handler = func(cmd Command) (Result, bool) {
			return Result{Code: -1, Err: context.Canceled}, true
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result := e.manager.TestConnection(ctx, ConnectRequest{Alias: "web"})
		if result.Outcome != OutcomeCancelled {
			t.Errorf("Outcome = %q, want %q", result.Outcome, OutcomeCancelled)
		}
	})
}

func TestConnectionTestRoundsTheTimeoutUpToASecond(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
		want    string
	}{
		{"the default bound", 0, "ConnectTimeout=10"},
		{"a bound under a second", 100 * time.Millisecond, "ConnectTimeout=1"},
		{"a bound in whole seconds", 3 * time.Second, "ConnectTimeout=3"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := testManagerSetup(t)
			e.runner.handler = func(cmd Command) (Result, bool) { return okResult(""), true }
			e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web", Timeout: tt.timeout})

			args := strings.Join(e.runner.calls[0].Args, " ")
			if !strings.Contains(args, tt.want) {
				t.Errorf("args = %q, want %q: the client rejects a ConnectTimeout below one second", args, tt.want)
			}
		})
	}
}

func TestConnectionTestWithoutAClientRunsNothing(t *testing.T) {
	e := testManagerSetup(t)
	e.manager.Caps.SSH = ""

	result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web"})
	if result.Outcome != OutcomeNoClient {
		t.Errorf("Outcome = %q, want %q", result.Outcome, OutcomeNoClient)
	}
	if len(e.runner.calls) != 0 {
		t.Errorf("a command ran without a client: %v", e.runner.calls)
	}
	if !strings.Contains(result.Explanation, "ssh client") {
		t.Errorf("Explanation = %q, want it to name what is missing", result.Explanation)
	}
}

func TestConnectionTestBoundsWhatTheHostSaid(t *testing.T) {
	e := testManagerSetup(t)
	wall := strings.Repeat("x", connectOutputLimit*3)
	e.runner.handler = func(cmd Command) (Result, bool) {
		return Result{Code: 1, Stdout: wall, Stderr: wall}, true
	}

	result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web"})
	if len([]rune(result.Stdout)) > connectOutputLimit {
		t.Errorf("Stdout is %d characters, want at most %d", len([]rune(result.Stdout)), connectOutputLimit)
	}
	if len([]rune(result.Stderr)) > connectOutputLimit {
		t.Errorf("Stderr is %d characters, want at most %d", len([]rune(result.Stderr)), connectOutputLimit)
	}
	// The explanation is built from the classification, never from the wall.
	if len([]rune(result.Explanation)) > 400 {
		t.Errorf("Explanation is %d characters, want a sentence rather than the host's output", len([]rune(result.Explanation)))
	}
}

func TestConnectionTestRunsTheCommandTheCallerChose(t *testing.T) {
	e := testManagerSetup(t)
	e.runner.handler = func(cmd Command) (Result, bool) { return okResult("uptime\n"), true }

	result := e.manager.TestConnection(context.Background(), ConnectRequest{Alias: "web", RemoteCommand: "uptime"})
	args := e.runner.calls[0].Args
	if args[len(args)-1] != "uptime" {
		t.Errorf("the remote command is %q, want the one the caller asked for", args[len(args)-1])
	}
	if !strings.Contains(result.Explanation, "uptime") {
		t.Errorf("Explanation = %q, want it to name the command that ran", result.Explanation)
	}
}
