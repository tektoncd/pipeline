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

// issue.go provides generic functions related to issues

package ghutil

import (
	"fmt"

	"github.com/google/go-github/github"
)

const (
	// IssueOpenState is the state of open github issue
	IssueOpenState IssueStateEnum = "open"
	// IssueCloseState is the state of closed github issue
	IssueCloseState IssueStateEnum = "closed"
	// IssueAllState is the state for all, useful when querying issues
	IssueAllState IssueStateEnum = "all"
)

// IssueStateEnum represents different states of Github Issues
type IssueStateEnum string

// ListRepos lists repos under org
func (gc *GithubClient) ListRepos(org string) ([]string, error) {
	repoListOptions := &github.RepositoryListOptions{}
	genericList, err := gc.depaginate(
		"listing repos",
		maxRetryCount,
		&repoListOptions.ListOptions,
		func() ([]interface{}, *github.Response, error) {
			page, resp, err := gc.Client.Repositories.List(ctx, org, repoListOptions)
			var interfaceList []interface{}
			if nil == err {
				for _, repo := range page {
					interfaceList = append(interfaceList, repo)
				}
			}
			return interfaceList, resp, err
		},
	)
	res := make([]string, len(genericList))
	for i, elem := range genericList {
		res[i] = elem.(*github.Repository).GetName()
	}
	return res, err
}

// ListIssuesByRepo lists issues within given repo, filters by labels if provided
func (gc *GithubClient) ListIssuesByRepo(org, repo string, labels []string) ([]*github.Issue, error) {
	issueListOptions := &github.IssueListByRepoOptions{
		State: string(IssueAllState),
	}
	if len(labels) > 0 {
		issueListOptions.Labels = labels
	}

	genericList, err := gc.depaginate(
		fmt.Sprintf("listing issues with label '%v'", labels),
		maxRetryCount,
		&issueListOptions.ListOptions,
		func() ([]interface{}, *github.Response, error) {
			page, resp, err := gc.Client.Issues.ListByRepo(ctx, org, repo, issueListOptions)
			var interfaceList []interface{}
			if nil == err {
				for _, issue := range page {
					interfaceList = append(interfaceList, issue)
				}
			}
			return interfaceList, resp, err
		},
	)
	res := make([]*github.Issue, len(genericList))
	for i, elem := range genericList {
		res[i] = elem.(*github.Issue)
	}
	return res, err
}

// CreateIssue creates issue
func (gc *GithubClient) CreateIssue(org, repo, title, body string) (*github.Issue, error) {
	issue := &github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	var res *github.Issue
	_, err := gc.retry(
		fmt.Sprintf("creating issue '%s %s' '%s'", org, repo, title),
		maxRetryCount,
		func() (*github.Response, error) {
			var resp *github.Response
			var err error
			res, resp, err = gc.Client.Issues.Create(ctx, org, repo, issue)
			return resp, err
		},
	)
	return res, err
}

// CloseIssue closes issue
func (gc *GithubClient) CloseIssue(org, repo string, issueNumber int) error {
	return gc.updateIssueState(org, repo, IssueCloseState, issueNumber)
}

// ReopenIssue reopen issue
func (gc *GithubClient) ReopenIssue(org, repo string, issueNumber int) error {
	return gc.updateIssueState(org, repo, IssueOpenState, issueNumber)
}

// ListComments gets all comments from issue
func (gc *GithubClient) ListComments(org, repo string, issueNumber int) ([]*github.IssueComment, error) {
	commentListOptions := &github.IssueListCommentsOptions{}
	genericList, err := gc.depaginate(
		fmt.Sprintf("listing comment for issue '%s %s %d'", org, repo, issueNumber),
		maxRetryCount,
		&commentListOptions.ListOptions,
		func() ([]interface{}, *github.Response, error) {
			page, resp, err := gc.Client.Issues.ListComments(ctx, org, repo, issueNumber, commentListOptions)
			var interfaceList []interface{}
			if nil == err {
				for _, issue := range page {
					interfaceList = append(interfaceList, issue)
				}
			}
			return interfaceList, resp, err
		},
	)
	res := make([]*github.IssueComment, len(genericList))
	for i, elem := range genericList {
		res[i] = elem.(*github.IssueComment)
	}
	return res, err
}

// GetComment gets comment by comment ID
func (gc *GithubClient) GetComment(org, repo string, commentID int64) (*github.IssueComment, error) {
	var res *github.IssueComment
	_, err := gc.retry(
		fmt.Sprintf("getting comment '%s %s %d'", org, repo, commentID),
		maxRetryCount,
		func() (*github.Response, error) {
			var resp *github.Response
			var err error
			res, resp, err = gc.Client.Issues.GetComment(ctx, org, repo, commentID)
			return resp, err
		},
	)
	return res, err
}

// CreateComment adds comment to issue
func (gc *GithubClient) CreateComment(org, repo string, issueNumber int, commentBody string) (*github.IssueComment, error) {
	var res *github.IssueComment
	comment := &github.IssueComment{
		Body: &commentBody,
	}
	_, err := gc.retry(
		fmt.Sprintf("commenting issue '%s %s %d'", org, repo, issueNumber),
		maxRetryCount,
		func() (*github.Response, error) {
			var resp *github.Response
			var err error
			res, resp, err = gc.Client.Issues.CreateComment(ctx, org, repo, issueNumber, comment)
			return resp, err
		},
	)
	return res, err
}

// EditComment edits comment by replacing with provided comment
func (gc *GithubClient) EditComment(org, repo string, commentID int64, commentBody string) error {
	comment := &github.IssueComment{
		Body: &commentBody,
	}
	_, err := gc.retry(
		fmt.Sprintf("editing comment '%s %s %d'", org, repo, commentID),
		maxRetryCount,
		func() (*github.Response, error) {
			_, resp, err := gc.Client.Issues.EditComment(ctx, org, repo, commentID, comment)
			return resp, err
		},
	)
	return err
}

// DeleteComment deletes comment from issue
func (gc *GithubClient) DeleteComment(org, repo string, commentID int64) error {
	_, err := gc.retry(
		fmt.Sprintf("deleting comment '%s %s %d'", org, repo, commentID),
		maxRetryCount,
		func() (*github.Response, error) {
			resp, err := gc.Client.Issues.DeleteComment(ctx, org, repo, commentID)
			return resp, err
		},
	)
	return err
}

// AddLabelsToIssue adds label on issue
func (gc *GithubClient) AddLabelsToIssue(org, repo string, issueNumber int, labels []string) error {
	_, err := gc.retry(
		fmt.Sprintf("add labels '%v' to '%s %s %d'", labels, org, repo, issueNumber),
		maxRetryCount,
		func() (*github.Response, error) {
			_, resp, err := gc.Client.Issues.AddLabelsToIssue(ctx, org, repo, issueNumber, labels)
			return resp, err
		},
	)
	return err
}

// RemoveLabelForIssue removes given label for issue
func (gc *GithubClient) RemoveLabelForIssue(org, repo string, issueNumber int, label string) error {
	_, err := gc.retry(
		fmt.Sprintf("remove label '%s' from '%s %s %d'", label, org, repo, issueNumber),
		maxRetryCount,
		func() (*github.Response, error) {
			return gc.Client.Issues.RemoveLabelForIssue(ctx, org, repo, issueNumber, label)
		},
	)
	return err
}

func (gc *GithubClient) updateIssueState(org, repo string, state IssueStateEnum, issueNumber int) error {
	stateString := string(state)
	issueRequest := &github.IssueRequest{
		State: &stateString,
	}
	_, err := gc.retry(
		fmt.Sprintf("applying '%s' action on issue '%s %s %d'", stateString, org, repo, issueNumber),
		maxRetryCount,
		func() (*github.Response, error) {
			_, resp, err := gc.Client.Issues.Edit(ctx, org, repo, issueNumber, issueRequest)
			return resp, err
		},
	)
	return err
}
