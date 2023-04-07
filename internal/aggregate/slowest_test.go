package aggregate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
)

func TestSlowest(t *testing.T) {
	newEvent := func(pkg, test string, elapsed float64) testjson.TestEvent {
		return testjson.TestEvent{
			Package: pkg,
			Test:    test,
			Action:  testjson.ActionPass,
			Elapsed: elapsed,
		}
	}

	exec := newExecutionFromEvents(t,
		newEvent("one", "TestOmega", 22.2),
		newEvent("one", "TestOmega", 1.5),
		newEvent("one", "TestOmega", 0.6),
		newEvent("one", "TestOnion", 0.5),
		newEvent("two", "TestTents", 2.5),
		newEvent("two", "TestTin", 0.3),
		newEvent("two", "TestTunnel", 1.1))

	cmpCasesShallow := cmp.Comparer(func(x, y testjson.TestCase) bool {
		return x.Package == y.Package && x.Test == y.Test
	})

	type testCase struct {
		name      string
		threshold time.Duration
		num       int
		expected  []testjson.TestCase
	}

	run := func(t *testing.T, tc testCase) {
		actual := Slowest(exec, tc.threshold, tc.num)
		assert.DeepEqual(t, actual, tc.expected, cmpCasesShallow)
	}

	testCases := []testCase{
		{
			name:      "threshold only",
			threshold: time.Second,
			expected: []testjson.TestCase{
				{Package: "two", Test: "TestTents"},
				{Package: "one", Test: "TestOmega"},
				{Package: "two", Test: "TestTunnel"},
			},
		},
		{
			name:      "threshold only 2s",
			threshold: 2 * time.Second,
			expected: []testjson.TestCase{
				{Package: "two", Test: "TestTents"},
			},
		},
		{
			name:      "threshold and num",
			threshold: 400 * time.Millisecond,
			num:       2,
			expected: []testjson.TestCase{
				{Package: "two", Test: "TestTents"},
				{Package: "one", Test: "TestOmega"},
			},
		},
		{
			name: "num only",
			num:  4,
			expected: []testjson.TestCase{
				{Package: "two", Test: "TestTents"},
				{Package: "one", Test: "TestOmega"},
				{Package: "two", Test: "TestTunnel"},
				{Package: "one", Test: "TestOnion"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func newExecutionFromEvents(t *testing.T, events ...testjson.TestEvent) *testjson.Execution {
	t.Helper()

	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	for i, event := range events {
		assert.NilError(t, encoder.Encode(event), "event %d", i)
	}

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: buf,
		Stderr: strings.NewReader(""),
	})
	assert.NilError(t, err)
	return exec
}

func TestByElapsed_WithMedian(t *testing.T) {
	cases := []testjson.TestCase{
		{Test: "TestOne", Package: "pkg", Elapsed: time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 2 * time.Second},
		{Test: "TestOne", Package: "pkg", Elapsed: 3 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 4 * time.Second},
		{Test: "TestOne", Package: "pkg", Elapsed: 5 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 6 * time.Second},
	}
	actual := ByElapsed(cases, median)
	expected := []testjson.TestCase{
		{Test: "TestOne", Package: "pkg", Elapsed: 3 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 4 * time.Second},
	}
	assert.DeepEqual(t, actual, expected,
		cmpopts.SortSlices(func(x, y testjson.TestCase) bool {
			return strings.Compare(x.Test.Name(), y.Test.Name()) == -1
		}),
		cmpopts.IgnoreUnexported(testjson.TestCase{}))
}

func TestMedian(t *testing.T) {
	var testcases = []struct {
		name     string
		times    []time.Duration
		expected time.Duration
	}{
		{
			name:     "one item slice",
			times:    []time.Duration{time.Minute},
			expected: time.Minute,
		},
		{
			name:     "odd number of items",
			times:    []time.Duration{time.Millisecond, time.Hour, time.Second},
			expected: time.Second,
		},
		{
			name:     "even number of items",
			times:    []time.Duration{time.Second, time.Millisecond, time.Microsecond, time.Hour},
			expected: time.Second,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actual := median(tc.times)
			assert.Equal(t, actual, tc.expected)
		})
	}
}
