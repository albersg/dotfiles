package system

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// backupManifestName is the file that makes a directory a backup this program
// wrote. The restore screen offers what the listing returns for restore and for
// deletion, so a directory that carries neither a manifest nor the shape of a
// backup written before manifests existed is not offered at all: a stranger's
// folder named `.dotfiles-backup-*` used to be one keypress away from being
// restored over the user's configuration and from being deleted outright.
const backupManifestName = "backup.json"

// backupManifestSchema versions the shape below, so a future reader can tell a
// manifest that means something else from one that means this.
const backupManifestSchema = 1

// backupManifestEntry is one thing the backup holds.
type backupManifestEntry struct {
	Key  string `json:"key"`  // the ConfigPaths key it came from
	Rel  string `json:"rel"`  // its path relative to that key's root, "." for the root
	Path string `json:"path"` // where it was read from
	Kind string `json:"kind"` // file | dir | symlink
}

// backupManifestOmission is one thing the backup does not hold, and why. A
// backup that is quietly incomplete is worse than one that says what is missing:
// the first is trusted, the second can be acted on.
type backupManifestOmission struct {
	Key    string `json:"key"`
	Rel    string `json:"rel,omitempty"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type backupManifest struct {
	SchemaVersion int                      `json:"schemaVersion"`
	CreatedAt     time.Time                `json:"createdAt"`
	Entries       []backupManifestEntry    `json:"entries"`
	Skipped       []backupManifestOmission `json:"skipped,omitempty"`
	Failed        []backupManifestOmission `json:"failed,omitempty"`
}

// kindOf reports how the backup holds a path, by base name, so a caller can ask
// "is the symlink still a symlink" without knowing its key.
func (m backupManifest) kindOf(name string) string {
	for _, entry := range m.Entries {
		if entry.Rel == name || filepath.Base(entry.Rel) == name {
			return entry.Kind
		}
	}
	return ""
}

func readBackupManifest(dir string) (backupManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, backupManifestName))
	if err != nil {
		return backupManifest{}, err
	}

	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return backupManifest{}, err
	}
	return manifest, nil
}

func writeBackupManifest(dir string, manifest backupManifest) error {
	manifest.SchemaVersion = backupManifestSchema
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, backupManifestName), data, 0o644)
}

// createBackupDir takes a directory of its own. The readable timestamp is kept,
// and a run that lands in the same second takes the next free suffix instead of
// merging into the other run's backup: two runs sharing a directory meant the
// second one's failure deleted the first one's copies.
func createBackupDir() (string, error) {
	base := GetBackupDir()
	for i := 1; i <= 100; i++ {
		candidate := base
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}

		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create backup directory: %w", err)
		}
	}
	return "", fmt.Errorf("failed to find a free backup directory beside %s", base)
}

// CreateBackup copies the requested configurations into a new backup directory
// and writes the manifest that says what the directory holds.
//
// It returns the backup path and the entries it could not copy from inside a
// directory, which are skipped rather than fatal: a live socket such as
// ~/.config/herdr/herdr.sock lives beside real configuration and would otherwise
// abort the whole installation. An entry that cannot be captured is recorded in
// the manifest either way, so the list of what is missing survives the run
// instead of scrolling past in a log.
//
// A path that cannot be captured counts as a failure of the backup, because the
// run must not overwrite what could not be protected. The copies already made
// stay on disk with the manifest that names what failed: they are the user's way
// back, and deleting them was the one outcome with nothing to show for it.
func CreateBackup(configs []string) (string, []string, error) {
	backupDir, err := createBackupDir()
	if err != nil {
		return "", nil, err
	}

	configPaths := ConfigPaths()
	manifest := backupManifest{CreatedAt: time.Now()}
	skippedPaths := []string{}

	for _, configKey := range configs {
		// Extract key from "key: path" format if present
		key := configKey
		if idx := strings.Index(configKey, ":"); idx > 0 {
			key = configKey[:idx]
		}

		srcPath, exists := configPaths[key]
		if !exists {
			continue
		}

		// Lstat, not Stat: a symlink the user made is part of the tree they had,
		// and following it here would replace it in the backup with whatever it
		// pointed at, so restoring would hand back a tree they never had.
		if _, err := os.Lstat(srcPath); err != nil {
			// Nothing to protect: the path does not exist yet, and the record of
			// the run is what says the install created it.
			continue
		}

		dstPath := filepath.Join(backupDir, key)
		entries := []backupManifestEntry{}
		omissions := []backupManifestOmission{}
		copyErr := copyTreePreservingShape(srcPath, dstPath,
			func(rel, path, kind string) {
				entries = append(entries, backupManifestEntry{Key: key, Rel: rel, Path: path, Kind: kind})
			},
			func(rel, path, reason string) {
				omissions = append(omissions, backupManifestOmission{Key: key, Rel: rel, Path: path, Reason: reason})
			})
		manifest.Entries = append(manifest.Entries, entries...)
		manifest.Skipped = append(manifest.Skipped, omissions...)
		for _, omission := range omissions {
			skippedPaths = append(skippedPaths, omission.Path)
		}

		if copyErr != nil {
			manifest.Failed = append(manifest.Failed, backupManifestOmission{
				Key:    key,
				Path:   srcPath,
				Reason: copyErr.Error(),
			})
			// The manifest is written on the failure path too, and its write
			// error is dropped rather than returned: the failure the caller has
			// to hear about is the one that stopped the backup, not this one.
			_ = writeBackupManifest(backupDir, manifest)
			return backupDir, skippedPaths, fmt.Errorf(
				"failed to backup %s: %w; what was copied is kept in %s, where %s names what could not be captured",
				key, copyErr, backupDir, backupManifestName)
		}
	}

	if err := writeBackupManifest(backupDir, manifest); err != nil {
		return backupDir, skippedPaths, fmt.Errorf("failed to write the backup's manifest in %s: %w", backupDir, err)
	}
	return backupDir, skippedPaths, nil
}

// copyTreePreservingShape copies a file, a directory or a symlink, keeping the
// shape of what it finds: a symlink stays a symlink and a directory stays a
// directory, because a copy that resolves a link hands back a tree the user
// never had. It is the one copier behind the backup and the restore, so both
// agree about what "a copy of this" means.
//
// record is told about every entry written and omit about every entry that could
// not be written; either may be nil. An entry that is not a regular file, a
// directory or a symlink is omitted rather than fatal when it lives inside a
// directory, and is an error when it is the path itself: a whole configuration
// path that cannot be copied is a path the caller cannot promise to protect.
func copyTreePreservingShape(src, dst string, record func(rel, path, kind string), omit func(rel, path, reason string)) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(target, dst); err != nil {
			return err
		}
		if record != nil {
			record(filepath.Base(src), src, "symlink")
		}
		return nil

	case info.Mode().IsRegular():
		if err := CopyFile(src, dst); err != nil {
			return err
		}
		if record != nil {
			record(filepath.Base(src), src, "file")
		}
		return nil

	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		if record != nil {
			record(".", src, "dir")
		}
		return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == src {
				return nil
			}

			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dst, rel)

			switch {
			case d.Type()&os.ModeSymlink != 0:
				link, err := os.Readlink(path)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.Symlink(link, target); err != nil {
					return err
				}
				if record != nil {
					record(rel, path, "symlink")
				}

			case d.IsDir():
				childInfo, err := d.Info()
				if err != nil {
					return err
				}
				if err := os.MkdirAll(target, childInfo.Mode().Perm()); err != nil {
					return err
				}
				if record != nil {
					record(rel, path, "dir")
				}

			case d.Type().IsRegular():
				if err := CopyFile(path, target); err != nil {
					return err
				}
				if record != nil {
					record(rel, path, "file")
				}

			default:
				if omit != nil {
					omit(rel, path, describeNonRegular(d.Type()))
				}
			}
			return nil
		})

	default:
		return fmt.Errorf("copy %s: %w", src, ErrNotRegularFile)
	}
}

// RestoreEntryReport is what happened to one entry of a backup.
type RestoreEntryReport struct {
	Key    string `json:"key"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"` // restored | skipped | failed
	Reason string `json:"reason,omitempty"`
}

// RestoreReport is the account a restore gives of itself: what it put back, what
// it could not place and why, and where the state it replaced is kept. "Your
// configuration is back" and "two of the five files are back" are different
// answers, and only the report can tell them apart.
type RestoreReport struct {
	SnapshotDir    string               `json:"snapshotDir,omitempty"`
	SnapshotReason string               `json:"snapshotReason,omitempty"`
	Entries        []RestoreEntryReport `json:"entries"`
	Restored       int                  `json:"restored"`
	Skipped        int                  `json:"skipped"`
	Failed         int                  `json:"failed"`
}

// RestoreBackupReport puts a backup back and reports what it did, entry by
// entry. Three rules make it safe to run on a machine whose configuration the
// user still cares about:
//
//   - It verifies before it removes. The replacement is copied beside the
//     destination first, so an entry that cannot be put back leaves the
//     destination exactly as it was.
//   - It keeps the state it replaces. The paths about to be overwritten are
//     copied into a backup of their own first, so a restore is itself a change
//     the user can undo.
//   - It refuses a directory this program did not write. The listing only offers
//     real backups, but a directory can be named by hand, and deleting a
//     configuration on the strength of a stranger's file is not a service.
func RestoreBackupReport(backupDir string) (RestoreReport, error) {
	report := RestoreReport{Entries: []RestoreEntryReport{}}
	configPaths := ConfigPaths()

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return report, fmt.Errorf("failed to read backup directory: %w", err)
	}

	if _, manifestErr := readBackupManifest(backupDir); manifestErr != nil && !looksLikeLegacyBackup(backupDir) {
		return report, fmt.Errorf("%s is not a backup this program wrote: it holds neither %s nor the shape backups had before manifests existed",
			backupDir, backupManifestName)
	}

	type restoreItem struct{ key, src, dst string }
	plan := []restoreItem{}
	keys := []string{}

	for _, entry := range entries {
		key := entry.Name()
		if key == backupManifestName {
			continue
		}

		dstPath, exists := configPaths[key]
		if !exists {
			report.Entries = append(report.Entries, RestoreEntryReport{
				Key:    key,
				Status: "skipped",
				Reason: "no configuration path is named " + key + ", so there is nothing to put this entry back to",
			})
			report.Skipped++
			continue
		}

		srcPath := filepath.Join(backupDir, key)
		if _, statErr := os.Lstat(srcPath); statErr != nil {
			report.Entries = append(report.Entries, RestoreEntryReport{
				Key:    key,
				Path:   dstPath,
				Status: "skipped",
				Reason: "the entry cannot be read, so nothing replaced anything: " + statErr.Error(),
			})
			report.Skipped++
			continue
		}

		plan = append(plan, restoreItem{key: key, src: srcPath, dst: dstPath})
		keys = append(keys, key)
	}

	if len(keys) > 0 {
		snapshotDir, _, snapshotErr := CreateBackup(keys)
		if snapshotErr != nil {
			// The restore still runs: the user asked for the earlier state back,
			// and refusing because the safety copy failed would deny them that.
			// What is not allowed is going quiet about it.
			report.SnapshotReason = snapshotErr.Error()
		} else {
			report.SnapshotDir = snapshotDir
		}
	}

	failures := []string{}
	for _, item := range plan {
		if err := swapIn(item.src, item.dst); err != nil {
			report.Entries = append(report.Entries, RestoreEntryReport{
				Key:    item.key,
				Path:   item.dst,
				Status: "failed",
				Reason: err.Error(),
			})
			report.Failed++
			failures = append(failures, item.key+": "+err.Error())
			continue
		}
		report.Entries = append(report.Entries, RestoreEntryReport{
			Key:    item.key,
			Path:   item.dst,
			Status: "restored",
		})
		report.Restored++
	}

	if len(failures) > 0 {
		return report, fmt.Errorf("restoring %s failed for %d of %d entries: %s",
			backupDir, report.Failed, len(plan), strings.Join(failures, "; "))
	}
	return report, nil
}

// swapIn puts src in place at dst: the copy is made beside the destination and
// only a complete copy is moved into position, so the destination is never
// absent because a copy failed halfway. A destination that cannot be moved aside
// stops the swap before anything is replaced.
func swapIn(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(parent, ".dotfiles-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, filepath.Base(dst))
	if err := copyTreePreservingShape(src, staged, nil, nil); err != nil {
		return err
	}

	if _, statErr := os.Lstat(dst); os.IsNotExist(statErr) {
		// Nothing to replace: the path did not exist when the backup was taken.
		return os.Rename(staged, dst)
	} else if statErr != nil {
		return statErr
	}

	aside, err := os.MkdirTemp(parent, ".dotfiles-restore-aside-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(aside)

	held := filepath.Join(aside, filepath.Base(dst))
	if err := os.Rename(dst, held); err != nil {
		return err
	}
	if err := os.Rename(staged, dst); err != nil {
		// Put the user's own back: a swap that cannot finish is a swap that did
		// not happen.
		_ = os.Rename(held, dst)
		return err
	}
	return nil
}

// describeNonRegular says what a socket, a pipe or a device is, in the words the
// person reading the manifest needs rather than the words of a mode bit.
func describeNonRegular(mode fs.FileMode) string {
	switch {
	case mode&os.ModeSocket != 0:
		return "a socket: live runtime state, not configuration"
	case mode&os.ModeNamedPipe != 0:
		return "a named pipe: live runtime state, not configuration"
	case mode&os.ModeDevice != 0:
		return "a device: live runtime state, not configuration"
	default:
		return "not a regular file, so it cannot be copied"
	}
}

// ListBackups returns the backups this program wrote, newest name order as the
// filesystem returns them. A directory counts only when it carries a manifest,
// or when it holds nothing but configuration keys, which is the shape backups
// had before manifests existed: the ones already on a machine stay restorable.
func ListBackups() []BackupInfo {
	home := os.Getenv("HOME")
	backups := []BackupInfo{}

	entries, err := os.ReadDir(home)
	if err != nil {
		return backups
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".dotfiles-backup-") {
			continue
		}

		backupPath := filepath.Join(home, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		manifest, manifestErr := readBackupManifest(backupPath)
		if manifestErr != nil && !looksLikeLegacyBackup(backupPath) {
			continue
		}

		files := []string{}
		subEntries, _ := os.ReadDir(backupPath)
		for _, sub := range subEntries {
			if sub.Name() == backupManifestName {
				continue
			}
			files = append(files, sub.Name())
		}

		timestamp := info.ModTime()
		if manifestErr == nil && !manifest.CreatedAt.IsZero() {
			// The name carries the second the backup ran and the mtime carries
			// whatever touched the directory last, so the manifest is the only
			// account of when this backup was taken that a copy cannot move.
			timestamp = manifest.CreatedAt
		}

		backups = append(backups, BackupInfo{
			Path:        backupPath,
			Timestamp:   timestamp,
			Files:       files,
			HasManifest: manifestErr == nil,
		})
	}

	return backups
}

// looksLikeLegacyBackup reports whether a directory has the shape backups had
// before this program wrote a manifest into one: a non-empty directory holding
// nothing but names from ConfigPaths(). The check exists so that backups already
// on a machine are still offered after the manifest arrived; a stranger's folder
// named like a backup holds what a stranger put there, and fails it.
func looksLikeLegacyBackup(dir string) bool {
	keys := ConfigPaths()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	found := 0
	for _, entry := range entries {
		if _, ok := keys[entry.Name()]; !ok {
			return false
		}
		found++
	}
	return found > 0
}
