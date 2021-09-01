// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/jenkins-x/go-scm/pkg/hmac"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/sirupsen/logrus"
)

var logWebHooks = os.Getenv("GO_SCM_LOG_WEBHOOKS") == "true"

type webhookService struct {
	client *wrapper
}

func (s *webhookService) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {
	data, err := ioutil.ReadAll(
		io.LimitReader(req.Body, 10000000),
	)
	if err != nil {
		return nil, err
	}

	log := logrus.WithFields(map[string]interface{}{
		"URL":     req.URL,
		"Headers": req.Header,
		"Body":    string(data),
	})
	if logWebHooks {
		log.Infof("received webhook")
	}

	guid := req.Header.Get("X-GitHub-Delivery")
	if guid == "" {
		return nil, scm.MissingHeader{Header: "X-GitHub-Delivery"}
	}

	var hook scm.Webhook
	event := req.Header.Get("X-GitHub-Event")
	switch event {
	case "check_run":
		hook, err = s.parseCheckRunHook(data)
	case "check_suite":
		hook, err = s.parseCheckSuiteHook(data)
	case "create":
		hook, err = s.parseCreateHook(data)
	case "delete":
		hook, err = s.parseDeleteHook(data)
	case "deployment":
		hook, err = s.parseDeploymentHook(data)
	case "deployment_status":
		hook, err = s.parseDeploymentStatusHook(data)
	case "fork":
		hook, err = s.parseForkHook(data)
	case "issues":
		hook, err = s.parseIssueHook(data)
	case "issue_comment":
		hook, err = s.parseIssueCommentHook(data, guid)
	case "installation", "integration_installation":
		hook, err = s.parseInstallationHook(data)
	case "installation_repositories", "integration_installation_repositories":
		hook, err = s.parseInstallationRepositoryHook(data)
	case "label":
		hook, err = s.parseLabelHook(data)
	case "ping":
		hook, err = s.parsePingHook(data, guid)
	case "push":
		hook, err = s.parsePushHook(data, guid)
	case "pull_request":
		hook, err = s.parsePullRequestHook(data, guid)
	case "pull_request_review":
		hook, err = s.parsePullRequestReviewHook(data, guid)
	case "pull_request_review_comment":
		hook, err = s.parsePullRequestReviewCommentHook(data, guid)
	case "release":
		hook, err = s.parseReleaseHook(data)
	case "repository":
		hook, err = s.parseRepositoryHook(data)
	case "status":
		hook, err = s.parseStatusHook(data)
	case "watch":
		hook, err = s.parseWatchHook(data)
	default:
		log.WithField("Event", event).Warnf("unknown webhook")
		return nil, scm.UnknownWebhook{Event: event}
	}
	if err != nil {
		return nil, err
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

	if logWebHooks {
		log.Infof("Webhook HMAC token: %s", key)
	}

	sig := req.Header.Get("X-Hub-Signature")
	if !hmac.ValidatePrefix(data, []byte(key), sig) {
		return hook, scm.ErrSignatureInvalid
	}

	return hook, nil
}

func (s *webhookService) parsePingHook(data []byte, guid string) (*scm.PingHook, error) {
	dst := new(pingHook)
	err := json.Unmarshal(data, dst)
	to := convertPingHook(dst)
	if to != nil {
		to.GUID = guid
	}
	return to, err
}

func (s *webhookService) parsePullRequestReviewHook(data []byte, guid string) (*scm.ReviewHook, error) {
	dst := new(pullRequestReviewHook)
	err := json.Unmarshal(data, dst)
	if err != nil {
		return nil, err
	}
	to := convertPullRequestReviewHook(dst)
	if to != nil {
		to.GUID = guid
	}
	return to, err
}

func (s *webhookService) parseWatchHook(data []byte) (*scm.WatchHook, error) {
	dst := new(watchHook)
	err := json.Unmarshal(data, dst)
	to := convertWatchHook(dst)
	return to, err
}

func (s *webhookService) parsePushHook(data []byte, guid string) (*scm.PushHook, error) {
	dst := new(pushHook)
	err := json.Unmarshal(data, dst)
	to := convertPushHook(dst)
	if to != nil {
		to.GUID = guid
	}
	return to, err
}

func (s *webhookService) parseCreateHook(data []byte) (scm.Webhook, error) {
	src := new(createDeleteHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	if src.RefType == "branch" {
		dst := convertBranchHook(src)
		dst.Action = scm.ActionCreate
		return dst, nil
	}
	dst := convertTagHook(src)
	dst.Action = scm.ActionCreate
	return dst, nil
}

func (s *webhookService) parseDeleteHook(data []byte) (scm.Webhook, error) {
	src := new(createDeleteHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	if src.RefType == "branch" {
		dst := convertBranchHook(src)
		dst.Action = scm.ActionDelete
		return dst, nil
	}
	dst := convertTagHook(src)
	dst.Action = scm.ActionDelete
	return dst, nil
}

func (s *webhookService) parseCheckRunHook(data []byte) (scm.Webhook, error) {
	src := new(checkRunHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertCheckRunHook(src)
	return to, err
}

func (s *webhookService) parseStarHook(data []byte) (scm.Webhook, error) {
	src := new(starHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertStarHook(src)
	return to, err
}

func (s *webhookService) parseCheckSuiteHook(data []byte) (scm.Webhook, error) {
	src := new(checkSuiteHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertCheckSuiteHook(src)
	return to, err
}

func (s *webhookService) parseDeploymentStatusHook(data []byte) (scm.Webhook, error) {
	src := new(deploymentStatusHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertDeploymentStatusHook(src)
	return to, err
}

func (s *webhookService) parseForkHook(data []byte) (scm.Webhook, error) {
	src := new(forkHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertForkHook(src)
	return to, err
}

func (s *webhookService) parseLabelHook(data []byte) (scm.Webhook, error) {
	src := new(labelHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertLabelHook(src)
	return to, err
}

func (s *webhookService) parseReleaseHook(data []byte) (scm.Webhook, error) {
	src := new(releaseHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertReleaseHook(src)
	return to, err
}

func (s *webhookService) parseRepositoryHook(data []byte) (scm.Webhook, error) {
	src := new(repositoryHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertRepositoryHook(src)
	return to, err
}

func (s *webhookService) parseStatusHook(data []byte) (scm.Webhook, error) {
	src := new(statusHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	to := convertStatusHook(src)
	return to, err
}

func (s *webhookService) parseDeploymentHook(data []byte) (scm.Webhook, error) {
	src := new(deploymentHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertDeploymentHook(src)
	return dst, nil
}

func (s *webhookService) parsePullRequestHook(data []byte, guid string) (scm.Webhook, error) {
	src := new(pullRequestHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestHook(src)
	dst.GUID = guid
	switch src.Action {
	case "assigned":
		dst.Action = scm.ActionAssigned
	case "unassigned":
		dst.Action = scm.ActionUnassigned
	case "review_requested":
		dst.Action = scm.ActionReviewRequested
	case "review_request_removed":
		dst.Action = scm.ActionReviewRequestRemoved
	case "labeled":
		dst.Action = scm.ActionLabel
	case "unlabeled":
		dst.Action = scm.ActionUnlabel
	case "opened":
		dst.Action = scm.ActionOpen
	case "edited":
		dst.Action = scm.ActionUpdate
	case "closed":
		// if merged == true
		//    dst.Action = scm.ActionMerge
		dst.Action = scm.ActionClose
	case "reopened":
		dst.Action = scm.ActionReopen
	case "synchronize":
		dst.Action = scm.ActionSync
	case "ready_for_review":
		dst.Action = scm.ActionReadyForReview
	case "converted_to_draft":
		dst.Action = scm.ActionConvertedToDraft
	}
	return dst, nil
}

func (s *webhookService) parsePullRequestReviewCommentHook(data []byte, guid string) (scm.Webhook, error) {
	src := new(pullRequestReviewCommentHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestReviewCommentHook(src)
	if dst != nil {
		dst.GUID = guid
	}
	return dst, nil
}

func (s *webhookService) parseIssueHook(data []byte) (*scm.IssueHook, error) {
	src := new(issueHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertIssueHook(src)
	return dst, nil
}

func (s *webhookService) parseIssueCommentHook(data []byte, guid string) (*scm.IssueCommentHook, error) {
	src := new(issueCommentHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertIssueCommentHook(src)
	if dst != nil {
		dst.GUID = guid
	}
	return dst, nil
}

func (s *webhookService) parseInstallationHook(data []byte) (*scm.InstallationHook, error) {
	src := new(installationHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertInstallationHook(src)
	return dst, nil
}

func (s *webhookService) parseInstallationRepositoryHook(data []byte) (*scm.InstallationRepositoryHook, error) {
	src := new(installationRepositoryHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertInstallationRepositoryHook(src)
	return dst, nil
}

//
// native data structures
//

type (
	// github create webhook payload
	createDeleteHook struct {
		Ref          string           `json:"ref"`
		RefType      string           `json:"ref_type"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	// github ping payload
	pingHook struct {
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	// github watch payload
	watchHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	// github check_run payload
	checkRunHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	// github star repo payload
	starHook struct {
		Action     string     `json:"action"`
		Repository repository `json:"repository"`
		Sender     user       `json:"sender"`
		StarredAt  time.Time  `json:"starred_at"`
	}

	// github check_suite payload
	checkSuiteHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	// github deployment webhook payload
	deploymentHook struct {
		Deployment   deployment       `json:"deployment"`
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	// github deployment_status payload
	deploymentStatusHook struct {
		DeploymentStatus deploymentStatus `json:"deployment_status"`
		Deployment       deployment       `json:"deployment"`
		Action           string           `json:"action"`
		Repository       repository       `json:"repository"`
		Sender           user             `json:"sender"`
		Label            label            `json:"label"`
		Installation     *installationRef `json:"installation"`
	}

	// github deployment_status payload
	forkHook struct {
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	// github label payload
	labelHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	// github release payload
	releaseHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Release      release          `json:"release"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	// github repository payload
	repositoryHook struct {
		Action       string           `json:"action"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	// github status payload
	statusHook struct {
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Label        label            `json:"label"`
		Installation *installationRef `json:"installation"`
	}

	pushCommit struct {
		ID        string `json:"id"`
		TreeID    string `json:"tree_id"`
		Distinct  bool   `json:"distinct"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"author"`
		Committer struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"committer"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}

	// github push webhook payload
	pushHook struct {
		Ref     string `json:"ref"`
		BaseRef string `json:"base_ref"`
		Before  string `json:"before"`
		After   string `json:"after"`
		Compare string `json:"compare"`
		Created bool   `json:"created"`
		Deleted bool   `json:"deleted"`
		Forced  bool   `json:"forced"`
		Head    struct {
			ID        string `json:"id"`
			TreeID    string `json:"tree_id"`
			Distinct  bool   `json:"distinct"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
			URL       string `json:"url"`
			Author    struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"author"`
			Committer struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"committer"`
			Added    []interface{} `json:"added"`
			Removed  []interface{} `json:"removed"`
			Modified []string      `json:"modified"`
		} `json:"head_commit"`
		Commits    []pushCommit `json:"commits"`
		Repository struct {
			ID    int64 `json:"id"`
			Owner struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			Private       bool   `json:"private"`
			Fork          bool   `json:"fork"`
			HTMLURL       string `json:"html_url"`
			SSHURL        string `json:"ssh_url"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
		} `json:"repository"`
		Pusher       user             `json:"pusher"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	pullRequestHookChanges struct {
		Base struct {
			Ref struct {
				From string `json:"from"`
			} `json:"ref"`
			Sha struct {
				From string `json:"from"`
			} `json:"sha"`
		} `json:"base"`
	}

	pullRequestHook struct {
		Action       string                 `json:"action"`
		Number       int                    `json:"number"`
		PullRequest  pr                     `json:"pull_request"`
		Repository   repository             `json:"repository"`
		Label        label                  `json:"label"`
		Sender       user                   `json:"sender"`
		Changes      pullRequestHookChanges `json:"changes"`
		Installation *installationRef       `json:"installation"`
	}

	label struct {
		URL         string `json:"url"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	pullRequestReviewHook struct {
		Action       string           `json:"action"`
		Review       review           `json:"review"`
		PullRequest  pr               `json:"pull_request"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	pullRequestReviewCommentHook struct {
		// Action see https://developer.github.com/v3/activity/events/types/#pullrequestreviewcommentevent
		Action       string                `json:"action"`
		PullRequest  pr                    `json:"pull_request"`
		Repository   repository            `json:"repository"`
		Comment      reviewCommentFromHook `json:"comment"`
		Installation *installationRef      `json:"installation"`
	}

	issueHook struct {
		Action       string           `json:"action"`
		Issue        issue            `json:"issue"`
		Changes      *editChange      `json:"changes"`
		Repository   repository       `json:"repository"`
		Sender       user             `json:"sender"`
		Installation *installationRef `json:"installation"`
	}

	editChange struct {
		Title *struct {
			From *string `json:"from,omitempty"`
		} `json:"title,omitempty"`
		Body *struct {
			From *string `json:"from,omitempty"`
		} `json:"body,omitempty"`
	}

	issueCommentHook struct {
		Action     string       `json:"action"`
		Issue      issue        `json:"issue"`
		Repository repository   `json:"repository"`
		Comment    issueComment `json:"comment"`
		Sender     user         `json:"sender"`

		Installation *installationRef `json:"installation"`
	}

	// reviewCommentFromHook describes a Pull Request d comment
	reviewCommentFromHook struct {
		ID        int       `json:"id"`
		ReviewID  int       `json:"pull_request_review_id"`
		User      user      `json:"user"`
		Body      string    `json:"body"`
		Path      string    `json:"path"`
		HTMLURL   string    `json:"html_url"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		// Position will be nil if the code has changed such that the comment is no
		// longer relevant.
		Position *int `json:"position"`
	}

	// installationHook a webhook invoked when the GitHub App is installed
	installationHook struct {
		Action       string        `json:"action"`
		Repositories []*repository `json:"repositories"`
		Installation *installation `json:"installation"`
		Sender       *user         `json:"sender"`
	}

	// installationRepositoryHook a webhook invoked when the GitHub App is installed
	installationRepositoryHook struct {
		Action              string        `json:"action"`
		RepositoriesAdded   []*repository `json:"repositories_added"`
		RepositoriesRemoved []*repository `json:"repositories_removed"`
		Installation        *installation `json:"installation"`
		Sender              *user         `json:"sender"`
	}

	// github app installation
	installation struct {
		ID                  int64  `json:"id"`
		AppID               int64  `json:"app_id"`
		TargetID            int64  `json:"target_id"`
		TargetType          string `json:"target_type"`
		RepositorySelection string `json:"repository_selection"`
		Account             struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
			Link  string `json:"html_url"`
		} `json:"account"`
		Events          []string `json:"events"`
		AccessTokensURL string   `json:"access_tokens_url"`
		RepositoriesURL string   `json:"repositories_url"`
		Link            string   `json:"html_url"`

		// TODO these are numbers or strings depending on if a webhook or regular query
		CreatedAt interface{} `json:"created_at"`
		UpdatedAt interface{} `json:"updated_at"`
	}

	// github app installation reference
	installationRef struct {
		ID     int64  `json:"id"`
		NodeID string `json:"node_id"`
	}
)

//
// native data structure conversion
//

func convertInstallationHook(dst *installationHook) *scm.InstallationHook {
	if dst == nil {
		return nil
	}
	return &scm.InstallationHook{
		Action:       convertAction(dst.Action),
		Repos:        convertRepositoryList(dst.Repositories),
		Sender:       *convertUser(dst.Sender),
		Installation: convertInstallation(dst.Installation),
	}
}

func convertInstallationRepositoryHook(dst *installationRepositoryHook) *scm.InstallationRepositoryHook {
	if dst == nil {
		return nil
	}
	return &scm.InstallationRepositoryHook{
		Action:       convertAction(dst.Action),
		ReposAdded:   convertRepositoryList(dst.RepositoriesAdded),
		ReposRemoved: convertRepositoryList(dst.RepositoriesRemoved),
		Sender:       *convertUser(dst.Sender),
		Installation: convertInstallation(dst.Installation),
	}
}

func convertInstallation(dst *installation) *scm.Installation {
	if dst == nil {
		return nil
	}
	acc := dst.Account
	return &scm.Installation{
		Account: scm.Account{
			ID:    acc.ID,
			Login: acc.Login,
			Link:  acc.Link,
		},
		ID:                  dst.ID,
		AppID:               dst.AppID,
		TargetID:            dst.TargetID,
		TargetType:          dst.TargetType,
		RepositorySelection: dst.RepositorySelection,
		AccessTokensLink:    dst.AccessTokensURL,
		RepositoriesURL:     dst.RepositoriesURL,
		Link:                dst.Link,
		Events:              dst.Events,
		CreatedAt:           asTimeStamp(dst.CreatedAt),
		UpdatedAt:           asTimeStamp(dst.UpdatedAt),
	}
}

func asTimeStamp(t interface{}) *time.Time {
	value, ok := t.(*time.Time)
	if ok {
		return value
	}
	text, ok := t.(string)
	if ok {
		answer, err := time.Parse("2006-01-02T15:04:05Z", text)
		if err == nil {
			return &answer
		}
	}
	f, ok := t.(float64)
	if ok {
		answer := time.Unix(int64(f), 0)
		return &answer
	}
	return nil
}

func convertInstallationRef(dst *installationRef) *scm.InstallationRef {
	if dst == nil {
		return nil
	}
	return &scm.InstallationRef{
		ID:     dst.ID,
		NodeID: dst.NodeID,
	}
}
func convertPingHook(dst *pingHook) *scm.PingHook {
	return &scm.PingHook{
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertWatchHook(dst *watchHook) *scm.WatchHook {
	return &scm.WatchHook{
		Action:       dst.Action,
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertCheckRunHook(dst *checkRunHook) *scm.CheckRunHook {
	return &scm.CheckRunHook{
		Action:       convertAction(dst.Action),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Label:        convertLabel(dst.Label),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertStarHook(dst *starHook) *scm.StarHook {
	return &scm.StarHook{
		Action:    convertAction(dst.Action),
		StarredAt: dst.StarredAt,
		Repo:      *convertRepository(&dst.Repository),
		Sender:    *convertUser(&dst.Sender),
	}
}

func convertCheckSuiteHook(dst *checkSuiteHook) *scm.CheckSuiteHook {
	return &scm.CheckSuiteHook{
		Action:       convertAction(dst.Action),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Label:        convertLabel(dst.Label),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertDeploymentHook(src *deploymentHook) *scm.DeployHook {
	dst := &scm.DeployHook{
		Deployment: *convertDeployment(&src.Deployment, src.Repository.FullName),
		Ref: scm.Reference{
			Name: src.Deployment.Ref,
			Path: src.Deployment.Ref,
			Sha:  src.Deployment.Sha,
		},
		Action:       convertAction(src.Action),
		Repo:         *convertRepository(&src.Repository),
		Sender:       *convertUser(&src.Sender),
		Label:        convertLabel(src.Label),
		Installation: convertInstallationRef(src.Installation),
	}
	if tagRE.MatchString(dst.Ref.Name) {
		dst.Ref.Path = scm.ExpandRef(dst.Ref.Path, "refs/tags/")
	} else {
		dst.Ref.Path = scm.ExpandRef(dst.Ref.Path, "refs/heads/")
	}
	return dst
}

func convertDeploymentStatusHook(dst *deploymentStatusHook) *scm.DeploymentStatusHook {
	return &scm.DeploymentStatusHook{
		Deployment:       *convertDeployment(&dst.Deployment, dst.Repository.FullName),
		DeploymentStatus: *convertDeploymentStatus(&dst.DeploymentStatus),
		Action:           convertAction(dst.Action),
		Repo:             *convertRepository(&dst.Repository),
		Sender:           *convertUser(&dst.Sender),
		Label:            convertLabel(dst.Label),
		Installation:     convertInstallationRef(dst.Installation),
	}
}

func convertForkHook(dst *forkHook) *scm.ForkHook {
	return &scm.ForkHook{
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertLabelHook(dst *labelHook) *scm.LabelHook {
	return &scm.LabelHook{
		Action:       convertAction(dst.Action),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Label:        convertLabel(dst.Label),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertReleaseHook(dst *releaseHook) *scm.ReleaseHook {
	return &scm.ReleaseHook{
		Action:       convertAction(dst.Action),
		Repo:         *convertRepository(&dst.Repository),
		Release:      *convertRelease(&dst.Release),
		Sender:       *convertUser(&dst.Sender),
		Label:        convertLabel(dst.Label),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertRepositoryHook(dst *repositoryHook) *scm.RepositoryHook {
	return &scm.RepositoryHook{
		Action:       convertAction(dst.Action),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertStatusHook(dst *statusHook) *scm.StatusHook {
	return &scm.StatusHook{
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Label:        convertLabel(dst.Label),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertPushHook(src *pushHook) *scm.PushHook {
	dst := &scm.PushHook{
		Ref:     src.Ref,
		BaseRef: src.BaseRef,
		Before:  src.Before,
		After:   src.After,
		Compare: src.Compare,
		Created: src.Created,
		Deleted: src.Deleted,
		Forced:  src.Forced,
		Commit: scm.Commit{
			Sha:     src.After,
			Message: src.Head.Message,
			Link:    src.Compare,
			Author: scm.Signature{
				Login: src.Head.Author.Username,
				Email: src.Head.Author.Email,
				Name:  src.Head.Author.Name,
				// TODO (bradrydzewski) set the timestamp
			},
			Committer: scm.Signature{
				Login: src.Head.Committer.Username,
				Email: src.Head.Committer.Email,
				Name:  src.Head.Committer.Name,
				// TODO (bradrydzewski) set the timestamp
			},
		},
		Commits: convertPushCommits(src.Commits),
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			FullName:  src.Repository.FullName,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender:       *convertUser(&src.Sender),
		Installation: convertInstallationRef(src.Installation),
	}
	// fix https://github.com/jenkins-x/go-scm/issues/8
	if scm.IsTag(dst.Ref) && src.Head.ID != "" {
		dst.Commit.Sha = src.Head.ID
		dst.After = src.Head.ID
	}
	return dst
}

func convertPushCommits(src []pushCommit) []scm.PushCommit {
	dst := []scm.PushCommit{}
	for _, s := range src {
		dst = append(dst, *convertPushCommit(&s))
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func convertPushCommit(src *pushCommit) *scm.PushCommit {
	return &scm.PushCommit{
		ID:       src.URL,
		Message:  src.Message,
		Added:    src.Added,
		Removed:  src.Removed,
		Modified: src.Modified,
	}
}

func convertBranchHook(src *createDeleteHook) *scm.BranchHook {
	return &scm.BranchHook{
		Ref: scm.Reference{
			Name: src.Ref,
		},
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			FullName:  src.Repository.FullName,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender:       *convertUser(&src.Sender),
		Installation: convertInstallationRef(src.Installation),
	}
}

func convertTagHook(src *createDeleteHook) *scm.TagHook {
	return &scm.TagHook{
		Ref: scm.Reference{
			Name: src.Ref,
		},
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			FullName:  src.Repository.FullName,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender:       *convertUser(&src.Sender),
		Installation: convertInstallationRef(src.Installation),
	}
}

func convertPullRequestHook(src *pullRequestHook) *scm.PullRequestHook {
	return &scm.PullRequestHook{
		// Action        Action
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			FullName:  src.Repository.FullName,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Label:        convertLabel(src.Label),
		PullRequest:  *convertPullRequest(&src.PullRequest),
		Sender:       *convertUser(&src.Sender),
		Changes:      *convertPullRequestChanges(&src.Changes),
		Installation: convertInstallationRef(src.Installation),
	}
}

func convertPullRequestChanges(src *pullRequestHookChanges) *scm.PullRequestHookChanges {
	to := &scm.PullRequestHookChanges{}
	to.Base.Sha.From = src.Base.Sha.From
	to.Base.Ref.From = src.Base.Ref.From
	return to
}

func convertLabel(src label) scm.Label {
	return scm.Label{
		Color:       src.Color,
		Description: src.Description,
		Name:        src.Name,
		URL:         src.URL,
	}
}

func convertPullRequestReviewHook(src *pullRequestReviewHook) *scm.ReviewHook {
	return &scm.ReviewHook{
		Action:       convertReviewAction(src.Action),
		PullRequest:  *convertPullRequest(&src.PullRequest),
		Repo:         *convertRepository(&src.Repository),
		Review:       *convertReview(&src.Review),
		Installation: convertInstallationRef(src.Installation),
	}
}

func convertPullRequestReviewCommentHook(src *pullRequestReviewCommentHook) *scm.PullRequestCommentHook {
	return &scm.PullRequestCommentHook{
		// Action        Action
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			FullName:  src.Repository.FullName,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		PullRequest:  *convertPullRequest(&src.PullRequest),
		Comment:      *convertPullRequestComment(&src.Comment),
		Sender:       *convertUser(&src.Comment.User),
		Installation: convertInstallationRef(src.Installation),
	}
}

func convertIssueHook(dst *issueHook) *scm.IssueHook {
	return &scm.IssueHook{
		Action:       convertAction(dst.Action),
		Issue:        *convertIssue(&dst.Issue),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertIssueCommentHook(dst *issueCommentHook) *scm.IssueCommentHook {
	return &scm.IssueCommentHook{
		Action:       convertAction(dst.Action),
		Issue:        *convertIssue(&dst.Issue),
		Comment:      *convertIssueComment(&dst.Comment),
		Repo:         *convertRepository(&dst.Repository),
		Sender:       *convertUser(&dst.Sender),
		Installation: convertInstallationRef(dst.Installation),
	}
}

func convertPullRequestComment(comment *reviewCommentFromHook) *scm.Comment {
	return &scm.Comment{
		ID:      comment.ID,
		Body:    comment.Body,
		Author:  *convertUser(&comment.User),
		Created: comment.CreatedAt,
		Updated: comment.UpdatedAt,
	}
}

// regexp help determine if the named git object is a tag.
// this is not meant to be 100% accurate.
var tagRE = regexp.MustCompile("^v?(\\d+).(.+)")

func convertReviewAction(src string) (action scm.Action) {
	switch src {
	case "submit", "submitted":
		return scm.ActionSubmitted
	case "edit", "edited":
		return scm.ActionEdited
	case "dismiss", "dismissed":
		return scm.ActionDismissed
	default:
		return
	}
}

func convertAction(src string) (action scm.Action) {
	switch src {
	case "create", "created":
		return scm.ActionCreate
	case "delete", "deleted":
		return scm.ActionDelete
	case "update", "updated", "edit", "edited":
		return scm.ActionUpdate
	case "open", "opened":
		return scm.ActionOpen
	case "reopen", "reopened":
		return scm.ActionReopen
	case "close", "closed":
		return scm.ActionClose
	case "label", "labeled":
		return scm.ActionLabel
	case "unlabel", "unlabeled":
		return scm.ActionUnlabel
	case "merge", "merged":
		return scm.ActionMerge
	case "synchronize", "synchronized":
		return scm.ActionSync
	case "complete", "completed":
		return scm.ActionCompleted
	default:
		return
	}
}
