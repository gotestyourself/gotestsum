package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fatih/color"
	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/env"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/skip"
)

func TestUsage_WithFlagsFromSetupFlags(t *testing.T) {
	env.PatchAll(t, nil)
	patchNoColor(t, false)

	name := "gotestsum"
	flags, _ := setupFlags(name)
	buf := new(bytes.Buffer)
	usage(buf, name, flags)

	golden.Assert(t, buf.String(), "gotestsum-help-text")
}

func patchNoColor(t *testing.T, value bool) {
	orig := color.NoColor
	color.NoColor = value
	t.Cleanup(func() {
		color.NoColor = orig
	})
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
		{
			name:     "rerun-fails with failfast",
			args:     []string{"--rerun-fails", "--packages=./...", "--", "-failfast"},
			expected: "-(test.)failfast can not be used with --rerun-fails",
		},
		{
			name:     "rerun-fails with failfast",
			args:     []string{"--rerun-fails", "--packages=./...", "--", "-test.failfast"},
			expected: "-(test.)failfast can not be used with --rerun-fails",
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

	run := func(t *testing.T, name string, tc testCase) {
		t.Helper()
		runCase(t, name, func(t *testing.T) {
			env.PatchAll(t, env.ToMap(tc.env))
			actual := goTestCmdArgs(tc.opts, tc.rerunOpts)
			assert.DeepEqual(t, actual, tc.expected)
		})
	}

	run(t, "raw command", testCase{
		opts: &options{
			rawCommand: true,
			args:       []string{"./script", "-test.timeout=20m"},
		},
		expected: []string{"./script", "-test.timeout=20m"},
	})
	run(t, "no args", testCase{
		opts:     &options{},
		expected: []string{"go", "test", "-json", "./..."},
	})
	run(t, "no args, with rerunPackageList arg", testCase{
		opts: &options{
			packages: []string{"./pkg"},
		},
		expected: []string{"go", "test", "-json", "./pkg"},
	})
	run(t, "TEST_DIRECTORY env var no args", testCase{
		opts:     &options{},
		env:      []string{"TEST_DIRECTORY=testdir"},
		expected: []string{"go", "test", "-json", "testdir"},
	})
	run(t, "TEST_DIRECTORY env var with args", testCase{
		opts: &options{
			args: []string{"-tags=integration"},
		},
		env:      []string{"TEST_DIRECTORY=testdir"},
		expected: []string{"go", "test", "-json", "-tags=integration", "testdir"},
	})
	run(t, "no -json arg", testCase{
		opts: &options{
			args: []string{"-timeout=2m", "./pkg"},
		},
		expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
	})
	run(t, "with -json arg", testCase{
		opts: &options{
			args: []string{"-json", "-timeout=2m", "./pkg"},
		},
		expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
	})
	run(t, "raw command, with rerunOpts", testCase{
		opts: &options{
			rawCommand: true,
			args:       []string{"./script", "-test.timeout=20m"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"./script", "-test.timeout=20m", "-run=TestOne|TestTwo", "./fails"},
	})
	run(t, "no args, with rerunOpts", testCase{
		opts: &options{},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
	})
	run(t, "TEST_DIRECTORY env var, no args, with rerunOpts", testCase{
		opts: &options{},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		env: []string{"TEST_DIRECTORY=testdir"},
		// TEST_DIRECTORY should be overridden by rerun opts
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
	})
	run(t, "TEST_DIRECTORY env var, with args, with rerunOpts", testCase{
		opts: &options{
			args: []string{"-tags=integration"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		env:      []string{"TEST_DIRECTORY=testdir"},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-tags=integration", "./fails"},
	})
	run(t, "no -json arg, with rerunOpts", testCase{
		opts: &options{
			args:     []string{"-timeout=2m"},
			packages: []string{"./pkg"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
	})
	run(t, "with -json arg, with rerunOpts", testCase{
		opts: &options{
			args:     []string{"-json", "-timeout=2m"},
			packages: []string{"./pkg"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-run=TestOne|TestTwo", "-json", "-timeout=2m", "./fails"},
	})
	run(t, "with args, with reunFailsPackageList args, with rerunOpts", testCase{
		opts: &options{
			args:     []string{"-timeout=2m"},
			packages: []string{"./pkg1", "./pkg2", "./pkg3"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
	})
	run(t, "with args, with reunFailsPackageList", testCase{
		opts: &options{
			args:     []string{"-timeout=2m"},
			packages: []string{"./pkg1", "./pkg2", "./pkg3"},
		},
		expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg1", "./pkg2", "./pkg3"},
	})
	run(t, "reunFailsPackageList args, with rerunOpts ", testCase{
		opts: &options{
			packages: []string{"./pkg1", "./pkg2", "./pkg3"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
	})
	run(t, "reunFailsPackageList args, with rerunOpts, with -args ", testCase{
		opts: &options{
			args:     []string{"before", "-args", "after"},
			packages: []string{"./pkg1"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "before", "./fails", "-args", "after"},
	})
	run(t, "reunFailsPackageList args, with rerunOpts, with -args at end", testCase{
		opts: &options{
			args:     []string{"before", "-args"},
			packages: []string{"./pkg1"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "before", "./fails", "-args"},
	})
	run(t, "reunFailsPackageList args, with -args at start", testCase{
		opts: &options{
			args:     []string{"-args", "after"},
			packages: []string{"./pkg1"},
		},
		expected: []string{"go", "test", "-json", "./pkg1", "-args", "after"},
	})
	run(t, "-run arg at start, with rerunOpts ", testCase{
		opts: &options{
			args:     []string{"-run=TestFoo", "-args"},
			packages: []string{"./pkg"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails", "-args"},
	})
	run(t, "-run arg in middle, with rerunOpts ", testCase{
		opts: &options{
			args:     []string{"-count", "1", "--run", "TestFoo", "-args"},
			packages: []string{"./pkg"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-count", "1", "./fails", "-args"},
	})
	run(t, "-run arg at end with missing value, with rerunOpts ", testCase{
		opts: &options{
			args:     []string{"-count", "1", "-run"},
			packages: []string{"./pkg"},
		},
		rerunOpts: rerunOpts{
			runFlag: "-run=TestOne|TestTwo",
			pkg:     "./fails",
		},
		expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-count", "1", "-run", "./fails"},
	})
	t.Run("rerun with -run flag", func(t *testing.T) {
		tc := testCase{
			opts: &options{
				args:     []string{"-run", "TestExample", "-tags", "some", "-json"},
				packages: []string{"./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-run=TestOne|TestTwo", "-tags", "some", "-json", "./fails"},
		}
		run(t, "first", tc)
		run(t, "second", tc)
	})
}

func runCase(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		t.Log("case:", name)
		fn(t)
	})
}

func TestRun_RerunFails_WithTooManyInitialFailures(t *testing.T) {
	jsonFailed := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "fail"}
{"Package": "pkg", "Test": "TestTwo", "Action": "run"}
{"Package": "pkg", "Test": "TestTwo", "Action": "fail"}
{"Package": "pkg", "Action": "fail"}
`

	fn := func([]string) *proc {
		return &proc{
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
		hideSummary:                  newHideSummaryValue(),
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

	fn := func([]string) *proc {
		return &proc{
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
		hideSummary:                  newHideSummaryValue(),
	}
	err := run(opts)
	assert.ErrorContains(t, err, "rerun aborted because previous run had errors", out.String())
}

// type checking of os/exec.ExitError is done in a test file so that users
// installing from source can continue to use versions prior to go1.12.
var _ exitCoder = &exec.ExitError{}

func TestRun_RerunFails_PanicPreventsRerun(t *testing.T) {
	jsonFailed := `{"Package": "pkg", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "run"}
{"Package": "pkg", "Test": "TestOne", "Action": "output","Output":"panic: something went wrong\n"}
{"Package": "pkg", "Action": "fail"}
`

	fn := func([]string) *proc {
		return &proc{
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
		hideSummary:                  newHideSummaryValue(),
	}
	err := run(opts)
	assert.ErrorContains(t, err, "rerun aborted because previous run had a suspected panic", out.String())
}

func TestRun_InputFromStdin(t *testing.T) {
	stdin := os.Stdin
	t.Cleanup(func() { os.Stdin = stdin })

	r, w, err := os.Pipe()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	os.Stdin = r

	go func() {
		defer func() { _ = w.Close() }()

		e := json.NewEncoder(w)
		for _, event := range []testjson.TestEvent{
			{Action: "run", Package: "pkg"},
			{Action: "run", Package: "pkg", Test: "TestOne"},
			{Action: "fail", Package: "pkg", Test: "TestOne"},
			{Action: "fail", Package: "pkg"},
		} {
			assert.Check(t, e.Encode(event))
		}
	}()

	stdout := new(bytes.Buffer)
	err = run(&options{
		args:        []string{"cat"},
		format:      "testname",
		hideSummary: newHideSummaryValue(),
		rawCommand:  true,

		stdout: stdout,
		stderr: os.Stderr,
	})
	assert.NilError(t, err)
	assert.Assert(t, cmp.Contains(stdout.String(), "DONE 1"))
}

func TestRun_JsonFileIsSyncedBeforePostRunCommand(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

	input := golden.Get(t, "../../testjson/testdata/input/go-test-json.out")

	fn := func([]string) *proc {
		return &proc{
			cmd:    fakeWaiter{},
			stdout: bytes.NewReader(input),
			stderr: bytes.NewReader(nil),
		}
	}
	reset := patchStartGoTestFn(fn)
	defer reset()

	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "json.log")

	out := new(bytes.Buffer)
	opts := &options{
		rawCommand:  true,
		args:        []string{"./test.test"},
		format:      "none",
		stdout:      out,
		stderr:      os.Stderr,
		hideSummary: &hideSummaryValue{value: testjson.SummarizeNone},
		jsonFile:    jsonFile,
		postRunHookCmd: &commandValue{
			command: []string{"cat", jsonFile},
		},
	}
	err := run(opts)
	assert.NilError(t, err)
	expected := string(input)
	_, actual, _ := strings.Cut(out.String(), "s\n") // remove the DONE line
	assert.Equal(t, actual, expected)
}

func TestRun_JsonFileTimingEvents(t *testing.T) {
	input := golden.Get(t, "../../testjson/testdata/input/go-test-json.out")

	fn := func([]string) *proc {
		return &proc{
			cmd:    fakeWaiter{},
			stdout: bytes.NewReader(input),
			stderr: bytes.NewReader(nil),
		}
	}
	reset := patchStartGoTestFn(fn)
	defer reset()

	tmp := t.TempDir()
	jsonFileTiming := filepath.Join(tmp, "json.log")

	out := new(bytes.Buffer)
	opts := &options{
		rawCommand:           true,
		args:                 []string{"./test.test"},
		format:               "none",
		stdout:               out,
		stderr:               os.Stderr,
		hideSummary:          &hideSummaryValue{value: testjson.SummarizeNone},
		jsonFileTimingEvents: jsonFileTiming,
	}
	err := run(opts)
	assert.NilError(t, err)

	raw, err := os.ReadFile(jsonFileTiming)
	assert.NilError(t, err)
	golden.Assert(t, string(raw), "expected-jsonfile-timing-events")
}
