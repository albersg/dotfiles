package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeSudo puts a script named `sudo` at the front of PATH that records what it
// was given: its arguments in one file, and everything on its standard input in
// another.
//
// This is what makes the credential path observable without invoking the real
// sudo, and it is deliberately a *process* rather than a seam: the question the
// tests below have to answer is which of the two channels the password travels
// on, and only a real child process can be asked that.
func fakeSudo(t *testing.T) (argsFile, stdinFile string) {
	t.Helper()

	dir := t.TempDir()
	argsFile = filepath.Join(dir, "argv")
	stdinFile = filepath.Join(dir, "stdin")

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argsFile + "\n" +
		"cat > " + stdinFile + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile, stdinFile
}

// TestThePasswordReachesSudoWithoutATerminal pins S1 and S4 together, because on
// this path they are the same question: sudo is told to read the password from
// its standard input, the password arrives there, it never appears among the
// arguments where any account on the host could read it, and the bytes the
// interface held are cleared once sudo has had them.
func TestThePasswordReachesSudoWithoutATerminal(t *testing.T) {
	argsFile, stdinFile := fakeSudo(t)

	password := []byte("correct horse battery staple")
	original := append([]byte(nil), password...)

	msg := defaultSudoPasswordPrimer(password, 7)()
	primed, ok := msg.(sudoPrimedMsg)
	if !ok {
		t.Fatalf("the attempt produced %T, want sudoPrimedMsg", msg)
	}
	if primed.attempt != 7 {
		t.Fatalf("the answer carries attempt %d, want the 7 the screen numbered it with, so a straggler can be told apart", primed.attempt)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the fake sudo recorded no arguments: %v", err)
	}
	if !strings.Contains(string(args), "-S") {
		t.Fatalf("sudo was not told to read the password from its standard input: %q", args)
	}
	if !strings.Contains(string(args), "-v") {
		t.Fatalf("sudo was not asked to validate the credential: %q", args)
	}
	if strings.Contains(string(args), string(original)) {
		t.Fatal("the password travelled in sudo's arguments, where any account on the host can read it")
	}

	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("the fake sudo recorded no standard input: %v", err)
	}
	if strings.TrimRight(string(stdin), "\n") != string(original) {
		t.Fatalf("sudo read %q from its standard input, want the password", stdin)
	}

	// S4. The same backing array the model's slice header points at must now be
	// zeros: a password left in memory is a password left for anyone who can read
	// the process.
	for i, b := range password {
		if b != 0 {
			t.Fatalf("byte %d of the password buffer is still %q after the attempt, so the bytes were not cleared", i, b)
		}
	}
}

// TestARejectedPasswordIsNotEchoedOrLeftBehind pins the other half of S4: what
// the screen shows when sudo refuses. The line names the outcome, and it never
// carries the password the user typed nor sudo's own stderr, which names the
// account and the host.
func TestARejectedPasswordIsNotEchoedOrLeftBehind(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat > /dev/null\necho 'sudo: 3 incorrect password attempts' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	password := []byte("wrong horse")
	msg := defaultSudoPasswordPrimer(password, 1)()

	primed, ok := msg.(sudoPrimedMsg)
	if !ok {
		t.Fatalf("a refused password produced %T, want sudoPrimedMsg carrying the failure", msg)
	}
	if primed.err == nil {
		t.Fatal("a refused password was reported as a success")
	}
	if line := primed.err.Error(); strings.Contains(line, "wrong horse") {
		t.Fatalf("the password was repeated back in the error line: %q", line)
	}
	for i, b := range password {
		if b != 0 {
			t.Fatalf("byte %d of the password buffer survives a rejected attempt, so a retry would read the old password", i)
		}
	}
}

// TestTheProbeAsksNothingWhenSudoWantsNoPassword pins the reason the probe exists
// at all. `sudo -n -v` succeeds where sudo is configured not to want a password,
// and the run must then never put a prompt in front of the user: on a machine
// that needs no password, an in-interface prompt would be a new annoyance rather
// than a fix.
func TestTheProbeAsksNothingWhenSudoWantsNoPassword(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + filepath.Join(dir, "argv") + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	msg := defaultSudoPrimer()()

	if _, ok := msg.(sudoPrimedMsg); !ok {
		t.Fatalf("a machine whose sudo needs no password produced %T, want sudoPrimedMsg and no prompt", msg)
	}

	args, err := os.ReadFile(filepath.Join(dir, "argv"))
	if err != nil {
		t.Fatalf("the fake sudo recorded no arguments: %v", err)
	}
	if !strings.Contains(string(args), "-n") {
		t.Fatalf("the probe would have let sudo prompt: %q", args)
	}
}

// TestTheProbeAsksForThePasswordOnTheInstallerScreen pins the other side of the
// probe and S2: when sudo does need a password, the run stops on the installer's
// own screen with its own field, and the terminal is never handed over.
func TestTheProbeAsksForThePasswordOnTheInstallerScreen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	msg := defaultSudoPrimer()()
	if _, ok := msg.(sudoNeedsPasswordMsg); !ok {
		t.Fatalf("a machine whose sudo needs a password produced %T, want sudoNeedsPasswordMsg", msg)
	}

	m := Model{Screen: ScreenInstalling, LogLines: []string{}}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.Screen != ScreenAdminPassword {
		t.Fatalf("the run stayed on screen %v, want the installer's own password screen", m.Screen)
	}
	if cap(m.AdminPassword) != adminPasswordCapacity {
		t.Fatalf("the field holds capacity %d, want the fixed %d so no append can allocate a second array behind us", cap(m.AdminPassword), adminPasswordCapacity)
	}
	if len(m.LogLines) == 0 {
		t.Fatal("the password request is not visible on the screen")
	}
}

// TestTheFieldShowsHowMuchIsTypedAndNotWhat pins S2's masking. The screen may
// say how many characters the field holds; it must never draw one of them, and it
// must never put them in a log line either.
func TestTheFieldShowsHowMuchIsTypedAndNotWhat(t *testing.T) {
	m := Model{
		Screen:        ScreenAdminPassword,
		AdminPassword: append(make([]byte, 0, adminPasswordCapacity), []byte("hunter2")...),
	}

	rendered := m.renderAdminPassword()

	if strings.Contains(rendered, "hunter2") {
		t.Fatal("the screen draws the password itself")
	}
	for _, line := range m.LogLines {
		if strings.Contains(line, "hunter2") {
			t.Fatal("a log line carries the password")
		}
	}
	if !strings.Contains(rendered, "7") {
		t.Fatalf("the screen does not say how much has been typed, so the field reads as unresponsive:\n%s", rendered)
	}
}

// TestEscapeReleasesTheScreenInsteadOfAskingElsewhere pins the one way out that
// does not leave the interface. Escape used to hand the terminal to `sudo -v` for a
// system whose PAM wants one; that handover is what put the prompt in the shell and
// then hung, so Escape now stops waiting: the field is cleared, the reason is on the
// run's own record, and the flow that asked resumes so its root command reports the
// credential it lacks.
func TestEscapeReleasesTheScreenInsteadOfAskingElsewhere(t *testing.T) {
	m := Model{
		Screen:        ScreenAdminPassword,
		AdminPassword: append(make([]byte, 0, adminPasswordCapacity), []byte("typed")...),
		LogLines:      []string{},
		Steps:         []InstallStep{{ID: "deps", Name: "Install Dependencies"}},
	}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword[:0], []byte("typed")...)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.Screen == ScreenAdminPassword {
		t.Fatal("escape left the run on the password screen, so the wait never ends")
	}
	if len(m.AdminPassword) != 0 {
		t.Fatalf("the field still holds %q after escape, so the typed password stays in the model", m.AdminPassword)
	}
	if m.CredentialAsk != credentialAskNone {
		t.Fatal("escape left the credential ask recorded, so a later answer could resume the run a second time")
	}
	if m.SudoPrimed {
		t.Fatal("a cancelled request is recorded as a cached credential")
	}
	if !m.SudoPrimeAttempted {
		t.Fatal("a cancelled request is not recorded as asked, so the run would ask again in a loop")
	}
	joined := strings.Join(m.LogLines, "\n")
	if !strings.Contains(joined, "not given") {
		t.Fatalf("the run's record does not say why the credential is missing:\n%s", joined)
	}
	if !strings.Contains(joined, "will report it") {
		t.Fatalf("the run's record does not say what happens next:\n%s", joined)
	}
	if cmd == nil {
		t.Fatal("escape released the screen but resumed nothing, so the run would stop without a report")
	}
}

// TestCtrlCWhileAskingReleasesTheScreenInsteadOfQuitting pins the second way out.
// ctrl+c quits everywhere else, and on this screen it must not: a run waiting for
// an answer has to end in a state the user can read, and a program that quits from
// under a prompt in flight is neither a report nor a release.
func TestCtrlCWhileAskingReleasesTheScreenInsteadOfQuitting(t *testing.T) {
	m := Model{
		Screen:   ScreenInstalling,
		LogLines: []string{},
		Steps:    []InstallStep{{ID: "deps", Name: "Install Dependencies"}},
	}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword, []byte("half typed")...)
	m.AdminPasswordPending = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = result.(Model)

	if m.Quitting {
		t.Fatal("ctrl+c quit the program from the password screen instead of releasing it")
	}
	if m.Screen == ScreenAdminPassword {
		t.Fatal("ctrl+c left the run waiting on the password screen")
	}
	if len(m.AdminPassword) != 0 {
		t.Fatalf("the field still holds %q after ctrl+c", m.AdminPassword)
	}
	joined := strings.Join(m.LogLines, "\n")
	if !strings.Contains(joined, "cancelled") {
		t.Fatalf("the run's record does not say the attempt was cancelled:\n%s", joined)
	}
}

// TestAnAnswerToACancelledAttemptIsDiscarded pins the other side of cancelling: the
// attempt that was in flight still answers, and that answer must not resume a flow
// the run has already resumed -- a second resume would run the step loop twice.
func TestAnAnswerToACancelledAttemptIsDiscarded(t *testing.T) {
	m := Model{Screen: ScreenInstalling, LogLines: []string{}, Steps: []InstallStep{{ID: "deps", Name: "Install Dependencies"}}}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword, []byte("typed")...)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	if cmd == nil || !m.AdminPasswordPending {
		t.Fatal("Enter did not start an attempt, so this guard measures nothing")
	}
	attempt := m.AdminPasswordAttempt

	// The user cancels while sudo still has the bytes.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)
	before := len(m.LogLines)

	// And then the answer lands.
	result, follow := m.Update(sudoPrimedMsg{attempt: attempt, err: errors.New("too late")})
	m = result.(Model)

	if follow != nil {
		t.Fatal("a cancelled attempt's answer resumed the flow a second time")
	}
	if len(m.LogLines) != before {
		t.Fatalf("a cancelled attempt's answer was still written to the run's record: %v", m.LogLines[before:])
	}
	if m.SudoPrimed {
		t.Fatal("a cancelled attempt's answer primed the run")
	}
}

// TestTheFieldOwnsItsKeys pins the bug this change would otherwise carry back in.
// handleKeyPress takes three keys before the per-screen dispatch: space opens the
// leader key, and backspace and escape both mean "go back". On a screen whose
// whole job is a text field, that takes three characters away -- a password
// containing a space could not be typed at all, the space would arm the leader
// key, and the next key would be read as a command instead of a character. The
// screen has to take the dispatch first, and this is what says so.
func TestTheFieldOwnsItsKeys(t *testing.T) {
	m := Model{
		Screen:        ScreenAdminPassword,
		AdminPassword: make([]byte, 0, adminPasswordCapacity),
		LogLines:      []string{},
	}

	// A space is a character of the password, not the leader key.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = result.(Model)
	if m.LeaderMode {
		t.Fatal("space armed the leader key instead of being typed into the field")
	}
	if string(m.AdminPassword) != " " {
		t.Fatalf("space went somewhere other than the field: it holds %q", m.AdminPassword)
	}

	// Backspace deletes what was just typed. It is not "go back".
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result.(Model)
	if len(m.AdminPassword) != 0 {
		t.Fatalf("backspace did not delete the last character: the field holds %q", m.AdminPassword)
	}
	if m.Screen != ScreenAdminPassword {
		t.Fatalf("backspace left the screen (%v), so a mistyped password could not be corrected", m.Screen)
	}

	// A paste arrives as runes and lands whole, spaces included.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a b")})
	m = result.(Model)
	if string(m.AdminPassword) != "a b" {
		t.Fatalf("the field holds %q, want the pasted text", m.AdminPassword)
	}

	// And the capacity rule holds under a paste longer than the field: it must be
	// refused, not silently reallocated into an array nothing can clear.
	long := strings.Repeat("x", adminPasswordCapacity*2)
	before := cap(m.AdminPassword)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(long)})
	m = result.(Model)
	if cap(m.AdminPassword) != before {
		t.Fatalf("a long paste grew the buffer from %d to %d, so the bytes typed so far now live in an array that cannot be cleared", before, cap(m.AdminPassword))
	}
	if len(m.AdminPassword) > adminPasswordCapacity {
		t.Fatalf("the field holds %d bytes, past its fixed capacity of %d", len(m.AdminPassword), adminPasswordCapacity)
	}
}

// normalizeRender folds a rendered screen into one line of lowercase words, so a
// guard can ask whether a phrase is on the screen at all without depending on
// where the frame happened to wrap it. It strips nothing: the words a styled
// segment carries stay words, which is all these assertions read.
func normalizeRender(rendered string) string {
	var b strings.Builder
	for _, word := range strings.Fields(rendered) {
		b.WriteString(strings.ToLower(word))
		b.WriteString(" ")
	}
	return b.String()
}

// TestTheScreenSaysWhatToTypeAndThatEnterValidates pins S5. The screen was
// readable only to someone who already knew what to do: an empty field drew the
// word "waiting", which reads as the installer waiting for something rather than
// as the place to type, and the only key the footer named was Enter/select -- when
// this screen's Enter is not a selection at all, it is an attempt. The guard holds
// the two halves that make it unmistakable: the screen says the action in its own
// words, and nothing on it calls Enter a selection.
func TestTheScreenSaysWhatToTypeAndThatEnterValidates(t *testing.T) {
	empty := Model{Screen: ScreenAdminPassword, AdminPassword: make([]byte, 0, adminPasswordCapacity)}
	plain := normalizeRender(empty.renderAdminPassword())

	if !strings.Contains(plain, "press enter") {
		t.Fatalf("the screen never says that Enter validates what was typed on it:\n%s", empty.renderAdminPassword())
	}
	if !strings.Contains(plain, "type") {
		t.Fatalf("the screen does not name the action where the password goes:\n%s", empty.renderAdminPassword())
	}
	if strings.Contains(plain, "waiting") {
		t.Fatalf("an empty field reads as the installer waiting for something rather than as a place to type:\n%s", empty.renderAdminPassword())
	}
	if strings.Contains(plain, "select") {
		t.Fatalf("the footer calls this screen's Enter a selection, which is not what it does here:\n%s", empty.renderAdminPassword())
	}

	// The instruction does not disappear once a character is typed: the screen a
	// user is looking at mid-password has to answer the same question as the empty
	// one, and that is the state it was reported from.
	typed := Model{
		Screen:        ScreenAdminPassword,
		AdminPassword: append(make([]byte, 0, adminPasswordCapacity), []byte("hunter2")...),
	}
	midway := normalizeRender(typed.renderAdminPassword())
	for _, want := range []string{"press enter", "type"} {
		if !strings.Contains(midway, want) {
			t.Fatalf("with something typed the screen stopped saying %q:\n%s", want, typed.renderAdminPassword())
		}
	}
}

// fakeSudoScript puts a `sudo` at the front of PATH whose body is body, so an
// attempt can be measured against a child process that refuses, hangs or ignores
// its standard input without any of those being the real sudo.
func fakeSudoScript(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// messageWithin runs one command and fails the test when it does not answer inside
// limit. It is how "no hang" is measured rather than asserted: a command that never
// returns takes the whole test binary down with it, and a failed test says which
// wait was left open.
func messageWithin(t *testing.T, cmd tea.Cmd, limit time.Duration) tea.Msg {
	t.Helper()

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(limit):
		t.Fatalf("the attempt did not answer within %s, so a stuck sudo froze the run", limit)
		return nil
	}
}

// TestARefusedPasswordReleasesTheScreenWithTheReason pins the reported shape of a
// wrong password. It is not a second question and it is not a wait: the answer is
// turned into a line on the run, the screen goes, and the flow that asked resumes
// so its command reports the credential it lacks through `sudo -n` -- in the
// manager's own words, which is the report the user can act on.
func TestARefusedPasswordReleasesTheScreenWithTheReason(t *testing.T) {
	fakeSudoScript(t, "cat > /dev/null\necho 'sudo: 1 incorrect password attempt' >&2\nexit 1")

	m := Model{Screen: ScreenInstalling, LogLines: []string{}, Steps: []InstallStep{{ID: "deps", Name: "Install Dependencies"}}}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword, []byte("wrong horse")...)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	if cmd == nil {
		t.Fatal("Enter started no attempt, so this guard measures nothing")
	}
	if !m.AdminPasswordPending {
		t.Fatal("the attempt is not recorded as in flight, so the screen cannot say it is waiting")
	}

	answer := messageWithin(t, cmd, 10*time.Second)
	result, _ = m.Update(answer)
	m = result.(Model)

	if m.Screen == ScreenAdminPassword {
		t.Fatal("a refused password left the run waiting on the password screen")
	}
	if m.AdminPasswordPending {
		t.Fatal("the screen is released but still recorded as waiting for sudo")
	}
	if m.SudoPrimed {
		t.Error("a refused password is recorded as a cached credential")
	}
	if len(m.AdminPassword) != 0 {
		t.Errorf("the field still holds %q after the attempt", m.AdminPassword)
	}
	joined := strings.Join(m.LogLines, "\n")
	if !strings.Contains(joined, "not accepted") {
		t.Fatalf("the run's record does not carry the reason the attempt failed:\n%s", joined)
	}
	if !strings.Contains(joined, "will report it") {
		t.Fatalf("the run's record does not say what happens next:\n%s", joined)
	}
	if strings.Contains(joined, "incorrect password attempt") {
		t.Fatalf("the run's record repeats sudo's stderr, which names the account and the host:\n%s", joined)
	}
}

// TestAStuckSudoCannotFreezeTheScreen pins the bound. A sudo that ignores its
// standard input -- the case a `Defaults requiretty` or an askpass-only
// configuration produces -- never answers, and the wait is what turns that into a
// line on the run instead of a screen that never returns.
func TestAStuckSudoCannotFreezeTheScreen(t *testing.T) {
	original := sudoPasswordTimeout
	t.Cleanup(func() { sudoPasswordTimeout = original })
	sudoPasswordTimeout = 100 * time.Millisecond

	fakeSudoScript(t, "exec sleep 30")

	m := Model{Screen: ScreenInstalling, LogLines: []string{}, Steps: []InstallStep{{ID: "deps", Name: "Install Dependencies"}}}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword, []byte("any password")...)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	answer := messageWithin(t, cmd, 10*time.Second)
	primed, ok := answer.(sudoPrimedMsg)
	if !ok {
		t.Fatalf("the stopped attempt produced %T, want sudoPrimedMsg", answer)
	}
	if primed.err == nil {
		t.Fatal("a sudo that never answered reported success")
	}
	if !strings.Contains(primed.err.Error(), "in time") {
		t.Fatalf("the stopped attempt does not say it was stopped: %v", primed.err)
	}

	result, _ = m.Update(answer)
	m = result.(Model)
	if m.Screen == ScreenAdminPassword {
		t.Fatal("a sudo that never answered left the run waiting on the password screen")
	}
	if !strings.Contains(strings.Join(m.LogLines, "\n"), "in time") {
		t.Fatalf("the run's record does not say the attempt was stopped:\n%s", strings.Join(m.LogLines, "\n"))
	}
}

// TestThePasswordNeverReachesTheScreenTheLogOrAModelCopy pins the promise the whole
// path exists to keep: the bytes exist for as long as sudo takes to read them and
// nowhere else. The screen is checked mid-attempt as well as after it, because the
// screen is the one place a mistake would be visible to somebody standing behind
// the user.
func TestThePasswordNeverReachesTheScreenTheLogOrAModelCopy(t *testing.T) {
	const password = "correct-horse-battery-staple"

	cases := []struct {
		name string
		body string
	}{
		{name: "sudo accepts it", body: "cat > /dev/null\nexit 0"},
		{name: "sudo refuses it", body: "cat > /dev/null\necho 'sudo: 3 incorrect password attempts' >&2\nexit 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeSudoScript(t, tc.body)

			m := Model{Screen: ScreenInstalling, LogLines: []string{}, Steps: []InstallStep{{ID: "deps", Name: "Install Dependencies"}}}
			m.askAdminPassword(credentialAskStepLoop)
			m.AdminPassword = append(m.AdminPassword, []byte(password)...)

			// The screen the user is looking at while sudo has the bytes.
			if drawn := m.renderAdminPassword(); strings.Contains(drawn, password) {
				t.Fatalf("the password screen draws the password:\n%s", drawn)
			}

			result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = result.(Model)
			if drawn := m.renderAdminPassword(); strings.Contains(drawn, password) {
				t.Fatalf("the password screen draws the password while the attempt is in flight:\n%s", drawn)
			}

			answer := messageWithin(t, cmd, 10*time.Second)
			result, _ = m.Update(answer)
			m = result.(Model)

			if len(m.AdminPassword) != 0 {
				t.Fatalf("the model kept %d bytes of the password after the attempt", len(m.AdminPassword))
			}
			for _, line := range m.LogLines {
				if strings.Contains(line, password) {
					t.Fatalf("a log line carries the password: %q", line)
				}
			}
			if strings.Contains(m.ErrorMsg, password) {
				t.Fatalf("the run's error carries the password: %q", m.ErrorMsg)
			}
			if primed, ok := answer.(sudoPrimedMsg); ok && primed.err != nil {
				if strings.Contains(primed.err.Error(), password) {
					t.Fatalf("the answer carries the password: %v", primed.err)
				}
			}
		})
	}
}

// TestThePromptAppearsForBothFlows pins that both callers ask in the same place and
// on the same screen, and that the answer is routed back to the flow that asked
// rather than to whichever one happens to be waiting.
func TestThePromptAppearsForBothFlows(t *testing.T) {
	cases := []struct {
		name string
		flow credentialAskFlow
		msg  tea.Msg
	}{
		{name: "the deps step's step loop", flow: credentialAskStepLoop, msg: sudoNeedsPasswordMsg{}},
		{name: "the update centre's apply", flow: credentialAskUpdateCenter, msg: sudoCredentialCheckMsg{err: errors.New("sudo: a password is required")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Screen: ScreenInstalling, LogLines: []string{}}
			if tc.flow == credentialAskUpdateCenter {
				m.Screen = ScreenUpdateCenter
				m.UpdateCenter.Applying = true
			}

			result, _ := m.Update(tc.msg)
			m = result.(Model)

			if m.Screen != ScreenAdminPassword {
				t.Fatalf("the run stayed on %v, want the installer's own password screen", m.Screen)
			}
			if m.CredentialAsk != tc.flow {
				t.Fatalf("the screen recorded flow %v, want %v: the answer would resume the wrong one", m.CredentialAsk, tc.flow)
			}
			if cap(m.AdminPassword) != adminPasswordCapacity || len(m.AdminPassword) != 0 {
				t.Fatalf("the field is capacity %d len %d, want an empty field of the fixed %d", cap(m.AdminPassword), len(m.AdminPassword), adminPasswordCapacity)
			}
			if !strings.Contains(strings.Join(m.LogLines, "\n"), "on this screen") {
				t.Fatalf("the screen's arrival is not on the run's record:\n%s", strings.Join(m.LogLines, "\n"))
			}
		})
	}
}

// TestTheCredentialPathNeverHandsTheTerminalOver pins the shape of the fix at the
// source, because the failure it replaces -- a prompt in the outer shell that then
// hung -- is invisible to any guard that only reads the model. The files on this
// path may not use the two mechanisms that hand the terminal to a child.
func TestTheCredentialPathNeverHandsTheTerminalOver(t *testing.T) {
	for _, path := range []string{"admin_password.go", "interactive.go", "update_center.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		// The call, not the word: interactive.go keeps the type that names the
		// handover this program no longer makes, and naming it is what the guard
		// against its return is built on.
		for _, forbidden := range []string{"tea.ExecProcess", "system.RunTerminal"} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s uses %s, which hands the terminal to a child and is the handover this path no longer makes", path, forbidden)
			}
		}
	}
}

// TestASudoThatNeedsATerminalSaysSoAndReleasesTheScreen pins the one case the
// in-screen prompt genuinely cannot cover: a PAM configuration that demands a
// terminal -- a two-factor or fingerprint module, or an old `Defaults requiretty` --
// refuses standard input however right the password is. The refusal is sudo's, and
// it is turned into a line that says what happened, on a screen that goes, so the
// run reports it instead of dead-ending on a prompt that cannot succeed.
func TestASudoThatNeedsATerminalSaysSoAndReleasesTheScreen(t *testing.T) {
	fakeSudoScript(t, "cat > /dev/null\necho 'sudo: no tty present and no askpass program specified' >&2\nexit 1")

	m := Model{Screen: ScreenInstalling, LogLines: []string{}, Steps: []InstallStep{{ID: "deps", Name: "Install Dependencies"}}}
	m.askAdminPassword(credentialAskStepLoop)
	m.AdminPassword = append(m.AdminPassword, []byte("a correct password")...)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	answer := messageWithin(t, cmd, 10*time.Second)
	primed, ok := answer.(sudoPrimedMsg)
	if !ok || primed.err == nil {
		t.Fatalf("a sudo that needs a terminal reported %v, want a failure", answer)
	}
	if !strings.Contains(primed.err.Error(), "without a terminal") {
		t.Fatalf("the refusal does not say what this machine needs: %v", primed.err)
	}
	if strings.Contains(primed.err.Error(), "no tty present") {
		t.Fatalf("the refusal repeats sudo's own words instead of saying what it means: %v", primed.err)
	}

	result, _ = m.Update(answer)
	m = result.(Model)
	if m.Screen == ScreenAdminPassword {
		t.Fatal("a machine that cannot be authenticated on this screen was left waiting on it")
	}
}
