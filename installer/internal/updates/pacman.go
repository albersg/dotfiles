package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PacmanAdapter reads Arch's package manager. Its scope is the machine's.
//
// `pacman -Qu` is the read: it lists the packages whose installed version is
// older than the one in the sync databases, and it is a query, so it changes
// nothing and does not take the transaction lock the way `pacman -Su` would.
type PacmanAdapter struct{}

func (PacmanAdapter) ID() ManagerID { return ManagerPacman }
func (PacmanAdapter) Label() string { return "pacman (Arch)" }
func (PacmanAdapter) Scope() Scope  { return ScopeSystem }
func (PacmanAdapter) Kinds() []Kind { return []Kind{KindPackage} }

// Probe answers where pacman is.
func (PacmanAdapter) Probe(env Env) Availability {
	if env.GOOS != "linux" && env.GOOS != "" {
		return unsupported("pacman serves Arch Linux hosts, not " + env.GOOS)
	}
	if env.Termux {
		return unsupported("this host is Termux, whose channel is pkg")
	}
	if _, ok := env.Resolve("pacman"); !ok {
		return missing("pacman is not installed here")
	}
	return present()
}

// ParsePacmanOutdated reads `pacman -Qu`, whose lines are
// `name oldversion -> newversion`, with `[ignored]` appended for a package
// pacman's own ignore list holds. The arrow is matched as a field so a package
// name or a version that contains it cannot be mistaken for the separator.
//
// A line without the arrow is skipped rather than guessed at: pacman prints
// warnings on the same stream, and a warning is not a package.
func ParsePacmanOutdated(output string, ignored map[string]bool) []Item {
	var items []Item
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		name, current, available, isIgnored, ok := parsePacmanLine(trimmed)
		if !ok {
			continue
		}
		item := Item{
			Manager:   ManagerPacman,
			Name:      name,
			Scope:     ScopeSystem,
			Kind:      KindPackage,
			Current:   current,
			Available: available,
		}
		switch {
		case isIgnored:
			item.Pinned = true
			item.PinReason = "listed in pacman's IgnorePkg; moving it needs the package removed from that list"
		case ignored[name]:
			item.Pinned = true
			item.PinReason = "listed in pacman's IgnorePkg; moving it needs the package removed from that list"
		}
		items = append(items, item)
	}
	return items
}

// parsePacmanLine reads one `pacman -Qu` row.
func parsePacmanLine(line string) (name, current, available string, ignored, ok bool) {
	fields := strings.Fields(line)
	arrow := -1
	for i, field := range fields {
		if field == "->" {
			arrow = i
			break
		}
	}
	if arrow < 1 || arrow+1 >= len(fields) {
		return "", "", "", false, false
	}
	name = fields[0]
	current = fields[arrow-1]
	available = fields[arrow+1]
	for _, field := range fields[arrow+2:] {
		if strings.EqualFold(field, "[ignored]") {
			ignored = true
		}
	}
	if name == "" || available == "" {
		return "", "", "", false, false
	}
	return name, current, available, ignored, true
}

// Read lists what pacman reports as out of date and reads its ignore list beside
// it.
func (a PacmanAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("pacman"); !ok {
		return result(a, env, StatusUnavailable, "pacman is not installed here", command("pacman", "-Qu"), nil)
	}

	cmd := command("pacman", "-Qu")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	ignored, ignoreNote := pacmanIgnoreList(ctx, runner, env)
	items := ParsePacmanOutdated(output.Stdout, ignored)

	res := result(a, env, StatusCurrent, "", cmd, sortedItems(items))
	if len(items) > 0 {
		res.Status = StatusUpdates
	}
	if ignoreNote != "" {
		res.Notes = append(res.Notes, ignoreNote)
	}
	res.Notes = append(res.Notes, "pacman's answer is as fresh as the last `pacman -Sy`; this check never refreshes the databases, because that writes to the machine's package state.")
	return res
}

// pacmanIgnoreList reads pacman's IgnorePkg through pacman-conf, the reader
// pacman ships for its own configuration. A machine without it keeps its ignore
// list where this package will not guess at it, and the note says so rather than
// claiming there are no holds.
func pacmanIgnoreList(ctx context.Context, runner Runner, env Env) (map[string]bool, string) {
	if _, ok := env.Resolve("pacman-conf"); !ok {
		return nil, "pacman's ignore list could not be read (pacman-conf is not installed), so a package it holds may appear here as an update"
	}
	cmd := command("pacman-conf", "IgnorePkg")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return nil, "pacman's ignore list could not be read, so a package it holds may appear here as an update"
	}
	ignored := map[string]bool{}
	for _, line := range strings.Split(output.Stdout, "\n") {
		for _, name := range strings.Fields(line) {
			ignored[name] = true
		}
	}
	return ignored, ""
}

// Plan builds the one command that moves a pacman selection.
//
// `--needed` is what keeps pacman from reinstalling a package that is already at
// the version the selection named, and `--noconfirm` is what keeps it from asking
// on a screen that cannot answer. Pacman's own cache is the one place on this path
// where going back is genuinely possible, which the plan states exactly.
func (a PacmanAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no pacman packages were selected")
	}
	for _, item := range items {
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
		if item.Kind != KindPackage {
			return Plan{}, fmt.Errorf("pacman cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("pacman"); !ok {
		return Plan{}, errors.New("pacman is not installed here")
	}

	argv := []string{"pacman", "-S", "--noconfirm", "--needed"}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerPacman,
		Scope:   ScopeSystem,
		Kind:    KindPackage,
		Command: Command{Argv: argv, Elevate: true, Timeout: applyTimeout},
		Items:   items,
		Restart: "A kernel, a C library or a service package keeps the running version until the machine or the " +
			"service is restarted; this run restarts nothing and does not reboot.",
		Rollback: "pacman keeps the package it replaced in its own cache until that cache is cleaned, so " +
			"`sudo pacman -U /var/cache/pacman/pkg/<name>-<old version>-<arch>.pkg.tar.zst` can put the previous " +
			"version back while it is still there. This run keeps no copy of its own.",
		Notes: []string{
			"pacman resolves dependencies itself and may install or move packages the selection did not name.",
			"pacman takes its database lock while this runs; another pacman holding it makes this refuse, and nothing here runs two managers at once.",
		},
	}, nil
}
