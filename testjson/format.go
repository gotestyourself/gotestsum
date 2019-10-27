package testjson

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// EventFormatter is a function which handles an event and returns a string to
// output for the event.
type EventFormatter func(event TestEvent, output *Execution) (string, error)

func debugFormat(event TestEvent, _ *Execution) (string, error) {
	return fmt.Sprintf("%s %s %s (%.3f) [%d] %s\n",
		event.Package,
		event.Test,
		event.Action,
		event.Elapsed,
		event.Time.Unix(),
		event.Output), nil
}

// go test -v
func standardVerboseFormat(event TestEvent, _ *Execution) (string, error) {
	if event.Action == ActionOutput {
		return event.Output, nil
	}
	return "", nil
}

// go test
func standardQuietFormat(event TestEvent, _ *Execution) (string, error) {
	if !event.PackageEvent() {
		return "", nil
	}
	if event.Output != "PASS\n" && !isCoverageOutput(event.Output) {
		return event.Output, nil
	}
	return "", nil
}

func shortVerboseFormat(event TestEvent, exec *Execution) (string, error) {
	result := colorEvent(event)(strings.ToUpper(string(event.Action)))
	formatTest := func() string {
		pkgPath := RelativePackagePath(event.Package)
		// If the package path isn't the current directory, we add
		// a period to separate the test name and the package path.
		// If it is the current directory, we don't show it at all.
		// This prevents output like ..MyTest when the test
		// is in the current directory.
		if pkgPath == "." {
			pkgPath = ""
		} else {
			pkgPath += "."
		}
		return fmt.Sprintf("%s %s%s %s\n",
			result,
			pkgPath,
			event.Test,
			event.ElapsedFormatted())
	}

	switch {
	case isPkgFailureOutput(event):
		return event.Output, nil

	case event.PackageEvent():
		switch event.Action {
		case ActionSkip:
			result = colorEvent(event)("EMPTY")
			fallthrough
		case ActionPass, ActionFail:
			var cached string
			if exec.Package(event.Package).cached {
				cached = cachedMessage
			}
			return fmt.Sprintf("%s %s%s\n",
				result,
				RelativePackagePath(event.Package),
				cached), nil
		}

	case event.Action == ActionFail:
		return exec.Output(event.Package, event.Test) + formatTest(), nil

	case event.Action == ActionPass:
		return formatTest(), nil

	}
	return "", nil
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
		!strings.HasPrefix(out, "FAIL\t"+event.Package),
		!strings.HasPrefix(out, "ok  \t"+event.Package),
		!strings.HasPrefix(out, "?   \t"+event.Package),
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

const cachedMessage = " (cached)"

func shortFormat(event TestEvent, exec *Execution) (string, error) {
	if !event.PackageEvent() {
		return "", nil
	}
	return shortFormatPackageEvent(event, exec)
}

func shortFormatPackageEvent(event TestEvent, exec *Execution) (string, error) {
	pkg := exec.Package(event.Package)

	fmtElapsed := func() string {
		if pkg.cached {
			return cachedMessage
		}
		d := elapsedDuration(event.Elapsed)
		if d == 0 {
			return ""
		}
		return fmt.Sprintf(" (%s)", d)
	}
	fmtCoverage := func() string {
		if pkg.coverage == "" {
			return ""
		}
		return " (" + pkg.coverage + ")"
	}
	fmtEvent := func(action string) (string, error) {
		return fmt.Sprintf("%s  %s%s%s\n",
			action,
			RelativePackagePath(event.Package),
			fmtElapsed(),
			fmtCoverage(),
		), nil
	}
	withColor := colorEvent(event)
	switch event.Action {
	case ActionSkip:
		return fmtEvent(withColor("∅"))
	case ActionPass:
		return fmtEvent(withColor("✓"))
	case ActionFail:
		return fmtEvent(withColor("✖"))
	}
	return "", nil
}

func shortWithFailuresFormat(event TestEvent, exec *Execution) (string, error) {
	if !event.PackageEvent() {
		if event.Action == ActionFail {
			return exec.Output(event.Package, event.Test), nil
		}
		return "", nil
	}
	return shortFormatPackageEvent(event, exec)
}

func dotsFormat(event TestEvent, exec *Execution) (string, error) {
	pkg := exec.Package(event.Package)
	withColor := colorEvent(event)

	switch {
	case event.PackageEvent():
		return "", nil
	case event.Action == ActionRun && pkg.Total == 1:
		return "[" + RelativePackagePath(event.Package) + "]", nil
	case event.Action == ActionPass:
		return withColor("·"), nil
	case event.Action == ActionFail:
		return withColor("✖"), nil
	case event.Action == ActionSkip:
		return withColor("↷"), nil
	}
	return "", nil
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

// NewEventFormatter returns a formatter for printing events.
func NewEventFormatter(format string) EventFormatter {
	switch format {
	case "debug":
		return debugFormat
	case "standard-verbose":
		return standardVerboseFormat
	case "standard-quiet":
		return standardQuietFormat
	case "dots":
		return dotsFormat
	case "short-verbose":
		return shortVerboseFormat
	case "short":
		return shortFormat
	case "short-with-failures":
		return shortWithFailuresFormat
	default:
		return nil
	}
}
