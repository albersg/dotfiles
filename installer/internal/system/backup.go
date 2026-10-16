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
		info, err := os.Lstat(srcPath)
		if err != nil {
			// Nothing to protect: the path does not exist yet, and the record of
			// the run is what says the install created it.
			continue
		}

		dstPath := filepath.Join(backupDir, key)
		entries, omissions, copyErr := copyBackupEntry(key, srcPath, dstPath, info)
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

// copyBackupEntry copies one configuration path and reports what it holds.
func copyBackupEntry(key, src, dst string, info os.FileInfo) ([]backupManifestEntry, []backupManifestOmission, error) {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return nil, nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, nil, err
		}
		if err := os.Symlink(target, dst); err != nil {
			return nil, nil, err
		}
		return []backupManifestEntry{{Key: key, Rel: filepath.Base(src), Path: src, Kind: "symlink"}}, nil, nil

	case info.IsDir():
		return copyDirForBackup(key, src, dst)

	case info.Mode().IsRegular():
		if err := CopyFile(src, dst); err != nil {
			return nil, nil, err
		}
		return []backupManifestEntry{{Key: key, Rel: filepath.Base(src), Path: src, Kind: "file"}}, nil, nil

	default:
		return nil, nil, fmt.Errorf("copy %s: %w", src, ErrNotRegularFile)
	}
}

// copyDirForBackup copies a directory tree for the backup, preserving the shape
// of what it finds: a symlink stays a symlink, a directory stays a directory,
// and an entry that cannot be copied is left out and named in the manifest.
func copyDirForBackup(key, src, dst string) ([]backupManifestEntry, []backupManifestOmission, error) {
	rootInfo, err := os.Lstat(src)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(dst, rootInfo.Mode().Perm()); err != nil {
		return nil, nil, err
	}

	entries := []backupManifestEntry{{Key: key, Rel: ".", Path: src, Kind: "dir"}}
	omissions := []backupManifestOmission{}

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
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
			entries = append(entries, backupManifestEntry{Key: key, Rel: rel, Path: path, Kind: "symlink"})

		case d.IsDir():
			info, err := d.Info()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
			entries = append(entries, backupManifestEntry{Key: key, Rel: rel, Path: path, Kind: "dir"})

		case d.Type().IsRegular():
			if err := CopyFile(path, target); err != nil {
				return err
			}
			entries = append(entries, backupManifestEntry{Key: key, Rel: rel, Path: path, Kind: "file"})

		default:
			omissions = append(omissions, backupManifestOmission{
				Key:    key,
				Rel:    rel,
				Path:   path,
				Reason: describeNonRegular(d.Type()),
			})
		}
		return nil
	})
	if err != nil {
		return entries, omissions, err
	}
	return entries, omissions, nil
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
