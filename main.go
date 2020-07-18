package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/cmd"
	"gotest.tools/gotestsum/cmd/tool"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

var version = "master"

func main() {
	err := route(os.Args)
	switch err.(type) {
	case nil:
		return
	case *exec.ExitError:
		// go test should already report the error to stderr, exit with
		// the same status code
		os.Exit(ExitCodeWithDefault(err))
	default:
		log.Error(err.Error())
		os.Exit(3)
	}
}

func route(args []string) error {
	name := args[0]
	next, rest := cmd.Next(args[1:])
	switch next {
	case "tool":
		return tool.Run(name+" "+next, rest)
	default:
		return runMain(name, args[1:])
	}
}

func runMain(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	opts.args = flags.Args()
	setupLogging(opts)

	if opts.version {
		fmt.Fprintf(os.Stdout, "gotestsum version %s\n", version)
		return nil
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{
		noSummary:                    newNoSummaryValue(),
		junitTestCaseClassnameFormat: &junitFieldFormatValue{},
		junitTestSuiteNameFormat:     &junitFieldFormatValue{},
		postRunHookCmd:               &commandValue{},
		stdout:                       os.Stdout,
		stderr:                       os.Stderr,
	}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.StringVarP(&opts.format, "format", "f",
		lookEnvWithDefault("GOTESTSUM_FORMAT", "short"),
		"print format of test input")
	flags.BoolVar(&opts.rawCommand, "raw-command", false,
		"don't prepend 'go test -json' to the 'go test' command")
	flags.StringVar(&opts.jsonFile, "jsonfile",
		lookEnvWithDefault("GOTESTSUM_JSONFILE", ""),
		"write all TestEvents to file")
	flags.BoolVar(&opts.noColor, "no-color", color.NoColor, "disable color output")
	flags.Var(opts.noSummary, "no-summary",
		"do not print summary of: "+testjson.SummarizeAll.String())
	flags.Var(opts.postRunHookCmd, "post-run-command",
		"command to run after the tests have completed")

	flags.StringVar(&opts.junitFile, "junitfile",
		lookEnvWithDefault("GOTESTSUM_JUNITFILE", ""),
		"write a JUnit XML file")
	flags.Var(opts.junitTestSuiteNameFormat, "junitfile-testsuite-name",
		"format the testsuite name field as: "+junitFieldFormatValues)
	flags.Var(opts.junitTestCaseClassnameFormat, "junitfile-testcase-classname",
		"format the testcase classname field as: "+junitFieldFormatValues)

	flags.IntVar(&opts.rerunFailsMaxAttempts, "rerun-fails", 0,
		"rerun failed tests until they all pass, or attempts exceeds maximum. Defaults to max 2 reruns when enabled.")
	flags.Lookup("rerun-fails").NoOptDefVal = "2"
	flags.IntVar(&opts.rerunFailsMaxInitialFailures, "rerun-fails-max-failures", 10,
		"do not rerun any tests if the initial run has more than this number of failures")
	flags.Var((*stringSlice)(&opts.packages), "packages",
		"space separated list of package to test")
	flags.StringVar(&opts.rerunFailsReportFile, "rerun-fails-report", "",
		"write a report to the file, of the tests that were rerun")
	flags.BoolVar(&opts.rerunFailsOnlyRootCases, "rerun-fails-only-root-testcases", false,
		"rerun only root testcaes, instead of only subtests")
	flags.Lookup("rerun-fails-only-root-testcases").Hidden = true

	flags.BoolVar(&opts.debug, "debug", false, "enabled debug logging")
	flags.BoolVar(&opts.version, "version", false, "show version and exit")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags] [--] [go test flags]
    %[1]s [command]

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
	fmt.Fprint(out, `
Formats:
    dots                    print a character for each test
    dots-v2                 experimental dots format, one package per line
    pkgname                 print a line for each package
    pkgname-and-test-fails  print a line for each package and failed test output
    testname                print a line for each test and package
    standard-quiet          standard go test format
    standard-verbose        standard go test -v format

Commands:
    tool                    tools for working with test2json output
`)
}

func lookEnvWithDefault(key, defValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defValue
}

type options struct {
	args                         []string
	format                       string
	debug                        bool
	rawCommand                   bool
	jsonFile                     string
	junitFile                    string
	postRunHookCmd               *commandValue
	noColor                      bool
	noSummary                    *noSummaryValue
	junitTestSuiteNameFormat     *junitFieldFormatValue
	junitTestCaseClassnameFormat *junitFieldFormatValue
	rerunFailsMaxAttempts        int
	rerunFailsMaxInitialFailures int
	rerunFailsReportFile         string
	rerunFailsOnlyRootCases      bool
	packages                     []string
	version                      bool

	// shims for testing
	stdout io.Writer
	stderr io.Writer
}

func (o options) Validate() error {
	if o.rerunFailsMaxAttempts > 0 && len(o.args) > 0 && !o.rawCommand && len(o.packages) == 0 {
		return fmt.Errorf(
			"when go test args are used with --rerun-fails-max-attempts " +
				"the list of packages to test must be specified by the --packages flag")
	}
	return nil
}

func setupLogging(opts *options) {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	color.NoColor = opts.noColor
}

func run(opts *options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := opts.Validate(); err != nil {
		return err
	}

	goTestProc, err := startGoTest(ctx, goTestCmdArgs(opts, rerunOpts{}))
	if err != nil {
		return errors.Wrapf(err, "failed to run %s", strings.Join(goTestProc.cmd.Args, " "))
	}

	handler, err := newEventHandler(opts)
	if err != nil {
		return err
	}
	defer handler.Close() // nolint: errcheck
	cfg := testjson.ScanConfig{
		Stdout:  goTestProc.stdout,
		Stderr:  goTestProc.stderr,
		Handler: handler,
	}
	exec, err := testjson.ScanTestOutput(cfg)
	if err != nil {
		return err
	}
	goTestExitErr := goTestProc.cmd.Wait()

	if goTestExitErr != nil && opts.rerunFailsMaxAttempts > 0 {
		goTestExitErr = hasErrors(goTestExitErr, exec)
		if goTestExitErr == nil {
			cfg := testjson.ScanConfig{Execution: exec, Handler: handler}
			goTestExitErr = rerunFailed(ctx, opts, cfg)
		}
	}

	testjson.PrintSummary(opts.stdout, exec, opts.noSummary.value)
	if err := writeJUnitFile(opts, exec); err != nil {
		return err
	}
	if err := writeRerunFailsReport(opts, exec); err != nil {
		return err
	}
	if err := postRunHook(opts, exec); err != nil {
		return err
	}
	return goTestExitErr
}

func goTestCmdArgs(opts *options, rerunOpts rerunOpts) []string {
	if opts.rawCommand {
		var result []string
		result = append(result, opts.args...)
		result = append(result, rerunOpts.Args()...)
		return result
	}

	args := opts.args
	result := []string{"go", "test"}

	if len(args) == 0 {
		result = append(result, "-json")
		if rerunOpts.runFlag != "" {
			result = append(result, rerunOpts.runFlag)
		}
		return append(result, cmdArgPackageList(opts, rerunOpts, "./...")...)
	}

	if boolArgIndex("json", args) < 0 {
		result = append(result, "-json")
	}

	if rerunOpts.runFlag != "" {
		// Remove any existing run arg, it needs to be replaced with our new one
		// and duplicate args are not allowed by 'go test'.
		runIndex, runIndexEnd := argIndex("run", args)
		if runIndex >= 0 && runIndexEnd < len(args) {
			args = append(args[:runIndex], args[runIndexEnd+1:]...)
		}
		result = append(result, rerunOpts.runFlag)
	}

	pkgArgIndex := findPkgArgPosition(args)
	result = append(result, args[:pkgArgIndex]...)
	result = append(result, cmdArgPackageList(opts, rerunOpts)...)
	result = append(result, args[pkgArgIndex:]...)
	return result
}

func cmdArgPackageList(opts *options, rerunOpts rerunOpts, defPkgList ...string) []string {
	switch {
	case rerunOpts.pkg != "":
		return []string{rerunOpts.pkg}
	case len(opts.packages) > 0:
		return opts.packages
	case os.Getenv("TEST_DIRECTORY") != "":
		return []string{os.Getenv("TEST_DIRECTORY")}
	default:
		return defPkgList
	}
}

func boolArgIndex(flag string, args []string) int {
	for i, arg := range args {
		if arg == "-"+flag || arg == "--"+flag {
			return i
		}
	}
	return -1
}

func argIndex(flag string, args []string) (start, end int) {
	for i, arg := range args {
		if arg == "-"+flag || arg == "--"+flag {
			return i, i + 1
		}
		if strings.HasPrefix(arg, "-"+flag+"=") || strings.HasPrefix(arg, "--"+flag+"=") {
			return i, i
		}
	}
	return -1, -1
}

// The package list is before the -args flag, or at the end of the args list
// if the -args flag is not in args.
// The -args flag is a 'go test' flag that indicates that all subsequent
// args should be passed to the test binary. It requires that the list of
// packages comes before -args, so we re-use it as a placeholder in the case
// where some args must be passed to the test binary.
func findPkgArgPosition(args []string) int {
	if i := boolArgIndex("args", args); i >= 0 {
		return i
	}
	return len(args)
}

type proc struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stderr io.Reader
}

func startGoTest(ctx context.Context, args []string) (proc, error) {
	if len(args) == 0 {
		return proc{}, errors.New("missing command to run")
	}

	p := proc{
		cmd: exec.CommandContext(ctx, args[0], args[1:]...),
	}
	log.Debugf("exec: %s", p.cmd.Args)
	var err error
	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return p, err
	}
	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return p, err
	}
	err = p.cmd.Start()
	if err == nil {
		log.Debugf("go test pid: %d", p.cmd.Process.Pid)
	}
	return p, err
}
