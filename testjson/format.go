package testjson

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

func debugFormat(out io.Writer) eventFormatterFunc {
	return func(event TestEvent, _ *Execution) error {
		_, err := fmt.Fprintf(out, "%s %s %s (%.3f) [%d] %s\n",
			event.Package,
			event.Test,
			event.Action,
			event.Elapsed,
			event.Time.Unix(),
			event.Output)
		return err
	}
}

// go test -v
func standardVerboseFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		if event.Action == ActionOutput {
			_, _ = buf.WriteString(event.Output)
			return buf.Flush()
		}
		return nil
	})
}

// go test
func standardQuietFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		if !event.PackageEvent() {
			return nil
		}
		if event.Output == "PASS\n" || isCoverageOutput(event.Output) {
			return nil
		}
		if isWarningNoTestsToRunOutput(event.Output) {
			return nil
		}

		_, _ = buf.WriteString(event.Output)
		return buf.Flush()
	})
}

// go test -json
func standardJSONFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	// nolint:errcheck // errors are returned by Flush
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		buf.Write(event.raw)
		buf.WriteRune('\n')
		return buf.Flush()
	})
}

func testNameFormat(out io.Writer, opts FormatOptions) EventFormatter {
	buf := bufio.NewWriter(out)
	// nolint:errcheck
	return eventFormatterFunc(func(event TestEvent, exec *Execution) error {
		formatTest := func() error {
			pkgPath := RelativePackagePath(event.Package)

			if opts.OutputWallTime {
				buf.WriteString(fmtElapsed(exec.Elapsed(), false)) // nolint:errcheck
			}
			fmt.Fprintf(buf, "%s %s%s %s\n",
				colorEvent(event)(strings.ToUpper(string(event.Action))),
				joinPkgToTestName(pkgPath, event.Test),
				formatRunID(event.RunID),
				event.ElapsedFormatted())
			return buf.Flush()
		}

		switch {
		case isPkgFailureOutput(event):
			buf.WriteString(event.Output)
			return buf.Flush()

		case event.PackageEvent():
			if !event.Action.IsTerminal() {
				return nil
			}

			result := colorEvent(event)(strings.ToUpper(string(event.Action)))
			pkg := exec.Package(event.Package)
			if event.Action == ActionSkip || (event.Action == ActionPass && pkg.Total == 0) {
				result = colorEvent(event)("EMPTY")
			}

			if opts.OutputWallTime {
				buf.WriteString(fmtElapsed(exec.Elapsed(), false)) // nolint:errcheck
			}
			event.Elapsed = 0 // hide elapsed for now, for backwards compat
			buf.WriteString(result)
			buf.WriteRune(' ')
			buf.WriteString(packageLine(event, exec.Package(event.Package)))
			return buf.Flush()

		case event.Action == ActionFail:
			pkg := exec.Package(event.Package)
			tc := pkg.LastFailedByName(event.Test)
			pkg.WriteOutputTo(buf, tc.ID)
			return formatTest()

		case event.Action == ActionPass:
			return formatTest()
		}
		return nil
	})
}

// joinPkgToTestName for formatting.
// If the package path isn't the current directory, we add a period to separate
// the test name and the package path. If it is the current directory, we don't
// show it at all. This prevents output like ..MyTest when the test is in the
// current directory.
func joinPkgToTestName(pkg string, test string) string {
	if pkg == "." {
		return test
	}
	return pkg + "." + test
}

// formatRunID returns a formatted string of the runID.
func formatRunID(runID int) string {
	if runID <= 0 {
		return ""
	}
	return fmt.Sprintf(" (re-run %d)", runID)
}

// isPkgFailureOutput returns true if the event is package output, and the output
// doesn't match any of the expected framing messages. Events which match this
// pattern should be package-level failures (ex: exit(1) or panic in an init() or
// TestMain).
func isPkgFailureOutput(event TestEvent) bool {
	out := event.Output
	return all(
		event.PackageEvent(),
		event.Action == ActionOutput,
		out != "PASS\n",
		out != "FAIL\n",
		!isWarningNoTestsToRunOutput(out),
		!strings.HasPrefix(out, "FAIL\t"+event.Package),
		!strings.HasPrefix(out, "ok  \t"+event.Package),
		!strings.HasPrefix(out, "?   \t"+event.Package),
		!isShuffleSeedOutput(out),
	)
}

func all(cond ...bool) bool {
	for _, c := range cond {
		if !c {
			return false
		}
	}
	return true
}

func pkgNameFormat(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && opts.OutputTestFailures {
				pkg := exec.Package(event.Package)
				tc := pkg.LastFailedByName(event.Test)
				pkg.WriteOutputTo(buf, tc.ID) // nolint:errcheck
				return buf.Flush()
			}
			return nil
		}
		eventStr := shortFormatPackageEvent(opts, event, exec)
		if eventStr != "" && opts.OutputWallTime {
			buf.WriteString(fmtElapsed(exec.Elapsed(), false)) // nolint:errcheck
		}
		buf.WriteString(eventStr) // nolint:errcheck
		return buf.Flush()
	}
}

func shortFormatPackageEvent(opts FormatOptions, event TestEvent, exec *Execution) string {
	pkg := exec.Package(event.Package)

	var iconSkipped, iconSuccess, iconFailure string
	if opts.UseHiVisibilityIcons {
		iconSkipped = "➖"
		iconSuccess = "✅"
		iconFailure = "❌"
	} else {
		iconSkipped = "∅"
		iconSuccess = "✓"
		iconFailure = "✖"
	}

	fmtEvent := func(action string) string {
		return action + "  " + packageLine(event, exec.Package(event.Package))
	}
	withColor := colorEvent(event)
	switch event.Action {
	case ActionSkip:
		if opts.HideEmptyPackages {
			return ""
		}
		return fmtEvent(withColor(iconSkipped))
	case ActionPass:
		if pkg.Total == 0 {
			if opts.HideEmptyPackages {
				return ""
			}
			return fmtEvent(withColor(iconSkipped))
		}
		return fmtEvent(withColor(iconSuccess))
	case ActionFail:
		return fmtEvent(withColor(iconFailure))
	}
	return ""
}

func packageLine(event TestEvent, pkg *Package) string {
	var buf strings.Builder
	buf.WriteString(RelativePackagePath(event.Package))

	switch {
	case pkg.cached:
		buf.WriteString(" (cached)")
	case event.Elapsed != 0:
		d := elapsedDuration(event.Elapsed)
		buf.WriteString(fmt.Sprintf(" (%s)", d))
	}

	if pkg.coverage != "" {
		buf.WriteString(" (" + pkg.coverage + ")")
	}

	if event.Action == ActionFail && pkg.shuffleSeed != "" {
		buf.WriteString(" (" + pkg.shuffleSeed + ")")
	}
	buf.WriteString("\n")
	return buf.String()
}

func colorEvent(event TestEvent) func(format string, a ...interface{}) string {
	switch event.Action {
	case ActionPass:
		return color.GreenString
	case ActionFail:
		return color.RedString
	case ActionSkip:
		return color.YellowString
	}
	return color.WhiteString
}

// EventFormatter is a function which handles an event and returns a string to
// output for the event.
type EventFormatter interface {
	Format(event TestEvent, output *Execution) error
}

type eventFormatterFunc func(event TestEvent, output *Execution) error

func (e eventFormatterFunc) Format(event TestEvent, output *Execution) error {
	return e(event, output)
}

type FormatOptions struct {
	HideEmptyPackages    bool
	UseHiVisibilityIcons bool
	OutputTestFailures   bool
	OutputWallTime       bool

	// for pkgname-compact format:
	CompactPkgNameFormat string
}

// NewEventFormatter returns a formatter for printing events.
func NewEventFormatter(out io.Writer, format string, formatOpts FormatOptions) EventFormatter {
	switch format {
	case "none":
		return eventFormatterFunc(func(TestEvent, *Execution) error { return nil })
	case "debug":
		return debugFormat(out)
	case "standard-json":
		return standardJSONFormat(out)
	case "standard-verbose":
		return standardVerboseFormat(out)
	case "standard-quiet":
		return standardQuietFormat(out)
	case "dots", "dots-v1":
		return dotsFormatV1(out)
	case "dots-v2":
		return newDotFormatter(out, formatOpts)
	case "testname", "short-verbose":
		return testNameFormat(out, formatOpts)
	case "pkgname", "short":
		return pkgNameFormat(out, formatOpts)
	case "pkgname-and-test-fails", "short-with-failures":
		formatOpts.OutputTestFailures = true
		return pkgNameFormat(out, formatOpts)
	case "pkgname-compact":
		return pkgNameCompactFormat(out, formatOpts)
	case "pkgname-compact2":
		return pkgNameCompactFormat2(out, formatOpts)
	default:
		return nil
	}
}
