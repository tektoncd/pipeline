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

package github

import (
	"fmt"
	"time"

	"github.com/google/go-github/github"

	"knative.dev/pkg/test/ghutil"
	"knative.dev/pkg/test/mako/alerter"
)

const (
	// perfLabel is the Github issue label used for querying all auto-generated performance issues.
	perfLabel         = "auto:perf"
	daysConsideredOld = 10 // arbitrary number of days for an issue to be considered old

	// issueTitleTemplate is a template for issue title
	issueTitleTemplate = "[performance] %s"

	// issueBodyTemplate is a template for issue body
	issueBodyTemplate = `
### Auto-generated issue tracking performance regression
* **Test name**: %s`

	// reopenIssueCommentTemplate is a template for the comment of an issue that is reopened
	reopenIssueCommentTemplate = `
New regression has been detected, reopening this issue:
%s`

	// newIssueCommentTemplate is a template for the comment of an issue that has been quiet for a long time
	newIssueCommentTemplate = `
A new regression for this test has been detected:
%s`

	// closeIssueComment is the comment of an issue when we close it
	closeIssueComment = `
The performance regression goes way for this test, closing this issue.`
)

// issueHandler handles methods for github issues
type issueHandler struct {
	client ghutil.GithubOperations
	config config
}

// config is the global config that can be used in Github operations
type config struct {
	org    string
	repo   string
	dryrun bool
}

// Setup creates the necessary setup to make calls to work with github issues
func Setup(githubToken string, config config) (*issueHandler, error) {
	ghc, err := ghutil.NewGithubClient(githubToken)
	if err != nil {
		return nil, fmt.Errorf("cannot authenticate to github: %v", err)
	}
	return &issueHandler{client: ghc, config: config}, nil
}

// CreateIssueForTest will try to add an issue with the given testName and description.
// If there is already an issue related to the test, it will try to update that issue.
func (gih *issueHandler) CreateIssueForTest(testName, desc string) error {
	org := gih.config.org
	repo := gih.config.repo
	dryrun := gih.config.dryrun
	title := fmt.Sprintf(issueTitleTemplate, testName)
	issue := gih.findIssue(org, repo, title, dryrun)
	// If the issue hasn't been created, create one
	if issue == nil {
		body := fmt.Sprintf(issueBodyTemplate, testName)
		if err := gih.createNewIssue(org, repo, title, body, dryrun); err != nil {
			return err
		}
		comment := fmt.Sprintf(newIssueCommentTemplate, desc)
		if err := gih.addComment(org, repo, *issue.Number, comment, dryrun); err != nil {
			return err
		}
		// If one issue with the same title has been closed, reopen it and add new comment
	} else if *issue.State == string(ghutil.IssueCloseState) {
		if err := gih.reopenIssue(org, repo, *issue.Number, dryrun); err != nil {
			return err
		}
		comment := fmt.Sprintf(reopenIssueCommentTemplate, desc)
		if err := gih.addComment(org, repo, *issue.Number, comment, dryrun); err != nil {
			return err
		}
	} else {
		// If the issue hasn't been updated for a long time, add a new comment
		if time.Now().Sub(*issue.UpdatedAt) > daysConsideredOld*24*time.Hour {
			comment := fmt.Sprintf(newIssueCommentTemplate, desc)
			// TODO(Fredy-Z): edit the old comment instead of adding a new one, like flaky-test-reporter
			if err := gih.addComment(org, repo, *issue.Number, comment, dryrun); err != nil {
				return err
			}
		}
	}

	return nil
}

// createNewIssue will create a new issue, and add perfLabel for it.
func (gih *issueHandler) createNewIssue(org, repo, title, body string, dryrun bool) error {
	var newIssue *github.Issue
	if err := alerter.Run(
		"creating issue",
		func() error {
			var err error
			newIssue, err = gih.client.CreateIssue(org, repo, title, body)
			return err
		},
		dryrun,
	); nil != err {
		return err
	}
	return alerter.Run(
		"adding perf label",
		func() error {
			return gih.client.AddLabelsToIssue(org, repo, *newIssue.Number, []string{perfLabel})
		},
		dryrun,
	)
}

// CloseIssueForTest will try to close the issue for the given testName.
// If there is no issue related to the test or the issue is already closed, the function will do nothing.
func (gih *issueHandler) CloseIssueForTest(testName string) error {
	org := gih.config.org
	repo := gih.config.repo
	dryrun := gih.config.dryrun
	title := fmt.Sprintf(issueTitleTemplate, testName)
	issue := gih.findIssue(org, repo, title, dryrun)
	if issue == nil || *issue.State == string(ghutil.IssueCloseState) {
		return nil
	}

	issueNumber := *issue.Number
	if err := alerter.Run(
		"add comment for the issue to close",
		func() error {
			_, cErr := gih.client.CreateComment(org, repo, issueNumber, closeIssueComment)
			return cErr
		},
		dryrun,
	); err != nil {
		return err
	}
	return alerter.Run(
		"closing issue",
		func() error {
			return gih.client.CloseIssue(org, repo, issueNumber)
		},
		dryrun,
	)
}

// reopenIssue will reopen the given issue.
func (gih *issueHandler) reopenIssue(org, repo string, issueNumber int, dryrun bool) error {
	return alerter.Run(
		"reopen the issue",
		func() error {
			return gih.client.ReopenIssue(org, repo, issueNumber)
		},
		dryrun,
	)
}

// findIssue will return the issue in the given repo if it exists.
func (gih *issueHandler) findIssue(org, repo, title string, dryrun bool) *github.Issue {
	var issues []*github.Issue
	alerter.Run(
		"list issues in the repo",
		func() error {
			var err error
			issues, err = gih.client.ListIssuesByRepo(org, repo, []string{perfLabel})
			return err
		},
		dryrun,
	)
	for _, issue := range issues {
		if *issue.Title == title {
			return issue
		}
	}
	return nil
}

// addComment will add comment for the given issue.
func (gih *issueHandler) addComment(org, repo string, issueNumber int, commentBody string, dryrun bool) error {
	return alerter.Run(
		"add comment for issue",
		func() error {
			_, err := gih.client.CreateComment(org, repo, issueNumber, commentBody)
			return err
		},
		dryrun,
	)
}
