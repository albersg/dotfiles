package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The installer's own state file.
//
// The installer leaves one small record behind when a run finishes, so the next
// run can say when this machine was last installed, from which build, and which
// configuration paths that run touched. It is a convenience, not a source of
// truth: the installer does not read it to decide anything, so a missing, stale
// or corrupt file only costs the "Last install" panel, never a step.
//
// The file lives under the XDG state directory -- $XDG_STATE_HOME, falling back
// to ~/.local/state -- in a dotfiles directory, because that is the directory
// the XDG base-directory specification reserves for state a program wants to
// keep between runs and that other programs must not read. It is not under the
// config directory: nothing here configures the installer.
//
// The render path never touches it. It is read once on the startup path, like
// the backups and the existing-config scan, and written once when the run
// completes; both happen outside View.
const (
	// stateAppDir is the directory under the state home that belongs to this
	// program, so the file is namespaced instead of loose in the state root.
	stateAppDir = "dotfiles"

	// lastInstallFileName is the record's name inside that directory.
	lastInstallFileName = "last-install.json"

	// stateDirMode and stateFileMode are the permissions the file is created with.
	// The record names local paths and holds nothing sensitive, so it is readable
	// by the user it belongs to and no wider.
	stateDirMode  = 0o755
	stateFileMode = 0o644
)

// installAction names what a run did to one configuration path. The record
// exists so a reader can tell what changed, not merely what was on disk before
// the run: a path that was detected but never written must not read like one the
// run overwrote, which is the defect reported in issue #239.
type installAction string

const (
	// installActionFound: the path existed before the run and the run left it
	// alone, with no copy taken.
	installActionFound installAction = "found"
	// installActionBackedUp: the path existed before the run and was copied into
	// the backup directory, but the run did not rewrite it.
	installActionBackedUp installAction = "backed-up"
	// installActionReplaced: the run wrote over a path that existed before it.
	installActionReplaced installAction = "replaced"
	// installActionCreated: the run wrote a path that did not exist before it.
	installActionCreated installAction = "created"
)

// installEntry is one configuration path and what the run did to it. Backup
// names the copy saved before a replacement or existence check, when the backup
// step took one; it is empty when no copy was made. The record never infers a
// backup: an entry that names one names a file that is there.
type installEntry struct {
	Key    string        `json:"key"`
	Path   string        `json:"path"`
	Action installAction `json:"action"`
	Backup string        `json:"backup,omitempty"`
}

// lastInstall is what the installer writes when a run completes: when it
// finished (RFC 3339 through the JSON encoder), the build that produced it, and
// what the run did to each configuration path it names.
//
// Entries is the authoritative list: one record per configuration path, each
// naming its outcome. Files is the legacy field and keeps a reader that predates
// Entries working; for a record written since Entries existed it holds the paths
// the run actually wrote (replaced or created), one "key: path" string each, and
// is empty on a run that wrote nothing, which the panel reports by leaving the
// row out rather than printing a zero.
//
// A record written by an earlier build carries only Files. It still loads: the
// reader turns each of those paths into an entry whose outcome is unknown and is
// therefore reported as "found", never as a replacement the old build did not
// establish.
type lastInstall struct {
	Timestamp time.Time      `json:"timestamp"`
	Version   string         `json:"version"`
	Files     []string       `json:"files"`
	Entries   []installEntry `json:"entries,omitempty"`
}

// stateDir returns the directory the record lives in: $XDG_STATE_HOME/dotfiles
// when the variable is set, and ~/.local/state/dotfiles otherwise. An empty
// string means neither could be determined, and the caller then reads no record
// and writes none.
func stateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, stateAppDir)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", stateAppDir)
}

// lastInstallPath is the exact file the installer reads and writes. It is named
// once here so the documentation, the writer and the reader cannot disagree.
func lastInstallPath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, lastInstallFileName)
}

// writeLastInstall records a completed run. It is best effort: the caller ignores
// its error, because a state directory that cannot be created or written must
// never turn an install that finished into an install that failed.
func writeLastInstall(rec lastInstall) error {
	path := lastInstallPath()
	if path == "" {
		return errors.New("could not determine the state directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), stateDirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, stateFileMode)
}

// readLastInstall returns the record of the last completed run, or nil when
// there is none. A missing file, an unreadable one, a corrupt one and one whose
// timestamp is zero all read as "no record": a partial file is not a run, and
// the panel would rather say nothing than say "never".
func readLastInstall() *lastInstall {
	path := lastInstallPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rec lastInstall
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil
	}
	if rec.Timestamp.IsZero() {
		return nil
	}
	if len(rec.Entries) == 0 && len(rec.Files) > 0 {
		rec.Entries = legacyEntries(rec.Files)
	}
	return &rec
}

// legacyEntries turns a record written before Entries existed into entries. The
// old build recorded the paths the run found, not the ones it replaced, so the
// only honest outcome to give them is "found": the file existed and this build
// cannot say it was rewritten. The "key: path" shape is the one the detection
// has always written.
func legacyEntries(files []string) []installEntry {
	entries := make([]installEntry, 0, len(files))
	for _, file := range files {
		key, path, ok := strings.Cut(file, ":")
		if !ok {
			entries = append(entries, installEntry{Path: file, Action: installActionFound})
			continue
		}
		entries = append(entries, installEntry{
			Key:    strings.TrimSpace(key),
			Path:   strings.TrimSpace(path),
			Action: installActionFound,
		})
	}
	return entries
}

// lastInstallRecord builds the record of the run that just finished. It names
// what the run did from the change in each configuration path's bytes, not from
// the pre-run detection that recordLastInstall used to copy:
//
//   - a configuration path whose content the run wrote is "replaced" when it
//     existed before the run and "created" when it did not;
//   - a path the run did not write is "backed-up" when the backup step saved a
//     copy of it and "found" when it did not.
//
// The change is read from the fingerprint the run took before its first step
// (Model.ConfigFingerprints), not from a modification time: a filesystem may
// leave a file's timestamp where it was when the file is rewritten, so a record
// built on timestamps would call a replacement a found file on exactly those
// machines. A model with no fingerprint (a unit test that never started a run)
// has nothing to compare, so it claims no replacement at all.
//
// The record agrees with ConfigPaths() by construction, because it walks that map
// and classifies every key. A path the map does not name is not written into the
// user's home in the first place (the guard
// TestEveryHomePathTheInstallerWritesIsClassified keeps that true), so every write
// this run made is classified here and none of the pre-run detection is mistaken
// for one.
func lastInstallRecord(m *Model, now time.Time) lastInstall {
	rec := lastInstall{Timestamp: now, Version: VersionLabel()}

	detected := detectedConfigKeys(m.ExistingConfigs)
	before := m.ConfigFingerprints
	after := configFingerprints()

	paths := system.ConfigPaths()
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		path := paths[key]
		_, statErr := os.Stat(path)
		exists := statErr == nil

		backup := ""
		if m.BackupDir != "" {
			if candidate := filepath.Join(m.BackupDir, key); pathExists(candidate) {
				backup = candidate
			}
		}

		beforeFingerprint := before[key]
		existedBefore := beforeFingerprint != "" || detected[key] || backup != ""
		changed := before != nil && beforeFingerprint != after[key]

		switch {
		case changed && existedBefore:
			rec.Entries = append(rec.Entries, installEntry{Key: key, Path: path, Action: installActionReplaced, Backup: backup})
			rec.Files = append(rec.Files, key+": "+path)
		case changed:
			rec.Entries = append(rec.Entries, installEntry{Key: key, Path: path, Action: installActionCreated})
			rec.Files = append(rec.Files, key+": "+path)
		case backup != "":
			rec.Entries = append(rec.Entries, installEntry{Key: key, Path: path, Action: installActionBackedUp, Backup: backup})
		case existedBefore && exists:
			rec.Entries = append(rec.Entries, installEntry{Key: key, Path: path, Action: installActionFound})
		}
	}

	return rec
}

// detectedConfigKeys reads the keys out of the "key: path" strings
// DetectExistingConfigs returns, so the record can tell a path that was already
// on disk from one the run created.
func detectedConfigKeys(configs []string) map[string]bool {
	keys := make(map[string]bool, len(configs))
	for _, config := range configs {
		key, _, ok := strings.Cut(config, ":")
		if !ok {
			continue
		}
		keys[strings.TrimSpace(key)] = true
	}
	return keys
}

// ConfigFingerprints hashes the content of every configuration path before a run
// writes anything. The run takes it once, in executeStep, and carries it to the
// record; lastInstallRecord takes it again at the end and compares the two.
func configFingerprints() map[string]string {
	paths := system.ConfigPaths()
	fingerprints := make(map[string]string, len(paths))
	for key, path := range paths {
		fingerprints[key] = fingerprintPath(path)
	}
	return fingerprints
}

// fingerprintPath returns a stable hash of a file's bytes, or of every regular
// file under a directory, or "" when the path does not exist. The empty string
// is the value for a path that is not there, so a fingerprint that is no longer
// empty is a path the run created.
func fingerprintPath(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	digest := sha256.New()
	if !info.IsDir() {
		writeFileFingerprint(digest, path)
		return hex.EncodeToString(digest.Sum(nil))
	}

	// WalkDir visits in lexical order, so the hash does not depend on the
	// filesystem's directory order.
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return nil
		}
		_, _ = fmt.Fprintf(digest, "%s\x00", filepath.ToSlash(rel))
		writeFileFingerprint(digest, p)
		return nil
	})
	return hex.EncodeToString(digest.Sum(nil))
}

// writeFileFingerprint folds one file's bytes into a digest. A file that cannot
// be read contributes nothing, which keeps the fingerprint over the files that
// are readable rather than failing the run's bookkeeping over one of them.
func writeFileFingerprint(digest hash.Hash, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(digest, "%d\x00", len(data))
	_, _ = digest.Write(data)
	_, _ = digest.Write([]byte{0})
}

// pathExists reports whether path is there, following the same rule the rest of
// the installer uses for a backup copy: a stat that succeeds is a file it can
// point at.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
