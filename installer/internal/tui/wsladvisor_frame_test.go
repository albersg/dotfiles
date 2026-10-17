package tui

import (
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE ADVISOR'S FRAME FIXTURES
// ============================================================================
//
// The advisor is reachable only inside a WSL instance, and a frame guard that measured whatever
// machine ran it would measure a different screen on a developer's laptop than in CI: on a plain
// Linux host the row is absent and the screen unreachable, and on a WSL host the report would carry
// that machine's mounts, memory and DNS. The host and the reading are therefore pinned here, and they
// are pinned from the same fixture the advisor's own tests use -- so the guard measures the screen
// this repository ships rather than the machine under it.

// wslAdvisorFrameModel is the advisor's screens over a pinned reading of a WSL instance whose current
// directory is a project on the Windows host: the case the whole utility exists for, and the one that
// puts the most rows on the report.
func wslAdvisorFrameModel(t *testing.T, screen Screen) Model {
	t.Helper()
	m := installerFrameModel(t, screen)
	m.SystemInfo = &system.SystemInfo{
		OS: system.OSWSL, OSName: "WSL", Arch: "x86_64",
		IsWSL: true, WSLVersion: 2, HomeDir: "/home/ana", UserShell: "zsh",
	}
	m.WSLRuntime = system.WSLRuntime{
		Inside: true, Version: 2, Distro: "Ubuntu-24.04", KernelMarked: true,
		Reason: "the kernel names WSL and the instance carries the artefacts a distribution has",
	}

	facts := healthyWSLLinuxFacts(t)
	m.WSLAdvisor = wslAdvisorState{
		Resolved: true,
		Facts:    facts,
		Findings: wslAdvisorFindings(facts),
	}

	switch screen {
	case ScreenWSLBenchmark:
		m.WSLAdvisor.Benchmark = wslBenchmarkState{Ran: true, Report: pinnedWSLBenchmarkReport(),
			Notice: "2 targets, 10 operations"}
	case ScreenWSLMigration:
		m.WSLAdvisor.Migration = wslAdvisorMigrationFrameState(facts, m)
	}
	return m
}

// wslAdvisorMigrationFrameState is the assisted copy at its preview, which is the phase with the most
// on screen: the two lists, the plan's own statistics, its warnings and its by-hand instructions.
func wslAdvisorMigrationFrameState(facts wslAdvisorFacts, m Model) wslMigrationState {
	sources := m.wslMigrationSources()
	state := wslMigrationState{
		Phase:        wslMigratePreview,
		Sources:      sources,
		Destinations: wslNativeDestinations(facts, "/windir/c/Users/ana/work/shop"),
		Source:       0,
		Destination:  0,
		Plan: system.MigrationPlan{
			Source:      "/windir/c/Users/ana/work/shop",
			Destination: "/home/ana/projects/shop",
			Exclusions:  wslMigrationDefaultExclusions,
			Stats: system.MigrationStats{
				Files: 418, Directories: 37, Symlinks: 2, Hidden: 6, GitRepositories: 1,
				Skipped: 1243, Bytes: 12 * 1024 * 1024, SkippedBytes: 184 * 1024 * 1024,
			},
			FreeBytes:  41 << 30,
			SpaceKnown: true,
			Warnings: []string{
				"A symlink pointing to /windir/c/Users/ana/work/shared will keep that absolute target.",
				"The .git directory of one repository is copied with it, so its remotes come along.",
			},
			Instructions: []string{
				"cp -a --no-clobber '/windir/c/Users/ana/work/shop' '/home/ana/projects/'",
				"the source removal is yours to run: rm -rf '/windir/c/Users/ana/work/shop'",
			},
		},
	}
	return state
}

// pinnedWSLBenchmarkReport is a finished measurement of two targets, built from the engine's own
// types with every rate derived from its own duration: the fixture cannot state a rate that
// contradicts the time it took, which is the one thing a hand-written table of numbers gets wrong.
func pinnedWSLBenchmarkReport() system.BenchReport {
	targets := []system.BenchTarget{
		{Name: "this project", Path: "/windir/c/Users/ana/work/shop"},
		{Name: "the distribution's storage", Path: "/home/ana"},
	}
	durations := [][]time.Duration{
		{420 * time.Millisecond, 96 * time.Millisecond, 61 * time.Millisecond, 780 * time.Millisecond, 240 * time.Millisecond},
		{18 * time.Millisecond, 6 * time.Millisecond, 4 * time.Millisecond, 22 * time.Millisecond, 9 * time.Millisecond},
	}
	report := system.BenchReport{
		Started:  goldenGreetingTime,
		Duration: 1652 * time.Millisecond,
		Cleaned:  true,
		Contamination: []string{
			"This machine's caches are warmer for the target measured second, because the first target's pass " +
				"read the same files.",
		},
	}
	for i, target := range targets {
		result := system.BenchResult{Target: target}
		for j, operation := range []system.BenchOperation{
			system.BenchWrite, system.BenchRead, system.BenchStat, system.BenchCreate, system.BenchRemove,
		} {
			result.Measurements = append(result.Measurements, fixtureBenchMeasurement(
				operation, 64, 64<<10, durations[i][j]))
		}
		if i == 0 {
			result.Notes = []string{"The create pass stopped at the file cap, so its rate covers a " +
				"shorter set than the write pass."}
		}
		report.Results = append(report.Results, result)
	}
	return report
}

// fixtureBenchMeasurement is one measurement with its rates derived from its own duration, so the
// fixture is internally consistent by construction.
func fixtureBenchMeasurement(operation system.BenchOperation, files int, bytes int64, duration time.Duration) system.BenchMeasurement {
	measurement := system.BenchMeasurement{
		Operation: operation, Files: files, Bytes: bytes, Duration: duration,
	}
	if seconds := duration.Seconds(); seconds > 0 {
		measurement.PerSecond = float64(files) / seconds
		measurement.BytesPerSecond = float64(bytes) / seconds
	}
	return measurement
}
