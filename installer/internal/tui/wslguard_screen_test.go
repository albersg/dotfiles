package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// wslGuardScreenModel is the WSL resource screen with one scripted guard on it.
// The state around the guard comes from the same fixture the other WSL screen
// tests use, so these tests are about the guard's paragraphs and rows and not
// about the recommendation, which has its own.
func wslGuardScreenModel(t *testing.T, guard wslGuardState) Model {
	t.Helper()

	m := NewModel()
	m.Screen = ScreenWSLResources
	m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: true, OSName: "WSL"}
	state := wslResourceTestState(t)
	state.Guard = guard
	m.WSLState = state
	return m
}

// wslGuardScreenBody is the screen's whole description as one string, which is
// what a reader sees rather than the slice the renderer walks.
func wslGuardScreenBody(m Model) string {
	return strings.Join(m.wslResourceDescription(), " ")
}

// TestTheScreenNamesTheFractionThePolicyApplies pins the one sentence that states
// the fraction in words against the divisor the policy divides by. Nothing held
// the two together, so 5d312a9 changed the arithmetic to leave Windows a quarter
// of the host and left this screen saying "two thirds" of it with the whole suite
// green: the reader was told a proportion the code no longer applied. The map is
// the sentence's half of the contract, so a divisor it has no words for fails
// here rather than in front of a user.
func TestTheScreenNamesTheFractionThePolicyApplies(t *testing.T) {
	fractions := map[uint64]string{2: "half", 3: "two thirds", 4: "three quarters"}

	words, ok := fractions[wslMemoryRatioDivisor]
	if !ok {
		t.Fatalf("wslMemoryRatioDivisor is %d and the screen's sentence has no words for it: "+
			"teach the sentence the fraction before changing the policy", wslMemoryRatioDivisor)
	}

	body := wslGuardScreenBody(wslGuardScreenModel(t, wslGuardState{}))

	sentence := "Memory is " + words + " of the host's RAM"
	if !strings.Contains(body, sentence) {
		t.Errorf("the screen does not name the fraction the policy applies (%q):\n%s", sentence, body)
	}

	// The stale half: a sentence naming a fraction the policy does not divide by is
	// the defect this test exists for, and the other words in the map are exactly
	// the ones it could be reading.
	for divisor, word := range fractions {
		if divisor == wslMemoryRatioDivisor {
			continue
		}
		stale := "Memory is " + word + " of the host's RAM"
		if strings.Contains(body, stale) {
			t.Errorf("the screen names %q while the policy divides by %d:\n%s",
				stale, wslMemoryRatioDivisor, body)
		}
	}
}

// wslGuardRepairRowIndex finds the guard row in the screen's own options, so a
// test presses the row the screen drew rather than an index it guessed at.
func wslGuardRepairRowIndex(t *testing.T, m Model) int {
	t.Helper()

	for i, option := range m.GetCurrentOptions() {
		if option == wslGuardRepairRow {
			return i
		}
	}
	t.Fatalf("the screen does not offer %q: %v", wslGuardRepairRow, m.GetCurrentOptions())
	return 0
}

// TestWSLResourceScreenNamesWhatAMissingGuardCosts pins the half of S6 that is a
// warning rather than a value. The two protections fail in opposite directions,
// so the screen names the failure that was measured instead of printing one
// generic "not fully protected": a policy of stop killed the whole distribution
// twice, and a throttle band froze it for six minutes.
func TestWSLResourceScreenNamesWhatAMissingGuardCosts(t *testing.T) {
	tests := []struct {
		name string
		// guard is the state the screen is given.
		guard wslGuardState
		// want is the failure this state has to name.
		want string
		// absent is a failure the screen must NOT name, because naming a
		// protection that is in place is its own kind of lie.
		absent string
	}{
		{
			name:   "a stop policy turns one kill into the whole distribution",
			guard:  wslGuardState{Installed: true, Active: true, OOMPolicy: "stop", HighBytes: "max", MaxBytes: 8817475584},
			want:   "one OOM kill inside init.scope still stops the scope",
			absent: "memory.high is",
		},
		{
			name:   "a throttle band freezes instead of killing",
			guard:  wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy, HighBytes: "7743733760", MaxBytes: 8817475584},
			want:   "a throttle rather than a limit",
			absent: "OOMPolicy reads",
		},
		{
			name:   "no cap at all leaves the victim to the OOM killer",
			guard:  wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy, HighBytes: "max", MaxBytes: 0},
			want:   "memory.max is unlimited",
			absent: "throttle rather than a limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := wslGuardScreenBody(wslGuardScreenModel(t, tt.guard))
			if !strings.Contains(body, tt.want) {
				t.Errorf("the screen does not name what is missing; want %q in:\n%s", tt.want, body)
			}
			if strings.Contains(body, tt.absent) {
				t.Errorf("the screen names %q, which is a protection that is in place:\n%s", tt.absent, body)
			}
		})
	}
}

// TestWSLResourceScreenSaysWhenTheGuardIsInPlace is the other half of the rule:
// a guard that holds all three values draws no warning at all, because a screen
// that warns about everything warns about nothing.
func TestWSLResourceScreenSaysWhenTheGuardIsInPlace(t *testing.T) {
	m := wslGuardScreenModel(t, wslGuardState{
		Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy,
		HighBytes: "max", MaxBytes: 8817475584, SwapBytes: 536870912,
	})

	body := wslGuardScreenBody(m)
	for _, want := range []string{
		"The memory guard is installed and systemd is running it",
		"memory.high=max",
		"memory.max=8409 MB",
		"OOMPolicy=continue",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not report %q:\n%s", want, body)
		}
	}
	for _, absent := range []string{"still stops the scope", "throttle rather than a limit", "memory.max is unlimited"} {
		if strings.Contains(body, absent) {
			t.Errorf("the screen warns with %q about a guard that holds:\n%s", absent, body)
		}
	}
}

// TestWSLResourceScreenReportsTheGuardItCannotRead pins the honest gap: a guard
// that is not installed and a guard whose state could not be read are different
// sentences, and neither is a zero that would read as "unlimited".
func TestWSLResourceScreenReportsTheGuardItCannotRead(t *testing.T) {
	t.Run("no guard installed", func(t *testing.T) {
		body := wslGuardScreenBody(wslGuardScreenModel(t, wslGuardState{}))
		if !strings.Contains(body, "The WSL memory guard is not installed on this host") {
			t.Errorf("an absent guard is not reported:\n%s", body)
		}
	})

	t.Run("the reason it could not be read is named", func(t *testing.T) {
		reason := "systemd is not the init of this distribution"
		body := wslGuardScreenBody(wslGuardScreenModel(t, wslGuardState{Reason: reason}))
		if !strings.Contains(body, reason) {
			t.Errorf("the reason the guard cannot be read is missing:\n%s", body)
		}
	})

	t.Run("a value that was not read says so instead of unlimited", func(t *testing.T) {
		body := wslGuardScreenBody(wslGuardScreenModel(t, wslGuardState{Installed: true, Active: true}))
		if !strings.Contains(body, "memory.high=unread") || !strings.Contains(body, "OOMPolicy=unread") {
			t.Errorf("an unread attribute is not named as unread:\n%s", body)
		}
		if strings.Contains(body, "memory.high=max") {
			t.Errorf("an unread memory.high was reported as unlimited:\n%s", body)
		}
	})
}

// TestWSLResourceRowsOfferTheGuardRepair pins that the repair is a row on the
// screen and not only a paragraph: a warning with no way to act on it is the gap
// S6 exists to close.
func TestWSLResourceRowsOfferTheGuardRepair(t *testing.T) {
	m := wslGuardScreenModel(t, wslGuardState{})

	rows := m.wslResourceRows()
	found := false
	for _, row := range rows {
		if row == wslGuardRepairRow {
			found = true
		}
	}
	if !found {
		t.Fatalf("the screen does not offer %q: %v", wslGuardRepairRow, rows)
	}

	// The three value rows keep their indices: the arrow keys adjust the value
	// under the cursor by index, so a row inserted before them would silently
	// adjust the wrong thing.
	for i, row := range rows[:3] {
		want := []string{"Memory: ", "Processors: ", "Swap: "}[i]
		if !strings.HasPrefix(row, want) {
			t.Fatalf("row %d is %q, want it to begin %q: the value rows moved", i, row, want)
		}
	}
}

// TestWSLResourceScreenRepairsTheGuardAndReportsWhatItRead pins the row's whole
// contract: it installs through the same call the installation step makes, and
// the notice reports the state read afterwards rather than the install's own word
// for it.
func TestWSLResourceScreenRepairsTheGuardAndReportsWhatItRead(t *testing.T) {
	installs := 0
	stepID := ""
	originalInstall, originalRead := wslGuardInstaller, wslGuardReader
	t.Cleanup(func() { wslGuardInstaller, wslGuardReader = originalInstall, originalRead })

	wslGuardInstaller = func(_ string, got string) error {
		installs++
		stepID = got
		return nil
	}
	wslGuardReader = func(string) wslGuardState {
		return wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy,
			HighBytes: "max", MaxBytes: 8817475584}
	}

	m := wslGuardScreenModel(t, wslGuardState{})
	m.Cursor = wslGuardRepairRowIndex(t, m)

	next, cmd := m.handleWSLResourceKeys("g")
	if cmd == nil {
		t.Fatal("pressing the guard row started no work")
	}
	if got := next.(Model).WSLNotice; !strings.Contains(got, "Installing the WSL memory guard") {
		t.Errorf("the press does not say what it is doing: %q", got)
	}

	msg, ok := cmd().(wslGuardRepairedMsg)
	if !ok {
		t.Fatalf("the guard row's command did not return a wslGuardRepairedMsg")
	}
	if installs != 1 {
		t.Errorf("the row installed the guard %d times, want 1", installs)
	}
	if stepID != "utilities" {
		t.Errorf("the row installed under stepID %q, want the utilities screen", stepID)
	}

	updated, _ := next.(Model).Update(msg)
	got := updated.(Model)
	if got.WSLState.Guard.OOMPolicy != wslGuardWantOOMPolicy || got.WSLState.Guard.MaxBytes == 0 {
		t.Errorf("the screen did not take the state read after the repair: %+v", got.WSLState.Guard)
	}
	if !strings.Contains(got.WSLNotice, "in place") {
		t.Errorf("the notice does not report the verified state: %q", got.WSLNotice)
	}
}

// TestWSLResourceScreenSaysWhenTheRepairDidNotTake is the failure half. A repair
// that ran and did not take is not a success with a footnote: the notice names the
// protection that is still missing, and a checkout that no longer ships the
// payload is reported as the packaging defect it is.
func TestWSLResourceScreenSaysWhenTheRepairDidNotTake(t *testing.T) {
	originalInstall, originalRead := wslGuardInstaller, wslGuardReader
	t.Cleanup(func() { wslGuardInstaller, wslGuardReader = originalInstall, originalRead })

	t.Run("a protection is still missing", func(t *testing.T) {
		wslGuardInstaller = func(string, string) error { return nil }
		wslGuardReader = func(string) wslGuardState {
			return wslGuardState{Installed: true, Active: true, OOMPolicy: "stop",
				HighBytes: "max", MaxBytes: 8817475584}
		}

		m := wslGuardScreenModel(t, wslGuardState{})
		msg := m.wslGuardRepairCmd()()
		updated, _ := m.Update(msg)

		notice := updated.(Model).WSLNotice
		if !strings.Contains(notice, "OOMPolicy reads stop") {
			t.Errorf("the notice does not name the protection that is missing: %q", notice)
		}
		if strings.Contains(notice, "in place") {
			t.Errorf("a guard that is still unprotected was reported as in place: %q", notice)
		}
	})

	t.Run("systemd is not running it", func(t *testing.T) {
		wslGuardInstaller = func(string, string) error { return nil }
		wslGuardReader = func(string) wslGuardState {
			return wslGuardState{Installed: true, Active: false, OOMPolicy: wslGuardWantOOMPolicy,
				HighBytes: "max", MaxBytes: 8817475584}
		}

		m := wslGuardScreenModel(t, wslGuardState{})
		updated, _ := m.Update(m.wslGuardRepairCmd()())
		if notice := updated.(Model).WSLNotice; !strings.Contains(notice, "not running it") {
			t.Errorf("an inactive unit was not reported: %q", notice)
		}
	})

	t.Run("the checkout no longer ships the payload", func(t *testing.T) {
		wslGuardInstaller = func(string, string) error { return errors.New("reading the payload: no such file") }
		wslGuardReader = func(string) wslGuardState { return wslGuardState{} }

		m := wslGuardScreenModel(t, wslGuardState{})
		updated, _ := m.Update(m.wslGuardRepairCmd()())
		if notice := updated.(Model).WSLNotice; !strings.Contains(notice, "could not be installed") {
			t.Errorf("a packaging defect was not reported as an install failure: %q", notice)
		}
	})
}

// TestWSLGuardPanelLabelNamesTheGuardInTwoWords pins what the Utilities section
// shows before the row is opened: the section is where a user reads the state of
// their host, and a guard that is missing or unprotected is the one value there
// that has to be acted on.
func TestWSLGuardPanelLabelNamesTheGuardInTwoWords(t *testing.T) {
	tests := []struct {
		name  string
		guard wslGuardState
		want  string
	}{
		{"nothing installed", wslGuardState{}, "missing"},
		{"installed but not running", wslGuardState{Installed: true}, "not running"},
		{"running with a stop policy", wslGuardState{Installed: true, Active: true, OOMPolicy: "stop", HighBytes: "max", MaxBytes: 8817475584}, "unprotected"},
		{"running with a throttle band", wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy, HighBytes: "7743733760", MaxBytes: 8817475584}, "unprotected"},
		{"running and uncapped", wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy, HighBytes: "max"}, "unprotected"},
		{"holding all three", wslGuardState{Installed: true, Active: true, OOMPolicy: wslGuardWantOOMPolicy, HighBytes: "max", MaxBytes: 8817475584}, "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wslGuardPanelLabel(tt.guard); got != tt.want {
				t.Errorf("wslGuardPanelLabel(%+v) = %q, want %q", tt.guard, got, tt.want)
			}
		})
	}
}
