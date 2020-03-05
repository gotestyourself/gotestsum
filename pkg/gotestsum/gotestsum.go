package gotestsum

import (
	"context"
	"fmt"
	"github.com/astralkn/gotestmng/pkg/options"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gotest.tools/gotestsum/testjson"
)

var version = "master"

func GoTestSum(opts *options.Options) error {
	if opts.Version {
		fmt.Fprintf(os.Stdout, "gotestsum version %s\n", version)
		os.Exit(0)
	}

	return run(opts)
}

func lookEnvWithDefault(key, defValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defValue
}

func run(opts *options.Options) error {
	ctx := context.Background()
	goTestProc, err := startGoTest(ctx, goTestCmdArgs(opts))
	if err != nil {
		return errors.Wrapf(err, "failed to run %s %s",
			goTestProc.cmd.Path,
			strings.Join(goTestProc.cmd.Args, " "))
	}
	defer goTestProc.cancel()

	out := os.Stdout
	handler, err := newEventHandler(opts, out, os.Stderr)
	if err != nil {
		return err
	}
	defer handler.Close() // nolint: errcheck
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  goTestProc.stdout,
		Stderr:  goTestProc.stderr,
		Handler: handler,
	})
	if err != nil {
		return err
	}
	testjson.PrintSummary(out, exec, testjson.Summary(opts.NoSummary.Value))
	if err := writeJUnitFile(opts, exec); err != nil {
		return err
	}
	return goTestProc.cmd.Wait()
}

func goTestCmdArgs(opts *options.Options) []string {
	args := opts.Args
	defaultArgs := []string{"go", "test"}
	switch {
	case opts.RawCommand:
		return args
	case len(args) == 0:
		return append(defaultArgs, "-json", pathFromEnv("./..."))
	case !hasJSONArg(args):
		defaultArgs = append(defaultArgs, "-json")
	}
	if testPath := pathFromEnv(""); testPath != "" {
		args = append(args, testPath)
	}
	return append(defaultArgs, args...)
}

func pathFromEnv(defaultPath string) string {
	return lookEnvWithDefault("TEST_DIRECTORY", defaultPath)
}

func hasJSONArg(args []string) bool {
	for _, arg := range args {
		if arg == "-json" || arg == "--json" {
			return true
		}
	}
	return false
}

type proc struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stderr io.Reader
	cancel func()
}

func startGoTest(ctx context.Context, args []string) (proc, error) {
	if len(args) == 0 {
		return proc{}, errors.New("missing command to run")
	}

	ctx, cancel := context.WithCancel(ctx)
	p := proc{
		cmd:    exec.CommandContext(ctx, args[0], args[1:]...),
		cancel: cancel,
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
