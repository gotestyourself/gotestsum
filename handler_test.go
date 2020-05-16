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
