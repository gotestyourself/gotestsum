package testjson

import (
	"bufio"
	"fmt"
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
	lastPkg string
	col     int

	startTime time.Time
	pkgs      map[string]*pkgLine
}

type pkgLine struct {
	pkg        string
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

func isEmpty(event TestEvent, exec *Execution) bool {
	pkg := exec.Package(event.Package)
	switch event.Action {
	case ActionSkip:
		return true
	case ActionPass:
		if pkg.Total == 0 {
			return true
		}
	}
	return false
}

var colorRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func multiPkgNameFormat(out io.Writer, opts FormatOptions, withFailures, withWallTime bool) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	pkgTracker := &PkgTracker{startTime: time.Now()}

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 800
	}

	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail && withFailures {
				if pkgTracker.col > 0 {
					buf.WriteString("\n")
				}
				pkgTracker.col = 0
				pkg := exec.Package(event.Package)
				tc := pkg.LastFailedByName(event.Test)
				pkg.WriteOutputTo(buf, tc.ID) // nolint:errcheck
				return buf.Flush()
			}
			return nil
		}

		// Remove newline from shortFormatPackageEvent
		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			return nil
		}
		//eventStr = fmt.Sprintf("%d/%d %s", pkgTracker.col, w, eventStr)

		pkgPath := RelativePackagePath(event.Package)
		join, commonPrefix := shouldJoinPkgs(pkgTracker.lastPkg, pkgPath)
		if event.Action == ActionFail {
			join = false
		}

		if join {
			eventStrJoin := strings.ReplaceAll(eventStr, commonPrefix, "")
			eventStrJoin = strings.ReplaceAll(eventStrJoin, "  ", " ")
			noColorJoinStr := colorRe.ReplaceAllString(eventStr, "")
			if pkgTracker.col == 0 || pkgTracker.col+len([]rune(noColorJoinStr)) >= w {
				join = false
				eventStr = strings.ReplaceAll(eventStr, commonPrefix, "…/")
			} else {
				eventStr = eventStrJoin
			}
		}
		if join {
			buf.WriteString(" ")
			pkgTracker.col++
		} else {
			buf.WriteString("\n")
			pkgTracker.col = 0

			if withWallTime {
				t := time.Since(pkgTracker.startTime).Round(time.Millisecond)
				eventStr = fmt.Sprintf("%.3fs %s", float64(t.Milliseconds())/1000, eventStr)
			}
		}
		pkgTracker.lastPkg = pkgPath
		noColorStr := colorRe.ReplaceAllString(eventStr, "")
		pkgTracker.col += len([]rune(noColorStr))

		buf.WriteString(eventStr) // nolint:errcheck
		return buf.Flush()
	}
}

// ---

func multiPkgNameFormat2(out io.Writer, opts FormatOptions, withFailures, withWallTime bool) eventFormatterFunc {
	pkgTracker := &PkgTracker{
		startTime: time.Now(),
		pkgs:      map[string]*pkgLine{},
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
				writer.Flush()
				writer = dotwriter.New(out)
				// continue to mark the package as failed early
				//return pkgTracker.flush(writer, opts, exec, withWallTime)
			} else {
				return nil // pkgTracker.flush(writer, opts, exec, withWallTime) // nil
			}
		}

		// Remove newline from shortFormatPackageEvent
		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			return nil // pkgTracker.flush(writer, opts, exec, withWallTime) // nil
		}

		pkgPath := RelativePackagePath(event.Package)
		pkgTracker.pkgs[pkgPath] = &pkgLine{
			pkg:        pkgPath,
			event:      event,
			lastUpdate: time.Now(),
		}
		return pkgTracker.flush(writer, opts, exec, withWallTime)
	}
}

func (pt *PkgTracker) flush(writer *dotwriter.Writer, opts FormatOptions, exec *Execution,
	withWallTime bool) error {
	writer.Write([]byte("\n\n"))

	var pkgPaths []string
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

	var groupPaths []string
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
		lastPkg := ""
		col := 0
		for pkgI, pkg := range pkgs {
			join, commonPrefix := shouldJoinPkgs(lastPkg, pkg.pkg)
			lastPkg = pkg.pkg
			if pkg.event.Action == ActionFail {
				// put failures on new lines, to include full package name
				join = false
			}

			eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, pkg.event, exec), "\n")
			if join {
				eventStrJoin := strings.ReplaceAll(eventStr, commonPrefix, "")
				eventStrJoin = strings.ReplaceAll(eventStrJoin, "  ", " ")
				noColorJoinStr := colorRe.ReplaceAllString(eventStr, "")
				if col+len([]rune(noColorJoinStr)) >= w {
					join = false
					eventStr = strings.ReplaceAll(eventStr, commonPrefix, "…/")
				} else {
					eventStr = eventStrJoin
				}
			}
			if join {
				buf.WriteString(" ")
				col++
			} else {
				if pkgI > 0 {
					buf.WriteString("\n")
				}
				col = 0

				if withWallTime {
					var lastUpdatePkg *pkgLine
					for _, p := range pkgs {
						if lastUpdatePkg == nil || p.lastUpdate.After(lastUpdatePkg.lastUpdate) {
							lastUpdatePkg = p
						}
					}
					eventStr = fmtDotElapsed(exec.Package(lastUpdatePkg.event.Package)) + eventStr
				}
			}
			noColorStr := colorRe.ReplaceAllString(eventStr, "")
			col += len([]rune(noColorStr))

			buf.WriteString(eventStr) // nolint:errcheck
		}
		buf.WriteString("\n")
	}
	buf.Flush() // nolint:errcheck
	PrintSummary(writer, exec, SummarizeNone)
	return writer.Flush()
}
