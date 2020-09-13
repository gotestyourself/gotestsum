package reaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

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
