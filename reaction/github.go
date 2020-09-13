package reaction

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type GithubConfig struct {
	Token    string
	Project  string
	PRNumber int
	Comment  io.Reader
	Client   httpDoer
}

const urlPostGithubComment = `https://api.github.com/repos/%s/issues/%d/comments`

func postGithubComment(ctx context.Context, cfg GithubConfig) error {
	url := fmt.Sprintf(urlPostGithubComment, cfg.Project, cfg.PRNumber)
	req, err := http.NewRequest(http.MethodPost, url, cfg.Comment)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", "token "+cfg.Token)
	resp, err := cfg.Client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	if err := statusError(resp); err != nil {
		return fmt.Errorf("failed to post Github comment: %w", err)
	}
	return nil
}
