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
	// repository checkout onto the model, with the directory they were read from so
	// the switch can compare an installed file against the file the repository
	// ships there.
	dotfilesThemesLoadedMsg struct {
		themes  []themeDefinition
		repoDir string
		err     error
	}

	// dotfilesThemeChangedMsg is the result of applying or undoing the dotfiles
	// theme. A failure stays on the section as a notice, like the desktop switch.
	// reload is the per-tool list an apply or undo produced: what the installer
	// brought forward and what the user still has to do. It is empty for a dry
	// run, which changes no file and so has nothing to reload.
	dotfilesThemeChangedMsg struct {
		notice string
		reload []themeReloadTool
		err    error
	}

	// themeRefreshDetectedMsg carries the files the refresh found out of date. A
	// failure stays on the picker as a notice, like the switch's own failures.
	themeRefreshDetectedMsg struct {
		candidates []themeRefreshCandidate
		err        error
	}

	// themeRefreshDoneMsg carries the result of a refresh: the record that makes
	// it reversible, the paragraphs the review shows, and the short notice.
	themeRefreshDoneMsg struct {
		record     *dotfilesThemeRecord
		paragraphs []string
		notice     string
		err        error
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

	// wslResourceLoadedMsg carries the WSL resource read onto the section: the
	// destination, the host capacities, the recommendation derived from them and
	// the managed values the file holds. It is read in a command because detecting
	// the host runs powershell.exe.
	wslResourceLoadedMsg struct {
		state wslResourceState
	}

	// wslResourceWrittenMsg carries the result of a .wslconfig write back to
	// Update, which is where the screen's notice is set. wrote says whether the
	// write happened, so a dry run leaves the draft alone.
	wslResourceWrittenMsg struct {
		notice string
		wrote  bool
		err    error
	}

	// shellAuditMeasuredMsg carries a finished measurement back to the screen: the
	// runs, their median and range, and the functions zprof named or the reason it
	// named none. The measurement starts the user's shell several times, so it is a
	// command rather than something a keypress does.
	shellAuditMeasuredMsg struct {
		state shellAuditState
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
	} // The slow animation tick is armed only when the run may animate. With
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
	// The last published release is read on that same gate, and on the record's
	// own lifetime: a run that may not animate -- a piped run, DOTFILES_ANIM=0 --
	// does not reach the network on its own, and a record younger than
	// updateCheckTTL means no start asks twice. The check is a command, never a
	// call here, so the network cannot be waited on.
	if cmd := m.updateCheckCmdFor(); cmd != nil {
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

// loadDotfilesThemesCmd reads the theme definitions from the first repository
// candidate that holds themes/*.toml. It is issued when the utilities section is
// first opened. It no longer waits for the clone step: launching the installer
// from inside the checkout is the normal case, and the checkout is found by
// looking at $DOTFILES_DIR, the clone, the working directory and its parents,
// then ~/dotfiles and ~/.dotfiles. The order lives in resolveThemeDefinitionsDir
// and is documented in docs/tui-installer.md.
func loadDotfilesThemesCmd(repoDir string) tea.Cmd {
	return func() tea.Msg {
		dir, err := resolveThemeDefinitionsDir(repoDir)
		if err != nil {
			return dotfilesThemesLoadedMsg{err: err}
		}
		defs, err := loadThemeDefinitions(dir)
		return dotfilesThemesLoadedMsg{themes: defs, repoDir: dir, err: err}
	}
}

// dotfilesThemesCmdIfNeeded reads the definitions the first time the utilities
// section is opened. It returns no command only when they have already been
// read: the checkout is found by resolution, not by the clone step, so an empty
// RepoDir is no longer a reason to skip the read.
func (m *Model) dotfilesThemesCmdIfNeeded() tea.Cmd {
	if m.DotfilesThemes != nil {
		return nil
	}
	return loadDotfilesThemesCmd(m.RepoDir)
}

// loadWSLResourceStateCmd reads the WSL resource state off the update loop: the
// Windows profile lookup, the host query and the file read all touch the
// machine, and a screen must never block on interop.
func loadWSLResourceStateCmd(repoDir string, isWSL bool) tea.Cmd {
	return func() tea.Msg {
		return wslResourceLoadedMsg{state: loadWSLResourceState(repoDir, isWSL)}
	}
}

// wslResourceStateCmdIfNeeded reads the WSL resource state the first time the
// utilities section is opened, the way the theme definitions are read. It
// returns no command only when the read has already finished, so the section's
// own state -- available or not -- is what it draws.
func (m *Model) wslResourceStateCmdIfNeeded() tea.Cmd {
	if m.WSLState.Resolved && !m.WSLState.Refreshing {
		return nil
	}
	return loadWSLResourceStateCmd(m.RepoDir, m.SystemInfo != nil && m.SystemInfo.IsWSL)
}

// wslResourceWriteCmd runs one .wslconfig write off the update loop, through the
// same content builder and the same writer the installation step uses. The state
// travels with it, so what is written is exactly what the screen showed, and the
// answer carries whether it wrote: a dry run must not re-read the file, which
// would silently discard the draft the user was editing.
func (m Model) wslResourceWriteCmd() tea.Cmd {
	state := m.WSLState
	return func() tea.Msg {
		notice, wrote, err := writeWSLResourceDraft(state, "utilities")
		return wslResourceWrittenMsg{notice: notice, wrote: wrote, err: err}
	}
}

// shellAuditCmd runs one measurement off the update loop. The resolved state
// travels with it, so what is measured is the shell the screen named, and the
// answer is the whole state the screen draws from. It changes nothing on the
// machine: the runs start the login shell and read back what zprof wrote into a
// directory of the utility's own.
func (m Model) shellAuditCmd() tea.Cmd {
	state := m.ShellAudit
	return func() tea.Msg {
		return shellAuditMeasuredMsg{state: measureShellAudit(state, defaultShellAuditProbe())}
	}
}

// startThemeActivity opens the picker's activity slot with the line the press
// stands behind. It is the same slot the result will use: the row is drawn at
// the height the outcome needs from the press on, so the list under the cursor
// is the same list at the same height when the outcome lands.
func (m *Model) startThemeActivity(pending string) {
	m.ThemeNotice = ""
	m.ThemeActivity = &themeActivity{Pending: pending}
}

// themeActivityPending says whether a dotfiles-theme switch is running, so the
// list's rows can stay inert until it reports and a second write cannot start
// beside the first.
func (m Model) themeActivityPending() bool {
	return m.ThemeActivity != nil && m.ThemeActivity.Pending != ""
}

// finishThemeActivity fills the slot the pending line opened with the outcome,
// in the same rows. A switch is only ever started from the picker, so a slot
// that is gone means the picker was left while the switch ran: the outcome then
// goes where leaving the picker put it, the section's notice, rather than being
// dropped. The reducer is followed by one paragraph on a success and one line on
// a failure, which is why the arguments are paragraphs and not a whole view.
func (m *Model) finishThemeActivity(paragraphs ...string) {
	if m.ThemeActivity == nil {
		m.ThemeNotice = strings.Join(paragraphs, " ")
		return
	}
	m.ThemeNotice = ""
	m.ThemeActivity.Pending = ""
	m.ThemeActivity.Result = paragraphs
	// An undo removes the row it was pressed on, so the cursor is settled onto a
	// row the frame can still draw rather than left on the separator that took
	// its place. An apply leaves the cursor where it was.
	m.settleThemePickerCursor()
}

// applyDotfilesThemeCmd runs one dotfiles-theme switch off the update loop,
// behind the same dry-run gate as the desktop switch. The repository directory
// the definitions were read from is what lets the switch recognise an installed
// file whose content still matches what the repository ships.
func (m Model) applyDotfilesThemeCmd(def themeDefinition) tea.Cmd {
	return func() tea.Msg {
		homeDir := os.Getenv("HOME")
		rec, notice, err := applyDotfilesTheme(homeDir, m.DotfilesRepoDir, def)
		if err != nil {
			return dotfilesThemeChangedMsg{notice: notice, err: err}
		}
		if rec == nil {
			// A dry run wrote nothing, so no tool is holding a stale file.
			return dotfilesThemeChangedMsg{notice: notice}
		}
		// The files changed; the list says which tool was reached and which was not.
		reload := reloadThemeTools(homeDir)
		for _, tool := range reload {
			SendLog("utilities", fmt.Sprintf("Theme reload: %s — %s", tool.Tool, tool.Note))
		}
		return dotfilesThemeChangedMsg{notice: notice, reload: reload}
	}
}

// undoDotfilesThemeCmd puts the recorded blocks back off the update loop. It
// reloads the same way an apply does: putting the old bytes back changes the
// files too, so the tools that were told about the new theme have to be told
// about the old one.
func undoDotfilesThemeCmd(rec dotfilesThemeRecord) tea.Cmd {
	return func() tea.Msg {
		notice, err := undoDotfilesTheme(rec)
		if err != nil {
			return dotfilesThemeChangedMsg{notice: notice, err: err}
		}
		if dryRun() {
			// A dry run put nothing back, so no tool is holding a stale file.
			return dotfilesThemeChangedMsg{notice: notice}
		}
		homeDir := os.Getenv("HOME")
		reload := reloadThemeTools(homeDir)
		for _, tool := range reload {
			SendLog("utilities", fmt.Sprintf("Theme reload: %s — %s", tool.Tool, tool.Note))
		}
		return dotfilesThemeChangedMsg{notice: notice, reload: reload}
	}
}

// detectThemeRefreshCmd lists the installed theme files that are not up to date
// off the update loop. It only reads: the review it opens is what names the
// files before any of them is written.
func detectThemeRefreshCmd(homeDir string, defs []themeDefinition) tea.Cmd {
	return func() tea.Msg {
		return themeRefreshDetectedMsg{candidates: findThemeRefreshCandidates(homeDir, defs)}
	}
}

// refreshThemeFilesCmd runs the confirmed refresh off the update loop, behind
// the same dry-run gate as the switch.
func refreshThemeFilesCmd(homeDir string, defs []themeDefinition, candidates []themeRefreshCandidate) tea.Cmd {
	return func() tea.Msg {
		rec, paragraphs, notice, err := refreshThemeFiles(homeDir, defs, candidates)
		return themeRefreshDoneMsg{record: rec, paragraphs: paragraphs, notice: notice, err: err}
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
		// back to sleep. It also gives the user's reading/typing priority over a
		// pending autonomous stroll.
		m.CompanionIdle = 0
		m.pauseCompanionAfterInput()
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
		updated.pauseCompanionAfterInput()
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
		// The Restore row is inserted above Utilities when backups exist, so the
		// index the cursor held before this answer no longer names the same row.
		// The label is kept and re-found, so a row that arrived late cannot take
		// the keypress that was aimed at the row that was already there.
		held := m.selectedOption()
		m.AvailableBackups = msg.backups
		m.holdCursorOn(held)
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
		// The record names the theme the run wears; the definitions are what turns
		// that name into chrome. A run that opens with a theme applied reads them
		// here, so the first frame is already painted rather than the utilities
		// section being the first to find out which theme is on.
		if m.DotfilesThemeRecord != nil && m.DotfilesThemes == nil {
			return m, m.dotfilesThemesCmdIfNeeded()
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
		m.DotfilesRepoDir = msg.repoDir
		m.DotfilesThemesErr = ""
		return m, nil

	case terminalCapabilityMsg:
		// The report is filled once. It is read-only, so there is no draft to
		// protect and no write to refresh from; the first answer is the answer.
		if !m.TerminalCapabilities.Resolved {
			m.TerminalCapabilities = msg.caps
		}
		return m, nil

	case updateCheckMsg:
		// An answer for a channel this run has left is not an answer. The channel
		// row can be pressed while a check is in flight, so the older check's reply
		// must not overwrite the state the channel this run now follows owns. A
		// message with no channel -- none is produced in a real run -- is accepted,
		// which keeps the interface's own seam honest.
		if msg.channel != "" && msg.channel != m.updateChannel() {
			return m, nil
		}
		// A finished check replaces the state whole, the way a measurement does.
		// A failed check is a result here too: the reason travels in the record, so
		// the screen says why it does not know instead of saying it is current. The
		// last update's notice survives, because a check is not an update. The
		// cursor is held by the row it was naming before the menu changed: an
		// answer that offers a newer release inserts a row above Utilities, and an
		// index-based cursor would otherwise hand the next Enter to the update row.
		held := m.selectedOption()
		notice := m.UpdateCheck.Notice
		m.UpdateCheck = stateFromRecord(msg.record)
		m.UpdateCheck.Notice = notice
		m.holdCursorOn(held)
		return m, nil

	case updateAppliedMsg:
		// The swap is not an install step, so a failure stays on the screen as a
		// notice rather than taking the run to a failed screen. A refusal to touch a
		// binary a package manager owns arrives here as well, with the command that
		// does own the update in its message. The attempt is over either way, so the
		// in-flight mark is cleared or a failed press could never be asked for again.
		m.UpdateCheck.InFlight = false
		if msg.err != nil {
			m.UpdateCheck.Notice = msg.err.Error()
			return m, nil
		}
		// The file on disk is now the published release, so the button that offered
		// it is withdrawn even though this process keeps running the old build until
		// it restarts. The sentence leads with the outcome, because the main menu
		// shows only its first row and the tag is the part worth reading there; the
		// kept path follows, and is read whole on the update screen's own notice.
		installed := m.UpdateCheck.Latest
		m.UpdateCheck.Installed = true
		source := ""
		if m.UpdateCheck.Channel == channelDev {
			source = " (dev channel)"
		}
		m.UpdateCheck.Notice = fmt.Sprintf(
			"Updated to %s%s; the previous binary is kept at %s. Restart dotfiles to run the new release.",
			installed, source, msg.kept)
		return m, nil

	case wslResourceLoadedMsg:
		// The first read fills the state once: a later answer arriving after the
		// screen has been written would roll the draft back to what the file held.
		// The refresh a write asked for is the one exception -- it is what replaces
		// the values the write just changed -- so it is accepted too, and the state
		// it brings clears the refreshing mark by not carrying one.
		if !m.WSLState.Resolved || m.WSLState.Refreshing {
			m.WSLState = msg.state
		}
		return m, nil

	case wslResourceWrittenMsg:
		// The write is not an install step: a failure stays on the screen as a
		// notice, exactly as a theme switch failure does.
		if msg.err != nil {
			m.WSLNotice = msg.err.Error()
			return m, nil
		}
		m.WSLNotice = msg.notice
		if !msg.wrote {
			// A dry run changed nothing, so the draft the user was editing is what
			// the screen keeps showing.
			return m, nil
		}
		// What the file holds has changed, so the read half is refreshed from the
		// file the write left behind rather than from what the screen remembered.
		// The table the user was reading stays on screen while that read runs: the
		// refresh updates its values in place instead of replacing the body, so the
		// press produces one change, not two.
		m.WSLState.Refreshing = true
		return m, m.wslResourceStateCmdIfNeeded()

	case shellAuditMeasuredMsg:
		// The measurement is not an install step and cannot fail as one: every run
		// it could not complete is already part of the state it brings back, with
		// its reason. The state replaces the old one whole, so the screen never
		// shows a mixture of two measurements.
		m.ShellAudit = msg.state
		return m, nil

	case dotfilesThemeChangedMsg:
		// The dotfiles switch is not an install step, so a failure is told on the
		// picker rather than taking the run to a failed screen. The outcome is
		// not a navigation either: it lands in the rows the press put the pending
		// line in, on the list the user is still standing on, so one press is one
		// change on screen and the result does not move anything.
		if msg.err != nil {
			m.finishThemeActivity(msg.err.Error())
			return m, nil
		}
		// The record is what makes the change reversible, so it is re-read after
		// every successful change: an apply leaves one, an undo clears it.
		m.DotfilesThemeRecord = readDotfilesThemeRecord()
		if len(msg.reload) > 0 {
			// The files changed; the slot says which tool was reached and which was
			// not, one line per tool, in the rows the pending line already held.
			m.finishThemeActivity(themeReloadResultParagraphs(msg.reload)...)
			return m, nil
		}
		m.finishThemeActivity(msg.notice)
		return m, nil

	case themeRefreshDetectedMsg:
		// Detection opens the review. Nothing is written here or in the command:
		// the review is the list the user confirms.
		if msg.err != nil {
			m.ThemeNotice = msg.err.Error()
			return m, nil
		}
		m.ThemeRefreshCandidates = msg.candidates
		m.ThemeNotice = ""
		refreshable := 0
		for _, cand := range msg.candidates {
			if cand.Problem == "" {
				refreshable++
			}
		}
		if refreshable == 0 {
			if len(msg.candidates) == 0 {
				m.ThemeRefreshReview, m.ThemeRefreshDone = false, false
				m.ThemeRefreshResult = nil
				m.ThemeNotice = "Every installed theme file is already up to date."
				return m, nil
			}
			// Nothing can be refreshed, but the files that could not are still
			// named rather than silently dropped.
			var problems []string
			for _, cand := range msg.candidates {
				problems = append(problems, cand.Path+" — "+cand.Problem)
			}
			m.ThemeRefreshReview, m.ThemeRefreshDone = true, true
			m.ThemeRefreshResult = themeRefreshResultParagraphs(nil, nil, problems)
			m.Cursor = 0
			return m, nil
		}
		m.ThemeRefreshReview, m.ThemeRefreshDone = true, false
		m.ThemeRefreshResult = nil
		m.Cursor = 1 // Cancel is the safe default.
		return m, nil

	case themeRefreshDoneMsg:
		if msg.err != nil {
			m.ThemeNotice = msg.err.Error()
			return m, nil
		}
		m.DotfilesThemeRecord = readDotfilesThemeRecord()
		m.ThemeRefreshReview, m.ThemeRefreshDone = true, true
		m.ThemeRefreshResult = msg.paragraphs
		m.ThemeNotice = msg.notice
		m.Cursor = 0
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

	// Backspace is the second way to say "back": it means exactly what Esc means
	// on every screen, through this same dispatch, so the two cannot drift apart.
	// The one place it does not is the trainer's answer line, where backspace is
	// text -- it deletes the last unit the player typed, and taking that away to
	// make the map uniform would stop a player correcting an answer. Backspace
	// used to work on only four screens and to be announced on none, so the same
	// key did two different things and the user could not tell which.
	if key == "backspace" && !isTrainerInputScreen(m.Screen) {
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

	case ScreenThemePicker:
		return m.handleThemePickerKeys(key)

	case ScreenWSLResources:
		return m.handleWSLResourceKeys(key)

	case ScreenTerminalCapabilities:
		return m.handleTerminalCapabilitiesKeys(key)

	case ScreenShellAudit:
		return m.handleShellAuditKeys(key)

	case ScreenUpdate:
		return m.handleUpdateKeys(key)

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
	case ScreenRestoreBackup:
		// The list is the top of the restore flow, so Esc leaves it for the menu.
		m.Screen = ScreenMainMenu
		m.Cursor = 0
	case ScreenRestoreConfirm:
		// The confirmation is one level in: Esc is the "cancel" the footer names,
		// and it puts the cursor back on the backup it was opened from. It used to
		// share the list's case and skip that level.
		m.Screen = ScreenRestoreBackup
		m.Cursor = m.SelectedBackup
	case ScreenUtilities:
		// The section is reached straight from the menu, so Esc is the "back" its
		// footer names. It had no case here, which left the section's own esc
		// branch unreachable and the annotated key dead.
		m.Screen = ScreenMainMenu
		m.Cursor = 0
		m.ThemeNotice = ""
	case ScreenThemePicker:
		// The refresh review is a second state of the picker: Esc cancels it and
		// leaves the files alone, exactly as the review's handler and the "back"
		// in the footer say. Only the list itself steps back to the section.
		if m.ThemeRefreshReview {
			m.resetThemeRefresh()
			m.ThemeNotice = ""
			return m, nil
		}
		// The theme list is one level in from the utilities section, so Esc steps
		// back there rather than all the way to the main menu.
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.ThemeNotice = ""
		m.resetThemeRefresh()
	case ScreenWSLResources:
		// The WSL resource screen is one level in too, and leaving it clears the
		// write's notice so a later visit does not open on a stale result.
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.WSLNotice = ""
	case ScreenTerminalCapabilities:
		// The capability report is read-only and one level in; leaving it steps
		// back to the section that opened it.
		m.Screen = ScreenUtilities
		m.Cursor = 0
	case ScreenShellAudit:
		// The shell startup screen is one level in as well. Leaving it cancels
		// nothing: a measurement that is still running writes its answer onto the
		// model regardless, which is what keeps the number from being lost because
		// the user stepped back while it ran.
		m.Screen = ScreenUtilities
		m.Cursor = 0
	case ScreenUpdate:
		// The update screen steps back to the main menu, which is where its row now
		// lives. Leaving it clears the last update's notice so a later visit does
		// not open on a stale result. A check that is still running writes its
		// answer onto the model regardless, which is what keeps the answer from
		// being lost behind the step back.
		m.Screen = ScreenMainMenu
		m.Cursor = 0
		m.UpdateCheck.Notice = ""
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
	// The main menu is the root of the flow, so Esc has nowhere back to go. It
	// used to quit the whole application, which made this one unannounced global
	// key the most destructive key in the interface, while the footer announced
	// quit on [Space q] and the documentation said "Go back". Esc is a no-op here;
	// quitting keeps its own keys, and the key, the documentation and the footer
	// now say the same thing.
	case ScreenMainMenu:
		return m, nil
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
		return m, tea.Batch(m.dotfilesThemesCmdIfNeeded(), m.wslResourceStateCmdIfNeeded())
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
		case selected == updateInstallerRow:
			// The row is the button: pressing it runs the same verified swap
			// `--self-update` runs, off the update loop, and the result appears on
			// this screen in the notice slot above the menu.
			if !m.UpdateInstallable() || m.UpdateCheck.InFlight {
				return m, nil
			}
			m.UpdateCheck.InFlight = true
			m.UpdateCheck.Notice = "Downloading " + m.UpdateCheck.Latest + "…"
			return m, updateApplyCmd(m.UpdateCheck.Latest, m.UpdateTarget)
		case strings.HasPrefix(selected, updateChannelRowPrefix):
			// The row is the switch: pressing it changes which stream this run
			// follows, remembers the choice in the same record the answers live in, and
			// asks for a fresh answer on the new channel. The cursor stays on the row
			// because the option list's length does not change when its label does.
			m = m.setUpdateChannel(m.updateChannel().toggle())
			return m, m.startUpdateCheck()
		case strings.Contains(selected, "Utilities"):
			// The row and the `u` key reach the same section: the row is how a
			// user finds it, the key is the shortcut for someone who has.
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.ThemeNotice = ""
			return m, tea.Batch(m.dotfilesThemesCmdIfNeeded(), m.wslResourceStateCmdIfNeeded())
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
	case "esc":
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
			// The line the result will use says the switch has started, so a tool
			// that takes a moment to write is not a keypress that seemed to do
			// nothing.
			m.ThemeNotice = "Switching the desktop's theme to dark…"
			return m, applyThemeCmd(m.ThemeSwitch, true)
		case strings.Contains(selected, "light theme"):
			m.ThemeNotice = "Switching the desktop's theme to light…"
			return m, applyThemeCmd(m.ThemeSwitch, false)
		case selected == utilitiesThemeRow:
			// The theme list is one level in: the section opens the picker rather
			// than listing the themes itself. A previous refresh review is cleared so
			// a later visit starts from the list.
			m.Screen = ScreenThemePicker
			m.Cursor = 0
			m.ThemeNotice = ""
			m.resetThemeRefresh()
		case selected == utilitiesWSLRow:
			// The WSL resources are one level in for the same reason. The read is
			// started when the section opens, so this only resets the view.
			m.Screen = ScreenWSLResources
			m.Cursor = 0
			m.WSLNotice = ""
		case selected == utilitiesTerminalRow:
			// The capability report is one level in as well, and it is read-only.
			// The probe is armed here -- when the user opens the screen, never at
			// startup -- and it runs off the update loop behind a bounded query, so
			// a terminal that never answers cannot freeze the interface.
			m.Screen = ScreenTerminalCapabilities
			m.Cursor = 0
			return m, m.terminalCapabilityCmdIfNeeded()
		case selected == utilitiesShellAuditRow:
			// The shell's startup is one level in too. Nothing is measured on the way
			// in: starting the user's shell five times is a choice, and it is made on
			// that screen's own row.
			m.Screen = ScreenShellAudit
			m.Cursor = 0
		case strings.Contains(selected, "Undo") && m.themeUndoAvailable():
			m.ThemeNotice = "Putting the previous desktop theme back…"
			return m, undoThemeCmd(m.ThemeSwitch, *m.ThemeRecord)
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			m.ThemeNotice = ""
		}
	}

	return m, nil
}

// handleWSLResourceKeys drives the WSL resource screen. It only ever changes the
// cursor, the draft or the notice: the write is a command, so a slow filesystem
// cannot block the update loop, and the dry-run gate it runs behind is the one
// the installation step runs behind.
//
// The arrow keys (and h/l) move the value under the cursor by one step, `r` puts
// every row back on the host's recommendation, and the write row is the only row
// that touches the file.
func (m Model) handleWSLResourceKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	adjust := func(value, step, delta int) int { return wslAdjustValue(value, step, delta) }
	applyToCursor := func(delta int) {
		switch m.Cursor {
		case wslResourceRowMemory:
			m.WSLState.Draft.MemoryMB = adjust(m.WSLState.Draft.MemoryMB, wslMemoryStepMB, delta)
		case wslResourceRowProcessors:
			m.WSLState.Draft.Processors = adjust(m.WSLState.Draft.Processors, 1, delta)
		case wslResourceRowSwap:
			m.WSLState.Draft.SwapMB = adjust(m.WSLState.Draft.SwapMB, wslMemoryStepMB, delta)
		}
	}
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
	case "left", "h", "-":
		applyToCursor(-1)
		m.WSLNotice = ""
	case "right", "l", "+":
		applyToCursor(1)
		m.WSLNotice = ""
	case "r":
		// The recommendation is one key away rather than a number to remember.
		m.WSLState.Draft = m.WSLState.Plan
		m.WSLNotice = ""
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.WSLNotice = ""
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch {
		case options[m.Cursor] == wslWriteRow:
			// The write's own notice names the file before the write starts, so the
			// wait is visible and the result replaces it in the same rows.
			m.WSLNotice = fmt.Sprintf("Writing %s…", m.WSLState.Path)
			return m, m.wslResourceWriteCmd()
		case strings.Contains(options[m.Cursor], "Back"):
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.WSLNotice = ""
		}
	}

	return m, nil
}

// handleTerminalCapabilitiesKeys drives the capability report. The screen is
// read-only -- it writes nothing and runs nothing -- so the only thing a key
// does is leave. The probe itself is a command armed when the screen is opened,
// never work done here.
func (m Model) handleTerminalCapabilitiesKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter", " ":
		m.Screen = ScreenUtilities
		m.Cursor = 0
	}
	return m, nil
}

// handleShellAuditKeys drives the shell startup screen. It only ever changes the
// screen, the cursor or the one flag that says a measurement is in flight: the
// measurement itself is a command, because it starts the user's shell several
// times and a keypress must not block the update loop.
//
// The measure row is the only row that does anything. The result rows under it
// are the data and are inert by design: this screen offers no way to change the
// shell's configuration, so there is nothing for them to do.
func (m Model) handleShellAuditKeys(key string) (tea.Model, tea.Cmd) {
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
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch {
		case options[m.Cursor] == utilitiesShellAuditRow:
			// One measurement at a time: a second press while the first is still
			// running its five starts would start five more beside it and report
			// whichever finished last.
			if m.ShellAudit.Measuring || !m.ShellAudit.Available {
				return m, nil
			}
			m.ShellAudit.Measuring = true
			return m, m.shellAuditCmd()
		case strings.Contains(options[m.Cursor], "Back"):
			m.Screen = ScreenUtilities
			m.Cursor = 0
		}
	}

	return m, nil
}

// handleUpdateKeys drives this installer's own release screen. Like the other
// utilities screens it only ever changes the screen, the cursor or the notice:
// the check and the swap are both commands, so neither the network nor a disk can
// block the update loop.
//
// The refusal is here as well as in the swap itself, and deliberately so: a file a
// package manager owns gets a sentence and no row at all, so the press that would
// have been refused is never offered.
func (m Model) handleUpdateKeys(key string) (tea.Model, tea.Cmd) {
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
	case "esc":
		m.Screen = ScreenMainMenu
		m.Cursor = 0
		m.UpdateCheck.Notice = ""
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch {
		case options[m.Cursor] == updateInstallRow:
			// The swap is one command: the ownership question, the two downloads, the
			// checksum and the rename all happen off the update loop, and the row is
			// only ever drawn when this run may replace its own file.
			if !m.UpdateInstallable() || m.UpdateCheck.InFlight {
				return m, nil
			}
			m.UpdateCheck.InFlight = true
			m.UpdateCheck.Notice = "Downloading " + m.UpdateCheck.Latest + "…"
			return m, updateApplyCmd(m.UpdateCheck.Latest, m.UpdateTarget)
		case options[m.Cursor] == updateCheckRow:
			// Asking again is always allowed: an answer that is stale, wrong or
			// missing is exactly what a user asks a second time for. One check at a
			// time, so the last answer to arrive is the answer that was asked for.
			return m, m.startUpdateCheck()
		case strings.Contains(options[m.Cursor], "Back"):
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			m.UpdateCheck.Notice = ""
		}
	}

	return m, nil
}

// handleThemePickerKeys drives the dotfiles theme list. Like the utilities
// section it only ever changes the screen, the cursor or the notice: applying and
// undoing are commands, so a slow filesystem cannot block the update loop. The
// preview follows the cursor and writes nothing, which is why moving it costs no
// command here.
func (m Model) handleThemePickerKeys(key string) (tea.Model, tea.Cmd) {
	if m.ThemeRefreshReview {
		return m.handleThemeRefreshKeys(key)
	}

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
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.ThemeNotice = ""
		m.resetThemeRefresh()
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if m.themeActivityPending() {
			// One switch at a time. A second press while the first is still
			// writing would start a second write and the first result would be
			// replaced by whichever finished last, so the pending row is inert
			// until it reports.
			return m, nil
		}
		selected := options[m.Cursor]
		switch {
		case strings.HasPrefix(selected, "Apply the "):
			if def, ok := m.dotfilesThemeForRow(selected); ok {
				m.startThemeActivity(fmt.Sprintf("Applying the %s theme…", def.Name))
				return m, m.applyDotfilesThemeCmd(def)
			}
		case selected == themeRefreshRow:
			return m, detectThemeRefreshCmd(os.Getenv("HOME"), m.DotfilesThemes)
		case selected == dotfilesThemeUndoRow && m.DotfilesThemeRecord != nil:
			m.startThemeActivity("Undoing the last dotfiles theme change…")
			return m, undoDotfilesThemeCmd(*m.DotfilesThemeRecord)
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.ThemeNotice = ""
			m.resetThemeRefresh()
		}
	}

	return m, nil
}

// handleThemeRefreshKeys drives the refresh review: the confirmation before
// anything is written, and the result after. Cancel and esc leave every file as
// it is; the only command it returns is the confirmed refresh.
func (m Model) handleThemeRefreshKeys(key string) (tea.Model, tea.Cmd) {
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
	case "esc":
		m.resetThemeRefresh()
		m.ThemeNotice = ""
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := options[m.Cursor]
		switch {
		case m.ThemeRefreshDone:
			// The result's only row is the way back to the list.
			m.resetThemeRefresh()
			m.ThemeNotice = ""
		case selected == themeRefreshCancelRow:
			m.resetThemeRefresh()
		case strings.HasPrefix(selected, "Yes, refresh"):
			return m, refreshThemeFilesCmd(os.Getenv("HOME"), m.DotfilesThemes, m.ThemeRefreshCandidates)
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

	case "esc":
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
	case "esc":
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

// isTrainerInputScreen reports whether the screen reads backspace as text rather
// than as the "back" key. The lesson, practice and boss screens own the answer
// line, so a backspace there deletes the last unit the player typed; everywhere
// else Backspace means what Esc means. The trainer menu and the two result
// screens are not input screens: backspace steps back from them like Esc does.
func isTrainerInputScreen(s Screen) bool {
	switch s {
	case ScreenTrainerLesson, ScreenTrainerPractice, ScreenTrainerBoss:
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
