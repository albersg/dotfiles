package tui

import (
	"bytes"
	"text/template"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// WSLResources is the machine-derived part of a rendered .wslconfig.
//
// A zero field means "omit the key", which is a deliberate answer rather than a
// missing one: WSL then applies its own proportional default, computed from the
// real host by Windows. Omitting a key is always better than writing a limit
// that does not fit the machine, so every failure path in the host detection
// lands here as zeros.
type WSLResources struct {
	MemoryMB   int // 0 means "omit the key"
	Processors int // 0 means "omit the key"
	SwapMB     int // 0 means "omit the key"
}

const (
	bytesPerMiB = 1 << 20

	// wslMemoryStepMB is the granularity memory and swap are rounded down to.
	// It is policy rather than a WSL requirement: whole 512 MiB steps keep the
	// rendered file readable and stop the plan from promising a capacity the
	// rounding would not survive.
	wslMemoryStepMB = 512

	// wslMemoryReserveBytes is what the policy leaves for Windows itself. WSL2
	// runs on top of Windows, so a VM limit of half the host can still starve
	// the desktop on a small machine; the reserve is the floor Windows keeps.
	wslMemoryReserveBytes = 2 << 30 // 2 GiB

	// wslMinMemoryBytes is the smallest VM limit worth writing. Below 1 GiB the
	// distribution is too cramped to be useful and WSL's own default is better.
	wslMinMemoryBytes = 1 << 30 // 1 GiB
)

// PlanWSLResources turns host capacities into the values the .wslconfig template
// should render. It is pure: no environment, no filesystem, no processes.
//
// The policy is WSL's own proportional default made explicit and clamped to what
// the host can actually give:
//   - memory: half the host, but never so much that Windows is left below 2 GiB,
//     rounded down to wslMemoryStepMB, and omitted below wslMinMemoryBytes;
//   - processors: every host logical CPU, omitted when the host is unknown;
//   - swap: a quarter of the planned memory, rounded down to the same step, and
//     omitted when that rounds to nothing.
//
// Rounding happens in MiB, because .wslconfig expresses memory in MB. The host
// reports bytes, so the conversion happens once, in integer arithmetic, and no
// fractional MiB is ever produced.
func PlanWSLResources(host system.HostResources) WSLResources {
	plan := WSLResources{}

	if host.LogicalCPUs > 0 {
		plan.Processors = host.LogicalCPUs
	}

	plan.MemoryMB = planMemoryMB(host.MemoryBytes)
	if plan.MemoryMB > 0 {
		plan.SwapMB = planSwapMB(plan.MemoryMB)
	}

	return plan
}

// planMemoryMB applies the memory policy. An unknown host (0 bytes) and a host
// at or below the Windows reserve have no room to give, and both omit the key.
//
// The reserve is checked before it is subtracted rather than computed and
// clamped afterwards: MemoryBytes is unsigned, so subtracting the reserve
// unconditionally would wrap around on a sub-2 GiB host and report a huge
// "available" value.
func planMemoryMB(memoryBytes uint64) int {
	if memoryBytes <= wslMemoryReserveBytes {
		return 0
	}

	target := memoryBytes / 2
	if available := memoryBytes - wslMemoryReserveBytes; available < target {
		target = available
	}

	if target < wslMinMemoryBytes {
		return 0
	}

	// The division is exact on every host WSL runs on: WSL needs 64-bit Windows,
	// where int holds any plausible VM size.
	return int(roundDownToMemoryStep(target) / bytesPerMiB)
}

// planSwapMB applies the swap policy: a quarter of the planned memory, rounded
// down to the same step as memory. A quarter of the smallest plan is below one
// step, which is the documented "omit" case.
func planSwapMB(memoryMB int) int {
	return memoryMB / 4 / wslMemoryStepMB * wslMemoryStepMB
}

// roundDownToMemoryStep rounds a byte count down to a whole wslMemoryStepMB step.
func roundDownToMemoryStep(bytes uint64) uint64 {
	step := uint64(wslMemoryStepMB) * bytesPerMiB
	return bytes / step * step
}

// RenderWSLConfig renders the shipped .wslconfig template for one host plan.
//
// It is pure: the same template text and the same plan always produce the same
// bytes, which is what lets the step and the interactive script be checked
// against each other. A template that does not parse, or one that fails while
// executing against the plan, is returned as an error instead of a partially
// written file: a truncated .wslconfig would still be read by WSL.
func RenderWSLConfig(templateText string, res WSLResources) ([]byte, error) {
	tmpl, err := template.New("wslconfig").Parse(templateText)
	if err != nil {
		return nil, err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, res); err != nil {
		return nil, err
	}

	return rendered.Bytes(), nil
}
