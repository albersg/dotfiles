package system

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The machine's own pulse, read the cheapest way each platform allows.
//
// Everything here is a command, never a renderer: the readers touch /proc, spawn
// a sysctl or ask the filesystem, all of which are I/O, and a render that did
// any of it could not be snapshotted and would change its own output. The
// installer samples on a tick and stores the results on the model, so the screen
// only ever draws samples it was given.
//
// A reading degrades row by row, not as a whole. Every field carries its own
// *OK flag, and a reader clears the flag of anything the host does not report;
// Termux, an unknown platform and an unreadable file all come back with the
// flags clear rather than with zeros dressed up as a measurement. That is what
// lets a panel leave the row out instead of printing "0%".
//
// Two of the readings (CPU busy and, on Linux, nothing else) are rates rather
// than gauges and need the previous sample to be computed. They are kept in two
// pieces: Sample is one raw reading from the platform, and Derive turns a pair of
// them into the display-ready Metrics. The pair lives on the model, so the delta
// is model state and a snapshot pins it like any other field.

// Sample is one raw platform reading. It is platform-shaped but not
// platform-typed: every reader fills the fields it can and clears the flag of
// every field it cannot.
type Sample struct {
	// CPU counters are cumulative since boot; CPU busy is the busy share of the
	// difference between two samples, which is why a single sample carries no
	// rate of its own.
	CPUOK             bool
	CPUTotal, CPUIdle uint64

	MemOK             bool
	MemUsed, MemTotal uint64

	LoadOK               bool
	Load1, Load5, Load15 float64

	DiskOK              bool
	DiskFree, DiskTotal uint64

	ProcOK    bool
	ProcCount int
}

// Metrics is a display-ready reading: the gauges copied from a Sample plus the
// CPU rate the previous Sample supplied. The *OK flags are the same contract as
// Sample's: false means "this host does not report it", never "it is zero".
type Metrics struct {
	CPUOK   bool
	CPUBusy float64 // 0..1

	MemOK             bool
	MemUsed, MemTotal uint64

	LoadOK               bool
	Load1, Load5, Load15 float64

	DiskOK              bool
	DiskFree, DiskTotal uint64

	ProcOK    bool
	ProcCount int
}

// Derive turns a raw reading into display-ready Metrics using the previous raw
// reading for the rates. A first sample (prev is the zero Sample, or before a
// counter reset) has no interval to measure over, so the CPU row is left out
// rather than reported as idle.
func Derive(prev, cur Sample) Metrics {
	m := Metrics{
		MemOK:    cur.MemOK,
		MemUsed:  cur.MemUsed,
		MemTotal: cur.MemTotal,

		LoadOK: cur.LoadOK,
		Load1:  cur.Load1,
		Load5:  cur.Load5,
		Load15: cur.Load15,

		DiskOK:    cur.DiskOK,
		DiskFree:  cur.DiskFree,
		DiskTotal: cur.DiskTotal,

		ProcOK:    cur.ProcOK,
		ProcCount: cur.ProcCount,
	}

	if prev.CPUOK && cur.CPUOK &&
		cur.CPUTotal > prev.CPUTotal &&
		cur.CPUIdle >= prev.CPUIdle {
		dTotal := cur.CPUTotal - prev.CPUTotal
		dIdle := cur.CPUIdle - prev.CPUIdle
		if dIdle <= dTotal {
			m.CPUOK = true
			m.CPUBusy = float64(dTotal-dIdle) / float64(dTotal)
		}
	}
	return m
}

// ReadSample reads the host's pulse from the cheapest source the platform has.
// It is called from a command, so the I/O it does never lands on a render path.
func ReadSample() Sample {
	return readSample(defaultMetricsProbe())
}

// metricsCommandTimeout bounds the macOS sysctl/ps reads. A probe that hangs is
// worse than a missing row, so a timeout degrades exactly like an unreadable
// file.
const metricsCommandTimeout = 2 * time.Second

// metricsProbe carries every host effect the readers need, so a test can feed
// the real file formats and the failure paths without a /proc, a sysctl or a
// real HOME.
type metricsProbe struct {
	goos     string
	termux   bool
	home     string
	readFile func(string) ([]byte, error)
	run      func(name string, args ...string) ([]byte, error)
	statfs   func(path string) (free, total uint64, ok bool)
}

func defaultMetricsProbe() metricsProbe {
	return metricsProbe{
		goos:     runtime.GOOS,
		termux:   isTermux(),
		home:     os.Getenv("HOME"),
		readFile: os.ReadFile,
		run:      runMetricsCommand,
		statfs:   statfsFree,
	}
}

// readSample dispatches to the platform reader. Termux is refused before the
// platform is even looked at: Android restricts most of /proc, and a partial
// reading that silently loses the CPU row is worse than a host honestly
// reporting nothing.
func readSample(p metricsProbe) Sample {
	if p.termux {
		return Sample{}
	}
	switch p.goos {
	case "linux":
		return readLinuxSample(p)
	case "darwin":
		return readDarwinSample(p)
	default:
		return Sample{}
	}
}

// ============================================================================
// LINUX (and WSL, which is Linux with a Windows host above it)
// ============================================================================

func readLinuxSample(p metricsProbe) Sample {
	var s Sample

	if data, err := p.readFile("/proc/stat"); err == nil {
		if total, idle, ok := parseProcStatCPU(data); ok {
			s.CPUOK, s.CPUTotal, s.CPUIdle = true, total, idle
		}
	}
	if data, err := p.readFile("/proc/meminfo"); err == nil {
		if used, total, ok := parseMeminfo(data); ok {
			s.MemOK, s.MemUsed, s.MemTotal = true, used, total
		}
	}
	if data, err := p.readFile("/proc/loadavg"); err == nil {
		if l1, l5, l15, ok := parseLoadavg(data); ok {
			s.LoadOK, s.Load1, s.Load5, s.Load15 = true, l1, l5, l15
		}
		if n, ok := parseProcCount(data); ok {
			s.ProcOK, s.ProcCount = true, n
		}
	}
	if free, total, ok := probeStatfs(p, p.home); ok {
		s.DiskOK, s.DiskFree, s.DiskTotal = true, free, total
	}
	return s
}

// parseProcStatCPU reads the aggregate "cpu" line of /proc/stat. Its fields are
// user nice system idle iowait irq softirq steal guest guest_nice; busy is every
// field but idle and iowait, so the total and the idle pair is enough for the
// delta Derive computes. Extra fields the kernel may add are summed as busy,
// which is the honest default: a field the parser does not know is still CPU
// time the host spent.
func parseProcStatCPU(data []byte) (total, idle uint64, ok bool) {
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 4 {
			return 0, 0, false
		}
		values := make([]uint64, 0, len(fields))
		for _, f := range fields {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			values = append(values, v)
		}
		for _, v := range values {
			total += v
		}
		idle = values[3]
		if len(values) > 4 {
			idle += values[4] // iowait is not CPU the machine spent working
		}
		return total, idle, true
	}
	return 0, 0, false
}

// parseMeminfo reads MemTotal and the used share from /proc/meminfo.
// MemAvailable is preferred because MemFree excludes reclaimable cache and
// would read as "full" on a healthy desktop; MemFree is the fallback. Used is
// total minus available, clamped so a nonsense pair cannot underflow.
func parseMeminfo(data []byte) (used, total uint64, ok bool) {
	var memTotal, memAvail, memFree uint64
	var haveAvail, haveFree bool

	for _, line := range strings.Split(string(data), "\n") {
		key, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			memTotal = kb * 1024
		case "MemAvailable":
			memAvail, haveAvail = kb*1024, true
		case "MemFree":
			memFree, haveFree = kb*1024, true
		}
	}
	if memTotal == 0 {
		return 0, 0, false
	}
	switch {
	case haveAvail:
		used = memTotal - min(memAvail, memTotal)
	case haveFree:
		used = memTotal - min(memFree, memTotal)
	default:
		return 0, 0, false
	}
	return used, memTotal, true
}

// parseLoadavg reads the three load averages from /proc/loadavg.
func parseLoadavg(data []byte) (l1, l5, l15 float64, ok bool) {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	values := make([]float64, 3)
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, 0, 0, false
		}
		values[i] = v
	}
	return values[0], values[1], values[2], true
}

// parseProcCount reads the fourth field of /proc/loadavg, "runnable/total", for
// the total number of processes. The file is already read for the load, so the
// count costs nothing extra.
func parseProcCount(data []byte) (int, bool) {
	fields := strings.Fields(string(data))
	if len(fields) < 4 {
		return 0, false
	}
	_, total, found := strings.Cut(fields[3], "/")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(total)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// ============================================================================
// MACOS
// ============================================================================

func readDarwinSample(p metricsProbe) Sample {
	var s Sample

	// kern.cp_time is the same cumulative tick counter /proc/stat exposes on
	// Linux: user, nice, sys, intr and idle. Idle is the last field, so busy is
	// everything before it.
	if out, err := p.run("sysctl", "-n", "kern.cp_time"); err == nil {
		if total, idle, ok := parseCPTime(out); ok {
			s.CPUOK, s.CPUTotal, s.CPUIdle = true, total, idle
		}
	}

	total, totalOK := darwinMemoryTotal(p)
	used, usedOK := darwinMemoryUsed(p)
	if totalOK && usedOK {
		s.MemOK, s.MemUsed, s.MemTotal = true, min(used, total), total
	}

	if out, err := p.run("sysctl", "-n", "vm.loadavg"); err == nil {
		if l1, l5, l15, ok := parseSysctlLoadavg(out); ok {
			s.LoadOK, s.Load1, s.Load5, s.Load15 = true, l1, l5, l15
		}
	}
	if out, err := p.run("ps", "-A", "-o", "pid="); err == nil {
		if n, ok := countNonEmptyLines(out); ok {
			s.ProcOK, s.ProcCount = true, n
		}
	}
	if free, diskTotal, ok := probeStatfs(p, p.home); ok {
		s.DiskOK, s.DiskFree, s.DiskTotal = true, free, diskTotal
	}
	return s
}

// parseCPTime reads the whitespace-separated tick counts of kern.cp_time. The
// last field is idle, so busy is every earlier field together.
func parseCPTime(data []byte) (total, idle uint64, ok bool) {
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, 0, false
	}
	values := make([]uint64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		values = append(values, v)
	}
	idle = values[len(values)-1]
	for _, v := range values[:len(values)-1] {
		total += v
	}
	return total + idle, idle, true
}

// darwinMemoryTotal reads hw.memsize, the physical memory in bytes.
func darwinMemoryTotal(p metricsProbe) (uint64, bool) {
	out, err := p.run("sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return v, true
}

// darwinMemoryUsed approximates the used share from vm_stat: active plus wired
// plus compressed pages, times the page size the first line reports. It is an
// approximation and is documented as one -- macOS has no MemAvailable
// equivalent, and a more precise figure would need cgo's host_statistics, which
// this installer does not pay for a decoration.
func darwinMemoryUsed(p metricsProbe) (uint64, bool) {
	out, err := p.run("vm_stat")
	if err != nil {
		return 0, false
	}
	return parseVMStatUsed(out)
}

// parseVMStatUsed reads vm_stat's page size and the active, wired and
// compressor page counts. A missing page size or any of the three counters
// leaves the whole reading absent rather than summing part of it.
func parseVMStatUsed(data []byte) (uint64, bool) {
	var pageSize uint64
	var active, wired, compressed uint64
	var havePage, haveActive, haveWired, haveCompressed bool

	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "of" && i+1 < len(fields) {
					if v, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
						pageSize, havePage = v, true
					}
				}
			}
			continue
		}
		key, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value, ok := firstUint(rest)
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Pages active":
			active, haveActive = value, true
		case "Pages wired down":
			wired, haveWired = value, true
		case "Pages occupied by compressor":
			compressed, haveCompressed = value, true
		}
	}
	if !havePage || !haveActive || !haveWired || !haveCompressed {
		return 0, false
	}
	pages := active + wired + compressed
	if pages > (^uint64(0))/pageSize {
		return 0, false
	}
	return pages * pageSize, true
}

// parseSysctlLoadavg reads sysctl's vm.loadavg format, "{ 1.23 4.56 7.89 }".
func parseSysctlLoadavg(data []byte) (l1, l5, l15 float64, ok bool) {
	cleaned := strings.NewReplacer("{", " ", "}", " ").Replace(string(data))
	fields := strings.Fields(cleaned)
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	values := make([]float64, 3)
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, 0, 0, false
		}
		values[i] = v
	}
	return values[0], values[1], values[2], true
}

// firstUint returns the first unsigned integer in a vm_stat value like
// " 12345." -- the trailing period is part of the format.
func firstUint(s string) (uint64, bool) {
	for _, f := range strings.Fields(s) {
		f = strings.TrimSuffix(f, ".")
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// countNonEmptyLines counts the lines of a command's output that hold anything,
// which is how the macOS process list is counted.
func countNonEmptyLines(data []byte) (int, bool) {
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return n, true
}

// ============================================================================
// SHARED
// ============================================================================

// runMetricsCommand runs a short read-only command with a deadline. The timeout
// matters: ps and sysctl are cheap but not free, and a wedged one must not hold
// the sampling tick past its own interval.
func runMetricsCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// probeStatfs asks the filesystem behind path for its free and total bytes. It
// reports false when the path is empty, when there is no probe, or when the
// platform call fails, so a host with no readable HOME simply loses the row.
func probeStatfs(p metricsProbe, path string) (free, total uint64, ok bool) {
	if p.statfs == nil || path == "" {
		return 0, 0, false
	}
	return p.statfs(path)
}

// statfsFree is the real disk reading. It uses syscall.Statfs directly because
// the installer is built for darwin and linux only (the release matrix has no
// Windows target) and this is the one call both platforms spell the same way.
func statfsFree(path string) (free, total uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	blockSize := uint64(st.Bsize)
	if blockSize == 0 {
		return 0, 0, false
	}
	return uint64(st.Bavail) * blockSize, uint64(st.Blocks) * blockSize, true
}
