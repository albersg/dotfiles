package system

import (
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
)

// fixtureProbe builds a WSLProbe from fixtures, which is the only way to test a detection whose
// evidence lives in /proc: no test can create /proc/sys/kernel/osrelease, and a detection that can
// only be exercised on a real WSL host is a detection nobody exercises.
func fixtureProbe(files map[string]string, existing []string, env map[string]string) WSLProbe {
	present := make(map[string]bool, len(existing))
	for _, path := range existing {
		present[path] = true
	}
	return WSLProbe{
		ReadFile: func(path string) ([]byte, error) {
			if contents, ok := files[path]; ok {
				return []byte(contents), nil
			}
			return nil, errors.New("no such file: " + path)
		},
		Exists: func(path string) bool { return present[path] },
		Getenv: func(name string) string { return env[name] },
	}
}

// wsl2Kernel is the release a current WSL2 distribution reports.
const wsl2Kernel = "5.15.153.1-microsoft-standard-WSL2"

// wsl1KernelVersion is the /proc/version line WSL1 exposes.
const wsl1KernelVersion = "Linux version 4.4.0-19041-Microsoft (Microsoft@Microsoft.com) " +
	"(gcc version 9.3.0) #1000-Microsoft"

func TestDetectWSLRuntimeInsideFixtures(t *testing.T) {
	t.Run("WSL2 kernel release is decisive on its own", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(
			map[string]string{procKernelRelease: wsl2Kernel + "\n"}, nil, nil))

		if !got.Inside {
			t.Fatalf("a WSL2 kernel release was not accepted: %s", got.Reason)
		}
		if got.Version != 2 {
			t.Errorf("version = %d, want 2", got.Version)
		}
		if !got.KernelMarked {
			t.Error("KernelMarked = false, want true")
		}
		if got.Reason != "" {
			t.Errorf("Reason = %q, want empty when the answer is yes", got.Reason)
		}
	})

	t.Run("WSL1 kernel version is decisive and named as version 1", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: "4.4.0-19041-Microsoft\n",
			procVersion:       wsl1KernelVersion,
		}, nil, nil))

		if !got.Inside {
			t.Fatalf("a WSL1 kernel was not accepted: %s", got.Reason)
		}
		if got.Version != 1 {
			t.Errorf("version = %d, want 1 for a 4.x Microsoft kernel", got.Version)
		}
	})

	t.Run("an unnumbered marker is read as version 2", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(
			map[string]string{procKernelRelease: "5.10.60.1-microsoft-standard\n"}, nil, nil))

		if !got.Inside || got.Version != 2 {
			t.Errorf("inside = %v version = %d, want true and 2", got.Inside, got.Version)
		}
	})

	t.Run("two instance artefacts are decisive without a kernel marker", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(nil, []string{
			binfmtMiscDir + "/WSLInterop",
			runWSLDir,
		}, nil))

		if !got.Inside {
			t.Fatalf("two instance artefacts were not accepted: %s", got.Reason)
		}
		if got.KernelMarked {
			t.Error("KernelMarked = true without a WSL kernel string")
		}
	})

	t.Run("one instance artefact with an instance variable is decisive", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(nil, []string{wslLibraryDir},
			map[string]string{envWSLDistroName: "Ubuntu-24.04"}))

		if !got.Inside {
			t.Fatalf("an instance artefact plus the distro variable was not accepted: %s", got.Reason)
		}
		if got.Distro != "Ubuntu-24.04" {
			t.Errorf("Distro = %q, want the value of %s", got.Distro, envWSLDistroName)
		}
	})

	t.Run("the late interop registration counts like the first one", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(nil, []string{
			binfmtMiscDir + "/WSLInterop-late",
			runWSLDir,
		}, nil))

		if !got.Inside {
			t.Errorf("WSLInterop-late and /run/WSL were not accepted: %s", got.Reason)
		}
	})
}

func TestDetectWSLRuntimeOutsideFixtures(t *testing.T) {
	t.Run("a conventional Linux host is refused", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: "6.8.0-45-generic\n",
			procVersion:       "Linux version 6.8.0-45-generic (buildd@lcy02) (gcc 13.2.0)\n",
		}, nil, nil))

		if got.Inside {
			t.Fatal("a conventional Linux kernel was accepted as WSL")
		}
		if got.Reason == "" {
			t.Error("a refusal must carry its reason")
		}
	})

	t.Run("a host with no /proc at all is refused rather than guessed", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(nil, nil, nil))

		if got.Inside {
			t.Fatal("no evidence was accepted as WSL")
		}
		if !strings.Contains(got.Reason, "No WSL evidence") {
			t.Errorf("reason = %q, want the no-evidence sentence", got.Reason)
		}
	})

	t.Run("the environment variables alone are never enough", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: "6.8.0-45-generic\n",
		}, nil, map[string]string{
			envWSLDistroName: "Ubuntu-24.04",
			envWSLInterop:    "/run/WSL/8_interop",
		}))

		if got.Inside {
			t.Fatal("inherited variables were accepted as evidence of a WSL instance")
		}
		if !strings.Contains(got.Reason, "inherited by any child process") {
			t.Errorf("reason = %q, want the inherited-variable sentence", got.Reason)
		}
	})

	t.Run("a single instance artefact is not enough", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(nil, []string{runWSLDir}, nil))

		if got.Inside {
			t.Fatal("one directory was accepted as evidence of a WSL instance")
		}
	})

	t.Run("a container on the WSL kernel is refused", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: wsl2Kernel + "\n",
			procCgroup:        "0::/\n",
		}, []string{"/.dockerenv"}, map[string]string{envWSLDistroName: "Ubuntu-24.04"}))

		if got.Inside {
			t.Fatal("a Docker container on the WSL kernel was accepted as a WSL instance")
		}
		if got.Container == "" {
			t.Error("Container is empty, so the refusal cannot name what it refused")
		}
		if !got.KernelMarked {
			t.Error("KernelMarked = false, so the reason cannot say the kernel marker was seen")
		}
		if !strings.Contains(got.Reason, "container") {
			t.Errorf("reason = %q, want the container sentence", got.Reason)
		}
	})

	t.Run("a container marker in the cgroup alone is enough to refuse", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: wsl2Kernel + "\n",
			procCgroup:        "0::/kubepods/burstable/pod1234/abc\n",
		}, []string{binfmtMiscDir + "/WSLInterop", runWSLDir, wslLibraryDir}, nil))

		if got.Inside {
			t.Fatal("a container named only by its cgroup was accepted as a WSL instance")
		}
		if !strings.Contains(got.Container, "kubepods") {
			t.Errorf("Container = %q, want the cgroup runtime name", got.Container)
		}
		if !strings.Contains(got.Reason, "still refused") && !strings.Contains(got.Reason, "even though") {
			t.Errorf("reason = %q, want it to name the misleading kernel marker", got.Reason)
		}
	})

	t.Run("an inventory-named container without a WSL kernel is refused", func(t *testing.T) {
		got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{
			procKernelRelease: "6.8.0-45-generic\n",
			procCgroup:        "1:name=systemd:/docker/9f2c\n",
		}, nil, nil))

		if got.Inside {
			t.Fatal("a container was accepted as a WSL instance")
		}
		if strings.Contains(got.Reason, "even though") {
			t.Errorf("reason = %q, but there was no WSL kernel marker to explain", got.Reason)
		}
	})
}

// TestDetectWSLRuntimeReportsEverySignalLookedAt pins the transparency rule: the answer is only
// checkable if the report says what was read, including the evidence that was not there.
func TestDetectWSLRuntimeReportsEverySignalLookedAt(t *testing.T) {
	got := DetectWSLRuntimeWith(fixtureProbe(map[string]string{procKernelRelease: wsl2Kernel}, nil, nil))

	want := []string{
		"kernel-release", "kernel-version", "interop-binfmt",
		"run-wsl", "wsl-library-dir", "distro-name", "interop-env",
	}
	if len(got.Signals) != len(want) {
		t.Fatalf("%d signals reported, want %d: %+v", len(got.Signals), len(want), got.Signals)
	}
	names := make([]string, 0, len(got.Signals))
	for _, signal := range got.Signals {
		names = append(names, signal.Name)
		if signal.Detail == "" {
			t.Errorf("signal %q has no detail, so the report cannot say what was read", signal.Name)
		}
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("signals = %v, want %v in the order they are read", names, want)
	}

	// The kernel string that was read is in the signal, and the unread ones say so instead of
	// showing a blank row.
	if !strings.Contains(got.Signals[0].Detail, "microsoft") {
		t.Errorf("kernel-release detail = %q, want the string that was read", got.Signals[0].Detail)
	}
	if !strings.Contains(got.Signals[1].Detail, "not readable") {
		t.Errorf("kernel-version detail = %q, want the unreadable reason", got.Signals[1].Detail)
	}
}

// TestDetectWSLRuntimeStrengthLabelsAreStable pins the three names a report shows, because the
// ordering they express -- kernel beats instance beats environment -- is the rule itself.
func TestDetectWSLRuntimeStrengthLabelsAreStable(t *testing.T) {
	cases := map[WSLSignalStrength]string{
		WSLSignalKernel:      "kernel",
		WSLSignalInstance:    "instance",
		WSLSignalEnvironment: "environment",
	}
	for strength, want := range cases {
		if got := strength.String(); got != want {
			t.Errorf("WSLSignalStrength(%d) = %q, want %q", strength, got, want)
		}
	}
	if !(WSLSignalEnvironment < WSLSignalInstance && WSLSignalInstance < WSLSignalKernel) {
		t.Error("the strengths must be ordered weakest to strongest, so a report can sort by them")
	}
}

// TestDetectWSLRuntimeNeverContradictsSystemInfo is the invariant that keeps the two answers from
// drifting apart in the direction that matters: the strict detection may be narrower than
// SystemInfo.IsWSL, never wider. On a host with no /proc this test still proves the negative half.
func TestDetectWSLRuntimeNeverContradictsSystemInfo(t *testing.T) {
	strict := DetectWSLRuntime()
	loose := Detect()

	if strict.Inside && !loose.IsWSL {
		t.Fatalf("the strict detection accepted this host (%s) while SystemInfo.IsWSL is false; "+
			"one of the two is reading evidence the other rejects", strict.Reason)
	}
	if !strict.Inside && strict.Reason == "" {
		t.Error("a refusal with no reason cannot be reported to the user")
	}

	// Every signal this host really produced is reported, so a failing run on a WSL machine can be
	// read from the test log alone.
	for _, signal := range strict.Signals {
		t.Logf("signal %-16s strength=%-11s found=%-5v detail=%s",
			signal.Name, signal.Strength, signal.Found, signal.Detail)
	}

	// The evidence that has to be absent outside WSL is asserted where it can be: on a host whose
	// kernel carries no marker, no instance artefact alone may produce a yes.
	if !strict.Inside && strict.KernelMarked {
		t.Error("KernelMarked is true while the answer is no; the container refusal must say so")
	}
}

// TestDefaultProbeReadsTheRealMachine exercises the default probe's own wiring, so a typo in a path
// constant is caught by a test rather than by a user who sees "no WSL evidence" on their WSL host.
func TestDefaultProbeReadsTheRealMachine(t *testing.T) {
	p := DefaultWSLProbe()
	if p.ReadFile == nil || p.Exists == nil || p.Getenv == nil {
		t.Fatal("DefaultWSLProbe left a reader nil, so a real machine would be probed as empty")
	}

	if _, err := p.ReadFile("/nonexistent/dotfiles-wslruntime-test"); err == nil {
		t.Error("ReadFile reported success for a missing path")
	}
	if p.Exists("/nonexistent/dotfiles-wslruntime-test") {
		t.Error("Exists reported a missing path as present")
	}
	if p.Getenv("DOTFILES_WSL_RUNTIME_DEFINITELY_UNSET") != "" {
		t.Error("Getenv invented a value for an unset variable")
	}

	// The kernel release is the one file every Linux host has. Where it is readable, the signal it
	// feeds has to have been read from it.
	if release, err := os.ReadFile(procKernelRelease); err == nil {
		got := DetectWSLRuntime()
		var detail string
		for _, signal := range got.Signals {
			if signal.Name == "kernel-release" {
				detail = signal.Detail
			}
		}
		if !strings.Contains(detail, strings.TrimSpace(string(release))) {
			t.Errorf("kernel-release detail = %q, want it to contain %q", detail, strings.TrimSpace(string(release)))
		}
	}
}

func TestWSLVersionFromKnownKernels(t *testing.T) {
	cases := []struct {
		name    string
		release string
		version string
		want    int
	}{
		{"WSL2 release", wsl2Kernel, "", 2},
		{"WSL1 release", "4.4.0-19041-Microsoft", wsl1KernelVersion, 1},
		{"unnumbered Microsoft release", "5.10.16.3-microsoft-standard", "", 2},
		{"conventional Linux", "6.8.0-45-generic", "Linux version 6.8.0", 0},
		{"empty", "", "", 0},
		{"a marker in the version line only", "", "Linux version 4.19.128-microsoft-standard", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wslVersionFrom(tc.release, tc.version); got != tc.want {
				t.Errorf("wslVersionFrom(%q, %q) = %d, want %d", tc.release, tc.version, got, tc.want)
			}
		})
	}
}

// TestWSLSignalDetailStaysShort keeps a report row readable: a /proc/version line is a whole build
// record and printing it whole would push the rest of the screen out of the frame.
func TestWSLSignalDetailStaysShort(t *testing.T) {
	long := strings.Repeat("x", 400)
	got := describeKernelString(long)
	if len(got) > 100 {
		t.Errorf("describeKernelString returned %d characters: %q", len(got), got)
	}
	if got := describeKernelString("  \n"); got != "empty" {
		t.Errorf("describeKernelString(blank) = %q, want \"empty\"", got)
	}
}

func TestWSLContainerCgroupNamesAreSortedForStableReports(t *testing.T) {
	// The list is written in prose order and only read by substring, so a duplicate or an empty entry
	// would silently widen or break the refusal. Sorting here also keeps the check meaningful when
	// someone adds a runtime.
	sorted := append([]string(nil), containerCgroupNames...)
	sort.Strings(sorted)
	for i, name := range sorted {
		if name == "" {
			t.Fatal("an empty container name would match every cgroup")
		}
		if i > 0 && sorted[i-1] == name {
			t.Fatalf("the container names contain %q twice", name)
		}
	}
}
