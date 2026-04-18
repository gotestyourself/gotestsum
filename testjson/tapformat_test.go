package testjson

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestTapFormat_VersionLine(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()
	err := formatter.Format(TestEvent{
		Action:  ActionPass,
		Package: "github.com/example/pkg",
		Test:    "TestOne",
		Elapsed: 0.01,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("TAP version 13")), "expected TAP version line")
}

func TestTapFormat_TestLine(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()
	err := formatter.Format(TestEvent{
		Action:  ActionPass,
		Package: "github.com/example/pkg",
		Test:    "TestExample",
		Elapsed: 0.05,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("ok 1 - github.com/example/pkg.TestExample")), "expected test line")
	assert.Assert(t, bytes.Contains([]byte(got), []byte("time=")), "expected time comment")
}

func TestTapFormat_SkipLine(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()
	err := formatter.Format(TestEvent{
		Action:  ActionSkip,
		Package: "github.com/example/pkg",
		Test:    "TestSkipped",
		Elapsed: 0.001,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("ok 1 - github.com/example/pkg.TestSkipped")), "expected ok line for skip")
	assert.Assert(t, bytes.Contains([]byte(got), []byte("# SKIP")), "expected SKIP directive")
}

func TestTapFormat_FailLine(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()
	err := formatter.Format(TestEvent{
		Action:  ActionFail,
		Package: "github.com/example/pkg",
		Test:    "TestFailed",
		Elapsed: 0.1,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("not ok 1 - github.com/example/pkg.TestFailed")), "expected not ok line for fail")
}

func TestTapFormat_WithOutput(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()

	// First, send output event
	err := formatter.Format(TestEvent{
		Package: "github.com/example/pkg",
		Test:    "TestWithOutput",
		Action:  ActionOutput,
		Output:  "debug message\n",
	}, exec)
	assert.NilError(t, err)

	// Then, send fail event
	err = formatter.Format(TestEvent{
		Package: "github.com/example/pkg",
		Test:    "TestWithOutput",
		Action:  ActionFail,
		Elapsed: 0.05,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("# debug message")), "expected output as diagnostic")
}

func TestTapFormat_MultipleTests(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	exec := newExecution()

	// Test 1
	err := formatter.Format(TestEvent{
		Action:  ActionPass,
		Package: "github.com/example/pkg",
		Test:    "TestOne",
		Elapsed: 0.01,
	}, exec)
	assert.NilError(t, err)

	// Test 2
	err = formatter.Format(TestEvent{
		Action:  ActionPass,
		Package: "github.com/example/pkg",
		Test:    "TestTwo",
		Elapsed: 0.02,
	}, exec)
	assert.NilError(t, err)

	// Test 3
	err = formatter.Format(TestEvent{
		Action:  ActionFail,
		Package: "github.com/example/pkg",
		Test:    "TestThree",
		Elapsed: 0.03,
	}, exec)
	assert.NilError(t, err)

	got := out.String()
	assert.Assert(t, bytes.Contains([]byte(got), []byte("ok 1 - github.com/example/pkg.TestOne")), "expected test 1")
	assert.Assert(t, bytes.Contains([]byte(got), []byte("ok 2 - github.com/example/pkg.TestTwo")), "expected test 2")
	assert.Assert(t, bytes.Contains([]byte(got), []byte("not ok 3 - github.com/example/pkg.TestThree")), "expected test 3")
}

func TestTapFormat_Golden(t *testing.T) {
	out := new(bytes.Buffer)
	formatter := tapFormat(out)

	shim := newFakeHandler(formatter, "input/go-test-json-tap-sample")
	exec, err := ScanTestOutput(shim.Config(t))
	assert.NilError(t, err)

	golden.Assert(t, out.String(), "tapformat-golden.tap")
	assert.Equal(t, len(exec.Failed()), 1)
	assert.Equal(t, len(exec.Skipped()), 1)
	assert.Assert(t, exec.Total() >= 3)
}
