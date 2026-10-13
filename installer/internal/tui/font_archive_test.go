package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The Linux font step shells out to curl, unzip and fc-cache, and the defect
// covered here is about a file on disk rather than about an argument string, so
// these tests put stub binaries for those three commands first on PATH instead
// of mocking the step away. The step runs its real command sequence -- real
// shell, real argument paths, real file creation and real removal -- while the
// network stays untouched. Everything the assertions look at is a real file.
const (
	stubLogEnv          = "FONT_STEP_STUB_LOG"
	stubCurlExitEnv     = "FONT_STEP_STUB_CURL_EXIT"
	stubCurlEmptyEnv    = "FONT_STEP_STUB_CURL_EMPTY"
	stubUnzipExitEnv    = "FONT_STEP_STUB_UNZIP_EXIT"
	stubUnzipFontEnv    = "FONT_STEP_STUB_UNZIP_FONT"
	stubFcListModeEnv   = "FONT_STEP_STUB_FC_LIST_MODE"
	stubFcListFamilyEnv = "FONT_STEP_STUB_FC_LIST_FAMILY"
	stubFcListExitEnv   = "FONT_STEP_STUB_FC_LIST_EXIT"
)

// stubFontFaceFile is the face the archive provides and the extraction stub
// writes. It comes from the same constant the Windows copy reads, so the stub
// cannot drift from the archive.
var stubFontFaceFile = windowsFontFamilyFilePrefix + "Regular.ttf"

// stubCurlScript records the arguments the step built and creates the output
// file, which is what the real curl does before a transfer can still fail. A
// zero-length body is reachable through the stub so the verification path can be
// exercised: curl really does exit 0 for an empty body.
var stubCurlScript = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ -n "$out" ]; then
  if [ "$%s" = "1" ]; then : > "$out"; else printf 'partial-archive' > "$out"; fi
fi
exit "$%s"
`, stubLogEnv, stubCurlEmptyEnv, stubCurlExitEnv)

// stubUnzipScript extracts one face, named by en environment variable so a test
// can model an extraction that succeeded while producing no face fontconfig
// can resolve.
var stubUnzipScript = fmt.Sprintf(`#!/bin/sh
echo "unzip $*" >> "$%s"
archive=""
dir=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then archive="$arg"; fi
  if [ "$prev" = "-d" ]; then dir="$arg"; fi
  prev="$arg"
done
if [ "$%s" != "0" ]; then
  echo "unzip: cannot find or open $archive" >&2
  exit "$%s"
fi
mkdir -p "$dir"
printf 'font-bytes' > "$dir/${%s:-%s}"
`, stubLogEnv, stubUnzipExitEnv, stubUnzipExitEnv, stubUnzipFontEnv, stubFontFaceFile)

var stubFcCacheScript = fmt.Sprintf(`#!/bin/sh
echo "fc-cache $*" >> "$%s"
`, stubLogEnv)

// stubFcListScript models the fontconfig registry the step asks. By default it
// reports the family of the face the extraction wrote -- fontconfig resolves a
// family once a face for it is on disk -- so the presence check before the
// download and the verification after extraction both run against real files in
// the temporary $HOME. A test can force an answer with the mode variable, and can
// rename the family to prove the step compares the family it names rather than
// merely accepting that some font is installed. The family is built from
// mesloArchiveFamily, the one family the pinned archive declares and the shipped
// configurations are checked against, so the response cannot drift from what the
// step asks for.
var stubFcListScript = fmt.Sprintf(`#!/bin/sh
echo "fc-list $*" >> "$%s"
if [ "$%s" != "0" ]; then exit "$%s"; fi
family="${%s:-%s}"
case "${%s:-auto}" in
  present) printf '%%s\n' "$family" ;;
  absent) ;;
  *) if [ -f "$HOME/.local/share/fonts/%s" ]; then printf '%%s\n' "$family"; fi ;;
esac
exit 0
`, stubLogEnv, stubFcListExitEnv, stubFcListExitEnv, stubFcListFamilyEnv, mesloArchiveFamily, stubFcListModeEnv, stubFontFaceFile)

// stubTermuxReloadScript records the Termux settings reload, so a test can assert
// the already-installed path has no side effect rather than only that no download
// ran.
var stubTermuxReloadScript = fmt.Sprintf(`#!/bin/sh
echo "termux-reload-settings $*" >> "$%s"
`, stubLogEnv)

// fontStepStubs is the environment one font-step test runs in: an empty $HOME,
// the stub commands, and the log they append to.
type fontStepStubs struct {
	home    string
	fontDir string
	log     string
}

// fontStubOptions says how the stubs behave: a zero exit code is a command that
// worked, which is how the failure paths are reached without touching the
// network or the filesystem. unzipFontFile, fcListMode and fcListFamily default
// to the extraction writing the archive's face and fontconfig reporting it.
type fontStubOptions struct {
	curlExit      int
	curlEmpty     bool
	unzipExit     int
	unzipFontFile string
	fcListMode    string
	fcListFamily  string
	fcListExit    int
}

// stubFontCommands installs the stubs and prepares an empty $HOME. The step runs
// its real command sequence against these -- real shell, real argument paths,
// real file creation and real removal -- while the network, the real $HOME and
// the real fontconfig all stay untouched.
func stubFontCommands(t *testing.T, opts fontStubOptions) *fontStepStubs {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	logPath := filepath.Join(t.TempDir(), "font-step-commands.log")
	t.Setenv(stubLogEnv, logPath)
	t.Setenv(stubCurlExitEnv, fmt.Sprintf("%d", opts.curlExit))
	t.Setenv(stubCurlEmptyEnv, boolEnv(opts.curlEmpty))
	t.Setenv(stubUnzipExitEnv, fmt.Sprintf("%d", opts.unzipExit))
	t.Setenv(stubUnzipFontEnv, opts.unzipFontFile)
	t.Setenv(stubFcListModeEnv, opts.fcListMode)
	t.Setenv(stubFcListFamilyEnv, opts.fcListFamily)
	t.Setenv(stubFcListExitEnv, fmt.Sprintf("%d", opts.fcListExit))

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "curl", stubCurlScript)
	writeStubCommand(t, binDir, "unzip", stubUnzipScript)
	writeStubCommand(t, binDir, "fc-cache", stubFcCacheScript)
	writeStubCommand(t, binDir, "fc-list", stubFcListScript)
	writeStubCommand(t, binDir, "termux-reload-settings", stubTermuxReloadScript)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &fontStepStubs{
		home:    home,
		fontDir: filepath.Join(home, ".local", "share", "fonts"),
		log:     logPath,
	}
}

func boolEnv(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

func writeStubCommand(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatalf("could not write the %s stub: %v", name, err)
	}
}

// linuxFontModel is a plain Linux selection, which is the only platform the
// reported defect covers.
func linuxFontModel() Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
	m.Choices = UserChoices{OS: "linux", InstallFont: true}
	return m
}

// termuxFontModel is a Termux selection, whose font step reads one file from
// Termux's own slot rather than fontconfig.
func termuxFontModel() Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, IsTermux: true}
	m.Choices = UserChoices{OS: "termux", InstallFont: true}
	return m
}

func (s *fontStepStubs) commands(t *testing.T) []string {
	t.Helper()

	lines := s.optionalCommands()
	if len(lines) == 0 {
		t.Fatal("the step ran no command at all")
	}
	return lines
}

// optionalCommands reads the command log when there is one. A path that stops
// before running anything -- Termux's already-installed branch -- leaves no log
// at all, and a guard has to be able to assert that absence rather than fail on
// it.
func (s *fontStepStubs) optionalCommands() []string {
	data, err := os.ReadFile(s.log)
	if err != nil {
		return nil
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// downloadedArchive is the path the step handed to `curl -o`. Reading it back
// from the command the step really ran keeps the assertions honest: they check
// the file this step downloaded, wherever the step decided to put it.
func (s *fontStepStubs) downloadedArchive(t *testing.T) string {
	t.Helper()

	lines := s.commands(t)
	for _, line := range lines {
		if !strings.HasPrefix(line, "curl ") {
			continue
		}
		args := strings.Fields(line)
		for i, arg := range args {
			if arg == "-o" && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	t.Fatalf("the step downloaded no archive: %v", lines)
	return ""
}

// fetchCommands returns every download the step made. A guard that the machine
// already has the font asserts this is empty rather than asserting a flag: the
// defect was that a download happened, so the download is what the guard reads.
func (s *fontStepStubs) fetchCommands(t *testing.T) []string {
	t.Helper()

	var fetches []string
	for _, line := range s.optionalCommands() {
		if strings.HasPrefix(line, "curl ") {
			fetches = append(fetches, line)
		}
	}
	return fetches
}

// installFontFile puts a face in the font directory before the step runs, which
// is how a machine that already has the family is modelled.
func (s *fontStepStubs) installFontFile(t *testing.T, name, content string) string {
	t.Helper()

	if err := os.MkdirAll(s.fontDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.fontDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// termuxFontPath is the one slot Termux's terminal reads.
func (s *fontStepStubs) termuxFontPath() string {
	return filepath.Join(s.home, termuxFontDirName, termuxFontFileName)
}

// assertNoArchiveLeft covers the reported symptom: no archive file may remain,
// neither the one the step downloaded nor any other .zip inside the directory
// fontconfig scans.
func (s *fontStepStubs) assertNoArchiveLeft(t *testing.T, archive string) {
	t.Helper()

	if _, err := os.Stat(archive); err == nil {
		t.Errorf("the font archive %s survived the step", archive)
	}
	for _, path := range filesIn(t, s.fontDir) {
		if strings.HasSuffix(path, ".zip") {
			t.Errorf("an archive was left in the font directory fontconfig scans: %s", path)
		}
	}
}

// assertFontDirHolds pins what is left in the font directory once the step is
// done. A directory the step created for the download would be just as wrong as
// the archive itself, because fontconfig scans this path.
func assertFontDirHolds(t *testing.T, dir string, want []string) {
	t.Helper()

	var got []string
	for _, path := range filesIn(t, dir) {
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, relative)
	}
	slices.Sort(got)

	want = slices.Clone(want)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("the font directory holds %v, want %v", got, want)
	}
}

func filesIn(t *testing.T, dir string) []string {
	t.Helper()

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("could not inspect %s: %v", dir, err)
	}
	return files
}

// captureStepStdout captures the install log, which only reaches stdout when the
// installer runs non-interactively with DOTFILES_VERBOSE set.
func captureStepStdout(t *testing.T) func() string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = original
		_ = writer.Close()
	})

	return func() string {
		_ = writer.Close()
		os.Stdout = original
		out, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("could not read the install log: %v", err)
		}
		return string(out)
	}
}

// TestStepInstallFontLinuxLeavesNoArchiveBehind is the reported defect: on Linux
// the step downloaded the 347 MB archive into ~/.local/share/fonts, extracted it
// there and never removed it, so it stayed in a directory fontconfig scans.
func TestStepInstallFontLinuxLeavesNoArchiveBehind(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })
	readLog := captureStepStdout(t)

	stubs := stubFontCommands(t, fontStubOptions{})
	m := linuxFontModel()

	if err := stepInstallFont(&m); err != nil {
		t.Fatalf("a successful font install must still report success: %v", err)
	}

	archive := stubs.downloadedArchive(t)
	if filepath.Base(archive) != "Meslo.zip" {
		t.Errorf("the step downloaded %q, which is not the Nerd Fonts archive", archive)
	}
	stubs.assertNoArchiveLeft(t, archive)

	assertFontDirHolds(t, stubs.fontDir, []string{"MesloLGSNerdFont-Regular.ttf"})

	commands := stubs.commands(t)
	var runners []string
	for _, line := range commands {
		runners = append(runners, strings.Fields(line)[0])
	}
	// The family is asked before the download and again after fc-cache: the first
	// check is what skips the download on a machine that already has the font,
	// and the second is what makes the success line a verification rather than an
	// assumption.
	if got := strings.Join(runners, ","); got != "fc-list,curl,unzip,fc-cache,fc-list" {
		t.Errorf("the step ran %q, want a presence check, the download, the extraction, the cache update and a verification", got)
	}
	if joined := strings.Join(commands, "\n"); !strings.Contains(joined, "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.3.0/Meslo.zip") {
		t.Errorf("the step no longer downloads the pinned Nerd Fonts release: %q", joined)
	}

	log := readLog()
	if !strings.Contains(log, "✓ Font installed") {
		t.Errorf("a successful install no longer reports success: %q", log)
	}
	if !strings.Contains(log, "fc-list") || !strings.Contains(log, mesloArchiveFamily) {
		t.Errorf("the success line does not report what was verified and which family resolved: %q", log)
	}
}

// TestStepInstallFontLinuxDoesNotDownloadWhenTheFamilyIsAlreadyInstalled is the
// first half of issue #269's guard. The machine already has the family, so the
// ~108 MB archive must not be fetched. The guard reads the commands the step ran
// and asserts the download is absent, rather than reading a flag the step might
// set and then fetch anyway. Without the presence check the step runs curl here,
// so this test fails on the download itself.
func TestStepInstallFontLinuxDoesNotDownloadWhenTheFamilyIsAlreadyInstalled(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })
	readLog := captureStepStdout(t)

	stubs := stubFontCommands(t, fontStubOptions{})
	stubs.installFontFile(t, stubFontFaceFile, "the font that is already there")
	m := linuxFontModel()

	if err := stepInstallFont(&m); err != nil {
		t.Fatalf("an already-installed font must be a successful run: %v", err)
	}

	if fetches := stubs.fetchCommands(t); len(fetches) != 0 {
		t.Errorf("the step downloaded with the family already installed: %v", fetches)
	}
	joined := strings.Join(stubs.commands(t), "\n")
	if !strings.Contains(joined, "fc-list : family") {
		t.Errorf("the presence check did not ask fontconfig by name: %q", joined)
	}
	if strings.Contains(joined, "unzip") {
		t.Errorf("the step extracted an archive it had no reason to download: %q", joined)
	}
	if strings.Contains(joined, "fc-cache") {
		t.Errorf("the step rebuilt the cache for a directory it did not change: %q", joined)
	}

	log := readLog()
	if !strings.Contains(log, "fc-list") {
		t.Errorf("the step's report does not name what answered: %q", log)
	}
	if !strings.Contains(log, "already installed") {
		t.Errorf("an already-installed font was not reported as already installed: %q", log)
	}
	if strings.Contains(log, "Downloading") {
		t.Errorf("the step announced a download it never made: %q", log)
	}
}

// TestStepInstallFontLinuxFailsWhenTheFamilyDoesNotResolve is the third guard:
// the download and the extraction really happened, but the face fontconfig can
// resolve is not the family the configurations name. The step must fail and say
// what it asked and what was missing; without the verification it prints
// "✓ Font installed" and the user's terminal silently falls back. The extraction
// writes a differently named face, so the defect is a resolvable-family mismatch
// rather than a failed command.
func TestStepInstallFontLinuxFailsWhenTheFamilyDoesNotResolve(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{unzipFontFile: "SomeOtherFont-Regular.ttf"})
	m := linuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("an extraction that produced no resolvable family must not be reported as a successful install")
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "font" {
		t.Fatalf("the failure is not reported as a font step failure: %v", err)
	}
	if !strings.Contains(err.Error(), mesloArchiveFamily) || !strings.Contains(err.Error(), linuxFontFamilyQuery) {
		t.Errorf("the failure does not name the family and the check that missed it: %v", err)
	}

	joined := strings.Join(stubs.commands(t), "\n")
	if !strings.Contains(joined, "curl") || !strings.Contains(joined, "unzip") || !strings.Contains(joined, "fc-cache") {
		t.Errorf("the guard did not exercise the install path it claims to verify: %q", joined)
	}
}

// TestStepInstallFontLinuxDoesNotAcceptAnotherFamily proves the check compares
// the family the configurations name rather than merely checking that some font
// is installed: fontconfig reports a different family both before and after the
// download, so the step must still fail rather than report success.
func TestStepInstallFontLinuxDoesNotAcceptAnotherFamily(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{fcListMode: "present", fcListFamily: "SomeOtherNF"})
	m := linuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("a fontconfig answer for another family must not be reported as the configured family")
	}
	if !strings.Contains(err.Error(), mesloArchiveFamily) {
		t.Errorf("the failure does not name the family the check asked for: %v", err)
	}
	if len(stubs.fetchCommands(t)) == 0 {
		t.Error("the guard did not exercise the download path it claims to verify")
	}
}

// TestStepInstallFontLinuxLeavesNoArchiveWhenExtractionFails covers the failure
// path, which matters more than the success path: the cleanup must run there
// too, and it must not turn the failure into a success.
func TestStepInstallFontLinuxLeavesNoArchiveWhenExtractionFails(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{unzipExit: 1})
	m := linuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("a failed extraction must not be reported as a successful install")
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "font" {
		t.Fatalf("the failure is not reported as a font step failure: %v", err)
	}
	if !strings.Contains(err.Error(), "Failed to extract font archive") {
		t.Errorf("the failure does not name the extraction: %v", err)
	}

	stubs.assertNoArchiveLeft(t, stubs.downloadedArchive(t))
	assertFontDirHolds(t, stubs.fontDir, nil)

	if joined := strings.Join(stubs.commands(t), "\n"); strings.Contains(joined, "fc-cache") {
		t.Error("a failed extraction must not update the font cache as if fonts had been installed")
	}
}

// TestStepInstallFontLinuxReportsAFailedDownload pins how a download that never
// produced an archive is handled: the step fails, and the partial file curl
// leaves at its output path does not stay in the font directory either.
func TestStepInstallFontLinuxReportsAFailedDownload(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{curlExit: 22})
	m := linuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("a failed download must not be reported as a successful install")
	}
	if !strings.Contains(err.Error(), "Failed to download font") {
		t.Errorf("the failure does not name the download: %v", err)
	}

	stubs.assertNoArchiveLeft(t, stubs.downloadedArchive(t))
	assertFontDirHolds(t, stubs.fontDir, nil)

	if joined := strings.Join(stubs.commands(t), "\n"); strings.Contains(joined, "unzip") {
		t.Error("the step extracted an archive it never downloaded")
	}
}

// TestStepInstallFontLinuxLeavesFilesItDidNotWriteAlone pins the narrow scope of
// the cleanup: the font directory belongs to the user, so the step removes only
// what it wrote itself. A user's own Meslo.zip is not the step's to delete or
// overwrite, and an unrelated file must survive untouched.
func TestStepInstallFontLinuxLeavesFilesItDidNotWriteAlone(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{})
	if err := os.MkdirAll(stubs.fontDir, 0o755); err != nil {
		t.Fatal(err)
	}

	userFiles := map[string]string{
		filepath.Join(stubs.fontDir, "Meslo.zip"):  "the user's own archive\n",
		filepath.Join(stubs.fontDir, "MyFont.ttf"): "the user's own font\n",
	}
	for path, content := range userFiles {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := linuxFontModel()
	if err := stepInstallFont(&m); err != nil {
		t.Fatalf("font step failed: %v", err)
	}

	for path, want := range userFiles {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("the step removed %s, which it did not write: %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("the step overwrote %s: got %q, want %q", path, got, want)
		}
	}

	assertFontDirHolds(t, stubs.fontDir, []string{
		"Meslo.zip",
		"MyFont.ttf",
		"MesloLGSNerdFont-Regular.ttf",
	})
}

// TestStepInstallFontLinuxFailsWhenFontconfigCannotBeAsked is the guard for the
// other half of "verify, don't assume": an fc-list that cannot be asked is not an
// answer that the family resolves. The download and extraction succeed, but the
// step must fail rather than print ✓, because a machine whose fontconfig cannot
// report the family is a machine where the configured font may not resolve.
func TestStepInstallFontLinuxFailsWhenFontconfigCannotBeAsked(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{fcListExit: 127})
	m := linuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("an unaskable fontconfig must not be reported as a successful install")
	}
	if !strings.Contains(err.Error(), "could not be asked") || !strings.Contains(err.Error(), linuxFontFamilyQuery) {
		t.Errorf("the failure does not say fontconfig could not be asked, naming the check: %v", err)
	}
	// The download still ran: an absent answer is not a reason to skip it, and
	// the failure is about verification rather than about the fetch.
	if len(stubs.fetchCommands(t)) == 0 {
		t.Error("the guard did not exercise the download path it claims to verify")
	}
}

// TestStepInstallFontTermuxDoesNotDownloadWhenTheSlotIsFilled is Termux's half of
// the presence guard. Termux has no fontconfig: its terminal reads one file, so a
// non-empty ~/.termux/font.ttf is the whole answer. The guard asserts no curl ran,
// and the reload stub proves the step really stopped instead of continuing to
// termux-reload-settings as if it had changed the slot.
func TestStepInstallFontTermuxDoesNotDownloadWhenTheSlotIsFilled(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })
	readLog := captureStepStdout(t)

	stubs := stubFontCommands(t, fontStubOptions{})
	fontPath := stubs.termuxFontPath()
	if err := os.MkdirAll(filepath.Dir(fontPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontPath, []byte("the font that is already there"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := termuxFontModel()

	if err := stepInstallFont(&m); err != nil {
		t.Fatalf("an already-installed Termux font must be a successful run: %v", err)
	}

	if fetches := stubs.fetchCommands(t); len(fetches) != 0 {
		t.Errorf("the step downloaded with the Termux font slot already filled: %v", fetches)
	}
	if joined := strings.Join(stubs.optionalCommands(), "\n"); strings.Contains(joined, "termux-reload-settings") {
		t.Errorf("the step reloaded Termux settings for a slot it did not change: %q", joined)
	}
	if log := readLog(); !strings.Contains(log, "already installed") || strings.Contains(log, "Downloading") {
		t.Errorf("the already-installed path did not report the truth: %q", log)
	}
}

// TestStepInstallFontTermuxDownloadsAndVerifiesTheSlot is Termux's half of the
// verification guard: the file must really be there and non-empty before the step
// reports success.
func TestStepInstallFontTermuxDownloadsAndVerifiesTheSlot(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{})
	m := termuxFontModel()

	if err := stepInstallFont(&m); err != nil {
		t.Fatalf("a Termux font install must succeed: %v", err)
	}

	joined := strings.Join(stubs.commands(t), "\n")
	if !strings.Contains(joined, "curl") || !strings.Contains(joined, termuxFontURL) {
		t.Errorf("the step did not download the Termux font: %q", joined)
	}
	if !strings.Contains(joined, "termux-reload-settings") {
		t.Errorf("the step did not reload Termux settings after installing the font: %q", joined)
	}

	info, err := os.Stat(stubs.termuxFontPath())
	if err != nil {
		t.Fatalf("the step reported success without a font in the slot: %v", err)
	}
	if info.Size() == 0 {
		t.Error("the step reported success with an empty font slot")
	}
}

// TestStepInstallFontTermuxFailsWhenTheSlotIsEmpty covers the verification's
// teeth: curl exits 0 for an empty body, so a step that trusted curl's exit code
// would report "✓ Font installed" over a zero-byte font.ttf that Termux cannot
// read.
func TestStepInstallFontTermuxFailsWhenTheSlotIsEmpty(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{curlEmpty: true})
	m := termuxFontModel()

	err := stepInstallFont(&m)
	if err == nil {
		t.Fatal("an empty Termux font slot must not be reported as a successful install")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("the failure does not name the empty slot: %v", err)
	}
	if strings.Contains(strings.Join(stubs.commands(t), "\n"), "termux-reload-settings") {
		t.Error("the step reloaded Termux settings for a font it never installed")
	}
}
