package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UPDATE APPLY'S ADMINISTRATOR CREDENTIAL
// ============================================================================
//
// The screen runs a manager's system-wide upgrade through the update engine, and
// the engine spells its root commands `sudo -n` -- the flag that refuses instead
// of prompting, chosen because an interface cannot answer a prompt it never sees.
// On a machine whose sudo would ask for a password, that refusal reached the user
// as "sudo password is required" with nothing on screen able to fix it.
//
// The apply now makes the run's own credential request before its first root
// command, and these guards hold the three promises that make it usable: the ask
// comes before the FIRST root command, a machine that already holds a credential
// is never prompted, and the request is made at most once per run.
//
// Every seam here is a variable the apply reads (`sudoCredentialCached`,
// `sudoTerminalExec`), so no guard can invoke sudo or start a package manager.

// updateCredentialRecorder records, in one ordered list, everything the apply did
// about the administrator credential and every command it ran. The order is the
// point: the credential has to be cached before the first root command.
type updateCredentialRecorder struct {
	events []string
}

func (r *updateCredentialRecorder) record(event string) {
	r.events = append(r.events, event)
}

// count returns how many times the named event was recorded.
func (r *updateCredentialRecorder) count(event string) int {
	total := 0
	for _, recorded := range r.events {
		if recorded == event {
			total++
		}
	}
	return total
}

// indexOf is where an event was recorded, and -1 when it was not.
func (r *updateCredentialRecorder) indexOf(event string) int {
	for at, recorded := range r.events {
		if recorded == event {
			return at
		}
	}
	return -1
}

// withUpdateCredentialMocks replaces the credential seams the update apply reaches
// and returns the recorder they append to.
//
// probeErr is what `sudo -n -v` answers: nil means sudo already holds a credential
// it can use, which is the case the apply must leave alone. askErr is what the
// terminal ask answers, so a refused password can be exercised without a terminal.
func withUpdateCredentialMocks(t *testing.T, probeErr, askErr error) *updateCredentialRecorder {
	t.Helper()

	originalProbe := sudoCredentialCached
	originalExec := sudoTerminalExec

	recorder := &updateCredentialRecorder{}

	sudoCredentialCached = func() error {
		recorder.record("probe")
		return probeErr
	}
	sudoTerminalExec = func(respond func(error) tea.Msg) tea.Cmd {
		recorder.record("ask")
		return func() tea.Msg { return respond(askErr) }
	}

	t.Cleanup(func() {
		sudoCredentialCached = originalProbe
		sudoTerminalExec = originalExec
	})

	return recorder
}

// updateCredentialRunner records the commands the sequence runs, so the guard can
// see the ask happen before the first one.
type updateCredentialRunner struct {
	inner  updates.Runner
	record func(string)
}

func (r updateCredentialRunner) Run(ctx context.Context, cmd updates.Command, onLine func(string)) updates.Output {
	r.record("run:" + cmd.Argv[0])
	return r.inner.Run(ctx, cmd, onLine)
}

// updateCenterConfirmedModel drives the screen to the confirmation and presses the
// key that starts the run, returning the model and the command that press issued.
func updateCenterConfirmedModel(t *testing.T, m Model, items []updates.Item) (Model, tea.Cmd) {
	t.Helper()

	for _, item := range items {
		for _, row := range m.GetCurrentOptions() {
			if strings.Contains(row, " "+item.Name+" ") && !strings.HasPrefix(row, updateCenterRowHeld) {
				m, _ = pressUpdateCenterRow(t, m, row)
			}
		}
	}
	m, _ = pressUpdateCenterRow(t, m, updateCenterApplyRow(len(items)))
	if !m.UpdateCenter.Confirming {
		t.Fatalf("the confirmation did not open, so no keypress could start the run: %v", m.GetCurrentOptions())
	}
	return pressUpdateCenterRow(t, m, updateCenterConfirmRow)
}

// updateCenterRunToEnd delivers the credential answers and every command of the
// confirmed sequence, in the order the program receives them, and stops when the
// run reports its end.
func updateCenterRunToEnd(t *testing.T, m Model, pending ...tea.Cmd) Model {
	t.Helper()

	for steps := 0; len(pending) > 0; steps++ {
		if steps > 256 {
			t.Fatalf("the run did not finish: %v", m.UpdateCenter.Notice)
		}
		cmd := pending[0]
		pending = pending[1:]
		if cmd == nil {
			continue
		}
		message := cmd()
		if batch, ok := message.(tea.BatchMsg); ok {
			pending = append(pending, batch...)
			continue
		}
		next, follow := m.Update(message)
		m = next.(Model)
		if follow != nil {
			pending = append(pending, follow)
		}
	}
	return m
}

// TestTheUpdateApplyAsksForTheCredentialBeforeItsFirstRootCommand is the reported
// failure, pinned. A system-wide upgrade runs `sudo -n`, so a machine whose sudo
// would ask for a password needs the credential cached first -- and the ask has to
// precede the command that needs it, not follow its refusal.
func TestTheUpdateApplyAsksForTheCredentialBeforeItsFirstRootCommand(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	if probe == nil {
		t.Fatal("the confirmed run dispatched nothing, so the credential could never be asked for")
	}
	if !m.UpdateCenter.Applying {
		t.Fatal("the confirmed run is not running, so this guard measures nothing")
	}
	if got := recorder.count("run:apt"); got != 0 {
		t.Fatalf("a root command ran before the credential was asked for: %v", recorder.events)
	}

	m = updateCenterRunToEnd(t, m, probe)

	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("ask"); got != 1 {
		t.Fatalf("the credential was asked for %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("the plan's command never ran after the credential was cached; events = %v", recorder.events)
	}
	if askAt, firstRunAt := recorder.indexOf("ask"), recorder.indexOf("run:apt"); askAt > firstRunAt {
		t.Fatalf("the credential was asked for after the first root command ran: %v", recorder.events)
	}
	if !m.SudoPrimed {
		t.Error("the ask cached a credential but the model does not record the run as primed")
	}
	if !m.SudoPrimeAttempted {
		t.Error("the request is not recorded as made, so a later screen would ask again")
	}
}

// TestTheCachedCredentialIsNeverAskedForBeforeTheUpdateRuns pins the other side of
// the same decision: a machine where sudo already holds a credential it can use --
// a container running as root, a NOPASSWD rule -- is not prompted at all, so the
// sequence stays as silent as it was.
func TestTheCachedCredentialIsNeverAskedForBeforeTheUpdateRuns(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, nil, errors.New("the ask should never have happened"))

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	m = updateCenterRunToEnd(t, m, probe)

	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("ask"); got != 0 {
		t.Fatalf("a machine with a usable credential was asked for one: %v", recorder.events)
	}
	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("the plan's command never ran; events = %v", recorder.events)
	}
	if !m.SudoPrimed {
		t.Error("a usable credential was found but the model does not record the run as primed")
	}
}

// TestTheUpdateApplyAsksForTheCredentialOnlyOnce pins the at-most-once promise
// across a sequence of several root commands: the second plan runs on the
// credential the first one's ask cached, so the user is prompted for one password
// and not one per command.
func TestTheUpdateApplyAsksForTheCredentialOnlyOnce(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	libc := updateCenterFakeItem(apt, "libc6", "2.41-1", "2.41-2")
	apt.reads[0].Items = []updates.Item{tar, libc}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar, libc})
	if len(m.UpdateCenter.Plans) != 1 {
		t.Fatalf("the run holds %d plan(s), want the one command both entries share", len(m.UpdateCenter.Plans))
	}
	m = updateCenterRunToEnd(t, m, probe)

	if got := recorder.count("ask"); got != 1 {
		t.Fatalf("the run asked for the credential %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
}

// TestAUserScopedUpdatePlanAsksForNothing keeps the request tied to the command:
// Homebrew is user-scoped, so a sequence that runs no root command asks for no
// credential at all.
func TestAUserScopedUpdatePlanAsksForNothing(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	git := updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")
	brew.reads[0].Items = []updates.Item{git}

	m, runner := updateCenterTestModel(t, brew)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, start := updateCenterConfirmedModel(t, m, []updates.Item{git})
	m = updateCenterRunToEnd(t, m, start)

	if got := recorder.count("probe"); got != 0 {
		t.Fatalf("a user-scoped plan probed sudo: %v", recorder.events)
	}
	if got := recorder.count("ask"); got != 0 {
		t.Fatalf("a user-scoped plan asked for the administrator credential: %v", recorder.events)
	}
	if got := recorder.count("run:brew"); got == 0 {
		t.Fatalf("the plan's command never ran; events = %v", recorder.events)
	}
}

// TestAnUpdateApplyAfterTheRunsRequestAsksNothing pins the other half of "once per
// run": a session that already answered the question -- a run that cached the
// credential, or one that asked and was refused -- is not asked again, because a
// repeated refused ask is the loop the run exists to avoid.
func TestAnUpdateApplyAfterTheRunsRequestAsksNothing(t *testing.T) {
	cases := []struct {
		name  string
		prime func(*Model)
	}{
		{name: "the credential is already cached", prime: func(m *Model) { m.SudoPrimed, m.SudoPrimeAttempted = true, true }},
		{name: "the run already asked and was refused", prime: func(m *Model) { m.SudoPrimed, m.SudoPrimeAttempted = false, true }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"), nil)

			apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
			tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
			apt.reads[0].Items = []updates.Item{tar}

			m, runner := updateCenterTestModel(t, apt)
			m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
			m, cmd := updateCenterOpened(t, m)
			m = updateCenterReadEveryManager(t, m, cmd)

			// The request belongs to the run, so it is made before the
			// confirmation is pressed: this guard measures a screen that follows it.
			tc.prime(&m)
			m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
			m = updateCenterRunToEnd(t, m, probe)

			if got := recorder.count("probe"); got != 0 {
				t.Fatalf("the run probed sudo again after its one request: %v", recorder.events)
			}
			if got := recorder.count("ask"); got != 0 {
				t.Fatalf("the run asked for the credential a second time: %v", recorder.events)
			}
		})
	}
}

// TestARefusedCredentialDoesNotSkipTheUpdateRun pins what happens when the person
// declines the prompt: the sequence still runs, and the command that needed the
// credential reports the refusal itself through `sudo -n`, in the manager's own
// words. A skipped run would leave the screen with nothing to say about why.
func TestARefusedCredentialDoesNotSkipTheUpdateRun(t *testing.T) {
	recorder := withUpdateCredentialMocks(t,
		errors.New("sudo: a password is required"),
		errors.New("the terminal ask was cancelled"))

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	m = updateCenterRunToEnd(t, m, probe)

	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("a refused credential stopped the sequence from running: %v", recorder.events)
	}
	if m.SudoPrimed {
		t.Error("a refused credential is recorded as a cached one")
	}
	if !m.SudoPrimeAttempted {
		t.Error("a refused credential is not recorded as asked, so it would be asked again")
	}
}

// TestTheUpdateApplyAndTheInstallingScreenAskThroughTheSameHandover pins that the
// ask itself is one seam with two answers rather than two commands: the program's
// `sudo -v` handover is spelled once, the installing screen resumes its step loop
// on its own answer, and the update centre resumes its command sequence on its own.
func TestTheUpdateApplyAndTheInstallingScreenAskThroughTheSameHandover(t *testing.T) {
	originalExec := sudoTerminalExec
	defer func() { sudoTerminalExec = originalExec }()

	var asked int
	var answers []tea.Msg
	sudoTerminalExec = func(respond func(error) tea.Msg) tea.Cmd {
		asked++
		answers = append(answers, respond(nil))
		return func() tea.Msg { return respond(nil) }
	}

	if cmd := defaultSudoTerminalPrimer(); cmd == nil {
		t.Fatal("the installing screen's ask issued no command")
	}
	if cmd := sudoTerminalExec(func(err error) tea.Msg { return updateCenterCredentialMsg{err: err} }); cmd == nil {
		t.Fatal("the update centre's ask issued no command")
	}

	if asked != 2 {
		t.Fatalf("the two screens made %d asks through the handover, want 2", asked)
	}
	if _, ok := answers[0].(sudoPrimedMsg); !ok {
		t.Errorf("the installing screen's answer is %T, want sudoPrimedMsg: it must resume the step loop", answers[0])
	}
	if _, ok := answers[1].(updateCenterCredentialMsg); !ok {
		t.Errorf("the update centre's answer is %T, want updateCenterCredentialMsg: it must resume its own sequence", answers[1])
	}
}

// TestTheUpdateApplyAsksWithTheRunsOwnFlags keeps the request itself unchanged: the
// probe is `sudo -n -v`, which never prompts, and the ask is `sudo -v` with no `-n`
// and no `-S`, so the password is typed where sudo reads it and never travels
// through the program. It is the same pair the installing screen uses.
func TestTheUpdateApplyAsksWithTheRunsOwnFlags(t *testing.T) {
	argsFile, _ := fakeSudo(t)

	if err := sudoCredentialCached(); err != nil {
		t.Fatalf("the probe against a sudo that succeeds reported %v, want no error", err)
	}
	probeBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the probe ran no sudo: %v", err)
	}
	probeArgs := string(probeBytes)
	for _, want := range []string{"-n", "-v"} {
		if !strings.Contains(probeArgs, want) {
			t.Fatalf("the probe did not give sudo %q: %q", want, probeArgs)
		}
	}
	if strings.Contains(probeArgs, "-S") {
		t.Fatalf("the probe asked sudo to read a password from standard input: %q", probeArgs)
	}

	if err := defaultSudoTerminalAsk(); err != nil {
		t.Fatalf("the terminal ask against a sudo that succeeds reported %v, want no error", err)
	}
	askBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the terminal ask ran no sudo: %v", err)
	}
	askArgs := strings.Split(strings.TrimSpace(string(askBytes)), "\n")
	if len(askArgs) != 1 || askArgs[0] != "-v" {
		t.Fatalf("the terminal ask gave sudo %v, want exactly [-v]", askArgs)
	}
}
