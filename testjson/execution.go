package testjson

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Action of TestEvent
type Action string

// nolint: unused
const (
	ActionRun    Action = "run"
	ActionPause  Action = "pause"
	ActionCont   Action = "cont"
	ActionPass   Action = "pass"
	ActionBench  Action = "bench"
	ActionFail   Action = "fail"
	ActionOutput Action = "output"
	ActionSkip   Action = "skip"
)

// TestEvent is a structure output by go tool test2json and go test -json.
type TestEvent struct {
	// Time encoded as an RFC3339-format string
	Time    time.Time
	Action  Action
	Package string
	Test    string
	// Elapsed time in seconds
	Elapsed float64
	// Output of test or benchmark
	Output string
	// raw is the raw JSON bytes of the event
	raw []byte
}

// PackageEvent returns true if the event is a package start or end event
func (e TestEvent) PackageEvent() bool {
	return e.Test == ""
}

// ElapsedFormatted returns Elapsed formatted in the go test format, ex (0.00s).
func (e TestEvent) ElapsedFormatted() string {
	return fmt.Sprintf("(%.2fs)", e.Elapsed)
}

// Bytes returns the serialized JSON bytes that were parsed to create the event.
func (e TestEvent) Bytes() []byte {
	return e.raw
}

// Package is the set of TestEvents for a single go package
type Package struct {
	// TODO: this could be Total()
	Total   int
	Failed  []TestCase
	Skipped []TestCase
	Passed  []TestCase
	output  map[string][]string
	// action identifies if the package passed or failed. A package may fail
	// with no test failures if an init() or TestMain exits non-zero.
	// skip indicates there were no tests.
	action Action
}

// Result returns if the package passed, failed, or was skipped because there
// were no tests.
func (p Package) Result() Action {
	return p.action
}

// Elapsed returns the sum of the elapsed time for all tests in the package.
func (p Package) Elapsed() time.Duration {
	elapsed := time.Duration(0)
	for _, testcase := range p.TestCases() {
		elapsed = elapsed + testcase.Elapsed
	}
	return elapsed
}

// TestCases returns all the test cases.
func (p Package) TestCases() []TestCase {
	return append(append(p.Passed, p.Failed...), p.Skipped...)
}

// Output returns the full test output for a test.
func (p Package) Output(test string) string {
	return strings.Join(p.output[test], "")
}

// TestMainFailed returns true if the package failed, but there were no tests.
// This may occur if the package init() or TestMain exited non-zero.
func (p Package) TestMainFailed() bool {
	return p.action == ActionFail && len(p.Failed) == 0
}

// TestCase stores the name and elapsed time for a test case.
type TestCase struct {
	Package string
	Test    string
	Elapsed time.Duration
}

func newPackage() *Package {
	return &Package{output: make(map[string][]string)}
}

// Execution of one or more test packages
type Execution struct {
	started  time.Time
	packages map[string]*Package
	errors   []string
}

func (e *Execution) add(event TestEvent) {
	pkg, ok := e.packages[event.Package]
	if !ok {
		pkg = newPackage()
		e.packages[event.Package] = pkg
	}
	if event.PackageEvent() {
		switch event.Action {
		case ActionPass, ActionFail:
			pkg.action = event.Action
		case ActionOutput:
			pkg.output[""] = append(pkg.output[""], event.Output)
		}
		return
	}

	switch event.Action {
	case ActionRun:
		pkg.Total++
	case ActionFail:
		pkg.Failed = append(pkg.Failed, TestCase{
			Package: event.Package,
			Test:    event.Test,
			Elapsed: elapsedDuration(event.Elapsed),
		})
	case ActionSkip:
		pkg.Skipped = append(pkg.Skipped, TestCase{
			Package: event.Package,
			Test:    event.Test,
			Elapsed: elapsedDuration(event.Elapsed),
		})
	case ActionOutput, ActionBench:
		// TODO: limit size of buffered test output
		pkg.output[event.Test] = append(pkg.output[event.Test], event.Output)
	case ActionPass:
		pkg.Passed = append(pkg.Passed, TestCase{
			Package: event.Package,
			Test:    event.Test,
			Elapsed: elapsedDuration(event.Elapsed),
		})
		// Remove test output once a test passes, it wont be used
		pkg.output[event.Test] = nil
	}
}

func elapsedDuration(elapsed float64) time.Duration {
	return time.Duration(elapsed*1000) * time.Millisecond
}

// Output returns the full test output for a test.
func (e *Execution) Output(pkg, test string) string {
	return strings.Join(e.packages[pkg].output[test], "")
}

// OutputLines returns the full test output for a test as an array of lines.
func (e *Execution) OutputLines(pkg, test string) []string {
	return e.packages[pkg].output[test]
}

// Package returns the Package by name.
func (e *Execution) Package(name string) *Package {
	return e.packages[name]
}

// Packages returns a sorted list of all package names.
func (e *Execution) Packages() []string {
	return sortedKeys(e.packages)
}

var clock = clockwork.NewRealClock()

// Elapsed returns the time elapsed since the execution started.
func (e *Execution) Elapsed() time.Duration {
	return clock.Now().Sub(e.started)
}

// Failed returns a list of all the failed test cases.
func (e *Execution) Failed() []TestCase {
	var failed []TestCase
	for _, name := range sortedKeys(e.packages) {
		pkg := e.packages[name]

		// Add package-level failure output if there were no failed tests.
		if pkg.TestMainFailed() {
			failed = append(failed, TestCase{Package: name})
		} else {
			failed = append(failed, pkg.Failed...)
		}
	}
	return failed
}

func sortedKeys(pkgs map[string]*Package) []string {
	var keys []string
	for key := range pkgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Skipped returns a list of all the skipped test cases.
func (e *Execution) Skipped() []TestCase {
	var skipped []TestCase
	for _, pkg := range sortedKeys(e.packages) {
		skipped = append(skipped, e.packages[pkg].Skipped...)
	}
	return skipped
}

// Total returns a count of all test cases.
func (e *Execution) Total() int {
	total := 0
	for _, pkg := range e.packages {
		total += pkg.Total
	}
	return total
}

func (e *Execution) addError(err string) {
	// Build errors start with a header
	if strings.HasPrefix(err, "# ") {
		return
	}
	// TODO: may need locking, or use a channel
	e.errors = append(e.errors, err)
}

// Errors returns a list of all the errors.
func (e *Execution) Errors() []string {
	return e.errors
}

// NewExecution returns a new Execution and records the current time as the
// time the test execution started.
func NewExecution() *Execution {
	return &Execution{
		started:  time.Now(),
		packages: make(map[string]*Package),
	}
}

// ScanConfig used by ScanTestOutput
type ScanConfig struct {
	Stdout  io.Reader
	Stderr  io.Reader
	Handler EventHandler
}

// EventHandler is called by ScanTestOutput for each event and write to stderr.
type EventHandler interface {
	Event(event TestEvent, execution *Execution) error
	Err(text string) error
}

// ScanTestOutput reads lines from stdout and stderr, creates an Execution,
// calls the Handler for each event, and returns the Execution.
func ScanTestOutput(config ScanConfig) (*Execution, error) {
	execution := NewExecution()
	waitOnStderr := readStderr(config.Stderr, config.Handler.Err, execution)
	scanner := bufio.NewScanner(config.Stdout)

	for scanner.Scan() {
		raw := scanner.Bytes()
		event, err := parseEvent(raw)
		switch err {
		case errBadEvent:
			// TODO: put raw into errors.
			continue
		case nil:
		default:
			return nil, errors.Wrapf(err, "failed to parse test output: %s", string(raw))
		}
		execution.add(event)
		if err := config.Handler.Event(event, execution); err != nil {
			return nil, err
		}
	}

	// TODO: this is not reached if pareseEvent or Handler.Event returns an error
	if err := <-waitOnStderr; err != nil {
		logrus.Warnf("failed reading stderr: %s", err)
	}
	return execution, errors.Wrap(scanner.Err(), "failed to scan test output")
}

type errHandler func(text string) error

func readStderr(in io.Reader, handle errHandler, exec *Execution) chan error {
	wait := make(chan error, 1)
	go func() {
		defer close(wait)
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			// TODO: remove this check if go module events stop being output as stdErr
			if checkIsGoModuleEvent(scanner.Text()) {
				continue
			}

			exec.addError(scanner.Text())
			if err := handle(scanner.Text()); err != nil {
				wait <- err
				return
			}
		}
		wait <- scanner.Err()
	}()
	return wait
}

func checkIsGoModuleEvent(scannerText string) bool {
	prefixes := [2]string{"go: extracting", "go: downloading"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(scannerText, prefix) {
			return true
		}
	}
	return false
}

func parseEvent(raw []byte) (TestEvent, error) {
	// TODO: this seems to be a bug in the `go test -json` output
	if bytes.HasPrefix(raw, []byte("FAIL")) {
		logrus.Warn(string(raw))
		return TestEvent{}, errBadEvent
	}

	event := TestEvent{}
	err := json.Unmarshal(raw, &event)
	event.raw = raw
	return event, err
}

var errBadEvent = errors.New("bad output from test2json")
