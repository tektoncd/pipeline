// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type gitService struct {
	client *wrapper
}

func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/refs/update-refs?view=azure-devops-rest-6.0
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/refs?api-version=6.0", ro.org, ro.project, ro.name)

	in := make(gitRefs, 1)
	in[0].Name = scm.ExpandRef(ref, "refs/heads")
	in[0].NewObjectID = sha
	in[0].OldObjectID = "0000000000000000000000000000000000000000"

	out := gitRefsResult{}
	res, err := s.client.do(ctx, "POST", endpoint, in, &out)

	return findGitRef(out, ref, in[0].Name), res, err

}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	_, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) FindCommit(ctx context.Context, repo, ref string) (*scm.Commit, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/commits/get?view=azure-devops-rest-6.0#get-by-id
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/commits/%s?api-version=6.0", ro.org, ro.project, ro.name, ref)
	out := new(gitCommit)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	return convertCommit(out), res, err
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListBranches(ctx context.Context, repo string, _ *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/refs/list?view=azure-devops-rest-6.0
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/refs?includeMyBranches=true&api-version=6.0", ro.org, ro.project, ro.name)
	out := new(branchList)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertBranchList(out.Value), res, err
}

func (s *gitService) ListCommits(ctx context.Context, repo string, opts scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/commits/get-commits?view=azure-devops-rest-6.0
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/commits?", ro.org, ro.project, ro.name)
	if opts.Ref != "" {
		endpoint += fmt.Sprintf("searchCriteria.itemVersion.version=%s&", opts.Ref)
	}
	if opts.Path != "" {
		endpoint += fmt.Sprintf("searchCriteria.itemPath=%s&", opts.Path)
	}
	endpoint += "api-version=6.0"

	out := new(commitList)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertCommitList(out.Value), res, err
}

func (s *gitService) ListTags(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, _ *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) CompareCommits(ctx context.Context, repo, source, target string, _ *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/diffs/get?view=azure-devops-rest-6.0
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/diffs/commits?", ro.org, ro.project, ro.name)
	// add base
	endpoint += fmt.Sprintf("baseVersion=%s&baseVersionType=commit&", source)
	// add target
	endpoint += fmt.Sprintf("targetVersion=%s&targetVersionType=commit&api-version=6.0", target)
	out := new(compare)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)

	changes := out.Changes
	return convertChangeList(changes), res, err
}

type gitRef struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

type gitRefsResult struct {
	Value gitRefs `json:"value"`
	Count int     `json:"count"`
}
type gitRefs []gitRef

type branchList struct {
	Value []*branch `json:"value"`
	Count int       `json:"count"`
}

type branch struct {
	Name     string `json:"name"`
	ObjectID string `json:"objectId"`
	Creator  struct {
		DisplayName string `json:"displayName"`
		URL         string `json:"url"`
		Links       struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"_links"`
		ID         string `json:"id"`
		UniqueName string `json:"uniqueName"`
		ImageURL   string `json:"imageUrl"`
		Descriptor string `json:"descriptor"`
	} `json:"creator"`
	URL string `json:"url"`
}

type commitList struct {
	Value []*gitCommit `json:"value"`
	Count int          `json:"count"`
}
type gitCommit struct {
	CommitID string `json:"commitId"`
	Author   struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"author"`
	Committer struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"committer"`
	Comment          string `json:"comment"`
	CommentTruncated bool   `json:"commentTruncated"`
	ChangeCounts     struct {
		Add    int `json:"Add"`
		Edit   int `json:"Edit"`
		Delete int `json:"Delete"`
	} `json:"changeCounts"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

type file struct {
	ChangeType string `json:"changeType"`
	Item       struct {
		CommitID         string `json:"commitId"`
		GitObjectType    string `json:"gitObjectType"`
		IsFolder         bool   `json:"isFolder"`
		ObjectID         string `json:"objectId"`
		OriginalObjectID string `json:"originalObjectId"`
		Path             string `json:"path"`
		URL              string `json:"url"`
	} `json:"item"`
}

type compare struct {
	AheadCount         int64  `json:"aheadCount"`
	AllChangesIncluded bool   `json:"allChangesIncluded"`
	BaseCommit         string `json:"baseCommit"`
	BehindCount        int64  `json:"behindCount"`
	ChangeCounts       struct {
		Add  int64 `json:"Add"`
		Edit int64 `json:"Edit"`
	} `json:"changeCounts"`
	Changes      []*file `json:"changes"`
	CommonCommit string  `json:"commonCommit"`
	TargetCommit string  `json:"targetCommit"`
}

func convertBranchList(from []*branch) []*scm.Reference {
	to := []*scm.Reference{}
	for _, v := range from {
		to = append(to, convertBranch(v))
	}
	return to
}

func findGitRef(from gitRefsResult, refName, refPath string) *scm.Reference {
	for _, ref := range from.Value {
		if ref.Name == refPath {
			return &scm.Reference{
				Name: refName,
				Path: refPath,
				Sha:  ref.NewObjectID,
			}
		}
	}
	return &scm.Reference{}
}

func convertBranch(from *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(from.Name),
		Path: from.Name,
		Sha:  from.ObjectID,
	}
}

func convertCommitList(from []*gitCommit) []*scm.Commit {
	to := []*scm.Commit{}
	for _, v := range from {
		to = append(to, convertCommit(v))
	}
	return to
}

func convertCommit(from *gitCommit) *scm.Commit {
	return &scm.Commit{
		Message: from.Comment,
		Sha:     from.CommitID,
		Link:    from.URL,
		Author: scm.Signature{
			Login: from.Author.Name,
			Name:  from.Author.Name,
			Email: from.Author.Email,
			Date:  from.Author.Date,
		},
		Committer: scm.Signature{
			Login: from.Committer.Name,
			Name:  from.Committer.Name,
			Email: from.Committer.Email,
			Date:  from.Committer.Date,
		},
	}
}

func convertChangeList(from []*file) []*scm.Change {
	to := []*scm.Change{}
	for _, v := range from {
		to = append(to, convertChange(v))
	}
	return to
}

func convertChange(from *file) *scm.Change {
	returnVal := &scm.Change{
		Path: from.Item.Path,
	}
	switch from.ChangeType {
	case "add":
		returnVal.Added = true
	case "delete":
		returnVal.Deleted = true
	case "rename":
		returnVal.Renamed = true
	}

	return returnVal
}
