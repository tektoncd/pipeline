package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

var (
	toGitHub = map[StatusCode]string{
		Unknown: "error",
		Success: "success",
		Failure: "failure",
		Error:   "error",
		// There's no analog for neutral in GitHub statuses, so default to success
		// to make this non-blocking.
		Neutral:        "success",
		Queued:         "pending",
		InProgress:     "pending",
		Timeout:        "error",
		Canceled:       "error",
		ActionRequired: "error",
	}
	toTekton = map[string]StatusCode{
		"success": Success,
		"failure": Failure,
		"error":   Error,
		"pending": Queued,
	}
)

// GitHubHandler handles interactions with the GitHub API.
type GitHubHandler struct {
	*github.Client

	owner, repo string
	prNum       int

	Logger *zap.SugaredLogger
}

// NewGitHubHandler initializes a new handler for interacting with GitHub
// resources.
func NewGitHubHandler(ctx context.Context, logger *zap.SugaredLogger, rawURL string) (*GitHubHandler, error) {
	token := strings.TrimSpace(os.Getenv("GITHUBTOKEN"))
	var hc *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		hc = oauth2.NewClient(ctx, ts)
	}

	owner, repo, host, prNumber, err := parseGitHubURL(rawURL)
	if err != nil {
		return nil, err
	}
	var client *github.Client
	if !strings.Contains(host, "github.com") {
		u := fmt.Sprintf("%s/api/v3/", host)
		client, err = github.NewEnterpriseClient(u, u, hc)
		if err != nil {
			return nil, err
		}
	} else {
		client = github.NewClient(hc)
	}
	return &GitHubHandler{
		Client: client,
		Logger: logger,
		owner:  owner,
		repo:   repo,
		prNum:  prNumber,
	}, nil
}

// parseURL takes in a raw GitHub URL
// (e.g. https://github.com/owner/repo/pull/1) and extracts the owner, repo, host,
// and pull request number.
func parseGitHubURL(raw string) (string, string, string, int, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", 0, err
	}
	split := strings.Split(u.Path, "/")
	if len(split) < 5 {
		return "", "", "", 0, fmt.Errorf("could not determine PR from URL: %v", raw)
	}
	owner, repo, pr := split[1], split[2], split[4]
	prNumber, err := strconv.Atoi(pr)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("error parsing PR number: %s", pr)
	}

	return owner, repo, u.Scheme + "://" + u.Host, prNumber, nil
}

// writeJSON writes an arbitrary interface to the given path.
func writeJSON(path string, i interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(i)
}

// Download fetches and stores the desired pull request.
func (h *GitHubHandler) Download(ctx context.Context, path string) error {
	rawPrefix := filepath.Join(path, "github")
	if err := os.MkdirAll(rawPrefix, 0755); err != nil {
		return err
	}

	gpr, _, err := h.PullRequests.Get(ctx, h.owner, h.repo, h.prNum)
	if err != nil {
		return err
	}
	pr := baseGitHubPullRequest(gpr)

	rawStatus := filepath.Join(rawPrefix, "status.json")
	statuses, err := h.getStatuses(ctx, pr.Head.SHA, rawStatus)
	if err != nil {
		return err
	}
	pr.RawStatus = rawStatus
	pr.Statuses = statuses

	rawPR := filepath.Join(rawPrefix, "pr.json")
	if err := writeJSON(rawPR, gpr); err != nil {
		return err
	}
	pr.Raw = rawPR

	// Comments
	pr.Comments, err = h.downloadComments(ctx, rawPrefix)
	if err != nil {
		return err
	}

	prPath := filepath.Join(path, prFile)
	h.Logger.Infof("Writing pull request to file: %s", prPath)
	return writeJSON(prPath, pr)
}

func baseGitHubPullRequest(pr *github.PullRequest) *PullRequest {
	return &PullRequest{
		Type: "github",
		ID:   pr.GetID(),
		Head: &GitReference{
			Repo:   pr.GetHead().GetRepo().GetCloneURL(),
			Branch: pr.GetHead().GetRef(),
			SHA:    pr.GetHead().GetSHA(),
		},
		Base: &GitReference{
			Repo:   pr.GetBase().GetRepo().GetCloneURL(),
			Branch: pr.GetBase().GetRef(),
			SHA:    pr.GetBase().GetSHA(),
		},
		Labels: githubLabels(pr),
	}
}

func githubLabels(pr *github.PullRequest) []*Label {
	labels := make([]*Label, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, &Label{
			Text: l.GetName(),
		})
	}
	return labels
}

func (h *GitHubHandler) downloadComments(ctx context.Context, rawPath string) ([]*Comment, error) {
	commentsPrefix := filepath.Join(rawPath, "comments")
	for _, p := range []string{commentsPrefix} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return nil, err
		}
	}
	ic, _, err := h.Issues.ListComments(ctx, h.owner, h.repo, h.prNum, nil)
	if err != nil {
		return nil, err
	}
	comments := make([]*Comment, 0, len(ic))
	for _, c := range ic {
		rawComment := filepath.Join(commentsPrefix, fmt.Sprintf("%d.json", c.GetID()))
		h.Logger.Infof("Writing comment %d to file: %s", c.GetID(), rawComment)
		if err := writeJSON(rawComment, c); err != nil {
			return nil, err
		}

		comment := &Comment{
			Author: c.GetUser().GetLogin(),
			Text:   c.GetBody(),
			ID:     c.GetID(),

			Raw: rawComment,
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

// readJSON reads an arbitrary JSON payload from path and decodes it into the
// given interface.
func readJSON(path string, i interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return json.NewDecoder(f).Decode(i)
}

// Upload takes files stored on the filesystem and uploads new changes to
// GitHub.
func (h *GitHubHandler) Upload(ctx context.Context, path string) error {
	h.Logger.Infof("Syncing path: %s to pr %d", path, h.prNum)

	// TODO: Allow syncing from GitHub specific sources.

	prPath := filepath.Join(path, prFile)
	pr := new(PullRequest)
	if err := readJSON(prPath, pr); err != nil {
		return err
	}

	var merr error

	if err := h.uploadStatuses(ctx, pr.Head.SHA, pr.Statuses); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := h.uploadLabels(ctx, pr.Labels); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := h.uploadComments(ctx, pr.Comments); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

func (h *GitHubHandler) uploadLabels(ctx context.Context, labels []*Label) error {
	labelNames := make([]string, 0, len(labels))
	for _, l := range labels {
		labelNames = append(labelNames, l.Text)
	}
	h.Logger.Infof("Setting labels for PR %d to %v", h.prNum, labelNames)
	_, _, err := h.Issues.ReplaceLabelsForIssue(ctx, h.owner, h.repo, h.prNum, labelNames)
	return err
}

func (h *GitHubHandler) uploadComments(ctx context.Context, comments []*Comment) error {
	h.Logger.Infof("Setting comments for PR %d to: %v", h.prNum, comments)

	// Sort comments into whether they are new or existing comments (based on
	// whether there is an ID defined).
	existingComments := map[int64]*Comment{}
	newComments := []*Comment{}
	for _, c := range comments {
		if c.ID != 0 {
			existingComments[c.ID] = c
		} else {
			newComments = append(newComments, c)
		}
	}

	var merr error
	if err := h.updateExistingComments(ctx, existingComments); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := h.createNewComments(ctx, newComments); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

func (h *GitHubHandler) updateExistingComments(ctx context.Context, comments map[int64]*Comment) error {
	existingComments, _, err := h.Issues.ListComments(ctx, h.owner, h.repo, h.prNum, nil)
	if err != nil {
		return err
	}

	h.Logger.Info(existingComments)
	h.Logger.Info(comments)

	var merr error
	for _, ec := range existingComments {
		dc, ok := comments[ec.GetID()]
		if !ok {
			// Delete
			h.Logger.Infof("Deleting comment %d for PR %d", ec.GetID(), h.prNum)
			if _, err := h.Issues.DeleteComment(ctx, h.owner, h.repo, ec.GetID()); err != nil {
				h.Logger.Warnf("Error deleting comment: %v", err)
				merr = multierror.Append(merr, err)
				continue
			}
		} else if dc.Text != ec.GetBody() {
			// Update
			c := &github.IssueComment{
				ID:   ec.ID,
				Body: github.String(dc.Text),
				User: ec.User,
			}
			h.Logger.Infof("Updating comment %d for PR %d to %s", ec.GetID(), h.prNum, dc.Text)
			if _, _, err := h.Issues.EditComment(ctx, h.owner, h.repo, ec.GetID(), c); err != nil {
				h.Logger.Warnf("Error editing comment: %v", err)
				merr = multierror.Append(merr, err)
				continue
			}
		}
	}
	return merr
}

func (h *GitHubHandler) createNewComments(ctx context.Context, comments []*Comment) error {
	var merr error
	for _, dc := range comments {
		c := &github.IssueComment{
			Body: github.String(dc.Text),
		}
		h.Logger.Infof("Creating comment %s for PR %d", dc.Text, h.prNum)
		if _, _, err := h.Issues.CreateComment(ctx, h.owner, h.repo, h.prNum, c); err != nil {
			h.Logger.Warnf("Error creating comment: %v", err)
			merr = multierror.Append(merr, err)
		}
	}
	return merr
}

func (h *GitHubHandler) getStatuses(ctx context.Context, sha string, path string) ([]*Status, error) {
	resp, _, err := h.Repositories.GetCombinedStatus(ctx, h.owner, h.repo, sha, nil)
	if err != nil {
		return nil, err
	}
	if err := writeJSON(path, resp); err != nil {
		return nil, err
	}

	statuses := make([]*Status, 0, len(resp.Statuses))
	for _, s := range resp.Statuses {
		code, ok := toTekton[s.GetState()]
		if !ok {
			return nil, fmt.Errorf("unknown GitHub status state: %s", s.GetState())
		}
		statuses = append(statuses, &Status{
			ID:          s.GetContext(),
			Code:        code,
			Description: s.GetDescription(),
			URL:         s.GetTargetURL(),
		})
	}
	return statuses, nil
}

func (h *GitHubHandler) uploadStatuses(ctx context.Context, sha string, statuses []*Status) error {
	var merr error

	for _, s := range statuses {
		state, ok := toGitHub[s.Code]
		if !ok {
			merr = multierror.Append(merr, fmt.Errorf("unknown status code %s", s.Code))
			continue
		}

		rs := &github.RepoStatus{
			Context:     github.String(s.ID),
			State:       github.String(state),
			Description: github.String(s.Description),
			TargetURL:   github.String(s.URL),
		}
		if _, _, err := h.Client.Repositories.CreateStatus(ctx, h.owner, h.repo, sha, rs); err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
	}

	return merr
}
