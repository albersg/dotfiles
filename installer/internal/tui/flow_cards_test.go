package tui

import (
	"fmt"
	"strings"
	"testing"
)

// The flow's screens are the backup question, the two restore screens, the
// completion and the failure. These guards hold the shape they were brought onto
// (flow_cards.go): one card from the reference grammar with the screen's own rows
// inside it, and the compact body still under the frames the card cannot hold.

// flowScreenCase is one screen of the flow with the chip its card carries and the
// hints the screen's footer offers.
type flowScreenCase struct {
	name  string
	chip  string
	hints []installerHint
}

// flowScreenCases is the five renderers the slice converted, with the chip each
// one's card carries: three name the screen and two name the block, exactly as the
// header above them names the section.
func flowScreenCases() []flowScreenCase {
	return []flowScreenCase{
		{name: "backup-confirm", chip: "Existing configurations",
			hints: []installerHint{hintUp, hintDown, hintChoose, hintBack}},
		{name: "restore-backup", chip: "Restore from Backup",
			hints: []installerHint{hintUp, hintDown, hintOpen, hintBack}},
		{name: "restore-confirm", chip: "Confirm Restore",
			hints: []installerHint{hintUp, hintDown, hintChoose, hintCancel}},
		{name: "complete", chip: completeSummaryTitle,
			hints: []installerHint{hintExit}},
		{name: "error", chip: errorCardTitle,
			hints: []installerHint{hintRetry, hintQuit}},
	}
}

// flowScreenFrame renders one of the flow's screens as text, with the styling
// taken out so a guard asserts on the words and the box glyphs and not on escapes.
func flowScreenFrame(t *testing.T, name string, width, height int) (Model, string) {
	t.Helper()
	m := installerFrameCase(t, name)
	m.Width, m.Height = width, height
	return m, plainView(m)
}

// flowScreenAnswers is the answers a screen of the flow offers, without the
// separator that opens the last group. The completion and the failure offer none:
// their footer carries the only keys they have.
func flowScreenAnswers(m Model) []string {
	var answers []string
	for _, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, menuSeparatorPrefix) {
			continue
		}
		answers = append(answers, option)
	}
	return answers
}

// TestFlowScreensDrawTheReferenceCard pins the shape: the chip set into the card's
// top border, the rule under it, every row the key handler reads inside the card,
// and the row under the cursor carrying the grammar's marker. The screens used to
// draw a branded label, a guttered block of prose and a separate box for the
// answers, which is three objects where the rest of the program draws one.
func TestFlowScreensDrawTheReferenceCard(t *testing.T) {
	for _, tc := range flowScreenCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m, view := flowScreenFrame(t, tc.name, 80, 24)

			lines := strings.Split(view, "\n")
			top, bottom := referenceCardBounds(t, view)
			// The chip is the first thing the top border says: the card names itself
			// there, and the header above it names the section.
			if !strings.HasPrefix(strings.TrimSpace(lines[top]), "╭─") || !strings.Contains(lines[top], tc.chip) {
				t.Fatalf("%s does not set %q into its card's top border:\n%s", tc.name, tc.chip, view)
			}

			// The rule under the chip is the grammar's own row: read through the
			// borders, it holds nothing.
			if rule := strings.Trim(strings.TrimSpace(lines[top+1]), "│├┤─"); rule != "" {
				t.Fatalf("%s does not draw the rule under its chip, it draws %q:\n%s",
					tc.name, rule, view)
			}

			answers := flowScreenAnswers(m)
			for _, option := range answers {
				if !rowAppearsBetween(lines, plainOptionName(option), top, bottom) {
					t.Fatalf("%s does not draw the answer %q inside its card:\n%s",
						tc.name, plainOptionName(option), view)
				}
			}
			if len(answers) == 0 {
				return
			}

			// The cursor's row is the grammar's bar, marked in its gutter: the state
			// is one filled row and not an indent somewhere else.
			if !rowAppearsBetween(lines, grammarMarker, top, bottom) {
				t.Fatalf("%s marks no row with %q:\n%s", tc.name, grammarMarker, view)
			}
			selected := m.GetCurrentOptions()[max(0, m.Cursor)]
			if !rowAppearsBetween(lines, grammarMarker+" "+plainOptionName(selected), top, bottom) {
				t.Fatalf("%s does not mark the row under the cursor (%q):\n%s",
					tc.name, plainOptionName(selected), view)
			}
		})
	}
}

// TestFlowScreensKeepTheirOwnData pins the other half of the slice: the card is a
// new shape for the same material, so every fact these screens stated before is
// still on them, in the screen's own words, at the floor and at a wide frame.
func TestFlowScreensKeepTheirOwnData(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"the floor", 80, 24}, {"a wide frame", 140, 44}} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			// The backup question names every configuration it would overwrite and the
			// reason the backup exists.
			m, view := flowScreenFrame(t, "backup-confirm", size.width, size.height)
			for _, config := range m.ExistingConfigs {
				if !strings.Contains(view, config) {
					t.Errorf("the backup screen does not name the configuration %q:\n%s", config, view)
				}
			}
			if !strings.Contains(view, backupConfigNote) {
				t.Errorf("the backup screen lost its note:\n%s", view)
			}

			// The restore list names each backup with the file count it carries, and
			// the way back stays the last row of the same card.
			m, view = flowScreenFrame(t, "restore-backup", size.width, size.height)
			for _, backup := range m.AvailableBackups {
				label := fmt.Sprintf("%s (%d items)",
					backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
				if !strings.Contains(view, label) {
					t.Errorf("the restore list does not name %q:\n%s", label, view)
				}
			}
			if !strings.Contains(view, restoreHistoryBack) {
				t.Errorf("the restore list lost its way back:\n%s", view)
			}
			if !strings.Contains(view, restoreHistoryCaption) {
				t.Errorf("the restore list lost its own sentence:\n%s", view)
			}

			// The confirmation names which backup it is (with the backup's own
			// timestamp), every file it carries, the warning and the three answers.
			m, view = flowScreenFrame(t, "restore-confirm", size.width, size.height)
			backup := m.AvailableBackups[m.SelectedBackup]
			stamp := backup.Timestamp.Format("2006-01-02 15:04:05")
			if !strings.Contains(view, restoreConfirmCaption+stamp) {
				t.Errorf("the confirmation does not name the backup's timestamp %q:\n%s", stamp, view)
			}
			for _, file := range backup.Files {
				if !strings.Contains(view, file) {
					t.Errorf("the confirmation does not name the file %q:\n%s", file, view)
				}
			}
			if !strings.Contains(view, restoreConfirmWarning) {
				t.Errorf("the confirmation lost its warning:\n%s", view)
			}

			// The completion states every fact the run recorded, in the wizard's own
			// vocabulary, and the command that puts the shell in front of the reader.
			m, view = flowScreenFrame(t, "complete", size.width, size.height)
			facts := m.completionFacts()
			if len(facts) == 0 {
				t.Fatal("the completion screen states no fact, so the guard measures nothing")
			}
			for _, fact := range facts {
				if !strings.Contains(view, fact.Label) {
					t.Errorf("the completion summary does not label %q:\n%s", fact.Label, view)
				}
				if !strings.Contains(view, fact.Value) {
					t.Errorf("the completion summary lost the value %q for %q:\n%s", fact.Value, fact.Label, view)
				}
				if strings.Contains(view, fact.Label+":") {
					t.Errorf("the completion spells %q as a sentence instead of a fact row:\n%s", fact.Label+":", view)
				}
			}
			if !strings.Contains(view, fmt.Sprintf("exec %s", m.completionShellCommand())) {
				t.Errorf("the completion does not name the command that starts the shell:\n%s", view)
			}

			// The failure states its message, the directory the run left, the command
			// that puts it back, what no command undoes, and the log it kept.
			m, view = flowScreenFrame(t, "error", size.width, size.height)
			lines := strings.Split(view, "\n")
			top, bottom := referenceCardBounds(t, view)
			inside := cardBodyText(lines, top, bottom)
			wants := append([]string{m.ErrorMsg}, wayBackAfterAFailure(m.BackupDir)...)
			for _, want := range wants {
				if !strings.Contains(inside, want) {
					t.Errorf("the failure screen does not say %q inside its card:\n%s", want, view)
				}
			}
			for _, line := range m.LogLines {
				if !strings.Contains(inside, line) {
					t.Errorf("the failure screen dropped the log line %q:\n%s", line, view)
				}
			}
		})
	}
}

// TestFlowScreensGateOnTheFrameAndNothingElse pins the two-sided bargain: the
// card is a shape for a frame that can hold it, and the frame that cannot is drawn
// by the compact renderer the screen always had rather than by a card with no room
// for its own rows. The gate reads the frame -- a content width its rows are
// readable in and a row budget that holds the composed card -- and nothing about
// the cursor, so a screen cannot change shape while it is walked.
func TestFlowScreensGateOnTheFrameAndNothingElse(t *testing.T) {
	accepts := func(name string, m Model, hints []installerHint) bool {
		switch name {
		case "backup-confirm":
			_, ok := m.backupConfirmCardBody(layoutFor(m), hints)
			return ok
		case "restore-backup":
			_, _, _, ok := m.restoreHistoryCardBody(layoutFor(m), hints)
			return ok
		case "restore-confirm":
			_, ok := m.restoreConfirmCardBody(m.AvailableBackups[m.SelectedBackup], layoutFor(m), hints)
			return ok
		case "complete":
			_, ok := m.completeCardBody(layoutFor(m), hints)
			return ok
		case "error":
			_, ok := m.errorCardBody(layoutFor(m), hints)
			return ok
		}
		return false
	}

	for _, tc := range flowScreenCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := installerFrameCase(t, tc.name)

			// The floor the program supports draws the card.
			m.Width, m.Height = 80, 24
			if !accepts(tc.name, m, tc.hints) {
				t.Fatalf("%s does not draw its card at the 80x24 floor", tc.name)
			}

			// Thirty columns leave a card content narrower than one readable row, so
			// the screen falls back to the compact body it always had -- and still
			// draws, which is what makes the fallback a shape and not a hole.
			m.Width, m.Height = 30, 24
			if accepts(tc.name, m, tc.hints) {
				t.Fatalf("%s claims a card fits a frame with no room for its own rows", tc.name)
			}
			view := plainView(m)
			if strings.TrimSpace(view) == "" {
				t.Fatalf("%s renders nothing at all on a narrow frame", tc.name)
			}
		})
	}
}
