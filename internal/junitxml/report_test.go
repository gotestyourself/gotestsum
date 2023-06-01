package junitxml

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"gotest.tools/gotestsum/testjson"
)

func TestWrite(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, "go-test-json")

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report.golden")
}

func TestWriteWithAlwaysIncludeOutput(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, "go-test-json")

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:         "test",
		customTimestamp:     new(time.Time).Format(time.RFC3339),
		customElapsed:       "2.1",
		AlwaysIncludeOutput: true,
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-always-include-output.golden")
}



func TestWrite_HideEmptyPackages(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, "go-test-json")

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:       "test",
		HideEmptyPackages: true,
		customTimestamp:   new(time.Time).Format(time.RFC3339),
		customElapsed:     "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-skip-empty.golden")
}

func TestWriteReruns(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, "go-test-json-reruns")

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:       "test",
		HideEmptyPackages: true,
		customTimestamp:   new(time.Time).Format(time.RFC3339),
		customElapsed:     "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-reruns.golden")
}

func createExecution(t *testing.T, input string) *testjson.Execution {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: readTestData(t, input, "out"),
		Stderr: readTestData(t, input, "err"),
	})
	assert.NilError(t, err)
	return exec
}

func readTestData(t *testing.T, input, stream string) io.Reader {
	raw, err := ioutil.ReadFile("../../testjson/testdata/input/"+input+"." + stream)
	assert.NilError(t, err)
	fmt.Println(string(raw))
	return bytes.NewReader(raw)
}

func TestGoVersion(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		t.Setenv("PATH", "/bogus")
		assert.Equal(t, goVersion(), "unknown")
	})

	t.Run("current version", func(t *testing.T) {
		expected := fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		assert.Equal(t, goVersion(), expected)
	})
}
