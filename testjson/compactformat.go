package testjson

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gotest.tools/gotestsum/internal/dotwriter"

	"golang.org/x/term"
)

const CompactFormats = "relative, short, partial, partial-back, -dots, -plain"

func CompactFormatUsage(out io.Writer, name string) {
	fmt.Fprintf(out, `
Formats:
	relative (default)       print the full relative path to the package
	short                    print the last path segment of the package
	partial                  print newly entered path segments for each package
	partial-back             partial with an indication when it backs out
	-dots[N]                 print test dots summary after the package
	-plain                   do not rewrite lines, print each package when finished

`)
}

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
	dots        []string
}

func shouldJoinPkgs(opts FormatOptions, lastPkg, pkg string) (join bool, commonPrefix string, backUp int) {
	pkgNameFormat := dotFmtRe.ReplaceAllString(opts.CompactPkgNameFormat, "")
	pkgNameFormat = strings.TrimSuffix(pkgNameFormat, "-plain")
	switch pkgNameFormat {
	case "relative", "plain", "":
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
	if strings.Contains(opts.CompactPkgNameFormat, "plain") {
		return pkgNameCompactFormatPlain(out, opts)
	}
	return pkgNameCompactFormatDotwriter(out, opts)
}

func pkgNameCompactFormatPlain(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	pt := &PkgTracker{
		opts: opts,
		pkgs: map[string]*pkgLine{},
	}

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 120
	}
	var wallTimeCol int
	if pt.opts.OutputWallTime {
		wallTimeCol = len([]rune(fmtElapsed(time.Second, false)))
	}

	return func(event TestEvent, exec *Execution) error {
		pkgPath := RelativePackagePath(event.Package)

		p := pt.pkgs[pkgPath]
		if p == nil {
			p = &pkgLine{
				path:  pkgPath,
				event: event,
			}
			pt.pkgs[pkgPath] = p
		}

		if !event.PackageEvent() {
			var dot string
			if dotFmtRe.MatchString(opts.CompactPkgNameFormat) {
				dot = fmtDot(event)
				if dot != "" {
					p.dots = append(p.dots, dot)
				}
			}
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

		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			return nil
		}
		eventStr = addPkgDots(p, eventStr, w, wallTimeCol, pt.opts)

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
		if backUp > 0 && strings.Contains(pt.opts.CompactPkgNameFormat, "partial-back") {
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

var dotFmtRe = regexp.MustCompile(`-?dots([0-9]+)?`)
var colorRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func noColorLen(s string) int {
	var wideCount int
	for _, wide := range "➖✅❌" {
		wideCount += strings.Count(s, string(wide))
	}
	return len([]rune(colorRe.ReplaceAllString(s, ""))) + wideCount
}

// ---

func pkgNameCompactFormatDotwriter(out io.Writer, opts FormatOptions) eventFormatterFunc {
	pt := &PkgTracker{
		opts: opts,
		pkgs: map[string]*pkgLine{},
	}

	writer := dotwriter.New(out)
	lastNonEventFlush := time.Now()

	return func(event TestEvent, exec *Execution) error {
		pkgPath := RelativePackagePath(event.Package)

		p := pt.pkgs[pkgPath]
		if p == nil {
			p = &pkgLine{
				path:  pkgPath,
				event: event,
			}
			pt.pkgs[pkgPath] = p
		}

		if !event.PackageEvent() {
			var dot string
			if dotFmtRe.MatchString(opts.CompactPkgNameFormat) {
				dot = fmtDot(event)
				if dot != "" {
					p.dots = append(p.dots, dot)
				}
			}
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
				if dot != "" {
					if time.Since(lastNonEventFlush) < 50*time.Millisecond {
						return nil
					}
					lastNonEventFlush = time.Now()
					return pt.flush(writer, exec)
				}
				return nil
			}
		}
		p.lastElapsed = exec.Elapsed()

		// Remove newline from shortFormatPackageEvent
		eventStr := strings.TrimSuffix(shortFormatPackageEvent(opts, event, exec), "\n")
		if eventStr == "" {
			return pt.flush(writer, exec)
		}

		p.event = event
		return pt.flush(writer, exec)
	}
}

func (pt *PkgTracker) groups() (map[string][]*pkgLine, []string) {
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

	return groupPkgs, groupPaths
}

func (pt *PkgTracker) flush(writer *dotwriter.Writer, exec *Execution) error {
	//writer.Write([]byte("\n"))

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		w = 120
	}
	var wallTimeCol int
	if pt.opts.OutputWallTime {
		wallTimeCol = len([]rune(fmtElapsed(time.Second, false)))
	}

	buf := bufio.NewWriter(writer)

	groupPkgs, groupPaths := pt.groups()
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
			eventStr := strings.TrimSuffix(shortFormatPackageEvent(pt.opts, event, exec), "\n")
			eventStr = addPkgDots(pkg, eventStr, w, wallTimeCol, pt.opts)
			compactStr, join := pt.compactEventStr(pkg.path, eventStr, event, w)
			if !join || i == 0 {
				flushLine()
				pt.col += wallTimeCol
			}
			if compactStr != "" {
				parts = append(parts, compactStr)
			}
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

func addPkgDots(pkg *pkgLine, eventStr string, w int, wallTimeCol int, opts FormatOptions) string {
	dotFmt := dotFmtRe.FindString(opts.CompactPkgNameFormat)
	if len(pkg.dots) > 0 {
		if eventStr == "" {
			eventStr = pkg.path
		}
		// TODO: maxDots should consider shortened package names from compactEventStr() but without updating pt.col
		maxDots := w - wallTimeCol - noColorLen(eventStr)
		eventStr += dotSummary(pkg.dots, dotFmt, maxDots)
	}
	return eventStr
}

func dotSummary(dots []string, dotFmt string, maxLen int) string {
	var limit = 1
	if nstr := strings.TrimLeft(dotFmt, "-dots"); nstr != "" {
		if n, err := strconv.Atoi(nstr); err == nil {
			limit = n
		}
	}
	margin := 11
	if maxLen > margin && limit+margin > maxLen {
		limit = maxLen - margin
	}
	if len(dots) > limit {
		sort.Strings(dots)
	}
	var s string
	var prev string
	var count int
	add := func() {
		if count == 0 {
			return
		}
		if count <= limit+3 {
			s += strings.Repeat(prev, count)
		} else {
			if s == "" {
				s += strings.Repeat(prev, limit-1)
			}
			reset := "\x1b[0m"
			if !strings.Contains(prev, reset) {
				reset = ""
			}
			s += fmt.Sprintf("%s[%d]%s", strings.TrimSuffix(prev, reset), count, reset)
		}
		limit = 1
	}
	for _, dot := range dots {
		if dot != prev {
			add()
			prev = dot
			count = 0
		}
		count++
	}
	add()
	return s
}
