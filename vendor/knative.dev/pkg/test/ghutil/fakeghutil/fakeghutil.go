/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// fakeghutil.go fakes GithubClient for testing purpose

package fakeghutil

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"knative.dev/pkg/test/ghutil"
)

// FakeGithubClient is a faked client, implements all functions of ghutil.GithubOperations
type FakeGithubClient struct {
	User         *github.User
	Repos        []string
	Issues       map[string]map[int]*github.Issue       // map of repo: map of issueNumber: issues
	Comments     map[int]map[int64]*github.IssueComment // map of issueNumber: map of commentID: comments
	PullRequests map[string]map[int]*github.PullRequest // map of repo: map of PullRequest Number: pullrequests
	PRCommits    map[int][]*github.RepositoryCommit     // map of PR number: slice of commits
	CommitFiles  map[string][]*github.CommitFile        // map of commit SHA: slice of files

	NextNumber int    // number to be assigned to next newly created issue/comment
	BaseURL    string // base URL of Github
}

// NewFakeGithubClient creates a FakeGithubClient and initialize it's maps
func NewFakeGithubClient() *FakeGithubClient {
	return &FakeGithubClient{
		Issues:       make(map[string]map[int]*github.Issue),
		Comments:     make(map[int]map[int64]*github.IssueComment),
		PullRequests: make(map[string]map[int]*github.PullRequest),
		PRCommits:    make(map[int][]*github.RepositoryCommit),
		CommitFiles:  make(map[string][]*github.CommitFile),
		BaseURL:      "fakeurl",
	}
}

// GetGithubUser gets current authenticated user
func (fgc *FakeGithubClient) GetGithubUser() (*github.User, error) {
	return fgc.User, nil
}

// ListRepos lists repos under org
func (fgc *FakeGithubClient) ListRepos(org string) ([]string, error) {
	return fgc.Repos, nil
}

// ListIssuesByRepo lists issues within given repo, filters by labels if provided
func (fgc *FakeGithubClient) ListIssuesByRepo(org, repo string, labels []string) ([]*github.Issue, error) {
	var issues []*github.Issue
	for _, issue := range fgc.Issues[repo] {
		labelMap := make(map[string]bool)
		for _, label := range issue.Labels {
			labelMap[*label.Name] = true
		}
		missingLabel := false
		for _, label := range labels {
			if _, ok := labelMap[label]; !ok {
				missingLabel = true
				break
			}
		}
		if !missingLabel {
			issues = append(issues, issue)
		}
	}
	return issues, nil
}

// CreateIssue creates issue
func (fgc *FakeGithubClient) CreateIssue(org, repo, title, body string) (*github.Issue, error) {
	issueNumber := fgc.getNextNumber()
	stateStr := string(ghutil.IssueOpenState)
	repoURL := fmt.Sprintf("%s/%s/%s", fgc.BaseURL, org, repo)
	url := fmt.Sprintf("%s/%d", repoURL, issueNumber)
	newIssue := &github.Issue{
		Title:         &title,
		Body:          &body,
		Number:        &issueNumber,
		State:         &stateStr,
		URL:           &url,
		RepositoryURL: &repoURL,
	}
	if _, ok := fgc.Issues[repo]; !ok {
		fgc.Issues[repo] = make(map[int]*github.Issue)
	}
	fgc.Issues[repo][issueNumber] = newIssue
	return newIssue, nil
}

// CloseIssue closes issue
func (fgc *FakeGithubClient) CloseIssue(org, repo string, issueNumber int) error {
	return fgc.updateIssueState(org, repo, ghutil.IssueCloseState, issueNumber)
}

// ReopenIssue reopen issue
func (fgc *FakeGithubClient) ReopenIssue(org, repo string, issueNumber int) error {
	return fgc.updateIssueState(org, repo, ghutil.IssueOpenState, issueNumber)
}

// ListComments gets all comments from issue
func (fgc *FakeGithubClient) ListComments(org, repo string, issueNumber int) ([]*github.IssueComment, error) {
	var comments []*github.IssueComment
	for _, comment := range fgc.Comments[issueNumber] {
		comments = append(comments, comment)
	}
	return comments, nil
}

// GetComment gets comment by comment ID
func (fgc *FakeGithubClient) GetComment(org, repo string, commentID int64) (*github.IssueComment, error) {
	for _, comments := range fgc.Comments {
		if comment, ok := comments[commentID]; ok {
			return comment, nil
		}
	}
	return nil, fmt.Errorf("cannot find comment")
}

// CreateComment adds comment to issue
func (fgc *FakeGithubClient) CreateComment(org, repo string, issueNumber int, commentBody string) (*github.IssueComment, error) {
	commentID := int64(fgc.getNextNumber())
	newComment := &github.IssueComment{
		ID:   &commentID,
		Body: &commentBody,
		User: fgc.User,
	}
	if _, ok := fgc.Comments[issueNumber]; !ok {
		fgc.Comments[issueNumber] = make(map[int64]*github.IssueComment)
	}
	fgc.Comments[issueNumber][commentID] = newComment
	return newComment, nil
}

// EditComment edits comment by replacing with provided comment
func (fgc *FakeGithubClient) EditComment(org, repo string, commentID int64, commentBody string) error {
	comment, err := fgc.GetComment(org, repo, commentID)
	if nil != err {
		return err
	}
	comment.Body = &commentBody
	return nil
}

// DeleteComment deletes comment from issue
func (fgc *FakeGithubClient) DeleteComment(org, repo string, commentID int64) error {
	for _, comments := range fgc.Comments {
		if _, ok := comments[commentID]; ok {
			delete(comments, commentID)
			return nil
		}
	}
	return fmt.Errorf("cannot find comment")
}

// AddLabelsToIssue adds label on issue
func (fgc *FakeGithubClient) AddLabelsToIssue(org, repo string, issueNumber int, labels []string) error {
	targetIssue := fgc.Issues[repo][issueNumber]
	if nil == targetIssue {
		return fmt.Errorf("cannot find issue")
	}
	for _, label := range labels {
		targetIssue.Labels = append(targetIssue.Labels, github.Label{
			Name: &label,
		})
	}
	return nil
}

// RemoveLabelForIssue removes given label for issue
func (fgc *FakeGithubClient) RemoveLabelForIssue(org, repo string, issueNumber int, label string) error {
	targetIssue := fgc.Issues[repo][issueNumber]
	if nil == targetIssue {
		return fmt.Errorf("cannot find issue")
	}
	targetI := -1
	for i, l := range targetIssue.Labels {
		if *l.Name == label {
			targetI = i
		}
	}
	if -1 == targetI {
		return fmt.Errorf("cannot find label")
	}
	targetIssue.Labels = append(targetIssue.Labels[:targetI], targetIssue.Labels[targetI+1:]...)
	return nil
}

// ListPullRequests lists pull requests within given repo, filters by head user and branch name if
// provided as "user:ref-name", and by base name if provided, i.e. "master"
func (fgc *FakeGithubClient) ListPullRequests(org, repo, head, base string) ([]*github.PullRequest, error) {
	var res []*github.PullRequest
	PRs, ok := fgc.PullRequests[repo]
	if !ok {
		return res, fmt.Errorf("repo %s not exist", repo)
	}
	for _, PR := range PRs {
		// Filter with consistent logic of CreatePullRequest function below
		if ("" == head || *PR.Head.Label == head) &&
			("" == base || *PR.Base.Ref == base) {
			res = append(res, PR)
		}
	}
	// Sort by createdate is default in List
	sort.Slice(res, func(i, j int) bool {
		return nil != res[i].CreatedAt && res[i].CreatedAt.After(*res[j].CreatedAt)
	})
	return res, nil
}

// ListCommits lists commits from a pull request
func (fgc *FakeGithubClient) ListCommits(org, repo string, ID int) ([]*github.RepositoryCommit, error) {
	commits, ok := fgc.PRCommits[ID]
	if !ok {
		return commits, fmt.Errorf("no commits found for PR '%d'", ID)
	}
	return commits, nil
}

// ListFiles lists files from a pull request
func (fgc *FakeGithubClient) ListFiles(org, repo string, ID int) ([]*github.CommitFile, error) {
	var res []*github.CommitFile
	commits, err := fgc.ListCommits(org, repo, ID)
	if nil != err {
		return res, err
	}
	for _, commit := range commits {
		files, ok := fgc.CommitFiles[*commit.SHA]
		if !ok {
			// don't allow empty commit
			return res, fmt.Errorf("no files found for commit '%s'", *commit.SHA)
		}
		res = append(res, files...)
	}
	return res, nil
}

// GetPullRequest gets PullRequest by ID
func (fgc *FakeGithubClient) GetPullRequest(org, repo string, ID int) (*github.PullRequest, error) {
	if PRs, ok := fgc.PullRequests[repo]; ok {
		if PR, ok := PRs[ID]; ok {
			return PR, nil
		}
	}
	return nil, fmt.Errorf("PR not exist: '%d'", ID)
}

// GetPullRequestByCommitID gets PullRequest by commit ID
func (fgc *FakeGithubClient) GetPullRequestByCommitID(org, repo string, commitID string) (*github.PullRequest, error) {
	res := make([]*github.PullRequest, 0)
	for prNum, commits := range fgc.PRCommits {
		for _, commit := range commits {
			if commit.GetSHA() == commitID {
				if pullRequest, err := fgc.GetPullRequest(org, repo, prNum); err != nil {
					res = append(res, pullRequest)
				}
			}
		}
	}
	if len(res) != 1 {
		return nil, fmt.Errorf("GetPullRequestByCommitID is expected to return 1 PullRequest, got %d", len(res))
	}
	return res[0], nil
}

// EditPullRequest updates PullRequest
func (fgc *FakeGithubClient) EditPullRequest(org, repo string, ID int, title, body string) (*github.PullRequest, error) {
	PR, err := fgc.GetPullRequest(org, repo, ID)
	if nil != err {
		return nil, err
	}
	PR.Title = &title
	PR.Body = &body
	return PR, nil
}

// CreatePullRequest creates PullRequest, passing head user and branch name "user:ref-name", and base branch name like "master"
func (fgc *FakeGithubClient) CreatePullRequest(org, repo, head, base, title, body string) (*github.PullRequest, error) {
	PRNumber := fgc.getNextNumber()
	stateStr := string(ghutil.PullRequestOpenState)
	b := true
	PR := &github.PullRequest{
		Title:               &title,
		Body:                &body,
		MaintainerCanModify: &b,
		State:               &stateStr,
		Number:              &PRNumber,
	}
	if "" != head {
		tokens := strings.Split(head, ":")
		if len(tokens) != 2 {
			return nil, fmt.Errorf("invalid head, want: 'user:ref', got: '%s'", head)
		}
		PR.Head = &github.PullRequestBranch{
			Label: &head,
			Ref:   &tokens[1],
		}
	}
	if "" != base {
		l := fmt.Sprintf("%s:%s", repo, base)
		PR.Base = &github.PullRequestBranch{
			Label: &l,
			Ref:   &base,
		}
	}

	if _, ok := fgc.PullRequests[repo]; !ok {
		fgc.PullRequests[repo] = make(map[int]*github.PullRequest)
	}
	fgc.PullRequests[repo][PRNumber] = PR
	return PR, nil
}

// AddFileToCommit adds file to commit
// This is complementary of mocking CreatePullRequest, so that newly created pull request can have files
func (fgc *FakeGithubClient) AddFileToCommit(org, repo, SHA, filename, patch string) error {
	var commit *github.RepositoryCommit
	for _, commits := range fgc.PRCommits {
		for _, c := range commits {
			if *c.SHA == SHA {
				commit = c
				break
			}
		}
	}
	if nil == commit {
		return fmt.Errorf("commit %s not exist", SHA)
	}
	f := &github.CommitFile{
		Filename: &filename,
		Patch:    &patch,
	}
	if _, ok := fgc.CommitFiles[SHA]; !ok {
		fgc.CommitFiles[SHA] = make([]*github.CommitFile, 0, 0)
	}
	fgc.CommitFiles[SHA] = append(fgc.CommitFiles[SHA], f)
	return nil
}

// AddCommitToPullRequest adds commit to pullrequest
// This is complementary of mocking CreatePullRequest, so that newly created pull request can have commits
func (fgc *FakeGithubClient) AddCommitToPullRequest(org, repo string, ID int, SHA string) error {
	PRs, ok := fgc.PullRequests[repo]
	if !ok {
		return fmt.Errorf("repo %s not exist", repo)
	}
	if _, ok = PRs[ID]; !ok {
		return fmt.Errorf("Pull Request %d not exist", ID)
	}
	if _, ok = fgc.PRCommits[ID]; !ok {
		fgc.PRCommits[ID] = make([]*github.RepositoryCommit, 0, 0)
	}
	fgc.PRCommits[ID] = append(fgc.PRCommits[ID], &github.RepositoryCommit{SHA: &SHA})
	return nil
}

func (fgc *FakeGithubClient) updateIssueState(org, repo string, state ghutil.IssueStateEnum, issueNumber int) error {
	targetIssue := fgc.Issues[repo][issueNumber]
	if nil == targetIssue {
		return fmt.Errorf("cannot find issue")
	}
	stateStr := string(state)
	targetIssue.State = &stateStr
	return nil
}

func (fgc *FakeGithubClient) getNextNumber() int {
	fgc.NextNumber++
	return fgc.NextNumber
}
