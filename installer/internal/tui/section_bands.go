package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
)

// ============================================================================
// THE SECTION FRAME: BANDS THAT DO NOT MOVE
// ============================================================================
//
// A section screen -- the utilities hub and every destination one level in --
// draws its body as a stack of bands. The height of every band comes from the
// frame and a per-screen constant; none of them reads the row the cursor is on,
// the prose that row would print, a notice's length, or an action that appears
// once something is selected. That is the whole contract, and it is what keeps a
// screen from moving under the user while they walk a list: moving the cursor
// changes what a row says, never where the rows are.
//
// The bands, top to bottom:
//
//	title    one row naming the screen, omitted when the header already names it
//	detail   the screen's prose: a fixed reservation, or as many rows as it wraps to
//	gap      one blank row between the prose and the list
//	list     the rows left: the entries window, then any activity and preview, then
//	         the action tail (the separator, the actions and the way back) anchored
//	         to the band's foot
//	notice   a blank row and the result line, at the foot of the body
//
// The list band is sized from the action tail's RESERVED height, not from the
// tail on screen. A tail that grows when the first entry is selected therefore
// spends the slack that was already held above it, and the entry window stays the
// same size: the row the cursor is on cannot move because an action appeared.
//
// A screen whose prose follows the cursor -- the finding under the cursor, the
// entry being looked at -- reserves a fixed detail band and clips its prose to
// it. A screen whose prose is page-level -- a report, a section's reasons --
// takes the rows it wraps to, after the list has taken the rows it needs, and
// keeps a floor so a short frame cannot cut it away entirely.

// detailAsNeeded is the detail band's value on a screen whose prose does not
// follow the cursor: the band takes the rows the prose wraps to, within the rows
// the list leaves.
const detailAsNeeded = -1

// sectionNoticeRows is the most content rows the notice band reserves. It is the
// same two rows the notice has always been allowed to wrap to, so a result that
// used to be readable still is; the band reserves them even when there is no
// notice, which is what keeps the list band's height constant.
const sectionNoticeRows = 2

// sectionMinEntryRows is the smallest window the list band keeps for the entries
// beside the action tail. The reservations yield before it does: a band whose
// actions fill it is a band the user cannot walk.
const sectionMinEntryRows = 3

// themeActivitySlotMax is the tallest the theme picker's activity slot gets from
// the spare rows. The slot names the tools a switch painted; past this it would
// be a second screen rather than a line under the list it belongs to.
const themeActivitySlotMax = 8

// usesSectionBands reports whether the screen renders through renderThemeScreen
// and therefore answers to the band contract. It is the same list the renderer's
// callers make, named in one place so a guard can ask which screens it governs.
func (m Model) usesSectionBands() bool {
	switch m.Screen {
	case ScreenUtilities, ScreenThemePicker, ScreenWSLResources,
		ScreenTerminalCapabilities, ScreenShellAudit, ScreenUpdate, ScreenUpdateCenter,
		ScreenDiskCleaner, ScreenInstallVerify, ScreenNetworkDiagnostics,
		ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys, ScreenSSHAgent,
		ScreenSSHDiagnostics, ScreenSSHTest, ScreenDoctor, ScreenDoctorFix,
		ScreenWSLAdvisor, ScreenWSLBenchmark, ScreenWSLMigration:
		return true
	}
	return false
}

// sectionBand is one screen's own reservation. The zero value is the default:
// as-needed prose, no floor and a two-row notice band. A screen overrides only
// the field it needs to.
type sectionBand struct {
	Detail int // detailAsNeeded, or the rows a cursor-following prose reserves
	Floor  int // the prose's floor when the list would otherwise take the frame
	Notice int // content rows in the notice band; 0 leaves the band out
}

// sectionBandFor is the registry: the screens whose prose follows the cursor,
// the screens that print no notice at all, and the floor each prose keeps. It is
// a switch over the screen rather than a field on the model so a screen cannot
// drift from the contract its guard measures.
func (m Model) sectionBandFor() sectionBand {
	switch m.Screen {
	case ScreenUpdateCenter:
		// The detail band explains the entry under the cursor: what manager owns
		// it, what the installed and available versions are, why a hold or a note
		// applies. Four rows hold the label/value rows the screen prints.
		return sectionBand{Detail: 4, Floor: 4, Notice: sectionNoticeRows}
	case ScreenDiskCleaner:
		// The card is drawn for the finding under the cursor, but the band is
		// reserved as soon as the scan has findings to look at: whether the cursor
		// happens to be on the scan row is a cursor position, and a band may not
		// read one. Before a scan there is no card to hold, so the section's own
		// safety prose is the content and must not be cut to a card's height.
		if m.DiskClean.Scanned && len(m.DiskClean.Items) > 0 {
			if m.DiskClean.Phase == diskCleanPreview {
				// The preview adds its own headline above the card, so it reserves
				// the two rows the headline wraps to on top of the card's floor.
				return sectionBand{Detail: diskCleanCardFloor + diskCleanPreviewHeadFloor, Notice: sectionNoticeRows}
			}
			return sectionBand{Detail: diskCleanCardFloor, Notice: sectionNoticeRows}
		}
		return sectionBand{Detail: detailAsNeeded, Floor: 4, Notice: sectionNoticeRows}
	case ScreenDoctor:
		return sectionBand{Detail: 6, Floor: 6, Notice: sectionNoticeRows}
	case ScreenDoctorFix:
		// The confirmation's prose is the change, its risk, its permission and its
		// backup: it is the screen, so it takes what it needs.
		return sectionBand{Detail: detailAsNeeded, Floor: 4, Notice: sectionNoticeRows}
	case ScreenThemePicker:
		return sectionBand{Detail: 2, Floor: 2, Notice: 1}
	case ScreenTerminalCapabilities, ScreenInstallVerify:
		// These screens report rather than list: their prose IS the screen, so the
		// notice band is not drawn at all and the list band is the way back.
		return sectionBand{Detail: detailAsNeeded, Floor: 2, Notice: 0}
	case ScreenUtilities:
		// The section's prose names what is missing and how the switch works; the
		// list of jobs is the screen. Both matter, so the prose keeps a floor.
		return sectionBand{Detail: detailAsNeeded, Floor: 4, Notice: sectionNoticeRows}
	case ScreenShellAudit:
		return sectionBand{Detail: detailAsNeeded, Floor: 0, Notice: sectionNoticeRows}
	case ScreenWSLResources:
		return sectionBand{Detail: detailAsNeeded, Floor: 0, Notice: sectionNoticeRows}
	}
	return sectionBand{Detail: detailAsNeeded, Floor: 0, Notice: sectionNoticeRows}
}

// sectionBands is the row arithmetic of one rendered section screen. Every field
// is a number of rows, and the sum of the bands is exactly Body -- the rows the
// frame leaves between the header rule and the rule above the footer -- so a
// section screen fills its frame instead of floating in it.
type sectionBands struct {
	Footer   int // rows the footer reserves, including the hints a selection can add
	Body     int // rows between the two rules
	Title    int // 1 when the body names the screen, 0 when the header does
	Detail   int // rows the detail band reserves
	Gap      int // 1 between the prose and the list, 0 when there is no prose
	List     int // the rows left for entries, activity, preview and the action tail
	Pinned   int // rows the action tail reserves at the foot of the list band
	Notice   int // the blank row and the notice, at the foot of the body
	Preview  int // rows the live preview reserves under the entries
	Activity int // rows the theme picker's activity slot reserves under the entries
}

// entryRows is the window the entries scroll in: the list band minus the action
// tail's RESERVED rows, minus the activity and preview rows drawn under the
// entries. It is measured from the reservation, never from the tail the current
// state happens to draw, so an action that appears with a selection cannot
// resize the window.
func (b sectionBands) entryRows(activity int) int {
	rows := b.List - b.Pinned - activity - b.Preview
	if rows < 1 {
		rows = 1
	}
	return rows
}

// sectionBandsFor measures one screen. It reads the screen, the height, the
// width, the hints, the wrapped detail count and the menu length -- never the
// cursor, the selection or the notice. The menu length is a parameter because a
// menu's rows do not change while the cursor walks them, and giving the list the
// rows it needs is what keeps a menu whole on a frame with room for it.
func (m Model) sectionBandsFor(width int, hints []installerHint, detailLines, menuRows int) sectionBands {
	band := m.sectionBandFor()
	b := sectionBands{
		Footer: m.footerReserveRows(width, hints),
		Pinned: m.pinnedTailReserve(),
	}
	b.Body = installerBodyRows(m.Height, b.Footer)
	if m.sectionTitleInBody() {
		b.Title = 1
	}
	// The preview follows the cursor, so the band reserves its row on every state
	// of the picker: a preview that appears on one row and not the next must not
	// change the window the entries scroll in.
	if m.Screen == ScreenThemePicker {
		b.Preview = 1
	}
	notice := 0
	if band.Notice > 0 {
		notice = 1 + band.Notice
	}

	// The list's own need is the whole menu, plus the preview row the picker draws
	// under it. Its floor is the action tail plus a few entries -- as many entries
	// as the menu has, when it has fewer.
	entries := menuRows - b.Pinned
	if entries < 1 {
		entries = 1
	}
	minimum := b.Pinned + min(entries, sectionMinEntryRows)
	listNeed := max(menuRows+b.Preview, minimum)

	// The wait on the theme picker owns the rows the prose and the notice were
	// holding, never the list's: the press that starts it was made on the list and
	// the user is still choosing from it. Pending and landed allocate the same
	// rows, so the slot cannot move the list when the result arrives.
	if m.pickerActivity() != nil {
		available := b.Body - b.Title
		spare := available - listNeed
		if spare < 0 {
			spare = 0
		}
		if spare > themeActivitySlotMax {
			spare = themeActivitySlotMax
		}
		b.Activity = spare
		// The slot is inside the list band, under the entries: the band is the whole
		// available height, and entryRows subtracts the slot, the preview and the
		// tail from it.
		b.List = available
		return b
	}

	available := b.Body - b.Title - notice
	if available < 1 {
		available = 1
	}

	if band.Detail >= 0 {
		b.Detail = min(band.Detail, available)
		if b.Detail > 0 {
			b.Gap = 1
		}
	} else {
		// As-needed prose: the list keeps the rows it needs and the prose takes what
		// is left, never below its floor. A frame too short for both keeps the
		// floor and windows the list.
		room := available - 1 - listNeed
		if room < band.Floor {
			room = band.Floor
		}
		if room < 0 {
			room = 0
		}
		b.Detail = min(detailLines, room)
		if b.Detail > 0 {
			b.Gap = 1
		}
	}
	b.Notice = notice
	b.List = available - b.Detail - b.Gap
	if b.List < 0 {
		b.List = 0
	}

	// A frame too short for the reservations gives its rows back in the order that
	// costs the reader least: the notice (a status line), then the prose -- which
	// is clipped out loud rather than silently. The list keeps the action tail and
	// a few entries.
	for b.List < minimum && b.Notice > 0 {
		b.Notice--
		b.List++
	}
	for b.List < minimum && b.Detail > 0 {
		b.Detail--
		b.List++
		if b.Detail == 0 && b.Gap > 0 {
			b.Gap = 0
			b.List++
		}
	}
	if b.List < 1 {
		b.List = 1
	}
	return b
}

// sectionTitleInBody reports whether the body opens with the screen's own name.
// When the frame's header already carries that name the body starts with what
// the screen is for, instead of repeating the label one row lower.
func (m Model) sectionTitleInBody() bool {
	return m.GetScreenTitle() != m.headerName()
}

// footerReserveRows is how many rows the footer reserves for a screen. It is the
// packing of every hint the screen can offer in this state, not only the ones it
// offers now: an action that appears once something is selected must not take a
// row from the body, because taking it from the body is what moved the list.
func (m Model) footerReserveRows(width int, hints []installerHint) int {
	return footerRowCount(width, dedupeHints(append(append([]installerHint{}, hints...), m.selectionHintReserve()...)))
}

// selectionHintReserve is the hints a screen's footer can gain while the user
// walks it, and nothing else. A screen that names the same keys in every state
// has no entry here.
func (m Model) selectionHintReserve() []installerHint {
	switch m.Screen {
	case ScreenUpdateCenter:
		return []installerHint{
			updateCenterHintPick, updateCenterHintSelectShown, updateCenterHintApply,
			updateCenterHintClear, updateCenterHintCheck, updateCenterHintFilter,
		}
	case ScreenDiskCleaner:
		return []installerHint{
			diskCleanToggleHint, diskCleanPreviewHint, diskCleanAddRootHint,
			diskCleanForgetRootHint, diskCleanScanHint, diskCleanCleanHint,
		}
	case ScreenUtilities:
		return []installerHint{hintDark, hintLight, hintUndo}
	case ScreenThemePicker:
		return []installerHint{hintRefresh, hintUndo}
	}
	return nil
}

// dedupeHints drops a hint whose keys are already in the list. The reserve is
// the union of what the screen offers and what it can offer, so a key both sets
// name is one hint and one key's worth of columns.
func dedupeHints(hints []installerHint) []installerHint {
	seen := make(map[string]bool, len(hints))
	out := make([]installerHint, 0, len(hints))
	for _, h := range hints {
		if seen[h.keys] {
			continue
		}
		seen[h.keys] = true
		out = append(out, h)
	}
	return out
}

// pinnedTailLen is how many trailing menu rows are always drawn: the action
// block that ends a section screen's list, from the separator that opens it
// through the way back. It is measured from the options the handler itself
// reads, so the rows the frame pins and the rows the keys accept are one list.
//
// A menu with no trailing block -- a list that simply ends -- pins nothing; its
// rows scroll like its entries. The cap is the tallest action block a section
// screen draws, and it keeps a group separator that happens to be the last one
// from pinning a screenful of data as if it were an action.
const pinnedTailMaxRows = 8

func pinnedTailLen(options []string) int {
	for i := len(options) - 1; i >= 0; i-- {
		if strings.HasPrefix(options[i], menuSeparatorPrefix) {
			if tail := len(options) - i; tail <= pinnedTailMaxRows {
				return tail
			}
			return 0
		}
	}
	return 0
}

// themePickerPinnedReserve is the picker's tallest action tail: the separator,
// the refresh and the undo rows and the way back. The undo row appears once a
// theme has been applied, so the tail must be measured with it even before the
// apply that creates it -- otherwise the list would shrink by a row when the
// result lands and take the cursor row with it.
const themePickerPinnedReserve = 4

// pinnedTailReserve is the rows the action tail's height is measured against,
// which is the tallest the tail can get in this state: the current tail, or the
// tail the state would draw with every entry selected. The update center's is
// its tallest action block in every state, because a selection adds two rows to
// it and those rows must come out of the slack held above the tail.
func (m Model) pinnedTailReserve() int {
	actual := pinnedTailLen(m.GetCurrentOptions())
	switch m.Screen {
	case ScreenUpdateCenter:
		return max(actual, updateCenterPinnedReserve)
	case ScreenThemePicker:
		return max(actual, themePickerPinnedReserve)
	case ScreenDiskCleaner:
		// Marking the first finding adds the preview row to the action block, so the
		// reserve is measured with every finding marked. The selection is a map and a
		// model copy shares it: the probe gets a copy of its own, because a render
		// that selected the findings on the model would change the screen it was
		// measuring -- and the user's choice with it.
		probe := m
		probe.DiskClean.Selection = make(diskclean.Selection, len(m.DiskClean.Selection)+len(m.DiskClean.Items))
		for id := range m.DiskClean.Selection {
			probe.DiskClean.Selection[id] = true
		}
		for _, item := range probe.DiskClean.Items {
			probe.DiskClean.Selection[item.ID] = true
		}
		return max(actual, pinnedTailLen(probe.GetCurrentOptions()))
	}
	return actual
}

// wrapParagraphs wraps every paragraph of a screen's prose to width, one slice
// per paragraph, without cutting anything.
func wrapParagraphs(paragraphs []string, width int) []string {
	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapText(paragraph, width, 0)...)
	}
	return lines
}

// clipLines cuts lines to rows the same way every other block in the installer
// is cut: the rows that fit, with a marker naming what it could not show. A
// block within its budget is returned unchanged.
func clipLines(lines []string, rows, width int) []string {
	if rows < 1 {
		return nil
	}
	if len(lines) <= rows {
		return lines
	}
	hidden := len(lines) - rows + 1
	kept := append([]string{}, lines[:rows-1]...)
	return append(kept, MutedStyle.Render(fmt.Sprintf("… and %d more", hidden)))
}

// padRows pads lines at the foot to exactly rows rows. A block already at or
// past its budget is returned unchanged: a renderer that draws more rows than it
// reserved must fail the frame guard, not be quietly clipped here.
func padRows(lines []string, rows int) []string {
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}

// noticeBand is a screen's result line in its reserved rows: the blank that
// separates it from the list, then the line wrapped to the width, cut with the
// usual marker when it needs more rows than the band holds, then the rest of the
// band left blank. An empty notice keeps the blank and the rows, which is what
// keeps the frame from shrinking when a result is cleared.
func noticeBand(text string, rows, width int) []string {
	if rows <= 0 {
		return nil
	}
	out := []string{""}
	content := rows - 1
	if content > 0 && text != "" {
		out = append(out, clipLines(wrapText(text, width, 0), content, width)...)
	}
	return padRows(out, rows)
}
