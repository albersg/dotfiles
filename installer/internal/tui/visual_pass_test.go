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
	bordersAt := func(cursor int) []int {
		m := Model{Screen: ScreenVimHub, Width: 160, Height: 48, Cursor: cursor}
		lines := strings.Split(m.View(), "\n")
		var found []int
		for i, line := range lines {
			if strings.Contains(line, "╭") || strings.Contains(line, "╰") {
				found = append(found, i)
			}
		}
		return found
	}

	first := bordersAt(0)
	last := bordersAt(3)
	if len(first) != 2 {
		t.Fatalf("the hub card's borders are on %v rows, want a top and a bottom: %v", len(first), first)
	}
	if !reflect.DeepEqual(first, last) {
		t.Errorf("the card moved with the cursor: borders at %v with the first row, %v with the last", first, last)
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
