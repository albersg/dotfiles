package tui

import (
	"strings"
	"testing"
)

// ============================================================================
// THE PORTS IN THE GRAMMAR — the guards
// ============================================================================
//
// The port manager and the stop confirmation draw the reference's cards on a
// frame that holds them (port_cards.go). These guards hold the three things that
// makes true: the cards are the shared ones, every fact the reading holds is still
// on the screen, and the 80x24 floor keeps the compact body it had.

// portCardFrame is one of the two port screens measured at a size.
type portCardFrame struct {
	name  string
	build func(*testing.T) Model
}

func portCardFrames() []portCardFrame {
	return []portCardFrame{
		{"port-manager", portManagerFrameCase},
		{"port-stop", portStopFrameCase},
	}
}

// TestPortScreensDrawTheGrammarsCards is the shape guard: on a frame that holds
// the cards the screen's own title is the chip set into the card's top border with
// the rule under it, and at the 80x24 floor the compact body is drawn and no card
// is -- the floor keeps the renderer it always had.
func TestPortScreensDrawTheGrammarsCards(t *testing.T) {
	for _, frame := range portCardFrames() {
		frame := frame
		t.Run(frame.name, func(t *testing.T) {
			m := frame.build(t)
			m.Width, m.Height = 160, 50
			m.Animating = false

			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if !sectionCardDrawn(m, m.GetScreenTitle()) {
				t.Errorf("%s does not carry its own title as the card's chip at 160x50:\n%s", frame.name, plain)
			}
			if !strings.Contains(plain, "╰") {
				t.Errorf("%s draws no closing card border:\n%s", frame.name, plain)
			}

			floor := frame.build(t)
			floor.Width, floor.Height = 80, 24
			floor.Animating = false
			if floorPlain := ansiEscape.ReplaceAllString(floor.View(), ""); sectionCardDrawn(floor, floor.GetScreenTitle()) {
				t.Errorf("%s draws the card at the 80x24 floor:\n%s", frame.name, floorPlain)
			}
		})
	}
}

// cardText flattens a rendered frame to the words it carries: the escapes and the
// card's own chrome are removed and every run of whitespace becomes one space, so
// a guard can read a sentence the card wrapped across two rows.
func cardText(view string) string {
	plain := ansiEscape.ReplaceAllString(view, "")
	plain = strings.NewReplacer(
		"│", " ", "╭", " ", "╮", " ", "╰", " ", "╯", " ",
		"├", " ", "┤", " ", "─", " ", "\n", " ",
	).Replace(plain)
	return strings.Join(strings.Fields(plain), " ")
}

// squash is cardText's own whitespace rule applied to a sentence the model holds,
// so a guard can compare the two.
func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TestPortScreensFitEveryCardFrame measures the two cards the way the terminal
// does: no row wider than the terminal and no more rows than it has, on the frames
// that hold two columns. The card shape is the one thing here that could draw past
// the frame, because its two columns are composed from the reading.
func TestPortScreensFitEveryCardFrame(t *testing.T) {
	for _, frame := range portCardFrames() {
		frame := frame
		t.Run(frame.name, func(t *testing.T) {
			for _, size := range measuredTerminalSizes {
				m := frame.build(t)
				m.Width, m.Height = size.width, size.height
				m.Animating = false
				assertScreenFitsTerminal(t, frame.name, size.width, size.height, m.View())
			}
		})
	}
}

// TestPortManagerCardCarriesTheReadingAndItsFacts is the data guard: every socket
// the reading holds is a row in the card -- its port, its protocol, its state, its
// address, its owner and its reach -- the scope and the filter state are in the
// card beside it, and the selected socket's own facts are there once Enter has
// opened them. Nothing the compact body stated has gone missing.
func TestPortManagerCardCarriesTheReadingAndItsFacts(t *testing.T) {
	m := portManagerFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plain := cardText(m.View())
	for _, want := range []string{
		// the card and its facts column
		"Ports in use", portFactsChip, portSelectedGroup,
		// one row per socket, with the port first and the owner under it
		":3000 tcp LISTEN 127.0.0.1", "node (pid 4242, alber) loopback only",
		":5353 udp UNCONN 127.0.0.1", "owner not readable loopback only",
		":8080 tcp LISTEN 0.0.0.0", "python3 (pid 900, alber) all interfaces",
		// the reading's own scope and state, in the column beside the list
		squash(m.portScopeSentence()), squash(m.portStateLine()),
		// the selected socket's facts, as the compact detail block stated them
		"/proc/net/tcp", "IPv4", "LISTEN", "/usr/bin/node",
		"(redacted) node server.js --token ****",
		"cannot be revalidated on this platform; a stop checks liveness only.",
		"the process behind them is not shown.",
		// and the notice the reading left
		"1 new, 1 gone since the last reading",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the manager's card does not state %q:\n%s", want, plain)
		}
	}
}

// TestPortManagerCardKeepsItsShapeWhileTheCursorMoves is the stability guard: the
// window and the facts column are measured from the frame, never from the row
// under the cursor, so walking the reading changes the row that is filled and
// nothing above it.
func TestPortManagerCardKeepsItsShapeWhileTheCursorMoves(t *testing.T) {
	base := portManagerFrameCase(t)
	base.Width, base.Height = 160, 50
	base.Animating = false

	top := func(m Model) string {
		plain := ansiEscape.ReplaceAllString(m.View(), "")
		return strings.Split(plain, "\n")[3] // the cards' top border row
	}
	want := top(base)
	for cursor := 0; cursor < len(base.portVisibleListeners()); cursor++ {
		moved := base
		moved.Ports.Cursor = cursor
		if got := top(moved); got != want {
			t.Fatalf("the manager's card moves with the cursor on row %d:\n%q\nthen:\n%q", cursor, want, got)
		}
	}
}

// TestPortStopCardCarriesTheAccountAndTheAnswer is the confirmation's data guard:
// the identity, the owner, every port the process holds, the outcome and its
// verification are in the card, and the answers it offers are under the card's own
// rule with the letters that run them.
func TestPortStopCardCarriesTheAccountAndTheAnswer(t *testing.T) {
	m := portStopFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plain := cardText(m.View())
	for _, want := range []string{
		portStopTitle,
		"Process: node, pid 4242, owned by alber.",
		"Executable: /usr/bin/node",
		"Arguments (redacted): node server.js --token ****",
		"Every port this process holds, and which a stop releases: :3000 tcp LISTEN 127.0.0.1.",
		"Outcome: pid 4242 (node) exited. The ports it held are free.",
		"The machine was re-read after the attempt, so this outcome is what was observed, not what was expected.",
		"← Back",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the confirmation's card does not state %q:\n%s", want, plain)
		}
	}
}

// TestPortCardsKeepTheScreensOwnKeys is the key guard: the footer of each card
// shape names the same presses as the compact body's, because the grammar changes
// where a row is drawn and never which key runs it.
func TestPortCardsKeepTheScreensOwnKeys(t *testing.T) {
	for _, frame := range portCardFrames() {
		frame := frame
		t.Run(frame.name, func(t *testing.T) {
			floor := frame.build(t)
			floor.Width, floor.Height = 80, 24
			floor.Animating = false
			wide := frame.build(t)
			wide.Width, wide.Height = 160, 50
			wide.Animating = false

			// The keys are the hints, which is the one list the footer and the handler
			// share, so a shape that dropped one would show up here.
			hints := wide.portManagerHints()
			if wide.Screen == ScreenPortStop {
				hints = wide.portStopHints()
			}
			plain := ansiEscape.ReplaceAllString(wide.View(), "")
			for _, hint := range hints {
				if !strings.Contains(plain, hint.keys) {
					t.Errorf("%s's card does not name %q in its footer:\n%s", frame.name, hint.keys, plain)
				}
			}
		})
	}
}

// TestPortFactsCardHoldsItsTallestReading is the guard for the padded facts
// column: it used to be as tall as the column the frame gave it, so a reading with
// three sockets and four facts left a card running to the footer. It is the
// tallest reading the cursor can reach now, measured with the facts open, so
// walking the reading -- and the press that opens the detail -- cannot resize the
// column they are looking at.
func TestPortFactsCardHoldsItsTallestReading(t *testing.T) {
	base := portManagerFrameCase(t)
	base.Width, base.Height = 160, 50
	base.Animating = false

	measure := func(m Model) (span [2]int, body int, card []string) {
		t.Helper()
		rows := strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
		rule, footer := frameBodyEnds(rows)
		if rule < 0 || footer < 0 {
			t.Fatalf("the manager does not draw the frame's own rule and footer:\n%s", strings.Join(rows, "\n"))
		}
		bodyRows := rows[rule+1 : footer]
		columns := cardColumns(bodyRows)
		if len(columns) < 2 {
			t.Fatalf("the manager draws one card at 160x50, want the reading and its facts beside it:\n%s", strings.Join(rows, "\n"))
		}
		spans := cardSpansInColumn(bodyRows, columns[len(columns)-1])
		if len(spans) != 1 {
			t.Fatalf("the manager draws %d cards in its facts column:\n%s", len(spans), strings.Join(rows, "\n"))
		}
		return spans[0], len(bodyRows), bodyRows[spans[0][0] : spans[0][1]+1]
	}

	want, body, card := measure(base)
	if want[1]-want[0]+1 >= body {
		t.Errorf("the facts card draws %d rows in a %d-row body: a short reading still fills the column", want[1]-want[0]+1, body)
	}
	if cardTrailingBlanks(card) != 0 {
		t.Errorf("the facts card keeps %d blank rows under its last fact, so it is padded rather than measured", cardTrailingBlanks(card))
	}

	for cursor := 0; cursor < len(base.portVisibleListeners()); cursor++ {
		moved := base
		moved.Ports.Cursor = cursor
		for _, open := range []bool{true, false} {
			moved.Ports.Detail = open
			got, _, _ := measure(moved)
			if got != want {
				t.Errorf("with the detail open=%v and the cursor on socket %d the facts card sits on rows %d-%d, where it sat on %d-%d: opening the detail or walking the reading moves the composition",
					open, cursor, got[0], got[1], want[0], want[1])
			}
		}
	}
}
