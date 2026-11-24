package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

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

// newRerunOptsFromTestCases groups failed test cases that share a package and
// a parent test, so each group is re-run by a single go test invocation.
func newRerunOptsFromTestCases(tcs []testjson.TestCase) []rerunOpts {
	type groupKey struct{ pkg, parent string }
	var keys []groupKey
	names := map[groupKey][]testjson.TestName{}
	for _, tc := range tcs {
		key := groupKey{pkg: tc.Package, parent: tc.Test.Parent()}
		if _, ok := names[key]; !ok {
			keys = append(keys, key)
		}
		if !slices.Contains(names[key], tc.Test) {
			names[key] = append(names[key], tc.Test)
		}
	}

	result := make([]rerunOpts, 0, len(keys))
	for _, key := range keys {
		result = append(result, rerunOpts{
			runFlag: goTestRunFlagForTestCases(names[key]),
			pkg:     key.pkg,
		})
	}
	return result
}

type testCaseFilter func([]testjson.TestCase) []testjson.TestCase

func rerunFailsFilter(o *options) testCaseFilter {
	if o.rerunFailsRunRootCases {
		return func(tcs []testjson.TestCase) []testjson.TestCase {
			var result []testjson.TestCase
			for _, tc := range tcs {
				if !tc.Test.IsSubTest() {
					result = append(result, tc)
				}
			}
			return result
		}
	}
	return testjson.FilterFailedUnique
}

func rerunFailed(ctx context.Context, opts *options, scanConfig testjson.ScanConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tcFilter := rerunFailsFilter(opts)

	rec := newFailureRecorderFromExecution(scanConfig.Execution)
	for attempts := 0; rec.count() > 0 && attempts < opts.rerunFailsMaxAttempts; attempts++ {
		testjson.PrintSummary(opts.stdout, scanConfig.Execution, testjson.SummarizeNone)
		opts.stdout.Write([]byte("\n")) //nolint:errcheck

		nextRec := newFailureRecorder(scanConfig.Handler)
		for _, rerun := range newRerunOptsFromTestCases(tcFilter(rec.failures)) {
			goTestProc, err := startGoTestFn(ctx, "", goTestCmdArgs(opts, rerun))
			if err != nil {
				return err
			}

			cfg := testjson.ScanConfig{
				RunID:     attempts + 1,
				Stdout:    goTestProc.stdout,
				Stderr:    goTestProc.stderr,
				Handler:   nextRec,
				Execution: scanConfig.Execution,
				Stop:      cancel,
			}
			if _, err := testjson.ScanTestOutput(cfg); err != nil {
				return err
			}
			exitErr := goTestProc.cmd.Wait()
			if exitErr != nil {
				nextRec.lastErr = exitErr
			}
			if err := hasErrors(exitErr, scanConfig.Execution, opts); err != nil {
				return err
			}
		}
		rec = nextRec
	}
	return rec.lastErr
}

// startGoTestFn is a shim for testing
var startGoTestFn = startGoTest

func hasErrors(err error, exec *testjson.Execution, opts *options) error {
	switch {
	case len(exec.Errors()) > 0:
		return fmt.Errorf("rerun aborted because previous run had errors")
	// Exit code 0 and 1 are expected.
	case ExitCodeWithDefault(err) > 1:
		return fmt.Errorf("unexpected go test exit code: %v", err)
	case exec.HasPanic():
		return fmt.Errorf("rerun aborted because previous run had a suspected panic and some test may not have run")
	case exec.HasDataRace() && opts.rerunFailsMaxAttempts > 0 && opts.rerunFailsAbortOnDataRace:
		return fmt.Errorf("rerun aborted because previous run had a data race")
	default:
		return nil
	}
}

type failureRecorder struct {
	testjson.EventHandler
	failures []testjson.TestCase
	lastErr  error
}

func newFailureRecorder(handler testjson.EventHandler) *failureRecorder {
	return &failureRecorder{EventHandler: handler}
}

func newFailureRecorderFromExecution(exec *testjson.Execution) *failureRecorder {
	return &failureRecorder{failures: exec.Failed()}
}

func (r *failureRecorder) Event(event testjson.TestEvent, execution *testjson.Execution) error {
	if !event.PackageEvent() && event.Action == testjson.ActionFail {
		pkg := execution.Package(event.Package)
		tc := pkg.LastFailedByName(event.Test)
		r.failures = append(r.failures, tc)
	}
	return r.EventHandler.Event(event, execution)
}

func (r *failureRecorder) count() int {
	return len(r.failures)
}

// goTestRunFlagForTestCases returns a -test.run flag matching all the tests,
// which must share the same parent test.
func goTestRunFlagForTestCases(tests []testjson.TestName) string {
	var sb strings.Builder
	sb.WriteString("-test.run=")
	if parent := tests[0].Parent(); parent != "" {
		for _, p := range strings.Split(parent, "/") {
			sb.WriteString("^" + regexp.QuoteMeta(p) + "$/")
		}
	}

	leaves := make([]string, len(tests))
	for i, test := range tests {
		parts := strings.Split(string(test), "/")
		leaves[i] = regexp.QuoteMeta(parts[len(parts)-1])
	}
	if len(leaves) == 1 {
		sb.WriteString("^" + leaves[0] + "$")
	} else {
		sb.WriteString("^(" + strings.Join(leaves, "|") + ")$")
	}
	return sb.String()
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
		name := failure.Package + "." + failure.Test.Name()
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

	defer func() {
		_ = fh.Close()
	}()

	sort.Strings(names)
	for _, name := range names {
		counts := results[name]
		fmt.Fprintf(fh, "%s: %d runs, %d failures\n", name, counts.total, counts.failed)
	}
	return nil
}
