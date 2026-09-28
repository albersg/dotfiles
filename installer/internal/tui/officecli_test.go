package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The OfficeCLI step shells out to curl, so these tests put a stub curl first on
// PATH and hand it a locally generated fixture. The step then runs its real
// download, checksum, chmod and rename sequence against real files while the
// network stays untouched.
const (
	officeCLICurlLogEnv     = "OFFICECLI_CURL_LOG"
	officeCLICurlFixtureEnv = "OFFICECLI_CURL_FIXTURE"
	officeCLICurlExitEnv    = "OFFICECLI_CURL_EXIT"
)

var stubOfficeCLICurl = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ -n "$out" ] && [ -n "$%s" ]; then cp "$%s" "$out"; fi
exit "${%s:-0}"
`, officeCLICurlLogEnv, officeCLICurlFixtureEnv, officeCLICurlFixtureEnv, officeCLICurlExitEnv)

type officeCLIStubs struct {
	home   string
	log    string
	binDir string
}

func stubOfficeCLICommands(t *testing.T, fixture string, exitCode int) *officeCLIStubs {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	logPath := filepath.Join(t.TempDir(), "officecli-curl.log")
	t.Setenv(officeCLICurlLogEnv, logPath)
	t.Setenv(officeCLICurlFixtureEnv, fixture)
	t.Setenv(officeCLICurlExitEnv, fmt.Sprintf("%d", exitCode))

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "curl", stubOfficeCLICurl)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &officeCLIStubs{
		home:   home,
		log:    logPath,
		binDir: filepath.Join(home, ".local", "bin"),
	}
}

// ranCommands returns the curl invocations the step actually made. An absent log
// means curl never ran.
func (s *officeCLIStubs) ranCommands(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(s.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("could not read the stub log: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// assertNoStagingLeft fails when the atomic-install staging directory survived
// the step, which would mean a mismatch or failure leaked state.
func (s *officeCLIStubs) assertNoStagingLeft(t *testing.T) {
	t.Helper()

	entries, err := os.ReadDir(s.binDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("could not read %s: %v", s.binDir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".officecli-staging-") {
			t.Errorf("the staging directory %s was left behind", entry.Name())
		}
	}
}

// withOfficeCLIPlatform overrides the detected platform so a test can exercise
// an unsupported host or a specific OS/architecture without cross-compiling.
func withOfficeCLIPlatform(t *testing.T, goos, goarch string, alpine bool) {
	t.Helper()
	original := officeCLITargetPlatform
	officeCLITargetPlatform = func() (string, string, bool) { return goos, goarch, alpine }
	t.Cleanup(func() { officeCLITargetPlatform = original })
}

// withOfficeCLIAsset swaps the asset resolver for a fixture whose SHA-256 the
// test controls, keeping the real download/verify/install sequence intact.
func withOfficeCLIAsset(t *testing.T, asset officeCLIAsset) {
	t.Helper()
	original := officeCLIAssetLookup
	officeCLIAssetLookup = func(_, _ string, _ bool) (officeCLIAsset, bool) { return asset, true }
	t.Cleanup(func() { officeCLIAssetLookup = original })
}

func officeCLIModel() *Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
	m.Choices = UserChoices{OS: "linux"}
	return &m
}

func writeOfficeCLIFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "officecli")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("could not write the fixture: %v", err)
	}
	return path
}

// TestOfficeCLIAssetsArePinned locks the immutable release and the six per-
// platform asset checksums. A changed upstream asset or a stale digest fails
// here before it can reach a download.
func TestOfficeCLIAssetsArePinned(t *testing.T) {
	if officeCLIReleaseVersion != "v1.0.152" {
		t.Errorf("the pinned OfficeCLI version is %q, want v1.0.152", officeCLIReleaseVersion)
	}
	wantBase := "https://github.com/iOfficeAI/OfficeCLI/releases/download/v1.0.152/"
	if officeCLIReleaseBaseURL != wantBase {
		t.Errorf("the OfficeCLI release base URL is %q, want %q", officeCLIReleaseBaseURL, wantBase)
	}

	cases := []struct {
		goos, goarch string
		alpine       bool
		name         string
		sha256       string
	}{
		{"linux", "amd64", false, "officecli-linux-x64", "e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9"},
		{"linux", "arm64", false, "officecli-linux-arm64", "bc06deaa0ad931f5208717a40b94018dc44cdff0d8eefa842c4f4daf89fb35a8"},
		{"linux", "amd64", true, "officecli-linux-alpine-x64", "390e246303bf43b4739e3195e9b171a660223b4c11218b65894d5fadf9a52755"},
		{"linux", "arm64", true, "officecli-linux-alpine-arm64", "65c65e05100bac1376e23f6ca97086a046afdfcaf50f2bc215a8c139b3bc22ba"},
		{"darwin", "amd64", false, "officecli-mac-x64", "5071abef56c1d4a4d60e28ed12bc66183d8dc6a9783529c3f1a9cf6bdfe6c2dd"},
		{"darwin", "arm64", false, "officecli-mac-arm64", "e2ed6eba5cd46d6800139f2835097828b8ccd7c8c9b679463b50e45ba2f1dbf5"},
	}
	for _, c := range cases {
		asset, ok := officeCLIAssetForPlatform(c.goos, c.goarch, c.alpine)
		if !ok {
			t.Errorf("no asset for %s/%s (alpine=%v)", c.goos, c.goarch, c.alpine)
			continue
		}
		if asset.name != c.name {
			t.Errorf("asset for %s/%s (alpine=%v) is %q, want %q", c.goos, c.goarch, c.alpine, asset.name, c.name)
		}
		if asset.sha256 != c.sha256 {
			t.Errorf("SHA-256 for %s is %q, want %q", c.name, asset.sha256, c.sha256)
		}
		if url := officeCLIAssetURL(asset.name); url != wantBase+c.name {
			t.Errorf("asset URL for %s is %q, want %q", c.name, url, wantBase+c.name)
		}
	}
}

// TestOfficeCLIAssetForPlatformRejectsUnsupported covers the fail-closed path:
// a platform the release does not build for yields no asset.
func TestOfficeCLIAssetForPlatformRejectsUnsupported(t *testing.T) {
	cases := []struct {
		goos, goarch string
		alpine       bool
	}{
		{"windows", "amd64", false},
		{"freebsd", "amd64", false},
		{"linux", "386", false},
		{"linux", "arm", false},
		{"darwin", "386", false},
		{"plan9", "amd64", false},
	}
	for _, c := range cases {
		if asset, ok := officeCLIAssetForPlatform(c.goos, c.goarch, c.alpine); ok {
			t.Errorf("officeCLIAssetForPlatform accepted unsupported %s/%s and returned %q", c.goos, c.goarch, asset.name)
		}
	}
}

// TestIsAlpineLinuxUsesMarkerFile pins the Alpine probe to the marker file so
// the musl asset is chosen only on a real Alpine host.
func TestIsAlpineLinuxUsesMarkerFile(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "alpine-release")
	t.Setenv(envAlpineReleasePath, marker)

	if isAlpineLinux() {
		t.Error("isAlpineLinux reported Alpine while the marker file does not exist")
	}
	if err := os.WriteFile(marker, []byte("3.20.0\n"), 0o644); err != nil {
		t.Fatalf("could not write the marker fixture: %v", err)
	}
	if !isAlpineLinux() {
		t.Error("isAlpineLinux did not report Alpine while the marker file exists")
	}
}

// TestOfficeCLIStepIsScheduledAndDispatchable pins the wiring: both schedulers
// put the step on a run and the executor table can run it.
func TestOfficeCLIStepIsScheduledAndDispatchable(t *testing.T) {
	if _, ok := stepExecutors[officeCLIStepID]; !ok {
		t.Fatalf("the executor table has no entry for %q", officeCLIStepID)
	}

	interactive := officeCLIModel()
	interactive.SetupInstallSteps()
	if !stepIDsContain(interactive.Steps, officeCLIStepID) {
		t.Errorf("SetupInstallSteps does not schedule %q", officeCLIStepID)
	}

	nonInteractive := officeCLIModel()
	if !stepIDsContain(buildStepsForChoices(nonInteractive), officeCLIStepID) {
		t.Errorf("buildStepsForChoices does not schedule %q", officeCLIStepID)
	}
}

// TestStepInstallOfficeCLIDownloadsVerifiedBinary covers the happy path: the
// pinned URL is fetched, the verified bytes land at ~/.local/bin/officecli with
// the executable bit, and no staging directory survives.
func TestStepInstallOfficeCLIDownloadsVerifiedBinary(t *testing.T) {
	fixture := writeOfficeCLIFixture(t, "officecli binary bytes")
	stubs := stubOfficeCLICommands(t, fixture, 0)
	withOfficeCLIPlatform(t, "linux", "amd64", false)
	withOfficeCLIAsset(t, officeCLIAsset{name: "officecli-linux-x64", sha256: sha256HexOfFile(t, fixture)})

	if err := stepInstallOfficeCLI(officeCLIModel()); err != nil {
		t.Fatalf("stepInstallOfficeCLI failed: %v", err)
	}

	dest := filepath.Join(stubs.binDir, "officecli")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected the binary at %s: %v", dest, err)
	}
	if string(data) != "officecli binary bytes" {
		t.Errorf("the installed binary holds %q", data)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("could not stat the installed binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("the installed binary mode is %v, want 0755", info.Mode().Perm())
	}

	commands := stubs.ranCommands(t)
	if len(commands) != 1 {
		t.Fatalf("expected exactly one download, got %d: %v", len(commands), commands)
	}
	wantURL := "https://github.com/iOfficeAI/OfficeCLI/releases/download/v1.0.152/officecli-linux-x64"
	if !strings.Contains(commands[0], wantURL) {
		t.Errorf("the download did not use the pinned URL %q: %v", wantURL, commands)
	}
	stubs.assertNoStagingLeft(t)
}

// TestStepInstallOfficeCLIIsIdempotent runs the step twice: the second run sees
// the installed binary and downloads nothing.
func TestStepInstallOfficeCLIIsIdempotent(t *testing.T) {
	fixture := writeOfficeCLIFixture(t, "officecli binary bytes")
	stubs := stubOfficeCLICommands(t, fixture, 0)
	withOfficeCLIPlatform(t, "linux", "amd64", false)
	withOfficeCLIAsset(t, officeCLIAsset{name: "officecli-linux-x64", sha256: sha256HexOfFile(t, fixture)})

	if err := stepInstallOfficeCLI(officeCLIModel()); err != nil {
		t.Fatalf("the first run failed: %v", err)
	}
	if err := stepInstallOfficeCLI(officeCLIModel()); err != nil {
		t.Fatalf("the second run failed: %v", err)
	}

	if commands := stubs.ranCommands(t); len(commands) != 1 {
		t.Errorf("the idempotent second run downloaded again: %v", commands)
	}
	stubs.assertNoStagingLeft(t)
}

// TestStepInstallOfficeCLIPreservesExistingBinary is the user-data guarantee: a
// binary that is already at the destination is left byte-for-byte untouched and
// no download is attempted.
func TestStepInstallOfficeCLIPreservesExistingBinary(t *testing.T) {
	stubs := stubOfficeCLICommands(t, "", 0)
	withOfficeCLIPlatform(t, "linux", "amd64", false)
	withOfficeCLIAsset(t, officeCLIAsset{name: "officecli-linux-x64", sha256: strings.Repeat("0", 64)})

	dest := filepath.Join(stubs.binDir, "officecli")
	if err := os.MkdirAll(stubs.binDir, 0o755); err != nil {
		t.Fatalf("could not create the destination directory: %v", err)
	}
	if err := os.WriteFile(dest, []byte("user binary"), 0o755); err != nil {
		t.Fatalf("could not write the existing binary: %v", err)
	}

	if err := stepInstallOfficeCLI(officeCLIModel()); err != nil {
		t.Fatalf("an existing binary must be skipped, not failed: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("the existing binary disappeared: %v", err)
	}
	if string(data) != "user binary" {
		t.Errorf("the existing binary was overwritten: %q", data)
	}
	if commands := stubs.ranCommands(t); len(commands) > 0 {
		t.Errorf("the step downloaded for an existing binary: %v", commands)
	}
	stubs.assertNoStagingLeft(t)
}

// TestStepInstallOfficeCLIRejectsChecksumMismatch covers a tampered or stale
// transfer against the real pinned digest: the step fails, leaves no target and
// cleans its staging directory.
func TestStepInstallOfficeCLIRejectsChecksumMismatch(t *testing.T) {
	fixture := writeOfficeCLIFixture(t, "tampered bytes")
	stubs := stubOfficeCLICommands(t, fixture, 0)
	withOfficeCLIPlatform(t, "linux", "amd64", false)

	err := stepInstallOfficeCLI(officeCLIModel())
	if err == nil {
		t.Fatal("expected a checksum failure")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("the failure does not name the checksum mismatch: %v", err)
	}
	if system.PathExists(filepath.Join(stubs.binDir, "officecli")) {
		t.Error("a binary whose checksum did not match was installed")
	}
	stubs.assertNoStagingLeft(t)
}

// TestStepInstallOfficeCLIReportsFailedDownload covers curl itself failing: the
// step reports it, installs nothing and cleans its staging directory.
func TestStepInstallOfficeCLIReportsFailedDownload(t *testing.T) {
	stubs := stubOfficeCLICommands(t, "", 22)
	withOfficeCLIPlatform(t, "linux", "amd64", false)

	err := stepInstallOfficeCLI(officeCLIModel())
	if err == nil {
		t.Fatal("expected a download failure")
	}
	if system.PathExists(filepath.Join(stubs.binDir, "officecli")) {
		t.Error("a binary whose download failed was installed")
	}
	stubs.assertNoStagingLeft(t)
}

// TestStepInstallOfficeCLISkipsTermux is the graceful skip: Termux has no
// release asset, so the step succeeds without a download.
func TestStepInstallOfficeCLISkipsTermux(t *testing.T) {
	stubs := stubOfficeCLICommands(t, "", 0)

	m := officeCLIModel()
	m.SystemInfo.IsTermux = true

	if err := stepInstallOfficeCLI(m); err != nil {
		t.Fatalf("Termux must be skipped, not failed: %v", err)
	}
	if commands := stubs.ranCommands(t); len(commands) > 0 {
		t.Errorf("the step downloaded on Termux: %v", commands)
	}
	if system.PathExists(filepath.Join(stubs.binDir, "officecli")) {
		t.Error("the step installed a binary on Termux")
	}
}

// TestStepInstallOfficeCLIFailsUnsupportedPlatform is the fail-closed path: an
// OS/architecture the release does not build for fails clearly instead of
// installing an unverified or incompatible binary.
func TestStepInstallOfficeCLIFailsUnsupportedPlatform(t *testing.T) {
	stubs := stubOfficeCLICommands(t, "", 0)
	withOfficeCLIPlatform(t, "windows", "amd64", false)

	err := stepInstallOfficeCLI(officeCLIModel())
	if err == nil {
		t.Fatal("expected an unsupported-platform failure")
	}
	if !strings.Contains(err.Error(), "no release asset") {
		t.Errorf("the failure does not name the missing asset: %v", err)
	}
	if commands := stubs.ranCommands(t); len(commands) > 0 {
		t.Errorf("the step downloaded for an unsupported platform: %v", commands)
	}
	if system.PathExists(filepath.Join(stubs.binDir, "officecli")) {
		t.Error("the step installed a binary for an unsupported platform")
	}
}
