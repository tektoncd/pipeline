// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// WebhookKind is the kind of webhook event represented
type WebhookKind string

const (
	// WebhookKindBranch is for branch events
	WebhookKindBranch WebhookKind = "branch"
	// WebhookKindCheckRun is for check run events
	WebhookKindCheckRun WebhookKind = "check_run"
	// WebhookKindCheckSuite is for check suite events
	WebhookKindCheckSuite WebhookKind = "check_suite"
	// WebhookKindDeploy is for deploy events
	WebhookKindDeploy WebhookKind = "deploy"
	// WebhookKindDeploymentStatus is for deployment status events
	WebhookKindDeploymentStatus WebhookKind = "deployment_status"
	// WebhookKindFork is for fork events
	WebhookKindFork WebhookKind = "fork"
	// WebhookKindInstallation is for app installation events
	WebhookKindInstallation WebhookKind = "installation"
	// WebhookKindInstallationRepository is for app isntallation in a repository events
	WebhookKindInstallationRepository WebhookKind = "installation_repository"
	// WebhookKindIssue is for issue events
	WebhookKindIssue WebhookKind = "issue"
	// WebhookKindIssueComment is for issue comment events
	WebhookKindIssueComment WebhookKind = "issue_comment"
	// WebhookKindLabel is for label events
	WebhookKindLabel WebhookKind = "label"
	// WebhookKindPing is for ping events
	WebhookKindPing WebhookKind = "ping"
	// WebhookKindPullRequest is for pull request events
	WebhookKindPullRequest WebhookKind = "pull_request"
	// WebhookKindPullRequestComment is for pull request comment events
	WebhookKindPullRequestComment WebhookKind = "pull_request_comment"
	// WebhookKindPush is for push events
	WebhookKindPush WebhookKind = "push"
	// WebhookKindRelease is for release events
	WebhookKindRelease WebhookKind = "release"
	// WebhookKindRepository is for repository events
	WebhookKindRepository WebhookKind = "repository"
	// WebhookKindReview is for review events
	WebhookKindReview WebhookKind = "review"
	// WebhookKindReviewCommentHook is for review comment events
	WebhookKindReviewCommentHook WebhookKind = "review_comment"
	// WebhookKindStar is for star events
	WebhookKindStar WebhookKind = "star"
	// WebhookKindStatus is for status events
	WebhookKindStatus WebhookKind = "status"
	// WebhookKindTag is for tag events
	WebhookKindTag WebhookKind = "tag"
	// WebhookKindWatch is for watch events
	WebhookKindWatch WebhookKind = "watch"
)

var (
	// ErrSignatureInvalid is returned when the webhook
	// signature is invalid or cannot be calculated.
	ErrSignatureInvalid = errors.New("Invalid webhook signature")
)

type (
	// Webhook defines a webhook for repository events.
	Webhook interface {
		Repository() Repository
		GetInstallationRef() *InstallationRef
		Kind() WebhookKind
	}

	// Label on a PR
	Label struct {
		ID          int64
		URL         string
		Name        string
		Description string
		Color       string
	}

	// PingHook a ping webhook.
	PingHook struct {
		Repo         Repository
		Sender       User
		Installation *InstallationRef
		GUID         string
	}

	// PushCommit represents general info about a commit.
	PushCommit struct {
		ID       string
		Message  string
		Added    []string
		Removed  []string
		Modified []string
	}

	// PushHook represents a push hook, eg push events.
	PushHook struct {
		Ref          string
		BaseRef      string
		Repo         Repository
		Before       string
		After        string
		Created      bool
		Deleted      bool
		Forced       bool
		Compare      string
		Commits      []PushCommit
		Commit       Commit
		Sender       User
		GUID         string
		Installation *InstallationRef
	}

	// BranchHook represents a branch or tag event,
	// eg create and delete github event types.
	BranchHook struct {
		Ref          Reference
		Repo         Repository
		Action       Action
		Sender       User
		Installation *InstallationRef
	}

	// CheckRunHook represents a check run event
	CheckRunHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// CheckSuiteHook represents a check suite event
	CheckSuiteHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// DeployHook represents a deployment event.
	// This is currently a GitHub-specific event type.
	DeployHook struct {
		Deployment   Deployment
		Action       Action
		Ref          Reference
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// DeploymentStatusHook represents a deployment status event.
	// This is currently a GitHub-specific event type.
	DeploymentStatusHook struct {
		Deployment       Deployment
		DeploymentStatus DeploymentStatus
		Action           Action
		Repo             Repository
		Sender           User
		Label            Label
		Installation     *InstallationRef
	}

	// ForkHook represents a fork event
	ForkHook struct {
		Repo         Repository
		Sender       User
		Installation *InstallationRef
	}

	// TagHook represents a tag event, eg create and delete
	// github event types.
	TagHook struct {
		Ref          Reference
		Repo         Repository
		Action       Action
		Sender       User
		Installation *InstallationRef
	}

	// IssueHook represents an issue event, eg issues.
	IssueHook struct {
		Action       Action
		Repo         Repository
		Issue        Issue
		Sender       User
		Installation *InstallationRef
	}

	// IssueCommentHook represents an issue comment event,
	// eg issue_comment.
	IssueCommentHook struct {
		Action       Action
		Repo         Repository
		Issue        Issue
		Comment      Comment
		Sender       User
		GUID         string
		Installation *InstallationRef
	}

	// InstallationHook represents an installation of a GitHub App
	InstallationHook struct {
		Action       Action
		Repos        []*Repository
		Sender       User
		Installation *Installation
	}

	// InstallationRepositoryHook represents an installation of a GitHub App
	InstallationRepositoryHook struct {
		Action              Action
		RepositorySelection string
		ReposAdded          []*Repository
		ReposRemoved        []*Repository
		Sender              User
		Installation        *Installation
	}

	// InstallationRef references a GitHub app install on a webhook
	InstallationRef struct {
		ID     int64
		NodeID string
	}

	// LabelHook represents a label event
	LabelHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// ReleaseHook represents a release event
	ReleaseHook struct {
		Action       Action
		Repo         Repository
		Release      Release
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// RepositoryHook represents a repository event
	RepositoryHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Installation *InstallationRef
	}

	// StatusHook represents a status event
	StatusHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
	}

	// Account represents the account of a GitHub app install
	Account struct {
		ID    int
		Login string
		Link  string
	}

	// PullRequestHookBranchFrom represents the branch or ref a PR is from
	PullRequestHookBranchFrom struct {
		From string
	}

	// PullRequestHookBranch represents a branch in a PR
	PullRequestHookBranch struct {
		Ref  PullRequestHookBranchFrom
		Sha  PullRequestHookBranchFrom
		Repo Repository
	}

	// PullRequestHookChanges represents the changes in a PR
	PullRequestHookChanges struct {
		Base PullRequestHookBranch
	}

	// PullRequestHook represents an pull request event,
	// eg pull_request.
	PullRequestHook struct {
		Action       Action
		Repo         Repository
		Label        Label
		PullRequest  PullRequest
		Sender       User
		Changes      PullRequestHookChanges
		GUID         string
		Installation *InstallationRef
	}

	// PullRequestCommentHook represents an pull request
	// comment event, eg pull_request_comment.
	PullRequestCommentHook struct {
		Action       Action
		Repo         Repository
		PullRequest  PullRequest
		Comment      Comment
		Sender       User
		GUID         string
		Installation *InstallationRef
	}

	// ReviewCommentHook represents a pull request review
	// comment, eg pull_request_review_comment.
	ReviewCommentHook struct {
		Action       Action
		Repo         Repository
		PullRequest  PullRequest
		Review       Review
		Installation *InstallationRef
	}

	// WatchHook represents a watch event. This is currently GitHub-specific.
	WatchHook struct {
		Action       string
		Repo         Repository
		Sender       User
		Installation *InstallationRef
	}

	// StarHook represents a star event. This is currently GitHub-specific.
	StarHook struct {
		Action    Action
		StarredAt time.Time
		Repo      Repository
		Sender    User
	}

	// WebhookWrapper lets us parse any webhook
	WebhookWrapper struct {
		PingHook                   *PingHook                   `json:",omitempty"`
		PushHook                   *PushHook                   `json:",omitempty"`
		BranchHook                 *BranchHook                 `json:",omitempty"`
		CheckRunHook               *CheckRunHook               `json:",omitempty"`
		CheckSuiteHook             *CheckSuiteHook             `json:",omitempty"`
		DeployHook                 *DeployHook                 `json:",omitempty"`
		DeploymentStatusHook       *DeploymentStatusHook       `json:",omitempty"`
		ForkHook                   *ForkHook                   `json:",omitempty"`
		TagHook                    *TagHook                    `json:",omitempty"`
		IssueHook                  *IssueHook                  `json:",omitempty"`
		IssueCommentHook           *IssueCommentHook           `json:",omitempty"`
		InstallationHook           *InstallationHook           `json:",omitempty"`
		InstallationRepositoryHook *InstallationRepositoryHook `json:",omitempty"`
		LabelHook                  *LabelHook                  `json:",omitempty"`
		ReleaseHook                *ReleaseHook                `json:",omitempty"`
		RepositoryHook             *RepositoryHook             `json:",omitempty"`
		PullRequestHook            *PullRequestHook            `json:",omitempty"`
		PullRequestCommentHook     *PullRequestCommentHook     `json:",omitempty"`
		ReviewCommentHook          *ReviewCommentHook          `json:",omitempty"`
		WatchHook                  *WatchHook                  `json:",omitempty"`
		StarHook                   *StarHook                   `json:",omitempty"`
	}

	// SecretFunc provides the Webhook parser with the
	// secret key used to validate webhook authenticity.
	SecretFunc func(webhook Webhook) (string, error)

	// WebhookService provides abstract functions for
	// parsing and validating webhooks requests.
	WebhookService interface {
		// Parse returns the parsed the repository webhook payload.
		Parse(req *http.Request, fn SecretFunc) (Webhook, error)
	}
)

// Kind returns the kind of webhook
func (h *PingHook) Kind() WebhookKind { return WebhookKindPing }

// Kind returns the kind of webhook
func (h *PushHook) Kind() WebhookKind { return WebhookKindPush }

// Kind returns the kind of webhook
func (h *BranchHook) Kind() WebhookKind { return WebhookKindBranch }

// Kind returns the kind of webhook
func (h *DeployHook) Kind() WebhookKind { return WebhookKindDeploy }

// Kind returns the kind of webhook
func (h *TagHook) Kind() WebhookKind { return WebhookKindTag }

// Kind returns the kind of webhook
func (h *IssueHook) Kind() WebhookKind { return WebhookKindIssue }

// Kind returns the kind of webhook
func (h *IssueCommentHook) Kind() WebhookKind { return WebhookKindIssueComment }

// Kind returns the kind of webhook
func (h *PullRequestHook) Kind() WebhookKind { return WebhookKindPullRequest }

// Kind returns the kind of webhook
func (h *PullRequestCommentHook) Kind() WebhookKind { return WebhookKindPullRequestComment }

// Kind returns the kind of webhook
func (h *ReviewHook) Kind() WebhookKind { return WebhookKindReview }

// Kind returns the kind of webhook
func (h *ReviewCommentHook) Kind() WebhookKind { return WebhookKindReviewCommentHook }

// Kind returns the kind of webhook
func (h *InstallationHook) Kind() WebhookKind { return WebhookKindInstallation }

// Kind returns the kind of webhook
func (h *LabelHook) Kind() WebhookKind { return WebhookKindLabel }

// Kind returns the kind of webhook
func (h *StatusHook) Kind() WebhookKind { return WebhookKindStatus }

// Kind returns the kind of webhook
func (h *CheckRunHook) Kind() WebhookKind { return WebhookKindCheckRun }

// Kind returns the kind of webhook
func (h *CheckSuiteHook) Kind() WebhookKind { return WebhookKindCheckSuite }

// Kind returns the kind of webhook
func (h *DeploymentStatusHook) Kind() WebhookKind { return WebhookKindDeploymentStatus }

// Kind returns the kind of webhook
func (h *ReleaseHook) Kind() WebhookKind { return WebhookKindRelease }

// Kind returns the kind of webhook
func (h *RepositoryHook) Kind() WebhookKind { return WebhookKindRepository }

// Kind returns the kind of webhook
func (h *ForkHook) Kind() WebhookKind { return WebhookKindFork }

// Kind returns the kind of webhook
func (h *InstallationRepositoryHook) Kind() WebhookKind { return WebhookKindInstallationRepository }

// Kind returns the kind of webhook
func (h *WatchHook) Kind() WebhookKind { return WebhookKindWatch }

// Kind returns the kind of webhook
func (h *StarHook) Kind() WebhookKind { return WebhookKindStar }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *PingHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *PushHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *BranchHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *DeployHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *TagHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *IssueHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *IssueCommentHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *PullRequestHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *PullRequestCommentHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *ReviewCommentHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *ReviewHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *LabelHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *StatusHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *CheckRunHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *CheckSuiteHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *DeploymentStatusHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *ReleaseHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *RepositoryHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *ForkHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *WatchHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *StarHook) Repository() Repository { return h.Repo }

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *InstallationHook) Repository() Repository {
	if len(h.Repos) > 0 {
		return *h.Repos[0]
	}
	return Repository{}
}

// Repository defines the repository webhook and provides a convenient way to get the associated repository without
// having to cast the type.
func (h *InstallationRepositoryHook) Repository() Repository {
	if len(h.ReposAdded) > 0 {
		return *h.ReposAdded[0]
	}
	return Repository{}
}

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *PingHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *PushHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *BranchHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *DeployHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *TagHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *IssueHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *IssueCommentHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *PullRequestHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *PullRequestCommentHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *ReviewHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *ReviewCommentHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *LabelHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *StatusHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *CheckRunHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *CheckSuiteHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *DeploymentStatusHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *ReleaseHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *RepositoryHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *ForkHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *WatchHook) GetInstallationRef() *InstallationRef { return h.Installation }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *StarHook) GetInstallationRef() *InstallationRef { return nil }

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *InstallationHook) GetInstallationRef() *InstallationRef {
	if h.Installation == nil {
		return nil
	}
	return &InstallationRef{
		ID: h.Installation.ID,
	}
}

// GetInstallationRef returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *InstallationRepositoryHook) GetInstallationRef() *InstallationRef {
	if h.Installation == nil {
		return nil
	}
	return &InstallationRef{
		ID: h.Installation.ID,
	}
}

// ToWebhook converts the webhook wrapper to a webhook
func (h *WebhookWrapper) ToWebhook() (Webhook, error) {
	if h == nil {
		return nil, fmt.Errorf("no webhook supplied")
	}
	if h.PingHook != nil {
		return h.PingHook, nil
	}
	if h.PushHook != nil {
		return h.PushHook, nil
	}
	if h.BranchHook != nil {
		return h.BranchHook, nil
	}
	if h.CheckRunHook != nil {
		return h.CheckRunHook, nil
	}
	if h.CheckSuiteHook != nil {
		return h.CheckSuiteHook, nil
	}
	if h.DeployHook != nil {
		return h.DeployHook, nil
	}
	if h.DeploymentStatusHook != nil {
		return h.DeploymentStatusHook, nil
	}
	if h.ForkHook != nil {
		return h.ForkHook, nil
	}
	if h.TagHook != nil {
		return h.TagHook, nil
	}
	if h.IssueHook != nil {
		return h.IssueHook, nil
	}
	if h.IssueCommentHook != nil {
		return h.IssueCommentHook, nil
	}
	if h.InstallationHook != nil {
		return h.InstallationHook, nil
	}
	if h.InstallationRepositoryHook != nil {
		return h.InstallationRepositoryHook, nil
	}
	if h.LabelHook != nil {
		return h.LabelHook, nil
	}
	if h.RepositoryHook != nil {
		return h.RepositoryHook, nil
	}
	if h.PullRequestHook != nil {
		return h.PullRequestHook, nil
	}
	if h.PullRequestCommentHook != nil {
		return h.PullRequestCommentHook, nil
	}
	if h.ReviewCommentHook != nil {
		return h.ReviewCommentHook, nil
	}
	if h.WatchHook != nil {
		return h.WatchHook, nil
	}
	if h.StarHook != nil {
		return h.StarHook, nil
	}
	return nil, fmt.Errorf("unsupported webhook")
}
