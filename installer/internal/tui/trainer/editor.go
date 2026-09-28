package trainer

import "unicode/utf8"

// Mode is the editing mode the engine is in after an answer. Only ModeNormal is
// reachable from the commands this engine understands today; the remaining
// constants exist so the insert and visual commands of the following tasks can
// be added without changing the shape of EditingResult.
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
type EditingResult struct {
	Buffer     []string // the buffer after the answer
	Cursor     Position // the cursor after the answer, in runes
	Mode       Mode     // the mode after the answer
	Recognized bool     // false when the answer contains something the engine cannot parse
}

// indentUnit is one shift for >> and <<. The trainer's exercises are written
// with two-space indentation, so the engine models 'shiftwidth=2' with
// 'expandtab' set: >> inserts two spaces, and << removes up to two columns of
// leading whitespace (a single leading tab counts as one shift).
const indentUnit = "  "

// indentWidth is the column width of one shift, in bytes; indentUnit is spaces
// so bytes and runes agree.
const indentWidth = len(indentUnit)

// editingRegister is the unnamed yank/delete register. linewise records whether
// the content is whole lines (yy, dd) or a character range, because p and P mean
// different things for each. Only linewise content exists today: yy and dd fill
// the register, and put always inserts whole lines.
type editingRegister struct {
	lines    []string
	linewise bool
	set      bool
}

// editorSnapshot is one undo history entry: the buffer and cursor as they were
// before a normal-mode command changed the buffer.
type editorSnapshot struct {
	buffer []string
	cursor Position
}

// editor is the mutable editing state for a single SimulateEditing call.
type editor struct {
	buffer []string
	cursor Position
	undo   []editorSnapshot
	redo   []editorSnapshot
	reg    editingRegister
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
// runes within that line. An empty line puts the cursor at column 0.
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

	length := utf8.RuneCountInString(e.buffer[e.cursor.Line])
	if length == 0 {
		e.cursor.Col = 0
		return
	}
	if e.cursor.Col < 0 {
		e.cursor.Col = 0
	}
	if e.cursor.Col >= length {
		e.cursor.Col = length - 1
	}
}

// execute parses input one normal-mode command at a time and returns whether
// every keystroke was consumed. A command is either a mutation this engine
// implements or a motion parsed by the simulator's own motion parser, so an
// answer may reposition the cursor before it mutates, and a motion ahead of a
// mutation acts on the line the motion reached rather than a second, divergent
// motion implementation.
//
// A motion changes only the cursor, so it never records an undo snapshot; the
// one-snapshot-per-command rule still holds because only a buffer change
// commits. On the first keystroke it cannot parse it stops and reports false;
// the commands performed before that point remain applied, because the caller
// decides correctness from Recognized, not from the buffer.
func (e *editor) execute(input string) bool {
	i := 0
	for i < len(input) {
		// Parse the count once so a mutation and a motion share the same one.
		// The count never starts with '0': that byte is the first-column
		// motion, which countPrefix leaves for the motion parser.
		count, afterCount := countPrefix(input, i)

		if afterCount < len(input) {
			if consumed, ok := e.applyMutation(input[afterCount], count, input[afterCount+1:]); ok {
				i = afterCount + 1 + consumed
				continue
			}
		}

		// Not a mutation: hand the whole token to the shared motion parser,
		// count included, so the engine recognizes exactly the motions
		// SimulateMotions recognizes and lands exactly where it lands. The
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
	cursor.Col = firstNonBlankColumn(remaining[cursor.Line])

	e.commit(remaining, cursor, !sameLines(e.buffer, remaining))
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
// on the current line and the lines below it, then leaves the cursor on the
// first non-blank of the current line.
func (e *editor) shiftLines(count int, indent bool) {
	next := copyLines(e.buffer)
	last := e.cursor.Line + count
	if last > len(next) {
		last = len(next)
	}
	for line := e.cursor.Line; line < last; line++ {
		if indent {
			next[line] = indentUnit + next[line]
		} else {
			next[line] = removeShift(next[line])
		}
	}

	cursor := Position{Line: e.cursor.Line, Col: firstNonBlankColumn(next[e.cursor.Line])}
	e.commit(next, cursor, !sameLines(e.buffer, next))
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

	e.commit(buffer, cursor, !sameLines(e.buffer, buffer))
}

// deleteToLineEnd implements D: it deletes from the cursor to the end of the
// current line, leaving the cursor on the last remaining rune. An empty line
// has nothing to delete, so the command is a no-op there. A count prefix is
// parsed but ignored; [count]D is out of scope for this engine.
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

	e.commit(buffer, cursor, !sameLines(e.buffer, buffer))
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

	cursor := Position{Line: insertAt, Col: firstNonBlankColumn(buffer[insertAt])}
	e.commit(buffer, cursor, !sameLines(e.buffer, buffer))
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
	e.redo = append(e.redo, editorSnapshot{buffer: copyLines(e.buffer), cursor: e.cursor})
	e.buffer = snapshot.buffer
	e.cursor = snapshot.cursor
}

// redoStep implements Ctrl-r: it reapplies the most recently undone command.
func (e *editor) redoStep() {
	if len(e.redo) == 0 {
		return
	}
	snapshot := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.undo = append(e.undo, editorSnapshot{buffer: copyLines(e.buffer), cursor: e.cursor})
	e.buffer = snapshot.buffer
	e.cursor = snapshot.cursor
}

// commit applies a command's result. changed reports whether the buffer really
// changed; only a real change records an undo snapshot and clears the redo
// stack, so no-op commands never become undo points.
func (e *editor) commit(buffer []string, cursor Position, changed bool) {
	if changed {
		e.undo = append(e.undo, editorSnapshot{buffer: copyLines(e.buffer), cursor: e.cursor})
		e.redo = nil
	}
	e.buffer = buffer
	e.cursor = cursor
}

func (e *editor) result(recognized bool) EditingResult {
	// One clamp at the end, exactly where SimulateMotions clamps: an adopted
	// motion cursor is kept unclamped while commands run, so a later motion
	// starts from the same position the simulator would start from.
	e.clampCursor()
	return EditingResult{
		Buffer:     copyLines(e.buffer),
		Cursor:     e.cursor,
		Mode:       ModeNormal,
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

// firstNonBlankColumn returns the rune column of the first non-blank character,
// or 0 for a blank or empty line.
func firstNonBlankColumn(line string) int {
	for i, r := range []rune(line) {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return 0
}

// removeShift removes up to one shift of leading whitespace: one leading tab,
// or up to indentWidth leading spaces.
func removeShift(line string) string {
	runes := []rune(line)
	removed := 0
	for removed < indentWidth && len(runes) > 0 && runes[0] == ' ' {
		runes = runes[1:]
		removed++
	}
	if removed == 0 && len(runes) > 0 && runes[0] == '\t' {
		runes = runes[1:]
	}
	return string(runes)
}
