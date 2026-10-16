package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Byte quantities the table is written in, so every host size reads as the
// number the arithmetic in the case comment uses.
const (
	testMiB = 1 << 20
	testGiB = 1 << 30
)

// The host overrides are the deterministic seam for every WSL render in this
// package. They are unexported in package system, so the names are repeated
// here as literals and a change there breaks these tests loudly.
const (
	envWSLHostMemoryMB = "DOTFILES_WSL_HOST_MEMORY_MB"
	envWSLHostCPUs     = "DOTFILES_WSL_HOST_CPUS"
)

// testWSLConfigTemplate is the checkout fixture the WSL tests install. It has
// the same shape as the shipped template -- the three machine-derived keys
// behind template actions, the fixed networking key kept -- but drops the
// shipped prose so the tests can assert exact rendered bytes instead of prose.
const testWSLConfigTemplate = "[wsl2]\n" +
	"{{if .MemoryMB}}memory={{.MemoryMB}}MB\n{{end}}" +
	"{{if .Processors}}processors={{.Processors}}\n{{end}}" +
	"{{if .SwapMB}}swap={{.SwapMB}}MB\n{{end}}" +
	"networkingMode=mirrored\n"

// TestPlanWSLResources pins the WSL resource policy. Every case states its own
// arithmetic, because the numbers are the contract:
//
//	memory = max(min(host - host/3, host - 4 GiB), host/2)
//
// rounded down to 512 MiB and omitted below 1 GiB. The two clamps are a ratio and
// a floor, and the floor is WSL's own proportional default, so the plan never
// hands the VM less than WSL would choose by itself. Processors are every logical
// CPU but one, so Windows keeps one; swap is a quarter of the planned memory,
// rounded down to 512 MiB and omitted when that is zero.
//
// The table deliberately covers all four ways the memory expression can be
// decided: the ratio alone, the Windows reserve alone, the floor alone, and the
// floor winning over the reserve on a host too small to honour both.
func TestPlanWSLResources(t *testing.T) {
	tests := []struct {
		name string
		host system.HostResources
		want WSLResources
	}{
		{
			// host == the 4 GiB reserve is not "over" it, so the reserve branch
			// is skipped and the floor decides: 4096/2 = 2048. Swap = 512.
			name: "4 GiB, 4 CPUs: host is not over the 4 GiB reserve, so the floor decides = 2048 MiB; swap = 512",
			host: system.HostResources{MemoryBytes: 4 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 2048, Processors: 3, SwapMB: 512},
		},
		{
			// The floor wins over the reserve: the reserve leaves 6144-4096 =
			// 2048 MiB, the ratio is 4096 MiB, and the floor is 3072 MiB, so the
			// VM takes 3 GiB and Windows keeps 3 GiB rather than the promised 4.
			name: "6 GiB, 8 CPUs: floor 3072 beats ratio 4096 clamped by the reserve to 2048; swap = 768 -> 512",
			host: system.HostResources{MemoryBytes: 6 * testGiB, LogicalCPUs: 8},
			want: WSLResources{MemoryMB: 3072, Processors: 7, SwapMB: 512},
		},
		{
			// The reserve wins over the ratio: the ratio is 6827 MiB, the
			// reserve leaves 6144 MiB, and the floor (5120) is below both. 6144
			// is already a whole 512 step, so it is the plan. Swap = 1536.
			name: "10 GiB, 8 CPUs: ratio 6827 clamped by the reserve to 6144, above the 5120 floor; swap = 1536",
			host: system.HostResources{MemoryBytes: 10 * testGiB, LogicalCPUs: 8},
			want: WSLResources{MemoryMB: 6144, Processors: 7, SwapMB: 1536},
		},
		{
			// 16777216000 B = 16000 MiB. The ratio is 16000 - 5333 = 10667 MiB,
			// the reserve leaves 11904 MiB and the floor is 8000 MiB, so the
			// ratio decides. 10667 = 20x512 + 427 -> 20x512 = 10240. Swap =
			// 10240/4 = 2560 = exactly 5x512.
			name: "16000 MiB, 12 CPUs: min(10667, 11904) = 10667 above the 8000 floor -> 10240; swap = 2560",
			host: system.HostResources{MemoryBytes: 16777216000, LogicalCPUs: 12},
			want: WSLResources{MemoryMB: 10240, Processors: 11, SwapMB: 2560},
		},
		{
			// This host, to the byte: 16016 MiB of RAM and 12 logical CPUs. It is
			// the machine the 2026-10-16 OOMs ran on, when the policy planned
			// 7680 MB and the VM died twice. The plan that replaces it is the
			// value the user chose by hand: 16016-5338 = 10678 -> 10240.
			name: "this host, 16016 MiB and 12 CPUs: ratio 10678 above the 8008 floor -> 10240; swap = 2560; 11 processors",
			host: system.HostResources{MemoryBytes: 16016 * testMiB, LogicalCPUs: 12},
			want: WSLResources{MemoryMB: 10240, Processors: 11, SwapMB: 2560},
		},
		{
			// 32768 - 10922 = 21846, reserve leaves 28672, floor is 16384, so
			// the ratio decides. 21846 = 42x512 + 342 -> 21504. Swap = 5376 ->
			// 10x512 = 5120.
			name: "32 GiB, 16 CPUs: min(21846, 28672) = 21846 above the 16384 floor -> 21504; swap = 5376 -> 5120",
			host: system.HostResources{MemoryBytes: 32 * testGiB, LogicalCPUs: 16},
			want: WSLResources{MemoryMB: 21504, Processors: 15, SwapMB: 5120},
		},
		{
			// 65536 - 21845 = 43691 -> 85x512 = 43520. Swap = 10880 -> 21x512
			// = 10752.
			name: "64 GiB, 32 CPUs: min(43691, 61440) = 43691 -> 43520; swap = 10880 -> 10752",
			host: system.HostResources{MemoryBytes: 64 * testGiB, LogicalCPUs: 32},
			want: WSLResources{MemoryMB: 43520, Processors: 31, SwapMB: 10752},
		},
		{
			// Below the reserve, so only the floor applies: 3072/2 = 1536 MiB.
			// Swap would be 1536/4 = 384 MiB, which rounds down to nothing.
			// This is the case the old 2 GiB reserve used to starve to 1024 MiB.
			name: "3 GiB, 4 CPUs: below the reserve, floor = 1536 MiB; swap = 384 -> 0 (omitted)",
			host: system.HostResources{MemoryBytes: 3 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 1536, Processors: 3, SwapMB: 0},
		},
		{
			// Exactly the reserve: the branch is `>`, so the floor decides and a
			// 2 GiB host plans the 1 GiB WSL would give itself, rather than the
			// omitted key the old reserve rule wrote.
			name: "2 GiB, 4 CPUs: host == 4 GiB reserve; floor = 1024 MiB is exactly the 1 GiB minimum, so it is written",
			host: system.HostResources{MemoryBytes: 2 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 1024, Processors: 3, SwapMB: 0},
		},
		{
			// 2560/2 = 1280 MiB, which rounds down to 2x512 = 1024. The floor
			// must be rounded too, or a plan would promise more than the step can
			// hold.
			name: "2.5 GiB, 4 CPUs: floor = 1280 MiB -> 1024 after the 512 step; swap = 256 -> 0",
			host: system.HostResources{MemoryBytes: 2560 * testMiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 1024, Processors: 3, SwapMB: 0},
		},
		{
			// 1024/2 = 512 MiB, below the 1 GiB minimum, so the key is omitted
			// rather than written as a limit that would leave the distribution
			// unable to start. One logical CPU still leaves one for Windows.
			name: "1 GiB, 2 CPUs: floor = 512 MiB is below the 1 GiB minimum, so memory and swap are omitted; 1 processor",
			host: system.HostResources{MemoryBytes: 1 * testGiB, LogicalCPUs: 2},
			want: WSLResources{MemoryMB: 0, Processors: 1, SwapMB: 0},
		},
		{
			// 5632 - 1877 = 3755, reserve leaves 1536, floor is 2816, so the
			// floor decides: 2816 -> 5x512 = 2560. Swap = 640 -> 512.
			name: "5.5 GiB, 8 CPUs: floor 2816 beats ratio 3755 clamped to 1536 -> 2560; swap = 640 -> 512",
			host: system.HostResources{MemoryBytes: 5632 * testMiB, LogicalCPUs: 8},
			want: WSLResources{MemoryMB: 2560, Processors: 7, SwapMB: 512},
		},
		{
			// The reserve and the floor agree here: 8192 - 4096 = 4096 either way.
			name: "8 GiB, unknown CPUs: reserve 4096 == floor 4096, so both give 4096 MiB; swap = 1024",
			host: system.HostResources{MemoryBytes: 8 * testGiB, LogicalCPUs: 0},
			want: WSLResources{MemoryMB: 4096, Processors: 0, SwapMB: 1024},
		},
		{
			// The processor floor: "all but one" cannot become zero. A one-CPU
			// host must still say processors=1, because omitting the key would
			// hand WSL every CPU it can find.
			name: "16 GiB, 1 CPU: the all-but-one rule floors at 1 instead of omitting; memory = 10752",
			host: system.HostResources{MemoryBytes: 16 * testGiB, LogicalCPUs: 1},
			want: WSLResources{MemoryMB: 10752, Processors: 1, SwapMB: 2560},
		},
		{
			name: "unknown host: all three keys omitted, so WSL applies its own defaults",
			host: system.HostResources{},
			want: WSLResources{MemoryMB: 0, Processors: 0, SwapMB: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanWSLResources(tt.host)

			if got != tt.want {
				t.Errorf("PlanWSLResources(%+v) = %+v, want %+v", tt.host, got, tt.want)
			}
		})
	}
}

// TestRenderWSLConfigForAKnownHost pins the exact rendered bytes for a host
// supplied through the override seam, so the expectation cannot depend on the
// machine running the test. 16 GiB and 8 CPUs plan to 10752 MB of memory
// (16384-5461 = 10923 MiB, above the 12288 the reserve allows and well above the
// 8192 floor, rounded down to 21 steps of 512), 7 processors (all but one) and
// 2560 MB of swap (a quarter of the memory, rounded down).
func TestRenderWSLConfigForAKnownHost(t *testing.T) {
	t.Setenv(envWSLHostMemoryMB, "16384")
	t.Setenv(envWSLHostCPUs, "8")

	rendered, err := RenderWSLConfig(testWSLConfigTemplate, PlanWSLResources(system.DetectHostResources()))
	if err != nil {
		t.Fatalf("RenderWSLConfig failed: %v", err)
	}

	want := "[wsl2]\nmemory=10752MB\nprocessors=7\nswap=2560MB\nnetworkingMode=mirrored\n"
	if string(rendered) != want {
		t.Errorf("rendered .wslconfig =\n%q\nwant\n%q", rendered, want)
	}
}

// TestRenderWSLConfigOmitsUnavailableKeys is the fallback contract: when the
// host cannot be read, the three derived keys disappear entirely and WSL applies
// its own proportional defaults. The overrides are cleared and PATH is emptied,
// so the detector can neither read a capacity from the environment nor resolve
// a Windows binary.
func TestRenderWSLConfigOmitsUnavailableKeys(t *testing.T) {
	t.Setenv(envWSLHostMemoryMB, "")
	t.Setenv(envWSLHostCPUs, "")
	t.Setenv("PATH", "")

	rendered, err := RenderWSLConfig(testWSLConfigTemplate, PlanWSLResources(system.DetectHostResources()))
	if err != nil {
		t.Fatalf("RenderWSLConfig failed: %v", err)
	}

	want := "[wsl2]\nnetworkingMode=mirrored\n"
	if string(rendered) != want {
		t.Errorf("rendered .wslconfig =\n%q\nwant\n%q", rendered, want)
	}
	if strings.Contains(string(rendered), "{{") {
		t.Errorf("a template delimiter leaked into the fallback render: %q", rendered)
	}
}

// TestRenderWSLConfigRejectsABadTemplate keeps the failure direction safe: a
// template the parser or the executor rejects is an error, never a half-written
// .wslconfig that WSL would read as configuration.
func TestRenderWSLConfigRejectsABadTemplate(t *testing.T) {
	t.Run("parse error", func(t *testing.T) {
		if _, err := RenderWSLConfig("memory={{if .MemoryMB}}", WSLResources{MemoryMB: 2048}); err == nil {
			t.Fatal("a template the parser rejects must be reported as an error")
		}
	})

	t.Run("execution error", func(t *testing.T) {
		if _, err := RenderWSLConfig("processors={{.MissingField}}", WSLResources{}); err == nil {
			t.Fatal("a template that references an unknown field must be reported as an error")
		}
	})
}

// TestShippedWSLConfigTemplatePreservesFixedSettings renders the real checkout
// template for a known host and for the omit-everything fallback. Every fixed
// key and section must survive both renders, the three derived keys must follow
// the host, and no template delimiter may leak into the file that reaches the
// Windows profile.
func TestShippedWSLConfigTemplatePreservesFixedSettings(t *testing.T) {
	templateText, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetWSLConfig))
	if err != nil {
		t.Fatalf("reading the shipped WSL template: %v", err)
	}

	fixed := []string{
		"[wsl2]",
		"localhostForwarding=true",
		"networkingMode=mirrored",
		"dnsTunneling=true",
		"[experimental]",
		"autoMemoryReclaim=gradual",
		"sparseVhd=true",
	}

	for _, tc := range []struct {
		name     string
		memoryMB string
		cpus     string
		want     []string
		omit     []string
	}{
		{
			name:     "known host",
			memoryMB: "16384",
			cpus:     "8",
			want:     []string{"memory=10752MB", "processors=7", "swap=2560MB"},
		},
		{
			name:     "unknown host",
			memoryMB: "",
			cpus:     "",
			omit:     []string{"memory=", "processors=", "swap="},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envWSLHostMemoryMB, tc.memoryMB)
			t.Setenv(envWSLHostCPUs, tc.cpus)
			if tc.memoryMB == "" {
				t.Setenv("PATH", "")
			}

			rendered, err := RenderWSLConfig(string(templateText), PlanWSLResources(system.DetectHostResources()))
			if err != nil {
				t.Fatalf("rendering the shipped template: %v", err)
			}
			got := string(rendered)

			for _, setting := range fixed {
				if !strings.Contains(got, setting) {
					t.Errorf("rendered .wslconfig lost the fixed setting %q:\n%s", setting, got)
				}
			}
			for _, key := range tc.want {
				if !strings.Contains(got, key) {
					t.Errorf("rendered .wslconfig is missing the derived key %q:\n%s", key, got)
				}
			}
			for _, key := range tc.omit {
				if strings.Contains(got, key) {
					t.Errorf("rendered .wslconfig should omit %q when the host is unknown:\n%s", key, got)
				}
			}
			if strings.Contains(got, "{{") {
				t.Errorf("a template delimiter leaked into the rendered file:\n%s", got)
			}
			if strings.Contains(got, "\r") || !strings.HasSuffix(got, "\n") {
				t.Errorf("rendered .wslconfig must keep LF line endings, got %q", got)
			}
		})
	}
}

// renderedFixture is the shipped template rendered for one plan, which is the
// content a fresh install writes and the content the utilities screen starts
// from. Round-tripping it through the merge must be the identity: a file this
// repository wrote is never reformatted behind the user's back.
func renderedFixture(t *testing.T, plan WSLResources) []byte {
	t.Helper()

	shipped, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetWSLConfig))
	if err != nil {
		t.Fatalf("reading the shipped WSL template: %v", err)
	}
	rendered, err := RenderWSLConfig(string(shipped), plan)
	if err != nil {
		t.Fatalf("rendering the shipped template: %v", err)
	}
	return rendered
}

// TestMergeWSLConfigKeepsEverythingItDoesNotManage is the preservation guard: the
// user's .wslconfig can hold keys, comments and sections this installer knows
// nothing about, and a write may only touch the keys the template manages. It
// also pins the fresh-install case, where an empty destination receives the
// render byte for byte.
func TestMergeWSLConfigKeepsEverythingItDoesNotManage(t *testing.T) {
	plan := WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048}
	rendered := renderedFixture(t, plan)

	tests := []struct {
		name     string
		existing string
		kept     []string
		applied  []string
	}{
		{
			name:     "a file the repository itself wrote is not reformatted",
			existing: string(rendered),
			kept:     []string{"# Settings apply across all Linux distros", "[wsl2]", "[experimental]"},
			applied:  []string{"memory=8192MB", "processors=8", "swap=2048MB", "sparseVhd=true"},
		},
		{
			name: "the user's own keys, comments and sections survive",
			existing: "[wsl2]\n" +
				"# my own kernel\n" +
				"kernel=C:\\\\kernel\n" +
				"memory=32GB\n" +
				"[experimental]\n" +
				"useWindowsDnsCache=true\n" +
				"[interop]\n" +
				"enabled=false\n",
			kept:    []string{"# my own kernel", "kernel=C:\\\\kernel", "useWindowsDnsCache=true", "[interop]", "enabled=false"},
			applied: []string{"memory=8192MB", "processors=8", "swap=2048MB", "networkingMode=mirrored"},
		},
		{
			// A key outside a section is not the setting WSL reads, so it is not the
			// managed one either: the merge leaves it where it is and writes the
			// managed keys under their own section instead of guessing.
			name:     "a file with no section at all still receives the managed ones",
			existing: "memory=4GB\n",
			kept:     []string{"memory=4GB"},
			applied:  []string{"[wsl2]", "memory=8192MB", "processors=8", "swap=2048MB", "[experimental]"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := string(MergeWSLConfig([]byte(tt.existing), rendered))

			for _, want := range append(append([]string{}, tt.kept...), tt.applied...) {
				if !strings.Contains(got, want) {
					t.Errorf("the merge dropped or failed to apply %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, "memory=32GB") {
				t.Errorf("the superseded memory key is still there:\n%s", got)
			}
			if tt.name == "a file the repository itself wrote is not reformatted" && got != tt.existing {
				t.Errorf("the merge reformatted a file the repository wrote:\n--- want ---\n%s\n--- got ---\n%s", tt.existing, got)
			}
		})
	}

	// A fresh install: nothing there yet, so the render is the whole answer.
	if got := string(MergeWSLConfig(nil, rendered)); got != string(rendered) {
		t.Errorf("an empty destination was not given the render unchanged:\n%s", got)
	}
}

// TestMergeWSLConfigLeavesWhatTheTemplateOmits pins the safety half of the merge:
// an unknown host renders no memory, processors or swap, and a key the template
// omits is left exactly as the file has it rather than deleted. A limit this
// installer cannot compute must never be thrown away.
func TestMergeWSLConfigLeavesWhatTheTemplateOmits(t *testing.T) {
	omitted := renderedFixture(t, WSLResources{})
	existing := "[wsl2]\nmemory=32GB\nprocessors=16\n"

	got := string(MergeWSLConfig([]byte(existing), omitted))
	for _, want := range []string{"memory=32GB", "processors=16"} {
		if !strings.Contains(got, want) {
			t.Errorf("an omitted key was deleted from the user's file: %q is gone:\n%s", want, got)
		}
	}
}

// TestParseWSLConfigValues pins the read half the screen shows: the three keys
// under [wsl2], in the units WSL accepts, and 0 -- "not set" -- for anything the
// parser cannot read rather than a limit the file does not impose.
func TestParseWSLConfigValues(t *testing.T) {
	tests := []struct {
		name  string
		lines string
		want  WSLResources
	}{
		{"the template's own form", "[wsl2]\nmemory=8192MB\nprocessors=8\nswap=2048MB\n", WSLResources{8192, 8, 2048}},
		{"gigabytes are read as such", "[wsl2]\nmemory=32GB\nswap=1GB\n", WSLResources{MemoryMB: 32768, SwapMB: 1024}},
		{"an absent key is not set", "[wsl2]\nnetworkingMode=mirrored\n", WSLResources{}},
		{"a value under another section is not the WSL limit", "[experimental]\nmemory=64GB\n", WSLResources{}},
		{"a comment is not a key", "[wsl2]\n# memory=64GB\nmemory=4096MB\n", WSLResources{MemoryMB: 4096}},
		{"an inline comment is not part of the value", "[wsl2]\nmemory=4096MB # half\n", WSLResources{MemoryMB: 4096}},
		{"an unreadable value is not set", "[wsl2]\nmemory=plenty\nprocessors=many\n", WSLResources{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseWSLConfigValues([]byte(tt.lines)); got != tt.want {
				t.Errorf("ParseWSLConfigValues = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestWSLAdjustValue pins the editing arithmetic, including the two edges the
// screen promises: the first step up from "not set" lands on the step itself, and
// nothing can reach a negative value.
func TestWSLAdjustValue(t *testing.T) {
	tests := []struct {
		name        string
		value, step int
		delta       int
		want        int
	}{
		{"a step up", 8192, wslMemoryStepMB, 1, 8704},
		{"a step down", 8192, wslMemoryStepMB, -1, 7680},
		{"the first step from not set", 0, wslMemoryStepMB, 1, wslMemoryStepMB},
		{"down from not set stays not set", 0, wslMemoryStepMB, -1, 0},
		{"down past the step is not set again", 512, wslMemoryStepMB, -1, 0},
		{"one processor at a time", 8, 1, -1, 7},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := wslAdjustValue(tt.value, tt.step, tt.delta); got != tt.want {
				t.Errorf("wslAdjustValue(%d, %d, %d) = %d, want %d", tt.value, tt.step, tt.delta, got, tt.want)
			}
		})
	}
}
