package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

const (
	// gaiStepID is the installation step that raises the RFC 3484 precedence of
	// IPv4-mapped addresses, so getaddrinfo(3) returns IPv4 first.
	gaiStepID = "gai"

	// envGAIConfPath overrides the gai.conf destination. It exists for tests and
	// for distributions that keep the file elsewhere, exactly as
	// DOTFILES_WSL_CONF_PATH does for /etc/wsl.conf.
	envGAIConfPath = "DOTFILES_GAI_CONF_PATH"

	// defaultGAIConfPath is where glibc reads its address-selection policy.
	defaultGAIConfPath = "/etc/gai.conf"

	// gaiMarkerStart and gaiMarkerEnd delimit the block the installer owns. When
	// the start marker is already present the block is replaced in place, so a
	// second run leaves the file byte for byte the same (S8).
	gaiMarkerStart = "# >>> dotfiles: prefer IPv4 (gai) >>>"
	gaiMarkerEnd   = "# <<< dotfiles: prefer IPv4 (gai) <<<"

	// gaiIPv4MappedPrefix and gaiIPv4MappedValue are the deciding precedence:
	// raising IPv4-mapped addresses above every other precedence is the
	// documented way to make getaddrinfo(3) return IPv4 first. The line is quoted
	// from the owner's /etc/gai.conf (odd/tasks/gai-preference.md, S2).
	gaiIPv4MappedPrefix = "::ffff:0:0/96"
	gaiIPv4MappedValue  = "100"

	// gaiPrecedenceLine is the deciding line exactly as the owner's file holds it
	// (S2).
	gaiPrecedenceLine = "precedence " + gaiIPv4MappedPrefix + " " + gaiIPv4MappedValue
)

// gaiLog is the step's log sink. It is a variable so a test can capture what the
// step reports.
var gaiLog = SendLog

// gaiDpkgAvailable reports whether the package tool that owns /etc/gai.conf is
// present. It is a variable so a test can exercise both answers without
// depending on the machine it runs on.
var gaiDpkgAvailable = func() bool { return system.CommandExists("dpkg") }

// gaiDpkgOwner runs `dpkg -S <path>` and returns its output. It is a variable so
// a test can substitute the lookup instead of spawning dpkg. The step never
// writes through it: it only asks which package owns the file (S3).
var gaiDpkgOwner = func(dst string) (string, error) {
	out, err := exec.Command("dpkg", "-S", dst).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// gaiBlockLines is the marked block the installer appends or replaces: a marker
// pair, a comment naming where it came from, and the deciding precedence line.
//
// It holds only the deciding line. The other precedence lines the owner's file
// holds are the RFC 3484 defaults that gai.conf documents and glibc already
// applies, so restating them would write the default back as though the
// installer had chosen it (S2, S6).
func gaiBlockLines() []string {
	return []string{
		gaiMarkerStart,
		"# Managed by the dotfiles installer. The line below raises IPv4-mapped",
		"# addresses above every other precedence, which is how getaddrinfo(3)",
		"# returns IPv4 first. The rest of the precedence table gai.conf",
		"# documents is the RFC 3484 default glibc already applies, so it is not",
		"# restated here. Remove this block to undo the change.",
		gaiPrecedenceLine,
		gaiMarkerEnd,
	}
}

// gaiBlock is gaiBlockLines as the bytes written to the file, ending in a
// newline so appending it never joins it to the previous line.
func gaiBlock() []byte {
	return []byte(strings.Join(gaiBlockLines(), "\n") + "\n")
}

// gaiDecisionKind is what the step will do to the file it found.
type gaiDecisionKind int

const (
	// gaiAppend adds the block to a file with no marker and no precedence the
	// owner set.
	gaiAppend gaiDecisionKind = iota
	// gaiReplace rewrites the marked block in place.
	gaiReplace
	// gaiAlreadyIPv4 leaves a file whose uncommented precedence already makes
	// IPv4 first.
	gaiAlreadyIPv4
	// gaiForeign leaves a file whose uncommented precedence is the owner's.
	gaiForeign
)

// gaiDecision is the step's answer for one file: what it will do, the bytes to
// write when it writes, and the foreign line to name when it does not.
type gaiDecision struct {
	kind   gaiDecisionKind
	result []byte
	found  string
}

// gaiDecide reads the file the step found and says what to do with it, without
// writing anything. Splitting and joining on "\n" is byte-preserving, so the
// bytes outside the block come back exactly as they went in.
func gaiDecide(existing []byte) gaiDecision {
	lines := strings.Split(string(existing), "\n")

	if start, end, ok := gaiMarkerSpan(lines); ok {
		return gaiDecision{kind: gaiReplace, result: gaiReplaceSpan(lines, start, end)}
	}

	found, deciding, example := gaiUncommentedPrecedence(lines)
	if deciding {
		return gaiDecision{kind: gaiAlreadyIPv4, found: example}
	}
	if found {
		return gaiDecision{kind: gaiForeign, found: example}
	}

	return gaiDecision{kind: gaiAppend, result: gaiAppendBlock(existing)}
}

// gaiMarkerSpan finds the line the block starts on and the line it ends on. A
// start marker without an end marker runs to the end of the file, so a
// half-written block is still replaced rather than duplicated.
func gaiMarkerSpan(lines []string) (start, end int, ok bool) {
	start, end = -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 && trimmed == gaiMarkerStart {
			start = i
			continue
		}
		if start >= 0 && trimmed == gaiMarkerEnd {
			end = i
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	if end < 0 {
		end = len(lines) - 1
	}
	return start, end, true
}

// gaiReplaceSpan swaps the marked lines for the canonical block, leaving every
// byte outside the span untouched.
func gaiReplaceSpan(lines []string, start, end int) []byte {
	replaced := make([]string, 0, len(lines)+len(gaiBlockLines()))
	replaced = append(replaced, lines[:start]...)
	replaced = append(replaced, gaiBlockLines()...)
	replaced = append(replaced, lines[end+1:]...)
	return []byte(strings.Join(replaced, "\n"))
}

// gaiAppendBlock puts the block at the end of the file. A file that does not end
// in a newline gets one first so the block starts on its own line; nothing else
// changes.
func gaiAppendBlock(existing []byte) []byte {
	result := make([]byte, 0, len(existing)+len(gaiBlock())+1)
	result = append(result, existing...)
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}
	return append(result, gaiBlock()...)
}

// gaiUncommentedPrecedence scans the file for precedence lines the owner left
// uncommented. deciding is true when one of them already makes IPv4 first, found
// is true when any uncommented precedence exists, and example is the first one,
// so a report can name what it found.
//
// The whole file is scanned before the answer is given: the owner's file carries
// the RFC 3484 defaults before the deciding line, and stopping at the first
// precedence line would report those as a foreign choice (S2, S7).
func gaiUncommentedPrecedence(lines []string) (found, deciding bool, example string) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 || fields[0] != "precedence" {
			continue
		}
		found = true
		if len(fields) >= 3 && fields[1] == gaiIPv4MappedPrefix && fields[2] == gaiIPv4MappedValue {
			deciding = true
			continue
		}
		if example == "" {
			example = trimmed
		}
	}
	return found, deciding, example
}

// installGAIConf applies the IPv4-first preference to dst through the same
// writer the WSL step uses: applyArtifactContent backs up the file it replaces
// and verifies what it wrote (S11). The step reads the file first and decides,
// so a file that already prefers IPv4, or one whose precedence the owner set, is
// never rewritten (S7, S8).
func installGAIConf(dst, stepID string) error {
	existing, err := os.ReadFile(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	decision := gaiDecide(existing)
	switch decision.kind {
	case gaiAlreadyIPv4:
		gaiLog(stepID, fmt.Sprintf("%s already prefers IPv4: %s is uncommented, so the installer leaves the file unchanged.", dst, gaiPrecedenceLine))
		return nil
	case gaiForeign:
		gaiLog(stepID, fmt.Sprintf("%s already sets a precedence of its own (%q). The installer leaves it alone: overriding a precedence the owner chose is his decision, not this run's.", dst, decision.found))
		return nil
	}

	// A file that already holds the canonical block is left untouched rather than
	// rewritten: a second run must leave it byte for byte the same, and rewriting
	// it would only add another backup (S8).
	if bytes.Equal(existing, decision.result) {
		gaiLog(stepID, fmt.Sprintf("%s already holds the dotfiles IPv4-first block; leaving it unchanged.", dst))
		return nil
	}

	if err := applyArtifactContent(decision.result, dst, stepID); err != nil {
		return err
	}
	if decision.kind == gaiReplace {
		gaiLog(stepID, fmt.Sprintf("Updated the dotfiles IPv4-first block in %s", dst))
	} else {
		gaiLog(stepID, fmt.Sprintf("Added the dotfiles IPv4-first block to %s", dst))
	}
	return nil
}

// stepInstallGAIConf is the gai step: it makes the machine prefer IPv4 by
// raising the precedence of IPv4-mapped addresses, and reports what a future
// libc-bin upgrade may do to the file it edited. The report never fails the
// step (S3), and the file itself is only written through the shared writer
// (S11).
func stepInstallGAIConf(_ *Model) error {
	dst := os.Getenv(envGAIConfPath)
	if dst == "" {
		dst = defaultGAIConfPath
	}

	if err := installGAIConf(dst, gaiStepID); err != nil {
		return wrapStepError(gaiStepID, "Prefer IPv4", "Failed to configure "+dst, err)
	}

	reportGAIPackage(dst, gaiStepID)
	return nil
}

// reportGAIPackage says which package owns the file and what a future upgrade of
// that package may do to it. It reads only: it asks `dpkg -S` when dpkg is there
// and never touches dpkg's state, installs nothing and records nothing (S3). It
// never returns an error, so a lookup that is missing or fails cannot fail the
// run.
func reportGAIPackage(dst, stepID string) {
	if !gaiDpkgAvailable() {
		gaiLog(stepID, fmt.Sprintf("dpkg is not available, so the installer cannot say which package owns %s; a dpkg-managed file can be replaced or queried about by a future upgrade.", dst))
		return
	}

	out, err := gaiDpkgOwner(dst)
	if err != nil || out == "" {
		gaiLog(stepID, fmt.Sprintf("`dpkg -S %s` did not answer, so the installer cannot say which package owns it; if it is a conffile, a future upgrade may replace it or ask about the change.", dst))
		return
	}

	owner := gaiPackageName(out)
	if owner == "" {
		gaiLog(stepID, fmt.Sprintf("`dpkg -S %s` answered %q, which names no package.", dst, out))
		return
	}

	gaiLog(stepID, fmt.Sprintf("%s belongs to %s.", dst, owner))
	gaiLog(stepID, fmt.Sprintf("A future `apt upgrade` of %s treats the file as a conffile: if the package ships a new version and this file changed, dpkg asks, and a question during an unattended upgrade blocks it. If the package replaces the file, this IPv4 preference can be lost. The installer never touches dpkg's state; the backup the writer logs is the way back.", owner))
}

// gaiPackageName pulls the package name out of a `dpkg -S` line, which reads
// "libc-bin: /etc/gai.conf". Multiple owners are reported one per line; the
// first is named.
func gaiPackageName(out string) string {
	line := out
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if i := strings.IndexByte(line, ':'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}
