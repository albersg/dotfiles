package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gaiFactoryFile is a factory-shaped gai.conf: every line is a comment or blank,
// the way the distribution ships it. It is not a copy of the real file; it has
// the shape the four cases care about, so a case that touches a factory line
// fails on the shape rather than on the distribution's wording.
const gaiFactoryFile = `# /etc/gai.conf -- the GNU C Library address-selection policy.
#
# The default precedence table is built into glibc. The lines below are the
# RFC 3484 defaults this file documents; they are commented out so the built-in
# default applies.
#
#precedence ::1/128 50
#precedence ::/0 40
#precedence 2002::/16 30
#precedence ::/96 20
#precedence ::ffff:0:0/96 100
`

// gaiOwnerFile is the owner's file as S2 quotes it: the RFC 3484 defaults and the
// deciding line, all uncommented, with no dotfiles marker.
const gaiOwnerFile = "precedence ::1/128 50\n" +
	"precedence ::/0 40\n" +
	"precedence 2002::/16 30\n" +
	"precedence ::/96 20\n" +
	"precedence ::ffff:0:0/96 100\n"

// captureGAILogs captures what the step reports, and restores the sink when the
// test ends.
func captureGAILogs(t *testing.T) *[]string {
	t.Helper()
	lines := &[]string{}
	previous := gaiLog
	gaiLog = func(_ string, line string) { *lines = append(*lines, line) }
	t.Cleanup(func() { gaiLog = previous })
	return lines
}

// stubGAIDpkg replaces the dpkg boundary, so a test can exercise the report
// without spawning dpkg or depending on the machine's packages.
func stubGAIDpkg(t *testing.T, available bool, owner string, err error) {
	t.Helper()
	previousAvailable, previousOwner := gaiDpkgAvailable, gaiDpkgOwner
	gaiDpkgAvailable = func() bool { return available }
	gaiDpkgOwner = func(string) (string, error) { return owner, err }
	t.Cleanup(func() {
		gaiDpkgAvailable, gaiDpkgOwner = previousAvailable, previousOwner
	})
}

// gaiFixture writes content to a temporary file and points the step at it, so
// every test runs without root and without touching the real /etc.
func gaiFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gai.conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envGAIConfPath, path)
	return path
}

func readGAIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}

// TestGAIStepIsScheduledOnLinuxAndWSLOnly pins S4 and S5 at the planner: the step
// is planned on Linux and WSL, immediately before deps, and nowhere else.
func TestGAIStepIsScheduledOnLinuxAndWSLOnly(t *testing.T) {
	if _, ok := stepExecutors[gaiStepID]; !ok {
		t.Fatalf("the executor table has no entry for %q", gaiStepID)
	}

	for _, tc := range []struct {
		name string
		opts planOptions
		want bool
	}{
		{"linux", planOptions{OS: "linux"}, true},
		{"wsl", planOptions{OS: "linux", IsWSL: true}, true},
		{"wsl detected without a linux choice", planOptions{OS: "mac", IsWSL: true}, true},
		{"mac", planOptions{OS: "mac"}, false},
		{"termux choice", planOptions{OS: "termux"}, false},
		{"detected termux", planOptions{OS: "linux", DetectedTermux: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := planFor(tc.opts)
			_, got := planStepByID(steps, gaiStepID)
			if got != tc.want {
				t.Errorf("gai planned = %v, want %v (plan %s)", got, tc.want, planIDList(steps))
			}
		})
	}

	steps := planFor(planOptions{OS: "linux"})
	for i, step := range steps {
		if step.ID != gaiStepID {
			continue
		}
		if i+1 >= len(steps) || steps[i+1].ID != "deps" {
			t.Errorf("gai is not immediately before deps: %s", planIDList(steps))
		}
	}
}

// TestGAIStepAppendsBlockToFactoryFile is case 4 (S6): a factory file gains the
// marked block at the end, and no other byte moves.
func TestGAIStepAppendsBlockToFactoryFile(t *testing.T) {
	captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)
	path := gaiFixture(t, gaiFactoryFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed on a factory file: %v", err)
	}

	got := readGAIFile(t, path)
	want := gaiFactoryFile + string(gaiBlock())
	if string(got) != want {
		t.Errorf("the appended file is not the factory file plus the block:\n got:\n%s\nwant:\n%s", got, want)
	}
	if n := strings.Count(string(got), gaiMarkerStart); n != 1 {
		t.Errorf("the file holds %d start markers, want 1", n)
	}
	for i, line := range strings.Split(strings.TrimSuffix(gaiFactoryFile, "\n"), "\n") {
		if !strings.Contains(string(got), line) {
			t.Errorf("factory line %d (%q) is missing from the result", i+1, line)
		}
	}
}

// TestGAIStepReplacesBlockInPlaceAndIsIdempotent is case 1 (S8): a file whose
// marker pair is already present has the block replaced in place, and a second
// run leaves the whole file byte for byte identical.
func TestGAIStepReplacesBlockInPlaceAndIsIdempotent(t *testing.T) {
	captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	stale := "# factory header\n" + gaiMarkerStart + "\n" +
		"# an older dotfiles block\nprecedence ::ffff:0:0/96 10\n" + gaiMarkerEnd + "\n# factory footer\n"
	path := gaiFixture(t, stale)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed: %v", err)
	}
	first := readGAIFile(t, path)
	want := "# factory header\n" + string(gaiBlock()) + "# factory footer\n"
	if string(first) != want {
		t.Fatalf("the block was not replaced in place:\n got:\n%s\nwant:\n%s", first, want)
	}
	if n := strings.Count(string(first), gaiMarkerStart); n != 1 {
		t.Errorf("the file holds %d start markers after the replacement, want 1", n)
	}

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the second gai run failed: %v", err)
	}
	second := readGAIFile(t, path)
	if !bytes.Equal(first, second) {
		t.Errorf("a second run changed the file:\n first:\n%s\nsecond:\n%s", first, second)
	}
}

// TestGAIStepLeavesAFileThatAlreadyPrefersIPv4 is case 2 (S7): an uncommented
// deciding line with no marker means the file already prefers IPv4, so nothing is
// written and the step says so.
func TestGAIStepLeavesAFileThatAlreadyPrefersIPv4(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)
	path := gaiFixture(t, gaiOwnerFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed on the owner's file: %v", err)
	}

	got := readGAIFile(t, path)
	if string(got) != gaiOwnerFile {
		t.Errorf("the owner's file changed:\n got:\n%s\nwant:\n%s", got, gaiOwnerFile)
	}
	if joined := strings.Join(*logs, "\n"); !strings.Contains(joined, "already prefers IPv4") {
		t.Errorf("the step did not say the file already prefers IPv4; logs: %v", *logs)
	}
}

// TestGAIStepLeavesAForeignPrecedence is case 3 (S7): an uncommented precedence
// the owner set is never overridden, the step says what it found and that the
// choice is the owner's, and it does not fail.
func TestGAIStepLeavesAForeignPrecedence(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	foreign := "# hand-written by the owner\nprecedence ::ffff:0:0/96 50\n"
	path := gaiFixture(t, foreign)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a foreign precedence must not fail the step: %v", err)
	}

	got := readGAIFile(t, path)
	if string(got) != foreign {
		t.Errorf("the foreign precedence was rewritten:\n got:\n%s\nwant:\n%s", got, foreign)
	}
	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "precedence ::ffff:0:0/96 50") {
		t.Errorf("the step did not say what it found; logs: %v", *logs)
	}
	if !strings.Contains(joined, "owner") {
		t.Errorf("the step did not say the choice is the owner's; logs: %v", *logs)
	}
	if strings.Contains(joined, "already prefers IPv4") {
		t.Errorf("the step read a foreign precedence as IPv4-first; logs: %v", *logs)
	}
}

// TestGAIStepNamesTheOwningPackageAndTheUpgradeRisk is S3's reporting half: the
// step says which package owns the file and what a future upgrade may do to it.
func TestGAIStepNamesTheOwningPackageAndTheUpgradeRisk(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, true, "libc-bin: /etc/gai.conf\n", nil)
	gaiFixture(t, gaiFactoryFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed: %v", err)
	}

	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "belongs to libc-bin") {
		t.Errorf("the step did not name the owning package; logs: %v", *logs)
	}
	if !strings.Contains(joined, "conffile") {
		t.Errorf("the step did not say how an upgrade treats the file; logs: %v", *logs)
	}
	if !strings.Contains(joined, "unattended upgrade") {
		t.Errorf("the step did not say what an unanswered question does to an unattended upgrade; logs: %v", *logs)
	}
}

// TestGAIStepSaysWhenDpkgIsMissing is S3's other branch: no dpkg, so the step
// says so instead of guessing, and does not fail.
func TestGAIStepSaysWhenDpkgIsMissing(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)
	gaiFixture(t, gaiFactoryFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a missing dpkg must not fail the step: %v", err)
	}
	if joined := strings.Join(*logs, "\n"); !strings.Contains(joined, "dpkg is not available") {
		t.Errorf("the step did not say dpkg is missing; logs: %v", *logs)
	}
}

// TestGAIStepSurvivesAFailedDpkgLookup is S3's failure branch: a dpkg that
// answers with an error is reported and the run goes on.
func TestGAIStepSurvivesAFailedDpkgLookup(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, true, "", errors.New("dpkg: no path found matching pattern"))
	gaiFixture(t, gaiFactoryFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a failed dpkg lookup must not fail the step: %v", err)
	}
	if joined := strings.Join(*logs, "\n"); !strings.Contains(joined, "did not answer") {
		t.Errorf("the step did not report the failed lookup; logs: %v", *logs)
	}
}

// TestGAIPackageNameReadsDpkgOutput holds the parser to the shape dpkg prints, so
// the report names the package rather than the whole line.
func TestGAIPackageNameReadsDpkgOutput(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"libc-bin: /etc/gai.conf", "libc-bin"},
		{"libc-bin: /etc/gai.conf\nlibc6: /etc/gai.conf", "libc-bin"},
		{"  libc-bin  :  /etc/gai.conf  ", "libc-bin"},
		{"", ""},
	} {
		if got := gaiPackageName(tc.in); got != tc.want {
			t.Errorf("gaiPackageName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestGAIStepUsesTheSharedWriter pins S11: the step writes through
// applyArtifactContent, whose backup is what a user restores from. The writer's
// own guard covers the backup-and-verify behaviour; this checks that the step
// took that path by the backup file it leaves behind.
func TestGAIStepUsesTheSharedWriter(t *testing.T) {
	captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)
	path := gaiFixture(t, gaiFactoryFile)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed: %v", err)
	}

	backups, err := filepath.Glob(path + ".bak-dotfiles-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Errorf("the writer left %d backups, want 1: %v", len(backups), backups)
	}
	if len(backups) == 1 {
		if got := readGAIFile(t, backups[0]); string(got) != gaiFactoryFile {
			t.Errorf("the backup does not hold the file the step found:\n got:\n%s\nwant:\n%s", got, gaiFactoryFile)
		}
	}
}
