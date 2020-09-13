package reaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"gotest.tools/gotestsum/log"
)

type Config struct {
	CircleCIConfig CircleCIConfig
	ActionConfig   ActionConfig
	GithubConfig   GithubConfig
}

type ActionConfig struct {
	RerunFailsReportPattern string
}

func Act(ctx context.Context, cfg Config) error {
	jobs, err := getCircleCIArtifacts(ctx, cfg.CircleCIConfig)
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

func actionRerunFailsReport(ctx context.Context, cfg GithubConfig, url string) error {
	body, err := getArtifact(ctx, cfg.Client, url)
	if err != nil {
		return err
	}
	defer body.Close() //nolint:errcheck

	buf := new(bytes.Buffer)
	// TODO: add circleci job url to the comment
	buf.WriteString("gotestsum re-ran some tests:\n\n```\n")
	if _, err := buf.ReadFrom(body); err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	buf.WriteString("\n```\n")

	type commentBody struct {
		Body string `json:"body"`
	}
	raw, err := json.Marshal(commentBody{Body: buf.String()})
	if err != nil {
		return err
	}

	cfg.Comment = bytes.NewReader(raw)
	return postGithubComment(ctx, cfg)
}
