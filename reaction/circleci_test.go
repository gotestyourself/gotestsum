package reaction

import (
	"context"
	"net/http"
	"os"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestGetArtifactsURLs(t *testing.T) {
	token := os.Getenv("CIRCLECI_API_TOKEN")
	skip.If(t, token == "", "CIRCLECI_API_TOKEN env var is required")
	t.Skip("skip to avoid hitting API rate limit")

	ctx := context.Background()
	// TODO: test may start to fail after 30 days since artifacts are deleted.
	job := artifactURLRequest{
		ProjectSlug: "github/hashicorp/consul",
		JobNum:      236276,
		Token:       token,
	}
	arts, err := getArtifactURLs(ctx, &http.Client{}, job)
	assert.NilError(t, err)

	assert.Assert(t, len(arts.Items) > 0, arts)
}

func TestFilterArtifactURLs(t *testing.T) {
	a := responseArtifact{
		Items: []responseArtifactItem{
			{Path: "tmp/jsonfile/somethingelse.log", URL: "https://artifacts/somethingelse.log"},
			{Path: "tmp/jsonfile/go-test-1.log", URL: "https://artifacts/tmp/jsonfile/go-test-1.log"},
			{Path: "tmp/jsonfile/go-test-2.log", URL: "https://artifacts/tmp/jsonfile/go-test-2.log"},
		},
	}

	urls, err := filterArtifactURLs(a, "*/jsonfile/go-test-?.log")
	assert.NilError(t, err)
	expected := []string{
		"https://artifacts/tmp/jsonfile/go-test-1.log",
		"https://artifacts/tmp/jsonfile/go-test-2.log",
	}
	assert.DeepEqual(t, urls, expected)
}

func TestWorkflowJobs(t *testing.T) {
	token := os.Getenv("CIRCLECI_API_TOKEN")
	skip.If(t, token == "", "CIRCLECI_API_TOKEN env var is required")
	t.Skip("skip to avoid hitting API rate limit")

	ctx := context.Background()
	req := workflowJobsRequest{
		WorkflowID: "f44ffea6-9814-4906-ab29-b8235d0ee4ed",
		Token:      token,
	}

	jobs, err := getWorkflowJobs(ctx, &http.Client{}, req)
	assert.NilError(t, err)
	expected := []workflowJob{
		{Name: "test-connect-ca-providers", Num: 275978},
		{Name: "lint-consul-retry", Num: 275961},
		{Name: "lint", Num: 275970},
		{Name: "go-test-sdk", Num: 275973},
		{Name: "go-test-race", Num: 275977},
		{Name: "dev-build", Num: 275974},
		{Name: "go-test", Num: 276015},
		{Name: "go-test-api", Num: 276016},
		{Name: "check-vendor", Num: 275967},
		{Name: "check-generated-protobuf", Num: 275963},
	}
	assert.DeepEqual(t, jobs, expected)
}
