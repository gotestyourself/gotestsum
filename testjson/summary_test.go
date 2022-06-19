package testjson

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

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
	patchTimeNow(t)

	out := new(bytes.Buffer)
	start := time.Now()
	exec := &Execution{
		started: start,
		done:    true,
		packages: map[string]*Package{
			"foo":   {Total: 12},
			"other": {Total: 1},
		},
	}
	timeNow = func() time.Time {
		return start.Add(34123111 * time.Microsecond)
	}
	PrintSummary(out, exec, SummarizeAll)

	expected := "\nDONE 13 tests in 34.123s\n"
	assert.Equal(t, out.String(), expected)
}

func TestPrintSummary_WithFailures(t *testing.T) {
	t.Cleanup(patchPkgPathPrefix("example.com"))
	patchTimeNow(t)

	start := time.Now()
	exec := &Execution{
		started: start,
		done:    true,
		packages: map[string]*Package{
			"example.com/project/fs": {
				Total: 12,
				Failed: []TestCase{
					{
						Package: "example.com/project/fs",
						Test:    "TestFileDo",
						Elapsed: 1411 * time.Millisecond,
						ID:      1,
					},
					{
						Package: "example.com/project/fs",
						Test:    "TestFileDoError",
						Elapsed: 12 * time.Millisecond,
						ID:      2,
					},
				},
				output: map[int][]string{
					1: multiLine(`=== RUN   TestFileDo
Some stdout/stderr here
--- FAIL: TestFileDo (1.41s)
	do_test.go:33 assertion failed
`),
					2: multiLine(`=== RUN   TestFileDoError
--- FAIL: TestFileDoError (0.01s)
	do_test.go:50 assertion failed: expected nil error, got WHAT!
`),
					0: multiLine("FAIL\n"),
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
						ID:      1,
					},
				},
				Skipped: []TestCase{
					{
						Package: "example.com/project/pkg/more",
						Test:    "TestOnlySometimes",
						Elapsed: 0,
						ID:      2,
					},
				},
				output: map[int][]string{
					1: multiLine(`=== RUN   TestAlbatross
--- FAIL: TestAlbatross (0.04s)
`),
					2: multiLine(`=== RUN   TestOnlySometimes
--- SKIP: TestOnlySometimes (0.00s)
	good_test.go:27: the skip message
`),
				},
			},
			"example.com/project/badmain": {
				action: ActionFail,
				output: map[int][]string{
					0: multiLine("sometimes main can exit 2\n"),
				},
			},
		},
		errors: []string{
			"pkg/file.go:99:12: missing ',' before newline",
		},
	}
	timeNow = func() time.Time {
		return start.Add(34123111 * time.Microsecond)
	}

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

func patchTimeNow(t *testing.T) func() {
	timeNow = func() time.Time {
		return time.Date(2022, 1, 2, 3, 4, 5, 600, time.UTC)
	}
	reset := func() {
		timeNow = time.Now
	}
	t.Cleanup(reset)
	return reset
}

func multiLine(s string) []string {
	return strings.SplitAfter(s, "\n")
}

func TestPrintSummary_MissingTestFailEvent(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json-missing-test-fail.out")),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-missing-test-fail-event")

	for name, pkg := range exec.packages {
		assert.Equal(t, len(pkg.running), 0, "package %v still had tests in running", name)
	}
}

func TestPrintSummary_WithMisattributedOutput(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json-misattributed.out")),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-misattributed-output")
}

func TestPrintSummary_WithSubtestFailures(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json.out")),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-root-test-has-subtest-failures")
}

func TestPrintSummary_WithParallelFailures(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json-with-parallel-fails.out")),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-parallel-failures.out")
}

func TestPrintSummary_WithMissingSkipMessage(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json-missing-skip-msg.out")),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "bug-missing-skip-message-summary.out")
}

func TestPrintSummary_WithRepeatedTestCases(t *testing.T) {
	patchTimeNow(t)

	in := golden.Get(t, "go-test-json.out")
	exec, err := ScanTestOutput(ScanConfig{
		Stdout: io.MultiReader(
			bytes.NewReader(in),
			bytes.NewReader(in),
			bytes.NewReader(in)),
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "bug-repeated-test-case-output.out")
}

func TestPrintSummary_WithRerunID(t *testing.T) {
	patchTimeNow(t)

	exec, err := ScanTestOutput(ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json.out")),
		RunID:  7,
	})
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	PrintSummary(buf, exec, SummarizeAll)
	golden.Assert(t, buf.String(), "summary-with-run-id.out")
}
