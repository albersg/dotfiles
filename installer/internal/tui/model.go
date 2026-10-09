package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen represents the current screen being displayed
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenMainMenu
	ScreenOSSelect
	ScreenTerminalSelect
	ScreenFontSelect
	ScreenShellSelect
	ScreenWMSelect
	ScreenNvimSelect
	ScreenInstalling
	ScreenComplete
	ScreenError
	// Learn screens
	ScreenLearnTerminals
	ScreenLearnShells
	ScreenLearnWM
	ScreenLearnNvim
	// Keymaps screen
	ScreenKeymaps
	ScreenKeymapCategory
	// Tool Keymaps screens
	ScreenKeymapsMenu       // Menu to select which tool's keymaps to view
	ScreenKeymapsTmux       // Tmux keymaps
	ScreenKeymapsTmuxCat    // Tmux keymap category
	ScreenKeymapsZellij     // Zellij keymaps
	ScreenKeymapsZellijCat  // Zellij keymap category
	ScreenKeymapsGhostty    // Ghostty keymaps
	ScreenKeymapsGhosttyCat // Ghostty keymap category
	// LazyVim learn screens
	ScreenLearnLazyVim
	ScreenLazyVimTopic
	// Backup screens
	ScreenBackupConfirm
	ScreenRestoreBackup
	ScreenRestoreConfirm
	// Warning screens
	ScreenGhosttyWarning // Warning about Ghostty compatibility on Debian/Ubuntu
	// Vim Trainer screens
	ScreenTrainerMenu       // Module selection
	ScreenTrainerLesson     // Lesson mode
	ScreenTrainerPractice   // Practice mode
	ScreenTrainerBoss       // Boss fight
	ScreenTrainerResult     // Result after exercise
	ScreenTrainerBossResult // Result after boss fight
	// Herdr keymap screens. Appended at the end to keep the numeric values of
	// the existing screens stable.
	ScreenKeymapsHerdr    // Herdr keymaps
	ScreenKeymapsHerdrCat // Herdr keymap category
	// Utilities section. Appended at the end for the same reason. It is reached
	// from the main menu both by its own row, just above Exit, and by the `u`
	// shortcut.
	ScreenUtilities // System utilities, starting with the theme switch
	// Theme picker. The utilities section opens it from its single "Change the
	// dotfiles theme" row, and the list of themes lives here so the section is a
	// list of jobs rather than a list of themes.
	ScreenThemePicker // The dotfiles themes, previewed and applied
	// WSL resources. The utilities section opens it from its own row, one level
	// in for the same reason: the section is a list of jobs. It shows the memory,
	// processors and swap the user's .wslconfig holds beside the values
	// recommended from the Windows host, and writes them back through the same
	// builder and writer the installation step uses.
	ScreenWSLResources // The WSL memory, processors and swap, seen and set
	// Terminal capabilities. The utilities section opens it from its own row.
	// It reports what the terminal the installer is running in can do -- the
	// colour depth, OSC 52, synchronized output and the Nerd Font glyphs -- from
	// the environment and from a bounded query, and says unknown with a reason
	// and a manual check for what it cannot determine. It is read-only: nothing
	// is written and nothing is run.
	ScreenTerminalCapabilities // What this terminal can do, and what it means
	// The shell's startup. The utilities section opens it from its own row, one
	// level in for the same reason: the section is a list of jobs. It starts the
	// login shell the way a terminal does, several times, reports the median and
	// the range, and names the functions zsh's own profiler blames -- or says
	// plainly that only the total is measurable. It changes nothing at all.
	ScreenShellAudit // How long the login shell takes to start, and what eats it
	// This installer's own release. Appended at the end for the same reason as the
	// screens above. The utilities section opens it from its own row, one level in
	// so the section stays a list of jobs. It reports which release is published
	// beside the build that is running, where that answer came from and when it was
	// read, and can install the published release over this binary after verifying
	// it against the release's own SHA256SUMS -- unless a package manager owns the
	// file, in which case it says so and names the command that does own it instead
	// of writing over a file another program owns.
	ScreenUpdate // The published release, beside this build
)

// InstallStep represents a single installation step
type InstallStep struct {
	ID          string
	Name        string
	Description string
	Status      StepStatus
	Progress    float64
	Error       error
	Interactive bool // If true, this step needs terminal control (sudo, chsh, etc)
}

type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
	StatusSkipped
)

// UserChoices stores all user selections
type UserChoices struct {
	OS           string // "mac", "linux"
	Terminal     string // "alacritty", "wezterm", "kitty", "ghostty", "none"
	InstallFont  bool
	Shell        string // "fish", "zsh", "nushell"
	WindowMgr    string // "tmux", "zellij", "herdr", "none"
	InstallNvim  bool
	CreateBackup bool // Whether to backup existing configs
}

// Model is the main application state
type Model struct {
	Screen      Screen
	PrevScreen  Screen // For going back from learn/keymaps screens
	Width       int
	Height      int
	SystemInfo  *system.SystemInfo
	Choices     UserChoices
	Steps       []InstallStep
	CurrentStep int
	Cursor      int
	// PanelIndex is which panel of the current screen's list is being shown. The
	// registry's first panel is the default, and the index is reset to it whenever
	// the screen changes, so a screen always opens on the panel its own question
	// is about.
	PanelIndex int
	// Animating is the animation gate: true when this run may schedule the frame
	// tick. It is decided once, when the model is built, from the environment and
	// the stream the run would draw to, and stored here so a test can force either
	// side without touching the environment.
	Animating bool
	// AnimTick counts the animation frames since the run started: one per frame
	// tick, animTicksPerSecond of them a second. It stays at zero while animation
	// is off, and it is the clock the tip rotation and the companion read instead
	// of time.Now, so a snapshot can pin a frame instead of flaking.
	AnimTick int
	// MenuKeyBuffer holds only a prefix of one of the main-menu easter eggs. It is
	// capped at the longest form (three runes) and discarded as soon as an event
	// does not extend it, so it never queues or delays ordinary input.
	MenuKeyBuffer string
	// MenuGagRow is the option row struck by dd, and MenuGagTicks counts the
	// deterministic ticks the decoration has been showing. MenuGagActive keeps
	// row zero distinguishable from the inactive zero value.
	MenuGagRow    int
	MenuGagTicks  int
	MenuGagActive bool
	// CompanionPos is the cell the companion starts at on the stage it walks along.
	// It is state rather than a function of the tick because the creature walks
	// toward a target that moves, and together with AnimTick it is everything a
	// snapshot needs to pin a frame and a position.
	CompanionPos int
	// CompanionFollow is set by a key that moved the selection or changed the
	// screen and cleared when the creature reaches the row the cursor points at. It
	// is what tells a walk that has somewhere to go from a creature standing still.
	CompanionFollow bool
	// CompanionMoving records whether the last tick actually moved it, which is
	// what the walking frames mean: a standing creature draws the idle frame while
	// the clock keeps running.
	CompanionMoving bool
	// CompanionIdle counts the animation frames since the last key press. It is the
	// stretch the sleeping frame is drawn from, and it lives on the model rather
	// than in the renderer so a test can reach the sleeping state without waiting
	// out companionSleepSeconds.
	CompanionIdle int
	// CompanionPleased counts the frames left of a celebration: an installation
	// step that has just finished is worth companionPleasedTicks of them.
	CompanionPleased int
	// CompanionGaze is where the creature is looking: the horizontal pupil
	// position (-1 left, 0 centre, +1 right) and the vertical one (-1 up, 0
	// level). It lives on the model, like the position and the frame, so a
	// snapshot can pin a gaze and the pointer task only has to write it from a
	// mouse message; a render never computes it.
	CompanionGaze companionGaze
	// Hovering says whether this run asked the terminal for pointer motion. It is
	// the mouse's own gate, decided once when the model is built and read from the
	// model afterwards, so a render never reads the environment and a test can force
	// either side without touching it.
	Hovering bool
	// PointerCol and PointerRow are the last cell the pointer was seen on, in the
	// terminal's own coordinates, and PointerSet records whether any pointer event
	// has arrived at all: before the first one the creature looks at the selection,
	// which is what it did before the pointer existed, and a terminal that refuses
	// mouse reporting keeps it there for the whole run.
	PointerCol, PointerRow int
	PointerSet             bool
	// CompanionHop counts the frames left of the little jump a click earns. It is
	// counted on the model, not in the renderer, so a snapshot can pin the jump the
	// way it pins a frame, and a frame that cannot spare the row draws the creature
	// on the ground instead of failing.
	CompanionHop int
	// PixelSprite says whether this run draws the creature as the shaded pixel sprite
	// instead of the glyph art. It is the terminal's answer, read once when the model
	// is built, so a render never asks the terminal anything.
	PixelSprite bool
	// ink is the palette the pixel sprite is drawn with, resolved once beside the
	// gate for the same reason: resolving it asks the terminal about its background.
	ink companionInk

	// Metrics is the ring of the host's derived readings, oldest first and newest
	// last, capped at metricsRingSize. It is on the model so a snapshot can pin a
	// series: a renderer never samples the machine, it draws the samples it was
	// handed.
	Metrics []system.Metrics
	// MetricsPrev is the raw sample the next derived reading is measured against.
	// CPU busy is a delta between two readings, so the previous one is model state
	// like everything else the chart shows.
	MetricsPrev system.Sample
	// ProgressSamples is the run's own progress over time, one value per metrics
	// sample, appended only while a run is in flight and capped at metricsRingSize.
	// It is the series behind the installing screen's run chart.
	ProgressSamples []float64
	// Celebrating is true for the short burst that greets a finished run, and
	// CelebrationTick counts its frames. Both live on the model so the burst is a
	// deterministic function of state and a snapshot can pin the frame it is on.
	Celebrating     bool
	CelebrationTick int
	// Particles are the burst's cells. They live on the model for the same reason
	// the creature's position does: a snapshot must be able to pin a frame.
	Particles   []celebrationParticle
	ErrorMsg    string
	ShowDetails bool
	LogLines    []string
	TotalTime   float64
	// CreatedAt is the moment the model was built. It is state so the welcome and
	// main menu greet by the time of day without a renderer ever reading the
	// clock: a render that called time.Now could not be tested or snapshotted, and
	// the greeting would change when nothing else on the screen did.
	CreatedAt time.Time
	// Now is the time of the latest tick, copied from the tick's own timestamp. The
	// installing screen derives its elapsed time from this and InstallStartedAt
	// rather than reading the clock while rendering, for the same reason: the
	// estimate would move on every repaint even when the run did not.
	Now time.Time
	// InstallStartedAt is when the run began. It is zero before a run and on the
	// models the golden tests build, and a zero start means the screen shows no
	// elapsed time and no estimate instead of a number it cannot justify.
	InstallStartedAt time.Time
	Quitting         bool
	// Program reference for sending messages during installation
	Program *tea.Program
	// Learn mode
	ViewingTool string // Current tool being viewed in learn mode
	// Keymaps mode
	KeymapCategories []KeymapCategory
	SelectedCategory int
	KeymapScroll     int // For scrolling through keymaps
	// Tool-specific keymaps
	TmuxKeymapCategories    []KeymapCategory
	TmuxSelectedCategory    int
	TmuxKeymapScroll        int
	ZellijKeymapCategories  []KeymapCategory
	ZellijSelectedCategory  int
	ZellijKeymapScroll      int
	GhosttyKeymapCategories []KeymapCategory
	GhosttySelectedCategory int
	GhosttyKeymapScroll     int
	HerdrKeymapCategories   []KeymapCategory
	HerdrSelectedCategory   int
	HerdrKeymapScroll       int
	// LazyVim mode
	LazyVimTopics        []LazyVimTopic
	SelectedLazyVimTopic int
	LazyVimScroll        int // For scrolling through topic content
	// Backup mode
	ExistingConfigs  []string            // Configs that will be overwritten
	AvailableBackups []system.BackupInfo // Available backups for restore
	SelectedBackup   int                 // Selected backup index
	BackupDir        string              // Last backup directory created
	// LastInstall is the record of the previous completed run -- when it finished,
	// the build it ran and the configuration paths it replaced -- read from the
	// state file on the startup path. It is nil when there is no record, and the
	// main menu then offers no "Last install" panel at all rather than a section
	// that says "never".
	LastInstall *lastInstall
	// Repository checkout created by the clone step
	WorkDir string // Private temporary directory owned by this run (empty until clone)
	RepoDir string // Repository checkout inside WorkDir (empty until clone)
	// Vim Trainer mode
	TrainerStats       *trainer.UserStats   // User's training stats
	TrainerGameState   *trainer.GameState   // Current game session state
	TrainerModules     []trainer.ModuleInfo // Available modules
	TrainerCursor      int                  // Cursor for module selection
	TrainerInput       string               // User's input for current exercise
	TrainerLastCorrect bool                 // Was last answer correct
	TrainerMessage     string               // Feedback message to display
	// TrainerValidation is the detailed validation of the answer shown on the
	// result screen. It is nil until an answer is submitted, and only
	// buffer-verified answers carry a buffer for the result screen to show.
	TrainerValidation *trainer.ValidationResult
	// TrainerCodeScroll is the first code row the exercise screen's code window
	// shows, and TrainerCodeScrollFor names the exercise those rows were chosen
	// for. The window shows the exercise's entry position until the two agree,
	// which is how a freshly presented exercise opens on the code row its start
	// cursor is on without every presentation path having to remember to reset it.
	TrainerCodeScroll    int
	TrainerCodeScrollFor string
	// Leader key mode (like Vim's <space> leader)
	LeaderMode bool // True when waiting for next key after <space>

	// ThemeSwitch is the desktop's reversible light/dark switch, and
	// ThemeSwitchFound says whether this host has one. Both are decided once when
	// the model is built, so the utilities section reads the answer off the model
	// and never probes the machine while drawing.
	ThemeSwitch      themeSwitch
	ThemeSwitchFound bool
	// ThemeRecord is the setting this installer last replaced, read from its own
	// state file on the startup path. It is nil when there is no record, and the
	// section then offers no undo row rather than one that would fail.
	ThemeRecord *themeRecord
	// ThemeNotice is the one-line result of the last switch or undo, cleared when
	// the section is left. An error is shown here too: the theme switch is not an
	// install step, so it does not take the run to the failure screen.
	ThemeNotice string
	// DotfilesThemes are the theme definitions read from the repository checkout,
	// DotfilesRepoDir is the directory they were read from (the same checkout), and
	// DotfilesThemesErr is why they could not be read. The section offers a row per
	// complete definition; an empty list with no error means the checkout does not
	// exist yet, which the section says in its own body.
	DotfilesThemes    []themeDefinition
	DotfilesRepoDir   string
	DotfilesThemesErr string
	// DotfilesThemeRecord is the last dotfiles-theme change, read from the same
	// theme.json. It is what makes the section's Undo row appear, and it is nil
	// when there is nothing to put back.
	DotfilesThemeRecord *dotfilesThemeRecord
	// WSLState is what the WSL resource utility draws and writes: where the
	// user's .wslconfig is, what the Windows host can give, what was recommended
	// from it, what the file holds and the draft being edited. It is read in a
	// command, because detecting the host runs powershell.exe.
	WSLState wslResourceState
	// WSLNotice is the one-line result of the last write, cleared when the screen
	// is left, exactly as ThemeNotice is.
	WSLNotice string
	// TerminalCapabilities is the terminal capability report: what the terminal
	// the installer is running in can do, where each answer came from, and what
	// it implies. It is read by a command only when the capability screen is
	// opened -- never at startup -- and stored here so the render never probes.
	TerminalCapabilities terminalCapabilities
	// ShellAudit is what the shell startup utility draws: the login shell, the
	// method it is measured with, the runs and their median, and the functions
	// zprof named -- or the reason nothing could be named. Its environment half is
	// resolved once, here; the measurement runs only when the row is pressed,
	// because starting the user's shell five times is not a render.
	ShellAudit shellAuditState
	// UpdateCheck is what the installer knows about the latest published release:
	// the tag, when it was read, and the reason it could not be read. The cached
	// record is loaded on the startup path, so a start never waits on the network;
	// the refresh itself runs behind a command on the same gate the drawing uses.
	// It is named UpdateCheck rather than Update because Update is the model's own
	// method.
	UpdateCheck updateState
	// UpdateTarget is the binary this run would replace if the user asked it to
	// install the published release, and the asset that would replace it. It is
	// resolved once, here, for the same reason the desktop's theme switch is: which
	// file this run is running from, and who owns it, are questions a render must
	// not ask. UpdateTargetErr is why it could not be resolved at all.
	UpdateTarget    updateTarget
	UpdateTargetErr string
	// ThemeRefreshCandidates is the list the refresh detection found, or nil when
	// no detection has run. The review names it before anything is written.
	ThemeRefreshCandidates []themeRefreshCandidate
	// ThemeRefreshReview is true while the picker is showing the refresh review:
	// the candidate list with its confirmation, or the result once it has run.
	ThemeRefreshReview bool
	// ThemeRefreshDone is true once a refresh has run, so the review shows the
	// result rather than the confirmation.
	ThemeRefreshDone bool
	// ThemeRefreshResult is the review's result paragraphs: what was refreshed,
	// where the user's files were preserved, and what could not be refreshed.
	ThemeRefreshResult []string
	// ThemeActivity is the dotfiles-theme switch the picker is running, or the
	// one whose result is on screen. Its pending line and its result are drawn in
	// the same slot, which is what makes choosing a theme one change on screen
	// instead of a selection now and a different panel a second later. It is nil
	// when the picker has nothing to report, and the result view of the refresh
	// review is a different thing entirely.
	ThemeActivity *themeActivity
}

// themeActivity is one dotfiles-theme switch on the picker. Pending is the line
// the screen stands behind while the switch runs; Result is what came of it, in
// the same rows. Exactly one of them carries the slot at a time, so a switch is
// drawn at one height from the press to the outcome and the row under the
// cursor never has to move for the result to appear.
type themeActivity struct {
	// Pending is non-empty from the press until the outcome arrives.
	Pending string
	// Result is the outcome: the per-tool reload paragraphs, or the one line that
	// says why the switch failed. It is empty while Pending is set.
	Result []string
}

// NewModel creates a new Model with initial state
func NewModel() Model {
	m := Model{
		Screen:                  ScreenWelcome,
		PrevScreen:              ScreenWelcome,
		Width:                   80,
		Height:                  24,
		SystemInfo:              system.Detect(),
		Choices:                 UserChoices{},
		Steps:                   []InstallStep{},
		CurrentStep:             0,
		Cursor:                  0,
		PanelIndex:              0,
		Animating:               animationGate(os.Stdout),
		Hovering:                hoverRequested(),
		PixelSprite:             pixelSpriteGate(),
		ink:                     companionInkFor(lipgloss.HasDarkBackground()),
		AnimTick:                0,
		CreatedAt:               time.Now(),
		ShowDetails:             false,
		LogLines:                []string{},
		KeymapCategories:        GetNvimKeymaps(),
		SelectedCategory:        0,
		KeymapScroll:            0,
		TmuxKeymapCategories:    GetTmuxKeymaps(),
		TmuxSelectedCategory:    0,
		TmuxKeymapScroll:        0,
		ZellijKeymapCategories:  GetZellijKeymaps(),
		ZellijSelectedCategory:  0,
		ZellijKeymapScroll:      0,
		GhosttyKeymapCategories: GetGhosttyKeymaps(),
		GhosttySelectedCategory: 0,
		GhosttyKeymapScroll:     0,
		HerdrKeymapCategories:   GetHerdrKeymaps(),
		HerdrSelectedCategory:   0,
		HerdrKeymapScroll:       0,
		LazyVimTopics:           GetLazyVimTopics(),
		SelectedLazyVimTopic:    0,
		LazyVimScroll:           0,
		ExistingConfigs:         []string{},
		AvailableBackups:        []system.BackupInfo{},
		SelectedBackup:          0,
		BackupDir:               "",
		Program:                 nil, // Will be set after tea.Program is created
		// Trainer initialization
		TrainerStats:       nil, // Will be loaded when entering trainer
		TrainerGameState:   nil,
		TrainerModules:     trainer.GetAllModules(),
		TrainerCursor:      0,
		TrainerInput:       "",
		TrainerLastCorrect: false,
		TrainerMessage:     "",
		TrainerValidation:  nil,
	}
	// The desktop's theme switch is detected once here, the way the host itself
	// is, so no screen probes the environment or PATH while it draws.
	m.ThemeSwitch, m.ThemeSwitchFound = currentThemeSwitch(m.SystemInfo)
	// The shell the startup utility would measure is resolved here for the same
	// reason: which shell, and whether this run can start an interactive one, are
	// questions a render must not ask. The measurement itself is a keypress away.
	m.ShellAudit = resolveShellAudit(defaultShellAuditProbe())
	// The last release check is read here, from its own state file, exactly as the
	// last-install record is: one file read, no waiting. The request itself is a
	// command the run arms from the drawing gate, so the startup path never reaches
	// the network. The binary this run would replace is resolved here too, so the
	// update screen can say who owns the file before the user presses anything.
	m.UpdateCheck = loadUpdateState()
	target, updateTargetErr := resolveUpdateTarget()
	m.UpdateTarget = target
	if updateTargetErr != nil {
		m.UpdateTargetErr = updateTargetErr.Error()
	}
	// The gaze is settled once here, so a model that never sees a key, a resize or
	// a pointer event still draws eyes that are looking at what it starts on, and
	// the first tick does not have to move them.
	m.aimCompanion()
	return m
}

// SetProgram sets the tea.Program reference for sending messages during installation
func (m *Model) SetProgram(p *tea.Program) {
	m.Program = p
}

// globalProgram holds a reference to the tea.Program for sending logs during installation
var globalProgram *tea.Program

// SetGlobalProgram sets the global program reference
func SetGlobalProgram(p *tea.Program) {
	globalProgram = p
}

// nonInteractiveMode indicates if we're running without TUI
var nonInteractiveMode bool

// SetNonInteractiveMode enables or disables non-interactive mode
func SetNonInteractiveMode(enabled bool) {
	nonInteractiveMode = enabled
}

// SendLog sends a log message to the TUI during installation
func SendLog(stepID string, log string) {
	if nonInteractiveMode {
		// In non-interactive mode, print to stdout if verbose
		if os.Getenv("DOTFILES_VERBOSE") == "1" {
			fmt.Printf("    %s\n", log)
		}
		return
	}
	if globalProgram != nil {
		globalProgram.Send(stepProgressMsg{
			stepID: stepID,
			log:    log,
		})
	}
}

// SendLogLine is an alias for SendLog for compatibility
func (m *Model) SendLog(stepID string, log string) {
	SendLog(stepID, log)
}

// menuSeparatorPrefix is the leading rune of the divider row. The key handlers
// test the same run of dashes to skip a divider instead of selecting it, and
// menuRows uses this name so the view's divider and the keys' divider are the
// same marker.
const menuSeparatorPrefix = "───"

// menuSeparator is the divider row a menu uses to group its choices. It is a
// frame-width rule rather than the fixed 13-glyph string it used to be, so the
// divider fits the terminal it is drawn in and matches every other rule in the
// TUI. The views detect it by its dashes and it is rendered by rule(), so the
// data and the renderer cannot disagree on its length.
func (m Model) menuSeparator() string {
	return ruleText(contentWidth(m))
}

// GetCurrentOptions returns the options for the current screen
// alacrittyNeedsBuild reports whether the Linux path installs Alacritty from
// source, which is the one terminal choice with a cost worth stating beside the
// menu instead of inside its label. The condition is the one the installer step
// uses: a Debian-based host, or plain Linux, with Linux chosen as the platform.
func (m Model) alacrittyNeedsBuild() bool {
	return m.SystemInfo != nil &&
		(m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux) &&
		m.Choices.OS == "linux"
}

func (m Model) GetCurrentOptions() []string {
	switch m.Screen {
	case ScreenMainMenu:
		// The rows carry words, never an emoji: a terminal without an emoji font
		// draws one as a box, which is why the title lost its toolbox glyph. The
		// rule is the trainer's own, and the emoji guard holds every screen to it.
		opts := []string{
			"Start Installation",
			"Learn About Tools",
			"Keymaps Reference",
			"LazyVim Guide",
			"Vim Trainer",
		}
		// The update row is offered only when a later release is published and this
		// file is the installer's own to replace: the button acts, so a run that has
		// nothing to install shows no row rather than one that opens onto nothing.
		// It sits above Restore so the action rows stay together, and the cursor is
		// held by its label when it arrives late -- a background check lands after
		// the first frame, and an index-based cursor would otherwise hand Enter to
		// whichever option the insertion pushed under it.
		if m.UpdateInstallable() {
			opts = append(opts, updateInstallerRow)
		}
		// Add restore option if backups exist
		if len(m.AvailableBackups) > 0 {
			opts = append(opts, "Restore from Backup")
		}
		// Utilities sits immediately above Exit so Exit stays the last row: the
		// section is a visible destination like the others, and quitting keeps
		// the place muscle memory puts it.
		opts = append(opts, "Utilities")
		opts = append(opts, "Exit")
		return opts
	case ScreenKeymapsMenu:
		return []string{"Neovim", "Tmux", "Zellij", "Herdr", "Ghostty", m.menuSeparator(), "← Back"}
	case ScreenOSSelect:
		// The detected platform is already named on the line above the menu, so
		// the option labels do not repeat it: they were "Linux (detected)" and
		// friends, which restated the description under the title.
		return []string{"macOS", "Linux", "Termux"}
	case ScreenTerminalSelect:
		if m.Choices.OS == "mac" {
			return []string{"Alacritty", "WezTerm", "Kitty", "Ghostty", "None", m.menuSeparator(), "Learn about terminals"}
		}
		return []string{"Alacritty", "WezTerm", "Ghostty", "None", m.menuSeparator(), "Learn about terminals"}
	case ScreenFontSelect:
		return []string{"Yes, install Iosevka Term Nerd Font", "No, I already have it"}
	case ScreenShellSelect:
		return []string{"Fish", "Zsh", "Nushell", m.menuSeparator(), "Learn about shells"}
	case ScreenWMSelect:
		if m.SystemInfo != nil && m.SystemInfo.IsTermux {
			return []string{"Tmux", "Zellij", "None", m.menuSeparator(), "Learn about multiplexers"}
		}
		return []string{"Tmux", "Zellij", "Herdr", "None", m.menuSeparator(), "Learn about multiplexers"}
	case ScreenNvimSelect:
		return []string{"Yes, install Neovim with config", "No, skip Neovim", m.menuSeparator(), "Learn about Neovim", "View Keymaps", "LazyVim Guide"}
	case ScreenUtilities:
		// Only what exists is offered: the switch rows need a detected desktop and
		// the undo row needs a record this installer wrote for that same desktop.
		// A host with none of them gets the explanation in the screen's own body
		// and the way back, not a row that fails when it is pressed.
		opts := []string{}
		if m.ThemeSwitchFound {
			opts = append(opts, "Switch to the dark theme", "Switch to the light theme")
		}
		if m.themeUndoAvailable() {
			opts = append(opts, "Undo the last theme change")
		}
		if len(m.dotfilesThemeOptions()) > 0 || m.DotfilesThemeRecord != nil {
			if len(opts) > 0 {
				opts = append(opts, m.menuSeparator())
			}
			opts = append(opts, utilitiesThemeRow)
		}
		// The WSL resources are offered only where there is a .wslconfig to edit.
		// Everywhere else the section's own body names the reason, so the gap is
		// declared rather than left for the user to guess at.
		if m.WSLState.Available {
			if len(opts) > 0 {
				opts = append(opts, m.menuSeparator())
			}
			opts = append(opts, utilitiesWSLRow)
		}
		// The shell's startup is offered wherever the login shell and a terminal
		// were both resolved. It is the utility that changes nothing, so it is
		// offered rather than gated: reading a number is always safe.
		if m.ShellAudit.Available {
			if len(opts) > 0 {
				opts = append(opts, m.menuSeparator())
			}
			opts = append(opts, utilitiesShellAuditRow)
		}
		// The terminal capability report is read-only and offered everywhere: there
		// is always a terminal to describe, and where there is not one the screen's
		// own body says so rather than the row being silently absent. It sits last
		// so it never displaces the utilities above it on a short frame.
		if len(opts) > 0 {
			opts = append(opts, m.menuSeparator())
		}
		opts = append(opts, utilitiesTerminalRow)
		if len(opts) > 0 {
			opts = append(opts, m.menuSeparator())
		}
		return append(opts, "← Back")
	case ScreenUpdate:
		return m.updateRows()
	case ScreenTerminalCapabilities:
		// The report is read-only: there is nothing to select, only the way back.
		return []string{"← Back"}
	case ScreenShellAudit:
		return m.shellAuditRows()
	case ScreenWSLResources:
		return m.wslResourceRows()
	case ScreenThemePicker:
		// The list of themes lives here, one level in from the utilities section,
		// and it is derived, never typed: one row per complete definition, in the
		// definitions' order. The refresh and the undo are grouped under one
		// separator, and the way back is always the last row.
		if m.ThemeRefreshReview {
			if m.ThemeRefreshDone {
				return []string{"← Back"}
			}
			return []string{themeRefreshConfirmRow(m.ThemeRefreshCandidates), themeRefreshCancelRow}
		}
		opts := m.dotfilesThemeOptions()
		var actions []string
		if len(m.DotfilesThemes) > 0 {
			actions = append(actions, themeRefreshRow)
		}
		if m.DotfilesThemeRecord != nil {
			actions = append(actions, dotfilesThemeUndoRow)
		}
		if len(actions) > 0 {
			if len(opts) > 0 {
				opts = append(opts, m.menuSeparator())
			}
			opts = append(opts, actions...)
		}
		if len(opts) > 0 {
			opts = append(opts, m.menuSeparator())
		}
		return append(opts, "← Back")
	case ScreenBackupConfirm:
		return []string{
			"Install with Backup (recommended)",
			"Install without Backup",
			"Cancel",
		}
	case ScreenRestoreBackup:
		opts := make([]string, len(m.AvailableBackups)+2)
		for i, backup := range m.AvailableBackups {
			// Format: timestamp + file count
			opts[i] = fmt.Sprintf("%s (%d items)", backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
		}
		opts[len(m.AvailableBackups)] = m.menuSeparator()
		opts[len(m.AvailableBackups)+1] = "← Back"
		return opts
	case ScreenRestoreConfirm:
		return []string{
			"Yes, restore this backup",
			"Delete this backup",
			"Cancel",
		}
	case ScreenGhosttyWarning:
		return []string{
			"Continue with Ghostty anyway",
			"Choose a different terminal",
			"Cancel installation",
		}
	case ScreenLearnTerminals:
		return []string{"Alacritty", "WezTerm", "Kitty", "Ghostty", m.menuSeparator(), "← Back"}
	case ScreenLearnShells:
		return []string{"Fish", "Zsh", "Nushell", m.menuSeparator(), "← Back"}
	case ScreenLearnWM:
		return []string{"Tmux", "Zellij", "Herdr", m.menuSeparator(), "← Back"}
	case ScreenLearnNvim:
		return []string{"View Features", "View Keymaps", "LazyVim Guide", m.menuSeparator(), "← Back"}
	case ScreenKeymaps:
		categories := make([]string, len(m.KeymapCategories)+2)
		for i, cat := range m.KeymapCategories {
			categories[i] = cat.Name
		}
		categories[len(m.KeymapCategories)] = m.menuSeparator()
		categories[len(m.KeymapCategories)+1] = "← Back"
		return categories
	case ScreenKeymapsTmux:
		categories := make([]string, len(m.TmuxKeymapCategories)+2)
		for i, cat := range m.TmuxKeymapCategories {
			categories[i] = cat.Name
		}
		categories[len(m.TmuxKeymapCategories)] = m.menuSeparator()
		categories[len(m.TmuxKeymapCategories)+1] = "← Back"
		return categories
	case ScreenKeymapsZellij:
		categories := make([]string, len(m.ZellijKeymapCategories)+2)
		for i, cat := range m.ZellijKeymapCategories {
			categories[i] = cat.Name
		}
		categories[len(m.ZellijKeymapCategories)] = m.menuSeparator()
		categories[len(m.ZellijKeymapCategories)+1] = "← Back"
		return categories
	case ScreenKeymapsGhostty:
		categories := make([]string, len(m.GhosttyKeymapCategories)+2)
		for i, cat := range m.GhosttyKeymapCategories {
			categories[i] = cat.Name
		}
		categories[len(m.GhosttyKeymapCategories)] = m.menuSeparator()
		categories[len(m.GhosttyKeymapCategories)+1] = "← Back"
		return categories
	case ScreenKeymapsHerdr:
		categories := make([]string, len(m.HerdrKeymapCategories)+2)
		for i, cat := range m.HerdrKeymapCategories {
			categories[i] = cat.Name
		}
		categories[len(m.HerdrKeymapCategories)] = m.menuSeparator()
		categories[len(m.HerdrKeymapCategories)+1] = "← Back"
		return categories
	case ScreenLearnLazyVim:
		titles := GetLazyVimTopicTitles()
		result := make([]string, len(titles)+2)
		copy(result, titles)
		result[len(titles)] = m.menuSeparator()
		result[len(titles)+1] = "← Back"
		return result
	default:
		return []string{}
	}
}

// selectedOption is the label the cursor names, or "" when the cursor is out of
// range. The label is a row's stable identity: an index is not, because a row
// that a background read inserts above the cursor shifts every index below it.
func (m Model) selectedOption() string {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return ""
	}
	return options[m.Cursor]
}

// holdCursorOn points the cursor back at the row it was naming after the option
// list changed underneath it, so a row that arrives from a background read
// cannot hand Enter to a different choice. It is a no-op when the row is still
// where it was, and when the row is gone the cursor is clamped into range rather
// than left pointing past the end.
func (m *Model) holdCursorOn(label string) {
	if label == "" {
		return
	}
	options := m.GetCurrentOptions()
	for i, option := range options {
		if option == label {
			m.Cursor = i
			return
		}
	}
	if m.Cursor >= len(options) {
		m.Cursor = len(options) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// GetScreenTitle returns the title for the current screen
func (m Model) GetScreenTitle() string {
	switch m.Screen {
	case ScreenWelcome:
		return "Welcome to dotfiles Installer"
	case ScreenMainMenu:
		return "Main Menu"
	case ScreenOSSelect:
		return "Step 1: Select Your Operating System"
	case ScreenTerminalSelect:
		return "Step 2: Choose Terminal Emulator"
	case ScreenFontSelect:
		return "Step 3: Nerd Font Installation"
	case ScreenShellSelect:
		return "Step 4: Choose Your Shell"
	case ScreenWMSelect:
		return "Step 5: Choose Window Manager"
	case ScreenNvimSelect:
		return "Step 6: Neovim Configuration"
	case ScreenBackupConfirm:
		return "Existing Configs Detected"
	case ScreenRestoreBackup:
		return "Restore from Backup"
	case ScreenRestoreConfirm:
		return "Confirm Restore"
	case ScreenGhosttyWarning:
		return "Ghostty Compatibility Warning"
	case ScreenInstalling:
		return "Installing dotfiles"
	case ScreenComplete:
		return "Installation complete"
	case ScreenError:
		return "Installation failed"
	case ScreenLearnTerminals:
		return "Learn: Terminal Emulators"
	case ScreenLearnShells:
		return "Learn: Shells"
	case ScreenLearnWM:
		return "Learn: Window Managers"
	case ScreenLearnNvim:
		return "Learn: Neovim"
	case ScreenKeymaps:
		return "Neovim Keymaps Reference"
	case ScreenKeymapCategory:
		if m.SelectedCategory < len(m.KeymapCategories) {
			return m.KeymapCategories[m.SelectedCategory].Name
		}
		return "Keymaps"
	case ScreenKeymapsMenu:
		return "Keymaps Reference"
	case ScreenKeymapsTmux:
		return "Tmux Keymaps"
	case ScreenKeymapsTmuxCat:
		if m.TmuxSelectedCategory < len(m.TmuxKeymapCategories) {
			return m.TmuxKeymapCategories[m.TmuxSelectedCategory].Name
		}
		return "Tmux Keymaps"
	case ScreenKeymapsZellij:
		return "Zellij Keymaps"
	case ScreenKeymapsZellijCat:
		if m.ZellijSelectedCategory < len(m.ZellijKeymapCategories) {
			return m.ZellijKeymapCategories[m.ZellijSelectedCategory].Name
		}
		return "Zellij Keymaps"
	case ScreenKeymapsGhostty:
		return "Ghostty Keymaps"
	case ScreenKeymapsGhosttyCat:
		if m.GhosttySelectedCategory < len(m.GhosttyKeymapCategories) {
			return m.GhosttyKeymapCategories[m.GhosttySelectedCategory].Name
		}
		return "Ghostty Keymaps"
	case ScreenKeymapsHerdr:
		return "Herdr Keymaps"
	case ScreenKeymapsHerdrCat:
		if m.HerdrSelectedCategory < len(m.HerdrKeymapCategories) {
			return m.HerdrKeymapCategories[m.HerdrSelectedCategory].Name
		}
		return "Herdr Keymaps"
	case ScreenLearnLazyVim:
		return "LazyVim Guide"
	case ScreenLazyVimTopic:
		if m.SelectedLazyVimTopic < len(m.LazyVimTopics) {
			return m.LazyVimTopics[m.SelectedLazyVimTopic].Title
		}
		return "LazyVim"
	// The trainer composes its own header, so these titles are never drawn; they
	// follow the same rule as the rest rather than keeping the emoji the header
	// does not use.
	case ScreenTrainerMenu:
		return "Vim Trainer - Module Selection"
	case ScreenTrainerLesson:
		return "Vim Trainer - Lesson"
	case ScreenTrainerPractice:
		return "Vim Trainer - Practice"
	case ScreenTrainerBoss:
		return "Vim Trainer - Boss Fight!"
	case ScreenTrainerResult:
		return "Vim Trainer - Result"
	case ScreenTrainerBossResult:
		return "Vim Trainer - Boss Battle Complete"
	case ScreenUtilities:
		return "Utilities"
	case ScreenThemePicker:
		return "Change the dotfiles theme"
	case ScreenWSLResources:
		return "WSL resources"
	case ScreenTerminalCapabilities:
		return "Terminal capabilities"
	case ScreenShellAudit:
		return "The shell's startup"
	case ScreenUpdate:
		return "Updates"
	default:
		return ""
	}
}

// GetScreenDescription returns a description for the current screen
func (m Model) GetScreenDescription() string {
	switch m.Screen {
	case ScreenOSSelect:
		// A platform the mapping does not know has no menu entry to sit on, so
		// the screen says detection found nothing instead of printing an
		// "unknown" under a highlighted macOS.
		if m.SystemInfo == nil {
			return "Platform not detected — select your operating system"
		}
		if _, ok := osOptionIndex(m.SystemInfo.OS); !ok {
			return "Platform not detected — select your operating system"
		}
		detected := m.SystemInfo.OSName
		if m.SystemInfo.IsWSL && m.SystemInfo.OSName != "WSL" {
			detected += " (WSL)"
		}
		return "Detected: " + detected
	case ScreenTerminalSelect:
		if m.SystemInfo.IsWSL {
			return "WSL detected: terminal emulators should be installed on Windows.\nThe installer will skip terminal setup — use Windows Terminal or your preferred Windows terminal."
		}
		if m.alacrittyNeedsBuild() {
			// The build cost used to sit inside the Alacritty menu label, which
			// made a warning read like part of the terminal's name. It is a
			// sentence beside the menu now, where the reader can weigh it before
			// picking.
			return "Select your preferred terminal emulator.\nAlacritty builds from source on this system, so it needs Rust and about 5–10 minutes."
		}
		return "Select your preferred terminal emulator"
	case ScreenFontSelect:
		return "Iosevka Term Nerd Font is required for icons and glyphs"
	case ScreenShellSelect:
		// The line names the detected shell and says what the cursor is doing: it
		// starts on the detected shell when the menu lists it. The old line stated
		// the current shell and left the cursor on Fish, so the two contradicted
		// each other whenever the account did not already use Fish. When nothing
		// was detected the line says so rather than leaving the cursor on Fish.
		if m.SystemInfo == nil || m.SystemInfo.UserShell == "" || m.SystemInfo.UserShell == "unknown" {
			return "No current shell detected — choose the shell you want"
		}
		return "Current shell: " + m.SystemInfo.UserShell + " — the cursor starts on it when the menu lists it"
	case ScreenWMSelect:
		return "Terminal multiplexer for managing sessions"
	case ScreenNvimSelect:
		return "Includes LSP, TreeSitter, and dotfiles config"
	case ScreenGhosttyWarning:
		return "Ghostty installation may fail on Ubuntu/Debian.\nThe installer script only supports certain versions."
	default:
		return ""
	}
}

// planOptions is every fact the install plan is built from. The plan is a pure
// function of these, so the wizard and the main menu's panel can both ask for it
// without either owning the answer: SetupInstallSteps fills the struct from the
// model it is about to run, and the panel fills it from the model it is showing.
type planOptions struct {
	// OS is the choice the wizard recorded ("mac", "linux" or "termux"), or ""
	// while the question is still open.
	OS string
	// DetectedOS is the distribution family detection found. The Homebrew
	// decision reads it: Arch and Fedora install through their own package
	// manager instead.
	DetectedOS system.OSType

	HasBrew  bool
	HasXcode bool
	IsWSL    bool
	// DetectedTermux is what the host is, and it is separate from OS because the
	// two questions differ: the dependencies step treats a chosen termux like a
	// detected one, while the Homebrew step only skips a host detection actually
	// recognised as Termux.
	DetectedTermux bool

	// ConfigCount is how many existing configs the run would overwrite, and
	// CreateBackup is whether the player asked for the backup step that saves
	// them. The step exists only when both are set.
	ConfigCount  int
	CreateBackup bool

	Terminal    string
	InstallFont bool
	Shell       string
	WindowMgr   string
	InstallNvim bool
}

// planOptions reads the facts the plan is built from out of the model. It only
// reads: nothing here records progress, scans the machine or starts a step.
func (m Model) planOptions() planOptions {
	opts := planOptions{
		OS:           m.Choices.OS,
		CreateBackup: m.Choices.CreateBackup,
		ConfigCount:  len(m.ExistingConfigs),
		Terminal:     m.Choices.Terminal,
		InstallFont:  m.Choices.InstallFont,
		Shell:        m.Choices.Shell,
		WindowMgr:    m.Choices.WindowMgr,
		InstallNvim:  m.Choices.InstallNvim,
	}
	if m.SystemInfo != nil {
		opts.DetectedOS = m.SystemInfo.OS
		opts.HasBrew = m.SystemInfo.HasBrew
		opts.HasXcode = m.SystemInfo.HasXcode
		opts.IsWSL = m.SystemInfo.IsWSL
		opts.DetectedTermux = m.SystemInfo.IsTermux
	}
	return opts
}

// planFor builds the install plan. It is pure: the same options always produce
// the same steps, and it touches no model and no machine. That is what lets the
// main menu's panel show the plan the wizard is about to run instead of a
// second, hand-written copy of it that can drift.
func planFor(opts planOptions) []InstallStep {
	steps := []InstallStep{}

	// Backup step if user chose to backup (not interactive - just file copies)
	if opts.CreateBackup && opts.ConfigCount > 0 {
		steps = append(steps, InstallStep{
			ID:          "backup",
			Name:        "Backup Existing Configs",
			Description: "Saves a copy of your current configuration first.",
			Status:      StatusPending,
		})
	}

	// Dependencies based on OS
	// Check both the OS choice and the detected host for Termux (redundancy)
	// Must run BEFORE clone and homebrew on Linux so git is available for clone
	isTermux := opts.OS == "termux" || opts.DetectedTermux
	if opts.OS == "linux" && !isTermux {
		steps = append(steps, InstallStep{
			ID:          "deps",
			Name:        "Install Dependencies",
			Description: "Installs base packages with your distribution's package manager.",
			Status:      StatusPending,
			Interactive: true, // Needs sudo
		})
	} else if isTermux {
		steps = append(steps, InstallStep{
			ID:          "deps",
			Name:        "Install Dependencies",
			Description: "Installs base packages with pkg.",
			Status:      StatusPending,
			Interactive: false, // Termux doesn't need sudo
		})
	} else if opts.OS == "mac" && !opts.HasXcode {
		steps = append(steps, InstallStep{
			ID:          "xcode",
			Name:        "Install Xcode CLI",
			Description: "Installs the Apple developer command-line tools.",
			Status:      StatusPending,
		})
	}

	// Clone repo (after deps so git is available on fresh Linux installs)
	steps = append(steps, InstallStep{
		ID:          "clone",
		Name:        "Clone Repository",
		Description: "Downloads your dotfiles repository.",
		Status:      StatusPending,
	})

	// Homebrew (interactive - first install needs password)
	// Skip Termux and native package manager Linux distributions.
	// WSL systems use Homebrew (Debian-based approach).
	if !opts.HasBrew && !opts.DetectedTermux && opts.DetectedOS != system.OSArch && opts.DetectedOS != system.OSFedora {
		steps = append(steps, InstallStep{
			ID:          "homebrew",
			Name:        "Install Homebrew",
			Description: "Installs Homebrew, the package manager.",
			Status:      StatusPending,
			Interactive: true,
		})
	}

	// Terminal
	if opts.Terminal != "none" && opts.Terminal != "" {
		steps = append(steps, InstallStep{
			ID:          "terminal",
			Name:        "Install " + opts.Terminal,
			Description: "Installs your terminal emulator.",
			Status:      StatusPending,
			Interactive: opts.OS == "linux", // Linux needs sudo for pacman/apt
		})
	}

	// Font (not interactive - brew doesn't need password after installed)
	if opts.InstallFont {
		steps = append(steps, InstallStep{
			ID:          "font",
			Name:        "Install Iosevka Nerd Font",
			Description: "Installs the Iosevka Nerd Font for icons.",
			Status:      StatusPending,
		})
	}

	// Shell installation runs through executeStep. Native Linux package managers use sudo there.
	// The shell is the one choice with no "skip", so in the wizard it is always set
	// and the step is always planned. The main menu's preview runs before the
	// question is asked, so it has no name to give the step and leaves it out
	// rather than showing "Install " -- the same rule the terminal and the
	// multiplexer already follow for an unmade choice.
	if opts.Shell != "" {
		steps = append(steps, InstallStep{
			ID:          "shell",
			Name:        "Install " + opts.Shell,
			Description: "Installs your shell and its plugins.",
			Status:      StatusPending,
		})
	}

	// Window manager installation runs through executeStep. Herdr downloads to ~/.local/bin on non-Homebrew Linux.
	if opts.WindowMgr != "none" && opts.WindowMgr != "" {
		steps = append(steps, InstallStep{
			ID:          "wm",
			Name:        "Install " + opts.WindowMgr,
			Description: "Installs your terminal multiplexer.",
			Status:      StatusPending,
		})
	}

	// Neovim installation runs through executeStep. Native Linux package managers use sudo there.
	if opts.InstallNvim {
		steps = append(steps, InstallStep{
			ID:          "nvim",
			Name:        "Install Neovim",
			Description: "Installs Neovim with your configuration.",
			Status:      StatusPending,
		})
	}

	// Toolset runs after the shell step because the Brewfile's npm entries need
	// the Node runtime fnm provides, and after the Homebrew step because it needs
	// brew. It is best-effort and never fails the run, so it stays
	// non-interactive: brew bundle runs unattended.
	steps = append(steps, InstallStep{
		ID:          "toolset",
		Name:        "Install Toolset",
		Description: "Installs the command-line tools listed in your Brewfile.",
		Status:      StatusPending,
	})

	// Pi agent skills. Pinned, checksum-verified packages installed under
	// ~/.pi/agent/skills. It needs no sudo, so it runs through executeStep.
	steps = append(steps, InstallStep{
		ID:          "agentskills",
		Name:        "Install Pi Agent Skills",
		Description: "Installs the pinned security-audit, archify and officecli skills.",
		Status:      StatusPending,
	})

	// OfficeCLI binary. A pinned, checksum-verified release asset installed to
	// ~/.local/bin/officecli. It needs no sudo, so it runs through executeStep.
	steps = append(steps, InstallStep{
		ID:          "officecli",
		Name:        "Install OfficeCLI",
		Description: "Installs the pinned, checksum-verified OfficeCLI binary.",
		Status:      StatusPending,
	})

	// WSL configuration (Windows host + in-distribution settings). The files are
	// only read when the WSL VM restarts, so this runs late in the sequence.
	if opts.IsWSL {
		steps = append(steps, InstallStep{
			ID:          "wslconfig",
			Name:        "Configure WSL",
			Description: "Applies the WSL settings on Windows and in this distribution.",
			Status:      StatusPending,
			Interactive: true, // /etc/wsl.conf needs sudo
		})
	}

	// Set default shell (interactive - chsh needs password)
	steps = append(steps, InstallStep{
		ID:          "setshell",
		Name:        "Set Default Shell",
		Description: "Sets your shell as the default.",
		Status:      StatusPending,
		Interactive: true,
	})

	// Cleanup (not interactive - just file deletion)
	steps = append(steps, InstallStep{
		ID:          "cleanup",
		Name:        "Cleanup",
		Description: "Removes the temporary files it created.",
		Status:      StatusPending,
	})

	return steps
}

// SetupInstallSteps records the plan the run will execute, from the choices the
// wizard has collected and the host detection filled. It is a thin wrapper over
// planFor so the wizard and the main menu's panel read the same source.
func (m *Model) SetupInstallSteps() {
	m.Steps = planFor(m.planOptions())
}

// scrollTrainerCode moves the code window one row, clamped to the code it is
// shown over. The window starts where the exercise's own cursor is (see
// trainerCodeAnchor), so the first press of a scroll key starts from there; both
// bounds come from the same window size the renderer uses, which is why a press
// at either end does nothing instead of banking presses the player then has to
// press back.
func (m *Model) scrollTrainerCode(down bool) {
	exercise := m.trainerCurrentExercise()
	if exercise == nil {
		return
	}
	_, codeRows := m.trainerTextBudget(exercise)

	offset := m.TrainerCodeScroll
	if !m.trainerCodeScrolled(exercise) {
		offset = trainerCodeAnchor(exercise.CursorPos.Line, codeRows, len(exercise.Code))
	}
	if down {
		offset++
	} else {
		offset--
	}

	m.TrainerCodeScroll = trainerCodeOffset(offset, codeRows, len(exercise.Code))
	m.TrainerCodeScrollFor = exercise.ID
}

// trainerCodeScrolled reports whether the player has moved the code window for
// this exercise. An exercise with no ID never counts as scrolled: there would be
// nothing to tell one presentation of it from the next, so its window stays at
// the entry position.
func (m Model) trainerCodeScrolled(exercise *trainer.Exercise) bool {
	return exercise.ID != "" && m.TrainerCodeScrollFor == exercise.ID
}
