/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"bufio"
	"bytes"
	"io"
	"time"
)

// ESC is the ASCII code for escape character
const ESC = 27

// Writer buffers writes until Flush is called. Flush clears previously written
// lines before writing new lines from the buffer.
type Writer struct {
	out       io.Writer
	buf       bytes.Buffer
	last      []byte
	lineCount int
	t         *time.Timer
}

// New returns a new Writer
func New(out io.Writer) *Writer {
	out = bufio.NewWriter(out)
	w := &Writer{out: out}
	return w
}

var Clears int

// Flush the buffer, writing all buffered lines to out
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}
	w.hideCursor()
	b := w.buf.Bytes()
	Clears = w.lineCount
	w.clearLines(w.lineCount)
	w.lineCount = bytes.Count(b, []byte{'\n'})
	_, err := w.out.Write(b)
	w.showCursor()
	w.buf.Reset()
	w.out.(*bufio.Writer).Flush()
	return err
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
