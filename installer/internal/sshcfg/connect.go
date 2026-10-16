package sshcfg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// THE CONNECTION TEST
// ============================================================================
//
// The test runs only when the user asks for it. It never prompts for anything,
// never accepts a host key, never writes known_hosts and never touches the
// user's credentials. Its whole job is to answer one question -- can this alias
// be reached and authenticated -- and to bring back a sentence the user can act
// on.

// ConnectOutcome is the classified result of a connection test. It is a string
// so it can be written to a log or a report, and it is a closed set so a screen
// can switch on it exhaustively.
type ConnectOutcome string

const (
	// OutcomeOK is a connection that opened and ran the remote command.
	OutcomeOK ConnectOutcome = "ok"
	// OutcomeTimeout is a connection that did not answer inside the bound.
	OutcomeTimeout ConnectOutcome = "timeout"
	// OutcomeCancelled is a test the caller stopped.
	OutcomeCancelled ConnectOutcome = "cancelled"
	// OutcomeHostKeyUnknown is a host whose key is not in known_hosts.
	OutcomeHostKeyUnknown ConnectOutcome = "host-key-unknown"
	// OutcomeHostKeyChanged is a host whose key differs from the stored one.
	OutcomeHostKeyChanged ConnectOutcome = "host-key-changed"
	// OutcomeAuth is a host that refused the client's authentication.
	OutcomeAuth ConnectOutcome = "auth"
	// OutcomeDNS is a hostname that could not be resolved.
	OutcomeDNS ConnectOutcome = "dns"
	// OutcomeRefused is a host that refused the connection on the port.
	OutcomeRefused ConnectOutcome = "refused"
	// OutcomeUnreachable is a host that could not be reached on the network.
	OutcomeUnreachable ConnectOutcome = "unreachable"
	// OutcomeFailed is every other failure.
	OutcomeFailed ConnectOutcome = "failed"
	// OutcomeNoClient is a host without an ssh client to run.
	OutcomeNoClient ConnectOutcome = "no-client"
)

// ConnectRequest is one test the user asked for.
type ConnectRequest struct {
	// Alias is the host to test, as the client would resolve it.
	Alias string
	// Timeout bounds the test. Zero means the package default, 10s.
	Timeout time.Duration
	// RemoteCommand is what to run on the other side. Empty means "true": the
	// test proves that a session opens and then leaves the host alone.
	RemoteCommand string
}

// ConnectResult is what the test found. Stdout and Stderr are the client's own
// output, bounded; Explanation is the sentence built for the user.
type ConnectResult struct {
	Alias          string
	Outcome        ConnectOutcome
	Duration       time.Duration
	ExitCode       int
	Stdout         string
	Stderr         string
	Explanation    string
	Args           []string
	HostKeyChanged bool
}

const (
	// defaultConnectTimeout bounds a test when the caller names none.
	defaultConnectTimeout = 10 * time.Second

	// connectOutputLimit bounds each of Stdout and Stderr in characters, so a
	// host that answers with a wall of text cannot be drawn into the
	// interface. The explanation is built from the classification and never
	// copies the wall.
	connectOutputLimit = 4096
)

// TestConnection runs one non-interactive `ssh` and classifies what it said.
func (m *Manager) TestConnection(ctx context.Context, req ConnectRequest) ConnectResult {
	start := time.Now()
	result := ConnectResult{Alias: req.Alias}

	// Without the client there is nothing to run. Saying so is more useful than
	// an exec error that names a missing binary.
	if m == nil || m.Caps.SSH == "" {
		result.Duration = time.Since(start)
		result.Outcome = OutcomeNoClient
		result.Explanation = "The ssh client was not found on this host, so nothing was run. Install an OpenSSH client and try again."
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultConnectTimeout
	}
	// ConnectTimeout takes whole seconds and the client rejects anything below
	// one, so a sub-second request is rounded up rather than dropped.
	seconds := int(timeout / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	remote := req.RemoteCommand
	if remote == "" {
		remote = "true"
	}

	// The flag list is the safety of the test, and every option is here for a
	// reason:
	//
	//   - BatchMode=yes and NumberOfPasswordPrompts=0 mean no authentication
	//     prompt is ever raised, so the test cannot hang on one.
	//   - StrictHostKeyChecking=yes and UpdateHostKeys=no mean a new host key
	//     is never accepted automatically and known_hosts is never written.
	//   - PasswordAuthentication=no and KbdInteractiveAuthentication=no mean
	//     the test does not try credentials a script cannot supply.
	//   - ControlMaster=no and ControlPath=none mean the test opens its own
	//     connection, so an existing multiplexed session cannot answer for it
	//     and produce a false "ok".
	//
	// The remote command is not an option: it is the trailing argument. A
	// comment does not repeat each flag separately because the list is exact:
	// changing it changes the guarantees above.
	args := []string{
		"-o", "BatchMode=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", seconds),
		"-o", "StrictHostKeyChecking=yes",
		"-o", "UpdateHostKeys=no",
		"-o", "NumberOfPasswordPrompts=0",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
		"-o", "ClearAllForwardings=yes",
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
		"-n", "-T", "--",
		req.Alias, remote,
	}
	// The program comes first so the list can be copied and repeated as it is.
	// No element carries a passphrase: this package passes none.
	result.Args = append([]string{m.Caps.SSH}, args...)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	run := m.Run
	if run == nil {
		run = ExecRunner{}
	}
	res := run.Run(runCtx, Command{
		Name: m.Caps.SSH,
		Args: args,
		// No terminal is needed for the test, and the process must not be able
		// to read the terminal the interface is drawing on. The `-n` option
		// and the runner's null stdin already cover that, so Detach stays
		// false: the test does not need its own session.
		Detach: false,
	})

	result.Duration = time.Since(start)
	result.ExitCode = res.Code
	result.Stdout = connectTruncate(res.Stdout)
	result.Stderr = connectTruncate(res.Stderr)
	result.Outcome, result.Explanation, result.HostKeyChanged = connectClassify(runCtx, res, timeout, remote)
	return result
}

// connectClassify turns the exit status and the client's own words into an
// outcome. The order matters: a deadline or a cancellation first, then the
// security messages, then the network messages, and only then the exit status.
func connectClassify(ctx context.Context, res Result, timeout time.Duration, remote string) (ConnectOutcome, string, bool) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return OutcomeTimeout, fmt.Sprintf("The host did not answer within %s, so the test gave up. Nothing was changed.", timeout), false
		}
		return OutcomeCancelled, "The test was cancelled before it finished. Nothing was changed.", false
	}

	lower := strings.ToLower(res.Stderr)
	changed := strings.ToUpper(res.Stderr)
	switch {
	case strings.Contains(changed, "REMOTE HOST IDENTIFICATION HAS CHANGED"):
		return OutcomeHostKeyChanged,
			"The host key this host presented is different from the one stored in known_hosts. That is what a man-in-the-middle attack looks like, so treat it as a security warning until proven otherwise. This test did not accept, replace or remove any key.",
			true
	case strings.Contains(lower, "host key verification failed") || strings.Contains(lower, "host key is known"):
		return OutcomeHostKeyUnknown,
			"The host key is not in known_hosts and strict checking refused it. Verify the fingerprint out of band before adding it; nothing was added by this test.",
			false
	case strings.Contains(lower, "could not resolve hostname"):
		return OutcomeDNS, "The hostname could not be resolved, so there was nothing to connect to.", false
	case strings.Contains(lower, "connection refused"):
		return OutcomeRefused, "The host answered but refused the connection on that port.", false
	case strings.Contains(lower, "connection timed out") ||
		strings.Contains(lower, "operation timed out") ||
		strings.Contains(lower, "no route to host") ||
		strings.Contains(lower, "network is unreachable"):
		return OutcomeUnreachable, "The host could not be reached on the network.", false
	case strings.Contains(lower, "permission denied"):
		return OutcomeAuth, "The client could not authenticate. This test does not use a password, and BatchMode means an agent or an unencrypted key is required.", false
	}

	if res.OK() {
		return OutcomeOK, fmt.Sprintf("The connection succeeded and the remote command %q ran.", remote), false
	}

	detail := messageSummary(res)
	if detail == "" {
		detail = fmt.Sprintf("ssh exited with status %d.", res.Code)
	}
	return OutcomeFailed, "The connection failed: " + detail, false
}

// connectTruncate bounds one stream of the client's output. The bound is in
// characters rather than bytes so a multi-byte character is never cut in half.
func connectTruncate(s string) string {
	runes := []rune(s)
	if len(runes) <= connectOutputLimit {
		return s
	}
	return string(runes[:connectOutputLimit])
}
