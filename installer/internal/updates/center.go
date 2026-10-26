package updates

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Channel is one manager's presence on this host: what the interface names, what
// it owns, and whether it can be asked. A channel that cannot be asked is present
// here with its reason: the update center declares what it cannot see rather than
// listing a shorter inventory and leaving the gap to be guessed at.
type Channel struct {
	Manager ManagerID
	Label   string
	Scope   Scope
	Kinds   []Kind
	// Available reports whether the manager can be asked here.
	Available bool
	// Status is StatusUnavailable or StatusUnsupported when it cannot.
	Status Status
	// Reason is why not, in the interface's words.
	Reason string
	// Elevation reports whether moving what this manager owns needs an
	// administrator. It is what the plan's confirmation names before anything is
	// run.
	Elevation bool
}

// Center is the whole update path: the adapters of the managers this machine
// really has, one read at a time, and one plan applied at a time.
//
// One at a time is not a performance choice. apt, pacman, dnf and pkg keep locks
// over the same package state, and two of them running together is how a machine
// ends up with a half-configured database; the center therefore never starts a
// second command while one is running, and the interface drives it one step at a
// time.
type Center struct {
	// Adapters are the managers, in the order the interface presents them.
	Adapters []Adapter
	// Runner runs the commands. It is a seam: every test substitutes one.
	Runner Runner
	// Env is the machine the adapters ask about.
	Env Env
}

// Default returns the center the interface uses: the real adapters, the real
// runner, and the environment of the machine the program is running on.
//
// termux is the caller's own detection -- the installer already knows whether it
// is on Termux -- and it is passed in rather than re-detected here, so the center
// and the rest of the installer cannot disagree about which host this is.
func Default(termux bool) *Center {
	return &Center{
		Adapters: DefaultAdapters(),
		Runner:   ExecRunner{},
		Env:      DefaultEnv(termux),
	}
}

// DefaultAdapters returns every adapter this package ships, in the order the
// interface presents them: the managers the installer itself provisions first,
// then the machine's distribution channel, then the rest. The order is stable so
// two runs of the same machine produce the same list.
func DefaultAdapters() []Adapter {
	return []Adapter{
		BrewAdapter{},
		AptAdapter{},
		PacmanAdapter{},
		DnfAdapter{},
		PkgAdapter{},
		SnapAdapter{},
		FlatpakAdapter{},
		RustupAdapter{},
	}
}

// DefaultEnv builds the real environment.
func DefaultEnv(termux bool) Env {
	env := Env{
		GOOS:          runtime.GOOS,
		Termux:        termux,
		LookPath:      exec.LookPath,
		Getenv:        os.Getenv,
		SudoAvailable: sudoAvailable(),
		Now:           time.Now,
		BrewPrefix:    system.GetBrewPrefix(),
	}
	return env
}

// sudoAvailable reports whether an elevation program is on PATH at all. It is
// only ever used to say so: nothing here starts one.
func sudoAvailable() bool {
	_, err := exec.LookPath(sudoBinary)
	return err == nil
}

// now is the center's clock, so a zero Env built in a test does not panic.
func (c *Center) now() time.Time {
	if c.Env.Now != nil {
		return c.Env.Now()
	}
	return time.Now()
}

// RunnerOrDefault returns the runner, or the real one when the center carries
// none, so a zero Center is still usable.
func (c *Center) runnerOrDefault() Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return ExecRunner{}
}

// Channels returns every adapter with its availability here. The availability of
// each one is asked once, which is a PATH lookup and a stat: no command is run,
// so this is safe on the interface's own path.
func (c *Center) Channels() []Channel {
	channels := make([]Channel, 0, len(c.Adapters))
	for _, adapter := range c.Adapters {
		availability := adapter.Probe(c.Env)
		channels = append(channels, Channel{
			Manager:   adapter.ID(),
			Label:     adapter.Label(),
			Scope:     adapter.Scope(),
			Kinds:     adapter.Kinds(),
			Available: availability.Available,
			Status:    availability.Status,
			Reason:    availability.Reason,
			Elevation: adapter.Scope() == ScopeSystem,
		})
	}
	return channels
}

// Adapter returns the registered adapter for a manager, or false when no adapter
// owns it.
func (c *Center) Adapter(manager ManagerID) (Adapter, bool) {
	for _, adapter := range c.Adapters {
		if adapter.ID() == manager {
			return adapter, true
		}
	}
	return nil, false
}

// Read asks one manager what it has that is newer. It never starts more than the
// one manager's own commands, and a manager that cannot be asked here answers
// with StatusUnavailable rather than an error.
func (c *Center) Read(ctx context.Context, manager ManagerID) Result {
	adapter, ok := c.Adapter(manager)
	if !ok {
		return Result{
			Manager: manager,
			Status:  StatusUnavailable,
			Reason:  "no update channel is registered for " + string(manager),
		}
	}
	if availability := adapter.Probe(c.Env); !availability.Available {
		return Result{
			Manager: manager,
			Label:   adapter.Label(),
			Scope:   adapter.Scope(),
			Status:  availability.Status,
			Reason:  availability.Reason,
		}
	}
	return adapter.Read(ctx, c.runnerOrDefault(), c.Env)
}

// LeftOut is one selected entry no command will move, with the reason. It is how
// a pinned entry is reported: shown, selected, and left alone with its hold named
// rather than silently skipped or silently moved.
type LeftOut struct {
	Item   Item
	Reason string
}

// PlansFor turns a selection into the commands the owning managers would run: one
// plan per manager, scope and kind, in the adapters' own order. A pinned entry is
// never planned; it is returned in leftOut with the hold that keeps it where it
// is.
//
// The plans are built from the selection and nothing else. There is no
// "everything" here: a center that planned every entry on the machine would be
// the "update all" button the interface must not have.
func (c *Center) PlansFor(items []Item) (plans []Plan, leftOut []LeftOut) {
	var movable []Item
	for _, item := range items {
		if item.Pinned {
			leftOut = append(leftOut, LeftOut{Item: item, Reason: item.PinReason})
			continue
		}
		movable = append(movable, item)
	}

	for _, group := range GroupByKey(movable) {
		adapter, ok := c.Adapter(group.Key.Manager)
		if !ok {
			for _, item := range group.Items {
				leftOut = append(leftOut, LeftOut{Item: item, Reason: "no update channel is registered for " + string(group.Key.Manager)})
			}
			continue
		}
		plan, err := adapter.Plan(group.Items, c.Env)
		if err != nil {
			for _, item := range group.Items {
				leftOut = append(leftOut, LeftOut{Item: item, Reason: err.Error()})
			}
			continue
		}
		plans = append(plans, plan)
	}
	return plans, leftOut
}

// Apply runs one plan and then re-reads its manager, so what is reported per
// entry is what the manager says after the fact rather than what the command's
// exit status suggested.
//
// The re-read happens on every path, including the failed and the cancelled one:
// a command that refused after moving one package really did move it, and a
// command that was stopped halfway really may have moved some. An entry the
// manager no longer reports as out of date is confirmed; one it still reports is
// failed when the command failed, executed when it succeeded, and unknown when
// the run was stopped before the manager could say.
func (c *Center) Apply(ctx context.Context, plan Plan, onLine func(string)) ApplyResult {
	runner := c.runnerOrDefault()
	// The redaction is applied here as well as in the real runner, so the promise
	// belongs to the center rather than to whichever runner a caller supplied: a
	// line handed to a screen has already had its credentials removed, whatever
	// ran the command.
	var sink func(string)
	if onLine != nil {
		sink = func(line string) { onLine(RedactLine(line)) }
	}
	commands := plan.Sequence()
	var last Output
	var lines []string
	for _, cmd := range commands {
		last = runner.Run(ctx, cmd, sink)
		lines = append(lines, last.Text())
		// A refresh that failed stops the sequence: the install would only fail
		// again on the same stale metadata, and the refresh's own reason is the
		// one worth reporting.
		if st, _ := Classify(last); st == StatusFailed || st == StatusCancelled {
			break
		}
	}
	output := last
	status, reason := Classify(output)

	result := ApplyResult{
		Plan:     plan,
		Status:   status,
		Reason:   reason,
		Output:   strings.Join(lines, "\n"),
		Command:  plan.Display(),
		Duration: output.Duration.String(),
	}

	// The re-read is not part of the cancelled or timed-out transaction: it asks
	// the manager what the state is now, which is exactly what a stopped run
	// leaves in doubt. It keeps the values of the original context but not its
	// cancellation.
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), readTimeout)
	defer cancel()
	after := c.Read(readCtx, plan.Manager)
	result.After = after

	known := after.Status == StatusUpdates || after.Status == StatusCurrent || after.Status == StatusUnavailable
	for _, item := range plan.Items {
		result.Outcomes = append(result.Outcomes, outcome(item, after, known, status, reason))
	}
	return result
}

// outcome is one entry's result, read from the manager's fresh answer.
func outcome(item Item, after Result, known bool, status Status, reason string) Outcome {
	stillOutdated := itemStillOutdated(item, after)

	switch {
	case stillOutdated && status == StatusCancelled:
		return Outcome{Item: item, Step: StepUnknown, Detail: "the run was stopped before the manager could say whether this moved"}
	case stillOutdated && status != StatusCurrent:
		return Outcome{Item: item, Step: StepFailed, Detail: reason}
	case stillOutdated:
		return Outcome{Item: item, Step: StepExecuted, Detail: "the manager accepted the command and still reports this as out of date"}
	case !known:
		// The manager could not be asked again, so nothing here can stand behind
		// a claim that the entry moved: what is known is that the command ran.
		if status == StatusCurrent {
			return Outcome{Item: item, Step: StepExecuted, Detail: "the command finished, and the manager could not be read again to confirm it: " + after.Reason}
		}
		if status == StatusCancelled {
			return Outcome{Item: item, Step: StepUnknown, Detail: "the run was stopped and the manager could not be read again to say what it had done"}
		}
		return Outcome{Item: item, Step: StepFailed, Detail: reason}
	case after.Status == StatusUnavailable:
		// A manager that is not here any more cannot have moved anything.
		if status == StatusCurrent {
			return Outcome{Item: item, Step: StepExecuted, Detail: "the command finished, and the manager is no longer available to confirm it"}
		}
		return Outcome{Item: item, Step: StepFailed, Detail: reason}
	default:
		return Outcome{Item: item, Step: StepConfirmed, Detail: "the manager no longer reports this as out of date"}
	}
}

// itemStillOutdated reports whether the manager's fresh answer still lists an
// entry. Identity is the whole key -- manager, scope and name -- so an entry that
// shares its name with something else the same manager owns is not mistaken for
// it, and an entry the manager reports under another scope is not either.
func itemStillOutdated(item Item, after Result) bool {
	for _, listed := range after.Items {
		if listed.Key() == item.Key() {
			return true
		}
	}
	return false
}

// determined, so a caller can leave a row out rather than printing a path.
