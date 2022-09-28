// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"strings"
	"time"
)

type (
	// MergeableState represents whether the PR can be merged
	MergeableState string

	// PullRequest represents a repository pull request.
	PullRequest struct {
		Number         int
		Title          string
		Body           string
		Labels         []*Label
		Sha            string
		Ref            string
		Source         string
		Target         string
		Base           PullRequestBranch
		Head           PullRequestBranch
		Fork           string
		State          string
		Closed         bool
		Draft          bool
		Merged         bool
		Mergeable      bool
		Rebaseable     bool
		MergeableState MergeableState
		MergeSha       string
		Author         User
		Assignees      []User
		Reviewers      []User
		Milestone      Milestone
		Created        time.Time
		Updated        time.Time

		// Link links to the main pull request page
		Link string

		// DiffLink links to the diff report of a pull request
		DiffLink string
	}

	// PullRequestInput provides the input needed to create or update a PR.
	PullRequestInput struct {
		Title string
		Head  string
		Base  string
		Body  string
	}

	// Milestone the milestone
	Milestone struct {
		Number      int
		ID          int
		Title       string
		Description string
		Link        string
		State       string
		DueDate     *time.Time
	}

	// PullRequestListOptions provides options for querying
	// a list of repository merge requests.
	PullRequestListOptions struct {
		Page          int
		Size          int
		Open          bool
		Closed        bool
		Labels        []string
		UpdatedAfter  *time.Time
		UpdatedBefore *time.Time
		CreatedAfter  *time.Time
		CreatedBefore *time.Time
	}

	// PullRequestBranch contains information about a particular branch in a PR.
	PullRequestBranch struct {
		Ref  string
		Sha  string
		Repo Repository
	}

	// Change represents a changed file.
	Change struct {
		Path         string
		PreviousPath string
		Added        bool
		Renamed      bool
		Deleted      bool
		Patch        string
		Additions    int
		Deletions    int
		Changes      int
		BlobURL      string
		Sha          string
	}

	// PullRequestMergeOptions lets you define how a pull request will be merged.
	PullRequestMergeOptions struct {
		CommitTitle string // Extra detail to append to automatic commit message. (Optional.)
		SHA         string // SHA that pull request head must match to allow merge. (Optional.)

		// The merge method to use. Possible values include: "merge", "squash", and "rebase" with the default being merge. (Optional.)
		MergeMethod string

		// Merge automatically once the pipeline completes. (Supported only in gitlab)
		MergeWhenPipelineSucceeds bool

		// Signals to the SCM to remove the source branch during merge
		DeleteSourceBranch bool
	}

	// PullRequestService provides access to pull request resources.
	PullRequestService interface {
		// Find returns the repository pull request by number.
		Find(context.Context, string, int) (*PullRequest, *Response, error)

		// Update modifies an existing pull request.
		Update(context.Context, string, int, *PullRequestInput) (*PullRequest, *Response, error)

		// FindComment returns the pull request comment by id.
		FindComment(context.Context, string, int, int) (*Comment, *Response, error)

		// Find returns the repository pull request list.
		List(context.Context, string, *PullRequestListOptions) ([]*PullRequest, *Response, error)

		// ListChanges returns the pull request changeset.
		ListChanges(context.Context, string, int, *ListOptions) ([]*Change, *Response, error)

		// ListCommits returns the pull request commits.
		ListCommits(context.Context, string, int, *ListOptions) ([]*Commit, *Response, error)

		// ListComments returns the pull request comment list.
		ListComments(context.Context, string, int, *ListOptions) ([]*Comment, *Response, error)

		// ListLabels returns the labels on a pull request
		ListLabels(context.Context, string, int, *ListOptions) ([]*Label, *Response, error)

		// ListEvents returns the events creating and removing the labels on an pull request
		ListEvents(context.Context, string, int, *ListOptions) ([]*ListedIssueEvent, *Response, error)

		// Merge merges the repository pull request.
		Merge(context.Context, string, int, *PullRequestMergeOptions) (*Response, error)

		// Close closes the repository pull request.
		Close(context.Context, string, int) (*Response, error)

		// Reopen reopens a closed repository pull request.
		Reopen(context.Context, string, int) (*Response, error)

		// CreateComment creates a new pull request comment.
		CreateComment(context.Context, string, int, *CommentInput) (*Comment, *Response, error)

		// DeleteComment deletes an pull request comment.
		DeleteComment(context.Context, string, int, int) (*Response, error)

		// EditComment edits an existing pull request comment.
		EditComment(context.Context, string, int, int, *CommentInput) (*Comment, *Response, error)

		// AddLabel adds a label to a pull request.
		AddLabel(ctx context.Context, repo string, number int, label string) (*Response, error)

		// DeleteLabel deletes a label from a pull request
		DeleteLabel(ctx context.Context, repo string, number int, label string) (*Response, error)

		// AssignIssue assigns one or more  users to an issue
		AssignIssue(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// UnassignIssue removes the assignment of ne or more users on an issue
		UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// Create creates a new pull request in a repo.
		Create(context.Context, string, *PullRequestInput) (*PullRequest, *Response, error)

		// RequestReview adds one or more users as a reviewer on a pull request.
		RequestReview(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// UnrequestReview removes one or more users as a reviewer on a pull request.
		UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// SetMilestone adds a milestone to a pull request
		SetMilestone(ctx context.Context, repo string, prID int, number int) (*Response, error)

		// ClearMilestone removes the milestone from a pull request
		ClearMilestone(ctx context.Context, repo string, prID int) (*Response, error)
	}
)

// Action values.
const (
	// MergeableStateMergeable The pull request can be merged.
	MergeableStateMergeable MergeableState = "mergeable"
	// MergeableStateConflicting The pull request cannot be merged due to merge conflicts.
	MergeableStateConflicting MergeableState = "conflicting"
	// MergeableStateUnknown The mergeability of the pull request is still being calculated.
	MergeableStateUnknown MergeableState = ""
)

// Repository returns the base repository where the PR will merge to
func (pr *PullRequest) Repository() Repository {
	return pr.Base.Repo
}

// ToMergeableState converts the given string to a mergeable state
func ToMergeableState(text string) MergeableState {
	switch strings.ToLower(text) {
	case "clean", "mergeable", "can_be_merged":
		return MergeableStateMergeable
	case "conflict", "conflicting", "cannot_be_merged":
		return MergeableStateConflicting
	default:
		return MergeableStateUnknown
	}
}

// String returns the string representation
func (s MergeableState) String() string {
	return string(s)
}
