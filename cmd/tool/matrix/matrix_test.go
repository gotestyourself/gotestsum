package matrix

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func TestPackagePercentile(t *testing.T) {
	ms := time.Millisecond
	timing := map[string][]time.Duration{
		"none":  {},
		"one":   {time.Second},
		"two":   {4 * ms, 2 * ms},
		"three": {2 * ms, 3 * ms, 5 * ms},
		"four":  {4 * ms, 3 * ms, ms, 2 * ms},
		"five":  {6 * ms, 2 * ms, 3 * ms, 4 * ms, 9 * ms},
		"nine":  {6 * ms, 2 * ms, 3 * ms, 4 * ms, 9 * ms, 1 * ms, 5 * ms, 7 * ms, 8 * ms},
		"ten":   {6 * ms, 2 * ms, 3 * ms, 4 * ms, 9 * ms, 5 * ms, 7 * ms, 8 * ms, ms, ms},
		"twenty": {
			6 * ms, 2 * ms, 3 * ms, 4 * ms, 9 * ms, 5 * ms, 7 * ms, 8 * ms, ms, ms,
			100, 200, 600, 700, 800, 900, 200, 300, 400, 500,
		},
	}

	out := packagePercentile(timing)
	expected := map[string]time.Duration{
		"none":   0,
		"one":    time.Second,
		"two":    4 * ms,
		"three":  5 * ms,
		"four":   4 * ms,
		"five":   9 * ms,
		"nine":   8 * ms,
		"ten":    8 * ms,
		"twenty": 6 * ms,
	}
	assert.DeepEqual(t, out, expected)
}

func TestBucketPackages(t *testing.T) {
	ms := time.Millisecond
	timing := map[string]time.Duration{
		"one":   190 * ms,
		"two":   200 * ms,
		"three": 3800 * ms,
		"four":  4000 * ms,
		"five":  50 * ms,
		"six":   606 * ms,
		"rm1":   time.Second,
		"rm2":   time.Second,
	}
	packages := []string{"new1", "new2", "one", "two", "three", "four", "five", "six"}

	type testCase struct {
		n        uint
		expected []bucket
	}

	run := func(t *testing.T, tc testCase) {
		buckets := bucketPackages(timing, packages, tc.n)
		assert.DeepEqual(t, buckets, tc.expected)
	}

	testCases := []testCase{
		{
			n: 2,
			expected: []bucket{
				0: {Total: 4440 * ms, Packages: []string{"four", "two", "one", "five"}},
				1: {Total: 4406 * ms, Packages: []string{"three", "six", "new2", "new1"}},
			},
		},
		{
			n: 3,
			expected: []bucket{
				0: {Total: 4000 * ms, Packages: []string{"four"}},
				1: {Total: 3800 * ms, Packages: []string{"three"}},
				2: {Total: 1046 * ms, Packages: []string{"six", "two", "one", "five", "new1", "new2"}},
			},
		},
		{
			n: 4,
			expected: []bucket{
				0: {Total: 4000 * ms, Packages: []string{"four"}},
				1: {Total: 3800 * ms, Packages: []string{"three"}},
				2: {Total: 606 * ms, Packages: []string{"six"}},
				3: {Total: 440 * ms, Packages: []string{"two", "one", "five", "new2", "new1"}},
			},
		},
		{
			n: 8,
			expected: []bucket{
				0: {Total: 4000 * ms, Packages: []string{"four"}},
				1: {Total: 3800 * ms, Packages: []string{"three"}},
				2: {Total: 606 * ms, Packages: []string{"six"}},
				3: {Total: 200 * ms, Packages: []string{"two"}},
				4: {Total: 190 * ms, Packages: []string{"one"}},
				5: {Total: 50 * ms, Packages: []string{"five"}},
				6: {Packages: []string{"new1"}},
				7: {Packages: []string{"new2"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(strconv.FormatUint(uint64(tc.n), 10), func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestReadAndPruneTimingReports(t *testing.T) {
	events := func(t *testing.T, start time.Time) string {
		t.Helper()
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		for _, i := range []int{0, 1, 2} {
			assert.NilError(t, encoder.Encode(testjson.TestEvent{
				Time:    start.Add(time.Duration(i) * time.Second),
				Action:  testjson.ActionRun,
				Package: "pkg" + strconv.Itoa(i),
			}))
			buf.WriteString("\n")
		}
		return buf.String()
	}

	now := time.Now()
	dir := fs.NewDir(t, "timing-files",
		fs.WithFile("report1.log", events(t, now.Add(-time.Hour))),
		fs.WithFile("report2.log", events(t, now.Add(-47*time.Hour))),
		fs.WithFile("report3.log", events(t, now.Add(-49*time.Hour))),
		fs.WithFile("report4.log", events(t, now.Add(-101*time.Hour))))

	t.Run("no prune", func(t *testing.T) {
		opts := options{
			timingFilesPattern: dir.Join("*.log"),
		}

		files, err := readAndPruneTimingReports(opts)
		assert.NilError(t, err)
		defer closeFiles(files)
		assert.Equal(t, len(files), 4)

		for _, fh := range files {
			// check the files are properly seeked to 0
			event, err := parseEvent(fh)
			assert.NilError(t, err)
			assert.Equal(t, event.Package, "pkg0")
		}

		actual, err := os.ReadDir(dir.Path())
		assert.NilError(t, err)
		assert.Equal(t, len(actual), 4)
	})

	t.Run("no glob match, func", func(t *testing.T) {
		opts := options{
			timingFilesPattern: dir.Join("*.json"),
		}

		files, err := readAndPruneTimingReports(opts)
		assert.NilError(t, err)
		assert.Equal(t, len(files), 0)
	})

	t.Run("prune older than max age", func(t *testing.T) {
		opts := options{
			timingFilesPattern:   dir.Join("*.log"),
			pruneFilesMaxAgeDays: 2,
		}

		files, err := readAndPruneTimingReports(opts)
		assert.NilError(t, err)
		defer closeFiles(files)
		assert.Equal(t, len(files), 2)

		for _, fh := range files {
			// check the files are properly seeked to 0
			event, err := parseEvent(fh)
			assert.NilError(t, err)
			assert.Equal(t, event.Package, "pkg0")
		}

		actual, err := os.ReadDir(dir.Path())
		assert.NilError(t, err)
		assert.Equal(t, len(actual), 2)
	})
}
