package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// --- sparklines and charts --------------------------------------------------

func TestSparklineDrawsLevelsAndKeepsTheNewest(t *testing.T) {
	if got := sparkline([]float64{0, 1}, 0, 1, 2); got != "▁█" {
		t.Errorf("sparkline(0,1) = %q, want %q", got, "▁█")
	}
	if got := sparkline([]float64{0, 0.5, 1}, 0, 1, 3); got != "▁▅█" {
		t.Errorf("sparkline three levels = %q, want %q", got, "▁▅█")
	}
	// Longer than the width: the newest values are the ones drawn.
	if got := sparkline([]float64{1, 0, 0}, 0, 1, 2); got != "▁▁" {
		t.Errorf("sparkline windowed = %q, want the newest two %q", got, "▁▁")
	}
	// Values outside the range are clamped rather than indexing past the table.
	if got := sparkline([]float64{-5, 5}, 0, 1, 2); got != "▁█" {
		t.Errorf("sparkline clamped = %q, want %q", got, "▁█")
	}
	// A flat series is the lowest glyph, not an invented shape.
	if got := sparkline([]float64{3, 3, 3}, 3, 3, 3); got != "▁▁▁" {
		t.Errorf("sparkline flat = %q, want %q", got, "▁▁▁")
	}
	if got := sparkline(nil, 0, 1, 5); got != "" {
		t.Errorf("sparkline of nothing = %q, want empty", got)
	}
	if got := sparkline([]float64{1}, 0, 1, 0); got != "" {
		t.Errorf("sparkline of zero width = %q, want empty", got)
	}
}

// brailleDots counts the set dots in a rendered braille chart, which is how the
// test reads the shape back out of the glyphs.
func brailleDots(rows []string) int {
	total := 0
	for _, row := range rows {
		for _, r := range row {
			bits := int(r - brailleBase)
			for bits != 0 {
				total += bits & 1
				bits >>= 1
			}
		}
	}
	return total
}

func TestBrailleChartIsADotMatrix(t *testing.T) {
	if got := brailleChart([]float64{1, 1}, 0, 1, 1, 1); len(got) != 1 || got[0] != string(rune(brailleBase+0xFF)) {
		t.Errorf("a single full cell = %q, want one full braille glyph", got)
	}
	if got := brailleChart([]float64{0, 0}, 0, 1, 1, 1); len(got) != 1 || got[0] != string(rune(brailleBase+0x09)) {
		t.Errorf("a single empty value = %q, want the two bottom dots", got)
	}

	rows := brailleChart([]float64{0, 0.25, 0.5, 0.75, 1}, 0, 1, 3, 2)
	if len(rows) != 2 {
		t.Fatalf("chart height = %d rows, want 2", len(rows))
	}
	for i, row := range rows {
		if width := len([]rune(row)); width != 3 {
			t.Errorf("chart row %d is %d cells wide, want 3", i, width)
		}
	}
	// A rising series is filled from the bottom, so the bottom row carries at
	// least as many dots as the top one.
	if brailleDots(rows[:1]) > brailleDots(rows[1:]) {
		t.Errorf("the top row has more dots than the bottom for a rising series:\n%s", strings.Join(rows, "\n"))
	}
	if got := brailleChart(nil, 0, 1, 3, 2); got != nil {
		t.Errorf("a chart of nothing = %v, want nil", got)
	}
	if got := brailleChart([]float64{1}, 0, 1, 0, 2); got != nil {
		t.Errorf("a zero-width chart = %v, want nil", got)
	}
}

// --- the ring ---------------------------------------------------------------

func TestMetricsRingKeepsTheNewestSamples(t *testing.T) {
	var m Model
	for i := 0; i < metricsRingSize+5; i++ {
		m = m.withMetricsRead(system.Sample{MemOK: true, MemUsed: uint64(i), MemTotal: 1000})
	}
	if len(m.Metrics) != metricsRingSize {
		t.Fatalf("ring holds %d samples, want %d", len(m.Metrics), metricsRingSize)
	}
	if last := m.Metrics[len(m.Metrics)-1].MemUsed; last != uint64(metricsRingSize+4) {
		t.Errorf("newest sample = %d, want %d", last, metricsRingSize+4)
	}
	if first := m.Metrics[0].MemUsed; first != 5 {
		t.Errorf("oldest kept sample = %d, want 5", first)
	}
}

func TestMetricsRingDoesNotWriteThroughASnapshot(t *testing.T) {
	m := Model{}
	m = m.withMetricsRead(system.Sample{MemOK: true, MemUsed: 1, MemTotal: 10})
	kept := m.Metrics

	m = m.withMetricsRead(system.Sample{MemOK: true, MemUsed: 2, MemTotal: 10})
	if len(kept) != 1 || kept[0].MemUsed != 1 {
		t.Errorf("adding a sample changed the earlier snapshot: %+v", kept)
	}
}

func TestWithMetricsReadDerivesCPUFromThePreviousSample(t *testing.T) {
	var m Model
	m = m.withMetricsRead(system.Sample{CPUOK: true, CPUTotal: 1000, CPUIdle: 600})
	if m.Metrics[0].CPUOK {
		t.Error("the first sample reported a CPU rate with nothing to measure it against")
	}
	m = m.withMetricsRead(system.Sample{CPUOK: true, CPUTotal: 2000, CPUIdle: 1200})
	if !m.Metrics[1].CPUOK {
		t.Fatal("the second sample reported no CPU row")
	}
	if m.Metrics[1].CPUBusy < 0.39 || m.Metrics[1].CPUBusy > 0.41 {
		t.Errorf("CPU busy = %v, want ~0.4", m.Metrics[1].CPUBusy)
	}
}

func TestProgressHistoryOnlyRecordsWhileARunIsInFlight(t *testing.T) {
	var m Model
	m = m.withMetricsRead(system.Sample{})
	if len(m.ProgressSamples) != 0 {
		t.Errorf("a model with no run recorded %d progress values, want 0", len(m.ProgressSamples))
	}

	m.InstallStartedAt = m.InstallStartedAt.Add(1)
	m.Steps = []InstallStep{{Status: StatusDone}, {Status: StatusRunning, Progress: 0.5}}
	m = m.withMetricsRead(system.Sample{})
	if len(m.ProgressSamples) != 1 || m.ProgressSamples[0] != 0.75 {
		t.Errorf("progress history = %v, want [0.75]", m.ProgressSamples)
	}
}

// --- the gate ---------------------------------------------------------------

func TestMetricsTickIsArmedOnlyOnTheAnimationGate(t *testing.T) {
	if cmd := (Model{Animating: false}).metricsTickCmdFor(); cmd != nil {
		t.Error("the sampling tick was armed with animation off")
	}
	if cmd := (Model{Animating: true}).metricsTickCmdFor(); cmd == nil {
		t.Error("the sampling tick was not armed with animation on")
	}

	// A gate forced off after the tick was armed stops the sampling rather than
	// letting it run on into a chart nothing may draw.
	next, cmd := (Model{Animating: false}).Update(metricsTickMsg{})
	if cmd != nil {
		t.Error("a metrics tick with the gate off still armed work")
	}
	if got := next.(Model); got.Animating {
		t.Error("a metrics tick turned the gate on")
	}
}

// --- the panel and the installing block -------------------------------------

func TestLivePanelStatesWhatItKnows(t *testing.T) {
	l := narrowPanelLayout()

	off := Model{Animating: false}
	if rows := off.livePanelFacts(l); len(rows) != 1 || !strings.Contains(panelText(rows), "off") {
		t.Errorf("the live panel with the gate off = %v, want one row saying the sampling is off", panelText(rows))
	}
	if got := off.liveHeadline(); !strings.Contains(got, "off") {
		t.Errorf("the off headline = %q, want it to say the sampling is off", got)
	}

	reading := Model{Animating: true}
	if rows := reading.livePanelFacts(l); len(rows) != 1 || !strings.Contains(panelText(rows), "reading") {
		t.Errorf("the live panel before the first sample = %v, want one row saying it is reading", panelText(rows))
	}

	live := Model{Animating: true, Metrics: []system.Metrics{{
		CPUOK: true, CPUBusy: 0.42,
		MemOK: true, MemUsed: 4 << 30, MemTotal: 8 << 30,
		LoadOK: true, Load1: 1.25, Load5: 1.5, Load15: 1.75,
		DiskOK: true, DiskFree: 200 << 30, DiskTotal: 500 << 30,
		ProcOK: true, ProcCount: 431,
	}}}
	rows := live.livePanelFacts(l)
	text := panelText(rows)
	for _, want := range []string{"CPU", "42%", "Memory", "50%", "Load", "1.25", "Disk free", "200.0 GiB", "Processes", "431"} {
		if !strings.Contains(text, want) {
			t.Errorf("the live panel is missing %q:\n%s", want, text)
		}
	}
	if headline := live.liveHeadline(); !strings.Contains(headline, "CPU 42%") {
		t.Errorf("the live headline = %q, want it to name the CPU share", headline)
	}
}

// TestLivePanelLeavesUnreadableRowsOut is the honesty half of the panel: a host
// that reports memory but not CPU gets a memory row and no CPU row, never a CPU
// row of zeros.
func TestLivePanelLeavesUnreadableRowsOut(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{Animating: true, Metrics: []system.Metrics{{
		MemOK: true, MemUsed: 1 << 30, MemTotal: 4 << 30,
	}}}
	text := panelText(m.livePanelFacts(l))
	if strings.Contains(text, "CPU") {
		t.Errorf("a host without a CPU reading still got a CPU row:\n%s", text)
	}
	if !strings.Contains(text, "Memory") {
		t.Errorf("a host with a memory reading lost its memory row:\n%s", text)
	}
}

// TestLivePanelSaysWhenTheHostReportsNothing covers the Termux case: the host is
// asked and answers with nothing, and the panel says that instead of drawing an
// empty chart, while the installing block spends no row at all on it.
func TestLivePanelSaysWhenTheHostReportsNothing(t *testing.T) {
	l := narrowPanelLayout()
	empty := Model{Animating: true, Metrics: []system.Metrics{{}, {}}}
	rows := empty.livePanelFacts(l)
	if len(rows) != 1 || !strings.Contains(panelText(rows), "no readings") {
		t.Errorf("a host that reports nothing produced %v, want one row saying so", panelText(rows))
	}
	if got := empty.liveHeadline(); !strings.Contains(got, "no readings") {
		t.Errorf("the empty-host headline = %q, want it to say there are no readings", got)
	}
	if n := empty.installingLiveRowCount(); n != 0 {
		t.Errorf("the installing live block spent %d rows on a host with no reading, want 0", n)
	}
}

func TestInstallingLiveBlockCountsItsRows(t *testing.T) {
	if n := (Model{Animating: false, Metrics: []system.Metrics{{CPUOK: true}}}).installingLiveRowCount(); n != 0 {
		t.Errorf("live rows with the gate off = %d, want 0", n)
	}
	if n := (Model{Animating: true}).installingLiveRowCount(); n != 0 {
		t.Errorf("live rows before the first sample = %d, want 0", n)
	}
	one := Model{Animating: true, Metrics: []system.Metrics{{CPUOK: true, CPUBusy: 0.5}}}
	if n := one.installingLiveRowCount(); n != 1 {
		t.Errorf("live rows with a pulse and no history = %d, want 1", n)
	}
	two := one
	two.ProgressSamples = []float64{0.1, 0.2}
	if n := two.installingLiveRowCount(); n != 2 {
		t.Errorf("live rows with a pulse and a history = %d, want 2", n)
	}
	if rows := two.installingLiveRows(76); len(rows) != 2 {
		t.Errorf("the live block drew %d rows, want the 2 it counted", len(rows))
	}

	// A tall frame buys the braille run chart: three rows instead of one.
	tall := two
	tall.Height = installingBrailleMinHeight
	if n := tall.installingLiveRowCount(); n != 4 {
		t.Errorf("live rows on a tall frame = %d, want 4 (pulse plus a 3-row chart)", n)
	}
	if rows := tall.installingLiveRows(120); len(rows) != 4 {
		t.Errorf("the tall live block drew %d rows, want 4", len(rows))
	}

	if rows := (Model{Animating: true}).installingLiveRows(76); len(rows) != 0 {
		t.Errorf("the live block drew %d rows with no sample, want none", len(rows))
	}
}

// --- the celebration --------------------------------------------------------

func TestCelebrationBurstIsDeterministicAndBounded(t *testing.T) {
	a, b := Model{Animating: true, Width: 80}, Model{Animating: true, Width: 80}
	a.startCelebration()
	b.startCelebration()
	if len(a.Particles) != celebrationParticleCount {
		t.Fatalf("the burst launched %d particles, want %d", len(a.Particles), celebrationParticleCount)
	}
	for i := range a.Particles {
		if a.Particles[i] != b.Particles[i] {
			t.Fatalf("two bursts differ at particle %d: %+v vs %+v", i, a.Particles[i], b.Particles[i])
		}
	}
	if rows := a.celebrationRows(80); len(rows) != celebrationRowCount {
		t.Fatalf("the burst drew %d rows, want %d", len(rows), celebrationRowCount)
	}

	for i := 0; i < celebrationFrames; i++ {
		a.advanceCelebration()
	}
	if a.Celebrating {
		t.Error("the burst outlived its frames")
	}
	if rows := a.celebrationRows(80); rows != nil {
		t.Error("a finished burst still drew rows")
	}
}

func TestCelebrationIsOffWithTheGateOff(t *testing.T) {
	// startCelebration is only ever called behind the gate, and celebrationRows
	// draws nothing while it is not running, so a model the gate never armed
	// renders no burst.
	m := Model{Animating: false, Width: 80}
	m.advanceCelebration()
	if m.Celebrating || m.celebrationRows(80) != nil {
		t.Error("a model with the gate off drew a celebration")
	}
}

// --- facts ------------------------------------------------------------------

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1 << 20, "1.0 MiB"},
		{8 << 30, "8.0 GiB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- the cost bound ---------------------------------------------------------

// TestAMetricsReadChangesNothingOnAScreenWithoutTheWidget is the sampling half
// of the companion cost guard: a sample may only repaint the rows a live widget
// owns, so a screen that has none -- the main menu -- is byte-for-byte the same
// after a reading lands.
func TestAMetricsReadChangesNothingOnAScreenWithoutTheWidget(t *testing.T) {
	base := NewModel()
	isolateGoldenTest(t, &base)
	base.Screen = ScreenMainMenu
	base.Width, base.Height = 96, 30
	base.Animating = true

	before := base.View()
	after := base.withMetricsRead(system.Sample{
		CPUOK: true, CPUTotal: 1000, CPUIdle: 400,
		MemOK: true, MemUsed: 1 << 30, MemTotal: 4 << 30,
		LoadOK: true, Load1: 1, Load5: 2, Load15: 3,
		DiskOK: true, DiskFree: 1, DiskTotal: 2,
		ProcOK: true, ProcCount: 7,
	}).View()
	if before != after {
		t.Errorf("a metrics read changed a screen that shows no reading:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestAMetricsReadChangesOnlyTheLiveRows pins the other half of the cost bound:
// on the installing screen, where the live block does draw, a reading may change
// only the rows that block owns. The rail, the bar, the step line and every
// other row are the same bytes before and after.
func TestAMetricsReadChangesOnlyTheLiveRows(t *testing.T) {
	base := NewModel()
	isolateGoldenTest(t, &base)
	base.Screen = ScreenInstalling
	base.Width, base.Height = 100, 30
	base.Animating = true
	base.Steps = []InstallStep{
		{Status: StatusDone, Progress: 1},
		{Status: StatusRunning, Progress: 0.5},
		{Status: StatusPending},
	}
	base.CurrentStep = 1
	base.InstallStartedAt = goldenGreetingTime
	base.Now = goldenGreetingTime.Add(30 * time.Second)
	base.Metrics = pinnedMetrics(6)
	base.MetricsPrev = system.Sample{CPUOK: true, CPUTotal: 1000, CPUIdle: 500}
	base.ProgressSamples = []float64{0.1, 0.2, 0.3}

	owned := base.installingLiveRowCount()
	if owned != 2 {
		t.Fatalf("the live block owns %d rows, want 2 for this fixture", owned)
	}

	beforeRows := strings.Split(base.View(), "\n")
	afterRows := strings.Split(base.withMetricsRead(system.Sample{
		CPUOK: true, CPUTotal: 2000, CPUIdle: 900,
		MemOK: true, MemUsed: 5 << 30, MemTotal: 16 << 30,
		LoadOK: true, Load1: 2, Load5: 2, Load15: 2,
		DiskOK: true, DiskFree: 300 << 30, DiskTotal: 500 << 30,
		ProcOK: true, ProcCount: 500,
	}).View(), "\n")

	if len(beforeRows) != len(afterRows) {
		t.Fatalf("a reading changed the row count from %d to %d", len(beforeRows), len(afterRows))
	}
	changed := 0
	for i := range beforeRows {
		if beforeRows[i] != afterRows[i] {
			changed++
		}
	}
	if changed > owned {
		t.Errorf("a reading changed %d rows, want at most the live block's %d:\nbefore:\n%s\nafter:\n%s",
			changed, owned, strings.Join(beforeRows, "\n"), strings.Join(afterRows, "\n"))
	}
	if changed == 0 {
		t.Error("a reading said a different thing but changed no row: the chart is not drawing the sample")
	}
}
