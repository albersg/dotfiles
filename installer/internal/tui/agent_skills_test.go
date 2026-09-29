package tui

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The skill step shells out to curl, so these tests put a stub curl first on
// PATH and hand it a locally generated fixture. The step then runs its real
// download, checksum, staging and rename sequence against real files while the
// network stays untouched.
const (
	agentSkillCurlLogEnv     = "AGENT_SKILL_CURL_LOG"
	agentSkillCurlFixtureEnv = "AGENT_SKILL_CURL_FIXTURE"
	agentSkillCurlExitEnv    = "AGENT_SKILL_CURL_EXIT"
)

var stubAgentSkillCurl = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ -n "$out" ] && [ -n "$%s" ]; then cp "$%s" "$out"; fi
exit "${%s:-0}"
`, agentSkillCurlLogEnv, agentSkillCurlFixtureEnv, agentSkillCurlFixtureEnv, agentSkillCurlExitEnv)

type agentSkillStubs struct {
	home      string
	log       string
	skillsDir string
}

func stubAgentSkillCommands(t *testing.T, fixture string, exitCode int) *agentSkillStubs {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	logPath := filepath.Join(t.TempDir(), "agent-skill-curl.log")
	t.Setenv(agentSkillCurlLogEnv, logPath)
	t.Setenv(agentSkillCurlFixtureEnv, fixture)
	t.Setenv(agentSkillCurlExitEnv, fmt.Sprintf("%d", exitCode))

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "curl", stubAgentSkillCurl)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &agentSkillStubs{
		home:      home,
		log:       logPath,
		skillsDir: filepath.Join(home, ".pi", "agent", "skills"),
	}
}

// ranCommands returns the curl invocations the step actually made. An absent log
// means curl never ran.
func (s *agentSkillStubs) ranCommands(t *testing.T) []string {
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

// withAgentSkillPackages swaps the pinned package list for a test fixture and
// restores it afterwards.
func withAgentSkillPackages(t *testing.T, packages ...agentSkillPackage) {
	t.Helper()
	original := agentSkillPackages
	agentSkillPackages = packages
	t.Cleanup(func() { agentSkillPackages = original })
}

type zipFixtureEntry struct {
	name string
	body string
	mode os.FileMode
}

// writeAgentSkillZip builds a real ZIP archive and returns its path.
func writeAgentSkillZip(t *testing.T, entries []zipFixtureEntry) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "fixture.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("could not create the fixture archive: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		w, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("could not add %s to the fixture archive: %v", entry.name, err)
		}
		if _, err := io.WriteString(w, entry.body); err != nil {
			t.Fatalf("could not write %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("could not close the fixture archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("could not close the fixture file: %v", err)
	}
	return archivePath
}

func writeAgentSkillFixtureFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("could not write the fixture %s: %v", name, err)
	}
	return path
}

func sha256HexOfFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func agentSkillModel() *Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
	m.Choices = UserChoices{OS: "linux"}
	return &m
}

func assertAgentSkillFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	if string(data) != want {
		t.Errorf("%s holds %q, want %q", path, data, want)
	}
}

// TestAgentSkillPackagesArePinned locks the immutable sources and layout the
// installer trusts. A changed upstream ref or a stale checksum fails here before
// it can reach a download.
func TestAgentSkillPackagesArePinned(t *testing.T) {
	if securityAuditSkill.name != "security-audit" {
		t.Errorf("security-audit package name is %q", securityAuditSkill.name)
	}
	if securityAuditSkill.archiveURL != "https://codeload.github.com/Cloudflare/security-audit-skill/zip/c1c8a8c1471069fb0e188eeaff69b8e8db6564a8" {
		t.Errorf("security-audit archive URL is %q", securityAuditSkill.archiveURL)
	}
	if securityAuditSkill.archiveSHA256 != "18b53d57762ce312c8438e4252fdd72e33533fc50c7287d4d9eda2588f9c729d" {
		t.Errorf("security-audit archive SHA-256 is %q", securityAuditSkill.archiveSHA256)
	}
	if securityAuditSkill.packagePath != "skills/security-audit" {
		t.Errorf("security-audit package path is %q", securityAuditSkill.packagePath)
	}
	if len(securityAuditSkill.archiveExtra) != 1 ||
		securityAuditSkill.archiveExtra[0].source != "LICENSE" ||
		securityAuditSkill.archiveExtra[0].dest != "LICENSE" {
		t.Errorf("security-audit extras are %+v, want the archive root LICENSE", securityAuditSkill.archiveExtra)
	}

	if archifySkill.name != "archify" {
		t.Errorf("archify package name is %q", archifySkill.name)
	}
	if archifySkill.archiveURL != "https://codeload.github.com/tt-a1i/archify/zip/9e35d2b0b39b155553ba9fcfe0b4f2a5198dd993" {
		t.Errorf("archify archive URL is %q", archifySkill.archiveURL)
	}
	if archifySkill.archiveSHA256 != "2bb330db382f281247ad490c0a0291913b9f9c10cd357426ee4338e84334c3f5" {
		t.Errorf("archify archive SHA-256 is %q", archifySkill.archiveSHA256)
	}
	if archifySkill.packagePath != "archify" {
		t.Errorf("archify package path is %q", archifySkill.packagePath)
	}

	if officeCLISkill.name != "officecli" || officeCLISkill.kind != agentSkillFiles {
		t.Fatalf("officecli package is %+v, want a pinned-file package", officeCLISkill)
	}
	if len(officeCLISkill.files) != 2 {
		t.Fatalf("officecli has %d pinned files, want 2", len(officeCLISkill.files))
	}
	byDest := map[string]agentSkillFile{}
	for _, file := range officeCLISkill.files {
		byDest[file.dest] = file
	}
	license, ok := byDest["LICENSE"]
	if !ok {
		t.Fatal("officecli is missing the pinned LICENSE")
	}
	if license.url != officeCLISkillFileURL("LICENSE") {
		t.Errorf("officecli LICENSE URL is %q", license.url)
	}
	if license.sha256 != "7e282402a5a6db33995fe638bb3fe79013f9884d8f7d15a42e481c1e86aadda1" {
		t.Errorf("officecli LICENSE SHA-256 is %q", license.sha256)
	}
	manifest, ok := byDest["SKILL.md"]
	if !ok {
		t.Fatal("officecli is missing the pinned SKILL.md")
	}
	if manifest.url != officeCLISkillFileURL("skills/officecli/SKILL.md") {
		t.Errorf("officecli SKILL.md URL is %q", manifest.url)
	}
	if manifest.sha256 != "c950d285ce60021712b4753fb2d9f592308d5622bab776229061dfecb1ce55d4" {
		t.Errorf("officecli SKILL.md SHA-256 is %q", manifest.sha256)
	}
	if !strings.Contains(manifest.url, "ffa8a0afbe2e9686abd636368e3da38c50f22131") {
		t.Errorf("officecli SKILL.md URL is not pinned to the recorded commit: %q", manifest.url)
	}
}

// TestAgentSkillStepIsScheduledAndDispatchable pins the wiring: both schedulers
// put the step on a run and the executor table can run it.
func TestAgentSkillStepIsScheduledAndDispatchable(t *testing.T) {
	if _, ok := stepExecutors[agentSkillsStepID]; !ok {
		t.Fatalf("the executor table has no entry for %q", agentSkillsStepID)
	}

	interactive := agentSkillModel()
	interactive.SetupInstallSteps()
	if !stepIDsContain(interactive.Steps, agentSkillsStepID) {
		t.Errorf("SetupInstallSteps does not schedule %q", agentSkillsStepID)
	}

	nonInteractive := agentSkillModel()
	if !stepIDsContain(buildStepsForChoices(nonInteractive), agentSkillsStepID) {
		t.Errorf("buildStepsForChoices does not schedule %q", agentSkillsStepID)
	}
}

func stepIDsContain(steps []InstallStep, id string) bool {
	for _, step := range steps {
		if step.ID == id {
			return true
		}
	}
	return false
}

// TestStepInstallAgentSkillsInstallsVerifiedZipPackage covers the happy path for
// an archive source: the package subtree and the archive-root licence land in
// the destination, content outside the subtree is ignored, the executable bit is
// preserved and the staging directory is gone.
func TestStepInstallAgentSkillsInstallsVerifiedZipPackage(t *testing.T) {
	fixture := writeAgentSkillZip(t, []zipFixtureEntry{
		{name: "security-audit-skill-c1c8a8c/skills/security-audit/SKILL.md", body: "manifest", mode: 0o644},
		{name: "security-audit-skill-c1c8a8c/skills/security-audit/scripts/run.sh", body: "#!/bin/sh\n", mode: 0o755},
		{name: "security-audit-skill-c1c8a8c/LICENSE", body: "licence", mode: 0o644},
		{name: "security-audit-skill-c1c8a8c/other/ignored.txt", body: "ignored", mode: 0o644},
	})
	stubs := stubAgentSkillCommands(t, fixture, 0)
	withAgentSkillPackages(t, agentSkillPackage{
		name:          "security-audit",
		kind:          agentSkillZip,
		archiveURL:    "https://example.invalid/security-audit.zip",
		archiveSHA256: sha256HexOfFile(t, fixture),
		packagePath:   "skills/security-audit",
		archiveExtra:  []agentSkillExtra{{source: "LICENSE", dest: "LICENSE"}},
	})

	if err := stepInstallAgentSkills(agentSkillModel()); err != nil {
		t.Fatalf("stepInstallAgentSkills failed: %v", err)
	}

	dest := filepath.Join(stubs.skillsDir, "security-audit")
	assertAgentSkillFile(t, filepath.Join(dest, "SKILL.md"), "manifest")
	assertAgentSkillFile(t, filepath.Join(dest, "LICENSE"), "licence")
	assertAgentSkillFile(t, filepath.Join(dest, "scripts", "run.sh"), "#!/bin/sh\n")

	if _, err := os.Stat(filepath.Join(dest, "other")); !os.IsNotExist(err) {
		t.Errorf("content outside the package subtree was installed: %v", err)
	}

	info, err := os.Stat(filepath.Join(dest, "scripts", "run.sh"))
	if err != nil {
		t.Fatalf("could not stat the extracted script: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("the script mode is %v, want 0755", info.Mode().Perm())
	}

	entries, err := os.ReadDir(stubs.skillsDir)
	if err != nil {
		t.Fatalf("could not read the skills directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".staging-") {
			t.Errorf("the staging directory %s was left behind", entry.Name())
		}
	}
}

// TestStepInstallAgentSkillsInstallsVerifiedFilePackage covers the happy path for
// a set of commit-pinned files: the files land in the destination with their
// verified bytes, and the transport is a plain raw.githubusercontent.com URL at
// the pinned commit with no Accept header and no contents API request.
func TestStepInstallAgentSkillsInstallsVerifiedFilePackage(t *testing.T) {
	fixture := writeAgentSkillFixtureFile(t, "SKILL.md", "officecli skill")
	sum := sha256HexOfFile(t, fixture)
	stubs := stubAgentSkillCommands(t, fixture, 0)
	withAgentSkillPackages(t, agentSkillPackage{
		name: "officecli",
		kind: agentSkillFiles,
		files: []agentSkillFile{
			{url: officeCLISkillFileURL("LICENSE"), dest: "LICENSE", sha256: sum},
			{url: officeCLISkillFileURL("skills/officecli/SKILL.md"), dest: "SKILL.md", sha256: sum},
		},
	})

	if err := stepInstallAgentSkills(agentSkillModel()); err != nil {
		t.Fatalf("stepInstallAgentSkills failed: %v", err)
	}

	dest := filepath.Join(stubs.skillsDir, "officecli")
	assertAgentSkillFile(t, filepath.Join(dest, "LICENSE"), "officecli skill")
	assertAgentSkillFile(t, filepath.Join(dest, "SKILL.md"), "officecli skill")

	// The files are fetched by raw URL at the pinned commit, not through the
	// rate-limited contents API.
	wantURL := "https://raw.githubusercontent.com/iOfficeAI/OfficeCLI/" + officeCLISkillCommit + "/skills/officecli/SKILL.md"
	if gotURL := officeCLISkillFileURL("skills/officecli/SKILL.md"); gotURL != wantURL {
		t.Errorf("officeCLISkillFileURL returned %q, want %q", gotURL, wantURL)
	}

	commands := stubs.ranCommands(t)
	// A media-type Accept header is dead weight without the contents API; the
	// URL above is the whole transport contract.
	if anyCommandContains(commands, "Accept") {
		t.Errorf("the pinned-file download still sends an Accept header: %v", commands)
	}
	if anyCommandContains(commands, "api.github.com") {
		t.Errorf("the pinned-file download still uses the rate-limited contents API: %v", commands)
	}
}

func anyCommandContains(commands []string, needle string) bool {
	for _, command := range commands {
		if strings.Contains(command, needle) {
			return true
		}
	}
	return false
}

// TestStepInstallAgentSkillsRejectsChecksumMismatch covers a tampered or stale
// transfer: the step fails and installs nothing.
func TestStepInstallAgentSkillsRejectsChecksumMismatch(t *testing.T) {
	fixture := writeAgentSkillZip(t, []zipFixtureEntry{
		{name: "archify-9e35d2b/archify/SKILL.md", body: "tampered", mode: 0o644},
	})
	stubs := stubAgentSkillCommands(t, fixture, 0)
	withAgentSkillPackages(t, agentSkillPackage{
		name:          "archify",
		kind:          agentSkillZip,
		archiveURL:    "https://example.invalid/archify.zip",
		archiveSHA256: strings.Repeat("0", 64),
		packagePath:   "archify",
	})

	err := stepInstallAgentSkills(agentSkillModel())
	if err == nil {
		t.Fatal("expected a checksum failure")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("the failure does not name the checksum mismatch: %v", err)
	}
	if system.DirExists(filepath.Join(stubs.skillsDir, "archify")) {
		t.Error("a package whose checksum did not match was installed")
	}
}

// TestStepInstallAgentSkillsReportsFailedDownload covers curl itself failing:
// the step reports it and installs nothing.
func TestStepInstallAgentSkillsReportsFailedDownload(t *testing.T) {
	stubs := stubAgentSkillCommands(t, "", 22)
	withAgentSkillPackages(t, agentSkillPackage{
		name:          "archify",
		kind:          agentSkillZip,
		archiveURL:    "https://example.invalid/archify.zip",
		archiveSHA256: strings.Repeat("0", 64),
		packagePath:   "archify",
	})

	err := stepInstallAgentSkills(agentSkillModel())
	if err == nil {
		t.Fatal("expected a download failure")
	}
	if system.DirExists(filepath.Join(stubs.skillsDir, "archify")) {
		t.Error("a package whose download failed was installed")
	}
}

// TestStepInstallAgentSkillsPreservesExistingDestination is the user-data
// guarantee: an existing skill directory is skipped, not overwritten or removed,
// and no download is attempted for it.
func TestStepInstallAgentSkillsPreservesExistingDestination(t *testing.T) {
	stubs := stubAgentSkillCommands(t, "", 0)

	dest := filepath.Join(stubs.skillsDir, "security-audit")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("could not create the existing destination: %v", err)
	}
	sentinel := filepath.Join(dest, "USER-NOTE.md")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("could not write the sentinel: %v", err)
	}

	withAgentSkillPackages(t, agentSkillPackage{
		name:          "security-audit",
		kind:          agentSkillZip,
		archiveURL:    "https://example.invalid/security-audit.zip",
		archiveSHA256: strings.Repeat("0", 64),
		packagePath:   "skills/security-audit",
	})

	if err := stepInstallAgentSkills(agentSkillModel()); err != nil {
		t.Fatalf("an existing destination must be skipped, not failed: %v", err)
	}

	assertAgentSkillFile(t, sentinel, "keep me")
	if commands := stubs.ranCommands(t); len(commands) > 0 {
		t.Errorf("the step downloaded for an existing destination: %v", commands)
	}
}

// TestStepInstallAgentSkillsInstallsMissingAndSkipsExisting runs two packages at
// once: the missing one is installed and the existing one is left alone.
func TestStepInstallAgentSkillsInstallsMissingAndSkipsExisting(t *testing.T) {
	fixture := writeAgentSkillZip(t, []zipFixtureEntry{
		{name: "archify-9e35d2b/archify/SKILL.md", body: "archify", mode: 0o644},
	})
	stubs := stubAgentSkillCommands(t, fixture, 0)

	existing := filepath.Join(stubs.skillsDir, "security-audit")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatalf("could not create the existing destination: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existing, "SKILL.md"), []byte("user version"), 0o644); err != nil {
		t.Fatalf("could not write the existing manifest: %v", err)
	}

	withAgentSkillPackages(t,
		agentSkillPackage{
			name:          "security-audit",
			kind:          agentSkillZip,
			archiveURL:    "https://example.invalid/security-audit.zip",
			archiveSHA256: strings.Repeat("0", 64),
			packagePath:   "skills/security-audit",
		},
		agentSkillPackage{
			name:          "archify",
			kind:          agentSkillZip,
			archiveURL:    "https://example.invalid/archify.zip",
			archiveSHA256: sha256HexOfFile(t, fixture),
			packagePath:   "archify",
		},
	)

	if err := stepInstallAgentSkills(agentSkillModel()); err != nil {
		t.Fatalf("stepInstallAgentSkills failed: %v", err)
	}

	assertAgentSkillFile(t, filepath.Join(existing, "SKILL.md"), "user version")
	assertAgentSkillFile(t, filepath.Join(stubs.skillsDir, "archify", "SKILL.md"), "archify")

	commands := stubs.ranCommands(t)
	if len(commands) != 1 {
		t.Errorf("expected exactly one download, got %d: %v", len(commands), commands)
	}
}

// TestStepInstallAgentSkillsRejectsPathTraversal covers a hostile archive whose
// entry escapes the destination. The step fails and nothing is written outside
// the staging tree.
func TestStepInstallAgentSkillsRejectsPathTraversal(t *testing.T) {
	fixture := writeAgentSkillZip(t, []zipFixtureEntry{
		{name: "archify-9e35d2b/../../escape.txt", body: "escaped", mode: 0o644},
	})
	stubs := stubAgentSkillCommands(t, fixture, 0)
	withAgentSkillPackages(t, agentSkillPackage{
		name:          "archify",
		kind:          agentSkillZip,
		archiveURL:    "https://example.invalid/archify.zip",
		archiveSHA256: sha256HexOfFile(t, fixture),
		packagePath:   "archify",
	})

	err := stepInstallAgentSkills(agentSkillModel())
	if err == nil {
		t.Fatal("expected a path traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes the destination") {
		t.Errorf("the failure does not name the escape: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stubs.skillsDir, "escape.txt")); !os.IsNotExist(err) {
		t.Errorf("a traversing entry was written inside the skills directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stubs.home, ".pi", "escape.txt")); !os.IsNotExist(err) {
		t.Errorf("a traversing entry was written outside the skills directory: %v", err)
	}
	if system.DirExists(filepath.Join(stubs.skillsDir, "archify")) {
		t.Error("a package with a traversing entry was installed")
	}
}

// TestSafeArchivePathRejectsEscapes unit-tests the traversal defence directly.
func TestSafeArchivePathRejectsEscapes(t *testing.T) {
	base := t.TempDir()
	for _, rel := range []string{
		"../escape.txt",
		"../../etc/passwd",
		"sub/../../escape.txt",
		"/etc/passwd",
		"",
	} {
		if _, err := safeArchivePath(base, rel); err == nil {
			t.Errorf("safeArchivePath accepted the escaping entry %q", rel)
		}
	}

	target, err := safeArchivePath(base, "pkg/SKILL.md")
	if err != nil {
		t.Fatalf("safeArchivePath rejected a valid entry: %v", err)
	}
	if target != filepath.Join(base, "pkg", "SKILL.md") {
		t.Errorf("safeArchivePath returned %q", target)
	}
}
