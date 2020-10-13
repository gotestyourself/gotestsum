package color

import (
	"bufio"
	"fmt"
	"io"
	"strconv"

	"github.com/fatih/color"
)

const escape = "\x1b"

func Unset(w io.Writer) (int, error) {
	if color.NoColor {
		return 0, nil
	}
	return fmt.Fprintf(w, "%s[%dm", escape, color.Reset)
}

func Color(a ...color.Attribute) func(w io.Writer) (int, error) {
	if color.NoColor {
		return nil
	}
	return func(w io.Writer) (int, error) {
		buf := bufio.NewWriter(w)

		fmt.Fprint(buf, escape, "[")
		for i, v := range a {
			if i != 0 {
				buf.WriteString(";")
			}
			buf.WriteString(strconv.Itoa(int(v)))
		}
		buf.WriteString("m")
		return buf.Buffered(), buf.Flush()
	}
}
