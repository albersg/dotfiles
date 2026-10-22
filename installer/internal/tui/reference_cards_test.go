package tui

import (
	"strings"
	"testing"
)

// referencePage is one of the pages this pass brought onto the shared grammar,
// named with the rows it draws and the name it wears on its card's chip. The
// guards below walk this table instead of repeating a screen's setup four times.
type referencePage struct {
	name   string
	screen Screen
	build  func(*testing.T) Model
	chip   func(*testing.T, Model) string
	rows   func(*testing.T, Model) []string
	// windowed marks a page whose list is longer than the frame: it draws a window
	// and says so in its header, so only the rows that window holds are required
	// inside the card. The scroll guards hold the rest.
	windowed bool
}

// referencePages is the Learn section, the keymap reference and the LazyVim guide:
// every page the pass converted, with the rows each one promises.
func referencePages() []referencePage {
	// optionLabels is a menu's own rows, without its separators and without the way
	// back: the way back is the footer's promise, not a row the card has to hold.
	optionLabels := func(m Model) []string {
		var labels []string
		for _, option := range m.GetCurrentOptions() {
			if strings.HasPrefix(option, menuSeparatorPrefix) || strings.Contains(option, "Back") {
				continue
			}
			labels = append(labels, option)
		}
		return labels
	}
	constChip := func(name string) func(*testing.T, Model) string {
		return func(*testing.T, Model) string { return name }
	}
	build := func(screen Screen, adjust func(*Model)) func(*testing.T) Model {
		return func(t *testing.T) Model {
			m := installerFrameModel(t, screen)
			if adjust != nil {
				adjust(&m)
			}
			return m
		}
	}

	return []referencePage{
		{
			name: "the Learn hub", screen: ScreenLearnTools,
			build: build(ScreenLearnTools, nil), chip: constChip(learnSectionChosenTitle),
			rows: func(t *testing.T, m Model) []string { return optionLabels(m) },
		},
		{
			name: "the terminal menu", screen: ScreenLearnTerminals,
			build: build(ScreenLearnTerminals, nil), chip: constChip("Learn: Terminal Emulators"),
			rows: func(t *testing.T, m Model) []string { return optionLabels(m) },
		},
		{
			name: "the prompt menu", screen: ScreenLearnPrompts,
			build: build(ScreenLearnPrompts, nil), chip: constChip("Learn: Prompts"),
			rows: func(t *testing.T, m Model) []string { return optionLabels(m) },
		},
		{
			name: "the Vim hub", screen: ScreenVimHub,
			build: build(ScreenVimHub, nil), chip: constChip("Vim & Neovim"),
			rows: func(t *testing.T, m Model) []string { return optionLabels(m) },
		},
		{
			name: "the keymap menu", screen: ScreenKeymaps,
			build: build(ScreenKeymaps, nil), chip: constChip("Neovim Keymaps Reference"),
			// The reference lists more categories than the frame has rows, so the
			// card draws its first window and names the rest in the header.
			windowed: true,
			rows:     func(t *testing.T, m Model) []string { return optionLabels(m)[:3] },
		},
		{
			name: "the LazyVim guide menu", screen: ScreenLearnLazyVim,
			build: build(ScreenLearnLazyVim, nil), chip: constChip("LazyVim Guide"),
			rows: func(t *testing.T, m Model) []string { return optionLabels(m) },
		},
		{
			name: "one tool page", screen: ScreenLearnTerminals,
			build: build(ScreenLearnTerminals, func(m *Model) { m.ViewingTool = "alacritty" }),
			chip:  constChip("Alacritty"),
			rows: func(t *testing.T, m Model) []string {
				info := GetTerminalInfo()["alacritty"]
				return append(append([]string{info.Description}, info.Pros...), info.Cons...)
			},
		},
		{
			name: "one Learn table", screen: ScreenLearnTooling,
			build: build(ScreenLearnTooling, nil), chip: constChip("Shell tooling"),
			// The table is longer than the frame, so the card draws its first window
			// and names how many entries it hid.
			windowed: true,
			rows: func(t *testing.T, m Model) []string {
				var names []string
				for _, row := range shellToolingRows() {
					names = append(names, row.Name)
				}
				return names[:2]
			},
		},
		{
			name: "one keymap category", screen: ScreenKeymapCategory,
			build: build(ScreenKeymapCategory, nil),
			chip:  func(t *testing.T, m Model) string { return m.KeymapCategories[0].Name },
			rows: func(t *testing.T, m Model) []string {
				var keys []string
				for _, km := range m.KeymapCategories[0].Keymaps {
					keys = append(keys, km.Keys)
				}
				return keys
			},
		},
		{
			name: "one LazyVim topic", screen: ScreenLazyVimTopic,
			build: build(ScreenLazyVimTopic, nil),
			chip:  func(t *testing.T, m Model) string { return m.LazyVimTopics[0].Title },
			rows:  func(t *testing.T, m Model) []string { return m.LazyVimTopics[0].Content[:3] },
		},
	}
}

// referenceCardBounds is the card's first and last row in a rendered view, so a
// guard can ask what is inside it rather than what is anywhere on the screen.
func referenceCardBounds(t *testing.T, view string) (top, bottom int) {
	t.Helper()
	top, bottom = -1, -1
	for i, line := range strings.Split(view, "\n") {
		if top < 0 && strings.Contains(line, "╭") {
			top = i
		}
		if strings.Contains(line, "╰") {
			bottom = i
			break
		}
	}
	if top < 0 || bottom < 0 {
		t.Fatalf("the page draws no card:\n%s", view)
	}
	return top, bottom
}

// TestReferencePagesDrawTheFramesCard is the guard for this pass: every page of
// the Learn section, the keymap reference and the LazyVim guide draws the shared
// card -- its own name as the chip on the top border and its own rows inside it --
// instead of the bold title and floating lines it used to draw. The rows are the
// page's own, so the card is a new frame around the same facts.
func TestReferencePagesDrawTheFramesCard(t *testing.T) {
	for _, page := range referencePages() {
		page := page
		t.Run(page.name, func(t *testing.T) {
			m := page.build(t)
			m.Width, m.Height = 80, 24
			view := m.View()
			lines := strings.Split(view, "\n")
			top, bottom := referenceCardBounds(t, view)

			chip := page.chip(t, m)
			if !strings.Contains(lines[top], chip) {
				t.Errorf("the card's chip does not name %q:\n%s", chip, view)
			}
			for _, row := range page.rows(t, m) {
				if !rowAppearsBetween(lines, row, top, bottom) {
					t.Errorf("the row %q is not drawn inside the card:\n%s", row, view)
				}
			}
			if page.windowed && !strings.Contains(view, "Showing") {
				t.Errorf("a windowed page no longer says which rows of its list it shows:\n%s", view)
			}
			if rows := renderedRowCount(view); rows != m.Height {
				t.Errorf("the page renders %d rows in a %d-row frame: the footer is off the screen", rows, m.Height)
			}
		})
	}
}

// TestReferenceCardsKeepTheirKeys pins the footer and the handler promises the
// conversion must not have spent: each page still advertises the keys it answers
// to, and every key it advertises still moves the screen.
func TestReferenceCardsKeepTheirKeys(t *testing.T) {
	cases := []struct {
		name   string
		screen Screen
		want   []string
	}{
		{"the Learn hub", ScreenLearnTools, []string{"[Enter] open", "[Esc] back"}},
		{"the keymap category", ScreenKeymapCategory, []string{"[Enter] open", "[Esc] back"}},
		{"one LazyVim topic", ScreenLazyVimTopic, []string{"[Esc] back"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := installerFrameModel(t, c.screen)
			m.Width, m.Height = 80, 24
			view := m.View()
			for _, want := range c.want {
				if !strings.Contains(view, want) {
					t.Errorf("the footer no longer offers %q:\n%s", want, view)
				}
			}
		})
	}

	// The keymap categories keep their scroll keys: the last binding of the longest
	// category is reachable at the handler's own bound (the guard beside the window
	// arithmetic holds the exact range, this one holds the key path was not lost).
	m := installerFrameModel(t, ScreenKeymapCategory)
	m.SelectedCategory = longKeymapCategory
	m = scrollUntilStill(t, m, func(m Model) int { return m.KeymapScroll })
	last := m.KeymapCategories[longKeymapCategory].Keymaps
	if !strings.Contains(m.View(), last[len(last)-1].Keys) {
		t.Errorf("the last binding of %q is off screen at the keys' own bound", m.KeymapCategories[longKeymapCategory].Name)
	}
}

// TestToolPageKeepsEveryReason pins the two lists the tool pages exist for: the
// card names each list and draws every reason whole, or counts the ones its rows
// could not hold -- what it may not do is cut a reason mid-sentence.
func TestToolPageKeepsEveryReason(t *testing.T) {
	info := GetFontInfo()
	m := installerFrameModel(t, ScreenLearnFont)
	m.Width, m.Height = 80, 24
	view := m.View()

	if !strings.Contains(view, info.Name) {
		t.Errorf("the card does not name the tool:\n%s", view)
	}
	for _, want := range []string{"Pros", "Cons"} {
		if !strings.Contains(view, want) {
			t.Errorf("the page lost its %s list:\n%s", want, view)
		}
	}
	// One reason from each list is the longest the page carries, so it is the one a
	// renderer that wraps nothing would cut.
	lines := strings.Split(view, "\n")
	for _, reason := range []string{info.Pros[0], info.Cons[0]} {
		if !rowAppearsBetween(lines, strings.Join(strings.Fields(reason), " "), 0, len(lines)-1) &&
			!strings.Contains(view, "… and ") {
			t.Errorf("the reason %q is neither drawn nor counted:\n%s", reason, view)
		}
	}
}

// TestReferenceCardsFitTheirBudget pins the window arithmetic to the card: the
// rows a page draws inside its card are the rows its budget allowed, so a card can
// never grow a row past the frame as the cursor or the scroll moves. The scroll
// keys read the same budget, which is why the range the header prints ends on the
// list's last row.
func TestReferenceCardsFitTheirBudget(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {160, 50}} {
		m := installerFrameModel(t, ScreenKeymapCategory)
		m.Width, m.Height = size[0], size[1]
		width, content := m.referenceCardWidth()

		for _, category := range m.KeymapCategories {
			table := keymapCategoryTable(category)
			rows := m.referenceRowBudget(table)
			start, end := referenceWindow(table, content, rows, 0)
			card := referenceTableCard(table, start, end, width, content)
			budget := installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), referencePageHints()))
			if len(card) > budget {
				t.Errorf("%s at %dx%d draws a %d-row card in a %d-row body",
					category.Name, size[0], size[1], len(card), budget)
			}
			// The table's own chrome is the header and the rule, the caption that
			// moved inside is the head, and every other row inside the card is a
			// binding.
			head := len(referenceCardHead(table.Name, table.Description, width))
			if bound := end - start; len(card)-cardChromeRows-2-head != bound {
				t.Errorf("%s at %dx%d draws %d binding rows for a window of %d",
					category.Name, size[0], size[1], len(card)-cardChromeRows-2-head, bound)
			}
		}
	}
}

// TestLazyVimTopicCardFitsItsWindow pins the topic's window to its card: the card
// is the window plus its own three chrome rows, so the keys that scroll the window
// and the rows the screen draws cannot drift apart.
func TestLazyVimTopicCardFitsItsWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {160, 50}} {
		m := installerFrameModel(t, ScreenLazyVimTopic)
		m.Width, m.Height = size[0], size[1]
		for _, topic := range m.LazyVimTopics {
			width, _ := m.referenceCardWidth()
			rows := m.lazyVimTopicRows(topic)
			start, end := offsetWindow(0, rows, len(m.lazyVimTopicLines(topic)))
			card := lazyVimTopicCard(topic, m.lazyVimTopicLines(topic)[start:end], width)
			head := len(referenceCardHead(topic.Title, topic.Description, width))
			if got := len(card) - cardChromeRows - head; got != end-start {
				t.Errorf("%s at %dx%d draws %d content rows for a window of %d",
					topic.Title, size[0], size[1], got, end-start)
			}
			budget := installerBodyRows(m.Height, footerMaxRows)
			if len(card) > budget {
				t.Errorf("%s at %dx%d draws a %d-row card in a %d-row body",
					topic.Title, size[0], size[1], len(card), budget)
			}
		}
	}
}

// TestToolPageKeepsTheOlderBodyOnATinyFrame pins the fallback: the card is a frame
// for the facts, so a frame too narrow or too short for it draws the older body --
// the same lists, without a border that would leave them no room -- rather than a
// card whose rows are cut.
func TestToolPageKeepsTheOlderBodyOnATinyFrame(t *testing.T) {
	m := installerFrameModel(t, ScreenLearnFont)
	m.Width, m.Height = 40, 12
	hints := []installerHint{hintBack, hintQuit}
	if body, ok := m.toolInfoCardBody(GetFontInfo(), hints); ok {
		t.Fatalf("a %dx%d frame claims it can hold the card in %d rows", m.Width, m.Height, len(body))
	}

	view := m.View()
	for _, want := range []string{"Pros", "Cons"} {
		if !strings.Contains(view, want) {
			t.Errorf("the fallback body lost its %s list:\n%s", want, view)
		}
	}
}
