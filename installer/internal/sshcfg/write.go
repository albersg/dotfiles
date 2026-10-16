package sshcfg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// WRITING THE CONFIGURATION
// ============================================================================
//
// Two rules shape everything in this file.
//
// The first is that the user's file is not this tool's to reformat. A write is a
// surgical replacement of the lines the change is about: every comment, blank
// line, Include, Match, wildcard, unknown option and trailing newline outside
// those lines keeps its exact bytes. That is why the plan carries the whole
// file's text, before and after: the caller can show precisely what would
// change, and Apply can write exactly what was shown.
//
// The second is that nothing reaches the destination unproven. Every write takes
// a backup first, validates the new content with the client's own parser, and
// only then replaces the file -- atomically, so a failure at any step leaves the
// user with the file they had.

// The two markers of the block this tool owns. Everything between them is
// managed by this tool; everything outside them is never touched. They are
// comments, so a client that reads the file sees exactly the same
// configuration whether they are there or not.
const (
	ManagedBlockStart = "# >>> dotfiles ssh-manager >>>"
	ManagedBlockEnd   = "# <<< dotfiles ssh-manager <<<"
)

// managedBlockHeader is what a newly created block says about itself. It is
// written once, when the block is created, and never rewritten afterwards: a
// user who edits it is entitled to keep their edit.
const managedBlockHeader = "# The entries between these two markers are managed by the dotfiles installer.\n" +
	"# Everything outside them is left exactly as it is."

// HostSpec is one entry as the interface describes it: the fields the client
// understands as a host's identity, plus any directives to write verbatim.
type HostSpec struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
	// Options are written verbatim as `Key Value` lines, in order, after the
	// fields above. They are how a value the form does not model survives an
	// edit of an entry that has one.
	Options []Option
}

// WritePlan is a change that has not happened yet.
//
// Before and After are the whole file as it is and as it would be, so the
// caller can show the change and Apply can prove it is still applicable. The
// unexported fields are that proof: the bytes the plan was built from, and the
// range of lines it replaces.
type WritePlan struct {
	Path    string
	Before  string
	After   string
	Backup  string
	Summary string
	Preview []string
	// Kind is "managed" for an entry this tool owns and "stanza" for one of the
	// user's own simple entries edited in place.
	Kind  string
	Alias string

	start   int // first replaced line, 0-based, inclusive
	end     int // one past the last replaced line
	added   int
	removed int
	existed bool
	mode    os.FileMode
	digest  string
}

// WriteResult is what a completed write did.
type WriteResult struct {
	Path string
	// Backup is the file's previous content, or empty when there was nothing to
	// back up.
	Backup string
	// Added and Removed count the lines the change wrote and dropped.
	Added   int
	Removed int
	// Validated reports whether the client parsed the new content before it
	// replaced the old one. Validation carries the reason when it did not run.
	Validated  bool
	Validation string
	// FileMode is the mode the file was left with.
	FileMode os.FileMode
}

// ErrPlanStale is returned when the destination changed between the preview and
// the write. Nothing is written when it is returned: the file the user read is
// not the file the plan was built from, so applying it would discard an edit
// nobody saw.
var ErrPlanStale = errors.New("the file changed since the preview, so nothing was written")

// PlanCreate plans a new entry inside the block this tool owns.
func (m *Manager) PlanCreate(ctx context.Context, spec HostSpec) (WritePlan, error) {
	if err := validateSpec(spec); err != nil {
		return WritePlan{}, err
	}
	if existing, ok := m.findHost(spec.Alias); ok && existing.Managed {
		return WritePlan{}, fmt.Errorf("an entry named %s is already managed by this tool in %s, so nothing was written", spec.Alias, existing.File)
	}

	lines, existed, mode := m.readUserConfigLines()
	stanza := formatStanza(spec)
	after, at := addManagedLines(lines, stanza)
	kind := "managed"

	return m.buildPlan(WritePlan{
		Path:    m.Paths.UserConfig,
		Kind:    kind,
		Alias:   spec.Alias,
		Summary: fmt.Sprintf("Add the entry %s inside the block this tool manages in %s.", spec.Alias, m.Paths.UserConfig),
		start:   at,
		end:     at,
		added:   len(stanza),
		existed: existed,
		mode:    mode,
	}, lines, after, stanza), nil
}

// PlanUpdate plans an edit of one entry. Only the lines whose value changes are
// rewritten; the entry's other directives, their order and its comments stay
// where they are.
func (m *Manager) PlanUpdate(ctx context.Context, alias string, spec HostSpec) (WritePlan, error) {
	if err := validateSpec(spec); err != nil {
		return WritePlan{}, err
	}
	host, ok := m.findHost(alias)
	if !ok {
		return WritePlan{}, fmt.Errorf("no entry named %s was found in the configuration, so nothing was written", alias)
	}
	if host.ReadOnly != "" {
		return WritePlan{}, fmt.Errorf("%s, so this tool will not rewrite it", host.ReadOnly)
	}
	if spec.Alias != alias {
		if other, found := m.findHost(spec.Alias); found {
			return WritePlan{}, fmt.Errorf("another entry already uses the name %s in %s, so nothing was written", spec.Alias, other.File)
		}
	}

	lines, existed, mode := m.readConfigFileLines(host.File)
	start, end, ok := stanzaRange(lines, host.Line-1)
	if !ok {
		return WritePlan{}, fmt.Errorf("the entry %s was not found at %s:%d any more, so nothing was written", alias, host.File, host.Line)
	}
	updated, added, removed := rewriteStanza(lines[start:end], spec)

	after := replaceRange(lines, start, end, updated)
	kind := "stanza"
	if host.Managed {
		kind = "managed"
	}
	return m.buildPlan(WritePlan{
		Path:  host.File,
		Kind:  kind,
		Alias: spec.Alias,
		Summary: fmt.Sprintf("Update the entry %s in %s: %d %s rewritten, %d added, %d removed.",
			alias, host.File, len(updated)-added, sshLineWord(len(updated)-added), added, removed),
		start:   start,
		end:     end,
		added:   added,
		removed: removed,
		existed: existed,
		mode:    mode,
	}, lines, after, updated), nil
}

// PlanDelete plans the removal of one entry.
func (m *Manager) PlanDelete(ctx context.Context, alias string) (WritePlan, error) {
	host, ok := m.findHost(alias)
	if !ok {
		return WritePlan{}, fmt.Errorf("no entry named %s was found in the configuration, so nothing was written", alias)
	}
	if host.ReadOnly != "" {
		return WritePlan{}, fmt.Errorf("%s, so this tool will not rewrite it", host.ReadOnly)
	}

	lines, existed, mode := m.readConfigFileLines(host.File)
	start, end, ok := stanzaRange(lines, host.Line-1)
	if !ok {
		return WritePlan{}, fmt.Errorf("the entry %s was not found at %s:%d any more, so nothing was written", alias, host.File, host.Line)
	}
	// The stanza's own trailing blank line goes with it; a blank line that
	// separates it from the next entry is furniture, not content, and leaving it
	// behind would grow the file by one line per removal.
	for end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}
	after := replaceRange(lines, start, end, nil)
	kind := "stanza"
	if host.Managed {
		kind = "managed"
	}
	return m.buildPlan(WritePlan{
		Path:    host.File,
		Kind:    kind,
		Alias:   alias,
		Summary: fmt.Sprintf("Remove the entry %s from %s. The lines around it are left as they are.", alias, host.File),
		start:   start,
		end:     end,
		removed: end - start,
		existed: existed,
		mode:    mode,
	}, lines, after, nil), nil
}

// PlanDuplicate plans a copy of one entry under a new name. The copy is written
// as an entry this tool manages, with the source entry's directives carried over
// verbatim, so an option this package does not model survives the copy.
func (m *Manager) PlanDuplicate(ctx context.Context, alias, newAlias string) (WritePlan, error) {
	newAlias = strings.TrimSpace(newAlias)
	if newAlias == "" {
		return WritePlan{}, errors.New("a duplicated entry needs a name of its own, so nothing was written")
	}
	host, ok := m.findHost(alias)
	if !ok {
		return WritePlan{}, fmt.Errorf("no entry named %s was found in the configuration, so nothing was written", alias)
	}
	if existing, found := m.findHost(newAlias); found {
		return WritePlan{}, fmt.Errorf("an entry named %s already exists in %s, so nothing was written", newAlias, existing.File)
	}

	source, _, _ := m.readConfigFileLines(host.File)
	start, end, ok := stanzaRange(source, host.Line-1)
	if !ok {
		return WritePlan{}, fmt.Errorf("the entry %s was not found at %s:%d any more, so nothing was written", alias, host.File, host.Line)
	}
	copyLines := append([]string(nil), source[start:end]...)
	copyLines[0] = "Host " + newAlias

	lines, existed, mode := m.readUserConfigLines()
	after, at := addManagedLines(lines, copyLines)

	return m.buildPlan(WritePlan{
		Path:    m.Paths.UserConfig,
		Kind:    "managed",
		Alias:   newAlias,
		Summary: fmt.Sprintf("Copy the entry %s to %s, inside the block this tool manages in %s.", alias, newAlias, m.Paths.UserConfig),
		start:   at,
		end:     at,
		added:   len(copyLines),
		existed: existed,
		mode:    mode,
	}, lines, after, copyLines), nil
}

// buildPlan completes a plan: the old and the new text, the preview of what
// changed, the backup the write would take, and the digest that proves the
// destination has not moved since the preview. replacement is the run of lines
// the plan wrote in place of the range it replaced, which is what the preview
// draws with a + marker.
func (m *Manager) buildPlan(plan WritePlan, lines, after, replacement []string) WritePlan {
	// joinFileLines is the exact inverse of splitFileLines: a file that ended with
	// a newline keeps it, because the trailing empty element the split produced is
	// joined back the same way.
	plan.Before = joinFileLines(lines)
	plan.After = joinFileLines(after)
	plan.digest = digestOf(plan.Before)
	if plan.existed {
		plan.Backup = m.backupPathFor(plan.Path)
	}
	plan.Preview = previewRange(lines, plan.start, plan.end, replacement)
	return plan
}

// Apply writes a plan. It refuses when the destination changed since the
// preview, backs up what is there, validates the new text with the client, and
// replaces the file atomically.
func (m *Manager) Apply(ctx context.Context, plan WritePlan) (WriteResult, error) {
	if plan.Path == "" {
		return WriteResult{}, errors.New("the plan names no file, so nothing was written")
	}
	current, err := os.ReadFile(plan.Path)
	switch {
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return WriteResult{}, fmt.Errorf("the file %s could not be read, so nothing was written: %w", plan.Path, err)
	case err != nil && plan.existed:
		return WriteResult{}, fmt.Errorf("%s does not exist any more, so nothing was written", plan.Path)
	case err == nil && digestOf(string(current)) != plan.digest:
		return WriteResult{}, ErrPlanStale
	}

	result := WriteResult{Path: plan.Path, Added: plan.added, Removed: plan.removed}

	if _, err := os.Stat(plan.Path); err == nil {
		// The backup path is derived here rather than taken from the plan. The plan was
		// built when the change was previewed, and a backup that appeared since then --
		// another write, or the user's own copy -- must not be overwritten: a backup is
		// the last copy of something the user wrote. The path the plan shows and the
		// path written agree in every ordinary case, because both carry the same
		// timestamp to the second; when they do not, the result reports the one used.
		plan.Backup = m.backupPathFor(plan.Path)
	}
	if plan.existed && plan.Backup != "" {
		mode := plan.mode
		if mode == 0 {
			mode = 0o600
		}
		if err := writeFileAtomic(plan.Backup, string(current), mode|0o600); err != nil {
			return WriteResult{}, fmt.Errorf("the backup at %s could not be written, so nothing was written: %w", plan.Backup, err)
		}
		result.Backup = plan.Backup
	}

	mode := plan.mode.Perm()
	if mode == 0 {
		// A file this tool creates is private: a configuration file names hosts,
		// users and key paths, and there is no reason for another account to read it.
		mode = 0o600
	}
	if mode&0o022 != 0 {
		// A file that others can write is a file others can add a ProxyCommand to, so
		// an expanded mode is tightened while it is written. Read bits are left alone:
		// they are not a hole, and changing them would be an edit the user did not ask
		// for.
		mode = 0o600
	}

	temp, err := writeTempFile(filepath.Dir(plan.Path), plan.After, mode)
	if err != nil {
		return WriteResult{}, fmt.Errorf("the new configuration could not be written, so %s is unchanged: %w", plan.Path, err)
	}
	defer os.Remove(temp)

	ran, problem := m.validateFile(ctx, temp, plan.Alias)
	result.Validated = ran && problem == ""
	result.Validation = problem
	if ran && problem != "" {
		// The client parsed the new content and refused it. Writing it anyway would
		// replace a working file with one the client will not read, which is the one
		// outcome worse than refusing the change.
		return WriteResult{}, fmt.Errorf("the client refused the new configuration, so %s is unchanged: %s", plan.Path, problem)
	}

	if err := os.Rename(temp, plan.Path); err != nil {
		return WriteResult{}, fmt.Errorf("the new configuration could not replace %s, so the file is unchanged: %w", plan.Path, err)
	}
	if err := os.Chmod(plan.Path, mode); err != nil {
		return WriteResult{}, fmt.Errorf("the file was written but its mode could not be set: %w", err)
	}
	result.FileMode = mode
	return result, nil
}

// validateFile parses a written file with the client's own parser. It is the
// only validation this package trusts: reimplementing the client's grammar would
// be a second parser to disagree with the first, and the client's answer is the
// one that decides whether the file works.
//
// ran says whether the client was run at all, and problem is what it said when
// it refused. The two are separate on purpose: a machine with no client cannot
// answer, and that is not the same as a client that answered no.
func (m *Manager) validateFile(ctx context.Context, path, alias string) (ran bool, problem string) {
	client := m.sshBinary()
	if client == "" {
		return false, "the ssh client was not found, so the new configuration was not parsed before it replaced the old one"
	}
	host := alias
	if host == "" || strings.ContainsAny(host, "*?!") {
		// A wildcard cannot be asked about; the file still has to parse, which is
		// what any name proves.
		host = "localhost"
	}
	res := m.Run.Run(ctx, Command{Name: client, Args: []string{"-G", "-F", path, "--", host}})
	if res.Err != nil {
		return false, "the client could not be run to parse the new configuration: " + messageSummary(res)
	}
	if res.Code != 0 {
		return true, messageSummary(res)
	}
	return true, ""
}

// ============================================================================
// Reading and rebuilding the file
// ============================================================================

// readUserConfigLines reads the user's configuration file.
func (m *Manager) readUserConfigLines() (lines []string, existed bool, mode os.FileMode) {
	return m.readConfigFileLines(m.Paths.UserConfig)
}

// readConfigFileLines reads one configuration file into lines. splitFileLines
// keeps a file's final newline as a trailing empty element, so rebuilding the
// text with joinFileLines returns the file byte for byte.
func (m *Manager) readConfigFileLines(path string) (lines []string, existed bool, mode os.FileMode) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0
	}
	info, statErr := os.Stat(path)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return splitFileLines(string(data)), true, mode
}

// addManagedLines puts a run of lines into the block this tool owns, creating the
// block when the file has none, and returns the new file with the position the
// preview shows the insertion at.
//
// The block is always written with a blank line between it and the user's own
// content, and the file always ends with a newline afterwards: a file that grows
// a block should not also grow a missing terminator. A file that already holds
// the block only gains the lines, in front of the end marker -- which is where
// the entries of a block this tool owns live.
func addManagedLines(lines []string, block []string) (after []string, at int) {
	if _, end, ok := managedBlockRange(lines); ok {
		return insertLines(lines, end, block), end
	}
	body := trimTrailingEmpty(lines)
	if len(body) == 0 {
		out := append([]string{managedBlockHeader, ManagedBlockStart}, block...)
		out = append(out, ManagedBlockEnd, "")
		return out, 0
	}
	out := append([]string(nil), body...)
	out = append(out, "", ManagedBlockStart)
	out = append(out, block...)
	out = append(out, ManagedBlockEnd, "")
	return out, len(body)
}

// trimTrailingEmpty drops the empty element splitFileLines leaves behind for a
// file's final newline, so the lines can be treated as the file's real lines.
func trimTrailingEmpty(lines []string) []string {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		return lines[:n-1]
	}
	return lines
}

// managedBlockRange is the line range between the two markers, exclusive of the
// markers themselves: [start, end) is what this tool owns.
func managedBlockRange(lines []string) (start, end int, ok bool) {
	start, end = -1, -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case ManagedBlockStart:
			if start < 0 {
				start = i + 1
			}
		case ManagedBlockEnd:
			if start >= 0 {
				return start, i, true
			}
		}
	}
	return 0, 0, false
}

// stanzaRange is the line range of the Host block that starts at start:
// [start, end) covers the block's Host line and the directives that belong to
// it. A blank line or a comment at the end of the block belongs to the file's
// layout rather than to the entry, so it is left outside the range: that is what
// keeps a removal from taking the comment a user wrote between two entries with
// it, and what keeps an edit from rewriting lines it has no business in.
func stanzaRange(lines []string, start int) (int, int, bool) {
	if start < 0 || start >= len(lines) {
		return 0, 0, false
	}
	if !isHostLine(lines[start]) {
		return 0, 0, false
	}
	end := start + 1
	for end < len(lines) {
		if isHostLine(lines[end]) || isMatchLine(lines[end]) || isIncludeLine(lines[end]) {
			break
		}
		end++
	}
	last := start + 1
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			last = i + 1
		}
	}
	return start, last, true
}

// isHostLine reports whether a line opens a Host block.
func isHostLine(line string) bool {
	key, _ := splitDirective(strings.TrimSpace(line))
	return strings.EqualFold(key, "host")
}

// isMatchLine reports whether a line opens a Match block.
func isMatchLine(line string) bool {
	key, _ := splitDirective(strings.TrimSpace(line))
	return strings.EqualFold(key, "match")
}

// isIncludeLine reports whether a line pulls in another file.
func isIncludeLine(line string) bool {
	key, _ := splitDirective(strings.TrimSpace(line))
	return strings.EqualFold(key, "include")
}

// findHost is the entry named by an alias, as the parser reads the whole
// configuration: the client's own file order, Includes inlined.
func (m *Manager) findHost(alias string) (Host, bool) {
	if alias == "" {
		return Host{}, false
	}
	model := m.parseModel()
	for _, host := range model.hosts {
		if host.Alias == alias {
			return host, true
		}
	}
	return Host{}, false
}

// insertLines inserts block at position at.
func insertLines(lines []string, at int, block []string) []string {
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:at]...)
	out = append(out, block...)
	out = append(out, lines[at:]...)
	return out
}

// replaceRange replaces [start, end) with block.
func replaceRange(lines []string, start, end int, block []string) []string {
	out := make([]string, 0, len(lines)-(end-start)+len(block))
	out = append(out, lines[:start]...)
	out = append(out, block...)
	out = append(out, lines[end:]...)
	return out
}

// formatStanza renders one entry as the lines a managed block holds.
func formatStanza(spec HostSpec) []string {
	lines := []string{"Host " + strings.TrimSpace(spec.Alias)}
	add := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		lines = append(lines, managedIndent+key+" "+strings.TrimSpace(value))
	}
	add("HostName", spec.HostName)
	add("User", spec.User)
	add("Port", spec.Port)
	add("IdentityFile", spec.IdentityFile)
	add("ProxyJump", spec.ProxyJump)
	for _, option := range spec.Options {
		if strings.TrimSpace(option.Key) == "" {
			continue
		}
		lines = append(lines, managedIndent+strings.TrimSpace(option.Key)+" "+strings.TrimSpace(option.Value))
	}
	return lines
}

// managedIndent is what a written entry's directives are indented with. It
// matches the indentation the dotfiles' own configuration uses, so a new entry
// does not look like it came from another tool.
const managedIndent = "    "

// rewriteStanza applies a spec to one entry's lines. It returns the new lines
// and how many were added and removed. A directive whose value is unchanged is
// left byte-for-byte as it was, including its own spacing and inline comment.
func rewriteStanza(stanza []string, spec HostSpec) (lines []string, added, removed int) {
	lines = append([]string(nil), stanza...)
	if spec.Alias != "" {
		lines[0] = "Host " + strings.TrimSpace(spec.Alias)
	}

	wanted := []struct {
		key   string
		value string
	}{
		{"HostName", spec.HostName},
		{"User", spec.User},
		{"Port", spec.Port},
		{"IdentityFile", spec.IdentityFile},
		{"ProxyJump", spec.ProxyJump},
	}
	for _, want := range wanted {
		value := strings.TrimSpace(want.value)
		index := directiveIndex(lines, want.key)
		switch {
		case value == "" && index >= 0:
			// A cleared field removes the directive, because an empty value is
			// not a value the client understands.
			lines = append(append([]string(nil), lines[:index]...), lines[index+1:]...)
			removed++
		case value == "":
			continue
		case index >= 0:
			current := optionResolvedValue(splitValue(lines[index]))
			if current == value {
				continue
			}
			lines[index] = rewriteDirective(lines[index], want.key, value)
		default:
			lines = append(lines, managedIndent+want.key+" "+value)
			added++
		}
	}

	for _, option := range spec.Options {
		key := strings.TrimSpace(option.Key)
		if key == "" || directiveIndex(lines, key) >= 0 {
			continue
		}
		lines = append(lines, managedIndent+key+" "+strings.TrimSpace(option.Value))
		added++
	}
	return lines, added, removed
}

// splitValue is the value half of a directive line.
func splitValue(line string) string {
	_, value := splitDirective(strings.TrimSpace(line))
	return value
}

// directiveIndex is the index of the first line in an entry that sets a key.
func directiveIndex(lines []string, key string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		got, _ := splitDirective(trimmed)
		if strings.EqualFold(got, key) {
			return i
		}
	}
	return -1
}

// rewriteDirective replaces a directive's value while keeping the line's own
// indentation and key spelling. The line carries no comment: an entry whose
// value holds an unquoted `#` is refused as read-only before a plan is built, so
// this never has to decide what to do with a comment.
func rewriteDirective(line, key, value string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	got, _ := splitDirective(strings.TrimSpace(line))
	return indent + got + " " + value
}

// ============================================================================
// The preview, the backup and the atomic write
// ============================================================================

// previewContext is how many unchanged lines are shown on each side of a change.
const previewContext = 2

// previewRange is what the confirmation shows: the lines that go with a - and
// the lines that arrive with a +, with a little context and a marker when the
// change sits deeper in a long file.
func previewRange(lines []string, start, end int, replacement []string) []string {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		// The trailing empty element is the file's final newline, not a line.
		lines = lines[:n-1]
		if end > len(lines) {
			end = len(lines)
		}
	}
	var preview []string
	if start > previewContext {
		preview = append(preview, fmt.Sprintf("… %d more lines above", start-previewContext))
	}
	from := start - previewContext
	if from < 0 {
		from = 0
	}
	for _, line := range lines[from:start] {
		preview = append(preview, "  "+line)
	}
	for _, line := range lines[start:end] {
		preview = append(preview, "- "+line)
	}
	for _, line := range replacement {
		preview = append(preview, "+ "+line)
	}
	to := end + previewContext
	if to > len(lines) {
		to = len(lines)
	}
	for _, line := range lines[end:to] {
		preview = append(preview, "  "+line)
	}
	if len(lines)-to > 0 {
		preview = append(preview, fmt.Sprintf("… %d more lines below", len(lines)-to))
	}
	return preview
}

// backupPathFor is the path a write's backup takes: the file's own name with the
// suffix the rest of this installer uses, and a counter when that name is
// already taken -- a backup is never overwritten, because the file it holds may
// be the last copy of something the user wrote.
func (m *Manager) backupPathFor(path string) string {
	stamp := time.Now().Format("20060102-150405")
	base := fmt.Sprintf("%s.bak-dotfiles-%s", path, stamp)
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
		if i > 100 {
			return candidate
		}
	}
}

// writeTempFile writes content to a fresh file beside the destination, so the
// rename that follows is on one filesystem and therefore atomic.
func writeTempFile(dir, content string, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, ".dotfiles-ssh-*.tmp")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(name)
		return "", err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		os.Remove(name)
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(name)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// writeFileAtomic writes content to path, replacing whatever is there.
func writeFileAtomic(path, content string, mode os.FileMode) error {
	temp, err := writeTempFile(filepath.Dir(path), content, mode)
	if err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}
	return os.Chmod(path, mode)
}

// validateSpec refuses a spec that could not become a valid entry: an entry
// needs a name, and the client's Host line cannot hold a wildcard this tool
// would then own.
func validateSpec(spec HostSpec) error {
	alias := strings.TrimSpace(spec.Alias)
	switch {
	case alias == "":
		return errors.New("an entry needs a name, so nothing was written")
	case strings.ContainsAny(alias, "*?!"):
		return errors.New("an entry this tool manages needs a single literal name, not a wildcard or a negation, so nothing was written")
	case strings.ContainsAny(alias, " \t"):
		return errors.New("an entry's name cannot contain a space, so nothing was written")
	}
	return nil
}

// digestOf is what the plan compares the destination against: the whole file's
// bytes, so any edit between the preview and the write is noticed, whether it
// touched the entry or not.
func digestOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// sshLineWord is the word for a count of lines, in the singular when there is
// one, because "1 lines" reads as a count nobody looked at.
func sshLineWord(n int) string {
	if n == 1 {
		return "line"
	}
	return "lines"
}
