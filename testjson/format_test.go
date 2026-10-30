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
			name: "testdox",
			format: func(out io.Writer) EventFormatter {
				return testDoxFormat(out, FormatOptions{})
			},
			expectedOut: "format/testdox.out",
		},
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
			name: "pkgname",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{})
			},
			expectedOut: "format/pkgname.out",
		},
		{
			name: "pkgname with hivis",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{Icons: "hivis"})
			},
			expectedOut: "format/pkgname-hivis.out",
		},
		{
			name: "pkgname with text",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{Icons: "text"})
			},
			expectedOut: "format/pkgname-text.out",
		},
		{
			name: "pkgname with codicons",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{Icons: "codicons"})
			},
			expectedOut: "format/pkgname-codicons.out",
		},
		{
			name: "pkgname with octicons",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{Icons: "octicons"})
			},
			expectedOut: "format/pkgname-octicons.out",
		},
		{
			name: "pkgname with emoticons",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{Icons: "emoticons"})
			},
			expectedOut: "format/pkgname-emoticons.out",
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
		{
			name:        "github-actions",
			format:      githubActionsFormat,
			expectedOut: "format/github-actions.out",
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
		input       string
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}

	run := func(t *testing.T, tc testCase) {
		patchPkgPathPrefix(t, "gotest.tools")
		out := new(bytes.Buffer)

		if tc.input == "" {
			tc.input = "input/go-test-json-with-cover"
		}

		shim := newFakeHandler(tc.format(out), tc.input)
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
			name: "testdox",
			format: func(out io.Writer) EventFormatter {
				return testDoxFormat(out, FormatOptions{})
			},
			expectedOut: "format/testdox-coverage.out",
		},
		{
			name:        "testname go1.19-",
			format:      testNameFormat,
			expectedOut: "format/testname-coverage.out",
		},
		{
			name:        "testname go1.20+",
			format:      testNameFormat,
			input:       "input/go-test-json-with-cover-go1.20",
			expectedOut: "format/testname-coverage-go1.20.out",
		},
		{
			name: "pkgname go1.19-",
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
			name:        "standard-quiet go1.19-",
			format:      standardQuietFormat,
			expectedOut: "format/standard-quiet-coverage.out",
		},
		{
			name: "pkgname go1.20+",
			format: func(out io.Writer) EventFormatter {
				return pkgNameFormat(out, FormatOptions{})
			},
			input:       "input/go-test-json-with-cover-go1.20",
			expectedOut: "format/pkgname-coverage-go1.20.out",
		},
		{
			name:        "standard-quiet go.20+",
			format:      standardQuietFormat,
			input:       "input/go-test-json-with-cover-go1.20",
			expectedOut: "format/standard-quiet-coverage-go1.20.out",
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
			name: "testdox",
			format: func(out io.Writer) EventFormatter {
				return testDoxFormat(out, FormatOptions{})
			},
			expectedOut: "format/testdox-shuffle.out",
		},
		{
			name:        "testname",
			format:      testNameFormat,
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

// The input is the Go 1.22.1 output from https://github.com/gotestyourself/gotestsum/issues/391.
func TestTestNameFormat_CoverProfile(t *testing.T) {
	patchPkgPathPrefix(t, "")
	out := new(bytes.Buffer)
	shim := newFakeHandler(testNameFormat(out), "")
	_, err := ScanTestOutput(ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, "input/go-test-json-with-coverprofile.out")),
		Handler: shim,
	})
	assert.NilError(t, err)
	assert.Equal(t, shim.err.String(), "")
	assert.Equal(t, out.String(), "PASS example/hello.TestHello (0.00s)\n"+
		"PASS example/hello/apackage.TestFoo (0.00s)\n"+
		"PASS example/hello (coverage: 0.0% of statements)\n"+
		"PASS example/hello/apackage (coverage: 100.0% of statements)\n"+
		"EMPTY example/hello/anotherpkg (coverage: 0.0% of statements)\n")
}

func TestIsPkgFailureOutput_Coverage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		output  string
		failure bool
	}{
		{name: "coverage", output: "coverage: 33.3% of statements\n"},
		{name: "coverpkg", output: "coverage: 33.3% of statements in ./...\n"},
		{name: "no statements", output: "coverage: [no statements]\n"},
		{name: "go1.20 coverage", output: "\texample/pkg\tcoverage: 33.3% of statements\n"},
		{name: "go1.20 no statements", output: "\texample/pkg\tcoverage: [no statements]\n"},
		{name: "empty package", output: "\texample/pkg\t\tcoverage: 0.0% of statements\n"},
		{name: "empty package coverpkg", output: "\texample/pkg\t\tcoverage: 0.0% of statements in ./...\n"},
		{name: "panic", output: "panic: failed to initialize\n", failure: true},
		{name: "build error", output: "./main.go:3: undefined: value\n", failure: true},
		{name: "testmain failure", output: "TestMain: setup failed\n", failure: true},
		{name: "coverage error", output: "coverage: failed to write profile\n", failure: true},
		{name: "package coverage error", output: "\texample/pkg\tcoverage: failed to write profile\n", failure: true},
		{
			name:    "coverage error with percentage",
			output:  "coverage: failed to write profile after measuring 12.3% of statements\n",
			failure: true,
		},
		{name: "coverage trailing diagnostic", output: "coverage: 12.3% of statements: failed\n", failure: true},
		{name: "coverage mention", output: "expected coverage: 100.0% of statements\n", failure: true},
		{name: "other package", output: "\tother/pkg\tcoverage: 0.0% of statements\n", failure: true},
		{name: "package prefix", output: "\texample/pkg/other\tcoverage: 0.0% of statements\n", failure: true},
		{name: "package suffix", output: "\tother/example/pkg\tcoverage: 0.0% of statements\n", failure: true},
		{name: "package diagnostic", output: "\texample/pkg\tfailed to initialize\n", failure: true},
		{name: "indented test output", output: "    coverage: 100.0% of statements\n", failure: true},
		{name: "tab-indented output", output: "\tcoverage: 100.0% of statements\n", failure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := TestEvent{Action: ActionOutput, Package: "example/pkg", Output: tc.output}
			assert.Equal(t, isPkgFailureOutput(event), tc.failure)
			event.Test = "TestCoverage/subtest"
			assert.Assert(t, !isPkgFailureOutput(event))
		})
	}
}
