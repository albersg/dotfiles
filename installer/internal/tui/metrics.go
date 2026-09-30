package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// The machine's pulse, as the installer sees it.
//
// A sampler reads the host about once a second and the derived samples live in a
// ring on the model, newest last. Nothing here reads the clock or a file while
// rendering: the numbers a panel or a chart draws were put there by a command,
// which is what makes a snapshot able to pin a series and stops a "live" panel
// from redrawing a frozen frame as if it were alive.
//
// The sampler is gated by the same switch as the animation. With the gate off no
// sample is scheduled at all and the panel says the sampling is off rather than
// showing the last reading as if it were current; a stale chart presented as live
// is exactly the lie the gate exists to prevent.
const (
	// metricsTickInterval is how often the host is read. One second is the
	// cheapest cadence at which a CPU delta is still a live reading, and it is
	// the interval the sampler's own cost is measured against.
	metricsTickInterval = time.Second

	// metricsRingSize is how many samples the model keeps: one minute of history
	// at one sample a second, which is what the charts draw from. A ring is the
	// right shape because every chart wants a window, not a log.
	metricsRingSize = 60

	// installingBrailleMinHeight is the shortest installing frame that spends
	// three rows on a braille run chart instead of one on a block sparkline. Below
	// it the rail and the log need every row more than the chart needs density, so
	// the compact one-row form is what draws; at or above it there is room for the
	// four-times-finer chart the issue asks for.
	installingBrailleMinHeight = 32
)

// metricsTickMsg fires while animation is on and asks for one host read.
type metricsTickMsg time.Time

// metricsReadMsg carries one raw host reading back from the read command. The
// derived Metrics are computed in Update from this sample and the previous one,
// so the CPU delta is model state and the command itself stays a pure read.
type metricsReadMsg struct {
	sample system.Sample
}

// metricsTickCmd schedules the next host read.
func metricsTickCmd() tea.Cmd {
	return tea.Tick(metricsTickInterval, func(time.Time) tea.Msg {
		return metricsTickMsg{}
	})
}

// metricsTickCmdFor is the sampling decision in one place: the command to arm,
// or nil when the animation gate is off. Init and the tick handler both read it,
// so the two cannot disagree about whether the host is being read.
func (m Model) metricsTickCmdFor() tea.Cmd {
	if !m.Animating {
		return nil
	}
	return metricsTickCmd()
}

// readMetricsCmd reads the host. It is the command the tick arms, so the I/O
// happens off the render path and off the update path.
func readMetricsCmd() tea.Cmd {
	return func() tea.Msg {
		return metricsReadMsg{sample: system.ReadSample()}
	}
}

// appendMetric adds a sample to the ring, dropping the oldest when the ring is
// full. It copies into a fresh slice instead of reslicing in place: the model is
// a value and an earlier model may still hold the slice it was given, so growing
// the ring must not write through a snapshot the caller kept.
func appendMetric(ring []system.Metrics, v system.Metrics) []system.Metrics {
	out := make([]system.Metrics, 0, len(ring)+1)
	out = append(out, ring...)
	out = append(out, v)
	if len(out) > metricsRingSize {
		out = out[len(out)-metricsRingSize:]
	}
	return out
}

// appendProgress adds one progress value to the run's own history, with the same
// copy-first rule as appendMetric.
func appendProgress(ring []float64, v float64) []float64 {
	out := make([]float64, 0, len(ring)+1)
	out = append(out, ring...)
	out = append(out, v)
	if len(out) > metricsRingSize {
		out = out[len(out)-metricsRingSize:]
	}
	return out
}

// withMetricsRead folds one raw reading into the model: it derives the display
// metrics against the previous sample, appends them to the ring, remembers the
// raw sample as the next delta's base, and appends the run's progress when a run
// is in flight. It is a pure model transition -- no clock, no I/O -- so a test
// can drive it with samples it chose.
func (m Model) withMetricsRead(sample system.Sample) Model {
	m.Metrics = appendMetric(m.Metrics, system.Derive(m.MetricsPrev, sample))
	m.MetricsPrev = sample
	if !m.InstallStartedAt.IsZero() {
		m.ProgressSamples = appendProgress(m.ProgressSamples, m.installProgress())
	}
	return m
}

// metricSeries extracts one series from the ring for a chart: only the samples
// that carry the field, so a host that reports memory but not CPU draws a memory
// chart rather than a CPU chart of zeros.
func metricSeries(ring []system.Metrics, value func(system.Metrics) (float64, bool)) []float64 {
	out := make([]float64, 0, len(ring))
	for _, s := range ring {
		if v, ok := value(s); ok {
			out = append(out, v)
		}
	}
	return out
}

// livePanel is the live machine panel's registry entry: the pulse of the host
// right now, read from the model's ring and nothing else.
func livePanel() panel {
	return panel{
		ID:       panelLive,
		Title:    livePanelLabel,
		Short:    "Live",
		Headline: func(m Model) string { return m.liveHeadline() },
		Facts:    func(m Model, l layout) []string { return m.livePanelFacts(l) },
	}
}

// livePanelLabel names the live panel. It is deliberately not "Your machine"
// again: the welcome screen already has a panel for what the machine is, and this
// one is what the machine is doing.
const livePanelLabel = "This machine, now"

// liveSparkCells is how many block glyphs a value may spend in the live panel,
// derived from the column so a wide panel gets a longer chart and a narrow one
// still shows a shape. The reserved tail is the percentage that follows it.
func liveSparkCells(width int) int {
	cells := width - panelLabelMeasure - panelLabelGap - 4
	if cells < 1 {
		return 1
	}
	if cells > 24 {
		return 24
	}
	return cells
}

// metricsHaveAny reports whether a derived sample carries at least one reading.
// A host that reports nothing -- Termux behind Android's /proc restrictions, or
// any platform this build does not read -- produces a sample with every flag
// clear, which is how the panel can tell "not read yet" from "this host will
// never answer".
func metricsHaveAny(s system.Metrics) bool {
	return s.CPUOK || s.MemOK || s.LoadOK || s.DiskOK || s.ProcOK
}

// livePanelFacts is the live panel's rows: the CPU and memory shapes, then the
// load, disk and process facts. It invents nothing: with the gate off it says so,
// before the first sample it says it is reading, on a host that answers with
// nothing it says that too, and each row appears only when its own flag is set.
func (m Model) livePanelFacts(l layout) []string {
	width := l.Right
	if width < 1 {
		width = l.Inner
	}
	if !m.Animating {
		return panelFact("Live", "off (animation off)", width)
	}
	if len(m.Metrics) == 0 {
		return panelFact("Live", "reading this machine…", width)
	}
	last := m.Metrics[len(m.Metrics)-1]
	if !metricsHaveAny(last) {
		return panelFact("Live", "no readings on this host", width)
	}
	cells := liveSparkCells(width)
	var rows []string

	if last.CPUOK {
		spark := sparkline(metricSeries(m.Metrics, func(s system.Metrics) (float64, bool) {
			return s.CPUBusy, s.CPUOK
		}), 0, 1, cells)
		rows = append(rows, panelFact("CPU", fmt.Sprintf("%s %3.0f%%", spark, last.CPUBusy*100), width)...)
	}
	if last.MemOK {
		spark := sparkline(metricSeries(m.Metrics, func(s system.Metrics) (float64, bool) {
			if !s.MemOK || s.MemTotal == 0 {
				return 0, false
			}
			return float64(s.MemUsed) / float64(s.MemTotal), true
		}), 0, 1, cells)
		rows = append(rows, panelFact("Memory",
			fmt.Sprintf("%s %3.0f%% · %s/%s", spark, memPercent(last), humanBytes(last.MemUsed), humanBytes(last.MemTotal)), width)...)
	}
	if last.LoadOK {
		rows = append(rows, panelFact("Load", fmt.Sprintf("%.2f %.2f %.2f", last.Load1, last.Load5, last.Load15), width)...)
	}
	if last.DiskOK {
		rows = append(rows, panelFact("Disk free", humanBytes(last.DiskFree)+" of "+humanBytes(last.DiskTotal), width)...)
	}
	if last.ProcOK {
		rows = append(rows, panelFact("Processes", fmt.Sprintf("%d", last.ProcCount), width)...)
	}
	return rows
}

// liveHeadline is the one-line summary of the live panel the narrow terminal
// shows: the two facts that answer "how is it, now", or the state of the sampling
// when there is nothing current to say.
func (m Model) liveHeadline() string {
	if !m.Animating {
		return "live sampling is off"
	}
	if len(m.Metrics) == 0 {
		return "reading this machine…"
	}
	last := m.Metrics[len(m.Metrics)-1]
	if !metricsHaveAny(last) {
		return "no readings on this host"
	}
	var parts []string
	if last.CPUOK {
		parts = append(parts, fmt.Sprintf("CPU %.0f%%", last.CPUBusy*100))
	}
	if last.MemOK {
		parts = append(parts, fmt.Sprintf("mem %.0f%%", memPercent(last)))
	}
	if len(parts) == 0 {
		return "live"
	}
	return strings.Join(parts, panelTabSeparator)
}

// memPercent is the used share of memory as a percentage, with a zero total
// treated as "no reading" rather than as a division by zero.
func memPercent(s system.Metrics) float64 {
	if s.MemTotal == 0 {
		return 0
	}
	return float64(s.MemUsed) / float64(s.MemTotal) * 100
}

// installingLiveRowCount is how many rows the installing screen's live block
// spends. It is zero until the first sample lands, so a screen that has no
// reading to show -- and any model the gate never sampled -- spends no row on a
// placeholder. Once there is a pulse it is one row, plus a second once the run
// has a history worth charting. The row budget reads it, so the rail and the log
// cannot be handed rows the live block will also draw in.
func (m Model) installingLiveRowCount() int {
	if !m.Animating || len(m.Metrics) == 0 {
		return 0
	}
	if !metricsHaveAny(m.Metrics[len(m.Metrics)-1]) {
		return 0
	}
	return 1 + m.installingRunChartRows()
}

// installingRunChartRows is how many rows the run's progress chart spends: none
// before there is a history to draw, one for the block sparkline, and three for
// the braille chart when the frame is tall enough to give it the density.
func (m Model) installingRunChartRows() int {
	if len(m.ProgressSamples) < 2 {
		return 0
	}
	if m.Height >= installingBrailleMinHeight {
		return 3
	}
	return 1
}

// installingLiveRows is the installing screen's live block: the machine's pulse
// on one row, and, once there is a run history, the run's own progress drawn over
// time on the next. It draws exactly installingLiveRowCount rows, so the budget
// and the block agree by construction, and nothing at all before the first
// sample, so a run the gate never sampled shows no frozen chart.
func (m Model) installingLiveRows(width int) []string {
	if m.installingLiveRowCount() == 0 {
		return nil
	}

	cells := width / 8
	if cells < 4 {
		cells = 4
	}
	if cells > 20 {
		cells = 20
	}

	last := m.Metrics[len(m.Metrics)-1]
	var parts []string
	if last.CPUOK {
		spark := sparkline(metricSeries(m.Metrics, func(s system.Metrics) (float64, bool) {
			return s.CPUBusy, s.CPUOK
		}), 0, 1, cells)
		parts = append(parts, liveFact("cpu", fmt.Sprintf("%s %.0f%%", spark, last.CPUBusy*100)))
	}
	if last.MemOK {
		spark := sparkline(metricSeries(m.Metrics, func(s system.Metrics) (float64, bool) {
			if !s.MemOK || s.MemTotal == 0 {
				return 0, false
			}
			return float64(s.MemUsed) / float64(s.MemTotal), true
		}), 0, 1, cells)
		parts = append(parts, liveFact("mem", fmt.Sprintf("%s %.0f%%", spark, memPercent(last))))
	}
	if last.LoadOK {
		parts = append(parts, liveFact("load", fmt.Sprintf("%.2f", last.Load1)))
	}
	if last.DiskOK {
		parts = append(parts, liveFact("disk", humanBytes(last.DiskFree)+" free"))
	}
	rows := []string{truncate(strings.Join(parts, "  "), width)}

	// The run's own progress over time. A tall frame gets the braille chart, which
	// resolves four times the levels in the same columns; a short one keeps the
	// single block row. Either way the chart is the run's, from its own history.
	switch m.installingRunChartRows() {
	case 0:
		// no history yet: the pulse row is all there is to draw
	case 1:
		spark := sparkline(m.ProgressSamples, 0, 1, cells*2)
		rows = append(rows, liveFact("run", spark))
	default:
		chart := brailleChart(m.ProgressSamples, 0, 1, max(1, width-4), 3)
		indent := strings.Repeat(" ", 4)
		rows = append(rows, liveFact("run", chart[0]))
		for _, line := range chart[1:] {
			rows = append(rows, indent+InkStyle.Render(line))
		}
	}
	return rows
}

// liveFact renders one labelled value inside the installing live row: the label
// dim, the value in ink, so the row reads as facts and not as one string.
func liveFact(label, value string) string {
	return MutedStyle.Render(label+" ") + InkStyle.Render(value)
}

// humanBytes states a byte count the way a person reads a capacity: whole
// bytes under a kilobyte, then one decimal place at KiB and above. It is used
// for memory and disk sizes, where a rounded number is the point.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	for i, u := range units {
		value /= unit
		if value < unit || i == len(units)-1 {
			return fmt.Sprintf("%.1f %s", value, u)
		}
	}
	return fmt.Sprintf("%d B", n)
}
