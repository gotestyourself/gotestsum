package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gotest.tools/gotestsum/internal/text"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestMain(m *testing.M) {
	code := m.Run()
	binaryFixture.Cleanup()
	os.Exit(code)
}

func TestE2E_RerunFails(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for short run")
	}

	type testCase struct {
		name        string
		args        []string
		expectedErr string
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
		if tc.expectedErr != "" {
			assert.Error(t, err, tc.expectedErr)
		} else {
			assert.NilError(t, err)
		}
		out := text.ProcessLines(t, bufStdout,
			text.OpRemoveSummaryLineElapsedTime,
			text.OpRemoveTestElapsedTime,
			filepath.ToSlash, // for windows
		)
		golden.Assert(t, out, "e2e/expected/"+expectedFilename(t.Name()))
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
			expectedErr: "exit status 1",
		},
		{
			name: "first run has errors, abort rerun",
			args: []string{
				"-f=testname",
				"--rerun-fails=2",
				"--packages=../testjson/internal/broken",
				"--", "-count=1", "-tags=stubpkg",
			},
			expectedErr: "rerun aborted because previous run had errors",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
	ver := runtime.Version()
	switch {
	case isPreGo114(ver):
		return name + "-go1.13"
	default:
		return name
	}
}

// go1.14.6 changed how it prints messages from tests. go1.14.{0-5} used a format
// that was different from both go1.14.6 and previous versions of Go. These tests
// no longer support that format.
func isPreGo114(ver string) bool {
	prefix := "go1.1"
	if !strings.HasPrefix(ver, prefix) || len(ver) < len(prefix)+1 {
		return false
	}
	switch ver[len(prefix)] {
	case '0', '1', '2', '3':
		return true
	}
	return false
}

var binaryFixture pkgFixtureFile

type pkgFixtureFile struct {
	filename string
	once     sync.Once
	cleanup  func()
}

func (p *pkgFixtureFile) Path() string {
	return p.filename
}

func (p *pkgFixtureFile) Do(f func() string) {
	p.once.Do(func() {
		p.filename = f()
		p.cleanup = func() {
			os.RemoveAll(p.filename) // nolint: errcheck
		}
	})
}

func (p *pkgFixtureFile) Cleanup() {
	if p.cleanup != nil {
		p.cleanup()
	}
}

// compileBinary once the first time this function is called. Subsequent calls
// will return the path to the compiled binary. The binary is removed when all
// the tests in this package have completed.
func compileBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("too slow for short run")
	}

	binaryFixture.Do(func() string {
		tmpDir, err := ioutil.TempDir("", "gotestsum-binary")
		assert.NilError(t, err)

		path := filepath.Join(tmpDir, "gotestsum")
		result := icmd.RunCommand("go", "build", "-o", path, "..")
		result.Assert(t, icmd.Success)
		return path
	})

	if binaryFixture.Path() == "" {
		t.Skip("previous attempt to compile the binary failed")
	}
	return binaryFixture.Path()
}

func TestE2E_SignalHandler(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "test timeout waiting for pidfile")
	bin := compileBinary(t)

	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()

	driver := tmpDir.Join("driver")
	target := filepath.FromSlash("./internal/signalhandlerdriver/")
	icmd.RunCommand("go", "build", "-o", driver, target).
		Assert(t, icmd.Success)

	pidFile := tmpDir.Join("pidfile")
	args := []string{"--raw-command", "--", driver, pidFile}
	result := icmd.StartCmd(icmd.Command(bin, args...))

	poll.WaitOn(t, poll.FileExists(pidFile), poll.WithTimeout(time.Second))
	assert.NilError(t, result.Cmd.Process.Signal(os.Interrupt))
	icmd.WaitOnCmd(2*time.Second, result)

	result.Assert(t, icmd.Expected{ExitCode: 130})
}

func TestE2E_MaxFails_EndTestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for short run")
	}

	tmpFile := fs.NewFile(t, t.Name()+"-seedfile", fs.WithContent("0"))
	defer tmpFile.Remove()

	envVars := osEnviron()
	envVars["TEST_SEEDFILE"] = tmpFile.Path()
	defer env.PatchAll(t, envVars)()

	flags, opts := setupFlags("gotestsum")
	args := []string{"--max-fails=2", "--packages=./testdata/e2e/flaky/", "--", "-tags=testdata"}
	assert.NilError(t, flags.Parse(args))
	opts.args = flags.Args()

	bufStdout := new(bytes.Buffer)
	opts.stdout = bufStdout
	bufStderr := new(bytes.Buffer)
	opts.stderr = bufStderr

	err := run(opts)
	assert.Error(t, err, "ending test run because max failures was reached")
	out := text.ProcessLines(t, bufStdout,
		text.OpRemoveSummaryLineElapsedTime,
		text.OpRemoveTestElapsedTime,
		filepath.ToSlash, // for windows
	)
	golden.Assert(t, out, "e2e/expected/"+t.Name())
}
