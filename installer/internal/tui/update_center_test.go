package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/updates"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UPDATE CENTER'S SCREEN
// ============================================================================
//
// Every guard here drives the screen the way a keypress does -- through Update,
// with the real rows -- and every manager behind it is a stand-in. No test in
// this file can start a package manager: the adapters answer from a queue and the
// runner records the commands it was asked to run instead of running them.
//
// That is the point of the seam. A guard that let the screen detect this runner's
// own managers would measure whatever the machine happens to have installed and,
// worse, would run `apt-get` or `brew` as a side effect of `go test`.

// tuiFakeAdapter is one manager that answers from a queue. Its availability, its
// results and its plan are all supplied by the test, so a guard can put the
// screen in a state no real machine would produce on demand: a locked apt, an
// unreachable mirror, a pinned formula.
type tuiFakeAdapter struct {
	id    updates.ManagerID
	label string
	scope updates.Scope
	kinds []updates.Kind
	avail updates.Availability
	// reads are the answers this manager gives, in order. The last one repeats
	// once the queue is spent, because a manager that has been read keeps
	// answering the same thing.
	reads []updates.Result
	// readAt is how many answers have been handed out.
	readAt int
	// planErr refuses to build a command, the way an adapter refuses one it is not
	// sure about.
	planErr error
}

func (a *tuiFakeAdapter) ID() updates.ManagerID { return a.id }
func (a *tuiFakeAdapter) Label() string         { return a.label }
func (a *tuiFakeAdapter) Scope() updates.Scope  { return a.scope }
func (a *tuiFakeAdapter) Kinds() []updates.Kind {
	if len(a.kinds) == 0 {
		return []updates.Kind{updates.KindPackage}
	}
	return a.kinds
}

func (a *tuiFakeAdapter) Probe(updates.Env) updates.Availability { return a.avail }

func (a *tuiFakeAdapter) Read(context.Context, updates.Runner, updates.Env) updates.Result {
	if a.readAt < len(a.reads) {
		result := a.reads[a.readAt]
		a.readAt++
		return result
	}
	if len(a.reads) > 0 {
		return a.reads[len(a.reads)-1]
	}
	return updates.Result{
		Manager: a.id,
		Label:   a.label,
		Scope:   a.scope,
		Status:  updates.StatusCurrent,
		Command: a.label + " list",
	}
}

func (a *tuiFakeAdapter) Plan(items []updates.Item, _ updates.Env) (updates.Plan, error) {
	if a.planErr != nil {
		return updates.Plan{}, a.planErr
	}
	if len(items) == 0 {
		return updates.Plan{}, fmt.Errorf("%s was asked to plan nothing", a.label)
	}
	argv := []string{a.label, "upgrade"}
	for _, item := range items {
		if item.Scope == updates.ScopeSystem {
			argv = append(argv, item.Name)
		}
	}
	return updates.Plan{
		Manager: a.id,
		Scope:   items[0].Scope,
		Kind:    items[0].Kind,
		// A system installation is moved by the administrator, exactly as the real
		// adapters demand: a guard that planned an unelevated system command would
		// measure a confirmation that never says who has to run it.
		Command:  updates.Command{Argv: argv, Elevate: items[0].Scope == updates.ScopeSystem},
		Items:    items,
		Restart:  "the service keeps the old version until it is restarted",
		Rollback: "the package manager keeps no previous version to go back to",
	}, nil
}

// tuiFakeRunner records the commands instead of running them, answers every
// command with success, and reports the lines the manager would have printed.
type tuiFakeRunner struct {
	commands []updates.Command
	output   updates.Output
	lines    []string
}

func (r *tuiFakeRunner) Run(_ context.Context, cmd updates.Command, onLine func(string)) updates.Output {
	r.commands = append(r.commands, cmd)
	for _, line := range r.lines {
		if onLine != nil {
			onLine(line)
		}
	}
	return r.output
}

// updateCenterTestModel is a model whose update center is wired to the fakes.
//
// The engine is installed before the screen is opened, so the screen never builds
// its own: a screen that called updates.Default would look this machine's
// managers up, which is both a measurement of the runner instead of the screen
// and a command away from running one.
func updateCenterTestModel(t *testing.T, adapters ...*tuiFakeAdapter) (Model, *tuiFakeRunner) {
	t.Helper()

	runner := &tuiFakeRunner{
		output: updates.Output{Stdout: "installed ok", ExitCode: 0, Started: true, Duration: time.Second},
		lines:  []string{"Preparing…", "Setting up tar (1.35-2)…", "Done."},
	}
	center := &updates.Center{
		Runner: runner,
		Env: updates.Env{
			GOOS:          "linux",
			LookPath:      func(string) (string, error) { return "/usr/bin/manager", nil },
			SudoAvailable: true,
			Now:           func() time.Time { return time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC) },
		},
	}
	for _, adapter := range adapters {
		center.Adapters = append(center.Adapters, adapter)
	}

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenMainMenu
	m.UpdateCenter.Engine = center
	return m, runner
}

// updateCenterFakeItem builds one entry the way a manager's own parser would.
func updateCenterFakeItem(adapter *tuiFakeAdapter, name, current, available string) updates.Item {
	kind := updates.KindPackage
	if kinds := adapter.Kinds(); len(kinds) > 0 {
		kind = kinds[0]
	}
	return updates.Item{
		Manager:   adapter.id,
		Name:      name,
		Scope:     adapter.scope,
		Kind:      kind,
		Current:   current,
		Available: available,
	}
}

// updateCenterAvailableAdapter is one manager that answers with the items given.
func updateCenterAvailableAdapter(id updates.ManagerID, scope updates.Scope, items ...updates.Item) *tuiFakeAdapter {
	adapter := &tuiFakeAdapter{
		id:    id,
		label: string(id),
		scope: scope,
		avail: updates.Availability{Available: true},
	}
	status := updates.StatusCurrent
	if len(items) > 0 {
		status = updates.StatusUpdates
	}
	adapter.reads = []updates.Result{{
		Manager: id,
		Label:   string(id),
		Scope:   scope,
		Status:  status,
		Command: string(id) + " outdated --json",
		Items:   items,
	}}
	return adapter
}

// updateCenterReadEveryManager finishes a reading pass: it delivers each answer
// and follows the next manager the screen asks for, until the inventory is
// exhausted. It is how a guard reaches the state a user reaches by waiting.
func updateCenterReadEveryManager(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()

	for cmd != nil {
		var next tea.Cmd
		m, next = runUpdateCenterReadCmd(t, m, cmd)
		cmd = next
	}
	if m.UpdateCenter.InFlight {
		t.Fatal("the reading pass never finished, so the screen would draw a list that is still being read")
	}
	return m
}

// pressUpdateCenterRow presses the key the row declares on itself: a row that
// opens a screen carries Enter, a switch row carries its state and is toggled
// with Space, a command row carries its own letter in brackets, and the way back
// is Esc. Nothing here knows which row is which: the row says how it is pressed,
// and a row that says nothing is a row this helper cannot press, which is the
// failure the grammar wants.
func pressUpdateCenterRow(t *testing.T, m Model, row string) (Model, tea.Cmd) {
	t.Helper()

	options := m.GetCurrentOptions()
	for index, option := range options {
		if option != row {
			continue
		}
		m.Cursor = index
		next, cmd := m.Update(updateCenterRowKeyMsg(t, row))
		return next.(Model), cmd
	}
	t.Fatalf("the screen offers no row %q, so no keypress could reach it: %v", row, options)
	return m, nil
}

// updateCenterRowKeyMsg reads the key a row declares and spells it as a terminal
// delivers it.
func updateCenterRowKeyMsg(t *testing.T, row string) tea.KeyMsg {
	t.Helper()

	switch {
	case strings.HasPrefix(row, "→ "):
		// A row that opens a screen.
		return tea.KeyMsg{Type: tea.KeyEnter}
	case isUpdateCenterItemRow(row):
		// A switch row: its state is in front of it and Space flips it.
		return updateCenterKeySpace()
	case row == "← Back":
		return updateCenterKeyEsc()
	}
	if len(row) >= 3 && row[0] == '[' && row[2] == ']' && row[3] == ' ' {
		return updateCenterKeyRune(rune(row[1]))
	}
	t.Fatalf("the row %q declares no key, so nothing says how it is pressed", row)
	return tea.KeyMsg{}
}

// pressUpdateCenterKey sends one key through the real dispatch.
func pressUpdateCenterKey(t *testing.T, m Model, key tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(key)
	return next.(Model), cmd
}

// updateCenterKeySpace and its neighbours are the three keys this screen reads
// besides Enter, spelled the way a terminal delivers them.
func updateCenterKeySpace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
}

func updateCenterKeyEsc() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEsc} }
func updateCenterKeyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// runUpdateCenterReadCmd delivers the message a read command would send, so the
// guard follows the same path the program does instead of calling the handler
// directly.
func runUpdateCenterReadCmd(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()

	if cmd == nil {
		t.Fatal("the screen issued no read, so a manager was never asked")
	}
	msg := cmd()
	next, fixed := m.Update(msg)
	return next.(Model), fixed
}

// updateCenterOpened opens the screen from the section's own row and runs the
// first manager's answer, so a guard starts from the state a user reaches in two
// keypresses.
func updateCenterOpened(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()

	m.Screen = ScreenUtilities
	m.Cursor = 0
	next, cmd := pressUpdateCenterRow(t, m, utilitiesScreenRow(utilitiesUpdatesRow))
	if next.Screen != ScreenUpdateCenter {
		t.Fatalf("the section's row opened %v, want ScreenUpdateCenter", next.Screen)
	}
	return next, cmd
}

// ---------------------------------------------------------------------------
// The door and the reading pass
// ---------------------------------------------------------------------------

// TestTheUtilitiesSectionOpensTheUpdateCenterAndAsksTheFirstManager pins the way
// in: the section carries the row, Enter opens the screen, and the read starts
// there -- one manager, chosen by the inventory's order, with the rest still
// queued behind it.
func TestTheUtilitiesSectionOpensTheUpdateCenterAndAsksTheFirstManager(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	m, runner := updateCenterTestModel(t, brew, apt)

	if next, _ := updateCenterOpened(t, m); next.UpdateCenter.Pending != updates.ManagerBrew {
		t.Fatalf("the pass started on %q, want the first available manager %q",
			next.UpdateCenter.Pending, updates.ManagerBrew)
	}

	// Nothing has been run: reading a manager is not updating anything.
	if len(runner.commands) != 0 {
		t.Fatalf("opening the screen ran %d command(s): %v", len(runner.commands), runner.commands)
	}

	next, cmd := updateCenterOpened(t, m)
	next, _ = runUpdateCenterReadCmd(t, next, cmd)

	if len(next.UpdateCenter.Results) != 1 {
		t.Fatalf("one manager was asked and %d answered: %+v", len(next.UpdateCenter.Results), next.UpdateCenter.Results)
	}
	if next.UpdateCenter.Results[0].Manager != updates.ManagerBrew {
		t.Errorf("the first answer came from %q, want brew", next.UpdateCenter.Results[0].Manager)
	}
}

// TestTheUpdateCenterAsksOneManagerAtATime pins the rule the locks demand: apt,
// pacman, dnf and pkg keep locks over the same package state, so the readers run
// in sequence. Each answer names exactly one next manager, and the pass ends when
// the inventory is exhausted.
func TestTheUpdateCenterAsksOneManagerAtATime(t *testing.T) {
	adapters := []*tuiFakeAdapter{
		updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser),
		updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem),
		updateCenterAvailableAdapter(updates.ManagerSnap, updates.ScopeSystem),
	}
	m, _ := updateCenterTestModel(t, adapters...)
	m, cmd := updateCenterOpened(t, m)

	want := []updates.ManagerID{updates.ManagerBrew, updates.ManagerApt, updates.ManagerSnap}
	for index, manager := range want {
		if m.UpdateCenter.Pending != manager {
			t.Fatalf("step %d asked %q, want %q", index+1, m.UpdateCenter.Pending, manager)
		}
		if !m.UpdateCenter.InFlight {
			t.Fatalf("step %d: the pass is not marked in flight, so the screen would draw a finished list", index+1)
		}
		m, cmd = runUpdateCenterReadCmd(t, m, cmd)
	}

	if m.UpdateCenter.InFlight || m.UpdateCenter.Pending != "" {
		t.Fatalf("the pass did not finish: inFlight=%v pending=%q", m.UpdateCenter.InFlight, m.UpdateCenter.Pending)
	}
	if cmd != nil {
		t.Error("the last answer asked for another manager, after the inventory was exhausted")
	}
	if len(m.UpdateCenter.Results) != len(want) {
		t.Fatalf("%d managers answered, want %d", len(m.UpdateCenter.Results), len(want))
	}
}

// TestTwoManagersThatCarryTheSameNameStayTwoEntries is the identity rule at the
// screen: an entry is its manager, its scope and its name. A formula called git
// and a package called git are two installations owned by two different programs,
// and selecting one must never select the other.
func TestTwoManagersThatCarryTheSameNameStayTwoEntries(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	brewItem := updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")
	aptItem := updateCenterFakeItem(apt, "git", "1:2.47.3-1", "1:2.47.3-2")
	brew.reads[0].Items = []updates.Item{brewItem}
	apt.reads[0].Items = []updates.Item{aptItem}

	m, _ := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	rows := m.GetCurrentOptions()
	var brewRow, aptRow string
	for _, row := range rows {
		if !strings.Contains(row, " git ") {
			continue
		}
		if strings.Contains(row, "brew") {
			brewRow = row
		}
		if strings.Contains(row, "apt") {
			aptRow = row
		}
	}
	if brewRow == "" || aptRow == "" {
		t.Fatalf("the two installs of git are not two rows: %v", rows)
	}
	if !strings.Contains(brewRow, "user") {
		t.Errorf("the brew row does not say whose installation it is: %q", brewRow)
	}
	if !strings.Contains(aptRow, "system") {
		t.Errorf("the apt row does not say whose installation it is: %q", aptRow)
	}

	next, _ := pressUpdateCenterRow(t, m, brewRow)
	if next.UpdateCenter.Selected.Count() != 1 {
		t.Fatalf("selecting the brew entry selected %d entries, want 1", next.UpdateCenter.Selected.Count())
	}
	if !next.UpdateCenter.Selected.Has(brewItem) {
		t.Error("the brew entry is not the one that got selected")
	}
	if next.UpdateCenter.Selected.Has(aptItem) {
		t.Error("selecting the brew entry selected apt's installation of the same name")
	}
	for _, row := range next.GetCurrentOptions() {
		if row == aptRow && !strings.HasPrefix(row, updateCenterRowUnselected) {
			t.Errorf("apt's row changed when brew's entry was selected: %q", row)
		}
	}
}

// TestAPinnedEntryIsShownAndRefusedInTheManagersOwnWords pins the honest half of
// a hold: the entry is on the screen -- hiding it would make the machine look
// like it had nothing to say -- and selecting it is refused with the hold the
// manager itself reported, rather than silently doing nothing.
func TestAPinnedEntryIsShownAndRefusedInTheManagersOwnWords(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	tar.Pinned, tar.PinReason = true, "apt-mark holds tar"
	apt.reads[0].Items = []updates.Item{tar}

	m, _ := updateCenterTestModel(t, apt)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	var held string
	for _, row := range m.GetCurrentOptions() {
		if strings.Contains(row, "tar") {
			held = row
		}
	}
	if !strings.HasPrefix(held, updateCenterRowHeld) {
		t.Fatalf("a held entry is not marked as held: %q", held)
	}

	next, _ := pressUpdateCenterRow(t, m, held)
	if next.UpdateCenter.Selected.Count() != 0 {
		t.Fatal("a held entry was selected; the plan would have to refuse it later")
	}
	if !strings.Contains(next.UpdateCenter.Notice, "apt-mark holds tar") {
		t.Errorf("the refusal does not say which hold it is: %q", next.UpdateCenter.Notice)
	}
}

// ---------------------------------------------------------------------------
// Nothing runs without a confirmed plan
// ---------------------------------------------------------------------------

// TestNoCommandRunsUntilThePlanIsConfirmed is the screen's central promise: a
// selection is not a run. Enter on the apply row opens the plan -- no command is
// issued -- and only the confirmation's own row starts it. Escape is the way back
// with the selection untouched.
func TestNoCommandRunsUntilThePlanIsConfirmed(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}

	m, runner := updateCenterTestModel(t, apt)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	var itemRow string
	for _, row := range m.GetCurrentOptions() {
		if strings.Contains(row, "tar") {
			itemRow = row
		}
	}
	m, _ = pressUpdateCenterRow(t, m, itemRow)

	// The apply row carries the selection's own count, and it is the only way into
	// the confirmation.
	applyRow := updateCenterApplyRow(1)
	next, cmd := pressUpdateCenterRow(t, m, applyRow)
	if cmd != nil {
		t.Fatal("opening the plan issued a command; a selection is not a run")
	}
	if !next.UpdateCenter.Confirming {
		t.Fatal("the apply row built no plan, so there is nothing to confirm")
	}
	if len(next.UpdateCenter.Plans) != 1 {
		t.Fatalf("the plan holds %d command(s), want 1", len(next.UpdateCenter.Plans))
	}
	if len(runner.commands) != 0 {
		t.Fatalf("opening the plan ran %d command(s)", len(runner.commands))
	}

	// The confirmation names the command exactly: a command that is only named
	// after it ran is not a plan.
	body := strings.Join(next.updateCenterDescription(), " ")
	if !strings.Contains(body, "apt upgrade") {
		t.Errorf("the confirmation does not name the command it would run: %q", body)
	}
	if !strings.Contains(body, "administrator permission") {
		t.Errorf("the confirmation does not say the command needs an administrator: %q", body)
	}
	if !strings.Contains(body, "going back") {
		t.Errorf("the confirmation does not say what going back would cost: %q", body)
	}

	// Escape cancels the plan and keeps the selection: confirming is a level in,
	// not a commitment.
	back, _ := pressUpdateCenterKey(t, next, updateCenterKeyEsc())
	if back.UpdateCenter.Confirming || back.Screen != ScreenUpdateCenter {
		t.Fatalf("Escape did not return to the list: confirming=%v screen=%v", back.UpdateCenter.Confirming, back.Screen)
	}
	if back.UpdateCenter.Selected.Count() != 1 {
		t.Error("Escape dropped the selection as well as the plan")
	}

	// The confirmation's own "no" row does the same thing.
	next, _ = pressUpdateCenterRow(t, back, applyRow)
	cancelled, _ := pressUpdateCenterRow(t, next, updateCenterCancelConfirmRow)
	if cancelled.UpdateCenter.Confirming {
		t.Error("the confirmation's cancel row left the screen confirming")
	}

	// And the plan's own row is what starts it.
	next, _ = pressUpdateCenterRow(t, cancelled, applyRow)
	applying, cmd := pressUpdateCenterRow(t, next, updateCenterConfirmRow)
	if cmd == nil {
		t.Fatal("confirming the plan issued no command")
	}
	if !applying.UpdateCenter.Applying {
		t.Fatal("confirming the plan did not put the screen in its running state")
	}
	if len(runner.commands) != 0 {
		t.Error("the command ran during the keypress instead of behind a command")
	}
}

// TestThePlanLeavesPinnedEntriesOutWithTheirReason pins what the confirmation
// says about a selection it cannot carry whole: the entries a hold keeps in place
// are named with the reason, rather than silently missing from a plan the user
// just asked for.
//
// The selection is built the only way a real one can hold a pinned entry: the
// entry was selected while it was still movable, and the manager reported the
// hold afterwards -- a re-read keeps it selected because the entry is still part
// of the answer, which is exactly why the plan has to say what it will not do.
func TestThePlanLeavesPinnedEntriesOutWithTheirReason(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	pinned := updateCenterFakeItem(apt, "libc6", "2.41-1", "2.41-2")
	pinned.Pinned, pinned.PinReason = true, "apt-mark holds libc6"
	apt.reads[0].Items = []updates.Item{tar, pinned}

	m, _ := updateCenterTestModel(t, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m.UpdateCenter.Selected.Toggle(pinned)
	for _, row := range m.GetCurrentOptions() {
		if strings.Contains(row, "tar") {
			m, _ = pressUpdateCenterRow(t, m, row)
		}
	}

	// An entry the row refused cannot be selected by a press, so what the plan
	// leaves out is what the selection already carried.
	m, _ = pressUpdateCenterRow(t, m, updateCenterApplyRow(2))
	if !m.UpdateCenter.Confirming {
		t.Fatalf("the plan was not built: %v", m.GetCurrentOptions())
	}

	body := strings.Join(m.updateCenterDescription(), " ")
	if !strings.Contains(body, "libc6") || !strings.Contains(body, "apt-mark holds libc6") {
		t.Errorf("the plan does not name the held entry and its hold: %q", body)
	}
	if len(m.UpdateCenter.Plans) != 1 {
		t.Errorf("the plan holds %d command(s), want the one the movable entry needs", len(m.UpdateCenter.Plans))
	}
}

// ---------------------------------------------------------------------------
// While it runs
// ---------------------------------------------------------------------------

// updateCenterRunningModel drives the screen into its running state with one
// confirmed plan, and returns the plan's entry so a guard can finish the run.
func updateCenterRunningModel(t *testing.T, adapter *tuiFakeAdapter, items ...updates.Item) (Model, *tuiFakeRunner) {
	t.Helper()

	adapter.reads[0].Items = items
	m, runner := updateCenterTestModel(t, adapter)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	for _, item := range items {
		for _, row := range m.GetCurrentOptions() {
			if strings.Contains(row, " "+item.Name+" ") && !strings.HasPrefix(row, updateCenterRowHeld) {
				m, _ = pressUpdateCenterRow(t, m, row)
			}
		}
	}
	m, _ = pressUpdateCenterRow(t, m, updateCenterApplyRow(len(items)))
	m, _ = pressUpdateCenterRow(t, m, updateCenterConfirmRow)
	return m, runner
}

// TestTheRunningUpdateRefusesToBeLeftAndStopsOnlyWhenAsked pins the two rules of
// the running state: Escape does not hide the only screen that can say what is
// happening, and stopping is a request the manager answers through its own
// interrupt rather than a silent disappearance of the frame.
func TestTheRunningUpdateRefusesToBeLeftAndStopsOnlyWhenAsked(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	m, _ := updateCenterRunningModel(t, apt, updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2"))

	if !m.UpdateCenter.Applying {
		t.Fatal("the confirmed plan is not running, so this guard measures nothing")
	}
	rows := m.GetCurrentOptions()
	if len(rows) != 1 || rows[0] != updateCenterStopRow {
		t.Fatalf("the running screen offers %v, want the stop row alone", rows)
	}

	escaped, cmd := pressUpdateCenterKey(t, m, updateCenterKeyEsc())
	if escaped.Screen != ScreenUpdateCenter {
		t.Fatal("Escape left the screen while its command was running")
	}
	if cmd != nil {
		t.Error("Escape issued a command")
	}
	if !strings.Contains(escaped.UpdateCenter.Notice, "running") {
		t.Errorf("Escape while running said nothing about what to do instead: %q", escaped.UpdateCenter.Notice)
	}

	stopped, _ := pressUpdateCenterRow(t, m, updateCenterStopRow)
	if stopped.UpdateCenter.ctx == nil || stopped.UpdateCenter.ctx.Err() == nil {
		t.Fatal("the stop row did not cancel the running command's context")
	}
	if !stopped.UpdateCenter.Applying {
		t.Error("the stop row closed the run before the manager answered for it")
	}
	if !strings.Contains(stopped.UpdateCenter.Notice, "Stopping") {
		t.Errorf("stopping said nothing about what is happening: %q", stopped.UpdateCenter.Notice)
	}
}

// TestTheResultSaysConfirmedOnlyWhenTheManagerSaysSoAndThenReadsItAgain pins the
// three different facts a result holds: requested, executed and confirmed. A
// finished command whose manager still reports the entry is not a move, and the
// screen reads the managers again before it reports anything as one.
func TestTheResultSaysConfirmedOnlyWhenTheManagerSaysSoAndThenReadsItAgain(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	item := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	m, _ := updateCenterRunningModel(t, apt, item)

	plan := m.UpdateCenter.Plans[0]
	after := apt.reads[0]
	after.Items = nil
	after.Status = updates.StatusCurrent
	apt.reads = append(apt.reads, after)

	done := updateCenterApplyDoneMsg{index: 0, result: updates.ApplyResult{
		Plan:     plan,
		Status:   updates.StatusCurrent,
		Command:  updates.QuoteArgv(plan.Command.Argv),
		After:    after,
		Outcomes: []updates.Outcome{{Item: item, Step: updates.StepConfirmed, Detail: "2.47.3-2"}},
	}}
	next, cmd := m.Update(done)
	m = next.(Model)

	if m.UpdateCenter.Applying {
		t.Error("the run is still marked as applying after its only command finished")
	}
	if !strings.Contains(m.UpdateCenter.Notice, "1 confirmed") {
		t.Errorf("the result does not report the confirmed entry: %q", m.UpdateCenter.Notice)
	}
	if cmd == nil {
		t.Fatal("the run finished without reading the managers again")
	}
	if !m.UpdateCenter.InFlight {
		t.Error("the re-read after the run is not marked in flight")
	}
	if m.UpdateCenter.Pending != updates.ManagerApt {
		t.Errorf("the re-read asks %q, want the manager that was just run", m.UpdateCenter.Pending)
	}
}

// TestAStoppedRunEndsTheSequenceAndReportsWhatWasNotReached pins the honest
// report of a stop: the plans that had not run are named as left out, the run
// ends instead of continuing with the rest, and the managers are still read again
// so the screen says what the machine really holds.
func TestAStoppedRunEndsTheSequenceAndReportsWhatWasNotReached(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	first := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	m, _ := updateCenterRunningModel(t, apt, first)

	// Two plans of one entry each: the shape that shows a stop ending the rest.
	m.UpdateCenter.Plans = append(m.UpdateCenter.Plans, updates.Plan{
		Manager: updates.ManagerApt,
		Scope:   updates.ScopeSystem,
		Command: updates.Command{Argv: []string{"apt", "upgrade", "libc6"}, Elevate: true},
		Items:   []updates.Item{updateCenterFakeItem(apt, "libc6", "2.41-1", "2.41-2")},
	})

	done := updateCenterApplyDoneMsg{index: 0, result: updates.ApplyResult{
		Plan:   m.UpdateCenter.Plans[0],
		Status: updates.StatusCancelled,
		Outcomes: []updates.Outcome{
			{Item: first, Step: updates.StepUnknown, Detail: "the run was stopped before the manager answered"},
		},
	}}
	next, cmd := m.Update(done)
	m = next.(Model)

	if m.UpdateCenter.Applying {
		t.Error("a stopped run is still marked as applying")
	}
	requested, claimedLeftOut := false, false
	for _, item := range m.UpdateCenter.NotReached {
		if item.Name == "libc6" {
			requested = true
		}
	}
	for _, out := range m.UpdateCenter.LeftOut {
		if out.Item.Name == "libc6" {
			claimedLeftOut = true
		}
	}
	if !requested {
		t.Errorf("the command that was never reached is not kept as requested and never run: %+v", m.UpdateCenter.NotReached)
	}
	if claimedLeftOut {
		t.Error("a command that was never started is reported as an entry left out, which claims no command could carry it")
	}
	if cmd == nil || !m.UpdateCenter.InFlight {
		t.Error("a stopped run did not read the managers again, so the screen cannot say what the machine holds")
	}
}

// TestTheLogKeepsTheEndOfWhatTheManagerPrinted pins the progress the running
// screen shows: the lines the manager printed arrive as they come, the end is
// what is kept, and the buffer stays bounded so a chatty manager cannot grow the
// model without limit.
func TestTheLogKeepsTheEndOfWhatTheManagerPrinted(t *testing.T) {
	m := Model{}
	m.UpdateCenter.Lines = make(chan string, 1)

	next, cmd := m.Update(updateCenterLineMsg("Preparing…"))
	m = next.(Model)
	if len(m.UpdateCenter.Log) != 1 || m.UpdateCenter.Log[0] != "Preparing…" {
		t.Fatalf("the line was not kept: %+v", m.UpdateCenter.Log)
	}
	if cmd == nil {
		t.Error("the read loop stopped after one line, so the rest of the output would be lost")
	}

	m.UpdateCenter.Lines = nil
	next, cmd = m.Update(updateCenterLineMsg("ignored"))
	m = next.(Model)
	if len(m.UpdateCenter.Log) != 1 {
		t.Errorf("a line arrived with no command running and was kept anyway: %+v", m.UpdateCenter.Log)
	}
	if cmd != nil {
		t.Error("a line arriving with no command running asked for another read")
	}

	for i := 0; i < updateCenterLogKept+50; i++ {
		m.updateCenterAppendLine(fmt.Sprintf("line %d", i))
	}
	if len(m.UpdateCenter.Log) != updateCenterLogKept {
		t.Errorf("the log holds %d lines, want it bounded at %d", len(m.UpdateCenter.Log), updateCenterLogKept)
	}
	if !strings.HasSuffix(m.UpdateCenter.Log[len(m.UpdateCenter.Log)-1], fmt.Sprintf("line %d", updateCenterLogKept+49)) {
		t.Error("the log kept the beginning of a chatty command instead of its end")
	}
}

// ---------------------------------------------------------------------------
// What the screen says when it cannot answer
// ---------------------------------------------------------------------------

// TestTheUpdateCenterSaysWhyAChannelCouldNotBeAsked pins the states that are not
// "up to date": a manager that is not here, a manager that could not be reached,
// and a manager with nothing to update are three different sentences. Saying
// "nothing to update" for a mirror that was unreachable is the lie this guard
// exists to prevent.
func TestTheUpdateCenterSaysWhyAChannelCouldNotBeAsked(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	brew.reads[0].Items = []updates.Item{updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")}

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	apt.reads = []updates.Result{{
		Manager: updates.ManagerApt,
		Label:   "apt",
		Scope:   updates.ScopeSystem,
		Status:  updates.StatusOffline,
		Reason:  "the mirror did not answer",
		Command: "apt-get --simulate upgrade",
	}}

	flatpak := &tuiFakeAdapter{
		id:    updates.ManagerFlatpak,
		label: "flatpak",
		scope: updates.ScopeUser,
		avail: updates.Availability{Available: false, Status: updates.StatusUnavailable, Reason: "flatpak is not on PATH"},
	}

	m, _ := updateCenterTestModel(t, brew, apt, flatpak)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	body := strings.Join(m.updateCenterDescription(), " ")
	if !strings.Contains(body, "the mirror did not answer") {
		t.Errorf("the unreachable manager's reason is not on the screen: %q", body)
	}
	if !strings.Contains(body, "could not be read") {
		t.Errorf("a manager that could not be read is not described as such: %q", body)
	}
	if !strings.Contains(body, "flatpak is not on PATH") {
		t.Errorf("the missing manager's reason is not on the screen: %q", body)
	}
	if !strings.Contains(body, "Read with") {
		t.Errorf("the screen does not name the command it read a manager with: %q", body)
	}
	if !strings.Contains(m.UpdateCenter.Notice, "could not be asked") {
		t.Errorf("the notice claims a clean answer while a channel was unreadable: %q", m.UpdateCenter.Notice)
	}
	if !strings.Contains(m.UpdateCenter.Notice, "1 update is available") {
		t.Errorf("the notice loses the entry that was found: %q", m.UpdateCenter.Notice)
	}

	// A manager that is not installed here is not an error: the screen keeps its
	// row and the pass does not stop on it.
	if len(m.UpdateCenter.Results) != 2 {
		t.Errorf("%d managers answered, want the two that could be asked", len(m.UpdateCenter.Results))
	}
	for _, row := range m.GetCurrentOptions() {
		if strings.Contains(row, "flatpak") {
			t.Errorf("an unavailable manager put an entry row on the screen: %q", row)
		}
	}
}

// TestAManagerThatAnswersWithNothingSaysNothingToUpdate pins the other half: a
// manager that was asked and has no entries says exactly that, and the notice
// does not treat an empty answer as a failure.
func TestAManagerThatAnswersWithNothingSaysNothingToUpdate(t *testing.T) {
	snap := updateCenterAvailableAdapter(updates.ManagerSnap, updates.ScopeSystem)
	m, _ := updateCenterTestModel(t, snap)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	body := strings.Join(m.updateCenterDescription(), " ")
	if !strings.Contains(body, "snap: nothing to update") {
		t.Errorf("a manager with nothing to update does not say so: %q", body)
	}
	if strings.Contains(body, "could not be read") {
		t.Errorf("an empty answer was described as a failure: %q", body)
	}
	if !strings.Contains(m.UpdateCenter.Notice, "up to date") {
		t.Errorf("the notice does not report the clean answer: %q", m.UpdateCenter.Notice)
	}
	options := m.GetCurrentOptions()
	if updateCenterIndexOfRow(options, updateCenterCheckRow) < 0 || updateCenterIndexOfRow(options, "← Back") < 0 {
		t.Errorf("the screen offers %v, want the check row and the way back", options)
	}
	for _, row := range options {
		if strings.HasPrefix(row, updateCenterRowUnselected) || strings.HasPrefix(row, updateCenterRowHeld) {
			t.Errorf("a manager with nothing to update put an entry row on the screen: %q", row)
		}
	}
}

// ---------------------------------------------------------------------------
// The rows themselves
// ---------------------------------------------------------------------------

// TestTheUpdateCenterNeverOffersToUpdateEverything pins the rule that has no
// exception: there is no row that applies every update the machine has. The only
// way into a command is a selection, and the apply row's count is the selection's
// own count.
func TestTheUpdateCenterNeverOffersToUpdateEverything(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	brew.reads[0].Items = []updates.Item{updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")}
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}

	m, _ := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	for _, row := range m.GetCurrentOptions() {
		lower := strings.ToLower(row)
		if strings.Contains(lower, "everything") || strings.Contains(lower, "update all") {
			t.Errorf("the screen offers a row that would update everything: %q", row)
		}
		if strings.HasPrefix(row, updateCenterApplyRowPrefix) {
			t.Errorf("an apply row is offered with nothing selected: %q", row)
		}
	}

	// An apply row only exists once something is selected: there is no row to press
	// at zero, which is the point -- the loop above already proves it is absent.
	options := m.GetCurrentOptions()
	var gitRow string
	for _, row := range options {
		if strings.Contains(row, "git") {
			gitRow = row
		}
	}
	m, _ = pressUpdateCenterRow(t, m, gitRow)

	options = m.GetCurrentOptions()
	apply := updateCenterApplyRow(1)
	if updateCenterIndexOfRow(options, apply) < 0 {
		t.Fatalf("selecting one entry offered no %q row: %v", apply, options)
	}
	for _, row := range options {
		if strings.HasPrefix(row, updateCenterApplyRowPrefix) && row != apply {
			t.Errorf("two apply rows are offered, so the count cannot be trusted: %v", options)
		}
	}

	m, _ = pressUpdateCenterRow(t, m, updateCenterClearRow)
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, updateCenterApplyRowPrefix) {
			t.Errorf("clearing the selection left an apply row behind: %q", row)
		}
	}
	if m.UpdateCenter.Selected.Count() != 0 {
		t.Errorf("clearing the selection left %d entries selected", m.UpdateCenter.Selected.Count())
	}
}

// TestTheFilterRowNarrowsTheListToOneManagerAndComesBack pins the one filter row:
// it appears only when there is more than one manager to filter, it cycles
// through the managers that have entries, and it returns to every manager rather
// than trapping the list on a filter that cannot be left.
func TestTheFilterRowNarrowsTheListToOneManagerAndComesBack(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	brew.reads[0].Items = []updates.Item{updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")}
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}

	m, _ := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	// The cycle is asserted on the state rather than on the row's label: a guard
	// that reads the label of the next value would pass on an off-by-one cycle,
	// which is the defect it exists to catch.
	step := func(m Model, current updates.ManagerID) Model {
		t.Helper()

		if m.UpdateCenter.Filter != current {
			t.Fatalf("the filter is %q, want %q before the row is pressed", m.UpdateCenter.Filter, current)
		}
		label := updateCenterFilterEvery
		if current != "" {
			label = string(current)
		}
		row := updateCenterFilterRowPrefix + label
		if updateCenterIndexOfRow(m.GetCurrentOptions(), row) < 0 {
			t.Fatalf("the filter row is not %q: %v", row, m.GetCurrentOptions())
		}
		next, _ := pressUpdateCenterRow(t, m, row)
		return next
	}

	m = step(m, "")
	if m.UpdateCenter.Filter != updates.ManagerBrew {
		t.Fatalf("the first press moved the filter to %q, want the inventory's first manager", m.UpdateCenter.Filter)
	}
	if !hasRowWithName(m.GetCurrentOptions(), "git") || hasRowWithName(m.GetCurrentOptions(), "tar") {
		t.Errorf("filtering to brew did not narrow the list to its entries: %v", m.GetCurrentOptions())
	}

	m = step(m, updates.ManagerBrew)
	if m.UpdateCenter.Filter != updates.ManagerApt {
		t.Fatalf("the second press moved the filter to %q, want the next manager", m.UpdateCenter.Filter)
	}
	if hasRowWithName(m.GetCurrentOptions(), "git") || !hasRowWithName(m.GetCurrentOptions(), "tar") {
		t.Errorf("filtering to apt did not narrow the list to its entries: %v", m.GetCurrentOptions())
	}

	m = step(m, updates.ManagerApt)
	if m.UpdateCenter.Filter != "" {
		t.Fatalf("cycling past the last manager left the filter on %q, want every manager", m.UpdateCenter.Filter)
	}
	if !hasRowWithName(m.GetCurrentOptions(), "git") || !hasRowWithName(m.GetCurrentOptions(), "tar") {
		t.Errorf("coming back to every manager did not show both entries: %v", m.GetCurrentOptions())
	}

	// One manager with entries needs no filter: a row that would always show the
	// same list is a row that does nothing.
	single := updateCenterAvailableAdapter(updates.ManagerSnap, updates.ScopeSystem)
	single.reads[0].Items = []updates.Item{updateCenterFakeItem(single, "core", "1", "2")}
	one, _ := updateCenterTestModel(t, single)
	one, cmd = updateCenterOpened(t, one)
	one, _ = runUpdateCenterReadCmd(t, one, cmd)
	for _, row := range one.GetCurrentOptions() {
		if strings.HasPrefix(row, updateCenterFilterRowPrefix) {
			t.Errorf("a single manager is offered a filter row: %q", row)
		}
	}
}

// TestNavigationSkipsTheRuleBetweenTwoManagersGroups pins the separator rule the
// section uses: the rules between groups are drawn, never landed on, so the
// cursor only ever stops on an entry or an action.
func TestNavigationSkipsTheRuleBetweenTwoManagersGroups(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	brew.reads[0].Items = []updates.Item{updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")}
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}

	m, _ := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	options := m.GetCurrentOptions()
	var gitRow string
	for _, row := range options {
		if strings.Contains(row, "git") {
			gitRow = row
		}
	}
	m.Cursor = updateCenterIndexOfRow(options, gitRow)

	next, _ := pressUpdateCenterKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if landed := next.GetCurrentOptions()[next.Cursor]; strings.HasPrefix(landed, menuSeparatorPrefix) {
		t.Fatalf("the cursor landed on the rule between the two managers: %q", landed)
	}
	if landed := next.GetCurrentOptions()[next.Cursor]; !strings.Contains(landed, "tar") {
		t.Errorf("the cursor landed on %q, want the next manager's entry", landed)
	}

	up, _ := pressUpdateCenterKey(t, next, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if landed := up.GetCurrentOptions()[up.Cursor]; strings.HasPrefix(landed, menuSeparatorPrefix) {
		t.Fatalf("the cursor landed on the rule going back up: %q", landed)
	}
	if landed := up.GetCurrentOptions()[up.Cursor]; !strings.Contains(landed, "git") {
		t.Errorf("going back up landed on %q, want the first manager's entry", landed)
	}
}

// TestTheUtilitiesPanelNamesTheUpdateStateHonestly pins the panel's line: before
// the question is asked it says so -- "not read yet" is a different fact from
// "nothing to update" -- and after a pass it reports what the managers answered.
func TestTheUtilitiesPanelNamesTheUpdateStateHonestly(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}
	m, _ := updateCenterTestModel(t, apt)

	entry := func(m Model) string {
		t.Helper()
		for _, candidate := range m.utilitiesPanelEntries() {
			if candidate.label == "Updates" {
				return candidate.value
			}
		}
		t.Fatal("the utilities panel does not name the update center at all")
		return ""
	}

	if got := entry(m); got != "not read yet" {
		t.Errorf("the panel says %q before anything has been read, want %q", got, "not read yet")
	}

	m, cmd := updateCenterOpened(t, m)
	if got := entry(m); !strings.Contains(got, "reading") {
		t.Errorf("the panel says %q while a pass is running", got)
	}
	m, _ = runUpdateCenterReadCmd(t, m, cmd)
	if got := entry(m); got != "1 update available" {
		t.Errorf("the panel says %q after a manager reported an update, want %q", got, "1 update available")
	}
}

// updateCenterIndexOfRow is where a row sits in the list the screen draws, or -1.
// It is named for this screen because the package has a second helper of the same
// shape for the network diagnosis, which takes a model rather than a list.
func updateCenterIndexOfRow(options []string, row string) int {
	for index, option := range options {
		if option == row {
			return index
		}
	}
	return -1
}

// hasRowWithName reports whether an entry row carries the name given.
func hasRowWithName(options []string, name string) bool {
	for _, option := range options {
		if strings.HasPrefix(option, updateCenterRowUnselected) ||
			strings.HasPrefix(option, updateCenterRowSelected) ||
			strings.HasPrefix(option, updateCenterRowHeld) {
			if strings.Contains(option, " "+name+" ") {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The key grammar
// ---------------------------------------------------------------------------
//
// The screen's rows are what they are: entries whose marker is a switch, actions
// that run on their own letter, and the way back. Nothing here is pressed with
// Enter, which belongs to the rows that open a screen or a detail -- this screen
// has neither.

// updateCenterGrammarModel is the screen as a user leaves it: two managers that
// answered with entries, one entry selected, so the list draws its switches, its
// category row, its plan and clear rows, its re-check and its filter.
func updateCenterGrammarModel(t *testing.T) (Model, *tuiFakeRunner) {
	t.Helper()

	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	brew.reads[0].Items = []updates.Item{
		updateCenterFakeItem(brew, "git", "2.50.0", "2.51.0"),
		updateCenterFakeItem(brew, "jq", "1.7.1", "1.8.0"),
	}
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	apt.reads[0].Items = []updates.Item{updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")}

	m, runner := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	return updateCenterReadEveryManager(t, m, cmd), runner
}

// updateCenterRowKey is the key a drawn row says it answers to: Space on a row
// whose marker is a switch, the bracketed letter on a command row, and Esc on the
// way back. A row that says nothing is a failure, because then nothing on screen
// tells the reader how to press it.
func updateCenterRowKey(t *testing.T, row string) string {
	t.Helper()

	switch {
	case isUpdateCenterItemRow(row):
		return "Space"
	case row == "← Back":
		return "Esc"
	}
	if len(row) >= 4 && row[0] == '[' && row[2] == ']' && row[3] == ' ' {
		return string(row[1])
	}
	t.Fatalf("the row %q carries no key, so nothing says how it is pressed", row)
	return ""
}

// assertUpdateCenterRowsDeclareTheirKeys is the grammar's guard on one drawn
// state: every row names its key, no letter runs two rows, no letter is a move
// key, and the footer names exactly the keys the rows offer.
func assertUpdateCenterRowsDeclareTheirKeys(t *testing.T, m Model) {
	t.Helper()

	drawn := map[string]string{}
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, menuSeparatorPrefix) {
			continue
		}
		key := updateCenterRowKey(t, row)
		// Space is the switch's, and there is more than one switch on the list:
		// it acts on the row under the cursor, so it is the one key several rows
		// may declare. Every other key is a command, and a command has one row.
		if key != "Space" {
			if other, ok := drawn[key]; ok {
				t.Errorf("%q runs two rows on this screen: %q and %q", key, other, row)
			}
		}
		drawn[key] = row
		if key == "j" || key == "k" {
			t.Errorf("the row %q runs on %q, which is a move key", row, key)
		}
	}

	// The arrows and the letters beside them are the cursor's, not a command's,
	// and Esc is the way back in every state of the screen -- including the
	// confirmation, whose two answer rows are drawn without it.
	moves := map[string]bool{"↑": true, "↓": true, "k": true, "j": true, "Esc": true}
	offered := map[string]bool{}
	for _, hint := range m.updateCenterHints() {
		for _, key := range hintKeyTokens(hint.keys) {
			offered[key] = true
		}
	}
	for key, row := range drawn {
		if !offered[key] {
			t.Errorf("the row %q declares %q and the footer does not name it", row, key)
		}
	}
	for key := range offered {
		if _, ok := drawn[key]; !ok && !moves[key] {
			t.Errorf("the footer names %q and no row on the screen offers it", key)
		}
	}

	// The footer spends two rows at most and drops what does not fit without a
	// word, so every key it names has to be on the screen: a hint the packing
	// dropped is a key the footer no longer advertises.
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	for _, hint := range m.updateCenterHints() {
		if hint.keys != "" && !strings.Contains(plain, hint.keys) {
			t.Errorf("the footer draws no %q, so the hint was packed away: %v", hint.keys, m.updateCenterHints())
		}
	}
}

// updateCenterCursorTo puts the cursor on a row the screen draws.
func updateCenterCursorTo(t *testing.T, m Model, row string) Model {
	t.Helper()

	for index, option := range m.GetCurrentOptions() {
		if option == row {
			m.Cursor = index
			return m
		}
	}
	t.Fatalf("the screen offers no row %q, so nothing could be pressed on it: %v", row, m.GetCurrentOptions())
	return m
}

// updateCenterRowContaining is the one drawn row that carries the name given.
func updateCenterRowContaining(t *testing.T, m Model, name string) string {
	t.Helper()

	for _, option := range m.GetCurrentOptions() {
		if isUpdateCenterItemRow(option) && strings.Contains(option, " "+name+" ") {
			return option
		}
	}
	t.Fatalf("no entry row carries %q: %v", name, m.GetCurrentOptions())
	return ""
}

// TestTheUpdateCentreSaysHowEveryRowIsPressed pins the screen's half of the
// grammar: an entry row shows its state and is a switch, every action row carries
// its own letter, the letters are unique and none of them is a move key, and the
// footer names those same keys and no others. It is checked in the three states
// the rows differ in -- settled, with a selection, and the confirmation.
func TestTheUpdateCentreSaysHowEveryRowIsPressed(t *testing.T) {
	m, _ := updateCenterGrammarModel(t)

	// Settled with nothing selected: the switches, the category row, the re-check
	// and the filter.
	if m.UpdateCenter.Selected.Count() != 0 {
		t.Fatalf("the fixture starts with %d entries selected", m.UpdateCenter.Selected.Count())
	}
	assertUpdateCenterRowsDeclareTheirKeys(t, m)

	// With an entry taken: the plan row and the clear row join them.
	m, _ = pressUpdateCenterRow(t, m, updateCenterRowContaining(t, m, "git"))
	if m.UpdateCenter.Selected.Count() != 1 {
		t.Fatalf("%d entries are selected after flipping one switch, want 1", m.UpdateCenter.Selected.Count())
	}
	assertUpdateCenterRowsDeclareTheirKeys(t, m)

	// The confirmation: two answers and the way back, each on its own key.
	m, _ = pressUpdateCenterRow(t, m, updateCenterApplyRow(1))
	if !m.UpdateCenter.Confirming {
		t.Fatal("the plan row opened no confirmation")
	}
	assertUpdateCenterRowsDeclareTheirKeys(t, m)

	// Space is on the entries and nowhere else, and Enter is on nothing at all.
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, menuSeparatorPrefix) {
			continue
		}
		key := updateCenterRowKey(t, row)
		if isUpdateCenterItemRow(row) && key != "Space" {
			t.Errorf("the entry row %q is pressed with %q, and an entry is a switch", row, key)
		}
		if !isUpdateCenterItemRow(row) && key == "Space" {
			t.Errorf("the row %q claims Space, which belongs to a switch", row)
		}
		if key == "Enter" {
			t.Errorf("the row %q runs on Enter, which opens or chooses", row)
		}
	}
}

// TestTheUpdateCentreRunsEachCommandOnItsLetter presses what each row says and
// checks that row, and only that row, happened: the plan opens, the cancel goes
// back, the plan runs, the running update stops, the selection clears, the shown
// category is taken, the filter cycles, the managers are asked again, and Enter
// runs none of it.
func TestTheUpdateCentreRunsEachCommandOnItsLetter(t *testing.T) {
	m, runner := updateCenterGrammarModel(t)

	// Enter is not a command on this screen. On the plan row it opens nothing, on
	// an entry row it flips nothing, and it issues no command.
	planRow := updateCenterApplyRow(1)
	itemRow := updateCenterRowContaining(t, m, "git")
	m, _ = pressUpdateCenterRow(t, m, itemRow)
	onPlan := updateCenterCursorTo(t, m, planRow)
	next, cmd := pressUpdateCenterKey(t, onPlan, tea.KeyMsg{Type: tea.KeyEnter})
	if next.UpdateCenter.Confirming {
		t.Fatal("Enter opened the plan; applying is not opening or choosing")
	}
	if cmd != nil {
		t.Fatal("Enter issued a command")
	}
	onItem := updateCenterCursorTo(t, m, updateCenterRowContaining(t, m, "git"))
	if next, _ = pressUpdateCenterKey(t, onItem, tea.KeyMsg{Type: tea.KeyEnter}); next.UpdateCenter.Selected.Count() != 1 {
		t.Fatalf("Enter changed the selection to %d entries; an entry is a switch", next.UpdateCenter.Selected.Count())
	}

	// Space belongs to the switch: on a command row it is not a second Enter.
	if next, cmd = pressUpdateCenterKey(t, onPlan, updateCenterKeySpace()); next.UpdateCenter.Confirming || cmd != nil {
		t.Fatal("Space opened the plan; Space only toggles a switch")
	}
	if len(runner.commands) != 0 {
		t.Fatalf("pressing keys that do nothing ran %d command(s)", len(runner.commands))
	}

	// The plan row's letter opens the plan, and no command runs on the way in.
	m, cmd = pressUpdateCenterKey(t, m, updateCenterKeyRune('a'))
	if !m.UpdateCenter.Confirming {
		t.Fatal("the plan row's letter did not open the confirmation")
	}
	if cmd != nil || len(runner.commands) != 0 {
		t.Fatal("opening the plan issued a command; a selection is not a run")
	}

	// The confirmation's "no" goes back with the selection kept.
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('n'))
	if m.UpdateCenter.Confirming {
		t.Fatal("the cancel's letter left the screen confirming")
	}
	if m.UpdateCenter.Selected.Count() != 1 {
		t.Fatal("the cancel dropped the selection as well as the plan")
	}

	// The confirmation's "yes" starts the run.
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('a'))
	m, cmd = pressUpdateCenterKey(t, m, updateCenterKeyRune('y'))
	if !m.UpdateCenter.Applying {
		t.Fatal("the confirmation's letter did not start the run")
	}
	if cmd == nil {
		t.Fatal("the run started no command")
	}

	// While it runs, its own row is the stop, on its own letter.
	if next, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('i')); next.UpdateCenter.Notice == "" {
		t.Fatal("the stop row's letter did not ask the manager to stop")
	}

	// A settled list again: the clear row empties the selection.
	m, _ = updateCenterGrammarModel(t)
	m, _ = pressUpdateCenterRow(t, m, updateCenterRowContaining(t, m, "git"))
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('c'))
	if m.UpdateCenter.Selected.Count() != 0 {
		t.Fatalf("%d entries are still selected after the clear row's letter", m.UpdateCenter.Selected.Count())
	}

	// The category row takes everything the filter shows, in one press.
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('s'))
	if m.UpdateCenter.Selected.Count() != 3 {
		t.Fatalf("the category row selected %d of the 3 shown entries", m.UpdateCenter.Selected.Count())
	}

	// The filter row cycles to the first manager that answered.
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyRune('f'))
	if m.UpdateCenter.Filter != updates.ManagerBrew {
		t.Fatalf("the filter row set the filter to %q, want %q", m.UpdateCenter.Filter, updates.ManagerBrew)
	}

	// The re-check row asks the managers again, and runs nothing.
	m, cmd = pressUpdateCenterKey(t, m, updateCenterKeyRune('r'))
	if cmd == nil || !m.UpdateCenter.InFlight {
		t.Fatal("the re-check row's letter did not start a reading pass")
	}
	if len(runner.commands) != 0 {
		t.Fatalf("asking the managers again ran %d update command(s)", len(runner.commands))
	}
}
