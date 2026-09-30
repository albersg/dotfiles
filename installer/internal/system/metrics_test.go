package system

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// These tests exercise the readers through the injected probe, so every parse
// path runs against the exact text the platform produces -- and every failure
// path runs without a /proc, a sysctl, a ps or a real HOME.

func TestParseProcStatCPU(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantTotal uint64
		wantIdle  uint64
		wantOK    bool
	}{
		{
			name:      "aggregate line",
			input:     "cpu  100 20 30 400 50 5 5 0 0 0\ncpu0 1 2 3 4 5 6 7 8 9 10\n",
			wantTotal: 610,
			wantIdle:  450,
			wantOK:    true,
		},
		{
			name:      "kernel adds fields they all count as busy",
			input:     "cpu  1 2 3 4 5 6 7 8 9 10 11\n",
			wantTotal: 66,
			wantIdle:  9,
			wantOK:    true,
		},
		{
			name:   "no aggregate line",
			input:  "cpu0 1 2 3 4\n",
			wantOK: false,
		},
		{
			name:   "too few fields",
			input:  "cpu  1 2 3\n",
			wantOK: false,
		},
		{
			name:   "garbage field",
			input:  "cpu  1 2 x 4\n",
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, idle, ok := parseProcStatCPU([]byte(c.input))
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if total != c.wantTotal || idle != c.wantIdle {
				t.Errorf("(total, idle) = (%d, %d), want (%d, %d)", total, idle, c.wantTotal, c.wantIdle)
			}
		})
	}
}

func TestParseMeminfo(t *testing.T) {
	t.Run("MemAvailable is preferred over MemFree", func(t *testing.T) {
		used, total, ok := parseMeminfo([]byte(
			"MemTotal:       1000 kB\nMemFree:         100 kB\nMemAvailable:    400 kB\n"))
		if !ok {
			t.Fatal("parse failed")
		}
		if total != 1000*1024 {
			t.Errorf("total = %d, want %d", total, 1000*1024)
		}
		if used != 600*1024 {
			t.Errorf("used = %d, want %d (total - available)", used, 600*1024)
		}
	})

	t.Run("MemFree is the fallback", func(t *testing.T) {
		used, _, ok := parseMeminfo([]byte("MemTotal: 1000 kB\nMemFree: 250 kB\n"))
		if !ok || used != 750*1024 {
			t.Errorf("(used, ok) = (%d, %v), want (%d, true)", used, ok, 750*1024)
		}
	})

	t.Run("available larger than total is clamped", func(t *testing.T) {
		used, _, ok := parseMeminfo([]byte("MemTotal: 100 kB\nMemAvailable: 200 kB\n"))
		if !ok || used != 0 {
			t.Errorf("(used, ok) = (%d, %v), want (0, true)", used, ok)
		}
	})

	t.Run("no total is no reading", func(t *testing.T) {
		if _, _, ok := parseMeminfo([]byte("MemFree: 100 kB\n")); ok {
			t.Error("a meminfo without MemTotal parsed as a reading")
		}
	})

	t.Run("no capacity pair is no reading", func(t *testing.T) {
		if _, _, ok := parseMeminfo([]byte("MemTotal: 100 kB\nBuffers: 1 kB\n")); ok {
			t.Error("a meminfo without available or free parsed as a reading")
		}
	})
}

func TestParseLoadavgAndProcCount(t *testing.T) {
	line := []byte("0.52 0.61 0.55 2/431 12345\n")
	l1, l5, l15, ok := parseLoadavg(line)
	if !ok || l1 != 0.52 || l5 != 0.61 || l15 != 0.55 {
		t.Errorf("loadavg = (%v, %v, %v, %v), want (0.52, 0.61, 0.55, true)", l1, l5, l15, ok)
	}
	n, ok := parseProcCount(line)
	if !ok || n != 431 {
		t.Errorf("proc count = (%d, %v), want (431, true)", n, ok)
	}

	if _, _, _, ok := parseLoadavg([]byte("only two\n")); ok {
		t.Error("a loadavg with two fields parsed as a reading")
	}
	if _, ok := parseProcCount([]byte("1.0 2.0 3.0 2\n")); ok {
		t.Error("a process field with no slash parsed as a count")
	}
}

func TestParseCPTimeMacOS(t *testing.T) {
	// user nice sys intr idle, macOS order.
	total, idle, ok := parseCPTime([]byte("100 20 30 5 400\n"))
	if !ok || idle != 400 || total != 555 {
		t.Errorf("(total, idle, ok) = (%d, %d, %v), want (555, 400, true)", total, idle, ok)
	}
	if _, _, ok := parseCPTime([]byte("nonsense\n")); ok {
		t.Error("non-numeric cp_time parsed as a reading")
	}
}

func TestParseVMStatUsedMacOS(t *testing.T) {
	input := "Mach Virtual Memory Statistics: (page size of 4096 bytes)\n" +
		"Pages free:                           1000.\n" +
		"Pages active:                         2000.\n" +
		"Pages inactive:                       5000.\n" +
		"Pages wired down:                      300.\n" +
		"Pages occupied by compressor:           50.\n"
	used, ok := parseVMStatUsed([]byte(input))
	if !ok {
		t.Fatal("vm_stat parse failed")
	}
	want := uint64((2000 + 300 + 50) * 4096)
	if used != want {
		t.Errorf("used = %d, want %d", used, want)
	}

	// A vm_stat missing the compressor line must lose the whole reading rather
	// than sum a partial approximation and call it "used".
	partial := "Mach Virtual Memory Statistics: (page size of 4096 bytes)\n" +
		"Pages active: 2000.\nPages wired down: 300.\n"
	if _, ok := parseVMStatUsed([]byte(partial)); ok {
		t.Error("a partial vm_stat parsed as a reading")
	}
}

func TestParseSysctlLoadavgMacOS(t *testing.T) {
	l1, l5, l15, ok := parseSysctlLoadavg([]byte("{ 1.50 2.25 3.00 }\n"))
	if !ok || l1 != 1.5 || l5 != 2.25 || l15 != 3.0 {
		t.Errorf("loadavg = (%v, %v, %v, %v), want (1.5, 2.25, 3, true)", l1, l5, l15, ok)
	}
}

func TestDeriveCPUIsADelta(t *testing.T) {
	first := Sample{CPUOK: true, CPUTotal: 1000, CPUIdle: 600}

	t.Run("a first sample has no rate", func(t *testing.T) {
		m := Derive(Sample{}, first)
		if m.CPUOK {
			t.Errorf("the first sample reported CPU busy %v, want no row", m.CPUBusy)
		}
	})

	t.Run("the delta is the busy share", func(t *testing.T) {
		second := Sample{CPUOK: true, CPUTotal: 2000, CPUIdle: 1200}
		m := Derive(first, second)
		if !m.CPUOK {
			t.Fatal("the second sample reported no CPU row")
		}
		// 400 busy of 1000 total.
		if m.CPUBusy < 0.39 || m.CPUBusy > 0.41 {
			t.Errorf("CPU busy = %v, want ~0.4", m.CPUBusy)
		}
	})

	t.Run("a counter reset drops the row instead of underflowing", func(t *testing.T) {
		m := Derive(Sample{CPUOK: true, CPUTotal: 2000, CPUIdle: 1200},
			Sample{CPUOK: true, CPUTotal: 1000, CPUIdle: 600})
		if m.CPUOK {
			t.Error("a reset counter still reported a CPU row")
		}
	})

	t.Run("a stalled counter drops the row", func(t *testing.T) {
		m := Derive(first, first)
		if m.CPUOK {
			t.Error("a zero interval still reported a CPU row")
		}
	})
}

func TestDeriveCopiesTheGauges(t *testing.T) {
	cur := Sample{
		MemOK: true, MemUsed: 4, MemTotal: 8,
		LoadOK: true, Load1: 1, Load5: 2, Load15: 3,
		DiskOK: true, DiskFree: 10, DiskTotal: 20,
		ProcOK: true, ProcCount: 99,
	}
	m := Derive(Sample{}, cur)
	if !m.MemOK || m.MemUsed != 4 || m.MemTotal != 8 {
		t.Errorf("memory = (%v, %d, %d)", m.MemOK, m.MemUsed, m.MemTotal)
	}
	if !m.LoadOK || m.Load1 != 1 || m.Load5 != 2 || m.Load15 != 3 {
		t.Errorf("load = (%v, %v, %v, %v)", m.LoadOK, m.Load1, m.Load5, m.Load15)
	}
	if !m.DiskOK || m.DiskFree != 10 || m.DiskTotal != 20 {
		t.Errorf("disk = (%v, %d, %d)", m.DiskOK, m.DiskFree, m.DiskTotal)
	}
	if !m.ProcOK || m.ProcCount != 99 {
		t.Errorf("processes = (%v, %d)", m.ProcOK, m.ProcCount)
	}
}

// fakeProbe builds a probe whose every host effect is a map lookup, so a reader
// can be exercised without touching the real machine.
func fakeProbe(goos string) metricsProbe {
	return metricsProbe{
		goos: goos,
		home: "/home/testuser",
		readFile: func(path string) ([]byte, error) {
			return nil, errors.New("no such file")
		},
		run: func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("not found")
		},
		statfs: func(string) (uint64, uint64, bool) { return 0, 0, false },
	}
}

func TestReadSampleRefusesTermuxAndUnknownPlatforms(t *testing.T) {
	termux := fakeProbe("linux")
	termux.termux = true
	termux.readFile = func(string) ([]byte, error) {
		t.Fatal("Termux read a platform file")
		return nil, nil
	}
	if got := readSample(termux); got != (Sample{}) {
		t.Errorf("Termux sample = %+v, want the zero reading", got)
	}

	if got := readSample(fakeProbe("windows")); got != (Sample{}) {
		t.Errorf("unknown platform sample = %+v, want the zero reading", got)
	}
}

func TestReadLinuxSampleReadsEveryRow(t *testing.T) {
	p := fakeProbe("linux")
	files := map[string]string{
		"/proc/stat":    "cpu  100 20 30 400 50 5 5 0 0 0\n",
		"/proc/meminfo": "MemTotal: 1000 kB\nMemAvailable: 400 kB\n",
		"/proc/loadavg": "0.5 0.6 0.7 2/431 1\n",
	}
	p.readFile = func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}
	p.statfs = func(string) (uint64, uint64, bool) { return 500, 1000, true }

	s := readSample(p)
	if !s.CPUOK || !s.MemOK || !s.LoadOK || !s.ProcOK || !s.DiskOK {
		t.Fatalf("linux sample lost a row: %+v", s)
	}
	if s.ProcCount != 431 {
		t.Errorf("process count = %d, want 431", s.ProcCount)
	}
	if s.DiskFree != 500 || s.DiskTotal != 1000 {
		t.Errorf("disk = (%d, %d), want (500, 1000)", s.DiskFree, s.DiskTotal)
	}
}

func TestReadLinuxSampleDegradesRowByRow(t *testing.T) {
	// Only /proc/stat is readable: the CPU row survives, the rest is absent.
	p := fakeProbe("linux")
	p.readFile = func(path string) ([]byte, error) {
		if path == "/proc/stat" {
			return []byte("cpu  1 2 3 4\n"), nil
		}
		return nil, os.ErrNotExist
	}
	s := readSample(p)
	if !s.CPUOK {
		t.Error("the readable CPU row was lost")
	}
	if s.MemOK || s.LoadOK || s.ProcOK || s.DiskOK {
		t.Errorf("unreadable rows were reported anyway: %+v", s)
	}
}

func TestReadDarwinSampleReadsEveryRow(t *testing.T) {
	p := fakeProbe("darwin")
	cmd := func(name string, args ...string) ([]byte, error) {
		key := strings.TrimSpace(name + " " + strings.Join(args, " "))
		switch key {
		case "sysctl -n kern.cp_time":
			return []byte("100 20 30 5 400\n"), nil
		case "sysctl -n hw.memsize":
			return []byte("8589934592\n"), nil
		case "sysctl -n vm.loadavg":
			return []byte("{ 1.5 2.0 3.0 }\n"), nil
		case "vm_stat":
			return []byte("Mach Virtual Memory Statistics: (page size of 4096 bytes)\n" +
				"Pages active: 100.\nPages wired down: 20.\nPages occupied by compressor: 5.\n"), nil
		case "ps -A -o pid=":
			return []byte("  1\n  2\n  3\n"), nil
		default:
			return nil, errors.New("unexpected command " + key)
		}
	}
	p.run = cmd
	p.statfs = func(string) (uint64, uint64, bool) { return 1, 2, true }

	s := readSample(p)
	if !s.CPUOK || !s.MemOK || !s.LoadOK || !s.ProcOK || !s.DiskOK {
		t.Fatalf("darwin sample lost a row: %+v", s)
	}
	if s.ProcCount != 3 {
		t.Errorf("process count = %d, want 3", s.ProcCount)
	}
	if s.MemTotal != 8589934592 {
		t.Errorf("memory total = %d, want 8589934592", s.MemTotal)
	}
}

func TestStatfsFreeReadsARealDirectory(t *testing.T) {
	free, total, ok := statfsFree(t.TempDir())
	if !ok {
		t.Skip("this platform would not stat the test directory")
	}
	if total == 0 || free > total {
		t.Errorf("statfs = (free %d, total %d), want 0 <= free <= total and total > 0", free, total)
	}
}
