package testjson

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"gotest.tools/gotestsum/internal/dotwriter"
	"gotest.tools/gotestsum/internal/log"
)

func dotsFormatV1(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	// nolint:errcheck
	return eventFormatterFunc(func(event TestEvent, exec *Execution) error {
		pkg := exec.Package(event.Package)
		switch {
		case event.PackageEvent():
			return nil
		case event.Action == ActionRun && pkg.Total == 1:
			buf.WriteString("[" + RelativePackagePath(event.Package) + "]")
			return buf.Flush()
		}
		buf.WriteString(fmtDot(event))
		return buf.Flush()
	})
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

type dotFormatter struct {
	pkgs       map[string]*dotLine
	order      []string
	writer     *dotwriter.Writer
	opts       FormatOptions
	termWidth  int
	termHeight int
	stop       chan struct{}
	flushed    chan struct{}
	mu         sync.RWMutex
	last       string
	summary    []byte
}

type dotLine struct {
	runes      int
	builder    *strings.Builder
	lastUpdate time.Time
	out        string
	empty      bool
}

func (l *dotLine) update(dot string) {
	if dot == "" {
		return
	}
	if l.runes == -1 { // Stop once we hit max length. TODO: add back line wrapping.
		return
	}
	l.builder.WriteString(dot)
	l.runes++
}

// checkWidth marks the line as full when the width of the line hits the
// terminal width.
func (l *dotLine) checkWidth(prefix, terminal int) {
	if prefix+l.runes >= terminal-1 {
		l.runes = -1
	}
}

func newDotFormatter(out io.Writer, opts FormatOptions) EventFormatter {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		log.Warnf("Failed to detect terminal width for dots format, error: %v", err)
		return dotsFormatV1(out)
	}
	f := &dotFormatter{
		pkgs:       make(map[string]*dotLine),
		writer:     dotwriter.New(out),
		termWidth:  w,
		termHeight: h - 10,
		opts:       opts,
		stop:       make(chan struct{}),
		flushed:    make(chan struct{}),
	}
	go f.runWriter()
	return f
}

func (d *dotFormatter) Close() error {
	close(d.stop)
	<-d.flushed // Wait until we write the last data
	return nil
}

func (d *dotFormatter) Format(event TestEvent, exec *Execution) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pkgs[event.Package] == nil {
		d.pkgs[event.Package] = &dotLine{builder: new(strings.Builder)}
		d.order = append(d.order, event.Package)
	}
	line := d.pkgs[event.Package]
	line.lastUpdate = event.Time

	if !event.PackageEvent() {
		line.update(fmtDot(event))
	}
	pkg := exec.Package(event.Package)

	pkgname := RelativePackagePath(event.Package) + " "
	prefix := fmtDotElapsed(pkg)
	line.checkWidth(len(prefix+pkgname), d.termWidth)
	line.checkWidth(len(prefix+pkgname), d.termWidth)
	line.out = prefix + pkgname + line.builder.String()

	line.empty = pkg.IsEmpty()
	buf := bytes.Buffer{}
	PrintSummary(&buf, exec, SummarizeNone)
	d.summary = buf.Bytes()

	return nil
}

func (d *dotFormatter) runWriter() {
	t := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-d.stop:
			if err := d.write(); err != nil {
				log.Warnf("failed to write: %v", err)
			}
			close(d.flushed)
			return
		case <-t.C:
			if err := d.write(); err != nil {
				log.Warnf("failed to write: %v", err)
			}
		}
	}
}

func (d *dotFormatter) write() error {
	d.mu.RLock() // TODO: lock is not sufficient, we need to read from d.exec in the event handler.
	defer d.mu.RUnlock()

	// TODO summary time should update on each iteration ideally. Although that drops our "skip" optimization
	summaryLines := strings.Split(string(d.summary), "\n")

	lines := []string{}
	for _, pkg := range d.order {
		line := d.pkgs[pkg]
		if d.opts.HideEmptyPackages && line.empty {
			continue
		}

		lines = append(lines, line.out)
	}
	maxTestLines := d.termHeight - len(summaryLines)
	if len(lines) > maxTestLines {
		// Pick the last lines
		lines = lines[len(lines)-maxTestLines:]
	}
	lines = append(lines, summaryLines...)
	res := strings.Join(lines, "\n")
	if res == d.last {
		return nil
	}
	d.last = res

	// Write empty lines for some padding
	fmt.Fprint(d.writer, "\n")
	d.writer.Write([]byte(res))

	return d.writer.Flush()
}

func fmtDotElapsed(p *Package) string {
	f := func(v string) string {
		c := colorEvent(TestEvent{Action: p.Result()})
		act := c("⏱️")
		switch p.Result() {
		case ActionPass:
			act = c("✔️")
		case ActionFail:
			act = c("✖")
		case ActionSkip:
			act = c("↷")
		}
		return fmt.Sprintf(" %5s %v ", v, act)
	}

	elapsed := p.Elapsed()
	switch {
	case p.cached:
		return f("🖴 ")
	case elapsed <= 0:
		return f("")
	case elapsed >= time.Hour:
		return f("⏳ ")
	case elapsed < time.Second:
		return f(elapsed.String())
	}

	const maxWidth = 7
	steps := []time.Duration{
		time.Millisecond,
		10 * time.Millisecond,
		100 * time.Millisecond,
		time.Second,
		10 * time.Second,
		time.Minute,
		10 * time.Minute,
	}

	for _, trunc := range steps {
		r := f(elapsed.Truncate(trunc).String())
		if len(r) <= maxWidth {
			return r
		}
	}
	return f("")
}
