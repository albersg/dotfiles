package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ampleSpace is an injected FreeSpace that never gets in the way. Tests that care about the space
// rule supply their own reading; tests about anything else use this so a real, nearly full developer
// disk cannot fail an unrelated case.
func ampleSpace(string) (free, total uint64, ok bool) { return 1 << 40, 1 << 40, true }

// writeMigrationFile creates a file with an exact mode, creating parents as needed. os.WriteFile
// alone is not enough because the umask would silently drop bits a mode test is checking.
func writeMigrationFile(t *testing.T, path, content string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// migrationState is one source entry's identity: everything about it a migration must not change.
// atime is deliberately absent, because reading a file is exactly what a preview does and it would
// make the snapshot compare a change this package is allowed to cause.
type migrationState struct {
	kind   string
	size   int64
	perm   fs.FileMode
	mtime  int64
	target string
	hash   string
}

// snapshotMigrationTree records every path under root. A migration that changes the source in any
// way -- content, mode, mtime, type, link target, or the presence of a path -- makes two snapshots
// differ, and the rule-1 test fails.
func snapshotMigrationTree(t *testing.T, root string) map[string]migrationState {
	t.Helper()
	out := map[string]migrationState{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		state := migrationState{size: info.Size(), perm: info.Mode().Perm(), mtime: info.ModTime().UnixNano()}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			state.kind = "symlink"
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			state.target = target
		case d.IsDir():
			state.kind = "dir"
		case d.Type().IsRegular():
			state.kind = "file"
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				// A file the test itself made unreadable cannot be hashed; size, mode and mtime still pin
				// it, and "unreadable" compares equal on both sides of the run.
				state.hash = "unreadable"
				break
			}
			sum := sha256.Sum256(data)
			state.hash = hex.EncodeToString(sum[:])
		default:
			state.kind = "other"
		}
		out[filepath.ToSlash(rel)] = state
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// assertSameSource fails when two snapshots of one tree differ in any way.
func assertSameSource(t *testing.T, before, after map[string]migrationState) {
	t.Helper()
	for rel, want := range before {
		got, ok := after[rel]
		if !ok {
			t.Errorf("source entry %s disappeared", rel)
			continue
		}
		if got != want {
			t.Errorf("source entry %s changed:\n before %+v\n  after %+v", rel, want, got)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("source gained entry %s", rel)
		}
	}
}

// buildCopiedMap mirrors the set RunMigration records, so verification can be exercised against a
// destination a test has tampered with.
func buildCopiedMap(t *testing.T, root string) map[string]copiedEntry {
	t.Helper()
	out := map[string]copiedEntry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			out[filepath.ToSlash(rel)] = copiedEntry{kind: kindSymlink, linkTarget: target}
		case d.IsDir():
			out[filepath.ToSlash(rel)] = copiedEntry{kind: kindDir, perm: info.Mode().Perm()}
		default:
			out[filepath.ToSlash(rel)] = copiedEntry{kind: kindFile, size: info.Size(), perm: info.Mode().Perm()}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("buildCopiedMap %s: %v", root, err)
	}
	return out
}

func containsAny(lines []string, substrings ...string) bool {
	for _, line := range lines {
		for _, want := range substrings {
			if strings.Contains(line, want) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The preview: PlanMigration
// ---------------------------------------------------------------------------

func TestPlanMigrationRefusesNonsenseLayouts(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "project")
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	if err := os.MkdirAll(filepath.Join(source, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		source      string
		destination string
		want        string
	}{
		{"source equals destination", source, source, "same path"},
		{"destination inside source", source, filepath.Join(source, "out"), "destination is inside"},
		{"source inside destination", filepath.Join(source, "sub"), source, "source is inside"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := PlanMigration(tt.source, tt.destination, MigrationOptions{FreeSpace: ampleSpace})
			if err != nil {
				t.Fatalf("PlanMigration: %v", err)
			}
			if plan.Runnable() {
				t.Fatalf("plan is Runnable, want blocked; blockers=%v", plan.Blockers)
			}
			if !containsAny(plan.Blockers, tt.want) {
				t.Errorf("blockers = %v, want one containing %q", plan.Blockers, tt.want)
			}
		})
	}
}

func TestPlanMigrationRefusesADestinationInsideTheSourceThroughASymlink(t *testing.T) {
	// The layout check has to hold on a path whose ancestor is a symlink, because that is the shape
	// this failure really arrives in: macOS resolves its temporary directory to
	// /private/var/folders/..., a destination that does not exist yet can only be resolved through
	// the ancestor that does, and comparing an unresolved spelling against a resolved source is how
	// "destination inside the source" reads as Runnable -- the copy would then read what it writes.
	base := t.TempDir()
	source := filepath.Join(base, "project")
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)

	inside := filepath.Join(base, "link to the project")
	if err := os.Symlink(source, inside); err != nil {
		t.Skipf("this platform cannot create a symlink in the temporary directory: %v", err)
	}
	plan, err := PlanMigration(source, filepath.Join(inside, "out"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.Runnable() {
		t.Fatalf("a destination inside the source through a symlink is Runnable; blockers=%v", plan.Blockers)
	}
	if !containsAny(plan.Blockers, "destination is inside") {
		t.Errorf("blockers = %v, want one containing %q", plan.Blockers, "destination is inside")
	}

	// The other direction, so the fix cannot be "refuse every path reached through a link": a
	// destination whose ancestor is a symlink to somewhere else is a perfectly ordinary copy.
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "link to elsewhere")
	if err := os.Symlink(elsewhere, outside); err != nil {
		t.Skipf("this platform cannot create a symlink in the temporary directory: %v", err)
	}
	ordinary, err := PlanMigration(source, filepath.Join(outside, "copy"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if !ordinary.Runnable() {
		t.Errorf("a destination reached through an unrelated symlink is blocked; blockers=%v", ordinary.Blockers)
	}
}

func TestPlanMigrationBlocksANonEmptyDestinationAndCreatesNothing(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)

	occupied := t.TempDir()
	writeMigrationFile(t, filepath.Join(occupied, "already.txt"), "keep me", 0o644)

	plan, err := PlanMigration(source, occupied, MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.Runnable() {
		t.Fatalf("a non-empty destination must block; blockers=%v", plan.Blockers)
	}
	if !containsAny(plan.Blockers, "already holds something") {
		t.Errorf("blockers = %v, want the non-empty-destination wording", plan.Blockers)
	}

	// Planning a destination that does not exist must not bring it into existence: only RunMigration
	// creates, and only then.
	fresh := filepath.Join(t.TempDir(), "fresh")
	if _, err := PlanMigration(source, fresh, MigrationOptions{FreeSpace: ampleSpace}); err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if _, serr := os.Stat(fresh); !os.IsNotExist(serr) {
		t.Errorf("PlanMigration created the destination; stat error = %v", serr)
	}
}

func TestPlanMigrationExclusionsPruneComponentsAndRelativePaths(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	writeMigrationFile(t, filepath.Join(source, "node_modules", "a.js"), "a", 0o644)
	writeMigrationFile(t, filepath.Join(source, "node_modules", "deep", "b.js"), "b", 0o644)
	writeMigrationFile(t, filepath.Join(source, "build", "tmp", "x.txt"), "x", 0o644)
	writeMigrationFile(t, filepath.Join(source, "src", "node_modules", "y.js"), "y", 0o644)

	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{
		Exclusions: []string{"node_modules", "build/tmp"},
		FreeSpace:  ampleSpace,
	})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if !plan.Runnable() {
		t.Fatalf("plan not runnable: %v", plan.Blockers)
	}

	// keep.txt is the only file copied. Every node_modules directory at any depth is pruned, and the
	// separator pattern build/tmp is matched against the relative path, so build itself stays.
	if plan.Stats.Files != 1 {
		t.Errorf("Files = %d, want 1 (only keep.txt)", plan.Stats.Files)
	}
	if plan.Stats.Directories != 2 {
		t.Errorf("Directories = %d, want 2 (build and src)", plan.Stats.Directories)
	}
	if plan.Stats.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3 (node_modules, build/tmp, src/node_modules)", plan.Stats.Skipped)
	}
	if plan.Stats.Bytes != uint64(len("keep")) {
		t.Errorf("Bytes = %d, want %d", plan.Stats.Bytes, len("keep"))
	}
	if !strings.Contains(plan.Stats.Summary(), "skipped") {
		t.Errorf("Summary = %q, want it to mention skipped entries", plan.Stats.Summary())
	}
}

func TestPlanMigrationCountsFilesDirectoriesSymlinksHiddenAndGit(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "root.txt"), "hello", 0o644)
	writeMigrationFile(t, filepath.Join(source, "sub", "nested.txt"), "world!", 0o644)
	writeMigrationFile(t, filepath.Join(source, ".hidden"), "h", 0o644)
	writeMigrationFile(t, filepath.Join(source, ".git", "config"), "cfg", 0o644)
	if err := os.Symlink("root.txt", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	want := MigrationStats{
		Files:           4,
		Directories:     2,
		Symlinks:        1,
		Hidden:          2, // .hidden and .git
		GitRepositories: 1,
		Bytes:           5 + 6 + 1 + 3,
	}
	if plan.Stats != want {
		t.Errorf("Stats = %+v, want %+v", plan.Stats, want)
	}
}

func TestPlanMigrationWarnsAboutSymlinksThatWillNotSurviveTheMove(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "target.txt"), "t", 0o644)
	if err := os.Symlink(filepath.Join(source, "target.txt"), filepath.Join(source, "inside-abs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/hosts", filepath.Join(source, "outside-abs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "elsewhere"), filepath.Join(source, "outside-rel")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(source, "inside-rel")); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.Stats.Symlinks != 4 {
		t.Fatalf("Symlinks = %d, want 4", plan.Stats.Symlinks)
	}
	for _, name := range []string{"inside-abs", "outside-abs", "outside-rel"} {
		if !containsAny(plan.Warnings, name) {
			t.Errorf("warnings = %v, want one naming %s", plan.Warnings, name)
		}
	}
	if containsAny(plan.Warnings, "inside-rel") {
		t.Errorf("warnings = %v, want none for a relative symlink that resolves inside the source", plan.Warnings)
	}
}

func TestPlanMigrationWarnsAboutGitRepositories(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, ".git", "config"), "[core]", 0o644)

	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.Stats.GitRepositories != 1 {
		t.Fatalf("GitRepositories = %d, want 1", plan.Stats.GitRepositories)
	}
	if !containsAny(plan.Warnings, ".git/config") {
		t.Errorf("warnings = %v, want one naming .git/config", plan.Warnings)
	}
}

func TestPlanMigrationMirrorsSpaceWithoutFillingADisk(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "hello", 0o644) // 5 bytes

	tests := []struct {
		name         string
		space        func(string) (uint64, uint64, bool)
		wantBlocker  bool
		wantWarning  bool
		wantFree     uint64
		wantSpaceSet bool
	}{
		{"enough room", func(string) (uint64, uint64, bool) { return 1 << 20, 1 << 30, true }, false, false, 1 << 20, true},
		{"not enough room", func(string) (uint64, uint64, bool) { return 1, 1 << 30, true }, true, false, 1, true},
		{"space unknown", func(string) (uint64, uint64, bool) { return 0, 0, false }, false, true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: tt.space})
			if err != nil {
				t.Fatalf("PlanMigration: %v", err)
			}
			if got := !plan.Runnable(); got != tt.wantBlocker {
				t.Errorf("blocked = %v, want %v; blockers=%v", got, tt.wantBlocker, plan.Blockers)
			}
			if tt.wantBlocker && !containsAny(plan.Blockers, "free") {
				t.Errorf("blockers = %v, want the free-space wording", plan.Blockers)
			}
			if got := containsAny(plan.Warnings, "space"); got != tt.wantWarning {
				t.Errorf("space warning = %v, want %v; warnings=%v", got, tt.wantWarning, plan.Warnings)
			}
			if plan.FreeBytes != tt.wantFree || plan.SpaceKnown != tt.wantSpaceSet {
				t.Errorf("FreeBytes=%d SpaceKnown=%v, want %d/%v", plan.FreeBytes, plan.SpaceKnown, tt.wantFree, tt.wantSpaceSet)
			}
		})
	}
}

func TestPlanMigrationBlocksAnUnreadableSourceEntry(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permission bits, so unreadability cannot be produced")
	}
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "visible.txt"), "v", 0o644)
	writeMigrationFile(t, filepath.Join(source, "secret.txt"), "s", 0o000)

	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if plan.Runnable() {
		t.Fatalf("an unreadable source file must block; blockers=%v", plan.Blockers)
	}
	if !containsAny(plan.Blockers, "secret.txt") {
		t.Errorf("blockers = %v, want one naming secret.txt", plan.Blockers)
	}
}

func TestPlanMigrationReturnsErrorsOnlyForUsage(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "project")
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	fileNotDir := filepath.Join(base, "file.txt")
	writeMigrationFile(t, fileNotDir, "x", 0o644)

	tests := []struct {
		name        string
		source      string
		destination string
	}{
		{"empty source", "", filepath.Join(base, "dest")},
		{"empty destination", source, ""},
		{"source is a file", fileNotDir, filepath.Join(base, "dest")},
		{"source does not exist", filepath.Join(base, "missing"), filepath.Join(base, "dest")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PlanMigration(tt.source, tt.destination, MigrationOptions{FreeSpace: ampleSpace}); err == nil {
				t.Errorf("PlanMigration(%q, %q) = nil error, want a usage error", tt.source, tt.destination)
			}
		})
	}
}

func TestMigrationStatsSummaryReadsForZeroOneAndMany(t *testing.T) {
	tests := []struct {
		name  string
		stats MigrationStats
		want  string
	}{
		{
			name:  "nothing to copy",
			stats: MigrationStats{},
			want:  "0 files (0 B) in 0 directories",
		},
		{
			name:  "the smallest real project",
			stats: MigrationStats{Files: 1, Directories: 1, Bytes: 1024},
			want:  "1 file (1.0 KiB) in 1 directory",
		},
		{
			name:  "the documented example",
			stats: MigrationStats{Files: 12, Directories: 4, Symlinks: 1, GitRepositories: 1, Bytes: 3 << 20},
			want:  "12 files (3.0 MiB) in 4 directories, 1 symlink, and a Git repository",
		},
		{
			name: "hidden and skipped are named",
			stats: MigrationStats{
				Files: 2, Directories: 1, Symlinks: 2, Hidden: 1,
				Skipped: 3, SkippedBytes: 2 << 20, Bytes: 10,
			},
			want: "2 files (10 B) in 1 directory, 2 symlinks, 1 hidden entry (included), and 3 entries skipped (2.0 MiB)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stats.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellQuoteIsSafeForAPathThatIsNotPlain(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"/home/a b", "'/home/a b'"},
		{`it's a "test"`, `'it'\''s a "test"'`},
		{"/a$b", "'/a$b'"},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// The run: RunMigration
// ---------------------------------------------------------------------------

func TestRunMigrationCopiesAndVerifiesEverything(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "hello.txt"), "hello world", 0o640) // 11 bytes
	writeMigrationFile(t, filepath.Join(source, "sub", "small.txt"), "x", 0o600)    // 1 byte
	writeMigrationFile(t, filepath.Join(source, ".hidden"), "h", 0o644)             // 1 byte
	writeMigrationFile(t, filepath.Join(source, ".git", "config"), "cfg", 0o644)    // 3 bytes
	if err := os.Symlink("hello.txt", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(source, "linkdir")); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "dest")

	plan, err := PlanMigration(source, destination, MigrationOptions{FreeSpace: ampleSpace, VerifyLimit: 4})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if !plan.Runnable() {
		t.Fatalf("plan not runnable: %v", plan.Blockers)
	}

	before := snapshotMigrationTree(t, source)
	var progress []MigrationProgress
	report, err := RunMigration(context.Background(), plan, func(p MigrationProgress) { progress = append(progress, p) })
	if err != nil {
		t.Fatalf("RunMigration: %v", err)
	}
	if !report.Verified {
		t.Fatalf("Verified = false, Failures = %v", report.Failures)
	}
	if !report.SourceUntouched {
		t.Error("SourceUntouched = false")
	}

	if report.Copied.Files != 4 || report.Copied.Directories != 2 || report.Copied.Symlinks != 2 {
		t.Errorf("Copied = %+v, want 4 files, 2 directories, 2 symlinks", report.Copied)
	}
	if report.Copied.Hidden != 2 {
		t.Errorf("Copied.Hidden = %d, want 2 (.hidden and .git)", report.Copied.Hidden)
	}
	if report.Copied.GitRepositories != 1 {
		t.Errorf("Copied.GitRepositories = %d, want 1", report.Copied.GitRepositories)
	}
	// hello.txt (11 bytes) exceeds the 4-byte limit; small.txt, .hidden and .git/config are compared.
	if report.SizeOnlyFiles != 1 {
		t.Errorf("SizeOnlyFiles = %d, want 1", report.SizeOnlyFiles)
	}
	if report.VerifiedBytes != 1+1+3 {
		t.Errorf("VerifiedBytes = %d, want 5", report.VerifiedBytes)
	}
	if !containsAny(report.VerificationNotes, "byte-for-byte") || !containsAny(report.VerificationNotes, "size and mode") {
		t.Errorf("VerificationNotes = %v, want both verification rules named", report.VerificationNotes)
	}

	// The copy is really there, with content, modes and link targets matching the source.
	data, rerr := os.ReadFile(filepath.Join(destination, "hello.txt"))
	if rerr != nil || string(data) != "hello world" {
		t.Errorf("hello.txt = %q, %v", data, rerr)
	}
	info, ierr := os.Stat(filepath.Join(destination, "hello.txt"))
	if ierr != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("hello.txt mode = %v, %v; want 0640", info.Mode().Perm(), ierr)
	}
	target, lerr := os.Readlink(filepath.Join(destination, "link"))
	if lerr != nil || target != "hello.txt" {
		t.Errorf("link target = %q, %v; want hello.txt", target, lerr)
	}
	if target, lerr := os.Readlink(filepath.Join(destination, "linkdir")); lerr != nil || target != "sub" {
		t.Errorf("linkdir target = %q, %v; want sub", target, lerr)
	}
	if info, lerr := os.Lstat(filepath.Join(destination, "linkdir")); lerr != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("linkdir was followed rather than recreated: %v, %v", info, lerr)
	}

	if len(progress) == 0 {
		t.Fatal("progress callback was never called")
	}
	last := progress[len(progress)-1]
	if last.Total != plan.Stats.Files+plan.Stats.Directories+plan.Stats.Symlinks || last.Done != last.Total {
		t.Errorf("last progress = %+v, want Done == Total == %d", last, last.Total)
	}
	for _, p := range progress {
		if p.Path == "" {
			t.Errorf("progress path is empty: %+v", p)
		}
	}

	after := snapshotMigrationTree(t, source)
	assertSameSource(t, before, after)
}

func TestRunMigrationWithANilProgressCallback(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	plan, err := PlanMigration(source, filepath.Join(t.TempDir(), "dest"), MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	report, err := RunMigration(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunMigration with a nil callback: %v", err)
	}
	if !report.Verified {
		t.Errorf("Verified = false, Failures = %v", report.Failures)
	}
}

func TestRunMigrationHonoursExclusions(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	writeMigrationFile(t, filepath.Join(source, "node_modules", "a.js"), "a", 0o644)
	destination := filepath.Join(t.TempDir(), "dest")

	plan, err := PlanMigration(source, destination, MigrationOptions{
		Exclusions: []string{"node_modules"}, FreeSpace: ampleSpace,
	})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	report, err := RunMigration(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunMigration: %v", err)
	}
	if !report.Verified {
		t.Fatalf("Verified = false, Failures = %v", report.Failures)
	}
	if report.Copied.Files != 1 || report.Copied.Skipped != 1 {
		t.Errorf("Copied = %+v, want 1 file and 1 skipped", report.Copied)
	}
	if _, serr := os.Stat(filepath.Join(destination, "keep.txt")); serr != nil {
		t.Errorf("keep.txt missing from the copy: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(destination, "node_modules")); !os.IsNotExist(serr) {
		t.Errorf("node_modules was copied; stat error = %v", serr)
	}
}

func TestRunMigrationNeverOverwritesAnExistingDestinationEntry(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "keep.txt"), "from source", 0o644)
	destination := t.TempDir()
	writeMigrationFile(t, filepath.Join(destination, "keep.txt"), "original", 0o644)

	// A hand-built plan skips the preview, so this proves RunMigration itself refuses, not just that
	// PlanMigration warns.
	plan := MigrationPlan{Source: source, Destination: destination}
	report, err := RunMigration(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("RunMigration = nil error, want a refusal")
	}
	if !containsAny(report.Failures, "already holds something") {
		t.Errorf("Failures = %v, want the non-empty-destination wording", report.Failures)
	}
	data, rerr := os.ReadFile(filepath.Join(destination, "keep.txt"))
	if rerr != nil || string(data) != "original" {
		t.Errorf("destination entry = %q, %v; it must not have been overwritten", data, rerr)
	}
}

func TestRunMigrationStopsOnAReadErrorAndKeepsTheSourceAndThePartialCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permission bits, so a read error cannot be produced")
	}
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "aaa", 0o644)
	writeMigrationFile(t, filepath.Join(source, "b.txt"), "bbb", 0o644)
	destination := filepath.Join(t.TempDir(), "dest")

	plan, err := PlanMigration(source, destination, MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	// Planning read both files fine; the read error is introduced afterwards, as a real permission
	// change between the preview and the run would be.
	if cerr := os.Chmod(filepath.Join(source, "b.txt"), 0o000); cerr != nil {
		t.Fatal(cerr)
	}

	before := snapshotMigrationTree(t, source)
	report, err := RunMigration(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("RunMigration = nil error, want a read failure")
	}
	if !report.SourceUntouched {
		t.Error("SourceUntouched = false")
	}
	if !containsAny(report.Failures, "b.txt") {
		t.Errorf("Failures = %v, want one naming b.txt", report.Failures)
	}
	if !containsAny(report.Failures, "source was not touched") {
		t.Errorf("Failures = %v, want the source-untouched statement", report.Failures)
	}
	// The partial copy is evidence, not garbage: it must still be there.
	if _, serr := os.Stat(filepath.Join(destination, "a.txt")); serr != nil {
		t.Errorf("the partial copy was removed: %v", serr)
	}
	assertSameSource(t, before, snapshotMigrationTree(t, source))
}

func TestRunMigrationCancelledContextTouchesNothing(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	destination := filepath.Join(t.TempDir(), "dest")

	plan, err := PlanMigration(source, destination, MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	before := snapshotMigrationTree(t, source)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := RunMigration(ctx, plan, nil)
	if err == nil {
		t.Fatal("RunMigration = nil error, want the cancellation")
	}
	if !report.Cancelled {
		t.Error("Cancelled = false")
	}
	if !report.SourceUntouched {
		t.Error("SourceUntouched = false")
	}
	if _, serr := os.Stat(destination); !os.IsNotExist(serr) {
		t.Errorf("the destination was created despite the cancellation; stat error = %v", serr)
	}
	assertSameSource(t, before, snapshotMigrationTree(t, source))
}

func TestRunMigrationRefusesABlockedPlan(t *testing.T) {
	plan := MigrationPlan{Source: "/src", Destination: "/dst", Blockers: []string{"nope"}}
	report, err := RunMigration(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("RunMigration = nil error, want a refusal")
	}
	if !containsAny(report.Failures, "nope") {
		t.Errorf("Failures = %v, want the blocker carried through", report.Failures)
	}
}

func TestRunMigrationPreservesModesAndTimes(t *testing.T) {
	source := t.TempDir()
	fixed := time.Unix(1_600_000_000, 0)
	file := filepath.Join(source, "f.txt")
	writeMigrationFile(t, file, "content", 0o600)
	if err := os.Chtimes(file, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(source, "sub")
	writeMigrationFile(t, filepath.Join(sub, "x"), "x", 0o644)
	if err := os.Chtimes(sub, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "dest")

	plan, err := PlanMigration(source, destination, MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if _, err := RunMigration(context.Background(), plan, nil); err != nil {
		t.Fatalf("RunMigration: %v", err)
	}

	info, err := os.Stat(filepath.Join(destination, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %#o, want 0600", info.Mode().Perm())
	}
	if info.ModTime().Unix() != fixed.Unix() {
		t.Errorf("file mtime = %v, want %v", info.ModTime(), fixed)
	}
	dirInfo, err := os.Stat(filepath.Join(destination, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o755 {
		t.Errorf("directory mode = %#o, want 0755", dirInfo.Mode().Perm())
	}
	if dirInfo.ModTime().Unix() != fixed.Unix() {
		t.Errorf("directory mtime = %v, want %v", dirInfo.ModTime(), fixed)
	}
}

func TestRunMigrationInstructionsAreQuotedAndNotExecuted(t *testing.T) {
	// The path deliberately carries a space and a single quote, so the quoting is exercised by real
	// instructions rather than only by the helper's own test.
	source := filepath.Join(t.TempDir(), "my project's dir")
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	destination := filepath.Join(t.TempDir(), "dest with space")

	// The plan names the directories it copies in their canonical form, because that is what the run
	// walks and verifies. t.TempDir() returns a spelling rather than a canonical path, and the two
	// differ exactly where comparing them failed: macOS reaches its temporary directory through
	// /var -> /private/var. Resolving the expectation is what keeps this assertion about the
	// instruction instead of about the platform it runs on.
	wantSource := resolvedForTest(t, source)
	wantDestination := resolvedForTest(t, destination)

	plan, err := PlanMigration(source, destination, MigrationOptions{FreeSpace: ampleSpace})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if !containsAny(plan.Instructions, "cp -a", "rsync -a") {
		t.Errorf("Instructions = %v, want the cp/rsync equivalents", plan.Instructions)
	}
	if !containsAny(plan.Instructions, "rm -rf") {
		t.Errorf("Instructions = %v, want the by-hand removal", plan.Instructions)
	}
	if !containsAny(plan.Instructions, shellQuote(wantSource)) {
		t.Errorf("Instructions = %v, want the quoted source path %q", plan.Instructions, wantSource)
	}
	if !containsAny(plan.Instructions, shellQuote(wantDestination)) {
		t.Errorf("Instructions = %v, want the quoted destination path %q", plan.Instructions, wantDestination)
	}

	report, err := RunMigration(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunMigration: %v", err)
	}
	if !containsAny(report.Instructions, shellQuote(wantSource)) {
		t.Errorf("report Instructions = %v, want the quoted source path %q", report.Instructions, wantSource)
	}
	// The instruction text is data: after a successful copy the source is still there, because nothing
	// in this package ran the rm line.
	if _, serr := os.Stat(filepath.Join(source, "a.txt")); serr != nil {
		t.Errorf("the source was removed, so an instruction was executed: %v", serr)
	}
}

// resolvedForTest resolves the symlinks of the part of p that exists and keeps the part that does
// not, which is the form a plan names a path in. It uses the standard library rather than the
// package's own resolver on purpose: an expectation computed by the code under test would agree
// with that code instead of checking it.
func resolvedForTest(t *testing.T, p string) string {
	t.Helper()
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolvedForTest(t, parent), filepath.Base(p))
}

// ---------------------------------------------------------------------------
// Verification itself, exercised on a deliberately wrong destination
// ---------------------------------------------------------------------------

func TestVerifyMigrationNamesEveryDifference(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "same.txt"), "same", 0o644)
	writeMigrationFile(t, filepath.Join(source, "diff.txt"), "source", 0o644)
	writeMigrationFile(t, filepath.Join(source, "mode.txt"), "m", 0o600)
	writeMigrationFile(t, filepath.Join(source, "missing.txt"), "gone", 0o644)
	if err := os.MkdirAll(filepath.Join(source, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMigrationFile(t, filepath.Join(source, "sub", "child.txt"), "c", 0o644)
	if err := os.Symlink("same.txt", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	writeMigrationFile(t, filepath.Join(destination, "same.txt"), "same", 0o644)
	writeMigrationFile(t, filepath.Join(destination, "diff.txt"), "other", 0o644)
	writeMigrationFile(t, filepath.Join(destination, "mode.txt"), "m", 0o644)
	if err := os.MkdirAll(filepath.Join(destination, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeMigrationFile(t, filepath.Join(destination, "sub", "child.txt"), "c", 0o644)
	if err := os.Symlink("other.txt", filepath.Join(destination, "link")); err != nil {
		t.Fatal(err)
	}
	writeMigrationFile(t, filepath.Join(destination, "extra.txt"), "extra", 0o644)

	copied := buildCopiedMap(t, source)
	verified, verifiedBytes, sizeOnly, notes, failures := verifyMigration(source, destination, copied, 1<<20)

	if verified {
		t.Fatal("verified = true for a destination with differences")
	}
	if sizeOnly != 0 {
		t.Errorf("sizeOnly = %d, want 0", sizeOnly)
	}
	// same.txt, sub/child.txt and mode.txt matched byte-for-byte; mode.txt still counted as compared
	// bytes even though its permission bits differed, which is reported separately.
	if verifiedBytes != uint64(len("same")+len("c")+len("m")) {
		t.Errorf("verifiedBytes = %d, want %d", verifiedBytes, len("same")+len("c")+len("m"))
	}
	for _, want := range []string{"diff.txt", "mode.txt", "sub", "link", "extra.txt", "missing.txt"} {
		if !containsAny(failures, want) {
			t.Errorf("failures = %v, want one naming %s", failures, want)
		}
	}
	if !containsAny(notes, "byte-for-byte") || !containsAny(notes, "size and mode") {
		t.Errorf("notes = %v, want both verification rules named", notes)
	}
}

func TestVerifyMigrationReportsMissingExpectedEntries(t *testing.T) {
	source := t.TempDir()
	writeMigrationFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	destination := t.TempDir()

	verified, _, _, _, failures := verifyMigration(source, destination, buildCopiedMap(t, source), 1<<20)
	if verified {
		t.Fatal("verified = true for an empty copy")
	}
	if !containsAny(failures, "missing a.txt") {
		t.Errorf("failures = %v, want a missing a.txt", failures)
	}
}
