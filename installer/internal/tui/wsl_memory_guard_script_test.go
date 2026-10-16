package tui

// The guard payload is the only artifact in this repository that writes to
// /etc and changes the OOM semantics of a running distribution, so its four
// protected paths are pinned here rather than argued in a comment. Every case
// runs the shipped script against a fabricated cgroup and a stub systemctl,
// because the real one cannot be asked to stop refusing OOMPolicy.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// guardScriptProbe is the fabricated host a case ran against.
type guardScriptProbe struct {
	cgroup string
	dropin string
}

// runWSLMemoryGuardScript runs the shipped payload against a fake cgroup and a
// stub systemctl, and returns its combined output, its exit code and the fake
// host so the caller can assert on what was written.
//
// refuseSetprop models systemd 257 refusing the resource-control properties;
// basePolicy is the policy read before the guard runs; dropinHonoured says
// whether the stub honours a drop-in once one is written.
func runWSLMemoryGuardScript(t *testing.T, refuseSetprop bool, basePolicy string, dropinHonoured bool) (string, int, guardScriptProbe) {
	t.Helper()

	root := t.TempDir()
	p := guardScriptProbe{
		cgroup: filepath.Join(root, "init.scope"),
		dropin: filepath.Join(root, "init.scope.d", "10-oompolicy.conf"),
	}
	if err := os.MkdirAll(p.cgroup, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, value string }{
		{"memory.high", "max"},
		{"memory.max", "max"},
		{"memory.swap.max", "max"},
		{"memory.oom.group", "1"},
	} {
		if err := os.WriteFile(filepath.Join(p.cgroup, f.name), []byte(f.value+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	meminfo := filepath.Join(root, "meminfo")
	if err := os.WriteFile(meminfo, []byte("MemTotal:        8388608 kB\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "systemctl", `#!/bin/sh
printf 'systemctl %s\n' "$*" >> "$STUB_STATE/systemctl.log"
case "$1" in
set-property)
	shift 3
	if [ "$STUB_REFUSE_SETPROP" = 1 ]; then exit 1; fi
	for pair in "$@"; do
		key=${pair%%=*}; val=${pair#*=}
		case "$key" in
		MemoryMax) mib=${val%M}; printf '%s\n' "$((mib*1048576))" > "$STUB_CGROUP/memory.max" ;;
		MemoryHigh) printf 'max\n' > "$STUB_CGROUP/memory.high" ;;
		MemorySwapMax) mib=${val%M}; printf '%s\n' "$((mib*1048576))" > "$STUB_CGROUP/memory.swap.max" ;;
		OOMPolicy) exit 1 ;;
		esac
	done
	exit 0 ;;
show)
	if [ "$STUB_DROPIN_HONOURED" = 1 ] && [ -f "$STUB_DROPIN" ]; then printf 'continue\n'; else printf '%s\n' "$STUB_BASE_POLICY"; fi
	exit 0 ;;
esac
exit 0
`)

	t.Setenv("STUB_STATE", root)
	t.Setenv("STUB_CGROUP", p.cgroup)
	t.Setenv("STUB_DROPIN", p.dropin)
	t.Setenv("STUB_BASE_POLICY", basePolicy)
	t.Setenv("STUB_DROPIN_HONOURED", map[bool]string{true: "1", false: "0"}[dropinHonoured])
	t.Setenv("STUB_REFUSE_SETPROP", map[bool]string{true: "1", false: "0"}[refuseSetprop])
	t.Setenv("WSL_GUARD_MEMINFO", meminfo)
	t.Setenv("WSL_GUARD_CGROUP", p.cgroup)
	t.Setenv("WSL_GUARD_SYSTEMCTL", filepath.Join(binDir, "systemctl"))
	t.Setenv("WSL_GUARD_OOMPOLICY_DROPIN", p.dropin)

	cmd := exec.Command("bash", filepath.Join(repoRoot(t), repoAssetWSLGuardScript))
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()

	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the guard: %v", err)
	}
	return string(out), exit, p
}

func readCgroup(t *testing.T, dir, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// TestWSLMemoryGuardScriptAppliesTheFourLimits pins S8 (both scoped routes,
// each verified by reading the policy back) and S9 (the guard never writes the
// manager-wide default that would reach every unit).
func TestWSLMemoryGuardScriptAppliesTheFourLimits(t *testing.T) {
	const wantMax = (8192 - 1536) * 1048576
	const wantSwap = 512 * 1048576

	t.Run("systemd accepts the property", func(t *testing.T) {
		out, exit, p := runWSLMemoryGuardScript(t, false, "continue", false)
		if exit != 0 {
			t.Fatalf("exit=%d\n%s", exit, out)
		}
		if got := readCgroup(t, p.cgroup, "memory.max"); got != fmt.Sprintf("%d", wantMax) {
			t.Errorf("memory.max=%q, want %d", got, wantMax)
		}
		if got := readCgroup(t, p.cgroup, "memory.high"); got != "max" {
			t.Errorf("memory.high=%q, want max", got)
		}
		if got := readCgroup(t, p.cgroup, "memory.swap.max"); got != fmt.Sprintf("%d", wantSwap) {
			t.Errorf("memory.swap.max=%q, want %d", got, wantSwap)
		}
		if got := readCgroup(t, p.cgroup, "memory.oom.group"); got != "0" {
			t.Errorf("memory.oom.group=%q, want 0", got)
		}
		if _, err := os.Stat(p.dropin); err == nil {
			t.Error("a scope that already reads the wanted policy must not need a drop-in")
		}
	})

	t.Run("systemd refuses the cap", func(t *testing.T) {
		out, exit, p := runWSLMemoryGuardScript(t, true, "continue", false)
		if exit != 1 || !strings.Contains(out, "NOT durable") {
			t.Errorf("exit=%d out=%q, want exit 1 naming the cap as NOT durable", exit, out)
		}
		if got := readCgroup(t, p.cgroup, "memory.max"); got == "max" {
			t.Error("the direct-write fallback must still cap this boot")
		}
	})

	t.Run("drop-in honoured", func(t *testing.T) {
		out, exit, p := runWSLMemoryGuardScript(t, false, "stop", true)
		if exit != 0 || !strings.Contains(out, "OOMPolicy is continue") {
			t.Errorf("exit=%d out=%q", exit, out)
		}
		if _, err := os.Stat(p.dropin); err != nil {
			t.Errorf("the drop-in was not written: %v", err)
		}
	})

	t.Run("drop-in ignored", func(t *testing.T) {
		out, exit, _ := runWSLMemoryGuardScript(t, false, "stop", false)
		if exit != 1 {
			t.Errorf("exit=%d, want 1", exit)
		}
		for _, want := range []string{"DefaultOOMPolicy=continue", "Restart=", "system.conf"} {
			if !strings.Contains(out, want) {
				t.Errorf("the remedy does not name %q:\n%s", want, out)
			}
		}
	})
}

// TestWSLMemoryGuardScriptLeavesAScopeItCannotReadAlone pins the boundaries: an
// input the guard cannot use must leave the boot uncapped and still exit 0, so
// a host without cgroup v2 never reads as a broken installation.
func TestWSLMemoryGuardScriptLeavesAScopeItCannotReadAlone(t *testing.T) {
	guard := filepath.Join(repoRoot(t), repoAssetWSLGuardScript)

	// cgroupWithControllers returns a cgroup directory that looks like a live
	// one, so only the input under test can decide the outcome.
	cgroupWithControllers := func(t *testing.T) string {
		t.Helper()

		dir := filepath.Join(t.TempDir(), "init.scope")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"memory.high", "memory.max", "memory.swap.max", "memory.oom.group"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("max\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	cases := []struct {
		name     string
		cgroup   func(t *testing.T) string
		meminfo  string
		wantLine string
	}{
		{
			name:     "no memory controller",
			cgroup:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") },
			meminfo:  "MemTotal:        8388608 kB\n",
			wantLine: "no memory controller",
		},
		{
			name:     "too small to divide",
			cgroup:   cgroupWithControllers,
			meminfo:  "MemTotal:        2097152 kB\n",
			wantLine: "too small",
		},
		{
			name:     "unreadable MemTotal",
			cgroup:   cgroupWithControllers,
			meminfo:  "no total on this line\n",
			wantLine: "could not read",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meminfo := filepath.Join(t.TempDir(), "meminfo")
			if err := os.WriteFile(meminfo, []byte(tc.meminfo), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("WSL_GUARD_MEMINFO", meminfo)
			t.Setenv("WSL_GUARD_CGROUP", tc.cgroup(t))
			t.Setenv("WSL_GUARD_SYSTEMCTL", "/bin/true")

			cmd := exec.Command("bash", guard)
			cmd.Env = os.Environ()
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("the guard must not fail a host it cannot read: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.wantLine) {
				t.Errorf("the report does not name %q:\n%s", tc.wantLine, out)
			}
		})
	}
}
