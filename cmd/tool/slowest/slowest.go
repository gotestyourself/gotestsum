package slowest

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

// Run the command
func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		flags.Usage()
		return err
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
    %s [flags]

Read a json file and print or update tests which are slower than threshold.
The json file can be created with 'gotestsum --jsonfile' or 'go test -json'.

By default this command will print the list of tests slower than threshold to stdout.
If --skip-stmt is set, instead of printing the list of stdout, the AST for the
Go source code in the working directory tree will be modified. The --skip-stmt
will be added to Go test files as the first statement in all the test functions
which are slower than threshold.

The --skip-stmt flag may be set to the name of a predefine statement, or a
source code which will be parsed as a go/ast.Stmt. Currently there is only one
predefined statement: testing.Short:

    if testing.Short() {
        t.Skip("too slow for testing.Short")
    }

Example - using a custom --skip-stmt:

    skip_stmt='
        if os.Getenv("TEST_FAST") {
            t.Skip("too slow for TEST_FAST")
        }
    '
    go test -json -short ./... | %s --skip-stmt "$skip_stmt"

Note that this tool does not add imports, so using a custom statement may require
you to add any necessary imports to the file.

Go build flags, such as build tags, may be set using the GOFLAGS environment
variable, following the same rules as the go toolchain. See
https://golang.org/cmd/go/#hdr-Environment_variables.

Flags:
`, name, name)
		flags.PrintDefaults()
	}
	flags.StringVar(&opts.jsonfile, "jsonfile", "",
		"path to test2json output, defaults to stdin")
	flags.DurationVar(&opts.threshold, "threshold", 100*time.Millisecond,
		"tests faster than this threshold will be omitted from the output")
	flags.StringVar(&opts.skipStatement, "skip-stmt", "",
		"add this go statement to slow tests, instead of printing the list of slow tests")
	flags.BoolVar(&opts.debug, "debug", false,
		"enable debug logging.")
	return flags, opts
}

type options struct {
	threshold     time.Duration
	jsonfile      string
	skipStatement string
	debug         bool
}

func run(opts *options) error {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	in, err := jsonfileReader(opts.jsonfile)
	if err != nil {
		return fmt.Errorf("failed to read jsonfile: %v", err)
	}
	defer func() {
		if err := in.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", opts.jsonfile, err)
		}
	}()

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: in,
		Stderr: bytes.NewReader(nil),
	})
	if err != nil {
		return fmt.Errorf("failed to scan testjson: %v", err)
	}

	tcs := slowTestCases(exec, opts.threshold)
	if opts.skipStatement != "" {
		skipStmt, err := parseSkipStatement(opts.skipStatement)
		if err != nil {
			return fmt.Errorf("failed to parse skip expr: %v", err)
		}
		return writeTestSkip(tcs, skipStmt)
	}
	for _, tc := range tcs {
		fmt.Printf("%s %s %v\n", tc.Package, tc.Test, tc.Elapsed)
	}

	return nil
}

// slowTestCases returns a slice of all tests with an elapsed time greater than
// threshold. The slice is sorted by Elapsed time in descending order (slowest
// test first).
//
// If there are multiple runs of a TestCase, all of them will be represented
// by a single TestCase with the median elapsed time in the returned slice.
func slowTestCases(exec *testjson.Execution, threshold time.Duration) []testjson.TestCase {
	if threshold == 0 {
		return nil
	}
	pkgs := exec.Packages()
	tests := make([]testjson.TestCase, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkgTests := aggregateTestCases(exec.Package(pkg).TestCases())
		tests = append(tests, pkgTests...)
	}
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Elapsed > tests[j].Elapsed
	})
	end := sort.Search(len(tests), func(i int) bool {
		return tests[i].Elapsed < threshold
	})
	return tests[:end]
}

// collectTestCases maps all test cases by name, and if there is more than one
// instance of a TestCase, finds the median elapsed time for all the runs.
//
// All cases are assumed to be part of the same package.
func aggregateTestCases(cases []testjson.TestCase) []testjson.TestCase {
	if len(cases) < 2 {
		return cases
	}
	pkg := cases[0].Package
	// nolint: prealloc // size is not predictable
	m := make(map[string][]time.Duration)
	for _, tc := range cases {
		m[tc.Test] = append(m[tc.Test], tc.Elapsed)
	}
	result := make([]testjson.TestCase, 0, len(m))
	for name, timing := range m {
		result = append(result, testjson.TestCase{
			Package: pkg,
			Test:    name,
			Elapsed: median(timing),
		})
	}
	return result
}

func median(times []time.Duration) time.Duration {
	switch len(times) {
	case 0:
		return 0
	case 1:
		return times[0]
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})
	return times[len(times)/2]
}

func jsonfileReader(v string) (io.ReadCloser, error) {
	switch v {
	case "", "-":
		return ioutil.NopCloser(os.Stdin), nil
	default:
		return os.Open(v)
	}
}
