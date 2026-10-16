package system

import (
	"os"
	"strconv"
	"strings"
)

// ============================================================================
// RUNNING INSIDE A WSL DISTRIBUTION, AS OPPOSED TO HAVING WSL SOMEWHERE NEARBY
// ============================================================================
//
// Two different questions are easy to confuse and expensive to confuse here:
//
//	"Is this process inside a WSL instance?"      -- what a WSL-only utility needs
//	"Is this machine a Windows host with WSL?"    -- what nothing on Linux can answer
//
// A program running inside a distribution cannot see whether WSL is installed on the Windows side,
// and it does not need to: everything it observes -- the kernel, the mounts, the interop bridge --
// belongs to the instance it is running in. What it must get right is the other direction, and the
// installer already had the short version of the answer: `classifyLinux` sets SystemInfo.IsWSL from
// the kernel string *or* from WSL_DISTRO_NAME. For the installer's own generic behaviour that is
// fine. For a utility that claims to be WSL-specific it is not, because an environment variable is
// inherited by every child process: a container started from a WSL shell, a `sudo -E`, or a copied
// environment carries WSL_DISTRO_NAME to a process that is not in a WSL instance at all.
//
// So this file answers the question with named evidence and a stated rule, and it is deliberately
// separate from SystemInfo.IsWSL: the install step and the WSL resources utility keep reading the
// field they already read, and nothing here widens or narrows their behaviour.
//
// The rule:
//
//  1. A container is refused outright, with its own reason. The markers (/.dockerenv,
//     /run/.containerenv, a PID 1 cgroup naming a runtime) mean the process sees a container's
//     filesystem, not the distribution's: the mounts, /etc/wsl.conf and the native storage the
//     advisor would report on are all out of view, while the WSL kernel marker stays visible through
//     the shared kernel. Docker Desktop's WSL2 backend is exactly this case.
//  2. Otherwise, kernel evidence is decisive on its own. The kernel release or version string
//     carrying the Microsoft/WSL marker is a property of the running kernel, not of the process, and
//     it is what WSL1 and WSL2 both report.
//  3. Two or more instance artefacts are decisive: /proc/sys/fs/binfmt_misc/WSLInterop (registered by
//     WSL's own init), /run/WSL and /usr/lib/wsl (created by the instance). One alone is not: a
//     single directory could exist on a distribution that merely has the WSL packages installed.
//  4. One instance artefact plus one environment variable is decisive, which keeps a distribution
//     with a restricted /proc -- or a kernel string this rule does not know yet -- detectable.
//  5. Environment variables alone are never decisive. This is the exact case the request names.
//
// Nothing here guesses at the host. When the answer is "no", the reason says which evidence was
// looked for and what was found, and the utility that gates on it stays invisible.

// WSLSignalStrength says how much one piece of evidence is worth. The names are what a screen shows,
// so they read as the three kinds of fact and not as a score.
type WSLSignalStrength int

const (
	// WSLSignalEnvironment is an inherited variable. It is corroboration and never a verdict: every
	// child process of a WSL shell inherits it, including one running somewhere else entirely.
	WSLSignalEnvironment WSLSignalStrength = iota
	// WSLSignalInstance is an artefact WSL itself created: the interop registration, the instance's
	// own run directory, the library directory the instance mounts.
	WSLSignalInstance
	// WSLSignalKernel is a property of the running kernel, visible from inside and inherited by
	// nothing.
	WSLSignalKernel
)

// String names the strength the way a screen says it.
func (s WSLSignalStrength) String() string {
	switch s {
	case WSLSignalKernel:
		return "kernel"
	case WSLSignalInstance:
		return "instance"
	default:
		return "environment"
	}
}

// WSLSignal is one thing the detection looked at. Found is recorded for every signal, not only the
// ones that matched, because a report that says "not in WSL" has to be able to say what it looked
// for -- an answer nobody can check is not evidence.
type WSLSignal struct {
	// Name is a stable short name for the evidence, e.g. "kernel-release".
	Name string
	// Detail is what was read or looked for: a kernel string, a path, or the variable that was
	// empty. It is shown to the user, so it must not carry anything private.
	Detail string
	// Strength is what this kind of evidence is worth.
	Strength WSLSignalStrength
	// Found reports whether the evidence was there.
	Found bool
}

// WSLRuntime is the answer: whether this process runs inside a WSL instance, and on what evidence.
type WSLRuntime struct {
	// Inside is the verdict the WSL-only utility gates on.
	Inside bool
	// Version is 1 or 2 when the evidence names it and 0 when it does not. WSL1 reports a 4.x kernel
	// and WSL2 reports 5.x or later, so an unnumbered Microsoft marker is read as the modern default
	// only when the release does not contradict it.
	Version int
	// Distro is WSL_DISTRO_NAME when the environment carries it. It is a label for the report, never
	// part of the verdict.
	Distro string
	// KernelMarked reports that the kernel itself carries the WSL marker, including the container
	// case where the kernel belongs to the instance the container runs on.
	KernelMarked bool
	// Container names the container marker that was found, empty when none was. It is the reason a
	// container on the WSL kernel is still refused.
	Container string
	// Signals is every piece of evidence looked at, in the order it was read.
	Signals []WSLSignal
	// Reason says why Inside is false, in the words the report shows. It is empty when Inside.
	Reason string
}

// The paths and variable names the detection reads. They are constants rather than configuration:
// each one is a fixed artefact of WSL or of the Linux container runtimes, and none of them is a mount
// point a host may relocate (which is why there is no /mnt/c anywhere in this file).
const (
	// procKernelRelease is the running kernel's release string, e.g.
	// "5.15.153.1-microsoft-standard-WSL2" on WSL2 and "4.4.0-19041-Microsoft" on WSL1.
	procKernelRelease = "/proc/sys/kernel/osrelease"
	// procVersion is the kernel's own version line, which carries the build host on WSL1 and WSL2.
	procVersion = "/proc/version"
	// procCgroup is PID 1's cgroup, the one container marker that needs no file in the container's
	// own root and can therefore not be explained away by a base image.
	procCgroup = "/proc/1/cgroup"
	// binfmtMiscDir is where the kernel exposes registered binary formats. WSL's init registers
	// WSLInterop -- and, on recent versions, WSLInterop-late -- to make Windows executables runnable.
	binfmtMiscDir = "/proc/sys/fs/binfmt_misc"
	// runWSLDir is the instance's own run directory, created by WSL on the Linux side.
	runWSLDir = "/run/WSL"
	// wslLibraryDir is where the instance exposes the Windows driver and 9p libraries it mounts.
	wslLibraryDir = "/usr/lib/wsl"
	// envWSLDistroName and envWSLInterop are set by WSL's init for a shell inside a distribution.
	// They are read as corroboration only.
	envWSLDistroName = "WSL_DISTRO_NAME"
	envWSLInterop    = "WSL_INTEROP"
)

// wslContainerMarkers are the files a container runtime leaves in the container's own root. A file is
// only as good as the image that ships it, so the cgroup is read as well; either is enough to refuse.
var wslContainerMarkers = []struct{ Path, Name string }{
	{"/.dockerenv", "Docker (/.dockerenv)"},
	{"/run/.containerenv", "Podman (/run/.containerenv)"},
}

// containerCgroupNames are the runtime names a PID 1 cgroup path carries. They are matched against
// the whole file rather than a parsed path because the formats differ between cgroup v1 and v2 and
// the substring is the only part that means the same thing in both.
var containerCgroupNames = []string{"docker", "podman", "libpod", "containerd", "kubepods", "lxc", "garden"}

// WSLProbe is where the detection reads from. It is a value rather than a set of direct calls so the
// decision can be exercised against fixtures: a test cannot create /proc/sys/kernel/osrelease, and a
// detection that can only be tested on a real WSL host is a detection nobody tests.
type WSLProbe struct {
	// ReadFile returns a file's contents. A missing file is reported as an error, which is the same
	// thing the detection does with a restricted /proc.
	ReadFile func(string) ([]byte, error)
	// Exists reports whether a path is there, file or directory. Both kinds are used for
	// kernel-exposed files and for directories, so the distinction does not matter to the caller.
	Exists func(string) bool
	// Getenv returns an environment variable, empty when it is not set.
	Getenv func(string) string
}

// DefaultWSLProbe reads the real machine.
func DefaultWSLProbe() WSLProbe {
	return WSLProbe{
		ReadFile: os.ReadFile,
		Exists: func(path string) bool {
			_, err := os.Lstat(path)
			return err == nil
		},
		Getenv: os.Getenv,
	}
}

// DetectWSLRuntime answers whether this process runs inside a WSL instance, on the real machine.
func DetectWSLRuntime() WSLRuntime {
	return DetectWSLRuntimeWith(DefaultWSLProbe())
}

// DetectWSLRuntimeWith is the decision itself. It reads every signal, records what it found, and
// applies the rule documented at the top of this file in the order that rule states: a container is
// refused first, then the kernel, then the instances artefacts, and the environment only ever
// corroborates.
func DetectWSLRuntimeWith(p WSLProbe) WSLRuntime {
	result := WSLRuntime{Distro: p.Getenv(envWSLDistroName)}

	release, releaseErr := p.ReadFile(procKernelRelease)
	version, versionErr := p.ReadFile(procVersion)
	cgroup, _ := p.ReadFile(procCgroup)

	kernelText := string(release) + " " + string(version)
	result.Signals = []WSLSignal{
		kernelSignal("kernel-release", string(release), string(release), releaseErr),
		kernelSignal("kernel-version", string(version), kernelText, versionErr),
		{
			Name:     "interop-binfmt",
			Detail:   binfmtMiscDir + "/WSLInterop",
			Strength: WSLSignalInstance,
			Found: p.Exists(binfmtMiscDir+"/WSLInterop") ||
				p.Exists(binfmtMiscDir+"/WSLInterop-late"),
		},
		{
			Name:     "run-wsl",
			Detail:   runWSLDir,
			Strength: WSLSignalInstance,
			Found:    p.Exists(runWSLDir),
		},
		{
			Name:     "wsl-library-dir",
			Detail:   wslLibraryDir,
			Strength: WSLSignalInstance,
			Found:    p.Exists(wslLibraryDir),
		},
		{
			Name:     "distro-name",
			Detail:   envWSLDistroName,
			Strength: WSLSignalEnvironment,
			Found:    p.Getenv(envWSLDistroName) != "",
		},
		{
			Name:     "interop-env",
			Detail:   envWSLInterop,
			Strength: WSLSignalEnvironment,
			Found:    p.Getenv(envWSLInterop) != "",
		},
	}

	result.KernelMarked = kernelMatched(string(release), string(version))
	result.Version = wslVersionFrom(string(release), string(version))

	// The container is decided before anything else, so its reason can say that the kernel marker was
	// seen and still refused. A container on the WSL kernel is the one case where the strongest
	// evidence is misleading, and it deserves its own sentence rather than the generic answer.
	if marker := containerMarker(p, string(cgroup)); marker != "" {
		result.Container = marker
		result.Reason = "This process runs inside a container (" + marker + "), so what it sees is the " +
			"container's filesystem, not the WSL distribution's own mounts and configuration"
		if result.KernelMarked {
			result.Reason += ", even though the kernel it shares carries the WSL marker"
		}
		result.Reason += "."
		return result
	}

	var kernel, instance, environment int
	for i := range result.Signals {
		if !result.Signals[i].Found {
			continue
		}
		switch result.Signals[i].Strength {
		case WSLSignalKernel:
			kernel++
		case WSLSignalInstance:
			instance++
		default:
			environment++
		}
	}

	switch {
	case kernel > 0, instance >= 2, instance >= 1 && environment > 0:
		result.Inside = true
		return result
	}

	result.Reason = wslRefusalReason(result.Signals)
	return result
}

// kernelSignal builds the signal for one kernel string. A string that could not be read is recorded
// with the reason rather than as an empty detail, so a report can tell "no marker" from "not read".
func kernelSignal(name, detail, haystack string, err error) WSLSignal {
	signal := WSLSignal{
		Name:     name,
		Detail:   describeKernelString(detail),
		Strength: WSLSignalKernel,
		Found:    hasWSLKernelMarker(haystack),
	}
	if err != nil {
		signal.Detail = "not readable: " + err.Error()
	}
	return signal
}

// describeKernelString shortens a kernel string for a row and marks an empty one, so a report never
// shows a blank where evidence belongs.
func describeKernelString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "empty"
	}
	// /proc/version is a whole line with the compiler and the build host in it. The first three
	// fields carry the version, so the rest is dropped rather than shown as noise.
	if len(trimmed) > 96 {
		trimmed = trimmed[:93] + "..."
	}
	return trimmed
}

// hasWSLKernelMarker reports whether a kernel string identifies a WSL kernel.
//
// "microsoft" is what both versions report: WSL1 builds a 4.4 kernel named "-Microsoft" and WSL2 the
// "-microsoft-standard-WSL2" one. "wsl" catches a custom kernel that spells it that way. This is the
// same test `isWSLKernel` applies to classifyLinux, kept here as well because this file must be able
// to explain its own answer and because a fixture kernel string is not always /proc/version.
func hasWSLKernelMarker(kernelString string) bool {
	content := strings.ToLower(kernelString)
	return strings.Contains(content, "microsoft") || strings.Contains(content, "wsl")
}

// kernelMatched reports whether either kernel string carries the marker.
func kernelMatched(release, version string) bool {
	return hasWSLKernelMarker(release) || hasWSLKernelMarker(version)
}

// wslVersionFrom reads the WSL generation out of the kernel strings. It returns 0 when nothing in
// them names a version rather than guessing, which is what the report shows as "version unknown".
func wslVersionFrom(release, version string) int {
	content := strings.ToLower(release + " " + version)
	if strings.Contains(content, "wsl2") {
		return 2
	}
	if !strings.Contains(content, "microsoft") && !strings.Contains(content, "wsl") {
		return 0
	}
	// WSL1 is the only version whose kernel is a 4.x series, and WSL2 the only one with 5.x or later.
	if major, ok := kernelMajor(release); ok && major < 5 {
		return 1
	}
	// A marker without a usable release is read as WSL2: it is the current default, and calling it
	// WSL1 would send the reader to migration advice for the wrong version.
	return 2
}

// kernelMajor reads the major version out of a kernel release like "5.15.153.1-microsoft-standard".
func kernelMajor(release string) (int, bool) {
	head, _, _ := strings.Cut(strings.TrimSpace(release), ".")
	if head == "" {
		return 0, false
	}
	major, err := strconv.Atoi(head)
	if err != nil || major < 0 {
		return 0, false
	}
	return major, true
}

// containerMarker names the container runtime this process runs under, empty when there is none. The
// files are checked first because they carry the friendlier name; the cgroup catches the images that
// ship none of them.
func containerMarker(p WSLProbe, cgroup string) string {
	for _, marker := range wslContainerMarkers {
		if p.Exists(marker.Path) {
			return marker.Name
		}
	}
	lowered := strings.ToLower(cgroup)
	for _, name := range containerCgroupNames {
		if strings.Contains(lowered, name) {
			return name + " (" + procCgroup + ")"
		}
	}
	return ""
}

// wslRefusalReason explains a "no" in one sentence, naming what was looked for and what was there.
// The environment is called out by name when it was the only thing found, because that is the answer
// the request asks the utility to refuse: an inherited variable is not an instance.
func wslRefusalReason(signals []WSLSignal) string {
	var found []string
	environmentOnly := true
	for i := range signals {
		if !signals[i].Found {
			continue
		}
		found = append(found, signals[i].Name)
		if signals[i].Strength != WSLSignalEnvironment {
			environmentOnly = false
		}
	}

	switch {
	case environmentOnly && len(found) > 0:
		return "Only environment variables (" + strings.Join(found, ", ") + ") suggest WSL. Those are " +
			"inherited by any child process, so they are not evidence that this process runs inside a " +
			"WSL distribution, and the kernel and the instance's own artefacts do not carry the marker."
	case len(found) == 0:
		return "No WSL evidence was found: the kernel carries no WSL marker and none of the instance's " +
			"artefacts (" + binfmtMiscDir + "/WSLInterop, " + runWSLDir + ", " + wslLibraryDir + ") is here."
	default:
		return "The evidence found (" + strings.Join(found, ", ") + ") is not enough for a WSL " +
			"instance: a WSL kernel or at least two of the instance's own artefacts are needed, and the " +
			"environment variables only corroborate."
	}
}
