package tui

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

// The terminal's single reader.
//
// The probe writes a query to the terminal and the terminal answers into the
// terminal's own input queue -- the same queue a keystroke goes into, and the
// same queue the interface reads. With two readers on it, the answer goes to
// whichever reads first: measured, the interface took the whole reply in 1 run
// in 20 and the probe got nothing, and in that run the terminal's answer reached
// the interface as seven input messages (odd/tasks/terminal-probe-ownership.md).
//
// So there is one reader. bubbletea reads the terminal through it, and while the
// probe has a query outstanding it takes the answer out of the stream. Outside
// that window every byte is handed on untouched, which is the property that
// matters most: this reader is on the path every keystroke takes.
//
// That sample put the interface on its own open of the terminal, the arrangement
// bubbletea creates whenever stdin is not a terminal. With the pty on stdin the
// same twenty runs lost nothing: the race is real, and neither sample is a rate.
// The front's log has both readings and the correction they forced.

// terminalInput is the reader the interface was given, registered here because
// the probe is armed from the update loop while the reader is built by
// cmd/dotfiles before the program exists -- the same reason SetGlobalProgram
// registers the program.
var (
	terminalInputMu sync.Mutex
	terminalInput   *replyDivertingReader
)

// TerminalInput returns the terminal's only reader, and registers it as the
// reader the capability probe takes its answer from.
//
// The caller must pass it to the program (tea.WithInput). It satisfies the shape
// bubbletea requires before it will put the terminal into raw mode -- a
// ReadWriteCloser with a descriptor -- so the terminal is made raw exactly as it
// was when bubbletea read os.Stdin directly.
func TerminalInput(stdin *os.File) io.Reader {
	reader := newReplyDivertingReader(stdin)
	terminalInputMu.Lock()
	terminalInput = reader
	terminalInputMu.Unlock()
	return reader
}

// activeTerminalInput is the registered reader, or nil when the program was
// started without TerminalInput. Nil is not "read it yourself": the probe writes
// no query in that arrangement, because the answer would land in a queue two
// readers share, which is the race this reader exists to remove.
func activeTerminalInput() *replyDivertingReader {
	terminalInputMu.Lock()
	defer terminalInputMu.Unlock()
	return terminalInput
}

// replyDivertingReader is the terminal's one reader.
type replyDivertingReader struct {
	source io.Reader

	mu      sync.Mutex
	cond    *sync.Cond
	asking  bool         // a query is outstanding: an answer is expected
	filter  answerFilter // what of the last reads was the terminal's answer
	answers []byte       // what the terminal has answered so far
}

func newReplyDivertingReader(source io.Reader) *replyDivertingReader {
	reader := &replyDivertingReader{source: source}
	reader.cond = sync.NewCond(&reader.mu)
	return reader
}

// Read hands the interface the terminal's bytes, minus the answer to a query
// that is still outstanding. It is the whole point of this type: one reader on
// the terminal, so there is no race to lose and no answer to read as keystrokes.
//
// Two consequences are deliberate and bounded. An answer that arrives after the
// deadline is input again, because by then nothing is waiting for it -- which is
// the position the interface would be in if the probe never read the terminal at
// all. And a read that ends inside an answer's own header leaves a fragment no
// keystroke could have produced, which is handed on rather than held back: holding
// a byte is how a keystroke gets delayed, and this reader sits on the path every
// keystroke takes.
func (r *replyDivertingReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n == 0 {
		return n, err
	}

	r.mu.Lock()
	if !r.asking {
		// The window is closed: the interface's input is exactly what it always
		// was, and this reader must not change a byte of it.
		r.mu.Unlock()
		return n, err
	}
	forward, answer := r.filter.Sift(p[:n])
	r.answers = append(r.answers, answer...)
	r.mu.Unlock()

	// A read that carried nothing but the answer reports nothing: there is
	// nothing of it for the interface, and the interface asks again.
	return copy(p, forward), err
}

// ask writes the query and returns what the terminal answered by the deadline.
// The window opens before the write, because a fast terminal answers while the
// query is still on its way out.
func (r *replyDivertingReader) ask(write func() error, timeout time.Duration) string {
	r.mu.Lock()
	r.asking = true
	r.filter = answerFilter{}
	r.answers = r.answers[:0]
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.asking = false
		r.mu.Unlock()
	}()

	if err := write(); err != nil {
		return ""
	}

	// What arrived by the deadline is the answer and what did not arrive is no
	// answer at all: the same rule the probe has always stated, so a terminal
	// that has nothing to say costs the interface a bounded wait.
	timer := time.AfterFunc(timeout, func() {
		r.mu.Lock()
		r.asking = false
		r.mu.Unlock()
		r.cond.Broadcast()
	})
	defer timer.Stop()

	r.mu.Lock()
	defer r.mu.Unlock()
	for r.asking {
		r.cond.Wait()
	}
	return string(r.answers)
}

// Fd is the terminal's descriptor, which is what bubbletea asks for before it
// makes the terminal raw and before it reads the window size. A source that is
// not a file reports an invalid descriptor, and bubbletea then leaves the
// terminal alone, which is what it already does for a pipe.
func (r *replyDivertingReader) Fd() uintptr {
	if file, ok := r.source.(interface{ Fd() uintptr }); ok {
		return file.Fd()
	}
	return invalidDescriptor
}

// invalidDescriptor is not a descriptor any host hands out, so a caller that
// tests it finds a terminal that is not there.
const invalidDescriptor = ^uintptr(0)

// Write and Close complete the shape bubbletea requires of a program's input.
// The interface writes to the program's output and never to its input, so Write
// exists for the type rather than for a caller.
func (r *replyDivertingReader) Write(p []byte) (int, error) {
	if w, ok := r.source.(io.Writer); ok {
		return w.Write(p)
	}
	return 0, os.ErrInvalid
}

func (r *replyDivertingReader) Close() error {
	if closer, ok := r.source.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// answerFilter takes the terminal's answer to the probe's query out of the
// interface's input stream.
//
// It holds state for one case only: a read that ends in the middle of an answer.
// The answer continues in the next read, and those bytes are still the probe's,
// not the interface's. It never holds a byte back to decide later -- a keystroke
// is never delayed, and nothing is dropped.
type answerFilter struct {
	inReply bool // an answer header was seen and its terminator has not arrived
	// escapeTail records that the last diverted byte was an escape, so a string
	// terminator split between its two bytes still ends the answer.
	escapeTail bool
}

// Sift splits one read into what the interface should receive and what is the
// terminal's answer.
func (f *answerFilter) Sift(data []byte) (forward, answer []byte) {
	for len(data) > 0 {
		if f.inReply {
			if f.escapeTail {
				f.escapeTail = false
				if data[0] == '\\' {
					answer = append(answer, data[0])
					f.inReply = false
					data = data[1:]
					continue
				}
			}

			end, complete := answerEnd(data)
			answer = append(answer, data[:end]...)
			data = data[end:]
			if !complete {
				f.escapeTail = len(answer) > 0 && answer[len(answer)-1] == escape
				return forward, answer
			}
			f.inReply = false
			continue
		}

		start := answerStart(data)
		if start < 0 {
			return append(forward, data...), answer
		}
		forward = append(forward, data[:start]...)
		f.inReply = true
		data = data[start:]
	}
	return forward, answer
}

// escape is the escape byte, which introduces both answers.
const escape = 0x1b

// answerStart is the index of the first complete answer header in data, or -1.
//
// Both headers are things a person cannot produce with a keystroke, and that is
// what makes this safe: an arrow key starts "\x1b[" but never "\x1b[?2026;", and
// no keyboard types a DCS introducer. A header cut in half by the read boundary
// is the one case this cannot see, and that fragment is forwarded -- the limit
// the guards record.
func answerStart(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == escape && hasAnswerHeader(data[i:]) {
			return i
		}
	}
	return -1
}

// hasAnswerHeader reports whether data begins with one of the two answers the
// probe asks for: a DECRQM reply about mode 2026, or an XTGETTCAP reply, whose
// first character after the introducer is the success or the refusal.
func hasAnswerHeader(data []byte) bool {
	if bytes.HasPrefix(data, []byte("\x1b[?2026;")) {
		return true
	}
	return len(data) >= 5 && data[1] == 'P' && (data[2] == '0' || data[2] == '1') && data[3] == '+' && data[4] == 'r'
}

// answerEnd is where the answer in data ends: at the mode query's "$y", at the
// XTGETTCAP string terminator, or at the end of data when the answer has not
// finished arriving.
//
// The two terminators cannot be confused with each other or with content: "$"
// is not a hex digit, so it cannot appear inside an XTGETTCAP value, and the
// earliest of the two in the stream is the one that ends the answer.
func answerEnd(data []byte) (int, bool) {
	decrqm := bytes.Index(data, []byte("$y"))
	st := bytes.Index(data, []byte("\x1b\\"))
	switch {
	case decrqm >= 0 && (st < 0 || decrqm < st):
		return decrqm + 2, true
	case st >= 0:
		return st + 2, true
	}
	return len(data), false
}
