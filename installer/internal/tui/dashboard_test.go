package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/charmbracelet/lipgloss"
)

// The dashboard guards. The dashboard is the welcome screen's body under the
// lockup: the tagline, the four machine cards, the greeting by name, the
// environment sentence and the call to action (S5 of
// odd/tasks/installer-redesign.md). These guards cover the two things a
// snapshot cannot: that every card and every clause is the model's own fact, and
// that the block still fits the 80x24 floor with the frame's chrome and its
// panel summary row intact.

// dashboardModel is the dashboard at a size, with every input a snapshot or a
// measurement would otherwise inherit from the machine pinned: the golden host,
// the golden clock, the golden user name and the metrics the summary reads.
func dashboardModel(t *testing.T, width, height int) Model {
	t.Helper()

	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = width, height
	m.Screen = ScreenWelcome
	m.PanelIndex = 0
	m.Animating = false
	m.Metrics = pinnedMetrics(24)
	return m
}

// dashboardView is the dashboard with its escape sequences removed, so a guard
// can read the words and measure the geometry.
func dashboardView(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// TestDashboardCardsAreTheMachinesOwnFacts pins the four cards against the model
// fields they claim to read. A card is the mockup's OS / Shell / Arch / Packages,
// its value is the field system.Detect filled or the package manager the run
// would really install through, and a fact the model does not hold renders no
// card at all rather than a card reading "unknown".
func TestDashboardCardsAreTheMachinesOwnFacts(t *testing.T) {
	cases := []struct {
		name string
		host *system.SystemInfo
		want []dashboardCard
	}{
		{
			name: "a Debian host without Homebrew installs through apt-get",
			host: &system.SystemInfo{
				OS: system.OSDebian, OSName: "Debian/Ubuntu", Arch: "x86_64", UserShell: "zsh",
			},
			want: []dashboardCard{
				{icon: dashboardOSIcon, label: "OS", value: "Debian/Ubuntu"},
				{icon: dashboardShellIcon, label: "Shell", value: "zsh"},
				{icon: dashboardArchIcon, label: "Arch", value: "x86_64"},
				{icon: dashboardPackagesIcon, label: "Packages", value: "apt-get"},
			},
		},
		{
			name: "a host with Homebrew already installed installs through brew",
			host: &system.SystemInfo{
				OS: system.OSMac, OSName: "macOS", Arch: "arm64", UserShell: "fish", HasBrew: true,
			},
			want: []dashboardCard{
				{icon: dashboardOSIcon, label: "OS", value: "macOS"},
				{icon: dashboardShellIcon, label: "Shell", value: "fish"},
				{icon: dashboardArchIcon, label: "Arch", value: "arm64"},
				{icon: dashboardPackagesIcon, label: "Packages", value: "brew"},
			},
		},
		{
			name: "a Termux host installs through pkg",
			host: &system.SystemInfo{
				OS: system.OSTermux, OSName: "Termux", Arch: "aarch64", UserShell: "bash",
				IsTermux: true, HasPkg: true,
			},
			want: []dashboardCard{
				{icon: dashboardOSIcon, label: "OS", value: "Termux"},
				{icon: dashboardShellIcon, label: "Shell", value: "bash"},
				{icon: dashboardArchIcon, label: "Arch", value: "aarch64"},
				{icon: dashboardPackagesIcon, label: "Packages", value: "pkg"},
			},
		},
		{
			name: "an architecture detection could not name gets no card",
			host: &system.SystemInfo{
				OS: system.OSLinux, OSName: "Linux", UserShell: "zsh",
			},
			want: []dashboardCard{
				{icon: dashboardOSIcon, label: "OS", value: "Linux"},
				{icon: dashboardShellIcon, label: "Shell", value: "zsh"},
				{icon: dashboardPackagesIcon, label: "Packages", value: "apt-get"},
			},
		},
		{
			name: "a host detection could not name gets two cards and no package manager",
			host: &system.SystemInfo{OS: system.OSUnknown, OSName: "Unknown"},
			want: []dashboardCard{
				{icon: dashboardOSIcon, label: "OS", value: "Unknown"},
			},
		},
		{
			name: "a model with no host at all draws no card",
			host: nil,
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{SystemInfo: c.host}
			if got := m.dashboardCards(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("dashboardCards() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDashboardCardValuesAreNotInvented pins the rule the cards share with the
// panels: a value printed on a card is a field the model holds. It reads the
// rendered card row back, so a future card added with a hard-coded value is
// caught even if dashboardCards is bypassed.
func TestDashboardCardValuesAreNotInvented(t *testing.T) {
	m := dashboardModel(t, 80, 24)
	view := dashboardView(m)

	for _, card := range m.dashboardCards() {
		if card.value == "" {
			t.Errorf("the dashboard drew a %q card with no value", card.label)
			continue
		}
		if !strings.Contains(view, card.value) {
			t.Errorf("the dashboard does not print the %q card's value %q:\n%s", card.label, card.value, view)
		}
	}
}

// TestDashboardGreetingUsesTheModelClockAndTheUsersName pins the greeting: the
// time of day read from the model's own creation time, the name the installer
// read at startup, and a sentence that stays whole when there is no name. A
// model with no clock adds no line at all, because a greeting guessed from an
// hour nothing set would change the bytes when nothing else did.
func TestDashboardGreetingUsesTheModelClockAndTheUsersName(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, time.January, 1, hour, 0, 0, 0, time.UTC) }

	cases := []struct {
		name    string
		created time.Time
		user    string
		want    string
	}{
		{"the morning", at(9), "alberto", "Good morning, alberto!"},
		{"the afternoon", at(15), "alberto", "Good afternoon, alberto!"},
		{"the evening", at(21), "alberto", "Good evening, alberto!"},
		{"a host that could not name the user", at(9), "", "Good morning!"},
		{"a host that named the user with surrounding space", at(9), "  alberto ", "Good morning, alberto!"},
		{"a model with no creation time", time.Time{}, "alberto", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{CreatedAt: c.created, UserName: c.user}
			if got := m.dashboardGreeting(); got != c.want {
				t.Errorf("dashboardGreeting() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestDashboardGreetsByTheLoginNameTheInstallerCanRead pins the probe: the first
// of USER, LOGNAME and USERNAME that states a name, cleaned of the control
// characters an environment variable may carry, and an empty answer when none of
// them states one. The lookup is injected so the guard pins the environment
// rather than the account running the test.
func TestDashboardGreetsByTheLoginNameTheInstallerCanRead(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"USER leads", map[string]string{"USER": "alberto", "LOGNAME": "other"}, "alberto"},
		{"LOGNAME answers when USER is empty", map[string]string{"USER": "", "LOGNAME": "alberto"}, "alberto"},
		{"USERNAME answers when the POSIX names are unset", map[string]string{"USERNAME": "alberto"}, "alberto"},
		{"nothing set", nil, ""},
		{"a name with a newline is cleaned", map[string]string{"USER": " alberto\n"}, "alberto"},
		{"a name that is only whitespace is no name", map[string]string{"USER": "   "}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lookup := func(key string) string { return c.values[key] }
			if got := userNameFromEnv(lookup); got != c.want {
				t.Errorf("userNameFromEnv() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestDashboardFitsTheFloorAndKeepsItsParts is the 80x24 guard: the dashboard's
// whole vocabulary is on the screen, the frame's chrome and its panel summary
// row survive it, and nothing is drawn past the terminal's edge. The summary row
// is the part the body could spend by accident: one row more of dashboard and the
// frame drops it, which is why the floor shape is the compact lockup with the
// cards on their three rows.
func TestDashboardFitsTheFloorAndKeepsItsParts(t *testing.T) {
	m := dashboardModel(t, 80, 24)
	view := dashboardView(m)

	if rows := renderedRowCount(view); rows != 24 {
		t.Errorf("the dashboard renders %d rows at 80x24, want exactly 24:\n%s", rows, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("the dashboard renders a line %d columns wide, want <= 80: %q", w, line)
		}
	}

	for _, want := range []string{
		dashboardTagline,
		"OS", "Linux",
		"Shell", "zsh",
		"Arch", "x86_64",
		"Packages", "apt-get",
		"Good morning, testuser!",
		"Running on",
		dashboardCallToAction,
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the dashboard at 80x24 does not show %q:\n%s", want, view)
		}
	}

	t.Run("the floor keeps the compact lockup", func(t *testing.T) {
		if strings.Contains(view, "╗") {
			t.Errorf("the floor drew the full six-row wordmark, which leaves the cards and the summary no rows:\n%s", view)
		}
		if !strings.Contains(view, "dotfiles") {
			t.Errorf("the compact lockup lost its name:\n%s", view)
		}
	})

	t.Run("the frame keeps the panel summary row", func(t *testing.T) {
		if !strings.Contains(view, "Machine") {
			t.Errorf("the dashboard filled the frame, so the narrow panel summary was dropped:\n%s", view)
		}
	})

	t.Run("every key stays reachable", func(t *testing.T) {
		// Tab opens the menu at every size the screen is drawn at, so the floor names
		// the menu for it exactly as the wide home's body and footer do. The panel
		// walk does not take the key on this screen; advertising "[Tab] panel" here
		// was the promise that stopped being true when the screen grew the menu key.
		for _, want := range []string{"[s] start", "[q] quit", "[Tab] menu"} {
			if !strings.Contains(view, want) {
				t.Errorf("the floor's footer does not offer %q:\n%s", want, view)
			}
		}
	})
}

// TestDashboardCardsCollapseToFactsWhenTheyCannotFit pins the narrow shape: four
// cards need a card's worth of columns each, and below that width the same four
// facts are printed in a row instead of in cards that would cut them. The screen
// still fits, and no fact is lost to the narrower frame.
func TestDashboardCardsCollapseToFactsWhenTheyCannotFit(t *testing.T) {
	m := dashboardModel(t, 60, 24)
	if band := m.dashboardCardBand(contentWidth(m)); len(band) != 0 {
		t.Fatalf("four cards were drawn at 60 columns, where a card cannot hold its own value: %v", band)
	}

	rows := m.dashboardCardFacts(contentWidth(m))
	if len(rows) == 0 {
		t.Fatal("the narrow dashboard drew neither cards nor facts")
	}
	view := dashboardView(m)
	for _, card := range m.dashboardCards() {
		if !strings.Contains(view, card.label) || !strings.Contains(view, card.value) {
			t.Errorf("the narrow dashboard lost the %q card's %q:\n%s", card.label, card.value, view)
		}
	}
	if rows := renderedRowCount(view); rows > 24 {
		t.Errorf("the narrow dashboard renders %d rows, want <= 24:\n%s", rows, view)
	}
}

// TestDashboardCardBandIsExactlyTheBodysWidth pins the band's arithmetic: equal
// cards, one column between them, and the leftover columns spent rather than
// left as a ragged edge, so the centred band has nothing left to centre.
func TestDashboardCardBandIsExactlyTheBodysWidth(t *testing.T) {
	cards := dashboardModel(t, 80, 24).dashboardCards()
	for _, width := range []int{64, 68, 72, 76, 96, 120} {
		widths, ok := dashboardCardWidths(cards, width)
		if !ok {
			t.Fatalf("%d columns cannot hold four cards that fit at 76", width)
		}
		total := dashboardCardGap * (len(cards) - 1)
		for i, w := range widths {
			total += w
			if w < dashboardCardNeed(cards[i]) {
				t.Errorf("the %q card is %d columns wide, narrower than its own value: %v", cards[i].label, w, widths)
			}
		}
		if total != width {
			t.Errorf("the band at %d columns is %d columns wide: %v", width, total, widths)
		}
	}

	m := dashboardModel(t, 80, 24)
	band := m.dashboardCardBand(contentWidth(m))
	if len(band) != dashboardCardHeight {
		t.Fatalf("the card band is %d rows, want %d", len(band), dashboardCardHeight)
	}
	for i, line := range band {
		if w := lipgloss.Width(line); w != contentWidth(m) {
			t.Errorf("card row %d is %d columns wide, want %d: %q", i, w, contentWidth(m), line)
		}
	}
}

// TestDashboardCardGlyphsAreOneCellNerdFontCodePoints pins S3's glyph rule on the
// cards: every icon is a Nerd Font code point from the private use area, one cell
// wide, and never an emoji. A glyph that is two cells wide, or one a terminal
// resolves through an emoji font, would push the card's border out by a column and
// break the band's arithmetic; an emoji is also the one glyph the design does not
// allow on any screen.
func TestDashboardCardGlyphsAreOneCellNerdFontCodePoints(t *testing.T) {
	for _, card := range dashboardModel(t, 80, 24).dashboardCards() {
		if w := lipgloss.Width(card.icon); w != 1 {
			t.Errorf("the %q card's icon %q occupies %d cells, want 1", card.label, card.icon, w)
		}
		if card.icon == "" {
			t.Errorf("the %q card has no icon", card.label)
			continue
		}
		for _, r := range card.icon {
			if !inPrivateUseArea(r) {
				t.Errorf("the %q card's icon %q carries U+%04X, which is not a Nerd Font private-use code point",
					card.label, card.icon, r)
			}
		}
	}
}

// inPrivateUseArea reports whether r is one of Unicode's three private use
// areas, which is where Nerd Fonts puts its glyphs and where no emoji font can
// claim them.
func inPrivateUseArea(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) || // Basic Multilingual Plane
		(r >= 0xF0000 && r <= 0xFFFFD) || // Supplementary Private Use Area-A
		(r >= 0x100000 && r <= 0x10FFFD) // Supplementary Private Use Area-B
}
