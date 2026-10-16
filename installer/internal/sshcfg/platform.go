package sshcfg

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
)

// detachSysProcAttr puts the child in its own session, with no controlling
// terminal.
//
// It is what makes a passphrase read from the Stdin pipe instead of from the
// terminal the interface is drawing on: ssh-keygen and ssh-add read a
// passphrase from the controlling terminal when they have one, and from the
// pipe when they do not. This package only builds for the platforms the
// installer ships (linux and darwin), where a session leader without a terminal
// cannot open one.
func detachSysProcAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// systemConfigPath is the system-wide client configuration the client reads
// after the user's own file.
func systemConfigPath() string {
	return "/etc/ssh/ssh_config"
}

// isWSL reports whether this process runs inside WSL. It reads only the two
// variables WSL itself sets, and never looks at /mnt/c: the Windows-side
// configuration belongs to a different system and this package does not open it.
func isWSL(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	return getenv("WSL_DISTRO_NAME") != "" || getenv("WSL_INTEROP") != ""
}

// lookupTool resolves one tool on PATH. It is a variable so a test can state
// which tools a host has without changing the machine's PATH.
var lookupTool = exec.LookPath

// The two version/key-type probes detectCaps makes.
const (
	versionArgsKey = "-V"
	keyQueryArgs   = "-Q"
	keyQueryTopic  = "key"
)

// detectCaps probes the client and its tools. Every probe is read-only: it asks
// for a version and the list of key types the client supports, never for a key,
// an agent identity or a connection.
func detectCaps(ctx context.Context, run Runner, paths Paths) Caps {
	caps := Caps{AgentSocket: paths.AgentSocket}
	if path, err := lookupTool("ssh"); err == nil {
		caps.SSH = path
	}
	if path, err := lookupTool("ssh-keygen"); err == nil {
		caps.Keygen = path
	}
	if path, err := lookupTool("ssh-add"); err == nil {
		caps.Add = path
	}

	if caps.SSH == "" {
		return caps
	}
	// `ssh -V` prints its version on standard error and exits zero. The line is
	// kept whole: it names the port and the library the client was built with,
	// which is what a report about a client should say.
	if res := run.Run(ctx, Command{Name: caps.SSH, Args: []string{versionArgsKey}}); res.Err == nil {
		caps.SSHVersion = firstLine(res.Stderr)
		if caps.SSHVersion == "" {
			caps.SSHVersion = firstLine(res.Stdout)
		}
	}
	if res := run.Run(ctx, Command{Name: caps.SSH, Args: []string{keyQueryArgs, keyQueryTopic}}); res.OK() {
		for _, line := range strings.Split(res.Output(), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				caps.KeygenSupports = append(caps.KeygenSupports, line)
			}
		}
	}
	return caps
}

// firstLine is the first non-empty line of a tool's output.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
