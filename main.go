package main

import (
	"fmt"
	"github.com/astralkn/gotestmng/pkg/gotestsum"
	"github.com/astralkn/gotestmng/pkg/inspector"
	"github.com/astralkn/gotestmng/pkg/options"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/testjson"
	"os"
	"os/exec"
)

var version = "master"

func main() {
	name := os.Args[0]
	flags, opts := setupFlags(name)
	switch err := flags.Parse(os.Args[1:]); {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		log.Error(err.Error())
		flags.Usage()
		os.Exit(1)
	}
	opts.Args = flags.Args()
	setupLogging(opts)
	if opts.Version {
		fmt.Fprintf(os.Stdout, "gotestmng version %s\n", version)
		os.Exit(0)
	}
	err := run(opts)
	switch err.(type) {
	case nil:
	case *exec.ExitError:
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, name+": Error :"+err.Error())
		os.Exit(3)
	}
}

func setupFlags(name string) (*pflag.FlagSet, *options.Options) {
	opts := &options.Options{
		NoSummary:                    options.NewNoSummaryValue(),
		JunitTestCaseClassnameFormat: &options.JunitFieldFormatValue{},
		JunitTestSuiteNameFormat:     &options.JunitFieldFormatValue{},
	}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
    %s [flags] [--] [go test flags]

Flags:
`, name)
		flags.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Formats:
    dots                    print a character for each test
    dots-v2                 experimental dots format, one package per line
    pkgname                 print a line for each package
    pkgname-and-test-fails  print a line for each package and failed test output
    testname                print a line for each test and package
    standard-quiet          standard go test format
    standard-verbose        standard go test -v format
`)
	}
	flags.StringVar(&opts.Token, "token", "", "set remote github auth token")
	flags.StringVar(&opts.Owner, "owner", "", "set remote github repository owner ")
	flags.StringVar(&opts.Repo, "repo", "", "set remote github repository")
	flags.BoolVar(&opts.Post, "post", false, "post found failures on github")
	flags.BoolVar(&opts.Debug, "debug", false, "enabled debug")
	flags.StringVarP(&opts.Format, "format", "f", lookEnvWithDefault("GOTESTSUM_FORMAT", "short"), "print format of test input")
	flags.BoolVar(&opts.RawCommand, "raw-command", false, "don't prepend 'go test -json' to the 'go test' command")
	flags.StringVar(&opts.JsonFile, "jsonfile", lookEnvWithDefault("GOTESTSUM_JSONFILE", ""), "write all TestEvents to file")
	flags.StringVar(&opts.JunitFile, "junitfile", lookEnvWithDefault("GOTESTSUM_JUNITFILE", ""), "write a JUnit XML file")
	flags.BoolVar(&opts.NoColor, "no-color", color.NoColor, "disable color output")
	flags.Var(opts.NoSummary, "no-summary", "do not print summary of: "+testjson.SummarizeAll.String())
	flags.Var(opts.JunitTestSuiteNameFormat, "junitfile-testsuite-name", "format the testsuite name field as: "+options.JunitFieldFormatValues)
	flags.Var(opts.JunitTestCaseClassnameFormat, "junitfile-testcase-classname", "format the testcase classname field as: "+options.JunitFieldFormatValues)
	flags.BoolVar(&opts.Version, "version", false, "show version and exit")
	return flags, opts
}

func run(opts *options.Options) error {
	err := gotestsum.GoTestSum(opts)
	//if err != nil {
	//	return err
	//}
	inspector.NewJunitInspector().Inspect(opts)
	//ins := inspector.NewGitInspector(context.Background(), opts.Token, opts.Owner, opts.Repo)
	//err = ins.Inspect(opts)
	//if err != nil {
	//	log.Error(err)
	//	os.Exit(3)
	//}
	return err
}

func lookEnvWithDefault(key, defValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defValue
}

func setupLogging(opts *options.Options) {
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}
	color.NoColor = opts.NoColor
}
