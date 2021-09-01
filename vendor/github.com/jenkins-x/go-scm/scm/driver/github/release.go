package github

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type releaseService struct {
	client *wrapper
}

type release struct {
	ID          int       `json:"id"`
	Title       string    `json:"name"`
	Description string    `json:"body"`
	Link        string    `json:"html_url,omitempty"`
	Tag         string    `json:"tag_name,omitempty"`
	Commitish   string    `json:"target_commitish,omitempty"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Created     time.Time `json:"created_at"`
	Published   time.Time `json:"published_at"`
}

type releaseInput struct {
	Title       string `json:"name"`
	Description string `json:"body"`
	Tag         string `json:"tag_name"`
	Commitish   string `json:"target_commitish"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

func (s *releaseService) Find(ctx context.Context, repo string, id int) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases/%d", repo, id)
	out := new(release)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRelease(out), res, err
}

func (s *releaseService) FindByTag(ctx context.Context, repo string, tag string) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases/tags/%s", repo, tag)
	out := new(release)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRelease(out), res, err
}

func (s *releaseService) List(ctx context.Context, repo string, opts scm.ReleaseListOptions) ([]*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases?%s", repo, encodeReleaseListOptions(opts))
	out := []*release{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertReleaseList(out), res, err
}

func (s *releaseService) Create(ctx context.Context, repo string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases", repo)
	in := &releaseInput{
		Title:       input.Title,
		Commitish:   input.Commitish,
		Description: input.Description,
		Draft:       input.Draft,
		Prerelease:  input.Prerelease,
		Tag:         input.Tag,
	}
	out := new(release)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRelease(out), res, err
}

func (s *releaseService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases/%d", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *releaseService) DeleteByTag(ctx context.Context, repo string, tag string) (*scm.Response, error) {
	rel, _, _ := s.FindByTag(ctx, repo, tag)
	return s.Delete(ctx, repo, rel.ID)
}

func (s *releaseService) Update(ctx context.Context, repo string, id int, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/releases/%d", repo, id)
	in := &releaseInput{}
	if input.Title != "" {
		in.Title = input.Title
	}
	if input.Description != "" {
		in.Description = input.Description
	}
	if input.Commitish != "" {
		in.Commitish = input.Commitish
	}
	if input.Tag != "" {
		in.Tag = input.Tag
	}
	in.Draft = input.Draft
	in.Prerelease = input.Prerelease
	out := new(release)
	res, err := s.client.do(ctx, "PATCH", path, in, out)
	return convertRelease(out), res, err
}

func (s *releaseService) UpdateByTag(ctx context.Context, repo string, tag string, input *scm.ReleaseInput) (*scm.Release, *scm.Response, error) {
	rel, _, _ := s.FindByTag(ctx, repo, tag)
	return s.Update(ctx, repo, rel.ID, input)
}

func convertReleaseList(from []*release) []*scm.Release {
	var to []*scm.Release
	for _, m := range from {
		to = append(to, convertRelease(m))
	}
	return to
}

func convertRelease(from *release) *scm.Release {
	return &scm.Release{
		ID:          from.ID,
		Title:       from.Title,
		Description: from.Description,
		Link:        from.Link,
		Tag:         from.Tag,
		Commitish:   from.Commitish,
		Draft:       from.Draft,
		Prerelease:  from.Prerelease,
		Created:     from.Created,
		Published:   from.Published,
	}
}
