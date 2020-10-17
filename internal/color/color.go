package color

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

var NoColor = os.Getenv("TERM") == "dumb" ||
	(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))

const escape = "\x1b"

type Attribute struct {
	background       bool
	modifier         uint8
	red, green, blue uint8
	code256          uint8
}

func (a Attribute) BG() Attribute {
	a.background = true
	return a
}

func (a Attribute) Bold() Attribute {
	a.modifier = 1
	return a
}

func (a Attribute) Underline() Attribute {
	a.modifier = 4
	return a
}

func RGB(r, g, b uint8) Attribute {
	return Attribute{red: r, green: g, blue: b}
}

func Code256(code uint8) Attribute {
	return Attribute{code256: code}
}

func Hex(hex uint32) Attribute {
	return Attribute{
		red:   uint8(hex & (255 << 16) >> 16),
		green: uint8(hex & (255 << 8) >> 8),
		blue:  uint8(hex & 255),
	}
}

func Unset(w io.Writer) (int, error) {
	if NoColor {
		return 0, nil
	}
	return fmt.Fprintf(w, "%s[0m", escape)
}

func Color(a Attribute) func(w io.Writer) (int, error) {
	if NoColor {
		return nil
	}
	return func(w io.Writer) (int, error) {
		buf := bufio.NewWriter(w)

		fmt.Fprint(buf, escape, "[")
		if a.modifier != 0 {
			fmt.Fprintf(buf, "%d;", a.modifier)
		}
		if a.background {
			buf.WriteString("48;")
		} else {
			buf.WriteString("38;")
		}
		switch {
		case a.code256 > 0:
			fmt.Fprintf(buf, "5;%d", a.code256)
		default:
			fmt.Fprintf(buf, "2;%d;%d;%d", a.red, a.green, a.blue)
		}
		buf.WriteString("m")
		return buf.Buffered(), buf.Flush()
	}
}
