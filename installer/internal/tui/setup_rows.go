package tui

import (
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// setupRowKind says what a row's value is: a list the left and right arrows
// cycle, a switch the space bar toggles, or the action row that leaves the
// screen. It is declared here rather than inferred from the screen because the
// backup decision is a switch with no screen of its own.
type setupRowKind int

const (
	setupRowCycle setupRowKind = iota
	setupRowSwitch
	setupRowAction
	// setupRowPluginSet is the row of a plugin set the tool in the row above installs.
	// It is not a decision of its own: it is work that tool's step does, named beside
	// the tool so that a set a run would install cannot stay off the screen.
	setupRowPluginSet
)

// setupRow is one decision of the setup screen: what it is called, which of the
// wizard's question screens opens it as a detour, and whether its value is a
// list or a switch. A row carries no option strings and no way to record an
// answer: both come from the question screen's own helpers (model.go's
// *Options functions and applyQuestionAnswer), so a row and its detour cannot
// disagree about what may be chosen or what a choice records.
type setupRow struct {
	Label  string
	Screen Screen
	Kind   setupRowKind
	// Visible reports whether the row is offered for this model. It is nil for a
	// row that is always offered; the backup decision is the one that is not,
	// because there is nothing to decide when the machine holds no configuration
	// the run would replace.
	Visible func(m Model) bool
	// SetID is the registry ID of the plugin set this row stands for, and empty for
	// every other row. The row carries the ID rather than a state: what the set holds,
	// whether a run can leave it out and why are the registry's answers, so the row
	// cannot drift from the plan or from --skip.
	SetID string
}

// setupRowTable is every decision of the flow, in the order the decisions are
// taken. It is the one declaration of the row list: the renderer, the keyboard
// and the guards all read it, so a row added here is drawn, reachable and
// measured without a second place to remember.
var setupRowTable = []setupRow{
	{Label: "Platform", Screen: ScreenOSSelect, Kind: setupRowCycle},
	{Label: "Shell", Screen: ScreenShellSelect, Kind: setupRowCycle},
	{Label: "Terminal", Screen: ScreenTerminalSelect, Kind: setupRowCycle},
	{Label: "Font", Screen: ScreenFontSelect, Kind: setupRowSwitch},
	{Label: "Multiplexer", Screen: ScreenWMSelect, Kind: setupRowCycle},
	{Label: "Editor (Neovim)", Screen: ScreenNvimSelect, Kind: setupRowSwitch},
	{
		Label:  "Existing configurations",
		Screen: ScreenBackupConfirm,
		Kind:   setupRowSwitch,
		Visible: func(m Model) bool {
			return len(m.ExistingConfigs) > 0
		},
	},
	{Label: "Continue to the plan", Kind: setupRowAction},
}

// setupRows is the table as this model shows it: every row whose Visible admits
// the model, in the table's order. It is the list the cursor indexes, the list
// the renderer draws and the list the guards read, so a hidden row can never be
// acted on by a key that cannot see it.
func (m Model) setupRows() []setupRow {
	rows := make([]setupRow, 0, len(setupRowTable))
	for _, row := range setupRowTable {
		if row.Visible != nil && !row.Visible(m) {
			continue
		}
		rows = append(rows, row)
		// The set this tool's step installs is named directly under it, so the screen
		// reads as one list of decisions with the work that follows from each.
		if set, ok := m.pluginSetForRow(row); ok {
			rows = append(rows, setupRow{Label: set.Label(), Kind: setupRowPluginSet, SetID: set.ID})
		}
	}
	return rows
}

// pluginSetStepOfScreen names the step whose tool this question screen chooses, or
// the empty string when the screen chooses no tool. It is how the row list asks the
// registry which set a row carries without writing the step IDs a second time.
func pluginSetStepOfScreen(screen Screen) string {
	switch screen {
	case ScreenShellSelect:
		return "shell"
	case ScreenWMSelect:
		return "wm"
	case ScreenNvimSelect:
		return "nvim"
	}
	return ""
}

// pluginSetForRow is the set the choice in this row installs, when the registry holds
// one. It asks the registry the same question the plan asks, with the same chosen-tool
// test, so a row and a plan step cannot disagree about which set a tool carries; a tool
// with no registered set contributes no row.
func (m Model) pluginSetForRow(row setupRow) (pluginSet, bool) {
	step := pluginSetStepOfScreen(row.Screen)
	if step == "" {
		return pluginSet{}, false
	}
	opts := m.planOptions()
	for _, set := range pluginSets {
		if set.Step == step && pluginSetToolChosen(set, opts) {
			return set, true
		}
	}
	return pluginSet{}, false
}

// rowLabel is the label as the list draws it. A set's row is indented under the tool
// that installs it, because the set is that tool's work and not a decision of its own;
// the option list keeps the plain label, which names decisions.
func (r setupRow) rowLabel() string {
	if r.Kind == setupRowPluginSet {
		return "  " + r.Label
	}
	return r.Label
}

// setupRowLabels is the row list without the values, which is what
// GetCurrentOptions answers for the setup screen. The values are the renderer's
// business: they are state, and the option list is the set of decisions.
func (m Model) setupRowLabels() []string {
	rows := m.setupRows()
	labels := make([]string, len(rows))
	for i, row := range rows {
		labels[i] = row.Label
	}
	return labels
}

// questionValueOptions is the answers a question screen offers, without the
// separator and the "Learn about ..." rows the rich screen adds. The setup row
// cycles exactly this list and the detour screen's menu is built from it, so the
// two are the same list seen in two places.
func (m Model) questionValueOptions(screen Screen) []string {
	switch screen {
	case ScreenOSSelect:
		return m.osOptions()
	case ScreenTerminalSelect:
		return m.terminalOptions()
	case ScreenFontSelect:
		return m.fontOptions()
	case ScreenShellSelect:
		return m.shellOptions()
	case ScreenWMSelect:
		return m.wmOptions()
	case ScreenNvimSelect:
		return m.nvimOptions()
	case ScreenBackupConfirm:
		return m.backupOptions()
	}
	return nil
}

// questionAnswerKey is the answer an option records, in the vocabulary the
// choices hold: "mac"/"linux"/"termux" for the platform, a tool key for the
// tools, and "on"/"off" for the two questions that are really switches. It is
// read from the option label the same way the question screen's own handler
// reads it, so a row can find the option its recorded answer is on without a
// second copy of the label list.
func questionAnswerKey(screen Screen, option string) string {
	name := strings.ToLower(plainOptionName(option))
	switch screen {
	case ScreenOSSelect:
		switch {
		case strings.HasPrefix(name, "mac"):
			return "mac"
		case strings.HasPrefix(name, "termux"):
			return "termux"
		default:
			return "linux"
		}
	case ScreenTerminalSelect, ScreenWMSelect:
		fields := strings.Fields(name)
		if len(fields) == 0 {
			return ""
		}
		return fields[0]
	case ScreenShellSelect:
		return name
	case ScreenFontSelect:
		if strings.HasPrefix(name, "install") {
			return "on"
		}
		return "off"
	case ScreenNvimSelect:
		if strings.HasPrefix(name, "yes") {
			return "on"
		}
		return "off"
	case ScreenBackupConfirm:
		if strings.HasPrefix(name, "install without") {
			return "off"
		}
		return "on"
	}
	return ""
}

// questionRecordedAnswer is the answer the choices hold for a question, in the
// same vocabulary questionAnswerKey answers in. The tool answers are the choice
// strings themselves, and the two switch questions read "on"/"off".
func (m Model) questionRecordedAnswer(screen Screen) string {
	switch screen {
	case ScreenOSSelect:
		return m.Choices.OS
	case ScreenTerminalSelect:
		return m.Choices.Terminal
	case ScreenFontSelect:
		if m.Choices.InstallFont {
			return "on"
		}
		return "off"
	case ScreenShellSelect:
		return m.Choices.Shell
	case ScreenWMSelect:
		return m.Choices.WindowMgr
	case ScreenNvimSelect:
		if m.Choices.InstallNvim {
			return "on"
		}
		return "off"
	case ScreenBackupConfirm:
		if m.Choices.CreateBackup {
			return "on"
		}
		return "off"
	}
	return ""
}

// questionValueIndex is the option the recorded answer is on, or -1 when the
// choices hold no answer for the question yet.
func (m Model) questionValueIndex(screen Screen) int {
	answer := m.questionRecordedAnswer(screen)
	if answer == "" {
		return -1
	}
	for i, option := range m.questionValueOptions(screen) {
		if questionAnswerKey(screen, option) == answer {
			return i
		}
	}
	return -1
}

// applyQuestionAnswer records the answer an option carries. It is the one place
// a question's answer is written, shared by the rich screen's Enter and the
// setup row's cycle, so the two cannot record different things for the same
// label.
func (m *Model) applyQuestionAnswer(screen Screen, option string) {
	answer := questionAnswerKey(screen, option)
	switch screen {
	case ScreenOSSelect:
		m.Choices.OS = answer
		// The platform forces the answers it cannot ask for; keeping that in one
		// place is what makes the rows show them instead of leaving them unsaid.
		m.applyForcedPlatformAnswers()
	case ScreenTerminalSelect:
		m.Choices.Terminal = answer
	case ScreenFontSelect:
		m.Choices.InstallFont = answer == "on"
	case ScreenShellSelect:
		m.Choices.Shell = answer
	case ScreenWMSelect:
		m.Choices.WindowMgr = answer
	case ScreenNvimSelect:
		m.Choices.InstallNvim = answer == "on"
	case ScreenBackupConfirm:
		m.Choices.CreateBackup = answer == "on"
	}
}

// applyForcedPlatformAnswers records the answers the platform forces. Termux has
// no terminal emulator of its own to install and takes the Nerd Font; on WSL the
// terminal belongs to the Windows host. It is the one place those values are
// written, called when the setup screen is entered and whenever the Platform row
// changes.
func (m *Model) applyForcedPlatformAnswers() {
	isWSL := m.SystemInfo != nil && m.SystemInfo.IsWSL
	switch {
	case m.Choices.OS == "termux":
		m.Choices.Terminal = "none"
		m.Choices.InstallFont = true
	case isWSL:
		m.Choices.Terminal = "none"
		m.Choices.InstallFont = false
	}
}

// setupRowIsOn reports whether a switch row is on. A switch reads the question it
// opens, which is how the font, editor and backup rows report the answer they
// record; the branch for a screen-less switch is what a row owning a choice
// directly would need.
func (m Model) setupRowIsOn(row setupRow) bool {
	if row.Kind != setupRowSwitch {
		return false
	}
	if row.Screen == 0 {
		return m.Choices.CreateBackup
	}
	return m.questionRecordedAnswer(row.Screen) == "on"
}

// setupRowValue is the value a row carries beside its label. A question row
// shows the option the recorded answer is on, so the row and the plan cannot
// disagree; a switch row shows its state in words; and the action row has no
// value because pressing it decides nothing.
func (m Model) setupRowValue(row setupRow) string {
	switch {
	case row.Kind == setupRowAction:
		return ""
	case row.Screen == ScreenFontSelect, row.Screen == ScreenNvimSelect:
		if m.setupRowIsOn(row) {
			return "[x] install"
		}
		return "[ ] skip"
	case row.Kind == setupRowSwitch:
		if m.setupRowIsOn(row) {
			return "[x] back up"
		}
		return "[ ] install anyway"
	case row.Kind == setupRowPluginSet:
		if !pluginSetIsSwitchable(row.SetID) {
			// A set a run cannot leave out says so where its value would stand, and the
			// plan says where it comes from: a switch here would promise to leave out
			// work this run cannot leave out.
			return "[x] always installed"
		}
		if pluginSetIsOff(m.Choices.DisabledPluginSets, row.SetID) {
			return "[ ] leave it out"
		}
		return "[x] install"
	}
	options := m.questionValueOptions(row.Screen)
	if index := m.questionValueIndex(row.Screen); index >= 0 && index < len(options) {
		return options[index]
	}
	return "Not chosen"
}

// cycleSetupRow moves the focused row's value one step in place. A cycle row
// steps through its question's options and wraps; a switch row flips, because a
// switch has only the two states the arrows can cycle. It reports whether it
// changed anything.
func (m *Model) cycleSetupRow(delta int) bool {
	rows := m.setupRows()
	if m.SetupCursor < 0 || m.SetupCursor >= len(rows) {
		return false
	}
	row := rows[m.SetupCursor]
	if row.Kind == setupRowSwitch || row.Kind == setupRowPluginSet {
		return m.toggleSetupRow()
	}
	if row.Kind != setupRowCycle {
		return false
	}
	options := m.questionValueOptions(row.Screen)
	if len(options) == 0 {
		return false
	}
	next := m.questionValueIndex(row.Screen)
	if next < 0 {
		if delta < 0 {
			next = len(options) - 1
		} else {
			next = 0
		}
	} else {
		next = (next + delta + len(options)) % len(options)
	}
	m.applyQuestionAnswer(row.Screen, options[next])
	return true
}

// toggleSetupRow flips the focused switch row. The two question-backed switches
// record the same answer the rich screen would: the first option means on and
// the second means off, read from the question's own option list.
func (m *Model) toggleSetupRow() bool {
	rows := m.setupRows()
	if m.SetupCursor < 0 || m.SetupCursor >= len(rows) {
		return false
	}
	row := rows[m.SetupCursor]
	if row.Kind == setupRowPluginSet {
		// The set's row carries the same switch the flag carries: the two write the
		// list the plan reads, so a set turned off here is off in --skip too.
		if !pluginSetIsSwitchable(row.SetID) {
			return false
		}
		m.Choices.DisabledPluginSets = togglePluginSet(m.Choices.DisabledPluginSets, row.SetID)
		return true
	}
	if row.Kind != setupRowSwitch {
		return false
	}
	if row.Screen == 0 {
		m.Choices.CreateBackup = !m.Choices.CreateBackup
		return true
	}
	options := m.questionValueOptions(row.Screen)
	if len(options) < 2 {
		return false
	}
	next := 0
	if m.setupRowIsOn(row) {
		next = 1
	}
	m.applyQuestionAnswer(row.Screen, options[next])
	return true
}

// openSetupRow is the setup screen's Enter: it opens the focused row. A row with
// a rich screen becomes a detour that remembers the setup screen as where it
// came from; the action row detects the configuration this run would replace and
// opens the plan; and a switch with no screen of its own flips.
func (m *Model) openSetupRow() {
	rows := m.setupRows()
	if m.SetupCursor < 0 || m.SetupCursor >= len(rows) {
		return
	}
	row := rows[m.SetupCursor]
	switch {
	case row.Kind == setupRowAction:
		m.finishSetup()
	case row.Screen != 0:
		m.SetupDetour = true
		m.PrevScreen = ScreenSetup
		m.Screen = row.Screen
		// The detour opens on the answer already recorded, and with nothing
		// highlighted when the choices hold none: a row under the cursor reads as
		// the installer's answer, and no answer has been given yet.
		m.Cursor = m.questionValueIndex(row.Screen)
	default:
		m.toggleSetupRow()
	}
}

// detectExistingConfigsForSetup is the one place the setup screen takes the
// machine's existing configuration. It runs when the screen is entered, so the
// backup row can be offered, and again when the action row leaves for the plan,
// so the plan is built from what is on the machine at that moment.
func (m *Model) detectExistingConfigsForSetup() {
	m.ExistingConfigs = system.DetectExistingConfigs()
}

// enterSetup opens the setup screen: the one place the wizard starts asking, and
// the one place the platform's forced answers are applied. It records the
// detected platform and the detected shell while nothing has chosen them, so
// every row carries the value the choices hold rather than leaving a question
// that will never be asked. It is enterShellSelect promoted: that function used
// to open the shell question on the detected shell, and this opens the screen
// that question now lives on.
func (m *Model) enterSetup() {
	m.Screen = ScreenSetup
	m.SetupCursor = 0
	m.SetupDetour = false
	m.detectExistingConfigsForSetup()
	if m.SystemInfo != nil {
		if _, chosen := osChoiceOptionIndex(m.Choices.OS); !chosen {
			if index := m.osCursor(); index >= 0 {
				m.applyQuestionAnswer(ScreenOSSelect, m.osOptions()[index])
			}
		}
		m.applyForcedPlatformAnswers()
		if m.Choices.Shell == "" {
			if index := m.shellCursor(); index >= 0 {
				m.applyQuestionAnswer(ScreenShellSelect, m.shellOptions()[index])
			}
		}
	}
	// The backup is the recommended answer whenever there is something to back
	// up: the old confirmation opened on it. The row shows it, so it is a default
	// on screen rather than a decision taken behind the user's back.
	if len(m.ExistingConfigs) > 0 {
		m.Choices.CreateBackup = true
	}
}

// finishSetup is the action row: it detects the configuration the run would
// replace and opens the plan, exactly where the flow's last question used to
// land. The backup decision is a row on the setup screen now, so the plan
// follows the setup screen directly.
func (m *Model) finishSetup() {
	m.detectExistingConfigsForSetup()
	m.openInstallReview()
}

// returnToSetup is where a detour reports back: the choice was recorded by the
// detour's own handler and the flow is the setup screen again, with the cursor
// still on the row the detour was opened from. It changes no answer, which is
// what makes Escape and Enter share it.
func (m *Model) returnToSetup() {
	m.Screen = ScreenSetup
	m.SetupDetour = false
	m.Cursor = 0
}

// leaveSetup is the setup screen's Escape: it leaves for the main menu and
// forgets the answers, the way the old first question's Escape did.
func (m *Model) leaveSetup() {
	m.Screen = ScreenMainMenu
	m.Cursor = 0
	m.SetupCursor = 0
	m.SetupDetour = false
	m.Choices = UserChoices{}
}

// handleSetupKeys drives the setup screen: the arrows move and cycle, Space
// toggles the switches, Enter opens the focused row's screen or continues to the
// plan, and Escape leaves for the main menu. Every value key acts on the row the
// cursor is on in the same filtered list the renderer drew.
func (m Model) handleSetupKeys(key string) (tea.Model, tea.Cmd) {
	rows := m.setupRows()

	switch key {
	case "up", "k":
		if m.SetupCursor > 0 {
			m.SetupCursor--
		}
	case "down", "j":
		if m.SetupCursor < len(rows)-1 {
			m.SetupCursor++
		}
	case "left", "h":
		m.cycleSetupRow(-1)
	case "right", "l":
		m.cycleSetupRow(1)
	case " ":
		m.toggleSetupRow()
	case "enter":
		m.openSetupRow()
	case "esc":
		m.leaveSetup()
	}

	return m, nil
}
