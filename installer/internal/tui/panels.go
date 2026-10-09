package tui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	"github.com/charmbracelet/lipgloss"
)

// A panel is a screen's second column: a chip naming it, a rule under the chip,
// and then rows of labelled facts. Each panel answers the question its screen
// asks -- the welcome screen asks "where am I", so its panel is the machine the
// run is about to change; the main menu asks "what is about to happen", so its
// panel is the plan and the state that run will read.
//
// Three rules hold for every panel, and they are why a panel is not just a block
// of text:
//
//   - A row is a label in the dim tone inside a fixed measure, then a value in
//     ink. The measure is fixed so the values line up down the column and the
//     panel reads as a table of facts. The tones are a convenience: the words
//     already say which half of the row is which, so the panel survives a
//     terminal with no colour at all.
//   - Nothing is invented. Every value comes from a field system.Detect already
//     filled or from the model's own state, and a value the model does not have
//     is left out rather than printed as "unknown".
//   - Nothing is clipped silently. A value wider than the column wraps under
//     itself, and a panel with more rows than the frame leaves says how many it
//     could not show.
//
// The render path stays pure: a panel reads the model and runs no probe, no
// command and no file scan of its own, so it cannot change what the installer
// does. A fact that would need a new probe is a gap to report, not a thing to
// add here.
const (
	// panelLabelMeasure is the columns a panel row spends on its label. It is a
	// constant, not the width of the longest label, so the value column is one
	// column down the whole panel.
	panelLabelMeasure = 14

	// panelLabelGap is the blank run between the label and its value, so the two
	// read as a label and a value instead of as one sentence.
	panelLabelGap = 2

	// panelBackupStamp is how a panel writes a backup's timestamp. It is the
	// layout the restore list already uses, so the same moment reads the same way
	// wherever it is shown.
	panelBackupStamp = "2006-01-02 15:04:05"
)

// A screen's panels are a list, not a single column: the registry below answers
// "which panels does this screen offer, in what order", the first being the
// default, and the model stores only which one is active. Everything else -- the
// tab row that names them, the single chip, the one-line summary a narrow
// terminal gets -- is derived from that list at render time, so a screen cannot
// advertise a panel it does not have.

// panelID addresses a panel. It is a stable name so a test can assert which
// panel a screen offers without depending on the order the screen lists them in,
// and so a later screen can refer to a panel another screen already defines.
type panelID string

const (
	// panelMachine is the welcome screen's panel: the machine the run is about to
	// change.
	panelMachine panelID = "machine"

	// panelLive is the live machine panel: the same machine's pulse right now,
	// sampled by a command and drawn from the model's ring. It is distinct from
	// panelMachine on purpose -- one answers what the machine is, the other what
	// it is doing -- and it is on the welcome screen, where the question is still
	// "where am I".
	panelLive panelID = "live"

	// panelPlan is the plan panel: the wizard's own steps and the state the run
	// will read. The main menu and the wizard's own questions all show it.
	panelPlan panelID = "plan"

	// panelTrainer is the trainer panel: what the player has gained in the Vim
	// Trainer so far. It is on the main menu, next to the plan.
	panelTrainer panelID = "trainer"

	// panelTip is the "Did you know?" panel: one fact rotating on the animation
	// tick, sourced from content the repository already ships. It is on the
	// welcome screen and the main menu.
	panelTip panelID = "tip"

	// panelLastInstall is the state panel: when this machine was last installed,
	// from which build, and what that run touched. It is on the main menu, and only
	// when a record exists.
	panelLastInstall panelID = "last-install"
)

// panel is one column a screen can offer. A panel is a value built by a
// function, not an interface a screen implements, so the registry stays a list
// of data and a screen cannot grow a second way to render one. It renders as:
//
//	Title     the name the tab row and the single chip show
//	Short     the name the narrow terminal's one-line summary uses, so a summary
//	          does not have to fit a panel's full sentence-length title
//	Headline  one short line saying what the panel is about, for the narrow
//	          terminal's summary row
//	Facts     the body rows at the layout's width, built from the model alone
//
// Facts is the same pure function the two-column panel already used: a panel
// reads the model and runs no probe, no command and no file scan of its own.
type panel struct {
	ID       panelID
	Title    string
	Short    string
	Headline func(Model) string
	Facts    func(Model, layout) []string
}

// panelsFor is the registry: the ordered panels a screen offers, the first being
// the default. A screen that offers none -- every screen but the welcome, the
// main menu and the wizard's own questions -- returns nil, and the frame then
// renders it exactly as it did before panels existed.
//
// The welcome screen asks where it is, so it offers the machine panel first --
// what the machine is -- then the live panel, what it is doing right now, and
// then a tip: after knowing the machine, the next thing worth seeing is its
// pulse and the next thing worth learning is something it can do. The main menu
// asks what is about to happen, so it offers the plan first, then what the player
// has gained in the trainer, then a tip, and last when this machine was installed
// -- each in the order it is worth reading. The wizard's own questions draw
// through renderSelection, and the plan is what is happening on them, so they
// offer the plan panel alone.
//
// Only the panels that exist are listed: an empty slot is not a panel, and a
// screen may not advertise a column it cannot fill. That is why "Last install"
// is offered only once the model holds a record: a tab row that named a panel
// with nothing to say would be worse than the shorter row.
func (m Model) panelsFor() []panel {
	switch m.Screen {
	case ScreenWelcome:
		return []panel{machinePanel(), livePanel(), tipPanel()}
	case ScreenMainMenu:
		panels := []panel{planPanel(), trainerPanel(), tipPanel()}
		if m.LastInstall != nil {
			panels = append(panels, lastInstallPanel())
		}
		return panels
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect,
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		return []panel{planPanel()}
	}
	return nil
}

// panelsOfferPlan reports whether a screen draws the plan panel. The plan panel
// is the one that lists a whole plan, so it is the panel whose screens reserve
// the creature's block: the plan's rows and the creature compete for the same
// frame, and the plan yields rather than the creature. The machine, live and tip
// panels are short enough that they never crowd it, and reserving for them would
// move bodies that do not need it.
func panelsOfferPlan(panels []panel) bool {
	for _, p := range panels {
		if p.ID == panelPlan {
			return true
		}
	}
	return false
}

// machinePanel is the machine panel's registry entry. Its facts and headline are
// the same pure reads the panel has always done, wrapped so the frame can name
// the panel without knowing what it holds.
func machinePanel() panel {
	return panel{
		ID:       panelMachine,
		Title:    welcomePanelLabel,
		Short:    "Machine",
		Headline: func(m Model) string { return m.machineHeadline() },
		Facts:    func(m Model, l layout) []string { return m.welcomePanelFacts(l) },
	}
}

// planPanel is the plan panel's registry entry. Its facts follow the selection:
// on the main menu they describe the option under the cursor, and on the
// wizard's own questions the choice being made. The name stays put, because the
// tab row must not move under the key that walks it.
func planPanel() panel {
	return panel{
		ID:       panelPlan,
		Title:    mainMenuPanelLabel,
		Short:    "Plan",
		Headline: func(m Model) string { return m.contextHeadline() },
		Facts:    func(m Model, l layout) []string { return m.contextPanelFacts(l) },
	}
}

// trainerPanel is the trainer panel's registry entry.
func trainerPanel() panel {
	return panel{
		ID:       panelTrainer,
		Title:    trainerPanelLabel,
		Short:    "Trainer",
		Headline: func(m Model) string { return m.trainerHeadline() },
		Facts:    func(m Model, l layout) []string { return m.trainerPanelFacts(l) },
	}
}

// tipPanel is the "Did you know?" panel's registry entry.
func tipPanel() panel {
	return panel{
		ID:       panelTip,
		Title:    tipPanelLabel,
		Short:    "Tip",
		Headline: func(m Model) string { return m.tipHeadline() },
		Facts:    func(m Model, l layout) []string { return m.tipFacts(l) },
	}
}

// lastInstallPanel is the state panel's registry entry.
func lastInstallPanel() panel {
	return panel{
		ID:       panelLastInstall,
		Title:    lastInstallPanelLabel,
		Short:    "Last install",
		Headline: func(m Model) string { return m.lastInstallHeadline() },
		Facts:    func(m Model, l layout) []string { return m.lastInstallPanelFacts(l) },
	}
}

// activePanelIndex is the panel the screen is showing: the model's stored index
// when it still addresses a panel of this screen, and the default (the first)
// otherwise. A screen change resets the stored index to the default, so the
// clamp here is the safety net for a model built by hand rather than the rule.
func (m Model) activePanelIndex(panels []panel) int {
	if m.PanelIndex < 0 || m.PanelIndex >= len(panels) {
		return 0
	}
	return m.PanelIndex
}

// nextPanelIndex is where Tab moves the active panel: the next panel of the
// screen, wrapping from the last back to the first. A screen with fewer than two
// panels has nowhere to go, and the bool says so: the key is then not consumed,
// which is what leaves Tab free for the screens that give it another meaning.
func nextPanelIndex(panels []panel, index int) (int, bool) {
	if len(panels) < 2 {
		return index, false
	}
	return (index + 1) % len(panels), true
}

// panelColumn renders a screen's right column: the header that names the panels
// the screen offers, the rule under it, and the active panel's facts clipped to
// the rows the frame leaves.
//
// A screen that offers exactly one panel keeps the single chip it has always
// shown, through the same panelBody the two panels used before the registry
// existed. A screen that offers more than one starts the column with a tab row
// naming them, the active one in brand chrome and the rest dim, separated by a
// dim separator, so the reader can see which panel is on and that there is
// another. The facts are clipped out loud either way: a panel with more rows than
// the frame leaves keeps the ones that fit and says how many it dropped.
func (m Model) panelColumn(panels []panel, l layout, budget int) []string {
	if len(panels) == 0 {
		return nil
	}
	active := m.activePanelIndex(panels)
	p := panels[active]
	if len(panels) == 1 {
		return panelBody(p.Title, p.Facts(m, l), l.Right, budget)
	}

	head := []string{tabRow(panels, active, l.Right), rule(l.Right)}
	room := budget - len(head)
	if room < 1 {
		return head
	}
	rows := p.Facts(m, l)
	if len(rows) <= room {
		return append(head, rows...)
	}

	kept := room - 1
	out := append(head, rows[:kept]...)
	return append(out, MutedStyle.Render(fmt.Sprintf("… and %d more", len(rows)-kept)))
}

// panelTabSeparator is the middle dot the panel layer separates two names with:
// the panels a tab row lists, and the machine headline's own facts. It is rendered
// in the dim tone wherever it draws the eye away from a name, so the row reads as
// a list of names and not as one name, and so it survives a terminal with no
// colour at all.
const panelTabSeparator = " · "

// tabRow names the panels a screen offers, the active one in brand chrome and
// the others dim. It is built from the plain titles first so the width check can
// never cut an escape sequence in half: a row that does not fit falls back to
// its plain text, truncated by the shared marker, rather than a styled string
// sliced mid-sequence.
func tabRow(panels []panel, active, width int) string {
	titles := make([]string, len(panels))
	for i, p := range panels {
		titles[i] = p.Title
	}
	if lipgloss.Width(strings.Join(titles, panelTabSeparator)) > width {
		return MutedStyle.Render(truncate(strings.Join(titles, panelTabSeparator), width))
	}

	parts := make([]string, len(panels))
	for i, p := range panels {
		if i == active {
			parts[i] = BrandStyle.Render(p.Title)
			continue
		}
		parts[i] = MutedStyle.Render(p.Title)
	}
	return strings.Join(parts, MutedStyle.Render(panelTabSeparator))
}

// rotatorMaxRows is how many rows the narrow terminal's summary may spend. Two
// is the design's limit: one row for a short fact, two when the name and the
// fact together need the second. A summary that would need a third row is not
// shown at all rather than cut, because a headline that stops mid-sentence says
// less than none.
const rotatorMaxRows = 2

// rotatorLines is the narrow terminal's summary of the active panel: its short
// name in the dim tone, then its headline fact in ink, wrapped at word boundaries
// to the width it has. It is the same panel the two-column screen shows, said in
// one line instead of a column, so the narrow terminal gets the same answer. The
// short name is used rather than the tab row's full title because a summary has
// one line to spend and a panel's title is a sentence.
//
// Nothing is truncated: a summary that fits in rotatorMaxRows rows is returned
// whole, and one that would not fit is not returned at all. It also refuses to
// show a name the width cannot hold on one line: a wrapped name is not the name.
func (m Model) rotatorLines(panels []panel, width int) []string {
	if len(panels) == 0 {
		return nil
	}
	p := panels[m.activePanelIndex(panels)]
	name := p.Short
	if name == "" {
		name = p.Title
	}

	plain := name
	if fact := p.Headline(m); fact != "" {
		plain = name + "  " + fact
	}
	lines := wrapText(plain, width, 0)
	if len(lines) > rotatorMaxRows || !strings.HasPrefix(lines[0], name) {
		return nil
	}

	out := make([]string, len(lines))
	for i, line := range lines {
		if i == 0 {
			out[i] = MutedStyle.Render(name) + InkStyle.Render(strings.TrimPrefix(line, name))
			continue
		}
		out[i] = InkStyle.Render(line)
	}
	return out
}

// placeRotator puts the narrow terminal's summary in the last rows the body did
// not need, and only there. A screen whose body fills its frame -- or leaves
// fewer spare rows than the summary needs -- is left alone: nothing may be
// dropped from a body to make room for a summary of it, so the rotator never
// displaces a body row, it only fills a blank one.
func placeRotator(placed, rotator []string) []string {
	if len(rotator) == 0 {
		return placed
	}
	spare := 0
	for spare < len(placed) && placed[len(placed)-1-spare] == "" {
		spare++
	}
	if spare < len(rotator) {
		return placed
	}

	out := append([]string(nil), placed...)
	start := len(out) - len(rotator)
	copy(out[start:], rotator)
	return out
}

// panelHints adds the [Tab] hint to a screen's own hints when the screen offers
// more than one panel, and leaves them alone otherwise. A screen that offers one
// panel gets no hint: Tab there does nothing, and a key that does nothing must
// not be advertised. The panel column and the footer are packed from this same
// list, so the hint and the row budget cannot disagree about how many rows the
// footer spends.
func (m Model) panelHints(panels []panel, hints []installerHint) []installerHint {
	if len(panels) < 2 {
		return hints
	}
	return append(append([]installerHint{}, hints...), hintTab)
}

// panelBody assembles a right-column body: the chip that names it, the rule under
// the chip, and its rows, clipped to the rows the frame leaves. A panel with more
// rows than it has room for keeps the ones that fit and says how many it dropped,
// so a wide-but-short terminal loses rows out loud instead of pushing the footer
// off the screen.
func panelBody(title string, rows []string, width, budget int) []string {
	out := []string{chip(title), rule(width)}
	room := budget - len(out)
	if room < 1 {
		return out
	}
	if len(rows) <= room {
		return append(out, rows...)
	}

	kept := room - 1
	out = append(out, rows[:kept]...)
	return append(out, MutedStyle.Render(fmt.Sprintf("… and %d more", len(rows)-kept)))
}

// panelFact renders one labelled fact: the label dim in the fixed measure, then
// the value in ink wrapped to the columns the label leaves, with every
// continuation line indented under the value so a wrapped row still reads as one
// row. A row with no value renders nothing, so a caller can offer a field the
// model may not have and get no row at all when it is empty.
func panelFact(label, value string, width int) []string {
	measure := min(panelLabelMeasure, max(1, width-panelLabelGap-1))
	valueWidth := max(1, width-measure-panelLabelGap)
	lines := wrapText(value, valueWidth, 0)
	if len(lines) == 0 {
		return nil
	}

	head := MutedStyle.Render(padRight(truncate(label, measure), measure)) + strings.Repeat(" ", panelLabelGap)
	indent := strings.Repeat(" ", measure+panelLabelGap)

	rows := []string{head + InkStyle.Render(lines[0])}
	for _, line := range lines[1:] {
		rows = append(rows, indent+InkStyle.Render(line))
	}
	return rows
}

// welcomePanelLabel names the welcome screen's right column. The welcome screen
// answers "where am I", so its panel is the machine the installer is about to
// change.
const welcomePanelLabel = "Your machine"

// welcomePanel is the welcome screen's right column: the machine the run is about
// to change, read from the fields system.Detect already filled and from nothing
// else. It takes no measurement of its own -- the render path is pure -- so a
// fact detection does not carry is left out and reported as a gap rather than
// filled in here with a command.
func (m Model) welcomePanel(l layout, budget int) []string {
	return panelBody(welcomePanelLabel, m.welcomePanelFacts(l), l.Right, budget)
}

// welcomePanelFacts is the machine panel's rows without the header that names
// the panel: every value comes from a field system.Detect filled, and a value the
// model does not hold renders no row at all. It is separate from welcomePanel so
// the panel registry can render the same rows under a tab row instead of the
// single chip.
func (m Model) welcomePanelFacts(l layout) []string {
	info := m.SystemInfo
	if info == nil {
		return nil
	}

	rows := panelFact("OS", info.OSName, l.Right)
	rows = append(rows, panelFact("Host", m.platformHost(), l.Right)...)
	rows = append(rows, panelFact("Arch", archName(info), l.Right)...)
	rows = append(rows, panelFact("Shell", info.UserShell, l.Right)...)
	rows = append(rows, panelFact("Packages", m.packageManager(), l.Right)...)
	rows = append(rows, panelFact("Xcode CLT", xcodeState(info), l.Right)...)
	rows = append(rows, panelFact("Home", info.HomeDir, l.Right)...)

	return rows
}

// machineHeadline is the machine panel said in one short line, for the narrow
// terminal's summary row: the operating system, the environment it runs inside
// when that is a fact of its own, and the architecture when detection named it.
// A fact the model does not have is left out, exactly as the panel leaves it out.
func (m Model) machineHeadline() string {
	info := m.SystemInfo
	if info == nil {
		return ""
	}

	parts := []string{info.OSName}
	if host := m.platformHost(); host != "" {
		parts = append(parts, host)
	}
	if arch := archName(info); arch != "" {
		parts = append(parts, arch)
	}
	return strings.Join(parts, panelTabSeparator)
}

// platformHost names the environment the installer is running inside, when that
// is a fact of its own. WSL is a layer over another operating system, so this row
// is the only place its version appears. Termux and macOS are already named by
// OSName, so repeating them here would print the same fact twice.
func (m Model) platformHost() string {
	info := m.SystemInfo
	if info == nil || !info.IsWSL {
		return ""
	}
	if info.WSLVersion > 0 {
		return fmt.Sprintf("WSL %d", info.WSLVersion)
	}
	return "WSL"
}

// archName names the architecture the model actually knows. It reads the fact
// detection records in Arch, not IsARM: x86_64 is as useful to name as ARM is,
// and IsARM stays behind for the callers that only ask whether the host is ARM.
// A model whose Arch is empty gets no row, so a host detection could not name is
// left out instead of guessed.
func archName(info *system.SystemInfo) string {
	if info == nil {
		return ""
	}
	return info.Arch
}

// xcodeState reports whether Xcode's command line tools are present. Detect
// probes them on macOS only, where HasXcode is a real answer; on any other host
// the field is false because the question was never asked, so the row is left out
// rather than claiming the tools are missing on Linux.
func xcodeState(info *system.SystemInfo) string {
	if info == nil || info.OS != system.OSMac {
		return ""
	}
	if info.HasXcode {
		return "installed"
	}
	return "missing"
}

// packageManager names the manager the installer will install through on this
// host. It reads the same predicates the dependency step dispatches through --
// the Termux branch of planPlatformInstall and usesPacman, usesDnf and usesApt --
// so the panel cannot name a manager the run would not use. A host with none of
// them gets no row instead of a guess.
func (m Model) packageManager() string {
	info := m.SystemInfo
	if info == nil {
		return ""
	}
	switch {
	case info.IsTermux:
		return "pkg"
	case usesPacman(&m):
		return "pacman"
	case usesDnf(&m):
		return "dnf"
	case usesApt(&m):
		return "apt-get"
	case info.HasBrew:
		return "brew"
	}
	return ""
}

// mainMenuPanelLabel names the main menu's right column: the main menu asks what
// the run about to start will do, so its panel is the plan and the state that run
// will read. It is a constant so the frame guard can read the panel back out of a
// rendered screen by the name a reader sees.
const mainMenuPanelLabel = "What will happen"

// mainMenuPanel is the main menu's right column. It shows the option under the
// cursor and, under that, what the option holds:
//
//   - Start Installation: the plan the run would execute, the configs it would
//     overwrite and the newest backup;
//   - Learn About Tools: how many terminals, shells and multiplexers the learn
//     screens describe;
//   - Keymaps Reference: how many bindings each of the five tools ships;
//   - LazyVim Guide: how many topics the guide holds;
//   - Vim Trainer: how many modules, lessons and bosses the curriculum ships;
//   - Restore from Backup: every backup the model holds, with when it was taken
//     and how many files it carries;
//   - Update this installer: the published release this build is behind;
//   - Exit: one line that says quitting changes nothing.
//
// The panel's name does not move with the cursor -- the tab row would wander,
// and Tab would become a moving target on the very key that walks the panels --
// so the option is named in the panel's first row instead. The wizard's own
// questions draw the same panel through contextPanelFacts, so the two screens
// cannot describe the selection differently.
//
// It is a glance, not a document: the panel names how many steps there are and
// the one the run starts at -- or, once a run is in progress, the step it is on
// -- with that step's description, because the next action is the part worth
// reading here. The other steps' names and descriptions are not lost: they are on
// the installing screen, next to the step that is running, which is where a plan
// is read rather than skimmed.
//
// Nothing is invented: every value comes from a field the model already holds,
// and a fact the model does not have renders no row rather than a zero or a
// guess. A fact the panel does have is never dropped to save room: a long list
// wraps under its label, and panelBody says how many rows it could not show.
func (m Model) mainMenuPanel(l layout, budget int) []string {
	return panelBody(mainMenuPanelLabel, m.contextPanelFacts(l), l.Right, budget)
}

// contextPanelFacts is the plan panel's rows, whatever screen it is drawn on:
// the wizard's own questions describe the choice under the cursor, and the main
// menu describes the option under its cursor. It is the one entry the panel
// registry calls, so a screen cannot draw a different plan through a second path.
func (m Model) contextPanelFacts(l layout) []string {
	if isWizardChoiceScreen(m.Screen) {
		return m.wizardChoicePanelFacts(l)
	}
	return m.mainMenuPanelFacts(l)
}

// isWizardChoiceScreen reports whether a screen is one of the wizard's own
// questions, which draw the plan beside the choice they are asking.
func isWizardChoiceScreen(screen Screen) bool {
	switch screen {
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect,
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		return true
	}
	return false
}

// contextHeadline is the plan panel said in one short line, for the narrow
// terminal's summary row: the same answer the two-column panel gives, in one
// line. A selection with nothing to say -- a plan the model cannot build, a
// restore panel with no backups -- gets no line rather than an invented one.
func (m Model) contextHeadline() string {
	if isWizardChoiceScreen(m.Screen) {
		// An unanswered question has no line: the summary answers "what would
		// this choice do", and there is no choice yet.
		if !m.wizardHasSelection() {
			return ""
		}
		opts, _ := m.wizardHighlightedChoice()
		return planHeadlineFor(m.previewPlanFor(opts))
	}

	choice, _ := m.mainMenuSelection()
	switch choice {
	case choiceLearnTools:
		return fmt.Sprintf("%d terminals%s%d shells%s%d multiplexers",
			len(GetTerminalInfo()), panelTabSeparator, len(GetShellInfo()),
			panelTabSeparator, len(GetWMInfo()))
	case choiceKeymaps:
		return fmt.Sprintf("%d tools%s%s", len(keymapSources()), panelTabSeparator, keymapBindingCount(totalKeymapCount()))
	case choiceLazyVim:
		return fmt.Sprintf("%d topics", len(GetLazyVimTopics()))
	case choiceTrainer:
		return trainerCurriculumHeadline()
	case choiceRestore:
		if len(m.AvailableBackups) == 0 {
			return ""
		}
		return backupCount(len(m.AvailableBackups)) + " to restore"
	case choiceUpdate:
		if !m.UpdateInstallable() {
			return ""
		}
		return m.UpdateCheck.Latest + " ready"
	case choiceUtilities:
		if !m.ThemeSwitchFound {
			return ""
		}
		return m.ThemeSwitch.Name + " theme switch"
	case choiceExit:
		return "nothing changes"
	}
	return m.planHeadline()
}

// mainMenuChoice names the main-menu option the cursor is on. It is an enum
// rather than the option's label so the panel switches on a stable name, and a
// label the menu rewords does not silently become a different panel.
type mainMenuChoice int

const (
	choiceStartInstallation mainMenuChoice = iota
	choiceLearnTools
	choiceKeymaps
	choiceLazyVim
	choiceTrainer
	choiceRestore
	choiceUpdate
	choiceUtilities
	choiceExit
)

// mainMenuSelection is the option under the cursor and its plain name: the label
// without the leading emoji, so the panel names the option in words a terminal
// without an emoji font can draw. A cursor the menu does not cover -- a model
// built by hand -- is treated as the first option, which is the menu's own
// default, and gets no name.
func (m Model) mainMenuSelection() (mainMenuChoice, string) {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return choiceStartInstallation, ""
	}
	label := options[m.Cursor]
	name := plainOptionName(label)
	switch {
	case strings.Contains(label, "Start Installation"):
		return choiceStartInstallation, name
	case strings.Contains(label, "Learn About Tools"):
		return choiceLearnTools, name
	case strings.Contains(label, "Keymaps Reference"):
		return choiceKeymaps, name
	case strings.Contains(label, "LazyVim Guide"):
		return choiceLazyVim, name
	case strings.Contains(label, "Vim Trainer"):
		return choiceTrainer, name
	case strings.Contains(label, "Restore from Backup"):
		return choiceRestore, name
	case label == updateInstallerRow:
		return choiceUpdate, name
	case strings.Contains(label, "Utilities"):
		return choiceUtilities, name
	case strings.Contains(label, "Exit"):
		return choiceExit, name
	default:
		return choiceStartInstallation, name
	}
}

// mainMenuPanelFacts is the main menu's contextual panel: the option under the
// cursor, then what that option holds. The first row names the option, so the
// panel answers about the right thing without changing its own name.
func (m Model) mainMenuPanelFacts(l layout) []string {
	choice, name := m.mainMenuSelection()
	rows := selectedPanelRow(name, l.Right)
	switch choice {
	case choiceLearnTools:
		rows = append(rows, learnToolsPanelFacts(l)...)
	case choiceKeymaps:
		rows = append(rows, keymapsPanelFacts(l)...)
	case choiceLazyVim:
		rows = append(rows, lazyVimPanelFacts(l)...)
	case choiceTrainer:
		rows = append(rows, trainerCurriculumFacts(l)...)
	case choiceRestore:
		rows = append(rows, m.backupPanelFacts(l)...)
	case choiceUpdate:
		rows = append(rows, m.updatePanelFacts(l)...)
	case choiceUtilities:
		rows = append(rows, m.utilitiesPanelFacts(l)...)
	case choiceExit:
		rows = append(rows, exitPanelFacts(l)...)
	default:
		rows = append(rows, m.planPanelFacts(l)...)
	}
	return rows
}

// selectedPanelRow names the option the panel is describing. It is the one row
// every contextual panel shares, so the reader always knows which selection the
// facts belong to, and it is left out when there is no name to give.
func selectedPanelRow(name string, width int) []string {
	if name == "" {
		return nil
	}
	return panelFact("Selected", name, width)
}

// plainOptionName is a menu option's label without the leading emoji or symbol
// the menus draw to catch the eye. The panel names the option in words, so a
// terminal without an emoji font is not handed a second row of boxes.
func plainOptionName(label string) string {
	for i, r := range label {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.TrimSpace(label[i:])
		}
	}
	return strings.TrimSpace(label)
}

// planPanelFacts is the plan body the main menu's Start Installation option and
// the wizard's own questions share: the preview plan with the host it is for,
// the configs the run would overwrite and the newest backup. It is separate from
// the contextual rows so both callers render the same plan.
func (m Model) planPanelFacts(l layout) []string {
	steps, hostNote := m.previewPlan()
	return m.planPanelRows(steps, hostNote, l)
}

// planPanelRows renders the plan body: the host note when the plan is being
// previewed for a host, the step count and the step the run is on with its
// description, the configs the run would overwrite, and the newest backup.
//
// The step list is a glance, not the whole plan: it names how many steps there
// are and the one the run will act on next, which is the step a reader is about
// to act on and the only description worth spending rows on here. The other
// steps' names are not lost -- the count says how many there are, and the
// installing screen shows each step with its description as it runs, which is
// where a plan is read rather than skimmed. The destructive facts stay whole:
// the overwrite list and the newest backup are never dropped to save rows,
// because a reader must not be surprised by what the run is about to replace.
func (m Model) planPanelRows(steps []InstallStep, hostNote string, l layout) []string {
	var rows []string
	if hostNote != "" {
		rows = append(rows, MutedStyle.Render(truncate(hostNote, l.Right)))
	}
	if len(steps) > 0 {
		rows = append(rows, panelFact("Steps", strconv.Itoa(len(steps)), l.Right)...)
		rows = append(rows, nextStepRow(steps, currentStepIndex(steps), l.Right)...)
	}
	if len(m.ExistingConfigs) > 0 {
		rows = append(rows, panelFact("Overwrites", configCount(len(m.ExistingConfigs)), l.Right)...)
		for _, config := range m.ExistingConfigs {
			rows = append(rows, panelFact("", config, l.Right)...)
		}
	}
	if backup, ok := newestBackup(m.AvailableBackups); ok {
		rows = append(rows, panelFact("Newest backup", backupLine(backup, m.Now), l.Right)...)
	}
	return rows
}

// learnToolsPanelFacts counts what the Learn About Tools screens ship: the
// terminals, shells and multiplexers the repository declares, taken from the
// same tables those screens look a tool up in.
func learnToolsPanelFacts(l layout) []string {
	rows := panelFact("Terminals", strconv.Itoa(len(GetTerminalInfo())), l.Right)
	rows = append(rows, panelFact("Shells", strconv.Itoa(len(GetShellInfo())), l.Right)...)
	rows = append(rows, panelFact("Multiplexers", strconv.Itoa(len(GetWMInfo())), l.Right)...)
	return rows
}

// keymapSource is one tool's keymap reference: the name the tab row and the
// reference menu use, and the categories the tables are built from.
type keymapSource struct {
	name string
	cats []KeymapCategory
}

// keymapSources is the keymap reference the panel counts: the five tools the
// reference menu offers, in the order it offers them, read through the same
// getters the tables are loaded from so the panel and the tables cannot disagree.
func keymapSources() []keymapSource {
	return []keymapSource{
		{"Neovim", GetNvimKeymaps()},
		{"Tmux", GetTmuxKeymaps()},
		{"Zellij", GetZellijKeymaps()},
		{"Herdr", GetHerdrKeymaps()},
		{"Ghostty", GetGhosttyKeymaps()},
	}
}

// keymapsPanelFacts counts the bindings each tool's keymap reference ships. A
// tool whose tables are empty is left out rather than shown as a zero.
func keymapsPanelFacts(l layout) []string {
	var rows []string
	for _, src := range keymapSources() {
		if n := keymapCount(src.cats); n > 0 {
			rows = append(rows, panelFact(src.name, keymapBindingCount(n), l.Right)...)
		}
	}
	return rows
}

// keymapCount is how many bindings a tool's categories hold.
func keymapCount(cats []KeymapCategory) int {
	n := 0
	for _, cat := range cats {
		n += len(cat.Keymaps)
	}
	return n
}

// totalKeymapCount is every binding the whole keymap reference ships.
func totalKeymapCount() int {
	n := 0
	for _, src := range keymapSources() {
		n += keymapCount(src.cats)
	}
	return n
}

// keymapBindingCount names a binding count, in the singular when there is one.
func keymapBindingCount(n int) string {
	if n == 1 {
		return "1 binding"
	}
	return fmt.Sprintf("%d bindings", n)
}

// lazyVimPanelFacts says how many topics the LazyVim guide holds. The count is
// the one the guide's own menu is built from.
func lazyVimPanelFacts(l layout) []string {
	return panelFact("Topics", strconv.Itoa(len(GetLazyVimTopics())), l.Right)
}

// trainerCurriculum counts what the Vim Trainer ships: the modules, the lessons
// and the bosses its catalogue declares. It is the content, not the player's
// progress, which the "Your trainer" panel already answers.
func trainerCurriculum() (modules, lessons, bosses int) {
	all := trainer.GetAllModules()
	for _, mod := range all {
		lessons += len(trainer.GetLessons(mod.ID))
		if trainer.GetBoss(mod.ID) != nil {
			bosses++
		}
	}
	return len(all), lessons, bosses
}

// trainerCurriculumFacts renders the curriculum count for the panel.
func trainerCurriculumFacts(l layout) []string {
	modules, lessons, bosses := trainerCurriculum()
	rows := panelFact("Modules", strconv.Itoa(modules), l.Right)
	rows = append(rows, panelFact("Lessons", strconv.Itoa(lessons), l.Right)...)
	if bosses > 0 {
		rows = append(rows, panelFact("Bosses", strconv.Itoa(bosses), l.Right)...)
	}
	return rows
}

// trainerCurriculumHeadline says the curriculum count in one line for the narrow
// terminal's summary.
func trainerCurriculumHeadline() string {
	modules, lessons, _ := trainerCurriculum()
	return fmt.Sprintf("%d modules%s%d lessons", modules, panelTabSeparator, lessons)
}

// backupPanelFacts lists the backups the model holds, each with when it was
// taken and how many files it carries. It reads the model's own list, so the
// panel and the restore screen show the same set in the same order, and it says
// nothing when there is nothing to restore.
func (m Model) backupPanelFacts(l layout) []string {
	if len(m.AvailableBackups) == 0 {
		return nil
	}
	rows := panelFact("Backups", strconv.Itoa(len(m.AvailableBackups)), l.Right)
	for _, backup := range m.AvailableBackups {
		rows = append(rows, panelFact("", backupLine(backup, m.Now), l.Right)...)
	}
	return rows
}

// updatePanelFacts is what the panel says for the main menu's update button: the
// release that is published, the build this run is, and the one thing pressing
// the row does. Every value comes from the model's own update state, so the panel
// and the button cannot disagree about which release is being offered.
func (m Model) updatePanelFacts(l layout) []string {
	if !m.UpdateInstallable() {
		return nil
	}
	rows := panelFact("Published", m.UpdateCheck.Latest, l.Right)
	if build := BuildLabel(); build != "" {
		rows = append(rows, panelFact("This build", build, l.Right)...)
	}
	for _, line := range wrapText("Pressing it downloads the release, checks it against that release's own "+
		"SHA256SUMS and installs it over this binary, keeping the replaced one beside it.", l.Right, 0) {
		rows = append(rows, InkStyle.Render(line))
	}
	return rows
}

// exitPanelFacts is the one honest line the panel says for Exit: quitting
// changes nothing. It claims no state, because there is none to read.
func exitPanelFacts(l layout) []string {
	var rows []string
	for _, line := range wrapText("Nothing on this machine changes.", l.Right, 0) {
		rows = append(rows, InkStyle.Render(line))
	}
	return rows
}

// utilitiesPanelEntry is one utility as the panel describes it: the short label
// the panel's own column can hold, the fact it states, the line it says instead
// when the utility is not offered, and whether the section offers it. The label
// is the panel's, but it describes the section's row, which is named in the
// entry's comment so the two cannot drift into two names for one job.
type utilitiesPanelEntry struct {
	label   string
	value   string
	absent  string
	offered bool
}

// utilitiesPanelFacts is what the plan panel says for the Utilities row. The
// section changes nothing until a row inside it is chosen, so the panel names
// what each utility holds, or says plainly that this host does not offer it.
//
// Every row comes from utilitiesPanelEntries, which derives them from the same
// model state the section's own options are built from, never from the host and
// never from a list typed here: the panel's size is len(entries), so a utility
// the section offers shows up in the panel or is visibly missing from that one
// function. A second list beside the section is the defect the palette's sixth
// hand-written copy already paid for.
func (m Model) utilitiesPanelFacts(l layout) []string {
	var rows []string
	for _, entry := range m.utilitiesPanelEntries() {
		if !entry.offered {
			for _, line := range wrapText(entry.absent, l.Right, 0) {
				rows = append(rows, InkStyle.Render(line))
			}
			continue
		}
		rows = append(rows, panelFact(entry.label, entry.value, l.Right)...)
	}
	return rows
}

// utilitiesPanelEntries derives the panel's rows, one entry per utility the
// section lists, in the section's own order. Each entry reads the model fields
// the section reads for the same row -- ThemeSwitchFound,
// themeUndoAvailable, DotfilesThemeRecord, dotfilesThemeOptions and WSLState --
// so the panel and the section cannot disagree about what is offered.
func (m Model) utilitiesPanelEntries() []utilitiesPanelEntry {
	entries := []utilitiesPanelEntry{
		// The section's "Switch to the dark theme" and "Switch to the light
		// theme" rows, which are one fact: which switch this host has.
		{
			label:   "Switch",
			value:   m.ThemeSwitch.Name,
			absent:  "No desktop theme switch is available here.",
			offered: m.ThemeSwitchFound,
		},
	}

	// The section's "Undo the last theme change" row, offered under the same
	// condition the section offers it: a detected switch and a record written
	// for that same desktop.
	if m.themeUndoAvailable() {
		entries = append(entries, utilitiesPanelEntry{label: "Undo", value: "available", offered: true})
	}

	// The section's utilitiesThemeRow, whose theme list lives one level in on
	// the picker. It is offered exactly when the section offers it: there are
	// themes to apply, or there is a dotfiles-theme record to put back.
	themes := m.dotfilesThemeOptions()
	switch {
	case len(themes) > 0:
		entries = append(entries, utilitiesPanelEntry{
			label:   "Themes",
			value:   themeCountLabel(len(themes)),
			offered: true,
		})
		if m.DotfilesThemeRecord != nil {
			entries = append(entries, utilitiesPanelEntry{label: "Themes undo", value: "available", offered: true})
		}
	case m.DotfilesThemeRecord != nil:
		// A record with no definitions read is an undo and nothing to list, so the
		// one entry says exactly that rather than a second row repeating it.
		entries = append(entries, utilitiesPanelEntry{label: "Themes", value: "undo available", offered: true})
	default:
		// The definitions are read only when the section is opened, so before
		// that the panel cannot say whether the theme is switchable. It says the
		// question has not been checked yet rather than answering it with a fact
		// nobody established; once a read has finished -- with a list or with a
		// reason -- the panel can state the verdict.
		absent := "The dotfiles' own theme is not switchable here."
		if m.dotfilesThemesPending() {
			absent = "The dotfiles' own theme has not been checked yet."
		}
		entries = append(entries, utilitiesPanelEntry{
			label:  "Themes",
			absent: absent,
		})
	}

	// The section's utilitiesWSLRow. It is offered only where there is a
	// .wslconfig to edit, and the value is the recommendation the screen would
	// start from -- read from the plan the section holds, never recomputed here.
	entries = append(entries, utilitiesPanelEntry{
		label:   "WSL resources",
		value:   wslPanelValue(m.WSLState),
		absent:  "The WSL resources are not adjustable here.",
		offered: m.WSLState.Available,
	})

	// The section's utilitiesShellAuditRow, the utility that changes nothing. It
	// is offered wherever the login shell and a terminal were resolved, and the
	// value is the last measurement the model holds: nothing is measured until the
	// screen's own row is pressed, so before that the panel says so rather than
	// printing a number nobody took.
	entries = append(entries, utilitiesPanelEntry{
		label:   "Shell start",
		value:   shellAuditPanelValue(m.ShellAudit),
		absent:  "The shell's startup is not measurable here.",
		offered: m.ShellAudit.Available,
	})

	// The section's utilitiesTerminalRow. The report is read-only and offered
	// everywhere, so the panel always names it: before the screen has been
	// opened the value says the terminal has not been inspected, and afterwards
	// it is the depth the report found. It sits last so it never displaces a
	// utility above it on a short frame.
	entries = append(entries, utilitiesPanelEntry{
		label:   "Terminal",
		value:   m.TerminalCapabilities.panelValue(),
		offered: true,
	})

	return entries
}

// shellAuditPanelValue is what the Utilities panel says the shell startup row
// holds. It reads the model's audit state and measures nothing: a panel is drawn
// on every keypress, and starting the user's shell is not a render.
func shellAuditPanelValue(st shellAuditState) string {
	switch {
	case st.Measuring:
		return st.Shell + ": measuring"
	case !st.Measured:
		return st.Shell + ": not measured yet"
	case st.Median > 0:
		return fmt.Sprintf("%s: median %s of %d runs", st.Shell, shellAuditDuration(st.Median), len(st.Samples))
	default:
		return st.Shell + ": no start finished"
	}
}

// themeCountLabel names how many themes can be applied, in the singular when
// there is one, because "1 themes" reads as a count nobody looked at.
func themeCountLabel(n int) string {
	if n == 1 {
		return "1 theme"
	}
	return fmt.Sprintf("%d themes", n)
}

// wslPanelValue is what the Utilities panel says the WSL resource row holds: the
// values the section would recommend for this host, built from the plan the
// section already holds rather than from a second calculation. A host whose
// capacities were not read says so instead of printing zeroes as limits, and the
// units come from the same label helper the screen uses.
func wslPanelValue(state wslResourceState) string {
	var parts []string
	if state.Plan.MemoryMB > 0 {
		parts = append(parts, "memory "+wslUnitLabel(state.Plan.MemoryMB, "MB"))
	}
	if state.Plan.Processors > 0 {
		parts = append(parts, "processors "+wslUnitLabel(state.Plan.Processors, ""))
	}
	if state.Plan.SwapMB > 0 {
		parts = append(parts, "swap "+wslUnitLabel(state.Plan.SwapMB, "MB"))
	}
	if len(parts) == 0 {
		return "capacities not read"
	}
	return strings.Join(parts, ", ")
}

// planHeadline is the plan said in one short line: how many steps the run would
// execute and, before the host has been chosen, which host the plan was built
// for. A plan the model cannot build -- no detection to build it from -- gets no
// line rather than an invented one.
func (m Model) planHeadline() string {
	return planHeadlineFor(m.previewPlan())
}

// planHeadlineFor says what a plan is in one line. A plan with no steps gets no
// line rather than an invented one.
func planHeadlineFor(steps []InstallStep, hostNote string) string {
	if len(steps) == 0 {
		return ""
	}
	headline := stepCount(len(steps))
	if hostNote != "" {
		headline += " " + hostNote
	}
	return headline
}

// stepCount names how many steps a plan has, in the singular when there is one,
// because "1 steps" reads as a count nobody looked at.
func stepCount(n int) string {
	if n == 1 {
		return "1 step"
	}
	return fmt.Sprintf("%d steps", n)
}

// stepPanelRows renders the steps as a numbered list: one row per step holding
// its name, with the step the run starts at (or is on) marked by the same ▸ the
// menus use for "here" and carrying its description under it. The marker has a
// column of its own and the number is right-aligned in the column beside it, so
// the names start on one column and the digits form a straight edge whether the
// plan has nine steps or ninety, and marking a step shifts neither. A name or
// description wider than the column wraps under itself rather than being cut.
//
// The plan panel no longer spends a row per step; it draws the count and the one
// step the run is on through nextStepRow below, and this full list is the shape
// the installing rail and the tests still pin.
func stepPanelRows(steps []InstallStep, start, width int) []string {
	numberWidth := len(strconv.Itoa(len(steps)))
	var rows []string
	for i, step := range steps {
		rows = append(rows, stepRow(step, i+1, numberWidth, i == start, width)...)
	}
	return rows
}

// nextStepRow renders the one step the plan panel spends rows on: the step the
// run starts at, or the step it is on once a run is in progress. It is the same
// row the full list draws -- the ▸ marker, the number in the list's own column,
// the name, and the description under it -- so the panel and the installing
// screen name the next action the same way. The number keeps its place in the
// plan, so "▸ 3" still means the third step of the eight the count above names.
func nextStepRow(steps []InstallStep, start, width int) []string {
	if start < 0 || start >= len(steps) {
		return nil
	}
	return stepRow(steps[start], start+1, len(strconv.Itoa(len(steps))), true, width)
}

// stepRow renders one numbered step: the marker and number head, the name
// wrapped under the value column, and the description under the name when the
// step is the marked one. It is split out of stepPanelRows so the one-row glance
// and the full list cannot drift apart on the marker, the number column or the
// wrap width.
func stepRow(step InstallStep, number, numberWidth int, marked bool, width int) []string {
	const markerWidth, gap = 2, 2
	indentWidth := markerWidth + numberWidth + gap
	valueWidth := max(1, width-indentWidth)
	indent := strings.Repeat(" ", indentWidth)

	marker := "  "
	if marked {
		marker = AccentKeyStyle.Render("▸ ")
	}
	head := marker + MeterStyle.Render(fmt.Sprintf("%*d", numberWidth, number)) + "  "

	name := wrapText(step.Name, valueWidth, 0)
	if len(name) == 0 {
		name = []string{""}
	}
	rows := []string{head + InkStyle.Render(name[0])}
	for _, line := range name[1:] {
		rows = append(rows, indent+InkStyle.Render(line))
	}

	// The description belongs to the step that is about to run, and to no other:
	// it is the one step the reader will act on next.
	if marked && step.Description != "" {
		for _, line := range wrapText(step.Description, valueWidth, 0) {
			rows = append(rows, indent+MutedStyle.Render(line))
		}
	}
	return rows
}

// previewPlan returns the plan the panel shows and, when it is describing a host
// the player has not chosen yet, one dim line naming that host.
//
// Once the wizard has built the plan, that plan is the answer and no line is
// needed. Before that -- the main menu is reached before the first question --
// the plan is rebuilt from the same pure builder with the OS filled in from the
// choice if it has been made, and otherwise from detection. Rebuilding it is
// what stops the panel from being empty on the one screen that asks "what will
// happen"; taking the OS from detection is what stops it describing a plan for a
// host the run will not use.
func (m Model) previewPlan() ([]InstallStep, string) {
	if len(m.Steps) > 0 {
		return m.Steps, ""
	}
	return m.previewPlanFor(m.planOptions())
}

// previewPlanFor returns the plan the given options build and, when the options
// describe a host other than the detected one, one dim line naming that host.
// The wizard's own questions call it with the highlighted choice, so the panel
// describes the plan that choice would lead to rather than one already recorded.
func (m Model) previewPlanFor(opts planOptions) ([]InstallStep, string) {
	// A model with no detection has no host to describe, so it has no plan to
	// show: the panel says nothing rather than inventing an OS to build one from.
	if m.SystemInfo == nil {
		return nil, ""
	}

	detected := detectedOSChoice(m.SystemInfo)
	switch {
	case opts.OS == "":
		opts.OS = detected
		return planFor(opts), "on " + m.SystemInfo.OSName + " (detected)"
	case opts.OS != detected:
		return planFor(opts), "on " + chosenOSName(opts.OS)
	default:
		return planFor(opts), ""
	}
}

// wizardHasSelection reports whether the wizard's cursor is on a row of the
// question it is showing. A step PR #143 opened with noSelection has no row
// marked, and neither would a cursor past the end of the options; both mean there
// is nothing under the cursor for the panel to describe.
func (m Model) wizardHasSelection() bool {
	return m.Cursor >= 0 && m.Cursor < len(m.GetCurrentOptions())
}

// noChoiceNote is the wizard panel's line while a question has nothing under the
// cursor: the panel describes the choice under the cursor and the plan it would
// lead to, and with no row marked there is no choice to describe, so it says so
// rather than previewing a plan nobody selected.
const noChoiceNote = "No option selected yet"

// wizardChoicePanelFacts is a wizard question's panel: the choice under the
// cursor, then the plan that choice would lead to, with the step the run is on
// marked. The choice is read from the cursor rather than from the model's
// recorded answers, because the question is still open and the panel describes
// what the player is about to pick. With nothing under the cursor there is no
// choice to describe -- and no plan to preview, because the plan would be for a
// host nobody picked -- so the panel says the question is still unanswered.
func (m Model) wizardChoicePanelFacts(l layout) []string {
	if !m.wizardHasSelection() {
		return []string{MutedStyle.Render(noChoiceNote)}
	}
	opts, name := m.wizardHighlightedChoice()
	steps, hostNote := m.previewPlanFor(opts)
	rows := selectedPanelRow(name, l.Right)
	return append(rows, m.planPanelRows(steps, hostNote, l)...)
}

// wizardHighlightedChoice is the choice under the cursor expressed as the plan
// options it would record, together with the option's plain name. Only the
// question the screen is asking is overridden: the choices already answered come
// from the model, so the plan the panel shows is the one the run would build.
// An option that is not a plan choice -- a separator, or a "learn about" row --
// leaves the recorded options alone, and the plan describes that state.
func (m Model) wizardHighlightedChoice() (planOptions, string) {
	opts := m.planOptions()
	options := m.GetCurrentOptions()
	name := ""
	if m.Cursor >= 0 && m.Cursor < len(options) {
		name = plainOptionName(options[m.Cursor])
	}

	switch m.Screen {
	case ScreenOSSelect:
		// The row the cursor is on, and only that row: a cursor with no row under
		// it -- the noSelection PR #143 opens an undetected step with -- leaves the
		// recorded answer alone instead of answering "Linux" for the player.
		switch m.Cursor {
		case osOptionMac:
			opts.OS = "mac"
		case osOptionLinux:
			opts.OS = "linux"
		case osOptionTermux:
			opts.OS = "termux"
		}
	case ScreenTerminalSelect:
		if term, ok := installedChoice(name, "alacritty", "wezterm", "kitty", "ghostty", "none"); ok {
			opts.Terminal = term
		}
	case ScreenFontSelect:
		if yesNoChoice(name) {
			opts.InstallFont = m.Cursor == 0
		}
	case ScreenShellSelect:
		if shell, ok := installedChoice(name, "fish", "zsh", "nushell"); ok {
			opts.Shell = shell
		}
	case ScreenWMSelect:
		if wm, ok := installedChoice(name, "tmux", "zellij", "herdr", "none"); ok {
			opts.WindowMgr = wm
		}
	case ScreenNvimSelect:
		if yesNoChoice(name) {
			opts.InstallNvim = m.Cursor == 0
		}
	}
	return opts, name
}

// installedChoice matches an option's plain name against the tool names a
// question offers, case-insensitively, and returns the tool's key. It matches
// the first word, so "None" is "none" while a "learn about" row is not a tool.
// The bool says whether the option names a tool at all.
func installedChoice(name string, tools ...string) (string, bool) {
	fields := strings.Fields(strings.ToLower(name))
	if len(fields) == 0 {
		return "", false
	}
	for _, tool := range tools {
		if fields[0] == tool {
			return tool, true
		}
	}
	return "", false
}

// yesNoChoice reports whether an option's plain name starts with yes or no, the
// two halves of the font and Neovim questions. The cursor picks the half, exactly
// as the wizard's own handler does.
func yesNoChoice(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "yes") || strings.HasPrefix(lower, "no")
}

// detectedOSChoice maps what detection found to the wizard's own OS choice
// values, so a preview plan is built for the OS the run would actually use. It
// is the same three-way split the wizard offers: Termux, macOS, or the Linux
// family (whose distribution decides the package manager).
func detectedOSChoice(info *system.SystemInfo) string {
	switch {
	case info.IsTermux || info.OS == system.OSTermux:
		return "termux"
	case info.OS == system.OSMac:
		return "mac"
	default:
		return "linux"
	}
}

// chosenOSName names an OS the player chose, for the panel's one-line note when
// the choice differs from what detection found.
func chosenOSName(os string) string {
	switch os {
	case "mac":
		return "macOS"
	case "termux":
		return "Termux"
	default:
		return "Linux"
	}
}

// currentStepIndex is the step the run is on, or the first one it would run: the
// first step that is neither done nor skipped, which is the first step while
// nothing has started. It is -1 when there are no steps or every step has
// finished, so no step is marked with the ▸ the menus use for "here".
func currentStepIndex(steps []InstallStep) int {
	for i, step := range steps {
		if step.Status != StatusDone && step.Status != StatusSkipped {
			return i
		}
	}
	return -1
}

// newestBackup returns the most recent backup the model holds. The list
// system.ListBackups returns is in directory order, not date order, so the newest
// is chosen here rather than taken from an end of the slice.
func newestBackup(backups []system.BackupInfo) (system.BackupInfo, bool) {
	if len(backups) == 0 {
		return system.BackupInfo{}, false
	}
	newest := backups[0]
	for _, backup := range backups[1:] {
		if backup.Timestamp.After(newest.Timestamp) {
			newest = backup
		}
	}
	return newest, true
}

// backupLine is one line about a backup: when it was taken, how many files it
// holds and, when the model carries a reference time, how old it is. The age is
// measured against the model's own clock (Model.Now, copied from the run's tick
// or left zero by a model the startup path has not ticked), never read from the
// renderer, so two renders of one model are the same bytes and a zero reference
// time simply leaves the age out rather than printing a moment nobody measured.
func backupLine(backup system.BackupInfo, now time.Time) string {
	line := fmt.Sprintf("%s, %s", backup.Timestamp.Format(panelBackupStamp), fileCount(len(backup.Files)))
	if now.IsZero() {
		return line
	}
	return line + ", " + backupAge(backup.Timestamp, now)
}

// configCount names how many configs a run will overwrite, in the singular when
// there is one, because "1 configs" reads as a count nobody looked at.
func configCount(n int) string {
	if n == 1 {
		return "1 config"
	}
	return fmt.Sprintf("%d configs", n)
}

// fileCount is configCount's counterpart for a backup's file list.
func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// backupCount names how many backups the model holds, in the singular when
// there is one.
func backupCount(n int) string {
	if n == 1 {
		return "1 backup"
	}
	return fmt.Sprintf("%d backups", n)
}

// backupAge names how long ago a backup was taken, in the coarsest unit that
// still says something: minutes under an hour, hours under a day, days under a
// month, then months. It takes the reference time as a parameter rather than
// reading it, so the arithmetic is a pure function its test can pin to the
// minute and the panel never reads a clock while rendering.
func backupAge(then, now time.Time) string {
	age := now.Sub(then)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return pluralUnit(int(age.Minutes()), "minute")
	case age < 24*time.Hour:
		return pluralUnit(int(age.Hours()), "hour")
	case age < 30*24*time.Hour:
		return pluralUnit(int(age.Hours()/24), "day")
	default:
		return pluralUnit(int(age.Hours()/(24*30)), "month")
	}
}

// pluralUnit renders a count and its unit, so one minute does not read as "1
// minutes".
func pluralUnit(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

// --- Your trainer ----------------------------------------------------------

// trainerPanelLabel names the main menu's "what have I gained" panel. The main
// menu asks what is about to happen, and this panel answers the part of that
// question that is about the player rather than the machine.
const trainerPanelLabel = "Your trainer"

// trainerPanelFacts is the trainer panel's rows without the header that names
// the panel. It reads only what the trainer already persisted, through the same
// read-only accessors the trainer menu uses, so it neither manufactures the
// empty module records a later save would persist nor disagrees with the menu
// about what the player has done.
//
// When there is no record of any run, it renders one line saying so instead of a
// column of zeros: zero lessons, a zero percent and a zero streak would all read
// as measured progress when nothing has been played.
func (m Model) trainerPanelFacts(l layout) []string {
	stats := m.TrainerStats
	if !trainerHasRuns(stats) {
		line := wrapText("No runs recorded yet — the Vim Trainer is on the main menu.", l.Right, 1)
		if len(line) == 0 {
			return nil
		}
		return []string{MutedStyle.Render(line[0])}
	}

	modules := m.trainerModuleList()
	var rows []string
	for _, mod := range modules {
		progress := stats.ModuleProgress[mod.ID]
		if !trainerModuleStarted(progress) {
			continue
		}
		practice := trainer.GetPracticeStatsForModule(mod.ID, progress)
		rows = append(rows, panelFact(mod.Name, trainerModuleSummary(progress, practice), l.Right)...)
	}

	if accuracy := trainerOverallAccuracy(stats); accuracy != "" {
		rows = append(rows, panelFact("Accuracy", accuracy, l.Right)...)
	}
	if stats.BestStreak > 0 {
		rows = append(rows, panelFact("Best streak", strconv.Itoa(stats.BestStreak), l.Right)...)
	}
	if boss, ok := trainerNextBoss(stats, modules); ok {
		rows = append(rows, panelFact("Next boss", boss.BossName, l.Right)...)
		rows = append(rows, panelFact("Needs", trainerBossRequirement(stats, boss), l.Right)...)
	} else {
		rows = append(rows, panelFact("Bosses", "all cleared", l.Right)...)
	}
	return rows
}

// trainerHeadline is the trainer panel said in one short line for the narrow
// terminal's summary: the overall accuracy and the best streak, or the one line
// that says nothing has been played yet. It carries no number the panel would not
// show, so a narrow terminal and a wide one cannot disagree about the player.
func (m Model) trainerHeadline() string {
	stats := m.TrainerStats
	if !trainerHasRuns(stats) {
		return "no runs recorded yet"
	}
	var parts []string
	if accuracy := trainerOverallAccuracy(stats); accuracy != "" {
		parts = append(parts, "accuracy "+accuracy)
	}
	if stats.BestStreak > 0 {
		parts = append(parts, fmt.Sprintf("best streak %d", stats.BestStreak))
	}
	if len(parts) == 0 {
		return "progress recorded"
	}
	return strings.Join(parts, panelTabSeparator)
}

// trainerModuleList is the module order the panel walks: the model's own list
// when it has one, and the trainer's catalogue otherwise, so a hand-built model
// in a test still gets the shipped order instead of an empty panel.
func (m Model) trainerModuleList() []trainer.ModuleInfo {
	if len(m.TrainerModules) > 0 {
		return m.TrainerModules
	}
	return trainer.GetAllModules()
}

// trainerHasRuns reports whether the persisted stats record any run at all. A
// nil profile -- no file, an unreadable one or a corrupt one -- records none, and
// so does a file that was saved before anything was played: every counter is
// zero, so the panel says so in one line rather than drawing that zero as
// progress. It is the single predicate the panel and its headline share.
func trainerHasRuns(stats *trainer.UserStats) bool {
	if stats == nil {
		return false
	}
	if stats.TotalScore > 0 || stats.BestStreak > 0 || !stats.LastPlayed.IsZero() || len(stats.BossesDefeated) > 0 {
		return true
	}
	for _, progress := range stats.ModuleProgress {
		if trainerModuleStarted(progress) {
			return true
		}
	}
	return false
}

// trainerModuleStarted reports whether a module has a recorded run behind it. It
// reads the recorded counters and never creates an entry, so a module the player
// never opened is left out of the panel instead of appearing as "0/5".
func trainerModuleStarted(progress *trainer.ModuleProgress) bool {
	if progress == nil {
		return false
	}
	return progress.LessonsCompleted > 0 || progress.PracticeAttempts > 0 ||
		progress.BossAttempts > 0 || progress.BossDefeated || len(progress.ExerciseStats) > 0
}

// trainerModuleSummary is one module's lessons and mastery as the panel says
// them, with the boss marked when it is cleared. The lesson total falls back to
// the module's real exercise count when the module was opened without recording
// it -- the same fallback the trainer menu uses -- so a started module never
// shows "0/0" beside a mastery count that already knows the total.
func trainerModuleSummary(progress *trainer.ModuleProgress, practice trainer.PracticeStats) string {
	lessonsTotal := progress.LessonsTotal
	if lessonsTotal == 0 {
		lessonsTotal = practice.TotalExercises
	}
	parts := []string{
		fmt.Sprintf("%d/%d lessons", progress.LessonsCompleted, lessonsTotal),
		fmt.Sprintf("%d/%d mastered", practice.MasteredCount, practice.TotalExercises),
	}
	if progress.BossDefeated {
		parts = append(parts, "boss ✓")
	}
	return strings.Join(parts, panelTabSeparator)
}

// trainerOverallAccuracy is the accuracy across every module with recorded
// attempts, or "" when nothing has been attempted. It is read from the recorded
// counters rather than from a stored average, so it cannot drift from them, and
// it is left out entirely rather than reported as 0% when there is no data.
func trainerOverallAccuracy(stats *trainer.UserStats) string {
	attempts, correct := 0, 0
	for _, progress := range stats.ModuleProgress {
		if progress == nil {
			continue
		}
		attempts += progress.PracticeAttempts
		correct += progress.PracticeCorrect
	}
	if attempts == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", float64(correct)/float64(attempts)*100)
}

// trainerNextBoss is the next boss the player can fight: the first unlocked
// module whose boss is not cleared, in the catalogue's own order. It returns
// false when every boss has been beaten, so the panel names the bosses cleared
// instead of pointing at a fight that no longer exists.
func trainerNextBoss(stats *trainer.UserStats, modules []trainer.ModuleInfo) (trainer.ModuleInfo, bool) {
	for _, mod := range modules {
		if stats.IsModuleUnlocked(mod.ID) && !stats.IsBossDefeated(mod.ID) {
			return mod, true
		}
	}
	return trainer.ModuleInfo{}, false
}

// trainerBossRequirement says what the next boss needs: the practice gate the
// trainer itself enforces (80% accuracy over at least ten attempts), with where
// the player stands now, so the panel answers "what does it take" instead of only
// naming the fight. A boss already through its gate reads "ready".
func trainerBossRequirement(stats *trainer.UserStats, mod trainer.ModuleInfo) string {
	if stats.IsBossReady(mod.ID) {
		return "ready"
	}
	accuracy, attempts := 0.0, 0
	if progress := stats.ModuleProgressOrNil(mod.ID); progress != nil {
		accuracy = progress.PracticeAccuracy * 100
		attempts = progress.PracticeAttempts
	}
	return fmt.Sprintf("80%% practice accuracy and 10 tries (at %.0f%%, %d)", accuracy, attempts)
}

// --- Did you know? ---------------------------------------------------------

// tipPanelLabel names the tip panel. The question it answers is "what can I
// learn right now", and it answers it with one fact at a time.
const tipPanelLabel = "Did you know?"

// The tip's shape is fixed so the rotation cannot make a screen jump: one dim
// context row naming where the tip comes from, then at most tipContentRows rows
// carrying the keys and what they do. Ten seconds is how long each tip stays,
// derived from the frame rate as animTicksPerSecond * 10 frames so the rotation
// keeps its ten seconds wherever the rate moves, and it is a named constant so
// the rotation, the tests that name tip 0 and tip 1 and the documentation cannot
// disagree.
const (
	ticksPerTip    = animTicksPerSecond * 10
	tipContentRows = 2
	tipKeyGap      = 2
)

// tipMinPanelWidth is the narrowest right column a two-column screen can offer:
// at the 124-terminal-column floor the content width is 120 and the panel is 62
// columns. A trainer tip is admitted only when all of its content fits that
// column, so a shipped tip is whole at every width the layout composes and the
// renderer's cap is a safety net that never fires.
const tipMinPanelWidth = 62

// tip is one "Did you know?" item: where it comes from, the keys as key tokens,
// and what those keys do. It is data, not a renderer, so the pool can be built,
// counted and named by a test without going through the screen.
type tip struct {
	Context string
	Keys    string
	Action  string
}

// tipPoolOnce is the pool built lazily once. The pool is built from static
// content and a small amount of arithmetic, so caching it costs nothing and keeps
// the render path from rebuilding several hundred entries on every tick.
var (
	tipPoolOnce  sync.Once
	tipPoolCache []tip
)

// buildTipPool flattens the content the repository already ships into the tip
// pool, in a declared order and with no randomness: the keymap reference data
// first -- Neovim, Tmux, Zellij, Ghostty, Herdr, each in its own declared order --
// then the trainer's own lessons, in module order and lesson order. Two runs on
// the same machine therefore walk the same sequence, and a test can name tip 0
// and tip 1 by the source they came from.
func buildTipPool() []tip {
	sources := []struct {
		label string
		cats  []KeymapCategory
	}{
		{"Neovim", GetNvimKeymaps()},
		{"Tmux", GetTmuxKeymaps()},
		{"Zellij", GetZellijKeymaps()},
		{"Ghostty", GetGhosttyKeymaps()},
		{"Herdr", GetHerdrKeymaps()},
	}

	var pool []tip
	for _, src := range sources {
		for _, cat := range src.cats {
			for _, km := range cat.Keymaps {
				pool = append(pool, tip{
					Context: src.label + panelTabSeparator + cat.Name,
					Keys:    km.Keys,
					Action:  km.Description,
				})
			}
		}
	}

	for _, mod := range trainer.GetAllModules() {
		for _, exercise := range trainer.GetLessons(mod.ID) {
			t := tip{
				Context: "Vim Trainer" + panelTabSeparator + mod.Name,
				Keys:    exercise.Optimal,
				Action:  firstSentence(exercise.Mission),
			}
			// An exercise whose mission does not fit the tip's content rows is left
			// out rather than admitted and cut: a tip that ends mid-sentence says less
			// than none, and a pool that only ever holds whole tips cannot show one.
			if tipContentOverflows(t, tipMinPanelWidth) {
				continue
			}
			pool = append(pool, t)
		}
	}
	return pool
}

// tipContentOverflows reports whether a tip's action needs more than the tip's
// content rows at width, which is exactly the case the renderer would end with
// the cut marker. The pool uses it to keep only whole tips; the renderer keeps
// its own cap so a future tip that slips through is still cut visibly rather
// than silently.
func tipContentOverflows(t tip, width int) bool {
	if width < 1 {
		width = 1
	}
	if t.Keys == "" {
		return len(wrapText(t.Action, width, 0)) > tipContentRows
	}
	keyWidth := lipgloss.Width(t.Keys)
	if keyWidth+tipKeyGap+8 <= width {
		return len(wrapText(t.Action, width-keyWidth-tipKeyGap, 0)) > tipContentRows
	}
	return len(wrapText(t.Action, width, 0)) > tipContentRows-1
}

// tipPool returns the shipped pool. It is the one place the pool is read, so the
// panel, the headline and a test all walk the same sequence.
func tipPool() []tip {
	tipPoolOnce.Do(func() { tipPoolCache = buildTipPool() })
	return tipPoolCache
}

// tipIndex is the tip the clock selects: the pool is walked one tip per
// ticksPerTip ticks, so tip 0 holds for the first ten seconds, tip 1 the next
// ten, and the sequence wraps at the end. With animation off AnimTick never
// leaves zero, so the screen stays on tip 0; a negative counter, which only a
// hand-built model can carry, is treated as zero rather than indexed backwards.
func tipIndex(m Model) int {
	pool := tipPool()
	if len(pool) == 0 {
		return -1
	}
	tick := max(m.AnimTick, 0)
	return (tick / ticksPerTip) % len(pool)
}

// firstSentence returns the first sentence of s, so a trainer tip says one thing
// instead of a paragraph. The pool then admits only the exercises whose sentence
// fits the tip's rows; the renderer's own cap remains a safety net for a tip that
// slips through.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}

// tipFacts is the tip panel's rows: the tip the tick selects, rendered as the
// two or three rows a tip is allowed.
func (m Model) tipFacts(l layout) []string {
	index := tipIndex(m)
	if index < 0 {
		return nil
	}
	return tipRows(tipPool()[index], l.Right)
}

// tipHeadline is the current tip said in one line for the narrow terminal's
// summary. It is the keys and their action, which is what the panel shows.
func (m Model) tipHeadline() string {
	index := tipIndex(m)
	if index < 0 {
		return ""
	}
	t := tipPool()[index]
	if t.Keys == "" {
		return t.Action
	}
	return t.Keys + " " + t.Action
}

// tipRows renders one tip as at most three rows: the context in the dim tone,
// then the keys as key tokens beside their action in ink. The keys and the action
// share the first content row while the keys leave room for words; otherwise the
// keys take a row of their own so neither is cut to fit the other. Both halves are
// wrapped and capped by the shared marker rather than cut silently, so a tip can
// never grow past the rows a tip is promised.
func tipRows(t tip, width int) []string {
	if width < 1 {
		width = 1
	}

	var rows []string
	if context := wrapText(t.Context, width, 1); len(context) > 0 {
		rows = append(rows, MutedStyle.Render(context[0]))
	}

	if t.Keys == "" {
		for _, line := range wrapText(t.Action, width, tipContentRows) {
			rows = append(rows, InkStyle.Render(line))
		}
		return rows
	}

	keyWidth := lipgloss.Width(t.Keys)
	if keyWidth+tipKeyGap+8 <= width {
		head := AccentKeyStyle.Render(t.Keys) + strings.Repeat(" ", tipKeyGap)
		body := wrapText(t.Action, width-keyWidth-tipKeyGap, tipContentRows)
		if len(body) == 0 {
			return append(rows, strings.TrimRight(head, " "))
		}
		rows = append(rows, head+InkStyle.Render(body[0]))
		indent := strings.Repeat(" ", keyWidth+tipKeyGap)
		for _, line := range body[1:] {
			rows = append(rows, indent+InkStyle.Render(line))
		}
		return rows
	}

	keys := wrapText(t.Keys, width, 1)
	if len(keys) == 0 {
		return rows
	}
	rows = append(rows, AccentKeyStyle.Render(keys[0]))
	for _, line := range wrapText(t.Action, width, tipContentRows-1) {
		rows = append(rows, InkStyle.Render(line))
	}
	return rows
}

// --- Last install ----------------------------------------------------------

// lastInstallPanelLabel names the state panel: when this machine was last
// installed, and by which build.
const lastInstallPanelLabel = "Last install"

// lastInstallPanelFacts is the state panel's rows without the header that names
// the panel: when the run finished, the build that produced it, and the
// configuration paths it touched. The files row is left out when the run had
// nothing to overwrite, so the panel reports what it knows and no zero in place
// of a fact.
func (m Model) lastInstallPanelFacts(l layout) []string {
	record := m.LastInstall
	if record == nil {
		return nil
	}

	rows := panelFact("Ran", record.Timestamp.Format(panelBackupStamp), l.Right)
	rows = append(rows, panelFact("Version", record.Version, l.Right)...)
	if len(record.Files) > 0 {
		rows = append(rows, panelFact("Touched", fileCount(len(record.Files)), l.Right)...)
		for _, file := range record.Files {
			rows = append(rows, panelFact("", file, l.Right)...)
		}
	}
	return rows
}

// lastInstallHeadline is the state panel said in one line for the narrow
// terminal's summary: when the run finished and, when the run touched anything,
// how many configuration paths it replaced.
func (m Model) lastInstallHeadline() string {
	if m.LastInstall == nil {
		return ""
	}
	when := m.LastInstall.Timestamp.Format(panelBackupStamp)
	if len(m.LastInstall.Files) == 0 {
		return when
	}
	return when + panelTabSeparator + fileCount(len(m.LastInstall.Files)) + " touched"
}
