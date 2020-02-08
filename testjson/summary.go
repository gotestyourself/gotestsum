package testjson

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fatih/color"
)

// Summary enumerates the sections which can be printed by PrintSummary
type Summary int

// nolint: golint
const (
	SummarizeNone    Summary = 0
	SummarizeSkipped Summary = (1 << iota) / 2
	SummarizeFailed
	SummarizeErrors
	SummarizeOutput
	SummarizeSlowestTests
	SummarizeAll = SummarizeSkipped |
		SummarizeFailed |
		SummarizeErrors |
		SummarizeOutput |
		SummarizeSlowestTests
)

var summaryValues = map[Summary]string{
	SummarizeSkipped:      "skipped",
	SummarizeFailed:       "failed",
	SummarizeErrors:       "errors",
	SummarizeOutput:       "output",
	SummarizeSlowestTests: "slowest",
}

var summaryFromValue = map[string]Summary{
	"none":    SummarizeNone,
	"skipped": SummarizeSkipped,
	"failed":  SummarizeFailed,
	"errors":  SummarizeErrors,
	"output":  SummarizeOutput,
	"slowest": SummarizeSlowestTests,
	"all":     SummarizeAll,
}

func (s Summary) String() string {
	if s == SummarizeNone {
		return "none"
	}
	var result []string
	for v := Summary(1); v <= s; v <<= 1 {
		if s.Includes(v) {
			result = append(result, summaryValues[v])
		}
	}
	return strings.Join(result, ",")
}

// Includes returns true if Summary includes all the values set by other.
func (s Summary) Includes(other Summary) bool {
	return s&other == other
}

// NewSummary returns a new Summary from a string value. If the string does not
// match any known values returns false for the second value.
func NewSummary(value string) (Summary, bool) {
	s, ok := summaryFromValue[value]
	return s, ok
}

// SummaryOptions used to configure the behaviour of PrintSummary.
type SummaryOptions struct {
	Summary           Summary
	SlowTestThreshold time.Duration
}

// PrintSummary of a test Execution. Prints a section for each summary type
// followed by a DONE line.
func PrintSummary(out io.Writer, execution *Execution, opts SummaryOptions) {
	sum := opts.Summary
	execSummary := newExecSummary(execution, sum)
	slowTests := slowTestCases(execution, opts.SlowTestThreshold)

	if sum.Includes(SummarizeSkipped) {
		writeTestCaseSummary(out, execSummary, formatSkipped())
	}
	if sum.Includes(SummarizeSlowestTests) {
		writeSlowestTests(out, slowTests)
	}
	if sum.Includes(SummarizeFailed) {
		writeTestCaseSummary(out, execSummary, formatFailed())
	}
	if sum.Includes(SummarizeErrors) {
		writeErrorSummary(out, execution.Errors())
	}
	printSummaryLine(out, execution, slowTests)
}

func printSummaryLine(out io.Writer, execution *Execution, slowTests []TestCase) {
	fmt.Fprintf(out, "\n%s %d tests%s%s%s%s in %s\n",
		formatExecStatus(execution.done),
		execution.Total(),
		formatTestCount(len(execution.Skipped()), "skipped", ""),
		formatTestCount(len(slowTests), "slow", ""),
		formatTestCount(len(execution.Failed()), "failure", "s"),
		formatTestCount(countErrors(execution.Errors()), "error", "s"),
		FormatDurationAsSeconds(execution.Elapsed(), 3))
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

// TODO: maybe color this?
func formatExecStatus(done bool) string {
	if done {
		return "DONE"
	}
	return ""
}

// FormatDurationAsSeconds formats a time.Duration as a float with an s suffix.
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

// countErrors in stderr lines. Build errors may include multiple lines where
// subsequent lines are indented.
// FIXME: Panics will include multiple lines, and are still overcounted.
func countErrors(errors []string) int {
	var count int
	for _, line := range errors {
		r, _ := utf8.DecodeRuneInString(line)
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

type executionSummary interface {
	Failed() []TestCase
	Skipped() []TestCase
	OutputLines(pkg, test string) []string
}

type noOutputSummary struct {
	Execution
}

func (s *noOutputSummary) OutputLines(_, _ string) []string {
	return nil
}

func newExecSummary(execution *Execution, opts Summary) executionSummary {
	if opts.Includes(SummarizeOutput) {
		return execution
	}
	return &noOutputSummary{Execution: *execution}
}

func writeTestCaseSummary(out io.Writer, execution executionSummary, conf testCaseFormatConfig) {
	testCases := conf.getter(execution)
	if len(testCases) == 0 {
		return
	}
	fmt.Fprintln(out, "\n=== "+conf.header)
	for _, tc := range testCases {
		fmt.Fprintf(out, "=== %s: %s %s (%s)\n",
			conf.prefix,
			RelativePackagePath(tc.Package),
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
	getter func(executionSummary) []TestCase
}

func formatFailed() testCaseFormatConfig {
	withColor := color.RedString
	return testCaseFormatConfig{
		header: withColor("Failed"),
		prefix: withColor("FAIL"),
		filter: func(line string) bool {
			return strings.HasPrefix(line, "--- FAIL: Test")
		},
		getter: func(execution executionSummary) []TestCase {
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
		getter: func(execution executionSummary) []TestCase {
			return execution.Skipped()
		},
	}
}

func isRunLine(line string) bool {
	return strings.HasPrefix(line, "=== RUN   Test")
}

// slowTestCases returns a slice of all tests with an elapsed time greater than
// threshold. The slice is sorted by Elapsed time in descending order (slowest
// test first).
func slowTestCases(exec *Execution, threshold time.Duration) []TestCase {
	if threshold == 0 {
		return nil
	}
	tests := make([]TestCase, 0, len(exec.packages))
	for _, tc := range exec.packages {
		tests = append(tests, tc.TestCases()...)
	}
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Elapsed > tests[j].Elapsed
	})
	end := sort.Search(len(tests), func(i int) bool {
		return tests[i].Elapsed < threshold
	})
	return tests[:end]
}

func writeSlowestTests(out io.Writer, tests []TestCase) {
	if len(tests) == 0 {
		return
	}
	// TODO make max configurable
	const maxSlowTests = 5
	end := len(tests)
	if end > maxSlowTests {
		end = maxSlowTests
	}
	fmt.Fprintln(out, "\n=== Slowest")
	for _, tc := range tests[:end] {
		fmt.Fprintf(out, "    %s %s (%v)\n", tc.Package, tc.Test, tc.Elapsed)
	}
	fmt.Fprintln(out)
}
