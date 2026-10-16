package system

import (
	"fmt"
	"os"
	"path/filepath"

	"strings"
)

// ============================================================================
// WHERE A PROJECT ACTUALLY LIVES, READ FROM THE MOUNT TABLE
// ============================================================================
//
// The difference this module exists for: a Linux filesystem inside the distribution costs one
// syscall per operation, while a Windows drive mounted into the distribution costs a round trip
// through the host for the same call. Git stats every tracked file, a package manager stats a whole
// dependency tree, a build walks a source tree and a watcher polls all of it, so the difference is
// paid per file rather than per project.
//
// The question "is this path on the host drive?" is answered from /proc/self/mountinfo and from the
// filesystem types alone. There is deliberately **no /mnt/c in this file**: `[automount] root` may
// put the drives anywhere (a common setting is /windir), a distribution may run with automount off,
// a drive may be added later with `wsl --mount`, and a mount point may contain spaces -- all of
// which the mount table describes and a hardcoded prefix does not. The mount point is therefore
// never recognised by its path; only the filesystem type is classified, and the path is used to find
// which mount it belongs to.

// MountEntry is one line of /proc/self/mountinfo, reduced to the fields this module reads.
//
// The format is: mount ID, parent ID, major:minor, root, mount point, options, optional fields, "-",
// filesystem type, source, super options. The separator matters because the optional fields vary in
// number between kernels.
type MountEntry struct {
	// MountPoint is where the filesystem is visible, unescaped. It may contain spaces: mountinfo
	// writes a space as \040, a tab as \011, a newline as \012 and a backslash as \134.
	MountPoint string
	// FileSystem is the type the kernel reports, e.g. "ext4", "drvfs", "9p" or "virtiofs". This is the
	// field the whole classification rests on.
	FileSystem string
	// Source is the device or the host's own name for it, e.g. "/dev/sdc", "C:\" or "drvfs". It is
	// shown as evidence and never parsed for meaning.
	Source string
	// Device is the major:minor field, which is what makes two mounts comparable when the same
	// filesystem appears twice.
	Device string
	// Root is the path within the filesystem that the mount exposes.
	Root string
	// Options are the per-mount options, e.g. "rw,noatime".
	Options string
}

// MountTable is the mount table as it was read, including the reason it could not be read. A table
// with an empty Entries and a set Err is not "no mounts": it is "the mount table is not readable
// here", which is a different sentence and the only honest one on a host without /proc.
type MountTable struct {
	// Path is the file the table was read from.
	Path string
	// Entries are the parsed mounts, in the file's own order.
	Entries []MountEntry
	// Err is why the table could not be read, empty when it could.
	Err string
}

// ReadMountTable reads this process's own mount table. /proc/self/mountinfo is used rather than
// /proc/mounts because it is the one the kernel guarantees to unescape into a single line per mount
// and because it carries the mount's root and device, which /proc/mounts does not.
func ReadMountTable() MountTable {
	return ReadMountTableFrom("/proc/self/mountinfo")
}

// ReadMountTableFrom reads a mount table from a path, so a fixture can be used instead of the real
// one. The path is a parameter rather than a variable for the same reason.
func ReadMountTableFrom(path string) MountTable {
	table := MountTable{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		table.Err = err.Error()
		return table
	}
	entries, err := ParseMountTable(data)
	if err != nil {
		table.Err = err.Error()
		return table
	}
	table.Entries = entries
	return table
}

// ParseMountTable parses the contents of a mountinfo file. A line it cannot parse is an error
// rather than a silently skipped mount: a skipped mount is a moved verdict, and this module's whole
// output is a verdict about storage.
func ParseMountTable(data []byte) ([]MountEntry, error) {
	var entries []MountEntry
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		entry, err := parseMountInfoLine(fields)
		if err != nil {
			return nil, fmt.Errorf("mountinfo line %d: %w", i+1, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseMountInfoLine reads one already-split mountinfo line. Field 4 is the mount point, and the
// filesystem type is the field after the "-" separator.
func parseMountInfoLine(fields []string) (MountEntry, error) {
	if len(fields) < 6 {
		return MountEntry{}, fmt.Errorf("only %d fields", len(fields))
	}
	separator := -1
	for i := 6; i < len(fields); i++ {
		if fields[i] == "-" {
			separator = i
			break
		}
	}
	if separator < 0 || separator+1 >= len(fields) {
		return MountEntry{}, fmt.Errorf("no filesystem type after the separator")
	}
	entry := MountEntry{
		Device:     fields[2],
		Root:       unescapeMountField(fields[3]),
		MountPoint: unescapeMountField(fields[4]),
		Options:    fields[5],
		FileSystem: fields[separator+1],
	}
	// The source is optional in the format: a pseudo-filesystem line can end at the type, and reading
	// past the end of a slice is how a parser turns an unusual host into a crash instead of a row.
	if separator+2 < len(fields) {
		entry.Source = unescapeMountField(fields[separator+2])
	}
	return entry, nil
}

// unescapeMountField turns the kernel's octal escapes back into the characters they stand for. Only
// the four the kernel documents are decoded: anything else is left as written rather than guessed
// at, because a mangled mount point would classify the wrong path.
func unescapeMountField(field string) string {
	if !strings.Contains(field, "\\") {
		return field
	}
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(field)
}

// ResolveMount finds the filesystem a path belongs to: the entry with the longest mount point that
// is a prefix of the path. The comparison is on path boundaries, so "/mnt/cd" is not answered by a
// mount at "/mnt/c" -- the classic prefix bug, and the reason this is a function with its own test
// instead of a strings.HasPrefix at the call site.
//
// The path is cleaned first, so "/mnt/c/../d" and a trailing slash answer the same way. A relative
// path is not made absolute here: the caller passes the path it means, and an absolute path is what
// the caller has.
func (t MountTable) Resolve(path string) (MountEntry, bool) {
	if path == "" {
		return MountEntry{}, false
	}
	target := filepath.Clean(path)
	best := -1
	bestLen := -1
	for i, entry := range t.Entries {
		mountPoint := filepath.Clean(entry.MountPoint)
		if mountPoint == "" {
			continue
		}
		if !pathWithin(target, mountPoint) {
			continue
		}
		if len(mountPoint) > bestLen {
			best, bestLen = i, len(mountPoint)
		}
	}
	if best < 0 {
		return MountEntry{}, false
	}
	return t.Entries[best], true
}

// pathWithin reports whether target is the mount point itself or lives under it. The "/" case is
// spelled out because the root mount has no trailing slash to compare against.
func pathWithin(target, mountPoint string) bool {
	if mountPoint == "/" {
		return strings.HasPrefix(target, "/")
	}
	if target == mountPoint {
		return true
	}
	return strings.HasPrefix(target, mountPoint+"/")
}

// StorageKind is what a path's filesystem means for Linux tooling.
type StorageKind int

const (
	// StorageUnknown is the honest answer when no mount matched or the table could not be read. It is
	// not a synonym for "fine": nothing may be recommended from it.
	StorageUnknown StorageKind = iota
	// StorageDistribution is a Linux filesystem inside the distribution. Git, package managers and
	// builds behave as they do on any Linux host.
	StorageDistribution
	// StorageHost is a filesystem the Windows host exposes to the distribution. Every metadata
	// operation crosses the boundary.
	StorageHost
	// StorageVirtual is a filesystem that does not survive a restart or is not writable in place:
	// tmpfs, overlay, ramfs, squashfs. A project here is not where its author thinks it is.
	StorageVirtual
	// StorageRemote is a filesystem over the network (cifs, sshfs, nfs): the cost is the network, and
	// the host-versus-distribution question does not apply.
	StorageRemote
	// StorageOther is a real local filesystem this module has no opinion about, e.g. a removable
	// medium. It is a real place to keep a project; it is just not classified as either side.
	StorageOther
)

// StorageKind classifies the filesystem this mount exposes. It is the same decision
// ClassifyStorage makes, exposed on the entry so a report that has already resolved a mount does not
// have to rebuild a one-entry table to ask what it is.
func (e MountEntry) StorageKind() StorageKind { return storageKindOf(e.FileSystem) }

// String names the kind the way a report says it.
func (k StorageKind) String() string {
	switch k {
	case StorageDistribution:
		return "inside the distribution"
	case StorageHost:
		return "on the Windows host"
	case StorageVirtual:
		return "in memory or an overlay"
	case StorageRemote:
		return "over the network"
	case StorageOther:
		return "on other local storage"
	default:
		return "unknown"
	}
}

// HostBacked reports whether the Windows host mediates this storage. It is the one kind the advisor
// warns about, so it is answered in one place.
func (k StorageKind) HostBacked() bool { return k == StorageHost }

// NativeForTooling reports whether Linux tooling has the cost it expects here. Only the
// distribution's own filesystem qualifies: a remote or removable one has its own cost, and this
// module's advice is about the host boundary, not about those.
func (k StorageKind) NativeForTooling() bool { return k == StorageDistribution }

// The filesystem types, by the meaning they have for this module. They are tables rather than a
// switch so a test can enumerate them, and so a report can name the type it classified.
var (
	// distributionFileSystems are the Linux filesystems a distribution's own storage can be. The
	// presence of one of these is not a promise about performance; it is the absence of the host
	// boundary, which is the only claim this module makes.
	distributionFileSystems = map[string]bool{
		"ext2": true, "ext3": true, "ext4": true, "btrfs": true, "xfs": true,
		"zfs": true, "f2fs": true, "jfs": true, "bcachefs": true, "nilfs2": true,
		"reiserfs": true, "ufs": true, "minix": true,
	}

	// hostFileSystems are the types the Windows side exposes into the distribution. drvfs is what
	// WSL1 and WSL2 mount a Windows drive with; 9p is the WSL2 file server that serves the Windows
	// filesystem, the driver store and the interop runtime directory; virtiofs is the newer
	// share WSL uses for the same purpose; cifs and ntfs3 are a Windows share or a bare NTFS volume
	// mounted in from the host.
	hostFileSystems = map[string]bool{
		"drvfs": true, "9p": true, "virtiofs": true, "cifs": true, "smb3": true,
		"ntfs": true, "ntfs3": true, "fuseblk": true, "v9fs": true,
	}

	// virtualFileSystems do not hold a project: tmpfs and ramfs are memory, overlay and
	// overlayfs are a container's view, squashfs is read-only.
	virtualFileSystems = map[string]bool{
		"tmpfs": true, "ramfs": true, "overlay": true, "overlayfs": true,
		"squashfs": true, "devtmpfs": true, "efivarfs": true, "autofs": true,
	}

	// otherFileSystems are real, local filesystems that are neither the distribution's own nor the
	// host's: a removable medium, an optical disc, a phone's share. A project can live on one; it is
	// simply not what this module's advice is about.
	otherFileSystems = map[string]bool{
		"vfat": true, "exfat": true, "msdos": true, "iso9660": true, "udf": true,
		"hfs": true, "hfsplus": true, "apfs": true,
	}

	// remoteFileSystems are real filesystems reached over a network. cifs is deliberately not here:
	// it is listed with the host types, because on WSL a Windows share is reached through the same
	// boundary the drives are, and that is the boundary this module warns about.
	remoteFileSystems = map[string]bool{
		"nfs": true, "nfs4": true, "nfsd": true, "sshfs": true,
	}
)

// storageKindOf classifies a filesystem type. The map lookups are ordered by specificity: a type
// listed as host-backed is host-backed even when it could also be described otherwise.
func storageKindOf(fileSystem string) StorageKind {
	normalised := strings.ToLower(strings.TrimSpace(fileSystem))
	// A subtype such as "fuse.sshfs" or "fuse.drvfs" spells the family first.
	if base, sub, found := strings.Cut(normalised, "."); found && base == "fuse" {
		switch sub {
		case "drvfs", "ntfs-3g", "fuseblk":
			return StorageHost
		case "sshfs", "nfs":
			return StorageRemote
		default:
			// An unlisted fuse filesystem is still a filesystem, but not one with a meaning known here;
			// the caller reports the type so the reader can judge it.
			return StorageUnknown
		}
	}
	switch {
	case normalised == "":
		return StorageUnknown
	case hostFileSystems[normalised]:
		return StorageHost
	case distributionFileSystems[normalised]:
		return StorageDistribution
	case virtualFileSystems[normalised]:
		return StorageVirtual
	case remoteFileSystems[normalised]:
		return StorageRemote
	case otherFileSystems[normalised]:
		return StorageOther
	default:
		// A type in no table is unknown, not "other local storage": this module cannot call a
		// filesystem local just because it has not heard of it.
		return StorageUnknown
	}
}

// StorageVerdict is what this module can say about one path: the mount it lives on, what kind of
// storage that is and how much room is left. Notes carries every reason the answer is narrower than
// it looks -- an unreadable mount table, a symlink that could not be followed, a path that does not
// exist -- so nothing is inferred from a probe that did not run.
type StorageVerdict struct {
	// Path is the path as the caller gave it.
	Path string
	// ResolvedPath is the path after following symlinks, equal to Path when it did not move.
	ResolvedPath string
	// Kind is the classification of the filesystem the path lives on.
	Kind StorageKind
	// Mount is the mount the path resolved into, zero when none matched.
	Mount MountEntry
	// MountKnown reports whether a mount matched.
	MountKnown bool
	// FreeBytes and TotalBytes are the space behind the path, zero when it could not be read.
	FreeBytes  uint64
	TotalBytes uint64
	// SpaceKnown reports whether the space reading succeeded.
	SpaceKnown bool
	// Notes are the limits of this verdict, in the report's own words.
	Notes []string
}

// IsHostBacked is the one question most callers ask.
func (v StorageVerdict) IsHostBacked() bool { return v.Kind.HostBacked() }

// ClassifyStorage answers where a path lives, using an already resolved path so the function itself
// stays pure: following a symlink is filesystem work, and the caller does it on its own thread.
func ClassifyStorage(table MountTable, path, resolvedPath string) StorageVerdict {
	verdict := StorageVerdict{Path: path, ResolvedPath: resolvedPath, Kind: StorageUnknown}
	if verdict.ResolvedPath == "" {
		verdict.ResolvedPath = path
	}

	switch {
	case table.Err != "":
		verdict.Notes = append(verdict.Notes, "The mount table could not be read ("+table.Err+
			"), so the filesystem behind this path was not identified.")
		return verdict
	case len(table.Entries) == 0:
		verdict.Notes = append(verdict.Notes, "The mount table is empty, so the filesystem behind "+
			"this path was not identified.")
		return verdict
	}

	entry, ok := table.Resolve(verdict.ResolvedPath)
	if !ok {
		verdict.Notes = append(verdict.Notes, "No mount covers this path, so the filesystem behind "+
			"it was not identified.")
		return verdict
	}
	verdict.Mount = entry
	verdict.MountKnown = true
	verdict.Kind = storageKindOf(entry.FileSystem)
	if verdict.Kind == StorageUnknown {
		verdict.Notes = append(verdict.Notes, "The filesystem type "+entry.FileSystem+
			" is not one this module can classify.")
	}

	return verdict
}

// NativeRoot returns the mount the distribution's own root directory lives on, which is what "the
// distribution's native storage" means. It reports false when the table is unreadable or the root
// has no entry -- a chroot or a fixture -- rather than assuming "/" is fine.
func (t MountTable) NativeRoot() (MountEntry, bool) {
	if t.Err != "" {
		return MountEntry{}, false
	}
	if entry, ok := t.Resolve("/"); ok {
		return entry, true
	}
	return MountEntry{}, false
}

// StorageSpace reads how much room is left behind a path. It reports false rather than zeroes when
// the path does not exist or the call fails, so a report never prints "0 bytes free" for a question
// it could not ask -- the difference between a full disk and an unreadable one is the whole point of
// the row.
func StorageSpace(path string) (free, total uint64, ok bool) {
	if path == "" {
		return 0, 0, false
	}
	return statfsFree(path)
}

// ============================================================================
// /etc/wsl.conf
// ============================================================================

// WSLConfValue is one setting from /etc/wsl.conf: the value and whether the file sets it at all.
// The distinction is the point: WSL's defaults and a distribution's own choice are different facts,
// and a report that cannot tell them apart is guessing about the host.
type WSLConfValue struct {
	// Value is the value as written, with the comment removed and the quotes trimmed.
	Value string
	// Set reports whether the file set this key.
	Set bool
}

// WSLConf is the part of /etc/wsl.conf this module reads. It is a small INI file with four sections
// that matter here: where the Windows drives are mounted, whether interop and automount are on, and
// whether systemd starts the distribution.
type WSLConf struct {
	// Path is the file that was read.
	Path string
	// Read reports whether the file exists and was read. A distribution with no /etc/wsl.conf runs on
	// WSL's defaults, which is a normal state and not a failure.
	Read bool
	// Err is why the file could not be read, empty when it was read or absent.
	Err string

	// AutomountRoot is [automount] root: where the Windows drives are mounted.
	AutomountRoot WSLConfValue
	// AutomountEnabled is [automount] enabled.
	AutomountEnabled WSLConfValue
	// InteropEnabled is [interop] enabled.
	InteropEnabled WSLConfValue
	// Systemd is [boot] systemd: whether the distribution boots systemd.
	Systemd WSLConfValue
	// GenerateResolvConf is [network] generateResolvConf: whether WSL writes /etc/resolv.conf.
	GenerateResolvConf WSLConfValue
	// Notes are lines this parser skipped because it could not make sense of them, so a file it
	// partly understood says so instead of reporting defaults as facts.
	Notes []string
}

// ParseWSLConf parses /etc/wsl.conf. Keys are read case-insensitively, section by section, and an
// unrecognised key is kept out of the report rather than guessed at; a line that is neither a
// section, a key nor a comment is recorded in Notes.
func ParseWSLConf(path string, data []byte) WSLConf {
	conf := WSLConf{Path: path, Read: true}
	section := ""

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			conf.Notes = append(conf.Notes, "Skipped a line without a key and a value: "+line)
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = stripWSLConfComment(strings.TrimSpace(value))
		value = strings.Trim(value, `"'`)

		switch section {
		case "automount":
			switch key {
			case "root":
				conf.AutomountRoot = WSLConfValue{Value: value, Set: true}
			case "enabled":
				conf.AutomountEnabled = WSLConfValue{Value: value, Set: true}
			}
		case "interop":
			if key == "enabled" {
				conf.InteropEnabled = WSLConfValue{Value: value, Set: true}
			}
		case "boot":
			if key == "systemd" {
				conf.Systemd = WSLConfValue{Value: value, Set: true}
			}
		case "network":
			if key == "generateresolvconf" {
				conf.GenerateResolvConf = WSLConfValue{Value: value, Set: true}
			}
		}
	}

	return conf
}

// ReadWSLConfFrom reads a wsl.conf file. A missing file is not an error: it means the distribution
// runs on WSL's documented defaults, and reporting a default as a fact is exactly what the Set flag
// prevents. The path is the caller's, because the installer already lets a test or a distribution
// move that file with its own environment override, and this module must read the same one.
func ReadWSLConfFrom(path string) WSLConf {
	if path == "" {
		return WSLConf{Err: "no path to read"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WSLConf{Path: path}
		}
		return WSLConf{Path: path, Err: err.Error()}
	}
	return ParseWSLConf(path, data)
}

// stripWSLConfComment removes an inline comment from a value. A "#" inside a quoted value is part of
// the value, which matters because a mount root is a path and a path may contain one.
func stripWSLConfComment(value string) string {
	quote := byte(0)
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' || c == ';':
			return strings.TrimSpace(value[:i])
		}
	}
	return value
}

// FlagOn reads a boolean WSL setting. It reports the value and whether the setting was there at all,
// so a caller can apply WSL's own default instead of inventing one: a key that is absent and a key
// that is set to false are different states of the world.
func (v WSLConfValue) FlagOn() (value, set bool) {
	if !v.Set {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(v.Value)) {
	case "true", "yes", "1", "on":
		return true, true
	case "false", "no", "0", "off":
		return false, true
	default:
		return false, false
	}
}

// String renders a setting the way a report row shows it: the value, or that the file does not set
// it, with the raw value quoted when it is not a flag.
func (v WSLConfValue) String() string {
	if !v.Set {
		return "not set"
	}
	if v.Value == "" {
		return "set to an empty value"
	}
	return v.Value
}

// Describe is the report's own wording for a setting: the value and whether it came from the file
// or from WSL's documented default, which is the difference between a measured fact and a documented
// one. The path in the wording is the file that was read, so a distribution that moved it says where
// the setting was looked for.
func (c WSLConf) Describe(v WSLConfValue, documentedDefault string) string {
	path := c.Path
	if path == "" {
		path = "/etc/wsl.conf"
	}
	if !v.Set {
		if documentedDefault == "" {
			return "not set in " + path
		}
		return "not set in " + path + " (WSL's default is " + documentedDefault + ")"
	}
	return v.Value + " (set in " + path + ")"
}
