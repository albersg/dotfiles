package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RustupAdapter reads rustup, the modern runtime manager this installer
// provisions. It is here as a manager of toolchains rather than of packages,
// because the mission is about everything installed on the machine and a
// toolchain is one of them.
//
// `rustup check` is the read: it is rustup's own answer about the toolchains it
// has installed and the ones its channels carry. Nothing here reads a project:
// a project's `rust-toolchain.toml` is that project's decision, this package never
// writes one and never overrides one.
type RustupAdapter struct{}

func (RustupAdapter) ID() ManagerID { return ManagerRustup }
func (RustupAdapter) Label() string { return "rustup (Rust toolchains)" }
func (RustupAdapter) Scope() Scope  { return ScopeUser }
func (RustupAdapter) Kinds() []Kind { return []Kind{KindToolchain} }

// Probe answers where rustup is.
func (RustupAdapter) Probe(env Env) Availability {
	if env.Termux {
		return unsupported("this host is Termux; a rustup toolchain is managed by the Termux channel there")
	}
	if _, ok := env.Resolve("rustup"); !ok {
		return missing("rustup is not installed here")
	}
	return present()
}

// rustToolchainUpdate is one toolchain rustup reports a newer version of.
type rustToolchainUpdate struct {
	Name      string
	Current   string
	Available string
}

// ParseRustupCheck reads `rustup check`. Its lines are
//
//	toolchain - Up to date : 1.79.0
//	toolchain - Update available : 1.79.0 -> 1.80.0
//
// and a line with neither shape is returned in unread, so a caller can say that
// rustup's answer had parts it did not understand instead of reporting them as
// "up to date".
func ParseRustupCheck(output string) (updates []rustToolchainUpdate, unread []string) {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		name, rest, ok := strings.Cut(trimmed, " - ")
		if !ok {
			unread = append(unread, trimmed)
			continue
		}
		name, rest = strings.TrimSpace(name), strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, "Up to date"):
			continue
		case strings.HasPrefix(rest, "Update available"):
			_, versions, ok := strings.Cut(rest, ":")
			if !ok {
				unread = append(unread, trimmed)
				continue
			}
			current, available, ok := strings.Cut(strings.TrimSpace(versions), "->")
			if !ok {
				unread = append(unread, trimmed)
				continue
			}
			updates = append(updates, rustToolchainUpdate{
				Name:      name,
				Current:   strings.TrimSpace(current),
				Available: strings.TrimSpace(available),
			})
		default:
			unread = append(unread, trimmed)
		}
	}
	return updates, unread
}

// Read asks rustup what it has that is newer.
//
// rustup's exit status is not read as the answer: some of its releases report
// available updates through a non-zero status and some do not, so a parsed answer
// with entries is taken as the answer whatever the status, and only an answer with
// nothing in it is classified.
func (a RustupAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("rustup"); !ok {
		return result(a, env, StatusUnavailable, "rustup is not installed here", command("rustup", "check"), nil)
	}

	cmd := command("rustup", "check")
	output := cmd.run(ctx, runner, env)
	updates, unread := ParseRustupCheck(output.Stdout)

	if len(updates) == 0 {
		if status, _ := Classify(output); status != StatusCurrent {
			return failedResult(a, env, cmd, output)
		}
		if len(unread) > 0 {
			return result(a, env, StatusFailed, fmt.Sprintf(
				"rustup's answer was not in the shape this installer reads: %s", boundEvidence(unread[0])), cmd, nil)
		}
		res := result(a, env, StatusCurrent, "", cmd, nil)
		res.Notes = append(res.Notes, rustupReadNotes...)
		return res
	}

	items := make([]Item, 0, len(updates))
	for _, update := range updates {
		items = append(items, Item{
			Manager:   ManagerRustup,
			Name:      update.Name,
			Scope:     ScopeUser,
			Kind:      KindToolchain,
			Current:   update.Current,
			Available: update.Available,
		})
	}

	res := result(a, env, StatusUpdates, "", cmd, sortedItems(items))
	if len(unread) > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf(
			"rustup answered %d line(s) this installer does not read: %s", len(unread), boundEvidence(unread[0])))
	}
	res.Notes = append(res.Notes, rustupReadNotes...)
	return res
}

// rustupReadNotes are the caveats a rustup answer carries.
var rustupReadNotes = []string{
	"a project's rust-toolchain.toml pins a toolchain for that project; this check never reads it, and this run never writes one",
	"the toolchains named are the ones rustup has installed; an override in a directory does not change what is installed",
}

// Plan builds the one command that moves a rustup selection. `rustup update`
// names the toolchains to move, which is precisely the selection.
func (a RustupAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no rustup toolchains were selected")
	}
	for _, item := range items {
		if item.Kind != KindToolchain {
			return Plan{}, fmt.Errorf("rustup cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("rustup"); !ok {
		return Plan{}, errors.New("rustup is not installed here")
	}

	argv := []string{"rustup", "update"}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerRustup,
		Scope:   ScopeUser,
		Kind:    KindToolchain,
		Command: Command{Argv: argv, Env: []string{"LC_ALL=C"}, Timeout: applyTimeout},
		Items:   items,
		Restart: "A compiler or cargo process that is already running keeps the binary it started with; a new shell " +
			"uses the new toolchain.",
		Rollback: "rustup replaces the toolchain in place and keeps no previous copy of it, so this is not a " +
			"rollback; the version can be installed again under its own name if the channel still serves it.",
		Notes: []string{
			"rustup updates the named toolchains and their components; nothing here touches a project's files or its toolchain override.",
		},
	}, nil
}
