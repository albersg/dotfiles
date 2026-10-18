package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ============================================================================
// PATH AND THE EXECUTABLES IT RESOLVES
// ============================================================================
//
// PATH is where a broken development environment hides: an entry that does not
// exist costs a lookup, two entries for the same tool decide which version runs,
// and a login shell that builds a different PATH from the one this process sees
// means the tool the installer found is not the tool the user gets.
//
// The checks here read the environment and the filesystem. The one command they
// run is the login shell's own startup, and only to ask it what PATH it ends up
// with -- bounded, and reported as unknown when it does not answer.

// doctorPathChecks is the PATH category's catalogue.
func doctorPathChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "path.entries",
			Category: doctorCategoryPath,
			Title:    "Every PATH entry resolves to a directory that can be searched",
			Required: true,
			Run:      doctorCheckPathEntries,
		},
		{
			ID:       "path.shadowed",
			Category: doctorCategoryPath,
			Title:    "The tools this installer manages resolve to one copy",
			Run:      doctorCheckPathShadowed,
		},
		{
			ID:       "path.effective",
			Category: doctorCategoryPath,
			Title:    "The login shell resolves the same PATH this process sees",
			Run:      doctorCheckPathEffective,
		},
	}
}

// doctorPathEntryIssue is one entry and what is wrong with it.
type doctorPathEntryIssue struct {
	index int
	entry string
	// reason says what was read, not what it means: the impact is the caller's.
	reason string
	// status is the state the entry deserves.
	status doctorStatus
}

// doctorCheckPathEntries reads PATH itself: an unset PATH, an entry that cannot
// resolve, two entries that name the same directory, and a directory that exists
// but cannot be searched because its mode does not allow it.
//
// The duplicates are reported after the entries that cannot resolve at all, and
// each entry is reported once even when it suffers twice: the list is what a
// user has to edit, and a finding per symptom per entry would be a wall.
func doctorCheckPathEntries(scan *doctorScan) []doctorFinding {
	raw := scan.getenv("PATH")
	if strings.TrimSpace(raw) == "" {
		return []doctorFinding{{
			Subject:  "PATH",
			Status:   doctorError,
			Evidence: "PATH is unset or empty",
			Impact:   "No command can be found by name: the installer's own steps, the shells' startup and every tool they call would fail to resolve.",
			Advice:   "Set PATH in the login shell's configuration, for example `export PATH=\"$HOME/.local/bin:/usr/local/bin:/usr/bin:/bin\"`, and open a new terminal.",
		}}
	}

	entries := filepath.SplitList(raw)
	type seenEntry struct {
		first int
	}
	seen := map[string]seenEntry{}
	var issues []doctorPathEntryIssue
	var duplicates []doctorPathEntryIssue

	for index, entry := range entries {
		if entry == "" {
			issues = append(issues, doctorPathEntryIssue{
				index: index, entry: "",
				reason: "the entry is empty",
				status: doctorWarning,
			})
			continue
		}
		if strings.HasPrefix(entry, "~") {
			issues = append(issues, doctorPathEntryIssue{
				index: index, entry: entry,
				reason: "the entry starts with ~, which the shell does not expand inside PATH",
				status: doctorWarning,
			})
			continue
		}
		if strings.Contains(entry, "$") {
			issues = append(issues, doctorPathEntryIssue{
				index: index, entry: entry,
				reason: "the entry still holds a variable reference ($), so it names a directory literally called that",
				status: doctorWarning,
			})
			continue
		}

		if first, ok := seen[entry]; ok {
			duplicates = append(duplicates, doctorPathEntryIssue{
				index: index, entry: entry,
				reason: fmt.Sprintf("the directory is already named at position %d", first.first+1),
				status: doctorWarning,
			})
			continue
		}
		seen[entry] = seenEntry{first: index}

		info, err := scan.stat(entry)
		switch {
		case err != nil:
			status := doctorWarning
			reason := "the directory does not exist"
			if os.IsPermission(err) {
				status = doctorWarning
				reason = "the directory cannot be read: " + err.Error()
			} else if !os.IsNotExist(err) {
				reason = "the directory cannot be read: " + err.Error()
			}
			issues = append(issues, doctorPathEntryIssue{index: index, entry: entry, reason: reason, status: status})
		case !info.IsDir():
			issues = append(issues, doctorPathEntryIssue{
				index: index, entry: entry,
				reason: "the path is a file, not a directory",
				status: doctorWarning,
			})
		case info.Mode().Perm()&0o111 == 0:
			issues = append(issues, doctorPathEntryIssue{
				index: index, entry: entry,
				reason: "the directory has mode " + doctorPermString(info.Mode()) + ": it cannot be searched",
				status: doctorWarning,
			})
		}
	}

	findings := make([]doctorFinding, 0, len(issues)+len(duplicates)+1)
	for _, issue := range issues {
		position := fmt.Sprintf("PATH entry %d of %d", issue.index+1, len(entries))
		subject := position
		if issue.entry != "" {
			subject = position + ": " + doctorRelHome(scan, issue.entry)
		}
		findings = append(findings, doctorFinding{
			Subject:  subject,
			Status:   issue.status,
			Evidence: issue.reason,
			Impact:   "The shell walks this entry, finds nothing usable and moves on: a token of startup time for nothing, and a tool somebody expected here is not found.",
			Advice:   "Remove the entry from PATH where the login shell builds it, or create the directory it names.",
		})
	}
	for _, issue := range duplicates {
		findings = append(findings, doctorFinding{
			Subject:  fmt.Sprintf("PATH entry %d of %d: %s", issue.index+1, len(entries), doctorRelHome(scan, issue.entry)),
			Status:   issue.status,
			Evidence: issue.reason,
			Impact:   "Two entries for one directory: harmless to resolution, but it makes PATH harder to read and hides a later mistake behind a duplicate.",
			Advice:   "Keep the first occurrence and drop the duplicate from wherever PATH is built.",
		})
	}

	if len(findings) == 0 {
		return []doctorFinding{{
			Subject:  "PATH",
			Status:   doctorOK,
			Evidence: fmt.Sprintf("%s, every one a directory that can be searched", doctorCounted(len(entries), "entry", "entries")),
			Impact:   "Command resolution works: the first match in this PATH is the copy that runs.",
			Advice:   "Nothing to do.",
		}}
	}
	return findings
}

// doctorCheckPathShadowed answers which copy of each tool this installer manages
// actually runs, and what that copy hides.
//
// One copy is ok. Two copies with the same version are stated as ok, because
// nothing can go wrong from it. Two copies with different versions are a
// warning: which one runs depends on PATH order, which is exactly the class of
// fault a user cannot see. When no version can be read, the answer is unknown --
// the doctor does not call two copies colliding without having compared them.
func doctorCheckPathShadowed(scan *doctorScan) []doctorFinding {
	entries := scan.pathEntries()
	if len(entries) == 0 {
		return []doctorFinding{{
			Subject:  "Tool resolution",
			Status:   doctorNotApplicable,
			Evidence: "PATH holds no entries",
			Impact:   "No tool can resolve by name, so there is no precedence to compare.",
			Advice:   "Set PATH first; the doctor's PATH check names the same fault.",
		}}
	}

	var findings []doctorFinding
	checked := 0
	// The platform's own list, not the raw catalogue: a tool the catalogue names
	// once for macOS and once for every other host must not be reported twice.
	for _, tool := range doctorToolsForPlatform(scan) {
		copies := doctorToolCopies(scan, tool.Name, entries)
		checked++
		switch len(copies) {
		case 0:
			// Presence is the tools check's question, not this one's.
			continue
		case 1:
			findings = append(findings, doctorFinding{
				Subject:  tool.Name,
				Status:   doctorOK,
				Evidence: fmt.Sprintf("one copy on PATH: %s", doctorRelHome(scan, copies[0].path)),
				Impact:   "Nothing is hidden: this is the copy that runs.",
				Advice:   "Nothing to do.",
			})
		default:
			findings = append(findings, doctorShadowedFinding(scan, tool, copies))
		}
	}

	if checked == 0 {
		return []doctorFinding{{
			Subject:  "Tool resolution",
			Status:   doctorUnknown,
			Evidence: "the doctor's own catalogue of managed tools is empty",
			Impact:   "Nothing about tool precedence is known.",
			Advice:   "This is a defect in the doctor: the catalogue should never be empty.",
		}}
	}
	if len(findings) == 0 {
		return []doctorFinding{{
			Subject:  "Tool resolution",
			Status:   doctorNotApplicable,
			Evidence: "none of the tools this installer manages is installed",
			Impact:   "There is no precedence to compare.",
			Advice:   "Nothing to do; install a tool and this check reports it.",
		}}
	}
	return findings
}

// doctorShadowedFinding is one tool with more than one copy on PATH.
func doctorShadowedFinding(scan *doctorScan, tool doctorTool, copies []doctorToolCopy) doctorFinding {
	winner := copies[0]
	others := copies[1:]

	winnerVersion := scan.toolVersion(winner.path, tool.VersionArgs...)
	subject := fmt.Sprintf("%s resolves to %s", tool.Name, doctorRelHome(scan, winner.path))

	// Every other copy is asked for its version too: the point of the finding is
	// the difference, not the fact that a second file exists.
	type comparison struct {
		copy    doctorToolCopy
		version doctorVersion
	}
	comparisons := make([]comparison, 0, len(others))
	unreadable := 0
	for _, copy := range others {
		version := scan.toolVersion(copy.path, tool.VersionArgs...)
		if version.Line == "" {
			unreadable++
		}
		comparisons = append(comparisons, comparison{copy: copy, version: version})
	}

	switch {
	case unreadable > 0 && winnerVersion.Line == "":
		var reasons []string
		for _, c := range comparisons {
			if c.version.Line == "" {
				reasons = append(reasons, fmt.Sprintf("%s: %s", doctorRelHome(scan, c.copy.path), doctorVersionProblem(c.version)))
			}
		}
		return doctorFinding{
			Subject: subject,
			Status:  doctorUnknown,
			Evidence: fmt.Sprintf("%s is the copy that runs, and %s after it; no version could be read (%s)",
				doctorRelHome(scan, winner.path), doctorCounted(len(others), "other copy sits", "other copies sit"),
				strings.Join(reasons, "; ")),
			Impact: "The precedence is known -- the first match wins -- but whether the copies differ is not.",
			Advice: fmt.Sprintf("Ask each copy directly, for example `%s %s`, and decide which one you want first.",
				doctorRelHome(scan, winner.path), strings.Join(tool.VersionArgs, " ")),
		}
	case unreadable > 0:
		var reasons []string
		for _, c := range comparisons {
			if c.version.Line == "" {
				reasons = append(reasons, fmt.Sprintf("%s: %s", doctorRelHome(scan, c.copy.path), doctorVersionProblem(c.version)))
			}
		}
		return doctorFinding{
			Subject: subject,
			Status:  doctorWarning,
			Evidence: fmt.Sprintf("%s runs and reports %q; %s after it could not be asked (%s)",
				doctorRelHome(scan, winner.path), winnerVersion.Line,
				doctorCounted(unreadable, "copy", "copies"), strings.Join(reasons, "; ")),
			Impact: "Which copy runs is decided by PATH order, and one of the copies cannot state its version.",
			Advice: fmt.Sprintf("Check the copy that answered nothing, or remove it: `%s`", doctorRelHome(scan, winner.path)),
		}
	}

	// Every copy answered. The verdict is the comparison.
	differing := []string{}
	for _, c := range comparisons {
		if c.version.Line != winnerVersion.Line {
			differing = append(differing, fmt.Sprintf("%s reports %q", doctorRelHome(scan, c.copy.path), c.version.Line))
		}
	}
	evidence := fmt.Sprintf("%s reports %q; %s later in PATH", doctorRelHome(scan, winner.path),
		winnerVersion.Command+" → "+winnerVersion.Line, doctorCounted(len(others), "copy", "copies"))
	if len(differing) > 0 {
		evidence += ": " + strings.Join(differing, ", ")
	}

	if len(differing) == 0 {
		return doctorFinding{
			Subject:  subject,
			Status:   doctorOK,
			Evidence: evidence + " — every copy reports the same version",
			Impact:   "A second copy of the same version is shadowed, which changes nothing about the version that runs.",
			Advice:   "Nothing to do; remove the extra copy if you want a shorter PATH.",
		}
	}
	return doctorFinding{
		Subject:  subject,
		Status:   doctorWarning,
		Evidence: evidence,
		Impact:   "Which version runs depends only on PATH order, so a shell that builds PATH differently runs a different version of " + tool.Name + ".",
		Advice:   "Put the copy you want first, or remove the other one, in whatever builds PATH.",
	}
}

// doctorVersionProblem names why a version command produced nothing.
func doctorVersionProblem(version doctorVersion) string {
	switch {
	case version.TimedOut:
		return "it did not answer inside the doctor's bound"
	case version.Error != "":
		return version.Error
	default:
		return "it printed nothing"
	}
}

// doctorToolCopy is one file named by a PATH entry that matches a tool.
type doctorToolCopy struct {
	path  string
	entry string
}

// doctorToolCopies lists every PATH entry that holds an executable of this name,
// in PATH order, skipping the entries that cannot be read. An entry that cannot
// be listed is skipped rather than reported: the entry check already names a
// directory that cannot be searched.
func doctorToolCopies(scan *doctorScan, name string, entries []string) []doctorToolCopy {
	var copies []doctorToolCopy
	for _, entry := range entries {
		if entry == "" || strings.HasPrefix(entry, "~") || strings.Contains(entry, "$") {
			continue
		}
		candidate := filepath.Join(entry, name)
		info, err := scan.stat(candidate)
		if err != nil || info.IsDir() {
			// The Windows executable suffixes are not chased here: this branch of
			// the installer runs on the Unix side of WSL, where a Windows binary
			// reachable through the interop PATH is reported by the entry's own
			// directory rather than resolved as a copy of a Unix tool.
			continue
		}
		if info.Mode().Perm()&0o111 == 0 {
			continue
		}
		copies = append(copies, doctorToolCopy{path: candidate, entry: entry})
	}
	return copies
}

// doctorCheckPathEffective compares the PATH this process resolves against the
// PATH the login shell's own startup produces, which is the "declared versus
// effective" question: a tool can be installed and still not be found in a
// terminal, and a tool the terminal finds can be invisible to this process.
//
// The shell is started once, with the same bound the startup audit uses, and a
// shell that does not answer is unknown with the reason: no verdict is invented
// from a startup that failed.
func doctorCheckPathEffective(scan *doctorScan) []doctorFinding {
	shell := scan.getenv("SHELL")
	if shell == "" {
		return []doctorFinding{{
			Subject:  "The login shell's PATH",
			Status:   doctorNotApplicable,
			Evidence: "$SHELL is not set",
			Impact:   "The shell this process would compare with is not named, so the comparison cannot be made.",
			Advice:   "Set SHELL in the environment that starts the installer if you want this comparison.",
		}}
	}
	lookup := scan.lookup(shell)
	if !lookup.found() {
		return []doctorFinding{{
			Subject:  "The login shell's PATH",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("$SHELL names %s, which does not resolve on PATH (%s)", shell, lookup.Error),
			Impact:   "The shell that would answer cannot be started, so nothing about its PATH is known.",
			Advice:   "Fix $SHELL, or ignore this: the rest of the scan does not depend on it.",
		}}
	}

	name := filepath.Base(lookup.Path)
	args := doctorShellPathArgs(name)
	result := scan.run(doctorShellStartTimeout, lookup.Path, args...)
	if result.TimedOut {
		return []doctorFinding{{
			Subject: "The login shell's PATH",
			Status:  doctorUnknown,
			Evidence: fmt.Sprintf("%s %s did not finish inside %s", doctorRelHome(scan, lookup.Path),
				strings.Join(args, " "), doctorDuration(doctorShellStartTimeout)),
			Impact: "The login shell's startup hangs or waits for something, so its PATH cannot be compared; a terminal that starts this shell waits the same way.",
			Advice: "Run the same command by hand to see where it stops: `" + lookup.Path + " " + strings.Join(args, " ") + "`",
		}}
	}
	if result.Err != "" {
		return []doctorFinding{{
			Subject: "The login shell's PATH",
			Status:  doctorUnknown,
			Evidence: fmt.Sprintf("%s %s failed: %s", doctorRelHome(scan, lookup.Path),
				strings.Join(args, " "), result.Err),
			Impact: "The login shell's startup fails, so its PATH is not the one printed here, and a terminal that starts this shell reads the same failure.",
			Advice: "Start the shell by hand and read the error it prints; the shells check names the startup files it reads.",
		}}
	}

	shellPath := strings.TrimSpace(result.Stdout)
	if shellPath == "" {
		return []doctorFinding{{
			Subject: "The login shell's PATH",
			Status:  doctorUnknown,
			Evidence: fmt.Sprintf("%s %s exited zero and printed no PATH", doctorRelHome(scan, lookup.Path),
				strings.Join(args, " ")),
			Impact: "Nothing about the login shell's PATH is known.",
			Advice: "Print it by hand to see: `" + lookup.Path + " " + strings.Join(args, " ") + "`",
		}}
	}

	current := scan.pathEntries()
	shellEntries := filepath.SplitList(shellPath)
	theirs := doctorPathSets(shellEntries)
	ours := doctorPathSets(current)

	var onlyShell, onlyProcess []string
	for index, entry := range shellEntries {
		if _, ok := ours[entry]; !ok && !doctorContains(onlyShell, entry) {
			onlyShell = append(onlyShell, entry)
			_ = index
		}
	}
	for _, entry := range current {
		if _, ok := theirs[entry]; !ok && !doctorContains(onlyProcess, entry) {
			onlyProcess = append(onlyProcess, entry)
		}
	}

	if len(onlyShell) == 0 && len(onlyProcess) == 0 {
		return []doctorFinding{{
			Subject: "PATH, declared and effective",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("%s adds no entry this process does not have, and this process adds none it does not have (%s)",
				doctorRelHome(scan, lookup.Path), doctorCounted(len(current), "entry", "entries")),
			Impact: "The tools this installer finds are the tools a terminal started from the same login shell finds.",
			Advice: "Nothing to do.",
		}}
	}

	var findings []doctorFinding
	if len(onlyShell) > 0 {
		findings = append(findings, doctorFinding{
			Subject: fmt.Sprintf("%s adds %s this process does not have", doctorRelHome(scan, lookup.Path),
				doctorCounted(len(onlyShell), "PATH entry", "PATH entries")),
			Status: doctorWarning,
			Evidence: fmt.Sprintf("the login shell resolves %s; this process resolves %s",
				doctorPathList(onlyShell, 4), doctorPathList(current, 4)),
			Impact: "The installer runs with a narrower PATH than a terminal does, so a step that runs a tool found only through the shell's own configuration may not find it.",
			Advice: "Start the installer from the login shell (a terminal opened normally), or add the missing directory where this process gets its PATH.",
		})
	}
	if len(onlyProcess) > 0 {
		findings = append(findings, doctorFinding{
			Subject: fmt.Sprintf("This process has %s the login shell does not", doctorCounted(len(onlyProcess), "PATH entry", "PATH entries")),
			Status:  doctorWarning,
			Evidence: fmt.Sprintf("this process resolves %s; the login shell resolves %s",
				doctorPathList(onlyProcess, 4), doctorPathList(shellEntries, 4)),
			Impact: "A tool found here is not found in a new terminal, which is how an installation that worked becomes a command that cannot be found.",
			Advice: "Add the directory to the login shell's own configuration, not only to the environment that starts this process.",
		})
	}
	return findings
}

// doctorShellPathArgs is the one command that prints a shell's own PATH, in that
// shell's own words. Every supported shell is named; a shell the installer does
// not know is asked the POSIX way, which is right for zsh, bash, ksh and sh and
// wrong for fish and nushell, which are named.
func doctorShellPathArgs(name string) []string {
	switch name {
	case "fish":
		return []string{"-l", "-c", "string join : $PATH"}
	case "nu":
		return []string{"-l", "-c", "$env.PATH | str join ':'"}
	default:
		return []string{"-l", "-c", `printf %s "$PATH"`}
	}
}

// doctorPathSets turns entries into a set, keeping the first position of each.
func doctorPathSets(entries []string) map[string]int {
	set := make(map[string]int, len(entries))
	for index, entry := range entries {
		if _, ok := set[entry]; !ok {
			set[entry] = index
		}
	}
	return set
}

// doctorContains is a small linear membership test for the entry lists, which
// hold tens of entries at most.
func doctorContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// doctorSortedKeys is used by the checks that report a map's keys in a stable
// order, so two runs over the same machine produce the same report.
func doctorSortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
