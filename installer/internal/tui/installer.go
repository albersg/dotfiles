package tui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// StepError provides context about which step failed and why
type StepError struct {
	StepID      string
	StepName    string
	Description string
	Cause       error
}

func (e *StepError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Step '%s' failed\n", e.StepName))
	sb.WriteString(fmt.Sprintf("Description: %s\n", e.Description))
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf("\nDetails:\n%v", e.Cause))
	}
	return sb.String()
}

func (e *StepError) Unwrap() error {
	return e.Cause
}

// wrapStepError creates a detailed error for a step failure
func wrapStepError(stepID, stepName, description string, cause error) error {
	return &StepError{
		StepID:      stepID,
		StepName:    stepName,
		Description: description,
		Cause:       cause,
	}
}

// dryRun reports whether this run was started with --dry-run. The flag used to
// be advertised in --help but never read, so a "dry run" performed a real
// installation.
func dryRun() bool {
	switch os.Getenv("DOTFILES_DRY_RUN") {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// ============================================================================
// THE UTILITIES SECTION: THE SYSTEM THEME SWITCH
// ============================================================================
//
// The utilities section's first utility changes the desktop's light/dark theme
// through the desktop's own tool. It is deliberately narrow: it supports the
// desktops whose theme can be read back exactly as it can be written, because a
// switch this installer cannot put back is a switch that destroys a setting the
// user chose.
//
// Two things are written, and the copy says so:
//
//   - the desktop's own setting, through the desktop's own tool (GNOME's
//     color-scheme through gsettings, Plasma's colour scheme through
//     plasma-apply-colorscheme, macOS's global appearance preference through
//     defaults). The installer does not own that store and never edits it
//     itself.
//   - the value that was there before the change, in the installer's own state
//     file, theme.json beside last-install.json. That file is what makes the
//     change undoable, and it is the only file the installer writes for this.
//
// Nothing here runs while a screen draws: the detected switch is decided when
// the model is built and the record is read on the startup path, and the switch
// itself runs inside a tea.Cmd. Both entry points are gated on dryRun() exactly
// as executeStep is.

// themeSwitch is one desktop's reversible light/dark switch: the command that
// reads the setting that is there, the two commands that set it, how the read
// output is understood, and how a recorded value is written back. It is data
// rather than code so detection can name the switch it found and a test can
// enumerate the whole table.
//
// Interpret is what keeps the utility safe. It turns the read command's output
// into the value to put back, whether that value means the dark theme, and
// whether it is a value this installer can carry to a shell at all. When it
// reports false the utility refuses: it will not change a setting it could not
// restore.
type themeSwitch struct {
	ID    string
	Name  string
	Read  string
	Dark  string
	Light string
	// Writes names the desktop's own store the switch changes, so the section can
	// tell the reader what it touches instead of saying "your theme".
	Writes    string
	Interpret func(output string, readFailed bool) (value string, dark bool, ok bool)
	Restore   func(value string) string
}

// themeSwitches is the desktops the utilities section can switch, in the order
// detection tries them. A desktop is only in the table when its setting can be
// read back in full: a writer without a reader cannot honour the undo.
var themeSwitches = []themeSwitch{
	{
		ID:    "gnome",
		Name:  "GNOME",
		Read:  "gsettings get org.gnome.desktop.interface color-scheme",
		Dark:  "gsettings set org.gnome.desktop.interface color-scheme prefer-dark",
		Light: "gsettings set org.gnome.desktop.interface color-scheme default",
		// The GNOME theme preference is one key in the dconf database, which is
		// why this is the desktop the switch supports first.
		Writes: "the GNOME color-scheme preference, which gsettings stores in the dconf database (~/.config/dconf/user)",
		Interpret: func(output string, readFailed bool) (string, bool, bool) {
			value := bareThemeValue(output)
			if readFailed || !safeThemeValue(value) {
				return "", false, false
			}
			return value, strings.Contains(strings.ToLower(value), "dark"), true
		},
		Restore: func(value string) string {
			return "gsettings set org.gnome.desktop.interface color-scheme " + quoteThemeValue(value)
		},
	},
	{
		ID:     "kde",
		Name:   "KDE Plasma",
		Read:   "kreadconfig6 --file kdeglobals --group General --key ColorScheme",
		Dark:   "plasma-apply-colorscheme BreezeDark",
		Light:  "plasma-apply-colorscheme BreezeLight",
		Writes: "the Plasma colour scheme, which plasma-apply-colorscheme writes to ~/.config/kdeglobals",
		Interpret: func(output string, readFailed bool) (string, bool, bool) {
			// A Plasma user's scheme is often a custom one with a space in its
			// name, so a value with spaces is carried rather than refused; the
			// restore command quotes it.
			value := strings.TrimSpace(output)
			if readFailed || !safeThemeValue(value) {
				return "", false, false
			}
			return value, strings.Contains(strings.ToLower(value), "dark"), true
		},
		Restore: func(value string) string {
			return "plasma-apply-colorscheme " + quoteThemeValue(value)
		},
	},
	{
		ID:    "macos",
		Name:  "macOS",
		Read:  "defaults read -g AppleInterfaceStyle",
		Dark:  "defaults write -g AppleInterfaceStyle Dark",
		Light: "defaults delete -g AppleInterfaceStyle",
		// macOS stores the appearance preference in the global preferences file.
		// A deleted key is the default (light), and the read command fails when it
		// is absent, which is the reading this interprets as light.
		Writes: "the global appearance preference, which defaults stores in ~/Library/Preferences/.GlobalPreferences.plist",
		Interpret: func(output string, readFailed bool) (string, bool, bool) {
			value := strings.TrimSpace(output)
			if readFailed {
				// No key is the light theme, which is a state this can restore with
				// `defaults delete`.
				return "", false, true
			}
			if !safeThemeValue(value) {
				return "", false, false
			}
			return value, strings.Contains(strings.ToLower(value), "dark"), true
		},
		Restore: func(value string) string {
			if value == "" {
				return "defaults delete -g AppleInterfaceStyle"
			}
			return "defaults write -g AppleInterfaceStyle " + quoteThemeValue(value)
		},
	},
}

// themeSwitchByID finds a switch by the id the record stores, so the undo can
// refuse a record written by a desktop this host is not on.
func themeSwitchByID(id string) (themeSwitch, bool) {
	for _, target := range themeSwitches {
		if target.ID == id {
			return target, true
		}
	}
	return themeSwitch{}, false
}

// currentThemeSwitch detects the switch this host offers. It is the wrapper that
// knows what the host is; detectThemeSwitch below is the pure rule, so the rule
// can be tested without an environment. Termux and an undetected host are
// refused here rather than by the rule, because only SystemInfo knows them.
func currentThemeSwitch(info *system.SystemInfo) (themeSwitch, bool) {
	if info == nil || info.IsTermux {
		return themeSwitch{}, false
	}
	return detectThemeSwitch(runtime.GOOS,
		os.Getenv("XDG_CURRENT_DESKTOP"), os.Getenv("DESKTOP_SESSION"),
		system.CommandExists)
}

// detectThemeSwitch is the detection rule: the session has to name a desktop
// this table supports, and the tools that make that desktop's change both
// possible and undoable have to be on PATH. Plasma needs its reader as well as
// its writer, which is why kreadconfig6 is required with
// plasma-apply-colorscheme and not treated as a nicety.
func detectThemeSwitch(goos, desktop, session string, has func(string) bool) (themeSwitch, bool) {
	if goos == "darwin" && has("defaults") {
		return mustThemeSwitch("macos")
	}
	named := strings.ToLower(desktop + " " + session)
	switch {
	case has("gsettings") && strings.Contains(named, "gnome"):
		return mustThemeSwitch("gnome")
	case has("plasma-apply-colorscheme") && has("kreadconfig6") && strings.Contains(named, "kde"):
		return mustThemeSwitch("kde")
	}
	return themeSwitch{}, false
}

// mustThemeSwitch looks an entry up by the id the rule names. The table is a
// constant, so a missing entry is a programming error rather than a host's
// answer; the panic keeps the bound-checked lookup out of detection's signature
// instead of silently returning a switch with no commands.
func mustThemeSwitch(id string) (themeSwitch, bool) {
	target, ok := themeSwitchByID(id)
	if !ok {
		panic("theme switch table has no entry " + id)
	}
	return target, true
}

// bareThemeValue strips the single pair of quotes gsettings prints around its
// GVariant strings, so the value the record holds is the setting and not the
// tool's own punctuation. Every other tool prints a bare value, which this
// returns unchanged.
func bareThemeValue(output string) string {
	value := strings.TrimSpace(output)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

// safeThemeValue reports whether a reading is one the restore command can carry
// to a shell as a single argument. Letters, digits, spaces and the punctuation
// real theme and colour-scheme names use are allowed; anything else -- a quote, a
// dollar, a backtick, a semicolon -- means the reading is not understood and the
// utility refuses to change a setting it could not put back.
func safeThemeValue(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ' || r == '.' || r == '_' || r == '-' || r == '+':
		default:
			return false
		}
	}
	return true
}

// quoteThemeValue wraps a reading in single quotes so the restore command
// carries it as one argument. The value has already passed safeThemeValue, so
// there is no quote inside it for this to break on.
func quoteThemeValue(value string) string {
	return "'" + value + "'"
}

// themeRecord is the installer's memory of one theme switch: which desktop it
// was made on, the setting that was there before it, and when it was made. It is
// what the undo reads, so it is written only after the desktop's own tool has
// reported success.
//
// Dotfiles is the dotfiles' own theme switch, recorded in the same file rather
// than a second one: the two switches are independent, so each write keeps the
// other's half.
type themeRecord struct {
	Target    string    `json:"target"`
	Value     string    `json:"value"`
	WasDark   bool      `json:"was_dark"`
	ToDark    bool      `json:"to_dark"`
	AppliedAt time.Time `json:"applied_at"`
	// Dotfiles is the last dotfiles-theme change: the theme applied and the
	// exact bytes each file held before it, which is what makes the change
	// reversible byte-for-byte.
	Dotfiles *dotfilesThemeRecord `json:"dotfiles,omitempty"`
}

// dotfilesThemeRecord is one dotfiles-theme change. Files maps an installed
// file's path to its full previous content, so the undo restores the file
// exactly rather than regenerating it.
type dotfilesThemeRecord struct {
	Theme     string            `json:"theme"`
	Files     map[string]string `json:"files"`
	AppliedAt time.Time         `json:"applied_at"`
}

// themeStateFile is the record's name inside the installer's state directory. It
// sits beside last-install.json, in the directory the XDG base-directory
// specification reserves for state a program keeps between runs.
const themeStateFile = "theme.json"

// themeStatePath is the exact file the installer reads and writes. It is named
// once here so the writer, the reader and the prose cannot disagree.
func themeStatePath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, themeStateFile)
}

// writeThemeRecord records a completed switch. Unlike the last-install record
// this write is not best effort: the record is what makes the change undoable,
// so a switch whose record could not be saved is reported rather than left
// looking reversible when it is not.
func writeThemeRecord(rec themeRecord) error {
	path := themeStatePath()
	if path == "" {
		return fmt.Errorf("could not determine the state directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), stateDirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, stateFileMode)
}

// readThemeFile returns the whole record, desktop half and dotfiles half. A
// missing, unreadable or corrupt file reads as the zero record: a restore must
// never be built from half a file.
func readThemeFile() themeRecord {
	path := themeStatePath()
	if path == "" {
		return themeRecord{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return themeRecord{}
	}
	var rec themeRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return themeRecord{}
	}
	return rec
}

// readThemeRecord returns the setting this installer last replaced on the
// desktop, or nil when there is none.
func readThemeRecord() *themeRecord {
	rec := readThemeFile()
	if rec.Target == "" {
		return nil
	}
	return &rec
}

// readDotfilesThemeRecord returns the last dotfiles-theme change, or nil.
func readDotfilesThemeRecord() *dotfilesThemeRecord {
	return readThemeFile().Dotfiles
}

// writeDotfilesThemeRecord stores a dotfiles-theme change in the one record
// file, keeping the desktop half that is already there.
func writeDotfilesThemeRecord(next *dotfilesThemeRecord) error {
	rec := readThemeFile()
	rec.Dotfiles = next
	return writeThemeRecord(rec)
}

// runThemeCommand is the one seam the theme utility runs a command through. It
// is a variable so a test can drive every branch -- the reading, the switch and
// the restore -- without a desktop, a PATH shim or a real gsettings.
var runThemeCommand = system.Run

// readThemeValue asks the desktop's own tool what the setting is, and interprets
// the answer through the switch's own rule. A value the switch cannot understand
// or cannot safely put back is reported as not ok, and the caller then refuses to
// change anything.
func readThemeValue(target themeSwitch) (value string, dark, ok bool) {
	result := runThemeCommand(target.Read, &system.ExecOptions{Timeout: themeCommandTimeout})
	return target.Interpret(result.Output, result.Error != nil)
}

// themeCommandTimeout bounds each theme command. These tools answer in
// milliseconds; the bound exists so a stuck one cannot hang the screen.
const themeCommandTimeout = 10 * time.Second

// applyTheme switches the desktop's theme and records the setting it replaced.
// It is gated on dryRun() exactly as executeStep is: --dry-run documents a run
// that changes nothing, so the flag has to stop this path too rather than only
// the installation steps.
func applyTheme(target themeSwitch, dark bool) (*themeRecord, string, error) {
	verb := "light"
	command := target.Light
	if dark {
		verb = "dark"
		command = target.Dark
	}
	if dryRun() {
		SendLog("utilities", fmt.Sprintf("DRY RUN: skipping the switch to the %s theme", verb))
		return nil, fmt.Sprintf("DRY RUN: the %s theme was not changed.", verb), nil
	}

	previous, wasDark, ok := readThemeValue(target)
	if !ok {
		return nil, "", fmt.Errorf("could not read the current %s setting, so it was left exactly as it is: "+
			"this installer only changes a theme it can put back", target.Name)
	}

	result := runThemeCommand(command, &system.ExecOptions{Timeout: themeCommandTimeout})
	if result.Error != nil {
		return nil, "", fmt.Errorf("the %s tool failed to switch the theme: %w", target.Name, result.Error)
	}

	rec := readThemeFile()
	rec.Target = target.ID
	rec.Value = previous
	rec.WasDark = wasDark
	rec.ToDark = dark
	rec.AppliedAt = time.Now()
	if err := writeThemeRecord(rec); err != nil {
		return nil, "", fmt.Errorf("the theme changed but the setting it replaced could not be recorded, "+
			"so it cannot be undone: %w", err)
	}
	return &rec, fmt.Sprintf("The %s theme is on. The setting it replaced is recorded; use Undo to put it back.", verb), nil
}

// undoTheme puts the recorded setting back. It reads the current value first, so
// the undo is itself reversible: the record it leaves holds the value the undo
// replaced, which is why the row is named "Undo the last theme change" rather
// than "restore". It is gated on dryRun() for the same reason applyTheme is.
func undoTheme(target themeSwitch, rec themeRecord) (*themeRecord, string, error) {
	if rec.Target != target.ID {
		return nil, "", fmt.Errorf("the recorded change was made on the %s desktop, not on %s", rec.Target, target.Name)
	}
	if dryRun() {
		SendLog("utilities", "DRY RUN: skipping the undo of the last theme change")
		return nil, "DRY RUN: the last theme change was not undone.", nil
	}

	current, currentDark, ok := readThemeValue(target)
	if !ok {
		return nil, "", fmt.Errorf("could not read the current %s setting, so it was left exactly as it is", target.Name)
	}

	command := target.Restore(rec.Value)
	result := runThemeCommand(command, &system.ExecOptions{Timeout: themeCommandTimeout})
	if result.Error != nil {
		return nil, "", fmt.Errorf("the %s tool failed to put the previous setting back: %w", target.Name, result.Error)
	}

	next := readThemeFile()
	next.Target = target.ID
	next.Value = current
	next.WasDark = currentDark
	next.ToDark = rec.WasDark
	next.AppliedAt = time.Now()
	if err := writeThemeRecord(next); err != nil {
		return nil, "", fmt.Errorf("the previous setting was restored but the new record could not be saved: %w", err)
	}
	return &next, "The previous setting is back. Undo again puts the last one back.", nil
}

// themeUndoAvailable reports whether the section may offer the undo row: there
// has to be a detected switch and a record written for that same desktop. An
// old record from another desktop is not offered, because the undo would refuse
// it anyway and a row that fails is worse than no row.
func (m Model) themeUndoAvailable() bool {
	return m.ThemeSwitchFound && m.ThemeRecord != nil && m.ThemeRecord.Target == m.ThemeSwitch.ID
}

// ============================================================================
// THE DOTFILES THEME: ONE DEFINITION, EVERY PART
// ============================================================================
//
// The desktop switch above changes the desktop's light/dark mode. This section
// is the other one: the dotfiles' own theme - the palette this repository ships
// across its terminals, its prompt, bat, fish and Herdr.
//
// The palette used to be hand-written in five files (alacritty.toml,
// .wezterm.lua, dotfiles-kitty/kitty.conf, starship.toml, dotfiles-zsh/.p10k.zsh,
// and again in dotfiles-zsh/.zshrc), so the same colour was maintained by hand
// in six places and any drift between them was invisible until two terminals
// were put side by side. The definition is themes/*.toml now: one file per
// theme, the blocks are generated from it, and the guards in
// install_paths_test.go fail when a shipped block stops matching its
// definition. Adding a theme is adding a definition file; nothing here is
// typed by hand.
//
// No colour is invented. A role either exists in the repository already (the
// six hand-written blocks, the Catppuccin ghostty file, the fish theme files)
// or the role is left empty and the theme is reported partial. See
// themes/README.md for the provenance of every value.

// themesDirName is the directory of theme definitions, relative to the
// repository root the installer clones.
const themesDirName = "themes"

// dotfilesDirEnv is the environment variable that names a repository checkout
// when the installer is not launched from one. It is a new interface: before
// this, only the clone the installer itself created could supply the theme
// definitions, so a user who keeps the checkout anywhere else could never see
// them. $DOTFILES_DIR answers first, before any guess, and the checkout's
// themes/*.toml is read from disk - the definitions are never packaged into the
// binary.
const dotfilesDirEnv = "DOTFILES_DIR"

// themeDefinitionDirs returns the repository roots that are tried for
// themes/*.toml, in the order they win. The order is the contract:
//
//  1. $DOTFILES_DIR, when it is set: a user naming a checkout is answered
//     before any guess.
//  2. the clone this run made (m.RepoDir), when it exists.
//  3. the working directory and every parent that holds themes/*.toml, nearest
//     first: launching the installer from inside the checkout is the normal
//     case, and the checkout may be several levels above the working directory.
//  4. ~/dotfiles, then ~/.dotfiles: the conventional locations.
//
// A candidate without themes/*.toml is skipped; the first one that has it wins
// and no later candidate is read. The order is documented in
// docs/tui-installer.md, because a search order that is not written down is
// magic.
func themeDefinitionDirs(repoDir string) []string {
	var dirs []string
	if env := strings.TrimSpace(os.Getenv(dotfilesDirEnv)); env != "" {
		dirs = append(dirs, env)
	}
	if repoDir != "" {
		dirs = append(dirs, repoDir)
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, themeDirsFromWorkdir(cwd)...)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "dotfiles"), filepath.Join(home, ".dotfiles"))
	}
	return dirs
}

// dotfilesDataDir is the single per-user data root every runtime asset the
// installer copies is installed under: $XDG_DATA_HOME/dotfiles, falling back to
// ~/.local/share/dotfiles. It is the directory the specification reserves for
// data a program needs to run, as opposed to the state directory the records
// live in. It is defined once so the two assets installed here -- the theme
// definitions and the WSL template -- cannot drift into two different trees:
// each keeps its repository-relative shape inside this root (themes/ and
// dotfiles-wsl/), so a copy is shaped exactly like a checkout and the readers
// treat the two the same way. An empty string means neither location could be
// determined, and no copy is then made or searched.
func dotfilesDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
		return filepath.Join(dir, stateAppDir)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", stateAppDir)
}

// themeDefinitionsDataDir is the per-user directory the installer copies the
// theme definitions into, so the theme switch is offered from any working
// directory and after the temporary clone this run made is cleaned up. It is
// dotfilesDataDir, the shared root the installer's runtime assets live under, so
// it lands on the same tree as the WSL template the clone step also installs;
// the definitions sit under themes/ inside it, the same shape a checkout has, so
// the reader treats a copy and a checkout the same way.
func themeDefinitionsDataDir() string {
	return dotfilesDataDir()
}

// themeDefinitionSearchDirs is the full candidate list the theme resolver
// walks: the repository roots themeDefinitionDirs names, then the per-user data
// directory the installer copied the definitions into. The copy is the last
// candidate on purpose, so a checkout the user named, cloned or is standing in
// is always answered first and a copy is read only when no checkout is present.
// Resolution takes the first candidate that holds themes/*.toml and never
// merges two of them, so a stale copy cannot mix half of one checkout's list
// into another's.
func themeDefinitionSearchDirs(repoDir string) []string {
	dirs := themeDefinitionDirs(repoDir)
	if dataDir := themeDefinitionsDataDir(); dataDir != "" {
		dirs = append(dirs, dataDir)
	}
	return dirs
}

// themeDefinitionsNotFoundMessage names every candidate the resolver walked, for
// the one honest failure the utilities section draws when none of them holds
// themes/*.toml. It is built from the same candidates resolveThemeDefinitionsDir
// walks, so a directory added to the search cannot go unmentioned here.
func themeDefinitionsNotFoundMessage() string {
	where := "$" + dotfilesDirEnv + ", the clone, the working directory or its parents, ~/dotfiles and ~/.dotfiles"
	if dataDir := themeDefinitionsDataDir(); dataDir != "" {
		where += ", and the installer's own copy under " + dataDir
	}
	return "no repository holding theme definitions was found in " + where
}

// themeDirsFromWorkdir walks from dir up to the filesystem root and returns
// every directory that holds themes/*.toml, nearest first. An ancestor without
// a themes/ directory is skipped rather than stopping the walk, because a
// checkout is normally several levels above the working directory.
func themeDirsFromWorkdir(dir string) []string {
	var dirs []string
	for {
		if hasThemeDefinitions(dir) {
			dirs = append(dirs, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dirs
		}
		dir = parent
	}
}

// hasThemeDefinitions reports whether dir holds a themes/ directory with at
// least one .toml definition.
func hasThemeDefinitions(dir string) bool {
	entries, err := os.ReadDir(filepath.Join(dir, themesDirName))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			return true
		}
	}
	return false
}

// resolveThemeDefinitionsDir returns the first candidate that holds
// themes/*.toml, or the honest failure when none does. It is the one place the
// search order is applied.
func resolveThemeDefinitionsDir(repoDir string) (string, error) {
	for _, dir := range themeDefinitionSearchDirs(repoDir) {
		if hasThemeDefinitions(dir) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("%s", themeDefinitionsNotFoundMessage())
}

// installThemeDefinitions copies the repository's themes/*.toml into the
// per-user data directory the resolver searches, so the theme switch is offered
// from any working directory and after the temporary clone this run made is
// removed. The rules are the installer's own: only the files the repository
// ships are written, so a file under another name is never touched; a
// definition whose bytes already match is not rewritten; and the counts of what
// was written and what was already current are returned, so the clone step can
// say what it did instead of reporting a copy it did not make. Nothing is ever
// deleted, so a file an earlier run left behind is kept rather than pruned.
func installThemeDefinitions(repoDir string) (copied, current int, dest string, err error) {
	src := filepath.Join(repoDir, themesDirName)
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, 0, "", fmt.Errorf("read theme definitions: %w", err)
	}
	root := themeDefinitionsDataDir()
	if root == "" {
		return 0, 0, "", fmt.Errorf("the data directory for the theme definitions could not be determined")
	}
	dest = filepath.Join(root, themesDirName)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, 0, "", fmt.Errorf("create the theme definitions directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		want, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			return copied, current, dest, err
		}
		path := filepath.Join(dest, entry.Name())
		if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, want) {
			current++
			continue
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			return copied, current, dest, err
		}
		copied++
	}
	return copied, current, dest, nil
}

// copyThemeDefinitionsIntoDataDir runs installThemeDefinitions for the clone
// step and says what happened. It is best effort: the checkout the clone left
// behind already serves this run, so a copy that could not be made is a warning
// about later runs, not a reason to fail an installation that otherwise
// succeeded.
func copyThemeDefinitionsIntoDataDir(stepID, repoDir string) {
	if !hasThemeDefinitions(repoDir) {
		SendLog(stepID, "Skipping the theme definitions: this checkout does not ship them")
		return
	}
	copied, current, dest, err := installThemeDefinitions(repoDir)
	if err != nil {
		SendLog(stepID, fmt.Sprintf("Warning: the theme definitions could not be installed for later runs: %v", err))
		return
	}
	SendLog(stepID, fmt.Sprintf("✓ Theme definitions installed to %s (%d written, %d already current)", dest, copied, current))
}

// themePaletteRoles is the canonical role set, in render order. A theme is
// complete when it defines every one of them; anything less is partial and is
// reported rather than offered, so a switch can never apply half a theme and
// call it unified.
var themePaletteRoles = []string{
	"base",
	"text",
	"cursor",
	"cursor_text",
	"selection",
	"selection_text",
	"black",
	"red",
	"green",
	"yellow",
	"blue",
	"magenta",
	"cyan",
	"white",
	"bright_black",
	"bright_red",
	"bright_green",
	"bright_yellow",
	"bright_blue",
	"bright_magenta",
	"bright_cyan",
	"bright_white",
}

// themeSyntaxRoles is every [syntax] role the installer's own code display
// knows: a keyword tint and a string tint, each with a light and a dark member.
// A key outside this set is refused at load time rather than painted, so a
// typo cannot land as an invented syntax role.
var themeSyntaxRoles = []string{
	"keyword_light",
	"keyword_dark",
	"string_light",
	"string_dark",
}

// themeSyntaxRequired is the subset a theme has to define to be shown. The
// preview reads the dark member; a theme whose published palette ships no light
// flavour leaves the light member empty (the preview falls back to the dark
// one) rather than having a light value invented for it.
var themeSyntaxRequired = []string{
	"keyword_dark",
	"string_dark",
}

// themeDefinition is one themes/*.toml file: the theme's id and display name,
// whether it is partial and why, where its values come from, and the palette.
// A missing role is the empty string, never a guess.
type themeDefinition struct {
	ID            string
	Name          string
	Partial       bool
	PartialReason string
	Provenance    string
	Palette       map[string]string
	// Nvim is the Neovim colorscheme name this theme selects. It is empty for a
	// theme whose plugin ships no colorscheme, and Neovim is then reported rather
	// than pointed at a name that does not exist.
	Nvim string
	// Bat is the name the .tmTheme carries inside itself (its <key>name</key>), and
	// BatFile is the generated .tmTheme's file name, which is also the name bat
	// registers it under and the switch selects it by (batSelectionName). They are
	// empty for a theme with no bat theme, and bat is then reported rather than
	// pointed at a name that does not exist.
	Bat     string
	BatFile string
	// Syntax holds the installer's own code-display tints: keyword_light,
	// keyword_dark, string_light, string_dark. They are adaptive (a light and a
	// dark member) because the chrome asks the terminal which background it has,
	// and a theme that ships no light flavour leaves the light member empty rather
	// than having one invented.
	Syntax map[string]string
	// Prompt holds the prompt's own roles (Catppuccin's naming: mauve, peach,
	// subtext0, overlay0, ...), which Starship reads and the terminal palette does
	// not contain. A role the repository has no value for is left empty, and the
	// tool that needs it is then reported instead of extrapolated.
	Prompt map[string]string
	// Fish holds the fish shell theme's own roles, which are not the canonical
	// terminal roles (fish_color_normal, fish_pager_color_prefix, ...). They are
	// transcribed from the repository's fish theme files, or derived from the
	// canonical palette for a theme that ships no fish file.
	Fish map[string]string
}

// missingRoles is the terminal roles the definition does not define.
func (d themeDefinition) missingRoles() []string {
	var missing []string
	for _, role := range themePaletteRoles {
		if d.Palette[role] == "" {
			missing = append(missing, role)
		}
	}
	return missing
}

// missingSyntaxRoles is the [syntax] roles the definition does not define.
func (d themeDefinition) missingSyntaxRoles() []string {
	var missing []string
	for _, role := range themeSyntaxRequired {
		if d.Syntax[role] == "" {
			missing = append(missing, role)
		}
	}
	return missing
}

// missingRequiredRoles is every role a theme needs to be applicable and
// showable: the twenty-two canonical terminal roles and the [syntax] members the
// preview reads. The two are checked together so a definition missing either is
// reported the same way.
func (d themeDefinition) missingRequiredRoles() []string {
	return append(d.missingRoles(), d.missingSyntaxRoles()...)
}

// Complete reports whether the theme can be applied to every tool without
// leaving one on the old palette *and* previewed in the installer: the
// twenty-two canonical roles and the [syntax] tints the preview reads. A theme
// that can be applied but not shown is not offered, because a row whose
// selection cannot repaint the interface would be half a feature.
func (d themeDefinition) Complete() bool { return len(d.missingRequiredRoles()) == 0 }

// themeHexRE is the shape a colour value has to have. A value that is not a
// hex colour is a definition error, not a tool's problem: it is caught when the
// definitions are loaded rather than when a block is written.
var themeHexRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// themeFishHexRE is the same shape without the leading '#', which is how the
// fish theme files write a colour.
var themeFishHexRE = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

// parseThemeDefinition reads the small TOML subset the definitions use: [theme]
// and [palette] tables, key = "value" pairs, comments. It is deliberately not a
// general parser - a new dependency is not worth it for five files whose shape
// this test pins - and it rejects anything it does not understand rather than
// ignoring it.
func parseThemeDefinition(data []byte) (themeDefinition, error) {
	def := themeDefinition{Palette: map[string]string{}, Fish: map[string]string{}, Prompt: map[string]string{}, Syntax: map[string]string{}}
	section := ""

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return def, fmt.Errorf("cannot parse line %q", line)
		}
		key = strings.TrimSpace(key)
		value = themeValueToken(value)

		switch section {
		case "theme":
			switch key {
			case "id":
				def.ID = value
			case "name":
				def.Name = value
			case "partial":
				def.Partial = value == "true"
			case "partial_reason":
				def.PartialReason = value
			case "provenance":
				def.Provenance = value
			default:
				return def, fmt.Errorf("unknown [theme] key %q", key)
			}
		case "palette":
			def.Palette[key] = value
		case "fish":
			def.Fish[key] = value
		case "prompt":
			def.Prompt[key] = value
		case "syntax":
			def.Syntax[key] = value
		case "nvim":
			if key == "name" {
				def.Nvim = value
			}
		case "bat":
			switch key {
			case "name":
				def.Bat = value
			case "file":
				def.BatFile = value
			}
		default:
			return def, fmt.Errorf("key %q is outside any known table", key)
		}
	}

	for role, value := range def.Palette {
		if value == "" {
			delete(def.Palette, role)
			continue
		}
		if !slices.Contains(themePaletteRoles, role) {
			return def, fmt.Errorf("palette role %q is not a canonical role", role)
		}
		if !themeHexRE.MatchString(value) {
			return def, fmt.Errorf("palette role %q is not a #rrggbb colour: %q", role, value)
		}
	}
	if def.ID == "" {
		return def, fmt.Errorf("the definition names no id")
	}
	for role, value := range def.Fish {
		if !themeFishHexRE.MatchString(value) {
			return def, fmt.Errorf("fish role %q is not a six-digit hex colour: %q", role, value)
		}
	}
	for role, value := range def.Prompt {
		// "none" is Starship's own "no colour", so it is a value it accepts.
		if value != "none" && !themeHexRE.MatchString(value) {
			return def, fmt.Errorf("prompt role %q is neither a #rrggbb colour nor \"none\": %q", role, value)
		}
	}
	for role, value := range def.Syntax {
		if value == "" {
			continue
		}
		if !slices.Contains(themeSyntaxRoles, role) {
			return def, fmt.Errorf("syntax role %q is not a known syntax role", role)
		}
		if !themeHexRE.MatchString(value) {
			return def, fmt.Errorf("syntax role %q is not a #rrggbb colour: %q", role, value)
		}
	}
	if !def.Complete() && !def.Partial {
		return def, fmt.Errorf("the definition misses roles but is not marked partial: %v", def.missingRequiredRoles())
	}
	if def.Partial && def.PartialReason == "" {
		return def, fmt.Errorf("the definition is partial without saying why")
	}
	return def, nil
}

// themeValueToken pulls the value out of a `key = value` line, dropping a
// trailing comment. A quoted value is read to its closing quote, so a '#'
// inside a citation is not mistaken for a comment.
func themeValueToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if quote := value[0]; quote == '"' || quote == '\'' {
		if end := strings.IndexByte(value[1:], quote); end >= 0 {
			return value[1 : 1+end]
		}
		return value[1:]
	}
	if i := strings.IndexByte(value, '#'); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value
}

// loadThemeDefinitions reads every themes/*.toml under the repository checkout.
// The file name is the id, so the menu, the definitions and the guard cannot
// disagree about what a theme is called.
func loadThemeDefinitions(repoDir string) ([]themeDefinition, error) {
	dir := filepath.Join(repoDir, themesDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read theme definitions: %w", err)
	}

	var defs []themeDefinition
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		def, err := parseThemeDefinition(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if want := strings.TrimSuffix(entry.Name(), ".toml"); def.ID != want {
			return nil, fmt.Errorf("themes/%s defines id %q; the file name is the id", entry.Name(), def.ID)
		}
		if def.Name == "" {
			def.Name = def.ID
		}
		defs = append(defs, def)
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("no theme definition found in %s", dir)
	}
	return defs, nil
}

// themeByID finds a definition by id.
func themeByID(defs []themeDefinition, id string) (themeDefinition, bool) {
	for _, def := range defs {
		if def.ID == id {
			return def, true
		}
	}
	return themeDefinition{}, false
}

// offeredThemeIDs is the list the menu offers: the complete themes, in the
// order the definitions were read. A partial theme is reported, never offered.
func offeredThemeIDs(defs []themeDefinition) []string {
	var ids []string
	for _, def := range defs {
		if def.Complete() {
			ids = append(ids, def.ID)
		}
	}
	return ids
}

// themeTool is one tool the switch can theme, and the roles it needs.
type themeTool struct {
	ID   string
	Name string
	// Needs are the canonical palette roles the tool cannot be painted without.
	Needs []string
	// PromptNeeds are the prompt roles (Catppuccin naming) the tool needs on top;
	// Starship's palette table is written in those, not in ANSI roles.
	PromptNeeds []string
	// Available reports whether the definition can paint this tool through a name
	// rather than a palette (Neovim's colorscheme). A nil Available means the
	// artifact itself is the whole question.
	Available func(themeDefinition) bool
}

// themeTools is every tool whose colours this repository owns, in the order the
// switch reports them. A tool is painted when the definition gives its generator
// everything it needs: Needs names the canonical roles, PromptNeeds the prompt
// roles, and Available asks the generator itself about the roles a tool reads
// through a name or a file (fish's own roles, bat's theme name, Neovim's
// colorscheme). There is no second table saying which theme may paint which
// tool: the definitions are that table, so a theme that cannot paint a tool says
// so through the role it is missing rather than through a list somebody forgot
// to extend.
var themeTools = []themeTool{
	{ID: "alacritty", Name: "Alacritty", Needs: themePaletteRoles},
	{ID: "kitty", Name: "Kitty", Needs: themePaletteRoles},
	{ID: "wezterm", Name: "WezTerm", Needs: themePaletteRoles},
	{ID: "ghostty", Name: "Ghostty", Needs: themePaletteRoles},
	{ID: "starship", Name: "Starship", Needs: themePaletteRoles, PromptNeeds: themePromptRequired},
	{ID: "zsh", Name: "the zsh line editor", Needs: themePaletteRoles},
	{ID: "p10k", Name: "the p10k prompt", Needs: themePaletteRoles},
	{ID: "herdr", Name: "Herdr", Needs: []string{"selection", "blue"}},
	// lazygit is the git UI the Neovim configuration opens. Its colours live in its
	// own config.yml, generated from the definition like the rest, and the render is
	// what decides whether a theme can paint it: a definition missing a role the
	// block names is reported here rather than on the row that applies it.
	{ID: "lazygit", Name: "lazygit", Available: func(d themeDefinition) bool {
		_, err := renderLazygitTheme(d)
		return err == nil
	}},
	// zellij paints its panes from a named theme in its own config.kdl. The block is
	// generated like the rest, so a machine that switches theme does not leave the
	// multiplexer on whatever palette the file happened to be written with.
	{ID: "zellij", Name: "Zellij", Available: func(d themeDefinition) bool {
		_, err := renderZellijTheme(d)
		return err == nil
	}},
	// delta is the git pager the configuration selects. Its syntax theme is bat's,
	// so the diff and the shell highlight with the same file, and every colour the
	// section names is a palette role.
	{ID: "delta", Name: "delta", Available: func(d themeDefinition) bool {
		_, err := renderDeltaTheme(d)
		return err == nil
	}},
	// btop ships no configuration at all, so it drew its built-in palette beside
	// every themed terminal. Its colours come from a .theme file this repository
	// generates and from the color_theme line in btop.conf that selects it, so a
	// theme can paint it only when the renderer can fill every key the file names.
	{ID: "btop", Name: "btop", Available: func(d themeDefinition) bool {
		_, err := renderBtopTheme(d)
		return err == nil
	}},
	// fish, bat, Neovim and tmux are driven by a file or a theme name rather than
	// by the canonical palette alone: fish and bat read a generated file, Neovim
	// names a colorscheme, and tmux gets this repository's palette applied to its
	// own style options. Each asks its own generator, so a definition the generator
	// would refuse (no fish roles to derive from, no [bat] name, no colorscheme) is
	// reported here rather than on the row that applies it.
	{ID: "fish", Name: "fish", Available: func(d themeDefinition) bool {
		_, err := themeFishRolesFor(d)
		return err == nil
	}},
	{ID: "bat", Name: "bat", Available: func(d themeDefinition) bool {
		_, err := renderBatTheme(d)
		return err == nil
	}},
	{ID: "nvim", Name: "Neovim", Available: themeNvimAvailable},
	{ID: "tmux", Name: "tmux", Available: func(d themeDefinition) bool {
		_, err := renderTmuxTheme(d)
		return err == nil
	}},
	// nushell's colours live in the shell's own config.nu, in the LS_COLORS table
	// nushell paints `ls` output from and the $dark_theme record the shipped
	// configuration selects with `color_config`. Both are derived from the
	// definition's own palette by the fixed mapping recorded in
	// themes/README.md, so the shell cannot stay on a palette no theme declares
	// while the row says nothing (issue #240).
	{ID: "nushell", Name: "nushell", Available: func(d themeDefinition) bool {
		_, err := renderNushellTheme(d)
		return err == nil
	}},
}

// themeCoverage splits the tools into the ones a theme can paint and the ones it
// cannot, the reason always being something the definition is missing: a
// canonical role, a prompt role it can neither declare nor derive, or the name
// or file a tool reads. The menu reports the second list, so a reader is told
// which tools would stay on the old palette before anything changes. The list is
// empty for every theme the library offers, and
// TestEveryOfferedThemePaintsEveryTool is what holds it there.
func themeCoverage(def themeDefinition) (covered, uncovered []string) {
	for _, tool := range themeTools {
		missing := false
		for _, role := range tool.Needs {
			if def.Palette[role] == "" {
				missing = true
				break
			}
		}
		if !missing && len(tool.PromptNeeds) > 0 {
			roles, err := themePromptRolesFor(def)
			if err != nil {
				missing = true
			} else {
				for _, role := range tool.PromptNeeds {
					if roles[role] == "" {
						missing = true
						break
					}
				}
			}
		}
		if !missing && tool.Available != nil && !tool.Available(def) {
			missing = true
		}
		if missing {
			uncovered = append(uncovered, tool.Name)
			continue
		}
		covered = append(covered, tool.Name)
	}
	return covered, uncovered
}

// themeSourceBlock is a shipped file a definition claims to come from.
type themeSourceBlock struct {
	Tool string
	Path string
	// Roles is the palette roles this file is expected to carry. It is per file
	// because the files do not all carry every role: the prompt reads ten, Herdr
	// reads two. A role missing from its file is drift.
	Roles []string
}

// themeSourceBlocks names the files each theme's values have to keep matching.
// The guard in install_paths_test.go reads these and fails when a shipped block
// no longer contains its definition's value, which is the drift that the six
// hand-written copies used to hide.
var themeSourceBlocks = map[string][]themeSourceBlock{
	"dotfiles": {
		{Tool: "Alacritty", Path: "alacritty.toml", Roles: themePaletteRoles},
		{Tool: "Kitty", Path: "dotfiles-kitty/kitty.conf", Roles: themePaletteRoles},
		{Tool: "WezTerm", Path: ".wezterm.lua", Roles: themePaletteRoles},
		{Tool: "Starship", Path: "starship.toml", Roles: []string{"base", "text", "red", "green", "yellow", "blue", "magenta", "cyan", "bright_black"}},
		{Tool: "zsh", Path: "dotfiles-zsh/.zshrc", Roles: []string{"base", "text", "red", "green", "yellow", "blue", "magenta", "cyan", "bright_black"}},
		{Tool: "p10k", Path: "dotfiles-zsh/.p10k.zsh", Roles: []string{"base", "text", "red", "green", "yellow", "blue", "magenta", "cyan", "bright_black"}},
		{Tool: "Herdr", Path: "dotfiles-herdr/config.toml", Roles: []string{"selection", "blue"}},
		{Tool: "fish", Path: "dotfiles-fish/fish/config.fish", Roles: []string{"text", "green", "magenta", "yellow", "cyan", "red", "blue", "bright_black", "selection"}},
		{Tool: "tmux", Path: "dotfiles-tmux/tmux.conf", Roles: []string{"base", "text", "blue", "bright_black", "yellow", "red", "green", "selection"}},
		{Tool: "nushell", Path: "dotfiles-nushell/config.nu", Roles: []string{"base", "blue", "magenta", "cyan", "red", "yellow", "bright_black", "bright_green", "bright_yellow"}},
	},
	"catppuccin-mocha": {
		{Tool: "Ghostty", Path: "dotfiles-ghostty/themes/catppuccin-mocha.conf", Roles: []string{"base", "text", "cursor", "cursor_text", "selection", "selection_text", "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "bright_black", "bright_red", "bright_green", "bright_yellow", "bright_blue", "bright_magenta", "bright_cyan", "bright_white"}},
	},
}

// themeValueInFile reports whether a palette value appears in a shipped file.
// The comparison is case-insensitive and ignores the leading '#', because the
// files write the same colour as #06080f, 06080f and #F3F6F9 across the tools,
// and the drift that matters is a changed digit, not a changed spelling.
func themeValueInFile(value, file string) bool {
	needle := strings.ToLower(strings.TrimPrefix(value, "#"))
	return strings.Contains(strings.ToLower(file), needle)
}

// ----------------------------------------------------------------------------
// Generating the shipped blocks from the definition
// ----------------------------------------------------------------------------
//
// The blocks below are generated from themes/*.toml and rewritten by the
// installer when a theme is applied. The committed files hold defaultThemeID.
// A block is delimited by two marker lines so it can be found and replaced
// without parsing the tool's own file format, and it carries the ownership
// marker the preserve-user-configs rule reads, so a switch refuses to write a
// file this repository does not own.

// defaultThemeID is the theme the committed files hold: the dotfiles palette.
const defaultThemeID = "dotfiles"

const (
	// themeBlockBeginTag and themeBlockEndTag bracket a generated block. The text
	// after the tag is the theme id, so a reader (and the switch) can tell which
	// theme a file currently holds. A file that holds two generated regions (the
	// Starship palette line and its table) names them, so the two cannot be
	// confused.
	themeBlockBeginTag = ">>> dotfiles-theme"
	themeBlockEndTag   = "<<< dotfiles-theme"
)

// themeBeginTag and themeEndTag are the markers for one named block. An empty
// name is the file's single block.
func themeBeginTag(block string) string {
	if block == "" {
		return themeBlockBeginTag + ":"
	}
	return themeBlockBeginTag + "-" + block + ":"
}

// themeEndTag is the closing marker for one named block.
func themeEndTag(block string) string {
	if block == "" {
		return themeBlockEndTag + " <<<"
	}
	return themeBlockEndTag + "-" + block + " <<<"
}

// themeArtifact is one file whose theme block is generated from a definition.
type themeArtifact struct {
	Tool    string
	Path    string
	Comment string
	// DefaultTheme is the theme the committed file holds. Empty means
	// defaultThemeID, which is what every artifact that does not hold a separate
	// default uses - Neovim included. A default is what a fresh install starts
	// on, and a fresh install has only the plugins the configuration declares as
	// installed, so a default that names a plugin's colorscheme is a default that
	// breaks on a machine without that plugin.
	DefaultTheme string
	// Block names the generated region inside the file. It is empty for a file
	// with one block; Starship has two (its palette line and its palette table),
	// and naming them keeps the two markers apart.
	Block string
	// AdoptStart is the line the existing hand-written block begins at, used once
	// to wrap it in markers. Empty means the DOTFILES THEME box title.
	AdoptStart string
	// NotBoxed marks a block with no box header, so adoption starts at the
	// AdoptStart line rather than walking back to a box corner.
	NotBoxed bool
	// AdoptEnd is the line the existing block ends at. Empty means end of file;
	// a boxed block ends before the next box, a box-less one includes this line.
	AdoptEnd string
	// AdoptEndKeep means the AdoptEnd line is NOT part of the block: the region
	// ends just before it, where the following line is the next section's header.
	AdoptEndKeep bool
	Render       func(themeDefinition) (string, error)
}

// themeActiveArtifacts are the files that hold the currently applied theme.
// They are regenerated in place by the switch and by the -update guard.
var themeActiveArtifacts = []themeArtifact{
	{Tool: "alacritty", Path: "alacritty.toml", Comment: "#", Render: renderAlacrittyTheme},
	{Tool: "kitty", Path: "dotfiles-kitty/kitty.conf", Comment: "#", Render: renderKittyTheme},
	{Tool: "wezterm", Path: ".wezterm.lua", Comment: "--", AdoptEnd: "WINDOWS (WSL)", Render: renderWeztermTheme},
	{Tool: "ghostty", Path: "dotfiles-ghostty/config", Comment: "#", Render: renderGhosttyTheme},
	{Tool: "herdr", Path: "dotfiles-herdr/config.toml", Comment: "#", AdoptStart: "# Token overrides on top of that theme", NotBoxed: true, AdoptEnd: "accent =", Render: renderHerdrTheme},
	{Tool: "lazygit", Path: "dotfiles-lazygit/lazygit/config.yml", Comment: "#", AdoptStart: "  theme:", NotBoxed: true, AdoptEnd: "    defaultFgColor:", AdoptEndKeep: true, Render: renderLazygitTheme},
	{Tool: "zellij", Path: "dotfiles-zellij/zellij/config.kdl", Comment: "//", NotBoxed: true, AdoptStart: "themes {", AdoptEnd: `theme "kanagawa_wave"`, Render: renderZellijTheme},
	{Tool: "delta", Path: ".gitconfig", Comment: "#", NotBoxed: true, AdoptStart: "[delta]", AdoptEnd: "\tnavigate = true", AdoptEndKeep: true, Render: renderDeltaTheme},
	{Tool: "zellij-layout", Path: "dotfiles-zellij/zellij/layouts/work.kdl", Comment: "//", Render: renderZellijLayout},
	{Tool: "nushell-bat", Path: "dotfiles-nushell/config.nu", Comment: "#", Block: "bat", NotBoxed: true, AdoptStart: "$env.BAT_THEME = ", AdoptEnd: "$env.BAT_THEME = ", Render: renderNushellBatSelection},
	// btop reads color_theme from btop.conf and the palette from the .theme file it
	// names. The file is the one line this repository owns, and its own ".theme"
	// suffix is btop's, not the key's: btop appends it, which is why the committed
	// theme file is named after the theme id.
	{Tool: "btop", Path: "dotfiles-btop/btop/btop.conf", Comment: "#", NotBoxed: true, AdoptStart: "color_theme =", AdoptEnd: "color_theme =", Render: renderBtopSelection},
	{Tool: "starship", Path: "starship.toml", Comment: "#", Block: "palette", AdoptStart: `palette = "dotfiles"`, NotBoxed: true, AdoptEnd: `palette = "dotfiles"`, Render: renderStarshipPaletteLine},
	{Tool: "starship", Path: "starship.toml", Comment: "#", Block: "palettes", AdoptStart: "[palettes.catppuccin_mocha]", NotBoxed: true, AdoptEnd: `crust = "#06080f"`, Render: renderStarshipPaletteTable},
	{Tool: "zsh", Path: "dotfiles-zsh/.zshrc", Comment: "#", AdoptStart: "# ─── Palette", NotBoxed: true, AdoptEnd: `Gd=${PALETTE_YELLOW_SGR}"`, Render: renderZshTheme},
	{Tool: "p10k", Path: "dotfiles-zsh/.p10k.zsh", Comment: "#", AdoptStart: "  # ── Palette", NotBoxed: true, AdoptEnd: `typeset -g PALETTE_CYAN=`, Render: renderP10kTheme},
	// Neovim's committed colorscheme is the dotfiles palette (DefaultTheme
	// empty), whose file the same config copy installs, so a fresh machine can
	// always load it. The anchors are the assignment itself, not one theme's
	// name: the line the switch rewrites holds whatever theme was applied, and a
	// fixed name would only match one of them.
	{Tool: "nvim", Path: "dotfiles-nvim/nvim/lua/plugins/colorscheme.lua", Comment: "--", AdoptStart: "colorscheme = ", NotBoxed: true, AdoptEnd: "colorscheme = ", Render: renderNvimColorscheme},
	{Tool: "bat", Path: "dotfiles-zsh/.zshrc", Comment: "#", Block: "bat", NotBoxed: true, AdoptStart: "# --- bat ", AdoptEnd: "# --- zsh-autosuggestions", AdoptEndKeep: true, Render: renderBatSelection},
	// fish's own config file is the artifact: its palette lives there as global
	// variables, so the switch rewrites a file this repository owns instead of
	// writing the user's theme state in fish_variables.
	{Tool: "fish", Path: "dotfiles-fish/fish/config.fish", Comment: "#", NotBoxed: true, AdoptStart: "set -l foreground F3F6F9 normal", AdoptEnd: "set -g fish_pager_color_description $comment", Render: renderFishConfig},
	// tmux gets its own style block rather than depending on the kanagawa plugin.
	// The block is committed after the TPM run line; TestTmuxThemeBlockLoadsAfterPlugins
	// pins that order.
	{Tool: "tmux", Path: "dotfiles-tmux/tmux.conf", Comment: "#", NotBoxed: true, AdoptStart: "# DOTFILES THEME", AdoptEnd: "# DOTFILES THEME", Render: renderTmuxTheme},
	// nushell's own config.nu holds the colour region this repository owns: the
	// LS_COLORS table nushell paints `ls` output from and the `$dark_theme` record
	// the configuration selects. The region runs from the table's opening line to
	// the record's last key; the record's own closing brace, the file's comments,
	// the light theme below it and the user's own lines are all outside it. The end
	// anchor is that last key rather than the `}` line, which is not unique in the
	// file and which would make a file that lost its following section have its
	// whole tail rewritten as the region.
	{Tool: "nushell", Path: "dotfiles-nushell/config.nu", Comment: "#", NotBoxed: true,
		AdoptStart: "$env.LS_COLORS = (", AdoptEnd: "shape_variable:",
		Render: renderNushellTheme},
}

// themeHex returns a role's value and refuses a definition that misses it, so a
// renderer can never emit an empty colour.
func themeHex(def themeDefinition, role string) (string, error) {
	value := def.Palette[role]
	if value == "" {
		return "", fmt.Errorf("theme %q defines no %q, so its %s block cannot be generated", def.ID, role, role)
	}
	return value, nil
}

// themeArtifactBlock is the marked, generated block for one artifact.
func themeArtifactBlock(art themeArtifact, def themeDefinition) (string, error) {
	body, err := art.Render(def)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s dotfiles-managed-config: %s\n", art.Comment, art.Tool)
	fmt.Fprintf(&b, "%s %s %s (generated from themes/%s.toml; edit the definition, not this block) >>>\n", art.Comment, themeBeginTag(art.Block), def.ID, def.ID)
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s %s\n", art.Comment, themeEndTag(art.Block))
	return b.String(), nil
}

// replaceThemeBlock swaps the lines between the two markers for block. It
// reports false when the file carries no markers, which is how the guard says a
// file has not been adopted yet.
func replaceThemeBlock(content, block, blockName string) (string, bool) {
	beginTag := themeBeginTag(blockName)
	endTag := themeEndTag(blockName)
	lines := strings.Split(content, "\n")
	begin, end := -1, -1
	for i, line := range lines {
		if begin < 0 && strings.Contains(line, beginTag) {
			begin = i
			continue
		}
		if begin >= 0 && strings.Contains(line, endTag) {
			end = i
			break
		}
	}
	if begin < 0 || end < 0 {
		return content, false
	}
	// The ownership marker sits immediately above the begin marker and belongs to
	// the block, so it is replaced with it rather than left behind and duplicated.
	if begin > 0 && strings.Contains(lines[begin-1], "dotfiles-managed-config:") {
		begin--
	}

	out := make([]string, 0, len(lines))
	out = append(out, lines[:begin]...)
	out = append(out, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	out = append(out, lines[end+1:]...)
	return strings.Join(out, "\n"), true
}

// adoptThemeBlock wraps a file's existing hand-written block in the two markers,
// so the next -update can replace the content between them. It is the one-time
// bridge from hand-written to generated; it finds the block by its box header
// and its per-file end anchor rather than by parsing the tool's format.
func adoptThemeBlock(content string, art themeArtifact, block string) (string, bool) {
	lines := strings.Split(content, "\n")

	startNeedle := art.AdoptStart
	if startNeedle == "" {
		startNeedle = "DOTFILES THEME"
	}
	start := -1
	for i, line := range lines {
		if strings.Contains(line, startNeedle) {
			start = i
			break
		}
	}
	if start < 0 {
		return content, false
	}
	if !art.NotBoxed {
		for start > 0 && !strings.Contains(lines[start], "┌") {
			start--
		}
		if !strings.Contains(lines[start], "┌") {
			return content, false
		}
	}
	// The generated block carries its own ownership-marker line, so when the
	// marker sits immediately above the region it is consumed by the block rather
	// than left behind. Without this a marked file that lost its block would gain a
	// second marker line (the same dedup replaceThemeBlock does on the next run).
	if start > 0 && strings.Contains(lines[start-1], themeOwnershipMarker) {
		start--
	}

	end := len(lines)
	if art.AdoptEnd != "" {
		for i := start; i < len(lines); i++ {
			if !strings.Contains(lines[i], art.AdoptEnd) {
				continue
			}
			if art.NotBoxed {
				if art.AdoptEndKeep {
					// The block ends before this line, which is the next section.
					end = i
					for end > start && strings.TrimSpace(lines[end-1]) == "" {
						end--
					}
				} else {
					// The block owns this line, so the region ends after it.
					end = i + 1
				}
				break
			}
			// Walk back over the next section's own box to its top line, so the
			// block is replaced and the following box is left whole. The blank
			// line that separated them is dropped; the block supplies its own.
			end = i
			for end > start && !strings.Contains(lines[end], "┌") {
				end--
			}
			break
		}
	}

	out := make([]string, 0, len(lines))
	out = append(out, lines[:start]...)
	out = append(out, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	if !art.NotBoxed && end < len(lines) {
		// Keep the blank line that separated the block from the next section.
		out = append(out, "")
	}
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n"), true
}

// themeBlockID reads the theme id out of a marked block, so the switch can tell
// what a file currently holds before it changes it.
func themeBlockID(content, blockName string) (string, bool) {
	beginTag := themeBeginTag(blockName)
	for _, line := range strings.Split(content, "\n") {
		i := strings.Index(line, beginTag)
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(line[i+len(beginTag):])
		if id, _, ok := strings.Cut(rest, " "); ok {
			return id, true
		}
		if rest != "" {
			return rest, true
		}
	}
	return "", false
}

// themeBox is the three-line header every generated block opens with, so the
// files keep the shape a reader already knows.
func themeBox(comment, title string) string {
	const inner = 78
	return comment + " ┌" + strings.Repeat("─", inner) + "┐\n" +
		comment + " │" + padCentre(title, inner) + "│\n" +
		comment + " └" + strings.Repeat("─", inner) + "┘"
}

// padCentre centres title in width columns, padding with spaces.
func padCentre(title string, width int) string {
	if len(title) >= width {
		return title[:width]
	}
	left := (width - len(title)) / 2
	right := width - len(title) - left
	return strings.Repeat(" ", left) + title + strings.Repeat(" ", right)
}

// renderAlacrittyTheme renders the [colors.*] block of alacritty.toml.
func renderAlacrittyTheme(def themeDefinition) (string, error) {
	role := func(name string) string {
		value, _ := themeHex(def, name)
		return value
	}
	for _, name := range themePaletteRoles {
		if _, err := themeHex(def, name); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf(`%s

# --- Base Colors ---
[colors.primary]
background = "%s"
foreground = "%s"

[colors.cursor]
cursor = "%s"
text = "%s"

[colors.selection]
background = "%s"
text = "%s"

# --- Normal Colors ---
[colors.normal]
black   = "%s"
red     = "%s"
green   = "%s"
yellow  = "%s"
blue    = "%s"
magenta = "%s"
cyan    = "%s"
white   = "%s"

# --- Bright Colors ---
[colors.bright]
black   = "%s"
red     = "%s"
green   = "%s"
yellow  = "%s"
blue    = "%s"
magenta = "%s"
cyan    = "%s"
white   = "%s"`,
		themeBox("#", "DOTFILES THEME"),
		role("base"), role("text"), role("cursor"), role("cursor_text"), role("selection"), role("selection_text"),
		role("black"), role("red"), role("green"), role("yellow"), role("blue"), role("magenta"), role("cyan"), role("white"),
		role("bright_black"), role("bright_red"), role("bright_green"), role("bright_yellow"), role("bright_blue"), role("bright_magenta"), role("bright_cyan"), role("bright_white")), nil
}

// renderKittyTheme renders the flat colour keys of kitty.conf.
func renderKittyTheme(def themeDefinition) (string, error) {
	for _, name := range themePaletteRoles {
		if _, err := themeHex(def, name); err != nil {
			return "", err
		}
	}
	role := func(name string) string {
		value, _ := themeHex(def, name)
		return value
	}

	return fmt.Sprintf(`%s

# --- Base Colors ---
background            %s
foreground            %s
cursor                %s
selection_background  %s
selection_foreground  %s
url_color             %s

# --- Tabs ---
active_tab_background   %s
active_tab_foreground   %s
inactive_tab_background %s
inactive_tab_foreground %s

# --- Normal Colors ---
color0  %s
color1  %s
color2  %s
color3  %s
color4  %s
color5  %s
color6  %s
color7  %s

# --- Bright Colors ---
color8  %s
color9  %s
color10 %s
color11 %s
color12 %s
color13 %s
color14 %s
color15 %s`,
		themeBox("#", "DOTFILES THEME"),
		role("base"), role("text"), role("cursor"), role("selection"), role("selection_text"), role("blue"),
		role("selection"), role("text"), role("base"), role("bright_black"),
		role("black"), role("red"), role("green"), role("yellow"), role("blue"), role("magenta"), role("cyan"), role("white"),
		role("bright_black"), role("bright_red"), role("bright_green"), role("bright_yellow"), role("bright_blue"), role("bright_magenta"), role("bright_cyan"), role("bright_white")), nil
}

// renderGhosttyTheme renders the config keys of dotfiles-ghostty/config. Ghostty
// writes background and foreground without the leading '#' and the palette with
// it, which is the shape the file already had.
func renderGhosttyTheme(def themeDefinition) (string, error) {
	for _, name := range themePaletteRoles {
		if _, err := themeHex(def, name); err != nil {
			return "", err
		}
	}
	role := func(name string) string {
		value, _ := themeHex(def, name)
		return value
	}
	bare := func(name string) string { return strings.TrimPrefix(role(name), "#") }

	return fmt.Sprintf(`%s

# --- Base Colors ---
background = %s
foreground = %s
cursor-color = %s
selection-background = %s
selection-foreground = %s

# --- Normal Colors ---
palette = 0=#%s
palette = 1=#%s
palette = 2=#%s
palette = 3=#%s
palette = 4=#%s
palette = 5=#%s
palette = 6=#%s
palette = 7=#%s

# --- Bright Colors ---
palette = 8=#%s
palette = 9=#%s
palette = 10=#%s
palette = 11=#%s
palette = 12=#%s
palette = 13=#%s
palette = 14=#%s
palette = 15=#%s`,
		themeBox("#", "DOTFILES THEME"),
		bare("base"), bare("text"), bare("cursor"), bare("selection"), bare("selection_text"),
		bare("black"), bare("red"), bare("green"), bare("yellow"), bare("blue"), bare("magenta"), bare("cyan"), bare("white"),
		bare("bright_black"), bare("bright_red"), bare("bright_green"), bare("bright_yellow"), bare("bright_blue"), bare("bright_magenta"), bare("bright_cyan"), bare("bright_white")), nil
}

// renderWeztermTheme renders the config.colors tables of .wezterm.lua.
func renderWeztermTheme(def themeDefinition) (string, error) {
	for _, name := range themePaletteRoles {
		if _, err := themeHex(def, name); err != nil {
			return "", err
		}
	}
	role := func(name string) string {
		value, _ := themeHex(def, name)
		return value
	}

	return fmt.Sprintf(`%s

config.colors = {
	-- Base Colors
	foreground = "%s",
	background = "%s",

	-- Cursor
	cursor_bg = "%s",
	cursor_fg = "%s",
	cursor_border = "%s",

	-- Selection
	selection_fg = "%s",
	selection_bg = "%s",

	-- Normal Colors
	ansi = {
		"%s", -- black
		"%s", -- red
		"%s", -- green
		"%s", -- yellow
		"%s", -- blue
		"%s", -- magenta
		"%s", -- cyan
		"%s", -- white
	},

	-- Bright Colors
	brights = {
		"%s", -- black
		"%s", -- red
		"%s", -- green
		"%s", -- yellow
		"%s", -- blue
		"%s", -- magenta
		"%s", -- cyan
		"%s", -- white
	},
}`,
		themeBox("--", "DOTFILES THEME"),
		role("text"), role("base"),
		role("cursor"), role("cursor_text"), role("cursor"),
		role("selection_text"), role("selection"),
		role("black"), role("red"), role("green"), role("yellow"), role("blue"), role("magenta"), role("cyan"), role("white"),
		role("bright_black"), role("bright_red"), role("bright_green"), role("bright_yellow"), role("bright_blue"), role("bright_magenta"), role("bright_cyan"), role("bright_white")), nil
}

// themeInstalledPath is where a tool's config lands in the user's home, which is
// the file the switch rewrites. It mirrors the destinations stepInstallTerminal
// copies to, so the two cannot drift.
func themeInstalledPath(art themeArtifact, homeDir string) string {
	switch art.Tool {
	case "alacritty":
		return filepath.Join(homeDir, ".config/alacritty/alacritty.toml")
	case "kitty":
		return filepath.Join(homeDir, ".config/kitty/kitty.conf")
	case "wezterm":
		return filepath.Join(homeDir, ".config/wezterm/wezterm.lua")
	case "ghostty":
		return filepath.Join(homeDir, ".config/ghostty/config")
	case "herdr":
		return filepath.Join(homeDir, ".config/herdr/config.toml")
	case "lazygit":
		return filepath.Join(homeDir, ".config/lazygit/config.yml")
	case "zellij":
		return filepath.Join(homeDir, ".config/zellij/config.kdl")
	case "delta":
		return filepath.Join(homeDir, ".gitconfig")
	case "nushell-bat":
		return filepath.Join(homeDir, ".config/nushell/config.nu")
	case "zellij-layout":
		return filepath.Join(homeDir, ".config/zellij/layouts/work.kdl")
	case "btop":
		return filepath.Join(homeDir, ".config/btop/btop.conf")
	case "starship":
		return filepath.Join(homeDir, ".config/starship.toml")
	case "zsh", "bat":
		// One file holds two artifacts: the shell palette and, further down it, the
		// bat theme selection. The switch rewrites each region in turn, so both
		// artifacts have to answer with the same path or one of them is skipped.
		return filepath.Join(homeDir, ".zshrc")
	case "p10k":
		return filepath.Join(homeDir, ".p10k.zsh")
	case "nvim":
		return filepath.Join(homeDir, ".config/nvim/lua/plugins/colorscheme.lua")
	case "fish":
		return filepath.Join(homeDir, ".config/fish/config.fish")
	case "tmux":
		return filepath.Join(homeDir, ".tmux.conf")
	case "nushell":
		// nushell reads its configuration from one directory on macOS and from
		// XDG's on Linux, which is the split the shell step installs into.
		if runtime.GOOS == "darwin" {
			return filepath.Join(homeDir, "Library/Application Support/nushell/config.nu")
		}
		return filepath.Join(homeDir, ".config/nushell/config.nu")
	}
	return ""
}

// themeOwnershipMarker is the standalone line the preserve-user-configs rule
// reads: a file that carries it belongs to dotfiles and may be rewritten, and
// one that does not is left exactly as the user wrote it.
const themeOwnershipMarker = "dotfiles-managed-config:"

// themeOwnershipMarkerLine is the exact line adoption writes into a file whose
// content proved it is ours. It is the same line themeArtifactBlock writes above
// the generated block, so an adopted file and a generated one are the same shape
// to replaceThemeBlock, and it is the only line adoption adds.
func themeOwnershipMarkerLine(art themeArtifact) string {
	return art.Comment + " " + themeOwnershipMarker + " " + art.Tool
}

// stripThemeOwnershipMarkers removes every ownership-marker line from content,
// so two versions of a file can be compared without the one line this repository
// adds to a file it owns.
func stripThemeOwnershipMarkers(content string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, themeOwnershipMarker) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// themeInstalledFileIsOurs reports whether an installed file's content proves it
// belongs to dotfiles, so the switch may adopt it rather than refuse it. The
// proof is content, never the path: a file called ~/.config/herdr/config.toml is
// not ours for being there. Two proofs are accepted:
//
//  1. the file carries a generated block begin marker (">>> dotfiles-theme...",
//     named or not) that only this repository's generator writes; or
//  2. with the ownership-marker lines removed it is byte-identical to the file
//     the repository ships for this artifact - the repository version without
//     the marker.
//
// Anything else is left exactly as it is. repoDir may be empty, in which case
// only proof 1 is available because there is no shipped file to compare with.
func themeInstalledFileIsOurs(repoDir string, art themeArtifact, installed string) bool {
	if strings.Contains(installed, themeBeginTag(art.Block)) {
		return true
	}
	if repoDir == "" {
		return false
	}
	shipped, err := os.ReadFile(filepath.Join(repoDir, art.Path))
	if err != nil {
		return false
	}
	return stripThemeOwnershipMarkers(installed) == stripThemeOwnershipMarkers(string(shipped))
}

// adoptThemeFile writes the ownership marker into a file whose content proved it
// is ours, and nothing else: no content line is touched. The marker goes above
// the generated block so replaceThemeBlock finds it with the block; a proven file
// with no generated block gets it on the first line. It returns false when the
// file already carries the marker, so adoption can never add it twice.
func adoptThemeFile(art themeArtifact, content string) (string, bool) {
	if strings.Contains(content, themeOwnershipMarker) {
		return content, false
	}

	marker := themeOwnershipMarkerLine(art)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, themeBeginTag(art.Block)) {
			continue
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:i]...)
		out = append(out, marker)
		out = append(out, lines[i:]...)
		return strings.Join(out, "\n"), true
	}

	out := make([]string, 0, len(lines)+1)
	out = append(out, marker)
	out = append(out, lines...)
	return strings.Join(out, "\n"), true
}

// themeSkippedFile is one installed file the switch left exactly as it was, and
// why it could not be themed. The switch collects every one of them in one pass,
// so one run reports the whole list rather than stopping at the first file whose
// ownership it cannot prove. That first-stop behaviour left every later managed
// file on the old palette for a reason that was about one file.
type themeSkippedFile struct {
	Path   string
	Reason string
}

// themeSkipOnce appends a file to the skipped list unless it is already there.
// Several artifacts share one file (Starship's two blocks, .zshrc's zsh and bat
// blocks), and a file left alone must be named once, not once per artifact.
func themeSkipOnce(skipped []themeSkippedFile, path, reason string) []themeSkippedFile {
	for _, s := range skipped {
		if s.Path == path {
			return skipped
		}
	}
	return append(skipped, themeSkippedFile{Path: path, Reason: reason})
}

// themeSkippedList renders one line per skipped file, the path first and the
// reason second, for the result and for the failure. It is the whole list, not
// the first file: three unowned files used to take three runs to discover.
func themeSkippedList(skipped []themeSkippedFile) string {
	lines := make([]string, 0, len(skipped))
	for _, s := range skipped {
		lines = append(lines, fmt.Sprintf("%s — %s", s.Path, s.Reason))
	}
	return strings.Join(lines, "; ")
}

// themeLeftAloneWayForward names the actions the interface actually offers for a
// file the switch cannot prove it owns. Naming an action the product does not
// offer is worse than naming none, and the picker has no per-file selection, so
// the sentence points only at paths that exist: running the installer again
// (its backup step copies an existing config before its configuration step
// writes it in the marked form) and the picker's own refresh row.
const themeLeftAloneWayForward = "what you can do: run the installer again — its backup step keeps a copy of this file first and its configuration step writes it in the marked form; the theme picker's \"" + themeRefreshRow + "\" is the in-place path for a file whose dotfiles block can still be recognised"

// themePartialNoticePrefix marks a result that themed some files and left others
// alone. The caller renders a marked notice beside the reload list instead of
// replacing it with a plain "applied", so a partial run is never reported as a
// whole one.
const themePartialNoticePrefix = "Partial: "

// themeNoticeIsPartial reports whether a switch notice describes a run that left
// at least one file alone, so the caller can keep that report on screen beside
// the reload list.
func themeNoticeIsPartial(notice string) bool {
	return strings.HasPrefix(notice, themePartialNoticePrefix)
}

// unownedThemeFileReason is why the switch could not prove it owns a file: every
// proof it attempted, named apart so a reader can tell which one the file
// failed. It carries "not owned by dotfiles" because that is the rule the file
// fell outside.
func unownedThemeFileReason(art themeArtifact) string {
	anchorStart := art.AdoptStart
	if anchorStart == "" {
		anchorStart = "DOTFILES THEME"
	}
	anchorEnd := art.AdoptEnd
	if anchorEnd == "" {
		anchorEnd = "end of file"
	}
	return fmt.Sprintf(
		"not owned by dotfiles: no %q ownership marker, no generated theme block (%s), no dotfiles region anchors (%q ... %q), and its bytes are not identical to the file this repository ships",
		themeOwnershipMarker, themeBeginTag(art.Block), anchorStart, anchorEnd)
}

// themeNothingThemedError is the refusal when a run changed nothing: every
// installed managed file was left exactly as it is, and the message names each
// one, why, and the way forward.
func themeNothingThemedError(skipped []themeSkippedFile) error {
	return fmt.Errorf(
		"nothing was themed: every installed managed file was left exactly as it is; %s. The file(s): %s",
		themeLeftAloneWayForward, themeSkippedList(skipped))
}

// themePartialResultSentence reports a run that themed what it could and left the
// rest alone: how many files were themed, and every file that was skipped with
// its reason. Its prefix is what keeps the result from reading as a plain
// success.
func themePartialResultSentence(themed int, skipped []themeSkippedFile) string {
	return fmt.Sprintf("%s%d file(s) were themed and %d file(s) were left exactly as they are: %s. %s.",
		themePartialNoticePrefix, themed, len(skipped), themeSkippedList(skipped), themeLeftAloneWayForward)
}

// applyDotfilesTheme switches every installed terminal file to the theme,
// recording the exact bytes each one held so the change can be undone. It is
// gated on dryRun() exactly as executeStep is, leaves a file it cannot prove it
// owns exactly as it is, and restores anything it already wrote if a later write
// fails. It never invents a file: a tool that is not installed is skipped.
//
// A file with no ownership marker is not refused outright, because there are two
// more degrees of ownership below the marker, and they are not the same thing:
//
//  1. the whole file is ours - themeInstalledFileIsOurs proves it by a generated
//     block begin marker or by byte-identity to what the repository ships (minus
//     the marker lines). It is adopted first, which writes the marker line and
//     nothing else, and the bytes from before adoption are what the record
//     holds, so undo takes the marker back out.
//  2. only a region inside a foreign file is ours: the file carries the anchors
//     the generator knows, but its bytes elsewhere (or inside) have drifted, so
//     no whole-file proof holds. Only the bytes between the anchors are
//     rewritten; every other byte of the file is left exactly as it was, and the
//     original is recorded so undo restores it byte-for-byte.
//
// A file that proves neither degree is left exactly as it is and named, but it no
// longer stops the run: the switch themes every file it can prove it owns, collects
// every file it could not in one pass, and reports the run as partial rather than
// as a plain success. Stopping at the first such file left every later managed file
// on the old palette for a reason that was about one file.
//
// A managed file that already carries the ownership marker but has neither the
// generated block nor the region anchors is the one case the two proofs above
// cannot bring forward - there is nothing to replace and no region to rewrite -
// and the switch does not skip it in silence: it applies the files it can, names
// each file it could not update with what refreshes it, and reports the run as
// partial. The file is left exactly as it is either way.
func applyDotfilesTheme(homeDir, repoDir string, def themeDefinition) (*dotfilesThemeRecord, string, error) {
	if dryRun() {
		SendLog("utilities", fmt.Sprintf("DRY RUN: skipping the switch to the %s theme", def.Name))
		return nil, fmt.Sprintf("DRY RUN: the %s theme was not applied.", def.Name), nil
	}

	previous := map[string]string{}
	var written []string
	var adopted []string
	var regionAdopted []string
	// skipped names every installed file the switch left exactly as it is, and why,
	// in the order the artifacts reach them. It is the whole list, not the first
	// file: ownership that cannot be proven about one artifact does not stop the
	// files that can be themed.
	var skipped []themeSkippedFile
	themed := 0
	// record keeps the first bytes read for a path. Several artifacts can share
	// one file (Starship's two blocks, .zshrc's zsh and bat blocks), and a later
	// artifact reads the file an earlier one already wrote, so it must not
	// overwrite the record: undo has to reach the file's original bytes.
	record := func(path, original string) {
		if _, ok := previous[path]; !ok {
			previous[path] = original
		}
		if !slices.Contains(written, path) {
			written = append(written, path)
		}
	}
	restoreWritten := func() {
		for _, path := range written {
			_ = os.WriteFile(path, []byte(previous[path]), 0o644)
		}
	}

	for _, art := range themeActiveArtifacts {
		path := themeInstalledPath(art, homeDir)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			restoreWritten()
			return nil, "", fmt.Errorf("could not read %s, so nothing was changed: %w", path, err)
		}
		original := string(data)
		content := original
		owned := strings.Contains(original, themeOwnershipMarker)
		if !owned && themeInstalledFileIsOurs(repoDir, art, original) {
			// The whole file is ours: it carries a generated block begin marker, or,
			// with the ownership-marker lines removed, it is what the repository
			// ships. Only then is the marker line added, and nothing else; the bytes
			// recorded are the ones from before adoption, so undo takes the marker
			// back out and leaves the file exactly as it was.
			adoptedContent, ok := adoptThemeFile(art, original)
			if !ok {
				restoreWritten()
				return nil, "", fmt.Errorf("%s could not be adopted, so nothing was changed", path)
			}
			if err := os.WriteFile(path, []byte(adoptedContent), 0o644); err != nil {
				restoreWritten()
				return nil, "", fmt.Errorf("could not write the ownership marker into %s, so the files already changed were put back: %w", path, err)
			}
			content = adoptedContent
			record(path, original)
			adopted = append(adopted, path)
		}

		block, err := themeArtifactBlock(art, def)
		if err != nil {
			restoreWritten()
			return nil, "", err
		}
		updated, ok := replaceThemeBlock(content, block, art.Block)
		if !ok {
			// The file predates the generated block. If it still carries the
			// hand-written region's anchors - the zsh palette, the p10k fallbacks,
			// Herdr's [theme.custom], the Starship tables, fish's colours - adopt
			// only that region: the bytes between the anchors are rewritten and every
			// other byte of the file is left exactly as it was.
			region, regionOK := adoptThemeBlock(content, art, block)
			if !regionOK {
				if owned || slices.Contains(adopted, path) {
					// A marked file with neither a generated block nor region anchors has
					// nothing to rewrite here. Name it so the switch says so instead of
					// reporting a change it did not make; an adopted one is already
					// recorded, so undo takes it back.
					if owned {
						skipped = themeSkipOnce(skipped, path,
							"carries the dotfiles ownership marker but no generated theme block and no region anchors, so it cannot be updated in place without refreshing it")
					}
					continue
				}
				// A file neither proof covers is left exactly as it is and named, and
				// the run keeps going: the files it can prove it owns are still themed,
				// and the ones it could not are reported together at the end.
				skipped = themeSkipOnce(skipped, path, unownedThemeFileReason(art))
				continue
			}
			updated = region
		}
		if art.Tool == "nvim" {
			// The colorscheme line names a colorscheme Neovim has to find, so the
			// name is made reachable on this machine before the line is written: a
			// generated colorscheme is installed under ~/.config/nvim/colors, and a
			// plugin's is only named when the plugin is installed. When it cannot be
			// reached the line is left out rather than starting Neovim broken, and
			// the reason is said out loud below.
			if reason := reachableNvimColorscheme(homeDir, def); reason != "" {
				skipped = themeSkipOnce(skipped, path, "Neovim was left on its previous colorscheme: "+reason)
				continue
			}
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			restoreWritten()
			return nil, "", fmt.Errorf("could not write %s, so the files already changed were put back: %w", path, err)
		}
		record(path, original)
		if !owned && !slices.Contains(adopted, path) {
			regionAdopted = append(regionAdopted, path)
		}
		themed++
	}

	// Every file left alone is named once per file, whether the switch applied the
	// rest or nothing at all, so a switch that left files on the old palette is
	// never reported as a whole one.
	for _, s := range skipped {
		SendLog("utilities", fmt.Sprintf("%s was left exactly as it is: %s", s.Path, s.Reason))
	}

	if themed == 0 {
		restoreWritten()
		if len(skipped) > 0 {
			return nil, "", themeNothingThemedError(skipped)
		}
		return nil, "", fmt.Errorf("no installed theme block was found, so nothing was changed")
	}

	next := &dotfilesThemeRecord{Theme: def.ID, Files: previous, AppliedAt: time.Now()}
	if err := writeDotfilesThemeRecord(next); err != nil {
		restoreWritten()
		return nil, "", fmt.Errorf("the theme changed but the blocks it replaced could not be recorded, "+
			"so it cannot be undone; the files were put back: %w", err)
	}

	// Adoption is said out loud, once per file and once in the notice: an
	// installer that writes a config without saying so is what the ownership rule
	// exists to prevent. The two degrees of ownership are named apart: a whole
	// file adopted by content, and a foreign file whose dotfiles region was
	// rewritten in place.
	for _, path := range adopted {
		SendLog("utilities", fmt.Sprintf("Adopted %s: it carried no ownership marker, so only the marker line was added (its content proved it is dotfiles'); Undo takes it back out.", path))
	}
	for _, path := range regionAdopted {
		SendLog("utilities", fmt.Sprintf("Adopted the dotfiles region in %s: the bytes between its anchors were rewritten and the rest of the file was left untouched; Undo puts it back.", path))
	}
	notice := ""
	if len(skipped) > 0 {
		notice = themePartialResultSentence(themed, skipped) + " "
	}
	if len(adopted) > 0 {
		notice += fmt.Sprintf("Adopted %d file(s) whose whole content proved they are dotfiles', adding the ownership marker and nothing else. ", len(adopted))
	}
	if len(regionAdopted) > 0 {
		notice += fmt.Sprintf("Adopted %d file(s) whose dotfiles region was rewritten in place, leaving the rest of each file untouched. ", len(regionAdopted))
	}
	notice += fmt.Sprintf("The %s theme is applied to %d file(s). The blocks it replaced are recorded; use Undo to put them back.",
		def.Name, themed)
	return next, notice, nil
}

// undoDotfilesTheme puts back the exact bytes each file held before the change,
// then clears the record. It is gated on dryRun() like applyDotfilesTheme.
func undoDotfilesTheme(rec dotfilesThemeRecord) (string, error) {
	if dryRun() {
		SendLog("utilities", "DRY RUN: skipping the undo of the last dotfiles theme change")
		return "DRY RUN: the last dotfiles theme change was not undone.", nil
	}
	if len(rec.Files) == 0 {
		return "", fmt.Errorf("the record holds no file, so there is nothing to put back")
	}

	for path, content := range rec.Files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("could not put %s back: %w", path, err)
		}
	}
	if err := writeDotfilesThemeRecord(nil); err != nil {
		return "", fmt.Errorf("the files were put back but the record could not be cleared: %w", err)
	}
	return fmt.Sprintf("The previous theme blocks are back in %d file(s).", len(rec.Files)), nil
}

// ----------------------------------------------------------------------------
// Bringing a theme change forward in the tools that read it
// ----------------------------------------------------------------------------
//
// Writing a tool's file is not the same as the tool showing the theme. Some
// tools watch their file and reload it live, some read it when they next start,
// and some are only reachable by the person in front of them. The switch runs
// the reloads it can do safely and idempotently -- rebuilding bat's theme cache,
// sourcing tmux's config into a server that is already running, and asking the
// Herdr session the installer runs inside to reload its own config over that
// session's socket -- and names every tool it cannot reach with the exact action
// that applies the theme. It never starts a server, closes a session or signals
// a terminal: a Herdr reload is the command behind the key, not a keypress sent
// to a pane and not a signal, so a tool that is only reachable by a live session
// (Neovim, an already-open shell) is reported, never touched.

// themeReloadTimeout bounds one reload command. A tmux, Herdr or bat that hangs
// must not hang the screen: the command runs off the update loop and is killed
// when the timeout elapses.
const themeReloadTimeout = 5 * time.Second

// themeReloadTool is one tool's answer to "does the new theme reach it?".
type themeReloadTool struct {
	Tool string
	// Done is true when nothing is left for the user: the installer ran the
	// reload, or the tool reads the file on its own. It is false when the theme is
	// on disk but a tool that is already running still shows the old one.
	Done bool
	// Note is the tool's line: what was done, or the exact action left and where
	// to run it.
	Note string
}

// themeReloadRun runs one reload command with a bounded timeout and returns its
// output. It is a variable so a guard can observe the exact commands without a
// real tmux or bat on the path.
var themeReloadRun = func(command string, env []string) (string, error) {
	result := system.Run(command, &system.ExecOptions{Env: env, Timeout: themeReloadTimeout})
	if result.Error != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Output)
		}
		if detail != "" {
			return "", fmt.Errorf("%v: %s", result.Error, detail)
		}
		return "", result.Error
	}
	return strings.TrimSpace(result.Output), nil
}

// themeReloadCommandExists is system.CommandExists, indirected so a guard can
// answer it without touching the runner's PATH.
var themeReloadCommandExists = system.CommandExists

// themeReloadInstalled reports whether a tool is on this machine. The switch only
// writes a file that exists, so only a tool whose own config is there was painted
// and belongs in the reload list. bat is the exception: its selection lives in
// .zshrc, so it is offered by the bat command or its config directory instead.
func themeReloadInstalled(tool, path, homeDir string) bool {
	if tool == "bat" {
		return themeReloadCommandExists("bat") || system.DirExists(filepath.Join(homeDir, ".config", "bat"))
	}
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// reloadThemeTools brings the applied theme forward in every installed tool and
// returns one line per tool, in the artifact table's order. It runs only the
// reloads that are idempotent and can never start a server: bat's cache is
// rebuilt from the theme files already installed, tmux's config is sourced into
// a server that is already running, and a Herdr the installer is running inside
// is asked to reload its config over that session's own socket (`herdr server
// reload-config`, the command behind Ctrl+b Shift+r). Everything else is named
// with the exact action and where to run it.
func reloadThemeTools(homeDir string) []themeReloadTool {
	var out []themeReloadTool
	seen := map[string]bool{}
	for _, art := range themeActiveArtifacts {
		if seen[art.Tool] {
			continue
		}
		seen[art.Tool] = true
		path := themeInstalledPath(art, homeDir)
		if !themeReloadInstalled(art.Tool, path, homeDir) {
			continue
		}
		out = append(out, reloadThemeToolFor(art, path, homeDir))
	}
	return out
}

// reloadThemeToolFor is the per-tool knowledge: what the installer can do for a
// tool, and what it must leave to the user. Every manual line names the key, the
// command and the shell it belongs to, because "reload the tool" is not an
// instruction.
func reloadThemeToolFor(art themeArtifact, path, homeDir string) themeReloadTool {
	switch art.Tool {
	case "alacritty":
		return themeReloadTool{Tool: "Alacritty", Done: true,
			Note: "Alacritty watches its config and reloads it live, so the new colours are already on screen."}
	case "wezterm":
		return themeReloadTool{Tool: "WezTerm", Done: true,
			Note: "WezTerm watches its config and reloads it live, so the new colours are already on screen."}
	case "starship":
		return themeReloadTool{Tool: "Starship", Done: true,
			Note: "Starship reads its config at each prompt, so the next prompt uses the theme."}
	case "kitty":
		if os.Getenv("KITTY_LISTEN_ON") != "" && themeReloadCommandExists("kitty") {
			if _, err := themeReloadRun("kitty @ load-config", nil); err == nil {
				return themeReloadTool{Tool: "Kitty", Done: true,
					Note: "reloaded the running Kitty over its own socket (`kitty @ load-config`)."}
			}
		}
		return themeReloadTool{Tool: "Kitty", Done: false,
			Note: "a Kitty that is already open keeps the old colours: press Ctrl+Shift+F5, or run `kitty @ load-config`. A new window reads the file."}
	case "ghostty":
		return themeReloadTool{Tool: "Ghostty", Done: false,
			Note: "a Ghostty that is already open keeps the old colours: press Ctrl+Shift+, on macOS, or send it SIGUSR2 on Linux. A new window reads the file."}
	case "herdr":
		// herdr's own precondition for controlling a session from inside one, and
		// what scopes `server reload-config` to the session the installer is in:
		// the client injects HERDR_ENV=1 and HERDR_SOCKET_PATH into every pane it
		// manages. Run from outside, the same command would reload whichever
		// session owns the socket in the environment, so it runs only from inside.
		if os.Getenv("HERDR_ENV") == "1" && themeReloadCommandExists("herdr") {
			if _, err := themeReloadRun("herdr server reload-config", nil); err == nil {
				return themeReloadTool{Tool: "Herdr", Done: true,
					Note: "reloaded the running Herdr over its own socket (`herdr server reload-config`) -- the same reload as Ctrl+b Shift+r, with no signal and no keypress sent to a pane."}
			}
		}
		return themeReloadTool{Tool: "Herdr", Done: false,
			Note: "a Herdr session that is already running keeps the old theme: press Ctrl+b Shift+r to reload its config, or restart it. A Herdr is reloaded only from inside it (`HERDR_ENV`), over its own socket, and it is never signalled or restarted by the installer."}
	case "zsh":
		return themeReloadTool{Tool: "zsh", Done: false,
			Note: "a shell that is already open keeps the old palette: open a new shell, or run `exec zsh`."}
	case "p10k":
		return themeReloadTool{Tool: "p10k", Done: false,
			Note: "a prompt that is already drawn keeps the old palette: run `p10k reload` in that shell, or open a new shell."}
	case "nvim":
		return themeReloadTool{Tool: "Neovim", Done: false,
			Note: "a Neovim that is already open keeps the old colorscheme: run `:colorscheme <name>` in it. A new one reads the selection."}
	case "lazygit":
		return themeReloadTool{Tool: "lazygit", Done: false,
			Note: "lazygit reads its configuration when it starts: reopen it so the new palette is the one drawn."}
	case "zellij":
		return themeReloadTool{Tool: "zellij", Done: false,
			Note: "zellij reads its configuration when a session starts: a session that is already open keeps the old theme until it is restarted."}
	case "delta":
		return themeReloadTool{Tool: "delta", Done: true,
			Note: "nothing has to be restarted: git runs delta on the next diff, and that run reads the new colours."}
	case "btop":
		return themeReloadTool{Tool: "btop", Done: false,
			Note: "btop reads its configuration when it starts: reopen it so the new palette is the one drawn."}
	case "fish":
		return themeReloadTool{Tool: "fish", Done: false,
			Note: "a fish shell that is already open keeps the old palette: open a new shell, or run `exec fish`."}
	case "nushell":
		return themeReloadTool{Tool: "nushell", Done: false,
			Note: "a nushell that is already open keeps the old palette: open a new shell, or run `exec nu`."}
	case "bat":
		if themeReloadCommandExists("bat") {
			batConfigDir := filepath.Join(homeDir, ".config", "bat")
			if _, err := themeReloadRun("bat cache --build", []string{"BAT_CONFIG_DIR=" + batConfigDir}); err == nil {
				return themeReloadTool{Tool: "bat", Done: false,
					Note: "bat's theme cache was rebuilt, so the new theme exists; open a new shell (or `exec zsh`) so BAT_THEME is read."}
			}
		}
		return themeReloadTool{Tool: "bat", Done: false,
			Note: "the new theme is not in bat's cache yet: run `bat cache --build`, then open a new shell so BAT_THEME is read."}
	case "tmux":
		if themeReloadCommandExists("tmux") {
			if _, err := themeReloadRun("tmux list-sessions", nil); err == nil {
				if _, err := themeReloadRun("tmux source-file "+shellSingleQuote(path), nil); err == nil {
					return themeReloadTool{Tool: "tmux", Done: true,
						Note: "sourced the config into the tmux server that is already running (`tmux source-file`)."}
				}
			}
		}
		return themeReloadTool{Tool: "tmux", Done: false,
			Note: "a tmux that is already running keeps the old style: run `tmux source-file ~/.tmux.conf`. A new server reads the file."}
	}
	return themeReloadTool{Tool: art.Tool, Done: false,
		Note: "what this tool needs in order to read the new theme is not known here; restart it after the change."}
}

// themeReloadResultParagraphs is what the picker says after a switch: one line
// per installed tool, marking what the installer did and what is left for the
// user. It carries the list because "applied" promised more than it delivered --
// the files changed, but a tool that was already running did not.
func themeReloadResultParagraphs(tools []themeReloadTool) []string {
	if len(tools) == 0 {
		return []string{"The theme is written to every file it owns. No tool that reads one of those files is installed, so there is nothing left to reload."}
	}
	paragraphs := []string{
		"The theme is written to every file it owns. Writing a file is not the same as the tool showing it, so here is each tool the switch painted, what the installer did for it, and what is left:",
	}
	for _, tool := range tools {
		mark := "→"
		if tool.Done {
			mark = "✓"
		}
		paragraphs = append(paragraphs, fmt.Sprintf("%s %s — %s", mark, tool.Tool, tool.Note))
	}
	paragraphs = append(paragraphs, "A tool that is not installed is not listed; a ✓ tool needs nothing more, and a → tool names the action that applies the theme.")
	return paragraphs
}

// ----------------------------------------------------------------------------
// Refreshing the managed files that predate the markers
// ----------------------------------------------------------------------------
//
// A machine installed before the generated blocks and the ownership marker
// carry files the switch cannot touch: they have neither marker nor block, so
// applyDotfilesTheme refuses them, and telling the user to reinstall is not an
// answer. The refresh adopts those files into the generated form. It is
// deliberately narrow, and it never runs by surprise:
//
//   - detection only reads, and names the files it would touch and where an
//     unowned one would be preserved before the user confirms anything;
//   - a file that may hold the user's own content is copied first, to the same
//     place the install steps use (~/.zshrc.d, ~/.config/fish/dotfiles.d) or to
//     the .bak-dotfiles-<stamp> the installer already writes;
//   - the previous bytes are recorded in the same theme.json the switch uses,
//     so Undo puts each file back byte-for-byte;
//   - it is gated on dryRun() like every other writer;
//   - a file that cannot be refreshed (unreadable, not regular, unrecognizable)
//     is named and skipped, and the rest of the refresh carries on.

// themeRefreshCandidate is one installed file the refresh would bring up to
// date. It is the list the review names before anything is written.
type themeRefreshCandidate struct {
	Tool string
	Path string
	// Reason is why the file is not up to date: no ownership marker, no generated
	// block, or a block that no longer matches its definition.
	Reason string
	// Preserve is the drop-in directory an unowned file is copied into first, or
	// empty when it is preserved beside itself as <path>.bak-dotfiles-<stamp>.
	Preserve string
	// PreserveLabel is the sentence the review shows for where the file goes,
	// with <stamp> standing in for the timestamp chosen at write time.
	PreserveLabel string
	// Problem, when set, is why this file cannot be refreshed. It is still named,
	// so the user is told rather than left wondering.
	Problem string
}

// themeInstalledFile is one installed path and the generated artifacts that live
// in it. A path can hold more than one (starship's palette line and its table,
// the zsh palette and the bat selection), so the file is read and written once
// and every artifact is applied to the same content.
type themeInstalledFile struct {
	Path      string
	Artifacts []themeArtifact
}

// themeInstalledFiles groups the active artifacts by the file they land in, in
// the order the artifacts are declared, so a shared file is not read twice.
func themeInstalledFiles(homeDir string) []themeInstalledFile {
	var files []themeInstalledFile
	index := map[string]int{}
	for _, art := range themeActiveArtifacts {
		path := themeInstalledPath(art, homeDir)
		if path == "" {
			continue
		}
		if i, ok := index[path]; ok {
			files[i].Artifacts = append(files[i].Artifacts, art)
			continue
		}
		index[path] = len(files)
		files = append(files, themeInstalledFile{Path: path, Artifacts: []themeArtifact{art}})
	}
	return files
}

// themeRefreshTarget picks the definition a refresh should write into one
// artifact. A file that already names a theme keeps it; a file from before the
// markers gets the artifact's own default when it declares a separate one, and
// the committed theme otherwise - never an arbitrary one.
func themeRefreshTarget(content string, art themeArtifact, defs []themeDefinition) (themeDefinition, bool) {
	if id, ok := themeBlockID(content, art.Block); ok {
		if def, found := themeByID(defs, id); found {
			if _, err := themeArtifactBlock(art, def); err == nil {
				return def, true
			}
		}
	}
	id := art.DefaultTheme
	if id == "" {
		id = defaultThemeID
	}
	def, found := themeByID(defs, id)
	if !found {
		return themeDefinition{}, false
	}
	if _, err := themeArtifactBlock(art, def); err != nil {
		return themeDefinition{}, false
	}
	return def, true
}

// themeRefreshContent replaces an artifact's generated block, or adopts the
// hand-written block of a file that predates the markers. It reports false when
// neither anchor exists, which is the file's content being unrecognizable.
func themeRefreshContent(content string, art themeArtifact, def themeDefinition) (string, bool) {
	block, err := themeArtifactBlock(art, def)
	if err != nil {
		return content, false
	}
	if updated, ok := replaceThemeBlock(content, block, art.Block); ok {
		return updated, true
	}
	if updated, ok := adoptThemeBlock(content, art, block); ok {
		return updated, true
	}
	return content, false
}

// themeFileRefreshPlan applies every artifact a file holds and reports the
// updated content, whether anything changed, and why the file cannot be
// refreshed when no artifact's anchor was found. An artifact whose anchor is
// absent is left as it is: a minimal hand-written file need not carry every
// region the generators write today.
func themeFileRefreshPlan(content string, arts []themeArtifact, defs []themeDefinition) (updated string, changed bool, problem string) {
	updated = content
	found := false
	for _, art := range arts {
		def, ok := themeRefreshTarget(updated, art, defs)
		if !ok {
			continue
		}
		next, ok := themeRefreshContent(updated, art, def)
		if !ok {
			continue
		}
		found = true
		if next != updated {
			changed = true
			updated = next
		}
	}
	if !found {
		return content, false, "its content is not a recognizable dotfiles theme block"
	}
	return updated, changed, ""
}

// themeFileRefreshReason names why a stale file is stale. A missing marker is
// reported first because it is the reason the switch refuses the file.
func themeFileRefreshReason(content string, arts []themeArtifact) string {
	if !strings.Contains(content, themeOwnershipMarker) {
		return "no dotfiles ownership marker"
	}
	for _, art := range arts {
		if _, ok := themeBlockID(content, art.Block); !ok {
			return "no generated theme block"
		}
	}
	return "its generated theme block is out of date"
}

// themeFilePreserveDir is the sourced drop-in directory an unowned file is
// copied into before it is replaced, or empty when it is preserved beside
// itself. The zsh and fish directories are the ones the install steps use, so a
// refreshed .zshrc and a refreshed config.fish land where the shell already
// sources them.
func themeFilePreserveDir(arts []themeArtifact, homeDir string) string {
	for _, art := range arts {
		switch art.Tool {
		case "zsh":
			return filepath.Join(homeDir, ".zshrc.d")
		case "fish":
			return filepath.Join(homeDir, ".config", "fish", "dotfiles.d")
		}
	}
	return ""
}

// themeFilePreserveLabel is the review's sentence for where a file goes, with
// <stamp> standing in for the timestamp the writer picks.
func themeFilePreserveLabel(arts []themeArtifact, homeDir, path string) string {
	if dir := themeFilePreserveDir(arts, homeDir); dir != "" {
		return "your current file is preserved first in " + dir + "/"
	}
	return "your current file is preserved first as " + path + ".bak-dotfiles-<stamp>"
}

// preserveThemeFile copies an unowned file to the place the install steps use
// before the refresh replaces it, and returns where it went. The drop-in dirs
// are shared with the shell install; everything else is preserved beside itself
// as the .bak the installer already writes.
func preserveThemeFile(file themeInstalledFile, homeDir, content string) (string, error) {
	first := file.Artifacts[0]
	if dir := themeFilePreserveDir(file.Artifacts, homeDir); dir != "" {
		ext := ".zsh"
		if first.Tool == "fish" {
			ext = ".fish"
		}
		marker := first.Comment + " " + themeOwnershipMarker + " " + first.Tool
		return system.PreserveUserConfig(file.Path, marker, dir, "dotfiles-user-config", ext)
	}
	stamp := time.Now().Format("20060102-150405")
	backup := fmt.Sprintf("%s.bak-dotfiles-%s", file.Path, stamp)
	if err := os.WriteFile(backup, []byte(content), 0o600); err != nil {
		return "", err
	}
	return backup, nil
}

// findThemeRefreshCandidates lists the installed theme files that are not in the
// generated form. It only reads: the decision to write is the user's, on the
// review this list feeds.
func findThemeRefreshCandidates(homeDir string, defs []themeDefinition) []themeRefreshCandidate {
	var out []themeRefreshCandidate
	for _, file := range themeInstalledFiles(homeDir) {
		tool := file.Artifacts[0].Tool
		info, err := os.Stat(file.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			out = append(out, themeRefreshCandidate{Tool: tool, Path: file.Path, Problem: "cannot inspect it: " + err.Error()})
			continue
		}
		if !info.Mode().IsRegular() {
			out = append(out, themeRefreshCandidate{Tool: tool, Path: file.Path, Problem: "it is not a regular file"})
			continue
		}
		data, err := os.ReadFile(file.Path)
		if err != nil {
			out = append(out, themeRefreshCandidate{Tool: tool, Path: file.Path, Problem: "cannot read it: " + err.Error()})
			continue
		}
		content := string(data)
		_, changed, problem := themeFileRefreshPlan(content, file.Artifacts, defs)
		if problem != "" {
			out = append(out, themeRefreshCandidate{Tool: tool, Path: file.Path, Problem: problem})
			continue
		}
		if !changed {
			continue
		}
		cand := themeRefreshCandidate{Tool: tool, Path: file.Path, Reason: themeFileRefreshReason(content, file.Artifacts)}
		if !strings.Contains(content, themeOwnershipMarker) {
			cand.Preserve = themeFilePreserveDir(file.Artifacts, homeDir)
			cand.PreserveLabel = themeFilePreserveLabel(file.Artifacts, homeDir, file.Path)
		}
		out = append(out, cand)
	}
	return out
}

// refreshThemeFiles brings the candidates the user confirmed up to date. It
// acts only on the files the review named, preserves each unowned file first,
// records the previous bytes, and never lets one file's failure abort the rest.
// It is gated on dryRun() like applyDotfilesTheme.
func refreshThemeFiles(homeDir string, defs []themeDefinition, candidates []themeRefreshCandidate) (*dotfilesThemeRecord, []string, string, error) {
	if dryRun() {
		SendLog("utilities", "DRY RUN: skipping the refresh of outdated theme files")
		return nil, nil, "DRY RUN: no theme file was refreshed.", nil
	}

	byPath := map[string]themeInstalledFile{}
	for _, file := range themeInstalledFiles(homeDir) {
		byPath[file.Path] = file
	}

	previous := map[string]string{}
	var written, preserved, failed, refreshed []string
	restoreWritten := func() {
		for _, path := range written {
			_ = os.WriteFile(path, []byte(previous[path]), 0o644)
		}
	}

	for _, cand := range candidates {
		if cand.Problem != "" {
			failed = append(failed, cand.Path+" — "+cand.Problem)
			continue
		}
		file, ok := byPath[cand.Path]
		if !ok {
			failed = append(failed, cand.Path+" — it is no longer an installed theme file")
			continue
		}
		data, err := os.ReadFile(file.Path)
		if err != nil {
			failed = append(failed, file.Path+" — cannot read it: "+err.Error())
			continue
		}
		original := string(data)
		updated, changed, problem := themeFileRefreshPlan(original, file.Artifacts, defs)
		if problem != "" {
			failed = append(failed, file.Path+" — "+problem)
			continue
		}
		if !changed {
			continue
		}
		if !strings.Contains(original, themeOwnershipMarker) {
			where, err := preserveThemeFile(file, homeDir, original)
			if err != nil {
				failed = append(failed, file.Path+" — could not preserve it first: "+err.Error())
				continue
			}
			preserved = append(preserved, where)
		}
		if err := os.WriteFile(file.Path, []byte(updated), 0o644); err != nil {
			failed = append(failed, file.Path+" — could not write it: "+err.Error())
			continue
		}
		previous[file.Path] = original
		written = append(written, file.Path)
		refreshed = append(refreshed, file.Path)
	}

	notice := "No theme file needed refreshing; every file was left as it is."
	switch {
	case len(written) > 0 && len(failed) > 0:
		notice = fmt.Sprintf("Refreshed %d theme file(s); %d could not be refreshed.", len(written), len(failed))
	case len(written) > 0:
		notice = fmt.Sprintf("Refreshed %d theme file(s). Use Undo to put them back.", len(written))
	case len(failed) > 0:
		notice = fmt.Sprintf("No theme file was refreshed; %d could not be refreshed.", len(failed))
	}

	if len(written) == 0 {
		return nil, themeRefreshResultParagraphs(nil, nil, failed), notice, nil
	}

	rec := &dotfilesThemeRecord{Theme: "refresh", Files: previous, AppliedAt: time.Now()}
	if err := writeDotfilesThemeRecord(rec); err != nil {
		restoreWritten()
		return nil, nil, "", fmt.Errorf("the files were refreshed but the bytes they replaced could not be recorded, "+
			"so the change cannot be undone; the files were put back: %w", err)
	}
	return rec, themeRefreshResultParagraphs(refreshed, preserved, failed), notice, nil
}

// themeRefreshResultParagraphs is what the review says once a refresh has run:
// which files were refreshed, where the user's own files were preserved, and
// which files were left alone and why.
func themeRefreshResultParagraphs(refreshed, preserved, failed []string) []string {
	var paragraphs []string
	if len(refreshed) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("Refreshed %d theme file(s):", len(refreshed)))
		for _, path := range refreshed {
			paragraphs = append(paragraphs, "  • "+path)
		}
	}
	if len(preserved) > 0 {
		paragraphs = append(paragraphs, "Your previous files were preserved at:")
		for _, path := range preserved {
			paragraphs = append(paragraphs, "  • "+path)
		}
	}
	if len(failed) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%d file(s) could not be refreshed and were left alone:", len(failed)))
		for _, failure := range failed {
			paragraphs = append(paragraphs, "  • "+failure)
		}
	}
	if len(refreshed) > 0 {
		paragraphs = append(paragraphs, "The previous bytes are recorded, so Undo puts each file back exactly as it was.")
	}
	return paragraphs
}

// themeRefreshReviewParagraphs is what the review says before anything is
// written: exactly which files a refresh would touch, why, and where each file
// that may be the user's would be preserved first.
func themeRefreshReviewParagraphs(candidates []themeRefreshCandidate) []string {
	var refreshable, blocked []themeRefreshCandidate
	for _, cand := range candidates {
		if cand.Problem != "" {
			blocked = append(blocked, cand)
			continue
		}
		refreshable = append(refreshable, cand)
	}

	var paragraphs []string
	if len(refreshable) > 0 {
		paragraphs = append(paragraphs, "Refresh will bring these managed files that are not up to date. Nothing has been written yet:")
		for _, cand := range refreshable {
			line := "  • " + cand.Path + " — " + cand.Reason
			if cand.PreserveLabel != "" {
				line += "; " + cand.PreserveLabel
			} else {
				line += "; it carries the dotfiles marker, so it is refreshed in place"
			}
			paragraphs = append(paragraphs, line)
		}
		paragraphs = append(paragraphs, "The previous bytes are recorded before anything is written, so Undo can put them back. "+
			"Choose “Yes, refresh” to write; choose Cancel to leave every file as it is.")
	}
	if len(blocked) > 0 {
		paragraphs = append(paragraphs, "These files cannot be refreshed and are left exactly as they are:")
		for _, cand := range blocked {
			paragraphs = append(paragraphs, "  • "+cand.Path+" — "+cand.Problem)
		}
	}
	return paragraphs
}

// renderHerdrTheme renders the [theme.custom] overrides and the [ui] accent of
// dotfiles-herdr/config.toml. Herdr's [theme] name is left as the built-in base
// it is: [theme.custom] is an override layer, and the tokens this repository
// owns are the ones generated here.
func renderHerdrTheme(def themeDefinition) (string, error) {
	selection, err := themeHex(def, "selection")
	if err != nil {
		return "", err
	}
	blue, err := themeHex(def, "blue")
	if err != nil {
		return "", err
	}
	text, err := themeHex(def, "text")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`# Token overrides on top of that theme, so the chrome Herdr draws agrees with the
# terminal palette (Alacritty, Kitty, WezTerm, Ghostty) and with the zsh prompt,
# which all read the same values from themes/%s.toml.
[theme.custom]
# "reset" is the terminal's own background. Without it Herdr paints its own
# near-black behind every pane, which is a second black sitting next to the
# terminal's.
panel_bg = "reset"
# The value the terminal uses for a text selection, so selecting inside a pane
# and selecting inside the terminal look like the same operation.
selection_bg = "%s"
# The body text. Herdr exposes four custom tokens and this is the fourth
# (panel_bg, selection_bg, accent and text; herdr config check rejects any
# other name under [theme.custom]), so the custom layer is complete here and the
# [theme] name below stays the built-in base it always was.
text = "%s"

[ui]
# Herdr's accent for highlights, borders and navigation UI: the palette's blue,
# the one the zsh prompt already uses for the directory.
accent = "%s"`, def.ID, selection, text, blue), nil
}

// renderLazygitTheme renders the gui.theme block of dotfiles-lazygit/lazygit/
// config.yml. lazygit ships no palette that follows the terminal, so the git UI
// kept its own colours beside every themed terminal. Every role below is one the
// definition holds, and the switch that paints the terminals paints this too.
func renderLazygitTheme(def themeDefinition) (string, error) {
	roles := []string{"blue", "bright_black", "selection_text", "selection", "green", "red", "text"}
	values := make(map[string]string, len(roles))
	for _, role := range roles {
		value, err := themeHex(def, role)
		if err != nil {
			return "", err
		}
		values[role] = value
	}

	return fmt.Sprintf(`  theme:
    activeBorderColor:
      - "%s"
      - bold
    inactiveBorderColor:
      - "%s"
    optionsTextColor:
      - "%s"
    selectedLineFgColor:
      - "%s"
    selectedLineBgColor:
      - "%s"
    cherryPickedCommitFgColor:
      - "%s"
    cherryPickedCommitBgColor:
      - "%s"
    unstagedChangesColor:
      - "%s"
    defaultFgColor:
      - "%s"
`,
		values["blue"], values["bright_black"], values["blue"],
		values["selection_text"], values["selection"],
		values["green"], values["selection"], values["red"], values["text"],
	), nil
}

// renderZellijTheme renders the themes block of dotfiles-zellij/zellij/config.kdl.
// Zellij paints its own chrome from a named theme, and the file used to carry five
// hand-written palettes with `theme "kanagawa_wave"` selected by hand: the
// multiplexer stayed on Kanagawa whatever the rest of the machine wore. The block
// below builds one theme from the definition under its own name and selects it, so
// the switch that paints the terminals paints the panes too.
func renderZellijTheme(def themeDefinition) (string, error) {
	roles := []string{"text", "base", "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "bright_yellow"}
	values := make(map[string]string, len(roles))
	for _, role := range roles {
		value, err := themeHex(def, role)
		if err != nil {
			return "", err
		}
		values[role] = value
	}
	// A KDL identifier may not carry the hyphen the theme ids use, so the name is
	// the id with its underscores; the selected name and the block agree because
	// both come from this one string.
	name := strings.ReplaceAll(def.ID, "-", "_")

	return fmt.Sprintf(`themes {
    %s {
        fg "%s"
        bg "%s"
        black "%s"
        red "%s"
        green "%s"
        yellow "%s"
        blue "%s"
        magenta "%s"
        cyan "%s"
        white "%s"
        orange "%s"
    }
}

theme "%s"
`, name,
		values["text"], values["base"], values["black"], values["red"],
		values["green"], values["yellow"], values["blue"], values["magenta"],
		values["cyan"], values["white"], values["bright_yellow"], name,
	), nil
}

// themeBtopChrome maps btop's per-surface theme[...] keys onto palette roles, in
// the order btop's own bundled .theme files write them. btop reads a palette from
// a theme file rather than from the terminal, and its keys are not the canonical
// roles: the mapping is data here for the same reason bat's is, because a key name
// does not say which role it takes.
//
//	key            role            what it paints
//	main_bg        base            the background of every box
//	main_fg        text            the body text
//	title          blue            box titles
//	hi_fg          blue            the highlight on a keyboard shortcut
//	selected_bg    selection       the selected row's background
//	selected_fg    selection_text  the selected row's text
//	inactive_fg    bright_black    disabled and inactive text
//	div_line       bright_black    the divider and small-box lines
//	proc_misc      cyan            the processes box's mini graphs and status text
//	cpu_box        green           the CPU box outline
//	mem_box        yellow          the memory/disks box outline
//	net_box        cyan            the network box outline
//	proc_box       blue            the processes box outline
//
// The gradient keys below are the five accent roles read as a scale: a quantity
// that is worse as it grows (cpu load, temperature, used memory, running
// processes) ramps green -> yellow -> red, a quantity that is better as it grows
// (free and available memory) ramps the same scale the other way round, and the
// neutral gauges take one end of it (cached cold in blue/cyan, upload warm in
// yellow/red). No value is chosen here: every one of them is a role the
// definition holds, which is what TestGeneratedThemeBlocksInventNoColour reads.
var themeBtopChrome = []struct{ Key, Role string }{
	{"main_bg", "base"},
	{"main_fg", "text"},
	{"title", "blue"},
	{"hi_fg", "blue"},
	{"selected_bg", "selection"},
	{"selected_fg", "selection_text"},
	{"inactive_fg", "bright_black"},
	{"proc_misc", "cyan"},
	{"cpu_box", "green"},
	{"mem_box", "yellow"},
	{"net_box", "cyan"},
	{"proc_box", "blue"},
	{"div_line", "bright_black"},
}

// themeBtopGradients are the keys btop ramps between two extremes, read from the
// same five accent roles. They are a separate table because they are a separate
// statement: the chrome above is one value per surface, and each line here is a
// scale.
var themeBtopGradients = []struct{ Key, Role string }{
	{"temp_start", "green"},
	{"temp_mid", "yellow"},
	{"temp_end", "red"},
	{"cpu_start", "green"},
	{"cpu_mid", "yellow"},
	{"cpu_end", "red"},
	{"free_start", "red"},
	{"free_mid", "yellow"},
	{"free_end", "green"},
	{"cached_start", "blue"},
	{"cached_mid", "cyan"},
	{"cached_end", "cyan"},
	{"available_start", "red"},
	{"available_mid", "yellow"},
	{"available_end", "green"},
	{"used_start", "green"},
	{"used_mid", "yellow"},
	{"used_end", "red"},
	{"download_start", "cyan"},
	{"download_mid", "blue"},
	{"download_end", "blue"},
	{"upload_start", "yellow"},
	{"upload_mid", "red"},
	{"upload_end", "red"},
	{"process_start", "green"},
	{"process_mid", "yellow"},
	{"process_end", "red"},
}

// btopThemeFile is the name btop reads a theme from, which is the theme id with
// btop's own ".theme" suffix: color_theme names the file, not a path, and btop
// appends the suffix itself. The id comes from a file in themes/, so it is
// checked before it is used to build a path rather than trusted.
func btopThemeFile(id string) (string, error) {
	name := id + ".theme"
	if id == "" || filepath.Base(name) != name {
		return "", fmt.Errorf("theme %q is not a plain theme id, so btop's theme file cannot be named", id)
	}
	return name, nil
}

// renderBtopTheme renders a btop .theme file from the definition. Every colour is
// a role the definition holds, and a definition missing one is refused rather
// than emitted with a hole. btop resolves color_theme against its own themes
// directory, so this file's name is what the selection in btop.conf names.
func renderBtopTheme(def themeDefinition) (string, error) {
	file, err := btopThemeFile(def.ID)
	if err != nil {
		return "", err
	}

	values := make(map[string]string, len(themeBtopChrome)+len(themeBtopGradients))
	for _, table := range [][]struct{ Key, Role string }{themeBtopChrome, themeBtopGradients} {
		for _, entry := range table {
			value, err := themeHex(def, entry.Role)
			if err != nil {
				return "", err
			}
			values[entry.Key] = value
		}
	}

	var b strings.Builder
	b.WriteString("# dotfiles-managed-config: btop\n")
	fmt.Fprintf(&b, "# btop theme %q, generated from themes/%s.toml; edit the definition, not this file.\n", def.ID, def.ID)
	fmt.Fprintf(&b, "# btop.conf selects it by this file's name (%s); btop reads the directory\n", file)
	b.WriteString("# ~/.config/btop/themes for it.\n")
	b.WriteString("\n# The chrome every box shares.\n")
	for _, entry := range themeBtopChrome {
		fmt.Fprintf(&b, "theme[%s]=\"%s\"\n", entry.Key, values[entry.Key])
	}
	b.WriteString("\n# The gradients, in btop's own order: green -> yellow -> red as a quantity\n")
	b.WriteString("# gets worse, and the same scale the other way round where more is better.\n")
	for _, entry := range themeBtopGradients {
		fmt.Fprintf(&b, "theme[%s]=\"%s\"\n", entry.Key, values[entry.Key])
	}
	return b.String(), nil
}

// renderBtopSelection renders the color_theme line of
// dotfiles-btop/btop/btop.conf. btop ships no configuration, so this is the only
// setting the repository owns in that file: everything else btop ships a default
// for, and a key written here would be a second copy of a btop default that no
// theme reads. The line is refused when the theme's file cannot be generated, so
// the selection never names a theme btop has no .theme for.
func renderBtopSelection(def themeDefinition) (string, error) {
	if _, err := btopThemeFile(def.ID); err != nil {
		return "", err
	}
	if _, err := renderBtopTheme(def); err != nil {
		return "", err
	}
	return fmt.Sprintf("color_theme = \"%s\"\n", def.ID), nil
}

// renderDeltaTheme renders the [delta] section of .gitconfig. delta is the git pager
// the configuration selects, and it used to keep its own defaults: the diff a user
// read was not the palette the rest of the machine wore. The syntax theme is the bat
// theme the install builds, so a diff highlights with the same .tmTheme the shell
// selects BAT_THEME from, and the diff's own marks are palette roles.
func renderDeltaTheme(def themeDefinition) (string, error) {
	batTheme, err := batSelectionName(def)
	if err != nil {
		return "", err
	}
	roles := []string{"red", "green", "blue", "yellow", "bright_black"}
	values := make(map[string]string, len(roles))
	for _, role := range roles {
		value, err := themeHex(def, role)
		if err != nil {
			return "", err
		}
		values[role] = value
	}

	return fmt.Sprintf(`[delta]
	# The syntax highlighting is bat's: delta reads bat's assets, and this is the same
	# .tmTheme file the shell selects BAT_THEME from.
	syntax-theme = %s
	minus-style = "normal %s"
	minus-emph-style = "bold %s"
	plus-style = "normal %s"
	plus-emph-style = "bold %s"
	file-style = "bold %s"
	file-decoration-style = "ul"
	hunk-header-style = "file line-number %s"
	hunk-header-decoration-style = "box"
	line-numbers-left-style = "%s"
	line-numbers-right-style = "%s"
	line-numbers-minus-style = "%s"
	line-numbers-plus-style = "%s"
	line-numbers-zero-style = "%s"`,
		batTheme,
		values["red"], values["red"],
		values["green"], values["green"],
		values["blue"], values["yellow"],
		values["bright_black"], values["bright_black"],
		values["red"], values["green"], values["bright_black"],
	), nil
}

// renderNushellBatSelection renders the $env.BAT_THEME assignment of
// dotfiles-nushell/config.nu. It is a second block rather than a line of the theme
// record because the record's closing brace sits outside its markers; the shell
// selects the same bat theme the zsh and fish blocks select, so the fzf previews
// that call bat follow the theme here too.
func renderNushellBatSelection(def themeDefinition) (string, error) {
	selected, err := batSelectionName(def)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$env.BAT_THEME = %q", selected), nil
}

// renderZellijLayout renders dotfiles-zellij/zellij/layouts/work.kdl, the one
// layout the configuration loads. The zjstatus bar is the part of Zellij that is not
// the config's `theme`: its segments are painted by inline hex in the layout, so the
// five per-theme layouts each carried their own palette and the default one was
// selected by hand in `default_layout`. This one is generated from the definition,
// and every colour in it is a role the definition holds.
func renderZellijLayout(def themeDefinition) (string, error) {
	hex := func(role string) (string, error) { return themeHex(def, role) }
	values := map[string]string{}
	for _, role := range []string{"text", "bright_black", "blue", "magenta", "yellow", "green"} {
		value, err := hex(role)
		if err != nil {
			return "", err
		}
		values[role] = value
	}

	return fmt.Sprintf(`layout {
    tab name="base" focus=true {
        pane
    }

    default_tab_template {
        pane size=1 borderless=true {
            plugin location="file:~/.config/zellij/plugins/zjstatus.wasm" {
              format_left   "{mode} #[fg=%s,bold]{tabs}"
              format_center "#[fg=%s,bold]{session}"
              format_right  "{command_git_branch} {datetime}"
              format_space  ""

              border_enabled  "false"
              border_char     "─"
              border_format   "#[fg=%s]{char}"
              border_position "top"

              mode_normal  "#[bg=%s] "
              mode_tmux    "#[bg=%s] "

              tab_normal   "#[fg=%s]{index}:{name} "
              tab_active   "#[fg=%s,bold,italic]{index}:{name} "

              command_git_branch_command     "git rev-parse --abbrev-ref HEAD"
              command_git_branch_format      "#[fg=%s] {stdout} "
              command_git_branch_interval    "10"
              command_git_branch_rendermode  "static"

              datetime        "#[fg=%s,bold] {format} "
              datetime_format "%%A, %%d %%b %%Y %%H:%%M"
              datetime_timezone "Europe/Berlin"
            }
        }
        children
        pane size=1 borderless=true  {
          plugin location="zellij:status-bar"
        }
    }
}`,
		values["text"], values["text"], values["bright_black"],
		values["blue"], values["magenta"],
		values["bright_black"], values["yellow"], values["green"], values["bright_black"],
	), nil
}

// renderFishTheme renders a whole fish theme file. The fish roles are not the
// canonical ones, so a theme carries them in its [fish] table; a theme with any
// of them missing is refused rather than emitted with a hole.
func renderFishTheme(def themeDefinition) (string, error) {
	for _, role := range themeFishRoles {
		if def.Fish[role] == "" {
			return "", fmt.Errorf("theme %q defines no fish %q, so its fish theme cannot be generated", def.ID, role)
		}
	}

	return fmt.Sprintf(`# dotfiles-managed-config: fish
# name: %s Fish shell theme
# generated from themes/%s.toml; edit the definition, not this file

fish_color_normal %s
fish_color_command %s
fish_color_keyword %s
fish_color_quote %s
fish_color_redirection %s
fish_color_end %s
fish_color_error %s
fish_color_param %s
fish_color_comment %s
fish_color_selection --background=%s
fish_color_search_match --background=%s
fish_color_operator %s
fish_color_escape %s
fish_color_autosuggestion %s

# Completion Pager Colors
fish_pager_color_progress %s
fish_pager_color_prefix %s
fish_pager_color_completion %s
fish_pager_color_description %s`,
		def.Name, def.ID,
		def.Fish["normal"], def.Fish["command"], def.Fish["keyword"], def.Fish["quote"],
		def.Fish["redirection"], def.Fish["end"], def.Fish["error"], def.Fish["param"],
		def.Fish["comment"], def.Fish["selection"], def.Fish["search_match"], def.Fish["operator"],
		def.Fish["escape"], def.Fish["autosuggestion"], def.Fish["pager_progress"], def.Fish["pager_prefix"],
		def.Fish["pager_completion"], def.Fish["pager_description"]), nil
}

// themeFishRoles is every fish role a generated fish theme file needs, in the
// order the file writes them.
var themeFishRoles = []string{
	"normal", "command", "keyword", "quote", "redirection", "end", "error",
	"param", "comment", "selection", "search_match", "operator", "escape",
	"autosuggestion", "pager_progress", "pager_prefix", "pager_completion", "pager_description",
}

// themeFishDerivation is the mechanical mapping from the canonical palette to
// fish's own role names, written down as code so a theme that ships no [fish]
// table (catppuccin-mocha) is derived from its palette rather than left out or
// filled by eye. Every palette role it names already holds a value the
// definition was given, so a derived fish colour is never an invented one; it is
// the same mapping the dotfiles definition records in its [fish] table.
var themeFishDerivation = map[string]string{
	"normal":            "text",
	"command":           "green",
	"keyword":           "magenta",
	"quote":             "yellow",
	"redirection":       "text",
	"end":               "cyan",
	"error":             "red",
	"param":             "blue",
	"comment":           "bright_black",
	"selection":         "selection",
	"search_match":      "selection",
	"operator":          "green",
	"escape":            "magenta",
	"autosuggestion":    "bright_black",
	"pager_progress":    "bright_black",
	"pager_prefix":      "green",
	"pager_completion":  "text",
	"pager_description": "bright_black",
}

// themeFishRolesFor returns the fish roles a definition paints. A definition
// that carries a complete [fish] table (the partial themes transcribed from their
// own files) uses it; one that does not derives each role from its canonical
// palette. A theme missing either half is refused rather than emitted with a
// hole, which is what lets the menu report fish honestly instead of drawing a
// half palette.
func themeFishRolesFor(def themeDefinition) (map[string]string, error) {
	complete := true
	for _, role := range themeFishRoles {
		if def.Fish[role] == "" {
			complete = false
			break
		}
	}
	if complete {
		return def.Fish, nil
	}

	derived := make(map[string]string, len(themeFishRoles))
	for _, role := range themeFishRoles {
		paletteRole := themeFishDerivation[role]
		value := def.Palette[paletteRole]
		if value == "" {
			return nil, fmt.Errorf("theme %q defines neither fish role %q nor palette role %q, so its fish colours cannot be generated",
				def.ID, role, paletteRole)
		}
		derived[role] = strings.TrimPrefix(value, "#")
	}
	return derived, nil
}

// renderFishConfig renders the fish palette block of dotfiles-fish/fish/config.fish.
// fish's active theme is otherwise the user's own state: fish keeps the colour
// variables a `fish_config theme choose` writes in fish_variables, which this
// repository does not own and the switch must not rewrite. Expressing the palette
// in the config file the repository does own is what makes the fish switch
// reversible, and the global scope is what makes it win over a universal choice
// made once through fish_config.
func renderFishConfig(def themeDefinition) (string, error) {
	roles, err := themeFishRolesFor(def)
	if err != nil {
		return "", err
	}
	palette, err := themeShellPaletteFor(def)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", themeBox("#", "DOTFILES THEME"))
	fmt.Fprintf(&b, `# The fish palette. Generated from themes/%s.toml; edit the definition, not this
# block. These are global variables, so they also win over a universal colour the
# user chose once through fish_config; the switch never writes the user's own
# state in fish_variables.
`, def.ID)
	// The SGR form only: the two listing tables are what read it here, while the
	// shell's own roles below take the hex form the tool's variables hold.
	for _, role := range themeShellRoles {
		fmt.Fprintf(&b, "set -g PALETTE_%s_SGR \"%s\"\n", role.Name, palette.SGR[role.Name])
	}
	fmt.Fprintf(&b, "set -g PALETTE_SURFACE_BG_SGR \"%s\"\n", palette.SurfaceBgSGR)
	b.WriteString("# The two listing tables, from the same mapping the zsh block reads: fish's\n")
	b.WriteString("# spelling of a reference and of an export is the only difference, so a file\n")
	b.WriteString("# keeps the colour the other shell gives it and a switch moves both listings.\n")
	b.WriteString(themeListingTableForShell(zshLsColorsBlock, "fish"))
	b.WriteString(themeListingTableForShell(zshEzaColorsBlock, "fish"))
	fmt.Fprintf(&b, `

# The shell's own roles: the prompt, strings, errors, and the completion pager.
set -g fish_color_normal %s
set -g fish_color_command %s
set -g fish_color_keyword %s
set -g fish_color_quote %s
set -g fish_color_redirection %s
set -g fish_color_end %s
set -g fish_color_error %s
set -g fish_color_param %s
set -g fish_color_comment %s
set -g fish_color_selection --background=%s
set -g fish_color_search_match --background=%s
set -g fish_color_operator %s
set -g fish_color_escape %s
set -g fish_color_autosuggestion %s
# The matching-bracket colour: the shipped fish_variables pinned it to a zenburn
# blue no definition holds, and it is not one of the eighteen roles above, so it
# takes the selection colour here like search_match does.
set -g fish_color_match %s

# Completion pager colours.
set -g fish_pager_color_progress %s
set -g fish_pager_color_prefix %s
set -g fish_pager_color_completion %s
set -g fish_pager_color_description %s`,
		roles["normal"], roles["command"], roles["keyword"], roles["quote"],
		roles["redirection"], roles["end"], roles["error"], roles["param"],
		roles["comment"], roles["selection"], roles["search_match"], roles["operator"],
		roles["escape"], roles["autosuggestion"], roles["search_match"], roles["pager_progress"], roles["pager_prefix"],
		roles["pager_completion"], roles["pager_description"])

	// fish selects the bat theme too, so `bat` and the fzf previews that call it
	// follow the theme in this shell the way the zsh block already makes them
	// follow it. It is the same file the zsh selection names, read through
	// batSelectionName so the two shells cannot select two different names.
	batSelected, err := batSelectionName(def)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, `

# The bat theme this shell selects: the same file the zsh block names, so bat and
# the fzf previews that call it follow the theme in this shell too.
set -gx BAT_THEME "%s"`, batSelected)

	return strings.TrimRight(b.String(), "\n"), nil
}

// renderTmuxTheme renders tmux's own style options from the canonical palette.
// tmux's only theme in this repository is the name of the kanagawa plugin, and
// Kanagawa is partial, so this block invents no theme: it is this repository's
// palette applied to tmux's status bar, windows, panes and copy-mode. The block
// is placed after the TPM run line (pinned by TestTmuxThemeBlockLoadsAfterPlugins)
// because tmux runs run-shell synchronously: the plugin styles are already
// written when this block is read, so these options win.
func renderTmuxTheme(def themeDefinition) (string, error) {
	for _, name := range []string{"base", "text", "blue", "bright_black", "yellow", "red", "green", "selection"} {
		if _, err := themeHex(def, name); err != nil {
			return "", err
		}
	}
	hex := func(name string) string {
		value, _ := themeHex(def, name)
		return value
	}

	return fmt.Sprintf(`%s
# tmux's own style options, painted from themes/%s.toml. tmux's theme in this
# repository used to be only the kanagawa plugin's name, and Kanagawa is partial,
# so this is the palette applied to tmux itself rather than a second theme.
# It sits after the TPM run line on purpose: tmux runs run-shell
# synchronously, so the plugins TPM sources have already written their styles
# when this block is read, and these options win.
set -g status-style "fg=%s,bg=%s"
set -g status-left-style "fg=%s,bg=%s,bold"
set -g status-right-style "fg=%s,bg=%s"
set -g message-style "fg=%s,bg=%s"
set -g message-command-style "fg=%s,bg=%s"
setw -g window-status-style "fg=%s,bg=%s"
setw -g window-status-current-style "fg=%s,bg=%s,bold"
setw -g window-status-activity-style "fg=%s,bg=%s"
setw -g window-status-bell-style "fg=%s,bg=%s"
setw -g window-status-last-style "fg=%s,bg=%s"
setw -g pane-border-style "fg=%s"
setw -g pane-active-border-style "fg=%s"
setw -g copy-mode-match-style "fg=%s,bg=%s"
setw -g copy-mode-current-match-style "fg=%s,bg=%s"
set -g mode-style "fg=%s,bg=%s"
set -g display-panes-colour "%s"
set -g display-panes-active-colour "%s"
set -g clock-mode-colour "%s"`,
		themeBox("#", "DOTFILES THEME"),
		def.ID,
		hex("text"), hex("base"),
		hex("base"), hex("blue"),
		hex("bright_black"), hex("base"),
		hex("base"), hex("yellow"),
		hex("base"), hex("yellow"),
		hex("bright_black"), hex("base"),
		hex("base"), hex("blue"),
		hex("yellow"), hex("base"),
		hex("red"), hex("base"),
		hex("green"), hex("base"),
		hex("bright_black"),
		hex("blue"),
		hex("text"), hex("selection"),
		hex("base"), hex("yellow"),
		hex("text"), hex("selection"),
		hex("bright_black"),
		hex("blue"),
		hex("blue")), nil
}

// ---------------------------------------------------------------------------
// nushell
// ---------------------------------------------------------------------------
//
// nushell's colours live in the shell's own config.nu, in two regions this
// repository owns: the `$env.LS_COLORS` table nushell paints `ls` output from,
// and the `let dark_theme` record the shipped configuration selects with
// `color_config: $dark_theme`. Neither is a published palette of its own, so
// both are derived from the definition by the mapping below, exactly as the fish
// roles and tmux's style options are: every value emitted is a role the
// definition already holds, and a definition that cannot fill one is refused
// rather than painted with a hole.
//
// The two regions were hand-written Kanagawa values until issue #240: eighteen
// colours that appear in no themes/*.toml, kept while every other tool changed,
// and invisible to the coverage row because nushell was not one of the tools the
// model names. Two guards hold the fix: a generated block may not invent a
// colour (TestGeneratedThemeBlocksInventNoColour), and the nushell block must
// carry the definition's own palette and none of those eighteen
// (TestNushellIsPaintedByTheThemeSwitch).

// themeNushellLSColours is the LS_COLORS table nushell paints `ls` output from,
// as key -> the canonical role it takes. The keys and their order are the ones
// the shipped dotfiles-nushell/config.nu already carried, so the fix changes the
// colours and not which file kinds the shell paints.
var themeNushellLSColours = []struct{ Key, Role string }{
	{"di", "blue"},    // directories
	{"fi", "text"},    // regular files
	{"ln", "magenta"}, // symbolic links
	{"ex", "cyan"},    // executables
	{"or", "red"},     // broken links
	{"*.txt", "text"},
	{"*.jpg", "magenta"},
	{"*.png", "magenta"},
	{"*.zip", "cyan"},
	{"*.gz", "cyan"},
	{"*.tar", "cyan"},
	{"*.log", "magenta"},
	{"*.md", "magenta"},
	{"*.py", "cyan"},
	{"*.rs", "red"},
	{"*.sh", "cyan"},
	{"*", "text"},
}

// themeNushellRoles is every key of nushell's `let dark_theme` record, in the
// record's own order, and the canonical role it takes. BgRole is set for the two
// keys nushell writes as a `{ fg, bg }` record; Attr is nushell's own attribute,
// appended to a colour as the `_bold` suffix nushell also accepts. A key with no
// role is one that carries no colour (leading_trailing_space_bg).
var themeNushellRoles = []struct {
	Key    string
	Role   string
	BgRole string
	Attr   string
}{
	{"separator", "bright_black", "", ""},
	{"leading_trailing_space_bg", "", "", "n"},
	{"header", "blue", "", "b"},
	{"empty", "magenta", "", ""},
	{"bool", "red", "", ""},
	{"int", "bright_black", "", ""},
	{"filesize", "cyan", "", ""},
	{"duration", "bright_green", "", ""},
	{"date", "bright_yellow", "", ""},
	{"range", "bright_black", "", ""},
	{"float", "yellow", "", ""},
	{"string", "bright_black", "", ""},
	{"nothing", "blue", "", ""},
	{"binary", "cyan", "", ""},
	{"cellpath", "bright_green", "", ""},
	{"row_index", "blue", "", "b"},
	{"record", "magenta", "", ""},
	{"list", "bright_black", "", ""},
	{"block", "magenta", "", "b"},
	{"hints", "bright_green", "", ""},
	{"search_result", "base", "red", ""},
	{"shape_and", "magenta", "", "b"},
	{"shape_binary", "cyan", "", "b"},
	{"shape_block", "blue", "", ""},
	{"shape_bool", "red", "", ""},
	{"shape_closure", "yellow", "", ""},
	{"shape_custom", "cyan", "", ""},
	{"shape_datetime", "bright_yellow", "", "b"},
	{"shape_directory", "blue", "", ""},
	{"shape_external", "cyan", "", ""},
	{"shape_externalarg", "magenta", "", "b"},
	{"shape_filepath", "bright_green", "", ""},
	{"shape_flag", "blue", "", "b"},
	{"shape_float", "yellow", "", ""},
	{"shape_garbage", "base", "yellow", "b"},
	{"shape_globpattern", "cyan", "", "b"},
	{"shape_int", "magenta", "", ""},
	{"shape_internalcall", "cyan", "", "b"},
	{"shape_keyword", "blue", "", ""},
	{"shape_literal", "bright_yellow", "", ""},
	{"shape_operator", "red", "", ""},
	{"shape_or", "red", "", "b"},
	{"shape_pipe", "cyan", "", ""},
	{"shape_string", "bright_green", "", ""},
	{"shape_variable", "yellow", "", ""},
}

// themeRGB renders a #rrggbb colour as the decimal `r;g;b` triple LS_COLORS
// carries, so the table is written the way the shell reads it without a second
// colour being chosen for it.
func themeRGB(value string) (string, error) {
	hex := strings.TrimPrefix(value, "#")
	if len(hex) != 6 {
		return "", fmt.Errorf("%q is not a #rrggbb colour", value)
	}
	parts := make([]string, 3)
	for i := 0; i < 3; i++ {
		n, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return "", fmt.Errorf("%q is not a #rrggbb colour: %w", value, err)
		}
		parts[i] = strconv.FormatUint(n, 10)
	}
	return strings.Join(parts, ";"), nil
}

// renderNushellTheme renders the colour region of dotfiles-nushell/config.nu:
// the LS_COLORS table and the `let dark_theme` record the shipped configuration
// selects. The record's opening line and its closing brace are the file's own
// and stay outside the marked region, so the block ends at the record's last
// key. A definition that misses any role either half reads is refused, so
// nushell is reported by the coverage row instead of painted with a hole.
func renderNushellTheme(def themeDefinition) (string, error) {
	colour := func(role string) (string, error) {
		value, err := themeHex(def, role)
		if err != nil {
			return "", fmt.Errorf("theme %q cannot paint nushell: %w", def.ID, err)
		}
		return strings.ToUpper(value), nil
	}

	var b strings.Builder
	b.WriteString("$env.LS_COLORS = (\n")
	b.WriteString("    # Every file kind nushell paints `ls` output with, each one the palette\n")
	b.WriteString("    # role named beside it.\n")
	for i, entry := range themeNushellLSColours {
		value, err := colour(entry.Role)
		if err != nil {
			return "", err
		}
		rgb, err := themeRGB(value)
		if err != nil {
			return "", fmt.Errorf("theme %q cannot paint nushell: %w", def.ID, err)
		}
		switch {
		case entry.Key == "*":
			fmt.Fprintf(&b, "\n    # Everything else.\n    \"*=38;2;%s\"  # %s\n", rgb, entry.Role)
		case i == 0:
			fmt.Fprintf(&b, "    \"%s=38;2;%s:\" +  # %s\n", entry.Key, rgb, entry.Role)
		case entry.Key == "*.txt":
			fmt.Fprintf(&b, "\n    # The specific extensions.\n    \"%s=38;2;%s:\" +  # %s\n", entry.Key, rgb, entry.Role)
		default:
			fmt.Fprintf(&b, "    \"%s=38;2;%s:\" +  # %s\n", entry.Key, rgb, entry.Role)
		}
	}
	b.WriteString(")\n\n")

	b.WriteString("# The record the configuration selects with `color_config`, so these are the\n")
	b.WriteString("# colours the shell draws with. Every one is a role of the theme's own palette.\n")
	b.WriteString("let dark_theme = {\n")
	b.WriteString("    # Base elements.\n")
	for _, entry := range themeNushellRoles {
		switch {
		case entry.Role == "":
			fmt.Fprintf(&b, "    %s: { attr: \"%s\" } # no fg and no bg\n", entry.Key, entry.Attr)
		case entry.BgRole != "":
			fg, err := colour(entry.Role)
			if err != nil {
				return "", err
			}
			bg, err := colour(entry.BgRole)
			if err != nil {
				return "", err
			}
			attr := ""
			if entry.Attr != "" {
				attr = fmt.Sprintf(", attr: \"%s\"", entry.Attr)
			}
			fmt.Fprintf(&b, "    %s: { fg: \"%s\", bg: \"%s\"%s } # fg %s, bg %s\n", entry.Key, fg, bg, attr, entry.Role, entry.BgRole)
		default:
			value, err := colour(entry.Role)
			if err != nil {
				return "", err
			}
			suffix := ""
			if entry.Attr != "" {
				suffix = "_bold"
			}
			fmt.Fprintf(&b, "    %s: \"%s%s\" # %s\n", entry.Key, value, suffix, entry.Role)
		}
	}
	// The record's closing brace is the file's own line and stays outside the
	// markers, which is why the region's end anchor is the last key rather than a
	// `}` that is not unique in the file.
	return b.String(), nil
}

// themeThemeFile is a whole-file artifact, one per theme: the fish theme files
// are named after the theme they hold, so they are generated for every theme
// that has fish roles rather than swapped in place like the active artifacts.
// Tool says which tool the file belongs to, so coverage can ask whether a theme
// has a shipped file without re-parsing the path.
type themeThemeFile struct {
	Tool   string
	Theme  string
	Path   string
	Render func(themeDefinition) (string, error)
}

// themeThemeFiles are the per-theme files generated from the definitions for the
// tools whose artifact is a whole file rather than a block inside a file the
// switch rewrites: fish's own theme files, bat's .tmTheme files, and the Neovim
// colorschemes this repository has to generate for the themes whose plugin ships
// none. The paths keep the repository's own capitalisation.
//
// The bat entries are deliberately the files the repository ships under version
// control; the installer generates a .tmTheme for every theme that names one at
// install time (generateBatThemesFromDefinitions), so a theme with no committed
// .tmTheme still paints bat. Nocturne's is committed so its generated file is
// pinned byte-for-byte like the other two, exactly as its colorscheme is.
var themeThemeFiles = []themeThemeFile{
	{Tool: "fish", Theme: "dotfiles", Path: "dotfiles-fish/fish/themes/dotfiles.theme", Render: renderFishTheme},
	{Tool: "fish", Theme: "everforest", Path: "dotfiles-fish/fish/themes/Everforest.theme", Render: renderFishTheme},
	{Tool: "fish", Theme: "kanagawa", Path: "dotfiles-fish/fish/themes/Kanagawa.theme", Render: renderFishTheme},
	{Tool: "bat", Theme: "dotfiles", Path: "dotfiles-bat/themes/dotfiles.tmTheme", Render: renderBatTheme},
	{Tool: "bat", Theme: "catppuccin-mocha", Path: "dotfiles-bat/themes/catppuccin-mocha.tmTheme", Render: renderBatTheme},
	{Tool: "bat", Theme: "nocturne", Path: "dotfiles-bat/themes/nocturne.tmTheme", Render: renderBatTheme},
	// btop's committed theme is the default one; the other themes' .theme files are
	// written at install time (generateBtopThemesFromDefinitions) into btop's own
	// themes directory, which is where color_theme resolves a name.
	{Tool: "btop", Theme: "dotfiles", Path: "dotfiles-btop/btop/themes/dotfiles.theme", Render: renderBtopTheme},
	{Tool: "nvim", Theme: "dotfiles", Path: "dotfiles-nvim/nvim/colors/dotfiles.lua", Render: renderNvimTheme},
	{Tool: "nvim", Theme: "catppuccin-latte", Path: "dotfiles-nvim/nvim/colors/catppuccin-latte.lua", Render: renderNvimTheme},
	{Tool: "nvim", Theme: "everforest", Path: "dotfiles-nvim/nvim/colors/everforest.lua", Render: renderNvimTheme},
	{Tool: "nvim", Theme: "rose-pine", Path: "dotfiles-nvim/nvim/colors/rose-pine.lua", Render: renderNvimTheme},
	{Tool: "nvim", Theme: "nocturne", Path: "dotfiles-nvim/nvim/colors/nocturne.lua", Render: renderNvimTheme},
}

// themePromptRoles is every prompt role the definitions may declare, in render
// order. They are Catppuccin's naming, not the terminal's.
var themePromptRoles = []string{
	"text", "red", "green", "yellow", "blue", "mauve", "pink", "teal", "peach",
	"subtext0", "overlay0", "rosewater", "flamingo", "maroon", "lavender",
	"subtext1", "overlay2", "overlay1", "surface2", "surface1", "surface0",
	"base", "mantle", "crust", "sky", "sapphire",
}

// themePromptRequired is the subset Starship's palette table must have. sky and
// sapphire are extra Catppuccin roles the repository has values for but the
// table does not use: they are generated when a definition carries them and are
// not required, so a theme without them is not reported as missing Starship.
var themePromptRequired = []string{
	"text", "red", "green", "yellow", "blue", "mauve", "pink", "teal", "peach",
	"subtext0", "overlay0", "rosewater", "flamingo", "maroon", "lavender",
	"subtext1", "overlay2", "overlay1", "surface2", "surface1", "surface0",
	"base", "mantle", "crust",
}

// themePromptDerivation is the mechanical mapping from the canonical palette to
// the prompt's own role names, written down as code for the same reason the fish
// derivation is: Starship's palette table is written in Catppuccin's naming, and
// only five of the roles it needs are terminal roles (text, red, green, yellow,
// blue), so a theme whose published palette ships no [prompt] table (Kanagawa,
// Everforest, Rosé Pine) must derive the rest to paint the prompt. Deriving each
// role from a palette role the definition already holds means every derived
// prompt colour is a colour of that theme; no value is chosen here.
//
// The three roles the installer's own preview falls back to read the same way
// (theme_preview.go: subtext0 -> bright_black, mauve -> bright_blue, peach ->
// yellow), so the Starship table and the preview cannot disagree about what a
// muted or accent role is. TestThePromptDerivationAgreesWithThePreview pins it.
//
//	prompt role  palette role     why
//	text         text             body text
//	red          red              ANSI 1
//	green        green            ANSI 2
//	yellow       yellow           ANSI 3
//	blue         blue             ANSI 4
//	mauve        bright_blue      the light purple the preview also reads
//	pink         magenta          ANSI 5, the pink of the terminal mapping
//	teal         cyan             ANSI 6, the teal/aqua slot
//	peach        yellow           the warm accent the preview also reads
//	subtext0     bright_black     the muted text slot (the prompt's "muted")
//	subtext1     white            one step above muted
//	overlay0     selection        the surface tone (the prompt's "surface")
//	overlay1     selection        the same one surface
//	overlay2     selection        the same one surface
//	surface0     selection        the same one surface
//	surface1     selection        the same one surface
//	surface2     selection        the same one surface
//	rosewater    cursor           the theme's one non-ANSI accent: its cursor
//	flamingo     bright_red       ANSI 9
//	maroon       red              ANSI 1
//	lavender     bright_magenta   ANSI 13
//	base         base             the background
//	mantle       base             the same one background
//	crust        base             the same one background
var themePromptDerivation = map[string]string{
	"text":      "text",
	"red":       "red",
	"green":     "green",
	"yellow":    "yellow",
	"blue":      "blue",
	"mauve":     "bright_blue",
	"pink":      "magenta",
	"teal":      "cyan",
	"peach":     "yellow",
	"subtext0":  "bright_black",
	"subtext1":  "white",
	"overlay0":  "selection",
	"overlay1":  "selection",
	"overlay2":  "selection",
	"surface0":  "selection",
	"surface1":  "selection",
	"surface2":  "selection",
	"rosewater": "cursor",
	"flamingo":  "bright_red",
	"maroon":    "red",
	"lavender":  "bright_magenta",
	"base":      "base",
	"mantle":    "base",
	"crust":     "base",
}

// themePromptRolesFor returns the prompt roles a definition paints. A role the
// definition declares itself (dotfiles and the two Catppuccin flavours carry the
// table the repository or the published palette holds) is used as it is; a role
// it does not declare is derived from its canonical palette by
// themePromptDerivation. A role that can neither be read nor derived is refused
// rather than emitted as a hole, which is what lets the menu report Starship
// honestly instead of drawing a half palette.
//
// The optional Catppuccin extras (sky, sapphire) are kept when a definition
// carries them and are not required: the generated Starship table emits them when
// present, and a theme without them is not reported as missing Starship.
func themePromptRolesFor(def themeDefinition) (map[string]string, error) {
	roles := make(map[string]string, len(def.Prompt))
	for _, role := range themePromptRequired {
		if value := def.Prompt[role]; value != "" {
			roles[role] = value
			continue
		}
		paletteRole, mapped := themePromptDerivation[role]
		if !mapped {
			return nil, fmt.Errorf("theme %q needs prompt role %q and the derivation names no palette role for it", def.ID, role)
		}
		value := def.Palette[paletteRole]
		if value == "" {
			return nil, fmt.Errorf("theme %q defines neither prompt role %q nor palette role %q, so its Starship palette cannot be generated",
				def.ID, role, paletteRole)
		}
		roles[role] = value
	}
	for role, value := range def.Prompt {
		if _, ok := roles[role]; !ok {
			roles[role] = value
		}
	}
	return roles, nil
}

// renderStarshipPaletteLine renders the palette = "<id>" selection.
func renderStarshipPaletteLine(def themeDefinition) (string, error) {
	return fmt.Sprintf("palette = %q", def.ID), nil
}

// renderStarshipPaletteTable renders the active theme's [palettes.<id>] table.
// The role names are Starship's (Catppuccin's naming): a role the definition
// declares is its own value, a role it does not is derived from its palette by
// themePromptRolesFor, and a role neither can supply is refused rather than
// filled.
func renderStarshipPaletteTable(def themeDefinition) (string, error) {
	roles, err := themePromptRolesFor(def)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[palettes.%s]\n", def.ID)
	for _, role := range themePromptRoles {
		value := roles[role]
		if value == "" {
			continue
		}
		if value == "none" {
			fmt.Fprintf(&b, "%s = \"none\"\n", role)
			continue
		}
		fmt.Fprintf(&b, "%s = %q\n", role, value)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// themeShellRoles maps the canonical palette onto the names the zsh prompt uses.
// surface is the terminal's selection colour and muted is its bright black: the
// prompt has no ANSI roles of its own, it reads these six plus the two extras.
var themeShellRoles = []struct{ Name, Role string }{
	{"BASE", "base"},
	{"SURFACE", "selection"},
	{"TEXT", "text"},
	{"MUTED", "bright_black"},
	{"RED", "red"},
	{"GREEN", "green"},
	{"YELLOW", "yellow"},
	{"BLUE", "blue"},
	{"MAGENTA", "magenta"},
	{"CYAN", "cyan"},
}

// themeSGR turns #rrggbb into the 24-bit foreground SGR sequence zsh and
// LS_COLORS take. The index form the file used before addressed the terminal's
// colour cube, which a custom palette never redefines.
func themeSGR(hex string, background bool) (string, error) {
	if !themeHexRE.MatchString(hex) {
		return "", fmt.Errorf("%q is not a #rrggbb colour", hex)
	}
	r, err := strconv.ParseInt(hex[1:3], 16, 0)
	if err != nil {
		return "", err
	}
	g, err := strconv.ParseInt(hex[3:5], 16, 0)
	if err != nil {
		return "", err
	}
	b, err := strconv.ParseInt(hex[5:7], 16, 0)
	if err != nil {
		return "", err
	}
	kind := 38
	if background {
		kind = 48
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", kind, r, g, b), nil
}

// renderZshTheme renders the whole palette region of dotfiles-zsh/.zshrc: the
// PALETTE_* variables, their *_SGR twins, PALETTE_ESC, and the LS_COLORS and
// EZA_COLORS tables. It is generated whole rather than partly, because a value
// left outside the region would be a seventh hand-written copy.
//
// The LS_COLORS and EZA_COLORS mappings (which extension or key takes which
// role) are fixed here because they are structure, not palette data: every
// colour they name is one of the PALETTE_*_SGR variables defined above them, so
// a theme switch moves them all. No hand-written value outside the palette was
// found, which is why the region is reproducible in full.
func renderZshTheme(def themeDefinition) (string, error) {
	palette, err := themeShellPaletteFor(def)
	if err != nil {
		return "", err
	}
	values, sgrs, surfaceBg := palette.Hex, palette.SGR, palette.SurfaceBgSGR

	var b strings.Builder
	b.WriteString("# ─── Palette ─────────────────────────────────────────────────────────────────\n")
	b.WriteString("# One palette, defined once in themes/" + def.ID + ".toml. The terminal\n")
	b.WriteString("# emulators set the same values, so everything painted inside them resolves to\n")
	b.WriteString("# the same colours instead of each tool falling back to its own defaults.\n#\n")
	purpose := []struct{ Name, Purpose string }{
		{"BASE", "background"},
		{"SURFACE", "selection, de-emphasised punctuation"},
		{"TEXT", "foreground"},
		{"MUTED", "comments, hints, autosuggestions"},
		{"RED", "errors, archives, orphan links"},
		{"GREEN", "success, commands, executables"},
		{"YELLOW", "warnings, strings, documents"},
		{"BLUE", "accent: directories, headers"},
		{"MAGENTA", "constants, images, devices"},
		{"CYAN", "operators, symlinks, media"},
	}
	for _, p := range purpose {
		fmt.Fprintf(&b, "#   %-8s %s   %s\n", strings.ToLower(p.Name), values[p.Name], p.Purpose)
	}
	b.WriteString("#\n")
	b.WriteString("# Every entry is declared twice because the consumers disagree on the format:\n")
	b.WriteString("# the prompt and the line editor take hex, while LS_COLORS and EZA_COLORS take\n")
	b.WriteString("# an SGR sequence. Zsh expands both at file-read time, so the indirection costs\n")
	b.WriteString("# nothing at startup, and having one list is what stops the two forms drifting.\n")
	b.WriteString("#\n")
	b.WriteString("# The 24-bit form is not a style preference. The index form addresses entries\n")
	b.WriteString("# 16-255 of the terminal's colour cube, which a custom theme never redefines, so\n")
	b.WriteString("# those values would render as unrelated hues.\n")
	for _, role := range themeShellRoles {
		first := fmt.Sprintf("typeset -g PALETTE_%s=\"%s\"", role.Name, values[role.Name])
		pad := " "
		if len(first) < 40 {
			pad = strings.Repeat(" ", 40-len(first))
		}
		fmt.Fprintf(&b, "%s%sPALETTE_%s_SGR=\"%s\"\n", first, pad, role.Name, sgrs[role.Name])
	}
	b.WriteString("# The only value that needs the background form rather than the foreground one.\n")
	fmt.Fprintf(&b, "typeset -g PALETTE_SURFACE_BG_SGR=\"%s\"\n", surfaceBg)
	b.WriteString("# A bare escape, so the strings that need a literal sequence can be built from\n")
	b.WriteString("# the palette instead of repeating its digits.\n")
	b.WriteString("typeset -g PALETTE_ESC=$'\\e'\n")
	b.WriteString(zshLsColorsBlock)
	b.WriteString(zshEzaColorsBlock)
	return strings.TrimRight(b.String(), "\n"), nil
}

// zshLsColorsBlock is the LS_COLORS table. Every colour is a PALETTE_*_SGR
// variable, so the mapping is structure and the values are the palette's.
const zshLsColorsBlock = `
# --- File listings: GNU ls reads LS_COLORS, eza reads it as its base layer ----
export LS_COLORS="rs=0:\
di=${PALETTE_BLUE_SGR}:\
ln=${PALETTE_CYAN_SGR}:\
mh=${PALETTE_MUTED_SGR}:\
pi=${PALETTE_YELLOW_SGR}:so=${PALETTE_YELLOW_SGR}:do=${PALETTE_YELLOW_SGR}:\
bd=${PALETTE_MAGENTA_SGR}:cd=${PALETTE_MAGENTA_SGR}:\
or=${PALETTE_RED_SGR};1:mi=${PALETTE_RED_SGR};1:ca=${PALETTE_RED_SGR}:\
su=${PALETTE_RED_SGR};${PALETTE_SURFACE_BG_SGR}:sg=${PALETTE_RED_SGR};${PALETTE_SURFACE_BG_SGR}:\
tw=${PALETTE_GREEN_SGR};${PALETTE_SURFACE_BG_SGR}:ow=${PALETTE_GREEN_SGR};${PALETTE_SURFACE_BG_SGR}:\
st=${PALETTE_BLUE_SGR};${PALETTE_SURFACE_BG_SGR}:\
ex=${PALETTE_GREEN_SGR}:\
*.tar=${PALETTE_RED_SGR}:*.tgz=${PALETTE_RED_SGR}:*.tbz2=${PALETTE_RED_SGR}:*.txz=${PALETTE_RED_SGR}:*.zst=${PALETTE_RED_SGR}:\
*.zip=${PALETTE_RED_SGR}:*.7z=${PALETTE_RED_SGR}:*.rar=${PALETTE_RED_SGR}:\
*.gz=${PALETTE_RED_SGR}:*.bz2=${PALETTE_RED_SGR}:*.xz=${PALETTE_RED_SGR}:\
*.png=${PALETTE_MAGENTA_SGR}:*.jpg=${PALETTE_MAGENTA_SGR}:*.jpeg=${PALETTE_MAGENTA_SGR}:*.gif=${PALETTE_MAGENTA_SGR}:*.webp=${PALETTE_MAGENTA_SGR}:*.svg=${PALETTE_MAGENTA_SGR}:*.ico=${PALETTE_MAGENTA_SGR}:\
*.mp4=${PALETTE_MAGENTA_SGR}:*.mkv=${PALETTE_MAGENTA_SGR}:*.mov=${PALETTE_MAGENTA_SGR}:*.webm=${PALETTE_MAGENTA_SGR}:\
*.mp3=${PALETTE_CYAN_SGR}:*.flac=${PALETTE_CYAN_SGR}:*.wav=${PALETTE_CYAN_SGR}:*.ogg=${PALETTE_CYAN_SGR}:*.m4a=${PALETTE_CYAN_SGR}:\
*.pdf=${PALETTE_YELLOW_SGR}:*.md=${PALETTE_YELLOW_SGR}:*.txt=${PALETTE_YELLOW_SGR}:*.rst=${PALETTE_YELLOW_SGR}:\
*.sh=${PALETTE_GREEN_SGR}:*.bash=${PALETTE_GREEN_SGR}:*.zsh=${PALETTE_GREEN_SGR}:*.fish=${PALETTE_GREEN_SGR}:\
*.py=${PALETTE_GREEN_SGR}:*.go=${PALETTE_GREEN_SGR}:*.rs=${PALETTE_GREEN_SGR}:*.js=${PALETTE_GREEN_SGR}:*.ts=${PALETTE_GREEN_SGR}:\
*.json=${PALETTE_GREEN_SGR}:*.yaml=${PALETTE_GREEN_SGR}:*.yml=${PALETTE_GREEN_SGR}:*.toml=${PALETTE_GREEN_SGR}:\
*.db=${PALETTE_BLUE_SGR}:*.sqlite=${PALETTE_BLUE_SGR}:*.sql=${PALETTE_BLUE_SGR}:\
*.log=${PALETTE_MUTED_SGR}:*.lock=${PALETTE_MUTED_SGR}"`

// zshEzaColorsBlock is the EZA_COLORS table, the same shape as LS_COLORS.
const zshEzaColorsBlock = `

# --- eza metadata ------------------------------------------------------------
# eza paints permissions, owner, size and date from its own defaults (bold
# yellow, red, green, blue) which fight with the file names and with every other
# tool in the terminal. Metadata is de-emphasised here and colour is left to
# carry meaning: the names, the git state, and the security bits in the
# permission column.
export EZA_COLORS="\
oc=${PALETTE_MUTED_SGR}:\
ur=${PALETTE_MUTED_SGR}:uw=${PALETTE_MUTED_SGR}:ux=${PALETTE_MUTED_SGR}:ue=${PALETTE_MUTED_SGR}:\
gr=${PALETTE_MUTED_SGR}:gw=${PALETTE_MUTED_SGR}:gx=${PALETTE_MUTED_SGR}:\
tr=${PALETTE_MUTED_SGR}:tw=${PALETTE_MUTED_SGR}:tx=${PALETTE_MUTED_SGR}:\
su=${PALETTE_YELLOW_SGR};1:sf=${PALETTE_YELLOW_SGR};1:xa=${PALETTE_MAGENTA_SGR}:\
sn=${PALETTE_CYAN_SGR}:nb=${PALETTE_CYAN_SGR}:nk=${PALETTE_CYAN_SGR}:nm=${PALETTE_CYAN_SGR}:ng=${PALETTE_CYAN_SGR}:nt=${PALETTE_CYAN_SGR}:\
sb=${PALETTE_MUTED_SGR}:ub=${PALETTE_MUTED_SGR}:uk=${PALETTE_MUTED_SGR}:um=${PALETTE_MUTED_SGR}:ug=${PALETTE_MUTED_SGR}:ut=${PALETTE_MUTED_SGR}:\
df=${PALETTE_MUTED_SGR}:ds=${PALETTE_MUTED_SGR}:lc=${PALETTE_MUTED_SGR}:lm=${PALETTE_MUTED_SGR}:\
uu=${PALETTE_BLUE_SGR}:un=${PALETTE_MUTED_SGR}:uR=${PALETTE_RED_SGR}:\
gu=${PALETTE_BLUE_SGR}:gn=${PALETTE_MUTED_SGR}:gR=${PALETTE_RED_SGR}:\
xx=${PALETTE_SURFACE_SGR}:\
da=${PALETTE_MUTED_SGR}:in=${PALETTE_MUTED_SGR}:bl=${PALETTE_MUTED_SGR}:\
hd=${PALETTE_BLUE_SGR};1:lp=${PALETTE_CYAN_SGR}:cc=${PALETTE_RED_SGR}:bO=${PALETTE_RED_SGR};4:\
sp=${PALETTE_MAGENTA_SGR}:mp=${PALETTE_MAGENTA_SGR}:\
im=${PALETTE_MAGENTA_SGR}:vi=${PALETTE_MAGENTA_SGR}:mu=${PALETTE_CYAN_SGR}:lo=${PALETTE_CYAN_SGR}:\
cr=${PALETTE_YELLOW_SGR}:do=${PALETTE_YELLOW_SGR}:co=${PALETTE_RED_SGR}:tm=${PALETTE_MUTED_SGR}:cm=${PALETTE_MUTED_SGR}:\
ga=${PALETTE_GREEN_SGR}:gm=${PALETTE_YELLOW_SGR}:gd=${PALETTE_RED_SGR}:gv=${PALETTE_MAGENTA_SGR}:\
gt=${PALETTE_YELLOW_SGR}:gi=${PALETTE_MUTED_SGR}:gc=${PALETTE_RED_SGR};1:\
Gm=${PALETTE_BLUE_SGR};1:Go=${PALETTE_CYAN_SGR}:Gc=${PALETTE_GREEN_SGR}:Gd=${PALETTE_YELLOW_SGR}"`

// themeListingTableForShell renders one of the two listing tables above for a shell.
// The tables are structure: which extension or key takes which role is settled once,
// and every value in them is a PALETTE_*_SGR variable. What the shells disagree about
// is only how a variable is spoken -- zsh writes a braced reference and exports with
// `export`, fish writes a bare one and exports with `set -gx` -- so the mapping is
// translated rather than restated, and a key added to either table reaches both
// shells.
//
// Measured with fish on this machine: a `$VAR` reference expands inside a
// double-quoted value, a backslash at the end of a line continues that value without
// keeping the newline, and `set -gx` exports the result, so eza reads LS_COLORS out of
// the environment exactly as it does under zsh. The space before the quote is not
// cosmetic: fish's `set` refuses a joined NAME=value with "invalid variable name", so
// the value has to be an argument of its own.
var (
	themeZshExportLine     = regexp.MustCompile(`(?m)^export ([A-Z_]+)="`)
	themeZshVariableInText = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)
)

func themeListingTableForShell(table, shell string) string {
	if shell != "fish" {
		return table
	}
	exported := themeZshExportLine.ReplaceAllString(table, `set -gx $1 "`)
	return themeZshVariableInText.ReplaceAllString(exported, `$$$1`)
}

// themeShellPalette is the palette every renderer that spills colour into a shell
// configuration reads: the ten roles the prompts, the listings and the line editors
// share, in the two forms those consumers disagree about (hex for a prompt, an SGR
// sequence for LS_COLORS and EZA_COLORS), plus the one background sequence LS_COLORS
// needs to mark a mode bit without hiding the name under it.
type themeShellPalette struct {
	Hex          map[string]string
	SGR          map[string]string
	SurfaceBgSGR string
}

// themeShellPaletteFor computes that palette once, from the theme's own definition.
// Every shell renderer reads this, which is what stops the two forms -- and the two
// shells -- from drifting apart: one list of roles, one value each.
func themeShellPaletteFor(def themeDefinition) (themeShellPalette, error) {
	palette := themeShellPalette{Hex: map[string]string{}, SGR: map[string]string{}}
	for _, role := range themeShellRoles {
		hex := def.Palette[role.Role]
		if hex == "" {
			return themeShellPalette{}, fmt.Errorf("theme %q defines no %q, so its palette cannot be rendered for the shells that read it", def.ID, role.Role)
		}
		sgr, err := themeSGR(hex, false)
		if err != nil {
			return themeShellPalette{}, err
		}
		palette.Hex[role.Name] = hex
		palette.SGR[role.Name] = sgr
	}
	background, err := themeSGR(palette.Hex["SURFACE"], true)
	if err != nil {
		return themeShellPalette{}, err
	}
	palette.SurfaceBgSGR = background
	return palette, nil
}

// renderP10kTheme renders the p10k prompt's palette fallbacks. They are the same
// ten roles the zsh line editor reads, written as ${VAR:-default} so the prompt
// still works when .p10k.zsh is sourced on its own.
func renderP10kTheme(def themeDefinition) (string, error) {
	palette, err := themeShellPaletteFor(def)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("  # ── Palette ────────────────────────────────────────────────────────────────\n")
	b.WriteString("  # Every colour below comes from the palette declared once in themes/" + def.ID + ".toml,\n")
	b.WriteString("  # which is what keeps the prompt from drifting away from the listings and the\n")
	b.WriteString("  # line editor, which read the same values. They are assignments rather than\n")
	b.WriteString("  # fallbacks, so re-sourcing this file is what applies a theme change: p10k\n")
	b.WriteString("  # reload re-sources it, and a fallback would keep the value the shell already\n")
	b.WriteString("  # held from the theme before it. The file still stands on its own without\n")
	b.WriteString("  # .zshrc, because an assignment sets the value whether or not it is defined.\n")
	b.WriteString("  #\n")
	b.WriteString("  # The block this used to carry held a Kanagawa palette (background #1f1f28,\n")
	b.WriteString("  # red #c34043, green #76946a, blue #7e9cd8) while the terminal emulators\n")
	b.WriteString("  # defined different values, so the prompt and the terminal disagreed.\n")
	for _, role := range themeShellRoles {
		fmt.Fprintf(&b, "  typeset -g PALETTE_%s=\"%s\"\n", role.Name, palette.Hex[role.Name])
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// batSelectionName is the value BAT_THEME has to hold for a definition: the stem
// of its [bat] file, because that is the name bat registers a custom .tmTheme
// under. The <key>name</key> inside the file is **not** the selection key -
// measured with bat 0.26.1, whose cache lists a theme directory's
// catppuccin-mocha.tmTheme as "catppuccin-mocha" beside its own bundled
// "Catppuccin Mocha", paints the repository's blue keyword for
// BAT_THEME=catppuccin-mocha, and paints the bundled theme's mauve for
// BAT_THEME="Catppuccin Mocha". Exporting [bat] name therefore reached bat's own
// bundled Catppuccin Mocha rather than the file this repository ships;
// TestTheBatSelectionNamesTheFileBatRegisters pins the file's name instead.
func batSelectionName(def themeDefinition) (string, error) {
	if def.BatFile == "" {
		return "", fmt.Errorf("theme %q names no bat theme file, so its bat selection cannot be generated", def.ID)
	}
	return strings.TrimSuffix(filepath.Base(def.BatFile), ".tmTheme"), nil
}

// renderBatSelection renders the BAT_THEME selection in dotfiles-zsh/.zshrc. It
// is the switch's half of bat: the theme files are generated too, and the
// installer copies every shipped .tmTheme into bat's directory and rebuilds its
// cache, so the name below resolves. The file check keeps a machine where that
// has not happened yet from turning every bat call into "Unknown theme".
func renderBatSelection(def themeDefinition) (string, error) {
	if def.Bat == "" {
		return "", fmt.Errorf("theme %q names no bat theme, so its selection cannot be generated", def.ID)
	}
	selected, err := batSelectionName(def)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`# --- bat --------------------------------------------------------------------
# The theme bat uses. Generated from themes/%s.toml; the .tmTheme files ship with
# these dotfiles and the installer builds each into bat's cache, so the name below
# resolves. The file check keeps a machine where that has not happened yet from
# turning every bat call into "Unknown theme".
if [[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/bat/themes/%s" ]]; then
    export BAT_THEME="%s"
else
    export BAT_THEME="Catppuccin Mocha"
fi`, def.ID, def.BatFile, selected), nil
}

// renderNvimColorscheme renders the LazyVim colorscheme selection. Neovim's
// theme comes from a plugin, so the definition names the colorscheme rather than
// a palette; a theme whose plugin ships none is refused.
func renderNvimColorscheme(def themeDefinition) (string, error) {
	if def.Nvim == "" {
		return "", fmt.Errorf("theme %q names no Neovim colorscheme, so Neovim cannot be themed with it", def.ID)
	}
	return fmt.Sprintf("        colorscheme = %q,", def.Nvim), nil
}

// themeNvimPluginColorschemes are the Neovim colorschemes the repository's own
// plugin install provides, so a definition that names one of them is painted by
// the plugin and needs no generated file. The repository's nvim configuration
// installs exactly these two (dotfiles-nvim/nvim/lua/plugins/colorscheme.lua).
// Its Catppuccin spec pins flavour = "mocha", which is why Catppuccin Latte
// names a colorscheme this repository generates instead of pointing at the
// plugin: the plugin's own opts would paint Mocha's flavour under a Latte name.
var themeNvimPluginColorschemes = []string{"catppuccin", "kanagawa"}

// themeNvimGeneratedFile returns the Neovim colorscheme file this repository
// ships for a theme, or "" when it ships none. It is read from themeThemeFiles
// rather than from disk on purpose: coverage is a property of the definitions
// and what this change generates, so the menu and the guard answer the same
// question without either of them opening a file.
func themeNvimGeneratedFile(theme string) string {
	for _, file := range themeThemeFiles {
		if file.Tool == "nvim" && file.Theme == theme {
			return file.Path
		}
	}
	return ""
}

// themeNvimAvailable reports whether a definition can paint Neovim: it has to
// name a colorscheme, and that name has to resolve - either to a colorscheme the
// repository's plugin install provides, or to a file this repository generates
// and ships under dotfiles-nvim/nvim/colors/.
func themeNvimAvailable(def themeDefinition) bool {
	if def.Nvim == "" {
		return false
	}
	if slices.Contains(themeNvimPluginColorschemes, def.Nvim) {
		return true
	}
	return themeNvimGeneratedFile(def.ID) != ""
}

// nvimColorsDir is the directory Neovim reads a colorscheme this repository
// generates from: ~/.config/nvim/colors, on the user's own config, which is on
// the runtimepath ahead of any plugin. Neovim resolves :colorscheme <name> by
// finding colors/<name>.lua or colors/<name>.vim there, so the file is the
// machine-side proof the line loads.
func nvimColorsDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "nvim", "colors")
}

// nvimColorschemeInstalledPath is the file Neovim has to hold for a definition's
// generated colorscheme to resolve.
func nvimColorschemeInstalledPath(homeDir string, def themeDefinition) string {
	if def.Nvim == "" {
		return ""
	}
	return filepath.Join(nvimColorsDir(homeDir), def.Nvim+".lua")
}

// nvimPluginColorsDirs lists the colors/ directories the plugins installed on
// this machine put a colorscheme in, under Neovim's data root: lazy.nvim's
// plugin tree and Vim's native packages. The file name is the colorscheme name
// :colorscheme resolves, so a file here is the proof a plugin-named colorscheme
// will load.
func nvimPluginColorsDirs(homeDir string) []string {
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		dataHome = filepath.Join(homeDir, ".local", "share")
	}
	nvimData := filepath.Join(dataHome, "nvim")
	var dirs []string
	if entries, err := os.ReadDir(filepath.Join(nvimData, "lazy")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, filepath.Join(nvimData, "lazy", entry.Name(), "colors"))
			}
		}
	}
	for _, kind := range []string{"start", "opt"} {
		packed, _ := filepath.Glob(filepath.Join(nvimData, "site", "pack", "*", kind, "*", "colors"))
		dirs = append(dirs, packed...)
	}
	return dirs
}

// nvimColorschemeFileExists reports whether a colorscheme name has a colors file
// in any directory Neovim reads on this machine. It is the check the switch runs
// before it writes a line naming a colorscheme it does not generate itself.
func nvimColorschemeFileExists(homeDir, name string) bool {
	if name == "" {
		return false
	}
	dirs := append([]string{nvimColorsDir(homeDir)}, nvimPluginColorsDirs(homeDir)...)
	for _, dir := range dirs {
		for _, ext := range []string{".lua", ".vim"} {
			if _, err := os.Stat(filepath.Join(dir, name+ext)); err == nil {
				return true
			}
		}
	}
	return false
}

// installGeneratedNvimColorscheme writes a definition's generated colorscheme
// into ~/.config/nvim/colors, so the line the switch writes names a colorscheme
// Neovim can load. It never clobbers a file this repository did not write: a
// colors file with no ownership marker belongs to the user, and it still
// resolves the name, so it is left exactly as it is. The generated files are
// install assets like bat's .tmTheme files rather than part of the undo record:
// an undo restores the colorscheme line and leaves the file available, which
// changes nothing Neovim shows.
func installGeneratedNvimColorscheme(homeDir string, def themeDefinition) (string, error) {
	target := nvimColorschemeInstalledPath(homeDir, def)
	if target == "" {
		return "", fmt.Errorf("theme %q names no Neovim colorscheme", def.ID)
	}
	if existing, err := os.ReadFile(target); err == nil && !strings.Contains(string(existing), themeOwnershipMarker) {
		return target, nil
	}
	content, err := renderNvimTheme(def)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return "", err
	}
	return target, nil
}

// reachableNvimColorscheme makes a definition's Neovim colorscheme reachable on
// this machine and returns the reason when it cannot be. A generated colorscheme
// is installed under ~/.config/nvim/colors; a plugin's colorscheme is only named
// when a colors file this machine's Neovim reads proves the plugin is installed.
// An empty string means :colorscheme on the name the switch is about to write
// will load rather than raise E185.
func reachableNvimColorscheme(homeDir string, def themeDefinition) string {
	if def.Nvim == "" {
		return fmt.Sprintf("the %s theme names no Neovim colorscheme", def.Name)
	}
	if themeNvimGeneratedFile(def.ID) != "" {
		if _, err := installGeneratedNvimColorscheme(homeDir, def); err != nil {
			return fmt.Sprintf("its generated colorscheme could not be installed: %v", err)
		}
		return ""
	}
	if slices.Contains(themeNvimPluginColorschemes, def.Nvim) {
		if nvimColorschemeFileExists(homeDir, def.Nvim) {
			return ""
		}
		return fmt.Sprintf("the plugin that registers the %s colorscheme is not installed", def.Nvim)
	}
	return fmt.Sprintf("no plugin or generated file provides the %s colorscheme", def.Nvim)
}

// themeBaseIsLight reports whether a theme's background is a light colour, so
// the generated Neovim colorscheme declares the background Neovim's own default
// groups expect. It reads the theme's own base role and compares its relative
// luminance with mid grey: that is a rule about a value the definition already
// holds, not a second colour, and it is what makes Catppuccin Latte a light
// colorscheme and the other five dark ones.
func themeBaseIsLight(def themeDefinition) bool {
	base := strings.TrimPrefix(def.Palette["base"], "#")
	if len(base) != 6 {
		return false
	}
	value, err := strconv.ParseUint(base, 16, 32)
	if err != nil {
		return false
	}
	r := float64((value>>16)&0xff) / 255
	g := float64((value>>8)&0xff) / 255
	b := float64(value&0xff) / 255
	return 0.2126*r+0.7152*g+0.0722*b > 0.5
}

// themeNvimGroups is every highlight group the generated Neovim colorscheme
// paints, and which palette role each of its slots takes. It is data for the
// same reason themeBatRoles is: a group name does not say which role it takes,
// and writing the mapping down once is what keeps Neovim and bat agreeing about
// what a comment, a string or a keyword looks like. Fg and Bg name palette
// roles; an empty one is a slot the group does not paint, and Attr is the style
// Neovim adds on top.
var themeNvimGroups = []struct{ Group, Fg, Bg, Attr string }{
	{"Normal", "text", "base", ""},
	{"NormalNC", "text", "base", ""},
	{"NormalFloat", "text", "base", ""},
	{"FloatBorder", "bright_black", "base", ""},
	{"FloatTitle", "blue", "base", "bold"},
	{"MsgArea", "text", "base", ""},
	{"Cursor", "cursor_text", "cursor", ""},
	{"lCursor", "cursor_text", "cursor", ""},
	{"TermCursor", "cursor_text", "cursor", ""},
	{"CursorLine", "", "selection", ""},
	{"CursorColumn", "", "selection", ""},
	{"ColorColumn", "", "selection", ""},
	{"CursorLineNr", "yellow", "", "bold"},
	{"LineNr", "bright_black", "", ""},
	{"SignColumn", "bright_black", "", ""},
	{"FoldColumn", "bright_black", "", ""},
	{"Folded", "bright_black", "selection", ""},
	{"NonText", "bright_black", "", ""},
	{"SpecialKey", "bright_black", "", ""},
	{"Whitespace", "bright_black", "", ""},
	{"EndOfBuffer", "base", "", ""},
	{"WinSeparator", "bright_black", "base", ""},
	{"Visual", "", "selection", ""},
	{"VisualNOS", "", "selection", ""},
	{"Search", "base", "yellow", ""},
	{"IncSearch", "base", "green", ""},
	{"CurSearch", "base", "green", ""},
	{"MatchParen", "cyan", "selection", "bold"},
	{"Pmenu", "text", "selection", ""},
	{"PmenuSel", "base", "blue", "bold"},
	{"PmenuSbar", "", "selection", ""},
	{"PmenuThumb", "", "bright_black", ""},
	{"StatusLine", "text", "selection", ""},
	{"StatusLineNC", "bright_black", "base", ""},
	{"TabLine", "bright_black", "base", ""},
	{"TabLineSel", "text", "selection", "bold"},
	{"TabLineFill", "", "base", ""},
	{"Title", "blue", "", "bold"},
	{"Directory", "blue", "", ""},
	{"ErrorMsg", "red", "base", ""},
	{"WarningMsg", "yellow", "", ""},
	{"MoreMsg", "green", "", ""},
	{"ModeMsg", "text", "", "bold"},
	{"Question", "green", "", ""},
	{"WildMenu", "base", "blue", "bold"},
	{"QuickFixLine", "", "selection", ""},
	{"Comment", "bright_black", "", "italic"},
	{"SpecialComment", "bright_black", "", "italic"},
	{"Constant", "magenta", "", ""},
	{"String", "yellow", "", ""},
	{"Character", "yellow", "", ""},
	{"Number", "magenta", "", ""},
	{"Boolean", "magenta", "", ""},
	{"Float", "magenta", "", ""},
	{"Identifier", "text", "", ""},
	{"Function", "green", "", ""},
	{"Statement", "blue", "", ""},
	{"Conditional", "blue", "", ""},
	{"Repeat", "blue", "", ""},
	{"Label", "blue", "", ""},
	{"Operator", "cyan", "", ""},
	{"Keyword", "blue", "", ""},
	{"Exception", "red", "", ""},
	{"PreProc", "magenta", "", ""},
	{"Include", "magenta", "", ""},
	{"Define", "magenta", "", ""},
	{"Macro", "magenta", "", ""},
	{"PreCondit", "magenta", "", ""},
	{"Type", "blue", "", ""},
	{"StorageClass", "blue", "", ""},
	{"Structure", "blue", "", ""},
	{"Typedef", "blue", "", ""},
	{"Special", "cyan", "", ""},
	{"SpecialChar", "cyan", "", ""},
	{"Tag", "green", "", ""},
	{"Delimiter", "cyan", "", ""},
	{"Debug", "red", "", ""},
	{"Underlined", "blue", "", "underline"},
	{"Ignore", "bright_black", "", ""},
	{"Error", "red", "base", ""},
	{"Todo", "base", "yellow", "bold"},
	{"DiffAdd", "green", "base", ""},
	{"DiffChange", "yellow", "base", ""},
	{"DiffDelete", "red", "base", ""},
	{"DiffText", "blue", "base", "bold"},
	{"Added", "green", "", ""},
	{"Changed", "yellow", "", ""},
	{"Removed", "red", "", ""},
	{"DiagnosticError", "red", "", ""},
	{"DiagnosticWarn", "yellow", "", ""},
	{"DiagnosticInfo", "cyan", "", ""},
	{"DiagnosticHint", "bright_black", "", ""},
	{"DiagnosticOk", "green", "", ""},
	{"LspReferenceText", "", "selection", ""},
	{"LspReferenceRead", "", "selection", ""},
	{"LspReferenceWrite", "", "selection", ""},
}

// themeNvimTerminalRoles are the palette roles Neovim's own :terminal reads, in
// the order the terminal's colour numbers run: g:terminal_color_0 is the
// canonical black and g:terminal_color_15 the canonical bright white.
var themeNvimTerminalRoles = []string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"bright_black", "bright_red", "bright_green", "bright_yellow",
	"bright_blue", "bright_magenta", "bright_cyan", "bright_white",
}

// themeNvimTemplate is the colorscheme file's fixed frame: what it is, where it
// is selected, and the two loops that fill it. The braces are placeholders the
// renderer replaces, so the template holds no colour of its own.
const themeNvimTemplate = `-- dotfiles-managed-config: nvim
-- name: {{colorscheme}}
-- generated from themes/{{id}}.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the {{display}} palette. Every colour below is a
-- role themes/{{id}}.toml holds: a highlight group's slot is mapped to a palette
-- role by a table written down once, in installer/internal/tui/installer.go
-- (themeNvimGroups) and in themes/README.md, so no value here was chosen by eye.
-- The switch selects this file through the colorscheme line it generates in
-- lua/plugins/colorscheme.lua, and the file is found because ~/.config/nvim is on
-- Neovim's runtimepath ahead of any plugin.
--
-- The background is the theme's own: its base role, the colour the terminals in
-- this repository paint behind everything, and "{{background}}" because that base
-- is {{backgroundness}}.

vim.cmd("highlight clear")
if vim.fn.exists("syntax_on") == 1 then
  vim.cmd("syntax reset")
end

vim.o.termguicolors = true
vim.o.background = "{{background}}"
vim.g.colors_name = "{{colorscheme}}"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

{{groups}}

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
{{terminal}}
`

// renderNvimTheme renders a whole Neovim colorscheme from the definition. Neovim
// has no plugin for every theme in the library, so the colorscheme is generated
// rather than named: each highlight group takes a palette role through
// themeNvimGroups, and a definition that misses one is refused instead of emitted
// with a hole.
func renderNvimTheme(def themeDefinition) (string, error) {
	if def.Nvim == "" {
		return "", fmt.Errorf("theme %q names no Neovim colorscheme, so its colorscheme cannot be generated", def.ID)
	}

	var groups strings.Builder
	for _, group := range themeNvimGroups {
		var slots []string
		for _, slot := range []struct{ Key, Role string }{{"fg", group.Fg}, {"bg", group.Bg}} {
			if slot.Role == "" {
				continue
			}
			value, err := themeHex(def, slot.Role)
			if err != nil {
				return "", err
			}
			slots = append(slots, fmt.Sprintf("%s = %q", slot.Key, value))
		}
		if group.Attr != "" {
			slots = append(slots, group.Attr+" = true")
		}
		if len(slots) == 0 {
			return "", fmt.Errorf("theme %q: the Neovim group %q maps to no palette role", def.ID, group.Group)
		}
		fmt.Fprintf(&groups, "set(%q, { %s })\n", group.Group, strings.Join(slots, ", "))
	}

	var terminal strings.Builder
	for i, role := range themeNvimTerminalRoles {
		value, err := themeHex(def, role)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&terminal, "vim.g.terminal_color_%d = %q\n", i, value)
	}

	background, backgroundness := "dark", "darker than mid grey"
	if themeBaseIsLight(def) {
		background, backgroundness = "light", "lighter than mid grey"
	}

	return strings.NewReplacer(
		"{{id}}", def.ID,
		"{{display}}", def.Name,
		"{{colorscheme}}", def.Nvim,
		"{{background}}", background,
		"{{backgroundness}}", backgroundness,
		"{{groups}}", strings.TrimRight(groups.String(), "\n"),
		"{{terminal}}", strings.TrimRight(terminal.String(), "\n"),
	).Replace(themeNvimTemplate), nil
}

// themeBatTemplate is the bat .tmTheme, keyed by palette role. The syntax
// mapping (which scope takes which role) is fixed here because it is structure,
// not palette data; every colour it emits is a role the definition holds, so
// there is no hand-written colour left in the generated file. The one backtick
// in the file is parked behind [[BT]] so the template can live in a Go raw
// string; the renderer puts it back.
const themeBatTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<!--
  The {{id}} theme for bat.

  Every colour is one of the ten the terminal emulators in this repository
  define, so syntax highlighting inside the terminal cannot disagree with the
  prompt, the file listings or Herdr. The mapping is by role, not by taste:

    muted   {{muted}}   comments, punctuation, invisible characters
    yellow  {{yellow}}   strings
    magenta {{magenta}}   numbers and language constants, preprocessor directives
    blue    {{blue}}   keywords, storage, types, tags, headings
    cyan    {{cyan}}   operators, escapes, attribute names, class names
    green   {{green}}   function names, markdown code spans
    red     {{red}}   variables, invalid syntax, deletions
    text    {{text}}   everything else

  No background is set on purpose. The terminal paints one and it is partly
  transparent, so a themed background would sit on top of it as a panel with a
  second black.

  Regenerate-to-install: rebuild bat's cache (run [[BT]]bat cache[[BT]] with
  its build flag) after any change here.
-->
<plist version="1.0">
<dict>
	<key>name</key>
	<string>{{batname}}</string>
	<key>settings</key>
	<array>
		<dict>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{text}}</string>
				<key>caret</key>
				<string>{{blue}}</string>
				<key>selection</key>
				<string>{{selection}}</string>
				<key>selectionForeground</key>
				<string>{{text}}</string>
				<key>lineHighlight</key>
				<string>{{selection}}</string>
				<key>invisibles</key>
				<string>{{muted}}</string>
			</dict>
		</dict>

		<!-- Comments and the punctuation that should recede with them. -->
		<dict>
			<key>name</key>
			<string>Comment</string>
			<key>scope</key>
			<string>comment, punctuation.definition.comment</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{muted}}</string>
				<key>fontStyle</key>
				<string>italic</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Punctuation</string>
			<key>scope</key>
			<string>punctuation, punctuation.separator, punctuation.terminator, meta.brace</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{muted}}</string>
			</dict>
		</dict>

		<!-- Strings: one colour for the whole family. -->
		<dict>
			<key>name</key>
			<string>String</string>
			<key>scope</key>
			<string>string, punctuation.definition.string</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{yellow}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>String escape</string>
			<key>scope</key>
			<string>constant.character.escape, constant.other.placeholder</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{cyan}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>String interpolation</string>
			<key>scope</key>
			<string>punctuation.section.embedded, punctuation.definition.template-expression</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{red}}</string>
			</dict>
		</dict>

		<!-- Numbers and the language constants sit together. -->
		<dict>
			<key>name</key>
			<string>Constant</string>
			<key>scope</key>
			<string>constant.numeric, constant.language, constant.other, support.constant, variable.language</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{magenta}}</string>
			</dict>
		</dict>

		<!-- Keywords, storage and types carry the accent. -->
		<dict>
			<key>name</key>
			<string>Keyword</string>
			<key>scope</key>
			<string>keyword, keyword.control, keyword.other, storage, storage.type, storage.modifier</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{blue}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Operator</string>
			<key>scope</key>
			<string>keyword.operator, keyword.operator.assignment, keyword.operator.arithmetic</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{cyan}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Preprocessor</string>
			<key>scope</key>
			<string>meta.preprocessor, keyword.control.import, keyword.control.directive</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{magenta}}</string>
			</dict>
		</dict>

		<!-- Names: functions green, types cyan, tags accent. -->
		<dict>
			<key>name</key>
			<string>Function</string>
			<key>scope</key>
			<string>entity.name.function, support.function, meta.function-call, entity.name.function.macro</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{green}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Type</string>
			<key>scope</key>
			<string>entity.name.type, entity.name.class, entity.name.struct, support.class, support.type, entity.other.inherited-class</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{cyan}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Tag</string>
			<key>scope</key>
			<string>entity.name.tag, punctuation.definition.tag</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{blue}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Attribute</string>
			<key>scope</key>
			<string>entity.other.attribute-name, meta.object-literal.key, support.type.property-name</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{green}}</string>
			</dict>
		</dict>

		<!-- Variables, the one place a fourth tone is needed. -->
		<dict>
			<key>name</key>
			<string>Variable</string>
			<key>scope</key>
			<string>variable, variable.other, variable.parameter, variable.function, entity.name.variable</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{red}}</string>
			</dict>
		</dict>

		<!-- Markdown and documentation. -->
		<dict>
			<key>name</key>
			<string>Heading</string>
			<key>scope</key>
			<string>markup.heading, markup.heading punctuation.definition.heading</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{blue}}</string>
				<key>fontStyle</key>
				<string>bold</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Bold</string>
			<key>scope</key>
			<string>markup.bold</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{text}}</string>
				<key>fontStyle</key>
				<string>bold</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Italic</string>
			<key>scope</key>
			<string>markup.italic</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{text}}</string>
				<key>fontStyle</key>
				<string>italic</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Raw block</string>
			<key>scope</key>
			<string>markup.raw, markup.raw.block, markup.inline.raw</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{green}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Link</string>
			<key>scope</key>
			<string>markup.underline.link, string.other.link</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{cyan}}</string>
				<key>fontStyle</key>
				<string>underline</string>
			</dict>
		</dict>

		<!-- Diff. -->
		<dict>
			<key>name</key>
			<string>Diff insert</string>
			<key>scope</key>
			<string>markup.inserted, markup.inserted.diff</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{green}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Diff delete</string>
			<key>scope</key>
			<string>markup.deleted, markup.deleted.diff</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{red}}</string>
			</dict>
		</dict>
		<dict>
			<key>name</key>
			<string>Diff change</string>
			<key>scope</key>
			<string>markup.changed, markup.changed.diff</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{yellow}}</string>
			</dict>
		</dict>

		<!-- Errors. -->
		<dict>
			<key>name</key>
			<string>Invalid</string>
			<key>scope</key>
			<string>invalid, invalid.illegal, invalid.broken</string>
			<key>settings</key>
			<dict>
				<key>foreground</key>
				<string>{{red}}</string>
				<key>fontStyle</key>
				<string>bold</string>
			</dict>
		</dict>
	</array>
</dict>
</plist>`

// themeBatRoles maps the bat template's tokens onto palette roles. The mapping is
// the file's own documented one:
//
//	muted   #8a8fa3   comments, punctuation, invisible characters
//	yellow  #ffe066   strings
//	magenta #ff8dd7   numbers and language constants, preprocessor directives
//	blue    #7fb4ca   keywords, storage, types, tags, headings
//	cyan    #7aa89f   operators, escapes, attribute names, class names
//	green   #b7cc85   function names, markdown code spans
//	red     #cb7c94   variables, invalid syntax, deletions
//	text    #f3f6f9   everything else
//	selection #263356 the selection and line-highlight backgrounds
//
// It is data because it is not obvious: a reader cannot tell from a scope name
// which role it takes, so the mapping is written down once here.
var themeBatRoles = []struct{ Token, Role string }{
	{"muted", "bright_black"},
	{"yellow", "yellow"},
	{"magenta", "magenta"},
	{"blue", "blue"},
	{"cyan", "cyan"},
	{"green", "green"},
	{"red", "red"},
	{"text", "text"},
	{"selection", "selection"},
}

// renderBatTheme renders a bat .tmTheme from the definition. Every colour it
// emits is a role the definition holds; a definition missing one is refused
// rather than emitted with a hole, and the syntax mapping is themeBatRoles above.
func renderBatTheme(def themeDefinition) (string, error) {
	pairs := []string{"{{id}}", def.ID}
	if def.Bat == "" {
		return "", fmt.Errorf("theme %q names no bat theme, so its bat theme cannot be generated", def.ID)
	}
	pairs = append(pairs, "{{batname}}", def.Bat)
	for _, role := range themeBatRoles {
		value := def.Palette[role.Role]
		if value == "" {
			return "", fmt.Errorf("theme %q defines no %q, so its bat theme cannot be generated", def.ID, role.Role)
		}
		pairs = append(pairs, "{{"+role.Token+"}}", value)
	}
	// The one backtick the template parks behind [[BT]].
	pairs = append(pairs, "[[BT]]", "`")
	return strings.NewReplacer(pairs...).Replace(themeBatTemplate), nil
}

// stepExecutors is the dispatch table for the non-interactive executor. Each
// entry runs one step against the model the run owns.
//
// It is a map rather than a switch so the scheduled-steps-versus-executor
// invariant test can enumerate the cases instead of restating them: a step that
// buildStepsForChoices or SetupInstallSteps schedules and this table does not
// know is a step that reports itself done having done nothing, the defect class
// behind the last two fixes on this path. The switch this replaces could not be
// enumerated, so nothing could close that agreement for more than one step.
var stepExecutors = map[string]func(*Model) error{
	"backup":      stepBackupConfigs,
	"gai":         stepInstallGAIConf,
	"clone":       stepCloneRepo,
	"homebrew":    stepInstallHomebrew,
	"deps":        stepInstallDeps,
	"xcode":       stepInstallXcode,
	"terminal":    stepInstallTerminal,
	"font":        stepInstallFont,
	"shell":       stepInstallShell,
	"wm":          stepInstallWM,
	"nvim":        stepInstallNvim,
	"toolset":     stepInstallToolset,
	"agentskills": stepInstallAgentSkills,
	"officecli":   stepInstallOfficeCLI,
	"wslconfig":   stepInstallWSLConfig,
	"theme":       stepReapplyTheme,
	"cleanup":     stepCleanup,
	"setshell":    stepSetDefaultShell,
}

// captureRunBaseline records the state of every configuration path when the first
// step of a run begins, before any step has written one. The record written when
// the run ends compares the paths against it, so it names what each path's outcome
// was instead of what merely existed before the run (issue #239), and the per-file
// pass below is what a revert reads. No step before the first writes a
// configuration path: the backup step copies out of the home, and the dependency
// and clone steps install packages and a checkout rather than a configuration
// file.
//
// It fills what is empty and leaves what is set, so calling it at every step is
// the same as calling it at the first. A run that follows another one has to
// start from a clean baseline -- beginInstallFromReview clears both -- because a
// baseline is what a run found, and a failed attempt's baseline is not.
func (m *Model) captureRunBaseline() {
	if m.ConfigFingerprints == nil {
		m.ConfigFingerprints = configFingerprints()
	}
	if m.ConfigFileIndex == nil {
		m.ConfigFileIndex = configFileIndex()
	}
}

// executeStep runs the actual installation for a step
func executeStep(stepID string, m *Model) error {
	if dryRun() {
		SendLog(stepID, fmt.Sprintf("DRY RUN: skipping step %q", stepID))
		return nil
	}

	m.captureRunBaseline()

	execute, ok := stepExecutors[stepID]
	if !ok {
		return fmt.Errorf("unknown step: %s", stepID)
	}
	return execute(m)
}

func stepBackupConfigs(m *Model) error {
	stepID := "backup"
	if len(m.ExistingConfigs) == 0 {
		SendLog(stepID, "No existing configs to backup")
		return nil
	}

	SendLog(stepID, fmt.Sprintf("Backing up %d existing configs...", len(m.ExistingConfigs)))

	// Extract just the config keys from the ExistingConfigs slice
	configKeys := make([]string, len(m.ExistingConfigs))
	for i, config := range m.ExistingConfigs {
		configKeys[i] = config
		SendLog(stepID, fmt.Sprintf("  → %s", config))
	}

	backupDir, skipped, err := system.CreateBackup(configKeys)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	m.BackupDir = backupDir
	// Runtime state such as a live Unix socket cannot be copied. Say so instead
	// of leaving the user with a backup that is quietly incomplete.
	if len(skipped) > 0 {
		SendLog(stepID, fmt.Sprintf("Skipped %d entry(ies) that are not regular files:", len(skipped)))
		for _, path := range skipped {
			SendLog(stepID, fmt.Sprintf("  ⤫ %s", path))
		}
	}
	SendLog(stepID, fmt.Sprintf("✓ Backup created at: %s", backupDir))
	return nil
}

// dotfilesRepoURL is the repository the installer clones at run time.
const dotfilesRepoURL = "https://github.com/albersg/dotfiles.git"

// cloneTimeout bounds the clone step. A transfer that stalls neither fails nor
// returns, so an unbounded clone leaves the installer -- and any test that
// drives it -- waiting with no deadline. It is a variable so a test can lower
// it instead of waiting minutes to observe the bound.
var cloneTimeout = 5 * time.Minute

// envDotfilesRepoRef selects the revision to clone, defaulting to the
// repository's default branch.
//
// It exists because the container end-to-end tests build a binary from the
// revision under test and then let it clone the default branch, so the installer
// ran against a repository that did not contain the very files it was written to
// deploy. That mismatch failed those tests twice for two different reasons
// before this override existed.
const envDotfilesRepoRef = "DOTFILES_REPO_REF"

func stepCloneRepo(m *Model) error {
	stepID := "clone"

	// Clone into a directory owned by this run. The previous implementation used
	// the cwd-relative name "dotfiles", so running the installer from a directory
	// that already contained an unrelated "dotfiles" folder removed it with
	// `rm -rf dotfiles` before cloning.
	workDir, err := os.MkdirTemp("", "dotfiles-install-")
	if err != nil {
		return wrapStepError("clone", "Clone Repository",
			"Failed to create a temporary installation directory",
			err)
	}
	repoDir := filepath.Join(workDir, "dotfiles")

	SendLog(stepID, fmt.Sprintf("Cloning repository into %s...", repoDir))
	// --branch accepts a branch or a tag, and an empty value keeps the default
	// branch, so an ordinary installation is unaffected.
	branch := ""
	if ref := os.Getenv(envDotfilesRepoRef); ref != "" {
		branch = fmt.Sprintf(" --branch %q", ref)
		SendLog(stepID, fmt.Sprintf("Using revision %s", ref))
	}
	result := system.RunWithLogs(fmt.Sprintf("git clone --progress%s %s %q", branch, dotfilesRepoURL, repoDir), &system.ExecOptions{Timeout: cloneTimeout}, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		os.RemoveAll(workDir)
		return wrapStepError("clone", "Clone Repository",
			"Failed to clone the repository. Check your internet connection and git installation.",
			result.Error)
	}

	// A clone can exit zero and still leave nothing usable behind (interrupted
	// transfer, missing git). Verify the checkout before any step reads from it.
	if !system.DirExists(filepath.Join(repoDir, ".git")) {
		os.RemoveAll(workDir)
		return wrapStepError("clone", "Clone Repository",
			"Repository was cloned but is not a git checkout",
			fmt.Errorf("%s does not contain a .git directory", repoDir))
	}

	m.WorkDir = workDir
	m.RepoDir = repoDir
	// The clone lives in a temporary directory the cleanup step removes, so the
	// runtime assets the utilities read -- the theme definitions and the WSL
	// template -- are copied out of it now, before any step that reads the
	// checkout. Each copy is what makes its utility offered from any working
	// directory, not only from inside a checkout, and both land under the same
	// per-user data root.
	copyThemeDefinitionsIntoDataDir(stepID, repoDir)
	copyWSLTemplateIntoDataDir(stepID, repoDir)
	SendLog(stepID, "✓ Repository cloned successfully")
	return nil
}

// repoDir returns the checkout created by the clone step. Steps fail loudly
// instead of silently falling back to a guessed, cwd-relative path.
func (m *Model) repoDir() (string, error) {
	if m.RepoDir == "" {
		return "", fmt.Errorf("the repository has not been cloned in this run")
	}
	return m.RepoDir, nil
}

// homebrewInstallerURL is the upstream install script the step downloads. It is
// fetched to a file before it is run instead of through `bash -c "$(curl ...)"`:
// see stepInstallHomebrew.
const homebrewInstallerURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

// ghosttyInstallerURL is the upstream install script the terminal step runs on
// the hosts where Ghostty is not a native package. It is downloaded to a file
// before it is run, for the same reason homebrewInstallerURL is: a failed
// download must keep curl's exit status instead of expanding to an empty
// command substitution. Both the step and the interactive script use it.
const ghosttyInstallerURL = "https://raw.githubusercontent.com/mkasberg/ghostty-ubuntu/HEAD/install.sh"

func stepInstallHomebrew(m *Model) error {
	stepID := "homebrew"

	// Termux doesn't use Homebrew - it uses pkg
	if m.SystemInfo.IsTermux {
		SendLog(stepID, "Skipping Homebrew (Termux uses pkg package manager)")
		return nil
	}

	if system.BrewInstalled() {
		SendLog(stepID, "Homebrew already installed, skipping...")
		m.SystemInfo.HasBrew = true
		return nil
	}

	SendLog(stepID, "Installing Homebrew package manager...")

	// The installer is downloaded and then run as two commands. The old
	// `/bin/bash -c "$(curl -fsSL ...)"` ran curl inside a command substitution,
	// where a failed download expands to the empty string: the shell it was
	// handed ran nothing and still exited 0. A 403 from
	// raw.githubusercontent.com therefore reached the caller as a successful
	// installation. Downloading to a file keeps curl's own exit status, which is
	// the failure this step has to see. The file lives in a temporary directory
	// this step owns and is removed on every path out.
	installer, err := os.CreateTemp("", "homebrew-install-*.sh")
	if err != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to create a temporary file for the Homebrew install script",
			err)
	}
	installerPath := installer.Name()
	if err := installer.Close(); err != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to create a temporary file for the Homebrew install script",
			err)
	}
	defer func() { _ = os.Remove(installerPath) }()

	logLine := func(line string) { SendLog(stepID, line) }

	if result := system.RunWithLogs(
		fmt.Sprintf("curl -fsSL -o %q %s", installerPath, homebrewInstallerURL), nil, logLine); result.Error != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to download the Homebrew install script. Check your internet connection and whether raw.githubusercontent.com is reachable; a proxy or firewall can return an HTTP error there.",
			result.Error)
	}

	if result := system.RunWithLogs(fmt.Sprintf("/bin/bash %q", installerPath), nil, logLine); result.Error != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to run the Homebrew install script",
			result.Error)
	}

	// Tell the rest of the run that Homebrew exists. SystemInfo is detected once
	// at startup, and the install script only exports brew into its own child
	// shell, so without this refresh every later step keeps using the native
	// package manager and ignores the Homebrew it just installed.
	//
	// The refresh is also the step's success test. The script can exit 0 having
	// installed nothing, and that is exactly what the empty-download idiom above
	// produced, so a missing brew is reported as the failure it is instead of
	// being logged as a warning under a "✓ installed successfully" line. The
	// message names the download because that is the cause this guard exists to
	// catch.
	if !system.BrewInstalled() {
		return wrapStepError("homebrew", "Install Homebrew",
			"Homebrew is still missing after the install script ran. The download or the script itself most likely failed; check your internet connection and whether raw.githubusercontent.com is reachable, then run the installer again.",
			fmt.Errorf("brew not found at %s after the install script completed", system.GetBrewPrefix()))
	}
	m.SystemInfo.HasBrew = true
	SendLog(stepID, fmt.Sprintf("✓ Homebrew installed at %s", system.GetBrewPrefix()))

	// Add to PATH
	homeDir := os.Getenv("HOME")
	brewPrefix := system.GetBrewPrefix()

	shellConfig := fmt.Sprintf(`eval "$(%s/bin/brew shellenv)"`, brewPrefix)

	SendLog(stepID, "Configuring shell to use Homebrew...")
	// Add to common shell configs. Both destinations are spelled out as literals
	// rather than read from a loop variable, because the guard that checks every
	// home path the installer writes against ConfigPaths() can only classify a
	// destination it can resolve (TestEveryHomePathTheInstallerWritesIsClassified).
	for _, rcPath := range []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".zshrc"),
	} {
		if f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString("\n" + shellConfig + "\n")
			f.Close()
		}
	}

	// Source it now
	system.Run(shellConfig, nil)

	SendLog(stepID, "✓ Homebrew installed successfully")
	return nil
}

// envSkipDeps lets a run proceed without the dependency step. It exists for
// environments where sudo cannot prompt for a password (containers, CI, the
// non-interactive installer) and the base packages are already present or are
// installed another way. The failure below names it, so a run that cannot
// install dependencies has a documented way forward instead of a raw sudo
// error.
const envSkipDeps = "DOTFILES_SKIP_DEPS"

// skipDeps reports whether the dependency step was explicitly skipped.
func skipDeps() bool {
	switch os.Getenv(envSkipDeps) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// envSkipToolset lets a run proceed without the toolset step. It exists for
// containers, CI and anyone who does not want the declared machine toolset
// installed: the step provisions roughly sixty entries from the Brewfile, which
// is more than those environments need and, in a container, more time than the
// run is worth. The step reports the skip so it is never mistaken for a silent
// failure.
const envSkipToolset = "DOTFILES_SKIP_TOOLSET"

// skipToolset reports whether the toolset step was explicitly skipped.
func skipToolset() bool {
	switch os.Getenv(envSkipToolset) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// baseDependencies names the tools every later step relies on, per package
// manager. Homebrew is listed separately because it is preferred whenever it is
// present, matching planPlatformInstall.
func baseDependencies() platformPackages {
	return platformPackages{
		Brew:   "curl file git wget unzip fontconfig",
		Arch:   "base-devel curl file git wget unzip fontconfig",
		Fedora: "@development-tools curl file git wget unzip fontconfig",
		Debian: "build-essential curl file git unzip fontconfig procps",
	}
}

// depsForInstall returns the base dependency package set for a host, with the
// WSL-only wslu package added and every distribution column passed through the
// availability filter. It is the single source of truth for the dependency step:
// stepInstallDeps executes the result and getDepsScript renders it, so a name the
// distribution does not carry cannot reach apt, pacman or dnf on either path
// while the other path keeps it.
func depsForInstall(m *Model) (platformPackages, []string) {
	deps := baseDependencies()
	if m.SystemInfo.IsWSL {
		// wslu provides wslview/wslpath integration on Debian-like WSL. Debian 12
		// does not carry it, so it travels through the filter below and is
		// reported rather than aborting the whole apt transaction.
		deps.Debian += " wslu"
	}
	return filterPlatformPackages(deps)
}

// filterPlatformPackages runs every distribution column through its
// availability filter, dropping the names the distribution does not carry and
// reporting the Debian ones separately, exactly as the shell, window-manager and
// Neovim constructors do.
func filterPlatformPackages(packages platformPackages) (platformPackages, []string) {
	debian, debianGaps := debianPackages(strings.Fields(packages.Debian)...)
	arch, archGaps := archPackages(strings.Fields(packages.Arch)...)
	fedora, fedoraGaps := fedoraPackages(strings.Fields(packages.Fedora)...)

	packages.Debian = debian
	packages.Arch = arch
	packages.Fedora = fedora
	packages.archUnavailable = archGaps
	packages.fedoraUnavailable = fedoraGaps
	return packages, debianGaps
}

// usesApt mirrors the Debian branch of planPlatformInstall: apt runs only for a
// Debian-like host without Homebrew.
func usesApt(m *Model) bool {
	return (m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux) && !m.SystemInfo.HasBrew
}

// usesPacman mirrors the Arch branch of planPlatformInstall: pacman runs
// only for an Arch host without Homebrew, because Homebrew takes over the
// install whenever it is present, exactly as it does for Debian and Fedora. The
// Arch package lists are the ones the pacman route reaches and the ones the Arch
// filter protects; with Homebrew present the unfiltered Brew list is reached
// instead.
func usesPacman(m *Model) bool {
	return m.SystemInfo.OS == system.OSArch && !m.SystemInfo.HasBrew
}

// usesDnf mirrors the Fedora branch of planPlatformInstall: dnf runs only
// for a Fedora host without Homebrew, because Homebrew takes over the install
// whenever it is present, exactly as it does for Debian. The Fedora package
// lists are the ones the dnf route reaches and the ones the Fedora filter
// protects; with Homebrew present the unfiltered Brew list is reached instead.
func usesDnf(m *Model) bool {
	return m.SystemInfo.OS == system.OSFedora && !m.SystemInfo.HasBrew
}

// componentPresentAfterInstall reports whether a component is on the machine
// once an install attempt has finished. The install result is authoritative for
// failure: a route that returned an error did not install it. On success the
// component is looked up on PATH and in the Homebrew prefix, because Homebrew
// writes its binaries into its own prefix and this process's PATH does not
// necessarily contain it moments after a successful install. A PATH-only lookup
// would report a component that was installed moments earlier as missing.
func componentPresentAfterInstall(result *system.ExecResult, command string) bool {
	if result != nil && result.Error != nil {
		return false
	}
	if system.CommandExists(command) {
		return true
	}
	prefix := os.Getenv("HOMEBREW_PREFIX")
	if prefix == "" {
		prefix = system.GetBrewPrefix()
	}
	info, err := os.Stat(filepath.Join(prefix, "bin", command))
	return err == nil && !info.IsDir()
}

// dependencyManualCommands returns the root-only commands the user can run by
// hand when sudo cannot prompt. It mirrors the dispatch of the dependency
// install so the guidance cannot drift from what would actually run.
func dependencyManualCommands(m *Model, deps platformPackages) string {
	switch {
	case usesPacman(m):
		return "sudo pacman -S --needed --noconfirm " + deps.Arch
	case usesDnf(m):
		return "sudo dnf install -y " + deps.Fedora
	case usesApt(m):
		return "sudo apt-get update\nsudo apt-get install -y " + deps.Debian
	default:
		return "brew install " + deps.Brew
	}
}

// sudoPasswordUnavailable reports whether a failed command failed because sudo
// could not obtain a password. The non-interactive installer and container runs
// have no TTY to prompt on, so sudo refuses instead of running the command.
func sudoPasswordUnavailable(result *system.ExecResult) bool {
	if result == nil || result.Error == nil {
		return false
	}
	message := strings.ToLower(result.Stderr)
	if message == "" {
		message = strings.ToLower(result.Error.Error())
	}
	for _, marker := range []string{
		"a password is required",
		"no tty present",
		"a terminal is required",
		"askpass",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// dependencyInstallError turns a failed dependency command into a visible
// failure. When sudo could not prompt for a password it names the commands to
// run and how to skip the step, instead of surfacing the raw sudo error. Every
// other failure keeps the step visible as a failure.
func dependencyInstallError(m *Model, deps platformPackages, result *system.ExecResult) error {
	if sudoPasswordUnavailable(result) {
		return wrapStepError("deps", "Install Dependencies",
			fmt.Sprintf("Installing dependencies needs root, and sudo could not ask for a password in this run.\n"+
				"Run these commands in a terminal, then run the installer again:\n\n%s\n\n"+
				"To skip installing dependencies instead, set %s=1 and run the installer again.",
				dependencyManualCommands(m, deps), envSkipDeps),
			nil)
	}
	return wrapStepError("deps", "Install Dependencies",
		"Failed to install base dependencies",
		result.Error)
}

func stepInstallDeps(m *Model) error {
	stepID := "deps"

	if skipDeps() {
		SendLog(stepID, fmt.Sprintf("Skipping dependency installation (%s is set)", envSkipDeps))
		return nil
	}

	// Termux: use pkg (no sudo needed)
	// Check both SystemInfo and Choices.OS for redundancy
	isTermux := m.SystemInfo.IsTermux || m.Choices.OS == "termux"
	if isTermux {
		SendLog(stepID, "Updating Termux packages...")
		result := system.RunPkgWithLogs("update", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("deps", "Install Dependencies",
				"Failed to update Termux packages",
				result.Error)
		}
		result = system.RunPkgWithLogs("upgrade -y", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			// Upgrade failures are not critical
			SendLog(stepID, "Warning: package upgrade had issues, continuing...")
		}
		SendLog(stepID, "Installing base dependencies...")
		result = system.RunPkgInstall("git curl", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("deps", "Install Dependencies",
				"Failed to install base dependencies on Termux",
				result.Error)
		}
		return nil
	}

	// Everything below uses the same package selection and package-manager
	// preference as the interactive script: depsForInstall picks and filters the
	// names, planPlatformInstall picks the manager and the index refresh, and both
	// paths render the same decision. The distribution is read from OS, which
	// detection fills in on WSL too, so Fedora-on-WSL runs dnf and Arch-on-WSL
	// runs pacman.
	deps, debianGaps := depsForInstall(m)
	plan := planPlatformInstall(m, deps)

	if plan.Manager == "apt-get" {
		logDebianUnavailable(stepID, debianGaps)
	}
	if plan.Update != "" {
		result := runSudoWithLogs(plan.Update, nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return dependencyInstallError(m, deps, result)
		}
	}

	result := runPlatformInstall(m, plan, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return dependencyInstallError(m, deps, result)
	}

	// wslu is only reported as installed when it was actually in the command. On
	// Debian the filter drops it, so claiming it was installed would be false.
	if m.SystemInfo.IsWSL && !m.SystemInfo.HasBrew && !slices.Contains(debianGaps, "wslu") {
		SendLog(stepID, "✓ WSL utilities (wslu) installed for clipboard/browser integration")
	}
	return nil
}

func stepInstallXcode(m *Model) error {
	result := system.Run("xcode-select --install", nil)
	if result.Error != nil {
		// xcode-select returns error if already installed, which is fine
		if result.ExitCode == 1 && strings.Contains(result.Stderr, "already installed") {
			return nil
		}
		return wrapStepError("xcode", "Install Xcode CLI",
			"Failed to install Xcode Command Line Tools. You may need to install them manually from the App Store.",
			result.Error)
	}
	return nil
}

// supportedTerminals lists, in the order the help text shows them, the
// --terminal values this installer will honour on a platform. Only one value
// varies: kitty has a Homebrew cask on macOS and no installation route at all
// on the platforms this installer otherwise supports.
//
// This is the single source of truth for that rule. The --terminal flag
// description, the CLI validator and stepInstallTerminal all read it, so the
// value the tool refuses and the values it names as supported cannot drift
// apart. "none" is listed because the CLI accepts it, not because it is a
// terminal that gets installed.
func supportedTerminals(onMac bool) []string {
	if onMac {
		return []string{"alacritty", "wezterm", "kitty", "ghostty", "none"}
	}
	return []string{"alacritty", "wezterm", "ghostty", "none"}
}

// SupportedTerminals returns the --terminal values this installer honours on
// the platform named by goos (a runtime.GOOS value).
func SupportedTerminals(goos string) []string {
	return supportedTerminals(goos == "darwin")
}

// terminalSupported reports whether --terminal=<terminal> is a value this
// installer will install on the given platform.
func terminalSupported(onMac bool, terminal string) bool {
	for _, supported := range supportedTerminals(onMac) {
		if terminal == supported {
			return true
		}
	}
	return false
}

// unsupportedTerminalError is the refusal for a --terminal value this installer
// will not honour on a platform. It names the values it does support there, so
// the caller is told what to ask for instead of only what was rejected.
func unsupportedTerminalError(onMac bool, terminal string) error {
	return fmt.Errorf("invalid terminal: %s (valid: %s)", terminal, strings.Join(supportedTerminals(onMac), ", "))
}

// ValidateTerminal reports whether --terminal=<terminal> is honoured on the
// platform named by goos. The CLI refuses an unsupported value here, before
// anything is planned or touched, so an input this tool will not honour never
// reaches an installation step.
func ValidateTerminal(terminal, goos string) error {
	onMac := goos == "darwin"
	if terminalSupported(onMac, terminal) {
		return nil
	}
	return unsupportedTerminalError(onMac, terminal)
}

// kittyAction is what the kitty branch of stepInstallTerminal does once the
// platform and the result of the presence check are known.
type kittyAction int

const (
	// kittyRefuse means this platform has no route this installer will take.
	kittyRefuse kittyAction = iota
	// kittyInstall means the binary was verified absent and macOS is honoured.
	kittyInstall
	// kittyPresent means the binary was verified present.
	kittyPresent
)

// kittyActionFor decides what the kitty branch does for a platform and for the
// result of the presence check. The only way to reach the "already installed"
// report is present == true, which is what it used to get wrong: the old branch
// fell through to that report on Linux, for a binary it had never looked for.
func kittyActionFor(onMac, present bool) kittyAction {
	if !terminalSupported(onMac, "kitty") {
		return kittyRefuse
	}
	if present {
		return kittyPresent
	}
	return kittyInstall
}

// weztermInstallCommands returns the shell command lines that install WezTerm
// on a host, in order. It is the single source of truth for the WezTerm route:
// stepInstallTerminal runs each line through the log-streaming runner and
// getTerminalScript renders the same lines into the interactive script, so the
// two paths cannot disagree about how WezTerm is installed. That disagreement is
// exactly what the interactive step got wrong for Debian/Ubuntu and WSL, where
// it returned an empty script while the non-interactive step installed through
// Homebrew.
//
// The Debian/Ubuntu and WSL route is a Homebrew formula, so the tap precedes the
// install, and both lines spell out the Homebrew prefix so a brew the current
// PATH does not carry is still reached.
func weztermInstallCommands(si *system.SystemInfo) []string {
	brew := filepath.Join(system.GetBrewPrefix(), "bin", "brew")
	switch si.OS {
	case system.OSArch:
		return []string{"sudo -n pacman -S --noconfirm wezterm"}
	case system.OSFedora:
		return []string{
			"sudo -n dnf copr enable -y wezfurlong/wezterm-nightly",
			"sudo -n dnf install -y wezterm",
		}
	case system.OSMac:
		return []string{brew + " install --cask wezterm"}
	default:
		return []string{
			brew + " tap wez/wezterm-linuxbrew",
			brew + " install wezterm",
		}
	}
}

// terminalConfigSource resolves a terminal asset inside the checkout created by
// the clone step for this run. The terminal step used to read its sources from
// the literal "dotfiles" directory under the working directory, which only
// resolved when the installer happened to run from a directory that contained a
// checkout of that name. Resolving through the recorded checkout keeps the copy
// independent of the working directory.
func terminalConfigSource(m *Model, asset string) (string, error) {
	repoDir, err := m.repoDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoDir, asset), nil
}

func stepInstallTerminal(m *Model) error {
	terminal := m.Choices.Terminal
	homeDir := os.Getenv("HOME")
	stepID := "terminal"

	switch terminal {
	case "alacritty":
		if !system.CommandExists("alacritty") {
			SendLog(stepID, "Installing Alacritty...")
			var result *system.ExecResult
			if m.SystemInfo.OS == system.OSArch {
				result = runSudoWithLogs("pacman -S --noconfirm alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSMac {
				result = system.RunBrewWithLogs("install --cask alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSFedora {
				// Fedora: install from dnf
				result = runSudoWithLogs("dnf install -y alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux {
				// Debian/Ubuntu: compile from source (PPAs are unreliable)
				SendLog(stepID, "Building Alacritty from source...")
				SendLog(stepID, "Installing build dependencies...")
				result = runSudoWithLogs("apt-get install -y cmake pkg-config libfreetype6-dev libfontconfig1-dev libxcb-xfixes0-dev libxkbcommon-dev python3 gzip scdoc git curl", nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to install build dependencies",
						result.Error)
				}
				// Install Rust/Cargo only for this build
				cargoPath := filepath.Join(homeDir, ".cargo/bin/cargo")
				if !system.CommandExists("cargo") && !system.CommandExists(cargoPath) {
					SendLog(stepID, "Installing Rust/Cargo toolchain...")
					result = system.RunWithLogs("curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y", nil, func(line string) {
						SendLog(stepID, line)
					})
					if result.Error != nil {
						return wrapStepError("terminal", "Install Alacritty",
							"Failed to install Rust",
							result.Error)
					}
					cargoPath = filepath.Join(homeDir, ".cargo/bin/cargo")
				}
				// Clone and build Alacritty
				SendLog(stepID, "Cloning Alacritty repository...")
				alacrittyDir := filepath.Join(os.TempDir(), "alacritty-build")
				os.RemoveAll(alacrittyDir)
				// The clone and its release target directory are gigabytes, and every
				// way out of this block below is an error return. Removing the tree
				// only after the last command succeeded left the whole build in the
				// shared temporary directory whenever anything failed, which on a
				// tmpfs distribution is memory no process can reclaim. The defer is
				// what makes the removal follow the block instead of its happy path.
				defer os.RemoveAll(alacrittyDir)
				result = system.RunWithLogs(fmt.Sprintf("git clone https://github.com/alacritty/alacritty.git %s", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to clone Alacritty repository",
						result.Error)
				}
				SendLog(stepID, "Building Alacritty (this may take 5-10 minutes)...")
				if !system.CommandExists("cargo") {
					cargoPath = filepath.Join(homeDir, ".cargo/bin/cargo")
				} else {
					cargoPath = "cargo"
				}
				result = system.RunWithLogs(fmt.Sprintf("%s build --release --manifest-path %s/Cargo.toml", cargoPath, alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to build Alacritty",
						result.Error)
				}
				SendLog(stepID, "Installing Alacritty binary...")
				result = runSudoWithLogs(fmt.Sprintf("cp %s/target/release/alacritty /usr/local/bin/alacritty", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to install Alacritty binary",
						result.Error)
				}
				runSudoWithLogs(fmt.Sprintf("cp %s/extra/linux/Alacritty.desktop /usr/share/applications/", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				SendLog(stepID, "✓ Alacritty built and installed from source")
			} else {
				return wrapStepError("terminal", "Install Alacritty",
					"Unsupported operating system for Alacritty installation",
					fmt.Errorf("OS type: %v", m.SystemInfo.OS))
			}
			if result.Error != nil {
				return wrapStepError("terminal", "Install Alacritty",
					"Failed to install Alacritty terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "Alacritty already installed")
		}
		SendLog(stepID, "Copying Alacritty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/alacritty")); err != nil {
			return wrapStepError("terminal", "Install Alacritty",
				"Failed to create Alacritty config directory",
				err)
		}
		alacrittySource, err := terminalConfigSource(m, repoAssetAlacritty)
		if err != nil {
			return wrapStepError("terminal", "Install Alacritty",
				"Failed to copy Alacritty configuration",
				err)
		}
		if err := system.CopyFile(alacrittySource, filepath.Join(homeDir, ".config/alacritty/alacritty.toml")); err != nil {
			return wrapStepError("terminal", "Install Alacritty",
				"Failed to copy Alacritty configuration",
				err)
		}
		SendLog(stepID, "✓ Alacritty configured")

	case "wezterm":
		if !system.CommandExists("wezterm") {
			SendLog(stepID, "Installing WezTerm...")
			var result *system.ExecResult
			for _, command := range weztermInstallCommands(m.SystemInfo) {
				result = system.RunWithLogs(command, nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					break
				}
			}
			if result != nil && result.Error != nil {
				return wrapStepError("terminal", "Install WezTerm",
					"Failed to install WezTerm terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "WezTerm already installed")
		}
		SendLog(stepID, "Copying WezTerm configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/wezterm")); err != nil {
			return wrapStepError("terminal", "Install WezTerm",
				"Failed to create WezTerm config directory",
				err)
		}
		weztermSource, err := terminalConfigSource(m, repoAssetWezterm)
		if err != nil {
			return wrapStepError("terminal", "Install WezTerm",
				"Failed to copy WezTerm configuration",
				err)
		}
		if err := system.CopyFile(weztermSource, filepath.Join(homeDir, ".config/wezterm/wezterm.lua")); err != nil {
			return wrapStepError("terminal", "Install WezTerm",
				"Failed to copy WezTerm configuration",
				err)
		}
		SendLog(stepID, "✓ WezTerm configured")

	case "kitty":
		onMac := m.SystemInfo.OS == system.OSMac
		// Only macOS has a kitty route this installer takes, and the "already
		// installed" report is only reachable once CommandExists verified the
		// binary. The old branch installed only on macOS and ran its else branch
		// everywhere else, so on Linux it reported "Kitty already installed"
		// about a binary it had never checked for.
		switch kittyActionFor(onMac, system.CommandExists("kitty")) {
		case kittyRefuse:
			return wrapStepError("terminal", "Install Kitty",
				"Kitty is only installable on macOS",
				unsupportedTerminalError(onMac, terminal))
		case kittyInstall:
			SendLog(stepID, "Installing Kitty...")
			result := system.RunBrewWithLogs("install --cask kitty", nil, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				return wrapStepError("terminal", "Install Kitty",
					"Failed to install Kitty terminal emulator",
					result.Error)
			}
		case kittyPresent:
			SendLog(stepID, "Kitty already installed")
		}
		SendLog(stepID, "Copying Kitty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/kitty")); err != nil {
			return wrapStepError("terminal", "Install Kitty",
				"Failed to create Kitty config directory",
				err)
		}
		kittySource, err := terminalConfigSource(m, repoAssetKitty)
		if err != nil {
			return wrapStepError("terminal", "Install Kitty",
				"Failed to copy Kitty configuration",
				err)
		}
		if err := system.CopyDir(kittySource, filepath.Join(homeDir, ".config", "kitty")); err != nil {
			return wrapStepError("terminal", "Install Kitty",
				"Failed to copy Kitty configuration",
				err)
		}
		SendLog(stepID, "✓ Kitty configured")

	case "ghostty":
		if !system.CommandExists("ghostty") {
			SendLog(stepID, "Installing Ghostty...")
			var result *system.ExecResult
			if m.SystemInfo.OS == system.OSArch {
				result = runSudoWithLogs("pacman -S --noconfirm ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSFedora {
				// Fedora: enable COPR and install
				runSudoSilently("dnf copr enable -y pgdev/ghostty", nil)
				result = runSudoWithLogs("dnf install -y ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSMac {
				result = system.RunBrewWithLogs("install --cask ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else {
				// The installer is downloaded and then run as two commands, for the
				// same reason the Homebrew step does it: the old
				// `/bin/bash -c "$(curl -fsSL ...)"` ran curl inside a command
				// substitution, where a failed download expands to the empty
				// string: the shell it was handed ran nothing and still exited 0.
				// A 403 from raw.githubusercontent.com therefore reached the
				// caller as a successful installation and the step copied
				// configuration under a success line. Downloading to a file keeps
				// curl's own exit status, which is the failure this step has to
				// see. The file lives in a temporary directory this step owns and
				// is removed on every path out.
				installer, err := os.CreateTemp("", "ghostty-install-*.sh")
				if err != nil {
					return wrapStepError("terminal", "Install Ghostty",
						"Failed to create a temporary file for the Ghostty install script",
						err)
				}
				installerPath := installer.Name()
				if err := installer.Close(); err != nil {
					return wrapStepError("terminal", "Install Ghostty",
						"Failed to create a temporary file for the Ghostty install script",
						err)
				}
				defer func() { _ = os.Remove(installerPath) }()

				if result := system.RunWithLogs(
					fmt.Sprintf("curl -fsSL -o %q %s", installerPath, ghosttyInstallerURL), nil, func(line string) {
						SendLog(stepID, line)
					}); result.Error != nil {
					return wrapStepError("terminal", "Install Ghostty",
						"Failed to download the Ghostty install script. Check your internet connection and whether raw.githubusercontent.com is reachable; a proxy or firewall can return an HTTP error there.",
						result.Error)
				}

				result = system.RunWithLogs(fmt.Sprintf("/bin/bash %q", installerPath), nil, func(line string) {
					SendLog(stepID, line)
				})
			}
			if result.Error != nil {
				return wrapStepError("terminal", "Install Ghostty",
					"Failed to install Ghostty terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "Ghostty already installed")
		}
		SendLog(stepID, "Copying Ghostty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/ghostty")); err != nil {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to create Ghostty config directory",
				err)
		}
		ghosttySource, err := terminalConfigSource(m, repoAssetGhostty)
		if err != nil {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to copy Ghostty configuration",
				err)
		}
		if err := system.CopyDir(ghosttySource, filepath.Join(homeDir, ".config", "ghostty")); err != nil {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to copy Ghostty configuration",
				err)
		}
		// dotfiles-ghostty/themes/catppuccin-mocha.conf is the provenance of
		// themes/catppuccin-mocha.toml, not a live theme: the config never references
		// it, and a copy in the user's directory would be a palette no switch keeps
		// current. It stays in the checkout and out of the installed tree.
		if err := os.Remove(filepath.Join(homeDir, ".config", "ghostty", "themes", "catppuccin-mocha.conf")); err != nil && !os.IsNotExist(err) {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to leave the stale theme copy out of the Ghostty configuration",
				err)
		}
		SendLog(stepID, "✓ Ghostty configured")
	}

	return nil
}

// The Linux font step downloads the Nerd Fonts release archive and extracts it.
// The release URL is pinned, and the archive is fetched into a scratch directory
// the step owns instead of into the font directory: see stepInstallFont.
//
// The pin is a minimum, not a preference. v3.3.0, which this replaced, carries no
// `dev-yaml` (U+E8EB) at all: `eza` draws a `.yaml` file with that codepoint, so
// every `.yaml` in a listing came out as a blank box on Linux, WSL and Termux
// while macOS -- whose cask installs a later release of the same tree -- drew it
// correctly. Reading the codepoint out of the release's own `glyphnames.json` is
// what separates that from a font that failed to install, and what a later bump
// has to be checked against: `U+F086F` and `U+F15DE` are present at both tags,
// and `U+E8EB` only at the later one. Lowering this URL restores the blank box.
const (
	mesloArchiveURL  = "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.5.1/Meslo.zip"
	mesloArchiveName = "Meslo.zip"

	// mesloArchiveFamily is the family the pinned Meslo archive declares. The
	// archive name does not encode it, so it was read once from a face in the
	// release -- MesloLGSNerdFont-Regular.ttf declares "MesloLGS Nerd Font" -- and
	// recorded here rather than typed again wherever the family is needed. The
	// macOS cask downloads the same release tree (v3.5.1 at the time of writing)
	// and installs the same family.
	mesloArchiveFamily = "MesloLGS Nerd Font"

	fontScratchDirPrefix = ".meslo-"

	// linuxFontFamilyQuery is how the step asks fontconfig which families this
	// machine can already resolve. `fc-list : family` prints one line per face,
	// with the face's families comma-separated, and changes nothing, so asking
	// is cheap and read-only. The answer decides whether the pinned archive is
	// downloaded at all and, after extraction, whether the install may be
	// reported as done. It is one constant so the presence check and the
	// verification ask fontconfig the same question.
	linuxFontFamilyQuery = "fc-list : family"

	// termuxFontDirName, termuxFontFileName, termuxFontURL and termuxFontFace
	// describe Termux's font slot. Termux has no fontconfig: its terminal reads
	// one file, so the honest presence check is the file rather than a family.
	//
	// The face is the one the decision names -- Meslo (see #265) -- not the
	// JetBrainsMono this step used to fetch while the wizard said Meslo. The URL
	// is the S (small line gap, the terminal variant) Meslo face in the same
	// nerd-fonts repository the other platforms' archive comes from, and
	// termuxFontFace is the family that file declares, read from its own name
	// table rather than assumed: the v3 patcher names it "MesloLGS Nerd Font".
	termuxFontDirName  = ".termux"
	termuxFontFileName = "font.ttf"
	termuxFontFace     = "MesloLGS Nerd Font"
	termuxFontURL      = "https://github.com/ryanoasis/nerd-fonts/raw/HEAD/patched-fonts/Meslo/S/MesloLGSNerdFont-Regular.ttf"
)

// fontInstallFamily is the family the font step installs for a platform. Termux
// reads one file from its own slot; every other platform extracts the pinned
// Meslo archive, on macOS through the cask that downloads the same release tree.
// The interface reads this one decision, so a screen cannot name a face the step
// does not put on the machine.
func fontInstallFamily(isTermux bool) string {
	if isTermux {
		return termuxFontFace
	}
	return mesloArchiveFamily
}

// fontInstallLabel is the name the interface shows for the font the step
// installs. It is computed from the family rather than typed beside it, so a
// screen cannot claim Meslo over a family the step did not install: a Meslo face
// gets the installer's own phrase, and any other family is shown as itself.
func fontInstallLabel(family string) string {
	if strings.HasPrefix(family, "Meslo") {
		return "Meslo Nerd Font"
	}
	return family
}

// linuxFontFamilyResolves reports whether fontconfig's family listing resolves
// family. Each line names one face and its families are comma-separated, so the
// whole family name is compared rather than a substring: a different face that
// merely shares a prefix must not satisfy the check.
func linuxFontFamilyResolves(family, listing string) bool {
	for _, line := range strings.Split(listing, "\n") {
		for _, name := range strings.Split(line, ",") {
			if strings.TrimSpace(name) == family {
				return true
			}
		}
	}
	return false
}

// installedLinuxFontFamily asks fontconfig whether family is already installed.
// It returns known=false when fc-list could not be asked at all -- a machine
// without fontconfig -- which is a different answer from "absent": the first
// means the download should still run, and the second means the family simply
// is not there yet. Nothing is downloaded to answer the question.
func installedLinuxFontFamily(family string) (resolved, known bool) {
	result := system.RunWithLogs(linuxFontFamilyQuery, nil, nil)
	if result.Error != nil {
		return false, false
	}
	return linuxFontFamilyResolves(family, result.Output), true
}

// fontPresence is what the font step's own check found on this machine, so a
// screen can state the measurement instead of asking the user to remember it.
// Measured is false when the step would not ask at all -- the macOS branch
// installs through the Homebrew cask and never reads the font store first --
// which is a different answer from "not installed".
type fontPresence struct {
	Measured  bool
	Installed bool
}

// fontPresenceProbe is the model's one call into the font step's own presence
// check. It is indirected for two reasons: on Linux the check shells out to
// fc-list, so the model runs it once when it is built and every render reads the
// result instead; and a test can describe a machine with the font or without one
// rather than depending on the machine that runs the test. Rendering never calls
// this.
var fontPresenceProbe = fontPresenceForHost

// fontPresenceForHost answers the question the font step asks before it
// downloads anything, for the host detection named. It is the same check the
// step makes: the Termux slot's file when the step's Termux branch would run,
// the fontconfig family listing everywhere the Linux branch would, and no check
// at all on macOS, where the step installs through Homebrew's cask. The family
// is the one constant the step itself installs, so the screen cannot report a
// different family's presence.
func fontPresenceForHost(info *system.SystemInfo) fontPresence {
	if info == nil {
		return fontPresence{}
	}
	if info.IsTermux {
		path := filepath.Join(info.HomeDir, termuxFontDirName, termuxFontFileName)
		if stat, err := os.Stat(path); err == nil && stat.Size() > 0 {
			return fontPresence{Measured: true, Installed: true}
		}
		return fontPresence{Measured: true}
	}
	if info.OS == system.OSMac {
		return fontPresence{}
	}
	resolved, known := installedLinuxFontFamily(mesloArchiveFamily)
	return fontPresence{Measured: known, Installed: resolved}
}

func stepInstallFont(m *Model) error {
	homeDir := os.Getenv("HOME")
	stepID := "font"

	// Termux: fonts work differently. Its terminal reads exactly one file in its
	// own slot, so a non-empty file already there is the whole answer and the
	// download is skipped before anything is created or fetched.
	isTermux := m.SystemInfo.IsTermux || m.Choices.OS == "termux"
	if isTermux {
		termuxDir := filepath.Join(homeDir, termuxFontDirName)
		termuxFontPath := filepath.Join(termuxDir, termuxFontFileName)

		if info, err := os.Stat(termuxFontPath); err == nil && info.Size() > 0 {
			SendLog(stepID, fmt.Sprintf("%s is already installed at %s, so nothing was downloaded.", termuxFontFace, termuxFontPath))
			SendLog(stepID, "✓ Font already installed - restart Termux to apply")
			return nil
		}

		SendLog(stepID, fmt.Sprintf("Downloading %s for Termux...", termuxFontFace))
		if err := system.EnsureDir(termuxDir); err != nil {
			return wrapStepError("font", "Install Nerd Font",
				"Failed to create .termux directory",
				err)
		}

		// Download the single TTF file Termux reads.
		result := system.RunWithLogs(fmt.Sprintf("curl -fsSL -o %s %s", termuxFontPath, termuxFontURL), nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("font", "Install Nerd Font",
				"Failed to download font. Check your internet connection.",
				result.Error)
		}

		// Verify the slot the terminal will read instead of the transfer that was
		// attempted: curl exits 0 on an empty body, and a zero-length font.ttf is
		// not an installed font.
		if info, err := os.Stat(termuxFontPath); err != nil {
			return wrapStepError("font", "Install Nerd Font",
				fmt.Sprintf("The download reported success, but %s is not there, so Termux has no font to read.", termuxFontPath),
				err)
		} else if info.Size() == 0 {
			return wrapStepError("font", "Install Nerd Font",
				fmt.Sprintf("The download wrote an empty %s, so Termux has no font to read.", termuxFontPath),
				nil)
		}

		SendLog(stepID, "Reloading Termux settings...")
		system.Run("termux-reload-settings", nil)
		SendLog(stepID, fmt.Sprintf("✓ Font installed - %s is in place; restart Termux to apply", termuxFontPath))
		return nil
	}

	// The family the non-Termux branch installs, and the label this branch's own
	// messages name it by. Both are computed from the one family constant, so the
	// step's log cannot drift from the family it puts on the machine.
	fontName := fontInstallLabel(mesloArchiveFamily)

	if m.SystemInfo.OS == system.OSMac {
		SendLog(stepID, "Installing "+fontName+"...")
		result := system.RunBrewWithLogs("install --cask font-meslo-lg-nerd-font", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("font", "Install "+fontName,
				"Failed to install font via Homebrew. Try installing manually from https://www.nerdfonts.com/",
				result.Error)
		}
		SendLog(stepID, "✓ Font installed")
		return nil
	}

	// Linux
	fontDir := filepath.Join(homeDir, ".local/share/fonts")

	// Ask the machine before downloading. The pinned archive is about 116 MB
	// (116,387,026 bytes at v3.5.1, not the 347 MB the old comment claimed), the
	// most expensive network operation in the installer apart from a package
	// install, and without this check it is re-fetched and re-extracted over
	// identical files on every run. fc-list is asked whether the family the
	// terminal configurations name already resolves; when it does, the download,
	// the extraction and the cache rebuild are skipped and the step reports what
	// answered. The family comes from mesloArchiveFamily, the one family the pinned
	// archive declares and the shipped configurations are guarded against
	// (font_family_test.go), so the check cannot name a family the archive does not
	// provide.
	if resolved, known := installedLinuxFontFamily(mesloArchiveFamily); known && resolved {
		SendLog(stepID, fmt.Sprintf("%s is already installed: %s resolves it, so the font archive was not downloaded.", mesloArchiveFamily, linuxFontFamilyQuery))
		SendLog(stepID, "✓ Font already installed")
		return nil
	}

	SendLog(stepID, "Creating fonts directory...")
	if err := system.EnsureDir(fontDir); err != nil {
		return wrapStepError("font", "Install "+fontName,
			"Failed to create fonts directory",
			err)
	}

	// The archive used to be downloaded into fontDir itself and never removed, so
	// the ~108 MB zip stayed in a directory fontconfig scans. It is fetched into
	// a scratch directory this step owns instead: inside fontDir, so the download
	// lands on the same filesystem rather than in a memory-backed /tmp, but never
	// the font directory itself, and removing that directory covers the failure
	// paths too. Doing this also keeps the cleanup away from every other file in
	// ~/.local/share/fonts, which belongs to the user.
	scratchDir, err := os.MkdirTemp(fontDir, fontScratchDirPrefix)
	if err != nil {
		return wrapStepError("font", "Install "+fontName,
			"Failed to create a temporary directory for the font archive",
			err)
	}
	defer func() {
		// A cleanup that fails is reported but never turns the step's own outcome
		// into a different one: the fonts are installed either way.
		if err := os.RemoveAll(scratchDir); err != nil {
			SendLog(stepID, fmt.Sprintf("Warning: could not remove the downloaded font archive: %v", err))
		}
	}()

	archivePath := filepath.Join(scratchDir, mesloArchiveName)

	SendLog(stepID, "Downloading "+fontName+"...")
	result := system.RunWithLogs(fmt.Sprintf("curl -fsSL -o %s %s", archivePath, mesloArchiveURL), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return wrapStepError("font", "Install "+fontName,
			"Failed to download font. Check your internet connection.",
			result.Error)
	}

	SendLog(stepID, "Extracting font archive...")
	result = system.RunWithLogs(fmt.Sprintf("unzip -o %s -d %s/", archivePath, fontDir), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return wrapStepError("font", "Install "+fontName,
			"Failed to extract font archive",
			result.Error)
	}

	SendLog(stepID, "Updating font cache...")
	system.RunWithLogs("fc-cache -fv", nil, func(line string) {
		SendLog(stepID, line)
	})

	// Verify, and report the verification rather than the attempt. A broken
	// extraction, a fontconfig cache that did not pick the directory up, or a
	// user font directory the terminal cannot read all leave the family
	// unresolved, and none of them may be reported as ✓. This is the same
	// discipline wslterminal.go follows against the Windows font store.
	resolved, known := installedLinuxFontFamily(mesloArchiveFamily)
	if !known {
		return wrapStepError("font", "Install "+fontName,
			fmt.Sprintf("The font files were extracted into %s, but %s could not be asked, so %q was not verified and may not resolve.", fontDir, linuxFontFamilyQuery, mesloArchiveFamily),
			nil)
	}
	if !resolved {
		return wrapStepError("font", "Install "+fontName,
			fmt.Sprintf("The font files were extracted into %s, but %s does not resolve %q, so the configured font would fall back silently.", fontDir, linuxFontFamilyQuery, mesloArchiveFamily),
			nil)
	}
	SendLog(stepID, fmt.Sprintf("✓ Font installed - %s resolves %s", linuxFontFamilyQuery, mesloArchiveFamily))
	return nil
}

type platformPackages struct {
	Termux string
	Brew   string
	Arch   string
	Fedora string
	Debian string

	// archUnavailable names the packages the Arch list had to drop because the
	// official repositories do not carry them. It travels beside the filtered
	// list so the caller can report the gap for the package manager that
	// actually runs. The Debian gap is returned separately by the constructors,
	// which predate this field.
	archUnavailable []string

	// fedoraUnavailable names the packages the Fedora list had to drop because
	// the distribution's own repositories do not carry them. It travels beside
	// the filtered list so the caller can report the gap for dnf, the manager
	// that actually runs on a Fedora host. It exists for the same reason as
	// archUnavailable beside the constructors' separate Debian return.
	fedoraUnavailable []string
}

var (
	runPkgInstallWithLogs = system.RunPkgInstall
	runSudoWithLogs       = sudoWithLogs
	runSudoSilently       = sudoSilently
	runBrewWithLogs       = system.RunBrewWithLogs
	runOhMyZshInstaller   = system.RunWithLogs
	runFnmWithLogs        = system.RunWithLogs
)

// nonInteractiveSudo spells the sudo invocation every root command uses. The -n
// flag makes sudo fail immediately when the credential cached for the run is
// absent, instead of prompting on a terminal the captured executor does not
// own, so a step that needs a password it does not have reports the failure
// where the user can read it.
func nonInteractiveSudo(command string) string {
	return "sudo -n " + command
}

// sudoWithLogs runs a root command and streams its output, with -n so it can
// never prompt. It is the default behind the runSudoWithLogs seam.
func sudoWithLogs(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
	return system.RunWithLogs(nonInteractiveSudo(command), opts, onLog)
}

// sudoSilently runs a root command without streaming, with -n for the same
// reason as sudoWithLogs. It is the default behind the runSudoSilently seam.
func sudoSilently(command string, opts *system.ExecOptions) *system.ExecResult {
	return system.Run(nonInteractiveSudo(command), opts)
}

// fnmPath resolves the fnm executable. It is a variable so a test can exercise
// the alias setup without a real fnm installation.
var fnmPath = fnmBinaryPath

// debianUnavailable names the packages this installer requests on other
// platforms but that Debian and Ubuntu do not carry in their own repositories.
//
// apt aborts the whole transaction when a single requested name is unknown, so
// a name listed in a Debian package set but missing from the distribution takes
// every package beside it down as well. The names below were verified against
// ubuntu:22.04, the image the installer's own E2E target runs, with
// `apt-cache policy`; a name that was not there is left out of every Debian
// list and reported to the user instead. apt is only reached when Homebrew is
// absent, so the Homebrew path already covers these tools where it works.
//
// The omission is version-specific: kubectx and tree-sitter-cli appear in
// Ubuntu 24.04 (and tree-sitter-cli again in 25.04) but not in 24.10, and
// starship only appears in 25.04. The list has to hold what the oldest
// supported release carries, because apt fails as a unit.
//
// wslu is listed for the same reason even though Ubuntu carries it: Debian 12,
// the release this was reported on, does not, and the installer cannot tell the
// two apart at the package-manager level. apt aborts the whole transaction on
// the unknown name, which used to take the base dependencies down with it, so
// wslu is dropped and reported instead.
var debianUnavailable = map[string]bool{
	"starship":        true,
	"kubectx":         true,
	"nushell":         true,
	"zellij":          true,
	"lazygit":         true,
	"tree-sitter-cli": true,
	"wslu":            true,
}

// debianPackages joins wanted into a package list apt can install, dropping the
// names Debian/Ubuntu does not carry. It returns the dropped names separately so
// the caller can tell the user where to get them instead.
func debianPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if debianUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logDebianUnavailable tells the user which requested tools the distribution
// does not provide, so a skipped tool is never mistaken for an installed one.
func logDebianUnavailable(stepID string, unavailable []string) {
	for _, line := range debianUnavailableMessage(unavailable) {
		SendLog(stepID, line)
	}
}

// debianUnavailableMessage returns the lines that name the requested packages
// the Debian/Ubuntu repositories do not carry and where to get them. The
// executed step logs them and the interactive script echoes them, so both paths
// give the same account of a package the filter dropped.
func debianUnavailableMessage(unavailable []string) []string {
	if len(unavailable) == 0 {
		return nil
	}
	return []string{
		fmt.Sprintf("Not available in the Debian/Ubuntu repositories, so not installed: %s.",
			strings.Join(unavailable, ", ")),
		"Install them with Homebrew (brew install) or from each tool's own upstream installer.",
	}
}

// archUnavailable names the packages this installer requests on other platforms
// but that Arch does not carry in its official repositories.
//
// pacman aborts the whole transaction when a single requested name is unknown,
// exactly as apt does, so a name listed in an Arch package set but missing from
// the distribution takes every package beside it down as well. Every name the
// Arch columns request was verified against archlinux:latest with `pacman -Sy`
// followed by `pacman -Si`; the names below were the only ones not found. A
// name that was not found is left out of every Arch list and reported to the
// user instead. pacman is reached whenever the host is Arch, with or without
// Homebrew, so the Homebrew fallback does not cover these tools by itself.
//
// Both names are companion packages the configuration reads at shell start, not
// components the user selects: carapace provides the completions fish, zsh and
// nushell source, and zsh-theme-powerlevel10k is the prompt theme .zshrc
// sources. Both are available from Homebrew as carapace and powerlevel10k and
// from the AUR, so the report names a route that exists.
var archUnavailable = map[string]bool{
	"carapace":                true,
	"zsh-theme-powerlevel10k": true,
}

// archPackages joins wanted into a package list pacman can install, dropping
// the names Arch does not carry. It returns the dropped names separately so the
// caller can tell the user where to get them instead.
func archPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if archUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logArchUnavailable tells the user which requested tools the Arch repositories
// do not provide, so a skipped tool is never mistaken for an installed one.
func logArchUnavailable(stepID string, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	SendLog(stepID, fmt.Sprintf(
		"Not available in the Arch repositories, so not installed: %s.",
		strings.Join(unavailable, ", ")))
	SendLog(stepID, "Install them with Homebrew (brew install) or from each tool's own upstream installer.")
}

// logArchStillMissing reports the Arch-filtered packages that are still absent
// once an install attempt has finished. The notice has to describe the gap that
// remains, not the names the filter dropped before anything ran: a component the
// available route installed, or one the host already had, is not missing and
// must not be named. The filtered list is only consulted when the pacman route
// is the one that ran, so an Arch host whose Homebrew installed the components
// is never told they are unavailable.
func logArchStillMissing(stepID string, result *system.ExecResult, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	missing := make([]string, 0, len(unavailable))
	for _, name := range unavailable {
		if !componentPresentAfterInstall(result, name) {
			missing = append(missing, name)
		}
	}
	logArchUnavailable(stepID, missing)
}

// fedoraUnavailable names the packages this installer requests on other
// platforms but that Fedora does not carry in its own repositories.
//
// dnf aborts the whole transaction when a single requested name is unknown,
// exactly as apt and pacman do, so a name listed in a Fedora package set but
// missing from the distribution takes every package beside it down as well and
// leaves the shell or window manager that did exist uninstalled. Every name the
// Fedora columns request was checked against fedora:latest, the image the E2E
// target builds from, with `dnf install --assumeno`. That check resolves
// virtual provides and groups the same way the real install command does, so
// wget (provided by wget2-wget) and npm (provided by nodejs-npm) count as
// present instead of being dropped by a bare name comparison. The names below
// are the only ones that did not resolve; a name that did not resolve is left
// out of every Fedora list and reported to the user instead.
//
// The check is version-specific, and this note is the Fedora counterpart of the
// Debian one: nushell is absent from Fedora 40 but present in Fedora 44, so it
// is deliberately not listed here. Holding the names the target image actually
// reaches keeps a release that does not carry one failing dnf loudly, exactly
// as it would for apt or pacman.
//
// zellij is a component the user can select, and it is the one name below the
// step must fail on. carapace (the completions fish, zsh and nushell source),
// starship (the prompt those shells start) and lazygit (the Neovim git UI) are
// companions the configuration reads when present, so they are a logged notice.
// All four are available from Homebrew and from each tool's upstream installer.
var fedoraUnavailable = map[string]bool{
	"carapace": true,
	"starship": true,
	"zellij":   true,
	"lazygit":  true,
}

// fedoraPackages joins wanted into a package list dnf can install, dropping the
// names Fedora does not carry. It returns the dropped names separately so the
// caller can tell the user where to get them instead.
func fedoraPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if fedoraUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logFedoraUnavailable tells the user which requested tools the Fedora
// repositories do not provide, so a skipped tool is never mistaken for an
// installed one.
func logFedoraUnavailable(stepID string, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	SendLog(stepID, fmt.Sprintf(
		"Not available in the Fedora repositories, so not installed: %s.",
		strings.Join(unavailable, ", ")))
	SendLog(stepID, "Install them with Homebrew (brew install) or from each tool's own upstream installer.")
}

// logFedoraStillMissing reports the Fedora-filtered packages that are still
// absent once an install attempt has finished. The notice has to describe the
// gap that remains, not the names the filter dropped before anything ran: a
// component the available route installed, or one the host already had, is not
// missing and must not be named. The filtered list is only consulted when the
// dnf route is the one that ran, so a Fedora host whose Homebrew installed the
// components is never told they are unavailable.
func logFedoraStillMissing(stepID string, result *system.ExecResult, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	missing := make([]string, 0, len(unavailable))
	for _, name := range unavailable {
		if !componentPresentAfterInstall(result, name) {
			missing = append(missing, name)
		}
	}
	logFedoraUnavailable(stepID, missing)
}

// ohMyZshInstallerURL is the official Oh My Zsh installer, pinned to the exact
// revision this repository ships and the local machine runs. An unpinned master
// URL would execute whatever upstream publishes next, which the previous
// repository-committed snapshot never did.
const (
	ohMyZshInstallerURL = "https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/0ee67f042872d1dfab74270c31867771ca35aef4/tools/install.sh"
	ohMyZshInstallerRef = "0ee67f042872d1dfab74270c31867771ca35aef4"
)

// ohMyZshEntrypoint is the file .zshrc sources. Its presence is what tells a
// complete installation apart from a directory an interrupted run left behind.
const ohMyZshEntrypoint = "oh-my-zsh.sh"

// shouldInstallOhMyZsh reports whether the installation is missing or incomplete.
//
// Checking the directory alone was not enough: the installer creates it early,
// so a download or clone that fails afterwards leaves a directory that every
// later run would report as installed and would never repair. PathExists follows
// symlinks on purpose, so a symlinked ~/.oh-my-zsh counts as present.
func shouldInstallOhMyZsh(dir string) bool {
	return !system.PathExists(filepath.Join(dir, ohMyZshEntrypoint))
}

// installOhMyZsh downloads the official installer and runs it against dir.
//
// It is deliberately two commands instead of the usual
// `sh -c "$(curl -fsSL ...)"`. That idiom needs an outer shell to expand the
// substitution; Termux does not use one because Go's fork/exec through a shell
// misbehaves on Android, so the substitution reached `sh -c` literally, which
// then tried to execute the first word of the downloaded script as a command:
//
//	sh: #!/bin/sh: not found
//
// Two plain commands also give a clear error for the download and for the run.
func installOhMyZsh(dir, stepID string) error {
	installer, err := os.CreateTemp("", "oh-my-zsh-install-*.sh")
	if err != nil {
		return fmt.Errorf("could not create a temporary file for the installer: %w", err)
	}
	installerPath := installer.Name()
	if err := installer.Close(); err != nil {
		return err
	}
	defer func() { _ = os.Remove(installerPath) }()

	logLine := func(line string) { SendLog(stepID, line) }

	if result := runOhMyZshInstaller(
		fmt.Sprintf("curl -fsSL -o %q %q", installerPath, ohMyZshInstallerURL), nil, logLine); result.Error != nil {
		return fmt.Errorf("could not download the Oh My Zsh installer: %w", result.Error)
	}

	// RUNZSH keeps the installer from starting a shell, CHSH from changing the
	// login shell and KEEP_ZSHRC from overwriting the .zshrc copied above.
	if result := runOhMyZshInstaller(
		fmt.Sprintf("env ZSH=%q RUNZSH=no CHSH=no KEEP_ZSHRC=yes sh %q", dir, installerPath), nil, logLine); result.Error != nil {
		removePartialOhMyZsh(dir)
		return fmt.Errorf("could not run the Oh My Zsh installer: %w", result.Error)
	}

	if !system.PathExists(filepath.Join(dir, ohMyZshEntrypoint)) {
		removePartialOhMyZsh(dir)
		return fmt.Errorf("the installer completed but %s does not exist", filepath.Join(dir, ohMyZshEntrypoint))
	}
	return nil
}

// removePartialOhMyZsh deletes a directory an interrupted install left behind.
// Without it the guard would read that directory as a finished installation and
// no later run could ever repair the shell.
func removePartialOhMyZsh(dir string) {
	if system.PathExists(dir) {
		_ = os.RemoveAll(dir)
	}
}

// platformInstallPlan is the single decision of which package manager a package
// set installs through and with which names. installPlatformPackages executes it
// and getDepsScript renders it as a shell script, so the executing and
// interactive paths read the same value and cannot disagree about the manager or
// the packages.
type platformInstallPlan struct {
	// Manager is "pkg", "pacman", "dnf", "apt-get" or "brew"; empty when no
	// package manager is available for the host.
	Manager string
	// Update is an index refresh run before Packages; empty when the manager does
	// not need one. It is part of the plan so the executed step and the
	// interactive script cannot disagree about whether it runs.
	Update string
	// Packages is the list handed to that manager.
	Packages string
	// HomebrewPackages is the Homebrew list tried when a native command fails and
	// Homebrew is present; empty when there is no fallback.
	HomebrewPackages string
}

// planPlatformInstall decides the package manager and the command for a package
// set. It is deliberately pure: a caller either executes the plan or renders it,
// so the decision itself has exactly one implementation.
func planPlatformInstall(m *Model, packages platformPackages) platformInstallPlan {
	switch {
	case m.SystemInfo.IsTermux:
		return platformInstallPlan{Manager: "pkg", Packages: packages.Termux}
	case usesPacman(m) && packages.Arch != "":
		return platformInstallPlan{Manager: "pacman", Packages: packages.Arch, HomebrewPackages: packages.Brew}
	// dnf aborts the whole transaction on a single unknown name, exactly as apt
	// and pacman do, so the Fedora lists are filtered by fedoraPackages before
	// they reach this command and the names Fedora does not carry never arrive.
	//
	// dnf's own answer to that abort, --skip-unavailable, is deliberately not
	// used. With the filter in place it no longer buys the install its coverage,
	// and keeping it would let a name the filter does not know about be dropped
	// without a word, which is the defect this change removes. A name that is
	// genuinely absent now fails the command loudly instead of disappearing.
	//
	// dnf runs only when Homebrew is absent. With Homebrew present the switch
	// falls through to the default branch and the unfiltered Brew list installs
	// everything, so a component the Fedora repositories do not carry is still
	// installed instead of being left to a step that would fail.
	case usesDnf(m) && packages.Fedora != "":
		return platformInstallPlan{Manager: "dnf", Packages: packages.Fedora, HomebrewPackages: packages.Brew}
	case usesApt(m) && packages.Debian != "":
		// apt is the only manager here that needs its index refreshed first; the
		// non-interactive step never did one for pacman or dnf, so the shared plan
		// gives them none and the TUI stops doing a full `pacman -Syu` the other
		// path never ran.
		return platformInstallPlan{Manager: "apt-get", Update: "apt-get update", Packages: packages.Debian, HomebrewPackages: packages.Brew}
	case m.SystemInfo.HasBrew && packages.Brew != "":
		return platformInstallPlan{Manager: "brew", Packages: packages.Brew}
	}
	return platformInstallPlan{}
}

// nativeCommand returns the command the native managers run through sudo.
func (p platformInstallPlan) nativeCommand() string {
	switch p.Manager {
	case "pacman":
		return "pacman -S --needed --noconfirm " + p.Packages
	case "dnf":
		return "dnf install -y " + p.Packages
	case "apt-get":
		return "apt-get install -y " + p.Packages
	}
	return ""
}

// sudoCommand returns the full command line a rendered script runs through sudo
// for a native manager.
func (p platformInstallPlan) sudoCommand() string {
	if command := p.nativeCommand(); command != "" {
		return nonInteractiveSudo(command)
	}
	return ""
}

func installPlatformPackages(m *Model, stepID string, packages platformPackages, onLog func(string)) *system.ExecResult {
	return runPlatformInstall(m, planPlatformInstall(m, packages), onLog)
}

// runPlatformInstall executes a plan through the same runners the dependency and
// component steps have always used. The host queries the native branches depend
// on are made here from the plan, so a rendered script cannot drift from what
// would actually run.
func runPlatformInstall(m *Model, plan platformInstallPlan, onLog func(string)) *system.ExecResult {
	switch plan.Manager {
	case "pkg":
		return runPkgInstallWithLogs(plan.Packages, nil, onLog)
	case "pacman", "dnf", "apt-get":
		return runNativeWithBrewFallback(plan.nativeCommand(), plan.HomebrewPackages, m.SystemInfo.HasBrew, onLog)
	case "brew":
		return runBrewWithLogs("install "+plan.Packages, nil, onLog)
	default:
		return &system.ExecResult{
			Error: fmt.Errorf("no package manager available for this platform"),
		}
	}
}

func runNativeWithBrewFallback(nativeCommand string, brewPackages string, hasBrew bool, onLog func(string)) *system.ExecResult {
	result := runSudoWithLogs(nativeCommand, nil, onLog)
	if result.Error == nil || !hasBrew || brewPackages == "" {
		return result
	}

	return runBrewWithLogs("install "+brewPackages, nil, onLog)
}

// Herdr release used by the fallback download. Keep the repository and the tag
// in sync with the asset checksums below. The tag matters beyond the download:
// it is the version a machine gets when Herdr is not already installed, so it
// has to be one that can update its plugins (0.9.4 is the first, and v0.9.1, the
// previous pin, could not). v0.9.5 is the current stable release, whose linux
// x86_64 and aarch64 asset digests are the two values below.
const (
	herdrRepo    = "herdrdev/herdr"
	herdrVersion = "v0.9.5"
)

func installHerdrBinary(m *Model, stepID string) error {
	if system.CommandExists("herdr") {
		SendLog(stepID, "Herdr already installed")
		return nil
	}
	if m.SystemInfo.IsTermux {
		return fmt.Errorf("herdr is not available through the Termux package installer")
	}
	if m.SystemInfo.OS == system.OSMac || m.SystemInfo.HasBrew {
		result := system.RunBrewWithLogs("install herdr", nil, func(line string) {
			SendLog(stepID, line)
		})
		return result.Error
	}

	assetArch := ""
	expectedSHA256 := ""
	switch runtime.GOARCH {
	case "amd64":
		assetArch = "x86_64"
		expectedSHA256 = "14d9e8bc8b4a9e0118d77c32266f76d461f09b40342eb23752960bd89f59929d"
	case "arm64":
		assetArch = "aarch64"
		expectedSHA256 = "17d7536f2da50859523680ef279dffcac8e4a235f4e46fe8da6c2ec44141897c"
	default:
		return fmt.Errorf("unsupported Herdr architecture: %s", runtime.GOARCH)
	}

	homeDir := os.Getenv("HOME")
	binDir := filepath.Join(homeDir, ".local", "bin")
	if err := system.EnsureDir(binDir); err != nil {
		return err
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/herdr-linux-%s", herdrRepo, herdrVersion, assetArch)
	dest := filepath.Join(binDir, "herdr")
	SendLog(stepID, "Downloading Herdr release binary...")
	result := system.RunWithLogs(fmt.Sprintf("curl -fsSL %q -o %q", url, dest), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return result.Error
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	actualSHA256 := sha256.Sum256(data)
	if hex.EncodeToString(actualSHA256[:]) != expectedSHA256 {
		os.Remove(dest)
		return fmt.Errorf("Herdr checksum mismatch for %s", url)
	}

	return os.Chmod(dest, 0755)
}

// ============================================================================
// HERDR PLUGINS: THE DECLARED SET, INSTALLED, UPDATED AND VERIFIED
// ============================================================================
//
// Unlike the plugin set for Tmux, Fish, Zsh and Neovim, Herdr's is not declared
// in a file this repository copies: Herdr installs plugins itself, from GitHub
// shorthand, into a directory it owns. The set is still a decision this
// repository makes, so it is named here and in the documentation, and nowhere
// is it inferred.
//
// The set is installed on every run, and updated on every run when the machine's
// herdr can: 0.9.4 is the first release whose plugin surface has an `update`
// subcommand, and `herdr plugin update --yes` is what this step runs after the
// install. The install is deliberately not pinned either -- `herdr plugin
// install` re-resolves the repository's default branch and replaces the managed
// checkout it owns -- so a re-run both reinstalls the set and, where it can,
// updates it. The revision that actually landed is Herdr's fact, not this
// installer's: it is read back from `herdr plugin list`
// (`source.resolved_commit`) after the update and reported, never chosen in
// advance.
//
// The update is capability-guarded rather than attempted-and-caught, because
// `herdr plugin update` exits non-zero on a CLI that has no such subcommand:
// running it unconditionally would fail this step on exactly the machines that
// have been running this installer longest (0.9.1 and 0.9.3). On those the step
// names the version it found and says the plugins were installed but cannot be
// updated until Herdr is upgraded. A step that fails on a machine which is
// merely old is a worse defect than one that does not update.
//
// Nothing else is touched. The step never uninstalls, unlinks or disables a
// plugin, so a plugin the user added themselves survives a reinstall, and a
// plugin that fails to install or fails to build is never reported as installed.

// herdrPlugin is one plugin this installer declares and provisions.
type herdrPlugin struct {
	// ID is the plugin id Herdr records and `herdr plugin list` reports. It is
	// what verification looks for.
	ID string
	// Owner and Repo are the GitHub shorthand `herdr plugin install` accepts.
	Owner string
	Repo  string
	// Name is the plugin's own name, for the install log.
	Name string
}

func (p herdrPlugin) shorthand() string { return p.Owner + "/" + p.Repo }

// herdrPluginSet is the declared set, and the single source of truth for it: the
// step installs exactly these, and the documentation guard fails if a declared
// plugin stops being named in docs/tools.md.
var herdrPluginSet = []herdrPlugin{
	{ID: "aigora.lantern", Owner: "aigorahub", Repo: "herdr-lantern", Name: "Lantern, by Elves"},
	{ID: "herdr.auto-title", Owner: "kryptamine", Repo: "herdr-auto-title", Name: "Auto Title"},
}

// herdrPluginRun runs one Herdr plugin CLI command and returns its result. It is
// a variable so a test replaces the boundary; production uses system.RunWithLogs
// so the plugin's own install and build output reaches the step log.
var herdrPluginRun = system.RunWithLogs

// herdrPluginList reads the installed-plugin registry back from the machine. It
// is a variable for the same reason as herdrPluginRun, and it is separate
// because verification must read the machine's answer rather than interleave it
// with log lines.
var herdrPluginList = func() (string, error) {
	result := system.Run("herdr plugin list --json", nil)
	if result.Error != nil {
		return "", result.Error
	}
	return result.Output, nil
}

// herdrPluginUpdateMinVersion is the first Herdr release with a plugin update
// command. 0.9.4's own `--help` says `update   Update GitHub-installed plugins`;
// 0.9.3 lists eleven subcommands and none of them updates, so the version is the
// capability boundary the step has to honour before it runs anything.
const herdrPluginUpdateMinVersion = "0.9.4"

// herdrPluginUpdateCommand is the noninteractive form of the update. Read from
// 0.9.4's own `--help`:
//
//	Usage: herdr plugin update [OPTIONS] [PLUGIN]...
//	Arguments:
//	  [PLUGIN]...
//	Options:
//	  -y, --yes
//
// The plugin arguments are optional and omitted here: with none, the command
// updates every GitHub-installed plugin. --yes is required rather than optional
// for this step: with stdin not a terminal, `herdr plugin update` exits 2 with
// "plugin update requires --yes when stdin is not interactive", and a TUI step
// cannot answer a prompt.
const herdrPluginUpdateCommand = "herdr plugin update --yes"

// herdrPluginVersion reads the version of the herdr binary on PATH. It is a
// variable for the same reason as herdrPluginList, and it is separate because
// the version decides whether the update command may run at all.
var herdrPluginVersion = func() (string, error) {
	result := system.Run("herdr --version", nil)
	if result.Error != nil {
		return "", result.Error
	}
	return result.Output, nil
}

// parseHerdrVersion pulls the version out of `herdr --version`, which prints a
// single line like "herdr 0.9.3". It returns "" when no version is present, and
// the caller treats that as "cannot update" rather than guessing.
func parseHerdrVersion(output string) string {
	for _, field := range strings.Fields(output) {
		field = strings.TrimPrefix(field, "v")
		if field == "" || field[0] < '0' || field[0] > '9' {
			continue
		}
		end := 0
		for end < len(field) && (field[end] >= '0' && field[end] <= '9' || field[end] == '.') {
			end++
		}
		if version := strings.Trim(field[:end], "."); version != "" {
			return version
		}
	}
	return ""
}

// herdrVersionAtLeast reports whether found is at least want, comparing dotted
// numeric components. A string comparison would order 0.10.0 below 0.9.4 and
// silently stop updating a newer Herdr; a missing component counts as zero, so
// "0.9" is below "0.9.4" and "1.0" is at least "0.9.4".
func herdrVersionAtLeast(found, want string) bool {
	foundParts, ok := herdrVersionComponents(found)
	if !ok {
		return false
	}
	wantParts, ok := herdrVersionComponents(want)
	if !ok {
		return false
	}
	for i := 0; i < len(wantParts); i++ {
		got := 0
		if i < len(foundParts) {
			got = foundParts[i]
		}
		if got != wantParts[i] {
			return got > wantParts[i]
		}
	}
	return true
}

// herdrVersionComponents turns "0.9.4" into [0 9 4]. A suffix such as the
// "-dev" in "0.9.4-dev" ends the numeric run rather than failing the parse, so a
// development build of a release that can update is not mistaken for one that
// cannot. A component with no digits at all makes the whole version unreadable.
func herdrVersionComponents(version string) ([]int, bool) {
	parts := strings.Split(version, ".")
	components := make([]int, 0, len(parts))
	for _, part := range parts {
		value := 0
		digits := 0
		for i := 0; i < len(part) && part[i] >= '0' && part[i] <= '9'; i++ {
			value = value*10 + int(part[i]-'0')
			digits++
		}
		if digits == 0 {
			return nil, false
		}
		components = append(components, value)
	}
	return components, true
}

// installedHerdrPlugin is one plugin `herdr plugin list --json` reports.
type installedHerdrPlugin struct {
	PluginID string `json:"plugin_id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Source   struct {
		ResolvedCommit string `json:"resolved_commit"`
	} `json:"source"`
}

// herdrPluginListDocument mirrors the response envelope of `herdr plugin list
// --json`: {"id":"cli:plugin","result":{"type":"plugin_list","plugins":[...]}}.
type herdrPluginListDocument struct {
	Result struct {
		Plugins []installedHerdrPlugin `json:"plugins"`
	} `json:"result"`
}

// parseInstalledHerdrPlugins reads the machine's answer into a map keyed by
// plugin id. A response that cannot be read is an error, never an empty list:
// "no plugins" and "no answer" must not look the same to a caller that is about
// to claim an install succeeded.
func parseInstalledHerdrPlugins(output string) (map[string]installedHerdrPlugin, error) {
	var doc herdrPluginListDocument
	if err := json.Unmarshal([]byte(output), &doc); err != nil {
		return nil, fmt.Errorf("could not read the `herdr plugin list --json` response: %w", err)
	}
	installed := make(map[string]installedHerdrPlugin, len(doc.Result.Plugins))
	for _, plugin := range doc.Result.Plugins {
		installed[plugin.PluginID] = plugin
	}
	return installed, nil
}

// herdrPluginRevision keeps the reported revision readable while still
// identifying it. Herdr records the full commit; the installer never chooses it.
func herdrPluginRevision(commit string) string {
	if commit == "" {
		return "unknown revision"
	}
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// installHerdrPlugins installs every plugin in the declared set, updates them
// when the herdr on PATH can, and verifies each one against the machine. A
// plugin that did not install, or whose build failed, is returned as an error so
// the step cannot report success over it.
//
// It runs only for the Herdr multiplexer choice, and only where the herdr binary
// resolves: a machine without it gets nothing and no error, because there is
// nothing to install into.
func installHerdrPlugins(m *Model, stepID string) error {
	if m == nil || m.Choices.WindowMgr != "herdr" {
		return nil
	}
	// A set the user turned off is not a failure and not a silent no-op: nothing is
	// installed and nothing is updated, and the log says which set was left out, the
	// way every other thing a run leaves out is reported.
	if pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDHerdr) {
		herdrLog(stepID, "The Herdr plugin set is turned off; nothing was installed or updated")
		return nil
	}
	if !herdrBinaryPresent("herdr") {
		herdrLog(stepID, "Herdr is not on PATH; the Herdr plugins were not touched")
		return nil
	}
	if len(herdrPluginSet) == 0 {
		return fmt.Errorf("the declared Herdr plugin set is empty, so this step proves nothing")
	}

	var failures []error
	for _, plugin := range herdrPluginSet {
		herdrLog(stepID, fmt.Sprintf("Installing Herdr plugin %s (%s)...", plugin.Name, plugin.shorthand()))
		// --yes is the noninteractive form: without it the CLI shows a trust
		// preview and waits on a [y/N] prompt, which a TUI step cannot answer.
		result := herdrPluginRun("herdr plugin install "+plugin.shorthand()+" --yes", nil, func(line string) {
			herdrLog(stepID, line)
		})
		if result.Error != nil {
			failures = append(failures, fmt.Errorf("%s (%s) failed to install: %w", plugin.Name, plugin.ID, result.Error))
		}
	}

	// The update runs before the read-back so the revision reported below is the
	// one the update left behind, not the one the install landed a moment
	// earlier. Its failure is a failure: on a CLI that has the command, an update
	// that did not happen is not a successful update. On a CLI that does not have
	// it, updateHerdrPlugins answers without running anything, so a machine that
	// is merely old never fails here.
	if err := updateHerdrPlugins(stepID); err != nil {
		failures = append(failures, err)
	}

	// Ask the machine rather than assume, the way the WSL font path asks the
	// Windows font store: the install command's exit status is not proof, so the
	// registry is read back and every declared plugin has to be in it. This is
	// also what makes a failed build visible, because herdr.auto-title is built
	// at install time.
	output, err := herdrPluginList()
	if err != nil {
		return fmt.Errorf("the installed Herdr plugins could not be read back to verify the install: %w", err)
	}
	installed, err := parseInstalledHerdrPlugins(output)
	if err != nil {
		return err
	}
	for _, plugin := range herdrPluginSet {
		got, ok := installed[plugin.ID]
		if !ok {
			failures = append(failures, fmt.Errorf("%s (%s) is not in `herdr plugin list` after installing it", plugin.Name, plugin.ID))
			continue
		}
		herdrLog(stepID, fmt.Sprintf("✓ %s (%s) installed at %s", plugin.Name, plugin.ID, herdrPluginRevision(got.Source.ResolvedCommit)))
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

// updateHerdrPlugins updates the installed GitHub plugins when the herdr on
// PATH can, and explains the gap when it cannot.
//
// The capability is decided from the version before the command runs, and that
// order is the whole point: `herdr plugin update` exits non-zero on a CLI that
// has no such subcommand, so running it unconditionally would fail the step on
// exactly the machines that have been running this installer longest. Herdr
// 0.9.4 is where the command first appears; 0.9.1 and 0.9.3 install plugins but
// cannot update them, and for those the step says so, installs as it always did,
// and succeeds. An upgrade plus a re-run is what updates them there.
//
// A version that cannot be read is treated as "cannot update": the step cannot
// prove the command exists, so it does not run it, and it says that too.
func updateHerdrPlugins(stepID string) error {
	version, err := herdrPluginVersion()
	if err != nil {
		herdrLog(stepID, fmt.Sprintf("Herdr's version could not be read (%v), so the plugins were installed but not updated", err))
		return nil
	}
	found := parseHerdrVersion(version)
	if found == "" {
		herdrLog(stepID, fmt.Sprintf("Herdr did not report a version (output %q), so the plugins were installed but not updated", strings.TrimSpace(version)))
		return nil
	}
	herdrLog(stepID, fmt.Sprintf("Herdr %s found", found))
	if !herdrVersionAtLeast(found, herdrPluginUpdateMinVersion) {
		herdrLog(stepID, fmt.Sprintf("Herdr %s is older than %s, which is where `herdr plugin update` was added, so the plugins were installed but not updated; upgrade Herdr and run the installer again to update them", found, herdrPluginUpdateMinVersion))
		return nil
	}

	result := herdrPluginRun(herdrPluginUpdateCommand, nil, func(line string) {
		herdrLog(stepID, line)
	})
	if result.Error != nil {
		return fmt.Errorf("the Herdr plugins were installed but could not be updated with `%s`: %w", herdrPluginUpdateCommand, result.Error)
	}
	herdrLog(stepID, "✓ Herdr plugins updated")
	return nil
}

// shellPlatformPackages returns the packages each package manager needs for the
// requested shell and the tools its configuration reads at start.
//
// The Debian entry is built with debianPackages, the Arch entry with
// archPackages and the Fedora entry with fedoraPackages, so a name the
// distribution does not carry never reaches apt, pacman or dnf and cannot abort
// the whole transaction. The second return value names the omitted Debian
// packages, and the archUnavailable and fedoraUnavailable fields name the
// omitted Arch and Fedora ones, so the caller can report them for the manager
// that actually runs.
func shellPlatformPackages(shell string) (platformPackages, []string) {
	switch shell {
	case "fish":
		debian, unavailable := debianPackages("fish", "zoxide", "starship")
		arch, archGaps := archPackages("fish", "carapace", "zoxide", "atuin", "starship")
		fedora, fedoraGaps := fedoraPackages("fish", "carapace", "zoxide", "atuin", "starship")
		return platformPackages{
			Termux:            "fish starship zoxide",
			Brew:              "fish carapace zoxide atuin starship",
			Arch:              arch,
			Fedora:            fedora,
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	case "zsh":
		// The zsh configuration depends on these at shell start: eza/bat/rg/fd/fzf
		// drive the aliases and the fzf integration, delta drives .gitconfig,
		// fnm owns Node, direnv hooks directory environments, and jq/gh/xh/trip
		// back the documented helper aliases. kubectx is only in the Homebrew and
		// Arch lists: the Debian/Ubuntu repositories do not carry it, and apt
		// cannot skip it the way dnf can. zsh-completions and fzf-tab are
		// Homebrew-only names, and fnm, eza, delta and xh are not in the Debian
		// repositories either. btop backs the `top` alias. It is absent from the
		// Termux list because termux-main does not package it and the Termux route
		// passes every name to pkg install with no availability filter, so one
		// unknown name would abort the whole transaction.
		debian, unavailable := debianPackages(
			"zsh", "zoxide", "zsh-autosuggestions", "zsh-syntax-highlighting",
			"kubectx", "direnv", "jq", "gh", "bat", "fd-find", "ripgrep", "fzf",
			"btop",
		)
		arch, archGaps := archPackages(
			"zsh", "carapace", "zoxide", "atuin", "zsh-autosuggestions",
			"zsh-syntax-highlighting", "zsh-autocomplete",
			"zsh-theme-powerlevel10k", "kubectx", "eza", "bat", "fd",
			"ripgrep", "fzf", "direnv", "jq", "github-cli", "git-delta", "btop",
		)
		fedora, fedoraGaps := fedoraPackages(
			"zsh", "carapace", "zoxide", "atuin", "zsh-autosuggestions",
			"zsh-syntax-highlighting", "starship", "eza", "bat", "fd-find",
			"ripgrep", "fzf", "direnv", "jq", "gh", "git-delta", "btop",
		)
		return platformPackages{
			Termux: "zsh starship zoxide",
			Brew:   "zsh carapace zoxide atuin zsh-autosuggestions zsh-syntax-highlighting zsh-completions fzf-tab zsh-autocomplete powerlevel10k kubectx eza bat fd ripgrep fzf fnm direnv jq gh git-delta xh trippy btop",
			Arch:   arch,
			Fedora: fedora,
			// Debian stable does not package starship, fnm, eza, delta or xh; those
			// come from Homebrew, which this installer puts in place for Debian hosts.
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	case "nushell":
		debian, unavailable := debianPackages("nushell", "zoxide", "jq", "bash", "starship")
		arch, archGaps := archPackages("nushell", "carapace", "zoxide", "atuin", "jq", "bash", "starship")
		fedora, fedoraGaps := fedoraPackages("nushell", "carapace", "zoxide", "atuin", "jq", "bash", "starship")
		return platformPackages{
			Termux:            "nushell starship zoxide jq",
			Brew:              "nushell carapace zoxide atuin jq bash starship",
			Arch:              arch,
			Fedora:            fedora,
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	}
	return platformPackages{}, nil
}

// shellPackageName maps the installer's shell choice to the package that
// provides the shell binary itself, rather than a plugin beside it.
func shellPackageName(shell string) string {
	if shell == "nushell" {
		return "nushell"
	}
	return shell
}

// shellCommandName maps the installer's shell choice to the command a user
// runs, which is what a presence check has to look for. The nushell package
// provides the nu binary.
func shellCommandName(shell string) string {
	if shell == "nushell" {
		return "nu"
	}
	return shell
}

// archMissingSelectedShell is the selected-component rule for the shell, applied
// after the install instead of before it. A shell the Arch filter dropped is
// still missing only when it is absent once pacman and the Homebrew fallback
// have both been attempted; pacman can report success while the filtered shell
// was never in its command, so the install result alone does not settle it and
// the shell itself has to be looked for. A nil return means the step may
// continue.
func archMissingSelectedShell(m *Model, stepID, shell string, result *system.ExecResult) error {
	if !usesPacman(m) || !archUnavailable[shellPackageName(shell)] {
		return nil
	}
	if componentPresentAfterInstall(result, shellCommandName(shell)) {
		return nil
	}
	return wrapStepError(stepID, "Install Shell",
		fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
			"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
			shell, shellPackageName(shell)),
		nil)
}

// fedoraMissingSelectedShell is the selected-component rule for the shell,
// applied after the install instead of before it. A shell the Fedora filter
// dropped is still missing only when it is absent once dnf and the Homebrew
// fallback have both been attempted; dnf can report success while the filtered
// shell was never in its command, so the install result alone does not settle
// it and the shell itself has to be looked for. A nil return means the step may
// continue.
func fedoraMissingSelectedShell(m *Model, stepID, shell string, result *system.ExecResult) error {
	if !usesDnf(m) || !fedoraUnavailable[shellPackageName(shell)] {
		return nil
	}
	if componentPresentAfterInstall(result, shellCommandName(shell)) {
		return nil
	}
	return wrapStepError(stepID, "Install Shell",
		fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
			"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
			shell, shellPackageName(shell)),
		nil)
}

// fnmAliasesDir returns the directory fnm stores its aliases in. It matches the
// FNM_DIR the shipped .zshrc exports, so the alias the installer creates is the
// one the configuration reads. Keeping the two in one function is what stops
// them from drifting apart.
func fnmAliasesDir(home string) string {
	return filepath.Join(home, ".local", "share", "fnm", "aliases")
}

// fnmBinaryPath returns the fnm executable, looking on PATH first and in the
// Homebrew prefix after. The installer installs fnm through Homebrew in this
// same run, so Homebrew's bin directory is not necessarily on this process's
// PATH yet when the alias is created.
func fnmBinaryPath() string {
	if path, err := exec.LookPath("fnm"); err == nil {
		return path
	}
	candidate := filepath.Join(system.GetBrewPrefix(), "bin", "fnm")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// ensureFnmDefaultAlias makes the `default` alias the shipped .zshrc starts on
// exist once this run installs fnm. The alias is created rather than assumed so
// the configuration is never left in a state it cannot satisfy.
//
// It is best-effort and idempotent: an alias that already exists, a missing fnm
// and a failed download all leave the machine as they were, and the .zshrc is
// written to stay quiet in those cases. The call site only reaches it when fnm
// was not already on the machine, so a user's own fnm setup is never changed.
func ensureFnmDefaultAlias(stepID string) {
	fnm := fnmPath()
	if fnm == "" {
		return
	}

	home := os.Getenv("HOME")
	if system.PathExists(filepath.Join(fnmAliasesDir(home), "default")) {
		SendLog(stepID, "fnm already has a `default` alias")
		return
	}

	fnmDir := filepath.Join(home, ".local", "share", "fnm")
	env := &system.ExecOptions{Env: []string{
		"FNM_DIR=" + fnmDir,
		"FNM_COREPACK_ENABLED=false",
	}}
	onLog := func(line string) { SendLog(stepID, line) }

	// --lts installs the latest LTS and records it under the `lts-latest` alias,
	// which the next command points `default` at. `fnm default` is a shorthand
	// for `fnm alias <version> default`.
	SendLog(stepID, "Installing a Node version through fnm for the `default` alias...")
	if result := runFnmWithLogs(fmt.Sprintf("%q install --lts", fnm), env, onLog); result.Error != nil {
		SendLog(stepID, "Warning: could not install a Node version through fnm, so the `default` alias was not created: "+result.Error.Error())
		return
	}
	if result := runFnmWithLogs(fmt.Sprintf("%q default lts-latest", fnm), env, onLog); result.Error != nil {
		SendLog(stepID, "Warning: could not point the fnm `default` alias at the installed Node: "+result.Error.Error())
		return
	}
	SendLog(stepID, "✓ fnm `default` alias ready")
}

// generateBatThemesFromDefinitions writes a bat .tmTheme for every definition
// that names one into dir, so a theme whose file the checkout does not ship still
// paints bat: bat reads the file from its themes directory and selects it by the
// name inside it, and the definition is the same one the switch reads, so the
// generated file and the selected name cannot disagree. A checkout with no
// themes/ directory - one that predates the theme library - has no definitions to
// generate and returns zero, and the copied files are then all bat can have.
func generateBatThemesFromDefinitions(repoDir, dir string) (int, error) {
	defs, err := loadThemeDefinitions(repoDir)
	if err != nil {
		return 0, nil
	}

	generated := 0
	for _, def := range defs {
		if def.Bat == "" || def.BatFile == "" {
			continue
		}
		// The file name comes from a definition, so it is checked before it is used
		// to build a path: a name that is not a plain file name under dir is refused
		// rather than written.
		if def.BatFile != filepath.Base(def.BatFile) || !strings.HasSuffix(def.BatFile, ".tmTheme") {
			return generated, fmt.Errorf("theme %q names the bat theme file %q, which is not a plain .tmTheme file name", def.ID, def.BatFile)
		}
		content, err := renderBatTheme(def)
		if err != nil {
			return generated, err
		}
		if err := os.WriteFile(filepath.Join(dir, def.BatFile), []byte(content), 0o644); err != nil {
			return generated, err
		}
		generated++
	}
	return generated, nil
}

// generateBtopThemesFromDefinitions writes a btop .theme file for every theme
// that can be rendered into dir, so a theme whose file the checkout does not ship
// still paints btop: btop reads the file from its own themes directory and selects
// it by the name color_theme holds, and the definition is the same one the switch
// reads, so the generated file and the selected name cannot disagree. Only the
// default theme's file is under version control; the rest exist because this runs,
// which is what makes a switch to any offered theme land on a file btop has. A
// checkout with no themes/ directory - one that predates the theme library - has
// no definitions to generate and returns zero.
func generateBtopThemesFromDefinitions(repoDir, dir string) (int, error) {
	defs, err := loadThemeDefinitions(repoDir)
	if err != nil {
		return 0, nil
	}

	generated := 0
	for _, def := range defs {
		file, err := btopThemeFile(def.ID)
		if err != nil {
			return generated, err
		}
		content, err := renderBtopTheme(def)
		if err != nil {
			// A partial theme has no palette to paint btop with, and the row reports
			// that; it is not a failure of the install.
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			return generated, err
		}
		generated++
	}
	return generated, nil
}

func stepInstallShell(m *Model) error {
	homeDir := os.Getenv("HOME")
	shell := m.Choices.Shell
	stepID := "shell"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Shell",
			"Failed to locate the cloned repository",
			err)
	}

	packages, unavailable := shellPlatformPackages(shell)

	// The shell's plugin set is a switch (install_plugin_sets.go). With it off, none of
	// the plugin work runs and no package that exists only to provide a plugin of the
	// set is requested: the configuration this step installs names none of them, so a
	// package installed for it would sit unused and the plugin itself is what the
	// configuration no longer loads.
	withPlugins := !pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDShell)
	if shell == "zsh" && !withPlugins {
		packages, unavailable = withoutZshPluginPackages(packages, unavailable)
		SendLog(stepID, "The Zsh plugin set is turned off; Oh My Zsh and the plugin packages are not installed and the configuration names none of them")
	}

	// Common dependencies
	SendLog(stepID, "Creating required directories...")
	system.EnsureDir(filepath.Join(homeDir, ".config"))
	system.EnsureDir(filepath.Join(homeDir, ".cache/starship"))
	system.EnsureDir(filepath.Join(homeDir, ".cache/carapace"))
	system.EnsureDir(filepath.Join(homeDir, ".local/share/atuin"))

	// On a Debian-like host without Homebrew, apt is the only route and it cannot
	// install the names its repositories do not carry. Say which ones are being
	// skipped so a missing tool is never mistaken for an installed one, and fail
	// outright when the shell the user explicitly selected is the missing one:
	// a companion package can be a logged note, the requested shell cannot.
	if usesApt(m) {
		logDebianUnavailable(stepID, unavailable)
		if debianUnavailable[shellPackageName(shell)] {
			return wrapStepError(stepID, "Install Shell",
				fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
					"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
					shell, shellPackageName(shell)),
				nil)
		}
	}

	// Fedora reaches dnf only when Homebrew is absent, and nushell is absent
	// from Fedora 40 while Fedora 44 carries it, so the filter holds the names
	// the target image actually lacks. Every shell this installer can select is
	// present there, which makes this the same companion-versus-selected rule the
	// Debian and Arch branches apply: the companions are a logged note. The
	// selected-component guard is not applied before the install: dnf can succeed
	// while a shell the filter dropped was never in its command, so the guard is
	// applied after the install below, once the route that ran has finished. The
	// companion notice is logged after that same attempt, so it names only what
	// is still missing rather than what the filter dropped.
	switch shell {
	case "fish":
		SendLog(stepID, "Installing Fish shell and plugins...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesPacman(m) {
			logArchStillMissing(stepID, result, packages.archUnavailable)
		}
		if err := archMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to install Fish shell and dependencies",
				result.Error)
		}
		SendLog(stepID, "Copying Fish configuration...")
		fishConfig := filepath.Join(homeDir, ".config", "fish", "config.fish")
		preservedFish, err := system.ReplaceUserConfig(filepath.Join(repoDir, repoAssetFish, "config.fish"), fishConfig,
			"# dotfiles-managed-config: fish", filepath.Join(homeDir, ".config", "fish", "dotfiles.d"),
			"dotfiles-user-config", ".fish")
		if err != nil {
			return wrapStepError("shell", "Install Fish", "Failed to preserve existing Fish configuration", err)
		}
		if preservedFish != "" {
			SendLog(stepID, fmt.Sprintf("Preserved your existing Fish configuration at %s; dotfiles loads it last from ~/.config/fish/dotfiles.d/.", preservedFish))
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetStarship), filepath.Join(homeDir, ".config/starship.toml")); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to copy starship configuration",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetFish), filepath.Join(homeDir, ".config", "fish")); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to copy Fish configuration",
				err)
		}
		// Patch config.fish based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchFishForWM(filepath.Join(homeDir, ".config/fish/config.fish"), m.Choices.WindowMgr, m.Choices.InstallNvim); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to configure config.fish for window manager",
				err)
		}
		// Remove tmux.fish function if not using tmux
		if m.Choices.WindowMgr != "tmux" {
			os.Remove(filepath.Join(homeDir, ".config/fish/functions/tmux.fish"))
		}
		// Termux: Add fish to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding fish to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "fish") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Fish shell configured")

	case "zsh":
		// fnm is installed by this step's package list. Remember whether it was
		// already on the machine, so the `default` alias the shipped .zshrc reads
		// is only created for the fnm this run installed and the user's own fnm is
		// left untouched.
		fnmWasPresent := fnmPath() != ""
		SendLog(stepID, "Installing Zsh and plugins...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesPacman(m) {
			logArchStillMissing(stepID, result, packages.archUnavailable)
		}
		if err := archMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to install Zsh and plugins",
				result.Error)
		}
		if !fnmWasPresent {
			ensureFnmDefaultAlias(stepID)
		}
		SendLog(stepID, "Copying Zsh configuration...")
		// Git's configuration is installed here rather than in a step of its own
		// because it belongs with the shell: delta and fzf, both installed by this
		// step, are what .gitconfig actually calls, and .gitconfig is a loose home
		// dotfile in the same family as .zshenv below. It is listed in ConfigPaths
		// so the backup step protects the existing file before this overwrites it.
		SendLog(stepID, "Copying Git configuration...")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetGitconfig), filepath.Join(homeDir, ".gitconfig")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .gitconfig",
				err)
		}
		// The personal identity is optional by design: a machine may simply not
		// want one, and .gitconfig's includeIf does nothing when the file is
		// absent. It is also the file most likely to be missing from an older
		// checkout, because the installer clones the repository's default branch
		// while the binary can come from a branch that already ships this file.
		// TestRepoAssetsExist is what guarantees the repository contains it.
		personalSrc := filepath.Join(repoDir, repoAssetGitconfigPersonal)
		if _, err := os.Stat(personalSrc); err != nil {
			SendLog(stepID, "Skipping .gitconfig-personal: not present in this checkout")
		} else if err := system.CopyFile(personalSrc, filepath.Join(homeDir, ".gitconfig-personal")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .gitconfig-personal",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetZshEnv), filepath.Join(homeDir, ".zshenv")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .zshenv configuration",
				err)
		}
		zshrcPath := filepath.Join(homeDir, ".zshrc")
		preservedZsh, err := system.ReplaceUserConfig(filepath.Join(repoDir, repoAssetZshrc), zshrcPath,
			"# dotfiles-managed-config: zsh", filepath.Join(homeDir, ".zshrc.d"), "dotfiles-user-config", ".zsh")
		if err != nil {
			return wrapStepError("shell", "Install Zsh", "Failed to preserve existing .zshrc", err)
		}
		if preservedZsh != "" {
			SendLog(stepID, fmt.Sprintf("Preserved your existing .zshrc at %s; it is sourced from ~/.zshrc.d/.", preservedZsh))
		}
		// Patch .zshrc based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchZshForWM(filepath.Join(homeDir, ".zshrc"), m.Choices.WindowMgr, m.Choices.InstallNvim); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to configure .zshrc for window manager",
				err)
		}
		// A run that left the plugin set out must not leave a configuration that sources
		// $ZSH/oh-my-zsh.sh or loads a plugin that is not installed: the block the
		// markers bound (zsh_plugins.go) is replaced with a line saying why, and the
		// strip refuses rather than guesses when a marker is missing.
		if !withPlugins {
			if err := leaveZshPluginsOut(zshrcPath); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to leave the plugin set out of .zshrc",
					err)
			}
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetP10k), filepath.Join(homeDir, ".p10k.zsh")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy Powerlevel10k configuration",
				err)
		}
		// The theme ships in the repository rather than being fetched at install
		// time because it has to match the palette the terminal emulators declare,
		// and no published theme does: the closest one only looked like it. It is
		// copied before the cache rebuild below, which is what makes bat find it.
		// It is also the newest asset here, so an older checkout may not ship it
		// yet: the installer clones the repository's default branch while the
		// binary can come from a branch that already carries this file, and a
		// missing theme must not abort the whole shell step. TestRepoAssetsExist is
		// what guarantees the repository contains it.
		batThemesSrcDir := filepath.Join(repoDir, "dotfiles-bat", "themes")
		batThemeEntries, readErr := os.ReadDir(batThemesSrcDir)
		if readErr != nil {
			SendLog(stepID, "Skipping the bat theme: this checkout predates it")
		} else {
			batConfigDir := filepath.Join(homeDir, ".config", "bat")
			batThemesDir := filepath.Join(batConfigDir, "themes")
			if err := os.MkdirAll(batThemesDir, 0o755); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to create the bat themes directory",
					err)
			}
			// Every shipped .tmTheme is copied, not just the dotfiles one: the theme
			// switch selects between them by name, so a theme the machine does not have
			// built would be an "Unknown theme" at the moment it was chosen.
			batInstalled := 0
			for _, entry := range batThemeEntries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmTheme") {
					continue
				}
				if err := system.CopyFile(filepath.Join(batThemesSrcDir, entry.Name()), filepath.Join(batThemesDir, entry.Name())); err != nil {
					return wrapStepError("shell", "Install Zsh",
						"Failed to copy the bat theme",
						err)
				}
				batInstalled++
			}
			// The themes whose .tmTheme this checkout does not ship are generated here
			// from their definitions, so every theme the switch offers paints bat and
			// the installed set is the definitions' set rather than "the shipped files
			// plus whatever was added last". The two files under version control are
			// written by this too, with the bytes the guard pins.
			generated, err := generateBatThemesFromDefinitions(repoDir, batThemesDir)
			if err != nil {
				// Best effort, like the cache rebuild below: the copied themes are
				// installed and usable, and a theme that could not be generated is named
				// in the log rather than aborting the whole shell step.
				SendLog(stepID, fmt.Sprintf("Warning: a bat theme could not be generated from the theme definitions: %v", err))
			} else if generated > 0 {
				SendLog(stepID, fmt.Sprintf("Generated %d bat theme(s) from themes/*.toml", generated))
			}
			// Count what is there rather than what was copied: the number the installer
			// reports is the number of themes bat can select.
			if present, err := os.ReadDir(batThemesDir); err == nil {
				batInstalled = 0
				for _, entry := range present {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tmTheme") {
						batInstalled++
					}
				}
			}
			// bat reads a theme from its cache, not from the themes directory, so the
			// cache has to be rebuilt for the copy above to have any effect. The rebuild
			// is best-effort on purpose: bat comes from the platform package lists, but
			// not on every platform, and a missing or failing bat must not fail the
			// whole shell step. BAT_CONFIG_DIR is set explicitly so the build reads the
			// directory the theme was just written into instead of resolving whatever
			// XDG_CONFIG_HOME the ambient environment happens to carry.
			if !system.CommandExists("bat") {
				SendLog(stepID, "bat not found in PATH, skipping the theme cache rebuild")
			} else if result := system.RunWithLogs("bat cache --build", &system.ExecOptions{
				Env: []string{"BAT_CONFIG_DIR=" + batConfigDir},
			}, func(line string) {
				SendLog(stepID, line)
			}); result.Error != nil {
				SendLog(stepID, fmt.Sprintf("Warning: could not rebuild the bat cache: %v", result.Error))
			} else {
				SendLog(stepID, fmt.Sprintf("✓ %d bat theme(s) installed", batInstalled))
			}
		}
		// Oh My Zsh manages its own checkout. Writing a vendored copy over an
		// existing clone dirties its tracked files, and `omz update` then fails on
		// the autostash pop, so the official installer runs only when nothing is
		// installed yet and an existing installation is never written into.
		ohMyZshDir := filepath.Join(homeDir, ".oh-my-zsh")
		switch {
		case !withPlugins:
			// The set was left out: no checkout, no installer, and nothing to repair.
			SendLog(stepID, "Oh My Zsh left out with the plugin set")
		case shouldInstallOhMyZsh(ohMyZshDir):
			SendLog(stepID, "Installing Oh My Zsh...")
			if err := installOhMyZsh(ohMyZshDir, stepID); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to install Oh My Zsh",
					err)
			}
		default:
			SendLog(stepID, "Oh My Zsh already installed, leaving it untouched")
		}
		// Termux: Add zsh to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding zsh to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "zsh") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Zsh configured with Powerlevel10k")

	case "nushell":
		SendLog(stepID, "Installing Nushell and dependencies...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesPacman(m) {
			logArchStillMissing(stepID, result, packages.archUnavailable)
		}
		if err := archMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to install Nushell and dependencies",
				result.Error)
		}
		SendLog(stepID, "Copying Nushell configuration...")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetStarship), filepath.Join(homeDir, ".config/starship.toml")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy starship configuration",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetBashEnvJSON), filepath.Join(homeDir, ".config/bash-env-json")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy bash-env-json",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetBashEnvNu), filepath.Join(homeDir, ".config/bash-env.nu")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy bash-env.nu",
				err)
		}

		var nuDir string
		if runtime.GOOS == "darwin" {
			nuDir = filepath.Join(homeDir, "Library/Application Support/nushell")
		} else {
			nuDir = filepath.Join(homeDir, ".config/nushell")
		}
		if err := system.EnsureDir(nuDir); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to create Nushell config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetNushell), nuDir); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy Nushell configuration",
				err)
		}
		// Patch config.nu based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchNushellForWM(filepath.Join(nuDir, "config.nu"), m.Choices.WindowMgr); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to configure config.nu for window manager",
				err)
		}
		// Termux: Add nu to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding nushell to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "nu") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Nushell configured")
	}

	return nil
}

// zellijPlatformPackages returns the packages for the Zellij window manager.
// Debian/Ubuntu does not carry zellij at all and Fedora does not either, so
// those entries are empty and the caller reports the omission instead of
// handing apt or dnf a name it would reject and fail the whole transaction on.
// Arch carries zellij, so its entry is unchanged, but it still goes through
// archPackages so a future edit cannot slip an unknown name past the filter.
func zellijPlatformPackages() (platformPackages, []string) {
	debian, unavailable := debianPackages("zellij")
	arch, archGaps := archPackages("zellij")
	fedora, fedoraGaps := fedoraPackages("zellij")
	return platformPackages{
		Termux:            "zellij",
		Brew:              "zellij",
		Arch:              arch,
		Fedora:            fedora,
		Debian:            debian,
		archUnavailable:   archGaps,
		fedoraUnavailable: fedoraGaps,
	}, unavailable
}

// herdrSoundPlayers names the external mp3 players Herdr shells out to for its
// sound alerts, in the order it tries them. Herdr ships no audio decoder, so the
// managed `[ui.sound] enabled = true` is only a working setting when one of these
// resolves on the machine; when none does Herdr fails silently, which is the
// defect issue #270 describes.
var herdrSoundPlayers = []string{"paplay", "pw-play", "ffplay", "mpg123", "mpv"}

// herdrSoundProvision is the player the installer installs when the managed
// configuration enables sound and none of Herdr's own players resolves. mpg123
// is the smallest of the five and the one every package manager this installer
// reaches carries, so it is the smallest declared dependency that makes the
// setting able to fire.
const herdrSoundProvision = "mpg123"

// herdrCommandExists is system.CommandExists, indirected so a guard can resolve
// a player without a real installation.
var herdrCommandExists = system.CommandExists

// herdrBinaryPresent reports whether Herdr itself is installed. It is a variable
// so the Herdr step's guards exercise the configuration and the sound player
// without a real Herdr binary or a real package manager.
var herdrBinaryPresent = system.CommandExists

// herdrLog is the Herdr step's log sink. It is a variable so a guard can capture
// what the step reports and fail the moment a managed `[ui.sound] enabled = true`
// stops naming the dependency the machine is missing.
var herdrLog = SendLog

// herdrSoundPlayer returns the first of Herdr's players that resolves on this
// machine, or "" when none does. The order is Herdr's own, so the answer is the
// player that would actually fire.
func herdrSoundPlayer(exists func(string) bool) string {
	for _, player := range herdrSoundPlayers {
		if exists(player) {
			return player
		}
	}
	return ""
}

// herdrSoundPlayerLookup resolves a player on PATH and in the Homebrew prefix,
// for the same reason componentPresentAfterInstall does: Homebrew writes its
// binaries into its own prefix and this process's PATH does not necessarily
// contain it moments after a successful install.
func herdrSoundPlayerLookup(command string) bool {
	if herdrCommandExists(command) {
		return true
	}
	prefix := os.Getenv("HOMEBREW_PREFIX")
	if prefix == "" {
		prefix = system.GetBrewPrefix()
	}
	info, err := os.Stat(filepath.Join(prefix, "bin", command))
	return err == nil && !info.IsDir()
}

// herdrSoundPlayerPresent is the lookup the step uses. It is a variable so a
// guard can describe the machine without a real player on it.
var herdrSoundPlayerPresent = herdrSoundPlayerLookup

// parseHerdrSoundEnabled reports whether the managed Herdr configuration turns
// sound alerts on. It reads the `[ui.sound] enabled` key from the small TOML
// subset the file uses, the way parseThemeDefinition does, rather than pulling in
// a parser for one boolean.
func parseHerdrSoundEnabled(data []byte) bool {
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section != "ui.sound" || strings.TrimSpace(key) != "enabled" {
			continue
		}
		value = strings.TrimSpace(value)
		if i := strings.IndexByte(value, '#'); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		return value == "true"
	}
	return false
}

// herdrSoundPlatformPackages returns the player the installer provisions for
// Herdr's sound alerts. The columns go through the same availability filters as
// every other package set, so a distribution that stopped carrying mpg123 is
// reported rather than aborting the transaction.
func herdrSoundPlatformPackages() platformPackages {
	debian, _ := debianPackages(herdrSoundProvision)
	arch, archGaps := archPackages(herdrSoundProvision)
	fedora, fedoraGaps := fedoraPackages(herdrSoundProvision)
	return platformPackages{
		Termux:            herdrSoundProvision,
		Brew:              herdrSoundProvision,
		Arch:              arch,
		Fedora:            fedora,
		Debian:            debian,
		archUnavailable:   archGaps,
		fedoraUnavailable: fedoraGaps,
	}
}

// ensureHerdrSound makes the installed Herdr configuration honest: it reads
// whether sound is enabled, resolves one of the players Herdr shells out to and
// provisions the smallest declared player when none resolves. It reports the gap
// explicitly when a player still cannot be provided, so the failure Herdr hides
// in herdr-client.log is named by the installer instead.
//
// It returns whether sound is ready to fire or was not asked for. That is the
// condition the Herdr step's success line is gated on, so a configuration whose
// dependency is missing never gets an unqualified success. A configuration that
// does not enable sound installs nothing: a player is only provisioned where the
// sound is wanted.
func ensureHerdrSound(m *Model, stepID, configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		herdrLog(stepID, fmt.Sprintf(
			"Could not read the Herdr configuration to verify sound alerts: %v", err))
		return false
	}
	if !parseHerdrSoundEnabled(data) {
		return true
	}

	if player := herdrSoundPlayer(herdrSoundPlayerPresent); player != "" {
		herdrLog(stepID, fmt.Sprintf("Sound alerts are enabled; Herdr will use %s", player))
		return true
	}

	herdrLog(stepID, fmt.Sprintf(
		"Sound alerts are enabled but none of Herdr's players (%s) resolves; installing %s...",
		strings.Join(herdrSoundPlayers, ", "), herdrSoundProvision))
	result := installPlatformPackages(m, stepID, herdrSoundPlatformPackages(), func(line string) {
		herdrLog(stepID, line)
	})
	if result != nil && result.Error != nil {
		herdrLog(stepID, fmt.Sprintf("Could not install %s: %v", herdrSoundProvision, result.Error))
	}

	if player := herdrSoundPlayer(herdrSoundPlayerPresent); player != "" {
		herdrLog(stepID, fmt.Sprintf("Sound alerts are enabled; Herdr will use %s", player))
		return true
	}

	herdrLog(stepID, fmt.Sprintf(
		"Sound alerts cannot play: the managed configuration enables them but none of %s resolves. "+
			"Herdr fails silently when no player is installed; install one (for example %s) and run the installer again.",
		strings.Join(herdrSoundPlayers, ", "), herdrSoundProvision))
	return false
}

func stepInstallWM(m *Model) error {
	homeDir := os.Getenv("HOME")
	wm := m.Choices.WindowMgr
	stepID := "wm"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Window Manager",
			"Failed to locate the cloned repository",
			err)
	}

	switch wm {
	case "tmux":
		if !system.CommandExists("tmux") {
			SendLog(stepID, "Installing Tmux...")
			result := installPlatformPackages(m, stepID, platformPackages{
				Termux: "tmux",
				Brew:   "tmux",
				Arch:   "tmux",
				Fedora: "tmux",
				Debian: "tmux",
			}, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				return wrapStepError("wm", "Install Tmux",
					"Failed to install Tmux",
					result.Error)
			}
		} else {
			SendLog(stepID, "Tmux already installed")
		}

		// The plugin set is a switch (install_plugin_sets.go). With it off, none of the
		// plugin work runs, and the configuration this step installs carries no plugin
		// declaration and no line that runs TPM, because a machine that left the set
		// out must not be asked to run a checkout that is not there.
		withPlugins := !pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDTmux)
		if !withPlugins {
			SendLog(stepID, "The Tmux plugin set is turned off; TPM is not installed and no plugin is downloaded")
		}

		// TPM
		if withPlugins {
			tpmDir := filepath.Join(homeDir, ".tmux/plugins/tpm")
			if _, err := os.Stat(tpmDir); os.IsNotExist(err) {
				SendLog(stepID, "Cloning TPM (Tmux Plugin Manager)...")
				result := system.RunWithLogs(fmt.Sprintf("git clone https://github.com/tmux-plugins/tpm %s", tpmDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("wm", "Install Tmux",
						"Failed to clone TPM (Tmux Plugin Manager)",
						result.Error)
				}
			}
		}

		SendLog(stepID, "Copying Tmux configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".tmux")); err != nil {
			return wrapStepError("wm", "Install Tmux",
				"Failed to create .tmux directory",
				err)
		}
		// The plugin seed is optional: tmux.conf declares every plugin through TPM
		// and the install_plugins run below downloads them. Copy only when the
		// repository actually ships a seed, instead of failing the whole step.
		if withPlugins {
			pluginsSrc := filepath.Join(repoDir, repoAssetTmuxPlugins)
			if system.DirExists(pluginsSrc) {
				if err := system.CopyDir(pluginsSrc, filepath.Join(homeDir, ".tmux", "plugins")); err != nil {
					return wrapStepError("wm", "Install Tmux",
						"Failed to copy Tmux plugins",
						err)
				}
			} else {
				SendLog(stepID, "No seeded Tmux plugins in the repository; TPM will install them")
			}
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetTmuxConf), filepath.Join(homeDir, ".tmux.conf")); err != nil {
			return wrapStepError("wm", "Install Tmux",
				"Failed to copy tmux.conf",
				err)
		}

		// Configure tmux to use the user's chosen shell
		SendLog(stepID, "Configuring tmux default shell...")
		tmuxConfPath := filepath.Join(homeDir, ".tmux.conf")
		shellName := ""
		switch m.Choices.Shell {
		case "fish":
			shellName = "fish"
		case "zsh":
			shellName = "zsh"
		case "nushell":
			shellName = "nu"
		}
		if shellName != "" {
			// Find the full path to the shell
			shellFullPath := ""
			if m.SystemInfo.IsTermux {
				// In Termux, construct the path directly (which command has issues)
				prefix := os.Getenv("PREFIX")
				if prefix == "" {
					prefix = "/data/data/com.termux/files/usr"
				}
				shellFullPath = filepath.Join(prefix, "bin", shellName)
			} else {
				result := system.Run(fmt.Sprintf("which %s", shellName), nil)
				if result.Error == nil && result.Output != "" {
					shellFullPath = strings.TrimSpace(result.Output)
				}
			}
			if shellFullPath == "" {
				shellFullPath = shellName // Fallback
			}

			// Replace placeholder in tmux.conf with actual shell config
			content, err := os.ReadFile(tmuxConfPath)
			if err == nil {
				shellConfig := fmt.Sprintf("set -g default-command \"%s\"\nset -g default-shell \"%s\"", shellFullPath, shellFullPath)
				newContent := strings.Replace(string(content), "# DOTFILES_DEFAULT_SHELL", shellConfig, 1)
				os.WriteFile(tmuxConfPath, []byte(newContent), 0644)
			}
		}

		// A run that left the plugin set out must not install a configuration that
		// declares the plugins or runs TPM: the checkout is not there, and tmux would be
		// asked to run a missing file on every start. The configuration carries markers
		// around exactly that text (tmux_plugins.go), and the patch refuses rather than
		// guesses when they are missing.
		if !withPlugins {
			if err := leaveTmuxPluginsOut(tmuxConfPath); err != nil {
				return wrapStepError("wm", "Install Tmux",
					"Failed to leave the plugin set out of tmux.conf",
					err)
			}
		}

		// Install plugins. The result is checked instead of discarded: TPM fails
		// here with no running tmux server or no network, and logging
		// "✓ Tmux configured" over it left the user with an empty plugin set and
		// no way to tell. The step's own work is already done and tmux retries
		// the plugins through TPM at startup, so the failure is reported as a
		// warning and the success line is withheld.
		if withPlugins {
			SendLog(stepID, "Installing Tmux plugins...")
			result := system.RunWithLogs(filepath.Join(homeDir, ".tmux/plugins/tpm/bin/install_plugins"), nil, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				SendLog(stepID, fmt.Sprintf("Warning: could not install the Tmux plugins: %v", result.Error))
			} else {
				SendLog(stepID, "✓ Tmux configured")
			}
		} else {
			// Nothing about the plugins ran, so nothing about them could have failed:
			// the configuration this step installed is complete.
			SendLog(stepID, "✓ Tmux configured")
		}

	case "zellij":
		if !system.CommandExists("zellij") {
			SendLog(stepID, "Installing Zellij...")
			packages, unavailable := zellijPlatformPackages()
			// Zellij is the window manager the user explicitly asked for, not a
			// companion package, so a distribution that cannot install it has to
			// fail the step rather than report a success that never happened.
			if usesApt(m) && len(unavailable) > 0 {
				logDebianUnavailable(stepID, unavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			// Neither Arch nor Fedora carries zellij on every host: Fedora does not
			// carry it at all, and Arch carries it but the guard is kept for a
			// future edit that drops it. The selected-component rule is applied after
			// the install rather than before it: zellij is only reported missing
			// once every available route has been attempted, so a host whose
			// Homebrew can provide it is not told it cannot be installed. When the
			// distribution's own list is empty or filtered, installPlatformPackages
			// goes to its default branch and Homebrew is the only route; the
			// presence check below confirms the result, looking in the Homebrew
			// prefix as well as PATH.
			result := installPlatformPackages(m, stepID, packages, func(line string) {
				SendLog(stepID, line)
			})
			if usesPacman(m) && len(packages.archUnavailable) > 0 && !componentPresentAfterInstall(result, "zellij") {
				logArchUnavailable(stepID, packages.archUnavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			if usesDnf(m) && len(packages.fedoraUnavailable) > 0 && !componentPresentAfterInstall(result, "zellij") {
				logFedoraUnavailable(stepID, packages.fedoraUnavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			if result.Error != nil {
				return wrapStepError("wm", "Install Zellij",
					"Failed to install Zellij",
					result.Error)
			}
		} else {
			SendLog(stepID, "Zellij already installed")
		}

		SendLog(stepID, "Copying Zellij configuration...")
		zellijDir := filepath.Join(homeDir, ".config/zellij")
		if err := system.EnsureDir(zellijDir); err != nil {
			return wrapStepError("wm", "Install Zellij",
				"Failed to create Zellij config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetZellij), zellijDir); err != nil {
			return wrapStepError("wm", "Install Zellij",
				"Failed to copy Zellij configuration",
				err)
		}

		// Configure zellij to use the user's chosen shell
		SendLog(stepID, "Configuring zellij default shell...")
		zellijConfPath := filepath.Join(zellijDir, "config.kdl")
		shellPath := ""
		switch m.Choices.Shell {
		case "fish":
			shellPath = "fish"
		case "zsh":
			shellPath = "zsh"
		case "nushell":
			shellPath = "nu"
		}
		if shellPath != "" {
			// Append default_shell config to zellij config.kdl
			f, err := os.OpenFile(zellijConfPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(fmt.Sprintf("\n// Default shell (configured by dotfiles)\ndefault_shell \"%s\"\n", shellPath))
				f.Close()
			}
		}
		SendLog(stepID, "✓ Zellij configured")

	case "herdr":
		if !herdrBinaryPresent("herdr") {
			herdrLog(stepID, "Installing Herdr...")
			if err := installHerdrBinary(m, stepID); err != nil {
				return wrapStepError("wm", "Install Herdr",
					"Failed to install Herdr",
					err)
			}
		} else {
			herdrLog(stepID, "Herdr already installed")
		}

		herdrLog(stepID, "Copying Herdr configuration...")
		herdrDir := filepath.Join(homeDir, ".config", "herdr")
		if err := system.EnsureDir(herdrDir); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to create Herdr config directory",
				err)
		}
		installedConfig := filepath.Join(herdrDir, "config.toml")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetHerdrConfig), installedConfig); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to copy Herdr configuration",
				err)
		}
		if err := os.Chmod(installedConfig, 0644); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to make Herdr configuration writable",
				err)
		}
		// The managed configuration turns sound alerts on, and Herdr ships no
		// decoder. Verify a player resolves and provision the declared one when it
		// does not, so the step cannot report a success the machine cannot show.
		if ensureHerdrSound(m, stepID, installedConfig) {
			herdrLog(stepID, "✓ Herdr configured")
		} else {
			herdrLog(stepID, "Herdr configuration installed, but its sound alerts will stay silent until a player resolves")
		}

		// The plugins come after the binary resolves and the config is in place,
		// and the step fails if a declared plugin did not install or verify: a
		// consumer that only sees the exit status would otherwise read a failed
		// build as a finished install.
		if err := installHerdrPlugins(m, stepID); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to install the Herdr plugins",
				err)
		}
	}

	return nil
}

// clipboardProvidersFor reports which packages Neovim needs for
// clipboard=unnamedplus on this platform, and whether it needs any at all.
//
// macOS needs none, because Neovim uses pbcopy and pbpaste there. Termux has
// neither package and uses the termux-api clipboard instead. Everything else
// gets both: the installer cannot know whether the session offers Wayland or
// X11, and under WSL either may be the one that works.
func clipboardProvidersFor(info *system.SystemInfo) (packages string, needed bool) {
	if info == nil || info.OS == system.OSMac || info.IsTermux {
		return "", false
	}
	return "xclip wl-clipboard", true
}

// clipboardPlatformPackages names the providers for every package manager that
// needs them. All four use the same two package names, so they are filled from
// one string: leaving a field empty would silently skip that platform.
func clipboardPlatformPackages(providers string) platformPackages {
	return platformPackages{
		Brew:   providers,
		Arch:   providers,
		Fedora: providers,
		Debian: providers,
	}
}

// nvimUserOwnedEntries are destination-relative paths inside ~/.config/nvim
// that belong to the user and to lazy.nvim rather than to the repository.
// lazy-lock.json is rewritten on the machine whenever plugins are updated, so
// its content legitimately differs from the repository copy and the installer
// must neither overwrite nor delete it when it is already present.
var nvimUserOwnedEntries = []string{"lazy-lock.json"}

// installConfigDir copies a configuration directory into place and reports every
// path the repository no longer ships that had to be removed, so a pruned file
// is visible in the install log instead of disappearing silently. keep names
// destination-relative paths that are user-owned runtime state.
func installConfigDir(stepID, src, dst string, keep ...string) error {
	removed, err := system.CopyDirPruned(src, dst, keep...)
	if err != nil {
		return err
	}
	if len(removed) == 0 {
		return nil
	}
	SendLog(stepID, fmt.Sprintf("Removed %d path(s) the repository no longer ships:", len(removed)))
	for _, path := range removed {
		SendLog(stepID, fmt.Sprintf("  ✗ %s", path))
	}
	return nil
}

// nvimPlatformPackages returns the packages the Neovim step installs. The
// Debian entry drops lazygit and tree-sitter-cli, which its repositories do not
// carry and apt cannot skip, and the Fedora entry drops lazygit, which Fedora
// does not carry for the same reason. The Arch entry goes through archPackages
// for the same reason even though every name it asks for exists there today.
func nvimPlatformPackages() (platformPackages, []string) {
	debian, unavailable := debianPackages(
		"neovim", "git", "gcc", "fzf", "fd-find", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter-cli",
	)
	arch, archGaps := archPackages(
		"neovim", "git", "gcc", "fzf", "fd", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter",
	)
	fedora, fedoraGaps := fedoraPackages(
		"neovim", "git", "gcc", "fzf", "fd-find", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter-cli",
	)
	return platformPackages{
		Termux:            "neovim git clang fzf fd ripgrep bat curl lazygit",
		Brew:              "nvim git gcc fzf fd ripgrep coreutils bat curl lazygit tree-sitter",
		Arch:              arch,
		Fedora:            fedora,
		Debian:            debian,
		archUnavailable:   archGaps,
		fedoraUnavailable: fedoraGaps,
	}, unavailable
}

func stepInstallNvim(m *Model) error {
	homeDir := os.Getenv("HOME")
	stepID := "nvim"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Neovim",
			"Failed to locate the cloned repository",
			err)
	}

	// Obsidian path
	SendLog(stepID, "Creating Obsidian directories...")
	obsidianDir := filepath.Join(homeDir, ".config/obsidian")
	system.EnsureDir(obsidianDir)
	system.EnsureDir(filepath.Join(obsidianDir, "templates"))

	// Check Node.js
	if !system.CommandExists("node") {
		SendLog(stepID, "Installing Node.js...")
		result := installPlatformPackages(m, stepID, platformPackages{
			Termux: "nodejs",
			Brew:   "node",
			Arch:   "nodejs npm",
			Fedora: "nodejs npm",
			Debian: "nodejs npm",
		}, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("nvim", "Install Neovim",
				"Failed to install Node.js (required for LSP servers)",
				result.Error)
		}
	} else {
		SendLog(stepID, "Node.js already installed")
	}

	// Install dependencies
	SendLog(stepID, "Installing Neovim and dependencies...")
	// Termux package names differ from desktop Linux package managers.
	packages, unavailable := nvimPlatformPackages()
	if usesApt(m) {
		logDebianUnavailable(stepID, unavailable)
	}
	result := installPlatformPackages(m, stepID, packages, func(line string) {
		SendLog(stepID, line)
	})
	if usesDnf(m) {
		logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
	}
	if result.Error != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to install Neovim and dependencies",
			result.Error)
	}

	// Neovim runs with clipboard=unnamedplus, so it needs a provider from the
	// system. Without one every yank stays inside Neovim, never reaches the
	// desktop clipboard, and the editor reports nothing at all.
	if providers, needed := clipboardProvidersFor(m.SystemInfo); needed {
		SendLog(stepID, "Installing clipboard providers for Neovim...")
		clipboard := installPlatformPackages(m, stepID, clipboardPlatformPackages(providers), func(line string) {
			SendLog(stepID, line)
		})
		if clipboard.Error != nil {
			// Deliberately not fatal. Neovim itself is installed and usable, only
			// the clipboard integration is missing, and a distribution that names
			// the packages differently should not abort a finished installation.
			SendLog(stepID, "Could not install xclip and wl-clipboard, so yanks will stay inside Neovim: "+clipboard.Error.Error())
		} else {
			SendLog(stepID, "✓ Clipboard providers installed")
		}
	}

	// Copy config
	SendLog(stepID, "Copying Neovim configuration...")
	nvimDir := filepath.Join(homeDir, ".config/nvim")
	if err := system.EnsureDir(nvimDir); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to create Neovim config directory",
			err)
	}
	// Copy nvim config directory. The destination must end up matching the
	// checkout: a file the repository dropped used to survive forever and keep
	// being loaded beside the file that replaced it (issue #13).
	srcNvim := filepath.Join(repoDir, repoAssetNvim)
	if err := installConfigDir(stepID, srcNvim, nvimDir, nvimUserOwnedEntries...); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to copy Neovim configuration",
			err)
	}

	// lazygit is the git UI the Neovim configuration opens, and lazygit ships no
	// palette that follows the terminal. Its config.yml is copied here so the theme
	// switch has the file to write, and so a machine that never applies a theme
	// still starts on the same palette as everything else.
	SendLog(stepID, "Copying lazygit configuration...")
	lazygitDir := filepath.Join(homeDir, ".config/lazygit")
	if err := system.EnsureDir(lazygitDir); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to create the lazygit configuration directory",
			err)
	}
	if err := system.CopyFile(filepath.Join(repoDir, repoAssetLazygit), filepath.Join(lazygitDir, "config.yml")); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to copy the lazygit configuration",
			err)
	}

	// btop ships no configuration, so it drew its own built-in palette beside every
	// themed terminal. Its config carries the color_theme selection and nothing else
	// btop does not already default, and the themes directory beside it gets a
	// .theme file for every theme that can paint btop - generated from the same
	// definitions the switch reads, so a switch to any offered theme lands on a file
	// btop has. Without the generated files color_theme would name a theme btop does
	// not have and btop would fall back to its built-in one.
	SendLog(stepID, "Copying btop configuration...")
	btopDir := filepath.Join(homeDir, ".config/btop")
	btopThemesDir := filepath.Join(btopDir, "themes")
	if err := system.EnsureDir(btopThemesDir); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to create the btop configuration directory",
			err)
	}
	if err := system.CopyFile(filepath.Join(repoDir, repoAssetBtop), filepath.Join(btopDir, "btop.conf")); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to copy the btop configuration",
			err)
	}
	if generated, err := generateBtopThemesFromDefinitions(repoDir, btopThemesDir); err != nil {
		// Best effort, like bat's generated themes: the copied config is installed and
		// the default theme is selectable, so a theme that could not be generated is
		// named in the log rather than aborting the whole step.
		SendLog(stepID, fmt.Sprintf("Warning: a btop theme could not be generated from the theme definitions: %v", err))
	} else if generated > 0 {
		SendLog(stepID, fmt.Sprintf("Generated %d btop theme(s) from themes/*.toml", generated))
	}

	// Install Claude Code CLI (optional, don't fail on error)
	// Skip on Termux - Claude Code doesn't support Android
	if !m.SystemInfo.IsTermux {
		SendLog(stepID, "Installing Claude Code CLI (optional)...")
		system.RunWithLogs(`curl -fsSL https://claude.ai/install.sh | bash`, nil, func(line string) {
			SendLog(stepID, line)
		})
	} else {
		SendLog(stepID, "Skipping Claude Code (not supported on Termux)")
	}

	// Install OpenCode CLI (optional, don't fail on error)
	// Skip on Termux - OpenCode doesn't support Android
	if !m.SystemInfo.IsTermux {
		SendLog(stepID, "Installing OpenCode CLI (optional)...")
		system.RunWithLogs(`curl -fsSL https://opencode.ai/install | bash`, nil, func(line string) {
			SendLog(stepID, line)
		})
	} else {
		SendLog(stepID, "Skipping OpenCode (not supported on Termux)")
	}

	SendLog(stepID, "✓ Neovim configured with dotfiles setup")
	return nil
}

// reapplyRecordedTheme writes the theme this machine has applied back into the
// configuration files a run replaced, and reports how many files it wrote.
//
// It is the picker's write without the picker's transaction. It does not touch the
// record of which theme is applied, and it keeps no undo: choosing a theme is a
// user's action with a way back, and an install is not one. It uses the picker's
// own gates -- the artifact list, the path each artifact answers with, the
// ownership proof, the Neovim reachability rule and the same renderer -- so a file
// the picker would refuse is refused here for the same reason, and a file it would
// write is written with the bytes it would have written. A file that carries no
// marker of ours is left exactly as it is: unlike the picker, an install never
// adopts.
//
// Three cases leave every file alone and are not errors: no theme is recorded, the
// record cannot be read, and the record names a theme this checkout does not
// define. The last is a normal race -- the installed binary can come from a branch
// newer than the checkout it reads -- and failing there would report an install as
// failed after it had already installed everything, which is the choice the
// Windows Terminal scheme makes for the same reason.
func reapplyRecordedTheme(homeDir, repoDir string) (int, error) {
	rec := readDotfilesThemeRecord()
	if rec == nil || rec.Theme == "" || repoDir == "" {
		return 0, nil
	}

	defs, err := loadThemeDefinitions(repoDir)
	if err != nil {
		return 0, fmt.Errorf("the theme definitions could not be read, so no file was restored: %w", err)
	}
	def, ok := themeByID(defs, rec.Theme)
	if !ok {
		return 0, nil
	}

	var files []string
	for _, art := range themeActiveArtifacts {
		path := themeInstalledPath(art, homeDir)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return len(files), fmt.Errorf("could not read %s, so it was left as it is: %w", path, err)
		}
		content := string(data)
		if !strings.Contains(content, themeOwnershipMarker) && !themeInstalledFileIsOurs(repoDir, art, content) {
			continue
		}
		if art.Tool == "nvim" {
			// The same rule the picker applies: the line names a colorscheme
			// Neovim has to find, and a generated one is installed here before the
			// line that names it is written. A name Neovim cannot load starts the
			// editor with E185 on every launch, which is worse than the previous
			// colorscheme.
			if reason := reachableNvimColorscheme(homeDir, def); reason != "" {
				SendLog("theme", fmt.Sprintf("Neovim was left on its previous colorscheme: %s", reason))
				continue
			}
		}
		block, err := themeArtifactBlock(art, def)
		if err != nil {
			return len(files), err
		}
		updated, ok := replaceThemeBlock(content, block, art.Block)
		if !ok || updated == content {
			continue
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return len(files), fmt.Errorf("could not write the %s block into %s: %w", art.Tool, path, err)
		}
		if !slices.Contains(files, path) {
			files = append(files, path)
		}
	}
	return len(files), nil
}

// stepReapplyTheme is the run's last word on the theme.
//
// Every step that writes a configuration file writes the file this repository
// ships, and those carry the default theme's generated block. The picker is the
// only caller of applyDotfilesTheme, and the record of which theme is applied
// was read only to be displayed, so a run left a machine that records one theme
// rendering another: measured on one machine, an install at 16:59:42 put the
// default palette back into ~/.zshrc, ~/.p10k.zsh and the Herdr configuration
// while theme.json went on naming Nocturne over six files, and the prompt --
// which reads its colours from the first two -- painted the default palette.
//
// It runs after every step that writes a configuration file, and before the
// cleanup step removes the checkout the theme definitions are read from.
func stepReapplyTheme(m *Model) error {
	stepID := "theme"

	repoDir := m.RepoDir
	if repoDir == "" {
		repoDir = m.DotfilesRepoDir
	}

	changed, err := reapplyRecordedTheme(os.Getenv("HOME"), repoDir)
	if err != nil {
		return wrapStepError("theme", "Restore the Applied Theme",
			"Failed to restore the theme this machine has applied", err)
	}
	if changed == 0 {
		SendLog(stepID, "No theme is recorded, so every file is left as the repository ships it")
		return nil
	}
	SendLog(stepID, fmt.Sprintf("Restored the applied theme in %d file(s)", changed))
	return nil
}

func stepCleanup(m *Model) error {
	stepID := "cleanup"
	if m.WorkDir == "" {
		SendLog(stepID, "Nothing to clean up")
		return nil
	}

	// Only remove the directory this run created, and only when the checkout is
	// actually inside it. Never derive the target from the current directory.
	if !strings.HasPrefix(m.RepoDir, m.WorkDir+string(os.PathSeparator)) {
		SendLog(stepID, fmt.Sprintf("Warning: refusing to remove %s", m.WorkDir))
		return nil
	}

	SendLog(stepID, fmt.Sprintf("Removing temporary checkout at %s...", m.WorkDir))
	if err := os.RemoveAll(m.WorkDir); err != nil {
		// Non-critical error, just log it
		SendLog(stepID, "Warning: Could not remove temporary directory")
		return nil
	}
	m.WorkDir = ""
	m.RepoDir = ""
	SendLog(stepID, "✓ Cleanup complete")
	return nil
}

// stepSetDefaultShell sets the selected shell as the user's default shell
// In non-interactive mode, this handles Termux specially (via .bashrc)
// and attempts to set the shell on other systems if possible
func stepSetDefaultShell(m *Model) error {
	stepID := "setshell"
	shell := m.Choices.Shell
	homeDir := os.Getenv("HOME")

	var shellCmd string
	switch shell {
	case "fish":
		shellCmd = "fish"
	case "zsh":
		shellCmd = "zsh"
	case "nushell":
		shellCmd = "nu"
	default:
		SendLog(stepID, fmt.Sprintf("Unknown shell: %s, skipping", shell))
		return nil
	}

	// Termux: no chsh available, modify .bashrc to auto-start shell
	if m.SystemInfo.IsTermux {
		SendLog(stepID, "Configuring shell auto-start for Termux...")

		// Find the shell path
		shellPath := system.Run(fmt.Sprintf("which %s", shellCmd), nil)
		if shellPath.Error != nil || strings.TrimSpace(shellPath.Output) == "" {
			SendLog(stepID, fmt.Sprintf("Shell '%s' not found in PATH, skipping", shellCmd))
			return nil
		}
		shellPathStr := strings.TrimSpace(shellPath.Output)

		// Read existing .bashrc
		bashrcPath := filepath.Join(homeDir, ".bashrc")
		existingContent := ""
		if data, err := os.ReadFile(bashrcPath); err == nil {
			existingContent = string(data)
		}

		// Check if already configured
		if strings.Contains(existingContent, "# dotfiles shell auto-start") {
			SendLog(stepID, "Shell auto-start already configured in ~/.bashrc")
			return nil
		}

		// Append auto-start configuration
		autoStartConfig := fmt.Sprintf(`
# dotfiles shell auto-start
if [ -x "%s" ] && [ -z "$DOTFILES_SHELL_STARTED" ]; then
    export DOTFILES_SHELL_STARTED=1
    exec %s
fi
`, shellPathStr, shellPathStr)

		f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return wrapStepError("setshell", "Set Default Shell",
				"Failed to open ~/.bashrc for writing",
				err)
		}
		defer f.Close()

		if _, err := f.WriteString(autoStartConfig); err != nil {
			return wrapStepError("setshell", "Set Default Shell",
				"Failed to write shell auto-start to ~/.bashrc",
				err)
		}

		SendLog(stepID, fmt.Sprintf("✓ Configured %s to auto-start in ~/.bashrc", shell))
		SendLog(stepID, "Close and reopen Termux for changes to take effect")
		return nil
	}

	// Non-Termux: set the shell as root through the credential cached for the run.
	// Find the shell path first
	shellPath := system.Run(fmt.Sprintf("which %s", shellCmd), nil)
	if shellPath.Error != nil || strings.TrimSpace(shellPath.Output) == "" {
		SendLog(stepID, fmt.Sprintf("Shell '%s' not found in PATH, skipping", shellCmd))
		return nil
	}
	shellPathStr := strings.TrimSpace(shellPath.Output)

	// Get current username
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}
	if currentUser == "" {
		// Fallback to whoami command (useful in Docker containers)
		whoamiResult := system.Run("whoami", nil)
		if whoamiResult.Error == nil {
			currentUser = strings.TrimSpace(whoamiResult.Output)
		}
	}
	if currentUser == "" {
		SendLog(stepID, "Could not determine current user, skipping shell change")
		return nil
	}

	// First, ensure shell is in /etc/shells
	SendLog(stepID, fmt.Sprintf("Adding %s to /etc/shells if needed...", shellPathStr))
	checkShells := system.Run(fmt.Sprintf("grep -q '^%s$' /etc/shells", shellPathStr), nil)
	if checkShells.Error != nil {
		// Shell not in /etc/shells, try to add it
		addResult := runSudoSilently(fmt.Sprintf("sh -c 'echo \"%s\" >> /etc/shells'", shellPathStr), nil)
		if addResult.Error != nil {
			SendLog(stepID, fmt.Sprintf("Could not add %s to /etc/shells (may need manual setup)", shellPathStr))
		}
	}

	// Try usermod first (more reliable than chsh in scripts, and it needs no
	// terminal because sudo reads the credential cached for the run).
	SendLog(stepID, fmt.Sprintf("Setting %s as default shell for %s...", shell, currentUser))
	result := runSudoSilently(fmt.Sprintf("usermod -s %s %s", shellPathStr, currentUser), nil)
	if result.Error != nil {
		// usermod failed, try chsh as fallback
		SendLog(stepID, "usermod failed, trying chsh...")
		result = runSudoSilently(fmt.Sprintf("chsh -s %s %s", shellPathStr, currentUser), nil)
		if result.Error != nil {
			// Both failed - not critical, just inform user
			SendLog(stepID, fmt.Sprintf("Could not set default shell automatically"))
			SendLog(stepID, fmt.Sprintf("Run manually: chsh -s %s", shellPathStr))
			return nil
		}
	}

	SendLog(stepID, fmt.Sprintf("✓ Default shell set to %s", shell))
	SendLog(stepID, "Log out and log back in for changes to take effect")
	return nil
}

// droppableBrewfileEntry matches the directives the toolset step deliberately
// does not install.
var droppableBrewfileEntry = regexp.MustCompile(`^(vscode|winget)[ \t]+"`)

// filterBrewfile returns the part of a Brewfile the toolset step installs,
// together with the number of lines it dropped per directive.
//
// The vscode and winget sections are out of scope on purpose. Installing Windows
// desktop applications (Chrome, Office, Teams, a JDK) as a side effect of a
// dotfiles install is out of proportion, and the vscode extensions need the
// `code` CLI, which is absent on the servers this installer also runs on. Both
// sections stay in the Brewfile for whoever applies it by hand on a workstation,
// and the step reports how many lines it dropped instead of hiding the omission.
//
// Everything else is kept byte for byte, in order, including comments, blank
// lines and the inline `if OS.linux?` guard. The Brewfile stays the single
// source of truth: there is no second list in Go to drift from it, so a tool
// added to the file is provisioned without a code change.
func filterBrewfile(content string) (string, map[string]int) {
	dropped := map[string]int{}
	var kept strings.Builder

	for _, line := range strings.SplitAfter(content, "\n") {
		// SplitAfter keeps the newline attached, so a kept line is written back
		// exactly as it was read and a dropped line disappears with it.
		match := droppableBrewfileEntry.FindStringSubmatch(strings.TrimSpace(line))
		if match != nil {
			dropped[match[1]]++
			continue
		}
		kept.WriteString(line)
	}

	return kept.String(), dropped
}

// brewfileEntryLine matches a Brewfile directive line: a lowercase directive
// name, whitespace and the quoted entry name, for example `brew "btop"`.
// Comment and blank lines do not match.
var brewfileEntryLine = regexp.MustCompile(`^[a-z]+[ \t]+"`)

// countBrewfileEntries counts the installable lines in an already filtered
// Brewfile, so the step can report the scale of the install and skip a file that
// declares nothing to install.
func countBrewfileEntries(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if brewfileEntryLine.MatchString(strings.TrimSpace(line)) {
			count++
		}
	}
	return count
}

// describeDroppedBrewfileDirectives renders the dropped counts in a stable
// order, so two runs over the same Brewfile log the same line.
func describeDroppedBrewfileDirectives(dropped map[string]int) string {
	names := make([]string, 0, len(dropped))
	for name := range dropped {
		names = append(names, name)
	}
	slices.Sort(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s (%d lines)", name, dropped[name]))
	}
	return strings.Join(parts, ", ")
}

// stepInstallToolset provisions the machine toolset the repository declares in
// its Brewfile, on top of what the shell step installs for the configuration to
// work at shell start. It reads the Brewfile from the checkout the clone step
// created, filters out the sections this step does not install, and hands the
// result to `brew bundle`.
//
// The step is best-effort by design: a missing Brewfile, an entry that no longer
// exists upstream or a network failure is logged and the installation continues.
// The one failure it reports is a missing checkout, because a run that has no
// checkout has already failed at the clone step and a silent skip there would
// hide that.
func stepInstallToolset(m *Model) error {
	stepID := "toolset"

	if skipToolset() {
		SendLog(stepID, fmt.Sprintf("Skipping the toolset by request (%s is set)", envSkipToolset))
		return nil
	}

	if m.SystemInfo.IsTermux {
		SendLog(stepID, "Skipping the toolset: Homebrew is not available on Termux")
		return nil
	}

	if !system.BrewInstalled() {
		SendLog(stepID, "Skipping the toolset: Homebrew is not installed")
		return nil
	}

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError("toolset", "Install Toolset",
			"Failed to read the Brewfile toolset",
			err)
	}

	brewfilePath := filepath.Join(repoDir, repoAssetBrewfile)
	content, err := os.ReadFile(brewfilePath)
	if err != nil {
		SendLog(stepID, fmt.Sprintf("Skipping the toolset: cannot read %s (%v)", brewfilePath, err))
		return nil
	}

	filtered, dropped := filterBrewfile(string(content))
	entries := countBrewfileEntries(filtered)
	if entries == 0 {
		SendLog(stepID, fmt.Sprintf("Skipping the toolset: %s declares no installable entries", brewfilePath))
		return nil
	}

	SendLog(stepID, fmt.Sprintf("Installing %d entries declared in %s...", entries, brewfilePath))
	if len(dropped) > 0 {
		SendLog(stepID, fmt.Sprintf("Out of scope, dropped from this run: %s", describeDroppedBrewfileDirectives(dropped)))
	}

	// brew bundle needs a file, and the Brewfile lives inside the checkout the
	// cleanup step removes, so the filtered copy is written to a temporary file
	// this step owns and removes on every path out.
	tmp, err := os.CreateTemp("", "dotfiles-brewfile-*")
	if err != nil {
		SendLog(stepID, "Skipping the toolset: cannot create a temporary Brewfile: "+err.Error())
		return nil
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.WriteString(filtered); err != nil {
		_ = tmp.Close()
		SendLog(stepID, "Skipping the toolset: cannot write the temporary Brewfile: "+err.Error())
		return nil
	}
	if err := tmp.Close(); err != nil {
		SendLog(stepID, "Skipping the toolset: cannot write the temporary Brewfile: "+err.Error())
		return nil
	}

	// HOMEBREW_BUNDLE_NO_UPGRADE keeps brew bundle to installing what is missing.
	// Without it `brew bundle install` upgrades every already-installed formula,
	// which an installer must never do to a machine it is only setting up.
	result := runBrewWithLogs(
		fmt.Sprintf("bundle install --file=%q", tmpPath),
		&system.ExecOptions{Env: []string{"HOMEBREW_BUNDLE_NO_UPGRADE=1"}},
		func(line string) { SendLog(stepID, line) },
	)
	if result.Error != nil {
		SendLog(stepID, fmt.Sprintf("Some Brewfile entries could not be installed; the installation continues: %v", result.Error))
		return nil
	}

	SendLog(stepID, "✓ Toolset installed")
	return nil
}
