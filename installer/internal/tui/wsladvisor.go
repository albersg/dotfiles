package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE WSL PERFORMANCE ADVISOR: WHAT THE ENVIRONMENT COSTS, AND WHAT IS SAFE TO DO
// ============================================================================
//
// This is the utility that only exists inside a WSL instance. It answers one question -- where do the
// files I work on actually live, and what does that cost me -- and then a few smaller ones around it:
// how the instance's memory and swap are behaving, whether a .wslconfig change is still waiting for a
// restart, whether DNS goes through the Windows host, whether interop still works, and how much room
// is left on the distribution's own disk.
//
// Three rules hold for everything in this file:
//
//   - Nothing is inferred about the Windows host that was not read from somewhere inside the
//     distribution. The mount table, /proc/meminfo, /etc/wsl.conf, /etc/resolv.conf and the
//     .wslconfig the installer already reads are facts; "the host probably has antivirus" is not.
//   - Every finding carries its evidence and says whether it came from a measurement taken here or
//     from documented behaviour (wslOrigin). A recommendation may not be phrased as a measurement.
//   - Nothing is changed. The advisor writes no configuration file, edits no mount, disables nothing
//     and touches no virtual disk. The two rows that act -- the benchmark and the assisted migration
//     -- are separate screens with their own explicit confirmation, and the migration only ever
//     copies.
//
// The severity ladder is small on purpose. "Critical" is reserved for the one case that costs time on
// every operation a developer runs; "warning" for a real cost that is not on that path; "notice" for
// something worth knowing that may well be intentional; "info" for a fact that needs no action.

// wslSeverity says how much a finding matters.
type wslSeverity int

const (
	// wslSeverityInfo is a fact with no action attached: the healthy case is stated, not omitted.
	wslSeverityInfo wslSeverity = iota
	// wslSeverityNotice is worth knowing and may be intentional.
	wslSeverityNotice
	// wslSeverityWarning is a real cost or a real gap that has an action.
	wslSeverityWarning
	// wslSeverityCritical is paid on every file operation the developer's tools run.
	wslSeverityCritical
)

// String names the severity the way a row shows it.
func (s wslSeverity) String() string {
	switch s {
	case wslSeverityCritical:
		return "Critical"
	case wslSeverityWarning:
		return "Warning"
	case wslSeverityNotice:
		return "Notice"
	default:
		return "Info"
	}
}

// rank orders findings from the one that matters most to the one that matters least, so the list the
// screen shows never buries a critical row under an informational one.
func (s wslSeverity) rank() int { return -int(s) }

// wslScope says where a recommended change would have to be made, which is the difference between an
// action the user can take here and one that is not ours to take at all.
type wslScope int

const (
	// wslScopeReadOnly is a finding with nothing to change.
	wslScopeReadOnly wslScope = iota
	// wslScopeDistribution is a change made inside the distribution, by the user, in its own files.
	wslScopeDistribution
	// wslScopeRestart is a change that only takes effect when WSL restarts the instance, so the
	// window it is made in keeps behaving as it did.
	wslScopeRestart
	// wslScopeHost is a change on the Windows side. This program cannot make it from inside the
	// distribution and must not pretend otherwise.
	wslScopeHost
)

// String names the scope in the words a finding shows.
func (s wslScope) String() string {
	switch s {
	case wslScopeDistribution:
		return "changed inside this distribution"
	case wslScopeRestart:
		return "changed inside this distribution, and applied only after WSL restarts it"
	case wslScopeHost:
		return "changed on the Windows side, by you"
	default:
		return "nothing to change"
	}
}

// wslOrigin says where the claim came from: a measurement taken on this machine now, or documented
// behaviour of WSL. The two are never mixed in one sentence without being named, because a documented
// recommendation dressed as a measurement is the failure this field exists to prevent.
type wslOrigin int

const (
	// wslOriginMeasured is what the probes in this run read from this machine.
	wslOriginMeasured wslOrigin = iota
	// wslOriginDocumented is how WSL behaves according to its own documentation, applied to what was
	// measured here.
	wslOriginDocumented
)

// String names the origin.
func (o wslOrigin) String() string {
	if o == wslOriginDocumented {
		return "documented behaviour"
	}
	return "measured here"
}

// wslFinding is one thing the advisor found. Every field is required except Benefit and Action, which
// a finding with nothing to do legitimately leaves empty.
type wslFinding struct {
	// ID is a stable name, so a test asserts the finding it means without depending on the wording.
	ID string
	// Severity is what it costs.
	Severity wslSeverity
	// Title is the row the list shows.
	Title string
	// Evidence is what was read: a path, a mount, a number. It is never a conclusion.
	Evidence string
	// Why explains the consequence in plain words; it must not promise a number this run did not
	// measure.
	Why string
	// Benefit is the qualitative gain from acting, empty when there is nothing to act on.
	Benefit string
	// Action is what to do, empty when the finding is informational.
	Action string
	// Scope says where the change belongs and whether it waits for a restart.
	Scope wslScope
	// Origin says whether the claim was measured or is documented.
	Origin wslOrigin
}

// ============================================================================
// THE FACTS A REPORT IS BUILT FROM
// ============================================================================

// wslProjectFacts is what was read about one directory that may be a project. Only metadata is read:
// whether it is a Git repository, which dependency or build directories sit directly inside it, and
// which filesystem it lives on. No file inside a project is opened, and no file name inside one is
// reported, because the report has no use for either.
type wslProjectFacts struct {
	// Path is the directory as the user would name it.
	Path string
	// Label is the path with $HOME abbreviated, which is what a row shows.
	Label string
	// Storage is the verdict the mount table produced for it.
	Storage system.StorageVerdict
	// GitRepo reports a .git entry directly inside it.
	GitRepo bool
	// Marker is the entry that made it look like a project, e.g. "go.mod" or "node_modules".
	Marker string
	// DependencyDirs are the dependency or build directories directly inside it, by their own names.
	DependencyDirs []string
	// LinkNotes describe symlinks directly inside it whose target lives on other storage, up to
	// wslLinkedEntryLimit of them.
	LinkNotes []string
	// Err is why this directory could not be read, empty when it could.
	Err string
}

// wslMemoryFacts is /proc/meminfo, as far as this report reads it.
type wslMemoryFacts struct {
	// Read reports whether the file was read at all.
	Read bool
	// Err is why it was not, empty when it was.
	Err string
	// TotalBytes, UsedBytes, SwapTotalBytes and SwapFreeBytes are zero when the field was absent.
	TotalBytes, UsedBytes, SwapTotalBytes, SwapFreeBytes uint64
	// HaveTotal, HaveUsed, HaveSwap report which of them the file actually carried.
	HaveTotal, HaveUsed, HaveSwap bool
}

// wslDiskFacts is the space behind one path.
type wslDiskFacts struct {
	Path       string
	Known      bool
	FreeBytes  uint64
	TotalBytes uint64
	Err        string
}

// wslResolvFacts is what /etc/resolv.conf says, and whether WSL wrote it.
type wslResolvFacts struct {
	// Path is the file that was read.
	Path string
	// Read reports whether it could be read.
	Read bool
	// Err is why it was not.
	Err string
	// LinkTarget is where the path points when it is a symlink, which is how a distribution that
	// manages its own resolver is recognised.
	LinkTarget string
	// GeneratedHeader reports the file says WSL generated it, which WSL writes into the file it
	// creates.
	GeneratedHeader bool
	// Nameservers are the nameserver addresses it lists, in order.
	Nameservers []string
	// SearchDomains are the search entries it lists, in order.
	SearchDomains []string
}

// wslAdvisorFacts is everything the findings are derived from. It holds the raw readings, never a
// conclusion, so wslAdvisorFindings stays a pure function of it and a test can build the machine it
// wants to talk about.
type wslAdvisorFacts struct {
	// Runtime is the strict in-WSL answer, with the evidence it was decided on.
	Runtime system.WSLRuntime
	// Cwd is the directory the installer was started in, which is the first thing analysed.
	Cwd string
	// Table is the mount table as it was read, including its read error.
	Table system.MountTable
	// Conf is /etc/wsl.conf as it was read, including the difference between a key it sets and one
	// WSL defaults.
	Conf system.WSLConf
	// Home is $HOME, used to abbreviate paths in rows and to find the native destinations a project
	// could move to.
	Home string
	// LocalProject is the current directory when it looks like a project (or is simply where the user
	// is), and Projects are the other candidate project directories found beside it.
	LocalProject wslProjectFacts
	Projects     []wslProjectFacts
	// Memory is /proc/meminfo.
	Memory wslMemoryFacts
	// CPUs is the number of logical processors this instance sees.
	CPUs int
	// CPUsErr is why the count could not be read, empty when it could.
	CPUsErr string
	// NativeParents are the places a copy could land, read here so the assisted copy cannot offer a
	// destination this report did not classify.
	NativeParents []wslNativeParent
	// RootDisk is the space behind the distribution's own root.
	RootDisk wslDiskFacts
	// ProjectDisk is the space behind the current directory.
	ProjectDisk wslDiskFacts
	// Resolv is /etc/resolv.conf.
	Resolv wslResolvFacts
	// WindowsConfigPath is the .wslconfig the installer already reads, and WindowsConfig is what that
	// file sets today. Both are filled from the WSL resources state, which reads the same file with
	// the same lookup: a second reader is how the two screens come to disagree about one file.
	WindowsConfigPath string
	WindowsConfig     *WSLResources
	WindowsConfigErr  string
	// Limits are the readings that could not be taken, in the report's own words. They are shown, so
	// an empty section is never mistaken for a healthy one.
	Limits []string
}

// wslAdvisorInput is where the facts are read from. It is a value rather than direct calls so the
// report can be built for a machine that is not this one, and so the probes stay in one place.
type wslAdvisorInput struct {
	Cwd          string
	Runtime      system.WSLRuntime
	ReadMounts   func() system.MountTable
	ReadConf     func() system.WSLConf
	ReadMeminfo  func() ([]byte, error)
	ReadResolv   func() (path string, data []byte, linkTarget string, err error)
	NumCPU       func() int
	Statfs       func(path string) (free, total uint64, ok bool)
	ReadDir      func(path string) ([]os.DirEntry, error)
	EvalSymlinks func(path string) (string, error)
	Exists       func(path string) bool
	// Getenv reads the environment this report is about, so the home directory a row abbreviates and
	// the discovery walks is part of the input rather than of the machine running the code.
	Getenv     func(string) string
	ConfigPath string
	Config     *WSLResources
	ConfigErr  string
}

// defaultWSLAdvisorInput reads the real machine. The .wslconfig facts are passed in rather than read
// again: the utilities section already resolved that file, through the lookup that honours the same
// environment override, and this report must not disagree with the screen beside it.
func defaultWSLAdvisorInput(cwd string, cfgPath string, cfg *WSLResources, cfgErr string) wslAdvisorInput {
	return wslAdvisorInput{
		Cwd:        cwd,
		Runtime:    system.DetectWSLRuntime(),
		ReadMounts: system.ReadMountTable,
		ReadConf: func() system.WSLConf {
			// The same override the installation step honours, so a test and a distribution that moved
			// the file read the one the installer would write.
			path := os.Getenv(envWSLConfPath)
			if path == "" {
				path = defaultWSLConfPath
			}
			return system.ReadWSLConfFrom(path)
		},
		ReadMeminfo: func() ([]byte, error) { return os.ReadFile(procMeminfoPath) },
		ReadResolv:  readResolvConfFacts,
		NumCPU:      runtimeNumCPU,
		Statfs:      system.StorageSpace,
		ReadDir:     os.ReadDir,
		EvalSymlinks: func(path string) (string, error) {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return path, err
			}
			return resolved, nil
		},
		Exists: func(path string) bool {
			_, err := os.Lstat(path)
			return err == nil
		},
		Getenv:     os.Getenv,
		ConfigPath: cfgPath,
		Config:     cfg,
		ConfigErr:  cfgErr,
	}
}

// procMeminfoPath is where the kernel reports the memory this instance sees. It is the instance's
// own view: the Windows host's total memory is not readable from here and is never guessed at.
const procMeminfoPath = "/proc/meminfo"

// resolvConfPath is the file the resolver reads.
const resolvConfPath = "/etc/resolv.conf"

// runtimeNumCPU is a variable so the report can be built for a machine with a different processor
// count than the one running the test. The count is the instance's own: WSL shows the distribution
// the limit it was given, not the Windows host's total.
var runtimeNumCPU = defaultRuntimeNumCPU

// defaultRuntimeNumCPU is the real reading.
func defaultRuntimeNumCPU() int { return runtime.NumCPU() }

// collectWSLAdvisorFacts reads everything the report talks about, in one pass, off the update loop.
func collectWSLAdvisorFacts(in wslAdvisorInput) wslAdvisorFacts {
	facts := wslAdvisorFacts{
		Runtime:           in.Runtime,
		Cwd:               in.Cwd,
		Home:              in.Getenv("HOME"),
		WindowsConfigPath: in.ConfigPath,
		WindowsConfig:     in.Config,
	}
	if in.ConfigErr != "" {
		facts.Limits = append(facts.Limits, "The Windows-side .wslconfig could not be read: "+in.ConfigErr)
	}

	facts.Table = in.ReadMounts()
	if facts.Table.Err != "" {
		facts.Limits = append(facts.Limits, "The mount table could not be read ("+facts.Table.Err+
			"), so no path could be classified.")
	}
	facts.Conf = in.ReadConf()
	if facts.Conf.Err != "" {
		facts.Limits = append(facts.Limits, "The distribution's own /etc/wsl.conf could not be read: "+
			facts.Conf.Err)
	}

	if data, err := in.ReadMeminfo(); err != nil {
		facts.Memory.Err = err.Error()
		facts.Limits = append(facts.Limits, "The instance's memory could not be read: "+err.Error())
	} else {
		facts.Memory = wslMemoryFactsFrom(data)
		if !facts.Memory.Read {
			facts.Limits = append(facts.Limits, "The instance's memory could not be read: "+procMeminfoPath+
				" carried no value for it.")
		}
	}

	facts.CPUs = in.NumCPU()
	if facts.CPUs <= 0 {
		facts.CPUsErr = "the instance reported no processor count"
		facts.Limits = append(facts.Limits, "The instance's processor count could not be read.")
	}

	resolvPath, resolvData, linkTarget, resolvErr := in.ReadResolv()
	facts.Resolv = wslResolvFactsFrom(resolvPath, resolvData, linkTarget, resolvErr)

	root := facts.NativeRootPath()
	facts.RootDisk = wslDiskFactsFrom(in, root)
	facts.ProjectDisk = wslDiskFactsFrom(in, facts.Cwd)

	facts.NativeParents = wslNativeParents(in, facts)
	facts.LocalProject = readWSLProjectFacts(in, facts.Cwd, "")
	facts.Projects = discoverWSLProjects(in, facts)

	return facts
}

// wslMemoryFactsFrom reads the four memory fields this report uses.
func wslMemoryFactsFrom(data []byte) wslMemoryFacts {
	memory := wslMemoryFacts{Read: len(data) > 0}
	memory.TotalBytes, memory.HaveTotal = system.MeminfoBytes(data, "MemTotal")
	if used, total, ok := system.MeminfoUsed(data); ok {
		memory.UsedBytes, memory.TotalBytes, memory.HaveUsed = used, total, true
	}
	memory.SwapTotalBytes, memory.HaveSwap = system.MeminfoBytes(data, "SwapTotal")
	if memory.HaveSwap {
		if free, ok := system.MeminfoBytes(data, "SwapFree"); ok {
			memory.SwapFreeBytes = free
		}
	}
	return memory
}

// wslDiskFactsFrom reads the space behind a path. A path that could not be read keeps the reason, so
// the report shows the gap rather than a zero.
func wslDiskFactsFrom(in wslAdvisorInput, path string) wslDiskFacts {
	facts := wslDiskFacts{Path: path}
	if path == "" {
		facts.Err = "no path to read"
		return facts
	}
	free, total, ok := in.Statfs(path)
	if !ok {
		facts.Err = "the filesystem behind " + path + " did not answer"
		return facts
	}
	facts.Known, facts.FreeBytes, facts.TotalBytes = true, free, total
	return facts
}

// readResolvConfFacts reads the resolver file and reports where it came from. The symlink target is
// read with Lstat rather than followed: a distribution that manages its own resolver does it through
// a link to systemd-resolved's stub, and that link is the difference between a file WSL writes and
// one something else maintains.
func readResolvConfFacts() (path string, data []byte, linkTarget string, err error) {
	path = resolvConfPath
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		if target, readErr := os.Readlink(path); readErr == nil {
			linkTarget = target
		}
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return path, nil, linkTarget, err
	}
	return path, data, linkTarget, nil
}

// wslResolvFactsFrom parses the resolver file. It reads the two line kinds that matter -- nameserver
// and search -- and reports whether WSL's generator wrote the file, from the file's own header.
func wslResolvFactsFrom(path string, data []byte, linkTarget string, err error) wslResolvFacts {
	facts := wslResolvFacts{Path: path, LinkTarget: linkTarget}
	if err != nil {
		facts.Err = err.Error()
		return facts
	}
	facts.Read = true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			if strings.Contains(strings.ToLower(line), "generated by") {
				facts.GeneratedHeader = true
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "nameserver":
			facts.Nameservers = append(facts.Nameservers, fields[1])
		case "search":
			facts.SearchDomains = append(facts.SearchDomains, fields[1:]...)
		}
	}
	return facts
}

// NativeRootPath is where the distribution's own storage begins, used as the destination root for a
// suggested move. It is the root mount's mount point, which is "/" on every distribution; the mount
// entry is what proves the filesystem behind it is the distribution's own rather than a host share.
func (f wslAdvisorFacts) NativeRootPath() string {
	if entry, ok := f.Table.NativeRoot(); ok {
		return entry.MountPoint
	}
	return ""
}

// NativeStorageName names the distribution's own filesystem for a row, e.g. "ext4 at /". It is empty
// when the root mount could not be read, so a row is left out rather than filled with a guess.
func (f wslAdvisorFacts) NativeStorageName() string {
	entry, ok := f.Table.NativeRoot()
	if !ok {
		return ""
	}
	return entry.FileSystem + " at " + entry.MountPoint
}

// ============================================================================
// THE PROJECTS BESIDE THE CURRENT DIRECTORY
// ============================================================================

const (
	// wslAdvisorProjectLimit caps how many candidate project directories are reported. It is a bound
	// on reading rather than a preference: a home directory can hold hundreds of directories, and a
	// report that walks them all is a report that takes a minute to open.
	wslAdvisorProjectLimit = 12
	// wslAdvisorScanLimit caps how many entries of one directory are examined while looking for
	// projects. Names are read; nothing else is.
	wslAdvisorScanLimit = 200
	// wslLinkedEntryLimit caps how many symlinks are followed and reported per project.
	wslLinkedEntryLimit = 8
)

// wslProjectMarkers are the entries that make a directory look like a project, with the marker that
// was found reported as the evidence. They are all metadata checks: one Lstat each.
var wslProjectMarkers = []string{
	".git", "go.mod", "package.json", "pyproject.toml", "Cargo.toml", "composer.json",
	"Gemfile", "pom.xml", "build.gradle", "CMakeLists.txt", "Makefile",
}

// wslDependencyDirs are the dependency and build directories worth naming. A project sitting on host
// storage with a node_modules directory pays the boundary per file, and a migration preview needs to
// know it is there: it is the largest single candidate for an exclusion.
var wslDependencyDirs = []string{
	"node_modules", ".venv", "venv", "target", "build", "dist", ".next", ".nuxt", "out",
	".tox", "vendor", "bower_components", "__pycache__", ".pytest_cache", ".gradle",
}

// readWSLProjectFacts reads the metadata of one directory: whether it is a Git repository, which
// dependency directories it holds, and which symlinks immediately inside it land on other storage.
// A directory that cannot be read keeps the reason instead of an empty verdict.
func readWSLProjectFacts(in wslAdvisorInput, path, label string) wslProjectFacts {
	project := wslProjectFacts{Path: path, Label: label}
	if project.Label == "" {
		project.Label = abbreviateHome(path, in.Getenv("HOME"))
	}

	resolved, resolveErr := in.EvalSymlinks(path)
	if resolveErr != nil {
		// A path that cannot be resolved is still classified where it was given: refusing to answer
		// because a link is broken would hide the storage question behind an unrelated one.
		resolved = path
	}
	project.Storage = system.ClassifyStorage(in.mountTable(), path, resolved)
	if project.Storage.ResolvedPath != path {
		project.LinkNotes = append(project.LinkNotes, path+" is a link to "+project.Storage.ResolvedPath)
	}

	entries, err := in.DirEntries(path)
	if err != nil {
		project.Err = err.Error()
		return project
	}

	links := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == ".git" {
				project.GitRepo = true
			}
			if wslListHas(wslDependencyDirs, entry.Name()) {
				project.DependencyDirs = append(project.DependencyDirs, entry.Name())
			}
			continue
		}
		// A .git entry that is a file is a linked worktree or a submodule pointer; either way the
		// directory is a repository, and either way it is copied whole rather than reconstructed.
		if entry.Name() == ".git" {
			project.GitRepo = true
		}
		if entry.Type()&os.ModeSymlink == 0 || links >= wslLinkedEntryLimit {
			continue
		}
		links++
		if note := wslLinkStorageNote(in, path, entry.Name(), project.Storage.Kind); note != "" {
			project.LinkNotes = append(project.LinkNotes, note)
		}
	}
	return project
}

// wslLinkStorageNote describes a symlink inside a project whose target lives on other storage. That
// is worth reporting because the link's cost is the target's cost: a dependency directory linked onto
// a Windows drive is paid for by every read through the link.
func wslLinkStorageNote(in wslAdvisorInput, projectPath, name string, projectKind system.StorageKind) string {
	linkPath := filepath.Join(projectPath, name)
	resolved, err := in.EvalSymlinks(linkPath)
	if err != nil || resolved == linkPath {
		return ""
	}
	verdict := system.ClassifyStorage(in.mountTable(), linkPath, resolved)
	if verdict.Kind == system.StorageUnknown || verdict.Kind == projectKind {
		return ""
	}
	return linkPath + " points at " + resolved + ", which is " + verdict.Kind.String()
}

// discoverWSLProjects finds the candidate project directories beside the current one: the children
// of the current directory first, then the children of the home directory. Only directories that
// carry a project marker are reported, and the scan stops at the limits above.
func discoverWSLProjects(in wslAdvisorInput, facts wslAdvisorFacts) []wslProjectFacts {
	var projects []wslProjectFacts
	seen := map[string]bool{facts.Cwd: true}

	roots := []string{facts.Cwd}
	if facts.Home != "" && facts.Home != facts.Cwd {
		roots = append(roots, facts.Home)
	}

	for _, root := range roots {
		entries, err := in.DirEntries(root)
		if err != nil {
			continue
		}
		if len(entries) > wslAdvisorScanLimit {
			entries = entries[:wslAdvisorScanLimit]
		}
		for _, entry := range entries {
			if len(projects) >= wslAdvisorProjectLimit {
				return projects
			}
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if seen[path] || path == facts.Cwd {
				continue
			}
			seen[path] = true

			// The marker is what makes it a candidate, and the marker is the evidence a row shows.
			marker := ""
			for _, candidate := range wslProjectMarkers {
				if in.Exists(filepath.Join(path, candidate)) {
					marker = candidate
					break
				}
			}
			if marker == "" {
				continue
			}
			project := readWSLProjectFacts(in, path, "")
			project.Marker = marker
			projects = append(projects, project)
		}
	}
	return projects
}

// ============================================================================
// FINDINGS
// ============================================================================

// wslAdvisorFindings turns the readings into findings. It is pure: every fact it uses was read by
// collectWSLAdvisorFacts, so a test can describe any machine as a value and assert what the advisor
// would say about it.
func wslAdvisorFindings(facts wslAdvisorFacts) []wslFinding {
	var findings []wslFinding
	findings = append(findings, wslStorageFindings(facts)...)
	findings = append(findings, wslMemoryFindings(facts)...)
	findings = append(findings, wslResourceFindings(facts)...)
	findings = append(findings, wslDiskFindings(facts)...)
	findings = append(findings, wslNetworkFindings(facts)...)
	findings = append(findings, wslInteropFindings(facts)...)

	// The list is sorted once, here, so the screen and the tests read the same order: the finding
	// that costs the most comes first, and findings of equal severity keep the order they were built
	// in, which is the order of the sections above.
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].Severity.rank() < findings[j].Severity.rank()
	})
	return findings
}

// wslStorageFindings is the core of the report: where the files the user works on actually live.
func wslStorageFindings(facts wslAdvisorFacts) []wslFinding {
	var findings []wslFinding
	local := facts.LocalProject

	switch {
	case !local.Storage.MountKnown:
		reason := "the mount table could not be read"
		if len(local.Storage.Notes) > 0 {
			reason = local.Storage.Notes[0]
		}
		findings = append(findings, wslFinding{
			ID:       "storage-unknown",
			Severity: wslSeverityNotice,
			Title:    "The storage could not be identified",
			Evidence: "Working directory: " + facts.Cwd + ". " + reason,
			Why: "Until the filesystem is identified, nothing can be said about whether these files " +
				"cross the Windows boundary, and no recommendation would be evidence-based.",
			Action: "Re-run the installer from a directory whose mount is readable, or read " +
				"`/proc/self/mountinfo` yourself.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		})

	case local.Storage.Kind == system.StorageHost:
		findings = append(findings, wslFinding{
			ID:       "storage-cwd-host",
			Severity: wslSeverityCritical,
			Title:    "The working directory is on the Windows host",
			Evidence: wslEvidenceForProject(local),
			Why: "Every metadata operation on these files crosses the boundary between the " +
				"distribution and Windows: Git stats every tracked file on each status and checkout, a " +
				"package manager stats a whole dependency tree, a build walks the source tree, and a " +
				"watcher polls all of it. Each of those is paid per file rather than per project. " +
				"Windows-side antivirus and the host's own filesystem behaviour are added on top of " +
				"the round trip, and this report cannot measure them from here.",
			Benefit: "Moving the project onto the distribution's own filesystem removes that boundary " +
				"from every one of those operations. How much it is worth on this machine is what the " +
				"benchmark below measures, rather than something this report asserts.",
			Action: "Use the migration row below to copy it onto the distribution's own storage " +
				"(nothing is deleted), or move it yourself with the instructions that screen prints.",
			Scope:  wslScopeDistribution,
			Origin: wslOriginMeasured,
		})

	case local.Storage.Kind == system.StorageVirtual:
		findings = append(findings, wslFinding{
			ID:       "storage-cwd-virtual",
			Severity: wslSeverityWarning,
			Title:    "The working directory is not on the distribution's disk",
			Evidence: wslEvidenceForProject(local),
			Why: "A tmpfs is memory: its contents are lost when the instance shuts down, and it " +
				"competes with the memory the rest of the distribution needs. An overlay is a " +
				"container's view of a filesystem, not the distribution's own storage.",
			Benefit: "A project on the distribution's own filesystem survives a restart and stops " +
				"the working tree from being lost with the instance.",
			Action: "Keep working trees on the distribution's own filesystem, and leave temporary " +
				"build output where it is only if it can be regenerated.",
			Scope:  wslScopeDistribution,
			Origin: wslOriginMeasured,
		})

	case local.Storage.Kind == system.StorageDistribution:
		findings = append(findings, wslFinding{
			ID:       "storage-cwd-native",
			Severity: wslSeverityInfo,
			Title:    "The working directory is on the distribution's disk",
			Evidence: wslEvidenceForProject(local),
			Why: "This is the case a WSL setup wants for source trees: the filesystem is a Linux one " +
				"and no operation crosses the Windows boundary.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		})

	default:
		findings = append(findings, wslFinding{
			ID:       "storage-cwd-other",
			Severity: wslSeverityNotice,
			Title:    "The working directory is on other storage",
			Evidence: wslEvidenceForProject(local),
			Why: "The filesystem is neither the distribution's own nor a Windows one, so the " +
				"host-boundary question does not apply here -- and neither does the advice for it.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		})
	}

	if len(local.LinkNotes) > 0 {
		findings = append(findings, wslFinding{
			ID:       "storage-links",
			Severity: wslSeverityWarning,
			Title:    "A link crosses the boundary",
			Evidence: strings.Join(local.LinkNotes, "; "),
			Why: "A link is followed on every access, so the cost of the target is the cost of the " +
				"link: a dependency directory linked onto a Windows drive is paid for by every read " +
				"through it, and an absolute link keeps naming the old location after a move.",
			Benefit: "Replacing the link with a real directory on the distribution's own storage " +
				"removes the crossing. Nothing here changes it for you.",
			Action: "Check each link above and decide per link; the migration preview copies links " +
				"as links and names the ones it cannot keep pointing where they did.",
			Scope:  wslScopeDistribution,
			Origin: wslOriginMeasured,
		})
	}

	// The other projects found beside the current one: named, counted, and with the same finding shape
	// as the current directory, so a user who ran the installer from their home directory still finds
	// the repository that is on the wrong side.
	var hosted []string
	for _, project := range facts.Projects {
		if project.Storage.Kind == system.StorageHost {
			hosted = append(hosted, project.Label)
		}
	}
	if len(hosted) > 0 {
		findings = append(findings, wslFinding{
			ID:       "storage-projects-host",
			Severity: wslSeverityWarning,
			Title:    wslCountLabel(len(hosted), "other project is", "other projects are") + " on the Windows host",
			Evidence: strings.Join(hosted, ", "),
			Why: "The same per-file boundary described above applies to each of these directories when " +
				"they are built, tested or committed.",
			Benefit: "Moving them onto the distribution's own filesystem removes the boundary once, " +
				"for every tool that uses them.",
			Action: "Open the migration row below and choose the project to copy.",
			Scope:  wslScopeDistribution,
			Origin: wslOriginMeasured,
		})
	}

	// A drive list is a fact rather than a finding: it is how the reader checks the classification
	// against the mount table they can read themselves.
	if drives := wslHostMounts(facts.Table); len(drives) > 0 {
		findings = append(findings, wslFinding{
			ID:       "mounts-host",
			Severity: wslSeverityInfo,
			Title:    wslCountLabel(len(drives), "Windows filesystem is", "Windows filesystems are") + " mounted in",
			Evidence: facts.Table.Path + ": " + strings.Join(drives, "; "),
			Why: "These are the mounts whose filesystems belong to the Windows host, whatever their " +
				"mount points are called. They are recognised by filesystem type, never by a path: " +
				"`[automount] root` can put them anywhere.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		})
	}

	return findings
}

// wslMemoryFindings reports memory pressure and swap, from the numbers the instance itself reports.
func wslMemoryFindings(facts wslAdvisorFacts) []wslFinding {
	memory := facts.Memory
	if !memory.Read || !memory.HaveTotal || memory.TotalBytes == 0 {
		return []wslFinding{{
			ID:       "memory-unreadable",
			Severity: wslSeverityNotice,
			Title:    "The instance's memory could not be read",
			Evidence: "Reading " + procMeminfoPath + " did not produce a value. " + memory.Err,
			Why: "Memory advice without the instance's own numbers would be a guess, and this report " +
				"does not guess at the machine it is running on.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		}}
	}

	var findings []wslFinding
	usedShare := 0
	if memory.HaveUsed {
		usedShare = wslPercent(memory.UsedBytes, memory.TotalBytes)
	}
	swapShare := 0
	if memory.HaveSwap && memory.SwapTotalBytes > 0 {
		swapShare = wslPercent(memory.SwapTotalBytes-memory.SwapFreeBytes, memory.SwapTotalBytes)
	}

	switch {
	case !memory.HaveSwap || memory.SwapTotalBytes == 0:
		findings = append(findings, wslFinding{
			ID:       "swap-absent",
			Severity: wslSeverityNotice,
			Title:    "No swap is visible in the distribution",
			Evidence: fmt.Sprintf("MemTotal %s, used %d%%. %s carries no swap total.",
				humanBytes(memory.TotalBytes), usedShare, procMeminfoPath),
			Why: "Without swap, memory pressure inside the limit WSL gives the instance ends in the " +
				"kernel's out-of-memory killer terminating a process rather than in reclaimable " +
				"pressure. Whether that is what you want depends on the work; WSL's own default swap " +
				"is a host-side setting this report does not touch.",
			Benefit: "Leaving swap at WSL's default keeps a build from being killed outright, at the " +
				"cost of placing the swapped pages on the Windows filesystem.",
			Action: "Decide deliberately: a swap setting is written in .wslconfig on the Windows side " +
				"and applies after WSL restarts the instance. The resources utility above edits that " +
				"file with a preview.",
			Scope:  wslScopeHost,
			Origin: wslOriginMeasured,
		})
	case swapShare >= 50:
		findings = append(findings, wslFinding{
			ID:       "swap-heavy",
			Severity: wslSeverityWarning,
			Title:    "Swap is heavily used",
			Evidence: fmt.Sprintf("SwapTotal %s with %s in use (%d%%); MemTotal %s with %d%% in use.",
				humanBytes(memory.SwapTotalBytes),
				humanBytes(memory.SwapTotalBytes-memory.SwapFreeBytes), swapShare,
				humanBytes(memory.TotalBytes), usedShare),
			Why: "Swapped pages live on the Windows filesystem, so a working set that no longer fits " +
				"in the instance's memory pays the host boundary for every page fault -- the same " +
				"boundary the storage findings above are about.",
			Benefit: "Freeing memory inside the instance, or giving the instance more of it, removes " +
				"those page faults from the work rather than making each one faster.",
			Action: "Close what you are not using, or raise the memory limit in .wslconfig on the " +
				"Windows side and let WSL restart the instance.",
			Scope:  wslScopeHost,
			Origin: wslOriginMeasured,
		})
	}

	if usedShare >= 90 && memory.HaveUsed {
		findings = append(findings, wslFinding{
			ID:       "memory-pressure",
			Severity: wslSeverityWarning,
			Title:    "Memory is nearly exhausted",
			Evidence: fmt.Sprintf("MemTotal %s with %d%% in use (MemAvailable or MemFree from %s).",
				humanBytes(memory.TotalBytes), usedShare, procMeminfoPath),
			Why: "The instance's memory is a limit WSL enforces: the Linux side sees the limit, not " +
				"the Windows host's total. What does not fit is swapped or killed, and either one " +
				"slows every process in the distribution.",
			Benefit: "A limit that matches the work removes both the killing and the swapping.",
			Action: "The resources utility above shows what .wslconfig sets today and what this host " +
				"can give; a change there needs WSL to restart the instance.",
			Scope:  wslScopeHost,
			Origin: wslOriginMeasured,
		})
	}

	return findings
}

// wslResourceFindings is the one finding that compares the Windows-side file with what the running
// instance sees, which is how a change that is still waiting for a restart is caught: the file says
// one thing and the instance behaves as it did before.
func wslResourceFindings(facts wslAdvisorFacts) []wslFinding {
	var findings []wslFinding

	if facts.WindowsConfig == nil {
		evidence := facts.WindowsConfigErr
		if evidence == "" {
			evidence = "No .wslconfig was read for this instance, so what it sets is unknown here."
		}
		findings = append(findings, wslFinding{
			ID:       "resources-unreadable",
			Severity: wslSeverityNotice,
			Title:    "The .wslconfig could not be read",
			Evidence: evidence,
			Why: "Without that file this report cannot say whether a configured limit differs from " +
				"what the running instance has, which is the one case where the difference is the " +
				"whole answer: a limit changed in .wslconfig applies only after WSL restarts the " +
				"instance.",
			Action: "Set " + envWSLWindowsHome + " to the Windows user profile, or check the file " +
				"from Windows.",
			Scope:  wslScopeHost,
			Origin: wslOriginMeasured,
		})
		return findings
	}

	config := *facts.WindowsConfig
	var pending []string
	if config.MemoryMB > 0 && facts.Memory.HaveTotal {
		instanceMB := int(facts.Memory.TotalBytes / (1 << 20))
		// The comparison carries a tolerance because the file's unit and the kernel's view are not
		// the same measurement: the point is a limit that was clearly changed and clearly not applied,
		// not the last megabyte.
		if tolerance := wslMemoryToleranceMB(config.MemoryMB); wslAbs(instanceMB-config.MemoryMB) > tolerance {
			pending = append(pending, fmt.Sprintf("memory: the file asks for %d MB, this instance has %d MB",
				config.MemoryMB, instanceMB))
		}
	}
	if config.Processors > 0 && facts.CPUs > 0 && config.Processors != facts.CPUs {
		pending = append(pending, fmt.Sprintf("processors: the file asks for %d, this instance sees %d",
			config.Processors, facts.CPUs))
	}

	if len(pending) == 0 {
		if facts.Memory.HaveTotal {
			findings = append(findings, wslFinding{
				ID:       "resources-applied",
				Severity: wslSeverityInfo,
				Title:    "The resource limits are applied",
				Evidence: fmt.Sprintf("%s: %s. This instance sees %d processors and %s of memory.",
					facts.WindowsConfigPath, wslResourcesSummary(*facts.WindowsConfig), facts.CPUs,
					humanBytes(facts.Memory.TotalBytes)),
				Why: "A .wslconfig change applies when WSL restarts the instance. Here the file and " +
					"the running instance agree, so nothing is waiting.",
				Scope:  wslScopeReadOnly,
				Origin: wslOriginMeasured,
			})
		}
		return findings
	}

	findings = append(findings, wslFinding{
		ID:       "resources-restart-pending",
		Severity: wslSeverityNotice,
		Title:    "A resource limit waits for a restart",
		Evidence: facts.WindowsConfigPath + " — " + strings.Join(pending, "; "),
		Why: "The Windows-side file is read when the instance starts. A difference between the two is " +
			"either a change that has not been applied yet or a limit the running instance predates; " +
			"in both cases the instance keeps behaving as it does now until it is restarted.",
		Benefit: "Restarting once applies the file, so the numbers the rest of this report shows would " +
			"then describe the configuration you actually chose.",
		Action: "From Windows, run `wsl --shutdown` and open the terminal again. This program does not " +
			"restart WSL for you, and cannot from inside the distribution.",
		Scope:  wslScopeRestart,
		Origin: wslOriginMeasured,
	})
	return findings
}

// wslDiskFindings reports the space behind the distribution's own root. The Windows-side disk is not
// visible from here, so its free space is deliberately not guessed at: only what this instance can
// see is measured.
func wslDiskFindings(facts wslAdvisorFacts) []wslFinding {
	disk := facts.RootDisk
	if !disk.Known || disk.TotalBytes == 0 {
		return nil
	}
	share := wslPercent(disk.FreeBytes, disk.TotalBytes)
	if share > 10 && disk.FreeBytes > 1<<30 {
		return nil
	}
	return []wslFinding{{
		ID:       "disk-low",
		Severity: wslSeverityWarning,
		Title:    "The distribution's disk is nearly full",
		Evidence: fmt.Sprintf("%s holds %s free of %s (%d%% free).",
			disk.Path, humanBytes(disk.FreeBytes), humanBytes(disk.TotalBytes), share),
		Why: "A full filesystem inside the distribution stops builds and package installs with write " +
			"errors, and WSL2 keeps its own virtual disk inside a Windows file, so the room the " +
			"distribution has is also Windows' room. Compacting or resizing that virtual disk is a " +
			"Windows-side operation this program does not perform.",
		Benefit: "Freeing space inside the distribution, or removing a copy you no longer need, " +
			"restores the headroom builds need.",
		Action: "Remove what you no longer need inside the distribution. Do not delete the source of " +
			"a project after a migration until you have checked the copy yourself.",
		Scope:  wslScopeDistribution,
		Origin: wslOriginMeasured,
	}}
}

// wslNetworkFindings reports DNS, using the file's own contents and the resolver settings the
// distribution itself carries.
func wslNetworkFindings(facts wslAdvisorFacts) []wslFinding {
	resolv := facts.Resolv
	if !resolv.Read {
		return []wslFinding{{
			ID:       "dns-unreadable",
			Severity: wslSeverityNotice,
			Title:    "The resolver file could not be read",
			Evidence: resolv.Path + ": " + resolv.Err,
			Why: "DNS behaviour is a common cause of a distribution that feels slow on every new " +
				"connection. With the file unreadable, this report cannot say where name resolution " +
				"goes, and it will not infer it.",
			Scope:  wslScopeReadOnly,
			Origin: wslOriginMeasured,
		}}
	}

	if len(resolv.Nameservers) == 0 {
		return []wslFinding{{
			ID:       "dns-no-nameserver",
			Severity: wslSeverityWarning,
			Title:    "No nameserver is configured",
			Evidence: resolv.Path + " lists no `nameserver` line" + wslResolvPathNote(resolv) + ".",
			Why: "With no resolver configured, name lookups fail or fall back to whatever the libc's " +
				"fallback does, which usually shows up as every network command taking a long time " +
				"before it fails.",
			Benefit: "A working resolver removes that wait from the first connection of every tool.",
			Action: "Check " + resolv.Path + " inside the distribution, and the [network] settings in " +
				"/etc/wsl.conf that decide whether WSL writes it. If the file is managed by systemd, " +
				"check that service instead.",
			Scope:  wslScopeDistribution,
			Origin: wslOriginMeasured,
		}}
	}

	// The generator setting comes from /etc/wsl.conf, which is where it lives: the resolver facts
	// deliberately do not carry a copy of it, because two copies of one setting is how a screen comes
	// to state a contradiction.
	generated := resolv.GeneratedHeader
	generatorOn := true
	if value, ok := facts.Conf.GenerateResolvConf.FlagOn(); ok {
		generatorOn = value
	}

	if !generated && !generatorOn {
		return nil
	}

	evidence := resolv.Path + " lists " + strings.Join(resolv.Nameservers, ", ")
	if resolv.LinkTarget != "" {
		evidence += ", reached through " + resolv.LinkTarget
	}
	if generated {
		evidence += "; the file says WSL generated it"
	}

	return []wslFinding{{
		ID:       "dns-through-host",
		Severity: wslSeverityNotice,
		Title:    "DNS goes through the Windows host",
		Evidence: evidence,
		Why: "WSL writes a resolver that points at the host, so every lookup leaves the distribution " +
			"and is answered on the Windows side. That is WSL's documented default and it usually " +
			"works; it is also the reason DNS depends on the host's connection, its VPN and its proxy " +
			"state, and on a machine whose host resolver is slow every new connection waits.",
		Benefit: "A resolver the distribution manages itself makes lookups independent of the host's " +
			"resolver -- and of what a VPN does to it. Whether it is faster here is not something " +
			"this report measured.",
		Action: "The documented route is [network] generateResolvConf = false in /etc/wsl.conf plus a " +
			"resolv.conf the distribution manages (systemd-resolved when systemd is enabled). It needs " +
			"WSL to restart the instance and it is your change to make; this program writes neither " +
			"file.",
		Scope:  wslScopeRestart,
		Origin: wslOriginDocumented,
	}}
}

// wslInteropFindings reports whether Windows executables can run from this shell, which is how the
// clipboard bridge, the Windows Terminal settings read and any host-side command work.
func wslInteropFindings(facts wslAdvisorFacts) []wslFinding {
	var findings []wslFinding

	if value, ok := facts.Conf.InteropEnabled.FlagOn(); ok && !value {
		findings = append(findings, wslFinding{
			ID:       "interop-disabled",
			Severity: wslSeverityNotice,
			Title:    "Interop with Windows is off",
			Evidence: facts.Conf.Describe(facts.Conf.InteropEnabled, "true"),
			Why: "With interop off, Windows programs cannot be started from this shell -- including " +
				"`wsl.exe`, the clipboard bridge this installer ships and any host-side command. That " +
				"is a deliberate setting on many hardened setups, and it also means the Windows-side " +
				"half of this installer's WSL step cannot run from here.",
			Benefit: "Turning it back on restores those tools; leaving it off keeps the isolation it " +
				"was set for.",
			Action: "If you want it back: set [interop] enabled = true in /etc/wsl.conf, then run " +
				"`wsl --shutdown` from Windows and reopen the terminal.",
			Scope:  wslScopeRestart,
			Origin: wslOriginMeasured,
		})
	}

	if !wslInteropRegistered(facts.Runtime) {
		findings = append(findings, wslFinding{
			ID:       "interop-missing",
			Severity: wslSeverityWarning,
			Title:    "No interop registration was found",
			Evidence: wslSignalEvidence(facts.Runtime, "interop-binfmt"),
			Why: "WSL registers the binfmt handler that lets this distribution run Windows " +
				"executables. Without the registration, `wsl.exe`, `explorer.exe` and anything else " +
				"on the Windows side fails with an executable-format error even though the kernel is " +
				"a WSL one. WSL's own documentation covers this interaction, including the case that " +
				"systemd apparently changes when the registration happens.",
			Benefit: "Restoring the registration gives back the host tools, the clipboard bridge and " +
				"the Windows-side half of this installer's WSL step.",
			Action: "Check " + binfmtMiscPathForReport() + " inside the distribution " +
				"(a file named WSLInterop or WSLInterop-late), and the [interop] section of " +
				"/etc/wsl.conf. A restart of the instance is the usual fix after changing it.",
			Scope:  wslScopeRestart,
			Origin: wslOriginMeasured,
		})
	}

	return findings
}

// ============================================================================
// WHERE A COPY COULD LAND
// ============================================================================

// wslNativeParent is a directory a project could be copied into, read once when the report is taken.
// The reading is separate from the path the copy would create because the parent's filesystem and
// room do not depend on which project is being copied.
type wslNativeParent struct {
	// Path is the directory that would hold the copy.
	Path string
	// Label is what a row shows: the path abbreviated, with the room it has.
	Label string
	// Storage is the verdict for the parent, which must be the distribution's own filesystem.
	Storage system.StorageVerdict
	// FreeBytes and TotalBytes are the room behind the parent; SpaceKnown says whether it was read.
	FreeBytes  uint64
	TotalBytes uint64
	SpaceKnown bool
}

// wslNativeDestination is one place a project could be copied to: a native parent joined with the
// project's own name.
type wslNativeDestination struct {
	// Parent is the reading the destination was derived from.
	Parent wslNativeParent
	// Path is the directory the copy would create.
	Path string
	// Label is the row's label, i.e. the parent's own with the project's name.
	Label string
}

// wslNativeParents reads the places a copy could land. Only the distribution's own filesystem is
// offered: suggesting a Windows mount as the destination of a move away from a Windows mount would be
// the exact defect this utility exists to prevent, so the filter is on the filesystem kind and never
// on a path. A parent a copy must not land in -- the source itself or a directory inside it -- is
// filtered where the source is known, in wslNativeDestinations.
func wslNativeParents(in wslAdvisorInput, facts wslAdvisorFacts) []wslNativeParent {
	var paths []string
	if facts.Home != "" {
		paths = append(paths, facts.Home)
		// The conventional project directories, when they are already there: offering to create one
		// would be inventing a layout for this user.
		for _, name := range []string{"projects", "src", "dev", "code", "work"} {
			candidate := filepath.Join(facts.Home, name)
			if in.Exists(candidate) {
				paths = append(paths, candidate)
			}
		}
	}
	// Every other mount whose filesystem is a Linux one, e.g. a disk attached with `wsl --mount`.
	for _, entry := range facts.Table.Entries {
		if !entry.StorageKind().NativeForTooling() {
			continue
		}
		if filepath.Clean(entry.MountPoint) == filepath.Clean(facts.NativeRootPath()) {
			// The root is offered only as the last resort below: joining it with a project's name puts
			// the copy directly under /, which is valid but rarely what someone means.
			continue
		}
		paths = append(paths, entry.MountPoint)
	}
	if len(paths) == 0 {
		if root := facts.NativeRootPath(); root != "" {
			paths = append(paths, root)
		}
	}

	seen := map[string]bool{}
	var parents []wslNativeParent
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "" || path == "." || seen[path] {
			continue
		}
		seen[path] = true
		resolved, err := in.EvalSymlinks(path)
		if err != nil {
			resolved = path
		}
		storage := system.ClassifyStorage(facts.Table, path, resolved)
		if !storage.Kind.NativeForTooling() {
			// A home directory that is itself a Windows mount is a real layout (WSL can be configured
			// that way), and it is exactly the case this utility must not offer as a destination.
			continue
		}
		parent := wslNativeParent{Path: path, Storage: storage, Label: abbreviateHome(path, facts.Home)}
		parent.FreeBytes, parent.TotalBytes, parent.SpaceKnown = in.Statfs(path)
		if parent.SpaceKnown {
			parent.Label += " — " + humanBytes(parent.FreeBytes) + " free"
		} else {
			parent.Label += " — space not read"
		}
		parents = append(parents, parent)
	}
	return parents
}

// wslNativeDestinations derives the copies one source could become: every native parent joined with
// the project's own name, minus the ones a copy must not land in. It is a pure function of the
// reading, so a test can ask where a project on a Windows mount would land without a Windows mount.
func wslNativeDestinations(facts wslAdvisorFacts, sourcePath string) []wslNativeDestination {
	if sourcePath == "" {
		return nil
	}
	source := filepath.Clean(sourcePath)
	base := filepath.Base(source)
	if base == "" || base == "/" || base == "." || base == ".." {
		return nil
	}

	var destinations []wslNativeDestination
	for _, parent := range facts.NativeParents {
		if len(destinations) >= wslDestinationLimit {
			break
		}
		// A copy must not land inside the tree it copies, and must not be the source itself: the
		// first would make the copy recursive, and the second is not a copy at all.
		if parent.Path == source || strings.HasPrefix(parent.Path, source+"/") ||
			strings.HasPrefix(source, parent.Path+"/") {
			continue
		}
		path := filepath.Join(parent.Path, base)
		if path == source {
			continue
		}
		destinations = append(destinations, wslNativeDestination{
			Parent: parent,
			Path:   path,
			Label:  parent.Label + " — copy to " + base,
		})
	}
	return destinations
}

// wslDestinationLimit bounds how many destinations a row list offers. It is a bound on the menu,
// not on the search: every candidate is still classified and the ones left out are counted.
const wslDestinationLimit = 6

// ============================================================================
// HELPERS THE FINDINGS USE
// ============================================================================

// wslEvidenceForProject states what was measured about one project: the path, the mount point, the
// filesystem type, the Git and dependency facts, and every limit of the reading. It is evidence
// rather than a conclusion, which is why it names the mount the path resolved into.
func wslEvidenceForProject(project wslProjectFacts) string {
	// The evidence leads with the real path rather than the abbreviated label a row shows: a reader
	// has to be able to check the claim, and "~/app" is not something they can stat.
	parts := []string{project.Path}
	if project.Storage.MountKnown {
		parts = append(parts, fmt.Sprintf("on %s (%s) mounted at %s",
			project.Storage.Mount.FileSystem, project.Storage.Kind, project.Storage.Mount.MountPoint))
	} else {
		parts = append(parts, "with no mount identified for it")
	}
	if project.Storage.ResolvedPath != project.Path {
		parts = append(parts, "reached through a link from "+project.Path)
	}
	if project.GitRepo {
		parts = append(parts, "a Git repository")
	}
	if len(project.DependencyDirs) > 0 {
		parts = append(parts, "holding "+strings.Join(project.DependencyDirs, ", "))
	}
	if project.Err != "" {
		parts = append(parts, "unreadable: "+project.Err)
	}
	parts = append(parts, project.Storage.Notes...)
	return strings.Join(parts, "; ") + "."
}

// wslHostMounts names the mounts whose filesystem belongs to the Windows host, with their mount
// points, so the classification can be checked against the mount table by hand. The list is capped:
// a host with many drives should not push the rest of the screen out of the frame.
func wslHostMounts(table system.MountTable) []string {
	const limit = 6
	var mounts []string
	for _, entry := range table.Entries {
		if !entry.StorageKind().HostBacked() {
			continue
		}
		// The interop runtime directory is a host mount too, and naming it is what stops a reader from
		// thinking the list is only about drives.
		mounts = append(mounts, entry.MountPoint+" ("+entry.FileSystem+")")
		if len(mounts) == limit {
			mounts = append(mounts, "and more")
			break
		}
	}
	return mounts
}

// wslInteropRegistered reports whether the WSL interop binfmt handler was seen. It reads the signal
// the detection already recorded instead of probing again: one answer, one evidence trail.
func wslInteropRegistered(runtime system.WSLRuntime) bool {
	for _, signal := range runtime.Signals {
		if signal.Name == "interop-binfmt" {
			return signal.Found
		}
	}
	return false
}

// wslSignalEvidence renders the detail the detection recorded for one signal.
func wslSignalEvidence(runtime system.WSLRuntime, name string) string {
	for _, signal := range runtime.Signals {
		if signal.Name == name {
			return signal.Name + ": " + signal.Detail + " was not found"
		}
	}
	return name + ": not looked at"
}

// binfmtMiscPathForReport names the kernel directory the interop registration lives in, for a
// finding's action. The path is the kernel's own fixed location and not a mount a host may move.
func binfmtMiscPathForReport() string { return "/proc/sys/fs/binfmt_misc" }

// wslResolvPathNote adds the symlink note to a resolver sentence when the file is a link, so a
// distribution whose resolver is managed elsewhere is visible in the evidence.
func wslResolvPathNote(resolv wslResolvFacts) string {
	if resolv.LinkTarget == "" {
		return ""
	}
	return " and it is a link to " + resolv.LinkTarget
}

// wslPercent is a share of a whole, clamped so a nonsense pair cannot print 1,200%.
func wslPercent(part, whole uint64) int {
	if whole == 0 {
		return 0
	}
	share := int(part * 100 / whole)
	if share > 100 {
		share = 100
	}
	if share < 0 {
		share = 0
	}
	return share
}

// wslAbs is the magnitude of an integer difference, used by the tolerance comparison.
func wslAbs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// wslMemoryToleranceMB is how far the .wslconfig memory value and the instance's own memory may
// differ before the difference is reported as a pending change. The two are not the same
// measurement -- the file is a request in megabytes, the kernel reports what it was given -- so the
// tolerance keeps the rounding and the unit out of the verdict while a real change stays visible.
func wslMemoryToleranceMB(configuredMB int) int {
	tolerance := configuredMB / 10
	if tolerance < 256 {
		tolerance = 256
	}
	return tolerance
}

// wslResourcesSummary names the values a .wslconfig sets, in the words the resources screen already
// uses for the same numbers, so the two screens describe one file the same way. A file that sets
// none of the three keys says so rather than showing an empty sentence.
func wslResourcesSummary(resources WSLResources) string {
	var parts []string
	if resources.MemoryMB > 0 {
		parts = append(parts, "memory "+wslUnitLabel(resources.MemoryMB, "MB"))
	}
	if resources.Processors > 0 {
		parts = append(parts, "processors "+wslUnitLabel(resources.Processors, ""))
	}
	if resources.SwapMB > 0 {
		parts = append(parts, "swap "+wslUnitLabel(resources.SwapMB, "MB"))
	}
	if len(parts) == 0 {
		return "no memory, processor or swap limit is set"
	}
	return strings.Join(parts, ", ")
}

// wslCountLabel renders a count with its verb, in the singular when there is one, because "1 other
// projects are" is how a report reads when nobody checked.
func wslCountLabel(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// abbreviateHome shortens a path under $HOME to "~", which is what a row has room for.
func abbreviateHome(path, home string) string {
	if home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// wslListHas reports whether a list holds a value, for the small tables above. The name is prefixed
// deliberately: the package already has a containsString for substrings, and two helpers with one
// name is how a call comes to test the wrong thing.
func wslListHas(list []string, want string) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

// mountTable is the reader the input was built with, or the real one when the input was constructed
// without one, so a partially built input still classifies rather than panicking. Every caller in
// this file reads the table through it, so two readings of one machine cannot disagree.
func (in wslAdvisorInput) mountTable() system.MountTable {
	if in.ReadMounts != nil {
		return in.ReadMounts()
	}
	return system.ReadMountTable()
}

// DirEntries reads one directory's entries, through the injected reader when there is one.
func (in wslAdvisorInput) DirEntries(path string) ([]os.DirEntry, error) {
	if in.ReadDir != nil {
		return in.ReadDir(path)
	}
	return os.ReadDir(path)
}
