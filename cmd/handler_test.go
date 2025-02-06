package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/gotestsum/internal/junitxml"
	"gotest.tools/gotestsum/internal/text"
	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestPostRunHook(t *testing.T) {
	command := &commandValue{}
	err := command.Set("go run ./testdata/postrunhook/main.go")
	assert.NilError(t, err)

	buf := new(bytes.Buffer)
	opts := &options{
		postRunHookCmd:       command,
		jsonFile:             "events.json",
		jsonFileTimingEvents: "timing.json",
		junitFile:            "junit.xml",
		stdout:               buf,
	}

	t.Setenv("GOTESTSUM_FORMAT", "short")
	t.Setenv("GOTESTSUM_FORMAT_ICONS", "default")

	exec := newExecFromTestData(t)
	err = postRunHook(opts, exec)
	assert.NilError(t, err)

	actual := text.ProcessLines(t, buf, func(line string) string {
		if strings.HasPrefix(line, "GOTESTSUM_ELAPSED=0.0") &&
			strings.HasSuffix(line, "s") {
			i := strings.Index(line, "=")
			return line[:i] + "=0.000s"
		}
		return line
	})
	golden.Assert(t, actual, "post-run-hook-expected")
}

func newExecFromTestData(t *testing.T) *testjson.Execution {
	t.Helper()
	f, err := os.Open("../testjson/testdata/input/go-test-json.out")
	assert.NilError(t, err)
	defer f.Close() //nolint:errcheck

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: f,
		Stderr: strings.NewReader(""),
	})
	assert.NilError(t, err)
	return exec
}

type bufferCloser struct {
	bytes.Buffer
}

func (bufferCloser) Close() error { return nil }

func (bufferCloser) Sync() error { return nil }

func TestEventHandler_Event_WithMissingActionFail(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "no")

	buf := new(bufferCloser)
	errBuf := new(bytes.Buffer)
	format := testjson.NewEventFormatter(errBuf, "testname", testjson.FormatOptions{})

	source := golden.Get(t, "../../testjson/testdata/input/go-test-json-missing-test-fail.out")
	cfg := testjson.ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: &eventHandler{jsonFile: buf, formatter: format},
	}
	_, err := testjson.ScanTestOutput(cfg)
	assert.NilError(t, err)

	assert.Equal(t, buf.String(), string(source))
	// confirm the artificial event was sent to the handler by checking the output
	// of the formatter.
	golden.Assert(t, errBuf.String(), "event-handler-missing-test-fail-expected")
}

func TestEventHandler_Event_MaxFails(t *testing.T) {
	format := testjson.NewEventFormatter(io.Discard, "testname", testjson.FormatOptions{})

	source := golden.Get(t, "../../testjson/testdata/input/go-test-json.out")
	cfg := testjson.ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: &eventHandler{formatter: format, maxFails: 2},
	}

	_, err := testjson.ScanTestOutput(cfg)
	assert.Error(t, err, "ending test run because max failures was reached")
}

func TestNewEventHandler_CreatesDirectory(t *testing.T) {
	dir := fs.NewDir(t, t.Name())
	jsonFile := filepath.Join(dir.Path(), "new-path", "log.json")

	opts := &options{
		stdout:   new(bytes.Buffer),
		format:   "testname",
		jsonFile: jsonFile,
	}
	_, err := newEventHandler(opts)
	assert.NilError(t, err)

	_, err = os.Stat(jsonFile)
	assert.NilError(t, err)
}

func TestWriteJunitFile_CreatesDirectory(t *testing.T) {
	dir := fs.NewDir(t, t.Name())
	junitFile := filepath.Join(dir.Path(), "new-path", "junit.xml")

	opts := &options{
		junitFile:                    junitFile,
		junitTestCaseClassnameFormat: &junitFieldFormatValue{},
		junitTestSuiteNameFormat:     &junitFieldFormatValue{},
	}
	exec := &testjson.Execution{}
	err := writeJUnitFile(opts, exec)
	assert.NilError(t, err)

	_, err = os.Stat(junitFile)
	assert.NilError(t, err)
}

func TestScanTestOutput_TestTimeoutPanicRace(t *testing.T) {
	run := func(t *testing.T, name string) {
		format := testjson.NewEventFormatter(io.Discard, "testname", testjson.FormatOptions{})

		source := golden.Get(t, "input/go-test-json-"+name+".out")
		cfg := testjson.ScanConfig{
			Stdout:  bytes.NewReader(source),
			Handler: &eventHandler{formatter: format},
		}
		exec, err := testjson.ScanTestOutput(cfg)
		assert.NilError(t, err)

		out := new(bytes.Buffer)
		testjson.PrintSummary(out, exec, testjson.SummarizeAll)

		actual := text.ProcessLines(t, out, text.OpRemoveSummaryLineElapsedTime)
		golden.Assert(t, actual, "expected/"+name+"-summary")

		var buf bytes.Buffer
		err = junitxml.Write(&buf, exec, junitxml.Config{})
		assert.NilError(t, err)

		assert.Assert(t, cmp.Contains(buf.String(), "panic: test timed out"))
	}

	testCases := []string{
		"panic-race-1",
		"panic-race-2",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			run(t, tc)
		})
	}
}
