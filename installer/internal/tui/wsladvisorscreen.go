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
	return m.renderThemeScreen(Model.wslAdvisorDescription, m.WSLAdvisor.Notice, m.wslAdvisorHints())
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

// wslAdvisorFacts is that answer as one list of facts, which is what the compact body's prose is
// derived from. It is a flatten of wslAdvisorAreas, so the floor and the card read one list and the
// card's dividers never reach the words the floor prints.
func (m Model) wslAdvisorFacts() []iconFact {
	return factAreaFacts(m.wslAdvisorAreas())
}

// wslAdvisorAreas is the report's detail as the areas its card divides: what this instance is, the
// storage the working directory really lives on, its resolver, the Windows-side configuration, what
// no reading could establish, and the finding under the cursor.
//
// Every row inside an area is one short fact -- an icon, the label that names it and the model's own
// value -- because the previous body was the same readings written as sentences: "This instance has
// 8 processors, 8.0 GiB of memory with 37% in use, swap 0 B of 2.0 GiB in use. WSL gives the
// distribution the limit it was configured with..." is four facts and one explanation in a
// paragraph, and a reader cannot find the number they came for in it. The explanation is not
// deleted, it is demoted: what the limit means is the Windows config area's own row (the value the
// file sets), and the derivation lives where it is derived.
func (m Model) wslAdvisorAreas() []factArea {
	state := m.WSLAdvisor
	if !m.wslAdvisorOffered() {
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "WSL performance",
			Value: m.wslAdvisorUnavailableReason()}}}}
	}
	if !state.Resolved {
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconRuntime, Label: "Reading",
			Value: "This instance's mounts, limits, project directories and resolver…"}}}}
	}

	areas := wslEnvironmentAreas(state.Facts)

	// The read-only promise is two facts, not a paragraph: what this screen writes
	// (nothing) and what the two rows that act really do. The "each finding says
	// where its change belongs" half of the old sentence is the finding area's own
	// Scope row, which says exactly that about the finding the reader is looking at.
	areas = append(areas, factArea{Name: "This report", Facts: []iconFact{
		{Icon: wslFactIconReadOnly, Label: "Read-only", Value: "nothing here changes a file, a mount, a disk or a network setting"},
		{Icon: wslFactIconWrites, Label: "Writes", Value: "only the assisted copy, and only where you confirm it; it never deletes the source"},
	}})

	if len(state.Facts.Limits) > 0 {
		// One row per reading that did not happen: each is the reading's own
		// sentence, and joining them into one value would put several unrelated
		// failures in a single row a reader has to unpick.
		limits := make([]iconFact, 0, len(state.Facts.Limits))
		for _, limit := range state.Facts.Limits {
			limits = append(limits, iconFact{Icon: wslFactIconLimit, Label: "Not read", Value: factClause(limit)})
		}
		areas = append(areas, factArea{Name: "Not read", Facts: limits})
	}

	if finding, ok := m.selectedFinding(); ok {
		areas = append(areas, factArea{Name: "Finding", Facts: wslFindingFacts(finding)})
	}
	return areas
}

// wslEnvironmentParagraphs states what this instance is as the compact body's paragraphs. It is the
// prose of wslEnvironmentFacts, so the floor and the card read one list.
func wslEnvironmentParagraphs(facts wslAdvisorFacts) []string {
	return factProse(wslEnvironmentFacts(facts))
}

// wslEnvironmentFacts is the environment as one list, which is what its prose is derived from.
func wslEnvironmentFacts(facts wslAdvisorFacts) []iconFact {
	return factAreaFacts(wslEnvironmentAreas(facts))
}

// The icons the report's rows wear. They are decoration in the gutter -- the label
// beside them is the fact -- and each is one cell, so a terminal without the font
// still draws the row.
const (
	wslFactIconRuntime  = "\uf17a" // nf-fa-windows: the runtime this report is about
	wslFactIconDistro   = "\uf109" // nf-fa-linux: the distribution's own name
	wslFactIconKernel   = "\uf085" // nf-fa-cogs: the kernel that was read
	wslFactIconCPU      = "\uf2db" // nf-fa-microchip: the processors
	wslFactIconMemory   = "\uf0a0" // nf-fa-hdd-o: memory and the disk behind a path
	wslFactIconDisk     = "\uf0a0"
	wslFactIconFolder   = "\uf07b" // nf-fa-folder: the working directory's filesystem
	wslFactIconDrives   = "\uf1c0" // nf-fa-database: the Windows drives
	wslFactIconMounts   = "\uf0c1" // nf-fa-link: a mounted filesystem
	wslFactIconResolver = "\uf0ac" // nf-fa-globe: name resolution
	wslFactIconConfig   = "\uf013" // nf-fa-cog: the Windows-side configuration
	wslFactIconShell    = "\uf120" // nf-fa-terminal: the shell's startup
	wslFactIconReadOnly = "\uf023" // nf-fa-lock: the read-only promise
	wslFactIconWrites   = "\uf044" // nf-fa-pencil: what the screen may write
	wslFactIconLimit    = "\uf071" // nf-fa-warning: a reading that did not happen
	wslFactIconFinding  = "\uf0eb" // nf-fa-lightbulb: the finding under the cursor
	wslFactIconEvidence = "\uf0c3" // nf-fa-flask: what was read
	wslFactIconWhy      = "\uf128" // nf-fa-question: what it means
	wslFactIconBenefit  = "\uf0d6" // nf-fa-money: what acting is worth
	wslFactIconAction   = "\uf0ad" // nf-fa-wrench: what to do
)

// wslEnvironmentAreas states what this instance is, from the readings and nothing else: the runtime,
// the instance's capacity, the storage the working directory lives on, the resolver and the
// Windows-side configuration. Every fact is omitted rather than filled in when its reading did not
// happen, and the readings that did not happen are the "Not read" area of the report.
func wslEnvironmentAreas(facts wslAdvisorFacts) []factArea {
	var areas []factArea

	runtime := []iconFact{
		{Icon: wslFactIconRuntime, Label: "Runtime", Value: wslRuntimeValue(facts.Runtime)},
		{Icon: wslFactIconDistro, Label: "Distro", Value: wslDistroValue(facts.Runtime)},
		{Icon: wslFactIconKernel, Label: "Kernel", Value: wslSignalDetail(facts.Runtime, "kernel-release")},
	}

	// The capacity is the limit WSL gives this distribution, not the Windows host's total: the number
	// is what the instance's own file reported, and the separation of the two is the report's point.
	capacity := make([]iconFact, 0, 3)
	if facts.CPUs > 0 {
		capacity = append(capacity, iconFact{Icon: wslFactIconCPU, Label: "Processors",
			Value: fmt.Sprintf("%d", facts.CPUs)})
	}
	if facts.Memory.Read && facts.Memory.HaveTotal {
		value := humanBytes(facts.Memory.TotalBytes) + " total"
		if facts.Memory.HaveUsed {
			value += fmt.Sprintf(", %d%% in use", wslPercent(facts.Memory.UsedBytes, facts.Memory.TotalBytes))
		}
		capacity = append(capacity, iconFact{Icon: wslFactIconMemory, Label: "Memory", Value: value})
	}
	if facts.Memory.Read {
		value := "none configured"
		if facts.Memory.HaveSwap && facts.Memory.SwapTotalBytes > 0 {
			value = fmt.Sprintf("%s total, %s in use",
				humanBytes(facts.Memory.SwapTotalBytes),
				humanBytes(facts.Memory.SwapTotalBytes-facts.Memory.SwapFreeBytes))
		}
		capacity = append(capacity, iconFact{Icon: wslFactIconMemory, Label: "Swap", Value: value})
	}
	if len(runtime)+len(capacity) > 0 {
		areas = append(areas, factArea{Name: "Instance", Facts: append(runtime, capacity...)})
	}

	var storage []iconFact
	if name := facts.NativeStorageName(); name != "" {
		value := name
		switch {
		case facts.RootDisk.Known:
			value += fmt.Sprintf(", %s free of %s",
				humanBytes(facts.RootDisk.FreeBytes), humanBytes(facts.RootDisk.TotalBytes))
		case facts.RootDisk.Err != "":
			value += ", space not read (" + facts.RootDisk.Err + ")"
		}
		storage = append(storage, iconFact{Icon: wslFactIconDisk, Label: "Distribution", Value: value})
	}
	if facts.ProjectDisk.Known && facts.ProjectDisk.Path != facts.RootDisk.Path {
		storage = append(storage, iconFact{Icon: wslFactIconFolder, Label: "Project disk",
			Value: fmt.Sprintf("%s free of %s",
				humanBytes(facts.ProjectDisk.FreeBytes), humanBytes(facts.ProjectDisk.TotalBytes))})
	}
	storage = append(storage, iconFact{Icon: wslFactIconDrives, Label: "Drives",
		Value: facts.Conf.Describe(facts.Conf.AutomountRoot, "/mnt")})
	if drives := wslHostMounts(facts.Table); len(drives) > 0 {
		storage = append(storage, iconFact{Icon: wslFactIconMounts, Label: "Mounts",
			Value: strings.Join(drives, ", ")})
	}
	if len(storage) > 0 {
		areas = append(areas, factArea{Name: "Storage", Facts: storage})
	}

	if resolv := wslResolverFacts(facts); len(resolv) > 0 {
		areas = append(areas, factArea{Name: "Resolver", Facts: resolv})
	}

	if windows := wslWindowsConfigFacts(facts); len(windows) > 0 {
		areas = append(areas, factArea{Name: "Windows", Facts: windows})
	}

	// The shell's startup has its own measurement in the same section, and this report must not
	// duplicate it: naming where it lives is how the reader finds it.
	areas = append(areas, factArea{Name: "Elsewhere", Facts: []iconFact{
		{Icon: wslFactIconShell, Label: "Shell startup", Value: "measured by " + utilitiesShellAuditRow + " in this same section"},
	}})

	return areas
}

// wslRuntimeValue is the version every row and sentence names the runtime by.
func wslRuntimeValue(runtime system.WSLRuntime) string {
	if runtime.Version > 0 {
		return fmt.Sprintf("WSL %d", runtime.Version)
	}
	return "WSL, version not reported by the kernel"
}

// wslDistroValue is the distribution's own name, or that the environment did not carry it.
func wslDistroValue(runtime system.WSLRuntime) string {
	if runtime.Distro != "" {
		return runtime.Distro
	}
	return "name not in the environment"
}

// wslResolverFacts is the resolver as rows: the file, the nameservers it lists and where the path
// points when it is a link. A file that lists no nameserver, or could not be read, is one row saying
// so rather than a value a reader has to interpret.
func wslResolverFacts(facts wslAdvisorFacts) []iconFact {
	resolv := facts.Resolv
	switch {
	case resolv.Read && len(resolv.Nameservers) > 0:
		out := []iconFact{{Icon: wslFactIconResolver, Label: "Nameservers",
			Value: strings.Join(resolv.Nameservers, ", ")}}
		if resolv.LinkTarget != "" {
			out = append(out, iconFact{Icon: wslFactIconMounts, Label: "Via", Value: resolv.LinkTarget})
		}
		return out
	case resolv.Read:
		return []iconFact{{Icon: wslFactIconResolver, Label: "Nameservers", Value: resolv.Path + " lists none"}}
	case resolv.Err != "":
		return []iconFact{{Icon: wslFactIconResolver, Label: "Resolver", Value: resolv.Path + ": " + resolv.Err}}
	}
	return nil
}

// wslWindowsConfigFacts is the Windows-side .wslconfig as rows: the file, then the three keys it can
// set. A key the file does not carry is reported as not set rather than left out, because an absent
// key is what makes WSL apply its own proportional default -- which is the fact the old sentence
// spent four lines explaining.
func wslWindowsConfigFacts(facts wslAdvisorFacts) []iconFact {
	if facts.WindowsConfig == nil {
		if facts.WindowsConfigErr != "" {
			return []iconFact{{Icon: wslFactIconConfig, Label: "Config",
				Value: facts.WindowsConfigPath + ": " + facts.WindowsConfigErr}}
		}
		return nil
	}
	cfg := *facts.WindowsConfig
	limit := func(mb int, unit string) string {
		if mb <= 0 {
			return "not set"
		}
		return wslUnitLabel(mb, unit)
	}
	return []iconFact{
		{Icon: wslFactIconConfig, Label: "Config", Value: facts.WindowsConfigPath},
		{Icon: wslFactIconMemory, Label: "Memory limit", Value: limit(cfg.MemoryMB, "MB")},
		{Icon: wslFactIconCPU, Label: "Processors", Value: limit(cfg.Processors, "")},
		{Icon: wslFactIconMemory, Label: "Swap", Value: limit(cfg.SwapMB, "MB")},
	}
}

// wslFindingParagraphs renders one finding as the compact body's paragraphs. It is the prose of
// wslFindingFacts, so the report and the card name one finding the same way.
func wslFindingParagraphs(finding wslFinding) []string {
	return factProse(wslFindingFacts(finding))
}

// wslFindingFacts renders one finding as rows: what it is, the three facts that qualify it, and then
// its evidence, its consequence, what acting is worth and what to do. A finding with nothing to do
// omits the last two rather than printing empty labels.
//
// The title, the severity, the origin and the scope used to be one value -- "DNS goes through the
// Windows host — Notice (documented behaviour; changed inside this distribution, and applied only
// after WSL restarts it)." -- which is four facts in a sentence: the reader had to parse the
// parentheses to learn whether the claim was measured here and whether the change waits for a
// restart. They are four rows now, one fact each.
func wslFindingFacts(finding wslFinding) []iconFact {
	facts := []iconFact{
		{Icon: wslFactIconFinding, Label: "Finding", Value: finding.Title},
		{Icon: wslFactIconLimit, Label: "Severity", Value: finding.Severity.String()},
		{Icon: wslFactIconEvidence, Label: "Origin", Value: finding.Origin.String()},
		{Icon: wslFactIconAction, Label: "Scope", Value: finding.Scope.String()},
		{Icon: wslFactIconEvidence, Label: "Evidence", Value: factClause(finding.Evidence)},
		{Icon: wslFactIconWhy, Label: "Why it matters", Value: factClause(finding.Why)},
	}
	if finding.Benefit != "" {
		facts = append(facts, iconFact{Icon: wslFactIconBenefit, Label: "Benefit", Value: factClause(finding.Benefit)})
	}
	if finding.Action != "" {
		facts = append(facts, iconFact{Icon: wslFactIconAction, Label: "What to do", Value: factClause(finding.Action)})
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
