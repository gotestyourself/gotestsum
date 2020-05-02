package slowest

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/testjson"
)

func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	if err := flags.Parse(args); err != nil {
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

Flags:
`, name)
		flags.PrintDefaults()
	}
	flags.DurationVar(&opts.threshold, "threshold", 100*time.Millisecond,
		"tests faster than this threshold will be omitted from the output")
	return flags, opts
}

type options struct {
	threshold time.Duration
}

func run(opts *options) error {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  os.Stdin,
		Stderr:  bytes.NewReader(nil),
		Handler: eventHandler{},
	})
	if err != nil {
		return err
	}
	for _, tc := range slowTestCases(exec, opts.threshold) {
		// TODO: allow elapsed time unit to be configurable
		fmt.Printf("%s %s %d\n", tc.Package, tc.Test, tc.Elapsed.Milliseconds())
	}

	return nil
}

// slowTestCases returns a slice of all tests with an elapsed time greater than
// threshold. The slice is sorted by Elapsed time in descending order (slowest
// test first).
// TODO: may be shared with testjson Summary
func slowTestCases(exec *testjson.Execution, threshold time.Duration) []testjson.TestCase {
	if threshold == 0 {
		return nil
	}
	pkgs := exec.Packages()
	tests := make([]testjson.TestCase, 0, len(pkgs))
	for _, pkg := range pkgs {
		tests = append(tests, exec.Package(pkg).TestCases()...)
	}
	// TODO: use median test runtime
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Elapsed > tests[j].Elapsed
	})
	end := sort.Search(len(tests), func(i int) bool {
		return tests[i].Elapsed < threshold
	})
	return tests[:end]
}

type eventHandler struct{}

func (h eventHandler) Err(text string) error {
	_, err := fmt.Fprintln(os.Stdout, text)
	return err
}

func (h eventHandler) Event(_ testjson.TestEvent, _ *testjson.Execution) error {
	return nil
}
