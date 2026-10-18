package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE DOTFILES THIS INSTALLER OWNS
// ============================================================================
//
// The question here is narrower than "is everything in the home directory
// right": it is what happened to the files **this installer wrote**. The record
// of the last run (state.go) names every path it touched and every backup it
// took, so the check reads that record rather than scanning the home directory
// and inventing ownership. A file the record does not name is reported as not
// this installer's, which is the honest answer and the one the brief asks for:
// "No asumir propiedad sobre archivos ajenos."
//
// Two other things live here. The links: a path the record names that is a
// symbolic link whose target is gone is a broken installation even though the
// link is present, and `os.Stat` alone cannot tell the two apart. And the modes:
// a configuration file the installer wrote that others can write is a fault the
// user did not ask for, and the two most private paths on the machine (a private
// key, a credential file) are checked in the git check where they belong.
//
// The reader is readLastInstallRecord, the same one --verify uses, so the two
// halves of the record cannot disagree about what it says.

// doctorDotfilesChecks is the dotfiles category's catalogue.
func doctorDotfilesChecks() []doctorCheck {
	return []doctorCheck{
		{
			ID:       "dotfiles.record",
			Category: doctorCategoryDotfiles,
			Title:    "The record of the last installation can be read",
			Run:      doctorCheckDotfilesRecord,
		},
		{
			ID:       "dotfiles.paths",
			Category: doctorCategoryDotfiles,
			Title:    "Every path the last run touched is still there",
			Required: true,
			Run:      doctorCheckDotfilesPaths,
		},
		{
			ID:       "dotfiles.links",
			Category: doctorCategoryDotfiles,
			Title:    "Every symbolic link this installer manages has a target",
			Run:      doctorCheckDotfilesLinks,
		},
		{
			ID:       "dotfiles.permissions",
			Category: doctorCategoryDotfiles,
			Title:    "No configuration file this installer wrote is writable by others",
			Run:      doctorCheckDotfilesPermissions,
		},
		{
			ID:       "dotfiles.foreign",
			Category: doctorCategoryDotfiles,
			Title:    "A configuration this installer does not manage is named, not claimed",
			Run:      doctorCheckDotfilesForeign,
		},
	}
}

// doctorRecord is what the dotfiles checks read once: the record and why it
// could not be read, so four checks do not read the same file four times.
type doctorRecord struct {
	record *lastInstall
	// absent reports there is no record at all, which is not a failure: nothing
	// was installed here.
	absent bool
	// err is why a record that is present could not be read.
	err error
}

// doctorReadRecord reads the last-install record through the same function
// --verify uses.
func doctorReadRecord(scan *doctorScan) doctorRecord {
	path := lastInstallPath()
	if path == "" {
		return doctorRecord{err: errors.New("the state directory could not be determined")}
	}
	if _, err := scan.stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return doctorRecord{absent: true}
		}
		return doctorRecord{err: err}
	}
	rec, err := readLastInstallRecord()
	if errors.Is(err, errNoRecord) {
		return doctorRecord{absent: true}
	}
	if err != nil {
		return doctorRecord{err: err}
	}
	return doctorRecord{record: rec}
}

// doctorCheckDotfilesRecord reports the record itself: its identity, and the
// three outcomes that are not the same thing (there is a record; there is none
// because nothing was installed here; a record is there and cannot be read).
func doctorCheckDotfilesRecord(scan *doctorScan) []doctorFinding {
	read := doctorReadRecord(scan)
	path := doctorRelHome(scan, lastInstallPath())

	switch {
	case read.absent:
		return []doctorFinding{{
			Subject:  "The record of the last installation",
			Status:   doctorNotApplicable,
			Evidence: "there is no record at " + path,
			Impact:   "Nothing has been installed by this tool as this user, so there are no managed paths to check. This is not a fault.",
			Advice:   "Run the installer if you want a machine installed and a record written.",
		}}
	case read.err != nil:
		return []doctorFinding{{
			Subject:  "The record of the last installation",
			Status:   doctorWarning,
			Evidence: fmt.Sprintf("%s cannot be read: %v", path, read.err),
			Impact:   "What the last run touched is unknown, so the checks that read the record cannot say anything; the installer's own `--verify` reports the same fault.",
			Advice:   "Check the file's permissions and its JSON; a record that is half-written is replaced by the next run.",
		}}
	}

	rec := read.record
	entries := len(rec.Entries)
	evidence := fmt.Sprintf("%s records the run of %s by %s: %s",
		path, rec.Timestamp.Format("2006-01-02 15:04:05"), rec.Version,
		doctorCounted(entries, "path", "paths"))
	if entries == 0 {
		return []doctorFinding{{
			Subject:  "The record of the last installation",
			Status:   doctorWarning,
			Evidence: evidence + ", and no path",
			Impact:   "A run that recorded no path wrote nothing, or wrote before the record carried paths: either way there is nothing to compare with the machine.",
			Advice:   "Run the installer again; the current version records every path it touches.",
		}}
	}
	return []doctorFinding{{
		Subject:  "The record of the last installation",
		Status:   doctorOK,
		Evidence: evidence,
		Impact:   "The checks below compare these paths with the machine.",
		Advice:   "Nothing to do.",
	}}
}

// doctorCheckDotfilesPaths compares the paths the record names with the machine,
// and the backup copies with the machine. It is the read half of the record, at
// the same level of detail --verify prints, but classified.
//
// A path the record says the run created or replaced, and that is gone, is a
// warning: the user may have deleted it deliberately, and the installer does not
// claim ownership of what the user removes. A missing backup is a warning of its
// own, because the copy is what makes the next replacement safe.
func doctorCheckDotfilesPaths(scan *doctorScan) []doctorFinding {
	read := doctorReadRecord(scan)
	if read.absent {
		return []doctorFinding{{
			Subject:  "The paths the last run touched",
			Status:   doctorNotApplicable,
			Evidence: "there is no record of an install on this machine",
			Impact:   "Nothing was installed here, so there is nothing to compare.",
			Advice:   "Nothing to do.",
		}}
	}
	if read.err != nil {
		return []doctorFinding{{
			Subject:  "The paths the last run touched",
			Status:   doctorUnknown,
			Evidence: fmt.Sprintf("the record could not be read: %v", read.err),
			Impact:   "The paths the last run touched are unknown.",
			Advice:   "Repair the record, or run the installer again.",
		}}
	}

	var findings []doctorFinding
	var present []string
	missingCreations, missingBackups := 0, 0
	for _, entry := range read.record.Entries {
		if entry.Path == "" {
			continue
		}
		info, err := scan.lstat(entry.Path)
		if err != nil {
			status := doctorWarning
			impact := "The configuration the installer wrote here is not on the machine, so the tool it configures reads whatever else it finds."
			if entry.Action == installActionFound || entry.Action == installActionBackedUp {
				// The run found this path and left it alone. That it is gone now is
				// the user's doing, not the installer's, so the finding states the
				// difference rather than crying damage.
				status = doctorWarning
				impact = "The run found this path and left it alone; it is gone now, which is the user's own change and not a fault of the installation."
			}
			findings = append(findings, doctorFinding{
				Subject:  entry.Key + ": " + doctorRelHome(scan, entry.Path),
				Status:   status,
				Evidence: fmt.Sprintf("the record says the run %s it, and it is not there now (%v)", doctorActionPhrase(entry.Action), err),
				Impact:   impact,
				Advice:   "Restore it from the backup the record names, or run the installer's step for this tool again.",
			})
			missingCreations++
			continue
		}
		present = append(present, doctorRelHome(scan, entry.Path)+" ("+doctorPermString(info.Mode())+")")

		if entry.Backup == "" {
			continue
		}
		if _, err := scan.lstat(entry.Backup); err != nil {
			missingBackups++
			findings = append(findings, doctorFinding{
				Subject: "backup of " + entry.Key,
				Status:  doctorWarning,
				Evidence: fmt.Sprintf("the record names %s as the copy taken before the run, and it is not there (%v)",
					doctorRelHome(scan, entry.Backup), err),
				Impact: "The copy that would put the previous configuration back is gone, so the next replacement of this path cannot be undone.",
				Advice: "Take a copy of the file as it is now if you want a way back, or accept the loss: the installer's backup step only copies what is there at the time.",
			})
		}
	}

	if missingCreations == 0 && missingBackups == 0 {
		return []doctorFinding{{
			Subject: "The paths the last run touched",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("all %s the record names are present, and every backup it names too: %s",
				doctorCounted(len(present), "path", "paths"), doctorPathList(present, 5)),
			Impact: "The installation the record describes is the installation on the machine.",
			Advice: "Nothing to do.",
		}}
	}
	return findings
}

// doctorActionPhrase names what the run did to a path, reusing the phrases
// --verify prints so the two readings of one record cannot drift.
func doctorActionPhrase(action installAction) string {
	switch action {
	case installActionCreated:
		return "created"
	case installActionReplaced:
		return "replaced"
	case installActionBackedUp:
		return "backed up"
	case installActionFound:
		return "found"
	default:
		return "recorded"
	}
}

// doctorCheckDotfilesLinks follows every path the record names that is a
// symbolic link, and reports a link whose target is not there.
//
// A dangling link is the fault a naive check misses: `os.Stat` fails on it and
// so does the tool that reads it, while `os.Lstat` shows a link that looks
// perfectly present. The finding names the link and the target it points at, so
// the answer is actionable.
func doctorCheckDotfilesLinks(scan *doctorScan) []doctorFinding {
	links, reason := doctorManagedLinks(scan)
	if reason != "" {
		return []doctorFinding{{
			Subject:  "Symbolic links this installer manages",
			Status:   doctorNotApplicable,
			Evidence: reason,
			Impact:   "There is nothing to follow.",
			Advice:   "Nothing to do.",
		}}
	}
	if len(links) == 0 {
		return []doctorFinding{{
			Subject:  "Symbolic links this installer manages",
			Status:   doctorNotApplicable,
			Evidence: "none of the paths the record names, and none of the paths this installer writes, is a symbolic link",
			Impact:   "This machine's configuration is made of real files, which is what the installer writes.",
			Advice:   "Nothing to do.",
		}}
	}

	var findings []doctorFinding
	var good []string
	for _, link := range links {
		target, err := scan.readlink(link)
		if err != nil {
			findings = append(findings, doctorFinding{
				Subject:  doctorRelHome(scan, link),
				Status:   doctorWarning,
				Evidence: "the path is a symbolic link and its target could not be read: " + err.Error(),
				Impact:   "Nothing that reads this path can follow it.",
				Advice:   "Remove the link and let the installer write the file, or point it at the file you want.",
			})
			continue
		}
		resolved := target
		if !filepath.IsAbs(target) {
			resolved = filepath.Join(filepath.Dir(link), target)
		}
		if _, err := scan.stat(resolved); err != nil {
			status := doctorWarning
			impact := "The link is present and points at nothing, so the tool it configures reads a broken path: the configuration it names does not exist."
			if errors.Is(err, fs.ErrNotExist) {
				// A link into a directory that is not mounted (a removable volume,
				// an unmounted network share) reads the same way; the finding names
				// the target rather than claiming the file was deleted.
				impact += " The target may also live on a volume that is not mounted right now."
			}
			findings = append(findings, doctorFinding{
				Subject:  doctorRelHome(scan, link),
				Status:   status,
				Evidence: fmt.Sprintf("the link points at %s, which is not there (%v)", doctorRelHome(scan, resolved), err),
				Impact:   impact,
				Advice:   "Recreate the target, or replace the link with a real file: let the installer's step for this tool write it.",
			})
			continue
		}
		good = append(good, doctorRelHome(scan, link)+" → "+doctorRelHome(scan, resolved))
	}

	if len(good) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  "Symbolic links this installer manages",
			Status:   doctorOK,
			Evidence: doctorCounted(len(good), "link resolves", "links resolve") + ": " + doctorPathList(good, 4),
			Impact:   "Every link this installer manages leads somewhere the configuration can read.",
			Advice:   "Nothing to do.",
		})
	}
	return findings
}

// doctorManagedLinks collects the symbolic links this installer is answerable
// for: the paths the record names, and the paths this installer writes that
// resolve into a checkout of this repository.
//
// The second half is what makes an unrecorded machine useful: a user who linked
// the repository's files by hand has links under this installer's own config
// paths, and a dangling one among them is the fault the check exists for. A link
// elsewhere in the home directory is not collected -- that is somebody else's
// tree.
func doctorManagedLinks(scan *doctorScan) ([]string, string) {
	seen := map[string]bool{}
	var links []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		info, err := scan.lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return
		}
		seen[path] = true
		links = append(links, path)
	}

	read := doctorReadRecord(scan)
	for _, entry := range read.EntriesOrNothing() {
		add(entry.Path)
	}

	// The paths this installer writes, at one level: a directory among them is
	// not descended into, because the installer writes whole directories and a
	// link inside somebody's own file tree is not this check's subject.
	paths := system.ConfigPathsFor(scan.home())
	for _, key := range doctorSortedKeys(paths) {
		path := paths[key]
		if path == "" {
			continue
		}
		add(path)
		info, err := scan.lstat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		entries, err := scan.readDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			add(filepath.Join(path, entry.Name()))
		}
	}

	if len(links) == 0 && scan.home() == "" {
		return nil, "neither $HOME nor the platform's user lookup answered"
	}
	return links, ""
}

// EntriesOrNothing is the entries of a record, or nothing when there is no
// record. It exists so a caller that wants only the paths does not have to
// unpack the two failure cases it does not care about.
func (r doctorRecord) EntriesOrNothing() []installEntry {
	if r.record == nil {
		return nil
	}
	return r.record.Entries
}

// doctorCheckDotfilesPermissions looks for configuration the installer wrote
// that others can write. A file the installer created with the user's own mode
// is that user's business; a file anybody can rewrite is a way into every tool
// that reads it, and it is the one mode fault a user cannot see.
func doctorCheckDotfilesPermissions(scan *doctorScan) []doctorFinding {
	paths := system.ConfigPathsFor(scan.home())
	if scan.home() == "" {
		return []doctorFinding{{
			Subject:  "The permissions of the configuration files",
			Status:   doctorUnknown,
			Evidence: "neither $HOME nor the platform's user lookup answered",
			Impact:   "The files this installer writes cannot be looked at.",
			Advice:   "Set HOME and run the doctor again.",
		}}
	}

	var worldWritable []string
	var unreadable []string
	var checked []string
	for _, key := range doctorSortedKeys(paths) {
		path := paths[key]
		info, err := scan.lstat(path)
		if err != nil {
			continue
		}
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			// The mode of a link is the mode of its target on some platforms and
			// nothing at all on others; the link check is the one that reads it.
			continue
		case mode.Perm()&0o002 != 0:
			worldWritable = append(worldWritable, fmt.Sprintf("%s (mode %s)", doctorRelHome(scan, path), doctorPermString(mode)))
		case mode.Perm()&0o400 == 0 && !info.IsDir():
			unreadable = append(unreadable, fmt.Sprintf("%s (mode %s)", doctorRelHome(scan, path), doctorPermString(mode)))
		default:
			checked = append(checked, doctorRelHome(scan, path))
		}
	}

	var findings []doctorFinding
	if len(worldWritable) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  "Configuration this installer wrote that anybody can rewrite",
			Status:   doctorWarning,
			Evidence: doctorPathList(worldWritable, 4),
			Impact:   "Another user on this machine can change what every shell and editor reads, which is a way into every command run from this account.",
			Advice:   "Take the write bit from others: `chmod o-w <path>`.",
		})
	}
	if len(unreadable) > 0 {
		findings = append(findings, doctorFinding{
			Subject:  "Configuration this installer wrote that its owner cannot read",
			Status:   doctorWarning,
			Evidence: doctorPathList(unreadable, 4),
			Impact:   "The tool this file configures cannot read its own configuration, so it starts with the defaults instead.",
			Advice:   "Restore the read bit: `chmod u+r <path>`.",
		})
	}
	if len(findings) == 0 {
		if len(checked) == 0 {
			return []doctorFinding{{
				Subject:  "The permissions of the configuration files",
				Status:   doctorNotApplicable,
				Evidence: "none of the paths this installer writes is present",
				Impact:   "There is no mode of this installer's to check.",
				Advice:   "Nothing to do.",
			}}
		}
		return []doctorFinding{{
			Subject: "The permissions of the configuration files",
			Status:  doctorOK,
			Evidence: fmt.Sprintf("%s readable by their owner and writable only by their owner: %s",
				doctorCounted(len(checked), "path is", "paths are"), doctorPathList(checked, 5)),
			Impact: "No other user can change the configuration this installer wrote.",
			Advice: "Nothing to do.",
		}}
	}
	return findings
}

// doctorCheckDotfilesForeign names the configurations this installer would
// replace, and says plainly that they are not its own.
//
// This is the check the brief's "No asumir propiedad sobre archivos ajenos"
// turns into behaviour: the doctor does not call a user's own configuration a
// fault, and it does not offer to fix one. It states which paths of its own
// registry exist, that the record does not claim them, and what a future run
// would do about it (back them up and replace them).
func doctorCheckDotfilesForeign(scan *doctorScan) []doctorFinding {
	if scan.home() == "" {
		return []doctorFinding{{
			Subject:  "Configurations this installer does not own",
			Status:   doctorUnknown,
			Evidence: "neither $HOME nor the platform's user lookup answered",
			Impact:   "The paths to compare cannot be resolved.",
			Advice:   "Set HOME and run the doctor again.",
		}}
	}

	read := doctorReadRecord(scan)
	claimed := map[string]bool{}
	for _, entry := range read.EntriesOrNothing() {
		claimed[entry.Path] = true
	}

	var foreign []string
	for _, key := range doctorSortedKeys(system.ConfigPathsFor(scan.home())) {
		path := system.ConfigPathsFor(scan.home())[key]
		if path == "" || claimed[path] {
			continue
		}
		if _, err := scan.lstat(path); err != nil {
			continue
		}
		foreign = append(foreign, doctorRelHome(scan, path))
	}

	if len(foreign) == 0 {
		return []doctorFinding{{
			Subject:  "Configurations this installer does not own",
			Status:   doctorOK,
			Evidence: "every path of this installer's own registry that exists is named by the record of the last run",
			Impact:   "Nothing on this machine looks like a configuration this installer would replace without having written it.",
			Advice:   "Nothing to do.",
		}}
	}

	if read.absent {
		return []doctorFinding{{
			Subject:  fmt.Sprintf("%s this installer has not written", doctorCounted(len(foreign), "configuration", "configurations")),
			Status:   doctorNotApplicable,
			Evidence: fmt.Sprintf("there is no record of an install, and these paths of this installer's own registry exist: %s", doctorPathList(foreign, 5)),
			Impact:   "They are the user's own or another tool's, and this installer has not claimed them. Running the installer would copy them into a backup and then write its own.",
			Advice:   "Nothing to do now; if you want the installer's version, its backup step takes a copy first and the record names it.",
		}}
	}
	return []doctorFinding{{
		Subject:  fmt.Sprintf("%s the last run did not touch", doctorCounted(len(foreign), "configuration", "configurations")),
		Status:   doctorNotApplicable,
		Evidence: fmt.Sprintf("these paths of this installer's registry exist and the record does not name them: %s", doctorPathList(foreign, 5)),
		Impact:   "They belong to the user or to another tool. This installer neither owns nor repairs them; a later run that replaces one takes a backup first.",
		Advice:   "Nothing to do. The doctor reports them so the ownership is stated rather than assumed.",
	}}
}

// doctorWritable reports whether the scan's host can write a path, used by the
// checks that have to know before they claim a fix can create something. It
// answers about the path itself and, when the path does not exist, about the
// nearest directory that does.
func doctorWritable(scan *doctorScan, path string) bool {
	probe := path
	for {
		if _, err := scan.stat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe || parent == "." || parent == string(filepath.Separator) {
			return false
		}
		probe = parent
	}

	// A machine with no way to ask whether a directory is writable reports the
	// structural answer: the mode bits. It is the same rule the fixes use before
	// they offer themselves.
	if strings.HasPrefix(scan.goos(), "windows") {
		return true
	}
	info, err := scan.stat(probe)
	if err != nil {
		return false
	}
	return info.IsDir() && info.Mode().Perm()&0o200 != 0
}
