package testjson

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
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
	if event.PackageEvent() && event.Output != "PASS\n" {
		return event.Output, nil
	}
	return "", nil
}

func shortVerboseFormat(event TestEvent, exec *Execution) (string, error) {
	switch {
	case isPkgFailureOutput(event):
		return event.Output, nil
	// TODO: share more code with shortFormat() for these PackageEvent cases
	case event.Action == ActionSkip && event.PackageEvent():
		return "EMPTY " + relativePackagePath(event.Package) + "\n", nil
	case event.Action == ActionPass && event.PackageEvent():
		return "PASS " + relativePackagePath(event.Package) + "\n", nil
	case event.Action == ActionFail && event.PackageEvent():
		return "FAIL " + relativePackagePath(event.Package) + "\n", nil
	case event.Action == ActionPass:
		return fmt.Sprintf("--- PASS %s %s %s\n",
			relativePackagePath(event.Package),
			event.Test,
			event.ElapsedFormatted(),
		), nil
	case event.Action == ActionFail:
		return fmt.Sprintf("%s--- FAIL %s %s %s\n",
			strings.Join(exec.Output(event.Package, event.Test), ""),
			relativePackagePath(event.Package),
			event.Test,
			event.ElapsedFormatted(),
		), nil
	}
	return "", nil
}

func isPkgFailureOutput(event TestEvent) bool {
	out := event.Output
	return all(
		event.PackageEvent(),
		event.Action == ActionOutput,
		out != "PASS\n",
		out != "FAIL\n",
		!strings.HasPrefix(out, "FAIL\t"+event.Package),
		!strings.HasPrefix(out, "ok  \t"+event.Package),
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

func shortFormat(event TestEvent, _ *Execution) (string, error) {
	if !event.PackageEvent() {
		return "", nil
	}
	fmtElapsed := func() string {
		d := elapsedDuration(event)
		if d == 0 {
			return ""
		}
		return fmt.Sprintf(" (%s)", d)
	}
	fmtEvent := func(action string) (string, error) {
		return fmt.Sprintf("%s  %s%s\n",
			action, relativePackagePath(event.Package), fmtElapsed()), nil
	}
	switch event.Action {
	case ActionSkip:
		return fmtEvent("∅")
	case ActionPass:
		return fmtEvent("✓")
	case ActionFail:
		return fmtEvent("✖")
	}
	return "", nil
}

func dotsFormat(event TestEvent, exec *Execution) (string, error) {
	pkg := exec.Package(event)

	switch {
	case event.PackageEvent():
		return "", nil
	case event.Action == ActionRun && pkg.run == 1:
		return "[" + relativePackagePath(event.Package) + "]", nil
	case event.Action == ActionPass:
		return "·", nil
	case event.Action == ActionFail:
		return "✖", nil
	case event.Action == ActionSkip:
		return "↷", nil
	}
	return "", nil
}

func relativePackagePath(pkgpath string) string {
	if pkgpath == pkgPathPrefix {
		return "."
	}
	return strings.TrimPrefix(pkgpath, pkgPathPrefix+"/")
}

// TODO: might not work on windows
func getPkgPathPrefix() string {
	cwd, _ := os.Getwd()
	gopaths := strings.Split(build.Default.GOPATH, string(filepath.ListSeparator))
	for _, gopath := range gopaths {
		gosrcpath := gopath + "/src/"
		if strings.HasPrefix(cwd, gosrcpath) {
			return strings.TrimPrefix(cwd, gosrcpath)
		}
	}
	return ""
}

var pkgPathPrefix = getPkgPathPrefix()

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
	default:
		return nil
	}
}
