// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jenkins-x/go-scm/pkg/hmac"
	"github.com/jenkins-x/go-scm/scm"
)

// TODO(bradrydzewski) push hook does not include commit message
// TODO(bradrydzewski) push hook does not include commit link
// TODO(bradrydzewski) push hook does not include repository git+http link
// TODO(bradrydzewski) push hook does not include repository git+ssh link
// TODO(bradrydzewski) push hook does not include repository html link
// TODO(bradrydzewski) missing pull request synchrnoized webhook. See https://jira.atlassian.com/browse/BSERV-10279
// TODO(bradrydzewski) pr hook does not include repository git+http link
// TODO(bradrydzewski) pr hook does not include repository git+ssh link
// TODO(bradrydzewski) pr hook does not include repository html link

type webhookService struct {
	client *wrapper
}

// Parse for the bitbucket server webhook payloads see: https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html
func (s *webhookService) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {
	data, err := io.ReadAll(
		io.LimitReader(req.Body, 10000000),
	)
	if err != nil {
		return nil, err
	}

	guid := req.Header.Get("X-Request-Id")

	var hook scm.Webhook
	event := req.Header.Get("X-Event-Key")
	switch event {
	case "repo:refs_changed":
		hook, err = s.parsePushHook(data, guid)
	case "pr:opened", "pr:declined", "pr:merged", "pr:from_ref_updated", "pr:modified":
		hook, err = s.parsePullRequest(data)
	case "pr:comment:added", "pr:comment:edited":
		hook, err = s.parsePullRequestComment(data, guid)
	case "pr:reviewer:approved", "pr:reviewer:unapproved", "pr:reviewer:needs_work":
		hook, err = s.parsePullRequestApproval(data)
	default:
		return nil, scm.UnknownWebhook{Event: event}
	}
	if err != nil {
		return nil, err
	}
	if hook == nil {
		return nil, nil
	}

	// get the gogs signature key to verify the payload
	// signature. If no key is provided, no validation
	// is performed.
	key, err := fn(hook)
	if err != nil {
		return hook, err
	} else if key == "" {
		return hook, nil
	}

	sig := req.Header.Get("X-Hub-Signature")
	if !hmac.ValidatePrefix(data, []byte(key), sig) {
		return hook, scm.ErrSignatureInvalid
	}

	return hook, nil
}

func (s *webhookService) parsePushHook(data []byte, guid string) (scm.Webhook, error) {
	dst := new(pushHook)
	err := json.Unmarshal(data, dst)
	if err != nil {
		return nil, err
	}
	if len(dst.Changes) == 0 {
		return nil, errors.New("push hook has empty changeset")
	}
	change := dst.Changes[0]
	switch {
	case change.Ref.Type == "BRANCH" && change.Type != "UPDATE":
		return convertBranchHook(dst), nil
	case change.Ref.Type == "TAG":
		return convertTagHook(dst), nil
	default:
		hook := convertPushHook(dst)
		hook.GUID = guid
		return hook, err
	}
}

func (s *webhookService) parsePullRequest(data []byte) (scm.Webhook, error) {
	src := new(pullRequestHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestHook(src)
	switch src.EventKey {
	case "pr:opened":
		dst.Action = scm.ActionOpen
	case "pr:declined":
		dst.Action = scm.ActionClose
	case "pr:merged":
		dst.Action = scm.ActionMerge
	case "pr:from_ref_updated":
		dst.Action = scm.ActionSync
	case "pr:modified":
		dst.Action = scm.ActionUpdate
	default:
		return nil, nil
	}
	return dst, nil
}

func (s *webhookService) parsePullRequestComment(data []byte, guid string) (scm.Webhook, error) {
	src := new(pullRequestCommentHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestCommentHook(src)
	dst.GUID = guid
	return dst, nil
}

func (s *webhookService) parsePullRequestApproval(data []byte) (scm.Webhook, error) {
	src := new(pullRequestApprovalHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestApprovalHook(src)
	switch src.EventKey {
	case "pr:reviewer:approved":
		dst.Action = scm.ActionSubmitted
	case "pr:reviewer:unapproved":
		dst.Action = scm.ActionDismissed
	case "pr:reviewer:needs_work":
		dst.Action = scm.ActionEdited
	default:
		return nil, nil
	}

	return dst, nil
}

//
// native data structures
//

type pushHook struct {
	EventKey   string      `json:"eventKey"`
	Date       string      `json:"date"`
	Actor      *user       `json:"actor"`
	Repository *repository `json:"repository"`
	Changes    []*change   `json:"changes"`
}

type pullRequestHook struct {
	EventKey            string       `json:"eventKey"`
	Date                string       `json:"date"`
	Actor               *user        `json:"actor"`
	PullRequest         *pullRequest `json:"pullRequest"`
	PreviousFromHash    string       `json:"previousFromHash"`
	PreviousTitle       string       `json:"previousTitle"`
	PreviousDescription string       `json:"previousDescription"`
	PreviousTarget      interface{}  `json:"previousTarget"`
}

type pullRequestCommentHook struct {
	EventKey    string       `json:"eventKey"`
	Date        string       `json:"date"`
	Author      *user        `json:"author"`
	PullRequest *pullRequest `json:"pullRequest"`
	Comment     *prComment   `json:"comment"`
}

type prComment struct {
	ID        int    `json:"id"`
	Version   int    `json:"version"`
	Text      string `json:"text"`
	Author    *user  `json:"author"`
	CreatedAt int64  `json:"createdDate"`
	UpdatedAt int64  `json:"updatedDate"`
}

type change struct {
	Ref struct {
		ID        string `json:"id"`
		DisplayID string `json:"displayId"`
		Type      string `json:"type"`
	} `json:"ref"`
	RefID    string `json:"refId"`
	FromHash string `json:"fromHash"`
	ToHash   string `json:"toHash"`
	Type     string `json:"type"`
}

type pullRequestApprovalHook struct {
	EventKey       string       `json:"eventKey"`
	Date           string       `json:"date"`
	Actor          *user        `json:"actor"`
	PullRequest    *pullRequest `json:"pullRequest"`
	Participant    *prUser      `json:"participant"`
	PreviousStatus string       `json:"previousParticipant"`
}

//
// push hooks
//

func convertPushHook(src *pushHook) *scm.PushHook {
	change := src.Changes[0]
	repo := convertRepository(src.Repository)
	sender := convertUser(src.Actor)
	signer := convertSignature(src.Actor)
	signer.Date, _ = time.Parse("2006-01-02T15:04:05+0000", src.Date)
	sha := change.ToHash
	return &scm.PushHook{
		Ref: change.RefID,
		Commit: scm.Commit{
			Sha:       sha,
			Message:   "",
			Link:      "",
			Author:    signer,
			Committer: signer,
		},
		After:  sha,
		Repo:   *repo,
		Sender: *sender,
	}
}

func convertTagHook(src *pushHook) *scm.TagHook {
	change := src.Changes[0]
	sender := convertUser(src.Actor)
	repo := convertRepository(src.Repository)

	dst := &scm.TagHook{
		Ref: scm.Reference{
			Name: change.Ref.DisplayID,
			Sha:  change.ToHash,
		},
		Action: scm.ActionCreate,
		Repo:   *repo,
		Sender: *sender,
	}
	if change.Type == "DELETE" {
		dst.Action = scm.ActionDelete
		dst.Ref.Sha = change.FromHash
	}
	return dst
}

func convertBranchHook(src *pushHook) *scm.BranchHook {
	change := src.Changes[0]
	sender := convertUser(src.Actor)
	repo := convertRepository(src.Repository)

	dst := &scm.BranchHook{
		Ref: scm.Reference{
			Name: change.Ref.DisplayID,
			Sha:  change.ToHash,
		},
		Action: scm.ActionCreate,
		Repo:   *repo,
		Sender: *sender,
	}
	if change.Type == "DELETE" {
		dst.Action = scm.ActionDelete
		dst.Ref.Sha = change.FromHash
	}
	return dst
}

func convertSignature(actor *user) scm.Signature {
	return scm.Signature{
		Name:   actor.DisplayName,
		Email:  actor.EmailAddress,
		Login:  actor.Slug,
		Avatar: avatarLink(actor.EmailAddress),
	}
}

func convertPullRequestHook(src *pullRequestHook) *scm.PullRequestHook {
	toRepo := convertRepository(&src.PullRequest.ToRef.Repository)
	fromRepo := convertRepository(&src.PullRequest.FromRef.Repository)
	pr := convertPullRequest(src.PullRequest)
	sender := convertUser(src.Actor)
	pr.Base.Repo = *toRepo
	pr.Head.Repo = *fromRepo
	if pr.Base.Ref == "" {
		pr.Base.Ref = toRepo.Branch
	}
	if pr.Head.Ref == "" {
		pr.Head.Ref = fromRepo.Branch
	}
	return &scm.PullRequestHook{
		Action:      scm.ActionOpen,
		Repo:        *toRepo,
		PullRequest: *pr,
		Sender:      *sender,
	}
}

func convertPullRequestCommentHook(src *pullRequestCommentHook) *scm.PullRequestCommentHook {
	toRepo := convertRepository(&src.PullRequest.ToRef.Repository)
	fromRepo := convertRepository(&src.PullRequest.FromRef.Repository)
	pr := convertPullRequest(src.PullRequest)
	author := src.Comment.Author
	if author == nil {
		author = src.Author
	}
	sender := convertUser(author)
	pr.Base.Repo = *toRepo
	pr.Head.Repo = *fromRepo
	return &scm.PullRequestCommentHook{
		Action:      scm.ActionCreate,
		Repo:        *toRepo,
		PullRequest: *pr,
		Sender:      *sender,
		Comment:     convertComment(src.Comment),
	}
}

func convertComment(src *prComment) scm.Comment {
	dst := scm.Comment{}
	if src != nil {
		dst.ID = src.ID
		dst.Body = src.Text
		author := convertUser(src.Author)
		if author != nil {
			dst.Author = *author
		}
		dst.Created = time.Unix(src.CreatedAt/1000, 0)
		dst.Updated = time.Unix(src.UpdatedAt/1000, 0)
	}
	return dst
}

func convertPullRequestApprovalHook(src *pullRequestApprovalHook) *scm.ReviewHook {
	toRepo := convertRepository(&src.PullRequest.ToRef.Repository)
	fromRepo := convertRepository(&src.PullRequest.FromRef.Repository)
	pr := convertPullRequest(src.PullRequest)
	pr.Base.Repo = *toRepo
	pr.Head.Repo = *fromRepo
	if pr.Base.Ref == "" {
		pr.Base.Ref = toRepo.Branch
	}
	if pr.Head.Ref == "" {
		pr.Head.Ref = fromRepo.Branch
	}
	review := scm.Review{
		State:  convertReviewStateFromEvent(src.EventKey),
		Author: *convertUser(&src.Participant.User),
	}

	return &scm.ReviewHook{
		PullRequest: *pr,
		Repo:        *toRepo,
		Review:      review,
	}
}

func convertReviewStateFromEvent(src string) string {
	switch src {
	case "pr:reviewer:approved":
		return scm.ReviewStateApproved
	case "pr:reviewer:unapproved":
		return scm.ReviewStateDismissed
	case "pr:reviewer:needs_work":
		return scm.ReviewStateChangesRequested
	default:
		return scm.ReviewStatePending
	}
}
