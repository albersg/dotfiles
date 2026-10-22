package tui

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The visual pass put the build label on every frame's header, gave the hubs a
// card and a contextual panel, and made the Vim hub's advertised Escape work.
// These guards pin those three rules where a golden cannot: a golden says the
// screen looked a certain way once, these say why it looks that way.

func TestFrameVitalCarriesTheBuildOnEveryPage(t *testing.T) {
	label := VersionLabel()

	if got := frameVital(""); got != label {
		t.Errorf("a page with no vital = %q, want the build %q", got, label)
	}
	got := frameVital("Showing 1–2 of 5")
	if !strings.HasSuffix(got, label) {
		t.Errorf("a page with a vital = %q, want it to end with the build %q", got, label)
	}
	if !strings.Contains(got, "Showing 1–2 of 5") {
		t.Errorf("frameVital dropped the screen's own vital: %q", got)
	}
	if dup := frameVital(label); strings.Count(dup, label) != 1 {
		t.Errorf("a page that already names the build repeats it: %q", dup)
	}
}

func TestVimHubEscapeReturnsToTheMainMenu(t *testing.T) {
	m := Model{Screen: ScreenVimHub, Width: 120, Height: 36}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got.Screen != ScreenMainMenu {
		t.Fatalf("Escape on the Vim hub left it on screen %v, want the main menu", got.Screen)
	}
	if got.Cursor != 0 {
		t.Errorf("Escape left the main menu's cursor at %d, want 0", got.Cursor)
	}
}

func TestHubPageDrawsTheCardAndItsPanel(t *testing.T) {
	m := Model{Screen: ScreenVimHub, Width: 160, Height: 48, Cursor: 0}
	view := m.View()

	for _, want := range []string{"╭", "╰", "About", "Selected", "Vim Trainer", VersionLabel()} {
		if !strings.Contains(view, want) {
			t.Errorf("the Vim hub does not draw %q:\n%s", want, view)
		}
	}
}

// TestHubCardStaysWhereItIsWhenTheCursorMoves is the stability rule applied to
// the hub pages: the card's borders and the panel's chip keep their rows while
// the cursor walks the list. The row under the cursor changes; the frame does
// not.
func TestHubCardStaysWhereItIsWhenTheCursorMoves(t *testing.T) {
	bordersAt := func(cursor int) (tops, bottoms []int) {
		m := Model{Screen: ScreenVimHub, Width: 160, Height: 48, Cursor: cursor}
		lines := strings.Split(m.View(), "\n")
		for i, line := range lines {
			if strings.Contains(line, "╭") {
				tops = append(tops, i)
			}
			if strings.Contains(line, "╰") {
				bottoms = append(bottoms, i)
			}
		}
		return tops, bottoms
	}

	firstTops, firstBottoms := bordersAt(0)
	lastTops, lastBottoms := bordersAt(3)
	// The screen draws two cards: the hub's own and the panel column beside it,
	// which is the same grammar's card. Their top borders share a row now that the
	// body is a card and no longer spends a title row above it, so the guard asks
	// for a top and a bottom rather than for matched pairs: what it holds is that a
	// card is closed and that no border moves with the cursor.
	if len(firstTops) == 0 || len(firstBottoms) == 0 {
		t.Fatalf("the hub draws no closed card: tops on %v, bottoms on %v", firstTops, firstBottoms)
	}
	if !reflect.DeepEqual(firstTops, lastTops) || !reflect.DeepEqual(firstBottoms, lastBottoms) {
		t.Errorf("the card moved with the cursor: tops %v/%v, bottoms %v/%v",
			firstTops, lastTops, firstBottoms, lastBottoms)
	}
}

// TestLearnHubGroupsTheCardByWhatTheRunDoes pins the two named sections: the
// hub's card is not a flat list any more, and the divider says which question
// each half answers.
func TestLearnHubGroupsTheCardByWhatTheRunDoes(t *testing.T) {
	m := Model{Screen: ScreenLearnTools, Width: 160, Height: 48}
	view := m.View()
	for _, want := range []string{"What the run chooses", "What the run installs"} {
		if !strings.Contains(view, want) {
			t.Errorf("the Learn hub card does not carry %q:\n%s", want, view)
		}
	}
}

// TestHubPanelFactsReadTheRowsTheyBelongTo keeps the panel honest: the Vim hub's
// trainer row must count the curriculum the trainer screen ships, and the
// keymaps row must count the bindings the reference ships, not a number written
// down a second time.
func TestHubPanelFactsReadTheRowsTheyBelongTo(t *testing.T) {
	l := layoutFor(Model{Width: 160, Height: 48})
	modules, _, _ := trainerCurriculum()

	cases := []struct {
		label string
		want  string
	}{
		{"Vim Trainer", strconv.Itoa(modules)},
		{"Neovim Keymaps", keymapBindingCount(totalKeymapCount())},
		{"LazyVim Guide", strconv.Itoa(len(GetLazyVimTopics()))},
	}
	for _, c := range cases {
		m := Model{Screen: ScreenVimHub, Width: 160, Height: 48, Cursor: vimHubRow(t, c.label)}
		facts := strings.Join(m.hubPanelFacts(l), "\n")
		if !strings.Contains(facts, c.want) {
			t.Errorf("the %q panel does not state %q:\n%s", c.label, c.want, facts)
		}
	}
}

// vimHubRow is the index of a Vim hub option by its label. A test that address
// the row by name keeps working when a row is added above it.
func vimHubRow(t *testing.T, label string) int {
	t.Helper()
	m := Model{Screen: ScreenVimHub}
	for i, option := range m.GetCurrentOptions() {
		if option == label {
			return i
		}
	}
	t.Fatalf("the Vim hub has no row %q", label)
	return 0
}

// TestLearnHubPanelListsTheDestinationInMenuOrder guards the preview panel of
// the Learn hub: it must list the tools of the page the row opens, in the order
// the page lists them. Reading the hub's own options instead produced "Pages 0":
// the hub lists pages, not the tools inside them.
func TestLearnHubPanelListsTheDestinationInMenuOrder(t *testing.T) {
	l := layoutFor(Model{Width: 160, Height: 48})

	for _, label := range []string{"Terminals", "Shells", "Multiplexers", "Prompts"} {
		m := Model{Screen: ScreenLearnTools, Width: 160, Height: 48, Cursor: learnHubRow(t, label)}
		facts := strings.Join(m.hubPanelFacts(l), "\n")
		if strings.Contains(facts, "Pages 0") {
			t.Errorf("the %q panel previews no page: %s", label, facts)
		}
	}

	m := Model{Screen: ScreenLearnTools, Width: 160, Height: 48, Cursor: learnHubRow(t, "Terminals")}
	facts := strings.Join(m.hubPanelFacts(l), "\n")
	last := -1
	for _, name := range []string{"Alacritty", "WezTerm", "Kitty", "Ghostty"} {
		at := strings.Index(facts, name)
		if at < 0 {
			t.Fatalf("the Terminals panel does not list %q:\n%s", name, facts)
		}
		if at <= last {
			t.Errorf("the Terminals panel lists %q out of the menu's order:\n%s", name, facts)
		}
		last = at
	}
}

// learnHubRow is the index of a Learn hub option by its label.
func learnHubRow(t *testing.T, label string) int {
	t.Helper()
	m := Model{Screen: ScreenLearnTools}
	for i, option := range m.GetCurrentOptions() {
		if option == label {
			return i
		}
	}
	t.Fatalf("the Learn hub has no row %q", label)
	return 0
}

// TestLazyVimTopicFooterNamesOnlyLiveKeys pins the topic page's footer: it is
// prose, so Enter opens nothing and must not be advertised.
func TestLazyVimTopicFooterNamesOnlyLiveKeys(t *testing.T) {
	m := Model{Screen: ScreenLazyVimTopic, Width: 120, Height: 40, LazyVimTopics: GetLazyVimTopics(), SelectedLazyVimTopic: 0}
	view := m.renderLazyVimTopic()
	if strings.Contains(view, "[Enter] open") {
		t.Errorf("the topic footer advertises Enter on a page with no rows:\n%s", view)
	}
	if !strings.Contains(view, "[Esc] back") {
		t.Errorf("the topic footer does not name the way back:\n%s", view)
	}
}

// TestTrainerScrollHintOnlyWhenTheCodeScrolls pins the trainer's scroll hint:
// the code window is one line in a short exercise, and a scroll key that cannot
// move is not advertised.
func TestTrainerScrollHintOnlyWhenTheCodeScrolls(t *testing.T) {
	hasScroll := func(hints []installerHint) bool {
		for _, hint := range hints {
			if hint == trainerHintScroll {
				return true
			}
		}
		return false
	}
	m := Model{}
	if hasScroll(m.trainerExerciseHints(false)) {
		t.Error("a code window that fits still advertises the scroll key")
	}
	if !hasScroll(m.trainerExerciseHints(true)) {
		t.Error("a scrolling code window does not advertise the scroll key")
	}
}

// TestInstallReviewSpaceTogglesWithoutLeaderMode pins the review's one switch:
// Space must reach the review's handler. The screen used to fall through to the
// global leader key, so the footer's "[Space] turn it off or on" armed the leader
// and swallowed the next key; no unit could be turned off from the keyboard.
func TestInstallReviewSpaceTogglesWithoutLeaderMode(t *testing.T) {
	m := Model{Screen: ScreenInstallReview}
	m.snapUnitCursor()
	rows := m.planReviewRows()
	if m.UnitCursor < 0 || m.UnitCursor >= len(rows) || rows[m.UnitCursor].StepID == "" {
		t.Fatal("the review did not start on a unit")
	}
	id := rows[m.UnitCursor].StepID
	before := m.unitIsDisabled(id)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.LeaderMode {
		t.Fatal("Space armed leader mode on the install review")
	}
	if got.unitIsDisabled(id) == before {
		t.Errorf("Space did not toggle the unit %q", id)
	}
}

// TestInstallReviewPanelFollowsTheCursor pins the review's right column: it
// answers for the unit the cursor is on, not for the wizard's last choice.
func TestInstallReviewPanelFollowsTheCursor(t *testing.T) {
	m := Model{Screen: ScreenInstallReview, Width: 160, Height: 48}
	m.snapUnitCursor()
	l := layoutFor(m)
	first := strings.Join(m.installReviewPanelFacts(l), "\n")
	rows := m.planReviewRows()
	m.UnitCursor = len(rows) - 1
	last := strings.Join(m.installReviewPanelFacts(l), "\n")
	if first == last {
		t.Errorf("the review panel did not change with the cursor:\n%s", first)
	}
	if !strings.Contains(first, "Selected") {
		t.Errorf("the review panel has no selected row:\n%s", first)
	}
}

// TestCompletionDryRunSaysNothingWasWritten pins the dry run's completion: it
// must not claim the machine was configured or send the reader to a shell the
// rehearsal never set up.
func TestCompletionDryRunSaysNothingWasWritten(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	m := NewModel()
	m.Screen = ScreenComplete
	view := m.renderComplete()
	if !strings.Contains(view, "Dry run") {
		t.Errorf("the dry-run completion does not say it was a rehearsal:\n%s", view)
	}
	if strings.Contains(view, "Next Step") {
		t.Errorf("the dry-run completion offers a next step it did not set up:\n%s", view)
	}
	if !strings.Contains(view, "Multiplexer") || strings.Contains(view, "Window Manager") {
		t.Errorf("the completion summary still calls the multiplexer a window manager:\n%s", view)
	}
}

// TestWMDetourStartsOnNoRowAndTheFirstArrowPicksOne pins the no-answer state: a
// question whose answer was not recorded must not present a guess, but the first
// arrow press has to pick a row instead of being dead (and the footer must not
// offer Enter until there is a row to choose).
func TestWMDetourStartsOnNoRowAndTheFirstArrowPicksOne(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenSetup
	rows := m.setupRows()
	at := -1
	for i, row := range rows {
		if row.Screen == ScreenWMSelect {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatal("the setup has no multiplexer row")
	}
	m.SetupCursor = at
	m.openSetupRow()
	if m.Screen != ScreenWMSelect {
		t.Fatalf("opening the multiplexer row went to %v, not the question", m.Screen)
	}
	if m.Cursor != -1 {
		t.Errorf("the multiplexer question highlights row %d although no answer is recorded", m.Cursor)
	}
	if strings.Contains(m.View(), "[Enter] choose") {
		t.Error("the no-answer question advertises Enter choose")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := updated.(Model)
	if got.Cursor != 0 {
		t.Errorf("Up left the no-answer question on row %d, want the first", got.Cursor)
	}
}
