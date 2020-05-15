package slowest

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
)

func TestAggregateTestCases(t *testing.T) {
	cases := []testjson.TestCase{
		{Test: "TestOne", Package: "pkg", Elapsed: time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 2 * time.Second},
		{Test: "TestOne", Package: "pkg", Elapsed: 3 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 4 * time.Second},
		{Test: "TestOne", Package: "pkg", Elapsed: 5 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 6 * time.Second},
	}
	actual := aggregateTestCases(cases)
	expected := []testjson.TestCase{
		{Test: "TestOne", Package: "pkg", Elapsed: 3 * time.Second},
		{Test: "TestTwo", Package: "pkg", Elapsed: 4 * time.Second},
	}
	assert.DeepEqual(t, actual, expected,
		cmpopts.SortSlices(func(x, y testjson.TestCase) bool {
			return strings.Compare(x.Test, y.Test) == -1
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
