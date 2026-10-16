package system

import (
	"strings"
	"testing"
)

// wslMountInfoFixture is a mount table from a WSL2 distribution whose automount root was moved to
// /windir and whose project directory has a space in it. It is deliberately not the default /mnt
// layout: the module must classify the Windows drives wherever they are, so the fixture is the case
// a hardcoded /mnt/c would get wrong.
const wslMountInfoFixture = `24 30 0:22 / / rw,relatime shared:1 - ext4 /dev/sdc rw,discard
25 24 0:23 / /windir rw,relatime shared:2 - tmpfs tmpfs rw
26 25 0:25 / /windir/c rw,noatime shared:4 - 9p C:\134 rw,aname=drvfs
27 25 0:26 / /windir/d rw,noatime shared:5 - drvfs D:\ rw
28 24 0:27 / /mnt/wslg rw,relatime shared:6 - 9p none rw,aname=drvfs
29 24 0:28 / /home/ana/My\040Projects rw,relatime shared:7 - ext4 /dev/sdc rw
30 24 0:29 / /tmp rw,nosuid,nodev shared:8 - tmpfs tmpfs rw
31 24 0:30 / /mnt/ntfs rw,relatime shared:9 - ntfs3 /dev/sdd rw
32 24 0:31 / /run/user/1000/doc rw,nosuid shared:10 - fuse.portal
`

func parseFixture(t *testing.T, fixture string) MountTable {
	t.Helper()
	entries, err := ParseMountTable([]byte(fixture))
	if err != nil {
		t.Fatalf("ParseMountTable: %v", err)
	}
	return MountTable{Path: "fixture", Entries: entries}
}

func TestParseMountTableReadsPointsTypesAndSources(t *testing.T) {
	table := parseFixture(t, wslMountInfoFixture)

	if len(table.Entries) != 9 {
		t.Fatalf("parsed %d entries, want 9", len(table.Entries))
	}

	cases := []struct {
		index      int
		mountPoint string
		fileSystem string
		source     string
	}{
		{0, "/", "ext4", "/dev/sdc"},
		{1, "/windir", "tmpfs", "tmpfs"},
		// The source of a Windows drive is written with an escaped backslash in the file, and the
		// unescaping is what makes the row readable as evidence.
		{2, "/windir/c", "9p", `C:\`},
		{3, "/windir/d", "drvfs", `D:\`},
		{4, "/mnt/wslg", "9p", "none"},
		// A mount point with a space arrives as \040 and must come back as a space.
		{5, "/home/ana/My Projects", "ext4", "/dev/sdc"},
		{6, "/tmp", "tmpfs", "tmpfs"},
		{7, "/mnt/ntfs", "ntfs3", "/dev/sdd"},
		// A line whose fstype has no source and no options is still parsed: the fields after the
		// separator run out, and the entry keeps what was there.
		{8, "/run/user/1000/doc", "fuse.portal", ""},
	}
	for _, tc := range cases {
		got := table.Entries[tc.index]
		if got.MountPoint != tc.mountPoint {
			t.Errorf("entry %d mount point = %q, want %q", tc.index, got.MountPoint, tc.mountPoint)
		}
		if got.FileSystem != tc.fileSystem {
			t.Errorf("entry %d file system = %q, want %q", tc.index, got.FileSystem, tc.fileSystem)
		}
		if got.Source != tc.source {
			t.Errorf("entry %d source = %q, want %q", tc.index, got.Source, tc.source)
		}
		if got.Device == "" {
			t.Errorf("entry %d lost its major:minor device field", tc.index)
		}
	}
}

func TestParseMountTableRejectsAMalformedLineByNameAndNumber(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		want    string
	}{
		{
			name:    "too few fields",
			fixture: "24 30 0:22 /\n",
			want:    "line 1",
		},
		{
			name:    "no separator",
			fixture: "24 30 0:22 / / rw,relatime\n",
			want:    "no filesystem type",
		},
		{
			name:    "separator last",
			fixture: "24 30 0:22 / / rw,relatime -\n",
			want:    "no filesystem type",
		},
		{
			name:    "the second line is the broken one",
			fixture: "24 30 0:22 / / rw,relatime - ext4 /dev/sdc rw\nbroken\n",
			want:    "line 2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := ParseMountTable([]byte(tc.fixture))
			if err == nil {
				t.Fatalf("ParseMountTable accepted a malformed table: %+v", entries)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q", err, tc.want)
			}
		})
	}

	// A skipped mount would be a moved verdict, so the first broken line stops the parse rather than
	// returning the entries before it.
	if entries, err := ParseMountTable([]byte("broken\n24 30 0:22 / / rw - ext4 /dev/sdc rw\n")); err == nil || len(entries) != 0 {
		t.Errorf("a broken first line returned %d entries and error %v, want none and an error", len(entries), err)
	}
}

func TestMountTableResolveUsesTheLongestBoundary(t *testing.T) {
	table := parseFixture(t, `24 30 0:22 / / rw shared:1 - ext4 /dev/sdc rw
26 25 0:25 / /mnt/c rw shared:4 - 9p C:\ rw
27 25 0:26 / /mnt/cd rw shared:5 - 9p D:\ rw
`)

	cases := []struct {
		path       string
		wantPoint  string
		wantFS     string
		wantExists bool
	}{
		{"/mnt/c/projects/app", "/mnt/c", "9p", true},
		{"/mnt/c", "/mnt/c", "9p", true},
		{"/mnt/cd", "/mnt/cd", "9p", true},
		{"/mnt/cd/thing", "/mnt/cd", "9p", true},
		// The prefix bug this test exists for: /mnt/cd must not be answered by the mount at /mnt/c.
		{"/mnt/cde", "/", "ext4", true},
		{"/mnt/c/../c/x", "/mnt/c", "9p", true},
		{"/mnt/c/", "/mnt/c", "9p", true},
		{"/home/ana", "/", "ext4", true},
		{"/", "/", "ext4", true},
		{"", "", "", false},
	}
	for _, tc := range cases {
		got, ok := table.Resolve(tc.path)
		if ok != tc.wantExists {
			t.Errorf("Resolve(%q) found = %v, want %v", tc.path, ok, tc.wantExists)
			continue
		}
		if !ok {
			continue
		}
		if got.MountPoint != tc.wantPoint || got.FileSystem != tc.wantFS {
			t.Errorf("Resolve(%q) = %s on %s, want %s on %s",
				tc.path, got.FileSystem, got.MountPoint, tc.wantFS, tc.wantPoint)
		}
	}
}

func TestClassifyStorageKinds(t *testing.T) {
	table := parseFixture(t, wslMountInfoFixture)

	cases := []struct {
		path string
		want StorageKind
	}{
		// A Linux filesystem inside the distribution, wherever it is mounted.
		{"/home/ana/My Projects/app", StorageDistribution},
		{"/", StorageDistribution},
		// The Windows drives, wherever the automount root put them.
		{"/windir/c/Users/ana/app", StorageHost},
		{"/windir/d/work", StorageHost},
		// The interop runtime directory is served by the host too.
		{"/mnt/wslg", StorageHost},
		// A bare NTFS volume mounted in from the host.
		{"/mnt/ntfs/project", StorageHost},
		// Memory and the instance's own tmpfs.
		{"/windir", StorageVirtual},
		{"/tmp/scratch", StorageVirtual},
		// A fuse filesystem whose subtype this module does not classify is reported as unknown rather
		// than called "other local storage".
		{"/run/user/1000/doc/x", StorageUnknown},
	}
	for _, tc := range cases {
		got := ClassifyStorage(table, tc.path, tc.path)
		if got.Kind != tc.want {
			t.Errorf("ClassifyStorage(%q).Kind = %v (%s), want %v",
				tc.path, got.Kind, got.Kind, tc.want)
		}
		if !got.MountKnown {
			t.Errorf("ClassifyStorage(%q) found no mount", tc.path)
		}
		if got.ResolvedPath != tc.path {
			t.Errorf("ClassifyStorage(%q) resolved to %q, want the path unchanged", tc.path, got.ResolvedPath)
		}
	}
}

func TestClassifyStorageSaysWhenItCannotTell(t *testing.T) {
	t.Run("an unreadable table is not an empty table", func(t *testing.T) {
		got := ClassifyStorage(MountTable{Path: "/proc/self/mountinfo", Err: "open: permission denied"}, "/x", "/x")

		if got.Kind != StorageUnknown {
			t.Errorf("Kind = %v, want unknown", got.Kind)
		}
		if got.MountKnown {
			t.Error("MountKnown = true with no table")
		}
		if len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "permission denied") {
			t.Errorf("Notes = %v, want the reason the table could not be read", got.Notes)
		}
	})

	t.Run("an empty table says so", func(t *testing.T) {
		got := ClassifyStorage(MountTable{Path: "/proc/self/mountinfo"}, "/x", "/x")

		if got.Kind != StorageUnknown || len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "empty") {
			t.Errorf("verdict = %+v, want unknown with an empty-table note", got)
		}
	})

	t.Run("a path no mount covers says so", func(t *testing.T) {
		table := MountTable{Entries: []MountEntry{{MountPoint: "/somewhere", FileSystem: "ext4"}}}
		got := ClassifyStorage(table, "/elsewhere", "/elsewhere")

		if got.Kind != StorageUnknown || len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "No mount covers") {
			t.Errorf("verdict = %+v, want unknown with a no-mount note", got)
		}
	})

	t.Run("an unrecognised filesystem type is named", func(t *testing.T) {
		table := MountTable{Entries: []MountEntry{{MountPoint: "/", FileSystem: "weirdfs"}}}
		got := ClassifyStorage(table, "/x", "/x")

		// An unknown type is not silently filed as "other local storage": the verdict stays unknown
		// and the note names the type, so the reader can judge it.
		if got.Kind != StorageUnknown {
			t.Errorf("Kind = %v, want unknown for a filesystem type this module does not classify", got.Kind)
		}
		if len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "weirdfs") {
			t.Errorf("Notes = %v, want the type named", got.Notes)
		}
	})

	t.Run("a removable medium is classified as other local storage", func(t *testing.T) {
		table := MountTable{Entries: []MountEntry{{MountPoint: "/media/usb", FileSystem: "exfat"}}}
		got := ClassifyStorage(table, "/media/usb/app", "/media/usb/app")

		if got.Kind != StorageOther {
			t.Errorf("Kind = %v, want other for a real local filesystem that is neither side", got.Kind)
		}
		if len(got.Notes) != 0 {
			t.Errorf("Notes = %v, want none for a type the module knows and has no advice about", got.Notes)
		}
	})

	t.Run("a symlinked path is classified where it really is", func(t *testing.T) {
		table := parseFixture(t, wslMountInfoFixture)
		got := ClassifyStorage(table, "/home/ana/app", "/windir/c/Users/ana/app")

		if got.Kind != StorageHost {
			t.Errorf("Kind = %v, want the host classification of the resolved path", got.Kind)
		}
		if got.Path != "/home/ana/app" || got.ResolvedPath != "/windir/c/Users/ana/app" {
			t.Errorf("verdict = %+v, want both paths kept so the report can explain the link", got)
		}
	})
}

func TestStorageKindPredicatesAndLabels(t *testing.T) {
	cases := []struct {
		kind        StorageKind
		label       string
		hostBacked  bool
		nativeForIt bool
	}{
		{StorageUnknown, "unknown", false, false},
		{StorageDistribution, "inside the distribution", false, true},
		{StorageHost, "on the Windows host", true, false},
		{StorageVirtual, "in memory or an overlay", false, false},
		{StorageRemote, "over the network", false, false},
		{StorageOther, "on other local storage", false, false},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.label {
			t.Errorf("StorageKind(%d).String() = %q, want %q", tc.kind, got, tc.label)
		}
		if got := tc.kind.HostBacked(); got != tc.hostBacked {
			t.Errorf("%v.HostBacked() = %v, want %v", tc.kind, got, tc.hostBacked)
		}
		if got := tc.kind.NativeForTooling(); got != tc.nativeForIt {
			t.Errorf("%v.NativeForTooling() = %v, want %v", tc.kind, got, tc.nativeForIt)
		}
	}
}

// TestFileSystemTablesDoNotOverlap is the guard for the class of defect a table like this produces:
// the same type listed twice, in two tables, so which answer a reader gets depends on the order the
// switch happens to test them in.
func TestFileSystemTablesDoNotOverlap(t *testing.T) {
	tables := map[string]map[string]bool{
		"distribution": distributionFileSystems,
		"host":         hostFileSystems,
		"virtual":      virtualFileSystems,
		"remote":       remoteFileSystems,
	}
	seen := map[string]string{}
	for name, table := range tables {
		if len(table) == 0 {
			t.Fatalf("the %s table is empty, so this guard proves nothing", name)
		}
		for fileSystem := range table {
			if fileSystem != strings.ToLower(fileSystem) {
				t.Errorf("the %s table lists %q, which is not lowercase", name, fileSystem)
			}
			if previous, ok := seen[fileSystem]; ok {
				t.Errorf("%q is listed as both %s and %s; one answer must win by name, not by order",
					fileSystem, previous, name)
			}
			seen[fileSystem] = name
		}
	}

	// The classifications the module promises, checked through the function a caller uses: a type
	// listed in a table but classified differently would be silent otherwise.
	promised := map[string]StorageKind{
		"ext4": StorageDistribution, "btrfs": StorageDistribution,
		"drvfs": StorageHost, "9p": StorageHost, "virtiofs": StorageHost, "ntfs3": StorageHost,
		"tmpfs": StorageVirtual, "overlay": StorageVirtual,
		"nfs": StorageRemote, "fuse.sshfs": StorageRemote, "fuse.drvfs": StorageHost,
	}
	for fileSystem, want := range promised {
		if got := storageKindOf(fileSystem); got != want {
			t.Errorf("storageKindOf(%q) = %v, want %v", fileSystem, got, want)
		}
	}
	if got := storageKindOf(""); got != StorageUnknown {
		t.Errorf("storageKindOf(empty) = %v, want unknown", got)
	}
	if got := storageKindOf("EXT4"); got != StorageDistribution {
		t.Errorf("storageKindOf(\"EXT4\") = %v, want it read case-insensitively", got)
	}
}

func TestNativeRootFindsWhereTheDistributionLives(t *testing.T) {
	table := parseFixture(t, wslMountInfoFixture)
	got, ok := table.NativeRoot()
	if !ok {
		t.Fatal("NativeRoot found no root mount")
	}
	if got.MountPoint != "/" || got.FileSystem != "ext4" {
		t.Errorf("NativeRoot = %s on %s, want ext4 on /", got.FileSystem, got.MountPoint)
	}

	if _, ok := (MountTable{Err: "not readable"}).NativeRoot(); ok {
		t.Error("NativeRoot claimed a root mount for an unreadable table")
	}
	if _, ok := (MountTable{}).NativeRoot(); ok {
		t.Error("NativeRoot claimed a root mount for a table with no entries")
	}
}

func TestReadMountTableFromMissingFileReportsTheReason(t *testing.T) {
	table := ReadMountTableFrom("/nonexistent/dotfiles-mountinfo-test")

	if table.Err == "" {
		t.Fatal("a missing mount table was read as an empty one")
	}
	if len(table.Entries) != 0 {
		t.Errorf("Entries = %v, want none", table.Entries)
	}
	if table.Path != "/nonexistent/dotfiles-mountinfo-test" {
		t.Errorf("Path = %q, want the path that was read", table.Path)
	}
}

// TestStorageSpaceReadsARealDirectory checks the one probe that touches the machine, on a directory
// the test owns: a temporary directory the test created is a real path on a real filesystem.
func TestStorageSpaceReadsARealDirectory(t *testing.T) {
	dir := t.TempDir()
	free, total, ok := StorageSpace(dir)
	if !ok {
		t.Fatalf("StorageSpace(%q) could not read the filesystem", dir)
	}
	if total == 0 {
		t.Errorf("StorageSpace(%q) reported a total of zero", dir)
	}
	if free > total {
		t.Errorf("StorageSpace(%q) reported %d free of %d total", dir, free, total)
	}

	if _, _, ok := StorageSpace("/nonexistent/dotfiles-space-test"); ok {
		t.Error("StorageSpace reported success for a path that does not exist")
	}
	if _, _, ok := StorageSpace(""); ok {
		t.Error("StorageSpace reported success for an empty path")
	}
}

func TestParseWSLConf(t *testing.T) {
	conf := ParseWSLConf("/etc/wsl.conf", []byte(`# WSL settings
[automount]
root = /windir
enabled = true
[interop]
enabled = false # the host tools are not wanted here
[boot]
systemd = true
[network]
generateResolvConf = false
[user]
default = ana
unknown = value
`))

	if !conf.Read {
		t.Fatal("Read = false for a file that was parsed")
	}
	if got := conf.AutomountRoot.Value; got != "/windir" {
		t.Errorf("AutomountRoot = %q, want /windir", got)
	}
	if got := conf.AutomountEnabled.Value; got != "true" {
		t.Errorf("AutomountEnabled = %q, want true", got)
	}
	if got := conf.InteropEnabled.Value; got != "false" {
		t.Errorf("InteropEnabled = %q, want false (the inline comment must be stripped)", got)
	}
	if got := conf.Systemd.Value; got != "true" {
		t.Errorf("Systemd = %q, want true", got)
	}
	if got := conf.GenerateResolvConf.Value; got != "false" {
		t.Errorf("GenerateResolvConf = %q, want false", got)
	}
	if len(conf.Notes) != 0 {
		t.Errorf("Notes = %v, want none for a file this parser understands", conf.Notes)
	}
}

func TestParseWSLConfKeepsTheDifferenceBetweenAbsentAndSet(t *testing.T) {
	conf := ParseWSLConf("/etc/wsl.conf", []byte("[boot]\nsystemd = false\n"))

	if conf.AutomountRoot.Set {
		t.Error("AutomountRoot.Set = true for a key the file does not set")
	}
	if !conf.Systemd.Set || conf.Systemd.Value != "false" {
		t.Errorf("Systemd = %+v, want the value false and Set", conf.Systemd)
	}
	if got := conf.Describe(conf.AutomountRoot, "/mnt"); !strings.Contains(got, "default is /mnt") {
		t.Errorf("Describe(absent) = %q, want the documented default named", got)
	}
	if got := conf.Describe(conf.Systemd, ""); !strings.Contains(got, "set in /etc/wsl.conf") {
		t.Errorf("Describe(set) = %q, want it to say the file set it", got)
	}
	if got := (WSLConf{}).Describe(WSLConfValue{}, ""); !strings.Contains(got, "/etc/wsl.conf") {
		t.Errorf("Describe with no path = %q, want the default path named", got)
	}
}

func TestParseWSLConfHandlesQuotesCommentsAndJunk(t *testing.T) {
	conf := ParseWSLConf("/etc/wsl.conf", []byte(strings.Join([]string{
		"[automount]",
		`root = "/mnt/with # hash"`,
		"; a whole-line comment",
		"not a key value line",
		"[interop]",
		"enabled = maybe",
		"[unknown-section]",
		"enabled = true",
		"",
	}, "\r\n")))

	if got := conf.AutomountRoot.Value; got != "/mnt/with # hash" {
		t.Errorf("AutomountRoot = %q, want the quoted value with its hash kept", got)
	}
	if len(conf.Notes) != 1 || !strings.Contains(conf.Notes[0], "not a key value line") {
		t.Errorf("Notes = %v, want the unparsable line recorded", conf.Notes)
	}
	if value, ok := conf.InteropEnabled.FlagOn(); ok || value {
		t.Errorf("FlagOn(maybe) = (%v, %v), want (false, false) rather than a guess", value, ok)
	}
}

func TestWSLConfValueFlagOnAndString(t *testing.T) {
	cases := []struct {
		value      string
		wantFlag   bool
		wantParsed bool
		wantText   string
	}{
		{"true", true, true, "true"},
		{"TRUE", true, true, "TRUE"},
		{"yes", true, true, "yes"},
		{"1", true, true, "1"},
		{"on", true, true, "on"},
		{"false", false, true, "false"},
		{"no", false, true, "no"},
		{"0", false, true, "0"},
		{"off", false, true, "off"},
		// A value WSL does not define must be reported as unparsed rather than rounded to a meaning.
		{"maybe", false, false, "maybe"},
	}
	for _, tc := range cases {
		setting := WSLConfValue{Value: tc.value, Set: true}
		got, parsed := setting.FlagOn()
		if parsed != tc.wantParsed || got != tc.wantFlag {
			t.Errorf("FlagOn(%q) = (%v, %v), want (%v, %v)",
				tc.value, got, parsed, tc.wantFlag, tc.wantParsed)
		}
		if rendered := setting.String(); rendered != tc.wantText {
			t.Errorf("String(%q) = %q, want %q", tc.value, rendered, tc.wantText)
		}
	}

	if got, ok := (WSLConfValue{}).FlagOn(); ok || got {
		t.Error("an unset setting must not report a value")
	}
	if got := (WSLConfValue{}).String(); got != "not set" {
		t.Errorf("String(unset) = %q, want \"not set\"", got)
	}
	if got := (WSLConfValue{Set: true}).String(); got != "set to an empty value" {
		t.Errorf("String(empty) = %q, want it to say the value is empty", got)
	}
}

func TestReadWSLConfFromMissingFileIsNotAnError(t *testing.T) {
	conf := ReadWSLConfFrom("")

	if conf.Err == "" {
		t.Error("an empty path must be reported instead of reading a random file")
	}
	if conf.Read {
		t.Error("Read = true with no path")
	}

	conf = ReadWSLConfFrom("/nonexistent/dotfiles-wslconf-test")
	if conf.Err != "" {
		t.Errorf("Err = %q, want an absent file to be a normal state", conf.Err)
	}
	if conf.Read {
		t.Error("Read = true for a file that is not there")
	}
	if conf.Path != "/nonexistent/dotfiles-wslconf-test" {
		t.Errorf("Path = %q, want the path that was looked for", conf.Path)
	}
}
