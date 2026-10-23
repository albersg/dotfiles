package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE RUN'S LIVE SURFACE
// ============================================================================
//
// The guards below measure the surface a run in flight draws: the bar and the
// count that fill it, the command in flight and the entries it is moving, the
// entries that did not move, and the end of the manager's output. They read the
// rendered frame rather than the helpers, because the promise is about what the
// reader sees, and they read the numbers back out of the model's own state so a
// renderer that invented a percentage would fail here.

// The run's fixture: three commands over four entries, the first finished with
// one entry moved and one refused, the second in flight and the third behind it.
// The table holds a fifth entry the run was not asked to carry, so the bar's
// count and the list's count are visibly different facts.
const (
	updateRunFixtureDone   = 2 // entries the finished command settled
	updateRunFixtureTotal  = 4 // entries the sequence carries
	updateRunFixtureFailed = 1 // entries the finished command did not move
)

// updateCenterRunPlans is the sequence the fixture's screen is running, in the
// order the engine runs it.
func updateCenterRunPlans() (plans []updates.Plan, items []updates.Item) {
	libc6 := updates.Item{Manager: updates.ManagerApt, Scope: updates.ScopeSystem, Name: "libc6",
		Current: "2.41-1", Available: "2.41-2"}
	openssl := updates.Item{Manager: updates.ManagerApt, Scope: updates.ScopeSystem, Name: "openssl",
		Current: "3.5.0-1", Available: "3.5.1-1"}
	tar := updates.Item{Manager: updates.ManagerApt, Scope: updates.ScopeSystem, Name: "tar",
		Current: "1.35-1", Available: "1.35-2"}
	core22 := updates.Item{Manager: updates.ManagerSnap, Scope: updates.ScopeSystem, Name: "core22",
		Current: "20250101", Available: "20250201"}
	firefox := updates.Item{Manager: updates.ManagerSnap, Scope: updates.ScopeSystem, Name: "firefox",
		Current: "135.0-1", Available: "136.0-1"}

	plans = []updates.Plan{
		{
			Manager: updates.ManagerApt,
			Scope:   updates.ScopeSystem,
			Kind:    updates.KindPackage,
			Command: updates.Command{Argv: []string{"apt-get", "--yes", "install", "libc6", "openssl"}, Elevate: true},
			Items:   []updates.Item{libc6, openssl},
		},
		{
			Manager: updates.ManagerSnap,
			Scope:   updates.ScopeSystem,
			Kind:    updates.KindSnap,
			Command: updates.Command{Argv: []string{"snap", "refresh", "core22"}},
			Items:   []updates.Item{core22},
		},
		{
			Manager: updates.ManagerSnap,
			Scope:   updates.ScopeSystem,
			Kind:    updates.KindSnap,
			Command: updates.Command{Argv: []string{"snap", "refresh", "firefox"}},
			Items:   []updates.Item{firefox},
		},
	}
	return plans, []updates.Item{libc6, openssl, tar, core22, firefox}
}

// updateCenterRunFrameCase frames the update center in the middle of the run:
// system apt and snap, one command finished with a refusal, one in flight.
func updateCenterRunFrameCase(t *testing.T) Model {
	t.Helper()

	m := updateCenterFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plans, items := updateCenterRunPlans()
	m.UpdateCenter.Results[0].Items = items[:3]
	m.UpdateCenter.Results[1].Status = updates.StatusUpdates
	m.UpdateCenter.Results[1].Items = items[3:]
	m.UpdateCenter.Selected = updates.Selection{}
	m.UpdateCenter.Plans = plans
	m.UpdateCenter.Applied = []updates.ApplyResult{{
		Plan:    plans[0],
		Status:  updates.StatusPermission,
		Reason:  "sudo refused: no credential is cached",
		Command: plans[0].Display(),
		Outcomes: []updates.Outcome{
			{Item: items[0], Step: updates.StepConfirmed, Detail: "the manager no longer reports this as out of date"},
			{Item: items[1], Step: updates.StepFailed, Detail: "sudo refused: no credential is cached"},
		},
	}}
	m.UpdateCenter.Applying = true
	m.UpdateCenter.PlanIndex = 1
	m.UpdateCenter.Notice = ""
	// A run in flight owns the output channel, so a line arriving from the manager
	// is kept instead of dropped as a straggler from a run that already ended.
	m.UpdateCenter.Lines = make(chan string, updateCenterLineBuffer)
	m.UpdateCenter.Log = []string{
		"Reading package lists…",
		"Building dependency tree…",
		"Preparing to unpack core22_20250201.snap…",
	}
	return m
}

// updateCenterFrameRowsOf renders the frame and returns its lines without the
// escapes, the way the guards below read them.
func updateCenterFrameRowsOf(m Model) []string {
	return strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
}

// TestUpdateCenterRunTallyIsTheRunsOwnArithmetic pins the numbers the bar and the
// count are drawn from: an entry is settled when its command answered, an entry
// behind the command in flight is not, and every one of the sequence's entries is
// counted once.
func TestUpdateCenterRunTallyIsTheRunsOwnArithmetic(t *testing.T) {
	plans, items := updateCenterRunPlans()

	confirmed := updates.ApplyResult{Plan: plans[0], Outcomes: []updates.Outcome{
		{Item: items[0], Step: updates.StepConfirmed},
		{Item: items[1], Step: updates.StepConfirmed},
	}}
	refused := updates.ApplyResult{Plan: plans[1], Outcomes: []updates.Outcome{
		{Item: items[2], Step: updates.StepFailed},
	}}
	stopped := updates.ApplyResult{Plan: plans[2], Outcomes: []updates.Outcome{
		{Item: items[3], Step: updates.StepUnknown},
	}}

	for _, tc := range []struct {
		name                string
		steps               []updateCenterRunStep
		done, total, failed int
	}{
		{
			name:  "a command in flight settles nothing",
			steps: []updateCenterRunStep{{Plan: plans[0], Running: true}, {Plan: plans[1]}, {Plan: plans[2]}},
			done:  0, total: 4, failed: 0,
		},
		{
			name:  "a finished command settles what it answered for",
			steps: []updateCenterRunStep{{Plan: plans[0], Result: &confirmed}, {Plan: plans[1], Running: true}, {Plan: plans[2]}},
			done:  2, total: 4, failed: 0,
		},
		{
			name:  "a refused entry is settled and not moved",
			steps: []updateCenterRunStep{{Plan: plans[0], Result: &confirmed}, {Plan: plans[1], Result: &refused}, {Plan: plans[2], Running: true}},
			done:  3, total: 4, failed: 1,
		},
		{
			name:  "a stopped entry is settled and not moved either",
			steps: []updateCenterRunStep{{Plan: plans[0], Result: &confirmed}, {Plan: plans[1], Result: &refused}, {Plan: plans[2], Result: &stopped}},
			done:  4, total: 4, failed: 2,
		},
	} {
		done, total, failed := updateCenterRunTally(tc.steps)
		if done != tc.done || total != tc.total || failed != tc.failed {
			t.Errorf("%s: the tally is %d/%d with %d failed, want %d/%d with %d",
				tc.name, done, total, failed, tc.done, tc.total, tc.failed)
		}
	}

	// And the fixture itself says what the guards below expect of it.
	done, total, failed := updateCenterRunTally(updateCenterRunFrameCase(t).updateCenterRunSteps())
	if done != updateRunFixtureDone || total != updateRunFixtureTotal || failed != updateRunFixtureFailed {
		t.Fatalf("the run fixture settles %d/%d with %d refused, want %d/%d with %d",
			done, total, failed, updateRunFixtureDone, updateRunFixtureTotal, updateRunFixtureFailed)
	}
}

// TestTheUpdateCenterShowsTheRunProgressWhileItInstalls is the guard for the
// screen this pass added: while the sequence runs, the frame names the command in
// flight and the entries it is moving, fills a bar to the entries the finished
// commands settled, counts them, and keeps the keys and the table it had.
func TestTheUpdateCenterShowsTheRunProgressWhileItInstalls(t *testing.T) {
	m := updateCenterRunFrameCase(t)

	view := m.View()
	plain := ansiEscape.ReplaceAllString(view, "")
	assertScreenFitsTerminal(t, "update-center-running", m.Width, m.Height, view)

	for _, want := range []string{
		"Progress",    // the run's own card
		"step 2 of 3", // which command of how many
		fmt.Sprintf("%d/%d", updateRunFixtureDone, updateRunFixtureTotal), // the count the bar fills to
		"entries",  // what that count counts
		"Commands", // the sequence's block
		"apt (system) — 1 confirmed, 1 not moved", // the finished command, in the manager's own words
		"snap (system) — moving core22",           // the command in flight, with what it moves
		"Not moved",                               // the failures' block
		"openssl",                                 // the entry that did not move
		"sudo refused",                            // and the manager's own reason
		"Output",                                  // the live output block
		"Preparing to unpack core22_20250201.snap…", // the last line the manager printed
		"Installing…", // the frame's state word while the run is in flight
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the running screen does not draw %q:\n%s", want, plain)
		}
	}

	// The keys and the table are the screen's, not the run's: the update center
	// keeps its rows and its stop key while it installs.
	for _, want := range []string{"Package", "Current → New version", "Actions", "stop"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the running screen dropped %q, which the update center keeps:\n%s", want, plain)
		}
	}

	// The bar is the run's own fraction. It is drawn in whole entries, so the
	// number of filled cells is that fraction of the bar and never a figure the
	// screen rounded up for the eye.
	bar := ""
	for _, row := range updateCenterFrameRowsOf(m) {
		if strings.Contains(row, "entries") && strings.ContainsAny(row, "▓░") {
			bar = row
			break
		}
	}
	if bar == "" {
		t.Fatalf("the running screen draws no bar row:\n%s", plain)
	}
	filled, empty := strings.Count(bar, "▓"), strings.Count(bar, "░")
	if filled+empty != updateRunBarCells {
		t.Errorf("the bar is %d cells wide, want %d: %q", filled+empty, updateRunBarCells, bar)
	}
	if want := updateRunFixtureDone * updateRunBarCells / updateRunFixtureTotal; filled != want {
		t.Errorf("the bar fills %d of %d cells for %d of %d entries, want %d: %q",
			filled, updateRunBarCells, updateRunFixtureDone, updateRunFixtureTotal, want, bar)
	}
	if !strings.Contains(m.updateCenterFooterStatus(), "Running 2 of 3") {
		t.Errorf("the footer does not say which command is running: %q", m.updateCenterFooterStatus())
	}
}

// runBodyEnds is where a running frame's chrome is: the rule under the header and
// the footer's status row, which a run in flight draws as `● Installing…`. It is
// frameBodyEnds for the one screen whose state word is not `Ready`.
func runBodyEnds(rows []string) (rule, footer int) {
	rule, footer = -1, -1
	for i, row := range rows {
		if rule < 0 && strings.Contains(row, "━━━") {
			rule = i
		}
		if footer < 0 && strings.Contains(row, "● Installing") {
			footer = i
		}
	}
	return rule, footer
}

// TestTheRunCardHoldsItsRowsWhileEventsArrive is the guard for the asynchronous
// half: a result landing and a log line arriving change the words on the run's
// rows and never the rows themselves, so the table, the keys and the frame stay
// where they are while the run reports.
func TestTheRunCardHoldsItsRowsWhileEventsArrive(t *testing.T) {
	base := updateCenterRunFrameCase(t)

	// The run's column is the right one; every card in it is measured from the
	// top row of its box to the bottom row of its box, so a card that grew or
	// shrank shows up as a moved edge, and so does a card that moved.
	shape := func(m Model) (spans [][2]int, top, bottom int) {
		t.Helper()
		rows := updateCenterFrameRowsOf(m)
		rule, footer := runBodyEnds(rows)
		if rule < 0 || footer < 0 {
			t.Fatalf("the frame does not draw its own rule and footer:\n%s", strings.Join(rows, "\n"))
		}
		body := rows[rule+1 : footer]
		columns := cardColumns(body)
		if len(columns) != 2 {
			t.Fatalf("the running screen draws %d card columns, want the table and the run:\n%s",
				len(columns), strings.Join(rows, "\n"))
		}
		spans = cardSpansInColumn(body, columns[1])
		if len(spans) < 2 {
			t.Fatalf("the run's column draws %d cards, want the progress card and the keys:\n%s",
				len(spans), strings.Join(rows, "\n"))
		}
		top = -1
		for i, row := range body {
			if strings.TrimSpace(row) == "" {
				continue
			}
			if top < 0 {
				top = i
			}
			bottom = i
		}
		return spans, top, bottom
	}

	wantSpans, wantTop, wantBottom := shape(base)
	wantRows := len(updateCenterFrameRowsOf(base))

	// One command finishes -- the one in flight -- and the next one starts.
	next := base
	done := updateCenterApplyDoneMsg{index: 1, result: updates.ApplyResult{
		Plan:   next.UpdateCenter.Plans[1],
		Status: updates.StatusCurrent,
		Outcomes: []updates.Outcome{
			{Item: next.UpdateCenter.Plans[1].Items[0], Step: updates.StepConfirmed, Detail: "the manager no longer reports this as out of date"},
		},
	}}
	updated, _ := next.Update(done)
	next = updated.(Model)

	// And the manager prints while it works.
	for i := 0; i < 6; i++ {
		updated, _ = next.Update(updateCenterLineMsg(fmt.Sprintf("Unpacking line %d…", i)))
		next = updated.(Model)
	}

	if got := len(updateCenterFrameRowsOf(next)); got != wantRows {
		t.Errorf("the frame draws %d rows after the events and %d before them: the composition moved", got, wantRows)
	}
	gotSpans, gotTop, gotBottom := shape(next)
	if gotTop != wantTop || gotBottom != wantBottom {
		t.Errorf("the composition spans rows %d-%d after the events and %d-%d before them",
			gotTop, gotBottom, wantTop, wantBottom)
	}
	if len(gotSpans) != len(wantSpans) {
		t.Fatalf("the run's column draws %d cards after the events and %d before them", len(gotSpans), len(wantSpans))
	}
	for i := range wantSpans {
		if gotSpans[i] != wantSpans[i] {
			t.Errorf("card %d spans rows %v after the events and %v before them: the card moved",
				i, gotSpans[i], wantSpans[i])
		}
	}

	// The words did change: the second command finished and the live output is
	// the line the manager printed last, not the one it printed first.
	plain := ansiEscape.ReplaceAllString(next.View(), "")
	if !strings.Contains(plain, "snap (system) — 1 confirmed") {
		t.Errorf("the finished command's row does not report what it did:\n%s", plain)
	}
	if !strings.Contains(plain, "Unpacking line 5…") {
		t.Errorf("the output block does not show the last line the manager printed:\n%s", plain)
	}
	if !strings.Contains(plain, "step 3 of 3") {
		t.Errorf("the card's caption does not follow the command in flight:\n%s", plain)
	}
}

// TestTheFloorShowsTheRunProgress is the guard for the 80x24 fallback: the same
// facts, in the four rows the section's detail band holds there, led by the bar
// rather than by the sentence that used to spend them.
func TestTheFloorShowsTheRunProgress(t *testing.T) {
	m := updateCenterRunFrameCase(t)
	m.Width, m.Height = 80, 24

	view := m.View()
	assertScreenFitsTerminal(t, "update-center-running/80x24", m.Width, m.Height, view)
	plain := ansiEscape.ReplaceAllString(view, "")

	// The band holds four rows, and the first three are the run's: the bar and its
	// count, the command in flight, and the entries it is moving.
	lines := updateCenterFrameRowsOf(m)
	at := -1
	for i, line := range lines {
		if strings.Contains(line, "entries") && strings.ContainsAny(line, "▓░") {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatalf("the floor draws no bar for the run:\n%s", plain)
	}
	band := strings.Join(lines[at:min(at+4, len(lines))], "\n")
	for _, want := range []string{
		fmt.Sprintf("%d/%d entries", updateRunFixtureDone, updateRunFixtureTotal),
		"Step 2 of 3 · snap (system)",
		"Moving core22",
		"openssl — sudo refused",
	} {
		if !strings.Contains(band, want) {
			t.Errorf("the floor's four rows do not draw %q:\n%s", want, band)
		}
	}
	if strings.Contains(plain, "An update is running. Esc does not leave") {
		t.Errorf("the floor still spends its band on the sentence the chrome already states:\n%s", plain)
	}
}

// TestFailuresStayReadableAfterTheRun is the guard for the report: a failure is
// drawn where it happened, and it is still on the screen once the sequence has
// ended -- named with the manager's own reason, not folded into a count.
func TestFailuresStayReadableAfterTheRun(t *testing.T) {
	m := updateCenterRunFrameCase(t)

	// The command in flight finishes and the sequence ends: the run's report is
	// what the screen keeps.
	updated, _ := m.Update(updateCenterApplyDoneMsg{index: 1, result: updates.ApplyResult{
		Plan:   m.UpdateCenter.Plans[1],
		Status: updates.StatusCurrent,
		Outcomes: []updates.Outcome{
			{Item: m.UpdateCenter.Plans[1].Items[0], Step: updates.StepConfirmed, Detail: "the manager no longer reports this as out of date"},
		},
	}})
	ended := updated.(Model)
	ended.UpdateCenter.InFlight = false
	updated, _ = ended.Update(updateCenterApplyDoneMsg{index: 2, result: updates.ApplyResult{
		Plan:   ended.UpdateCenter.Plans[2],
		Status: updates.StatusFailed,
		Reason: "snapd is not running",
		Outcomes: []updates.Outcome{
			{Item: ended.UpdateCenter.Plans[2].Items[0], Step: updates.StepFailed, Detail: "snapd is not running"},
		},
	}})
	ended = updated.(Model)
	ended.UpdateCenter.InFlight = false

	if ended.UpdateCenter.Applying {
		t.Fatal("the sequence is still marked as applying after its last command finished")
	}

	plain := ansiEscape.ReplaceAllString(ended.View(), "")
	for _, want := range []string{"openssl", "sudo refused", "firefox", "snapd is not running", "Finished: 2 confirmed, 2 not moved"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the ended run's report does not draw %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Installing…") {
		t.Errorf("the frame still says it is installing after the run ended:\n%s", plain)
	}

	// The floor reads the same report: the entries that did not move lead the
	// prose, so the failure is the first thing the 80x24 band says.
	floor := ended
	floor.Width, floor.Height = 80, 24
	body := strings.Join(floor.updateCenterDescription(), " ")
	for _, want := range []string{"openssl", "sudo refused", "firefox", "snapd is not running"} {
		if !strings.Contains(body, want) {
			t.Errorf("the floor's prose does not keep %q after the run: %q", want, body)
		}
	}
	assertScreenFitsTerminal(t, "update-center-report/80x24", floor.Width, floor.Height, floor.View())
}

// TestTheRunSurfaceOnlyAppearsWhileThereIsARun pins the boundary: the settled
// screen the update center always drew is untouched, so a screen that has run
// nothing keeps its table, its detail card and its keys -- and the confirmation
// that asks about a new plan hides the last run's report instead of answering a
// question nobody asked.
func TestTheRunSurfaceOnlyAppearsWhileThereIsARun(t *testing.T) {
	settled := updateCenterFrameCase(t)
	settled.Width, settled.Height = 160, 50
	settled.Animating = false
	plain := ansiEscape.ReplaceAllString(settled.View(), "")
	if strings.Contains(plain, "Progress") || strings.Contains(plain, "Not moved") {
		t.Errorf("a screen with no run draws the run's surface:\n%s", plain)
	}
	if !strings.Contains(plain, "Read with") {
		t.Errorf("a screen with no run lost the entry's card:\n%s", plain)
	}

	running := updateCenterRunFrameCase(t)
	running.UpdateCenter.Applying = false
	running.UpdateCenter.Confirming = true
	plain = ansiEscape.ReplaceAllString(running.View(), "")
	if strings.Contains(plain, "Not moved") {
		t.Errorf("the confirmation draws the last run's failure report:\n%s", plain)
	}
}

// TestTheRunCardDrawsTheSequenceInTheEnginesOrder is the guard for the checklist
// itself: every command of the sequence is a row, in the order the engine runs
// them, and the command in flight is the one the card's caption names.
func TestTheRunCardDrawsTheSequenceInTheEnginesOrder(t *testing.T) {
	m := updateCenterRunFrameCase(t)
	rows := m.updateCenterRunStepRows(cardContentWidth(84))

	if len(rows) != len(m.UpdateCenter.Plans) {
		t.Fatalf("the checklist draws %d rows for %d commands", len(rows), len(m.UpdateCenter.Plans))
	}
	for i, row := range rows {
		plain := ansiEscape.ReplaceAllString(row, "")
		manager := string(m.UpdateCenter.Plans[i].Manager)
		if !strings.Contains(plain, manager+" (") {
			t.Errorf("row %d draws %q, want the command's own manager %q", i, plain, manager)
		}
	}
	if plain := ansiEscape.ReplaceAllString(rows[1], ""); !strings.HasPrefix(plain, "▸") {
		t.Errorf("the command in flight is not the marked row: %q", plain)
	}
	// The finished command moved one entry and refused the other, so its mark is
	// the refusal it carries rather than a clean tick: the glyph says what the row
	// says, and a partially refused command is not drawn as a success.
	if plain := ansiEscape.ReplaceAllString(rows[0], ""); !strings.HasPrefix(plain, "✗") {
		t.Errorf("the command that refused an entry is not marked as a failure: %q", plain)
	}
	clean := m
	clean.UpdateCenter.Applied = []updates.ApplyResult{{Plan: clean.UpdateCenter.Plans[0], Outcomes: []updates.Outcome{
		{Item: clean.UpdateCenter.Plans[0].Items[0], Step: updates.StepConfirmed},
		{Item: clean.UpdateCenter.Plans[0].Items[1], Step: updates.StepConfirmed},
	}}}
	if plain := ansiEscape.ReplaceAllString(clean.updateCenterRunStepRows(cardContentWidth(84))[0], ""); !strings.HasPrefix(plain, "✓") {
		t.Errorf("a command that moved every entry it carried is not marked as done: %q", plain)
	}
	if plain := ansiEscape.ReplaceAllString(rows[2], ""); !strings.HasPrefix(plain, "○") {
		t.Errorf("the command behind it is not marked as not started: %q", plain)
	}

	// A sequence longer than the card's window scrolls inside it and says how many
	// rows it hid: the card never grows past the rows it reserved.
	m.UpdateCenter.Plans = append(m.UpdateCenter.Plans, make([]updates.Plan, 10)...)
	for i := 3; i < len(m.UpdateCenter.Plans); i++ {
		m.UpdateCenter.Plans[i] = updates.Plan{
			Manager: updates.ManagerBrew,
			Scope:   updates.ScopeUser,
			Command: updates.Command{Argv: []string{"brew", "upgrade", fmt.Sprintf("formula-%d", i)}},
			Items:   []updates.Item{{Manager: updates.ManagerBrew, Scope: updates.ScopeUser, Name: fmt.Sprintf("formula-%d", i)}},
		}
	}
	rows = m.updateCenterRunStepRows(cardContentWidth(84))
	if len(rows) != updateRunStepRows {
		t.Errorf("a windowed checklist draws %d rows, want its reserved %d", len(rows), updateRunStepRows)
	}
	joined := ansiEscape.ReplaceAllString(strings.Join(rows, "\n"), "")
	if !strings.Contains(joined, "more below") {
		t.Errorf("the window does not say how many commands it hid:\n%s", joined)
	}
	if !strings.Contains(joined, "snap (system)") {
		t.Errorf("the window scrolled away from the command in flight:\n%s", joined)
	}
}

// updateCenterRunKey is a key press for the guards that drive the screen.
func updateCenterRunKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// TestTheRunningScreenCannotBeAskedToRunSomethingElse pins the keys while the
// surface is on screen: the run is stopped or waited on, and no press starts a
// second sequence over the one in flight.
func TestTheRunningScreenCannotBeAskedToRunSomethingElse(t *testing.T) {
	m := updateCenterRunFrameCase(t)
	m.UpdateCenter.ctx, m.UpdateCenter.cancel = context.WithCancel(context.Background())

	before := len(m.UpdateCenter.Applied)
	for _, key := range []rune{'r', 'a', 'c', 's', 'y', ' '} {
		next, cmd := m.Update(updateCenterRunKey(key))
		m = next.(Model)
		if cmd != nil {
			t.Errorf("pressing %q during the run issued a command", string(key))
		}
		if len(m.UpdateCenter.Applied) != before {
			t.Errorf("pressing %q changed the run's report while it is in flight", string(key))
		}
	}
	if !m.UpdateCenter.Applying {
		t.Fatal("a key ended the run before its command answered")
	}
}
