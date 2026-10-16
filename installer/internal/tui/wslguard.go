package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE WSL MEMORY GUARD: THE INSTALLER'S HALF
// ============================================================================
//
// The guard is two files the WSL step installs and one verification the step
// runs afterwards: the payload (`dotfiles-wsl/wsl-memory-guard.sh`), the unit
// (`dotfiles-wsl/wsl-memory-guard.service`), and a read-back that the cap
// systemd knows about survives a `daemon-reload`. The measured failure this
// change answers is in the task: in WSL every terminal process lands in
// `init.scope`, that scope carried `MemoryMax=infinity`, and a single runaway
// workload could take the whole distribution down with it -- or freeze it.
//
// The payload owns all of the policy. It computes the cap from `/proc/meminfo`,
// it asks systemd for the property, it falls back to a drop-in scoped to
// `init.scope` for OOMPolicy, and it verifies each of those itself. Nothing here
// duplicates any of that: this half installs the two files, runs the unit, and
// then reads the host back to see whether the protections are actually in
// place. That is what "verify by reading, never by assuming" means here.
//
// Every host effect goes through wslGuardProbe, so a test can drive installed,
// systemd absent, a refused property and a cap a reload clears without systemd,
// root, a real cgroup or sudo. The real probe is defaultWSLGuardProbe, in the
// same shape system.hostResourceProbe uses.

const (
	// defaultWSLGuardScript is where the payload is installed. It is outside the
	// home directory because the unit executes it as root, and it is beside
	// nothing else so the unit's ExecStart names it exactly.
	defaultWSLGuardScript = "/usr/local/lib/dotfiles-wsl/wsl-memory-guard.sh"

	// defaultWSLGuardUnit is where the unit is installed, which is one of the
	// directories systemd reads units from.
	defaultWSLGuardUnit = "/etc/systemd/system/wsl-memory-guard.service"

	// defaultWSLGuardCgroup is the cgroup the guard caps. In WSL every terminal
	// process lives in init.scope, so it is the only cgroup that can hold the
	// limit for the whole distribution.
	defaultWSLGuardCgroup = "/sys/fs/cgroup/init.scope"

	// defaultSystemdRunDir is the directory systemd creates when it is the init
	// system. Its absence is exactly the state `[boot] systemd=true` in
	// dotfiles-wsl/wsl.conf exists to change, so it is what decides whether the
	// guard can run at all.
	defaultSystemdRunDir = "/run/systemd/system"

	// envWSLGuardScript, envWSLGuardUnit, envWSLGuardCgroup,
	// envWSLSystemdRunDir and envWSLGuardSystemctl override the guard's host
	// paths. They exist for tests and for a distribution that keeps systemd's
	// files somewhere unusual, and they are what lets the interactive route's
	// generated script be exercised against a temporary directory instead of
	// /etc.
	envWSLGuardScript    = "DOTFILES_WSL_GUARD_SCRIPT"
	envWSLGuardUnit      = "DOTFILES_WSL_GUARD_UNIT"
	envWSLGuardCgroup    = "DOTFILES_WSL_GUARD_CGROUP"
	envWSLSystemdRunDir  = "DOTFILES_WSL_SYSTEMD_DIR"
	envWSLGuardSystemctl = "DOTFILES_WSL_GUARD_SYSTEMCTL"

	// wslGuardUnitName is the unit's name as systemctl knows it.
	wslGuardUnitName = "wsl-memory-guard.service"

	// wslGuardPayloadMode is the mode the payload must have: systemd executes it
	// as root, so a 0644 write would install a file that can never run.
	wslGuardPayloadMode os.FileMode = 0o755

	// wslGuardUnitMode is the mode the unit is installed with. It is read, not
	// executed, so 0644 is enough.
	wslGuardUnitMode os.FileMode = 0o644

	// wslGuardWantOOMPolicy is the effective policy that keeps one OOM kill
	// inside the scope from making systemd stop the scope -- and, in WSL, every
	// process in the distribution with it.
	wslGuardWantOOMPolicy = "continue"

	// The cgroup v2 attributes the guard reads back. memory.high is read as a
	// string because "max" is a real answer (no throttle band), not a number.
	wslGuardCgroupHigh    = "memory.high"
	wslGuardCgroupMax     = "memory.max"
	wslGuardCgroupSwapMax = "memory.swap.max"
)

// wslGuardState is the guard's state as read from the host.
//
// Every field is an observation, never an assumption: a value that could not be
// read is zero, and Reason says why at least one of them could not be read.
type wslGuardState struct {
	// Installed reports whether both files are where systemd reads them.
	Installed bool
	// Active reports whether the unit is active.
	Active bool
	// OOMPolicy is the effective OOMPolicy of init.scope.
	OOMPolicy string
	// HighBytes is memory.high as the cgroup holds it ("max" or a number).
	HighBytes string
	// MaxBytes is memory.max, 0 when unreadable.
	MaxBytes uint64
	// SwapBytes is memory.swap.max, 0 when unreadable or unsupported.
	SwapBytes uint64
	// Reason is why the state could not be read, empty when it could.
	Reason string
}

// wslGuardProbe carries every host effect the guard's installer and reader
// need, so a test can replace all of them. defaultWSLGuardProbe builds the real
// one, and no test may reach systemd, a real cgroup, /etc or sudo.
type wslGuardProbe struct {
	// getenv resolves the guard's destinations and the systemctl binary.
	getenv func(string) string
	// install writes one artifact with an explicit mode through the installer's
	// backup-and-escalate writer. The mode is a parameter because the payload
	// must be 0755; applyArtifactContent's 0644 would install a file systemd
	// cannot execute.
	install func(data []byte, dst string, mode os.FileMode, stepID string) error
	// readFile reads a file: the installed artifacts and the cgroup attributes.
	readFile func(path string) ([]byte, error)
	// runSystemctl runs one systemctl invocation. The mutating verbs pass
	// root=true so the production probe escalates with sudo -n; the read-only
	// verbs do not.
	runSystemctl func(args string, root bool, onLog system.LogCallback) *system.ExecResult
	// systemdRunning reports whether systemd is the init system, which /run/
	// systemd/system decides.
	systemdRunning func() bool
}

// defaultWSLGuardProbe returns the real probe: the process environment, the
// installer's own artifact writer, the real filesystem, sudo -n for the
// mutating systemctl verbs, and /run/systemd/system for the init check.
func defaultWSLGuardProbe() wslGuardProbe {
	return wslGuardProbe{
		getenv:   os.Getenv,
		install:  applyArtifactContentMode,
		readFile: os.ReadFile,
		runSystemctl: func(args string, root bool, onLog system.LogCallback) *system.ExecResult {
			command := wslGuardSystemctlBinary(os.Getenv) + " " + args
			if root {
				return runSudoWithLogs(command, nil, onLog)
			}
			return system.RunWithLogs(command, nil, onLog)
		},
		systemdRunning: func() bool {
			return system.DirExists(wslGuardSystemdDir(os.Getenv))
		},
	}
}

// wslGuardEnvOr resolves one destination: the override when it is set and
// non-empty, the default otherwise. An empty override is not a path, so it does
// not shadow the default.
func wslGuardEnvOr(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func wslGuardScriptDestination(getenv func(string) string) string {
	return wslGuardEnvOr(getenv, envWSLGuardScript, defaultWSLGuardScript)
}

func wslGuardUnitDestination(getenv func(string) string) string {
	return wslGuardEnvOr(getenv, envWSLGuardUnit, defaultWSLGuardUnit)
}

func wslGuardCgroupDir(getenv func(string) string) string {
	return wslGuardEnvOr(getenv, envWSLGuardCgroup, defaultWSLGuardCgroup)
}

func wslGuardSystemdDir(getenv func(string) string) string {
	return wslGuardEnvOr(getenv, envWSLSystemdRunDir, defaultSystemdRunDir)
}

func wslGuardSystemctlBinary(getenv func(string) string) string {
	return wslGuardEnvOr(getenv, envWSLGuardSystemctl, "systemctl")
}

// installWSLMemoryGuard installs the guard and proves, by reading the host, what
// actually took effect. It degrades honestly instead of failing: an absent
// systemd, a property systemd refused and a cap a reload clears each produce
// their own line naming what was observed, and the step continues.
//
// The error it returns is an installation one, and it fires only when the
// payload or the unit could not be put in place: a checkout that does not ship
// them, or a host that refused the write. Nothing it then measures about the
// running scope is an error -- an unreadable policy, a refused property and a
// cap a reload clears are all reported as lines -- because the step calls this
// for its report and carries on either way.
func installWSLMemoryGuard(repoDir, stepID string) error {
	return installWSLMemoryGuardWith(repoDir, stepID, defaultWSLGuardProbe())
}

// installWSLMemoryGuardWith is installWSLMemoryGuard with every host effect
// injected, in the shape system.detectHostResources uses so the failure paths
// can be exercised without a machine that has systemd and root.
func installWSLMemoryGuardWith(repoDir, stepID string, probe wslGuardProbe) error {
	scriptDst := wslGuardScriptDestination(probe.getenv)
	unitDst := wslGuardUnitDestination(probe.getenv)
	cgroupDir := wslGuardCgroupDir(probe.getenv)

	// --dry-run writes no file, creates no backup, runs no systemctl and
	// records nothing, exactly as writeWSLConfig is gated. It is checked before
	// anything is probed, so a dry run touches no part of the host at all.
	if dryRun() {
		wslLog(stepID, fmt.Sprintf("DRY RUN: wsl-memory-guard not installed (would write %s and %s, and run no systemctl)", scriptDst, unitDst))
		return nil
	}

	// systemd is what makes the unit runnable, and it is not the init of a
	// distribution that has not set [boot] systemd=true. Say so and leave the
	// files unwritten: an installed unit systemd will never read is not a
	// safeguard.
	if !probe.systemdRunning() {
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: systemd is not the init of this distribution (%s is missing), so the guard unit cannot run. Set [boot] systemd=true in /etc/wsl.conf and restart WSL, then run the installer again.", wslGuardSystemdDir(probe.getenv)))
		return nil
	}

	script, err := os.ReadFile(filepath.Join(repoDir, repoAssetWSLGuardScript))
	if err != nil {
		return fmt.Errorf("reading %s from %s: %w", repoAssetWSLGuardScript, repoDir, err)
	}
	unit, err := os.ReadFile(filepath.Join(repoDir, repoAssetWSLGuardUnit))
	if err != nil {
		return fmt.Errorf("reading %s from %s: %w", repoAssetWSLGuardUnit, repoDir, err)
	}

	// The payload is 0755 so systemd can execute it; the unit is 0644.
	if err := probe.install(script, scriptDst, wslGuardPayloadMode, stepID); err != nil {
		return fmt.Errorf("installing the WSL memory guard payload at %s: %w", scriptDst, err)
	}
	wslLog(stepID, fmt.Sprintf("✓ wsl-memory-guard payload installed at %s (mode 0755)", scriptDst))

	if err := probe.install(unit, unitDst, wslGuardUnitMode, stepID); err != nil {
		return fmt.Errorf("installing the WSL memory guard unit at %s: %w", unitDst, err)
	}
	wslLog(stepID, fmt.Sprintf("✓ wsl-memory-guard unit installed at %s", unitDst))

	// The guard's own report streams through onLog, so a unit that exits
	// non-zero still prints the line naming the protection that is missing. The
	// restart status is captured rather than run under `set -e`, which is S10:
	// the first hand-run guard died at the failed restart and never printed its
	// own verdict.
	log := func(line string) { wslLog(stepID, line) }
	for _, args := range []string{
		"daemon-reload",
		"enable " + wslGuardUnitName,
		"restart " + wslGuardUnitName,
	} {
		result := probe.runSystemctl(args, true, log)
		if result.Error == nil {
			continue
		}
		if args == "restart "+wslGuardUnitName {
			wslLog(stepID, fmt.Sprintf("wsl-memory-guard: the unit exited non-zero (systemctl restart: %v), so one of its protections is not in place; the guard's own report above names which one. The step continues.", result.Error))
			continue
		}
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: systemctl %s failed: %v", args, result.Error))
	}

	verifyWSLGuard(stepID, cgroupDir, probe)
	return nil
}

// verifyWSLGuard reads back what the install actually produced. It checks the
// effective OOMPolicy of init.scope and that memory.max is a finite number,
// then runs a second daemon-reload and requires the cap to be unchanged. A
// value that does not match is reported with the value observed, and never
// turned into a failure: the step continues either way.
func verifyWSLGuard(stepID, cgroupDir string, probe wslGuardProbe) {
	maxPath := filepath.Join(cgroupDir, wslGuardCgroupMax)
	ok := true

	policy, policyErr := wslGuardOOMPolicy(probe)
	switch {
	case policyErr != nil:
		ok = false
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: the effective init.scope OOMPolicy could not be read (%v), so the guard's protection is unproven", policyErr))
	case policy != wslGuardWantOOMPolicy:
		ok = false
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: init.scope OOMPolicy reads %q, want %s; one OOM kill can still stop the scope and take every process in the distribution with it", policy, wslGuardWantOOMPolicy))
	}

	maxBefore, maxBeforeErr := wslGuardCgroupField(probe, cgroupDir, wslGuardCgroupMax)
	if maxBeforeErr != nil || parseWSLGuardBytes(maxBefore) == 0 {
		ok = false
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: %s reads %q (want a finite number), so the scope is uncapped", maxPath, wslGuardObserved(maxBefore, maxBeforeErr)))
	}

	// The durability check runs whatever the first read said: a cap that is
	// there and then clears is the failure this verification exists for, and
	// skipping the reload for a cap that was never there would hide it.
	probe.runSystemctl("daemon-reload", true, func(line string) { wslLog(stepID, line) })
	maxAfter, maxAfterErr := wslGuardCgroupField(probe, cgroupDir, wslGuardCgroupMax)
	if maxAfterErr != nil || maxAfter != maxBefore {
		ok = false
		wslLog(stepID, fmt.Sprintf("wsl-memory-guard: the cap did not survive a daemon-reload: %s read %q and then %q", maxPath, wslGuardObserved(maxBefore, maxBeforeErr), wslGuardObserved(maxAfter, maxAfterErr)))
	}

	if ok {
		wslLog(stepID, fmt.Sprintf("✓ wsl-memory-guard verified: init.scope OOMPolicy=%s and %s held at %s across a daemon-reload", policy, maxPath, maxBefore))
	}
}

// wslGuardObserved renders a cgroup read for a report: the value when it was
// read, and why not when it was not. A report that printed an empty string for
// an unreadable attribute would read as "unlimited" instead of "unread".
func wslGuardObserved(raw string, err error) string {
	if err != nil {
		return "unreadable: " + err.Error()
	}
	return raw
}

// wslGuardOOMPolicy reads the effective OOMPolicy of init.scope. It is a read;
// the payload owns the two write routes and their verification.
func wslGuardOOMPolicy(probe wslGuardProbe) (string, error) {
	result := probe.runSystemctl("show init.scope -p OOMPolicy --value", false, nil)
	if result.Error != nil {
		return "", result.Error
	}
	return strings.TrimSpace(result.Output), nil
}

// wslGuardCgroupField reads one cgroup attribute as text.
func wslGuardCgroupField(probe wslGuardProbe, cgroupDir, name string) (string, error) {
	data, err := probe.readFile(filepath.Join(cgroupDir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// parseWSLGuardBytes reads a cgroup byte count. The literal "max" means the
// attribute is unlimited, which the guard reports as 0 -- the same "not a
// finite cap" the verification and the Utilities screen care about.
func parseWSLGuardBytes(raw string) uint64 {
	value := strings.TrimSpace(raw)
	if value == "" || value == "max" {
		return 0
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// readWSLGuardState reports what the guard holds on this host. It is read-only:
// it runs no systemctl write, so the Utilities screen and any other reader can
// call it without changing the machine.
//
// repoDir is not consulted -- the state is read from the host's own paths -- but
// it is accepted so the reader shares the installer's call shape.
func readWSLGuardState(repoDir string) wslGuardState {
	return readWSLGuardStateWith(repoDir, defaultWSLGuardProbe())
}

// readWSLGuardStateWith is readWSLGuardState with every host effect injected.
func readWSLGuardStateWith(_ string, probe wslGuardProbe) wslGuardState {
	st := wslGuardState{}

	scriptDst := wslGuardScriptDestination(probe.getenv)
	unitDst := wslGuardUnitDestination(probe.getenv)
	cgroupDir := wslGuardCgroupDir(probe.getenv)

	_, scriptErr := probe.readFile(scriptDst)
	_, unitErr := probe.readFile(unitDst)
	st.Installed = scriptErr == nil && unitErr == nil

	if !probe.systemdRunning() {
		st.Reason = fmt.Sprintf("systemd is not the init of this distribution (%s is missing)", wslGuardSystemdDir(probe.getenv))
		return st
	}

	// is-active answers "not active" for a unit that is absent, which is a
	// successful read of the state, not a failure to read it, so it never sets
	// Reason.
	active := probe.runSystemctl("is-active "+wslGuardUnitName, false, nil)
	st.Active = active.Error == nil && strings.TrimSpace(active.Output) == "active"

	if policy, err := wslGuardOOMPolicy(probe); err != nil {
		st.Reason = err.Error()
	} else {
		st.OOMPolicy = policy
	}

	if high, err := wslGuardCgroupField(probe, cgroupDir, wslGuardCgroupHigh); err == nil {
		st.HighBytes = high
	} else if st.Reason == "" {
		st.Reason = err.Error()
	}

	if max, err := wslGuardCgroupField(probe, cgroupDir, wslGuardCgroupMax); err == nil {
		st.MaxBytes = parseWSLGuardBytes(max)
	} else if st.Reason == "" {
		st.Reason = err.Error()
	}

	// memory.swap.max is absent on a kernel built without swap accounting. That
	// is "unsupported", not a failed read, so it is reported as 0 without
	// claiming the state could not be read.
	if swap, err := wslGuardCgroupField(probe, cgroupDir, wslGuardCgroupSwapMax); err == nil {
		st.SwapBytes = parseWSLGuardBytes(swap)
	}

	return st
}
