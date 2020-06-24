package main

import (
	"bytes"
	"io/ioutil"
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

	raw, err := ioutil.ReadFile(reportFile.Path())
	assert.NilError(t, err)
	golden.Assert(t, string(raw), t.Name()+"-expected")
}
