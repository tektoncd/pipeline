// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

func encode(s string) string {
	return strings.Replace(s, "/", "%2F", -1)
}

func encodeListOptions(opts *scm.ListOptions) string {
	params := url.Values{}
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	if opts.From != "" {
		params.Set("from", opts.From)
	}
	if opts.To != "" {
		params.Set("to", opts.To)
	}
	if opts.Sort != "" {
		params.Set("sort", opts.Sort)
	}
	return params.Encode()
}

func encodeMemberListOptions(opts *scm.ListOptions) string {
	params := url.Values{}
	params.Set("membership", "true")
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	return params.Encode()
}

func encodeCommitListOptions(opts scm.CommitListOptions) string {
	params := url.Values{}
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	if opts.Ref != "" {
		params.Set("ref_name", opts.Ref)
	}
	return params.Encode()
}

func encodeIssueListOptions(opts scm.IssueListOptions) string {
	params := url.Values{}
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	if opts.Open && opts.Closed {
		params.Set("state", "all")
	} else if opts.Closed {
		params.Set("state", "closed")
	} else if opts.Open {
		params.Set("state", "opened")
	}
	return params.Encode()
}

func encodeMilestoneListOptions(opts scm.MilestoneListOptions) string {
	params := url.Values{}
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	if opts.Closed && !opts.Open {
		params.Set("state", "closed")
	} else if opts.Open && !opts.Closed {
		params.Set("state", "active")
	}
	return params.Encode()
}

func encodePullRequestListOptions(opts *scm.PullRequestListOptions) string {
	params := url.Values{}
	if opts.Page != 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Size != 0 {
		params.Set("per_page", strconv.Itoa(opts.Size))
	}
	if opts.Open && opts.Closed {
		params.Set("state", "all")
	} else if opts.Closed {
		params.Set("state", "closed")
	} else if opts.Open {
		params.Set("state", "opened")
	}
	if len(opts.Labels) > 0 {
		params.Set("labels", strings.Join(opts.Labels, ","))
	}
	if opts.CreatedAfter != nil {
		params.Set("created_after", opts.CreatedAfter.Format(scm.SearchTimeFormat))
	}
	if opts.CreatedBefore != nil {
		params.Set("created_before", opts.CreatedBefore.Format(scm.SearchTimeFormat))
	}
	if opts.UpdatedAfter != nil {
		params.Set("updated_after", opts.UpdatedAfter.Format(scm.SearchTimeFormat))
	}
	if opts.UpdatedBefore != nil {
		params.Set("updated_before", opts.UpdatedBefore.Format(scm.SearchTimeFormat))
	}
	return params.Encode()
}

func encodePullRequestMergeOptions(opts *scm.PullRequestMergeOptions) *pullRequestMergeRequest {
	var prRequest *pullRequestMergeRequest
	if opts != nil {
		prRequest = &pullRequestMergeRequest{}
		if opts.SHA != "" {
			prRequest.SHA = opts.SHA
		}
		switch opts.MergeMethod {
		case "squash":
			if opts.CommitTitle != "" {
				prRequest.SquashCommitMessage = opts.CommitTitle
			}
			prRequest.Squash = "true"
		default:
			if opts.CommitTitle != "" {
				prRequest.CommitMessage = opts.CommitTitle
			}
		}
		if opts.MergeWhenPipelineSucceeds {
			prRequest.MergeWhenPipelineSucceeds = "true"
		}
		if opts.DeleteSourceBranch {
			prRequest.RemoveSourceBranch = "true"
		}

	}
	return prRequest
}

func gitlabStateToSCMState(glState string) string {
	switch glState {
	case "opened":
		return "open"
	default:
		return "closed"
	}
}
