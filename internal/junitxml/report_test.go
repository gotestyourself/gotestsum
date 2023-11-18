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
	t.Setenv("GOVERSION", "go7.7.7")

	out := new(bufferCloser)
	exec := createExecution(t)
	cfg := Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "2.1",
	}
	enc := NewEncoder(out, cfg)
	assert.NilError(t, enc.Handle(testjson.TestEvent{}, exec))

	err := enc.Close()
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report.golden")
}

type bufferCloser struct {
	bytes.Buffer
}

func (bufferCloser) Close() error { return nil }

func TestWrite_HideEmptyPackages(t *testing.T) {
	t.Setenv("GOVERSION", "go7.7.7")

	out := new(bufferCloser)
	exec := createExecution(t)
	cfg := Config{
		ProjectName:       "test",
		HideEmptyPackages: true,
		customTimestamp:   new(time.Time).Format(time.RFC3339),
		customElapsed:     "2.1",
	}
	enc := NewEncoder(out, cfg)
	assert.NilError(t, enc.Handle(testjson.TestEvent{}, exec))

	err := enc.Close()
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-skip-empty.golden")
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
	raw, err := ioutil.ReadFile("../../testjson/testdata/input/go-test-json." + stream)
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
