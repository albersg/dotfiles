package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// TestTheInteractiveRoutePreservesTheUsersOwnKeys pins the third route into the
// file: the interactive install copies the bytes renderedRepoWSLConfig returns
// straight over the destination from its shell script, so the merge has to
// happen before that copy. Without it the interactive route -- which is the route
// a TUI install actually takes, because /etc/wsl.conf needs sudo -- would
// overwrite keys the user set, while the step and the utilities section kept
// them. The guard reads the bytes the script is pointed at rather than running
// the shell, so it checks the content the same way the existing route guard does.
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
