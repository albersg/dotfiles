package trainer

import (
	"fmt"
	"reflect"
	"testing"
)

// editingCase is one table-driven specification row for the mutable editing
// engine. Every field is asserted: the buffer after the answer, the cursor, the
// mode and the recognized flag.
type editingCase struct {
	name       string
	code       []string
	start      Position
	input      string
	wantBuffer []string
	wantCursor Position
	wantMode   Mode
	wantSel    Selection
	wantRec    bool
}

func runEditingCases(t *testing.T, cases []editingCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SimulateEditing(tc.code, tc.start, tc.input)

			if !reflect.DeepEqual(got.Buffer, tc.wantBuffer) {
				t.Errorf("Buffer = %#v, want %#v", got.Buffer, tc.wantBuffer)
			}
			if got.Cursor != tc.wantCursor {
				t.Errorf("Cursor = %+v, want %+v", got.Cursor, tc.wantCursor)
			}
			if got.Mode != tc.wantMode {
				t.Errorf("Mode = %v, want %v", got.Mode, tc.wantMode)
			}
			if !reflect.DeepEqual(got.Selection, tc.wantSel) {
				t.Errorf("Selection = %+v, want %+v", got.Selection, tc.wantSel)
			}
			if got.Recognized != tc.wantRec {
				t.Errorf("Recognized = %v, want %v", got.Recognized, tc.wantRec)
			}
		})
	}
}

// multiLineBuffer is the canonical four-line fixture used by the mutation
// specifications. The second line is indented so >> and << have something to
// act on and p/P have a visible first-non-blank column.
func multiLineBuffer() []string {
	return []string{"alpha", "  beta", "gamma", "delta"}
}

func TestSimulateEditing_MutationsOnMultiLineBuffer(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "dd deletes the current line",
			code: base, start: Position{Line: 1, Col: 2}, input: "dd",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd deletes two lines as one command",
			code: base, start: Position{Line: 1, Col: 0}, input: "2dd",
			wantBuffer: []string{"alpha", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3dd deletes three lines",
			code: base, start: Position{Line: 1, Col: 0}, input: "3dd",
			wantBuffer: []string{"alpha"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "count larger than the buffer empties it",
			code: base, start: Position{Line: 0, Col: 0}, input: "9dd",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd at the first line",
			code: base, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{"  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd at the last line",
			code: base, start: Position{Line: 3, Col: 0}, input: "dd",
			wantBuffer: []string{"alpha", "  beta", "gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x deletes the rune under the cursor",
			code: base, start: Position{Line: 1, Col: 2}, input: "x",
			wantBuffer: []string{"alpha", "  eta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3x deletes three runes",
			code: base, start: Position{Line: 1, Col: 2}, input: "3x",
			wantBuffer: []string{"alpha", "  a", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x at the last character deletes it",
			code: base, start: Position{Line: 0, Col: 4}, input: "x",
			wantBuffer: []string{"alph", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x with a count past the end stops at end of line",
			code: base, start: Position{Line: 0, Col: 3}, input: "9x",
			wantBuffer: []string{"alp", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D deletes to the end of the line",
			code: base, start: Position{Line: 0, Col: 2}, input: "D",
			wantBuffer: []string{"al", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D at column zero empties the line",
			code: base, start: Position{Line: 2, Col: 0}, input: "D",
			wantBuffer: []string{"alpha", "  beta", "", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p duplicates the line below",
			code: base, start: Position{Line: 1, Col: 0}, input: "yyp",
			wantBuffer: []string{"alpha", "  beta", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then P duplicates the line above",
			code: base, start: Position{Line: 1, Col: 0}, input: "yyP",
			wantBuffer: []string{"alpha", "  beta", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p then P leaves three copies",
			code: base, start: Position{Line: 1, Col: 0}, input: "yypP",
			wantBuffer: []string{"alpha", "  beta", "  beta", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then p pastes below the line that took its place",
			code: base, start: Position{Line: 1, Col: 0}, input: "ddp",
			wantBuffer: []string{"alpha", "gamma", "  beta", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then P restores the buffer",
			code: base, start: Position{Line: 1, Col: 0}, input: "ddP",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift indents the current line",
			code: base, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"  alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on an indented line adds one unit",
			code: base, start: Position{Line: 1, Col: 0}, input: ">>",
			wantBuffer: []string{"alpha", "    beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2>> indents two lines",
			code: base, start: Position{Line: 0, Col: 0}, input: "2>>",
			wantBuffer: []string{"  alpha", "    beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes one unit",
			code: base, start: Position{Line: 1, Col: 0}, input: "<<",
			wantBuffer: []string{"alpha", "beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2<< removes one unit from two lines",
			code: base, start: Position{Line: 1, Col: 0}, input: "2<<",
			wantBuffer: []string{"alpha", "beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift on an unindented line is a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift then left shift restores the indentation",
			code: base, start: Position{Line: 1, Col: 0}, input: ">><<",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift then right shift restores the indentation",
			code: base, start: Position{Line: 1, Col: 0}, input: "<<>>",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_FirstAndLastLine(t *testing.T) {
	base := []string{"alpha", "beta", "gamma"}

	runEditingCases(t, []editingCase{
		{
			name: "x on the first line",
			code: base, start: Position{Line: 0, Col: 0}, input: "x",
			wantBuffer: []string{"lpha", "beta", "gamma"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x on the last line",
			code: base, start: Position{Line: 2, Col: 4}, input: "x",
			wantBuffer: []string{"alpha", "beta", "gamm"},
			wantCursor: Position{Line: 2, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D on the first line",
			code: base, start: Position{Line: 0, Col: 1}, input: "D",
			wantBuffer: []string{"a", "beta", "gamma"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D on the last line",
			code: base, start: Position{Line: 2, Col: 1}, input: "D",
			wantBuffer: []string{"alpha", "beta", "g"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on the last line",
			code: base, start: Position{Line: 2, Col: 0}, input: ">>",
			wantBuffer: []string{"alpha", "beta", "  gamma"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift on the last line",
			code: []string{"alpha", "beta", "  gamma"}, start: Position{Line: 2, Col: 0}, input: "<<",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p on the last line appends below it",
			code: base, start: Position{Line: 2, Col: 0}, input: "yyp",
			wantBuffer: []string{"alpha", "beta", "gamma", "gamma"},
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then P on the first line inserts above it",
			code: base, start: Position{Line: 0, Col: 0}, input: "yyP",
			wantBuffer: []string{"alpha", "alpha", "beta", "gamma"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then p on the last line restores it",
			code: base, start: Position{Line: 2, Col: 0}, input: "ddp",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "p with an empty register is a no-op on the last line",
			code: base, start: Position{Line: 2, Col: 0}, input: "p",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_EmptyAndSingleLineBuffers(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "dd on an empty buffer stays empty",
			code: []string{}, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x on a nil buffer stays empty",
			code: nil, start: Position{Line: 0, Col: 0}, input: "x",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D on an empty buffer is safe",
			code: []string{}, start: Position{Line: 0, Col: 0}, input: "D",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "p on an empty buffer is safe",
			code: []string{}, start: Position{Line: 0, Col: 0}, input: "p",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo on an empty buffer is safe",
			code: []string{}, start: Position{Line: 0, Col: 0}, input: "u",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo on an empty buffer is safe",
			code: []string{}, start: Position{Line: 0, Col: 0}, input: "\x12",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "deleting the only line leaves an empty buffer",
			code: []string{"only"}, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x empties a single character line",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "x",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D empties a single character line",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "D",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p duplicates a single line",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "yyp",
			wantBuffer: []string{"a", "a"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then p pastes below the empty remainder of a single line",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "ddp",
			wantBuffer: []string{"", "a"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_RuneColumns(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "x deletes a multi-byte rune without corrupting the line",
			code: []string{"aébc"}, start: Position{Line: 0, Col: 1}, input: "x",
			wantBuffer: []string{"abc"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x at the last multi-byte rune",
			code: []string{"aébc"}, start: Position{Line: 0, Col: 3}, input: "x",
			wantBuffer: []string{"aéb"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D keeps rune columns",
			code: []string{"aébc"}, start: Position{Line: 0, Col: 1}, input: "D",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "start column clamps in runes",
			code: []string{"aébc"}, start: Position{Line: 0, Col: 99}, input: "x",
			wantBuffer: []string{"aéb"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "start line clamps into range",
			code: []string{"a", "b"}, start: Position{Line: 9, Col: 0}, input: "dd",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_UndoRedo(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "undo with empty history is a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "u",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo with empty history is a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "\x12",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "u undoes x",
			code: base, start: Position{Line: 0, Col: 0}, input: "xu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo reapplies x",
			code: base, start: Position{Line: 0, Col: 0}, input: "xu\x12",
			wantBuffer: []string{"lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3x is a single undo step",
			code: base, start: Position{Line: 0, Col: 0}, input: "3xu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "u restores the cursor too",
			code: base, start: Position{Line: 1, Col: 2}, input: "xu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u restores the buffer",
			code: base, start: Position{Line: 1, Col: 2}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim with "set shiftwidth=2 expandtab tabstop=2 startofline":
			// dd then u leaves the cursor on the first non-blank of the restored
			// line (column 3 of "  beta"), and Ctrl-r puts the line back without
			// moving the cursor, so the redo ends at column 3 rather than at the
			// column dd itself landed on.
			name: "dd then u then redo deletes again",
			code: base, start: Position{Line: 1, Col: 2}, input: "ddu\x12",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift then u restores the indentation",
			code: base, start: Position{Line: 0, Col: 0}, input: ">>u",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p then u removes the duplicate",
			code: base, start: Position{Line: 1, Col: 0}, input: "yypu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then P then u removes the restored line",
			code: base, start: Position{Line: 1, Col: 0}, input: "ddPu",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an empty register p leaves nothing to undo",
			code: base, start: Position{Line: 0, Col: 0}, input: "pu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "two no-op commands leave nothing to undo",
			code: base, start: Position{Line: 0, Col: 0}, input: "puu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a second u is a no-op once history is exhausted",
			code: base, start: Position{Line: 0, Col: 0}, input: "xuu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo stack clears on a new change",
			code: base, start: Position{Line: 0, Col: 0}, input: "xu\x12x\x12",
			wantBuffer: []string{"pha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op delete does not create an undo entry",
			code: []string{""}, start: Position{Line: 0, Col: 0}, input: "ddu",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_Recognition(t *testing.T) {
	base := []string{"alpha", "  beta"}

	runEditingCases(t, []editingCase{
		{
			name: "empty input is recognized and copies the buffer",
			code: base, start: Position{Line: 0, Col: 0}, input: "",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{"  beta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "yy",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"  alpha", "  beta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift is recognized",
			code: base, start: Position{Line: 1, Col: 0}, input: "<<",
			wantBuffer: []string{"alpha", "beta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "x",
			wantBuffer: []string{"lpha", "  beta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "D",
			wantBuffer: []string{"", "  beta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "p is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "p",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "u is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "u",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "count survives recognition",
			code: base, start: Position{Line: 0, Col: 0}, input: "3x",
			wantBuffer: []string{"ha", "  beta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an incomplete operator is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "d",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a bare count is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "3",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an unknown key is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "q",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a mutation followed by a motion is recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "ddw",
			wantBuffer: []string{"  beta"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an unknown key stops parsing the tail",
			code: base, start: Position{Line: 0, Col: 0}, input: "ddq",
			wantBuffer: []string{"  beta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a mutation before an unknown key is still applied",
			code: base, start: Position{Line: 0, Col: 0}, input: "xq",
			wantBuffer: []string{"lpha", "  beta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
	})
}

// TestSimulateEditing_RecognitionDiffersFromMotionSimulator records why the
// editing engine needs its own recognizer: the shipped motion recognizer marks
// every mutating command as unrecognized, so it cannot express the editing
// engine's answers. Both agree on the shared notion, "the whole input was
// consumed", which is the only thing callers may rely on. A mark belongs to the
// same list: m{a-z} and the jumps to it only mean something if a table survives
// between two commands, and the motion simulator is stateless, so the mark
// commands exist in the editor alone.
func TestSimulateEditing_RecognitionDiffersFromMotionSimulator(t *testing.T) {
	code := []string{"alpha", "  beta"}

	for _, input := range []string{"dd", "yy", ">>", "<<", "x", "D", "p", "P", "u", "\x12", "ma", "`a", "'a"} {
		if !SimulateEditing(code, Position{}, input).Recognized {
			t.Errorf("SimulateEditing(%q).Recognized = false, want true", input)
		}
	}

	for _, input := range []string{"x", "p", "P", "u", ">>", "<<", "\x12", "ma", "`a", "'a"} {
		if IsRecognizedInput(code, input) {
			t.Errorf("IsRecognizedInput(%q) = true; the motion recognizer has no notion of this command", input)
		}
	}
}

// TestSimulateEditing_PreservesExistingEntryPoints is the explicit regression
// bar for the additive engine: the motion simulator, its selection math and the
// shipped recognizer must return exactly what they returned before editor.go
// existed. The expected values were captured from the implementation on the
// commit that introduced this engine.
func TestSimulateEditing_PreservesExistingEntryPoints(t *testing.T) {
	code := []string{
		"func main() {",
		"  x := 1",
		"  return x",
		"}",
	}

	tests := []struct {
		name    string
		input   string
		start   Position
		wantPos SimulatedPosition
		wantSel Selection
		wantRec bool
	}{
		{"empty", "", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, true},
		{"word forward", "w", Position{0, 0}, SimulatedPosition{0, 5}, Selection{}, true},
		{"word end", "e", Position{0, 0}, SimulatedPosition{0, 3}, Selection{}, true},
		{"word backward", "b", Position{1, 2}, SimulatedPosition{1, 0}, Selection{}, true},
		{"line start", "0", Position{1, 2}, SimulatedPosition{1, 0}, Selection{}, true},
		{"first non blank", "^", Position{1, 2}, SimulatedPosition{1, 2}, Selection{}, true},
		{"line end", "$", Position{0, 0}, SimulatedPosition{0, 12}, Selection{}, true},
		{"find char", "fe", Position{1, 2}, SimulatedPosition{1, 2}, Selection{}, true},
		{"line down", "j", Position{0, 0}, SimulatedPosition{1, 0}, Selection{}, true},
		{"go first line", "gg", Position{1, 2}, SimulatedPosition{0, 0}, Selection{}, true},
		{"go last line", "G", Position{0, 0}, SimulatedPosition{3, 0}, Selection{}, true},
		{"counted word", "3w", Position{0, 0}, SimulatedPosition{0, 12}, Selection{}, true},
		{"counted down clamps", "10j", Position{0, 0}, SimulatedPosition{3, 0}, Selection{}, true},
		{"delete word selection", "dw", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 4, true}, true},
		{"delete to end selection", "d$", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 12, true}, true},
		{"delete line selection", "dd", Position{1, 2}, SimulatedPosition{1, 2}, Selection{1, 0, 1, 7, true}, true},
		{"change inner word selection", "ciw", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 3, true}, true},
		{"visual inner word selection", "viw", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 3, true}, true},
		{"visual around word selection", "vaw", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 4, true}, true},
		{"yank line selection", "yy", Position{1, 2}, SimulatedPosition{1, 2}, Selection{1, 0, 1, 7, true}, true},
		{"delete to end", "D", Position{1, 2}, SimulatedPosition{1, 2}, Selection{1, 2, 1, 7, true}, true},
		{"change to end", "C", Position{0, 0}, SimulatedPosition{0, 0}, Selection{0, 0, 0, 12, true}, true},
		{"unknown x", "x", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, false},
		{"unknown p", "p", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, false},
		{"unknown q", "q", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, false},
		{"incomplete d", "d", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, false},
		{"bare count", "3", Position{0, 0}, SimulatedPosition{0, 0}, Selection{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SimulateMotionsWithSelection(tt.start, code, tt.input)

			if got.Position != tt.wantPos {
				t.Errorf("Position = %+v, want %+v", got.Position, tt.wantPos)
			}
			if !reflect.DeepEqual(got.Selection, tt.wantSel) {
				t.Errorf("Selection = %+v, want %+v", got.Selection, tt.wantSel)
			}
			if got.Recognized != tt.wantRec {
				t.Errorf("Recognized = %v, want %v", got.Recognized, tt.wantRec)
			}

			motionPos := SimulateMotions(tt.start, code, tt.input)
			if motionPos != tt.wantPos {
				t.Errorf("SimulateMotions position = %+v, want %+v", motionPos, tt.wantPos)
			}
		})
	}

	recognizedTrue := []string{"", "w", "e", "b", "0", "^", "$", "fe", "j", "gg", "G", "3w", "10j", "dw", "d$", "dd", "ciw", "viw", "vaw", "yy", "D", "C"}
	for _, input := range recognizedTrue {
		if !IsRecognizedInput(code, input) {
			t.Errorf("IsRecognizedInput(%q) = false, want true", input)
		}
	}

	recognizedFalse := []string{"x", "p", "q", "d", "3"}
	for _, input := range recognizedFalse {
		if IsRecognizedInput(code, input) {
			t.Errorf("IsRecognizedInput(%q) = true, want false", input)
		}
	}
}

// TestSimulateEditing_MotionsOnMultiLineBuffer specifies the motion vocabulary
// inside the editing engine. A motion changes only the cursor, so every row
// asserts the buffer is unchanged.
func TestSimulateEditing_MotionsOnMultiLineBuffer(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "w moves to the first non-blank of the next line",
			code: base, start: Position{Line: 0, Col: 0}, input: "w",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "b moves to the start of the word",
			code: base, start: Position{Line: 2, Col: 4}, input: "b",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "e moves to the end of the word",
			code: base, start: Position{Line: 1, Col: 2}, input: "e",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "0 moves to the first column",
			code: base, start: Position{Line: 1, Col: 4}, input: "0",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "^ moves to the first non-blank column",
			code: base, start: Position{Line: 1, Col: 0}, input: "^",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "$ moves to the last character of the line",
			code: base, start: Position{Line: 3, Col: 0}, input: "$",
			wantBuffer: base,
			wantCursor: Position{Line: 3, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "gg moves to the first line",
			code: base, start: Position{Line: 2, Col: 3}, input: "gg",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "G moves to the last line",
			code: base, start: Position{Line: 0, Col: 0}, input: "G",
			wantBuffer: base,
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "f finds the character under the cursor",
			code: base, start: Position{Line: 0, Col: 0}, input: "fa",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "t stops before the character",
			code: base, start: Position{Line: 0, Col: 0}, input: "ta",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count repeats the motion",
			code: base, start: Position{Line: 0, Col: 0}, input: "10j",
			wantBuffer: base,
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a motion past the last line clamps there",
			code: base, start: Position{Line: 0, Col: 0}, input: "99G",
			wantBuffer: base,
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_MotionThenMutation covers the behavior the engine did not
// have before: a motion repositions the cursor and the mutation that follows
// acts on the line the motion reached. Each row asserts the exact resulting
// buffer, so a mutation that was dropped or applied twice fails loudly.
func TestSimulateEditing_MotionThenMutation(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "jdd deletes the line below the cursor",
			code: base, start: Position{Line: 0, Col: 0}, input: "jdd",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3Gdd deletes the third line",
			code: base, start: Position{Line: 0, Col: 0}, input: "3Gdd",
			wantBuffer: []string{"alpha", "  beta", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Gdd deletes the last line",
			code: base, start: Position{Line: 0, Col: 0}, input: "Gdd",
			wantBuffer: []string{"alpha", "  beta", "gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "$D deletes to the end of the line",
			code: base, start: Position{Line: 0, Col: 0}, input: "$D",
			wantBuffer: []string{"alph", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "wx deletes the rune the motion lands on",
			code: base, start: Position{Line: 0, Col: 0}, input: "wx",
			wantBuffer: []string{"alpha", "  eta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a motion then >> indents the line it reached",
			code: base, start: Position{Line: 0, Col: 0}, input: "w>>",
			wantBuffer: []string{"alpha", "    beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3jdd deletes the line three motions reached",
			code: base, start: Position{Line: 0, Col: 0}, input: "3jdd",
			wantBuffer: []string{"alpha", "  beta", "gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a motion past the end still deletes the last line",
			code: base, start: Position{Line: 0, Col: 0}, input: "10jdd",
			wantBuffer: []string{"alpha", "  beta", "gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an unknown key after a motion keeps the cursor it reached",
			code: base, start: Position{Line: 0, Col: 0}, input: "jq",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an unknown key after a motion and a mutation keeps the deletion",
			code: base, start: Position{Line: 0, Col: 0}, input: "jddq",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a motion is not its own undo step",
			code: base, start: Position{Line: 0, Col: 0}, input: "jddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_MotionsDelegateToMotionSimulator is the delegation
// invariant: for a motion-only answer the editing engine must land exactly
// where SimulateMotions lands, from the same start, on the same code. It is
// what proves the engine routes through the simulator's motion parser instead
// of growing a second, subtly different motion implementation.
func TestSimulateEditing_MotionsDelegateToMotionSimulator(t *testing.T) {
	fixtures := map[string][]string{
		"func": {
			"func main() {",
			"  x := 1",
			"  return x",
			"}",
		},
		"short":  {"ab", "cd", ""},
		"spaced": {"ab ", "cd", "ef"},
	}

	inputs := []string{
		"w", "W", "b", "B", "e", "E", "0", "^", "$", "gg", "G",
		"j", "k", "h", "l", "fe", "te", "Fe", "Te", ";", ",",
		"3w", "2j", "10j", "2G", "{", "}", "H", "M", "L", "_",
		"wh", "ww", "wl", "fb;", "fb,", "j0",
	}
	starts := []Position{{Line: 0, Col: 0}, {Line: 1, Col: 2}, {Line: 2, Col: 0}}

	for name, code := range fixtures {
		for _, start := range starts {
			for _, input := range inputs {
				t.Run(fmt.Sprintf("%s/%s/from-%d-%d", name, input, start.Line, start.Col), func(t *testing.T) {
					want := SimulateMotions(start, code, input)
					got := SimulateEditing(code, start, input)

					if !got.Recognized {
						t.Fatalf("SimulateEditing(%q).Recognized = false, want true", input)
					}
					if got.Cursor != (Position{Line: want.Line, Col: want.Col}) {
						t.Errorf("Cursor = %+v, want %+v (SimulateMotions)", got.Cursor, want)
					}
					if !reflect.DeepEqual(got.Buffer, code) {
						t.Errorf("Buffer = %#v, want the code unchanged", got.Buffer)
					}
				})
			}
		}
	}
}

// TestSimulateEditing_UnrecognizedMotionsStayUnrecognized is the other half of
// the delegation invariant: a keystroke or token the motion parser rejects must
// not be smuggled in as a recognized command. Each row asserts the buffer and
// cursor are untouched and the answer is reported unrecognized.
func TestSimulateEditing_UnrecognizedMotionsStayUnrecognized(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "an unknown key is unrecognized",
			code: base, start: Position{Line: 1, Col: 2}, input: "z",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a repeated unknown key is unrecognized",
			code: base, start: Position{Line: 1, Col: 2}, input: "zz",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an unknown g command is unrecognized",
			code: base, start: Position{Line: 1, Col: 2}, input: "gq",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an incomplete find is unrecognized",
			code: base, start: Position{Line: 1, Col: 2}, input: "f",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a bare count is unrecognized",
			code: base, start: Position{Line: 1, Col: 2}, input: "5",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an unknown key after a valid motion keeps the motion",
			code: base, start: Position{Line: 0, Col: 0}, input: "jz",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "an unknown key after a count and a motion keeps the motion",
			code: base, start: Position{Line: 0, Col: 0}, input: "2jz",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
	})
}

// nvimShiftReference names the reference implementation and the exact settings
// every expectation below was read from: nvim v0.12.5 run as
// "nvim --clean --headless" with "set shiftwidth=2 expandtab tabstop=2
// startofline", one command per undo block, on the buffer and start position
// of each case. The trainer's exercises are written for those settings, which
// is what makes this the reference rather than a test written beside the
// engine.
//
// Two of nvim's behaviours are part of the model and are easy to get wrong when
// indentation is treated as a prefix of characters:
//
//   - Indentation is a width in columns. A tab advances to the next tabstop, so
//     with 'tabstop=2' the tab of "\ta" is two columns wide: >> rewrites it as
//     spaces and << removes two columns of whitespace, never a whole tab.
//   - An empty line is left alone, but a line holding only whitespace is
//     shifted. Inside a range nvim skips "", and still indents "   ".
//
// Note that this nvim build defaults 'startofline' off while Vim documents it
// on. The engine models it on, which is also what its own mutations already do:
// dd lands on the first non-blank.
const nvimShiftReference = "nvim --clean --headless, shiftwidth=2 expandtab tabstop=2 startofline"

// TestSimulateEditing_ShiftIndentationColumns specifies >> and << against
// nvimShiftReference. Every single-line case uses a one-line buffer with the
// cursor in column 1; each range case states its start position in its name.
func TestSimulateEditing_ShiftIndentationColumns(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "right shift leaves an empty line empty",
			code: []string{""}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift leaves an empty line empty",
			code: []string{""}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift indents an unindented line",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"  a"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift on an unindented line is a no-op",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift adds one shift to space indentation",
			code: []string{"  a"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"    a"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes one shift of space indentation",
			code: []string{"  a"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift rewrites a tab as spaces",
			code: []string{"\ta"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"    a"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes the two columns a tab occupies",
			code: []string{"\ta"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift measures spaces and a tab in columns",
			code: []string{"  \ta"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"      a"},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes two of the four columns of spaces plus tab",
			code: []string{"  \ta"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"  a"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift measures two tabs as four columns",
			code: []string{"\t\ta"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"      a"},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift leaves two of the four columns of two tabs",
			code: []string{"\t\ta"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"  a"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on a single leading space",
			code: []string{" a"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"   a"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes the single leading space",
			code: []string{" a"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on three leading spaces",
			code: []string{"   a"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"     a"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes exactly one shift of three spaces",
			code: []string{"   a"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{" a"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift measures a space and a tab in columns",
			code: []string{" \ta"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"    a"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift removes the space and tab columns",
			code: []string{" \ta"}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted right shift skips a blank line in its range",
			code: []string{"a", "", "b"}, start: Position{Line: 0, Col: 0}, input: "2>>",
			wantBuffer: []string{"  a", "", "b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted left shift skips a blank line in its range",
			code: []string{"  a", "", "  b"}, start: Position{Line: 0, Col: 0}, input: "2<<",
			wantBuffer: []string{"a", "", "  b"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted right shift skips a blank line inside its range",
			code: []string{"a", "", "\tb"}, start: Position{Line: 0, Col: 0}, input: "3>>",
			wantBuffer: []string{"  a", "", "    b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted right shift from a blank line expands the tab line",
			code: []string{"a", "", "\tb"}, start: Position{Line: 1, Col: 0}, input: "2>>",
			wantBuffer: []string{"a", "", "    b"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted left shift skips a blank line before a tab line",
			code: []string{"a", "  ", "\tb"}, start: Position{Line: 1, Col: 0}, input: "2<<",
			wantBuffer: []string{"a", "", "b"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted right shift indents a whitespace-only line",
			code: []string{"a", "   ", "b"}, start: Position{Line: 0, Col: 0}, input: "2>>",
			wantBuffer: []string{"  a", "     ", "b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted left shift outdents a whitespace-only line",
			code: []string{"a", "   ", "b"}, start: Position{Line: 0, Col: 0}, input: "2<<",
			wantBuffer: []string{"a", " ", "b"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count past the buffer shifts every remaining line",
			code: []string{"a", "", "b"}, start: Position{Line: 0, Col: 0}, input: "5>>",
			wantBuffer: []string{"  a", "", "  b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the shift cursor does not depend on the starting column",
			code: []string{"a", "", "\tb"}, start: Position{Line: 0, Col: 2}, input: "2>>",
			wantBuffer: []string{"  a", "", "\tb"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_StartOfLineLandingOnBlankLines pins the cursor landing
// shared by every linewise mutation against nvimShiftReference: the first
// non-blank, or the last character when the line has no non-blank. nvim uses
// that landing for >>, <<, dd, p and P, so an all-blank line is entered at its
// last column and an empty line at column zero.
func TestSimulateEditing_StartOfLineLandingOnBlankLines(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "right shift on a whitespace-only line ends on its last character",
			code: []string{"   "}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"     "},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on a whitespace-only line ignores the starting column",
			code: []string{"   "}, start: Position{Line: 0, Col: 2}, input: ">>",
			wantBuffer: []string{"     "},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on a two-space line ends on its last character",
			code: []string{"  "}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"    "},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on a tab-only line ends on its last character",
			code: []string{"\t"}, start: Position{Line: 0, Col: 0}, input: ">>",
			wantBuffer: []string{"    "},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "left shift on a whitespace-only line ends on its last character",
			code: []string{"   "}, start: Position{Line: 0, Col: 0}, input: "<<",
			wantBuffer: []string{" "},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "right shift on a whitespace-only line below the first line",
			code: []string{"a", "   ", "b"}, start: Position{Line: 1, Col: 0}, input: ">>",
			wantBuffer: []string{"a", "     ", "b"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd landing on a whitespace-only line ends on its last character",
			code: []string{"alpha", "   ", "gamma"}, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{"   ", "gamma"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd landing on an empty line lands in column zero",
			code: []string{"alpha", "", "gamma"}, start: Position{Line: 0, Col: 0}, input: "dd",
			wantBuffer: []string{"", "gamma"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a put of a whitespace-only line ends on its last character",
			code: []string{"a", "   ", "b"}, start: Position{Line: 1, Col: 0}, input: "yyp",
			wantBuffer: []string{"a", "   ", "   ", "b"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_UndoOfLinewiseChangeLandsLikeNVim specifies where u
// leaves the cursor for the linewise changes, against nvimShiftReference. nvim
// restores the column the command was issued from, except that a one-line
// linewise shift or delete stops at the first non-blank of the restored line,
// and a line with no non-blank is left exactly where it was. A counted linewise
// change and a linewise put keep the column, and a character-wise change always
// keeps it. Redo puts the buffer back without moving the cursor, so it keeps
// the undo landing.
func TestSimulateEditing_UndoOfLinewiseChangeLandsLikeNVim(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "dd then u stops at the first non-blank of the restored line",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u keeps a column before the first non-blank",
			code: base, start: Position{Line: 1, Col: 1}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u keeps column zero",
			code: base, start: Position{Line: 1, Col: 0}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u clamps a column past the first non-blank",
			code: base, start: Position{Line: 1, Col: 5}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u on the first line stops at its first non-blank",
			code: base, start: Position{Line: 0, Col: 3}, input: "ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then u on a change on the last line",
			code: []string{"alpha", "  beta", "gamma", "  delta"}, start: Position{Line: 3, Col: 3}, input: "ddu",
			wantBuffer: []string{"alpha", "  beta", "gamma", "  delta"},
			wantCursor: Position{Line: 3, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then dd then u then u keeps the deepest landing",
			code: base, start: Position{Line: 1, Col: 3}, input: "dddduu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the first of two undos lands on the shorter line",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddddu",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted delete keeps the issued column on undo",
			code: base, start: Position{Line: 1, Col: 3}, input: "2ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted delete on the first line keeps the issued column",
			code: base, start: Position{Line: 0, Col: 3}, input: "2ddu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p then u keeps the issued column",
			code: base, start: Position{Line: 1, Col: 3}, input: "yypu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then P then u keeps the issued column",
			code: base, start: Position{Line: 1, Col: 3}, input: "yyPu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yy then p then p then u then u keeps the issued column",
			code: base, start: Position{Line: 1, Col: 3}, input: "yyppuu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then p then u keeps the column the put was issued from",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddpu",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dd then P then u keeps the column the put was issued from",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddPu",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a one-line right shift keeps the same landing on undo",
			code: base, start: Position{Line: 1, Col: 3}, input: ">>u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a one-line left shift keeps the same landing on undo",
			code: base, start: Position{Line: 1, Col: 3}, input: "<<u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a right shift of an unindented line undoes to column zero",
			code: base, start: Position{Line: 0, Col: 3}, input: ">>u",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted right shift keeps the issued column on undo",
			code: base, start: Position{Line: 1, Col: 3}, input: "2>>u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of a character delete keeps the issued column",
			code: base, start: Position{Line: 1, Col: 3}, input: "xu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of D keeps the issued column",
			code: base, start: Position{Line: 1, Col: 3}, input: "Du",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo leaves a restored whitespace-only line where it was",
			code: []string{"alpha", "   ", "gamma"}, start: Position{Line: 1, Col: 1}, input: "ddu",
			wantBuffer: []string{"alpha", "   ", "gamma"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of a whitespace-only first line keeps column zero",
			code: []string{"alpha", "   ", "gamma"}, start: Position{Line: 0, Col: 2}, input: "ddu",
			wantBuffer: []string{"alpha", "   ", "gamma"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo keeps the cursor where the undo left it",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddu\x12",
			wantBuffer: []string{"alpha", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a second undo after a redo keeps the same landing",
			code: base, start: Position{Line: 1, Col: 3}, input: "ddu\x12u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_CountedDeleteToEndIsRefused pins the one count the engine
// refuses instead of modelling. nvim runs [count]D as a linewise delete from
// the cursor to the end of the line count-1 lines below, which this engine does
// not implement, and on a single line nvim's 2D is a no-op. Accepting either as
// a result would score an answer Vim never produces, so the whole answer is
// unrecognised and the buffer is left untouched. A count of one is exactly
// Vim's D - verified against nvimShiftReference at every column of one-, two-
// and three-line buffers - and keeps working.
func TestSimulateEditing_CountedDeleteToEndIsRefused(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "a count on D is unrecognised",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 0}, input: "2D",
			wantBuffer: []string{"abcdef"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted D leaves the cursor where it started",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 1}, input: "2D",
			wantBuffer: []string{"abcdef"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted D across two lines is unrecognised",
			code: []string{"abc", "def"}, start: Position{Line: 0, Col: 0}, input: "2D",
			wantBuffer: []string{"abc", "def"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted D across three lines is unrecognised",
			code: []string{"abc", "def", "ghi"}, start: Position{Line: 0, Col: 0}, input: "3D",
			wantBuffer: []string{"abc", "def", "ghi"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted D on a later line is unrecognised",
			code: []string{"abc", "def", "ghi"}, start: Position{Line: 1, Col: 0}, input: "2D",
			wantBuffer: []string{"abc", "def", "ghi"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted D stops the answer after earlier commands",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 0}, input: "x2D",
			wantBuffer: []string{"bcdef"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a count of one is Vim's D",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 0}, input: "1D",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a bare D is unaffected",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 0}, input: "D",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count of one deletes to the end like D",
			code: []string{"abcdef"}, start: Position{Line: 0, Col: 1}, input: "1D",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count of one on a later line deletes to the end like D",
			code: []string{"abc", "def", "ghi"}, start: Position{Line: 1, Col: 1}, input: "1D",
			wantBuffer: []string{"abc", "d", "ghi"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_CountedLinewiseOnLastLineIsNoOp encodes an nvim
// behaviour that is easy to get wrong and that this engine got wrong: a counted
// linewise operator is a COMPLETE no-op when the cursor starts on the last
// line of the buffer. nvim abandons the whole command instead of clamping the
// count to the single line under the cursor, and it does so for dd, yy, >> and
// << alike, leaving the buffer, the cursor, the unnamed register and the undo
// history untouched.
//
// Every row was read from nvimShiftReference (nvim --clean --headless,
// shiftwidth=2 expandtab tabstop=2 startofline), one fresh process per case,
// the buffer loaded from a file and the keys run with :normal!. The rows record
// nvim's resulting buffer and cursor verbatim. The control rows are the other
// half of the rule: one line further from the end, or with a count of one, nvim
// still applies the operator, and a count larger than the remaining lines
// clamps to the lines that remain rather than becoming a no-op.
//
// The starting column matters: the no-op leaves the cursor exactly where it
// was, so a case that starts at column 2 stays at column 2 and is not pulled to
// the first non-blank the way an applied >> or dd is.
func TestSimulateEditing_CountedLinewiseOnLastLineIsNoOp(t *testing.T) {
	runEditingCases(t, []editingCase{
		{
			name: "2dd on the last line of a two-line buffer is a no-op",
			code: []string{"a", "b"}, start: Position{Line: 1, Col: 0}, input: "2dd",
			wantBuffer: []string{"a", "b"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd on the last line of a three-line buffer is a no-op",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2dd",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3dd on the last line is a no-op even though the count exceeds the buffer",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "3dd",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9dd on the last line is a no-op",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "9dd",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd on a one-line buffer is a no-op",
			code: []string{"a"}, start: Position{Line: 0, Col: 0}, input: "2dd",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd on the last line leaves the cursor at its starting column",
			code: []string{"a", "b", "gamma"}, start: Position{Line: 2, Col: 2}, input: "2dd",
			wantBuffer: []string{"a", "b", "gamma"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd on an empty last line is a no-op",
			code: []string{"a", ""}, start: Position{Line: 1, Col: 0}, input: "2dd",
			wantBuffer: []string{"a", ""},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2yy on the last line is a no-op",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2yy",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op 2yy leaves nothing for p to put",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2yyp",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2>> on the last line is a no-op",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2>>",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2<< on the last line is a no-op",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 2, Col: 2}, input: "2<<",
			wantBuffer: []string{"  a", "  b", "  c"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9>> on the last line is a no-op",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 2, Col: 2}, input: "9>>",
			wantBuffer: []string{"  a", "  b", "  c"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9<< on the last line is a no-op",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 2, Col: 2}, input: "9<<",
			wantBuffer: []string{"  a", "  b", "  c"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op shift keeps a column before the first non-blank",
			code: []string{"alpha", "beta", "  gamma"}, start: Position{Line: 2, Col: 0}, input: "2>>",
			wantBuffer: []string{"alpha", "beta", "  gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op delete leaves nothing to undo",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "9ddu",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op shift leaves nothing to undo",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2>>u",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a no-op yank leaves nothing to undo",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "2yyu",
			wantBuffer: []string{"a", "b", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		// Control rows: one line further from the end the operator still
		// applies, and a count past the end clamps to the lines that remain.
		{
			name: "2dd one line from the end deletes the remaining two lines",
			code: []string{"a", "b", "c"}, start: Position{Line: 1, Col: 0}, input: "2dd",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2dd from the first line deletes the first two lines",
			code: []string{"a", "b", "c"}, start: Position{Line: 0, Col: 0}, input: "2dd",
			wantBuffer: []string{"c"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "3dd one line from the end clamps to the two remaining lines",
			code: []string{"a", "b", "c"}, start: Position{Line: 1, Col: 0}, input: "3dd",
			wantBuffer: []string{"a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9dd two lines from the end clamps to the two remaining lines",
			code: []string{"a", "b", "c", "d"}, start: Position{Line: 2, Col: 0}, input: "9dd",
			wantBuffer: []string{"a", "b"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2>> one line from the end shifts the remaining two lines",
			code: []string{"a", "b", "c"}, start: Position{Line: 1, Col: 0}, input: "2>>",
			wantBuffer: []string{"a", "  b", "  c"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9>> one line from the end shifts the remaining two lines",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 1, Col: 2}, input: "9>>",
			wantBuffer: []string{"  a", "    b", "    c"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "9>> from the first line shifts every line",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 0, Col: 2}, input: "9>>",
			wantBuffer: []string{"    a", "    b", "    c"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2<< one line from the end outdents the remaining two lines",
			code: []string{"  a", "  b", "  c"}, start: Position{Line: 1, Col: 2}, input: "2<<",
			wantBuffer: []string{"  a", "b", "c"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "2yy one line from the end still fills the register",
			code: []string{"a", "b", "c"}, start: Position{Line: 1, Col: 0}, input: "2yyp",
			wantBuffer: []string{"a", "b", "b", "c", "c"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count of one on the last line still deletes it",
			code: []string{"a", "b", "c"}, start: Position{Line: 2, Col: 0}, input: "dd",
			wantBuffer: []string{"a", "b"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// ===========================================================================
// INSERT MODE
// ===========================================================================

// nvimInsertReference names the reference implementation and the exact settings
// every insert expectation below was read from: nvim v0.12.5 run as
// "nvim --clean --headless" with "set shiftwidth=2 expandtab tabstop=2
// startofline", a fresh process per case, the buffer loaded from a file and the
// keys fed with feedkeys(..., "x"). A fresh process is required because
// scripted input coalesces undo blocks, which hides how many undos a session
// costs and where each one lands.
//
// 'autoindent' is on in this build (-u NONE reports the same), and that is what
// makes o and O inherit the current line's indentation. The observations
// encoded below:
//
//   - Insert mode's cursor is the insertion point. col('.')-1 during insert mode
//     is the index the next typed rune goes at, and typing advances it; at the
//     end of a line it is one past the last rune.
//   - i inserts at the cursor, a after it, I at the first non-blank, A at the
//     end of the line, o on a new line below and O on a new line above with the
//     cursor at the start of that line.
//   - I on an all-blank line inserts after the blanks (at the line's length),
//     not on the last blank.
//   - o and O inherit the current line's indentation: "\tfoo" becomes two
//     spaces under 'expandtab' with 'tabstop=2', a whitespace-only line keeps
//     its blanks and an empty line inherits none.
//   - Leaving insert mode moves the cursor one column left, unless it is at the
//     start of the line. No typed character is needed for the move: "i<Esc>"
//     alone still lands one column left.
//   - Undo of an insert session restores the buffer and the cursor the session
//     started from - the insertion point for i/a/I/A, the position the command
//     was issued from for o/O - clamped into the restored line. That clamp is
//     why A on "  beta" lands on the last character rather than one past it.
//   - One session is one undo block and one redo; a session that changed
//     nothing is neither, and it does not break the redo stack.
//   - [count]i and [count]o repeat the inserted text count times in Vim ("2iAB"
//     produces "ABAB"), which this engine does not model, so a counted entry
//     command is refused rather than applied once.
//
// The trainer's Esc key is EscToken: the answer is a flat typed string and Esc
// is the trainer's global exit key, so every case spells leaving insert mode
// with the token.

// blankInsertBuffer is the canonical fixture with an empty middle line, so A, I,
// o and O can be observed on an empty line.
func blankInsertBuffer() []string {
	return []string{"alpha", "", "gamma"}
}

// allBlankInsertBuffer has a line of only blanks, where I has no first
// non-blank to land on.
func allBlankInsertBuffer() []string {
	return []string{"   ", "x"}
}

// tabIndentInsertBuffer has a tab-indented line, so o and O prove the inherited
// indentation is measured in columns and written back as spaces.
func tabIndentInsertBuffer() []string {
	return []string{"alpha", "\tfoo", "gamma"}
}

// TestSimulateEditing_InsertEntries specifies where each entry command starts
// its insert session and where the escape token leaves the cursor, against
// nvimInsertReference.
func TestSimulateEditing_InsertEntries(t *testing.T) {
	base := multiLineBuffer()
	blank := blankInsertBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "i inserts at the cursor", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken,
			wantBuffer: []string{"aXYlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a inserts after the cursor", code: base, start: Position{Line: 0, Col: 1},
			input:      "aXY" + EscToken,
			wantBuffer: []string{"alXYpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "I inserts at the first non-blank", code: base, start: Position{Line: 1, Col: 3},
			input:      "IXY" + EscToken,
			wantBuffer: []string{"alpha", "  XYbeta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "I from inside the indentation reaches the same place", code: base, start: Position{Line: 1, Col: 1},
			input:      "IXY" + EscToken,
			wantBuffer: []string{"alpha", "  XYbeta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "I on an empty line inserts at column zero", code: blank, start: Position{Line: 1, Col: 0},
			input:      "IXY" + EscToken,
			wantBuffer: []string{"alpha", "XY", "gamma"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "I on an all-blank line inserts after the blanks", code: allBlankInsertBuffer(), start: Position{Line: 0, Col: 1},
			input:      "IXY" + EscToken,
			wantBuffer: []string{"   XY", "x"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "A appends at the end of the line", code: base, start: Position{Line: 0, Col: 1},
			input:      "AXY" + EscToken,
			wantBuffer: []string{"alphaXY", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "A on an empty line inserts at column zero", code: blank, start: Position{Line: 1, Col: 0},
			input:      "AXY" + EscToken,
			wantBuffer: []string{"alpha", "XY", "gamma"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o opens a line below with the current indentation", code: base, start: Position{Line: 1, Col: 3},
			input:      "oXY" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "  XY", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o on the last line appends", code: base, start: Position{Line: 3, Col: 0},
			input:      "oXY" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta", "XY"},
			wantCursor: Position{Line: 4, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o on an empty line leaves that line alone", code: blank, start: Position{Line: 1, Col: 0},
			input:      "oXY" + EscToken,
			wantBuffer: []string{"alpha", "", "XY", "gamma"},
			wantCursor: Position{Line: 2, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o keeps the blanks of an all-blank line", code: allBlankInsertBuffer(), start: Position{Line: 0, Col: 1},
			input:      "oXY" + EscToken,
			wantBuffer: []string{"   ", "   XY", "x"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o rewrites a tab indent as spaces", code: tabIndentInsertBuffer(), start: Position{Line: 1, Col: 3},
			input:      "oXY" + EscToken,
			wantBuffer: []string{"alpha", "\tfoo", "  XY", "gamma"},
			wantCursor: Position{Line: 2, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "O opens a line above with the current indentation", code: base, start: Position{Line: 1, Col: 3},
			input:      "OXY" + EscToken,
			wantBuffer: []string{"alpha", "  XY", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "O on the first line prepends", code: base, start: Position{Line: 0, Col: 0},
			input:      "OXY" + EscToken,
			wantBuffer: []string{"XY", "alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "O on an empty line inserts above it", code: blank, start: Position{Line: 1, Col: 0},
			input:      "OXY" + EscToken,
			wantBuffer: []string{"alpha", "XY", "", "gamma"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_InsertTyping specifies what typing does inside an insert
// session: a printable rune goes in at the insertion point and advances it, and
// anything the engine does not model leaves the answer unrecognised instead of
// silently typing nothing. The rows that stop before the token are the ones that
// pin the insert cursor, which nvim reports as col('.')-1 while insert mode is
// still open.
func TestSimulateEditing_InsertTyping(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "typing advances the insertion point and stays in insert mode",
			code: base, start: Position{Line: 0, Col: 1}, input: "iXY",
			wantBuffer: []string{"aXYlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeInsert, wantRec: true,
		},
		{
			name: "A leaves the insertion point one past the last rune",
			code: base, start: Position{Line: 0, Col: 1}, input: "AZ",
			wantBuffer: []string{"alphaZ", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeInsert, wantRec: true,
		},
		{
			name: "i inserts before the character under the cursor",
			code: base, start: Position{Line: 0, Col: 4}, input: "iZ",
			wantBuffer: []string{"alphZa", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeInsert, wantRec: true,
		},
		{
			name: "a space is text in insert mode", code: base, start: Position{Line: 0, Col: 1}, input: "iX Y",
			wantBuffer: []string{"aX Ylpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeInsert, wantRec: true,
		},
		{
			name: "a digit is text in insert mode, not a count", code: base, start: Position{Line: 0, Col: 1}, input: "i2",
			wantBuffer: []string{"a2lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: true,
		},
		{
			name: "digits before the token are still text", code: base, start: Position{Line: 0, Col: 1}, input: "i2x" + EscToken,
			wantBuffer: []string{"a2xlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		// An unmodelled byte inside insert mode is not swallowed: the answer is
		// reported unrecognised and the characters typed before it stay applied,
		// exactly as an unparsable normal-mode command behaves.
		{
			name: "a control byte in insert mode is unrecognised", code: base, start: Position{Line: 0, Col: 1}, input: "iX\x04",
			wantBuffer: []string{"aXlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: false,
		},
		{
			name: "ctrl-r in insert mode is unrecognised, not the redo of normal mode",
			code: base, start: Position{Line: 0, Col: 1}, input: "iX\x12",
			wantBuffer: []string{"aXlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: false,
		},
		{
			name: "a newline in insert mode is unrecognised", code: base, start: Position{Line: 0, Col: 1}, input: "iX\n",
			wantBuffer: []string{"aXlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: false,
		},
		{
			name: "a tab in insert mode is unrecognised", code: base, start: Position{Line: 0, Col: 1}, input: "iX\t",
			wantBuffer: []string{"aXlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: false,
		},
		{
			name: "backspace in insert mode is unrecognised: the interface owns it",
			code: base, start: Position{Line: 0, Col: 1}, input: "iX\x7f",
			wantBuffer: []string{"aXlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeInsert, wantRec: false,
		},
	})
}

// TestSimulateEditing_LeavingInsertMode specifies the escape token's landing
// against nvimInsertReference: one column left, never past the start of the
// line, with no typed character required for the move.
func TestSimulateEditing_LeavingInsertMode(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "i alone still lands one column left", code: base, start: Position{Line: 0, Col: 1},
			input:      "i" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "typing at the start of the line leaves the cursor there", code: base, start: Position{Line: 0, Col: 0},
			input:      "iX" + EscToken,
			wantBuffer: []string{"Xalpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a alone lands back on the character", code: base, start: Position{Line: 0, Col: 1},
			input:      "a" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "A alone stays on the last character", code: base, start: Position{Line: 0, Col: 1},
			input:      "A" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "A alone on an empty line stays at column zero", code: blankInsertBuffer(), start: Position{Line: 1, Col: 0},
			input:      "A" + EscToken,
			wantBuffer: []string{"alpha", "", "gamma"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o alone leaves an empty line and the cursor at its start", code: base, start: Position{Line: 1, Col: 3},
			input:      "o" + EscToken,
			wantBuffer: []string{"alpha", "  beta", "", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "O alone leaves an empty line above", code: base, start: Position{Line: 1, Col: 3},
			input:      "O" + EscToken,
			wantBuffer: []string{"alpha", "", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o alone on an all-blank line leaves an empty line", code: allBlankInsertBuffer(), start: Position{Line: 0, Col: 1},
			input:      "o" + EscToken,
			wantBuffer: []string{"   ", "", "x"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "o alone on a tab-indented line leaves an empty line", code: tabIndentInsertBuffer(), start: Position{Line: 1, Col: 3},
			input:      "o" + EscToken,
			wantBuffer: []string{"alpha", "\tfoo", "", "gamma"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a typed space keeps the auto-indent", code: base, start: Position{Line: 1, Col: 3},
			input:      "o " + EscToken,
			wantBuffer: []string{"alpha", "  beta", "   ", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_InsertUndoRedo specifies insert undo granularity and the
// cursor each undo restores, against nvimInsertReference. One session is one
// undo block however many runes were typed, a session that changed nothing is
// not a block, and a second u undoes the session before it.
func TestSimulateEditing_InsertUndoRedo(t *testing.T) {
	base := multiLineBuffer()
	blank := blankInsertBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "one u undoes the whole session", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a second u has nothing left to undo", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "uu",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an a session lands on the insertion point", code: base, start: Position{Line: 0, Col: 1},
			input:      "aXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an A session clamps to the last character", code: base, start: Position{Line: 1, Col: 3},
			input:      "AXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an I session lands on the first non-blank", code: base, start: Position{Line: 1, Col: 4},
			input:      "IXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an I session on an all-blank line lands on the last blank", code: allBlankInsertBuffer(), start: Position{Line: 0, Col: 1},
			input:      "IX" + EscToken + "u",
			wantBuffer: []string{"   ", "x"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an i session on an empty line lands at column zero", code: blank, start: Position{Line: 1, Col: 0},
			input:      "iX" + EscToken + "u",
			wantBuffer: []string{"alpha", "", "gamma"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an o session returns to the line it was issued from", code: base, start: Position{Line: 1, Col: 3},
			input:      "oXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undo of an O session returns to the line it was issued from", code: base, start: Position{Line: 1, Col: 3},
			input:      "OXY" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "two sessions are two undo steps", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "iZ" + EscToken + "u",
			wantBuffer: []string{"aXYlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "two u undo both sessions", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "iZ" + EscToken + "uu",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a session that changed nothing is not an undo step", code: base, start: Position{Line: 0, Col: 1},
			input:      "x" + "i" + EscToken + "u",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a session that changed nothing does not break redo", code: base, start: Position{Line: 0, Col: 1},
			input:      "x" + "u" + "i" + EscToken + "\x12",
			wantBuffer: []string{"apha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo restores the whole session where the undo left the cursor", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "u\x12",
			wantBuffer: []string{"aXYlpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "redo of an o session restores the line", code: base, start: Position{Line: 1, Col: 3},
			input:      "oXY" + EscToken + "u\x12",
			wantBuffer: []string{"alpha", "  beta", "  XY", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the undo landing ignores a motion in between", code: base, start: Position{Line: 0, Col: 1},
			input:      "iXY" + EscToken + "wu",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_EscTokenInNormalModeIsNoOp pins the trainer's one
// non-Vim rule: the escape token means nothing in normal mode, so it is
// consumed and the answer stays recognised rather than becoming an error. The
// count row is Vim's own behaviour, read from nvimInsertReference: an Esc in
// normal mode abandons a pending count, so 2<Esc>x deletes one character.
func TestSimulateEditing_EscTokenInNormalModeIsNoOp(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "the token alone changes nothing", code: base, start: Position{Line: 0, Col: 1},
			input:      EscToken,
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the token after a command is a no-op", code: base, start: Position{Line: 0, Col: 0},
			input:      "x" + EscToken,
			wantBuffer: []string{"lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the token before a command is a no-op", code: base, start: Position{Line: 0, Col: 0},
			input:      EscToken + "x",
			wantBuffer: []string{"lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the token between two commands is a no-op", code: base, start: Position{Line: 0, Col: 0},
			input:      "x" + EscToken + "x",
			wantBuffer: []string{"pha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the token abandons a pending count", code: base, start: Position{Line: 0, Col: 0},
			input:      "2" + EscToken + "x",
			wantBuffer: []string{"lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_CountedInsertEntryIsRefused pins the one insert shape the
// engine refuses instead of modelling. Vim repeats the inserted text [count]
// times - nvimInsertReference turns "2iAB" into "ABAB" and "2oAB" into two
// lines - so accepting the count by ignoring it would score an answer Vim never
// produces. The refused answer is left untouched, like a count on D.
func TestSimulateEditing_CountedInsertEntryIsRefused(t *testing.T) {
	base := multiLineBuffer()

	for _, entry := range []string{"iAB", "aAB", "IAB", "AAB", "oAB", "OAB"} {
		t.Run("2"+entry, func(t *testing.T) {
			runEditingCases(t, []editingCase{
				{
					name: "counted entry is refused", code: base, start: Position{Line: 1, Col: 3},
					input:      "2" + entry + EscToken,
					wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
					wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: false,
				},
			})
		})
	}
}

// TestBufferJudge_AcceptsEquivalentInsertAnswers proves the buffer judge decides
// insert answers by the result they leave: A and $a reach the same buffer and
// cursor by different keys, so both are correct and neither has to be listed in
// the exercise's Solutions.
func TestBufferJudge_AcceptsEquivalentInsertAnswers(t *testing.T) {
	exercise := &Exercise{
		ID:             "insert_equivalent",
		Code:           []string{"alpha"},
		CursorPos:      Position{Line: 0, Col: 1},
		Optimal:        "AXY" + EscToken,
		Solutions:      []string{"AXY" + EscToken},
		BufferVerified: true,
	}

	target := SimulateEditing(exercise.Code, exercise.CursorPos, exercise.Optimal)
	if target.Buffer[0] != "alphaXY" || target.Cursor != (Position{Line: 0, Col: 6}) || target.Mode != ModeNormal {
		t.Fatalf("optimal result = buffer %#v cursor %+v mode %v, want alphaXY at 0:6 in normal mode",
			target.Buffer, target.Cursor, target.Mode)
	}

	// A different, unlisted route to the same result.
	equivalent := "$aXY" + EscToken
	if IsInSolutions(exercise, equivalent) {
		t.Fatalf("%q is listed as a solution; the row must prove the judge, not the fast path", equivalent)
	}

	result := ValidateAnswerDetailed(exercise, equivalent)
	if !result.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(%q).IsCorrect = false, want true; mismatch = %q",
			equivalent, result.MismatchSummary())
	}

	// An answer that reaches a different result is still rejected.
	result = ValidateAnswerDetailed(exercise, "$aXY")
	if result.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(%q).IsCorrect = true, want false", "$aXY")
	}
}

// TestBufferJudge_SeparatesModesForInsertAnswers is the mode clause with a real
// answer instead of a synthetic result: the answer leaves the same buffer and
// the same cursor as the optimal but in ModeNormal where the optimal stops in
// ModeInsert, so the judge must reject it and name the mode.
func TestBufferJudge_SeparatesModesForInsertAnswers(t *testing.T) {
	exercise := &Exercise{
		ID:             "insert_mode_divergence",
		Code:           []string{"alpha"},
		CursorPos:      Position{Line: 0, Col: 1},
		Optimal:        "iX",
		BufferVerified: true,
	}

	answer := "iX" + EscToken + "l"

	optimal := SimulateEditing(exercise.Code, exercise.CursorPos, exercise.Optimal)
	actual := SimulateEditing(exercise.Code, exercise.CursorPos, answer)
	if optimal.Buffer[0] != actual.Buffer[0] {
		t.Fatalf("buffers differ: optimal %#v, answer %#v", optimal.Buffer, actual.Buffer)
	}
	if optimal.Cursor != actual.Cursor {
		t.Fatalf("cursors differ: optimal %+v, answer %+v", optimal.Cursor, actual.Cursor)
	}
	if optimal.Mode != ModeInsert || actual.Mode != ModeNormal {
		t.Fatalf("modes = optimal %v, answer %v; want %v and %v", optimal.Mode, actual.Mode, ModeInsert, ModeNormal)
	}

	result := ValidateAnswerDetailed(exercise, answer)
	if result.IsCorrect {
		t.Error("ValidateAnswerDetailed(...).IsCorrect = true, want false")
	}
	if result.TargetMode != ModeInsert || result.ActualMode != ModeNormal {
		t.Errorf("result modes = %v/%v, want %v/%v", result.TargetMode, result.ActualMode, ModeInsert, ModeNormal)
	}
	if summary := result.MismatchSummary(); summary != "mode differs" {
		t.Errorf("MismatchSummary() = %q, want %q", summary, "mode differs")
	}
}

// nvimRegisterReference names the reference implementation and the exact
// settings every expectation in the register tests below was read from: nvim
// v0.12.5 run as "nvim --clean --headless" with "set shiftwidth=2 expandtab
// tabstop=2 startofline", one fresh process per case and the buffer loaded from
// a file. The cases record nvim's resulting buffer and cursor verbatim.
//
// The observations that shaped the engine, beyond the obvious buffer contents:
//
//   - yy, y$, yw and yj leave the cursor where it was; yiw, yaw and ygg move it
//     to the start of the yanked text when that start is before the cursor.
//   - A character-wise put inserts after the cursor with p and before it with P,
//     and lands on the last inserted character in both cases.
//   - A linewise put inserts below with p and above with P, keeps the stored
//     indentation, and lands on the first non-blank of the first inserted line.
//   - Register 0 holds the last yank; a delete changes the unnamed register but
//     never 0. A named register is filled alongside the unnamed one, and a
//     named yank leaves 0 alone.
//   - yj on the last line and yk on the first line are complete no-ops, as is a
//     put from an empty register.
const nvimRegisterReference = "nvim --clean --headless, shiftwidth=2 expandtab tabstop=2 startofline"

// TestSimulateEditing_YankWithMotion specifies y followed by a motion and yy
// against nvimRegisterReference. Every row observes the register through a put,
// because the register is the engine's state and the put is what makes its
// linewise/charwise flag and its text visible.
func TestSimulateEditing_YankWithMotion(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "yy leaves the cursor where it was",
			code: base, start: Position{Line: 1, Col: 4}, input: "yy",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yiw yanks the inner word charwise and moves to its start",
			code: base, start: Position{Line: 1, Col: 3}, input: "yiw",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yiw at the start of the word does not move the cursor",
			code: base, start: Position{Line: 1, Col: 2}, input: "yiw",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a charwise yank puts after the cursor",
			code: base, start: Position{Line: 1, Col: 2}, input: "yiwp",
			wantBuffer: []string{"alpha", "  bbetaeta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a charwise yank puts before the cursor with P",
			code: base, start: Position{Line: 1, Col: 2}, input: "yiwP",
			wantBuffer: []string{"alpha", "  betabeta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y$ yanks from the cursor to the end of the line",
			code: base, start: Position{Line: 1, Col: 1}, input: "y$p",
			wantBuffer: []string{"alpha", "   betabeta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y$ then P puts the range before the cursor",
			code: base, start: Position{Line: 1, Col: 1}, input: "y$P",
			wantBuffer: []string{"alpha", "  beta beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yw yanks to the start of the next word",
			code: base, start: Position{Line: 0, Col: 0}, input: "ywp",
			wantBuffer: []string{"aalphalpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yj yanks two whole lines linewise and leaves the cursor",
			code: base, start: Position{Line: 1, Col: 3}, input: "yj",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yj then p pastes the two lines below",
			code: base, start: Position{Line: 1, Col: 3}, input: "yjp",
			wantBuffer: []string{"alpha", "  beta", "  beta", "gamma", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "ygg yanks back to the first line and moves the cursor there",
			code: base, start: Position{Line: 2, Col: 1}, input: "ygg",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yj on the last line is a no-op",
			code: base, start: Position{Line: 3, Col: 0}, input: "yj",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "yk on the first line is a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "yk",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an incomplete register prefix is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a register name the engine does not model is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"1p",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
	})
}

// TestSimulateEditing_OperatorDeletes specifies d followed by a motion against
// nvimRegisterReference. A character-wise range is refused when it crosses
// lines, and a count before the operator is refused outright, because Vim
// multiplies the motion there and this engine does not model the product; both
// refusals leave the buffer untouched and report the answer unrecognized,
// exactly like the counted D the engine already refuses.
func TestSimulateEditing_OperatorDeletes(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "d$ deletes from the cursor to the end of the line",
			code: base, start: Position{Line: 0, Col: 1}, input: "d$",
			wantBuffer: []string{"a", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d$ then p restores the line charwise",
			code: base, start: Position{Line: 0, Col: 1}, input: "d$p",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dw deletes to the start of the next word",
			code: base, start: Position{Line: 0, Col: 0}, input: "dw",
			wantBuffer: []string{"", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "diw deletes the inner word",
			code: base, start: Position{Line: 1, Col: 3}, input: "diw",
			wantBuffer: []string{"alpha", "  ", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "diw then p restores the word",
			code: base, start: Position{Line: 1, Col: 3}, input: "diwp",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dj deletes two whole lines linewise",
			code: base, start: Position{Line: 1, Col: 3}, input: "dj",
			wantBuffer: []string{"alpha", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dG deletes to the last line",
			code: base, start: Position{Line: 1, Col: 3}, input: "dG",
			wantBuffer: []string{"alpha"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dgg deletes back to the first line",
			code: base, start: Position{Line: 1, Col: 3}, input: "dgg",
			wantBuffer: []string{"gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "dj on the last line is a no-op",
			code: base, start: Position{Line: 3, Col: 0}, input: "dj",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted operator plus motion is refused",
			code: base, start: Position{Line: 0, Col: 0}, input: "2dw",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
	})
}

// TestSimulateEditing_Registers specifies the unnamed, 0, named a-z and +
// registers against nvimRegisterReference. The paired rows are the point: 0
// keeps the last yank while a delete takes over the unnamed register, and a
// named register survives the yanks that pass through the unnamed one.
func TestSimulateEditing_Registers(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "a named yank fills the named register and survives another yank",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"ayyjyy\"ap",
			wantBuffer: []string{"alpha", "  beta", "alpha", "gamma", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a named yank also fills the unnamed register",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"ayyp",
			wantBuffer: []string{"alpha", "alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a named delete fills the named register",
			code: base, start: Position{Line: 1, Col: 3}, input: "\"addk\"ap",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "register 0 keeps the last yank after a delete changed the unnamed one",
			code: base, start: Position{Line: 0, Col: 0}, input: "yyjdd\"0p",
			wantBuffer: []string{"alpha", "gamma", "alpha", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the unnamed register follows a delete",
			code: base, start: Position{Line: 0, Col: 0}, input: "yyjddp",
			wantBuffer: []string{"alpha", "gamma", "  beta", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x fills the unnamed register charwise",
			code: base, start: Position{Line: 0, Col: 0}, input: "xp",
			wantBuffer: []string{"lapha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted x fills the unnamed register with every deleted rune",
			code: base, start: Position{Line: 0, Col: 0}, input: "3xp",
			wantBuffer: []string{"halpa", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "D fills the unnamed register charwise",
			code: base, start: Position{Line: 0, Col: 1}, input: "Dp",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x does not touch register 0",
			code: base, start: Position{Line: 0, Col: 0}, input: "x\"0p",
			wantBuffer: []string{"lpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the + register aliases the unnamed register on a yank",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"+yyp",
			wantBuffer: []string{"alpha", "alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "the + register aliases the unnamed register on a put",
			code: base, start: Position{Line: 0, Col: 0}, input: "yy\"+p",
			wantBuffer: []string{"alpha", "alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an empty register makes p a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "p",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an empty register 0 makes the prefixed put a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"0p",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an empty named register makes the prefixed put a no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: "\"ap",
			wantBuffer: []string{"alpha", "  beta", "gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestBufferJudge_AcceptsEquivalentOperatorAnswers proves the point of the
// character-wise register: an operator answer such as d$ now leaves a buffer
// the judge can compare. D and d$ reach the same buffer and cursor by different
// keys, so d$ is correct without being listed in the exercise's Solutions.
func TestBufferJudge_AcceptsEquivalentOperatorAnswers(t *testing.T) {
	exercise := &Exercise{
		ID:             "operator_dollar",
		Code:           []string{"alpha", "  beta"},
		CursorPos:      Position{Line: 0, Col: 1},
		Optimal:        "D",
		Solutions:      []string{"D"},
		BufferVerified: true,
	}

	target := SimulateEditing(exercise.Code, exercise.CursorPos, exercise.Optimal)
	if !reflect.DeepEqual(target.Buffer, []string{"a", "  beta"}) || target.Cursor != (Position{Line: 0, Col: 0}) {
		t.Fatalf("optimal result = buffer %#v cursor %+v, want [a  beta] at 0:0", target.Buffer, target.Cursor)
	}

	equivalent := "d$"
	if IsInSolutions(exercise, equivalent) {
		t.Fatalf("%q is listed as a solution; the row must prove the judge, not the fast path", equivalent)
	}

	result := ValidateAnswerDetailed(exercise, equivalent)
	if !result.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(%q).IsCorrect = false, want true; mismatch = %q",
			equivalent, result.MismatchSummary())
	}

	// An answer that deletes fewer characters still reaches a different buffer.
	if wrong := ValidateAnswerDetailed(exercise, "x"); wrong.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(%q).IsCorrect = true, want false", "x")
	}
}

// TestSimulateEditing_MatchPairMotion specifies % inside the editing engine. It
// is the motion simulator's % adopted through the shared motion parser: the same
// jump, the same recognition, and no buffer change. The reference observations
// are the ones recorded on TestSimulateMotions_MatchPairPercent (nvim 0.12.5,
// `nvim --clean --headless -u NONE --cmd 'set shiftwidth=2 expandtab tabstop=2
// startofline'`, default 'matchpairs'), and the engine must land where the
// simulator lands for the same keys.
func TestSimulateEditing_MatchPairMotion(t *testing.T) {
	base := []string{
		"func main() {",
		"  return",
		"}",
	}

	runEditingCases(t, []editingCase{
		{
			name: "on { jumps to its }",
			code: base, start: Position{Line: 0, Col: 12}, input: "%",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "on } jumps back to its {",
			code: base, start: Position{Line: 2, Col: 0}, input: "%",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 12}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "on ( jumps to its )",
			code: []string{"if (a) {}"}, start: Position{Line: 0, Col: 3}, input: "%",
			wantBuffer: []string{"if (a) {}"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "not on a bracket scans forward on the line",
			code: []string{"x = foo(a)"}, start: Position{Line: 0, Col: 0}, input: "%",
			wantBuffer: []string{"x = foo(a)"},
			wantCursor: Position{Line: 0, Col: 9}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an unmatched bracket is a recognized no-op",
			code: []string{"if (a {"}, start: Position{Line: 0, Col: 3}, input: "%",
			wantBuffer: []string{"if (a {"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "no bracket on the line is a recognized no-op",
			code: []string{"plain text"}, start: Position{Line: 0, Col: 0}, input: "%",
			wantBuffer: []string{"plain text"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim's [count]% is the percent-of-file motion, a different
			// command, so the engine refuses it instead of jumping twice.
			name: "counted percent is refused",
			code: base, start: Position{Line: 0, Col: 12}, input: "2%",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 12}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "percent then a mutation acts on the bracket it reached",
			code: []string{"x = foo(a)"}, start: Position{Line: 0, Col: 0}, input: "%x",
			wantBuffer: []string{"x = foo(a"},
			wantCursor: Position{Line: 0, Col: 8}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_Marks specifies the editor's mark commands. Marks need a
// table that survives between two commands, which is state the stateless motion
// simulator does not have, so they live only in the editing engine.
//
// Reference: nvim 0.12.5, one case per process, the buffer loaded from a file:
//
//	nvim --clean --headless -u NONE \
//	  --cmd 'set shiftwidth=2 expandtab tabstop=2 startofline' <file> \
//	  -c 'lua ...' -c 'qa!'
//
// with the keys under observation run through `:normal!` and the mark read back
// with nvim_buf_get_mark. Observed there: m{a-z} records the cursor; `a jumps to
// that exact position and 'a to the first non-blank of the marked line, or
// column 0 when the line has none; a line inserted at or above the mark shifts
// it down and a line inserted below it leaves it alone; deleting lines above it
// lifts it; deleting the marked line itself unsets the mark, and a jump to an
// unset mark is nvim's E20, which leaves the cursor where it was (the engine
// keeps its convention of a recognized no-op for a recognized command that
// cannot act); a count in front of m or of a mark jump is ignored; and same-line
// character edits do not move the mark's column, so nothing here adjusts one.
//
// Undo and redo are not modeled: nvim re-applies the line insertion or deletion
// an undo performs to the mark as well, which this engine does not do.
func TestSimulateEditing_Marks(t *testing.T) {
	base := []string{"alpha", "  bravo", "charlie", "delta"}

	runEditingCases(t, []editingCase{
		{
			name: "backtick jumps to the exact marked position",
			code: base, start: Position{Line: 2, Col: 3}, input: "magg`a",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "backtick keeps the exact column, not the first non-blank",
			code: base, start: Position{Line: 1, Col: 4}, input: "ma`a",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "single quote jumps to the first non-blank of the marked line",
			code: base, start: Position{Line: 1, Col: 4}, input: "ma0'a",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "single quote lands at column 0 on a blank marked line",
			code: []string{"alpha", "   ", "charlie"}, start: Position{Line: 1, Col: 1}, input: "ma'a",
			wantBuffer: []string{"alpha", "   ", "charlie"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a line inserted above the mark shifts it down",
			code: base, start: Position{Line: 2, Col: 2}, input: "maggO" + EscToken + "`a",
			wantBuffer: []string{"", "alpha", "  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 3, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a line inserted below the mark leaves it alone",
			code: base, start: Position{Line: 1, Col: 3}, input: "mao" + EscToken + "`a",
			wantBuffer: []string{"alpha", "  bravo", "", "charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a linewise put above the mark shifts it down",
			code: base, start: Position{Line: 1, Col: 0}, input: "mayyP`a",
			wantBuffer: []string{"alpha", "  bravo", "  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 2, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "deleting lines above the mark lifts it",
			code: base, start: Position{Line: 2, Col: 1}, input: "maggdd`a",
			wantBuffer: []string{"  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "deleting lines below the mark leaves it alone",
			code: base, start: Position{Line: 1, Col: 0}, input: "maGdd`a",
			wantBuffer: []string{"alpha", "  bravo", "charlie"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "deleting the marked line unsets the mark, so the jump does nothing",
			code: base, start: Position{Line: 1, Col: 0}, input: "madd`a",
			wantBuffer: []string{"alpha", "charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a linewise operator that spans the marked line unsets it",
			code: base, start: Position{Line: 1, Col: 0}, input: "madj`a",
			wantBuffer: []string{"alpha", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an unset mark is a recognized no-op",
			code: base, start: Position{Line: 2, Col: 2}, input: "`a",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "an insert session keeps the mark where it was",
			code: base, start: Position{Line: 1, Col: 0}, input: "maiXY" + EscToken + "`a",
			wantBuffer: []string{"alpha", "XY  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count in front of a mark command is ignored, as nvim ignores it",
			code: base, start: Position{Line: 1, Col: 0}, input: "2ma",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count in front of a mark jump is ignored, as nvim ignores it",
			code: base, start: Position{Line: 1, Col: 2}, input: "magg2`a",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a mark name outside a-z is not a command",
			code: base, start: Position{Line: 1, Col: 0}, input: "mA",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "m with no name is not a complete command",
			code: base, start: Position{Line: 1, Col: 0}, input: "m",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a jump name outside a-z is not a command",
			code: base, start: Position{Line: 1, Col: 0}, input: "`A",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a counted linewise delete above the mark lifts it by the count",
			code: base, start: Position{Line: 3, Col: 0}, input: "magg2dd`a",
			wantBuffer: []string{"charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted linewise put above the mark shifts it by every inserted line",
			code: base, start: Position{Line: 1, Col: 0}, input: "maggyy2G3P`a",
			wantBuffer: []string{"alpha", "alpha", "alpha", "alpha", "  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 4, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "deleting one marked line leaves the other mark alone",
			code: base, start: Position{Line: 2, Col: 0}, input: "maggmbdd`a",
			wantBuffer: []string{"  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "single quote lands on the first non-blank after a line was inserted above",
			code: base, start: Position{Line: 1, Col: 4}, input: "maggO" + EscToken + "'a",
			wantBuffer: []string{"", "alpha", "  bravo", "charlie", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a mutation after a mark jump acts at the mark",
			code: base, start: Position{Line: 2, Col: 2}, input: "magg`ax",
			wantBuffer: []string{"alpha", "  bravo", "chrlie", "delta"},
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_ShiftOperatorMotion specifies > and < as operators over a
// motion, against nvimShiftReference. The range is delegated to the shared
// motion parser, so the operator spans exactly the lines the motion judge
// reaches, and each line is rewritten by shiftIndent, so a line shifted here is
// the line >> produces for it.
//
// Reference: nvim 0.12.5, "nvim --clean --headless" with `set shiftwidth=2
// expandtab tabstop=2 startofline`, one fresh process per case, the buffer
// loaded from a file, the cursor placed with cursor(). The observations the rows
// below encode are:
//
//   - The operator is linewise whatever the motion is. >w shifts the whole
//     cursor line, and >} shifts every line from the cursor to the line the
//     paragraph motion reached, skipping any empty line inside the range the
//     way >> does.
//   - The cursor lands on the first non-blank of the first line of the range
//     ('startofline'), which is the line the motion reached when the motion went
//     upwards and the cursor's own line when it went downwards.
//   - Undo does not use that landing. A shifted range undoes to the column the
//     command was issued from, even when the range was a single line: >wu from
//     column 8 of "func main() {" returns to column 8, while >>u from the same
//     place stops at the first non-blank, because >> is the one-line linewise
//     case the existing undo rule names.
//   - A motion nvim treats as failing aborts the command before it touches the
//     buffer, so the keys are consumed and the answer stays recognized: >j on
//     the last line, >k on the first, and >fZ when the line holds no Z are
//     no-ops. Reaching a legal position without moving the cursor is not a
//     failure: >l at the end of a line, >h in column one, >0 and >$ and >t(
//     where ( is the next character all shift the cursor's line. A counted j or
//     k that moves part of its count still shifts, so >3j from the
//     second-to-last line shifts to the last.
//   - A count before the operator is refused, exactly as the d and y path
//     refuses 2dw: nvim multiplies it into the motion's own count (2>j and >2j
//     shift the same three lines, and 2>G from the first line is >2G, which
//     shifts to line 2 rather than to the last line), and this engine does not
//     model the product. A count on the motion itself is the parser's own, so
//     >2j shifts the cursor line and the two below it.
func TestSimulateEditing_ShiftOperatorMotion(t *testing.T) {
	base := []string{
		"func main() {",
		"  if x {",
		"    y",
		"  }",
		"  z",
		"}",
	}

	runEditingCases(t, []editingCase{
		{
			name: ">j shifts the cursor line and the line below it",
			code: base, start: Position{Line: 0, Col: 0}, input: ">j",
			wantBuffer: []string{"  func main() {", "    if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">w shifts the whole cursor line even though w is character-wise",
			code: base, start: Position{Line: 0, Col: 0}, input: ">w",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">G shifts every line down to the last",
			code: base, start: Position{Line: 0, Col: 0}, input: ">G",
			wantBuffer: []string{"  func main() {", "    if x {", "      y", "    }", "    z", "  }"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">gg shifts from the first line to the cursor's line",
			code: base, start: Position{Line: 4, Col: 2}, input: ">gg",
			wantBuffer: []string{"  func main() {", "    if x {", "      y", "    }", "    z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">} shifts to the last line when the buffer has no blank line",
			code: base, start: Position{Line: 1, Col: 2}, input: ">}",
			wantBuffer: []string{"func main() {", "    if x {", "      y", "    }", "    z", "  }"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">} stops at the blank line the paragraph motion reached",
			code: []string{"func main() {", "  a", "", "  b", "}"}, start: Position{Line: 1, Col: 2}, input: ">}",
			wantBuffer: []string{"func main() {", "    a", "", "  b", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<} outdents the line the paragraph motion reached and the cursor line",
			code: []string{"func main() {", "  a", "", "  b", "}"}, start: Position{Line: 3, Col: 2}, input: "<}",
			wantBuffer: []string{"func main() {", "  a", "", "b", "}"},
			wantCursor: Position{Line: 3, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">2j shifts three lines, the cursor line and the two below it",
			code: base, start: Position{Line: 0, Col: 0}, input: ">2j",
			wantBuffer: []string{"  func main() {", "    if x {", "      y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted j that only partly moves still shifts to the last line",
			code: base, start: Position{Line: 4, Col: 2}, input: ">3j",
			wantBuffer: []string{"func main() {", "  if x {", "    y", "  }", "    z", "  }"},
			wantCursor: Position{Line: 4, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<j outdents the cursor line and the line below it",
			code: base, start: Position{Line: 0, Col: 0}, input: "<j",
			wantBuffer: []string{"func main() {", "if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<G outdents every line down to the last",
			code: base, start: Position{Line: 0, Col: 0}, input: "<G",
			wantBuffer: []string{"func main() {", "if x {", "  y", "}", "z", "}"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<gg outdents from the first line to the cursor's line",
			code: base, start: Position{Line: 4, Col: 2}, input: "<gg",
			wantBuffer: []string{"func main() {", "if x {", "  y", "}", "z", "}"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">k shifts the line k reached and the cursor line",
			code: base, start: Position{Line: 2, Col: 2}, input: ">k",
			wantBuffer: []string{"func main() {", "    if x {", "      y", "  }", "  z", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<k outdents the line k reached and the cursor line",
			code: base, start: Position{Line: 2, Col: 6}, input: "<k",
			wantBuffer: []string{"func main() {", "if x {", "  y", "  }", "  z", "}"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<2j outdents the cursor line and the two below it",
			code: base, start: Position{Line: 1, Col: 2}, input: "<2j",
			wantBuffer: []string{"func main() {", "if x {", "  y", "}", "  z", "}"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">j on the last line is a complete no-op",
			code: base, start: Position{Line: 5, Col: 0}, input: ">j",
			wantBuffer: base,
			wantCursor: Position{Line: 5, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">G on the last line still shifts the line it already reached",
			code: base, start: Position{Line: 5, Col: 0}, input: ">G",
			wantBuffer: []string{"func main() {", "  if x {", "    y", "  }", "  z", "  }"},
			wantCursor: Position{Line: 5, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">gg on the first line still shifts the line it already reached",
			code: base, start: Position{Line: 0, Col: 0}, input: ">gg",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">g is an incomplete g command and is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: ">g",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: ">2 leaves the operator without a motion and is not recognized",
			code: base, start: Position{Line: 0, Col: 0}, input: ">2",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: ">; with no previous find is a recognized no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: ">;",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">; aborts when the repeated find's target is gone",
			code: base, start: Position{Line: 0, Col: 0}, input: "fa>;",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">; shifts the line when the repeated find reaches a target",
			code: []string{"alpha beta"}, start: Position{Line: 0, Col: 0}, input: "fa>;",
			wantBuffer: []string{"  alpha beta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">, repeats the find backwards and shifts the line",
			code: []string{"alpha beta"}, start: Position{Line: 0, Col: 0}, input: "fa>,",
			wantBuffer: []string{"  alpha beta"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">k on the first line is a complete no-op",
			code: base, start: Position{Line: 0, Col: 0}, input: ">k",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">j on a two-line buffer shifts both lines",
			code: []string{"a", "b"}, start: Position{Line: 0, Col: 0}, input: ">j",
			wantBuffer: []string{"  a", "  b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">j on a single-line buffer is a no-op",
			code: []string{"only line"}, start: Position{Line: 0, Col: 0}, input: ">j",
			wantBuffer: []string{"only line"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">w on a single-line buffer shifts the line",
			code: []string{"only line"}, start: Position{Line: 0, Col: 0}, input: ">w",
			wantBuffer: []string{"  only line"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">l at the end of a line still shifts the line",
			code: base, start: Position{Line: 0, Col: 12}, input: ">l",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">h in column one still shifts the line",
			code: base, start: Position{Line: 0, Col: 0}, input: ">h",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">0 shifts the cursor line from a column past the first blank",
			code: base, start: Position{Line: 1, Col: 6}, input: ">0",
			wantBuffer: []string{"func main() {", "    if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">$ shifts the cursor line to its end",
			code: base, start: Position{Line: 1, Col: 6}, input: ">$",
			wantBuffer: []string{"func main() {", "    if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">fa shifts the cursor line when the target is found",
			code: base, start: Position{Line: 0, Col: 0}, input: ">fa",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">fZ is a no-op when the line holds no Z",
			code: base, start: Position{Line: 0, Col: 0}, input: ">fZ",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">t( shifts the line when the target is the next character",
			code: base, start: Position{Line: 0, Col: 3}, input: ">t(",
			wantBuffer: []string{"  func main() {", "  if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">ju returns to the column the command was issued from",
			code: base, start: Position{Line: 0, Col: 0}, input: ">ju",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">wu returns to the issued column even though the range was one line",
			code: base, start: Position{Line: 0, Col: 8}, input: ">wu",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 8}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">>u still stops at the first non-blank of the changed line",
			code: base, start: Position{Line: 1, Col: 6}, input: ">>u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a count before the operator is refused",
			code: base, start: Position{Line: 0, Col: 0}, input: "2>j",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "a count before the operator is refused for G too",
			code: base, start: Position{Line: 0, Col: 0}, input: "2>G",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: ">2j shifts a whitespace-only line in its range, as >> does",
			code: []string{"a", "   ", "b"}, start: Position{Line: 0, Col: 0}, input: ">2j",
			wantBuffer: []string{"  a", "     ", "  b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">2j skips an empty line in its range, as >> does",
			code: []string{"a", "", "b"}, start: Position{Line: 0, Col: 0}, input: ">2j",
			wantBuffer: []string{"  a", "", "  b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">j stops before an empty line outside its range",
			code: []string{"a", "", "b"}, start: Position{Line: 0, Col: 0}, input: ">j",
			wantBuffer: []string{"  a", "", "b"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">2j on an all-blank first line lands on its last character",
			code: []string{"   ", "a"}, start: Position{Line: 0, Col: 0}, input: ">2j",
			wantBuffer: []string{"     ", "  a"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<2j on an all-blank first line lands on its last character",
			code: []string{"   ", "a"}, start: Position{Line: 0, Col: 0}, input: "<2j",
			wantBuffer: []string{" ", "a"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a range shift leaves a mark where it was set",
			code: base, start: Position{Line: 0, Col: 0}, input: "ma>j`a",
			wantBuffer: []string{"  func main() {", "    if x {", "    y", "  }", "  z", "}"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a range shift leaves the unnamed register alone",
			code: []string{"alpha", "beta", "gamma"}, start: Position{Line: 0, Col: 0}, input: "yy>jp",
			wantBuffer: []string{"  alpha", "alpha", "  beta", "gamma"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_OperatorMotionPercent specifies % accepted after an
// operator. The span runs from the cursor to the bracket the shared motion
// parser's % reaches, and it is the operator's own kind of range:
//
//   - > and < are linewise, so >% shifts every line between the two brackets
//     however far apart they are and lands on the first non-blank of the first
//     of them, while its undo returns to the issued column like every other
//     operator-plus-motion shift.
//   - d% and y% act on the inclusive character-wise span. A span that stays on
//     one line is the engine's ordinary character-wise range, so y%p and y%P put
//     it back under the same rules as the d$ and y$ the engine already stores.
//   - A % that reaches no bracket is a recognized no-op: nvim leaves the buffer
//     and the cursor exactly as they were for >%, d% and y%.
//   - A character-wise span that crosses lines is refused exactly as every other
//     crossing range is, because the engine keeps character-wise content on one
//     line. nvim's own result there needs a register holding newlines (d% on the
//     "call(" of a four-line argument list leaves the text before the opening
//     bracket, "  call"), which is a register model this engine does not have,
//     so the answer is reported unrecognized and nothing is applied.
//   - A counted % is refused on this path as it already is as a bare motion:
//     nvim's [count]% is percent-of-file, a different command.
//
// Reference: nvim 0.12.5, "nvim --clean --headless" with `set shiftwidth=2
// expandtab tabstop=2 startofline`, one fresh process per case, the buffer
// loaded from a file, the cursor placed with cursor().
func TestSimulateEditing_OperatorMotionPercent(t *testing.T) {
	brackets := []string{
		"func main() {",
		"  call(",
		"    a,",
		"    b",
		"  )",
		"}",
	}
	noBracket := []string{"plain text here", "second line"}

	runEditingCases(t, []editingCase{
		{
			name: ">% on the opening bracket shifts every line the span covers",
			code: brackets, start: Position{Line: 1, Col: 6}, input: ">%",
			wantBuffer: []string{"func main() {", "    call(", "      a,", "      b", "    )", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">% on the closing bracket shifts the same lines and lands on the opening one",
			code: brackets, start: Position{Line: 4, Col: 2}, input: ">%",
			wantBuffer: []string{"func main() {", "    call(", "      a,", "      b", "    )", "}"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "<% outdents every line the span covers",
			code: brackets, start: Position{Line: 1, Col: 6}, input: "<%",
			wantBuffer: []string{"func main() {", "call(", "  a,", "  b", ")", "}"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undoing a cross-line >% returns to the issued column",
			code: brackets, start: Position{Line: 1, Col: 6}, input: ">%u",
			wantBuffer: brackets,
			wantCursor: Position{Line: 1, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% across lines is refused by the one-line character-wise rule",
			code: brackets, start: Position{Line: 1, Col: 6}, input: "d%",
			wantBuffer: brackets,
			wantCursor: Position{Line: 1, Col: 6}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: "y% across lines is refused by the one-line character-wise rule",
			code: brackets, start: Position{Line: 1, Col: 6}, input: "y%",
			wantBuffer: brackets,
			wantCursor: Position{Line: 1, Col: 6}, wantMode: ModeNormal, wantRec: false,
		},
		{
			name: ">% on a line with no bracket is a recognized no-op",
			code: noBracket, start: Position{Line: 0, Col: 6}, input: ">%",
			wantBuffer: noBracket,
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% on a line with no bracket is a recognized no-op",
			code: noBracket, start: Position{Line: 0, Col: 6}, input: "d%",
			wantBuffer: noBracket,
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y% on a line with no bracket is a recognized no-op",
			code: noBracket, start: Position{Line: 0, Col: 6}, input: "y%",
			wantBuffer: noBracket,
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% on the opening bracket deletes the inclusive span",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: "d%",
			wantBuffer: []string{"call"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% on the closing bracket deletes the same span",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 9}, input: "d%",
			wantBuffer: []string{"call"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% from before the bracket scans forward and deletes to its match",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 0}, input: "d%",
			wantBuffer: []string{""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y% on the opening bracket leaves the cursor on the bracket",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: "y%",
			wantBuffer: []string{"call(a, b)"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y% on the closing bracket backs the cursor up to the span's start",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 9}, input: "y%",
			wantBuffer: []string{"call(a, b)"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y% then p puts the span after the cursor",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: "y%p",
			wantBuffer: []string{"call((a, b)a, b)"},
			wantCursor: Position{Line: 0, Col: 10}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "y% then P puts the span before the cursor",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: "y%P",
			wantBuffer: []string{"call(a, b)(a, b)"},
			wantCursor: Position{Line: 0, Col: 9}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">% on a same-line span shifts the whole line",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: ">%",
			wantBuffer: []string{"  call(a, b)"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">% from a column before the bracket scans forward to it",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 0}, input: ">%",
			wantBuffer: []string{"  call(a, b)"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "d% on an unmatched bracket is a recognized no-op",
			code: []string{"if (a {", "b"}, start: Position{Line: 0, Col: 3}, input: "d%",
			wantBuffer: []string{"if (a {", "b"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: ">% on an unmatched bracket is a recognized no-op",
			code: []string{"if (a {", "b"}, start: Position{Line: 0, Col: 3}, input: ">%",
			wantBuffer: []string{"if (a {", "b"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "a counted percent after an operator is refused",
			code: []string{"call(a, b)"}, start: Position{Line: 0, Col: 4}, input: "d2%",
			wantBuffer: []string{"call(a, b)"},
			wantCursor: Position{Line: 0, Col: 4}, wantMode: ModeNormal, wantRec: false,
		},
	})
}

// Visual mode is specified against a reference, not reasoned about:
// nvim --clean --headless with `set shiftwidth=2 expandtab tabstop=2
// startofline`, a fresh process per case, and the buffer loaded from a file.
// The blockwise cases were typed key by key because a block selection is a live
// mode: which columns it covers when the lines have different lengths, what
// happens when it reaches past the end of a short line, where the cursor lands
// after each operation, whether I skips a line it cannot reach, whether A pads
// a short line, and the top-to-bottom order the insert applies to were all read
// from that reference. The observation each case encodes is named in its own
// comment.
const blockVisual = string(blockVisualKey)

// blockVisualBuffer is the fixture with three different line lengths used by the
// blockwise specifications: long, short, medium. The short middle line is what
// makes the column and padding rules observable.
func blockVisualBuffer() []string {
	return []string{"abcdef", "ab", "abcd"}
}

func TestSimulateEditing_VisualLineSelectsWholeLines(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "V enters linewise visual and keeps the cursor",
			code: base, start: Position{Line: 1, Col: 2}, input: "V",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeVisualLine,
			wantSel: Selection{StartLine: 1, StartCol: 0, EndLine: 1, EndCol: 5, Active: true},
			wantRec: true,
		},
		{
			name: "V then j extends to the next whole line",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vj",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 2}, wantMode: ModeVisualLine,
			wantSel: Selection{StartLine: 1, StartCol: 0, EndLine: 2, EndCol: 4, Active: true},
			wantRec: true,
		},
		{
			name: "V then k extends up and the selection is normalised",
			code: base, start: Position{Line: 2, Col: 3}, input: "Vk",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 3}, wantMode: ModeVisualLine,
			wantSel: Selection{StartLine: 1, StartCol: 0, EndLine: 2, EndCol: 4, Active: true},
			wantRec: true,
		},
		{
			name: "the escape token leaves visual line mode in normal mode",
			code: base, start: Position{Line: 1, Col: 2}, input: "V" + EscToken,
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal,
			wantSel: Selection{}, wantRec: true,
		},
	})
}

// TestSimulateEditing_VisualLineOperations pins the linewise operators. The
// reference is the pty run of nvim with the settings above: Vjd and Vjx delete
// the selected lines and land on the first non-blank of the line that takes
// their place, Vjy leaves the cursor at the first line of the selection, Vj>
// and Vj< shift the whole lines, and Vjc replaces them with the first line's
// indentation plus the typed text.
func TestSimulateEditing_VisualLineOperations(t *testing.T) {
	base := multiLineBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "Vjd deletes the selected lines",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vjd",
			wantBuffer: []string{"alpha", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vjd from the first line deletes down",
			code: base, start: Position{Line: 0, Col: 0}, input: "Vjd",
			wantBuffer: []string{"gamma", "delta"},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vjx is the same delete",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vjx",
			wantBuffer: []string{"alpha", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vjy yanks the lines and leaves the cursor on the first of them",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vjy",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vj> shifts the selected lines right",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vj>",
			wantBuffer: []string{"alpha", "    beta", "  gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 4}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vj< shifts the selected lines left",
			code: base, start: Position{Line: 1, Col: 2}, input: "Vj<",
			wantBuffer: []string{"alpha", "beta", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vjc replaces the lines with the first line's indentation plus typed text",
			code: base, start: Position{Line: 1, Col: 2}, input: "VjcX" + EscToken,
			wantBuffer: []string{"alpha", "  X", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "Vc changes the single selected line",
			code: base, start: Position{Line: 1, Col: 2}, input: "VcX" + EscToken,
			wantBuffer: []string{"alpha", "  X", "gamma", "delta"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

func TestSimulateEditing_VisualBlockSelectsColumns(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "Ctrl-v enters blockwise visual on the cursor's cell",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual,
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 2, Active: true},
			wantRec: true,
		},
		{
			name: "j extends the block and the cursor rests one past a short line",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "j",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 1, EndCol: 2, Active: true},
			wantRec: true,
		},
		{
			name: "jjl extends the block down and right",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jjl",
			wantBuffer: base,
			wantCursor: Position{Line: 2, Col: 3}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 2, EndCol: 3, Active: true},
			wantRec: true,
		},
		{
			// nvim observation: with the cursor on the last rune, l moves one
			// column past it, and the block covers that virtual column.
			name: "l reaches one column past the last rune",
			code: []string{"ab"}, start: Position{Line: 0, Col: 1}, input: blockVisual + "l",
			wantBuffer: []string{"ab"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 2, Active: true},
			wantRec: true,
		},
		{
			name: "the escape token leaves blockwise visual in normal mode",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "j" + EscToken,
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal,
			wantSel: Selection{}, wantRec: true,
		},
	})
}

// TestSimulateEditing_VisualBlockDeleteAndYank pins the block mutations. The
// reference is the same pty run: the block covers the columns between the anchor
// and the cursor, a line too short to reach the block's left edge contributes
// nothing, and the cursor lands on the block's left edge of the first line,
// clamped into that line.
func TestSimulateEditing_VisualBlockDeleteAndYank(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			// nvim: "gg0ll<C-v>jd" left abdef/ab/abcd at line 1, col 3.
			name: "j then d deletes the single column on the lines it reaches",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jd",
			wantBuffer: []string{"abdef", "ab", "abcd"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0ll<C-v>jjld" left abef/ab/ab. The l on the short middle
			// line does not move, so the block is one column on that row.
			name: "the block covers the columns the cursor actually reached",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jjl" + "d",
			wantBuffer: []string{"abef", "ab", "ab"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0lll<C-v>jjd" left abcef/ab/abc.
			name: "a short line contributes nothing to a block that starts past it",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjd",
			wantBuffer: []string{"abcef", "ab", "abc"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "x is the same delete as d",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jl" + "x",
			wantBuffer: []string{"abdef", "ab", "abcd"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0ll<C-v>jly" left the buffer alone and put the cursor on
			// the block's top-left corner.
			name: "y leaves the buffer alone and lands on the top-left corner",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jl" + "y",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0ll<C-v>jjd" on ["  ab","a",""] left b/""/"" at 1,1.
			// The empty last line clamps the cursor to column 0, so the block
			// spans columns 0..2 and the delete starts at column 0.
			name: "an empty line clamps the cursor and widens the block",
			code: []string{"  ab", "a", ""}, start: Position{Line: 0, Col: 2}, input: blockVisual + "jjd",
			wantBuffer: []string{"b", "", ""},
			wantCursor: Position{Line: 0, Col: 0}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "ggjjll<C-v>kd" left abcdef/ab/abd at 2,2. The delete lands
			// on column 2 but "ab" is only two runes long, so it clamps to 2,2.
			name: "the cursor clamps into the first line after a delete",
			code: base, start: Position{Line: 2, Col: 2}, input: blockVisual + "kd",
			wantBuffer: []string{"abcdef", "ab", "abd"},
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_VisualBlockInsert is the point of block mode and the part
// the reference run settles. I inserts at the block's left edge on every line it
// can reach and skips a line that is too short to reach it; A appends after the
// block's right edge on every line and pads a short line with spaces; c deletes
// the block and then inserts at its left edge. All three land the cursor on the
// block's left edge of the first line. The values are the ones the pty run of
// nvim with the settings above printed, repeated twice per case.
func TestSimulateEditing_VisualBlockInsert(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			// nvim: "gg0ll<C-v>jIX<Esc>" abXcdef/abX/abcd at line 1, col 3.
			name: "I inserts at the left edge on every line",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jI" + "X" + EscToken,
			wantBuffer: []string{"abXcdef", "abX", "abcd"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0lll<C-v>jjIX<Esc>" abcXdef/ab/abcXd. The middle line is
			// shorter than the block's left edge, so I leaves it alone rather
			// than padding it.
			name: "I skips a line too short to reach the left edge",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjI" + "X" + EscToken,
			wantBuffer: []string{"abcXdef", "ab", "abcXd"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0ll<C-v>jlAX<Esc>" abcXdef/ab X/abcd.
			name: "A appends after the block's right edge on every line",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jlA" + "X" + EscToken,
			wantBuffer: []string{"abcXdef", "ab X", "abcd"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0lll<C-v>jjAX<Esc>" abcdXef/ab  X/abcdX. The short middle
			// line is padded with spaces so the appended text lines up.
			name: "A pads a short line instead of skipping it",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjA" + "X" + EscToken,
			wantBuffer: []string{"abcdXef", "ab  X", "abcdX"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "A with nothing typed leaves the buffer alone",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjA" + EscToken,
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0ll<C-v>jlcX<Esc>" abXdef/abX/abcd.
			name: "c deletes the block and inserts at its left edge",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jlc" + "X" + EscToken,
			wantBuffer: []string{"abXdef", "abX", "abcd"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "ggjjll<C-v>kIX<Esc>" abcdef/abX/abXcd at 2,3. The anchor is
			// below the cursor here, so the top row is the cursor's line and the
			// typed text is written from the top down.
			name: "I works when the selection was extended upwards",
			code: base, start: Position{Line: 2, Col: 2}, input: blockVisual + "kI" + "X" + EscToken,
			wantBuffer: []string{"abcdef", "abX", "abXcd"},
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0lll<C-v>jjcX<Esc>" abcXef/ab/abcX.
			name: "c skips the short line after deleting the block",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjc" + "X" + EscToken,
			wantBuffer: []string{"abcXef", "ab", "abcX"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "c with nothing typed only deletes the block",
			code: base, start: Position{Line: 0, Col: 3}, input: blockVisual + "jjc" + EscToken,
			wantBuffer: []string{"abcef", "ab", "abc"},
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_VisualBlockYankThenPut verifies the block yank by the only
// thing that can: putting it back. The block register is blockwise, so p and P
// insert each row's cell at the cursor's column on that row, padding a short
// line to reach it. The reference is the pty run: "gg0ll<C-v>jlyp" produced
// abccdef/ab , "$p" produced abcdefc/ab    , and "$P" produced abcdecf/ab   .
func TestSimulateEditing_VisualBlockYankThenPut(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "p puts the block after the cursor, padding a short line",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jl" + "y" + "p",
			wantBuffer: []string{"abccdef", "ab ", "abcd"},
			wantCursor: Position{Line: 0, Col: 3}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "p at the end of the line puts after the last rune",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jl" + "y$p",
			wantBuffer: []string{"abcdefc", "ab    ", "abcd"},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "P puts the block before the cursor",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jl" + "y$P",
			wantBuffer: []string{"abcdecf", "ab   ", "abcd"},
			wantCursor: Position{Line: 0, Col: 5}, wantMode: ModeNormal, wantRec: true,
		},
		{
			// nvim: "gg0lll<C-v>jy$p" on ["abcdef","ab"] left abcdefcd/ab    .
			// The second row's cell is empty, but the put still pads it to the
			// insertion column so the block's shape survives.
			name: "an empty cell still pads its line",
			code: []string{"abcdef", "ab"}, start: Position{Line: 0, Col: 3}, input: blockVisual + "jy$p",
			wantBuffer: []string{"abcdefcd", "ab    "},
			wantCursor: Position{Line: 0, Col: 6}, wantMode: ModeNormal, wantRec: true,
		},
	})
}

// TestSimulateEditing_VisualModeRefusals pins what the engine deliberately does
// not model, so it is refused rather than applied with the wrong meaning. A
// count in front of a visual operator scales a motion in Vim; a blockwise $ does
// not move to the end of the current line but extends the block to the longest
// line of the selection; and a blockwise > shifts lines, which is a different
// operation from the linewise shift this engine models. Each is recognized as
// unrecognized input, never applied as something else.
func TestSimulateEditing_VisualModeRefusals(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "a counted V is refused",
			code: base, start: Position{Line: 0, Col: 2}, input: "2V",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: false,
		},
		{
			// The count is in front of the operator, not in front of a motion:
			// 2jd is a valid Vim answer (move down and delete), while 2d scales a
			// motion this engine does not model.
			name: "a counted block delete is refused",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "2d",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 2, Active: true}, wantRec: false,
		},
		{
			name: "blockwise $ is refused rather than read as end-of-line",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "j$d",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 1, EndCol: 2, Active: true}, wantRec: false,
		},
		{
			name: "blockwise > is refused",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "j>",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 2}, wantMode: ModeVisualBlock,
			wantSel: Selection{StartLine: 0, StartCol: 2, EndLine: 1, EndCol: 2, Active: true}, wantRec: false,
		},
	})
}

// TestSimulateEditing_VisualOperationsUndoAsOneStep triangulates the visual
// operators against the undo model: each is one normal-mode command, so one u
// restores the buffer it found. A blockwise insert is one session block, so a
// single u takes back the text written on every row.
func TestSimulateEditing_VisualOperationsUndoAsOneStep(t *testing.T) {
	base := blockVisualBuffer()

	runEditingCases(t, []editingCase{
		{
			name: "undoing a linewise visual delete restores the lines",
			code: base, start: Position{Line: 0, Col: 2}, input: "Vjdu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undoing a linewise visual shift restores the indentation",
			code: base, start: Position{Line: 0, Col: 2}, input: "Vj>u",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undoing a blockwise delete restores the block",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jdu",
			wantBuffer: base,
			wantCursor: Position{Line: 1, Col: 1}, wantMode: ModeNormal, wantRec: true,
		},
		{
			name: "undoing a blockwise insert restores every row in one step",
			code: base, start: Position{Line: 0, Col: 2}, input: blockVisual + "jI" + "XY" + EscToken + "u",
			wantBuffer: base,
			wantCursor: Position{Line: 0, Col: 2}, wantMode: ModeNormal, wantRec: true,
		},
	})
}
