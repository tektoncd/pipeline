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
		Link           string
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
		Milestone      Milestone
		Created        time.Time
		Updated        time.Time
	}

	// Milestone the milestotne
	Milestone struct {
		Number      int
		ID          int
		Title       string
		Description string
		Link        string
		State       string
	}

	// PullRequestListOptions provides options for querying
	// a list of repository merge requests.
	PullRequestListOptions struct {
		Page   int
		Size   int
		Open   bool
		Closed bool
	}

	// PullRequestBranch contains information about a particular branch in a PR.
	PullRequestBranch struct {
		Ref  string
		Sha  string
		Repo Repository
	}

	// Change represents a changed file.
	Change struct {
		Path      string
		Added     bool
		Renamed   bool
		Deleted   bool
		Additions int
		Deletions int
		Changes   int
		BlobURL   string
		Sha       string
	}

	// PullRequestService provides access to pull request resources.
	PullRequestService interface {
		// Find returns the repository pull request by number.
		Find(context.Context, string, int) (*PullRequest, *Response, error)

		// FindComment returns the pull request comment by id.
		FindComment(context.Context, string, int, int) (*Comment, *Response, error)

		// Find returns the repository pull request list.
		List(context.Context, string, PullRequestListOptions) ([]*PullRequest, *Response, error)

		// ListChanges returns the pull request changeset.
		ListChanges(context.Context, string, int, ListOptions) ([]*Change, *Response, error)

		// ListComments returns the pull request comment list.
		ListComments(context.Context, string, int, ListOptions) ([]*Comment, *Response, error)

		// ListLabels returns the labels on a pull request
		ListLabels(context.Context, string, int, ListOptions) ([]*Label, *Response, error)

		// Merge merges the repository pull request.
		Merge(context.Context, string, int) (*Response, error)

		// Close closes the repository pull request.
		Close(context.Context, string, int) (*Response, error)

		// CreateComment creates a new pull request comment.
		CreateComment(context.Context, string, int, *CommentInput) (*Comment, *Response, error)

		// DeleteComment deletes an pull request comment.
		DeleteComment(context.Context, string, int, int) (*Response, error)

		// AddLabel adds a label to a pull request.
		AddLabel(ctx context.Context, repo string, number int, label string) (*Response, error)

		// DeleteLabel deletes a label from a pull request
		DeleteLabel(ctx context.Context, repo string, number int, label string) (*Response, error)
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
	case "clean", "mergeable":
		return MergeableStateMergeable
	case "conflict", "conflicting":
		return MergeableStateConflicting
	default:
		return MergeableStateUnknown
	}
}

// String returns the string representation
func (s MergeableState) String() string {
	return string(s)
}
