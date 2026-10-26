package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
// command, and the password is typed on the installer's own screen: the terminal
// is never handed over, and a refusal or a cancelled attempt releases the screen
// so the command reports itself in the manager's own words.
//
// Every seam here is a variable the apply reads (`sudoCredentialCached`) or a
// `sudo` found through PATH, so no guard can invoke the real sudo or start a
// package manager.

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

// withUpdateCredentialMocks replaces the credential probe the update apply reaches
// and returns the recorder it appends to.
//
// probeErr is what `sudo -n -v` answers: nil means sudo already holds a credential
// it can use, which is the case the apply must leave alone. There is no second
// seam, because the second half is the installer's own screen: a guard that has to
// answer it puts a fake `sudo` on PATH and types into that screen, which is what
// the program itself does.
func withUpdateCredentialMocks(t *testing.T, probeErr error) *updateCredentialRecorder {
	t.Helper()

	originalProbe := sudoCredentialCached

	recorder := &updateCredentialRecorder{}

	sudoCredentialCached = func() error {
		recorder.record("probe")
		return probeErr
	}

	t.Cleanup(func() {
		sudoCredentialCached = originalProbe
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

// answerCredentialScreen types password on the installer's own screen, presses
// Enter and delivers the answer, which is exactly what the program does with it.
// It returns the model and whatever the answer resumed.
func answerCredentialScreen(t *testing.T, m Model, password string) (Model, tea.Cmd) {
	t.Helper()

	if m.Screen != ScreenAdminPassword {
		t.Fatalf("the run is on screen %v, want the installer's own password screen", m.Screen)
	}
	if m.CredentialAsk != credentialAskUpdateCenter {
		t.Fatalf("the screen recorded flow %v, want the update centre's: the answer would resume the wrong flow", m.CredentialAsk)
	}

	for _, r := range password {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("Enter started no attempt, so the password could never be checked")
	}

	answer := messageWithin(t, cmd, 10*time.Second)
	next, follow := m.Update(answer)
	return next.(Model), follow
}

// TestTheUpdateApplyAsksOnTheInstallerScreenBeforeItsFirstRootCommand is the
// reported failure, pinned. A system-wide upgrade runs `sudo -n`, so a machine
// whose sudo would ask for a password needs the credential cached first -- and the
// asking happens on the installer's own screen, before the command that needs it,
// not in the shell the installer was started from.
func TestTheUpdateApplyAsksOnTheInstallerScreenBeforeItsFirstRootCommand(t *testing.T) {
	fakeSudoScript(t, "cat > /dev/null\nexit 0")
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

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

	// The probe's answer puts the installer's own screen up, and nothing has run.
	m = updateCenterRunToEnd(t, m, probe)
	if m.Screen != ScreenAdminPassword {
		t.Fatalf("the run is on screen %v, want the installer's own password screen: nothing may be asked in the shell", m.Screen)
	}
	if got := recorder.count("run:apt"); got != 0 {
		t.Fatalf("a root command ran before the credential was asked for: %v", recorder.events)
	}

	m, follow := answerCredentialScreen(t, m, "the password")
	m = updateCenterRunToEnd(t, m, follow)

	if m.Screen != ScreenUpdateCenter {
		t.Fatalf("the run ended on screen %v, want the update centre it came from", m.Screen)
	}
	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("the plan's command never ran after the credential was cached; events = %v", recorder.events)
	}
	if !m.SudoPrimed {
		t.Error("the attempt cached a credential but the model does not record the run as primed")
	}
	if !m.SudoPrimeAttempted {
		t.Error("the request is not recorded as made, so a later screen would ask again")
	}
	if len(m.AdminPassword) != 0 {
		t.Errorf("the model kept %d bytes of the password after the attempt", len(m.AdminPassword))
	}
}

// TestTheCachedCredentialIsNeverAskedForBeforeTheUpdateRuns pins the other side of
// the same decision: a machine where sudo already holds a credential it can use --
// a container running as root, a NOPASSWD rule -- is not prompted at all, so the
// sequence stays as silent as it was.
func TestTheCachedCredentialIsNeverAskedForBeforeTheUpdateRuns(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, nil)

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	m = updateCenterRunToEnd(t, m, probe)

	if m.Screen == ScreenAdminPassword {
		t.Fatal("a machine whose sudo already holds a credential was asked for a password")
	}
	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("the plan's command never ran; events = %v", got)
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
	fakeSudoScript(t, "cat > /dev/null\nexit 0")
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

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
	m, follow := answerCredentialScreen(t, m, "the password")
	m = updateCenterRunToEnd(t, m, follow)

	if got := recorder.count("probe"); got != 1 {
		t.Fatalf("the run probed sudo %d times, want exactly 1; events = %v", got, recorder.events)
	}
	if m.Screen == ScreenAdminPassword {
		t.Fatal("the run was left on the password screen, so it asked more than once")
	}
}

// TestAUserScopedUpdatePlanAsksForNothing keeps the request tied to the command:
// Homebrew is user-scoped, so a sequence that runs no root command asks for no
// credential at all.
func TestAUserScopedUpdatePlanAsksForNothing(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

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
	if m.Screen == ScreenAdminPassword {
		t.Fatal("a user-scoped plan asked for the administrator credential")
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
			recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

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
			if m.Screen == ScreenAdminPassword {
				t.Fatal("the run asked for the credential a second time")
			}
		})
	}
}

// TestARefusedPasswordReleasesTheUpdateRunWithTheManagersReason pins what happens
// when sudo refuses the password typed on the screen. The sequence still runs --
// waiting or asking again would be the loop this run exists to avoid -- and the
// command that needed the credential reports the refusal itself through `sudo -n`,
// in the manager's own words, which is the report the user can act on.
func TestARefusedPasswordReleasesTheUpdateRunWithTheManagersReason(t *testing.T) {
	fakeSudoScript(t, "cat > /dev/null\necho 'sudo: 3 incorrect password attempts' >&2\nexit 1")
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	// The manager's own refusal, which is what `sudo -n` prints on a machine whose
	// credential was never cached.
	runner.output = updates.Output{
		Stderr:   "sudo: a password is required\n",
		ExitCode: 1,
		Err:      errors.New("exit status 1"),
		Started:  true,
	}
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	m = updateCenterRunToEnd(t, m, probe)

	m, follow := answerCredentialScreen(t, m, "the wrong password")

	if m.Screen == ScreenAdminPassword {
		t.Fatal("a refused password left the run waiting on the password screen")
	}
	if m.SudoPrimed {
		t.Error("a refused password is recorded as a cached credential")
	}
	joined := strings.Join(m.LogLines, "\n")
	if !strings.Contains(joined, "not accepted") {
		t.Fatalf("the run's record does not carry the reason the attempt failed:\n%s", joined)
	}
	if strings.Contains(joined, "incorrect password attempts") {
		t.Fatalf("the run's record repeats sudo's stderr:\n%s", joined)
	}
	if len(m.AdminPassword) != 0 {
		t.Errorf("the model kept %d bytes of the password after a refused attempt", len(m.AdminPassword))
	}

	// The sequence is resumed by the release, so its commands run next. Waiting, or
	// asking again, is the loop this run exists to avoid.
	m = updateCenterRunToEnd(t, m, follow)

	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("a refused credential stopped the sequence from running: %v", recorder.events)
	}

	if len(m.UpdateCenter.Applied) == 0 {
		t.Fatal("the refused run reported nothing")
	}
	result := m.UpdateCenter.Applied[0]
	if result.Status != updates.StatusPermission {
		t.Fatalf("the run's report says %s, want the permission refusal the manager gave", result.Status)
	}
	if !strings.Contains(result.Reason, "a password is required") {
		t.Fatalf("the run's report does not carry the manager's reason: %q", result.Reason)
	}
}

// TestACancelledPasswordReleasesTheUpdateRun pins the other release: the user
// decides not to give it. The screen goes, the field is cleared, and the sequence
// runs so every command reports for itself -- a run that silently skipped would
// leave the screen with nothing to say about why.
func TestACancelledPasswordReleasesTheUpdateRun(t *testing.T) {
	recorder := withUpdateCredentialMocks(t, errors.New("sudo: a password is required"))

	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}

	m, runner := updateCenterTestModel(t, apt)
	m.UpdateCenter.Engine.Runner = updateCredentialRunner{inner: runner, record: recorder.record}
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, probe := updateCenterConfirmedModel(t, m, []updates.Item{tar})
	m = updateCenterRunToEnd(t, m, probe)
	if m.Screen != ScreenAdminPassword {
		t.Fatalf("the run is on screen %v, want the password screen", m.Screen)
	}

	next, follow := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.Screen != ScreenUpdateCenter {
		t.Fatalf("escape left the run on screen %v, want the update centre", m.Screen)
	}
	if !strings.Contains(strings.Join(m.LogLines, "\n"), "not given") {
		t.Fatalf("the run's record does not say the password was not given:\n%s", strings.Join(m.LogLines, "\n"))
	}

	m = updateCenterRunToEnd(t, m, follow)
	if got := recorder.count("run:apt"); got == 0 {
		t.Fatalf("a cancelled credential stopped the sequence from running: %v", recorder.events)
	}
	if m.SudoPrimed {
		t.Error("a cancelled request is recorded as a cached credential")
	}
}

// TestTheUpdateApplyAndTheStepLoopAskThroughTheSameScreen pins that the ask is one
// thing and not two: both flows put the same screen up and record which one asked,
// so the answer goes back to the flow that is waiting.
func TestTheUpdateApplyAndTheStepLoopAskThroughTheSameScreen(t *testing.T) {
	cases := []struct {
		name string
		flow credentialAskFlow
		msg  tea.Msg
	}{
		{name: "the step loop", flow: credentialAskStepLoop, msg: sudoNeedsPasswordMsg{}},
		{name: "the update apply", flow: credentialAskUpdateCenter, msg: sudoCredentialCheckMsg{err: errors.New("sudo: a password is required")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Screen: ScreenInstalling, LogLines: []string{}}
			if tc.flow == credentialAskUpdateCenter {
				m.Screen = ScreenUpdateCenter
				m.UpdateCenter.Applying = true
			}
			next, cmd := m.Update(tc.msg)
			m = next.(Model)

			if m.Screen != ScreenAdminPassword {
				t.Fatalf("the run stayed on %v, want the installer's own password screen", m.Screen)
			}
			if cmd != nil {
				t.Fatal("the screen was put up with work already in flight, so it would race the answer")
			}
			if m.CredentialAsk != tc.flow {
				t.Fatalf("the screen recorded flow %v, want %v", m.CredentialAsk, tc.flow)
			}
		})
	}
}
