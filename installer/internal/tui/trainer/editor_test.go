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
// consumed", which is the only thing callers may rely on.
func TestSimulateEditing_RecognitionDiffersFromMotionSimulator(t *testing.T) {
	code := []string{"alpha", "  beta"}

	for _, input := range []string{"dd", "yy", ">>", "<<", "x", "D", "p", "P", "u", "\x12"} {
		if !SimulateEditing(code, Position{}, input).Recognized {
			t.Errorf("SimulateEditing(%q).Recognized = false, want true", input)
		}
	}

	for _, input := range []string{"x", "p", "P", "u", ">>", "<<", "\x12"} {
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
