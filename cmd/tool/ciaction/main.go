package ciaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/reaction"
)

// Run the command
func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}

	flags.BoolVar(&opts.debug, "debug", false, "enable debug logging.")

	flags.StringVar(&opts.circleCI.token, "circleci-token", os.Getenv("CIRCLECI_TOKEN"),
		"CircleCI API token")
	flags.Var(newWorkflowIDValue(&opts.circleCI.workflowID), "circleci-workflow",
		"CircleCI workflow id, or github check_run.external_id")
	flags.StringVar(&opts.circleCI.jobPattern, "circleci-job-pattern",
		getEnvWithDefault("CIRCLECI_JOB_PATTERN", "*"),
		"Glob pattern used to select which jobs in the workflow have relevant artifacts")
	flags.StringVar(&opts.rerunFailsReportPattern, "rerun-fails-report",
		getEnvWithDefault("RERUN_FAILS_PATTERN", "tmp/rerun-fails-report"),
		"Glob pattern used to match artifact paths for rerun-fails reports")

	flags.StringVar(&opts.github.project, "github-project", os.Getenv("GITHUB_PROJECT"),
		"Github project name in the form 'owner/repo'")
	flags.StringVar(&opts.github.token, "github-token", os.Getenv("GITHUB_TOKEN"),
		"Github API token")
	flags.IntVar(&opts.github.pullRequest, "github-pr", getEnvInt("GITHUB_PR"),
		"Github PR number")

	return flags, opts
}

func getEnvWithDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getEnvInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		log.Warnf("failed to parse value as int: %v", key)
	}
	return i
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

Fetch artifacts from a CI job, and perform for any actions that match a known
pattern.

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
}

type options struct {
	circleCI                circleCI
	github                  github
	rerunFailsReportPattern string
	debug                   bool
}

type circleCI struct {
	workflowID string
	jobNum     int
	token      string
	jobPattern string
}

type github struct {
	token       string
	project     string
	pullRequest int
}

func (o options) Validate() error {
	if o.circleCI.jobNum == 0 && o.circleCI.workflowID == "" {
		return fmt.Errorf("one of CIRCLECI_JOB or CIRCLECI_WORKFLOW is required")
	}
	if o.circleCI.token == "" {
		return fmt.Errorf("a CIRCLECI_TOKEN is required")
	}
	if o.github.project == "" {
		return fmt.Errorf("a GITHUB_PROJECT is required")
	}
	return nil
}

func run(opts *options) error {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	cfg := newConfigFromOptions(opts)
	err := reaction.Act(ctx, cfg)
	return err
}

func newConfigFromOptions(opts *options) reaction.Config {
	client := &http.Client{}
	return reaction.Config{
		CircleCIConfig: reaction.CircleCIConfig{
			ProjectSlug: "gh/" + opts.github.project,
			Token:       opts.circleCI.token,
			Client:      client,
			JobNum:      opts.circleCI.jobNum,
			WorkflowID:  opts.circleCI.workflowID,
			JobPattern:  opts.circleCI.jobPattern,
		},
		ActionConfig: reaction.ActionConfig{
			RerunFailsReportPattern: opts.rerunFailsReportPattern,
		},
		GithubConfig: reaction.GithubConfig{
			Token:    opts.github.token,
			Project:  opts.github.project,
			PRNumber: opts.github.pullRequest,
			Client:   client,
		},
	}
}

func newWorkflowIDValue(v *string) *workflowIDValue {
	w := (*workflowIDValue)(v)
	if err := w.Set(os.Getenv("CIRCLECI_WORKFLOW")); err != nil {
		log.Warnf("failed to parse CIRCLECI_WORKFLOW env var: %v", err)
	}
	return w
}

type workflowIDValue string

func (v *workflowIDValue) Set(id string) error {
	if !strings.Contains(id, `"workflow-id"`) {
		*v = workflowIDValue(id)
		return nil
	}

	type externalID struct {
		Value string `json:"workflow-id"`
	}
	target := &externalID{}
	if err := json.Unmarshal([]byte(id), target); err != nil {
		log.Warnf("failed to parse workflow-id from %v", v)
	}
	*v = workflowIDValue(target.Value)
	return nil
}

func (v *workflowIDValue) String() string {
	return string(*v)
}

func (v *workflowIDValue) Type() string {
	return "workflow-id"
}
