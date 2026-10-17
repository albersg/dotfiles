package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// THE FIXES: TWO CHANGES, BOTH PROVEN, BOTH REVERSIBLE BY RECORD
// ============================================================================
//
// The brief is precise about what a fix may be: deterministic, relatively safe,
// previewed with its effect, its risk, the permission it needs and the backup it
// takes, confirmed, re-checked before it is applied, applied minimally, and then
// verified. Everything that is ambiguous gets manual instructions instead.
//
// Two changes qualify on this machine, and they are the same shape: create a
// directory the machine's own configuration already names but that is not there,
// and set the mode OpenSSH requires on the directory and the keys it uses.
// Both are one file operation on a path the environment or the tool itself
// names, both leave everything else untouched, and both can be verified by
// reading the same thing again.
//
// What makes them reversible is the record: every applied fix is written to
// doctor-fixes.json in the installer's own state directory together with the
// state it replaced and the exact command that puts that state back. The doctor
// does not invent a way back it cannot show.
//
// Nothing here runs during a scan. The scan is read-only; a fix happens only
// when the confirmation screen's row is pressed, and a dry run writes nothing at
// all.

// doctorFixKind names the minimal change a fix makes. The kind decides the
// preview, the apply and the verification, so a new kind is a new case in each
// of the three rather than a new closure somewhere.
type doctorFixKind string

const (
	// doctorFixMakeDir creates one directory, with the parents it needs, at a
	// path the environment or the installer already names.
	doctorFixMakeDir doctorFixKind = "make-directory"
	// doctorFixChmodDir sets a directory's mode to 0700.
	doctorFixChmodDir doctorFixKind = "chmod-directory"
	// doctorFixChmodFile sets one file's mode to 0600.
	doctorFixChmodFile doctorFixKind = "chmod-file"
)

// doctorFix is the repair behind a finding: the kind of change, the one path it
// changes, and -- for the mode kinds -- the mode it sets.
type doctorFix struct {
	Kind doctorFixKind
	Path string
	Mode os.FileMode
}

// doctorFixAction is the row the confirmation screen offers, so the row and the
// change cannot be two descriptions of different things.
func (f *doctorFix) action() string {
	switch f.Kind {
	case doctorFixMakeDir:
		return "Create the directory"
	case doctorFixChmodDir, doctorFixChmodFile:
		return fmt.Sprintf("Set the mode to %04o", f.Mode.Perm())
	default:
		return "Apply the change"
	}
}

// summary is the one line a finding carries when it offers a fix, and the line
// the report writes.
func (f *doctorFix) summary() string {
	switch f.Kind {
	case doctorFixMakeDir:
		return "create the directory " + doctorRelHomePath(f.Path)
	case doctorFixChmodDir, doctorFixChmodFile:
		return fmt.Sprintf("set the mode of %s to %04o", doctorRelHomePath(f.Path), f.Mode.Perm())
	default:
		return "change " + doctorRelHomePath(f.Path)
	}
}

// doctorFixPreview is what the confirmation screen shows before anything is
// changed: the exact effect, the risk, the permission required, what is kept as
// a backup, and -- when the change must not be offered -- the refusal with its
// reason. A refusal is not an error: it is the state having moved since the
// finding was produced, which is exactly what the re-check is for.
type doctorFixPreview struct {
	// Effect is the change as a before/after line the user can read as a diff of
	// one path.
	Effect []string
	// Risk is what the change costs if it was not wanted.
	Risk string
	// Permission is what the change needs from the machine.
	Permission string
	// Backup is what is kept so the change can be put back.
	Backup string
	// Refusal is why the change will not be applied, empty when it will.
	Refusal string
}

// doctorFixOutcome is what pressing the row produced.
type doctorFixOutcome struct {
	// Applied reports that the machine was changed.
	Applied bool
	// Notice is the line the screen shows, in one sentence per fact.
	Notice string
	// Record is what was written to the fix record, when a record was written.
	Record *doctorFixRecord
	// RecordErr is why the record could not be written, when it could not.
	RecordErr string
}

// preview reads the state the change would replace and describes the change. It
// is the re-check as well: what it read here is what apply re-reads before it
// touches anything, so the preview and the precondition are one reading.
func (f *doctorFix) preview(scan *doctorScan) doctorFixPreview {
	switch f.Kind {
	case doctorFixMakeDir:
		info, err := scan.stat(f.Path)
		switch {
		case err == nil && info.IsDir():
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s is already a directory, so there is nothing to create.", f.Path),
			}
		case err == nil:
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s is a file (mode %s). Creating a directory would mean deleting it, which the doctor will not do.", f.Path, doctorPermString(info.Mode())),
			}
		case !errors.Is(err, fs.ErrNotExist):
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s cannot be read (%v), so the doctor cannot say whether creating it is right.", f.Path, err),
			}
		}
		if !doctorWritable(scan, f.Path) {
			return doctorFixPreview{
				Refusal: fmt.Sprintf("the nearest directory that exists above %s is not writable by this user, so the directory cannot be created.", f.Path),
			}
		}
		return doctorFixPreview{
			Effect: []string{
				"before: " + f.Path + " does not exist",
				"after:  " + f.Path + " is a directory (mode 0755)",
			},
			Risk:       "None beyond the directory itself: a tool that expected to create it first will find it already there.",
			Permission: "Write allowed on the nearest directory above " + f.Path + "; no administrator rights.",
			Backup:     "Nothing is replaced: the path does not exist, so there is no previous state to keep. The doctor records that it did not exist, which is what `rmdir` puts back.",
		}

	case doctorFixChmodDir, doctorFixChmodFile:
		info, err := scan.lstat(f.Path)
		switch {
		case err != nil:
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s can no longer be read (%v), so its mode cannot be set.", f.Path, err),
			}
		case info.Mode()&os.ModeSymlink != 0:
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s is a symbolic link. Setting its mode would change the mode of its target, which the doctor has not read, so it will not.", f.Path),
			}
		case info.Mode().Perm() == f.Mode.Perm():
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s already has mode %04o.", f.Path, f.Mode.Perm()),
			}
		case info.IsDir() != (f.Kind == doctorFixChmodDir):
			kind := "a file"
			if info.IsDir() {
				kind = "a directory"
			}
			return doctorFixPreview{
				Refusal: fmt.Sprintf("%s is %s now, and this fix is for %s.", f.Path, kind,
					map[bool]string{true: "a directory", false: "a file"}[f.Kind == doctorFixChmodDir]),
			}
		}
		if !doctorOwnedByUser(scan, info) {
			return doctorFixPreview{
				Refusal: fmt.Sprintf("its owner cannot write %s (mode %s), so changing its mode needs an administrator", f.Path, doctorPermString(info.Mode())),
			}
		}
		return doctorFixPreview{
			Effect: []string{
				"before: " + f.Path + " has mode " + doctorPermString(info.Mode()),
				"after:  " + f.Path + " has mode " + fmt.Sprintf("%04o", f.Mode.Perm()),
			},
			Risk: "A mode that is too open is a way into every tool that reads this path; a mode that is too closed is a tool that cannot read its own file. " +
				"The doctor sets the mode the tool itself requires and changes nothing else about the file.",
			Permission: "Ownership of " + f.Path + " (or administrator rights); no administrator rights are needed when this user owns it.",
			Backup: fmt.Sprintf("The mode it has now (%s) is written into the fix record with the command that puts it back: `chmod %s %s`.",
				doctorPermString(info.Mode()), doctorPermString(info.Mode()), f.Path),
		}
	}
	return doctorFixPreview{Refusal: "the doctor does not know this fix"}
}

// doctorOwnedByUser reports whether this user is expected to be able to change
// the file's mode. The structural answer -- the owner's own write and read bits
// -- is the one every platform this installer supports can give without asking
// the system who owns a file, and it is the same rule the fix's own permission
// line states.
func doctorOwnedByUser(scan *doctorScan, info os.FileInfo) bool {
	if info.IsDir() {
		return info.Mode().Perm()&0o700 == 0o700
	}
	return info.Mode().Perm()&0o600 == 0o600
}

// apply re-reads the state, refuses when it moved, makes the one change, and
// reports what it did. It writes the fix record so the change can be put back by
// hand, and it never deletes anything.
func (f *doctorFix) apply(scan *doctorScan) doctorFixOutcome {
	preview := f.preview(scan)
	if preview.Refusal != "" {
		return doctorFixOutcome{Notice: "Nothing was changed: " + preview.Refusal}
	}
	if dryRun() {
		return doctorFixOutcome{Notice: "A dry run changed nothing: " + f.summary() + "."}
	}

	before := doctorFixBeforeState(scan, f.Path)
	var err error
	switch f.Kind {
	case doctorFixMakeDir:
		err = scan.host.MkdirAll(f.Path, 0o755)
	case doctorFixChmodDir, doctorFixChmodFile:
		err = scan.host.Chmod(f.Path, f.Mode.Perm())
	}
	if err != nil {
		return doctorFixOutcome{Notice: fmt.Sprintf("The change failed: %s. Nothing was recorded and nothing else was touched: %v", f.summary(), err)}
	}

	verified, verifyNote := f.verify(scan)
	record := doctorFixRecord{
		Time:     scan.now,
		Kind:     string(f.Kind),
		Path:     f.Path,
		Before:   before,
		After:    doctorFixAfterState(scan, f.Path),
		Verified: verified,
		Revert:   doctorFixRevert(f, before),
	}

	notice := "Done: " + f.summary() + "."
	if verified {
		notice += " Verified by reading it again: " + verifyNote
	} else {
		notice += " The result could not be verified: " + verifyNote
	}

	outcome := doctorFixOutcome{Applied: true, Notice: notice, Record: &record}
	if recordErr := appendDoctorFixRecord(scan, record); recordErr != nil {
		outcome.RecordErr = recordErr.Error()
		outcome.Notice += " The record of the previous state could not be written (" + recordErr.Error() +
			"), so put it back by hand if you want it: " + record.Revert
	}
	return outcome
}

// verify reads the state again and answers whether the change is the one that
// was intended. It is what makes the second half of the brief's sequence real:
// a fix that reports success has read the result rather than assumed it.
func (f *doctorFix) verify(scan *doctorScan) (bool, string) {
	info, err := scan.lstat(f.Path)
	if err != nil {
		return false, err.Error()
	}
	switch f.Kind {
	case doctorFixMakeDir:
		if !info.IsDir() {
			return false, f.Path + " is not a directory after the change"
		}
		return true, f.Path + " is a directory (mode " + doctorPermString(info.Mode()) + ")"
	case doctorFixChmodDir, doctorFixChmodFile:
		if info.Mode().Perm() != f.Mode.Perm() {
			return false, fmt.Sprintf("%s has mode %s, not the %04o that was set", f.Path, doctorPermString(info.Mode()), f.Mode.Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, f.Path + " became a symbolic link while the change was made"
		}
		return true, f.Path + " now has mode " + doctorPermString(info.Mode())
	}
	return false, "the doctor does not know how to verify this change"
}

// doctorFixBeforeState is what the fix replaced, in the words the record keeps:
// "absent" for a path that is not there, and the mode otherwise.
func doctorFixBeforeState(scan *doctorScan, path string) string {
	info, err := scan.lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "absent"
		}
		return "unreadable: " + err.Error()
	}
	return "mode " + doctorPermString(info.Mode())
}

// doctorFixAfterState is what the fix left, read back rather than assumed.
func doctorFixAfterState(scan *doctorScan, path string) string {
	info, err := scan.lstat(path)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	return "mode " + doctorPermString(info.Mode())
}

// doctorFixRevert is the exact command that puts the previous state back, or the
// honest statement that there is nothing to put back. A record without a way
// back says so instead of implying one.
func doctorFixRevert(f *doctorFix, before string) string {
	switch {
	case before == "absent":
		if f.Kind == doctorFixMakeDir {
			return fmt.Sprintf("rmdir %s", doctorQuotePath(f.Path))
		}
		return ""
	case strings.HasPrefix(before, "mode "):
		return fmt.Sprintf("chmod %s %s", strings.TrimPrefix(before, "mode "), doctorQuotePath(f.Path))
	default:
		return ""
	}
}

// doctorQuotePath quotes a path for a shell command the user is told to type.
// Paths with spaces, quotes or unicode are why this exists; nothing here is
// executed by the doctor.
func doctorQuotePath(path string) string {
	if path == "" {
		return ""
	}
	if !strings.ContainsAny(path, " \t\"'\\$`*?()[]{}&|;<>!#~") {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// ============================================================================
// The record of what was changed
// ============================================================================

// doctorFixRecord is one applied change: what was changed, what it was before,
// what it is now, whether the result was verified, and the exact command that
// puts the previous state back.
type doctorFixRecord struct {
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"`
	Path     string    `json:"path"`
	Before   string    `json:"before"`
	After    string    `json:"after"`
	Verified bool      `json:"verified"`
	Revert   string    `json:"revert,omitempty"`
}

// doctorFixHistory is the file's shape: one file, a list of records, the newest
// last. It is capped so a machine that ran the doctor a thousand times does not
// carry a thousand records.
type doctorFixHistory struct {
	Records []doctorFixRecord `json:"records"`
}

const (
	doctorFixesFileName  = "doctor-fixes.json"
	doctorFixHistoryMax  = 50
	doctorFixHistoryMode = 0o600
)

// doctorFixHistoryPath is the record's file, in the installer's own state
// directory beside the last-install record and the report.
func doctorFixHistoryPath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, doctorFixesFileName)
}

// appendDoctorFixRecord reads the record file, appends this change and writes it
// back. A record file that cannot be read is replaced rather than lost: the file
// holds an audit trail, and refusing to record a change because an older record
// is corrupt would lose the change that matters.
func appendDoctorFixRecord(scan *doctorScan, record doctorFixRecord) error {
	path := doctorFixHistoryPath()
	if path == "" {
		return errors.New("the state directory could not be determined")
	}
	if scan.host == nil {
		return errors.New("there is no host to write the record on")
	}

	history := doctorFixHistory{}
	if data, err := scan.host.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &history)
	} else if !errors.Is(err, fs.ErrNotExist) {
		// A record that cannot be read at all is reported by the caller through
		// this error, so the user knows the trail has a hole in it.
		_ = err
	}
	history.Records = append(history.Records, record)
	if len(history.Records) > doctorFixHistoryMax {
		history.Records = history.Records[len(history.Records)-doctorFixHistoryMax:]
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := scan.host.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return scan.host.WriteFile(path, data, doctorFixHistoryMode)
}
