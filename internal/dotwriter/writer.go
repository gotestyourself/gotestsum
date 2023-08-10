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
	height    int
}

// New returns a new Writer
func New(out io.Writer, h int) *Writer {
	t := time.NewTicker(time.Millisecond * 100)
	out = bufio.NewWriter(out)
	// Give some buffer from the terminals height so they can see the original command
	w := &Writer{out: out, height: h - 2}
	go func() {
		for {
			select {
			case <-t.C:
				w.emit()
			}
		}
	}()
	return w
}

// Flush the buffer, writing all buffered lines to out
func (w *Writer) Flush() error {
	w.last = w.buf.Bytes()
	w.buf.Reset()
	return nil
}

func (w *Writer) emit() error {
	if w.buf.Len() == 0 {
		return nil
	}
	w.hideCursor()
	w.clearLines(w.lineCount)
	lines := bytes.Split(w.last, []byte{'\n'})
	if len(lines) > w.height {
		lines = lines[len(lines)-w.height:]
	}
	w.lineCount = len(lines) - 1
	_, err := w.out.Write(bytes.Join(lines, []byte{'\n'}))
	//w.lineCount = bytes.Count(w.buf.Bytes(), []byte{'\n'})
	//_, err := w.out.Write(w.buf.Bytes())
	w.showCursor()
	w.out.(*bufio.Writer).Flush()
	return err
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
