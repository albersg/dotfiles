package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BrewAdapter reads Homebrew's outdated list. It is the manager the installer
// prefers whenever it is present, and it is the only one on this path whose
// metadata lives in a git repository of its own: reading it must not update that
// repository, because what this center may do without consent is read.
type BrewAdapter struct{}

func (BrewAdapter) ID() ManagerID { return ManagerBrew }
func (BrewAdapter) Label() string { return "Homebrew" }
func (BrewAdapter) Scope() Scope  { return ScopeUser }
func (BrewAdapter) Kinds() []Kind { return []Kind{KindFormula, KindCask} }

// Probe answers where Homebrew is. It resolves brew through the environment's own
// PATH and then through Homebrew's prefix, because a run whose own PATH predates
// the Homebrew install still has to see it.
func (BrewAdapter) Probe(env Env) Availability {
	switch env.GOOS {
	case "darwin", "linux", "":
	default:
		return unsupported("Homebrew serves macOS and Linux, not " + env.GOOS)
	}
	if _, ok := env.Resolve("brew"); !ok {
		return missing("Homebrew is not installed here: no brew on PATH and none in its prefix")
	}
	return present()
}

// brewOutdated is the part of `brew outdated --json=v2` this package reads.
//
// The JSON interface is Homebrew's own and is what its other clients read. The
// package reads only the fields it needs and treats a missing one as "the manager
// did not say": an installed version it does not report is left empty rather than
// invented, and a cask whose version field is named differently is read from
// either spelling rather than from one.
type brewOutdated struct {
	Formulae []brewEntry `json:"formulae"`
	Casks    []brewEntry `json:"casks"`
}

type brewEntry struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
	Version           string   `json:"version"`
	Pinned            bool     `json:"pinned"`
}

// Read asks Homebrew what is out of date.
//
// HOMEBREW_NO_AUTO_UPDATE=1 is set on the read and deliberately not on the
// update: reading must not write Homebrew's own repository, while applying may
// leave Homebrew to do what it always does, which the plan states.
func (a BrewAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	brew, ok := env.Resolve("brew")
	if !ok {
		return result(a, env, StatusUnavailable, "Homebrew is not installed here", command("brew", "outdated"), nil)
	}
	cmd := command(brew, "outdated", "--json=v2").withEnv("HOMEBREW_NO_AUTO_UPDATE=1")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return failedResult(a, env, cmd, output)
	}

	var parsed brewOutdated
	if err := json.Unmarshal([]byte(output.Stdout), &parsed); err != nil {
		return result(a, env, StatusFailed,
			fmt.Sprintf("Homebrew's answer was not the JSON this installer reads: %v", err), cmd, nil)
	}

	var items []Item
	for _, entry := range parsed.Formulae {
		if item, ok := brewItem(entry, KindFormula); ok {
			items = append(items, item)
		}
	}
	for _, entry := range parsed.Casks {
		if item, ok := brewItem(entry, KindCask); ok {
			items = append(items, item)
		}
	}

	res := result(a, env, StatusCurrent, "", cmd, sortedItems(items))
	if len(items) > 0 {
		res.Status = StatusUpdates
	}
	// The answer is as fresh as the last `brew update`, and this read never runs
	// one. Saying so is the difference between a stale answer and a wrong one.
	res.Notes = append(res.Notes, "Homebrew's own metadata is whatever `brew update` last fetched; this check does not run it, because updating Homebrew's repository writes to it.")
	return res
}

// brewItem turns one entry of Homebrew's answer into an item. An entry with no
// current version at all is skipped rather than shown: without it there is
// nothing to compare, and a row reading "→ 1.2" is not an update.
func brewItem(entry brewEntry, kind Kind) (Item, bool) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return Item{}, false
	}
	available := strings.TrimSpace(entry.CurrentVersion)
	if available == "" {
		available = strings.TrimSpace(entry.Version)
	}
	if available == "" {
		return Item{}, false
	}
	current := ""
	if versions := entry.InstalledVersions; len(versions) > 0 {
		current = strings.TrimSpace(versions[len(versions)-1])
	}

	item := Item{
		Manager:   ManagerBrew,
		Name:      name,
		Scope:     ScopeUser,
		Kind:      kind,
		Current:   current,
		Available: available,
		Pinned:    entry.Pinned,
	}
	if entry.Pinned {
		item.PinReason = "pinned with `brew pin`; moving it needs `brew unpin " + name + "`"
	}
	if len(entry.InstalledVersions) > 1 {
		item.Note = itemNote(item.Note, fmt.Sprintf("%d versions are installed; the newest is shown", len(entry.InstalledVersions)))
	}
	if current == "" {
		item.Note = itemNote(item.Note, "Homebrew did not report the installed version")
	}
	return item, true
}

// Plan builds the one command that moves a Homebrew selection. Formulae and
// casks need two different commands, which is why a plan carries one kind and the
// center builds one plan per kind.
func (a BrewAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no Homebrew entries were selected")
	}
	kind := items[0].Kind
	if kind != KindFormula && kind != KindCask {
		return Plan{}, fmt.Errorf("Homebrew cannot update an entry of kind %q", kind)
	}
	for _, item := range items {
		if item.Kind != kind {
			return Plan{}, errors.New("Homebrew needs one command per kind of entry: formulae and casks cannot be moved together")
		}
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
	}
	brew, ok := env.Resolve("brew")
	if !ok {
		return Plan{}, errors.New("Homebrew is not installed here")
	}

	argv := []string{brew, "upgrade"}
	if kind == KindCask {
		argv = append(argv, "--cask")
	}
	for _, item := range items {
		argv = append(argv, item.Name)
	}

	plan := Plan{
		Manager: ManagerBrew,
		Scope:   ScopeUser,
		Kind:    kind,
		Command: Command{Argv: argv, Timeout: applyTimeout},
		Items:   items,
		Restart: "A formula that ships a background service keeps the one already running until it is restarted " +
			"(`brew services restart <name>`), and a program that is running keeps its old version until it is started again.",
		Rollback: "Homebrew keeps the version it replaced only until its next cleanup, and the old version may have " +
			"to be fetched again, so this is not a rollback.",
		Notes: []string{
			"Homebrew refreshes its own metadata first unless HOMEBREW_NO_AUTO_UPDATE is set; the plan leaves Homebrew to its own default.",
			"Homebrew resolves dependencies itself and may install or move packages the selection did not name.",
		},
	}
	return plan, nil
}
