package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
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

// lastInstall is what the installer writes when a run completes: when it
// finished (RFC 3339 through the JSON encoder), the build that produced it, and
// the configuration paths that run found and replaced. Files is empty on a
// machine that had nothing to overwrite, which the panel reports by leaving the
// row out rather than printing a zero.
type lastInstall struct {
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Files     []string  `json:"files"`
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
	return &rec
}
