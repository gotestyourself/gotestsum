package testjson

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestSummary_String(t *testing.T) {
	var testcases = []struct {
		name     string
		summary  Summary
		expected string
	}{
		{
			name:     "none",
			summary:  SummarizeNone,
			expected: "none",
		},
		{
			name:     "all",
			summary:  SummarizeAll,
			expected: "skipped,failed,errors,output",
		},
		{
			name:     "one value",
			summary:  SummarizeErrors,
			expected: "errors",
		},
		{
			name:     "a few values",
			summary:  SummarizeOutput | SummarizeSkipped | SummarizeErrors,
			expected: "skipped,errors,output",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.summary.String(), tc.expected)
		})
	}
}

func TestPrintSummary_NoFailures(t *testing.T) {
	fake, reset := patchClock()
	defer reset()

	out := new(bytes.Buffer)
	exec := &Execution{
		started: fake.Now(),
		done:    true,
		packages: map[string]*Package{
			"foo":   {Total: 12},
			"other": {Total: 1},
		},
	}
	fake.Advance(34123111 * time.Microsecond)
	PrintSummary(out, exec, SummarizeAll)

	expected := "\nDONE 13 tests in 34.123s\n"
	assert.Equal(t, out.String(), expected)
}

func TestPrintSummary_WithFailures(t *testing.T) {
	defer patchPkgPathPrefix("example.com")()
	fake, reset := patchClock()
	defer reset()

	exec := &Execution{
		started: fake.Now(),
		done:    true,
		packages: map[string]*Package{
			"example.com/project/fs": {
				Total: 12,
				Failed: []TestCase{
					{
						Package: "example.com/project/fs",
						Test:    "TestFileDo",
						Elapsed: 1411 * time.Millisecond,
					},
					{
						Package: "example.com/project/fs",
						Test:    "TestFileDoError",
						Elapsed: 12 * time.Millisecond,
					},
				},
				output: map[string]map[string][]string{
					"TestFileDo": multiLine(`=== RUN   TestFileDo
Some stdout/stderr here
--- FAIL: TestFileDo (1.41s)
	do_test.go:33 assertion failed
`),
					"TestFileDoError": multiLine(`=== RUN   TestFileDoError
--- FAIL: TestFileDoError (0.01s)
	do_test.go:50 assertion failed: expected nil error, got WHAT!
`),
					"": multiLine("FAIL\n"),
				},
				action: ActionFail,
			},
			"example.com/project/pkg/more": {
				Total: 1,
				Failed: []TestCase{
					{
						Package: "example.com/project/pkg/more",
						Test:    "TestAlbatross",
						Elapsed: 40 * time.Millisecond,
					},
				},
				Skipped: []TestCase{
					{
						Package: "example.com/project/pkg/more",
						Test:    "TestOnlySometimes",
						Elapsed: 0,
					},
				},
				output: map[string]map[string][]string{
					"TestAlbatross": multiLine(`=== RUN   TestAlbatross
--- FAIL: TestAlbatross (0.04s)
`),
					"TestOnlySometimes": multiLine(`=== RUN   TestOnlySometimes
--- SKIP: TestOnlySometimes (0.00s)
	good_test.go:27: the skip message
`),
				},
			},
			"example.com/project/badmain": {
				action: ActionFail,
				output: map[string]map[string][]string{
					"": multiLine("sometimes main can exit 2\n"),
				},
			},
		},
		errors: []string{
			"pkg/file.go:99:12: missing ',' before newline",
		},
	}
	fake.Advance(34123111 * time.Microsecond)

	t.Run("summarize all", func(t *testing.T) {
		out := new(bytes.Buffer)
		PrintSummary(out, exec, SummarizeAll)

		expected := `
=== Skipped
=== SKIP: project/pkg/more TestOnlySometimes (0.00s)
	good_test.go:27: the skip message


=== Failed
=== FAIL: project/badmain  (0.00s)
sometimes main can exit 2

=== FAIL: project/fs TestFileDo (1.41s)
Some stdout/stderr here
	do_test.go:33 assertion failed

=== FAIL: project/fs TestFileDoError (0.01s)
	do_test.go:50 assertion failed: expected nil error, got WHAT!

=== FAIL: project/pkg/more TestAlbatross (0.04s)


=== Errors
pkg/file.go:99:12: missing ',' before newline

DONE 13 tests, 1 skipped, 4 failures, 1 error in 34.123s
`
		assert.Equal(t, out.String(), expected)
	})

	t.Run("summarize no output", func(t *testing.T) {
		out := new(bytes.Buffer)
		PrintSummary(out, exec, SummarizeAll-SummarizeOutput)

		expected := `
=== Skipped
=== SKIP: project/pkg/more TestOnlySometimes (0.00s)


=== Failed
=== FAIL: project/badmain  (0.00s)

=== FAIL: project/fs TestFileDo (1.41s)

=== FAIL: project/fs TestFileDoError (0.01s)

=== FAIL: project/pkg/more TestAlbatross (0.04s)


=== Errors
pkg/file.go:99:12: missing ',' before newline

DONE 13 tests, 1 skipped, 4 failures, 1 error in 34.123s
`
		assert.Equal(t, out.String(), expected)
	})

	t.Run("summarize only errors", func(t *testing.T) {
		out := new(bytes.Buffer)
		PrintSummary(out, exec, SummarizeErrors)

		expected := `
=== Errors
pkg/file.go:99:12: missing ',' before newline

DONE 13 tests, 1 skipped, 4 failures, 1 error in 34.123s
`
		assert.Equal(t, out.String(), expected)
	})
}

func patchClock() (clockwork.FakeClock, func()) {
	fake := clockwork.NewFakeClock()
	clock = fake
	return fake, func() { clock = clockwork.NewRealClock() }
}

func multiLine(s string) map[string][]string {
	return map[string][]string{
		"": strings.SplitAfter(s, "\n"),
	}
}

func TestPrintSummary_MissingTestFailEvent(t *testing.T) {
	_, reset := patchClock()
	defer reset()
	exec, err := ScanTestOutput(ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, "go-test-json-missing-test-fail.out")),
		Stderr:  bytes.NewReader(nil),
		Handler: noopHandler{},
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-missing-test-fail-event")
}

type noopHandler struct{}

func (s noopHandler) Event(TestEvent, *Execution) error {
	return nil
}

func (s noopHandler) Err(string) error {
	return nil
}
