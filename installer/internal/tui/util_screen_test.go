package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// The utilities section: the WSL resource utility
// ---------------------------------------------------------------------------
//
// The utility is the menu entry to what the installation step already does:
// both routes render the shipped dotfiles-wsl/.wslconfig.tmpl for the Windows
// host this installer runs on, merge it over whatever the user's .wslconfig
// holds so their own keys survive, back the previous file up and write with the
// same writer. The guards below pin the three things that can go wrong: offering
// the utility where there is no .wslconfig to edit, showing a recommendation that
// does not come from the host, and letting the two routes drift into two
// implementations.

// wslResourceTestState is the state the screen is drawn from with the host and
// the file pinned, so a guard measures the utility rather than whatever Windows
// host the runner happens to be on.
func wslResourceTestState(t *testing.T) wslResourceState {
	t.Helper()

	return wslResourceState{
		Resolved:  true,
		Available: true,
		Path:      "/mnt/c/Users/alber/.wslconfig",
		RepoDir:   repoRoot(t),
		Host:      system.HostResources{MemoryBytes: 16 * testGiB, LogicalCPUs: 8},
		Plan:      WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
		Current:   WSLResources{MemoryMB: 4096, Processors: 0, SwapMB: 1024},
		HasFile:   true,
		Draft:     WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
	}
}

// TestUtilitiesSectionOffersTheWSLResourcesOnlyOnWSL pins the availability rule:
// the row exists where a .wslconfig exists, and is absent on Linux, macOS and
// Termux, where the section's own body says why rather than leaving a hole the
// user has to guess at.
func TestUtilitiesSectionOffersTheWSLResourcesOnlyOnWSL(t *testing.T) {
	tests := []struct {
		name    string
		info    *system.SystemInfo
		wantRow bool
	}{
		{"a WSL host", &system.SystemInfo{OS: system.OSWSL, IsWSL: true, OSName: "WSL"}, true},
		{"a Linux host", &system.SystemInfo{OS: system.OSLinux, OSName: "Linux"}, false},
		{"macOS", &system.SystemInfo{OS: system.OSMac, OSName: "macOS"}, false},
		{"Termux", &system.SystemInfo{OS: system.OSTermux, IsTermux: true, OSName: "Termux"}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel()
			m.SystemInfo = tt.info
			m.Screen = ScreenUtilities
			m.WSLState = wslResourceTestState(t)
			if !tt.wantRow {
				// The unreachable half of the resolution: on a host without WSL
				// the read never finds a destination.
				m.WSLState = wslResourceState{Resolved: true, Available: false,
					Reason: "no .wslconfig exists outside WSL"}
			}

			found := false
			for _, row := range m.GetCurrentOptions() {
				if row == utilitiesWSLRow {
					found = true
				}
			}
			if found != tt.wantRow {
				t.Errorf("the utilities section offers %q = %v, want %v", utilitiesWSLRow, found, tt.wantRow)
			}

			body := strings.Join(m.utilitiesDescription(), " ")
			if tt.wantRow {
				if strings.Contains(body, "no .wslconfig") {
					t.Errorf("a WSL host's section declared the utility unavailable: %q", body)
				}
				return
			}
			// A hole the user cannot see the reason for is the failure this
			// assertion exists to prevent.
			if !strings.Contains(body, "WSL") || !strings.Contains(body, tt.info.OSName) {
				t.Errorf("the section does not declare why the WSL resources cannot be adjusted here: %q", body)
			}
		})
	}
}

// TestWSLResourcesAreOfferedFromTheInstalledCopyWithoutACheckout is the user's
// case: the installer has been run once, so it left a copy of the shipped WSL
// template in the per-user data directory, and the program is now launched from
// a working directory that is not a checkout, with no clone and nothing under
// ~/dotfiles. The installed copy is the last candidate, and it is what makes the
// resources utility offered from anywhere.
func TestWSLResourcesAreOfferedFromTheInstalledCopyWithoutACheckout(t *testing.T) {
	// The Windows destination and the host capacities are pinned by the same
	// fixture the other WSL guards use, so the resolution is exercised without a
	// Windows host.
	newWSLLayout(t)

	// A prior install: the shipped template sits where the clone step copies it,
	// under the per-user data directory.
	shipped, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetWSLConfig))
	if err != nil {
		t.Fatal(err)
	}
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	installed := filepath.Join(dataHome, stateAppDir, repoAssetWSLConfig)
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, shipped, 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: true, OSName: "WSL"}
	m.Screen = ScreenUtilities
	m.WSLState = loadWSLResourceState("", true)

	if !m.WSLState.Available {
		t.Fatalf("the resources utility was not offered without a checkout: %s", m.WSLState.Reason)
	}
	if want := filepath.Join(dataHome, stateAppDir); m.WSLState.RepoDir != want {
		t.Errorf("resolved the template in %q, want the installed copy %q", m.WSLState.RepoDir, want)
	}
	found := false
	for _, row := range m.GetCurrentOptions() {
		if row == utilitiesWSLRow {
			found = true
		}
	}
	if !found {
		t.Errorf("the utilities section does not offer %q from the installed copy", utilitiesWSLRow)
	}
}

// TestInstallWSLTemplateCopiesOnlyMissingOrDifferent pins the copy rules the
// installed template follows: the shipped template is written when it is missing
// or differs, an identical one is left as it is, and a file the repository does
// not ship is never touched. Nothing is ever deleted, so a file an earlier run
// left behind stays.
func TestInstallWSLTemplateCopiesOnlyMissingOrDifferent(t *testing.T) {
	repoDir, _, _ := newWSLLayout(t)
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	dest := filepath.Join(dataHome, stateAppDir, repoAssetWSLConfig)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(filepath.Dir(dest), "mine.conf")
	if err := os.WriteFile(userFile, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	copied, current, gotDest, err := installWSLTemplate(repoDir)
	if err != nil {
		t.Fatalf("install the WSL template: %v", err)
	}
	if copied != 1 || current != 0 {
		t.Errorf("the first copy wrote %d and kept %d, want 1 written and 0 current", copied, current)
	}
	if gotDest != dest {
		t.Errorf("wrote to %q, want %q", gotDest, dest)
	}
	shipped, err := os.ReadFile(filepath.Join(repoDir, repoAssetWSLConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, shipped) {
		t.Errorf("the differing shipped template was not refreshed: %q", got)
	}
	if got, _ := os.ReadFile(userFile); string(got) != "mine\n" {
		t.Errorf("a file the repository does not ship was touched: %q", got)
	}

	// A second run with nothing new writes nothing and says what is current.
	copied, current, _, err = installWSLTemplate(repoDir)
	if err != nil {
		t.Fatalf("install the WSL template a second time: %v", err)
	}
	if copied != 0 || current != 1 {
		t.Errorf("the second copy wrote %d and kept %d, want 0 written and 1 current", copied, current)
	}
}

// TestWSLTemplateResolutionReportsWhereItLooked pins the honest failure: with no
// candidate holding the shipped template, resolution fails and names the
// installed-copy directory the clone step writes to, so a user who launches the
// program from anywhere knows where the template was sought rather than being
// handed an unexplained missing row.
func TestWSLTemplateResolutionReportsWhereItLooked(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, err := resolveWSLTemplateDir("")
	if err == nil {
		t.Fatal("expected resolution to fail when nothing holds the template")
	}
	if !strings.Contains(err.Error(), repoAssetWSLConfig) {
		t.Errorf("the failure does not name what it looked for: %v", err)
	}
	if want := filepath.Join(dataHome, stateAppDir); !strings.Contains(err.Error(), want) {
		t.Errorf("the failure does not name the installed-copy directory %q: %v", want, err)
	}
}

// TestScreenWSLResourcesShowsTheHostNumbersAndTheRecommendation is the screen's
// own guard: the values on screen come from the host capacities and the file,
// and the screen names all three of them plus the one action it does not take.
func TestScreenWSLResourcesShowsTheHostNumbersAndTheRecommendation(t *testing.T) {
	m := installerFrameModel(t, ScreenWSLResources)
	// The row is offered on a WSL host and nowhere else, so the frame this guard
	// measures is the WSL one rather than whatever the runner detects.
	m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: true, OSName: "WSL"}
	m.WSLState = wslResourceTestState(t)
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"16384 MiB",                     // the host's RAM, in the unit WSL uses
		"8 logical processors",          // the host's CPUs
		"8192 MB",                       // the recommended memory, half the host
		"2048 MB",                       // the recommended swap, a quarter of it
		"4096MB",                        // what the file holds today, as the file writes it
		"/mnt/c/Users/alber/.wslconfig", // where the file is
		"wsl --shutdown",                // the effect, and the command the screen does not run
		"Memory: 8192 MB",               // the draft, as a row
		"Processors: 8",                 // the draft, as a row
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the WSL resources screen does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "{{") {
		t.Errorf("a template delimiter reached the screen:\n%s", view)
	}
}

// TestWSLResourceScreenAdjustsTheDraftAndReturnsToTheRecommendation pins the
// editing loop: the arrow keys move the value under the cursor by one step, and
// the recommendation is one row away rather than lost.
func TestWSLResourceScreenAdjustsTheDraftAndReturnsToTheRecommendation(t *testing.T) {
	m := installerFrameModel(t, ScreenWSLResources)
	m.WSLState = wslResourceTestState(t)

	press := func(m Model, key tea.KeyType) Model {
		next, _ := m.Update(tea.KeyMsg{Type: key})
		return next.(Model)
	}
	pressRune := func(m Model, r rune) Model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		return next.(Model)
	}

	m.Cursor = wslResourceRowMemory
	m = press(m, tea.KeyRight)
	if got := m.WSLState.Draft.MemoryMB; got != 8192+wslMemoryStepMB {
		t.Errorf("memory after one step right = %d, want %d", got, 8192+wslMemoryStepMB)
	}
	m = press(m, tea.KeyLeft)
	if got := m.WSLState.Draft.MemoryMB; got != 8192 {
		t.Errorf("memory after one step back = %d, want the recommendation 8192", got)
	}
	// Left at the recommendation is not a floor: it goes down, because the
	// recommendation is a starting point rather than a minimum.
	m = press(m, tea.KeyLeft)
	if got := m.WSLState.Draft.MemoryMB; got != 8192-wslMemoryStepMB {
		t.Errorf("memory below the recommendation = %d, want %d", got, 8192-wslMemoryStepMB)
	}

	// The processors row moves by one CPU, not by a memory step.
	m.Cursor = wslResourceRowProcessors
	m = press(m, tea.KeyRight)
	if got := m.WSLState.Draft.Processors; got != 9 {
		t.Errorf("processors after one step right = %d, want 9", got)
	}

	// The recommendation is reachable again without remembering the numbers.
	m.Cursor = wslResourceRowMemory
	m = pressRune(m, 'r')
	if got := m.WSLState.Draft; got != m.WSLState.Plan {
		t.Errorf("draft after the recommendation key = %+v, want the plan %+v", got, m.WSLState.Plan)
	}
}

// TestWSLResourceWriteIsDryRunGated is the same regression guard the theme
// utility carries: --dry-run documents a run that changes nothing, so the write
// path is gated exactly as executeStep is.
func TestWSLResourceWriteIsDryRunGated(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")

	repoDir, _ := writeWSLUtilityRepo(t)
	dst := filepath.Join(t.TempDir(), ".wslconfig")
	before := "[wsl2]\nmemory=32GB\n"
	if err := os.WriteFile(dst, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	st := wslResourceState{
		Resolved: true, Available: true, Path: dst, RepoDir: repoDir,
		Host:  system.HostResources{MemoryBytes: 16 * testGiB, LogicalCPUs: 8},
		Plan:  WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
		Draft: WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
	}

	notice, wrote, err := writeWSLResourceDraft(st, "utilities")
	if err != nil {
		t.Fatalf("a dry run returned an error: %v", err)
	}
	if wrote {
		t.Error("a dry run reported that it wrote the file")
	}
	if !strings.Contains(notice, "DRY RUN") {
		t.Errorf("the notice does not say the run changed nothing: %q", notice)
	}
	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Errorf("a dry run changed the file:\n%s", after)
	}
	backups, _ := filepath.Glob(dst + ".bak-dotfiles-*")
	if len(backups) != 0 {
		t.Errorf("a dry run wrote a backup: %v", backups)
	}
}

// TestWSLResourceWritePreservesTheKeysThatAreNotOurs pins the defect this
// repository already paid for once: the user's .wslconfig holds settings this
// installer knows nothing about, and a write must update the managed keys
// without rewriting the file around them. The previous file is backed up.
func TestWSLResourceWritePreservesTheKeysThatAreNotOurs(t *testing.T) {
	repoDir, _ := writeWSLUtilityRepo(t)
	dst := filepath.Join(t.TempDir(), ".wslconfig")
	before := "[wsl2]\n" +
		"# my own kernel\n" +
		"kernel=C:\\\\kernel\n" +
		"memory=32GB\n" +
		"[experimental]\n" +
		"useWindowsDnsCache=true\n"
	if err := os.WriteFile(dst, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	st := wslResourceState{
		Resolved: true, Available: true, Path: dst, RepoDir: repoDir,
		Host:  system.HostResources{MemoryBytes: 16 * testGiB, LogicalCPUs: 8},
		Plan:  WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
		Draft: WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
	}

	if _, _, err := writeWSLResourceDraft(st, "utilities"); err != nil {
		t.Fatalf("the write failed: %v", err)
	}

	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)
	for _, kept := range []string{"# my own kernel", "kernel=C:\\\\kernel", "useWindowsDnsCache=true"} {
		if !strings.Contains(got, kept) {
			t.Errorf("the write dropped the user's own line %q:\n%s", kept, got)
		}
	}
	for _, managed := range []string{"memory=8192MB", "processors=8", "swap=2048MB"} {
		if !strings.Contains(got, managed) {
			t.Errorf("the write did not apply %q:\n%s", managed, got)
		}
	}
	if strings.Contains(got, "memory=32GB") {
		t.Errorf("the superseded managed key is still there:\n%s", got)
	}

	backups, err := filepath.Glob(dst + ".bak-dotfiles-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("the write left %d backups, want the previous file saved once: %v", len(backups), backups)
	}
	saved, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != before {
		t.Errorf("the backup does not hold the previous file:\n%s", saved)
	}
}

// TestWSLResourceUtilityAndTheInstallerAgreeByteForByte is the agreement guard
// the two routes exist under: the installation step and the utility, given the
// same host and the same pre-existing file, must produce the same bytes. The
// values and the writer may live in exactly one place; a second calculation or a
// second writer on either route breaks this test rather than drifting silently
// until two machines disagree about the same host.
//
// The host capacities are pinned through the same overrides the WSL step tests
// use, so this is a statement about the routes rather than about the runner.
func TestWSLResourceUtilityAndTheInstallerAgreeByteForByte(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())

	repoDir, winHome, _ := newWSLLayout(t)
	existing := "[wsl2]\n" +
		"# my own kernel\n" +
		"kernel=C:\\\\kernel\n" +
		"memory=32GB\n" +
		"[experimental]\n" +
		"useWindowsDnsCache=true\n"

	// The installation route: the real step, into the Windows profile the step
	// resolves.
	installerDst := filepath.Join(winHome, ".wslconfig")
	if err := os.WriteFile(installerDst, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	m := wslModel(repoDir, true)
	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("the installation step failed: %v", err)
	}

	// The utilities route: the same pre-existing file, the same host, the
	// recommended values the screen starts from.
	utilDst := filepath.Join(t.TempDir(), ".wslconfig")
	if err := os.WriteFile(utilDst, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	host := system.DetectHostResources()
	st := wslResourceState{
		Resolved: true, Available: true, Path: utilDst, RepoDir: repoDir,
		Host: host, Plan: PlanWSLResources(host), Draft: PlanWSLResources(host),
	}
	if _, _, err := writeWSLResourceDraft(st, "utilities"); err != nil {
		t.Fatalf("the utilities write failed: %v", err)
	}

	fromInstaller, err := os.ReadFile(installerDst)
	if err != nil {
		t.Fatal(err)
	}
	fromUtility, err := os.ReadFile(utilDst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fromInstaller, fromUtility) {
		t.Errorf("the two routes disagree about the same host and the same file:\n--- installer ---\n%s\n--- utilities ---\n%s",
			fromInstaller, fromUtility)
	}
	if strings.Contains(string(fromInstaller), "32GB") {
		t.Errorf("neither route updated the managed memory key:\n%s", fromInstaller)
	}
}

// writeWSLUtilityRepo builds a throwaway checkout holding the shipped WSL
// template, for the utility guards that write through the shared content
// builder. It is the utility-side counterpart of newWSLLayout, which pins the
// destinations the installation step resolves.
func writeWSLUtilityRepo(t *testing.T) (repoDir, templatePath string) {
	t.Helper()

	repoDir = t.TempDir()
	templatePath = filepath.Join(repoDir, repoAssetWSLConfig)
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	shipped, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetWSLConfig))
	if err != nil {
		t.Fatalf("reading the shipped WSL template: %v", err)
	}
	if err := os.WriteFile(templatePath, shipped, 0o644); err != nil {
		t.Fatal(err)
	}
	return repoDir, templatePath
}

// TestTheRecommendationComesFromTheHost is the teeth behind the recommendation:
// two different machines must produce two different plans from the same code.
// The values are the host's, never a number typed into the screen.
func TestTheRecommendationComesFromTheHost(t *testing.T) {
	small := PlanWSLResources(system.HostResources{MemoryBytes: 8 * testGiB, LogicalCPUs: 4})
	large := PlanWSLResources(system.HostResources{MemoryBytes: 32 * testGiB, LogicalCPUs: 16})

	if small.MemoryMB != 4096 || small.Processors != 4 {
		t.Errorf("an 8 GiB / 4 CPU host plans %+v, want 4096 MB and 4 processors", small)
	}
	if large.MemoryMB != 16384 || large.Processors != 16 {
		t.Errorf("a 32 GiB / 16 CPU host plans %+v, want 16384 MB and 16 processors", large)
	}
	if small == large {
		t.Error("two different hosts produced the same plan")
	}
}

// TestTheInteractiveRoutePreservesTheUsersOwnKeys pins one property, and it is
// the whole reason the merge lives where it lives: **the interactive route
// preserves the keys the user wrote**.
//
// It has to, because that route never writes through Go: the generated script
// copies the bytes mergedRepoWSLConfig returns straight over the destination, so
// those bytes are the file the machine ends up with. Without the merge inside
// that function the interactive route -- which is the route a TUI install
// actually takes, because /etc/wsl.conf needs sudo -- would overwrite keys the
// user set, while the non-interactive step and the utilities section kept them.
// The merge cannot be moved to the point of writing instead: there is no Go
// write on this route to move it to.
//
// The guard reads the bytes the script is pointed at rather than running the
// shell, so it checks the content the same way the existing route guard does.
func TestTheInteractiveRoutePreservesTheUsersOwnKeys(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())

	repoDir, winHome, _ := newWSLLayout(t)
	existing := "[wsl2]\n" +
		"# my own kernel\n" +
		"kernel=C:\\\\kernel\n" +
		"memory=32GB\n"
	if err := os.WriteFile(filepath.Join(winHome, ".wslconfig"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	m := wslModel(repoDir, true)
	script := generateWSLConfigScript(t, &m)

	rendered, err := os.ReadFile(wslConfigSourceFromScript(t, script))
	if err != nil {
		t.Fatalf("reading the content the interactive script copies: %v", err)
	}
	got := string(rendered)

	for _, kept := range []string{"# my own kernel", "kernel=C:\\\\kernel"} {
		if !strings.Contains(got, kept) {
			t.Errorf("the interactive route would drop the user's own line %q:\n%s", kept, got)
		}
	}
	if !strings.Contains(got, "memory=4096MB") {
		t.Errorf("the interactive route did not apply the host's memory:\n%s", got)
	}
	if strings.Contains(got, "memory=32GB") {
		t.Errorf("the interactive route would keep the superseded memory key:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// The main menu's panel, for the Utilities row
// ---------------------------------------------------------------------------

// utilitiesPanelFlat renders the main-menu panel with the cursor on the
// Utilities row -- the frame that describes the section -- and returns its rows
// as one escape-free string. The cursor is found by label rather than by index,
// because the menu's rows move when a backup exists.
func utilitiesPanelFlat(t *testing.T, m Model) string {
	t.Helper()

	m.Screen = ScreenMainMenu
	m.Cursor = -1
	for i, option := range m.GetCurrentOptions() {
		if strings.Contains(option, "Utilities") {
			m.Cursor = i
			break
		}
	}
	if m.Cursor < 0 {
		t.Fatal("the main menu no longer holds a Utilities row")
	}
	return panelFlat(m.mainMenuPanel(narrowPanelLayout(), 200))
}

// TestUtilitiesPanelNamesTheWSLResourcesWithTheHostsValues pins the panel's half
// of the section's contract: the Utilities panel describes the section, so a user
// reading the panel before opening it sees the row the section offers and what
// the values on it would be -- the recommendation already derived from this host,
// not a number invented beside it.
func TestUtilitiesPanelNamesTheWSLResourcesWithTheHostsValues(t *testing.T) {
	m := contextualMainMenuModel()
	m.WSLState = wslResourceTestState(t)

	flat := utilitiesPanelFlat(t, m)
	for _, want := range []string{
		"WSL resources",
		"memory 8192 MB",
		"processors 8",
		"swap 2048 MB",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the Utilities panel does not name %q:\n%s", want, flat)
		}
	}
	if strings.Contains(flat, "not adjustable") {
		t.Errorf("the panel says the WSL resources are not adjustable while the section offers them:\n%s", flat)
	}
}

// TestUtilitiesPanelSaysWhenTheWSLResourcesAreNotOffered pins the other half: on
// a host with no .wslconfig the panel says the utility is not adjustable rather
// than leaving a hole or naming a row the section does not draw. The long reason
// (which names the platform) stays in the section's own body; the panel is a
// summary and says the short version.
func TestUtilitiesPanelSaysWhenTheWSLResourcesAreNotOffered(t *testing.T) {
	m := contextualMainMenuModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, OSName: "Linux"}
	m.WSLState = wslResourceState{Resolved: true}

	flat := utilitiesPanelFlat(t, m)
	if !strings.Contains(flat, "The WSL resources are not adjustable here.") {
		t.Errorf("the panel does not say the WSL resources are unavailable:\n%s", flat)
	}
	if strings.Contains(flat, "memory ") {
		t.Errorf("the panel names values for a utility the section does not offer:\n%s", flat)
	}
}

// TestUtilitiesPanelFactsAreDerivedFromTheModelState is the panel's derivation
// guard. Every fact has to come from the field the section reads, so turning that
// field on changes the panel and turning it off takes the fact away: a fact typed
// into the panel instead -- the second list this repository already paid for once
// -- cannot survive this, and a utility added to the section has nowhere to be
// named but utilitiesPanelEntries.
func TestUtilitiesPanelFactsAreDerivedFromTheModelState(t *testing.T) {
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}

	empty := contextualMainMenuModel()
	empty.SystemInfo = &system.SystemInfo{OS: system.OSLinux, OSName: "Linux"}
	// The shell audit is forced unavailable rather than left to the runner: a run
	// whose stdout happens to be a terminal would otherwise offer the row and
	// change this panel with the machine.
	empty.ShellAudit = shellAuditState{Resolved: true, Reason: "the login shell is not named: $SHELL is empty"}
	emptyFlat := utilitiesPanelFlat(t, empty)
	t.Logf("utilities panel, nothing offered:\n%s", emptyFlat)
	for _, absent := range []string{
		"No desktop theme switch is available here.",
		"The dotfiles' own theme is not switchable here.",
		"The WSL resources are not adjustable here.",
		"The shell's startup is not measurable here.",
	} {
		if !strings.Contains(emptyFlat, absent) {
			t.Errorf("an empty model's panel does not declare %q:\n%s", absent, emptyFlat)
		}
	}

	withSwitch := contextualMainMenuModel()
	withSwitch.ThemeSwitch, withSwitch.ThemeSwitchFound = target, true
	switchFlat := utilitiesPanelFlat(t, withSwitch)
	t.Logf("utilities panel, a detected switch:\n%s", switchFlat)
	if !strings.Contains(switchFlat, "Switch") || !strings.Contains(switchFlat, "GNOME") {
		t.Errorf("the panel did not pick up the detected switch:\n%s", switchFlat)
	}

	withRecord := contextualMainMenuModel()
	withRecord.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	recordFlat := utilitiesPanelFlat(t, withRecord)
	t.Logf("utilities panel, a dotfiles-theme record:\n%s", recordFlat)
	if !strings.Contains(recordFlat, "Themes") || !strings.Contains(recordFlat, "undo available") {
		t.Errorf("the panel did not pick up the dotfiles-theme record:\n%s", recordFlat)
	}

	withWSL := contextualMainMenuModel()
	withWSL.WSLState = wslResourceTestState(t)
	wslFlat := utilitiesPanelFlat(t, withWSL)
	t.Logf("utilities panel, WSL resource state:\n%s", wslFlat)
	if !strings.Contains(wslFlat, "WSL resources") || !strings.Contains(wslFlat, "8192 MB") {
		t.Errorf("the panel did not pick up the WSL resource state:\n%s", wslFlat)
	}

	withAudit := contextualMainMenuModel()
	withAudit.ShellAudit = shellAuditTestState()
	auditFlat := utilitiesPanelFlat(t, withAudit)
	t.Logf("utilities panel, a finished shell startup measurement:\n%s", auditFlat)
	if !strings.Contains(auditFlat, "Shell start") || !strings.Contains(auditFlat, "median") {
		t.Errorf("the panel did not pick up the shell startup measurement:\n%s", auditFlat)
	}

	// The five panels are five different answers to the same question, which is
	// what makes them facts about the model rather than text about the section.
	seen := map[string]string{
		emptyFlat:  "an empty model",
		switchFlat: "a detected switch",
		recordFlat: "a dotfiles-theme record",
		wslFlat:    "WSL resource state",
		auditFlat:  "shell startup state",
	}
	if len(seen) != 5 {
		t.Errorf("two of the five model states produced the same panel, so a fact is not read from the model: %d distinct panels", len(seen))
	}
}

// These tests cover the third ownership proof: a file with no ownership marker,
// no generated block and no byte-identity to what the repository ships, but
// whose hand-written region carries the anchors the generator knows. The switch
// must adopt only that region and leave every other byte exactly as it was.

// userForeignPrefix and userForeignSuffix are the bytes a user's own file holds
// around the managed region - comments, keys and order dotfiles never wrote. The
// guards below assert they survive the apply byte-for-byte.
const (
	userForeignPrefix = "# my own configuration, written by hand\nset -x MY_OWN_KEY 1\n\n"
	userForeignSuffix = "# --- my own footer ---\nmy_own_function() { echo mine; }\n"
)

// userLegacyStarshipPalettes is the hand-written Starship palette table an
// older checkout installed, transcribed from
// git show b16e4ac^:starship.toml. Its [palettes.catppuccin_mocha] header is the
// region anchor; the generator no longer writes that table (nor these uppercase
// values), so byte-identity cannot prove the file and only the anchors can.
const userLegacyStarshipPalettes = `[palettes.catppuccin_mocha]
rosewater = "#f5e0dc"
flamingo = "#f2cdcd"
pink = "#f5c2e7"
mauve = "#cba6f7"
red = "#f38ba8"
maroon = "#eba0ac"
peach = "#fab387"
yellow = "#f9e2af"
green = "#a6e3a1"
teal = "#94e2d5"
sky = "#89dceb"
sapphire = "#74c7ec"
blue = "#89b4fa"
lavender = "#b4befe"
text = "#cdd6f4"
subtext1 = "#bac2de"
subtext0 = "#a6adc8"
overlay2 = "#9399b2"
overlay1 = "#7f849c"
overlay0 = "#6c7086"
surface2 = "#585b70"
surface1 = "#45475a"
surface0 = "#313244"
base = "#1e1e2e"
mantle = "#181825"
crust = "#11111b"

[palettes.dotfiles]
text = "#F3F6F9"
red = "#CB7C94"
green = "#B7CC85"
yellow = "#FFE066"
blue = "#7FB4CA"
mauve = "#A3B5D6"
pink = "#FF8DD7"
teal = "#7AA89F"
peach = "#DEBA87"
subtext0 = "#5C6170"
overlay0 = "#232A40"
rosewater = "#E0C15A"
flamingo = "#FF8DD7"
maroon = "#C4746E"
lavender = "#B99BF2"
subtext1 = "#8A8FA3"
overlay2 = "#313342"
overlay1 = "#191E28"
surface2 = "#27345C"
surface1 = "#232A40"
surface0 = "#191E28"
base = "none"
mantle = "#06080f"
crust = "#06080f"`

// userLegacyFishRegion is the hand-written fish palette an older checkout
// installed, transcribed from
// git show 758790b^:dotfiles-fish/fish/config.fish. The generator writes
// `set -g fish_color_normal f3f6f9` today, so none of this file matches what the
// repository ships; only the region anchors identify it.
const userLegacyFishRegion = `set -l foreground F3F6F9 normal
set -l selection 263356 normal
set -l comment 8394A3 brblack
set -l red CB7C94 red
set -l orange DEBA87 orange
set -l yellow FFE066 yellow
set -l green B7CC85 green
set -l purple A3B5D6 purple
set -l cyan 7AA89F cyan
set -l pink FF8DD7 magenta

# Syntax Highlighting Colors
set -g fish_color_normal $foreground
set -g fish_color_command $cyan
set -g fish_color_keyword $pink
set -g fish_color_quote $yellow
set -g fish_color_redirection $foreground
set -g fish_color_end $orange
set -g fish_color_error $red
set -g fish_color_param $purple
set -g fish_color_comment $comment
set -g fish_color_selection --background=$selection
set -g fish_color_search_match --background=$selection
set -g fish_color_operator $green
set -g fish_color_escape $pink
set -g fish_color_autosuggestion $comment

# Completion Pager Colors
set -g fish_pager_color_progress $comment
set -g fish_pager_color_prefix $cyan
set -g fish_pager_color_completion $foreground
set -g fish_pager_color_description $comment`

// userHerdrFixture is the user's Herdr config: the hand-written region the
// repository shipped before the generator, with the file's own keys and order
// around it. The region is the generated block with the marker and tag lines
// stripped, which is exactly the hand-written shape the anchors bracket.
func userHerdrFixture(t *testing.T) string {
	t.Helper()
	art := artifactByName(t, "herdr")
	def, ok := themeByID(mustLoadDefinitions(t), defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}
	return userForeignPrefix + legacyThemeFile(t, art, def) + userForeignSuffix
}

func mustLoadDefinitions(t *testing.T) []themeDefinition {
	t.Helper()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	return defs
}

// applyTarget loads the repository definitions and returns catppuccin-mocha,
// the theme the switch tests apply on top of the dotfiles fixtures.
func applyTarget(t *testing.T) themeDefinition {
	t.Helper()
	def, ok := themeByID(mustLoadDefinitions(t), "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}
	return def
}

// TestThemeRegionAdoptionRewritesOnlyTheManagedRegion is the user's exact case:
// an unmarked file that has drifted and carries no generated block, but whose
// hand-written region still holds the anchors the generator knows. Today the
// switch refuses it; it must instead rewrite the region between the anchors and
// leave the bytes around it untouched.
func TestThemeRegionAdoptionRewritesOnlyTheManagedRegion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)
	original := userHerdrFixture(t)
	writeThemeFileAt(t, dst, original)

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to a file whose region carries the anchors: %v", err)
	}
	if rec == nil {
		t.Fatal("the region adoption returned no record, so the change is not reversible")
	}
	if strings.Contains(notice, "DRY RUN") {
		t.Fatalf("a real run reported a dry run: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the adopted file carries no generated block:\n%s", got)
	}
	if !strings.Contains(string(got), target.Palette["selection"]) {
		t.Errorf("the region was not rewritten with the %s theme (no %s)", target.Name, target.Palette["selection"])
	}
	// The teeth: the bytes outside the anchors are preserved exactly. A rewrite
	// that reformatted, reordered or dropped the surrounding file fails here.
	if !strings.HasPrefix(string(got), userForeignPrefix) {
		t.Errorf("the bytes before the region changed:\n%s", got)
	}
	if !strings.HasSuffix(string(got), userForeignSuffix) {
		t.Errorf("the bytes after the region changed:\n%s", got)
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the region adoption: %v", err)
	}
	back, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, []byte(original)) {
		t.Errorf("undo did not restore the file byte-for-byte\n got: %q\nwant: %q", back, original)
	}
}

// TestThemeRegionAdoptionLeavesForeignContentIntact is the guard with the
// user's own content around the region: their comments, their keys and their
// order. It proves the region adoption writes nothing outside the anchors by
// comparing the exact prefix and suffix bytes before and after.
func TestThemeRegionAdoptionLeavesForeignContentIntact(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "zsh")
	dst := themeInstalledPath(art, home)
	def, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)

	prefix := "# my zsh aliases\nalias ll='ls -la'\nalias gs='git status'\n\nexport MY_EDITOR=nvim\n\n"
	suffix := "\n# my own functions\nmkcd() { mkdir -p \"$1\" && cd \"$1\"; }\n\nsetopt hist_ignore_all_dups\n"
	region := legacyThemeFile(t, art, def)
	original := prefix + region + suffix
	writeThemeFileAt(t, dst, original)

	if _, _, err := applyDotfilesTheme(home, repoRoot(t), target); err != nil {
		t.Fatalf("apply the theme around the user's own .zshrc content: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the zsh region was not adopted:\n%s", got)
	}
	if !strings.HasPrefix(string(got), prefix) {
		t.Errorf("the user's aliases and exports, before the region, were changed:\n%s", got)
	}
	if !strings.HasSuffix(string(got), suffix) {
		t.Errorf("the user's functions and options, after the region, were changed:\n%s", got)
	}
	if !strings.Contains(string(got), "alias ll='ls -la'") || !strings.Contains(string(got), "setopt hist_ignore_all_dups") {
		t.Errorf("the user's own lines are missing from the result:\n%s", got)
	}
}

// TestThemeRegionAdoptionCoversTheUsersFiveFiles is the user's machine in one
// test. Each of the five files they reported has no marker and no generated
// block, and each carried the hand-written region whose anchors the generator
// knows, with the file's own bytes around it. Every one must be adopted and
// every foreign byte preserved.
func TestThemeRegionAdoptionCoversTheUsersFiveFiles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)
	def, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)

	herdrArt := artifactByName(t, "herdr")
	zshArt := artifactByName(t, "zsh")
	p10kArt := artifactByName(t, "p10k")
	paletteArt := artifactByName(t, "starship")

	files := map[string]string{
		themeInstalledPath(herdrArt, home): userForeignPrefix + legacyThemeFile(t, herdrArt, def) + userForeignSuffix,
		themeInstalledPath(zshArt, home):   userForeignPrefix + legacyThemeFile(t, zshArt, def) + userForeignSuffix,
		themeInstalledPath(p10kArt, home):  userForeignPrefix + legacyThemeFile(t, p10kArt, def) + userForeignSuffix,
		themeInstalledPath(paletteArt, home): userForeignPrefix +
			`palette = "dotfiles"` + "\n\n[fill]\nsymbol = ' '\n\n" +
			userLegacyStarshipPalettes + "\n" + userForeignSuffix,
	}
	// fish has no artifact helper that matches it by tool, so it is addressed by
	// its own path and the legacy region the generator no longer reproduces.
	fishPath := themeInstalledPath(themeArtifact{Tool: "fish"}, home)
	files[fishPath] = userForeignPrefix + userLegacyFishRegion + "\n" + userForeignSuffix

	for path, content := range files {
		writeThemeFileAt(t, path, content)
	}

	rec, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to the user's five files: %v", err)
	}
	if rec == nil {
		t.Fatal("the five-file adoption returned no record, so it is not reversible")
	}
	if len(rec.Files) != len(files) {
		t.Errorf("the record holds %d file(s), the switch touched %d", len(rec.Files), len(files))
	}

	for path := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), ">>> dotfiles-theme") {
			t.Errorf("%s was not adopted into the generated form:\n%s", path, got)
		}
		if !strings.HasPrefix(string(got), userForeignPrefix) {
			t.Errorf("%s lost the user's bytes before the region", path)
		}
		if !strings.HasSuffix(string(got), userForeignSuffix) {
			t.Errorf("%s lost the user's bytes after the region", path)
		}
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the five-file adoption: %v", err)
	}
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s was not restored byte-for-byte after undo\n got: %q\nwant: %q", path, got, want)
		}
	}
}

// TestThemeRefusalNamesTheProbesAndTheWayForward pins the message. A refusal
// that says only "not owned" is a closed door with no sign, so it must name the
// file, every proof the switch attempted - the marker, the generated block, the
// region anchors, the byte-identity - and what the user can do next.
func TestThemeRefusalNamesTheProbesAndTheWayForward(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)
	user := "# my own herdr config\nsome_setting = true\n"
	writeThemeFileAt(t, dst, user)

	_, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err == nil {
		t.Fatal("the switch accepted a file that carries none of the proofs")
	}
	msg := err.Error()
	for _, want := range []string{
		dst,
		"not owned",
		themeOwnershipMarker,
		"generated",
		"anchor",
		"identical",
		"reinstall",
		"leave",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, msg)
		}
	}
	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != user {
		t.Error("the refused file was changed")
	}
}

// ---------------------------------------------------------------------------
// The terminal capability utility
// ---------------------------------------------------------------------------
//
// The utility reports what the terminal can do and what each answer means. The
// guards below pin its three obligations: the section offers the row and arms
// the probe only when the screen is opened, the screen shows every answer with
// its source and its consequence, and the main menu's Utilities panel derives
// the terminal row from the model rather than from a second list.

// TestTheUtilitiesSectionOffersTheTerminalReportAndProbesOnlyWhenOpened pins the
// lifecycle rule: the row is in the section, choosing it opens the screen, and
// the probe is armed there -- not at startup, and not synchronously while the
// screen is built.
func TestTheUtilitiesSectionOffersTheTerminalReportAndProbesOnlyWhenOpened(t *testing.T) {
	// Nothing probes at startup: a fresh model has no report and has not run one.
	fresh := NewModel()
	if fresh.TerminalCapabilities.Resolved {
		t.Fatal("a fresh model already holds a terminal report, so something probed at startup")
	}

	m := NewModel()
	m.Screen = ScreenUtilities
	// Keep the section to just the terminal row so the cursor index is stable.
	m.WSLState = wslResourceState{Resolved: true, Available: false, Reason: "no .wslconfig"}
	m.ThemeSwitchFound = false

	cursor := -1
	for i, row := range m.GetCurrentOptions() {
		if row == utilitiesTerminalRow {
			cursor = i
		}
	}
	if cursor < 0 {
		t.Fatalf("the utilities section does not offer %q: %v", utilitiesTerminalRow, m.GetCurrentOptions())
	}
	m.Cursor = cursor

	next, cmd := m.handleUtilitiesKeys("enter")
	got := next.(Model)
	if got.Screen != ScreenTerminalCapabilities {
		t.Fatalf("choosing the terminal row went to %v, want ScreenTerminalCapabilities", got.Screen)
	}
	if got.TerminalCapabilities.Resolved {
		t.Error("the report was resolved synchronously; the probe must run off the update loop")
	}
	if cmd == nil {
		t.Error("choosing the terminal row armed no command, so the report would never be read")
	}
}

// TestTerminalCapabilitiesScreenReportsEveryAnswerWithItsSource is the screen's
// own guard: each capability is named with the answer, where it came from and
// what it implies, and the one answer the probe cannot determine is shown as
// unknown with a reason and a manual check.
func TestTerminalCapabilitiesScreenReportsEveryAnswerWithItsSource(t *testing.T) {
	m := terminalCapabilitiesFrameCase(t)
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"Colour depth",
		colorValueTrueColor,
		"COLORTERM=truecolor",
		"the themes are painted with their exact colours",
		"Clipboard (OSC 52)",
		"supported",
		"copying reaches the clipboard, including over SSH",
		"Synchronized output (mode 2026)",
		"not supported",
		"flicker or tear",
		"Nerd Font glyphs",
		"unknown",
		"look at the marks on this screen",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the capability report does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Writing") || strings.Contains(view, "Write ") {
		t.Errorf("the read-only report mentions a write:\n%s", view)
	}
}

// TestTerminalCapabilitiesScreenIsReadOnly pins the only key the screen offers:
// there is nothing to select or write, so every key either does nothing or steps
// back to the utilities section.
func TestTerminalCapabilitiesScreenIsReadOnly(t *testing.T) {
	m := terminalCapabilitiesFrameCase(t)
	if got := m.GetCurrentOptions(); len(got) != 1 || !strings.Contains(got[0], "Back") {
		t.Fatalf("the report screen offers %v, want only the way back", got)
	}

	next, cmd := m.handleTerminalCapabilitiesKeys("x")
	if cmd != nil {
		t.Error("an unrelated key armed a command on the read-only screen")
	}
	if !next.(Model).TerminalCapabilities.Resolved {
		t.Error("a key cleared the report")
	}

	next, _ = m.handleTerminalCapabilitiesKeys("esc")
	if got := next.(Model).Screen; got != ScreenUtilities {
		t.Errorf("Esc went to %v, want ScreenUtilities", got)
	}
}

// TestUtilitiesPanelTerminalRowComesFromTheReport is the panel's derivation
// guard for the new row: the value is the report's own colour depth, so turning
// the field off changes the panel. A fact typed into the panel cannot survive
// this the way it could not survive the other utilities' guards.
func TestUtilitiesPanelTerminalRowComesFromTheReport(t *testing.T) {
	uninspected := contextualMainMenuModel()
	uninspectedFlat := utilitiesPanelFlat(t, uninspected)
	if !strings.Contains(uninspectedFlat, "Terminal") || !strings.Contains(uninspectedFlat, "not inspected yet") {
		t.Errorf("the panel does not name the terminal report before it is inspected:\n%s", uninspectedFlat)
	}

	inspected := contextualMainMenuModel()
	inspected.TerminalCapabilities = terminalCapabilities{
		Resolved: true,
		Color:    terminalAnswer{State: capabilitySupported, Value: colorValueTrueColor},
	}
	inspectedFlat := utilitiesPanelFlat(t, inspected)
	if !strings.Contains(inspectedFlat, colorValueTrueColor) {
		t.Errorf("the panel did not pick up the inspected colour depth:\n%s", inspectedFlat)
	}
	if strings.Contains(inspectedFlat, "not inspected yet") {
		t.Errorf("the panel still says the terminal was not inspected:\n%s", inspectedFlat)
	}
	if uninspectedFlat == inspectedFlat {
		t.Error("turning the report on did not change the panel, so the row is not read from the model")
	}
}

// TestUtilitiesSectionOffersBothNewUtilitiesAfterTheMerge is the merge's own
// guard. Two features arrived at the same list from opposite sides -- the
// read-only terminal capability report and the shell startup audit -- and the
// merge has to keep both. It renders the section and finds both rows, checks the
// panel's single entry list offers both, and renders the main menu's Utilities
// panel and finds both entries, so a merge that dropped one side fails here
// rather than only shrinking the panel's count.
func TestUtilitiesSectionOffersBothNewUtilitiesAfterTheMerge(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities
	m.Width, m.Height = 120, 40
	// The shell audit is pinned available rather than left to whatever $SHELL the
	// runner has, and the WSL resources are forced away so this guard does not
	// depend on the host either.
	m.ShellAudit = shellAuditTestState()
	m.WSLState = wslResourceState{Resolved: true, Available: false, Reason: "no .wslconfig"}

	rows := m.GetCurrentOptions()
	for _, want := range []string{utilitiesShellAuditRow, utilitiesTerminalRow} {
		found := false
		for _, row := range rows {
			if row == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("the utilities section after the merge does not offer %q: %v", want, rows)
		}
	}

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{utilitiesShellAuditRow, utilitiesTerminalRow} {
		if !strings.Contains(view, want) {
			t.Errorf("the rendered utilities section does not show %q:\n%s", want, view)
		}
	}

	// The panel's single entry list offers both, and the panel names both: the
	// count is len(entries), never a number typed beside it.
	offered := map[string]bool{}
	for _, entry := range m.utilitiesPanelEntries() {
		if entry.offered {
			offered[entry.label] = true
		}
	}
	flat := utilitiesPanelFlat(t, m)
	for _, want := range []string{"Shell start", "Terminal"} {
		if !offered[want] {
			t.Errorf("utilitiesPanelEntries does not offer %q after the merge", want)
		}
		if !strings.Contains(flat, want) {
			t.Errorf("the Utilities panel does not name %q after the merge:\n%s", want, flat)
		}
	}
}

// ---------------------------------------------------------------------------
// The utilities section: the shell startup audit
// ---------------------------------------------------------------------------
//
// The utility is the one that changes nothing: it starts the login shell the way
// a terminal does, several times, reports the median with the range, and names
// the functions zprof blames -- or says plainly that only the total is
// measurable. The guards below pin the offer, the method that travels with the
// number, the timeout reported as a result of its own, and the honest nothing
// when zprof cannot name anyone.

// shellAuditTestState is the state the screen is drawn from with the shell, the
// runs and zprof's table pinned, so a guard measures the utility rather than
// whatever shell the runner happens to have. The numbers are the ones the live
// check on the machine this was written on reported, with one of the five starts
// replaced by a timeout so the row for the starts that did not finish is measured
// too. The summary is computed by the shipped function, so the fixture cannot
// drift from what a real measurement would hold.
func shellAuditTestState() shellAuditState {
	samples := []time.Duration{
		977 * time.Millisecond,
		915 * time.Millisecond,
		872 * time.Millisecond,
		932 * time.Millisecond,
	}
	st := shellAuditState{
		Resolved:  true,
		Available: true,
		Shell:     "zsh",
		ShellPath: "/usr/bin/zsh",
		Command:   "zsh -i -c exit",
		Runs:      shellAuditRuns,
		Timeout:   shellAuditTimeout,
		Measured:  true,
		Samples:   samples,
		Timeouts:  1,
		Attribution: []shellAuditFunction{
			{Name: "compdump", Calls: 1, Total: 725250 * time.Microsecond, Self: 725250 * time.Microsecond, Percent: "42.01%"},
			{Name: "compdef", Calls: 1027, Total: 306380 * time.Microsecond, Self: 306380 * time.Microsecond, Percent: "17.75%"},
			{Name: "_omz_source", Calls: 22, Total: 249880 * time.Microsecond, Self: 244600 * time.Microsecond, Percent: "14.48%"},
			{Name: "compinit", Calls: 1, Total: 1260 * time.Millisecond, Self: 237990 * time.Microsecond, Percent: "73.19%"},
			{Name: "(anon) [/home/testuser/.p10k.zsh:22]", Calls: 1, Total: 33840 * time.Microsecond, Self: 33520 * time.Microsecond, Percent: "1.96%"},
			{Name: "compaudit", Calls: 2, Total: 46620 * time.Microsecond, Self: 46620 * time.Microsecond, Percent: "2.70%"},
		},
	}
	st.Median, st.Fastest, st.Slowest = shellAuditSummary(samples)
	return st
}

// TestUtilitiesSectionOffersTheShellAuditWhereItCanMeasure pins the availability
// rule: the row is there where the login shell and a terminal were resolved, and
// the section's own body names the reason everywhere else -- no login shell, no
// terminal -- rather than leaving a hole the user has to guess at.
func TestUtilitiesSectionOffersTheShellAuditWhereItCanMeasure(t *testing.T) {
	tests := []struct {
		name       string
		state      shellAuditState
		wantRow    bool
		wantReason string
	}{
		{"a shell and a terminal", shellAuditTestState(), true, ""},
		{
			"no login shell",
			shellAuditState{Resolved: true, Reason: "the login shell is not named: $SHELL is empty, so there is no shell to open and nothing to measure."},
			false, "$SHELL",
		},
		{
			"no terminal",
			shellAuditState{Resolved: true, Shell: "zsh", Reason: "this run has no terminal attached, so zsh cannot be started the way a terminal starts it."},
			false, "terminal",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenUtilities
			m.ShellAudit = tt.state

			found := false
			for _, row := range m.GetCurrentOptions() {
				if row == utilitiesShellAuditRow {
					found = true
				}
			}
			if found != tt.wantRow {
				t.Errorf("the utilities section offers %q = %v, want %v", utilitiesShellAuditRow, found, tt.wantRow)
			}

			body := strings.Join(m.utilitiesDescription(), " ")
			if tt.wantRow {
				if !strings.Contains(body, utilitiesShellAuditRow) || !strings.Contains(body, "zsh -i -c exit") {
					t.Errorf("the section does not name the row and the command it runs: %q", body)
				}
				if strings.Contains(body, "not measurable here") {
					t.Errorf("the section declared the audit unavailable while it offers the row: %q", body)
				}
				return
			}
			if !strings.Contains(body, tt.wantReason) || !strings.Contains(body, "not measurable here") {
				t.Errorf("the section does not declare why the shell's startup cannot be measured: %q", body)
			}
		})
	}
}

// TestScreenShellAuditShowsTheMethodTheMedianAndTheAttribution is the screen's
// own guard: the number is on screen with the count it covers, the range, the
// start that did not finish, the command and the bound it was run under, and the
// function zprof put first -- so the number can be reproduced and the culprit can
// be acted on.
func TestScreenShellAuditShowsTheMethodTheMedianAndTheAttribution(t *testing.T) {
	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = shellAuditTestState()
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	// The prose wraps to the width it is drawn at, so the assertions read it with
	// its whitespace collapsed: a phrase a line break split is one phrase again,
	// which is what a reader sees.
	flat := strings.Join(strings.Fields(view), " ")
	for _, want := range []string{
		"zsh -i -c exit",                                  // the exact command
		"bounded by 10 s",                                 // the timeout every start ran under
		"The measurement is 5 such starts",                // how many starts the method uses
		"Median of 4 of 5 runs: 924 ms",                   // the median, with the count it covers
		"Range: 872 ms to 977 ms",                         // the spread
		"1 of 5 runs did not finish: 1 timed out at 10 s", // the timeout, as its own result
		"Slowest function: compdump (725.25 ms self, 1 call(s))",
		"zsh/zprof profiled one further start",        // how the attribution was taken
		"in zprof's own order",                        // what the order means
		"_omz_source",                                 // the list, not only its head
		"1 more were profiled and are not named here", // the declared cut
		"The starts, in the order they ran: 977 ms, 915 ms, 872 ms, 932 ms",
		"Nothing here is changed", // the read-only promise
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the shell startup screen does not show %q:\n%s", want, view)
		}
	}
	// The sixth function zprof reported is past the named cut, and the screen says
	// so rather than appearing to have listed the whole table.
	if strings.Contains(flat, "compaudit") {
		t.Errorf("the screen named a function past the declared cut:\n%s", view)
	}
}

// TestScreenShellAuditSaysWhyNoFunctionIsNamed pins the honest half: when zprof
// cannot blame anyone the screen says so, names no function at all, and keeps the
// total -- which is the only number it can stand behind.
func TestScreenShellAuditSaysWhyNoFunctionIsNamed(t *testing.T) {
	st := shellAuditTestState()
	st.Shell = "bash"
	st.Command = "bash -i -c exit"
	st.Attribution = nil
	st.AttributionReason = "zprof is zsh's own module, and bash has no equivalent built in, so only the total is measurable here."

	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = st
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "No function is named") {
		t.Errorf("the screen does not say that no function is named:\n%s", view)
	}
	if !strings.Contains(view, "Median of 4 of 5 runs: 924 ms") {
		t.Errorf("the total was dropped along with the attribution:\n%s", view)
	}
	if !strings.Contains(strings.Join(m.shellAuditDescription(), " "), st.AttributionReason) {
		t.Errorf("the reason is not on the screen:\n%s", strings.Join(m.shellAuditDescription(), " "))
	}
	for _, invented := range []string{"compinit", "compdump", "compdef", "compaudit"} {
		if strings.Contains(view, invented) {
			t.Errorf("the screen names %q, which nothing measured:\n%s", invented, view)
		}
	}
}

// TestShellAuditScreenMeasuresOnceAndSaysSo pins the wiring and the re-entrancy
// rule: the row returns the command, the screen says a measurement is in flight
// while it runs, a second press starts no second measurement, the answer replaces
// the state whole, and esc goes back to the section.
func TestShellAuditScreenMeasuresOnceAndSaysSo(t *testing.T) {
	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = shellAuditTestState()
	m.Cursor = 0 // the measure row is the first row.
	if rows := m.GetCurrentOptions(); rows[0] != utilitiesShellAuditRow {
		t.Fatalf("the first row is %q, want the measure row %q", rows[0], utilitiesShellAuditRow)
	}

	next, cmd := m.handleShellAuditKeys("enter")
	m = next.(Model)
	if cmd == nil {
		t.Fatal("the measure row returned no command")
	}
	if !m.ShellAudit.Measuring {
		t.Error("the screen does not report a measurement in flight")
	}
	if notice := m.shellAuditNotice(); !strings.Contains(notice, "Measuring:") {
		t.Errorf("the footer does not say what is running: %q", notice)
	}

	next, second := m.handleShellAuditKeys("enter")
	m = next.(Model)
	if second != nil {
		t.Error("a second press while a measurement was running started a second one")
	}

	// The answer replaces the state whole, exactly as a real measurement would.
	done := shellAuditTestState()
	updated, _ := m.Update(shellAuditMeasuredMsg{state: done})
	m = updated.(Model)
	if !m.ShellAudit.Measured || m.ShellAudit.Measuring {
		t.Errorf("after the answer: measured=%v measuring=%v, want true and false", m.ShellAudit.Measured, m.ShellAudit.Measuring)
	}

	next, _ = m.handleShellAuditKeys("esc")
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Errorf("esc left the screen on %v, want ScreenUtilities", m.Screen)
	}
}

// TestUtilitiesSectionOpensTheShellAuditWithoutMeasuring pins the other end of
// the wiring: opening the screen from the section starts nothing, because
// starting the user's shell five times is a choice made on that screen.
func TestUtilitiesSectionOpensTheShellAuditWithoutMeasuring(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities
	m.ShellAudit = shellAuditTestState()
	m.Cursor = 0
	for i, row := range m.GetCurrentOptions() {
		if row == utilitiesShellAuditRow {
			m.Cursor = i
		}
	}

	next, cmd := m.handleUtilitiesKeys("enter")
	m = next.(Model)
	if m.Screen != ScreenShellAudit {
		t.Fatalf("selecting the row landed on %v, want ScreenShellAudit", m.Screen)
	}
	if cmd != nil {
		t.Error("opening the screen started a measurement")
	}
}

// ---------------------------------------------------------------------------
// Bringing the applied theme forward in each tool
// ---------------------------------------------------------------------------
//
// The user's report is the whole reason for this guard: the switch wrote the
// files and said "applied", but Herdr, tmux, Neovim and the rest went on
// showing the old theme because nothing told them the file had changed. The
// screen after a switch must name every tool it painted, what the installer did
// for it, and what only the user can do. PATH is emptied so the reload step runs
// no command here: the guard measures the list, never a tmux server the runner
// happens to have.
func TestApplyingAThemeListsWhatWasReloadedAndWhatIsLeft(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")
	home := t.TempDir()
	t.Setenv("HOME", home)

	// One tool that reloads itself (Alacritty), one only the user can reach
	// (Neovim), and one the installer can reach when a server is running (tmux).
	installThemeFiles(t, home, "alacritty", "nvim", "tmux")

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	m.DotfilesRepoDir = repoRoot(t)

	// No tmux or bat on PATH: the reload step must not run a command here.
	t.Setenv("PATH", t.TempDir())

	msg := m.applyDotfilesThemeCmd(def)()
	next, _ := m.Update(msg)
	m = next.(Model)

	if !m.ThemeRefreshReview || !m.ThemeRefreshDone {
		t.Fatalf("after a switch the picker is in review=%v done=%v, want the per-tool reload list",
			m.ThemeRefreshReview, m.ThemeRefreshDone)
	}
	report := strings.Join(m.themePickerDescription(), "\n")
	for _, want := range []string{"Alacritty", "Neovim", "tmux"} {
		if !strings.Contains(report, want) {
			t.Errorf("the reload list does not name %s:\n%s", want, report)
		}
	}
	// The manual steps name where to run them, not just "reload".
	if !strings.Contains(report, ":colorscheme") {
		t.Errorf("the reload list does not say what Neovim needs (:colorscheme):\n%s", report)
	}
	if !strings.Contains(report, "source-file") {
		t.Errorf("the reload list does not say what tmux needs (source-file):\n%s", report)
	}
}

// reloadStub is the pair of package variables the reload step reads, stubbed so
// a guard observes the exact commands and the environment without a tmux or bat
// on the runner. The originals are restored when the test ends.
type reloadCall struct {
	command string
	env     []string
}

func stubThemeReload(t *testing.T, exists func(string) bool, run func(string, []string) (string, error)) *[]reloadCall {
	t.Helper()
	originalRun, originalExists := themeReloadRun, themeReloadCommandExists
	t.Cleanup(func() {
		themeReloadRun, themeReloadCommandExists = originalRun, originalExists
	})
	calls := &[]reloadCall{}
	themeReloadCommandExists = exists
	themeReloadRun = func(command string, env []string) (string, error) {
		*calls = append(*calls, reloadCall{command: command, env: env})
		return run(command, env)
	}
	return calls
}

// reloadByTool indexes the reload list by tool so a guard can assert one line.
func reloadByTool(tools []themeReloadTool) map[string]themeReloadTool {
	byTool := map[string]themeReloadTool{}
	for _, tool := range tools {
		byTool[tool.Tool] = tool
	}
	return byTool
}

// TestReloadThemeToolsRunsOnlyTheSafeCommands is the teeth on the reload step.
// It observes the exact command strings and the environment, and it refuses the
// whole class of commands that can end a session: no kill, no signal, no
// shutdown, no restart. The safe reloads run; every other tool gets a line that
// names the exact action the user must take.
func TestReloadThemeToolsRunsOnlyTheSafeCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installThemeFiles(t, home, "alacritty", "kitty", "herdr", "nvim", "tmux")
	if err := os.MkdirAll(filepath.Join(home, ".config", "bat"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KITTY_LISTEN_ON", "unix:/tmp/kitty-reload-test")

	calls := stubThemeReload(t, func(cmd string) bool {
		switch cmd {
		case "tmux", "bat", "kitty":
			return true
		}
		return false
	}, func(command string, env []string) (string, error) {
		if command == "tmux list-sessions" {
			return "main: 1 windows", nil
		}
		return "", nil
	})

	tools := reloadThemeTools(home)

	joined := ""
	for _, c := range *calls {
		joined += c.command + "\n"
	}
	for _, want := range []string{"tmux source-file", "bat cache --build", "kitty @ load-config"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the reload step did not run %q; it ran:\n%s", want, joined)
		}
	}
	for _, dangerous := range []string{"kill", "pkill", "shutdown", "reboot", "poweroff", "restart", "reset", "sigusr", "sigterm", "sigkill", "wsl --"} {
		if strings.Contains(strings.ToLower(joined), dangerous) {
			t.Errorf("the reload step ran a command containing %q, which can end a session:\n%s", dangerous, joined)
		}
	}
	// bat reads the directory the new theme was written into, not the ambient one.
	foundBatEnv := false
	for _, c := range *calls {
		if c.command != "bat cache --build" {
			continue
		}
		for _, e := range c.env {
			if e == "BAT_CONFIG_DIR="+filepath.Join(home, ".config", "bat") {
				foundBatEnv = true
			}
		}
	}
	if !foundBatEnv {
		t.Errorf("bat's cache rebuild did not name BAT_CONFIG_DIR; calls: %+v", *calls)
	}

	byTool := reloadByTool(tools)
	if got := byTool["tmux"]; !got.Done || !strings.Contains(got.Note, "source-file") {
		t.Errorf("tmux with a running server = %+v, want done and source-file", got)
	}
	if got := byTool["Alacritty"]; !got.Done {
		t.Errorf("Alacritty = %+v, want done (it watches its file)", got)
	}
	if got := byTool["Neovim"]; got.Done || !strings.Contains(got.Note, ":colorscheme") {
		t.Errorf("Neovim = %+v, want not done and :colorscheme", got)
	}
	if got := byTool["Herdr"]; got.Done || !strings.Contains(got.Note, "Ctrl+b Shift+r") {
		t.Errorf("Herdr = %+v, want a manual reload that names its key", got)
	}
	if got := byTool["Kitty"]; !got.Done {
		t.Errorf("Kitty with a listening socket = %+v, want done", got)
	}
}

// TestReloadThemeToolsDoesNotStartATmuxServer pins the other half of the tmux
// rule: when no server is running, no command is sent, because `tmux
// source-file` would start one. The tool is named with the command the user can
// run instead.
func TestReloadThemeToolsDoesNotStartATmuxServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installThemeFiles(t, home, "tmux")

	calls := stubThemeReload(t, func(cmd string) bool { return cmd == "tmux" },
		func(command string, env []string) (string, error) {
			if command == "tmux list-sessions" {
				return "", os.ErrNotExist
			}
			return "", nil
		})

	tools := reloadThemeTools(home)
	for _, c := range *calls {
		if strings.Contains(c.command, "source-file") {
			t.Errorf("no server was running, yet the reload ran %q", c.command)
		}
	}
	got := reloadByTool(tools)["tmux"]
	if got.Done {
		t.Errorf("tmux with no server = %+v, want not done", got)
	}
	if !strings.Contains(got.Note, "source-file") {
		t.Errorf("the no-server line does not name the command to run: %+v", got)
	}
}

// TestThemeReloadRunnerTimesOut keeps the promise that a hung tmux or bat cannot
// hang the screen: the runner kills a command that outlives themeReloadTimeout
// and returns an error instead of waiting forever.
func TestThemeReloadRunnerTimesOut(t *testing.T) {
	start := time.Now()
	if _, err := themeReloadRun("sleep 30", nil); err == nil {
		t.Error("a command that outlived the timeout returned no error")
	}
	if elapsed := time.Since(start); elapsed > themeReloadTimeout+3*time.Second {
		t.Errorf("the runner waited %v, want the %v timeout to bound it", elapsed, themeReloadTimeout)
	}
}
