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
	now := time.Date(2022, 1, 2, 3, 4, 5, 600, time.UTC)
	patchTimeNow(t, now)

	out := new(bytes.Buffer)
	start := time.Now()
	exec := &Execution{
		testStart: start,
		done:      true,
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
	patchPkgPathPrefix(t, "example.com")
	now := time.Date(2022, 1, 2, 3, 4, 5, 600, time.UTC)
	patchTimeNow(t, now)

	start := time.Now()
	exec := &Execution{
		testStart: start,
		done:      true,
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

func patchTimeNow(t *testing.T, to time.Time) {
	timeNow = func() time.Time {
		return to
	}
	t.Cleanup(func() {
		timeNow = time.Now
	})
}

func multiLine(s string) []string {
	return strings.SplitAfter(s, "\n")
}

func TestPrintSummary(t *testing.T) {
	type testCase struct {
		name        string
		config      func(t *testing.T) ScanConfig
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		exec, err := ScanTestOutput(tc.config(t))
		assert.NilError(t, err)

		buf := new(bytes.Buffer)
		PrintSummary(buf, exec, SummarizeAll)
		golden.Assert(t, buf.String(), tc.expectedOut)

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	testCases := []testCase{
		{
			name:        "missing test fail event",
			config:      scanConfigFromGolden("input/go-test-json-missing-test-fail.out"),
			expectedOut: "summary/missing-test-fail-event",
			expected: func(t *testing.T, exec *Execution) {
				for name, pkg := range exec.packages {
					assert.Equal(t, len(pkg.running), 0, "package %v still had tests in running", name)
				}
			},
		},
		{
			name:        "output attributed to wrong test",
			config:      scanConfigFromGolden("input/go-test-json-misattributed.out"),
			expectedOut: "summary/misattributed-output",
		},
		{
			name:        "with subtest failures",
			config:      scanConfigFromGolden("input/go-test-json.out"),
			expectedOut: "summary/root-test-has-subtest-failures",
		},
		{
			name:        "with parallel failures",
			config:      scanConfigFromGolden("input/go-test-json-with-parallel-fails.out"),
			expectedOut: "summary/parallel-failures",
		},
		{
			name:        "with attributes",
			config:      scanConfigFromGolden("input/go-test-json-with-attributes.out"),
			expectedOut: "summary/with-attributes",
		},
		{
			name:        "missing skip message",
			config:      scanConfigFromGolden("input/go-test-json-missing-skip-msg.out"),
			expectedOut: "summary/bug-missing-skip-message",
		},
		{
			name: "repeated test case",
			config: func(t *testing.T) ScanConfig {
				in := golden.Get(t, "input/go-test-json.out")
				return ScanConfig{
					Stdout: io.MultiReader(
						bytes.NewReader(in),
						bytes.NewReader(in),
						bytes.NewReader(in)),
				}
			},
			expectedOut: "summary/bug-repeated-test-case-output",
		},
		{
			name: "with rerun id",
			config: func(t *testing.T) ScanConfig {
				return ScanConfig{
					Stdout: bytes.NewReader(golden.Get(t, "input/go-test-json.out")),
					RunID:  7,
				}
			},
			expectedOut: "summary/with-run-id",
		},
		{
			name:        "Go 1.20 benchmark",
			config:      scanConfigFromGolden("input/go-1-20-benchmark.out"),
			expectedOut: "summary/go-1-20-benchmark",
		},
		{
			name:        "Go 1.20 benchmark panic",
			config:      scanConfigFromGolden("input/go-1-20-panicked-benchmark.out"),
			expectedOut: "summary/go-1-20-benchmark-panic",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func scanConfigFromGolden(filename string) func(t *testing.T) ScanConfig {
	return func(t *testing.T) ScanConfig {
		return ScanConfig{Stdout: bytes.NewReader(golden.Get(t, filename))}
	}
}
