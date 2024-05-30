//go:build !windows
// +build !windows

package dotwriter

import (
	"fmt"
)

// ESC is the ASCII code for escape character
const ESC = 27

// hide cursor
var hide = fmt.Sprintf("%c[?25l", ESC)

// show cursor
var show = fmt.Sprintf("%c[?25h", ESC)

func (w *Writer) write(b []byte) {
	_, _ = w.out.Write(b)
}

func (w *Writer) up(count int) {
	if count == 0 {
		return
	}
	_, _ = fmt.Fprintf(w.out, "%c[%dA", ESC, count)
}

func (w *Writer) clearRest() {
	_, _ = fmt.Fprintf(w.out, "%c[0K", ESC)
}

// hideCursor hides the cursor and returns a function to restore the cursor back.
func (w *Writer) hideCursor() func() {
	_, _ = fmt.Fprint(w.out, hide)
	return func() {
		_, _ = fmt.Fprint(w.out, show)
	}
}
