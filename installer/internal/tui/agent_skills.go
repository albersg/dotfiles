package tui

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Pinned upstream sources for the Pi agent skills the installer provisions under
// ~/.pi/agent/skills.
//
// Every source is immutable. The two repositories that publish a whole tree are
// pinned to a commit archive, and the OfficeCLI skill is pinned to a commit in
// the GitHub contents API. The installer downloads each artifact, verifies the
// SHA-256 recorded here, and only then stages it for installation, so a changed
// upstream or a tampered transfer is refused rather than installed.
//
// The update process for these pins is documented in docs/ai-configuration.md
// ("Updating the pinned skill sources").
const (
	securityAuditSkillName = "security-audit"
	securityAuditSkillURL  = "https://codeload.github.com/Cloudflare/security-audit-skill/zip/c1c8a8c1471069fb0e188eeaff69b8e8db6564a8"
	securityAuditSkillSHA  = "18b53d57762ce312c8438e4252fdd72e33533fc50c7287d4d9eda2588f9c729d"
	// The skill package lives under skills/security-audit/ inside the archive
	// root, and its licence is the archive root LICENSE rather than a copy
	// inside the package directory.
	securityAuditSkillPackagePath = "skills/security-audit"
	securityAuditSkillLicensePath = "LICENSE"

	archifySkillName        = "archify"
	archifySkillURL         = "https://codeload.github.com/tt-a1i/archify/zip/9e35d2b0b39b155553ba9fcfe0b4f2a5198dd993"
	archifySkillSHA         = "2bb330db382f281247ad490c0a0291913b9f9c10cd357426ee4338e84334c3f5"
	archifySkillPackagePath = "archify"

	officeCLISkillName        = "officecli"
	officeCLISkillCommit      = "ffa8a0afbe2e9686abd636368e3da38c50f22131"
	officeCLISkillLicenseSHA  = "7e282402a5a6db33995fe638bb3fe79013f9884d8f7d15a42e481c1e86aadda1"
	officeCLISkillManifestSHA = "c950d285ce60021712b4753fb2d9f592308d5622bab776229061dfecb1ce55d4"
)

// agentSkillsStepID is the step that provisions the pinned Pi skills. It needs
// no sudo, so it runs through the shared executor rather than the interactive
// script builders.
const agentSkillsStepID = "agentskills"

// officeCLISkillFileURL builds the contents-API URL for one file at the pinned
// OfficeCLI commit. The API returns the raw bytes only for the media type sent
// with the request; see downloadToFile.
func officeCLISkillFileURL(filePath string) string {
	return fmt.Sprintf("https://api.github.com/repos/iOfficeAI/OfficeCLI/contents/%s?ref=%s", filePath, officeCLISkillCommit)
}

// agentSkillKind is the shape of a pinned source: a commit ZIP that contains a
// whole repository, or a set of individual commit-pinned files.
type agentSkillKind int

const (
	agentSkillZip agentSkillKind = iota
	agentSkillFiles
)

// agentSkillExtra is a file taken from the archive root and placed inside the
// installed package. The Cloudflare archive keeps its licence at the root while
// the package it belongs to lives in a subdirectory.
type agentSkillExtra struct {
	source string // archive-root-relative source path
	dest   string // destination-relative path
}

// agentSkillFile is one commit-pinned file fetched through the GitHub contents
// API and written to dest inside the installed package.
type agentSkillFile struct {
	url    string
	dest   string
	sha256 string
}

// agentSkillPackage is one skill destination and the pinned sources that fill it.
type agentSkillPackage struct {
	name string
	kind agentSkillKind

	// ZIP sources.
	archiveURL    string
	archiveSHA256 string
	packagePath   string // directory inside the archive root; "" means the root itself
	archiveExtra  []agentSkillExtra

	// Individual file sources.
	files []agentSkillFile
}

var (
	securityAuditSkill = agentSkillPackage{
		name:          securityAuditSkillName,
		kind:          agentSkillZip,
		archiveURL:    securityAuditSkillURL,
		archiveSHA256: securityAuditSkillSHA,
		packagePath:   securityAuditSkillPackagePath,
		archiveExtra: []agentSkillExtra{
			{source: securityAuditSkillLicensePath, dest: securityAuditSkillLicensePath},
		},
	}

	archifySkill = agentSkillPackage{
		name:          archifySkillName,
		kind:          agentSkillZip,
		archiveURL:    archifySkillURL,
		archiveSHA256: archifySkillSHA,
		packagePath:   archifySkillPackagePath,
	}

	officeCLISkill = agentSkillPackage{
		name: officeCLISkillName,
		kind: agentSkillFiles,
		files: []agentSkillFile{
			{
				url:    officeCLISkillFileURL("LICENSE"),
				dest:   "LICENSE",
				sha256: officeCLISkillLicenseSHA,
			},
			{
				url:    officeCLISkillFileURL("skills/officecli/SKILL.md"),
				dest:   "SKILL.md",
				sha256: officeCLISkillManifestSHA,
			},
		},
	}
)

// agentSkillPackages is the install order. It is a variable so tests can swap in
// fixtures with locally generated archives and hashes; production always uses
// the three pinned packages above.
var agentSkillPackages = []agentSkillPackage{securityAuditSkill, archifySkill, officeCLISkill}

// stepInstallAgentSkills provisions the pinned Pi skill packages under
// ~/.pi/agent/skills.
//
// It is idempotent and never discards user data: a destination that already
// exists is reported and left untouched, and only missing destinations are
// installed. Each package is staged and checksum-verified before anything is
// moved into place, so a failed download or a mismatch leaves the destination
// exactly as it was. One failing package does not stop the others.
func stepInstallAgentSkills(m *Model) error {
	stepID := agentSkillsStepID

	homeDir := os.Getenv("HOME")
	if homeDir == "" && m.SystemInfo != nil {
		homeDir = m.SystemInfo.HomeDir
	}
	if homeDir == "" {
		return wrapStepError(stepID, "Install Pi Agent Skills",
			"Cannot determine the home directory to install the Pi skills into",
			errors.New("HOME is not set"))
	}
	skillsDir := filepath.Join(homeDir, ".pi", "agent", "skills")

	SendLog(stepID, "Installing pinned Pi skill packages...")

	var failures []error
	skillsDirReady := false
	for _, pkg := range agentSkillPackages {
		dest := filepath.Join(skillsDir, pkg.name)
		if system.DirExists(dest) {
			SendLog(stepID, fmt.Sprintf("  → %s already exists at %s; leaving it untouched", pkg.name, dest))
			continue
		}

		if !skillsDirReady {
			if err := system.EnsureDir(skillsDir); err != nil {
				return wrapStepError(stepID, "Install Pi Agent Skills",
					"Failed to create the Pi skills directory",
					err)
			}
			skillsDirReady = true
		}

		if err := installAgentSkillPackage(pkg, skillsDir, stepID); err != nil {
			SendLog(stepID, fmt.Sprintf("  ✗ %s: %v", pkg.name, err))
			failures = append(failures, fmt.Errorf("%s: %w", pkg.name, err))
			continue
		}
		SendLog(stepID, fmt.Sprintf("  ✓ %s installed", pkg.name))
	}

	if len(failures) > 0 {
		return wrapStepError(stepID, "Install Pi Agent Skills",
			"One or more pinned Pi skill packages could not be installed",
			errors.Join(failures...))
	}

	SendLog(stepID, "✓ Pi skills provisioned")
	return nil
}

// installAgentSkillPackage stages one package in a scratch directory beside the
// destination, verifies every artifact, and moves the verified tree into place.
//
// The staging directory lives under skillsDir so the final move is a rename on
// the same filesystem, and it is removed on every path out. The destination is
// checked again immediately before the rename, so a directory that appeared
// while the download ran is never overwritten.
func installAgentSkillPackage(pkg agentSkillPackage, skillsDir, stepID string) error {
	dest := filepath.Join(skillsDir, pkg.name)
	if system.DirExists(dest) {
		SendLog(stepID, fmt.Sprintf("  → %s already exists; leaving it untouched", pkg.name))
		return nil
	}

	stagingRoot, err := os.MkdirTemp(skillsDir, ".staging-"+pkg.name+"-")
	if err != nil {
		return fmt.Errorf("failed to create a staging directory: %w", err)
	}
	defer func() {
		// A cleanup failure is reported but never changes the step's outcome.
		if err := os.RemoveAll(stagingRoot); err != nil {
			SendLog(stepID, fmt.Sprintf("Warning: could not remove the staging directory %s: %v", stagingRoot, err))
		}
	}()

	stagedPkg := filepath.Join(stagingRoot, pkg.name)
	if err := os.MkdirAll(stagedPkg, 0o755); err != nil {
		return fmt.Errorf("failed to create the staged package directory: %w", err)
	}

	SendLog(stepID, fmt.Sprintf("  Downloading %s...", pkg.name))
	switch pkg.kind {
	case agentSkillZip:
		if err := stageAgentSkillZip(pkg, stagingRoot, stagedPkg, stepID); err != nil {
			return err
		}
	case agentSkillFiles:
		if err := stageAgentSkillFiles(pkg, stagedPkg, stepID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown pinned source kind for %s", pkg.name)
	}

	if system.DirExists(dest) {
		SendLog(stepID, fmt.Sprintf("  → %s appeared while downloading; leaving it untouched", pkg.name))
		return nil
	}
	if err := os.Rename(stagedPkg, dest); err != nil {
		return fmt.Errorf("failed to install the staged package: %w", err)
	}
	return nil
}

func stageAgentSkillZip(pkg agentSkillPackage, stagingRoot, stagedPkg, stepID string) error {
	archivePath := filepath.Join(stagingRoot, pkg.name+".zip")
	if err := downloadToFile(pkg.archiveURL, archivePath, stepID, ""); err != nil {
		return fmt.Errorf("failed to download %s: %w", pkg.archiveURL, err)
	}
	if err := verifyFileSHA256(archivePath, pkg.archiveSHA256); err != nil {
		return fmt.Errorf("archive %s: %w", pkg.archiveURL, err)
	}
	if err := extractZipPackage(archivePath, pkg.packagePath, pkg.archiveExtra, stagedPkg); err != nil {
		return fmt.Errorf("failed to extract %s: %w", pkg.name, err)
	}
	return nil
}

func stageAgentSkillFiles(pkg agentSkillPackage, stagedPkg, stepID string) error {
	for _, file := range pkg.files {
		dest := filepath.Join(stagedPkg, filepath.FromSlash(file.dest))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(dest), err)
		}
		// The GitHub contents API returns the raw bytes only for the media type
		// below; without it the response is a JSON envelope and the checksum
		// would never match.
		if err := downloadToFile(file.url, dest, stepID, "application/vnd.github.raw"); err != nil {
			return fmt.Errorf("failed to download %s: %w", file.url, err)
		}
		if err := verifyFileSHA256(dest, file.sha256); err != nil {
			return fmt.Errorf("%s: %w", file.dest, err)
		}
	}
	return nil
}

// downloadToFile fetches url with curl, exactly as the rest of the installer
// does, so curl's own exit status is preserved and a failed transfer is seen
// rather than swallowed. accept adds an Accept header when the source needs one
// (the GitHub contents API).
func downloadToFile(url, dest, stepID, accept string) error {
	command := "curl -fsSL"
	if accept != "" {
		command += fmt.Sprintf(" -H %q", "Accept: "+accept)
	}
	command += fmt.Sprintf(" -o %q %q", dest, url)
	result := system.RunWithLogs(command, nil, func(line string) {
		SendLog(stepID, line)
	})
	return result.Error
}

// verifyFileSHA256 refuses a file whose bytes do not hash to expected.
func verifyFileSHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// stripArchiveRoot removes the single top-level directory codeload wraps every
// repository archive in ("<repo>-<commit>/"). An entry that has no wrapper, or
// whose first component is not a real directory name, yields ok=false.
func stripArchiveRoot(name string) (string, bool) {
	cleaned := strings.TrimPrefix(name, "./")
	idx := strings.IndexByte(cleaned, '/')
	if idx <= 0 {
		return "", false
	}
	root := cleaned[:idx]
	if root == "." || root == ".." {
		return "", false
	}
	return cleaned[idx+1:], true
}

// safeArchivePath joins a destination-relative archive path onto base and
// refuses anything that would resolve outside base. A hostile or malformed
// entry such as "../../etc/passwd" or an absolute path is rejected instead of
// written, so extraction can never escape the staging directory.
func safeArchivePath(base, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("archive entry has an empty path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(cleaned) || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes the destination: %q", rel)
	}
	target := filepath.Join(base, cleaned)
	within, err := filepath.Rel(base, target)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes the destination: %q", rel)
	}
	return target, nil
}

// extractZipPackage writes the package subtree of a codeload archive into dest.
// Codeload wraps every repository in a single top-level directory, so that
// prefix is stripped before the package path is matched; entries outside the
// package subtree are ignored except for the extras named by the caller. Every
// entry is validated with safeArchivePath first, and each file keeps the
// permission bits the archive recorded.
func extractZipPackage(archivePath, packagePath string, extra []agentSkillExtra, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	extraBySource := make(map[string]string, len(extra))
	for _, e := range extra {
		extraBySource[path.Clean(e.source)] = e.dest
	}

	packageDir := ""
	if packagePath != "" {
		packageDir = path.Clean(packagePath)
	}

	for _, entry := range reader.File {
		rel, ok := stripArchiveRoot(entry.Name)
		if !ok || rel == "" {
			continue
		}
		// Reject an entry that resolves outside the staging directory before it
		// is considered for extraction, so a hostile archive aborts the package
		// instead of writing through "../" or an absolute path.
		if _, err := safeArchivePath(dest, rel); err != nil {
			return err
		}

		cleaned := path.Clean(rel)
		destRel := ""
		switch {
		case packageDir == "":
			destRel = cleaned
		case cleaned == packageDir:
			continue // the package directory itself; created on demand
		case strings.HasPrefix(cleaned, packageDir+"/"):
			destRel = strings.TrimPrefix(cleaned, packageDir+"/")
		default:
			if mapped, ok := extraBySource[cleaned]; ok {
				destRel = mapped
			} else {
				continue
			}
		}

		if destRel == "" || entry.FileInfo().IsDir() {
			continue
		}
		if err := extractZipEntry(entry, dest, destRel); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(entry *zip.File, dest, rel string) error {
	target, err := safeArchivePath(dest, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	mode := entry.Mode()
	if mode == 0 {
		mode = 0o644
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, source); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
