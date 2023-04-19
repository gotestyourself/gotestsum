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
	withWallTime bool
	lastPkg      string
	col          int

	startTime time.Time
	pkgs      map[string]*pkgLine
}

type pkgLine struct {
	path       string
	event      TestEvent
	lastUpdate time.Time
}

func shouldJoinPkgs(lastPkg, pkg string) (join bool, commonPrefix string) {
	lastIndex := strings.LastIndex(lastPkg, "/")
	if lastIndex < 0 || lastIndex+1 > len(pkg) {
		return false, ""
	}

	if pkg[:lastIndex] == lastPkg[:lastIndex] {
		return true, pkg[:lastIndex+1] // note: include the slash
	}
	return false, ""
}

var colorRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func multiPkgNameFormat(out io.Writer, opts FormatOptions, withFailures, withWallTime bool) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	pt := &PkgTracker{startTime: time.Now(), withWallTime: withWallTime}

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 800
	}

	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && withFailures {
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

		elapsed := time.Since(pt.startTime).Round(time.Millisecond)
		pt.writeEventStr(pkgPath, eventStr, event, w, buf, elapsed)
		return buf.Flush()
	}
}

func (pt *PkgTracker) writeEventStr(pkgPath string, eventStr string, event TestEvent, w int, buf io.StringWriter,
	elapsed time.Duration) {
	join, commonPrefix := shouldJoinPkgs(pt.lastPkg, pkgPath)
	pt.lastPkg = pkgPath
	if event.Action == ActionFail || pt.col == 0 {
		// put failures and lines after fail output on new lines, to include full package name
		join = false
	}

	if join {
		eventStrJoin := strings.ReplaceAll(eventStr, commonPrefix, "")
		eventStrJoin = strings.ReplaceAll(eventStrJoin, "  ", " ")
		noColorJoinStr := colorRe.ReplaceAllString(eventStr, "")
		if pt.col+len([]rune(noColorJoinStr)) >= w {
			join = false
			eventStr = strings.ReplaceAll(eventStr, commonPrefix, "…/")
		} else {
			eventStr = eventStrJoin
		}
	}
	if join {
		buf.WriteString(" ")
		pt.col++
	} else {
		buf.WriteString("\n")
		pt.col = 0

		if pt.withWallTime {
			eventStr = fmtElapsed(elapsed, false) + eventStr
		}
	}
	noColorStr := colorRe.ReplaceAllString(eventStr, "")
	pt.col += len([]rune(noColorStr))

	buf.WriteString(eventStr) // nolint:errcheck
}

// ---

func multiPkgNameFormat2(out io.Writer, opts FormatOptions, withFailures, withWallTime bool) eventFormatterFunc {
	pkgTracker := &PkgTracker{
		startTime:    time.Now(),
		withWallTime: withWallTime,
		pkgs:         map[string]*pkgLine{},
	}

	writer := dotwriter.New(out)

	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && withFailures {
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
				p.lastUpdate = time.Now()
			}
			return nil // pkgTracker.flush(writer, opts, exec, withWallTime) // nil
		}

		pkgTracker.pkgs[pkgPath] = &pkgLine{
			path:       pkgPath,
			event:      event,
			lastUpdate: time.Now(),
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
		join, _ := shouldJoinPkgs(lastPkg, pkgPath)
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
		w = 800
	}

	buf := bufio.NewWriter(writer)

	for _, groupPath := range groupPaths {
		pkgs := groupPkgs[groupPath]
		pt.lastPkg = ""
		pt.col = 0
		for _, pkg := range pkgs {
			var lastUpdatePkg *pkgLine
			for _, p := range pkgs {
				if lastUpdatePkg == nil || p.lastUpdate.After(lastUpdatePkg.lastUpdate) {
					lastUpdatePkg = p
				}
			}
			elapsed := lastUpdatePkg.lastUpdate.Sub(pt.startTime).Round(time.Millisecond)

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
