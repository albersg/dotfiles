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

// gaiApplyArtifactContent is the shared writer the step installs through (S11).
// It is a variable so a test can exercise a failed write -- a read-only /etc, a
// refused sudo -- without needing either.
var gaiApplyArtifactContent = applyArtifactContent

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
	// gaiForeign leaves a file whose uncommented precedence, outside the managed
	// block, is the owner's.
	gaiForeign
	// gaiMalformed leaves a file whose markers are not a shape the step owns:
	// half-written, duplicated, or out of order.
	gaiMalformed
)

// gaiDecision is the step's answer for one file: what it will do, the bytes to
// write when it writes, the foreign line to name when it does not, the reason a
// malformed file is left alone, and the precedence line the replacement restored.
type gaiDecision struct {
	kind     gaiDecisionKind
	result   []byte
	found    string
	reason   string
	replaced string
}

// gaiDecide reads the file the step found and says what to do with it, without
// writing anything. Splitting and joining on "\n" is byte-preserving, so the
// bytes outside the block come back exactly as they went in.
//
// The whole file is read before a decision, even when the marker is present: a
// precedence the owner set outside the managed block governs the file, and the
// step must not rewrite its own block over a choice it would then be hiding (F2).
func gaiDecide(existing []byte) gaiDecision {
	lines := strings.Split(string(existing), "\n")

	start, end, span := gaiMarkerSpan(lines)
	switch span {
	case gaiSpanMalformed:
		return gaiDecision{kind: gaiMalformed, reason: gaiMalformedReason(lines)}
	case gaiSpanFound:
		if found, example := gaiForeignOutside(lines, start, end); found {
			return gaiDecision{kind: gaiForeign, found: example}
		}
		return gaiDecision{
			kind:     gaiReplace,
			result:   gaiReplaceSpan(lines, start, end),
			replaced: gaiSpanPrecedence(lines[start : end+1]),
		}
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

// gaiSpanKind is whether the file holds no marker pair, exactly one, or a shape
// the step does not own.
type gaiSpanKind int

const (
	gaiSpanNone gaiSpanKind = iota
	gaiSpanFound
	gaiSpanMalformed
)

// gaiMarkerSpan finds the one start marker and the one end marker that make up a
// block the step owns. Anything else -- a start without an end, an end without a
// start, more than one of either, or the end before the start -- is malformed:
// replacing part of it would silently drop the lines the step cannot see, which
// is the defect an unterminated block used to carry (F1, F3).
func gaiMarkerSpan(lines []string) (start, end int, kind gaiSpanKind) {
	start, end = -1, -1
	starts, ends := 0, 0
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case gaiMarkerStart:
			starts++
			if start < 0 {
				start = i
			}
		case gaiMarkerEnd:
			ends++
			if end < 0 {
				end = i
			}
		}
	}
	switch {
	case starts == 0 && ends == 0:
		return 0, 0, gaiSpanNone
	case starts == 1 && ends == 1 && start <= end:
		return start, end, gaiSpanFound
	default:
		return 0, 0, gaiSpanMalformed
	}
}

// gaiMalformedReason says, in the log's own words, which marker shape the step
// refused. It reads the same lines gaiMarkerSpan did, so the message cannot
// describe a shape the decision did not see.
func gaiMalformedReason(lines []string) string {
	starts, ends := 0, 0
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case gaiMarkerStart:
			starts++
		case gaiMarkerEnd:
			ends++
		}
	}
	switch {
	case starts > 1 || ends > 1:
		return "the file holds more than one marked block"
	case starts == 1 && ends == 0:
		return "a marked block is missing its end marker"
	case starts == 0 && ends == 1:
		return "an end marker has no start marker"
	default:
		return "the markers are out of order"
	}
}

// gaiForeignOutside reports the first uncommented precedence line outside the
// managed span that is not the deciding line. The deciding line outside the span
// is harmless -- it is the same preference -- so it is not foreign; any other
// precedence is the owner's and governs the file (F2).
func gaiForeignOutside(lines []string, start, end int) (found bool, example string) {
	for i, line := range lines {
		if i >= start && i <= end {
			continue
		}
		if trimmed := gaiPrecedenceLineOrEmpty(line); trimmed != "" && !gaiIsDecidingPrecedence(trimmed) {
			return true, trimmed
		}
	}
	return false, ""
}

// gaiSpanPrecedence returns the precedence line inside the managed span that the
// replacement would restore, or an empty string when the block already holds the
// deciding line. It is what lets the step name the value it replaced instead of
// restoring it silently (F4).
func gaiSpanPrecedence(span []string) string {
	for _, line := range span {
		if trimmed := gaiPrecedenceLineOrEmpty(line); trimmed != "" && !gaiIsDecidingPrecedence(trimmed) {
			return trimmed
		}
	}
	return ""
}

// gaiPrecedenceLineOrEmpty returns the trimmed text of an uncommented line whose
// first field is "precedence", or an empty string for anything else.
func gaiPrecedenceLineOrEmpty(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 || fields[0] != "precedence" {
		return ""
	}
	return trimmed
}

// gaiIsDecidingPrecedence reports whether a precedence line is the IPv4-first
// line, which is the one the block installs and the one that is not foreign.
func gaiIsDecidingPrecedence(line string) bool {
	fields := strings.Fields(line)
	return len(fields) >= 3 && fields[0] == "precedence" &&
		fields[1] == gaiIPv4MappedPrefix && fields[2] == gaiIPv4MappedValue
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
		trimmed := gaiPrecedenceLineOrEmpty(line)
		if trimmed == "" {
			continue
		}
		found = true
		if gaiIsDecidingPrecedence(trimmed) {
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
// so a file that already prefers IPv4, one whose precedence the owner set
// outside the block, or one whose markers are not a shape this step owns, is
// never rewritten (S7, S8).
//
// A file the run cannot read or write is reported and left alone rather than
// failing the step: the preference is an improvement, and a read-only /etc must
// not abort an interactive run or make a non-interactive one exit non-zero (F5).
func installGAIConf(dst, stepID string) {
	existing, err := os.ReadFile(dst)
	if err != nil && !os.IsNotExist(err) {
		gaiLog(stepID, fmt.Sprintf("Could not read %s, so the installer leaves the IPv4 preference unchanged: %v", dst, err))
		return
	}

	decision := gaiDecide(existing)
	switch decision.kind {
	case gaiAlreadyIPv4:
		gaiLog(stepID, fmt.Sprintf("%s already prefers IPv4: %s is uncommented, so the installer leaves the file unchanged.", dst, gaiPrecedenceLine))
		return
	case gaiForeign:
		gaiLog(stepID, fmt.Sprintf("%s already sets a precedence of its own (%q) outside the managed block. The installer leaves it alone: overriding a precedence the owner chose is his decision, not this run's.", dst, decision.found))
		return
	case gaiMalformed:
		gaiLog(stepID, fmt.Sprintf("%s holds a marked block the installer will not rewrite: %s. Leaving the file unchanged rather than repairing it blindly.", dst, decision.reason))
		return
	}

	// A file that already holds the canonical block is left untouched rather than
	// rewritten: a second run must leave it byte for byte the same, and rewriting
	// it would only add another backup (S8).
	if bytes.Equal(existing, decision.result) {
		gaiLog(stepID, fmt.Sprintf("%s already holds the dotfiles IPv4-first block; leaving it unchanged.", dst))
		return
	}

	if err := gaiApplyArtifactContent(decision.result, dst, stepID); err != nil {
		gaiLog(stepID, fmt.Sprintf("Could not write %s, so the IPv4 preference was not applied: %v", dst, err))
		return
	}
	// The block is the installer's, so a stale value inside it is restored; naming
	// the value it replaced keeps the restore from being a silent edit (F4).
	if decision.replaced != "" {
		gaiLog(stepID, fmt.Sprintf("Restored the dotfiles precedence in %s: replaced %q with %q.", dst, decision.replaced, gaiPrecedenceLine))
	}
	if decision.kind == gaiReplace {
		gaiLog(stepID, fmt.Sprintf("Updated the dotfiles IPv4-first block in %s", dst))
	} else {
		gaiLog(stepID, fmt.Sprintf("Added the dotfiles IPv4-first block to %s", dst))
	}
}

// stepInstallGAIConf is the gai step: it makes the machine prefer IPv4 by
// raising the precedence of IPv4-mapped addresses, and reports what a future
// libc-bin upgrade may do to the file it edited. Neither the write nor the
// report fails the step (S3, F5), and the file itself is only written through
// the shared writer (S11).
func stepInstallGAIConf(_ *Model) error {
	dst := os.Getenv(envGAIConfPath)
	if dst == "" {
		dst = defaultGAIConfPath
	}

	installGAIConf(dst, gaiStepID)
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
