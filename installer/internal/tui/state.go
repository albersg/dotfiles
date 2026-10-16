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
	// installActionRemoved: the run removed a file that was there before it. It is
	// a file-level action: the run removes files inside a configuration directory,
	// never a whole key, and the record's own Deleted change is what names it.
	installActionRemoved installAction = "removed"
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

// fileAction names what a run did to one file. The path-level record could not
// tell a rewritten directory from a deleted one, and neither could be undone
// from it, so the file is the unit the record names.
type fileAction string

const (
	fileActionCreated  fileAction = "created"
	fileActionModified fileAction = "modified"
	fileActionDeleted  fileAction = "deleted"
)

// fileChange is one file a run changed, named with the digests that make the
// change checkable after the fact: Before is what the file held before the run
// (empty when the run created it), After is what it held when the run recorded
// itself (empty when the run removed it), and Backup is the copy that puts Before
// back, when the backup holds one. A change with no copy is a change nothing can
// undo, and the revert has to report that rather than perform it.
type fileChange struct {
	Key    string     `json:"key,omitempty"`
	Path   string     `json:"path"`
	Action fileAction `json:"action"`
	Before string     `json:"before,omitempty"`
	After  string     `json:"after,omitempty"`
	Backup string     `json:"backup,omitempty"`
}

// runStatus says how a run ended. A run that stopped at a failed step is recorded
// as failed and names the steps that failed: the record of the last run is what a
// reader trusts to know the state of the machine, and a failed run that left no
// record made that state unreadable at exactly the moment it mattered.
type runStatus string

const (
	runStatusComplete runStatus = "complete"
	runStatusFailed   runStatus = "failed"
)

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
// Status, FailedSteps and Changes are what the record added for a person who has
// to know what a run did to their machine. A record written by an older build
// carries none of them, and reads as what it was: a run of unknown outcome whose
// files are listed as found.
type lastInstall struct {
	Timestamp   time.Time      `json:"timestamp"`
	Version     string         `json:"version"`
	Status      runStatus      `json:"status,omitempty"`
	FailedSteps []string       `json:"failedSteps,omitempty"`
	Files       []string       `json:"files"`
	Entries     []installEntry `json:"entries,omitempty"`
	Changes     []fileChange   `json:"changes,omitempty"`
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

	// What the run did file by file, which is the granularity a revert needs and
	// the only one that can name a file the run removed.
	rec.Changes = m.resolveChanges(fileChanges(m.ConfigFileIndex, configFileIndex()))

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
// namedPath maps a path inside a walked tree back to the name the record uses for it.
// A walk that follows a link visits the paths behind it, and the record has to name
// what a person sees -- the key's path -- or the revert looks for a copy that the
// backup, which keeps the link, was never going to hold under that name.
func namedPath(keyPath, walkRoot, walked string) string {
	rel, err := filepath.Rel(walkRoot, walked)
	if err != nil || rel == "." {
		return keyPath
	}
	return filepath.Join(keyPath, rel)
}

func fingerprintPath(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	// The walk follows a key that is a link, for the same reason the index does: a
	// fingerprint taken at the link sees a key that never changes, and a run that
	// wrote through it would be recorded as having found it.
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
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

// configFileIndex is every regular file under every configuration path, mapped to
// the digest of its bytes. A run takes it before its first step and again when it
// records itself; the two are what name the individual files the run created,
// modified and deleted.
//
// It walks the same trees fingerprintPath does, and it is deliberately a second
// read rather than a rewrite of the path-level fingerprint: this happens once per
// install, and the record it enables is worth one pass over the configuration.
func configFileIndex() map[string]string {
	index := make(map[string]string)

	for _, path := range system.ConfigPaths() {
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		// A key is read through its link on purpose: the file's content is what a
		// change is measured against, and the backup keeps the link itself. A link to
		// a DIRECTORY is the same question one level up, and getting it wrong is how a
		// run that wrote through one is recorded as having found a key it changed: the
		// run writes through the link (the install copies into the key), so the files
		// inside it are files this run can change, and a walk that stops at the link
		// records none of them.
		walkRoot := path
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(path)
			resolvedInfo, statErr := os.Stat(path)
			if resolveErr != nil || statErr != nil || !resolvedInfo.IsDir() {
				if digest := fileDigest(path); digest != "" {
					index[path] = digest
				}
				continue
			}
			walkRoot = resolved
		} else if !info.IsDir() {
			if digest := fileDigest(path); digest != "" {
				index[path] = digest
			}
			continue
		}

		_ = filepath.WalkDir(walkRoot, func(p string, entry fs.DirEntry, err error) error {
			if err != nil || !entry.Type().IsRegular() {
				return nil
			}
			// The record names the paths a person sees, never the paths behind their
			// links: the same file has two names once a key is a link.
			if digest := fileDigest(p); digest != "" {
				index[namedPath(path, walkRoot, p)] = digest
			}
			return nil
		})
	}

	return index
}

// fileDigest returns the digest of a file's bytes, or "" when it cannot be read.
// A file that cannot be read stays out of the index rather than being recorded as
// empty, so "not there" and "unreadable" never read as a change to nothing.
func fileDigest(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fileChanges compares the index taken before a run with the one taken after it
// and names every file the run created, modified or removed. It is the whole
// answer to "what did this run change", one file at a time, and it is what the
// record's Changes holds and what a revert reads.
//
// An index that was never taken is not evidence: a model built by hand in a test,
// or a run that failed before its first step, would otherwise have every file on
// the machine recorded as one this run created.
func fileChanges(before, after map[string]string) []fileChange {
	if before == nil || after == nil {
		return nil
	}

	changes := []fileChange{}
	for path, digest := range after {
		was, existed := before[path]
		switch {
		case !existed:
			changes = append(changes, fileChange{Path: path, Action: fileActionCreated, After: digest})
		case was != digest:
			changes = append(changes, fileChange{Path: path, Action: fileActionModified, Before: was, After: digest})
		}
	}
	for path, digest := range before {
		if _, still := after[path]; !still {
			changes = append(changes, fileChange{Path: path, Action: fileActionDeleted, Before: digest})
		}
	}

	// Lexical order, so two records of the same machine are comparable by eye and
	// by diff, and a revert walks the files in the order a reader expects.
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

// resolveChanges names the configuration key each changed file belongs to and the
// copy that puts it back, so the record is readable and actionable on its own.
func (m *Model) resolveChanges(changes []fileChange) []fileChange {
	if len(changes) == 0 {
		return changes
	}
	paths := system.ConfigPaths()
	for i := range changes {
		changes[i].Key, changes[i].Backup = changeOwner(changes[i].Path, m.BackupDir, paths)
	}
	return changes
}

// changeOwner names the configuration key a file belongs to and, when the backup
// took a copy of it, the copy's path. The copy holds what the file was before the
// run -- the backup step runs before anything writes -- so a file the run deleted
// is still in there, and it is what a revert copies back.
func changeOwner(path, backupDir string, paths map[string]string) (string, string) {
	key := ownerKey(path, paths)
	if key == "" || backupDir == "" {
		return key, ""
	}

	rel, err := filepath.Rel(paths[key], path)
	if err != nil {
		return key, ""
	}
	copy := filepath.Join(backupDir, key, rel)
	if _, err := os.Lstat(copy); err != nil {
		// The backup does not hold this file, so nothing here can put it back.
		// Naming a path that is not there would be worse than naming none.
		return key, ""
	}
	return key, copy
}

// ownerKey names the configuration key a file belongs to: the longest root that
// holds it, so a key nested inside another one wins over the directory that
// contains it.
func ownerKey(path string, paths map[string]string) string {
	key, best := "", -1
	for candidate, root := range paths {
		if path == root {
			return candidate
		}
		if strings.HasPrefix(path, root+string(filepath.Separator)) && len(root) > best {
			key, best = candidate, len(root)
		}
	}
	return key
}

// --- Verification against the record ---------------------------------------

// ErrNoRecord is the one read failure that is not a failure: the machine has no
// record because nothing was installed here, so there is nothing to verify and
// nothing to revert. It is a sentinel so a caller -- the report inside this
// package, and the command line outside it -- can tell it apart from a record
// that is present and cannot be read: "nothing to do" and "it went wrong" are
// different answers, and they carry different exit statuses.
var ErrNoRecord = errors.New("no record of an install")

// errNoRecord is the name this package reads it by.
var errNoRecord = ErrNoRecord

// VerifyReport is what --verify found and what it could not. Lines is the report
// the user reads, in order. Consistent is true only when every path and backup
// the record names is still there. Unreadable is true when a record is present
// and could not be read at all, which is not the same as there being no record.
//
// The two booleans are how a script and a reader tell the three outcomes apart:
// a record that cannot be read and a record whose paths are gone are different
// facts about the machine, and a machine with no record is a third.
type VerifyReport struct {
	Lines      []string
	Consistent bool
	Unreadable bool
}

// readLastInstallRecord reads the record for verification. Unlike readLastInstall
// it tells "there is no record" (errNoRecord) apart from "there is a record and
// it cannot be read" (any other error). The panel folds both into nil because a
// panel that would say "never" is better silent; verification must not fold
// them, because its exit codes differ: nothing installed here is not a failure,
// while a record that cannot be read is.
func readLastInstallRecord() (*lastInstall, error) {
	path := lastInstallPath()
	if path == "" {
		return nil, errors.New("the state directory could not be determined")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errNoRecord
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var rec lastInstall
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if rec.Timestamp.IsZero() {
		return nil, fmt.Errorf("parse %s: the record names no run", path)
	}
	if len(rec.Entries) == 0 && len(rec.Files) > 0 {
		rec.Entries = legacyEntries(rec.Files)
	}
	return &rec, nil
}

// installFindingKind names what one finding is: a path the record names that is
// still here or gone, or the copy a replacement took. The two questions are kept
// apart because a path that is here with its backup gone is a different fact from
// a path that is gone: the first machine can still be rolled back to the second's
// state only if the copy survived, which is exactly what this catches.
type installFindingKind int

const (
	// findingPathPresent: the record names a path and it is still there. Its
	// contents cannot be judged, because the record holds no digest.
	findingPathPresent installFindingKind = iota
	// findingPathMissing: the record names a path and the machine no longer has it.
	findingPathMissing
	// findingBackupPresent: the record names the copy a replacement took and the
	// copy is still there.
	findingBackupPresent
	// findingBackupMissing: the record names a copy and it is gone, so the change
	// it was taken for can no longer be put back.
	findingBackupMissing
	// findingFileHoldsWhatTheRunLeft: the record names a file and the digest it had
	// when the run finished writing, and the file still holds it. This is the only
	// kind that is a claim about content, and it is a claim the record supports.
	findingFileHoldsWhatTheRunLeft
	// findingFileChangedSinceTheRun: the record names a file and a digest, and the
	// file no longer holds it -- edited afterwards, or put back after the run
	// removed it.
	findingFileChangedSinceTheRun
	// findingFileUnreadable: the file is here and could not be read, so the report
	// cannot say whether it holds what the run left. A third answer on purpose:
	// calling it changed and calling it fine are both claims the record does not
	// support.
	findingFileUnreadable
)

// ChangedSinceTheRun reports whether the finding is a file the report can no
// longer say holds what the run left.
func (k installFindingKind) ChangedSinceTheRun() bool {
	return k == findingFileChangedSinceTheRun || k == findingFileUnreadable
}

// Missing reports whether the finding is something the machine no longer has.
func (k installFindingKind) Missing() bool {
	return k == findingPathMissing || k == findingBackupMissing
}

// Copy reports whether the finding is about the copy the run took rather than
// about the path itself.
func (k installFindingKind) Copy() bool {
	return k == findingBackupPresent || k == findingBackupMissing
}

// installFinding is one fact the verification established: something the record
// names and what the machine says about it now. It is a fact rather than a
// sentence so the command line and the interface can each say it in their own
// shape. They are two presenters of one walk: a second walk of the same record
// is what this shape exists to prevent, because two walks can disagree about the
// same file and only one of them would be right.
type installFinding struct {
	// Kind is what the machine says about this path or copy.
	Kind installFindingKind
	// Key is the configuration path's own name, as the record carries it.
	Key string
	// Path is the path itself, or the copy, depending on Kind.
	Path string
	// Action is what the run did to the path, kept for the path's own findings and
	// unused by the ones about a copy.
	Action installAction
	// Detail is the one comparison behind a per-file finding, already written out:
	// the digests that differ, or that the file could not be read. The presenters
	// print it as it stands rather than working anything out themselves.
	Detail string
}

// installVerdict is what comparing this machine with the record of the last
// completed run found, as facts: the run, or the reason there is no readable run
// to compare with, and one finding per path and copy the record names.
type installVerdict struct {
	// NoRecord is true when there is no record at all. Nothing was installed on
	// this machine, which is not the same fact as a record that cannot be read.
	NoRecord bool
	// Unreadable reports a record that is present and could not be read.
	Unreadable bool
	// Reason is why the record could not be read. It is set only beside Unreadable.
	Reason string
	// Record is the record that was read, and nil unless one was.
	Record *lastInstall
	// Findings is one fact per path, copy and changed file the record names: the
	// paths and copies in the record's own order, each with the files the record
	// knows about inside it. A path a per-file finding covers is not repeated, so
	// nothing is said twice and nothing is said in weaker words than it deserves.
	Findings []installFinding
}

// Consistent reports whether every path and copy the record names is still here.
// A machine with no record is consistent rather than divergent: nothing was
// installed, so nothing was lost. That is the same reading the command line's
// exit status takes, and the two must agree, because a script and a reader are
// looking at one fact.
func (v installVerdict) Consistent() bool {
	if v.NoRecord {
		return true
	}
	if v.Unreadable {
		return false
	}
	return v.Missing() == 0 && v.Divergent() == 0
}

// Divergent counts the findings that say the machine is no longer what the run
// left: something it names that is gone, and a file that no longer holds the
// digest the run recorded. The two are counted together because they are the same
// answer to the question --verify asks, and they are reported apart because they
// are different things to do something about.
func (v installVerdict) Divergent() int {
	divergent := 0
	for _, finding := range v.Findings {
		if finding.Kind.Missing() || finding.Kind.ChangedSinceTheRun() {
			divergent++
		}
	}
	return divergent
}

// Missing counts the findings the machine no longer has.
func (v installVerdict) Missing() int {
	missing := 0
	for _, finding := range v.Findings {
		if finding.Kind.Missing() {
			missing++
		}
	}
	return missing
}

// InspectInstall compares this machine with the record of the last completed run
// and returns what it found as facts. It is the one walk: VerifyInstall renders
// these facts for the command line, and the utilities section's own screen
// renders them for a reader, so the two cannot come to different conclusions
// about one record.
//
// Nothing is written and nothing but the record and the paths it names is read.
func InspectInstall() installVerdict {
	rec, err := readLastInstallRecord()
	if errors.Is(err, errNoRecord) {
		return installVerdict{NoRecord: true}
	}
	if err != nil {
		return installVerdict{Unreadable: true, Reason: err.Error()}
	}

	verdict := installVerdict{Record: rec}

	// The per-file findings are what make this a check of content rather than of
	// presence: each one names the digest the file held when the run finished, so
	// the report can say whether it still holds it. A change the record cannot
	// check -- one that names no digest -- takes no part, and leaves its path to
	// the path-level finding, because presence is all such a record supports.
	byKey := map[string][]fileChange{}
	covered := map[string]bool{}
	for _, change := range rec.Changes {
		if change.Path == "" {
			continue
		}
		if change.Action != fileActionDeleted && change.After == "" {
			continue
		}
		byKey[change.Key] = append(byKey[change.Key], change)
		covered[change.Path] = true
	}

	for _, entry := range rec.Entries {
		// A record that names no path has nothing to compare with the machine. It is
		// skipped rather than counted as gone: the machine did not lose anything.
		if entry.Path == "" {
			continue
		}

		// A path a per-file finding covers is not reported twice: the file's own
		// finding is the stronger fact, and a path-level line would repeat it with
		// less confidence and in weaker words.
		if !covered[entry.Path] {
			kind := findingPathMissing
			if pathExists(entry.Path) {
				kind = findingPathPresent
			}
			verdict.Findings = append(verdict.Findings, installFinding{
				Kind:   kind,
				Key:    entry.Key,
				Path:   entry.Path,
				Action: entry.Action,
			})
		}

		if entry.Backup != "" {
			copyKind := findingBackupMissing
			if pathExists(entry.Backup) {
				copyKind = findingBackupPresent
			}
			verdict.Findings = append(verdict.Findings, installFinding{
				Kind: copyKind,
				Key:  entry.Key,
				Path: entry.Backup,
			})
		}

		for _, change := range byKey[entry.Key] {
			verdict.Findings = append(verdict.Findings, inspectFileChange(change))
		}
		delete(byKey, entry.Key)
	}

	// A file whose key the entries do not name still belongs in the report: the run
	// recorded changing it, and the reader is owed it.
	orphans := make([]string, 0, len(byKey))
	for key := range byKey {
		orphans = append(orphans, key)
	}
	sort.Strings(orphans)
	for _, key := range orphans {
		for _, change := range byKey[key] {
			verdict.Findings = append(verdict.Findings, inspectFileChange(change))
		}
	}

	return verdict
}

// inspectFileChange establishes one fact about one file the record names, from the
// digest the run took and the bytes on disk now.
func inspectFileChange(change fileChange) installFinding {
	finding := installFinding{
		Key:    change.Key,
		Path:   change.Path,
		Action: actionForChange(change.Action),
	}

	if !pathExists(change.Path) {
		if change.Action == fileActionDeleted {
			// The run removed it and it is still gone, which is what the run left.
			finding.Kind = findingFileHoldsWhatTheRunLeft
			return finding
		}
		finding.Kind = findingPathMissing
		return finding
	}

	if change.Action == fileActionDeleted {
		// Something put it back after the run removed it.
		finding.Kind = findingFileChangedSinceTheRun
		finding.Detail = "the run removed it and it is here again"
		return finding
	}

	switch digest := fileDigest(change.Path); {
	case digest == "":
		finding.Kind = findingFileUnreadable
	case digest == change.After:
		finding.Kind = findingFileHoldsWhatTheRunLeft
	default:
		finding.Kind = findingFileChangedSinceTheRun
		finding.Detail = "expected " + shortDigest(change.After) + ", found " + shortDigest(digest)
	}
	return finding
}

// actionForChange says what the run did in the path-level vocabulary, so a
// per-file finding reads in the same words as the path-level ones.
func actionForChange(action fileAction) installAction {
	switch action {
	case fileActionCreated:
		return installActionCreated
	case fileActionDeleted:
		return installActionRemoved
	default:
		return installActionReplaced
	}
}

// shortDigest is the first twelve characters of a digest: enough for a reader to
// compare two of them, short enough to keep the report a report.
func shortDigest(digest string) string {
	if digest == "" {
		return "(none)"
	}
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

// verifyLines renders a verdict as the report --verify prints: one line per
// fact, aligned so the outcome is the first column, and the limit the record
// carries stated on every line that could otherwise be read as a stronger claim
// than presence. It is a presenter and nothing else: every fact it prints comes
// from the verdict it was handed.
func verifyLines(v installVerdict) []string {
	switch {
	case v.NoRecord:
		return []string{
			"dotfiles " + BuildLabel(),
			"No record of an install on this machine: nothing was installed here, so there is nothing to verify.",
		}
	case v.Unreadable:
		return []string{
			"dotfiles " + BuildLabel(),
			"Could not read the record: " + v.Reason,
		}
	}

	lines := []string{
		"dotfiles " + BuildLabel(),
		"Record from " + v.Record.Timestamp.Format(panelBackupStamp) + " by " + v.Record.Version + ".",
	}
	for _, finding := range v.Findings {
		phrase := verifyActionPhrase(finding.Action)
		switch finding.Kind {
		case findingPathPresent:
			lines = append(lines, fmt.Sprintf(
				"here     %s  %s  %s; the record names no digest, so it cannot say whether this is still what the install left",
				finding.Key, finding.Path, phrase))
		case findingPathMissing:
			lines = append(lines, fmt.Sprintf(
				"missing  %s  %s  %s, and the path the record names is not here",
				finding.Key, finding.Path, phrase))
		case findingBackupPresent:
			lines = append(lines, fmt.Sprintf(
				"here     backup of %s  %s  the record names no digest, so it cannot say whether this is still that copy",
				finding.Key, finding.Path))
		case findingBackupMissing:
			lines = append(lines, fmt.Sprintf(
				"missing  backup of %s  %s  the record names this copy and it is not here",
				finding.Key, finding.Path))
		case findingFileHoldsWhatTheRunLeft:
			phrase := "still holds what the run left"
			if finding.Action == installActionRemoved {
				phrase = "is still gone, as the run left it"
			}
			lines = append(lines, fmt.Sprintf("here     %s  %s  %s", finding.Key, finding.Path, phrase))
		case findingFileChangedSinceTheRun:
			phrase := "no longer holds what the run left"
			if finding.Action == installActionRemoved {
				phrase = "was removed by the run and is here again"
			}
			if finding.Detail != "" {
				phrase += ": " + finding.Detail
			}
			lines = append(lines, fmt.Sprintf("changed  %s  %s  %s", finding.Key, finding.Path, phrase))
		case findingFileUnreadable:
			lines = append(lines, fmt.Sprintf(
				"unknown  %s  %s  the file could not be read, so this report cannot say whether it still holds what the run left",
				finding.Key, finding.Path))
		}
	}
	return lines
}

// VerifyInstall compares the machine with the record of the last completed run
// and returns the report --verify prints. It reports what the record supports.
//
// For a file the run changed, the record carries the digest it held when the run
// finished, so the report says whether it still holds it: that is the difference
// between "the file is here" and "the machine is what the install left", and only
// the second is worth checking. Where the record carries no digest -- a record
// written before it kept them, or a path the run did not write -- presence is all
// that can be established, and the report says so rather than turning presence
// into a claim about content.
//
// Verification never writes: it reads the machine and the record and nothing else.
func VerifyInstall() VerifyReport {
	verdict := InspectInstall()
	return VerifyReport{
		Lines:      verifyLines(verdict),
		Consistent: verdict.Consistent(),
		Unreadable: verdict.Unreadable,
	}
}

// verifyActionPhrase renders what the run did to a path as the clause the
// verification report uses. An action this build does not know is reported as
// recorded rather than guessed at.
func verifyActionPhrase(action installAction) string {
	switch action {
	case installActionCreated:
		return "created by the run"
	case installActionReplaced:
		return "replaced by the run"
	case installActionBackedUp:
		return "backed up by the run"
	case installActionFound:
		return "found and left alone by the run"
	case installActionRemoved:
		return "removed by the run"
	default:
		return "recorded by the run"
	}
}
