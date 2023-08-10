//go:build !windows
// +build !windows

package dotwriter

import (
	"fmt"
	"strings"
)

// clear the line and move the cursor up
var clear = fmt.Sprintf("%c[%dA%c[2K", ESC, 1, ESC)
var hide = fmt.Sprintf("%c[?25l", ESC)
var show = fmt.Sprintf("%c[?25h", ESC)

func (w *Writer) clearLines(count int) {
	_, _ = fmt.Fprint(w.out, strings.Repeat(clear, count))
}

// hideCursor hides the cursor and returns a function to restore the cursor back.
func (w *Writer) hideCursor() {
	_, _ = fmt.Fprint(w.out, hide)
}
func (w *Writer) showCursor() {
	_, _ = fmt.Fprint(w.out, show)
}
