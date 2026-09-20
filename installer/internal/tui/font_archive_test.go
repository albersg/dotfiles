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
	stubLogEnv       = "FONT_STEP_STUB_LOG"
	stubCurlExitEnv  = "FONT_STEP_STUB_CURL_EXIT"
	stubUnzipExitEnv = "FONT_STEP_STUB_UNZIP_EXIT"
)

// stubCurlScript records the arguments the step built and creates the output
// file, which is what the real curl does before a transfer can still fail.
var stubCurlScript = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ -n "$out" ]; then printf 'partial-archive' > "$out"; fi
exit "$%s"
`, stubLogEnv, stubCurlExitEnv)

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
printf 'font-bytes' > "$dir/IosevkaTermNerdFont-Regular.ttf"
`, stubLogEnv, stubUnzipExitEnv, stubUnzipExitEnv)

var stubFcCacheScript = fmt.Sprintf(`#!/bin/sh
echo "fc-cache $*" >> "$%s"
`, stubLogEnv)

// fontStepStubs is the environment one font-step test runs in: an empty $HOME,
// the stub commands, and the log they append to.
type fontStepStubs struct {
	fontDir string
	log     string
}

// fontStubOptions says how the stubs behave: a zero exit code is a command that
// worked, which is how the failure paths are reached without touching the
// network or the filesystem.
type fontStubOptions struct {
	curlExit  int
	unzipExit int
}

// stubFontCommands installs the three stubs and prepares an empty $HOME.
func stubFontCommands(t *testing.T, opts fontStubOptions) *fontStepStubs {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	logPath := filepath.Join(t.TempDir(), "font-step-commands.log")
	t.Setenv(stubLogEnv, logPath)
	t.Setenv(stubCurlExitEnv, fmt.Sprintf("%d", opts.curlExit))
	t.Setenv(stubUnzipExitEnv, fmt.Sprintf("%d", opts.unzipExit))

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "curl", stubCurlScript)
	writeStubCommand(t, binDir, "unzip", stubUnzipScript)
	writeStubCommand(t, binDir, "fc-cache", stubFcCacheScript)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &fontStepStubs{
		fontDir: filepath.Join(home, ".local", "share", "fonts"),
		log:     logPath,
	}
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

func (s *fontStepStubs) commands(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(s.log)
	if err != nil {
		t.Fatalf("the step ran no command at all: %v", err)
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
	if filepath.Base(archive) != "IosevkaTerm.zip" {
		t.Errorf("the step downloaded %q, which is not the Nerd Fonts archive", archive)
	}
	stubs.assertNoArchiveLeft(t, archive)

	assertFontDirHolds(t, stubs.fontDir, []string{"IosevkaTermNerdFont-Regular.ttf"})

	commands := stubs.commands(t)
	var runners []string
	for _, line := range commands {
		runners = append(runners, strings.Fields(line)[0])
	}
	if got := strings.Join(runners, ","); got != "curl,unzip,fc-cache" {
		t.Errorf("the step ran %q, want the real download, extraction and cache update", got)
	}
	if joined := strings.Join(commands, "\n"); !strings.Contains(joined, "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.3.0/IosevkaTerm.zip") {
		t.Errorf("the step no longer downloads the pinned Nerd Fonts release: %q", joined)
	}

	if log := readLog(); !strings.Contains(log, "✓ Font installed") {
		t.Errorf("a successful install no longer reports success: %q", log)
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
// what it wrote itself. A user's own IosevkaTerm.zip is not the step's to delete
// or overwrite, and an unrelated file must survive untouched.
func TestStepInstallFontLinuxLeavesFilesItDidNotWriteAlone(t *testing.T) {
	stubs := stubFontCommands(t, fontStubOptions{})
	if err := os.MkdirAll(stubs.fontDir, 0o755); err != nil {
		t.Fatal(err)
	}

	userFiles := map[string]string{
		filepath.Join(stubs.fontDir, "IosevkaTerm.zip"): "the user's own archive\n",
		filepath.Join(stubs.fontDir, "MyFont.ttf"):      "the user's own font\n",
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
		"IosevkaTerm.zip",
		"MyFont.ttf",
		"IosevkaTermNerdFont-Regular.ttf",
	})
}
