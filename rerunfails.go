package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gotest.tools/gotestsum/testjson"
)

type rerunOpts struct {
	runFlag string
	pkg     string
}

func (o rerunOpts) Args() []string {
	var result []string
	if o.runFlag != "" {
		result = append(result, o.runFlag)
	}
	if o.pkg != "" {
		result = append(result, o.pkg)
	}
	return result
}

func rerunFailed(ctx context.Context, opts *options, scanConfig testjson.ScanConfig) error {
	failed := len(scanConfig.Execution.Failed())
	if failed > opts.rerunFailsMaxInitialFailures {
		return fmt.Errorf(
			"number of test failures (%d) exceeds maximum (%d) set by --rerun-fails-max-failures",
			failed, opts.rerunFailsMaxInitialFailures)
	}

	rec := newFailureRecorderFromExecution(scanConfig.Execution)

	var lastErr error
	for count := 0; rec.count() > 0 && count < opts.rerunFailsMaxAttempts; count++ {
		nextRec := newFailureRecorder(scanConfig.Handler)

		for pkg, testCases := range rec.pkgFailures {
			rerun := rerunOpts{
				runFlag: goTestRunFlagFromTestCases(testCases),
				pkg:     pkg,
			}
			goTestProc, err := startGoTest(ctx, goTestCmdArgs(opts, rerun))
			if err != nil {
				return errors.Wrapf(err, "failed to run %s", strings.Join(goTestProc.cmd.Args, " "))
			}

			cfg := testjson.ScanConfig{
				Stdout:    goTestProc.stdout,
				Stderr:    goTestProc.stderr,
				Handler:   nextRec,
				Execution: scanConfig.Execution,
			}
			if _, err := testjson.ScanTestOutput(cfg); err != nil {
				return err
			}
			lastErr = goTestProc.cmd.Wait()
			rec = nextRec
		}
	}
	return lastErr
}

type failureRecorder struct {
	testjson.EventHandler
	pkgFailures map[string][]string
}

func newFailureRecorder(handler testjson.EventHandler) *failureRecorder {
	return &failureRecorder{
		EventHandler: handler,
		pkgFailures:  make(map[string][]string),
	}
}

func newFailureRecorderFromExecution(exec *testjson.Execution) *failureRecorder {
	r := newFailureRecorder(nil)
	for _, tc := range exec.Failed() {
		r.pkgFailures[tc.Package] = append(r.pkgFailures[tc.Package], tc.Test)
	}
	return r
}

func (r *failureRecorder) Event(event testjson.TestEvent, execution *testjson.Execution) error {
	if !event.PackageEvent() && event.Action == testjson.ActionFail {
		r.pkgFailures[event.Package] = append(r.pkgFailures[event.Package], event.Test)
	}
	return r.EventHandler.Event(event, execution)
}

func (r *failureRecorder) count() int {
	total := 0
	for _, tcs := range r.pkgFailures {
		total += len(tcs)
	}
	return total
}

func goTestRunFlagFromTestCases(tcs []string) string {
	buf := new(strings.Builder)
	buf.WriteString("-run=")
	for i, tc := range tcs {
		if i != 0 {
			buf.WriteString("|")
		}
		buf.WriteString(tc)
	}
	return buf.String()
}
