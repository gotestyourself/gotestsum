package testjson

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestPackage_Elapsed(t *testing.T) {
	pkg := &Package{
		Failed: []TestCase{
			{Elapsed: 300 * time.Millisecond},
		},
		Passed: []TestCase{
			{Elapsed: 200 * time.Millisecond},
			{Elapsed: 2500 * time.Millisecond},
		},
		Skipped: []TestCase{
			{Elapsed: 100 * time.Millisecond},
		},
	}
	assert.Equal(t, pkg.Elapsed(), 3100*time.Millisecond)
}

func TestExecution_Add_PackageCoverage(t *testing.T) {
	exec := newExecution()
	exec.add(TestEvent{
		Package: "mytestpkg",
		Action:  ActionOutput,
		Output:  "coverage: 33.1% of statements\n",
	})

	pkg := exec.Package("mytestpkg")
	expected := &Package{
		coverage: "coverage: 33.1% of statements",
		output: map[int][]string{
			0: {"coverage: 33.1% of statements\n"},
		},
		running: map[string]TestCase{},
	}
	assert.DeepEqual(t, pkg, expected, cmpPackage)
}

var cmpPackage = cmp.AllowUnexported(Package{})

func TestScanTestOutput_MinimalConfig(t *testing.T) {
	in := bytes.NewReader(golden.Get(t, "go-test-json.out"))
	exec, err := ScanTestOutput(ScanConfig{Stdout: in})
	assert.NilError(t, err)
	// a weak check to show that all the stdout was scanned
	assert.Equal(t, exec.Total(), 46)
}
