package sshcfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ============================================================================
// THE DIAGNOSIS
// ============================================================================
//
// The diagnosis reads and reports. It opens no file for writing, applies no
// recommended edit and changes nothing: a finding names the problem, the exact
// place and, when there is a single obvious command, the one line that fixes
// it. The decision always stays with the user.
//
// A private key is never opened, parsed or copied. Only its name and its mode
// are read, which is all that a permission problem needs.

// Severity ranks a finding by how much it matters. The order is also the order
// the report is drawn in: a problem before a warning before an information.
type Severity int

const (
	// SeverityInfo is a fact worth knowing that breaks nothing.
	SeverityInfo Severity = iota
	// SeverityWarning is something that weakens a guarantee or will surprise
	// the user later.
	SeverityWarning
	// SeverityProblem is something that is wrong now, or that the client will
	// refuse.
	SeverityProblem
)

// String is the lowercase word the interface prints.
func (s Severity) String() string {
	switch s {
	case SeverityProblem:
		return "problem"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// Finding is one thing the diagnosis noticed.
//
// Code is the stable identifier a screen and a test key on; the other text
// fields are the human wording and may be reworded freely. Fix is the single
// line that resolves it, or empty when resolving it is a decision rather than a
// command. File is empty when the finding is about the environment, and Line is
// zero when a line number makes no sense.
type Finding struct {
	Severity Severity
	Code     string
	Title    string
	Detail   string
	Fix      string
	File     string
	Line     int
}

// Diagnose reports the problems it can see in the user's SSH setup.
//
// The findings are ordered: problems first, then warnings, then information,
// and within one severity by Code, so two runs on the same machine produce the
// same report.
func (m *Manager) Diagnose(ctx context.Context) []Finding {
	if m == nil {
		return nil
	}
	var findings []Finding

	dirInfo, dirErr := os.Stat(m.Paths.SSHDir)
	switch {
	case dirErr != nil && os.IsNotExist(dirErr):
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Code:     "no-ssh-directory",
			Title:    "No ~/.ssh directory",
			Detail: fmt.Sprintf("There is no %s directory yet. Nothing is broken: the manager creates it with permissions 0700 when it first writes a key or an entry.",
				m.Paths.SSHDir),
		})
		// With no directory the config and known_hosts are missing too, and
		// the finding above already says so. Reporting them separately would
		// only repeat the same fact three times.
	case dirErr != nil:
		findings = append(findings, Finding{
			Severity: SeverityProblem,
			Code:     "ssh-dir-unreadable",
			Title:    "SSH directory cannot be read",
			Detail:   fmt.Sprintf("The directory %s could not be read: %v.", m.Paths.SSHDir, dirErr),
			File:     m.Paths.SSHDir,
		})
	default:
		findings = append(findings, diagDirPermissionFindings(m.Paths.SSHDir, dirInfo.Mode())...)
		findings = append(findings, diagKeyPermissionFindings(m.Paths.SSHDir)...)
		if _, err := os.Stat(m.Paths.KnownHosts); err != nil && os.IsNotExist(err) {
			findings = append(findings, Finding{
				Severity: SeverityInfo,
				Code:     "known-hosts-missing",
				Title:    "known_hosts is missing",
				Detail:   "There is no known_hosts file yet: the first connection to a host will ask you to verify its key.",
				File:     m.Paths.KnownHosts,
			})
		}
		findings = append(findings, m.diagConfig(ctx)...)
	}

	// WSL is a statement about what this tool does and not a probe. The
	// Windows-side files under /mnt/c belong to a different system, so they are
	// neither stat-ed nor listed nor opened here: there is nothing to probe,
	// because the finding is that the report deliberately stays on the WSL side.
	if m.Paths.WSL {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Code:     "wsl-separate-config",
			Title:    "WSL has its own configuration",
			Detail:   "This process runs inside WSL, so this report reads and writes only the WSL side of the configuration. The Windows-side files under /mnt/c belong to a different system and are not opened, listed or mixed in.",
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity > findings[j].Severity
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}

// diagDirPermissionFindings checks the mode of the ~/.ssh directory. A write
// bit for the group or for others is a problem; a read bit alone only costs
// privacy.
func diagDirPermissionFindings(dir string, mode os.FileMode) []Finding {
	perm := mode.Perm()
	if perm&0o022 != 0 {
		return []Finding{{
			Severity: SeverityProblem,
			Code:     "ssh-dir-permissions",
			Title:    "SSH directory is writable by others",
			Detail:   fmt.Sprintf("The directory %s has permissions %04o; another user can add, replace or remove files there.", dir, int(perm)),
			Fix:      "chmod 700 ~/.ssh",
			File:     dir,
		}}
	}
	if perm&0o044 != 0 {
		return []Finding{{
			Severity: SeverityInfo,
			Code:     "ssh-dir-readable-by-others",
			Title:    "SSH directory is readable by others",
			Detail:   fmt.Sprintf("The directory %s has permissions %04o; another user can list it. That is not a lock-out, but it is not private either.", dir, int(perm)),
			Fix:      "chmod 700 ~/.ssh",
			File:     dir,
		}}
	}
	return nil
}

// diagKeyPermissionFindings reports every private key in dir that another user
// could read.
//
// The client offers no way to recognise a private key without reading it, and
// this package never reads one. The heuristic is the client's own naming --
// id_*, *.pem, *_rsa, *_ed25519, *_ecdsa, *.key -- plus a file that has a .pub
// sibling, which is the convention the client itself follows. A file matching
// neither is left alone rather than guessed at.
func diagKeyPermissionFindings(dir string) []Finding {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []Finding{{
			Severity: SeverityProblem,
			Code:     "ssh-dir-unreadable",
			Title:    "SSH directory cannot be read",
			Detail:   fmt.Sprintf("The directory %s could not be read: %v.", dir, err),
			File:     dir,
		}}
	}
	var findings []Finding
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// The public half is meant to be readable, so it is never a finding
		// on its own. The private file it belongs to is checked on its own
		// pass through the directory.
		if strings.HasSuffix(name, ".pub") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if !diagLooksLikeKey(name) && !diagHasPubSibling(dir, name) {
			continue
		}
		perm := info.Mode().Perm()
		if perm&0o077 == 0 {
			continue
		}
		full := filepath.Join(dir, name)
		findings = append(findings, Finding{
			Severity: SeverityProblem,
			Code:     "private-key-permissions",
			Title:    "Private key is readable by others",
			Detail:   fmt.Sprintf("The private key %s has permissions %04o; another user can read it.", full, int(perm)),
			Fix:      fmt.Sprintf("chmod 600 %s", full),
			File:     full,
		})
	}
	return findings
}

// diagLooksLikeKey applies the client's own naming conventions. It reads a name
// and nothing else.
func diagLooksLikeKey(name string) bool {
	if strings.HasPrefix(name, "id_") {
		return true
	}
	for _, suffix := range []string{".pem", "_rsa", "_ed25519", "_ecdsa", ".key"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// diagHasPubSibling reports whether name has a regular `<name>.pub` beside it,
// which is how the client marks a private key.
func diagHasPubSibling(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name+".pub"))
	return err == nil && info.Mode().IsRegular()
}

// diagConfig checks the user's client configuration: its permissions, its
// content through a minimal scan, and its syntax through the client itself.
func (m *Manager) diagConfig(ctx context.Context) []Finding {
	info, err := os.Stat(m.Paths.UserConfig)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []Finding{{
			Severity: SeverityProblem,
			Code:     "config-unreadable",
			Title:    "SSH config cannot be read",
			Detail:   fmt.Sprintf("The configuration file %s could not be read: %v.", m.Paths.UserConfig, err),
			File:     m.Paths.UserConfig,
		}}
	}

	var findings []Finding
	perm := info.Mode().Perm()
	// The client refuses to use a config file that the group or others can
	// write, so this is a problem rather than a warning.
	if perm&0o022 != 0 {
		findings = append(findings, Finding{
			Severity: SeverityProblem,
			Code:     "config-permissions",
			Title:    "SSH config is writable by others",
			Detail:   fmt.Sprintf("The configuration file %s has permissions %04o; the client refuses to use a file that other users can write.", m.Paths.UserConfig, int(perm)),
			Fix:      fmt.Sprintf("chmod 600 %s", m.Paths.UserConfig),
			File:     m.Paths.UserConfig,
		})
	}
	// No read bit means neither the client nor this report can read the file.
	// The mode answers the question even for a process that could read it.
	if perm&0o444 == 0 {
		findings = append(findings, Finding{
			Severity: SeverityProblem,
			Code:     "config-unreadable",
			Title:    "SSH config cannot be read",
			Detail:   fmt.Sprintf("The configuration file %s has permissions %04o; it has no read bit, so it cannot be read.", m.Paths.UserConfig, int(perm)),
			File:     m.Paths.UserConfig,
		})
		return findings
	}

	// The scan is deliberately its own and deliberately minimal. It finds the
	// directives the findings are about and never normalises or rewrites
	// anything, so the diagnosis does not depend on the package's parser.
	scan := newDiagScan(m)
	scan.scan(m.Paths.UserConfig)
	findings = append(findings, scan.findings...)

	// `ssh -G -F <file> localhost` parses the whole file exactly the way the
	// client will and exits non-zero on the first error it finds. That is the
	// reliable mechanism for syntax, and this package does not attempt to
	// reimplement the client's parser: a second parser would only disagree with
	// the client about the cases that matter.
	if m.Caps.HasSSH() {
		run := m.Run
		if run == nil {
			run = ExecRunner{}
		}
		res := run.Run(ctx, Command{Name: m.Caps.SSH, Args: []string{"-G", "-F", m.Paths.UserConfig, "localhost"}})
		if !res.OK() {
			detail := res.Message()
			if detail == "" {
				detail = fmt.Sprintf("ssh -G exited with status %d.", res.Code)
			}
			findings = append(findings, Finding{
				Severity: SeverityProblem,
				Code:     "config-syntax",
				Title:    "SSH config has a syntax error",
				Detail:   detail,
				File:     m.Paths.UserConfig,
			})
		}
	} else {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Code:     "config-syntax-unchecked",
			Title:    "SSH config was not checked",
			Detail:   "The ssh client was not found, so the configuration file was not checked for syntax errors.",
			File:     m.Paths.UserConfig,
		})
	}
	return findings
}

// ============================================================================
// THE MINIMAL SCAN
// ============================================================================

// diagSite is one occurrence of an alias in a configuration file.
type diagSite struct {
	file string
	line int
}

// diagScan is the diagnosis' own minimal read of the configuration files. It
// understands exactly the directives the findings are about -- Host, Include,
// IdentityFile, StrictHostKeyChecking, UserKnownHostsFile, Match -- and nothing
// else. It never writes and never normalises.
type diagScan struct {
	m        *Manager
	findings []Finding
	aliases  map[string]diagSite
	seen     map[string]bool
}

func newDiagScan(m *Manager) *diagScan {
	return &diagScan{
		m:       m,
		aliases: map[string]diagSite{},
		seen:    map[string]bool{},
	}
}

func (s *diagScan) add(f Finding) { s.findings = append(s.findings, f) }

// scan reads one file and the files it includes. A file is read once, so a
// cycle between two Include directives cannot loop.
func (s *diagScan) scan(path string) {
	path = filepath.Clean(path)
	if s.seen[path] {
		return
	}
	s.seen[path] = true

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		s.add(Finding{
			Severity: SeverityProblem,
			Code:     "config-unreadable",
			Title:    "SSH config cannot be read",
			Detail:   fmt.Sprintf("The configuration file %s could not be read: %v.", path, err),
			File:     path,
		})
		return
	}

	// A wildcard stanza applies to many hosts rather than to one, so a missing
	// IdentityFile inside it is not something this check can pin on a host. The
	// flag is set by the Host line and read by IdentityFile below.
	stanzaHasWildcard := false

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keyword, rest := diagSplitKeyword(line)
		switch strings.ToLower(keyword) {
		case "host":
			stanzaHasWildcard = false
			for _, pattern := range diagFields(rest) {
				if diagIsPattern(pattern) {
					stanzaHasWildcard = true
					continue
				}
				if first, ok := s.aliases[pattern]; ok {
					s.add(Finding{
						Severity: SeverityWarning,
						Code:     "alias-duplicate",
						Title:    "Duplicate alias",
						Detail: fmt.Sprintf("Alias %q is defined at %s:%d and again at %s:%d; the client stops at the first definition, so the second one never applies.",
							pattern, first.file, first.line, path, lineNo),
						File: path,
						Line: lineNo,
					})
					continue
				}
				s.aliases[pattern] = diagSite{file: path, line: lineNo}
			}
		case "include":
			s.scanInclude(path, lineNo, rest)
		case "identityfile":
			if stanzaHasWildcard {
				continue
			}
			for _, value := range diagFields(rest) {
				if diagPathExists(value, s.m.Paths.Home) {
					continue
				}
				s.add(Finding{
					Severity: SeverityWarning,
					Code:     "identity-missing",
					Title:    "Identity file is missing",
					Detail: fmt.Sprintf("The identity %q at %s:%d does not exist; the client will fall back to the default identities.",
						value, path, lineNo),
					File: path,
					Line: lineNo,
				})
			}
		case "stricthostkeychecking":
			value := strings.ToLower(diagFirstField(rest))
			if value == "no" || value == "accept-new" {
				s.add(diagHostKeyCheckFinding(path, lineNo, line))
			}
		case "userknownhostsfile":
			for _, value := range diagFields(rest) {
				if value == "/dev/null" {
					s.add(diagHostKeyCheckFinding(path, lineNo, line))
					break
				}
			}
		case "match":
			kind := strings.ToLower(diagFirstField(rest))
			if kind == "exec" || kind == "canonical" {
				s.add(Finding{
					Severity: SeverityInfo,
					Code:     "match-not-evaluated",
					Title:    "Match block not evaluated",
					Detail:   fmt.Sprintf("The block starting at %s:%d is evaluated by the client and not by this report.", path, lineNo),
					File:     path,
					Line:     lineNo,
				})
			}
		}
	}
}

// scanInclude expands one Include line. A pattern that matches no file is a
// warning of its own, because a configuration that includes nothing is almost
// always a filename that has moved or a directory that was never created.
func (s *diagScan) scanInclude(path string, line int, rest string) {
	for _, pattern := range diagFields(rest) {
		resolved := diagExpandTilde(pattern, s.m.Paths.Home)
		if !filepath.IsAbs(resolved) {
			// The client resolves a relative Include against ~/.ssh, not
			// against the directory of the file that names it.
			resolved = filepath.Join(s.m.Paths.UserConfigDir, resolved)
		}
		matches, _ := filepath.Glob(resolved)
		var files []string
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
				files = append(files, match)
			}
		}
		if len(files) == 0 {
			s.add(Finding{
				Severity: SeverityWarning,
				Code:     "include-empty",
				Title:    "Include matches nothing",
				Detail:   fmt.Sprintf("The Include pattern %q at %s:%d matches no file, so it is silently ignored.", pattern, path, line),
				File:     path,
				Line:     line,
			})
			continue
		}
		for _, file := range files {
			s.scan(file)
		}
	}
}

// diagHostKeyCheckFinding is what a directive that turns host verification off
// produces. Fix stays empty: switching verification off is the user's decision,
// and this tool never changes it back.
func diagHostKeyCheckFinding(file string, line int, directive string) Finding {
	return Finding{
		Severity: SeverityWarning,
		Code:     "host-key-check-disabled",
		Title:    "Host verification is disabled",
		Detail: fmt.Sprintf("The directive %q at %s:%d turns host verification off. A host that is impersonated would not be noticed. This tool never changes it.",
			directive, file, line),
		File: file,
		Line: line,
	}
}

// diagSplitKeyword splits a line into its first word and the rest.
func diagSplitKeyword(line string) (keyword, rest string) {
	if i := strings.IndexAny(line, " \t"); i >= 0 {
		return line[:i], strings.TrimSpace(line[i+1:])
	}
	return line, ""
}

// diagFirstField is the first word of a directive's value.
func diagFirstField(s string) string {
	fields := diagFields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// diagFields splits a value into words, keeping a double-quoted value together
// and dropping the quotes. The client treats a quoted argument as one value,
// and so does this scan.
func diagFields(s string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return fields
}

// diagIsPattern reports whether a Host token is a pattern rather than one
// literal alias. A negated or wildcard token belongs to no single host.
func diagIsPattern(pattern string) bool {
	if pattern == "" || strings.HasPrefix(pattern, "!") {
		return true
	}
	return strings.ContainsAny(pattern, "*?[")
}

// diagExpandTilde resolves a leading ~ to the home directory. It expands only
// the forms the client supports: ~ and ~/path.
func diagExpandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// diagPathExists answers whether an IdentityFile path points at something. It
// expands ~ for the check but the caller prints the path as the file wrote it.
func diagPathExists(path, home string) bool {
	resolved := diagExpandTilde(path, home)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(home, resolved)
	}
	_, err := os.Stat(resolved)
	return err == nil
}
