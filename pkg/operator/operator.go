package operator

import (
	"context"
	"github.com/astralkn/gotestmng/pkg/options"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"strings"
)

type FailedTest struct {
	Title   string
	Issues  string
	IssueNo int
}

const (
	testFailTag = "@BotTestIssue"
	FailureTag  = "testFailure"
)

type GitOperator struct {
	client *github.Client
	ctx    context.Context
	owner  string
	repo   string
}

func NewGitOperator(owner, repo, token string, ctx context.Context) *GitOperator {
	g := &GitOperator{
		ctx:   ctx,
		owner: owner,
		repo:  repo,
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	g.client = github.NewClient(tc)
	return g
}

func NewUnauthenticatedGitOperator(owner, repo string, ctx context.Context) *GitOperator {
	return &GitOperator{
		ctx:    ctx,
		owner:  owner,
		repo:   repo,
		client: github.NewClient(nil),
	}
}

func (g *GitOperator) GetTestIssues() (*[]FailedTest, error) {
	issues, err := g.GetIssuesByLabel(FailureTag)
	if err != nil {
		return nil, err
	}
	var res []FailedTest
	for _, i := range issues {
		res = append(res, FailedTest{
			Title:   *i.Title,
			Issues:  *i.Body,
			IssueNo: *i.Number,
		})
	}
	return &res, nil
}

func (g *GitOperator) GetIssuesByLabel(label string) ([]*github.Issue, error) {
	list, _, err := g.client.Issues.ListByRepo(g.ctx, g.owner, g.repo, &github.IssueListByRepoOptions{
		Labels: []string{label},
	})
	if err != nil {
		return nil, err
	}
	return list, err
}

func (g *GitOperator) PostNewIssue(f *FailedTest) error {
	_, _, err := g.client.Issues.Create(g.ctx, g.owner, g.repo, g.getIssueForTest(f))
	return err
}

func (g *GitOperator) getIssueForTest(f *FailedTest) *github.IssueRequest {
	return &github.IssueRequest{
		Title:  &f.Title,
		Body:   &f.Issues,
		Labels: &[]string{FailureTag},
	}
}

func (g *GitOperator) CloseSolvedIssue(f *FailedTest) error {
	req := g.getIssueForTest(f)
	s := "closed"
	req.State = &s
	_, _, err := g.client.Issues.Edit(g.ctx, g.owner, g.repo, f.IssueNo, req)
	return err
}

func labelContains(s []github.Label, e string) bool {
	for _, a := range s {
		if *a.Name == e {
			return true
		}
	}
	return false
}

type JUnitOperator struct {
}

func (_ *JUnitOperator) GetFailedTests(opts *options.Options) *[]FailedTest {
	ft := &[]FailedTest{}
	for _, s := range opts.JUnitTestSuite.Suites {
		if s.Failures == 0 {
			continue
		}
		for _, t := range s.TestCases {
			if t.Failure != nil {
				*ft = append(*ft, FailedTest{
					Title:  t.Name,
					Issues: strings.Replace(t.Failure.Type+t.Failure.Contents+t.Failure.Message, "\n", " ", -1),
				})
			}
		}
	}
	return ft
}
