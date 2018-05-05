package junitxml

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/golden"
	"gotest.tools/gotestsum/testjson"
)

func TestWrite(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t)

	err := Write(out, exec)
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report.golden")
}

func createExecution(t *testing.T) *testjson.Execution {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  readTestData(t, "out"),
		Stderr:  readTestData(t, "err"),
		Handler: &noopHandler{},
	})
	assert.NilError(t, err)
	return exec
}

func readTestData(t *testing.T, stream string) io.Reader {
	raw, err := ioutil.ReadFile("../../testjson/testdata/go-test-json." + stream)
	assert.NilError(t, err)
	return bytes.NewReader(raw)
}

type noopHandler struct{}

func (s *noopHandler) Event(testjson.TestEvent, *testjson.Execution) error {
	return nil
}

func (s *noopHandler) Err(string) error {
	return nil
}
