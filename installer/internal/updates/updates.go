// Package updates reads and applies the updates the package managers and modern
// toolchain managers on this machine report.
//
// It is deliberately not part of the interface. The four questions the update
// center answers -- which channels exist here, what each channel says is
// out of date, what command would update a selection, and what actually
// happened -- are asked in this order below the interface, and every answer is a
// value a test can read without a terminal:
//
//   - detection and inventory: Env, Channel and Adapter.Probe;
//   - one adapter per manager: Adapter, with a pure parser per manager's output;
//   - resolution: Center.PlansFor, which turns a selection into the commands the
//     owning managers would run;
//   - execution: Center.Apply, one manager at a time, with a re-read afterwards
//     so "requested", "executed" and "confirmed" are three different facts.
//
// Nothing here writes on its own initiative and nothing here is specific to AI:
// the package updates what is installed, whatever it is.
package updates

import (
	"sort"
	"strings"
)

// ManagerID identifies the program that owns an installation. It is part of an
// item's identity, never a label: two managers that both carry a tool called
// "git" own two different installations, and this package must never merge them.
type ManagerID string

const (
	// ManagerBrew is Homebrew, the manager the installer prefers whenever it is
	// present. Its scope is the user's: a Homebrew install lives under the
	// user's prefix.
	ManagerBrew ManagerID = "brew"
	// ManagerApt is Debian's and Ubuntu's package manager.
	ManagerApt ManagerID = "apt"
	// ManagerPacman is Arch's package manager.
	ManagerPacman ManagerID = "pacman"
	// ManagerDnf is Fedora's package manager.
	ManagerDnf ManagerID = "dnf"
	// ManagerPkg is Termux's package manager. It is not apt: Termux builds its
	// own repository and its own prefixes, so its updates are read and applied
	// through Termux's own channel.
	ManagerPkg ManagerID = "pkg"
	// ManagerSnap is snapd's channel.
	ManagerSnap ManagerID = "snap"
	// ManagerFlatpak is Flatpak's channel.
	ManagerFlatpak ManagerID = "flatpak"
	// ManagerRustup is the Rust toolchain manager, the modern runtime manager
	// this installer provisions. Its updates are toolchains, not packages.
	ManagerRustup ManagerID = "rustup"
)

// Scope is who owns an installation. It is part of an item's identity and it is
// what decides whether an update needs elevation, so it is never inferred from a
// name: a system-wide and a per-user installation of the same program are two
// installations with two owners.
type Scope string

const (
	// ScopeUser is an installation owned by the user running the installer: a
	// Homebrew prefix, a Termux prefix, a user Flatpak installation, a rustup
	// toolchain, a user snap.
	ScopeUser Scope = "user"
	// ScopeSystem is an installation owned by the machine: a distribution
	// package, a system Flatpak installation, a system snap.
	ScopeSystem Scope = "system"
)

// Kind is what an entry is, in the owning manager's own vocabulary. It is part
// of an item's identity because one manager can need different commands for
// different kinds: Homebrew updates a formula and a cask with two commands, and
// Flatpak needs the installation named.
type Kind string

const (
	KindFormula   Kind = "formula"
	KindCask      Kind = "cask"
	KindPackage   Kind = "package"
	KindSnap      Kind = "snap"
	KindFlatpak   Kind = "flatpak"
	KindToolchain Kind = "toolchain"
)

// Item is one installation with an update available. It is the unit the user
// selects, and the unit the results are reported for.
//
// Current and Available are the manager's own version strings, kept as the
// manager printed them: this package never rewrites a version into its own
// notation, because a rewritten version is a second claim about the machine.
// Either may be empty when the manager does not report it -- an empty string is
// "the manager did not say", never "no version".
type Item struct {
	Manager ManagerID
	Name    string
	Scope   Scope
	Kind    Kind
	// Current is the installed version, Available the version the manager says
	// it would install.
	Current   string
	Available string
	// Pinned is true when the owning manager has been told to hold this
	// installation where it is: Homebrew's `brew pin`, apt's holds, pacman's
	// IgnorePkg, dnf's versionlock, snap's holds. A pinned item is shown and is
	// never applied; PinReason says which hold it is under.
	Pinned    bool
	PinReason string
	// Note carries what the manager says about this entry that is neither a
	// version nor a hold: a cask the vendor maintains itself, a toolchain a
	// project pins, an entry whose installed version the manager does not report.
	Note string
}

// Key is the identity this package compares, groups and deduplicates by. It
// carries the manager and the scope as well as the name, which is what keeps two
// managers' installs of the same program apart and what keeps one manager's
// system and user installs apart.
func (i Item) Key() string {
	return string(i.Manager) + "\x1f" + string(i.Scope) + "\x1f" + i.Name
}

// Origin names where an entry belongs, for a screen that must say it in a row.
func (i Item) Origin() string {
	if i.Scope == ScopeSystem {
		return string(i.Manager) + " (system)"
	}
	return string(i.Manager) + " (user)"
}

// Status is what one channel's read, or one plan's application, produced. The
// states are named separately because they are different facts about the
// machine, and a screen that folded them into one "could not check" would make
// the user guess at which of them to act on.
type Status string

const (
	// StatusUpdates: the manager answered and has entries with updates.
	StatusUpdates Status = "updates"
	// StatusCurrent: the manager answered and has nothing to update. This is an
	// answer, not an absence.
	StatusCurrent Status = "current"
	// StatusUnavailable: the manager is not installed here. It is not an error
	// and never a failure of the whole check.
	StatusUnavailable Status = "unavailable"
	// StatusUnsupported: the manager exists but does not serve this host or this
	// channel, which is a fact about the platform rather than an error.
	StatusUnsupported Status = "unsupported"
	// StatusPermission: the command needed elevation and the machine refused it
	// without asking, or the manager reported that it must run as an
	// administrator.
	StatusPermission Status = "permission"
	// StatusOffline: the manager could not reach its source.
	StatusOffline Status = "offline"
	// StatusLocked: another process holds the manager's own lock.
	StatusLocked Status = "locked"
	// StatusConflict: the manager refused on dependency grounds.
	StatusConflict Status = "conflict"
	// StatusCancelled: the run was interrupted at the user's request.
	StatusCancelled Status = "cancelled"
	// StatusFailed: anything else the manager or this package refused, with
	// Reason carrying the manager's own words.
	StatusFailed Status = "failed"
)

// Settled reports whether a status is an answer that no retry inside this
// session would improve. A failed read may be retried; an unavailable manager
// will not appear by asking again.
func (s Status) Settled() bool {
	switch s {
	case StatusUpdates, StatusCurrent, StatusUnavailable, StatusUnsupported:
		return true
	}
	return false
}

// Result is one channel's read: what this manager says, or why it could not be
// asked. The manager's own output is not carried here; the exact read command
// is, so the answer can be reproduced by hand.
type Result struct {
	Manager ManagerID
	Label   string
	Scope   Scope
	Status  Status
	// Reason is why the status is not StatusUpdates or StatusCurrent, in words a
	// reader can act on: the missing program, the refused permission, the lock
	// holder, the unreachable source.
	Reason string
	// Items are the entries the manager reports an update for. A pinned item is
	// present here with Pinned set: it is part of the answer, and what the
	// package refuses is applying it.
	Items []Item
	// Command is the exact read command, so the screen can name its source.
	Command string
	// Notes are the caveats about this channel's answer that are neither a
	// status nor an entry: metadata that was not refreshed, an installed version
	// the manager does not report, an entry left out.
	Notes []string
}

// Updates reports how many entries the channel offered.
func (r Result) Updates() int { return len(r.Items) }

// Plan is one command that a manager would run, built from a selection and
// nothing else. It is what the interface shows before anything happens and what
// Apply executes, so the screen and the run cannot disagree about what would
// happen.
type Plan struct {
	Manager ManagerID
	Scope   Scope
	Kind    Kind
	// Command is the program and its arguments, exactly as they will be run.
	// There is no shell anywhere on this path: a package name is an argument, so
	// no name can become a second command.
	Command Command
	// Items are the entries this command would move.
	Items []Item
	// Restart is what the manager says, or what is known, about what keeps the
	// old version until something else happens: a service that is not restarted,
	// a running kernel, an application that must be closed and reopened. It is
	// empty when nothing is known to be pending, never a guess.
	Restart string
	// Rollback is the honest word on going back. It is never "yes" for a package
	// manager unless the manager itself keeps the previous version: what it says
	// instead is what going back would actually cost.
	Rollback string
	// Notes are the consequences the manager carries beyond the named entries: a
	// dependency resolution that may add or move packages, metadata that will be
	// refreshed first, a hold that was left out.
	Notes []string
}

// sudoDisplayPrefix is how an elevated command reads to a person. It is the
// program, the non-interactive flag and the end of sudo's own options: the exact
// words the run will use, and the same words a user can type themselves after
// priming their credential.
const sudoDisplayPrefix = "sudo -n -- "

// Display is the command as a reader would type it, elevation included. Arguments
// that need quoting are quoted for the eye only; nothing here is ever handed to a
// shell.
func (p Plan) Display() string {
	if p.Command.Elevate {
		return sudoDisplayPrefix + QuoteArgv(p.Command.Argv)
	}
	return QuoteArgv(p.Command.Argv)
}

// Elevation reports whether this plan needs an administrator. It is what the
// confirmation screen names, and what the runner turns into sudo.
func (p Plan) Elevation() bool { return p.Command.Elevate }

// OutcomeStep is how far one entry of a selection got. The three steps are the
// distinction the update center exists to make: a selection is a request, a
// command that exited zero is an execution, and only a re-read of the manager
// says whether the entry actually moved.
type OutcomeStep string

const (
	// StepRequested: the entry was in the selection this plan carried.
	StepRequested OutcomeStep = "requested"
	// StepExecuted: the plan's command finished successfully, so the manager
	// accepted the request. It does not yet say the entry moved.
	StepExecuted OutcomeStep = "executed"
	// StepConfirmed: the manager's own answer after the run no longer lists the
	// entry as out of date.
	StepConfirmed OutcomeStep = "confirmed"
	// StepLeftOut: the entry was not carried by any command: it is pinned, or no
	// manager that owns it could build one.
	StepLeftOut OutcomeStep = "left out"
	// StepFailed: the command refused, so the entry was not moved.
	StepFailed OutcomeStep = "failed"
	// StepUnknown: the command was cancelled or interrupted, and whether the
	// entry moved is not knowable from here. It is reported as unknown rather
	// than guessed at in either direction.
	StepUnknown OutcomeStep = "unknown"
)

// Outcome is one entry's result, with the detail that makes it readable: the
// version it is on now when the manager reports one, or the manager's reason.
type Outcome struct {
	Item   Item
	Step   OutcomeStep
	Detail string
}

// ApplyResult is what one plan's run produced: the plan, the command's outcome,
// the tail of its output, the manager's fresh answer, and one outcome per entry
// the plan carried.
type ApplyResult struct {
	Plan Plan
	// Status is the command's classification: current when the command exited
	// zero, and otherwise the state the manager's output names.
	Status  Status
	Reason  string
	Output  string
	Command string
	// After is the manager's answer re-read once the command finished. It is
	// best effort: a re-read that fails leaves After empty, and every outcome is
	// then StepExecuted rather than StepConfirmed, because this package will not
	// claim a move it did not see.
	After    Result
	Outcomes []Outcome
	// Duration is how long the command ran.
	Duration string
}

// Confirmed reports how many entries the re-read proved had moved.
func (r ApplyResult) Confirmed() int { return countStep(r.Outcomes, StepConfirmed) }

// Moved reports how many entries the command was accepted for, confirmed or not.
func (r ApplyResult) Moved() int {
	return countStep(r.Outcomes, StepExecuted) + countStep(r.Outcomes, StepConfirmed)
}

// Failed reports how many entries the run did not move.
func (r ApplyResult) Failed() int {
	return countStep(r.Outcomes, StepFailed) + countStep(r.Outcomes, StepUnknown)
}

func countStep(outcomes []Outcome, step OutcomeStep) int {
	count := 0
	for _, outcome := range outcomes {
		if outcome.Step == step {
			count++
		}
	}
	return count
}

// Selection is the set of entries the user has chosen, keyed by identity. A set
// rather than a slice is the whole point of the type: the same item must not be
// counted twice when a re-read returns it again, and an item that disappears
// between the read and the selection must be dropped rather than kept as a key
// nothing answers to.
type Selection struct {
	keys map[string]bool
}

// NewSelection returns an empty selection. The zero value is empty too, and every
// method that changes a selection writes through a pointer: a selection made with
// `var s Selection` must not lose its first toggle to a copy.
func NewSelection() Selection { return Selection{keys: map[string]bool{}} }

// Count reports how many entries are selected.
func (s Selection) Count() int { return len(s.keys) }

// Has reports whether an entry is selected.
func (s Selection) Has(item Item) bool { return s.keys[item.Key()] }

// Toggle adds an entry to the selection or removes it, and reports the new state.
func (s *Selection) Toggle(item Item) bool {
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	key := item.Key()
	if s.keys[key] {
		delete(s.keys, key)
		return false
	}
	s.keys[key] = true
	return true
}

// Drop removes an entry from the selection if it is there. It is the counterpart
// Toggle needs for the one action that changes a whole group at once: selecting
// what the filter shows and then putting it back must not depend on the group
// still being on screen.
func (s *Selection) Drop(item Item) {
	if s.keys == nil {
		return
	}
	delete(s.keys, item.Key())
}

// Clear empties the selection.
func (s *Selection) Clear() {
	for key := range s.keys {
		delete(s.keys, key)
	}
}

// Retain drops every key that freshItems no longer names, so a selection made
// before a re-read cannot claim an entry the manager has stopped reporting. It
// reports how many keys it dropped.
func (s *Selection) Retain(freshItems []Item) int {
	live := make(map[string]bool, len(freshItems))
	for _, item := range freshItems {
		live[item.Key()] = true
	}
	dropped := 0
	for key := range s.keys {
		if !live[key] {
			delete(s.keys, key)
			dropped++
		}
	}
	return dropped
}

// Selected returns the entries of candidates the selection holds, in the order
// the candidates are given, which is the managers' own order.
func (s Selection) Selected(candidates []Item) []Item {
	selected := make([]Item, 0, len(s.keys))
	for _, item := range candidates {
		if s.Has(item) {
			selected = append(selected, item)
		}
	}
	return selected
}

// GroupKey is the identity one command is built for: a manager's one scope and
// kind. Entries that share it can be moved by one command; entries that do not
// need a command of their own, which is why a Homebrew look could produce one
// formula command and one cask command.
type GroupKey struct {
	Manager ManagerID
	Scope   Scope
	Kind    Kind
}

// GroupByKey groups entries by the command they would need, in the order the
// entries arrive. A group's entries keep their relative order, so the screen's
// list and the commands agree.
func GroupByKey(items []Item) []struct {
	Key   GroupKey
	Items []Item
} {
	var order []GroupKey
	groups := map[GroupKey][]Item{}
	for _, item := range items {
		key := GroupKey{Manager: item.Manager, Scope: item.Scope, Kind: item.Kind}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], item)
	}
	sorted := make([]struct {
		Key   GroupKey
		Items []Item
	}, 0, len(order))
	// The managers' own order, as the adapters are registered, and the scope and
	// kind inside it, so two runs of the same selection produce the same plans in
	// the same order.
	sort.SliceStable(order, func(a, b int) bool {
		return order[a].less(order[b])
	})
	for _, key := range order {
		sorted = append(sorted, struct {
			Key   GroupKey
			Items []Item
		}{Key: key, Items: groups[key]})
	}
	return sorted
}

// less orders two group keys by manager, then by the kind's own order, then by
// scope with the user's installation first, so a plan list is stable across runs
// and the least privileged command is the one shown first.
func (k GroupKey) less(other GroupKey) bool {
	if k.Manager != other.Manager {
		return k.Manager < other.Manager
	}
	if kindOrder(k.Kind) != kindOrder(other.Kind) {
		return kindOrder(k.Kind) < kindOrder(other.Kind)
	}
	if k.Kind != other.Kind {
		return k.Kind < other.Kind
	}
	if k.Scope != other.Scope {
		return scopeOrder(k.Scope) < scopeOrder(other.Scope)
	}
	return false
}

// kindOrder is the canonical order of the entry kinds, so formulae are planned
// before casks and an unknown kind sorts after every known one rather than before
// it.
func kindOrder(kind Kind) int {
	switch kind {
	case KindFormula:
		return 0
	case KindCask:
		return 1
	case KindPackage:
		return 2
	case KindSnap:
		return 3
	case KindFlatpak:
		return 4
	case KindToolchain:
		return 5
	}
	return 6
}

// scopeOrder puts the user's own installation before the machine's, because it is
// the one that needs nothing from anybody else.
func scopeOrder(scope Scope) int {
	if scope == ScopeSystem {
		return 1
	}
	return 0
}

// QuoteArgv renders a program and its arguments the way a reader would type
// them. It quotes for the eye only: no caller of this function ever hands the
// result to a shell.
func QuoteArgv(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		if arg != "" && !strings.ContainsAny(arg, " \t\n\"'\\$`*?[](){}|&;<>#~!") {
			quoted = append(quoted, arg)
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
	}
	return strings.Join(quoted, " ")
}

// FirstLine is the first non-empty line of a command's output, trimmed and
// bounded. It is what a result row shows when the manager's whole answer will
// not fit: one line is readable, and the rest is in the log. The bound counts
// runes, so a line of box-drawing or accented text is cut at a character rather
// than in the middle of one.
func FirstLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return boundText(line, firstLineLimit)
	}
	return ""
}

// firstLineLimit is how many runes a one-line summary keeps.
const firstLineLimit = 200

// boundText cuts text to limit runes and marks that it was cut. A text that fits
// is returned unchanged, so a caller can compare it with what the manager
// printed.
func boundText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// Tail returns the last n non-empty lines of output, oldest first. It is what a
// running screen shows: the end of the log is where the progress is.
func Tail(output string, n int) []string {
	if n <= 0 {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// RedactLine removes anything that looks like a credential from one line of a
// manager's output before it is kept or drawn. Package managers print URLs, and a
// private registry's URL can carry a token in its userinfo; a log that keeps it
// would be a log that leaks it.
//
// The rule is deliberately blunt and never tries to be clever about which lines
// are safe: everything between "//" and "@" in a URL-shaped token becomes
// "***", and a token in a `key=value` whose key names a secret is replaced. A
// line that carries no such shape is returned unchanged.
func RedactLine(line string) string {
	return redactSecrets(line)
}

// redactSecrets is RedactLine's implementation, kept separate so the shape rules
// can be tested on their own.
func redactSecrets(line string) string {
	redacted := redactURLUserinfo(line)
	return redactSecretAssignments(redacted)
}

// redactURLUserinfo replaces the userinfo of every URL-shaped token in a line.
// A token is URL-shaped when it carries "://" and an "@" after it; redacting a
// token that is merely similar costs a reader nothing, while keeping a credential
// costs the user the secret.
func redactURLUserinfo(line string) string {
	var out []byte
	for i := 0; i < len(line); {
		offset := strings.Index(line[i:], "://")
		if offset < 0 {
			out = append(out, line[i:]...)
			break
		}
		after := i + offset + len("://")
		end := after
		for end < len(line) && !strings.ContainsRune(" \t\"'`", rune(line[end])) {
			end++
		}
		if at := strings.IndexByte(line[after:end], '@'); at >= 0 {
			out = append(out, line[i:after]...)
			out = append(out, "***"...)
			out = append(out, line[after+at:]...)
		} else {
			out = append(out, line[i:end]...)
		}
		i = end
	}
	return string(out)
}

// secretKeyMarkers are the words that make a `key=value` pair a secret. A pair
// whose key contains one of them is redacted whatever the value looks like.
var secretKeyMarkers = []string{"token", "password", "passwd", "secret", "apikey", "api_key", "auth", "credential"}

// redactSecretAssignments redacts the value of every KEY=VALUE pair whose key
// names a secret.
func redactSecretAssignments(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return line
	}
	changed := false
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || value == "" {
			continue
		}
		lowered := strings.ToLower(key)
		secret := false
		for _, marker := range secretKeyMarkers {
			if strings.Contains(lowered, marker) {
				secret = true
				break
			}
		}
		if !secret {
			continue
		}
		fields[i] = key + "=***"
		changed = true
	}
	if !changed {
		return line
	}
	// Rebuilding from fields is safe here: the values that remain are the ones
	// this function decided are not secrets, and a line whose shape mattered has
	// already been redacted by the URL rule.
	return strings.Join(fields, " ")
}
