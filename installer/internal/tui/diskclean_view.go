package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
)

// ============================================================================
// WHAT THE DEVELOPMENT STORES HOLD, DRAWN
// ============================================================================
//
// The rendering half of the disk cleaner. It reads the model and writes
// strings: no scan, no measurement, no removal, and no question about the
// machine that a render must not ask. Every number it prints is one the
// detection or the run already measured, with the source named beside it.

// diskCleanPanelValue is the utilities panel's Caches row: what the last scan
// measured and how much of it is marked. It never walks anything -- the panel
// is drawn on the section screen, where a scan has not run -- so before a visit
// it says the question has not been answered rather than printing a zero.
func diskCleanPanelValue(st diskCleanState) string {
	switch {
	case !st.Resolved:
		return "not resolved yet"
	case len(st.Items) == 0 && !st.Scanned:
		return "not measured yet"
	case len(st.Items) == 0:
		return "nothing found"
	}

	total := int64(0)
	for _, item := range st.Items {
		total += item.Reclaimable()
	}
	marked := st.Selection.Count(st.Items)
	if marked == 0 {
		return fmt.Sprintf("%s reclaimable", humanBytes(uint64(total)))
	}
	return fmt.Sprintf("%s reclaimable, %s marked", humanBytes(uint64(total)), diskCleanCount(marked))
}

// The disk cleaner's own hints. Each names the key its row carries, so the row and
// the footer say the same thing: a command shows its letter in brackets, a switch
// shows its state and Space, and Escape is the way back or the way to stop what is
// running. Enter is in none of them: nothing on this screen opens a screen, a
// row's detail or a value.
var (
	diskCleanToggleHint     = installerHint{"[Space]", "select or drop"}
	diskCleanScanHint       = installerHint{"[" + diskCleanScanKey + "]", "scan"}
	diskCleanPreviewHint    = installerHint{"[" + diskCleanPreviewKey + "]", "preview"}
	diskCleanCleanHint      = installerHint{"[" + diskCleanCleanKey + "]", "clean"}
	diskCleanAddRootHint    = installerHint{"[" + diskCleanAddRootKey + "]", "add the project directory"}
	diskCleanForgetRootHint = installerHint{"[" + diskCleanForgetRootKey + "]", "forget the project directory"}
	// Esc means two things on this screen, and the footer says which of them it is
	// doing: a run or a scan in flight is stopped by it, and otherwise it walks back.
	diskCleanStopRunHint  = installerHint{"[Esc]", "stop the run"}
	diskCleanStopScanHint = installerHint{"[Esc]", "stop the scan"}
)

// renderDiskCleaner draws the screen. It uses the same frame, the same rows and
// the same hints as the other utilities, so a screen that removes things looks
// like the screens that only report.
func (m Model) renderDiskCleaner() string {
	// The footer names the keys the handler accepts in the state on screen, and
	// nothing else. The moves come first because the list is walked with them, Space
	// is named only where a finding's row can hold a mark, and every other press is
	// the action list itself -- the same list the handler reads, so the footer cannot
	// advertise a key this state does not accept or hide one it does.
	var hints []installerHint
	if len(m.diskCleanRows()) > 0 {
		hints = append(hints, hintUp, hintDown)
	}
	if m.DiskClean.Phase == diskCleanList && len(m.DiskClean.Items) > 0 {
		hints = append(hints, diskCleanToggleHint)
	}
	for _, action := range m.diskCleanerActions() {
		shown := false
		for _, already := range hints {
			if already == action.Hint {
				shown = true
				break
			}
		}
		if !shown {
			hints = append(hints, action.Hint)
		}
	}
	return m.renderThemeScreen(Model.diskCleanerDescription, m.diskCleanerNotice(), hints)
}

// diskCleanerNotice is the screen's live line: what is happening right now, and
// the last answer's own first sentence.
func (m Model) diskCleanerNotice() string {
	st := m.DiskClean
	switch {
	case st.Scanning && st.Progress != "":
		return fmt.Sprintf("Scanning, and measuring what was found — %s Nothing is removed by a scan.", st.Progress)
	case st.Phase == diskCleanRunning && st.Progress != "":
		return fmt.Sprintf("Cleaning the items you approved — %s Stopping is one press, and the items it had not reached are left alone.", st.Progress)
	case st.Notice != "":
		return st.Notice
	default:
		return ""
	}
}

// diskCleanerDescription is what the screen says about what it found and what
// the rows would do. It is the prose of diskCleanerAreas, so the floor and the card
// beside the list read one list.
func (m Model) diskCleanerDescription() []string {
	return factProse(factAreaFacts(m.diskCleanerAreas()))
}

// diskCleanerAreas is the screen's detail as areas: what the scan is doing, which
// finding is under the cursor and what it holds, what was measured and what those
// numbers are worth, what is marked, what the run did, the scope it searched, and
// what it will not do.
//
// The body was one paragraph list: nine sentences of orientation, then the focused
// finding as five more paragraphs, then the measurement's caveats, the selection,
// the outcome, the scope and the safety account -- all in the same voice, so the
// finding a reader was deciding about read exactly like the boilerplate under it. It
// is rows now, each under the label that names it, and the areas a reader skips when
// they already know them.
func (m Model) diskCleanerAreas() []factArea {
	st := m.DiskClean

	if !st.Resolved {
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "Resolving",
			Value: "the development stores; nothing here has been measured"}}}}
	}

	var areas []factArea
	if st.Reason != "" {
		areas = append(areas, factArea{Name: "Why not everything", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Limit", Value: factClause(st.Reason)},
		}})
	}

	switch {
	case !st.Scanned && !st.Scanning:
		areas = append(areas, factArea{Name: "The scan", Facts: []iconFact{
			{Icon: wslFactIconReadOnly, Label: "Writes", Value: "nothing: a scan measures and removes nothing"},
			{Icon: wslFactIconLimit, Label: "Bounded", Value: "a store it cannot reach is reported rather than guessed at"},
			{Icon: wslFactIconBenefit, Label: "Stops", Value: "Escape"},
		}})
	case st.Scanning:
		areas = append(areas, factArea{Name: "The scan", Facts: []iconFact{
			{Icon: wslFactIconBenefit, Label: "Measuring", Value: "the stores now"},
			{Icon: wslFactIconReadOnly, Label: "Writes", Value: "nothing: a scan removes nothing"},
			{Icon: wslFactIconBenefit, Label: "Stops", Value: "Escape"},
		}})
	}

	// The preview's head is what the confirmation below would mean, so it comes
	// before the item it is about.
	if st.Phase == diskCleanPreview {
		plan := st.Plan
		rows := []iconFact{{Icon: wslFactIconReadOnly, Label: "Nothing run", Value: "the preview removes nothing"}}
		if len(plan.Items) == 0 {
			rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Empty", Value: "nothing is selected"})
		} else {
			runnable := len(selectedRunnable(plan.Items))
			rows = append(rows,
				iconFact{Icon: wslFactIconBenefit, Label: "Would run", Value: diskCleanCount(runnable)},
				iconFact{Icon: wslFactIconLimit, Label: "Would not", Value: diskCleanCount(len(plan.Items) - runnable)},
			)
		}
		areas = append(areas, factArea{Name: "Preview", Facts: rows})
	}

	// The finding under the cursor, always: what it is, what it holds and where that
	// number came from, why it was found, where it is or what it covers, and what
	// removing it would cost to get back.
	if item, action, ok := m.diskCleanFocusedFinding(); ok {
		areas = append(areas, diskCleanItemArea(item, action))
	}

	if st.Scanned && !st.Scanning {
		areas = append(areas, diskCleanMeasurementArea(st.Items))
	}
	if len(st.Items) > 0 {
		areas = append(areas, diskCleanSelectionArea(st))
	}
	if st.Phase == diskCleanResults {
		if result := diskCleanResultArea(st.CleanReport); len(result.Facts) > 0 {
			areas = append(areas, result)
		}
	}

	// The screen's own scope and safety notes come last on purpose: they are what a
	// first-time reader wants and what a small terminal can afford to lose, while the
	// evidence of the finding under the cursor is neither.
	areas = append(areas, diskCleanScopeArea(m, st), diskCleanSafetyArea(m))
	return areas
}

// diskCleanItemArea is one finding as rows: what it is and how it is classified,
// what it holds and where that number came from, why the scan found it, where it is,
// and what removing it would cost to get back. In the preview the exact action the
// confirmation would run is a row of its own at the top, where a small terminal
// cannot cut it away from the finding it belongs to.
func diskCleanItemArea(item diskclean.Item, action string) factArea {
	// The order is the reader's: what it is and how it is classified, what it holds,
	// why the scan found it, where it is, and what removing it would cost to get
	// back. The finer numbers -- the second size, the file count, the source of the
	// number -- come after those, because they qualify the size rather than decide
	// anything on their own, and the frame's floor cuts from the bottom.
	classification := item.Risk.Label()
	if item.Category != "" {
		classification += " · " + item.Category.Label()
	}
	rows := []iconFact{
		{Icon: wslFactIconFinding, Label: "Finding", Value: item.Title},
		{Icon: wslFactIconLimit, Label: "Risk", Value: classification},
	}
	if strings.TrimSpace(action) != "" {
		rows = append(rows, iconFact{Icon: wslFactIconWrites, Label: "Would run", Value: action})
	}
	rows = append(rows, diskCleanSizeFacts(item)...)
	if reason := strings.TrimSpace(item.Reason); reason != "" {
		rows = append(rows, iconFact{Icon: wslFactIconWhy, Label: "Found because", Value: factClause(reason)})
	}
	// The paths and the engine objects are the answer to "where is this?". In the
	// preview they are not repeated: the action row above already names the exact
	// path a removal would take, and at the frame the request treats as the floor
	// that row is what buys the reconstruction cost its place on the screen.
	if strings.TrimSpace(action) == "" {
		for _, path := range item.Paths {
			rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "On disk", Value: path})
		}
		if len(item.VolatileObjects) > 0 && item.Size.Source != diskclean.SourceTool {
			rows = append(rows, iconFact{Icon: wslFactIconDrives, Label: "Inside engine",
				Value: strings.Join(item.VolatileObjects, ", ")})
		}
	}
	if detail := strings.TrimSpace(item.Detail); detail != "" {
		rows = append(rows, iconFact{Icon: wslFactIconEvidence, Label: "Reconstruct", Value: factClause(detail)})
	}
	return factArea{Name: "Under the cursor", Facts: rows}
}

// diskCleanSizeFacts is what one finding holds, split into the facts a reader can
// check: the space on disk, the contents behind it, the number of files, where the
// number came from, and the two caveats that decide how far to trust it.
func diskCleanSizeFacts(item diskclean.Item) []iconFact {
	switch {
	case item.Size.Missing:
		return []iconFact{{Icon: wslFactIconLimit, Label: "Holds",
			Value: "nothing: the store is not there any more, or was never created"}}
	case item.Size.Err != "":
		return []iconFact{{Icon: wslFactIconLimit, Label: "Holds",
			Value: "size could not be measured: " + factClause(item.Size.Err)}}
	}

	rows := []iconFact{{Icon: wslFactIconDisk, Label: "On disk", Value: humanBytes(uint64(item.Size.Physical))}}
	if item.Size.Logical != item.Size.Physical {
		rows = append(rows, iconFact{Icon: wslFactIconDisk, Label: "Contents", Value: humanBytes(uint64(item.Size.Logical))})
	}
	// The count and the source share a row: together they say how far to trust the
	// number above, and neither is a fact a reader acts on alone.
	source := "walked"
	if item.Size.Source == diskclean.SourceTool {
		source = "tool-reported"
	}
	measure := source
	if item.Size.Files > 0 {
		measure = fmt.Sprintf("%d files, %s", item.Size.Files, source)
	}
	rows = append(rows, iconFact{Icon: wslFactIconEvidence, Label: "Measured", Value: measure})
	if len(item.VolatileObjects) > 0 && item.Size.Source == diskclean.SourceTool {
		rows = append(rows, iconFact{Icon: wslFactIconDrives, Label: "Inside engine",
			Value: strings.Join(item.VolatileObjects, ", ")})
	}
	if item.Size.Truncated {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Lower bound",
			Value: "the walk stopped at its own ceiling"})
	}
	if item.Size.Shared > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Shared",
			Value: "some files are counted under another finding too, and only once here"})
	}
	return rows
}

// diskCleanMeasurementArea is what the scan measured, and every reason a number in it
// could be read for more than it is worth.
func diskCleanMeasurementArea(items []diskclean.Item) factArea {
	if len(items) == 0 {
		return factArea{Name: "Measured", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Measured", Value: diskCleanScanNotice(items)},
		}}
	}

	logical, physical, toolReports, missing, truncated, overlaps := int64(0), int64(0), 0, 0, 0, 0
	for _, item := range items {
		logical += item.Size.Logical
		physical += item.Size.Physical
		switch {
		case item.Size.Missing:
			missing++
		case item.Size.Err != "":
			missing++
		case item.Size.Source == diskclean.SourceTool:
			toolReports++
		}
		if item.Size.Truncated {
			truncated++
		}
		if len(item.Overlaps) > 0 {
			overlaps++
		}
	}

	rows := []iconFact{
		{Icon: wslFactIconDisk, Label: "On disk", Value: humanBytes(uint64(physical))},
		{Icon: wslFactIconDisk, Label: "Contents", Value: humanBytes(uint64(logical))},
		// The two sizes are not the same number, and the difference is a fact about the
		// filesystem rather than a mistake in the arithmetic.
		{Icon: wslFactIconWhy, Label: "They differ when", Value: "files are sparse, compressed, or still shared by another name"},
	}
	if toolReports > 0 {
		verb := "report"
		if toolReports == 1 {
			verb = "reports"
		}
		rows = append(rows, iconFact{Icon: wslFactIconEvidence, Label: "Tool-reported",
			Value: diskCleanCount(toolReports) + " " + verb + " its own number rather than being walked"})
	}
	if overlapping(items) {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Shared files",
			Value: "counted once here, and a finding that shares bytes says so in its own card"})
	}
	if truncated > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Stopped",
			Value: diskCleanCount(truncated) + " hit the ceilings a scan runs under, so the size shown is a lower bound"})
	}
	if overlaps > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Overlapping",
			Value: diskCleanCount(overlaps) + " overlap another finding, so each would free less alone"})
	}
	if missing > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Not measured",
			Value: diskCleanCount(missing) + ", marked not measured rather than shown as empty"})
	}
	return factArea{Name: "Measured", Facts: rows}
}

// diskCleanSelectionArea is what the user has marked, split by what removing it would
// cost, so a delicate item in the selection is visible before the preview rather than
// inside it.
func diskCleanSelectionArea(st diskCleanState) factArea {
	count := st.Selection.Count(st.Items)
	if count == 0 {
		return factArea{Name: "Marked", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Selected", Value: "nothing"},
			{Icon: wslFactIconAction, Label: "Next", Value: "mark the findings you want, then preview them"},
		}}
	}

	safe, careful, delicate := 0, 0, 0
	for _, item := range st.Items {
		if !st.Selection.Has(item.ID) {
			continue
		}
		switch item.Risk {
		case diskclean.RiskSafe:
			safe++
		case diskclean.RiskCareful:
			careful++
		default:
			delicate++
		}
	}

	rows := []iconFact{{Icon: wslFactIconBenefit, Label: "Selected", Value: diskCleanCount(count)}}
	if safe > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconReadOnly, Label: "Safe", Value: diskCleanCount(safe) + " to rebuild"})
	}
	if careful > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Careful",
			Value: diskCleanCount(careful) + " downloaded or rebuilt again, at a cost in time and traffic"})
	}
	if delicate > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Delicate",
			Value: diskCleanCount(delicate) + " a tool's own data, which may hold something irreproducible"})
	}
	rows = append(rows, iconFact{Icon: wslFactIconAction, Label: "Preview",
		Value: "names each one's command or path before anything runs"})
	return factArea{Name: "Marked", Facts: rows}
}

// diskCleanResultArea is what the run did, item by item: what came back, what did
// not, and why. The report's own lead number is the footer's, so this names the items
// and their outcomes rather than repeating the total.
func diskCleanResultArea(report diskclean.CleanReport) factArea {
	if len(report.Results) == 0 {
		return factArea{Name: "The run", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Run", Value: "finished without reaching an item"},
		}}
	}

	rows := make([]iconFact, 0, len(report.Results)+3)
	for _, result := range report.Results {
		outcome := ""
		switch {
		case !result.Ran:
			outcome = "not run: " + factClause(result.Error)
		case result.Refused:
			outcome = "refused before anything was removed: " + factClause(result.Error)
		case result.Error != "":
			outcome = fmt.Sprintf("%s, and %s came back: %s", diskCleanFreedPhrase(result),
				diskCleanCount(len(result.Skipped)), factClause(result.Error))
		default:
			outcome = diskCleanFreedPhrase(result)
		}
		// The item's name is part of the fact, not the label: a label is a column of
		// about twenty cells and a store's name is often longer than that, so a name
		// used as a label would be the one thing on this card that gets cut.
		rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "Result", Value: result.Item.Title + ": " + outcome})
		if len(result.Skipped) > 0 {
			rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Left in place",
				Value: strings.Join(boundedList(result.Skipped, 4), ", ") + ": held open, owned by another user, or refused"})
		}
	}

	if report.FreedMeasured {
		rows = append(rows, iconFact{Icon: wslFactIconDisk, Label: "Filesystem gained",
			Value: humanBytes(uint64(report.FreedBytes)) + ", measured before the first command and after the last"})
	} else if report.FreedNote != "" {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Not one number", Value: factClause(report.FreedNote)})
	}
	if report.Cancelled {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Stopped",
			Value: "the items it had not reached were left exactly as they were"})
	}
	return factArea{Name: "The run", Facts: rows}
}

// diskCleanScopeArea says what was searched and how that scope is changed: the stores
// are known locations, and the project directories are only the ones the user added.
func diskCleanScopeArea(m Model, st diskCleanState) factArea {
	rows := []iconFact{
		{Icon: wslFactIconFolder, Label: "Looks at", Value: "the stores tools keep in their own locations, and the project directories here"},
		{Icon: wslFactIconLimit, Label: "Never", Value: "walks a whole disk, or follows a symbolic link out of a store it knows"},
	}
	if len(st.ProjectRoots) == 0 {
		rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Projects",
			Value: "none added, so project-level stores are not part of this scan"})
	} else {
		rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "Projects",
			Value: strings.Join(st.ProjectRoots, ", ") + " (node_modules, target, build, dist, __pycache__, a virtualenv)"})
	}
	if st.Cwd != "" && !diskCleanHasRoot(st.ProjectRoots, st.Cwd) {
		rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "Started from",
			Value: st.Cwd + ", which the [" + diskCleanAddRootKey + "] key adds to the scope"})
	}
	return factArea{Name: "The scope", Facts: rows}
}

// diskCleanSafetyArea is the honest account of what the utility will not do, kept on
// screen rather than in the documentation only. Each promise is one row, because the
// paragraph this used to be was read by nobody who had already decided to mark a
// store -- and the WSL row is the one that changes what the user should expect.
func diskCleanSafetyArea(m Model) factArea {
	rows := []iconFact{
		{Icon: wslFactIconReadOnly, Label: "Engines", Value: "never deleted file by file: the engine's or manager's own prune command"},
		{Icon: wslFactIconLimit, Label: "No command", Value: "the finding is reported rather than removed"},
		{Icon: wslFactIconAction, Label: "Admin", Value: "attempted without a prompt; if refused, the exact command is named"},
		{Icon: wslFactIconLimit, Label: "Removal", Value: "a removal is a removal; nothing is quarantined"},
		{Icon: wslFactIconReadOnly, Label: "Marked", Value: "never for you: dependencies, container images and volumes are yours to choose"},
		{Icon: wslFactIconEvidence, Label: "Telemetry", Value: "none: no network call and no background cleanup"},
		{Icon: wslFactIconBenefit, Label: "Runs", Value: "only while you are on this screen, and Escape stops either one"},
	}
	if m.SystemInfo != nil && m.SystemInfo.IsWSL {
		rows = append(rows,
			iconFact{Icon: wslFactIconLimit, Label: "Inside WSL",
				Value: "this frees space in the Linux filesystem, not on the Windows drive"},
			iconFact{Icon: wslFactIconLimit, Label: "Virtual disk",
				Value: "does not shrink on its own, and this utility will not compact it"},
		)
	}
	return factArea{Name: "What it will not do", Facts: rows}
}

// diskCleanMeasurementParagraph is what the scan measured, and every reason a
// number in it could be read for more than it is worth.
func diskCleanMeasurementParagraph(items []diskclean.Item) []string {
	if len(items) == 0 {
		return []string{diskCleanScanNotice(items)}
	}

	logical, physical, toolReports, missing, truncated, overlaps := int64(0), int64(0), 0, 0, 0, 0
	for _, item := range items {
		logical += item.Size.Logical
		physical += item.Size.Physical
		switch {
		case item.Size.Missing:
			missing++
		case item.Size.Err != "":
			missing++
		case item.Size.Source == diskclean.SourceTool:
			toolReports++
		}
		if item.Size.Truncated {
			truncated++
		}
		if len(item.Overlaps) > 0 {
			overlaps++
		}
	}

	paragraphs := []string{fmt.Sprintf("Measured: %s, of which %s takes space on the disk. Logical size is what "+
		"the files hold; physical size is the space the filesystem spends on them, and it is the number the "+
		"rows carry. The two differ when files are sparse, compressed, or still shared by another name.",
		humanBytes(uint64(logical)), humanBytes(uint64(physical)))}

	if toolReports > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%s report their own numbers rather than being walked: a "+
			"container engine's store is measured by the engine, because walking it would describe the files "+
			"and not what the engine could actually reclaim. Those rows say (tool-reported).",
			diskCleanCount(toolReports)))
	}
	if overlapping(items) {
		paragraphs = append(paragraphs, "Some findings share files -- one store living inside another, or several "+
			"names for the same bytes. The totals count each file once, and a row that shares bytes says so, so "+
			"adding the rows together cannot overshoot what the disk actually holds.")
	}
	if overlaps > 0 || truncated > 0 {
		paragraphs = append(paragraphs, diskCleanBoundParagraph(truncated, overlaps))
	}
	if missing > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%s could not be measured: the store is gone, or reading it "+
			"was refused. Those findings are marked not measured rather than shown as empty, because an unmeasured "+
			"store and an empty one are not the same thing.", diskCleanCount(missing)))
	}
	return paragraphs
}

// diskCleanBoundParagraph says why a number might be a floor rather than the
// whole store.
func diskCleanBoundParagraph(truncated, overlaps int) string {
	var reasons []string
	if truncated > 0 {
		reasons = append(reasons, fmt.Sprintf("%s were stopped by the ceilings a scan runs under, so their sizes "+
			"are the part that was read", diskCleanCount(truncated)))
	}
	if overlaps > 0 {
		reasons = append(reasons, fmt.Sprintf("%s overlap another finding, so what they would free alone is less "+
			"than their own size", diskCleanCount(overlaps)))
	}
	return "Read these numbers as the truth about what was looked at, not as a promise about what removal frees: " +
		strings.Join(reasons, ", and ") + "."
}

// overlapping reports whether any finding shares bytes with another one.
func overlapping(items []diskclean.Item) bool {
	for _, item := range items {
		if len(item.Overlaps) > 0 {
			return true
		}
	}
	return false
}

// diskCleanSelectionParagraph is what the user has marked, split by what
// removing it would cost, so a delicate item in the selection is visible before
// the preview rather than inside it.
func diskCleanSelectionParagraph(st diskCleanState) string {
	count := st.Selection.Count(st.Items)
	if count == 0 {
		return "Nothing is selected. Mark the findings you want, then preview them: the preview names each one's " +
			"command or path before anything runs."
	}

	safe, careful, delicate := 0, 0, 0
	for _, item := range st.Items {
		if !st.Selection.Has(item.ID) {
			continue
		}
		switch item.Risk {
		case diskclean.RiskSafe:
			safe++
		case diskclean.RiskCareful:
			careful++
		default:
			delicate++
		}
	}

	parts := []string{fmt.Sprintf("%s are marked:", diskCleanCount(count))}
	if safe > 0 {
		parts = append(parts, fmt.Sprintf("%s safe to rebuild", diskCleanCount(safe)))
	}
	if careful > 0 {
		parts = append(parts, fmt.Sprintf("%s whose contents are downloaded or rebuilt again, at a cost in time and "+
			"traffic", diskCleanCount(careful)))
	}
	if delicate > 0 {
		parts = append(parts, fmt.Sprintf("%s delicate: data a tool owns, which may hold something that is not "+
			"reproducible", diskCleanCount(delicate)))
	}
	return strings.Join(parts, " ") + " The preview names each one's command or path before anything runs."
}

// diskCleanCardFloor is the rows the card of the finding under the cursor is
// guaranteed at any terminal size. The card is what the decision about a store is
// made on -- the classification, the sizes and their source, the reason the scan
// found it, the paths or engine objects it covers, and what removing it would cost
// -- so the frame the request treats as the floor reserves these rows for it and
// windows the list instead of letting the list take every row it asks for.
//
// It is ten now, not seven, because the card states those as labelled rows instead
// of sentences: each row costs the line its label and value share, and a row of
// evidence is the one thing the floor may not cut. The rows come out of the list,
// which is windowed around the cursor and says how many findings it hid.
const diskCleanCardFloor = 10

// diskCleanPreviewHeadFloor is what the preview's own headline costs above that
// card: the three rows that say nothing has run yet, how many items would run and
// how many would not. The card is not pushed off the floor by it, and the card is
// one row taller here because the preview inserts the exact action line under its
// title.
const diskCleanPreviewHeadFloor = 3

// diskCleanFocusedFinding is the finding the cursor is on, and in the preview the
// exact thing the confirmation would do to it. It is how one screen can show
// every finding's evidence without printing a list as long as the scan.
func (m Model) diskCleanFocusedFinding() (diskclean.Item, string, bool) {
	st := m.DiskClean
	if len(st.Items) == 0 || !st.Scanned {
		return diskclean.Item{}, "", false
	}
	rows := m.diskCleanRows()
	at := m.Cursor
	if at < 0 || at >= len(rows) || rows[at].item < 0 {
		// The cursor is on a heading, which is a divider rather than a finding. The card
		// describes the first finding instead of going blank: a card that emptied itself
		// on every heading would hide the evidence behind the arrow key that reached one,
		// and the heading is not something to read about.
		at = -1
		for i, row := range rows {
			if row.item >= 0 {
				at = i
				break
			}
		}
		if at < 0 {
			return diskclean.Item{}, "", false
		}
	}
	row := rows[at]
	if row.item < 0 || row.item >= len(st.Items) {
		return diskclean.Item{}, "", false
	}
	item := st.Items[row.item]
	if st.Phase != diskCleanPreview || !st.Planned {
		return item, "", true
	}
	for _, planned := range st.Plan.Items {
		if planned.Item.ID != item.ID {
			continue
		}
		return item, diskCleanPlannedAction(planned), true
	}
	return item, "", true
}

// diskCleanPlannedAction is what the confirmation would do to one item, in the
// words the item itself carries: the tool's own command, the exact path a removal
// would take, or the reason nothing would run at all.
func diskCleanPlannedAction(planned diskclean.PlannedItem) string {
	if !planned.Runs() {
		return "Nothing would run for this one. " + planned.Blocked
	}
	switch planned.Item.Mechanism.Kind {
	case diskclean.MechanismTool:
		return "The confirmation would run `" + planned.Item.Mechanism.Command() + "`."
	case diskclean.MechanismRemove:
		return "The confirmation would remove " + strings.Join(planned.Item.Paths, ", ") + " — irreversibly."
	default:
		return "Reported only: " + planned.Item.Mechanism.Reason
	}
}

// diskCleanPreviewHead is the preview's headline: how much of the selection would
// actually run, and the promise that nothing has yet. It is one sentence on
// purpose -- the rows below name each action and the card spells the focused one
// out in full, so at the frame the request treats as the floor this sentence must
// not be what pushes the reconstruction cost off the screen.
func diskCleanPreviewHead(plan diskclean.Plan) string {
	if len(plan.Items) == 0 {
		return "The preview is empty: nothing is selected."
	}
	runnable := len(selectedRunnable(plan.Items))
	return fmt.Sprintf("Preview: nothing has run yet. %s would run and %s would not.",
		diskCleanCount(runnable), diskCleanCount(len(plan.Items)-runnable))
}

// diskCleanItemCard is one finding's evidence, one line per fact: what it is and
// how it is classified, what it holds and where that number came from, why the
// scan found it, where it is or what it covers, and what removing it would cost
// to get back. It is the answer to the question a row cannot hold, and it is the
// reason a row never has to carry a number without its source.
func diskCleanItemCard(item diskclean.Item, action string) []string {
	head := item.Title
	if item.Category != "" {
		head += " — " + item.Category.Label()
	}
	head += " — " + item.Risk.Label()

	// What it holds comes before why it was found: the number is the reason the row
	// is interesting, and the evidence explains where it came from. Both are on
	// screen at the smallest frame this screen is measured at.
	lines := []string{head}
	if strings.TrimSpace(action) != "" {
		// In the preview the action is the fact the confirmation is about, so it sits
		// directly under the title where a small terminal cannot cut it away from it.
		lines = append(lines, action)
	}
	lines = append(lines, diskCleanSizeSentence(item))
	if reason := strings.TrimSpace(item.Reason); reason != "" {
		lines = append(lines, "Found because "+diskCleanSentence(reason))
	}
	// The paths and the engine objects are here for the findings list, where they
	// are the answer to "where is this?". In the preview they are not repeated: the
	// action line above already names the exact path a removal would take, the
	// objects a tool command would prune are in the size line, and at the frame the
	// request treats as the floor that line is what buys the reconstruction cost its
	// place on the screen.
	if strings.TrimSpace(action) == "" {
		if len(item.Paths) > 0 {
			// The label and the paths are separate lines: a path is one token, and a
			// token that does not fit is broken mid-name by the shared wrap. On its
			// own line a path either fits or is cut with the marker, and the reader
			// never sees a directory name split in two.
			lines = append(lines, "On disk:", "  "+strings.Join(item.Paths, ", "))
		}
		if len(item.VolatileObjects) > 0 && item.Size.Source != diskclean.SourceTool {
			lines = append(lines, "Inside the engine: "+strings.Join(item.VolatileObjects, ", "))
		}
	}
	if detail := strings.TrimSpace(item.Detail); detail != "" {
		lines = append(lines, diskCleanSentence(detail))
	}
	return lines
}

// diskCleanSizeSentence says what the finding holds and which half of the number
// is the disk's, in one line: the row above already carries the reclaimable
// figure and the risk, and a second wrapped sentence about the same number would
// push the reason and the paths off a small terminal. The caveats that qualify the
// number -- a walk cut short, bytes shared with another finding -- are appended
// here rather than kept for the paragraph below, because they say what the number
// is worth and they belong beside it.
func diskCleanSizeSentence(item diskclean.Item) string {
	switch {
	case item.Size.Missing:
		return "Holds nothing: the store is not there any more, or was never created."
	case item.Size.Err != "":
		return "Its size could not be measured: " + diskCleanSentence(item.Size.Err)
	}

	parts := []string{fmt.Sprintf("Holds %s on disk", humanBytes(uint64(item.Size.Physical)))}
	if item.Size.Logical != item.Size.Physical {
		parts = append(parts, fmt.Sprintf("%s in contents", humanBytes(uint64(item.Size.Logical))))
	}
	if item.Size.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d files", item.Size.Files))
	}
	source := "walked"
	if item.Size.Source == diskclean.SourceTool {
		source = "tool-reported"
	}
	sentence := strings.Join(parts, ", ") + " (" + source + ")"
	if len(item.VolatileObjects) > 0 {
		sentence += ": " + strings.Join(item.VolatileObjects, ", ")
	}
	sentence += "."
	if item.Size.Truncated {
		sentence += " The walk stopped at its own ceiling, so this is a lower bound rather than the whole store."
	}
	if item.Size.Shared > 0 {
		sentence += " Some files are counted under another finding too; they are counted once and listed here as shared."
	}
	return sentence
}

// diskCleanSentence makes a fragment written to be read into a sentence: one
// terminal stop, no doubled one, so the pieces join without printing ".." when a
// reason already ended in one.
func diskCleanSentence(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, ". ")
	return trimmed + "."
}

// diskCleanResultParagraphs is what the run did, item by item: what came back,
// what did not, and why. The report's own lead number is the footer's, so this
// names the items and their outcomes rather than repeating the total.
func diskCleanResultParagraphs(report diskclean.CleanReport) []string {
	if len(report.Results) == 0 {
		return []string{"The run finished without reaching an item."}
	}

	var paragraphs []string
	for _, result := range report.Results {
		switch {
		case !result.Ran:
			paragraphs = append(paragraphs, fmt.Sprintf("%s: not run. %s", result.Item.Title, result.Error))
		case result.Refused:
			paragraphs = append(paragraphs, fmt.Sprintf("%s: refused before anything was removed. %s",
				result.Item.Title, result.Error))
		case result.Error != "":
			paragraphs = append(paragraphs, fmt.Sprintf("%s: %s, and %s came back; %s",
				result.Item.Title, diskCleanFreedPhrase(result), diskCleanCount(len(result.Skipped)), result.Error))
		default:
			paragraphs = append(paragraphs, fmt.Sprintf("%s: %s.", result.Item.Title, diskCleanFreedPhrase(result)))
		}
		if len(result.Skipped) > 0 {
			paragraphs = append(paragraphs, "  Left in place: "+strings.Join(boundedList(result.Skipped, 4), ", ")+
				". Entries are listed because a program holds them open, they belong to another user, or removing "+
				"them was refused; they are not silently skipped.")
		}
	}

	if report.FreedMeasured {
		paragraphs = append(paragraphs, fmt.Sprintf("The filesystem gained %s around the run, and that is the number "+
			"to trust: it is what the disk reported before the first command and after the last one.",
			humanBytes(uint64(report.FreedBytes))))
	} else if report.FreedNote != "" {
		paragraphs = append(paragraphs, "How much the run released is not stated as a single number: "+report.FreedNote+
			". The per-item numbers above are what the run itself measured.")
	}
	if report.Cancelled {
		paragraphs = append(paragraphs, "The run was stopped, so the items it had not reached were left exactly as "+
			"they were.")
	}
	return paragraphs
}

// diskCleanFreedPhrase is one item's outcome in words, with the source of the
// number it quotes.
func diskCleanFreedPhrase(result diskclean.Result) string {
	if result.Freed <= 0 {
		return "nothing came back"
	}
	switch result.FreedSource {
	case diskclean.SourceTool:
		return fmt.Sprintf("the tool reported %s reclaimed", humanBytes(uint64(result.Freed)))
	default:
		return fmt.Sprintf("%s came back", humanBytes(uint64(result.Freed)))
	}
}

// boundedList keeps a result line readable when a removal left many entries
// behind, without dropping the fact that there were more.
func boundedList(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	kept := append([]string(nil), values[:limit]...)
	return append(kept, fmt.Sprintf("and %d more", len(values)-limit))
}

// diskCleanScopeParagraph says what was searched and how that scope is changed:
// the stores are known locations, and the project directories are only the ones
// the user added.
func diskCleanScopeParagraph(st diskCleanState) string {
	paragraphs := []string{"The scan looks at the stores the tools keep in their own locations and at the project " +
		"directories listed here. It never walks a whole disk, and it never follows a symbolic link out of a store " +
		"it knows."}

	if len(st.ProjectRoots) == 0 {
		paragraphs = append(paragraphs, "No project directory has been added, so the build artifacts and dependency "+
			"trees inside projects are not part of this scan.")
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf("Project directories in scope: %s. Only these are searched for "+
			"project-level stores -- node_modules, target, build, dist, __pycache__, a virtual environment and the "+
			"like -- and the row above takes the last one back out.",
			strings.Join(st.ProjectRoots, ", ")))
	}
	if st.Cwd != "" && !diskCleanHasRoot(st.ProjectRoots, st.Cwd) {
		paragraphs = append(paragraphs, fmt.Sprintf("%s adds %s, the directory this installer was started from.",
			diskCleanAddRootPrefix+st.Cwd, st.Cwd))
	}
	return strings.Join(paragraphs, " ")
}

// diskCleanSafetyParagraph is the honest account of what the utility will not
// do, kept on screen rather than in the documentation only.
func diskCleanSafetyParagraph(m Model) string {
	paragraphs := []string{"A container engine's or a package manager's own store is never deleted file by file: " +
		"the engine's own prune command and the manager's own cache command are used, and where no such command " +
		"exists the finding is reported rather than removed. A command that needs administrator rights is attempted " +
		"without a prompt, and if it is refused the screen names the exact command to run yourself. Nothing is " +
		"quarantined: a removal is a removal, and the preview says what would have to be downloaded or rebuilt " +
		"again."}

	if m.SystemInfo != nil && m.SystemInfo.IsWSL {
		paragraphs = append(paragraphs, "Inside WSL this frees space in the Linux filesystem, not on the Windows "+
			"drive: the distribution's virtual disk does not shrink on its own, and this utility will not compact it "+
			"for you. Compacting is a separate decision made from Windows, after the space has actually been freed "+
			"here.")
	}
	paragraphs = append(paragraphs, "Nothing is reported anywhere: no telemetry, no network call, and no cleanup "+
		"in the background. A scan or a cleanup runs only while you are on this screen, and Escape stops either one.")
	return strings.Join(paragraphs, " ")
}
