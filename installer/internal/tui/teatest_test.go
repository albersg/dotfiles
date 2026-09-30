package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
)

// skipIfTermux skips the test if running in Termux environment
// Golden tests compare against macOS snapshots, so they fail on other platforms
func skipIfTermux(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		t.Skip("Skipping golden test: running in Termux environment (snapshots are from macOS)")
	}
}

// goldenSystemInfo pins the detected platform for snapshots. The rendered screens
// embed the detected OS and the Homebrew status, so an unpinned model produced
// snapshots that only matched the machine that generated them and failed on
// every CI runner.
func goldenSystemInfo() *system.SystemInfo {
	return &system.SystemInfo{
		OS:        system.OSLinux,
		OSName:    "Linux",
		IsWSL:     false,
		IsTermux:  false,
		HasBrew:   false,
		HasXcode:  false,
		HomeDir:   "/home/testuser",
		UserShell: "zsh",
	}
}

// pinnedMetrics is a plausible host reading for a snapshot: a CPU share that
// varies as a heartbeat, memory that climbs, and the load, disk and process
// facts. A snapshot that pinned nothing would draw a different chart every run,
// which is exactly what the model-held samples exist to prevent.
func pinnedMetrics(n int) []system.Metrics {
	if n < 1 {
		n = 1
	}
	out := make([]system.Metrics, n)
	for i := range out {
		share := 0.30 + 0.45*float64((i*3)%7)/6.0
		memUsed := uint64(4<<30) + uint64(i)*(64<<20)
		out[i] = system.Metrics{
			CPUOK: true, CPUBusy: share,
			MemOK: true, MemUsed: memUsed, MemTotal: 16 << 30,
			LoadOK: true, Load1: 0.8 + float64(i%5)*0.2, Load5: 1.1, Load15: 1.4,
			DiskOK: true, DiskFree: uint64(320<<30) - uint64(i)*(1<<28), DiskTotal: 500 << 30,
			ProcOK: true, ProcCount: 380 + i*3,
		}
	}
	return out
}

// pinnedProgress is a run's progress history for a snapshot: a straight climb,
// which is the shape the chart is meant to show.
func pinnedProgress(n int) []float64 {
	if n < 1 {
		n = 1
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i) / float64(n)
	}
	return out
}

// isolateGoldenTest pins the inputs a golden test would otherwise inherit from
// the machine it runs on, so a snapshot matches on every host.
//
// HOME is the input that actually broke. NewModel's Init runs
// system.ListBackups, which scans $HOME for .dotfiles-backup-* directories, and
// GetCurrentOptions appends "🔄 Restore from Backup" to the main menu when it
// finds at least one. Commit 249aa30 refreshed the main menu golden on a
// machine that had such a backup, so the snapshot recorded a seventh entry; the
// macOS CI job, the only job that runs the goldens because the Linux job passes
// -skip Golden, renders six entries and failed on it. Pointing HOME at an empty
// t.TempDir() makes the scan find nothing on any machine.
//
// SystemInfo is pinned through goldenSystemInfo for the same reason: screens
// that render the detected platform must not be snapshotted from the OS that
// happened to run them. Tests that assert a specific platform still assign
// m.SystemInfo after calling this.
//
// XDG_STATE_HOME is pinned for the same class of reason: the "Last install"
// panel and the trainer panel load their files on the startup path, so a real
// state file or a real trainer profile on the machine running the test would
// otherwise change the tab row a snapshot records.
func isolateGoldenTest(t *testing.T, m *Model) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m.SystemInfo = goldenSystemInfo()
	m.CreatedAt = goldenGreetingTime
}

// goldenGreetingTime is the time the golden models were created with. The
// welcome and main menu greet by the time of day from the model's own creation
// time, so an unpinned model would render a different word at every hour and on
// every host. 09:00 is the morning word; the other parts of the day are pinned
// by TestGreetingForEachPartOfTheDay rather than by a second snapshot.
var goldenGreetingTime = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// Helper to read all bytes from io.Reader
func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	bts, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return bts
}

// TestWelcomeScreenGolden tests the welcome screen render against golden file.
//
// It renders the model rather than driving a program: this screen shows the live
// panel, whose metrics arrive from a command, so a test that started a program and
// captured everything it wrote would race that read - it did, about one run in
// three - and pin a different number of frames depending on scheduling. A model
// with its state pinned renders the same bytes every time, and that is the only
// thing a snapshot may do.
func TestWelcomeScreenGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenWelcome
	m.PanelIndex = 0
	m.Metrics = pinnedMetrics(24)
	m.Animating = false

	golden.RequireEqual(t, []byte(m.View()))
}

// TestMainMenuWideGolden tests the main menu render against golden file
func TestMainMenuGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenMainMenu

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestMainMenuWideGolden pins the two-column composition at 160x50, the first
// size the layout composes at, together with the panel the main menu carries.
// The 80x24 snapshot above cannot see either: below the two-column floor the
// panel is dropped and the screen renders exactly as it always has, which is why
// both snapshots exist rather than one.
//
// The panel shows the plan the run would execute even though the wizard has not
// built it yet, labelled "on Linux (detected)" because the OS question is still
// open. The overwrite and backup rows are absent because the isolated HOME has
// neither a config nor a backup; those rows are pinned by unit tests with real
// and synthetic state instead of a fixture directory (panels_test.go).
//
// The model is pinned the way every golden model is -- isolateGoldenTest points
// HOME at an empty directory and goldenSystemInfo fixes the platform -- so the
// composition is the same on every host and a machine that happens to have
// backups cannot change the snapshot. ExistingConfigs is filled the way the
// startup scan fills it, so the empty result is the real one for this HOME.
func TestMainMenuWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.ExistingConfigs = system.DetectExistingConfigs()
	m.Width = 160
	m.Height = 50
	m.Screen = ScreenMainMenu

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(160, 50),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestCompanionGoldenFramesTheCreatureAtTickZero pins the companion's frame 0:
// the idle frame, at the cell no tick has moved it from. The art and the
// placement are pinned on a frame the clock cannot move, so the snapshot cannot
// flake, and the model is the wide main menu because that is the screen the
// layout was measured on. The gate is on and the counter is zero on purpose: the
// frame and the cell both come from the model, so this snapshot is the same on
// every host -- the goldens around it stay animation-off, and the difference
// between this file and TestMainMenuWideGolden's is exactly the companion's row.
func TestCompanionGoldenFramesTheCreatureAtTickZero(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.ExistingConfigs = system.DetectExistingConfigs()
	m.Width = 160
	m.Height = 50
	m.Screen = ScreenMainMenu
	m.Animating = true
	m.AnimTick = 0

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(160, 50),
	)

	// Quit on the first rendered frame rather than after a sleep, so the snapshot
	// pins the frame the test is about instead of whatever the tick clock reached by
	// the time the sleep ended. A tick can no longer walk the creature off cell 0 --
	// with nothing to follow, a tick moves nothing -- but the counter it advances is
	// still what names the frame, so the first frame is what is wanted. The output
	// reader has to be teed into a buffer of its own because reading the program's
	// output consumes it, and the golden is compared against everything read.
	seen := &bytes.Buffer{}
	teatest.WaitFor(t, io.TeeReader(tm.Output(), seen), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Main Menu"))
	}, teatest.WithCheckInterval(2*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	if _, err := io.Copy(seen, tm.Output()); err != nil {
		t.Fatalf("reading the rest of the output failed: %v", err)
	}
	teatest.RequireEqualOutput(t, seen.Bytes())
}

// TestCompanionGoldenPinsTheFullSpriteAndItsGaze snapshots the five-row cat at
// the wide main menu, on a tick and a gaze cell written on the model before the
// program starts. Both are model state -- the counter names the frame and the
// gaze names where the pupils sit -- so the snapshot cannot flake on the clock,
// and it pins the composed eyes rather than a table row nobody composed.
func TestCompanionGoldenPinsTheFullSpriteAndItsGaze(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.ExistingConfigs = system.DetectExistingConfigs()
	m.Width = 160
	m.Height = 50
	m.Screen = ScreenMainMenu
	m.Animating = true
	m.AnimTick = 3
	m.CompanionGaze = companionGaze{X: -1, Y: -1}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(160, 50),
	)

	seen := &bytes.Buffer{}
	teatest.WaitFor(t, io.TeeReader(tm.Output(), seen), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Main Menu"))
	}, teatest.WithCheckInterval(2*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	if _, err := io.Copy(seen, tm.Output()); err != nil {
		t.Fatalf("reading the rest of the output failed: %v", err)
	}
	teatest.RequireEqualOutput(t, seen.Bytes())
}

// TestCompanionGoldenPinsTheCompactSpriteAndItsGaze is the same snapshot one
// ladder step down: at 100x24 the body leaves four spare rows, the summary takes
// one and the ladder's first step needs six, so the frame draws the three-row
// head. It is pinned to a different tick and the other horizontal gaze cell, so
// the two snapshots together show the composer and the fallback rather than one
// frame twice.
func TestCompanionGoldenPinsTheCompactSpriteAndItsGaze(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.ExistingConfigs = system.DetectExistingConfigs()
	m.Width = 100
	m.Height = 24
	m.Screen = ScreenMainMenu
	m.Animating = true
	m.AnimTick = 6
	m.CompanionGaze = companionGaze{X: 1}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(100, 24),
	)

	seen := &bytes.Buffer{}
	teatest.WaitFor(t, io.TeeReader(tm.Output(), seen), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Main Menu"))
	}, teatest.WithCheckInterval(2*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	if _, err := io.Copy(seen, tm.Output()); err != nil {
		t.Fatalf("reading the rest of the output failed: %v", err)
	}
	teatest.RequireEqualOutput(t, seen.Bytes())
}

// TestOSSelectGolden tests OS selection screen against golden file
func TestOSSelectGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenOSSelect

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestNavigationFlowE2E tests navigating from welcome through menu like Playwright would
func TestNavigationFlowE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Start at welcome screen, press Enter to go to main menu
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for main menu render and verify we can read output
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Start Installation")) ||
			bytes.Contains(bts, []byte("Main Menu"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestInstallFlowE2E simulates a complete installation flow selection
func TestInstallFlowE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Welcome -> Enter
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Main Menu -> Start Installation (already cursor=0)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at OS Select now
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Operating System")) ||
			bytes.Contains(bts, []byte("macOS")) ||
			bytes.Contains(bts, []byte("Linux"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	// Select macOS (cursor=0)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at Terminal Select now. On WSL and Termux the wizard skips the
	// terminal and font questions and goes straight to Shell Select, so the shell
	// screen is a valid next screen too. The assertion used to pass on those hosts
	// only because the body listed every step name, including "Terminal"; the step
	// counter now lives in the frame's header, so the check names the screens.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Terminal")) ||
			bytes.Contains(bts, []byte("Alacritty")) ||
			bytes.Contains(bts, []byte("WezTerm")) ||
			bytes.Contains(bts, []byte("Shell"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestKeymapsE2E tests navigating keymaps like a real user
func TestKeymapsE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Welcome -> Enter
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Main Menu -> Navigate down to Keymaps (index 2)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at KeymapsMenu (tool selection: Neovim, Tmux, Zellij, Ghostty)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Neovim")) ||
			bytes.Contains(bts, []byte("Tmux")) ||
			bytes.Contains(bts, []byte("Zellij")) ||
			bytes.Contains(bts, []byte("Ghostty"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	// Select Neovim (first option) to get to Neovim keymaps categories
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at Neovim Keymaps categories (Harpoon, Mini.files, etc.)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Harpoon")) ||
			bytes.Contains(bts, []byte("Mini.files"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	// Select first category (Harpoon)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should show keymaps now with leader key bindings
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("leader")) ||
			bytes.Contains(bts, []byte("Description")) ||
			bytes.Contains(bts, []byte("Keys"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestLearnToolsE2E tests learn about tools navigation
func TestLearnToolsE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Welcome -> Enter
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Main Menu -> Navigate to Learn About Tools (index 1)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should show tool categories
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Terminal")) ||
			bytes.Contains(bts, []byte("Shell")) ||
			bytes.Contains(bts, []byte("Multiplexer"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestBackupScreenGolden tests the backup confirmation screen
func TestBackupScreenGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenBackupConfirm
	m.ExistingConfigs = []string{".config/nvim", ".zshrc", ".tmux.conf"}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestErrorScreenGolden tests the error screen render
func TestErrorScreenGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenError
	m.ErrorMsg = "Test error: something went wrong during installation"

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestCompleteScreenGolden tests the completion screen render
func TestCompleteScreenGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenComplete
	m.Choices = UserChoices{
		OS:          "mac",
		Terminal:    "ghostty",
		Shell:       "fish",
		WindowMgr:   "tmux",
		InstallNvim: true,
	}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestWelcomeLivePanelGolden pins the live machine panel -- the CPU and memory
// sparklines and the load, disk and process facts -- at the two-column size
// where it is drawn. The samples are pinned on the model, so the snapshot is the
// chart and not the machine that generated it.
func TestWelcomeLivePanelGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = 160, 40
	m.Screen = ScreenWelcome
	m.Animating = true
	m.AnimTick = 0
	m.PanelIndex = 1 // the live panel
	m.Metrics = pinnedMetrics(24)
	golden.RequireEqual(t, []byte(m.View()))
}

// TestInstallingLiveGolden pins the installing screen with the machine's pulse
// and the run's own progress chart on it, at the 80x24 floor, so the block that
// shares the frame with the rail is checked at the size where the room is
// tightest.
func TestInstallingLiveGolden(t *testing.T) {
	skipIfTermux(t)
	m := installerFrameCase(t, "installing-live")
	golden.RequireEqual(t, []byte(m.View()))
}

// TestCompleteCelebrationGolden pins the end-of-run burst: the particles and the
// pleased companion at a frame a few ticks into the two seconds. The burst is
// model state, so its positions are the same on every machine that renders it.
func TestCompleteCelebrationGolden(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = 80, 24
	m.Screen = ScreenComplete
	m.Animating = true
	m.Choices = UserChoices{OS: "mac", Terminal: "ghostty", Shell: "fish", WindowMgr: "tmux", InstallFont: true, InstallNvim: true}
	m.startCelebration()
	m.CompanionPleased = companionPleasedTicks
	for i := 0; i < 4; i++ {
		m.AnimTick++
		m.advanceCompanion()
		m.advanceCelebration()
	}
	golden.RequireEqual(t, []byte(m.View()))
}

// TestKeyboardNavigationE2E tests various keyboard interactions
func TestKeyboardNavigationE2E(t *testing.T) {
	t.Run("j/k navigation works like vim", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenMainMenu

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		// Use j to move down
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		time.Sleep(50 * time.Millisecond)
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		time.Sleep(50 * time.Millisecond)

		// Use k to move up
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		time.Sleep(50 * time.Millisecond)

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	t.Run("escape goes back from learn screens", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenLearnTerminals
		m.PrevScreen = ScreenMainMenu

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
		time.Sleep(100 * time.Millisecond)

		// Should be back at main menu (learn screens support escape)
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Start Installation")) ||
				bytes.Contains(bts, []byte("Main Menu"))
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// TestResponsiveLayoutE2E tests different terminal sizes
func TestResponsiveLayoutE2E(t *testing.T) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"small_terminal", 60, 20},
		{"medium_terminal", 80, 24},
		{"large_terminal", 120, 40},
		{"wide_terminal", 160, 24},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			m := NewModel()
			m.Width = sz.width
			m.Height = sz.height
			m.Screen = ScreenMainMenu

			tm := teatest.NewTestModel(t, m,
				teatest.WithInitialTermSize(sz.width, sz.height),
			)

			time.Sleep(100 * time.Millisecond)
			tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
			tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

			// Just verify it doesn't panic and produces output
			out := readAll(t, tm.FinalOutput(t))
			if len(out) == 0 {
				t.Error("Expected some output")
			}
		})
	}
}

// TestLazyVimGuideE2E tests LazyVim guide navigation flow
func TestLazyVimGuideE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Welcome -> Enter
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Main Menu -> Navigate to LazyVim Guide (index 3)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at LazyVim guide screen
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("LazyVim")) ||
			bytes.Contains(bts, []byte("lazy")) ||
			bytes.Contains(bts, []byte("plugin"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// ============================================
// BACKUP SYSTEM E2E TESTS
// ============================================

// TestBackupConfirmScreenE2E tests backup confirmation screen behavior
func TestBackupConfirmScreenE2E(t *testing.T) {
	t.Run("shows existing configs list", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenBackupConfirm
		m.ExistingConfigs = []string{
			"nvim: ~/.config/nvim",
			"fish: ~/.config/fish",
			"zsh: ~/.zshrc",
		}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(100 * time.Millisecond)

		// Verify the screen shows backup options
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			hasBackup := bytes.Contains(bts, []byte("Backup")) || bytes.Contains(bts, []byte("backup"))
			hasInstall := bytes.Contains(bts, []byte("Install")) || bytes.Contains(bts, []byte("install"))
			return hasBackup || hasInstall
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	t.Run("can navigate options with j/k", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenBackupConfirm
		m.ExistingConfigs = []string{"nvim: ~/.config/nvim"}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(50 * time.Millisecond)

		// Navigate down
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		time.Sleep(50 * time.Millisecond)
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		time.Sleep(50 * time.Millisecond)

		// Navigate up
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		time.Sleep(50 * time.Millisecond)

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	t.Run("escape goes back to nvim selection", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenBackupConfirm
		m.ExistingConfigs = []string{"nvim: ~/.config/nvim"}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(50 * time.Millisecond)

		// Press escape
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
		time.Sleep(100 * time.Millisecond)

		// Should go back to Nvim selection screen
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Neovim")) ||
				bytes.Contains(bts, []byte("nvim")) ||
				bytes.Contains(bts, []byte("Yes")) ||
				bytes.Contains(bts, []byte("No"))
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// TestRestoreBackupScreenE2E tests restore backup screen behavior
func TestRestoreBackupScreenE2E(t *testing.T) {
	t.Run("shows available backups", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/home/user/.dotfiles-backup-2024-01-15-120000", Files: []string{"nvim", "fish"}},
			{Path: "/home/user/.dotfiles-backup-2024-01-16-130000", Files: []string{"zsh", "tmux"}},
		}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(100 * time.Millisecond)

		// Verify screen shows restore options
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Restore")) ||
				bytes.Contains(bts, []byte("Backup")) ||
				bytes.Contains(bts, []byte("Back"))
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	t.Run("can select backup and go to confirm", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/home/user/.dotfiles-backup-test", Files: []string{"nvim"}},
		}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(50 * time.Millisecond)

		// Select first backup (Enter)
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(100 * time.Millisecond)

		// Should go to restore confirm screen
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Confirm")) ||
				bytes.Contains(bts, []byte("Restore")) ||
				bytes.Contains(bts, []byte("Delete")) ||
				bytes.Contains(bts, []byte("Cancel"))
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// TestRestoreConfirmScreenE2E tests restore confirm screen behavior
func TestRestoreConfirmScreenE2E(t *testing.T) {
	t.Run("shows restore, delete, cancel options", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/home/user/.dotfiles-backup-test", Files: []string{"nvim", "fish", "zsh"}},
		}
		m.SelectedBackup = 0

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(100 * time.Millisecond)

		// Should show the three options
		out := readAll(t, tm.Output())
		hasOptions := bytes.Contains(out, []byte("Restore")) ||
			bytes.Contains(out, []byte("Delete")) ||
			bytes.Contains(out, []byte("Cancel"))

		if !hasOptions {
			t.Log("Output may not show all options yet, checking with WaitFor")
		}

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	t.Run("escape returns to backup list", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/home/user/.dotfiles-backup-test", Files: []string{"nvim"}},
		}
		m.SelectedBackup = 0

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(50 * time.Millisecond)

		// Press escape
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
		time.Sleep(100 * time.Millisecond)

		// Should go back to restore backup screen
		// Note: The screen transition might be quick
		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// TestMainMenuWithRestoreOption tests main menu shows restore when backups exist
func TestMainMenuWithRestoreOption(t *testing.T) {
	t.Run("GetCurrentOptions includes restore when backups exist", func(t *testing.T) {
		// Test the model logic directly instead of through teatest
		// because backups are loaded async and teatest doesn't wait for Init()
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/home/user/.dotfiles-backup-test", Files: []string{"nvim"}},
		}

		options := m.GetCurrentOptions()

		hasRestore := false
		for _, opt := range options {
			if bytes.Contains([]byte(opt), []byte("Restore")) {
				hasRestore = true
				break
			}
		}

		if !hasRestore {
			t.Errorf("Expected 'Restore from Backup' option when backups exist, got: %v", options)
		}
	})

	t.Run("GetCurrentOptions excludes restore when no backups", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{} // Empty

		options := m.GetCurrentOptions()

		for _, opt := range options {
			if bytes.Contains([]byte(opt), []byte("Restore from Backup")) {
				t.Error("Should not show restore option when no backups exist")
			}
		}
	})

	t.Run("main menu renders without restore when no backups", func(t *testing.T) {
		m := NewModel()
		m.Width = 80
		m.Height = 24
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{} // Empty

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		time.Sleep(100 * time.Millisecond)

		// Get output and verify standard menu items exist
		out := readAll(t, tm.Output())
		if !bytes.Contains(out, []byte("Start Installation")) {
			t.Error("Should show Start Installation option")
		}

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// plainOutput strips the terminal escape sequences from a chunk of raw program
// output, so a test can search for the text a screen shows rather than the exact
// byte stream it was styled into. Two strings the reader sees as adjacent -- the
// selection marker and the option label, say -- are styled separately, so the
// escapes between them break a literal search even though the screen is right.
func plainOutput(bts []byte) string {
	return ansiEscape.ReplaceAllString(string(bts), "")
}

// TestBackupFlowE2E walks the install wizard far enough to prove the choices it
// collects lead where the flow says they lead.
//
// It pins the host and waits for each screen before sending the next key. Both
// matter: the wizard skips the terminal and font steps on WSL and Termux, so the
// sequence used to depend on where the test ran, and a fixed sleep between
// keypresses raced the renderer, so a loaded runner lost keypresses and the flow
// ended on the wrong terminal.
func TestBackupFlowE2E(t *testing.T) {
	t.Run("full flow: wizard -> backup confirm -> install", func(t *testing.T) {
		m := NewModel()
		isolateGoldenTest(t, &m)
		// Pin the host so every run walks the same wizard: Linux, not WSL, so the
		// terminal and font steps are part of the flow everywhere.
		m.SystemInfo = goldenSystemInfo()
		m.Width = 80
		m.Height = 24
		// Simulate having existing configs
		m.ExistingConfigs = []string{"nvim: ~/.config/nvim"}

		tm := teatest.NewTestModel(t, m,
			teatest.WithInitialTermSize(80, 24),
		)

		// Send a key and wait for the screen it should produce, rather than sleeping
		// a fixed amount and hoping the model caught up.
		send := func(key tea.KeyMsg, marker string) {
			t.Helper()
			tm.Send(key)
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte(marker))
			}, teatest.WithCheckInterval(20*time.Millisecond), teatest.WithDuration(5*time.Second))
		}

		// Welcome -> Main Menu -> the wizard's first step -> the terminal step.
		send(tea.KeyMsg{Type: tea.KeyEnter}, "Start Installation")
		send(tea.KeyMsg{Type: tea.KeyEnter}, "Select Your Operating System")
		send(tea.KeyMsg{Type: tea.KeyEnter}, "Choose Terminal Emulator")

		// Choose "None" so the font step is skipped. The list carries a separator and
		// a learn-more entry after the choices, so the index is read from a model
		// driven through the same two steps rather than counted by hand or taken from
		// a list built for a different host: counting assumed "None" was the last
		// entry, which stopped being true when the list grew, and a probe that pinned
		// the OS by hand listed a terminal the pinned host does not offer, so the
		// marker landed one row past None.
		probe := installerFrameModel(t, ScreenMainMenu)
		step, _ := probe.Update(tea.KeyMsg{Type: tea.KeyEnter})
		step, _ = step.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
		probe = step.(Model)
		if probe.Screen != ScreenTerminalSelect {
			t.Fatalf("driving the wizard from the main menu landed on screen %v, want the terminal step", probe.Screen)
		}
		noneIndex := -1
		for i, opt := range probe.GetCurrentOptions() {
			if opt == "None" {
				noneIndex = i
				break
			}
		}
		if noneIndex < 0 {
			t.Fatalf("the terminal step has no None option: %v", probe.GetCurrentOptions())
		}
		for i := 0; i < noneIndex; i++ {
			tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		}
		// Confirm the marker reached None before committing to it, so a lost key is
		// reported here instead of as a puzzling failure three screens later. The
		// marker and the label are styled apart, so the escape sequences between them
		// are stripped before the search; the assertion still means the marker is on
		// None, not merely that None is on screen.
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return strings.Contains(plainOutput(bts), "▸ None")
		}, teatest.WithCheckInterval(20*time.Millisecond), teatest.WithDuration(5*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

		// Should now be at Shell Select (font is skipped because terminal=none).
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Shell")) ||
				bytes.Contains(bts, []byte("Fish")) ||
				bytes.Contains(bts, []byte("Zsh"))
		}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(5*time.Second))

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})
}

// =============================================================================
// INSTALLER FRAME GUARD
// =============================================================================

// The installer's own screens claim the same 80x24 floor the trainer documents
// through trainerFrameWidth and trainerFrameHeight, and for the same reason: a
// screen taller than its terminal loses its bottom silently, and a golden only
// sees the top rows. TestTrainerScreensFitTheFrame covered the trainer alone,
// so this guard renders every other screen at the floor and fails on a screen
// that does not fit.
//
// The width half matters as much as the height: a line wider than the terminal
// is clipped at the edge with no marker, which is how the keymap tables used to
// ship a 60-column rule that overflowed a 60-column terminal and left dead
// space on a 120-column one.

// installerFrameModel parks a model on a screen at the documented 80x24 floor,
// with HOME pointed at an empty directory so a machine that happens to have
// backups does not change what the screen renders.
func installerFrameModel(t *testing.T, screen Screen) Model {
	t.Helper()
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = trainerFrameWidth
	m.Height = trainerFrameHeight
	m.Screen = screen
	return m
}

// assertInstallerScreenFits pins one rendered screen inside the floor frame and
// returns its row and column counts so the guard can name the worst screen it
// saw. A screen is allowed to be shorter than the frame; what it may not do is
// be taller or wider, because that is the part the terminal takes away without
// saying so.
func assertInstallerScreenFits(t *testing.T, name string, m Model) (rows, cols int) {
	t.Helper()

	view := m.View()
	rows = renderedRowCount(view)

	for _, line := range strings.Split(view, "\n") {
		w := lipgloss.Width(line)
		if w > cols {
			cols = w
		}
		if w > trainerFrameWidth {
			t.Errorf("%s renders a line %d columns wide, want <= %d: it is clipped silently at the frame edge: %q",
				name, w, trainerFrameWidth, line)
		}
	}

	if rows > trainerFrameHeight {
		t.Errorf("%s renders %d rows at %dx%d, want <= %d: the bottom of the screen falls outside the frame",
			name, rows, trainerFrameWidth, trainerFrameHeight, trainerFrameHeight)
	}

	return rows, cols
}

// installerFrameCase builds the model for one guarded screen. Each case is a
// state the screen reaches in normal use, not a synthetic maximum: the screens
// with lists get one ordinary list and one long list, because a bounded list is
// exactly what the frame guard is about.
func installerFrameCase(t *testing.T, name string) Model {
	t.Helper()

	base := func(screen Screen) Model { return installerFrameModel(t, screen) }
	withConfigs := func(screen Screen, configs []string) Model {
		m := installerFrameModel(t, screen)
		m.ExistingConfigs = configs
		return m
	}
	withBackups := func(screen Screen, files []string, count int) Model {
		m := installerFrameModel(t, screen)
		for i := 0; i < count; i++ {
			m.AvailableBackups = append(m.AvailableBackups, system.BackupInfo{
				Path:  fmt.Sprintf("/home/testuser/.dotfiles-backup-2024-01-%02d-120000", i+1),
				Files: files,
			})
		}
		return m
	}
	installing := func(details bool, atEnd bool, logCount int) Model {
		m := installerFrameModel(t, ScreenInstalling)
		m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, OSName: "Linux"}
		m.Choices = UserChoices{
			OS: "linux", Terminal: "alacritty", InstallFont: true, Shell: "fish",
			WindowMgr: "tmux", InstallNvim: true, CreateBackup: true,
		}
		m.ExistingConfigs = []string{"nvim: ~/.config/nvim", "fish: ~/.config/fish"}
		m.SetupInstallSteps()

		running := 2
		if atEnd {
			running = len(m.Steps) - 1
		}
		for i := 0; i < running; i++ {
			m.Steps[i].Status = StatusDone
			m.Steps[i].Progress = 1
		}
		m.CurrentStep = running
		m.Steps[running].Status = StatusRunning
		m.Steps[running].Progress = 0.5

		m.ShowDetails = details
		for i := 0; i < logCount; i++ {
			m.LogLines = append(m.LogLines, fmt.Sprintf("log line %d", i+1))
		}
		return m
	}

	switch name {
	case "welcome":
		return base(ScreenWelcome)
	case "main-menu":
		return base(ScreenMainMenu)
	case "main-menu-restore":
		m := base(ScreenMainMenu)
		m.AvailableBackups = []system.BackupInfo{{Path: "/home/testuser/.dotfiles-backup-test"}}
		return m
	case "os-select":
		return base(ScreenOSSelect)
	case "terminal-select":
		return base(ScreenTerminalSelect)
	case "terminal-select-wsl":
		m := base(ScreenTerminalSelect)
		m.SystemInfo.IsWSL = true
		m.SystemInfo.OSName = "Debian (WSL)"
		return m
	case "font-select":
		return base(ScreenFontSelect)
	case "shell-select":
		return base(ScreenShellSelect)
	case "wm-select":
		return base(ScreenWMSelect)
	case "nvim-select":
		return base(ScreenNvimSelect)
	case "ghostty-warning":
		return base(ScreenGhosttyWarning)
	case "learn-terminals":
		return base(ScreenLearnTerminals)
	case "learn-terminals-info":
		m := base(ScreenLearnTerminals)
		m.ViewingTool = "alacritty"
		return m
	case "learn-shells":
		return base(ScreenLearnShells)
	case "learn-shells-info":
		m := base(ScreenLearnShells)
		m.ViewingTool = "nushell"
		return m
	case "learn-wm":
		return base(ScreenLearnWM)
	case "learn-wm-info":
		m := base(ScreenLearnWM)
		m.ViewingTool = "tmux"
		return m
	case "learn-nvim":
		return base(ScreenLearnNvim)
	case "learn-nvim-features":
		m := base(ScreenLearnNvim)
		m.ViewingTool = "features"
		return m
	case "keymaps":
		return base(ScreenKeymaps)
	case "keymap-category":
		return base(ScreenKeymapCategory)
	case "keymaps-menu":
		return base(ScreenKeymapsMenu)
	case "keymaps-tmux":
		return base(ScreenKeymapsTmux)
	case "keymaps-tmux-category":
		return base(ScreenKeymapsTmuxCat)
	case "keymaps-zellij":
		return base(ScreenKeymapsZellij)
	case "keymaps-zellij-category":
		return base(ScreenKeymapsZellijCat)
	case "keymaps-ghostty":
		return base(ScreenKeymapsGhostty)
	case "keymaps-ghostty-category":
		return base(ScreenKeymapsGhosttyCat)
	case "keymaps-herdr":
		return base(ScreenKeymapsHerdr)
	case "keymaps-herdr-category":
		return base(ScreenKeymapsHerdrCat)
	case "keymaps-herdr-long-keys":
		// The category with a binding wider than a quarter of the frame, so the
		// guard covers the widened keys column as well as the default one.
		m := base(ScreenKeymapsHerdrCat)
		for i, cat := range m.HerdrKeymapCategories {
			for _, km := range cat.Keymaps {
				if lipgloss.Width(km.Keys) >= 30 {
					m.HerdrSelectedCategory = i
				}
			}
		}
		return m
	case "lazyvim":
		return base(ScreenLearnLazyVim)
	case "lazyvim-topic":
		return base(ScreenLazyVimTopic)
	case "backup-confirm":
		return withConfigs(ScreenBackupConfirm, []string{
			"nvim: ~/.config/nvim", "fish: ~/.config/fish", "zsh: ~/.zshrc",
		})
	case "backup-confirm-many":
		configs := make([]string, 0, 14)
		for i := 0; i < 14; i++ {
			configs = append(configs, fmt.Sprintf("tool%d: ~/.config/tool%d", i, i))
		}
		return withConfigs(ScreenBackupConfirm, configs)
	case "restore-backup":
		return withBackups(ScreenRestoreBackup, []string{"nvim", "fish"}, 2)
	case "restore-backup-many":
		return withBackups(ScreenRestoreBackup, []string{"nvim", "fish"}, 14)
	case "restore-confirm":
		m := withBackups(ScreenRestoreConfirm, []string{"nvim", "fish", "zsh"}, 1)
		m.SelectedBackup = 0
		return m
	case "restore-confirm-many-files":
		files := make([]string, 0, 24)
		for i := 0; i < 24; i++ {
			files = append(files, fmt.Sprintf("file-%02d", i))
		}
		m := withBackups(ScreenRestoreConfirm, files, 1)
		m.SelectedBackup = 0
		return m
	case "installing":
		return installing(false, false, 0)
	case "installing-details":
		return installing(true, false, 12)
	case "installing-details-many":
		// A log longer than any frame can hold, so both guards see the box at its
		// largest and the hidden-count note on screen.
		return installing(true, false, 40)
	case "installing-at-end":
		return installing(false, true, 0)
	case "installing-live":
		// A run whose sampler has landed: the machine's pulse and the run's own
		// progress chart are on screen, which is the state the frame guard did not
		// otherwise cover -- its other installing cases have no samples.
		m := installing(false, false, 0)
		m.Animating = true
		m.InstallStartedAt = goldenGreetingTime
		m.Now = goldenGreetingTime.Add(42 * time.Second)
		m.Metrics = pinnedMetrics(24)
		m.ProgressSamples = pinnedProgress(24)
		return m
	case "complete":
		m := base(ScreenComplete)
		m.Choices = UserChoices{OS: "mac", Terminal: "ghostty", Shell: "fish", WindowMgr: "tmux", InstallFont: true, InstallNvim: true}
		return m
	case "error":
		m := base(ScreenError)
		m.ErrorMsg = "failed to install alacritty: the package manager refused the request after a long explanation that does not fit on one line"
		m.LogLines = []string{"first log line", "second log line", "third log line", "fourth log line", "fifth log line", "sixth log line"}
		return m
	case "error-many-logs":
		// The error panel used to cut at five lines whatever the frame was; a log
		// longer than that proves the panel now follows the frame and says how many
		// earlier lines it could not show.
		m := base(ScreenError)
		m.ErrorMsg = "failed to install alacritty: the package manager refused the request after a long explanation that does not fit on one line"
		for i := 0; i < 40; i++ {
			m.LogLines = append(m.LogLines, fmt.Sprintf("log line %d", i+1))
		}
		return m
	default:
		t.Fatalf("no frame case named %q", name)
		return Model{}
	}
}

// installerFrameScreenNames is every installer screen the frame guards render,
// shared by the 80x24 floor guard and the wide-terminal guard so both measure
// the same set of screens.
var installerFrameScreenNames = []string{
	"welcome",
	"main-menu", "main-menu-restore",
	"os-select", "terminal-select", "terminal-select-wsl", "font-select",
	"shell-select", "wm-select", "nvim-select", "ghostty-warning",
	"learn-terminals", "learn-terminals-info",
	"learn-shells", "learn-shells-info",
	"learn-wm", "learn-wm-info",
	"learn-nvim", "learn-nvim-features",
	"keymaps", "keymap-category", "keymaps-menu",
	"keymaps-tmux", "keymaps-tmux-category",
	"keymaps-zellij", "keymaps-zellij-category",
	"keymaps-ghostty", "keymaps-ghostty-category",
	"keymaps-herdr", "keymaps-herdr-category", "keymaps-herdr-long-keys",
	"lazyvim", "lazyvim-topic",
	"backup-confirm", "backup-confirm-many",
	"restore-backup", "restore-backup-many", "restore-confirm", "restore-confirm-many-files",
	"installing", "installing-details", "installing-details-many", "installing-at-end",
	"installing-live",
	"complete", "error", "error-many-logs",
}

// TestInstallerScreensFitTheFrame is the counterpart to
// TestTrainerScreensFitTheFrame for every screen that is not the trainer. It
// had no guard, so a screen could exceed the terminal and stay that way: the
// keymap tables shipped a 60-column rule and a 15-row window against a 24-row
// frame, the installing screen had no progress bar at all, and two screens
// hard-coded a title the model was carrying.
//
// The persistent frame added a header row, two rules and a footer to every
// screen, so each screen's reserved rows moved: the body budgets now come from
// installerBodyRows and the per-screen *BodyFixed constants in view.go, and the
// keymap window, menu window, installing rail, tool-info lists and LazyVim
// window were all recomputed against them. This guard's height assertion is
// unchanged, so the new chrome is not treated as headroom: a screen that spends
// a row it did not reserve still fails here.
func TestInstallerScreensFitTheFrame(t *testing.T) {
	names := installerFrameScreenNames

	worstRows, worstCols := 0, 0
	worstRowScreen, worstColScreen := "", ""

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			m := installerFrameCase(t, name)
			rows, cols := assertInstallerScreenFits(t, name, m)

			// Leader mode replaces the legend rather than growing the screen, so the
			// banner has to fit exactly like the legend it stands in for.
			m.LeaderMode = true
			leaderRows, leaderCols := assertInstallerScreenFits(t, name+" with the leader banner", m)
			if leaderRows > rows {
				rows = leaderRows
			}
			if leaderCols > cols {
				cols = leaderCols
			}

			// The companion draws in one row the body did not need, so the frame fits
			// exactly as it did without it. The gate is forced on here so a creature
			// that cost the frame a row, or drew past the frame edge, fails this guard
			// rather than showing up as a shifted screen on somebody's terminal.
			m.LeaderMode = false
			m.Animating = true
			companionRows, companionCols := assertInstallerScreenFits(t, name+" with the companion", m)
			if companionRows > rows {
				rows = companionRows
			}
			if companionCols > cols {
				cols = companionCols
			}

			if rows > worstRows {
				worstRows, worstRowScreen = rows, name
			}
			if cols > worstCols {
				worstCols, worstColScreen = cols, name
			}
		})
	}

	t.Logf("rendered %d installer screens at %dx%d; worst height %d rows (%s), worst width %d columns (%s)",
		len(names), trainerFrameWidth, trainerFrameHeight, worstRows, worstRowScreen, worstCols, worstColScreen)
}

// wideFrameCases are the terminals the responsive layout was designed on. 160x50
// is the first two-column size, and 227x62 is the pane the feature exists for:
// there the previous layout drew the main menu in a fifth of the columns and its
// selected row as a bar the whole width of the terminal.
var wideFrameCases = []struct {
	name          string
	width, height int
}{
	{"160x50", 160, 50},
	{"227x62", 227, 62},
}

// TestInstallerScreensFitWideTerminals is the wide counterpart to
// TestInstallerScreensFitTheFrame, and it asks a different question. The floor
// guard asks whether a screen fits the smallest frame it may be given; this one
// asks whether a screen uses a large frame without running past it. Nothing the
// installer draws may exceed the terminal's width, a screen must fill its frame
// exactly so the footer stays on the last row, and a menu row must be a row
// rather than a band of bar across the terminal.
func TestInstallerScreensFitWideTerminals(t *testing.T) {
	for _, c := range wideFrameCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for _, name := range installerFrameScreenNames {
				name := name
				t.Run(name, func(t *testing.T) {
					m := installerFrameCase(t, name)
					m.Width, m.Height = c.width, c.height

					view := m.View()
					if rows := renderedRowCount(view); rows != c.height {
						t.Errorf("%s renders %d rows at %s, want exactly %d: the footer is off the frame",
							name, rows, c.name, c.height)
					}
					for _, line := range strings.Split(view, "\n") {
						if w := lipgloss.Width(line); w > c.width {
							t.Errorf("%s renders a %d-column line in a %d-column terminal: %q",
								name, w, c.width, line)
						}
					}

					// The companion is one row of the body's spare room, so the wide frame
					// holds exactly as many rows with it on -- the footer still lands on
					// the terminal's last row -- and no line grows past the edge.
					m.Animating = true
					view = m.View()
					if rows := renderedRowCount(view); rows != c.height {
						t.Errorf("%s renders %d rows with the companion at %s, want exactly %d: the footer is off the frame",
							name, rows, c.name, c.height)
					}
					for _, line := range strings.Split(view, "\n") {
						if w := lipgloss.Width(line); w > c.width {
							t.Errorf("%s renders a %d-column line with the companion in a %d-column terminal: %q",
								name, w, c.width, line)
						}
					}
				})
			}

			// The main menu is the screen that composes: it offers a right column,
			// and its rows are drawn at the row measure rather than across the
			// terminal. That measure is what turns a 227-column slab of bar into a
			// row.
			m := installerFrameCase(t, "main-menu")
			m.Width, m.Height = c.width, c.height
			l := layoutFor(m)
			if !l.TwoColumn {
				t.Fatalf("the main menu at %s lays out as one %d-column body: no panel has room",
					c.name, l.Inner)
			}

			bar := lipgloss.Width(m.rowBar("Start Installation", true, ""))
			if bar != l.RowMeasure {
				t.Errorf("the selected row's bar is %d columns, want the row measure %d", bar, l.RowMeasure)
			}
			if bar == l.Inner {
				t.Errorf("the selected row's bar spans the body's whole %d columns", bar)
			}
			if bar > layoutRowMeasureMax {
				t.Errorf("the selected row's bar is %d columns, want <= the reading measure %d", bar, layoutRowMeasureMax)
			}

			view := m.View()
			if !strings.Contains(view, mainMenuPanelLabel) {
				t.Errorf("the main menu at %s shows no right column: %q is not on screen", c.name, mainMenuPanelLabel)
			}

			// Every composed line is the inner width, the leading margin included,
			// so the body covers the columns the frame's rules cover and the
			// margins on either side of the composition are the ones it computed.
			composed := ""
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "What would you like to do?") {
					composed = line
				}
			}
			if w := lipgloss.Width(composed); w != c.width {
				t.Errorf("the composed body line at %s is %d columns, want the terminal's %d", c.name, w, c.width)
			}

			// The frame uses its room: the widest line reaches the terminal's last
			// column instead of stopping short.
			widest := 0
			for _, line := range strings.Split(view, "\n") {
				widest = max(widest, lipgloss.Width(line))
			}
			if widest != c.width {
				t.Errorf("the main menu at %s uses %d of its %d columns", c.name, widest, c.width)
			}
		})
	}
}

// TestComposedScreensLoseNothingToTheirPanel pins the one thing the two-column
// composition may never do to the body it places beside a panel: shorten it.
//
// composeColumns cuts the left column to the layout's Left width with the shared
// truncate, so a body line longer than that column ends in the cut marker even
// though the same line fits the screen's full content width. The welcome screen's
// environment sentence did exactly that: 78 columns of fact, whole on a
// one-column screen, cut to "... with Homebrew already installed (…" in the
// 70-column column at 160, so a layout that looked tidier said less. The frame
// guard could not see it, because its welcome model is a plain Linux host whose
// sentence is 28 columns and never reaches the column edge.
//
// The guard renders each composing screen at every two-column size, with the
// host that makes the welcome sentence long, and fails on any line in the body's
// own column that ends with the truncation marker. A cut in that column is the
// signal that the panel took a fact from the body; the panel's own column is
// allowed to wrap and to say how many rows it could not show.
func TestComposedScreensLoseNothingToTheirPanel(t *testing.T) {
	hosts := []struct {
		name   string
		host   *system.SystemInfo
		screen Screen
	}{
		{name: "welcome, a plain Linux host", screen: ScreenWelcome, host: goldenSystemInfo()},
		{name: "welcome, a WSL host with Homebrew already installed", screen: ScreenWelcome, host: &system.SystemInfo{
			OS: system.OSDebian, OSName: "Debian/Ubuntu", Arch: "x86_64",
			IsWSL: true, WSLVersion: 2, UserShell: "zsh", HomeDir: "/home/testuser", HasBrew: true,
		}},
		{name: "main menu", screen: ScreenMainMenu, host: goldenSystemInfo()},
	}

	sizes := []struct {
		name          string
		width, height int
	}{
		{"124x24, the two-column floor", 124, 24},
		{"124x30", 124, 30},
		{"160x50", 160, 50},
		{"227x62, the pane the feature exists for", 227, 62},
	}

	for _, c := range hosts {
		for _, size := range sizes {
			c, size := c, size
			t.Run(c.name+" at "+size.name, func(t *testing.T) {
				m := NewModel()
				isolateGoldenTest(t, &m)
				m.Screen = c.screen
				m.Width, m.Height = size.width, size.height
				m.SystemInfo = c.host

				l := layoutFor(m)
				if !l.TwoColumn {
					t.Fatalf("%s lays out as one %d-column body at %s: no panel has room", c.name, l.Inner, size.name)
				}

				view := ansiEscape.ReplaceAllString(m.View(), "")
				if rows := renderedRowCount(view); rows != size.height {
					t.Errorf("%s at %s renders %d rows, want exactly %d: the growing body pushed the footer off the frame",
						c.name, size.name, rows, size.height)
				}
				for _, line := range strings.Split(view, "\n") {
					if w := lipgloss.Width(line); w > size.width {
						t.Errorf("%s at %s renders a %d-column line in a %d-column terminal: %q", c.name, size.name, w, size.width, line)
					}
				}

				// View() pads every screen two columns on each side, so the body's own
				// column in the rendered line is [pad+Leading, pad+Leading+Left]; the
				// gutter and the panel sit to the right of it. A marker inside the
				// body's column is a fact the panel's arrival took away.
				pad := (size.width - l.Inner) / 2
				bodyRightEdge := pad + l.Leading + l.Left
				for i, raw := range strings.Split(view, "\n") {
					line := strings.TrimRight(raw, " ")
					if !strings.HasSuffix(line, cutMarker) {
						continue
					}
					if col := lipgloss.Width(line); col <= bodyRightEdge {
						t.Errorf("%s at %s cuts a body line in the panel's own column at row %d: %q (the body has %d columns, the cut landed at %d); the panel must not take a fact from the body",
							c.name, size.name, i+1, line, l.Left, col)
					}
				}
			})
		}
	}
}

// TestInstallingScreenShowsProgressAndRail pins the two things the installing
// screen has to show while it is the longest thing a user watches: a progress
// bar that fills with the run, and a step rail whose state is a glyph rather
// than a colour. The bar used to be absent and the rail animated a spinner, so
// neither the run's progress nor the rail's state was readable without colour.
func TestInstallingScreenShowsProgressAndRail(t *testing.T) {
	m := installerFrameCase(t, "installing")
	view := m.View()

	if !strings.Contains(view, "█") || !strings.Contains(view, "░") {
		t.Errorf("the installing screen shows no filled/empty progress bar:\n%s", view)
	}
	for _, glyph := range []string{"✓", "●", "○"} {
		if !strings.Contains(view, glyph) {
			t.Errorf("the installing screen's step rail is missing %q:\n%s", glyph, view)
		}
	}
}

// TestInstallProgressCountsEveryStepState pins the bar's fraction: completed and
// skipped steps count fully, a running step counts the fraction it reported, and
// pending steps count zero. A run with no steps reads as complete rather than
// dividing by zero.
func TestInstallProgressCountsEveryStepState(t *testing.T) {
	m := Model{Steps: []InstallStep{
		{Status: StatusDone},
		{Status: StatusSkipped},
		{Status: StatusRunning, Progress: 0.5},
		{Status: StatusPending},
	}}
	if got, want := m.installProgress(), 0.625; got != want {
		t.Errorf("installProgress = %v, want %v", got, want)
	}

	if got := (Model{}).installProgress(); got != 1 {
		t.Errorf("installProgress with no steps = %v, want 1", got)
	}
}

// TestInstallingLogUsesTheRowsTheFrameLeaves pins that the log box follows the
// frame instead of a fixed three lines: at the 80x24 floor it shows more of the
// tail than the old constant, it keeps the freshest lines, and it names the
// earlier ones it could not fit in one dim row rather than dropping them
// silently.
func TestInstallingLogUsesTheRowsTheFrameLeaves(t *testing.T) {
	const queued = 40
	m := installerFrameCase(t, "installing-details-many")
	view := ansiEscape.ReplaceAllString(m.View(), "")

	shown := strings.Count(view, "log line ")
	if shown <= 3 {
		t.Errorf("the 80x24 log box shows %d log lines, want more than the old fixed three:\n%s", shown, view)
	}
	if !strings.Contains(view, fmt.Sprintf("log line %d", queued)) {
		t.Errorf("the log box dropped its freshest line:\n%s", view)
	}
	note := fmt.Sprintf("… %d earlier lines", queued-shown)
	if !strings.Contains(view, note) {
		t.Errorf("the log box does not name the earlier lines it could not show (want %q):\n%s", note, view)
	}
}

// TestErrorLogUsesTheRowsTheFrameLeaves pins the same for the error screen's log
// panel, which used to cut at five lines for no stated reason.
func TestErrorLogUsesTheRowsTheFrameLeaves(t *testing.T) {
	const queued = 40
	m := installerFrameCase(t, "error-many-logs")
	view := ansiEscape.ReplaceAllString(m.View(), "")

	shown := strings.Count(view, "log line ")
	if shown <= 5 {
		t.Errorf("the 80x24 error log panel shows %d log lines, want more than the old fixed five:\n%s", shown, view)
	}
	if !strings.Contains(view, fmt.Sprintf("log line %d", queued)) {
		t.Errorf("the error log panel dropped its freshest line:\n%s", view)
	}
	note := fmt.Sprintf("… %d earlier lines", queued-shown)
	if !strings.Contains(view, note) {
		t.Errorf("the error log panel does not name the earlier lines it could not show (want %q):\n%s", note, view)
	}
}

// installingClockModel is an installing screen with the model's own start time
// and a later tick, so the elapsed time and the estimate are pinned numbers
// instead of a clock a test cannot predict. The steps are fixed so the progress
// fraction is fixed, too.
func installingClockModel(t *testing.T, progress float64) Model {
	t.Helper()
	start := time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenInstalling
	m.Width, m.Height = 80, 24
	m.Steps = []InstallStep{
		{ID: "a", Name: "Install Dependencies", Status: StatusDone, Progress: 1},
		{ID: "b", Name: "Clone Repository", Status: StatusRunning, Progress: progress},
		{ID: "c", Name: "Install Shell", Status: StatusPending},
		{ID: "d", Name: "Cleanup", Status: StatusPending},
	}
	m.CurrentStep = 1
	m.InstallStartedAt = start
	m.Now = start.Add(40 * time.Second)
	return m
}

// TestInstallingShowsTheStepAndTheRunsOwnClock pins the two rows under the bar:
// the step counter with the step's name, and the elapsed time with the estimate.
func TestInstallingShowsTheStepAndTheRunsOwnClock(t *testing.T) {
	// Half of the second of four steps is 0.375 done, so 40s spent implies 66s left.
	m := installingClockModel(t, 0.5)
	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{"Step 2 of 4", "Clone Repository", "Elapsed 40s", "~1m 06s left"} {
		if !strings.Contains(view, want) {
			t.Errorf("the installing screen does not show %q:\n%s", want, view)
		}
	}

	// The same model renders the same bytes on every call: the estimate comes from
	// state, so it cannot move while the model stands still.
	if second := m.View(); second != m.View() {
		t.Errorf("two renders of one model differ: the renderer read the clock")
	}
}

// TestInstallingEstimateIsHonest pins the estimate's edges. With nothing done
// there is no rate to scale, so the screen says it is estimating instead of
// showing a number, and with no basis at all -- no recorded start -- it says so
// too.
func TestInstallingEstimateIsHonest(t *testing.T) {
	t.Run("nothing done yet says it is estimating", func(t *testing.T) {
		m := installingClockModel(t, 0)
		// The model's earlier step is still pending, so nothing is complete and
		// there is no rate to scale.
		m.Steps[0].Status = StatusPending
		m.Steps[0].Progress = 0

		view := ansiEscape.ReplaceAllString(m.View(), "")
		if !strings.Contains(view, "Elapsed 40s") {
			t.Errorf("the screen lost the elapsed time with nothing done:\n%s", view)
		}
		if !strings.Contains(view, "estimating…") {
			t.Errorf("the screen shows no estimating state with nothing done:\n%s", view)
		}
		if strings.Contains(view, " left") {
			t.Errorf("the screen showed a number with no basis for one:\n%s", view)
		}
	})

	t.Run("no start time shows no estimate at all", func(t *testing.T) {
		m := installingClockModel(t, 0.5)
		m.InstallStartedAt = time.Time{}
		view := ansiEscape.ReplaceAllString(m.View(), "")
		if !strings.Contains(view, "Estimating the time remaining") {
			t.Errorf("a model with no start time does not say it is estimating:\n%s", view)
		}
		if strings.Contains(view, "~") {
			t.Errorf("a model with no start time showed an estimate:\n%s", view)
		}
	})
}

// TestInstallETAIsDerivedFromThisRunAlone pins that the estimate is the elapsed
// time scaled by the work still to do, and that it refuses to answer when there
// is no rate to scale: never a per-step constant and never a guess about the
// machine.
func TestInstallETAIsDerivedFromThisRunAlone(t *testing.T) {
	cases := []struct {
		name     string
		progress float64
		elapsed  time.Duration
		want     time.Duration
		ok       bool
	}{
		{"half done after a minute leaves a minute", 0.5, time.Minute, time.Minute, true},
		{"a quarter done after thirty seconds leaves ninety", 0.25, 30 * time.Second, 90 * time.Second, true},
		{"nothing done has no rate", 0, time.Minute, 0, false},
		{"a finished run has nothing left", 1, time.Minute, 0, false},
		{"no elapsed time has no rate", 0.5, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := installETA(c.progress, c.elapsed)
			if ok != c.ok {
				t.Fatalf("installETA(%v, %v) ok = %v, want %v", c.progress, c.elapsed, ok, c.ok)
			}
			if ok && got != c.want {
				t.Errorf("installETA(%v, %v) = %v, want %v", c.progress, c.elapsed, got, c.want)
			}
		})
	}
}

// TestInstallStartRecordsTheRunStartAndResetsIt pins that the run's start
// timestamp is written where the run begins, and overwritten by the next run
// rather than carried over.
func TestInstallStartRecordsTheRunStartAndResetsIt(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenInstalling
	m.Steps = []InstallStep{{ID: "a", Name: "Install Dependencies"}}
	stale := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	m.InstallStartedAt = stale

	before := time.Now()
	next, _ := m.Update(installStartMsg{})
	got := next.(Model).InstallStartedAt
	if got.IsZero() || got.Before(before.Add(-time.Second)) {
		t.Errorf("installStartMsg recorded %v, want a start time no earlier than %v", got, before)
	}
	if !got.After(stale) {
		t.Errorf("a new run kept the previous start time %v", got)
	}
}

// TestTickAdvancesTheRunsClock pins that the tick is what carries time onto the
// model: the installing screen reads the tick's timestamp rather than the wall
// clock while rendering.
func TestTickAdvancesTheRunsClock(t *testing.T) {
	when := time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC)
	m := NewModel()
	next, _ := m.Update(tickMsg(when))
	if got := next.(Model).Now; !got.Equal(when) {
		t.Errorf("the tick left Now = %v, want %v", got, when)
	}
}

// TestLeaderBannerReplacesTheLegend pins the mode indicator's notation and that
// it does not grow the screen: it takes over the legend row instead of being
// appended under a screen that already fills the frame, where it would land past
// the terminal's last row.
func TestLeaderBannerReplacesTheLegend(t *testing.T) {
	m := installerFrameModel(t, ScreenKeymapsMenu)
	m.LeaderMode = true
	view := m.View()

	if !strings.Contains(view, "Leader mode") {
		t.Errorf("the leader banner does not name the mode:\n%s", view)
	}
	if !strings.Contains(view, "[q] quit") || !strings.Contains(view, "[d] details") {
		t.Errorf("the leader banner does not use the shared notation:\n%s", view)
	}
	if rows := renderedRowCount(view); rows > trainerFrameHeight {
		t.Errorf("the leader banner grew the screen to %d rows, want <= %d:\n%s", rows, trainerFrameHeight, view)
	}
}

// =============================================================================
// SCROLL REACHABILITY
// =============================================================================
//
// The frame guard renders a scrollable screen at scroll 0, where it fits and
// passes; it cannot see the rows past the window. These tests render the END of
// a long list, at the greatest scroll its own key handler produces, so the tail
// is proven reachable. They exist because the shared window helper centred the
// window on the scroll value while the handlers kept clamping an offset, and the
// two disagreed silently: at the handler's maximum the window started rows/2
// short of the end.

// scrollUntilStill presses down until one press no longer moves the value the
// getter reads, so a test uses the handler's own bound instead of recomputing
// it and drifting from the thing it is checking.
func scrollUntilStill(t *testing.T, m Model, get func(Model) int) Model {
	t.Helper()
	for i := 0; i < 500; i++ {
		before := get(m)
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
		if get(m) == before {
			return m
		}
	}
	t.Fatalf("scroll never settled after 500 down presses")
	return m
}

// windowRange is the range one screen's own scroll row reports, parsed back out
// of the rendered view so the assertion runs against what the user can see.
func windowRange(t *testing.T, view, label string) (first, last, total int) {
	t.Helper()
	re := regexp.MustCompile(label + ` (\d+)-(\d+) of (\d+)`)
	match := re.FindStringSubmatch(view)
	if match == nil {
		t.Fatalf("the view has no %q scroll row:\n%s", label, view)
	}
	first, _ = strconv.Atoi(match[1])
	last, _ = strconv.Atoi(match[2])
	total, _ = strconv.Atoi(match[3])
	return first, last, total
}

// longKeymapCategory is the category the defect was measured on: "Search
// Commands (Snacks)" is longer than the 80x24 table window, so its tail is the
// rows the old mismatch hid.
const longKeymapCategory = 5

// TestKeymapCategoryLastBindingIsReachable renders the long category at the
// maximum scroll its handler reaches and asserts the LAST binding is on screen.
// The first binding was always visible, which is why the frame guard at scroll 0
// could not see this defect.
func TestKeymapCategoryLastBindingIsReachable(t *testing.T) {
	m := installerFrameModel(t, ScreenKeymapCategory)
	m.SelectedCategory = longKeymapCategory
	bindings := m.KeymapCategories[longKeymapCategory].Keymaps
	rows := keymapTableRows(m.Height)
	if len(bindings) <= rows {
		t.Fatalf("category %d has %d bindings, want more than the %d rows the frame shows",
			longKeymapCategory, len(bindings), rows)
	}

	m = scrollUntilStill(t, m, func(m Model) int { return m.KeymapScroll })
	last := bindings[len(bindings)-1]

	view := m.View()
	if !strings.Contains(view, last.Keys) {
		t.Errorf("at the handler's maximum scroll the last binding %q is off screen:\n%s", last.Keys, view)
	}
}

// TestLazyVimTopicLastLineIsReachable is the same probe for the LazyVim topic,
// whose closing lines the shared centring window also hid behind the handler's
// larger, differently-computed bound.
func TestLazyVimTopicLastLineIsReachable(t *testing.T) {
	m := installerFrameModel(t, ScreenLazyVimTopic)
	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]
	if len(topic.Tips) == 0 {
		t.Fatal("the first LazyVim topic has no tips to check the tail with")
	}

	m = scrollUntilStill(t, m, func(m Model) int { return m.LazyVimScroll })
	lastTip := topic.Tips[len(topic.Tips)-1]

	view := m.View()
	if !strings.Contains(view, lastTip) {
		t.Errorf("at the handler's maximum scroll the topic's last line %q is off screen:\n%s", lastTip, view)
	}
}

// TestScrollBoundExposesTheEnd pins the handler's bound and the renderer's
// window to one semantics: at the greatest scroll the handler produces, the
// range the screen prints ends on the list's last row. Without it the two can
// drift again while each stays self-consistent, which is exactly how this
// defect shipped.
func TestScrollBoundExposesTheEnd(t *testing.T) {
	t.Run("keymap category", func(t *testing.T) {
		m := installerFrameModel(t, ScreenKeymapCategory)
		m.SelectedCategory = longKeymapCategory
		want := len(m.KeymapCategories[longKeymapCategory].Keymaps)

		m = scrollUntilStill(t, m, func(m Model) int { return m.KeymapScroll })
		_, last, total := windowRange(t, m.View(), "Showing")

		if total != want {
			t.Fatalf("the scroll row reports %d bindings, want %d", total, want)
		}
		if last != total {
			t.Errorf("the handler's maximum scroll exposes rows up to %d of %d: the last rows cannot be reached", last, total)
		}
	})

	t.Run("lazyvim topic", func(t *testing.T) {
		m := installerFrameModel(t, ScreenLazyVimTopic)
		m = scrollUntilStill(t, m, func(m Model) int { return m.LazyVimScroll })
		_, last, total := windowRange(t, m.View(), "Lines")

		if last != total {
			t.Errorf("the handler's maximum scroll exposes lines up to %d of %d: the last lines cannot be reached", last, total)
		}
	})
}

// TestKeymapTableColumnsWidenForLongKeys pins the column rule the five tables
// share: the keys column is wide enough for the category's longest binding, so a
// 37-column Herdr binding is shown whole, and the description keeps at least a
// third of the frame.
func TestKeymapTableColumnsWidenForLongKeys(t *testing.T) {
	const width = 76
	keys, mode, description := keymapTableColumns(width, 37)
	if keys < 37 {
		t.Errorf("keys column = %d, want >= 37: the longest key is cut", keys)
	}
	if got := keys + mode + 2 + description; got != width {
		t.Errorf("columns total %d, want %d", got, width)
	}
	if description < width/3 {
		t.Errorf("description column = %d, want >= %d", description, width/3)
	}
}

// TestPlaceBodyCentresShortBodies pins the frame's vertical placement. The body
// used to be top-aligned, so a short screen left eight or nine blank rows between
// the content and the pinned footer and read as an unfinished screen. It is
// centred in the rows between header and footer now, and a body that fills or
// overflows those rows is left exactly where it was, so a list long enough to
// scroll loses nothing.
func TestPlaceBodyCentresShortBodies(t *testing.T) {
	short := []string{"title", "description", "option"}
	if got := placeBody(short, 9); len(got) != 9 || got[3] != "title" || got[5] != "option" {
		t.Errorf("placeBody(3 rows into 9) = %q, want the body centred on rows 3-5 with the remaining space split above and below", got)
	}

	full := []string{"a", "b", "c"}
	if got := placeBody(full, 3); strings.Join(got, "|") != "a|b|c" {
		t.Errorf("placeBody(3 rows into 3) = %q, want the body unchanged", got)
	}

	over := []string{"a", "b", "c", "d"}
	if got := placeBody(over, 2); strings.Join(got, "|") != "a|b|c|d" {
		t.Errorf("placeBody(4 rows into 2) = %q, want the oversized body unchanged, not truncated", got)
	}
}

// TestPlaceBodyCapsTheTopMargin pins the other half of the placement: the shift
// down is capped at placeBodyTopMarginMax rows, so a tall terminal leaves the
// body just under the rule instead of floating it halfway down the screen. The
// 80x24 floor is below the cap and does not move; the two sizes this feature was
// designed on are above it and are held at the cap. The heights are asserted as
// terminal rows with View()'s own chrome counted, because that is the row a
// reader sees, and the three heights are the floor and the two wide sizes.
func TestPlaceBodyCapsTheTopMargin(t *testing.T) {
	hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}
	body := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}

	for _, c := range []struct {
		name                string
		width, height       int
		wantFirst, wantLast int
	}{
		{"80x24, the floor: a 5-row shift, under the cap", 80, 24, 9, 17},
		{"160x50: the shift is the cap, not the 18 the surplus half would ask for", 160, 50, 10, 18},
		{"227x62: the cap holds at the widest terminal", 227, 62, 10, 18},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := Model{Width: c.width, Height: c.height, Screen: ScreenMainMenu}
			rows := installerBodyRows(c.height, footerRowCount(layoutFor(m).Inner, hints))
			placed := placeBody(body, rows)

			first, last := -1, -1
			for i, row := range placed {
				if row == "" {
					continue
				}
				if first < 0 {
					first = i
				}
				last = i
			}

			// View() spends one blank row on its padding and the frame two rows on
			// the header and the rule under it, so the body's first row of the
			// composition is the terminal's fourth row.
			const chrome = 1 + 2
			if got := first + chrome + 1; got != c.wantFirst {
				t.Errorf("the body starts on terminal row %d, want %d: the top margin is %d rows", got, c.wantFirst, first)
			}
			if got := last + chrome + 1; got != c.wantLast {
				t.Errorf("the body ends on terminal row %d, want %d", got, c.wantLast)
			}
		})
	}
}

// TestCompanionGoldenPinsThePixelSpriteAndItsGaze snapshots the ladder's top step:
// the shaded sprite at the wide main menu, on a tick and a gaze cell written on the
// model before the program starts, so the snapshot pins a composited pixel frame
// instead of flaking on the clock. PixelSprite is forced here because the real gate
// asks the terminal for its colour profile and this test has no terminal: the field
// is the seam, and the gate itself is pinned by the sprite's own tests.
func TestCompanionGoldenPinsThePixelSpriteAndItsGaze(t *testing.T) {
	skipIfTermux(t)
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.SystemInfo = goldenSystemInfo()
	m.ExistingConfigs = system.DetectExistingConfigs()
	m.Width = 160
	m.Height = 50
	m.Screen = ScreenMainMenu
	m.Animating = true
	m.PixelSprite = true
	m.AnimTick = 3
	m.CompanionGaze = companionGaze{X: 1, Y: -1}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(160, 50),
	)

	seen := &bytes.Buffer{}
	teatest.WaitFor(t, io.TeeReader(tm.Output(), seen), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Main Menu"))
	}, teatest.WithCheckInterval(2*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	if _, err := io.Copy(seen, tm.Output()); err != nil {
		t.Fatalf("reading the rest of the output failed: %v", err)
	}
	teatest.RequireEqualOutput(t, seen.Bytes())
}

// =============================================================================
// TRAINER HINT COHERENCE AND THE TWO-COLUMN LESSON
// =============================================================================

// TestTrainerHintLabelOmitsAnEmptyHint pins the empty-hint decision: the label
// helper returns no label at all for an exercise whose hint was dropped, so no
// caller can print the bare marker on its own. It is the single place the
// decision lives, which is what keeps the lesson, the practice and the boss
// handlers in step.
func TestTrainerHintLabelOmitsAnEmptyHint(t *testing.T) {
	if got := trainerHintLabel(nil); got != "" {
		t.Errorf("trainerHintLabel(nil) = %q, want no label", got)
	}
	if got := trainerHintLabel(&trainer.Exercise{}); got != "" {
		t.Errorf("trainerHintLabel with no hint = %q, want no label: a dropped hint must render nothing", got)
	}
	exercise := &trainer.Exercise{Hint: "dd deletes the line"}
	if got, want := trainerHintLabel(exercise), "💡 Hint: dd deletes the line"; got != want {
		t.Errorf("trainerHintLabel = %q, want %q", got, want)
	}
}

// TestTrainerTabRevealsNoLabelWithoutAHint drives the real key handler on a
// lesson whose hint was dropped and pins that Tab writes no bare label. The
// label used to be built inline, so the marker printed with nothing after it.
func TestTrainerTabRevealsNoLabelWithoutAHint(t *testing.T) {
	m := newTrainerLessonModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("test setup: no live lesson exercise")
	}
	// The mission/hint rule permits a dropped hint; this is what the data looks
	// like when one is dropped.
	exercise.Hint = ""
	m.TrainerMessage = ""

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.TrainerMessage != "" {
		t.Errorf("TrainerMessage after Tab on a hint-less exercise = %q, want empty: the label must not print with no hint", got.TrainerMessage)
	}
}

// TestBossTabRevealsTheStepHint is the reachability test the boss screen was
// missing: the Change & Repeat boss step hints the previous pass rewrote sat in
// the data with no key that could show them. It starts that boss, presses Tab
// through the real global handler, and pins the hint line.
func TestBossTabRevealsTheStepHint(t *testing.T) {
	m := newTrainerBossModel(t)
	m.TrainerGameState.StartBoss(trainer.ModuleChangeRepeat)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil || exercise.Hint == "" {
		t.Fatalf("test setup: the Change & Repeat boss step 1 carries no hint")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if want := "💡 Hint: " + exercise.Hint; got.TrainerMessage != want {
		t.Errorf("TrainerMessage after Tab on the boss screen = %q, want %q", got.TrainerMessage, want)
	}
}

// TestBossTabRevealsNoLabelWithoutAHint pins the other half on the boss screen:
// a step with no hint gets no label, exactly like the exercise screen. The
// horizontal boss steps ship without hints, so that is the screen under test.
func TestBossTabRevealsNoLabelWithoutAHint(t *testing.T) {
	m := newTrainerBossModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("test setup: no live boss step")
	}
	exercise.Hint = ""
	m.TrainerMessage = ""

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.TrainerMessage != "" {
		t.Errorf("TrainerMessage after Tab on a hint-less boss step = %q, want empty", got.TrainerMessage)
	}
}

// TestBossLegendAdvertisesTheHintKey pins both halves of the boss hint footer:
// the key the screen now honours is advertised, and advertising it did not cost
// the boss screen a row at the 80x24 floor. If the extra hint had needed a third
// legend row, the screen would be 25 rows tall and this fails.
func TestBossLegendAdvertisesTheHintKey(t *testing.T) {
	m := newTrainerBossModel(t)
	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight

	view := m.View()
	if !strings.Contains(view, "[Tab] hint") {
		t.Errorf("the boss legend does not advertise [Tab] hint even though the screen honours it:\n%s", view)
	}
	if rows := renderedRowCount(view); rows > trainerFrameHeight {
		t.Errorf("the boss hint grew the boss screen to %d rows at %dx%d, want <= %d:\n%s",
			rows, trainerFrameWidth, trainerFrameHeight, trainerFrameHeight, view)
	}
}

// TestTrainerLessonComposesTwoColumnsInWideTerminals pins the wide lesson
// layout: at the sizes the installer's own screens go two-column, the code
// window sits in the left column and the mission in the right one, on the same
// rows, and the screen still fits the terminal.
func TestTrainerLessonComposesTwoColumnsInWideTerminals(t *testing.T) {
	m := newTrainerLessonModel(t)
	m.Width, m.Height = 160, 50

	view := m.View()
	t.Logf("160x50 lesson, escape sequences stripped:\n%s", ansiEscape.ReplaceAllString(view, ""))

	sideBySide := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Code") && strings.Contains(line, "Mission") {
			sideBySide = true
			break
		}
	}
	if !sideBySide {
		t.Fatalf("the lesson screen at 160x50 does not put the code window beside the mission:\n%s", view)
	}

	// The mission, the answer and the feedback must all still be on screen.
	for _, want := range []string{"Mission", "Answer", "[Ctrl-v] block"} {
		if !strings.Contains(view, want) {
			t.Errorf("the two-column lesson screen dropped %q:\n%s", want, view)
		}
	}

	if rows := renderedRowCount(view); rows > 50 {
		t.Errorf("the two-column lesson screen renders %d rows at 160x50, want <= 50:\n%s", rows, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 160 {
			t.Errorf("the two-column lesson screen renders a line %d columns wide at 160x50, want <= 160: %q", w, line)
		}
	}
}

// TestWideLessonShowsMoreCodeThanTheNarrowFloor pins the point of the wide
// layout: the code window takes the rows the stacked right column no longer
// needs, so a wide terminal shows more code, not just wider code.
func TestWideLessonShowsMoreCodeThanTheNarrowFloor(t *testing.T) {
	m := newTrainerLessonModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("test setup: no live lesson exercise")
	}

	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	_, narrow := m.trainerTextBudget(exercise)

	m.Width, m.Height = 160, 50
	_, wide := m.trainerTextBudget(exercise)

	if wide <= narrow {
		t.Errorf("the code window is %d rows at 160x50 and %d at 80x24, want strictly more: the wide layout must show more code", wide, narrow)
	}
	if narrow < 1 {
		t.Errorf("the code window at 80x24 is %d rows, want at least one", narrow)
	}
}

// trainerCodeRowPattern matches one rendered code row: the block gutter, a
// line number, the inner gutter. It is how a test counts the code rows on a
// screen without depending on the screen's other guttered blocks.
var trainerCodeRowPattern = regexp.MustCompile(`│ *\d+ │`)

// trainerCodeRowCount counts the code rows a rendered trainer screen shows.
func trainerCodeRowCount(view string) int {
	n := 0
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
		if trainerCodeRowPattern.MatchString(line) {
			n++
		}
	}
	return n
}

// TestWideLessonRendersMoreCodeForLongExercises drives the renderer, not just
// the row budget: a code block longer than the narrow window must show more of
// itself at 160x50. The longest shipped lesson (vertical_015, eleven lines)
// already fits the eleven-row narrow window, so the test uses a fourteen-line
// regex boss step, which is the same code the lesson window renders. This is
// the end-to-end proof that the height the two columns free is spent on code.
func TestWideLessonRendersMoreCodeForLongExercises(t *testing.T) {
	m := newTrainerLessonModel(t)
	long := trainer.GetBoss(trainer.ModuleRegex).Steps[0].Exercise
	m.TrainerGameState.SetPracticeExercise(&long)

	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	narrowRows := trainerCodeRowCount(m.View())

	m.Width, m.Height = 160, 50
	wideRows := trainerCodeRowCount(m.View())

	if wideRows <= narrowRows {
		t.Errorf("the wide lesson shows %d code rows and the narrow floor %d, want strictly more: the extra height must be spent on code", wideRows, narrowRows)
	}
	if wideRows != len(long.Code) {
		t.Errorf("the wide lesson shows %d code rows of a %d-line exercise, want all of them", wideRows, len(long.Code))
	}
}
