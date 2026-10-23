package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// ============================================================================
// UPDATES — the reference's third screen
// ============================================================================
//
// The reference draws the update center as a stats strip, a grouped table with a
// checkbox column, a header row and a scrollbar, and — at the right — the card of
// the package under the cursor with its real facts, plus the keys the screen
// reads. The footer carries the run's own status between the state and the keys.
//
// The wide shape is the reference's; the 80x24 floor keeps the section renderer
// (renderThemeScreen), where the same rows and the same keys are drawn in one
// column. Every count, version, manager, scope, note and command is the model's
// own; the screen adds no description a manager did not report.

const (
	updateTableCheckWidth = 2
	updateTableScrollCol  = 1
	updateTableGap        = 2

	// updateTableNoteRows is the rows the window's two notes hold: how many rows
	// above it and how many below it the window hid. They are reserved whether or
	// not the window draws them, so reaching the first or the last row cannot
	// change the card's height and move the frame around it.
	updateTableNoteRows = 2

	// updateTableWindowRows is the tallest window the table draws. The card used to
	// take every row the stats strip left, so a machine with seventy packages drew a
	// table from the strip to the footer: the rows existed whether or not the list
	// had that many, and the composition had no room left to sit centred. The
	// window is a reading measure in rows now -- about the table the reference draws
	// -- and a longer list scrolls inside it, so the card hugs the rows it shows.
	updateTableWindowRows = 20
)

// updateCenterWide reports whether the reference's two-column update screen fits
// the frame. It needs rows for the stats strip, a usable table and the two cards
// at the right; below that the section renderer's one column is the honest shape.
func (m Model) updateCenterWide(l layout, budget int) bool {
	if !l.TwoColumn || l.Left < 58 || l.Right < 58 {
		return false
	}
	return budget >= 30
}

// renderUpdateCenterWide draws the reference's update screen.
func (m Model) renderUpdateCenterWide(l layout, budget int) string {
	hints := m.updateCenterHints()
	left := m.updateCenterTableColumn(l.Left, budget)
	right := m.updateCenterDetailColumn(l.Right, budget)
	body := composeColumns(left, right, l)

	vital := frameVital("")
	if len(m.updateCenterItems()) > 0 || m.UpdateCenter.InFlight {
		if shown := len(m.updateCenterItems()); shown > 0 {
			vital = fmt.Sprintf("%d updates", shown)
		}
	}
	if selected := m.UpdateCenter.Selected.Count(); selected > 0 {
		vital = fmt.Sprintf("%d selected · %s", selected, vital)
	}
	return m.frameWithPanels(m.headerName(), vital, body, hints, nil)
}

// updateCenterTableColumn is the left column: the stats strip, the filter pills
// and the grouped table. The table is given the rows its own window needs, never
// the column's whole budget: a card that took the rows the frame had left would
// draw the frame's last row merely because it existed.
func (m Model) updateCenterTableColumn(width, budget int) []string {
	var out []string
	out = append(out, m.updateCenterStatsStrip(width)...)
	out = append(out, "")

	limit := max(cardChromeRows+1, budget-len(out))
	out = append(out, m.updateCenterTable(width, limit)...)
	return out
}

// updateCenterStatsStrip is the reference's strip: how many entries are known,
// the count each manager reports, and the filter the list is showing. A manager
// with no entries is left out of the counts, because a zero would look like an
// answer the manager did not give.
func (m Model) updateCenterStatsStrip(width int) []string {
	items := m.updateCenterItems()
	content := cardContentWidth(width)

	counts := map[updates.ManagerID]int{}
	for _, item := range items {
		counts[item.Manager]++
	}
	var managers []updates.ManagerID
	for _, manager := range m.updateCenterManagersWithItems() {
		if counts[manager] > 0 {
			managers = append(managers, manager)
		}
	}

	head := AccentKeyStyle.Render("\uf1b3") + " " +
		BrandStyle.Render(fmt.Sprintf("%d updates available", len(items)))
	if len(items) == 0 {
		head = AccentKeyStyle.Render("\uf1b3") + " " + BrandStyle.Render("No updates found")
	}
	subtitle := MutedStyle.Render("Keep your system up to date")

	body := []string{head + "  " + subtitle}

	var stats []string
	for _, manager := range managers {
		stats = append(stats, fmt.Sprintf("%d %s", counts[manager], manager))
	}
	if len(stats) == 0 {
		stats = append(stats, "No manager has reported yet")
	}
	body = append(body, MutedStyle.Render(truncate(strings.Join(stats, "   "), content)))

	filter := "Filter  "
	pills := []string{m.updateCenterFilterPill("all managers", m.UpdateCenter.Filter == "")}
	for _, manager := range managers {
		pills = append(pills, m.updateCenterFilterPill(string(manager), m.UpdateCenter.Filter == manager))
	}
	body = append(body, MutedStyle.Render(truncate(filter+strings.Join(pills, " "), content)))

	return card("", "", width, body)
}

// updateCenterFilterPill draws one filter choice: filled when it is the filter in
// force, bordered otherwise.
func (m Model) updateCenterFilterPill(label string, active bool) string {
	if active {
		return RowBarStyle.Render(" " + label + " ")
	}
	return PanelBorderStyle.Render("(") + MutedStyle.Render(" "+label+" ") + PanelBorderStyle.Render(")")
}

// updateCenterVisibleGroup is one manager's entries as the table draws them.
type updateCenterVisibleGroup struct {
	manager updates.ManagerID
	items   []updates.Item
}

// updateCenterVisibleGroups groups the filtered entries by the manager that owns
// them, in the inventory's order, skipping managers with nothing to show.
func (m Model) updateCenterVisibleGroups() []updateCenterVisibleGroup {
	counts := map[updates.ManagerID][]updates.Item{}
	for _, item := range m.updateCenterVisibleItems() {
		counts[item.Manager] = append(counts[item.Manager], item)
	}
	var groups []updateCenterVisibleGroup
	for _, manager := range m.updateCenterManagersWithItems() {
		if items := counts[manager]; len(items) > 0 {
			groups = append(groups, updateCenterVisibleGroup{manager: manager, items: items})
		}
	}
	return groups
}

// updateCenterTable draws the reference's table: the checkbox and the four
// columns on the header row, a titled group per manager with its own count, one
// row per entry, and a scrollbar on the right. The row under the cursor is one
// filled bar; the group headers are not cursor positions.
//
// The list scrolls INSIDE the card and the card is exactly as tall as the rows it
// draws: `limit` is the most it may take, and the window is the list when it fits
// in that room, the reading measure (updateTableWindowRows) when it does not. A
// windowed list spends a row at each end naming what it hid, and the two note rows
// are drawn even when one of them has nothing to say, because a card that grew a
// row when the cursor reached an end would move the rows around it. Nothing here
// pads: a card with more room than rows ends at its last row, so the composition
// around it stays centred.
func (m Model) updateCenterTable(width, limit int) []string {
	groups := m.updateCenterVisibleGroups()
	content := cardContentWidth(width)

	if len(groups) == 0 {
		message := "Reading the managers…"
		if m.UpdateCenter.Resolved && !m.UpdateCenter.InFlight {
			message = "The managers report nothing to update."
		}
		return card("", "", width, []string{MutedStyle.Render(truncate(message, content))})
	}

	// The columns share the content the checkbox and the scrollbar leave. A name
	// and a version pair get the room; the owner and the scope keep their words.
	room := content - updateTableCheckWidth - updateTableScrollCol - updateTableGap*4
	nameWidth := min(24, max(14, room*30/100))
	versionWidth := min(30, max(22, room*38/100))
	ownerWidth := min(10, max(8, room-nameWidth-versionWidth-12))
	scopeWidth := max(6, room-nameWidth-versionWidth-ownerWidth-4*updateTableGap)

	header := groupedTableHeader(
		[]string{" ", "Package", "Current → New version", "Manager", "Scope"},
		[]int{updateTableCheckWidth, nameWidth, versionWidth, ownerWidth, scopeWidth},
		content)

	focused, hasFocus := m.updateCenterDetailItem()

	// Build the flat list of drawn objects first, so the window arithmetic is one
	// pass over the same objects the renderer draws.
	type tableLine struct {
		header string
		item   updates.Item
		isItem bool
	}
	var all []tableLine
	for _, group := range groups {
		all = append(all, tableLine{header: fmt.Sprintf("▼ %s (%d updates)", group.manager, len(group.items))})
		for _, item := range group.items {
			all = append(all, tableLine{item: item, isItem: true})
		}
	}

	focusLine := 0
	if hasFocus {
		for i, line := range all {
			if line.isItem && line.item.Key() == focused.Key() {
				focusLine = i
				break
			}
		}
	}

	// The heading and the rule under it are the table's own; what is left is the
	// window and, when the list does not fit it, the two notes.
	bodyRows := max(1, limit-cardChromeRows-2)
	window := min(len(all), updateTableWindowRows, bodyRows)
	if window < len(all) {
		// A windowed list spends a row at each end naming what it hid, so the
		// window yields to them rather than pushing the card past its room.
		window = max(1, min(window, bodyRows-updateTableNoteRows))
	}
	start, end := listWindow(focusLine, window, len(all))

	body := []string{header, rule(content)}
	for i := start; i < end; i++ {
		line := all[i]
		if line.isItem {
			body = append(body, m.updateCenterTableRow(line.item, focused, hasFocus, content,
				nameWidth, versionWidth, ownerWidth, scopeWidth, updateScrollbarCell(i, start, end, len(all))))
			continue
		}
		// The group header carries its own count and the thin group mark.
		body = append(body, BrandStyle.Render(truncate(line.header, content)))
	}
	if start > 0 || end < len(all) {
		// Both note rows are drawn, blank where the window has nothing to report:
		// the card keeps its height, so the two rows under it stay where they were.
		above, below := "", ""
		if start > 0 {
			above = MutedStyle.Render(fmt.Sprintf("↑ %d more above", start))
		}
		if end < len(all) {
			below = MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(all)-end))
		}
		body = append(body, above, below)
	}
	if rows := max(1, limit-cardChromeRows); len(body) > rows {
		// A column too short for the heading, one row and the notes keeps the rows it
		// has: the two-column shape is gated well above this, so the cut is a floor
		// and never the shape a reader meets.
		body = body[:rows]
	}
	return card("", "", width, body)
}

// updateCenterTableRow draws one entry: the checkbox, the name, the two versions,
// the owner, the scope and the scrollbar cell. The row under the cursor is filled
// across the table, checkbox included.
func (m Model) updateCenterTableRow(item updates.Item, focused updates.Item, hasFocus bool, content,
	nameWidth, versionWidth, ownerWidth, scopeWidth int, scroll string) string {
	check := "[ ]"
	if m.UpdateCenter.Selected.Has(item) {
		check = "[x]"
	}
	name := padRight(truncate(item.Name, nameWidth), nameWidth)
	versions := item.Current
	if versions == "" {
		versions = "unknown"
	}
	if item.Available != "" {
		versions += " → " + item.Available
	}
	versions = padRight(truncate(versions, versionWidth), versionWidth)
	owner := padRight(truncate(string(item.Manager), ownerWidth), ownerWidth)
	scope := padRight(truncate(string(item.Scope), scopeWidth), scopeWidth)

	plain := padRight(check, updateTableCheckWidth) + " " +
		name + strings.Repeat(" ", updateTableGap) +
		versions + strings.Repeat(" ", updateTableGap) +
		owner + " " + scope + " " + scroll
	plain = padRight(truncate(plain, content), content)

	if hasFocus && item.Key() == focused.Key() {
		return RowBarStyle.Render(plain)
	}
	// The checkbox keeps its meaning when the row is not selected; the name is the
	// body ink and the metadata stays muted so the eye can compare down a column.
	rest := strings.TrimPrefix(plain, check)
	return MutedStyle.Render(check) + InkStyle.Render(rest)
}

// updateScrollbarCell is one cell of the table's scrollbar: a filled block where
// the window sits in the list, a hatched cell elsewhere. The track is one cell per
// row the window draws and it stands for the whole list, so the window's slice
// lands on the track in proportion: the thumb is as long as the slice's share of
// the list, and it begins where the slice's first row falls on that share. The
// thumb therefore starts at the top of the column and ends at its foot as the
// window walks the list, instead of sitting in the middle of the track wherever
// the window is.
func updateScrollbarCell(index, start, end, total int) string {
	if total <= 0 || end <= start {
		return " "
	}
	if index < start || index >= end {
		return " "
	}

	track := end - start
	// The slice's share of the track, rounded up so a window of one row in a long
	// list still fills a cell; the whole list fills the whole track.
	thumb := min(track, max(1, (track*track+total-1)/total))
	lo := min(track-thumb, start*track/total)
	hi := lo + thumb

	at := index - start
	if at >= lo && at < hi {
		return MeterFilledStyle.Render("█")
	}
	return MeterStyle.Render("░")
}

// updateCenterDetailColumn is the right column: the entry under the cursor and
// the keys this screen reads -- or, while a run is on the screen, the run's own
// progress instead of the entry. The entry's card is as tall as the tallest
// reading the cursor can reach and no taller: a card measured from the row under
// the cursor would change the composition's height as the user walks the list, and
// one padded to the column's whole budget left a short reading inside a card as
// tall as the frame.
func (m Model) updateCenterDetailColumn(width, budget int) []string {
	// The keys are the shorter card and the one that must not be cut through, so
	// they are measured first and the entry's card takes the rows that are left. A
	// long note then cuts the entry's card with the frame's marker instead of
	// growing the body past the rows the frame gave it.
	actions := card("Actions", "Keyboard shortcuts", width, m.updateCenterActionLines(cardContentWidth(width)))
	if m.updateCenterRunLive() {
		// The run is the subject while it is on the screen, so it takes the column
		// above the keys: the entry under the cursor is what the reader was looking
		// at, and the cursor is not moving anything now. The card is dropped whole
		// when the column cannot hold it beside the keys -- the keys are how the run
		// is stopped, and a card drawn half is worse than the floor's prose, which
		// still carries the same facts.
		run := m.updateCenterRunCard(width)
		if len(run)+1+len(actions) <= budget {
			return append(append(run, ""), actions...)
		}
		return actions
	}
	entry, ok := m.updateCenterDetailItem()
	if !ok {
		return actions
	}

	var out []string
	room := budget - len(actions) - 1 // the blank row between the two cards
	if room >= cardChromeRows+1 {
		content := cardContentWidth(width)
		rows := max(1, room-cardChromeRows)
		// The tallest reading any package the cursor can rest on draws; the
		// column's own rows are the ceiling.
		height := 1
		for _, item := range m.updateCenterVisibleItems() {
			height = max(height, min(rows, len(m.updateCenterItemCardBody(item, content))))
		}
		body := m.updateCenterItemCardBody(entry, content)
		body = clipLines(body, height, content)
		body = padRows(body, height)
		out = append(out, card(entry.Name, entry.Origin()+" package", width, body)...)
		out = append(out, "")
	}
	return append(out, actions...)
}

// updateCenterItemCardBody is the focused entry's card: its full facts, with the
// name, owner and versions already in the card's title and its table row. The
// detail therefore adds what the row cannot show -- the full installed and
// candidate versions, a hold's reason, the manager's note and the exact command
// the answer was read with -- instead of repeating the row.
func (m Model) updateCenterItemCardBody(item updates.Item, width int) []string {
	installed := item.Current
	if installed == "" {
		installed = "not reported"
	}

	body := []string{}
	for _, line := range wrapText("This entry is one the manager reports as out of date. Nothing changes until a selection is confirmed.", width, 0) {
		body = append(body, MutedStyle.Render(line))
	}
	body = append(body, "")
	body = append(body, iconFactLines([]iconFact{
		{Icon: "\uf02b", Label: "Installed version", Value: installed},
		{Icon: "\uf062", Label: "Candidate version", Value: item.Available},
		{Icon: "\uf05a", Label: "Held", Value: updateCenterHoldValue(item)},
		{Icon: "\uf044", Label: "Note", Value: item.Note},
		{Icon: "\uf120", Label: "Read with", Value: m.updateCenterCommandFor(item.Manager)},
	}, width)...)
	return body
}

// updateCenterHoldValue names the hold an entry is under, or nothing when it is
// not held. It is separate from Note because a hold is a decision and a note is
// the manager's caveat.
func updateCenterHoldValue(item updates.Item) string {
	if !item.Pinned {
		return ""
	}
	if item.PinReason != "" {
		return item.PinReason
	}
	return "held by the manager"
}

// updateCenterActionLines is the Actions card: every key this screen's footer
// names, one row each, with the key in the palette's key tone. It is built from
// the same hints the footer packs, so a key cannot be advertised here and absent
// there.
func (m Model) updateCenterActionLines(width int) []string {
	hints := m.updateCenterHints()
	var lines []string
	seen := map[string]bool{}
	for _, hint := range hints {
		if hint.keys == "" || seen[hint.keys] {
			continue
		}
		seen[hint.keys] = true
		lines = append(lines, AccentKeyStyle.Render(padRight(hint.keys, 9))+" "+InkStyle.Render(truncate(hint.verb, max(1, width-10))))
	}
	return lines
}

// updateCenterFooterStatus is the footer's middle line: the run in flight, what
// the last one did, or how many entries the managers reported. It is empty before
// the first read, so the footer's rule runs whole rather than saying zero.
//
// The run comes first because the wide frame draws no notice band: this line is
// where that shape reports what is happening, and "4 updates are available" on a
// sequence that is installing one of them describes the wrong moment.
func (m Model) updateCenterFooterStatus() string {
	state := m.UpdateCenter
	if state.Applying {
		return m.updateCenterRunFooterLine()
	}
	if len(state.Applied) > 0 && !state.Confirming {
		return m.updateCenterRunResultLine()
	}
	if state.InFlight {
		if state.Pending != "" {
			return "Reading " + string(state.Pending) + "…"
		}
		return "Reading the managers…"
	}
	if state.CheckedAt.IsZero() {
		return ""
	}
	total := len(m.updateCenterItems())
	switch total {
	case 0:
		return "Every manager reports the machine is up to date."
	case 1:
		return "1 update is available."
	default:
		return fmt.Sprintf("%d updates are available.", total)
	}
}
