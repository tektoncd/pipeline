// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/go-scm/scm"
)

// TODO(bradrydzewski) default repository branch is missing in push webhook payloads
// TODO(bradrydzewski) default repository branch is missing in pr webhook payloads
// TODO(bradrydzewski) default repository is_private is missing in pr webhook payloads

type webhookService struct {
	client *wrapper
}

// Parse for the bitbucket cloud webhook payloads see: https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/
func (s *webhookService) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {
	data, err := io.ReadAll(
		io.LimitReader(req.Body, 10000000),
	)
	if err != nil {
		return nil, err
	}

	guid := req.Header.Get("X-Hook-UUID")

	var hook scm.Webhook
	switch req.Header.Get("x-event-key") {
	case "repo:push":
		hook, err = s.parsePushHook(data, guid)
	case "pullrequest:created":
		hook, err = s.parsePullRequestHook(data)
	case "pullrequest:updated":
		hook, err = s.parsePullRequestHook(data)
		hook.(*scm.PullRequestHook).Action = scm.ActionSync
	case "pullrequest:fulfilled":
		hook, err = s.parsePullRequestHook(data)
		hook.(*scm.PullRequestHook).Action = scm.ActionMerge
	case "pullrequest:rejected":
		hook, err = s.parsePullRequestHook(data)
		hook.(*scm.PullRequestHook).Action = scm.ActionClose
	case "pullrequest:comment_created", "pullrequest:comment_updated":
		hook, err = s.parsePullRequestCommentHook(data)
		hook.(*scm.PullRequestCommentHook).Action = scm.ActionCreate
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

	if req.FormValue("secret") != key {
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
	if len(dst.Push.Changes) == 0 {
		return nil, errors.New("Push hook has empty changeset")
	}
	change := dst.Push.Changes[0]
	switch {
	// case change.New.Type == "branch" && change.Created:
	// return convertBranchCreateHook(dst), nil
	case change.Old.Type == "branch" && change.Closed:
		return convertBranchDeleteHook(dst), nil
	// case change.New.Type == "tag" && change.Created:
	// return convertTagCreateHook(dst), nil
	case change.Old.Type == "tag" && change.Closed:
		return convertTagDeleteHook(dst), nil
	default:
		hook, err := s.convertPushHook(dst)
		if err != nil {
			return nil, err
		}
		hook.GUID = guid
		return hook, nil
	}
}

func (s *webhookService) parsePullRequestHook(data []byte) (*scm.PullRequestHook, error) {
	dst := new(webhook)
	err := json.Unmarshal(data, dst)
	if err != nil {
		return nil, err
	}

	return s.convertPullRequestHook(dst)
}

func (s *webhookService) parsePullRequestCommentHook(data []byte) (*scm.PullRequestCommentHook, error) {
	dst := new(webhookPRComment)
	err := json.Unmarshal(data, dst)
	if err != nil {
		return nil, err
	}
	return s.convertPullRequestCommentHook(dst)
}

//
// native data structures
//

type (
	pushHook struct {
		Push struct {
			Changes []struct {
				Forced bool `json:"forced"`
				Old    struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Links struct {
						Commits struct {
							Href string `json:"href"`
						} `json:"commits"`
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
					Target struct {
						Hash  string `json:"hash"`
						Links struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
						} `json:"links"`
						Author struct {
							Raw  string `json:"raw"`
							Type string `json:"type"`
							User struct {
								Username    string `json:"username"`
								DisplayName string `json:"display_name"`
								AccountID   string `json:"account_id"`
								Links       struct {
									Self struct {
										Href string `json:"href"`
									} `json:"self"`
									HTML struct {
										Href string `json:"href"`
									} `json:"html"`
									Avatar struct {
										Href string `json:"href"`
									} `json:"avatar"`
								} `json:"links"`
								Type string `json:"type"`
								UUID string `json:"uuid"`
							} `json:"user"`
						} `json:"author"`
						Summary struct {
							Raw    string `json:"raw"`
							Markup string `json:"markup"`
							HTML   string `json:"html"`
							Type   string `json:"type"`
						} `json:"summary"`
						Parents []interface{} `json:"parents"`
						Date    time.Time     `json:"date"`
						Message string        `json:"message"`
						Type    string        `json:"type"`
					} `json:"target"`
				} `json:"old"`
				Links struct {
					Commits struct {
						Href string `json:"href"`
					} `json:"commits"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
					Diff struct {
						Href string `json:"href"`
					} `json:"diff"`
				} `json:"links"`
				Truncated bool `json:"truncated"`
				Commits   []struct {
					Hash  string `json:"hash"`
					Links struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						Comments struct {
							Href string `json:"href"`
						} `json:"comments"`
						Patch struct {
							Href string `json:"href"`
						} `json:"patch"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
						Diff struct {
							Href string `json:"href"`
						} `json:"diff"`
						Approve struct {
							Href string `json:"href"`
						} `json:"approve"`
						Statuses struct {
							Href string `json:"href"`
						} `json:"statuses"`
					} `json:"links"`
					Author struct {
						Raw  string `json:"raw"`
						Type string `json:"type"`
						User struct {
							Username    string `json:"username"`
							DisplayName string `json:"display_name"`
							AccountID   string `json:"account_id"`
							Links       struct {
								Self struct {
									Href string `json:"href"`
								} `json:"self"`
								HTML struct {
									Href string `json:"href"`
								} `json:"html"`
								Avatar struct {
									Href string `json:"href"`
								} `json:"avatar"`
							} `json:"links"`
							Type string `json:"type"`
							UUID string `json:"uuid"`
						} `json:"user"`
					} `json:"author"`
					Summary struct {
						Raw    string `json:"raw"`
						Markup string `json:"markup"`
						HTML   string `json:"html"`
						Type   string `json:"type"`
					} `json:"summary"`
					Parents []struct {
						Type  string `json:"type"`
						Hash  string `json:"hash"`
						Links struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
						} `json:"links"`
					} `json:"parents"`
					Date    time.Time `json:"date"`
					Message string    `json:"message"`
					Type    string    `json:"type"`
				} `json:"commits"`
				Created bool `json:"created"`
				Closed  bool `json:"closed"`
				New     struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Links struct {
						Commits struct {
							Href string `json:"href"`
						} `json:"commits"`
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
					Target struct {
						Hash  string `json:"hash"`
						Links struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
						} `json:"links"`
						Author struct {
							Raw  string `json:"raw"`
							Type string `json:"type"`
							User struct {
								Username    string `json:"username"`
								DisplayName string `json:"display_name"`
								AccountID   string `json:"account_id"`
								Links       struct {
									Self struct {
										Href string `json:"href"`
									} `json:"self"`
									HTML struct {
										Href string `json:"href"`
									} `json:"html"`
									Avatar struct {
										Href string `json:"href"`
									} `json:"avatar"`
								} `json:"links"`
								Type string `json:"type"`
								UUID string `json:"uuid"`
							} `json:"user"`
						} `json:"author"`
						Summary struct {
							Raw    string `json:"raw"`
							Markup string `json:"markup"`
							HTML   string `json:"html"`
							Type   string `json:"type"`
						} `json:"summary"`
						Parents []struct {
							Type  string `json:"type"`
							Hash  string `json:"hash"`
							Links struct {
								Self struct {
									Href string `json:"href"`
								} `json:"self"`
								HTML struct {
									Href string `json:"href"`
								} `json:"html"`
							} `json:"links"`
						} `json:"parents"`
						Date    time.Time `json:"date"`
						Message string    `json:"message"`
						Type    string    `json:"type"`
					} `json:"target"`
				} `json:"new"`
			} `json:"changes"`
		} `json:"push"`
		Repository webhookRepository `json:"repository"`
		Actor      webhookActor      `json:"actor"`
	}

	webhook struct {
		PullRequest webhookPullRequest `json:"pullrequest"`
		Repository  webhookRepository  `json:"repository"`
		Actor       webhookActor       `json:"actor"`
	}

	webhookPullRequest struct {
		Description string `json:"description"`
		Links       struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Diff struct {
				Href string `json:"href"`
			} `json:"diff"`
		} `json:"links"`
		Title       string `json:"title"`
		ID          int    `json:"id"`
		Destination struct {
			Commit struct {
				Hash  string `json:"hash"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
				} `json:"links"`
			} `json:"commit"`
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Repository struct {
				FullName string `json:"full_name"`
				Type     string `json:"type"`
				Name     string `json:"name"`
				Links    struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
					Avatar struct {
						Href string `json:"href"`
					} `json:"avatar"`
				} `json:"links"`
				UUID string `json:"uuid"`
			} `json:"repository"`
		} `json:"destination"`
		CommentCount int `json:"comment_count"`
		Summary      struct {
			Raw    string `json:"raw"`
			Markup string `json:"markup"`
			HTML   string `json:"html"`
			Type   string `json:"type"`
		} `json:"summary"`
		Source struct {
			Commit struct {
				Hash  string `json:"hash"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
				} `json:"links"`
			} `json:"commit"`
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Repository struct {
				FullName string `json:"full_name"`
				Type     string `json:"type"`
				Name     string `json:"name"`
				Links    struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
					Avatar struct {
						Href string `json:"href"`
					} `json:"avatar"`
				} `json:"links"`
				UUID string `json:"uuid"`
			} `json:"repository"`
		} `json:"source"`
		State  string `json:"state"`
		Author struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			AccountID   string `json:"account_id"`
			Links       struct {
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
				Avatar struct {
					Href string `json:"href"`
				} `json:"avatar"`
			} `json:"links"`
			Type string `json:"type"`
			UUID string `json:"uuid"`
		} `json:"author"`
		CreatedOn time.Time `json:"created_on"`
		UpdatedOn time.Time `json:"updated_on"`
	}

	webhookRepository struct {
		Scm   string `json:"scm"`
		Name  string `json:"name"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		FullName string `json:"full_name"`
		Owner    struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			AccountID   string `json:"account_id"`
			Links       struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			UUID string `json:"uuid"`
		} `json:"owner"`
		IsPrivate bool   `json:"is_private"`
		UUID      string `json:"uuid"`
	}

	webhookActor struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		AccountID   string `json:"account_id"`
		Links       struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
		UUID string `json:"uuid"`
	}
)

type webhookPRComment struct {
	PullRequest *webhookPullRequest `json:"pullrequest"`
	Comment     *prComment          `json:"comment"` // this struct definition is available in pr.go
	Repository  *webhookRepository  `json:"repository"`
	Actor       *webhookActor       `json:"actor"`
}

//
// push hooks
//

func (s *webhookService) convertPushHook(src *pushHook) (*scm.PushHook, error) {
	change := src.Push.Changes[0]
	namespace, name := scm.Split(src.Repository.FullName)
	repo := scm.Repository{
		ID:        src.Repository.UUID,
		Namespace: namespace,
		Name:      name,
		FullName:  src.Repository.FullName,
		Private:   src.Repository.IsPrivate,
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.Repository.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.Repository.FullName),
		Link:      src.Repository.Links.HTML.Href,
	}
	if repo.FullName == "" {
		repo.FullName = scm.Join(repo.Namespace, repo.Name)
	}
	sha := change.New.Target.Hash
	dst := &scm.PushHook{
		Ref: scm.ExpandRef(change.New.Name, "refs/heads/"),
		Commit: scm.Commit{
			Sha:     sha,
			Message: change.New.Target.Message,
			Link:    change.New.Target.Links.HTML.Href,
			Author: scm.Signature{
				Login:  validUser(change.New.Target.Author.User.AccountID, change.New.Target.Author.User.Username),
				Email:  extractEmail(change.New.Target.Author.Raw),
				Name:   change.New.Target.Author.User.DisplayName,
				Avatar: change.New.Target.Author.User.Links.Avatar.Href,
				Date:   change.New.Target.Date,
			},
			Committer: scm.Signature{
				Login:  validUser(change.New.Target.Author.User.AccountID, change.New.Target.Author.User.Username),
				Email:  extractEmail(change.New.Target.Author.Raw),
				Name:   change.New.Target.Author.User.DisplayName,
				Avatar: change.New.Target.Author.User.Links.Avatar.Href,
				Date:   change.New.Target.Date,
			},
		},
		Repo: repo,
		Sender: scm.User{
			Login:  validUser(src.Actor.AccountID, src.Actor.Username),
			Name:   src.Actor.DisplayName,
			Avatar: src.Actor.Links.Avatar.Href,
		},
	}
	if change.New.Type == "tag" {
		dst.Ref = scm.ExpandRef(change.New.Name, "refs/tags/")
	}
	dst.After = sha
	if len(sha) <= 12 && sha != "" {
		// lets convert to a full hash
		repo := repo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, sha)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve full hash %s", sha)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.After = fullHash
		}
	}
	return dst, nil
}

func convertBranchDeleteHook(src *pushHook) *scm.BranchHook {
	namespace, name := scm.Split(src.Repository.FullName)
	change := src.Push.Changes[0].Old
	action := scm.ActionDelete
	return &scm.BranchHook{
		Action: action,
		Ref: scm.Reference{
			Name: change.Name,
			Sha:  change.Target.Hash,
		},
		Repo: scm.Repository{
			ID:        src.Repository.UUID,
			Namespace: namespace,
			Name:      name,
			FullName:  src.Repository.FullName,
			Private:   src.Repository.IsPrivate,
			Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.Repository.FullName),
			CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.Repository.FullName),
			Link:      src.Repository.Links.HTML.Href,
		},
		Sender: scm.User{
			Login:  validUser(src.Actor.AccountID, src.Actor.Username),
			Name:   src.Actor.DisplayName,
			Avatar: src.Actor.Links.Avatar.Href,
		},
	}
}

func convertTagDeleteHook(src *pushHook) *scm.TagHook {
	namespace, name := scm.Split(src.Repository.FullName)
	change := src.Push.Changes[0].Old
	action := scm.ActionDelete
	return &scm.TagHook{
		Action: action,
		Ref: scm.Reference{
			Name: change.Name,
			Sha:  change.Target.Hash,
		},
		Repo: scm.Repository{
			ID:        src.Repository.UUID,
			Namespace: namespace,
			Name:      name,
			FullName:  src.Repository.FullName,
			Private:   src.Repository.IsPrivate,
			Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.Repository.FullName),
			CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.Repository.FullName),
			Link:      src.Repository.Links.HTML.Href,
		},
		Sender: scm.User{
			Login:  validUser(src.Actor.AccountID, src.Actor.Username),
			Name:   src.Actor.DisplayName,
			Avatar: src.Actor.Links.Avatar.Href,
		},
	}
}

// TODO, this is hack to support 2.0 API amendment.
// username is unavailable in response since 2.0 release
// this hack may not be needed since other sources are assuming 2.0 API version
// return account Id in case user name is empty
func validUser(acID, userName string) string {
	result := userName

	if userName == "" {
		result = acID
	}

	return result
}

//
// pull request hooks
//

func (s *webhookService) convertPullRequestHook(src *webhook) (*scm.PullRequestHook, error) {
	namespace, name := scm.Split(src.Repository.FullName)
	repo := scm.Repository{
		ID:        src.Repository.UUID,
		Namespace: namespace,
		Name:      name,
		FullName:  src.Repository.FullName,
		Private:   src.Repository.IsPrivate,
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.Repository.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.Repository.FullName),
		Link:      src.Repository.Links.HTML.Href,
	}
	sha := src.PullRequest.Source.Commit.Hash
	dst := &scm.PullRequestHook{
		Action: scm.ActionOpen,
		PullRequest: scm.PullRequest{
			Number: src.PullRequest.ID,
			Title:  src.PullRequest.Title,
			Body:   src.PullRequest.Description,
			Sha:    sha,
			Ref:    fmt.Sprintf("refs/pull-requests/%d/from", src.PullRequest.ID),
			Source: src.PullRequest.Source.Branch.Name,
			Target: src.PullRequest.Destination.Branch.Name,
			Fork:   src.PullRequest.Source.Repository.FullName,
			Link:   src.PullRequest.Links.HTML.Href,
			Closed: src.PullRequest.State != "OPEN",
			Merged: src.PullRequest.State == "MERGED",
			Author: scm.User{
				Login:  validUser(src.PullRequest.Author.AccountID, src.PullRequest.Author.Username),
				Name:   src.PullRequest.Author.DisplayName,
				Avatar: src.PullRequest.Author.Links.Avatar.Href,
			},
			Created: src.PullRequest.CreatedOn,
			Updated: src.PullRequest.UpdatedOn,
		},
		Repo: repo,
		Sender: scm.User{
			Login:  validUser(src.Actor.AccountID, src.Actor.Username),
			Name:   src.Actor.DisplayName,
			Avatar: src.Actor.Links.Avatar.Href,
		},
	}
	dst.PullRequest.Base.Repo = repo
	dst.PullRequest.Base.Ref = src.PullRequest.Destination.Branch.Name
	dst.PullRequest.Head.Ref = src.PullRequest.Source.Branch.Name

	if len(sha) <= 12 && sha != "" && s.client != nil {
		// lets convert to a full hash
		repo := repo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, sha)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve full hash %s", sha)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.PullRequest.Sha = fullHash
		}
	}
	if dst.PullRequest.Head.Sha == "" && dst.PullRequest.Head.Ref != "" && s.client != nil {
		repo := repo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, dst.PullRequest.Head.Ref)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve sha for ref %s", dst.PullRequest.Head.Ref)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.PullRequest.Head.Sha = fullHash
		}
	}
	return dst, nil
}

func (s *webhookService) convertPullRequestCommentHook(src *webhookPRComment) (*scm.PullRequestCommentHook, error) {
	namespace, name := scm.Split(src.PullRequest.Source.Repository.FullName)
	prRepo := scm.Repository{
		ID:        src.Repository.UUID,
		Namespace: namespace,
		Name:      src.Repository.Name,
		FullName:  src.Repository.FullName,
		Branch:    src.PullRequest.Destination.Branch.Name,
		Private:   src.Repository.IsPrivate,
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.Repository.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.Repository.FullName),
		Link:      src.Repository.Links.HTML.Href,
	}

	featureRepo := scm.Repository{
		ID:        src.PullRequest.Source.Repository.UUID,
		Namespace: namespace,
		Name:      name,
		FullName:  src.PullRequest.Source.Repository.FullName,
		Branch:    src.PullRequest.Source.Branch.Name,
		Private:   true, // (TODO) Private value is set to default(true) as this value does not come with the PR Source Repo payload
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.PullRequest.Source.Repository.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.PullRequest.Source.Repository.FullName),
		Link:      src.PullRequest.Source.Repository.Links.HTML.Href,
	}
	baseRepo := scm.Repository{
		ID:        src.PullRequest.Destination.Repository.UUID,
		Namespace: namespace,
		Name:      name,
		FullName:  src.PullRequest.Destination.Repository.FullName,
		Branch:    src.PullRequest.Destination.Branch.Name,
		Private:   true, // (TODO) Private value is set to default(true) as this value does not come with the PR Destination Repo payload
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", src.PullRequest.Destination.Repository.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", src.PullRequest.Destination.Repository.FullName),
		Link:      src.PullRequest.Destination.Repository.Links.HTML.Href,
	}
	featureSha := src.PullRequest.Source.Commit.Hash

	dst := &scm.PullRequestCommentHook{
		Action: scm.ActionCreate,
		PullRequest: scm.PullRequest{
			Number: src.PullRequest.ID,
			Title:  src.PullRequest.Title,
			Body:   src.PullRequest.Description,
			Sha:    featureSha,
			Ref:    fmt.Sprintf("refs/pull-requests/%d/from", src.PullRequest.ID),
			Source: src.PullRequest.Source.Branch.Name,
			Target: src.PullRequest.Destination.Branch.Name,
			Fork:   src.PullRequest.Source.Repository.FullName,
			Link:   src.PullRequest.Links.HTML.Href,
			Closed: src.PullRequest.State != "OPEN",
			Merged: src.PullRequest.State == "MERGED",
			Author: scm.User{
				Login:  validUser(src.PullRequest.Author.AccountID, src.PullRequest.Author.Username),
				Name:   src.PullRequest.Author.DisplayName,
				Avatar: src.PullRequest.Author.Links.Avatar.Href,
			},
			Created: src.PullRequest.CreatedOn,
			Updated: src.PullRequest.UpdatedOn,
			State:   strings.ToLower(src.PullRequest.State),
		},
		Repo: prRepo,
		Sender: scm.User{
			Login:  validUser(src.Actor.AccountID, src.Actor.Username),
			Name:   src.Actor.DisplayName,
			Avatar: src.Actor.Links.Avatar.Href,
		},
		Comment: scm.Comment{
			ID:   src.Comment.ID,
			Body: src.Comment.Content.Raw,
			Author: scm.User{
				Login:  src.Comment.User.AccountID,
				Name:   src.Comment.User.DisplayName,
				Avatar: src.Comment.User.Links.Avatar.Href,
			},
			Created: src.Comment.CreatedOn,
			Updated: src.Comment.UpdatedOn,
		},
	}
	dst.PullRequest.Base.Repo = baseRepo
	dst.PullRequest.Head.Repo = featureRepo
	dst.PullRequest.Base.Ref = src.PullRequest.Destination.Branch.Name
	dst.PullRequest.Head.Ref = src.PullRequest.Source.Branch.Name

	if len(featureSha) <= 12 && featureSha != "" && s.client != nil {
		// TODO - need to consider the forking scenario to determine which Repo whould be considered for mapping "repo" variable Full name
		repo := featureRepo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, featureSha)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve full hash %s", featureSha)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.PullRequest.Sha = fullHash
		}
	}

	if dst.PullRequest.Head.Sha == "" && dst.PullRequest.Head.Ref != "" && s.client != nil {
		repo := featureRepo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, dst.PullRequest.Head.Ref)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve sha for ref %s", dst.PullRequest.Head.Ref)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.PullRequest.Head.Sha = fullHash
		}
	}

	if dst.PullRequest.Head.Sha == "" {
		dst.PullRequest.Head.Sha = dst.PullRequest.Sha
	}

	masterSha := src.PullRequest.Destination.Commit.Hash
	if len(masterSha) <= 12 && masterSha != "" && s.client != nil {
		repo := baseRepo.FullName
		fullHash, _, err := s.client.Git.FindRef(context.TODO(), repo, masterSha)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve full hash %s", masterSha)
		}
		fullHash = strings.TrimSpace(fullHash)
		if fullHash != "" {
			dst.PullRequest.Base.Sha = fullHash
		}
	}
	return dst, nil
}
