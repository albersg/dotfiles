package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// wslAdvisorMountTableFixture is a distribution whose automount root was moved to /windir: the
// Windows drives are at /windir/c and /windir/d, and there is a native ext4 root. Every test that
// needs a mount table uses this one, so a finding's evidence can be checked against mount points a
// hardcoded /mnt/c would have got wrong.
func wslAdvisorMountTableFixture(t *testing.T) system.MountTable {
	t.Helper()
	entries, err := system.ParseMountTable([]byte(`24 30 0:22 / / rw,relatime shared:1 - ext4 /dev/sdc rw,discard
26 25 0:25 / /windir/c rw,noatime shared:4 - 9p C:\ rw,aname=drvfs
31 24 0:30 / /mnt/disk rw,relatime shared:9 - ext4 /dev/sdd rw
`))
	if err != nil {
		t.Fatalf("ParseMountTable: %v", err)
	}
	return system.MountTable{Path: "/proc/self/mountinfo", Entries: entries}
}

// healthyWSLLinuxFacts is a distribution with nothing wrong with it: the working directory is on the
// native filesystem, memory and swap are comfortable, the disk has room, the resolver goes through
// the host as WSL documents, interop is registered, and the .wslconfig values match what the
// instance has. It is the baseline every finding test varies one thing from.
func healthyWSLLinuxFacts(t *testing.T) wslAdvisorFacts {
	t.Helper()
	table := wslAdvisorMountTableFixture(t)
	return wslAdvisorFacts{
		Runtime: system.WSLRuntime{
			Inside: true, Version: 2, KernelMarked: true, Distro: "Ubuntu-24.04",
			Signals: []system.WSLSignal{
				{Name: "kernel-release", Detail: "5.15.153.1-microsoft-standard-WSL2", Strength: system.WSLSignalKernel, Found: true},
				{Name: "interop-binfmt", Detail: "/proc/sys/fs/binfmt_misc/WSLInterop", Strength: system.WSLSignalInstance, Found: true},
				{Name: "distro-name", Detail: "WSL_DISTRO_NAME", Strength: system.WSLSignalEnvironment, Found: true},
			},
		},
		Cwd:   "/home/ana/app",
		Home:  "/home/ana",
		Table: table,
		Conf: system.WSLConf{
			Path:               "/etc/wsl.conf",
			Read:               true,
			AutomountRoot:      system.WSLConfValue{Value: "/windir", Set: true},
			InteropEnabled:     system.WSLConfValue{Value: "true", Set: true},
			GenerateResolvConf: system.WSLConfValue{Value: "true", Set: true},
		},
		LocalProject: wslProjectFacts{
			Path: "/home/ana/app", Label: "~/app", GitRepo: true,
			DependencyDirs: []string{"node_modules"},
			Storage:        system.ClassifyStorage(table, "/home/ana/app", "/home/ana/app"),
		},
		Memory: wslMemoryFacts{
			Read: true, HaveTotal: true, TotalBytes: 8 << 30,
			HaveUsed: true, UsedBytes: 3 << 30,
			HaveSwap: true, SwapTotalBytes: 2 << 30, SwapFreeBytes: 2 << 30,
		},
		CPUs:     8,
		RootDisk: wslDiskFacts{Path: "/", Known: true, FreeBytes: 50 << 30, TotalBytes: 100 << 30},
		Resolv: wslResolvFacts{
			Path: "/etc/resolv.conf", Read: true,
			Nameservers: []string{"10.255.255.254"}, GeneratedHeader: true,
		},
		WindowsConfigPath: `C:\Users\ana\.wslconfig`,
		WindowsConfig:     &WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
	}
}

// withLocalStorage classifies the working directory through the fixture table, so a test can move it
// to a different mount by changing nothing but the path.
func withLocalStorage(t *testing.T, facts wslAdvisorFacts, path string) wslAdvisorFacts {
	t.Helper()
	facts.Cwd = path
	facts.LocalProject.Path = path
	facts.LocalProject.Label = path
	facts.LocalProject.Storage = system.ClassifyStorage(facts.Table, path, path)
	return facts
}

func findingByID(t *testing.T, findings []wslFinding, id string) wslFinding {
	t.Helper()
	for _, finding := range findings {
		if finding.ID == id {
			return finding
		}
	}
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.ID)
	}
	t.Fatalf("no finding %q; the report held %v", id, ids)
	return wslFinding{}
}

func hasFinding(findings []wslFinding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func findingIDs(findings []wslFinding) []string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.ID)
	}
	return ids
}

// TestWSLFindingsOnAHealthyHost pins the whole healthy report: the storage is native, the limits are
// applied, the drives are listed as a fact, and the only notice is the documented resolver route. A
// report that invents a problem on a host with none is a report nobody trusts with the real ones.
func TestWSLFindingsOnAHealthyHost(t *testing.T) {
	findings := wslAdvisorFindings(healthyWSLLinuxFacts(t))

	want := []string{"dns-through-host", "storage-cwd-native", "mounts-host", "resources-applied"}
	if got := findingIDs(findings); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("findings = %v, want %v", got, want)
	}

	native := findingByID(t, findings, "storage-cwd-native")
	if native.Severity != wslSeverityInfo || native.Scope != wslScopeReadOnly {
		t.Errorf("the native finding = %v/%v, want an informational read-only one", native.Severity, native.Scope)
	}
	if !strings.Contains(native.Evidence, "/home/ana/app") || !strings.Contains(native.Evidence, "ext4") {
		t.Errorf("evidence = %q, want the path and the filesystem that was read", native.Evidence)
	}
	if !strings.Contains(native.Evidence, "a Git repository") {
		t.Errorf("evidence = %q, want the Git repository named", native.Evidence)
	}
	// The healthy case must not offer a move: there is nothing to move it away from.
	if native.Action != "" {
		t.Errorf("the native finding carries an action (%q) though there is nothing to do", native.Action)
	}
}

// TestWSLFindingsNameTheHostMountWhereverItIs is the central case: the drives are recognised by
// filesystem type and reported with the mount points this host actually uses.
func TestWSLFindingsNameTheHostMountWhereverItIs(t *testing.T) {
	facts := withLocalStorage(t, healthyWSLLinuxFacts(t), "/windir/c/Users/ana/app")
	findings := wslAdvisorFindings(facts)

	hosted := findingByID(t, findings, "storage-cwd-host")
	if hosted.Severity != wslSeverityCritical {
		t.Errorf("severity = %v, want critical for the working tree on the host mount", hosted.Severity)
	}
	if hosted.Scope != wslScopeDistribution || hosted.Origin != wslOriginMeasured {
		t.Errorf("scope/origin = %v/%v, want a change made here from a measurement", hosted.Scope, hosted.Origin)
	}
	if !strings.Contains(hosted.Evidence, "/windir/c") || !strings.Contains(hosted.Evidence, "9p") {
		t.Errorf("evidence = %q, want the real mount point and filesystem type", hosted.Evidence)
	}
	if hosted.Benefit == "" || hosted.Action == "" {
		t.Errorf("an actionable finding must carry a benefit and an action: %+v", hosted)
	}
	// The impact is explained without a number this run did not measure.
	for _, word := range []string{"Git", "package manager", "build", "watcher"} {
		if !strings.Contains(hosted.Why, word) {
			t.Errorf("the explanation does not name %q: %q", word, hosted.Why)
		}
	}
	if strings.Contains(hosted.Why, "%") {
		t.Errorf("the explanation promises a percentage that was not measured: %q", hosted.Why)
	}

	// The drive list is a separate fact, and it names the custom automount root rather than /mnt/c.
	mounts := findingByID(t, findings, "mounts-host")
	if !strings.Contains(mounts.Evidence, "/windir/c (9p)") {
		t.Errorf("mount evidence = %q, want the mount point the host configured", mounts.Evidence)
	}
	if strings.Contains(mounts.Evidence, "/mnt/c") {
		t.Errorf("mount evidence = %q, which is the hardcoded path this module must not assume", mounts.Evidence)
	}
	// The attached ext4 disk is native storage and must not be presented as a Windows filesystem.
	if strings.Contains(mounts.Evidence, "/mnt/disk") {
		t.Errorf("mount evidence lists the native ext4 mount: %q", mounts.Evidence)
	}
}

func TestWSLFindingsStorageCases(t *testing.T) {
	t.Run("a virtual working tree is a warning", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Table = system.MountTable{Path: "/proc/self/mountinfo", Entries: []system.MountEntry{
			{MountPoint: "/", FileSystem: "overlay"},
		}}
		facts = withLocalStorage(t, facts, "/home/ana/app")

		finding := findingByID(t, wslAdvisorFindings(facts), "storage-cwd-virtual")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning for a working tree in an overlay", finding.Severity)
		}
		if !strings.Contains(finding.Why, "overlay") {
			t.Errorf("why = %q, want the overlay case explained", finding.Why)
		}
	})

	t.Run("an unreadable mount table is reported as such", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Table = system.MountTable{Path: "/proc/self/mountinfo", Err: "open: permission denied"}
		facts.LocalProject.Storage = system.ClassifyStorage(facts.Table, facts.Cwd, facts.Cwd)

		finding := findingByID(t, wslAdvisorFindings(facts), "storage-unknown")
		if finding.Severity != wslSeverityNotice {
			t.Errorf("severity = %v, want a notice", finding.Severity)
		}
		if !strings.Contains(finding.Evidence, "permission denied") {
			t.Errorf("evidence = %q, want the read error in it", finding.Evidence)
		}
		if hasFinding(wslAdvisorFindings(facts), "storage-cwd-host") {
			t.Error("an unreadable mount table produced a host-storage verdict")
		}
	})

	t.Run("other local filesystems get the neutral answer", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Table = system.MountTable{Path: "/proc/self/mountinfo", Entries: []system.MountEntry{
			{MountPoint: "/", FileSystem: "ext4", Device: "0:1"},
			{MountPoint: "/media/usb", FileSystem: "exfat", Device: "8:1"},
		}}
		facts = withLocalStorage(t, facts, "/media/usb/app/project")

		finding := findingByID(t, wslAdvisorFindings(facts), "storage-cwd-other")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeReadOnly {
			t.Errorf("finding = %v/%v, want a neutral notice with nothing to change", finding.Severity, finding.Scope)
		}
	})

	t.Run("a link that crosses the boundary is a warning with its evidence", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.LocalProject.Path = "/home/ana/app"
		facts.LocalProject.Storage = system.ClassifyStorage(facts.Table, "/home/ana/app", "/windir/c/Users/ana/app")
		facts.LocalProject.LinkNotes = []string{"/home/ana/app -> /windir/c/Users/ana/app"}

		finding := findingByID(t, wslAdvisorFindings(facts), "storage-links")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning", finding.Severity)
		}
		if !strings.Contains(finding.Evidence, "/windir/c/Users/ana/app") {
			t.Errorf("evidence = %q, want the link target", finding.Evidence)
		}
		if finding.Scope != wslScopeDistribution {
			t.Errorf("scope = %v, want a change made inside the distribution", finding.Scope)
		}
	})

	t.Run("other host-backed projects are counted and named", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Projects = []wslProjectFacts{
			{Path: "/windir/c/Users/ana/work", Label: "~/work", Marker: "go.mod",
				Storage: system.ClassifyStorage(facts.Table, "/windir/c/Users/ana/work", "/windir/c/Users/ana/work")},
			{Path: "/home/ana/other", Label: "~/other", Marker: "package.json",
				Storage: system.ClassifyStorage(facts.Table, "/home/ana/other", "/home/ana/other")},
		}

		finding := findingByID(t, wslAdvisorFindings(facts), "storage-projects-host")
		if !strings.HasPrefix(finding.Title, "1 other project is") {
			t.Errorf("title = %q, want the singular for one project", finding.Title)
		}
		if !strings.Contains(finding.Evidence, "~/work") || strings.Contains(finding.Evidence, "~/other") {
			t.Errorf("evidence = %q, want only the host-backed project named", finding.Evidence)
		}
	})
}

func TestWSLFindingsMemoryAndSwap(t *testing.T) {
	t.Run("no swap is a notice with the numbers in its evidence", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Memory.HaveSwap = false
		facts.Memory.SwapTotalBytes = 0

		finding := findingByID(t, wslAdvisorFindings(facts), "swap-absent")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeHost {
			t.Errorf("finding = %v/%v, want a host-scoped notice", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "8.0 GiB") {
			t.Errorf("evidence = %q, want the measured total in it", finding.Evidence)
		}
	})

	t.Run("heavy swap is a warning", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Memory.SwapFreeBytes = 512 << 20 // 75% of 2 GiB in use

		finding := findingByID(t, wslAdvisorFindings(facts), "swap-heavy")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning", finding.Severity)
		}
		if !strings.Contains(finding.Evidence, "75%") {
			t.Errorf("evidence = %q, want the share that was measured", finding.Evidence)
		}
	})

	t.Run("memory pressure is a warning and swap of the same run is not", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Memory.UsedBytes = 8<<30 - (512 << 20) // 94% used

		findings := wslAdvisorFindings(facts)
		finding := findingByID(t, findings, "memory-pressure")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning", finding.Severity)
		}
		if hasFinding(findings, "swap-heavy") {
			t.Error("swap was reported as heavy though it is 0% used")
		}
	})

	t.Run("unreadable memory is reported rather than assumed", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Memory = wslMemoryFacts{Err: "read /proc/meminfo: no such file"}

		finding := findingByID(t, wslAdvisorFindings(facts), "memory-unreadable")
		if finding.Severity != wslSeverityNotice {
			t.Errorf("severity = %v, want a notice", finding.Severity)
		}
		if !strings.Contains(finding.Evidence, "no such file") {
			t.Errorf("evidence = %q, want the read error", finding.Evidence)
		}
	})
}

func TestWSLFindingsResourceLimitsWaitingForARestart(t *testing.T) {
	t.Run("a memory limit the instance does not have is a pending change", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.WindowsConfig = &WSLResources{MemoryMB: 16384, Processors: 8}

		finding := findingByID(t, wslAdvisorFindings(facts), "resources-restart-pending")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeRestart {
			t.Errorf("finding = %v/%v, want a notice that waits for a restart", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "16384") || !strings.Contains(finding.Evidence, "8192") {
			t.Errorf("evidence = %q, want both numbers", finding.Evidence)
		}
		if !strings.Contains(finding.Action, "wsl --shutdown") {
			t.Errorf("action = %q, want the documented restart named as the user's step", finding.Action)
		}
	})

	t.Run("a processor difference is reported too", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.WindowsConfig = &WSLResources{MemoryMB: 8192, Processors: 16}

		finding := findingByID(t, wslAdvisorFindings(facts), "resources-restart-pending")
		if !strings.Contains(finding.Evidence, "processors") {
			t.Errorf("evidence = %q, want the processor difference named", finding.Evidence)
		}
	})

	t.Run("a match in the file and the instance is stated as applied", func(t *testing.T) {
		finding := findingByID(t, wslAdvisorFindings(healthyWSLLinuxFacts(t)), "resources-applied")
		if finding.Severity != wslSeverityInfo || finding.Scope != wslScopeReadOnly {
			t.Errorf("finding = %v/%v, want an informational reading", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "8 processors") {
			t.Errorf("evidence = %q, want the instance's own numbers", finding.Evidence)
		}
	})

	t.Run("the tolerance keeps the rounding out of the verdict", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		// A 3% difference is rounding, not a pending change: the file is a request in megabytes and
		// the kernel reports what it was given.
		facts.Memory.TotalBytes = uint64(8000) << 20
		if !hasFinding(wslAdvisorFindings(facts), "resources-applied") {
			t.Error("a 3% difference between the file and the instance was reported as pending")
		}
	})

	t.Run("an unread .wslconfig says so instead of guessing", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.WindowsConfig = nil
		facts.WindowsConfigPath = ""
		facts.WindowsConfigErr = "the Windows user profile could not be resolved"

		finding := findingByID(t, wslAdvisorFindings(facts), "resources-unreadable")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeHost {
			t.Errorf("finding = %v/%v, want a host-scoped notice", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Action, envWSLWindowsHome) {
			t.Errorf("action = %q, want the override named so the reader can fix it", finding.Action)
		}
		if !strings.Contains(finding.Evidence, "could not be resolved") {
			t.Errorf("evidence = %q, want the reason the file could not be read", finding.Evidence)
		}
	})

	t.Run("a file that was never read is a declared gap, not silence", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.WindowsConfig = nil
		facts.WindowsConfigPath = ""

		finding := findingByID(t, wslAdvisorFindings(facts), "resources-unreadable")
		if !strings.Contains(finding.Evidence, "No .wslconfig was read") {
			t.Errorf("evidence = %q, want the gap declared", finding.Evidence)
		}
	})
}

func TestWSLFindingsDiskSpace(t *testing.T) {
	t.Run("a nearly full filesystem is a warning", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.RootDisk = wslDiskFacts{Path: "/", Known: true, FreeBytes: 512 << 20, TotalBytes: 100 << 30}

		finding := findingByID(t, wslAdvisorFindings(facts), "disk-low")
		if finding.Severity != wslSeverityWarning || finding.Scope != wslScopeDistribution {
			t.Errorf("finding = %v/%v, want a warning inside the distribution", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Action, "Do not delete the source") {
			t.Errorf("action = %q, want the migration warning kept in it", finding.Action)
		}
	})

	t.Run("a filesystem with room produces no finding", func(t *testing.T) {
		if hasFinding(wslAdvisorFindings(healthyWSLLinuxFacts(t)), "disk-low") {
			t.Error("a filesystem with 50% free was reported as nearly full")
		}
	})

	t.Run("an unreadable filesystem is not reported as full", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.RootDisk = wslDiskFacts{Path: "/", Err: "no such path"}
		if hasFinding(wslAdvisorFindings(facts), "disk-low") {
			t.Error("a filesystem that could not be read was reported as nearly full")
		}
	})
}

func TestWSLFindingsDNS(t *testing.T) {
	t.Run("a WSL-generated resolver is a documented notice", func(t *testing.T) {
		finding := findingByID(t, wslAdvisorFindings(healthyWSLLinuxFacts(t)), "dns-through-host")
		if finding.Severity != wslSeverityNotice {
			t.Errorf("severity = %v, want a notice: WSL's default is not a defect", finding.Severity)
		}
		if finding.Origin != wslOriginDocumented {
			t.Errorf("origin = %v, want documented: the route is documented and no lookup was timed", finding.Origin)
		}
		if finding.Scope != wslScopeRestart {
			t.Errorf("scope = %v, want the restart-note, since the change lives in /etc/wsl.conf", finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "10.255.255.254") {
			t.Errorf("evidence = %q, want the resolver that was read", finding.Evidence)
		}
	})

	t.Run("a resolver the distribution manages is left alone", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Conf.GenerateResolvConf = system.WSLConfValue{Value: "false", Set: true}
		facts.Resolv.GeneratedHeader = false
		facts.Resolv.LinkTarget = "/run/systemd/resolve/stub-resolv.conf"

		if hasFinding(wslAdvisorFindings(facts), "dns-through-host") {
			t.Error("a hand-managed resolver was reported as WSL-generated")
		}
	})

	t.Run("a resolver with no nameserver is a warning", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Resolv.Nameservers = nil
		facts.Resolv.GeneratedHeader = false

		finding := findingByID(t, wslAdvisorFindings(facts), "dns-no-nameserver")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning", finding.Severity)
		}
		if !strings.Contains(finding.Action, "/etc/wsl.conf") {
			t.Errorf("action = %q, want the file that decides whether WSL writes it", finding.Action)
		}
	})

	t.Run("an unreadable resolver is reported as unreadable", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Resolv = wslResolvFacts{Path: "/etc/resolv.conf", Err: "permission denied"}

		finding := findingByID(t, wslAdvisorFindings(facts), "dns-unreadable")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeReadOnly {
			t.Errorf("finding = %v/%v, want a read-only notice", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "permission denied") {
			t.Errorf("evidence = %q, want the reason", finding.Evidence)
		}
	})
}

func TestWSLFindingsInterop(t *testing.T) {
	t.Run("interop turned off in the file is named as your choice", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Conf.InteropEnabled = system.WSLConfValue{Value: "false", Set: true}

		finding := findingByID(t, wslAdvisorFindings(facts), "interop-disabled")
		if finding.Severity != wslSeverityNotice || finding.Scope != wslScopeRestart {
			t.Errorf("finding = %v/%v, want a restart-scoped notice", finding.Severity, finding.Scope)
		}
		if !strings.Contains(finding.Evidence, "set in /etc/wsl.conf") {
			t.Errorf("evidence = %q, want the file named as where it was set", finding.Evidence)
		}
	})

	t.Run("a missing registration is a warning even with interop enabled", func(t *testing.T) {
		facts := healthyWSLLinuxFacts(t)
		facts.Runtime.Signals = []system.WSLSignal{
			{Name: "kernel-release", Detail: "5.15.153.1-microsoft-standard-WSL2", Strength: system.WSLSignalKernel, Found: true},
			{Name: "interop-binfmt", Detail: "/proc/sys/fs/binfmt_misc/WSLInterop", Strength: system.WSLSignalInstance, Found: false},
		}

		finding := findingByID(t, wslAdvisorFindings(facts), "interop-missing")
		if finding.Severity != wslSeverityWarning {
			t.Errorf("severity = %v, want a warning", finding.Severity)
		}
		if !strings.Contains(finding.Evidence, "interop-binfmt") || !strings.Contains(finding.Evidence, "was not found") {
			t.Errorf("evidence = %q, want the signal and that it was absent", finding.Evidence)
		}
		if !strings.Contains(finding.Action, "/proc/sys/fs/binfmt_misc") {
			t.Errorf("action = %q, want the path to check by hand", finding.Action)
		}
	})

	t.Run("a registered handler produces no interop finding", func(t *testing.T) {
		findings := wslAdvisorFindings(healthyWSLLinuxFacts(t))
		if hasFinding(findings, "interop-missing") || hasFinding(findings, "interop-disabled") {
			t.Errorf("interop findings were produced on a working setup: %v", findingIDs(findings))
		}
	})
}

// TestWSLFindingsAreOrderedAndComplete is the shape guard for the whole list: the most expensive
// finding comes first, every finding can be read on its own, and an actionable one always says what
// the action is worth -- an action without a stated benefit is an instruction nobody can judge.
func TestWSLFindingsAreOrderedAndComplete(t *testing.T) {
	facts := healthyWSLLinuxFacts(t)
	facts = withLocalStorage(t, facts, "/windir/c/Users/ana/app")
	facts.Memory.SwapFreeBytes = 0
	facts.RootDisk = wslDiskFacts{Path: "/", Known: true, FreeBytes: 100 << 20, TotalBytes: 100 << 30}
	facts.Conf.InteropEnabled = system.WSLConfValue{Value: "false", Set: true}
	facts.WindowsConfig = &WSLResources{MemoryMB: 32768}
	facts.Resolv.Nameservers = nil
	facts.LocalProject.LinkNotes = []string{"/home/ana/app -> /windir/c/app"}

	findings := wslAdvisorFindings(facts)
	if len(findings) < 6 {
		t.Fatalf("only %d findings from a host with six problems; the list is not being built", len(findings))
	}

	seen := map[string]bool{}
	// Ranks descend as severity rises, so a sorted list has non-decreasing ranks; the floor starts
	// below every rank so the first finding cannot fail the check by being the most severe.
	previousRank := -(1 << 30)
	for _, finding := range findings {
		if finding.ID == "" || finding.Title == "" || finding.Evidence == "" || finding.Why == "" {
			t.Errorf("a finding is missing a field a reader needs: %+v", finding)
		}
		if seen[finding.ID] {
			t.Errorf("finding %q appears twice in one report", finding.ID)
		}
		seen[finding.ID] = true

		if finding.Action != "" && finding.Benefit == "" {
			t.Errorf("finding %q tells the reader what to do without saying what it is worth", finding.ID)
		}
		if finding.Scope.String() == "" || finding.Origin.String() == "" {
			t.Errorf("finding %q cannot say where the change belongs or where the claim came from", finding.ID)
		}
		rank := finding.Severity.rank()
		if rank < previousRank {
			t.Errorf("finding %q (%v) came after a less severe one; the list is not ordered",
				finding.ID, finding.Severity)
		}
		previousRank = rank
	}

	// The critical finding must be first: it is the one that costs time on every operation.
	if findings[0].ID != "storage-cwd-host" {
		t.Errorf("first finding = %q, want the host-backed working tree first", findings[0].ID)
	}
	if !hasFinding(findings, "disk-low") || !hasFinding(findings, "swap-heavy") ||
		!hasFinding(findings, "interop-disabled") || !hasFinding(findings, "resources-restart-pending") {
		t.Errorf("a problem each test set up is missing from the report: %v", findingIDs(findings))
	}
}

func TestWSLSeverityAndScopeLabels(t *testing.T) {
	severities := map[wslSeverity]string{
		wslSeverityCritical: "Critical",
		wslSeverityWarning:  "Warning",
		wslSeverityNotice:   "Notice",
		wslSeverityInfo:     "Info",
	}
	for severity, want := range severities {
		if got := severity.String(); got != want {
			t.Errorf("wslSeverity(%d) = %q, want %q", severity, got, want)
		}
	}
	if !(wslSeverityInfo < wslSeverityNotice && wslSeverityNotice < wslSeverityWarning &&
		wslSeverityWarning < wslSeverityCritical) {
		t.Error("the severities must be ordered least to most severe, so a report can sort by them")
	}

	scopes := map[wslScope]string{
		wslScopeReadOnly:     "nothing to change",
		wslScopeDistribution: "changed inside this distribution",
		wslScopeRestart:      "changed inside this distribution, and applied only after WSL restarts it",
		wslScopeHost:         "changed on the Windows side, by you",
	}
	for scope, want := range scopes {
		if got := scope.String(); got != want {
			t.Errorf("wslScope(%d) = %q, want %q", scope, got, want)
		}
	}

	if got := wslOriginMeasured.String(); got != "measured here" {
		t.Errorf("wslOriginMeasured = %q, want \"measured here\"", got)
	}
	if got := wslOriginDocumented.String(); got != "documented behaviour" {
		t.Errorf("wslOriginDocumented = %q, want \"documented behaviour\"", got)
	}
}

// ============================================================================
// THE COLLECTOR AND THE PROJECT DISCOVERY
// ============================================================================

// advisorInputFor builds an input whose filesystem is a map of directory listings and whose mount
// table is the fixture, so the collector and the discovery can be exercised for a machine that is
// not this one.
func advisorInputFor(dirs map[string][]os.DirEntry, table system.MountTable, meminfo string) wslAdvisorInput {
	return wslAdvisorInput{
		Cwd:     "/home/ana/app",
		Runtime: system.WSLRuntime{Inside: true, Version: 2},
		ReadMounts: func() system.MountTable {
			return table
		},
		ReadConf: func() system.WSLConf {
			return system.WSLConf{Path: "/etc/wsl.conf", Read: true}
		},
		ReadMeminfo: func() ([]byte, error) { return []byte(meminfo), nil },
		ReadResolv: func() (string, []byte, string, error) {
			return "/etc/resolv.conf", []byte("nameserver 10.255.255.254\n"), "", nil
		},
		NumCPU: func() int { return 4 },
		Statfs: func(string) (uint64, uint64, bool) { return 10 << 30, 100 << 30, true },
		ReadDir: func(path string) ([]os.DirEntry, error) {
			entries, ok := dirs[path]
			if !ok {
				return nil, errors.New("no such directory: " + path)
			}
			return entries, nil
		},
		EvalSymlinks: func(path string) (string, error) { return path, nil },
		Exists:       fakeExists(dirs),
		Getenv:       func(string) string { return "/home/ana" },
	}
}

// fakeExists answers whether a path is there in the fixture filesystem: a listed directory, or a
// child of one, which is what a marker check asks about.
func fakeExists(dirs map[string][]os.DirEntry) func(string) bool {
	return func(path string) bool {
		if _, ok := dirs[path]; ok {
			return true
		}
		dir, base := filepath.Split(path)
		for _, entry := range dirs[filepath.Clean(dir)] {
			if entry.Name() == base {
				return true
			}
		}
		return false
	}
}

func dirEntry(name string, dir bool) os.DirEntry { return fakeDirEntry{name: name, dir: dir} }

type fakeDirEntry struct {
	name string
	dir  bool
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return e.dir }
func (e fakeDirEntry) Type() os.FileMode          { return 0 }
func (e fakeDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("not used") }

// TestWSLAdvisorCollectorReportsEveryLimitItHit pins the rule that a reading which did not happen is
// reported: the mount table, the memory and the resolver each fail here, and each one lands in the
// report as a limit rather than as an empty section that reads as healthy.
func TestWSLAdvisorCollectorReportsEveryLimitItHit(t *testing.T) {
	table := system.MountTable{Path: "/proc/self/mountinfo", Err: "open: permission denied"}
	in := advisorInputFor(map[string][]os.DirEntry{"/home/ana/app": {}}, table, "")
	in.ReadMeminfo = func() ([]byte, error) { return nil, errors.New("no /proc/meminfo here") }
	in.ReadResolv = func() (string, []byte, string, error) {
		return "/etc/resolv.conf", nil, "", errors.New("permission denied")
	}
	in.ReadConf = func() system.WSLConf {
		return system.WSLConf{Path: "/etc/wsl.conf", Err: "read: permission denied"}
	}
	in.NumCPU = func() int { return 0 }
	in.ConfigErr = "the Windows profile could not be resolved"
	in.Statfs = func(string) (uint64, uint64, bool) { return 0, 0, false }

	facts := collectWSLAdvisorFacts(in)

	if len(facts.Limits) < 5 {
		t.Errorf("Limits = %v, want a line for each reading that failed", facts.Limits)
	}
	if facts.Table.Err == "" || facts.Memory.Err == "" || facts.Resolv.Err == "" || facts.Conf.Err == "" {
		t.Errorf("the failures were not all kept: %+v", facts)
	}
	if facts.RootDisk.Known || facts.RootDisk.Err == "" {
		t.Error("an unread filesystem lost the reason it could not be read")
	}
	if facts.CPUsErr == "" {
		t.Error("a processor count of zero was not recorded as a reading that failed")
	}

	// The report still builds, and says what it could not see rather than inventing the missing half.
	findings := wslAdvisorFindings(facts)
	if !hasFinding(findings, "storage-unknown") || !hasFinding(findings, "memory-unreadable") {
		t.Errorf("findings = %v, want the unread storage and memory reported", findingIDs(findings))
	}
}

func TestWSLAdvisorCollectorReadsMemoryAndDisk(t *testing.T) {
	meminfo := "MemTotal:       8192000 kB\nMemAvailable:   4096000 kB\nSwapTotal:      2048000 kB\nSwapFree:       1024000 kB\n"
	in := advisorInputFor(map[string][]os.DirEntry{"/home/ana/app": {dirEntry(".git", true)}}, wslAdvisorMountTableFixture(t), meminfo)

	facts := collectWSLAdvisorFacts(in)

	if !facts.Memory.Read || !facts.Memory.HaveTotal {
		t.Fatalf("memory was not read: %+v", facts.Memory)
	}
	if facts.Memory.TotalBytes != 8192000*1024 {
		t.Errorf("MemTotal = %d, want the file's own value in bytes", facts.Memory.TotalBytes)
	}
	if facts.Memory.UsedBytes != (8192000-4096000)*1024 {
		t.Errorf("used = %d, want total minus MemAvailable", facts.Memory.UsedBytes)
	}
	if !facts.Memory.HaveSwap || facts.Memory.SwapTotalBytes != 2048000*1024 {
		t.Errorf("swap = %+v, want the file's own swap total", facts.Memory)
	}
	if facts.CPUs != 4 {
		t.Errorf("CPUs = %d, want the injected count", facts.CPUs)
	}
	if !facts.RootDisk.Known || facts.RootDisk.FreeBytes != 10<<30 {
		t.Errorf("root disk = %+v, want the injected space", facts.RootDisk)
	}
	if !facts.LocalProject.GitRepo {
		t.Error("the .git entry beside the working directory was not read as a repository")
	}
	if got := facts.NativeRootPath(); got != "/" {
		t.Errorf("NativeRootPath = %q, want the root mount", got)
	}
	if got := facts.NativeStorageName(); got != "ext4 at /" {
		t.Errorf("NativeStorageName = %q, want the filesystem and the mount point", got)
	}
}

// TestWSLAdvisorProjectDiscoveryIsBoundedAndMarkerDriven pins both halves of the discovery rule: a
// directory with no marker is not a project, and a home directory with a thousand entries cannot
// produce a thousand rows.
func TestWSLAdvisorProjectDiscoveryIsBoundedAndMarkerDriven(t *testing.T) {
	entries := []os.DirEntry{dirEntry("notes", true), dirEntry("dotfiles", true)}
	for i := 0; i < wslAdvisorProjectLimit*3; i++ {
		entries = append(entries, dirEntry("project"+string(rune('a'+i%26))+string(rune('0'+i/26)), true))
	}
	dirs := map[string][]os.DirEntry{"/home/ana": entries}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := "/home/ana/" + entry.Name()
		if entry.Name() == "notes" {
			dirs[path] = []os.DirEntry{dirEntry("todo.md", false)}
			continue
		}
		dirs[path] = []os.DirEntry{dirEntry("go.mod", false), dirEntry("node_modules", true)}
	}

	in := advisorInputFor(dirs, wslAdvisorMountTableFixture(t), "MemTotal: 1000 kB\n")
	in.Cwd = "/home/ana"

	facts := collectWSLAdvisorFacts(in)

	if len(facts.Projects) > wslAdvisorProjectLimit {
		t.Errorf("%d projects reported, want at most %d", len(facts.Projects), wslAdvisorProjectLimit)
	}
	if len(facts.Projects) == 0 {
		t.Fatal("no project was found though the directories carry markers")
	}
	for _, project := range facts.Projects {
		if strings.HasSuffix(project.Path, "/notes") {
			t.Errorf("a directory without a project marker was reported: %s", project.Path)
		}
		if project.Marker == "" {
			t.Errorf("project %s has no marker recorded, so the row cannot say why it is listed", project.Path)
		}
		if !wslListHas(project.DependencyDirs, "node_modules") {
			t.Errorf("project %s did not report its dependency directory: %v", project.Path, project.DependencyDirs)
		}
		if project.Label == "" {
			t.Errorf("project %s has no label for its row", project.Path)
		}
	}

	// The current directory is never repeated in the list beside it.
	for _, project := range facts.Projects {
		if project.Path == facts.Cwd {
			t.Errorf("the current directory %s appears in the list of the projects beside it", project.Path)
		}
	}
}

// TestWSLAdvisorLinkNotesOnlyForCrossingLinks keeps the link warning about the boundary: a link that
// stays on the same kind of storage is not a finding, because there is nothing to warn about.
func TestWSLAdvisorLinkNotesOnlyForCrossingLinks(t *testing.T) {
	table := wslAdvisorMountTableFixture(t)
	in := advisorInputFor(map[string][]os.DirEntry{
		"/home/ana/app": {dirEntry("shared", false)},
	}, table, "MemTotal: 1000 kB\n")
	// A link that lands on the same kind of storage produces no note.
	in.EvalSymlinks = func(path string) (string, error) {
		if strings.HasSuffix(path, "/shared") {
			return "/home/ana/other/shared", nil
		}
		return path, nil
	}
	if note := wslLinkStorageNote(in, "/home/ana/app", "shared", system.StorageDistribution); note != "" {
		t.Errorf("a link onto the same kind of storage produced %q", note)
	}

	// A link that crosses onto the host storage produces one, naming both ends.
	in.EvalSymlinks = func(path string) (string, error) {
		if strings.HasSuffix(path, "/shared") {
			return "/windir/c/shared", nil
		}
		return path, nil
	}
	note := wslLinkStorageNote(in, "/home/ana/app", "shared", system.StorageDistribution)
	if !strings.Contains(note, "/windir/c/shared") || !strings.Contains(note, "Windows host") {
		t.Errorf("note = %q, want the target and the classification", note)
	}

	// A broken link is not a storage claim.
	in.EvalSymlinks = func(string) (string, error) { return "", errors.New("broken") }
	if note := wslLinkStorageNote(in, "/home/ana/app", "shared", system.StorageDistribution); note != "" {
		t.Errorf("a link that cannot be resolved produced %q", note)
	}
}

func TestWSLResolvFactsReading(t *testing.T) {
	facts := wslResolvFactsFrom("/etc/resolv.conf", []byte(strings.Join([]string{
		"# This file was generated by WSL. To stop automatic generation, add",
		"# [network]",
		"# generateResolvConf = false",
		"nameserver 10.255.255.254",
		"nameserver 1.1.1.1",
		"search corp.example",
	}, "\n")), "/run/systemd/resolve/stub-resolv.conf", nil)

	if !facts.Read || !facts.GeneratedHeader {
		t.Errorf("facts = %+v, want the generated header read", facts)
	}
	if len(facts.Nameservers) != 2 || facts.Nameservers[1] != "1.1.1.1" {
		t.Errorf("nameservers = %v, want both in order", facts.Nameservers)
	}
	if len(facts.SearchDomains) != 1 || facts.SearchDomains[0] != "corp.example" {
		t.Errorf("search domains = %v, want the one listed", facts.SearchDomains)
	}
	if facts.LinkTarget != "/run/systemd/resolve/stub-resolv.conf" {
		t.Errorf("link target = %q, want the link that was read", facts.LinkTarget)
	}

	empty := wslResolvFactsFrom("/etc/resolv.conf", nil, "", errors.New("no such file"))
	if empty.Read || empty.Err == "" {
		t.Errorf("empty = %+v, want an unread file with its reason", empty)
	}
}

func TestWSLResourcesSummary(t *testing.T) {
	cases := []struct {
		resources WSLResources
		want      string
	}{
		{WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048}, "memory 8192 MB, processors 8, swap 2048 MB"},
		{WSLResources{MemoryMB: 512}, "memory 512 MB"},
		{WSLResources{}, "no memory, processor or swap limit is set"},
	}
	for _, tc := range cases {
		if got := wslResourcesSummary(tc.resources); got != tc.want {
			t.Errorf("wslResourcesSummary(%+v) = %q, want %q", tc.resources, got, tc.want)
		}
	}
}

func TestWSLHelperPercentAndTolerance(t *testing.T) {
	cases := []struct {
		part, whole uint64
		want        int
	}{
		{50, 100, 50},
		{0, 100, 0},
		{100, 0, 0},
		{200, 100, 100},
		{1, 3, 33},
	}
	for _, tc := range cases {
		if got := wslPercent(tc.part, tc.whole); got != tc.want {
			t.Errorf("wslPercent(%d, %d) = %d, want %d", tc.part, tc.whole, got, tc.want)
		}
	}

	if got := wslAbs(-5); got != 5 {
		t.Errorf("wslAbs(-5) = %d, want 5", got)
	}
	if got := wslMemoryToleranceMB(100); got != 256 {
		t.Errorf("tolerance for 100 MB = %d, want the 256 MB floor", got)
	}
	if got := wslMemoryToleranceMB(16384); got != 1638 {
		t.Errorf("tolerance for 16 GiB = %d, want a tenth of it", got)
	}

	if got := wslCountLabel(1, "project is", "projects are"); got != "1 project is" {
		t.Errorf("wslCountLabel(1) = %q, want the singular", got)
	}
	if got := wslCountLabel(3, "project is", "projects are"); got != "3 projects are" {
		t.Errorf("wslCountLabel(3) = %q, want the plural", got)
	}
	if got := abbreviateHome("/home/ana/app", "/home/ana"); got != "~/app" {
		t.Errorf("abbreviateHome = %q, want the home shorthand", got)
	}
	if got := abbreviateHome("/srv/app", "/home/ana"); got != "/srv/app" {
		t.Errorf("abbreviateHome = %q, want the path unchanged", got)
	}
	if got := abbreviateHome("", "/home/ana"); got != "" {
		t.Errorf("abbreviateHome(empty) = %q, want empty", got)
	}
}

// TestWSLAdvisorUsesNoShellAndNoHostGuess is a guard on the module's own source: the utility that
// must never run a command or claim something about Windows it did not read is checked for the two
// calls that would do it. It reads the file rather than the behaviour because the point is the
// absence of a mechanism, which a behavioural test cannot prove.
func TestWSLAdvisorUsesNoShellAndNoHostGuess(t *testing.T) {
	source, err := os.ReadFile("wsladvisor.go")
	if err != nil {
		t.Fatalf("read the advisor source: %v", err)
	}
	for _, forbidden := range []string{"os/exec", "exec.Command", "cmd.exe", "/mnt/c", "Start-Process"} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("wsladvisor.go contains %q: this module must not run commands or assume a host path", forbidden)
		}
	}
}

// wslModuleSourceFiles is every file the WSL module is made of: the report, its three screens and
// the four engines plus the shared /proc/meminfo reader. The list is written out rather than derived
// on purpose -- the guard below is only worth anything if a file added to the module has to be added
// here, and a glob would quietly cover a file nobody decided to include.
var wslModuleSourceFiles = []string{
	"wsladvisor.go",
	"wsladvisorscreen.go",
	"wsladvisorassist.go",
	"wsladvisorrun.go",
	"../system/wslruntime.go",
	"../system/wslmount.go",
	"../system/wslmigrate.go",
	"../system/wslbench.go",
	"../system/meminfo.go",
}

// TestWSLModuleRunsNoCommandAndAssumesNoHostPath is the class guard for the rule the module is built
// on: nothing in it executes a command, and nothing in it assumes a host path. It reads the sources
// because the subject is the absence of a mechanism, which no behavioural test can prove -- a test
// can only show that the paths it happened to exercise ran no command, while this shows that there is
// no call site that could.
//
// Whole-line comments are dropped before the check, because these files explain in prose exactly
// which paths they refuse to assume (a comment saying "there is no /mnt/c in this file" is the rule
// being stated, not broken), and a fragment kept inside a line of code is still checked.
func TestWSLModuleRunsNoCommandAndAssumesNoHostPath(t *testing.T) {
	forbidden := []string{"os/exec", "exec.Command", "cmd.exe", "powershell", "Start-Process", "/mnt/c"}

	for _, name := range wslModuleSourceFiles {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var code []string
		for _, line := range strings.Split(string(source), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			code = append(code, line)
		}
		joined := strings.Join(code, "\n")
		for _, token := range forbidden {
			if strings.Contains(joined, token) {
				t.Errorf("%s contains %q outside a comment: this module runs no command and assumes no host path",
					name, token)
			}
		}
	}
}

// TestWSLAdvisorCollectOnThisHost runs the real collector on whatever machine the tests are on. It
// proves the collector does not panic, that it reports the reason for anything it could not read,
// and that the report is well formed even when the machine is not a WSL one at all.
func TestWSLAdvisorCollectOnThisHost(t *testing.T) {
	facts := collectWSLAdvisorFacts(defaultWSLAdvisorInput(os.TempDir(), "", nil, ""))

	if facts.Table.Path == "" {
		t.Error("the mount table path was not recorded")
	}
	if facts.Cwd != os.TempDir() {
		t.Errorf("Cwd = %q, want the directory it was given", facts.Cwd)
	}
	findings := wslAdvisorFindings(facts)
	for _, finding := range findings {
		if finding.ID == "" || finding.Title == "" || finding.Evidence == "" {
			t.Errorf("a finding on this host is missing its fields: %+v", finding)
		}
	}
	t.Logf("this host produced %d findings: %v", len(findings), findingIDs(findings))
	t.Logf("limits reported: %v", facts.Limits)
	if facts.CPUsErr == "" && facts.CPUs <= 0 {
		t.Error("a processor count that could not be read was not reported")
	}

	// The deadline for the whole collection is a render-path concern rather than a test's, but the
	// collection must at least be bounded: if it takes this long it is doing far more than reading a
	// handful of files.
	done := make(chan struct{})
	go func() {
		collectWSLAdvisorFacts(defaultWSLAdvisorInput(os.TempDir(), "", nil, ""))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("collecting the facts took longer than 20 seconds")
	}
}

// ============================================================================
// THE KEYS: ONE GRAMMAR, PINNED ON THE SCREENS' OWN ROWS
// ============================================================================
//
// odd/tasks/key-grammar.md fixes one meaning per key: Enter opens a screen, a row's detail or the
// way back, and every command runs on its own letter, said on the row and in the footer. These tests
// press the key each row declares -- the letter read off the row itself -- and assert the effect, so
// a row, a footer and a handler cannot drift apart, and Enter can never quietly start, confirm, stop
// or cancel anything again.

// pressWSLMsg sends one key message through Update and returns the model with the command the press
// asked for. It goes through the whole dispatch rather than a screen's handler, so the test also
// proves the key reaches the screen instead of being taken by the leader key or a global meaning.
// The copy's own commands answer with the model's pointer, which is one of the two shapes Update
// returns, so both are unwrapped here.
func pressWSLMsg(t *testing.T, m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()

	updated, cmd := m.Update(msg)
	switch next := updated.(type) {
	case Model:
		return next, cmd
	case *Model:
		return *next, cmd
	}
	t.Fatalf("the update returned %T, want the model", updated)
	return m, cmd
}

// wslLetterKey is the key message for one rune, which is how a command letter arrives.
func wslLetterKey(letter string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(letter)}
}

// wslEnter and wslEscape are the two named keys these screens share.
var (
	wslEnter  = tea.KeyMsg{Type: tea.KeyEnter}
	wslEscape = tea.KeyMsg{Type: tea.KeyEsc}
)

// wslRowKey returns the letter a row declares in front of its wording. A row that acts where it
// stands has to carry one, so a row without it fails here rather than in the test that would have
// pressed it.
func wslRowKey(t *testing.T, label string) string {
	t.Helper()

	if !strings.HasPrefix(label, "[") {
		t.Fatalf("the row %q carries no letter in brackets, so it does not say it is a command", label)
	}
	end := strings.Index(label, "]")
	if end <= 1 {
		t.Fatalf("the row %q carries no key between its brackets", label)
	}
	return label[1:end]
}

// wslFooter is the frame's footer: the line that names the way out. It is read from the rendered
// frame, not from the hint slice, so the test checks what the user actually reads.
func wslFooter(t *testing.T, m Model) string {
	t.Helper()

	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(line, "[Esc]") {
			return line
		}
	}
	t.Fatalf("the frame drew no footer naming [Esc]:\n%s", m.View())
	return ""
}

// TestWSLAdvisorRowsSayWhatEachOneIs pins the report's three kinds of row: the read-again row acts
// where it stands and carries `r`, the two rows that act one level in carry the arrow the utilities
// section's own rows carry, and the way back carries it pointing the other way. Pressing `r` reads
// the machine again; pressing Enter on that same row does nothing, which is the rule the whole pass
// exists for.
func TestWSLAdvisorRowsSayWhatEachOneIs(t *testing.T) {
	m := wslAdvisorFrameModel(t, ScreenWSLAdvisor)
	// The two rows that act one level in are offered only where their engine can finish, so the
	// fixture is given the distribution's own storage to copy to: without it the report says in its
	// body why there is no copy row, and the row really is absent.
	m.WSLAdvisor.Facts.NativeParents = []wslNativeParent{{
		Path: "/mnt/disk", Label: "/mnt/disk", SpaceKnown: true, FreeBytes: 41 << 30,
		Storage: system.ClassifyStorage(m.WSLAdvisor.Facts.Table, "/mnt/disk", "/mnt/disk"),
	}}

	rows := m.wslAdvisorRows()
	index := map[wslAdvisorRowKind]int{}
	for i, row := range rows {
		if _, seen := index[row.Kind]; !seen {
			index[row.Kind] = i
		}
	}

	// The re-read row says it is a command, and the letter it says is the letter that reads.
	reread, ok := index[wslRowReread]
	if !ok {
		t.Fatalf("the report offers no re-read row: %v", wslAdvisorRowLabels(rows))
	}
	if got := wslRowKey(t, rows[reread].Label); got != "r" {
		t.Errorf("the re-read row declares %q; the footer and the handler say \"r\"", got)
	}

	onReread := m
	onReread.Cursor = reread
	afterEnter, cmd := pressWSLMsg(t, onReread, wslEnter)
	if cmd != nil {
		t.Error("Enter asked for a command on the re-read row; Enter opens or chooses and never runs")
	}
	if !afterEnter.WSLAdvisor.Resolved {
		t.Error("Enter started a re-read; the re-read runs on its own letter")
	}

	afterLetter, cmd := pressWSLMsg(t, onReread, wslLetterKey(wslRowKey(t, rows[reread].Label)))
	if cmd == nil {
		t.Error("the letter the re-read row declares asked for nothing")
	}
	if afterLetter.WSLAdvisor.Resolved {
		t.Error("the letter the re-read row declares did not start a re-read")
	}

	// The two rows that act one level in carry the arrow, and Enter is the key that presses them.
	for _, kind := range []wslAdvisorRowKind{wslRowBenchmark, wslRowMigrate} {
		at, ok := index[kind]
		if !ok {
			t.Fatalf("the report offers no row of kind %d: %v", kind, wslAdvisorRowLabels(rows))
		}
		if !strings.HasPrefix(rows[at].Label, utilitiesScreenMark) {
			t.Errorf("the row %q opens a screen and carries no %q", rows[at].Label, utilitiesScreenMark)
		}
		onRow := m
		onRow.Cursor = at
		after, _ := pressWSLMsg(t, onRow, wslEnter)
		want := ScreenWSLBenchmark
		if kind == wslRowMigrate {
			want = ScreenWSLMigration
		}
		if after.Screen != want {
			t.Errorf("Enter on %q went to screen %v, want %v", rows[at].Label, after.Screen, want)
		}
	}

	// The way back says so, and Enter opens it.
	at, ok := index[wslRowBack]
	if !ok {
		t.Fatalf("the report offers no way back: %v", wslAdvisorRowLabels(rows))
	}
	if got := rows[at].Label; got != "← Back" {
		t.Errorf("the way back reads %q, want \"← Back\"", got)
	}
	onBack := m
	onBack.Cursor = at
	if after, _ := pressWSLMsg(t, onBack, wslEnter); after.Screen != ScreenUtilities {
		t.Errorf("Enter on the way back went to screen %v, want the utilities section", after.Screen)
	}

	footer := wslFooter(t, m)
	if !strings.Contains(footer, "[Enter] open") {
		t.Errorf("the footer does not name Enter as the open it is: %q", footer)
	}
	if !strings.Contains(footer, "[r] read it again") {
		t.Errorf("the footer does not name the letter the re-read row carries: %q", footer)
	}
	if strings.Contains(footer, "Space") {
		t.Errorf("the footer offers Space on a screen with no switch: %q", footer)
	}
}

// TestWSLBenchmarkMeasuresOnItsLetterNotOnEnter pins the measurement screen: the measuring row
// carries `m`, the footer names it, `m` starts the run and Enter on that same row does not. A run in
// flight is not started twice.
func TestWSLBenchmarkMeasuresOnItsLetterNotOnEnter(t *testing.T) {
	m := wslAdvisorFrameModel(t, ScreenWSLBenchmark)
	m.Cursor = 0

	rows := m.wslBenchmarkRows()
	letter := wslRowKey(t, rows[0].Label)
	if letter != wslMeasureKey {
		t.Errorf("the measuring row declares %q; the footer and the handler use %q", letter, wslMeasureKey)
	}

	afterEnter, cmd := pressWSLMsg(t, m, wslEnter)
	if cmd != nil {
		t.Error("Enter asked for the measurement; measuring is a command and has its own letter")
	}
	if afterEnter.WSLAdvisor.Benchmark.Running {
		t.Error("Enter started the measurement")
	}

	started, cmd := pressWSLMsg(t, m, wslLetterKey(letter))
	if cmd == nil {
		t.Fatalf("the letter the measuring row declares (%q) asked for nothing", letter)
	}
	if !started.WSLAdvisor.Benchmark.Running {
		t.Fatalf("the letter the measuring row declares (%q) did not start the measurement", letter)
	}

	// A second press while the run is in flight must not spend the disk again.
	again, cmd := pressWSLMsg(t, started, wslLetterKey(letter))
	if cmd != nil {
		t.Error("the letter started a second measurement while one was running")
	}
	if !again.WSLAdvisor.Benchmark.Running {
		t.Error("the second press dropped the run that was in flight")
	}

	footer := wslFooter(t, m)
	if !strings.Contains(footer, "[m] measure") {
		t.Errorf("the footer does not name the letter the measuring row carries: %q", footer)
	}
	if strings.Contains(footer, "[Enter] select") || strings.Contains(footer, "Space") {
		t.Errorf("the footer offers a key this screen does not accept: %q", footer)
	}
}

// wslMigrationFrameModel is the copy screen at one phase over the pinned reading, with the rows and
// the cursor the screen would have reached by walking there.
func wslMigrationFrameModel(t *testing.T, phase wslMigrationPhase) Model {
	t.Helper()

	m := wslAdvisorFrameModel(t, ScreenWSLMigration)
	state := m.WSLAdvisor.Migration
	state.Phase = phase
	m.WSLAdvisor.Migration = state
	m.Cursor = 0
	if phase == wslMigratePickSource || phase == wslMigrateDone {
		m.Cursor = 0
	}
	return m
}

// TestWSLMigrationRunsEachActionOnItsOwnLetter pins the copy screen phase by phase: every command
// row carries a letter, that letter is the key that acts, and Enter on the same row does nothing.
// The preview's cancel and the copy's stop are different letters, because one letter never carries
// two commands on one screen.
func TestWSLMigrationRunsEachActionOnItsOwnLetter(t *testing.T) {
	t.Run("the preview copies on c and cancels on x", func(t *testing.T) {
		m := wslMigrationFrameModel(t, wslMigratePreview)
		rows := m.wslMigrationRows()

		confirm := wslRowKey(t, rows[0].Label)
		if confirm != wslCopyKey {
			t.Errorf("the confirming row declares %q; the footer and the handler use %q", confirm, wslCopyKey)
		}
		afterEnter, cmd := pressWSLMsg(t, m, wslEnter)
		if cmd != nil || afterEnter.WSLAdvisor.Migration.Running {
			t.Error("Enter started the copy; copying is a command and has its own letter")
		}
		started, cmd := pressWSLMsg(t, m, wslLetterKey(confirm))
		if cmd == nil || !started.WSLAdvisor.Migration.Running || started.WSLAdvisor.Migration.Cancel == nil {
			t.Errorf("the letter the confirming row declares (%q) did not start the copy", confirm)
		}

		// The cancel row is its own letter, and it puts the choice of destination back.
		cancelRow := m
		cancelRow.Cursor = 1
		if got := wslRowKey(t, rows[1].Label); got != wslCancelKey {
			t.Errorf("the cancel row declares %q; the footer and the handler use %q", got, wslCancelKey)
		}
		afterCancelEnter, _ := pressWSLMsg(t, cancelRow, wslEnter)
		if afterCancelEnter.WSLAdvisor.Migration.Phase != wslMigratePreview {
			t.Error("Enter cancelled the preview; cancelling is a command and has its own letter")
		}
		afterCancel, _ := pressWSLMsg(t, cancelRow, wslLetterKey(wslCancelKey))
		if afterCancel.WSLAdvisor.Migration.Phase != wslMigratePickDestination {
			t.Errorf("the cancel letter left the screen at phase %v, want the destination question",
				afterCancel.WSLAdvisor.Migration.Phase)
		}

		footer := wslFooter(t, m)
		if !strings.Contains(footer, "[c] copy") || !strings.Contains(footer, "[x] cancel") {
			t.Errorf("the footer does not name the letters the preview's rows carry: %q", footer)
		}
	})

	t.Run("a copy in flight stops on s and Escape", func(t *testing.T) {
		m := wslMigrationFrameModel(t, wslMigrateCopying)
		cancelled := 0
		m.WSLAdvisor.Migration.Running = true
		m.WSLAdvisor.Migration.Cancel = func() { cancelled++ }

		rows := m.wslMigrationRows()
		stop := wslRowKey(t, rows[0].Label)
		if stop != wslStopKey {
			t.Errorf("the stop row declares %q; the footer and the handler use %q", stop, wslStopKey)
		}
		afterEnter, _ := pressWSLMsg(t, m, wslEnter)
		if cancelled != 0 {
			t.Error("Enter stopped the copy; stopping is a command and has its own letter")
		}
		if !afterEnter.WSLAdvisor.Migration.Running {
			t.Error("Enter ended the copy it did not stop")
		}

		afterLetter, _ := pressWSLMsg(t, m, wslLetterKey(stop))
		if cancelled != 1 {
			t.Errorf("the letter the stop row declares (%q) stopped the copy %d times, want once", stop, cancelled)
		}
		if !afterLetter.WSLAdvisor.Migration.Running {
			// The copy is still running until its own report lands; the press only asked for the
			// stop. Losing the flag here would make the screen claim an outcome it has not read.
			t.Error("the stop press dropped the run before its report arrived")
		}

		// Escape is the same stop here, and the footer says so instead of offering the way back.
		afterEscape, _ := pressWSLMsg(t, m, wslEscape)
		if cancelled != 2 {
			t.Errorf("Escape stopped the copy %d times, want twice", cancelled)
		}
		if afterEscape.Screen != ScreenWSLMigration {
			t.Errorf("Escape left the screen (%v) instead of stopping the copy", afterEscape.Screen)
		}
		footer := wslFooter(t, m)
		if !strings.Contains(footer, "[Esc] stop the copy") {
			t.Errorf("the footer does not say Escape stops the copy while one runs: %q", footer)
		}
		if !strings.Contains(footer, "[s] stop the copy") {
			t.Errorf("the footer does not name the letter the stop row carries: %q", footer)
		}
	})

	t.Run("the outcome starts again on a", func(t *testing.T) {
		m := wslMigrationFrameModel(t, wslMigrateDone)
		rows := m.wslMigrationRows()

		if got := wslRowKey(t, rows[0].Label); got != wslAnotherKey {
			t.Errorf("the restart row declares %q; the footer and the handler use %q", got, wslAnotherKey)
		}
		afterEnter, _ := pressWSLMsg(t, m, wslEnter)
		if afterEnter.WSLAdvisor.Migration.Phase != wslMigrateDone {
			t.Error("Enter started the flow again; that is a command and has its own letter")
		}
		afterLetter, _ := pressWSLMsg(t, m, wslLetterKey(wslAnotherKey))
		if afterLetter.WSLAdvisor.Migration.Phase != wslMigratePickSource {
			t.Errorf("the restart letter left the screen at phase %v, want the source question",
				afterLetter.WSLAdvisor.Migration.Phase)
		}
		if !strings.Contains(wslFooter(t, m), "[a] copy another") {
			t.Errorf("the footer does not name the letter the restart row carries: %q", wslFooter(t, m))
		}
	})

	t.Run("the two lists choose on Enter and offer no letter", func(t *testing.T) {
		m := wslMigrationFrameModel(t, wslMigratePickSource)
		rows := m.wslMigrationRows()
		if len(rows) == 0 || rows[0].Kind != wslActionPick {
			t.Fatalf("the source question drew %v, want a list of projects", wslActionRowLabels(rows))
		}
		after, _ := pressWSLMsg(t, m, wslEnter)
		if after.WSLAdvisor.Migration.Phase != wslMigratePickDestination {
			t.Errorf("Enter on a project left the screen at phase %v, want the destination question",
				after.WSLAdvisor.Migration.Phase)
		}
		footer := wslFooter(t, m)
		if !strings.Contains(footer, "[Enter] choose") {
			t.Errorf("the footer does not name Enter as the choose it is: %q", footer)
		}
		if strings.Contains(footer, "Space") {
			t.Errorf("the footer offers Space on a screen with no switch: %q", footer)
		}
	})
}

// TestWSLAssistantCommandLettersAreUniqueAndNotMoveKeys pins the two rules the letters obey on every
// phase of both assistant screens: no command runs on j, k, h or l, no letter carries two commands,
// and each footer hint spells the same letter the row and the handler use.
func TestWSLAssistantCommandLettersAreUniqueAndNotMoveKeys(t *testing.T) {
	pairs := []struct {
		action string
		letter string
		hint   installerHint
	}{
		{"measure", wslMeasureKey, hintWSLMeasure},
		{"copy", wslCopyKey, hintWSLCopyNow},
		{"cancel", wslCancelKey, hintWSLCancelCopy},
		{"stop the copy", wslStopKey, hintWSLStopCopy},
		{"copy another project", wslAnotherKey, hintWSLAnotherCopy},
	}

	seen := map[string]string{}
	for _, pair := range pairs {
		if strings.ContainsAny(pair.letter, "jkh l") {
			t.Errorf("%s runs on %q, which is a move key", pair.action, pair.letter)
		}
		if other, taken := seen[pair.letter]; taken {
			t.Errorf("%q carries two commands on these screens: %s and %s", pair.letter, other, pair.action)
		}
		seen[pair.letter] = pair.action
		if want := "[" + pair.letter + "]"; pair.hint.keys != want {
			t.Errorf("the footer spells %q for %s while the row and the handler use %q",
				pair.hint.keys, pair.action, pair.letter)
		}
	}

	// Every letter a row of either assistant screen declares is one the same screen's footer names,
	// so the footer never names a key the handler does not accept and never omits one it does.
	for _, phase := range []wslMigrationPhase{
		wslMigratePickSource, wslMigratePickDestination, wslMigratePreview, wslMigrateCopying, wslMigrateDone,
	} {
		m := wslMigrationFrameModel(t, phase)
		if phase == wslMigrateCopying {
			m.WSLAdvisor.Migration.Running = true
		}
		footer := strings.Join(wslMigrationHelpHints(m.wslMigrationHints()), " ")
		for _, row := range m.wslMigrationRows() {
			if !strings.HasPrefix(row.Label, "[") {
				continue
			}
			letter := wslRowKey(t, row.Label)
			if !strings.Contains(footer, "["+letter+"]") {
				t.Errorf("the row %q carries %q and the footer of phase %d does not name it: %q",
					row.Label, letter, phase, footer)
			}
		}
	}

	footer := strings.Join(wslMigrationHelpHints(wslAdvisorFrameModel(t, ScreenWSLBenchmark).wslBenchmarkHints()), " ")
	for _, row := range wslAdvisorFrameModel(t, ScreenWSLBenchmark).wslBenchmarkRows() {
		if !strings.HasPrefix(row.Label, "[") {
			continue
		}
		letter := wslRowKey(t, row.Label)
		if !strings.Contains(footer, "["+letter+"]") {
			t.Errorf("the measuring row carries %q and the footer does not name it: %q", letter, footer)
		}
	}
}

// wslMigrationHelpHints renders the hints of one footer as the words a reader sees, so a test can
// look for the letter a row declares without depending on the styling the frame applies.
func wslMigrationHelpHints(hints []installerHint) []string {
	rendered := make([]string, 0, len(hints))
	for _, hint := range hints {
		rendered = append(rendered, hint.keys+" "+hint.verb)
	}
	return rendered
}

// TestTheBenchmarkStopsOnEscapeWhileItRuns pins rule 4 of the key grammar on the
// measurement screen: Escape stops a run in flight -- a measurement holds the disk
// while it runs -- and with nothing running it is the way back.
func TestTheBenchmarkStopsOnEscapeWhileItRuns(t *testing.T) {
	m := wslAdvisorFrameModel(t, ScreenWSLBenchmark)
	cancelled := false
	m.WSLAdvisor.Benchmark = wslBenchmarkState{Running: true, cancel: func() { cancelled = true }}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	stopped := next.(Model)
	if cmd != nil {
		t.Error("Escape issued a command")
	}
	if !cancelled {
		t.Fatal("Escape did not cancel the measurement in flight")
	}
	if stopped.Screen != ScreenWSLBenchmark {
		t.Fatalf("Escape stopped the run and left the screen on %v", stopped.Screen)
	}
	if !stopped.WSLAdvisor.Benchmark.Running {
		t.Error("Escape closed the run before its report landed")
	}
	if !strings.Contains(stopped.WSLAdvisor.Benchmark.Notice, "Stopping") {
		t.Errorf("Escape said nothing about stopping: %q", stopped.WSLAdvisor.Benchmark.Notice)
	}

	// The footer names the stop while the run is in flight.
	footer := strings.Join(wslMigrationHelpHints(stopped.wslBenchmarkHints()), " ")
	if !strings.Contains(footer, "[Esc] stop the measurement") {
		t.Errorf("the footer while measuring does not name the stop: %q", footer)
	}

	// With nothing running, Escape is the way back.
	idle := wslAdvisorFrameModel(t, ScreenWSLBenchmark)
	back, _ := idle.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if got := back.(Model).Screen; got != ScreenWSLAdvisor {
		t.Errorf("Escape with no run went to %v, want the report", got)
	}
}

// TestWSLScreenFootersFitOneRowAtTheFloor pins what the letters cost: each of the four WSL screens'
// footers still packs into one row at 80 columns, which is the row the list needs on the narrowest
// terminal the installer supports. A footer that grew to two rows would take a row of the list away
// on every one of these screens, and nothing else would say so.
func TestWSLScreenFootersFitOneRowAtTheFloor(t *testing.T) {
	type footerCase struct {
		name  string
		hints []installerHint
	}

	report := wslAdvisorFrameModel(t, ScreenWSLAdvisor)
	measurement := wslAdvisorFrameModel(t, ScreenWSLBenchmark)
	cases := []footerCase{
		{"the report", report.wslAdvisorHints()},
		{"the measurement", measurement.wslBenchmarkHints()},
	}
	for _, phase := range []wslMigrationPhase{
		wslMigratePickSource, wslMigratePickDestination, wslMigratePreview, wslMigrateCopying, wslMigrateDone,
	} {
		m := wslMigrationFrameModel(t, phase)
		if phase == wslMigrateCopying {
			m.WSLAdvisor.Migration.Running = true
		}
		cases = append(cases, footerCase{"the copy at its " + itoaWSLPhase(phase) + " question", m.wslMigrationHints()})
	}

	for _, footer := range cases {
		// The footer is measured on the frame it is drawn in: the width the frame gives a body at 80
		// columns, which is the narrowest terminal the installer supports.
		m := wslAdvisorFrameModel(t, ScreenWSLAdvisor)
		m.Width, m.Height = 80, 24
		inner := contentWidth(m)
		if rows := footerRowCount(inner, footer.hints); rows > 1 {
			t.Errorf("%s spends %d footer rows at 80 columns, and the list loses the second:\n%s",
				footer.name, rows, strings.Join(footerHints(inner, footer.hints), "\n"))
		}
	}
}

// itoaWSLPhase names a copy phase for a subtest's own label.
func itoaWSLPhase(phase wslMigrationPhase) string {
	switch phase {
	case wslMigratePickSource:
		return "source"
	case wslMigratePickDestination:
		return "destination"
	case wslMigrateReadingSource:
		return "reading"
	case wslMigratePreview:
		return "preview"
	case wslMigrateCopying:
		return "copying"
	case wslMigrateDone:
		return "done"
	}
	return "unknown"
}
