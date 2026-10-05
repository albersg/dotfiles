package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
)

// Messages
type (
	// tickMsg is sent periodically for animations
	tickMsg time.Time

	// installStartMsg signals to start installation
	installStartMsg struct{}

	// stepCompleteMsg signals a step completed
	stepCompleteMsg struct {
		stepID string
		err    error
		// recorded is the state the step left for the steps after it. The step
		// loop always fills it; it is nil for a message built by hand.
		recorded *stepRecordedState
	}

	// stepProgressMsg updates progress of current step
	stepProgressMsg struct {
		stepID   string
		progress float64
		log      string
	}

	// installCompleteMsg signals all installation is done
	installCompleteMsg struct {
		totalTime float64
	}

	// loadBackupsMsg signals to load available backups
	loadBackupsMsg struct {
		backups []system.BackupInfo
	}

	// dotfilesThemesLoadedMsg carries the theme definitions read from the
	// repository checkout onto the model.
	dotfilesThemesLoadedMsg struct {
		themes []themeDefinition
		err    error
	}

	// dotfilesThemeChangedMsg is the result of applying or undoing the dotfiles
	// theme. A failure stays on the section as a notice, like the desktop switch.
	dotfilesThemeChangedMsg struct {
		notice string
		err    error
	}

	// trainerStatsLoadedMsg carries the trainer's persisted stats onto the startup
	// path, so the main menu's "Your trainer" panel can describe real progress
	// before the player has opened the trainer.
	trainerStatsLoadedMsg struct {
		stats *trainer.UserStats
	}

	// lastInstallLoadedMsg carries the record of the previous completed run onto
	// the startup path. A nil record means there is no file, and the main menu
	// then offers no "Last install" panel.
	lastInstallLoadedMsg struct {
		record *lastInstall
	}

	// configsDetectedMsg carries the existing configs the startup scan found. It
	// arrives once, from Init, so the main menu's plan can say what the run will
	// overwrite before the wizard has asked anything.
	configsDetectedMsg struct {
		configs []string
	}

	// execFinishedMsg signals an interactive process finished
	execFinishedMsg struct {
		stepID string
		err    error
	}

	// themeRecordLoadedMsg carries the setting this installer last replaced onto
	// the startup path, the way the last-install record is loaded. It is loaded
	// here rather than built into the model so a snapshot can pin the state
	// directory and see no record on any host.
	themeRecordLoadedMsg struct {
		record *themeRecord
		// dotfiles is the dotfiles-theme half of the same record, read at the same
		// time so the section can offer its undo without a second file read.
		dotfiles *dotfilesThemeRecord
	}

	// themeChangedMsg carries the result of a switch or an undo back to Update,
	// which is the only place the section's notice and record are written. The
	// command runs off the update loop, the way every external command does.
	themeChangedMsg struct {
		record *themeRecord
		notice string
		err    error
	}
)

// stepRecordedState is the state one installation step records for the steps
// that run after it. The clone step's checkout is the one that matters: every
// later step reads it through repoDir(), and cleanup is what removes it.
//
// A step runs inside a tea.Cmd, after Update has already returned the model it
// keeps, so a step cannot write to that model directly. It runs against a copy
// and hands what it recorded back in stepCompleteMsg, and Update copies these
// fields into the model it returns. Only these fields travel that way: every
// other Model field belongs to the UI, which keeps changing while the step runs
// and must not be rolled back to the step's snapshot. A step that starts
// recording new state must add the field here.
type stepRecordedState struct {
	WorkDir   string
	RepoDir   string
	BackupDir string
}

// recordedState takes the state a step recorded off the model it ran against.
func (m Model) recordedState() stepRecordedState {
	return stepRecordedState{
		WorkDir:   m.WorkDir,
		RepoDir:   m.RepoDir,
		BackupDir: m.BackupDir,
	}
}

// applyTo copies the recorded state into the model the TUI keeps.
func (r stepRecordedState) applyTo(m *Model) {
	m.WorkDir = r.WorkDir
	m.RepoDir = r.RepoDir
	m.BackupDir = r.BackupDir
}

// recordLastInstall writes the record of a completed run and puts it on the
// model, so the "Last install" panel shows the run that just finished without
// waiting for the next startup read. The file write is deliberately unchecked:
// it is a convenience for the next run, and a read-only or missing state
// directory must not fail an install that already succeeded.
func (m *Model) recordLastInstall() {
	rec := lastInstall{
		Timestamp: time.Now(),
		Version:   VersionLabel(),
		Files:     append([]string(nil), m.ExistingConfigs...),
	}
	m.LastInstall = &rec
	_ = writeLastInstall(rec)
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle("dotfiles Installer"),
		tickCmd(),
		loadBackupsCmd(),
		detectConfigsCmd(),
		loadTrainerStatsCmd(),
		loadLastInstallCmd(),
		loadThemeRecordCmd(),
	}
	// The slow animation tick is armed only when the run may animate. With
	// animation off nothing is scheduled and the counter stays at zero.
	if cmd := m.animTickCmdFor(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// The host is read on the same gate: a run that may not animate also does not
	// sample the machine, so its live panel reports the sampling as off rather
	// than drawing a stale reading.
	if cmd := m.metricsTickCmdFor(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadBackupsCmd() tea.Cmd {
	return func() tea.Msg {
		backups := system.ListBackups()
		return loadBackupsMsg{backups: backups}
	}
}

// detectConfigsCmd scans the config paths once, on the startup path, so the main
// menu can show the overwrite facts before the wizard re-scans them at the
// Neovim question. The later scan is unchanged: it re-reads the same paths and
// replaces this value, so the two cannot disagree about what exists.
func detectConfigsCmd() tea.Cmd {
	return func() tea.Msg {
		return configsDetectedMsg{configs: system.DetectExistingConfigs()}
	}
}

// loadTrainerStatsCmd reads the trainer's persisted stats on the startup path,
// the way the backups are loaded, so the main menu's "Your trainer" panel
// describes real progress on the first frame the message lands instead of an
// empty panel that reads as "nothing played". The trainer's own entry point still
// re-reads them, so a session that just saved is reflected when it is reopened.
func loadTrainerStatsCmd() tea.Cmd {
	return func() tea.Msg {
		return trainerStatsLoadedMsg{stats: trainer.LoadStats()}
	}
}

// loadLastInstallCmd reads the record of the previous completed run on the
// startup path. It is the read half of the state file writeLastInstall writes
// when a run finishes.
func loadLastInstallCmd() tea.Cmd {
	return func() tea.Msg {
		return lastInstallLoadedMsg{record: readLastInstall()}
	}
}

// loadDotfilesThemesCmd reads the theme definitions from the checkout the clone
// step created. It is issued when the utilities section is first opened, not at
// startup, because before a clone there is nothing to read and the section says
// so rather than showing a switch that cannot work.
func loadDotfilesThemesCmd(repoDir string) tea.Cmd {
	return func() tea.Msg {
		if repoDir == "" {
			return dotfilesThemesLoadedMsg{err: fmt.Errorf("the repository has not been cloned yet")}
		}
		defs, err := loadThemeDefinitions(repoDir)
		return dotfilesThemesLoadedMsg{themes: defs, err: err}
	}
}

// dotfilesThemesCmdIfNeeded reads the definitions the first time the utilities
// section is opened. It returns no command when they are already read or when
// there is no checkout to read them from.
func (m *Model) dotfilesThemesCmdIfNeeded() tea.Cmd {
	if m.DotfilesThemes != nil || m.RepoDir == "" {
		return nil
	}
	return loadDotfilesThemesCmd(m.RepoDir)
}

// applyDotfilesThemeCmd runs one dotfiles-theme switch off the update loop,
// behind the same dry-run gate as the desktop switch.
func applyDotfilesThemeCmd(def themeDefinition) tea.Cmd {
	return func() tea.Msg {
		homeDir := os.Getenv("HOME")
		_, notice, err := applyDotfilesTheme(homeDir, def)
		return dotfilesThemeChangedMsg{notice: notice, err: err}
	}
}

// undoDotfilesThemeCmd puts the recorded blocks back off the update loop.
func undoDotfilesThemeCmd(rec dotfilesThemeRecord) tea.Cmd {
	return func() tea.Msg {
		notice, err := undoDotfilesTheme(rec)
		return dotfilesThemeChangedMsg{notice: notice, err: err}
	}
}

// loadThemeRecordCmd reads the record of the last theme change on the startup
// path. It is the read half of the state file writeThemeRecord writes, and the
// utilities section's undo row is offered only when this finds a record for the
// desktop this host is on.
func loadThemeRecordCmd() tea.Cmd {
	return func() tea.Msg {
		return themeRecordLoadedMsg{record: readThemeRecord(), dotfiles: readDotfilesThemeRecord()}
	}
}

// Update implements tea.Model. It is the one place a screen change is observed,
// so it is also the one place the active panel is reset: a screen always opens on
// its default panel, whichever path changed the screen. The companion's two
// event-driven fields are written here for the same reason: any key wakes it, and
// a key that moved the cursor or changed the screen gives it something to walk
// toward. The walking itself happens on the frame ticks that follow.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	screen := m.Screen
	cursor := m.Cursor
	if _, ok := msg.(tea.KeyMsg); ok {
		// Any key wakes the companion and restarts the stretch that would put it
		// back to sleep.
		m.CompanionIdle = 0
	}
	next, cmd := m.update(msg)

	updated, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	if updated.Screen != screen {
		updated.PanelIndex = 0
	}
	if updated.Cursor != cursor || updated.Screen != screen {
		updated.armCompanionFollow()
	}
	return updated, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A buffer is never allowed to survive a non-key event: in particular, a
	// frame tick cannot leave a stale prefix waiting to consume a later key.
	if m.MenuKeyBuffer != "" {
		if _, ok := msg.(tea.KeyMsg); !ok {
			m.MenuKeyBuffer = ""
		}
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.MouseMsg:
		// The pointer is the creature's to read and nobody else's: the installer
		// binds no mouse action, so a motion, a click and a wheel all arrive here and
		// the creature decides what they mean. It is handled in the inner switch so
		// the outer Update's own post-processing (the companion's follow arming) does
		// not run for a message that changed no screen and no cursor.
		return m.handleCompanionMouse(msg)

	case tickMsg:
		// A menu gag's lifetime uses this model tick, including when animation is
		// gated off. The renderer sees only the resulting state and never a clock.
		if m.MenuGagActive {
			m.MenuGagTicks++
			if m.MenuGagTicks >= menuGagDurationTicks {
				m.MenuGagActive = false
			}
		}
		// The tick is the run's clock. Its timestamp is copied onto the model here,
		// in Update, so the installing screen's elapsed time and estimate are reads
		// of model state and never a clock read inside a render.
		m.Now = time.Time(msg)
		// The tick wakes the model on its own, so the exercise countdown stays live
		// and an idle player's hint is revealed when its deadline passes without
		// any key press.
		m.revealExerciseHintOnDeadline()
		// The boss fight has a failure deadline instead of a hint: the same tick
		// charges the life when a boss step is left unanswered.
		m.expireBossStepOnDeadline()
		// Continue ticking for the trainer's deadlines. This is the trainer's clock,
		// not the animation clock: the frame tick below is the one the animation gate
		// owns, and this one keeps running whether or not the run animates. It never
		// touches AnimTick, so it cannot move the tip or the companion.
		return m, tickCmd()

	case animTickMsg:
		// The frame tick advances the counter the tip rotation and the companion
		// read, then takes one companion step. It only re-arms while animation is on,
		// so a gate that was forced off after the tick was armed stops the clock
		// rather than letting it run on.
		if !m.Animating {
			return m, nil
		}
		m.AnimTick++
		m.advanceCompanion()
		m.advanceCelebration()
		return m, m.animTickCmdFor()

	case metricsTickMsg:
		// The sampling tick reads the host and re-arms itself, on the same gate the
		// frame tick uses. A gate forced off after the tick was armed stops the
		// sampling rather than letting the reading go stale-but-live.
		if !m.Animating {
			return m, nil
		}
		return m, tea.Batch(readMetricsCmd(), metricsTickCmd())

	case metricsReadMsg:
		// One raw reading is folded into the ring here, in Update, so the render
		// path draws samples from the model and never touches the host.
		return m.withMetricsRead(msg.sample), nil

	case installStartMsg:
		// A run begins here, so this is where the run's start timestamp is set -- and
		// reset for a run that follows a retry. The installing screen measures its
		// elapsed time from it, and it lives on the model rather than being read
		// while rendering.
		m.InstallStartedAt = time.Now()
		// A new run starts its progress history over: the chart on the installing
		// screen is this run's, not the previous one's, and a retry begins clean for
		// the same reason the start timestamp is reset.
		m.ProgressSamples = nil
		m.Celebrating = false
		m.CelebrationTick = 0
		m.Particles = nil
		// Start the installation process
		cmd := m.runNextStep()
		return m, cmd

	case stepProgressMsg:
		// Update progress
		for i := range m.Steps {
			if m.Steps[i].ID == msg.stepID {
				m.Steps[i].Progress = msg.progress
				break
			}
		}
		if msg.log != "" {
			m.LogLines = append(m.LogLines, msg.log)
			// Keep only last 20 lines
			if len(m.LogLines) > 20 {
				m.LogLines = m.LogLines[len(m.LogLines)-20:]
			}
		}
		return m, nil

	case stepCompleteMsg:
		// A step runs after Update has returned its model, so the state it
		// recorded arrives with the message instead of being written through.
		// Carry it into the model this handler returns before the bookkeeping
		// below, so a later step sees it and cleanup can find the checkout.
		if msg.recorded != nil {
			msg.recorded.applyTo(&m)
		}
		// Mark step as complete
		for i := range m.Steps {
			if m.Steps[i].ID == msg.stepID {
				if msg.err != nil {
					m.Steps[i].Status = StatusFailed
					m.Steps[i].Error = msg.err
					m.Screen = ScreenError
					// Include step name in error message for clarity
					m.ErrorMsg = fmt.Sprintf("Step '%s' failed:\n%s", m.Steps[i].Name, msg.err.Error())
					return m, nil
				}
				m.Steps[i].Status = StatusDone
				m.Steps[i].Progress = 1.0
				break
			}
		}
		m.CurrentStep++
		// A step that finished is worth a cheer for the next few ticks. The failure
		// path above returns before this line, so an error is never celebrated.
		m.CompanionPleased = companionPleasedTicks
		return m, m.runNextStep()

	case installCompleteMsg:
		m.TotalTime = msg.totalTime
		m.Screen = ScreenComplete
		// The run is over, so this is where the celebration belongs: the burst of
		// particles and the companion's pleased state are both armed from here, and
		// both are no-ops when the animation gate is off.
		if m.Animating {
			m.startCelebration()
			m.CompanionPleased = companionPleasedTicks
		}
		// The run finished, so record it for the next one. The write is best effort:
		// a state directory the machine will not let us write must not fail an
		// install that has already succeeded.
		m.recordLastInstall()
		return m, nil

	case loadBackupsMsg:
		m.AvailableBackups = msg.backups
		return m, nil

	case trainerStatsLoadedMsg:
		// The startup load bootstraps the panel; it does not overwrite a model that
		// already holds stats, so a seeded model and a session that entered the
		// trainer before this arrived both keep what they have.
		if m.TrainerStats == nil {
			m.TrainerStats = msg.stats
		}
		return m, nil

	case lastInstallLoadedMsg:
		// As with the stats, the startup read fills a model the writer did not
		// already fill, and never clobbers one.
		if m.LastInstall == nil {
			m.LastInstall = msg.record
		}
		return m, nil

	case themeRecordLoadedMsg:
		// The theme record follows the same rule as the last-install one: the
		// startup read fills a model the writer did not already fill.
		if m.ThemeRecord == nil {
			m.ThemeRecord = msg.record
		}
		if m.DotfilesThemeRecord == nil {
			m.DotfilesThemeRecord = msg.dotfiles
		}
		return m, nil

	case themeChangedMsg:
		// The theme switch is not an install step, so a failure stays on the
		// section as a notice: the run is not failed, and there is nothing to
		// retry from another screen.
		if msg.err != nil {
			m.ThemeNotice = msg.err.Error()
			return m, nil
		}
		if msg.record != nil {
			m.ThemeRecord = msg.record
		}
		m.ThemeNotice = msg.notice
		return m, nil

	case dotfilesThemesLoadedMsg:
		if msg.err != nil {
			m.DotfilesThemesErr = msg.err.Error()
			return m, nil
		}
		m.DotfilesThemes = msg.themes
		m.DotfilesThemesErr = ""
		return m, nil

	case dotfilesThemeChangedMsg:
		// The dotfiles switch is not an install step either, so a failure is a
		// notice on the section rather than a failed run.
		if msg.err != nil {
			m.ThemeNotice = msg.err.Error()
			return m, nil
		}
		// The record is what makes the change reversible, so it is re-read after
		// every successful change: an apply leaves one, an undo clears it.
		m.DotfilesThemeRecord = readDotfilesThemeRecord()
		m.ThemeNotice = msg.notice
		return m, nil

	case configsDetectedMsg:
		// The startup scan bootstraps the overwrite facts that the main menu's plan
		// is built from; it does not overwrite a model that already holds them. In
		// the program the field is empty when this arrives, so the scan fills it,
		// while the wizard's own scan at the Neovim question stays the authority
		// that replaces what the model knows.
		if len(m.ExistingConfigs) == 0 {
			m.ExistingConfigs = msg.configs
		}
		return m, nil

	case execFinishedMsg:
		// Interactive process finished (sudo commands, chsh, etc)
		for i := range m.Steps {
			if m.Steps[i].ID == msg.stepID {
				if msg.err != nil {
					msg.err = describeInteractiveStepError(msg.stepID, msg.err)
					m.Steps[i].Status = StatusFailed
					m.Steps[i].Error = msg.err
					m.Screen = ScreenError
					// Include step name in error message for clarity
					m.ErrorMsg = fmt.Sprintf("Step '%s' failed:\n%s", m.Steps[i].Name, msg.err.Error())
					return m, nil
				}
				m.Steps[i].Status = StatusDone
				m.Steps[i].Progress = 1.0
				break
			}
		}
		m.CurrentStep++
		// An interactive step counts as a finished step like any other, so it cheers
		// the companion the same way; a failure returns above and never does.
		m.CompanionPleased = companionPleasedTicks
		return m, m.runNextStep()

	case needsExecProcessMsg:
		// This step needs to run with tea.ExecProcess for interactive input
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			return execFinishedMsg{stepID: msg.stepID, err: err}
		})
	}

	return m, nil
}

func describeInteractiveStepError(stepID string, err error) error {
	if err == nil {
		return nil
	}

	if stepID == "homebrew" {
		if system.CommandExists("brew") {
			return fmt.Errorf("Homebrew installed, but post-install shell setup did not complete cleanly: %w", err)
		}

		return fmt.Errorf("Homebrew installation command failed: %w", err)
	}

	return err
}

// execInteractiveCmd creates a tea.Cmd that runs an interactive process
// This suspends the TUI and gives full terminal control to the process
func execInteractiveCmd(stepID string, name string, args ...string) tea.Cmd {
	c := exec.Command(name, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{stepID: stepID, err: err}
	})
}

const (
	menuEasterEggBufferMax = 3
	menuGagDurationTicks   = 8
)

var menuEasterEggForms = [...]string{"dd", ":q", "vim"}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// A trainer save warning carried onto the main menu is transient like the
	// trainer's own messages: the next key clears it, so the line is shown once
	// where the player landed and the main menu never becomes a log.
	if m.Screen == ScreenMainMenu {
		m.TrainerMessage = ""
	}

	// On the main menu, printable characters only remain buffered while they
	// extend one of the three known forms. A mismatch clears the prefix and falls
	// through to the ordinary handler below with this same key.
	if m.Screen == ScreenMainMenu {
		candidate := m.MenuKeyBuffer + key
		if len([]rune(candidate)) <= menuEasterEggBufferMax && isMenuEasterEggPrefix(candidate) {
			m.MenuKeyBuffer = candidate
			if isMenuEasterEgg(candidate) {
				m.MenuKeyBuffer = ""
				switch candidate {
				case "dd":
					m.MenuGagRow = m.Cursor
					m.MenuGagTicks = 0
					m.MenuGagActive = true
				case ":q":
					m.Quitting = true
					return m, tea.Quit
				case "vim":
					return m.startTrainer()
				}
			}
			return m, nil
		}
		m.MenuKeyBuffer = ""
	} else {
		m.MenuKeyBuffer = ""
	}

	// ctrl+c always quits immediately (no leader needed)
	if key == "ctrl+c" {
		m.Quitting = true
		return m, tea.Quit
	}

	// Leader key mode: <space> activates, next key executes command
	// Commands: <space>q = quit, <space>d = toggle details
	if m.LeaderMode {
		m.LeaderMode = false // Reset leader mode
		switch key {
		case "q":
			// Quit application
			if m.Screen != ScreenInstalling {
				m.Quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "d":
			// Toggle details during installation
			if m.Screen == ScreenInstalling {
				m.ShowDetails = !m.ShowDetails
			}
			return m, nil
		default:
			// Unknown leader command, ignore
			return m, nil
		}
	}

	// <space> activates leader mode EXCEPT in screens that need space for input
	// (Trainer screens use space in commands, Welcome screen uses space to continue)
	if key == " " {
		// Screens where space should NOT activate leader mode
		switch {
		case m.Screen == ScreenWelcome:
			// Welcome screen: space continues to main menu
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			return m, nil
		case m.Screen == ScreenComplete || m.Screen == ScreenError:
			// Complete/Error screens: space quits the app
			m.Quitting = true
			return m, tea.Quit
		case isTrainerScreen(m.Screen):
			// Trainer screens own space: the exercise screens append it to the
			// answer input, and the menu and result screens treat it like enter.
			// Pass through to the screen-specific handlers below.
		default:
			// All other screens: activate leader mode
			m.LeaderMode = true
			return m, nil
		}
	}

	// ESC goes back from content/learn screens (and cancels leader mode implicitly)
	if key == "esc" {
		return m.handleEscape()
	}

	// Tab cycles the panels of a screen that offers more than one, wrapping
	// around. It is taken here, before the per-screen dispatch, but only when
	// there is a panel to reach: a screen that offers one panel or none does not
	// consume the key, so whatever Tab means to it -- the trainer's exercise
	// screens read it as "hint" -- is left exactly as it was. The key does nothing
	// on a single-panel screen, which is why no such screen advertises it.
	if key == "tab" {
		if next, moved := nextPanelIndex(m.panelsFor(), m.PanelIndex); moved {
			m.PanelIndex = next
			return m, nil
		}
	}

	// Screen-specific keys
	switch m.Screen {
	case ScreenWelcome:
		switch key {
		case "enter":
			m.Screen = ScreenMainMenu
			m.Cursor = 0
		}

	case ScreenMainMenu:
		return m.handleMainMenuKeys(key)

	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		return m.handleSelectionKeys(key)

	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim:
		return m.handleLearnMenuKeys(key)

	case ScreenKeymaps:
		return m.handleKeymapsMenuKeys(key)

	case ScreenKeymapCategory:
		return m.handleKeymapCategoryKeys(key)

	case ScreenKeymapsMenu:
		return m.handleToolKeymapsMenuKeys(key)

	case ScreenKeymapsTmux:
		return m.handleTmuxKeymapsMenuKeys(key)

	case ScreenKeymapsTmuxCat:
		return m.handleTmuxKeymapCategoryKeys(key)

	case ScreenKeymapsZellij:
		return m.handleZellijKeymapsMenuKeys(key)

	case ScreenKeymapsZellijCat:
		return m.handleZellijKeymapCategoryKeys(key)

	case ScreenKeymapsGhostty:
		return m.handleGhosttyKeymapsMenuKeys(key)

	case ScreenKeymapsGhosttyCat:
		return m.handleGhosttyKeymapCategoryKeys(key)

	case ScreenKeymapsHerdr:
		return m.handleHerdrKeymapsMenuKeys(key)

	case ScreenKeymapsHerdrCat:
		return m.handleHerdrKeymapCategoryKeys(key)

	case ScreenLearnLazyVim:
		return m.handleLazyVimMenuKeys(key)

	case ScreenLazyVimTopic:
		return m.handleLazyVimTopicKeys(key)

	case ScreenUtilities:
		return m.handleUtilitiesKeys(key)

	case ScreenBackupConfirm:
		return m.handleBackupConfirmKeys(key)

	case ScreenRestoreBackup:
		return m.handleRestoreBackupKeys(key)

	case ScreenRestoreConfirm:
		return m.handleRestoreConfirmKeys(key)

	// Trainer screens
	case ScreenTrainerMenu:
		return m.handleTrainerMenuKeys(key)

	case ScreenTrainerLesson, ScreenTrainerPractice:
		return m.handleTrainerExerciseKeys(key)

	case ScreenTrainerBoss:
		return m.handleTrainerBossKeys(key)

	case ScreenTrainerResult:
		return m.handleTrainerResultKeys(key)

	case ScreenTrainerBossResult:
		return m.handleTrainerBossResultKeys(key)

	case ScreenComplete:
		switch key {
		case "enter", " ":
			m.Quitting = true
			return m, tea.Quit
		}

	case ScreenError:
		switch key {
		case "enter", " ":
			m.Quitting = true
			return m, tea.Quit
		case "r":
			// Retry - go back to beginning
			m.Screen = ScreenWelcome
			m.ErrorMsg = ""
		}
	}

	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.Screen {
	// Installation wizard screens - go back through the flow
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect:
		return m.goBackInstallStep()
	case ScreenGhosttyWarning:
		// Go back to terminal selection
		m.Screen = ScreenTerminalSelect
		m.Cursor = 0
	case ScreenBackupConfirm:
		// Go back to Nvim selection (not abort)
		m.Screen = ScreenNvimSelect
		m.Cursor = 0
	// Content/Learn screens
	case ScreenKeymapCategory:
		m.Screen = ScreenKeymaps
		m.KeymapScroll = 0
	case ScreenKeymapsTmuxCat:
		m.Screen = ScreenKeymapsTmux
		m.TmuxKeymapScroll = 0
	case ScreenKeymapsZellijCat:
		m.Screen = ScreenKeymapsZellij
		m.ZellijKeymapScroll = 0
	case ScreenKeymapsGhosttyCat:
		m.Screen = ScreenKeymapsGhostty
		m.GhosttyKeymapScroll = 0
	case ScreenKeymapsHerdrCat:
		m.Screen = ScreenKeymapsHerdr
		m.HerdrKeymapScroll = 0
	case ScreenLazyVimTopic:
		m.Screen = ScreenLearnLazyVim
		m.LazyVimScroll = 0
	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim:
		m.Screen = m.PrevScreen
		m.Cursor = 0
		m.ViewingTool = ""
	case ScreenKeymaps:
		m.Screen = ScreenKeymapsMenu
		m.Cursor = 0
	case ScreenKeymapsTmux, ScreenKeymapsZellij, ScreenKeymapsGhostty, ScreenKeymapsHerdr:
		m.Screen = ScreenKeymapsMenu
		m.Cursor = 0
	case ScreenKeymapsMenu, ScreenLearnLazyVim:
		m.Screen = m.PrevScreen
		m.Cursor = 0
	// Restore screens
	case ScreenRestoreBackup, ScreenRestoreConfirm:
		m.Screen = ScreenMainMenu
		m.Cursor = 0
	// Trainer screens
	case ScreenTrainerMenu:
		// Save stats and return to main menu. Escape also cancels an armed
		// whole-profile reset, so it can never stay armed behind the menu. The
		// message is cleared first so a failed save can replace it with the
		// warning instead of being wiped by it.
		m.TrainerMessage = ""
		saveTrainerStats(&m)
		m.Screen = ScreenMainMenu
		m.Cursor = 0
	case ScreenTrainerLesson, ScreenTrainerPractice:
		// Esc is handled here before the screen-specific handlers run, so this
		// path owns the save for the exercise screens.
		m.TrainerMessage = ""
		saveTrainerStats(&m)
		m.Screen = ScreenTrainerMenu
	case ScreenTrainerBoss:
		// Save the run and report the abandoned fight instead of leaving silently.
		// The save runs after the message so a failed save is reported instead
		// of the abandoned-fight line.
		m.Screen = ScreenTrainerMenu
		m.TrainerMessage = "Boss fight abandoned!"
		saveTrainerStats(&m)
	case ScreenTrainerResult, ScreenTrainerBossResult:
		// Return to trainer menu
		m.Screen = ScreenTrainerMenu
		m.TrainerMessage = ""
		saveTrainerStats(&m)
	// Main menu - quit
	case ScreenMainMenu:
		m.Quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func isMenuEasterEggPrefix(candidate string) bool {
	for _, form := range menuEasterEggForms {
		if strings.HasPrefix(form, candidate) {
			return true
		}
	}
	return false
}

func isMenuEasterEgg(candidate string) bool {
	for _, form := range menuEasterEggForms {
		if candidate == form {
			return true
		}
	}
	return false
}

// The OS step's option positions (model.go, ScreenOSSelect). They are named so
// the preselection and the menu cannot drift apart silently.
const (
	osOptionMac    = 0
	osOptionLinux  = 1
	osOptionTermux = 2
)

// noSelection is the cursor value a detection-driven step starts on when
// detection named nothing. No row is marked, so the screen cannot present a
// default as the installer's answer: menuRows treats every row as unselected,
// and every reader of the cursor -- the choice panels, the companion -- already
// treats a negative cursor as "nothing under it".
const noSelection = -1

// osOptionIndex returns the OS menu option the wizard's cursor should start on
// for a detected platform.
//
// Every platform constant the system package declares has an explicit case
// here, and the switch has no default: a platform it does not know returns
// ok=false, so nothing can fall through to macOS. That fall-through was the
// defect this replaces, where an else branch compared against one constant and
// silently meant "not generic Linux, therefore macOS" for Debian, Ubuntu, Arch,
// Fedora, Termux and WSL alike.
func osOptionIndex(detected system.OSType) (index int, ok bool) {
	switch detected {
	case system.OSMac:
		return osOptionMac, true
	case system.OSLinux, system.OSArch, system.OSDebian, system.OSFedora, system.OSWSL:
		// WSL is a hosting environment: detection reports the distribution when
		// it recognises one and OSWSL only when it does not, but either way the
		// menu option that matches it is Linux.
		return osOptionLinux, true
	case system.OSTermux:
		return osOptionTermux, true
	case system.OSUnknown:
		// Detection failed and the menu has no "unknown" entry. Signal that
		// there is nothing to preselect rather than claiming a platform.
		return 0, false
	}
	return 0, false
}

// osChoiceOptionIndex returns the OS menu option that matches a choice the
// wizard already recorded ("mac", "linux" or "termux"). The bool is false while
// the question is open, so a caller cannot mistake no answer for macOS.
func osChoiceOptionIndex(choice string) (index int, ok bool) {
	switch choice {
	case "mac":
		return osOptionMac, true
	case "linux":
		return osOptionLinux, true
	case "termux":
		return osOptionTermux, true
	}
	return 0, false
}

// osCursor is where the OS step's cursor starts. A platform the user already
// chose wins, so stepping back to the question keeps their answer; otherwise
// the detected platform is used. When neither is a platform the menu lists the
// step opens with noSelection rather than a highlighted macOS: a row under the
// cursor reads as the installer's answer, and detection gave none.
func (m Model) osCursor() int {
	if index, ok := osChoiceOptionIndex(m.Choices.OS); ok {
		return index
	}
	if m.SystemInfo != nil {
		if index, ok := osOptionIndex(m.SystemInfo.OS); ok {
			return index
		}
	}
	return noSelection
}

// The shell step's option positions (model.go, ScreenShellSelect), named for the
// same reason as the OS positions above.
const (
	shellOptionFish    = 0
	shellOptionZsh     = 1
	shellOptionNushell = 2
)

// shellOptionIndex returns the shell menu option that matches a detected login
// shell. The bool is false for a shell the menu does not offer (bash, dash and
// the rest), so a caller cannot mistake one of them for Fish.
func shellOptionIndex(shell string) (index int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "fish":
		return shellOptionFish, true
	case "zsh":
		return shellOptionZsh, true
	case "nu", "nushell":
		return shellOptionNushell, true
	}
	return 0, false
}

// shellCursor is where the shell step's cursor starts. A shell the user already
// chose wins, so stepping back to the screen keeps their answer; otherwise the
// detected login shell is used. When neither is a shell the menu lists the step
// opens with noSelection, so a shell detection did not name is never shown as
// the installer's answer.
func (m Model) shellCursor() int {
	if index, ok := shellOptionIndex(m.Choices.Shell); ok {
		return index
	}
	if m.SystemInfo != nil {
		if index, ok := shellOptionIndex(m.SystemInfo.UserShell); ok {
			return index
		}
	}
	return noSelection
}

// enterShellSelect moves to the shell step with the cursor on the detected
// shell, so the screen's "Current shell" line and the cursor agree.
func (m *Model) enterShellSelect() {
	m.Screen = ScreenShellSelect
	m.Cursor = m.shellCursor()
}

func (m Model) startTrainer() (tea.Model, tea.Cmd) {
	stats := trainer.LoadStats()
	if stats == nil {
		stats = trainer.NewUserStats()
	}
	m.TrainerStats = stats
	m.TrainerGameState = nil
	m.TrainerCursor = 0
	m.TrainerInput = ""
	m.TrainerMessage = ""
	m.Screen = ScreenTrainerMenu
	m.PrevScreen = ScreenMainMenu
	return m, nil
}

func (m Model) handleMainMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	hasRestoreOption := len(m.AvailableBackups) > 0

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
		}
	case "u":
		// The shortcut to the section the Utilities row opens. It stays because a
		// key costs nothing and someone who has learned it should not lose it; the
		// row is how everyone else finds the section. It is documented beside the
		// other main-menu keys, like `vim`.
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.ThemeNotice = ""
		return m, m.dotfilesThemesCmdIfNeeded()
	case "enter", " ":
		selected := options[m.Cursor]
		switch {
		case strings.Contains(selected, "Start Installation"):
			m.Screen = ScreenOSSelect
			// Start on the platform the user already chose, or the one detection
			// found. When detection named nothing -- or a platform the mapping does
			// not know -- the step opens with nothing highlighted, because a row
			// under the cursor reads as the installer's answer.
			m.Cursor = m.osCursor()
		case strings.Contains(selected, "Learn About Tools"):
			m.Screen = ScreenLearnTerminals
			m.PrevScreen = ScreenMainMenu
			m.Cursor = 0
		case strings.Contains(selected, "Keymaps Reference"):
			m.Screen = ScreenKeymapsMenu
			m.PrevScreen = ScreenMainMenu
			m.Cursor = 0
		case strings.Contains(selected, "LazyVim Guide"):
			m.Screen = ScreenLearnLazyVim
			m.PrevScreen = ScreenMainMenu
			m.Cursor = 0
		case strings.Contains(selected, "Vim Trainer"):
			return m.startTrainer()
		case strings.Contains(selected, "Utilities"):
			// The row and the `u` key reach the same section: the row is how a
			// user finds it, the key is the shortcut for someone who has.
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.ThemeNotice = ""
			return m, m.dotfilesThemesCmdIfNeeded()
		case strings.Contains(selected, "Restore from Backup") && hasRestoreOption:
			m.Screen = ScreenRestoreBackup
			m.Cursor = 0
		case strings.Contains(selected, "Exit"):
			m.Quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

// handleUtilitiesKeys drives the utilities section. It only ever changes the
// screen, the cursor or the notice: the switch itself is a command, so a slow
// desktop tool cannot block the update loop, and the dry-run gate it runs behind
// is the same one executeStep uses.
func (m Model) handleUtilitiesKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "esc", "backspace":
		m.Screen = ScreenMainMenu
		m.Cursor = 0
		m.ThemeNotice = ""
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := options[m.Cursor]
		switch {
		case strings.Contains(selected, "dark theme"):
			return m, applyThemeCmd(m.ThemeSwitch, true)
		case strings.Contains(selected, "light theme"):
			return m, applyThemeCmd(m.ThemeSwitch, false)
		case strings.HasPrefix(selected, "Apply the "):
			if def, ok := m.dotfilesThemeForRow(selected); ok {
				return m, applyDotfilesThemeCmd(def)
			}
		case selected == dotfilesThemeUndoRow && m.DotfilesThemeRecord != nil:
			return m, undoDotfilesThemeCmd(*m.DotfilesThemeRecord)
		case strings.Contains(selected, "Undo") && m.themeUndoAvailable():
			return m, undoThemeCmd(m.ThemeSwitch, *m.ThemeRecord)
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			m.ThemeNotice = ""
		}
	}

	return m, nil
}

// applyThemeCmd runs one theme switch off the update loop. The gate on
// --dry-run is inside applyTheme, like the one inside executeStep, so the flag
// stops the command whether it is reached through this command or called
// directly.
func applyThemeCmd(target themeSwitch, dark bool) tea.Cmd {
	return func() tea.Msg {
		rec, notice, err := applyTheme(target, dark)
		return themeChangedMsg{record: rec, notice: notice, err: err}
	}
}

// undoThemeCmd runs one undo off the update loop, behind the same gate.
func undoThemeCmd(target themeSwitch, rec themeRecord) tea.Cmd {
	return func() tea.Msg {
		next, notice, err := undoTheme(target, rec)
		return themeChangedMsg{record: next, notice: notice, err: err}
	}
}

func (m Model) handleSelectionKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			// Skip separator lines
			if strings.HasPrefix(options[m.Cursor], "───") {
				if m.Cursor > 0 {
					m.Cursor--
				}
			}
		}

	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			// Skip separator lines
			if strings.HasPrefix(options[m.Cursor], "───") {
				if m.Cursor < len(options)-1 {
					m.Cursor++
				}
			}
		}

	case "esc", "backspace":
		// Go back to previous installation step
		return m.goBackInstallStep()

	case "enter", " ":
		return m.handleSelection()
	}

	return m, nil
}

// goBackInstallStep handles going back during installation wizard
func (m Model) goBackInstallStep() (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenOSSelect:
		// Go back to main menu
		m.Screen = ScreenMainMenu
		m.Cursor = 0
		// Reset choices
		m.Choices = UserChoices{}

	case ScreenTerminalSelect:
		m.Screen = ScreenOSSelect
		// Keep the platform the user chose rather than reopening the question on
		// macOS; with no choice recorded this falls back to detection, then to
		// nothing highlighted.
		m.Cursor = m.osCursor()
		// Reset terminal choice
		m.Choices.Terminal = ""

	case ScreenFontSelect:
		m.Screen = ScreenTerminalSelect
		m.Cursor = 0
		// Reset font choice
		m.Choices.InstallFont = false

	case ScreenShellSelect:
		// Termux: go back to OS selection (skipped terminal and font)
		// WSL: go back to OS selection (skipped terminal and font)
		if m.SystemInfo.IsTermux || m.SystemInfo.IsWSL {
			m.Screen = ScreenOSSelect
			m.Cursor = m.osCursor()
		} else if m.Choices.Terminal == "none" {
			// If we skipped font selection (terminal = none), go back to terminal
			m.Screen = ScreenTerminalSelect
			m.Cursor = 0
		} else {
			m.Screen = ScreenFontSelect
			m.Cursor = 0
		}
		m.Choices.Shell = ""

	case ScreenWMSelect:
		m.enterShellSelect()
		m.Choices.WindowMgr = ""

	case ScreenNvimSelect:
		m.Screen = ScreenWMSelect
		m.Cursor = 0
		m.Choices.InstallNvim = false
	}

	return m, nil
}

func (m Model) handleSelection() (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	// A step that opened with noSelection is waiting for an explicit choice:
	// Enter must not answer for the user, and indexing a negative cursor would
	// be a panic rather than a refusal.
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return m, nil
	}

	selected := strings.ToLower(options[m.Cursor])

	// Check for "learn" options
	if strings.Contains(selected, "learn about terminals") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenLearnTerminals
		m.Cursor = 0
		return m, nil
	}
	if strings.Contains(selected, "learn about shells") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenLearnShells
		m.Cursor = 0
		return m, nil
	}
	if strings.Contains(selected, "learn about multiplexers") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenLearnWM
		m.Cursor = 0
		return m, nil
	}
	if strings.Contains(selected, "learn about neovim") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenLearnNvim
		m.Cursor = 0
		return m, nil
	}
	if strings.Contains(selected, "view keymaps") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenKeymaps
		m.Cursor = 0
		return m, nil
	}
	if strings.Contains(selected, "lazyvim guide") {
		m.PrevScreen = m.Screen
		m.Screen = ScreenLearnLazyVim
		m.Cursor = 0
		return m, nil
	}

	// Skip separators
	if strings.HasPrefix(selected, "───") {
		return m, nil
	}

	switch m.Screen {
	case ScreenOSSelect:
		selectedLower := strings.ToLower(selected)
		if strings.Contains(selectedLower, "mac") {
			m.Choices.OS = "mac"
		} else if strings.Contains(selectedLower, "termux") {
			m.Choices.OS = "termux"
		} else {
			m.Choices.OS = "linux"
		}
		// Termux: skip Terminal selection (you're already in a terminal!)
		// But allow font installation (Termux supports custom fonts)
		// WSL: skip Terminal selection (terminal emulators run on Windows host)
		if m.Choices.OS == "termux" {
			m.Choices.Terminal = "none"
			m.Choices.InstallFont = true // Install Nerd Font for Termux
			m.enterShellSelect()
		} else if m.SystemInfo.IsWSL {
			m.Choices.Terminal = "none"
			m.Choices.InstallFont = false // Fonts should be installed on Windows host
			m.enterShellSelect()
		} else {
			m.Screen = ScreenTerminalSelect
			m.Cursor = 0
		}

	case ScreenTerminalSelect:
		term := strings.ToLower(strings.Split(options[m.Cursor], " ")[0])
		m.Choices.Terminal = term

		// Check if Ghostty on Debian/Ubuntu - show warning
		if term == "ghostty" && m.Choices.OS == "linux" && m.SystemInfo.OS == system.OSDebian && !system.CommandExists("ghostty") {
			m.Screen = ScreenGhosttyWarning
			m.Cursor = 0
			return m, nil
		}

		if term != "none" {
			m.Screen = ScreenFontSelect
			m.Cursor = 0
		} else {
			m.enterShellSelect()
		}

	case ScreenFontSelect:
		m.Choices.InstallFont = m.Cursor == 0
		m.enterShellSelect()

	case ScreenShellSelect:
		m.Choices.Shell = strings.ToLower(options[m.Cursor])
		m.Screen = ScreenWMSelect
		m.Cursor = 0

	case ScreenGhosttyWarning:
		switch m.Cursor {
		case 0: // Continue with Ghostty anyway
			m.Screen = ScreenFontSelect
			m.Cursor = 0
		case 1: // Choose different terminal
			m.Screen = ScreenTerminalSelect
			m.Cursor = 0
		case 2: // Cancel
			m.Screen = ScreenMainMenu
			m.Cursor = 0
		}

	case ScreenWMSelect:
		m.Choices.WindowMgr = strings.ToLower(options[m.Cursor])
		m.Screen = ScreenNvimSelect
		m.Cursor = 0

	case ScreenNvimSelect:
		m.Choices.InstallNvim = m.Cursor == 0
		// Detect existing configs before proceeding
		m.ExistingConfigs = system.DetectExistingConfigs()
		if len(m.ExistingConfigs) > 0 {
			// Show backup confirmation screen
			m.Screen = ScreenBackupConfirm
			m.Cursor = 0
		} else {
			// No existing configs, proceed directly
			m.SetupInstallSteps()
			m.Screen = ScreenInstalling
			m.CurrentStep = 0
			return m, func() tea.Msg { return installStartMsg{} }
		}
	}

	return m, nil
}

func (m Model) handleLearnMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			m.ViewingTool = ""
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Handle Learn Nvim special options
		if m.Screen == ScreenLearnNvim {
			switch m.Cursor {
			case 0: // View Features
				m.ViewingTool = "features"
			case 1: // View Keymaps
				m.Screen = ScreenKeymaps
				m.PrevScreen = ScreenLearnNvim
				m.Cursor = 0
				return m, nil
			case 2: // LazyVim Guide
				m.Screen = ScreenLearnLazyVim
				m.PrevScreen = ScreenLearnNvim
				m.Cursor = 0
				return m, nil
			}
			return m, nil
		}

		// Set viewing tool for other learn screens
		m.ViewingTool = strings.ToLower(selected)
	}

	return m, nil
}

func (m Model) handleKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select category and show keymaps
		m.SelectedCategory = m.Cursor
		m.Screen = ScreenKeymapCategory
		m.KeymapScroll = 0
	}

	return m, nil
}

func (m Model) handleKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.KeymapCategories[m.SelectedCategory]

	// The view and the keys ask the same helper for the window size, and the keys
	// clamp the scroll value with the bound the renderer's own window uses, so the
	// scroll value is the table's top row and the last binding is reachable.
	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.KeymapScroll > 0 {
			m.KeymapScroll--
		}
	case "down", "j":
		if m.KeymapScroll < maxScroll {
			m.KeymapScroll++
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenKeymaps
		m.KeymapScroll = 0
	}

	return m, nil
}

// handleToolKeymapsMenuKeys handles the tool selection menu (Neovim, Tmux, Zellij, Herdr, Ghostty)
func (m Model) handleToolKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Navigate to specific tool's keymaps
		switch m.Cursor {
		case 0: // Neovim
			m.Screen = ScreenKeymaps
			m.Cursor = 0
		case 1: // Tmux
			m.Screen = ScreenKeymapsTmux
			m.Cursor = 0
		case 2: // Zellij
			m.Screen = ScreenKeymapsZellij
			m.Cursor = 0
		case 3: // Herdr
			m.Screen = ScreenKeymapsHerdr
			m.Cursor = 0
		case 4: // Ghostty
			m.Screen = ScreenKeymapsGhostty
			m.Cursor = 0
		}
	}

	return m, nil
}

// handleTmuxKeymapsMenuKeys handles Tmux keymap category selection
func (m Model) handleTmuxKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = ScreenKeymapsMenu
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select category and show keymaps
		m.TmuxSelectedCategory = m.Cursor
		m.Screen = ScreenKeymapsTmuxCat
		m.TmuxKeymapScroll = 0
	}

	return m, nil
}

// handleTmuxKeymapCategoryKeys handles scrolling in Tmux keymap category view
func (m Model) handleTmuxKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.TmuxKeymapCategories[m.TmuxSelectedCategory]

	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.TmuxKeymapScroll > 0 {
			m.TmuxKeymapScroll--
		}
	case "down", "j":
		if m.TmuxKeymapScroll < maxScroll {
			m.TmuxKeymapScroll++
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenKeymapsTmux
		m.TmuxKeymapScroll = 0
	}

	return m, nil
}

// handleZellijKeymapsMenuKeys handles Zellij keymap category selection
func (m Model) handleZellijKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = ScreenKeymapsMenu
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select category and show keymaps
		m.ZellijSelectedCategory = m.Cursor
		m.Screen = ScreenKeymapsZellijCat
		m.ZellijKeymapScroll = 0
	}

	return m, nil
}

// handleZellijKeymapCategoryKeys handles scrolling in Zellij keymap category view
func (m Model) handleZellijKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.ZellijKeymapCategories[m.ZellijSelectedCategory]

	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.ZellijKeymapScroll > 0 {
			m.ZellijKeymapScroll--
		}
	case "down", "j":
		if m.ZellijKeymapScroll < maxScroll {
			m.ZellijKeymapScroll++
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenKeymapsZellij
		m.ZellijKeymapScroll = 0
	}

	return m, nil
}

// handleGhosttyKeymapsMenuKeys handles Ghostty keymap category selection
func (m Model) handleGhosttyKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = ScreenKeymapsMenu
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select category and show keymaps
		m.GhosttySelectedCategory = m.Cursor
		m.Screen = ScreenKeymapsGhosttyCat
		m.GhosttyKeymapScroll = 0
	}

	return m, nil
}

// handleGhosttyKeymapCategoryKeys handles scrolling in Ghostty keymap category view
func (m Model) handleGhosttyKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.GhosttyKeymapCategories[m.GhosttySelectedCategory]

	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.GhosttyKeymapScroll > 0 {
			m.GhosttyKeymapScroll--
		}
	case "down", "j":
		if m.GhosttyKeymapScroll < maxScroll {
			m.GhosttyKeymapScroll++
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenKeymapsGhostty
		m.GhosttyKeymapScroll = 0
	}

	return m, nil
}

// handleHerdrKeymapsMenuKeys handles Herdr keymap category selection
func (m Model) handleHerdrKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = ScreenKeymapsMenu
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select category and show keymaps
		m.HerdrSelectedCategory = m.Cursor
		m.Screen = ScreenKeymapsHerdrCat
		m.HerdrKeymapScroll = 0
	}

	return m, nil
}

// handleHerdrKeymapCategoryKeys handles scrolling in Herdr keymap category view
func (m Model) handleHerdrKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.HerdrKeymapCategories[m.HerdrSelectedCategory]

	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.HerdrKeymapScroll > 0 {
			m.HerdrKeymapScroll--
		}
	case "down", "j":
		if m.HerdrKeymapScroll < maxScroll {
			m.HerdrKeymapScroll++
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenKeymapsHerdr
		m.HerdrKeymapScroll = 0
	}

	return m, nil
}

func (m Model) handleLazyVimMenuKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// Select topic and show content
		m.SelectedLazyVimTopic = m.Cursor
		m.Screen = ScreenLazyVimTopic
		m.LazyVimScroll = 0
	}

	return m, nil
}

func (m Model) handleLazyVimTopicKeys(key string) (tea.Model, tea.Cmd) {
	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]

	// The view and the keys measure the same lines and the same window, so the
	// scroll value is the topic's top line and its last line is reachable.
	maxScroll := offsetWindowMax(lazyVimTopicRows(m.Height), len(m.lazyVimTopicLines(topic)))

	switch key {
	case "up", "k":
		if m.LazyVimScroll > 0 {
			m.LazyVimScroll--
		}
	case "down", "j":
		if m.LazyVimScroll < maxScroll {
			m.LazyVimScroll++
		}
	case "pgup":
		m.LazyVimScroll -= 10
		if m.LazyVimScroll < 0 {
			m.LazyVimScroll = 0
		}
	case "pgdown":
		m.LazyVimScroll += 10
		if m.LazyVimScroll > maxScroll {
			m.LazyVimScroll = maxScroll
		}
	case "enter", " ", "q", "esc":
		m.Screen = ScreenLearnLazyVim
		m.LazyVimScroll = 0
	}

	return m, nil
}

func (m Model) handleBackupConfirmKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
		}
	case "enter", " ":
		switch m.Cursor {
		case 0: // Install with Backup
			m.Choices.CreateBackup = true
			m.SetupInstallSteps()
			m.Screen = ScreenInstalling
			m.CurrentStep = 0
			return m, func() tea.Msg { return installStartMsg{} }
		case 1: // Install without Backup
			m.Choices.CreateBackup = false
			m.SetupInstallSteps()
			m.Screen = ScreenInstalling
			m.CurrentStep = 0
			return m, func() tea.Msg { return installStartMsg{} }
		case 2: // Cancel - abort the entire wizard
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			// Reset choices when canceling
			m.Choices = UserChoices{}
		}
	case "esc", "backspace":
		// Go back to Nvim selection
		m.Screen = ScreenNvimSelect
		m.Cursor = 0
	}

	return m, nil
}

func (m Model) handleRestoreBackupKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			// Skip separator
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			// Skip separator
			if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter", " ":
		// Check if Back option
		if strings.Contains(options[m.Cursor], "Back") {
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			return m, nil
		}
		// Skip separator
		if strings.HasPrefix(options[m.Cursor], "───") {
			return m, nil
		}
		// Select a backup
		if m.Cursor < len(m.AvailableBackups) {
			m.SelectedBackup = m.Cursor
			m.Screen = ScreenRestoreConfirm
			m.Cursor = 0
		}
	case "esc":
		m.Screen = ScreenMainMenu
		m.Cursor = 0
	}

	return m, nil
}

func (m Model) handleRestoreConfirmKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
		}
	case "enter", " ":
		backup := m.AvailableBackups[m.SelectedBackup]
		switch m.Cursor {
		case 0: // Restore
			err := system.RestoreBackup(backup.Path)
			if err != nil {
				m.Screen = ScreenError
				m.ErrorMsg = "Failed to restore backup: " + err.Error()
				return m, nil
			}
			// Refresh backups list
			m.AvailableBackups = system.ListBackups()
			m.Screen = ScreenComplete
			m.Choices = UserChoices{} // Clear choices to indicate restore
		case 1: // Delete
			// A failed deletion used to be discarded and the screen advanced as
			// if the backup were gone, so the list still held it while the UI
			// claimed otherwise. Report it the same way the restore failure
			// above does instead of moving on.
			if err := system.DeleteBackup(backup.Path); err != nil {
				m.Screen = ScreenError
				m.ErrorMsg = "Failed to delete backup: " + err.Error()
				return m, nil
			}
			// Refresh backups list
			m.AvailableBackups = system.ListBackups()
			m.Screen = ScreenRestoreBackup
			m.Cursor = 0
			m.SelectedBackup = 0
		case 2: // Cancel
			m.Screen = ScreenRestoreBackup
			m.Cursor = m.SelectedBackup
		}
	case "esc":
		m.Screen = ScreenRestoreBackup
		m.Cursor = m.SelectedBackup
	}

	return m, nil
}

// stepExecutor runs one installation step against the model the step loop owns.
// It is a variable so a test can drive the TUI's own step loop end to end
// without cloning from the network or installing anything.
var stepExecutor = executeStep

// runNextStep starts the next installation step.
//
// The receiver is a pointer because a step records state its successors need:
// the clone step records the checkout every later step reads and cleanup
// removes, and BackupDir is recorded by the backup step. With a value receiver
// executeStep received a pointer into a copy that was thrown away, so none of
// that state reached the model Update kept.
func (m *Model) runNextStep() tea.Cmd {
	if m.CurrentStep >= len(m.Steps) {
		return func() tea.Msg {
			return installCompleteMsg{totalTime: 0}
		}
	}

	step := &m.Steps[m.CurrentStep]
	step.Status = StatusRunning

	// Check if this step needs interactive input (sudo, chsh, etc)
	if step.Interactive {
		return runInteractiveStep(step.ID, m)
	}

	// The step runs in a tea.Cmd, after Update has returned the model it keeps.
	// Run it against a copy and hand back what it recorded; Update applies that
	// to the model it returns.
	stepModel := *m
	return func() tea.Msg {
		// Execute the step
		err := stepExecutor(step.ID, &stepModel)
		recorded := stepModel.recordedState()
		return stepCompleteMsg{stepID: step.ID, err: err, recorded: &recorded}
	}
}

// ============================================================================
// Trainer Handlers
// ============================================================================

// isTrainerScreen reports whether s is one of the Vim Trainer screens.
//
// The trainer owns the space key on all of its screens: a space is ordinary
// Vim input on the exercise screens, and it selects the highlighted action like
// enter on the menu and result screens. The global key handler must therefore
// hand space to the trainer screens instead of turning it into the leader-key
// prefix.
func isTrainerScreen(s Screen) bool {
	switch s {
	case ScreenTrainerMenu, ScreenTrainerLesson, ScreenTrainerPractice,
		ScreenTrainerBoss, ScreenTrainerResult, ScreenTrainerBossResult:
		return true
	default:
		return false
	}
}

// revealExerciseHintOnDeadline reveals the current lesson or practice exercise's
// hint once its own TimeoutSecs has passed, so a stuck user learns the hint
// exists without having to discover the Tab key. It only writes the feedback
// message: it never touches the typed answer, never submits, and never changes
// the screen, so an expired exercise stays open and answerable. Boss steps
// mostly declare no TimeoutSecs (only the Change & Repeat boss's five do), and
// the boss TimeLimit is a separate mechanic, so the boss screen is deliberately
// left alone.
func (m *Model) revealExerciseHintOnDeadline() {
	state := m.TrainerGameState
	if state == nil || state.CurrentExercise == nil {
		return
	}

	switch m.Screen {
	case ScreenTrainerLesson, ScreenTrainerPractice:
	default:
		return
	}

	if !state.HintDue() {
		return
	}

	message := trainerHintLabel(state.CurrentExercise)
	if message == "" {
		return
	}

	if m.TrainerMessage != message {
		m.TrainerMessage = message
	}
}

// trainerHintLabel is the feedback line the hint key reveals and the deadline
// reveal writes: the exercise's hint behind the one marker the trainer uses, or
// "" when there is no hint. The empty case is the reason this is one function.
// The label used to be built inline as "💡 Hint: " + hint, with no check, so an
// exercise whose hint was legitimately dropped -- the mission/hint rule lets a
// hint that adds no mechanism go -- rendered a bare label with nothing after
// the marker. Returning "" here is the single place that decides "no hint means
// no label", so the lesson, the practice and the boss handlers, and the
// automatic deadline reveal, cannot disagree about what a missing hint looks
// like.
func trainerHintLabel(exercise *trainer.Exercise) string {
	if exercise == nil || exercise.Hint == "" {
		return ""
	}
	return "💡 Hint: " + exercise.Hint
}

// expireBossStepOnDeadline charges the clock when a boss step is left
// unanswered past its own TimeLimit. It mirrors the wrong-answer branch of
// handleTrainerBossKeys: the canonical recorder spends one life and records one
// attempt, the solution is shown, and the player stays on the same step with a
// fresh window. The guard is what makes the 100ms tick idempotent: the recorder
// re-arms the deadline a full limit ahead, so only the first tick of an expired
// window fires. Defeat ends the fight exactly as a wrong last answer does.
func (m *Model) expireBossStepOnDeadline() {
	state := m.TrainerGameState
	if state == nil || !state.IsBossMode || state.CurrentBoss == nil {
		return
	}
	if m.Screen != ScreenTrainerBoss {
		return
	}
	if !state.BossDeadlinePassed() {
		return
	}

	solutionHint := ""
	if state.CurrentExercise != nil {
		solutionHint = trainer.FormatSolutionsHint(state.CurrentExercise)
	}

	state.RecordBossStepTimeout()
	m.TrainerInput = ""

	if state.BossLives <= 0 {
		m.TrainerLastCorrect = false
		m.TrainerMessage = "💀 DEFEATED! Solution was: " + solutionHint
		m.Screen = ScreenTrainerBossResult
		return
	}

	livesStr := trainerLivesGlyphs(state.BossLives, state.CurrentBoss.Lives)
	m.TrainerMessage = "⏰ Time's up! Was: " + solutionHint + " | Lives: " + livesStr
}

// trainerResetKey is the trainer menu key that erases the whole profile. It is
// the shifted form of the per-module [r] reset, so terminal input delivers it as
// a distinct rune without a modifier chord, and no other menu key uses it.
const trainerResetKey = "R"

// trainerResetPrompt is shown while a whole-profile reset is armed. The menu
// treats this prompt as the armed state, so arm-then-confirm needs no extra
// model field: the first [R] puts the prompt on screen, and only a second [R]
// clears the profile. Any other key cancels by clearing the prompt.
const trainerResetPrompt = "⚠️ Press [R] again to erase ALL trainer progress · any other key cancels"

// handleTrainerMenuKeys handles module selection in the trainer
func (m Model) handleTrainerMenuKeys(key string) (tea.Model, tea.Cmd) {
	// A whole-profile reset is armed. Only a second [R] clears the profile; every
	// other key, escape included, cancels and is consumed, so a stray keystroke
	// cannot half-commit the wipe.
	if m.TrainerMessage == trainerResetPrompt {
		if key != trainerResetKey {
			m.TrainerMessage = ""
			return m, nil
		}
		return m.clearTrainerProfile()
	}

	switch key {
	case "up", "k":
		if m.TrainerCursor > 0 {
			m.TrainerCursor--
		}
	case "down", "j":
		if m.TrainerCursor < len(m.TrainerModules)-1 {
			m.TrainerCursor++
		}
	case "enter", " ":
		// Select module and start lesson
		module := m.TrainerModules[m.TrainerCursor]

		if !m.TrainerStats.IsModuleUnlocked(module.ID) {
			m.TrainerMessage = "🔒 Module locked! Complete previous boss first."
			return m, nil
		}

		// Start lessons for the module
		lessons := trainer.GetLessons(module.ID)
		if len(lessons) == 0 {
			m.TrainerMessage = "No lessons for this module yet. Choose another module."
			return m, nil
		}

		// Initialize game state with lesson count
		m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
		progress := m.TrainerStats.GetModuleProgress(module.ID)
		progress.LessonsTotal = len(lessons)
		m.TrainerGameState.StartLesson(module.ID)
		m.TrainerInput = ""
		m.TrainerMessage = ""
		m.Screen = ScreenTrainerLesson
	case "l":
		// L key for Lesson mode (if unlocked)
		if m.TrainerCursor < len(m.TrainerModules) {
			module := m.TrainerModules[m.TrainerCursor]
			if m.TrainerStats.IsModuleUnlocked(module.ID) {
				lessons := trainer.GetLessons(module.ID)
				if len(lessons) > 0 {
					m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
					progress := m.TrainerStats.GetModuleProgress(module.ID)
					progress.LessonsTotal = len(lessons)
					m.TrainerGameState.StartLesson(module.ID)
					m.TrainerInput = ""
					m.TrainerMessage = ""
					m.Screen = ScreenTrainerLesson
				}
			}
		}
	case "p":
		// P key for Practice mode (if ready)
		if m.TrainerCursor < len(m.TrainerModules) {
			module := m.TrainerModules[m.TrainerCursor]
			if m.TrainerStats.IsPracticeReady(module.ID) {
				// Check if practice is complete
				progress := m.TrainerStats.GetModuleProgress(module.ID)
				if progress.IsPracticeComplete(module.ID) {
					m.TrainerMessage = "🎉 Practice complete! All exercises mastered! Press [r] to reset."
					return m, nil
				}

				m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
				m.TrainerGameState.StartPractice(module.ID)

				// Check if we got an exercise (shouldn't fail if not complete, but safety check)
				if m.TrainerGameState.CurrentExercise == nil {
					m.TrainerMessage = "🎉 Practice complete! All exercises mastered! Press [r] to reset."
					return m, nil
				}

				m.TrainerInput = ""
				m.TrainerMessage = ""
				m.Screen = ScreenTrainerPractice
			} else {
				m.TrainerMessage = "Complete all lessons first to unlock practice!"
			}
		}
	case "r":
		// R key to reset practice progress for selected module
		if m.TrainerCursor < len(m.TrainerModules) {
			module := m.TrainerModules[m.TrainerCursor]
			if m.TrainerStats.IsModuleUnlocked(module.ID) {
				progress := m.TrainerStats.GetModuleProgress(module.ID)
				progress.ResetModulePractice()
				m.TrainerMessage = "🔄 Practice progress reset for " + module.Name + ". Try again!"
				saveTrainerStats(&m)
			} else {
				m.TrainerMessage = "🔒 Module locked. Complete previous boss first."
			}
		}
	case "b":
		// B key for Boss fight (if ready)
		if m.TrainerCursor < len(m.TrainerModules) {
			module := m.TrainerModules[m.TrainerCursor]
			if m.TrainerStats.IsBossReady(module.ID) {
				boss := trainer.GetBoss(module.ID)
				if boss != nil {
					m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
					m.TrainerGameState.StartBoss(module.ID)
					m.TrainerInput = ""
					m.TrainerMessage = ""
					m.Screen = ScreenTrainerBoss
				} else {
					m.TrainerMessage = "This module has no boss fight yet. Try another module."
				}
			} else {
				m.TrainerMessage = "Finish every lesson and reach 80% practice accuracy to unlock the boss."
			}
		}
	case "q":
		// Save stats and go back to main menu. The message is cleared first so a
		// failed save can leave only its warning, the way escape already does:
		// otherwise a stale trainer message would be carried onto the main menu and
		// shown as if it were about the save.
		m.TrainerMessage = ""
		saveTrainerStats(&m)
		m.Screen = ScreenMainMenu
		m.Cursor = 0
	case trainerResetKey:
		// First press: arm the whole-profile reset and say what it wants.
		m.TrainerMessage = trainerResetPrompt
	}

	return m, nil
}

// saveTrainerStats persists the trainer profile and reports a failed save on the
// trainer message line instead of discarding the error. clearTrainerProfile
// already checked its own save; every other save went straight to
// trainer.SaveStats, so on a read-only or missing HOME the player lost progress
// with no warning. A caller that sets its own message must call this after it,
// so the warning replaces the success text rather than being wiped by it.
func saveTrainerStats(m *Model) {
	if m.TrainerStats == nil {
		return
	}
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		m.TrainerMessage = "⚠️ Could not save trainer progress: " + err.Error()
	}
}

// clearTrainerProfile erases the whole trainer profile and persists the empty
// one through the same save path the rest of the trainer uses, so the menu and
// the stats file agree without a restart. It tolerates a missing or unreadable
// profile (TrainerStats nil), which is what LoadStats returns then.
func (m Model) clearTrainerProfile() (tea.Model, tea.Cmd) {
	// Remove any stale file first, then write the canonical empty profile, so a
	// corrupt stats file cannot survive the reset.
	if err := trainer.ResetStats(); err != nil {
		m.TrainerMessage = "⚠️ Could not erase trainer progress: " + err.Error()
		return m, nil
	}

	m.TrainerStats = trainer.NewUserStats()
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		m.TrainerMessage = "⚠️ Could not save erased progress: " + err.Error()
		return m, nil
	}

	m.TrainerCursor = 0
	m.TrainerMessage = "🧹 All trainer progress erased. Start fresh!"
	return m, nil
}

// trainerControlChars maps the control key names the trainer accepts as answer
// input to the bytes the engine parses for them. It mirrors the control keys
// handled by trainer.SimulateMotionsWithSelection (\x04, \x15, \x06 and \x02)
// plus trainer.SimulateEditing's redo (\x12) and blockwise visual (\x16); a
// ctrl+ combination absent here has no meaning in either engine, so both
// exercise handlers ignore it instead of typing its literal name into the
// answer. ctrl+e is the one value that is not a control character: it types
// trainer.EscToken, because Esc is the trainer's global exit key and an insert
// answer that has to leave insert mode cannot be spelled any other way. This is
// the single accepted set shared by the lesson/practice and boss handlers.
var trainerControlChars = map[string]string{
	"ctrl+d": "\x04",
	"ctrl+u": "\x15",
	"ctrl+f": "\x06",
	"ctrl+b": "\x02",
	"ctrl+r": "\x12",
	"ctrl+v": "\x16",
	"ctrl+e": trainer.EscToken,
}

// backspaceTrainerInput removes the last unit the player typed from the answer.
// The escape token went in as one keystroke, so it comes out as one keystroke
// rather than leaving the half-token "<Es" behind for the engine to reject;
// anything else loses its last byte. This is interface behaviour, not a Vim
// command: the engine never sees a backspace, which is why insert mode reports
// one as unrecognized.
func backspaceTrainerInput(input string) string {
	if strings.HasSuffix(input, trainer.EscToken) {
		return input[:len(input)-len(trainer.EscToken)]
	}
	if len(input) == 0 {
		return input
	}
	return input[:len(input)-1]
}

// handleTrainerExerciseKeys handles input during lesson/practice exercises
func (m Model) handleTrainerExerciseKeys(key string) (tea.Model, tea.Cmd) {
	if m.TrainerGameState == nil {
		m.Screen = ScreenTrainerMenu
		return m, nil
	}

	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		m.Screen = ScreenTrainerMenu
		return m, nil
	}

	switch key {
	case "pgup", "pgdown":
		// PgUp and PgDn scroll the code window, which is how a code block longer
		// than the frame is still readable. They are key names rather than
		// printable characters, so they cannot collide with typing, with the hint
		// key, with submit or with back, and neither engine parses them.
		m.scrollTrainerCode(key == "pgdown")
		return m, nil

	case "backspace":
		// Remove the last typed unit from the input.
		m.TrainerInput = backspaceTrainerInput(m.TrainerInput)
		return m, nil

	case "enter":
		// Submit answer
		if m.TrainerInput == "" {
			return m, nil
		}

		// Validate answer using detailed validation
		validation := trainer.ValidateAnswerDetailed(exercise, m.TrainerInput)
		// The result screen reads the answer's result, so keep the validation that
		// describes this answer. It is the only writer of this field.
		m.TrainerValidation = &validation

		if validation.IsCorrect {
			// The game state owns the answer clock: it stamps the moment the
			// exercise was presented, so the measured time is passed on instead of
			// a fixed placeholder.
			m.TrainerGameState.RecordCorrectAnswer(m.TrainerGameState.ElapsedSeconds(), validation.IsOptimal)
			m.TrainerLastCorrect = true

			if validation.IsOptimal {
				m.TrainerMessage = "✨ Perfect! Optimal solution!"
			} else if validation.IsInSolutions {
				// Valid predefined solution but not optimal
				m.TrainerMessage = "✓ Correct! But " + exercise.Optimal + " is more efficient."
			} else {
				// Creative solution that works but not in predefined list
				m.TrainerMessage = "✓ Correct! Creative solution! Optimal: " + exercise.Optimal
			}
		} else {
			m.TrainerGameState.RecordIncorrectAnswer()
			m.TrainerLastCorrect = false
			// Show all valid solutions, not just optimal. A buffer-verified answer
			// also names what diverged, derived from the compared results rather
			// than from the keystrokes; both are valid answers, so the solutions
			// stay listed.
			mismatch := ""
			if summary := validation.MismatchSummary(); summary != "" {
				mismatch = summary + ". "
			}
			m.TrainerMessage = "✗ Incorrect. " + mismatch + "Solutions: " + trainer.FormatSolutionsHint(exercise)
		}

		// Record practice result for intelligent practice system
		if m.TrainerGameState.IsPracticeMode && exercise.ID != "" {
			progress := m.TrainerStats.GetModuleProgress(m.TrainerGameState.CurrentModule)
			progress.RecordPracticeResult(exercise.ID, validation.IsCorrect)
			saveTrainerStats(&m)
		}

		m.Screen = ScreenTrainerResult
		return m, nil

	case "tab":
		// Show hint. trainerHintLabel returns nothing for a hint that was
		// dropped, so an exercise without a hint gets no bare label.
		if label := trainerHintLabel(exercise); label != "" {
			m.TrainerMessage = label
		}
		return m, nil

	default:
		// Add character to input (filter control keys)
		// Accept single printable chars, space, and the control combinations
		// the simulator can parse. Anything else is ignored.
		if len(key) == 1 {
			m.TrainerInput += key
		} else if key == "space" {
			m.TrainerInput += " "
		} else if control, ok := trainerControlChars[key]; ok {
			m.TrainerInput += control
		}
	}

	return m, nil
}

// handleTrainerBossKeys handles input during boss fights
func (m Model) handleTrainerBossKeys(key string) (tea.Model, tea.Cmd) {
	if m.TrainerGameState == nil || m.TrainerGameState.CurrentBoss == nil ||
		m.TrainerGameState.CurrentExercise == nil {
		m.Screen = ScreenTrainerMenu
		return m, nil
	}

	switch key {
	case "pgup", "pgdown":
		// PgUp and PgDn scroll the code window, shared with the lesson and
		// practice screens: a boss step is the longest code the trainer shows, so
		// it needs the keys most.
		m.scrollTrainerCode(key == "pgdown")
		return m, nil

	case "backspace":
		m.TrainerInput = backspaceTrainerInput(m.TrainerInput)
		return m, nil

	case "tab":
		// The boss screen reveals its step's hint on the same terms as the
		// exercise screen: trainerHintLabel's label, its empty check, and no
		// label at all when the step carries none. Before this case the hint the
		// rewritten Change & Repeat boss steps ship was unreachable copy: it sat
		// in the data and no boss key could show it.
		if label := trainerHintLabel(m.TrainerGameState.CurrentExercise); label != "" {
			m.TrainerMessage = label
		}
		return m, nil

	case "enter":
		if m.TrainerInput == "" {
			return m, nil
		}

		boss := m.TrainerGameState.CurrentBoss
		// The game state owns which step is on screen and when it was presented,
		// so answer the exercise it is presenting instead of re-pointing it from
		// the UI: that mutation was the second place the session exercise was set.
		answered := m.TrainerGameState.CurrentExercise
		isCorrect := trainer.ValidateAnswer(answered, m.TrainerInput)
		isOptimal := trainer.IsOptimalAnswer(answered, m.TrainerInput)

		if isCorrect {
			// The shared recorder owns streak, score and boss attempt accounting. It
			// measures the answer from the step's presentation, so a retry after a
			// mistake is honestly slower.
			m.TrainerGameState.RecordCorrectAnswer(m.TrainerGameState.ElapsedSeconds(), isOptimal)
			m.TrainerInput = ""

			if !m.TrainerGameState.NextBossExercise() {
				// Boss defeated!
				m.TrainerGameState.RecordBossVictory()
				m.TrainerLastCorrect = true
				m.TrainerMessage = "🏆 VICTORY! You defeated " + boss.Name + "!"
				m.Screen = ScreenTrainerBossResult
			} else {
				if isOptimal {
					m.TrainerMessage = "✨ Perfect! Next challenge..."
				} else {
					// The answered step, not the next one the fight has moved on to.
					m.TrainerMessage = "✓ Good! (Optimal: " + answered.Optimal + ") Next..."
				}
			}
		} else {
			// The shared recorder owns the life cost, the attempt and the reset
			// streak. SHOW THE CORRECT SOLUTION.
			m.TrainerGameState.RecordIncorrectAnswer()
			m.TrainerInput = ""

			// Format the solution hint
			solutionHint := trainer.FormatSolutionsHint(answered)

			if m.TrainerGameState.BossLives <= 0 {
				// Game over - show final solution
				m.TrainerLastCorrect = false
				m.TrainerMessage = "💀 DEFEATED! Solution was: " + solutionHint
				m.Screen = ScreenTrainerBossResult
			} else {
				// Still has lives - show solution and remaining lives
				livesStr := trainerLivesGlyphs(m.TrainerGameState.BossLives, boss.Lives)
				m.TrainerMessage = "✗ Wrong! Was: " + solutionHint + " | Lives: " + livesStr
			}
		}

		return m, nil

	default:
		// Add character to input
		// Accept single printable chars, space, and the same control
		// combinations as the exercise handler.
		if len(key) == 1 {
			m.TrainerInput += key
		} else if key == "space" {
			m.TrainerInput += " "
		} else if control, ok := trainerControlChars[key]; ok {
			m.TrainerInput += control
		}
	}

	return m, nil
}

// handleTrainerResultKeys handles the result screen after an exercise
func (m Model) handleTrainerResultKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", " ":
		// Continue to next exercise
		if m.TrainerGameState == nil {
			m.Screen = ScreenTrainerMenu
			return m, nil
		}

		var hasNext bool
		if m.TrainerGameState.IsPracticeMode {
			// Use intelligent practice selection
			hasNext = m.TrainerGameState.NextPracticeExercise()
		} else {
			// Lesson mode uses sequential
			hasNext = m.TrainerGameState.NextExercise()
		}

		if hasNext {
			m.TrainerInput = ""
			m.TrainerMessage = ""
			if m.TrainerGameState.IsLessonMode {
				m.Screen = ScreenTrainerLesson
			} else {
				m.Screen = ScreenTrainerPractice
			}
		} else {
			// Session complete. The save runs after the celebration message so a
			// failed save is reported instead of being hidden behind it.
			if m.TrainerGameState.IsPracticeMode {
				m.TrainerMessage = "🎉 All exercises mastered! You're a Vim master! 🏆"
			} else {
				m.TrainerMessage = "🎉 Lesson complete! Practice mode unlocked!"
			}
			saveTrainerStats(&m)
			m.Screen = ScreenTrainerMenu
		}

	case "q":
		// Return to menu
		saveTrainerStats(&m)
		m.Screen = ScreenTrainerMenu
	}

	return m, nil
}

// handleTrainerBossResultKeys handles the result screen after a boss fight
func (m Model) handleTrainerBossResultKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", " ", "q":
		// Return to menu
		m.TrainerMessage = ""
		saveTrainerStats(&m)
		m.Screen = ScreenTrainerMenu
	}

	return m, nil
}
