package testjson

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"
	"gotest.tools/gotestsum/internal/dotwriter"
)

func dotsFormatV1(event TestEvent, exec *Execution) (string, error) {
	pkg := exec.Package(event.Package)
	switch {
	case event.PackageEvent():
		return "", nil
	case event.Action == ActionRun && pkg.Total == 1:
		return "[" + RelativePackagePath(event.Package) + "]", nil
	}
	return fmtDot(event), nil
}

type dotFormatter struct {
	pkgs      map[string]*dotFmtPkg
	order     []string
	writer    *dotwriter.Writer
	termWidth int
}

type dotFmtPkg struct {
	runes      int
	builder    *strings.Builder
	lastUpdate time.Time
	atMaxWidth bool
}

func (d *dotFmtPkg) update(dot string) {
	if d.atMaxWidth || dot == "" {
		return
	}
	d.builder.WriteString(dot)
	d.runes++
}

func (d *dotFmtPkg) setAtMaxWidth() {
	if !d.atMaxWidth {
		d.builder.WriteString("↲")
		d.atMaxWidth = true
	}
}

func newDotFormatter(out io.Writer) EventFormatter {
	w, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
	return &dotFormatter{
		pkgs:      make(map[string]*dotFmtPkg),
		writer:    dotwriter.New(out),
		termWidth: w,
	}
}

func (d *dotFormatter) Format(event TestEvent, exec *Execution) error {
	if d.pkgs[event.Package] == nil {
		d.pkgs[event.Package] = &dotFmtPkg{builder: new(strings.Builder)}
		d.order = append(d.order, event.Package)
	}
	p := d.pkgs[event.Package]
	p.lastUpdate = event.Time

	if !event.PackageEvent() {
		p.update(fmtDot(event))
	}

	// move the most recently updated packages to the bottom
	sort.Slice(d.order, func(i, j int) bool {
		return d.pkgs[d.order[i]].lastUpdate.Before(d.pkgs[d.order[j]].lastUpdate)
	})
	for _, pkg := range d.order {
		p := d.pkgs[pkg]
		prefix, width := formatPkg(RelativePackagePath(pkg), exec.Package(pkg))
		if width+p.runes+2 >= d.termWidth {
			p.setAtMaxWidth()
		}
		fmt.Fprintf(d.writer, "%s %s\n", prefix, p.builder.String())
	}
	return d.writer.Flush()
}

// TODO: test case for timing format
func formatPkg(pkg string, p *Package) (string, int) {
	elapsed := p.Elapsed()
	var pkgTime string
	switch {
	case p.cached:
		pkgTime = "🖴"
	case elapsed == 0:
	case elapsed < time.Second:
		pkgTime = elapsed.String()
	case elapsed < 10*time.Second:
		pkgTime = elapsed.Truncate(time.Millisecond).String()
	case elapsed < time.Minute:
		pkgTime = elapsed.Truncate(time.Second).String()
	}

	return fmt.Sprintf("%6s %s", pkgTime, pkg), len(pkg) + 7
}

func fmtDot(event TestEvent) string {
	withColor := colorEvent(event)
	switch event.Action {
	case ActionPass:
		return withColor("·")
	case ActionFail:
		return withColor("✖")
	case ActionSkip:
		return withColor("↷")
	}
	return ""
}
