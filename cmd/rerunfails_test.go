package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestWriteRerunFailsReport(t *testing.T) {
	reportFile := fs.NewFile(t, t.Name())
	defer reportFile.Remove()

	opts := &options{
		rerunFailsReportFile:  reportFile.Path(),
		rerunFailsMaxAttempts: 4,
	}

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-json-flaky-rerun.out")),
	})
	assert.NilError(t, err)

	err = writeRerunFailsReport(opts, exec)
	assert.NilError(t, err)

	raw, err := os.ReadFile(reportFile.Path())
	assert.NilError(t, err)
	golden.Assert(t, string(raw), t.Name()+"-expected")
}

func TestWriteRerunFailsReport_HandlesMissingActionRunEvents(t *testing.T) {
	reportFile := fs.NewFile(t, t.Name())
	defer reportFile.Remove()

	opts := &options{
		rerunFailsReportFile:  reportFile.Path(),
		rerunFailsMaxAttempts: 4,
	}

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: bytes.NewReader(golden.Get(t, "go-test-missing-run-events.out")),
	})
	assert.NilError(t, err)

	err = writeRerunFailsReport(opts, exec)
	assert.NilError(t, err)

	raw, err := os.ReadFile(reportFile.Path())
	assert.NilError(t, err)
	golden.Assert(t, string(raw), t.Name()+"-expected")
}

func TestGoTestRunFlagFromTestCases(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}
	fn := func(t *testing.T, tc testCase) {
		actual := goTestRunFlagForTestCase(testjson.TestName(tc.input))
		assert.Equal(t, actual, tc.expected)
	}

	var testCases = map[string]testCase{
		"root test case": {
			input:    "TestOne",
			expected: "-test.run=^TestOne$",
		},
		"sub test case": {
			input:    "TestOne/SubtestA",
			expected: "-test.run=^TestOne$/^SubtestA$",
		},
		"sub test case with special characters": {
			input:    "TestOne/Subtest(A)[100]",
			expected: `-test.run=^TestOne$/^Subtest\(A\)\[100\]$`,
		},
		"nested sub test case": {
			input:    "TestOne/Nested/SubtestA",
			expected: `-test.run=^TestOne$/^Nested$/^SubtestA$`,
		},
	}

	for name := range testCases {
		t.Run(name, func(t *testing.T) {
			fn(t, testCases[name])
		})
	}
}

func TestRerunFailed_ReturnsAnErrorWhenTheLastTestIsSuccessful(t *testing.T) {
	type result struct {
		out string
		err error
	}
	jsonFailed := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "fail"}
{"Package": "pkg", "Action": "fail"}
`
	events := []result{
		{out: jsonFailed, err: newExitCode("run-failed-1", 1)},
		{out: jsonFailed, err: newExitCode("run-failed-2", 1)},
		{out: jsonFailed, err: newExitCode("run-failed-3", 1)},
		{
			out: `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "pass"}
{"Package": "pkg", "Action": "pass"}
`,
		},
	}

	fn := func([]string) *proc {
		next := events[0]
		events = events[1:]
		return &proc{
			cmd:    fakeWaiter{result: next.err},
			stdout: strings.NewReader(next.out),
			stderr: bytes.NewReader(nil),
		}
	}
	reset := patchStartGoTestFn(fn)
	defer reset()

	stdout := new(bytes.Buffer)
	ctx := context.Background()
	opts := &options{
		rerunFailsMaxInitialFailures: 10,
		rerunFailsMaxAttempts:        2,
		stdout:                       stdout,
	}
	cfg := testjson.ScanConfig{
		Execution: newExecutionWithTwoFailures(t),
		Handler:   noopHandler{},
	}
	err := rerunFailed(ctx, opts, cfg)
	assert.Error(t, err, "run-failed-3")
}

func patchStartGoTestFn(f func(args []string) *proc) func() {
	orig := startGoTestFn
	startGoTestFn = func(_ context.Context, _ string, args []string) (*proc, error) {
		return f(args), nil
	}
	return func() {
		startGoTestFn = orig
	}
}

func newExecutionWithTwoFailures(t *testing.T) *testjson.Execution {
	t.Helper()

	out := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "fail"}
{"Package": "pkg", "Test": "TestTwo", "Action": "run"}
{"Package": "pkg", "Test": "TestTwo", "Action": "fail"}
{"Package": "pkg", "Action": "fail"}
`
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: strings.NewReader(out),
		Stderr: strings.NewReader(""),
	})
	assert.NilError(t, err)
	return exec
}

type fakeWaiter struct {
	result error
}

func (f fakeWaiter) Wait() error {
	return f.result
}

type exitCodeError struct {
	error
	code int
}

func (e exitCodeError) ExitCode() int {
	return e.code
}

func newExitCode(msg string, code int) error {
	return exitCodeError{error: fmt.Errorf("%v", msg), code: code}
}

type noopHandler struct{}

func (s noopHandler) Event(testjson.TestEvent, *testjson.Execution) error {
	return nil
}

func (s noopHandler) Err(string) error {
	return nil
}
