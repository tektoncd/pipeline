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

// client.go defines clients, and provide helpers overcome Github rate limit errors

package ghutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	maxRetryCount = 5
	tokenReserve  = 50
)

var (
	ctx = context.Background()
)

// GithubOperations contains a set of functions for Github operations
type GithubOperations interface {
	GetGithubUser() (*github.User, error)
	ListRepos(org string) ([]string, error)
	ListIssuesByRepo(org, repo string, labels []string) ([]*github.Issue, error)
	CreateIssue(org, repo, title, body string) (*github.Issue, error)
	CloseIssue(org, repo string, issueNumber int) error
	ReopenIssue(org, repo string, issueNumber int) error
	ListComments(org, repo string, issueNumber int) ([]*github.IssueComment, error)
	GetComment(org, repo string, commentID int64) (*github.IssueComment, error)
	CreateComment(org, repo string, issueNumber int, commentBody string) (*github.IssueComment, error)
	EditComment(org, repo string, commentID int64, commentBody string) error
	DeleteComment(org, repo string, commentID int64) error
	AddLabelsToIssue(org, repo string, issueNumber int, labels []string) error
	RemoveLabelForIssue(org, repo string, issueNumber int, label string) error
	GetPullRequest(org, repo string, ID int) (*github.PullRequest, error)
	GetPullRequestByCommitID(org, repo, commitID string) (*github.PullRequest, error)
	EditPullRequest(org, repo string, ID int, title, body string) (*github.PullRequest, error)
	ListPullRequests(org, repo, head, base string) ([]*github.PullRequest, error)
	ListCommits(org, repo string, ID int) ([]*github.RepositoryCommit, error)
	ListFiles(org, repo string, ID int) ([]*github.CommitFile, error)
	CreatePullRequest(org, repo, head, base, title, body string) (*github.PullRequest, error)
}

// GithubClient provides methods to perform github operations
// It implements all functions in GithubOperations
type GithubClient struct {
	Client *github.Client
}

// NewGithubClient explicitly authenticates to github with giving token and returns a handle
func NewGithubClient(tokenFilePath string) (*GithubClient, error) {
	b, err := ioutil.ReadFile(tokenFilePath)
	if err != nil {
		return nil, err
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: strings.TrimSpace(string(b))},
	)

	return &GithubClient{github.NewClient(oauth2.NewClient(ctx, ts))}, nil
}

// GetGithubUser gets current authenticated user
func (gc *GithubClient) GetGithubUser() (*github.User, error) {
	var res *github.User
	_, err := gc.retry(
		"getting current user",
		maxRetryCount,
		func() (*github.Response, error) {
			var resp *github.Response
			var err error
			res, resp, err = gc.Client.Users.Get(ctx, "")
			return resp, err
		},
	)
	return res, err
}

func (gc *GithubClient) waitForRateReset(r *github.Rate) {
	if r.Remaining <= tokenReserve {
		sleepDuration := time.Until(r.Reset.Time) + (time.Second * 10)
		if sleepDuration > 0 {
			log.Printf("--Rate Limiting-- GitHub tokens reached minimum reserve %d. Sleeping %ds until reset.\n", tokenReserve, sleepDuration)
			time.Sleep(sleepDuration)
		}
	}
}

// Github API has a rate limit, retry waits until rate limit reset if request failed with RateLimitError,
// then retry maxRetries times until succeed
func (gc *GithubClient) retry(message string, maxRetries int, call func() (*github.Response, error)) (*github.Response, error) {
	var err error
	var resp *github.Response

	for retryCount := 0; retryCount <= maxRetries; retryCount++ {
		if resp, err = call(); nil == err {
			return resp, nil
		}
		switch err := err.(type) {
		case *github.RateLimitError:
			gc.waitForRateReset(&err.Rate)
		default:
			return resp, err
		}
		log.Printf("error %s: %v. Will retry.\n", message, err)
	}
	return resp, err
}

// depaginate adds depagination on top of the retry, in case list exceeds rate limit
func (gc *GithubClient) depaginate(message string, maxRetries int, options *github.ListOptions, call func() ([]interface{}, *github.Response, error)) ([]interface{}, error) {
	var allItems []interface{}
	wrapper := func() (*github.Response, error) {
		items, resp, err := call()
		if err == nil {
			allItems = append(allItems, items...)
		}
		return resp, err
	}

	options.Page = 1
	options.PerPage = 100
	lastPage := 1
	for ; options.Page <= lastPage; options.Page++ {
		resp, err := gc.retry(message, maxRetries, wrapper)
		if err != nil {
			return allItems, fmt.Errorf("error while depaginating page %d/%d: %v", options.Page, lastPage, err)
		}
		if resp.LastPage > 0 {
			lastPage = resp.LastPage
		}
	}
	return allItems, nil
}
