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
	"golang.org/x/sync/errgroup"
	"gotest.tools/gotestsum/log"
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
	running map[string]TestCase
	Failed  []TestCase
	Skipped []TestCase
	Passed  []TestCase
	// output printed by test cases. Output is stored first by root TestCase
	// name, then by subtest name to mitigate github.com/golang/go/issues/29755.
	// In the future when that bug is fixed this can be reverted to store all
	// output by full test name.
	output map[string]map[string][]string
	// coverage stores the code coverage output for the package without the
	// trailing newline (ex: coverage: 91.1% of statements).
	coverage string
	// action identifies if the package passed or failed. A package may fail
	// with no test failures if an init() or TestMain exits non-zero.
	// skip indicates there were no tests.
	action Action
	// cached is true if the package was marked as (cached)
	cached bool
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
		elapsed += testcase.Elapsed
	}
	return elapsed
}

// TestCases returns all the test cases.
func (p Package) TestCases() []TestCase {
	tc := append([]TestCase{}, p.Passed...)
	tc = append(tc, p.Failed...)
	tc = append(tc, p.Skipped...)
	return tc
}

// Output returns the full test output for a test.
//
// Unlike OutputLines() it does not return any extra lines in some cases.
func (p Package) Output(test string) string {
	root, sub := splitTestName(test)
	return strings.Join(p.output[root][sub], "")
}

// OutputLines returns the full test output for a test as a slice of strings.
//
// As a workaround for test output being attributed to the wrong subtest, if:
//   - the TestCase is a root TestCase (not a subtest), and
//   - the TestCase has no subtest failures;
// then all output for every subtest under the root test is returned.
// See https://github.com/golang/go/issues/29755.
func (p Package) OutputLines(tc TestCase) []string {
	root, sub := splitTestName(tc.Test)
	lines := p.output[root][sub]

	// If this is a subtest, or a root test case with subtest failures the
	// subtest failure output should contain the relevant lines, so we don't need
	// extra lines.
	if sub != "" || tc.subTestFailed {
		return lines
	}
	//
	result := make([]string, 0, len(p.output[root])*2)
	for _, sub := range testNamesSorted(p.output[root]) {
		result = append(result, p.output[root][sub]...)
	}
	return result
}

func testNamesSorted(subs map[string][]string) []string {
	names := make([]string, 0, len(subs))
	for name := range subs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (p Package) addOutput(test string, output string) {
	root, sub := splitTestName(test)
	if p.output[root] == nil {
		p.output[root] = make(map[string][]string)
	}
	// TODO: limit size of buffered test output
	p.output[root][sub] = append(p.output[root][sub], output)
}

// splitTestName into root test name and any subtest names.
func splitTestName(name string) (root, sub string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return name, ""
	}
	return parts[0], parts[1]
}

// TestMainFailed returns true if the package failed, but there were no tests.
// This may occur if the package init() or TestMain exited non-zero.
func (p Package) TestMainFailed() bool {
	return p.action == ActionFail && len(p.Failed) == 0
}

const neverFinished time.Duration = -1

func (p *Package) end() {
	// Add tests that were missing an ActionFail event to Failed
	for _, tc := range p.running {
		tc.Elapsed = neverFinished
		p.Failed = append(p.Failed, tc)
	}
}

// TestCase stores the name and elapsed time for a test case.
type TestCase struct {
	Package string
	Test    string
	Elapsed time.Duration
	// subTestFailed is true when a subtest of this TestCase has failed. It is
	// used to find root TestCases which have no failing subtests.
	subTestFailed bool
}

func newPackage() *Package {
	return &Package{
		output:  make(map[string]map[string][]string),
		running: make(map[string]TestCase),
	}
}

// Execution of one or more test packages
type Execution struct {
	started  time.Time
	packages map[string]*Package
	errors   []string
	done     bool
}

func (e *Execution) add(event TestEvent) {
	pkg, ok := e.packages[event.Package]
	if !ok {
		pkg = newPackage()
		e.packages[event.Package] = pkg
	}
	if event.PackageEvent() {
		e.addPackageEvent(pkg, event)
		return
	}
	e.addTestEvent(pkg, event)
}

func (e *Execution) addPackageEvent(pkg *Package, event TestEvent) {
	switch event.Action {
	case ActionPass, ActionFail:
		pkg.action = event.Action
	case ActionOutput:
		if isCoverageOutput(event.Output) {
			pkg.coverage = strings.TrimRight(event.Output, "\n")
		}
		if isCachedOutput(event.Output) {
			pkg.cached = true
		}
		pkg.addOutput("", event.Output)
	}
}

func (e *Execution) addTestEvent(pkg *Package, event TestEvent) {
	switch event.Action {
	case ActionRun:
		pkg.Total++
		pkg.running[event.Test] = TestCase{
			Package: event.Package,
			Test:    event.Test,
		}
		return
	case ActionOutput, ActionBench:
		pkg.addOutput(event.Test, event.Output)
		return
	case ActionPause, ActionCont:
		return
	}

	tc := pkg.running[event.Test]
	delete(pkg.running, event.Test)
	tc.Elapsed = elapsedDuration(event.Elapsed)

	switch event.Action {
	case ActionFail:
		pkg.Failed = append(pkg.Failed, tc)

		// If this is a subtest, mark the root test as having subtests.
		root, subTest := splitTestName(event.Test)
		if subTest != "" {
			rootTestCase := pkg.running[root]
			rootTestCase.subTestFailed = true
			pkg.running[root] = rootTestCase
		}
	case ActionSkip:
		pkg.Skipped = append(pkg.Skipped, tc)
	case ActionPass:
		pkg.Passed = append(pkg.Passed, tc)
		// Remove test output once a test passes, it wont be used
		delete(pkg.output, event.Test)
	}
}

func elapsedDuration(elapsed float64) time.Duration {
	return time.Duration(elapsed*1000) * time.Millisecond
}

func isCoverageOutput(output string) bool {
	return all(
		strings.HasPrefix(output, "coverage:"),
		strings.HasSuffix(output, "% of statements\n"))
}

func isCachedOutput(output string) bool {
	return strings.Contains(output, "\t(cached)")
}

// OutputLines returns the full test output for a test as an slice of lines.
// This function is a convenient wrapper around Package.OutputLines() to
// support the hiding of output in the summary.
//
// See Package.OutLines() for more details.
func (e *Execution) OutputLines(tc TestCase) []string {
	return e.packages[tc.Package].OutputLines(tc)
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
	var failed []TestCase //nolint:prealloc
	for _, name := range sortedKeys(e.packages) {
		pkg := e.packages[name]

		// Add package-level failure output if there were no failed tests.
		if pkg.TestMainFailed() {
			failed = append(failed, TestCase{Package: name})
		}
		failed = append(failed, pkg.Failed...)
	}
	return failed
}

func sortedKeys(pkgs map[string]*Package) []string {
	keys := make([]string, 0, len(pkgs))
	for key := range pkgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Skipped returns a list of all the skipped test cases.
func (e *Execution) Skipped() []TestCase {
	skipped := make([]TestCase, 0, len(e.packages))
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

func (e *Execution) end() {
	e.done = true
	for _, pkg := range e.packages {
		pkg.end()
	}
}

// NewExecution returns a new Execution and records the current time as the
// time the test execution started.
func NewExecution() *Execution {
	return &Execution{
		started:  clock.Now(),
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
	var group errgroup.Group
	group.Go(func() error {
		return readStdout(config, execution)
	})
	group.Go(func() error {
		return readStderr(config, execution)
	})
	err := group.Wait()
	execution.end()
	return execution, err
}

func readStdout(config ScanConfig, execution *Execution) error {
	scanner := bufio.NewScanner(config.Stdout)
	for scanner.Scan() {
		raw := scanner.Bytes()
		event, err := parseEvent(raw)
		switch {
		case err == errBadEvent:
			// nolint: errcheck
			config.Handler.Err(errBadEvent.Error() + ": " + scanner.Text())
			continue
		case err != nil:
			return errors.Wrapf(err, "failed to parse test output: %s", string(raw))
		}

		execution.add(event)
		if err := config.Handler.Event(event, execution); err != nil {
			return err
		}
	}
	return errors.Wrap(scanner.Err(), "failed to scan test output")
}

func readStderr(config ScanConfig, execution *Execution) error {
	scanner := bufio.NewScanner(config.Stderr)
	for scanner.Scan() {
		line := scanner.Text()
		config.Handler.Err(line) // nolint: errcheck
		if isGoModuleOutput(line) {
			continue
		}
		execution.addError(line)
	}
	return errors.Wrap(scanner.Err(), "failed to scan test stderr")
}

func isGoModuleOutput(scannerText string) bool {
	prefixes := []string{
		"go: copying",
		"go: creating",
		"go: downloading",
		"go: extracting",
		"go: finding",
	}

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
		log.Warnf(string(raw))
		return TestEvent{}, errBadEvent
	}

	event := TestEvent{}
	err := json.Unmarshal(raw, &event)
	event.raw = raw
	return event, err
}

var errBadEvent = errors.New("bad output from test2json")
