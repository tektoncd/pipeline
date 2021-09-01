// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"time"
)

// EmptyCommit is an empty commit sha.
const EmptyCommit = "0000000000000000000000000000000000000000"

type (
	// Reference represents a git reference.
	Reference struct {
		Name string
		Path string
		Sha  string
	}

	// CommitTree represents a commit tree
	CommitTree struct {
		Sha  string
		Link string
	}

	// Commit represents a repository commit.
	Commit struct {
		Sha       string
		Message   string
		Tree      CommitTree
		Author    Signature
		Committer Signature
		Link      string
	}

	// CommitListOptions provides options for querying a
	// list of repository commits.
	CommitListOptions struct {
		Ref  string
		Sha  string
		Page int
		Size int
	}

	// Signature identifies a git commit creator.
	Signature struct {
		Name  string
		Email string
		Date  time.Time

		// Fields are optional. The provider may choose to
		// include account information in the response.
		Login  string
		Avatar string
	}

	// GitService provides access to git resources.
	GitService interface {
		// FindBranch finds a git branch by name.
		FindBranch(ctx context.Context, repo, name string) (*Reference, *Response, error)

		// FindCommit finds a git commit by ref.
		FindCommit(ctx context.Context, repo, ref string) (*Commit, *Response, error)

		// FindTag finds a git tag by name.
		FindTag(ctx context.Context, repo, name string) (*Reference, *Response, error)

		// ListBranches returns a list of git branches.
		ListBranches(ctx context.Context, repo string, opts ListOptions) ([]*Reference, *Response, error)

		// ListCommits returns a list of git commits.
		ListCommits(ctx context.Context, repo string, opts CommitListOptions) ([]*Commit, *Response, error)

		// ListChanges returns the changeset between a commit and its parent.
		ListChanges(ctx context.Context, repo, ref string, opts ListOptions) ([]*Change, *Response, error)

		// ListChanges returns the changeset between two commits.
		CompareCommits(ctx context.Context, repo, ref1, ref2 string, opts ListOptions) ([]*Change, *Response, error)

		// ListTags returns a list of git tags.
		ListTags(ctx context.Context, repo string, opts ListOptions) ([]*Reference, *Response, error)

		// FindRef returns the SHA of the given ref, such as "heads/master".
		FindRef(ctx context.Context, repo, ref string) (string, *Response, error)

		// DeleteRef deletes the given ref
		DeleteRef(ctx context.Context, repo, ref string) (*Response, error)

		// CreateRef creates a new ref
		CreateRef(ctx context.Context, repo, ref, sha string) (*Reference, *Response, error)
	}
)
