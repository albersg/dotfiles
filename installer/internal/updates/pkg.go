package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PkgAdapter reads Termux's package channel. Termux is not a distribution: its
// packages come from its own repository into its own prefix, so its updates are
// Termux's own and never the host distribution's.
//
// The read is the apt simulation the channel is built on -- pkg drives apt and
// dpkg underneath -- while the update itself goes through pkg, which is the
// manager the machine owns. Reading through the backend and writing through the
// wrapper is deliberate: `pkg list-upgradable` exists but its output layout is
// Termux's own and has changed between releases, while the simulated transaction
// is apt's, which is the interface this package already parses for Debian.
type PkgAdapter struct{}

func (PkgAdapter) ID() ManagerID { return ManagerPkg }
func (PkgAdapter) Label() string { return "pkg (Termux)" }
func (PkgAdapter) Scope() Scope  { return ScopeUser }
func (PkgAdapter) Kinds() []Kind { return []Kind{KindPackage} }

// Probe answers where pkg is. A host that is not Termux is not a failure: it is
// simply not this channel's.
func (PkgAdapter) Probe(env Env) Availability {
	if !env.Termux {
		return unsupported("this host is not Termux, whose channel pkg is")
	}
	if _, ok := env.Resolve("pkg"); !ok {
		return missing("Termux's package manager (pkg) is not on PATH")
	}
	if _, ok := env.Resolve("apt-get"); !ok {
		return missing("Termux's apt backend (apt-get) is not on PATH, so the channel cannot be read")
	}
	return present()
}

// Read simulates the upgrade pkg would perform and reads Termux's holds beside
// it.
func (a PkgAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if !env.Termux || !resolvable(env, "pkg") || !resolvable(env, "apt-get") {
		return result(a, env, StatusUnavailable, "the Termux package channel is not available here", command("pkg", "upgrade"), nil)
	}

	cmd := command("apt-get", aptSimulationArgs...)
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	items, keptBack := ParseAptSimulation(output.Stdout)
	for i := range items {
		// The parsed entries say apt because the simulation is apt's; the channel
		// that owns them here is pkg's, and an item's identity must say so.
		items[i].Manager = ManagerPkg
		items[i].Scope = ScopeUser
	}
	pins, holdReason := aptHolds(ctx, runner, env)
	items = aptApplyHolds(items, pins)

	res := result(a, env, StatusCurrent, "", cmd, sortedItems(items))
	if len(items) > 0 {
		res.Status = StatusUpdates
	}
	if len(keptBack) > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf(
			"the channel is holding these back on its own dependency grounds, so they are not offered here: %s",
			strings.Join(keptBack, ", ")))
	}
	if holdReason != "" {
		res.Notes = append(res.Notes, "the channel's hold list could not be read here ("+holdReason+
			"), so an entry it is holding may be offered as movable")
	}
	res.Notes = append(res.Notes, "Termux's packages are its own: they are not the distribution's, even though dpkg installs them.")
	return res
}

// Plan builds the one command that moves a Termux selection. The command is
// pkg's own -- `pkg install` moves an installed package to the newest version the
// channel carries, which is exactly an update, and `-y` keeps it from asking a
// question nobody can answer on a full-screen interface.
func (a PkgAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no Termux packages were selected")
	}
	for _, item := range items {
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
		if item.Kind != KindPackage {
			return Plan{}, fmt.Errorf("pkg cannot update an entry of kind %q", item.Kind)
		}
	}
	if !resolvable(env, "pkg") {
		return Plan{}, errors.New("Termux's pkg is not on PATH")
	}

	argv := []string{"pkg", "install", "-y"}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerPkg,
		Scope:   ScopeUser,
		Kind:    KindPackage,
		Command: Command{
			Argv:    argv,
			Env:     []string{"DEBIAN_FRONTEND=noninteractive"},
			Timeout: applyTimeout,
		},
		Items: items,
		Restart: "A running program keeps the version it started with until it is started again; Termux's own " +
			"components may ask for the application to be restarted.",
		Rollback: "Termux keeps no previous version in its own channel, so there is nothing here to go back to.",
		Notes: []string{
			"the channel resolves dependencies itself and may install or move packages the selection did not name.",
			"pkg holds its own lock while this runs; another pkg or apt running at the same time makes it refuse, and nothing here runs two managers at once.",
		},
	}, nil
}

// resolvable reports whether a program is on this environment's PATH.
func resolvable(env Env, program string) bool {
	_, ok := env.Resolve(program)
	return ok
}
