// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"errors"
	"net/http"
)

type WebhookKind string

const (
	WebhookKindBranch                 WebhookKind = "branch"
	WebhookKindCheckRun               WebhookKind = "check_run"
	WebhookKindCheckSuite             WebhookKind = "check_suite"
	WebhookKindDeploy                 WebhookKind = "deploy"
	WebhookKindDeploymentStatus       WebhookKind = "deployment_status"
	WebhookKindFork                   WebhookKind = "fork"
	WebhookKindInstallation           WebhookKind = "installation"
	WebhookKindInstallationRepository WebhookKind = "installation_repository"
	WebhookKindIssue                  WebhookKind = "issue"
	WebhookKindIssueComment           WebhookKind = "issue_comment"
	WebhookKindLabel                  WebhookKind = "label"
	WebhookKindPing                   WebhookKind = "ping"
	WebhookKindPullRequest            WebhookKind = "pull_request"
	WebhookKindPullRequestComment     WebhookKind = "pull_request_comment"
	WebhookKindPush                   WebhookKind = "push"
	WebhookKindRelease                WebhookKind = "release"
	WebhookKindRepository             WebhookKind = "repository"
	WebhookKindReviewCommentHook      WebhookKind = "review_comment"
	WebhookKindStatus                 WebhookKind = "status"
	WebhookKindTag                    WebhookKind = "tag"
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

	// DeploymentStatusHook represents a check suite event
	DeploymentStatusHook struct {
		Action       Action
		Repo         Repository
		Sender       User
		Label        Label
		Installation *InstallationRef
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

	PullRequestHookBranchFrom struct {
		From string
	}

	PullRequestHookBranch struct {
		Ref  PullRequestHookBranchFrom
		Sha  PullRequestHookBranchFrom
		Repo Repository
	}

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

	// DeployHook represents a deployment event. This is
	// currently a GitHub-specific event type.
	DeployHook struct {
		Data         interface{}
		Desc         string
		Ref          Reference
		Repo         Repository
		Sender       User
		Target       string
		TargetURL    string
		Task         string
		Installation *InstallationRef
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

// Kind() returns the kind of webhook

func (h *PingHook) Kind() WebhookKind                   { return WebhookKindPing }
func (h *PushHook) Kind() WebhookKind                   { return WebhookKindPush }
func (h *BranchHook) Kind() WebhookKind                 { return WebhookKindBranch }
func (h *DeployHook) Kind() WebhookKind                 { return WebhookKindDeploy }
func (h *TagHook) Kind() WebhookKind                    { return WebhookKindTag }
func (h *IssueHook) Kind() WebhookKind                  { return WebhookKindIssue }
func (h *IssueCommentHook) Kind() WebhookKind           { return WebhookKindIssueComment }
func (h *PullRequestHook) Kind() WebhookKind            { return WebhookKindPullRequest }
func (h *PullRequestCommentHook) Kind() WebhookKind     { return WebhookKindPullRequestComment }
func (h *ReviewCommentHook) Kind() WebhookKind          { return WebhookKindReviewCommentHook }
func (h *InstallationHook) Kind() WebhookKind           { return WebhookKindInstallation }
func (h *LabelHook) Kind() WebhookKind                  { return WebhookKindLabel }
func (h *StatusHook) Kind() WebhookKind                 { return WebhookKindStatus }
func (h *CheckRunHook) Kind() WebhookKind               { return WebhookKindCheckRun }
func (h *CheckSuiteHook) Kind() WebhookKind             { return WebhookKindCheckSuite }
func (h *DeploymentStatusHook) Kind() WebhookKind       { return WebhookKindDeploymentStatus }
func (h *ReleaseHook) Kind() WebhookKind                { return WebhookKindRelease }
func (h *RepositoryHook) Kind() WebhookKind             { return WebhookKindRepository }
func (h *ForkHook) Kind() WebhookKind                   { return WebhookKindFork }
func (h *InstallationRepositoryHook) Kind() WebhookKind { return WebhookKindInstallationRepository }

// Repository() defines the repository webhook and provides
// a convenient way to get the associated repository without
// having to cast the type.

func (h *PingHook) Repository() Repository               { return h.Repo }
func (h *PushHook) Repository() Repository               { return h.Repo }
func (h *BranchHook) Repository() Repository             { return h.Repo }
func (h *DeployHook) Repository() Repository             { return h.Repo }
func (h *TagHook) Repository() Repository                { return h.Repo }
func (h *IssueHook) Repository() Repository              { return h.Repo }
func (h *IssueCommentHook) Repository() Repository       { return h.Repo }
func (h *PullRequestHook) Repository() Repository        { return h.Repo }
func (h *PullRequestCommentHook) Repository() Repository { return h.Repo }
func (h *ReviewCommentHook) Repository() Repository      { return h.Repo }
func (h *LabelHook) Repository() Repository              { return h.Repo }
func (h *StatusHook) Repository() Repository             { return h.Repo }
func (h *CheckRunHook) Repository() Repository           { return h.Repo }
func (h *CheckSuiteHook) Repository() Repository         { return h.Repo }
func (h *DeploymentStatusHook) Repository() Repository   { return h.Repo }
func (h *ReleaseHook) Repository() Repository            { return h.Repo }
func (h *RepositoryHook) Repository() Repository         { return h.Repo }
func (h *ForkHook) Repository() Repository               { return h.Repo }

func (h *InstallationHook) Repository() Repository {
	if len(h.Repos) > 0 {
		return *h.Repos[0]
	}
	return Repository{}
}

func (h *InstallationRepositoryHook) Repository() Repository {
	if len(h.ReposAdded) > 0 {
		return *h.ReposAdded[0]
	}
	return Repository{}
}

// GetInstallationRef() returns the installation reference if the webhook is invoked on a
// GitHub App
func (h *PingHook) GetInstallationRef() *InstallationRef               { return h.Installation }
func (h *PushHook) GetInstallationRef() *InstallationRef               { return h.Installation }
func (h *BranchHook) GetInstallationRef() *InstallationRef             { return h.Installation }
func (h *DeployHook) GetInstallationRef() *InstallationRef             { return h.Installation }
func (h *TagHook) GetInstallationRef() *InstallationRef                { return h.Installation }
func (h *IssueHook) GetInstallationRef() *InstallationRef              { return h.Installation }
func (h *IssueCommentHook) GetInstallationRef() *InstallationRef       { return h.Installation }
func (h *PullRequestHook) GetInstallationRef() *InstallationRef        { return h.Installation }
func (h *PullRequestCommentHook) GetInstallationRef() *InstallationRef { return h.Installation }
func (h *ReviewCommentHook) GetInstallationRef() *InstallationRef      { return h.Installation }
func (h *LabelHook) GetInstallationRef() *InstallationRef              { return h.Installation }
func (h *StatusHook) GetInstallationRef() *InstallationRef             { return h.Installation }
func (h *CheckRunHook) GetInstallationRef() *InstallationRef           { return h.Installation }
func (h *CheckSuiteHook) GetInstallationRef() *InstallationRef         { return h.Installation }
func (h *DeploymentStatusHook) GetInstallationRef() *InstallationRef   { return h.Installation }
func (h *ReleaseHook) GetInstallationRef() *InstallationRef            { return h.Installation }
func (h *RepositoryHook) GetInstallationRef() *InstallationRef         { return h.Installation }
func (h *ForkHook) GetInstallationRef() *InstallationRef               { return h.Installation }

func (h *InstallationHook) GetInstallationRef() *InstallationRef {
	if h.Installation == nil {
		return nil
	}
	return &InstallationRef{
		ID: h.Installation.ID,
	}
}

func (h *InstallationRepositoryHook) GetInstallationRef() *InstallationRef {
	if h.Installation == nil {
		return nil
	}
	return &InstallationRef{
		ID: h.Installation.ID,
	}
}
