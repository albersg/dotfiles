package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// RevertStatus names what a revert did, or would do, to one file. "Already" and
// "unrevertible" are separate answers on purpose: the first is a machine where
// nothing is left to do, the second is a file nothing can put back, and a person
// who is told "the revert is done" must never be in the second case.
// RevertStatus is one of the answers a revert gives about a file. The constants
// are exported because the command line prints them: the report is a contract
// between the package that decides and the program that shows it.
type RevertStatus string

const (
	// RevertReverted: the file now holds what it held before the run, checked
	// against the digest the record took.
	RevertReverted RevertStatus = "reverted"
	// RevertWouldRevert: a dry run, which names the plan without touching it.
	RevertWouldRevert RevertStatus = "would-revert"
	// RevertAlready: the file is already in the state the revert restores, so
	// running the command twice is not a second change.
	RevertAlready RevertStatus = "already"
	// RevertUnrevertible: nothing on this machine can put this file back.
	RevertUnrevertible RevertStatus = "unrevertible"
	// RevertFailed: the revert tried and the file does not hold what it should.
	RevertFailed RevertStatus = "failed"
)

// RevertOptions is what the caller decides about a revert.
type RevertOptions struct {
	// DryRun names the plan and touches nothing.
	DryRun bool
}

// RevertFileReport is what the revert did to one file.
type RevertFileReport struct {
	Path   string       `json:"path"`
	Key    string       `json:"key,omitempty"`
	Action fileAction   `json:"action"`
	Status RevertStatus `json:"status"`
	Reason string       `json:"reason,omitempty"`
}

// RevertReport is the account a revert gives of itself: what it put back, what
// was already in place, what it could not put back and why, where the state it
// replaced was kept, and what no command on this machine can undo.
type RevertReport struct {
	Record         *lastInstall       `json:"-"`
	Files          []RevertFileReport `json:"files"`
	SnapshotDir    string             `json:"snapshotDir,omitempty"`
	SnapshotReason string             `json:"snapshotReason,omitempty"`
	Irreversible   []string           `json:"irreversible"`
	Planned        int                `json:"planned"`
	Reverted       int                `json:"reverted"`
	AlreadyDone    int                `json:"already"`
	Unrevertible   int                `json:"unrevertible"`
	Failed         int                `json:"failed"`
	DryRun         bool               `json:"dryRun"`
}

// Complete reports whether the machine is back where the run found it, as far as
// the record can tell.
func (r RevertReport) Complete() bool {
	return r.Unrevertible == 0 && r.Failed == 0
}

// RevertLastRun puts the machine back where the last run found it, file by file,
// from the record that run wrote: a file it modified or removed is copied back
// out of the backup, and a file it created is removed.
//
// The rules that make it safe to run on a machine someone is using:
//
//   - It works file by file from the record, so a run that stopped at a failed
//     step is as revertible as one that finished: the record names what it had
//     already changed when it stopped.
//   - It proves itself per file. Every file it puts back is compared against the
//     digest the run recorded before it touched anything, so "the revert is done"
//     is a measurement and not an attempt.
//   - It keeps what it replaces. The configuration paths it is about to write are
//     copied into a backup of their own first, so a revert can itself be undone.
//   - It is idempotent. A file already in the state the revert restores is
//     reported as such rather than written again, so the command can be run twice
//     without a second change to the machine.
//   - It says what it cannot do. A file the backup holds no copy of is reported
//     as unrevertible instead of being skipped, and the effects the scope decision
//     left out of reversal are named rather than omitted.
//
// A machine with no record has nothing to revert, and that is the answer: the
// error is errNoRecord, which the command line reports without a traceback.
// firstUnaccountedFor returns the path inside a claim the record cannot put back --
// the claim's own path, or something inside it -- or "" when the record accounts for
// the whole of it.
func firstUnaccountedFor(record *lastInstall, entry installEntry) string {
	if entry.Path == "" {
		return ""
	}
	// Stat, not Lstat: a key that is a link to a directory is a tree this run writes
	// through, and the record now names what it wrote inside. Reading it as a file is
	// how a revert that put every change back came to report the key as one it could
	// not act on.
	info, err := os.Stat(entry.Path)
	if err != nil {
		// Gone from the machine, or a link to nothing. A path the run created and that
		// is not there is where the machine was before the run; anything else is covered
		// by the files the run removed, which the record lists.
		if entry.Action == installActionCreated || namesPathUnder(record.Changes, entry.Path) {
			return ""
		}
		return entry.Path
	}
	if !info.IsDir() {
		for _, change := range record.Changes {
			if change.Path == entry.Path {
				return "" // the change names this very file and carries its before-state
			}
		}
		// A file: the index holds every regular file and reads links through, so a path
		// the record says the run wrote with no change naming it is a claim the record
		// cannot back.
		return entry.Path
	}
	if !namesPathUnder(record.Changes, entry.Path) {
		return entry.Path
	}
	return firstUnaccountedInside(record, entry)
}

// firstUnaccountedInside walks a directory the record says the run wrote and returns
// the first path inside it that the record cannot vouch for: a kind the per-file
// index steps over -- a symlink, a socket, a device -- that no change names and that
// the run's own copy does not match.
func firstUnaccountedInside(record *lastInstall, entry installEntry) string {
	// A walk that starts at a link visits the link and nothing else, so a key that is
	// one would look empty. The tree is walked where it really is, and every path in
	// it is named as a person sees it, under the key.
	walkRoot := entry.Path
	if resolved, err := filepath.EvalSymlinks(entry.Path); err == nil {
		walkRoot = resolved
	}

	unaccounted := ""
	_ = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || unaccounted != "" {
			return nil
		}
		if d.IsDir() || d.Type().IsRegular() {
			return nil
		}
		named := namedPath(entry.Path, walkRoot, path)
		for _, change := range record.Changes {
			if change.Path == named || change.Path == path {
				return nil
			}
		}
		if unaccountedInside(record, entry, named) != "" {
			unaccounted = named
		}
		return nil
	})
	return unaccounted
}

// unaccountedInside explains why a path inside a key the record says the run wrote is
// one the record cannot vouch for, or returns "" when it can. Only the kinds the
// per-file record does not hold reach here: a change naming the path settles it first.
func unaccountedInside(record *lastInstall, entry installEntry, path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return path + ", which the record lists no change for and which cannot be read now"
	}
	if info.Mode()&os.ModeSymlink == 0 {
		if aKindTheRunNeverWrites(info.Mode()) {
			// A named pipe, a socket or a device: the installer writes files and links and
			// nothing else, and the copier that takes its backups omits these kinds, so no
			// copy of a key can hold one and the record's silence about it is not evidence
			// of anything. Reporting it would fail a revert whose every change was put back
			// over a path the run never wrote, and over one the reader of the report cannot
			// act on either.
			return ""
		}
		if sameKindAsCopy(entry, path) {
			return "" // the copy holds this kind at this path: the run found it, it did not make it
		}
		return path + ", which the record lists no change for and which the copy of the key does not hold"
	}

	// A link. Against the copy the run took, the target it holds says whether the link
	// itself moved; where that target resolves to says whether what it reaches is
	// something this record can speak for at all. The path is named in every answer,
	// because a report that says only which key failed leaves the reader looking.
	copied := copiedFor(entry, path)
	copiedTarget, target := linkTarget(copied), linkTarget(path)
	if copiedTarget != "" && copiedTarget != target {
		return path + ", which the run's copy points at " + copiedTarget + " and the machine points at " + target + ", and which the record lists no change for"
	}

	resolved, resolveErr := filepath.EvalSymlinks(path)
	if resolveErr != nil {
		// A link to nothing reaches nothing. Only a copy can say a link was already here,
		// and this installer never makes one: failing the revert for want of that copy
		// turns every dead link a person already had into "could not be put back" for a
		// run whose every change was put back.
		return ""
	}
	covering, written := coveringEntry(record, resolved)
	if covering == nil {
		return path + ", which is a link into " + resolved + " -- outside every path this record names, so nothing here can say what the run left there"
	}
	if !written {
		// A key the record says the run did not write is a key it left alone, and that
		// is the record's own word. Reporting it would be an alarm about a place this
		// run never reached.
		return ""
	}
	if copied == "" {
		// The copy is the record's only word on the files behind a link. With no copy of the
		// key there is nothing to compare them against, and the files reached this way are
		// judged as the same files are when the key is walked directly: by the changes the
		// record carries. Demanding the copy that is not there would report paths inside a
		// tree the run wrote, depending on which route the reader took to them.
		return ""
	}
	if why := unvouchedInside(record, *covering, resolved); why != "" {
		return path + ", which is a link into " + resolved + ", and " + why
	}
	return ""
}

// coveringEntry returns the entry whose path holds a resolved path, and whether that
// entry says the run wrote its key.
func coveringEntry(record *lastInstall, resolved string) (*installEntry, bool) {
	for i := range record.Entries {
		entry := &record.Entries[i]
		if entry.Path == "" {
			continue
		}
		// A key that is a link is the same tree under two names: the path a person
		// sees and the path the walk visits.
		if !underPath(resolved, entry.Path) {
			if keyRoot, err := filepath.EvalSymlinks(entry.Path); err != nil || !underPath(resolved, keyRoot) {
				continue
			}
		}
		return entry, entry.Action == installActionReplaced || entry.Action == installActionCreated
	}
	return nil, false
}

// underPath reports whether path is the given root or lives inside it.
func underPath(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(root, string(os.PathSeparator))+string(os.PathSeparator))
}

// unvouchedInside returns the first path inside a tree the record says the run wrote
// that neither a change names nor the run's own copy matches, or "" when the tree is
// vouched for. The copy is the before-state, so a file that agrees with it is a file
// the run left alone -- and the record's silence about a file that does NOT agree is
// not evidence of anything. That is the difference between trusting a claim and
// checking it, and it is what a record that names a tree while omitting a file inside
// it is caught by.
func unvouchedInside(record *lastInstall, entry installEntry, root string) string {
	if entry.Backup == "" {
		return "the record names no copy of it to check against"
	}
	// The walk below visits the paths behind a link, and the copy the run took is filed
	// under the names a person sees: the two have to be reconciled before either can be
	// compared, or the check looks for a copy of the wrong name and reports a file that
	// is where the run found it.
	namedRoot := root
	if keyRoot, err := filepath.EvalSymlinks(entry.Path); err == nil && underPath(root, keyRoot) {
		namedRoot = namedPath(entry.Path, keyRoot, root)
	}

	unvouched := ""
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || unvouched != "" || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		named := namedRoot
		if relErr == nil && rel != "." {
			named = filepath.Join(namedRoot, rel)
		}
		for _, change := range record.Changes {
			if change.Path == named || change.Path == p {
				return nil
			}
		}
		// Only a name that reads as a regular file has content to compare, and a named pipe
		// blocks the reader until a writer appears. A pipe, a socket or a device -- named
		// here directly or reached through a link -- is not the run's doing, and this check
		// has nothing to say about it.
		if !readsAsAFile(p) {
			return nil
		}
		copied := copiedFor(entry, named)
		if copied == "" {
			unvouched = named + " is inside it and no change names it"
			return nil
		}
		if before, now := fileDigest(copied), fileDigest(p); before == "" || before != now {
			unvouched = named + " does not hold what the run's copy of it holds and no change names it"
			return nil
		}
		return nil
	})
	return unvouched
}

// aKindTheRunNeverWrites reports whether a mode names something this installer cannot have
// put on a machine: a named pipe, a socket or a device. It writes regular files and links,
// the copier that takes its backups omits every other kind, and nothing in it calls mkfifo,
// mknod or opens a listener on a path. A path of such a kind is therefore never this run's
// doing, and the per-file record cannot carry one either.
func aKindTheRunNeverWrites(mode os.FileMode) bool {
	return !mode.IsRegular() && !mode.IsDir() && mode&os.ModeSymlink == 0
}

// readsAsAFile, the rule the guards above and the fingerprint both need, lives in state.go
// beside the digest helpers it serves: the rule is one and the callers are two files.

// sameKindAsCopy reports whether the run's copy of a path holds the same kind of thing the machine
// holds there. The copy is the state before the run, so a path whose copy holds the same kind is one
// the run found rather than made, and a path with no copy at all is one nothing here can speak for.
// The twelfth pass found no test that reaches it: the record shapes it was written for are settled by
// earlier rules today. It is kept because deleting it would guess what other shapes need.
func sameKindAsCopy(entry installEntry, path string) bool {
	copied := copiedFor(entry, path)
	if copied == "" {
		return false
	}
	onMachine, machineErr := os.Lstat(path)
	inCopy, copyErr := os.Lstat(copied)
	if machineErr != nil || copyErr != nil {
		return false
	}
	return onMachine.Mode() == inCopy.Mode()
}

// copiedFor returns the path the run's backup holds of one path inside a key, or "".
func copiedFor(entry installEntry, path string) string {
	if entry.Backup == "" || entry.Path == "" {
		return ""
	}
	rel, err := filepath.Rel(entry.Path, path)
	if err != nil {
		return ""
	}
	return filepath.Join(entry.Backup, rel)
}

// linkTarget returns what a link points at, or "" when it is not a link that can be read.
func linkTarget(path string) string {
	if path == "" {
		return ""
	}
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

// namesPathUnder reports whether any change names a path inside the given one.
func namesPathUnder(changes []fileChange, path string) bool {
	prefix := strings.TrimSuffix(path, string(os.PathSeparator)) + string(os.PathSeparator)
	for _, change := range changes {
		if strings.HasPrefix(change.Path, prefix) {
			return true
		}
	}
	return false
}

// writtenPaths returns the paths a record says the run wrote, for a record that
// carries no per-file changes: the entries whose own action says the run wrote them
// ("replaced", "created"), or -- for a record from before entries existed -- the
// Files list that reading it folds into entries. An entry the backup step merely
// copied or found is not one of them: that step copies what a run may write, so
// having a copy of a path says nothing about whether the run changed it. An empty
// result is the record saying the run changed nothing.
func writtenPaths(record *lastInstall) []installEntry {
	written := make([]installEntry, 0, len(record.Entries))
	for _, entry := range record.Entries {
		if entry.Action == installActionReplaced || entry.Action == installActionCreated {
			written = append(written, entry)
		}
	}
	if len(written) > 0 || len(record.Files) == 0 {
		return written
	}
	// Files is the record's only word on what it wrote, and the fold is where those
	// paths became entries: they are what there is to hand back.
	for _, entry := range record.Entries {
		if entry.Path != "" {
			written = append(written, entry)
		}
	}
	return written
}

// backupDirectoryFor returns the directory the entries' copies live in, or "" when
// none of them names a copy. A copy is named as a file inside that directory
// (BackupDir/key), and --restore takes the directory, so the way back a reader is
// handed has to be a command that works.
func backupDirectoryFor(entries []installEntry) string {
	for _, entry := range entries {
		if entry.Backup != "" {
			return filepath.Dir(entry.Backup)
		}
	}
	return ""
}

func RevertLastRun(opts RevertOptions) (RevertReport, error) {
	report := RevertReport{
		Files:        []RevertFileReport{},
		Irreversible: irreversibleEffects(),
		DryRun:       opts.DryRun,
	}

	record := readLastInstall()
	if record == nil {
		return report, errNoRecord
	}
	report.Record = record

	type revertItem struct {
		change fileChange
		status RevertStatus
		reason string
	}

	items := make([]revertItem, 0, len(record.Changes))
	keys := map[string]bool{}

	// Every path the record says the run wrote has to be accounted for by what the
	// record can put back. Two halves of a record make that claim -- the entries and
	// the Files list -- and only one of them carries a before-state file by file. The
	// index behind that half steps over anything that is not a regular file, so a path
	// can be named as written while the thing that changed inside it is a symlink the
	// record never lists. Acting on the changes and staying silent about the rest is
	// how a machine gets reported as back with a link still pointing where the run
	// left it, so the two halves are cross-checked here instead of one being trusted.
	// A record with no per-file changes at all reaches the same check with nothing
	// accounted for, which is what it means: it names paths and no before-state.
	written := writtenPaths(record)
	directory := backupDirectoryFor(written)
	for _, entry := range written {
		missing := firstUnaccountedFor(record, entry)
		if missing == "" {
			continue
		}

		action := fileActionModified
		if entry.Action == installActionCreated {
			action = fileActionCreated
		}
		reason := ""
		if missing == entry.Path {
			reason = "the record names this path as one the run wrote, but not the file it is or what it held, so nothing here can put it back"
		} else if why := unaccountedInside(record, entry, missing); why != "" {
			reason = "the record says the run wrote this path, and this is what it cannot account for: " + why
		} else {
			reason = "the record says the run wrote this path, and " + missing + " inside it is not a regular file the record lists, so nothing here can say what the run left there"
		}
		if directory != "" {
			reason += "; `dotfiles --restore=" + directory + "` puts that whole backup back"
		} else {
			reason += "; `ls -d ~/.dotfiles-backup-*` names the backups this machine took, and `dotfiles --restore=<one of them>` puts a whole backup back"
		}
		items = append(items, revertItem{
			change: fileChange{Key: entry.Key, Path: entry.Path, Action: action},
			status: RevertUnrevertible,
			reason: reason,
		})
	}

	for _, change := range record.Changes {
		item := revertItem{change: change}

		switch change.Action {
		case fileActionCreated:
			// Undoing a file the run created is removing it: there is no earlier
			// content to put back, because there was no file. A directory is not a
			// file, and putting one back means removing a tree that may hold work of
			// the reader's own: this command does not remove trees, and it says so
			// rather than reading an unreadable path as an absent one.
			if info, statErr := os.Lstat(change.Path); statErr == nil && info.IsDir() {
				item.status = RevertUnrevertible
				item.reason = "the record says the run created this path, and it is a directory now: putting that back means removing a tree"
			} else if currentFileDigest(change.Path) == "" {
				item.status = RevertAlready
				item.reason = "the run created this file and it is already gone"
			} else {
				item.status = RevertWouldRevert
			}

		case fileActionModified, fileActionDeleted:
			switch {
			case change.Before == "":
				item.status = RevertUnrevertible
				item.reason = "the record does not say what this file held before the run"
			case currentFileDigest(change.Path) == change.Before:
				item.status = RevertAlready
				item.reason = "the file already holds what it held before the run"
			case !findsBackupCopy(change):
				item.status = RevertUnrevertible
				item.reason = "the backup holds no copy of this file, so nothing can put it back"
			default:
				item.status = RevertWouldRevert
			}

		default:
			item.status = RevertUnrevertible
			item.reason = "the record names an action this revert does not know: " + string(change.Action)
		}

		if item.status == RevertWouldRevert && change.Key != "" {
			keys[change.Key] = true
		}
		items = append(items, item)
	}

	if !opts.DryRun && len(keys) > 0 {
		snapshotKeys := make([]string, 0, len(keys))
		for key := range keys {
			snapshotKeys = append(snapshotKeys, key)
		}
		sort.Strings(snapshotKeys)

		snapshotDir, _, snapshotErr := system.CreateBackup(snapshotKeys)
		if snapshotErr != nil {
			// The revert still runs: the user asked for the earlier state back, and
			// refusing because the safety copy failed would deny them that. What is
			// not allowed is going quiet about it.
			report.SnapshotReason = snapshotErr.Error()
		} else {
			report.SnapshotDir = snapshotDir
		}
	}

	for _, item := range items {
		file := RevertFileReport{
			Path:   item.change.Path,
			Key:    item.change.Key,
			Action: item.change.Action,
			Status: item.status,
			Reason: item.reason,
		}

		if !opts.DryRun && item.status == RevertWouldRevert {
			file.Status, file.Reason = applyRevert(item.change)
		}

		switch file.Status {
		case RevertReverted:
			report.Reverted++
			report.Planned++
		case RevertWouldRevert:
			report.Planned++
		case RevertAlready:
			report.AlreadyDone++
		case RevertUnrevertible:
			report.Unrevertible++
		case RevertFailed:
			report.Failed++
		}

		report.Files = append(report.Files, file)
	}

	if !report.Complete() {
		problems := make([]string, 0, report.Unrevertible+report.Failed)
		for _, file := range report.Files {
			if file.Status == RevertUnrevertible || file.Status == RevertFailed {
				problems = append(problems, file.Path+": "+file.Reason)
			}
		}
		return report, fmt.Errorf("the revert could not put back %d of %d files: %s",
			len(problems), len(report.Files), strings.Join(problems, "; "))
	}

	return report, nil
}

// applyRevert puts one file back and checks that it did, against the digest the
// record took before the run. The check is the point: a revert that reported
// success after copying the wrong bytes would be the defect it exists to fix, one
// layer further out.
func applyRevert(change fileChange) (RevertStatus, string) {
	note := ""

	if change.Action == fileActionCreated {
		if err := os.Remove(change.Path); err != nil && !os.IsNotExist(err) {
			return RevertFailed, err.Error()
		}
		if digest := currentFileDigest(change.Path); digest != "" {
			return RevertFailed, "the file is still there after removing it"
		}
		return RevertReverted, note
	}

	// A file that has changed again since the run is still reverted: the user
	// asked for the state before the run, and the state this replaces is kept in
	// the snapshot taken before any of this ran. Saying so is the difference
	// between a command they can trust and one that quietly eats an edit.
	if current := currentFileDigest(change.Path); current != "" && current != change.After {
		note = "the file had changed again since the run; the version this replaced is in the snapshot"
	}

	if err := system.PutBack(change.Backup, change.Path); err != nil {
		return RevertFailed, err.Error()
	}
	if digest := currentFileDigest(change.Path); digest != change.Before {
		return RevertFailed, "the file does not hold what it held before the run"
	}

	return RevertReverted, note
}

// findsBackupCopy reports whether the copy the record names is really there. The
// record is the run's own bookkeeping, so the revert checks the copy instead of
// trusting the path it was given.
func findsBackupCopy(change fileChange) bool {
	if change.Backup == "" {
		return false
	}
	_, err := os.Lstat(change.Backup)
	return err == nil
}

// currentFileDigest is the digest of a file as it is now, or "" when it is not
// there or cannot be read. It is fileDigest under a name that says which side of
// the comparison it belongs to.
func currentFileDigest(path string) string {
	return fileDigest(path)
}

// RestoreBackupDir puts a backup directory back and reports what it did, or what
// it would do with dryRun set. It is the path-level way back, for a machine whose
// record is gone: the record names individual files, and a backup directory names
// whole configurations.
func RestoreBackupDir(dir string, dryRun bool) (system.RestoreReport, error) {
	if dryRun {
		return system.RestorePlan(dir)
	}
	return system.RestoreBackupReport(dir)
}
