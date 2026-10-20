package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
	"github.com/albersg/dotfiles/installer/internal/portscan"
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

	// wslGuardRepairedMsg carries a guard repair back to Update: the state read
	// afterwards, and the error the install reported. The install runs in a command
	// because it writes two files and runs systemctl, and the read that follows it
	// is what the screen then reports rather than the install's own word for it.
	wslGuardRepairedMsg struct {
		state wslGuardState
		err   error
	}

	// wslTerminalOfferMsg carries the Windows Terminal offer the interactive WSL
	// step shows. It is built in a command because resolving the face asks the
	// Windows font store through interop, and the offer has to be on the model
	// before the installing screen draws it. The offer is read-only: no field here
	// has been written to disk.
	wslTerminalOfferMsg struct {
		offer      windowsTerminalOffer
		lines      []string
		profileDir string
		resolved   bool
	}

	// wslTerminalOfferAppliedMsg carries the result of the interactive WSL step's
	// answer back to Update, which logs it and then starts the rest of the step.
	// err is a Windows-side failure, reported in the log rather than on the error
	// screen for the same reason the non-interactive branch does not fail the
	// step: the shell is useful whether or not Windows Terminal was reached.
	wslTerminalOfferAppliedMsg struct {
		notice string
		err    error
	}

	// shellAuditMeasuredMsg carries a finished measurement back to the screen: the
	// runs, their median and range, and the functions the shell's own instrument
	// named or the reason it named none. The measurement starts the user's shell
	// several times, so it is a command rather than something a keypress does.
	//
	// It is the whole-state form, kept for the callers that have a finished state
	// already -- verification, a guard -- while a measurement the screen started
	// reports itself one event at a time through shellAuditEventMsg.
	shellAuditMeasuredMsg struct {
		state shellAuditState
	}

	// shellAuditEventMsg carries one thing that happened during a measurement: a
	// start that began or ended, and at the end the result itself. One event per
	// message is what lets the screen say which start it is waiting on and lets
	// the user stop the ones that are left, instead of showing a single wait that
	// cannot be interrupted.
	shellAuditEventMsg struct {
		event shellAuditEvent
	}

	// shellAuditClosedMsg reports that the events ended without a result, which is
	// the case of a measurement that died with its process. The screen stops
	// saying it is measuring rather than waiting for an answer that is not coming.
	shellAuditClosedMsg struct{}

	// shellAuditHistorySavedMsg reports how storing the measurements went. The
	// measurement itself is not affected by it: what failed is the file, and the
	// screen says so rather than letting the user believe their runs are kept.
	shellAuditHistorySavedMsg struct {
		err error
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
	// ConfigFingerprints is the before-run content hash of every configuration
	// path. The first step takes it, and it travels with the other recorded state
	// because the record built at the end of the run is written from the UI model,
	// which never saw the copy the step ran against.
	ConfigFingerprints map[string]string
	// ConfigFileIndex is the before-run digest of every file under every
	// configuration path, taken by the same step and travelling with it for the
	// same reason: the record built at the end of the run is written from the UI
	// model, which never saw the copy the step ran against.
	ConfigFileIndex map[string]string
}

// recordedState takes the state a step recorded off the model it ran against.
func (m Model) recordedState() stepRecordedState {
	return stepRecordedState{
		WorkDir:            m.WorkDir,
		RepoDir:            m.RepoDir,
		BackupDir:          m.BackupDir,
		ConfigFingerprints: m.ConfigFingerprints,
		ConfigFileIndex:    m.ConfigFileIndex,
	}
}

// applyTo copies the recorded state into the model the TUI keeps.
func (r stepRecordedState) applyTo(m *Model) {
	m.WorkDir = r.WorkDir
	m.RepoDir = r.RepoDir
	m.BackupDir = r.BackupDir
	m.ConfigFingerprints = r.ConfigFingerprints
	m.ConfigFileIndex = r.ConfigFileIndex
}

// recordRun writes the record of the run that just finished and puts it on the
// model, so the "Last install" panel shows the run that just finished without
// waiting for the next startup read. The record itself is built in state.go from
// what the run did to each configuration path and to each file under it, not from
// the pre-run detection; copying m.ExistingConfigs here is the defect reported in
// issue #239.
//
// It is called for a run that failed as well as for one that finished, and by the
// scripted route as well as by the interface. A run that stopped halfway is
// exactly the run whose changes a person needs to read -- those are the changes
// that are already on their machine -- and the record is the only place they are
// written down.
//
// The file write is deliberately unchecked: it is a convenience for the next run,
// and a read-only or missing state directory must not fail an install that already
// succeeded.
func (m *Model) recordRun(status runStatus, failedSteps []string) {
	// A dry run does nothing, so it has nothing to record -- and writing its empty record
	// over the one a real run left destroys the evidence the revert and the content check
	// read: the next --revert would report that the run changed nothing while the machine
	// still holds every file that run wrote. The rule lives at the one place a run records,
	// because the interface reaches it by routes of its own.
	if dryRun() {
		return
	}
	rec := lastInstallRecord(m, time.Now())
	rec.Status = status
	rec.FailedSteps = failedSteps
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
	// The last published release is read on the record's own lifetime, for every
	// real run whether or not it may animate: the user asked for the check when
	// dotfiles opens. A record younger than updateCheckTTL means no start asks
	// twice, and a test run is not armed at all. The check is a command, never a
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

// wslGuardInstaller and wslGuardReader are what the guard row runs and what it
// reads back afterwards. They are variables for the same reason stepExecutor is
// one: the row's own test has to observe the press without writing to /etc or
// restarting a real unit on the machine running the tests, and the two are kept
// apart so a test can prove the notice reports the read and not the install.
var (
	wslGuardInstaller = installWSLMemoryGuard
	wslGuardReader    = readWSLGuardState
)

// wslGuardRepairCmd installs the guard and then reads the host back, so the
// screen reports what the machine holds rather than what the install claims. It
// runs off the update loop for the same reason the write does: it writes to /etc
// and runs systemctl, and a render must never wait for either.
//
// The install returns an error only for a packaging defect -- a checkout that no
// longer ships the payload or the unit. A machine that cannot be guarded prints
// its reason and does not become an error, so the notice carries the reporting
// and the state carries the verdict.
func (m Model) wslGuardRepairCmd() tea.Cmd {
	repoDir := m.WSLState.RepoDir
	return func() tea.Msg {
		err := wslGuardInstaller(repoDir, "utilities")
		return wslGuardRepairedMsg{state: wslGuardReader(repoDir), err: err}
	}
}

// wslTerminalOfferCmd builds the interactive WSL step's Windows Terminal offer
// off the update loop: it resolves the Windows profile, reads the settings.json
// under it and asks the font store, so it runs as a command like every other
// machine touch. What it returns is the read-only offer and the lines the screen
// shows; the step writes nothing until the user answers. A profile that cannot
// be resolved skips the offer with the reason named rather than failing the
// step, exactly as the script's own Windows half does.
func (m Model) wslTerminalOfferCmd() tea.Cmd {
	repoDir := m.RepoDir
	return func() tea.Msg {
		profileDir, err := windowsUserProfile()
		if err != nil {
			return wslTerminalOfferMsg{lines: []string{fmt.Sprintf(
				"Skipping the Windows Terminal offer: %v. Set %s to override the lookup.", err, envWSLWindowsHome)}}
		}
		offer, lines, resolved := wslTerminalInteractiveOffer(repoDir, profileDir)
		return wslTerminalOfferMsg{offer: offer, lines: lines, profileDir: profileDir, resolved: resolved}
	}
}

// applyWSLTerminalOfferCmd performs the answer the user gave the interactive WSL
// step. A yes installs and verifies the font first when the store resolved no
// face, then writes through applyWindowsTerminalOffer, which copies the previous
// file beside itself before it writes; a no calls the same function with chosen
// false, so the refusal notice comes from the one place the write decision lives
// and nothing at all is written.
func (m Model) applyWSLTerminalOfferCmd(chosen bool) tea.Cmd {
	offer := *m.WSLTerminalOffer
	profileDir := m.WSLTerminalOfferProfileDir
	resolved := m.WSLTerminalOfferResolved
	return func() tea.Msg {
		if !chosen {
			notice, _, err := applyWindowsTerminalOffer(offer, false, wslStepID)
			return wslTerminalOfferAppliedMsg{notice: notice, err: err}
		}
		// Nothing resolved, so the installer's own family has to be registered
		// before its face can be written. installAndVerifyWindowsFont is the gate:
		// a family that does not verify is not written, and the gate's failure is
		// reported instead of a silent fallback face.
		if !resolved {
			if err := installAndVerifyWindowsFont(profileDir); err != nil {
				return wslTerminalOfferAppliedMsg{err: err}
			}
		}
		notice, _, err := applyWindowsTerminalOffer(offer, true, wslStepID)
		return wslTerminalOfferAppliedMsg{notice: notice, err: err}
	}
}

// appendInstallLog adds lines to the installing screen's log, keeping the same
// bound the step log uses so the model and the box agree on how much history
// exists. Empty lines are skipped: a notice that is nothing is not a log row.
func (m Model) appendInstallLog(lines ...string) Model {
	for _, line := range lines {
		if line == "" {
			continue
		}
		m.LogLines = append(m.LogLines, line)
	}
	if len(m.LogLines) > 20 {
		m.LogLines = m.LogLines[len(m.LogLines)-20:]
	}
	return m
}

// shellAuditProbe is the probe the screen measures with. The seam is the model's
// own field so a guard can drive the screen's start, progress and cancel path
// without starting the runner's real shell; in a real run the field is nil and the
// default probe is the only one reachable.
func (m Model) shellAuditProbe() shellAuditProbe {
	if m.shellAuditTestProbe != nil {
		return m.shellAuditTestProbe()
	}
	return defaultShellAuditProbe()
}

// shellAuditCmd starts one measurement off the update loop and answers its first
// event. The resolved state travels with it, so what is measured is the shell the
// screen named and the plan the rows show, and it changes nothing on the machine:
// the runs start the shell the user asked for and read back what the shell's own
// instrument wrote into a directory of the utility's own.
//
// The session it creates is returned as well as the command, because it is what a
// cancel reaches: a screen that only stopped listening would leave the user's
// shell being started behind it.
func (m Model) shellAuditCmd() (*shellAuditSession, tea.Cmd) {
	session := startShellAuditSession(m.ShellAudit, m.shellAuditProbe())
	return session, awaitShellAuditEvent(session)
}

// awaitShellAuditEvent reads one event from a measurement in flight. The reads are
// re-armed one at a time, so the update loop turns over between the starts and the
// frame redraws with them: a command that drained the channel would deliver the
// finished measurement in one step and the progress the user is watching would
// never be drawn.
func awaitShellAuditEvent(session *shellAuditSession) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-session.events
		if !ok {
			return shellAuditClosedMsg{}
		}
		return shellAuditEventMsg{event: event}
	}
}

// saveShellAuditHistoryCmd stores the measurements off the update loop, and it is
// the only thing on this screen that writes to the user's disk. It is a command
// for the same reason the install record is written from one: a render must not
// wait on a filesystem, and the answer the user is reading is already drawn.
func (m Model) saveShellAuditHistoryCmd() tea.Cmd {
	state := m.ShellAudit
	probe := m.shellAuditProbe()
	return func() tea.Msg {
		return shellAuditHistorySavedMsg{err: saveShellAuditHistory(state, probe)}
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
// its default panel, whichever path changed the screen. It is also where the two
// events that reach the creature from outside its own clock are turned into model
// state: a key wakes it, and a resize cancels a stroll whose stage just moved. The
// walking itself happens on the frame ticks that follow.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	screen := m.Screen
	if _, ok := msg.(tea.KeyMsg); ok {
		// A key wakes the companion and restarts the stretch that would put it back
		// to sleep. It does not cancel a stroll, reset the roam clock or hold the
		// idle counter below the point where autonomous play begins: the creature
		// owns reserved rows, never reflows anything and never touches the cursor, so
		// it has nothing to yield to a keypress, and suppressing it on every key is
		// what kept an active reader from ever seeing it move. See companionWake.
		m.companionWake()
	}
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		// A resize is the one event that can move the stage under a pending stroll,
		// so it is the one event that cancels one. It costs nothing when no stroll is
		// in progress: Bubble Tea sends an initial size when the program starts, and
		// that must not delay the first stroll by the calm interval.
		if m.CompanionRoamActive {
			m.pauseCompanionAfterInput()
		}
	}
	next, cmd := m.update(msg)

	updated, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	if updated.Screen != screen {
		updated.PanelIndex = 0
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

	case sshManagerLoadedMsg, sshEffectiveMsg, sshKeysLoadedMsg, sshAgentLoadedMsg,
		sshDiagnosticsMsg, sshTestedMsg, sshWrittenMsg, sshGeneratedMsg,
		sshCopiedMsg, sshAgentActionMsg:
		// The SSH manager's own answers, applied by one function in sshmanager.go so
		// the message types and the fields they fill live beside each other: a write
		// re-reads the configuration, a generated key re-reads the inventory, and an
		// agent action re-reads both. The case names every type on purpose -- a
		// default here would swallow the messages the rest of the file handles. The
		// third result says whether the message was one of them.
		if next, cmd, handled := m.handleSSHMessage(msg); handled {
			return next, cmd
		}
		return m, nil

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
		// A new run asks for the credential again and may offer the WSL decision
		// again: both are facts about this run, not about the model's whole life.
		m.SudoPrimed = false
		m.SudoPrimeAttempted = false
		m.WSLTerminalOfferDone = false
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
					// The run stops here, and the changes it already made are on
					// the machine: this record is what names them.
					m.recordRun(runStatusFailed, []string{m.Steps[i].Name})
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
		m.recordRun(runStatusComplete, nil)
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
		// startup read fills a model the writer did not already fill. The undo row
		// it can add is inserted above the section's other rows, so the cursor is
		// held by label here too.
		held := m.selectedOption()
		if m.ThemeRecord == nil {
			m.ThemeRecord = msg.record
		}
		if m.DotfilesThemeRecord == nil {
			m.DotfilesThemeRecord = msg.dotfiles
		}
		m.holdCursorOn(held)
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
		// The theme row is inserted at the TOP of the section's list, above every
		// row the user could already see, so the index the cursor holds no longer
		// names the row it was on. The label is kept and re-found, exactly as the
		// main menu does when the Restore row arrives: the read is armed when the
		// section opens, so the first frame is always drawn without it.
		held := m.selectedOption()
		m.DotfilesThemes = msg.themes
		m.DotfilesRepoDir = msg.repoDir
		m.DotfilesThemesErr = ""
		m.holdCursorOn(held)
		return m, nil

	case terminalCapabilityMsg:
		// The report is filled once. It is read-only, so there is no draft to
		// protect and no write to refresh from; the first answer is the answer.
		if !m.TerminalCapabilities.Resolved {
			m.TerminalCapabilities = msg.caps
		}
		return m, nil

	case installVerifyMsg:
		// The verdict is filled once, by the same rule: the check is read-only, so
		// the first answer is the answer and a return to the screen does not stat
		// the machine again.
		if !m.InstallVerify.Resolved {
			m.InstallVerify = installVerifyState{Resolved: true, Verdict: msg.verdict}
		}
		return m, nil

	case networkCheckMsg:
		// One check finishing while the run goes on. It is appended to the streamed
		// list and the next read is armed on the same run: the update loop keeps
		// reading while the run keeps producing, so a slow stage never hides a fast
		// one, and a run that is left in flight still lands its checks here.
		if m.Network.Running && m.Network.Run != nil {
			m.Network.Checks = append(m.Network.Checks, msg.check)
		}
		return m, waitNetworkEvent(m.Network.Run)

	case networkDoneMsg:
		// The finished report replaces the streamed checks whole, so the steps a stop
		// left unrun are named as skipped rather than missing. The cursor is held on
		// the row it was naming before the rows changed: a report that arrives while
		// the user is moving through the list must not hand the next Enter to a
		// different row.
		held := m.selectedOption()
		report := msg.report
		m.Network.Report = &report
		m.Network.Checks = nil
		m.Network.Running = false
		m.Network.Run = nil
		m.Network.ReportPath = networkReportPath(time.Now())
		if report.Stopped || msg.stopped {
			m.Network.Notice = "The run was stopped: the checks that finished are listed, and the rest are skipped."
		} else {
			m.Network.Notice = "The run finished. Move the cursor onto a step to read what it measured."
		}
		m.holdCursorOn(held)
		return m, nil

	case networkClosedMsg:
		// The event channel closed without a report, which is the run being
		// abandoned. The screen stops saying a run is in flight rather than waiting
		// for an answer that is not coming.
		m.Network.Running = false
		m.Network.Run = nil
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

	case doctorCheckDoneMsg:
		// A message from a run that was cancelled and restarted belongs to the run
		// that asked for it, never to the one on screen now. A run that has already
		// ended -- cancelled, or closed once every check answered -- takes no more
		// answers: the checks that were in flight when it ended are still on their
		// way, and writing one into the run now would rewrite a report that has
		// already been written. This is what makes cancelling the checks in flight
		// safe to implement.
		run := m.Doctor.Run
		if run == nil || run.ID != msg.RunID || !run.running() {
			return m, nil
		}
		run.Results[msg.Result.ID] = msg.Result
		if run.InFlight > 0 {
			run.InFlight--
		}
		run.Answered++
		if run.Canceled || run.Answered >= len(run.Plan) {
			// Nothing is left to start: the run was cancelled, or every check in
			// the plan has an answer.
			m.doctorFinishRun("")
			return m, nil
		}
		// The answer freed a place in the pool, and the progress line moves with it.
		cmd := m.doctorScheduleCmd()
		run.Message = m.doctorProgressLine()
		return m, cmd

	case doctorFixDoneMsg:
		m.Doctor.FixNotice = msg.Outcome.Notice
		if !msg.Outcome.Applied {
			return m, nil
		}
		// The finding that offered the fix is re-checked, so the list states the
		// state after the change rather than what the check found before it.
		m.Doctor.Pending = nil
		m.Screen = ScreenDoctor
		m.settleDoctorCursor()
		check, found := doctorCheckByID(msg.Finding.Check)
		if !found {
			m.Doctor.Notice = msg.Outcome.Notice
			return m, nil
		}
		m.Doctor.Notice = msg.Outcome.Notice
		return m, m.doctorStartRun([]doctorCheck{check}, false)

	case doctorReportWrittenMsg:
		switch {
		case msg.Err != "":
			m.Doctor.Notice = "The report could not be written: " + msg.Err
		case dryRun():
			m.Doctor.Notice = "A dry run wrote nothing. The report would have gone to " + doctorRelHomePath(msg.Path)
		default:
			m.Doctor.Notice = "Report written to " + doctorRelHomePath(msg.Path)
		}
		return m, nil

	case updateCenterReadMsg:
		// One manager's answer. The pass stores it and asks the next channel that can
		// be asked, so the screen fills in as the machine answers rather than waiting
		// for the slowest manager. A read that lands after the screen was left is
		// still stored: the answer belongs to the model, and losing it would make the
		// next visit pay for the same question again.
		return m.updateCenterHandleRead(msg)

	case updateCenterApplyDoneMsg:
		// One command of the confirmed sequence. The engine has already re-read the
		// manager this command belongs to, so the result carries what the manager says
		// now rather than what the exit status suggested. A cancelled command ends the
		// sequence: the remaining plans were not reached, and they are reported as
		// left out rather than run.
		return m.updateCenterHandleApplyDone(msg)

	case updateCenterLineMsg:
		// One line of a running command's output, kept as the tail the screen draws.
		// A line that arrives with no run open is a straggler from a command that has
		// already been reported on: the result keeps its own bounded copy, so the line
		// is dropped rather than appended to the log of whatever runs next.
		if m.UpdateCenter.Lines == nil {
			return m, nil
		}
		m.updateCenterAppendLine(string(msg))
		return m, updateCenterWaitForLine(m.UpdateCenter.Lines)

	case updateCenterLinesClosedMsg:
		// The command stopped writing. Nothing is left to read: the result itself is
		// what carries the outcome, and this message only ends the read loop.
		return m, nil

	case wslResourceLoadedMsg:
		// The first read fills the state once: a later answer arriving after the
		// screen has been written would roll the draft back to what the file held.
		// The refresh a write asked for is the one exception -- it is what replaces
		// the values the write just changed -- so it is accepted too, and the state
		// it brings clears the refreshing mark by not carrying one.
		if !m.WSLState.Resolved || m.WSLState.Refreshing {
			// The WSL row is inserted above the section's read-only rows when the
			// answer says there is a .wslconfig to edit, and the read is armed when
			// the section opens, so the row arrives under a cursor that was placed
			// on the rows already there. The label is kept and re-found.
			held := m.selectedOption()
			m.WSLState = msg.state
			m.holdCursorOn(held)
		}
		return m, nil

	case wslGuardRepairedMsg:
		// The state is replaced by what the host holds after the repair, never by
		// what the install reported: a guard whose unit exited non-zero leaves the
		// screen saying exactly which protection is still missing.
		m.WSLState.Guard = msg.state
		if msg.err != nil {
			m.WSLNotice = fmt.Sprintf("The WSL memory guard could not be installed: %v", msg.err)
			return m, nil
		}
		m.WSLNotice = wslGuardRepairNotice(msg.state)
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

	case wslTerminalOfferMsg:
		// The offer is read-only, so the first thing it does is reach the screen.
		// With nothing to offer the reason is logged and the step carries straight
		// on to its script, so a machine where Windows Terminal is not reachable
		// still gets the in-distribution half of the step.
		if !msg.offer.Available {
			m = m.appendInstallLog(msg.lines...)
			m.WSLTerminalOfferDone = true
			return m, m.runNextStep()
		}
		offer := msg.offer
		m.WSLTerminalOffer = &offer
		m.WSLTerminalOfferLines = msg.lines
		m.WSLTerminalOfferProfileDir = msg.profileDir
		m.WSLTerminalOfferResolved = msg.resolved
		m.Cursor = 0
		return m, nil

	case wslTerminalOfferAppliedMsg:
		// The answer is a fact about the run, so it is logged where the step's own
		// notices go, and the run then continues into the script that needs sudo.
		// The offer cannot be answered twice: it is cleared before the script's
		// own messages can arrive.
		// The notice goes first and the shortfall after it: the notice is the report read
		// from the file, and the shortfall is the verdict on it. Logging the verdict alone
		// would drop what the file actually holds, which is the half a person needs.
		if msg.notice != "" {
			m = m.appendInstallLog(msg.notice)
		}
		if msg.err != nil {
			m = m.appendInstallLog(msg.err.Error())
		}
		m.WSLTerminalOffer = nil
		m.WSLTerminalOfferLines = nil
		m.WSLTerminalOfferProfileDir = ""
		m.WSLTerminalOfferResolved = false
		m.WSLTerminalOfferDone = true
		return m, m.runNextStep()

	case wslBenchmarkDoneMsg:
		// The report replaces the previous one whole, so the screen never shows a
		// mixture of two runs; a run that measured nothing says so through the same
		// fields as a run that measured everything.
		m.WSLAdvisor.Benchmark.Running = false
		m.WSLAdvisor.Benchmark.cancel = nil
		m.WSLAdvisor.Benchmark.Ran = true
		m.WSLAdvisor.Benchmark.Report = msg.report
		m.WSLAdvisor.Benchmark.Notice = wslBenchmarkPanelValue(m.WSLAdvisor.Benchmark)
		return m, nil

	case wslMigrationPlannedMsg:
		// The preview is only accepted for the pair the screen is still on: a plan
		// for a source the user has already left behind must not appear as if it
		// described the current choice.
		migration := &m.WSLAdvisor.Migration
		if migration.Phase != wslMigrateReadingSource {
			return m, nil
		}
		if msg.err != "" {
			migration.PlanErr = msg.err
		} else {
			migration.Plan = msg.plan
			migration.PlanErr = ""
		}
		migration.Phase = wslMigratePreview
		m.Cursor = 0
		return m, nil

	case wslMigrationProgressMsg:
		m.WSLAdvisor.Migration.Progress = msg.progress
		m.WSLAdvisor.Migration.ProgressSeen = true
		return m, nil

	case wslMigrationCopiedMsg:
		// The outcome lands even when it arrived after the user stepped back, and
		// the run's own report is what the screen shows, including a cancelled run
		// and a partial copy: neither is turned into a success here.
		migration := &m.WSLAdvisor.Migration
		migration.Running = false
		migration.Cancel = nil
		migration.Report = msg.report
		migration.ReportErr = msg.err
		if msg.report.Destination != "" {
			migration.Plan.Destination = msg.report.Destination
		}
		migration.Phase = wslMigrateDone
		m.Cursor = 0
		migration.Notice = ""
		return m, nil

	case wslAdvisorLoadedMsg:
		// The reading replaces the old one whole, so the report never shows a
		// mixture of two readings: a half-updated table would let a reader compare
		// a mount table from one moment with a memory figure from another.
		msg.state.Notice = m.WSLAdvisor.Notice
		m.WSLAdvisor = msg.state
		// The cursor is clamped rather than reset: a re-read while the reader is on
		// a row should leave them on that row when the list is the same length, and
		// never past the end when it is not.
		if rows := m.wslAdvisorRows(); m.Cursor >= len(rows) {
			m.Cursor = 0
		}
		return m, nil

	case shellAuditMeasuredMsg:
		// The measurement is not an install step and cannot fail as one: every run
		// it could not complete is already part of the state it brings back, with
		// its reason. The state replaces the old one whole, so the screen never
		// shows a mixture of two measurements.
		m.ShellAudit = msg.state
		return m, nil

	case diskCleanScanMsg:
		// A scan is not an install step, so its answer lands on this screen rather
		// than on a failed one. It replaces the findings whole -- the selection goes
		// with them, so nothing from an older scan can be confirmed against a
		// number that has since changed -- and it says plainly when the answer is
		// only what the walk reached before it was cancelled or bounded.
		m.DiskClean.Scanning = false
		m.DiskClean.Scanned = true
		m.DiskClean.Cancel = nil
		m.DiskClean.Progress = ""
		m.DiskClean.Items = msg.items
		m.DiskClean.Selection = diskclean.Selection{}
		m.DiskClean.Planned = false
		m.DiskClean.Plan = diskclean.Plan{}
		// The cursor lands on the first finding rather than on the scan row: the
		// card of the finding under the cursor is where the reason it was found and
		// the paths it covers are read, and a scan that left the cursor above the
		// list would hide the evidence until the first arrow key.
		m.Cursor = m.diskCleanFirstItemCursor()
		if msg.cancelled {
			m.DiskClean.Notice = diskCleanScanCancelledNotice(msg.items)
		} else {
			m.DiskClean.Notice = diskCleanScanNotice(msg.items)
		}
		return m, nil

	case diskCleanProgressMsg:
		// Progress is a label, not a measurement: it names the store being worked
		// on so a long scan does not look like a frozen screen.
		m.DiskClean.Progress = msg.label
		return m, nil

	case diskCleanCleanedMsg:
		m.DiskClean.CleanReport = msg.report
		m.DiskClean.Phase = diskCleanResults
		m.DiskClean.Cancel = nil
		m.DiskClean.Progress = ""
		m.DiskClean.Notice = diskCleanResultNotice(msg.report)
		m.DiskClean.Planned = false
		m.Cursor = 0
		return m, nil

	case shellAuditEventMsg:
		// A start or the result itself. The result is folded in by the one
		// projection the engine and the screen share, and it is then stored when
		// the user asked for that; everything else is progress, and the next read
		// is armed from the same session so the measurement keeps reporting itself.
		if msg.event.Done {
			m.ShellAudit = m.ShellAudit.withShellAuditResult(msg.event)
			m.ShellAuditSession = nil
			if m.ShellAudit.History.Enabled {
				return m, m.saveShellAuditHistoryCmd()
			}
			return m, nil
		}
		m.ShellAudit.Progress = msg.event.Progress
		if m.ShellAuditSession == nil {
			// The session was cancelled and cleared while this event was in flight.
			// The reader is not re-armed: the channel is buffered for the whole
			// measurement, so nothing is left waiting on this end of it.
			return m, nil
		}
		return m, awaitShellAuditEvent(m.ShellAuditSession)

	case shellAuditClosedMsg:
		// The events ended without a result, which is not a measurement the screen
		// can draw. It stops claiming to be measuring and says which shell it was
		// waiting on, so the row can be pressed again.
		m.ShellAudit.Measuring = false
		m.ShellAudit.Progress = shellAuditProgress{}
		m.ShellAuditSession = nil
		return m, nil

	case shellAuditHistorySavedMsg:
		if msg.err != nil {
			m.ShellAudit.HistoryError = fmt.Sprintf("the measurements could not be stored (%v), so this run "+
				"keeps them only until it closes.", msg.err)
			return m, nil
		}
		m.ShellAudit.HistoryError = ""
		return m, nil

	case portsLoadedMsg:
		// One reading has landed. The cursor is restored by the identity captured
		// when the reading started, and the notice reports what changed without
		// ever moving the cursor itself.
		wasLoaded := m.Ports.Loaded
		m.Ports.Loading = false
		m.Ports.Loaded = true
		m.Ports.LoadErr = ""
		m.applyPortSnapshot(msg.snap, wasLoaded)
		return m, nil

	case portsStopDoneMsg:
		// The account of one attempt replaces any older one whole. A terminate that
		// left the process running turns the forced signal into its own visible
		// action; every other outcome settles the screen. The manager is re-read
		// afterwards so the ports column reflects what the verification actually
		// found rather than what was on screen before the signal.
		result := msg.result
		m.PortStop.Result = &result
		m.PortStop.Running = false
		m.PortStop.Forced = result.Signal == portscan.SignalKill
		m.Ports.SelectedKey = m.portSelectedKey()
		return m, m.portScanCmd()

	case portsTickMsg:
		// The periodic refresh is the manager's own while it is drawn, and never
		// fires while the filter is being typed, while the stop screen is in front,
		// or while a reading is already in flight. It re-arms itself for as long as
		// the refresh is on, so a screen that is left and returned to keeps the
		// choice. The cursor is captured before the reading, exactly as a manual
		// refresh does, so a tick can never move it.
		if m.Screen != ScreenPortManager || !m.Ports.Auto {
			return m, nil
		}
		if m.Ports.Filtering || m.Ports.Loading {
			return m, portsTickCmd()
		}
		m.Ports.SelectedKey = m.portSelectedKey()
		m.Ports.Loading = true
		return m, tea.Batch(m.portScanCmd(), portsTickCmd())

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
			// not, one line per tool, in the rows the pending line already held. A
			// partial run's notice heads that list: the files it left alone must not
			// be dropped in favour of a plain "applied".
			paragraphs := themeReloadResultParagraphs(msg.reload)
			if themeNoticeIsPartial(msg.notice) {
				paragraphs = append([]string{msg.notice}, paragraphs...)
			}
			m.finishThemeActivity(paragraphs...)
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

	case sudoNeedsPasswordMsg:
		// sudo has no credential it can use without a password. The installer asks
		// for it on its own screen, so nothing is handed over and the terminal stays
		// where it is. The field is a []byte of fixed capacity: see AdminPassword.
		m.AdminPassword = make([]byte, 0, adminPasswordCapacity)
		m.AdminPasswordErr = ""
		m.Screen = ScreenAdminPassword
		m.LogLines = append(m.LogLines, "Administrator access: type the password on this screen; the installer does not leave it to ask.")
		return m, nil

	case sudoPrimedMsg:
		// An attempt made from the installer's own screen. sudo has already read
		// whatever the field held and those bytes are already cleared, so the field
		// starts empty for a retry. A refusal stays on the screen rather than
		// becoming a loop: only the user starts another attempt.
		if m.Screen == ScreenAdminPassword {
			m.AdminPassword = m.AdminPassword[:0]
			if msg.err != nil {
				m.AdminPasswordErr = msg.err.Error()
				m.LogLines = append(m.LogLines, "⚠️  "+msg.err.Error())
				return m, nil
			}
			m.AdminPasswordErr = ""
			m.Screen = ScreenInstalling
			m.SudoPrimed = true
			m.LogLines = append(m.LogLines, "✓ Administrator credential cached for this run.")
			return m, tea.Batch(m.runNextStep(), sudoKeepAliveTickCmd())
		}

		// The one credential request has returned. Whether it succeeded or not,
		// the run continues: a step that needs root without the credential fails
		// visibly through sudo -n instead of prompting where the user cannot see
		// it. A success keeps the run's credential refreshed while it works.
		if msg.err != nil {
			m.SudoPrimed = false
			m.LogLines = append(m.LogLines, fmt.Sprintf("⚠️  Administrator credential was not cached (%v). Steps that need root will report it.", msg.err))
			return m, m.runNextStep()
		}
		m.SudoPrimed = true
		m.LogLines = append(m.LogLines, "✓ Administrator credential cached for this run.")
		return m, tea.Batch(m.runNextStep(), sudoKeepAliveTickCmd())

	case sudoKeepAliveMsg:
		if !m.SudoPrimed {
			return m, nil
		}
		return m, sudoRefreshCmd()

	case sudoRefreshedMsg:
		if msg.err != nil {
			m.SudoPrimed = false
			m.LogLines = append(m.LogLines, "⚠️  Could not refresh the administrator credential; a step that needs root will report it.")
			return m, nil
		}
		return m, sudoKeepAliveTickCmd()

	case execFinishedMsg:
		// A captured step finished. The script ran through the same output
		// executor every other step uses, so the installer never left the screen;
		// a failure still takes the run to the error screen with the step's name.
		for i := range m.Steps {
			if m.Steps[i].ID == msg.stepID {
				if msg.err != nil {
					msg.err = describeInteractiveStepError(msg.stepID, msg.err)
					m.Steps[i].Status = StatusFailed
					m.Steps[i].Error = msg.err
					m.Screen = ScreenError
					// Include step name in error message for clarity
					m.ErrorMsg = fmt.Sprintf("Step '%s' failed:\n%s", m.Steps[i].Name, msg.err.Error())
					// The steps before this one have already written configuration
					// files, and this run stops here: the record is what names them,
					// exactly as it is for a step that fails through its own status.
					m.recordRun(runStatusFailed, []string{m.Steps[i].Name})
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

	// The port manager's filter is a text field: while it is open it owns every
	// key, including space, escape and backspace, which would otherwise be taken
	// by the leader key or the global back dispatch before the field ever saw
	// them. It is taken before all of them so a filter really is text.
	if m.Screen == ScreenPortManager && m.Ports.Filtering {
		return m.handlePortFilterKeys(key)
	}

	// The interactive WSL step pauses the run on the Windows Terminal offer. Its
	// answer keys are taken here, before the navigation keys, so Esc is the refusal
	// the offer names rather than a way out of a screen the run still owns. A key
	// that is not an answer falls through, so the leader key, details and the rest
	// keep working while the offer waits.
	if m.Screen == ScreenInstalling && m.WSLTerminalOffer != nil {
		if next, cmd, handled := m.handleWSLTerminalOfferKeys(key); handled {
			return next, cmd
		}
	}

	// Leader key mode: <space> activates, next key executes command
	// Commands: <space>q = quit
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
		default:
			// Unknown leader command, ignore
			return m, nil
		}
	}

	// The installing screen's details run on their own letter: d toggles the log
	// box where it stands, in one press. It is read after the leader key, so Space
	// no longer carries the toggle -- Space toggles a switch, and this screen has
	// none -- and before the screen dispatch, so one d really does it.
	if key == "d" && m.Screen == ScreenInstalling {
		m.ShowDetails = !m.ShowDetails
		return m, nil
	}

	// The network screen owns every key while its destination field is open: a space
	// is not accepted by the field, backspace deletes a character and escape closes
	// the field rather than leaving the screen, exactly as the password screen keeps
	// its own keys. The field is dispatched here, before those global meanings,
	// because a field that loses a character to a navigation key is not a field.
	if m.Screen == ScreenNetworkDiagnostics && m.Network.InputActive {
		return m.updateNetworkInput(msg)
	}

	// The password screen owns every key it uses. It is reached in the middle of an
	// install, where the global meanings of these keys would take a character away
	// from the field: space would open the leader key instead of being typed,
	// backspace would mean "back" instead of deleting, and escape would leave the
	// screen rather than offer the way out it is meant to offer. Taking the
	// dispatch here, before any of them, is what keeps the field a field.
	if m.Screen == ScreenAdminPassword {
		return m.updateAdminPassword(msg)
	}

	// The SSH manager's screens own every key they use, for the same reason the
	// password screen does: the form, the generation form and the test's alias
	// field are text fields, and the global meanings of space and backspace would
	// take characters away from them. The dispatch is taken here, before the
	// leader key and before Escape, and each screen's own handler decides what its
	// Escape means -- cancelling a plan or a sub-state before it leaves the screen.
	if isSSHManagerScreen(m.Screen) {
		return m.handleKeyPressSSH(msg)
	}

	// <space> activates leader mode EXCEPT in screens that need space for input
	// (Trainer screens use space in commands) or where it is a row's switch. The
	// welcome, complete and error screens used to spend it on continue and quit;
	// they have no switch, so it no longer does anything there but arm the leader
	// key, and the letters the footer names are what actually act.
	if key == " " {
		// Screens where space should NOT activate leader mode
		switch {
		case isTrainerScreen(m.Screen):
			// Trainer screens own space: the exercise screens append it to the
			// answer input, and the menu and result screens treat it like enter.
			// Pass through to the screen-specific handlers below.
		case m.Screen == ScreenSetup:
			// The setup screen owns space: it is the switch key for the font, the
			// editor and the backup, and turning a switch on or off is the whole
			// point of the row. Passing it to the leader key would leave every
			// switch in the interface advertised and unreachable.
		case m.Screen == ScreenDiskCleaner:
			// The disk cleaner owns space too: it is the switch key on every
			// finding, and marking a finding for cleanup is the whole point of the
			// row. Passing it to the leader key would leave every checkbox on the
			// screen advertised and unreachable.
		case m.Screen == ScreenUpdateCenter:
			// The update center owns space for the same reason the setup screen's
			// switches do: on that list space selects or drops the entry the cursor is
			// on, which is the whole point of a row. Leaving it to the leader key would
			// advertise a key the screen never receives, and the first press would
			// silently arm the leader so the *next* key was read as a leader command.
		case isSSHManagerScreen(m.Screen):
			// The SSH manager's screens are dispatched above, before this switch:
			// the space belongs to the field or the row it is pressed on.
		case m.Screen == ScreenPortManager:
			// The port manager owns space too: it expands the selected row, which is
			// a read and never a write, and the footer names the key.
		default:
			// All other screens: activate leader mode
			m.LeaderMode = true
			return m, nil
		}
	}

	// The shell startup screen is the one screen where Escape means two things: the
	// way back when nothing is running, and the stop while the runs are still
	// coming. The stop has to be taken here, before the global Escape below,
	// because that dispatch leaves the screen -- and a screen the user has left
	// would go on starting their shell with nothing on screen saying so.
	if key == "esc" && m.Screen == ScreenShellAudit && m.ShellAudit.Measuring {
		if m.ShellAuditSession != nil {
			m.ShellAuditSession.cancel()
		}
		return m, nil
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

	// The single-tool info pages are read-only, and their footer names [q] quit, so
	// q quits here instead of arming the leader key. A list with a cursor is not one
	// of these pages: the same screens read it only while a tool is being viewed.
	if key == "q" && m.isSingleToolInfoScreen() {
		m.Quitting = true
		return m, tea.Quit
	}

	// Screen-specific keys
	switch m.Screen {
	case ScreenWelcome:
		switch key {
		case "s":
			m.Screen = ScreenMainMenu
			m.Cursor = 0
		case "q":
			m.Quitting = true
			return m, tea.Quit
		}

	case ScreenMainMenu:
		return m.handleMainMenuKeys(key)

	case ScreenInstallReview:
		return m.handleInstallReviewKeys(key)

	case ScreenSetup:
		return m.handleSetupKeys(key)

	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		return m.handleSelectionKeys(key)

	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim, ScreenLearnPrompts:
		return m.handleLearnMenuKeys(key)

	case ScreenLearnTools:
		return m.handleLearnHubKeys(key)

	case ScreenLearnTooling:
		return m.handleReferenceTableKeys(key, len(shellToolingRows()))

	case ScreenLearnPlugins:
		return m.handleReferenceTableKeys(key, len(pluginSetRows()))

	case ScreenLearnToolset:
		return m.handleReferenceTableKeys(key, len(toolsetDirectives()))

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

	case ScreenKeymapsKitty:
		return m.handleKittyKeymapsMenuKeys(key)

	case ScreenKeymapsKittyCat:
		return m.handleKittyKeymapCategoryKeys(key)

	case ScreenKeymapsWezTerm:
		return m.handleReferenceTableKeys(key, len(weztermConfigTable().Rows))

	case ScreenKeymapsAlacritty:
		return m.handleReferenceTableKeys(key, len(alacrittyConfigTable().Rows))

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

	case ScreenDiskCleaner:
		return m.handleDiskCleanerKeys(key)

	case ScreenUpdateCenter:
		return m.handleUpdateCenterKeys(key)

	case ScreenDoctor:
		// The doctor draws its rows, and every one of them is a press the screen
		// advertises: a, c, r, t, w, and Enter on a finding that offers a fix. Without
		// this case the screen rendered and answered nothing, which reads as a freeze
		// rather than as a screen that refuses every key.
		return m.handleDoctorKeys(key)

	case ScreenDoctorFix:
		// The confirmation is one level in from the doctor and owns the same kind of
		// press: the letter that applies the fix, and Escape, which steps back to the
		// list it came from.
		return m.handleDoctorFixKeys(key)

	case ScreenInstallVerify:
		return m.handleInstallVerifyKeys(key)

	case ScreenNetworkDiagnostics:
		return m.handleNetworkKeys(key)

	case ScreenWSLAdvisor:
		return m.handleWSLAdvisorKeys(key)

	case ScreenWSLBenchmark:
		return m.handleWSLBenchmarkKeys(key)

	case ScreenWSLMigration:
		return m.handleWSLMigrationKeys(key)

	case ScreenSSHManager:
		return m.handleSSHManagerKeys(key)

	case ScreenSSHHost:
		return m.handleSSHHostKeys(key)

	case ScreenSSHForm:
		return m.handleSSHFormKeys(key)

	case ScreenSSHKeys:
		return m.handleSSHKeysKeys(key)

	case ScreenSSHAgent:
		return m.handleSSHAgentKeys(key)

	case ScreenSSHDiagnostics:
		return m.handleSSHDiagnosticsKeys(key)

	case ScreenSSHTest:
		return m.handleSSHTestKeys(key)

	case ScreenPortManager:
		return m.handlePortManagerKeys(key)

	case ScreenPortStop:
		return m.handlePortStopKeys(key)

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
		case "q":
			m.Quitting = true
			return m, tea.Quit
		}

	case ScreenError:
		switch key {
		case "q":
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
	case ScreenSetup:
		// The setup screen is where the wizard's questions start, so its Escape
		// leaves the flow for the main menu and forgets the answers, exactly as
		// the old first question's Escape did.
		m.leaveSetup()
	case ScreenInstallReview:
		// The plan's Escape is the way back its footer names, and the screen it
		// was opened from is the setup screen now. It reaches the review's own
		// handler so the two ways of pressing Escape cannot drift apart.
		return m.handleInstallReviewKeys("esc")
	case ScreenGhosttyWarning:
		// Go back to terminal selection
		m.Screen = ScreenTerminalSelect
		m.Cursor = 0
	case ScreenBackupConfirm:
		// The backup decision is a row on the setup screen and this screen is the
		// detour that lists the configurations found, so its Escape is every other
		// detour's: back to the setup screen with the answers untouched. It used to
		// go back to the Neovim question, which was the question before it in the
		// chain this front removed.
		m.returnToSetup()
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
	case ScreenKeymapsKittyCat:
		m.Screen = ScreenKeymapsKitty
		m.KittyKeymapScroll = 0
	case ScreenLazyVimTopic:
		m.Screen = ScreenLearnLazyVim
		m.LazyVimScroll = 0
	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim, ScreenLearnTools, ScreenLearnFont, ScreenLearnPrompts, ScreenLearnTooling, ScreenLearnPlugins, ScreenLearnToolset:
		m.Screen = m.PrevScreen
		m.Cursor = 0
		m.ViewingTool = ""
		m.ReferenceTableScroll = 0
	case ScreenKeymaps:
		m.Screen = ScreenKeymapsMenu
		m.Cursor = 0
	case ScreenKeymapsTmux, ScreenKeymapsZellij, ScreenKeymapsGhostty, ScreenKeymapsHerdr,
		ScreenKeymapsKitty, ScreenKeymapsWezTerm, ScreenKeymapsAlacritty:
		m.Screen = ScreenKeymapsMenu
		m.Cursor = 0
		m.ReferenceTableScroll = 0
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
	case ScreenWSLAdvisor:
		// The report is one level in from the section. Leaving it clears the
		// notice of the last action, exactly as the other utility screens do, so a
		// later visit does not open on a stale result; the reading itself stays,
		// because it is what the panel shows.
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.WSLAdvisor.Notice = ""
	case ScreenWSLBenchmark:
		// The measurement screen is one level further in. Escape stops a run in
		// flight -- a measurement holds the disk while it runs -- and with nothing
		// running it is the way back. The back row still opens with Enter and leaves
		// a run to land its report on the model.
		if m.WSLAdvisor.Benchmark.Running {
			if m.WSLAdvisor.Benchmark.cancel != nil {
				m.WSLAdvisor.Benchmark.cancel()
			}
			m.WSLAdvisor.Benchmark.Notice = "Stopping the measurement…"
			return m, nil
		}
		m.Screen = ScreenWSLAdvisor
		m.Cursor = 0
		m.WSLAdvisor.Benchmark.Notice = ""
	case ScreenWSLMigration:
		// The copy screen walks its own phases in reverse, and stops a copy in
		// flight: a copy is the one action here that keeps costing something while
		// the user is elsewhere.
		return m.wslMigrationEscape()
	case ScreenShellAudit:
		// The shell startup screen is one level in as well, and with nothing running
		// this is the way back. Escape while a measurement runs never reaches this
		// dispatch: it is the stop, taken above, so the starts the user asked for
		// are the starts that are running.
		m.Screen = ScreenUtilities
		m.Cursor = 0
	case ScreenUpdateCenter:
		// The update center owns its own Escape, because what it means depends on the
		// state the screen is in: the confirmation cancels and returns to the list, a
		// running update is refused a way out (hiding the screen mid-transaction would
		// hide the only place that can stop it), and the list itself steps back to the
		// section it was opened from. Routing it to the screen's own handler keeps
		// those three in one place instead of splitting them between two files.
		return m.handleUpdateCenterKeys("esc")
	case ScreenInstallVerify:
		// The verification screen is one level in as well, and read-only: leaving
		// it cancels nothing, because nothing it does is a change.
		m.Screen = ScreenUtilities
		m.Cursor = 0
	case ScreenNetworkDiagnostics:
		// The diagnosis is one level in, and it is the one screen of the section
		// whose Esc has three meanings: it stops a run that is going, it closes an
		// open preview, and only then does it leave. That order lives in one place,
		// leaveNetworkScreen, so the key's meaning cannot drift between them.
		return m.leaveNetworkScreen()

	case ScreenSSHManager:
		// The manager is one level in from the utilities section. A test that is
		// still running is cancelled on the way out: an answer nobody is waiting for
		// must not land on a screen the user has left.
		m.cancelSSHTest("")
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.SSH.Notice = ""
	case ScreenSSHHost:
		m.Screen = ScreenSSHManager
		m.Cursor = m.SSH.HostIndex
		m.SSH.Notice = ""
	case ScreenSSHForm:
		// The form's Escape is the cancel its footer names: a plan is discarded
		// first, and only a second Escape leaves the form.
		if m.SSH.Plan != nil {
			m.SSH.Plan = nil
			m.SSH.Notice = "Nothing was written."
			return m, nil
		}
		m = m.leaveSSHForm()
	case ScreenSSHKeys:
		if m.SSH.KeysMode != "" {
			m.SSH.KeysMode = ""
			m.SSH.AddPassphrase = ""
			m.Cursor = 0
			m.SSH.Notice = ""
			return m, nil
		}
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
	case ScreenSSHAgent:
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
		m.SSH.AgentConfirm = false
	case ScreenSSHDiagnostics:
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
	case ScreenSSHTest:
		m = m.leaveSSHTest()
	case ScreenPortManager:
		// The filter field owns Escape while it is open, so this path is reached
		// only when it is not: the manager then steps back to the section that
		// opened it. The reading is not cancelled -- a reading already in flight
		// still lands on the model -- and the notice is cleared so a later visit
		// does not open on a stale line.
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.Ports.Notice = ""
		m.Ports.Filtering = false
	case ScreenPortStop:
		// The confirmation is one level in from the manager, so Escape steps back
		// there rather than all the way to the section.
		m.Screen = ScreenPortManager
		m.Cursor = 0
	case ScreenDiskCleaner:
		// The disk cleaner owns this key: escape is the way out of a preview, it
		// stops a scan or a cleanup in flight, and only when there is nothing to
		// step back from does it leave for the section. Its own handler says which
		// of those it is, so this case and its footer cannot drift apart.
		return m.diskCleanerEscape()
	case ScreenDoctor:
		// The doctor owns this key for the same reason the disk cleaner does: Escape
		// stops a run in flight -- the checks already handed to the runtime run to
		// their own bound, and their answers are recorded as unknown with the reason
		// -- and with nothing running it is the way back to the section.
		return m.handleDoctorKeys("esc")
	case ScreenDoctorFix:
		// The confirmation is one level in from the doctor, so Escape steps back
		// there rather than all the way to the section.
		return m.handleDoctorFixKeys("esc")
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

// enterSetup is where the wizard now starts. The shell cursor rule that used to
// live in a separate enterShellSelect is applied by enterSetup (setup_rows.go)
// when it records the detected shell.

func (m Model) startTrainer() (tea.Model, tea.Cmd) {
	// The reporting load is the one the trainer's own entry point uses: a profile that
	// could not be read is moved aside and the player is told where it went, instead of
	// being handed a silent zero that the next save would write over.
	stats, problem := trainer.LoadStatsReport()
	if stats == nil {
		stats = trainer.NewUserStats()
	}
	m.TrainerStats = stats
	m.TrainerGameState = nil
	m.TrainerCursor = 0
	m.TrainerInput = ""
	m.TrainerMessage = problem
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
		// Not cleared, for the reason the row's own case states: the outcome of a
		// switch that failed while the user was away has to survive the way back in.
		return m, tea.Batch(m.dotfilesThemesCmdIfNeeded(), m.wslResourceStateCmdIfNeeded())
	case "q":
		// Quit is its own key. The Exit row is still chosen with Enter, and the
		// footer names q so the leaving key is readable without opening the menu.
		m.Quitting = true
		return m, tea.Quit
	case "enter":
		selected := options[m.Cursor]
		switch {
		case strings.Contains(selected, "Start Installation"):
			// The wizard opens on the one screen where every decision is a row,
			// which records the detected platform and applies the answers that
			// platform forces before the first frame is drawn. The old first
			// question's cursor rule lives inside enterSetup now.
			m.enterSetup()
		case strings.Contains(selected, "Learn About Tools"):
			// The row opens the hub rather than the terminal list: the hub is the one
			// route the main menu has to the shell, multiplexer and Neovim pages.
			m.Screen = ScreenLearnTools
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
		case selected == updateInstallerLabel(m.updateChannel()):
			// The row is the button when this run may replace its own file:
			// pressing it runs the same verified swap `--self-update` runs, off the
			// update loop, and the result appears on this screen in the notice slot
			// above the menu. When the file is not the installer's to replace, the
			// row reports instead: it opens the update screen, where the reason is
			// stated in full and a fresh check can be asked for. It never attempts
			// the swap the reason says is not the installer's to make.
			if !m.UpdateInstallable() {
				m.Screen = ScreenUpdate
				m.Cursor = 0
				return m, nil
			}
			if m.UpdateCheck.InFlight {
				return m, nil
			}
			m.UpdateCheck.InFlight = true
			m.UpdateCheck.Notice = "Downloading " + m.UpdateCheck.Latest + "…"
			return m, updateApplyCmd(m.updateChannel(), m.UpdateCheck.Latest, m.UpdateTarget)
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
			// The notice is deliberately NOT cleared here. A switch is bounded by
			// themeCommandTimeout and its failure is the slow outcome, so a user who
			// steps away before it lands would have the one result they must see
			// erased by the act of coming back to read it. Leaving the section still
			// clears it, which is where the user has already read it.
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
	case "enter":
		// Enter opens a screen and nothing else. A row that acts where it stands
		// carries its own letter on the row, so Enter on one of them is a press with
		// no answer on purpose: it is what keeps one key from meaning two things.
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := strings.TrimPrefix(options[m.Cursor], utilitiesScreenMark)
		switch {
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
		case selected == utilitiesVerifyRow:
			// The verification is one level in as well, and it is the check that runs
			// off the update loop: the record and the paths it names are stated here,
			// and a keypress must not wait on them.
			m.Screen = ScreenInstallVerify
			m.Cursor = 0
			return m, m.installVerifyCmdIfNeeded()
		case selected == utilitiesTerminalRow:
			// The capability report is one level in as well, and it is read-only.
			// The probe is armed here -- when the user opens the screen, never at
			// startup -- and it runs off the update loop behind a bounded query, so
			// a terminal that never answers cannot freeze the interface.
			m.Screen = ScreenTerminalCapabilities
			m.Cursor = 0
			return m, m.terminalCapabilityCmdIfNeeded()
		case selected == utilitiesDiskCleanerRow:
			// The disk cleaner is one level in, for the same reason the other
			// utilities are: the section is a list of jobs. Nothing is scanned on the
			// way in -- the screen's own scan row starts that -- so opening the screen
			// never puts a walk of the machine between the press and the screen.
			m.Screen = ScreenDiskCleaner
			m.Cursor = 0
			m.DiskClean.Notice = ""
			return m, nil
		case selected == utilitiesShellAuditRow:
			// The shell's startup is one level in too. Nothing is measured on the way
			// in: starting the user's shell five times is a choice, and it is made on
			// that screen's own row.
			m.Screen = ScreenShellAudit
			m.Cursor = 0
		case selected == utilitiesUpdatesRow:
			// The update center is one level in, and it is the one row of this section
			// that asks the machine's package managers anything. The read starts here
			// -- when the user opens the screen, never at startup -- and it runs off the
			// update loop one manager at a time, so the rows appear as the machine
			// answers instead of the frame waiting on the slowest manager.
			//
			// The command is built before the model is returned: the call records which
			// manager is being read, and a `return m, m.startUpdateCenterScan()` would
			// copy the model before that call wrote to it, so the first answer would
			// arrive for a pass the screen does not believe is running.
			m.Screen = ScreenUpdateCenter
			m.Cursor = 0
			scan := m.startUpdateCenterScan()
			return m, scan
		case selected == utilitiesWSLAdvisorRow:
			// The advisor is one level in like the doctor and the network
			// diagnosis, and it reads nothing on the way in either: the reading is
			// started from the screen's own row, so opening it cannot leave the
			// user watching a walk they did not ask for. Before this branch
			// existed the row was drawn and handled by nothing, so Enter on it did
			// nothing at all.
			m.Screen = ScreenWSLAdvisor
			m.Cursor = 0
			m.WSLAdvisor.Notice = ""
		case selected == utilitiesDoctorRow:
			// The doctor is one level in as well, and nothing is checked on the way
			// in: a scan reads the whole machine, so it is started from the screen's
			// own row rather than by opening it.
			m.Screen = ScreenDoctor
			m.Cursor = 0
			m.Doctor.Notice = ""
		case selected == utilitiesNetworkRow:
			// The diagnosis is one level in for the same reason, and it dials nothing
			// on the way in either: the checks start from that screen's own row, and the
			// host the overview contacts is named there.
			m.Screen = ScreenNetworkDiagnostics
			m.Cursor = 0
			m.Network.Notice = ""

		case selected == utilitiesSSHRow:
			// The SSH manager is one level in as well. The client and the
			// configuration are read on the way in, because which hosts exist is what
			// the screen is for and a screen that opened empty and filled a moment
			// later would make the user press twice. The read runs as a command, so
			// probing the client cannot block the update loop.
			m.Screen = ScreenSSHManager
			m.Cursor = 0
			m.SSH.HostIndex = 0
			m.SSH.KeysMode = ""
			m.SSH.Notice = ""
			if m.SSH.Manager == nil {
				return m, sshLoadCmd()
			}
			return m, sshConfigCmd(m.SSH.Manager)
		case selected == utilitiesPortRow:
			// The port manager is one level in as well. The first reading starts on the
			// way in, off the update loop, so the screen says it is reading instead of
			// opening empty; a second reading is a keypress away.
			m.Screen = ScreenPortManager
			m.Cursor = 0
			m.Ports.Cursor = 0
			m.Ports.Notice = ""
			m.Ports.Detail = false
			m.Ports.SelectedKey = ""
			m.Ports.Loading = true
			m.Ports.LoadErr = ""
			return m, m.portScanCmd()
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenMainMenu
			m.Cursor = 0
			m.ThemeNotice = ""
		}
	case "d":
		// The dark row is offered only where a desktop switch was detected, so the
		// letter is honoured only then: a key that switched a theme through a tool
		// nobody found would be the section acting without a row to act on.
		if !m.ThemeSwitchFound {
			return m, nil
		}
		// The line the result will use says the switch has started, so a tool that
		// takes a moment to write is not a keypress that seemed to do nothing.
		m.ThemeNotice = "Switching the desktop's theme to dark…"
		return m, applyThemeCmd(m.ThemeSwitch, true)
	case "l":
		if !m.ThemeSwitchFound {
			return m, nil
		}
		m.ThemeNotice = "Switching the desktop's theme to light…"
		return m, applyThemeCmd(m.ThemeSwitch, false)
	case "u":
		// The undo row exists only while the record this installer wrote does.
		if !m.themeUndoAvailable() {
			return m, nil
		}
		m.ThemeNotice = "Putting the previous desktop theme back…"
		return m, undoThemeCmd(m.ThemeSwitch, *m.ThemeRecord)
	}

	return m, nil
}

// handleWSLTerminalOfferKeys answers the interactive WSL step's Windows Terminal
// offer. y is the explicit yes and n is the refusal, with Esc standing in for it
// because that is the way back everywhere else; the two both go through
// applyWSLTerminalOfferCmd, so the key handler never touches the file and the
// yes has to be spelled. handled is false for every other key, which lets the
// installing screen keep its own keys while the offer waits.
func (m Model) handleWSLTerminalOfferKeys(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "y", "Y":
		return m, m.applyWSLTerminalOfferCmd(true), true
	case "n", "N", "esc":
		return m, m.applyWSLTerminalOfferCmd(false), true
	}
	return m, nil, false
}

// handleWSLResourceKeys drives the WSL resource screen. It only ever changes the
// cursor, the draft or the notice: the write is a command, so a slow filesystem
// cannot block the update loop, and the dry-run gate it runs behind is the one
// the installation step runs behind.
//
// The arrow keys (and h/l) move the value under the cursor by one step, `r` puts
// every row back on the host's recommendation, `w` writes the .wslconfig and `g`
// repairs the memory guard. Enter opens the row that leads somewhere -- the way
// back -- and does nothing to the two rows that act where they stand.
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
	case "w":
		// The write's own notice names the file before the write starts, so the
		// wait is visible and the result replaces it in the same rows.
		m.WSLNotice = fmt.Sprintf("Writing %s…", m.WSLState.Path)
		return m, m.wslResourceWriteCmd()
	case "g":
		// The guard is the part of this screen that is not the .wslconfig, so its
		// row says what it is doing before it starts: the install writes two files
		// and restarts a unit, and a silent press would read as nothing having
		// happened while it runs.
		m.WSLNotice = "Installing the WSL memory guard…"
		return m, m.wslGuardRepairCmd()
	case "enter":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if strings.Contains(options[m.Cursor], "Back") {
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
	// The report is read-only, so Esc is its only key: Enter opens, and there is
	// nothing here to open.
	switch key {
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
	}
	return m, nil
}

// handleShellAuditKeys drives the shell startup screen: the plan the rows above
// the measure row carry, the measurement itself, the keys that stop it, and the
// one setting on this screen that writes to the user's disk.
//
// The measurement is a command rather than something a keypress does, because it
// starts the user's shell several times; the session it starts is kept on the
// model, so the starts the user asks to stop are really stopped rather than
// watched from a screen they have left.
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
	case "m", "n", "w", "b":
		// The plan. None of it can change while a measurement is running: the run
		// carries the plan it started with, and a screen that redrew a new one beside
		// a number measured under the old one would be describing a measurement that
		// never happened.
		if m.ShellAudit.Measuring {
			return m, nil
		}
		m = m.cycleShellAuditPlan(key)
	case "h":
		// The history switch has a key of its own, because it is the one setting
		// here that writes a file into the user's home: turning it on and off should
		// not require finding a row first.
		return m.toggleShellAuditHistory()
	case "r":
		// Opening the recommendations moves them to the top of the text below, where
		// the frame's own cut cannot reach them first.
		m.ShellAudit.AdviceOpen = !m.ShellAudit.AdviceOpen
	case "s":
		// The measure row runs on its own letter: it starts the runs where it stands,
		// and Enter is left to the row that leaves the screen. One measurement at a
		// time, and none at all where the shell could not be resolved.
		if m.ShellAudit.Measuring || !m.ShellAudit.Available {
			return m, nil
		}
		m.ShellAudit.Measuring = true
		m.ShellAudit.Cancelled = false
		m.ShellAudit.Progress = shellAuditProgress{Part: shellAuditPartVersion}
		session, cmd := m.shellAuditCmd()
		m.ShellAuditSession = session
		return m, cmd
	case "enter":
		// Enter opens, and the only row here it can open is the way back: the
		// measure row acts where it stands, so Enter on it does nothing.
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if strings.Contains(options[m.Cursor], "Back") {
			m.Screen = ScreenUtilities
			m.Cursor = 0
		}
	}

	return m, nil
}

// cycleShellAuditPlan is the plan key: one setting moves one step, and the others
// are left where they are. The cycles are the engine's own lists, so a setting the
// screen offers is a setting the measurement can make.
func (m Model) cycleShellAuditPlan(key string) Model {
	switch key {
	case "m":
		m.ShellAudit.Plan.Mode = nextShellAuditMode(m.ShellAudit.Plan.Mode, m.ShellAudit.Kind)
	case "n":
		m.ShellAudit.Plan.Samples = nextShellAuditSamples(m.ShellAudit.Plan.Samples)
	case "w":
		m.ShellAudit.Plan.Warmup = nextShellAuditWarmup(m.ShellAudit.Plan.Warmup)
	case "b":
		m.ShellAudit.Plan.Baseline = !m.ShellAudit.Plan.Baseline
	}
	return m
}

// toggleShellAuditHistory turns the local history on and off and stores the
// choice. The measurements already stored are kept when it is turned off: the
// switch is about whether this utility keeps measuring, and throwing away what
// the user did ask for would be a deletion nobody asked for. Turning it on writes
// the file, so the choice survives the next start.
func (m Model) toggleShellAuditHistory() (tea.Model, tea.Cmd) {
	m.ShellAudit.History.Enabled = !m.ShellAudit.History.Enabled
	if m.ShellAudit.History.Enabled {
		return m, m.saveShellAuditHistoryCmd()
	}
	// Turning it off is also worth writing down: the file keeps the decisions, so
	// the switch is where the next run reads it from.
	return m, m.saveShellAuditHistoryCmd()
}

// handlePortManagerKeys drives the manager. It only ever changes the cursor,
// the controls, the detail flag or the notice. The reading is a command and the
// stop is a screen of its own, so nothing here reads the machine or signals
// anything: the two expensive halves stay off the update loop.
func (m Model) handlePortManagerKeys(key string) (tea.Model, tea.Cmd) {
	visible := m.portVisibleListeners()

	switch key {
	case "up", "k":
		if m.Ports.Cursor > 0 {
			m.Ports.Cursor--
		}
	case "down", "j":
		if m.Ports.Cursor < len(visible)-1 {
			m.Ports.Cursor++
		}
	case "enter":
		// The detail block is an expansion of the selected row, so Enter opens and
		// closes it and nothing is written. Space is not a second way to press it:
		// Space toggles a switch, and this screen's rows are data.
		m.Ports.Detail = !m.Ports.Detail
	case "/":
		// The filter field starts empty of control and owns the keys until it is
		// kept or discarded; the current text stays editable rather than being
		// cleared, so a filter can be refined.
		m.Ports.Filtering = true
		m.Ports.Notice = ""
	case "f":
		m.Ports.Transport = nextPortTransport(m.Ports.Transport)
		m.clampPortCursor()
	case "o":
		m.Ports.Owner = nextPortOwner(m.Ports.Owner)
		m.clampPortCursor()
	case "s":
		m.Ports.Sort = nextPortSort(m.Ports.Sort)
		m.clampPortCursor()
	case "r":
		// A manual refresh captures the selected row's identity first, so the
		// reading lands with the cursor still on the same socket.
		m.Ports.SelectedKey = m.portSelectedKey()
		m.Ports.Loading = true
		m.Ports.LoadErr = ""
		return m, m.portScanCmd()
	case "p":
		m.Ports.Auto = !m.Ports.Auto
		if m.Ports.Auto {
			return m, portsTickCmd()
		}
	case "x":
		// The confirmation opens only for a row whose owner is known and whose
		// stop the package's own rules allow. Anything else is a refusal on the
		// notice line, so no screen ever offers a signal that would be refused.
		target, ok := m.portStopTarget()
		if !ok {
			m.Ports.Notice = "The process behind this socket is not known to this run, so there is nothing to stop here."
			return m, nil
		}
		if refusal := portscan.StopRefusal(m.Ports.Probe, target); refusal != "" {
			m.Ports.Notice = refusal
			return m, nil
		}
		m.PortStop = portStopState{Target: target}
		m.Screen = ScreenPortStop
		m.Cursor = 0
	case "y":
		text := m.portDiagnosticText()
		if text == "" {
			m.Ports.Notice = "There is no row to copy."
			return m, nil
		}
		if m.TerminalCapabilities.Clipboard.State != capabilitySupported {
			m.Ports.Notice = "This terminal has not confirmed OSC 52, so the diagnostic was left on screen to copy by hand."
			return m, nil
		}
		m.Ports.Notice = "Sent the diagnostic to the terminal's clipboard over OSC 52."
		return m, copyPortDiagnosticCmd(text)
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.Ports.Notice = ""
	}
	return m, nil
}

// handlePortFilterKeys edits the filter field. Escape and Enter both keep the
// text (leaving it to be refined later); backspace deletes the last rune and a
// printable rune appends. Everything else is ignored, so a control key cannot
// grow the filter with its own name.
func (m Model) handlePortFilterKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter":
		m.Ports.Filtering = false
	case "backspace":
		if runes := []rune(m.Ports.Filter); len(runes) > 0 {
			m.Ports.Filter = string(runes[:len(runes)-1])
		}
		m.clampPortCursor()
	default:
		if runes := []rune(key); len(runes) == 1 && runes[0] >= ' ' && runes[0] != 0x7f {
			m.Ports.Filter += key
			m.clampPortCursor()
		}
	}
	return m, nil
}

// handlePortStopKeys drives the confirmation. The soft stop is the only action
// until a soft attempt leaves the process running; then the forced signal is the
// only one offered, and it is spelled as its own action. Escape returns to the
// manager without sending anything.
func (m Model) handlePortStopKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Screen = ScreenPortManager
		m.Cursor = 0
	case "t":
		if m.PortStop.Refusal != "" || m.PortStop.Running || m.portStopForcedOffered() {
			return m, nil
		}
		if r := m.PortStop.Result; r != nil && r.Outcome != portscan.OutcomeStillRunning {
			return m, nil
		}
		m.PortStop.Running = true
		m.PortStop.Forced = false
		return m, m.portStopCmd(portscan.SignalTerminate)
	case "f":
		if !m.portStopForcedOffered() || m.PortStop.Running {
			return m, nil
		}
		m.PortStop.Running = true
		m.PortStop.Forced = true
		return m, m.portStopCmd(portscan.SignalKill)
	}
	return m, nil
}

// applyPortSnapshot folds one reading into the model. The cursor is restored by
// the identity captured when the reading started, never by index; a row that is
// gone clamps the cursor instead. The counts of rows that appeared and
// disappeared are put on the notice so a refresh reports what changed without
// moving anything.
func (m *Model) applyPortSnapshot(snap portscan.Snapshot, wasLoaded bool) {
	beforeKeys := portListenerKeySet(m.portVisibleListeners())
	m.Ports.Snapshot = snap
	visible := m.portVisibleListeners()
	m.Ports.Cursor = portCursorForKey(visible, m.Ports.SelectedKey, m.Ports.Cursor)

	afterKeys := portListenerKeySet(visible)
	newCount, goneCount := 0, 0
	for key := range afterKeys {
		if !beforeKeys[key] {
			newCount++
		}
	}
	for key := range beforeKeys {
		if !afterKeys[key] {
			goneCount++
		}
	}
	if !wasLoaded {
		// The first reading has nothing to compare against; every row would count
		// as new and the notice would be noise.
		m.Ports.NewSince, m.Ports.GoneSince = 0, 0
		return
	}
	m.Ports.NewSince, m.Ports.GoneSince = newCount, goneCount
	if newCount > 0 || goneCount > 0 {
		m.Ports.Notice = fmt.Sprintf("%d new, %d gone since the last reading", newCount, goneCount)
	}
}

// portListenerKeySet is the set of socket identities in a list.
func portListenerKeySet(list []portscan.Listener) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, l := range list {
		out[l.Key()] = true
	}
	return out
}

// portCursorForKey puts the cursor on the row whose identity matches, and clamps
// a cursor whose row is gone rather than leaving it past the end.
func portCursorForKey(visible []portscan.Listener, key string, fallback int) int {
	if key != "" {
		for i, l := range visible {
			if l.Key() == key {
				return i
			}
		}
	}
	if fallback < 0 {
		fallback = 0
	}
	if fallback >= len(visible) {
		fallback = len(visible) - 1
	}
	if fallback < 0 {
		fallback = 0
	}
	return fallback
}

// clampPortCursor keeps the manager's cursor inside the filtered list after the
// list changed under it.
func (m *Model) clampPortCursor() {
	n := len(m.portVisibleListeners())
	if m.Ports.Cursor >= n {
		m.Ports.Cursor = n - 1
	}
	if m.Ports.Cursor < 0 {
		m.Ports.Cursor = 0
	}
}

// The three cycles the manager's keys walk. Each is a small closed set, so the
// value the screen prints is a member of the vocabulary the handler accepts.
func nextPortTransport(value string) string {
	switch value {
	case "all":
		return "tcp"
	case "tcp":
		return "udp"
	default:
		return "all"
	}
}

func nextPortOwner(value string) string {
	switch value {
	case "all":
		return "me"
	case "me":
		return "others"
	default:
		return "all"
	}
}

func nextPortSort(value string) string {
	switch value {
	case "port":
		return "process"
	case "process":
		return "family"
	default:
		return "port"
	}
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
	case "i":
		// Installing the release writes the running binary, so it runs on its own
		// letter rather than Enter. The swap is one command: the ownership question,
		// the two downloads, the checksum and the rename all happen off the update
		// loop, and the row is only ever drawn when this run may replace its own file.
		if !m.UpdateInstallable() || m.UpdateCheck.InFlight {
			return m, nil
		}
		m.UpdateCheck.InFlight = true
		m.UpdateCheck.Notice = "Downloading " + m.UpdateCheck.Latest + "…"
		return m, updateApplyCmd(m.updateChannel(), m.UpdateCheck.Latest, m.UpdateTarget)
	case "r":
		// Asking again is always allowed: an answer that is stale, wrong or missing
		// is exactly what a user asks a second time for. One check at a time, so the
		// last answer to arrive is the answer that was asked for.
		if m.UpdateCheck.InFlight {
			return m, nil
		}
		return m, m.startUpdateCheck()
	case "enter":
		// Enter opens, and the only row it opens here is the way back: the two rows
		// above act where they stand and say so on themselves.
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if strings.Contains(options[m.Cursor], "Back") {
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
	case "enter":
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
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.ThemeNotice = ""
			m.resetThemeRefresh()
		}
	case "r":
		// The refresh only opens the review; nothing is written until its own
		// confirmation is answered, so it runs on its own letter without taking the
		// screen away from the theme the cursor is previewing.
		if len(m.DotfilesThemes) == 0 || m.themeActivityPending() {
			return m, nil
		}
		return m, detectThemeRefreshCmd(os.Getenv("HOME"), m.DotfilesThemes)
	case "u":
		// The undo puts back what the last apply replaced. It is a write, so it runs
		// on its own letter, and it is offered only while a record exists.
		if m.DotfilesThemeRecord == nil || m.themeActivityPending() {
			return m, nil
		}
		m.startThemeActivity("Undoing the last dotfiles theme change…")
		return m, undoDotfilesThemeCmd(*m.DotfilesThemeRecord)
	}

	return m, nil
}

// handleThemeRefreshKeys drives the refresh review: the confirmation before
// anything is written, and the result after. The confirmation writes files, so it
// runs on its own letter rather than Enter, and the cancel leaves every file as
// it is on the letter the row spells; Enter is left to the result's way back.
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
	case "y":
		if m.ThemeRefreshDone {
			return m, nil
		}
		return m, refreshThemeFilesCmd(os.Getenv("HOME"), m.DotfilesThemes, m.ThemeRefreshCandidates)
	case "n":
		if m.ThemeRefreshDone {
			return m, nil
		}
		m.resetThemeRefresh()
		m.ThemeNotice = ""
	case "enter":
		// The result's only row is the way back to the list, and Enter opens it.
		// The review's own answers run on their letters, so Enter writes nothing.
		if !m.ThemeRefreshDone {
			return m, nil
		}
		m.resetThemeRefresh()
		m.ThemeNotice = ""
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

	case "enter":
		return m.handleSelection()
	}

	return m, nil
}

// goBackInstallStep handles going back during installation wizard.
//
// Every question screen is a detour of the setup screen now, so going back from
// one returns to the setup screen without changing anything: the chain of
// question-to-question transitions is gone, and there is no next question to
// step back to. The review screen keeps its own way back to the screen it was
// opened from.
func (m Model) goBackInstallStep() (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect:
		m.returnToSetup()

	case ScreenInstallReview:
		// Back to the question this screen was opened from, which is the setup
		// screen. The fallback is for a model built without that record, which is
		// what a test constructing the screen directly looks like.
		m.Screen = m.PrevScreen
		if m.Screen == ScreenInstallReview || m.Screen == ScreenInstalling {
			m.Screen = ScreenSetup
		}
		m.Cursor = 0
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
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect:
		// Every one of these screens is a detour of the setup screen now: the
		// choice is recorded here, by the one apply helper the row also uses, and
		// the flow returns to the screen that opened it instead of advancing along
		// the old chain of questions. There is no next question to advance to.
		m.applyQuestionAnswer(m.Screen, options[m.Cursor])

		// The terminal question has one consequence that is not a value: on a
		// Debian host, choosing Ghostty puts the compatibility warning up. It is
		// part of this detour, and every route out of it leads back to the setup
		// screen.
		if m.Screen == ScreenTerminalSelect &&
			m.Choices.Terminal == "ghostty" && m.Choices.OS == "linux" &&
			m.SystemInfo != nil && m.SystemInfo.OS == system.OSDebian &&
			!system.CommandExists("ghostty") {
			m.Screen = ScreenGhosttyWarning
			m.Cursor = 0
			return m, nil
		}

		m.returnToSetup()

	case ScreenGhosttyWarning:
		switch m.Cursor {
		case 0: // Continue with Ghostty anyway
			m.returnToSetup()
		case 1: // Choose a different terminal
			m.Screen = ScreenTerminalSelect
			if index := m.questionValueIndex(ScreenTerminalSelect); index >= 0 {
				m.Cursor = index
			} else {
				m.Cursor = 0
			}
		case 2: // Cancel installation
			m.Screen = ScreenMainMenu
			m.Cursor = 0
		}
	}

	return m, nil
}

// handleLearnHubKeys walks the Learn About Tools hub. Enter resolves the row
// through learnToolRows() rather than by its position, so the page a row opens
// and the row the menu draws cannot drift apart, and Escape is the way back to
// whatever opened the hub.
// handleReferenceTableKeys scrolls a read-only reference page and leaves it. The
// pages share one scroll value because only one of them is on screen at a time,
// and the value is reset when the page is opened and when it is left, so a page
// never opens part-way down a table the reader has not seen. The window and its
// clamp come from the same helper the renderer windows with, so the last row is
// reachable.
func (m Model) handleReferenceTableKeys(key string, rows int) (tea.Model, tea.Cmd) {
	maxScroll := offsetWindowMax(keymapTableRows(m.Height), rows)

	switch key {
	case "up", "k":
		if m.ReferenceTableScroll > 0 {
			m.ReferenceTableScroll--
		}
	case "down", "j":
		if m.ReferenceTableScroll < maxScroll {
			m.ReferenceTableScroll++
		}
	case "enter":
		m.Screen = m.PrevScreen
		m.ReferenceTableScroll = 0
	}

	return m, nil
}

func (m Model) handleLearnHubKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if m.learnSeparatorAt(options, m.Cursor) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if m.learnSeparatorAt(options, m.Cursor) && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
	case "enter":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := options[m.Cursor]
		if m.learnSeparatorAt(options, m.Cursor) || strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			return m, nil
		}
		row, ok := learnToolRowByLabel(selected)
		if !ok {
			return m, nil
		}
		m.PrevScreen = ScreenLearnTools
		m.Screen = row.Screen
		m.Cursor = 0
		m.ViewingTool = ""
		m.ReferenceTableScroll = 0
	}

	return m, nil
}

// isSingleToolInfoScreen reports whether the screen is showing one tool's page
// rather than a list. The learn screens borrow the same handler for the list and
// the page, so the page's own key -- quit -- is read before that dispatch; a
// screen showing its menu is not one of these, and the list keeps only its own
// keys.
func (m Model) isSingleToolInfoScreen() bool {
	switch m.Screen {
	case ScreenLearnFont:
		return true
	case ScreenLearnPrompts, ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM:
		return m.ViewingTool != ""
	case ScreenLearnNvim:
		return m.ViewingTool == "features"
	}
	return false
}

// learnSeparatorAt reports whether the option at an index is the menu's rule.
func (m Model) learnSeparatorAt(options []string, index int) bool {
	if index < 0 || index >= len(options) {
		return false
	}
	return strings.HasPrefix(options[index], m.menuSeparator())
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
	case "enter":
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
	case "enter":
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
	case "enter", "esc":
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
	case "enter":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = m.PrevScreen
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		// The menu routes by the label rather than by the row index it used to
		// switch on: a positional switch silently sent a new row to the wrong
		// tool's reference as soon as a row was inserted above another one.
		switch selected {
		case "Neovim":
			m.Screen = ScreenKeymaps
		case "Tmux":
			m.Screen = ScreenKeymapsTmux
		case "Zellij":
			m.Screen = ScreenKeymapsZellij
		case "Herdr":
			m.Screen = ScreenKeymapsHerdr
		case "Ghostty":
			m.Screen = ScreenKeymapsGhostty
		case "Kitty":
			m.Screen = ScreenKeymapsKitty
		case "WezTerm":
			m.Screen = ScreenKeymapsWezTerm
		case "Alacritty":
			m.Screen = ScreenKeymapsAlacritty
		}
		m.Cursor = 0
		m.ReferenceTableScroll = 0
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
	case "enter":
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
	case "enter", "esc":
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
	case "enter":
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
	case "enter", "esc":
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
	case "enter":
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
	case "enter", "esc":
		m.Screen = ScreenKeymapsGhostty
		m.GhosttyKeymapScroll = 0
	}

	return m, nil
}

// handleKittyKeymapsMenuKeys handles Kitty keymap category selection
func (m Model) handleKittyKeymapsMenuKeys(key string) (tea.Model, tea.Cmd) {
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
	case "enter":
		selected := options[m.Cursor]
		if strings.Contains(selected, "Back") {
			m.Screen = ScreenKeymapsMenu
			m.Cursor = 0
			return m, nil
		}
		if strings.HasPrefix(selected, "───") {
			return m, nil
		}

		m.KittySelectedCategory = m.Cursor
		m.Screen = ScreenKeymapsKittyCat
		m.KittyKeymapScroll = 0
	}

	return m, nil
}

// handleKittyKeymapCategoryKeys handles scrolling in the Kitty keymap category view
func (m Model) handleKittyKeymapCategoryKeys(key string) (tea.Model, tea.Cmd) {
	category := m.KittyKeymapCategories[m.KittySelectedCategory]

	visibleItems := keymapTableRows(m.Height)
	maxScroll := offsetWindowMax(visibleItems, len(category.Keymaps))

	switch key {
	case "up", "k":
		if m.KittyKeymapScroll > 0 {
			m.KittyKeymapScroll--
		}
	case "down", "j":
		if m.KittyKeymapScroll < maxScroll {
			m.KittyKeymapScroll++
		}
	case "enter", "esc":
		m.Screen = ScreenKeymapsKitty
		m.KittyKeymapScroll = 0
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
	case "enter":
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
	case "enter", "esc":
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
	case "enter":
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
	case "enter", "esc":
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
	case "enter":
		switch m.Cursor {
		case 0: // Install with Backup
			m.Choices.CreateBackup = true
			m.returnToSetup()
			return m, nil
		case 1: // Install without Backup
			m.Choices.CreateBackup = false
			m.returnToSetup()
			return m, nil
		case 2:
			// This screen is a detour of the setup screen's backup row now, so its way
			// out is the detour's way out: back to the setup screen with nothing
			// changed, which is what Escape does. It used to be the flow's last
			// question, where its third row aborted the wizard; a row on a screen the
			// user can still leave cannot abort anything.
			m.returnToSetup()
		}
	case "esc":
		m.returnToSetup()
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
	case "enter":
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
	case "enter":
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

	// A unit the user turned off is planned as skipped rather than dropped, so the
	// list keeps showing it. Skipping it here -- before the status is set to running
	// and before a credential is asked for on its behalf -- is what "not executed"
	// means on this route, and the step advances exactly as one that finished does.
	//
	// The walk recurses rather than falling through, so a skipped unit at the end of
	// the plan finishes the run instead of leaving it waiting on a step that will
	// never execute. It cannot run away: each call advances CurrentStep, and the
	// guard above ends the walk at the plan's length.
	if stepIsSkipped(*step) {
		m.LogLines = append(m.LogLines, "⏭ Skipped: "+step.Name+" — you turned it off.")
		m.CurrentStep++
		return m.runNextStep()
	}

	step.Status = StatusRunning

	// Check if this step needs interactive input (sudo, chsh, etc)
	if step.Interactive {
		// The WSL step is the one interactive step with a decision to put to the
		// user before the script runs: whether Windows Terminal may be configured.
		// The offer is read in a command and the installing screen shows it, and the
		// script that needs sudo runs only after the answer. This is what makes the
		// offer reachable from the TUI at all -- for a release the offer functions
		// were called only by a test. A dry run skips the step entirely, offer
		// included, exactly as runInteractiveStep's own gate does.
		if step.ID == wslStepID && !dryRun() && !m.WSLTerminalOfferDone && m.SystemInfo != nil && m.SystemInfo.IsWSL {
			return m.wslTerminalOfferCmd()
		}
		if m.shouldPrimeSudo(step.ID) {
			return m.sudoPrimeCmd()
		}
		return runInteractiveStep(step.ID, m)
	}

	// A step that needs root asks for the credential once before it starts, if no
	// earlier step already did. The check is here rather than in executeStep so
	// both routes (the captured script and the Go executor) share it.
	if m.shouldPrimeSudo(step.ID) {
		return m.sudoPrimeCmd()
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

// shouldPrimeSudo reports whether starting stepID has to ask for the
// administrator credential first. It is true once per run, before the first
// step that needs root, and never under --dry-run, which runs nothing.
func (m *Model) shouldPrimeSudo(stepID string) bool {
	return !dryRun() && !m.SudoPrimed && !m.SudoPrimeAttempted && stepNeedsSudo(stepID, m)
}

// sudoPrimeCmd asks for the administrator credential once and records that the
// ask is visible on the installing screen. The prompt is what makes the rest of
// the run possible without owning the terminal: once sudo holds the cached
// credential, every step's `sudo -n` needs no TTY.
func (m *Model) sudoPrimeCmd() tea.Cmd {
	m.SudoPrimeAttempted = true
	m.LogLines = append(m.LogLines, "Administrator access: one password prompt will cover the whole run.")
	return sudoPrimer()
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
// trainerHintPrefix is what every hint the trainer reveals is labelled with, so the
// exercise screen and the boss screen say the same thing about the same kind of help.
const trainerHintPrefix = "💡 Hint: "

func trainerHintLabel(exercise *trainer.Exercise) string {
	if exercise == nil || exercise.Hint == "" {
		return ""
	}
	return trainerHintPrefix + exercise.Hint
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
			if !m.TrainerStats.IsPracticeReady(module.ID) {
				m.TrainerMessage = "Complete all lessons first to unlock practice!"
				return m, nil
			}

			// Nothing left to practise means every exercise is mastered AND the boss
			// is unlocked. That is the only state in which the session would end on
			// its first answer, so it is the only state that refuses to start; an
			// all-mastered module whose boss gate is unmet keeps offering review,
			// which is how the player reaches the gate at all.
			// time.Now is the menu's own clock: the rule lives in the trainer package
			// and takes the moment as an argument, and the menu is the view layer that
			// has to supply one. A due review is work, so a module with reviews due
			// starts a session even when the boss is already unlocked.
			if m.TrainerStats.PracticeComplete(module.ID, time.Now()) {
				m.TrainerMessage = "🎉 Nothing left to practise in " + module.Name + ": the boss is unlocked. Press [b]."
				return m, nil
			}

			m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
			m.TrainerGameState.StartPractice(module.ID)

			if m.TrainerGameState.CurrentExercise == nil {
				m.TrainerMessage = "No exercises to practise in " + module.Name + " yet."
				return m, nil
			}

			m.TrainerInput = ""
			if due := m.TrainerStats.ReviewsDue(module.ID, time.Now()); due > 0 {
				word := "reviews"
				if due == 1 {
					word = "review"
				}
				m.TrainerMessage = fmt.Sprintf("🔁 Review run: %d mastered %s due.", due, word)
			} else if m.TrainerStats.IsPracticeComplete(module.ID) {
				m.TrainerMessage = fmt.Sprintf("🔁 Review run: every exercise is mastered, and the boss still needs %.0f%% of your last %d practice answers.",
					trainer.BossGateAccuracy*100, trainer.BossGateWindow)
			} else {
				m.TrainerMessage = ""
			}
			m.Screen = ScreenTrainerPractice
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

		// The answer recorders above own every counter, practice included, so the
		// screen records nothing itself: counting the submission here as well is
		// what used to double every practice attempt. The screen only persists the
		// profile the answer moved.
		if m.TrainerGameState.IsPracticeMode {
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
		// The boss screen asks trainer.BossStepNudge rather than trainerHintLabel:
		// the step's own hint when it ships one, and the first keystrokes of its
		// answer when it does not. Tab used to be dead on every step without an
		// authored hint, which was all 35 of them, so the key did nothing at all in
		// the one place the player most wants a nudge.
		if nudge := trainer.BossStepNudge(m.TrainerGameState.CurrentExercise); nudge != "" {
			m.TrainerMessage = trainerHintPrefix + nudge
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
			// failed save is reported instead of being hidden behind it. A practice
			// session ends only when there is nothing left to do, so the message
			// names the thing that is now open rather than claiming mastery twice.
			if m.TrainerGameState.IsPracticeMode {
				m.TrainerMessage = "🎉 Practice complete — every exercise mastered and the boss is unlocked. Press [b] on the menu."
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
