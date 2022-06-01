package testjson

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

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

var cmpPackage = cmp.Options{
	cmp.AllowUnexported(Package{}),
	cmpopts.EquateEmpty(),
}

func TestScanTestOutput_MinimalConfig(t *testing.T) {
	in := bytes.NewReader(golden.Get(t, "go-test-json.out"))
	exec, err := ScanTestOutput(ScanConfig{Stdout: in})
	assert.NilError(t, err)
	// a weak check to show that all the stdout was scanned
	assert.Equal(t, exec.Total(), 46)
}

func TestScanTestOutput_CallsStopOnError(t *testing.T) {
	var called bool
	stop := func() {
		called = true
	}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, "go-test-json.out")),
		Handler: &handlerFails{},
		Stop:    stop,
	}
	_, err := ScanTestOutput(cfg)
	assert.Error(t, err, "something failed")
	assert.Assert(t, called)
}

type handlerFails struct {
	count int
}

func (s *handlerFails) Event(_ TestEvent, _ *Execution) error {
	if s.count > 1 {
		return fmt.Errorf("something failed")
	}
	s.count++
	return nil
}

func (s *handlerFails) Err(_ string) error {
	return nil
}

func TestParseEvent(t *testing.T) {
	// nolint: lll
	raw := `{"Time":"2018-03-22T22:33:35.168308334Z","Action":"output","Package":"example.com/good","Test": "TestOk","Output":"PASS\n"}`
	event, err := parseEvent([]byte(raw))
	assert.NilError(t, err)
	expected := TestEvent{
		Time:    time.Date(2018, 3, 22, 22, 33, 35, 168308334, time.UTC),
		Action:  "output",
		Package: "example.com/good",
		Test:    "TestOk",
		Output:  "PASS\n",
		raw:     []byte(raw),
	}
	cmpTestEvent := cmp.AllowUnexported(TestEvent{})
	assert.DeepEqual(t, event, expected, cmpTestEvent)
}

func TestPackage_AddEvent(t *testing.T) {
	type testCase struct {
		name     string
		event    string
		expected Package
	}

	run := func(t *testing.T, tc testCase) {
		te, err := parseEvent([]byte(tc.event))
		assert.NilError(t, err)

		p := newPackage()
		p.addEvent(te)
		assert.DeepEqual(t, p, &tc.expected, cmpPackage)
	}

	var testCases = []testCase{
		{
			name:  "coverage with -cover",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"coverage: 4.2% of statements\n"}`,
			expected: Package{
				coverage: "coverage: 4.2% of statements",
				output:   pkgOutput(0, "coverage: 4.2% of statements\n"),
			},
		},
		{
			name:  "coverage with -coverpkg",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"coverage: 7.5% of statements in ./testing\n"}`,
			expected: Package{
				coverage: "coverage: 7.5% of statements in ./testing",
				output:   pkgOutput(0, "coverage: 7.5% of statements in ./testing\n"),
			},
		},
		{
			name:     "package failed",
			event:    `{"Action":"fail","Package":"gotest.tools/testing","Elapsed":0.012}`,
			expected: Package{action: ActionFail, elapsed: 12 * time.Millisecond},
		},
		{
			name:  "package is cached",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"ok  \tgotest.tools/testing\t(cached)\n"}`,
			expected: Package{
				cached: true,
				output: pkgOutput(0, "ok  \tgotest.tools/testing\t(cached)\n"),
			},
		},
		{
			name:     "package pass",
			event:    `{"Action":"pass","Package":"gotest.tools/testing","Elapsed":0.012}`,
			expected: Package{action: ActionPass, elapsed: 12 * time.Millisecond},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func pkgOutput(id int, line string) map[int][]string {
	return map[int][]string{id: {line}}
}

func TestScanOutput_WithMissingEvents(t *testing.T) {
	source := golden.Get(t, "go-test-json-missing-test-events.out")
	handler := &captureHandler{}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: handler,
	}
	_, err := ScanTestOutput(cfg)
	assert.NilError(t, err)

	var cmpTestEventShallow = cmp.Options{
		cmp.Comparer(func(x, y TestEvent) bool {
			return x.Test == y.Test && x.Action == y.Action && x.Elapsed == y.Elapsed
		}),
		cmpopts.SortSlices(func(x, y TestEvent) bool {
			return x.Test < y.Test
		}),
	}

	// the package end event should come immediately before the artificial events
	expected := []TestEvent{
		{Action: ActionPass},
		{Action: ActionFail, Test: "TestMissing", Elapsed: -1},
		{Action: ActionFail, Test: "TestMissing/a", Elapsed: -1},
		// we should parse the success from the output despite the missing pass event
		{Action: ActionPass, Test: "TestMissingEvent", Elapsed: 0},
		{Action: ActionFail, Test: "TestFailed/a", Elapsed: -1},
		{Action: ActionFail, Test: "TestFailed/a/sub", Elapsed: -1},
	}
	assert.Assert(t, len(handler.events) > len(expected))
	start := len(handler.events) - len(expected)
	assert.DeepEqual(t, expected, handler.events[start:], cmpTestEventShallow)
}

func TestScanOutput_WithNonJSONLines(t *testing.T) {
	source := golden.Get(t, "go-test-json-with-nonjson-stdout.out")
	nonJSONLine := "|||This line is not valid test2json output.|||"

	// Test that when we ignore non-JSON lines, scanning completes, and test
	// that when we don't ignore non-JSON lines, scanning fails.
	for _, ignore := range []bool{true, false} {
		t.Run(fmt.Sprintf("ignore-non-json=%v", ignore), func(t *testing.T) {
			handler := &captureHandler{}
			cfg := ScanConfig{
				Stdout:                   bytes.NewReader(source),
				Handler:                  handler,
				IgnoreNonJSONOutputLines: ignore,
			}
			_, err := ScanTestOutput(cfg)
			if ignore {
				assert.DeepEqual(t, handler.errs, []string{nonJSONLine})
				assert.NilError(t, err)
				return
			}
			assert.DeepEqual(t, handler.errs, []string{}, cmpopts.EquateEmpty())
			expected := "failed to parse test output: " +
				nonJSONLine + ": invalid character '|' looking for beginning of value"
			assert.Error(t, err, expected)
		})
	}
}

func TestScanOutput_WithNewlineGarbledPass(t *testing.T) {
	source := golden.Get(t, "go-test-json-newline-garbled.out")
	handler := &captureHandler{}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: handler,
	}
	_, err := ScanTestOutput(cfg)
	assert.NilError(t, err)

	expected := TestEvent{Action: ActionPass, Test: "TestPassed", Elapsed: 0.18}
	assert.Assert(t, len(handler.events) > 2) // run, outcome, and some output
	lastEvent := handler.events[len(handler.events)-1]
	assert.Equal(t, expected.Test, lastEvent.Test)
	assert.Equal(t, expected.Action, lastEvent.Action)
	assert.Equal(t, expected.Elapsed, lastEvent.Elapsed)
}

type captureHandler struct {
	events []TestEvent
	errs   []string
}

func (s *captureHandler) Event(event TestEvent, _ *Execution) error {
	s.events = append(s.events, event)
	return nil
}

func (s *captureHandler) Err(text string) error {
	s.errs = append(s.errs, text)
	return fmt.Errorf(text)
}
