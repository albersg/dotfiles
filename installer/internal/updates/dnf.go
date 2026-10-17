package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DnfAdapter reads Fedora's package manager. Its scope is the machine's.
//
// `dnf check-update` is the read. It exits 100 when it has something to report
// and 0 when it does not, which is dnf's own convention rather than a failure, so
// the exit code is read here and not handed to the shared classifier.
type DnfAdapter struct{}

func (DnfAdapter) ID() ManagerID { return ManagerDnf }
func (DnfAdapter) Label() string { return "DNF (Fedora)" }
func (DnfAdapter) Scope() Scope  { return ScopeSystem }
func (DnfAdapter) Kinds() []Kind { return []Kind{KindPackage} }

// Probe answers where dnf is.
func (DnfAdapter) Probe(env Env) Availability {
	if env.GOOS != "linux" && env.GOOS != "" {
		return unsupported("dnf serves Fedora hosts, not " + env.GOOS)
	}
	if env.Termux {
		return unsupported("this host is Termux, whose channel is pkg")
	}
	if _, ok := env.Resolve("dnf"); !ok {
		return missing("dnf is not installed here")
	}
	return present()
}

// ParseDnfCheckUpdate reads `dnf check-update`. A row is
// `name.arch  version  repository`, the repository is optional in some releases,
// and the "Obsoleting Packages" section that follows is a different shape
// altogether: its rows name both the obsoleted and the obsoleting package, and
// taking one of those as an update would offer the wrong name.
//
// Only rows whose first field looks like `name.arch` are read, and a row of the
// obsoleted section is skipped, so nothing here is guessed from position alone.
func ParseDnfCheckUpdate(output string) []Item {
	var items []Item
	inObsoleting := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lowered := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lowered, "obsoleting packages"),
			strings.HasPrefix(lowered, "obsoleting package"):
			inObsoleting = true
			continue
		case strings.HasSuffix(trimmed, ":") || strings.HasPrefix(lowered, "security:"),
			strings.HasPrefix(lowered, "dependencies resolved"),
			strings.HasPrefix(lowered, "last metadata"):
			inObsoleting = false
			continue
		}
		if inObsoleting {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		name, arch, ok := strings.Cut(fields[0], ".")
		if !ok || name == "" || arch == "" {
			continue
		}
		// A version is what dnf prints in the second column; a line whose second
		// field is another name.arch pair belongs to a section this parser does
		// not read.
		if strings.ContainsAny(fields[1], " \t") || fields[1] == "" {
			continue
		}
		items = append(items, Item{
			Manager:   ManagerDnf,
			Name:      name,
			Scope:     ScopeSystem,
			Kind:      KindPackage,
			Available: fields[1],
		})
	}
	return items
}

// ParseRpmQuery reads `rpm -q --qf '%{NAME} %{VERSION}-%{RELEASE}\n'` for the
// packages it was asked about: one row per installed package. A package rpm does
// not know prints `package <name> is not installed`, which is a refusal rather
// than a version and is skipped, so a name that is absent simply has no installed
// version here.
func ParseRpmQuery(output string) map[string]string {
	versions := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasSuffix(trimmed, " is not installed") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || fields[0] == "package" {
			continue
		}
		versions[fields[0]] = fields[1]
	}
	return versions
}

// Read asks dnf what is out of date and reads the installed versions of what it
// named.
func (a DnfAdapter) Read(ctx context.Context, runner Runner, env Env) Result {
	if _, ok := env.Resolve("dnf"); !ok {
		return result(a, env, StatusUnavailable, "dnf is not installed here", command("dnf", "check-update"), nil)
	}

	cmd := command("dnf", "-q", "check-update")
	output := cmd.run(ctx, runner, env)
	// dnf's 100 is its "there is something to report", not a failure; anything
	// else that is not zero is classified by the shared rules.
	if status, _ := Classify(output); status != StatusCurrent && output.ExitCode != 100 {
		return failedResult(a, env, cmd, output)
	}

	items := ParseDnfCheckUpdate(output.Stdout)
	if versions := dnfInstalledVersions(ctx, runner, env, items); len(versions) > 0 {
		for i := range items {
			items[i].Current = versions[items[i].Name]
		}
	}

	ignored, ignoreNote := dnfVersionlock(ctx, runner, env)
	for i := range items {
		if ignored[items[i].Name] {
			items[i].Pinned = true
			items[i].PinReason = "locked with dnf's versionlock plugin; moving it needs the lock removed with `sudo dnf versionlock delete " + items[i].Name + "`"
		}
	}

	res := result(a, env, StatusCurrent, "", cmd, sortedItems(items))
	if len(items) > 0 {
		res.Status = StatusUpdates
	}
	if ignoreNote != "" {
		res.Notes = append(res.Notes, ignoreNote)
	}
	if len(items) > 0 && output.ExitCode == 100 {
		res.Notes = append(res.Notes, "dnf reports updates with its own exit status 100; that is its answer, not a failure.")
	}
	res.Notes = append(res.Notes, "dnf's answer is as fresh as the metadata it has cached; this check never refreshes that metadata on its own beyond what dnf does to answer.")
	return res
}

// dnfInstalledVersions reads the installed version of every package dnf named, in
// one rpm query. rpm is what actually owns the installed database, so asking it is
// asking the manager's own source rather than downloading a listing dnf would have
// to compose.
func dnfInstalledVersions(ctx context.Context, runner Runner, env Env, items []Item) map[string]string {
	if len(items) == 0 {
		return nil
	}
	if _, ok := env.Resolve("rpm"); !ok {
		return nil
	}
	argv := append([]string{"rpm", "-q", "--qf", "%{NAME} %{VERSION}-%{RELEASE}\n"}, itemNames(items)...)
	output := runner.Run(ctx, Command{Argv: argv, Env: []string{"LC_ALL=C"}, Timeout: readTimeout}, nil)
	versions := ParseRpmQuery(output.Stdout)
	if len(versions) == 0 {
		return nil
	}
	return versions
}

// dnfVersionlock reads the versionlock plugin's list. The plugin is optional, so a
// machine without it has no locks this package can read rather than an error.
func dnfVersionlock(ctx context.Context, runner Runner, env Env) (map[string]bool, string) {
	if _, ok := env.Resolve("dnf"); !ok {
		return nil, ""
	}
	cmd := command("dnf", "-q", "versionlock", "list")
	output := cmd.run(ctx, runner, env)
	if status, _ := Classify(output); status != StatusCurrent {
		return nil, ""
	}
	if strings.Contains(strings.ToLower(output.Stdout), "no such command") ||
		strings.Contains(strings.ToLower(output.Stdout), "unknown command") {
		return nil, ""
	}
	locked := map[string]bool{}
	for _, line := range strings.Split(output.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "versionlock") {
			continue
		}
		// A lock line is `epoch:name-version-release.arch` or `name-version...`.
		spec := trimmed
		if colon := strings.Index(spec, ":"); colon >= 0 && !strings.Contains(spec[:colon], "-") {
			spec = spec[colon+1:]
		}
		if name, _, ok := strings.Cut(spec, "-"); ok && name != "" {
			locked[name] = true
		}
	}
	if len(locked) == 0 {
		return nil, ""
	}
	return locked, ""
}

// itemNames returns the names of a slice of items.
func itemNames(items []Item) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// Plan builds the one command that moves a dnf selection.
func (a DnfAdapter) Plan(items []Item, env Env) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, errors.New("no dnf packages were selected")
	}
	for _, item := range items {
		if item.Pinned {
			return Plan{}, fmt.Errorf("%s is %s", item.Name, item.PinReason)
		}
		if item.Kind != KindPackage {
			return Plan{}, fmt.Errorf("dnf cannot update an entry of kind %q", item.Kind)
		}
	}
	if _, ok := env.Resolve("dnf"); !ok {
		return Plan{}, errors.New("dnf is not installed here")
	}

	argv := []string{"dnf", "-y", "upgrade"}
	for _, item := range items {
		argv = append(argv, item.Name)
	}
	return Plan{
		Manager: ManagerDnf,
		Scope:   ScopeSystem,
		Kind:    KindPackage,
		Command: Command{
			Argv:    argv,
			Elevate: true,
			Env:     []string{"LC_ALL=C"},
			Timeout: applyTimeout,
		},
		Items: items,
		Restart: "A kernel, a C library or a service package keeps the running version until the machine or the " +
			"service is restarted; this run restarts nothing and does not reboot.",
		Rollback: "dnf keeps the transaction in its own history, so `sudo dnf history undo <id>` can put the " +
			"previous packages back while they are still in its cache; this run keeps no copy of its own.",
		Notes: []string{
			"dnf resolves dependencies itself and may install or move packages the selection did not name.",
			"dnf takes its own lock while this runs; another dnf or PackageKit transaction holding it makes this refuse, and nothing here runs two managers at once.",
		},
	}, nil
}
