package reaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
	"gotest.tools/gotestsum/log"
)

type CircleCIConfig struct {
	ProjectSlug string
	Token       string
	Client      httpDoer

	JobNum     int
	WorkflowID string
	JobPattern string
}

type Config struct {
	CircleCIConfig CircleCIConfig
	ActionConfig   ActionConfig
	GithubConfig   GithubConfig
}

type ActionConfig struct {
	RerunFailsReportPattern string
}

func Act(ctx context.Context, cfg Config) error {
	jobs, err := getJobArtifacts(ctx, cfg.CircleCIConfig)
	if err != nil {
		return fmt.Errorf("failed to get artifacts for job(s): %w", err)
	}
	log.Debugf("found %d job(s)", len(jobs))

	var errs []error
	for _, job := range jobs {
		for _, art := range job.Artifacts {
			switch err := actionForArtifact(ctx, cfg, art); {
			case errors.Is(err, errNoAction):
				log.Debugf("artifact %v matched no patterns (%v)",
					art.Path, cfg.ActionConfig.RerunFailsReportPattern)
			case err != nil:
				errs = append(errs, err)
			}
		}
	}

	return fmtErrors("failed to perform some actions", errs)
}

var errNoAction = fmt.Errorf("no action")

func actionForArtifact(ctx context.Context, cfg Config, art responseArtifactItem) error {
	switch matched, err := path.Match(cfg.ActionConfig.RerunFailsReportPattern, art.Path); {
	case err != nil:
		return err
	case matched:
		if cfg.GithubConfig.PRNumber == 0 {
			log.Warnf("Missing Github PR number to send rerun-fails report")
			return nil
		}
		return actionRerunFailsReport(ctx, cfg.GithubConfig, art.URL)
	}

	return errNoAction
}

func fmtErrors(msg string, errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		b := new(strings.Builder)

		for _, err := range errs {
			b.WriteString("\n   ")
			b.WriteString(err.Error())
		}
		return fmt.Errorf(msg+":%s\n", b.String())
	}
}

func getJobArtifacts(ctx context.Context, cfg CircleCIConfig) ([]jobArtifacts, error) {
	if cfg.JobNum != 0 {
		arts, err := getArtifactURLsForJob(ctx, cfg)
		if err != nil {
			return nil, err
		}
		// TODO: Job name is not set
		return []jobArtifacts{{Artifacts: arts}}, nil
	}
	return getArtifactURLsForWorkflow(ctx, cfg)
}

// getArtifactURLsForWorkflow for projects with Github Checks enabled.
func getArtifactURLsForWorkflow(ctx context.Context, cfg CircleCIConfig) ([]jobArtifacts, error) {
	jobs, err := getWorkflowJobs(ctx, cfg.Client, workflowJobsRequest{
		WorkflowID: cfg.WorkflowID,
		Token:      cfg.Token,
	})
	if err != nil {
		return nil, err
	}

	var result []jobArtifacts // nolint:prealloc
	for _, job := range jobs {
		switch matched, err := path.Match(cfg.JobPattern, job.Name); {
		case err != nil:
			return nil, err
		case !matched:
			continue
		}

		cfg.JobNum = job.Num
		arts, err := getArtifactURLsForJob(ctx, cfg)
		if err != nil {
			return nil, err
		}

		log.Debugf("found %d artifacts for job %v", len(arts), job.Name)
		result = append(result, jobArtifacts{Job: job.Name, Artifacts: arts})
	}
	return result, nil
}

type jobArtifacts struct {
	Job       string
	Artifacts []responseArtifactItem
}

// getArtifactURLsForJob for a single CircleCI job.
func getArtifactURLsForJob(ctx context.Context, cfg CircleCIConfig) ([]responseArtifactItem, error) {
	req := artifactURLRequest{
		ProjectSlug: cfg.ProjectSlug,
		JobNum:      cfg.JobNum,
		Token:       cfg.Token,
	}
	arts, err := getArtifactURLs(ctx, cfg.Client, req)
	if err != nil {
		return nil, err
	}
	return arts.Items, nil
}

type responseArtifact struct {
	Items []responseArtifactItem `json:"items"`
}

type responseArtifactItem struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type artifactURLRequest struct {
	ProjectSlug string
	JobNum      int
	Token       string
}

const circleArtifactsURL = `https://circleci.com/api/v2/project/%s/%d/artifacts`

func getArtifactURLs(ctx context.Context, c httpDoer, opts artifactURLRequest) (*responseArtifact, error) {
	u := fmt.Sprintf(circleArtifactsURL, url.PathEscape(opts.ProjectSlug), opts.JobNum)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Circle-Token", opts.Token)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := statusError(resp); err != nil {
		return nil, fmt.Errorf("failed to query artifact URLs: %w", err)
	}
	arts := &responseArtifact{}
	err = json.NewDecoder(resp.Body).Decode(arts)
	return arts, err
}

func statusError(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		msg := readBodyError(resp.Body)
		return fmt.Errorf("http request failed: %v %v", resp.Status, msg)
	}
	return nil
}

func readBodyError(body io.Reader) string {
	msg, err := ioutil.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}
	return string(msg)
}

func filterArtifactURLs(arts responseArtifact, glob string) ([]string, error) {
	result := make([]string, 0, len(arts.Items))
	for _, item := range arts.Items {
		switch matched, err := path.Match(glob, item.Path); {
		case err != nil:
			return nil, err
		case !matched:
			continue
		}
		result = append(result, item.URL)
	}
	return result, nil
}

// getArtifact from url. The caller must close the returned ReadCloser.
//
// nolint: bodyclose
func getArtifact(ctx context.Context, c httpDoer, url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

type workflowJobsRequest struct {
	WorkflowID string
	Token      string
}

const circleWorkflowJobsURL = `https://circleci.com/api/v2/workflow/%s/job`

func getWorkflowJobs(ctx context.Context, c httpDoer, opts workflowJobsRequest) ([]workflowJob, error) {
	u := fmt.Sprintf(circleWorkflowJobsURL, url.PathEscape(opts.WorkflowID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Circle-Token", opts.Token)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if err := statusError(resp); err != nil {
		return nil, fmt.Errorf("failed to get workflow jobs: %w", err)
	}
	return decodeWorkflowJobs(resp.Body)
}

type workflowJob struct {
	Name string `json:"name"`
	Num  int    `json:"job_number"`
}

func decodeWorkflowJobs(body io.Reader) ([]workflowJob, error) {
	type response struct {
		Items []workflowJob
	}
	var out response
	err := json.NewDecoder(body).Decode(&out)
	return out.Items, err
}
