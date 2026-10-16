package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE ADVISOR'S OWN SCREEN
// ============================================================================
//
// One screen, and it is the report: the environment this instance is, the storage the files the user
// works on actually live on, and every finding with its evidence. The list on the right is the
// findings; the body shows what was read for the one under the cursor, so a row is never a claim
// without its measurement. The rows that act -- the benchmark and the assisted copy -- are one level
// in, and they are offered only where they can really run.
//
// The screen is read-only by construction: it draws from wslAdvisorState, which holds readings, and
// nothing in this file writes a file, runs a command or changes a mount. The two rows that do act
// hand off to their own screens, and the only one of those that writes anything is the copy, behind
// its own preview and confirmation.

// utilitiesWSLAdvisorRow is the utilities section's row for this utility. It is a constant here
// rather than in view.go beside the other rows because the section, the panel and this screen are
// the three places that must agree on one label, and only one of them owns it.
const utilitiesWSLAdvisorRow = "Diagnose this WSL environment"

// hintReread is the footer's hint for reading the machine again, which is how a stale report is
// refreshed after the user changes something in /etc/wsl.conf or frees space.
var hintReread = installerHint{"[r]", "read it again"}

// wslAdvisorState is what the screen draws. The readings are held rather than re-taken: a render
// path must not probe the machine, and the report has to be the same one the reader is reading while
// they scroll it.
type wslAdvisorState struct {
	// Resolved reports whether a reading has finished. Until it has, the screen says it is reading
	// rather than claiming there is nothing to report.
	Resolved bool
	// Facts is the reading the findings were built from, kept so a row can show its own evidence.
	Facts wslAdvisorFacts
	// Findings are the findings, already ordered by severity.
	Findings []wslFinding
	// Notice is the one-line result of the last action taken from here.
	Notice string
	// Benchmark and Migration are the two screens one level in. They live in this state rather than
	// on the model because they belong to this utility: they are cleared with it, and a reader of the
	// model can see that the report and its assistants are one thing.
	Benchmark wslBenchmarkState
	Migration wslMigrationState
}

// wslAdvisorOffered is the single place the module's visibility condition is answered: this utility
// is offered if and only if the process runs inside a real WSL instance, decided from the evidence
// system.DetectWSLRuntime read. A Windows host that merely has WSL installed is not enough, and a
// host that is not WSL at all cannot reach this screen: the row is absent from the section, and the
// screen has no other way in.
func (m Model) wslAdvisorOffered() bool {
	if !m.WSLRuntime.Inside {
		return false
	}
	// The installer's own host detection has to agree. Its answer is the broader one -- it accepts a
	// kernel that names WSL -- and requiring both is the conservative direction this module promises:
	// a host the installer classified as something other than WSL never shows the row, which is also
	// what keeps a pinned host (the tests, and an operator who forced the platform) in charge of what
	// is offered.
	return m.SystemInfo != nil && m.SystemInfo.IsWSL
}

// wslAdvisorUnavailableReason is what the section and the screen say when the utility is not
// offered. It is written from the detection's own answer, so a reader who is puzzled by the missing
// row can see what was looked for.
func (m Model) wslAdvisorUnavailableReason() string {
	if m.WSLRuntime.Inside {
		return ""
	}
	reason := m.WSLRuntime.Reason
	if reason == "" {
		if m.SystemInfo != nil && !m.SystemInfo.IsWSL && m.WSLRuntime.Container == "" {
			// The host detection already said this is not WSL at all, so the detection's own evidence
			// is beside the point: the section says what it knows.
			return fmt.Sprintf("%s needs a WSL instance and this host is not one.", utilitiesWSLAdvisorRow)
		}
		reason = "the environment does not carry the evidence a WSL instance leaves"
	}
	return fmt.Sprintf("%s needs a WSL instance: %s", utilitiesWSLAdvisorRow, reason)
}

// wslAdvisorLoadedMsg carries a finished reading to the model.
type wslAdvisorLoadedMsg struct {
	state wslAdvisorState
}

// wslAdvisorCmdIfNeeded reads the machine the first time the screen is opened, and again when the
// user asks for a re-read. It returns no command when the state is already resolved, so opening the
// screen twice does not read the machine twice.
func (m *Model) wslAdvisorCmdIfNeeded() tea.Cmd {
	if !m.wslAdvisorOffered() {
		return nil
	}
	if m.WSLAdvisor.Resolved {
		return nil
	}
	// The .wslconfig is read by the WSL resources utility, through the lookup that honours the
	// environment override; this report takes that answer rather than resolving the Windows profile a
	// second time, so the two screens cannot disagree about the same file.
	cfgPath, cfg, cfgErr := "", (*WSLResources)(nil), ""
	if m.WSLState.HasFile || m.WSLState.Reason != "" {
		cfgPath = m.WSLState.Path
		if m.WSLState.HasFile {
			current := m.WSLState.Current
			cfg = &current
		} else {
			cfgErr = m.WSLState.Reason
		}
	}
	return loadWSLAdvisorCmd(m.workingDirectory(), cfgPath, cfg, cfgErr)
}

// workingDirectory is the directory the report analyses: the process's own, which is where the
// installer was started and therefore the project the user is working on.
func (m Model) workingDirectory() string {
	dir, err := os.Getwd()
	if err != nil || dir == "" {
		return m.RepoDir
	}
	return dir
}

// loadWSLAdvisorCmd reads everything the report talks about off the update loop. Every read is a
// file read or a stat: the mount table, /proc/meminfo, /etc/wsl.conf, /etc/resolv.conf, and one
// directory listing per candidate project.
func loadWSLAdvisorCmd(cwd, cfgPath string, cfg *WSLResources, cfgErr string) tea.Cmd {
	return func() tea.Msg {
		in := defaultWSLAdvisorInput(cwd, cfgPath, cfg, cfgErr)
		facts := collectWSLAdvisorFacts(in)
		state := wslAdvisorState{
			Resolved: true,
			Facts:    facts,
			Findings: wslAdvisorFindings(facts),
		}
		return wslAdvisorLoadedMsg{state: state}
	}
}

// ============================================================================
// THE ROWS
// ============================================================================

// wslAdvisorRowKind says what a row of this screen is, so the key handler dispatches on the meaning
// of the row rather than on its wording.
type wslAdvisorRowKind int

const (
	// wslRowFinding is one finding. Selecting it shows its detail; the row itself does nothing,
	// because the report has nothing to apply.
	wslRowFinding wslAdvisorRowKind = iota
	// wslRowReread reads the machine again.
	wslRowReread
	// wslRowBenchmark opens the benchmark screen.
	wslRowBenchmark
	// wslRowMigrate opens the assisted copy.
	wslRowMigrate
	// wslRowBack leaves the screen.
	wslRowBack
	// wslRowSeparator is a rule, and the cursor never rests on it.
	wslRowSeparator
)

// wslAdvisorRow is one row of the report's list: its kind, the finding behind it, and the label the
// menu draws.
type wslAdvisorRow struct {
	Kind    wslAdvisorRowKind
	Finding wslFinding
	Label   string
}

// wslAdvisorRows derives the list. It is the only list: the menu labels, the panel and the key
// handler all read this one function, so a row cannot mean two things.
func (m Model) wslAdvisorRows() []wslAdvisorRow {
	rows := make([]wslAdvisorRow, 0, len(m.WSLAdvisor.Findings)+6)
	for _, finding := range m.WSLAdvisor.Findings {
		rows = append(rows, wslAdvisorRow{
			Kind:    wslRowFinding,
			Finding: finding,
			// The severity leads the row, so the order of the list is visible in the rows themselves
			// and not only in their order.
			Label: finding.Severity.String() + " · " + finding.Title,
		})
	}

	if len(rows) > 0 {
		rows = append(rows, wslAdvisorRow{Kind: wslRowSeparator, Label: m.menuSeparator()})
	}
	rows = append(rows, wslAdvisorRow{Kind: wslRowReread, Label: "Read the machine again"})

	// The two rows that act exist only where their engine can finish: a row that fails when it is
	// pressed is not offered in this installer, and the body says what it would have needed.
	if m.wslBenchmarkOffer().Offered {
		rows = append(rows, wslAdvisorRow{Kind: wslRowBenchmark, Label: wslBenchmarkRowLabel})
	}
	if m.wslMigrationOffer().Offered {
		rows = append(rows, wslAdvisorRow{Kind: wslRowMigrate, Label: wslMigrationRowLabel})
	}

	rows = append(rows, wslAdvisorRow{Kind: wslRowSeparator, Label: m.menuSeparator()})
	rows = append(rows, wslAdvisorRow{Kind: wslRowBack, Label: "← Back"})
	return rows
}

// wslAdvisorRowLabels turns the rows into the labels the menu draws.
func wslAdvisorRowLabels(rows []wslAdvisorRow) []string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.Label)
	}
	return labels
}

// wslAdvisorRowAt is the row the cursor is on, with the guard the separators need: a cursor cannot
// rest on a rule, so the answer is always a selectable row or nothing.
func (m Model) wslAdvisorRowAt(rows []wslAdvisorRow) (wslAdvisorRow, bool) {
	if m.Cursor < 0 || m.Cursor >= len(rows) {
		return wslAdvisorRow{}, false
	}
	row := rows[m.Cursor]
	if row.Kind == wslRowSeparator {
		return wslAdvisorRow{}, false
	}
	return row, true
}

// selectedFinding is the finding whose detail the body shows: the one under the cursor, or the first
// one when the cursor is not on a finding (on the re-read row, say), because a report that opened on
// a blank body would hide its own most important line.
func (m Model) selectedFinding() (wslFinding, bool) {
	rows := m.wslAdvisorRows()
	if row, ok := m.wslAdvisorRowAt(rows); ok && row.Kind == wslRowFinding {
		return row.Finding, true
	}
	if len(m.WSLAdvisor.Findings) > 0 {
		return m.WSLAdvisor.Findings[0], true
	}
	return wslFinding{}, false
}

// ============================================================================
// THE BODY
// ============================================================================

// renderWSLAdvisor draws the report. It is the same shape as the other utilities screens -- a
// description, a list, a notice and the legend -- so it inherits the budget arithmetic that keeps a
// narrow terminal from pushing the footer off screen.
func (m Model) renderWSLAdvisor() string {
	return m.renderThemeScreen(m.wslAdvisorDescription(), m.WSLAdvisor.Notice,
		[]installerHint{hintUp, hintDown, hintSelect, hintReread, hintBack})
}

// wslAdvisorDescription is what the screen says: the environment first, then the finding under the
// cursor with its evidence, and last what could not be read at all. The order is the reading order:
// a reader who does not know where they are cannot judge a finding about it.
func (m Model) wslAdvisorDescription() []string {
	state := m.WSLAdvisor
	if !m.wslAdvisorOffered() {
		return []string{m.wslAdvisorUnavailableReason()}
	}
	if !state.Resolved {
		return []string{"Reading this instance's mounts, limits, project directories and resolver…"}
	}

	paragraphs := wslEnvironmentParagraphs(state.Facts)
	paragraphs = append(paragraphs,
		"Read-only: nothing on this screen or the ones behind it changes a configuration file, a "+
			"mount, a virtual disk or a network setting. Each finding says where its change belongs, "+
			"and the two rows that act are the benchmark, which measures, and the copy, which writes "+
			"only where you confirm it and never deletes the source.")

	if len(state.Facts.Limits) > 0 {
		paragraphs = append(paragraphs, "What could not be read: "+strings.Join(state.Facts.Limits, " "))
	}

	if finding, ok := m.selectedFinding(); ok {
		paragraphs = append(paragraphs, wslFindingParagraphs(finding)...)
	}
	return paragraphs
}

// wslEnvironmentParagraphs states what this instance is, from the readings and nothing else. Every
// sentence is omitted rather than filled in when its reading did not happen.
func wslEnvironmentParagraphs(facts wslAdvisorFacts) []string {
	var paragraphs []string

	version := "version not reported by the kernel"
	if facts.Runtime.Version > 0 {
		version = fmt.Sprintf("version %d", facts.Runtime.Version)
	}
	distro := facts.Runtime.Distro
	if distro == "" {
		distro = "name not in the environment"
	}
	release := wslSignalDetail(facts.Runtime, "kernel-release")
	paragraphs = append(paragraphs, fmt.Sprintf("WSL %s, %s; kernel %s.", distro, version, release))

	var instance []string
	if facts.CPUs > 0 {
		instance = append(instance, fmt.Sprintf("%d processors", facts.CPUs))
	}
	if facts.Memory.Read && facts.Memory.HaveTotal {
		line := humanBytes(facts.Memory.TotalBytes) + " of memory"
		if facts.Memory.HaveUsed {
			line += fmt.Sprintf(" with %d%% in use", wslPercent(facts.Memory.UsedBytes, facts.Memory.TotalBytes))
		}
		instance = append(instance, line)
	}
	if facts.Memory.Read && facts.Memory.HaveSwap && facts.Memory.SwapTotalBytes > 0 {
		instance = append(instance, fmt.Sprintf("swap %s of %s in use",
			humanBytes(facts.Memory.SwapTotalBytes-facts.Memory.SwapFreeBytes),
			humanBytes(facts.Memory.SwapTotalBytes)))
	} else if facts.Memory.Read {
		instance = append(instance, "no swap")
	}
	if len(instance) > 0 {
		paragraphs = append(paragraphs, "This instance has "+strings.Join(instance, ", ")+
			". WSL gives the distribution the limit it was configured with, not the Windows host's total.")
	}

	if name := facts.NativeStorageName(); name != "" {
		line := "The distribution's own storage is " + name
		if facts.RootDisk.Known {
			line += fmt.Sprintf(", holding %s free of %s",
				humanBytes(facts.RootDisk.FreeBytes), humanBytes(facts.RootDisk.TotalBytes))
		} else if facts.RootDisk.Err != "" {
			line += "; its space could not be read (" + facts.RootDisk.Err + ")"
		}
		paragraphs = append(paragraphs, line+".")
	}
	if facts.ProjectDisk.Known && facts.ProjectDisk.Path != facts.RootDisk.Path {
		paragraphs = append(paragraphs, fmt.Sprintf("The working directory's filesystem holds %s free of %s.",
			humanBytes(facts.ProjectDisk.FreeBytes), humanBytes(facts.ProjectDisk.TotalBytes)))
	}

	paragraphs = append(paragraphs, "The Windows drives are mounted under "+
		facts.Conf.Describe(facts.Conf.AutomountRoot, "/mnt")+".")
	if drives := wslHostMounts(facts.Table); len(drives) > 0 {
		paragraphs = append(paragraphs, "Windows filesystems mounted in: "+strings.Join(drives, ", ")+
			". They are recognised by filesystem type, never by a path.")
	}

	if facts.Resolv.Read {
		if len(facts.Resolv.Nameservers) > 0 {
			line := facts.Resolv.Path + " lists " + strings.Join(facts.Resolv.Nameservers, ", ")
			if facts.Resolv.LinkTarget != "" {
				line += ", reached through " + facts.Resolv.LinkTarget
			}
			paragraphs = append(paragraphs, line+".")
		} else {
			paragraphs = append(paragraphs, facts.Resolv.Path+" lists no nameserver.")
		}
	} else if facts.Resolv.Err != "" {
		paragraphs = append(paragraphs, facts.Resolv.Path+" could not be read: "+facts.Resolv.Err+".")
	}

	if facts.WindowsConfig != nil {
		paragraphs = append(paragraphs, facts.WindowsConfigPath+" sets "+
			wslResourcesSummary(*facts.WindowsConfig)+".")
	} else if facts.WindowsConfigErr != "" {
		paragraphs = append(paragraphs, "The Windows-side .wslconfig could not be read: "+
			facts.WindowsConfigErr+".")
	}

	// The shell's startup has its own measurement in the same section, and this report must not
	// duplicate it: naming where it lives is how the reader finds it.
	paragraphs = append(paragraphs, "How long this shell takes to start is a separate measurement: "+
		utilitiesShellAuditRow+" in this same section.")

	return paragraphs
}

// wslFindingParagraphs renders one finding: the heading with what it is and where it came from, then
// the evidence, the consequence, what acting is worth and what to do. A finding with nothing to do
// omits the last two rather than printing empty labels.
func wslFindingParagraphs(finding wslFinding) []string {
	paragraphs := []string{fmt.Sprintf("%s — %s (%s; %s).",
		finding.Title, finding.Severity, finding.Origin, finding.Scope)}
	paragraphs = append(paragraphs, "Evidence: "+finding.Evidence)
	paragraphs = append(paragraphs, "Why it matters: "+finding.Why)
	if finding.Benefit != "" {
		paragraphs = append(paragraphs, "What acting is worth: "+finding.Benefit)
	}
	if finding.Action != "" {
		paragraphs = append(paragraphs, "What to do: "+finding.Action)
	}
	return paragraphs
}

// wslSignalDetail reads the detail the detection recorded for one signal, so the environment line
// shows the kernel string the answer came from.
func wslSignalDetail(runtime system.WSLRuntime, name string) string {
	for _, signal := range runtime.Signals {
		if signal.Name == name {
			return signal.Detail
		}
	}
	return "not read"
}

// ============================================================================
// THE KEYS
// ============================================================================

// handleWSLAdvisorKeys drives the report. Nothing here changes the machine: the cursor moves, the
// machine is read again, and the two rows that act open their own screens. Escape is handled by the
// shared key switch, which steps back to the utilities section.
func (m Model) handleWSLAdvisorKeys(key string) (tea.Model, tea.Cmd) {
	rows := m.wslAdvisorRows()

	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if rows[m.Cursor].Kind == wslRowSeparator && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(rows)-1 {
			m.Cursor++
			if rows[m.Cursor].Kind == wslRowSeparator && m.Cursor < len(rows)-1 {
				m.Cursor++
			}
		}
	case "r":
		// A re-read is how a report goes stale without the screen lying about it: the readings are
		// replaced whole, and the state says it is reading while it happens.
		m.WSLAdvisor.Resolved = false
		m.WSLAdvisor.Notice = ""
		return m, m.wslAdvisorCmdIfNeeded()
	case "enter", " ":
		row, ok := m.wslAdvisorRowAt(rows)
		if !ok {
			break
		}
		switch row.Kind {
		case wslRowReread:
			m.WSLAdvisor.Resolved = false
			m.WSLAdvisor.Notice = ""
			return m, m.wslAdvisorCmdIfNeeded()
		case wslRowBenchmark:
			return m.openWSLBenchmark()
		case wslRowMigrate:
			return m.openWSLMigration()
		case wslRowBack:
			m.Screen = ScreenUtilities
			m.Cursor = 0
		}
	}

	return m, nil
}

// wslAdvisorSummary is the one line the utilities panel shows for this utility: how many findings of
// each severity the reading produced, or that it has not read the machine yet.
func wslAdvisorSummary(state wslAdvisorState) string {
	// A copy in flight is the one thing here that keeps running after the reader leaves, so it leads
	// the line: the section says what this machine is busy with before it says what it found.
	if copyState := wslMigrationSummary(state.Migration); copyState != "" {
		return copyState
	}
	if !state.Resolved {
		return "not read yet"
	}
	counts := map[wslSeverity]int{}
	for _, finding := range state.Findings {
		counts[finding.Severity]++
	}
	var parts []string
	for _, severity := range []wslSeverity{wslSeverityCritical, wslSeverityWarning, wslSeverityNotice} {
		if counts[severity] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d %s", counts[severity], strings.ToLower(severity.String())))
	}
	if len(parts) == 0 {
		return "nothing to fix"
	}
	return strings.Join(parts, ", ")
}
