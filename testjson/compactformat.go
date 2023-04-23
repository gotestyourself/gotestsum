package testjson

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gotest.tools/gotestsum/internal/dotwriter"

	"golang.org/x/term"
)

type PkgTracker struct {
	withWallTime   bool
	lastPkg        string
	col            int
	lineLevel      int
	lineStartLevel int

	pkgs map[string]*pkgLine
}

type pkgLine struct {
	path        string
	event       TestEvent
	lastElapsed time.Duration
}

func shouldJoinPkgs(lastPkg, pkg string) (join bool, commonPrefix string, backStep bool) {
	lastIndex := strings.LastIndex(lastPkg, "/") + 1
	if lastIndex <= len(pkg) && pkg[:lastIndex] == lastPkg[:lastIndex] {
		return true, pkg[:lastIndex], false // note: include the slash
	}
	if lastIndex > 0 {
		nextToLastIndex := strings.LastIndex(lastPkg[:lastIndex-1], "/") + 1
		if nextToLastIndex > 0 && nextToLastIndex <= len(pkg) && pkg[:nextToLastIndex] == lastPkg[:nextToLastIndex] {
			return true, pkg[:nextToLastIndex], true // note: include the slash
		}
	}
	return false, "", false
}

func pkgNameCompactFormat(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	pt := &PkgTracker{withWallTime: opts.OutputWallTime}

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 800
	}

	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && opts.OutputTestFailures {
				if pt.col > 0 {
					buf.WriteString("\n")
				}
				pt.col = 0
				pkg := exec.Package(event.Package)
				tc := pkg.LastFailedByName(event.Test)
				pkg.WriteOutputTo(buf, tc.ID) // nolint:errcheck
				return buf.Flush()
			}
			return nil
		}

		pkgPath := RelativePackagePath(event.Package)

		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			return nil
		}

		pt.writeEventStr(pkgPath, eventStr, event, w, buf, exec.Elapsed())
		return buf.Flush()
	}
}

func (pt *PkgTracker) writeEventStr(pkgPath string, eventStr string, event TestEvent, w int, buf io.StringWriter,
	elapsed time.Duration) {
	join, commonPrefix, backUp := shouldJoinPkgs(pt.lastPkg, pkgPath)
	pt.lastPkg = pkgPath
	if event.Action == ActionFail || pt.col == 0 {
		// put failures and lines after fail output on new lines, to include full package name
		join = false
	}
	if backUp {
		pt.lineLevel--
		if pt.lineLevel <= 0 {
			join = false
		}
	}

	if join {
		pkgShort := strings.TrimPrefix(pkgPath, commonPrefix)
		if backUp {
			pkgShort = "↶" + pkgShort
		}
		eventStrJoin := strings.ReplaceAll(eventStr, pkgPath, pkgShort)
		eventStrJoin = strings.ReplaceAll(eventStrJoin, "  ", " ")
		if pt.col+noColorLen(eventStrJoin) >= w {
			join = false
			if len(commonPrefix) > 0 && !backUp && pt.lineLevel == pt.lineStartLevel {
				eventStr = strings.ReplaceAll(eventStr, pkgPath, "…/"+pkgShort)
			}
		} else {
			eventStr = eventStrJoin
			pt.lineLevel += strings.Count(pkgShort, "/")
		}
	}
	if join {
		buf.WriteString(" ")
		pt.col++
	} else {
		buf.WriteString("\n")
		pt.col = 0
		pt.lineLevel = strings.Count(eventStr, "/")
		pt.lineStartLevel = pt.lineLevel

		if pt.withWallTime {
			eventStr = fmtElapsed(elapsed, false) + eventStr
		}
	}
	pt.col += noColorLen(eventStr)

	buf.WriteString(eventStr) // nolint:errcheck
}

var colorRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func noColorLen(s string) int {
	var wideCount int
	for _, wide := range "➖✅❌" {
		wideCount += strings.Count(s, string(wide))
	}
	return len([]rune(colorRe.ReplaceAllString(s, ""))) + wideCount
}

// ---

func pkgNameCompactFormat2(out io.Writer, opts FormatOptions) eventFormatterFunc {
	pkgTracker := &PkgTracker{
		withWallTime: opts.OutputWallTime,
		pkgs:         map[string]*pkgLine{},
	}

	writer := dotwriter.New(out)

	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && opts.OutputTestFailures {
				pkg := exec.Package(event.Package)
				tc := pkg.LastFailedByName(event.Test)
				// output failures by writing only them to the dotwriter, and then resetting it after those lines.
				failBuf := bufio.NewWriter(writer)
				pkg.WriteOutputTo(failBuf, tc.ID) // nolint:errcheck
				failBuf.Flush()                   // nolint:errcheck
				writer.Flush()                    // nolint:errcheck
				writer = dotwriter.New(out)
				// continue to mark the package as failed early
				//return pkgTracker.flush(writer, opts, exec, withWallTime)
			} else {
				return nil // pkgTracker.flush(writer, opts, exec, withWallTime) // nil
			}
		}

		pkgPath := RelativePackagePath(event.Package)

		// Remove newline from shortFormatPackageEvent
		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			if p := pkgTracker.pkgs[pkgPath]; p != nil {
				p.lastElapsed = exec.Elapsed()
			}
			return nil // pkgTracker.flush(writer, opts, exec, withWallTime) // nil
		}

		pkgTracker.pkgs[pkgPath] = &pkgLine{
			path:        pkgPath,
			event:       event,
			lastElapsed: exec.Elapsed(),
		}
		return pkgTracker.flush(writer, opts, exec)
	}
}

func (pt *PkgTracker) flush(writer *dotwriter.Writer, opts FormatOptions, exec *Execution) error {
	//writer.Write([]byte("\n"))

	var pkgPaths []string // nolint:prealloc
	for pkgPath := range pt.pkgs {
		pkgPaths = append(pkgPaths, pkgPath)
	}
	sort.Strings(pkgPaths)

	// with all packages in order, make a group of each run of packages that can be joined
	groupPkgs := map[string][]*pkgLine{}
	groupPath := ""
	lastPkg := ""
	for _, pkgPath := range pkgPaths {
		join, _, _ := shouldJoinPkgs(lastPkg, pkgPath)
		if !join {
			groupPath = pkgPath
		}
		groupPkgs[groupPath] = append(groupPkgs[groupPath], pt.pkgs[pkgPath])
		lastPkg = pkgPath
	}

	var groupPaths []string // nolint:prealloc
	for groupPath := range groupPkgs {
		groupPaths = append(groupPaths, groupPath)
	}
	sort.Strings(groupPaths)

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 120
	}

	buf := bufio.NewWriter(writer)

	for _, groupPath := range groupPaths {
		pkgs := groupPkgs[groupPath]
		pt.lastPkg = ""
		pt.col = 0
		var elapsed time.Duration
		for _, p := range pkgs {
			if p.lastElapsed > elapsed {
				elapsed = p.lastElapsed
			}
		}
		for _, pkg := range pkgs {
			event := pkg.event
			eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
			pt.writeEventStr(pkg.path, eventStr, event, w, buf, elapsed)
		}
	}
	buf.WriteString("\n")
	buf.Flush() // nolint:errcheck
	PrintSummary(writer, exec, SummarizeNone)
	return writer.Flush()
}
