package junitxml

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestWrite(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json.out"),
		Stderr: readTestData(t, "go-test-json.err"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report.golden")
}

func TestWrite_HideEmptyPackages(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json.out"),
		Stderr: readTestData(t, "go-test-json.err"),
	})

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

func TestWrite_HideSkippedTests(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json.out"),
		Stderr: readTestData(t, "go-test-json.err"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:      "test",
		HideSkippedTests: true,
		customTimestamp:  new(time.Time).Format(time.RFC3339),
		customElapsed:    "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-hide-skipped-tests.golden")
}

func TestWrite_WithAttributes(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json-with-attributes.out"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-tc-with-attributes.golden")
}

func createExecution(t *testing.T, config testjson.ScanConfig) *testjson.Execution {
	exec, err := testjson.ScanTestOutput(config)
	assert.NilError(t, err)
	return exec
}

func readTestData(t *testing.T, inputFile string) io.Reader {
	raw, err := os.ReadFile(path.Join("../../testjson/testdata/input/", inputFile))
	assert.NilError(t, err)
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

func Test_encodeAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
		want       *JUnitProperties
	}{
		{
			name:       "should handle empty attributes",
			attributes: nil,
			want:       nil,
		},
		{
			name: "should encode attributes",
			attributes: map[string]string{
				"hello": "world",
			},
			want: &JUnitProperties{Properties: []JUnitProperty{
				{Name: "hello", Value: "world"},
			}},
		},
		{
			name: "should encode attributes in order",
			attributes: map[string]string{
				"a": "1",
				"d": "4",
				"c": "3",
				"b": "2",
				"e": "5",
			},
			want: &JUnitProperties{Properties: []JUnitProperty{
				{Name: "a", Value: "1"},
				{Name: "b", Value: "2"},
				{Name: "c", Value: "3"},
				{Name: "d", Value: "4"},
				{Name: "e", Value: "5"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeAttributes(tt.attributes)
			assert.DeepEqual(t, tt.want, got)
		})
	}
}

func TestWrite_PausedTestsAtTimeout(t *testing.T) {
	in := strings.NewReader(`{"Action":"start","Package":"example.com/p"}
{"Action":"run","Package":"example.com/p","Test":"TestRunning"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"=== RUN   TestRunning\n"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"=== PAUSE TestRunning\n"}
{"Action":"pause","Package":"example.com/p","Test":"TestRunning"}
{"Action":"run","Package":"example.com/p","Test":"TestPaused"}
{"Action":"output","Package":"example.com/p","Test":"TestPaused","Output":"=== RUN   TestPaused\n"}
{"Action":"output","Package":"example.com/p","Test":"TestPaused","Output":"=== PAUSE TestPaused\n"}
{"Action":"pause","Package":"example.com/p","Test":"TestPaused"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"=== CONT  TestRunning\n"}
{"Action":"cont","Package":"example.com/p","Test":"TestRunning"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"panic: test timed out after 1s\n"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"\trunning tests:\n"}
{"Action":"output","Package":"example.com/p","Test":"TestRunning","Output":"\t\tTestRunning (1s)\n"}
{"Action":"output","Package":"example.com/p","Output":"FAIL\texample.com/p\t1.002s\n"}
{"Action":"fail","Package":"example.com/p","Elapsed":1.002}
`)
	exec := createExecution(t, testjson.ScanConfig{Stdout: in})
	t.Setenv("GOVERSION", "go7.7.7")

	suites := generate(exec, Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "1.0",
	})
	assert.Equal(t, suites.Tests, 2)
	assert.Equal(t, suites.Failures, 2)
	assert.Equal(t, len(suites.Suites), 1)

	suite := suites.Suites[0]
	assert.Equal(t, suite.Failures, 1)
	names := make([]string, 0, len(suite.TestCases))
	for _, tc := range suite.TestCases {
		names = append(names, tc.Name)
	}
	assert.DeepEqual(t, names, []string{"TestMain", "TestRunning"})
}
