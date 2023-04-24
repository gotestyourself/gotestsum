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

const CompactFormats = "relative, short, partial, partial-back"

type PkgTracker struct {
	opts    FormatOptions
	lastPkg string
	col     int

	pkgs map[string]*pkgLine
}

type pkgLine struct {
	path        string
	event       TestEvent
	lastElapsed time.Duration
}

func shouldJoinPkgs(opts FormatOptions, lastPkg, pkg string) (join bool, commonPrefix string, backUp int) {
	switch opts.CompactPkgNameFormat {
	case "relative":
		return true, "", 0
	case "short":
		lastIndex := strings.LastIndex(pkg, "/") + 1
		return true, pkg[:lastIndex], 0
	case "partial", "partial-back":
		lastIndex := strings.LastIndex(lastPkg, "/") + 1
		for count := 0; lastIndex > 0; count++ {
			if lastIndex <= len(pkg) && pkg[:lastIndex] == lastPkg[:lastIndex] {
				return true, pkg[:lastIndex], count // note: include the slash
			}
			lastIndex = strings.LastIndex(lastPkg[:lastIndex-1], "/") + 1
		}
		return true, "", 0
	}
	return false, "", 0
}

func pkgNameCompactFormat(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	pt := &PkgTracker{opts: opts}

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 120
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
	initial := pt.lastPkg == ""
	eventStr, join := pt.compactEventStr(pkgPath, eventStr, event, w)
	if join && !initial {
		buf.WriteString(" ") // nolint:errcheck
	} else {
		buf.WriteString("\n") // nolint:errcheck
		if pt.opts.OutputWallTime {
			elapsedStr := fmtElapsed(elapsed, false)
			eventStr = elapsedStr + eventStr
			pt.col += len([]rune(elapsedStr))
		}
	}
	buf.WriteString(eventStr) // nolint:errcheck
}

func (pt *PkgTracker) compactEventStr(pkgPath string, eventStr string, event TestEvent, w int) (string, bool) {
	join, commonPrefix, backUp := shouldJoinPkgs(pt.opts, pt.lastPkg, pkgPath)
	pt.lastPkg = pkgPath
	if event.Action == ActionFail || (pt.opts.CompactPkgNameFormat == "partial" && pt.col == 0) {
		// put failures and lines after fail output on new lines, to include full package name
		join = false
	}

	if join {
		pkgShort := strings.TrimPrefix(pkgPath, commonPrefix)
		if backUp > 0 && pt.opts.CompactPkgNameFormat == "partial-back" {
			pkgShort = "↶" + pkgShort
		}
		eventStrJoin := strings.ReplaceAll(eventStr, pkgPath, pkgShort)
		eventStrJoin = strings.ReplaceAll(eventStrJoin, "  ", " ")
		if pt.col+noColorLen(eventStrJoin) >= w {
			join = false
		}
		eventStr = eventStrJoin
	}
	if join {
		pt.col++
	} else {
		pt.col = 0
	}
	pt.col += noColorLen(eventStr)

	return eventStr, join
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
		opts: opts,
		pkgs: map[string]*pkgLine{},
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
			} else {
				return nil
			}
		}

		pkgPath := RelativePackagePath(event.Package)

		// Remove newline from shortFormatPackageEvent
		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			if p := pkgTracker.pkgs[pkgPath]; p != nil {
				p.lastElapsed = exec.Elapsed()
			}
			return pkgTracker.flush(writer, opts, exec)
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
		join, _, _ := shouldJoinPkgs(pt.opts, lastPkg, pkgPath)
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

	var wallTimeCol int
	if pt.opts.OutputWallTime {
		wallTimeCol = len([]rune(fmtElapsed(time.Second, false)))
	}

	for _, groupPath := range groupPaths {
		pkgs := groupPkgs[groupPath]
		pt.lastPkg = ""
		pt.col = 0
		var elapsed time.Duration
		var parts []string
		flushLine := func() {
			if len(parts) == 0 {
				return
			}
			buf.WriteString("\n")
			if pt.opts.OutputWallTime {
				buf.WriteString(fmtElapsed(elapsed, false))
				elapsed = 0
			}
			buf.WriteString(strings.Join(parts, " "))
			parts = nil
		}
		for i, pkg := range pkgs {
			event := pkg.event
			eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
			compactStr, join := pt.compactEventStr(pkg.path, eventStr, event, w)
			if !join || i == 0 {
				flushLine()
				pt.col += wallTimeCol
			}
			parts = append(parts, compactStr)
			if pkg.lastElapsed > elapsed {
				elapsed = pkg.lastElapsed
			}
		}
		flushLine()
	}
	buf.WriteString("\n")
	buf.Flush() // nolint:errcheck
	PrintSummary(writer, exec, SummarizeNone)
	return writer.Flush()
}
