package tui

import (
	"slices"
	"strings"
	"testing"
)

// ============================================================================
// THE SECTION SCREENS' DETAIL IN THE GRAMMAR — the guards
// ============================================================================
//
// The SSH screens and the WSL screens state their detail as facts now
// (facts_cards.go): the wide frame draws iconFactLines in the card beside the list
// and the compact body is factProse of the same list. These guards hold what that
// costs: every fact the model holds is still on the screen, the two shapes read one
// list rather than two copies, a block (a preview, a key, a tool's output) keeps
// its own lines, and the 80x24 floor keeps the body it always drew.

// convertedScreen is one screen whose detail is facts, with the fixture that fills
// it and the sentence that proves its own data path is the one being drawn.
type convertedScreen struct {
	name     string
	build    func(*testing.T) Model
	sentence string
}

func convertedScreens(t *testing.T) []convertedScreen {
	t.Helper()
	return []convertedScreen{
		{"ssh-manager", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHManager) }, "1 of them, by this tool"},
		{"ssh-host", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHHost) }, "Declared in "},
		{"ssh-form", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHForm) }, "An alias is required"},
		{"ssh-keys", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHKeys) }, "A private key is never read by this screen"},
		{"ssh-agent", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHAgent) }, "holds"},
		{"ssh-diagnostics", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHDiagnostics) }, "to look at"},
		{"ssh-test", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHTest) }, "Host keys"},
		{"wsl-resources", wslResourcesFrameCase, "three quarters of the host's RAM"},
		{"wsl-advisor", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLAdvisor) }, "8.0 GiB total"},
		{"wsl-benchmark", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLBenchmark) }, "measured from top to bottom"},
		{"wsl-migration", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLMigration) }, "copy /windir"},
	}
}

// detailCardText is the detail card's own body as one line of words: the groups
// the wide frame draws, flattened the way cardText flattens a frame. It measures the
// card rather than the composed frame because the frame's two columns interleave
// when it is read this way, and because the card cuts the rows its column cannot
// hold -- which is a limit of the frame, not of the facts.
func detailCardText(t *testing.T, m Model) string {
	t.Helper()
	groups, ok := m.sectionScreenGroups(cardContentWidth(120))
	if !ok {
		t.Fatalf("%v is not in the facts registry", m.Screen)
	}
	var lines []string
	for _, group := range groups {
		lines = append(lines, group.Lines...)
	}
	return strings.Join(strings.Fields(ansiEscape.ReplaceAllString(strings.Join(lines, " "), "")), " ")
}

// TestEveryConvertedScreenKeepsEveryFactInBothShapes is the data guard: the screen
// is in the facts registry, every value it derives reaches the compact body whole,
// and the card is drawn from the same list -- every fact's label and value is in the
// card's body, and the frame carries the card with the section's own chip.
func TestEveryConvertedScreenKeepsEveryFactInBothShapes(t *testing.T) {
	for _, c := range convertedScreens(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			facts, ok := m.sectionScreenFacts()
			if !ok {
				t.Fatalf("%s is not in the facts registry", c.name)
			}
			if len(facts) == 0 {
				t.Fatalf("%s holds no fact", c.name)
			}

			card := detailCardText(t, m)
			for _, fact := range facts {
				if label := strings.TrimSpace(fact.Label); label != "" && !strings.Contains(card, label) {
					t.Errorf("the card of %s does not name the fact %q:\n%s", c.name, label, card)
				}
				for _, line := range strings.Split(fact.Value, "\n") {
					line = squash(line)
					if line != "" && !strings.Contains(card, line) {
						t.Errorf("the card of %s drops the value %q:\n%s", c.name, line, card)
					}
				}
			}

			body := squash(strings.Join(factProse(facts), " "))
			for _, fact := range facts {
				for _, line := range strings.Split(fact.Value, "\n") {
					line = squash(line)
					if line != "" && !strings.Contains(body, line) {
						t.Errorf("the compact body of %s drops the fact %q:\n%s", c.name, line, body)
					}
				}
			}
			if !strings.Contains(body, c.sentence) {
				t.Errorf("%s does not state its own sentence %q in the compact body:\n%s", c.name, c.sentence, body)
			}

			wide := m
			wide.Width, wide.Height = 160, 50
			wide.Animating = false
			plain := cardText(wide.View())
			if !strings.Contains(plain, m.sectionPaneTitle()) {
				t.Errorf("%s does not draw the detail card at 160x50:\n%s", c.name, plain)
			}
		})
	}
}

// factValueRowMeasure is the most a fact's value may be and still read as a row: two
// lines of the value column at the narrowest frame that draws the two-column card. A
// value longer than this is a paragraph the card is drawing under a label, which is
// the shape this pass exists to remove.
const factValueRowMeasure = 140

// factArgumentLabels are the one kind of value that stays long: a finding's own
// argument -- why it matters, what acting is worth, what to do. Those sentences are
// the model's own measured claims (wsladvisor.go writes them), and cutting one to fit
// a row would either drop a fact or invent a shorter one. They are still rows -- one
// label and one value each, wrapped in the card's column -- which is what the pass was
// for; what they are not is a row that could be split further without losing the
// argument the finding exists to make.
var factArgumentLabels = map[string]bool{
	"Why it matters": true,
	"Benefit":        true,
	"What to do":     true,
}

// TestEveryConvertedRowIsALabelAndAShortValue is the wall-of-text guard: a fact row
// carries the label that names it, and its value reads as a row -- two lines of the
// value column at the narrowest two-column frame -- everywhere except a finding's own
// argument. A block value (a preview's lines, a public key, a tool's output) keeps its
// own lines and is exempt from the measure, because wrapping it into a sentence would
// join lines that mean different things.
func TestEveryConvertedRowIsALabelAndAShortValue(t *testing.T) {
	for _, c := range convertedScreens(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			areas, ok := c.build(t).sectionScreenAreas()
			if !ok {
				t.Fatalf("%s is not in the facts registry", c.name)
			}
			for _, area := range areas {
				for _, fact := range area.Facts {
					if strings.TrimSpace(fact.Value) == "" {
						continue
					}
					if strings.TrimSpace(fact.Label) == "" {
						t.Errorf("%s draws %q without the label that names it", c.name, fact.Value)
						continue
					}
					if strings.Contains(fact.Value, "\n") || factArgumentLabels[fact.Label] {
						continue
					}
					if n := len([]rune(fact.Value)); n > factValueRowMeasure {
						t.Errorf("%s draws a paragraph under %q (%d runes, want <= %d): the card is not a page",
							c.name, fact.Label, n, factValueRowMeasure)
					}
				}
			}
		})
	}
}

// TestTheDetailCardDividesTheConvertedScreensByArea pins the dividers: a screen whose
// detail has more than one area draws each area under its own name, so a long detail
// is a set of short sections rather than one column of facts. The first area rides the
// card's own rule under the chip, which is the card's grammar and not a missing name.
func TestTheDetailCardDividesTheConvertedScreensByArea(t *testing.T) {
	m := wslAdvisorFrameModel(t, ScreenWSLAdvisor)
	areas, ok := m.sectionScreenAreas()
	if !ok || len(areas) < 2 {
		t.Fatalf("the advisor holds %d areas, want the instance, storage, resolver and the rest", len(areas))
	}
	groups, ok := m.sectionScreenGroups(cardContentWidth(120))
	if !ok {
		t.Fatal("the advisor draws no groups")
	}
	for _, want := range []string{"Instance", "Storage", "Resolver", "Windows", "This report", "Finding"} {
		found := false
		for _, group := range groups {
			if group.Title == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the advisor's card draws no divider named %q: %v", want, groupTitles(groups))
		}
	}
}

// groupTitles names the dividers a card draws, for a failure message.
func groupTitles(groups []cardGroup) []string {
	titles := make([]string, 0, len(groups))
	for _, group := range groups {
		titles = append(titles, group.Title)
	}
	return titles
}

// TestDiskCleanerListIsACardMenuAndItsPressesAreTheFooters pins the disk cleaner's
// two shapes after the redesign: the list is headings and two-line findings, and every
// press that is not a finding's switch is named in the footer and drawn nowhere in the
// card. It is the guard the user's report asked for on this screen.
func TestDiskCleanerListIsACardMenuAndItsPressesAreTheFooters(t *testing.T) {
	m := diskCleanerFrameCase(t)
	m.Width, m.Height = 160, 50

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"npm — 1 finding",             // a heading, drawn as the card's divider
		"apt — 1 finding",             // and the next one
		"The npm cache",               // a finding's title
		"900.0 MiB · safe to rebuild", // and what it holds beside what removing it costs
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the disk cleaner's list does not draw %q:\n%s", want, view)
		}
	}
	for _, gone := range []string{
		"[s] Scan again",
		"[s] Scan for development caches",
		"[d] Add the project directory",
		"[p] Preview the",
		"← Back",
	} {
		if strings.Contains(view, gone) {
			t.Errorf("the list still draws the press %q as a row:\n%s", gone, view)
		}
	}
	// The presses are the footer's, all of them, in the order the handler reads them.
	for _, press := range []string{"[s] scan", "[p] preview", "[d] add the project directory", "[Space] select or drop", "[Esc] back"} {
		if !strings.Contains(view, press) {
			t.Errorf("the footer does not name %q:\n%s", press, view)
		}
	}
}

// TestTheFactsAreTheModelsOwnValues is the no-second-copy guard: a datum changed in
// the model reaches both shapes, and the value it replaced is gone from both.
func TestTheFactsAreTheModelsOwnValues(t *testing.T) {
	manager := sshTestModel(t, ScreenSSHManager)
	manager.SSH.HostsErr = "the Include on line 3 matched nothing"
	if body := strings.Join(manager.sshManagerDescription(), " "); !strings.Contains(body, "matched nothing") {
		t.Errorf("the compact body does not draw the manager's own error:\n%s", body)
	}
	manager.Width, manager.Height = 160, 50
	manager.Animating = false
	plain := cardText(manager.View())
	if !strings.Contains(plain, "matched nothing") {
		t.Errorf("the card does not draw the manager's own error:\n%s", plain)
	}
	if strings.Contains(plain, "managed by this tool") {
		t.Errorf("the card draws the host count the model no longer holds:\n%s", plain)
	}

	resources := wslResourcesFrameCase(t)
	resources.WSLState.Plan.MemoryMB = 4321
	if body := strings.Join(resources.wslResourceDescription(), " "); !strings.Contains(body, "Memory: 4321 MB") {
		t.Errorf("the compact body does not draw the planned memory the model holds:\n%s", body)
	}
	resources.Width, resources.Height = 160, 50
	resources.Animating = false
	plain = cardText(resources.View())
	if !strings.Contains(plain, "4321 MB") {
		t.Errorf("the card does not draw the planned memory the model holds:\n%s", plain)
	}
}

// TestConvertedScreensFallBackToTheCompactBodyAtTheFloor is the floor guard: 80x24
// draws no detail card, and the body it does draw is the facts' prose.
func TestConvertedScreensFallBackToTheCompactBodyAtTheFloor(t *testing.T) {
	for _, c := range convertedScreens(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 80, 24
			m.Animating = false

			plain := cardText(m.View())
			if strings.Contains(plain, m.sectionPaneTitle()) {
				t.Fatalf("%s draws the detail card at the 80x24 floor:\n%s", c.name, plain)
			}
			// The floor keeps the compact body: its facts, cut to the rows the frame
			// has, with the marker that says how many it could not show.
			facts, _ := m.sectionScreenFacts()
			probe := squash(strings.Split(facts[0].Value, "\n")[0])
			probe = probe[:min(len(probe), 24)]
			if !strings.Contains(plain, probe) {
				t.Errorf("%s does not open the compact body with %q at the floor:\n%s", c.name, probe, plain)
			}
		})
	}
}

// TestConvertedScreensFitEveryFrameTheyAreDrawnIn measures the shape the way the
// terminal does: no row wider than the terminal and no more rows than it has.
func TestConvertedScreensFitEveryFrameTheyAreDrawnIn(t *testing.T) {
	for _, c := range convertedScreens(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for _, size := range measuredTerminalSizes {
				m := c.build(t)
				m.Width, m.Height = size.width, size.height
				m.Animating = true
				assertScreenFitsTerminal(t, c.name, size.width, size.height, m.View())
			}
		})
	}
}

// TestFactProseReadsOneSentenceOneWay pins the rule that lets the screens keep the
// sentences they were written with: a value that is already a sentence stands alone,
// a bare value is introduced by its label, and a block prints its label as the
// heading and its lines in their own order.
func TestFactProseReadsOneSentenceOneWay(t *testing.T) {
	facts := []iconFact{
		{Label: "Files", Value: "The client reads ~/.ssh/config."},
		{Label: "Host name", Value: "web.example.com  (the client's answer)"},
		{Label: "Fix", Value: "chmod 600 ~/.ssh/id_ed25519"},
		{Label: "Preview", Value: "  Host web\n+ Host staging"},
	}
	want := []string{
		"The client reads ~/.ssh/config.",
		"Host name: web.example.com  (the client's answer).",
		"Fix: chmod 600 ~/.ssh/id_ed25519.",
		"Preview:",
		"  Host web",
		"+ Host staging",
	}
	got := factProse(facts)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("factProse read the facts as:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if factClause("the mount took 6.4 s.") != "the mount took 6.4 s" {
		t.Errorf("factClause kept the full stop the label already ends")
	}
	if !factEndsSentence("No entry is selected.") || factEndsSentence("web.example.com") {
		t.Errorf("factEndsSentence does not tell a sentence from a value")
	}
}

// TestFactLinesKeepABlocksOwnRows holds the card's half of the same rule: a value
// whose lines mean different things -- a preview, a key, a tool's output -- is drawn
// as a block under its label, never wrapped into one sentence.
func TestFactLinesKeepABlocksOwnRows(t *testing.T) {
	lines := factLines([]iconFact{
		{Label: "File", Value: "/home/ana/.ssh/config"},
		{Label: "Preview", Value: "-  HostName old\n+  HostName new"},
	}, 60)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"File", "/home/ana/.ssh/config", "Preview", "-  HostName old", "+  HostName new"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the card does not draw %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "-  HostName old +  HostName new") {
		t.Errorf("the block was joined into one sentence:\n%s", joined)
	}
	// The label is the block's heading and its lines follow it at the value column,
	// so a diff keeps the indent that says which line it is.
	block := strings.Split(joined, "\n")
	labelAt := slices.IndexFunc(block, func(line string) bool { return strings.Contains(line, "Preview") })
	if labelAt < 0 || labelAt+2 >= len(block) {
		t.Fatalf("the block does not open with its label:\n%s", joined)
	}
	if !strings.HasPrefix(strings.TrimRight(block[labelAt+1], " "), "  ") {
		t.Errorf("the block's first line is not at the value column:\n%s", joined)
	}
}
