package testjson

import (
	"bytes"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
	"gotest.tools/assert/opt"
	"gotest.tools/golden"
)

//go:generate ./generate.sh

type fakeHandler struct {
	inputName string
	formatter EventFormatter
	out       *bytes.Buffer
	err       *bytes.Buffer
}

func (s *fakeHandler) Config(t *testing.T) ScanConfig {
	return ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, s.inputName+".out")),
		Stderr:  bytes.NewReader(golden.Get(t, s.inputName+".err")),
		Handler: s,
	}
}

func newFakeHandlerWithAdapter(
	format func(event TestEvent, output *Execution) (string, error),
	inputName string,
) *fakeHandler {
	out := new(bytes.Buffer)
	return &fakeHandler{
		inputName: inputName,
		formatter: &formatAdapter{out: out, format: format},
		out:       out,
		err:       new(bytes.Buffer),
	}
}

func newFakeHandler(formatter EventFormatter, inputName string) *fakeHandler {
	return &fakeHandler{
		inputName: inputName,
		formatter: formatter,
		out:       new(bytes.Buffer),
		err:       new(bytes.Buffer),
	}
}

func (s *fakeHandler) Event(event TestEvent, execution *Execution) error {
	return s.formatter.Format(event, execution)
}

func (s *fakeHandler) Err(text string) error {
	s.err.WriteString(text + "\n")
	return nil
}

func patchPkgPathPrefix(val string) func() {
	var oldVal string
	oldVal, pkgPathPrefix = pkgPathPrefix, val
	return func() { pkgPathPrefix = oldVal }
}

func TestScanTestOutputWithShortVerboseFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(shortVerboseFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-verbose-format.out")
	golden.Assert(t, shim.err.String(), "short-verbose-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

var expectedExecution = &Execution{
	started: time.Now(),
	errors:  []string{"internal/broken/broken.go:5:21: undefined: somepackage"},
	packages: map[string]*Package{
		"github.com/gotestyourself/gotestyourself/testjson/internal/good": {
			Total: 18,
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action: ActionPass,
			cached: true,
		},
		"github.com/gotestyourself/gotestyourself/testjson/internal/stub": {
			Total: 28,
			Failed: []TestCase{
				{Test: "TestFailed"},
				{Test: "TestFailedWithStderr"},
				{Test: "TestNestedWithFailure/c"},
				{Test: "TestNestedWithFailure"},
			},
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action: ActionFail,
		},
		"github.com/gotestyourself/gotestyourself/testjson/internal/badmain": {
			action: ActionFail,
		},
	},
}

var cmpExecutionShallow = gocmp.Options{
	gocmp.AllowUnexported(Execution{}, Package{}),
	gocmp.FilterPath(stringPath("started"), opt.TimeWithThreshold(10*time.Second)),
	cmpPackageShallow,
}

var cmpPackageShallow = gocmp.Options{
	// TODO: use opt.PathField(Package{}, "output")
	gocmp.FilterPath(stringPath("packages.output"), gocmp.Ignore()),
	gocmp.FilterPath(stringPath("packages.Passed"), gocmp.Ignore()),
	gocmp.Comparer(func(x, y TestCase) bool {
		return x.Test == y.Test
	}),
}

func stringPath(spec string) func(gocmp.Path) bool {
	return func(path gocmp.Path) bool {
		return path.String() == spec
	}
}

func TestScanTestOutputWithDotsFormatV1(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(dotsFormatV1, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "dots-v1-format.out")
	golden.Assert(t, shim.err.String(), "dots-v1-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithShortFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(shortFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format.out")
	golden.Assert(t, shim.err.String(), "short-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithShortFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(shortFormat, "go-test-json-with-cover")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format-coverage.out")
	golden.Assert(t, shim.err.String(), "short-format-coverage.err")
	assert.DeepEqual(t, exec, expectedCoverageExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithStandardVerboseFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(standardVerboseFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "go-test-verbose.out")
	golden.Assert(t, shim.err.String(), "go-test-verbose.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithStandardQuietFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(standardQuietFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithStandardQuietFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(standardQuietFormat, "go-test-json-with-cover")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format-coverage.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format-coverage.err")
	assert.DeepEqual(t, exec, expectedCoverageExecution, cmpExecutionShallow)
}

var expectedCoverageExecution = &Execution{
	started: time.Now(),
	errors:  []string{"internal/broken/broken.go:5:21: undefined: somepackage"},
	packages: map[string]*Package{
		"gotest.tools/gotestsum/testjson/internal/good": {
			Total: 18,
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action:   ActionPass,
			coverage: "coverage: 0.0% of statements",
		},
		"gotest.tools/gotestsum/testjson/internal/stub": {
			Total: 28,
			Failed: []TestCase{
				{Test: "TestFailed"},
				{Test: "TestFailedWithStderr"},
				{Test: "TestNestedWithFailure/c"},
				{Test: "TestNestedWithFailure"},
			},
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action:   ActionFail,
			coverage: "coverage: 0.0% of statements",
		},
		"gotest.tools/gotestsum/testjson/internal/badmain": {
			action: ActionFail,
		},
	},
}
