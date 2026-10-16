package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SnapAdapter reads snapd's channel. A snap is one of the machine's, and
// refreshing it needs the administrator the snapd socket asks for.
//
// The read is snapd's own listing pair: `snap list` for what is installed and
// `snap refresh --list` for what the store has that is newer. The two are joined
// by name, and snapd's tables are aligned rather than delimited, which is why the
// rows are split on runs of spaces.
type SnapAdapter struct{}

func (SnapAdapter) ID() ManagerID { return ManagerSnap }
func (SnapAdapter) Label() string { return "snap" }
func (SnapAdapter) Scope() Scope  { return ScopeSystem }
func (SnapAdapter) Kinds() []Kind { return []Kind{KindSnap} }

// Probe answers where snap is.
func (SnapAdapter) Probe(env Env) Availability {
	if env.GOOS != "linux" && env.GOOS != "" {
		return unsupported("snap serves Linux hosts, not " + env.GOOS)
	}
	if env.Termux {
		return unsupported("this host is Termux, which does not carry snapd")
	}
	if _, ok := env.Resolve("snap"); !ok {
		return missing("snap is not installed here")
	}
	return present()
}

// ParseSnapList reads `snap list`: one row per installed snap, whose first field
// is the name and whose second is the installed version. The header row and
// snapd's notes are skipped rather than guessed at.
func ParseSnapList(output string) map[string]string {
	versions := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Name" || fields[0] == "error:" {
			continue
		}
		versions[fields[0]] = fields[1]
	}
	return versions
}

// ParseSnapRefreshList reads `snap refresh --list`: one row per snap the store
// has a newer revision of. The first field is the name and the second the version
// the store would install.
func ParseSnapRefreshList(output string) []Item {
	var items []Item
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Name" || fields[0] == "error:" {
			continue
		}
		item := Item{
			Manager:   ManagerSnap,
			Name:      fields[0],
			Scope:     ScopeSystem,
			Kind:      KindSnap,
			Available: fields[1],
		}
		if len(fields) > 5 {
			notes := strings.Join(fields[5:], " ")
			if strings.Contains(strings.ToLower(notes), "held") {
				item.Pinned = true
				item.PinReason = "held by snapd; moving it needs the hold released with `sudo snap refresh --unhold " + fields[0] + "`"
			}
		}
		items = append(items, item)
	}
	return items
}

// ParseSnapHeld reads `snap held`, whose rows name the snaps snapd is holding.
// The command exists only on newer snapd, so an older machine simply has no holds
// this package can read.
func ParseSnapHeld(output string) map[string]bool {
	held := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		if fields[0] == "Name" || fields[0] == "error:" || strings.HasPrefix(fields[0], "No") {
			continue
		}
		held[fields[0]] = true
	}
	return held
}

// Read lists what snapd has that is newer and joins the installed versions onto
// it.
func (a SnapAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("snap"); !ok {
		return result(a, env, StatusUnavailable, "snap is not installed here", command("snap", "refresh", "--list"), nil)
	}

	cmd := command("snap", "refresh", "--list")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	items := ParseSnapRefreshList(output.Stdout)
	if len(items) == 0 {
		res := result(a, env, StatusCurrent, "", cmd, nil)
		res.Notes = append(res.Notes, snapReadNotes...)
		return res
	}

	if listOutput := command("snap", "list").run(ctx, runner, env); listOutput.ExitCode == 0 {
		installed := ParseSnapList(listOutput.Stdout)
		for i := range items {
			items[i].Current = installed[items[i].Name]
		}
	}

	if heldOutput := command("snap", "held").run(ctx, runner, env); heldOutput.ExitCode == 0 {
		held := ParseSnapHeld(heldOutput.Stdout)
		for i := range items {
			if held[items[i].Name] {
				items[i].Pinned = true
				items[i].PinReason = "held by snapd; moving it needs the hold released with `sudo snap refresh --unhold " + items[i].Name + "`"
			}
		}
	}

	res := result(a, env, StatusUpdates, "", cmd, sortedItems(items))
	res.Notes = append(res.Notes, snapReadNotes...)
	return res
}

// snapReadNotes are the caveats a snap listing carries.
var snapReadNotes = []string{
	"the list comes from the snap store, so a machine that cannot reach it reports that instead of an empty answer",
	"snapd's own `snap refresh --list` is what answered, and the installed versions come from `snap list`",
}

// Plan builds the one command that moves a snap selection.
func (a SnapAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no snaps were selected")
	}
	for _, item := range items {
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
		if item.Kind != KindSnap {
			return Plan{}, fmt.Errorf("snap cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("snap"); !ok {
		return Plan{}, errors.New("snap is not installed here")
	}

	argv := []string{"snap", "refresh"}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerSnap,
		Scope:   ScopeSystem,
		Kind:    KindSnap,
		Command: Command{Argv: argv, Elevate: true, Env: []string{"LC_ALL=C"}, Timeout: applyTimeout},
		Items:   items,
		Restart: "snapd restarts a refreshed service snap itself; a running application keeps its old version until " +
			"it is closed and reopened.",
		Rollback: "snap keeps the revision it replaced until snapd's own cleanup removes it, so `sudo snap revert " +
			"<name>` puts it back while it is still there. This run keeps no copy of its own.",
		Notes: []string{
			"snapd refreshes a snap together with the bases and content snaps it needs, so it may move snaps the selection did not name.",
			"snapd serialises its own refreshes; a refresh already in progress makes this one wait or refuse.",
		},
	}, nil
}
