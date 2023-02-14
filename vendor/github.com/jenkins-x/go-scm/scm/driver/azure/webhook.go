// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type webhookService struct {
	client *wrapper
}

func (s *webhookService) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {
	data, err := io.ReadAll(
		io.LimitReader(req.Body, 10000000),
	)
	if err != nil {
		return nil, err
	}
	// we need to read the json data then look at the eventType
	var unstructuredJSON map[string]interface{}
	jsonErr := json.Unmarshal(data, &unstructuredJSON)
	if jsonErr != nil {
		return nil, fmt.Errorf("Error parsing JSON from webhook: %s", jsonErr)
	}
	eventType := unstructuredJSON["eventType"].(string)

	switch eventType {
	case "git.push":
		// https://docs.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.push
		src := new(pushHook)
		err := json.Unmarshal(data, src)
		if err != nil {
			return nil, err
		}
		dst := convertPushHook(src)
		return dst, nil
	case "git.pullrequest.created":
		// https://docs.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.pullrequest.created
		src := new(createPullRequestHook)
		err := json.Unmarshal(data, src)
		if err != nil {
			return nil, err
		}
		dst := convertCreatePullRequestHook(src)
		dst.Action = scm.ActionCreate
		return dst, nil
	case "git.pullrequest.updated":
		// https://docs.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.pullrequest.updated
		src := new(updatePullRequestHook)
		err := json.Unmarshal(data, src)
		if err != nil {
			return nil, err
		}
		dst := convertUpdatePullRequestHook(src)
		dst.Action = scm.ActionUpdate
		return dst, nil
	case "git.pullrequest.merged":
		// https://docs.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.pullrequest.merged
		src := new(mergePullRequestHook)
		err := json.Unmarshal(data, src)
		if err != nil {
			return nil, err
		}
		dst := convertMergePullRequestHook(src)
		dst.Action = scm.ActionMerge
		return dst, nil
	case "ms.vss-code.git-pullrequest-comment-event":
		src := new(issueCommentPullRequestHook)
		err := json.Unmarshal(data, src)
		if err != nil {
			return nil, err
		}
		dst := convertIssueCommentHook(src)
		dst.Action = getIssueCommentAction(src)
		return dst, nil
	default:
		return nil, scm.ErrUnknownEvent
	}
}

func getIssueCommentAction(src *issueCommentPullRequestHook) scm.Action {
	if src.Resource.Comment.IsDeleted {
		return scm.ActionDelete
	} else if src.Resource.Comment.PublishedDate.Equal(src.Resource.Comment.LastUpdatedDate) {
		return scm.ActionCreate
	} else {
		return scm.ActionEdited
	}
}

func convertPushHook(src *pushHook) *scm.PushHook {
	var commits []scm.PushCommit
	for i := range src.Resource.Commits {
		commits = append(commits,
			scm.PushCommit{
				//Sha:     c.CommitID,
				Message: src.Resource.Commits[i].Comment,
				//Link:    c.URL,
				//Author: scm.Signature{
				//	Login: c.Author.Name,
				//	Email: c.Author.Email,
				//	Name:  c.Author.Name,
				//	Date:  c.Author.Date,
				// },
				//Committer: scm.Signature{
				//	Login: c.Committer.Name,
				//	Email: c.Committer.Email,
				//	Name:  c.Committer.Name,
				//	Date:  c.Committer.Date,
				// },
			})
	}
	dst := &scm.PushHook{
		Commit: scm.Commit{
			Sha:     src.Resource.RefUpdates[0].NewObjectID,
			Message: "",
			Author: scm.Signature{
				Login:  src.Resource.PushedBy.ID,
				Name:   src.Resource.PushedBy.DisplayName,
				Email:  src.Resource.PushedBy.UniqueName,
				Avatar: src.Resource.PushedBy.ImageURL,
			},
			Committer: scm.Signature{
				Login:  src.Resource.PushedBy.ID,
				Name:   src.Resource.PushedBy.DisplayName,
				Email:  src.Resource.PushedBy.UniqueName,
				Avatar: src.Resource.PushedBy.ImageURL,
			},
			Link: "",
		},
		Ref:    src.Resource.RefUpdates[0].Name,
		Before: src.Resource.RefUpdates[0].OldObjectID,
		After:  src.Resource.RefUpdates[0].NewObjectID,
		Sender: scm.User{
			Login:  src.Resource.PushedBy.ID,
			Name:   src.Resource.PushedBy.DisplayName,
			Email:  src.Resource.PushedBy.UniqueName,
			Avatar: src.Resource.PushedBy.ImageURL,
		},
		Repo: scm.Repository{
			ID:        src.Resource.Repository.ID,
			Branch:    scm.TrimRef(src.Resource.Repository.DefaultBranch),
			Name:      src.Resource.Repository.Name,
			Namespace: src.Resource.Repository.Project.Name,
			Clone:     src.Resource.Repository.RemoteURL,
			Link:      src.Resource.Repository.RemoteURL,
		},
		Commits: commits,
	}
	return dst
}

func convertCreatePullRequestHook(src *createPullRequestHook) (returnVal *scm.PullRequestHook) {
	returnVal = &scm.PullRequestHook{
		PullRequest: scm.PullRequest{
			Number: src.Resource.PullRequestID,
			Title:  src.Resource.Title,
			Body:   src.Resource.Description,
			Sha:    src.Resource.LastMergeSourceCommit.CommitID,
			Ref:    src.Resource.SourceRefName,
			Source: scm.TrimRef(src.Resource.SourceRefName),
			Target: scm.TrimRef(src.Resource.TargetRefName),
			Link:   src.Resource.URL,
			Closed: false,
			Merged: false,
			Author: scm.User{
				Login:  src.Resource.CreatedBy.DisplayName,
				Name:   src.Resource.CreatedBy.DisplayName,
				Email:  src.Resource.CreatedBy.UniqueName,
				Avatar: src.Resource.CreatedBy.ImageURL,
			},
			Created: src.Resource.CreationDate,
		},
		Repo: scm.Repository{
			ID:        src.Resource.Repository.ID,
			Name:      src.Resource.Repository.Name,
			Namespace: src.Resource.Repository.Project.Name,
			Link:      src.Resource.Repository.WebURL,
			Clone:     src.Resource.Repository.WebURL,
			CloneSSH:  src.Resource.Repository.SSHURL,
		},
		Sender: scm.User{
			Login:  src.Resource.CreatedBy.ID,
			Name:   src.Resource.CreatedBy.DisplayName,
			Email:  src.Resource.CreatedBy.UniqueName,
			Avatar: src.Resource.CreatedBy.ImageURL,
		},
	}
	return returnVal
}

func convertUpdatePullRequestHook(src *updatePullRequestHook) (returnVal *scm.PullRequestHook) {
	returnVal = &scm.PullRequestHook{
		PullRequest: scm.PullRequest{
			Number: src.Resource.PullRequestID,
			Title:  src.Resource.Title,
			Body:   src.Resource.Description,
			Sha:    src.Resource.LastMergeSourceCommit.CommitID,
			Ref:    src.Resource.SourceRefName,
			Source: scm.TrimRef(src.Resource.SourceRefName),
			Target: scm.TrimRef(src.Resource.TargetRefName),
			Link:   src.Resource.URL,
			Closed: src.Resource.ClosedDate.Valid,
			Merged: false,
			Author: scm.User{
				Login:  src.Resource.CreatedBy.DisplayName,
				Name:   src.Resource.CreatedBy.DisplayName,
				Email:  src.Resource.CreatedBy.UniqueName,
				Avatar: src.Resource.CreatedBy.ImageURL,
			},
			Created: src.Resource.CreationDate,
		},
		Repo: scm.Repository{
			ID:        src.Resource.Repository.ID,
			Name:      src.Resource.Repository.Name,
			Namespace: src.Resource.Repository.Project.Name,
			Link:      src.Resource.Repository.WebURL,
			Clone:     src.Resource.Repository.WebURL,
			CloneSSH:  src.Resource.Repository.SSHURL,
		},
		Sender: scm.User{
			Login:  src.Resource.CreatedBy.ID,
			Name:   src.Resource.CreatedBy.DisplayName,
			Email:  src.Resource.CreatedBy.UniqueName,
			Avatar: src.Resource.CreatedBy.ImageURL,
		},
	}
	return returnVal
}

func convertMergePullRequestHook(src *mergePullRequestHook) (returnVal *scm.PullRequestHook) {
	returnVal = &scm.PullRequestHook{
		PullRequest: scm.PullRequest{
			Number: src.Resource.PullRequestID,
			Title:  src.Resource.Title,
			Body:   src.Resource.Description,
			Sha:    src.Resource.LastMergeSourceCommit.CommitID,
			Ref:    src.Resource.SourceRefName,
			Source: scm.TrimRef(src.Resource.SourceRefName),
			Target: scm.TrimRef(src.Resource.TargetRefName),
			Link:   src.Resource.URL,
			Closed: false,
			Merged: true,
			Author: scm.User{
				Login:  src.Resource.CreatedBy.DisplayName,
				Name:   src.Resource.CreatedBy.DisplayName,
				Email:  src.Resource.CreatedBy.UniqueName,
				Avatar: src.Resource.CreatedBy.ImageURL,
			},
			Created: src.Resource.CreationDate,
		},
		Repo: scm.Repository{
			ID:        src.Resource.Repository.ID,
			Name:      src.Resource.Repository.Name,
			Namespace: src.Resource.Repository.Project.Name,
			Link:      src.Resource.Repository.WebURL,
			Clone:     src.Resource.Repository.WebURL,
			CloneSSH:  src.Resource.Repository.SSHURL,
		},
		Sender: scm.User{
			Login:  src.Resource.CreatedBy.ID,
			Name:   src.Resource.CreatedBy.DisplayName,
			Email:  src.Resource.CreatedBy.UniqueName,
			Avatar: src.Resource.CreatedBy.ImageURL,
		},
	}
	return returnVal
}

func convertIssueCommentHook(src *issueCommentPullRequestHook) *scm.IssueCommentHook {
	dst := &scm.IssueCommentHook{
		Repo: scm.Repository{
			ID:        src.Resource.PullRequest.Repository.ID,
			Namespace: src.Resource.PullRequest.Repository.Project.Name,
			Name:      src.Resource.PullRequest.Repository.Name,
			Branch:    scm.TrimRef(src.Resource.PullRequest.SourceRefName),
			Private:   false,
			Clone:     src.Resource.PullRequest.Repository.WebURL,
			CloneSSH:  src.Resource.PullRequest.Repository.SSHURL,
			Link:      src.Resource.PullRequest.Repository.WebURL,
		},
		Issue: scm.Issue{
			Number: src.Resource.PullRequest.PullRequestID,
			Title:  src.Resource.PullRequest.Title,
			Body:   src.Resource.PullRequest.Description,
			Link:   src.Resource.PullRequest.URL,
			Author: scm.User{
				Login:  src.Resource.PullRequest.CreatedBy.DisplayName,
				Name:   src.Resource.PullRequest.CreatedBy.DisplayName,
				Email:  src.Resource.PullRequest.CreatedBy.UniqueName,
				Avatar: src.Resource.PullRequest.CreatedBy.ImageURL,
			},
			PullRequest: &scm.PullRequest{
				Number: src.Resource.PullRequest.PullRequestID,
				Title:  src.Resource.PullRequest.Title,
				Body:   src.Resource.PullRequest.Description,
				Sha:    src.Resource.PullRequest.LastMergeSourceCommit.CommitID,
				Ref:    src.Resource.PullRequest.SourceRefName,
				Source: scm.TrimRef(src.Resource.PullRequest.SourceRefName),
				Target: scm.TrimRef(src.Resource.PullRequest.TargetRefName),
				Link:   src.Resource.PullRequest.URL,
				Closed: false,
				Merged: false,
				Author: scm.User{
					Login:  src.Resource.PullRequest.CreatedBy.DisplayName,
					Name:   src.Resource.PullRequest.CreatedBy.DisplayName,
					Email:  src.Resource.PullRequest.CreatedBy.UniqueName,
					Avatar: src.Resource.PullRequest.CreatedBy.ImageURL,
				},
				Created: src.Resource.PullRequest.CreationDate,
			},
			Created: src.Resource.PullRequest.CreationDate,
		},
		Comment: scm.Comment{
			ID:   src.Resource.Comment.ID,
			Body: src.Resource.Comment.Content,
			Author: scm.User{
				Email: src.Resource.Comment.Author.UniqueName,
				Login: src.Resource.Comment.Author.ID,
				Name:  src.Resource.Comment.Author.DisplayName,
			},
			Created: src.Resource.Comment.PublishedDate,
			Updated: src.Resource.Comment.LastUpdatedDate,
		},
		Sender: scm.User{
			Email: src.Resource.Comment.Author.UniqueName,
			Login: src.Resource.Comment.Author.ID,
			Name:  src.Resource.Comment.Author.DisplayName,
		},
	}
	return dst
}

type pushHook struct {
	CreatedDate     string `json:"createdDate"`
	DetailedMessage struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"detailedMessage"`
	EventType string `json:"eventType"`
	ID        string `json:"id"`
	Message   struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"message"`
	PublisherID string `json:"publisherId"`
	Resource    struct {
		Commits []struct {
			Author struct {
				Date  time.Time `json:"date"`
				Email string    `json:"email"`
				Name  string    `json:"name"`
			} `json:"author"`
			Comment   string `json:"comment"`
			CommitID  string `json:"commitId"`
			Committer struct {
				Date  time.Time `json:"date"`
				Email string    `json:"email"`
				Name  string    `json:"name"`
			} `json:"committer"`
			URL string `json:"url"`
		} `json:"commits"`
		Date     string `json:"date"`
		PushID   int64  `json:"pushId"`
		PushedBy struct {
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			UniqueName  string `json:"uniqueName"`
			ImageURL    string `json:"imageUrl"`
		} `json:"pushedBy"`
		RefUpdates []struct {
			Name        string `json:"name"`
			NewObjectID string `json:"newObjectId"`
			OldObjectID string `json:"oldObjectId"`
		} `json:"refUpdates"`
		Repository struct {
			DefaultBranch string `json:"defaultBranch"`
			ID            string `json:"id"`
			Name          string `json:"name"`
			Project       struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				State string `json:"state"`
				URL   string `json:"url"`
			} `json:"project"`
			RemoteURL string `json:"remoteUrl"`
			URL       string `json:"url"`
		} `json:"repository"`
		URL string `json:"url"`
	} `json:"resource"`
	ResourceContainers struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
	ResourceVersion string `json:"resourceVersion"`
	Scope           string `json:"scope"`
}

type createPullRequestHook struct {
	ID          string `json:"id"`
	EventType   string `json:"eventType"`
	PublisherID string `json:"publisherId"`
	Scope       string `json:"scope"`
	Message     struct {
		Text     string `json:"text"`
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
	} `json:"message"`
	DetailedMessage struct {
		Text     string `json:"text"`
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
	} `json:"detailedMessage"`
	Resource struct {
		Repository struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			URL     string `json:"url"`
			WebURL  string `json:"webUrl"`
			SSHURL  string `json:"sshUrl"`
			Project struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				URL   string `json:"url"`
				State string `json:"state"`
			} `json:"project"`
			DefaultBranch string `json:"defaultBranch"`
			RemoteURL     string `json:"remoteUrl"`
		} `json:"repository"`
		PullRequestID int    `json:"pullRequestId"`
		Status        string `json:"status"`
		CreatedBy     struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
			ImageURL    string `json:"imageUrl"`
		} `json:"createdBy"`
		CreationDate          time.Time `json:"creationDate"`
		Title                 string    `json:"title"`
		Description           string    `json:"description"`
		SourceRefName         string    `json:"sourceRefName"`
		TargetRefName         string    `json:"targetRefName"`
		MergeStatus           string    `json:"mergeStatus"`
		MergeID               string    `json:"mergeId"`
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeSourceCommit"`
		LastMergeTargetCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeTargetCommit"`
		LastMergeCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeCommit"`
		Reviewers []struct {
			ReviewerURL interface{} `json:"reviewerUrl"`
			Vote        int         `json:"vote"`
			ID          string      `json:"id"`
			DisplayName string      `json:"displayName"`
			UniqueName  string      `json:"uniqueName"`
			URL         string      `json:"url"`
			ImageURL    string      `json:"imageUrl"`
			IsContainer bool        `json:"isContainer"`
		} `json:"reviewers"`
		URL string `json:"url"`
	} `json:"resource"`
	ResourceVersion    string `json:"resourceVersion"`
	ResourceContainers struct {
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
	CreatedDate time.Time `json:"createdDate"`
}

type updatePullRequestHook struct {
	CreatedDate     string `json:"createdDate"`
	DetailedMessage struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"detailedMessage"`
	EventType string `json:"eventType"`
	ID        string `json:"id"`
	Message   struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"message"`
	PublisherID string `json:"publisherId"`
	Resource    struct {
		ClosedDate null.String `json:"closedDate"`
		Commits    []struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"commits"`
		CreatedBy struct {
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"createdBy"`
		CreationDate    time.Time `json:"creationDate"`
		Description     string    `json:"description"`
		LastMergeCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeCommit"`
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeSourceCommit"`
		LastMergeTargetCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeTargetCommit"`
		MergeID       string `json:"mergeId"`
		MergeStatus   string `json:"mergeStatus"`
		PullRequestID int    `json:"pullRequestId"`
		Repository    struct {
			DefaultBranch string `json:"defaultBranch"`
			ID            string `json:"id"`
			Name          string `json:"name"`
			Project       struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				State string `json:"state"`
				URL   string `json:"url"`
			} `json:"project"`
			RemoteURL string `json:"remoteUrl"`
			URL       string `json:"url"`
			WebURL    string `json:"webUrl"`
			SSHURL    string `json:"sshUrl"`
		} `json:"repository"`
		Reviewers []struct {
			DisplayName string      `json:"displayName"`
			ID          string      `json:"id"`
			ImageURL    string      `json:"imageUrl"`
			IsContainer bool        `json:"isContainer"`
			ReviewerURL interface{} `json:"reviewerUrl"`
			UniqueName  string      `json:"uniqueName"`
			URL         string      `json:"url"`
			Vote        int64       `json:"vote"`
		} `json:"reviewers"`
		SourceRefName string `json:"sourceRefName"`
		Status        string `json:"status"`
		TargetRefName string `json:"targetRefName"`
		Title         string `json:"title"`
		URL           string `json:"url"`
	} `json:"resource"`
	ResourceContainers struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
	ResourceVersion string `json:"resourceVersion"`
	Scope           string `json:"scope"`
}

type mergePullRequestHook struct {
	CreatedDate     string `json:"createdDate"`
	DetailedMessage struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"detailedMessage"`
	EventType string `json:"eventType"`
	ID        string `json:"id"`
	Message   struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"message"`
	PublisherID string `json:"publisherId"`
	Resource    struct {
		ClosedDate string `json:"closedDate"`
		CreatedBy  struct {
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"createdBy"`
		CreationDate    time.Time `json:"creationDate"`
		Description     string    `json:"description"`
		LastMergeCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeCommit"`
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeSourceCommit"`
		LastMergeTargetCommit struct {
			CommitID string `json:"commitId"`
			URL      string `json:"url"`
		} `json:"lastMergeTargetCommit"`
		MergeID       string `json:"mergeId"`
		MergeStatus   string `json:"mergeStatus"`
		PullRequestID int    `json:"pullRequestId"`
		Repository    struct {
			DefaultBranch string `json:"defaultBranch"`
			ID            string `json:"id"`
			Name          string `json:"name"`
			Project       struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				State string `json:"state"`
				URL   string `json:"url"`
			} `json:"project"`
			RemoteURL string `json:"remoteUrl"`
			URL       string `json:"url"`
			WebURL    string `json:"webUrl"`
			SSHURL    string `json:"sshUrl"`
		} `json:"repository"`
		Reviewers []struct {
			DisplayName string      `json:"displayName"`
			ID          string      `json:"id"`
			ImageURL    string      `json:"imageUrl"`
			IsContainer bool        `json:"isContainer"`
			ReviewerURL interface{} `json:"reviewerUrl"`
			UniqueName  string      `json:"uniqueName"`
			URL         string      `json:"url"`
			Vote        int64       `json:"vote"`
		} `json:"reviewers"`
		SourceRefName string `json:"sourceRefName"`
		Status        string `json:"status"`
		TargetRefName string `json:"targetRefName"`
		Title         string `json:"title"`
		URL           string `json:"url"`
	} `json:"resource"`
	ResourceContainers struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
	ResourceVersion string `json:"resourceVersion"`
	Scope           string `json:"scope"`
}

type issueCommentPullRequestHook struct {
	CreatedDate     string `json:"createdDate"`
	DetailedMessage struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"detailedMessage"`
	EventType string `json:"eventType"`
	ID        string `json:"id"`
	Message   struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"message"`
	PublisherID string `json:"publisherId"`
	Resource    struct {
		PullRequest struct {
			CreatedBy struct {
				DisplayName string `json:"displayName"`
				ID          string `json:"id"`
				ImageURL    string `json:"imageUrl"`
				UniqueName  string `json:"uniqueName"`
				URL         string `json:"url"`
			} `json:"createdBy"`
			CreationDate    time.Time `json:"creationDate"`
			Description     string    `json:"description"`
			LastMergeCommit struct {
				CommitID string `json:"commitId"`
				Author   struct {
					Date  time.Time `json:"date"`
					Email string    `json:"email"`
					Name  string    `json:"name"`
				} `json:"author"`
				URL string `json:"url"`
			} `json:"lastMergeCommit"`
			LastMergeSourceCommit struct {
				CommitID string `json:"commitId"`
				URL      string `json:"url"`
			} `json:"lastMergeSourceCommit"`
			LastMergeTargetCommit struct {
				CommitID string `json:"commitId"`
				URL      string `json:"url"`
			} `json:"lastMergeTargetCommit"`
			MergeID       string `json:"mergeId"`
			MergeStatus   string `json:"mergeStatus"`
			PullRequestID int    `json:"pullRequestId"`
			Repository    struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Project struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					State string `json:"state"`
					URL   string `json:"url"`
				} `json:"project"`
				RemoteURL string `json:"remoteUrl"`
				URL       string `json:"url"`
				WebURL    string `json:"webUrl"`
				SSHURL    string `json:"sshUrl"`
			} `json:"repository"`
			Reviewers []struct {
				DisplayName string      `json:"displayName"`
				ID          string      `json:"id"`
				ImageURL    string      `json:"imageUrl"`
				IsContainer bool        `json:"isContainer"`
				ReviewerURL interface{} `json:"reviewerUrl"`
				UniqueName  string      `json:"uniqueName"`
				URL         string      `json:"url"`
				Vote        int64       `json:"vote"`
			} `json:"reviewers"`
			SourceRefName string `json:"sourceRefName"`
			Status        string `json:"status"`
			TargetRefName string `json:"targetRefName"`
			Title         string `json:"title"`
			URL           string `json:"url"`
		} `json:"pullRequest"`
		Comment struct {
			ID                     int       `json:"id"`
			ParentCommentID        int       `json:"parentCommentId"`
			Content                string    `json:"content"`
			PublishedDate          time.Time `json:"publishedDate"`
			LastUpdatedDate        time.Time `json:"lastUpdatedDate"`
			LastContentUpdatedDate time.Time `json:"lastContentUpdatedDate"`
			CommentType            string    `json:"commentType"`
			IsDeleted              bool      `json:"isDeleted"`
			Author                 struct {
				DisplayName string `json:"displayName"`
				ID          string `json:"id"`
				ImageURL    string `json:"imageUrl"`
				UniqueName  string `json:"uniqueName"`
				URL         string `json:"url"`
			} `json:"author"`
		} `json:"comment"`
	} `json:"resource"`
	ResourceContainers struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	} `json:"resourceContainers"`
	ResourceVersion string `json:"resourceVersion"`
	Scope           string `json:"scope"`
}
