package testjson

import (
	"bytes"
	"io"
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
// Expected output for the standard-quiet and standard-verbose formats can be
// generated with the same command by removing the -json flag.

type fakeHandler struct {
	inputName string
	formatter EventFormatter
	err       *bytes.Buffer
}

func (s *fakeHandler) Config(t *testing.T) ScanConfig {
	return ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, s.inputName+".out")),
		Stderr:  bytes.NewReader(golden.Get(t, s.inputName+".err")),
		Handler: s,
	}
}

func newFakeHandler(formatter EventFormatter, inputName string) *fakeHandler {
	return &fakeHandler{
		inputName: inputName,
		formatter: formatter,
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

func patchPkgPathPrefix(t *testing.T, val string) {
	var oldVal string
	oldVal, pkgPathPrefix = pkgPathPrefix, val
	t.Cleanup(func() {
		pkgPathPrefix = oldVal
	})
}

func TestFormats_DefaultGoTestJson(t *testing.T) {
	type testCase struct {
		name        string
		format      func(io.Writer) EventFormatter
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		out := new(bytes.Buffer)
		shim := newFakeHandler(tc.format(out), "input/go-test-json")
		exec, err := ScanTestOutput(shim.Config(t))
		assert.NilError(t, err)

		golden.Assert(t, out.String(), tc.expectedOut)
		golden.Assert(t, shim.err.String(), "input/go-test-json.err")

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	testCases := []testCase{
		{
			name: "testname",
			format: func(out io.Writer) EventFormatter {
				return testNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/testname.out",
		},
		{
			name:        "dots-v1",
			format:      dotsFormatV1,
			expectedOut: "format/dots-v1.out",
		},
		{
			name: "pkgname",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/pkgname.out",
		},
		{
			name: "pkgname with hivis",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{UseHiVisibilityIcons: true})
			},
			expectedOut: "format/pkgname-hivis.out",
		},
		{
			name: "pkgname with hide-empty",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{HideEmptyPackages: true})
			},
			expectedOut: "format/pkgname-hide-empty.out",
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
		{
			name:        "standard-json",
			format:      standardJSONFormat,
			expectedOut: "input/go-test-json.out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestFormats_Coverage(t *testing.T) {
	type testCase struct {
		name        string
		format      func(writer io.Writer) EventFormatter
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		patchPkgPathPrefix(t, "gotest.tools")
		out := new(bytes.Buffer)

		shim := newFakeHandler(tc.format(out), "input/go-test-json-with-cover")
		exec, err := ScanTestOutput(shim.Config(t))
		assert.NilError(t, err)

		golden.Assert(t, out.String(), tc.expectedOut)
		golden.Assert(t, shim.err.String(), "go-test.err")

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	testCases := []testCase{
		{
			name: "testname",
			format: func(out io.Writer) EventFormatter {
				return testNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/testname-coverage.out",
		},
		{
			name: "pkgname",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/pkgname-coverage.out",
		},
		{
			name:        "standard-verbose",
			format:      standardVerboseFormat,
			expectedOut: "format/standard-verbose-coverage.out",
		},
		{
			name:        "standard-quiet",
			format:      standardQuietFormat,
			expectedOut: "format/standard-quiet-coverage.out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestFormats_Shuffle(t *testing.T) {
	type testCase struct {
		name        string
		format      func(io.Writer) EventFormatter
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		out := new(bytes.Buffer)
		shim := newFakeHandler(tc.format(out), "input/go-test-json-with-shuffle")
		exec, err := ScanTestOutput(shim.Config(t))
		assert.NilError(t, err)

		golden.Assert(t, out.String(), tc.expectedOut)
		golden.Assert(t, shim.err.String(), "go-test.err")

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	testCases := []testCase{
		{
			name: "testname",
			format: func(out io.Writer) EventFormatter {
				return testNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/testname-shuffle.out",
		},
		{
			name: "pkgname",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/pkgname-shuffle.out",
		},
		{
			name:        "standard-verbose",
			format:      standardVerboseFormat,
			expectedOut: "format/standard-verbose-shuffle.out",
		},
		{
			name:        "standard-quiet",
			format:      standardQuietFormat,
			expectedOut: "format/standard-quiet-shuffle.out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
