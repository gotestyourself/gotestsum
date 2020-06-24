package main

import (
	"context"
	"fmt"
	"os"
	"sort"
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
	for attempts := 0; rec.count() > 0 && attempts < opts.rerunFailsMaxAttempts; attempts++ {
		testjson.PrintSummary(opts.stdout, scanConfig.Execution, testjson.SummarizeNone)
		opts.stdout.Write([]byte("\n")) // nolint: errcheck

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
				RunID:     attempts + 1,
				Stdout:    goTestProc.stdout,
				Stderr:    goTestProc.stderr,
				Handler:   nextRec,
				Execution: scanConfig.Execution,
			}
			if _, err := testjson.ScanTestOutput(cfg); err != nil {
				return err
			}
			lastErr = goTestProc.cmd.Wait()
			if err := hasErrors(lastErr, scanConfig.Execution); err != nil {
				return err
			}
			rec = nextRec
		}
	}
	return lastErr
}

func hasErrors(err error, exec *testjson.Execution) error {
	switch {
	case len(exec.Errors()) > 0:
		return fmt.Errorf("rerun aborted because previous run had errors")
	// Exit code 0 and 1 are expected.
	case ExitCodeWithDefault(err) > 1:
		return fmt.Errorf("unexpected go test exit code: %v", err)
	default:
		return nil
	}
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
	buf.WriteString("-run=^(")
	for i, tc := range tcs {
		if i != 0 {
			buf.WriteString("|")
		}
		buf.WriteString(tc)
	}
	buf.WriteString(")$")
	return buf.String()
}

func writeRerunFailsReport(opts *options, exec *testjson.Execution) error {
	if opts.rerunFailsMaxAttempts == 0 || opts.rerunFailsReportFile == "" {
		return nil
	}

	type testCaseCounts struct {
		total  int
		failed int
	}

	names := []string{}
	results := map[string]testCaseCounts{}
	for _, failure := range exec.Failed() {
		name := failure.Package + "." + failure.Test
		if _, ok := results[name]; ok {
			continue
		}
		names = append(names, name)

		pkg := exec.Package(failure.Package)
		counts := testCaseCounts{}

		for _, tc := range pkg.Failed {
			if tc.Test == failure.Test {
				counts.total++
				counts.failed++
			}
		}
		for _, tc := range pkg.Passed {
			if tc.Test == failure.Test {
				counts.total++
			}
		}
		// Skipped tests are not counted, but presumably skipped tests can not fail
		results[name] = counts
	}

	fh, err := os.Create(opts.rerunFailsReportFile)
	if err != nil {
		return err
	}

	sort.Strings(names)
	for _, name := range names {
		counts := results[name]
		fmt.Fprintf(fh, "%s: %d runs, %d failures\n", name, counts.total, counts.failed)
	}
	return nil
}
