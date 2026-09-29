package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A panel is a screen's second column: a chip naming it, a rule under the chip,
// and then rows of labelled facts. Each panel answers the question its screen
// asks -- the welcome screen asks "where am I", so its panel is the machine the
// run is about to change; the main menu asks "what is about to happen", so its
// panel is the plan and the state that run will read.
//
// Three rules hold for every panel, and they are why a panel is not just a block
// of text:
//
//   - A row is a label in the dim tone inside a fixed measure, then a value in
//     ink. The measure is fixed so the values line up down the column and the
//     panel reads as a table of facts. The tones are a convenience: the words
//     already say which half of the row is which, so the panel survives a
//     terminal with no colour at all.
//   - Nothing is invented. Every value comes from a field system.Detect already
//     filled or from the model's own state, and a value the model does not have
//     is left out rather than printed as "unknown".
//   - Nothing is clipped silently. A value wider than the column wraps under
//     itself, and a panel with more rows than the frame leaves says how many it
//     could not show.
//
// The render path stays pure: a panel reads the model and runs no probe, no
// command and no file scan of its own, so it cannot change what the installer
// does. A fact that would need a new probe is a gap to report, not a thing to
// add here.
const (
	// panelLabelMeasure is the columns a panel row spends on its label. It is a
	// constant, not the width of the longest label, so the value column is one
	// column down the whole panel.
	panelLabelMeasure = 14

	// panelLabelGap is the blank run between the label and its value, so the two
	// read as a label and a value instead of as one sentence.
	panelLabelGap = 2

	// panelBackupStamp is how a panel writes a backup's timestamp. It is the
	// layout the restore list already uses, so the same moment reads the same way
	// wherever it is shown.
	panelBackupStamp = "2006-01-02 15:04:05"
)

// panelBody assembles a right-column body: the chip that names it, the rule under
// the chip, and its rows, clipped to the rows the frame leaves. A panel with more
// rows than it has room for keeps the ones that fit and says how many it dropped,
// so a wide-but-short terminal loses rows out loud instead of pushing the footer
// off the screen.
func panelBody(title string, rows []string, width, budget int) []string {
	out := []string{chip(title), rule(width)}
	room := budget - len(out)
	if room < 1 {
		return out
	}
	if len(rows) <= room {
		return append(out, rows...)
	}

	kept := room - 1
	out = append(out, rows[:kept]...)
	return append(out, MutedStyle.Render(fmt.Sprintf("… and %d more", len(rows)-kept)))
}

// panelFact renders one labelled fact: the label dim in the fixed measure, then
// the value in ink wrapped to the columns the label leaves, with every
// continuation line indented under the value so a wrapped row still reads as one
// row. A row with no value renders nothing, so a caller can offer a field the
// model may not have and get no row at all when it is empty.
func panelFact(label, value string, width int) []string {
	measure := min(panelLabelMeasure, max(1, width-panelLabelGap-1))
	valueWidth := max(1, width-measure-panelLabelGap)
	lines := wrapText(value, valueWidth, 0)
	if len(lines) == 0 {
		return nil
	}

	head := MutedStyle.Render(padRight(truncate(label, measure), measure)) + strings.Repeat(" ", panelLabelGap)
	indent := strings.Repeat(" ", measure+panelLabelGap)

	rows := []string{head + InkStyle.Render(lines[0])}
	for _, line := range lines[1:] {
		rows = append(rows, indent+InkStyle.Render(line))
	}
	return rows
}

// panelBudget is the rows a panel may spend: exactly the rows the frame leaves
// between its rules, so a panel cannot make the composition taller than the
// screen the body already reserved.
func panelBudget(m Model, hints []installerHint) int {
	return installerBodyRows(m.Height, footerRowCount(layoutFor(m).Inner, hints))
}

// welcomePanelLabel names the welcome screen's right column. The welcome screen
// answers "where am I", so its panel is the machine the installer is about to
// change.
const welcomePanelLabel = "Your machine"

// welcomePanel is the welcome screen's right column: the machine the run is about
// to change, read from the fields system.Detect already filled and from nothing
// else. It takes no measurement of its own -- the render path is pure -- so a
// fact detection does not carry is left out and reported as a gap rather than
// filled in here with a command.
func (m Model) welcomePanel(l layout, budget int) []string {
	info := m.SystemInfo
	if info == nil {
		return panelBody(welcomePanelLabel, nil, l.Right, budget)
	}

	rows := panelFact("OS", info.OSName, l.Right)
	rows = append(rows, panelFact("Host", m.platformHost(), l.Right)...)
	rows = append(rows, panelFact("Arch", archName(info), l.Right)...)
	rows = append(rows, panelFact("Shell", info.UserShell, l.Right)...)
	rows = append(rows, panelFact("Packages", m.packageManager(), l.Right)...)
	rows = append(rows, panelFact("Xcode CLT", xcodeState(info), l.Right)...)
	rows = append(rows, panelFact("Home", info.HomeDir, l.Right)...)

	return panelBody(welcomePanelLabel, rows, l.Right, budget)
}

// platformHost names the environment the installer is running inside, when that
// is a fact of its own. WSL is a layer over another operating system, so this row
// is the only place its version appears. Termux and macOS are already named by
// OSName, so repeating them here would print the same fact twice.
func (m Model) platformHost() string {
	info := m.SystemInfo
	if info == nil || !info.IsWSL {
		return ""
	}
	if info.WSLVersion > 0 {
		return fmt.Sprintf("WSL %d", info.WSLVersion)
	}
	return "WSL"
}

// archName names the architecture the model actually knows. It reads the fact
// detection records in Arch, not IsARM: x86_64 is as useful to name as ARM is,
// and IsARM stays behind for the callers that only ask whether the host is ARM.
// A model whose Arch is empty gets no row, so a host detection could not name is
// left out instead of guessed.
func archName(info *system.SystemInfo) string {
	if info == nil {
		return ""
	}
	return info.Arch
}

// xcodeState reports whether Xcode's command line tools are present. Detect
// probes them on macOS only, where HasXcode is a real answer; on any other host
// the field is false because the question was never asked, so the row is left out
// rather than claiming the tools are missing on Linux.
func xcodeState(info *system.SystemInfo) string {
	if info == nil || info.OS != system.OSMac {
		return ""
	}
	if info.HasXcode {
		return "installed"
	}
	return "missing"
}

// packageManager names the manager the installer will install through on this
// host. It reads the same predicates the dependency step dispatches through --
// the Termux branch of planPlatformInstall and usesPacman, usesDnf and usesApt --
// so the panel cannot name a manager the run would not use. A host with none of
// them gets no row instead of a guess.
func (m Model) packageManager() string {
	info := m.SystemInfo
	if info == nil {
		return ""
	}
	switch {
	case info.IsTermux:
		return "pkg"
	case usesPacman(&m):
		return "pacman"
	case usesDnf(&m):
		return "dnf"
	case usesApt(&m):
		return "apt-get"
	case info.HasBrew:
		return "brew"
	}
	return ""
}

// mainMenuPanelLabel names the main menu's right column: the main menu asks what
// the run about to start will do, so its panel is the plan and the state that run
// will read. It is a constant so the frame guard can read the panel back out of a
// rendered screen by the name a reader sees.
const mainMenuPanelLabel = "What will happen"

// mainMenuPanel is the main menu's right column. It shows the plan the run
// would execute and the state that run will read:
//
//   - the numbered steps, the one the run starts at (or is on) marked with the
//     ▸ the menus already use for "here", and that step's description under it;
//   - how many configs the run will overwrite and which they are;
//   - the newest backup, when it was taken and how old it is.
//
// It is a glance, not a document: the steps are their names rather than eight
// bodies of prose, because the names are what a reader scans and the paragraphs
// are not. The description is kept for the step the run starts at -- or, once a
// run is in progress, for the step it is on -- because that is the immediate
// next action and its detail is the part worth reading here. The other steps'
// descriptions are not lost: they are on the installing screen, next to the step
// that is running, which is where a description is read rather than skimmed.
//
// The plan is the wizard's own plan, not a second description of one: before the
// wizard has built it, the panel asks the same pure builder for the plan the
// model's state implies, and labels which host it is describing. A section the
// model has no fact for is left out rather than filled in with a claim: no
// scanned configs and no backups means no rows about them, not "none". A fact
// the panel does have is never dropped to save room: a long list wraps under its
// label, and panelBody says how many rows it could not show.
func (m Model) mainMenuPanel(l layout, budget int) []string {
	var rows []string

	steps, hostNote := m.previewPlan()
	if hostNote != "" {
		rows = append(rows, MutedStyle.Render(truncate(hostNote, l.Right)))
	}

	if len(steps) > 0 {
		rows = append(rows, panelFact("Steps", strconv.Itoa(len(steps)), l.Right)...)
		rows = append(rows, stepPanelRows(steps, currentStepIndex(steps), l.Right)...)
	}

	if len(m.ExistingConfigs) > 0 {
		rows = append(rows, panelFact("Overwrites", configCount(len(m.ExistingConfigs)), l.Right)...)
		for _, config := range m.ExistingConfigs {
			rows = append(rows, panelFact("", config, l.Right)...)
		}
	}

	if backup, ok := newestBackup(m.AvailableBackups); ok {
		rows = append(rows, panelFact("Newest backup", backupSummary(backup), l.Right)...)
	}

	return panelBody(mainMenuPanelLabel, rows, l.Right, budget)
}

// stepPanelRows renders the steps as a numbered list: one row per step holding
// its name, with the step the run starts at (or is on) marked by the same ▸ the
// menus use for "here" and carrying its description under it. The marker has a
// column of its own and the number is right-aligned in the column beside it, so
// the names start on one column and the digits form a straight edge whether the
// plan has nine steps or ninety, and marking a step shifts neither. A name or
// description wider than the column wraps under itself rather than being cut.
func stepPanelRows(steps []InstallStep, start, width int) []string {
	const markerWidth, gap = 2, 2
	numberWidth := len(strconv.Itoa(len(steps)))
	indentWidth := markerWidth + numberWidth + gap
	valueWidth := max(1, width-indentWidth)
	indent := strings.Repeat(" ", indentWidth)

	var rows []string
	for i, step := range steps {
		marker := "  "
		if i == start {
			marker = AccentKeyStyle.Render("▸ ")
		}
		head := marker + MeterStyle.Render(fmt.Sprintf("%*d", numberWidth, i+1)) + "  "

		name := wrapText(step.Name, valueWidth, 0)
		if len(name) == 0 {
			name = []string{""}
		}
		rows = append(rows, head+InkStyle.Render(name[0]))
		for _, line := range name[1:] {
			rows = append(rows, indent+InkStyle.Render(line))
		}

		// The description belongs to the step that is about to run, and to no
		// other: it is the one step the reader will act on next.
		if i == start && step.Description != "" {
			for _, line := range wrapText(step.Description, valueWidth, 0) {
				rows = append(rows, indent+MutedStyle.Render(line))
			}
		}
	}
	return rows
}

// previewPlan returns the plan the panel shows and, when it is describing a host
// the player has not chosen yet, one dim line naming that host.
//
// Once the wizard has built the plan, that plan is the answer and no line is
// needed. Before that -- the main menu is reached before the first question --
// the plan is rebuilt from the same pure builder with the OS filled in from the
// choice if it has been made, and otherwise from detection. Rebuilding it is
// what stops the panel from being empty on the one screen that asks "what will
// happen"; taking the OS from detection is what stops it describing a plan for a
// host the run will not use.
func (m Model) previewPlan() ([]InstallStep, string) {
	if len(m.Steps) > 0 {
		return m.Steps, ""
	}
	// A model with no detection has no host to describe, so it has no plan to
	// show: the panel says nothing rather than inventing an OS to build one from.
	if m.SystemInfo == nil {
		return nil, ""
	}

	opts := m.planOptions()
	detected := detectedOSChoice(m.SystemInfo)
	switch {
	case opts.OS == "":
		opts.OS = detected
		return planFor(opts), "on " + m.SystemInfo.OSName + " (detected)"
	case opts.OS != detected:
		return planFor(opts), "on " + chosenOSName(opts.OS)
	default:
		return planFor(opts), ""
	}
}

// detectedOSChoice maps what detection found to the wizard's own OS choice
// values, so a preview plan is built for the OS the run would actually use. It
// is the same three-way split the wizard offers: Termux, macOS, or the Linux
// family (whose distribution decides the package manager).
func detectedOSChoice(info *system.SystemInfo) string {
	switch {
	case info.IsTermux || info.OS == system.OSTermux:
		return "termux"
	case info.OS == system.OSMac:
		return "mac"
	default:
		return "linux"
	}
}

// chosenOSName names an OS the player chose, for the panel's one-line note when
// the choice differs from what detection found.
func chosenOSName(os string) string {
	switch os {
	case "mac":
		return "macOS"
	case "termux":
		return "Termux"
	default:
		return "Linux"
	}
}

// currentStepIndex is the step the run is on, or the first one it would run: the
// first step that is neither done nor skipped, which is the first step while
// nothing has started. It is -1 when there are no steps or every step has
// finished, so no step is marked with the ▸ the menus use for "here".
func currentStepIndex(steps []InstallStep) int {
	for i, step := range steps {
		if step.Status != StatusDone && step.Status != StatusSkipped {
			return i
		}
	}
	return -1
}

// newestBackup returns the most recent backup the model holds. The list
// system.ListBackups returns is in directory order, not date order, so the newest
// is chosen here rather than taken from an end of the slice.
func newestBackup(backups []system.BackupInfo) (system.BackupInfo, bool) {
	if len(backups) == 0 {
		return system.BackupInfo{}, false
	}
	newest := backups[0]
	for _, backup := range backups[1:] {
		if backup.Timestamp.After(newest.Timestamp) {
			newest = backup
		}
	}
	return newest, true
}

// backupSummary is one line about a backup: when it was taken, how many files it
// holds, and how old it is. The age is measured against the moment of rendering
// and is therefore not stored on the model: the model records when the backup was
// taken, and how old that is depends on now.
func backupSummary(backup system.BackupInfo) string {
	return fmt.Sprintf("%s, %s, %s",
		backup.Timestamp.Format(panelBackupStamp),
		fileCount(len(backup.Files)),
		backupAge(backup.Timestamp, time.Now()))
}

// configCount names how many configs a run will overwrite, in the singular when
// there is one, because "1 configs" reads as a count nobody looked at.
func configCount(n int) string {
	if n == 1 {
		return "1 config"
	}
	return fmt.Sprintf("%d configs", n)
}

// fileCount is configCount's counterpart for a backup's file list.
func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// backupAge names how long ago a backup was taken, in the coarsest unit that
// still says something: minutes under an hour, hours under a day, days under a
// month, then months. It takes the clock as a parameter rather than reading it,
// so the arithmetic is a pure function its test can pin to the minute.
func backupAge(then, now time.Time) string {
	age := now.Sub(then)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return pluralUnit(int(age.Minutes()), "minute")
	case age < 24*time.Hour:
		return pluralUnit(int(age.Hours()), "hour")
	case age < 30*24*time.Hour:
		return pluralUnit(int(age.Hours()/24), "day")
	default:
		return pluralUnit(int(age.Hours()/(24*30)), "month")
	}
}

// pluralUnit renders a count and its unit, so one minute does not read as "1
// minutes".
func pluralUnit(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}
