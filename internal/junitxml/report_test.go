package junitxml

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestWrite(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t)

	env.Patch(t, "GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{customTimestamp: new(time.Time).Format(time.RFC3339)})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report.golden")
}

func createExecution(t *testing.T) *testjson.Execution {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: readTestData(t, "out"),
		Stderr: readTestData(t, "err"),
	})
	assert.NilError(t, err)
	return exec
}

func readTestData(t *testing.T, stream string) io.Reader {
	raw, err := ioutil.ReadFile("../../testjson/testdata/go-test-json." + stream)
	assert.NilError(t, err)
	return bytes.NewReader(raw)
}

func TestGoVersion(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		env.Patch(t, "PATH", "/bogus")
		assert.Equal(t, goVersion(), "unknown")
	})

	t.Run("current version", func(t *testing.T) {
		expected := fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		assert.Equal(t, goVersion(), expected)
	})
}
