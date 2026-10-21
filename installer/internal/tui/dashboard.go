package tui

import (
	"os"
	"os/user"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// The dashboard is the mockup's first panel: the lockup, the tagline under it,
// four machine cards, the greeting with the user's own name, one factual
// sentence about the environment and the call to action. The lockup itself is
// drawn by view.go -- it is the splash the installer has always opened with --
// and everything below it lives here.
//
// Nothing on this screen is invented. Every value comes from the model: the host
// fields system.Detect filled, the package manager the run would really install
// through, and the model's own creation time. A fact the model does not hold is
// left out -- no card, no sentence clause, no name -- rather than filled in with
// a guess, which is the same rule the panels follow.
//
// The greeting's name is read from the account once, on the startup path (see
// currentUserName), and carried on the model: a render must not ask the account
// database anything, exactly as it must not run a probe or read a file.

const (
	// dashboardTagline is the mockup's line under the wordmark. It replaces the
	// older "Your terminal environment, configured in minutes.": the screen is a
	// dashboard now, and the tagline names the product rather than one of its
	// outcomes.
	dashboardTagline = "Your personal development environment."

	// dashboardCallToAction is the mockup's closing line, under the environment
	// sentence. It is the screen's prose ending, not a button: the key that starts
	// the run is the footer's, and a button here would offer a second one.
	dashboardCallToAction = "Let's make this machine yours."
)

// dashboardCard is one machine fact drawn as the mockup's card: an icon, the
// label naming the fact, and the value the model holds for it. The icon is
// decoration -- a terminal without the Nerd Font still reads the label and the
// value, which are the card -- and every glyph is one cell wide.
type dashboardCard struct {
	icon  string
	label string
	value string
}

const (
	// dashboardCardGap is the blank columns between two cards, so the row reads
	// as separate objects instead of one wide band.
	dashboardCardGap = 1

	// dashboardCardPad is the blank column each card keeps between its border and
	// its text, so a value never touches the edge.
	dashboardCardPad = 1

	// dashboardCardHeight is how many rows one card spends: the top border that
	// carries its label, the value, and the bottom border. The mockup gives the
	// label a row of its own inside the card; the frame's card grammar puts the
	// label on the border instead (the same shape panelBlock draws its title in),
	// and that is the one row that keeps four cards inside the 80x24 floor's body
	// budget with the summary row above the footer still showing.
	dashboardCardHeight = 3
)

// The four cards' glyphs, in the order the mockup shows them. They are Nerd Font
// code points, one cell each: a desktop for the operating system, a prompt for
// the shell, cogs for the architecture and cubes for the packages.
const (
	dashboardOSIcon       = "\uf108"
	dashboardShellIcon    = "\uf120"
	dashboardArchIcon     = "\uf085"
	dashboardPackagesIcon = "\uf1b3"
)

// dashboardCards is the machine facts the dashboard draws as cards, in the
// mockup's order: OS, Shell, Arch and Packages. A fact the model does not hold
// renders no card at all -- a host detection could not name gets three cards
// rather than a fourth card reading "unknown" -- so the row is always a
// statement about this machine and never about the screen's ambition.
func (m Model) dashboardCards() []dashboardCard {
	info := m.SystemInfo
	if info == nil {
		return nil
	}

	all := []dashboardCard{
		{icon: dashboardOSIcon, label: "OS", value: info.OSName},
		{icon: dashboardShellIcon, label: "Shell", value: info.UserShell},
		{icon: dashboardArchIcon, label: "Arch", value: archName(info)},
		{icon: dashboardPackagesIcon, label: "Packages", value: m.packageManager()},
	}

	cards := make([]dashboardCard, 0, len(all))
	for _, card := range all {
		if strings.TrimSpace(card.value) == "" {
			continue
		}
		cards = append(cards, card)
	}
	return cards
}

// dashboardCardBand draws the cards side by side across the body's width, or
// returns nil when they cannot be drawn without cutting one of them. It is
// three rows: every card's top border, every card's value, every card's bottom
// border, joined by the gap. The band is exactly as wide as the body it was
// measured against, so the caller's centring has nothing left to do.
func (m Model) dashboardCardBand(width int) []string {
	cards := m.dashboardCards()
	widths, ok := dashboardCardWidths(cards, width)
	if !ok {
		return nil
	}

	rows := make([]string, dashboardCardHeight)
	for i, card := range cards {
		lines := dashboardCardLines(card, widths[i])
		for r, line := range lines {
			if rows[r] == "" {
				rows[r] = line
				continue
			}
			rows[r] += strings.Repeat(" ", dashboardCardGap) + line
		}
	}
	return rows
}

// dashboardCardLines draws one card in exactly dashboardCardHeight rows: the top
// border with its icon and label, the value row, and the closing border. Every
// row is the card's own width, so the band's columns line up. The width comes
// from dashboardCardWidths, which never hands a card less than its own need, so
// nothing here has to cut a label or a value.
func dashboardCardLines(card dashboardCard, width int) []string {
	edge := func(s string) string { return PanelBorderStyle.Render(s) }

	inner := width - 2
	label := card.icon + " " + card.label
	dash := inner - 1 - lipgloss.Width(label)
	if dash < 0 {
		dash = 0
	}
	// A space before the rule whenever the card has one to spend, so the label
	// reads as a title on the border rather than as text jammed against the
	// corner. At the narrowest card the space is the column the border needs.
	gap := ""
	if dash > 0 {
		gap = " "
		dash--
	}

	top := edge("╭─") +
		AccentKeyStyle.Render(card.icon) + " " + MutedStyle.Render(card.label) + gap +
		edge(strings.Repeat("─", dash)+"╮")

	value := lipgloss.NewStyle().
		MaxWidth(inner - 2*dashboardCardPad).
		Render(InkStyle.Render(card.value))
	mid := edge("│") +
		strings.Repeat(" ", dashboardCardPad) + padRight(value, inner-2*dashboardCardPad) +
		strings.Repeat(" ", dashboardCardPad) + edge("│")

	bottom := edge("╰" + strings.Repeat("─", inner) + "╯")

	return []string{top, mid, bottom}
}

// dashboardCardWidths divides the body's columns among the cards. The cards are
// equal width, as the mockup draws them, and the columns the division cannot
// spend evenly go one each to the leading cards, so the band is exactly width
// columns wide. It reports false when the division would leave a card narrower
// than its own label or value, which is the caller's signal to say the facts in
// a row instead of drawing cards that cannot hold them.
func dashboardCardWidths(cards []dashboardCard, width int) ([]int, bool) {
	if len(cards) == 0 || width < 1 {
		return nil, false
	}

	room := width - dashboardCardGap*(len(cards)-1)
	each := room / len(cards)

	need := 0
	for _, card := range cards {
		need = max(need, dashboardCardNeed(card))
	}
	if each < need {
		return nil, false
	}

	widths := make([]int, len(cards))
	for i := range widths {
		widths[i] = each
	}
	for i, extra := 0, room-each*len(cards); i < extra; i++ {
		widths[i%len(cards)]++
	}
	return widths, true
}

// dashboardCardNeed is the narrowest a card can be drawn without cutting it: the
// two borders, the padding on each side, and the wider of its label line (the
// icon, a space and the label, sitting on the top border after the corner) and
// its value. A card is never narrower than its own value, because a value is the
// fact and a cut value is the screen saying less.
func dashboardCardNeed(card dashboardCard) int {
	label := card.icon + " " + card.label
	return max(lipgloss.Width(label), lipgloss.Width(card.value)) + 2*dashboardCardPad + 2
}

// dashboardCardFacts is the cards said as facts in one row instead of side by
// side. It is the narrow frame's shape: below the width where the cards fit, the
// row is the same facts with their labels, wrapped to the body rather than cut to
// it. The frame's card grammar needs a card's worth of columns per fact, and
// giving each a fifth of a narrow screen would either cut a value or spend the
// rows one card per line costs -- rows the 80x24 floor does not have. Two rows is
// the cap, because a frame that narrow has no row to spare; past it the row ends
// with the cut marker rather than growing the screen.
func (m Model) dashboardCardFacts(width int) []string {
	cards := m.dashboardCards()
	if len(cards) == 0 {
		return nil
	}

	parts := make([]string, len(cards))
	for i, card := range cards {
		parts[i] = card.label + " " + card.value
	}
	rows := wrapText(strings.Join(parts, " · "), width, 2)
	for i := range rows {
		rows[i] = InkStyle.Render(rows[i])
	}
	return rows
}

// dashboardCardsRows is the card block at this width: the band where four cards
// fit, and the facts in a row where they do not. One function asks, so a caller
// cannot draw the band at a width the band does not fit.
func (m Model) dashboardCardsRows(width int) []string {
	if band := m.dashboardCardBand(width); len(band) > 0 {
		return band
	}
	return m.dashboardCardFacts(width)
}

// dashboardGreeting is the dashboard's greeting: the time of day read from the
// model's own creation time and the name read from the account at startup. A
// model with neither still greets -- "Good morning!" is a complete sentence --
// and a model with no creation time greets not at all, so a run that pinned no
// clock adds no line rather than guessing an hour.
func (m Model) dashboardGreeting() string {
	word := greetingFor(m.CreatedAt)
	if word == "" {
		return ""
	}
	if name := strings.TrimSpace(m.UserName); name != "" {
		return word + ", " + name + "!"
	}
	return word + "!"
}

// environmentSentence is the dashboard's one factual sentence, in the mockup's
// shape: what this machine is, the environment it runs inside, whether Homebrew
// is already there, and the build the screen came from. Every clause is read
// from the model, so a host detection could not name simply gets a shorter
// sentence.
func (m Model) environmentSentence() string {
	if m.SystemInfo == nil {
		return ""
	}

	env := "Running on " + m.SystemInfo.OSName
	if m.SystemInfo.IsWSL && m.SystemInfo.OSName != "WSL" {
		env += " under WSL"
	}
	if m.SystemInfo.HasBrew {
		env += ", with Homebrew already installed"
	}
	return env + " (" + VersionLabel() + ")"
}

// dashboardBody is the dashboard under the lockup: the tagline, the machine
// cards, the greeting, the environment sentence and the call to action, each
// measured against the columns the body will actually occupy. It reads the model
// and draws; the frame places the block and the creature.
func (m Model) dashboardBody(width int) []string {
	body := []string{MutedStyle.Render(truncate(dashboardTagline, width))}

	body = append(body, m.dashboardCardsRows(width)...)

	if greeting := m.dashboardGreeting(); greeting != "" {
		body = append(body, BrandStyle.Render(truncate(greeting, width)))
	}

	for _, line := range wrapText(m.environmentSentence(), width, 0) {
		body = append(body, MeterStyle.Render(line))
	}
	body = append(body, SubtitleStyle.Render(truncate(dashboardCallToAction, width)))

	return body
}

// userNameEnvKeys are the environment variables a login name arrives in, in the
// order the installer trusts them: a POSIX session sets USER, an SSH session
// often carries LOGNAME instead, and Windows and WSL set USERNAME.
var userNameEnvKeys = []string{"USER", "LOGNAME", "USERNAME"}

// userNameFromEnv is the login name the environment states, or an empty string
// when it states none. It takes the lookup rather than reading os.Getenv itself
// so a guard can pin the environment without pinning the account running it.
func userNameFromEnv(getenv func(string) string) string {
	for _, key := range userNameEnvKeys {
		if name := cleanUserName(getenv(key)); name != "" {
			return name
		}
	}
	return ""
}

// currentUserName is the name the dashboard greets by: the environment's login
// name, and the account database's when the environment states none. It runs
// once on the startup path and the answer is carried on the model, so no render
// asks the environment or the account database anything. A host where neither
// answers yields an empty name, and the dashboard greets without one.
func currentUserName() string {
	if name := userNameFromEnv(os.Getenv); name != "" {
		return name
	}
	account, err := user.Current()
	if err != nil {
		return ""
	}
	return cleanUserName(account.Username)
}

// cleanUserName makes an environment-supplied name safe to print: a control
// character or a stray newline in the variable would otherwise reach the frame
// as a second row or as a terminal escape. Unicode letters are kept, so a name
// outside ASCII greets as itself. The name is the environment's, not the
// installer's, so this is the whole of what it is trusted for.
func cleanUserName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// welcomeBodyRows is the rows the welcome screen's body may spend, measured the
// way the frame measures them: the terminal minus the padding View adds, the
// header row, the two rules and the footer the screen reserves. renderWelcome
// asks it before spending the lockup's blank row, so the spacing that is the
// screen's own taste is the first thing given back when the frame is too tight
// for the whole block -- and the arithmetic stays in one place instead of being
// restated in the renderer.
func (m Model) welcomeBodyRows() int {
	hints := []installerHint{hintStart, hintQuit}
	return installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), m.panelHints(m.panelsFor(), hints)))
}
