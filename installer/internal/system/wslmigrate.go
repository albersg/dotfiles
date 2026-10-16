package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// THE ASSISTED MIGRATION: PREVIEW FIRST, COPY AND VERIFY, NEVER DELETE
// ============================================================================
//
// The WSL performance advisor can tell a user that a project lives on the Windows host and that
// Linux tooling pays the boundary for every file it touches. The one action that actually helps is
// moving the project onto the distribution's own storage. That action is also the most dangerous
// thing this installer could do, so this file is written around a single rule:
//
//	Nothing in the source is ever moved, changed or deleted by this package.
//
// A migration here is a copy-and-verify, and the deletion of the original is a line of text the user
// runs by hand after they have looked at the copy. The package never executes a shell, so a `rm -rf`
// it prints cannot run unless a person types it. This is not politeness: an interrupted move, a
// cross-filesystem rename that silently becomes a copy-and-delete, and a watcher holding a file open
// are all ways a "move" loses data, and none of them are visible after the fact.
//
// The work is split in two, and the split is the safety property:
//
//   - PlanMigration is read-only. It walks the source, applies exclusions, classifies symlinks and
//     Git repositories, estimates the space the copy needs, checks that the destination is a sane
//     target and collects the reasons it is not. It reports every limitation as a Blocker or a
//     Warning so the caller can show a preview before anything is written.
//   - RunMigration copies and then verifies. It refuses a plan that is not Runnable, creates the
//     destination only when it is absent, proves the destination is writable by writing to it, and
//     never opens a source path for writing. A cancelled context or a write error stops the copy,
//     says what was copied and what failed, and leaves the partial destination in place rather than
//     deleting evidence the user may want.

// DefaultMigrationVerifyLimit is how many bytes of each regular file are compared byte-for-byte
// before a file is verified by size and mode alone. It bounds the verification so a multi-gigabyte
// tree does not become a second full read: 8 MiB per file is far past the size of a source file and
// the point where a silent bit-flip stops being a realistic concern.
const DefaultMigrationVerifyLimit int64 = 8 << 20

// MigrationOptions are the choices the caller makes before a plan exists.
type MigrationOptions struct {
	// Exclusions are glob patterns for entries to leave behind. A pattern without a separator matches
	// any single path component at any depth (so "node_modules" excludes every directory with that
	// name), and a pattern with a separator is matched against the slash-separated path relative to
	// the source with path.Match. Excluded entries are counted in the stats so the preview can show
	// what would be left behind.
	Exclusions []string
	// FreeSpace reads the free and total bytes behind a path. It is injected so a full disk can be
	// tested without filling one; nil means the real StorageSpace.
	FreeSpace func(path string) (free, total uint64, ok bool)
	// VerifyLimit is the per-file byte budget for content comparison. Zero or negative means
	// DefaultMigrationVerifyLimit.
	VerifyLimit int64
}

// MigrationStats is what a plan would copy, and (for a report) what a run did. The counts are of
// entries, not of paths visited: an excluded directory is one skipped entry and is not descended
// into, so the entries inside it are neither copied nor counted.
type MigrationStats struct {
	Files           int
	Directories     int
	Symlinks        int
	Hidden          int
	GitRepositories int
	Skipped         int
	// Bytes is the sum of the regular files' sizes. It is an estimate, not a reservation: it ignores
	// the block rounding a filesystem applies and it does not include the space a symlink or a
	// directory entry takes.
	Bytes uint64
	// SkippedBytes is the sum of the sizes of the excluded entries that were seen. A pruned directory
	// contributes its own size only, because pruning is exactly the promise not to look inside.
	SkippedBytes uint64
}

// Summary renders the stats as one readable line, e.g.
// "12 files (3.0 MiB) in 4 directories, 1 symlink, and a Git repository". Files and directories are
// always named so a line reads correctly even when both are zero; the rest appear only when they are
// worth mentioning.
func (s MigrationStats) Summary() string {
	parts := []string{fmt.Sprintf("%s (%s) in %s",
		migrationCount(s.Files, "file"),
		humanBytes(s.Bytes),
		migrationCount(s.Directories, "directory"),
	)}
	if s.Symlinks > 0 {
		parts = append(parts, migrationCount(s.Symlinks, "symlink"))
	}
	if s.Hidden > 0 {
		parts = append(parts, migrationCount(s.Hidden, "hidden entry")+" (included)")
	}
	if s.GitRepositories == 1 {
		parts = append(parts, "a Git repository")
	} else if s.GitRepositories > 1 {
		parts = append(parts, migrationCount(s.GitRepositories, "Git repository"))
	}
	if s.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%s skipped (%s)",
			migrationCount(s.Skipped, "entry"), humanBytes(s.SkippedBytes)))
	}
	return joinMigrationClauses(parts)
}

// MigrationPlan is the preview of a migration: what would be copied, whether the destination can
// take it, and why it cannot. A plan with a non-empty Blockers must not be run.
type MigrationPlan struct {
	Source      string
	Destination string
	// Exclusions is the pattern list the stats were computed with, kept so the preview can show it.
	Exclusions []string
	// Stats is what the copy would take, with the exclusions already applied.
	Stats MigrationStats
	// FreeBytes is the room behind the destination, meaningful only when SpaceKnown is true.
	FreeBytes uint64
	// SpaceKnown reports whether the free-space reading succeeded. It is deliberately separate from
	// FreeBytes so "0 bytes free" and "the question could not be asked" never read the same.
	SpaceKnown bool
	// Warnings are true but not fatal: they describe something the copy cannot carry or a check that
	// could not run. The preview shows them.
	Warnings []string
	// Blockers are the reasons the plan must not run. Any non-empty Blockers makes Runnable false.
	Blockers []string
	// Instructions are human-runnable lines, already shell-quoted, that show the equivalent cp/rsync
	// command and the by-hand removal of the source. This package never executes them.
	Instructions []string

	// verifyLimit is the per-file content-comparison budget RunMigration uses. It is unexported
	// because the export surface is fixed; PlanMigration fills it from MigrationOptions.VerifyLimit
	// and RunMigration falls back to DefaultMigrationVerifyLimit when a caller builds a plan by hand.
	verifyLimit int64
}

// Runnable reports whether the plan may be handed to RunMigration.
func (p MigrationPlan) Runnable() bool {
	return p.Source != "" && p.Destination != "" && len(p.Blockers) == 0
}

// MigrationProgress is one step of the copy: Done entries of Total have been placed, the most
// recent being Path (source-relative, slash-separated). Total is the sum of the plan's file,
// directory and symlink counts, so a preview and its progress bar agree.
type MigrationProgress struct {
	Done, Total int
	Path        string
}

// MigrationReport is what a run did. It is returned even when the run failed, so a screen can show
// the partial copy, the failures and the instructions without a second call.
type MigrationReport struct {
	Destination string
	// Copied is what the run placed, plus the entries the exclusions or an unsupported type left out.
	Copied MigrationStats
	// Verified is true only when every compared fact matched, no entry was missing or extra, and there
	// was no failure anywhere in the run.
	Verified bool
	// VerifiedBytes is the number of file bytes compared byte-for-byte.
	VerifiedBytes uint64
	// SizeOnlyFiles is how many files exceeded the verify limit and were covered by size and mode only.
	SizeOnlyFiles int
	// VerificationNotes states the rules used and how many files each covered.
	VerificationNotes []string
	// Failures names every problem seen, including every verification difference.
	Failures []string
	// Cancelled reports that the context stopped the run.
	Cancelled bool
	// SourceUntouched is always true: this package never writes to the source.
	SourceUntouched bool
	// Instructions repeats the by-hand commands and, after a failure, how to discard the partial copy.
	Instructions []string
}

// PlanMigration reads the source and builds a preview. It never writes anything and never follows a
// symlink. A returned error is a usage error only -- an empty source or destination, or a source
// that is not a directory -- so a screen can tell "you did not give me a source" from "the source
// has a file I cannot read".
func PlanMigration(source, destination string, opts MigrationOptions) (MigrationPlan, error) {
	if strings.TrimSpace(source) == "" {
		return MigrationPlan{}, errors.New("a source directory is required")
	}
	if strings.TrimSpace(destination) == "" {
		return MigrationPlan{}, errors.New("a destination directory is required")
	}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return MigrationPlan{}, fmt.Errorf("the source %q does not exist", source)
		}
		return MigrationPlan{}, fmt.Errorf("reading the source %q: %w", source, err)
	}
	if !info.IsDir() {
		return MigrationPlan{}, fmt.Errorf("the source %q is not a directory", source)
	}

	src := canonicalMigrationPath(source)
	dst := canonicalMigrationPath(destination)
	plan := MigrationPlan{
		Source:      src,
		Destination: dst,
		Exclusions:  append([]string(nil), opts.Exclusions...),
		verifyLimit: opts.VerifyLimit,
	}

	// Refuse layouts that cannot mean anything before doing any work: a migration onto itself, or a
	// destination nested in the source (which would copy the copy), or a source nested in the
	// destination (which would delete the ground it stands on if the source were ever removed).
	switch {
	case src == dst:
		plan.Blockers = append(plan.Blockers, "the source and the destination are the same path")
	case migrationPathInside(dst, src):
		plan.Blockers = append(plan.Blockers, "the destination is inside the source")
	case migrationPathInside(src, dst):
		plan.Blockers = append(plan.Blockers, "the source is inside the destination")
	}

	// An exclusion pattern that is not a valid glob would silently match nothing, so it is named.
	for _, pattern := range plan.Exclusions {
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, "x"); err != nil {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("The exclusion pattern %q is not a valid glob and was ignored.", pattern))
		}
	}

	// The destination is checked without creating it. A destination that holds something is refused
	// rather than merged into: the promise is that this tool never overwrites, and merging a tree
	// into a non-empty directory is how a partial plan becomes a partial loss.
	if dinfo, derr := os.Stat(dst); derr == nil {
		if !dinfo.IsDir() {
			plan.Blockers = append(plan.Blockers, "the destination exists and is not a directory")
		} else if empty, eerr := migrationDirEmpty(dst); eerr != nil {
			plan.Blockers = append(plan.Blockers, "the destination could not be read: "+eerr.Error())
		} else if !empty {
			plan.Blockers = append(plan.Blockers,
				"the destination already holds something; nothing was written")
		}
	} else if !os.IsNotExist(derr) {
		plan.Blockers = append(plan.Blockers, "the destination could not be checked: "+derr.Error())
	}

	stats, walkWarnings, walkBlockers := scanMigrationSource(src, plan.Exclusions)
	plan.Stats = stats
	plan.Warnings = append(plan.Warnings, walkWarnings...)
	plan.Blockers = append(plan.Blockers, walkBlockers...)

	// Space is measured behind the destination, or behind its nearest existing parent when the
	// destination does not exist yet. An unreadable reading is a Warning, never a Blocker: a host that
	// cannot answer the question must not be reported as having no room.
	freeSpace := opts.FreeSpace
	if freeSpace == nil {
		freeSpace = StorageSpace
	}
	if _, derr := os.Stat(dst); derr != nil {
		plan.FreeBytes, _, plan.SpaceKnown = freeSpace(nearestExistingAncestor(dst))
	} else {
		plan.FreeBytes, _, plan.SpaceKnown = freeSpace(dst)
	}
	if !plan.SpaceKnown {
		plan.Warnings = append(plan.Warnings,
			"Free space at the destination could not be measured; check it before copying.")
	} else if plan.FreeBytes < plan.Stats.Bytes {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf(
			"the destination has %s free but the copy needs about %s",
			humanBytes(plan.FreeBytes), humanBytes(plan.Stats.Bytes)))
	}

	plan.Warnings = append(plan.Warnings,
		"File ownership is not copied: the copies will belong to the account that runs the migration.")
	plan.Instructions = migrationInstructions(src, dst, plan.Exclusions)
	return plan, nil
}

// scanMigrationSource walks the source and returns the stats plus the warnings that only the walk
// can produce (unreadable entries, symlinks that will dangle, Git repositories carrying absolute
// paths). It is read-only and never returns a fatal error: everything it finds is a Blocker inside
// the stats-bearing caller's plan or a Warning it hands back.
func scanMigrationSource(source string, exclusions []string) (MigrationStats, []string, []string) {
	stats := MigrationStats{}
	var warnings []string
	var blockers []string

	walkErr := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			blockers = append(blockers, "the source entry "+p+" could not be read: "+err.Error())
			return nil
		}
		if p == source {
			return nil
		}
		rel, rerr := filepath.Rel(source, p)
		if rerr != nil {
			blockers = append(blockers, "the source path "+p+" could not be described: "+rerr.Error())
			return nil
		}
		rel = filepath.ToSlash(rel)

		if migrationExcluded(rel, exclusions) {
			stats.Skipped++
			if fi, ierr := d.Info(); ierr == nil {
				stats.SkippedBytes += uint64(fi.Size())
			}
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if migrationHidden(d.Name()) {
			stats.Hidden++
		}
		if d.Name() == ".git" {
			stats.GitRepositories++
			warnings = append(warnings, migrationGitWarning(rel))
		}

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			stats.Symlinks++
			target, lerr := os.Readlink(p)
			if lerr != nil {
				blockers = append(blockers, "the symlink "+p+" could not be read: "+lerr.Error())
				return nil
			}
			if warning := migrationSymlinkWarning(source, p, target); warning != "" {
				warnings = append(warnings, warning)
			}
		case d.IsDir():
			stats.Directories++
		case d.Type().IsRegular():
			stats.Files++
			fi, ierr := d.Info()
			if ierr != nil {
				blockers = append(blockers, "the source file "+p+" could not be read: "+ierr.Error())
				return nil
			}
			stats.Bytes += uint64(fi.Size())
			// A file that cannot be opened is a Blocker now rather than a surprise halfway through the
			// copy. Opening proves read permission; reading one byte proves the file is not the kind of
			// special path that opens but never yields data.
			if rerr := migrationReadable(p); rerr != nil {
				blockers = append(blockers, "the source file "+p+" could not be read: "+rerr.Error())
			}
		default:
			// A device, socket or FIFO is not a project file, is not copyable as data, and copying it
			// blindly would block or fail. It is counted as skipped so the preview is not silent about it.
			stats.Skipped++
			if fi, ierr := d.Info(); ierr == nil {
				stats.SkippedBytes += uint64(fi.Size())
			}
			warnings = append(warnings, "The entry "+rel+" is not a file, directory or symlink and will not be copied.")
		}
		return nil
	})
	if walkErr != nil {
		// WalkDir only fails on the root it was given; the callback swallows everything else. Both are
		// limitations to show, not usage errors to return.
		blockers = append(blockers, "the source tree could not be walked: "+walkErr.Error())
	}
	return stats, warnings, blockers
}

// RunMigration copies the plan and then verifies the copy. It returns a report even when it fails,
// so the caller can show what was copied and what went wrong. SourceUntouched is always true: this
// function only ever opens a source path for reading.
//
// The returned error is non-nil when the plan could not be run (a Blocker, a missing source, an
// unwritable destination), when a copy failed, or when the context was cancelled. A completed copy
// whose verification found differences returns a nil error with Verified false and Failures filled:
// the migration ran, and the report is what says it did not verify.
func RunMigration(ctx context.Context, plan MigrationPlan, progress func(MigrationProgress)) (MigrationReport, error) {
	report := MigrationReport{
		Destination:     plan.Destination,
		SourceUntouched: true,
		Instructions:    append([]string(nil), plan.Instructions...),
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if !plan.Runnable() {
		report.Failures = append(report.Failures, plan.Blockers...)
		if len(report.Failures) == 0 {
			report.Failures = append(report.Failures, "the migration plan is not runnable")
		}
		return report, errors.New("the migration plan is not runnable")
	}
	if err := ctx.Err(); err != nil {
		report.Cancelled = true
		report.Failures = append(report.Failures, "the migration was cancelled before it started")
		return report, err
	}

	info, err := os.Stat(plan.Source)
	if err != nil {
		report.Failures = append(report.Failures, "the source could not be read: "+err.Error())
		return report, err
	}
	if !info.IsDir() {
		report.Failures = append(report.Failures, "the source is no longer a directory")
		return report, errors.New("the source is not a directory")
	}

	if err := migrationPrepareDestination(plan.Destination); err != nil {
		report.Failures = append(report.Failures, err.Error())
		return report, err
	}
	// Writability is proven by writing before anything else is copied, so a read-only destination is
	// a failure with nothing copied rather than a failure after half a tree.
	if err := migrationProbeWritable(plan.Destination); err != nil {
		report.Failures = append(report.Failures, "the destination is not writable: "+err.Error())
		return report, err
	}

	limit := plan.verifyLimit
	if limit <= 0 {
		limit = DefaultMigrationVerifyLimit
	}

	copied := map[string]copiedEntry{}
	type dirStamp struct {
		path string
		when time.Time
	}
	var dirs []dirStamp
	total := plan.Stats.Files + plan.Stats.Directories + plan.Stats.Symlinks
	done := 0

	walkErr := filepath.WalkDir(plan.Source, func(p string, d fs.DirEntry, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			report.Cancelled = true
			return cerr
		}
		if err != nil {
			return err
		}
		if p == plan.Source {
			return nil
		}
		rel, rerr := filepath.Rel(plan.Source, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if migrationExcluded(rel, plan.Exclusions) {
			report.Copied.Skipped++
			if fi, ierr := d.Info(); ierr == nil {
				report.Copied.SkippedBytes += uint64(fi.Size())
			}
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(plan.Destination, filepath.FromSlash(rel))
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			if serr := os.Symlink(target, dstPath); serr != nil {
				return serr
			}
			report.Copied.Symlinks++
			copied[rel] = copiedEntry{kind: kindSymlink, linkTarget: target}
		case d.IsDir():
			if merr := os.Mkdir(dstPath, fi.Mode().Perm()); merr != nil {
				return merr
			}
			if cerr := os.Chmod(dstPath, fi.Mode().Perm()); cerr != nil {
				return cerr
			}
			dirs = append(dirs, dirStamp{path: dstPath, when: fi.ModTime()})
			report.Copied.Directories++
			copied[rel] = copiedEntry{kind: kindDir, perm: fi.Mode().Perm()}
		case d.Type().IsRegular():
			if cerr := migrationCopyFile(p, dstPath, fi); cerr != nil {
				return cerr
			}
			report.Copied.Files++
			report.Copied.Bytes += uint64(fi.Size())
			copied[rel] = copiedEntry{kind: kindFile, size: fi.Size(), perm: fi.Mode().Perm()}
		default:
			// Same decision as the plan: a non-data entry is left out and named, not copied blindly.
			report.Copied.Skipped++
			report.VerificationNotes = append(report.VerificationNotes,
				"Skipped an entry this tool does not copy: "+rel)
			return nil
		}

		if migrationHidden(d.Name()) {
			report.Copied.Hidden++
		}
		if d.Name() == ".git" {
			report.Copied.GitRepositories++
		}
		done++
		if progress != nil {
			progress(MigrationProgress{Done: done, Total: total, Path: rel})
		}
		return nil
	})

	// Directory modification times are restored deepest first: creating a child updates its parent's
	// mtime, so the parent has to be stamped after the children exist.
	for i := len(dirs) - 1; i >= 0; i-- {
		if cerr := os.Chtimes(dirs[i].path, dirs[i].when, dirs[i].when); cerr != nil {
			report.Failures = append(report.Failures,
				"the modification time of the copy at "+dirs[i].path+" could not be restored: "+cerr.Error())
		}
	}

	if report.Cancelled {
		report.Failures = append(report.Failures,
			"the migration was cancelled; a partial copy may remain at the destination and the source was not touched")
		return report, ctx.Err()
	}
	if walkErr != nil {
		report.Failures = append(report.Failures,
			"the copy stopped: "+walkErr.Error()+
				"; a partial copy may remain at the destination and the source was not touched")
		report.Instructions = append(report.Instructions,
			"# If you want to discard the partial copy, remove it by hand:",
			"rm -rf "+shellQuote(plan.Destination))
		return report, walkErr
	}

	verified, verifiedBytes, sizeOnly, notes, failures := verifyMigration(plan.Source, plan.Destination, copied, limit)
	report.VerifiedBytes = verifiedBytes
	report.SizeOnlyFiles = sizeOnly
	report.VerificationNotes = append(report.VerificationNotes, notes...)
	report.Failures = append(report.Failures, failures...)
	report.Verified = verified && len(report.Failures) == 0
	return report, nil
}

// ============================================================================
// THE COPY PRIMITIVES
// ============================================================================

// migrationPrepareDestination creates the destination when it is absent and refuses when it is not
// an empty directory. It is the second half of "never overwrite": PlanMigration does not create, and
// this refuses anything the plan did not bless, so a hand-built plan cannot merge into a directory
// that already holds something.
func migrationPrepareDestination(destination string) error {
	info, err := os.Stat(destination)
	switch {
	case err == nil:
		if !info.IsDir() {
			return errors.New("the destination exists and is not a directory; nothing was written")
		}
		empty, eerr := migrationDirEmpty(destination)
		if eerr != nil {
			return errors.New("the destination could not be read: " + eerr.Error())
		}
		if !empty {
			return errors.New("the destination already holds something; nothing was written")
		}
		return nil
	case os.IsNotExist(err):
		// Created only now, and only because it was absent.
		if merr := os.MkdirAll(destination, 0o755); merr != nil {
			return errors.New("the destination could not be created: " + merr.Error())
		}
		return nil
	default:
		return errors.New("the destination could not be checked: " + err.Error())
	}
}

// migrationProbeWritable proves the destination accepts a write before the copy starts. A read-only
// destination then fails with an empty copy instead of failing on its first file.
func migrationProbeWritable(destination string) error {
	f, err := os.CreateTemp(destination, ".dotfiles-migration-probe-")
	if err != nil {
		return err
	}
	name := f.Name()
	_, writeErr := f.Write([]byte("probe"))
	closeErr := f.Close()
	removeErr := os.Remove(name)
	switch {
	case writeErr != nil:
		return writeErr
	case closeErr != nil:
		return closeErr
	default:
		return removeErr
	}
}

// migrationCopyFile copies one regular file's bytes and then restores the permission bits and the
// modification time. O_EXCL is what makes "never overwrite" true at the file level: an existing
// destination entry is an error, not a truncation.
func migrationCopyFile(source, destination string, info fs.FileInfo) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	// Mkdir/Create honour the umask, so the mode is set explicitly to preserve the source's bits.
	if err := os.Chmod(destination, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(destination, info.ModTime(), info.ModTime())
}

// migrationReadable proves a source file can be opened and yields data. It is used by the plan so an
// unreadable file is a Blocker in the preview rather than an error halfway through a copy.
func migrationReadable(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// migrationDirEmpty reports whether a directory has no entries.
func migrationDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(names) == 0, nil
}

// ============================================================================
// VERIFICATION: THE COPY IS COMPARED, NOT TRUSTED
// ============================================================================

// migrationKind is the type of a copied entry. The three kinds are the three things WalkDir can
// hand back that this tool copies; anything else is skipped and never enters the copied set.
type migrationKind int

const (
	kindFile migrationKind = iota
	kindDir
	kindSymlink
)

// copiedEntry is one fact about a placed entry that verification can compare. It is deliberately not
// a path: the path is the map key, so the set itself is compared too.
type copiedEntry struct {
	kind       migrationKind
	size       int64
	perm       fs.FileMode
	linkTarget string
}

// verifyMigration re-walks the destination and compares it against the set of entries the copy
// placed. It proves five things per entry -- the path set is exactly the copied set, the type, the
// size, the permission bits and a symlink's target -- and compares the content of every regular file
// byte-for-byte up to limit. A larger file is verified by size and mode only, and the count of those
// is returned so the notes can say which rule covered how many files.
//
// It returns failures sorted by path so a report reads the same way twice, and Verified is true only
// when the set matched completely with no difference anywhere.
func verifyMigration(source, destination string, copied map[string]copiedEntry, limit int64) (verified bool, verifiedBytes uint64, sizeOnly int, notes, failures []string) {
	contentFiles := 0
	seen := map[string]bool{}

	_ = filepath.WalkDir(destination, func(p string, d fs.DirEntry, err error) error {
		if p == destination {
			return nil
		}
		rel, rerr := filepath.Rel(destination, p)
		if rerr != nil {
			failures = append(failures, "a destination path could not be described: "+rerr.Error())
			return nil
		}
		rel = filepath.ToSlash(rel)

		if err != nil {
			failures = append(failures, "the copy at "+rel+" could not be read: "+err.Error())
			return nil
		}
		expected, ok := copied[rel]
		if !ok {
			failures = append(failures, "the destination holds an entry the copy did not place there: "+rel)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		seen[rel] = true

		info, ierr := d.Info()
		if ierr != nil {
			failures = append(failures, "the copy at "+rel+" could not be described: "+ierr.Error())
			return nil
		}

		switch expected.kind {
		case kindDir:
			if !d.IsDir() {
				failures = append(failures, rel+" is not a directory at the destination")
				return nil
			}
			if info.Mode().Perm() != expected.perm {
				failures = append(failures, fmt.Sprintf("%s has mode %#o at the destination, want %#o",
					rel, info.Mode().Perm(), expected.perm))
			}
		case kindSymlink:
			if d.Type()&fs.ModeSymlink == 0 {
				failures = append(failures, rel+" is not a symlink at the destination")
				return nil
			}
			target, lerr := os.Readlink(p)
			if lerr != nil {
				failures = append(failures, "the symlink "+rel+" could not be read: "+lerr.Error())
			} else if target != expected.linkTarget {
				failures = append(failures, fmt.Sprintf("%s points at %q at the destination, want %q",
					rel, target, expected.linkTarget))
			}
		case kindFile:
			if !d.Type().IsRegular() {
				failures = append(failures, rel+" is not a regular file at the destination")
				return nil
			}
			if info.Size() != expected.size {
				failures = append(failures, fmt.Sprintf("%s is %d bytes at the destination, want %d",
					rel, info.Size(), expected.size))
			}
			if info.Mode().Perm() != expected.perm {
				failures = append(failures, fmt.Sprintf("%s has mode %#o at the destination, want %#o",
					rel, info.Mode().Perm(), expected.perm))
			}
			// Content is compared only when the file is small enough to be read cheaply; a larger one
			// is covered by the size and mode checks above and counted as such.
			if expected.size <= limit {
				same, cerr := migrationSameContent(filepath.Join(source, filepath.FromSlash(rel)), p, limit)
				switch {
				case cerr != nil:
					failures = append(failures, "the copy at "+rel+" could not be compared: "+cerr.Error())
				case !same:
					failures = append(failures, rel+" differs from the source byte-for-byte")
				default:
					contentFiles++
					verifiedBytes += uint64(info.Size())
				}
			} else {
				sizeOnly++
			}
		}
		return nil
	})

	// The comparison runs both ways: an expected entry that the destination walk never saw is a
	// missing copy, and WalkDir on the destination cannot report it by itself.
	for rel := range copied {
		if !seen[rel] {
			failures = append(failures, "the copy is missing "+rel)
		}
	}
	sort.Strings(failures)

	notes = []string{
		"Compared the path set, entry types, permission bits and symlink targets of the copy against the source.",
		fmt.Sprintf("Verified %d file(s) byte-for-byte up to %s.", contentFiles, humanBytes(uint64(limit))),
		fmt.Sprintf("Verified %d file(s) larger than %s by size and mode only.", sizeOnly, humanBytes(uint64(limit))),
	}
	return len(failures) == 0, verifiedBytes, sizeOnly, notes, failures
}

// migrationSameContent compares two files byte-for-byte up to limit. The caller has already checked
// that the source is no larger than the limit, so the limit is an upper bound, not a truncation.
func migrationSameContent(source, destination string, limit int64) (bool, error) {
	want, err := readMigrationPrefix(source, limit)
	if err != nil {
		return false, err
	}
	got, err := readMigrationPrefix(destination, limit)
	if err != nil {
		return false, err
	}
	return bytes.Equal(want, got), nil
}

// readMigrationPrefix reads at most limit bytes from a path.
func readMigrationPrefix(p string, limit int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// ============================================================================
// EXCLUSIONS, SYMLINKS AND GIT
// ============================================================================

// migrationExcluded reports whether a source-relative slash path matches any exclusion pattern. A
// pattern with a separator is matched against the whole relative path; a pattern without one is
// matched against each component, which is what makes "node_modules" exclude the directory wherever
// it appears rather than only at the top level. A bad pattern matches nothing (the plan warns).
func migrationExcluded(rel string, patterns []string) bool {
	if rel == "" || rel == "." {
		return false
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if strings.Contains(pattern, "/") {
			if ok, err := path.Match(pattern, rel); err == nil && ok {
				return true
			}
			continue
		}
		for _, component := range strings.Split(rel, "/") {
			if ok, err := path.Match(pattern, component); err == nil && ok {
				return true
			}
		}
	}
	return false
}

// migrationHidden reports whether a name is a dot entry. Hidden entries are copied like everything
// else; this only exists so the preview can say they are included.
func migrationHidden(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// migrationSymlinkWarning describes the one thing a copied symlink cannot carry: a target that names
// the old location or that is not part of the copy. A relative symlink that resolves inside the
// source is left alone, because after the move it still resolves inside the source.
func migrationSymlinkWarning(source, linkPath, target string) string {
	var resolved string
	if filepath.IsAbs(target) {
		resolved = filepath.Clean(target)
	} else {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	}
	inside := resolved == source || migrationPathInside(resolved, source)
	switch {
	case filepath.IsAbs(target) && inside:
		return "The symlink " + linkPath + " points at " + target +
			" inside the source with an absolute path; after the move it will still name the old location."
	case !inside:
		return "The symlink " + linkPath + " points outside the source (at " + target +
			"); its target is not copied and the link will dangle."
	default:
		return ""
	}
}

// migrationGitWarning is the one warning every repository gets: the repository is copied whole, but
// its own configuration may name absolute paths, and checking that is the user's step, never ours.
func migrationGitWarning(rel string) string {
	return "The Git repository at " + rel + " is copied whole, but absolute paths recorded in " +
		".git/config or in a linked worktree's .git file may still name the old location; checking that " +
		"is your step, never this tool's."
}

// ============================================================================
// INSTRUCTIONS: TEXT A PERSON RUNS, NEVER TEXT THIS PACKAGE RUNS
// ============================================================================

// shellQuote renders s as one single-quoted POSIX shell word. A single quote inside the value is
// closed, escaped and reopened, so a path with a space, a quote or a dollar sign survives as data
// instead of becoming shell syntax. It is used only for instructions shown to a person; nothing here
// executes these strings.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// migrationInstructions is the by-hand equivalent of the copy, plus the reminder that removing the
// source is a human decision made after checking the copy. The paths are quoted so a space or a quote
// in a directory name cannot turn the text into a different command.
func migrationInstructions(source, destination string, exclusions []string) []string {
	lines := []string{
		"# Copy the project by hand, keeping permissions and times:",
		"cp -a " + shellQuote(source+"/.") + " " + shellQuote(destination+"/"),
	}
	if len(exclusions) > 0 {
		lines = append(lines, "# cp cannot exclude entries; use the rsync line below to leave them behind.")
	}
	rsync := "rsync -a --info=progress2"
	for _, pattern := range exclusions {
		if pattern != "" {
			rsync += " --exclude " + shellQuote(pattern)
		}
	}
	lines = append(lines,
		"# Or with rsync, which shows progress and applies the exclusions:",
		rsync+" "+shellQuote(source+"/")+" "+shellQuote(destination+"/"),
		"# Compare before deleting anything:",
		"diff -r "+shellQuote(source)+" "+shellQuote(destination),
		"# Only after you have checked the copy, remove the original by hand:",
		"rm -rf "+shellQuote(source),
	)
	return lines
}

// ============================================================================
// SMALL SHARED HELPERS
// ============================================================================

// migrationPathInside reports whether child lives under parent. The comparison is on path
// boundaries, so "/a/bc" is not inside "/a/b"; an identical pair is not "inside" either, so the
// caller can test equality separately.
func migrationPathInside(child, parent string) bool {
	if child == parent {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// canonicalMigrationPath resolves a path to an absolute, symlink-free form when it exists, so two
// spellings of one directory compare equal and the layout checks cannot be fooled by a symlink. A
// path that does not exist yet (the usual destination) falls back to a cleaned absolute path.
func canonicalMigrationPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// nearestExistingAncestor walks up until it finds a path that exists, so free space can be measured
// behind a destination that has not been created yet. It stops at the filesystem root.
func nearestExistingAncestor(p string) string {
	current := filepath.Clean(p)
	for {
		if _, err := os.Stat(current); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// humanBytes renders a byte count for a person: whole bytes below a kibibyte, one decimal above it.
// The unit names are binary because the numbers are: the filesystem counts in 1024s and calling them
// "MB" would overstate the free space by seven percent.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	value := float64(n)
	index := -1
	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}

// migrationCount renders "1 file" or "3 files".
func migrationCount(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, migrationPlural(singular))
}

// migrationPlural spells the plural of the small fixed vocabulary the summary uses.
func migrationPlural(singular string) string {
	switch singular {
	case "directory":
		return "directories"
	case "entry":
		return "entries"
	case "hidden entry":
		return "hidden entries"
	case "Git repository":
		return "Git repositories"
	default:
		return singular + "s"
	}
}

// joinMigrationClauses joins summary clauses with commas and an Oxford "and".
func joinMigrationClauses(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}
