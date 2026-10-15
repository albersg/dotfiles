package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	msg := defaultSudoPasswordPrimer(password)()
	if _, ok := msg.(sudoPrimedMsg); !ok {
		t.Fatalf("the attempt produced %T, want sudoPrimedMsg", msg)
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
	msg := defaultSudoPasswordPrimer(password)()

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

// TestEscapeLeavesTheTerminalHandoverReachable pins S3. A system whose PAM needs a
// terminal cannot be authenticated through `sudo -S`, and that must be a way out
// rather than a dead end: escape offers the handover the installer used before.
func TestEscapeLeavesTheTerminalHandoverReachable(t *testing.T) {
	original := sudoTerminalPrimer
	t.Cleanup(func() { sudoTerminalPrimer = original })

	offered := 0
	sudoTerminalPrimer = func() tea.Cmd {
		offered++
		return func() tea.Msg { return sudoPrimedMsg{} }
	}

	m := Model{
		Screen:        ScreenAdminPassword,
		AdminPassword: append(make([]byte, 0, adminPasswordCapacity), []byte("typed")...),
		LogLines:      []string{},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if offered != 1 {
		t.Fatalf("escape offered the handover %d times, want exactly 1", offered)
	}
	if cmd == nil {
		t.Fatal("escape offered no handover, so a system that cannot use sudo -S has no way forward")
	}
	for i, b := range m.AdminPassword {
		if b != 0 {
			t.Fatalf("byte %d of the password buffer survives escape, so the typed password stays in memory", i)
		}
	}
	if !strings.Contains(strings.Join(m.LogLines, "\n"), "terminal") {
		t.Fatalf("the screen does not say where the password is about to be asked:\n%s", strings.Join(m.LogLines, "\n"))
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
