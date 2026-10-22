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
		{"ssh-manager", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHManager) }, "managed by this tool"},
		{"ssh-host", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHHost) }, "Declared in "},
		{"ssh-form", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHForm) }, "An alias is required"},
		{"ssh-keys", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHKeys) }, "A private key is never read by this screen"},
		{"ssh-agent", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHAgent) }, "holds"},
		{"ssh-diagnostics", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHDiagnostics) }, "to look at"},
		{"ssh-test", func(t *testing.T) Model { return sshTestModel(t, ScreenSSHTest) }, "Host keys are verified"},
		{"wsl-resources", wslResourcesFrameCase, "Memory is three quarters"},
		{"wsl-advisor", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLAdvisor) }, "This instance has"},
		{"wsl-benchmark", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLBenchmark) }, "Targets, measured in this order"},
		{"wsl-migration", func(t *testing.T) Model { return wslAdvisorFrameModel(t, ScreenWSLMigration) }, "Copy "},
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
	if body := strings.Join(resources.wslResourceDescription(), " "); !strings.Contains(body, "memory 4321 MB") {
		t.Errorf("the compact body does not draw the planned memory the model holds:\n%s", body)
	}
	resources.Width, resources.Height = 160, 50
	resources.Animating = false
	plain = cardText(resources.View())
	if !strings.Contains(plain, "memory 4321 MB") {
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
