package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
)

func TestUsage_WithFlagsFromSetupFlags(t *testing.T) {
	defer env.PatchAll(t, nil)()

	name := "gotestsum"
	flags, _ := setupFlags(name)
	buf := new(bytes.Buffer)
	usage(buf, name, flags)

	golden.Assert(t, buf.String(), "gotestsum-help-text")
}

func TestOptions_Validate_FromFlags(t *testing.T) {
	type testCase struct {
		name     string
		args     []string
		expected string
	}
	fn := func(t *testing.T, tc testCase) {
		flags, opts := setupFlags("gotestsum")
		err := flags.Parse(tc.args)
		assert.NilError(t, err)
		opts.args = flags.Args()

		err = opts.Validate()
		if tc.expected == "" {
			assert.NilError(t, err)
			return
		}
		assert.ErrorContains(t, err, tc.expected, "opts: %#v", opts)
	}
	var testCases = []testCase{
		{
			name: "no flags",
		},
		{
			name: "rerun flag, raw command",
			args: []string{"--rerun-fails", "--raw-command", "--", "./test-all"},
		},
		{
			name: "rerun flag, no go-test args",
			args: []string{"--rerun-fails", "--"},
		},
		{
			name:     "rerun flag, go-test args, no packages flag",
			args:     []string{"--rerun-fails", "--", "./..."},
			expected: "the list of packages to test must be specified by the --packages flag",
		},
		{
			name: "rerun flag, go-test args, with packages flag",
			args: []string{"--rerun-fails", "--packages", "./...", "--", "--foo"},
		},
		{
			name: "rerun flag, no go-test args, with packages flag",
			args: []string{"--rerun-fails", "--packages", "./..."},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestGoTestCmdArgs(t *testing.T) {
	type testCase struct {
		opts      *options
		rerunOpts rerunOpts
		env       []string
		expected  []string
	}
	fn := func(t *testing.T, tc testCase) {
		defer env.PatchAll(t, env.ToMap(tc.env))()
		actual := goTestCmdArgs(tc.opts, tc.rerunOpts)
		assert.DeepEqual(t, actual, tc.expected)
	}
	var testcases = map[string]testCase{
		"raw command": {
			opts: &options{
				rawCommand: true,
				args:       []string{"./script", "-test.timeout=20m"},
			},
			expected: []string{"./script", "-test.timeout=20m"},
		},
		"no args": {
			opts:     &options{},
			expected: []string{"go", "test", "-json", "./..."},
		},
		"no args, with rerunPackageList arg": {
			opts: &options{
				packages: []string{"./pkg"},
			},
			expected: []string{"go", "test", "-json", "./pkg"},
		},
		"TEST_DIRECTORY env var no args": {
			opts:     &options{},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "testdir"},
		},
		"TEST_DIRECTORY env var with args": {
			opts: &options{
				args: []string{"-tags=integration"},
			},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "-tags=integration", "testdir"},
		},
		"no -json arg": {
			opts: &options{
				args: []string{"-timeout=2m", "./pkg"},
			},
			expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
		},
		"with -json arg": {
			opts: &options{
				args: []string{"-json", "-timeout=2m", "./pkg"},
			},
			expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
		},
		"raw command, with rerunOpts": {
			opts: &options{
				rawCommand: true,
				args:       []string{"./script", "-test.timeout=20m"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"./script", "-test.timeout=20m", "-run=TestOne|TestTwo", "./fails"},
		},
		"no args, with rerunOpts": {
			opts: &options{},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
		},
		"TEST_DIRECTORY env var, no args, with rerunOpts": {
			opts: &options{},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			env: []string{"TEST_DIRECTORY=testdir"},
			// TEST_DIRECTORY should be overridden by rerun opts
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
		},
		"TEST_DIRECTORY env var, with args, with rerunOpts": {
			opts: &options{
				args: []string{"-tags=integration"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-tags=integration", "./fails"},
		},
		"no -json arg, with rerunOpts": {
			opts: &options{
				args:     []string{"-timeout=2m"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
		},
		"with -json arg, with rerunOpts": {
			opts: &options{
				args:     []string{"-json", "-timeout=2m"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-run=TestOne|TestTwo", "-json", "-timeout=2m", "./fails"},
		},
		"with args, with reunFailsPackageList args, with rerunOpts": {
			opts: &options{
				args:     []string{"-timeout=2m"},
				packages: []string{"./pkg1", "./pkg2", "./pkg3"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
		},
		"with args, with reunFailsPackageList": {
			opts: &options{
				args:     []string{"-timeout=2m"},
				packages: []string{"./pkg1", "./pkg2", "./pkg3"},
			},
			expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg1", "./pkg2", "./pkg3"},
		},
		"reunFailsPackageList args, with rerunOpts ": {
			opts: &options{
				packages: []string{"./pkg1", "./pkg2", "./pkg3"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
		},
		"reunFailsPackageList args, with rerunOpts, with -args ": {
			opts: &options{
				args:     []string{"before", "-args", "after"},
				packages: []string{"./pkg1"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "before", "./fails", "-args", "after"},
		},
		"reunFailsPackageList args, with rerunOpts, with -args at end": {
			opts: &options{
				args:     []string{"before", "-args"},
				packages: []string{"./pkg1"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "before", "./fails", "-args"},
		},
		"reunFailsPackageList args, with -args at start": {
			opts: &options{
				args:     []string{"-args", "after"},
				packages: []string{"./pkg1"},
			},
			expected: []string{"go", "test", "-json", "./pkg1", "-args", "after"},
		},
		"-run arg at start, with rerunOpts ": {
			opts: &options{
				args:     []string{"-run=TestFoo", "-args"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails", "-args"},
		},
		"-run arg in middle, with rerunOpts ": {
			opts: &options{
				args:     []string{"-count", "1", "--run", "TestFoo", "-args"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-count", "1", "./fails", "-args"},
		},
		"-run arg at end with missing value, with rerunOpts ": {
			opts: &options{
				args:     []string{"-count", "1", "-run"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-count", "1", "-run", "./fails"},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestRun_RerunFails_WithTooManyInitialFailures(t *testing.T) {
	jsonFailed := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "fail"}
{"Package": "pkg", "Test": "TestTwo", "Action": "run"}
{"Package": "pkg", "Test": "TestTwo", "Action": "fail"}
{"Package": "pkg", "Action": "fail"}
`

	fn := func(args []string) proc {
		return proc{
			cmd:    fakeWaiter{result: newExitCode("failed", 1)},
			stdout: strings.NewReader(jsonFailed),
			stderr: bytes.NewReader(nil),
		}
	}
	reset := patchStartGoTestFn(fn)
	defer reset()

	out := new(bytes.Buffer)
	opts := &options{
		rawCommand:                   true,
		args:                         []string{"./test.test"},
		format:                       "testname",
		rerunFailsMaxAttempts:        3,
		rerunFailsMaxInitialFailures: 1,
		stdout:                       out,
		stderr:                       os.Stderr,
		noSummary:                    newNoSummaryValue(),
	}
	err := run(opts)
	assert.ErrorContains(t, err, "number of test failures (2) exceeds maximum (1)", out.String())
}

func TestRun_RerunFails_BuildErrorPreventsRerun(t *testing.T) {
	jsonFailed := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "fail"}
{"Package": "pkg", "Test": "TestTwo", "Action": "run"}
{"Package": "pkg", "Test": "TestTwo", "Action": "fail"}
{"Package": "pkg", "Action": "fail"}
`

	fn := func(args []string) proc {
		return proc{
			cmd:    fakeWaiter{result: newExitCode("failed", 1)},
			stdout: strings.NewReader(jsonFailed),
			stderr: strings.NewReader("anything here is an error\n"),
		}
	}
	reset := patchStartGoTestFn(fn)
	defer reset()

	out := new(bytes.Buffer)
	opts := &options{
		rawCommand:                   true,
		args:                         []string{"./test.test"},
		format:                       "testname",
		rerunFailsMaxAttempts:        3,
		rerunFailsMaxInitialFailures: 1,
		stdout:                       out,
		stderr:                       os.Stderr,
		noSummary:                    newNoSummaryValue(),
	}
	err := run(opts)
	assert.ErrorContains(t, err, "rerun aborted because previous run had errors", out.String())
}

// type checking of os/exec.ExitError is done in a test file so that users
// installing from source can continue to use versions prior to go1.12.
var _ exitCoder = &exec.ExitError{}
