package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestPostRunHook(t *testing.T) {
	command := &commandValue{}
	err := command.Set("go run ./testdata/postrunhook/main.go")
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	opts := &options{
		postRunHookCmd: command,
		jsonFile:       "events.json",
		junitFile:      "junit.xml",
		stdout:         buf,
	}

	defer env.Patch(t, "GOTESTSUM_FORMAT", "short")()

	exec := newExecFromTestData(t)
	err = postRunHook(opts, exec)
	assert.NilError(t, err)
	golden.Assert(t, buf.String(), "post-run-hook-expected")
}

func newExecFromTestData(t *testing.T) *testjson.Execution {
	t.Helper()
	f, err := os.Open("testjson/testdata/go-test-json.out")
	assert.NilError(t, err)
	defer f.Close() // nolint: errcheck

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: f,
		Stderr: strings.NewReader(""),
	})
	assert.NilError(t, err)
	return exec
}

type bufferCloser struct {
	bytes.Buffer
}

func (bufferCloser) Close() error { return nil }

func TestEventHandler_Event_WithMissingActionFail(t *testing.T) {
	buf := new(bufferCloser)
	errBuf := new(bytes.Buffer)
	format := testjson.NewEventFormatter(errBuf, "testname")

	source := golden.Get(t, "../testjson/testdata/go-test-json-missing-test-fail.out")
	cfg := testjson.ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: &eventHandler{jsonFile: buf, formatter: format},
	}
	_, err := testjson.ScanTestOutput(cfg)
	assert.NilError(t, err)

	assert.Equal(t, buf.String(), string(source))
	// confirm the artificial event was sent to the handler by checking the output
	// of the formatter.
	golden.Assert(t, errBuf.String(), "event-handler-missing-test-fail-expected")
}
