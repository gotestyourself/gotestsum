package testjson

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

// See format_test.go for a more detailed description.
//
// This test uses the deep tag:
//
//	go test -p=1 -parallel=1 -json -tags="stubpkg deep" ./testjson/internal/... \
//	  > testjson/testdata/input/go-test-json-deep.out \
//	  2>testjson/testdata/input/go-test-json-deep.err

func TestFormats_Deep(t *testing.T) {
	type testCase struct {
		name        string
		format      func(io.Writer) EventFormatter
		expectedOut string
		expected    func(t *testing.T, exec *Execution)
	}
	const ESC = 27
	// remove clear-line terminal codes to keep golden files escape-code free
	var clearRe = regexp.MustCompile(fmt.Sprintf("%c\\[[%d%d]A%c\\[2K\r?", ESC, 0, 1, ESC))

	run := func(t *testing.T, tc testCase) {
		out := new(bytes.Buffer)
		shim := newFakeHandler(tc.format(out), "input/go-test-json-deep")
		exec, err := ScanTestOutput(shim.Config(t))
		assert.NilError(t, err)
		// pkgNameCompactFormat -plain never line terminates
		out.WriteString("\n")
		out = bytes.NewBufferString(clearRe.ReplaceAllString(out.String(), ""))

		golden.Assert(t, out.String(), tc.expectedOut)
		golden.Assert(t, shim.err.String(), "input/go-test-json-deep.err")

		if tc.expected != nil {
			tc.expected(t, exec)
		}
	}

	compactFormat := func(fmt string) func(io.Writer) EventFormatter {
		return func(out io.Writer) EventFormatter {
			return pkgNameCompactFormat(out, FormatOptions{CompactPkgNameFormat: fmt})
		}
	}
	compactOpts := func(fmt string, base FormatOptions) func(io.Writer) EventFormatter {
		return func(out io.Writer) EventFormatter {
			opts := base
			opts.CompactPkgNameFormat = fmt
			return pkgNameCompactFormat(out, opts)
		}
	}

	testCases := []testCase{
		{
			name:        "pkgname-compact plain",
			format:      compactFormat("plain"),
			expectedOut: "format/pkgname-compact-plain.out",
		},
		{
			name:        "pkgname-compact non-plain",
			format:      compactFormat("relative"),
			expectedOut: "format/pkgname-compact-dotwriter.out",
		},
		{
			name:        "pkgname-compact short",
			format:      compactFormat("short-plain"),
			expectedOut: "format/pkgname-compact-short.out",
		},
		{
			name:        "pkgname-compact partial",
			format:      compactFormat("partial-plain"),
			expectedOut: "format/pkgname-compact-partial.out",
		},
		{
			name:        "pkgname-compact partial-back",
			format:      compactFormat("partial-back-plain"),
			expectedOut: "format/pkgname-compact-partial-back.out",
		},
		{
			name:        "pkgname-compact short dots",
			format:      compactFormat("short-dots-plain"),
			expectedOut: "format/pkgname-compact-short-dots.out",
		},
		{
			name:        "pkgname-compact hivis plain",
			format:      compactOpts("plain", FormatOptions{UseHiVisibilityIcons: true}),
			expectedOut: "format/pkgname-compact-hivis-plain.out",
		},
		{
			name:        "pkgname-compact hivis partial dots7",
			format:      compactOpts("partial-dots7-plain", FormatOptions{UseHiVisibilityIcons: true}),
			expectedOut: "format/pkgname-compact-hivis-partial-dots7.out",
		},
		{
			name:        "pkgname-compact wall plain",
			format:      compactOpts("plain", FormatOptions{OutputWallTime: true}),
			expectedOut: "format/pkgname-compact-wall-plain.out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(t.Name(), "wall") {
				patchTimeNowCounting(t, 337*time.Millisecond)
			} else if strings.Contains(t.Name(), "non-plain") {
				patchTimeNowCounting(t, 1*time.Millisecond)
			}
			run(t, tc)
		})
	}
}

func patchTimeNowCounting(t *testing.T, step time.Duration) {
	var dt time.Duration
	timeNow = func() time.Time {
		defer func() { dt += step }()
		return time.Date(2022, 1, 2, 3, 4, 5, 600, time.UTC).Add(dt)
	}
	t.Cleanup(func() {
		timeNow = time.Now
	})
}

func Test_shouldJoinPkgs(t *testing.T) {
	tests := []struct {
		name       string
		fmt        string
		lastPkg    string
		pkg        string
		wantJoin   bool
		wantShort  string
		wantBackUp int
	}{
		{
			name:      "relative",
			fmt:       "relative",
			lastPkg:   "pkg/sub/foo",
			pkg:       "pkg/sub/bar",
			wantJoin:  true,
			wantShort: "pkg/sub/bar",
		},
		{
			name:      "short",
			fmt:       "short",
			lastPkg:   "pkg/sub/foo",
			pkg:       "pkg/sub/bar",
			wantJoin:  true,
			wantShort: "bar",
		},
		{
			name:      "partial-sibling",
			fmt:       "partial",
			lastPkg:   "pkg/sub/foo",
			pkg:       "pkg/sub/bar",
			wantJoin:  true,
			wantShort: "bar",
		},
		{
			name:       "partial-one-up",
			fmt:        "partial",
			lastPkg:    "pkg/sub/foo",
			pkg:        "pkg/sub2/bar",
			wantJoin:   true,
			wantShort:  "sub2/bar",
			wantBackUp: 1,
		},
		{
			name:      "partial-toplevel",
			fmt:       "partial",
			lastPkg:   "pkg/sub/foo",
			pkg:       "pkg2/sub2/bar",
			wantJoin:  true,
			wantShort: "pkg2/sub2/bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts FormatOptions
			opts.CompactPkgNameFormat = tt.fmt
			gotJoin, gotCommonPrefix, gotBackUp := compactPkgPath(opts, tt.lastPkg, tt.pkg)
			assert.Equal(t, gotJoin, tt.wantJoin)
			assert.Equal(t, gotCommonPrefix, tt.wantShort)
			assert.Equal(t, gotBackUp, tt.wantBackUp)
		})
	}
}
