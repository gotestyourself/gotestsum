package testjson

import (
	"bytes"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/opt"
	"gotest.tools/v3/golden"
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
	format func(event TestEvent, output *Execution) string,
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

func TestScanTestOutput_WithTestNameFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(testNameFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-verbose-format.out")
	golden.Assert(t, shim.err.String(), "short-verbose-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

var expectedExecution = &Execution{
	done:    true,
	started: time.Now(),
	errors:  []string{"internal/broken/broken.go:5:21: undefined: somepackage"},
	packages: map[string]*Package{
		"github.com/gotestyourself/gotestyourself/testjson/internal/good": {
			Total: 18,
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action:  ActionPass,
			cached:  true,
			running: map[string]TestCase{},
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
			elapsed: 11 * time.Millisecond,
			action:  ActionFail,
			running: map[string]TestCase{},
		},
		"github.com/gotestyourself/gotestyourself/testjson/internal/badmain": {
			action:  ActionFail,
			running: map[string]TestCase{},
			elapsed: 10 * time.Millisecond,
		},
		"gotest.tools/gotestsum/internal/empty": {
			action:  ActionPass,
			elapsed: 4 * time.Millisecond,
		},
	},
}

var cmpExecutionShallow = gocmp.Options{
	gocmp.AllowUnexported(Execution{}, Package{}),
	gocmp.FilterPath(stringPath("started"), opt.TimeWithThreshold(10*time.Second)),
	cmpopts.IgnoreFields(Execution{}, "errorsLock"),
	cmpopts.EquateEmpty(),
	cmpPackageShallow,
}

var cmpPackageShallow = gocmp.Options{
	gocmp.FilterPath(opt.PathField(Package{}, "output"), gocmp.Ignore()),
	gocmp.FilterPath(opt.PathField(Package{}, "Passed"), gocmp.Ignore()),
	gocmp.FilterPath(opt.PathField(Package{}, "subTests"), gocmp.Ignore()),
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

func TestScanTestOutput_WithPkgNameFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(pkgNameFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format.out")
	golden.Assert(t, shim.err.String(), "short-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutput_WithPkgNameFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(pkgNameFormat, "go-test-json-with-cover")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format-coverage.out")
	golden.Assert(t, shim.err.String(), "short-format-coverage.err")
	assert.DeepEqual(t, exec, expectedCoverageExecution, cmpExecutionShallow)
}

func TestScanTestOutput_WithStandardVerboseFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(standardVerboseFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "go-test-verbose.out")
	golden.Assert(t, shim.err.String(), "go-test-verbose.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutput_WithStandardQuietFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandlerWithAdapter(standardQuietFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutput_WithStandardQuietFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(standardQuietFormat, "go-test-json-with-cover")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format-coverage.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format-coverage.err")
	assert.DeepEqual(t, exec, expectedCoverageExecution, cmpExecutionShallow)
}

var expectedCoverageExecution = &Execution{
	done:    true,
	started: time.Now(),
	errors:  []string{"internal/broken/broken.go:5:21: undefined: somepackage"},
	packages: map[string]*Package{
		"gotest.tools/gotestsum/testjson/internal/good": {
			Total: 18,
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			elapsed:  12 * time.Millisecond,
			action:   ActionPass,
			coverage: "coverage: 0.0% of statements",
			running:  map[string]TestCase{},
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
			elapsed:  11 * time.Millisecond,
			action:   ActionFail,
			coverage: "coverage: 0.0% of statements",
			running:  map[string]TestCase{},
		},
		"gotest.tools/gotestsum/testjson/internal/badmain": {
			action:  ActionFail,
			running: map[string]TestCase{},
			elapsed: time.Millisecond,
		},
	},
}

func TestScanTestOutput_WithStandardVerboseFormat_WithShuffle(t *testing.T) {
	shim := newFakeHandlerWithAdapter(standardVerboseFormat, "go-test-json-with-shuffle")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-verbose-format-shuffle.out")
	golden.Assert(t, shim.err.String(), "go-test.err")
}

func TestScanTestOutput_WithTestNameFormat_WithShuffle(t *testing.T) {
	shim := newFakeHandlerWithAdapter(testNameFormat, "go-test-json-with-shuffle")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "testname-format-shuffle.out")
	golden.Assert(t, shim.err.String(), "go-test.err")
}

func TestScanTestOutput_WithPkgNameFormat_WithShuffle(t *testing.T) {
	shim := newFakeHandlerWithAdapter(pkgNameFormat, "go-test-json-with-shuffle")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "pkgname-format-shuffle.out")
	golden.Assert(t, shim.err.String(), "go-test.err")
}
