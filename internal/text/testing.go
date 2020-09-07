package text

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// ProcessLines from the Reader by passing each one to ops. The output of each
// op is passed to the next. Returns the string created by joining all the
// processed lines.
func ProcessLines(t *testing.T, r io.Reader, ops ...func(string) string) string {
	t.Helper()
	out := new(strings.Builder)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		for _, op := range ops {
			line = op(line)
		}
		out.WriteString(line + "\n")
	}
	assert.NilError(t, scan.Err())
	return out.String()
}

func OpRemoveSummaryLineElapsedTime(line string) string {
	if i := strings.Index(line, " in "); i > 0 {
		return line[:i]
	}
	return line
}

func OpRemoveTestElapsedTime(line string) string {
	if i := strings.Index(line, " (0."); i > 0 && i+8 == len(line) {
		return line[:i]
	}
	return line
}
