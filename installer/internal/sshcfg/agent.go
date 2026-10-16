package sshcfg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// THE SSH AGENT
// ============================================================================
//
// An agent is not an interface: OpenSSH's ssh-agent, gpg-agent's SSH support,
// 1Password's and KeePassXC's agents all answer `ssh-add -l`, and they differ in
// what they do with what ssh-add asks of them. This file therefore does two
// things and refuses to do a third:
//
//   - It reports what the agent in this environment says about itself, from
//     `ssh-add -l` and the socket the environment advertises, and nothing else.
//   - It offers add and remove through ssh-add, which is the only mechanism
//     OpenSSH defines, and reports the tool's own answer.
//   - It never claims that a lifetime will be honoured, that a removal will
//     stick, or that an agent it cannot identify behaves like the one the reader
//     has in mind. Notes carries what is known about the kind it recognised,
//     and an unknown agent gets a sentence that says exactly that.

// AgentIdentity is one identity the agent holds, as `ssh-add -l` prints it.
type AgentIdentity struct {
	Bits        int
	Fingerprint string
	Comment     string
	Type        string
}

// AgentState is what this environment's agent says about itself.
type AgentState struct {
	// Advertised reports that SSH_AUTH_SOCK named a socket when this process
	// started. A process cannot see an agent it was not told about.
	Advertised bool
	Socket     string
	// Reachable reports that the agent answered the listing. An agent that is
	// advertised and unreachable is the common case after a session was restored
	// without its agent, and it is reported as such rather than as "no
	// identities".
	Reachable  bool
	Kind       string
	Notes      []string
	Identities []AgentIdentity
	Reason     string
	CanAdd     bool
	CanRemove  bool
	// CanRemoveAll reports that removing every identity is available. It needs
	// ssh-add and a reachable agent, nothing more.
	CanRemoveAll bool
	// LifetimeSupported reports whether `ssh-add -t` is expected to be honoured.
	// It is true for the OpenSSH agent and false for every other kind, because
	// the others are known to ignore or reinterpret the option.
	LifetimeSupported bool
}

// AgentAction is the outcome of one add, remove or clear.
//
// Args is the argument list that ran, which is safe to show and to log: a
// passphrase is never part of it. Output is the tool's own first line, so a
// refusal can be read in the words of the program that refused.
type AgentAction struct {
	OK          bool
	Output      string
	Explanation string
	Args        []string
}

// AgentState asks the agent what it holds. It runs `ssh-add -l` and reads the
// environment, and it never opens a key file or an agent socket itself.
func (m *Manager) AgentState(ctx context.Context) AgentState {
	state := AgentState{Socket: m.Paths.AgentSocket}
	if state.Socket == "" {
		state.Reason = "no SSH agent is advertised in this environment (SSH_AUTH_SOCK is unset)"
		return state
	}
	state.Advertised = true
	state.Kind, state.Notes = agentKind(state.Socket)
	state.LifetimeSupported = state.Kind == agentKindOpenSSH

	if m.Caps.Add == "" {
		state.Reason = "ssh-add was not found on PATH, so the agent cannot be asked what it holds"
		return state
	}

	res := m.Run.Run(ctx, Command{Name: m.Caps.Add, Args: []string{"-l"}})
	switch {
	case res.Err != nil:
		state.Reason = "the agent could not be asked: " + res.Message()
		if res.Err == context.DeadlineExceeded || res.Err == context.Canceled {
			state.Reason = "the agent did not answer in time"
		}
		return state
	case res.Code == 0:
		state.Reachable = true
		state.Identities = parseAgentList(res.Stdout)
	case res.Code == 1:
		// Exit 1 is the documented answer for an agent that is running and holds
		// nothing. Treating it as a failure is the classic way to tell a user
		// their agent is broken when it is merely empty.
		state.Reachable = true
	case res.Code == 2:
		state.Reason = firstLine(res.Stderr)
		if state.Reason == "" {
			state.Reason = "the agent could not be reached (ssh-add exited 2)"
		}
		return state
	default:
		state.Reason = res.Message()
		if state.Reason == "" {
			state.Reason = fmt.Sprintf("ssh-add exited %d without a message", res.Code)
		}
		return state
	}

	state.CanAdd = true
	state.CanRemove = true
	state.CanRemoveAll = true
	if len(state.Identities) == 0 {
		state.Reason = ""
	}
	return state
}

// The agent kinds this package recognises from the socket path alone. Nothing
// here probes a vendor's API: a socket path and the platform are the whole
// evidence, and an agent that does not match is reported as unknown rather than
// guessed at.
const (
	agentKindOpenSSH  = "OpenSSH ssh-agent"
	agentKindGPG      = "gpg-agent (SSH support)"
	agentKindOnepass  = "1Password"
	agentKindKeePass  = "KeePassXC"
	agentKindKeychain = "OpenSSH ssh-agent managed by the login keychain"
)

// agentKind names the agent a socket path belongs to, with what is known about
// its differences. The notes are the honest half of the report: they are why a
// user should not expect identical behaviour from every agent.
func agentKind(socket string) (string, []string) {
	lower := strings.ToLower(socket)
	switch {
	case strings.Contains(lower, "gnupg") || strings.Contains(lower, "gpg-agent"):
		return agentKindGPG, []string{
			"gpg-agent ignores ssh-add's lifetime option, so a key added here has gpg-agent's own expiry.",
			"gpg-agent decides from its own configuration what it serves, so a key removed here can be offered again later.",
		}
	case strings.Contains(lower, "1password") || strings.Contains(lower, "onepassword"):
		return agentKindOnepass, []string{
			"1Password serves the keys it owns, so ssh-add may report that a key is already there and may refuse to remove one.",
		}
	case strings.Contains(lower, "keepassxc"):
		return agentKindKeePass, []string{
			"KeePassXC serves the keys its own database holds, so adding and removing through ssh-add can be refused.",
		}
	case runtime.GOOS == "darwin" && (strings.Contains(lower, "keychain") || strings.Contains(lower, "com.apple.launchd")):
		return agentKindKeychain, []string{
			"The macOS agent keeps its identities in the login keychain: only ssh-add with the keychain option persists a key across logins, and this screen does not use it.",
		}
	case strings.Contains(lower, "ssh-agent") || agentSocketName(lower):
		return agentKindOpenSSH, nil
	}
	return "unknown agent at " + socket, []string{
		"This agent is not one this tool can describe, so the only operations it will attempt are the ones ssh-add itself supports.",
	}
}

// agentSocketName reports whether a socket path has the shape OpenSSH's agent
// creates: a directory named ssh-* holding agent.<pid>.
func agentSocketName(socket string) bool {
	dir := filepath.Dir(socket)
	base := filepath.Base(socket)
	return strings.HasPrefix(filepath.Base(dir), "ssh-") && strings.HasPrefix(base, "agent.")
}

// parseAgentList reads `ssh-add -l` output. Each line is `<bits> <fingerprint>
// <comment> (<TYPE>)`, and a comment may contain spaces -- so the comment is
// everything between the fingerprint and the parenthesised type, which is why
// this is parsed rather than split into fields.
func parseAgentList(output string) []AgentIdentity {
	var identities []AgentIdentity
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		var identity AgentIdentity
		rest := line
		if bits, remainder, ok := cutField(rest); ok {
			if n, err := strconv.Atoi(bits); err == nil {
				identity.Bits = n
				rest = remainder
			}
		}
		if fingerprint, remainder, ok := cutField(rest); ok {
			identity.Fingerprint = fingerprint
			rest = remainder
		}
		if strings.HasSuffix(rest, ")") {
			if open := strings.LastIndex(rest, "("); open >= 0 {
				identity.Type = rest[open+1 : len(rest)-1]
				rest = strings.TrimSpace(rest[:open])
			}
		}
		identity.Comment = strings.TrimSpace(rest)
		identities = append(identities, identity)
	}
	return identities
}

// cutField splits the first whitespace-separated field off a line.
func cutField(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return "", "", false
	}
	if index := strings.IndexAny(s, " \t"); index >= 0 {
		return s[:index], strings.TrimLeft(s[index+1:], " \t"), true
	}
	return s, "", true
}

// ============================================================================
// Adding and removing
// ============================================================================

// AddToAgent hands one key to the agent through ssh-add.
//
// The passphrase travels on the child's standard input and nowhere else: it is
// not an argument, and it is not an environment value. The child is detached
// from the controlling terminal so ssh-add reads the pipe instead of the
// terminal the interface is drawing on, and it is told there is no graphical
// prompter so it cannot fall back to one and silently take an empty passphrase.
func (m *Manager) AddToAgent(ctx context.Context, keyPath, passphrase string, lifetime time.Duration) AgentAction {
	if m.Caps.Add == "" {
		return AgentAction{Explanation: "ssh-add was not found on PATH, so no key can be added to the agent."}
	}
	if strings.TrimSpace(keyPath) == "" {
		return AgentAction{Explanation: "no key was named, so nothing was added."}
	}
	if _, err := os.Stat(keyPath); err != nil {
		return AgentAction{Explanation: fmt.Sprintf("the key %s could not be read, so nothing was added.", keyPath)}
	}

	args := []string{}
	if lifetime > 0 {
		args = append(args, "-t", strconv.Itoa(int(lifetime.Seconds())))
	}
	args = append(args, keyPath)
	command := Command{Name: m.Caps.Add, Args: args}
	if passphrase != "" {
		command.Stdin = passphrase + "\n"
		command.Detach = true
		command.Env = map[string]string{"DISPLAY": "", "SSH_ASKPASS": askPassDisabled}
	}

	res := m.Run.Run(ctx, command)
	action := AgentAction{Args: append([]string{m.Caps.Add}, args...), OK: res.OK()}
	if res.OK() {
		action.Output = firstLine(res.Stdout)
		action.Explanation = fmt.Sprintf("The identity %s was added to the agent.", filepath.Base(keyPath))
		return action
	}
	action.Output = res.Message()
	action.Explanation = "The agent refused the key: " + agentRefusal(res, keyPath)
	return action
}

// RemoveFromAgent removes one identity through ssh-add.
//
// ssh-add removes an identity by the public key file that describes it. A
// fingerprint is deliberately refused here rather than translated into a
// lookup: the translation would be this package deciding which file the agent
// meant, and the file it guessed could belong to a different key.
func (m *Manager) RemoveFromAgent(ctx context.Context, pubPathOrFingerprint string) AgentAction {
	argument := strings.TrimSpace(pubPathOrFingerprint)
	if argument == "" {
		return AgentAction{Explanation: "no identity was named, so nothing was removed."}
	}
	if looksLikeFingerprint(argument) {
		return AgentAction{Explanation: "ssh-add removes an identity by its public key file, not by a fingerprint, so nothing was removed."}
	}
	if m.Caps.Add == "" {
		return AgentAction{Explanation: "ssh-add was not found on PATH, so no identity can be removed."}
	}
	if _, err := os.Stat(argument); err != nil {
		return AgentAction{Explanation: fmt.Sprintf("the public key file %s could not be read, so nothing was removed.", argument)}
	}

	args := []string{"-d", argument}
	res := m.Run.Run(ctx, Command{Name: m.Caps.Add, Args: args})
	action := AgentAction{Args: append([]string{m.Caps.Add}, args...), OK: res.OK()}
	if res.OK() {
		action.Output = firstLine(res.Stdout)
		action.Explanation = fmt.Sprintf("The identity described by %s was removed from the agent.", filepath.Base(argument))
		return action
	}
	action.Output = res.Message()
	action.Explanation = "The agent refused the removal: " + agentRefusal(res, argument)
	return action
}

// RemoveAllFromAgent removes every identity the agent holds. It is the one
// action here that is not aimed at a single key, which is why the interface
// asks for it twice.
func (m *Manager) RemoveAllFromAgent(ctx context.Context) AgentAction {
	if m.Caps.Add == "" {
		return AgentAction{Explanation: "ssh-add was not found on PATH, so no identity can be removed."}
	}
	args := []string{"-D"}
	res := m.Run.Run(ctx, Command{Name: m.Caps.Add, Args: args})
	action := AgentAction{Args: append([]string{m.Caps.Add}, args...), OK: res.OK()}
	if res.OK() {
		action.Output = firstLine(res.Stdout)
		action.Explanation = "Every identity was removed from the agent. The keys themselves are untouched on disk."
		return action
	}
	action.Output = res.Message()
	action.Explanation = "The agent refused the removal: " + agentRefusal(res, "every identity")
	return action
}

// agentRefusal turns a failed ssh-add into a sentence a person can act on. The
// tool's own words come first: they name the reason, and this package only adds
// the one interpretation that is certain.
func agentRefusal(res Result, subject string) string {
	message := messageSummary(res)
	switch {
	case message == "":
		return fmt.Sprintf("ssh-add exited %d without a message.", res.Code)
	case strings.Contains(message, "Could not open a connection"):
		return message + " The agent is advertised but not answering, so nothing was changed."
	case strings.Contains(message, "incorrect passphrase"), strings.Contains(message, "Bad passphrase"), strings.Contains(message, "Passphrase"):
		return message + " The passphrase was not accepted, and it is not stored anywhere."
	case strings.Contains(message, "identity file"), strings.Contains(message, "Could not add identity"):
		return message + " The key could not be read by the agent."
	case strings.Contains(message, "no such identity"), strings.Contains(message, "not found in"):
		return message + " The agent does not hold an identity matching " + subject + "."
	default:
		return message
	}
}

// askPassDisabled is the path handed to a child as SSH_ASKPASS. It cannot exist,
// so an attempt to run a graphical prompter fails and the child falls back to
// the pipe this package controls. Without it, a host with an askpass program
// installed could answer the prompt with an empty string and ssh-keygen or
// ssh-add would silently accept a key with no passphrase.
const askPassDisabled = "/nonexistent/dotfiles-askpass-disabled"

// looksLikeFingerprint reports whether an argument is a fingerprint rather than
// a path. A path contains a separator or a dot; a fingerprint is either the
// SHA256 form or a hex MD5 with colons.
func looksLikeFingerprint(argument string) bool {
	if strings.HasPrefix(argument, "SHA256:") || strings.HasPrefix(argument, "MD5:") {
		return true
	}
	if strings.ContainsAny(argument, "/\\") || strings.HasPrefix(argument, "~") {
		return false
	}
	if idx := strings.Index(argument, ":"); idx > 0 {
		return true
	}
	return false
}

// ErrNoAgent is what an operation returns when this environment advertises no
// agent at all. It carries the environment's own name for the variable, so a
// report can say which setting was missing.
var ErrNoAgent = errors.New("no SSH agent is advertised in this environment (SSH_AUTH_SOCK is unset)")
