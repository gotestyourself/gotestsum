package testjson

import (
	"bytes"
	"fmt"
	"strings"
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
	in := bytes.NewReader(golden.Get(t, "input/go-test-json.out"))
	exec, err := ScanTestOutput(ScanConfig{Stdout: in})
	assert.NilError(t, err)
	// a weak check to show that all the stdout was scanned
	assert.Equal(t, exec.Total(), 59)
}

func TestScanTestOutput_CallsStopOnError(t *testing.T) {
	var called bool
	stop := func() {
		called = true
	}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, "input/go-test-json.out")),
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
	//nolint:lll
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

func TestScanTestOutput_WithMissingEvents(t *testing.T) {
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
		{Action: ActionFail, Test: "TestMissingEvent", Elapsed: -1},
		{Action: ActionFail, Test: "TestFailed/a", Elapsed: -1},
		{Action: ActionFail, Test: "TestFailed/a/sub", Elapsed: -1},
	}
	assert.Assert(t, len(handler.events) > len(expected))
	start := len(handler.events) - len(expected)
	assert.DeepEqual(t, expected, handler.events[start:], cmpTestEventShallow)
}

func TestScanTestOutput_WithNonJSONLines(t *testing.T) {
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

func TestScanTestOutput_WithGODEBUG(t *testing.T) {
	goDebugSource := `HASH[moduleIndex]
HASH[moduleIndex]: "go1.20.4"
HASH /usr/lib/go/src/runtime/debuglog_off.go: d6f147198
testcache: package: input list not found: ...`

	handler := &captureHandler{}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(nil),
		Stderr:  strings.NewReader(goDebugSource),
		Handler: handler,
	}
	exec, err := ScanTestOutput(cfg)
	assert.NilError(t, err)
	assert.DeepEqual(t, handler.errs, strings.Split(goDebugSource, "\n"))
	assert.DeepEqual(t, exec.Errors(), []string(nil))
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
	return nil
}

func TestFilterFailedUnique_MultipleNested(t *testing.T) {
	source := []byte(`{"Package": "pkg", "Action": "run"}
	{"Package": "pkg", "Test": "TestParent", "Action": "run"}
	{"Package": "pkg", "Test": "TestParent/TestNested", "Action": "run"}
	{"Package": "pkg", "Test": "TestParent/TestNested/TestOne", "Action": "run"}
	{"Package": "pkg", "Test": "TestParent/TestNested/TestOne", "Action": "fail"}
	{"Package": "pkg", "Test": "TestParent/TestNested/TestOnePrefix", "Action": "run"}
	{"Package": "pkg", "Test": "TestParent/TestNested/TestOnePrefix", "Action": "fail"}
	{"Package": "pkg", "Test": "TestParent/TestNested", "Action": "fail"}
	{"Package": "pkg", "Test": "TestParent", "Action": "fail"}
	{"Package": "pkg", "Test": "TestTop", "Action": "run"}
	{"Package": "pkg", "Test": "TestTop", "Action": "fail"}
	{"Package": "pkg", "Test": "TestTopPrefix", "Action": "run"}
	{"Package": "pkg", "Test": "TestTopPrefix", "Action": "fail"}
	{"Package": "pkg", "Action": "fail"}
	{"Package": "pkg2", "Action": "run"}
	{"Package": "pkg2", "Test": "TestParent", "Action": "run"}
	{"Package": "pkg2", "Test": "TestParent/TestNested", "Action": "run"}
	{"Package": "pkg2", "Test": "TestParent/TestNested", "Action": "fail"}
	{"Package": "pkg2", "Test": "TestParent/TestNestedPrefix", "Action": "run"}
	{"Package": "pkg2", "Test": "TestParent/TestNestedPrefix", "Action": "fail"}
	{"Package": "pkg2", "Test": "TestParent", "Action": "fail"}
	{"Package": "pkg2", "Test": "TestParentPrefix", "Action": "run"}
	{"Package": "pkg2", "Test": "TestParentPrefix", "Action": "fail"}
	{"Package": "pkg2", "Action": "fail"}`)

	handler := &captureHandler{}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(source),
		Handler: handler,
	}
	exec, err := ScanTestOutput(cfg)
	assert.NilError(t, err)
	actual := FilterFailedUnique(exec.Failed())

	expected := []TestCase{
		{ID: 3, Package: "pkg", Test: TestName("TestParent/TestNested/TestOne")},
		{ID: 4, Package: "pkg", Test: TestName("TestParent/TestNested/TestOnePrefix")},
		{ID: 5, Package: "pkg", Test: TestName("TestTop")},
		{ID: 6, Package: "pkg", Test: TestName("TestTopPrefix")},
		{ID: 2, Package: "pkg2", Test: TestName("TestParent/TestNested")},
		{ID: 3, Package: "pkg2", Test: TestName("TestParent/TestNestedPrefix")},
		{ID: 4, Package: "pkg2", Test: TestName("TestParentPrefix")},
	}
	cmpTestCase := cmp.AllowUnexported(TestCase{})
	assert.DeepEqual(t, expected, actual, cmpTestCase)
}

func TestFilterFailedUnique_NestedWithGaps(t *testing.T) {
	input := []TestCase{
		{ID: 1, Package: "pkg", Test: "TestParent/foo/bar/baz"},
		{ID: 2, Package: "pkg", Test: "TestParent"},
		{ID: 3, Package: "pkg", Test: "TestParent1/foo/bar"},
		{ID: 4, Package: "pkg", Test: "TestParent1"},
	}
	actual := FilterFailedUnique(input)

	expected := []TestCase{
		{ID: 1, Package: "pkg", Test: "TestParent/foo/bar/baz"},
		{ID: 3, Package: "pkg", Test: "TestParent1/foo/bar"},
	}
	cmpTestCase := cmp.AllowUnexported(TestCase{})
	assert.DeepEqual(t, expected, actual, cmpTestCase)
}

func TestScanTestOutput_PausedTestsNotMarkedFailed(t *testing.T) {
	const pkgName = "pkg"

	t.Run("paused top-level test at package timeout", func(t *testing.T) {
		in := strings.NewReader(`{"Action":"start","Package":"pkg"}
{"Action":"run","Package":"pkg","Test":"TestPassed"}
{"Action":"output","Package":"pkg","Test":"TestPassed","Output":"=== RUN   TestPassed\n"}
{"Action":"output","Package":"pkg","Test":"TestPassed","Output":"=== PAUSE TestPassed\n"}
{"Action":"pause","Package":"pkg","Test":"TestPassed"}
{"Action":"run","Package":"pkg","Test":"TestRunning"}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"=== RUN   TestRunning\n"}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"=== PAUSE TestRunning\n"}
{"Action":"pause","Package":"pkg","Test":"TestRunning"}
{"Action":"run","Package":"pkg","Test":"TestPaused"}
{"Action":"output","Package":"pkg","Test":"TestPaused","Output":"=== RUN   TestPaused\n"}
{"Action":"output","Package":"pkg","Test":"TestPaused","Output":"=== PAUSE TestPaused\n"}
{"Action":"pause","Package":"pkg","Test":"TestPaused"}
{"Action":"output","Package":"pkg","Test":"TestPassed","Output":"=== CONT  TestPassed\n"}
{"Action":"cont","Package":"pkg","Test":"TestPassed"}
{"Action":"output","Package":"pkg","Test":"TestPassed","Output":"--- PASS: TestPassed (0.01s)\n"}
{"Action":"pass","Package":"pkg","Test":"TestPassed","Elapsed":0.01}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"=== CONT  TestRunning\n"}
{"Action":"cont","Package":"pkg","Test":"TestRunning"}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"panic: test timed out after 1s\n"}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"\trunning tests:\n"}
{"Action":"output","Package":"pkg","Test":"TestRunning","Output":"\t\tTestRunning (1s)\n"}
{"Action":"output","Package":"pkg","Output":"FAIL\tpkg\t1.002s\n"}
{"Action":"fail","Package":"pkg","Elapsed":1.002}
`)
		handler := &captureHandler{}
		exec, err := ScanTestOutput(ScanConfig{Stdout: in, Handler: handler})
		assert.NilError(t, err)

		pkg := exec.Package(pkgName)
		assert.Equal(t, pkg.Total, 3)
		assert.Assert(t, pkg.TestMainFailed())
		assert.Equal(t, len(pkg.running), 0)
		assert.DeepEqual(t, testCaseNames(pkg.Passed), []string{"TestPassed"})
		assert.DeepEqual(t, testCaseNames(pkg.Failed), []string{"TestRunning"})
		assert.Equal(t, pkg.Failed[0].Elapsed, neverFinished)

		lastEvent := handler.events[len(handler.events)-1]
		assert.Equal(t, lastEvent.Action, ActionFail)
		assert.Equal(t, lastEvent.Test, "TestRunning")
		assert.Equal(t, lastEvent.Elapsed, float64(neverFinished))
	})

	t.Run("paused parallel subtest at package timeout", func(t *testing.T) {
		in := strings.NewReader(`{"Action":"start","Package":"pkg"}
{"Action":"run","Package":"pkg","Test":"TestSubGroup"}
{"Action":"output","Package":"pkg","Test":"TestSubGroup","Output":"=== RUN   TestSubGroup\n"}
{"Action":"run","Package":"pkg","Test":"TestSubGroup/SubRunning"}
{"Action":"output","Package":"pkg","Test":"TestSubGroup/SubRunning","Output":"=== RUN   TestSubGroup/SubRunning\n"}
{"Action":"pause","Package":"pkg","Test":"TestSubGroup/SubRunning"}
{"Action":"run","Package":"pkg","Test":"TestSubGroup/SubPaused"}
{"Action":"output","Package":"pkg","Test":"TestSubGroup/SubPaused","Output":"=== RUN   TestSubGroup/SubPaused\n"}
{"Action":"pause","Package":"pkg","Test":"TestSubGroup/SubPaused"}
{"Action":"cont","Package":"pkg","Test":"TestSubGroup/SubRunning"}
{"Action":"output","Package":"pkg","Test":"TestSubGroup/SubRunning","Output":"panic: test timed out after 1s\n"}
{"Action":"fail","Package":"pkg","Elapsed":1.002}
`)
		exec, err := ScanTestOutput(ScanConfig{Stdout: in})
		assert.NilError(t, err)

		pkg := exec.Package(pkgName)
		failed := testCaseNames(pkg.Failed)
		cmpSort := cmpopts.SortSlices(func(a, b string) bool { return a < b })
		assert.DeepEqual(t, failed, []string{"TestSubGroup", "TestSubGroup/SubRunning"}, cmpSort)
		assert.Equal(t, len(pkg.running), 0)
	})

	t.Run("paused test cleared before reusing Execution", func(t *testing.T) {
		run1 := strings.NewReader(`{"Action":"run","Package":"pkg","Test":"TestFailed"}
{"Action":"fail","Package":"pkg","Test":"TestFailed","Elapsed":0.01}
{"Action":"run","Package":"pkg","Test":"TestPaused"}
{"Action":"pause","Package":"pkg","Test":"TestPaused"}
{"Action":"fail","Package":"pkg","Elapsed":0.02}
`)
		exec, err := ScanTestOutput(ScanConfig{Stdout: run1, RunID: 0})
		assert.NilError(t, err)
		assert.DeepEqual(t, testCaseNames(exec.Package(pkgName).Failed), []string{"TestFailed"})

		// Simulate a rerun where TestPaused emits an event without ActionRun; it must
		// not inherit the stale paused flag from RunID 0.
		run2 := strings.NewReader(`{"Action":"output","Package":"pkg","Test":"TestPaused","Output":"boom\n"}
{"Action":"fail","Package":"pkg","Elapsed":0.01}
`)
		exec, err = ScanTestOutput(ScanConfig{Stdout: run2, Execution: exec, RunID: 1})
		assert.NilError(t, err)
		assert.DeepEqual(t, testCaseNames(exec.Package(pkgName).Failed), []string{"TestFailed", "TestPaused"})
	})
}

func testCaseNames(tcs []TestCase) []string {
	names := make([]string, 0, len(tcs))
	for _, tc := range tcs {
		names = append(names, tc.Test.Name())
	}
	return names
}
