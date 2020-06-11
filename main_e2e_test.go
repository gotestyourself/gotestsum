package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/gotestsum/internal/text"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestE2E_RerunFails(t *testing.T) {
	type testCase struct {
		name        string
		args        []string
		expectedErr bool
	}
	fn := func(t *testing.T, tc testCase) {
		tmpFile := fs.NewFile(t, t.Name()+"-seedfile", fs.WithContent("0"))
		defer tmpFile.Remove()

		envVars := osEnviron()
		envVars["TEST_SEEDFILE"] = tmpFile.Path()
		defer env.PatchAll(t, envVars)()

		flags, opts := setupFlags("gotestsum")
		assert.NilError(t, flags.Parse(tc.args))
		opts.args = flags.Args()

		bufStdout := new(bytes.Buffer)
		opts.stdout = bufStdout
		bufStderr := new(bytes.Buffer)
		opts.stderr = bufStderr

		err := run(opts)
		if tc.expectedErr {
			assert.Error(t, err, "exit status 1")
		} else {
			assert.NilError(t, err)
		}
		out := text.ProcessLines(t, bufStdout,
			text.OpRemoveSummaryLineElapsedTime,
			text.OpRemoveTestElapsedTime)
		golden.Assert(t, out, expectedFilename(t.Name()))
		assert.Equal(t, bufStderr.String(), "")
	}
	var testCases = []testCase{
		{
			name: "reruns until success",
			args: []string{
				"-f=testname",
				"--rerun-fails=4",
				"--packages=./testdata/e2e/flaky/",
				"--", "-count=1", "-tags=testdata",
			},
		},
		{
			name: "reruns continues to fail",
			args: []string{
				"-f=testname",
				"--rerun-fails=2",
				"--packages=./testdata/e2e/flaky/",
				"--", "-count=1", "-tags=testdata",
			},
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("too slow for short run")
			}
			fn(t, tc)
		})
	}
}

// osEnviron returns os.Environ() as a map, with any GOTESTSUM_ env vars removed
// so that they do not alter the test results.
func osEnviron() map[string]string {
	e := env.ToMap(os.Environ())
	for k := range e {
		if strings.HasPrefix(k, "GOTESTSUM_") {
			delete(e, k)
		}
	}
	return e
}

func expectedFilename(name string) string {
	// go1.14 changed how it prints messages from tests. It may be changed back
	// in go1.15, so special case this version for now.
	if strings.HasPrefix(runtime.Version(), "go1.14.") {
		name = name + "-go1.14"
	}
	return "e2e/expected/" + name
}
