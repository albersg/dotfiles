package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// FlatpakAdapter reads Flatpak's channel. It is the one adapter whose scope is
// per entry rather than per manager: Flatpak can hold the same application in a
// per-user installation and in the machine's own, and those are two deployments
// that must stay two rows.
type FlatpakAdapter struct{}

func (FlatpakAdapter) ID() ManagerID { return ManagerFlatpak }
func (FlatpakAdapter) Label() string { return "Flatpak" }
func (FlatpakAdapter) Scope() Scope  { return ScopeUser }
func (FlatpakAdapter) Kinds() []Kind { return []Kind{KindFlatpak} }

// Probe answers where flatpak is.
func (FlatpakAdapter) Probe(env Env) Availability {
	if env.GOOS != "linux" && env.GOOS != "" {
		return unsupported("Flatpak serves Linux hosts, not " + env.GOOS)
	}
	if env.Termux {
		return unsupported("this host is Termux, which does not carry Flatpak")
	}
	if _, ok := env.Resolve("flatpak"); !ok {
		return missing("Flatpak is not installed here")
	}
	return present()
}

// ParseFlatpakList reads `flatpak list --columns=application,branch,installation`:
// the installations a machine has of each application. The installation is
// `system` or `user`, and both spellings are flatpak's own.
func ParseFlatpakList(output string) map[string]Scope {
	installations := map[string]Scope{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		app := strings.TrimSpace(fields[0])
		branch := strings.TrimSpace(fields[1])
		if app == "" || branch == "" {
			continue
		}
		installations[app+"\t"+branch] = flatpakScope(fields[2])
	}
	return installations
}

// flatpakScope reads flatpak's own word for an installation.
func flatpakScope(field string) Scope {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "system":
		return ScopeSystem
	default:
		return ScopeUser
	}
}

// ParseFlatpakUpdates reads `flatpak remote-ls --updates
// --columns=application,branch,version`: one row per application the remotes have
// a newer version of, in the order the columns were asked for.
func ParseFlatpakUpdates(output string) []struct {
	App    string
	Branch string
	Latest string
} {
	var rows []struct {
		App    string
		Branch string
		Latest string
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		app := strings.TrimSpace(fields[0])
		branch := strings.TrimSpace(fields[1])
		latest := strings.TrimSpace(fields[2])
		if app == "" || latest == "" {
			continue
		}
		rows = append(rows, struct {
			App    string
			Branch string
			Latest string
		}{App: app, Branch: branch, Latest: latest})
	}
	return rows
}

// Read lists what Flatpak's remotes have that is newer, per installation.
func (a FlatpakAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("flatpak"); !ok {
		return result(a, env, StatusUnavailable, "Flatpak is not installed here", command("flatpak", "remote-ls", "--updates"), nil)
	}

	cmd := command("flatpak", "remote-ls", "--updates", "--columns=application,branch,version")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	rows := ParseFlatpakUpdates(output.Stdout)
	if len(rows) == 0 {
		res := result(a, env, StatusCurrent, "", cmd, nil)
		res.Notes = append(res.Notes, flatpakReadNotes...)
		return res
	}

	installations := map[string]Scope{}
	if listed := command("flatpak", "list", "--columns=application,branch,installation").run(ctx, runner, env); listed.ExitCode == 0 {
		installations = ParseFlatpakList(listed.Stdout)
	}

	var items []Item
	for _, row := range rows {
		scope := ScopeUser
		known := false
		if installed, ok := installations[row.App+"\t"+row.Branch]; ok {
			scope, known = installed, true
		}
		item := Item{
			Manager:   ManagerFlatpak,
			Name:      row.App,
			Scope:     scope,
			Kind:      KindFlatpak,
			Available: row.Latest,
		}
		if !known {
			item.Note = itemNote(item.Note, "the installation this deployment belongs to could not be read; the user installation is assumed, and the plan says which installation it named")
		}
		items = append(items, item)
	}

	res := result(a, env, StatusUpdates, "", cmd, sortedItems(items))
	res.Notes = append(res.Notes, flatpakReadNotes...)
	return res
}

// flatpakReadNotes are the caveats a Flatpak listing carries.
var flatpakReadNotes = []string{
	"flatpak's own `remote-ls --updates` is what answered; an installation whose remote cannot be reached is missing from the answer rather than reported by it",
	"every runtime the listed applications need is refreshed by the same update, so the run may move runtimes the selection did not name",
}

// Plan builds the one command that moves a Flatpak selection. Flatpak names the
// installation on the command line, so a selection of applications from both
// installations is two commands, which the center builds by scope.
func (a FlatpakAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no Flatpak applications were selected")
	}
	scope := items[0].Scope
	for _, item := range items {
		if item.Scope != scope {
			return Plan{}, errors.New("Flatpak names one installation per command: a user and a system selection cannot be moved together")
		}
		if item.Kind != KindFlatpak {
			return Plan{}, fmt.Errorf("Flatpak cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("flatpak"); !ok {
		return Plan{}, errors.New("Flatpak is not installed here")
	}

	elevate := scope == ScopeSystem
	argv := []string{"flatpak", "update", "--noninteractive", "-y"}
	if elevate {
		argv = append(argv, "--system")
	} else {
		argv = append(argv, "--user")
	}
	for _, item := range items {
		argv = append(argv, item.Name)
	}

	plan := Plan{
		Manager: ManagerFlatpak,
		Scope:   scope,
		Kind:    KindFlatpak,
		Command: Command{Argv: argv, Elevate: elevate, Env: []string{"LC_ALL=C"}, Timeout: applyTimeout},
		Items:   items,
		Restart: "A running application keeps the version it was started with until it is closed and reopened; " +
			"flatpak refreshes the files on disk underneath it.",
		Rollback: "flatpak keeps the deployment it replaced until `flatpak uninstall --unused` removes it, so the " +
			"previous revision can be deployed again while it is still there. This run keeps no copy of its own.",
		Notes: []string{
			"flatpak installs nothing that is not already installed; it moves the applications named and the runtimes they need.",
		},
	}
	if elevate {
		plan.Notes = append(plan.Notes, "the system installation needs an administrator; without one the command is refused rather than left waiting on a prompt.")
	}
	return plan, nil
}
