package inspector

import (
	"context"
	"fmt"
	"github.com/astralkn/gotestmng/pkg/options"
	"github.com/google/go-github/github"
	"github.com/joshdk/go-junit"
	"golang.org/x/oauth2"
	"log"
	"strings"
)

const testFailTag = "@BotTestIssue"
const FailureTag = "testFailure"

type Inspect interface {
	Inspect(opts *options.Options) error
}

type gitInspector struct {
	ctx    context.Context
	client *github.Client
	owner  string
	repo   string
	opts   options.Options
}

func NewGitInspector(ctx context.Context, token string, owner, repo string) *gitInspector {
	return &gitInspector{
		ctx:    ctx,
		client: newGitClient(ctx, token),
		owner:  owner,
		repo:   repo,
	}
}

func NewUnauthenticatedGitInspector(ctx context.Context) *gitInspector {
	return &gitInspector{
		ctx:    ctx,
		client: github.NewClient(nil),
	}
}

func newGitClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func (g *gitInspector) Inspect(opts *options.Options) error {
	//issues, err := g.getKnownTestIssues(FailureTag)
	//if err != nil {
	//	return err
	//}
	//tests, err := g.getFailedTests(opts)
	//if err != nil {
	//	return err
	//}
	//newIssues := g.extractNewIssues(tests, issues)
	//if opts.Post {
	//	requests := createGitIssuesRequest(*newIssues)
	//	for _, req := range *requests {
	//		resp, _, err := g.client.Issues.Create(g.ctx, opts.Owner, opts.Repo, &req)
	//		if err != nil {
	//			return err
	//		}
	//		log.Printf("Gitchab issue created :%v", resp)
	//	}
	//}
	//for k, v := range *newIssues {
	//	log.Printf("New Issue: %s\nErrors : %s", k, strings.Join(v, "\n"))
	//}
	return nil
}

func (g *gitInspector) getKnownTestIssues(label string) (*[]github.Issue, error) {
	issues := &[]github.Issue{}
	list, _, err := g.client.Issues.ListByRepo(g.ctx, g.owner, g.repo, nil)
	if err != nil {
		return nil, err
	}
	for _, i := range list {
		if labelContains(i.Labels, label) && *i.State == "open" {
			*issues = append(*issues, *i)
		}
	}
	return issues, err
}

func labelContains(s []github.Label, e string) bool {
	for _, a := range s {
		if *a.Name == e {
			return true
		}
	}
	return false
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func extractDiff(s1 []string, s2 []string) *[]string {
	res := &[]string{}
	for _, str := range s2 {
		if !contains(s1, str) {
			*res = append(*res, str)
		}
	}
	return res
}

func (_ *junitInspector) getFailedTests(opts *options.Options) *[]FailedTest {
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

func (g *gitInspector) extractNewIssues(j *[]junit.Suite, gh *[]github.Issue) *map[string][]string {
	foundIssues := make(map[string][]string)
	//for _, suite := range *j {
	//	for _, test := range suite.Tests {
	//		if test.Status == junit.StatusError || test.Status == junit.StatusFailed {
	//			if len(foundIssues[test.Name]) == 0 {
	//				foundIssues[test.Name] = []string{test.Error.Error()}
	//			} else {
	//				foundIssues[test.Name] = append(foundIssues[test.Name], test.Error.Error())
	//			}
	//
	//		}
	//	}
	//}
	//knownIssues := make(map[string]map[string]bool)
	//for _, i := range *gh {
	//	if len(knownIssues[*i.Title]) == 0 {
	//		knownIssues[*i.Title] = make(map[string]bool)
	//		knownIssues[*i.Title][*extractIssues(&i)] = true
	//	} else {
	//		knownIssues[*i.Title] = append(knownIssues[*i.Title], *extractIssues(&i)...)
	//	}
	//}
	//
	//for k, v := range foundIssues {
	//	if len(knownIssues[k]) != 0 {
	//		if !reflect.DeepEqual(v, knownIssues[k]) {
	//			foundIssues[k] = *extractDiff(v, knownIssues[k])
	//		}
	//	}
	//}
	return &foundIssues
}

func createGitIssuesRequest(issues map[string][]string) *[]github.IssueRequest {
	iss := &[]github.IssueRequest{}
	for k, v := range issues {
		join := fmt.Sprintf("Found issues:\n%s%s\n", testFailTag, strings.Join(v, "\n"+testFailTag))
		*iss = append(*iss, github.IssueRequest{
			Title:  &k,
			Body:   &join,
			Labels: &[]string{FailureTag},
		})
	}
	return iss
}

func extractIssues(issue *github.Issue) *[]string {
	issues := &[]string{}
	extractIssue(*issue.Body, issues)
	return issues
}

func extractIssue(str string, list *[]string) {
	split := strings.Split(str, "\n")
	for _, str := range split {
		if strings.HasPrefix(str, testFailTag) {
			*list = append(*list, strings.TrimPrefix(str, testFailTag))
		}
	}
}

type junitInspector struct{}

func (j *junitInspector) Inspect(opts *options.Options) error {
	tests := j.getFailedTests(opts)
	for _, t := range *tests {
		log.Println(t)
	}
	return nil
}

func NewJunitInspector() *junitInspector {
	return &junitInspector{}
}

type Issue struct {
}

type FailedTest struct {
	Title  string
	Issues string
}

func (f *FailedTest) equals(f2 *FailedTest) bool {
	if f.Title == f2.Title && f.Issues == f2.Issues {
		return true
	}
	return false
}
