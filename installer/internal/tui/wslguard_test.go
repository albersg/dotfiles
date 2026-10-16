package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// fakeWSLGuardHost is one scripted machine for the guard: its environment
// (which resolves every destination), whether systemd is the init, the files
// that can be read (the installed artifacts and the cgroup attributes), the
// systemctl results, and the two sequences those reads can follow. Every host
// effect installWSLMemoryGuard and readWSLGuardState make goes through it, so
// no guard test needs systemd, root, a real cgroup or sudo.
type fakeWSLGuardHost struct {
	env         map[string]string
	systemd     bool
	files       map[string]string
	sequences   map[string][]string
	results     map[string]*system.ExecResult
	commands    []string
	installs    []fakeGuardInstall
	installErr  error
	readFailing map[string]bool
}

// fakeGuardInstall records one artifact the install path wrote: where it went,
// the mode it was given and the bytes. The mode is the whole point of the
// fixture -- the payload must be 0755 and the unit 0644.
type fakeGuardInstall struct {
	dst  string
	mode os.FileMode
	data string
}

// probe converts the fixture into the injected seam under test.
func (f *fakeWSLGuardHost) probe() wslGuardProbe {
	return wslGuardProbe{
		getenv: func(key string) string { return f.env[key] },
		install: func(data []byte, dst string, mode os.FileMode, stepID string) error {
			if f.installErr != nil {
				return f.installErr
			}
			f.installs = append(f.installs, fakeGuardInstall{dst: dst, mode: mode, data: string(data)})
			return nil
		},
		readFile: func(path string) ([]byte, error) {
			if f.readFailing[path] {
				return nil, os.ErrNotExist
			}
			if seq, ok := f.sequences[path]; ok && len(seq) > 0 {
				value := seq[0]
				if len(seq) > 1 {
					f.sequences[path] = seq[1:]
				}
				return []byte(value), nil
			}
			if content, ok := f.files[path]; ok {
				return []byte(content), nil
			}
			return nil, os.ErrNotExist
		},
		runSystemctl: func(args string, root bool, onLog system.LogCallback) *system.ExecResult {
			f.commands = append(f.commands, args)
			if result, ok := f.results[args]; ok {
				return result
			}
			return &system.ExecResult{Output: ""}
		},
		systemdRunning: func() bool { return f.systemd },
	}
}

// newWSLGuardFixture builds a throwaway checkout holding the guard payload and
// unit plus the destinations the install writes to, all under t.TempDir(). The
// script and unit live in the checkout because the installer reads them from
// there; nothing else touches the real /etc, /usr or /sys.
func newWSLGuardFixture(t *testing.T) (repoDir string, host *fakeWSLGuardHost) {
	t.Helper()

	repoDir = t.TempDir()
	for asset, content := range map[string]string{
		repoAssetWSLGuardScript: "#!/usr/bin/env bash\necho guard\n",
		repoAssetWSLGuardUnit:   "[Unit]\nDescription=guard\n",
	} {
		path := filepath.Join(repoDir, asset)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	root := t.TempDir()
	cgroupDir := filepath.Join(root, "init.scope")
	maxPath := filepath.Join(cgroupDir, wslGuardCgroupMax)
	host = &fakeWSLGuardHost{
		env: map[string]string{
			envWSLGuardScript:    filepath.Join(root, "wsl-memory-guard.sh"),
			envWSLGuardUnit:      filepath.Join(root, "wsl-memory-guard.service"),
			envWSLGuardCgroup:    cgroupDir,
			envWSLSystemdRunDir:  filepath.Join(root, "systemd"),
			envWSLGuardSystemctl: "systemctl",
		},
		systemd: true,
		files: map[string]string{
			filepath.Join(cgroupDir, wslGuardCgroupHigh): "max",
			maxPath: "8388608",
			filepath.Join(cgroupDir, wslGuardCgroupSwapMax): "536870912",
		},
		sequences: map[string][]string{},
		results: map[string]*system.ExecResult{
			"is-active " + wslGuardUnitName:        {Output: "active\n"},
			"show init.scope -p OOMPolicy --value": {Output: wslGuardWantOOMPolicy + "\n"},
		},
		readFailing: map[string]bool{},
	}
	return repoDir, host
}

// captureWSLGuardLog records every line the guard reports, so a test can assert
// the exact message each degradation path prints.
func captureWSLGuardLog(t *testing.T) *[]string {
	t.Helper()

	lines := &[]string{}
	original := wslLog
	wslLog = func(stepID, line string) {
		*lines = append(*lines, line)
	}
	t.Cleanup(func() { wslLog = original })
	return lines
}

func guardLogContains(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

// TestInstallWSLMemoryGuardInstallsAndVerifies is the happy path: both files
// land with the modes systemd needs, the unit is reloaded, enabled and
// restarted, and the cap is read back and then held across a second
// daemon-reload. The payload mode is the trap this test exists for.
func TestInstallWSLMemoryGuardInstallsAndVerifies(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("installWSLMemoryGuard() = %v, want nil", err)
	}

	scriptDst := host.env[envWSLGuardScript]
	unitDst := host.env[envWSLGuardUnit]
	maxPath := filepath.Join(host.env[envWSLGuardCgroup], wslGuardCgroupMax)

	if len(host.installs) != 2 {
		t.Fatalf("wrote %d artifacts, want the payload and the unit", len(host.installs))
	}
	if host.installs[0].dst != scriptDst || host.installs[0].mode != 0o755 {
		t.Errorf("payload install = %s mode %o, want %s mode 0755", host.installs[0].dst, host.installs[0].mode, scriptDst)
	}
	if host.installs[1].dst != unitDst || host.installs[1].mode != 0o644 {
		t.Errorf("unit install = %s mode %o, want %s mode 0644", host.installs[1].dst, host.installs[1].mode, unitDst)
	}

	wantCommands := []string{
		"daemon-reload",
		"enable " + wslGuardUnitName,
		"restart " + wslGuardUnitName,
		"show init.scope -p OOMPolicy --value",
		"daemon-reload",
	}
	if !reflect.DeepEqual(host.commands, wantCommands) {
		t.Errorf("systemctl commands = %v, want %v", host.commands, wantCommands)
	}

	for _, want := range []string{
		fmt.Sprintf("✓ wsl-memory-guard payload installed at %s (mode 0755)", scriptDst),
		fmt.Sprintf("✓ wsl-memory-guard unit installed at %s", unitDst),
		fmt.Sprintf("✓ wsl-memory-guard verified: init.scope OOMPolicy=continue and %s held at 8388608 across a daemon-reload", maxPath),
	} {
		if !guardLogContains(*lines, want) {
			t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
		}
	}
}

// TestInstallWSLMemoryGuardReportsAUnitThatExitedNonZero is S10 at the step:
// the restart status is captured, the guard's own report is printed either way,
// and the read-back still runs. It must not fail the caller.
func TestInstallWSLMemoryGuardReportsAUnitThatExitedNonZero(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)

	restart := "restart " + wslGuardUnitName
	host.results[restart] = &system.ExecResult{Error: fmt.Errorf("exit status 1")}
	host.results["show init.scope -p OOMPolicy --value"] = &system.ExecResult{Output: "stop\n"}

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("a failed unit must not fail the caller, got %v", err)
	}

	for _, want := range []string{
		fmt.Sprintf("wsl-memory-guard: the unit exited non-zero (systemctl restart: %v), so one of its protections is not in place; the guard's own report above names which one. The step continues.", host.results[restart].Error),
		`wsl-memory-guard: init.scope OOMPolicy reads "stop", want continue; one OOM kill can still stop the scope and take every process in the distribution with it`,
	} {
		if !guardLogContains(*lines, want) {
			t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
		}
	}

	// Every check still ran: the OOMPolicy read and the durability re-read
	// happened even though the unit failed.
	wantCommands := []string{
		"daemon-reload",
		"enable " + wslGuardUnitName,
		restart,
		"show init.scope -p OOMPolicy --value",
		"daemon-reload",
	}
	if !reflect.DeepEqual(host.commands, wantCommands) {
		t.Errorf("systemctl commands = %v, want %v", host.commands, wantCommands)
	}
}

// TestInstallWSLMemoryGuardReportsACapAClearedReload covers the failure mode the
// whole change is about: a cap that is not durable comes back as "max" after a
// daemon-reload, and the guard says exactly what it saw.
func TestInstallWSLMemoryGuardReportsACapAClearedReload(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)

	maxPath := filepath.Join(host.env[envWSLGuardCgroup], wslGuardCgroupMax)
	host.sequences[maxPath] = []string{"8388608", "max"}

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("a cleared cap must not fail the caller, got %v", err)
	}

	want := fmt.Sprintf("wsl-memory-guard: the cap did not survive a daemon-reload: %s read %q and then %q", maxPath, "8388608", "max")
	if !guardLogContains(*lines, want) {
		t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
	}
	for _, line := range *lines {
		if strings.Contains(line, "✓ wsl-memory-guard verified") {
			t.Errorf("a cap that a reload cleared must never be reported as verified: %q", line)
		}
	}
}

// TestInstallWSLMemoryGuardReportsAnUncappedScope covers the first read being
// "max": there is no cap to hold, so the guard says so instead of claiming one.
func TestInstallWSLMemoryGuardReportsAnUncappedScope(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)

	maxPath := filepath.Join(host.env[envWSLGuardCgroup], wslGuardCgroupMax)
	host.files[maxPath] = "max"

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("an uncapped scope must not fail the caller, got %v", err)
	}

	want := fmt.Sprintf("wsl-memory-guard: %s reads %q (want a finite number), so the scope is uncapped", maxPath, "max")
	if !guardLogContains(*lines, want) {
		t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
	}
}

// TestInstallWSLMemoryGuardSkipsWhenSystemdIsNotTheInit is the honest
// degradation for the distribution whose /run/systemd/system is absent: nothing
// is written, no systemctl runs, and the step is told what to do about it.
func TestInstallWSLMemoryGuardSkipsWhenSystemdIsNotTheInit(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)
	host.systemd = false

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("an absent systemd must not fail the caller, got %v", err)
	}

	if len(host.installs) != 0 {
		t.Errorf("nothing may be installed without systemd, got %v", host.installs)
	}
	if len(host.commands) != 0 {
		t.Errorf("no systemctl command may run without systemd, got %v", host.commands)
	}

	want := fmt.Sprintf("wsl-memory-guard: systemd is not the init of this distribution (%s is missing), so the guard unit cannot run. Set [boot] systemd=true in /etc/wsl.conf and restart WSL, then run the installer again.", host.env[envWSLSystemdRunDir])
	if !guardLogContains(*lines, want) {
		t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
	}
}

// TestInstallWSLMemoryGuardDryRunWritesNothing is the --dry-run contract at the
// guard: no file, no backup, no systemctl, and no record of having done any of
// it.
func TestInstallWSLMemoryGuardDryRunWritesNothing(t *testing.T) {
	lines := captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)
	t.Setenv("DOTFILES_DRY_RUN", "1")

	if err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe()); err != nil {
		t.Fatalf("a dry run must succeed, got %v", err)
	}

	if len(host.installs) != 0 {
		t.Errorf("a dry run wrote %d artifacts, want none", len(host.installs))
	}
	if len(host.commands) != 0 {
		t.Errorf("a dry run ran systemctl %v, want nothing", host.commands)
	}

	want := fmt.Sprintf("DRY RUN: wsl-memory-guard not installed (would write %s and %s, and run no systemctl)", host.env[envWSLGuardScript], host.env[envWSLGuardUnit])
	if !guardLogContains(*lines, want) {
		t.Errorf("the guard did not report %q\nreported:\n%s", want, strings.Join(*lines, "\n"))
	}
}

// TestInstallWSLMemoryGuardReportsAMissingPayload keeps the one genuine error
// honest: a checkout that does not ship the payload installs nothing, and the
// caller is told which file was missing.
func TestInstallWSLMemoryGuardReportsAMissingPayload(t *testing.T) {
	captureWSLGuardLog(t)
	repoDir, host := newWSLGuardFixture(t)
	if err := os.Remove(filepath.Join(repoDir, repoAssetWSLGuardScript)); err != nil {
		t.Fatal(err)
	}

	err := installWSLMemoryGuardWith(repoDir, wslStepID, host.probe())
	if err == nil {
		t.Fatal("a missing payload must be reported instead of installing nothing")
	}
	if !strings.Contains(err.Error(), repoAssetWSLGuardScript) {
		t.Errorf("the error should name the missing payload: %v", err)
	}
	if len(host.installs) != 0 {
		t.Errorf("nothing may be installed when the payload is missing, got %v", host.installs)
	}
}

// TestReadWSLGuardStateReportsEveryField is the read-only half: the guard's
// state comes back from the host, and the only commands issued are reads, never
// a systemctl write.
func TestReadWSLGuardStateReportsEveryField(t *testing.T) {
	repoDir, host := newWSLGuardFixture(t)
	host.files[host.env[envWSLGuardScript]] = "#!/usr/bin/env bash\n"
	host.files[host.env[envWSLGuardUnit]] = "[Unit]\n"

	st := readWSLGuardStateWith(repoDir, host.probe())

	if !st.Installed {
		t.Error("Installed = false, want true when both files are present")
	}
	if !st.Active {
		t.Error("Active = false, want true when the unit reports active")
	}
	if st.OOMPolicy != wslGuardWantOOMPolicy {
		t.Errorf("OOMPolicy = %q, want %q", st.OOMPolicy, wslGuardWantOOMPolicy)
	}
	if st.HighBytes != "max" {
		t.Errorf("HighBytes = %q, want %q", st.HighBytes, "max")
	}
	if st.MaxBytes != 8388608 {
		t.Errorf("MaxBytes = %d, want 8388608", st.MaxBytes)
	}
	if st.SwapBytes != 536870912 {
		t.Errorf("SwapBytes = %d, want 536870912", st.SwapBytes)
	}
	if st.Reason != "" {
		t.Errorf("Reason = %q, want empty when every field was read", st.Reason)
	}

	for _, command := range host.commands {
		if strings.HasPrefix(command, "daemon-reload") ||
			strings.HasPrefix(command, "enable") ||
			strings.HasPrefix(command, "restart") ||
			strings.HasPrefix(command, "set-property") {
			t.Errorf("the reader issued a systemctl write: %q", command)
		}
	}
}

// TestReadWSLGuardStateSaysWhyItCouldNotRead covers the unavailable answers:
// systemd absent, a cgroup attribute that cannot be read, and a
// missing memory.swap.max (unsupported, not a failure).
func TestReadWSLGuardStateSaysWhyItCouldNotRead(t *testing.T) {
	t.Run("systemd is not the init", func(t *testing.T) {
		repoDir, host := newWSLGuardFixture(t)
		host.systemd = false

		st := readWSLGuardStateWith(repoDir, host.probe())

		want := fmt.Sprintf("systemd is not the init of this distribution (%s is missing)", host.env[envWSLSystemdRunDir])
		if st.Reason != want {
			t.Errorf("Reason = %q, want %q", st.Reason, want)
		}
		if len(host.commands) != 0 {
			t.Errorf("no command may run when systemd is absent, got %v", host.commands)
		}
	})

	t.Run("the cgroup attribute cannot be read", func(t *testing.T) {
		repoDir, host := newWSLGuardFixture(t)
		host.readFailing[filepath.Join(host.env[envWSLGuardCgroup], wslGuardCgroupMax)] = true

		st := readWSLGuardStateWith(repoDir, host.probe())

		if st.Reason == "" {
			t.Error("Reason is empty, want the read failure named")
		}
		if st.MaxBytes != 0 {
			t.Errorf("MaxBytes = %d, want 0 when the attribute is unreadable", st.MaxBytes)
		}
	})

	t.Run("memory.swap.max is unsupported", func(t *testing.T) {
		repoDir, host := newWSLGuardFixture(t)
		host.readFailing[filepath.Join(host.env[envWSLGuardCgroup], wslGuardCgroupSwapMax)] = true

		st := readWSLGuardStateWith(repoDir, host.probe())

		if st.SwapBytes != 0 {
			t.Errorf("SwapBytes = %d, want 0 when swap accounting is unsupported", st.SwapBytes)
		}
		if st.Reason != "" {
			t.Errorf("Reason = %q, want empty: a kernel without swap accounting is not a read failure", st.Reason)
		}
	})
}

// TestApplyArtifactContentModeWritesTheModeItIsGiven is the mode trap at the
// writer: the guard payload is 0755 because systemd executes it, and the shared
// default entry point still writes the installer's 0644, which every other
// artifact relies on. It runs against a t.TempDir(), so it needs no root and no
// sudo.
func TestApplyArtifactContentModeWritesTheModeItIsGiven(t *testing.T) {
	dir := t.TempDir()

	payload := filepath.Join(dir, "wsl-memory-guard.sh")
	if err := applyArtifactContentMode([]byte("#!/bin/sh\n"), payload, 0o755, wslStepID); err != nil {
		t.Fatalf("applyArtifactContentMode() = %v", err)
	}
	if info, err := os.Stat(payload); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o755 {
		t.Errorf("payload mode = %o, want 0755", info.Mode().Perm())
	}

	unit := filepath.Join(dir, "wsl-memory-guard.service")
	if err := applyArtifactContent([]byte("[Unit]\n"), unit, wslStepID); err != nil {
		t.Fatalf("applyArtifactContent() = %v", err)
	}
	if info, err := os.Stat(unit); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o644 {
		t.Errorf("default artifact mode = %o, want the installer's 0644", info.Mode().Perm())
	}
}

// TestDefaultWSLGuardProbeIsWired is the shipped-seam check, in the style
// TestShellAuditBoundsEveryStart uses: every effect the guard needs must be
// present on the default probe, or a real run silently degrades.
func TestDefaultWSLGuardProbeIsWired(t *testing.T) {
	probe := defaultWSLGuardProbe()

	if probe.getenv == nil || probe.install == nil || probe.readFile == nil ||
		probe.runSystemctl == nil || probe.systemdRunning == nil {
		t.Fatal("the shipped probe is missing an effect the guard needs")
	}

	if got := wslGuardScriptDestination(probe.getenv); got != defaultWSLGuardScript {
		t.Errorf("payload destination = %q, want %q", got, defaultWSLGuardScript)
	}
	if got := wslGuardUnitDestination(probe.getenv); got != defaultWSLGuardUnit {
		t.Errorf("unit destination = %q, want %q", got, defaultWSLGuardUnit)
	}
	if got := wslGuardCgroupDir(probe.getenv); got != defaultWSLGuardCgroup {
		t.Errorf("cgroup directory = %q, want %q", got, defaultWSLGuardCgroup)
	}
	if got := wslGuardSystemdDir(probe.getenv); got != defaultSystemdRunDir {
		t.Errorf("systemd directory = %q, want %q", got, defaultSystemdRunDir)
	}
}
