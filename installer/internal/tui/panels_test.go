package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/charmbracelet/lipgloss"
)

// narrowPanelLayout is the narrowest column a panel can be given: the first
// content width the layout composes at. A panel test measures against this rather
// than against the 160-column composition, because the narrow column is where a
// long value, a long label and a crowded panel all show up first.
func narrowPanelLayout() layout {
	return layoutFor(Model{Width: layoutTwoColumnWidth + 4})
}

// panelText strips a panel's styling and joins its rows, so a test reads the
// words a reader sees instead of the escape sequences they are wrapped in. The
// assertions are on the words on purpose: the panel's meaning has to survive a
// terminal with no colour at all.
func panelText(rows []string) string {
	plain := make([]string, len(rows))
	for i, row := range rows {
		plain[i] = ansiEscape.ReplaceAllString(row, "")
	}
	return strings.Join(plain, "\n")
}

// panelFlat is panelText with the wrapping undone: its rows joined by single
// spaces. A value long enough to wrap is one sentence across several rows, so an
// assertion about the sentence reads it flat, while an assertion about the layout
// reads panelText.
func panelFlat(rows []string) string {
	return strings.Join(strings.Fields(panelText(rows)), " ")
}

// assertPanelFits checks the two bounds every panel has to respect: no row wider
// than its column, and no more rows than the frame left for it.
func assertPanelFits(t *testing.T, rows []string, width, budget int) {
	t.Helper()
	if len(rows) > budget {
		t.Errorf("the panel is %d rows in a %d-row budget:\n%s", len(rows), budget, panelText(rows))
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got > width {
			t.Errorf("a panel row is %d columns in a %d-column panel: %q", got, width, row)
		}
	}
}

// TestWelcomePanelNamesTheMachineItWasGiven pins the machine panel against a
// pinned host: every row comes from a field system.Detect fills, and the package
// manager is the one the dependency step itself would dispatch through.
func TestWelcomePanelNamesTheMachineItWasGiven(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: &system.SystemInfo{
		OS:         system.OSDebian,
		OSName:     "Debian/Ubuntu",
		Arch:       "arm64",
		IsWSL:      true,
		WSLVersion: 2,
		IsARM:      true,
		UserShell:  "zsh",
		HomeDir:    "/home/testuser",
	}}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	for _, want := range []string{
		welcomePanelLabel,
		"OS", "Debian/Ubuntu",
		"Host", "WSL 2",
		"Arch", "arm64",
		"Shell", "zsh",
		"Packages", "apt-get",
		"Home", "/home/testuser",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the machine panel does not name %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, "Xcode") {
		t.Errorf("a Linux host got an Xcode row, though detection never probed the tools there:\n%s", text)
	}
}

// TestWelcomePanelOmitsFactsTheModelDoesNotHave pins the panel's honesty rule: a
// value the model does not hold is left out, not filled in with a guess or with
// the word "unknown". The pinned Linux host is not ARM, not WSL and not macOS, so
// neither of those rows may appear, and no row may claim a fact nobody measured.
func TestWelcomePanelOmitsFactsTheModelDoesNotHave(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	for _, absent := range []string{"WSL", "Termux", "Xcode", "ARM", "unknown", "Unknown", "none", "n/a"} {
		if strings.Contains(text, absent) {
			t.Errorf("the machine panel claims %q, which the model does not hold:\n%s", absent, text)
		}
	}
	for _, want := range []string{"OS", "Linux", "Shell", "zsh", "Packages", "apt-get", "Home", "/home/testuser"} {
		if !strings.Contains(text, want) {
			t.Errorf("the machine panel is missing %q:\n%s", want, text)
		}
	}
}

// TestWelcomePanelWrapsLongValuesRatherThanClipping pins the row contract: a
// value wider than the panel wraps onto the lines under it, indented under the
// value column, so nothing is dropped at the gutter.
func TestWelcomePanelWrapsLongValuesRatherThanClipping(t *testing.T) {
	l := narrowPanelLayout()
	const home = "/home/a/user/with/a/really/long/path/that/cannot/fit/one/panel/row/at/all"
	info := goldenSystemInfo()
	info.HomeDir = home
	m := Model{SystemInfo: info}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)

	// The value is wrapped, not cut: every segment is still on screen in order.
	joined := strings.Join(strings.Fields(panelText(rows)), "")
	if !strings.Contains(joined, home) {
		t.Errorf("the home directory was dropped instead of wrapped:\n%s", panelText(rows))
	}
}

// TestWillHappenPanelMarksWhereTheRunStarts pins the plan panel when the model
// holds a plan: the count, every step's name, the ▸ the menus already use for
// "here" on the step the run starts at, and that step's description and no
// others'. While nothing has run, that is the first step; mid-run it is the step
// that is running, because that is the next action.
func TestWillHappenPanelMarksWhereTheRunStarts(t *testing.T) {
	l := narrowPanelLayout()
	steps := []InstallStep{
		{Name: "Backup Existing Configs", Description: "Saves a copy of your current configuration first."},
		{Name: "Install Dependencies", Description: "Installs base packages with your distribution's package manager."},
		{Name: "Clone Repository", Description: "Downloads your dotfiles repository."},
	}

	// assertOnlyThisDescription is the rule the panel exists for: the names are
	// all readable, and exactly the current step carries its paragraph. The
	// descriptions wrap inside the narrow column, so a present sentence is
	// asserted on the flat text.
	assertOnlyThisDescription := func(t *testing.T, text, flat string, current int) {
		t.Helper()
		for _, step := range steps {
			if !strings.Contains(flat, step.Name) {
				t.Errorf("the plan panel does not show the step %q:\n%s", step.Name, text)
			}
		}
		for i, step := range steps {
			has := strings.Contains(flat, step.Description)
			if want := i == current; has != want {
				t.Errorf("the plan panel shows the description of step %d = %v, want %v:\n%s", i+1, has, want, text)
			}
		}
	}

	t.Run("nothing has started, so the first step is where it starts", func(t *testing.T) {
		m := Model{Steps: steps}
		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)
		flat := panelFlat(rows)

		assertOnlyThisDescription(t, text, flat, 0)
		if !strings.Contains(flat, "Steps 3") {
			t.Errorf("the plan panel does not count the steps:\n%s", text)
		}
		if got := strings.Count(text, "▸"); got != 1 {
			t.Errorf("the plan panel marks %d steps as where the run starts, want exactly one:\n%s", got, text)
		}
		if !strings.Contains(text, "▸ 1") {
			t.Errorf("the plan panel marks the wrong step as where the run starts:\n%s", text)
		}
	})

	t.Run("a run in progress marks the step it is on", func(t *testing.T) {
		running := append([]InstallStep{}, steps...)
		running[0].Status = StatusDone
		running[1].Status = StatusRunning
		m := Model{Steps: running, CurrentStep: 1}

		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)

		assertOnlyThisDescription(t, text, panelFlat(rows), 1)
		if !strings.Contains(text, "▸ 2") {
			t.Errorf("the plan panel does not mark the running step:\n%s", text)
		}
		if strings.Contains(text, "▸ 1") {
			t.Errorf("the plan panel still marks the finished first step as where the run starts:\n%s", text)
		}
	})
}

// TestStepPanelRowsNumberTheWholePlan pins the list shape directly: one row per
// name underneath the number column, the marker in that same column so marking a
// step does not shift it, and every row inside the column it was handed. A step
// count that grows past nine widens the column rather than letting the numbers
// run into the names.
func TestStepPanelRowsNumberTheWholePlan(t *testing.T) {
	steps := make([]InstallStep, 10)
	for i := range steps {
		steps[i] = InstallStep{Name: fmt.Sprintf("Step name %d", i+1)}
	}

	const width = 40
	rows := stepPanelRows(steps, 0, width)
	if len(rows) != len(steps) {
		t.Fatalf("stepPanelRows returned %d rows for %d steps with no descriptions:\n%s", len(rows), len(steps), panelText(rows))
	}

	// Every name starts on the same column and every number ends on the same
	// column: the marker has a column of its own, so marking a step shifts
	// neither, and a two-digit plan right-aligns its digits in a column of its own
	// instead of running them into the names.
	nameColumn, numberEdge := -1, -1
	for _, row := range rows {
		if got := lipgloss.Width(row); got > width {
			t.Errorf("a step row is %d columns in a %d-column panel: %q", got, width, row)
		}
		plain := ansiEscape.ReplaceAllString(row, "")
		at := strings.Index(plain, "Step name")
		if at < 0 {
			t.Fatalf("a step row does not name its step: %q", plain)
		}
		// Compare the COLUMN, not the byte offset: the marker glyph is one column
		// and three bytes, so a byte offset would call the marked row misaligned.
		column := lipgloss.Width(plain[:at])
		if nameColumn < 0 {
			nameColumn = column
		} else if column != nameColumn {
			t.Errorf("a step name starts at column %d, want %d: %q", column, nameColumn, plain)
		}

		// The first digit run in the row is the step's number; the name's own
		// digits come later. Its trailing edge is the same column on every row, so
		// a ten-step plan keeps a straight edge under the marker instead of a
		// ragged one.
		start := strings.IndexFunc(plain, func(r rune) bool { return r >= '0' && r <= '9' })
		if start < 0 {
			t.Fatalf("a step row does not number its step: %q", plain)
		}
		end := start
		for end < len(plain) && plain[end] >= '0' && plain[end] <= '9' {
			end++
		}
		edge := lipgloss.Width(plain[:end])
		if numberEdge < 0 {
			numberEdge = edge
		} else if edge != numberEdge {
			t.Errorf("a step number ends at column %d, want %d: %q", edge, numberEdge, plain)
		}
	}

	text := panelText(rows)
	if got := strings.Count(text, "▸"); got != 1 {
		t.Errorf("the step list marks %d steps, want exactly one:\n%s", got, text)
	}
	if !strings.Contains(panelFlat(rows), "▸ 1 Step name 1") {
		t.Errorf("the step list does not mark the first step:\n%s", text)
	}
	if !strings.Contains(text, "10") {
		t.Errorf("the step list does not number the last step:\n%s", text)
	}
}

// TestWillHappenPanelOmitsTheSectionsItHasNotScanned pins what the panel may say
// for a model with no detected host, where there is no OS to describe and so no
// plan to build. It shows nothing about the plan, the configs or the backups:
// "none" would be a claim the model cannot support, because nothing was
// detected. A model that does carry a host gets the plan instead, which
// TestWillHappenPanelPreviewsThePlan pins.
func TestWillHappenPanelOmitsTheSectionsItHasNotScanned(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	if !strings.Contains(text, mainMenuPanelLabel) {
		t.Errorf("the panel does not name itself:\n%s", text)
	}
	for _, absent := range []string{"Steps", "Overwrites", "Newest backup", "none", "None", "unknown", "0 configs", "0 files", "(detected)"} {
		if strings.Contains(text, absent) {
			t.Errorf("the plan panel claims %q for state nothing has filled in:\n%s", absent, text)
		}
	}
}

// TestWillHappenPanelPreviewsThePlan pins the fix for the empty panel: on the
// main menu, before the wizard has built the plan, the panel still shows the
// plan the run would execute, built by the wizard's own builder from the
// detected host. It labels which host it is describing, because the OS question
// has not been asked yet.
func TestWillHappenPanelPreviewsThePlan(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)
	flat := panelFlat(rows)

	for _, want := range []string{
		"on Linux (detected)",
		"Steps 8",
		"Install Dependencies",
		"Clone Repository",
		"Install Homebrew",
		"Set Default Shell",
		"Cleanup",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the previewed plan does not show %q:\n%s", want, text)
		}
	}
	if got := strings.Count(text, "▸"); got != 1 {
		t.Errorf("the previewed plan marks %d steps as where the run starts, want exactly one:\n%s", got, text)
	}
	if !strings.Contains(text, "▸ 1") {
		t.Errorf("the previewed plan does not mark the first step as where the run starts:\n%s", text)
	}
	// A choice that has not been made is not in the plan: the shell step has no
	// name until the player picks one.
	if strings.Contains(flat, "Install  ") {
		t.Errorf("the previewed plan shows a step whose choice is still open:\n%s", text)
	}
}

// TestWillHappenPanelDropsTheHostLineWhenTheChoiceMatchesDetection pins the
// other half of the labelling rule: once the player has chosen the OS detection
// already found, the plan is for the chosen host and the note is dropped.
func TestWillHappenPanelDropsTheHostLineWhenTheChoiceMatchesDetection(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), Choices: UserChoices{OS: "linux"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if strings.Contains(text, "(detected)") {
		t.Errorf("the panel still labels a host the player has chosen:\n%s", text)
	}
	if !strings.Contains(text, "Install Dependencies") {
		t.Errorf("the panel does not show the chosen Linux host's plan:\n%s", text)
	}
}

// TestWillHappenPanelNamesTheChosenHostWhenItDiffersFromDetection pins that the
// panel never describes a plan for a host the run will not use: when the choice
// differs from detection, the plan follows the choice and the note names it.
func TestWillHappenPanelNamesTheChosenHostWhenItDiffersFromDetection(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), Choices: UserChoices{OS: "mac"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if !strings.Contains(text, "on macOS") {
		t.Errorf("the panel does not name the chosen host:\n%s", text)
	}
	if strings.Contains(text, "Install Dependencies") {
		t.Errorf("the panel shows the Linux plan for a macOS choice:\n%s", text)
	}
	if !strings.Contains(text, "Install Xcode CLI") {
		t.Errorf("the panel does not show the chosen macOS host's plan:\n%s", text)
	}
}

// TestWillHappenPanelOmitsTheBackupRowWithoutBackups pins the omitting rule for
// the last row: a model with no backups shows no backup row, rather than a row
// claiming there is none. The presence half is pinned by
// TestWillHappenPanelShowsTheNewestBackupAndItsAge.
func TestWillHappenPanelOmitsTheBackupRowWithoutBackups(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	if strings.Contains(text, "Newest backup") {
		t.Errorf("the plan panel invents a backup row for a model with none:\n%s", text)
	}
	if strings.Contains(text, "Overwrites") {
		t.Errorf("the plan panel invents an overwrite row for a model with none:\n%s", text)
	}
}

// TestPlanForLeavesOutAStepWhoseChoiceIsOpen pins the one place the shared
// builder is not a pure move of SetupInstallSteps: the shell has no "skip"
// option, so the wizard always has one and its plan is unchanged, while the main
// menu's preview runs before the question and leaves the nameless step out
// instead of showing "Install ".
func TestPlanForLeavesOutAStepWhoseChoiceIsOpen(t *testing.T) {
	open := planFor(planOptions{OS: "linux"})
	for _, step := range open {
		if step.ID == "shell" {
			t.Errorf("planFor planned a shell step with no shell chosen: %q", step.Name)
		}
	}

	chosen := planFor(planOptions{OS: "linux", Shell: "fish"})
	found := false
	for _, step := range chosen {
		if step.ID == "shell" {
			found = true
			if step.Name != "Install fish" {
				t.Errorf("shell step name = %q, want %q", step.Name, "Install fish")
			}
		}
	}
	if !found {
		t.Error("planFor dropped the shell step once a shell was chosen")
	}
}

// TestInitDetectsExistingConfigsForTheMainMenu pins the startup scan end to end:
// Init's command reads the config paths, and the message it produces puts them
// on the model, so the main menu's panel can say what the run will overwrite
// before the wizard asks anything.
func TestInitDetectsExistingConfigsForTheMainMenu(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "nvim"), 0o755); err != nil {
		t.Fatalf("creating an existing config failed: %v", err)
	}

	m := NewModel()
	result, _ := m.Update(detectConfigsCmd()())
	got := result.(Model)

	found := false
	for _, config := range got.ExistingConfigs {
		if strings.HasPrefix(config, "nvim: ") {
			found = true
		}
	}
	if !found {
		t.Errorf("the startup scan did not reach the model: ExistingConfigs = %v", got.ExistingConfigs)
	}
}

// TestStartupScanDoesNotOverwriteKnownConfigs pins the bootstrap rule: the scan
// fills the model's overwrite facts when it has none, and leaves them alone when
// it already knows them. The wizard's own scan is the authority that replaces
// them; a startup scan that clobbered a model handed its state would make every
// end-to-end test that seeds the field depend on the HOME the suite runs under.
func TestStartupScanDoesNotOverwriteKnownConfigs(t *testing.T) {
	m := NewModel()
	m.ExistingConfigs = []string{"nvim: /known/nvim"}

	result, _ := m.Update(configsDetectedMsg{configs: []string{"fish: /scanned/fish"}})
	got := result.(Model)

	if len(got.ExistingConfigs) != 1 || got.ExistingConfigs[0] != "nvim: /known/nvim" {
		t.Errorf("the startup scan overwrote known configs: %v", got.ExistingConfigs)
	}
}

// TestPlanPanelRendersTheStartupConfigsAsOverwrites pins the join between the two
// halves: once the startup scan has filled the model, the panel's overwrite rows
// are real facts rather than the absent section TestWillHappenPanelOmitsTheBackupRow
// proposes for an unscanned model.
func TestPlanPanelRendersTheStartupConfigsAsOverwrites(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), ExistingConfigs: []string{"nvim: /home/testuser/.config/nvim"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if !strings.Contains(text, "1 config") {
		t.Errorf("the panel does not count the scanned config as an overwrite:\n%s", text)
	}
	if !strings.Contains(text, "nvim: /home/testuser/.config/nvim") {
		t.Errorf("the panel does not name the config the run will overwrite:\n%s", text)
	}
}

// TestWillHappenPanelCountsTheConfigsTheRunWillOverwrite pins the overwrite rows
// for a model that has scanned them: the count, in the singular when it is one,
// and which configs they are.
func TestWillHappenPanelCountsTheConfigsTheRunWillOverwrite(t *testing.T) {
	l := narrowPanelLayout()
	configs := []string{
		"nvim: /home/testuser/.config/nvim",
		"fish: /home/testuser/.config/fish",
	}

	t.Run("several", func(t *testing.T) {
		m := Model{ExistingConfigs: configs}
		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)

		if !strings.Contains(text, "2 configs") {
			t.Errorf("the plan panel does not count the configs the run will overwrite:\n%s", text)
		}
		for _, config := range configs {
			if !strings.Contains(text, config) {
				t.Errorf("the plan panel does not name %q:\n%s", config, text)
			}
		}
	})

	t.Run("one reads as one config", func(t *testing.T) {
		m := Model{ExistingConfigs: configs[:1]}
		text := panelText(m.mainMenuPanel(l, 40))
		if !strings.Contains(text, "1 config") || strings.Contains(text, "1 configs") {
			t.Errorf("the plan panel does not write a single config in the singular:\n%s", text)
		}
	})
}

// TestWillHappenPanelShowsTheNewestBackupAndItsAge pins the last row: the newest
// backup of whatever the model holds, with when it was taken and how old it is.
// The list system.ListBackups returns is in directory order, so the newest is the
// greatest timestamp and not the last row.
func TestWillHappenPanelShowsTheNewestBackupAndItsAge(t *testing.T) {
	l := narrowPanelLayout()
	newest := system.BackupInfo{
		Path:      "/home/testuser/.dotfiles-backup-20240103-120000",
		Timestamp: time.Now().Add(-72 * time.Hour),
		Files:     []string{"nvim", "fish", "zsh"},
	}
	older := system.BackupInfo{
		Path:      "/home/testuser/.dotfiles-backup-20240101-120000",
		Timestamp: time.Now().Add(-30 * 24 * time.Hour),
		Files:     []string{"nvim"},
	}
	m := Model{AvailableBackups: []system.BackupInfo{newest, older}}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	for _, want := range []string{"Newest backup", newest.Timestamp.Format(panelBackupStamp), "3 files", "3 days ago"} {
		if !strings.Contains(text, want) {
			t.Errorf("the plan panel does not show %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, older.Timestamp.Format(panelBackupStamp)) {
		t.Errorf("the plan panel shows the older backup as the newest:\n%s", text)
	}
}

// TestBackupAgeNamesTheCoarsestUnitThatSaysSomething pins the age arithmetic to
// fixed clock readings, so it cannot flake: the panel reads the clock once and
// hands the two instants here.
func TestBackupAgeNamesTheCoarsestUnitThatSaysSomething(t *testing.T) {
	now := time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		then time.Time
		want string
	}{
		{"just taken", now, "just now"},
		{"under a minute", now.Add(-30 * time.Second), "just now"},
		{"one minute", now.Add(-time.Minute), "1 minute ago"},
		{"minutes", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"one hour", now.Add(-time.Hour), "1 hour ago"},
		{"hours", now.Add(-5 * time.Hour), "5 hours ago"},
		{"one day", now.Add(-25 * time.Hour), "1 day ago"},
		{"days", now.Add(-3 * 24 * time.Hour), "3 days ago"},
		{"one month", now.Add(-40 * 24 * time.Hour), "1 month ago"},
		{"months", now.Add(-100 * 24 * time.Hour), "3 months ago"},
		{"a clock that is ahead makes it just now, not a negative age", now.Add(time.Hour), "just now"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := backupAge(c.then, now); got != c.want {
				t.Errorf("backupAge = %q, want %q", got, c.want)
			}
		})
	}
}

// TestPanelNeverExceedsItsColumnOrItsBudget pins the bounds at three budgets: a
// panel with more rows than the frame leaves keeps the rows that fit and says how
// many it dropped, so a wide-but-short terminal loses rows out loud instead of
// pushing the footer off the screen.
func TestPanelNeverExceedsItsColumnOrItsBudget(t *testing.T) {
	l := narrowPanelLayout()
	steps := make([]InstallStep, 0, 8)
	for i := 0; i < 8; i++ {
		steps = append(steps, InstallStep{
			Name:        "A step with a name long enough to wrap inside the column",
			Description: "And a description long enough to take more than one row of the panel beside it.",
		})
	}
	m := Model{
		Steps:            steps,
		ExistingConfigs:  []string{"nvim: /home/testuser/.config/nvim", "fish: /home/testuser/.config/fish"},
		AvailableBackups: []system.BackupInfo{{Timestamp: time.Now().Add(-time.Hour), Files: []string{"nvim"}}},
	}

	for _, budget := range []int{5, 12, 60} {
		rows := m.mainMenuPanel(l, budget)
		assertPanelFits(t, rows, l.Right, budget)

		text := panelText(rows)
		clipped := strings.Contains(text, cutMarker+" and ")
		if clipped != (budget < len(m.mainMenuPanel(l, 200))) {
			t.Errorf("at a %d-row budget the panel's cut note is %v:\n%s", budget, clipped, text)
		}
	}
}

// TestPanelsStayOffTheFloorFrame pins the collapse: below the two-column floor
// neither panel has room, so the screen renders exactly as it did before the
// panels existed and no panel title appears. The wide-terminal guard covers the
// other half of the same rule.
func TestPanelsStayOffTheFloorFrame(t *testing.T) {
	for _, screen := range []struct {
		name   string
		screen Screen
		panel  string
	}{
		{"welcome", ScreenWelcome, welcomePanelLabel},
		{"main menu", ScreenMainMenu, mainMenuPanelLabel},
	} {
		screen := screen
		t.Run(screen.name, func(t *testing.T) {
			m := installerFrameModel(t, screen.screen)
			view := ansiEscape.ReplaceAllString(m.View(), "")
			if strings.Contains(view, screen.panel) {
				t.Errorf("the %s screen shows the %q panel at 80x24, where there is no room for a second column:\n%s",
					screen.name, screen.panel, view)
			}
		})
	}
}

// TestWelcomeScreenCarriesTheMachinePanelWhenItHasRoom pins the wiring end to
// end: the welcome screen is the one that asks where am I, so at a two-column
// size it renders the panel the machine's own fields produce, and the frame
// drops that panel below the floor.
func TestWelcomeScreenCarriesTheMachinePanelWhenItHasRoom(t *testing.T) {
	m := Model{Width: 160, Height: 50, Screen: ScreenWelcome, SystemInfo: goldenSystemInfo()}
	view := ansiEscape.ReplaceAllString(m.View(), "")

	if !strings.Contains(view, welcomePanelLabel) {
		t.Errorf("the welcome screen at 160x50 has no room for its panel:\n%s", view)
	}
	for _, want := range []string{"Linux", "zsh", "apt-get", "/home/testuser"} {
		if !strings.Contains(view, want) {
			t.Errorf("the welcome screen's panel does not name %q:\n%s", want, view)
		}
	}
}
