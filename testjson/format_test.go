package testjson

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

// go-test-json files are generated using the following command:
//
//   go test -p=1 -parallel=1 -json -tags=stubpkg ./testjson/internal/...
//
// Additional flags (ex: -cover, -shuffle) may be added to test different
// scenarios.
//
// There are also special package scenarios:
//
//   -tags="stubpkg timeout"
//   -tags="stubpkg panic"
//
// Expect output for the standard-quiet and standard-verbose formats can be
// generated with the same command by removing the -json flag.

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

func TestFormats_DefaultGoTestJson(t *testing.T) {
	type testCase struct {
		name        string
		format      func(event TestEvent, exec *Execution) string
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		shim := newFakeHandlerWithAdapter(tc.format, "input/go-test-json")
		exec, err := ScanTestOutput(shim.Config(t))
		assert.NilError(t, err)

		golden.Assert(t, shim.out.String(), tc.expectedOut)
		golden.Assert(t, shim.err.String(), "input/go-test-json.err")

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	testCases := []testCase{
		{
			name:        "testname",
			format:      testNameFormat,
			expectedOut: "format/testname.out",
		},
		{
			name:        "dots-v1",
			format:      dotsFormatV1,
			expectedOut: "format/dots-v1.out",
		},
		{
			name:        "pkgname",
			format:      pkgNameFormat,
			expectedOut: "format/pkgname.out",
		},
		{
			name:        "standard-verbose",
			format:      standardVerboseFormat,
			expectedOut: "format/standard-verbose.out",
		},
		{
			name:        "standard-quiet",
			format:      standardQuietFormat,
			expectedOut: "format/standard-quiet.out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestScanTestOutput_WithPkgNameFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(pkgNameFormat, "go-test-json-with-cover")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format-coverage.out")
	golden.Assert(t, shim.err.String(), "short-format-coverage.err")
}

func TestScanTestOutput_WithStandardQuietFormat_WithCoverage(t *testing.T) {
	defer patchPkgPathPrefix("gotest.tools")()

	shim := newFakeHandlerWithAdapter(standardQuietFormat, "go-test-json-with-cover")
	_, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format-coverage.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format-coverage.err")
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
