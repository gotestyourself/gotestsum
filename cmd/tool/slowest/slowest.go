package slowest

import (
	"bytes"
	"fmt"
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

By default this command will print the list of tests slower than threshold to stdout.
If --skip-stmt is set, instead of printing the list of stdout, the AST for the
Go source code in the working directory tree will be modified. The --skip-stmt
will be added to Go test files as the first statement in all the test functions
which are slower than threshold.

Example - use testing.Short():

    skip_stmt='if testing.Short() { t.Skip("too slow for short run") }'
    go test -json -short ./... | %s --skip-stmt "$skip_stmt"

Flags:
`, name, name)
		flags.PrintDefaults()
	}
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
	skipStatement string
	debug         bool
}

func run(opts *options) error {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: os.Stdin,
		Stderr: bytes.NewReader(nil),
	})
	if err != nil {
		return fmt.Errorf("failed to scan testjson: %w", err)
	}

	tcs := slowTestCases(exec, opts.threshold)
	if opts.skipStatement != "" {
		skipStmt, err := parseSkipStatement(opts.skipStatement)
		if err != nil {
			return fmt.Errorf("failed to parse skip expr: %w", err)
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
// FIXME: use medium elapsed time when there are multiple instances of the same test
func slowTestCases(exec *testjson.Execution, threshold time.Duration) []testjson.TestCase {
	if threshold == 0 {
		return nil
	}
	pkgs := exec.Packages()
	tests := make([]testjson.TestCase, 0, len(pkgs))
	for _, pkg := range pkgs {
		tests = append(tests, exec.Package(pkg).TestCases()...)
	}
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Elapsed > tests[j].Elapsed
	})
	end := sort.Search(len(tests), func(i int) bool {
		return tests[i].Elapsed < threshold
	})
	return tests[:end]
}
