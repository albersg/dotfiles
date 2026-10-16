package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// AptAdapter reads Debian's and Ubuntu's package manager. Its scope is the
// machine's, and every command that moves anything needs an administrator.
//
// The read is a simulation: `apt-get --simulate upgrade` prints the transaction
// it would perform and touches nothing. `-o Debug::NoLocking=1` matters here as
// much as in a real run -- a simulation also takes apt's lock, and the check must
// not fail merely because an unattended-upgrade is running, nor hold up one.
type AptAdapter struct{}

func (AptAdapter) ID() ManagerID { return ManagerApt }
func (AptAdapter) Label() string { return "APT (Debian/Ubuntu)" }
func (AptAdapter) Scope() Scope  { return ScopeSystem }
func (AptAdapter) Kinds() []Kind { return []Kind{KindPackage} }

// Probe answers where apt is. Termux is excluded on purpose: its packages come
// from Termux's own repository through pkg, and offering apt on Termux would offer
// the wrong channel for the same files.
func (AptAdapter) Probe(env Env) Availability {
	if env.GOOS != "linux" && env.GOOS != "" {
		return unsupported("apt serves Linux hosts, not " + env.GOOS)
	}
	if env.Termux {
		return unsupported("this host is Termux, whose channel is pkg")
	}
	if _, ok := env.Resolve("apt-get"); !ok {
		return missing("apt-get is not installed here")
	}
	return present()
}

// aptSimulationArgs is the read: a simulated upgrade with apt's lock disabled.
// The empty command name is filled in by the caller, so the same arguments serve
// apt on a Debian host and the apt that backs Termux's pkg channel.
var aptSimulationArgs = []string{"--simulate", "-o", "Debug::NoLocking=1", "upgrade"}

// ParseAptSimulation reads the transaction `apt-get --simulate upgrade` prints.
//
// The lines are the transaction's own: `Inst name [old] (new ...)` is an upgrade,
// `Inst name (new ...)` is a package the transaction would newly install and is
// not an update of anything installed, and `Conf name (...)` is configuration of
// an already-listed package. Everything else -- the counts, the summary, apt's
// warnings -- is ignored.
//
// The kept-back list is returned separately rather than as items: apt is holding
// those packages back on its own dependency grounds, and a package apt refuses to
// upgrade in a plain `upgrade` has no available version to offer here.
func ParseAptSimulation(output string) (items []Item, keptBack []string) {
	inKeptBack := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Inst "):
			name, current, available, ok := parseAptInstLine(trimmed)
			// An `Inst` line without a version in brackets is a package the
			// transaction would newly install. It is not an update of anything
			// installed, and offering it here would be offering an install
			// through a screen that only moves what is already there.
			if ok && current != "" {
				items = append(items, Item{
					Manager:   ManagerApt,
					Name:      name,
					Scope:     ScopeSystem,
					Kind:      KindPackage,
					Current:   current,
					Available: available,
				})
			}
		case strings.HasPrefix(trimmed, "The following packages have been kept back"):
			// apt prints the names on the following indented lines, and on some
			// releases on this line after a colon instead.
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "The following packages have been kept back"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
			if rest != "" {
				keptBack = append(keptBack, strings.Fields(rest)...)
			} else {
				inKeptBack = true
			}
		case inKeptBack:
			if trimmed == "" || !strings.HasPrefix(line, "  ") {
				inKeptBack = false
				continue
			}
			keptBack = append(keptBack, strings.Fields(trimmed)...)
		}
	}
	return items, keptBack
}

// parseAptInstLine reads one `Inst` line: the name, the version being replaced in
// brackets when the package is installed, and the version to install inside the
// parentheses.
//
// The bracket is looked for in the part before the parentheses on purpose: the
// architecture inside the parentheses is written in square brackets too, and a
// search over the whole line would read `[amd64]` as the version being replaced.
func parseAptInstLine(line string) (name, current, available string, ok bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "Inst "))
	if rest == "" {
		return "", "", "", false
	}
	name, rest = cutField(rest)
	open := strings.Index(rest, "(")
	if name == "" || open < 0 {
		return "", "", "", false
	}
	head := rest[:open]
	if bracketOpen := strings.Index(head, "["); bracketOpen >= 0 {
		if close := strings.Index(head[bracketOpen:], "]"); close > 0 {
			current = strings.TrimSpace(head[bracketOpen+1 : bracketOpen+close])
		}
	}
	available = strings.TrimSpace(rest[open+1:])
	if end := strings.IndexAny(available, " )"); end >= 0 {
		available = strings.TrimSpace(available[:end])
	}
	if available == "" {
		return "", "", "", false
	}
	return name, current, available, true
}

// cutField splits the first whitespace-delimited field from the rest.
func cutField(value string) (field, rest string) {
	at := strings.IndexAny(value, " \t")
	if at < 0 {
		return value, ""
	}
	return value[:at], strings.TrimSpace(value[at:])
}

// Read simulates the upgrade apt would perform and reads the holds beside it.
func (a AptAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("apt-get"); !ok {
		return result(a, env, StatusUnavailable, "apt-get is not installed here", command("apt-get", aptSimulationArgs...), nil)
	}

	cmd := command("apt-get", aptSimulationArgs...)
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	items, keptBack := ParseAptSimulation(output.Stdout)
	pins := aptHolds(ctx, runner, env)
	items = aptApplyHolds(items, pins)

	res := result(a, env, StatusCurrent, "", cmd, sortedItems(items))
	if len(items) > 0 {
		res.Status = StatusUpdates
	}
	if len(keptBack) > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf(
			"apt is holding these back on its own dependency grounds, so they are not offered here: %s",
			strings.Join(keptBack, ", ")))
	}
	res.Notes = append(res.Notes, aptReadNotes...)
	return res
}

// aptReadNotes are the caveats both apt-family reads carry. They are shared so a
// Debian host and a Termux host read the same sentence about what the simulation
// is.
var aptReadNotes = []string{
	"the list is apt's own simulation of an upgrade, so a version it cannot reach with the metadata it has cached is not listed",
	"the installed and available versions are the ones apt printed, kept as printed",
}

// aptHolds reads apt's holds through apt-mark. A machine without apt-mark has no
// holds this package can read, and saying nothing is better than claiming there
// are none: the caller adds the note.
func aptHolds(ctx context.Context, runner Runner, env Env) map[string]bool {
	if _, ok := env.Resolve("apt-mark"); !ok {
		return nil
	}
	cmd := command("apt-mark", "showhold")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return nil
	}
	holds := map[string]bool{}
	for _, line := range strings.Split(output.Stdout, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		holds[name] = true
	}
	return holds
}

// aptApplyHolds marks the entries apt has been told to hold. It works on a copy
// of the slice the parser produced; the marks belong to the manager's state and
// never to the parser's output.
func aptApplyHolds(items []Item, holds map[string]bool) []Item {
	if len(holds) == 0 {
		return items
	}
	for i := range items {
		if holds[items[i].Name] {
			items[i].Pinned = true
			items[i].PinReason = "held with `apt-mark hold`; moving it needs `sudo apt-mark unhold " + items[i].Name + "`"
		}
	}
	return items
}

// Plan builds the one command that moves an apt selection.
//
// Three options are not optional here: `--only-upgrade` keeps the command from
// installing a package that is not installed, `-y` keeps apt from asking a
// question nobody can answer on a full-screen interface, and
// DEBIAN_FRONTEND=noninteractive is what keeps dpkg's own configuration prompts
// from appearing. The two dpkg options are the noninteractive dpkg policy: keep
// the configuration files that are on the machine, and never ask about them. The
// plan states that, because silently keeping a configuration file is exactly the
// kind of consequence a user has to be told about before confirming.
func (a AptAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no apt packages were selected")
	}
	for _, item := range items {
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
		if item.Kind != KindPackage {
			return Plan{}, fmt.Errorf("apt cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("apt-get"); !ok {
		return Plan{}, errors.New("apt-get is not installed here")
	}

	argv := []string{
		"apt-get", "install", "-y", "--only-upgrade",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
	}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerApt,
		Scope:   ScopeSystem,
		Kind:    KindPackage,
		Command: Command{
			Argv:    argv,
			Elevate: true,
			Env:     []string{"DEBIAN_FRONTEND=noninteractive"},
			Timeout: applyTimeout,
		},
		Items: items,
		Restart: "A kernel, a C library or a service package keeps the running version until the machine or the " +
			"service is restarted; this run restarts nothing and does not reboot.",
		Rollback: "apt keeps a downloaded .deb only while its cache holds it, and the previous version is usually no " +
			"longer downloadable, so a downgrade is not a rollback.",
		Notes: []string{
			"dpkg is told to keep your existing configuration files and never prompt (`--force-confdef --force-confold`).",
			"apt resolves dependencies itself and may install or move packages the selection did not name.",
			"apt holds its own lock while this runs; another apt or dpkg running at the same time makes it refuse, and nothing here runs two managers at once.",
		},
	}, nil
}
