package testjson

import (
	"bufio"
	"bytes"
	"github.com/astralkn/gotestmng/pkg/dotwriter"
	"io"
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"testing/quick"
	"time"
	"unicode/utf8"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
	"gotest.tools/golden"
	"gotest.tools/skip"
)

func TestScanTestOutput_WithDotsFormatter(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	out := new(bytes.Buffer)
	dotfmt := &dotFormatter{
		pkgs:      make(map[string]*dotLine),
		writer:    dotwriter.New(out),
		termWidth: 80,
	}
	shim := newFakeHandler(dotfmt, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))
	assert.NilError(t, err)

	actual := removeSummaryTime(t, out)
	golden.Assert(t, actual, outFile("dots-format"))
	golden.Assert(t, shim.err.String(), "dots-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func outFile(name string) string {
	if runtime.GOOS == "windows" {
		return name + "-windows.out"
	}
	return name + ".out"
}

func removeSummaryTime(t *testing.T, r io.Reader) string {
	t.Helper()
	out := new(strings.Builder)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if i := strings.Index(line, " in "); i > 0 {
			out.WriteString(line[:i] + "\n")
			continue
		}
		out.WriteString(line + "\n")
	}
	assert.NilError(t, scan.Err())
	return out.String()
}

func TestFmtDotElapsed(t *testing.T) {
	var testcases = []struct {
		cached   bool
		elapsed  time.Duration
		expected string
	}{
		{
			elapsed:  999 * time.Microsecond,
			expected: " 999µs ",
		},
		{
			elapsed:  7 * time.Millisecond,
			expected: "   7ms ",
		},
		{
			cached:   true,
			elapsed:  time.Millisecond,
			expected: "    🖴  ",
		},
		{
			elapsed:  3 * time.Hour,
			expected: "    ⏳  ",
		},
		{
			elapsed:  14 * time.Millisecond,
			expected: "  14ms ",
		},
		{
			elapsed:  333 * time.Millisecond,
			expected: " 333ms ",
		},
		{
			elapsed:  1337 * time.Millisecond,
			expected: " 1.33s ",
		},
		{
			elapsed:  14821 * time.Millisecond,
			expected: " 14.8s ",
		},
		{
			elapsed:  time.Minute + 59*time.Second,
			expected: " 1m59s ",
		},
		{
			elapsed:  59*time.Minute + 59*time.Second,
			expected: " 59m0s ",
		},
		{
			elapsed:  148213 * time.Millisecond,
			expected: " 2m28s ",
		},
		{
			elapsed:  1482137 * time.Millisecond,
			expected: " 24m0s ",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.expected, func(t *testing.T) {
			pkg := &Package{
				cached: tc.cached,
				Passed: []TestCase{{Elapsed: tc.elapsed}},
			}
			actual := fmtDotElapsed(pkg)
			assert.Check(t, cmp.Equal(utf8.RuneCountInString(actual), 7))
			assert.Equal(t, actual, tc.expected)
		})
	}
}

func TestFmtDotElapsed_RuneCountProperty(t *testing.T) {
	f := func(d time.Duration) bool {
		pkg := &Package{
			Passed: []TestCase{{Elapsed: d}},
		}
		actual := fmtDotElapsed(pkg)
		width := utf8.RuneCountInString(actual)
		if width == 7 {
			return true
		}
		t.Logf("actual %v (width %d)", actual, width)
		return false
	}

	seed := time.Now().Unix()
	t.Log("seed", seed)
	assert.Assert(t, quick.Check(f, &quick.Config{
		MaxCountScale: 2000,
		Rand:          rand.New(rand.NewSource(seed)),
	}))
}

func TestNewDotFormatter(t *testing.T) {
	buf := new(bytes.Buffer)
	ef := newDotFormatter(buf)

	d, ok := ef.(*dotFormatter)
	skip.If(t, !ok, "no terminal width")
	assert.Assert(t, d.termWidth != 0)
}
