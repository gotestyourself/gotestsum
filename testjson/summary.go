package testjson

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// PrintSummary of a test Execution. Prints a DONE line with counts, following
// by any skips, failures, or errors.
func PrintSummary(out io.Writer, execution *Execution) error {
	errors := execution.Errors()
	fmt.Fprintf(out, "\nDONE %d tests%s%s%s in %s\n",
		execution.Total(),
		formatTestCount(len(execution.Skipped()), "skipped", ""),
		formatTestCount(len(execution.Failed()), "failure", "s"),
		formatTestCount(len(errors), "error", "s"),
		FormatDurationAsSeconds(execution.Elapsed(), 3))

	writeTestCaseSummary(out, execution, formatSkipped)
	writeTestCaseSummary(out, execution, formatFailures)

	if len(errors) > 0 {
		fmt.Fprintln(out, "\n=== Errors")
	}
	for _, err := range errors {
		fmt.Fprintln(out, err)
	}

	return nil
}

func formatTestCount(count int, category string, pluralize string) string {
	switch count {
	case 0:
		return ""
	case 1:
	default:
		category += pluralize
	}
	return fmt.Sprintf(", %d %s", count, category)
}

// FormatDurationAsSeconds formats a time.Duration as a float.
func FormatDurationAsSeconds(d time.Duration, precision int) string {
	return fmt.Sprintf("%.[2]*[1]fs", d.Seconds(), precision)
}

func writeTestCaseSummary(out io.Writer, execution *Execution, conf testCaseFormatConfig) {
	testCases := conf.getter(execution)
	if len(testCases) == 0 {
		return
	}
	fmt.Fprintln(out, "\n"+conf.header)
	for _, tc := range testCases {
		fmt.Fprintf(out, "%s %s %s (%s)\n",
			conf.prefix,
			relativePackagePath(tc.Package),
			tc.Test,
			FormatDurationAsSeconds(tc.Elapsed, 2))
		for _, line := range execution.OutputLines(tc.Package, tc.Test) {
			if isRunLine(line) || conf.filter(line) {
				continue
			}
			fmt.Fprint(out, line)
		}
		fmt.Fprintln(out)
	}
}

type testCaseFormatConfig struct {
	header string
	prefix string
	filter func(string) bool
	getter func(*Execution) []TestCase
}

var formatFailures = testCaseFormatConfig{
	header: "=== Failures",
	prefix: "=== FAIL:",
	filter: func(line string) bool {
		return strings.HasPrefix(line, "--- FAIL: Test")
	},
	getter: func(execution *Execution) []TestCase {
		return execution.Failed()
	},
}

var formatSkipped = testCaseFormatConfig{
	header: "=== Skipped",
	prefix: "=== SKIP:",
	filter: func(line string) bool {
		return strings.HasPrefix(line, "--- SKIP: Test")
	},
	getter: func(execution *Execution) []TestCase {
		return execution.Skipped()
	},
}

func isRunLine(line string) bool {
	return strings.HasPrefix(line, "=== RUN   Test")
}
