package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// themeSurvivesHome builds a home in the state an install leaves behind: the
// files this repository ships, copied in, carrying the default theme's generated
// blocks. A machine that has a theme applied looks like this after a run, which is
// the state the step this file guards exists to repair.
func themeSurvivesHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	for _, asset := range []struct{ repo, installed string }{
		{repoAssetZshrc, ".zshrc"},
		{repoAssetP10k, ".p10k.zsh"},
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), asset.repo))
		if err != nil {
			t.Fatalf("read %s: %v", asset.repo, err)
		}
		if err := os.WriteFile(filepath.Join(home, asset.installed), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", asset.installed, err)
		}
	}
	return home
}

// themeSurvivesRead reads one of the files the guard inspects.
func themeSurvivesRead(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// themeSurvivesRecord writes the record of an applied theme the way the picker
// writes it, in a state directory of this test's own.
func themeSurvivesRecord(t *testing.T, id string) *dotfilesThemeRecord {
	t.Helper()

	rec := &dotfilesThemeRecord{
		Theme:     id,
		Files:     map[string]string{},
		AppliedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := writeDotfilesThemeRecord(rec); err != nil {
		t.Fatalf("write the theme record: %v", err)
	}
	return rec
}

// TestTheAppliedThemeSurvivesAnInstall is the guard for the defect this step
// exists to close. Every step that writes a configuration file writes the file
// this repository ships, and those carry the default theme's block; nothing put
// the applied theme back, so an install left a machine whose record named one
// theme rendering another, and the prompt -- which reads its palette from
// ~/.zshrc and ~/.p10k.zsh -- painted the default colours.
func TestTheAppliedThemeSurvivesAnInstall(t *testing.T) {
	const appliedID = "nocturne"

	t.Setenv("XDG_STATE_HOME", t.TempDir())
	home := themeSurvivesHome(t)
	t.Setenv("HOME", home)

	zshrcPath := filepath.Join(home, ".zshrc")
	p10kPath := filepath.Join(home, ".p10k.zsh")

	// The precondition is the defect itself: the shipped files name the default
	// theme, so a guard that passed here would be proving nothing.
	for _, path := range []string{zshrcPath, p10kPath} {
		if got := themeSurvivesRead(t, path); strings.Contains(got, appliedID) {
			t.Fatalf("%s already names %s, so this guard cannot tell whether the step did anything",
				filepath.Base(path), appliedID)
		}
	}

	record := themeSurvivesRecord(t, appliedID)

	m := &Model{RepoDir: repoRoot(t)}
	if err := stepReapplyTheme(m); err != nil {
		t.Fatalf("stepReapplyTheme: %v", err)
	}

	zshrc := themeSurvivesRead(t, zshrcPath)
	p10k := themeSurvivesRead(t, p10kPath)

	if !strings.Contains(zshrc, themeBeginTag("")+" "+appliedID) {
		t.Errorf("~/.zshrc does not name %s after the step: the shell palette and the bat selection %s",
			appliedID, "are whatever the shipped file carried")
	}
	if !strings.Contains(zshrc, `export BAT_THEME="`+appliedID+`"`) {
		t.Errorf("BAT_THEME was not set to %s, so bat renders a theme the machine did not apply", appliedID)
	}
	if !strings.Contains(p10k, themeBeginTag("")+" "+appliedID) {
		t.Errorf("~/.p10k.zsh does not name %s after the step: the prompt keeps the default palette", appliedID)
	}

	// The palette the shipped block carries is the default theme's, so the value
	// is checked against the definition rather than against a literal.
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	applied, ok := themeByID(defs, appliedID)
	if !ok {
		t.Fatalf("the %s theme is not in the repository", appliedID)
	}
	if want := `typeset -g PALETTE_BLUE="` + applied.Palette["blue"] + `"`; !strings.Contains(p10k, want) {
		t.Errorf("~/.p10k.zsh does not carry the applied palette: %s is missing", want)
	}

	// The record belongs to the picker and its undo, not to an install.
	after := readDotfilesThemeRecord()
	if after == nil {
		t.Fatal("the step removed the theme record")
	}
	if after.Theme != appliedID {
		t.Errorf("the record now names %q, want %q", after.Theme, appliedID)
	}
	if !after.AppliedAt.Equal(record.AppliedAt) {
		t.Errorf("the step rewrote the record's timestamp: %s became %s, which would make the picker's own report a lie",
			record.AppliedAt, after.AppliedAt)
	}
}

// TestTheThemeStepLeavesTheFilesAloneWithoutARecord covers the two cases where the
// step has nothing to do. Both have to leave the files exactly as the install left
// them and neither may fail the run: a machine that applied no theme is the normal
// case, and a checkout that predates a recorded theme is a normal race between the
// installed binary and the branch it reads, not a reason to fail an install that
// has already done its work.
func TestTheThemeStepLeavesTheFilesAloneWithoutARecord(t *testing.T) {
	cases := []struct {
		name   string
		record string // empty means no record is written at all
	}{
		{name: "no record"},
		{name: "a record naming a theme the checkout does not define", record: "a-theme-this-checkout-does-not-define"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			home := themeSurvivesHome(t)
			t.Setenv("HOME", home)

			if tc.record != "" {
				themeSurvivesRecord(t, tc.record)
			}

			beforeZshrc := themeSurvivesRead(t, filepath.Join(home, ".zshrc"))
			beforeP10k := themeSurvivesRead(t, filepath.Join(home, ".p10k.zsh"))

			m := &Model{RepoDir: repoRoot(t)}
			if err := stepReapplyTheme(m); err != nil {
				t.Fatalf("the step failed a run it had nothing to do for: %v", err)
			}

			if got := themeSurvivesRead(t, filepath.Join(home, ".zshrc")); got != beforeZshrc {
				t.Error("~/.zshrc was changed by a step that had no theme to restore")
			}
			if got := themeSurvivesRead(t, filepath.Join(home, ".p10k.zsh")); got != beforeP10k {
				t.Error("~/.p10k.zsh was changed by a step that had no theme to restore")
			}
		})
	}
}

// TestTheThemeStepRunsAfterTheStepsThatWriteConfigurations is the guard for where
// the step sits, which is the whole of its value: it repairs files the steps above
// replaced, so a step that runs earlier repairs nothing, and one that runs after
// cleanup has no checkout to read the theme definitions from. Both planners are
// swept over the configuration space the installer supports, because a step added
// to one and forgotten in the other is a defect this path has produced before.
func TestTheThemeStepRunsAfterTheStepsThatWriteConfigurations(t *testing.T) {
	// The steps that write the configuration files this step repairs. Cleanup is
	// not one of them; it is the bound on the other side, because it removes the
	// temporary checkout the definitions are read from.
	writers := []string{"terminal", "shell", "wm", "nvim", "wslconfig"}

	checked := 0
	for _, sysInfo := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			plans := map[string][]InstallStep{}

			nonInteractive := &Model{
				SystemInfo:      sysInfo,
				Choices:         choices,
				ExistingConfigs: []string{".config/nvim"},
			}
			plans["buildStepsForChoices"] = buildStepsForChoices(nonInteractive)

			interactive := Model{
				SystemInfo:      sysInfo,
				Choices:         choices,
				ExistingConfigs: []string{".config/nvim"},
			}
			interactive.SetupInstallSteps()
			plans["SetupInstallSteps"] = interactive.Steps

			for name, steps := range plans {
				index := map[string]int{}
				for i, step := range steps {
					index[step.ID] = i
				}

				themeAt, scheduled := index["theme"]
				if !scheduled {
					t.Fatalf("%s (%v, %s) does not schedule the theme step at all",
						name, sysInfo.OS, choices.Shell)
				}
				checked++

				for _, writer := range writers {
					at, present := index[writer]
					if !present {
						continue
					}
					if themeAt < at {
						t.Errorf("%s (%v, %s) runs the theme step at %d, before %q at %d: it would repair a file that step has not written yet",
							name, sysInfo.OS, choices.Shell, themeAt, writer, at)
					}
				}

				if cleanupAt, present := index["cleanup"]; present && themeAt > cleanupAt {
					t.Errorf("%s (%v, %s) runs the theme step at %d, after cleanup at %d, which removes the checkout the theme definitions are read from",
						name, sysInfo.OS, choices.Shell, themeAt, cleanupAt)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no plan was checked, so this guard proves nothing")
	}
}
