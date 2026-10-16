package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Version, Commit and Date are the build's identity, all three injected at link
// time by the release workflow:
//
//	-ldflags "-X main.Version=<tag> -X main.Commit=<sha> -X main.Date=<rfc3339>"
//
// They are handed to the TUI before anything can render or print, so --version,
// the splash and the update check all read the same three values. A build made by
// hand leaves all three at their defaults and reports "dev build".
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// CLI flags for non-interactive mode
type cliFlags struct {
	version        bool
	help           bool
	test           bool
	dryRun         bool
	nonInteractive bool
	checkUpdate    bool
	selfUpdate     bool
	verify         bool
	channel        string
	terminal       string
	// os is the platform the run works from. It is a choice like the terminal, and it is
	// read before anything is planned: the non-interactive path used to impose the
	// detected platform over whatever the caller said.
	os        string
	shell     string
	windowMgr string
	nvim      bool
	font      bool
	backup    bool
	noAnim    bool
	// revert is the write half of the record --verify reads: it puts this machine
	// back where the last run found it. restore is the same operation for a backup
	// directory named by hand, which is the way back for a machine whose record is
	// gone. Neither is a flag of the run: they are commands of their own, and both
	// honour --dry-run.
	revert  bool
	restore string

	noMouse  bool
	noSprite bool

	// skip is the comma-separated list of units the run is told to leave out. It is a
	// string rather than a list because that is what a command line carries; the set
	// is read out of it, validated against the catalogue, and ordered there.
	skip string
}

// registerFlags declares every flag on the given set, so the help text can be
// checked against the flags the binary really accepts without parsing anything.
// A flag nobody documents is a flag nobody finds, and the display switches added
// with the creature are exactly the ones a user on an odd terminal looks for.
func registerFlags(fs *flag.FlagSet, flags *cliFlags) {
	fs.BoolVar(&flags.version, "version", false, "Show version information")
	fs.BoolVar(&flags.version, "v", false, "Show version information (shorthand)")
	fs.BoolVar(&flags.help, "help", false, "Show help message")
	fs.BoolVar(&flags.help, "h", false, "Show help message (shorthand)")
	fs.BoolVar(&flags.checkUpdate, "check-update", false, "Read the latest published release and report it")
	fs.BoolVar(&flags.selfUpdate, "self-update", false, "Install the latest published release over this binary")
	fs.BoolVar(&flags.verify, "verify", false, "Check the machine against the record of the last install")
	fs.StringVar(&flags.channel, "channel", "", "Release channel to follow: stable (default) or dev")
	fs.BoolVar(&flags.test, "test", false, "Run in test mode (uses temporary directory)")
	fs.BoolVar(&flags.test, "t", false, "Run in test mode (shorthand)")
	fs.BoolVar(&flags.dryRun, "dry-run", false, "Show what would be installed, restored or reverted without doing it")
	fs.BoolVar(&flags.revert, "revert", false, "Put this machine back where the last install found it")
	fs.StringVar(&flags.restore, "restore", "", "Put a backup directory back: dotfiles --restore ~/.dotfiles-backup-<timestamp>")
	fs.BoolVar(&flags.nonInteractive, "non-interactive", false, "Run without TUI, use CLI flags")
	fs.StringVar(&flags.terminal, "terminal", "", "Terminal: "+strings.Join(tui.SupportedTerminals(runtime.GOOS), ", "))
	fs.StringVar(&flags.os, "os", "", "Platform to work from: "+strings.Join(tui.SupportedOperatingSystems(), ", ")+" (default: the machine this runs on)")
	fs.StringVar(&flags.shell, "shell", "", "Shell: fish, zsh, nushell")
	fs.StringVar(&flags.windowMgr, "wm", "", "Window manager: tmux, zellij, herdr, none")
	fs.BoolVar(&flags.nvim, "nvim", false, "Install Neovim configuration")
	fs.BoolVar(&flags.font, "font", false, "Install Nerd Font")
	fs.BoolVar(&flags.backup, "backup", true, "Backup existing configs (default: true)")
	fs.StringVar(&flags.skip, "skip", "", "Leave these units and plugin sets out, comma-separated: "+strings.Join(tui.SwitchableSkipIDs(), ", "))
	fs.BoolVar(&flags.noAnim, "no-anim", false, "Disable animations (same as DOTFILES_ANIM=0)")
	fs.BoolVar(&flags.noMouse, "no-mouse", false, "Do not track the mouse pointer (same as DOTFILES_MOUSE=0)")
	fs.BoolVar(&flags.noSprite, "no-sprite", false, "Draw the creature as glyphs, not as the shaded sprite (same as DOTFILES_SPRITE=0)")
}

func parseFlags() *cliFlags {
	flags := &cliFlags{}
	registerFlags(flag.CommandLine, flags)

	flag.Parse()
	return flags
}

func main() {
	// Hand the linker-injected build identity to the TUI before anything can
	// render or print, so the splash, --version and the update check all agree
	// about which build this is.
	tui.Version, tui.BuildCommit, tui.BuildDate = Version, Commit, Date

	flags := parseFlags()

	if flags.version {
		fmt.Println("dotfiles " + tui.BuildLabel())
		os.Exit(0)
	}

	if flags.help {
		printHelp()
		os.Exit(0)
	}

	// The channel is the one choice the update commands and the interface share,
	// so it is handed to the TUI package before either runs. An unknown channel is
	// refused here rather than treated as stable: a flag that silently did nothing
	// would leave the user reading a stable answer under a dev name.
	if err := tui.SetUpdateChannel(flags.channel); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	// The two update commands are the scriptable form of what the utilities
	// section offers, and they run the same code: one function, two entries. They
	// sit above test mode and dry run so a check never depends on either, and the
	// update itself honours DOTFILES_DRY_RUN inside the TUI package, where the
	// gate lives.
	if flags.checkUpdate {
		os.Exit(reportUpdate())
	}
	if flags.selfUpdate {
		if err := runSelfUpdate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// --verify is the read half of the record the run writes: it compares this
	// machine with what the last install said it did, and never writes. It sits
	// with the update commands, above test mode and dry run, because it makes no
	// change that either could gate.
	if flags.verify {
		os.Exit(reportVerify())
	}

	// --revert and --restore are the other half of the record --verify reads: what
	// a run changed, put back. They sit below the dry-run gate so --dry-run reaches
	// them the same way it reaches a run, and above test mode so a script can ask
	// for either without a temporary directory being invented for it.
	if flags.revert {
		os.Exit(reportRevert(flags.dryRun))
	}
	if flags.restore != "" {
		os.Exit(reportRestore(flags.restore, flags.dryRun))
	}

	if flags.test {
		setupTestMode()
	}

	if flags.dryRun {
		os.Setenv("DOTFILES_DRY_RUN", "1")
		fmt.Println("🧪 Dry-run mode: No actual installations will be performed")
	}

	// --no-anim and DOTFILES_ANIM=0 are the same switch: the flag sets the variable
	// the TUI reads when it builds the model, so the gate has one source of truth
	// and the flag cannot drift from the environment.
	if flags.noAnim {
		os.Setenv("DOTFILES_ANIM", "0")
	}

	// --no-mouse and DOTFILES_MOUSE=0 are the same switch, on the same terms: the
	// flag sets the variable the TUI reads when it builds the model. It is a switch
	// of its own rather than a side of --no-anim because the pointer costs the user
	// something -- the terminal gives its own selection up while an application is
	// reading the mouse -- so it has to be possible to want the animation without
	// it, and to get the pointer back without losing the creature.
	if flags.noMouse {
		os.Setenv("DOTFILES_MOUSE", "0")
	}

	// --no-sprite and DOTFILES_SPRITE=0 are the same switch on the same terms. It is
	// its own switch because the shaded sprite is the one drawing a terminal cannot
	// be asked to be good at: a run whose terminal claims true colour but renders
	// block glyphs badly keeps the glyph cat with this and loses nothing else.
	if flags.noSprite {
		os.Setenv("DOTFILES_SPRITE", "0")
	}

	// Non-interactive mode: run installation directly with provided flags
	if flags.nonInteractive {
		if err := runNonInteractive(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Interactive TUI mode
	model := tui.NewModel()
	// The pointer is asked for only when the run can use it, and the decision is
	// the model's: NewModel already read the gates to set Hovering, so the program
	// option and the model cannot disagree about whether a pointer event means
	// anything. With hover off NO mouse option is passed at all, which is what gives
	// the terminal's own selection back -- a run that merely ignored the events
	// would still have cost the user the drag.
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if model.Hovering {
		options = append(options, tea.WithMouseAllMotion())
	}
	// The renderer ends each frame with a single write to the program's output,
	// so bracketing that write in the terminal's synchronized-output mode (DECSET
	// 2026) is what stops the terminal from painting a frame as it arrives -- the
	// tearing the animation shows. outputWriter hands the stream back untouched
	// when stdout is not a terminal or DOTFILES_SYNC=0, so a piped run and a
	// gated run keep the bytes they always had, and the wrapper keeps the
	// stream's file descriptor, which is what bubbletea reads the window size
	// from and, on Windows, what it enables virtual terminal processing on.
	options = append(options, tea.WithOutput(outputWriter(os.Stdout)))

	p := tea.NewProgram(model, options...)
	tui.SetGlobalProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running installer: %v\n", err)
		os.Exit(1)
	}
}

// reportRevert puts this machine back where the last install found it and returns
// the exit status a script reads: 0 when every file the run changed is back, or was
// already in the state the revert restores, 1 when a file could not be put back,
// and 2 when there is no record to revert.
//
// It prints what it did file by file, because this is the one command whose reader
// has to be able to check it: a summary that says "done" while a file is still
// changed is the defect this command exists to answer.
func reportRevert(dryRun bool) int {
	report, err := tui.RevertLastRun(tui.RevertOptions{DryRun: dryRun})

	if errors.Is(err, tui.ErrNoRecord) {
		fmt.Println("No record of an install on this machine, so there is nothing to revert.")
		fmt.Println("A record is written by a run that installs; `dotfiles --restore=<backup>` puts one backup directory back.")
		return 2
	}

	if report.Record != nil && len(report.Files) > 0 {
		// The record's own count, not this run's plan: on a second run nothing is
		// left to do, and "changed 0 file(s)" would misdescribe the record the
		// lines below are read from.
		fmt.Printf("The run of %s changed %d file(s):\n\n",
			report.Record.Timestamp.Local().Format("2006-01-02 15:04:05"), len(report.Files))
	}
	for _, file := range report.Files {
		fmt.Printf("  %s %s (%s) %s\n", revertMark(file.Status), file.Path, file.Action, revertPhrase(file))
	}

	if report.SnapshotDir != "" {
		fmt.Printf("\nThe state this replaced is kept in %s\n", report.SnapshotDir)
	} else if report.SnapshotReason != "" {
		fmt.Printf("\nThe state this replaced could not be kept: %s\n", report.SnapshotReason)
	}

	if len(report.Irreversible) > 0 {
		fmt.Println("\nNot undone by this command:")
		for _, item := range report.Irreversible {
			fmt.Println("  - " + item)
		}
	}

	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		return 1
	case dryRun:
		fmt.Println("\nDry run: nothing was changed.")
		return 0
	case len(report.Files) == 0:
		// A run that changed no file has nothing to put back. Saying that every
		// file "already holds what it held before it" would describe a revert that
		// examined files the record never named.
		fmt.Println("The run changed no configuration file, so there is nothing to put back.")
		return 0
	case report.Reverted == 0:
		fmt.Println("\nEvery file the run changed already holds what it held before it.")
		return 0
	default:
		fmt.Printf("\n%d file(s) are back where the run found them.\n", report.Reverted)
		return 0
	}
}

// revertMark is the one-character summary of what happened to a file.
func revertMark(status tui.RevertStatus) string {
	switch status {
	case tui.RevertReverted:
		return "✓"
	case tui.RevertWouldRevert:
		return "→"
	case tui.RevertAlready:
		return "·"
	case tui.RevertUnrevertible, tui.RevertFailed:
		return "!"
	}
	return "?"
}

// revertPhrase is the sentence that follows a file's name: what the revert did to
// it, or why it could not.
func revertPhrase(file tui.RevertFileReport) string {
	switch file.Status {
	case tui.RevertReverted:
		if file.Reason != "" {
			return "put back -- " + file.Reason
		}
		return "put back"
	case tui.RevertWouldRevert:
		if string(file.Action) == "created" {
			return "would be removed"
		}
		return "would be put back"
	case tui.RevertAlready:
		return "already in the state the revert restores"
	case tui.RevertUnrevertible:
		return "cannot be put back -- " + file.Reason
	case tui.RevertFailed:
		return "FAILED -- " + file.Reason
	}
	return string(file.Status)
}

// reportRestore puts a backup directory back and returns the exit status a script
// reads: 0 when every entry went back, 1 when an entry could not be placed, and 2
// when the path is not a backup this program wrote or could not be read at all.
func reportRestore(path string, dryRun bool) int {
	report, err := tui.RestoreBackupDir(path, dryRun)

	if err != nil && report.Planned == 0 && report.Restored == 0 && report.Failed == 0 {
		// Nothing was planned and nothing was attempted: the directory itself was
		// refused, which is a different answer from an entry that did not fit.
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	fmt.Printf("Restoring %s\n\n", path)
	for _, entry := range report.Entries {
		switch entry.Status {
		case "restored":
			fmt.Printf("  ✓ %s put back\n", entry.Path)
		case "would-restore":
			fmt.Printf("  → %s would be put back\n", entry.Path)
		case "skipped":
			fmt.Printf("  · %s %s\n", entry.Path, entry.Reason)
		default:
			fmt.Printf("  ! %s %s\n", entry.Path, entry.Reason)
		}
	}

	if report.SnapshotDir != "" {
		fmt.Printf("\nThe state this replaced is kept in %s\n", report.SnapshotDir)
	} else if report.SnapshotReason != "" {
		fmt.Printf("\nThe state this replaced could not be kept: %s\n", report.SnapshotReason)
	}

	switch {
	case dryRun && report.Failed > 0:
		// A plan that names an entry it cannot place is not a yes: a script asking
		// whether this backup can go back has to be able to read the answer.
		fmt.Printf("\nDry run: nothing was changed, and %d entr(ies) could not be put back.\n", report.Failed)
		return 1
	case dryRun:
		fmt.Println("\nDry run: nothing was changed.")
		return 0
	case err != nil:
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		return 1
	default:
		fmt.Printf("\n%d entr(ies) are back where they were.\n", report.Restored)
		return 0
	}
}

// reportUpdate prints what the latest published release is beside this build and
// returns the exit status a script reads: 0 when this build is the published one,
// 1 when a newer one exists, 2 when the answer could not be read at all.
//
// The third status is the point of the split. "I could not reach GitHub" and
// "you are up to date" are different facts, and a script that cannot tell them
// apart would treat a network failure as a green light.
func reportUpdate() int {
	report := tui.CheckForUpdate()
	fmt.Println("dotfiles " + tui.BuildLabel())
	fmt.Println(report.Summary)

	switch {
	case !report.Known:
		return 2
	case report.Newer:
		return 1
	default:
		return 0
	}
}

// runSelfUpdate replaces this binary with the latest published release.
func runSelfUpdate() error {
	notice, err := tui.UpdateSelf()
	if err != nil {
		return err
	}
	fmt.Println(notice)
	return nil
}

// reportVerify prints the record's account of this machine and returns the exit
// status a script reads: 0 when every path and backup the record names is still
// there, 1 when something it names is gone, and 2 when a record is present and
// could not be read. A machine with no record is not a failure: the report says
// nothing was installed here and the status is 0.
//
// The report claims a file's contents only where the record carries a digest for it:
// the per-file changes do, and a record written before they existed does not. Where
// there is no digest the honest statement is presence, the action and the backup, with
// the unverifiable part said per line rather than in a footer.
func reportVerify() int {
	report := tui.VerifyInstall()
	for _, line := range report.Lines {
		fmt.Println(line)
	}

	switch {
	case report.Unreadable:
		return 2
	case !report.Consistent:
		return 1
	default:
		return 0
	}
}

// platformFromFlags reads --os and validates it before the run plans anything. The
// refusal is the tui package's, so the flag and the setup screen's first row keep one
// vocabulary; an unknown value stops the run instead of being read as Linux, which is
// what the non-interactive path did before the flag existed.
func platformFromFlags(flags *cliFlags) (string, error) {
	if err := tui.ValidateOperatingSystem(flags.os); err != nil {
		return "", err
	}
	return flags.os, nil
}

func runNonInteractive(flags *cliFlags) error {
	// Validate required flags
	if flags.shell == "" {
		return fmt.Errorf("--shell is required (fish, zsh, nushell)")
	}

	// Normalize inputs
	terminal := strings.ToLower(flags.terminal)
	shell := strings.ToLower(flags.shell)
	wm := strings.ToLower(flags.windowMgr)

	// Validate values
	validShells := map[string]bool{"fish": true, "zsh": true, "nushell": true}
	validWMs := map[string]bool{"tmux": true, "zellij": true, "herdr": true, "none": true, "": true}

	if terminal == "" {
		terminal = "none"
	}

	// The terminal axis is the one that depends on the platform: kitty is only
	// installable on macOS, so asking for it here is refused before anything is
	// planned, with the values this tool does support named in the error.
	// The selection the flag carries is read and validated against the catalogue here,
	// so a unit the run cannot leave out -- or a name nothing recognises -- stops the
	// run before anything is installed rather than being quietly dropped.
	skipUnits, skipSets, err := tui.ParseSkipSelection(flags.skip)
	if err != nil {
		return err
	}

	if err := tui.ValidateTerminal(terminal, runtime.GOOS); err != nil {
		return err
	}
	if !validShells[shell] {
		return fmt.Errorf("invalid shell: %s (valid: fish, zsh, nushell)", shell)
	}
	if !validWMs[wm] {
		return fmt.Errorf("invalid window manager: %s (valid: tmux, zellij, herdr, none)", wm)
	}
	// The platform is checked with the other values, before anything is planned: an
	// absent one is the machine's answer and an unknown one is refused rather than
	// replaced by it.
	platform, err := platformFromFlags(flags)
	if err != nil {
		return err
	}

	// Default an empty window manager to "none"
	if wm == "" {
		wm = "none"
	}

	// Create choices
	choices := tui.UserChoices{
		OS:           platform,
		Terminal:     terminal,
		Shell:        shell,
		WindowMgr:    wm,
		InstallNvim:  flags.nvim,
		InstallFont:  flags.font,
		CreateBackup: flags.backup,

		DisabledPluginSets: skipSets,
	}

	fmt.Println("🚀 dotfiles Non-Interactive Installer")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Terminal:    %s\n", choices.Terminal)
	fmt.Printf("  OS:          %s\n", tui.PlatformFor(choices))
	fmt.Printf("  Shell:       %s\n", choices.Shell)
	fmt.Printf("  Window Mgr:  %s\n", choices.WindowMgr)
	fmt.Printf("  Neovim:      %v\n", choices.InstallNvim)
	fmt.Printf("  Font:        %v\n", choices.InstallFont)
	fmt.Printf("  Backup:      %v\n", choices.CreateBackup)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Run the installation
	return tui.RunNonInteractive(choices, skipUnits...)
}

func setupTestMode() {
	// Create a temporary test directory
	testDir := filepath.Join(os.TempDir(), "dotfiles-test")
	testHome := filepath.Join(testDir, "home")
	testConfig := filepath.Join(testHome, ".config")

	// Create directories
	os.MkdirAll(testConfig, 0755)

	// Override HOME to use test directory
	os.Setenv("HOME", testHome)
	os.Setenv("DOTFILES_TEST_MODE", "1")

	fmt.Printf("🧪 Test mode enabled!\n")
	fmt.Printf("   Test HOME: %s\n", testHome)
	fmt.Printf("   Your real configs are SAFE.\n")
	fmt.Printf("   Press Enter to continue...\n")

	// Wait for user to acknowledge
	fmt.Scanln()
}

func printHelp() {
	fmt.Println(helpText)
}

// helpText is the help the binary prints, kept as a value so a test can read it
// rather than capture stdout.
const helpText = `dotfiles - TUI installer for dotfiles terminal environment

Usage:
  dotfiles [flags]

Interactive Mode (default):
  Just run 'dotfiles' to start the TUI installer.

Non-Interactive Mode:
  dotfiles --non-interactive --shell=<shell> [options]

Flags:
  -h, --help           Show this help message
  -v, --version        Show version information
  -t, --test           Run in test mode (uses temporary directory)
  --dry-run            Show what would be installed, restored or reverted without doing it
  --non-interactive    Run without TUI, use CLI flags instead
  --skip=<names>       Leave units and plugin sets out of the run,
                       comma-separated (--skip=officecli,plugins-herdr). The run
                       needs clone, cleanup and setshell and refuses to leave those
                       out; everything else it installs can be skipped, and the
                       plugin sets this installer installs itself can be turned
                       off the same way. A skipped unit is reported rather than
                       silently dropped. It is the same selection the review
                       screen offers before an interactive run, and the plan is
                       built by the same builder either way
  --check-update       Read the release the chosen channel offers and print it
                       beside this build. Exits 0 when this build is that release,
                       1 when a newer one exists, and 2 when the answer could not
                       be read at all
  --self-update        Install the release the chosen channel offers over this
                       binary. The download is verified against the release's own
                       SHA256SUMS before anything moves, the binary it replaced
                       is kept beside it as dotfiles.previous, and a binary a
                       package manager installed is refused (run
                       brew upgrade dotfiles for that one)
  --verify             Check this machine against the record the last install
                       wrote. Every configuration path and backup that run named
                       is reported as here or missing, and every file the run
                       changed is compared against the digest it held when the run
                       finished: still holding what the run left, changed since,
                       or unreadable. Where the record carries no digest -- one
                       written before it kept them, or a path the run did not
                       write -- the report says the path is here and unverifiable,
                       never that it matches. The digests are content digests, so
                       a path that is now a link to the same content reads as
                       still holding what the run left.
                       Exits 0 when the machine is what that run left, 1 when a
                       path or backup it names is gone or a file no longer holds
                       what the run left, and 2 when a record is present and could
                       not be read. A machine with no record exits 0 and says
                       nothing was installed here
  --revert             Put this machine back where the last install found it: every
                       file the run modified or removed is copied back out of the
                       backup, and every file it created is removed. The record the
                       run writes names them per file, and each file is checked
                       against the digest the run took before it wrote it, so the
                       command proves what it did. A file whose backup copy is gone
                       is reported, never skipped, and so is a path the record names
                       as written but cannot vouch for file by file -- a symlink inside
                       a configuration directory is the ordinary case, and a record
                       written before the installer recorded files one by one is the
                       same shape of claim. A link is not reported when the copy the
                       run took of it still points where it points and what it reaches
                       is inside a path the record names, with the files there holding
                       what the copies hold or named by the record. The report names the
                       backup directory that puts a whole entry back when the record
                       names one. The state it replaces is kept in a backup of its own. Exits 0 when the machine is back (or
                       already was), 1 when a file could not be put back, and 2 when
                       there is no record. Honours --dry-run
  --restore=<path>     Put a backup directory back, the same operation the main
                       menu's "Restore from Backup" offers: dotfiles --restore
                       ~/.dotfiles-backup-2026-10-16-120000. The replacement is
                       copied beside the destination and only a complete copy moves
                       into place, so a destination is never absent because a copy
                       failed halfway; the state it replaces is kept in a backup of
                       its own; and a directory this program did not write is
                       refused. Exits 0 when every entry went back, 1 when an entry
                       could not be placed, 2 when the directory is not a backup
                       this program wrote. --dry-run prints the plan and changes
                       nothing, and exits 1 when that plan names an entry the
                       backup does not hold
  --channel=<name>     Channel to follow: stable (the latest published release,
                       the default) or dev (the highest-versioned pre-release). The choice
                       is remembered in update-check.json and offered again as
                       the main menu's "Update channel" row
  --no-anim            Disable animations (same as DOTFILES_ANIM=0); animation
                       is also off when stdout is not a terminal or TERM=dumb
  --no-mouse           Do not ask the terminal for pointer motion (same as
                       DOTFILES_MOUSE=0): the terminal keeps its own selection
                       and scrolling, and the creature looks at the selection
  --no-sprite          Draw the creature as glyphs instead of the shaded volume
                       (same as DOTFILES_SPRITE=0)

Non-Interactive Options:
  --os=<platform>      Platform to work from: mac, linux, termux (default: the
                       machine this runs on)
  --shell=<shell>      Shell to install (required): fish, zsh, nushell
  --terminal=<term>    Terminal: alacritty, wezterm, ghostty, none
                       (kitty is available on macOS only)
  --wm=<wm>            Window manager: tmux, zellij, herdr, none
  --nvim               Install Neovim configuration
  --font               Install Nerd Font
  --backup=false       Disable config backup (default: true)

Display and terminals:
  The installer draws a frame, a moving tip and a shaded creature. Each can be
  turned off, and each switch is both a flag and an environment variable, so a
  script can pin it too.

  --no-anim            DOTFILES_ANIM=0 stops the tip, the creature's motion and
                       the drifting background. Off by itself when stdout is
                       not a terminal, or TERM=dumb.
  --no-mouse           DOTFILES_MOUSE=0 stops asking the terminal for pointer
                       motion, giving selection and scrolling back to it. Off by
                       itself when stdout is not a terminal and in Termux, where
                       a finger drag arrives as a wheel report; DOTFILES_MOUSE=1
                       asks for it anyway, for a Termux session with a real mouse
                       attached.
  --no-sprite          DOTFILES_SPRITE=0 keeps the creature but draws the ASCII
                       cat instead of the shaded volume. Use it on a terminal
                       that reports true colour and still renders block glyphs
                       badly.
  DOTFILES_SYNC=0      Stop bracketing each frame in synchronized output, which
                       is on whenever stdout is a terminal. Use it if a
                       multiplexer renders the frames worse with it.
  DOTFILES_VERBOSE=1   Print every command the installer runs.

Examples:
  # Interactive TUI
  dotfiles

  # Non-interactive with Fish + Herdr + Neovim
  dotfiles --non-interactive --shell=fish --wm=herdr --nvim

  # Test mode with Zsh + Tmux (no terminal, no nvim)
  dotfiles --test --non-interactive --shell=zsh --wm=tmux

  # Verbose output (shows all command logs)
  DOTFILES_VERBOSE=1 dotfiles --non-interactive --shell=fish --nvim

  # Check this machine against the last install's record
  dotfiles --verify

  # Read what reverting the last install would do, without doing it
  dotfiles --revert --dry-run

  # Put the last install back where it found this machine
  dotfiles --revert

  # Put one backup directory back
  dotfiles --restore ~/.dotfiles-backup-2026-10-16-120000

Navigation (TUI mode):
  ↑/k, ↓/j        Navigate up/down
  Enter/Space     Select option
  Esc             Go back
  q               Quit
  d               Toggle details (during installation)

For more info: https://github.com/albersg/dotfiles`
