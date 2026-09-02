package junitxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
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

func TestWrite_NestedFailures(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json-with-nested-fails.out"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:     "test",
		customTimestamp: new(time.Time).Format(time.RFC3339),
		customElapsed:   "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-nested-fails.golden")

	// Every level of a failing test is reported as a separate failure.
	suites := decodeTestSuites(t, out.String())
	assert.Equal(t, suites.Tests, 8)
	assert.Equal(t, suites.Failures, 6)
	assertCountsMatchTestCases(t, suites)
}

func TestWrite_HideParentFailures(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json-with-nested-fails.out"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:        "test",
		HideParentFailures: true,
		customTimestamp:    new(time.Time).Format(time.RFC3339),
		customElapsed:      "2.1",
	})
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "junitxml-report-hide-parent-failures.golden")

	// Only the tests that failed without a failing subtest remain:
	// TestParentFailsWithSubtestFailure/fails, TestParentFailsWithoutSubtestFailure,
	// and TestDeeplyNested/b/c.
	suites := decodeTestSuites(t, out.String())
	assert.Equal(t, suites.Tests, 5)
	assert.Equal(t, suites.Failures, 3)
	assertCountsMatchTestCases(t, suites)
}

func TestWrite_HideParentFailures_WithoutFailures(t *testing.T) {
	out := new(bytes.Buffer)
	exec := createExecution(t, testjson.ScanConfig{
		Stdout: readTestData(t, "go-test-json.out"),
		Stderr: readTestData(t, "go-test-json.err"),
	})

	t.Setenv("GOVERSION", "go7.7.7")
	err := Write(out, exec, Config{
		ProjectName:        "test",
		HideParentFailures: true,
		customTimestamp:    new(time.Time).Format(time.RFC3339),
		customElapsed:      "2.1",
	})
	assert.NilError(t, err)

	// Two parent tests, TestNestedWithFailure and TestNestedParallelFailures,
	// are removed from the 59 tests and 13 failures of the unfiltered report.
	suites := decodeTestSuites(t, out.String())
	assert.Equal(t, suites.Tests, 57)
	assert.Equal(t, suites.Failures, 11)
	assertCountsMatchTestCases(t, suites)
}

// reportTestSuites is used to decode a report, because the slice fields of
// JUnitTestSuites and JUnitTestSuite have no xml tag to match on.
type reportTestSuites struct {
	XMLName  xml.Name          `xml:"testsuites"`
	Tests    int               `xml:"tests,attr"`
	Failures int               `xml:"failures,attr"`
	Suites   []reportTestSuite `xml:"testsuite"`
}

type reportTestSuite struct {
	Name      string `xml:"name,attr"`
	Tests     int    `xml:"tests,attr"`
	Failures  int    `xml:"failures,attr"`
	Skipped   int    `xml:"skipped,attr"`
	TestCases []struct {
		Name        string            `xml:"name,attr"`
		Failure     *JUnitFailure     `xml:"failure"`
		SkipMessage *JUnitSkipMessage `xml:"skipped"`
	} `xml:"testcase"`
}

func decodeTestSuites(t *testing.T, raw string) reportTestSuites {
	t.Helper()
	var suites reportTestSuites
	assert.NilError(t, xml.Unmarshal([]byte(raw), &suites))
	return suites
}

// assertCountsMatchTestCases checks that the counts on each testsuite element
// are consistent with the testcases it contains.
func assertCountsMatchTestCases(t *testing.T, suites reportTestSuites) {
	t.Helper()
	for _, suite := range suites.Suites {
		suiteFailures, suiteSkipped := 0, 0
		for _, tc := range suite.TestCases {
			// The testcase used to report a package level failure is not
			// included in any of the counts.
			if tc.Name == "TestMain" {
				continue
			}
			if tc.Failure != nil {
				suiteFailures++
			}
			if tc.SkipMessage != nil {
				suiteSkipped++
			}
		}
		assert.Equal(t, suite.Failures, suiteFailures, "failures in testsuite %v", suite.Name)
		assert.Equal(t, suite.Skipped, suiteSkipped, "skipped in testsuite %v", suite.Name)
	}
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
