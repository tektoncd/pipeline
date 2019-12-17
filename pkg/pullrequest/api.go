/*
Copyright 2019 The Tekton Authors

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

package pullrequest

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/jenkins-x/go-scm/scm"
	"go.uber.org/zap"
)

// Handler handles interactions with the GitHub API.
type Handler struct {
	client *scm.Client

	repo  string
	prNum int

	logger *zap.SugaredLogger
}

// NewHandler initializes a new handler for interacting with SCM
// resources.
func NewHandler(logger *zap.SugaredLogger, client *scm.Client, repo string, pr int) *Handler {
	return &Handler{
		logger: logger,
		client: client,
		repo:   repo,
		prNum:  pr,
	}
}

// Download fetches and stores the desired pull request.
func (h *Handler) Download(ctx context.Context) (*Resource, error) {
	// Pull Request
	h.logger.Info("finding pr")
	pr, _, err := h.client.PullRequests.Find(ctx, h.repo, h.prNum)
	if err != nil {
		return nil, fmt.Errorf("finding pr %d: %w", h.prNum, err)
	}

	// Statuses
	h.logger.Info("finding combined status")
	status, out, err := h.client.Repositories.ListStatus(ctx, h.repo, pr.Sha, scm.ListOptions{})
	if err != nil {
		body, _ := ioutil.ReadAll(out.Body)
		defer out.Body.Close()
		h.logger.Warnf("%v: %s", err, string(body))
		return nil, fmt.Errorf("finding combined status for pr %d: %w", h.prNum, err)
	}

	// Comments
	// TODO: Pagination.
	h.logger.Info("finding comments: %v", h)
	comments, _, err := h.client.PullRequests.ListComments(ctx, h.repo, h.prNum, scm.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("finding comments for pr %d: %w", h.prNum, err)
	}
	h.logger.Info("found comments: %v", comments)

	labels, _, err := h.client.PullRequests.ListLabels(ctx, h.repo, h.prNum, scm.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("finding labels for pr %d: %w", h.prNum, err)
	}
	pr.Labels = labels

	r := &Resource{
		PR:       pr,
		Statuses: status,
		Comments: comments,
	}
	populateManifest(r)
	return r, nil
}

func populateManifest(r *Resource) {
	labels := make(Manifest)
	for _, l := range r.PR.Labels {
		labels[l.Name] = true
	}

	comments := make(Manifest)
	for _, c := range r.Comments {
		comments[strconv.Itoa(c.ID)] = true
	}

	r.Manifests = map[string]Manifest{
		"labels":   labels,
		"comments": comments,
	}
}

// Upload takes files stored on the filesystem and uploads new changes to
// GitHub.
func (h *Handler) Upload(ctx context.Context, r *Resource) error {
	h.logger.Infof("Syncing resource: %+v", r)

	var merr error

	if err := h.uploadLabels(ctx, r.Manifests["labels"], r.PR.Labels); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := h.uploadStatuses(ctx, r.Statuses, r.PR.Sha); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := h.uploadComments(ctx, r.Manifests["comments"], r.Comments); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

func (h *Handler) uploadLabels(ctx context.Context, manifest Manifest, raw []*scm.Label) error {
	// Convert requested labels to a map. This ensures that there are no
	// duplicates and makes it easier to query which labels are being requested.
	labels := make(map[string]bool)
	for _, l := range raw {
		labels[l.Name] = true
	}

	// Fetch current labels associated to the PR. We'll need to keep track of
	// which labels are new and should not be modified.
	currentLabels, _, err := h.client.PullRequests.ListLabels(ctx, h.repo, h.prNum, scm.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing labels for pr %d: %w", h.prNum, err)
	}
	current := make(map[string]bool)
	for _, l := range currentLabels {
		current[l.Name] = true
	}
	h.logger.Debugf("Current labels: %v", current)

	var merr error

	// Create new labels that are missing from the PR.
	create := []string{}
	for l := range labels {
		if !current[l] {
			create = append(create, l)
		}
	}
	h.logger.Debugf("Creating labels %v for PR %d", create, h.prNum)
	for _, l := range create {
		if _, err := h.client.PullRequests.AddLabel(ctx, h.repo, h.prNum, l); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("adding label %s: %w", l, err))
		}
	}

	// Remove labels that no longer exist in the workspace and were present in
	// the manifest.
	for l := range current {
		if !labels[l] && manifest[l] {
			h.logger.Debugf("Removing label %s for PR %d", l, h.prNum)
			if _, err := h.client.PullRequests.DeleteLabel(ctx, h.repo, h.prNum, l); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("finding pr %d: %w", h.prNum, err))
			}
		}
	}

	return merr
}

func (h *Handler) uploadComments(ctx context.Context, manifest Manifest, comments []*scm.Comment) error {
	h.logger.Infof("Setting comments for PR %d to: %v", h.prNum, comments)

	// Sort comments into whether they are new or existing comments (based on
	// whether there is an ID defined).
	existingComments := map[int]*scm.Comment{}
	newComments := []*scm.Comment{}
	for _, c := range comments {
		if c.ID != 0 {
			existingComments[c.ID] = c
		} else {
			newComments = append(newComments, c)
		}
	}

	var merr error
	if err := h.maybeDeleteComments(ctx, manifest, existingComments); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("deleting comments %v: %w", existingComments, err))
	}

	if err := h.createNewComments(ctx, newComments); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("creating comments %v: %s", newComments, err))
	}

	return merr
}

// maybeDeleteComments deletes a comment iif it no longer exists in the remote
// SCM and exists in the manifest (therefore was present during resource
// initialization).
func (h *Handler) maybeDeleteComments(ctx context.Context, manifest Manifest, comments map[int]*scm.Comment) error {
	currentComments, _, err := h.client.PullRequests.ListComments(ctx, h.repo, h.prNum, scm.ListOptions{})
	if err != nil {
		return err
	}

	var merr error
	for _, ec := range currentComments {
		if _, ok := comments[ec.ID]; ok {
			// Comment already exists. Carry on
			continue
		}

		// Current comment does not exist in the current resource.

		// Check if we were aware of the comment when the resource was
		// initialized.
		if _, ok := manifest[strconv.Itoa(ec.ID)]; !ok {
			// Comment did not exist when resource created, so this was created
			// recently. To not modify this comment.
			h.logger.Debugf("Not tracking comment %d. Skipping.", ec.ID)
			continue
		}

		// Comment existed beforehand, user intentionally deleted. Remove from
		// upstream source.
		h.logger.Infof("Deleting comment %d for PR %d", ec.ID, h.prNum)
		if _, err := h.client.PullRequests.DeleteComment(ctx, h.repo, h.prNum, ec.ID); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("deleting comment %d: %w", ec.ID, err))
			continue
		}
	}
	return merr
}

func (h *Handler) createNewComments(ctx context.Context, comments []*scm.Comment) error {
	var merr error
	for _, dc := range comments {
		c := &scm.CommentInput{
			Body: dc.Body,
		}
		h.logger.Infof("Creating comment %s for PR %d", c.Body, h.prNum)
		if _, _, err := h.client.PullRequests.CreateComment(ctx, h.repo, h.prNum, c); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("creating comment %v: %w", c, err))
		}
	}
	return merr
}

func (h *Handler) uploadStatuses(ctx context.Context, statuses []*scm.Status, sha string) error {
	if statuses == nil {
		h.logger.Info("Skipping statuses, nothing to set.")
		return nil
	}

	h.logger.Infof("Looking for existing status on %s", sha)
	cs, _, err := h.client.Repositories.ListStatus(ctx, h.repo, sha, scm.ListOptions{})
	if err != nil {
		return err
	}

	// Index the statuses so we can avoid sending them if they already exist.
	csMap := map[string]scm.State{}
	for _, s := range cs {
		csMap[s.Label] = s.State
	}

	var merr error

	for _, s := range statuses {
		if csMap[s.Label] == s.State {
			h.logger.Infof("Skipping setting %s because it already matches", s.Label)
			continue
		}

		si := &scm.StatusInput{
			Label:  s.Label,
			State:  s.State,
			Desc:   s.Desc,
			Target: s.Target,
		}
		h.logger.Infof("Creating status %s on %s", si.Label, sha)
		if _, _, err := h.client.Repositories.CreateStatus(ctx, h.repo, sha, si); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("creating status %q: %w", si.Label, err))
			continue
		}
	}

	return merr
}
