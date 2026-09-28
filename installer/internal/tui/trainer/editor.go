package trainer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// EscToken is the typed text the trainer's answer input uses for the Esc key.
// The answer is a flat string of typed keys and Esc is the trainer's global exit
// key, so an exercise that needs to leave insert mode cannot receive a real
// escape character. The interface inserts this literal token instead and the
// engine parses it as leaving insert mode wherever it appears, in normal mode as
// well, where it is consumed as a no-op. It is exported so the two cannot
// disagree about it.
const EscToken = "<Esc>"

// Mode is the editing mode the engine is in after an answer. ModeNormal and
// ModeInsert are reachable from the commands this engine understands; the
// remaining constants exist so the visual commands of a following task can be
// added without changing the shape of EditingResult.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeVisual
	ModeVisualLine
	ModeVisualBlock
)

// EditingResult is the state a mutable editing answer leaves behind: the buffer
// after the answer, the cursor, the mode and whether every keystroke was
// consumed. It is the editing counterpart of SimulationResult, and
// Recognized carries the same notion as SimulationResult.Recognized and
// IsRecognizedInput: the whole input was consumed by recognized commands. The
// command sets differ, so the editing engine parses recognized input itself
// instead of routing through the motion simulator, which has no notion of
// buffer mutation and marks x, p, >, < and u as unrecognized.
//
// Cursor is counted in runes. In ModeNormal it is the rune column of the
// character under the cursor; in ModeInsert it is the insertion point, the index
// the next typed rune would occupy, which may sit one past the last rune of its
// line. That is the same position nvim reports as col('.')-1 while insert mode
// is still open, and it is what makes an answer that never leaves insert mode
// comparable at all.
type EditingResult struct {
	Buffer     []string // the buffer after the answer
	Cursor     Position // the cursor after the answer, in runes
	Mode       Mode     // the mode after the answer
	Recognized bool     // false when the answer contains something the engine cannot parse
}

// shiftWidth is one shift for >> and <<, in columns. The trainer's exercises
// are written with two-space indentation, so the engine models 'shiftwidth=2'.
const shiftWidth = 2

// tabStop is the column width of one tab in the indentation model, matching
// 'tabstop=2'. Indentation is a width in columns rather than a count of
// characters, so a leading tab is two columns wide wherever a line places it.
const tabStop = 2

// editingRegister is the unnamed yank/delete register. linewise records whether
// the content is whole lines (yy, dd) or a character range, because p and P mean
// different things for each. Only linewise content exists today: yy and dd fill
// the register, and put always inserts whole lines.
type editingRegister struct {
	lines    []string
	linewise bool
	set      bool
}

// editorSnapshot is one undo history entry: the buffer and the cursor an undo
// restores, plus the cursor rule of the change it undoes. nvim restores the
// column the command was issued from, except for a one-line linewise shift or
// delete, where it stops at the first non-blank of the changed line; cursorFor
// applies that rule.
type editorSnapshot struct {
	buffer []string
	cursor Position
	// startOfLine marks a one-line linewise shift or delete, the only change
	// undo repositions the cursor for.
	startOfLine bool
	// insertSession marks the single snapshot of an insert session, whose undo
	// clamps the stored cursor into the restored line instead of keeping it as
	// it is (see cursorFor).
	insertSession bool
}

// editor is the mutable editing state for a single SimulateEditing call.
type editor struct {
	buffer []string
	cursor Position
	mode   Mode
	undo   []editorSnapshot
	redo   []editorSnapshot
	reg    editingRegister
	// insertStart is the snapshot the open insert session commits when the
	// escape token closes it, and nil in normal mode. It holds the buffer the
	// session started from, so the whole session - the line o and O insert and
	// every rune typed after it - undoes as one step.
	insertStart *editorSnapshot
	// autoIndent is the rune column that ends the indentation an o or O session
	// began with, and autoIndentSet says whether the open session began with
	// one. Leaving such a session without typing anything removes that
	// indentation again (see leaveInsert).
	autoIndent    int
	autoIndentSet bool
	// find is the last f/F/t/T target, so ';' and ',' repeat it exactly as
	// they do in SimulateMotions. The motion parser receives this state.
	find lastFindCommand
}

// SimulateEditing runs input against a mutable copy of code starting at start
// and returns the resulting editing state. It is additive: it never changes the
// behavior of SimulateMotions or SimulateMotionsWithSelection.
//
// The buffer always has at least one line, Vim's model: an empty or nil code is
// simulated as a single empty line, and deleting the last line leaves that empty
// line rather than an empty slice. start is clamped into the buffer, and columns
// are counted in runes.
//
// The input is parsed in whichever mode the answer has reached: normal-mode
// commands until an insert entry (i, a, I, A, o, O) opens an insert session,
// printable runes and EscToken until that session's token closes it, and normal
// mode again after. A byte the engine does not model in the current mode makes
// the whole answer unrecognized.
func SimulateEditing(code []string, start Position, input string) EditingResult {
	e := newEditor(code, start)
	recognized := e.execute(input)
	return e.result(recognized)
}

func newEditor(code []string, start Position) *editor {
	buffer := make([]string, len(code))
	copy(buffer, code)
	if len(buffer) == 0 {
		buffer = []string{""}
	}
	e := &editor{buffer: buffer, cursor: start}
	e.clampCursor()
	return e
}

// clampCursor keeps the cursor inside the buffer: a valid line, and a column in
// runes within that line. In normal mode the column indexes a character, so it
// stops at the last rune; in insert mode it is the insertion point, which may
// sit one past the last rune. An empty line puts the cursor at column 0.
func (e *editor) clampCursor() {
	if len(e.buffer) == 0 {
		e.buffer = []string{""}
	}
	if e.cursor.Line < 0 {
		e.cursor.Line = 0
	}
	if e.cursor.Line >= len(e.buffer) {
		e.cursor.Line = len(e.buffer) - 1
	}

	last := utf8.RuneCountInString(e.buffer[e.cursor.Line]) - 1
	if e.mode == ModeInsert {
		last++
	}
	if last < 0 {
		last = 0
	}
	if e.cursor.Col < 0 {
		e.cursor.Col = 0
	}
	if e.cursor.Col > last {
		e.cursor.Col = last
	}
}

// execute parses input one command at a time and returns whether every keystroke
// was consumed. A normal-mode command is either a mutation this engine
// implements, an insert entry or a motion parsed by the simulator's own motion
// parser, so an answer may reposition the cursor before it mutates, and a motion
// ahead of a mutation acts on the line the motion reached rather than a second,
// divergent motion implementation. Inside an insert session the only commands
// are the escape token, which closes the session, and a printable rune.
//
// A motion changes only the cursor, so it never records an undo snapshot; the
// one-snapshot-per-command rule still holds because only a buffer change
// commits, and an insert session commits its one snapshot when the token closes
// it. On the first keystroke it cannot parse it stops and reports false; the
// commands performed before that point remain applied, because the caller
// decides correctness from Recognized, not from the buffer.
func (e *editor) execute(input string) bool {
	i := 0
	for i < len(input) {
		if e.mode == ModeInsert {
			consumed, ok := e.applyInsert(input[i:])
			if !ok {
				return false
			}
			i += consumed
			continue
		}

		// Parse the count once so a mutation and a motion share the same one.
		// The count never starts with '0': that byte is the first-column
		// motion, which countPrefix leaves for the motion parser.
		count, afterCount := countPrefix(input, i)

		// The trainer's escape token is not a Vim command, so it is consumed here
		// in normal mode as a no-op and the answer stays recognized; a count in
		// front of it is abandoned with it, which is what an Esc does to a pending
		// count in nvim.
		if strings.HasPrefix(input[afterCount:], EscToken) {
			i = afterCount + len(EscToken)
			continue
		}

		// Refuse a count on D outright. Vim's [count]D reaches across lines and
		// is a no-op within a single one, so nothing this engine could compute
		// from it would be the answer Vim produces; an answer like 2D must be
		// unrecognised rather than accepted with a wrong buffer. A count of one
		// is exactly Vim's D (verified against nvim) and is accepted as such.
		if afterCount < len(input) && input[afterCount] == 'D' && count > 1 {
			return false
		}

		if afterCount < len(input) {
			cmd := input[afterCount]
			// An insert entry is one key with no second key and no buffer change of
			// its own, but Vim's [count]i and [count]o repeat the inserted text
			// count times, which this engine does not model, so a counted entry is
			// refused like a counted D instead of inserting the text once.
			if isInsertEntry(cmd) {
				if count > 1 {
					return false
				}
				e.applyInsertEntry(cmd)
				i = afterCount + 1
				continue
			}
			if consumed, ok := e.applyMutation(cmd, count, input[afterCount+1:]); ok {
				i = afterCount + 1 + consumed
				continue
			}
		}

		// Not a mutation or an insert entry: hand the whole token to the shared
		// motion parser, count included, so the engine recognizes exactly the
		// motions SimulateMotions recognizes and lands exactly where it lands. The
		// cursor is adopted unclamped, exactly as the simulator keeps it until
		// the simulation ends, so a later motion starts from the same position
		// the simulator would start from; result clamps once at the end.
		pos, consumed, ok := parseMotion(input[i:], e.motionPosition(), e.buffer, &e.find)
		if !ok {
			return false
		}
		e.cursor = Position{Line: pos.Line, Col: pos.Col}
		i += consumed
	}
	return true
}

// motionPosition is the cursor in the motion simulator's coordinate type.
func (e *editor) motionPosition() SimulatedPosition {
	return SimulatedPosition{Line: e.cursor.Line, Col: e.cursor.Col}
}

// countPrefix parses a normal-mode count prefix at input[from]. Vim counts begin
// with a non-zero digit, so a leading '0' is not a count and is left for the
// motion parser; a count with no digits is 1. It returns the count and the index
// of the first byte after the digits.
func countPrefix(input string, from int) (int, int) {
	i := from
	count := 0
	for i < len(input) && input[i] >= '1' && input[i] <= '9' {
		count = count*10 + int(input[i]-'0')
		i++
	}
	if count == 0 {
		return 1, from
	}
	// After the first digit, 0 can be part of the count (e.g. 10j).
	for i < len(input) && input[i] >= '0' && input[i] <= '9' {
		count = count*10 + int(input[i]-'0')
		i++
	}
	return count, i
}

// applyMutation applies the mutation command cmd with its already-parsed count
// and reports how many bytes of rest the command consumed. ok is false when cmd
// is not a mutation this engine implements, or when a two-key command such as dd
// is missing its second key; the caller then offers the input to the motion
// parser instead. Only the two-key commands (dd, yy, >>, <<) consume a byte of
// rest.
func (e *editor) applyMutation(cmd byte, count int, rest string) (int, bool) {
	switch cmd {
	case 'd', 'y', '>', '<':
		if len(rest) == 0 || rest[0] != cmd {
			return 0, false
		}
		// A counted linewise operator starting on the last line is a complete
		// no-op in nvim: it abandons the command rather than clamping the count
		// to the single line under the cursor, so the buffer, the cursor, the
		// unnamed register and the undo history are all left untouched. The keys
		// are still consumed and the answer stays recognized. A count of one is
		// the ordinary single-line command and never reaches this guard.
		// TestSimulateEditing_CountedLinewiseOnLastLineIsNoOp records the exact
		// nvim reference, settings and observations.
		if count > 1 && e.cursor.Line == len(e.buffer)-1 {
			return 1, true
		}
		switch cmd {
		case 'd':
			e.deleteLines(count)
		case 'y':
			e.yankLines(count)
		case '>':
			e.shiftLines(count, true)
		case '<':
			e.shiftLines(count, false)
		}
		return 1, true
	case 'x':
		e.deleteChars(count)
	case 'D':
		e.deleteToLineEnd()
	case 'p':
		e.put(count, false)
	case 'P':
		e.put(count, true)
	case 'u':
		e.undoStep()
	case '\x12': // Ctrl-r
		e.redoStep()
	default:
		return 0, false
	}
	return 0, true
}

// isInsertEntry reports whether cmd is a normal-mode command that starts an
// insert session: i inserts at the cursor, a after it, I at the first non-blank,
// A at the end of the line, and o and O on a new line below and above.
func isInsertEntry(cmd byte) bool {
	switch cmd {
	case 'i', 'a', 'I', 'A', 'o', 'O':
		return true
	}
	return false
}

// applyInsertEntry starts the insert session cmd opens, at the position nvim's
// reference leaves: i at the cursor, a after it, I at the first non-blank - at
// the end of an all-blank line, where its whitespace ends - and A at the end of
// the line. o and O open a line below and above the current one and inherit its
// indentation, because 'autoindent' is on and 'expandtab' with 'tabstop=2'
// writes that indentation back as spaces. Every entry command is one key.
func (e *editor) applyInsertEntry(cmd byte) {
	line := e.cursor.Line
	switch cmd {
	case 'i':
		e.beginInsert(e.buffer, e.cursor, e.cursor)
	case 'a':
		at := Position{Line: line, Col: e.cursor.Col + 1}
		e.beginInsert(e.buffer, e.clampedInsertPoint(at), at)
	case 'I':
		at := Position{Line: line, Col: firstNonBlankOrEndColumn(e.buffer[line])}
		e.beginInsert(e.buffer, e.clampedInsertPoint(at), at)
	case 'A':
		at := Position{Line: line, Col: utf8.RuneCountInString(e.buffer[line])}
		e.beginInsert(e.buffer, e.clampedInsertPoint(at), at)
	case 'o', 'O':
		e.openLine(cmd == 'O')
	}
}

// openLine implements o (above false) and O (above true): it inserts one line
// holding the current line's indentation below or above the cursor's line and
// starts an insert session with the cursor at the start of that line. The
// session's undo snapshot restores the buffer as it was and the cursor the
// command was issued from, which is where nvim puts it, so the line belongs to
// the session rather than being its own undo step.
func (e *editor) openLine(above bool) {
	line := e.cursor.Line
	indent := strings.Repeat(" ", indentColumns(e.buffer[line]))

	at := line + 1
	if above {
		at = line
	}
	next := make([]string, 0, len(e.buffer)+1)
	next = append(next, e.buffer[:at]...)
	next = append(next, indent)
	next = append(next, e.buffer[at:]...)

	e.beginInsert(next, Position{Line: at, Col: utf8.RuneCountInString(indent)}, e.cursor)
	// beginInsert clears the indent of a previous session, so the auto-indent
	// this one opened with is recorded after it.
	e.autoIndent = utf8.RuneCountInString(indent)
	e.autoIndentSet = true
}

// beginInsert starts an insert session. buffer is the session's starting buffer
// and insertCursor the insertion point in it; sessionCursor is the cursor the
// session's single undo snapshot restores, which is the insertion point for
// i/a/I/A and the position the command was issued from for o/O. The snapshot
// holds the buffer the session started from, so o and O pass their new buffer
// here instead of writing it themselves, and a session that changes nothing
// records no snapshot at all (see leaveInsert).
func (e *editor) beginInsert(buffer []string, insertCursor, sessionCursor Position) {
	e.autoIndentSet = false
	e.insertStart = &editorSnapshot{
		buffer:        copyLines(e.buffer),
		cursor:        sessionCursor,
		insertSession: true,
	}
	e.buffer = buffer
	e.cursor = insertCursor
	e.mode = ModeInsert
}

// clampedInsertPoint keeps an insertion point inside its line. The insertion
// point may sit one past the last rune, but not further: on an empty line even a
// has to insert at column zero, while the value its undo restores stays at the
// column the command was issued from plus one and is clamped only there.
func (e *editor) clampedInsertPoint(at Position) Position {
	length := utf8.RuneCountInString(e.buffer[at.Line])
	if at.Col > length {
		at.Col = length
	}
	if at.Col < 0 {
		at.Col = 0
	}
	return at
}

// applyInsert consumes one key inside an insert session and reports how many
// bytes it took. The escape token is tried first because its five characters are
// all printable and would otherwise be typed as text. A printable rune is
// inserted at the insertion point and advances past it. Every other byte - a
// control character, a newline, a tab, a backspace the interface owns, or an
// invalid encoding - is not modelled in insert mode, so the answer is reported
// unrecognised rather than typing nothing.
func (e *editor) applyInsert(rest string) (int, bool) {
	if strings.HasPrefix(rest, EscToken) {
		e.leaveInsert()
		return len(EscToken), true
	}

	r, size := utf8.DecodeRuneInString(rest)
	if r == utf8.RuneError && size <= 1 {
		return 0, false
	}
	if !unicode.IsPrint(r) {
		return 0, false
	}

	e.insertRune(r)
	return size, true
}

// insertRune inserts r at the insertion point and advances the insertion point
// past it.
func (e *editor) insertRune(r rune) {
	runes := []rune(e.buffer[e.cursor.Line])
	col := e.cursor.Col
	if col > len(runes) {
		col = len(runes)
	}

	next := make([]rune, 0, len(runes)+1)
	next = append(next, runes[:col]...)
	next = append(next, r)
	next = append(next, runes[col:]...)

	e.buffer[e.cursor.Line] = string(next)
	e.cursor.Col = col + 1
}

// leaveInsert is the escape token's effect inside an insert session: a session
// that opened an auto-indented line and never typed on it drops that indentation
// again, which is what nvim does for "o<Esc>" on an indented line, while
// "o <Esc>" typed a space and keeps the indentation; the cursor then moves one
// column left unless it is already at the start of the line; the mode returns to
// normal; and the session commits its one undo snapshot. A session that left the
// buffer as it found it - "i<Esc>" with nothing typed - commits nothing, so it
// neither adds an undo step nor clears the redo stack, which is what nvim does.
func (e *editor) leaveInsert() {
	if e.autoIndentSet && e.cursor.Col == e.autoIndent {
		e.buffer[e.cursor.Line] = ""
		e.cursor.Col = 0
	}
	e.autoIndentSet = false

	e.mode = ModeNormal
	if start := e.insertStart; start != nil && !sameLines(start.buffer, e.buffer) {
		e.undo = append(e.undo, *start)
		e.redo = nil
	}
	e.insertStart = nil
	if e.cursor.Col > 0 {
		e.cursor.Col--
	}
}

// deleteLines implements [count]dd: it yanks the deleted lines into the
// unnamed register as linewise content, removes them, and puts the cursor on
// the first non-blank of the line that took their place.
func (e *editor) deleteLines(count int) {
	start := e.cursor.Line
	end := start + count
	if end > len(e.buffer) {
		end = len(e.buffer)
	}
	e.reg = editingRegister{lines: copyLines(e.buffer[start:end]), linewise: true, set: true}

	remaining := make([]string, 0, len(e.buffer)-(end-start))
	remaining = append(remaining, e.buffer[:start]...)
	remaining = append(remaining, e.buffer[end:]...)
	if len(remaining) == 0 {
		remaining = []string{""}
	}

	cursor := Position{Line: start}
	if cursor.Line >= len(remaining) {
		cursor.Line = len(remaining) - 1
	}
	cursor.Col = startOfLineColumn(remaining[cursor.Line])

	e.commit(remaining, cursor, !sameLines(e.buffer, remaining), count == 1)
}

// yankLines implements [count]yy: it fills the unnamed register with whole
// lines as linewise content and leaves the buffer and cursor untouched.
func (e *editor) yankLines(count int) {
	start := e.cursor.Line
	end := start + count
	if end > len(e.buffer) {
		end = len(e.buffer)
	}
	e.reg = editingRegister{lines: copyLines(e.buffer[start:end]), linewise: true, set: true}
}

// shiftLines implements [count]>> (indent true) and [count]<< (indent false)
// on the current line and the lines below it. An empty line is left alone, but
// a line that holds only whitespace is shifted, exactly as nvim does. The
// cursor lands where nvim's startofline landing puts it on the current line:
// the first non-blank, or the last character of a line that has none.
func (e *editor) shiftLines(count int, indent bool) {
	next := copyLines(e.buffer)
	last := e.cursor.Line + count
	if last > len(next) {
		last = len(next)
	}
	for line := e.cursor.Line; line < last; line++ {
		if next[line] == "" {
			continue
		}
		next[line] = shiftIndent(next[line], indent)
	}

	cursor := Position{Line: e.cursor.Line, Col: startOfLineColumn(next[e.cursor.Line])}
	e.commit(next, cursor, !sameLines(e.buffer, next), count == 1)
}

// deleteChars implements [count]x: it deletes the runes at and after the cursor
// on the current line and clamps the cursor to the last remaining rune. It does
// not touch the unnamed register; character-wise register content is a later
// task.
func (e *editor) deleteChars(count int) {
	runes := []rune(e.buffer[e.cursor.Line])
	if e.cursor.Col >= len(runes) {
		return
	}
	end := e.cursor.Col + count
	if end > len(runes) {
		end = len(runes)
	}

	next := make([]rune, 0, len(runes)-(end-e.cursor.Col))
	next = append(next, runes[:e.cursor.Col]...)
	next = append(next, runes[end:]...)

	buffer := copyLines(e.buffer)
	buffer[e.cursor.Line] = string(next)

	cursor := Position{Line: e.cursor.Line, Col: e.cursor.Col}
	if cursor.Col >= len(next) {
		cursor.Col = len(next) - 1
	}
	if cursor.Col < 0 {
		cursor.Col = 0
	}

	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
}

// deleteToLineEnd implements D: it deletes from the cursor to the end of the
// current line, leaving the cursor on the last remaining rune. An empty line
// has nothing to delete, so the command is a no-op there. A count of one is
// Vim's D, but a larger count makes [count]D span lines, which this engine does
// not model: execute refuses those before they reach here.
func (e *editor) deleteToLineEnd() {
	runes := []rune(e.buffer[e.cursor.Line])
	if e.cursor.Col >= len(runes) {
		return
	}

	next := make([]rune, e.cursor.Col)
	copy(next, runes[:e.cursor.Col])

	buffer := copyLines(e.buffer)
	buffer[e.cursor.Line] = string(next)

	cursor := Position{Line: e.cursor.Line, Col: e.cursor.Col}
	if cursor.Col >= len(next) {
		cursor.Col = len(next) - 1
	}
	if cursor.Col < 0 {
		cursor.Col = 0
	}

	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
}

// put implements [count]p (before false) and [count]P (before true) for linewise
// register content: p inserts the register's lines below the current line, P
// above it, and the cursor lands on the first non-blank of the first inserted
// line. An empty register makes the command a no-op. Only linewise content
// exists today; the register's linewise flag is tracked now so the
// character-wise branch can be added later.
func (e *editor) put(count int, before bool) {
	if !e.reg.set || len(e.reg.lines) == 0 {
		return
	}

	insertAt := e.cursor.Line
	if !before {
		insertAt = e.cursor.Line + 1
	}

	block := make([]string, 0, count*len(e.reg.lines))
	for i := 0; i < count; i++ {
		block = append(block, e.reg.lines...)
	}

	buffer := make([]string, 0, len(e.buffer)+len(block))
	buffer = append(buffer, e.buffer[:insertAt]...)
	buffer = append(buffer, block...)
	buffer = append(buffer, e.buffer[insertAt:]...)

	cursor := Position{Line: insertAt, Col: startOfLineColumn(buffer[insertAt])}
	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
}

// undoStep implements u: one snapshot per normal-mode command. A command that
// did not change the buffer never pushed a snapshot, so an empty history is a
// no-op rather than a way to undo into nothing.
func (e *editor) undoStep() {
	if len(e.undo) == 0 {
		return
	}
	snapshot := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]

	restored := cursorFor(snapshot)
	// Redo restores the buffer and leaves the cursor where the undo left it,
	// which is what nvim does, so the redo entry carries the restored cursor
	// rather than the cursor the command ended on.
	e.redo = append(e.redo, editorSnapshot{
		buffer:        copyLines(e.buffer),
		cursor:        restored,
		startOfLine:   snapshot.startOfLine,
		insertSession: snapshot.insertSession,
	})
	e.buffer = snapshot.buffer
	e.cursor = restored
}

// cursorFor is the cursor an undo of snapshot leaves behind: the column the
// command was issued from, except that a one-line linewise shift or delete
// stops at the first non-blank of the restored line, which is what nvim does
// and what the mutation itself already does. An insert session restores the
// cursor it started from instead, clamped into the restored line: nvim lands A
// on the last character of the line rather than one past it, and an insert that
// began at the end of an all-blank line lands on the last blank.
func cursorFor(snapshot editorSnapshot) Position {
	cursor := snapshot.cursor
	if snapshot.insertSession {
		if cursor.Line < 0 || cursor.Line >= len(snapshot.buffer) {
			return cursor
		}
		return Position{Line: cursor.Line, Col: clampColumn(snapshot.buffer[cursor.Line], cursor.Col)}
	}
	if !snapshot.startOfLine {
		return cursor
	}
	return Position{
		Line: cursor.Line,
		Col:  undoStartOfLineColumn(snapshot.buffer[cursor.Line], cursor.Col),
	}
}

// clampColumn keeps a column inside a restored line, where a normal-mode cursor
// always indexes a character.
func clampColumn(line string, col int) int {
	last := utf8.RuneCountInString(line) - 1
	if last < 0 {
		last = 0
	}
	if col > last {
		return last
	}
	if col < 0 {
		return 0
	}
	return col
}

// redoStep implements Ctrl-r: it reapplies the most recently undone command.
func (e *editor) redoStep() {
	if len(e.redo) == 0 {
		return
	}
	snapshot := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.undo = append(e.undo, editorSnapshot{
		buffer:        copyLines(e.buffer),
		cursor:        e.cursor,
		startOfLine:   snapshot.startOfLine,
		insertSession: snapshot.insertSession,
	})
	e.buffer = snapshot.buffer
	e.cursor = snapshot.cursor
}

// commit applies a command's result. changed reports whether the buffer really
// changed; only a real change records an undo snapshot and clears the redo
// stack, so no-op commands never become undo points. startOfLine marks a
// one-line linewise shift or delete, whose undo stops at the first non-blank of
// the changed line (see cursorFor).
func (e *editor) commit(buffer []string, cursor Position, changed, startOfLine bool) {
	if changed {
		e.undo = append(e.undo, editorSnapshot{
			buffer:      copyLines(e.buffer),
			cursor:      e.cursor,
			startOfLine: startOfLine,
		})
		e.redo = nil
	}
	e.buffer = buffer
	e.cursor = cursor
}

func (e *editor) result(recognized bool) EditingResult {
	// One clamp at the end, exactly where SimulateMotions clamps: an adopted
	// motion cursor is kept unclamped while commands run, so a later motion
	// starts from the same position the simulator would start from. In insert
	// mode the clamp keeps the insertion point at most one past the last rune,
	// where nvim reports col('.')-1 while insert mode is open.
	e.clampCursor()
	return EditingResult{
		Buffer:     copyLines(e.buffer),
		Cursor:     e.cursor,
		Mode:       e.mode,
		Recognized: recognized,
	}
}

// copyLines returns a copy so a result or snapshot can never be mutated by a
// later command.
func copyLines(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

// sameLines reports whether two buffers hold the same lines.
func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstNonBlankColumn returns the rune column of line's first non-blank
// character and whether the line has one; a line of only whitespace has none.
func firstNonBlankColumn(line string) (int, bool) {
	for i, r := range []rune(line) {
		if r != ' ' && r != '\t' {
			return i, true
		}
	}
	return 0, false
}

// firstNonBlankOrEndColumn is where I starts its insert session: the rune column
// of the first non-blank, or the end of the line when there is none. nvim inserts
// after the blanks of an all-blank line, not on its last blank, which is why this
// is not startOfLineColumn.
func firstNonBlankOrEndColumn(line string) int {
	if first, ok := firstNonBlankColumn(line); ok {
		return first
	}
	return utf8.RuneCountInString(line)
}

// startOfLineColumn is where nvim's startofline landing leaves the cursor on a
// line: the first non-blank, or the last character when the line has none.
// Every linewise mutation lands here - dd, >>, <<, p and P - so an all-blank
// line is entered at its last column rather than at column zero.
func startOfLineColumn(line string) int {
	if first, ok := firstNonBlankColumn(line); ok {
		return first
	}
	if runes := []rune(line); len(runes) > 0 {
		return len(runes) - 1
	}
	return 0
}

// undoStartOfLineColumn is where undoing a one-line linewise shift or delete
// leaves the cursor: the column the command was issued from, but never past the
// first non-blank of the changed line. Unlike a mutation's own landing, a line
// with no non-blank is left exactly where it was.
func undoStartOfLineColumn(line string, col int) int {
	if first, ok := firstNonBlankColumn(line); ok && col > first {
		return first
	}
	return col
}

// indentColumns returns the width in columns of line's leading whitespace. A
// space is one column and a tab advances to the next tabstop, so with
// 'tabstop=2' the three characters of "\t\ta" are four columns wide.
func indentColumns(line string) int {
	columns := 0
	for _, r := range line {
		switch r {
		case ' ':
			columns++
		case '\t':
			columns += tabStop - columns%tabStop
		default:
			return columns
		}
	}
	return columns
}

// shiftIndent rewrites line's leading whitespace one shift to the right
// (indent true) or to the left. Indentation is measured and offset in columns
// and written back as spaces, which is what 'expandtab' with 'shiftwidth=2'
// does: >> turns "\tfoo" into four spaces, and << never removes more than the
// indentation a line actually has.
func shiftIndent(line string, indent bool) string {
	columns := indentColumns(line)
	if indent {
		columns += shiftWidth
	} else if columns -= shiftWidth; columns < 0 {
		columns = 0
	}
	return strings.Repeat(" ", columns) + strings.TrimLeft(line, " \t")
}
