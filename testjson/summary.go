package testjson

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Summary enumerates the sections which can be printed by PrintSummary
type Summary int

const (
	SummarizeNone Summary = 1 << (iota * 2)
	SummarizeSkipped
	SummarizeFailed
	SummarizeErrors

	SummarizeAll = SummarizeSkipped | SummarizeFailed | SummarizeErrors
)

// PrintSummary of a test Execution. Prints a section for each summary type
// followed by a DONE line.
func PrintSummary(out io.Writer, execution *Execution, opts Summary) error {
	if opts&SummarizeSkipped != 0 {
		writeTestCaseSummary(out, execution, formatSkipped())
	}
	if opts&SummarizeFailed != 0 {
		writeTestCaseSummary(out, execution, formatFailed())
	}

	errors := execution.Errors()
	if opts&SummarizeErrors != 0 {
		writeErrorSummary(out, errors)
	}

	fmt.Fprintf(out, "\n%s %d tests%s%s%s in %s\n",
		"DONE", // TODO: maybe color this?
		execution.Total(),
		formatTestCount(len(execution.Skipped()), "skipped", ""),
		formatTestCount(len(execution.Failed()), "failure", "s"),
		// TODO: only count lines without any leading whitespace as errors
		formatTestCount(len(errors), "error", "s"),
		FormatDurationAsSeconds(execution.Elapsed(), 3))

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

func writeErrorSummary(out io.Writer, errors []string) {
	if len(errors) > 0 {
		fmt.Fprintln(out, color.MagentaString("\n=== Errors"))
	}
	for _, err := range errors {
		fmt.Fprintln(out, err)
	}
}

func writeTestCaseSummary(out io.Writer, execution *Execution, conf testCaseFormatConfig) {
	testCases := conf.getter(execution)
	if len(testCases) == 0 {
		return
	}
	fmt.Fprintln(out, "\n=== "+conf.header)
	for _, tc := range testCases {
		fmt.Fprintf(out, "=== %s: %s %s (%s)\n",
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

func formatFailed() testCaseFormatConfig {
	withColor := color.RedString
	return testCaseFormatConfig{
		header: withColor("Failed"),
		prefix: withColor("FAIL"),
		filter: func(line string) bool {
			return strings.HasPrefix(line, "--- FAIL: Test")
		},
		getter: func(execution *Execution) []TestCase {
			return execution.Failed()
		},
	}
}

func formatSkipped() testCaseFormatConfig {
	withColor := color.YellowString
	return testCaseFormatConfig{
		header: withColor("Skipped"),
		prefix: withColor("SKIP"),
		filter: func(line string) bool {
			return strings.HasPrefix(line, "--- SKIP: Test")
		},
		getter: func(execution *Execution) []TestCase {
			return execution.Skipped()
		},
	}
}

func isRunLine(line string) bool {
	return strings.HasPrefix(line, "=== RUN   Test")
}
