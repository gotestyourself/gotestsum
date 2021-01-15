package log

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

type Level uint8

const (
	ErrorLevel Level = iota
	WarnLevel
	DebugLevel
)

var (
	level              = WarnLevel
	out   stringWriter = os.Stderr
)

func writeOrPanic(s string) int {
	n, err := out.WriteString(s)
	if err != nil {
		panic(err)
	}
	return n
}

// TODO: replace with io.StringWriter once support for go1.11 is dropped.
type stringWriter interface {
	WriteString(s string) (n int, err error)
}

// SetLevel for the global logger.
func SetLevel(l Level) {
	level = l
}

// Warnf prints the message to stderr, with a yellow WARN prefix.
func Warnf(format string, args ...interface{}) {
	if level < WarnLevel {
		return
	}
	writeOrPanic(color.YellowString("WARN "))
	writeOrPanic(fmt.Sprintf(format, args...))
	writeOrPanic("\n")
}

// Debugf prints the message to stderr, with no prefix.
func Debugf(format string, args ...interface{}) {
	if level < DebugLevel {
		return
	}
	writeOrPanic(fmt.Sprintf(format, args...))
	writeOrPanic("\n")
}

// Errorf prints the message to stderr, with a red ERROR prefix.
func Errorf(format string, args ...interface{}) {
	if level < ErrorLevel {
		return
	}
	writeOrPanic(color.RedString("ERROR "))
	writeOrPanic(fmt.Sprintf(format, args...))
	writeOrPanic("\n")
}

// Error prints the message to stderr, with a red ERROR prefix.
func Error(msg string) {
	if level < ErrorLevel {
		return
	}
	writeOrPanic(color.RedString("ERROR "))
	writeOrPanic(msg)
	writeOrPanic("\n")
}
