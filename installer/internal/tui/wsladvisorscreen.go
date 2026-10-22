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
const utilitiesWSLAdvisorRow = "Diagnose WSL"

// hintReread is the footer's hint for reading the machine again, which is how a stale report is
// refreshed after the user changes something in /etc/wsl.conf or frees space. Reading is not
// opening: the row carries the letter, under the grammar odd/tasks/key-grammar.md fixes, and Enter
// is left to the rows that open a screen or a finding's detail.
var hintReread = installerHint{"[r]", "read it again"}

// wslRereadRow is the report's re-read row. It acts where it stands, so it says so on the row by
// carrying the letter that runs it, and the footer names the same letter: the row and the key
// handler cannot disagree about which key re-reads the machine.
const wslRereadRow = "[r] Read the machine again"

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
	rows = append(rows, wslAdvisorRow{Kind: wslRowReread, Label: wslRereadRow})

	// The two rows that act exist only where their engine can finish: a row that fails when it is
	// pressed is not offered in this installer, and the body says what it would have needed. Both
	// open a screen, so both carry the arrow the utilities section's own rows carry, and Enter is
	// the one key that presses them.
	if m.wslBenchmarkOffer().Offered {
		rows = append(rows, wslAdvisorRow{Kind: wslRowBenchmark, Label: utilitiesScreenRow(wslBenchmarkRowLabel)})
	}
	if m.wslMigrationOffer().Offered {
		rows = append(rows, wslAdvisorRow{Kind: wslRowMigrate, Label: utilitiesScreenRow(wslMigrationRowLabel)})
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
	return m.renderThemeScreen(m.wslAdvisorDescription(), m.WSLAdvisor.Notice, m.wslAdvisorHints())
}

// wslAdvisorHints is the report's footer: the two move keys, the Enter that opens a screen or a
// finding's detail, the letter the re-read row carries, and the Escape that is the way back.
func (m Model) wslAdvisorHints() []installerHint {
	return []installerHint{hintUp, hintDown, hintOpen, hintReread, hintBack}
}

// wslAdvisorDescription is the report's compact body, derived from the facts the wide frame draws:
// the environment first, then the finding under the cursor with its evidence, and last what could not
// be read at all. The order is the reading order: a reader who does not know where they are cannot
// judge a finding about it.
func (m Model) wslAdvisorDescription() []string {
	return factProse(m.wslAdvisorFacts())
}

// wslAdvisorFacts is that answer as facts: the environment the report was taken in, the read-only
// promise, what no reading could establish, and the finding under the cursor as the facts it is.
func (m Model) wslAdvisorFacts() []iconFact {
	state := m.WSLAdvisor
	if !m.wslAdvisorOffered() {
		return []iconFact{{Label: "WSL performance", Value: m.wslAdvisorUnavailableReason()}}
	}
	if !state.Resolved {
		return []iconFact{{Label: "Reading", Value: "Reading this instance's mounts, limits, project directories and resolver…"}}
	}

	facts := wslEnvironmentFacts(state.Facts)
	facts = append(facts, iconFact{Label: "Read-only",
		Value: "Nothing on this screen or the ones behind it changes a configuration file, a " +
			"mount, a virtual disk or a network setting. Each finding says where its change belongs, " +
			"and the two rows that act are the benchmark, which measures, and the copy, which writes " +
			"only where you confirm it and never deletes the source."})

	if len(state.Facts.Limits) > 0 {
		facts = append(facts, iconFact{Label: "Not read", Value: factClause(strings.Join(state.Facts.Limits, " "))})
	}

	if finding, ok := m.selectedFinding(); ok {
		facts = append(facts, wslFindingFacts(finding)...)
	}
	return facts
}

// wslEnvironmentParagraphs states what this instance is as the compact body's paragraphs. It is the
// prose of wslEnvironmentFacts, so the floor and the card read one list.
func wslEnvironmentParagraphs(facts wslAdvisorFacts) []string {
	return factProse(wslEnvironmentFacts(facts))
}

// wslEnvironmentFacts states what this instance is, from the readings and nothing else. Every fact is
// omitted rather than filled in when its reading did not happen.
func wslEnvironmentFacts(facts wslAdvisorFacts) []iconFact {
	var out []iconFact

	version := "version not reported by the kernel"
	if facts.Runtime.Version > 0 {
		version = fmt.Sprintf("version %d", facts.Runtime.Version)
	}
	distro := facts.Runtime.Distro
	if distro == "" {
		distro = "name not in the environment"
	}
	release := wslSignalDetail(facts.Runtime, "kernel-release")
	out = append(out, iconFact{Label: "Runtime", Value: fmt.Sprintf("WSL %s, %s; kernel %s.", distro, version, release)})

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
		out = append(out, iconFact{Label: "Instance", Value: "This instance has " + strings.Join(instance, ", ") +
			". WSL gives the distribution the limit it was configured with, not the Windows host's total."})
	}

	if name := facts.NativeStorageName(); name != "" {
		line := "The distribution's own storage is " + name
		if facts.RootDisk.Known {
			line += fmt.Sprintf(", holding %s free of %s",
				humanBytes(facts.RootDisk.FreeBytes), humanBytes(facts.RootDisk.TotalBytes))
		} else if facts.RootDisk.Err != "" {
			line += "; its space could not be read (" + facts.RootDisk.Err + ")"
		}
		out = append(out, iconFact{Label: "Storage", Value: line + "."})
	}
	if facts.ProjectDisk.Known && facts.ProjectDisk.Path != facts.RootDisk.Path {
		out = append(out, iconFact{Label: "Project disk", Value: fmt.Sprintf("The working directory's filesystem holds %s free of %s.",
			humanBytes(facts.ProjectDisk.FreeBytes), humanBytes(facts.ProjectDisk.TotalBytes))})
	}

	out = append(out, iconFact{Label: "Drives", Value: "The Windows drives are mounted under " +
		facts.Conf.Describe(facts.Conf.AutomountRoot, "/mnt") + "."})
	if drives := wslHostMounts(facts.Table); len(drives) > 0 {
		out = append(out, iconFact{Label: "Mounts", Value: "Windows filesystems mounted in: " + strings.Join(drives, ", ") +
			". They are recognised by filesystem type, never by a path."})
	}

	if facts.Resolv.Read {
		if len(facts.Resolv.Nameservers) > 0 {
			line := facts.Resolv.Path + " lists " + strings.Join(facts.Resolv.Nameservers, ", ")
			if facts.Resolv.LinkTarget != "" {
				line += ", reached through " + facts.Resolv.LinkTarget
			}
			out = append(out, iconFact{Label: "Resolver", Value: line + "."})
		} else {
			out = append(out, iconFact{Label: "Resolver", Value: facts.Resolv.Path + " lists no nameserver."})
		}
	} else if facts.Resolv.Err != "" {
		out = append(out, iconFact{Label: "Resolver", Value: facts.Resolv.Path + " could not be read: " + facts.Resolv.Err + "."})
	}

	if facts.WindowsConfig != nil {
		out = append(out, iconFact{Label: "Windows config", Value: facts.WindowsConfigPath + " sets " +
			wslResourcesSummary(*facts.WindowsConfig) + "."})
	} else if facts.WindowsConfigErr != "" {
		out = append(out, iconFact{Label: "Windows config", Value: "The Windows-side .wslconfig could not be read: " +
			facts.WindowsConfigErr + "."})
	}

	// The shell's startup has its own measurement in the same section, and this report must not
	// duplicate it: naming where it lives is how the reader finds it.
	out = append(out, iconFact{Label: "Shell startup", Value: "How long this shell takes to start is a separate measurement: " +
		utilitiesShellAuditRow + " in this same section."})

	return out
}

// wslFindingParagraphs renders one finding as the compact body's paragraphs. It is the prose of
// wslFindingFacts, so the report and the card name one finding the same way.
func wslFindingParagraphs(finding wslFinding) []string {
	return factProse(wslFindingFacts(finding))
}

// wslFindingFacts renders one finding as facts: the heading with what it is and where it came from,
// then the evidence, the consequence, what acting is worth and what to do. A finding with nothing to
// do omits the last two rather than printing empty labels.
func wslFindingFacts(finding wslFinding) []iconFact {
	facts := []iconFact{{Label: "Finding", Value: fmt.Sprintf("%s — %s (%s; %s).",
		finding.Title, finding.Severity, finding.Origin, finding.Scope)}}
	facts = append(facts, iconFact{Label: "Evidence", Value: factClause(finding.Evidence)})
	facts = append(facts, iconFact{Label: "Why it matters", Value: factClause(finding.Why)})
	if finding.Benefit != "" {
		facts = append(facts, iconFact{Label: "Benefit", Value: factClause(finding.Benefit)})
	}
	if finding.Action != "" {
		facts = append(facts, iconFact{Label: "What to do", Value: factClause(finding.Action)})
	}
	return facts
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
// machine is read again on its own letter, and the two rows that act open their own screens on
// Enter. Escape is handled by the shared key switch, which steps back to the utilities section.
//
// The grammar is the document's: Enter opens a screen or a finding's detail and never runs a
// command, so the re-read carries `r` on its row and in the footer, and Space -- which this screen
// has no switch for -- is not offered at all.
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
	case "enter":
		// Enter opens and nothing else: on the re-read row it is a press with no answer on purpose,
		// which is what keeps one key from meaning two things. The re-read runs on `r`, the letter
		// the row carries.
		row, ok := m.wslAdvisorRowAt(rows)
		if !ok {
			break
		}
		switch row.Kind {
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
