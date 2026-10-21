package tui

import (
	"strings"
	"testing"
)

// ============================================================================
// The pages the mockup does not show
// ============================================================================
//
// The mockup draws six screens; these are the rest of the program, and the pass
// that brought them over is measured here. Four things were unified and each has
// a guard: the dead ends wear the frame instead of two bare lines, the wizard's
// own questions draw the same card every other menu draws, the completion
// summary draws the frame's fact rows, and a read-only table drops a column
// before it cuts a sentence.

// pickerScreens are the wizard's question screens: the six pickers and the
// terminal warning, which is the same shape with a different question.
var pickerScreens = []string{
	"os-select", "terminal-select", "terminal-select-wsl",
	"font-select", "shell-select", "wm-select", "nvim-select", "ghostty-warning",
}

// TestPickerQuestionsDrawTheFramesCard pins the shape the pickers share with
// every other menu: the question as the page's title, the model's description
// under it, and the answers inside the card. They used to draw a bare list, so
// the two wizard questions that fill the pickers' rows and the hubs one screen
// later were two different shapes for the same list of choices.
func TestPickerQuestionsDrawTheFramesCard(t *testing.T) {
	for _, name := range pickerScreens {
		name := name
		t.Run(name, func(t *testing.T) {
			m := installerFrameCase(t, name)
			m.Width, m.Height = 80, 24
			view := m.View()

			if !strings.Contains(view, "╭") || !strings.Contains(view, "╮") ||
				!strings.Contains(view, "╰") || !strings.Contains(view, "╯") {
				t.Fatalf("%s does not draw the frame's card around its answers:\n%s", name, view)
			}

			// Every answer the model offers is on the screen and inside the card:
			// a card that hid one of them behind its own bottom border would be a
			// list the cursor could walk into but not read.
			lines := strings.Split(view, "\n")
			top, bottom := -1, -1
			for i, line := range lines {
				if top < 0 && strings.Contains(line, "╭") {
					top = i
				}
				if strings.Contains(line, "╰") {
					bottom = i
				}
			}
			if top < 0 || bottom < top {
				t.Fatalf("%s has no card to measure its rows against:\n%s", name, view)
			}
			for _, option := range m.GetCurrentOptions() {
				if strings.HasPrefix(option, menuSeparatorPrefix) {
					continue
				}
				if !rowAppearsBetween(lines, plainOptionName(option), top, bottom) {
					t.Fatalf("%s does not draw the row %q inside its card (rows %d-%d):\n%s",
						name, plainOptionName(option), top, bottom, view)
				}
			}
		})
	}
}

// TestPickerQuestionsKeepTheirKeysAndTheirWayOut pins the footer the wizard's
// questions promise: the arrows, Enter for the answer, and the way back. The card
// changed the body and nothing else.
func TestPickerQuestionsKeepTheirKeysAndTheirWays(t *testing.T) {
	for _, name := range pickerScreens {
		name := name
		t.Run(name, func(t *testing.T) {
			m := installerFrameCase(t, name)
			m.Width, m.Height = 80, 24
			view := m.View()

			for _, want := range []string{"↑/k", "↓/j", "[Enter]", "[Esc]"} {
				if !strings.Contains(view, want) {
					t.Fatalf("%s no longer names %q in its legend:\n%s", name, want, view)
				}
			}
			if !strings.Contains(view, statusDotGlyph+" "+statusReady) {
				t.Fatalf("%s does not carry the frame's status bar:\n%s", name, view)
			}
		})
	}
}

// rowAppearsBetween reports whether a row carrying label is drawn between the
// two lines. It reads the rendered screen rather than the body builder, so a
// row the card clipped away cannot pass.
func rowAppearsBetween(lines []string, label string, top, bottom int) bool {
	for i := top; i >= 0 && i <= bottom && i < len(lines); i++ {
		if strings.Contains(lines[i], label) {
			return true
		}
	}
	return false
}

// TestDeadEndsWearTheFrame pins the one shape the program was missing: a screen
// with nothing to show used to be two bare lines -- no header chip, no state and
// no legend -- which is what the frame exists to prevent. Every dead end now
// carries the chrome, and the way out is the legend's, said once.
func TestDeadEndsWearTheFrame(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) Model
		want  string
	}{
		{"a tool with no page", func(t *testing.T) Model {
			m := installerFrameCase(t, "learn-terminals")
			m.ViewingTool = "no-such-terminal"
			return m
		}, "Tool not found"},
		{"a topic with no page", func(t *testing.T) Model {
			m := installerFrameCase(t, "lazyvim-topic")
			m.ViewingTool = "no-such-topic"
			return m
		}, "LazyVim topic"},
		{"a restore with no backup", func(t *testing.T) Model {
			m := installerFrameCase(t, "restore-confirm")
			m.AvailableBackups = nil
			m.SelectedBackup = 0
			return m
		}, "No backup selected"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 80, 24
			view := m.View()

			// The chrome: the header's chip, its rule, the state and the legend.
			if !strings.Contains(view, m.headerName()) {
				t.Errorf("the dead end does not name its section in the header:\n%s", view)
			}
			if !strings.Contains(view, statusDotGlyph+" "+statusReady) {
				t.Errorf("the dead end carries no state bar:\n%s", view)
			}
			if !strings.Contains(view, hintBack.keys) || !strings.Contains(view, hintBack.verb) {
				t.Errorf("the dead end's legend does not carry the way out:\n%s", view)
			}

			// The way out is said once: the legend names the key, the sentence says
			// where it goes.
			if strings.Contains(view, "Press [Esc] to") {
				t.Errorf("the dead end still spells the key its legend already names:\n%s", view)
			}
		})
	}
}

// TestDeadEndWayOutKeepsTheDestination pins the one rewrite the framed dead end
// makes: the sentence loses the key and keeps everything else, so no dead end
// loses the place it was telling the reader to go back to.
func TestDeadEndWayOutKeepsTheDestination(t *testing.T) {
	cases := map[string]string{
		"Press [Esc] to go back.":                  "Go back.",
		"Press [Esc] to pick another category.":    "Pick another category.",
		"Press [Esc] to return to the guide.":      "Return to the guide.",
		"Press [Esc] to return to the trainer.":    "Return to the trainer.",
		"Press [Esc] to choose another terminal.":  "Choose another terminal.",
		"Press [Esc] to leave the report open.":    "Leave the report open.",
		"Press [Esc] to return to the utilities.":  "Return to the utilities.",
		"Press [Esc] to go back to the main menu.": "Go back to the main menu.",
	}
	for action, want := range cases {
		if got := deadEndWayOut(action); got != want {
			t.Errorf("deadEndWayOut(%q) = %q, want %q", action, got, want)
		}
	}
	if got := deadEndWayOut(""); got != "" {
		t.Errorf("an empty action produced %q, want nothing", got)
	}
}

// TestCompletionSummaryDrawsTheFramesFactRows pins the completion screen's
// summary as the frame's fact rows: the label column at one measure, the value
// beside it, and every row reading label-then-value instead of one sentence. A
// reader comparing two machines can line the values up, which is what the
// utilities panel and the review already allow.
func TestCompletionSummaryDrawsTheFramesFactRows(t *testing.T) {
	m := installerFrameCase(t, "complete")
	m.Width, m.Height = 80, 24
	m.Choices = UserChoices{
		OS: "linux", Terminal: "ghostty", Shell: "fish",
		WindowMgr: "tmux", InstallFont: true, InstallNvim: true,
	}
	view := m.View()

	labels := []string{"OS", "Terminal", "Shell", "Multiplexer", "Font", "Editor"}
	values := []string{
		m.completionChoiceLabel(ScreenOSSelect, m.Choices.OS),
		m.completionChoiceLabel(ScreenTerminalSelect, m.Choices.Terminal),
		m.completionChoiceLabel(ScreenShellSelect, m.Choices.Shell),
		m.completionChoiceLabel(ScreenWMSelect, m.Choices.WindowMgr),
		m.fontSummaryValue(),
		"Neovim with dotfiles config",
	}
	for i, label := range labels {
		if !strings.Contains(view, label) {
			t.Fatalf("the summary does not name %q:\n%s", label, view)
		}
		if !strings.Contains(view, values[i]) {
			t.Fatalf("the summary lost the value %q for %q:\n%s", values[i], label, view)
		}
		// The value starts after the label on the same row: the summary is a fact
		// row, not a sentence.
		if strings.Contains(view, label+":") {
			t.Fatalf("the summary still spells %q as a sentence:\n%s", label+":", view)
		}
	}
}

// TestBackupAndRestoreDrawTheirAnswersInTheCard pins the three backup screens'
// shape: the answers a person presses are one card, with the content above it and
// the warning inside the flow. The card is what tells the eye which rows are
// rows, and every screen the pass has reached uses it.
func TestBackupAndRestoreDrawTheirAnswersInTheCard(t *testing.T) {
	for _, name := range []string{"backup-confirm", "restore-backup", "restore-confirm"} {
		name := name
		t.Run(name, func(t *testing.T) {
			m := installerFrameCase(t, name)
			m.Width, m.Height = 80, 24
			view := m.View()

			if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
				t.Fatalf("%s draws no card:\n%s", name, view)
			}
			lines := strings.Split(view, "\n")
			top, bottom := -1, -1
			for i, line := range lines {
				if top < 0 && strings.Contains(line, "╭") {
					top = i
				}
				if strings.Contains(line, "╰") {
					bottom = i
				}
			}
			for _, option := range m.GetCurrentOptions() {
				if strings.HasPrefix(option, menuSeparatorPrefix) {
					continue
				}
				if !rowAppearsBetween(lines, plainOptionName(option), top, bottom) {
					t.Fatalf("%s does not draw the answer %q inside its card:\n%s",
						name, plainOptionName(option), view)
				}
			}
		})
	}
}

// TestLearnTablesDrawEveryEntryWhole pins the shape the three learn tables take:
// each entry is the frame's two-line row -- the name and its kind on the first
// row, the sentence about it under them, whole -- and the page windows itself
// rather than cutting a sentence. The tables used to hold three fixed columns,
// which at the 80-column floor left the Kind column three letters and a cut
// marker and every description without its tail.
func TestLearnTablesDrawEveryEntryWhole(t *testing.T) {
	tables := map[string]referenceTable{
		"the shell tooling page": shellToolingTable(),
		"the plugin page":        pluginSetsTable(),
		"the toolset page":       machineToolsetTable(),
	}
	m := installerFrameModel(t, ScreenLearnTooling)
	for name, table := range tables {
		if !table.TwoLine {
			t.Fatalf("%s no longer asks for the two-line shape", name)
		}
		for _, width := range []int{60, 76, 156} {
			for _, row := range table.Rows {
				at := referenceRowIndex(table, row)
				rows := m.referenceEntryRows(table, at, at+1, width)
				flat := strings.Join(strings.Fields(strings.Join(rows, " ")), " ")
				if !strings.Contains(flat, strings.Join(strings.Fields(row.What), " ")) {
					t.Fatalf("%s at %d columns does not draw %q whole, it draws %q", name, width, row.What, flat)
				}
				if !strings.Contains(flat, row.Name) {
					t.Fatalf("%s at %d columns lost the name %q (%q)", name, width, row.Name, flat)
				}
			}
		}
	}
}

// referenceRowIndex finds an entry's position in its own table, so a guard can render one
// entry without depending on where the window happens to put it.
func referenceRowIndex(table referenceTable, row referenceRow) int {
	for i, candidate := range table.Rows {
		if candidate == row {
			return i
		}
	}
	return 0
}

// TestLearnTablesWindowInTheRowsTheyDraw pins the scroll arithmetic: the window
// the page draws and the offset its keys allow are one number, the last entry is
// reachable, and a page of two-line entries is not scrolled by a count of entries
// it does not spend.
func TestLearnTablesWindowInTheRowsTheyDraw(t *testing.T) {
	table := shellToolingTable()
	const width = 76
	budget := 14 // the 80x24 body, with the two rows the header and rule would take

	start, end := referenceWindow(table, width, budget, 0)
	if start != 0 || end == 0 {
		t.Fatalf("the first window is %d-%d, want it to start at the top and show entries", start, end)
	}
	spent := 0
	for i := start; i < end; i++ {
		spent += referenceEntryCost(table, table.Rows[i], width)
	}
	if spent > budget {
		t.Fatalf("the window drew %d rows in a budget of %d", spent, budget)
	}

	last := referenceMaxScroll(table, width, budget)
	if last <= 0 {
		t.Fatalf("the page cannot be scrolled at all")
	}
	if _, lastEnd := referenceWindow(table, width, budget, last); lastEnd != len(table.Rows) {
		t.Fatalf("the last offset %d shows up to %d of %d entries", last, lastEnd, len(table.Rows))
	}
}

// TestToolInfoWrapsItsReasonsInsteadOfCuttingThem pins the pros and cons: an
// entry is drawn whole, on as many rows as it needs, and the rows the budget
// could not show are counted in one note. The bullets used to be cut with the
// frame's marker, which on the longest ones left a reason unreadable.
func TestToolInfoWrapsItsReasonsInsteadOfCuttingThem(t *testing.T) {
	m := installerFrameModel(t, ScreenLearnFont)
	m.Width, m.Height = 80, 24
	view := m.View()

	lines := strings.Split(view, "\n")
	for _, line := range lines {
		plain := strings.TrimSpace(plainRow(line))
		if !strings.HasPrefix(plain, "│") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(plain), "│"))
		if strings.HasSuffix(body, cutMarker) {
			t.Fatalf("a guttered reason is still cut mid-sentence:\n%s", view)
		}
	}
	if !strings.Contains(view, "Pros") || !strings.Contains(view, "Cons") {
		t.Fatalf("the tool page lost one of its two lists:\n%s", view)
	}
}
