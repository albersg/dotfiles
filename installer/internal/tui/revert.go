package tui

import (
	"fmt"
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

	// A record with no per-file changes cannot be undone one file at a time. What it
	// can still say is which paths the run wrote, and whether it says any is the
	// question that separates two records that look alike: one written before this
	// command had per-file changes, and one written by a run that changed nothing.
	// Reading the record for that answer -- instead of recognising a shape -- is what
	// keeps a record written by any version this repository ever shipped from being
	// told one thing when it means the other.
	if len(record.Changes) == 0 {
		written := writtenPaths(record)
		directory := backupDirectoryFor(written)
		for _, entry := range written {
			action := fileActionModified
			if entry.Action == installActionCreated {
				action = fileActionCreated
			}
			reason := "the record names this path as one the run wrote, but not the file it is or what it held, so nothing here can put it back"
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
	}

	for _, change := range record.Changes {
		item := revertItem{change: change}

		switch change.Action {
		case fileActionCreated:
			// Undoing a file the run created is removing it: there is no earlier
			// content to put back, because there was no file.
			if currentFileDigest(change.Path) == "" {
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
