/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"io"
)

// Writer buffers writes until Flush is called. Flush clears previously written
// lines before writing new lines from the buffer.
// The main logic is platform specific, see the related files.
type Writer struct {
	out             io.Writer
	inProgressLines int
}

// New returns a new Writer
func New(out io.Writer) *Writer {
	return &Writer{out: out}
}

func (w *Writer) Write(persistent []string, progressing []string) {
	defer w.hideCursor()()
	// Move up to the top of our last output.
	up := w.inProgressLines
	w.up(up)
	for _, lines := range [][]string{persistent, progressing} {
		for _, l := range lines {
			w.write([]byte(l))
			w.clearRest()
			w.write([]byte{'\n'})
		}
	}
	w.inProgressLines = len(progressing)
}
