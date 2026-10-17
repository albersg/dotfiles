package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// GIT AND SSH
// ============================================================================
//
// Two things live here, and the brief is explicit about both. The first is the
// basics of git: which identity a commit would carry, whether a credential
// helper is configured (named, never read), and whether signing is on without
// the tool that signs. The second is the agent SSH keeps, and the permissions of
// the private material it uses.
//
//   - **No secret is read, and no credential store is opened.**
//     `credential.helper` is reported by name; a helper that is a shell command
//     is reported as a command and not printed, because a token can live inside
//     one. `~/.git-credentials` is reported by its presence and its mode and
//     never opened. The agent is asked how many identities it holds, never
//     which.
//   - **A local identity that differs from the global one is not a fault.**
//     It is the normal way to keep a work identity in one repository, and the
//     finding says so in as many words. The check that matters is an identity
//     that is missing or malformed, not one that is different.
//   - **This is not the SSH manager.** No key is generated, imported, added to
//     an agent or removed from one, and no configuration file is written. The
//     doctor reports modes and counts; the two fixes it can offer are the ones
//     in doctor_fix.go, and both are single file modes.

// doctorGitChecks is the git category's catalogue.
func doctorGitChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "git.binary",
			Category: doctorCategoryGit,
			Title:    "Git is installed and answers",
			Required: true,
			Run:      doctorCheckGitBinary,
		},
		{
			ID:       "git.identity",
			Category: doctorCategoryGit,
			Title:    "A commit would carry a name and an address",
			Run:      doctorCheckGitIdentity,
		},
		{
			ID:       "git.credentials",
			Category: doctorCategoryGit,
			Title:    "Credentials are reachable without typing them every time",
			Run:      doctorCheckGitCredentials,
		},
		{
			ID:       "git.sshauth",
			Category: doctorCategoryGit,
			Title:    "The SSH agent and the keys' permissions are sound",
			Run:      doctorCheckGitSSHAuth,
		},
	}
}

// doctorGitValue asks git for one configuration value. The key is a constant in
// this file, so nothing a user wrote is ever interpolated into the command.
func doctorGitValue(scan *doctorScan, args ...string) doctorCommandResult {
	lookup := scan.lookup("git")
	if !lookup.found() {
		return doctorCommandResult{Code: -1, Err: "git does not resolve on PATH"}
	}
	return scan.probe(doctorProbeTimeout, lookup.Path, args...)
}

// doctorCheckGitBinary answers whether git is there and which version it is. The
// tools check also names git, but that check is about the installer's managed
// set: this one is here so every git finding can lean on the answer instead of
// repeating "git is missing" six times.
func doctorCheckGitBinary(scan *doctorScan) []doctorFinding {
	lookup := scan.lookup("git")
	if !lookup.found() {
		return []doctorFinding{{
			Subject:  "Git",
			Status:   doctorError,
			Evidence: "git does not resolve on PATH",
			Impact:   "No repository can be cloned, inspected or committed to; the agent skills, the dotfiles' own maintenance and every git alias in this configuration stop working.",
			Advice:   "Install git with the platform's package manager, or with `xcode-select --install` on macOS.",
		}}
	}
	version := scan.toolVersion(lookup.Path, "--version")
	if version.Line == "" {
		return []doctorFinding{{
			Subject: "Git",
			Status:  doctorError,
			Evidence: fmt.Sprintf("git resolves to %s and `git --version` reported nothing: %s",
				doctorRelHome(scan, lookup.Path), doctorVersionProblem(version)),
			Impact: "The binary is there and does not run, so every git command fails although the file exists.",
			Advice: "Reinstall git: a git that cannot print its version cannot clone or commit either.",
		}}
	}
	return []doctorFinding{{
		Subject:  "Git",
		Status:   doctorOK,
		Evidence: fmt.Sprintf("git %s at %s", doctorVersionNumber(version.Line), doctorRelHome(scan, lookup.Path)),
		Impact:   "The git commands this configuration uses can run.",
		Advice:   "Nothing to do.",
	}}
}

// doctorGitIdentity is what one of the two identity keys holds.
type doctorGitIdentity struct {
	// Global is the value in the user's own configuration, empty when unset.
	Global string
	// Local is the value in the repository this process stands in, empty when
	// there is no repository or the key is unset in it.
	Local string
	// LocalRepo is the repository root the local value came from.
	LocalRepo string
	// Err is why the global value could not be read.
	Err string
}

// doctorCheckGitIdentity answers what a commit would carry, and refuses to call
// a local identity a fault for differing from the global one.
//
// The two are read separately: the global value from `--global`, the local one
// from `--local` when this process stands inside a work tree. The difference
// between them is stated as the normal configuration it is, with the repository
// it applies to named; the states that are faults are an identity that is
// missing everywhere and an address that cannot be an address.
func doctorCheckGitIdentity(scan *doctorScan) []doctorFinding {
	if !scan.lookup("git").found() {
		return []doctorFinding{{
			Subject:  "The identity a commit carries",
			Status:   doctorNotApplicable,
			Evidence: "git does not resolve on PATH",
			Impact:   "Without git there is no identity to check.",
			Advice:   "Install git first; the git check above says how.",
		}}
	}

	name := doctorReadGitIdentity(scan, "user.name")
	email := doctorReadGitIdentity(scan, "user.email")

	var findings []doctorFinding
	missing := []string{}
	if name.Global == "" {
		missing = append(missing, "user.name")
	}
	if email.Global == "" {
		missing = append(missing, "user.email")
	}

	if len(missing) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  "The global git identity",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("`git config --global %s` answers nothing", strings.Join(missing, "` and --global ")),
			Impact:   "A commit made in a repository without its own identity is refused by git (or, on an old git, recorded with a name derived from the machine), so the first commit fails.",
			Advice:   "Set them once, with the address you want on your commits: `git config --global user.name \"Your Name\"` and `git config --global user.email \"you@example.com\"`.",
		})
	} else {
		evidence := fmt.Sprintf("global: %s <%s>", name.Global, email.Global)
		impact := "A commit made anywhere without a repository-local identity carries this name and address."
		findings = append(findings, doctorFinding{
			Subject:  "The global git identity",
			Status:   doctorOK,
			Evidence: evidence,
			Impact:   impact,
			Advice:   "Nothing to do.",
		})
	}

	// A local value that differs from the global one is the normal way to keep a
	// separate identity in one repository. The finding exists so the difference is
	// seen and not guessed at, and it is deliberately an ok.
	if name.Local != "" || email.Local != "" {
		where := name.LocalRepo
		if where == "" {
			where = email.LocalRepo
		}
		differs := name.Local != name.Global || email.Local != email.Global
		evidence := fmt.Sprintf("in %s: %s <%s>", doctorRelHome(scan, where), doctorOrGlobal(name.Local, name.Global), doctorOrGlobal(email.Local, email.Global))
		impact := "A commit made in this repository carries this identity instead of the global one, which is a normal configuration and not a fault."
		if !differs {
			impact = "This repository's identity is explicit and the same as the global one."
		}
		findings = append(findings, doctorFinding{
			Subject:  "The identity this repository would use",
			Status:   doctorOK,
			Evidence: evidence,
			Impact:   impact,
			Advice:   "Nothing to do. A repository-local identity that differs from the global one is deliberate; unset it there with `git config --local --unset user.email` if you did not mean it.",
		})
	}

	// The address has to be one. git accepts almost anything, so a value with no
	// @ is worth naming: it is what a typo looks like.
	for _, check := range []struct {
		label string
		value doctorGitIdentity
	}{{"user.email", email}} {
		if check.value.Global == "" {
			continue
		}
		if strings.Contains(check.value.Global, "@") && !strings.ContainsAny(check.value.Global, " \t") {
			continue
		}
		findings = append(findings, doctorFinding{
			Subject:  "The global " + check.label,
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s is %q, which is not an address git can send anything to", check.label, check.value.Global),
			Impact:   "Commits are recorded with an address no service can match to an account, and a host that checks the address refuses the push.",
			Advice:   "Set the address you want on your commits: `git config --global " + check.label + " \"you@example.com\"`.",
		})
	}

	// Signing that is on without the tool that signs: a verifiable
	// incompatibility, and the one the doctor can state without touching a key.
	sign := doctorGitConfigValue(scan, "commit.gpgsign")
	key := doctorGitConfigValue(scan, "user.signingkey")
	if strings.EqualFold(strings.TrimSpace(sign), "true") || key != "" {
		gpg := scan.lookup("gpg")
		if !gpg.found() {
			evidence := fmt.Sprintf("commit.gpgsign is %q", sign)
			if key != "" {
				evidence = fmt.Sprintf("user.signingkey is set and commit.gpgsign is %q", sign)
			}
			findings = append(findings, doctorFinding{
				Subject:  "Signed commits without the tool that signs",
				Status:   doctorWarning,
				Evidence: evidence + ", and gpg does not resolve on PATH",
				Impact:   "Every commit fails with a signing error, which reads as a broken repository rather than as a missing tool.",
				Advice:   "Install gpg, or turn signing off for this machine: `git config --global commit.gpgsign false`.",
			})
		} else {
			findings = append(findings, doctorFinding{
				Subject:  "Signed commits",
				Status:   doctorOK,
				Evidence: fmt.Sprintf("commit.gpgsign is %q and gpg resolves to %s", sign, doctorRelHome(scan, gpg.Path)),
				Impact:   "Commits that ask to be signed have a tool to sign them with.",
				Advice:   "Nothing to do.",
			})
		}
	}
	return findings
}

// doctorOrGlobal is a local identity value, or the global one when the local key
// is unset: a repository that sets only one of the two still has an identity.
func doctorOrGlobal(local, global string) string {
	if local != "" {
		return local
	}
	if global != "" {
		return global + " (global)"
	}
	return "unset"
}

// doctorReadGitIdentity reads one identity key from the global configuration and
// from the repository this process stands in.
func doctorReadGitIdentity(scan *doctorScan, key string) doctorGitIdentity {
	identity := doctorGitIdentity{
		Global: strings.TrimSpace(doctorGitConfigValue(scan, key)),
	}
	if repo := doctorGitRepositoryRoot(scan); repo != "" {
		identity.LocalRepo = repo
		result := doctorGitValue(scan, "-C", repo, "config", "--local", "--get", key)
		if result.Err == "" {
			identity.Local = strings.TrimSpace(result.Stdout)
		}
	}
	return identity
}

// doctorGitConfigValue asks for one global key. git exits 1 for a key that is
// not set, which is not an error to report: it is the "unset" state the callers
// classify themselves.
func doctorGitConfigValue(scan *doctorScan, key string) string {
	result := doctorGitValue(scan, "config", "--global", "--get", key)
	if result.Err != "" {
		return ""
	}
	return result.Stdout
}

// doctorGitRepositoryRoot is the work tree this process stands in, or the empty
// string when it stands outside one. The doctor runs from whatever directory
// started it, so this is what makes a repository-local identity answerable.
func doctorGitRepositoryRoot(scan *doctorScan) string {
	result := doctorGitValue(scan, "rev-parse", "--show-toplevel")
	if result.Err != "" {
		return ""
	}
	root := strings.TrimSpace(result.Stdout)
	if root == "" {
		return ""
	}
	return root
}

// doctorCheckGitCredentials answers how git would authenticate, without reading
// a credential: the helper's name, where a stored credential file is and what
// mode it has, and whether the machine has another route at all.
func doctorCheckGitCredentials(scan *doctorScan) []doctorFinding {
	if !scan.lookup("git").found() {
		return []doctorFinding{{
			Subject:  "How git authenticates",
			Status:   doctorNotApplicable,
			Evidence: "git does not resolve on PATH",
			Impact:   "Without git there is no authentication to check.",
			Advice:   "Install git first; the git check above says how.",
		}}
	}

	var findings []doctorFinding
	helper := strings.TrimSpace(doctorGitConfigValue(scan, "credential.helper"))
	switch {
	case helper == "":
		// No helper is not a fault: SSH is a legitimate way to authenticate. It is
		// worth saying only when there is no key either, because then an HTTPS
		// clone asks for a password on every operation.
		if keys := doctorSSHPrivateKeys(scan); len(keys) == 0 {
			findings = append(findings, doctorFinding{
				Subject:  "How git authenticates",
				Status:   doctorWarning,
				Evidence: "no credential.helper is configured and no private key is present in ~/.ssh",
				Impact:   "Cloning or pushing over HTTPS asks for the username and the password every time, which is what makes a scripted clone stop and wait.",
				Advice:   "Configure a helper (`git config --global credential.helper osxkeychain` on macOS, `... credential.helper store` or `cache` elsewhere), or create an SSH key and use SSH remotes.",
			})
		} else {
			findings = append(findings, doctorFinding{
				Subject: "How git authenticates",
				Status:  doctorOK,
				Evidence: fmt.Sprintf("no credential.helper is configured; %s are present in ~/.ssh",
					doctorCounted(len(keys), "private key", "private keys")),
				Impact: "Git authenticates over SSH with those keys; an HTTPS remote would still ask for a password.",
				Advice: "Nothing to do unless you use HTTPS remotes, in which case configure a helper.",
			})
		}
	case strings.HasPrefix(helper, "!"):
		// A helper that is a shell command can carry a token inside the command
		// itself, so the value is never printed.
		findings = append(findings, doctorFinding{
			Subject:  "How git authenticates",
			Status:   doctorOK,
			Evidence: "credential.helper is a custom command, which is reported by kind and never printed: a token can live inside one",
			Impact:   "Git runs that command to obtain a credential; the doctor reads nothing it produces.",
			Advice:   "Nothing to do; the doctor will not read what the helper returns.",
		})
	default:
		findings = append(findings, doctorFinding{
			Subject:  "How git authenticates",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("credential.helper is %q", helper),
			Impact:   "Git asks that helper for a credential instead of asking the terminal.",
			Advice:   "Nothing to do.",
		})
	}

	// The stored credential file is reported by presence and mode, never opened:
	// its content is the secret.
	store := filepath.Join(scan.home(), ".git-credentials")
	if scan.home() != "" {
		if info, err := scan.lstat(store); err == nil {
			status := doctorOK
			impact := "Git stores credentials in a file only its owner can read."
			advice := "Nothing to do."
			if info.Mode().Perm()&0o077 != 0 {
				status = doctorWarning
				impact = "A file holding credentials in clear text can be read by other users on this machine."
				advice = fmt.Sprintf("Take the group and other bits away: `chmod 600 %s`, or use a helper that keeps them in the keychain.", doctorRelHome(scan, store))
			}
			findings = append(findings, doctorFinding{
				Subject:  "The stored git credentials",
				Status:   status,
				Evidence: fmt.Sprintf("%s is present with mode %s (the file is not opened: its content is the credential)", doctorRelHome(scan, store), doctorPermString(info.Mode())),
				Impact:   impact,
				Advice:   advice,
			})
		}
	}
	return findings
}

// doctorSSHPrivateKeys is every private key this check is allowed to look at: a
// file in ~/.ssh whose name starts with id_ and does not end in .pub, or a file
// whose first bytes are a private-key header. Public material and configuration
// are excluded by name.
func doctorSSHPrivateKeys(scan *doctorScan) []string {
	sshDir := filepath.Join(scan.home(), ".ssh")
	if scan.home() == "" {
		return nil
	}
	entries, err := scan.readDir(sshDir)
	if err != nil {
		return nil
	}
	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".pub") || name == "config" || name == "known_hosts" ||
			strings.HasPrefix(name, "known_hosts.") || name == "authorized_keys" ||
			strings.HasSuffix(name, ".sock") {
			continue
		}
		path := filepath.Join(sshDir, name)
		if strings.HasPrefix(name, "id_") {
			keys = append(keys, path)
			continue
		}
		// A differently named private key is recognised by its own header, read
		// only far enough to see it. The file is never printed.
		if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".key") {
			continue
		}
		if header, err := scan.readFile(path); err == nil && strings.Contains(header, "PRIVATE KEY") {
			keys = append(keys, path)
		}
	}
	sort.Strings(keys)
	return keys
}

// doctorCheckGitSSHAuth reports the agent's state and the permissions of the
// private material, and offers the two mode fixes it can prove.
//
// The identities the agent holds are counted, never listed: `ssh-add -l` prints
// a fingerprint and a comment for each, and a comment is often an address.
// Whether the agent is running at all is a normal difference between sessions,
// so an unset SSH_AUTH_SOCK is stated rather than warned about.
func doctorCheckGitSSHAuth(scan *doctorScan) []doctorFinding {
	home := scan.home()
	if home == "" {
		return []doctorFinding{{
			Subject:  "SSH material",
			Status:   doctorUnknown,
			Evidence: "neither $HOME nor the platform's user lookup answered",
			Impact:   "The SSH directory and the keys in it cannot be looked at.",
			Advice:   "Set HOME and run the doctor again.",
		}}
	}

	sshDir := filepath.Join(home, ".ssh")
	info, err := scan.lstat(sshDir)
	if err != nil {
		return []doctorFinding{{
			Subject:  "SSH material",
			Status:   doctorNotApplicable,
			Evidence: fmt.Sprintf("%s is not there", doctorRelHome(scan, sshDir)),
			Impact:   "There is no SSH configuration or key on this machine, so there is nothing to check. A machine that uses HTTPS remotes does not need one.",
			Advice:   "Nothing to do unless you want SSH: `ssh-keygen -t ed25519` creates a key.",
		}}
	}

	var findings []doctorFinding
	mode := info.Mode().Perm()
	if !info.IsDir() {
		findings = append(findings, doctorFinding{
			Subject:  "The SSH directory",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s is not a directory (mode %s)", doctorRelHome(scan, sshDir), doctorPermString(info.Mode())),
			Impact:   "SSH cannot store or read anything at a path it expects to be a directory.",
			Advice:   "Move the file aside and create the directory again with mode 0700.",
		})
	} else if mode&0o077 != 0 {
		findings = append(findings, doctorFinding{
			Subject:  "The SSH directory is readable by others",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s has mode %s", doctorRelHome(scan, sshDir), doctorPermString(info.Mode())),
			Impact:   "OpenSSH refuses a private key whose directory others can read, and every ssh command fails with \"bad permissions\" until it is fixed.",
			Advice:   "The doctor can set it to 0700 for you, or run `chmod 700 ~/.ssh`.",
			Fix:      &doctorFix{Kind: doctorFixChmodDir, Path: sshDir, Mode: 0o700},
		})
	} else {
		findings = append(findings, doctorFinding{
			Subject:  "The SSH directory",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("%s has mode %s", doctorRelHome(scan, sshDir), doctorPermString(info.Mode())),
			Impact:   "OpenSSH accepts a key from this directory.",
			Advice:   "Nothing to do.",
		})
	}

	for _, key := range doctorSSHPrivateKeys(scan) {
		keyInfo, err := scan.lstat(key)
		if err != nil {
			continue
		}
		if keyInfo.Mode().Perm()&0o077 == 0 {
			continue
		}
		findings = append(findings, doctorFinding{
			Subject:  "A private key others can read",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s has mode %s", doctorRelHome(scan, key), doctorPermString(keyInfo.Mode())),
			Impact:   "Any user on this machine can copy the key, and OpenSSH itself refuses to use it until the mode is tightened.",
			Advice:   "The doctor can set it to 0600 for you, or run `chmod 600 " + doctorRelHome(scan, key) + "`.",
			Fix:      &doctorFix{Kind: doctorFixChmodFile, Path: key, Mode: 0o600},
		})
	}

	findings = append(findings, doctorSSHAgentFindings(scan)...)
	return findings
}

// doctorSSHAgentFindings reports the agent: whether the environment names one,
// whether that socket is still there, and how many identities it holds. The
// count is the only thing read from the agent.
func doctorSSHAgentFindings(scan *doctorScan) []doctorFinding {
	socket := scan.getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorOK,
			Evidence: "SSH_AUTH_SOCK is not set, so no agent is running in this environment",
			Impact:   "SSH uses the key file directly and asks for its passphrase once per session. That is normal, and a long-running shell session usually starts an agent itself.",
			Advice:   "Nothing to do; start one with `eval \"$(ssh-agent -s)\"` if you want the passphrase asked once.",
		}}
	}
	if _, err := scan.stat(socket); err != nil {
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("SSH_AUTH_SOCK names %s, which is not there (%v)", socket, err),
			Impact:   "Every ssh command fails with \"could not connect to the agent\", which is what a stale socket inherited from a closed session looks like.",
			Advice:   "Start a new agent in this shell, or unset SSH_AUTH_SOCK if this environment does not need one.",
		}}
	}

	lookup := scan.lookup("ssh-add")
	if !lookup.found() {
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("SSH_AUTH_SOCK names %s and ssh-add does not resolve on PATH", socket),
			Impact:   "Whether the agent holds a usable identity is unknown, because the command that would ask is not installed.",
			Advice:   "Install an OpenSSH client so the agent can be asked.",
		}}
	}
	result := scan.probe(doctorProbeTimeout, lookup.Path, "-l")
	switch {
	case result.Err == "":
		count := len(doctorNonEmptyLines(result.Stdout))
		return []doctorFinding{{
			Subject: "The SSH agent",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("SSH_AUTH_SOCK names %s and `ssh-add -l` reports %s (the identities themselves are not read)",
				socket, doctorCounted(count, "identity", "identities")),
			Impact: "Git and ssh can authenticate without asking for the key's passphrase again.",
			Advice: "Nothing to do.",
		}}
	case result.Code == 1:
		// ssh-add exits 1 for "the agent has no identities", which is a state and
		// not a failure.
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("SSH_AUTH_SOCK names %s and `ssh-add -l` reports no identities", socket),
			Impact:   "The agent is running and empty, so ssh still asks for the passphrase; a machine that relies on the agent for a scripted command will stop and wait.",
			Advice:   "Add the key you use: `ssh-add ~/.ssh/id_ed25519`.",
		}}
	case result.Code == 2:
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("SSH_AUTH_SOCK names %s and `ssh-add -l` cannot reach the agent", socket),
			Impact:   "The socket exists and nothing is listening on it, which every ssh command reports as a failure of the agent.",
			Advice:   "Start a new agent in this shell: `eval \"$(ssh-agent -s)\"`.",
		}}
	default:
		detail := doctorFirstLine(result.Stderr)
		if detail == "" {
			detail = result.Err
		}
		return []doctorFinding{{
			Subject:  "The SSH agent",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("`ssh-add -l` answered %s (exit status %d)", detail, result.Code),
			Impact:   "Whether the agent holds an identity could not be established.",
			Advice:   "Run `ssh-add -l` by hand in the same shell to see the answer.",
		}}
	}
}

// doctorNonEmptyLines counts the lines with content, used to count identities
// without reading them into the report.
func doctorNonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// doctorParseInt is a small helper for the capacity check, which reads a number
// out of a command's output.
func doctorParseInt(value string) (int, bool) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return number, true
}

// doctorModeIsPrivate reports whether a mode allows nobody but the owner to read
// or write, which is the property OpenSSH wants from a key.
func doctorModeIsPrivate(mode os.FileMode) bool {
	return mode.Perm()&0o077 == 0
}
