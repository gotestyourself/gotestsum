package testjson

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
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
	exec := NewExecution()
	exec.add(TestEvent{
		Package: "mytestpkg",
		Action:  ActionOutput,
		Output:  "coverage: 33.1% of statements\n",
	})

	pkg := exec.Package("mytestpkg")
	expected := &Package{
		coverage: "coverage: 33.1% of statements",
		output: map[string][]string{
			"": {"coverage: 33.1% of statements\n"},
		},
		running: map[string]TestCase{},
	}
	assert.DeepEqual(t, pkg, expected, cmpPackage)
}

var cmpPackage = cmp.AllowUnexported(Package{})
