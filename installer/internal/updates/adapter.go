package updates

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"time"
)

// Env is the machine the adapters are asked about, taken as a value so that every
// question an adapter asks about the host -- is this program here, is this Termux,
// what does the environment say -- is one a test can answer. No adapter reads the
// process environment or the PATH directly.
type Env struct {
	// GOOS is runtime.GOOS.
	GOOS string
	// Termux reports that this host is Termux on Android, where the package
	// channel is pkg's and the distribution managers are not the owner.
	Termux bool
	// LookPath resolves a program in the PATH.
	LookPath func(string) (string, error)
	// Getenv reads one environment variable.
	Getenv func(string) string
	// SudoAvailable reports whether an elevation program exists here. It is what
	// lets the interface say "this needs administrator permission" before the user
	// confirms rather than after the command is refused.
	SudoAvailable bool
	// Now is the clock every result is stamped with.
	Now func() time.Time
	// BrewPrefix is Homebrew's prefix, used to find brew when it is installed but
	// not on this process's PATH -- which is exactly the state a run is in right
	// after the installer installed it.
	BrewPrefix string
}

// Availability is an adapter's answer about its own manager on this host.
type Availability struct {
	// Available reports whether the manager can be asked here.
	Available bool
	// Status is StatusUnavailable when the program is not installed, and
	// StatusUnsupported when it is installed but does not serve this host: a
	// manager for another platform is not a missing program, and telling the two
	// apart is what stops the screen from suggesting an install that would not
	// help.
	Status Status
	// Reason is why not, in the interface's words.
	Reason string
}

// missing is the answer for a manager whose program is not installed here.
func missing(reason string) Availability {
	return Availability{Status: StatusUnavailable, Reason: reason}
}

// unsupported is the answer for a manager that is installed but does not serve
// this host. Available is false for the same reason it is false for a missing
// program: the channel cannot be asked here. Status is what tells the two apart,
// and the interface says which of them it is.
func unsupported(reason string) Availability {
	return Availability{Available: false, Status: StatusUnsupported, Reason: reason}
}

// present is the answer for a manager that can be asked.
func present() Availability { return Availability{Available: true, Status: StatusUpdates} }

// Adapter is one manager's whole knowledge: whether it is here, how to read what
// it says is out of date, and how to build the command that would move a
// selection. Everything a manager knows lives in its adapter, which is why a new
// manager is one new file and one entry in the registry rather than a branch
// inside a shared function.
type Adapter interface {
	// ID is the manager's identity. It is part of every item's key.
	ID() ManagerID
	// Label is how the interface names the manager in a row.
	Label() string
	// Scope is who owns what this manager installs. A manager whose scope depends
	// on the installation -- Flatpak's does -- reports the scope it can prove and
	// carries the rest per item; this one is the default a result is reported
	// under.
	Scope() Scope
	// Kinds are the entry kinds this adapter can produce, in the order it
	// produces them.
	Kinds() []Kind
	// Probe reports whether the manager can be used on this host.
	Probe(env Env) Availability
	// Read returns what the manager reports as out of date, or why it could not
	// be asked. It never returns an error: a manager that refused is a result,
	// and the reason is the answer.
	Read(ctx context.Context, runner Runner, env Env) Result
	// Plan builds the one command that would move items, which all share the
	// adapter's manager, scope and kind. A manager that cannot build a command for
	// them returns the reason rather than a command it is not sure about.
	Plan(items []Item, env Env) (Plan, error)
}

// readCommand is one command an adapter runs to read its manager: the program and
// arguments, and the environment entries the answer needs to be parseable. The
// adapters build these through command() so no adapter has to remember LC_ALL.
type readCommand struct {
	argv []string
	env  []string
}

// command builds a read command whose output is parseable: LC_ALL=C keeps the
// manager's words in one language, and NO_COLOR keeps progress escapes out of the
// lines this package parses.
func command(program string, args ...string) readCommand {
	return readCommand{
		argv: append([]string{program}, args...),
		env:  []string{"LC_ALL=C", "LANG=C", "NO_COLOR=1"},
	}
}

// withEnv adds environment entries to a read command.
func (c readCommand) withEnv(entries ...string) readCommand {
	c.env = append(append([]string{}, c.env...), entries...)
	return c
}

// run executes the read command through the runner and returns its output.
func (c readCommand) run(ctx context.Context, runner Runner, env Env) Output {
	return runner.Run(ctx, Command{Argv: c.argv, Env: c.env, Timeout: readTimeout}, nil)
}

// Display is the command as a reader would type it.
func (c readCommand) Display() string { return QuoteArgv(c.argv) }

const (
	// readTimeout bounds one manager's read. Querying a package index over a slow
	// connection is slower than a listing of local state, and both must end: an
	// update center that can be made to wait forever by an unreachable mirror is
	// worse than one that says it could not ask.
	readTimeout = 90 * time.Second

	// applyTimeout bounds one manager's update. A real install over a slow
	// connection can take minutes, and it is not bounded by the read's budget: the
	// command is stopped here only so that a manager waiting on an unreachable
	// mirror cannot hold a chosen update open forever.
	applyTimeout = 30 * time.Minute
)

// result builds a Result with the fields every adapter's read shares.
func result(adapter Adapter, env Env, status Status, reason string, command readCommand, items []Item) Result {
	return Result{
		Manager: adapter.ID(),
		Label:   adapter.Label(),
		Scope:   adapter.Scope(),
		Status:  status,
		Reason:  reason,
		Items:   items,
		Command: command.Display(),
	}
}

// failedResult classifies a command's output and builds the result from it.
func failedResult(adapter Adapter, env Env, cmd readCommand, output Output) Result {
	status, reason := Classify(output)
	return result(adapter, env, status, reason, cmd, nil)
}

// sortedItems orders items by name so a manager's rows do not move between two
// reads of the same machine.
func sortedItems(items []Item) []Item {
	sort.SliceStable(items, func(a, b int) bool { return items[a].Name < items[b].Name })
	return items
}

// itemNote is the one place an item's Note is appended to, so two adapters cannot
// phrase the same caveat differently.
func itemNote(existing, note string) string {
	if existing == "" {
		return note
	}
	if note == "" {
		return existing
	}
	return existing + " " + note
}

// fakeGetenv is the environment reader a zero Env falls back to, so an Env built
// in a test without a Getenv does not panic.
func fakeGetenv(key string) string { return "" }

// LookPath resolves a program, trying env's own resolver first and the known
// Homebrew prefix second. It is the one place an adapter resolves a program.
func (e Env) Resolve(program string) (string, bool) {
	lookup := e.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	if path, err := lookup(program); err == nil && path != "" {
		return path, true
	}
	getenv := e.Getenv
	if getenv == nil {
		getenv = fakeGetenv
	}
	if program == "brew" {
		prefix := e.BrewPrefix
		if prefix == "" {
			prefix = getenv("HOMEBREW_PREFIX")
		}
		if prefix != "" {
			candidate := prefix + "/bin/brew"
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}
	return "", false
}

// Get reads one environment variable through env's own reader.
func (e Env) Get(key string) string {
	if e.Getenv == nil {
		return ""
	}
	return e.Getenv(key)
}
