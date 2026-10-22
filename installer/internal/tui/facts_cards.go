package tui

import (
	"strings"
)

// ============================================================================
// THE SECTION SCREENS' DETAIL — facts first, and the prose derived from them
// ============================================================================
//
// The SSH screens and the WSL screens draw the reference's cards on a frame that
// holds two columns (section_wide.go): their rows are cardRowLines and the detail
// of the screen is the card beside them. The detail was prose -- a list of
// paragraphs composed by each screen's own description function -- so the card was
// a paragraph block with a border, and nothing on the screen said which part of a
// sentence was the fact.
//
// This file is the same translation report_cards.go made for the two reports, for
// the screens whose body really is a list of facts: the detail is an []iconFact --
// the label that names the fact and its value -- and both shapes are drawn from
// that one list. iconFactLines draws it in the card at the value column, and
// factProse derives the compact body's paragraphs from it, so the floor and the
// wide frame cannot explain one fact two ways.
//
// Nothing here chooses a colour, a width or a row of its own beyond what the
// grammar already fixes, and nothing here invents a fact: the values are the
// model's own fields and the screens' own sentences, moved into the fact that
// names them. A screen that has nothing to state is not in the registry, and keeps
// drawing its prose.

// factLines draws a screen's facts in the grammar: iconFactLines for the facts
// that are a labelled value, and a labelled block for a value that carries its own
// lines. A preview is lines of a file, a public key is one long line, and a tool's
// output is the tool's own rows; wrapping those as one sentence would join lines
// that mean different things, so they keep their own rows under the label that
// names them.
func factLines(facts []iconFact, width int) []string {
	var out []string
	single := make([]iconFact, 0, len(facts))
	flush := func() {
		if len(single) > 0 {
			out = append(out, iconFactLines(single, width)...)
			single = single[:0]
		}
	}
	for _, fact := range facts {
		if strings.TrimSpace(fact.Value) == "" {
			continue
		}
		if !strings.Contains(fact.Value, "\n") {
			single = append(single, fact)
			continue
		}
		flush()
		out = append(out, factBlockLines(fact, width)...)
	}
	flush()
	return out
}

// factBlockLines draws one fact whose value is a block of lines: the label on its
// own row in the muted metadata tone, and then every line at the value column in
// the ink tone, wrapped rather than cut so a long line of a diff stays readable.
func factBlockLines(fact iconFact, width int) []string {
	width = max(1, width)
	measure := grammarLabelMeasure
	if width < grammarLabelMeasure+12 {
		measure = min(grammarLabelMeasure, max(1, width/2))
	}
	valueWidth := max(1, width-measure-1)
	indent := strings.Repeat(" ", measure+1)

	gutter := fact.Icon
	if gutter == "" {
		gutter = " "
	}
	out := []string{AccentKeyStyle.Render(gutter) + " " +
		MutedStyle.Render(padRight(truncate(fact.Label, max(1, measure-2)), max(1, measure-2)))}

	for _, line := range strings.Split(fact.Value, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, wrapped := range wrapText(line, max(1, valueWidth-2), 0) {
			out = append(out, indent+InkStyle.Render(wrapped))
		}
	}
	return out
}

// factProse derives a screen's compact body from the same facts the card draws.
// A value that is a whole sentence is the paragraph it always was; a bare value is
// introduced by the label that names it, so a path or a command reads as one
// sentence and not as a fragment; and a value that carries its own lines prints
// its label as the heading and its lines one paragraph each, in the order the
// screen wrote them.
func factProse(facts []iconFact) []string {
	paragraphs := make([]string, 0, len(facts))
	for _, fact := range facts {
		// A block is read before it is trimmed: the indentation of a preview line
		// says what the line is (a line that stays, one that arrives), so the block's
		// lines reach the body exactly as the plan wrote them.
		if lines := strings.Split(fact.Value, "\n"); len(lines) > 1 {
			if label := strings.TrimSpace(fact.Label); label != "" {
				paragraphs = append(paragraphs, strings.TrimSuffix(label, ":")+":")
			}
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				paragraphs = append(paragraphs, line)
			}
			continue
		}
		if strings.TrimSpace(fact.Value) == "" {
			continue
		}
		paragraphs = append(paragraphs, factSentence(fact))
	}
	return paragraphs
}

// factSentence is one fact as a sentence: the value stands alone when it already
// ends a sentence, and otherwise the label introduces it. It is the only place the
// two are joined, so a label can never say one thing on the card and another in
// the prose.
func factSentence(fact iconFact) string {
	value := strings.TrimSpace(fact.Value)
	if value == "" {
		return ""
	}
	label := strings.TrimSpace(fact.Label)
	if label == "" || factEndsSentence(value) {
		return value
	}
	return strings.TrimSuffix(label, ":") + ": " + value + "."
}

// factClause is a value a labelled fact introduces in the prose: the sentence's
// full stop is trimmed so the label and the clause read as one sentence
// ("Evidence: the mount took 6.4 s.") instead of two, and the card keeps the
// clause it names. A value that carries no full stop is returned untouched.
func factClause(value string) string {
	clause := strings.TrimSpace(value)
	for _, end := range []string{".", "…", "!", "?"} {
		if strings.HasSuffix(clause, end) {
			return strings.TrimSuffix(clause, end)
		}
	}
	return clause
}

// factEndsSentence reports whether a value is already a whole sentence, which is
// what lets the screens keep the sentences they were written with: the prose
// prints them unchanged and the card adds only the label that names them.
func factEndsSentence(value string) bool {
	return strings.HasSuffix(value, ".") || strings.HasSuffix(value, "…") ||
		strings.HasSuffix(value, "?") || strings.HasSuffix(value, "!")
}

// sectionScreenFacts is the registry: the detail of the screens whose body is a
// list of facts, and false for every screen that still composes its detail as
// prose. It is asked by both shapes -- sectionScreenGroups for the card and each
// screen's own description function for the compact body -- so a screen is either
// converted once or not at all.
func (m Model) sectionScreenFacts() ([]iconFact, bool) {
	switch m.Screen {
	case ScreenSSHManager:
		return m.sshManagerFacts(), true
	case ScreenSSHHost:
		return m.sshHostFacts(), true
	case ScreenSSHForm:
		return m.sshFormFacts(), true
	case ScreenSSHKeys:
		return m.sshKeyFacts(), true
	case ScreenSSHAgent:
		return m.sshAgentFacts(), true
	case ScreenSSHDiagnostics:
		return m.sshDiagnosticFacts(), true
	case ScreenSSHTest:
		return m.sshTestFacts(), true
	case ScreenWSLResources:
		return m.wslResourceFacts(), true
	case ScreenWSLAdvisor:
		return m.wslAdvisorFacts(), true
	case ScreenWSLBenchmark:
		return m.wslBenchmarkFacts(), true
	case ScreenWSLMigration:
		return m.wslMigrationFacts(), true
	}
	return nil, false
}

// sectionScreenGroups is the detail column's body for a converted screen: the
// facts as the grammar's rows, in one group, because a screen's facts are one
// block and a divider between them would name a category the screen does not have.
func (m Model) sectionScreenGroups(content int) ([]cardGroup, bool) {
	facts, ok := m.sectionScreenFacts()
	if !ok {
		return nil, false
	}
	return []cardGroup{{Lines: factLines(facts, content)}}, true
}
