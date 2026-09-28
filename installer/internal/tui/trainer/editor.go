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

// editingRegister is one yank/delete register's content. lines holds the whole
// lines of a linewise register, or the single line of a character-wise one, and
// linewise records which, because p and P mean different things for each. set
// separates a register holding an empty string from one that was never filled,
// so a put from an empty register stays a no-op.
type editingRegister struct {
	lines    []string
	linewise bool
	set      bool
}

// editorRegisters is the register file an answer can reach: the unnamed
// register, register 0 (the last yank) and the named a-z registers. Vim's '+'
// register is the system clipboard, which the trainer does not have, so it is
// not stored here and is aliased to the unnamed register when a prefix names
// it.
type editorRegisters struct {
	unnamed editingRegister
	yank    editingRegister
	named   map[byte]editingRegister
}

// registerRef is a parsed '"' register prefix: the register an answer named,
// and whether it named one at all.
type registerRef struct {
	name     byte
	explicit bool
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
	regs   editorRegisters
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
	// marks is the m{a-z} mark table, keyed by mark name. It holds the cursor
	// each mark was set at, in the engine's rune coordinates. Marks live here
	// and not in the motion simulator because they are state that has to
	// survive between two commands, and the simulator is stateless. Undo and
	// redo do not touch this table: nvim re-applies the line insertion or
	// deletion an undo performs to the mark as well, and this engine does not
	// model that.
	marks map[byte]Position
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
	e := &editor{buffer: buffer, cursor: start, marks: make(map[byte]Position)}
	e.regs.named = make(map[byte]editingRegister)
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

		// A '"' prefix names the register the command reads or writes, ahead of
		// the count as Vim writes it ("ayy, "0p, "+y). A prefix with no name,
		// or with a name this engine does not model, is not recognized.
		ref, afterRegister, ok := parseRegisterPrefix(input, i)
		if !ok {
			return false
		}

		// Parse the count once so a mutation and a motion share the same one.
		// The count never starts with '0': that byte is the first-column
		// motion, which countPrefix leaves for the motion parser.
		count, afterCount := countPrefix(input, afterRegister)

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
			if consumed, ok := e.applyMutation(cmd, ref, count, input[afterCount+1:]); ok {
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

// parseRegisterPrefix reads the optional '"' register prefix at input[from]
// and reports the register it names. ok is false for a '"' with no name and
// for a name the engine does not model, so an answer like "1p is unrecognized
// rather than silently read from the unnamed register. The names it accepts are
// the unnamed register ('"'), register 0, the named a-z registers and the '+'
// alias the engine keeps for Vim's system clipboard.
func parseRegisterPrefix(input string, from int) (registerRef, int, bool) {
	if from >= len(input) || input[from] != '"' {
		return registerRef{}, from, true
	}
	if from+1 >= len(input) {
		return registerRef{}, from, false
	}
	name := input[from+1]
	switch {
	case name == '"' || name == '+' || name == '0' || (name >= 'a' && name <= 'z'):
		return registerRef{name: name, explicit: true}, from + 2, true
	}
	return registerRef{}, from, false
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
func (e *editor) applyMutation(cmd byte, ref registerRef, count int, rest string) (int, bool) {
	switch cmd {
	case 'd', 'y', '>', '<':
		if len(rest) == 0 {
			return 0, false
		}
		if rest[0] == cmd {
			// A counted linewise operator starting on the last line is a
			// complete no-op in nvim: it abandons the command rather than
			// clamping the count to the single line under the cursor, so the
			// buffer, the cursor, the unnamed register and the undo history
			// are all left untouched. The keys are still consumed and the
			// answer stays recognized. A count of one is the ordinary
			// single-line command and never reaches this guard.
			// TestSimulateEditing_CountedLinewiseOnLastLineIsNoOp records the
			// exact nvim reference, settings and observations.
			if count > 1 && e.cursor.Line == len(e.buffer)-1 {
				return 1, true
			}
			switch cmd {
			case 'd':
				e.deleteLines(ref, count)
			case 'y':
				e.yankLines(ref, count)
			case '>':
				e.shiftLines(count, true)
			case '<':
				e.shiftLines(count, false)
			}
			return 1, true
		}
		// d and y also take a motion or a text object. A motion the motion
		// parser does not know leaves the whole command unrecognized.
		if cmd == 'd' || cmd == 'y' {
			if consumed, ok := e.applyOperatorMotion(cmd, ref, count, rest); ok {
				return consumed, true
			}
		}
		return 0, false
	case 'x':
		e.deleteChars(ref, count)
	case 'D':
		e.deleteToLineEnd(ref)
	case 'p':
		e.put(ref, count, false)
	case 'P':
		e.put(ref, count, true)
	case 'u':
		e.undoStep()
	case '\x12': // Ctrl-r
		e.redoStep()
	case 'm':
		// m{a-z} records the cursor in the mark named by the next byte. A
		// missing name, or one outside a-z (mA, m1, m'), is a command this
		// engine does not model, so it is left unrecognized. A count in front
		// of m is ignored, which is what nvim does with it.
		if len(rest) == 0 || rest[0] < 'a' || rest[0] > 'z' {
			return 0, false
		}
		e.marks[rest[0]] = e.cursor
		return 1, true
	case '`', '\'':
		// `a jumps to the exact marked position and 'a to the first non-blank
		// of the marked line, or column 0 when the line has none. A count is
		// ignored, as nvim ignores it. A mark that was never set, or whose
		// line was deleted, is consumed as a no-op: nvim reports E20 and
		// leaves the cursor where it was, and the engine keeps its convention
		// of a recognized no-op for a recognized command that cannot act. A
		// name outside a-z is not modeled and stays unrecognized.
		if len(rest) == 0 || rest[0] < 'a' || rest[0] > 'z' {
			return 0, false
		}
		mark, set := e.marks[rest[0]]
		if !set || mark.Line < 0 || mark.Line >= len(e.buffer) {
			return 1, true
		}
		if cmd == '\'' {
			mark.Col, _ = firstNonBlankColumn(e.buffer[mark.Line])
		}
		e.cursor = mark
		return 1, true
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
	e.marksInsertedLines(at, 1)

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

// marksInsertedLines shifts every mark that sits at or below the first inserted
// line down by count. nvim attaches a mark to its line, so a line inserted at or
// above the mark moves it down with the text and a line inserted below it leaves
// it alone (TestSimulateEditing_Marks records the observations).
func (e *editor) marksInsertedLines(at, count int) {
	for name, mark := range e.marks {
		if mark.Line >= at {
			mark.Line += count
			e.marks[name] = mark
		}
	}
}

// marksDeletedLines drops the marks whose line is inside the deleted range and
// lifts the marks below it by count. A deleted marked line leaves no mark at
// all, which is why jumping to it is a no-op.
func (e *editor) marksDeletedLines(at, count int) {
	for name, mark := range e.marks {
		switch {
		case mark.Line < at:
			// Above the deleted range: unchanged.
		case mark.Line < at+count:
			delete(e.marks, name)
		default:
			mark.Line -= count
			e.marks[name] = mark
		}
	}
}

// deleteLines implements [count]dd: it fills the selected register (the unnamed
// one by default) with the deleted lines as linewise content, removes them, and
// puts the cursor on the first non-blank of the line that took their place.
func (e *editor) deleteLines(ref registerRef, count int) {
	start := e.cursor.Line
	end := start + count
	if end > len(e.buffer) {
		end = len(e.buffer)
	}
	e.storeRegister(ref, linewiseRegister(e.buffer[start:end]), false)
	e.marksDeletedLines(start, end-start)

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

// yankLines implements [count]yy: it fills the selected register (the unnamed
// one by default) with whole lines as linewise content and leaves the buffer
// and cursor untouched.
func (e *editor) yankLines(ref registerRef, count int) {
	start := e.cursor.Line
	end := start + count
	if end > len(e.buffer) {
		end = len(e.buffer)
	}
	e.storeRegister(ref, linewiseRegister(e.buffer[start:end]), true)
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

// deleteChars implements [count]x: it fills the selected register with the
// deleted runes as character-wise content, deletes them at and after the cursor
// on the current line, and clamps the cursor to the last remaining rune.
func (e *editor) deleteChars(ref registerRef, count int) {
	runes := []rune(e.buffer[e.cursor.Line])
	if e.cursor.Col >= len(runes) {
		return
	}
	end := e.cursor.Col + count
	if end > len(runes) {
		end = len(runes)
	}
	e.storeRegister(ref, charwiseRegister(string(runes[e.cursor.Col:end])), false)

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

// deleteToLineEnd implements D: it fills the selected register with the
// character-wise text it deletes from the cursor to the end of the current
// line, leaving the cursor on the last remaining rune. An empty line has
// nothing to delete, so the command is a no-op there. A count of one is Vim's
// D, but a larger count makes [count]D span lines, which this engine does not
// model: execute refuses those before they reach here.
func (e *editor) deleteToLineEnd(ref registerRef) {
	runes := []rune(e.buffer[e.cursor.Line])
	if e.cursor.Col >= len(runes) {
		return
	}
	e.storeRegister(ref, charwiseRegister(string(runes[e.cursor.Col:])), false)

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

// put implements [count]p (before false) and [count]P (before true). A linewise
// register inserts its whole lines below the current line with p, above it with
// P, and the cursor lands on the first non-blank of the first inserted line. A
// character-wise register inserts its text after the character under the cursor
// with p, before it with P, and the cursor lands on the last inserted character
// (see putChars). An empty register makes the command a no-op.
func (e *editor) put(ref registerRef, count int, before bool) {
	reg := e.readRegister(ref)
	if !reg.set || len(reg.lines) == 0 {
		return
	}
	if !reg.linewise {
		e.putChars(reg.lines[0], count, before)
		return
	}

	insertAt := e.cursor.Line
	if !before {
		insertAt = e.cursor.Line + 1
	}

	block := make([]string, 0, count*len(reg.lines))
	for i := 0; i < count; i++ {
		block = append(block, reg.lines...)
	}
	e.marksInsertedLines(insertAt, len(block))

	buffer := make([]string, 0, len(e.buffer)+len(block))
	buffer = append(buffer, e.buffer[:insertAt]...)
	buffer = append(buffer, block...)
	buffer = append(buffer, e.buffer[insertAt:]...)

	cursor := Position{Line: insertAt, Col: startOfLineColumn(buffer[insertAt])}
	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
}

// putChars implements a character-wise p and P: p inserts the text after the
// character under the cursor and P before it, the text repeats [count] times,
// and the cursor lands on the last inserted character. On an empty line there
// is no character to put beside, so both forms land at column zero, which is
// where nvim puts it.
func (e *editor) putChars(text string, count int, before bool) {
	if text == "" {
		return
	}
	runes := []rune(e.buffer[e.cursor.Line])
	at := e.cursor.Col
	if !before {
		at++
	}
	if at > len(runes) {
		at = len(runes)
	}
	if at < 0 {
		at = 0
	}

	block := []rune(strings.Repeat(text, count))
	next := make([]rune, 0, len(runes)+len(block))
	next = append(next, runes[:at]...)
	next = append(next, block...)
	next = append(next, runes[at:]...)

	buffer := copyLines(e.buffer)
	buffer[e.cursor.Line] = string(next)

	cursor := Position{Line: e.cursor.Line, Col: at + len(block) - 1}
	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
}

// applyOperatorMotion applies d or y with a motion or a text object and reports
// how many bytes of rest the motion consumed. The motion range is delegated:
// tryParseTextObject and tryParseOperatorMotion are the motion simulator's own
// range math, so an operator affects exactly the range the motion judge sees. A
// linewise motion (j, k, G, gg) makes the operator linewise; every other range
// is character-wise, and a character-wise range that crosses lines is refused
// because this engine keeps character-wise content on one line. A count before
// the operator multiplies the motion in Vim, which this engine does not model,
// so it is refused like a counted D instead of applied once.
func (e *editor) applyOperatorMotion(cmd byte, ref registerRef, count int, rest string) (int, bool) {
	if count > 1 {
		return 0, false
	}

	pos := e.motionPosition()

	// A text object stays on one line in the simulator's range math. Vim's
	// paragraph object is linewise and spans blank lines, so it is refused
	// rather than stored as the single line the simplified object would return.
	if len(rest) >= 2 && (rest[0] == 'i' || rest[0] == 'a') && rest[1] != 'p' {
		if sel, consumed := tryParseTextObject(string(cmd)+rest, pos, e.buffer); consumed > 1 && sel.Active {
			return consumed - 1, e.applyCharwiseOperator(cmd, ref, sel.StartLine, sel.StartCol, sel.EndCol)
		}
	}

	sel, consumed := tryParseOperatorMotion(cmd, rest, pos, e.buffer)
	if consumed == 0 || !sel.Active {
		return 0, false
	}

	if linewise, boundary := linewiseOperatorMotion(rest); linewise {
		return consumed, e.applyLinewiseOperatorMotion(cmd, ref, sel, boundary)
	}
	if sel.StartLine != sel.EndLine {
		return 0, false
	}
	return consumed, e.applyCharwiseOperator(cmd, ref, sel.StartLine, sel.StartCol, sel.EndCol)
}

// linewiseOperatorMotion reports whether the motion in rest makes an operator
// linewise, and whether it is one of the one-line motions (j and k) whose
// failure to move aborts the whole command in nvim. rest may begin with a
// motion count.
func linewiseOperatorMotion(rest string) (linewise, boundary bool) {
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i >= len(rest) {
		return false, false
	}
	switch rest[i] {
	case 'j', 'k':
		return true, true
	case 'G':
		return true, false
	case 'g':
		return i+1 < len(rest) && rest[i+1] == 'g', false
	}
	return false, false
}

// applyLinewiseOperatorMotion applies a linewise d or y over the whole lines
// the motion reached. boundary marks j and k, whose failure to move is a
// complete no-op in nvim: yj on the last line and yk on the first leave the
// buffer, the cursor and the register untouched. A linewise yank backs the
// cursor up to the first line when the range starts above the cursor, and a
// linewise delete lands on the first non-blank of the line that took the
// deleted lines' place.
func (e *editor) applyLinewiseOperatorMotion(cmd byte, ref registerRef, sel Selection, boundary bool) bool {
	first, last := sel.StartLine, sel.EndLine
	if first > last {
		first, last = last, first
	}
	if first < 0 {
		first = 0
	}
	if last >= len(e.buffer) {
		last = len(e.buffer) - 1
	}
	if first > last {
		return false
	}
	if boundary && first == e.cursor.Line && last == e.cursor.Line {
		return true
	}

	if cmd == 'y' {
		e.storeRegister(ref, linewiseRegister(e.buffer[first:last+1]), true)
		if first < e.cursor.Line {
			e.cursor = Position{Line: first, Col: startOfLineColumn(e.buffer[first])}
		}
		return true
	}

	e.storeRegister(ref, linewiseRegister(e.buffer[first:last+1]), false)
	e.marksDeletedLines(first, last-first+1)
	remaining := make([]string, 0, len(e.buffer)-(last-first+1))
	remaining = append(remaining, e.buffer[:first]...)
	remaining = append(remaining, e.buffer[last+1:]...)
	if len(remaining) == 0 {
		remaining = []string{""}
	}
	cursor := Position{Line: first}
	if cursor.Line >= len(remaining) {
		cursor.Line = len(remaining) - 1
	}
	cursor.Col = startOfLineColumn(remaining[cursor.Line])
	e.commit(remaining, cursor, !sameLines(e.buffer, remaining), last == first)
	return true
}

// applyCharwiseOperator applies a character-wise d or y to the inclusive rune
// range line:startCol..endCol. A yank stores the range and, when the range
// starts before the cursor, backs the cursor up to its start, which is what
// nvim does for yiw and yaw. A delete stores the range, removes it, and lands
// the cursor on the range's start clamped into the shortened line.
func (e *editor) applyCharwiseOperator(cmd byte, ref registerRef, line, startCol, endCol int) bool {
	if line < 0 || line >= len(e.buffer) {
		return false
	}
	runes := []rune(e.buffer[line])
	if startCol < 0 {
		startCol = 0
	}
	if endCol >= len(runes) {
		endCol = len(runes) - 1
	}
	if startCol >= len(runes) || endCol < startCol {
		return false
	}

	text := string(runes[startCol : endCol+1])
	if cmd == 'y' {
		e.storeRegister(ref, charwiseRegister(text), true)
		if start := (Position{Line: line, Col: startCol}); lessPosition(start, e.cursor) {
			e.cursor = start
		}
		return true
	}

	e.storeRegister(ref, charwiseRegister(text), false)
	next := make([]rune, 0, len(runes)-(endCol-startCol+1))
	next = append(next, runes[:startCol]...)
	next = append(next, runes[endCol+1:]...)

	buffer := copyLines(e.buffer)
	buffer[line] = string(next)
	cursor := Position{Line: line, Col: startCol}
	if last := len(next) - 1; cursor.Col > last {
		cursor.Col = last
	}
	if cursor.Col < 0 {
		cursor.Col = 0
	}
	e.commit(buffer, cursor, !sameLines(e.buffer, buffer), false)
	return true
}

// lessPosition reports whether a comes before b in buffer order.
func lessPosition(a, b Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

// storeRegister records text in the register an answer named, following Vim's
// rules. Every yank and delete fills the unnamed register; a named register
// (a-z) is filled alongside it when one was named; and register 0 holds the
// last yank, written only when the yank went to the unnamed register. That is
// why a named yank leaves 0 alone while a delete takes over the unnamed
// register without touching 0. The '+' alias is an explicit selection, so it
// does not write 0 either, exactly as Vim's system clipboard does not.
func (e *editor) storeRegister(ref registerRef, text editingRegister, yank bool) {
	if ref.explicit && ref.name >= 'a' && ref.name <= 'z' {
		if e.regs.named == nil {
			e.regs.named = make(map[byte]editingRegister)
		}
		e.regs.named[ref.name] = text
		e.regs.unnamed = text
		return
	}
	e.regs.unnamed = text
	if ref.explicit && ref.name == '0' {
		e.regs.yank = text
		return
	}
	if yank && !(ref.explicit && ref.name == '+') {
		e.regs.yank = text
	}
}

// readRegister is the register a put reads: 0 for the prefixed "0, the named
// register for a-z, and the unnamed register for everything else, including the
// '+' alias the trainer keeps in place of a system clipboard.
func (e *editor) readRegister(ref registerRef) editingRegister {
	if ref.explicit {
		switch {
		case ref.name == '0':
			return e.regs.yank
		case ref.name >= 'a' && ref.name <= 'z':
			return e.regs.named[ref.name]
		}
	}
	return e.regs.unnamed
}

// linewiseRegister and charwiseRegister build the two register contents.
func linewiseRegister(lines []string) editingRegister {
	return editingRegister{lines: copyLines(lines), linewise: true, set: true}
}

func charwiseRegister(text string) editingRegister {
	return editingRegister{lines: []string{text}, linewise: false, set: true}
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
